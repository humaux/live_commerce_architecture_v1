// Purpose: the shared typed 403 errors of recent re-authentication (contracts/merchant-password-auth-v1.md §16, OC-15):
// ErrReauthRequired and ErrOwnerRoleDisabled wrap ErrForbidden, so every existing errors.Is(err, ErrForbidden) check still
// holds, while the HTTP layer tests them first and answers 403 with the exact code instead of the generic "forbidden".
// PT403 turns a definer's PT403 message into one of them.
// Depends on: platform.go (ErrForbidden); the PT403 messages raised by identity.require_recent_auth (0201/0203) and the
// owner-role refusals of identity.staff_invite / staff_set_role / staff_accept (0203).
// Used by: the PT403 mappers of internal/identity/staff.go, merchantorders/orders.go, reporting/finance.go,
// customers/read.go, notify/settings.go, payments/settlement/service.go; internal/httpapi/handler.go classify;
// internal/identityhttp/staff.go staffFailure; tests/identityhttp.
// Invariants: only the two fixed messages are recognised; any other PT403 message keeps the caller's fallback, so no
// driver text ever becomes a transport code.
package platform

import "fmt"

var (
	// ErrReauthRequired: the session is valid but was not authenticated recently enough (HTTP 403 reauth_required; never
	// 401, so the BFF and UI keep the session and open the re-auth dialog).
	ErrReauthRequired = fmt.Errorf("reauth_required: %w", ErrForbidden)
	// ErrOwnerRoleDisabled: the owner role cannot be granted or accepted (HTTP 403 owner_role_disabled).
	ErrOwnerRoleDisabled = fmt.Errorf("owner_role_disabled: %w", ErrForbidden)
)

// PT403 maps the MESSAGE of a definer's PT403 to its shared typed error, or returns fallback for any other message.
func PT403(message string, fallback error) error {
	switch message {
	case "reauth_required":
		return ErrReauthRequired
	case "owner_role_disabled":
		return ErrOwnerRoleDisabled
	}
	return fallback
}
