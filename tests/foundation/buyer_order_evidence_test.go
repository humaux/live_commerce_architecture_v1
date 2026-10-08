// Purpose: preserve all 23 order cases while requiring both new layout regressions; account explicitly for PR7's additive UTC history check.
// Depends on: order-gate.mjs completed-case records and the standard testing package; no browser or database fixture.
// Used by: TestBrowserBuyerOrderUI and TestBuyerOrderEvidenceCompleteness.

package foundation_test

import (
	"testing"
)

const orderDesktopLayout = "BO01 desktop delivery-head completion keeps the language target stable"
const orderMobileLayout = "BO01 mobile delivery-head completion keeps the language target stable"
const orderUTCHistory = "BH05 same owned history renders Taipei times in UTC browser context"

var orderCoreCases = []string{
	"BO01 native address; three locales keep the pinned quote and carry no unsaved PII across the language switch",
	"BO03 committed destination lost reply retries exact key and body",
	"BO03 lost destination reply reload reads head and explicitly replaces metadata intent",
	"BO03 reload reconfirmation, edits and concurrent/late address CAS fail closed",
	"BO04 actual confirmed form creates authoritative DRAFT and exactly one order/hold/job/receipt",
	"BO05 owned current-state GET renders real expiry rather than historical DRAFT receipt",
	"BO02 expired authoritative quote fails closed with no order or hold",
	"BO04 actual second-tab recovery queues on Web Lock and resumes same order without second POST",
	"BO05 lost checkout reply reload replays exact five-field intent and original key on mobile",
	"BO05 locator storage failure retains and reloads same checkout without duplicate hold",
	"BO05 later publication revocation preserves uncertain committed order and never resets owner",
	"BO03 delayed real options cannot replace edited confirmed address or order snapshot",
	"BO03 expired pending address retains recovery; quote-less new tab explicitly repairs shared intent",
	"BH01 lost next-cart reply reload keeps original key/body and exactly one cart receipt",
	"BH02 delayed initial owned GET cannot restore the old order after another tab continues",
	"BH03 same buyer creates distinct B with two exact order facts and unchanged A snapshot/key replay",
	"BH04 owned keyset pages and history loading/detail/locales/back preserve current B locator and cart",
	"BH05 authoritative history survives noncredential locator loss without repinning an old order",
	"BH06 explicit continuation preserves a newer authoritative cart without issuing a clearing PUT",
	"BH07 failed quote removal preserves continuation recovery and original key across reload",
	"BH08 foreign buyer history is empty and foreign or malformed cursors are denied",
	"BO02 service revision drift rejects a previously confirmed quotation without an order",
	"BO06 all attempted local/session writes, URLs and console exclude PII/bearer; other owner denied",
}

// buyerOrderCasesComplete requires every named core/layout observation exactly once.
// PR7's two renamed history cases are aliases, not additional coverage; its UTC case is additive.
func buyerOrderCasesComplete(count int, observations []string) bool {
	if count != len(observations) {
		return false
	}
	required := map[string]bool{orderDesktopLayout: true, orderMobileLayout: true}
	for _, item := range orderCoreCases {
		required[item] = true
	}
	aliases := map[string]string{
		"BH04 owned history renders each server created_at in Taipei across locales under Los Angeles browser time":       "BH04 owned keyset pages and history loading/detail/locales/back preserve current B locator and cart",
		"BH05 authoritative Taipei history survives reload and noncredential locator loss without repinning an old order": "BH05 authoritative history survives noncredential locator loss without repinning an old order",
	}
	seen := map[string]bool{}
	for _, item := range observations {
		if canonical, ok := aliases[item]; ok {
			item = canonical
		}
		if seen[item] || (!required[item] && item != orderUTCHistory) {
			return false
		}
		seen[item] = true
	}
	for item := range required {
		if !seen[item] {
			return false
		}
	}
	return true
}

func TestBuyerOrderEvidenceCompleteness(t *testing.T) {
	core := append([]string{}, orderCoreCases...)
	complete := append(append([]string{}, core...), orderDesktopLayout, orderMobileLayout)
	pr7 := append([]string{}, complete...)
	for i, item := range pr7 {
		switch item {
		case "BH04 owned keyset pages and history loading/detail/locales/back preserve current B locator and cart":
			pr7[i] = "BH04 owned history renders each server created_at in Taipei across locales under Los Angeles browser time"
		case "BH05 authoritative history survives noncredential locator loss without repinning an old order":
			pr7[i] = "BH05 authoritative Taipei history survives reload and noncredential locator loss without repinning an old order"
		}
	}
	for _, tc := range []struct {
		name string
		rows []string
		want bool
	}{
		{"trunk", complete, true},
		{"with-PR7-UTC", append(append([]string{}, pr7...), orderUTCHistory), true},
		{"duplicate-history-alias", append(append([]string{}, complete...), "BH04 owned history renders each server created_at in Taipei across locales under Los Angeles browser time"), false},
		{"missing-core-even-with-UTC", append(append([]string{}, complete[1:]...), orderUTCHistory), false},
		{"missing-mobile-layout", complete[:24], false},
		{"missing-desktop-layout", append(append([]string{}, core...), orderMobileLayout), false},
		{"duplicate-UTC", append(append([]string{}, complete...), orderUTCHistory, orderUTCHistory), false},
		{"duplicate-layout", append(append([]string{}, complete...), orderDesktopLayout), false},
		{"unknown-replaces-core", append([]string{"unknown"}, complete[1:]...), false},
		{"duplicate-replaces-core", append([]string{complete[1]}, complete[1:]...), false},
		{"unknown-extra", append(append([]string{}, complete...), "unknown-extra-case"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := buyerOrderCasesComplete(len(tc.rows), tc.rows); got != tc.want {
				t.Fatalf("complete=%v want %v", got, tc.want)
			}
		})
	}
	if buyerOrderCasesComplete(len(complete)-1, complete) {
		t.Fatal("reported count must equal completed observations")
	}
}
