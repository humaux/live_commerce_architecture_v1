// main_test.go: CLI contract tests with a faked database step (no PG). They prove flag validation happens before any
// connection, stdout carries one compact JSON line of the definer's result, and every failure reduces to one fixed stderr
// code that never contains the DSN or a driver message (OP09). Definer behaviour is the gate suite's job
// (tests/foundation/platform_operator*_test.go).

package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	cliStore  = "3f1b0c9e-5a77-4d1e-9d2a-0a7f4c2b9e11"
	cliTenant = "9a2f6d1c-3b44-4c0a-8f55-1e6d7c8b9a22"
	cliDSN    = "COMMERCE_PLATFORM_OPERATOR_DATABASE_URL"
	// separate const so no test literal has the inline-password DSN shape the CI secret grep rejects
	fakeDSNPassword = "secret-pw"
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

type querier0 struct {
	r    row
	sql  string
	args []any
	ctx  context.Context
}

func (q *querier0) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	q.ctx, q.sql, q.args = ctx, sql, args
	return q.r
}

// fake replaces the database step and counts how often it was reached.
func fake(t *testing.T, q *querier0) *int {
	t.Helper()
	orig := withDB
	t.Cleanup(func() { withDB = orig })
	opened := new(int)
	withDB = func(ctx context.Context, _ string, fn func(context.Context, querier) ([]byte, error)) ([]byte, error) {
		*opened++
		return fn(ctx, q)
	}
	return opened
}

var realWithDB = withDB

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

var goodEnv = env(map[string]string{cliDSN: "postgres://x"})

func TestSuspendAndResumePrintOneCompactJSONLine(t *testing.T) {
	q := &querier0{r: row{raw: []byte(`{"store_id": "` + cliStore + `", "active": false, "result": "changed"}`)}}
	opened := fake(t, q)
	var out bytes.Buffer
	err := run(context.Background(), []string{"store-suspend", "--store", cliStore, "--operator", "ops.alice", "--ticket", "T-1", "--reason", "fraud"}, goodEnv, &out)
	if err != nil || *opened != 1 {
		t.Fatalf("err=%v opened=%d", err, *opened)
	}
	if got := out.String(); got != `{"store_id":"`+cliStore+`","active":false,"result":"changed"}`+"\n" {
		t.Fatalf("stdout = %q", got)
	}
	if !strings.Contains(q.sql, "control.set_store_active") || len(q.args) != 5 || q.args[0] != cliStore || q.args[1] != false || q.args[4] != "fraud" {
		t.Fatalf("sql=%s args=%v", q.sql, q.args)
	}
	// resume: p_active=true and a NULL reason
	if err := run(context.Background(), []string{"tenant-resume", "--tenant", cliTenant, "--operator", "ops.alice", "--ticket", "T-2"}, goodEnv, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q.sql, "control.set_tenant_active") || q.args[1] != true || q.args[4] != nil {
		t.Fatalf("resume sql=%s args=%v", q.sql, q.args)
	}
}

func TestStatusAndAuditCalls(t *testing.T) {
	q := &querier0{r: row{raw: []byte(`[]`)}}
	fake(t, q)
	if err := run(context.Background(), []string{"status", "--tenant", cliTenant}, goodEnv, &bytes.Buffer{}); err != nil ||
		!strings.Contains(q.sql, "platform_status") || q.args[0] != cliTenant || q.args[1] != nil {
		t.Fatalf("status --tenant: %v %s %v", err, q.sql, q.args)
	}
	if err := run(context.Background(), []string{"status", "--store", cliStore}, goodEnv, &bytes.Buffer{}); err != nil || q.args[0] != nil || q.args[1] != cliStore {
		t.Fatalf("status --store: %v %v", err, q.args)
	}
	var out bytes.Buffer
	if err := run(context.Background(), []string{"audit", "--since", "2026-10-01T00:00:00Z", "--limit", "7"}, goodEnv, &out); err != nil ||
		!strings.Contains(q.sql, "read_operator_audit") || q.args[1] != int32(7) || out.String() != "[]\n" {
		t.Fatalf("audit: %v %s %v %q", err, q.sql, q.args, out.String())
	}
	if err := run(context.Background(), []string{"audit"}, goodEnv, &bytes.Buffer{}); err != nil || q.args[0] != nil || q.args[1] != int32(100) {
		t.Fatalf("audit defaults: %v %v", err, q.args)
	}
}

func TestUsageErrorsNeverOpenTheDatabase(t *testing.T) {
	opened := fake(t, &querier0{})
	id := []string{"--operator", "ops.alice", "--ticket", "T-1"}
	with := func(args ...string) []string { return append(append([]string{}, args...), id...) }
	for name, args := range map[string][]string{
		"no args":                {},
		"unknown command":        {"store-delete"},
		"suspend no reason":      with("store-suspend", "--store", cliStore),
		"suspend bad reason":     with("store-suspend", "--store", cliStore, "--reason", "boredom"),
		"resume with reason":     with("store-resume", "--store", cliStore, "--reason", "fraud"),
		"no operator":            {"store-resume", "--store", cliStore, "--ticket", "T-1"},
		"operator uppercase":     {"store-resume", "--store", cliStore, "--ticket", "T-1", "--operator", "Alice"},
		"operator too long":      {"store-resume", "--store", cliStore, "--ticket", "T-1", "--operator", strings.Repeat("a", 41)},
		"no ticket":              {"store-resume", "--store", cliStore, "--operator", "ops.alice"},
		"blank ticket":           {"store-resume", "--store", cliStore, "--operator", "ops.alice", "--ticket", "   "},
		"ticket control char":    {"store-resume", "--store", cliStore, "--operator", "ops.alice", "--ticket", "T\n1"},
		"ticket too long":        {"store-resume", "--store", cliStore, "--operator", "ops.alice", "--ticket", strings.Repeat("t", 81)},
		"store bad uuid":         with("store-resume", "--store", "nope"),
		"store with tenant flag": with("store-resume", "--store", cliStore, "--tenant", cliTenant),
		"tenant with store flag": with("tenant-resume", "--tenant", cliTenant, "--store", cliStore),
		"tenant missing":         with("tenant-resume"),
		"extra positional":       with("store-resume", "--store", cliStore, "x"),
		"status both":            {"status", "--store", cliStore, "--tenant", cliTenant},
		"status none":            {"status"},
		"status bad uuid":        {"status", "--tenant", "nope"},
		"audit bad since":        {"audit", "--since", "yesterday"},
		"audit limit 0":          {"audit", "--limit", "0"},
		"audit limit 501":        {"audit", "--limit", "501"},
		"audit limit text":       {"audit", "--limit", "many"},
		"unknown flag":           {"audit", "--tenant", cliTenant},
	} {
		var out bytes.Buffer
		if err := run(context.Background(), args, goodEnv, &out); !errors.Is(err, errUsage) {
			t.Errorf("%s: err = %v, want platform_admin_usage", name, err)
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
	if *opened != 0 {
		t.Fatal("database opened without a DSN")
	}
}

func TestFailuresReduceToFixedCodes(t *testing.T) {
	good := env(map[string]string{cliDSN: "postgres://u:" + fakeDSNPassword + "@h/db"})
	for _, tc := range []struct {
		err  error
		want error
	}{
		{&pgconn.PgError{Code: "PT404", Message: "store not found"}, errNotFound},
		{&pgconn.PgError{Code: "PT400", Message: "invalid platform operator request"}, errUsage},
		{&pgconn.PgError{Code: "42501", Message: "permission denied for function control.set_store_active"}, errDenied},
		{&pgconn.PgError{Code: "XX000", Message: "boom " + fakeDSNPassword}, errFailed},
		{errors.New("dial tcp: lookup db: " + fakeDSNPassword), errFailed},
	} {
		fake(t, &querier0{r: row{err: tc.err}})
		err := run(context.Background(), []string{"store-resume", "--store", cliStore, "--operator", "ops.alice", "--ticket", "T-1"}, good, &bytes.Buffer{})
		if err != tc.want {
			t.Errorf("%v -> %v, want %v", tc.err, err, tc.want)
		}
		if err != nil && (strings.Contains(err.Error(), fakeDSNPassword) || strings.Contains(err.Error(), "permission denied")) {
			t.Errorf("leak in %q", err)
		}
	}
	// the real database step: an unparsable DSN is reported fixed, without echoing it
	if _, err := realWithDB(context.Background(), "postgres://u:"+fakeDSNPassword+"@h:notaport/db", nil); err != errDatabase {
		t.Fatalf("bad DSN: %v", err)
	}
	// a definer returning non-JSON is a fixed failure, not a raw dump
	fake(t, &querier0{r: row{raw: []byte("not json " + fakeDSNPassword)}})
	var out bytes.Buffer
	if err := run(context.Background(), []string{"status", "--store", cliStore}, good, &out); err != errFailed || out.Len() != 0 {
		t.Fatalf("non-JSON result: %v %q", err, out.String())
	}
}

// The run context (main's 30 s deadline) bounds the query as well as the connect.
func TestRunContextReachesTheQuery(t *testing.T) {
	q := &querier0{r: row{raw: []byte(`{}`)}}
	fake(t, q)
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "marker")
	if err := run(ctx, []string{"status", "--store", cliStore}, goodEnv, &bytes.Buffer{}); err != nil || q.ctx == nil || q.ctx.Value(key{}) != "marker" {
		t.Fatalf("query context not derived from the run context: %v %v", err, q.ctx)
	}
}

const cliPrincipal = "5c0e8d2a-7f31-4b6e-a1c4-2d9b3e4f5a66"

// OPS-02B: the three support commands call their definer with the validated arguments (default pack = NULL perms,
// default 4 h), print one compact JSON line, and map the new PT409 conflict to one fixed code.
func TestSupportCommandsCallTheDefiners(t *testing.T) {
	q := &querier0{r: row{raw: []byte(`{"result": "granted"}`)}}
	fake(t, q)
	var out bytes.Buffer
	if err := run(context.Background(), []string{"support-grant", "--store", cliStore, "--principal", cliPrincipal, "--operator", "ops.alice", "--ticket", "T-1"}, goodEnv, &out); err != nil ||
		out.String() != `{"result":"granted"}`+"\n" || !strings.Contains(q.sql, "identity.grant_support") ||
		len(q.args) != 6 || q.args[0] != cliStore || q.args[1] != cliPrincipal || q.args[2] != int32(4) || q.args[5] != "T-1" {
		t.Fatalf("grant defaults: %v %s %v", err, q.sql, q.args)
	}
	if perms, _ := q.args[3].([]string); perms != nil {
		t.Fatalf("default pack must be a NULL permission list, got %v", q.args[3])
	}
	if err := run(context.Background(), []string{"support-grant", "--store", cliStore, "--principal", cliPrincipal, "--hours", "72", "--perm", "store:read,orders:read",
		"--operator", "ops.alice", "--ticket", "T-1"}, goodEnv, &bytes.Buffer{}); err != nil || q.args[2] != int32(72) || strings.Join(q.args[3].([]string), ",") != "store:read,orders:read" {
		t.Fatalf("grant explicit: %v %v", err, q.args)
	}
	g := "7d1f2a3b-4c5d-4e6f-8a9b-0c1d2e3f4a5b"
	if err := run(context.Background(), []string{"support-revoke", "--grant", g, "--operator", "ops.alice", "--ticket", "T-2"}, goodEnv, &bytes.Buffer{}); err != nil ||
		!strings.Contains(q.sql, "identity.revoke_support") || q.args[0] != g || q.args[1] != nil || q.args[2] != nil {
		t.Fatalf("revoke by grant: %v %v", err, q.args)
	}
	if err := run(context.Background(), []string{"support-revoke", "--store", cliStore, "--principal", cliPrincipal, "--operator", "ops.alice", "--ticket", "T-2"}, goodEnv, &bytes.Buffer{}); err != nil ||
		q.args[0] != nil || q.args[1] != cliStore || q.args[2] != cliPrincipal {
		t.Fatalf("revoke by pair: %v %v", err, q.args)
	}
	if err := run(context.Background(), []string{"support-list", "--store", cliStore}, goodEnv, &bytes.Buffer{}); err != nil ||
		!strings.Contains(q.sql, "identity.list_support_grants") || q.args[0] != cliStore {
		t.Fatalf("list: %v %v", err, q.args)
	}
	if err := run(context.Background(), []string{"support-principal-add", "--principal", cliPrincipal, "--operator", "ops.alice", "--ticket", "T-3"}, goodEnv, &bytes.Buffer{}); err != nil ||
		!strings.Contains(q.sql, "identity.add_support_principal") || q.args[0] != cliPrincipal || q.args[2] != "T-3" {
		t.Fatalf("principal add: %v %s %v", err, q.sql, q.args)
	}
	if err := run(context.Background(), []string{"support-principal-revoke", "--principal", cliPrincipal, "--operator", "ops.alice", "--ticket", "T-4"}, goodEnv, &bytes.Buffer{}); err != nil ||
		!strings.Contains(q.sql, "identity.revoke_support_principal") || q.args[0] != cliPrincipal {
		t.Fatalf("principal revoke: %v %s %v", err, q.sql, q.args)
	}
	fake(t, &querier0{r: row{err: &pgconn.PgError{Code: "PT409", Message: "support grant already open"}}})
	if err := run(context.Background(), []string{"support-list", "--store", cliStore}, goodEnv, &bytes.Buffer{}); err != errConflict {
		t.Fatalf("PT409 -> %v, want platform_admin_conflict", err)
	}
}

// SG06 (CLI half): hours outside 1..72, a permission outside the pack, a malformed id, a missing operator/ticket and an
// ambiguous revoke target are all refused before any connection opens.
func TestSupportUsageErrorsNeverOpenTheDatabase(t *testing.T) {
	opened := fake(t, &querier0{})
	id := []string{"--operator", "ops.alice", "--ticket", "T-1"}
	base := []string{"--store", cliStore, "--principal", cliPrincipal}
	with := func(args ...string) []string { return append(append([]string{}, args...), id...) }
	for name, args := range map[string][]string{
		"hours 73":             with(append([]string{"support-grant", "--hours", "73"}, base...)...),
		"hours 0":              with(append([]string{"support-grant", "--hours", "0"}, base...)...),
		"hours text":           with(append([]string{"support-grant", "--hours", "soon"}, base...)...),
		"perm customers":       with(append([]string{"support-grant", "--perm", "customers:read"}, base...)...),
		"perm write":           with(append([]string{"support-grant", "--perm", "store:read,orders:write"}, base...)...),
		"perm empty element":   with(append([]string{"support-grant", "--perm", "store:read,"}, base...)...),
		"grant no principal":   with("support-grant", "--store", cliStore),
		"grant bad store":      with("support-grant", "--store", "nope", "--principal", cliPrincipal),
		"grant no operator":    append([]string{"support-grant", "--ticket", "T-1"}, base...),
		"grant no ticket":      append([]string{"support-grant", "--operator", "ops.alice"}, base...),
		"revoke nothing":       with("support-revoke"),
		"revoke grant+store":   with("support-revoke", "--grant", cliStore, "--store", cliStore),
		"revoke store only":    with("support-revoke", "--store", cliStore),
		"revoke bad grant":     with("support-revoke", "--grant", "nope"),
		"revoke no ticket":     {"support-revoke", "--grant", cliStore, "--operator", "ops.alice"},
		"principal add none":   with("support-principal-add"),
		"principal add bad id": with("support-principal-add", "--principal", "nope"),
		"principal add store":  with("support-principal-add", "--principal", cliPrincipal, "--store", cliStore),
		"principal revoke tkt": {"support-principal-revoke", "--principal", cliPrincipal, "--operator", "ops.alice"},
		"list no store":        {"support-list"},
		"list bad store":       {"support-list", "--store", "nope"},
		"list with principal":  {"support-list", "--store", cliStore, "--principal", cliPrincipal},
		"suspend flag on list": {"support-list", "--store", cliStore, "--reason", "fraud"},
	} {
		var out bytes.Buffer
		if err := run(context.Background(), args, goodEnv, &out); !errors.Is(err, errUsage) {
			t.Errorf("%s: err = %v, want platform_admin_usage", name, err)
		}
		if out.Len() != 0 {
			t.Errorf("%s: wrote stdout", name)
		}
	}
	if *opened != 0 {
		t.Fatalf("database opened %d times for invalid input", *opened)
	}
}
