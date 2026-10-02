package foundation_test

// meta_connect_unsubscribe_test.go: PG gates of meta-connect D2 (migration 0100): disconnect leaves the Page subscribed at Meta because the API can
// only seal Page tokens. Disconnect now enqueues a durable job in the same transaction; the claims-worker's metareply.Unsubscriber (the only
// holder of the private ring) opens the sealed token and calls DELETE /{page}/subscribed_apps. Label REAL_PG, Meta = MOCK (fake Graph / httptest).
// Run: bash scripts/dev/test-focused.sh '^TestMetaConnectUnsubscribe'

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"livecommerce/internal/integrations/meta/pagetoken/pageopen"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/platform"
	"livecommerce/tests/metaconnect/fakegraph"
)

// mcnUnsubscriber builds the claims-worker's Unsubscriber over a claims-authority pool against graphURL (not started: tests drive RunOnce).
func mcnUnsubscriber(t *testing.T, f *testFixture, open *pageopen.Keyring, graphURL string) *metareply.Unsubscriber {
	t.Helper()
	keys, err := metareply.NewPageTokenKeyring("k1", map[string][]byte{"k1": randomBytes(32)})
	if err != nil {
		t.Fatal(err)
	}
	u, err := metareply.NewUnsubscriber(waOpen(t, f, waClaims, platform.WorkerClaims), keys, open, metareply.Config{GraphBaseURL: graphURL, GraphVersion: mcnVersion})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// mcnRunUnsubscriber runs u in the background (what cmd/claims-worker does) until the test ends; the browser gates use it.
func mcnRunUnsubscriber(t *testing.T, u *metareply.Unsubscriber) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	u.SetIdle(100 * time.Millisecond)
	go func() { defer close(done); u.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
}

// mcnAwaitUnsubscribeDrain waits (bounded, polling persisted state) until the store has no open unsubscribe job, so a browser gate ends with
// every disconnected Page unsubscribed and no sealed token left behind.
func mcnAwaitUnsubscribeDrain(t *testing.T, f *testFixture, store string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		if countRows(t, f.owner, `SELECT count(*) FROM integration.meta_unsubscribe_jobs WHERE store_id=$1 AND state IN ('PENDING','LEASED')`, store) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("unsubscribe jobs still open after 20s: the claims-worker loop did not drain them")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// mcnConnectPage connects page (with Instagram when it has one) through the real start -> callback -> pick path.
func (m *mcnEnv) connectPage(page fakegraph.Page) {
	m.t.Helper()
	state, cb := m.connect(m.store, m.token, fakegraph.User{Permissions: mcnFullPerms, Pages: []fakegraph.Page{page}})
	if cb.Status != 200 {
		m.t.Fatalf("callback: %d %s", cb.Status, cb.Raw)
	}
	if r := m.pick(state, page.ID, page.IGID != ""); r.Status != 201 {
		m.t.Fatalf("pick %s: %d %s", page.ID, r.Status, r.Raw)
	}
}

// hijackClose drops the connection without answering: a transport error at the client.
func hijackClose(w http.ResponseWriter) {
	if h, ok := w.(http.Hijacker); ok {
		if c, _, err := h.Hijack(); err == nil {
			_ = c.Close()
		}
	}
}

type mcnJob struct {
	State, Code string
	Attempts    int
	Sealed      bool
}

func (m *mcnEnv) job(page string) mcnJob {
	m.t.Helper()
	var j mcnJob
	if err := m.f.owner.QueryRow(context.Background(), `SELECT state,coalesce(code,''),attempts,ciphertext IS NOT NULL OR nonce IS NOT NULL OR key_id IS NOT NULL
		FROM integration.meta_unsubscribe_jobs WHERE store_id=$1 AND page_id=$2 ORDER BY created_at DESC LIMIT 1`, m.store, page).Scan(&j.State, &j.Code, &j.Attempts, &j.Sealed); err != nil {
		m.t.Fatalf("job of Page %s: %v", page, err)
	}
	return j
}

func TestMetaConnectUnsubscribe(t *testing.T) {
	m := newMcnEnv(t)
	f := m.f
	ctx := context.Background()
	// Jobs left open by earlier tests of this database must not be consumed (and counted) here.
	mustExec(t, f.owner, `UPDATE integration.meta_unsubscribe_jobs SET state='SUPERSEDED',lease_until=NULL,key_id=NULL,nonce=NULL,ciphertext=NULL,code='test_cleanup' WHERE state IN ('PENDING','LEASED')`)

	t.Run("disconnect enqueues one job holding only sealed bytes; the worker unsubscribes the Page at Meta, wipes the token and audits", func(t *testing.T) {
		m.reset()
		page := mcnPage("Unsub A", true)
		m.connectPage(page)
		if !m.fake.Subscribed(page.ID) {
			t.Fatal("the pick must subscribe the Page")
		}
		if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": page.ID}); r.Status != 200 {
			t.Fatalf("disconnect: %d %s", r.Status, r.Raw)
		}
		if j := m.job(page.ID); j.State != "PENDING" || !j.Sealed || j.Attempts != 0 {
			t.Fatalf("job after disconnect: %+v", j)
		}
		// Credentials are destroyed and the binding/route disabled immediately, whatever the job does.
		if m.count(`SELECT count(*) FROM integration.meta_page_credentials WHERE store_id=$1 AND asset_id=ANY($2)`, m.store, []string{page.ID, page.IGID}) != 0 ||
			m.count(`SELECT count(*) FROM integration.bindings WHERE store_id=$1 AND enabled AND external_asset_id=ANY($2)`, m.store, []string{page.ID, page.IGID}) != 0 {
			t.Fatal("disconnect must destroy the credentials and disable the bindings regardless of the job")
		}
		// The API process itself made no Graph call with a stored token; the job is the only way the subscription is removed.
		if m.fake.Count("DELETE", "/subscribed_apps") != 0 || !m.fake.Subscribed(page.ID) {
			t.Fatal("the API must not unsubscribe (it cannot open a token)")
		}
		u := mcnUnsubscriber(t, f, m.open, m.fake.URL())
		if did, err := u.RunOnce(ctx); err != nil || !did {
			t.Fatalf("RunOnce: %v %v", did, err)
		}
		if m.fake.Count("DELETE", "/subscribed_apps") != 1 || m.fake.Subscribed(page.ID) {
			t.Fatal("the worker must DELETE subscribed_apps once and leave the Page unsubscribed")
		}
		if j := m.job(page.ID); j.State != "SUCCEEDED" || j.Code != "graph_unsubscribed" || j.Attempts != 1 || j.Sealed {
			t.Fatalf("job after the call (token must be wiped): %+v", j)
		}
		if m.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='meta.connect.unsubscribed'`, m.store) != 1 {
			t.Fatal("the unsubscribe must be audited")
		}
		for _, r := range m.fake.Requests() {
			if r.Method == "DELETE" && (r.HasQueryToken || r.BodyHasToken) {
				t.Fatalf("the Page token must travel in the Authorization header only: %+v", r)
			}
		}
		if did, _ := u.RunOnce(ctx); did {
			t.Fatal("a finished job must not run again")
		}
		if m.count(`SELECT count(*) FROM integration.meta_unsubscribe_jobs t WHERE t::text LIKE '%'||$1||'%'`, page.Token) != 0 {
			t.Fatal("no plaintext Page token may ever sit in the jobs table")
		}
	})

	t.Run("an ambiguous answer is UNKNOWN and never repeated; a 4xx is FAILED; both wipe the token", func(t *testing.T) {
		for _, c := range []struct {
			name        string
			handler     http.HandlerFunc
			state, code string
		}{
			{"transport error", func(w http.ResponseWriter, r *http.Request) { hijackClose(w) }, "UNKNOWN", "graph_unconfirmed"},
			{"500", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }, "UNKNOWN", "graph_unconfirmed"},
			{"200 without success", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"success":false}`)) }, "UNKNOWN", "graph_unconfirmed"},
			{"400 refused", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(400) }, "FAILED", "graph_refused"},
		} {
			m.reset()
			page := mcnPage("Unsub "+c.name, false)
			m.connectPage(page)
			if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": page.ID}); r.Status != 200 {
				t.Fatalf("%s: disconnect: %d %s", c.name, r.Status, r.Raw)
			}
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); c.handler(w, r) }))
			u := mcnUnsubscriber(t, f, m.open, srv.URL)
			if did, err := u.RunOnce(ctx); err != nil || !did {
				t.Fatalf("%s: RunOnce: %v %v", c.name, did, err)
			}
			if did, _ := u.RunOnce(ctx); did {
				t.Fatalf("%s: a terminal job must never be retried", c.name)
			}
			srv.Close()
			if j := m.job(page.ID); j.State != c.state || j.Code != c.code || j.Attempts != 1 || j.Sealed || calls.Load() != 1 {
				t.Fatalf("%s: job %+v, calls %d", c.name, j, calls.Load())
			}
		}
	})

	t.Run("a definite not-applied answer (503) retries with backoff, at most 5 attempts, then fails with the token wiped", func(t *testing.T) {
		m.reset()
		page := mcnPage("Unsub busy", false)
		m.connectPage(page)
		if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": page.ID}); r.Status != 200 {
			t.Fatalf("disconnect: %d %s", r.Status, r.Raw)
		}
		audited := m.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='meta.connect.unsubscribe_failed'`, m.store)
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(503) }))
		defer srv.Close()
		u := mcnUnsubscriber(t, f, m.open, srv.URL)
		for attempt := 1; attempt <= 5; attempt++ {
			if did, err := u.RunOnce(ctx); err != nil || !did {
				t.Fatalf("attempt %d: %v %v", attempt, did, err)
			}
			if attempt < 5 {
				if j := m.job(page.ID); j.State != "PENDING" || j.Attempts != attempt || !j.Sealed {
					t.Fatalf("attempt %d: %+v", attempt, j)
				}
				if did, _ := u.RunOnce(ctx); did { // backoff: not due yet
					t.Fatalf("attempt %d: the retry ran before its backoff", attempt)
				}
				mustExec(t, f.owner, `UPDATE integration.meta_unsubscribe_jobs SET next_attempt_at=clock_timestamp() WHERE store_id=$1 AND page_id=$2`, m.store, page.ID)
			}
		}
		if j := m.job(page.ID); j.State != "FAILED" || j.Code != "retries_exhausted" || j.Attempts != 5 || j.Sealed || calls.Load() != 5 {
			t.Fatalf("after 5 attempts: %+v, calls %d", j, calls.Load())
		}
		if m.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='meta.connect.unsubscribe_failed'`, m.store) != audited+1 {
			t.Fatal("the exhausted job must be audited once")
		}
	})

	t.Run("a worker without the private ring cannot open a v2 token: bounded retries, then FAILED with the token wiped, never a Graph call", func(t *testing.T) {
		m.reset()
		page := mcnPage("Unsub noring", false)
		m.connectPage(page)
		if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": page.ID}); r.Status != 200 {
			t.Fatalf("disconnect: %d %s", r.Status, r.Raw)
		}
		before := m.fake.Count("DELETE", "/subscribed_apps")
		u := mcnUnsubscriber(t, f, nil, m.fake.URL())
		if did, err := u.RunOnce(ctx); err != nil || !did {
			t.Fatalf("RunOnce: %v %v", did, err)
		}
		if j := m.job(page.ID); j.State != "PENDING" || j.Code != "token_unavailable" || !j.Sealed {
			t.Fatalf("job: %+v", j)
		}
		if m.fake.Count("DELETE", "/subscribed_apps") != before {
			t.Fatal("no Graph call without an opened token")
		}
	})

	t.Run("an expired lease means the outcome is unknown: closed UNKNOWN, never repeated", func(t *testing.T) {
		m.reset()
		page := mcnPage("Unsub lease", false)
		m.connectPage(page)
		if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": page.ID}); r.Status != 200 {
			t.Fatalf("disconnect: %d %s", r.Status, r.Raw)
		}
		mustExec(t, f.owner, `UPDATE integration.meta_unsubscribe_jobs SET state='LEASED',attempts=1,lease_until=clock_timestamp()-interval '1 second' WHERE store_id=$1 AND page_id=$2`, m.store, page.ID)
		before := m.fake.Count("DELETE", "/subscribed_apps")
		u := mcnUnsubscriber(t, f, m.open, m.fake.URL())
		if did, _ := u.RunOnce(ctx); did {
			t.Fatal("nothing is due after the expired lease is closed")
		}
		if j := m.job(page.ID); j.State != "UNKNOWN" || j.Code != "lease_expired" || j.Sealed || m.fake.Count("DELETE", "/subscribed_apps") != before {
			t.Fatalf("expired lease: %+v", j)
		}
	})

	t.Run("disconnect then reconnect of the SAME Page before the worker runs: the pending job is superseded, the live Page is never unsubscribed", func(t *testing.T) {
		m.reset()
		page := mcnPage("Unsub reconnect", false)
		m.connectPage(page)
		if r := m.call("POST", "/disconnect", true, map[string]any{"page_id": page.ID}); r.Status != 200 {
			t.Fatalf("disconnect: %d %s", r.Status, r.Raw)
		}
		m.connectPage(page)
		if j := m.job(page.ID); j.State != "SUPERSEDED" || j.Code != "reconnected" || j.Sealed {
			t.Fatalf("job after reconnect: %+v", j)
		}
		before := m.fake.Count("DELETE", "/subscribed_apps")
		u := mcnUnsubscriber(t, f, m.open, m.fake.URL())
		if did, _ := u.RunOnce(ctx); did || m.fake.Count("DELETE", "/subscribed_apps") != before || !m.fake.Subscribed(page.ID) {
			t.Fatal("a superseded job must not touch the reconnected Page")
		}
	})

	t.Run("the merchant runtime cannot claim or finish a job (claims-worker authority only)", func(t *testing.T) {
		for _, q := range []string{`SELECT * FROM integration.claim_meta_unsubscribe()`,
			`SELECT integration.finish_meta_unsubscribe(gen_random_uuid(),'SUCCEEDED','x')`, `SELECT count(*) FROM integration.meta_unsubscribe_jobs`} {
			if _, err := f.runtime.Exec(ctx, q); sqlState(err) != "42501" {
				t.Fatalf("runtime must be denied %q: %v", q, err)
			}
		}
	})
}
