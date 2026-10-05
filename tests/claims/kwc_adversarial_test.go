// Purpose: runs the independent K3 adversarial corpus (kwc-v1-adversarial.json) against the
// exported kwc-v2 grammar entry points (ParseContainsV2, ParseForIngest, Parse; kwc-v1 stays frozen for replay).
// Depends on: livecommerce/internal/claims/grammar (pure functions only); encoding/json, os.
// Used by: go test ./tests/claims/... (DB-free; KCC01 adversarial extension).
// Invariants: I18 (an empty corpus file is a failure, not a pass); I11 (no comment text is
// persisted anywhere by these tests beyond the committed fixture itself).
// Status: UNIT (pure grammar; no DB, no Meta).

package claimsadversarial

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"livecommerce/internal/claims/grammar"
)

// adversarialCorpus mirrors tests/claims/kwc-v1-adversarial.json exactly; unknown keys fail
// the decode so a renamed field cannot silently drop cases.
type adversarialCorpus struct {
	GrammarVersion string            `json:"grammar_version"`
	Source         string            `json:"source"`
	Consumers      []string          `json:"consumers"`
	Format         map[string]string `json:"format"`
	Cases          []struct {
		ID       string `json:"id"`
		Category string `json:"category"`
		Safety   string `json:"safety"`
		Input    string `json:"input"`
		Expect   struct {
			Kind     string `json:"kind"`
			Keyword  string `json:"keyword"`
			Quantity int64  `json:"quantity"`
			Explicit bool   `json:"explicit"`
		} `json:"expect"`
		Note string `json:"note"`
	} `json:"cases"`
}

// loadAdversarialCorpus reads and strictly decodes the corpus fixture.
func loadAdversarialCorpus(t *testing.T) adversarialCorpus {
	t.Helper()
	raw, err := os.ReadFile("kwc-v1-adversarial.json")
	if err != nil {
		t.Fatalf("read kwc-v1 adversarial corpus: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var corpus adversarialCorpus
	if err := decoder.Decode(&corpus); err != nil {
		t.Fatalf("decode kwc-v1 adversarial corpus: %v", err)
	}
	return corpus
}

// TestKwcV1AdversarialCorpus runs every adversarial case through ParseContains. A frozen
// failure is a spec violation; a safe_outcome failure is a documented finding (the brief
// is silent and the implementation takes the unsafe direction).
func TestKwcV1AdversarialCorpus(t *testing.T) {
	corpus := loadAdversarialCorpus(t)
	if corpus.GrammarVersion != grammar.VersionContainsV2 {
		t.Fatalf("corpus grammar %q != %q", corpus.GrammarVersion, grammar.VersionContainsV2)
	}
	if len(corpus.Cases) < 120 {
		t.Fatalf("adversarial corpus has %d cases (<120, I18: a shrunk corpus is not a pass)", len(corpus.Cases))
	}
	byCategory := map[string]int{}
	for _, c := range corpus.Cases {
		byCategory[c.Category]++
		got := grammar.ParseContainsV2(c.Input)
		want := grammar.Result{
			Version:  grammar.VersionContainsV2,
			Kind:     grammar.Kind(c.Expect.Kind),
			Keyword:  c.Expect.Keyword,
			Quantity: c.Expect.Quantity,
			Explicit: c.Expect.Explicit,
		}
		if got == want {
			continue
		}
		if c.Safety == "frozen" {
			t.Errorf("FROZEN %s ParseContainsV2(%q) = kind=%s keyword=%q qty=%d explicit=%t; want kind=%s keyword=%q qty=%d explicit=%t (%s)",
				c.ID, c.Input, got.Kind, got.Keyword, got.Quantity, got.Explicit,
				want.Kind, want.Keyword, want.Quantity, want.Explicit, c.Note)
		} else {
			t.Errorf("FINDING %s ParseContainsV2(%q) = kind=%s keyword=%q qty=%d explicit=%t; safe outcome is kind=%s (%s)",
				c.ID, c.Input, got.Kind, got.Keyword, got.Quantity, got.Explicit,
				want.Kind, c.Note)
		}
	}
	// Each category must be non-trivial so the file cannot silently shrink a class away.
	for category, n := range map[string]int{
		"hit": 8, "negation": 8, "question": 8, "multi-fragment": 8, "glued": 8,
		"numbers": 8, "no-keyword": 8, "head-length": 8, "url": 8, "safe-outcome": 8,
	} {
		if byCategory[category] < n {
			t.Errorf("adversarial corpus category %q has %d cases (<%d)", category, byCategory[category], n)
		}
	}
}

// TestKwcV1AdversarialParseForIngest checks, over the whole corpus, the ingest-entry
// invariants that keep §2 "parsing is mode-independent" true: a kw-v1 verdict always wins;
// a kwc-v1 result surfaces only when kw-v1 said NO_MATCH; and every corpus NO_MATCH stays
// NO_MATCH at the single entry point used by manual and Meta intake.
func TestKwcV1AdversarialParseForIngest(t *testing.T) {
	corpus := loadAdversarialCorpus(t)
	for _, c := range corpus.Cases {
		exact := grammar.Parse(c.Input)
		entry := grammar.ParseForIngest(c.Input)
		if exact.Kind != grammar.NoMatch {
			if entry != exact {
				t.Errorf("%s ParseForIngest(%q) = %s kw=%q, but exact kw-v1 gave %s kw=%q: kw-v1 must win",
					c.ID, c.Input, entry.Kind, entry.Keyword, exact.Kind, exact.Keyword)
			}
			continue
		}
		if entry.Version == grammar.Version && entry.Kind != grammar.NoMatch {
			t.Errorf("%s ParseForIngest(%q) produced kw-v1 %s that Parse did not", c.ID, c.Input, entry.Kind)
		}
		if entry.Version != grammar.Version && entry.Version != grammar.VersionContainsV2 {
			t.Errorf("%s ParseForIngest(%q) version %q is neither kw-v1 nor kwc-v2", c.ID, c.Input, entry.Version)
		}
		if c.Expect.Kind == string(grammar.NoMatch) && entry.Kind != grammar.NoMatch {
			t.Errorf("%s ParseForIngest(%q) = %s kw=%q qty=%d: a corpus NO_MATCH became an order candidate at the ingest entry",
				c.ID, c.Input, entry.Kind, entry.Keyword, entry.Quantity)
		}
	}
}
