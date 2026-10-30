// Purpose: merchant replay visibility with authenticated idempotency and an independent CAS version.
// Depends on: live.sessions, live.replay_permalink (0213), authorize, command.Run/Audit.
// Used by: internal/httpapi live replay GET/PUT; no provider call or video backfill.
// Invariants: server scope only; ENDED plus stored official video before publication; revocation fences receipts.
package live

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

var ErrNoOfficialVideo = errors.New("no official video")

type ReplayInput struct {
	Public          bool  `json:"public"`
	ExpectedVersion int64 `json:"expected_version"`
}

type ReplayState struct {
	SessionID string `json:"session_id"`
	Public    bool   `json:"public"`
	Version   int64  `json:"version"`
}

func GetReplay(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, id string) (ReplayState, error) {
	if !command.ValidID(id) {
		return ReplayState{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return ReplayState{}, err
	}
	out := ReplayState{SessionID: id}
	err := tx.QueryRow(ctx, `SELECT replay_public,replay_version FROM live.sessions WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, id).Scan(&out.Public, &out.Version)
	if err != nil {
		return ReplayState{}, mapError(err)
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return ReplayState{}, err
	}
	return out, nil
}

func SetReplay(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, id string, in ReplayInput) (ReplayState, error) {
	if !command.ValidID(id) || in.ExpectedVersion < 1 || in.ExpectedVersion >= 9007199254740991 {
		return ReplayState{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return ReplayState{}, err
	}
	request := struct {
		PrincipalID string `json:"principal_id"`
		SessionID   string `json:"session_id"`
		ReplayInput
	}{scope.PrincipalID, id, in}
	out := ReplayState{SessionID: id}
	err := command.Run(ctx, tx, scope, "live.replay.set", key, request, &out, func() error {
		var lifecycle string
		var version int64
		err := tx.QueryRow(ctx, `SELECT lifecycle,replay_version FROM live.sessions WHERE tenant_id=$1 AND store_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.StoreID, id).Scan(&lifecycle, &version)
		if err != nil {
			return mapError(err)
		}
		if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
			return err
		}
		if version != in.ExpectedVersion {
			return command.ErrConflict
		}
		if lifecycle != "ended" {
			return claims.ErrInvalidTransition
		}
		if in.Public {
			var permalink string
			if err := tx.QueryRow(ctx, `SELECT live.replay_permalink($1::uuid)`, id).Scan(&permalink); err != nil {
				return err
			}
			if permalink == "" {
				return ErrNoOfficialVideo
			}
		}
		err = tx.QueryRow(ctx, `UPDATE live.sessions SET replay_public=$4,replay_version=replay_version+1 WHERE tenant_id=$1 AND store_id=$2 AND id=$3 RETURNING replay_public,replay_version`, scope.TenantID, scope.StoreID, id, in.Public).Scan(&out.Public, &out.Version)
		if err != nil {
			return mapError(err)
		}
		return command.Audit(ctx, tx, scope, "live.replay.updated")
	})
	if err != nil {
		return ReplayState{}, mapError(err)
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return ReplayState{}, err
	}
	return out, nil
}
