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

type MediaInputReserveInput struct {
	SessionID              string `json:"session_id"`
	AttemptID              string `json:"attempt_id"`
	ExpectedSessionVersion int64  `json:"expected_session_version"`
}

// MediaInputGrant is nonsecret reservation material, not a browser credential.
type MediaInputGrant struct {
	AttemptID         string `json:"attempt_id"`
	RoomName          string `json:"room_name"`
	PublisherIdentity string `json:"publisher_identity"`
	ProjectID         string `json:"project_id"`
	EndpointIdentity  string `json:"endpoint_identity"`
	CredentialVersion int64  `json:"credential_version"`
	IssuedAt          int64  `json:"issued_at"`
	ExpiresAt         int64  `json:"expires_at"`
}

func (p *MediaPlanner) ReserveInput(ctx context.Context, tx pgx.Tx, scope platform.Scope,
	token, key string, in MediaInputReserveInput) (out MediaInputGrant, err error) {
	if ctx == nil || p == nil || tx == nil || !command.ValidID(in.SessionID) ||
		!command.ValidID(in.AttemptID) ||
		in.SessionID == "00000000-0000-0000-0000-000000000000" ||
		in.AttemptID == "00000000-0000-0000-0000-000000000000" ||
		in.ExpectedSessionVersion < 1 || !mediaKeyPattern.MatchString(key) {
		return MediaInputGrant{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return MediaInputGrant{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.authz_revision',$1,true)`, strconv.FormatInt(scope.Revision, 10)); err != nil {
		return MediaInputGrant{}, err
	}
	request := struct {
		PrincipalID string `json:"principal_id"`
		MediaInputReserveInput
	}{scope.PrincipalID, in}
	err = command.Run(ctx, tx, scope, "live.media.input.reserve", key, request, &out, func() error {
		if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
			return err
		}
		var raw []byte
		hash := sha256.Sum256([]byte(token))
		if err := tx.QueryRow(ctx, `SELECT live.reserve_media_input($1,$2::uuid,$3::uuid,$4::uuid,$5)`,
			hash[:], scope.StoreID, in.SessionID, in.AttemptID, in.ExpectedSessionVersion).Scan(&raw); err != nil {
			return mediaPlanError(err)
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return command.ErrInvalid
		}
		return command.Audit(ctx, tx, scope, "live.media.input.reserved")
	})
	if err != nil {
		return MediaInputGrant{}, mediaPlanError(err)
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return MediaInputGrant{}, err
	}
	var raw []byte
	hash := sha256.Sum256([]byte(token))
	if err := tx.QueryRow(ctx, `SELECT live.reserve_media_input($1,$2::uuid,$3::uuid,$4::uuid,$5)`,
		hash[:], scope.StoreID, in.SessionID, in.AttemptID, in.ExpectedSessionVersion).Scan(&raw); err != nil {
		return MediaInputGrant{}, mediaPlanError(err)
	}
	var current MediaInputGrant
	if err := json.Unmarshal(raw, &current); err != nil || current != out ||
		!command.ValidID(current.AttemptID) || current.ExpiresAt <= current.IssuedAt {
		return MediaInputGrant{}, command.ErrConflict
	}
	return current, nil
}
