// Purpose: preserve all 23 order cases and both layout regressions; require six real cart Retry cases and retain PR7's additive UTC history check.
// Depends on: order-gate.mjs observations/cart facts/samples and standard json/reflect/testing; no browser or database fixture.
// Used by: TestBrowserBuyerOrderUI and DB-free evidence, exact fact and loading/continuity counterexamples.

package foundation_test

import (
	"encoding/json"
	"reflect"
	"testing"
)

const orderDesktopLayout = "BO01 desktop delivery-head completion keeps the language target stable"
const orderMobileLayout = "BO01 mobile delivery-head completion keeps the language target stable"
const orderUTCHistory = "BH05 same owned history renders Taipei times in UTC browser context"

var orderCartRetryCases = map[string]bool{
	"BC01 en drawer visible cart Retry preserves key/body and one committed write after reload":       true,
	"BC01 en cart-page visible cart Retry preserves key/body and one committed write after reload":    true,
	"BC01 zh-CN drawer visible cart Retry preserves key/body and one committed write after reload":    true,
	"BC01 zh-CN cart-page visible cart Retry preserves key/body and one committed write after reload": true,
	"BC01 zh-TW drawer visible cart Retry preserves key/body and one committed write after reload":    true,
	"BC01 zh-TW cart-page visible cart Retry preserves key/body and one committed write after reload": true,
}

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
	for item := range orderCartRetryCases {
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
	legacy := append([]string{}, complete...)
	for item := range orderCartRetryCases {
		complete = append(complete, item)
	}
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
		{"legacy-without-cart-retry", legacy, false},
		{"with-PR7-UTC", append(append([]string{}, pr7...), orderUTCHistory), true},
		{"duplicate-history-alias", append(append([]string{}, complete...), "BH04 owned history renders each server created_at in Taipei across locales under Los Angeles browser time"), false},
		{"missing-core-even-with-UTC", append(append([]string{}, complete[1:]...), orderUTCHistory), false},
		{"missing-mobile-layout", append(append([]string{}, complete[:24]...), complete[25:]...), false},
		{"missing-desktop-layout", append(append([]string{}, core...), complete[24:]...), false},
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
	for missing := range orderCartRetryCases {
		rows := []string{}
		for _, item := range complete {
			if item != missing {
				rows = append(rows, item)
			}
		}
		if buyerOrderCasesComplete(len(rows), rows) {
			t.Fatalf("missing cart Retry case accepted: %s", missing)
		}
		rows = append(append([]string{}, complete...), missing)
		if buyerOrderCasesComplete(len(rows), rows) {
			t.Fatalf("duplicate cart Retry case accepted: %s", missing)
		}
	}
}

type browserOrderCartFact struct {
	ID             string          `json:"id"`
	Version        int64           `json:"version"`
	Writes         int             `json:"writes"`
	Receipts       int             `json:"receipts"`
	ReceiptVersion int64           `json:"receipt_version"`
	RequestHash    string          `json:"request_hash"`
	Items          json.RawMessage `json:"items"`
}

// Node evidence indentation and PostgreSQL jsonb formatting are not cart facts. Every
// scalar and the complete semantic items object still must match the independent read.
func browserOrderCartFactsEqual(a, b browserOrderCartFact) bool {
	var left, right any
	return a.ID == b.ID && a.Version == b.Version && a.Writes == b.Writes && a.Receipts == b.Receipts && a.ReceiptVersion == b.ReceiptVersion && a.RequestHash == b.RequestHash && json.Unmarshal(a.Items, &left) == nil && json.Unmarshal(b.Items, &right) == nil && reflect.DeepEqual(left, right)
}

func TestBuyerOrderRefreshFactComparison(t *testing.T) {
	pg := browserOrderCartFact{ID: "cart", Version: 1, Writes: 1, Receipts: 1, ReceiptVersion: 1, RequestHash: "exact-request-hash", Items: json.RawMessage(`[{"sku_id": "sku", "quantity": 1}]`)}
	node := pg
	node.Items = json.RawMessage(`[
  {"quantity": 1, "sku_id": "sku"}
]`)
	if reflect.DeepEqual(pg, node) || !browserOrderCartFactsEqual(pg, node) {
		t.Fatal("must accept semantic JSON equality while reproducing the former RawMessage byte-comparison failure")
	}
	for name, change := range map[string]func(*browserOrderCartFact){
		"id":              func(f *browserOrderCartFact) { f.ID = "other" },
		"version":         func(f *browserOrderCartFact) { f.Version++ },
		"writes":          func(f *browserOrderCartFact) { f.Writes++ },
		"receipts":        func(f *browserOrderCartFact) { f.Receipts++ },
		"receipt_version": func(f *browserOrderCartFact) { f.ReceiptVersion++ },
		"request_hash":    func(f *browserOrderCartFact) { f.RequestHash = "different" },
		"quantity":        func(f *browserOrderCartFact) { f.Items = json.RawMessage(`[{"sku_id":"sku","quantity":2}]`) },
		"sku":             func(f *browserOrderCartFact) { f.Items = json.RawMessage(`[{"sku_id":"other","quantity":1}]`) },
		"extra_line": func(f *browserOrderCartFact) {
			f.Items = json.RawMessage(`[{"sku_id":"sku","quantity":1},{"sku_id":"other","quantity":1}]`)
		},
		"extra_field": func(f *browserOrderCartFact) {
			f.Items = json.RawMessage(`[{"sku_id":"sku","quantity":1,"unexpected":true}]`)
		},
		"invalid_json": func(f *browserOrderCartFact) { f.Items = json.RawMessage(`[`) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := node
			change(&changed)
			if browserOrderCartFactsEqual(pg, changed) {
				t.Fatal("different persisted fact incorrectly accepted")
			}
		})
	}
}

// A fresh mount has two explicit loading states: session/cart observation, then real
// CartLines detail hydration. Once its first line appears, continuity is mandatory.
type browserOrderCartSample struct {
	Title         *string `json:"title"`
	Lines         int     `json:"lines"`
	Qty           *string `json:"qty"`
	DetailLoading *string `json:"detail_loading"`
}

func browserOrderCartSamplesValid(surface string, samples []browserOrderCartSample) bool {
	if len(samples) == 0 || (surface != "drawer" && surface != "mount") {
		return false
	}
	seenLine := false
	for i, sample := range samples {
		if surface == "mount" && i == 0 {
			if sample.Title == nil || *sample.Title != "Loading…" || sample.Lines != 0 || sample.Qty != nil || sample.DetailLoading != nil {
				return false
			}
			continue
		}
		if sample.Lines == 1 && sample.Qty != nil && *sample.Qty == "1" && sample.Title == nil && sample.DetailLoading == nil {
			seenLine = true
			continue
		}
		if surface == "mount" && !seenLine && sample.Lines == 0 && sample.Qty == nil {
			initialLoading := sample.Title != nil && *sample.Title == "Loading…" && sample.DetailLoading == nil
			detailLoading := sample.Title == nil && sample.DetailLoading != nil && *sample.DetailLoading == "Loading…"
			if initialLoading || detailLoading {
				continue
			}
		}
		return false
	}
	return seenLine
}

func TestBuyerOrderRefreshSamples(t *testing.T) {
	text := func(s string) *string { return &s }
	initial := browserOrderCartSample{Title: text("Loading…")}
	detail := browserOrderCartSample{DetailLoading: text("Loading…")}
	line := browserOrderCartSample{Lines: 1, Qty: text("1")}
	for _, tc := range []struct {
		name    string
		surface string
		samples []browserOrderCartSample
		want    bool
	}{
		{"mount-direct", "mount", []browserOrderCartSample{initial, line}, true},
		{"mount-visible-detail-loading", "mount", []browserOrderCartSample{initial, detail, line}, true},
		{"drawer-continuous", "drawer", []browserOrderCartSample{line, line}, true},
		{"blank-before-first-line", "mount", []browserOrderCartSample{initial, {}, line}, false},
		{"wrong-loading-copy", "mount", []browserOrderCartSample{initial, {DetailLoading: text("Unavailable")}, line}, false},
		{"empty-even-with-loading", "mount", []browserOrderCartSample{initial, {Title: text("Your cart is empty"), DetailLoading: text("Loading…")}, line}, false},
		{"changed-quantity", "mount", []browserOrderCartSample{initial, {Lines: 1, Qty: text("2")}}, false},
		{"line-loss-after-first-line", "mount", []browserOrderCartSample{initial, line, detail, line}, false},
		{"blank-after-first-line", "mount", []browserOrderCartSample{initial, line, {}, line}, false},
		{"drawer-loading-does-not-excuse-line-loss", "drawer", []browserOrderCartSample{line, detail, line}, false},
		{"drawer-must-start-with-line", "drawer", []browserOrderCartSample{initial, line}, false},
		{"missing-initial-held-read-loading", "mount", []browserOrderCartSample{detail, line}, false},
		{"never-published-line", "mount", []browserOrderCartSample{initial, detail}, false},
		{"extra-line", "mount", []browserOrderCartSample{initial, {Lines: 2, Qty: text("1")}}, false},
		{"loading-with-quantity", "mount", []browserOrderCartSample{initial, {DetailLoading: text("Loading…"), Qty: text("1")}, line}, false},
		{"empty-title-with-line", "mount", []browserOrderCartSample{initial, {Title: text("Your cart is empty"), Lines: 1, Qty: text("1")}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if browserOrderCartSamplesValid(tc.surface, tc.samples) != tc.want {
				t.Fatal("visible cart sample sequence accepted/rejected incorrectly")
			}
		})
	}
}
