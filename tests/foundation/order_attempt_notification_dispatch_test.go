// Purpose: OA-2 C3/C4 model tests for one dispatch permit, paired outcome,
// stale-generation refusal, generic-retry refusal (M4), cancel-before-C3 (M2a/M2b)
// and lease expiry (M7). These are MODEL tests over the frozen command seam.
// Depends on: internal/integrations/metareply order payment dispatcher.
// Used by: OA-2 focused CI; REAL_PG paired-state acceptance remains NOT_RUN until
// the integrator-assigned definers in output/oa-2/SQL-REQUEST.md exist.
// Invariants: no test starts a provider, DB, HTTP server or periodic scheduler.
package foundation_test

import (
	"context"
	"errors"
	"testing"

	"livecommerce/internal/integrations/metareply"
)

type oaFakePaymentStore struct {
	claim       metareply.OrderPaymentClaim
	claimErr    error
	completeOut metareply.OrderPaymentNotification
	completeErr error
	completes   []metareply.OrderPaymentOutcome
	reads       map[string]metareply.OrderPaymentNotification
	expired     []string
	expireOut   metareply.OrderPaymentNotification
	expireErr   error
	expires     []string
}

func (f *oaFakePaymentStore) Claim(context.Context, string, int) (metareply.OrderPaymentClaim, error) {
	return f.claim, f.claimErr
}

func (f *oaFakePaymentStore) Complete(_ context.Context, _ string, _ int64, _ []byte, out metareply.OrderPaymentOutcome) (metareply.OrderPaymentNotification, error) {
	f.completes = append(f.completes, out)
	return f.completeOut, f.completeErr
}

func (f *oaFakePaymentStore) Read(_ context.Context, operationID string) (metareply.OrderPaymentNotification, bool, error) {
	n, ok := f.reads[operationID]
	return n, ok, nil
}

func (f *oaFakePaymentStore) Expired(context.Context, int) ([]string, error) {
	return append([]string(nil), f.expired...), nil
}

func (f *oaFakePaymentStore) Expire(_ context.Context, operationID string) (metareply.OrderPaymentNotification, error) {
	f.expires = append(f.expires, operationID)
	return f.expireOut, f.expireErr
}

func oaPaymentClaim(state string) metareply.OrderPaymentClaim {
	return metareply.OrderPaymentClaim{
		Disposition: "claimed", Generation: 7, Mode: "dispatch", LeaseToken: make([]byte, 32),
		Notification: metareply.OrderPaymentNotification{
			NotificationID: oaPaymentID, State: state, OperationID: oaPaymentOp,
			OrderID: oaPaymentOrder, LinkVersion: 1, WorkerGeneration: 7,
		},
	}
}

const (
	oaPaymentID    = "11111111-1111-4111-8111-111111111111"
	oaPaymentOp    = "22222222-2222-4222-8222-222222222222"
	oaPaymentOrder = "33333333-3333-4333-8333-333333333333"
)

func TestOrderAttemptNotificationDispatchSinglePermit(t *testing.T) {
	store := &oaFakePaymentStore{claim: oaPaymentClaim("READY"), completeOut: metareply.OrderPaymentNotification{State: "ACCEPTED"}}
	calls := 0
	d, err := metareply.NewOrderPaymentDispatcher(store, func(_ context.Context, in metareply.OrderPaymentDispatch) (metareply.OrderPaymentOutcome, error) {
		calls++
		if in.Generation != 7 || len(in.LeaseToken) != 32 || in.Mode != "dispatch" {
			t.Fatalf("dispatch input lost the C3 permit: %+v", in)
		}
		return metareply.OrderPaymentOutcome{State: "ACCEPTED", ProviderReference: "m_mock"}, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := d.Dispatch(context.Background(), oaPaymentOp)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || out.Calls != 1 || out.Outcome.State != "ACCEPTED" {
		t.Fatalf("one permit must make exactly one accepted call: calls=%d out=%+v", calls, out)
	}
	if len(store.completes) != 1 || store.completes[0].State != "ACCEPTED" {
		t.Fatalf("C4 must write the paired acceptance once: %+v", store.completes)
	}

	// A duplicate job sees the claimed/busy paired row and never calls again.
	store.claim = metareply.OrderPaymentClaim{Disposition: "busy", Generation: 7}
	out, err = d.Dispatch(context.Background(), oaPaymentOp)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || out.Calls != 0 {
		t.Fatalf("duplicate job must not send: calls=%d out=%+v", calls, out)
	}
}

func TestOrderAttemptNotificationUnknownNeverSecondSend(t *testing.T) {
	store := &oaFakePaymentStore{claim: oaPaymentClaim("READY"), completeOut: metareply.OrderPaymentNotification{State: "DELIVERY_UNKNOWN"}}
	calls := 0
	d, err := metareply.NewOrderPaymentDispatcher(store, func(context.Context, metareply.OrderPaymentDispatch) (metareply.OrderPaymentOutcome, error) {
		calls++
		return metareply.OrderPaymentOutcome{}, errors.New("fake provider accepted then disconnected")
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := d.Dispatch(context.Background(), oaPaymentOp)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || out.Calls != 1 {
		t.Fatalf("ambiguous call must be made once: calls=%d out=%+v", calls, out)
	}
	if len(store.completes) != 1 || store.completes[0].State != "DELIVERY_UNKNOWN" {
		t.Fatalf("ambiguous outcome must pair as DELIVERY_UNKNOWN: %+v", store.completes)
	}
}

func TestOrderAttemptNotificationStaleGenerationFence(t *testing.T) {
	store := &oaFakePaymentStore{claim: oaPaymentClaim("READY"), completeErr: metareply.ErrOrderPaymentStaleGeneration}
	calls := 0
	d, err := metareply.NewOrderPaymentDispatcher(store, func(context.Context, metareply.OrderPaymentDispatch) (metareply.OrderPaymentOutcome, error) {
		calls++
		return metareply.OrderPaymentOutcome{State: "ACCEPTED"}, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Dispatch(context.Background(), oaPaymentOp); !errors.Is(err, metareply.ErrOrderPaymentStaleGeneration) {
		t.Fatalf("stale completion must be refused: %v", err)
	}
	if calls != 1 || len(store.completes) != 1 {
		t.Fatalf("a refused C4 must not trigger another send: calls=%d", calls)
	}
	// The same invocation is not re-run by the dispatcher.
	if _, err := d.Dispatch(context.Background(), oaPaymentOp); err == nil {
		t.Fatal("stale row must stay refused until the sweep settles it")
	}
	if calls != 2 {
		t.Fatalf("a second job invocation is a fresh claim, never an internal retry: %d", calls)
	}
}

func TestOrderAttemptNotificationRetryRefusal(t *testing.T) {
	store := &oaFakePaymentStore{reads: map[string]metareply.OrderPaymentNotification{}}
	d, err := metareply.NewOrderPaymentDispatcher(store, func(context.Context, metareply.OrderPaymentDispatch) (metareply.OrderPaymentOutcome, error) {
		return metareply.OrderPaymentOutcome{State: "ACCEPTED"}, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"READY", "VOID", "FAILED_FINAL", "DISPATCHING", "ACCEPTED", "DELIVERY_UNKNOWN"} {
		store.reads[oaPaymentOp] = metareply.OrderPaymentNotification{State: state}
		blocked, got, err := d.NotRetryable(context.Background(), oaPaymentOp)
		if err != nil || !blocked || got != state {
			t.Fatalf("M4: %s must refuse generic retry: blocked=%v state=%q err=%v", state, blocked, got, err)
		}
	}
	// A non-payment operation is not owned by this seam and keeps its own retry.
	delete(store.reads, oaPaymentOp)
	blocked, got, err := d.NotRetryable(context.Background(), oaPaymentOp)
	if err != nil || blocked || got != "" {
		t.Fatalf("unpaired operation must not be claimed by this refusal: %v %q %v", blocked, got, err)
	}
}

func TestOrderAttemptNotificationCancelBeforeC3ZeroCalls(t *testing.T) {
	// M2a/M2b model: the cancel transaction pairs VOID/CANCELLED before C3; the
	// dispatcher then sees a terminal row and makes zero provider calls.
	for _, disposition := range []string{"terminal", "void"} {
		store := &oaFakePaymentStore{claim: metareply.OrderPaymentClaim{
			Disposition:  disposition,
			Notification: metareply.OrderPaymentNotification{State: "VOID", VoidStage: "before_c3", VoidCode: "cancelled"},
		}}
		calls := 0
		d, err := metareply.NewOrderPaymentDispatcher(store, func(context.Context, metareply.OrderPaymentDispatch) (metareply.OrderPaymentOutcome, error) {
			calls++
			return metareply.OrderPaymentOutcome{State: "ACCEPTED"}, nil
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		out, err := d.Dispatch(context.Background(), oaPaymentOp)
		if err != nil {
			t.Fatal(err)
		}
		if calls != 0 || out.Calls != 0 || out.Notification.State != "VOID" {
			t.Fatalf("%s: cancel-before-C3 must keep zero calls: calls=%d out=%+v", disposition, calls, out)
		}
	}
}

func TestOrderAttemptNotificationLeaseSweep(t *testing.T) {
	store := &oaFakePaymentStore{
		expired:   []string{oaPaymentOp},
		expireOut: metareply.OrderPaymentNotification{State: "DELIVERY_UNKNOWN", OperationID: oaPaymentOp},
	}
	d, err := metareply.NewOrderPaymentDispatcher(store, func(context.Context, metareply.OrderPaymentDispatch) (metareply.OrderPaymentOutcome, error) {
		t.Fatal("the lease sweep must never send")
		return metareply.OrderPaymentOutcome{}, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := d.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if out.Scanned != 1 || len(out.Expired) != 1 || len(store.expires) != 1 {
		t.Fatalf("M7: one discarded job must become one expired lease: %+v", out)
	}
	store.expired = nil
	out, err = d.Sweep(context.Background())
	if err != nil || out.Scanned != 0 || len(out.Expired) != 0 {
		t.Fatalf("settled lease must not be rediscovered: %+v %v", out, err)
	}
}

func TestOrderAttemptNotificationReminderKeys(t *testing.T) {
	other := "44444444-4444-4444-8444-444444444444"
	semanticA, err := metareply.ReminderSemanticKey(oaPaymentOrder, 1, "reminder-1")
	if err != nil {
		t.Fatal(err)
	}
	semanticB, err := metareply.ReminderSemanticKey(other, 1, "reminder-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(semanticA) == string(semanticB) {
		t.Fatal("buyer A's reminder key must not collide with buyer B's order")
	}
	rotated, err := metareply.ReminderSemanticKey(oaPaymentOrder, 2, "reminder-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(semanticA) == string(rotated) {
		t.Fatal("a new link version is a distinct per-version key")
	}
	intent1, err := metareply.ReminderIntentKey(oaPaymentOrder, "reminder-1")
	if err != nil {
		t.Fatal(err)
	}
	intent2, err := metareply.ReminderIntentKey(oaPaymentOrder, "reminder-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(intent1) != string(intent2) {
		t.Fatal("intent replay across rotation must be stable")
	}
	if _, err := metareply.ReminderSemanticKey(oaPaymentOrder, 0, "reminder-1"); err == nil {
		t.Fatal("link version zero is invalid")
	}
}

func TestOrderAttemptNotificationOutcomeClassification(t *testing.T) {
	store := &oaFakePaymentStore{claim: oaPaymentClaim("READY"), completeOut: metareply.OrderPaymentNotification{State: "DELIVERY_UNKNOWN"}}
	calls := 0
	d, err := metareply.NewOrderPaymentDispatcher(store, func(context.Context, metareply.OrderPaymentDispatch) (metareply.OrderPaymentOutcome, error) {
		calls++
		return metareply.OrderPaymentOutcome{State: "ACKNOWLEDGED"}, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := d.Dispatch(context.Background(), oaPaymentOp)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || out.Outcome.State != "DELIVERY_UNKNOWN" {
		t.Fatalf("acknowledgement without acceptance proof is not success: %+v", out)
	}
	if len(store.completes) != 1 || store.completes[0].State != "DELIVERY_UNKNOWN" {
		t.Fatalf("paired outcome must use UNKNOWN: %+v", store.completes)
	}
	// A dispatch callback cannot self-certify authoritative no-send.
	store.completes = nil
	store.claim = oaPaymentClaim("READY")
	d2, err := metareply.NewOrderPaymentDispatcher(store, func(context.Context, metareply.OrderPaymentDispatch) (metareply.OrderPaymentOutcome, error) {
		return metareply.OrderPaymentOutcome{State: "VOID", Code: "no_effect"}, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d2.Dispatch(context.Background(), oaPaymentOp); !errors.Is(err, metareply.ErrOrderPaymentState) {
		t.Fatalf("dispatch-mode VOID must be refused: %v", err)
	}
	if len(store.completes) != 0 {
		t.Fatalf("refused dispatch outcome must not be persisted: %+v", store.completes)
	}
}

func TestOrderAttemptNotificationLeaseExpiryOnlyUnknown(t *testing.T) {
	store := &oaFakePaymentStore{
		claim: metareply.OrderPaymentClaim{Disposition: "claimed", Generation: 8, Mode: "reconcile", LeaseToken: make([]byte, 32),
			Notification: metareply.OrderPaymentNotification{State: "DELIVERY_UNKNOWN"}},
		completeOut: metareply.OrderPaymentNotification{State: "ACCEPTED"},
	}
	calls := 0
	d, err := metareply.NewOrderPaymentDispatcher(store, nil, func(context.Context, metareply.OrderPaymentDispatch) (metareply.OrderPaymentOutcome, error) {
		calls++
		return metareply.OrderPaymentOutcome{State: "ACCEPTED", ProviderReference: "m_mock"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := d.Dispatch(context.Background(), oaPaymentOp)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || out.Disposition != "claimed" || out.Outcome.State != "ACCEPTED" {
		t.Fatalf("reconcile may only query: %+v calls=%d", out, calls)
	}
}
