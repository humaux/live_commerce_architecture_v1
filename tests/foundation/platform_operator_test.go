package foundation_test

// R3 unit OPS-01B REAL_PG gate (docs/delivery/units/ops-01b-store-suspend.md; migration 0143). Written from the brief:
//
//	OP01 merchant API stops for a suspended store/tenant, siblings unaffected   OP05 resume restores everything, data intact
//	OP02 buyer surface (resolver, capability) stops                              OP06 repeat suspend = unchanged + 2 audit rows
//	OP03 claim comment recorded, no reply planned; no mail claimed               OP07 operator_audit is append-only for every role
//	(OP04 payment notify for an inactive store keeps recording: pinned by TestK3W401BDisabledStoreStillRecordsAndWakes)
//	OP08 EXECUTE matrix lives in platform_operator_authority_test.go; OP09 CLI output scan in cmd/platform-admin + the process test below.
//
// Evidence class: REAL_PG, no external service.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/platform"
)

type poFix struct {
	t  *testing.T
	op *pgxpool.Pool // login holding commerce_platform_operator
}

func poSetup(t *testing.T, f *testFixture) *poFix {
	t.Helper()
	return &poFix{t: t, op: miPool(t, f, "commerce_platform_operator")}
}

// setStore / setTenant call the operator definers as the operator login and return the JSON result.
func (p *poFix) setStore(store string, active bool, reason any) (map[string]any, error) {
	return p.json(`SELECT control.set_store_active($1,$2,'op.test','TICKET-1',$3)::text`, store, active, reason)
}
func (p *poFix) setTenant(tenant string, active bool, reason any) (map[string]any, error) {
	return p.json(`SELECT control.set_tenant_active($1,$2,'op.test','TICKET-1',$3)::text`, tenant, active, reason)
}
func (p *poFix) json(sql string, args ...any) (map[string]any, error) {
	var raw string
	if err := p.op.QueryRow(context.Background(), sql, args...).Scan(&raw); err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		p.t.Fatal(err)
	}
	return out, nil
}
func (p *poFix) must(out map[string]any, err error) map[string]any {
	p.t.Helper()
	if err != nil {
		p.t.Fatalf("operator call: %v", err)
	}
	return out
}

// audits lists "action/changed" of the operator audit rows of one tenant, oldest first (read as the migration owner).
func poAudits(t *testing.T, owner *pgxpool.Pool, tenant string) []string {
	t.Helper()
	rows, err := owner.Query(context.Background(), `SELECT action||'/'||(detail->>'changed') FROM control.operator_audit WHERE tenant_id=$1 ORDER BY occurred_at,id`, tenant)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// OP01 + OP05: the merchant API answers 200 before, is refused while suspended (store, then tenant), the sibling store of
// the same tenant is unaffected by a store suspension, and resume restores service.
func TestPlatformOperatorOP01MerchantStopsAndSiblingUnaffected(t *testing.T) {
	s := spSetup(t)
	f, b := s.f, s.f.base
	p := poSetup(t, b)
	get := func(store string) int {
		code, _ := spJSON(t, s.h, "GET", "/v1/admin/stores/"+store+"/storefront", f.token, "")
		return code
	}
	denied := func(label string, code int) {
		t.Helper()
		if code != 401 && code != 403 && code != 404 {
			t.Fatalf("%s: merchant API answered %d, want a refusal", label, code)
		}
	}
	scopeErr := func(store string) error {
		return platform.WithScope(context.Background(), b.runtime, f.token, store, "store:read", func(pgx.Tx, platform.Scope) error { return nil })
	}
	if get(f.store) != 200 || get(f.otherStore) != 200 || scopeErr(f.store) != nil {
		t.Fatal("precondition: the merchant reaches both stores")
	}
	// store suspend
	out := p.must(p.setStore(f.store, false, "non_payment"))
	if out["result"] != "changed" || out["active"] != false || out["tenant_id"] != f.tenant {
		t.Fatalf("store suspend result %v", out)
	}
	denied("suspended store", get(f.store))
	if scopeErr(f.store) == nil {
		t.Fatal("platform.WithScope admitted a suspended store")
	}
	if get(f.otherStore) != 200 {
		t.Fatal("suspending one store stopped its sibling")
	}
	p.must(p.setStore(f.store, true, nil))
	if get(f.store) != 200 || scopeErr(f.store) != nil {
		t.Fatal("resume did not restore the store")
	}
	// tenant suspend stops every store; resume restores exactly the previous per-store state (OP05).
	p.must(p.setStore(f.otherStore, false, "legal"))
	out = p.must(p.setTenant(f.tenant, false, "fraud"))
	if out["result"] != "changed" || out["active"] != false {
		t.Fatalf("tenant suspend result %v", out)
	}
	denied("tenant suspended, store 1", get(f.store))
	denied("tenant suspended, store 2", get(f.otherStore))
	p.must(p.setTenant(f.tenant, true, nil))
	if get(f.store) != 200 {
		t.Fatal("tenant resume did not restore the serving store")
	}
	denied("tenant resumed but the store stays suspended", get(f.otherStore))
	st := p.must(p.json(`SELECT control.platform_status(NULL,$1)::text`, f.otherStore))
	if fmt.Sprint(st["stores"]) != fmt.Sprintf("[map[active:false serving:false store_id:%s]]", f.otherStore) || st["tenant_active"] != true {
		t.Fatalf("status of the suspended store: %v", st)
	}
	p.must(p.setStore(f.otherStore, true, nil))
	if get(f.otherStore) != 200 {
		t.Fatal("final resume did not restore the sibling")
	}
	want := "store_suspend/true,store_resume/true,store_suspend/true,tenant_suspend/true,tenant_resume/true,store_resume/true"
	if got := strings.Join(poAudits(t, b.owner, f.tenant), ","); got != want {
		t.Fatalf("operator audit = %s, want %s", got, want)
	}
}

// OP06: re-applying the target state is a no-op that is still audited; bad input and unknown targets refuse.
func TestPlatformOperatorOP06IdempotentAndInputBoundaries(t *testing.T) {
	s := spSetup(t)
	f, b := s.f, s.f.base
	p := poSetup(t, b)
	if out := p.must(p.setStore(f.store, false, "other")); out["result"] != "changed" {
		t.Fatal(out)
	}
	if out := p.must(p.setStore(f.store, false, "other")); out["result"] != "unchanged" || out["changed"] != false {
		t.Fatalf("repeat suspend: %v", out)
	}
	if got := strings.Join(poAudits(t, b.owner, f.tenant), ","); got != "store_suspend/true,store_suspend/false" {
		t.Fatalf("audit after repeat suspend = %s, want 2 rows", got)
	}
	var db, op, ticket string
	if err := b.owner.QueryRow(context.Background(), `SELECT db_user,operator,ticket FROM control.operator_audit WHERE tenant_id=$1 ORDER BY occurred_at,id LIMIT 1`, f.tenant).Scan(&db, &op, &ticket); err != nil ||
		!strings.HasPrefix(db, "mi_test_") || op != "op.test" || ticket != "TICKET-1" {
		t.Fatalf("audit identity db_user=%q operator=%q ticket=%q err=%v (db_user must be the login, not a parameter)", db, op, ticket, err)
	}
	code := func(err error) string { return sqlState(err) }
	bad := []struct {
		name, sql string
		args      []any
		want      string
	}{
		{"suspend without reason", `SELECT control.set_store_active($1,false,'op.test','T-1',NULL)`, []any{f.store}, "PT400"},
		{"suspend with unknown reason", `SELECT control.set_store_active($1,false,'op.test','T-1','because')`, []any{f.store}, "PT400"},
		{"resume with a reason", `SELECT control.set_store_active($1,true,'op.test','T-1','fraud')`, []any{f.store}, "PT400"},
		{"operator uppercase", `SELECT control.set_store_active($1,false,'Op','T-1','fraud')`, []any{f.store}, "PT400"},
		{"operator too long", `SELECT control.set_store_active($1,false,$2,'T-1','fraud')`, []any{f.store, strings.Repeat("a", 41)}, "PT400"},
		{"ticket blank", `SELECT control.set_store_active($1,false,'op','   ','fraud')`, []any{f.store}, "PT400"},
		{"ticket control char", `SELECT control.set_store_active($1,false,'op',E'T\n1','fraud')`, []any{f.store}, "PT400"},
		{"ticket too long", `SELECT control.set_store_active($1,false,'op',$2,'fraud')`, []any{f.store, strings.Repeat("t", 81)}, "PT400"},
		{"unknown store", `SELECT control.set_store_active($1,false,'op','T-1','fraud')`, []any{randomUUID()}, "PT404"},
		{"unknown tenant", `SELECT control.set_tenant_active($1,false,'op','T-1','fraud')`, []any{randomUUID()}, "PT404"},
		{"status both ids", `SELECT control.platform_status($1,$2)`, []any{f.tenant, f.store}, "PT400"},
		{"status no id", `SELECT control.platform_status(NULL,NULL)`, nil, "PT400"},
		{"audit limit 0", `SELECT * FROM control.read_operator_audit(NULL,0)`, nil, "PT400"},
		{"audit limit 501", `SELECT * FROM control.read_operator_audit(NULL,501)`, nil, "PT400"},
	}
	for _, c := range bad {
		if _, err := p.op.Exec(context.Background(), c.sql, c.args...); code(err) != c.want {
			t.Errorf("%s: SQLSTATE %q (err %v), want %s", c.name, code(err), err, c.want)
		}
	}
	// none of the refused calls changed state or wrote an audit row
	if got := strings.Join(poAudits(t, b.owner, f.tenant), ","); got != "store_suspend/true,store_suspend/false" {
		t.Fatalf("refused calls left audit rows: %s", got)
	}
	rows, err := p.op.Query(context.Background(), `SELECT operator,action,ticket FROM control.read_operator_audit($1,10) WHERE tenant_id=$2`, time.Now().Add(-time.Hour), f.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := pgx.CollectRows(rows, pgx.RowTo[string]); err != nil || len(got) != 2 {
		t.Fatalf("read_operator_audit rows = %v err=%v, want 2", got, err)
	}
}

// OP07: the audit table is append-only for every role, the migration owner included; no direct table access for the operator.
func TestPlatformOperatorOP07AuditAppendOnly(t *testing.T) {
	s := spSetup(t)
	f, b := s.f, s.f.base
	p := poSetup(t, b)
	p.must(p.setStore(f.store, false, "fraud"))
	ctx := context.Background()
	for _, q := range []string{
		`UPDATE control.operator_audit SET ticket='x' WHERE tenant_id=$1`,
		`DELETE FROM control.operator_audit WHERE tenant_id=$1`,
		`TRUNCATE control.operator_audit`,
	} {
		args := []any{f.tenant}
		if strings.HasPrefix(q, "TRUNCATE") {
			args = nil
		}
		if _, err := b.owner.Exec(ctx, q, args...); sqlState(err) != "42501" {
			t.Errorf("owner %q: SQLSTATE %q, want 42501 (trigger)", q, sqlState(err))
		}
		for _, role := range []string{"commerce_platform_writer", "commerce_platform_operator", "commerce_runtime"} {
			tx, err := b.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(ctx, `SET LOCAL ROLE `+role); err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, q, args...)
			_ = tx.Rollback(ctx)
			if sqlState(err) != "42501" {
				t.Errorf("%s %q: SQLSTATE %q, want 42501", role, q, sqlState(err))
			}
		}
	}
	if _, err := p.op.Exec(ctx, `SELECT * FROM control.operator_audit`); sqlState(err) != "42501" {
		t.Fatalf("operator read the audit table directly: %v", err)
	}
	if n := countRows(t, b.owner, `SELECT count(*) FROM control.operator_audit WHERE tenant_id=$1`, f.tenant); n != 1 {
		t.Fatalf("audit rows = %d after refused rewrites, want 1", n)
	}
}

// OP02: the buyer surface stops with the store and comes back with it; the published domain row and a buyer capability
// issued before the suspension survive untouched.
func TestPlatformOperatorOP02BuyerSurface(t *testing.T) {
	s := spSetup(t)
	f, b := s.f, s.f.base
	p := poSetup(t, b)
	a := openBuyerTestPools(t, b)
	ctx := context.Background()
	origin := spOrigin()
	if _, err := s.bind(f.store, origin, "po proof", 80*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := s.set(f.token, f.store, true, 0); err != nil {
		t.Fatal(err)
	}
	s.mustResolve(origin, f.store)
	hash := make([]byte, 32)
	copy(hash, randomBytes(32))
	issue := func(h []byte) error {
		_, err := a.issuer.Exec(ctx, `SELECT * FROM buyer.issue_capability($1::uuid,$2,3600)`, f.store, h)
		return err
	}
	if err := issue(hash); err != nil {
		t.Fatal(err)
	}
	resolves := func() int {
		return countRows(t, b.owner, `SELECT count(*) FROM buyer.resolve_scope($1,$2::uuid)`, hash, f.store)
	}
	if resolves() != 1 {
		t.Fatal("precondition: the capability resolves")
	}
	domVersion := countRows(t, b.owner, `SELECT version FROM control.storefront_domains WHERE origin=$1`, origin)
	p.must(p.setStore(f.store, false, "legal"))
	s.mustDeny(origin, "store suspended")
	if err := issue(randomBytes(32)); sqlState(err) != "PT401" {
		t.Fatalf("new capability for a suspended store: %v, want PT401", err)
	}
	if resolves() != 0 {
		t.Fatal("an existing buyer capability still resolved for a suspended store")
	}
	p.must(p.setStore(f.store, true, nil))
	s.mustResolve(origin, f.store)
	if resolves() != 1 || issue(randomBytes(32)) != nil {
		t.Fatal("resume did not restore the buyer capability surface")
	}
	if v := countRows(t, b.owner, `SELECT version FROM control.storefront_domains WHERE origin=$1`, origin); v != domVersion {
		t.Fatalf("domain version %d -> %d: suspension touched the domain row", domVersion, v)
	}
	// tenant suspension stops the resolver the same way
	p.must(p.setTenant(f.tenant, false, "other"))
	s.mustDeny(origin, "tenant suspended")
	p.must(p.setTenant(f.tenant, true, nil))
	s.mustResolve(origin, f.store)
}

// OP03 (claims half): the comment is recorded as an ACCEPTED claim, but no automatic private reply (operation, link) is
// planned while the store is suspended, one audited skip store_suspended is written, and resume plans the next reply.
func TestPlatformOperatorOP03ClaimRecordedNoReplyPlanned(t *testing.T) {
	e := mciSetup(t, mciOpts{private: true})
	f := e.h.f
	p := poSetup(t, f)
	t.Cleanup(func() { // the claims harness shares one database: never leave its store suspended for another test
		_, _ = f.owner.Exec(context.Background(), `UPDATE control.stores SET active=true WHERE id=$1`, f.storeA1)
	})
	skips := func() int64 {
		return miCount(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='claim_reply_skipped:store_suspended'`, f.tenantA, f.storeA1)
	}
	s := e.postFB(t, "", "", "A1", mciAt(3*time.Second), nil)
	p.must(p.setStore(f.storeA1, false, "non_payment"))
	e.apply(t)
	r := e.refresh(t, mciReply{s: s, provider: "facebook", asset: e.pageAsset})
	if r.intake.State != "APPLIED" || r.ev.outcome != "ACCEPTED" || r.bundleID == "" {
		t.Fatalf("the claim itself must still be recorded ACCEPTED/APPLIED: %+v", r.intake)
	}
	if e.opCount(t, s.comment) != 0 || miCount(t, f.owner, `SELECT count(*) FROM claims.links WHERE bundle_id=$1`, r.bundleID) != 0 {
		t.Fatal("a suspended store planned an automatic reply or issued a link")
	}
	if n := skips(); n != 1 {
		t.Fatalf("claim_reply_skipped:store_suspended audit rows = %d, want 1", n)
	}
	// OP05: after resume the next claim plans its reply again.
	p.must(p.setStore(f.storeA1, true, nil))
	s2 := e.postFB(t, "", "", "A1", mciAt(4*time.Second), nil)
	e.apply(t)
	if e.opCount(t, s2.comment) != 1 {
		t.Fatal("after resume the automatic reply was not planned")
	}
}

// OP03 (mail half): notify.claim_batch claims nothing for a suspended store (buyer and merchant mail rows stay PENDING,
// and do not starve another store), and claims them again after resume. The worker login is a real expiry-worker login.
func TestPlatformOperatorOP03MailNotClaimedForSuspendedStore(t *testing.T) {
	b := fixture(t)
	p := poSetup(t, b)
	worker := miPool(t, b, "commerce_expiry_worker")
	ctx := context.Background()
	tenant, sus, ok := randomUUID(), randomUUID(), randomUUID()
	mustExec(t, b.owner, `INSERT INTO control.tenants(id,name) VALUES($1,'po-mail-tenant')`, tenant)
	mustExec(t, b.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'po-sus','USD'),($1,$3,'po-ok','USD')`, tenant, sus, ok)
	t.Cleanup(func() {
		_, _ = b.owner.Exec(ctx, `DELETE FROM notify.outbox WHERE tenant_id=$1`, tenant)
		_, _ = b.owner.Exec(ctx, `DELETE FROM control.stores WHERE tenant_id=$1`, tenant)
		_, _ = b.owner.Exec(ctx, `DELETE FROM control.tenants WHERE id=$1`, tenant)
	})
	// Oldest rows first: the suspended store's rows would fill the claim window if the predicate sat in the loop.
	susBuyer, susMerchant, okBuyer := randomUUID(), randomUUID(), randomUUID()
	for _, r := range []struct{ store, order, kind string }{{sus, susBuyer, "placed"}, {sus, susMerchant, "merchant_new"}, {ok, okBuyer, "placed"}} {
		mustExec(t, b.owner, `INSERT INTO notify.outbox(tenant_id,store_id,order_id,kind) VALUES($1,$2,$3,$4)`, tenant, r.store, r.order, r.kind)
		time.Sleep(5 * time.Millisecond)
	}
	state := func(order string) string {
		var s string
		if err := b.owner.QueryRow(ctx, `SELECT state||':'||coalesce(skip_reason,'') FROM notify.outbox WHERE order_id=$1`, order).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	claim := func() {
		t.Helper()
		var raw []byte
		if err := worker.QueryRow(ctx, `SELECT notify.claim_batch(1,100000,1000)`).Scan(&raw); err != nil {
			t.Fatal(err)
		}
	}
	p.must(p.setStore(sus, false, "non_payment"))
	claim()
	// The serving store's row is considered (no order behind it -> SKIPPED no_order) although the suspended rows are older.
	if got := state(okBuyer); got != "SKIPPED:no_order" {
		t.Fatalf("serving store's row = %s, want SKIPPED:no_order (starved by the suspended store?)", got)
	}
	if state(susBuyer) != "PENDING:" || state(susMerchant) != "PENDING:" {
		t.Fatalf("suspended store's mail rows = %s / %s, want PENDING (untouched)", state(susBuyer), state(susMerchant))
	}
	p.must(p.setStore(sus, true, nil))
	claim()
	claim()
	if got := state(susBuyer); got != "SKIPPED:no_order" {
		t.Fatalf("after resume the buyer row = %s, want it considered again (SKIPPED:no_order)", got)
	}
	if got := state(susMerchant); got != "SKIPPED:no_owner" {
		t.Fatalf("after resume the merchant row = %s, want it considered again (SKIPPED:no_owner)", got)
	}
}

// OP09 (process level): cmd/platform-admin against the real operator login: one JSON line on stdout, one fixed code on
// stderr, nothing of the DSN or a driver message anywhere, and the audit shows the operator, ticket and session user.
func TestPlatformOperatorOP09CLIProcess(t *testing.T) {
	s := spSetup(t)
	f, b := s.f, s.f.base
	opURL := miRole(t, b, "commerce_platform_operator")
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "platform-admin")
	build := exec.Command("go", "build", "-o", bin, "./cmd/platform-admin")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build cmd/platform-admin: %v\n%s", err, out)
	}
	cfg, _ := pgxpool.ParseConfig(opURL)
	secret := cfg.ConnConfig.Password
	run := func(dsn string, args ...string) (int, string, string) {
		cmd := exec.Command(bin, args...)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
		if dsn != "" {
			cmd.Env = append(cmd.Env, "COMMERCE_PLATFORM_OPERATOR_DATABASE_URL="+dsn)
		}
		var so, se bytes.Buffer
		cmd.Stdout, cmd.Stderr = &so, &se
		code := 0
		var ee *exec.ExitError
		if err := cmd.Run(); errors.As(err, &ee) {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
		if all := so.String() + se.String(); strings.Contains(all, secret) || strings.Contains(all, "postgres://") {
			t.Fatalf("%v leaked the DSN/password: %q", args, all)
		}
		return code, so.String(), se.String()
	}
	ok := func(args ...string) map[string]any {
		t.Helper()
		code, so, se := run(opURL, args...)
		var out map[string]any
		if code != 0 || se != "" || strings.Count(so, "\n") != 1 || json.Unmarshal([]byte(so), &out) != nil {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", args, code, so, se)
		}
		return out
	}
	fail := func(want, dsn string, args ...string) {
		t.Helper()
		if code, so, se := run(dsn, args...); code != 1 || so != "" || strings.TrimSpace(se) != want {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q, want exit 1 and %q", args, code, so, se, want)
		}
	}
	id := []string{"--operator", "ops.alice", "--ticket", "TICKET-9"}
	if out := ok(append([]string{"store-suspend", "--store", f.store, "--reason", "fraud"}, id...)...); out["result"] != "changed" || out["active"] != false {
		t.Fatalf("store-suspend: %v", out)
	}
	if out := ok(append([]string{"store-suspend", "--store", f.store, "--reason", "fraud"}, id...)...); out["result"] != "unchanged" {
		t.Fatalf("repeat store-suspend: %v", out)
	}
	if code, _ := spJSON(t, s.h, "GET", "/v1/admin/stores/"+f.store+"/storefront", f.token, ""); code == 200 {
		t.Fatal("the CLI suspension did not stop the merchant API")
	}
	st := ok("status", "--store", f.store)
	if fmt.Sprint(st["stores"]) != fmt.Sprintf("[map[active:false serving:false store_id:%s]]", f.store) {
		t.Fatalf("status: %v", st)
	}
	if out := ok(append([]string{"store-resume", "--store", f.store}, id...)...); out["result"] != "changed" || out["active"] != true {
		t.Fatalf("store-resume: %v", out)
	}
	if out := ok(append([]string{"tenant-suspend", "--tenant", f.tenant, "--reason", "legal"}, id...)...); out["result"] != "changed" {
		t.Fatalf("tenant-suspend: %v", out)
	}
	if out := ok(append([]string{"tenant-resume", "--tenant", f.tenant}, id...)...); out["result"] != "changed" {
		t.Fatalf("tenant-resume: %v", out)
	}
	code, so, se := run(opURL, "audit", "--since", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), "--limit", "500")
	var rows []map[string]any
	if code != 0 || se != "" || json.Unmarshal([]byte(so), &rows) != nil {
		t.Fatalf("audit: exit=%d stdout=%q stderr=%q", code, so, se)
	}
	mine := 0
	for _, r := range rows {
		if r["tenant_id"] == f.tenant {
			mine++
			if r["operator"] != "ops.alice" || r["ticket"] != "TICKET-9" || !strings.HasPrefix(fmt.Sprint(r["db_user"]), "mi_test_") {
				t.Fatalf("audit row identity: %v", r)
			}
		}
	}
	if mine != 5 {
		t.Fatalf("audit rows for the tenant = %d, want 5", mine)
	}
	fail("platform_admin_not_found", opURL, append([]string{"store-suspend", "--store", randomUUID(), "--reason", "fraud"}, id...)...)
	fail("platform_admin_not_found", opURL, "status", "--tenant", randomUUID())
	fail("platform_admin_denied", miRole(t, b, "commerce_runtime"), append([]string{"store-suspend", "--store", f.store, "--reason", "fraud"}, id...)...)
	fail("platform_admin_usage", opURL, "store-suspend", "--store", f.store, "--operator", "ops.alice", "--ticket", "T")
	fail("platform_admin_config", "", "status", "--store", f.store)
	if code, _, se := run("postgres://nobody@127.0.0.1:1/none?connect_timeout=2", "status", "--store", f.store); code != 1 || !strings.HasPrefix(se, "platform_admin_") || strings.Count(se, "\n") != 1 {
		t.Fatalf("unreachable database: exit=%d stderr=%q", code, se)
	}
}
