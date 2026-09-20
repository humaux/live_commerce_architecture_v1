package storefront

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/pricing"
)

// RevalidateQuote verifies an immutable quote against locked current inputs in
// the caller-owned buyer transaction. The result is not reusable authorization:
// after later destination or balance waits, the caller must recheck expiry
// immediately before its first durable hold write. This function never writes.
func RevalidateQuote(ctx context.Context, tx pgx.Tx, s buyer.Scope, quoteID string, cartVersion int64) (out Quote, err error) {
	if !command.ValidID(quoteID) || cartVersion < 1 {
		return out, command.ErrInvalid
	}
	if err = buyer.CheckScope(ctx, tx, s); err != nil {
		return out, err
	}
	if out, err = readQuote(ctx, tx, s, quoteID); err != nil {
		return Quote{}, err
	}

	cart, err := readCart(ctx, tx, s, false)
	if err != nil {
		return Quote{}, err
	}
	items, err := canonicalItems(cart.Items)
	if err != nil || len(items) == 0 {
		return Quote{}, command.ErrConflict
	}
	if out.CartID != cart.ID || out.Currency != cart.Currency ||
		out.CartVersion != cartVersion || cart.Version != cartVersion {
		return Quote{}, command.ErrConflict
	}

	market, err := pricing.LockMarket(ctx, tx, s.TenantID, s.StoreID, out.Policy.MarketID)
	if err != nil {
		return Quote{}, err
	}
	if !market.Active || market.ID != out.Policy.MarketID || market.Currency != out.Currency || market.Version != out.MarketVersion {
		return Quote{}, command.ErrConflict
	}
	policy, err := pricing.LockCurrent(ctx, tx, s.TenantID, s.StoreID, out.Policy.MarketID, out.Policy.Country, out.Policy.Method)
	if err != nil {
		return Quote{}, err
	}
	if policy != out.Policy {
		return Quote{}, command.ErrConflict
	}

	currentLines, err := lockCatalog(ctx, tx, s, cart.Currency, items)
	if err != nil {
		return Quote{}, err
	}
	if len(currentLines) != len(out.Lines) {
		return Quote{}, command.ErrConflict
	}
	inputs := make([]pricing.AmountLine, len(currentLines))
	for i, current := range currentLines {
		quoted := out.Lines[i]
		if quoted.SKUID != current.SKUID || quoted.ProductID != current.ProductID ||
			quoted.Code != current.Code || quoted.Name != current.Name || quoted.Description != current.Description ||
			quoted.SKUVersion != current.SKUVersion || quoted.ProductVersion != current.ProductVersion ||
			quoted.Quantity != current.Quantity || quoted.UnitPriceMinor != current.UnitPriceMinor {
			return Quote{}, command.ErrConflict
		}
		inputs[i] = pricing.AmountLine{UnitPriceMinor: current.UnitPriceMinor, Quantity: current.Quantity}
	}
	amount, err := pricing.Calculate(policy, inputs)
	if err != nil {
		return Quote{}, err
	}
	if !sameCalculation(out.Amount, amount) {
		return Quote{}, command.ErrConflict
	}
	for i := range out.Lines {
		if out.Lines[i].Amount != amount.Lines[i] {
			return Quote{}, command.ErrConflict
		}
	}

	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return Quote{}, err
	}
	expires := out.CreatedAt.Add(time.Duration(policy.QuoteTTLSeconds) * time.Second)
	if out.CreatedAt.After(now) || !out.ExpiresAt.After(now) || !out.ExpiresAt.Equal(expires) {
		return Quote{}, command.ErrConflict
	}
	return out, nil
}

func sameCalculation(left, right pricing.Calculation) bool {
	if left.SubtotalMinor != right.SubtotalMinor || left.DiscountMinor != right.DiscountMinor ||
		left.ShippingMinor != right.ShippingMinor || left.ShippingTaxMinor != right.ShippingTaxMinor ||
		left.TaxMinor != right.TaxMinor || left.TotalMinor != right.TotalMinor || len(left.Lines) != len(right.Lines) {
		return false
	}
	for i := range left.Lines {
		if left.Lines[i] != right.Lines[i] {
			return false
		}
	}
	return true
}
