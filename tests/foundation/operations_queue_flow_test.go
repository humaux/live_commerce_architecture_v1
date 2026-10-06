// Purpose: REAL_PG end-to-end and concurrency gates of the W6-05B operations ledger with a real River dispatcher on the claims-lane worker login and a call-counting MOCK provider:
//   UNKNOWN needs proof, a proved FAILED_FINAL of a registered kind is re-dispatched exactly once under the same idempotency key, an exhausted budget is restored by `query`,
//   two concurrent actions yield one effect, and cancel racing a dispatch claim never both win.
// Depends on: operations_queue_test.go (oqEnv, call/post/row helpers), dispatcher_test.go (t06StartDispatcher, t06DispatchOptions), external_operation_test.go (t06GoFixture).
// Used by: scripts/dev/test-local.sh --operations-queue (go test -run '^TestOperationsQueue').
// Invariants: I06/I07 and contracts/external-operation-v1.md "Amendment W6-05B": no second provider effect for one idempotency key, UNKNOWN is never retried, generation stays monotonic.
// Status: REAL_PG + MOCK provider (no network).

package foundation_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	integration "livecommerce/internal/integrations/core"
)

// oqProvider is a MOCK provider that counts every call and every applied effect per idempotency key.
type oqProvider struct {
	mu                     sync.Mutex
	dispatches, reconciles int
	keys                   []string
	effects                map[string]int
	dispatchMode           string // lose: error before the effect; apply_lose: effect then error (lost ack); apply: effect and success
	reconcileMode          string // unknown | not_applied | applied
}

func newOQProvider(dispatchMode, reconcileMode string) *oqProvider {
	return &oqProvider{effects: map[string]int{}, dispatchMode: dispatchMode, reconcileMode: reconcileMode}
}

func (p *oqProvider) set(dispatchMode, reconcileMode string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dispatchMode, p.reconcileMode = dispatchMode, reconcileMode
}

func (p *oqProvider) counts() (dispatches, reconciles, effects int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, n := range p.effects {
		effects += n
	}
	return p.dispatches, p.reconciles, effects
}

func (p *oqProvider) route() integration.DispatchRoute {
	return integration.DispatchRoute{
		Provider: "facebook", Action: "meta.live_videos", Purpose: "service",
		Check: func(context.Context, integration.DispatchRequest) error { return nil },
		Dispatch: func(_ context.Context, in integration.DispatchRequest) (integration.Outcome, error) {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.dispatches++
			p.keys = append(p.keys, in.IdempotencyKey)
			switch p.dispatchMode {
			case "lose":
				return integration.Outcome{}, errors.New("PRIVATE_PROVIDER_DETAIL timeout")
			case "apply_lose":
				p.effects[in.IdempotencyKey]++
				return integration.Outcome{}, errors.New("PRIVATE_PROVIDER_DETAIL ack lost")
			}
			p.effects[in.IdempotencyKey]++
			return integration.Outcome{State: "SUCCEEDED", Code: "mock_observed", ProviderReference: "mock-1"}, nil
		},
		Reconcile: func(_ context.Context, in integration.DispatchRequest) (integration.Outcome, error) {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.reconciles++
			switch p.reconcileMode {
			case "not_applied":
				return integration.Outcome{State: "FAILED_FINAL", Code: "mock_not_applied"}, nil
			case "applied":
				if p.effects[in.IdempotencyKey] > 0 {
					return integration.Outcome{State: "SUCCEEDED", Code: "mock_found", ProviderReference: "mock-found"}, nil
				}
			}
			return integration.Outcome{State: "UNKNOWN", Code: "mock_not_found"}, nil
		},
	}
}

// isolate moves the operation's available default-queue jobs to a private queue so only this test's dispatcher works them.
func (e *oqEnv) isolate(t *testing.T, id, queue string) {
	t.Helper()
	res, err := e.base.owner.Exec(context.Background(), `UPDATE river.river_job SET queue=$1 WHERE kind='external_operation_v1' AND args->>'operation_id'=$2 AND state='available' AND queue='default'`, queue, id)
	if err != nil || res.RowsAffected() < 1 {
		t.Fatalf("isolate jobs of %s: rows=%v err=%v", id, res.RowsAffected(), err)
	}
}

func (e *oqEnv) await(t *testing.T, id, state, code string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var st, c string
	for time.Now().Before(deadline) {
		st, c, _, _ = e.row(t, id)
		if st == state && (code == "" || c == code) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("operation %s stayed %s/%s, want %s/%s", id, st, c, state, code)
}

// awaitJobs waits until the newest external_operation_v1 job of the operation (the one the last action or the plan enqueued) reaches state.
func (e *oqEnv) awaitJobs(t *testing.T, id, state string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if jobs := e.jobRows(t, id); len(jobs) > 0 && jobs[len(jobs)-1].State == state {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("newest job of %s never reached %s: %+v", id, state, e.jobRows(t, id))
}

// Dispatch is ambiguous -> UNKNOWN -> the automatic budget runs out. Retry is refused without proof; `query` restores the budget and the reconcile
// proves the action was NOT applied (FAILED_FINAL); retry then re-dispatches under the same key and the provider applies it exactly once.
func TestOperationsQueueProofThenRetryNeverDoublesEffect(t *testing.T) {
	e := newOQEnv(t)
	o := e.live(t)
	queue := "oq_" + strings.ReplaceAll(randomUUID(), "-", "")
	e.isolate(t, o.ID, queue)
	p := newOQProvider("lose", "unknown")
	opts := t06DispatchOptions()
	opts.MaxGenerations = 3
	t06StartDispatcher(t, e.worker, queue, []integration.DispatchRoute{p.route()}, opts)

	// 1. dispatch (lost before the effect) + two unknown reconciles exhaust the budget: manual-required UNKNOWN, queue job cancelled.
	e.await(t, o.ID, "UNKNOWN", "reconcile_budget_exhausted")
	e.awaitJobs(t, o.ID, "cancelled")
	if d, r, ef := p.counts(); d != 1 || r != 2 || ef != 0 {
		t.Fatalf("after exhaustion: dispatches=%d reconciles=%d effects=%d, want 1/2/0", d, r, ef)
	}
	list := oqItems(t, e.call(t, "GET", e.store, "?state=UNKNOWN", e.token, "", ""))
	if len(list) != 1 || list[0]["reason_code"] != "reconcile_budget_exhausted" || list[0]["attempts"].(float64) != 3 {
		t.Fatalf("ledger row: %v", list)
	}
	if ok, why := oqAction(list[0], "query"); !ok || why != "" {
		t.Fatalf("query should be available for an exhausted UNKNOWN: %v %q", ok, why)
	}
	if ok, why := oqAction(list[0], "retry"); ok || why != "reconcile_first" {
		t.Fatalf("retry must say reconcile_first for UNKNOWN: %v %q", ok, why)
	}

	// 2. retry of UNKNOWN is refused, whatever the kind registry says; no job, no dispatch.
	jobsBefore := len(e.jobRows(t, o.ID))
	if r := e.post(t, o.ID, "retry", 3); r.Status != 409 || r.code() != "reconcile_first" {
		t.Fatalf("retry UNKNOWN: %d %s", r.Status, r.Body)
	}
	if len(e.jobRows(t, o.ID)) != jobsBefore {
		t.Fatal("a refused retry enqueued a job")
	}

	// 3. query: a fresh bounded budget; the reconcile now proves "not applied" -> FAILED_FINAL. The dispatcher counts generation - floor (4-3), so it really reconciles.
	p.set("lose", "not_applied")
	if r := e.post(t, o.ID, "query", 3); r.Status != 200 {
		t.Fatalf("query: %d %s", r.Status, r.Body)
	}
	e.isolate(t, o.ID, queue)
	e.await(t, o.ID, "FAILED_FINAL", "mock_not_applied")
	if d, r, ef := p.counts(); d != 1 || r != 3 || ef != 0 {
		t.Fatalf("after proof: dispatches=%d reconciles=%d effects=%d, want 1/3/0", d, r, ef)
	}
	if st, _, gen, floor := e.row(t, o.ID); st != "FAILED_FINAL" || gen != 4 || floor != 3 {
		t.Fatalf("after proof: %s gen=%d floor=%d, want FAILED_FINAL 4/3", st, gen, floor)
	}
	if ok, why := oqAction(oqItems(t, e.call(t, "GET", e.store, "?state=FAILED", e.token, "", ""))[0], "retry"); !ok || why != "" {
		t.Fatalf("retry should be available after proof: %v %q", ok, why)
	}

	// 4. retry: same operation, READY, a fresh job; the dispatcher claims it in dispatch mode with the SAME idempotency key and the provider applies it once.
	p.set("apply", "unknown")
	if r := e.post(t, o.ID, "retry", 4); r.Status != 200 || r.M["state"] != "READY" {
		t.Fatalf("retry: %d %s", r.Status, r.Body)
	}
	e.isolate(t, o.ID, queue)
	e.await(t, o.ID, "SUCCEEDED", "mock_observed")
	e.awaitJobs(t, o.ID, "completed")
	d, r, ef := p.counts()
	if d != 2 || r != 3 || ef != 1 {
		t.Fatalf("after retry: dispatches=%d reconciles=%d effects=%d, want 2/3/1", d, r, ef)
	}
	key := "lc:" + o.ID
	if len(p.keys) != 2 || p.keys[0] != key || p.keys[1] != key || p.effects[key] != 1 {
		t.Fatalf("idempotency keys %v effects %v, want the same lc:<operation_id> twice and one effect", p.keys, p.effects)
	}
	if e.events(t, o.ID, "dispatch_claimed") != 2 || e.events(t, o.ID, "query_requested") != 1 || e.events(t, o.ID, "retry_authorized") != 1 {
		t.Errorf("events dispatch_claimed=%d query_requested=%d retry_authorized=%d, want 2/1/1",
			e.events(t, o.ID, "dispatch_claimed"), e.events(t, o.ID, "query_requested"), e.events(t, o.ID, "retry_authorized"))
	}
	if st, _, gen, _ := e.row(t, o.ID); st != "SUCCEEDED" || gen != 5 {
		t.Fatalf("final row %s gen=%d, want SUCCEEDED 5", st, gen)
	}

	// 5. Done is done: nothing is available any more and no action re-runs the effect.
	for act, want := range map[string]string{"retry": "already_succeeded", "query": "not_in_doubt", "cancel": "operation_closed"} {
		if r := e.post(t, o.ID, act, 5); r.Status != 409 || r.code() != want {
			t.Errorf("%s after success: %d %s, want 409 %s", act, r.Status, r.Body, want)
		}
	}
	if d, _, ef := p.counts(); d != 2 || ef != 1 {
		t.Fatalf("effects changed after refusals: dispatches=%d effects=%d", d, ef)
	}
	for _, leak := range []string{"PRIVATE_PROVIDER_DETAIL"} {
		var found bool
		if err := e.base.owner.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM river.river_job WHERE args->>'operation_id'=$1 AND errors::text LIKE '%'||$2||'%')
			OR EXISTS(SELECT 1 FROM integration.operation_events WHERE operation_id=$1::uuid AND reason_code LIKE '%'||$2||'%')`, o.ID, leak).Scan(&found); err != nil || found {
			t.Fatalf("raw provider error %q leaked (err=%v)", leak, err)
		}
	}
}

// The ack was lost AFTER the provider applied the effect: the reconcile finds it (SUCCEEDED), which is terminal; retry is refused and the effect stays single.
func TestOperationsQueueAppliedEffectIsNeverRetried(t *testing.T) {
	e := newOQEnv(t)
	o := e.live(t)
	queue := "oq_" + strings.ReplaceAll(randomUUID(), "-", "")
	e.isolate(t, o.ID, queue)
	p := newOQProvider("apply_lose", "unknown")
	opts := t06DispatchOptions()
	opts.MaxGenerations = 3
	t06StartDispatcher(t, e.worker, queue, []integration.DispatchRoute{p.route()}, opts)
	e.await(t, o.ID, "UNKNOWN", "reconcile_budget_exhausted")
	e.awaitJobs(t, o.ID, "cancelled")
	if r := e.post(t, o.ID, "retry", 3); r.Status != 409 || r.code() != "reconcile_first" {
		t.Fatalf("retry UNKNOWN: %d %s", r.Status, r.Body)
	}
	p.set("apply_lose", "applied")
	if r := e.post(t, o.ID, "query", 3); r.Status != 200 {
		t.Fatalf("query: %d %s", r.Status, r.Body)
	}
	e.isolate(t, o.ID, queue)
	e.await(t, o.ID, "SUCCEEDED", "mock_found")
	if r := e.post(t, o.ID, "retry", 4); r.Status != 409 || r.code() != "already_succeeded" {
		t.Fatalf("retry after the effect was found: %d %s", r.Status, r.Body)
	}
	if d, _, ef := p.counts(); d != 1 || ef != 1 {
		t.Fatalf("dispatches=%d effects=%d, want exactly 1/1", d, ef)
	}
}

// Exactly one of many concurrent actions wins; the effect happens once. Same-key replays all see the one stored answer.
func TestOperationsQueueConcurrentActionsYieldOneEffect(t *testing.T) {
	e := newOQEnv(t)
	run := func(n int, fn func(i int) oqResp) []oqResp {
		out := make([]oqResp, n)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				out[i] = fn(i)
			}(i)
		}
		close(start)
		wg.Wait()
		return out
	}
	tally := func(rs []oqResp) (ok int, codes map[string]int) {
		codes = map[string]int{}
		for _, r := range rs {
			if r.Status == http.StatusOK {
				ok++
			} else {
				codes[fmt.Sprintf("%d %s", r.Status, r.code())]++
			}
		}
		return
	}

	// Retries with distinct keys.
	rt := e.live(t)
	e.set(t, rt.ID, "FAILED_FINAL", 2, "", "provider_rejected")
	e.killJobs(t, rt.ID)
	before := len(e.jobRows(t, rt.ID))
	ok, codes := tally(run(8, func(int) oqResp { return e.post(t, rt.ID, "retry", 2) }))
	if ok != 1 || codes["409 already_queued"] != 7 {
		t.Fatalf("8 concurrent retries: ok=%d others=%v, want 1 and 7x 409 already_queued", ok, codes)
	}
	if got := len(e.jobRows(t, rt.ID)); got != before+1 || e.audits(t, "integration.operation_retry_authorized") != 1 || e.events(t, rt.ID, "retry_authorized") != 1 {
		t.Fatalf("jobs %d->%d audits=%d events=%d, want one of each", before, got, e.audits(t, "integration.operation_retry_authorized"), e.events(t, rt.ID, "retry_authorized"))
	}
	// ... and the one re-opened operation really runs the provider once.
	queue := "oq_" + strings.ReplaceAll(randomUUID(), "-", "")
	e.isolate(t, rt.ID, queue)
	p := newOQProvider("apply", "unknown")
	t06StartDispatcher(t, e.worker, queue, []integration.DispatchRoute{p.route()}, t06DispatchOptions())
	e.await(t, rt.ID, "SUCCEEDED", "mock_observed")
	e.awaitJobs(t, rt.ID, "completed")
	if d, _, ef := p.counts(); d != 1 || ef != 1 {
		t.Fatalf("provider dispatches=%d effects=%d after 8 concurrent retries, want 1/1", d, ef)
	}

	// Retries with ONE key and body: the command receipt serialises them; every caller gets the same stored answer and one job exists.
	same := e.live(t)
	e.set(t, same.ID, "FAILED_FINAL", 1, "", "provider_rejected")
	e.killJobs(t, same.ID)
	before = len(e.jobRows(t, same.ID))
	key := uniqueAction("oq-same")
	rs := run(6, func(int) oqResp {
		return e.call(t, "POST", e.store, "/"+same.ID+"/retry", e.token, key, `{"expected_attempts":1}`)
	})
	for _, r := range rs {
		if r.Status != 200 || string(r.Body) != string(rs[0].Body) {
			t.Fatalf("same-key replay: %d %s (first %s)", r.Status, r.Body, rs[0].Body)
		}
	}
	if got := len(e.jobRows(t, same.ID)); got != before+1 || e.events(t, same.ID, "retry_authorized") != 1 {
		t.Fatalf("same-key retries: jobs %d->%d events=%d, want exactly one", before, got, e.events(t, same.ID, "retry_authorized"))
	}

	// Concurrent queries with distinct keys: one new job, the rest see it live.
	q := e.mock(t)
	e.set(t, q.ID, "UNKNOWN", 3, "", "reconcile_budget_exhausted")
	e.killJobs(t, q.ID)
	before = len(e.jobRows(t, q.ID))
	ok, codes = tally(run(8, func(int) oqResp { return e.post(t, q.ID, "query", 3) }))
	if ok != 1 || codes["409 query_in_progress"] != 7 || len(e.jobRows(t, q.ID)) != before+1 || e.events(t, q.ID, "query_requested") != 1 {
		t.Fatalf("8 concurrent queries: ok=%d others=%v jobs %d->%d", ok, codes, before, len(e.jobRows(t, q.ID)))
	}
}

// Cancel racing a dispatch claim: exactly one wins, and a cancelled operation can never also have been claimed for dispatch.
func TestOperationsQueueCancelRacesDispatchClaim(t *testing.T) {
	e := newOQEnv(t)
	cancelled, claimed := 0, 0
	for i := 0; i < 20; i++ {
		o := e.mock(t)
		var cancelResp oqResp
		var disposition string
		var claimErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			cancelResp = e.post(t, o.ID, "cancel", 0)
		}()
		go func() {
			defer wg.Done()
			<-start
			time.Sleep(time.Duration(i%8) * time.Millisecond) // sweep the interleaving so each side wins sometimes
			var gen int64
			var mode string
			claimErr = e.worker.QueryRow(context.Background(), `SELECT disposition,generation,mode FROM integration.claim_operation($1,30,$2)`, o.ID, randomBytes(32)).Scan(&disposition, &gen, &mode)
		}()
		close(start)
		wg.Wait()
		if claimErr != nil {
			t.Fatalf("claim: %v", claimErr)
		}
		st, _, _, _ := e.row(t, o.ID)
		switch {
		case cancelResp.Status == 200 && disposition == "terminal" && st == "CANCELLED":
			cancelled++
		case cancelResp.Status == 409 && (cancelResp.code() == "already_dispatched" || cancelResp.code() == "operation_changed") && disposition == "claimed" && st == "DISPATCHING":
			claimed++ // the claim bumped the generation first: the cancel's CAS or its capability check refuses
		default:
			t.Fatalf("iteration %d: cancel=%d %s claim=%s state=%s: both or neither won", i, cancelResp.Status, cancelResp.Body, disposition, st)
		}
		if st == "CANCELLED" && e.events(t, o.ID, "dispatch_claimed") != 0 {
			t.Fatalf("iteration %d: a cancelled operation has a dispatch claim", i)
		}
	}
	t.Logf("cancel won %d, claim won %d of 20", cancelled, claimed)
	if cancelled == 0 || claimed == 0 {
		t.Fatalf("the race sweep never produced both outcomes (cancel %d, claim %d)", cancelled, claimed)
	}
}
