// Purpose: the private recent re-authentication routes (contracts/merchant-password-auth-v1.md §16, OC-15): password
// POST /v1/identity/password/reauth/{start,complete} (mounted in NewPasswordHandler) and OIDC POST
// /v1/identity/login/reauth/start plus the re-auth branch of POST /v1/identity/login/complete (mounted in NewHandler).
// Depends on: internal/identity (Passwords.StartReauth/CompleteReauth, Service.StartOIDCReauth/CompleteLogin, their error
// values); password.go (prepare, writePasswordError, passwordFailure, sixDigits); handler.go (bearer, body, respond, failure).
// Used by: the admin BFF (apps/admin/app/api/auth/password/reauth/*, apps/admin/app/api/auth/callback/route.ts) through
// cmd/api identityMux; tests/identityhttp (RH01-RH03, RH10, RH11).
// Invariants: the session comes only from the bearer (no email or principal in a body; no bearer = 401 before any work);
// a refused re-auth is 403 reauth_failed, never 401, so the BFF keeps the session; a re-auth callback never returns a
// session token; errors go through the password writer (passwordMessages).
package identityhttp

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"time"

	"livecommerce/internal/identity"
)

// passwordReauthService is the re-auth half of *identity.Passwords.
type passwordReauthService interface {
	StartReauth(ctx context.Context, ip netip.Addr, sessionToken, password, locale string) (time.Time, error)
	CompleteReauth(ctx context.Context, ip netip.Addr, sessionToken, code string) (time.Time, error)
}

// oidcReauthService is the re-auth half of *identity.Service.
type oidcReauthService interface {
	StartOIDCReauth(ctx context.Context, sessionToken string) (identity.Flow, error)
	CompleteLogin(ctx context.Context, state, binding, code string) (identity.LoginResult, error)
}

// reauthFailure maps the re-auth outcomes onto the password writer: refused 403 reauth_failed, wrong code 422 invalid_code
// (not the login 401), then the password table (throttled 429, busy / mail_unavailable 503) and the shared mapping.
func reauthFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, identity.ErrReauthFailed):
		writePasswordError(w, http.StatusForbidden, "reauth_failed", false, "")
	case errors.Is(err, identity.ErrInvalidCode):
		writePasswordError(w, http.StatusUnprocessableEntity, "invalid_code", false, "")
	default:
		passwordFailure(w, err)
	}
}

// registerPasswordReauth mounts the two password re-auth steps when the service has them (*identity.Passwords does).
func registerPasswordReauth(mux *http.ServeMux, p passwordService) {
	rs, ok := p.(passwordReauthService)
	if !ok {
		return
	}
	mux.HandleFunc("POST /v1/identity/password/reauth/start", func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			failure(w, identity.ErrUnauthorized)
			return
		}
		var in struct {
			Password string `json:"password"`
			Locale   string `json:"locale"`
		}
		ip, ok := prepare(w, r, &in)
		if !ok {
			return
		}
		expires, err := rs.StartReauth(r.Context(), ip, token, in.Password, in.Locale)
		if err != nil {
			reauthFailure(w, err)
			return
		}
		respond(w, map[string]any{"expires_at": expires.UTC()})
	})
	mux.HandleFunc("POST /v1/identity/password/reauth/complete", func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			failure(w, identity.ErrUnauthorized)
			return
		}
		var in struct {
			Code string `json:"code"`
		}
		ip, ok := prepare(w, r, &in)
		if !ok {
			return
		}
		if !sixDigits(in.Code) { // cannot be right: refused before any work
			reauthFailure(w, identity.ErrInvalidCode)
			return
		}
		at, err := rs.CompleteReauth(r.Context(), ip, token, in.Code)
		if err != nil {
			reauthFailure(w, err)
			return
		}
		respond(w, map[string]any{"authenticated_at": at.UTC()})
	})
}

// registerOIDCReauth mounts the session-authenticated OIDC re-auth start when the service has it (*identity.Service does).
func registerOIDCReauth(mux *http.ServeMux, rs oidcReauthService) {
	if rs == nil {
		return
	}
	mux.HandleFunc("POST /v1/identity/login/reauth/start", func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			failure(w, identity.ErrUnauthorized)
			return
		}
		var in struct{}
		if !body(w, r, &in) {
			return
		}
		flow, err := rs.StartOIDCReauth(r.Context(), token)
		if err != nil {
			reauthFailure(w, err)
			return
		}
		respond(w, map[string]any{"authorization_url": flow.URL, "binding": flow.Binding, "expires_at": flow.ExpiresAt})
	})
}

// completeLogin answers the shared callback through CompleteLogin: a login flow returns its token, a re-auth flow returns
// {reauth:true, authenticated_at} and no token (the BFF leaves the session cookie unchanged).
func completeLogin(w http.ResponseWriter, r *http.Request, rs oidcReauthService, state, binding, code string) {
	result, err := rs.CompleteLogin(r.Context(), state, binding, code)
	if err != nil {
		reauthFailure(w, err)
		return
	}
	if result.Reauth {
		respond(w, map[string]any{"reauth": true, "authenticated_at": result.AuthenticatedAt.UTC()})
		return
	}
	respond(w, map[string]any{"token": result.Session.Token, "expires_at": result.Session.ExpiresAt})
}
