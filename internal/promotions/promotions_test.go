package promotions

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

func TestNormalize(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"save10": "SAVE10", "  Save-10 ": "SAVE-10", "ABC": "ABC", "a-b": "A-B"} {
		if got, ok := Normalize(in); !ok || got != want {
			t.Errorf("Normalize(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "ab", "has space", strings.Repeat("A", 25), "emoji\U0001F600", "semi;colon", "ＡＢＣ", "a_b"} {
		if _, ok := Normalize(bad); ok {
			t.Errorf("Normalize(%q) accepted", bad)
		}
	}
}

// The per-buyer identity: the store salts it (same e-mail, two stores, two hashes), an unknown value is nil (never matches), phone formatting
// does not make a second buyer.
func TestIdentityHashAndPhoneDigits(t *testing.T) {
	t.Parallel()
	a, b := identityHash("store-a", "email", "x@example.com"), identityHash("store-b", "email", "x@example.com")
	if len(a) != 32 || string(a) == string(b) {
		t.Fatal("store must salt the hash")
	}
	if identityHash("store-a", "email", "") != nil {
		t.Fatal("unknown identity must be nil")
	}
	if string(identityHash("s", "email", "x")) == string(identityHash("s", "phone", "x")) {
		t.Fatal("kind must separate e-mail from phone")
	}
	if phoneDigits("0912-345 678") != "0912345678" || phoneDigits("+886 912 345 678") != "886912345678" || phoneDigits("abc") != "" {
		t.Fatal("phone digits")
	}
}

func TestMapErrorCodes(t *testing.T) {
	t.Parallel()
	pg := func(code, msg string) error { return &pgconn.PgError{Code: code, Message: msg} }
	for _, c := range []struct {
		err    error
		status int
		code   string
	}{
		{pg("PT422", "promo_used_up"), http.StatusUnprocessableEntity, "promo_used_up"},
		{pg("PT422", "invalid_promotion"), http.StatusUnprocessableEntity, "invalid_promotion"},
		{pg("PT409", "promo_exists"), http.StatusConflict, "promo_exists"},
		{pg("PT409", "version_changed"), http.StatusConflict, "version_changed"},
	} {
		var coded *Coded
		if err := mapError(c.err); !errors.As(err, &coded) || coded.Status != c.status || coded.Code != c.code {
			t.Errorf("%v -> %v", c.err, err)
		}
	}
	// A PT422 whose message is not a known code must never become a client-visible code.
	if err := mapError(pg("PT422", "Some Driver Text")); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("unknown PT422 text leaked: %v", err)
	}
	if err := mapError(pg("PT400", "x")); !errors.Is(err, command.ErrInvalid) {
		t.Error("PT400")
	}
	other := errors.New("boom")
	if mapError(other) != other || mapError(nil) != nil {
		t.Error("non-pg errors pass through")
	}
}

func TestAdminErrorMapsAuthority(t *testing.T) {
	t.Parallel()
	pg := func(code string) error { return &pgconn.PgError{Code: code, Message: "x"} }
	for code, want := range map[string]error{"PT401": platform.ErrUnauthorized, "PT403": platform.ErrForbidden, "PT404": platform.ErrScopeNotFound} {
		if err := adminError(pg(code)); !errors.Is(err, want) {
			t.Errorf("%s -> %v", code, err)
		}
	}
	if err := adminError(errors.New("driver text with a secret")); !errors.Is(err, ErrUnavailable) {
		t.Errorf("unknown error must become unavailable, got %v", err)
	}
}

func TestFieldsFitColumns(t *testing.T) {
	t.Parallel()
	n := func(v int64) *int64 { return &v }
	if !(Fields{Percent: n(10)}).fitsColumns() || (Fields{Percent: n(1 << 40)}).fitsColumns() || (Fields{TotalLimit: n(-1)}).fitsColumns() ||
		(Fields{MinSubtotalMinor: -1}).fitsColumns() || (Fields{FixedMinor: n(-5)}).fitsColumns() {
		t.Fatal("range guard")
	}
}
