// Purpose: prevent the OA-2 SQL handoff from applying fresh-send refusal to consumed permits.
// Depends on: output/oa-2/SQL-REQUEST.md C3 algorithm; order-attempt-v1 §4 transition authority.
// Used by: Foundation CI for RVFIX305; this document check is not REAL_PG acceptance.
// Invariants: no local runtime; UNKNOWN reconciliation precedes all fresh-send eligibility checks.
package foundation_test

import (
	"os"
	"regexp"
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
	t.Run("recent-operation-excludes-current-scoped-pair", func(t *testing.T) {
		// OA-1 creates the operation and READY together. The predicate must
		// distinguish that row from another recent operation, without a delay.
		predicate := regexp.MustCompile(`(?s)NOT EXISTS\s*\(SELECT 1 FROM integration\.operations recent\b.*?interval '30 seconds'\s*\)`).FindString(c3)
		for _, clause := range []string{
			"recent.tenant_id=o.tenant_id", "recent.store_id=o.store_id",
			"recent.request->>'order_id'=n.order_id::text", "recent.action='meta.dm_send'",
			"recent.id<>p_operation", "recent.created_at>clock_timestamp()-interval '30 seconds'",
		} {
			if !strings.Contains(predicate, clause) {
				t.Errorf("recent-operation refusal must scope and exclude the admitted operation: missing %q", clause)
			}
		}
	})
	t.Run("void-schema-covers-fixed-dispositions", func(t *testing.T) {
		// Inspect the requested CHECK itself: mentioning a refusal elsewhere
		// cannot make a paired write survive PostgreSQL's existing constraint.
		if !strings.Contains(request, "DROP CONSTRAINT order_payment_notifications_void_code_check") {
			t.Error("schema delta must replace the restrictive existing void_code CHECK")
		}
		check := regexp.MustCompile(`(?s)ADD CONSTRAINT order_payment_notifications_void_code_check\s+CHECK\s*\(void_code IN\s*\((.*?)\)\)`).FindStringSubmatch(request)
		if len(check) != 2 {
			t.Fatal("schema delta must replace the void_code CHECK with the complete fixed-code inventory")
		}
		for _, code := range []string{
			"forbidden", "window_closed", "capability", "conversation_gone", "takeover_changed",
			"duplicate_recent", "rate_limited", "template_not_found", "template_kind", "recipient_invalid",
			"format_invalid", "pay_entry_unavailable", "storefront_unavailable", "domain_selection_required",
			"cancelled", "rotated", "consent_refused", "invalid_request", "no_effect",
		} {
			if !strings.Contains(check[1], "'"+code+"'") {
				t.Errorf("requested void_code CHECK cannot persist %s", code)
			}
		}
	})
	t.Run("void-code-mapping-is-explicit", func(t *testing.T) {
		for _, mapping := range []string{
			"| invalid_request, outbound_missing | invalid_request | BLOCKED_POLICY | before_c3 |",
			"| principal_revoked | forbidden | BLOCKED_POLICY | before_c3 |",
			"| consent_refused | consent_refused | BLOCKED_POLICY | before_c3 |",
			"| authoritative-no-effect | no_effect | FAILED_FINAL | after_c3 |",
		} {
			if !strings.Contains(request, mapping) {
				t.Errorf("paired VOID writes need a schema-admitted fixed mapping: %s", mapping)
			}
		}
	})
	t.Run("immediate-and-distinct-refusal-witnesses", func(t *testing.T) {
		for _, witness := range []struct{ event, required string }{
			{"eligible-OA-READY", "immediately after OA-1 admission; operation age < 30 seconds; no other same-scope/order operation"},
			{"distinct-recent-operation-before-C3", "VOID/BLOCKED_POLICY; duplicate_recent; before_c3; zero provider calls; no dispatch permit"},
			{"consent-refused-before-C3", "commit succeeds under the requested void_code CHECK; raw paired readback"},
			{"authoritative-no-effect-after-C3", "VOID/FAILED_FINAL; no_effect; after_c3; commit succeeds under the requested void_code CHECK; raw paired readback"},
		} {
			t.Run(witness.event, func(t *testing.T) {
				for _, line := range strings.Split(request, "\n") {
					if strings.HasPrefix(line, "| "+witness.event+" |") && strings.Contains(line, witness.required) {
						return
					}
				}
				t.Fatalf("acceptance handoff must pin %s: %s", witness.event, witness.required)
			})
		}
	})
}
