// Purpose: keep the durable order-intent refusal typed at the public HTTP envelope seam.
// Depends on: httperror.Write, the fixed public message table and net/http/httptest.
// Used by: CI unit gates; merchant order placement must preserve 409 attempt_pending.
// Invariants: no order/private intent is disclosed, no automatic retry or replacement key is suggested.
package httperror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOrderIntentPendingPublicEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Write(w, http.StatusConflict, "attempt_pending")
	})).ServeHTTP(w, httptest.NewRequest("POST", "/orders", nil))
	var out Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusConflict || out.Code != "attempt_pending" || out.Retryable || out.Message != "Recover the unresolved order attempt before starting another order." || len(out.Details) != 0 || len(out.RequestID) != 32 {
		t.Fatalf("pending intent lost its typed private refusal: status=%d envelope=%+v", w.Code, out)
	}
}
