package foundation_test

// meta_multi_page_fix_test.go: independent gates of the backend follow-up to unit meta-multi-page (branch
// unit/meta-multi-page-tests), written from docs/delivery/units/meta-multi-page.md + the reviewer's P2 findings, not from
// the implementation. They pin the three fixes the reviewer named:
//
//   MPG10 TestMetaConnectIndDisconnectLock     disconnect takes the SAME per-store advisory lock as meta_connect_finish,
//                                              so a disconnect racing a reconnect cannot enqueue an unsubscribe job for a
//                                              Page that ends up connected (P2-1).
//   MPG11 TestMetaConnectIndRetireDisabledSource  rebinding a Studio scene from a disconnected Page's post to a live
//                                              Page's post retires the old source (B1 / P1, binding_missing fix).
//   MPG12 TestMetaConnectIndPickDisconnectIdempotent  pick/disconnect replay by Idempotency-Key (B2).
//
// Tier: MOCK / REAL_PG (fake Graph). Concurrency is proven with held locks and pg_stat_activity observation, never with sleeps.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"livecommerce/tests/metaconnect/fakegraph"
)

// callKey issues one meta-connect request with an explicit Idempotency-Key (the generic callOn always mints a fresh one).
func (m *mcnEnv) callKey(store, token, method, path, key string, body any) mcnResp {
	m.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			m.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	var req *http.Request
	if rd != nil {
		req = httptest.NewRequest(method, "/v1/admin/stores/"+store+"/meta-connect"+path, rd)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, "/v1/admin/stores/"+store+"/meta-connect"+path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	m.handler.ServeHTTP(w, req)
	out := mcnResp{Status: w.Code, Raw: w.Body.Bytes()}
	_ = json.Unmarshal(out.Raw, &out.JSON)
	return out
}

// ---------------------------------------------------------------------------------------------------------------------
// MPG10 (P2-1): disconnect takes the per-store advisory lock. We hold the lock, launch a disconnect, and observe (via
// pg_stat_activity) that it blocks on the advisory lock until we release it. A disconnect that does NOT take the lock
// completes while we still hold it, which is the failure this pin catches.
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectIndDisconnectLock(t *testing.T) {
	m := newMcnEnv(t)
	f := m.f
	ctx := context.Background()
	m.reset()
	defer m.reset()

	page := mcnPage("MPG10 lock", false)
	m.connectPage(page)

	// Take the exact advisory lock meta_connect_finish and meta_connect_disconnect share, on a held transaction.
	hold, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Rollback(ctx)
	if _, err := hold.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(jsonb_build_array('meta-connect-store',$1::uuid,$2::uuid)::text,0))`,
		f.tenantA, m.store); err != nil {
		t.Fatal(err)
	}

	done := make(chan mcnResp, 1)
	go func() { done <- m.call("POST", "/disconnect", true, map[string]any{"page_id": page.ID}) }()

	// Poll pg_stat_activity until the disconnect backend blocks on an advisory lock (no ordering conclusion rests on the
	// poll delay; a completed disconnect without blocking is reported immediately).
	blocked := false
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var n int64
		if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database()
			AND state='active' AND wait_event_type='Lock' AND wait_event='advisory' AND query LIKE '%meta_connect_disconnect%'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n >= 1 {
			blocked = true
			break
		}
		select {
		case r := <-done:
			t.Fatalf("disconnect completed without taking the store lock: %d %s", r.Status, r.Raw)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("disconnect never blocked on the per-store advisory lock")
	}
	select {
	case r := <-done:
		t.Fatalf("disconnect finished while the store lock was still held: %d %s", r.Status, r.Raw)
	default:
	}
	// Release the lock: the disconnect proceeds and succeeds.
	if err := hold.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if r := <-done; r.Status != 200 {
		t.Fatalf("disconnect after the lock was released: %d %s", r.Status, r.Raw)
	}
}

// MPG10b: the end-to-end invariant the lock guarantees — racing a disconnect and a reconnect of the SAME Page can never
// leave a connected Page with an open (PENDING/LEASED) unsubscribe job (that job would DELETE /{page}/subscribed_apps for a
// live Page). Both goroutines are released together by a barrier; the outcome may be "disconnected" or "reconnected", but
// never "reconnected with a stale job".
func TestMetaConnectIndDisconnectReconnectRace(t *testing.T) {
	m := newMcnEnv(t)
	m.reset()
	defer m.reset()

	for round := 0; round < 6; round++ {
		page := mcnPage(fmt.Sprintf("MPG10 race %d", round), false)
		m.connectPage(page)

		var wg sync.WaitGroup
		gate := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-gate
			m.call("POST", "/disconnect", true, map[string]any{"page_id": page.ID})
		}()
		go func() {
			defer wg.Done()
			<-gate
			// A non-fatal reconnect: the disconnect may win the race, leaving the pick with a disabled binding (409
			// binding_disabled). That is a legitimate outcome; the invariant is checked after both sides settle.
			state, cb := m.connect(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}})
			if cb.Status != 200 {
				return
			}
			_ = m.pick(state, page.ID, page.IGID != "")
		}()
		close(gate)
		wg.Wait()

		connected := m.count(`SELECT count(*) FROM integration.meta_connections WHERE store_id=$1 AND page_id=$2`, m.store, page.ID)
		open := m.count(`SELECT count(*) FROM integration.meta_unsubscribe_jobs WHERE store_id=$1 AND page_id=$2 AND state IN ('PENDING','LEASED')`, m.store, page.ID)
		if connected > 0 && open > 0 {
			t.Fatalf("round %d: Page %s is connected with an open unsubscribe job (a racing disconnect would unsubscribe the live Page)", round, page.ID)
		}
		m.reset()
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// MPG11 (B1 / P1): rebinding a Studio scene from a disconnected Page's post to a live Page's post retires the old source.
// live.put_claim_source must deactivate a source whose route a disconnect already disabled (binding_missing fix), while a
// NEW active source still requires an enabled route.
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectIndRetireDisabledSource(t *testing.T) {
	m := newMcnEnv(t)
	e, f := m.e, m.f
	ctx := context.Background()
	m.reset()
	defer m.reset()

	pageA, pageB := mcnPage("MPG11 rebind A", true), mcnPage("MPG11 rebind B", false)
	m.connectPage(pageA)
	m.connectPage(pageB)
	postA, postB := pageA.ID+"_"+mciDigits(10), pageB.ID+"_"+mciDigits(10)

	// Bind the scene to B's post (active source, version 1).
	if _, err := e.putSource(e.session, "page", pageB.ID, postB, false, "zh-TW", true, 0); err != nil {
		t.Fatalf("bind pageB: %v", err)
	}
	// Disconnect B: its route and binding are disabled but the route row survives.
	if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": pageB.ID}); r.Status != 200 {
		t.Fatalf("disconnect B: %d %s", r.Status, r.Raw)
	}
	// Rebind: retire B's source (deactivate; its route is now disabled) then bind A's post.
	if _, err := e.putSource(e.session, "page", pageB.ID, postB, false, "zh-TW", false, 1); err != nil {
		t.Fatalf("retire pageB's source (deactivation of a disconnected Page): %v", err)
	}
	if _, err := e.putSource(e.session, "page", pageA.ID, postA, false, "zh-TW", true, 0); err != nil {
		t.Fatalf("rebind to pageA: %v", err)
	}
	// Exactly one active source remains among the scene's two posts, and it is A's.
	if n := m.count(`SELECT count(*) FROM live.claim_sources WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3
		AND active AND object='page' AND source_object_id=$4 AND asset_id=$5`, f.tenantA, m.store, e.session, postA, pageA.ID); n != 1 {
		t.Fatalf("A's post (%s) is not the single active source bound to pageA (%s)", postA, pageA.ID)
	}
	var bActive bool
	if err := f.owner.QueryRow(ctx, `SELECT active FROM live.claim_sources WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3
		AND object='page' AND asset_id=$4 AND source_object_id=$5`, f.tenantA, m.store, e.session, pageB.ID, postB).Scan(&bActive); err != nil {
		t.Fatalf("pageB source: %v", err)
	}
	if bActive {
		t.Fatal("pageB's source must be inactive after the rebind")
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// MPG12 (B2): pick and disconnect replay by Idempotency-Key — same key + same body returns the first result (no re-run,
// no second Meta call / job / audit); same key + different body is refused 409.
// ---------------------------------------------------------------------------------------------------------------------

func TestMetaConnectIndPickDisconnectIdempotent(t *testing.T) {
	m := newMcnEnv(t)
	m.reset()
	defer m.reset()

	t.Run("pick replays by key and reaches Meta once", func(t *testing.T) {
		m.reset()
		page := mcnPage("MPG12 pick", false)
		state := m.mcgFresh(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}})
		key := t04Key("mcn-pick-replay")
		body := map[string]any{"state_id": state, "page_id": page.ID, "include_instagram": false}
		first := m.callKey(m.store, m.token, "POST", "/pick", key, body)
		if first.Status != 201 {
			t.Fatalf("first pick: %d %s", first.Status, first.Raw)
		}
		subs := m.fake.Count("POST", "/subscribed_apps")
		replay := m.callKey(m.store, m.token, "POST", "/pick", key, body)
		if replay.Status != 201 || replay.str("page_id") != page.ID || replay.JSON["instagram"] != false {
			t.Fatalf("replayed pick: %d %s", replay.Status, replay.Raw)
		}
		if m.fake.Count("POST", "/subscribed_apps") != subs {
			t.Fatal("a replayed pick reached Meta again")
		}
		if m.count(`SELECT count(*) FROM integration.meta_connections WHERE store_id=$1 AND page_id=$2`, m.store, page.ID) != 1 ||
			m.count(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND external_asset_id=$2 AND enabled`, m.store, page.ID) != 1 {
			t.Fatal("a replayed pick created a second connection or binding")
		}
		// Same key, different body: refused before any Meta call.
		if r := m.callKey(m.store, m.token, "POST", "/pick", key, map[string]any{"state_id": state, "page_id": page.ID, "include_instagram": true}); r.Status != 409 {
			t.Fatalf("pick with the same key but a different body: %d %s (want 409)", r.Status, r.Raw)
		}
	})

	t.Run("disconnect replays by key and enqueues one job", func(t *testing.T) {
		m.reset()
		page := mcnPage("MPG12 disc", false)
		m.connectPage(page)
		key := t04Key("mcn-disc-replay")
		body := map[string]any{"page_id": page.ID}
		auditsBefore := m.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='meta.connect.disconnected'`, m.store)
		if r := m.callKey(m.store, m.token, "POST", "/disconnect", key, body); r.Status != 200 {
			t.Fatalf("first disconnect: %d %s", r.Status, r.Raw)
		}
		if r := m.callKey(m.store, m.token, "POST", "/disconnect", key, body); r.Status != 200 {
			t.Fatalf("replayed disconnect: %d %s (want 200, not 404)", r.Status, r.Raw)
		}
		if m.count(`SELECT count(*) FROM integration.meta_unsubscribe_jobs WHERE store_id=$1 AND page_id=$2`, m.store, page.ID) != 1 {
			t.Fatal("a replayed disconnect enqueued a second unsubscribe job")
		}
		if m.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='meta.connect.disconnected'`, m.store) != auditsBefore+1 {
			t.Fatal("a replayed disconnect audited twice")
		}
		// Same key, different body: refused. A fresh key on the already-disconnected Page is a normal 404.
		other := mcnPage("MPG12 disc other", false)
		if r := m.callKey(m.store, m.token, "POST", "/disconnect", key, map[string]any{"page_id": other.ID}); r.Status != 409 {
			t.Fatalf("disconnect with the same key but a different body: %d %s (want 409)", r.Status, r.Raw)
		}
		if r := m.call("POST", "/disconnect", true, body); r.Status != 404 {
			t.Fatalf("disconnect with a fresh key on a disconnected Page: %d %s (want 404)", r.Status, r.Raw)
		}
	})
}
