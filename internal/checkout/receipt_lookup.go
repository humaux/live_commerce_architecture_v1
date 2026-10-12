// Purpose: authenticated durable Begin facts for merchant recovery, including crashes before merchant receipts.
// Depends on: checkout runtime pool, buyer.WithScope, existing checkout.command_results/orders RLS and Begin advisory lock.
// Used by: merchanttools request binding and authoritative recovery. No merchant table grants are required.
// Invariants: I01/I03/I06; absence is final only while the caller holds the Begin lock through its durable closure commit.
package checkout

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
)

// LookupBegin reads original checkout facts under their lock; it never establishes final absence by itself.
func (s *Service) LookupBegin(ctx context.Context, token, storeID, key string) (Result, bool, error) {
	return s.lookupBegin(ctx, token, storeID, key, nil)
}

// LookupBeginLocked is only for a trusted caller holding the exact checkout.begin tenant/store/owner/key advisory lock
// on its own transaction until its closure receipt COMMIT. expected is the server-registered private capability scope.
// The checkout runtime cannot read/write merchant receipts; the same PG lock serializes checkout facts and the closure.
func (s *Service) LookupBeginLocked(ctx context.Context, token, storeID, key string, expected buyer.Scope) (Result, bool, error) {
	return s.lookupBegin(ctx, token, storeID, key, &expected)
}

func (s *Service) lookupBegin(ctx context.Context, token, storeID, key string, held *buyer.Scope) (Result, bool, error) {
	if s == nil || !checkoutKey.MatchString(key) {
		return Result{}, false, command.ErrInvalid
	}
	var out Result
	var found bool
	err := buyer.WithScope(ctx, s.pool, token, storeID, func(c context.Context, tx pgx.Tx, scope buyer.Scope) error {
		if held == nil {
			if err := advisory(c, tx, "checkout.begin|"+scope.TenantID+"|"+scope.StoreID+"|"+scope.OwnerID+"|"+key); err != nil {
				return err
			}
		} else if *held != scope {
			return buyer.ErrUnauthorized
		}
		var hash []byte
		err := tx.QueryRow(c, `SELECT request_hash FROM checkout.command_results WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND operation='checkout.begin' AND idempotency_key=$4`, scope.TenantID, scope.StoreID, scope.OwnerID, key).Scan(&hash)
		if errors.Is(err, pgx.ErrNoRows) {
			// Order+Begin receipt are atomic. Any actual order under this private owner prevents an absence seal.
			var exists bool
			if err = tx.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3)`, scope.TenantID, scope.StoreID, scope.OwnerID).Scan(&exists); err != nil {
				return err
			}
			if exists {
				return errCheckoutDatabase
			}
			return nil
		}
		if err != nil {
			return err
		}
		if len(hash) != 32 {
			return command.ErrConflict
		}
		var digest [32]byte
		copy(digest[:], hash)
		out, found, err = readReceipt(c, tx, scope, key, digest)
		if err != nil {
			return err
		}
		var exists bool
		if err = tx.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3 AND id=$4)`, scope.TenantID, scope.StoreID, scope.OwnerID, out.OrderID).Scan(&exists); err != nil {
			return err
		}
		if !found || !exists {
			return command.ErrConflict
		}
		return nil
	})
	if err != nil {
		return Result{}, false, safeError(ctx, err)
	}
	return out, found, nil
}
