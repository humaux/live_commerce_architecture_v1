// Purpose: read a merchant-authorized adjustment for a synthetic manual-order owner.
// Depends on: storefront.manual_order_discounts (0216), buyer/checkout scoped transactions and pricing types.
// Used by: CreateManualQuote and RevalidateQuote; public CreateQuote never requests a manual discount.
// Invariants: buyer roles cannot write authorization; checkout must match the exact merchant-bound quote.

package storefront

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/pricing"
)

func manualDiscount(ctx context.Context, tx pgx.Tx, s buyer.Scope, quoteID string) (*pricing.ManualDiscount, error) {
	var name *string
	var amount *int64
	var bound *string
	err := tx.QueryRow(ctx, `SELECT name,amount_minor,quote_id::text FROM storefront.manual_order_discounts
  WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3`, s.TenantID, s.StoreID, s.OwnerID).Scan(&name, &amount, &bound)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, command.ErrConflict
	}
	if err != nil {
		return nil, err
	}
	if (quoteID == "" && bound != nil) || (quoteID != "" && (bound == nil || *bound != quoteID)) {
		return nil, command.ErrConflict
	}
	if name == nil && amount == nil {
		return nil, nil
	}
	if name == nil || amount == nil {
		return nil, command.ErrConflict
	}
	d := pricing.ManualDiscount{Name: *name, AmountMinor: *amount}
	if !pricing.ValidManualDiscount(d) {
		return nil, command.ErrConflict
	}
	return &d, nil
}
