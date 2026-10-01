// main_test.go: CLI contract tests with a faked database step (no PG). They prove flag validation happens before any
// connection, stdout carries only the definer's ids/versions, and every failure reduces to one fixed stderr code that
// never contains the DSN, evidence text or a driver message. Definer behaviour is the gate suite's job.

package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/storefrontadmin"
)

const (
	cliStore  = "3f1b0c9e-5a77-4d1e-9d2a-0a7f4c2b9e11"
	cliOrigin = "https://shop.example.com"
	cliDSN    = "COMMERCE_STORE_REGISTRAR_DATABASE_URL"
)

type row struct {
	raw []byte
	err error
}

func (r row) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*[]byte)) = r.raw
	return nil
}

type querier struct {
	r    row
	sql  string
	args []any
}

func (q *querier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.sql, q.args = sql, args
	return q.r
}

// fake replaces the database step and records whether it was reached.
func fake(t *testing.T, q *querier) *int {
	t.Helper()
	orig, origNow := withDB, now
	t.Cleanup(func() { withDB, now = orig, origNow })
	now = func() time.Time { return time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC) }
	opened := new(int)
	withDB = func(_ context.Context, dsn string, fn func(storefrontadmin.Querier) (any, error)) (any, error) {
		*opened++
		return fn(q)
	}
	return opened
}

// realWithDB is the production database step, captured before any test replaces withDB.
var realWithDB = withDB

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestBindHappyPathPrintsOneJSONLine(t *testing.T) {
	q := &querier{r: row{raw: []byte(`{"domain_id":"9a2f6d1c-3b44-4c0a-8f55-1e6d7c8b9a22","version":1,"state":"ACTIVE","renewed":false,"rebound":true}`)}}
	opened := fake(t, q)
	var out bytes.Buffer
	err := run(context.Background(), []string{"domain-bind", "--store", cliStore, "--origin", cliOrigin, "--evidence", "proof-ref",
		"--valid-until", "2026-12-01T00:00:00Z"}, env(map[string]string{cliDSN: "postgres://x"}), &out)
	if err != nil || *opened != 1 {
		t.Fatalf("err=%v opened=%d", err, *opened)
	}
	if got := out.String(); got != `{"domain_id":"9a2f6d1c-3b44-4c0a-8f55-1e6d7c8b9a22","version":1,"state":"ACTIVE","renewed":false,"rebound":true}`+"\n" {
		t.Fatalf("stdout = %q", got)
	}
	if !strings.Contains(q.sql, "operator_bind_domain") {
		t.Fatalf("sql = %s", q.sql)
	}
}

func TestUsageErrorsNeverOpenTheDatabase(t *testing.T) {
	opened := fake(t, &querier{})
	good := env(map[string]string{cliDSN: "postgres://x"})
	for name, args := range map[string][]string{
		"no args":            {},
		"unknown command":    {"domain-rebind"},
		"bind no store":      {"domain-bind", "--origin", cliOrigin, "--evidence", "e", "--valid-until", "2026-12-01T00:00:00Z"},
		"bind no valid":      {"domain-bind", "--store", cliStore, "--origin", cliOrigin, "--evidence", "e"},
		"bind bad time":      {"domain-bind", "--store", cliStore, "--origin", cliOrigin, "--evidence", "e", "--valid-until", "tomorrow"},
		"bind past":          {"domain-bind", "--store", cliStore, "--origin", cliOrigin, "--evidence", "e", "--valid-until", "2026-09-01T00:00:00Z"},
		"bind over 400 days": {"domain-bind", "--store", cliStore, "--origin", cliOrigin, "--evidence", "e", "--valid-until", "2028-01-01T00:00:00Z"},
		"bind http origin":   {"domain-bind", "--store", cliStore, "--origin", "http://shop.example.com", "--evidence", "e", "--valid-until", "2026-12-01T00:00:00Z"},
		"bind extra arg":     {"domain-bind", "--store", cliStore, "--origin", cliOrigin, "--evidence", "e", "--valid-until", "2026-12-01T00:00:00Z", "x"},
		"unknown flag":       {"status", "--store", cliStore, "--tenant", cliStore},
		"suspend no origin":  {"domain-suspend"},
		"detach localhost":   {"domain-detach", "--origin", "https://shop.localhost"},
		"status bad store":   {"status", "--store", "nope"},
	} {
		var out bytes.Buffer
		if err := run(context.Background(), args, good, &out); !errors.Is(err, errUsage) {
			t.Errorf("%s: err = %v, want store_admin_usage", name, err)
		}
		if out.Len() != 0 {
			t.Errorf("%s: wrote stdout", name)
		}
	}
	if *opened != 0 {
		t.Fatalf("database opened %d times for invalid input", *opened)
	}
	if err := run(context.Background(), []string{"status", "--store", cliStore}, env(nil), &bytes.Buffer{}); !errors.Is(err, errConfig) {
		t.Fatalf("missing DSN: %v", err)
	}
}

func TestFailuresReduceToFixedCodes(t *testing.T) {
	good := env(map[string]string{cliDSN: "postgres://u:secret-pw@h/db"})
	for _, tc := range []struct {
		err  error
		want error
	}{
		{&pgconn.PgError{Code: "PT404", Message: "store not found"}, errNotFound},
		{&pgconn.PgError{Code: "PT409", Message: "domain_detached"}, errDetached},
		{&pgconn.PgError{Code: "PT409", Message: "domain_owned_elsewhere"}, errOwnedElsewhere},
		{&pgconn.PgError{Code: "PT409", Message: "store has no owner principal"}, errNoOwner},
		{&pgconn.PgError{Code: "PT409", Message: "version_conflict"}, errConflict},
		{&pgconn.PgError{Code: "42501", Message: "permission denied for function control.operator_bind_domain"}, errFailed},
		{errors.New("dial tcp: lookup db: secret-pw"), errFailed},
	} {
		fake(t, &querier{r: row{err: tc.err}})
		err := run(context.Background(), []string{"domain-suspend", "--origin", cliOrigin}, good, &bytes.Buffer{})
		if err != tc.want {
			t.Errorf("%v -> %v, want %v", tc.err, err, tc.want)
		}
		if err != nil && (strings.Contains(err.Error(), "secret-pw") || strings.Contains(err.Error(), "permission denied")) {
			t.Errorf("leak in %q", err)
		}
	}
	// the real database step: an unparsable DSN is reported fixed, without echoing it
	if _, err := realWithDB(context.Background(), "postgres://u:secret-pw@h:notaport/db", nil); err != errDatabase {
		t.Fatalf("bad DSN: %v", err)
	}
}
