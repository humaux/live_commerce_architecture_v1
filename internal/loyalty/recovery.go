// Purpose: original manual-order award recovery and its scoped replay receipt.
// Depends on: loyalty.manual_order_award_recovery and command.Run.
// Used by: the merchant order award recovery endpoint.
package loyalty

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

type AwardRecovery struct {
	OrderID  string `json:"order_id"`
	State    string `json:"state"`
	Reason   string `json:"reason,omitempty"`
	MemberID string `json:"member_id,omitempty"`
	Points   int64  `json:"points,omitempty"`
	EntryID  string `json:"entry_id,omitempty"`
}

const recoverAwardSQL = `SELECT loyalty.manual_order_award_recovery($1,$2::uuid,$3::uuid,$4)`

func ReadAwardRecovery(ctx context.Context, tx pgx.Tx, s platform.Scope, token, order string) (AwardRecovery, error) {
	var out AwardRecovery
	if !command.ValidID(order) {
		return out, command.ErrInvalid
	}
	err := call(ctx, tx, s, token, "customers:read", recoverAwardSQL, &out, order, false)
	return out, err
}

func RecoverAward(ctx context.Context, tx pgx.Tx, s platform.Scope, token, key, order string) (AwardRecovery, bool, error) {
	var out AwardRecovery
	if tx == nil || !command.ValidID(order) {
		return out, false, command.ErrInvalid
	}
	authorize := func() error {
		if err := platform.RequirePermission(ctx, tx, s, token, "orders:read"); err != nil {
			return dbError(err)
		}
		return dbError(platform.RequirePermission(ctx, tx, s, token, "customers:points"))
	}
	if err := authorize(); err != nil {
		return out, false, err
	}
	var created bool
	err := command.Run(ctx, tx, s, "loyalty.order_award.recover", key, map[string]string{"order_id": order}, &out, func() error {
		if err := call(ctx, tx, s, token, "customers:points", recoverAwardSQL, &out, order, true); err != nil {
			return err
		}
		if out.State != StateAwarded && out.State != "already_awarded" {
			return ErrUnavailable
		}
		created = out.State == StateAwarded
		out.State = StateAwarded
		return nil
	})
	if errors.Is(err, command.ErrConflict) {
		return AwardRecovery{}, false, ErrIdempotencyConflict
	}
	if err != nil {
		return AwardRecovery{}, false, dbError(err)
	}
	if err := authorize(); err != nil {
		return AwardRecovery{}, false, err
	}
	if out.OrderID != order || out.State != StateAwarded || !command.ValidID(out.EntryID) || !ValidAward(Award{MemberID: out.MemberID, Points: out.Points}) {
		return AwardRecovery{}, false, ErrUnavailable
	}
	return out, created, nil
}
