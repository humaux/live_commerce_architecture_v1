// Purpose: DB-free tests of the W3-06B keyword tools: the conflict analysis, the auto-numbering and the batch plan/validation (the pure cores of CheckKeywords, NextKeywords and BatchOffers).
// Depends on: keyword_tools.go (analyzeKeywords, nextKeywords, planBatch, validBatch); grammar through NormalizeKeyword; no database.
// Used by: go test ./internal/claims (the REAL_PG gates live in tests/foundation/keyword_tools_test.go).
// Invariants: keywords stay immutable (rename = retire + create); a conflict anywhere means nothing is written (all-or-nothing plan).

package claims

import (
	"errors"
	"reflect"
	"testing"

	"livecommerce/internal/command"
)

func kwOffers(pairs ...string) []Offer { // id/keyword pairs
	out := make([]Offer, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, Offer{ID: pairs[i], Keyword: pairs[i+1], SKUID: "sku-" + pairs[i], Active: true, Version: 1, MaxQuantityPerClaim: 3})
	}
	return out
}

func kinds(fs []KeywordFinding) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, f.Kind)
	}
	return out
}

func TestKeywordToolsAnalyzeProposals(t *testing.T) {
	t.Parallel()
	existing := kwOffers("o1", "AB", "o2", "A1", "o3", "A1X2", "o4", "101")
	items, _ := analyzeKeywords(existing, []string{
		"ＡＢ",                // full-width: folds onto the existing AB
		"AB",                // same text as the existing keyword
		"c3",                // free (lower case folds to C3)
		"C3",                // folds onto the earlier proposal c3
		"C3",                // the same text as the previous C3
		"A1+2",              // not a keyword
		"",                  // empty
		"B1",                // free
		"B1X2",              // look-alike of the earlier proposal B1
		"ABCDEFGHIJKLMNOPQ", // 17 characters
		"1",                 // free digits-only, look-alike of nothing (101 is not 1X<digits>)
	}, MatchExact)
	want := [][]string{
		{"normalization_collision"},
		{"keyword_taken"},
		{},
		{"normalization_collision"},
		{"duplicate_in_request"},
		{"invalid_keyword"},
		{"invalid_keyword"},
		{},
		{"quantity_lookalike"},
		{"invalid_keyword"},
		{},
	}
	if len(items) != len(want) {
		t.Fatalf("items %d, want %d", len(items), len(want))
	}
	for i, w := range want {
		if got := kinds(items[i].Conflicts); !reflect.DeepEqual(got, w) {
			t.Errorf("proposal %d %q: findings %v, want %v", i, items[i].Input, got, w)
		}
	}
	if items[0].Canonical != "AB" || !items[0].Valid || items[5].Valid || items[5].Canonical != "" {
		t.Fatalf("canonical/valid: %+v %+v", items[0], items[5])
	}
	if f := items[0].Conflicts[0]; f.Severity != "error" || f.OfferID != "o1" || f.Keyword != "AB" {
		t.Fatalf("collision finding: %+v", f)
	}
	if f := items[8].Conflicts[0]; f.Severity != "warning" || f.Keyword != "B1X2" || f.With != "B1" {
		t.Fatalf("look-alike finding: %+v", f)
	}
}

func TestKeywordToolsAnalyzeExisting(t *testing.T) {
	t.Parallel()
	_, found := analyzeKeywords(kwOffers("o1", "A1", "o2", "A1X2", "o3", "A10", "o4", "H1", "o5", "2H1", "o6", "20"), nil, MatchExact)
	// A1/A10 and H1/2H1 are NOT findings: no mode matches a substring (§2.2, kwc-v2), only A1/A1X2 is a hazard.
	want := []KeywordFinding{{Kind: "quantity_lookalike", Severity: "warning", Keyword: "A1X2", With: "A1"}}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("existing findings %+v, want %+v", found, want)
	}
	_, contains := analyzeKeywords(kwOffers("o1", "A1", "o6", "20", "o7", "101"), nil, MatchKeywordQtyContains)
	want = []KeywordFinding{
		{Kind: "numeric_in_contains", Severity: "warning", Keyword: "101"},
		{Kind: "numeric_in_contains", Severity: "warning", Keyword: "20"},
	}
	if !reflect.DeepEqual(contains, want) {
		t.Fatalf("contains findings %+v, want %+v", contains, want)
	}
	if _, none := analyzeKeywords(kwOffers("o6", "20"), nil, MatchKeywordQtyOnly); len(none) != 0 {
		t.Fatalf("numeric_in_contains outside CONTAINS: %+v", none)
	}
}

func TestKeywordToolsNextKeywords(t *testing.T) {
	t.Parallel()
	got := nextKeywords(kwOffers("o1", "A1", "o2", "A2", "o3", "A4", "o4", "B1"), "A", 3)
	if want := []string{"A3", "A5", "A6"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("next A: %v, want %v", got, want)
	}
	// A1X2 exists: suggesting A1 would create a look-alike pair, so A1 is skipped even though it is unused.
	got = nextKeywords(kwOffers("o1", "A1X2", "o2", "A2"), "A", 2)
	if want := []string{"A3", "A4"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("next skipping look-alike: %v, want %v", got, want)
	}
	if got = nextKeywords(nil, "H", 1); !reflect.DeepEqual(got, []string{"H1"}) {
		t.Fatalf("empty session: %v", got)
	}
	// An inactive offer still holds its keyword (unique per session).
	off := kwOffers("o1", "A1")
	off[0].Active = false
	if got = nextKeywords(off, "A", 1); !reflect.DeepEqual(got, []string{"A2"}) {
		t.Fatalf("inactive keyword reused: %v", got)
	}
}

func TestKeywordToolsNextPrefixValidation(t *testing.T) {
	t.Parallel()
	for prefix, want := range map[string]string{"": "A", "a": "A", "ａ": "A", " sku ": "SKU", "ABCDEFGH": "ABCDEFGH"} {
		got, err := validNextPrefix(prefix)
		if err != nil || got != want {
			t.Errorf("prefix %q => %q %v, want %q", prefix, got, err, want)
		}
	}
	for _, prefix := range []string{"A1", "1", "ABCDEFGHI", "A+", "A B", "K"} {
		if _, err := validNextPrefix(prefix); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("prefix %q: err=%v, want ErrInvalid", prefix, err)
		}
	}
}

func batchOffers() []Offer {
	o := kwOffers("o1", "A1", "o2", "B2", "o3", "C3", "o4", "D4", "o5", "A1X2")
	o[0].Version = 3
	o[1].Version = 1
	o[2].Version, o[2].Active = 2, false // C3 is inactive
	return o
}

func TestKeywordToolsPlanBatch(t *testing.T) {
	t.Parallel()
	claimed := map[string]bool{"o1": true}
	sellable := map[string]bool{"sku-o1": true, "sku-o2": true, "sku-o3": true, "sku-o4": false, "sku-o5": true}
	deact := func(id string, v int64) BatchItem {
		return BatchItem{OfferID: id, ExpectedVersion: v, Action: BatchDeactivate}
	}
	rename := func(id string, v int64, kw string) BatchItem {
		return BatchItem{OfferID: id, ExpectedVersion: v, Action: BatchRename, Keyword: kw}
	}
	for _, tc := range []struct {
		name     string
		items    []BatchItem
		capacity int
		reasons  map[int]string // item index -> reason; empty = clean
		unchange []string
	}{
		{"deactivate", []BatchItem{deact("o1", 3), deact("o2", 1)}, 10, nil, nil},
		{"deactivate inactive is unchanged", []BatchItem{deact("o3", 2)}, 10, nil, []string{"o3"}},
		{"unknown offer", []BatchItem{deact("o9", 1)}, 10, map[int]string{0: "offer_not_found"}, nil},
		{"stale version", []BatchItem{deact("o1", 2)}, 10, map[int]string{0: "version_conflict"}, nil},
		{"rename ok", []BatchItem{rename("o2", 1, "e5")}, 10, nil, nil},
		{"rename inactive", []BatchItem{rename("o3", 2, "E5")}, 10, map[int]string{0: "offer_inactive"}, nil},
		{"rename with claims", []BatchItem{rename("o1", 3, "E5")}, 10, map[int]string{0: "has_claims"}, nil},
		{"rename to itself", []BatchItem{rename("o2", 1, "b2")}, 10, map[int]string{0: "same_keyword"}, nil},
		{"rename onto a taken keyword", []BatchItem{rename("o2", 1, "D4")}, 10, map[int]string{0: "keyword_taken"}, nil},
		{"rename onto a folded taken keyword", []BatchItem{rename("o2", 1, "ｄ４")}, 10, map[int]string{0: "normalization_collision"}, nil},
		{"rename invalid", []BatchItem{rename("o2", 1, "E5+1")}, 10, map[int]string{0: "invalid_keyword"}, nil},
		{"two renames to one folded keyword", []BatchItem{rename("o2", 1, "E5"), rename("o5", 1, "e5")}, 10, map[int]string{1: "normalization_collision"}, nil},
		{"two renames to one typed keyword", []BatchItem{rename("o2", 1, "E5"), rename("o5", 1, "E5")}, 10, map[int]string{1: "duplicate_in_request"}, nil},
		{"swap is refused (old keywords stay reserved)", []BatchItem{rename("o2", 1, "A1")}, 10, map[int]string{0: "keyword_taken"}, nil},
		{"rename sku unavailable", []BatchItem{rename("o4", 1, "E5")}, 10, map[int]string{0: "sku_unavailable"}, nil},
		{"rename needs a free slot", []BatchItem{rename("o2", 1, "E5"), rename("o5", 1, "E6")}, 1, map[int]string{1: "session_full"}, nil},
		{"mixed conflict reports every bad item", []BatchItem{deact("o2", 1), deact("o9", 1), rename("o5", 1, "D4")}, 10, map[int]string{1: "offer_not_found", 2: "keyword_taken"}, nil},
	} {
		plan := planBatch(tc.items, batchOffers(), claimed, sellable, tc.capacity, MatchExact)
		got := map[int]string{}
		for _, c := range plan.Conflicts {
			got[c.Index] = c.Reason
		}
		if len(tc.reasons) == 0 && len(got) == 0 {
			got = nil
		}
		if !reflect.DeepEqual(got, tc.reasons) {
			t.Errorf("%s: conflicts %v, want %v", tc.name, got, tc.reasons)
		}
		if len(tc.unchange) > 0 && !reflect.DeepEqual(plan.Unchanged, tc.unchange) {
			t.Errorf("%s: unchanged %v, want %v", tc.name, plan.Unchanged, tc.unchange)
		}
	}
	// A look-alike target is a warning, never a conflict.
	plan := planBatch([]BatchItem{rename("o2", 1, "A1X3")}, batchOffers(), claimed, sellable, 10, MatchExact)
	if len(plan.Conflicts) != 0 || len(plan.Warnings) != 1 || plan.Warnings[0].Kind != "quantity_lookalike" {
		t.Fatalf("look-alike plan: %+v", plan)
	}
	// The rename warnings follow the window's mode: a digits-only target warns in CONTAINS, not in EXACT.
	numeric := []BatchItem{rename("o2", 1, "777")}
	if plan = planBatch(numeric, batchOffers(), claimed, sellable, 10, MatchKeywordQtyContains); len(plan.Conflicts) != 0 || len(plan.Warnings) != 1 || plan.Warnings[0].Kind != "numeric_in_contains" || plan.Warnings[0].Keyword != "777" {
		t.Fatalf("numeric target in CONTAINS: %+v", plan)
	}
	if plan = planBatch(numeric, batchOffers(), claimed, sellable, 10, MatchExact); len(plan.Conflicts) != 0 || len(plan.Warnings) != 0 {
		t.Fatalf("numeric target in EXACT: %+v", plan)
	}
	// Deactivating an active offer that has claim lines is allowed but warned (it ends the buyers' live price at their next quote, LTG03);
	// an offer without lines, and one already inactive, is silent.
	plan = planBatch([]BatchItem{deact("o1", 3), deact("o2", 1), deact("o3", 2)}, batchOffers(), claimed, sellable, 10, MatchExact)
	if len(plan.Conflicts) != 0 || len(plan.Warnings) != 1 || plan.Warnings[0].Kind != "deactivate_has_claims" || plan.Warnings[0].Keyword != "A1" || plan.Warnings[0].OfferID != "o1" || plan.Warnings[0].Severity != "warning" {
		t.Fatalf("deactivate warning: %+v", plan)
	}
}

func TestKeywordToolsValidBatch(t *testing.T) {
	t.Parallel()
	good := BatchItem{OfferID: "11111111-1111-4111-8111-111111111111", ExpectedVersion: 1, Action: BatchDeactivate}
	rn := BatchItem{OfferID: "22222222-2222-4222-8222-222222222222", ExpectedVersion: 2, Action: BatchRename, Keyword: "ｅ５"}
	if err := validBatch(BatchInput{Items: []BatchItem{good, rn}}); err != nil {
		t.Fatalf("valid batch: %v", err)
	}
	many := make([]BatchItem, 51)
	for i := range many {
		many[i] = good
	}
	for name, in := range map[string]BatchInput{
		"empty":                {},
		"51 items":             {Items: many},
		"duplicate offer":      {Items: []BatchItem{good, good}},
		"bad id":               {Items: []BatchItem{{OfferID: "x", ExpectedVersion: 1, Action: BatchDeactivate}}},
		"zero version":         {Items: []BatchItem{{OfferID: good.OfferID, Action: BatchDeactivate}}},
		"unknown action":       {Items: []BatchItem{{OfferID: good.OfferID, ExpectedVersion: 1, Action: "delete"}}},
		"deactivate + keyword": {Items: []BatchItem{{OfferID: good.OfferID, ExpectedVersion: 1, Action: BatchDeactivate, Keyword: "E5"}}},
		"rename no keyword":    {Items: []BatchItem{{OfferID: good.OfferID, ExpectedVersion: 1, Action: BatchRename}}},
	} {
		if err := validBatch(in); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("%s: err=%v, want ErrInvalid", name, err)
		}
	}
	// An unnormalizable rename keyword is plan data (invalid_keyword), not a malformed request.
	if err := validBatch(BatchInput{Items: []BatchItem{{OfferID: good.OfferID, ExpectedVersion: 1, Action: BatchRename, Keyword: "E5+1"}}}); err != nil {
		t.Fatalf("invalid keyword must reach the plan: %v", err)
	}
}
