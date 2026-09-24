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
func btFillQuota(t *testing.T, h bhHarness, ceiling int) {
	t.Helper()
	var owned []string
	t.Cleanup(func() {
		// The shared foundation store survives this test; remove ONLY these seed
		// rows, never another test's owner or any issued/revoked runtime event.
		mustExec(t, h.f.owner, `DELETE FROM buyer.capability_sessions WHERE owner_id=ANY($1::uuid[])`, owned)
		mustExec(t, h.f.owner, `DELETE FROM buyer.owners WHERE id=ANY($1::uuid[])`, owned)
	})
	for minute := 0; minute < 2; minute++ {
		rows, err := h.f.owner.Query(context.Background(), `WITH quota_window AS (
          SELECT date_trunc('minute',clock_timestamp(),'UTC')+make_interval(mins=>$4) AS m
        ), needed AS (
          SELECT greatest(0,$3-count(c.id))::integer AS n FROM quota_window w LEFT JOIN buyer.capability_sessions c
          ON c.store_id=$2 AND c.created_at>=w.m AND c.created_at<w.m+interval '1 minute'
        ), owners AS (
          INSERT INTO buyer.owners(tenant_id,store_id) SELECT $1,$2 FROM needed,generate_series(1,needed.n) RETURNING id
        ) INSERT INTO buyer.capability_sessions(tenant_id,store_id,owner_id,token_hash,created_at,expires_at)
          SELECT $1,$2,o.id,decode(md5(o.id::text)||md5(o.id::text||'fixture'),'hex'),w.m,w.m+interval '1 hour'
          FROM owners o CROSS JOIN quota_window w RETURNING owner_id::text`, h.f.tenantA, h.f.storeA1, ceiling, minute)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			owned = append(owned, id)
		}
		rows.Close()
		if rows.Err() != nil {
			t.Fatal(rows.Err())
		}
	}
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
	btFillQuota(t, h, 600)
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

func TestBuyerHTTPSharedQuotaSerializesLastSlotAcrossInstances(t *testing.T) {
	h := bhSetup(t)
	ctx := context.Background()
	// Keep this causal wait/commit inside one quota window. Waiting at most10s
	// here is fixture clock alignment, never an application retry/relaxed gate.
	var second float64
	if err := h.f.owner.QueryRow(ctx, `SELECT extract(second FROM clock_timestamp())::float8`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	if second > 50 {
		time.Sleep(time.Duration((60.05 - second) * float64(time.Second)))
	}
	btFillQuota(t, h, 599)
	before := brCounts(t, h)
	tx, err := h.a.issuer.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	winnerToken := brToken()
	winnerHash := sha256.Sum256([]byte(winnerToken))
	var winnerID string
	if err = tx.QueryRow(ctx, `SELECT session_id::text FROM buyer.register_capability_limited($1,$2,3600)`, h.f.storeA1, winnerHash[:]).Scan(&winnerID); err != nil {
		t.Fatal(err)
	}
	app := "quota-loser-" + t04Tag()
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
	go func() { _, e := svc.RegisterForTrustedStore(ctx, h.f.storeA1, brToken()); done <- e }()
	waitForDatabaseLock(t, h.f.owner, app)
	var advisory bool
	if err = h.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock' AND wait_event='advisory')`, app).Scan(&advisory); err != nil || !advisory {
		t.Fatal("last-slot competitor did not wait on store admission lock")
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = waitError(t, done); !errors.Is(err, buyer.ErrRateLimited) {
		t.Fatalf("last-slot loser result=%v, want limited", err)
	}
	brAssertDelta(t, h, before, 1)
	if brStored(t, h, winnerToken).Scope.SessionID != winnerID {
		t.Fatal("quota winner changed")
	}
	brRead(t, h, winnerToken)
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
