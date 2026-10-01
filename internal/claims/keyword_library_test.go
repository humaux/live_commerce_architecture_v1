package claims

import (
	"reflect"
	"testing"
)

func TestPlanImportClassification(t *testing.T) {
	t.Parallel()
	cand := func(k, sku string) importCandidate { return importCandidate{Keyword: k, SKUID: sku, MaxQuantity: 3} }
	existing := []existingOffer{
		{Keyword: "A1", SKUID: "sku-a", Active: true},   // same keyword + same SKU: already_present
		{Keyword: "B2", SKUID: "sku-x", Active: false},  // keyword used by another SKU (inactive): keyword_taken
		{Keyword: "OLD", SKUID: "sku-d", Active: true},  // makes sku-d taken
		{Keyword: "OFF", SKUID: "sku-e", Active: false}, // inactive offer does not hold its SKU
	}
	sellable := map[string]bool{"sku-a": true, "sku-b": true, "sku-c": true, "sku-d": true, "sku-e": true, "sku-n": true, "sku-z": false}
	create, conflicts := planImport([]importCandidate{
		cand("Z9", "sku-z"), cand("A1", "sku-a"), cand("B2", "sku-b"), cand("C3", "sku-c"),
		cand("D4", "sku-d"), cand("E5", "sku-e"), cand("C4", "sku-c"), // second keyword for sku-c in the same run
	}, existing, sellable, 200)
	wantCreate := []string{"C3", "E5"}
	var got []string
	for _, c := range create {
		got = append(got, c.Keyword)
	}
	if !reflect.DeepEqual(got, wantCreate) {
		t.Fatalf("created %v, want %v", got, wantCreate)
	}
	want := map[string]string{"A1": ConflictAlreadyPresent, "B2": ConflictKeywordTaken, "D4": ConflictSKUTaken,
		"C4": ConflictSKUTaken, "Z9": ConflictSKUUnavailable}
	if len(conflicts) != len(want) {
		t.Fatalf("conflicts %v, want %v", conflicts, want)
	}
	for _, c := range conflicts {
		if want[c.Keyword] != c.Reason {
			t.Fatalf("keyword %s reason %s, want %s", c.Keyword, c.Reason, want[c.Keyword])
		}
	}
}

func TestPlanImportSessionCapAndDuplicateKeywordInRun(t *testing.T) {
	t.Parallel()
	sellable := map[string]bool{"s1": true, "s2": true, "s3": true}
	// capacity 2 with 1 existing: exactly one more fits; the rest are reported session_full.
	create, conflicts := planImport([]importCandidate{{Keyword: "K1", SKUID: "s1"}, {Keyword: "K2", SKUID: "s2"}, {Keyword: "K3", SKUID: "s3"}},
		[]existingOffer{{Keyword: "EX", SKUID: "s9", Active: true}}, sellable, 2)
	if len(create) != 1 || create[0].Keyword != "K1" || len(conflicts) != 2 || conflicts[0].Reason != ConflictSessionFull {
		t.Fatalf("cap handling wrong: create=%v conflicts=%v", create, conflicts)
	}
	// the same keyword twice in one run (two SKUs): the second is keyword_taken, never a duplicate insert.
	create, conflicts = planImport([]importCandidate{{Keyword: "K1", SKUID: "s1"}, {Keyword: "K1", SKUID: "s2"}}, nil, sellable, 200)
	if len(create) != 1 || len(conflicts) != 1 || conflicts[0].Reason != ConflictKeywordTaken {
		t.Fatalf("duplicate keyword in run: create=%v conflicts=%v", create, conflicts)
	}
}

func TestNormalizeLivePrice(t *testing.T) {
	t.Parallel()
	ptr := func(v int64) *int64 { return &v }
	for _, tc := range []struct {
		in   *int64
		want *int64
		ok   bool
	}{
		{nil, nil, true}, {ptr(0), nil, true}, {ptr(1), ptr(1), true}, {ptr(maxLivePriceMinor), ptr(maxLivePriceMinor), true},
		{ptr(-1), nil, false}, {ptr(maxLivePriceMinor + 1), nil, false},
	} {
		got, ok := normalizeLivePrice(tc.in)
		if ok != tc.ok || (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
			t.Fatalf("normalizeLivePrice(%v) = (%v,%v), want (%v,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
