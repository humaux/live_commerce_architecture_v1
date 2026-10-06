// Purpose: DB-free tests of the W3-06B match simulator: the pure decision table per window mode, the EXACT default, input validation and redaction.
// Depends on: simulate.go (simulateClaim, validSimulate), grammar (through the simulator only); no database.
// Used by: go test ./internal/claims (the SIM-PARITY gate against real ingest lives in tests/foundation/keyword_tools_test.go).
// Invariants: integrator ruling 2026-10-07 (omitted match_mode = the window's mode, EXACT without a window); I11 (comment text never formatted or serialized).

package claims

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/command"
)

// simOffers is the offer set of the table: A1 max 3, H1 max 5, H2 inactive, 101 numeric (max 9).
func simOffers() []Offer {
	t0 := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	mk := func(id, kw string, max int64, active bool) Offer {
		return Offer{ID: id, Keyword: kw, SKUID: "sku-" + id, SKUCode: "C-" + kw, ProductName: "P " + kw, MaxQuantityPerClaim: max, Active: active, Version: 1, ActivatedAt: t0}
	}
	return []Offer{mk("o-a1", "A1", 3, true), mk("o-h1", "H1", 5, true), mk("o-h2", "H2", 5, false), mk("o-101", "101", 9, true)}
}

func TestSimulateDecisionTable(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		mode    MatchMode
		text    string
		outcome string
		reason  Reason
		keyword string // the resolved offer's keyword, "" when none
		target  int64
	}{
		// EXACT (also the default for an empty mode).
		{"exact bare keyword", MatchExact, "A1", OutcomeAccepted, "", "A1", 1},
		{"exact full-width lower plus", MatchExact, "ａ１＋２", OutcomeAccepted, "", "A1", 2},
		{"default mode is EXACT", "", "A1+2", OutcomeAccepted, "", "A1", 2},
		{"default mode refuses a contains comment", "", "我要H1+1", OutcomeRejected, ReasonNoMatch, "", 0},
		{"exact refuses a sentence", MatchExact, "我要H1+1", OutcomeRejected, ReasonNoMatch, "", 0},
		{"over max", MatchExact, "A1+5", OutcomeRejected, ReasonQuantityOverMax, "A1", 0},
		{"zero quantity", MatchExact, "A1+0", OutcomeRejected, ReasonInvalidQuantity, "A1", 0},
		{"unknown keyword", MatchExact, "Z9", OutcomeRejected, ReasonUnknownKeyword, "", 0},
		{"inactive offer", MatchExact, "H2", OutcomeRejected, ReasonOfferInactive, "H2", 0},
		{"phone number is an unknown keyword", MatchExact, "0912345678", OutcomeRejected, ReasonUnknownKeyword, "", 0},
		{"numeric keyword", MatchExact, "101+2", OutcomeAccepted, "", "101", 2},
		// KEYWORD_QTY_ONLY.
		{"qty-only bare keyword", MatchKeywordQtyOnly, "A1", OutcomeRejected, ReasonQuantityRequired, "A1", 0},
		{"qty-only with quantity", MatchKeywordQtyOnly, "A1+3", OutcomeAccepted, "", "A1", 3},
		{"qty-only refuses a sentence", MatchKeywordQtyOnly, "我要A1+3", OutcomeRejected, ReasonNoMatch, "", 0},
		// KEYWORD_QTY_CONTAINS (the owner's acceptance table 2026-10-02).
		{"contains sentence hits", MatchKeywordQtyContains, "我要H1+1", OutcomeAccepted, "", "H1", 1},
		{"contains trailing thanks hits", MatchKeywordQtyContains, "H1+1謝謝", OutcomeAccepted, "", "H1", 1},
		{"contains negation", MatchKeywordQtyContains, "不要H1+1", OutcomeRejected, ReasonNoMatch, "", 0},
		{"contains question", MatchKeywordQtyContains, "H1+1多少錢?", OutcomeRejected, ReasonNoMatch, "", 0},
		{"contains two keywords", MatchKeywordQtyContains, "H1+1 H2+1", OutcomeRejected, ReasonNoMatch, "", 0},
		{"contains glued digit prefix is never carved", MatchKeywordQtyContains, "2H1+1", OutcomeRejected, ReasonUnknownKeyword, "", 0},
		{"contains bare keyword needs a quantity", MatchKeywordQtyContains, "我要H1", OutcomeRejected, ReasonQuantityRequired, "H1", 0},
		{"contains over max", MatchKeywordQtyContains, "我要H1+9", OutcomeRejected, ReasonQuantityOverMax, "H1", 0},
		{"contains inactive", MatchKeywordQtyContains, "我要H2+1", OutcomeRejected, ReasonOfferInactive, "H2", 0},
	} {
		got := simulateClaim(tc.text, tc.mode, simOffers(), now)
		wantMode := tc.mode
		if wantMode == "" {
			wantMode = MatchExact
		}
		gotKeyword := ""
		if got.Offer != nil {
			gotKeyword = got.Offer.Keyword
		}
		if got.Outcome != tc.outcome || got.Reason != tc.reason || gotKeyword != tc.keyword || got.TargetQuantity != tc.target || got.MatchMode != wantMode {
			t.Errorf("%s: %q => %+v, want outcome=%s reason=%s keyword=%q target=%d mode=%s", tc.name, tc.text, got, tc.outcome, tc.reason, tc.keyword, tc.target, wantMode)
		}
	}
}

// TestSimulateParsedFields pins the echo fields: the head is shown even when no offer resolves, and the
// grammar version follows the mode gate (a kwc result outside CONTAINS reads as the exact kw-v1 NO_MATCH).
func TestSimulateParsedFields(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	got := simulateClaim("我要Z9+2", MatchKeywordQtyContains, simOffers(), now)
	if got.ParsedKeyword != "Z9" || got.ParsedQuantity != 2 || !got.Explicit || got.Kind != "MATCH" || got.GrammarVersion != "kwc-v2" || got.Offer != nil {
		t.Fatalf("unknown head echo: %+v", got)
	}
	got = simulateClaim("我要H1+1", MatchExact, simOffers(), now)
	if got.Kind != "NO_MATCH" || got.GrammarVersion != "kw-v1" || got.ParsedKeyword != "" || got.ParsedQuantity != 0 {
		t.Fatalf("gated contains comment: %+v", got)
	}
	got = simulateClaim("A1+0", MatchExact, simOffers(), now)
	if got.Kind != "INVALID_QUANTITY" || got.ParsedKeyword != "A1" || got.ParsedQuantity != 0 || got.Offer == nil || got.Offer.ID != "o-a1" {
		t.Fatalf("invalid quantity echo: %+v", got)
	}
}

// TestSimulateBeforeActivation: real ingest compares occurred_at with activated_at, so a comment "now" on an offer that
// activates later is OFFER_INACTIVE; the simulator uses the same offerReason.
func TestSimulateBeforeActivation(t *testing.T) {
	t.Parallel()
	offers := simOffers()
	offers[0].ActivatedAt = time.Date(2026, 10, 7, 13, 0, 0, 0, time.UTC)
	got := simulateClaim("A1", MatchExact, offers, time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC))
	if got.Reason != ReasonOfferInactive {
		t.Fatalf("comment before activated_at: %+v", got)
	}
}

func TestSimulateValidation(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]SimulateInput{
		"mode unknown":   {Comment: "A1", MatchMode: "CONTAINS"},
		"mode lowercase": {Comment: "A1", MatchMode: "exact"},
		"comment 1025":   {Comment: strings.Repeat("a", 1025)},
		"not utf-8":      {Comment: "A1\xff"},
	} {
		if err := validSimulate(in); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("%s: err=%v, want ErrInvalid", name, err)
		}
	}
	for name, in := range map[string]SimulateInput{
		"empty comment is a NO_MATCH sample": {Comment: ""},
		"multi-line sample":                  {Comment: "A1\nA1"},
		"1024 bytes":                         {Comment: strings.Repeat("a", 1024)},
		"every mode":                         {Comment: "A1", MatchMode: MatchKeywordQtyContains},
	} {
		if err := validSimulate(in); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestSimulateResolveMode: an omitted match_mode takes the caller's fallback (the simulator passes the session's window mode, which is
// EXACT for a session without a window; the keyword check passes EXACT); an explicit mode always wins; an unknown one is invalid.
func TestSimulateResolveMode(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ mode, fallback, want MatchMode }{
		{"", MatchExact, MatchExact},
		{"", MatchKeywordQtyOnly, MatchKeywordQtyOnly},
		{"", MatchKeywordQtyContains, MatchKeywordQtyContains},
		{MatchExact, MatchKeywordQtyOnly, MatchExact},
		{MatchKeywordQtyContains, MatchExact, MatchKeywordQtyContains},
	} {
		if got, err := resolveMode(tc.mode, tc.fallback); err != nil || got != tc.want {
			t.Errorf("resolveMode(%q, %q) = %q %v, want %q", tc.mode, tc.fallback, got, err, tc.want)
		}
	}
	if _, err := resolveMode("CONTAINS", MatchExact); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("unknown mode: %v", err)
	}
}

func TestSimulateRedaction(t *testing.T) {
	t.Parallel()
	in := SimulateInput{Comment: "secret-comment-text"}
	raw, err := json.Marshal(in)
	if err != nil || string(raw) != `"[redacted]"` {
		t.Fatalf("json: %s %v", raw, err)
	}
	for _, verb := range []string{"%v", "%+v", "%s", "%#v"} {
		if out := fmt.Sprintf(verb, in); strings.Contains(out, "secret") {
			t.Fatalf("%s leaked the comment: %s", verb, out)
		}
	}
}
