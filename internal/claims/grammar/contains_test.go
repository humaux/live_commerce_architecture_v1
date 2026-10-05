// Unit gate KCC01 for kwc-v1 (ParseContains / ParseForIngest, contract §2.5). Owns: the
// kwc-v1 canonical-vector loader (tests/claims/kwc-v1-vectors.json), the
// "Parse != NO_MATCH ⇒ ParseContains agrees" property, ParseForIngest precedence, the
// "append another [A-Z0-9] fragment ⇒ NO_MATCH" property, and FuzzParseContains.
// Non-goals: no database, offers, windows or ingest precedence (package claims owns those).

package grammar

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// containsVectorFile mirrors tests/claims/kwc-v1-vectors.json exactly; unknown keys fail
// the decode so a renamed field cannot silently drop vectors.
type containsVectorFile struct {
	GrammarVersion string            `json:"grammar_version"`
	Source         string            `json:"source"`
	Consumers      []string          `json:"consumers"`
	Format         map[string]string `json:"format"`
	Parse          []struct {
		ID         string `json:"id"`
		Input      string `json:"input"`
		InputHex   string `json:"input_hex"`
		PadToBytes int    `json:"pad_to_bytes"`
		Pad        string `json:"pad"`
		Expect     struct {
			Kind     Kind   `json:"kind"`
			Keyword  string `json:"keyword"`
			Quantity int64  `json:"quantity"`
			Explicit bool   `json:"explicit"`
		} `json:"expect"`
	} `json:"parse"`
}

func loadContainsVectors(t *testing.T) containsVectorFile {
	t.Helper()
	raw, err := os.ReadFile("../../../tests/claims/kwc-v1-vectors.json")
	if err != nil {
		t.Fatalf("read kwc-v1 vectors: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var file containsVectorFile
	if err := decoder.Decode(&file); err != nil {
		t.Fatalf("decode kwc-v1 vectors: %v", err)
	}
	return file
}

// TestContainsKC01Vectors runs every §2.5 spec vector through ParseContains.
func TestContainsKC01Vectors(t *testing.T) {
	file := loadContainsVectors(t)
	if file.GrammarVersion != VersionContains {
		t.Fatalf("vector grammar %q != %q", file.GrammarVersion, VersionContains)
	}
	if len(file.Parse) == 0 {
		t.Fatal("kwc-v1 vector file parse array is empty (I18: SKIP is not PASS)")
	}
	ids := map[string]bool{}
	for _, v := range file.Parse {
		ids[v.ID] = true
		input := vectorInput(t, v.Input, v.InputHex, v.PadToBytes, v.Pad)
		got := ParseContains(input)
		want := Result{Version: VersionContains, Kind: v.Expect.Kind, Keyword: v.Expect.Keyword, Quantity: v.Expect.Quantity, Explicit: v.Expect.Explicit}
		if got != want {
			t.Errorf("%s ParseContains(%q) = kind=%s keyword=%q qty=%d explicit=%t; want kind=%s keyword=%q qty=%d explicit=%t",
				v.ID, input, got.Kind, got.Keyword, got.Quantity, got.Explicit, want.Kind, want.Keyword, want.Quantity, want.Explicit)
		}
	}
	// One representative per category must be present, so the file cannot silently shrink.
	for _, id := range []string{"C01", "C05", "C10", "C20", "C30", "C40", "C43", "C50", "C52", "C60", "C70", "C80", "C82", "C84", "C85", "C86"} {
		if !ids[id] {
			t.Errorf("kwc-v1 vector file lacks %s", id)
		}
	}
}

// TestContainsAgreesWithParse is the §2.5 property: whenever kw-v1 Parse matches, kwc-v1
// ParseContains must agree on (Kind, Keyword, Quantity, Explicit). This is what keeps
// ParseForIngest mode-independent (§2 "Parsing is mode-independent").
func TestContainsAgreesWithParse(t *testing.T) {
	cases := []string{"A1", "a1", "A1+2", "ａ１＋２", "A1+999", "A1+0", "A1+02",
		"0912345678", "ABCDEFGHIJKLMNOP", "a1x2", "\tA1+3\n", " A1 ", "101", "101+2"}
	for _, in := range cases {
		exact := Parse(in)
		if exact.Kind == NoMatch {
			continue
		}
		contains := ParseContains(in)
		if contains.Kind != exact.Kind || contains.Keyword != exact.Keyword ||
			contains.Quantity != exact.Quantity || contains.Explicit != exact.Explicit {
			t.Errorf("Parse(%q)=%#v but ParseContains=%#v", in, exact, contains)
		}
	}
}

// TestParseForIngestPrecedence fixes the entry semantics: exact kw-v1 wins; the kwc-v1
// fallback runs only on a kw-v1 NO_MATCH and only returns a non-NO_MATCH result.
func TestParseForIngestPrecedence(t *testing.T) {
	for _, in := range []string{"A1", "A1+2", "a1+2", "ａ１＋２"} {
		if got, want := ParseForIngest(in), Parse(in); got != want {
			t.Errorf("ParseForIngest(%q) = %#v, want exact %#v", in, got, want)
		}
	}
	if got, want := ParseForIngest("我要H1+1"), (Result{VersionContainsV2, Match, "H1", 1, true}); got != want { // new parses are kwc-v2
		t.Errorf("ParseForIngest(我要H1+1) = %#v, want %#v", got, want)
	}
	if got := ParseForIngest("不要H1+1"); got.Kind != NoMatch || got.Version != Version {
		t.Errorf("ParseForIngest(不要H1+1) = %#v, want kw-v1 NO_MATCH", got)
	}
	if got, want := ParseForIngest("H1+1 H2+1"), Parse("H1+1 H2+1"); got != want {
		t.Errorf("ParseForIngest(H1+1 H2+1) = %#v, want kw-v1 %#v", got, want)
	}
}

// TestContainsAppendedFragmentNoMatch is the §2.5 fuzz property: appending any extra
// [A-Z0-9] fragment (a new run separated by a boundary) to a text that would otherwise
// match forces NO_MATCH (two fragments). The separator must not be a trimmed space, and
// the extra must not just extend the existing keyword.
func TestContainsAppendedFragmentNoMatch(t *testing.T) {
	for _, in := range []string{"我要H1+1", "H1+1", "A1", "A1+2"} {
		for _, extra := range []string{" B2", ",B2", " 2", " 0912345678", "、B2"} {
			if got := ParseContains(in + extra); got.Kind != NoMatch {
				t.Errorf("ParseContains(%q) = %#v, want NO_MATCH after appending %q", in+extra, got, extra)
			}
		}
	}
}

// FuzzParseContains is the KCC01 property gate (run ≥60 s: go test -run='^$'
// -fuzz='^FuzzParseContains$' -fuzztime=60s ./internal/claims/grammar). Seeds are the §2.5
// spec-vector shape.
func FuzzParseContains(f *testing.F) {
	for _, seed := range []string{"我要H1+1", "不要H1+1", "H1+1多少錢?", "H1+1 H2+1", "2H1+1", "A1+2", "A1+0",
		"A1+1000", "h1+1", "ｈ１＋２", "", "   ", "A1 +2", "A1​+2", "A1+2😍", "A1+2 1000元"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		r := ParseContains(text)
		if again := ParseContains(text); again != r {
			t.Fatalf("non-deterministic ParseContains(%q)", text)
		}
		if r.Version != VersionContains {
			t.Fatalf("version %q", r.Version)
		}
		if len(text) > MaxTextBytes && r.Kind != NoMatch {
			t.Fatalf("over-long input matched: %d bytes", len(text))
		}
		// §2.5 property: an exact kw-v1 verdict must agree with the contains verdict.
		if e := Parse(text); e.Kind != NoMatch {
			if r.Kind != e.Kind || r.Keyword != e.Keyword || r.Quantity != e.Quantity || r.Explicit != e.Explicit {
				t.Fatalf("Parse(%q)=%#v disagrees with ParseContains=%#v", text, e, r)
			}
		}
		// A canonical negation/question fragment must never MATCH or INVALID_QUANTITY.
		if s, ok := canonical(text); ok && (containsAny(s, negationFragments) || containsAny(s, questionFragments)) {
			if r.Kind != NoMatch {
				t.Fatalf("negation/question in %q produced %#v", text, r)
			}
		}
		switch r.Kind {
		case Match:
			if !keywordShape.MatchString(r.Keyword) || r.Quantity < 1 || r.Quantity > MaxQuantity || (!r.Explicit && r.Quantity != 1) {
				t.Fatalf("invalid MATCH for %q: keyword=%q qty=%d explicit=%t", text, r.Keyword, r.Quantity, r.Explicit)
			}
		case InvalidQuantity:
			if !keywordShape.MatchString(r.Keyword) || r.Quantity != 0 || r.Explicit {
				t.Fatalf("invalid INVALID_QUANTITY for %q: keyword=%q", text, r.Keyword)
			}
		case NoMatch:
			if r.Keyword != "" || r.Quantity != 0 || r.Explicit {
				t.Fatalf("NO_MATCH carried data for %q", text)
			}
		default:
			t.Fatalf("unknown kind %q", r.Kind)
		}
	})
}
