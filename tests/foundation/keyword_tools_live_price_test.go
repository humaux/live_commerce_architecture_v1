// Purpose: REAL_PG verification of what a batch deactivate does to buyers who already hold a claim on the offer: it behaves exactly like the single M4 pause, so (owner decision 2026-10-07) they KEEP the live price they were granted (quote, redeem of an unopened link), new claims are refused, and reactivation changes nothing.
// Depends on: the ltg harness (live_tools_gate_test.go: ltgNew, session, claimLink, redeemed, line, wantLive, wantCatalog, mjson), lcHarness preview/redeem/claim; POST .../claims/offers/batch and PATCH .../offers/{id} through the real merchant handler.
// Used by: scripts/dev/test-focused.sh '^(TestSimulate|TestKeywordTools)'; CI foundation suite.
// Invariants: live-keyword-claims-v1 amendment "Live price kept on pause" (supersedes "Live tools (R4)" rule 7 for paused offers and gate LTG03's catalog-after-pause step); the claim line and link are untouched by a deactivate.
// Status: REAL_PG, MOCK (no PSP, no Meta). History: W3-06B pinned the former behaviour (catalog price, offer_inactive skip) and asked for a ruling; the owner ruled 2026-10-07 and this file now pins the ruling.

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
	bobBundle, l2 := e.claimLink(s1, "bob", "A1+1") // bob holds a link but has not opened it yet

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

	// Owner decision 2026-10-07 ("already-claimed buyers keep the live price, only new claims are refused") REPLACES the former assertions here
	// (catalog price for amy, offer_inactive skip for bob): amy's next quote is still the exact live price ...
	_, line = e.line(amy, "")
	e.wantLive("after a batch deactivate the claimed line keeps the live price", line, ltgLive, bundle1, o1.ID)
	if line.Quantity != 2 {
		t.Fatalf("amy quantity %d, want 2", line.Quantity)
	}
	// ... bob's pending line is applied (not skipped) when he opens his link, and is priced at the live price for exactly his 1 claimed unit ...
	bob := e.buyerCap()
	pv, err := e.h.preview(bob.cap, l2.Token)
	if err != nil {
		t.Fatalf("bob preview: %v", err)
	}
	redeemed, err := e.h.redeem(bob.cap, t04Key("kt-bob"), l2.Token, pv.BundleVersion)
	if err != nil || len(redeemed.Skipped) != 0 || !reflect.DeepEqual(lcItems(redeemed.Applied), map[string]int64{e.money: 1}) || !reflect.DeepEqual(lcItems(redeemed.Cart.Items), map[string]int64{e.money: 1}) {
		t.Fatalf("bob redeem after the deactivate: %+v %v", redeemed, err)
	}
	_, bobLine := e.line(bob, "")
	e.wantLive("bob after redeeming while the offer is deactivated", bobLine, ltgLive, bobBundle, o1.ID)
	if bobLine.Quantity != 1 {
		t.Fatalf("bob quantity %d, want 1", bobLine.Quantity)
	}
	// ... but NEW claims are refused (the deactivate still closes the offer to everyone else).
	e.h.open(t, s1, claims.MatchExact)
	if r := e.h.claim(t, s1, claims.ManualClaimInput{ActorLabel: "carol", Text: "A1+1"}); r.Outcome != claims.OutcomeRejected || r.Reason != claims.ReasonOfferInactive {
		t.Fatalf("a new claim on the deactivated offer must be refused: %+v", r)
	}
	e.h.closeWindow(t, s1)

	// Reactivating the offer changes nothing for the claimants: amy keeps the same live price at her next quote (claim line and link survive).
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
	e.wantLive("after reactivation (unchanged)", line, ltgLive, bundle1, o1.ID)
}
