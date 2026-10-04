package identityhttp

// Private staff-team routes /v1/identity/staff/{list,invite,revoke-invite,set-role,remove,accept} (contracts/storefront-v2.md §D).
// Called only by the admin BFF (apps/admin/app/api/team/[action]/route.ts), authenticated by the fixed BFF key; the merchant
// bearer rides in Authorization and is verified in SQL by the identity.staff_* definers (migration 0089), which also decide
// the OWNER role. The Go endpoints behind them are identity.Staff.* (internal/identity/staff.go).
//
// Same transport rules as the other identity routes: BFF key first, no Origin/Cookie, no query, strict JSON <= 64 KiB, no-store.
// Every route is POST (the BFF has one private-call helper). No client IP is needed: invitations are bounded per store in
// SQL and accept is a bearer-bound single-use token, not an anonymous endpoint. It never trusts a tenant from the request.

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"time"

	"livecommerce/internal/httperror"
	"livecommerce/internal/identity"
)

// staffService mirrors *identity.Staff; a narrow seam lets transport tests prove rejection before any identity I/O.
type staffService interface {
	List(ctx context.Context, bearer, store string) (identity.StaffTeam, error)
	Invite(ctx context.Context, bearer, store, email, role, locale string) (identity.StaffInvite, error)
	RevokeInvite(ctx context.Context, bearer, store, invite string) error
	SetRole(ctx context.Context, bearer, store, principal, role string) error
	Remove(ctx context.Context, bearer, store, principal string) error
	Accept(ctx context.Context, bearer, token string) (identity.StaffJoined, error)
}

// staffRequestTimeout covers the database work plus one synchronous invitation mail (10 s cap in identity.Staff.Invite).
const staffRequestTimeout = 14 * time.Second

type storeBody struct {
	StoreID string `json:"store_id"`
}
type inviteBody struct {
	StoreID string `json:"store_id"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Locale  string `json:"locale"`
}
type inviteIDBody struct {
	StoreID  string `json:"store_id"`
	InviteID string `json:"invite_id"`
}
type memberBody struct {
	StoreID     string `json:"store_id"`
	PrincipalID string `json:"principal_id"`
}
type roleBody struct {
	StoreID     string `json:"store_id"`
	PrincipalID string `json:"principal_id"`
	Role        string `json:"role"`
}
type acceptBody struct {
	Token string `json:"token"`
}

// route registers one POST: bearer first (no body read without it), then the strict body, then fn.
func route[T any](mux *http.ServeMux, path string, fn func(w http.ResponseWriter, r *http.Request, bearer string, in T) error) {
	mux.HandleFunc("POST /v1/identity/staff/"+path, func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			staffFailure(w, identity.ErrUnauthorized)
			return
		}
		var in T
		if !body(w, r, &in) {
			return
		}
		if err := fn(w, r, token, in); err != nil {
			staffFailure(w, err)
		}
	})
}

// NewStaffHandler serves the staff routes.
func NewStaffHandler(s staffService, bffKey string) (http.Handler, error) {
	if s == nil || !ValidSecret(bffKey) {
		return nil, errors.New("invalid staff transport configuration")
	}
	mux := http.NewServeMux()
	route(mux, "list", func(w http.ResponseWriter, r *http.Request, b string, in storeBody) error {
		team, err := s.List(r.Context(), b, in.StoreID)
		if err == nil {
			respond(w, team)
		}
		return err
	})
	route(mux, "invite", func(w http.ResponseWriter, r *http.Request, b string, in inviteBody) error {
		out, err := s.Invite(r.Context(), b, in.StoreID, in.Email, in.Role, in.Locale)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = jsonEncode(w, out)
		}
		return err
	})
	route(mux, "revoke-invite", func(w http.ResponseWriter, r *http.Request, b string, in inviteIDBody) error {
		return noContent(w, s.RevokeInvite(r.Context(), b, in.StoreID, in.InviteID))
	})
	route(mux, "set-role", func(w http.ResponseWriter, r *http.Request, b string, in roleBody) error {
		return noContent(w, s.SetRole(r.Context(), b, in.StoreID, in.PrincipalID, in.Role))
	})
	route(mux, "remove", func(w http.ResponseWriter, r *http.Request, b string, in memberBody) error {
		return noContent(w, s.Remove(r.Context(), b, in.StoreID, in.PrincipalID))
	})
	route(mux, "accept", func(w http.ResponseWriter, r *http.Request, b string, in acceptBody) error {
		out, err := s.Accept(r.Context(), b, in.Token)
		if err == nil {
			respond(w, out)
		}
		return err
	})
	return httperror.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header.Values("X-Commerce-BFF-Key")) != 1 || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Commerce-BFF-Key")), []byte(bffKey)) != 1 {
			failure(w, identity.ErrUnauthorized)
			return
		}
		if len(r.Header.Values("Origin")) > 0 || len(r.Header.Values("Cookie")) > 0 {
			httperror.Write(w, http.StatusForbidden, "forbidden")
			return
		}
		if r.URL.RawQuery != "" {
			failure(w, identity.ErrInvalid)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), staffRequestTimeout)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})), nil
}

func noContent(w http.ResponseWriter, err error) error {
	if err == nil {
		w.WriteHeader(http.StatusNoContent)
	}
	return err
}

// staffMessages are the user-safe texts of the staff codes. internal/httperror rewrites unknown codes to "internal", so these
// use the same envelope written here (like the password codes) instead of httperror.Write.
var staffMessages = map[string]string{
	"invalid_request": "Request validation failed.", "invalid_email": "Email address is not valid.", "unauthorized": "Sign-in required.",
	"forbidden": "Only a store owner can manage the team.", "not_found": "Resource not found.",
	"invite_invalid":       "This invitation is not valid. Ask the store owner for a new one.",
	"already_member":       "Already a member of this store.",
	"last_owner":           "A store must keep at least one owner.",
	"too_many_invitations": "Too many invitations. Try again later.", "conflict": "Request conflicts with current state.",
	"unavailable": "Temporarily unavailable.",
}

// staffFailure maps service errors to (status, code). ErrInviteInvalid is 404 for every reason (no oracle).
func staffFailure(w http.ResponseWriter, err error) {
	status, code := http.StatusServiceUnavailable, "unavailable"
	switch {
	case errors.Is(err, identity.ErrInvalidEmail):
		status, code = http.StatusUnprocessableEntity, "invalid_email"
	case errors.Is(err, identity.ErrInvalid):
		status, code = http.StatusUnprocessableEntity, "invalid_request"
	case errors.Is(err, identity.ErrUnauthorized):
		status, code = http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, identity.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, identity.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, identity.ErrInviteInvalid):
		status, code = http.StatusNotFound, "invite_invalid"
	case errors.Is(err, identity.ErrAlreadyMember):
		status, code = http.StatusConflict, "already_member"
	case errors.Is(err, identity.ErrLastOwner):
		status, code = http.StatusConflict, "last_owner"
	case errors.Is(err, identity.ErrTooManyInvitations):
		status, code = http.StatusTooManyRequests, "too_many_invitations"
	case errors.Is(err, identity.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = jsonEncode(w, httperror.Envelope{Code: code, Message: staffMessages[code], RequestID: w.Header().Get("X-Request-ID"),
		Retryable: status == http.StatusServiceUnavailable, Details: map[string]any{}})
}
