// Purpose: kwc-v2 unit gate: K3 findings F1-F3 fixed, kwc-v1 frozen for replay, v2 never more permissive than v1, fuzz/property gate for ParseContainsV2.
// Depends on: contains.go (ParseContains kwc-v1, ParseForIngest), contains_v2.go (ParseContainsV2), grammar.go internals (canonical, isKeyword); stdlib strings/testing/utf8.
// Used by: go test ./internal/claims/... and the kwc-v2 fuzz gate (-fuzz='^FuzzParseContainsV2$' -fuzztime=60s).
// Invariants: I02 (determinism per stored grammar version: kwc-v1 results unchanged); §2 "Parsing is mode-independent"; safe-miss direction.
// Status: UNIT (pure functions).

package grammar

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// k3Findings are the K3 F1-F3 inputs (all must be NO_MATCH in kwc-v2, still MATCH in frozen kwc-v1).
var k3Findings = []string{
	"請問A01+2", "請問一下A01+2", "A01+2如何", "請問A01+2還有貨", "A01+2價格", // F1
	"取 消A01+2", "取😍消A01+2", "算 了A01+2", "多 少A01+2", "可 否A01+2", "什 麼A01+2", "是 否A01+2", // F2
	"А01+2", "Α1+2", // F3 (Cyrillic А, Greek Α)
}

// TestContainsV2FixesK3Findings pins F1-F3 in v2 and proves v1 replay is byte-for-byte unchanged.
func TestContainsV2FixesK3Findings(t *testing.T) {
	for _, in := range k3Findings {
		if got := ParseContainsV2(in); got != (Result{Version: VersionContainsV2, Kind: NoMatch}) {
			t.Errorf("ParseContainsV2(%q) = %#v, want kwc-v2 NO_MATCH", in, got)
		}
		if got := ParseContains(in); got.Version != VersionContains || got.Kind != Match {
			t.Errorf("frozen ParseContains(%q) = %#v, want kwc-v1 MATCH (v1 must stay unchanged for replay)", in, got)
		}
		if got := ParseForIngest(in); got.Kind != NoMatch {
			t.Errorf("ParseForIngest(%q) = %#v, want NO_MATCH", in, got)
		}
	}
}

// TestContainsV2Variants covers evasion shapes beyond the K3 corpus and the safe hits that must survive.
func TestContainsV2Variants(t *testing.T) {
	no := []string{
		"取，消A01+2", "取​消A01+2", "取́消A01+2", "A01+2 算。了", "多\n少 A01+2", // punctuation / ZWSP / mark / newline between
		"A01+2价格", "A01+2價錢", "A01+2价钱", "请问A01+2", "A01+2如 何", // simplified forms + spaced 如何
		"Ⓐ01+2", "🅰01+2", "A01+2é", "Ａ01+2é", // symbol / trailing lookalike
	}
	for _, in := range no {
		if got := ParseContainsV2(in); got.Kind != NoMatch {
			t.Errorf("ParseContainsV2(%q) = %#v, want NO_MATCH", in, got)
		}
	}
	yes := map[string]Result{
		"我要A01+2":   {VersionContainsV2, Match, "A01", 2, true},
		"A01 +2 麻煩": {VersionContainsV2, NoMatch, "", 0, false}, // "A01 +2" is two fragments
		"麻煩A01+2":   {VersionContainsV2, Match, "A01", 2, true},
		"1000元":     {VersionContainsV2, Match, "1000", 1, false}, // implicit digits: grammar MATCH, QUANTITY_REQUIRED at ingest
		"我要A01+2😍":  {VersionContainsV2, Match, "A01", 2, true},
		"ＡＢ１+２":     {VersionContainsV2, Match, "AB1", 2, true},
		"我要01+2":    {VersionContainsV2, NoMatch, "", 0, false}, // numeric head + explicit +N + non-ASCII
	}
	if got := ParseForIngest("01+2"); got.Version != Version || got.Kind != Match { // pure ASCII numeric keyword stays kw-v1
		t.Errorf("ParseForIngest(01+2) = %#v, want exact kw-v1 MATCH", got)
	}
	for in, want := range yes {
		if got := ParseContainsV2(in); got != want {
			t.Errorf("ParseContainsV2(%q) = %#v, want %#v", in, got, want)
		}
	}
}

// TestContainsV2TablesKeepV1 guarantees the v2 question table is a superset of the frozen v1 one.
func TestContainsV2TablesKeepV1(t *testing.T) {
	have := map[string]bool{}
	for _, q := range questionFragmentsV2 {
		have[q] = true
	}
	for _, q := range questionFragments {
		if !have[q] {
			t.Errorf("kwc-v2 question table dropped kwc-v1 entry %q", q)
		}
	}
	for _, q := range []string{"問", "如何", "價格", "價錢"} {
		if !have[q] {
			t.Errorf("kwc-v2 question table lacks %q", q)
		}
	}
}

// fuzzPropertiesV2 holds the v2 properties: deterministic, v2 never more permissive than v1,
// no keyword invented, boundary-tolerant table entries unevadable, exact kw-v1 agreement.
func fuzzPropertiesV2(t *testing.T, text string) {
	t.Helper()
	v2 := ParseContainsV2(text)
	if v2 != ParseContainsV2(text) {
		t.Fatalf("non-deterministic ParseContainsV2")
	}
	if v2.Kind != NoMatch && v2.Version != VersionContainsV2 {
		t.Fatalf("v2 result carries version %q", v2.Version)
	}
	// Never more permissive than frozen v1: a v2 order candidate implies the same v1 verdict.
	if v1 := ParseContains(text); v2.Kind != NoMatch && (v1.Kind != v2.Kind || v1.Keyword != v2.Keyword || v1.Quantity != v2.Quantity || v1.Explicit != v2.Explicit) {
		t.Fatalf("kwc-v2 produced a verdict kwc-v1 did not")
	}
	if v2.Keyword == "" && (v2.Quantity != 0 || v2.Explicit) {
		t.Fatalf("quantity/explicit without keyword")
	}
	if v2.Keyword != "" {
		if !isKeyword(v2.Keyword) {
			t.Fatalf("keyword not keyword-shaped")
		}
		if n, ok := canonical(text); !ok || !strings.Contains(n, v2.Keyword) {
			t.Fatalf("keyword absent from normalized input")
		}
	}
	// Every table entry, even interleaved with a boundary rune, forces NO_MATCH.
	if n, ok := canonical(text); ok {
		sk := skeletonOf(n)
		for _, e := range append(append([]string{}, negationFragments...), questionFragmentsV2...) {
			if e != "?" && strings.Contains(sk, e) && v2.Kind != NoMatch {
				t.Fatalf("table entry %q present in skeleton but verdict %s", e, v2.Kind)
			}
		}
	}
	// A kw-v1 verdict wins; the ingest entry only ever returns kw-v1 or kwc-v2.
	exact := Parse(text)
	entry := ParseForIngest(text)
	if exact.Kind != NoMatch && (entry != exact || v2 != (Result{Version: VersionContainsV2, Kind: exact.Kind, Keyword: exact.Keyword, Quantity: exact.Quantity, Explicit: exact.Explicit})) {
		t.Fatalf("kw-v1 %s not agreed by v2/ingest entry", exact.Kind)
	}
	if entry.Version != Version && entry.Version != VersionContainsV2 {
		t.Fatalf("ingest entry version %q is neither kw-v1 nor kwc-v2", entry.Version)
	}
}

// FuzzParseContainsV2 is the kwc-v2 fuzz gate (go test -run='^$' -fuzz='^FuzzParseContainsV2$' -fuzztime=60s ./internal/claims/grammar).
func FuzzParseContainsV2(f *testing.F) {
	for _, seed := range append([]string{
		"我要A01+2", "A01+2", "不要A01+1", "A01+1多少錢?", "2A01+1", "Ａ０１＋２", "", "\xff", "A01+2\xff",
		"1000元", "0912345678", "我要01+2", "Ⓐ01+2", "取​消A01+2", strings.Repeat("我", 90), "A01+999", "A01+1000",
	}, k3Findings...) {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > MaxTextBytes && utf8.ValidString(text) && ParseContainsV2(text).Kind != NoMatch {
			t.Fatalf("over-long valid input matched")
		}
		fuzzPropertiesV2(t, text)
	})
}
