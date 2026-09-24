package foundation_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/buyer"
	"livecommerce/internal/platform"
)

const retirePath = "/v1/buyer/session/retire"

func btRetire(t *testing.T, h bhHarness, token string) {
	t.Helper()
	r := h.request(t, "POST", retirePath, token, "", struct{}{}, nil)
	if r.status != 204 || len(r.body) != 0 {
		t.Fatalf("retirement status=%d, want empty204", r.status)
	}
}

func TestBuyerHTTPRetirementWinsBeforeDelayedActivation(t *testing.T) {
	h := bhSetup(t)
	token, before := brToken(), brCounts(t, h)
	entered, release := make(chan struct{}), make(chan struct{})
	delayed := h
	handler := brHandler(t, h, time.Hour)
	delayed.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release // causal pause BEFORE registration, not a probabilistic sleep
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(delayed.server.Close)
	var once sync.Once
	defer once.Do(func() { close(release) })
	done := make(chan bhResponse, 1)
	go func() { done <- delayed.request(t, "POST", brPath, token, "", struct{}{}, nil) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("activation did not reach causal pause")
	}
	btRetire(t, h, token)
	first := brStored(t, h, token)
	once.Do(func() { close(release) })
	select {
	case r := <-done:
		bhError(t, r, 401, "unauthorized")
	case <-time.After(6 * time.Second):
		t.Fatal("late activation did not return")
	}
	btRetire(t, h, token)
	if brStored(t, h, token).Scope != first.Scope {
		t.Fatal("retirement retry replaced owner")
	}
	brAssertDelta(t, h, before, 1)
	if countRows(t, h.f.owner, `SELECT count(*) FROM buyer.capability_events WHERE session_id=$1 AND action='capability.revoked'`, first.Scope.SessionID) != 1 {
		t.Fatal("retirement must leave one durable revoked event")
	}
	bhError(t, h.request(t, "GET", "/v1/buyer/session", token, "", nil, nil), 401, "unauthorized")
}

func TestBuyerHTTPRetirementConcurrentAndUncommittedWinner(t *testing.T) {
	h := bhSetup(t)
	other := h
	other.server = httptest.NewServer(brHandler(t, h, time.Hour))
	t.Cleanup(other.server.Close)
	token, before := brToken(), brCounts(t, h)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			target := h
			if i%2 == 1 {
				target = other
			}
			btRetire(t, target, token)
		}(i)
	}
	wg.Wait()
	brAssertDelta(t, h, before, 1)
	bhError(t, h.request(t, "POST", brPath, token, "", struct{}{}, nil), 401, "unauthorized")

	// The registration winner is uncommitted. Retirement waits, then retires the
	// exact committed owner; it must not create a second owner on unique conflict.
	ctx := context.Background()
	token = brToken()
	hash := sha256.Sum256([]byte(token))
	tx, err := h.a.issuer.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var sessionID string
	err = tx.QueryRow(ctx, `SELECT session_id::text FROM buyer.register_capability($1,$2,3600)`, h.f.storeA1, hash[:]).Scan(&sessionID)
	if err != nil {
		t.Fatal(err)
	}
	app := "retire-wait-" + t04Tag()
	pool, err := platform.OpenBuyerIssuerPool(ctx, withApplicationName(t, h.a.issuerURL, app))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	svc, err := buyer.New(pool, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- svc.RetireForTrustedStore(ctx, h.f.storeA1, token) }()
	waitForDatabaseLock(t, h.f.owner, app)
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = waitError(t, done); err != nil {
		t.Fatal(err)
	}
	if brStored(t, h, token).Scope.SessionID != sessionID {
		t.Fatal("retired a different winner")
	}
	brAssertDelta(t, h, before, 2)
	bhError(t, other.request(t, "POST", brPath, token, "", struct{}{}, nil), 401, "unauthorized")
}

// Seed only disposable quota-count fixtures, not accepted orders or payments.
// Fill this and the next minute so a clock rollover cannot make a flaky429.
func btFillQuota(t *testing.T, h bhHarness) {
	t.Helper()
	mustExec(t, h.f.owner, `DO $body$
DECLARE m timestamptz; n integer; o uuid; k integer;
BEGIN
 FOR k IN 0..1 LOOP
  m:=date_trunc('minute',clock_timestamp(),'UTC')+make_interval(mins=>k);
  SELECT greatest(0,600-count(*))::integer INTO n FROM buyer.capability_sessions WHERE store_id='`+h.f.storeA1+`'::uuid AND created_at>=m AND created_at<m+interval '1 minute';
  FOR i IN 1..n LOOP
   INSERT INTO buyer.owners(tenant_id,store_id) VALUES('`+h.f.tenantA+`','`+h.f.storeA1+`') RETURNING id INTO o;
   INSERT INTO buyer.capability_sessions(tenant_id,store_id,owner_id,token_hash,created_at,expires_at)
    VALUES('`+h.f.tenantA+`','`+h.f.storeA1+`',o,decode(md5(o::text)||md5(o::text||'fixture'),'hex'),m,m+interval '1 hour');
  END LOOP;
 END LOOP;
END $body$`)
}

func TestBuyerHTTPSharedQuotaReplayRetireAndIsolation(t *testing.T) {
	h := bhSetup(t)
	ctx := context.Background()
	stale, err := h.a.issuer.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer stale.Rollback(ctx)
	// Anchor before another connection fills the store's budget, without reading
	// privileged tables or widening the issuer's grants.
	if _, err = stale.Exec(ctx, `SELECT txid_current_snapshot()`); err != nil {
		t.Fatal(err)
	}
	btFillQuota(t, h)
	before := brCounts(t, h)
	other := h
	other.server = httptest.NewServer(brHandler(t, h, time.Hour))
	t.Cleanup(other.server.Close)
	for _, target := range []bhHarness{h, other} {
		fresh := brToken()
		bhError(t, target.request(t, "POST", brPath, fresh, "", struct{}{}, nil), 429, "rate_limited")
		bhError(t, target.request(t, "POST", retirePath, fresh, "", struct{}{}, nil), 429, "rate_limited")
	}
	brAssertDelta(t, h, before, 0)
	// Known active replay and retirement never depend on spare admission budget.
	brRead(t, other, h.cap.Token)
	btRetire(t, other, h.cap.Token)
	btRetire(t, h, h.cap.Token)
	brAssertDelta(t, h, before, 0)
	hash := sha256.Sum256([]byte(brToken()))
	_, err = stale.Exec(ctx, `SELECT * FROM buyer.register_capability_limited($1,$2,3600)`, h.f.storeA1, hash[:])
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "PT503" {
		t.Fatal("stale snapshot did not fail closed")
	}
	if err = stale.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	brAssertDelta(t, h, before, 0)
	// Another store does not spend this store's budget.
	bhPublish(t, h.bcHarness, "https://quota-other.example", h.f.tenantA, h.f.storeA2)
	other.origin = "https://quota-other.example"
	brRead(t, other, brToken())
	brAssertDelta(t, h, before, 1)
	bhError(t, other.request(t, "POST", retirePath, h.cap.Token, "", struct{}{}, nil), 401, "unauthorized")
}

func TestBuyerHTTPRetirementAuthorityAndStrictInput(t *testing.T) {
	h := bhSetup(t)
	ctx := context.Background()
	for _, name := range []string{"register_capability_limited", "retire_capability"} {
		var safe bool
		err := h.f.owner.QueryRow(ctx, `SELECT p.prosecdef AND p.provolatile='v' AND r.rolname='commerce_buyer_writer' AND p.proconfig=ARRAY['search_path=pg_catalog'] AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE') AND has_function_privilege('commerce_buyer_issuer',p.oid,'EXECUTE') AND NOT has_function_privilege('commerce_buyer_runtime',p.oid,'EXECUTE') FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, "buyer."+name+"(uuid,bytea,bigint)").Scan(&safe)
		if err != nil || !safe {
			t.Fatal("unsafe public admission function authority")
		}
		for _, ttl := range []int64{0, 59, 2592001} {
			_, err = h.a.issuer.Exec(ctx, `SELECT buyer.`+name+`($1,$2,$3)`, h.f.storeA1, randomBytes(32), ttl)
			var failure *pgconn.PgError
			if !errors.As(err, &failure) || failure.Code != "PT400" {
				t.Fatal("invalid TTL accepted")
			}
		}
	}
	before := brCounts(t, h)
	for _, input := range []any{map[string]any{"unexpected": true}, nil} {
		r := h.request(t, "POST", retirePath, brToken(), "", input, nil)
		if r.status < 400 {
			t.Fatal("invalid retirement body accepted")
		}
	}
	bhError(t, h.request(t, "POST", retirePath, brToken(), "forbidden-key", struct{}{}, nil), 422, "invalid_request")
	bhError(t, h.request(t, "GET", retirePath, brToken(), "", nil, nil), 405, "method_not_allowed")
	bhError(t, h.request(t, "POST", retirePath, brToken(), "", struct{}{}, func(r *http.Request) { r.Header.Set("Origin", "https://buyer.example") }), 403, "forbidden")
	bhError(t, h.request(t, "POST", retirePath, strings.Repeat("!", 43), "", struct{}{}, nil), 401, "unauthorized")
	brAssertDelta(t, h, before, 0)
}
