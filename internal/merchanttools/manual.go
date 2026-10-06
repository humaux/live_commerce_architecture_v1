// manual.go is the merchant-created ("manual", admin Create Order) order of contract G3. It adds NO checkout rule: the merchant's SKUs and
// the customer's details go through the buyer path unchanged under a server-held buyer capability (buyer.Service.RegisterForTrustedStore
// with a token derived by HMAC from the Idempotency-Key): storefront.SetCart -> CreateQuote (the server price, the request has no price
// field) -> storefront.SetDestination / checkout.BuyerCVS.EnterStore -> checkout.Service.Begin (stock reserved by begin_hold, the same
// expiry job, the bank-transfer window, pay-at-pickup limits, the optional buyer email). Afterwards ONE merchant transaction records the
// command receipt, marks the order source through fulfillment.mark_order_merchant_manual (migration 0094, inventory:reserve re-verified in
// SQL) and writes the audit row order.manual_created.
//
// Non-goals: no card (a merchant cannot take a card for a buyer; ListOptions' card mode is dropped and Begin never sees it), no price,
// discount or shipping override, no stock write of its own, no payment fact, no message to the buyer, no note column (a free-text note would
// carry buyer PII outside the erasure paths; see output/merchant-tools/DEVIATIONS.md). The pipeline spans several transactions on three
// pools, so it is NOT atomic: every step is idempotent by a key derived from the merchant's Idempotency-Key, a retry replays the finished
// steps, and the order itself is created by exactly one atomic begin_hold.
// External services: none. Merchant pool = commerce_runtime; buyer pool = commerce_buyer_runtime; checkout pool = commerce_checkout_runtime
// (through checkout.Service); capability issuer = commerce_buyer_issuer (through buyer.Service).

package merchanttools

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/domains"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
	"livecommerce/internal/storefront"
)

const (
	manualOperation  = "merchanttools.order.manual"
	relinkOperation  = "merchanttools.order.relink"
	manualPermission = "inventory:reserve"
	maxManualLines   = 50
	maxManualQty     = 1000
)

// errNeedWork is the sentinel the replay probe returns from command.Run's function: "no receipt yet, run the pipeline".
var errNeedWork = errors.New("no receipt yet")

// pipelineHooks are the two server-only inputs an order made FOR a buyer (order_for_buyer.go) adds to the unchanged pipeline: the claim origins of
// the cart lines (CartInput.Origins is json:"-", never a client value) and a step that runs right after CreateQuote on the buyer pool to bind the
// quote's merchant-origin grant to that quote (migration 0129). A plain manual order carries none, so Place behaves exactly as before.
type pipelineHooks struct {
	origins    map[string]storefront.ClaimOrigin
	afterQuote func(ctx context.Context, capability, storeID, quoteID string) error
}

type pipelineHooksKey struct{}

func withPipelineHooks(ctx context.Context, h *pipelineHooks) context.Context {
	return context.WithValue(ctx, pipelineHooksKey{}, h)
}

func hooksFrom(ctx context.Context) *pipelineHooks {
	h, _ := ctx.Value(pipelineHooksKey{}).(*pipelineHooks)
	return h
}

var (
	optionKeyRx = regexp.MustCompile(`^[0-9a-f-]{36}\|[A-Z]{2}\|[a-z][a-z0-9_-]{0,39}$`)
	// The SQL twin is checkout.set_order_buyer_email's regexp.
	emailRx = regexp.MustCompile(`^[^\s\x00-\x1f\x7f@]+@[^\s\x00-\x1f\x7f@]+$`)
	phoneRx = regexp.MustCompile(`^[0-9+() -]{6,32}$`)
	locales = []string{"zh-CN", "zh-TW", "en"}
)

type ManualItem struct {
	SKUID    string `json:"sku_id"`
	Quantity int64  `json:"quantity"`
}

type ManualCustomer struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

type ManualCVS struct {
	StoreCode    string `json:"store_code"`
	StoreName    string `json:"store_name"`
	StoreAddress string `json:"store_address"`
}

type ManualDelivery struct {
	OptionKey   string                  `json:"option_key"`
	HomeAddress *storefront.HomeAddress `json:"home_address"`
	CVS         *ManualCVS              `json:"cvs"`
}

// ManualInput is the exact POST orders/manual body (no price, no card).
type ManualInput struct {
	Items       []ManualItem   `json:"items"`
	Customer    ManualCustomer `json:"customer"`
	Delivery    ManualDelivery `json:"delivery"`
	PaymentMode string         `json:"payment_mode"`
	Locale      string         `json:"locale"`
}

// ManualOption is one delivery choice of GET orders/manual/options (the buyer checkout option, card dropped).
type ManualOption struct {
	OptionKey         string   `json:"option_key"`
	MarketID          string   `json:"market_id"`
	Country           string   `json:"country"`
	DeliveryCode      string   `json:"delivery_code"`
	DeliveryKind      string   `json:"delivery_kind"`
	Mode              string   `json:"mode"`
	NameHans          string   `json:"name_hans"`
	NameHant          string   `json:"name_hant"`
	NameEN            string   `json:"name_en"`
	Currency          string   `json:"currency"`
	ServiceVersion    int64    `json:"service_version"`
	AllocationVersion int64    `json:"allocation_version"`
	PaymentModes      []string `json:"payment_modes"`
	PickupSelection   *string  `json:"pickup_selection"`
	// home-cod R5 (migration 0107): present only on a home row whose PaymentModes lists "cash_on_delivery", copied from the checkout option.
	// Whole TWD in minor units; CodSurchargeMinor is omitted at 0, CodCarrier and CodMaxMinor are always there. The quote stays the only price.
	CodSurchargeMinor int64  `json:"cod_surcharge_minor,omitempty"`
	CodCarrier        string `json:"cod_carrier,omitempty"`
	CodMaxMinor       int64  `json:"cod_max_minor,omitempty"`
}

// manualReceipt is what the command receipt keeps. It never holds the capability: the link is re-derived from the Idempotency-Key.
type manualReceipt struct {
	OrderID         string    `json:"order_id"`
	CommercialState string    `json:"commercial_state"`
	PaymentMode     string    `json:"payment_mode"`
	TotalMinor      int64     `json:"total_minor"`
	Currency        string    `json:"currency"`
	ExpiresAt       time.Time `json:"expires_at"`
}

// ManualResult is the 201 (or replayed 200) body.
type ManualResult struct {
	manualReceipt
	BuyerLink *string `json:"buyer_link"`
	LinkState string  `json:"link_state"`
	Source    string  `json:"source"`
}

// ManualRegenerateInput is the exact POST orders/manual/regenerate-link body (K3 F2): the order whose buyer link is re-issued and the locale
// of the link the storefront opens. The manual order's locale is NULL in the database (0097 only sets it for buyer checkout), so it travels here.
type ManualRegenerateInput struct {
	OrderID string `json:"order_id"`
	Locale  string `json:"locale"`
}

// regenerateReceipt is what the command receipt keeps. It never holds the link or the token: both are re-derived from the Idempotency-Key, so
// a replay returns the same link and a different key is a different link (the old one already invalidated in SQL).
type regenerateReceipt struct {
	OrderID string `json:"order_id"`
}

// RegenerateResult is the 201 (or replayed 200) body.
type RegenerateResult struct {
	regenerateReceipt
	BuyerLink string `json:"buyer_link"`
	Source    string `json:"source"`
}

// ManualOrders runs the pipeline. All pools belong to the process that built the buyer surface (cmd/api buildBuyerWithCVS).
type ManualOrders struct {
	pool      *pgxpool.Pool
	issuer    *buyer.Service
	buyerPool *pgxpool.Pool
	checkout  *checkout.Service
	secret    []byte
}

// NewManualOrders wires the pipeline. issuer must have been built with the 30-day capability TTL; secret (>= 32 bytes) keys the HMAC that
// derives the capability token, so it must be a deployment secret (the buyer BFF key).
func NewManualOrders(pool *pgxpool.Pool, issuer *buyer.Service, buyerPool *pgxpool.Pool, service *checkout.Service, secret []byte) (*ManualOrders, error) {
	if pool == nil || issuer == nil || buyerPool == nil || service == nil || len(secret) < 32 {
		return nil, command.ErrInvalid
	}
	return &ManualOrders{pool: pool, issuer: issuer, buyerPool: buyerPool, checkout: service, secret: slices.Clone(secret)}, nil
}

// capability derives the buyer capability token of one (label, store, key): the same inputs always give the same token, so a retry after an
// uncertain failure resumes the same buyer owner instead of orphaning a hold under a new one.
func (m *ManualOrders) capability(label, store, key string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte("merchanttools." + label + "|" + store + "|" + key))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// stepKey derives a buyer-command / checkout key from the merchant key and the step name; hex keeps the 8..128 [A-Za-z0-9_.:-] grammar.
func stepKey(key, step string) string {
	sum := sha256.Sum256([]byte("mt|" + step + "|" + key))
	return "mt-" + hex.EncodeToString(sum[:16])
}

// ValidateManual normalizes and validates a body before any database work. Delivery detail rules (CVS vs home against the option's kind,
// payment mode against the option) need the options and are checked in Place.
func ValidateManual(in ManualInput) (ManualInput, error) {
	bad := &Error{Status: http.StatusUnprocessableEntity, Code: "invalid_request"}
	if len(in.Items) < 1 || len(in.Items) > maxManualLines {
		return in, bad
	}
	seen := map[string]bool{}
	for _, it := range in.Items {
		if !command.ValidID(it.SKUID) || seen[it.SKUID] || it.Quantity < 1 || it.Quantity > maxManualQty {
			return in, bad
		}
		seen[it.SKUID] = true
	}
	in.Customer.Name, in.Customer.Email = strings.TrimSpace(in.Customer.Name), strings.TrimSpace(in.Customer.Email)
	in.Customer.Phone = strings.TrimSpace(in.Customer.Phone)
	if !plain(in.Customer.Name, 1, 120) || !phoneRx.MatchString(in.Customer.Phone) || digits(in.Customer.Phone) < 6 || digits(in.Customer.Phone) > 20 ||
		(in.Customer.Email != "" && (len(in.Customer.Email) > 254 || !emailRx.MatchString(in.Customer.Email))) {
		return in, bad
	}
	// cash_on_delivery (home-cod R5) is a mode the options list offers on a COD-enabled home row; Place then holds it to the chosen row's modes
	// (home delivery only) and begin_hold applies the whole-TWD total and the per-order cap.
	if (in.PaymentMode != "bank_transfer" && in.PaymentMode != "pay_at_pickup" && in.PaymentMode != "cash_on_delivery") || !slices.Contains(locales, in.Locale) ||
		!optionKeyRx.MatchString(in.Delivery.OptionKey) || (in.Delivery.HomeAddress == nil) == (in.Delivery.CVS == nil) {
		return in, bad
	}
	if c := in.Delivery.CVS; c != nil {
		c.StoreCode, c.StoreName, c.StoreAddress = strings.TrimSpace(c.StoreCode), strings.TrimSpace(c.StoreName), strings.TrimSpace(c.StoreAddress)
		if !plain(c.StoreCode, 1, 32) || !plain(c.StoreName, 1, 40) || !plain(c.StoreAddress, 5, 120) {
			return in, bad
		}
	}
	if h := in.Delivery.HomeAddress; h != nil {
		for _, f := range []*string{&h.Region, &h.City, &h.PostalCode, &h.Line1, &h.Line2} {
			*f = strings.TrimSpace(*f)
		}
		if !plain(h.City, 1, 100) || !plain(h.Line1, 1, 200) || (h.Region != "" && !plain(h.Region, 1, 100)) ||
			(h.PostalCode != "" && !plain(h.PostalCode, 1, 20)) || (h.Line2 != "" && !plain(h.Line2, 1, 200)) {
			return in, bad
		}
	}
	return in, nil
}

func plain(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	if !utf8.ValidString(s) || n < min || n > max {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func digits(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n++
		}
	}
	return n
}

// Options lists what a manual order can use: the buyer checkout options of the store with card removed. It authorizes with the same
// permission as Place and reads through a day-scoped derived capability (read-only; a new day derives a new, unexpired one).
func (m *ManualOrders) Options(ctx context.Context, token, storeID string) ([]ManualOption, error) {
	if m == nil {
		return nil, ErrManualDisabled
	}
	if err := platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		return requireCatalog(ctx, tx, scope, token)
	}); err != nil {
		return nil, err
	}
	capability := m.capability("options", storeID, time.Now().UTC().Format("2006-01-02"))
	options, err := m.options(ctx, capability, storeID)
	if err != nil {
		return nil, classifyPipeline(ctx, err)
	}
	return options, nil
}

func (m *ManualOrders) options(ctx context.Context, capability, storeID string) ([]ManualOption, error) {
	if _, err := m.issuer.RegisterForTrustedStore(ctx, storeID, capability); err != nil {
		return nil, err
	}
	out := []ManualOption{}
	cursor := ""
	for page := 0; page < 4; page++ {
		got, err := m.checkout.ListOptions(ctx, capability, storeID, checkout.OptionsRequest{Page: pagination.Request{Limit: 100, Cursor: cursor}})
		if err != nil {
			return nil, err
		}
		for _, o := range got.Items {
			if o.Available != nil && !*o.Available {
				continue
			}
			modes := []string{}
			for _, mode := range o.PaymentModes {
				if mode != "card" {
					modes = append(modes, mode)
				}
			}
			if len(modes) == 0 {
				continue // a row only card could complete is not usable by a merchant
			}
			var selection *string
			if o.PickupSelection != "" {
				s := o.PickupSelection
				selection = &s
			}
			out = append(out, ManualOption{OptionKey: o.MarketID + "|" + o.Country + "|" + o.DeliveryCode, MarketID: o.MarketID, Country: o.Country,
				DeliveryCode: o.DeliveryCode, DeliveryKind: o.DeliveryKind, Mode: o.Mode, NameHans: o.NameHans, NameHant: o.NameHant, NameEN: o.NameEN,
				Currency: o.Currency, ServiceVersion: o.ServiceVersion, AllocationVersion: o.AllocationVersion, PaymentModes: modes, PickupSelection: selection,
				CodSurchargeMinor: o.CodSurchargeMinor, CodCarrier: o.CodCarrier, CodMaxMinor: o.CodMaxMinor})
		}
		if got.NextCursor == "" {
			return out, nil
		}
		cursor = got.NextCursor
	}
	return out, nil
}

// Place creates (or replays) the order. replayed is true when the receipt of an earlier identical request answered.
func (m *ManualOrders) Place(ctx context.Context, token, storeID, key string, in ManualInput) (ManualResult, bool, error) {
	if m == nil {
		return ManualResult{}, false, ErrManualDisabled
	}
	in, err := ValidateManual(in)
	if err != nil {
		return ManualResult{}, false, err
	}
	var receipt manualReceipt
	var origin, state string
	replayed := false
	// Probe: authority, the storefront origin for the link, and any receipt of this key (command.Run replays it; nothing is written).
	err = platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		if inner := requireCatalog(ctx, tx, scope, token); inner != nil {
			return inner
		}
		var inner error
		if state, origin, inner = readOrigin(ctx, tx, token, scope); inner != nil {
			return inner
		}
		inner = command.Run(ctx, tx, scope, manualOperation, key, in, &receipt, func() error { return errNeedWork })
		switch {
		case inner == nil:
			replayed = true
		case errors.Is(inner, errNeedWork):
			inner = nil
		}
		return inner
	})
	if err != nil {
		return ManualResult{}, false, mapReceiptError(err)
	}
	capability := m.capability("order", storeID, key)
	// The link token is NOT the capability: it is a separate single-use secret (derived from the same key, so a replay re-derives the same link)
	// that the storefront exchanges for a fresh capability of the order's owner (checkout.redeem_order_link); the capability never leaves the server.
	linkToken := m.capability("link", storeID, key)
	if !replayed {
		if receipt, err = m.pipeline(ctx, capability, storeID, key, in); err != nil {
			return ManualResult{}, false, classifyPipeline(ctx, err)
		}
		// One merchant transaction: receipt + source mark + audit. A failure here leaves a placed order with source=storefront until the
		// merchant retries the same key (every earlier step replays and this one runs again).
		hash := sha256.Sum256([]byte(token))
		linkHash := sha256.Sum256([]byte(linkToken))
		var saved manualReceipt
		err = platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
			return command.Run(ctx, tx, scope, manualOperation, key, in, &saved, func() error {
				// fulfillment.mark_order_merchant_manual (0094): inventory:reserve re-verified in SQL, order of this store, <= 10 minutes old; it also
				// records sha256 of the buyer link token (single-use, 7 days), so the plaintext token exists only in this response.
				if _, err := tx.Exec(ctx, `SELECT fulfillment.mark_order_merchant_manual($1,$2::uuid,$3::uuid,$4)`, hash[:], storeID, receipt.OrderID, linkHash[:]); err != nil {
					return err
				}
				saved = receipt
				return command.Audit(ctx, tx, scope, "order.manual_created")
			})
		})
		if err != nil {
			return ManualResult{}, false, mapReceiptError(err)
		}
		receipt = saved
	}
	receipt.ExpiresAt = receipt.ExpiresAt.UTC() // the wire is RFC 3339 UTC ("Z"), which the admin parser requires; a replayed receipt decodes in local time
	out := ManualResult{manualReceipt: receipt, LinkState: state, Source: "merchant_manual"}
	if state == "configured" && domains.ValidOrigin(origin) {
		link := origin + "/" + in.Locale + "/order-link#o=" + receipt.OrderID + "&t=" + linkToken
		out.BuyerLink = &link
	}
	return out, replayed, nil
}

// RegenerateLink re-issues a manual order's buyer link (K3 F2). It re-verifies the merchant bearer the same way Place does (inventory:reserve
// via the scope, catalog:read via requireCatalog, the storefront origin via readOrigin), derives a NEW single-use link token from the
// Idempotency-Key, invalidates every still-unused old link of the order and records the new one (fulfillment.regenerate_order_link, 0104). It is
// idempotent by the key: a replay re-derives and returns the same link; a different key is a different link (the previous one already dead).
func (m *ManualOrders) RegenerateLink(ctx context.Context, token, storeID, key string, in ManualRegenerateInput) (RegenerateResult, bool, error) {
	if m == nil {
		return RegenerateResult{}, false, ErrManualDisabled
	}
	if !command.ValidID(in.OrderID) || !slices.Contains(locales, in.Locale) {
		return RegenerateResult{}, false, &Error{Status: http.StatusUnprocessableEntity, Code: "invalid_request"}
	}
	var receipt regenerateReceipt
	var origin string
	replayed := false
	// The new link token is NOT the capability and NOT the old link: a fresh single-use secret derived from the key, exchanged by the storefront
	// for a fresh capability of the order's owner (checkout.redeem_order_link); the old links are invalidated in SQL.
	linkToken := m.capability("relink", storeID, key)
	hash := sha256.Sum256([]byte(token))
	linkHash := sha256.Sum256([]byte(linkToken))
	err := platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		if inner := requireCatalog(ctx, tx, scope, token); inner != nil {
			return inner
		}
		var state string
		var inner error
		if state, origin, inner = readOrigin(ctx, tx, token, scope); inner != nil {
			return inner
		}
		if state != "configured" || !domains.ValidOrigin(origin) {
			return ErrUnavailable // no published storefront origin: there is no buyer link to re-issue
		}
		// Probe: any receipt of this key (command.Run replays it; nothing is written).
		inner = command.Run(ctx, tx, scope, relinkOperation, key, in, &receipt, func() error { return errNeedWork })
		switch {
		case inner == nil:
			replayed = true
		case errors.Is(inner, errNeedWork):
			inner = nil
		}
		if inner != nil {
			return inner
		}
		if replayed {
			return nil
		}
		return command.Run(ctx, tx, scope, relinkOperation, key, in, &receipt, func() error {
			if _, err := tx.Exec(ctx, `SELECT fulfillment.regenerate_order_link($1,$2::uuid,$3::uuid,$4)`, hash[:], storeID, in.OrderID, linkHash[:]); err != nil {
				return err
			}
			receipt = regenerateReceipt{OrderID: in.OrderID}
			return command.Audit(ctx, tx, scope, "order.manual_link_regenerated")
		})
	})
	if err != nil {
		return RegenerateResult{}, false, mapReceiptError(err)
	}
	out := RegenerateResult{regenerateReceipt: receipt, Source: "merchant_manual"}
	out.BuyerLink = origin + "/" + in.Locale + "/order-link#o=" + receipt.OrderID + "&t=" + linkToken
	return out, replayed, nil
}

// requireCatalog is the second permission of a manual order: picking SKUs needs catalog:read, and identity.resolve_storefront_origin (the link
// origin) checks it too, so a role with only inventory:reserve is refused up front with a plain 403 instead of a late definer error.
func requireCatalog(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) error {
	return platform.RequirePermission(ctx, tx, scope, token, "catalog:read")
}

// readOrigin resolves the published storefront origin (identity.resolve_storefront_origin re-verifies the bearer).
func readOrigin(ctx context.Context, tx pgx.Tx, token string, scope platform.Scope) (string, string, error) {
	hash := sha256.Sum256([]byte(token))
	var state, origin string
	if err := tx.QueryRow(ctx, `SELECT state,origin FROM identity.resolve_storefront_origin($1::bytea,$2::uuid)`, hash[:], scope.StoreID).Scan(&state, &origin); err != nil {
		return "", "", err
	}
	if state != "configured" && state != "storefront_unavailable" && state != "domain_selection_required" {
		return "", "", ErrUnavailable
	}
	return state, origin, nil
}

// pipeline runs the buyer path. Every step has its own derived key so a retry replays what already finished.
func (m *ManualOrders) pipeline(ctx context.Context, capability, storeID, key string, in ManualInput) (manualReceipt, error) {
	var zero manualReceipt
	if _, err := m.issuer.RegisterForTrustedStore(ctx, storeID, capability); err != nil {
		return zero, err
	}
	options, err := m.options(ctx, capability, storeID)
	if err != nil {
		return zero, err
	}
	i := slices.IndexFunc(options, func(o ManualOption) bool { return o.OptionKey == in.Delivery.OptionKey })
	if i < 0 {
		return zero, &Error{Status: http.StatusUnprocessableEntity, Code: "invalid_request"}
	}
	option := options[i]
	if !slices.Contains(option.PaymentModes, in.PaymentMode) {
		return zero, &Error{Status: http.StatusUnprocessableEntity, Code: in.PaymentMode + "_unavailable"}
	}
	home := option.DeliveryKind == "home"
	if home != (in.Delivery.HomeAddress != nil) {
		return zero, &Error{Status: http.StatusUnprocessableEntity, Code: "invalid_request"}
	}
	if !home && (option.PickupSelection == nil || *option.PickupSelection != "buyer_entered" || m.checkout.CVS() == nil) {
		return zero, &Error{Status: http.StatusUnprocessableEntity, Code: "cvs_entry_unavailable"}
	}

	items := make([]storefront.Item, len(in.Items))
	for k, it := range in.Items {
		items[k] = storefront.Item{SKUID: it.SKUID, Quantity: it.Quantity}
	}
	method, err := pricing.DeliveryMethod(option.DeliveryCode)
	if err != nil {
		return zero, err
	}
	hooks := hooksFrom(ctx)
	cartInput := storefront.CartInput{ExpectedVersion: 0, Items: items}
	if hooks != nil {
		cartInput.Origins = hooks.origins // for-buyer only: server-derived claim origins; the Quote below still decides every price
	}
	var cart storefront.Cart
	if err = buyer.WithScope(ctx, m.buyerPool, capability, storeID, func(c context.Context, tx pgx.Tx, s buyer.Scope) (e error) {
		cart, e = storefront.SetCart(c, tx, s, stepKey(key, "cart"), cartInput)
		return e
	}); err != nil {
		return zero, err
	}
	// The quote is the only price: CreateQuote prices every line from the catalog and applies the delivery policy (I05).
	var quote storefront.Quote
	if err = buyer.WithScope(ctx, m.buyerPool, capability, storeID, func(c context.Context, tx pgx.Tx, s buyer.Scope) (e error) {
		quote, e = storefront.CreateQuote(c, tx, s, stepKey(key, "quote"), storefront.QuoteInput{CartVersion: cart.Version, MarketID: option.MarketID,
			Country: option.Country, Method: method})
		return e
	}); err != nil {
		return zero, err
	}
	if hooks != nil && hooks.afterQuote != nil {
		if err = hooks.afterQuote(ctx, capability, storeID, quote.ID); err != nil {
			return zero, err
		}
	}
	destination := storefront.DestinationInput{ExpectedVersion: 0, CartVersion: cart.Version, Kind: option.DeliveryKind, Country: option.Country,
		RecipientName: in.Customer.Name, Phone: in.Customer.Phone}
	if home {
		destination.HomeAddress = *in.Delivery.HomeAddress
	} else {
		c := in.Delivery.CVS
		// checkout.BuyerCVS.EnterStore -> fulfillment.record_buyer_cvs_store: the same "buyer typed the store" record a buyer makes.
		entered, e := m.checkout.CVS().EnterStore(ctx, capability, storeID, stepKey(key, "store"), checkout.StoreEntryInput{CartVersion: cart.Version,
			MarketID: option.MarketID, ServiceCode: option.DeliveryCode, StoreCode: c.StoreCode, StoreName: c.StoreName, StoreAddress: c.StoreAddress})
		if e != nil {
			return zero, e
		}
		destination.PickupID = entered.PickupID
	}
	var selected storefront.Destination
	if err = buyer.WithScope(ctx, m.buyerPool, capability, storeID, func(c context.Context, tx pgx.Tx, s buyer.Scope) (e error) {
		selected, e = storefront.SetDestination(c, tx, s, stepKey(key, "destination"), destination)
		return e
	}); err != nil {
		return zero, err
	}
	// checkout.Service.Begin: the buyer begin path (begin_hold reserves stock, schedules the expiry job, applies the payment-mode rules).
	placed, err := m.checkout.Begin(ctx, capability, storeID, stepKey(key, "begin"), checkout.Input{QuoteID: quote.ID, DestinationID: selected.ID,
		CartVersion: cart.Version, ServiceVersion: option.ServiceVersion, AllocationVersion: option.AllocationVersion, PaymentMode: in.PaymentMode,
		BuyerEmail: in.Customer.Email})
	if err != nil {
		return zero, err
	}
	return manualReceipt{OrderID: placed.OrderID, CommercialState: placed.CommercialState, PaymentMode: placed.PaymentMode,
		TotalMinor: quote.Amount.TotalMinor, Currency: quote.Currency, ExpiresAt: placed.ExpiresAt}, nil
}

// classifyPipeline turns any pipeline failure into a coded refusal; nothing from a driver or a buyer-side error text escapes.
func classifyPipeline(ctx context.Context, err error) error {
	var coded *Error
	var cvs *fulfillment.CVSError
	switch {
	case errors.As(err, &coded):
		return coded
	case errors.As(err, &cvs):
		return &Error{Status: cvs.Status, Code: cvs.Code}
	case errors.Is(err, platform.ErrUnauthorized), errors.Is(err, platform.ErrForbidden), errors.Is(err, platform.ErrScopeNotFound):
		return err
	case ctx.Err() != nil:
		return ErrUnavailable
	case errors.Is(err, command.ErrInvalid), errors.Is(err, command.ErrNotFound):
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid_request"}
	case errors.Is(err, command.ErrInsufficient):
		return &Error{Status: http.StatusConflict, Code: "insufficient_inventory"}
	case errors.Is(err, command.ErrConflict):
		return &Error{Status: http.StatusConflict, Code: "conflict"}
	}
	return ErrUnavailable
}

// mapReceiptError maps the merchant-transaction failures (authority sentinels pass through; a PT401/403/404 of a definer too; a body
// that differs from the receipt under the same key is idempotency_conflict).
func mapReceiptError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "PT409" { // fulfillment.mark_order_merchant_manual: the order is older than 10 minutes
		return &Error{Status: http.StatusConflict, Code: "conflict"}
	}
	switch {
	case errors.Is(err, command.ErrConflict):
		return &Error{Status: http.StatusConflict, Code: "idempotency_conflict"}
	case errors.Is(err, command.ErrInvalid):
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid_request"}
	case errors.Is(err, platform.ErrUnauthorized), errors.Is(err, platform.ErrForbidden), errors.Is(err, platform.ErrScopeNotFound):
		return err
	}
	return mapDashboardError(err)
}
