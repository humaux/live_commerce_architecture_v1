// Purpose: real-runtime C1 admission through Go, native SQL and River InsertTx.
// Depends: OA1 0176/0023, lbuNew synthetic domain data and the real plan codec.
// Used: OA1 CI only; no provider calls, HTTP activation or C2 claim is made.
// Invariants: one admission transaction, safe opaque jobs, replay across sessions,
// rollback on unavailable sealing keys and principal/store generation CAS.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/orderattempt"
)

func oaServiceKeys(t *testing.T) *orderattempt.PlanKeys {
	t.Helper()
	keys, err := orderattempt.NewPlanKeys("fixture_v1", map[string][]byte{"fixture_v1": bytes.Repeat([]byte{7}, 32)})
	if err != nil { t.Fatal(err) }
	return keys
}

func oaServiceRequest(t *testing.T, e *lbuEnv) orderattempt.AdmissionRequest {
	t.Helper()
	var raw []byte
	if err := e.p.f.runtime.QueryRow(context.Background(), `SELECT ops.read_order_attempt_scope($1,$2)`, tokenHash(e.token()), e.store()).Scan(&raw); err != nil { t.Fatal(err) }
	var scope oaScopeReceipt
	if err := json.Unmarshal(raw, &scope); err != nil { t.Fatal(err) }
	selection, err := json.Marshal(oaFactsSelection(e))
	if err != nil { t.Fatal(err) }
	return orderattempt.AdmissionRequest{ExpectedScopeID: scope.ScopeID, ExpectedGeneration: scope.Generation,
		TransportKey: "oa-service-"+randomUUID(), Selection: selection}
}

func TestOrderAttemptServiceAtomicRuntimeAndPrivatePlan(t *testing.T) {
	t.Run("catalog origins", func(t *testing.T) { oaServiceAtomic(t, false) })
	t.Run("live and catalog same SKU", func(t *testing.T) { oaServiceAtomic(t, true) })
}

func oaServiceAtomic(t *testing.T, live bool) {
	t.Helper()
	e := lbuNew(t)
	keys := oaServiceKeys(t)
	svc, err := orderattempt.NewService(e.p.f.runtime, keys)
	if err != nil { t.Fatal(err) }
	req := oaServiceRequest(t, e)
	expectedSubtotal := int64(150000)
	if live {
		_, offer, bundle, conversation := e.scenario()
		selection := oaFactsSelection(e)
		line := selection["items"].([]map[string]any)[0]
		line["bundle_id"], line["offer_id"] = bundle, offer.ID
		selection["for"] = map[string]any{"bundle_ids":[]string{bundle}, "conversation_id":conversation}
		req.Selection, err = json.Marshal(selection)
		if err != nil { t.Fatal(err) }
		expectedSubtotal = 2*ltgLive+90000
	}
	got, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil { t.Fatalf("real C1 admission: %v", err) }
	if got.AttemptID == "" || got.ScopeID != req.ExpectedScopeID || got.Generation != 1 || got.State != "PENDING" || got.Replay {
		t.Fatalf("real admission positive sentinel: %+v", got)
	}
	var binding orderattempt.PlanBinding
	var sealed orderattempt.SealedPlan
	var requestHash, factsDigest, planDigest []byte
	var args []byte
	var queue string
	var priority, jobs, orders, carts int
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT a.attempt_id::text,a.scope_id::text,a.intent_generation,
	 a.tenant_id::text,a.store_id::text,s.principal_id::text,a.buyer_owner_id::text,a.admitted_at,a.execute_by,
	 a.request_hash,a.facts_digest,a.plan_digest,a.plan_key_version,a.plan_ciphertext,j.args,j.queue,j.priority,
	 (SELECT count(*) FROM river.river_job z WHERE z.kind='order_attempt_v1' AND z.args->>'attempt_id'=a.attempt_id::text),
	 (SELECT count(*) FROM checkout.orders o WHERE o.tenant_id=a.tenant_id AND o.store_id=a.store_id AND o.buyer_owner_id=a.buyer_owner_id),
	 (SELECT count(*) FROM storefront.carts c WHERE c.tenant_id=a.tenant_id AND c.store_id=a.store_id AND c.owner_id=a.buyer_owner_id)
	 FROM ops.order_attempts a JOIN ops.order_attempt_scopes s USING(scope_id)
	 JOIN river.river_job j ON j.id=a.execution_job_id AND j.id=a.enqueue_job_id WHERE a.attempt_id=$1`, got.AttemptID).
		Scan(&binding.AttemptID, &binding.ScopeID, &binding.Generation, &binding.TenantID, &binding.StoreID, &binding.PrincipalID,
			&binding.BuyerOwnerID, &binding.AdmittedAt, &binding.ExecuteBy, &requestHash, &factsDigest, &planDigest,
			&sealed.KeyVersion, &sealed.Ciphertext, &args, &queue, &priority, &jobs, &orders, &carts)
	if err != nil { t.Fatal(err) }
	binding.Schema = 1
	copy(binding.RequestHash[:], requestHash); copy(binding.FactsDigest[:], factsDigest); copy(sealed.Digest[:], planDigest)
	plan, err := keys.Open(binding, sealed)
	if err != nil { t.Fatalf("durable plan failed real codec: %v", err) }
	if len(plan.Calculation.Lines) != 2 || plan.Calculation.SubtotalMinor != expectedSubtotal || orders != 0 || carts != 0 || jobs != 1 || queue != "claims" || priority != 1 {
		t.Fatalf("C1 money/no-effects/jobs: lines=%d subtotal=%d orders=%d carts=%d jobs=%d queue=%s priority=%d", len(plan.Calculation.Lines), plan.Calculation.SubtotalMinor, orders, carts, jobs, queue, priority)
	}
	var job map[string]json.RawMessage
	if err = json.Unmarshal(args, &job); err != nil || len(job) != 2 || job["attempt_id"] == nil || job["intent_generation"] == nil {
		t.Fatalf("private data or wrong River shape: key count=%d err=%v", len(job), err)
	}
	public, err := json.Marshal(got)
	if err != nil || bytes.Contains(public, []byte("Synthetic")) || bytes.Contains(public, []byte("facts")) || bytes.Contains(public, []byte("ciphertext")) {
		t.Fatal("admission receipt disclosed private plan")
	}
}

func TestOrderAttemptServiceReplaySurvivesRelogin(t *testing.T) {
	e := lbuNew(t)
	svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil { t.Fatal(err) }
	req := oaServiceRequest(t, e)
	first, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil { t.Fatal(err) }
	var principal string
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT principal_id::text FROM ops.order_attempt_scopes WHERE scope_id=$1`, req.ExpectedScopeID).Scan(&principal); err != nil { t.Fatal(err) }
	relogin := "oa-service-relogin-"+randomUUID()
	tx, err := e.p.f.owner.Begin(context.Background())
	if err != nil { t.Fatal(err) }
	defer tx.Rollback(context.Background())
	if err = insertSession(context.Background(), tx, relogin, principal, "merchant", time.Now().Add(time.Hour), nil); err != nil { t.Fatal(err) }
	if err = tx.Commit(context.Background()); err != nil { t.Fatal(err) }
	replay, err := svc.Admit(context.Background(), relogin, e.store(), req)
	if err != nil || !replay.Replay || replay.AttemptID != first.AttemptID || replay.Generation != first.Generation {
		t.Fatalf("new-session same-key replay: %+v err=%v", replay, err)
	}
	var count int
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM river.river_job WHERE kind='order_attempt_v1' AND args->>'attempt_id'=$1`, first.AttemptID).Scan(&count); err != nil || count != 1 { t.Fatalf("replay enqueued again: %d %v", count, err) }
	var changed map[string]any
	if err = json.Unmarshal(req.Selection, &changed); err != nil { t.Fatal(err) }
	changed["locale"] = "zh-Hant"
	req.Selection, err = json.Marshal(changed)
	if err != nil { t.Fatal(err) }
	_, err = svc.Admit(context.Background(), relogin, e.store(), req)
	var coded interface { Code() string }
	if !errors.As(err, &coded) || coded.Code() != "idempotency_conflict" { t.Fatalf("changed body reused key: %v", err) }
}

func TestOrderAttemptServiceSealFailureRollsBackNativeAdmission(t *testing.T) {
	e := lbuNew(t)
	req := oaServiceRequest(t, e)
	// A nonnil but unavailable keyring fails at the real Seal seam after prepare.
	var priorOwners, priorJobs int
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM buyer.owners WHERE tenant_id=$1 AND store_id=$2),
	 (SELECT count(*) FROM river.river_job WHERE kind='order_attempt_v1')`, e.tenant(), e.store()).Scan(&priorOwners, &priorJobs); err != nil { t.Fatal(err) }
	svc, err := orderattempt.NewService(e.p.f.runtime, &orderattempt.PlanKeys{})
	if err != nil { t.Fatal(err) }
	_, err = svc.Admit(context.Background(), e.token(), e.store(), req)
	if !errors.Is(err, orderattempt.ErrPlanKeyUnavailable) { t.Fatalf("real Seal fault: %v", err) }
	var attempts, creators, subjects, jobs, owners int
	var generation int64
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT s.current_generation,
	 (SELECT count(*) FROM ops.order_attempts a WHERE a.scope_id=s.scope_id),
	 (SELECT count(*) FROM ops.order_attempt_creators c WHERE c.tenant_id=s.tenant_id AND c.store_id=s.store_id),
	 (SELECT count(*) FROM ops.order_attempt_subjects x WHERE x.tenant_id=s.tenant_id AND x.store_id=s.store_id),
	 (SELECT count(*) FROM river.river_job j WHERE j.kind='order_attempt_v1'),
	 (SELECT count(*) FROM buyer.owners o WHERE o.tenant_id=s.tenant_id AND o.store_id=s.store_id)
	 FROM ops.order_attempt_scopes s WHERE s.scope_id=$1`, req.ExpectedScopeID).Scan(&generation, &attempts, &creators, &subjects, &jobs, &owners)
	if err != nil || generation != req.ExpectedGeneration || attempts != 0 || creators != 0 || subjects != 0 || jobs != priorJobs || owners != priorOwners { t.Fatalf("partial C1 after Seal failure: generation=%d attempts=%d creators=%d subjects=%d jobs=%d owners=%d err=%v", generation, attempts, creators, subjects, jobs, owners, err) }
	// The exact same CAS/key can now succeed, proving rollback also released fences.
	good, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil { t.Fatal(err) }
	if _, err = good.Admit(context.Background(), e.token(), e.store(), req); err != nil { t.Fatalf("retry after known rollback: %v", err) }
}

func TestOrderAttemptServiceConcurrentFreshKeysOneCAS(t *testing.T) {
	e := lbuNew(t)
	svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil { t.Fatal(err) }
	req := oaServiceRequest(t, e)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		candidate := req
		candidate.TransportKey = "oa-race-"+randomUUID()
		wg.Add(1)
		go func() { defer wg.Done(); <-start; _, err := svc.Admit(context.Background(), e.token(), e.store(), candidate); results <- err }()
	}
	close(start); wg.Wait(); close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil { success++; continue }
		var coded interface { Code() string }
		if !errors.As(err, &coded) || coded.Code() != "intent_conflict" { t.Fatalf("unexpected concurrent result: %v", err) }
		conflicts++
	}
	if success != 1 || conflicts != 1 { t.Fatalf("CAS winners=%d conflicts=%d", success, conflicts) }
	var attempts, jobs int
	var generation int64
	err = e.p.f.owner.QueryRow(context.Background(), `SELECT s.current_generation,
	 (SELECT count(*) FROM ops.order_attempts a WHERE a.scope_id=s.scope_id),
	 (SELECT count(*) FROM river.river_job j JOIN ops.order_attempts a ON a.execution_job_id=j.id WHERE a.scope_id=s.scope_id)
	 FROM ops.order_attempt_scopes s WHERE scope_id=$1`, req.ExpectedScopeID).Scan(&generation,&attempts,&jobs)
	if err != nil || generation != req.ExpectedGeneration+1 || attempts != 1 || jobs != 1 { t.Fatalf("durable CAS: generation=%d attempts=%d jobs=%d err=%v",generation,attempts,jobs,err) }
}
