package metaconnect

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
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

// The USER token is held in memory only, per state, bound to tenant + store, expiring, zeroed on drop and bounded in number.
func TestPendingUserTokenLivesOnlyInMemory(t *testing.T) {
	s := &Service{pending: map[string]*pendingUser{}}
	tok := []byte("USER-TOKEN")
	if !s.hold("st1", "t1", "s1", tok, time.Now().Add(time.Minute)) {
		t.Fatal("hold refused")
	}
	clear(tok) // the caller zeroing its copy must not touch the held one
	got, ok := s.peek("st1", "t1", "s1")
	if !ok || string(got) != "USER-TOKEN" {
		t.Fatalf("peek: %q %v", got, ok)
	}
	clear(got) // nor must zeroing a peeked copy
	if again, _ := s.peek("st1", "t1", "s1"); string(again) != "USER-TOKEN" {
		t.Fatal("peek returned the live slice")
	}
	if _, ok := s.peek("st1", "t2", "s1"); ok {
		t.Error("another tenant read the token")
	}
	if _, ok := s.peek("st1", "t1", "s2"); ok {
		t.Error("another store read the token")
	}
	held := s.pending["st1"].token
	s.drop("st1")
	if _, ok := s.peek("st1", "t1", "s1"); ok || string(held) == "USER-TOKEN" {
		t.Error("drop must forget and zero the token")
	}
	s.hold("old", "t1", "s1", []byte("OLD"), time.Now().Add(-time.Second))
	oldTok := s.pending["old"].token
	if _, ok := s.peek("old", "t1", "s1"); ok {
		t.Error("an expired token was readable")
	}
	s.hold("new", "t1", "s1", []byte("NEW"), time.Now().Add(time.Minute)) // sweeps the expired entry
	if string(oldTok) == "OLD" || s.pending["old"] != nil {
		t.Error("an expired token must be zeroed and removed on the next hold")
	}
	for i := 0; i < maxPending; i++ {
		s.hold("fill"+strconv.Itoa(i), "t1", "s1", []byte("X"), time.Now().Add(time.Minute))
	}
	if s.hold("overflow", "t1", "s1", []byte("X"), time.Now().Add(time.Minute)) {
		t.Error("the in-memory store must be bounded")
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
