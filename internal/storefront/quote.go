package storefront

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/pricing"    // The only calculator; snapshots never re-price on GET.
	"livecommerce/internal/promotions" // quote_check: a typed discount code becomes a frozen pricing.Promo (storefront-v2 §F).
)

type QuoteInput struct {
	CartVersion int64  `json:"cart_version"`
	MarketID    string `json:"market_id"`
	Country     string `json:"country"`
	Method      string `json:"method"`
	// PromoCode is an optional discount code the buyer typed (storefront-v2 §F). omitempty keeps the request digest of a code-less quote
	// unchanged, so pre-0091 replays still match. It is validated and priced here, never trusted as an amount.
	PromoCode string `json:"promo_code,omitempty"`
}
type QuoteLine struct {
	SKUID          string             `json:"sku_id"`
	ProductID      string             `json:"product_id"`
	Code           string             `json:"code"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	SKUVersion     int64              `json:"sku_version"`
	ProductVersion int64              `json:"product_version"`
	Quantity       int64              `json:"quantity"`
	UnitPriceMinor int64              `json:"unit_price_minor"`
	Amount         pricing.LineAmount `json:"amount"`
}
type Quote struct {
	ID                 string              `json:"id"`
	CartID             string              `json:"cart_id"`
	Currency           string              `json:"currency"`
	CalculationVersion string              `json:"calculation_version"`
	CartVersion        int64               `json:"cart_version"`
	MarketVersion      int64               `json:"market_version"`
	Policy             pricing.Policy      `json:"policy"`
	Lines              []QuoteLine         `json:"lines"`
	Amount             pricing.Calculation `json:"amount"`
	// Promotion is the code this quote was priced with (nil = none). omitempty: a code-less snapshot has no key, so pre-0091 quotes
	// round-trip byte-equal through checkout.begin_hold's snapshot comparison. Version freezes the merchant's code row (promo_changed).
	Promotion *pricing.Promo `json:"promotion,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	ExpiresAt time.Time      `json:"expires_at"`
}

func CreateQuote(ctx context.Context, tx pgx.Tx, s buyer.Scope, key string, in QuoteInput) (out Quote, err error) {
	if in.CartVersion < 1 || !command.ValidID(in.MarketID) || len(in.Country) != 2 || in.Country[0] < 'A' || in.Country[0] > 'Z' || in.Country[1] < 'A' || in.Country[1] > 'Z' || !pricing.ValidMethod(in.Method) {
		return out, command.ErrInvalid
	}
	err = buyer.RunCommand(ctx, tx, s, "quote.create", key, in, &out, func() error {
		cart, err := readCart(ctx, tx, s, false)
		if err != nil {
			return err
		}
		if cart.Version != in.CartVersion {
			return command.ErrConflict
		}
		if len(cart.Items) == 0 {
			return command.ErrInvalid
		}
		market, err := pricing.LockMarket(ctx, tx, s.TenantID, s.StoreID, in.MarketID)
		if err != nil {
			return err
		}
		if !market.Active || market.Currency != cart.Currency {
			return command.ErrConflict
		}
		policy, err := pricing.LockCurrent(ctx, tx, s.TenantID, s.StoreID, in.MarketID, in.Country, in.Method)
		if err != nil {
			return err
		}
		if policy.Currency != cart.Currency {
			return command.ErrConflict
		}
		lines, err := lockCatalog(ctx, tx, s, cart.Currency, cart.Items)
		if err != nil {
			return err
		}
		inputs := make([]pricing.AmountLine, len(lines))
		for i, line := range lines {
			inputs[i] = pricing.AmountLine{UnitPriceMinor: line.UnitPriceMinor, Quantity: line.Quantity}
		}
		var promo *pricing.Promo
		if in.PromoCode != "" {
			// §F: the code is checked against the PRE-discount merchandise subtotal of the locked catalog lines (never a client amount);
			// the effect it returns is frozen below. Advisory here: checkout's promotions.redeem is the authoritative, locking check.
			var subtotal int64
			for _, line := range inputs {
				part, err := command.CheckMoney(line.UnitPriceMinor, line.Quantity)
				if err != nil {
					return err
				}
				if subtotal += part; subtotal > command.MaxMoney {
					return command.ErrInvalid
				}
			}
			found, err := promotions.Check(ctx, tx, s.TenantID, s.StoreID, s.OwnerID, in.PromoCode, subtotal)
			if err != nil {
				return err
			}
			promo = &found
		}
		amount, err := pricing.CalculateWith(policy, inputs, promo)
		if err != nil {
			return err
		}
		if promo != nil && amount.DiscountMinor == 0 {
			// A code that takes nothing off this cart (a tiny cart rounded down to a whole currency unit) is refused rather than silently
			// burning one of its uses on a zero discount. Same answer as an unknown code: nothing to enumerate.
			return &promotions.Coded{Status: 422, Code: "promo_invalid"}
		}
		for i := range lines {
			lines[i].Amount = amount.Lines[i]
		}
		out = Quote{CartID: cart.ID, Currency: cart.Currency, CalculationVersion: "v1", CartVersion: cart.Version, MarketVersion: market.Version, Policy: policy, Lines: lines, Amount: amount, Promotion: promo}
		if err = tx.QueryRow(ctx, `SELECT gen_random_uuid()::text,clock_timestamp()`).Scan(&out.ID, &out.CreatedAt); err != nil {
			return err
		}
		out.ExpiresAt = out.CreatedAt.Add(time.Duration(policy.QuoteTTLSeconds) * time.Second)
		snapshot, err := json.Marshal(out)
		if err != nil {
			return err
		}
		if len(snapshot) > 1048576 {
			return command.ErrInvalid
		}
		_, err = tx.Exec(ctx, `INSERT INTO storefront.quotes(tenant_id,store_id,owner_id,id,cart_id,creator_session_id,cart_version,market_id,market_version,country,method,policy_version,currency,created_at,expires_at,snapshot)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, s.TenantID, s.StoreID, s.OwnerID, out.ID, cart.ID, s.SessionID, cart.Version, market.ID, market.Version, policy.Country, policy.Method, policy.Version, cart.Currency, out.CreatedAt, out.ExpiresAt, snapshot)
		if err != nil {
			return err
		}
		return event(ctx, tx, s, cart.ID, out.ID, "quote.created")
	})
	return out, err
}

// GetQuote is historical: expiration is returned, never extended. Future
// checkout must revalidate in its own atomic transaction before charging money.
func GetQuote(ctx context.Context, tx pgx.Tx, s buyer.Scope, id string) (out Quote, err error) {
	if !command.ValidID(id) {
		return out, command.ErrInvalid
	}
	if err = buyer.CheckScope(ctx, tx, s); err != nil {
		return out, err
	}
	return readQuote(ctx, tx, s, id)
}

// readQuote keeps the immutable JSON and relational header consistency check in
// one place. Callers must validate the ID and bind the buyer scope first.
func readQuote(ctx context.Context, tx pgx.Tx, s buyer.Scope, id string) (out Quote, err error) {
	var body []byte
	var cart, currency, market, country, method string
	var cartVersion, marketVersion, policyVersion int64
	var created, expires time.Time
	err = tx.QueryRow(ctx, `SELECT snapshot,cart_id::text,currency,market_id::text,country,method,cart_version,market_version,policy_version,created_at,expires_at FROM storefront.quotes WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND id=$4`, s.TenantID, s.StoreID, s.OwnerID, id).Scan(&body, &cart, &currency, &market, &country, &method, &cartVersion, &marketVersion, &policyVersion, &created, &expires)
	if err != nil {
		return out, notFound(err)
	}
	// Buyer runtime may insert snapshots. Malformed stored JSON is a conflicting
	// immutable fact, not a decoder detail to expose through Get/Revalidate.
	if err = json.Unmarshal(body, &out); err != nil {
		return Quote{}, command.ErrConflict
	}
	if out.ID != id || out.CartID != cart || out.Currency != currency || out.CartVersion != cartVersion || out.MarketVersion != marketVersion || out.CalculationVersion != "v1" || out.Policy.MarketID != market || out.Policy.Country != country || out.Policy.Method != method || out.Policy.Version != policyVersion || out.Policy.Currency != currency || !out.CreatedAt.Equal(created) || !out.ExpiresAt.Equal(expires) {
		return Quote{}, command.ErrConflict
	}
	return out, nil
}
