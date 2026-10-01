package metaconnect

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/metareply"
)

func granted(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

func TestEntryOfListsWhatToRegrant(t *testing.T) {
	all := append(append([]string{}, fbPermissions...), igPermissions...)
	full := accountDoc{ID: "11", Name: "Shop", Tasks: []string{"MESSAGING", "MODERATE", "MANAGE"}}
	full.IG.ID, full.IG.Username = "22", "shop_ig"
	e, ok := entryOf(full, granted(all...))
	if !ok || len(e.Missing) != 0 || len(e.IGMissing) != 0 || e.IGID != "22" || e.IGUsername != "shop_ig" {
		t.Fatalf("fully granted: %+v", e)
	}
	// Missing a Facebook permission and the MODERATE task: both listed; Instagram permission missing is reported separately.
	partial := accountDoc{ID: "11", Name: "Shop", Tasks: []string{"MESSAGING"}}
	partial.IG.ID = "22"
	e, _ = entryOf(partial, granted("pages_show_list", "pages_manage_metadata", "pages_read_engagement", "instagram_basic"))
	if strings.Join(e.Missing, ",") != "pages_messaging,task_moderate" || strings.Join(e.IGMissing, ",") != "instagram_manage_comments,instagram_manage_messages" {
		t.Fatalf("partial: %+v", e)
	}
	// No Instagram account: IG permissions are not required and ig_missing is an empty array (the SQL wants an array).
	plain := accountDoc{ID: "33", Name: "Plain", Tasks: []string{"messaging", "moderate"}} // task names are case-insensitive
	e, _ = entryOf(plain, granted(fbPermissions...))
	if len(e.Missing) != 0 || e.IGID != "" || e.IGMissing == nil || e.Missing == nil {
		t.Fatalf("plain: %+v", e)
	}
	if _, ok := entryOf(accountDoc{ID: "not-numeric"}, nil); ok {
		t.Fatal("a non-numeric id must be skipped")
	}
}

func TestShortNameStripsControlAndBounds(t *testing.T) {
	if got := shortName("a\x00b\nc"); got != "abc" {
		t.Fatalf("control bytes kept: %q", got)
	}
	if got := shortName(strings.Repeat("頁", 200)); len([]rune(got)) != nameRunes {
		t.Fatalf("not bounded: %d", len([]rune(got)))
	}
	if shortName(string([]byte{0xff, 0xfe})) != "" {
		t.Fatal("invalid UTF-8 must be dropped")
	}
}

func TestMapErrorOnlyEchoesFrozenCodes(t *testing.T) {
	cases := []struct {
		state, msg string
		status     int
		code       string
	}{
		{"MC409", "page_taken", http.StatusConflict, "page_taken"}, {"MC410", "state_expired", http.StatusGone, "state_expired"},
		{"MC422", "missing_permission", http.StatusUnprocessableEntity, "missing_permission"},
		{"MC404", "not_found", http.StatusNotFound, "not_found"}, {"MC409", "secret detail from a driver", http.StatusConflict, "conflict"},
	}
	for _, c := range cases {
		var r *Refusal
		if err := mapError(&pgconn.PgError{Code: c.state, Message: c.msg}); !errors.As(err, &r) || r.Status != c.status || r.Code != c.code {
			t.Errorf("%s %q -> %v", c.state, c.msg, err)
		}
	}
	if err := mapError(&pgconn.PgError{Code: "PT409"}); !errors.Is(err, command.ErrConflict) {
		t.Errorf("PT409 -> %v", err)
	}
	if err := mapError(pgx.ErrNoRows); !errors.Is(err, command.ErrNotFound) {
		t.Errorf("no rows -> %v", err)
	}
	if err := mapError(&pgconn.PgError{Code: "XX000", Message: "boom"}); err == nil || errors.As(err, new(*Refusal)) {
		t.Errorf("unknown driver error must stay unclassified: %v", err)
	}
}

// The pending USER token is sealed with the Page-token keyring under a state-id scope: it must never open as a Page credential
// of any binding, and a Page credential must never open as a pending token.
func TestPendingSealIsNotAPageCredential(t *testing.T) {
	keys, err := metareply.NewPageTokenKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{9}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	const tenant, store, state, binding = "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002",
		"00000000-0000-4000-8000-000000000003", "00000000-0000-4000-8000-000000000004"
	keyID, nonce, ct, err := keys.Seal(pendingScope(tenant, store, state), "USER-TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	if sec, err := keys.Open(pendingScope(tenant, store, state), keyID, nonce, ct); err != nil || string(sec.Reveal()) != "USER-TOKEN" {
		t.Fatalf("pending roundtrip: %v", err)
	}
	for _, s := range []metareply.PageTokenScope{
		pendingScope(tenant, store, binding),
		{TenantID: tenant, StoreID: store, BindingID: state, Provider: "facebook", AssetID: "123", Version: 1},
		pendingScope(tenant, "00000000-0000-4000-8000-000000000009", state),
	} {
		if _, err := keys.Open(s, keyID, nonce, ct); err == nil {
			t.Fatalf("pending ciphertext opened under %+v", s)
		}
	}
}

func TestNewValidatesConfig(t *testing.T) {
	if _, err := New(Config{}, nil); err == nil {
		t.Fatal("empty config accepted")
	}
	if len(StateKeyFor([]byte("secret"))) != 32 || bytes.Equal(StateKeyFor([]byte("secret")), []byte("secret")) {
		t.Fatal("state key must be a derived 32-byte value, never the secret")
	}
}
