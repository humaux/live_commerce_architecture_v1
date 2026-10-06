package foundation_test

// R3 unit OPS-02B REAL_PG gate (docs/delivery/units/ops-02b-support-grant.md; migration 0153). Written from the brief and the
// integrator rulings (default-deny, read-only, audited, operator identity from the database login):
//
//	SG01 a live grant lets the support principal read; audit rows in both audit tables   SG05 suspended store / tenant stops support too
//	SG02 out-of-pack permissions and every write are refused (SQL pack + read-only tx)    SG06 hours / permission / duplicate limits (SQL + CHECK)
//	SG03 expiry is judged per request, no sweeper                                         SG07 regular principals are untouched; support never layers on a member
//	SG04 revoke is immediate and idempotent                                               SG08 only the operator login runs the definers; tables are unreachable
//
// Evidence class: REAL_PG, no external service. The CLI half (flag validation) lives in cmd/platform-admin/main_test.go.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/platform"
)

type sgrFix struct {
	t                         *testing.T
	b                         *testFixture
	op                        *pgxpool.Pool // login holding commerce_platform_operator
	tenant, store, store2     string
	tenantB, storeB           string
	member, support, supportB string
	memberTok, supportTok     string
	supportBTok               string
}

func sgrSetup(t *testing.T) *sgrFix {
	t.Helper()
	b := fixture(t)
	ctx := context.Background()
	s := &sgrFix{t: t, b: b, op: miPool(t, b, "commerce_platform_operator"),
		tenant: randomUUID(), store: randomUUID(), store2: randomUUID(), tenantB: randomUUID(), storeB: randomUUID(),
		member: randomUUID(), support: randomUUID(), supportB: randomUUID(),
		memberTok: randomToken(), supportTok: randomToken(), supportBTok: randomToken()}
	tx, err := b.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO control.tenants(id,name) VALUES($1,'sg-tenant'),($2,'sg-tenant-b')`, []any{s.tenant, s.tenantB}},
		{`INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'sg-store','USD'),($1,$3,'sg-store-2','USD'),($4,$5,'sg-store-b','USD')`,
			[]any{s.tenant, s.store, s.store2, s.tenantB, s.storeB}},
		{`INSERT INTO identity.principals(id) VALUES($1),($2),($3)`, []any{s.member, s.support, s.supportB}},
		{`INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, []any{s.tenant, s.member}},
		{`INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES
			($1,$2,$3,'store:read'),($1,$2,$3,'orders:read'),($1,$2,$3,'integration:manage'),($1,$2,$3,'customers:read'),($1,$2,$3,'payments:refund')`,
			[]any{s.tenant, s.store, s.member}},
	} {
		if _, err := tx.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("setup %q: %v", q.sql, err)
		}
	}
	for tok, p := range map[string]string{s.memberTok: s.member, s.supportTok: s.support, s.supportBTok: s.supportB} {
		if err := insertSession(ctx, tx, tok, p, "merchant", time.Now().Add(time.Hour), nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // FK order; operator_audit is append-only and stays (like the OPS-01B tests)
		ids := []string{s.member, s.support, s.supportB}
		ts := []string{s.tenant, s.tenantB}
		for _, q := range []string{
			`DELETE FROM identity.support_grants WHERE tenant_id=ANY($1::uuid[])`,
			`DELETE FROM ops.audit_events WHERE tenant_id=ANY($1::uuid[])`,
			`DELETE FROM identity.store_grants WHERE tenant_id=ANY($1::uuid[])`,
		} {
			_, _ = b.owner.Exec(ctx, q, ts)
		}
		_, _ = b.owner.Exec(ctx, `DELETE FROM identity.sessions WHERE principal_id=ANY($1::uuid[])`, ids)
		_, _ = b.owner.Exec(ctx, `DELETE FROM identity.memberships WHERE tenant_id=ANY($1::uuid[])`, ts)
		_, _ = b.owner.Exec(ctx, `DELETE FROM control.stores WHERE tenant_id=ANY($1::uuid[])`, ts)
		_, _ = b.owner.Exec(ctx, `DELETE FROM control.tenants WHERE id=ANY($1::uuid[])`, ts)
		_, _ = b.owner.Exec(ctx, `DELETE FROM identity.principals WHERE id=ANY($1::uuid[])`, ids)
	})
	return s
}

// grant calls identity.grant_support as the operator login.
func (s *sgrFix) grant(store, principal string, hours int, perms any) (map[string]any, error) {
	return s.json(`SELECT identity.grant_support($1,$2,$3,$4,'op.test','TICKET-SG')::text`, store, principal, hours, perms)
}
func (s *sgrFix) revokePair(store, principal string) (map[string]any, error) {
	return s.json(`SELECT identity.revoke_support(NULL,$1,$2,'op.test','TICKET-SG')::text`, store, principal)
}
func (s *sgrFix) json(sql string, args ...any) (map[string]any, error) {
	var raw string
	if err := s.op.QueryRow(context.Background(), sql, args...).Scan(&raw); err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		s.t.Fatal(err)
	}
	return out, nil
}
func (s *sgrFix) must(out map[string]any, err error) map[string]any {
	s.t.Helper()
	if err != nil {
		s.t.Fatalf("operator call: %v", err)
	}
	return out
}

// access runs platform.WithScope on the runtime pool (the real merchant path).
func (s *sgrFix) access(token, store, perm string) error {
	return platform.WithScope(context.Background(), s.b.runtime, token, store, perm, func(pgx.Tx, platform.Scope) error { return nil })
}

func (s *sgrFix) audits(action string) int {
	return countRows(s.t, s.b.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND action=$2`, s.tenant, action)
}
func (s *sgrFix) opAudits(action string) int {
	return countRows(s.t, s.b.owner, `SELECT count(*) FROM control.operator_audit WHERE tenant_id=$1 AND action=$2`, s.tenant, action)
}

// SG01 + SG02 (SQL half): a live default-pack grant opens exactly the six read permissions of that one store, everything else
// stays refused, both audit tables get one row, and the session store list offers the store with role support.
func TestSupportGrantSG01ReadAndSG02PackLimits(t *testing.T) {
	s := sgrSetup(t)
	for _, perm := range []string{"store:read", "orders:read"} { // before the grant: invisible
		if err := s.access(s.supportTok, s.store, perm); !errors.Is(err, platform.ErrScopeNotFound) {
			t.Fatalf("before grant %s: %v, want scope not found", perm, err)
		}
	}
	out := s.must(s.grant(s.store, s.support, 4, nil))
	if out["result"] != "granted" || out["store_id"] != s.store || out["principal_id"] != s.support || out["tenant_id"] != s.tenant {
		t.Fatalf("grant result %v", out)
	}
	for _, perm := range []string{"store:read", "orders:read", "catalog:read", "inventory:read", "live:read", "integration:read"} {
		if err := s.access(s.supportTok, s.store, perm); err != nil {
			t.Errorf("support %s: %v, want access", perm, err)
		}
	}
	// SG02: outside the pack (PII, inbox, refund) and every write permission: forbidden, never ok.
	for _, perm := range []string{"customers:read", "customers:write", "customers:privacy", "inbox:reply", "payments:refund", "orders:write",
		"integration:manage", "fulfillment:write", "pricing:write", "audit:read", "audit:write", "orders:export"} {
		if err := s.access(s.supportTok, s.store, perm); !errors.Is(err, platform.ErrForbidden) {
			t.Errorf("support %s: %v, want forbidden", perm, err)
		}
	}
	// one store only: sibling store and another tenant's store are invisible
	for _, store := range []string{s.store2, s.storeB} {
		if err := s.access(s.supportTok, store, "store:read"); !errors.Is(err, platform.ErrScopeNotFound) {
			t.Errorf("support on foreign store %s: %v, want scope not found", store, err)
		}
	}
	// a different principal with its own session sees nothing
	if err := s.access(s.supportBTok, s.store, "store:read"); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("other principal: %v", err)
	}
	// audit: one row in each table, merchant-visible detail carries operator and ticket
	if s.opAudits("support_grant") != 1 || s.audits("support.granted") != 1 {
		t.Fatalf("audit rows: operator=%d merchant=%d, want 1/1", s.opAudits("support_grant"), s.audits("support.granted"))
	}
	var detail string
	if err := s.b.owner.QueryRow(context.Background(), `SELECT details::text FROM ops.audit_events WHERE tenant_id=$1 AND action='support.granted'`, s.tenant).Scan(&detail); err != nil ||
		!strings.Contains(detail, "TICKET-SG") || !strings.Contains(detail, "op.test") {
		t.Fatalf("merchant audit detail %q err %v", detail, err)
	}
	if n := countRows(t, s.b.owner, `SELECT count(*) FROM control.operator_audit WHERE tenant_id=$1 AND action='support_grant' AND db_user<>'' AND store_id=$2`, s.tenant, s.store); n != 1 {
		t.Fatal("operator audit lacks db_user/store")
	}
	// store list: the support store appears with role support and exactly the pack
	h := httpapi.NewHandler(s.b.runtime, httpapi.Options{SessionStoreList: true}) // the full merchant router
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/admin/stores", nil)
	req.Header.Set("Authorization", "Bearer "+s.supportTok)
	h.ServeHTTP(rec, req)
	var list struct {
		Items []struct {
			ID          string   `json:"id"`
			Role        *string  `json:"role"`
			Permissions []string `json:"permissions"`
		} `json:"items"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list.Items) != 1 || list.Items[0].ID != s.store ||
		list.Items[0].Role == nil || *list.Items[0].Role != "support" || strings.Join(list.Items[0].Permissions, ",") != "catalog:read,integration:read,inventory:read,live:read,orders:read,store:read" {
		t.Fatalf("support store list: %d %s", rec.Code, rec.Body.String())
	}
	// real HTTP route: GET store as support = 200
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/v1/admin/stores/"+s.store, nil)
	req.Header.Set("Authorization", "Bearer "+s.supportTok)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET store as support: %d %s", rec.Code, rec.Body.String())
	}
	// Through the real router. The order list is served by definers with their own inline membership fence (0027/0063/0110, not
	// resolve_access), so a support principal is refused there by construction: pinned as a known default-deny limit, a conscious
	// follow-up (not a leak) if support order reads are wanted. A regular member reads the same list (positive control).
	get := func(token, method, path string) int {
		rec := httptest.NewRecorder()
		var body *strings.Reader
		if method != "GET" {
			body = strings.NewReader("{}")
		}
		var req *http.Request
		if body == nil {
			req = httptest.NewRequest(method, path, nil)
		} else {
			req = httptest.NewRequest(method, path, body)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := get(s.memberTok, "GET", "/v1/admin/stores/"+s.store+"/orders?view=v2&limit=10"); c != 200 {
		t.Fatalf("member order list: %d", c)
	}
	if c := get(s.supportTok, "GET", "/v1/admin/stores/"+s.store+"/orders?view=v2&limit=10"); c < 400 {
		t.Fatalf("support order list answered 2xx: the inline-fence definers were widened, update the contract and this pin")
	}
	for _, path := range []string{"/orders/export", "/orders/pick-list"} {
		if c := get(s.supportTok, "POST", "/v1/admin/stores/"+s.store+path); c < 400 {
			t.Fatalf("support POST %s: %d, want a refusal", path, c)
		}
	}
	// the audit-events route needs audit:read, which is outside the pack
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/v1/admin/stores/"+s.store+"/audit-events", nil)
	req.Header.Set("Authorization", "Bearer "+s.supportTok)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("support audit-events: %d, want 403", rec.Code)
	}
}

// SG02 (write half): inside a support scope the transaction is read-only, so no definer or statement can write; the same
// statement succeeds for the regular member (positive control). Use is audited at most once per 15 minutes.
func TestSupportGrantSG02ReadOnlyTransactionAndUseAudit(t *testing.T) {
	s := sgrSetup(t)
	s.must(s.grant(s.store, s.support, 4, nil))
	write := func(token, perm string) error {
		return platform.WithScope(context.Background(), s.b.runtime, token, s.store, perm, func(tx pgx.Tx, sc platform.Scope) error {
			_, err := tx.Exec(context.Background(), `INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES($1,$2,$3,'sg.write_probe')`, sc.TenantID, sc.StoreID, sc.PrincipalID)
			return err
		})
	}
	if err := write(s.memberTok, "orders:read"); err != nil {
		t.Fatalf("regular member write (positive control): %v", err)
	}
	err := write(s.supportTok, "orders:read")
	if !errors.Is(err, platform.ErrSupportReadOnly) || !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("support write: %v, want ErrSupportReadOnly (wrapping ErrForbidden)", err)
	}
	// the scope code cannot be talked out of read-only mode from inside the transaction
	escape := platform.WithScope(context.Background(), s.b.runtime, s.supportTok, s.store, "orders:read", func(tx pgx.Tx, _ platform.Scope) error {
		_, err := tx.Exec(context.Background(), `SET LOCAL transaction_read_only = off`)
		return err
	})
	if escape == nil {
		t.Fatal("a support scope could switch its transaction back to read-write")
	}
	if n := s.audits("sg.write_probe"); n != 1 {
		t.Fatalf("write probes persisted = %d, want only the member's", n)
	}
	// use audit: several requests, one support.used row; a regular member never produces one
	for i := 0; i < 3; i++ {
		if err := s.access(s.supportTok, s.store, "orders:read"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.access(s.memberTok, s.store, "orders:read"); err != nil {
		t.Fatal(err)
	}
	if n := s.audits("support.used"); n != 1 {
		t.Fatalf("support.used rows = %d, want 1 per 15 minutes", n)
	}
	if n := countRows(t, s.b.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND action='support.used' AND principal_id=$2`, s.tenant, s.support); n != 1 {
		t.Fatal("support.used not attributed to the support principal")
	}
	// older than the window: the next use writes a new row
	mustExec(t, s.b.owner, `UPDATE ops.audit_events SET created_at=created_at-interval '16 minutes' WHERE tenant_id=$1 AND action='support.used'`, s.tenant)
	if err := s.access(s.supportTok, s.store, "orders:read"); err != nil || s.audits("support.used") != 2 {
		t.Fatalf("second window: err=%v rows=%d", err, s.audits("support.used"))
	}
}

// SG03: expiry is judged by clock_timestamp() on the next request; nothing sweeps.
func TestSupportGrantSG03ExpiryNeedsNoSweeper(t *testing.T) {
	s := sgrSetup(t)
	s.must(s.grant(s.store, s.support, 1, nil))
	if err := s.access(s.supportTok, s.store, "orders:read"); err != nil {
		t.Fatal(err)
	}
	// age the grant in place (owner write; the CHECKs still hold: expires_at > granted_at)
	mustExec(t, s.b.owner, `UPDATE identity.support_grants SET granted_at=clock_timestamp()-interval '3 hours', expires_at=clock_timestamp()-interval '1 second' WHERE store_id=$1 AND principal_id=$2`, s.store, s.support)
	if err := s.access(s.supportTok, s.store, "orders:read"); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("expired grant: %v, want scope not found", err)
	}
	if n := countRows(t, s.b.owner, `SELECT count(*) FROM identity.support_grants WHERE store_id=$1 AND revoked_at IS NULL`, s.store); n != 1 {
		t.Fatal("test premise: the row is still unrevoked (nothing swept it)")
	}
	list := s.must(s.json(`SELECT jsonb_build_object('rows',identity.list_support_grants($1))::text`, s.store))
	if !strings.Contains(strings.ReplaceAll(toJSON(t, list), " ", ""), `"status":"expired"`) {
		t.Fatalf("support-list status: %v", list)
	}
	// re-granting after expiry works: the expired never-revoked row is closed at its expiry, one new open row exists
	s.must(s.grant(s.store, s.support, 2, nil))
	if err := s.access(s.supportTok, s.store, "orders:read"); err != nil {
		t.Fatalf("re-grant after expiry: %v", err)
	}
	if n := countRows(t, s.b.owner, `SELECT count(*) FROM identity.support_grants WHERE store_id=$1 AND principal_id=$2 AND revoked_at IS NULL`, s.store, s.support); n != 1 {
		t.Fatalf("open rows after re-grant = %d, want 1", n)
	}
}

func toJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// SG04: revoke is immediate, idempotent (second call = unchanged, still operator-audited), and audited in both tables once.
func TestSupportGrantSG04RevokeImmediateAndIdempotent(t *testing.T) {
	s := sgrSetup(t)
	g := s.must(s.grant(s.store, s.support, 4, nil))
	if err := s.access(s.supportTok, s.store, "orders:read"); err != nil {
		t.Fatal(err)
	}
	out := s.must(s.revokePair(s.store, s.support))
	if out["result"] != "revoked" || out["changed"] != true || out["grant_id"] != g["grant_id"] {
		t.Fatalf("revoke result %v", out)
	}
	if err := s.access(s.supportTok, s.store, "orders:read"); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("after revoke: %v, want scope not found", err)
	}
	if s.audits("support.revoked") != 1 || s.opAudits("support_revoke") != 1 {
		t.Fatal("revoke audit rows missing")
	}
	out = s.must(s.revokePair(s.store, s.support))
	if out["result"] != "unchanged" || out["changed"] != false || s.audits("support.revoked") != 1 || s.opAudits("support_revoke") != 2 {
		t.Fatalf("second revoke: %v merchant=%d operator=%d", out, s.audits("support.revoked"), s.opAudits("support_revoke"))
	}
	// by grant id; unknown id is not found; both forms at once is refused
	g2 := s.must(s.grant(s.store, s.support, 4, nil))
	out = s.must(s.json(`SELECT identity.revoke_support($1,NULL,NULL,'op.test','TICKET-SG')::text`, g2["grant_id"]))
	if out["result"] != "revoked" {
		t.Fatalf("revoke by id: %v", out)
	}
	if _, err := s.json(`SELECT identity.revoke_support($1,NULL,NULL,'op.test','TICKET-SG')::text`, randomUUID()); sqlState(err) != "PT404" {
		t.Fatalf("unknown grant id: %v", err)
	}
	if _, err := s.json(`SELECT identity.revoke_support($1,$2,$3,'op.test','TICKET-SG')::text`, g2["grant_id"], s.store, s.support); sqlState(err) != "PT400" {
		t.Fatalf("ambiguous revoke: %v", err)
	}
	// a revoked grant can be replaced by a fresh one (history stays)
	s.must(s.grant(s.store, s.support, 1, nil))
	if n := countRows(t, s.b.owner, `SELECT count(*) FROM identity.support_grants WHERE store_id=$1 AND principal_id=$2`, s.store, s.support); n != 3 {
		t.Fatalf("grant history rows = %d, want 3", n)
	}
}

// SG05: suspending the store or the tenant stops support access immediately (same predicates as the merchant), resume restores it;
// a grant for a suspended store is refused.
func TestSupportGrantSG05SuspensionStopsSupportToo(t *testing.T) {
	s := sgrSetup(t)
	s.must(s.grant(s.store, s.support, 4, nil))
	ok := func(label string, want error) {
		t.Helper()
		if err := s.access(s.supportTok, s.store, "orders:read"); !errors.Is(err, want) && !(want == nil && err == nil) {
			t.Fatalf("%s: %v, want %v", label, err, want)
		}
	}
	ok("before", nil)
	s.must(s.json(`SELECT control.set_store_active($1,false,'op.test','TICKET-SG','fraud')::text`, s.store))
	ok("store suspended", platform.ErrScopeNotFound)
	if _, err := s.grant(s.store, s.supportB, 4, nil); sqlState(err) != "PT409" {
		t.Fatalf("grant on suspended store: %v, want PT409", err)
	}
	s.must(s.json(`SELECT control.set_store_active($1,true,'op.test','TICKET-SG',NULL)::text`, s.store))
	ok("store resumed", nil)
	s.must(s.json(`SELECT control.set_tenant_active($1,false,'op.test','TICKET-SG','fraud')::text`, s.tenant))
	ok("tenant suspended", platform.ErrScopeNotFound)
	s.must(s.json(`SELECT control.set_tenant_active($1,true,'op.test','TICKET-SG',NULL)::text`, s.tenant))
	ok("tenant resumed", nil)
}

// SG06: limits. SQL refuses 0/73 hours, permissions outside the pack, a missing store:read, bad operator/ticket; the table CHECKs
// hold even for a direct owner insert; one open grant per (store, principal); unknown store/principal.
func TestSupportGrantSG06Limits(t *testing.T) {
	s := sgrSetup(t)
	for name, c := range map[string]struct {
		hours int
		perms any
	}{
		"73 hours":         {73, nil},
		"0 hours":          {0, nil},
		"negative hours":   {-4, nil},
		"customers:read":   {4, []string{"store:read", "customers:read"}},
		"write permission": {4, []string{"store:read", "orders:write"}},
		"empty list":       {4, []string{}},
		"no store:read":    {4, []string{"orders:read"}},
		"empty element":    {4, []string{"store:read", ""}},
	} {
		if _, err := s.grant(s.store, s.support, c.hours, c.perms); sqlState(err) != "PT400" {
			t.Errorf("%s: %v, want PT400", name, err)
		}
	}
	for name, sql := range map[string]string{
		"operator uppercase": `SELECT identity.grant_support($1,$2,4,NULL,'Alice','T-1')::text`,
		"blank ticket":       `SELECT identity.grant_support($1,$2,4,NULL,'op.test','   ')::text`,
		"null operator":      `SELECT identity.grant_support($1,$2,4,NULL,NULL,'T-1')::text`,
	} {
		if _, err := s.json(sql, s.store, s.support); sqlState(err) != "PT400" {
			t.Errorf("%s: %v, want PT400", name, err)
		}
	}
	if _, err := s.grant(randomUUID(), s.support, 4, nil); sqlState(err) != "PT404" {
		t.Errorf("unknown store: %v", err)
	}
	if _, err := s.grant(s.store, randomUUID(), 4, nil); sqlState(err) != "PT404" {
		t.Errorf("unknown principal: %v", err)
	}
	mustExec(t, s.b.owner, `UPDATE identity.principals SET active=false WHERE id=$1`, s.supportB) // owner may; used only as a premise
	if _, err := s.grant(s.store, s.supportB, 4, nil); sqlState(err) != "PT404" {
		t.Errorf("inactive principal: %v", err)
	}
	mustExec(t, s.b.owner, `UPDATE identity.principals SET active=true WHERE id=$1`, s.supportB)
	if n := countRows(t, s.b.owner, `SELECT count(*) FROM identity.support_grants WHERE tenant_id=$1`, s.tenant); n != 0 {
		t.Fatalf("a refused grant left %d rows", n)
	}
	// at the boundary: 72 hours is allowed and expires_at - granted_at is exactly 72 h
	out := s.must(s.grant(s.store, s.support, 72, []string{"store:read", "orders:read", "store:read"}))
	var span time.Duration
	if err := s.b.owner.QueryRow(context.Background(), `SELECT expires_at-granted_at FROM identity.support_grants WHERE id=$1`, out["grant_id"]).Scan(&span); err != nil || span != 72*time.Hour {
		t.Fatalf("72 h grant span %v err %v", span, err)
	}
	if err := s.access(s.supportTok, s.store, "orders:read"); err != nil {
		t.Fatal(err)
	}
	if err := s.access(s.supportTok, s.store, "catalog:read"); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("narrowed pack must not allow catalog:read: %v", err)
	}
	// duplicate open grant
	if _, err := s.grant(s.store, s.support, 4, nil); sqlState(err) != "PT409" {
		t.Fatalf("second open grant: %v, want PT409", err)
	}
	// table CHECKs, direct owner inserts
	row := func(perms string, hours int) error {
		_, err := s.b.owner.Exec(context.Background(), `INSERT INTO identity.support_grants(tenant_id,store_id,principal_id,permissions,granted_at,expires_at,operator,db_user,ticket)
			VALUES($1,$2,$3,$4::text[],clock_timestamp(),clock_timestamp()+make_interval(hours=>$5),'op.test','x','T')`, s.tenant, s.store2, s.supportB, perms, hours)
		return err
	}
	mustExec(t, s.b.owner, `INSERT INTO identity.memberships(tenant_id,principal_id,active) VALUES($1,$2,false)`, s.tenant, s.supportB)
	if sqlState(row(`{store:read,customers:read}`, 4)) != "23514" || sqlState(row(`{store:read,orders:write}`, 4)) != "23514" ||
		sqlState(row(`{orders:read}`, 4)) != "23514" || sqlState(row(`{store:read}`, 73)) != "23514" {
		t.Fatal("support_grants CHECK constraints do not hold")
	}
	if err := row(`{store:read}`, 71); err != nil { // 72 would race the two clock_timestamp() calls of this probe
		t.Fatalf("valid direct insert refused: %v", err)
	}
	if sqlState(row(`{store:read}`, 4)) != "23505" {
		t.Fatal("second open row for (store, principal) must violate the partial unique index")
	}
}

// SG07: nothing changes for regular principals, and support never layers on a member or on lingering regular grants.
func TestSupportGrantSG07RegularPrincipalsUnaffected(t *testing.T) {
	s := sgrSetup(t)
	regular := func() {
		t.Helper()
		for _, perm := range []string{"store:read", "orders:read", "integration:manage", "customers:read", "payments:refund"} {
			if err := s.access(s.memberTok, s.store, perm); err != nil {
				t.Fatalf("member %s: %v", perm, err)
			}
		}
		if err := s.access(s.memberTok, s.store, "orders:export"); !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("member without the grant: %v, want forbidden", err)
		}
		if err := s.access(s.memberTok, s.store2, "store:read"); !errors.Is(err, platform.ErrScopeNotFound) {
			t.Fatalf("member on a store without grants: %v", err)
		}
	}
	regular()
	// a grant for the member's own store is refused (never layered); granting for another principal changes nothing for the member
	if _, err := s.grant(s.store, s.member, 4, nil); sqlState(err) != "PT409" {
		t.Fatalf("grant to a member: %v, want PT409", err)
	}
	s.must(s.grant(s.store, s.support, 4, nil))
	s.must(s.grant(s.store2, s.support, 4, nil))
	regular()
	// the regular session keeps authz_revision > 0; a support scope is the only negative revision
	var sc, msc platform.Scope
	if err := platform.WithScope(context.Background(), s.b.runtime, s.memberTok, s.store, "store:read", func(_ pgx.Tx, x platform.Scope) error { msc = x; return nil }); err != nil || msc.Revision < 1 {
		t.Fatalf("member scope %v err %v", msc, err)
	}
	if err := platform.WithScope(context.Background(), s.b.runtime, s.supportTok, s.store, "store:read", func(_ pgx.Tx, x platform.Scope) error { sc = x; return nil }); err != nil || sc.Revision >= 0 || sc.PrincipalID != s.support || sc.TenantID != s.tenant {
		t.Fatalf("support scope %v err %v", sc, err)
	}
	// lingering regular grants disable the support branch (default-deny): put an inactive-member grant on store2 for the support principal
	mustExec(t, s.b.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'store:read')`, s.tenant, s.store2, s.support)
	if err := s.access(s.supportTok, s.store2, "store:read"); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("support grant over a regular grant row: %v, want scope not found", err)
	}
	if err := s.access(s.supportTok, s.store, "store:read"); err != nil {
		t.Fatalf("support on the other store stays: %v", err)
	}
	// the placeholder membership is inactive, so the pre-0153 scope resolver (active membership join) never admits support
	if n := countRows(t, s.b.owner, `SELECT count(*) FROM identity.memberships WHERE tenant_id=$1 AND principal_id=$2 AND NOT active`, s.tenant, s.support); n != 1 {
		t.Fatal("placeholder membership must be inactive")
	}
	hash := sha256.Sum256([]byte(s.supportTok))
	if n := countRows(t, s.b.runtime, `SELECT count(*) FROM identity.resolve_scope($1,$2,'store:read')`, hash[:], s.store); n != 0 {
		t.Fatal("identity.resolve_scope admitted a support principal")
	}
	// an existing membership is left exactly as it is by a grant to a member of another tenant
	s.must(s.grant(s.storeB, s.member, 4, nil))
	if n := countRows(t, s.b.owner, `SELECT count(*) FROM identity.memberships WHERE principal_id=$1 AND active`, s.member); n != 1 {
		t.Fatal("grant changed the member's own membership")
	}
}

// SG08: the definers belong to the operator login only; no runtime/identity/worker authority runs them, and the grant table is
// unreachable for everyone but the definer owner (and commerce_auth's six columns).
func TestSupportGrantSG08AuthorityMatrix(t *testing.T) {
	s := sgrSetup(t)
	b := s.b
	ctx := context.Background()
	fns := []string{"identity.grant_support(uuid,uuid,integer,text[],text,text)", "identity.revoke_support(uuid,uuid,uuid,text,text)", "identity.list_support_grants(uuid)"}
	for _, fn := range fns {
		var secdef bool
		var owner, cfg, comment string
		var acl []string
		if err := b.owner.QueryRow(ctx, `SELECT p.prosecdef,pg_get_userbyid(p.proowner),coalesce(p.proconfig::text,''),coalesce(obj_description(p.oid,'pg_proc'),''),
			coalesce((SELECT array_agg(a.grantee::regrole::text ORDER BY a.grantee::regrole::text) FROM aclexplode(p.proacl) a WHERE a.privilege_type='EXECUTE'),'{}')
			FROM pg_proc p WHERE p.oid=$1::regprocedure`, fn).Scan(&secdef, &owner, &cfg, &comment, &acl); err != nil {
			t.Fatalf("%s missing: %v", fn, err)
		}
		if !secdef || owner != "commerce_platform_writer" || !strings.Contains(cfg, "search_path=pg_catalog") || comment == "" {
			t.Fatalf("%s: secdef=%v owner=%s config=%s comment=%q", fn, secdef, owner, cfg, comment)
		}
		lcSameSet(t, fn+" EXECUTE grantees", acl, []string{"commerce_platform_operator", "commerce_platform_writer"})
		if over := lcStrings(t, b.owner, `SELECT r.rolname FROM pg_roles r WHERE r.rolname LIKE 'commerce\_%' AND r.rolname NOT IN ('commerce_platform_operator','commerce_platform_writer')
			AND has_function_privilege(r.oid,$1::regprocedure,'EXECUTE')`, fn); len(over) != 0 {
			t.Fatalf("%s is executable by %v", fn, over)
		}
	}
	// resolve_access / list_session_stores keep owner commerce_auth and the same callers (CREATE OR REPLACE kept the ACL)
	for _, fn := range []string{"identity.resolve_access(bytea,uuid,text)", "identity.list_session_stores(bytea)"} {
		var owner string
		var secdef bool
		var stable string
		if err := b.owner.QueryRow(ctx, `SELECT pg_get_userbyid(proowner),prosecdef,provolatile::text FROM pg_proc WHERE oid=$1::regprocedure`, fn).Scan(&owner, &secdef, &stable); err != nil ||
			owner != "commerce_auth" || !secdef || stable != "s" {
			t.Fatalf("%s: owner=%s secdef=%v volatility=%s err=%v", fn, owner, secdef, stable, err)
		}
		if has, err := scalarBool(b.owner, `SELECT has_function_privilege('commerce_platform_operator',$1::regprocedure,'EXECUTE')`, fn); err != nil || has {
			t.Fatalf("operator can run %s (err %v)", fn, err)
		}
	}
	// real logins of other authorities are refused 42501; the operator login runs them (positive control)
	calls := []string{
		`SELECT identity.grant_support($1::uuid,$2::uuid,4,NULL,'op.test','T-1')`,
		`SELECT identity.revoke_support(NULL,$1::uuid,$2::uuid,'op.test','T-1')`,
		`SELECT identity.list_support_grants($1::uuid) WHERE $2::uuid IS NOT NULL`,
	}
	for _, authority := range []string{"commerce_runtime", "commerce_identity", "commerce_auth", "commerce_staff_writer", "commerce_buyer_runtime", "commerce_checkout_runtime",
		"commerce_worker", "commerce_claims_worker", "commerce_expiry_worker", "commerce_storefront_registrar"} {
		pool := miPool(t, b, authority)
		for _, c := range calls {
			if _, err := pool.Exec(ctx, c, s.store, s.support); sqlState(err) != "42501" {
				t.Errorf("%s login ran %q: SQLSTATE %q (err %v), want 42501", authority, c, sqlState(err), err)
			}
		}
	}
	if n := countRows(t, b.owner, `SELECT count(*) FROM identity.support_grants WHERE tenant_id=$1`, s.tenant); n != 0 {
		t.Fatalf("a refused login created %d grants", n)
	}
	s.must(s.grant(s.store, s.support, 4, nil))
	if n := countRows(t, s.op, `SELECT jsonb_array_length(identity.list_support_grants($1))`, s.store); n != 1 {
		t.Fatal("operator list")
	}
	// table reachability: runtime and the operator have no direct privilege on support_grants; the operator cannot become the writer
	for _, who := range []string{"commerce_runtime", "commerce_platform_operator", "commerce_identity", "commerce_staff_writer", "commerce_buyer_runtime"} {
		if has, err := scalarBool(b.owner, `SELECT has_table_privilege($1,'identity.support_grants','SELECT,INSERT,UPDATE,DELETE') OR has_any_column_privilege($1,'identity.support_grants','SELECT,INSERT,UPDATE')`, who); err != nil || has {
			t.Errorf("%s has direct access to identity.support_grants (err %v)", who, err)
		}
	}
	acl := lcStrings(t, b.owner, `SELECT a.grantee::regrole::text||':'||a.privilege_type FROM pg_class c, aclexplode(c.relacl) a WHERE c.oid='identity.support_grants'::regclass AND a.grantee<>c.relowner ORDER BY 1`)
	lcSameSet(t, "support_grants table ACL", acl, []string{"commerce_platform_writer:INSERT", "commerce_platform_writer:SELECT"})
	cols := lcStrings(t, b.owner, `SELECT a.attname||':'||x.privilege_type||':'||x.grantee::regrole::text FROM pg_attribute a, LATERAL aclexplode(a.attacl) x
		WHERE a.attrelid='identity.support_grants'::regclass AND a.attnum>0 ORDER BY 1`)
	lcSameSet(t, "support_grants column ACL", cols, []string{
		"revoked_at:UPDATE:commerce_platform_writer", "tenant_id:SELECT:commerce_auth", "store_id:SELECT:commerce_auth", "principal_id:SELECT:commerce_auth",
		"permissions:SELECT:commerce_auth", "expires_at:SELECT:commerce_auth", "revoked_at:SELECT:commerce_auth"})
	if _, err := s.op.Exec(ctx, `SET ROLE commerce_platform_writer`); sqlState(err) != "42501" {
		t.Errorf("operator login SET ROLE commerce_platform_writer: %v, want 42501", err)
	}
	// the operator-audit CHECK now admits the two support actions and still refuses others
	if _, err := b.owner.Exec(ctx, `INSERT INTO control.operator_audit(operator,db_user,action,tenant_id,store_id,ticket) VALUES('op.test','x','support_bogus',$1,$2,'T')`, s.tenant, s.store); sqlState(err) != "23514" {
		t.Errorf("unknown operator action: %v, want 23514", err)
	}
	if _, err := b.owner.Exec(ctx, `INSERT INTO control.operator_audit(operator,db_user,action,tenant_id,store_id,ticket) VALUES('op.test','x','support_grant',$1,NULL,'T')`, s.tenant); sqlState(err) != "23514" {
		t.Errorf("support action without a store: %v, want 23514", err)
	}
	if _, err := b.owner.Exec(ctx, `UPDATE control.operator_audit SET ticket='x' WHERE tenant_id=$1`, s.tenant); sqlState(err) != "42501" {
		t.Errorf("operator audit must stay append-only: %v", err)
	}
}

func scalarBool(pool *pgxpool.Pool, sql string, args ...any) (bool, error) {
	var v bool
	err := pool.QueryRow(context.Background(), sql, args...).Scan(&v)
	return v, err
}
