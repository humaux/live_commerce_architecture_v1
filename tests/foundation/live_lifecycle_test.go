// live_lifecycle_test.go is the LC-B1 author smoke (REAL_PG, MOCK manual ingress): migrations/0122_live_lifecycle.sql
// and live.Lifecycle (A7, contracts/live-console-v1.md §9), the §8 per-store OPEN-window cap and the §7.3
// live.offer_timeline writer. It is NOT the independent gate (test_worker writes LCN09/LCN-lifecycle from the contract).
// Isolation: lcSetup's principal; lcPurgeSessions deletes the sessions (offer_timeline rows cascade with their offers).
package foundation_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

func (h *lcHarness) lifecycle(token, store, session, action string, version int64, open *bool) (out live.LifecycleResult, err error) {
	err = h.do(token, store, func(tx pgx.Tx, s platform.Scope) (e error) {
		out, e = live.Lifecycle(h.ctx, tx, s, token, t04Key("lc-lifecycle"), session,
			live.LifecycleInput{Action: action, ExpectedVersion: version, OpenWindow: open})
		return e
	})
	return out, err
}

func (h *lcHarness) mustLifecycle(t *testing.T, session, action string, version int64) live.LifecycleResult {
	t.Helper()
	r, err := h.lifecycle(h.token, h.f.storeA1, session, action, version, nil)
	if err != nil {
		t.Fatalf("lifecycle %s from v%d: %v", action, version, err)
	}
	return r
}

// TestLiveLifecycleTransitions walks draft -> live -> ended -> live (new window generation) -> ended -> archived and
// every refused edge: stale version, invalid transition, archive read-only (draft edit and M2 open).
func TestLiveLifecycleTransitions(t *testing.T) {
	h := lcSetup(t)
	s := h.draft(t, h.f.storeA1)

	r := h.mustLifecycle(t, s, "start", 1)
	if r.Lifecycle != "live" || r.Version != 2 || r.Window.State != claims.WindowOpen || r.Window.Generation != 1 {
		t.Fatalf("start: %+v", r)
	}
	lcIs(t, lcErr(h.lifecycle(h.token, h.f.storeA1, s, "start", 2, nil)), claims.ErrInvalidTransition, "start while live")
	lcIs(t, lcErr(h.lifecycle(h.token, h.f.storeA1, s, "archive", 2, nil)), claims.ErrInvalidTransition, "archive while live")
	lcIs(t, lcErr(h.lifecycle(h.token, h.f.storeA1, s, "end", 1, nil)), command.ErrConflict, "stale version")
	lcIs(t, lcErr(h.lifecycle(h.token, h.f.storeA1, s, "pause", 2, nil)), command.ErrInvalid, "unknown action")

	r = h.mustLifecycle(t, s, "end", 2)
	if r.Lifecycle != "ended" || r.Version != 3 || r.Window.State != claims.WindowClosed || r.Window.ClosedAt == nil {
		t.Fatalf("end: %+v", r)
	}
	// An ended session cannot be reopened through M2, only through lifecycle start (new generation).
	lcIs(t, lcErr(h.setWindow(h.token, h.f.storeA1, s, claims.WindowInput{ExpectedVersion: r.Window.Version, State: claims.WindowOpen, MatchMode: r.Window.MatchMode})),
		claims.ErrInvalidTransition, "M2 open on ended session")
	r = h.mustLifecycle(t, s, "start", 3)
	if r.Lifecycle != "live" || r.Window.State != claims.WindowOpen || r.Window.Generation != 2 {
		t.Fatalf("restart: %+v", r)
	}
	r = h.mustLifecycle(t, s, "end", 4)
	r = h.mustLifecycle(t, s, "archive", 5)
	if r.Lifecycle != "archived" || r.Version != 6 || r.Window.State != claims.WindowClosed {
		t.Fatalf("archive: %+v", r)
	}
	lcIs(t, lcErr(h.lifecycle(h.token, h.f.storeA1, s, "start", 6, nil)), claims.ErrInvalidTransition, "start after archive")
	// Archived = read-only: the draft cannot be edited and the window cannot be opened.
	err := h.do(h.token, h.f.storeA1, func(tx pgx.Tx, sc platform.Scope) error {
		_, e := live.UpdateDraft(h.ctx, tx, sc, h.token, t04Key("lc-archived-edit"), s, 1, live.DraftInput{Title: "edited " + t04Tag(), AspectRatio: "9:16"})
		return e
	})
	if err == nil {
		t.Fatal("archived draft was editable")
	}
	lcIs(t, lcErr(h.setWindow(h.token, h.f.storeA1, s, claims.WindowInput{ExpectedVersion: r.Window.Version, State: claims.WindowOpen, MatchMode: r.Window.MatchMode})),
		claims.ErrInvalidTransition, "M2 open on archived session")
}

// TestLiveLifecycleReplayAndAuthority: the same Idempotency-Key replays the stored result without a second transition;
// live:read alone and another store's token are refused; a draft session may start with open_window=false.
func TestLiveLifecycleReplayAndAuthority(t *testing.T) {
	h := lcSetup(t)
	s := h.draft(t, h.f.storeA1)
	key := t04Key("lc-lifecycle-replay")
	var first, second live.LifecycleResult
	for i, dst := range []*live.LifecycleResult{&first, &second} {
		if err := h.do(h.token, h.f.storeA1, func(tx pgx.Tx, sc platform.Scope) (e error) {
			*dst, e = live.Lifecycle(h.ctx, tx, sc, h.token, key, s, live.LifecycleInput{Action: "start", ExpectedVersion: 1})
			return e
		}); err != nil {
			t.Fatalf("start #%d: %v", i, err)
		}
	}
	if first.Version != 2 || second.Version != 2 || second.Lifecycle != "live" {
		t.Fatalf("replay: %+v %+v", first, second)
	}
	_, readToken := lcPrincipal(t, h.f, h.f.tenantA, []string{h.f.storeA1}, "store:read", "live:read")
	lcIs(t, lcErr(h.lifecycle(readToken, h.f.storeA1, s, "end", 2, nil)), platform.ErrForbidden, "live:read ends a session")
	// Session of store A1 addressed through store A2 -> not found (I01).
	lcIs(t, lcErr(h.lifecycle(h.token, h.f.storeA2, s, "end", 2, nil)), command.ErrNotFound, "cross-store lifecycle")

	quiet := h.draft(t, h.f.storeA1)
	no := false
	r, err := h.lifecycle(h.token, h.f.storeA1, quiet, "start", 1, &no)
	if err != nil || r.Lifecycle != "live" || r.Window.State != claims.WindowClosed {
		t.Fatalf("start open_window=false: %+v %v", r, err)
	}
}

// TestLiveLifecycleMultiWindow is LCN09: two sessions OPEN at once in one store, the same keyword claimed in each
// lands only in the URL session, closing one leaves the other OPEN, and the sixth OPEN window is refused.
func TestLiveLifecycleMultiWindow(t *testing.T) {
	h := lcSetup(t)
	sku := lcSKUs(t, h.f, h.f.tenantA, h.f.storeA1, "USD", 1)[0]
	a, b := h.draft(t, h.f.storeA1), h.draft(t, h.f.storeA1)
	h.offer(t, a, "MWKEY", sku, 3)
	h.offer(t, b, "MWKEY", sku, 3)
	ra := h.mustLifecycle(t, a, "start", 1)
	rb := h.mustLifecycle(t, b, "start", 1)
	if ra.Window.State != claims.WindowOpen || rb.Window.State != claims.WindowOpen {
		t.Fatalf("two OPEN windows: %+v %+v", ra.Window, rb.Window)
	}
	h.accepted(t, a, "", "amy", "MWKEY+1")
	if n := h.bundleCount(t, a); n != 1 {
		t.Fatalf("session a bundles %d", n)
	}
	if n := h.bundleCount(t, b); n != 0 {
		t.Fatalf("manual claim leaked into session b: %d bundles", n)
	}
	h.mustLifecycle(t, a, "end", 2)
	if w := h.board(t, b).Window; w.State != claims.WindowOpen {
		t.Fatalf("closing a closed b: %+v", w)
	}
	// Cap: b + 4 more = 5 OPEN; the sixth is refused with too_many_open_windows and leaves no window row.
	extra := make([]string, 0, 5)
	for i := 0; i < 4; i++ {
		s := h.draft(t, h.f.storeA1)
		h.mustLifecycle(t, s, "start", 1)
		extra = append(extra, s)
	}
	sixth := h.draft(t, h.f.storeA1)
	lcIs(t, lcErr(h.lifecycle(h.token, h.f.storeA1, sixth, "start", 1, nil)), claims.ErrTooManyOpenWindows, "sixth OPEN window")
	lcIs(t, lcErr(h.setWindow(h.token, h.f.storeA1, sixth, claims.WindowInput{State: claims.WindowOpen, MatchMode: claims.MatchExact})), claims.ErrTooManyOpenWindows, "sixth via M2")
	if h.board(t, sixth).Window.Version != 0 {
		t.Fatal("refused open left a window row")
	}
	// The refused start rolled back with its receipt, so the session is still a draft at version 1.
	h.mustLifecycle(t, extra[0], "end", 2)
	if r := h.mustLifecycle(t, sixth, "start", 1); r.Window.State != claims.WindowOpen {
		t.Fatalf("sixth after a close: %+v", r)
	}
}

func (h *lcHarness) bundleCount(t *testing.T, session string) int {
	t.Helper()
	var n int
	if err := h.f.owner.QueryRow(h.ctx, `SELECT count(*) FROM claims.bundles WHERE session_id=$1`, session).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestLiveLifecycleCapUnderRace: eight concurrent starts on eight fresh sessions -> exactly five win (the store-level
// advisory lock is taken before the count).
func TestLiveLifecycleCapUnderRace(t *testing.T) {
	h := lcSetup(t)
	const n = 8
	sessions := make([]string, n)
	for i := range sessions {
		sessions[i] = h.draft(t, h.f.storeA1)
	}
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range sessions {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = h.lifecycle(h.token, h.f.storeA1, sessions[i], "start", 1, nil)
		}(i)
	}
	wg.Wait()
	won, refused := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case errors.Is(err, claims.ErrTooManyOpenWindows):
			refused++
		default:
			t.Fatalf("unexpected race error: %v", err)
		}
	}
	if won != claims.MaxOpenWindowsPerStore || refused != n-won {
		t.Fatalf("won=%d refused=%d", won, refused)
	}
}

// TestLiveLifecycleOfferTimeline: RecordOfferFeatured appends rows (never updates); the runtime role cannot UPDATE or
// DELETE the table; an offer of another session is refused.
func TestLiveLifecycleOfferTimeline(t *testing.T) {
	h := lcSetup(t)
	sku := lcSKUs(t, h.f, h.f.tenantA, h.f.storeA1, "USD", 2)
	a, b := h.draft(t, h.f.storeA1), h.draft(t, h.f.storeA1)
	oa := h.offer(t, a, "TLA", sku[0], 2)
	ob := h.offer(t, b, "TLB", sku[1], 2)
	for i := 0; i < 2; i++ {
		if err := h.do(h.token, h.f.storeA1, func(tx pgx.Tx, sc platform.Scope) error {
			_, e := live.RecordOfferFeatured(h.ctx, tx, sc, h.token, a, oa.ID)
			return e
		}); err != nil {
			t.Fatalf("RecordOfferFeatured #%d: %v", i, err)
		}
	}
	var n int
	if err := h.f.owner.QueryRow(h.ctx, `SELECT count(*) FROM live.offer_timeline WHERE session_id=$1 AND offer_id=$2 AND kind='featured'`, a, oa.ID).Scan(&n); err != nil || n != 2 {
		t.Fatalf("timeline rows %d %v", n, err)
	}
	lcIs(t, h.do(h.token, h.f.storeA1, func(tx pgx.Tx, sc platform.Scope) error {
		_, e := live.RecordOfferFeatured(h.ctx, tx, sc, h.token, a, ob.ID)
		return e
	}), command.ErrNotFound, "offer of another session")
	for _, q := range []string{`UPDATE live.offer_timeline SET kind='featured'`, `DELETE FROM live.offer_timeline`} {
		if err := h.directMerchant(h.token, h.f.storeA1, func(tx pgx.Tx, _ platform.Scope) error {
			_, e := tx.Exec(h.ctx, q)
			return e
		}); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("%s must be denied: %v", q, err)
		}
	}
}

// TestLiveLifecycleNoSingleOpenWindowQuery is the LCN09 grep gate: outside the three accepted store-wide EXISTS/count
// readers, no production Go query selects OPEN claim windows without naming session_id (a "the current OPEN window"
// query would be wrong once several windows are OPEN).
func TestLiveLifecycleNoSingleOpenWindowQuery(t *testing.T) {
	root := filepath.Join("..", "..", "internal")
	query := regexp.MustCompile("(?s)`[^`]*claim_windows[^`]*`")
	allowed := map[string]bool{ // store-wide by design (contract §8.2): keyword-edit guard, cap count (takes a session exclusion)
		filepath.Join(root, "catalog", "document.go"): true,
		filepath.Join(root, "claims", "merchant.go"):  true,
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || allowed[path] {
			return err
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for _, q := range query.FindAllString(string(src), -1) {
			if strings.Contains(q, "'OPEN'") && !strings.Contains(q, "session_id") {
				t.Errorf("%s: OPEN claim_windows query without session_id: %.120s", path, q)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
