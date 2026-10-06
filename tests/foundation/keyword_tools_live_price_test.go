// Purpose: REAL_PG verification of what a batch deactivate does to buyers who already hold a claim on the offer: it behaves exactly like the single M4 pause (gate LTG03), so the live price ends at the next quote and a link not yet opened skips the line as offer_inactive; reactivation restores both. It also pins the deactivate_has_claims warning the batch returns.
// Depends on: the ltg harness (live_tools_gate_test.go: ltgNew, session, claimLink, redeemed, line, wantLive, wantCatalog, mjson), lcHarness preview/redeem; POST .../claims/offers/batch and PATCH .../offers/{id} through the real merchant handler.
// Used by: scripts/dev/test-focused.sh '^(TestSimulate|TestKeywordTools)'; CI foundation suite.
// Invariants: live-keyword-claims-v1 amendment "Live tools (R4)" rule 7 (an inactive offer earns no live price) and gate LTG03 (pausing ends the price at the NEXT quote, reactivating restores it: the price is evaluated at quote time); the claim line and link are untouched by a deactivate.
// Status: REAL_PG, MOCK (no PSP, no Meta). NOTE for the integrator: this pins CURRENT behaviour, which does NOT keep the granted live price after a deactivate; keeping it is a money-rule change that overturns LTG03 and needs its own ruling (see output/w3-06b-keyword-tools/DELIVERY.md).

package foundation_test

import (
	"fmt"
	"reflect"
	"testing"

	"livecommerce/internal/claims"
)

func TestKeywordToolsBatchDeactivateIsAPause(t *testing.T) {
	e := ltgNew(t)
	s1, o1 := e.session("A1", ltgLive, 5)
	bundle1, l1 := e.claimLink(s1, "amy", "A1+2")
	amy := e.redeemed(l1) // amy opened her link before the deactivate: the line is in her cart
	_, line := e.line(amy, "")
	e.wantLive("baseline", line, ltgLive, bundle1, o1.ID)
	e.h.open(t, s1, claims.MatchExact)
	_, l2 := e.claimLink(s1, "bob", "A1+1") // bob holds a link but has not opened it yet

	// Deactivate through the batch route.
	body := fmt.Sprintf(`{"items":[{"offer_id":%q,"expected_version":%d,"action":"deactivate"}]}`, o1.ID, o1.Version)
	st, out := e.mjson(e.token(), "POST", e.claimsPath(s1, "/offers/batch"), body)
	if st != 200 || out["applied"] != true {
		t.Fatalf("batch deactivate: %d %v", st, out)
	}
	warnings, _ := out["warnings"].([]any)
	if len(warnings) != 1 || warnings[0].(map[string]any)["kind"] != "deactivate_has_claims" || warnings[0].(map[string]any)["offer_id"] != o1.ID {
		t.Fatalf("the batch must warn that the deactivated offer has claims: %v", out["warnings"])
	}
	offers, _ := out["offers"].([]any)
	if len(offers) != 1 || offers[0].(map[string]any)["active"] != false {
		t.Fatalf("batch offers: %v", out["offers"])
	}

	// Verified behaviour (the same as the single PATCH pause, LTG03): amy's next quote is the catalog price ...
	_, line = e.line(amy, "")
	e.wantCatalog("after a batch deactivate the next quote is the catalog price", line)
	// ... and bob's pending line is skipped as offer_inactive when he opens his link.
	bob := e.buyerCap()
	pv, err := e.h.preview(bob.cap, l2.Token)
	if err != nil {
		t.Fatalf("bob preview: %v", err)
	}
	redeemed, err := e.h.redeem(bob.cap, t04Key("kt-bob"), l2.Token, pv.BundleVersion)
	if err != nil || !reflect.DeepEqual(lcSkipped(redeemed.Skipped), []claims.Skipped{{SKUID: e.money, Reason: "offer_inactive"}}) || len(redeemed.Cart.Items) != 0 {
		t.Fatalf("bob redeem after the deactivate: %+v %v", redeemed, err)
	}

	// Nothing of the entitlement is lost: reactivating the offer restores amy's live price at her next quote (claim line and link survive).
	v := int64(0)
	for _, o := range offers {
		if m := o.(map[string]any); m["offer_id"] == o1.ID {
			v = ltgNum(m, "version")
		}
	}
	st, out = e.mjson(e.token(), "PATCH", e.claimsPath(s1, "/offers/"+o1.ID), fmt.Sprintf(`{"expected_version":%d,"max_quantity_per_claim":5,"active":true}`, v))
	if st != 200 || out["active"] != true {
		t.Fatalf("reactivate: %d %v", st, out)
	}
	_, line = e.line(amy, "")
	e.wantLive("after reactivation", line, ltgLive, bundle1, o1.ID)
}
