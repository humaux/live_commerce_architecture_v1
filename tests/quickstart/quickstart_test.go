// Purpose: verify OC-05 new/old-store state, derived steps, idempotent dismissal and scope refusal through the real admin handler.
// Depends on: internal/httpapi, internal/platform, migrations, pgx; CI's disposable lc_foundation_test database.
// Used by: GitHub Gates shard:unit. Invariants: no mock SQL, no client tenant authority, no real buyer data; missing PG is NOT_RUN.
package quickstarttests

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/platform"
	"livecommerce/internal/quickstart"
	"livecommerce/migrations"
)

type state struct {
	Completed   bool       `json:"completed"`
	CompletedAt *time.Time `json:"completed_at"`
	Steps       struct {
		StoreName    bool `json:"store_name"`
		Shipping     bool `json:"shipping"`
		FirstProduct bool `json:"first_product"`
	} `json:"steps"`
}

type env struct {
	owner, runtime *pgxpool.Pool
	http           http.Handler
}

func random(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}
func id() string {
	b := random(16)
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func setup(t *testing.T) *env {
	t.Helper()
	if os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" || os.Getenv("LC_TEST_DATABASE_URL") == "" {
		t.Skip("OC-05 REAL_PG NOT_RUN: disposable database not enabled")
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("LC_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal("invalid disposable database configuration")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Database != "lc_foundation_test" {
		t.Fatal("refusing non-disposable database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	owner, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("open migration pool failed")
	}
	t.Cleanup(owner.Close)
	if err = migrations.Apply(ctx, owner); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	role, secret := "oc05_"+hex.EncodeToString(random(10)), hex.EncodeToString(random(32))
	if _, err = owner.Exec(ctx, `CREATE ROLE `+pgx.Identifier{role}.Sanitize()+` LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE IN ROLE commerce_runtime PASSWORD '`+secret+`'`); err != nil {
		t.Fatal("create disposable runtime role failed")
	}
	t.Cleanup(func() {
		if _, err := owner.Exec(context.Background(), `DROP ROLE `+pgx.Identifier{role}.Sanitize()); err != nil {
			t.Error("drop disposable runtime role failed")
		}
	})
	u, _ := url.Parse(os.Getenv("LC_TEST_DATABASE_URL"))
	u.User = url.UserPassword(role, secret)
	runtime, err := platform.OpenPool(ctx, u.String())
	if err != nil {
		t.Fatal("open runtime pool failed")
	}
	t.Cleanup(runtime.Close)
	return &env{owner: owner, runtime: runtime, http: httpapi.NewHandler(runtime)}
}

func exec(t *testing.T, p *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := p.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("synthetic fixture: %v", err)
	}
}
func store(t *testing.T, e *env, tenant string) string {
	t.Helper()
	s := id()
	exec(t, e.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'OC05 synthetic store','USD')`, tenant, s)
	return s
}
func tenant(t *testing.T, e *env) string {
	t.Helper()
	v := id()
	exec(t, e.owner, `INSERT INTO control.tenants(id,name) VALUES($1,'OC05 synthetic tenant')`, v)
	return v
}
func staff(t *testing.T, e *env, tenant, store string, manage bool) string {
	t.Helper()
	p, tok := id(), hex.EncodeToString(random(32))
	exec(t, e.owner, `INSERT INTO identity.principals(id) VALUES($1)`, p)
	exec(t, e.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, tenant, p)
	perms := []string{"store:read"}
	if manage {
		perms = append(perms, "integration:manage")
	}
	for _, perm := range perms {
		exec(t, e.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, tenant, store, p, perm)
	}
	h := sha256.Sum256([]byte(tok))
	exec(t, e.owner, `INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at) VALUES($1,$2,$3,'merchant',$4)`, id(), h[:], p, time.Now().Add(time.Hour))
	return tok
}
func request(e *env, method, store, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/v1/admin/stores/"+store+"/quickstart", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	if method == http.MethodPost {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "oc05-complete-marker")
	}
	w := httptest.NewRecorder()
	e.http.ServeHTTP(w, r)
	return w
}
func read(t *testing.T, e *env, s, tok string) state {
	t.Helper()
	w := request(e, http.MethodGet, s, tok, "")
	if w.Code != 200 {
		t.Fatalf("GET quickstart status=%d want 200 (new server route); body=%s", w.Code, w.Body.String())
	}
	var out state
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestOC05NewStoreDerivedSteps(t *testing.T) {
	e := setup(t)
	tn := tenant(t, e)
	s := store(t, e, tn)
	tok := staff(t, e, tn, s, true)
	got := read(t, e, s, tok)
	if got.Completed || got.CompletedAt != nil || !got.Steps.StoreName || got.Steps.Shipping || got.Steps.FirstProduct {
		t.Fatalf("new store state=%+v, want incomplete/name only", got)
	}
	// Fixtures prepare real domain records; the state is always read through the production handler.
	exec(t, e.owner, `INSERT INTO catalog.products(tenant_id,store_id,id,name) VALUES($1,$2,$3,'OC05 synthetic product')`, tn, s, id())
	if got = read(t, e, s, tok); !got.Steps.FirstProduct || got.Steps.Shipping || got.Completed {
		t.Fatalf("derived product state=%+v", got)
	}
	// A whitespace name is legal in the legacy table, but is not a configured display name.
	exec(t, e.owner, `UPDATE control.stores SET name='   ' WHERE tenant_id=$1 AND id=$2`, tn, s)
	if got = read(t, e, s, tok); got.Steps.StoreName {
		t.Fatal("blank store name counted as done")
	}
}

func TestOC05CompleteIdempotentAndPersistent(t *testing.T) {
	e := setup(t)
	tn := tenant(t, e)
	s := store(t, e, tn)
	tok := staff(t, e, tn, s, true)
	read(t, e, s, tok)
	for i := 0; i < 2; i++ {
		w := request(e, http.MethodPost, s, tok, `{}`)
		if w.Code != 200 {
			t.Fatalf("POST quickstart status=%d want 200; body=%s", w.Code, w.Body.String())
		}
	}
	first := read(t, e, s, tok)
	if !first.Completed || first.CompletedAt == nil {
		t.Fatalf("completion did not persist: %+v", first)
	}
	if w := request(e, http.MethodPost, s, tok, `{}`); w.Code != 200 {
		t.Fatalf("replay status=%d", w.Code)
	}
	if again := read(t, e, s, tok); again.CompletedAt == nil || !again.CompletedAt.Equal(*first.CompletedAt) {
		t.Fatalf("idempotent replay changed timestamp: %+v / %+v", first, again)
	}
	other := store(t, e, tn)
	otherToken := staff(t, e, tn, other, true)
	if read(t, e, other, otherToken).Completed {
		t.Fatal("one store completion affected sibling")
	}
}

func TestOC05CrossTenantAndStoreRefused(t *testing.T) {
	e := setup(t)
	ta, tb := tenant(t, e), tenant(t, e)
	a1, a2, b := store(t, e, ta), store(t, e, ta), store(t, e, tb)
	tok := staff(t, e, ta, a1, true)
	read(t, e, a1, tok)
	for _, s := range []string{a2, b} {
		for _, m := range []string{http.MethodGet, http.MethodPost} {
			w := request(e, m, s, tok, `{}`)
			if w.Code != 404 {
				t.Fatalf("foreign store %s status=%d want non-disclosing 404", m, w.Code)
			}
		}
	}
	for _, s := range []string{a2, b} {
		tn := ta
		if s == b {
			tn = tb
		}
		own := staff(t, e, tn, s, true)
		if read(t, e, s, own).Completed {
			t.Fatal("refused cross-scope write changed marker")
		}
	}
}

func TestOC05ReadOnlyCannotDismissAndNoClientState(t *testing.T) {
	e := setup(t)
	tn := tenant(t, e)
	s := store(t, e, tn)
	reader := staff(t, e, tn, s, false)
	manager := staff(t, e, tn, s, true)
	read(t, e, s, reader)
	if w := request(e, http.MethodPost, s, reader, `{}`); w.Code != 403 {
		t.Fatalf("read-only dismissal status=%d want 403", w.Code)
	}
	for _, body := range []string{`{"tenant_id":"` + id() + `"}`, `{"completed_at":"2020-01-01T00:00:00Z"}`, `{"steps":{"shipping":true}}`} {
		if w := request(e, http.MethodPost, s, manager, body); w.Code != 400 {
			t.Fatalf("client state accepted: status=%d want 400", w.Code)
		}
	}
	if read(t, e, s, manager).Completed {
		t.Fatal("invalid writes changed marker")
	}
}

func TestOC05ShippingDerivedFromCurrentStoreData(t *testing.T) {
	e := setup(t)
	tn := tenant(t, e)
	s := store(t, e, tn)
	tok := staff(t, e, tn, s, true)
	read(t, e, s, tok)
	var principal string
	h := sha256.Sum256([]byte(tok))
	if err := e.owner.QueryRow(context.Background(), `SELECT principal_id::text FROM identity.sessions WHERE token_hash=$1`, h[:]).Scan(&principal); err != nil {
		t.Fatal(err)
	}
	m := id()
	exec(t, e.owner, `INSERT INTO pricing.markets(tenant_id,store_id,id,code,name,currency,principal_id) VALUES($1,$2,$3,'oc05','Synthetic market','USD',$4)`, tn, s, m, principal)
	exec(t, e.owner, `INSERT INTO pricing.policy_versions(tenant_id,store_id,market_id,country,method,version,currency,shipping_mode,shipping_minor,tax_mode,tax_basis,tax_rate_bps,quote_ttl_seconds,enabled,configuration_ref,principal_id) VALUES($1,$2,$3,'US','delivery:oc05',1,'USD','country_flat',0,'none','goods',0,300,true,'oc05-fixture',$4)`, tn, s, m, principal)
	exec(t, e.owner, `INSERT INTO fulfillment.service_versions(tenant_id,store_id,market_id,country,code,version,policy_version,currency,name_hans,name_hant,name_en,delivery_kind,mode,enabled,visible,sort_order,principal_id) VALUES($1,$2,$3,'US','oc05',1,1,'USD','OC05','OC05','OC05','home','MANUAL',false,true,0,$4)`, tn, s, m, principal)
	exec(t, e.owner, `INSERT INTO fulfillment.service_heads(tenant_id,store_id,market_id,country,code,current_version) VALUES($1,$2,$3,'US','oc05',1)`, tn, s, m)
	if read(t, e, s, tok).Steps.Shipping {
		t.Fatal("disabled shipping counted as done")
	}
	exec(t, e.owner, `INSERT INTO fulfillment.service_versions(tenant_id,store_id,market_id,country,code,version,policy_version,currency,name_hans,name_hant,name_en,delivery_kind,mode,enabled,visible,sort_order,principal_id) VALUES($1,$2,$3,'US','oc05',2,1,'USD','OC05','OC05','OC05','home','MANUAL',true,true,0,$4)`, tn, s, m, principal)
	exec(t, e.owner, `UPDATE fulfillment.service_heads SET current_version=2 WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3`, tn, s, m)
	if !read(t, e, s, tok).Steps.Shipping {
		t.Fatal("enabled current shipping not derived")
	}
	other := store(t, e, tn)
	otherTok := staff(t, e, tn, other, true)
	if read(t, e, other, otherTok).Steps.Shipping {
		t.Fatal("sibling shipping leaked into steps")
	}
	exec(t, e.owner, `UPDATE fulfillment.service_heads SET current_version=1 WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3`, tn, s, m)
	if read(t, e, s, tok).Steps.Shipping {
		t.Fatal("historical enabled version counted as current shipping")
	}
}

func TestOC05MigrationBackfillsOnlyExistingStores(t *testing.T) {
	e := setup(t)
	tn := tenant(t, e)
	old := store(t, e, tn)
	tok := staff(t, e, tn, old, true)
	// Recreate only this card's table in a rollback-only owner transaction, then run the entire assigned
	// migration. No committed schema is reset, and normal Apply's checksum ledger is untouched.
	sql, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0205_quickstart_marker.sql"))
	if err != nil {
		t.Fatalf("OC-05 migration unavailable: %v", err)
	}
	tx, err := e.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `DROP TABLE control.quickstart_markers`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), string(sql)); err != nil {
		t.Fatal(err)
	}
	var completed time.Time
	if err = tx.QueryRow(context.Background(), `SELECT completed_at FROM control.quickstart_markers WHERE tenant_id=$1 AND store_id=$2`, tn, old).Scan(&completed); err != nil || completed.IsZero() {
		t.Fatalf("old-store backfill timestamp=%v err=%v", completed, err)
	}
	newStore := id()
	if _, err = tx.Exec(context.Background(), `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'OC05 post-migration','USD')`, tn, newStore); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = tx.QueryRow(context.Background(), `SELECT count(*) FROM control.quickstart_markers WHERE tenant_id=$1 AND store_id=$2`, tn, newStore).Scan(&count); err != nil || count != 0 {
		t.Fatalf("new store precompleted: count=%d err=%v", count, err)
	}
	// Authenticate from the real session, then use the normal service as commerce_runtime inside the upgrade transaction.
	// This keeps the recreated schema rollback-only while still exercising the production read and its RLS/permission fences.
	if _, err = tx.Exec(context.Background(), `SET LOCAL ROLE commerce_runtime`); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte(tok))
	var access string
	var scope platform.Scope
	scope.StoreID = old
	if err = tx.QueryRow(context.Background(), `SELECT access_status,tenant_id::text,principal_id::text,authz_revision FROM identity.resolve_access($1,$2,'store:read')`, h[:], old).Scan(&access, &scope.TenantID, &scope.PrincipalID, &scope.Revision); err != nil || access != "ok" {
		t.Fatalf("upgrade scope access=%s err=%v", access, err)
	}
	if _, err = tx.Exec(context.Background(), `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true)`, scope.TenantID, scope.StoreID, scope.PrincipalID); err != nil {
		t.Fatal(err)
	}
	got, err := quickstart.Get(context.Background(), tx, scope, tok)
	if err != nil || !got.Completed || got.CompletedAt == nil || !got.CompletedAt.Equal(completed) {
		t.Fatalf("old-store service state=%+v err=%v", got, err)
	}
}

func TestOC05MarkerRLSAndTimestampAuthority(t *testing.T) {
	e := setup(t)
	tn := tenant(t, e)
	s, other := store(t, e, tn), store(t, e, tn)
	tok := staff(t, e, tn, s, true)
	otherTok := staff(t, e, tn, other, true)
	for _, v := range []struct{ store, token string }{{s, tok}, {other, otherTok}} {
		if w := request(e, http.MethodPost, v.store, v.token, `{}`); w.Code != 200 {
			t.Fatalf("complete fixture status=%d", w.Code)
		}
	}
	err := platform.WithScope(context.Background(), e.runtime, tok, s, "store:read", func(tx pgx.Tx, _ platform.Scope) error {
		var count int
		if err := tx.QueryRow(context.Background(), `SELECT count(*) FROM control.quickstart_markers WHERE tenant_id=$1 AND store_id=$2`, tn, other).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			t.Fatalf("RLS exposed %d sibling marker rows", count)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO control.quickstart_markers(tenant_id,store_id) VALUES($1,$2)`, []any{tn, other}},
		{`UPDATE control.quickstart_markers SET completed_at='2000-01-01' WHERE tenant_id=$1 AND store_id=$2`, []any{tn, s}},
		{`DELETE FROM control.quickstart_markers WHERE tenant_id=$1 AND store_id=$2`, []any{tn, s}},
		{`INSERT INTO control.quickstart_markers(tenant_id,store_id,completed_at) VALUES($1,$2,'2000-01-01')`, []any{tn, s}},
	} {
		err = platform.WithScope(context.Background(), e.runtime, tok, s, "integration:manage", func(tx pgx.Tx, _ platform.Scope) error {
			_, err := tx.Exec(context.Background(), stmt.sql, stmt.args...)
			return err
		})
		pg, ok := err.(*pgconn.PgError)
		if !ok || pg.Code != "42501" {
			t.Fatalf("runtime marker authority was not denied by RLS/grants: %v", err)
		}
	}
}
