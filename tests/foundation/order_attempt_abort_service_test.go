// Purpose: REAL_PG tests of Go-owned atomic abort and durable terminal replay.
// Depends on: real C1 Service, runtime authentication and the fenced SQL abort.
// Used by: OA1 CI; the interface assertion makes the pre-implementation test red.
// Invariants: I01/I08/I11; no private plan needed, no fresh key before settlement.
// PLAN_INVALID supplies the existing authenticated trusted classification to test
// receipt/atomicity/CAS interfaces; this fixture proves no codec error or permanent loss.
package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"livecommerce/internal/orderattempt"
)

type oaAbortService interface {
	Abort(context.Context, string, string, string, int64, string) (orderattempt.ExecutionReceipt, error)
}

func oaAbortAPI(t *testing.T, e *lbuEnv) oaAbortService {
	t.Helper()
	s, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil {
		t.Fatal(err)
	}
	api, ok := any(s).(oaAbortService)
	if !ok {
		t.Fatal("Service lacks the authoritative fenced Abort operation")
	}
	return api
}

func TestOrderAttemptAbortServiceSettlesAndReplays(t *testing.T) {
	e := lbuNew(t)
	req := oaServiceRequest(t, e)
	s, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || a.State != "PENDING" {
		t.Fatalf("real admitted intent positive sentinel: %v", err)
	}
	api := oaAbortAPI(t, e)
	first, err := api.Abort(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation, "PLAN_INVALID")
	if err != nil || first.State != "ABORTED" {
		t.Fatalf("fenced abort: state=%s err=%v", first.State, err)
	}
	var receipt map[string]any
	if err := json.Unmarshal(first.Receipt, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt["state"] != "ABORTED" || receipt["attempt_id"] != a.AttemptID || receipt["intent_generation"] != float64(a.Generation) || receipt["code"] != "PLAN_INVALID" {
		t.Fatal("abort lost its durable identity or reason")
	}
	if _, exists := receipt["order_id"]; exists {
		t.Fatal("no-order receipt includes an order pointer")
	}
	var persisted bool
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT state='ABORTED' AND receipt=$2::jsonb
	 AND plan_ciphertext IS NULL AND plan_key_version IS NULL
	 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=a.attempt_id)
	 AND NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.create_attempt_id=a.attempt_id)
	 FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID, string(first.Receipt)).Scan(&persisted); err != nil || !persisted {
		t.Fatalf("Go returned before full abort commit: %v", err)
	}
	// Treat the first response as lost. A second call with another reason must
	// return the original terminal bytes, not overwrite or reopen the attempt.
	replay, err := api.Abort(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation, "EXPIRED")
	if err != nil || replay.State != first.State || !bytes.Equal(replay.Receipt, first.Receipt) {
		t.Fatalf("lost response did not replay exact terminal receipt: %v", err)
	}
	next := req
	next.PreviousAttemptID = &a.AttemptID
	next.ExpectedGeneration = a.Generation
	next.TransportKey += ":explicit-new"
	b, err := s.Admit(context.Background(), e.token(), e.store(), next)
	if err != nil || b.State != "PENDING" || b.AttemptID == a.AttemptID || b.Generation != a.Generation+1 {
		t.Fatalf("explicit next generation not admitted after authoritative abort: %v", err)
	}
}

func TestOrderAttemptAbortServiceRefusalDoesNotRelease(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	api := oaAbortAPI(t, e)
	for _, code := range []string{"", "unknown", "INTERNAL_RETRYABLE", "network_error", "EXPIRED"} {
		t.Run(code, func(t *testing.T) {
			out, err := api.Abort(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation, code)
			if err == nil || out.State != "" || len(out.Receipt) != 0 {
				t.Fatal("unproved or transient refusal falsely settled an intent")
			}
		})
	}
	var pending bool
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT state='PENDING' AND plan_ciphertext IS NOT NULL
	 AND EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=a.attempt_id)
	 FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID).Scan(&pending); err != nil || !pending {
		t.Fatalf("refused abort changed durable pending authority: %v", err)
	}
}
