// Purpose: authorize and bind the manual pipeline's discount before checkout, including uncertain-request replay.
// Depends on: buyer capability registration, merchant scope/permissions and storefront.manual_order_discounts (0220).
// Used by: ManualOrders.pipeline for both manual and for-buyer orders.
// Invariants: authorization is immutable per synthetic owner, even without a discount; a changed request conflicts before an order/hold.

package merchanttools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/platform"
)

func (m *ManualOrders) authorizeDiscount(ctx context.Context, token, ownerID, storeID string, in ManualInput) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	return platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, s platform.Scope) error {
		if err := requireCatalog(ctx, tx, s, token); err != nil {
			return err
		}
		var name, amount any
		if in.Discount != nil {
			name, amount = in.Discount.Name, in.Discount.AmountMinor
		}
		if _, err := tx.Exec(ctx, `INSERT INTO storefront.manual_order_discounts(tenant_id,store_id,owner_id,principal_id,request_hash,name,amount_minor)
   VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(tenant_id,store_id,owner_id) DO NOTHING`, s.TenantID, s.StoreID, ownerID, s.PrincipalID, hash[:], name, amount); err != nil {
			return err
		}
		var saved []byte
		if err := tx.QueryRow(ctx, `SELECT request_hash FROM storefront.manual_order_discounts WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3`, s.TenantID, s.StoreID, ownerID).Scan(&saved); err != nil {
			return err
		}
		if !bytes.Equal(saved, hash[:]) {
			return &Error{Status: 409, Code: "idempotency_conflict"}
		}
		return nil
	})
}

func (m *ManualOrders) bindDiscount(ctx context.Context, token, storeID, quoteID string) error {
	return platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, s platform.Scope) error {
		if err := requireCatalog(ctx, tx, s, token); err != nil {
			return err
		}
		// Quote ownership is checked by the FK plus this scoped definer; merchant runtime
		// cannot directly read buyer quote snapshots or mutate any quoted money.
		hash := sha256.Sum256([]byte(token))
		_, err := tx.Exec(ctx, `SELECT fulfillment.bind_manual_order_discount($1,$2::uuid,$3::uuid)`, hash[:], storeID, quoteID)
		return err
	})
}
