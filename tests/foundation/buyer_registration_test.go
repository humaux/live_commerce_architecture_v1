package foundation_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/buyer"
	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

const brPath = "/v1/buyer/session/bootstrap"

type brResponse struct {
	Authenticated bool      `json:"authenticated"`
	ExpiresAt     time.Time `json:"expires_at"`
}

func brToken() string { return base64.RawURLEncoding.EncodeToString(randomBytes(32)) }

func brCounts(t *testing.T, h bhHarness) [3]int {
	t.Helper()
	return [3]int{
		countRows(t, h.f.owner, `SELECT count(*) FROM buyer.owners`),
		countRows(t, h.f.owner, `SELECT count(*) FROM buyer.capability_sessions`),
		countRows(t, h.f.owner, `SELECT count(*) FROM buyer.capability_events WHERE action='capability.issued'`),
	}
}

func brAssertDelta(t *testing.T, h bhHarness, before [3]int, delta int) {
	t.Helper()
	after := brCounts(t, h)
	for i := range before {
		if after[i]-before[i] != delta {
			t.Fatalf("registration fact delta at %d=%d want=%d", i, after[i]-before[i], delta)
		}
	}
}

func brRead(t *testing.T, h bhHarness, token string) brResponse {
	t.Helper()
	r := h.request(t, "POST", brPath, token, "", struct{}{}, nil)
	v := bhRead[brResponse](t, r, 200)
	var fields map[string]json.RawMessage
	if json.Unmarshal(r.body, &fields) != nil || len(fields) != 2 || fields["authenticated"] == nil || fields["expires_at"] == nil || !v.Authenticated || v.ExpiresAt.IsZero() || bytes.Contains(r.body, []byte(token)) {
		t.Fatal("registration response is not the exact safe projection")
	}
	return v
}

func brStored(t *testing.T, h bhHarness, token string) buyer.Capability {
	t.Helper()
	hash := sha256.Sum256([]byte(token))
	var c buyer.Capability
	err := h.f.owner.QueryRow(context.Background(), `SELECT tenant_id::text,store_id::text,owner_id::text,id::text,expires_at FROM buyer.capability_sessions WHERE token_hash=$1`, hash[:]).Scan(
		&c.Scope.TenantID, &c.Scope.StoreID, &c.Scope.OwnerID, &c.Scope.SessionID, &c.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	c.Token = token
	return c
}

// Separate service and pool prove registration does not rely on in-process
// memoization. These are synthetic private HTTP servers, not a public browser.
func brHandler(t *testing.T, h bhHarness, ttl time.Duration) http.Handler {
	t.Helper()
	issuer, err := platform.OpenBuyerIssuerPool(context.Background(), h.a.issuerURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(issuer.Close)
	handler, err := buyerhttp.New(context.Background(), issuer, h.a.runtime, h.service, h.key, ttl)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func TestBuyerHTTPRegistrationLostResponseReplay(t *testing.T) {
	h := bhSetup(t)
	token, before := brToken(), brCounts(t, h)
	// Commit through the real handler, then close the TCP connection without
	// sending its buffered response. The caller cannot know whether issue ran.
	registered := make(chan int, 1)
	handler := brHandler(t, h, time.Hour)
	drop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		registered <- rec.Code
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(drop.Close)
	req, err := http.NewRequest("POST", drop.URL+brPath, strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Commerce-Buyer-BFF-Key", h.key)
	req.Header.Set("X-Commerce-Storefront-Origin", h.origin)
	client := &http.Client{Timeout: 12 * time.Second}
	if res, callErr := client.Do(req); callErr == nil {
		res.Body.Close()
		t.Fatal("response-loss fixture did not drop the committed response")
	}
	if <-registered != 200 {
		t.Fatal("dropped response was not a committed success")
	}
	first := brStored(t, h, token)
	for i := 0; i < 3; i++ {
		if got := brRead(t, h, token); !got.ExpiresAt.Equal(first.ExpiresAt) {
			t.Fatal("replay changed original expiry")
		}
	}
	cart := bhRead[storefront.Cart](t, h.request(t, "PUT", "/v1/buyer/cart", token, t04Key("registered-cart"), storefront.CartInput{Items: []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: 1}}}, nil), 200)
	// Changing the server TTL must not extend an already registered session.
	other := h
	other.server = httptest.NewServer(brHandler(t, h, 24*time.Hour))
	t.Cleanup(other.server.Close)
	if got := brRead(t, other, token); !got.ExpiresAt.Equal(first.ExpiresAt) {
		t.Fatal("new instance/TTL changed expiry")
	}
	again := bhRead[storefront.Cart](t, other.request(t, "GET", "/v1/buyer/cart", token, "", nil, nil), 200)
	if !reflect.DeepEqual(cart, again) || brStored(t, h, token).Scope != first.Scope {
		t.Fatal("replay switched cart owner")
	}
	brAssertDelta(t, h, before, 1)
	brRead(t, h, brToken())
	brAssertDelta(t, h, before, 2)
	// Existing issuance is intentionally still independent, not made replayable.
	for i := 0; i < 2; i++ {
		bhRead[map[string]any](t, h.request(t, "POST", "/v1/buyer/session", "", "", struct{}{}, nil), 200)
	}
	brAssertDelta(t, h, before, 4)
}

func TestBuyerHTTPRegistrationConcurrentAndLosingOwnerRollback(t *testing.T) {
	h := bhSetup(t)
	other := h
	other.server = httptest.NewServer(brHandler(t, h, time.Hour))
	t.Cleanup(other.server.Close)
	token, before := brToken(), brCounts(t, h)
	var wg sync.WaitGroup
	results := make([]brResponse, 8)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			target := h
			if i%2 == 1 {
				target = other
			}
			results[i] = brRead(t, target, token)
		}(i)
	}
	wg.Wait()
	for _, r := range results {
		if r != results[0] {
			t.Fatal("concurrent HTTP registration differs")
		}
	}
	brAssertDelta(t, h, before, 1)

	// Force the exception path: an uncommitted winner is invisible to the loser
	// until its duplicate token insert waits on the winner's transaction ID.
	token = brToken()
	hash := sha256.Sum256([]byte(token))
	ctx := context.Background()
	tx, err := h.a.issuer.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var winner buyer.Capability
	err = tx.QueryRow(ctx, `SELECT tenant_id::text,store_id::text,owner_id::text,session_id::text,expires_at FROM buyer.register_capability($1,$2,3600)`, h.f.storeA1, hash[:]).Scan(&winner.Scope.TenantID, &winner.Scope.StoreID, &winner.Scope.OwnerID, &winner.Scope.SessionID, &winner.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	app := "registration-loser-" + t04Tag()
	pool, err := platform.OpenBuyerIssuerPool(ctx, withApplicationName(t, h.a.issuerURL, app))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	service, err := buyer.New(pool, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		c   buyer.Capability
		err error
	}
	done := make(chan result, 1)
	go func() { c, e := service.RegisterForTrustedStore(ctx, h.f.storeA1, token); done <- result{c, e} }()
	waitForDatabaseLock(t, h.f.owner, app)
	var uniqueWait bool
	if err := h.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock' AND wait_event='transactionid')`, app).Scan(&uniqueWait); err != nil || !uniqueWait {
		t.Fatal("loser did not wait on uncommitted winner")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-done:
		if r.err != nil || r.c.Scope != winner.Scope || !r.c.ExpiresAt.Equal(winner.ExpiresAt) {
			t.Fatal("unique-conflict replay failed")
		}
	case <-time.After(6 * time.Second):
		t.Fatal("registration loser did not return")
	}
	brAssertDelta(t, h, before, 2)

	// A stale REPEATABLE READ snapshot cannot see a later winner. It must fail
	// closed, not invent another owner or spin in an internal retry loop.
	stale, err := h.a.issuer.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer stale.Rollback(ctx)
	anchor := sha256.Sum256([]byte(h.cap.Token))
	var anchorOwner string
	if err = stale.QueryRow(ctx, `SELECT owner_id::text FROM buyer.register_capability($1,$2,3600)`, h.f.storeA1, anchor[:]).Scan(&anchorOwner); err != nil {
		t.Fatal(err)
	}
	newToken := brToken()
	if _, err = service.RegisterForTrustedStore(ctx, h.f.storeA1, newToken); err != nil {
		t.Fatal(err)
	}
	newHash := sha256.Sum256([]byte(newToken))
	_, err = stale.Exec(ctx, `SELECT * FROM buyer.register_capability($1,$2,3600)`, h.f.storeA1, newHash[:])
	var failure *pgconn.PgError
	if !errors.As(err, &failure) || (failure.Code != "PT401" && failure.Code != "40001") {
		t.Fatal("stale snapshot did not fail closed")
	}
	if err = stale.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	brAssertDelta(t, h, before, 3)
}

func TestBuyerHTTPRegistrationAuthorityAndInvalidSQL(t *testing.T) {
	h := bhSetup(t)
	ctx := context.Background()
	token := brToken()
	hash := sha256.Sum256([]byte(token))
	query := `SELECT * FROM buyer.register_capability($1::uuid,$2::bytea,$3::bigint)`
	before := brCounts(t, h)
	for _, p := range []*pgxpool.Pool{h.a.runtime, h.f.runtime, h.a.identity, h.pool} {
		assertSQLDenied(t, p, query, h.f.storeA1, hash[:], 60)
	}
	assertSQLDenied(t, h.a.issuer, `SELECT * FROM buyer.capability_sessions`)
	assertSQLDenied(t, h.a.issuer, `INSERT INTO buyer.owners(tenant_id,store_id) VALUES($1,$2)`, h.f.tenantA, h.f.storeA1)
	var safe bool
	err := h.f.owner.QueryRow(ctx, `SELECT p.prosecdef AND p.provolatile='v' AND r.rolname='commerce_buyer_writer' AND p.proconfig=ARRAY['search_path=pg_catalog'] AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE') FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid='buyer.register_capability(uuid,bytea,bigint)'::regprocedure`).Scan(&safe)
	if err != nil || !safe {
		t.Fatal("registration function authority/search_path unsafe")
	}
	for _, args := range [][]any{{nil, hash[:], 60}, {h.f.storeA1, nil, 60}, {h.f.storeA1, []byte{1}, 60}, {h.f.storeA1, hash[:], nil}, {h.f.storeA1, hash[:], 59}, {h.f.storeA1, hash[:], 2592001}} {
		_, e := h.a.issuer.Exec(ctx, query, args...)
		var pe *pgconn.PgError
		if !errors.As(e, &pe) || pe.Code != "PT400" {
			t.Fatal("invalid registration SQL input accepted or wrong error")
		}
	}
	brAssertDelta(t, h, before, 0)
	service, _ := buyer.New(h.a.issuer, time.Hour)
	for _, bad := range []string{"", token + "=", strings.Repeat("a", 42), " " + token} {
		if _, err := service.RegisterForTrustedStore(ctx, h.f.storeA1, bad); !errors.Is(err, buyer.ErrInvalid) {
			t.Fatal("noncanonical token accepted")
		}
	}
	if _, err := service.RegisterForTrustedStore(ctx, randomUUID(), token); !errors.Is(err, buyer.ErrUnauthorized) {
		t.Fatal("unknown store accepted")
	}
	brAssertDelta(t, h, before, 0)

	// A synthetic constraint confined to a new fixture store forces a different
	// 23505. It must propagate, not become a successful token-conflict replay.
	_, isolatedStore, _ := seedBuyerStores(t, h.f)
	mustIssue(t, service, isolatedStore)
	index := "registration_unrelated_" + strings.ReplaceAll(randomUUID(), "-", "")
	mustExec(t, h.f.owner, `CREATE UNIQUE INDEX `+pgx.Identifier{index}.Sanitize()+` ON buyer.owners(store_id) WHERE store_id='`+isolatedStore+`'::uuid`)
	t.Cleanup(func() { mustExec(t, h.f.owner, `DROP INDEX buyer.`+pgx.Identifier{index}.Sanitize()) })
	before = brCounts(t, h)
	otherHash := sha256.Sum256([]byte(brToken()))
	_, err = h.a.issuer.Exec(ctx, query, isolatedStore, otherHash[:], 60)
	var unique *pgconn.PgError
	if !errors.As(err, &unique) || unique.Code != "23505" || unique.ConstraintName != index || unique.TableName != "owners" {
		t.Fatal("unrelated uniqueness error was swallowed")
	}
	brAssertDelta(t, h, before, 0)
}

func TestBuyerHTTPRegistrationDenialsAndCurrentPublication(t *testing.T) {
	h := bhSetup(t)
	ctx := context.Background()
	service, _ := buyer.New(h.a.issuer, time.Hour)
	token := brToken()
	brRead(t, h, token)
	c := brStored(t, h, token)
	before := brCounts(t, h)
	for _, store := range []string{h.f.storeA2, h.f.storeB} {
		if _, err := service.RegisterForTrustedStore(ctx, store, token); !errors.Is(err, buyer.ErrUnauthorized) {
			t.Fatal("cross-store registration accepted")
		}
	}
	for _, tc := range []struct{ table, id string }{{"control.tenants", h.f.tenantA}, {"control.stores", h.f.storeA1}, {"buyer.owners", c.Scope.OwnerID}} {
		func() {
			mustExec(t, h.f.owner, `UPDATE `+tc.table+` SET active=false WHERE id=$1`, tc.id)
			defer mustExec(t, h.f.owner, `UPDATE `+tc.table+` SET active=true WHERE id=$1`, tc.id)
			if _, err := service.RegisterForTrustedStore(ctx, h.f.storeA1, token); !errors.Is(err, buyer.ErrUnauthorized) {
				t.Fatal("inactive scope registration accepted")
			}
		}()
	}
	second := "https://registration-second.example"
	bhPublish(t, h.bcHarness, second, h.f.tenantA, h.f.storeA2)
	mustExec(t, h.f.owner, `UPDATE control.storefront_domains SET store_id=$2,version=version+1 WHERE origin=$1`, h.origin, h.f.storeA2)
	bhError(t, h.request(t, "POST", brPath, token, "", struct{}{}, nil), 401, "unauthorized")
	mustExec(t, h.f.owner, `UPDATE control.storefront_domains SET store_id=$2,version=version+1 WHERE origin=$1`, h.origin, h.f.storeA1)
	mustExec(t, h.f.owner, `UPDATE control.storefront_publications SET published=false,version=version+1 WHERE store_id=$1`, h.f.storeA1)
	bhError(t, h.request(t, "POST", brPath, token, "", struct{}{}, nil), 404, "not_found")
	mustExec(t, h.f.owner, `UPDATE control.storefront_publications SET published=true,version=version+1 WHERE store_id=$1`, h.f.storeA1)
	brRead(t, h, token)
	if err := service.Revoke(ctx, token, h.f.storeA1); err != nil {
		t.Fatal(err)
	}
	bhError(t, h.request(t, "POST", brPath, token, "", struct{}{}, nil), 401, "unauthorized")
	brAssertDelta(t, h, before, 0)
	expired := brToken()
	brRead(t, h, expired)
	ec := brStored(t, h, expired)
	mustExec(t, h.f.owner, `UPDATE buyer.capability_sessions SET created_at=clock_timestamp()-interval '2 seconds',expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, ec.Scope.SessionID)
	bhError(t, h.request(t, "POST", brPath, expired, "", struct{}{}, nil), 401, "unauthorized")
	brAssertDelta(t, h, before, 1)
}

func TestBuyerHTTPRegistrationRevokeWaitAndTransport(t *testing.T) {
	h := bhSetup(t)
	token := brToken()
	brRead(t, h, token)
	before := brCounts(t, h)
	ctx := context.Background()
	hash := sha256.Sum256([]byte(token))
	// Hold a successful replay transaction open. Revoke must wait until its
	// authorization locks release; after revoke commits another replay fails.
	tx, err := h.a.issuer.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var owner string
	if err = tx.QueryRow(ctx, `SELECT owner_id::text FROM buyer.register_capability($1,$2,3600)`, h.f.storeA1, hash[:]).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	app := "registration-revoke-" + t04Tag()
	pool, err := platform.OpenBuyerIssuerPool(ctx, withApplicationName(t, h.a.issuerURL, app))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	service, _ := buyer.New(pool, time.Hour)
	done := make(chan error, 1)
	go func() { done <- service.Revoke(ctx, token, h.f.storeA1) }()
	waitForDatabaseLock(t, h.f.owner, app)
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = waitError(t, done); err != nil {
		t.Fatal(err)
	}
	bhError(t, h.request(t, "POST", brPath, token, "", struct{}{}, nil), 401, "unauthorized")
	for _, tc := range []struct {
		name, value string
		status      int
		code        string
	}{
		{"Authorization", "", 401, "unauthorized"}, {"Authorization", "Bearer malformed", 401, "unauthorized"},
		{"X-Commerce-Buyer-BFF-Key", "bad", 401, "unauthorized"}, {"Origin", h.origin, 403, "forbidden"},
		{"Cookie", "x=y", 403, "forbidden"}, {"X-Store-ID", h.f.storeA1, 403, "forbidden"}, {"Idempotency-Key", "retry-key", 422, "invalid_request"},
	} {
		bhError(t, h.request(t, "POST", brPath, brToken(), "", struct{}{}, func(r *http.Request) { r.Header.Set(tc.name, tc.value) }), tc.status, tc.code)
	}
	for _, body := range []any{map[string]any{"token": token}, nil, map[string]any{"unknown": 1}} {
		bhError(t, h.request(t, "POST", brPath, brToken(), "", body, func(r *http.Request) { r.Header.Set("Content-Type", "application/json") }), 400, "invalid_json")
	}
	bhError(t, h.request(t, "POST", brPath+"?x=1", brToken(), "", struct{}{}, nil), 403, "forbidden")
	bhError(t, h.request(t, "GET", brPath, brToken(), "", nil, nil), 405, "method_not_allowed")
	brAssertDelta(t, h, before, 0)
	// A transient failure is retryable only for registration, not old issuance.
	h.a.issuer.Close()
	e := bhError(t, h.request(t, "POST", brPath, brToken(), "", struct{}{}, nil), 503, "unavailable")
	if !e.Retryable {
		t.Fatal("registration transient failure not retryable")
	}
	e = bhError(t, h.request(t, "POST", "/v1/buyer/session", "", "", struct{}{}, nil), 503, "unavailable")
	if e.Retryable {
		t.Fatal("legacy issue failure became retryable")
	}
}
