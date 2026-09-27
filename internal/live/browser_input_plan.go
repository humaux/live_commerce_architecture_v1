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

type browserRuntimeProfile struct {
	ProjectID         string `json:"project_id"`
	EndpointIdentity  string `json:"endpoint_identity"`
	CredentialVersion int64  `json:"credential_version"`
	RuntimeVersion    int64  `json:"runtime_version"`
}

// PlanBrowserInputStart checks the deployment-owned marker and exact local
// binding in the caller's transaction. Replay is checked again because
// command.Run skips the PlanInputStart callback for a cached receipt.
func (p *MediaPlanner) PlanBrowserInputStart(ctx context.Context, tx pgx.Tx, scope platform.Scope,
	token, key string, in MediaStartInput, runtime *BrowserInputRuntime,
) (MediaStartResult, error) {
	if ctx == nil || tx == nil || p == nil || p.jobs == nil || runtime == nil ||
		!command.ValidID(in.SessionID) || !command.ValidID(in.AuthorizationID) ||
		in.SessionID == "00000000-0000-0000-0000-000000000000" ||
		in.AuthorizationID == "00000000-0000-0000-0000-000000000000" ||
		in.ExpectedSessionVersion < 1 || !mediaKeyPattern.MatchString(key) {
		return MediaStartResult{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return MediaStartResult{}, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.authz_revision',$1,true)`, strconv.FormatInt(scope.Revision, 10)); err != nil {
		return MediaStartResult{}, err
	}
	if err := browserRuntimeProfileMatches(ctx, tx, scope, token, in, runtime); err != nil {
		return MediaStartResult{}, err
	}
	out, err := p.PlanInputStart(ctx, tx, scope, token, key, in)
	if err != nil {
		return MediaStartResult{}, err
	}
	if err := browserRuntimeProfileMatches(ctx, tx, scope, token, in, runtime); err != nil {
		return MediaStartResult{}, err
	}
	return out, nil
}

func browserRuntimeProfileMatches(ctx context.Context, tx pgx.Tx, scope platform.Scope,
	token string, in MediaStartInput, runtime *BrowserInputRuntime) error {
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT live.load_media_input_runtime_profile(
		$1::bytea,$2::uuid,$3::uuid,$4::uuid,$5::bigint)`,
		hash[:], scope.StoreID, in.SessionID, in.AuthorizationID, in.ExpectedSessionVersion).Scan(&raw)
	if err != nil {
		return mediaPlanError(err)
	}
	var fields map[string]json.RawMessage
	var profile browserRuntimeProfile
	if len(raw) > 4096 || json.Unmarshal(raw, &fields) != nil || len(fields) != 4 ||
		json.Unmarshal(raw, &profile) != nil || profile.RuntimeVersion != 1 ||
		!mediaProjectPattern.MatchString(profile.ProjectID) || profile.CredentialVersion < 1 ||
		profile.EndpointIdentity == "" {
		return command.ErrConflict
	}
	endpoint, ok := runtime.projects[mediaProjectKey{profile.ProjectID, profile.CredentialVersion}]
	if !ok || endpoint.identity != profile.EndpointIdentity {
		return command.ErrConflict
	}
	return nil
}
