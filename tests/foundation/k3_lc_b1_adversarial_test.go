// k3_lc_b1_adversarial_test.go is the independent K3 adversarial gate for LC-B1 (A7 session lifecycle + up to
// 5 OPEN claim windows per store), written from contracts/live-console-v1.md §8/§9/§11-A7 + Amendment 1 against
// internal/live/lifecycle.go, internal/claims/merchant.go (cap) and migrations/0122_live_lifecycle.sql. The author
// of the code under test did not write this file. Cases: concurrent start/end/archive races on one session, the
// per-store cap under cross-store and same-slot concurrency, lifecycle_version CAS stale/replay, Idempotency-Key
// reuse with a different body or session, claims ingest racing `end`, claims after end/archive, tenant isolation
// (RLS), and the author-known gaps (M2 opening a window on a draft session; archived sessions still accepting
// offer mutations) asserted to their SAFE outcome — a failing assertion here is a product finding, listed in
// output/k3-lc-b1/REPORT.md. Isolation: lcSetup's own principal; lcPurgeSessions cleans every session.
// Status: REAL_PG, MOCK manual ingress (no Meta consumer).
package foundation_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// k3Lifecycle calls live.Lifecycle with an explicit Idempotency-Key (the h.lifecycle helper mints a fresh key
// per call; key-reuse cases need to pin it).
func (h *lcHarness) k3Lifecycle(token, store, key, session string, in live.LifecycleInput) (out live.LifecycleResult, err error) {
	err = h.do(token, store, func(tx pgx.Tx, s platform.Scope) (e error) {
		out, e = live.Lifecycle(h.ctx, tx, s, token, key, session, in)
		return e
	})
	return out, err
}

// k3OpenWindows counts the store's OPEN claim windows through the owner pool (read-back of the §8.5 cap state).
func k3OpenWindows(t *testing.T, h *lcHarness, tenant, store string) int {
	t.Helper()
	var n int
	if err := h.f.owner.QueryRow(h.ctx, `SELECT count(*) FROM live.claim_windows
		WHERE tenant_id=$1 AND store_id=$2 AND state='OPEN'`, tenant, store).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// k3LifecycleRow reads the session's lifecycle columns directly (owner pool), bypassing every Go reader.
func k3LifecycleRow(t *testing.T, h *lcHarness, session string) (string, int64) {
	t.Helper()
	var lifecycle string
	var version int64
	if err := h.f.owner.QueryRow(h.ctx, `SELECT lifecycle,lifecycle_version FROM live.sessions WHERE id=$1`,
		session).Scan(&lifecycle, &version); err != nil {
		t.Fatal(err)
	}
	return lifecycle, version
}

// k3LineCount counts persisted claim lines of a session (owner pool).
func k3LineCount(t *testing.T, h *lcHarness, session string) int {
	t.Helper()
	var n int
	if err := h.f.owner.QueryRow(h.ctx, `SELECT count(*) FROM claims.lines WHERE session_id=$1`, session).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// k3AuditCount counts audit rows of one action of the harness store (owner pool).
func k3AuditCount(t *testing.T, h *lcHarness, action string) int {
	t.Helper()
	var n int
	if err := h.f.owner.QueryRow(h.ctx, `SELECT count(*) FROM ops.audit_events
		WHERE tenant_id=$1 AND store_id=$2 AND action=$3`, h.f.tenantA, h.f.storeA1, action).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// k3Race runs fn n times concurrently and returns the per-goroutine errors.
func k3Race(n int, fn func(i int) error) []error {
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = fn(i)
		}(i)
	}
	wg.Wait()
	return errs
}

// TestK3LcB1ConcurrentStartSameSession: §9 CAS — n concurrent starts of ONE draft session (same expected_version,
// distinct Idempotency-Keys) yield exactly one transition; the losers get version_conflict, the window opens once
// (generation 1), the store holds exactly one OPEN window and lifecycle_version is 2 (no double increment).
func TestK3LcB1ConcurrentStartSameSession(t *testing.T) {
	h := lcSetup(t)
	s := h.draft(t, h.f.storeA1)
	const n = 4
	errs := k3Race(n, func(i int) error {
		_, err := h.lifecycle(h.token, h.f.storeA1, s, "start", 1, nil)
		return err
	})
	won := 0
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case errors.Is(err, command.ErrConflict):
		default:
			t.Fatalf("concurrent start: unexpected error %v", err)
		}
	}
	if won != 1 {
		t.Fatalf("concurrent starts of one session: won=%d, want 1", won)
	}
	if w := h.board(t, s).Window; w.State != claims.WindowOpen || w.Generation != 1 {
		t.Fatalf("window after start race: %+v, want OPEN generation 1", w)
	}
	if n := k3OpenWindows(t, h, h.f.tenantA, h.f.storeA1); n != 1 {
		t.Fatalf("OPEN windows after start race: %d, want 1", n)
	}
	if lc, v := k3LifecycleRow(t, h, s); lc != "live" || v != 2 {
		t.Fatalf("lifecycle after start race: %s v%d, want live v2", lc, v)
	}
}

// TestK3LcB1ConcurrentEndArchiveRaces: (a) two concurrent ends — one wins, one version_conflict, the window closes
// exactly once; (b) archive from live is invalid_transition even when raced against end; (c) start vs archive raced
// from ended with the same expected_version — exactly one wins and the follow-up transition obeys the §9 table.
func TestK3LcB1ConcurrentEndArchiveRaces(t *testing.T) {
	h := lcSetup(t)

	// (a) concurrent end + end.
	a := h.draft(t, h.f.storeA1)
	h.mustLifecycle(t, a, "start", 1)
	errs := k3Race(2, func(i int) error {
		_, err := h.lifecycle(h.token, h.f.storeA1, a, "end", 2, nil)
		return err
	})
	k3OneWins(t, "end+end", errs, command.ErrConflict)
	if w := h.board(t, a).Window; w.State != claims.WindowClosed || w.ClosedAt == nil {
		t.Fatalf("window after end race: %+v", w)
	}
	if lc, v := k3LifecycleRow(t, h, a); lc != "ended" || v != 3 {
		t.Fatalf("lifecycle after end race: %s v%d, want ended v3", lc, v)
	}

	// (b) archive from live is refused even while a concurrent end commits.
	b := h.draft(t, h.f.storeA1)
	h.mustLifecycle(t, b, "start", 1)
	errs = k3Race(2, func(i int) error {
		action := "end"
		if i == 1 {
			action = "archive"
		}
		_, err := h.lifecycle(h.token, h.f.storeA1, b, action, 2, nil)
		return err
	})
	if !errors.Is(errs[1], claims.ErrInvalidTransition) {
		t.Fatalf("archive raced against end from live: err=%v, want invalid_transition", errs[1])
	}
	if errs[0] != nil {
		t.Fatalf("end raced against archive: %v", errs[0])
	}

	// (c) start vs archive from ended, same expected_version: exactly one transition lands.
	c := h.draft(t, h.f.storeA1)
	h.mustLifecycle(t, c, "start", 1)
	h.mustLifecycle(t, c, "end", 2)
	errs = k3Race(2, func(i int) error {
		action := "archive"
		if i == 1 {
			action = "start"
		}
		_, err := h.lifecycle(h.token, h.f.storeA1, c, action, 3, nil)
		return err
	})
	k3OneWins(t, "archive+start from ended", errs, command.ErrConflict)
	lc, v := k3LifecycleRow(t, h, c)
	if v != 4 {
		t.Fatalf("lifecycle_version after archive+start race: %d, want exactly one increment (4)", v)
	}
	switch lc {
	case "archived":
		lcIs(t, lcErr(h.lifecycle(h.token, h.f.storeA1, c, "start", 4, nil)), claims.ErrInvalidTransition, "start after archive won")
	case "live":
		r := h.mustLifecycle(t, c, "end", 4)
		if r.Window.State != claims.WindowClosed {
			t.Fatalf("end after start won: %+v", r)
		}
	default:
		t.Fatalf("lifecycle after archive+start race: %s", lc)
	}
}

func k3OneWins(t *testing.T, label string, errs []error, loser error) {
	t.Helper()
	won := 0
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case errors.Is(err, loser):
		default:
			t.Fatalf("%s: unexpected error %v", label, err)
		}
	}
	if won != 1 {
		t.Fatalf("%s: won=%d, want exactly 1 (errs=%v)", label, won, errs)
	}
}

// TestK3LcB1CapIsPerStoreUnderCrossStoreRace: §8.5 — the 5-OPEN cap is per store: 4 concurrent starts in store A1
// plus 4 concurrent starts in store A2 must ALL succeed (8 > 5 would trip a tenant-wide cap; a store-scoped
// advisory lock must not serialise one store's opens against the other's refusal).
func TestK3LcB1CapIsPerStoreUnderCrossStoreRace(t *testing.T) {
	h := lcSetup(t)
	const perStore = 4
	a1, a2 := make([]string, perStore), make([]string, perStore)
	for i := 0; i < perStore; i++ {
		a1[i] = h.draft(t, h.f.storeA1)
		a2[i] = h.draft(t, h.f.storeA2)
	}
	errs := k3Race(2*perStore, func(i int) error {
		if i%2 == 0 {
			_, err := h.lifecycle(h.token, h.f.storeA1, a1[i/2], "start", 1, nil)
			return err
		}
		_, err := h.lifecycle(h.token, h.f.storeA2, a2[i/2], "start", 1, nil)
		return err
	})
	for i, err := range errs {
		if err != nil {
			t.Fatalf("cross-store start #%d: %v (cap must be per store)", i, err)
		}
	}
	if n := k3OpenWindows(t, h, h.f.tenantA, h.f.storeA1); n != perStore {
		t.Fatalf("store A1 OPEN windows: %d, want %d", n, perStore)
	}
	if n := k3OpenWindows(t, h, h.f.tenantA, h.f.storeA2); n != perStore {
		t.Fatalf("store A2 OPEN windows: %d, want %d", n, perStore)
	}
}

// TestK3LcB1SixthSlotRace: with 5 windows OPEN, two concurrent would-be sixth opens are both refused and roll
// back (still draft v1, no window row, count stays 5); racing an end (frees a slot) against a sixth open never
// leaves 6 OPEN and leaves the loser fully rolled back.
func TestK3LcB1SixthSlotRace(t *testing.T) {
	h := lcSetup(t)
	open := make([]string, 0, claims.MaxOpenWindowsPerStore)
	for i := 0; i < claims.MaxOpenWindowsPerStore; i++ {
		s := h.draft(t, h.f.storeA1)
		h.mustLifecycle(t, s, "start", 1)
		open = append(open, s)
	}
	// Two concurrent contenders for the nonexistent sixth slot.
	c1, c2 := h.draft(t, h.f.storeA1), h.draft(t, h.f.storeA1)
	errs := k3Race(2, func(i int) error {
		s := c1
		if i == 1 {
			s = c2
		}
		_, err := h.lifecycle(h.token, h.f.storeA1, s, "start", 1, nil)
		return err
	})
	for i, err := range errs {
		lcIs(t, err, claims.ErrTooManyOpenWindows, fmt.Sprintf("sixth contender #%d", i))
	}
	for _, s := range []string{c1, c2} {
		if lc, v := k3LifecycleRow(t, h, s); lc != "draft" || v != 1 {
			t.Fatalf("refused sixth start must roll back: %s v%d, want draft v1", lc, v)
		}
		if w := h.board(t, s).Window; w.Version != 0 {
			t.Fatalf("refused sixth start left a window row: %+v", w)
		}
	}
	if n := k3OpenWindows(t, h, h.f.tenantA, h.f.storeA1); n != claims.MaxOpenWindowsPerStore {
		t.Fatalf("OPEN windows after refused sixth: %d, want %d", n, claims.MaxOpenWindowsPerStore)
	}

	// End (frees a slot) raced against a sixth open: the outcome depends on advisory-lock order, but the
	// invariant does not — never 6 OPEN, and a refused contender is still a clean draft.
	c3 := h.draft(t, h.f.storeA1)
	errs = k3Race(2, func(i int) error {
		if i == 0 {
			_, err := h.lifecycle(h.token, h.f.storeA1, open[0], "end", 2, nil)
			return err
		}
		_, err := h.lifecycle(h.token, h.f.storeA1, c3, "start", 1, nil)
		return err
	})
	if errs[0] != nil {
		t.Fatalf("end racing sixth open: %v", errs[0])
	}
	switch {
	case errs[1] == nil:
		if lc, _ := k3LifecycleRow(t, h, c3); lc != "live" {
			t.Fatalf("sixth open won the freed slot but lifecycle is %s", lc)
		}
	case errors.Is(errs[1], claims.ErrTooManyOpenWindows):
		if lc, v := k3LifecycleRow(t, h, c3); lc != "draft" || v != 1 {
			t.Fatalf("sixth open lost the race but was not rolled back: %s v%d", lc, v)
		}
	default:
		t.Fatalf("sixth open racing end: unexpected error %v", errs[1])
	}
	if n := k3OpenWindows(t, h, h.f.tenantA, h.f.storeA1); n > claims.MaxOpenWindowsPerStore {
		t.Fatalf("OPEN windows after end+open race: %d, cap %d", n, claims.MaxOpenWindowsPerStore)
	}
}

// TestK3LcB1IdempotencyKeyReuse: one Idempotency-Key replays only its exact body. The same key with a different
// action, a different open_window, a different expected_version or a different session is version_conflict
// (§11 A7 / command receipt rules), and replaying the original key+body after further transitions returns the
// stored result WITHOUT re-executing (no zombie restart of an ended session).
func TestK3LcB1IdempotencyKeyReuse(t *testing.T) {
	h := lcSetup(t)
	s := h.draft(t, h.f.storeA1)
	key := t04Key("k3-lc-key")
	first, err := h.k3Lifecycle(h.token, h.f.storeA1, key, s, live.LifecycleInput{Action: "start", ExpectedVersion: 1})
	if err != nil || first.Version != 2 {
		t.Fatalf("start: %+v %v", first, err)
	}
	no := false
	for label, in := range map[string]live.LifecycleInput{
		"different open_window":      {Action: "start", ExpectedVersion: 1, OpenWindow: &no},
		"different expected_version": {Action: "start", ExpectedVersion: 2},
		"different action":           {Action: "end", ExpectedVersion: 2},
	} {
		lcIs(t, lcErr(h.k3Lifecycle(h.token, h.f.storeA1, key, s, in)), command.ErrConflict, "key reuse: "+label)
	}
	// Same key, same body, different session.
	other := h.draft(t, h.f.storeA1)
	lcIs(t, lcErr(h.k3Lifecycle(h.token, h.f.storeA1, key, other, live.LifecycleInput{Action: "start", ExpectedVersion: 1})),
		command.ErrConflict, "key reuse across sessions")
	if lc, v := k3LifecycleRow(t, h, other); lc != "draft" || v != 1 {
		t.Fatalf("cross-session key reuse touched the other session: %s v%d", lc, v)
	}

	// Advance the session, then replay the original key+body: stored result, no re-execution.
	h.mustLifecycle(t, s, "end", 2)
	replay, err := h.k3Lifecycle(h.token, h.f.storeA1, key, s, live.LifecycleInput{Action: "start", ExpectedVersion: 1})
	if err != nil {
		t.Fatalf("replay of original key+body: %v", err)
	}
	if replay.Version != 2 || replay.Lifecycle != "live" {
		t.Fatalf("replay must return the stored result: %+v", replay)
	}
	if lc, v := k3LifecycleRow(t, h, s); lc != "ended" || v != 3 {
		t.Fatalf("replay re-executed the transition: %s v%d, want ended v3", lc, v)
	}
	if w := h.board(t, s).Window; w.State != claims.WindowClosed {
		t.Fatalf("replay reopened the window: %+v", w)
	}
}

// TestK3LcB1StaleVersionHasNoSideEffect: a stale expected_version is refused before any write — no lifecycle
// change, no window change, no audit row.
func TestK3LcB1StaleVersionHasNoSideEffect(t *testing.T) {
	h := lcSetup(t)
	s := h.draft(t, h.f.storeA1)
	r := h.mustLifecycle(t, s, "start", 1)
	audits := k3AuditCount(t, h, "live.session.ended")
	lcIs(t, lcErr(h.lifecycle(h.token, h.f.storeA1, s, "end", 1, nil)), command.ErrConflict, "stale expected_version")
	if lc, v := k3LifecycleRow(t, h, s); lc != "live" || v != 2 {
		t.Fatalf("stale request changed lifecycle: %s v%d", lc, v)
	}
	if w := h.board(t, s).Window; w.State != claims.WindowOpen || w.Generation != r.Window.Generation {
		t.Fatalf("stale request changed the window: %+v", w)
	}
	if n := k3AuditCount(t, h, "live.session.ended"); n != audits {
		t.Fatalf("stale request wrote an audit row: %d -> %d", audits, n)
	}
}

// TestK3LcB1DraftWindowViaM2: author-known risk. §9 routes window opening through lifecycle start; §2.2 polls
// never for draft sessions, so a draft session holding an OPEN window is a dead window that still consumes one of
// the 5 cap slots. SAFE outcome asserted: M2 (SetWindow) refuses to open a window on a draft (never-started)
// session. Failure here is a finding (see REPORT.md), not a test bug.
func TestK3LcB1DraftWindowViaM2(t *testing.T) {
	h := lcSetup(t)
	s := h.draft(t, h.f.storeA1)
	// Integrator ruling 2026-10-05 (F-M2-DRAFT): the claims board opens windows directly, so M2 open on a draft session
	// atomically starts it (lifecycle draft -> live, lifecycle_version+1, audit live.session.started) instead of being
	// refused. Only this block changed vs K3's original assertion (which expected invalid_transition).
	w, err := h.setWindow(h.token, h.f.storeA1, s, claims.WindowInput{State: claims.WindowOpen, MatchMode: claims.MatchExact})
	if err != nil {
		t.Fatalf("M2 open on draft session must auto-start it (ruling 2026-10-05): %v", err)
	}
	if w.State != claims.WindowOpen || w.Generation != 1 {
		t.Fatalf("window after auto-start: %+v, want OPEN generation 1", w)
	}
	if lc, v := k3LifecycleRow(t, h, s); lc != "live" || v != 2 {
		t.Fatalf("auto-started session: %s v%d, want live v2", lc, v)
	}
	if n := k3AuditCount(t, h, "live.session.started"); n < 1 {
		t.Fatalf("auto-start wrote no live.session.started audit row")
	}
	// The session is a real live session: a later lifecycle end (expected_version 2) closes the window.
	h.mustLifecycle(t, s, "end", 2)
	if w := h.board(t, s).Window; w.State != claims.WindowClosed {
		t.Fatalf("window after end: %+v", w)
	}
}

// TestK3LcB1ArchivedSessionMutations: §9 "archive ... read-only afterwards". The draft and the window are guarded
// (author tests); this asserts the SAME read-only rule for the session's remaining write surfaces: offer update,
// offer create and offer_timeline append. SAFE outcome asserted — failures are findings (author-known risk).
func TestK3LcB1ArchivedSessionMutations(t *testing.T) {
	h := lcSetup(t)
	skus := lcSKUs(t, h.f, h.f.tenantA, h.f.storeA1, "USD", 2)
	s := h.draft(t, h.f.storeA1)
	o := h.offer(t, s, "ARCHKEY", skus[0], 2)
	h.mustLifecycle(t, s, "start", 1)
	h.mustLifecycle(t, s, "end", 2)
	h.mustLifecycle(t, s, "archive", 3)

	if _, err := h.updateOffer(h.token, h.f.storeA1, t04Key("k3-arch-offer"), s, o.ID,
		claims.OfferUpdate{ExpectedVersion: o.Version, MaxQuantityPerClaim: 9, Active: false}); err == nil {
		t.Errorf("FINDING F-ARCH-OFFER-UPDATE: offer of an archived session is still mutable (§9 read-only)")
	}
	if _, err := h.createOffer(h.token, h.f.storeA1, t04Key("k3-arch-create"), s,
		claims.OfferInput{Keyword: "ARCHNEW", SKUID: skus[1], MaxQuantityPerClaim: 1}); err == nil {
		t.Errorf("FINDING F-ARCH-OFFER-CREATE: new offer accepted on an archived session (§9 read-only)")
	}
	if err := h.do(h.token, h.f.storeA1, func(tx pgx.Tx, sc platform.Scope) error {
		_, e := live.RecordOfferFeatured(h.ctx, tx, sc, h.token, s, o.ID)
		return e
	}); err == nil {
		t.Errorf("FINDING F-ARCH-TIMELINE: offer_timeline append accepted on an archived session (§9 read-only; contract silent — safe outcome asserted)")
	}
}

// TestK3LcB1EndWhileClaimsArrive: §9 OPEN-16 — end raced against in-flight manual claims (the MOCK ingress of the
// comment stream): every claim is either ACCEPTED with a persisted line or REJECTED/WINDOW_CLOSED with none; the
// reported outcomes must equal the database truth, and once end has returned, further claims are WINDOW_CLOSED.
func TestK3LcB1EndWhileClaimsArrive(t *testing.T) {
	h := lcSetup(t)
	sku := lcSKUs(t, h.f, h.f.tenantA, h.f.storeA1, "USD", 1)[0]
	s := h.draft(t, h.f.storeA1)
	h.offer(t, s, "RCKEY", sku, 1)
	h.mustLifecycle(t, s, "start", 1)

	const n = 8
	results := make([]claims.ManualClaimResult, n)
	errs := k3Race(n+1, func(i int) error {
		if i == n {
			_, err := h.lifecycle(h.token, h.f.storeA1, s, "end", 2, nil)
			return err
		}
		var err error
		results[i], err = h.manual(h.token, h.f.storeA1, t04Key("k3-race-claim"), s,
			claims.ManualClaimInput{ActorLabel: fmt.Sprintf("racer%d", i), Text: "RCKEY+1"})
		return err
	})
	for i, err := range errs {
		if err != nil {
			t.Fatalf("race op #%d hard-failed (deadlock/timeout/serialization leak?): %v", i, err)
		}
	}
	accepted := 0
	for i, r := range results {
		switch {
		case r.Outcome == claims.OutcomeAccepted:
			accepted++
		case r.Outcome == claims.OutcomeRejected && r.Reason == claims.ReasonWindowClosed:
		default:
			t.Fatalf("claim #%d under end race: unexpected outcome %+v", i, r)
		}
	}
	if lines := k3LineCount(t, h, s); lines != accepted {
		t.Fatalf("outcome/DB mismatch under end race: %d ACCEPTED but %d persisted lines", accepted, lines)
	}
	if lc, _ := k3LifecycleRow(t, h, s); lc != "ended" {
		t.Fatalf("lifecycle after end race: %s", lc)
	}
	if w := h.board(t, s).Window; w.State != claims.WindowClosed {
		t.Fatalf("window after end race: %+v", w)
	}
	// Late comment after end: never claimed (§9 OPEN-16).
	lines := k3LineCount(t, h, s)
	late, err := h.manual(h.token, h.f.storeA1, t04Key("k3-late"), s, claims.ManualClaimInput{ActorLabel: "lateone", Text: "RCKEY+1"})
	if err != nil {
		t.Fatalf("late claim: %v", err)
	}
	if late.Outcome != claims.OutcomeRejected || late.Reason != claims.ReasonWindowClosed {
		t.Fatalf("late claim after end: %+v, want REJECTED/WINDOW_CLOSED", late)
	}
	if n := k3LineCount(t, h, s); n != lines {
		t.Fatalf("late claim persisted a line: %d -> %d", lines, n)
	}
}

// TestK3LcB1ClaimsAfterEndAndArchive: deterministic OPEN-16 — claims on an ended and on an archived session are
// REJECTED/WINDOW_CLOSED and persist nothing.
func TestK3LcB1ClaimsAfterEndAndArchive(t *testing.T) {
	h := lcSetup(t)
	sku := lcSKUs(t, h.f, h.f.tenantA, h.f.storeA1, "USD", 1)[0]
	s := h.draft(t, h.f.storeA1)
	h.offer(t, s, "STOPKEY", sku, 1)
	h.mustLifecycle(t, s, "start", 1)
	h.accepted(t, s, "", "early", "STOPKEY+1")
	h.mustLifecycle(t, s, "end", 2)

	lines := k3LineCount(t, h, s)
	r, err := h.manual(h.token, h.f.storeA1, t04Key("k3-ended"), s, claims.ManualClaimInput{ActorLabel: "afterend", Text: "STOPKEY+1"})
	if err != nil || r.Outcome != claims.OutcomeRejected || r.Reason != claims.ReasonWindowClosed {
		t.Fatalf("claim on ended session: %+v %v", r, err)
	}
	h.mustLifecycle(t, s, "archive", 3)
	r, err = h.manual(h.token, h.f.storeA1, t04Key("k3-archclaim"), s, claims.ManualClaimInput{ActorLabel: "afterarch", Text: "STOPKEY+1"})
	if err != nil || r.Outcome != claims.OutcomeRejected || r.Reason != claims.ReasonWindowClosed {
		t.Fatalf("claim on archived session: %+v %v", r, err)
	}
	if n := k3LineCount(t, h, s); n != lines {
		t.Fatalf("claims after end/archive persisted lines: %d -> %d", lines, n)
	}
}

// TestK3LcB1TenantIsolation: I01 — a tenant-B principal with live:manage on its own store cannot transition,
// feature or even READ tenant A's session rows; a tenant-B token naming tenant A's store never resolves a scope.
func TestK3LcB1TenantIsolation(t *testing.T) {
	h := lcSetup(t)
	skus := lcSKUs(t, h.f, h.f.tenantA, h.f.storeA1, "USD", 1)
	s := h.draft(t, h.f.storeA1)
	o := h.offer(t, s, "ISOKEY", skus[0], 1)
	h.mustLifecycle(t, s, "start", 1)
	if err := h.do(h.token, h.f.storeA1, func(tx pgx.Tx, sc platform.Scope) error {
		_, e := live.RecordOfferFeatured(h.ctx, tx, sc, h.token, s, o.ID)
		return e
	}); err != nil {
		t.Fatalf("seed offer_timeline: %v", err)
	}

	_, tokenB := lcPrincipal(t, h.f, h.f.tenantB, []string{h.f.storeB}, "store:read", "live:read", "live:manage")
	// Tenant A's session addressed through tenant B's store: not found, never a cross-tenant hit.
	lcIs(t, lcErr(h.lifecycle(tokenB, h.f.storeB, s, "end", 2, nil)), command.ErrNotFound, "cross-tenant lifecycle")
	if err := h.do(tokenB, h.f.storeB, func(tx pgx.Tx, sc platform.Scope) error {
		_, e := live.RecordOfferFeatured(h.ctx, tx, sc, tokenB, s, o.ID)
		return e
	}); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("cross-tenant RecordOfferFeatured: %v, want not_found", err)
	}
	// Tenant B's token naming tenant A's store must fail scope resolution.
	if err := h.do(tokenB, h.f.storeA1, func(tx pgx.Tx, sc platform.Scope) error { return nil }); err == nil {
		t.Fatal("tenant B token resolved a scope on tenant A's store")
	}
	// RLS read-back: tenant B's runtime transaction sees none of tenant A's rows.
	for label, q := range map[string]string{
		"live.sessions":       `SELECT count(*) FROM live.sessions WHERE id=$1`,
		"live.offer_timeline": `SELECT count(*) FROM live.offer_timeline WHERE session_id=$1`,
		"live.claim_windows":  `SELECT count(*) FROM live.claim_windows WHERE session_id=$1`,
	} {
		if err := h.directMerchant(tokenB, h.f.storeB, func(tx pgx.Tx, _ platform.Scope) error {
			var n int
			if err := tx.QueryRow(h.ctx, q, s).Scan(&n); err != nil {
				return err
			}
			if n != 0 {
				return fmt.Errorf("RLS leak: tenant B sees %d %s rows of tenant A", n, label)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if lc, v := k3LifecycleRow(t, h, s); lc != "live" || v != 2 {
		t.Fatalf("cross-tenant attempts changed the session: %s v%d", lc, v)
	}
}

// TestK3LcB1EndWithoutWindow: start with open_window=false leaves no window row; end must still succeed
// (idempotent close, §9) and must NOT create a window row as a side effect; archive follows.
func TestK3LcB1EndWithoutWindow(t *testing.T) {
	h := lcSetup(t)
	s := h.draft(t, h.f.storeA1)
	no := false
	r, err := h.lifecycle(h.token, h.f.storeA1, s, "start", 1, &no)
	if err != nil || r.Lifecycle != "live" || r.Window.State != claims.WindowClosed {
		t.Fatalf("start open_window=false: %+v %v", r, err)
	}
	if r.Window.Version != 0 {
		t.Fatalf("start open_window=false created a window row: %+v", r.Window)
	}
	r, err = h.lifecycle(h.token, h.f.storeA1, s, "end", 2, nil)
	if err != nil || r.Lifecycle != "ended" {
		t.Fatalf("end without window: %+v %v", r, err)
	}
	if w := h.board(t, s).Window; w.Version != 0 {
		t.Fatalf("end without window created a window row: %+v", w)
	}
	if _, err := h.lifecycle(h.token, h.f.storeA1, s, "archive", 3, nil); err != nil {
		t.Fatalf("archive after end without window: %v", err)
	}
}

// TestK3LcB1ReopenNeverRacesPastEnd: regression loop — M2 reopen raced against end must never leave an
// ended/archived session with an OPEN window (the §9 reopen path is lifecycle start only, new generation).
func TestK3LcB1ReopenNeverRacesPastEnd(t *testing.T) {
	h := lcSetup(t)
	for iter := 0; iter < 6; iter++ {
		s := h.draft(t, h.f.storeA1)
		h.mustLifecycle(t, s, "start", 1)
		w := h.board(t, s).Window
		errs := k3Race(2, func(i int) error {
			if i == 0 {
				_, err := h.lifecycle(h.token, h.f.storeA1, s, "end", 2, nil)
				return err
			}
			_, err := h.setWindow(h.token, h.f.storeA1, s,
				claims.WindowInput{ExpectedVersion: w.Version, State: claims.WindowOpen, MatchMode: w.MatchMode})
			return err
		})
		if errs[0] != nil {
			t.Fatalf("iter %d: end: %v", iter, errs[0])
		}
		if errs[1] == nil {
			t.Fatalf("iter %d: M2 reopen raced past end (OPEN->OPEN or version CAS must refuse)", iter)
		}
		if !errors.Is(errs[1], command.ErrConflict) && !errors.Is(errs[1], claims.ErrInvalidTransition) {
			t.Fatalf("iter %d: M2 reopen error class: %v", iter, errs[1])
		}
		lc, _ := k3LifecycleRow(t, h, s)
		if lc != "ended" {
			t.Fatalf("iter %d: lifecycle %s after end", iter, lc)
		}
		if w := h.board(t, s).Window; w.State == claims.WindowOpen {
			t.Fatalf("iter %d: ended session with an OPEN window", iter)
		}
	}
}
