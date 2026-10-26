// Purpose: OIDC recent re-authentication of a live merchant session (contracts/merchant-password-auth-v1.md §16,
// OIDC-only principals): a session-authenticated start that opens a prompt=login + max_age flow bound to the calling
// session hash, and the shared callback completion (CompleteLogin) that, for a re-auth flow, checks auth_time and the
// principal match in SQL and never issues a session.
// Depends on: internal/oidclogin (ReauthAuthorizationURL, Identity.AuthTime) through the Provider seam; SQL definers
// identity.start_oidc_reauth_flow, identity.consume_login_flow_with_session, identity.complete_oidc_reauth (0201) and
// identity.issue_merchant_session (0004/0201) on the commerce_identity pool.
// Used by: internal/identityhttp (OC-15: the re-auth start route and POST /v1/identity/login/complete).
// Invariants: the session comes only from the bearer token (never a body field); a re-auth flow never inserts a
// session row or a session.issued event, on success or refusal; a refusal is ErrReauthFailed (403), never 401.
package identity

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ReauthProvider is the optional Provider capability for OIDC re-authentication (*oidclogin.Provider has it).
type ReauthProvider interface {
	ReauthAuthorizationURL(state, nonce, verifier string, maxAge time.Duration) (string, error)
}

// oidcReauthMaxAge is the §16 OIDC window: max_age on the authorize URL and the auth_time bound in
// identity.complete_oidc_reauth (fixed 10 minutes there too).
const oidcReauthMaxAge = 10 * time.Minute

// LoginResult is the callback outcome. A login flow yields Session (with a token); a re-auth flow yields
// Reauth=true and AuthenticatedAt, and no token: the caller must leave the session cookie unchanged.
type LoginResult struct {
	Session         Session
	Reauth          bool
	AuthenticatedAt time.Time
}

// StartOIDCReauth opens a re-auth flow for the session named by sessionToken (the caller's bearer token).
// ErrUnauthorized when the session is not live; ErrDisabled when no OIDC provider (or no re-auth capability) is wired.
func (s *Service) StartOIDCReauth(ctx context.Context, sessionToken string) (Flow, error) {
	rp, ok := s.provider.(ReauthProvider)
	if s.provider == nil || !ok {
		return Flow{}, ErrDisabled
	}
	if !validToken(sessionToken) {
		return Flow{}, ErrUnauthorized
	}
	state, binding, nonce, verifier := randomToken(), randomToken(), randomToken(), randomToken()
	authURL, err := rp.ReauthAuthorizationURL(state, nonce, verifier, oidcReauthMaxAge)
	if err != nil {
		return Flow{}, ErrUnavailable
	}
	flow := Flow{URL: authURL, State: state, Binding: binding}
	err = s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// identity.start_oidc_reauth_flow: PT401 unless the session is a live merchant session; stores its hash in the flow.
		return tx.QueryRow(ctx, `SELECT identity.start_oidc_reauth_flow($1,$2,$3,$4,$5,$6)`,
			digest(state), digest(binding), s.policy.ProviderKey, nonce, verifier, digest(sessionToken)).Scan(&flow.ExpiresAt)
	})
	if errors.Is(translateError(err), ErrUnauthorized) {
		return Flow{}, ErrUnauthorized
	}
	if err != nil {
		return Flow{}, ErrUnavailable
	}
	return flow, nil
}

// CompleteLogin is the callback for both flow kinds (one registered redirect URI). The flow is consumed before the
// external exchange, as in Complete. A re-auth flow calls identity.complete_oidc_reauth, never issue_merchant_session.
func (s *Service) CompleteLogin(ctx context.Context, state, binding, code string) (LoginResult, error) {
	if s.provider == nil {
		return LoginResult{}, ErrDisabled
	}
	if !validToken(state) || !validToken(binding) || len(code) == 0 || len(code) > 4096 {
		return LoginResult{}, ErrUnauthorized
	}
	var nonce, verifier string
	var sessionHash []byte
	err := s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT nonce,verifier,session_hash FROM identity.consume_login_flow_with_session($1,$2,$3)`,
			digest(state), digest(binding), s.policy.ProviderKey).Scan(&nonce, &verifier, &sessionHash)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginResult{}, ErrUnauthorized
	}
	if err != nil {
		return LoginResult{}, ErrUnavailable
	}
	if sessionHash == nil {
		session, err := s.issue(ctx, code, nonce, verifier)
		return LoginResult{Session: session}, err
	}
	subject, ok := s.exchange(ctx, code, nonce, verifier)
	if !ok || subject.AuthTime.IsZero() { // §16: a missing auth_time is refused
		return LoginResult{}, ErrReauthFailed
	}
	var outcome string
	var at *time.Time
	err = s.transaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// identity.complete_oidc_reauth: principal match + 10-minute auth_time window in SQL; updates only that session.
		return tx.QueryRow(ctx, `SELECT outcome,authenticated_at FROM identity.complete_oidc_reauth($1,$2,$3,$4)`,
			sessionHash, subject.Issuer, subject.Subject, subject.AuthTime).Scan(&outcome, &at)
	})
	if err != nil {
		return LoginResult{}, ErrUnavailable
	}
	switch {
	case outcome == "REAUTHENTICATED" && at != nil:
		return LoginResult{Reauth: true, AuthenticatedAt: *at}, nil
	case outcome == "UNAUTHORIZED":
		return LoginResult{}, ErrUnauthorized
	default: // REFUSED: another account, unknown identity or stale auth_time; authenticated_at unchanged
		return LoginResult{}, ErrReauthFailed
	}
}
