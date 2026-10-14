// Purpose: REAL_PG tests-first completion of CVS crash, key rotation and terminal CAS acceptance.
// Depends on: existing native C1/C2 fixtures, connection-kill barriers and production LoadKeys.
// Used by: parent-owned OA1 CI; no product hooks, provider calls or synthetic terminal rows.
// Invariants: original admitted owner survives configuration rotation; pickup writes roll back;
// terminal generations require explicit CAS, and concurrent fresh keys create one successor.
// ABORTED fixtures use synthetic trusted PLAN_INVALID classification through the
// authenticated API for CAS/replay checks, not actual codec-error or permanent-loss proof.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"livecommerce/internal/inbox"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/orderattempt"
)

func oaMatrixPickup(t *testing.T, s oaCrashScenario, committed bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var versions, heads int
	err := s.e.p.f.owner.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM fulfillment.pickup_versions WHERE create_attempt_id=$1),
	 (SELECT count(*) FROM fulfillment.pickup_heads WHERE tenant_id=$2 AND store_id=$3 AND namespace='attempt.'||($1::uuid)::text)`,
		s.a.AttemptID, s.e.tenant(), s.e.store()).Scan(&versions, &heads)
	want := 0
	if committed {
		want = 1
	}
	if err != nil || versions != want || heads != want {
		t.Fatalf("CVS durable versions/heads: %d/%d want %d: %v", versions, heads, want, err)
	}
	if !committed {
		return
	}
	var exact bool
	err = s.e.p.f.owner.QueryRow(ctx, `SELECT p.buyer_owner_id=a.buyer_owner_id AND p.principal_id IS NULL
	 AND p.kind='cvs_711' AND p.code='001234' AND p.namespace='attempt.'||a.attempt_id::text
	 AND p.verification_kind='BUYER_ENTERED' AND p.version=1 AND p.attested_at=a.admitted_at
	 AND p.valid_until=a.admitted_at+interval '24 hours' AND h.current_version=1 AND h.enabled
	 FROM ops.order_attempts a JOIN fulfillment.pickup_versions p ON p.create_attempt_id=a.attempt_id
	 JOIN fulfillment.pickup_heads h ON h.pickup_id=p.id WHERE a.attempt_id=$1`, s.a.AttemptID).Scan(&exact)
	if err != nil || !exact {
		t.Fatal("CVS original entered-store provenance was lost", err)
	}
}

func TestOrderAttemptMatrixCVSPickupCrash(t *testing.T) {
	if !t.Run("native-cvs-control", func(t *testing.T) {
		s := oaCrashNativeDeliveryScenario(t, true)
		out, err := s.executor.Execute(context.Background(), s.e.token(), s.e.store(), s.a.AttemptID, s.a.Generation)
		if err != nil {
			t.Fatal("native CVS complete-commit prerequisite", err)
		}
		durable := oaCrashCommitted(t, s, out)
		clear(durable)
		oaMatrixPickup(t, s, true)
	}) {
		t.Fatal("CVS control failed; no crash acceptance can be claimed")
	}
	pickup := `EXISTS(SELECT 1 FROM fulfillment.pickup_versions p JOIN ops.order_attempts a ON a.attempt_id=p.create_attempt_id
	 WHERE a.attempt_id=ATTEMPT AND p.buyer_owner_id=a.buyer_owner_id AND p.namespace='attempt.'||a.attempt_id::text
	 AND p.code='001234' AND p.verification_kind='BUYER_ENTERED' AND p.version=1)
	 AND NOT EXISTS(SELECT 1 FROM checkout.orders WHERE create_attempt_id=ATTEMPT)`
	for _, barrier := range []oaCrashBarrier{
		{"entered-store-version", "fulfillment.pickup_versions", "AFTER", "INSERT", "NEW.create_attempt_id=ATTEMPT", pickup},
		{"entered-store-head", "fulfillment.pickup_heads", "AFTER", "INSERT", "NEW.namespace='attempt.'||ATTEMPT::text", pickup + ` AND EXISTS(SELECT 1 FROM fulfillment.pickup_heads h JOIN fulfillment.pickup_versions p ON p.id=h.pickup_id WHERE p.create_attempt_id=ATTEMPT AND h.current_version=1 AND h.enabled)`},
	} {
		t.Run(barrier.name, func(t *testing.T) {
			s := oaCrashNativeDeliveryScenario(t, true)
			before := oaCrashCustody(t, s)
			defer clear(before)
			oaCrashKillAtBarrier(t, s, barrier)
			oaCrashPending(t, s, before)
			oaMatrixPickup(t, s, false)
			out, err := s.executor.Execute(context.Background(), s.e.token(), s.e.store(), s.a.AttemptID, s.a.Generation)
			if err != nil {
				t.Fatal("one explicit CVS crash recovery", err)
			}
			original := oaCrashCommitted(t, s, out)
			defer clear(original)
			oaMatrixPickup(t, s, true)
			again, err := s.executor.Execute(context.Background(), s.e.token(), s.e.store(), s.a.AttemptID, s.a.Generation)
			if err != nil {
				t.Fatal(err)
			}
			oaExecutorSameReceipt(t, out, again)
			repeated := oaCrashCommitted(t, s, again)
			defer clear(repeated)
			if !bytes.Equal(original, repeated) {
				t.Fatal("CVS replay rewrote durable artifacts")
			}
			oaMatrixPickup(t, s, true)
		})
	}
}

func oaMatrixKeys(t *testing.T, active string) (*orderattempt.PlanKeys, *orderattempt.PayEntryKeys) {
	t.Helper()
	ring := func(first, second byte) string {
		raw, err := json.Marshal(map[string]string{"v1": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{first}, 32)), "v2": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{second}, 32))})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	env := map[string]string{
		"LC_ORDER_ATTEMPT_PLAN_ACTIVE_KEY": active, "LC_ORDER_ATTEMPT_PLAN_KEYS": ring(7, 8),
		"LC_ORDER_ATTEMPT_PAY_ENTRY_ACTIVE_KEY": active, "LC_ORDER_ATTEMPT_PAY_ENTRY_KEYS": ring(65, 66),
	}
	plans, pay, err := orderattempt.LoadKeys(func(name string) string { return env[name] })
	if err != nil {
		t.Fatal("real startup configuration", err)
	}
	return plans, pay
}

func oaMatrixConfiguredExecutor(t *testing.T, e *lbuEnv, plans *orderattempt.PlanKeys, pay *orderattempt.PayEntryKeys) *orderattempt.Executor {
	t.Helper()
	keys, err := inbox.LoadKeyring(func(name string) string {
		if name == "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID" {
			return "matrix"
		}
		if name == "COMMERCE_META_PAYLOAD_KEYS_JSON" {
			return `{"keys":[{"id":"matrix","key_base64":"` + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{43}, 32)) + `"}]}`
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	notifications, err := inbox.NewService(keys)
	if err != nil {
		t.Fatal(err)
	}
	seal, _ := mcnPageRing(t)
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(e.p.f.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	notifications.EnableSend(seal, jobs, msgtemplates.NewService())
	executor, err := orderattempt.NewExecutor(e.p.f.runtime, plans, pay, notifications)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func TestOrderAttemptMatrixConfiguredRotationPreservesAdmittedOwner(t *testing.T) {
	e := lbuNew(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	oldPlans, _ := oaMatrixKeys(t, "v1")
	svc, err := orderattempt.NewService(e.p.f.runtime, oldPlans)
	if err != nil {
		t.Fatal(err)
	}
	req := oaServiceRequest(t, e)
	a, err := svc.Admit(ctx, e.token(), e.store(), req)
	if err != nil {
		t.Fatal(err)
	}
	plan, sealed := oaMaterialPlan(t, e, oldPlans, a.AttemptID)
	if sealed.KeyVersion != "v1" {
		t.Fatal("C1 did not use original configured plan version")
	}
	// This original is captured before any configuration change, independently
	// of the post-C2 row. Comparing current rows only could accept a wrong owner.
	originalOwner := plan.BuyerOwnerID
	plans, pay := oaMatrixKeys(t, "v2")
	opened, err := plans.Open(plan.Binding(), sealed)
	if err != nil || opened.BuyerOwnerID != originalOwner {
		t.Fatal("retained original plan key cannot open C1 owner", err)
	}
	executor := oaMatrixConfiguredExecutor(t, e, plans, pay)
	out, err := executor.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("actual C2 with rotated configuration", err)
	}
	original := oaExecutorCommitted(t, e, a, plan, out, 5)
	var exact bool
	var material orderattempt.PayEntryMaterial
	var digest []byte
	err = e.p.f.owner.QueryRow(ctx, `SELECT a.buyer_owner_id=$2::uuid AND o.owner_id=$2::uuid AND p.owner_id=$2::uuid
	 AND c.owner_id=$2::uuid AND d.owner_id=$2::uuid AND o.id=$1::uuid,
	 p.key_version,p.token_hash,p.credential_ciphertext
	 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id
	 JOIN checkout.order_pay_entries p ON p.order_id=o.id JOIN storefront.carts c ON c.create_attempt_id=a.attempt_id
	 JOIN storefront.destination_snapshots d ON d.create_attempt_id=a.attempt_id WHERE a.attempt_id=$1`,
		a.AttemptID, originalOwner).Scan(&exact, &material.KeyVersion, &digest, &material.Ciphertext)
	if err != nil || !exact || material.KeyVersion != "v2" {
		t.Fatal("rotated C2 changed original owner/order or ignored active pay key", err)
	}
	copy(material.Digest[:], digest)
	binding := orderattempt.PayEntryBinding{TenantID: e.tenant(), StoreID: e.store(), OrderID: a.AttemptID, Version: 1}
	credential, err := pay.Open(binding, material)
	if err != nil || credential == "" {
		t.Fatal("actual stored rotated credential cannot be decrypted", err)
	}
	replayPlans, replayPay := oaMatrixKeys(t, "v1")
	retained, err := replayPay.Open(binding, material)
	if err != nil || retained != credential {
		t.Fatal("retained pay version lost original credential", err)
	}
	replayService, err := orderattempt.NewService(e.p.f.runtime, replayPlans)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := replayService.Admit(ctx, e.token(), e.store(), req)
	if err != nil || !admission.Replay || admission.AttemptID != a.AttemptID || !bytes.Equal(admission.Receipt, out.Receipt) {
		t.Fatal("rotated C1 replay lost original terminal identity", err)
	}
	again, err := oaMatrixConfiguredExecutor(t, e, replayPlans, replayPay).Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, out, again)
	if oaExecutorCommitted(t, e, a, plan, again, 5) != original {
		t.Fatal("rotation replay rewrote original pay material")
	}
}

func TestOrderAttemptMatrixTerminalCASRace(t *testing.T) {
	for _, state := range []string{"COMMITTED", "ABORTED"} {
		t.Run(state, func(t *testing.T) {
			e := lbuNew(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
			if err != nil {
				t.Fatal(err)
			}
			req := oaServiceRequest(t, e)
			a, err := svc.Admit(ctx, e.token(), e.store(), req)
			if err != nil {
				t.Fatal(err)
			}
			plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
			var terminal orderattempt.ExecutionReceipt
			if state == "COMMITTED" {
				terminal, err = oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t)).Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
				if err != nil {
					t.Fatal(err)
				}
				oaExecutorCommitted(t, e, a, plan, terminal, 5)
			} else {
				terminal, err = svc.Abort(ctx, e.token(), e.store(), a.AttemptID, a.Generation, "PLAN_INVALID")
				if err != nil {
					t.Fatal(err)
				}
				oaExecutorAborted(t, e, a, terminal, "PLAN_INVALID")
			}
			stale := req
			stale.TransportKey = "matrix-stale-" + randomUUID()
			_, err = svc.Admit(ctx, e.token(), e.store(), stale)
			var coded *orderattempt.AdmissionError
			if !errors.As(err, &coded) || coded.Code() != "intent_conflict" {
				t.Fatal("fresh key accepted stale terminal CAS", err)
			}
			type result struct {
				receipt orderattempt.AdmissionReceipt
				err     error
			}
			start, results := make(chan struct{}), make(chan result, 2)
			for i := 0; i < 2; i++ {
				next := req
				next.ExpectedGeneration, next.PreviousAttemptID = a.Generation, &a.AttemptID
				next.TransportKey = "matrix-next-" + randomUUID()
				go func(candidate orderattempt.AdmissionRequest) {
					<-start
					got, callErr := svc.Admit(ctx, e.token(), e.store(), candidate)
					results <- result{got, callErr}
				}(next)
			}
			close(start)
			var winner orderattempt.AdmissionReceipt
			wins, conflicts := 0, 0
			for i := 0; i < 2; i++ {
				select {
				case got := <-results:
					if got.err == nil {
						wins++
						winner = got.receipt
					} else if errors.As(got.err, &coded) && coded.Code() == "intent_conflict" {
						conflicts++
					} else {
						t.Fatal("unexpected terminal successor race", got.err)
					}
				case <-ctx.Done():
					t.Fatal("bounded terminal successor race timed out")
				}
			}
			if wins != 1 || conflicts != 1 || winner.Generation != a.Generation+1 || winner.ScopeID != a.ScopeID || winner.State != "PENDING" || winner.Replay {
				t.Fatal("terminal CAS did not admit exactly one next generation")
			}
			var exact bool
			err = e.p.f.owner.QueryRow(ctx, `SELECT current_generation=$2::bigint
			 AND (SELECT count(*)=2 FROM ops.order_attempts WHERE scope_id=$1)
			 AND (SELECT count(*)=2 FROM river.river_job j JOIN ops.order_attempts a ON a.execution_job_id=j.id WHERE a.scope_id=$1)
			 FROM ops.order_attempt_scopes WHERE scope_id=$1`, a.ScopeID, winner.Generation).Scan(&exact)
			if err != nil || !exact {
				t.Fatal("race left duplicate durable attempt/job or wrong scope generation", err)
			}
			// Settle the real successor with the synthetic trusted classification,
			// then replay the original request; no terminal row is fabricated.
			settled, err := svc.Abort(ctx, e.token(), e.store(), winner.AttemptID, winner.Generation, "PLAN_INVALID")
			if err != nil {
				t.Fatal(err)
			}
			oaExecutorAborted(t, e, winner, settled, "PLAN_INVALID")
			replay, err := svc.Admit(ctx, e.token(), e.store(), req)
			if err != nil || !replay.Replay || replay.AttemptID != a.AttemptID || replay.Generation != a.Generation || replay.State != state || !bytes.Equal(replay.Receipt, terminal.Receipt) {
				t.Fatal("successor CAS changed original terminal key replay", err)
			}
			if state == "COMMITTED" {
				oaExecutorCommitted(t, e, a, plan, terminal, 5)
			} else {
				oaExecutorAborted(t, e, a, terminal, "PLAN_INVALID")
			}
		})
	}
}
