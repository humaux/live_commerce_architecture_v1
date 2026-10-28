// Purpose: unit gate of reauth_errors.go: the two fixed PT403 messages become the shared typed errors (which still match
// ErrForbidden), and every other message keeps the caller's fallback.
// Depends on: reauth_errors.go, platform.go (ErrForbidden).
// Used by: go test ./internal/platform (CI shard:unit).
package platform

import (
	"errors"
	"testing"
)

func TestPT403TypedErrors(t *testing.T) {
	fallback := errors.New("fallback")
	for msg, want := range map[string]error{
		"reauth_required": ErrReauthRequired, "owner_role_disabled": ErrOwnerRoleDisabled,
		"forbidden": fallback, "": fallback, "REAUTH_REQUIRED": fallback, "reauth_required ": fallback,
	} {
		if got := PT403(msg, fallback); got != want {
			t.Errorf("PT403(%q) = %v, want %v", msg, got, want)
		}
	}
	for _, err := range []error{ErrReauthRequired, ErrOwnerRoleDisabled} {
		if !errors.Is(err, ErrForbidden) || errors.Is(err, ErrUnauthorized) {
			t.Errorf("%v must wrap ErrForbidden and never ErrUnauthorized", err)
		}
	}
	if errors.Is(ErrReauthRequired, ErrOwnerRoleDisabled) || errors.Is(ErrOwnerRoleDisabled, ErrReauthRequired) {
		t.Error("the two typed errors must stay distinct")
	}
}
