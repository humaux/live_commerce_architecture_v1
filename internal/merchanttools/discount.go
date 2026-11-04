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
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/platform"
)

func (m *ManualOrders) authorizeDiscount(ctx context.Context, token, ownerID, storeID string, in ManualInput) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	err = platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, s platform.Scope) error {
		if err := requireCatalog(ctx, tx, s, token); err != nil {
			return err
		}
		var name, amount any
		if in.Discount != nil {
			name, amount = in.Discount.Name, in.Discount.AmountMinor
		}
		bearerHash := sha256.Sum256([]byte(token))
		if _, err := tx.Exec(ctx, `SELECT fulfillment.authorize_manual_order_discount($1,$2::uuid,$3::uuid,$4,$5::text,$6::bigint)`, bearerHash[:], s.StoreID, ownerID, hash[:], name, amount); err != nil {
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
	return discountBoundaryError(ctx, err)
}

func (m *ManualOrders) bindDiscount(ctx context.Context, token, storeID, quoteID string) error {
	err := platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, s platform.Scope) error {
		if err := requireCatalog(ctx, tx, s, token); err != nil {
			return err
		}
		// Quote ownership is checked by the FK plus this scoped definer; merchant runtime
		// cannot directly read buyer quote snapshots or mutate any quoted money.
		hash := sha256.Sum256([]byte(token))
		_, err := tx.Exec(ctx, `SELECT fulfillment.bind_manual_order_discount($1,$2::uuid,$3::uuid)`, hash[:], storeID, quoteID)
		return err
	})
	return discountBoundaryError(ctx, err)
}

// discountBoundaryError runs after WithScope's opaque store-visibility recheck. Only
// closed SQL refusals are normalized; uncertain and already typed errors keep their meaning.
func discountBoundaryError(ctx context.Context, err error) error {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.ErrForbidden
		case "PT404":
			return platform.ErrScopeNotFound
		case "PT409":
			return &Error{Status: 409, Code: "conflict"}
		}
	}
	return err
}
