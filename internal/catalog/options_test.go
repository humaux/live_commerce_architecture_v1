package catalog

// options_test.go: pure catalog-v2 logic of options.go and the v2 validators of catalog.go / collections.go. No SQL:
// visibility, uniqueness and stock hints run against real PG (tests/foundation/catalog_v2_smoke_test.go).

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Summer Dress 2026":            "summer-dress-2026",
		"  --Hello,   World!  ":        "hello-world",
		"50% OFF":                      "50-off",
		"夏季洋裝":                         "", // no ASCII letter or digit: caller falls back to the id prefix
		"":                             "",
		"A_B":                          "a-b",
		"Ünï":                          "n", // only ASCII survives
		"x" + strings.Repeat("y", 100): "x" + strings.Repeat("y", 79),
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q)=%q want %q", in, got, want)
		}
	}
	if got := Slugify(strings.Repeat("ab ", 60)); len(got) > maxSlug || strings.HasSuffix(got, "-") || !validSlug(got) {
		t.Errorf("long title slug %q is not a valid slug", got)
	}
}

func TestValidSlug(t *testing.T) {
	for _, ok := range []string{"a", "a-b", "abc-123", "0", strings.Repeat("a", 80)} {
		if !validSlug(ok) {
			t.Errorf("%q rejected", ok)
		}
	}
	for _, bad := range []string{"", "A", "a--b", "-a", "a-", "a_b", "a b", strings.Repeat("a", 81), "11111111-1111-4111-8111-111111111111", "é"} {
		if validSlug(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestValidOptions(t *testing.T) {
	ok := []OptionAxis{{"Size", []string{"S", "M"}}, {"Color", []string{"Red"}}}
	if !validOptions(nil) || !validOptions([]OptionAxis{}) || !validOptions(ok) {
		t.Fatal("valid options rejected")
	}
	many := make([]string, 51)
	for i := range many {
		many[i] = "v" + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	for name, axes := range map[string][]OptionAxis{
		"four axes":       {{"a", []string{"1"}}, {"b", []string{"1"}}, {"c", []string{"1"}}, {"d", []string{"1"}}},
		"empty name":      {{"", []string{"1"}}},
		"long name":       {{strings.Repeat("n", 31), []string{"1"}}},
		"no values":       {{"a", nil}},
		"51 values":       {{"a", many}},
		"long value":      {{"a", []string{strings.Repeat("v", 41)}}},
		"duplicate value": {{"a", []string{"x", "x"}}},
		"duplicate axis":  {{"a", []string{"1"}}, {"a", []string{"2"}}},
		"padded value":    {{"a", []string{" x"}}},
		"control char":    {{"a", []string{"x\ny"}}},
		"empty value":     {{"a", []string{""}}},
	} {
		if validOptions(axes) {
			t.Errorf("%s accepted", name)
		}
	}
	if !validOptions([]OptionAxis{{strings.Repeat("n", 30), many[:50]}}) {
		t.Error("boundary axis (30 chars, 50 values) rejected")
	}
}

func TestValuesFitAndTitle(t *testing.T) {
	axes := []OptionAxis{{"Size", []string{"S", "M"}}, {"Color", []string{"Red", "Blue"}}}
	for values, want := range map[string]bool{"S|Red": true, "M|Blue": true, "L|Red": false, "S": false, "S|Red|X": false, "Red|S": false, "": false} {
		var v []string
		if values != "" {
			v = strings.Split(values, "|")
		}
		if got := valuesFit(axes, v); got != want {
			t.Errorf("valuesFit(%q)=%v want %v", values, got, want)
		}
	}
	if !valuesFit(nil, nil) || !valuesFit(nil, []string{}) || valuesFit(nil, []string{"S"}) {
		t.Error("axis-less product must take exactly zero values")
	}
	if skuTitle(nil) != "預設" || skuTitle([]string{"S", "Red"}) != "S / Red" || skuTitle([]string{"S"}) != "S" {
		t.Error("derived SKU title wrong")
	}
}

func TestOptAbsentNullValue(t *testing.T) {
	var in PriceInput
	for body, want := range map[string]struct {
		set bool
		val *int64
	}{`{"price_minor":5}`: {false, nil}, `{"price_minor":5,"compare_at_minor":null}`: {true, nil}, `{"price_minor":5,"compare_at_minor":9}`: {true, ptr(9)}} {
		in = PriceInput{}
		if err := json.Unmarshal([]byte(body), &in); err != nil {
			t.Fatal(err)
		}
		if in.CompareAt.Set != want.set || (in.CompareAt.Val == nil) != (want.val == nil) || (want.val != nil && *in.CompareAt.Val != *want.val) {
			t.Errorf("%s -> %+v", body, in.CompareAt)
		}
	}
	// The idempotency hash must tell "unchanged" from "clear": absent is omitted, null is written.
	absent, _ := json.Marshal(PriceInput{PriceMinor: 5, ExpectedVersion: 1})
	clear, _ := json.Marshal(PriceInput{PriceMinor: 5, ExpectedVersion: 1, CompareAt: Opt[int64]{Set: true}})
	if string(absent) == string(clear) || strings.Contains(string(absent), "compare_at") || !strings.Contains(string(clear), `"compare_at_minor":null`) {
		t.Errorf("absent=%s clear=%s", absent, clear)
	}
	if err := json.Unmarshal([]byte(`{"price_minor":5,"compare_at_minor":"x"}`), &in); err == nil {
		t.Error("string compare_at accepted")
	}
}

func ptr(v int64) *int64 { return &v }

func TestProductValidatorsV2(t *testing.T) {
	good := ProductInput{Name: "n", Status: "active", Slug: "n-1", SEOTitle: strings.Repeat("t", 70), SEODescription: strings.Repeat("d", 160), Options: []OptionAxis{{"a", []string{"1"}}}}
	if !validProductInput(good, false) || !validProductInput(ProductInput{Name: "n"}, false) {
		t.Fatal("valid product rejected")
	}
	for name, in := range map[string]ProductInput{
		"archived on create": {Name: "n", Status: "archived"},
		"unknown status":     {Name: "n", Status: "live"},
		"bad slug":           {Name: "n", Slug: "Bad Slug"},
		"uuid slug":          {Name: "n", Slug: "11111111-1111-4111-8111-111111111111"},
		"seo title 71":       {Name: "n", SEOTitle: strings.Repeat("t", 71)},
		"seo desc 161":       {Name: "n", SEODescription: strings.Repeat("d", 161)},
		"bad options":        {Name: "n", Options: []OptionAxis{{"", nil}}},
	} {
		if validProductInput(in, false) {
			t.Errorf("%s accepted", name)
		}
	}
	s := func(v string) *string { return &v }
	if (ProductPatch{ExpectedVersion: 1}).empty() != true || (ProductPatch{Status: s("draft"), ExpectedVersion: 1}).empty() {
		t.Fatal("empty() wrong")
	}
	for name, p := range map[string]ProductPatch{
		"archived ok":  {Status: s("archived")},
		"draft ok":     {Status: s("draft")},
		"seo clear ok": {SEOTitle: s("")},
	} {
		if !validPatch(p) {
			t.Errorf("%s rejected", name)
		}
	}
	for name, p := range map[string]ProductPatch{
		"bad status": {Status: s("live")}, "empty name": {Name: s("")}, "bad slug": {Slug: s("A")}, "long seo": {SEOTitle: s(strings.Repeat("x", 71))},
	} {
		if validPatch(p) {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestSKUCompareAtValidation(t *testing.T) {
	base := SKUInput{ProductID: "11111111-1111-4111-8111-111111111111", Code: "c", PriceMinor: 100}
	if !validSKUInput(base, false) {
		t.Fatal("base rejected")
	}
	for compare, want := range map[int64]bool{101: true, 100: false, 99: false, 1_000_000_000_000: true, 1_000_000_000_001: false} {
		in := base
		in.CompareAtMinor = &compare
		if validSKUInput(in, false) != want {
			t.Errorf("compare_at %d accepted=%v want %v", compare, !want, want)
		}
	}
	in := base
	in.OptionValues = []string{"a", "b", "c", "d"}
	if validSKUInput(in, false) {
		t.Error("four option values accepted")
	}
}

func TestCollectionValidators(t *testing.T) {
	if !validCollectionTitle("x") || validCollectionTitle("") || validCollectionTitle(strings.Repeat("x", 81)) || !validCollectionTitle(strings.Repeat("字", 80)) {
		t.Error("collection title bounds")
	}
	for _, m := range []string{"manual", "newest", "price_asc", "price_desc"} {
		if !collectionSortModes[m] {
			t.Errorf("sort mode %s", m)
		}
	}
	if collectionSortModes["title"] || collectionStatuses["draft"] || !collectionStatuses["hidden"] {
		t.Error("collection enums drifted from migration 0086")
	}
}

func TestPrefixed(t *testing.T) {
	if got := prefixed("s", "id::text,code"); got != "s.id::text,s.code" {
		t.Error(got)
	}
	if n := strings.Count(skuColumns, ",") + 1; n != len(skuFields(&SKU{})) {
		t.Errorf("skuColumns has %d columns but skuFields %d targets", n, len(skuFields(&SKU{})))
	}
	if n := strings.Count(productColumns, ",") + 1; n != len(productFields(&Product{})) {
		t.Errorf("productColumns has %d columns but productFields %d targets", n, len(productFields(&Product{})))
	}
}
