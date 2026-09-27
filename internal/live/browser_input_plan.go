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
	if _, err := browserRuntimeProfileMatches(ctx, tx, scope, token, in, runtime); err != nil {
		return MediaStartResult{}, err
	}
	out, err := p.PlanInputStart(ctx, tx, scope, token, key, in)
	if err != nil {
		return MediaStartResult{}, err
	}
	if _, err := browserRuntimeProfileMatches(ctx, tx, scope, token, in, runtime); err != nil {
		return MediaStartResult{}, err
	}
	return out, nil
}

// ReserveBrowserInput keeps the nonsecret reservation inside the caller's
// scoped transaction. The caller may sign only after that transaction commits.
func (p *MediaPlanner) ReserveBrowserInput(ctx context.Context, tx pgx.Tx, scope platform.Scope,
	token, key string, in MediaInputReserveInput, runtime *BrowserInputRuntime,
) (MediaInputGrant, error) {
	if ctx == nil || tx == nil || p == nil || runtime == nil ||
		!command.ValidID(in.SessionID) || !command.ValidID(in.AttemptID) ||
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
	var authorizationID string
	err := tx.QueryRow(ctx, `SELECT authorization_id FROM live.media_attempts
		WHERE tenant_id=$1::uuid AND store_id=$2::uuid AND session_id=$3::uuid
		AND id=$4::uuid AND original_principal_id=$5::uuid`,
		scope.TenantID, scope.StoreID, in.SessionID, in.AttemptID, scope.PrincipalID).Scan(&authorizationID)
	if err != nil {
		return MediaInputGrant{}, mediaPlanError(err)
	}
	profileInput := MediaStartInput{SessionID: in.SessionID, AuthorizationID: authorizationID,
		ExpectedSessionVersion: in.ExpectedSessionVersion}
	before, err := browserRuntimeProfileMatches(ctx, tx, scope, token, profileInput, runtime)
	if err != nil {
		return MediaInputGrant{}, err
	}
	grant, err := p.ReserveInput(ctx, tx, scope, token, key, in)
	if err != nil {
		return MediaInputGrant{}, err
	}
	after, err := browserRuntimeProfileMatches(ctx, tx, scope, token, profileInput, runtime)
	if err != nil {
		return MediaInputGrant{}, err
	}
	if before != after || grant.AttemptID != in.AttemptID ||
		grant.ProjectID != after.ProjectID || grant.EndpointIdentity != after.EndpointIdentity ||
		grant.CredentialVersion != after.CredentialVersion {
		return MediaInputGrant{}, command.ErrConflict
	}
	return grant, nil
}

func browserRuntimeProfileMatches(ctx context.Context, tx pgx.Tx, scope platform.Scope,
	token string, in MediaStartInput, runtime *BrowserInputRuntime) (browserRuntimeProfile, error) {
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT live.load_media_input_runtime_profile(
		$1::bytea,$2::uuid,$3::uuid,$4::uuid,$5::bigint)`,
		hash[:], scope.StoreID, in.SessionID, in.AuthorizationID, in.ExpectedSessionVersion).Scan(&raw)
	if err != nil {
		return browserRuntimeProfile{}, mediaPlanError(err)
	}
	var fields map[string]json.RawMessage
	var profile browserRuntimeProfile
	if len(raw) > 4096 || json.Unmarshal(raw, &fields) != nil || len(fields) != 4 ||
		json.Unmarshal(raw, &profile) != nil || profile.RuntimeVersion != 1 ||
		!mediaProjectPattern.MatchString(profile.ProjectID) || profile.CredentialVersion < 1 ||
		profile.EndpointIdentity == "" {
		return browserRuntimeProfile{}, command.ErrConflict
	}
	endpoint, ok := runtime.projects[mediaProjectKey{profile.ProjectID, profile.CredentialVersion}]
	if !ok || endpoint.identity != profile.EndpointIdentity {
		return browserRuntimeProfile{}, command.ErrConflict
	}
	return profile, nil
}
