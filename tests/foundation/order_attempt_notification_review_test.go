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
	// 0128 check_send accepts only lcn-policy/v1; 0188 emits a different OA
	// policy. Pin the missing migration authority, not an invented runtime PASS.
	t.Run("oa-policy-authority", func(t *testing.T) {
		for _, required := range []string{
			"CREATE FUNCTION inbox.payment_dm_check_send(p_operation uuid)",
			"ALTER FUNCTION inbox.payment_dm_check_send(uuid) OWNER TO commerce_integration_writer;",
			"REVOKE ALL ON FUNCTION inbox.payment_dm_check_send(uuid) FROM PUBLIC;",
			"GRANT EXECUTE ON FUNCTION inbox.payment_dm_check_send(uuid) TO commerce_order_attempt_owner;",
			"inbox.check_send remains unchanged",
			"never rewrite a stored request policy",
		} {
			if !strings.Contains(request, required) {
				t.Errorf("OA policy authority handoff must specify %q", required)
			}
		}
		if !strings.Contains(c3, "inbox.payment_dm_check_send(p_operation)='OK'") {
			t.Error("eligible OA READY must use an OA-aware policy validator before receiving a dispatch permit")
		}
		if strings.Contains(c3, "inbox.check_send(p_operation)='OK'") {
			t.Error("legacy check_send rejects order-attempt-notification/v1; C3 cannot use it as OA eligibility authority")
		}
	})
	t.Run("oa-policy-witnesses", func(t *testing.T) {
		// Separate rows keep a valid OA policy from being rejected wholesale and
		// each policy refusal from being masked by a different failing condition.
		for _, witness := range []struct{ event, result string }{
			{"eligible-OA-READY", "DISPATCHING/DISPATCHING; exactly one dispatch permit"},
			{"consent-refused-before-C3", "VOID/BLOCKED_POLICY; zero provider calls"},
			{"window-closed-before-C3", "VOID/BLOCKED_POLICY; zero provider calls"},
			{"takeover-changed-before-C3", "VOID/BLOCKED_POLICY; zero provider calls"},
		} {
			t.Run(witness.event, func(t *testing.T) {
				for _, line := range strings.Split(request, "\n") {
					if strings.HasPrefix(line, "| "+witness.event+" |") && strings.Contains(line, witness.result) {
						return
					}
				}
				t.Fatalf("REAL_PG acceptance handoff needs a separate %s witness requiring %q", witness.event, witness.result)
			})
		}
	})
}
