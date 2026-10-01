package identity

// Pure-logic tests for the staff service: SQLSTATE mapping, mail content and the invitation link. No database.

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestStaffErrorMapping(t *testing.T) {
	pg := func(code, msg string) error { return &pgconn.PgError{Code: code, Message: msg} }
	for _, tc := range []struct {
		in   error
		want error
	}{
		{pg("PT400", "invalid staff request"), ErrInvalid}, {pg("PT401", "unauthorized"), ErrUnauthorized}, {pg("PT403", "forbidden"), ErrForbidden},
		{pg("PT404", "not found"), ErrNotFound}, {pg("PT404", "invite_invalid"), ErrInviteInvalid},
		{pg("PT409", "already_member"), ErrAlreadyMember}, {pg("PT409", "last_owner"), ErrLastOwner}, {pg("PT409", "other"), ErrConflict},
		{pg("PT429", "too_many_invitations"), ErrTooManyInvitations},
		{pg("55P03", "lock timeout"), ErrUnavailable}, {errors.New("network"), ErrUnavailable},
	} {
		if got := staffError(tc.in); !errors.Is(got, tc.want) {
			t.Errorf("staffError(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if staffError(nil) != nil {
		t.Error("nil must stay nil")
	}
}

func TestStaffInviteMailLocalesAndEscaping(t *testing.T) {
	link := "https://admin.example.test/zh-TW/invite/" + strings.Repeat("A", 43)
	for _, locale := range []string{"en", "zh-CN", "zh-TW"} {
		for role := range StaffRoles {
			m := staffInviteMail("a@example.test", locale, role, link)
			if m.To != "a@example.test" || m.Subject == "" || !strings.Contains(m.Text, link) || !strings.Contains(m.HTML, `href="`+link+`"`) {
				t.Fatalf("%s/%s: %+v", locale, role, m)
			}
			if !strings.Contains(m.Text, staffRoleNames[locale][role]) || strings.Contains(m.Subject, link) {
				t.Fatalf("%s/%s: role name missing or token in subject", locale, role)
			}
		}
	}
	m := staffInviteMail("a@example.test", "en", "viewer", `https://x.test/en/invite/"><script>`)
	if strings.Contains(m.HTML, "<script>") {
		t.Fatal("link must be HTML-escaped")
	}
}

func TestStaffRolesVocabularyMatchesSQL(t *testing.T) {
	// The CHECK in 0089 and StaffRoles must agree; the mail copy must name every role in every locale.
	if len(StaffRoles) != 5 {
		t.Fatalf("roles = %d", len(StaffRoles))
	}
	for locale, names := range staffRoleNames {
		for role := range StaffRoles {
			if names[role] == "" {
				t.Errorf("%s lacks a label for %s", locale, role)
			}
		}
	}
}

func TestNewUUIDShape(t *testing.T) {
	a, b := newUUID(), newUUID()
	if !uuidPattern.MatchString(a) || a == b || a[14] != '4' {
		t.Fatalf("uuid %q %q", a, b)
	}
}

func TestStaffBadInputNeverTouchesDatabase(t *testing.T) {
	s := &Staff{} // nil pool: any database access would panic, so these must be rejected first
	tok := randomToken()
	if _, err := s.List(nil, "short", "x"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err := s.List(nil, tok, "not-a-uuid"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Invite(nil, tok, newUUID(), "bad", "viewer", "en"); !errors.Is(err, ErrInvalidEmail) && !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Invite(nil, tok, newUUID(), "a@example.test", "root", "en"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := s.SetRole(nil, tok, newUUID(), newUUID(), "root"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Accept(nil, tok, "short"); !errors.Is(err, ErrInviteInvalid) {
		t.Fatal("a malformed token must look like any other invalid invitation:", err)
	}
}
