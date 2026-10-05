// Purpose: independent (K3) adversarial tests for kwc-v1 that need package internals:
// raw byte-length boundaries, invalid UTF-8, and the fuzz/property gate over
// ParseContains and ParseForIngest (no panic, deterministic, keyword ⊆ normalized input,
// single-char table fragments are unevadable, kw-v1 precedence).
// Depends on: grammar.go internals (canonical, isKeyword), contains.go (ParseContains,
// ParseForIngest, negation/question tables); stdlib strings/unicode/utf8.
// Used by: go test ./internal/claims/... and the KCC01 fuzz gate
// (-fuzz='^FuzzParseContainsAdversarial$' -fuzztime=60s).
// Invariants: §2 "parsing is mode-independent"; §2.1 step 0 raw byte bound; I11 (results
// stay redacted; tests never log comment text beyond committed fixtures).
// Status: UNIT (pure functions).

package grammar

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestContainsAdversarialByteBoundaries pins §2.1 step 0: the 256-byte bound applies to
// the RAW input, before the width map or trim can shrink it.
func TestContainsAdversarialByteBoundaries(t *testing.T) {
	// 251 spaces + "A01+2" = 256 raw bytes: allowed, then trimmed to a match.
	if got := ParseContains(strings.Repeat(" ", 251) + "A01+2"); got.Kind != Match || got.Quantity != 2 {
		t.Errorf("256 raw bytes (space pad) = %s kw=%q qty=%d, want MATCH A01 x2", got.Kind, got.Keyword, got.Quantity)
	}
	// 252 spaces + "A01+2" = 257 raw bytes: NO_MATCH even though the trimmed form is short.
	if got := ParseContains(strings.Repeat(" ", 252) + "A01+2"); got.Kind != NoMatch {
		t.Errorf("257 raw bytes (space pad) = %s, want NO_MATCH (bound is on raw input)", got.Kind)
	}
	// 83 CJK chars = 249 bytes + 5 = 254 raw bytes: one CJK boundary run, one fragment.
	if got := ParseContains(strings.Repeat("我", 83) + "A01+2"); got.Kind != Match || got.Quantity != 2 {
		t.Errorf("254 raw bytes (CJK pad) = %s kw=%q qty=%d, want MATCH A01 x2", got.Kind, got.Keyword, got.Quantity)
	}
	// 84 CJK chars = 252 bytes + 5 = 257 raw bytes: NO_MATCH.
	if got := ParseContains(strings.Repeat("我", 84) + "A01+2"); got.Kind != NoMatch {
		t.Errorf("257 raw bytes (CJK pad) = %s, want NO_MATCH", got.Kind)
	}
	// Full-width padding shrinks 3 bytes to 1 under the width map, but the bound still
	// applies to the raw input: 85 full-width spaces = 255 bytes + "A01+2" = 260 → NO_MATCH.
	if got := ParseContains(strings.Repeat("　", 85) + "A01+2"); got.Kind != NoMatch {
		t.Errorf("260 raw bytes of full-width pad = %s, want NO_MATCH (bound before width map)", got.Kind)
	}
	// Exactly at the bound with full-width pad: 82 × 3 = 246 + 5 = 251 → MATCH.
	if got := ParseContains(strings.Repeat("　", 82) + "A01+2"); got.Kind != Match {
		t.Errorf("251 raw bytes of full-width pad = %s, want MATCH", got.Kind)
	}
}

// TestContainsAdversarialInvalidUTF8 pins that any invalid UTF-8 anywhere kills the parse.
func TestContainsAdversarialInvalidUTF8(t *testing.T) {
	for _, in := range []string{"\xff", "A01+2\xff", "\xffA01+2", "A01+\xff2", "我\xff要A01+2", "A01\xc3", "A01+2\xed\xa0\x80"} {
		if got := ParseContains(in); got.Kind != NoMatch {
			t.Errorf("ParseContains(%q) = %s kw=%q, want NO_MATCH for invalid UTF-8", in, got.Kind, got.Keyword)
		}
		if got := ParseForIngest(in); got.Kind != NoMatch {
			t.Errorf("ParseForIngest(%q) = %s kw=%q, want NO_MATCH for invalid UTF-8", in, got.Kind, got.Keyword)
		}
	}
}

// TestContainsAdversarialControlAndMarkBytes covers boundary bytes nobody types on
// purpose: NUL, control characters, bidi marks, joiners. None of them may let a second
// fragment hide inside what looks like one keyword.
func TestContainsAdversarialControlAndMarkBytes(t *testing.T) {
	cases := []struct {
		in   string
		want Result
	}{
		// NUL is a boundary rune; the single remaining fragment still parses.
		{"\x00A01+2", Result{Version: VersionContains, Kind: Match, Keyword: "A01", Quantity: 2, Explicit: true}},
		// Interior NUL splits into two fragments.
		{"A01\x00+2", Result{Version: VersionContains, Kind: NoMatch}},
		// Zero-width joiner inside the head splits it.
		{"A‍01+2", Result{Version: VersionContains, Kind: NoMatch}},
		// Bidi override characters are boundaries, not whitespace.
		{"A01‮+2", Result{Version: VersionContains, Kind: NoMatch}},
		// Soft hyphen splits the fragment.
		{"A01­+2", Result{Version: VersionContains, Kind: NoMatch}},
		// Lookalike letters (Greek Α, Cyrillic А) are covered in the JSON corpus as
		// safe_outcome findings: kwc-v1 drops them as boundary runes and the remaining
		// digits become a DIFFERENT keyword ("А01+2" → MATCH "01" x2).
	}
	for _, c := range cases {
		if got := ParseContains(c.in); got != c.want {
			t.Errorf("ParseContains(%q) = %s kw=%q qty=%d explicit=%t, want %s kw=%q qty=%d explicit=%t",
				c.in, got.Kind, got.Keyword, got.Quantity, got.Explicit,
				c.want.Kind, c.want.Keyword, c.want.Quantity, c.want.Explicit)
		}
	}
}

// TestForIngestAdversarialCorpusCrossCheck reruns the author-frozen spec vectors through
// ParseForIngest and requires the kwc-v1 verdict exactly whenever kw-v1 missed.
func TestForIngestAdversarialCorpusCrossCheck(t *testing.T) {
	file := loadContainsVectors(t)
	for _, v := range file.Parse {
		input := vectorInput(t, v.Input, v.InputHex, v.PadToBytes, v.Pad)
		exact := Parse(input)
		entry := ParseForIngest(input)
		if exact.Kind != NoMatch {
			if entry != exact {
				t.Errorf("%s ParseForIngest = %s, exact kw-v1 = %s: kw-v1 must win", v.ID, entry.Kind, exact.Kind)
			}
			continue
		}
		want := Result{Version: VersionContains, Kind: v.Expect.Kind, Keyword: v.Expect.Keyword, Quantity: v.Expect.Quantity, Explicit: v.Expect.Explicit}
		if want.Kind == NoMatch {
			if entry.Kind != NoMatch {
				t.Errorf("%s ParseForIngest(%q) = %s kw=%q, spec vector says NO_MATCH", v.ID, input, entry.Kind, entry.Keyword)
			}
			continue
		}
		if entry != want {
			t.Errorf("%s ParseForIngest = kind=%s kw=%q qty=%d explicit=%t, want %s kw=%q qty=%d explicit=%t",
				v.ID, entry.Kind, entry.Keyword, entry.Quantity, entry.Explicit,
				want.Kind, want.Keyword, want.Quantity, want.Explicit)
		}
	}
}

// fuzzProperties holds every property that must survive arbitrary fuzzed input.
func fuzzProperties(t *testing.T, text string) {
	t.Helper()
	contains := ParseContains(text)
	containsAgain := ParseContains(text)
	entry := ParseForIngest(text)
	entryAgain := ParseForIngest(text)
	if contains != containsAgain {
		t.Fatalf("non-deterministic ParseContains")
	}
	if entry != entryAgain {
		t.Fatalf("non-deterministic ParseForIngest")
	}
	// Every non-empty keyword must be keyword-shaped and literally present in the
	// normalized input — the grammar may never invent characters.
	normalized, ok := canonical(text)
	for name, r := range map[string]Result{"ParseContains": contains, "ParseForIngest": entry} {
		if r.Keyword == "" {
			if r.Quantity != 0 || r.Explicit {
				t.Fatalf("%s carried quantity/explicit without a keyword", name)
			}
			continue
		}
		if !isKeyword(r.Keyword) {
			t.Fatalf("%s keyword fails ^[A-Z0-9]{1,16}$", name)
		}
		if ok && !strings.Contains(normalized, r.Keyword) {
			t.Fatalf("%s returned a keyword absent from the normalized input", name)
		}
	}
	// Result shape invariants.
	if contains.Kind == Match {
		if contains.Quantity < 1 || contains.Quantity > MaxQuantity {
			t.Fatalf("MATCH quantity %d out of 1..999", contains.Quantity)
		}
		if !contains.Explicit && contains.Quantity != 1 {
			t.Fatalf("implicit MATCH quantity %d != 1", contains.Quantity)
		}
	}
	if contains.Kind == InvalidQuantity && (contains.Quantity != 0 || contains.Explicit) {
		t.Fatalf("INVALID_QUANTITY carried quantity/explicit")
	}
	// Single-character table members cannot be evaded by interposed characters; when one
	// is present in the normalized text the only safe verdict is NO_MATCH.
	if ok {
		for _, frag := range []string{"不", "沒", "没", "別", "别", "勿", "莫", "退", "?", "嗎", "吗", "呢", "嘛", "幾", "几", "怎", "哪", "啥"} {
			if strings.Contains(normalized, frag) && contains.Kind != NoMatch {
				t.Fatalf("single-char table fragment %q present but verdict was %s", frag, contains.Kind)
			}
		}
	}
	// Precedence: a kw-v1 verdict always wins at the ingest entry; a kwc-v1 result may
	// surface only when kw-v1 said NO_MATCH.
	exact := Parse(text)
	if exact.Kind != NoMatch && entry != exact {
		t.Fatalf("kw-v1 %s lost to %s at ParseForIngest", exact.Kind, entry.Kind)
	}
	if exact.Kind == NoMatch && entry.Version == Version && entry.Kind != NoMatch {
		t.Fatalf("ParseForIngest invented a kw-v1 %s that Parse did not produce", entry.Kind)
	}
	if entry.Version != Version && entry.Version != VersionContains {
		t.Fatalf("unknown version %q", entry.Version)
	}
}

// FuzzParseContainsAdversarial is the K3 half of the KCC01 fuzz gate
// (go test -run='^$' -fuzz='^FuzzParseContainsAdversarial$' -fuzztime=60s
// ./internal/claims/grammar). A panic, nondeterminism or a property violation fails.
func FuzzParseContainsAdversarial(f *testing.F) {
	seeds := []string{
		"我要A01+2", "不要A01+1", "A01+1多少錢?", "A01+1 A02+1", "2A01+1", "A01A02+1",
		"A01+0", "A01+1000", "Ａ０１＋２", "", " ", "　", "A01 +2", "A01​+2",
		"A01+2😍", "A01+2 1000元", "0912345678", "#A01", "+1", "我也要", "A01-2",
		"A01x2", "A01*2", "https://shop.tw/A01+2", "請問A01+2", "取 消A01+2",
		"\xff", "A01+2\xff", strings.Repeat("我", 90), "ABCDEFGHIJKLMNOPQ+1",
		"A01++2", "A01+2+3", "+++A01+3", "\x00A01+2", "A01+999",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		// Guard: the corpus runner's invariant — valid long inputs still respect §2.1 step 0.
		if len(text) > MaxTextBytes && utf8.ValidString(text) {
			if got := ParseContains(text); got.Kind != NoMatch {
				t.Fatalf("over-long valid input matched")
			}
		}
		fuzzProperties(t, text)
	})
}
