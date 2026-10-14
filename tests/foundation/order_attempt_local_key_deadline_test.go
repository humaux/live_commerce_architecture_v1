// Purpose: REAL_PG proof that a missing local key cannot settle a pending intent.
// Depends on: real C1, actual claims worker, immutable SQL clock and OrderAttemptSweep.
// Used by: OA1 CI; the deadline case takes the original fifteen minutes, without backdating.
// Invariants: PLAN_UNAVAILABLE is reserved/unused; only fenced EXPIRED releases this intent.

package foundation_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"livecommerce/internal/orderattempt"
)

func TestOrderAttemptReservedPlanUnavailableCannotAbortPending(t *testing.T) {
	e := oaExpiryIsolated(t)
	a := oaFenceAdmit(t, e)
	before := oaKeySnapshot(t, e, a.ScopeID)
	for _, worker := range []bool{false, true} {
		pool, token, store := e.p.f.runtime, e.token(), e.store()
		if worker {
			pool, token, store = miPool(t, e.p.f, "commerce_claims_worker"), "", ""
		}
		svc, err := orderattempt.NewService(pool, oaServiceKeys(t))
		if err != nil {
			t.Fatal(err)
		}
		out, err := svc.Abort(context.Background(), token, store, a.AttemptID, a.Generation, "PLAN_UNAVAILABLE")
		var refused *orderattempt.AdmissionError
		if !errors.As(err, &refused) || refused.SQLState() != "PT400" || refused.OutcomeUnknown() || out.State != "" || len(out.Receipt) != 0 {
			t.Fatalf("reserved reason must refuse pending settlement in lane worker=%v: state=%q err=%v", worker, out.State, err)
		}
		if !bytes.Equal(before, oaKeySnapshot(t, e, a.ScopeID)) {
			t.Fatal("reserved reason altered admitted authority, ciphertext or its job")
		}
		oaExecutorNoEffects(t, e, a)
	}
}

func TestOrderAttemptLocalKeyMissingExpiresAtRealDeadline(t *testing.T) {
	e := oaAreaIsolated(t)
	_, offer, bundle, conversation := e.scenario()
	request := oaMaterialRequest(t, e, oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation))
	svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil {
		t.Fatal(err)
	}
	a, err := svc.Admit(context.Background(), e.token(), e.store(), request)
	if err != nil || a.State != "PENDING" || a.Replay {
		t.Fatal("real claim admission prerequisite", err)
	}
	oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	var deadline, now time.Time
	var exact bool
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT a.execute_by,clock_timestamp(),
	 a.execute_by=a.admitted_at+interval '15 minutes' AND a.admission_finished
	 AND (SELECT count(*)=1 FROM inbox.order_for_buyer r WHERE r.create_attempt_id=a.attempt_id
	  AND r.bundle_id=$2 AND r.state='pending' AND r.order_id IS NULL AND r.expires_at=a.execute_by)
	 AND (SELECT count(*)=1 FROM claims.merchant_origin_grants g WHERE g.create_attempt_id=a.attempt_id
	  AND g.bundle_id=$2 AND g.quote_id IS NULL AND g.consumed_at IS NULL AND g.expires_at=a.execute_by)
	 FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID, bundle).Scan(&deadline, &now, &exact)
	if err != nil || !exact || deadline.Sub(now) < 14*time.Minute {
		t.Fatal("immutable real deadline and reservation/grant prerequisite", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), deadline.Sub(now)+90*time.Second)
	defer cancel()
	missing, err := orderattempt.NewPlanKeys("local-other", map[string][]byte{"local-other": bytes.Repeat([]byte{19}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	pool := miPool(t, e.p.f, "commerce_claims_worker")
	executor := oaExecutorNew(t, pool, missing)
	intent := oaSweepIntent(t, e, a.AttemptID)
	oaSweepOnlyCandidate(t, ctx, pool, a)
	for i := 0; i < 2; i++ {
		out, callErr := executor.Execute(ctx, "", "", a.AttemptID, a.Generation)
		var retryable *orderattempt.AdmissionError
		if !errors.As(callErr, &retryable) || retryable.Code() != "order_attempt_retryable" || retryable.OutcomeUnknown() || out.State != "" || len(out.Receipt) != 0 {
			t.Fatal("local ring miss must remain retryable with no settled receipt", callErr)
		}
		oaExecutorNoEffects(t, e, a)
		if oaSweepIntent(t, e, a.AttemptID) != intent {
			t.Fatal("local ring miss changed the frozen intent or extended its deadline")
		}
	}
	swept, err := executor.Sweep(ctx)
	if err != nil || swept.Scanned != 1 || swept.Aborted != 0 || swept.Requeued != 0 {
		t.Fatal("pre-deadline Sweep cannot settle local key absence", swept, err)
	}
	t.Log("waiting for original immutable execute_by; local key remains absent", deadline.Sub(now))
	// Observe the authoritative clock throughout the original window. Never
	// replace a helper, update admission timestamps or increase the deadline.
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		var due, pending bool
		probe, stop := context.WithTimeout(ctx, 3*time.Second)
		err = e.p.f.owner.QueryRow(probe, `SELECT clock_timestamp()>=execute_by,
		 state='PENDING' AND execute_by=$2 AND plan_ciphertext IS NOT NULL AND receipt IS NULL
		 FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID, deadline).Scan(&due, &pending)
		stop()
		if err != nil || !pending {
			t.Fatal("missing key settled early or changed the immutable deadline", err)
		}
		if due {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("original deadline observation exceeded its existing bound")
		case <-tick.C:
		}
	}
	oaExecutorNoEffects(t, e, a)
	oaSweepOnlyCandidate(t, ctx, pool, a)
	swept, err = executor.Sweep(ctx)
	if err != nil || swept.Scanned != 1 || swept.Aborted != 1 || swept.Requeued != 0 {
		t.Fatal("fenced deadline Sweep must expire without the missing key", swept, err)
	}
	var terminal orderattempt.ExecutionReceipt
	if err = e.p.f.owner.QueryRow(ctx, `SELECT state,receipt FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID).Scan(&terminal.State, &terminal.Receipt); err != nil {
		t.Fatal(err)
	}
	oaExecutorAborted(t, e, a, terminal, "EXPIRED")
	var released bool
	err = e.p.f.owner.QueryRow(ctx, `SELECT
	 (SELECT count(*)=1 FROM inbox.order_for_buyer WHERE create_attempt_id=$1 AND bundle_id=$2
	  AND state='released' AND order_id IS NULL AND expires_at=$3)
	 AND (SELECT count(*)=1 FROM claims.merchant_origin_grants WHERE create_attempt_id=$1 AND bundle_id=$2
	  AND quote_id IS NULL AND consumed_at IS NULL AND expires_at=$3 AND expires_at<=clock_timestamp())`,
		a.AttemptID, bundle, deadline).Scan(&released)
	if err != nil || !released {
		t.Fatal("deadline settlement failed to release exactly this admission", err)
	}
	replay, err := executor.Execute(ctx, "", "", a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("terminal expiry replay must not need any plan key", err)
	}
	oaExecutorSameReceipt(t, terminal, replay)
	request.TransportKey = "oa-key-deadline-next-" + randomUUID()
	request.ExpectedGeneration, request.PreviousAttemptID = a.Generation, &a.AttemptID
	next, err := svc.Admit(ctx, lcToken(t, e.p.f, e.p.f.principalA), e.store(), request)
	if err != nil || next.State != "PENDING" || next.Replay || next.ScopeID != a.ScopeID || next.Generation != a.Generation+1 || next.AttemptID == a.AttemptID {
		t.Fatal("fenced expiry must permit explicit next-generation admission", err)
	}
}
