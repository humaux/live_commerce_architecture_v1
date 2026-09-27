package live

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"strconv"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

type MediaStopInput struct {
	SessionID string `json:"session_id"`
	AttemptID string `json:"attempt_id"`
}

type MediaStopResult struct {
	SessionID   string `json:"session_id"`
	AttemptID   string `json:"attempt_id"`
	OperationID string `json:"operation_id"`
	State       string `json:"state"`
}

// RequestStop records one merchant cleanup intent; it never enqueues or calls LiveKit.
func (p *MediaPlanner) RequestStop(ctx context.Context, tx pgx.Tx, scope platform.Scope,
	token, key string, in MediaStopInput) (out MediaStopResult, err error) {
	if ctx == nil || p == nil || p.jobs == nil || tx == nil ||
		!command.ValidID(in.SessionID) || !command.ValidID(in.AttemptID) ||
		in.SessionID == "00000000-0000-0000-0000-000000000000" ||
		in.AttemptID == "00000000-0000-0000-0000-000000000000" ||
		!mediaKeyPattern.MatchString(key) {
		return MediaStopResult{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return MediaStopResult{}, err
	}
	request := struct {
		PrincipalID string `json:"principal_id"`
		MediaStopInput
	}{scope.PrincipalID, in}
	err = command.Run(ctx, tx, scope, "live.media.stop", key, request, &out, func() error {
		if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
			return err
		}
		hash := sha256.Sum256([]byte(token))
		if _, err := tx.Exec(ctx, `SELECT set_config('app.authz_revision',$1,true)`,
			strconv.FormatInt(scope.Revision, 10)); err != nil {
			return err
		}
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT live.request_media_stop($1::bytea,$2::uuid,$3::uuid,$4::uuid)`,
			hash[:], scope.StoreID, in.SessionID, in.AttemptID).Scan(&raw); err != nil {
			return mediaPlanError(err)
		}
		if err := json.Unmarshal(raw, &out); err != nil ||
			out.SessionID != in.SessionID || out.AttemptID != in.AttemptID ||
			!command.ValidID(out.OperationID) {
			return command.ErrInvalid
		}
		switch out.State {
		case "requested", "cancelled_before_start", "terminal", "escalated":
		default:
			return command.ErrInvalid
		}
		return command.Audit(ctx, tx, scope, "live.media.stop.requested")
	})
	if err != nil {
		return MediaStopResult{}, mediaPlanError(err)
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return MediaStopResult{}, err
	}
	return out, nil
}
