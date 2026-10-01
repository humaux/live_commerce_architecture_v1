// storefrontadmin_test.go: pure-logic unit tests (no database). They fake the one-row definer answers to prove input
// validation at the trust boundary, strict result decoding and SQLSTATE mapping. The definers themselves (lifecycle,
// compare-and-set, audit, grants) are proven against real PG by the independent gate suite, not here.

package storefrontadmin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const (
	testStore  = "3f1b0c9e-5a77-4d1e-9d2a-0a7f4c2b9e11"
	testDomain = "9a2f6d1c-3b44-4c0a-8f55-1e6d7c8b9a22"
)

type fakeRow struct {
	raw []byte
	err error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*[]byte)) = r.raw
	return nil
}

type fakeQ struct {
	row   fakeRow
	calls int
	args  []any
}

func (f *fakeQ) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	f.calls++
	f.args = args
	return f.row
}

func pgErr(code, msg string) error { return &pgconn.PgError{Code: code, Message: msg} }

func TestValidEvidence(t *testing.T) {
	for _, tc := range []struct {
		in string
		ok bool
	}{
		{"cert-2026-10-01 openssl notAfter proof", true},
		{"x", true},
		{strings.Repeat("a", 240), true},
		{strings.Repeat("a", 241), false},
		{"", false},
		{"   ", false},
		{"tab\there", false},
		{"line\nbreak", false},
		{"nul\x00byte", false},
		{"bad\xffutf8", false},
	} {
		if got := ValidEvidence(tc.in); got != tc.ok {
			t.Errorf("ValidEvidence(%q) = %v, want %v", tc.in, got, tc.ok)
		}
	}
}

func TestBindDomainInputsFailBeforeTheDatabase(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	good := now.Add(90 * 24 * time.Hour)
	for name, tc := range map[string]struct {
		store, origin, evidence string
		until                   time.Time
	}{
		"bad store":        {"not-a-uuid", "https://shop.example.com", "proof", good},
		"http origin":      {testStore, "http://shop.example.com", "proof", good},
		"port":             {testStore, "https://shop.example.com:443", "proof", good},
		"localhost":        {testStore, "https://shop.localhost", "proof", good},
		"uppercase":        {testStore, "https://Shop.example.com", "proof", good},
		"trailing slash":   {testStore, "https://shop.example.com/", "proof", good},
		"empty evidence":   {testStore, "https://shop.example.com", "  ", good},
		"expired":          {testStore, "https://shop.example.com", "proof", now},
		"past":             {testStore, "https://shop.example.com", "proof", now.Add(-time.Hour)},
		"beyond 400 days":  {testStore, "https://shop.example.com", "proof", now.Add(401 * 24 * time.Hour)},
		"control evidence": {testStore, "https://shop.example.com", "a\x07b", good},
	} {
		q := &fakeQ{}
		if _, err := BindDomain(context.Background(), q, tc.store, tc.origin, tc.evidence, tc.until, now); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
		if q.calls != 0 {
			t.Errorf("%s: reached the database", name)
		}
	}
}

func TestBindDomainDecodesStrictly(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	until := now.Add(60 * 24 * time.Hour)
	ok := &fakeQ{row: fakeRow{raw: []byte(`{"domain_id":"` + testDomain + `","version":2,"state":"ACTIVE","renewed":true,"rebound":false}`)}}
	out, err := BindDomain(context.Background(), ok, testStore, "https://shop.example.com", "proof", until, now)
	if err != nil || out.DomainID != testDomain || out.Version != 2 || !out.Renewed {
		t.Fatalf("got %+v, %v", out, err)
	}
	if out.Rebound {
		t.Fatalf("rebound decoded true from false: %+v", out)
	}
	if ok.args[1] != "https://shop.example.com" || ok.args[3] != until.UTC() {
		t.Fatalf("definer args = %v", ok.args)
	}
	reb := &fakeQ{row: fakeRow{raw: []byte(`{"domain_id":"` + testDomain + `","version":5,"state":"ACTIVE","renewed":false,"rebound":true}`)}}
	if out, err := BindDomain(context.Background(), reb, testStore, "https://shop.example.com", "new-proof", until, now); err != nil || !out.Rebound || out.Version != 5 {
		t.Fatalf("rebind result: %+v %v", out, err)
	}
	for name, raw := range map[string]string{
		"unknown field": `{"domain_id":"` + testDomain + `","version":1,"state":"ACTIVE","renewed":false,"rebound":false,"extra":1}`,
		"wrong state":   `{"domain_id":"` + testDomain + `","version":1,"state":"SUSPENDED","renewed":false}`,
		"bad id":        `{"domain_id":"x","version":1,"state":"ACTIVE","renewed":false}`,
		"zero version":  `{"domain_id":"` + testDomain + `","version":0,"state":"ACTIVE","renewed":false}`,
		"trailing":      `{"domain_id":"` + testDomain + `","version":1,"state":"ACTIVE","renewed":false} {}`,
		"not json":      `nope`,
	} {
		q := &fakeQ{row: fakeRow{raw: []byte(raw)}}
		if _, err := BindDomain(context.Background(), q, testStore, "https://shop.example.com", "proof", until, now); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: err = %v, want ErrUnavailable", name, err)
		}
	}
}

func TestErrorMapping(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	until := now.Add(24 * time.Hour)
	for _, tc := range []struct {
		err  error
		want error
	}{
		{pgErr("PT409", "domain_detached"), ErrDomainDetached},
		{pgErr("PT409", "domain_owned_elsewhere"), ErrDomainOwnedElsewhere},
		{pgErr("PT409", "store has no owner principal"), ErrNoOwner},
		{pgErr("PT409", "version_conflict"), command.ErrConflict},
		{pgErr("PT400", "invalid domain request"), command.ErrInvalid},
		{pgErr("PT404", "store not found"), platform.ErrScopeNotFound},
		{pgErr("PT401", "unauthorized"), platform.ErrUnauthorized},
		{pgErr("PT403", "forbidden"), platform.ErrForbidden},
		{pgErr("42501", "permission denied for function secret_name"), ErrUnavailable},
		{errors.New("dial tcp 10.0.0.1: connection refused password=hunter2"), ErrUnavailable},
		{context.DeadlineExceeded, context.DeadlineExceeded},
	} {
		q := &fakeQ{row: fakeRow{err: tc.err}}
		_, err := BindDomain(context.Background(), q, testStore, "https://shop.example.com", "proof", until, now)
		if !errors.Is(err, tc.want) {
			t.Errorf("%v -> %v, want %v", tc.err, err, tc.want)
		}
		if err != nil && (strings.Contains(err.Error(), "hunter2") || strings.Contains(err.Error(), "secret_name")) {
			t.Errorf("driver text leaked: %v", err)
		}
	}
	// 40001/23505 pass through for the HTTP layer's retry/conflict mapping.
	raced := pgErr("23505", "dup")
	if _, err := SuspendDomain(context.Background(), &fakeQ{row: fakeRow{err: raced}}, "https://shop.example.com"); !errors.Is(err, raced) {
		t.Errorf("23505 not passed through: %v", err)
	}
}

func TestMoveAndStatus(t *testing.T) {
	q := &fakeQ{row: fakeRow{raw: []byte(`{"domain_id":"` + testDomain + `","version":3,"state":"SUSPENDED","changed":true}`)}}
	if out, err := SuspendDomain(context.Background(), q, "https://shop.example.com"); err != nil || !out.Changed || out.Version != 3 {
		t.Fatalf("suspend: %+v %v", out, err)
	}
	// a definer that answers the wrong state must not be reported as the requested one
	if _, err := DetachDomain(context.Background(), q, "https://shop.example.com"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("detach with SUSPENDED answer: %v", err)
	}
	for _, bad := range []string{"http://x.example.com", "https://localhost", ""} {
		if _, err := SuspendDomain(context.Background(), &fakeQ{}, bad); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("origin %q: %v", bad, err)
		}
	}

	st := &fakeQ{row: fakeRow{raw: []byte(`{"store_id":"` + testStore + `","published":true,"version":2,"domains":[` +
		`{"origin":"https://shop.example.com","state":"ACTIVE","version":4,"valid_until":"2027-01-01T00:00:00Z","serving":true},` +
		`{"origin":"https://old.example.com","state":"DETACHED","version":2,"valid_until":null,"serving":false}]}`)}}
	out, err := Status(context.Background(), st, testStore)
	if err != nil || !out.Published || len(out.Domains) != 2 || out.Domains[1].ValidUntil != nil {
		t.Fatalf("status: %+v %v", out, err)
	}
	other := &fakeQ{row: fakeRow{raw: []byte(`{"store_id":"` + testDomain + `","published":false,"version":0,"domains":[]}`)}}
	if _, err := Status(context.Background(), other, testStore); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("status of a different store id accepted: %v", err)
	}
}

func TestMerchantInputsFailBeforeTheDatabase(t *testing.T) {
	scope := platform.Scope{TenantID: testDomain, StoreID: testStore, PrincipalID: testDomain, Revision: 1}
	token := strings.Repeat("t", 43)
	if _, err := Read(context.Background(), nil, scope, token); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("nil tx: %v", err)
	}
	if _, err := SetPublished(context.Background(), nil, scope, token, true, 0); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("nil tx set: %v", err)
	}
	if _, err := SetPublished(context.Background(), nil, scope, token, true, -1); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("negative version: %v", err)
	}
}
