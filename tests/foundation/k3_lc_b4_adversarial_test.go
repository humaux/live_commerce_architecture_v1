package foundation_test

// Independent adversarial acceptance of unit LC-B4 (live-console sends + takeover, migration 0128), K3 round.
// These tests were written by a reviewer who did NOT author the unit; they try to break the implementation against
// contracts/live-console-v1.md §3.3-3.6, §4 and Amendment 1 (A1.1 120 s confirm gate, A1.3 advisory locks, A1.4.4
// redacted UNKNOWN reconcile, P2-3 effective takeover generation). New tests only: no product code, migration or
// existing test was changed. Harness: live_console_send_test.go (lbEnv, fake Graph counting every HTTP request).
// Evidence class of every assertion here: MOCK (fake Graph) over real PostgreSQL.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/inbox"
	"livecommerce/internal/platform"
)

// k3SendDMAs plans one DM as an arbitrary merchant token.
func (e *lbEnv) k3SendDMAs(token, conv, text string, gen int64) (inbox.SendOutput, error) {
	var out inbox.SendOutput
	err := platform.WithScope(context.Background(), e.h.f.runtime, token, e.h.f.storeA1, "inbox:reply", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = e.svc.SendDM(context.Background(), tx, s, lbKey(), conv, inbox.DMInput{TextInput: inbox.TextInput{Text: text}, ExpectedGeneration: &gen}, nil)
		return err
	})
	return out, err
}

// Idempotency-Key: replay returns the same operation and never re-plans; the same key with a different body is refused;
// the receipt still replays after the send succeeded (no second POST).
func TestK3LCB4IdempotencyReplayAndConflict(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	key := lbKey()
	send := func(k, text string) (inbox.SendOutput, error) {
		var out inbox.SendOutput
		err := platform.WithScope(context.Background(), f.runtime, e.h.token, f.storeA1, "inbox:reply", func(tx pgx.Tx, s platform.Scope) error {
			var err error
			gen := int64(0)
			out, err = e.svc.SendDM(context.Background(), tx, s, k, conv, inbox.DMInput{TextInput: inbox.TextInput{Text: text}, ExpectedGeneration: &gen}, nil)
			return err
		})
		return out, err
	}
	ops := func() int64 {
		return miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND action='meta.dm_send' AND request->>'conversation_id'=$2`, f.tenantA, conv)
	}
	first, err := send(key, "idem-body")
	if err != nil {
		t.Fatal(err)
	}
	again, err := send(key, "idem-body")
	if err != nil || again.OperationID != first.OperationID {
		t.Fatalf("same key + same body must replay the same operation: %+v %v (first %s)", again, err, first.OperationID)
	}
	if n := ops(); n != 1 {
		t.Fatalf("replay planned %d operations", n)
	}
	if _, err := send(key, "idem-body-CHANGED"); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("same key + different body must be a conflict: %v", err)
	}
	if n := ops(); n != 1 {
		t.Fatalf("conflict planned %d operations", n)
	}
	e.run(t, first.OperationID)
	e.awaitOp(t, first.OperationID, "SUCCEEDED", 10*time.Second, "completed")
	if _, err := send(key, "idem-body"); err != nil {
		t.Fatalf("receipt replay after SUCCEEDED: %v", err)
	}
	if n := ops(); n != 1 {
		t.Fatalf("post-success replay planned %d operations", n)
	}
	if n := e.g.count(); n != 1 {
		t.Fatalf("%d Graph POSTs for one operation", n)
	}
}

// A1.3: the lcn-dup / lcn-rec advisory locks serialise same-body sends before the duplicate read: N concurrent identical
// sends produce exactly ONE operation; the rest are duplicate_recent. (Two workers / double click.)
func TestK3LCB4ConcurrentDuplicateRecent(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f

	// 8 concurrent identical DMs (different Idempotency-Keys): exactly one plans.
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	if _, err := e.sendDM(conv, "setup", 0); err != nil { // gen 1, assignee = actor
		t.Fatal(err)
	}
	const n = 8
	var wg sync.WaitGroup
	outs := make([]inbox.SendOutput, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			outs[i], errs[i] = e.sendDM(conv, "dup-body", 1)
		}(i)
	}
	wg.Wait()
	planned, dup := 0, 0
	for i := range errs {
		switch {
		case errs[i] == nil && outs[i].OperationID != "":
			planned++
		case planCode(errs[i]) == "duplicate_recent":
			dup++
		default:
			t.Fatalf("goroutine %d: %+v %v", i, outs[i], errs[i])
		}
	}
	if planned != 1 || dup != n-1 {
		t.Fatalf("concurrent identical DMs: planned=%d duplicate_recent=%d", planned, dup)
	}
	if c := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND action='meta.dm_send' AND request->>'conversation_id'=$2`, f.tenantA, conv); c != 2 {
		t.Fatalf("dm operations for the conversation: %d (setup + one winner)", c)
	}

	// A stale expected_version is refused before anything exists.
	if err := e.scoped(t, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		_, err := e.svc.PlanRecommend(ctx, tx, s, e.session, e.offer.ID, e.offer.Version+1)
		return err
	}); planCode(err) != "version_conflict" {
		t.Fatalf("stale offer version: %v", err)
	}
	// 8 concurrent recommends of one offer: exactly one plans, the rest duplicate_recent (lcn-rec).
	recs := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = e.scoped(t, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
				var err error
				recs[i], err = e.svc.PlanRecommend(ctx, tx, s, e.session, e.offer.ID, e.offer.Version)
				errs[i] = err
				return err
			})
		}(i)
	}
	wg.Wait()
	planned, dup = 0, 0
	for i := range errs {
		switch {
		case errs[i] == nil && recs[i] != "":
			planned++
		case planCode(errs[i]) == "duplicate_recent":
			dup++
		default:
			t.Fatalf("recommend %d: %q %v", i, recs[i], errs[i])
		}
	}
	if planned != 1 || dup != n-1 {
		t.Fatalf("concurrent recommends: planned=%d duplicate_recent=%d", planned, dup)
	}

	// 2 concurrent identical public replies on one comment: exactly one plans.
	ref := lbRef()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			outs[i], errs[i] = e.publicReply(ref, "same-public-body")
		}(i)
	}
	wg.Wait()
	planned, dup = 0, 0
	for i := 0; i < 2; i++ {
		switch {
		case errs[i] == nil && outs[i].OperationID != "":
			planned++
		case planCode(errs[i]) == "duplicate_recent":
			dup++
		default:
			t.Fatalf("public reply %d: %+v %v", i, outs[i], errs[i])
		}
	}
	if planned != 1 || dup != 1 {
		t.Fatalf("concurrent public replies: planned=%d duplicate_recent=%d", planned, dup)
	}
	if c := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE action='meta.public_reply' AND request->>'comment_ref'=$1`, ref); c != 1 {
		t.Fatalf("public reply operations for the comment: %d", c)
	}
}

// A1.1: the 120 s confirm gate fires only for a fresh comment while a window is OPEN; confirm_preempt_auto must be
// exactly true or absent; the preempt audit is written only for a confirmed pre-emption.
func TestK3LCB4ConfirmPreemptAutoGate(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	base := e.auditCount(t, "inbox.private_reply.preempt_confirmed")
	text := func(s string) string { return s + "-" + t04Tag() }
	preempts := func() int64 { return e.auditCount(t, "inbox.private_reply.preempt_confirmed") - base }

	// Fresh comment + OPEN window, flag absent: refused, no audit, no operation.
	ref := lbRef()
	if _, err := e.manual(ref, text("gate"), time.Now(), false); planCode(err) != "auto_pending_confirm" {
		t.Fatalf("fresh comment without the flag: %v", err)
	}
	if preempts() != 0 {
		t.Fatal("preempt audited for a refused request")
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE request->>'comment_ref'=$1`, ref); n != 0 {
		t.Fatalf("refused gate left %d operations", n)
	}
	// confirm_preempt_auto=false is invalid (A1.1: only true or absent).
	no := false
	err := platform.WithScope(context.Background(), f.runtime, e.h.token, f.storeA1, "inbox:reply", func(tx pgx.Tx, s platform.Scope) error {
		_, err := e.svc.SendPrivateReply(context.Background(), tx, s, lbKey(), e.session, lbRef(),
			inbox.ReplyInput{TextInput: inbox.TextInput{Text: "x"}, ConfirmPreemptAuto: &no},
			func(context.Context) (time.Time, error) { return time.Now(), nil })
		return err
	})
	if !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("confirm_preempt_auto=false: %v", err)
	}
	// confirm=true on the same fresh comment: plans, exactly one audit.
	if _, err := e.manual(ref, text("gate"), time.Now(), true); err != nil {
		t.Fatalf("confirmed pre-emption: %v", err)
	}
	if preempts() != 1 {
		t.Fatalf("preempt audits=%d after the confirmed reply", preempts())
	}
	// Older than 120 s: no gate even with the flag set, no audit.
	if _, err := e.manual(lbRef(), text("old"), time.Now().Add(-10*time.Minute), true); err != nil {
		t.Fatalf("old comment: %v", err)
	}
	if preempts() != 1 {
		t.Fatal("audit written for a comment outside the 120 s window")
	}
	// Window CLOSED + fresh comment: no gate on Messenger (plans without any window), no audit.
	mustExec(t, f.owner, `UPDATE live.claim_windows SET state='CLOSED',closed_at=clock_timestamp() WHERE session_id=$1 AND state='OPEN'`, e.session)
	if _, err := e.manual(lbRef(), text("closed"), time.Now(), false); err != nil {
		t.Fatalf("fresh comment with a closed window: %v", err)
	}
	if preempts() != 1 {
		t.Fatal("audit written while no window was open")
	}
}

// P2-3: Check compares the frozen takeover generation against the EFFECTIVE generation (stored+1 while human_until is
// lazily expired): frozen at the stale generation -> takeover_changed with zero HTTP; frozen at the effective one -> sent.
func TestK3LCB4TakeoverExpiryAutoReplyCheck(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f

	// (a) Planned as known at generation 1; the human ownership then expires (lazy, no write): effective gen 2.
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	a := e.autoOp(t)
	e.linkPeer(t, a.bundleID, conv, a.op)
	if _, err := e.takeover(t, conv, 0, false); err != nil { // gen 1, human, +6 h
		t.Fatal(err)
	}
	e.freezeRequest(t, a.op, `{"conversation_known":true,"takeover_generation":1}`)
	mustExec(t, f.owner, `UPDATE inbox.conversation_state SET human_until=clock_timestamp()-interval '1 second' WHERE conversation_id=$1`, conv)
	before := e.g.count()
	e.run(t, a.op)
	if code, _ := e.awaitOp(t, a.op, "BLOCKED_POLICY", 10*time.Second, "completed"); code != "takeover_changed" || e.g.count() != before {
		t.Fatalf("frozen at the stale generation: code=%s graph %d -> %d", code, before, e.g.count())
	}
	if e.secretCount(t, a.op) != 0 {
		t.Fatal("send secret survived BLOCKED_POLICY")
	}
	if st := e.state(t, conv); st.Mode != "human" {
		t.Fatalf("the lazy expiry must not write the row at check time (check is lock-free): %+v", st)
	}

	// (b) Frozen at the EFFECTIVE generation (stored 1 + 1 while expired): the same check passes and the reply is sent.
	conv2 := e.postDM(t, mciDigits(15), "hello", time.Now())
	b := e.autoOp(t)
	e.linkPeer(t, b.bundleID, conv2, b.op)
	if _, err := e.takeover(t, conv2, 0, false); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `UPDATE inbox.conversation_state SET human_until=clock_timestamp()-interval '1 second' WHERE conversation_id=$1`, conv2)
	e.freezeRequest(t, b.op, `{"conversation_known":true,"takeover_generation":2}`)
	e.run(t, b.op)
	e.awaitOp(t, b.op, "SUCCEEDED", 10*time.Second, "completed")
}

// §3.6: two staff members race an explicit takeover on the same generation — exactly one wins the CAS; the loser is
// takeover_changed; a DM by the non-assignee (with the current generation) reassigns the thread (gen+1, audit).
func TestK3LCB4TakeoverConcurrentTwoStaff(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	pid2, tok2 := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inbox:read", "inbox:reply")
	base := e.auditCount(t, "inbox.takeover")

	takeoverAs := func(token string) (inbox.TakeoverOutput, error) {
		var out inbox.TakeoverOutput
		err := platform.WithScope(context.Background(), f.runtime, token, f.storeA1, "inbox:reply", func(tx pgx.Tx, s platform.Scope) error {
			var err error
			out, err = e.svc.Takeover(context.Background(), tx, conv, inbox.TakeoverInput{ExpectedGeneration: 0})
			return err
		})
		return out, err
	}
	toks := []string{e.h.token, tok2}
	pids := []string{e.h.actor, pid2}
	var wg sync.WaitGroup
	outs := make([]inbox.TakeoverOutput, 2)
	errs := make([]error, 2)
	for i := range toks {
		wg.Add(1)
		go func(i int) { defer wg.Done(); outs[i], errs[i] = takeoverAs(toks[i]) }(i)
	}
	wg.Wait()
	winner := -1
	for i := range errs {
		switch {
		case errs[i] == nil && outs[i].TakeoverGeneration == 1 && outs[i].Mode == "human":
			if winner != -1 {
				t.Fatalf("both takeovers won the CAS: %+v / %+v", outs[0], outs[1])
			}
			winner = i
		case planCode(errs[i]) == "takeover_changed":
		default:
			t.Fatalf("takeover %d: %+v %v", i, outs[i], errs[i])
		}
	}
	if winner == -1 {
		t.Fatalf("no takeover won: %v / %v", errs[0], errs[1])
	}
	loser := 1 - winner
	if st := e.state(t, conv); st.Mode != "human" || st.Assignee == nil || *st.Assignee != pids[winner] || st.Gen != 1 {
		t.Fatalf("state after the race: %+v (winner %s)", st, pids[winner])
	}

	// The loser (non-assignee) DMs with the CURRENT generation: reassignment, generation 2, one more takeover audit.
	dm, err := e.k3SendDMAs(toks[loser], conv, "reassign", 1)
	if err != nil || dm.TakeoverGeneration == nil || *dm.TakeoverGeneration != 2 {
		t.Fatalf("reassignment DM: %+v %v", dm, err)
	}
	if st := e.state(t, conv); st.Mode != "human" || st.Assignee == nil || *st.Assignee != pids[loser] || st.Gen != 2 {
		t.Fatalf("state after the reassignment: %+v", st)
	}
	if n := e.auditCount(t, "inbox.takeover") - base; n != 2 {
		t.Fatalf("takeover audits=%d, want the explicit takeover + the reassignment", n)
	}
	// A DM by the old assignee with the stale generation is refused.
	if _, err := e.k3SendDMAs(toks[winner], conv, "stale", 1); planCode(err) != "takeover_changed" {
		t.Fatalf("stale generation after the reassignment: %v", err)
	}
}

// Tenant/store scope comes from the server-side session only: a conversation of storeA1 does not exist for a sister
// store or another tenant — planning refuses with zero operations and the new read definers return nothing.
func TestK3LCB4CrossTenantStoreIsolation(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	_, tokA2 := lcPrincipal(t, f, f.tenantA, []string{f.storeA2}, "store:read", "inbox:read", "inbox:reply")
	_, tokB := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, "store:read", "inbox:read", "inbox:reply")
	for _, tc := range []struct{ name, token, store string }{
		{"sister store", tokA2, f.storeA2},
		{"other tenant", tokB, f.storeB},
	} {
		err := platform.WithScope(context.Background(), f.runtime, tc.token, tc.store, "inbox:reply", func(tx pgx.Tx, s platform.Scope) error {
			gen := int64(0)
			_, err := e.svc.SendDM(context.Background(), tx, s, lbKey(), conv, inbox.DMInput{TextInput: inbox.TextInput{Text: "cross"}, ExpectedGeneration: &gen}, nil)
			return err
		})
		if err == nil {
			t.Fatalf("%s: a foreign conversation planned a DM", tc.name)
		}
		err = platform.WithScope(context.Background(), f.runtime, tc.token, tc.store, "inbox:read", func(tx pgx.Tx, s platform.Scope) error {
			_, err := e.svc.ReadThread(context.Background(), tx, conv, nil, 50)
			return err
		})
		if err == nil {
			t.Fatalf("%s: a foreign conversation read back", tc.name)
		}
		var heads int64
		err = platform.WithScope(context.Background(), f.runtime, tc.token, tc.store, "inbox:read", func(tx pgx.Tx, s platform.Scope) error {
			return tx.QueryRow(context.Background(), `SELECT count(*) FROM social.conversation_heads(ARRAY[$1]::uuid[])`, conv).Scan(&heads)
		})
		if err != nil || heads != 0 {
			t.Fatalf("%s: conversation_heads leaked %d rows (err=%v)", tc.name, heads, err)
		}
		var outbound int64
		err = platform.WithScope(context.Background(), f.runtime, tc.token, tc.store, "inbox:read", func(tx pgx.Tx, s platform.Scope) error {
			return tx.QueryRow(context.Background(), `SELECT count(*) FROM inbox.read_outbound($1, 10)`, conv).Scan(&outbound)
		})
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "PT404" {
			t.Fatalf("%s: read_outbound of a foreign conversation: %v", tc.name, err)
		}
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE request->>'conversation_id'=$1`, conv); n != 0 {
		t.Fatalf("cross-scope planning left %d operations", n)
	}
}

// Permission split (definer-level principal_holds, not just the HTTP gate): inbox:reply plans DM/private/public;
// recommend needs live:manage AND inbox:reply; a live:manage-only principal is PT403 on every send planner.
func TestK3LCB4PermissionSplit(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	// live:manage + inbox:read but NOT inbox:reply (inbox:read so the DM reaches the planner gate past read_thread).
	pidLM, tokLM := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "live:manage", "inbox:read")
	pidIR, tokIR := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inbox:read", "inbox:reply")

	asLM := func(fn func(ctx context.Context, tx pgx.Tx, s platform.Scope) error) error {
		return platform.WithScope(context.Background(), f.runtime, tokLM, f.storeA1, "live:manage", func(tx pgx.Tx, s platform.Scope) error {
			return fn(context.Background(), tx, s)
		})
	}
	if err := asLM(func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		gen := int64(0)
		_, err := e.svc.SendDM(ctx, tx, s, lbKey(), conv, inbox.DMInput{TextInput: inbox.TextInput{Text: "no-reply-perm"}, ExpectedGeneration: &gen}, nil)
		return err
	}); planCode(err) != "forbidden" {
		t.Fatalf("plan_dm without inbox:reply: %v", err)
	}
	if err := asLM(func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		_, err := e.svc.SendPrivateReply(ctx, tx, s, lbKey(), e.session, lbRef(), inbox.ReplyInput{TextInput: inbox.TextInput{Text: "x"}},
			func(context.Context) (time.Time, error) { return time.Now().Add(-10 * time.Minute), nil })
		return err
	}); planCode(err) != "forbidden" {
		t.Fatalf("plan_manual_private_reply without inbox:reply: %v", err)
	}
	if err := asLM(func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		_, err := e.svc.SendPublicReply(ctx, tx, s, lbKey(), e.session, lbRef(), inbox.TextInput{Text: "x"})
		return err
	}); planCode(err) != "forbidden" {
		t.Fatalf("plan_public_reply without inbox:reply: %v", err)
	}
	if err := asLM(func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		_, err := e.svc.PlanRecommend(ctx, tx, s, e.session, e.offer.ID, e.offer.Version)
		return err
	}); planCode(err) != "forbidden" {
		t.Fatalf("plan_offer_recommend without inbox:reply: %v", err)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE principal_id=$1`, pidLM); n != 0 {
		t.Fatalf("the live:manage-only principal left %d operations", n)
	}

	// inbox:reply without live:manage: recommend refuses (needs BOTH), a DM plans (no over-denial).
	if err := platform.WithScope(context.Background(), f.runtime, tokIR, f.storeA1, "inbox:reply", func(tx pgx.Tx, s platform.Scope) error {
		_, err := e.svc.PlanRecommend(context.Background(), tx, s, e.session, e.offer.ID, e.offer.Version)
		return err
	}); planCode(err) != "forbidden" {
		t.Fatalf("plan_offer_recommend without live:manage: %v", err)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE principal_id=$1`, pidIR); n != 0 {
		t.Fatalf("the refused recommend left %d operations", n)
	}
	if _, err := e.k3SendDMAs(tokIR, conv, "reply-only is enough for a DM", 0); err != nil {
		t.Fatalf("inbox:reply alone must plan a DM: %v", err)
	}
}

// Capability derivation (0125 binding_capability_state via the meta_connections snapshot): a reauth_required connection
// refuses every planner with zero new operations, and an operation planned before the sever dies at Check with zero HTTP.
func TestK3LCB4CapabilityDenyPlanAndCheck(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	out, err := e.sendDM(conv, "planned-before-sever", 0)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `UPDATE integration.meta_connections SET status='reauth_required', updated_at=clock_timestamp()
		WHERE tenant_id=$1 AND store_id=$2 AND page_id=$3`, f.tenantA, f.storeA1, e.pageID)

	// Check-time: BLOCKED_POLICY capability, zero Graph calls, dispatch copy wiped.
	before := e.g.count()
	e.run(t, out.OperationID)
	if code, _ := e.awaitOp(t, out.OperationID, "BLOCKED_POLICY", 10*time.Second, "completed"); code != "capability" || e.g.count() != before {
		t.Fatalf("check-time capability: code=%s graph %d -> %d", code, before, e.g.count())
	}
	if e.secretCount(t, out.OperationID) != 0 {
		t.Fatal("send secret survived the capability denial")
	}

	// Plan-time: all four kinds refuse; the operation count does not move.
	opsBefore := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE tenant_id=$1`, f.tenantA)
	if _, err := e.sendDM(conv, "after-sever", 1); planCode(err) != "capability" {
		t.Fatalf("DM while severed: %v", err)
	}
	if _, err := e.manual(lbRef(), "after-sever", time.Now().Add(-10*time.Minute), false); planCode(err) != "capability" {
		t.Fatalf("manual reply while severed: %v", err)
	}
	if _, err := e.publicReply(lbRef(), "after-sever"); planCode(err) != "capability" {
		t.Fatalf("public reply while severed: %v", err)
	}
	if err := e.scoped(t, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		_, err := e.svc.PlanRecommend(ctx, tx, s, e.session, e.offer.ID, e.offer.Version)
		return err
	}); planCode(err) != "capability" {
		t.Fatalf("recommend while severed: %v", err)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE tenant_id=$1`, f.tenantA); n != opsBefore {
		t.Fatalf("refused planning changed the operation count %d -> %d", opsBefore, n)
	}
}

// §4.3 Check: a human send whose principal lost inbox:reply between plan and dispatch dies BLOCKED_POLICY
// principal_revoked with zero HTTP (the grant is restored on exit; the fixture is shared).
func TestK3LCB4PrincipalRevokedAtCheck(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	out, err := e.sendDM(conv, "revoke-me", 0)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='inbox:reply'`, f.tenantA, f.storeA1, e.h.actor)
	defer mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'inbox:reply') ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, e.h.actor)
	before := e.g.count()
	e.run(t, out.OperationID)
	if code, _ := e.awaitOp(t, out.OperationID, "BLOCKED_POLICY", 10*time.Second, "completed"); code != "principal_revoked" || e.g.count() != before {
		t.Fatalf("revoked principal at check: code=%s graph %d -> %d", code, before, e.g.count())
	}
	if e.secretCount(t, out.OperationID) != 0 {
		t.Fatal("send secret survived principal_revoked")
	}
}

// A1.4.4: a redacted UNKNOWN operation (request minus conversation_id/peer_key/comment_ref plus redacted:true, key
// renamed <prefix>-purged:) reconciles to UNKNOWN with ZERO Graph requests — the redacted body can never be re-POSTed.
func TestK3LCB4RedactedUnknownReconcileZeroHTTP(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	out, err := e.sendDM(conv, "redact-me", 0)
	if err != nil {
		t.Fatal(err)
	}
	if e.secretCount(t, out.OperationID) != 1 {
		t.Fatal("no dispatch copy before UNKNOWN")
	}
	// Post-dispatch ambiguity, then the A1.4.4 C4 redaction (owner SQL, as the retention job does).
	mustExec(t, f.owner, `UPDATE integration.operations SET state='UNKNOWN', generation=1, result_code='graph_unconfirmed' WHERE id=$1`, out.OperationID)
	if e.secretCount(t, out.OperationID) != 0 {
		t.Fatal("the wipe trigger did not fire on the UNKNOWN transition")
	}
	mustExec(t, f.owner, `UPDATE integration.operations
		SET request=(request - 'conversation_id' - 'comment_ref' - 'peer_key') || '{"redacted":true}'::jsonb,
		    semantic_key='mdm-purged:'||id::text
		WHERE id=$1`, out.OperationID)
	before := e.g.count()
	e.run(t, out.OperationID)
	code, _ := e.awaitOp(t, out.OperationID, "UNKNOWN", 25*time.Second, "cancelled")
	if code != "reconcile_budget_exhausted" {
		t.Fatalf("redacted UNKNOWN final code %s", code)
	}
	if n := e.g.count() - before; n != 0 {
		t.Fatalf("a redacted UNKNOWN operation made %d Graph requests during reconcile", n)
	}
	var key string
	var redacted bool
	if err := f.owner.QueryRow(context.Background(), `SELECT semantic_key, coalesce((request->>'redacted')::boolean,false) FROM integration.operations WHERE id=$1`, out.OperationID).Scan(&key, &redacted); err != nil {
		t.Fatal(err)
	}
	if !redacted || !strings.HasPrefix(key, "mdm-purged:") {
		t.Fatalf("redaction markers lost: key=%s redacted=%v", key, redacted)
	}
	if st := e.state(t, conv); st.LastOut != nil {
		t.Fatalf("a redacted reconcile advanced last_outbound_at: %+v", st)
	}
}

// §3.4/C7: the wipe trigger covers every path into a terminal state (CANCELLED included, even though no v1 send action
// reaches it), and the worker definers are lease-fenced: a wrong token or generation never releases the sealed copy.
func TestK3LCB4SecretWipeCancelledAndLeaseFence(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	out, err := e.sendDM(conv, "cancel-me", 0)
	if err != nil {
		t.Fatal(err)
	}
	if e.secretCount(t, out.OperationID) != 1 {
		t.Fatal("no dispatch copy")
	}
	mustExec(t, f.owner, `UPDATE integration.operations SET state='CANCELLED', generation=1, result_code='operator_cancel' WHERE id=$1`, out.OperationID)
	if e.secretCount(t, out.OperationID) != 0 {
		t.Fatal("send secret survived CANCELLED")
	}

	// Lease fencing on load_send_secret / finish_send (40001 on any mismatch).
	out2, err := e.sendDM(conv, "fence-me", 1)
	if err != nil {
		t.Fatal(err)
	}
	token := randomBytes(32)
	mustExec(t, f.owner, `UPDATE integration.operations SET state='DISPATCHING', generation=1, lease_mode='dispatch',
		lease_until=clock_timestamp()+interval '5 minutes', lease_token_hash=sha256($2) WHERE id=$1`, out2.OperationID, token)
	for _, q := range []struct {
		name, sql string
		args      []any
	}{
		{"load wrong token", `SELECT count(*) FROM inbox.load_send_secret($1,1,$2)`, []any{out2.OperationID, randomBytes(32)}},
		{"load wrong generation", `SELECT count(*) FROM inbox.load_send_secret($1,2,$2)`, []any{out2.OperationID, token}},
		{"finish wrong token", `SELECT inbox.finish_send($1,1,$2,'SUCCEEDED',NULL)`, []any{out2.OperationID, randomBytes(32)}},
		{"finish wrong generation", `SELECT inbox.finish_send($1,7,$2,'SUCCEEDED',NULL)`, []any{out2.OperationID, token}},
	} {
		var n int64
		qerr := f.owner.QueryRow(context.Background(), q.sql, q.args...).Scan(&n)
		var pg *pgconn.PgError
		if !errors.As(qerr, &pg) || pg.Code != "40001" {
			t.Fatalf("%s: want 40001, got %v", q.name, qerr)
		}
	}
	if e.secretCount(t, out2.OperationID) != 1 {
		t.Fatal("a fenced call touched the dispatch copy")
	}
	var cnt int64
	if err := f.owner.QueryRow(context.Background(), `SELECT count(*) FROM inbox.load_send_secret($1,1,$2)`, out2.OperationID, token).Scan(&cnt); err != nil || cnt != 1 {
		t.Fatalf("the correct lease must load exactly one sealed row: %d %v", cnt, err)
	}
	if _, err := f.owner.Exec(context.Background(), `SELECT inbox.finish_send($1,1,$2,'SUCCEEDED',NULL)`, out2.OperationID, token); err != nil {
		t.Fatalf("finish with the correct lease: %v", err)
	}
	if e.secretCount(t, out2.OperationID) != 0 {
		t.Fatal("finish did not wipe the dispatch copy")
	}
	if st := e.state(t, conv); st.LastOut == nil {
		t.Fatal("finish_send did not advance last_outbound_at")
	}
}

// §3.6: a manual private reply to a comment whose bundle peer already has a conversation takes that conversation over
// (human, assignee, gen+1, human_until +6 h, audit) and freezes conversation_known/conversation_id/generation (§14.1.3 shape).
func TestK3LCB4ManualReplyLinkedPeerTakeover(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	psid := mciDigits(15)
	conv := e.postDM(t, psid, "buyer asks", time.Now())
	base := e.auditCount(t, "inbox.takeover")

	first := e.planReply(t, false, psid, "A1") // creates the bundle + the automatic reply
	if first.op == "" || first.bundleID == "" {
		t.Fatal("no automatic reply planned")
	}
	e.linkPeer(t, first.bundleID, conv, first.op)
	second := e.planReply(t, false, psid, "A1") // same actor: joins the bundle, no second automatic reply
	if second.op != "" {
		t.Fatalf("a second automatic reply was planned: %s", second.op)
	}
	if second.bundleID != first.bundleID {
		t.Fatalf("the second comment did not join the bundle: %s vs %s", second.bundleID, first.bundleID)
	}

	out, err := e.manual(second.s.comment, "linked-"+t04Tag(), time.Now().Add(-10*time.Minute), false)
	if err != nil {
		t.Fatalf("manual reply to the linked bundle: %v", err)
	}
	st := e.state(t, conv)
	if st.Mode != "human" || st.Assignee == nil || *st.Assignee != e.h.actor || st.Gen != 1 {
		t.Fatalf("implicit takeover by the manual reply: %+v", st)
	}
	if st.HumanUntil == nil || time.Until(*st.HumanUntil) < 5*time.Hour || time.Until(*st.HumanUntil) > 6*time.Hour+time.Minute {
		t.Fatalf("human_until not ~6 h out: %+v", st.HumanUntil)
	}
	if n := e.auditCount(t, "inbox.takeover") - base; n != 1 {
		t.Fatalf("takeover audits=%d", n)
	}
	var known bool
	var gen int64
	var convID, bundleID string
	if err := f.owner.QueryRow(context.Background(),
		`SELECT (request->>'conversation_known')::boolean, (request->>'takeover_generation')::bigint, request->>'conversation_id', request->>'bundle_id'
		 FROM integration.operations WHERE id=$1`, out.OperationID).Scan(&known, &gen, &convID, &bundleID); err != nil {
		t.Fatal(err)
	}
	if !known || gen != 1 || convID != conv || bundleID != first.bundleID {
		t.Fatalf("frozen request: known=%v gen=%d conv=%s bundle=%s", known, gen, convID, bundleID)
	}
	// The reply dispatches: the comment budget check must ignore the OTHER comment's automatic operation.
	e.run(t, out.OperationID)
	e.awaitOp(t, out.OperationID, "SUCCEEDED", 10*time.Second, "completed")
}

// §3.3 windows: IG live comments close 15 min after creation (and need an OPEN window), Messenger manual replies close
// at 7 days − 1 h; the IG text limit is 1000 BYTES.
func TestK3LCB4ManualReplyPlatformWindows(t *testing.T) {
	e := lbSetup(t)
	f := e.h.f

	// Messenger 7-day-minus-1-hour boundary.
	e.onlySource(t, "facebook")
	if _, err := e.manual(lbRef(), "edge-in", time.Now().Add(-7*24*time.Hour+time.Hour+30*time.Second), false); err != nil {
		t.Fatalf("7d-1h+30s must plan: %v", err)
	}
	if _, err := e.manual(lbRef(), "edge-out", time.Now().Add(-7*24*time.Hour+time.Hour-30*time.Second), false); planCode(err) != "expired_7d" {
		t.Fatalf("7d-1h-30s must be expired_7d: %v", err)
	}

	// Instagram: 14 min plans with the deadline frozen at created+15 min; 16 min is ig_live_ended.
	e.onlySource(t, "instagram")
	created14 := time.Now().Add(-14 * time.Minute)
	out14, err := e.manual(lbRef(), "ig-14min", created14, false)
	if err != nil {
		t.Fatalf("IG comment 14 min old: %v", err)
	}
	var deadlineText string
	if err := f.owner.QueryRow(context.Background(), `SELECT request->>'deadline_at' FROM integration.operations WHERE id=$1`, out14.OperationID).Scan(&deadlineText); err != nil {
		t.Fatal(err)
	}
	deadline, err := time.Parse(time.RFC3339, deadlineText)
	if err != nil {
		t.Fatalf("deadline_at %q: %v", deadlineText, err)
	}
	if want := created14.Add(15 * time.Minute); deadline.Before(want.Add(-2*time.Second)) || deadline.After(want.Add(2*time.Second)) {
		t.Fatalf("IG deadline %s, want ~%s (created+15 min)", deadline, want)
	}
	if _, err := e.manual(lbRef(), "ig-16min", time.Now().Add(-16*time.Minute), false); planCode(err) != "ig_live_ended" {
		t.Fatalf("IG comment 16 min old: %v", err)
	}
	// No OPEN window: even a fresh IG comment is ig_live_ended.
	mustExec(t, f.owner, `UPDATE live.claim_windows SET state='CLOSED',closed_at=clock_timestamp() WHERE session_id=$1 AND state='OPEN'`, e.session)
	if _, err := e.manual(lbRef(), "ig-closed", time.Now(), false); planCode(err) != "ig_live_ended" {
		t.Fatalf("IG comment with a closed window: %v", err)
	}
	// 334 × 好 = 1002 UTF-8 bytes > 1000 (text validation precedes the planner, so this also holds with the window closed).
	var se *inbox.SendError
	if _, err := e.manual(lbRef(), strings.Repeat("好", 334), time.Now().Add(-14*time.Minute), false); !errors.As(err, &se) || se.Code != "invalid_text" || se.Max != 1000 {
		t.Fatalf("IG 1002-byte reply: %v", err)
	}
}

// §3.3/§3.5 text boundaries: public 300 runes; the storefront origin in scheme/case variants; buyer variables and the
// display placeholder are forbidden in public text; Messenger DM 2000 runes; Instagram DM 1000 bytes.
func TestK3LCB4TextValidationBoundaries(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	host := strings.TrimPrefix(e.origin, "https://")

	// Public: 300 runes plan, 301 refuse; every refused variant leaves zero operations.
	if _, err := e.publicReply(lbRef(), strings.Repeat("a", 300)); err != nil {
		t.Fatalf("300-rune public reply: %v", err)
	}
	var se *inbox.SendError
	if _, err := e.publicReply(lbRef(), strings.Repeat("a", 301)); !errors.As(err, &se) || se.Code != "invalid_text" || se.Max != 300 {
		t.Fatalf("301-rune public reply: %v", err)
	}
	for name, text := range map[string]string{
		"origin full":           e.origin,
		"origin upper scheme":   "HTTPS://" + host,
		"origin upper host":     "https://" + strings.ToUpper(host),
		"origin trailing slash": e.origin + "/",
		"buyer variable":        "下單{{order.pay_link}}",
		"display placeholder":   "結帳{{連結}}",
	} {
		ref := lbRef()
		if _, err := e.publicReply(ref, text); planCode(err) != "public_reply_forbidden_content" {
			t.Fatalf("%s: %v", name, err)
		}
		if n := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE action='meta.public_reply' AND request->>'comment_ref'=$1`, ref); n != 0 {
			t.Fatalf("%s: refused reply left %d operations", name, n)
		}
	}

	// Messenger DM: 2000 runes plan, 2001 refuse.
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	if _, err := e.sendDM(conv, strings.Repeat("好", 2000), 0); err != nil {
		t.Fatalf("2000-rune DM: %v", err)
	}
	if _, err := e.sendDM(conv, strings.Repeat("好", 2001), 1); !errors.As(err, &se) || se.Code != "invalid_text" || se.Max != 2000 {
		t.Fatalf("2001-rune DM: %v", err)
	}

	// Instagram DM: the limit is 1000 BYTES — 333 × 好 (999) plans, 334 × 好 (1002) refuses.
	convIG := e.postIGDM(t, mciDigits(15), "hello", time.Now())
	if _, err := e.sendDM(convIG, strings.Repeat("好", 333), 0); err != nil {
		t.Fatalf("999-byte IG DM: %v", err)
	}
	if _, err := e.sendDM(convIG, strings.Repeat("好", 334), 1); !errors.As(err, &se) || se.Code != "invalid_text" || se.Max != 1000 {
		t.Fatalf("1002-byte IG DM: %v", err)
	}
}

// §4.2 + §14.1.2 interplay: a manual private reply that provably never sent (BLOCKED_POLICY, zero HTTP) still counts as
// "an existing mpr: operation" for the auto path — the claim commits with reply_used + link_pending_manual, and the
// merchant's recovery is exactly one ':m1' manual reply (a third attempt is 'used').
func TestK3LCB4ReplyUsedBlockedManualRecovery(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	ref := lbRef()
	text := func(s string) string { return s + "-" + t04Tag() }

	// The manual reply is planned BEFORE the claim applies, then dies at check (window closed): zero HTTP.
	out, err := e.manual(ref, text("manual-first"), time.Now().Add(-10*time.Minute), false)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `UPDATE integration.operations SET state='BLOCKED_POLICY', generation=1, result_code='window_closed' WHERE id=$1`, out.OperationID)
	if e.secretCount(t, out.OperationID) != 0 {
		t.Fatal("the wipe trigger did not fire on BLOCKED_POLICY")
	}

	// The claim applies: no automatic reply (the blocked manual op still holds the reply), the bundle is flagged.
	skipBase := e.auditCount(t, "claim_reply_skipped:reply_used")
	e.postFBTo(t, e.postID, ref, "", "A1", mciAt(3*time.Second), nil, true)
	e.apply(t)
	c := e.claimOf(t, ref)
	if c.State != "APPLIED" || c.Outcome != "ACCEPTED" || c.Auto != 0 || c.Manual != 1 || !c.Flag {
		t.Fatalf("auto apply against a BLOCKED_POLICY manual op: %+v", c)
	}
	if e.auditCount(t, "claim_reply_skipped:reply_used") != skipBase+1 {
		t.Fatal("claim_reply_skipped:reply_used not audited")
	}

	// Recovery: exactly one ':m1' manual reply, which sends; a third attempt is 'used'.
	rec, err := e.manual(ref, text("recovery"), time.Now().Add(-10*time.Minute), false)
	if err != nil {
		t.Fatalf("the :m1 recovery reply must plan: %v", err)
	}
	var key string
	if err := f.owner.QueryRow(context.Background(), `SELECT semantic_key FROM integration.operations WHERE id=$1`, rec.OperationID).Scan(&key); err != nil {
		t.Fatal(err)
	}
	if want := lbMprKey("page", e.pageAsset, ref) + ":m1"; key != want {
		t.Fatalf("recovery key %q, want %q", key, want)
	}
	before := e.g.count()
	e.run(t, rec.OperationID)
	e.awaitOp(t, rec.OperationID, "SUCCEEDED", 10*time.Second, "completed")
	if n := e.g.count() - before; n != 1 {
		t.Fatalf("recovery Graph calls=%d", n)
	}
	if _, err := e.manual(ref, text("third"), time.Now().Add(-10*time.Minute), false); planCode(err) != "used" {
		t.Fatalf("a third private reply must be 'used': %v", err)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE action='meta.private_reply' AND request->>'comment_ref'=$1`, ref); n != 2 {
		t.Fatalf("private reply operations for the comment: %d (blocked manual + :m1 recovery)", n)
	}
}
