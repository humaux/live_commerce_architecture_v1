package foundation_test

// Shared helpers of the customers-billing gates (CB02-CB09). Prefix `cbx`. No test lives here.

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// cbxLogin opens a plain (non-strict) login that is a member of exactly the given roles. It is used only
// for calls the gate makes AS a role (consent_allows as commerce_auth, EXECUTE probes), never as a production pool.
func cbxLogin(t *testing.T, f *testFixture, roles ...string) *pgxpool.Pool {
	t.Helper()
	role := "cbx_" + t04Tag()
	password := randomToken()
	membership := ""
	for i, r := range roles {
		if i > 0 {
			membership += ","
		}
		membership += pgx.Identifier{r}.Sanitize()
	}
	mustExec(t, f.owner, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()+` LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION IN ROLE `+membership+` PASSWORD '`+password+`'`)
	pool, err := pgxpool.New(context.Background(), roleURL(t, f.databaseURL, role, password))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		mustExec(t, f.owner, `DROP OWNED BY `+pgx.Identifier{role}.Sanitize())
		mustExec(t, f.owner, `DROP ROLE `+pgx.Identifier{role}.Sanitize())
	})
	return pool
}

// cbxOne returns the first column of the first row as text ("" for NULL or no row).
func cbxOne(t *testing.T, f *testFixture, q string, args ...any) string {
	t.Helper()
	var s *string
	if err := f.owner.QueryRow(context.Background(), q, args...).Scan(&s); err != nil && err != pgx.ErrNoRows {
		t.Fatalf("query %q: %v", q, err)
	}
	if s == nil {
		return ""
	}
	return *s
}

// cbxAllows calls the consent gate as the owner role (a fixture superuser: the gate's own behaviour with real
// logins and GUC states is CB04's).
func cbxAllows(t *testing.T, f *testFixture, tenant, store, owner, purpose, channel string) bool {
	t.Helper()
	var ok bool
	if err := f.owner.QueryRow(context.Background(), `SELECT customers.consent_allows($1,$2,$3,$4,$5)`, tenant, store, owner, purpose, channel).Scan(&ok); err != nil {
		t.Fatalf("consent_allows: %v", err)
	}
	return ok
}
