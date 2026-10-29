// Purpose: prevent the OA-2 SQL handoff from applying fresh-send refusal to consumed permits.
// Depends on: output/oa-2/SQL-REQUEST.md C3 algorithm; order-attempt-v1 §4 transition authority.
// Used by: Foundation CI for RVFIX305; this document check is not REAL_PG acceptance.
// Invariants: no local runtime; UNKNOWN reconciliation precedes all fresh-send eligibility checks.
package foundation_test

import (
	"os"
	"strings"
	"testing"
)

func TestOrderAttemptNotificationC3RequestReconcileBeforeRefusal(t *testing.T) {
	raw, err := os.ReadFile("../../output/oa-2/SQL-REQUEST.md")
	if err != nil {
		t.Fatal(err)
	}
	request := string(raw)
	start := strings.Index(request, "CREATE FUNCTION inbox.payment_dm_c3(")
	if start < 0 {
		t.Fatal("C3 handoff is missing")
	}
	end := strings.Index(request[start:], "END $$;")
	if end < 0 {
		t.Fatal("C3 handoff is incomplete")
	}
	c3 := request[start : start+end]
	// This is a document-order gate only: the SQL implementer must still prove
	// each branch with real paired rows, actual worker logins and lock barriers.
	branch := strings.Index(c3, "-- f. branch on the locked paired state BEFORE fresh-send eligibility:")
	eligibility := strings.Index(c3, "checkout.order_payment_payable(")
	if branch < 0 || eligibility < 0 || branch >= eligibility {
		t.Error("post-C3 reconciliation must branch before payable/link/window/consent refusal")
	}
	for _, required := range []string{
		"READY/READY AND dispatch_authorized_at IS NULL",
		"DELIVERY_UNKNOWN/(UNKNOWN|ACKNOWLEDGED)",
		"mode=reconcile; never enter steps g/h",
		"DISPATCHING/DISPATCHING returns busy",
		"preserve dispatch_authorized_at, link_version and provider evidence",
		"stale binding after C3 returns blocked_binding without changing either state",
	} {
		if !strings.Contains(c3, required) {
			t.Errorf("C3 handoff must specify %q", required)
		}
	}
	// Pin the three reviewed counterexamples in the handoff, rather than claim
	// a predetermined store fake has exercised these missing database branches.
	for _, event := range []string{"cancel-after-UNKNOWN", "rotation-after-UNKNOWN", "policy-loss-after-UNKNOWN"} {
		t.Run(event, func(t *testing.T) {
			if !strings.Contains(request, "| "+event+" |") {
				t.Fatalf("SQL acceptance handoff is missing the %s paired-state witness", event)
			}
		})
	}
}
