// Purpose: REAL_PG gates of the W3-06B keyword tools: SIM-PARITY (the simulator equals real manual ingest on the kw-v1 and kwc vector corpora, in all three window modes), simulator read-only and scoping, conflict check, auto-numbering and the atomic batch deactivate/rename, plus an in-process HTTP smoke.
// Depends on: the lcHarness (live_claims_test.go), internal/claims (SimulateClaim, CheckKeywords, NextKeywords, BatchOffers, RecordManualClaim), internal/httpapi (NewHandler, claims routes), tests/claims/*.json corpora; no Meta wire.
// Used by: scripts/dev/test-focused.sh 'Simulate|KeywordTools'; CI foundation suite.
// Invariants: contract amendment "W3-06B" (read-only simulator, same matcher as ingest, batch all-or-nothing, rename = retire + create, omitted match_mode = the session's window mode, EXACT without a window); I02 (the receipt replays one result).
// Status: REAL_PG, MOCK (manual ingress; no Meta wire).

package foundation_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims"
	"livecommerce/internal/claims/grammar"
	"livecommerce/internal/command"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// ktCorpus reads the parse inputs of the three vector files (kw-v1, kwc-v1 seed, kwc adversarial). Entries that cannot go through
// the manual ingress (control characters, more than 256 bytes, invalid UTF-8, empty) are returned separately: the parity gate runs
// only what real ingest accepts as input, the pure simulator test (internal/claims) covers the rest.
func ktCorpus(t *testing.T) (usable []string, keywords []string) {
	t.Helper()
	type entry struct {
		Input  string `json:"input"`
		Hex    string `json:"input_hex"`
		Pad    int    `json:"pad_to_bytes"`
		Expect struct {
			Keyword string `json:"keyword"`
		} `json:"expect"`
	}
	seen, kw := map[string]bool{}, map[string]bool{}
	for file, key := range map[string]string{"kw-v1-vectors.json": "parse", "kwc-v1-vectors.json": "parse", "kwc-v1-adversarial.json": "cases"} {
		raw, err := os.ReadFile("../claims/" + file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("decode %s: %v", file, err)
		}
		var entries []entry
		if err := json.Unmarshal(doc[key], &entries); err != nil || len(entries) == 0 {
			t.Fatalf("%s %q: %d entries, %v (an empty corpus is not a pass)", file, key, len(entries), err)
		}
		for _, e := range entries {
			if e.Expect.Keyword != "" {
				kw[e.Expect.Keyword] = true
			}
			if e.Hex != "" || e.Pad != 0 || len(e.Input) < 1 || len(e.Input) > grammar.MaxTextBytes || !utf8.ValidString(e.Input) || strings.IndexFunc(e.Input, unicode.IsControl) >= 0 {
				continue
			}
			if !seen[e.Input] {
				seen[e.Input] = true
				usable = append(usable, e.Input)
			}
		}
	}
	for k := range kw {
		keywords = append(keywords, k)
	}
	sort.Strings(keywords)
	sort.Strings(usable)
	return usable, keywords
}

func (h *lcHarness) simulate(token, store, session string, in claims.SimulateInput) (out claims.SimulatedClaim, err error) {
	err = h.do(token, store, func(tx pgx.Tx, s platform.Scope) (e error) {
		out, e = claims.SimulateClaim(h.ctx, tx, s, token, session, in)
		return e
	})
	return out, err
}

func (h *lcHarness) check(token, store, session string, in claims.KeywordCheckInput) (out claims.KeywordCheck, err error) {
	err = h.do(token, store, func(tx pgx.Tx, s platform.Scope) (e error) {
		out, e = claims.CheckKeywords(h.ctx, tx, s, token, session, in)
		return e
	})
	return out, err
}

func (h *lcHarness) next(token, store, session, prefix string, count int) (out claims.KeywordSuggestion, err error) {
	err = h.do(token, store, func(tx pgx.Tx, s platform.Scope) (e error) {
		out, e = claims.NextKeywords(h.ctx, tx, s, token, session, prefix, count)
		return e
	})
	return out, err
}

func (h *lcHarness) batch(token, store, key, session string, in claims.BatchInput) (out claims.BatchResult, err error) {
	err = h.do(token, store, func(tx pgx.Tx, s platform.Scope) (e error) {
		out, e = claims.BatchOffers(h.ctx, tx, s, token, key, session, in)
		return e
	})
	return out, err
}

// TestSimulateParityWithIngest is SIM-PARITY and SIM-KWC: for every corpus comment, in each window mode, the simulator's outcome,
// reason, resolved offer and quantity equal what RecordManualClaim (the real ingest, same session, same offers) answers.
func TestSimulateParityWithIngest(t *testing.T) {
	h := lcSetup(t)
	f := h.f
	texts, keywords := ktCorpus(t)
	// Heads with no offer in the session: the corpus keywords all exist as offers, so add the unresolved-head samples explicitly.
	texts = append(texts, "Z9", "z9+2", "我要Z9+1", "Ｙ８＋３", "ZZ99+0")
	if len(texts) < 150 || len(keywords) < 8 {
		t.Fatalf("corpus too small: %d comments, %d keywords", len(texts), len(keywords))
	}
	s := h.draft(t, f.storeA1)
	skus := lcSKUs(t, f, f.tenantA, f.storeA1, "USD", len(keywords))
	for i, kw := range keywords {
		max := int64(5)
		switch kw {
		case "A1":
			max = 3 // QUANTITY_OVER_MAX path
		case "B2":
			max = 999
		}
		o := h.offer(t, s, kw, skus[i], max)
		if kw == "A01X2" || kw == "101" {
			h.setOffer(t, s, o, max, false) // OFFER_INACTIVE path
		}
	}
	reasons, accepted := map[claims.Reason]int{}, 0
	for m, mode := range []claims.MatchMode{claims.MatchExact, claims.MatchKeywordQtyOnly, claims.MatchKeywordQtyContains} {
		h.open(t, s, mode)
		for i, text := range texts {
			sim, err := h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: text, MatchMode: mode})
			if err != nil {
				t.Fatalf("simulate %q (%s): %v", text, mode, err)
			}
			if omitted, err := h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: text}); err != nil || !reflect.DeepEqual(omitted, sim) {
				t.Fatalf("omitted match_mode must equal the window's mode %s for %q: %+v vs %+v (%v)", mode, text, omitted, sim, err)
			}
			real := h.claim(t, s, claims.ManualClaimInput{ActorLabel: fmt.Sprintf("sim-%d-%d", m, i), Text: text})
			offerID, offerKeyword := "", ""
			if sim.Offer != nil {
				offerID, offerKeyword = sim.Offer.ID, sim.Offer.Keyword
			}
			quantity := sim.ParsedQuantity
			if sim.Offer == nil {
				quantity = 0 // real ingest stores no quantity for an unresolved head (I03)
			}
			if real.Outcome != sim.Outcome || real.Reason != sim.Reason || real.OfferID != offerID || real.Keyword != offerKeyword || real.Quantity != quantity {
				t.Fatalf("SIM-PARITY %s %q: ingest %+v, simulator %+v", mode, text, real, sim)
			}
			if sim.Outcome == claims.OutcomeAccepted && sim.TargetQuantity != real.Quantity {
				t.Fatalf("%s %q: target %d, ingest set %d", mode, text, sim.TargetQuantity, real.Quantity)
			}
			if sim.MatchMode != mode || sim.WindowMatchMode != mode || sim.WindowState != claims.WindowOpen {
				t.Fatalf("%s %q: mode fields %+v", mode, text, sim)
			}
			reasons[sim.Reason]++
			if sim.Outcome == claims.OutcomeAccepted {
				accepted++
			}
		}
	}
	// The gate must actually have crossed every reachable ingest outcome (a corpus that never hits one proves nothing about it).
	for _, r := range []claims.Reason{claims.ReasonNoMatch, claims.ReasonUnknownKeyword, claims.ReasonOfferInactive,
		claims.ReasonInvalidQuantity, claims.ReasonQuantityRequired, claims.ReasonQuantityOverMax} {
		if reasons[r] == 0 {
			t.Errorf("parity corpus never produced %s (counts %v)", r, reasons)
		}
	}
	if accepted == 0 {
		t.Fatal("parity corpus never produced ACCEPTED")
	}
}

// TestSimulateReadOnlyDefaultsAndScope: no row is written, an omitted match_mode is the window's mode (and the window's own mode is reported), a CLOSED window is reported (real ingest answers WINDOW_CLOSED), and the route is live:read scoped.
func TestSimulateReadOnlyDefaultsAndScope(t *testing.T) {
	h := lcSetup(t)
	f := h.f
	s := h.draft(t, f.storeA1)
	skus := lcSKUs(t, f, f.tenantA, f.storeA1, "USD", 2)
	h.offer(t, s, "H1", skus[0], 5)

	// A fresh session has no window row: it reads CLOSED/EXACT, and the rule outcome is still reported.
	closed, err := h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: "H1+2"})
	if err != nil || closed.Outcome != claims.OutcomeAccepted || closed.TargetQuantity != 2 || closed.WindowState != claims.WindowClosed || closed.MatchMode != claims.MatchExact {
		t.Fatalf("closed-window simulation: %+v %v", closed, err)
	}
	// The parse result is kept, and the answer says plainly that live ingest would say WINDOW_CLOSED, and what is never simulated.
	if !closed.WindowClosed || !reflect.DeepEqual(closed.NotSimulated, []string{"bundle_limit", "rate_limit"}) || closed.Offer == nil {
		t.Fatalf("closed window must set window_closed and keep the parse result: %+v", closed)
	}
	beforeClosed := lcDigest(t, f, "claims")
	if real := h.claim(t, s, claims.ManualClaimInput{ActorLabel: "closed", Text: "H1+2"}); real.Reason != claims.ReasonWindowClosed {
		t.Fatalf("real ingest into a closed window: %+v", real)
	}
	lcSameDigest(t, "a closed-window claim persists nothing, so the simulator's rule outcome is not a prediction of a stored row", beforeClosed, lcDigest(t, f, "claims"))
	h.open(t, s, claims.MatchKeywordQtyContains)
	omitted, err := h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: "我要H1+1"})
	if err != nil || omitted.MatchMode != claims.MatchKeywordQtyContains || omitted.Outcome != claims.OutcomeAccepted || omitted.WindowMatchMode != claims.MatchKeywordQtyContains || omitted.WindowState != claims.WindowOpen {
		t.Fatalf("omitted match_mode must be the window's mode (CONTAINS): %+v %v", omitted, err)
	}
	if omitted.WindowClosed {
		t.Fatalf("an OPEN window must not read window_closed: %+v", omitted)
	}
	override, err := h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: "我要H1+1", MatchMode: claims.MatchExact})
	if err != nil || override.MatchMode != claims.MatchExact || override.Reason != claims.ReasonNoMatch || override.WindowMatchMode != claims.MatchKeywordQtyContains {
		t.Fatalf("an explicit mode overrides the window's mode: %+v %v", override, err)
	}
	explicit, err := h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: "我要H1+1", MatchMode: claims.MatchKeywordQtyContains})
	if err != nil || explicit.Outcome != claims.OutcomeAccepted || explicit.TargetQuantity != 1 {
		t.Fatalf("explicit CONTAINS: %+v %v", explicit, err)
	}
	if _, err := h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: "H1", MatchMode: "CONTAINS"}); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("unknown mode: %v", err)
	}
	// A run of simulations changes nothing: no table of the claims/live/storefront/inventory schemas and no receipt row.
	schemas := []string{"live", "claims", "storefront", "inventory"}
	receipts := func() int {
		return countRows(t, f.owner, `SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation LIKE 'live.claim.%'`, f.tenantA, f.storeA1)
	}
	quiet, quietReceipts := lcDigest(t, f, schemas...), receipts()
	for _, text := range []string{"H1", "H1+2", "Z9", "不要H1+1", "H1+9", "我要H1+1"} {
		for _, mode := range []claims.MatchMode{"", claims.MatchExact, claims.MatchKeywordQtyOnly, claims.MatchKeywordQtyContains} {
			if _, err := h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: text, MatchMode: mode}); err != nil {
				t.Fatal(err)
			}
		}
	}
	lcSameDigest(t, "simulator is read only", quiet, lcDigest(t, f, schemas...))
	if receipts() != quietReceipts {
		t.Fatal("the simulator wrote a command receipt")
	}

	// Scope: live:read is enough, store:read alone is forbidden, a foreign-store session and a missing session are 404.
	_, readToken := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "live:read")
	if _, err := h.simulate(readToken, f.storeA1, s, claims.SimulateInput{Comment: "H1"}); err != nil {
		t.Fatalf("live:read principal: %v", err)
	}
	_, storeOnly := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read")
	lcIs(t, lcErr(h.simulate(storeOnly, f.storeA1, s, claims.SimulateInput{Comment: "H1"})), platform.ErrForbidden, "store:read only")
	other := h.draftAs(t, h.token, f.storeA2)
	lcIs(t, lcErr(h.simulate(h.token, f.storeA1, other, claims.SimulateInput{Comment: "H1"})), command.ErrNotFound, "session of another store")
	lcIs(t, lcErr(h.simulate(h.token, f.storeA1, randomUUID(), claims.SimulateInput{Comment: "H1"})), command.ErrNotFound, "missing session")
	lcIs(t, lcErr(h.simulate(h.token, f.storeA1, "not-a-uuid", claims.SimulateInput{Comment: "H1"})), command.ErrInvalid, "bad session id")
	lcIs(t, lcErr(h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: strings.Repeat("a", 1025)})), command.ErrInvalid, "oversized comment")
}

// TestSimulateOmittedModeFollowsWindow: integrator ruling 2026-10-07. Window in KEYWORD_QTY_ONLY and no match_mode: the answer is the
// QTY_ONLY result (a bare keyword is QUANTITY_REQUIRED, what live ingest would say). No window row at all: EXACT (the owner's default).
func TestSimulateOmittedModeFollowsWindow(t *testing.T) {
	h := lcSetup(t)
	f := h.f
	s := h.draft(t, f.storeA1) // no window row yet
	skus := lcSKUs(t, f, f.tenantA, f.storeA1, "USD", 1)
	h.offer(t, s, "A1", skus[0], 3)

	none, err := h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: "A1"})
	if err != nil || none.MatchMode != claims.MatchExact || none.WindowMatchMode != claims.MatchExact || none.WindowState != claims.WindowClosed ||
		none.Outcome != claims.OutcomeAccepted || none.TargetQuantity != 1 {
		t.Fatalf("no window: want EXACT and the bare keyword accepted: %+v %v", none, err)
	}

	h.open(t, s, claims.MatchKeywordQtyOnly)
	omitted, err := h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: "A1"})
	if err != nil || omitted.MatchMode != claims.MatchKeywordQtyOnly || omitted.WindowMatchMode != claims.MatchKeywordQtyOnly || omitted.WindowState != claims.WindowOpen ||
		omitted.Outcome != claims.OutcomeRejected || omitted.Reason != claims.ReasonQuantityRequired {
		t.Fatalf("QTY_ONLY window, no mode: want the QTY_ONLY result QUANTITY_REQUIRED: %+v %v", omitted, err)
	}
	if real := h.claim(t, s, claims.ManualClaimInput{ActorLabel: "live", Text: "A1"}); real.Reason != omitted.Reason {
		t.Fatalf("the simulator must say what live ingest says: ingest %+v, simulator %+v", real, omitted)
	}
	with, err := h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: "A1+2"})
	if err != nil || with.Outcome != claims.OutcomeAccepted || with.TargetQuantity != 2 || with.MatchMode != claims.MatchKeywordQtyOnly {
		t.Fatalf("QTY_ONLY window, A1+2: %+v %v", with, err)
	}
	explicit, err := h.simulate(h.token, f.storeA1, s, claims.SimulateInput{Comment: "A1", MatchMode: claims.MatchExact})
	if err != nil || explicit.MatchMode != claims.MatchExact || explicit.Outcome != claims.OutcomeAccepted || explicit.WindowMatchMode != claims.MatchKeywordQtyOnly {
		t.Fatalf("explicit EXACT overrides the window: %+v %v", explicit, err)
	}
}

// TestKeywordToolsCheckAndNext: the conflict check and auto-numbering against real offers, read only and live:read scoped.
func TestKeywordToolsCheckAndNext(t *testing.T) {
	h := lcSetup(t)
	f := h.f
	s := h.draft(t, f.storeA1)
	skus := lcSKUs(t, f, f.tenantA, f.storeA1, "USD", 6)
	for i, kw := range []string{"A2", "A3", "A1X2", "AB", "101"} {
		h.offer(t, s, kw, skus[i], 3)
	}
	before := lcDigest(t, f, "live", "claims")
	got, err := h.check(h.token, f.storeA1, s, claims.KeywordCheckInput{Keywords: []string{"ａｂ", "AB", "new1", "A1", "a1+2", "A3x4"}, MatchMode: claims.MatchKeywordQtyContains})
	if err != nil || got.SessionID != s || got.MatchMode != claims.MatchKeywordQtyContains || len(got.Items) != 6 {
		t.Fatalf("check: %+v %v", got, err)
	}
	kinds := func(i int) []string {
		out := []string{}
		for _, f := range got.Items[i].Conflicts {
			out = append(out, f.Kind)
		}
		return out
	}
	for i, want := range [][]string{{"normalization_collision"}, {"keyword_taken"}, {}, {"quantity_lookalike"}, {"invalid_keyword"}, {"quantity_lookalike"}} {
		if g := kinds(i); !reflect.DeepEqual(g, want) {
			t.Errorf("proposal %q: %v, want %v", got.Items[i].Input, g, want)
		}
	}
	if got.Items[0].Canonical != "AB" || got.Items[0].Conflicts[0].OfferID == "" || got.Items[3].Conflicts[0].With != "A1X2" {
		t.Fatalf("details: %+v", got.Items)
	}
	// Existing offers: A2/A3 vs A1X2 are no pair; 101 is digits-only in CONTAINS.
	var existing []string
	for _, f := range got.Existing {
		existing = append(existing, f.Kind+":"+f.Keyword)
	}
	if want := []string{"numeric_in_contains:101"}; !reflect.DeepEqual(existing, want) {
		t.Fatalf("existing findings %v, want %v", existing, want)
	}
	lcSameDigest(t, "check is read only", before, lcDigest(t, f, "live", "claims"))
	// A1X2 with A1 absent: the pair appears once A1 exists.
	h.offer(t, s, "A1", skus[5], 3)
	before = lcDigest(t, f, "live", "claims")
	again, err := h.check(h.token, f.storeA1, s, claims.KeywordCheckInput{Keywords: []string{}})
	if err != nil || len(again.Items) != 0 || len(again.Existing) != 1 || again.Existing[0].Kind != "quantity_lookalike" || again.Existing[0].Keyword != "A1X2" || again.Existing[0].With != "A1" || again.MatchMode != claims.MatchExact {
		t.Fatalf("existing pair: %+v %v", again, err)
	}
	// Auto-numbering: A1, A2, A3 used, A1X2 exists; the next free are A4, A5, A6. An inactive offer still holds its keyword.
	next, err := h.next(h.token, f.storeA1, s, "", 3)
	if err != nil || next.Prefix != "A" || !reflect.DeepEqual(next.Keywords, []string{"A4", "A5", "A6"}) {
		t.Fatalf("next: %+v %v", next, err)
	}
	if one, err := h.next(h.token, f.storeA1, s, "b", 0); err != nil || !reflect.DeepEqual(one.Keywords, []string{"B1"}) {
		t.Fatalf("next B: %+v %v", one, err)
	}
	for name, tc := range map[string]struct {
		prefix string
		count  int
	}{"digit prefix": {"A1", 1}, "long prefix": {"ABCDEFGHI", 1}, "count 21": {"A", 21}, "negative": {"A", -1}} {
		lcIs(t, lcErr(h.next(h.token, f.storeA1, s, tc.prefix, tc.count)), command.ErrInvalid, name)
	}
	lcSameDigest(t, "check and next are read only", before, lcDigest(t, f, "live", "claims"))
	// Bounds and scope: 51 proposals are refused; live:read is enough; store:read alone and a foreign-store session are not.
	lcIs(t, lcErr(h.check(h.token, f.storeA1, s, claims.KeywordCheckInput{Keywords: make([]string, 51)})), command.ErrInvalid, "51 keywords")
	lcIs(t, lcErr(h.check(h.token, f.storeA1, s, claims.KeywordCheckInput{Keywords: []string{"A1"}, MatchMode: "CONTAINS"})), command.ErrInvalid, "unknown mode")
	_, readToken := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "live:read")
	if _, err := h.check(readToken, f.storeA1, s, claims.KeywordCheckInput{Keywords: []string{"A9"}}); err != nil {
		t.Fatalf("live:read check: %v", err)
	}
	if _, err := h.next(readToken, f.storeA1, s, "", 1); err != nil {
		t.Fatalf("live:read next: %v", err)
	}
	_, storeOnly := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read")
	lcIs(t, lcErr(h.check(storeOnly, f.storeA1, s, claims.KeywordCheckInput{})), platform.ErrForbidden, "check without live:read")
	lcIs(t, lcErr(h.next(storeOnly, f.storeA1, s, "", 1)), platform.ErrForbidden, "next without live:read")
	other := h.draftAs(t, h.token, f.storeA2)
	lcIs(t, lcErr(h.check(h.token, f.storeA1, other, claims.KeywordCheckInput{})), command.ErrNotFound, "check: session of another store")
	lcIs(t, lcErr(h.next(h.token, f.storeA1, other, "", 1)), command.ErrNotFound, "next: session of another store")

	// Omitted match_mode follows the session's window (no window above: EXACT). In a CONTAINS window a digits-only proposal and the stored
	// digits-only offer are warned; an explicit EXACT overrides the window.
	h.open(t, s, claims.MatchKeywordQtyContains)
	win, err := h.check(h.token, f.storeA1, s, claims.KeywordCheckInput{Keywords: []string{"777"}})
	if err != nil || win.MatchMode != claims.MatchKeywordQtyContains || len(win.Items) != 1 || !reflect.DeepEqual(kindsOf(win.Items[0].Conflicts), []string{"numeric_in_contains"}) ||
		!reflect.DeepEqual(kindsOf(win.Existing), []string{"numeric_in_contains", "quantity_lookalike"}) { // stored 101, then the A1X2 / A1 pair (keyword order)
		t.Fatalf("CONTAINS window, no mode: %+v %v", win, err)
	}
	exact, err := h.check(h.token, f.storeA1, s, claims.KeywordCheckInput{Keywords: []string{"777"}, MatchMode: claims.MatchExact})
	if err != nil || exact.MatchMode != claims.MatchExact || len(exact.Items[0].Conflicts) != 0 {
		t.Fatalf("explicit EXACT in a CONTAINS window: %+v %v", exact, err)
	}
}

func kindsOf(fs []claims.KeywordFinding) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, f.Kind)
	}
	return out
}

// TestKeywordToolsBatch: atomic batch deactivate / rename on a real session: retire + create, receipt replay, all-or-nothing
// conflicts, claims protect an offer from renaming, concurrent create vs rename, permission and shape errors.
func TestKeywordToolsBatch(t *testing.T) {
	h := lcSetup(t)
	f := h.f
	s := h.draft(t, f.storeA1)
	skus := lcSKUs(t, f, f.tenantA, f.storeA1, "USD", 8)
	a1 := h.offer(t, s, "A1", skus[0], 3)
	b2 := h.offer(t, s, "B2", skus[1], 4)
	c3 := h.offer(t, s, "C3", skus[2], 5)
	d4 := h.offer(t, s, "D4", skus[3], 2)
	f5 := h.offer(t, s, "F5", skus[6], 2) // never claimed: the target of the conflict cases below
	price := int64(777)
	var err error
	if b2, err = h.updateOffer(h.token, f.storeA1, t04Key("kt-price"), s, b2.ID, claims.OfferUpdate{ExpectedVersion: b2.Version, MaxQuantityPerClaim: 4, Active: true, LivePriceMinor: &price}); err != nil {
		t.Fatal(err)
	}
	h.open(t, s, claims.MatchExact)
	h.accepted(t, s, "", "amy", "A1+2") // A1 now has a claim line: it can no longer be renamed

	// B2 is the console's recommended offer (the latest 'featured' event): a rename must carry that to the new offer.
	for _, offer := range []string{a1.ID, b2.ID} {
		if err := h.do(h.token, f.storeA1, func(tx pgx.Tx, sc platform.Scope) error {
			_, e := live.RecordOfferFeatured(h.ctx, tx, sc, h.token, s, offer)
			return e
		}); err != nil {
			t.Fatal(err)
		}
	}
	recommended := func() string {
		var id string
		if err := f.owner.QueryRow(h.ctx, `SELECT offer_id::text FROM live.offer_timeline WHERE session_id=$1 AND kind='featured' ORDER BY at DESC,id DESC LIMIT 1`, s).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	if recommended() != b2.ID {
		t.Fatal("setup: B2 must be the recommended offer")
	}

	// 1. rename B2 -> E5 and deactivate C3 in one call.
	key := t04Key("kt-batch")
	items := []claims.BatchItem{
		{OfferID: b2.ID, ExpectedVersion: b2.Version, Action: claims.BatchRename, Keyword: "ｅ５"},
		{OfferID: c3.ID, ExpectedVersion: c3.Version, Action: claims.BatchDeactivate},
	}
	res, err := h.batch(h.token, f.storeA1, key, s, claims.BatchInput{Items: items})
	if err != nil || !res.Applied || len(res.Conflicts) != 0 || len(res.Offers) != 3 || len(res.Unchanged) != 0 {
		t.Fatalf("batch: %+v %v", res, err)
	}
	byKeyword := map[string]claims.Offer{}
	for _, o := range res.Offers {
		byKeyword[o.Keyword] = o
	}
	oldB2, newE5, oldC3 := byKeyword["B2"], byKeyword["E5"], byKeyword["C3"]
	if oldB2.Active || oldB2.Version != b2.Version+1 || oldB2.ID != b2.ID || oldC3.Active || oldC3.Version != c3.Version+1 {
		t.Fatalf("old offers: %+v %+v", oldB2, oldC3)
	}
	if !newE5.Active || newE5.Version != 1 || newE5.SKUID != b2.SKUID || newE5.MaxQuantityPerClaim != 4 || newE5.LivePriceMinor == nil || *newE5.LivePriceMinor != price || newE5.ID == b2.ID {
		t.Fatalf("new offer must copy SKU, cap and live price: %+v", newE5)
	}
	if got := recommended(); got != newE5.ID {
		t.Fatalf("the console's recommended offer is %s after the rename, want the new offer %s (not the retired %s)", got, newE5.ID, b2.ID)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM live.offer_timeline WHERE session_id=$1 AND kind='featured'`, s); n != 3 {
		t.Fatalf("timeline rows %d, want 3 (the old rows stay as history, one new row for E5)", n)
	}
	// The next comment sees the final state: the old keyword is inactive, the new one accepts, D4 is untouched.
	if r := h.claim(t, s, claims.ManualClaimInput{ActorLabel: "bob", Text: "B2"}); r.Reason != claims.ReasonOfferInactive {
		t.Fatalf("old keyword after rename: %+v", r)
	}
	if r := h.claim(t, s, claims.ManualClaimInput{ActorLabel: "cat", Text: "e5+3"}); r.Outcome != claims.OutcomeAccepted || r.Quantity != 3 || r.OfferID != newE5.ID {
		t.Fatalf("new keyword after rename: %+v", r)
	}
	if r := h.claim(t, s, claims.ManualClaimInput{ActorLabel: "dan", Text: "D4"}); r.Outcome != claims.OutcomeAccepted {
		t.Fatalf("untouched offer: %+v", r)
	}
	var details string
	if err := f.owner.QueryRow(h.ctx, `SELECT details::text FROM ops.audit_events WHERE store_id=$1 AND principal_id=$2 AND action='live.claim.offer.batch.applied' ORDER BY id DESC LIMIT 1`,
		f.storeA1, h.actor).Scan(&details); err != nil || !strings.Contains(details, `"renamed": 1`) || !strings.Contains(details, `"deactivated": 2`) {
		t.Fatalf("audit row: %q %v", details, err)
	}
	if strings.Contains(details, "E5") || strings.Contains(details, "B2") {
		t.Fatalf("audit details carry a keyword: %s", details)
	}

	// 2. replay (I02): same key + same body = the stored result, nothing new; same key + other body = conflict.
	offersBefore := countRows(t, f.owner, `SELECT count(*) FROM live.offers WHERE session_id=$1`, s)
	replay, err := h.batch(h.token, f.storeA1, key, s, claims.BatchInput{Items: items})
	if err != nil || !reflect.DeepEqual(replay, res) || countRows(t, f.owner, `SELECT count(*) FROM live.offers WHERE session_id=$1`, s) != offersBefore {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	canon := []claims.BatchItem{{OfferID: b2.ID, ExpectedVersion: b2.Version, Action: claims.BatchRename, Keyword: "E5"}, items[1]}
	if r2, err := h.batch(h.token, f.storeA1, key, s, claims.BatchInput{Items: canon}); err != nil || !reflect.DeepEqual(r2, res) {
		t.Fatalf("canonical-keyword replay (ｅ５ = E5): %+v %v", r2, err)
	}
	lcIs(t, lcErr(h.batch(h.token, f.storeA1, key, s, claims.BatchInput{Items: items[:1]})), command.ErrConflict, "same key, other body")

	// 3. all or nothing: one bad item and the good one is not applied; every refusal is data, not an error.
	snapshot := lcDigest(t, f, "live", "claims")
	for name, tc := range map[string]struct {
		items  []claims.BatchItem
		reason map[int]string
	}{
		"rename an offer with claims": {[]claims.BatchItem{{OfferID: d4.ID, ExpectedVersion: d4.Version, Action: claims.BatchDeactivate},
			{OfferID: a1.ID, ExpectedVersion: a1.Version, Action: claims.BatchRename, Keyword: "F6"}}, map[int]string{1: "has_claims"}},
		"stale version":    {[]claims.BatchItem{{OfferID: d4.ID, ExpectedVersion: d4.Version + 5, Action: claims.BatchDeactivate}}, map[int]string{0: "version_conflict"}},
		"missing offer":    {[]claims.BatchItem{{OfferID: randomUUID(), ExpectedVersion: 1, Action: claims.BatchDeactivate}}, map[int]string{0: "offer_not_found"}},
		"old keyword kept": {[]claims.BatchItem{{OfferID: f5.ID, ExpectedVersion: f5.Version, Action: claims.BatchRename, Keyword: "B2"}}, map[int]string{0: "keyword_taken"}},
		"folded collision": {[]claims.BatchItem{{OfferID: f5.ID, ExpectedVersion: f5.Version, Action: claims.BatchRename, Keyword: "ｂ２"}}, map[int]string{0: "normalization_collision"}},
		"invalid keyword":  {[]claims.BatchItem{{OfferID: f5.ID, ExpectedVersion: f5.Version, Action: claims.BatchRename, Keyword: "F6+1"}}, map[int]string{0: "invalid_keyword"}},
		"same keyword":     {[]claims.BatchItem{{OfferID: f5.ID, ExpectedVersion: f5.Version, Action: claims.BatchRename, Keyword: "ｆ５"}}, map[int]string{0: "same_keyword"}},
		"rename inactive":  {[]claims.BatchItem{{OfferID: c3.ID, ExpectedVersion: c3.Version + 1, Action: claims.BatchRename, Keyword: "F6"}}, map[int]string{0: "offer_inactive"}},
	} {
		got, err := h.batch(h.token, f.storeA1, t04Key("kt-conflict"), s, claims.BatchInput{Items: tc.items})
		if err != nil || got.Applied || len(got.Offers) != 0 {
			t.Fatalf("%s: %+v %v", name, got, err)
		}
		reasons := map[int]string{}
		for _, c := range got.Conflicts {
			reasons[c.Index] = c.Reason
		}
		if !reflect.DeepEqual(reasons, tc.reason) {
			t.Fatalf("%s: conflicts %v, want %v", name, reasons, tc.reason)
		}
	}
	lcSameDigest(t, "refused batches wrote nothing", snapshot, lcDigest(t, f, "live", "claims"))

	// 4. an unsellable SKU refuses the rename (as CreateOffer would); an already inactive offer is "unchanged" for deactivate.
	mustExec(t, f.owner, `UPDATE catalog.skus SET status='archived' WHERE id=$1`, skus[6])
	got, err := h.batch(h.token, f.storeA1, t04Key("kt-sku"), s, claims.BatchInput{Items: []claims.BatchItem{{OfferID: f5.ID, ExpectedVersion: f5.Version, Action: claims.BatchRename, Keyword: "F6"}}})
	if err != nil || got.Applied || len(got.Conflicts) != 1 || got.Conflicts[0].Reason != "sku_unavailable" {
		t.Fatalf("unsellable SKU: %+v %v", got, err)
	}
	un, err := h.batch(h.token, f.storeA1, t04Key("kt-unchanged"), s, claims.BatchInput{Items: []claims.BatchItem{{OfferID: oldC3.ID, ExpectedVersion: oldC3.Version, Action: claims.BatchDeactivate}}})
	if err != nil || !un.Applied || len(un.Offers) != 0 || !reflect.DeepEqual(un.Unchanged, []string{c3.ID}) {
		t.Fatalf("deactivate of an inactive offer: %+v %v", un, err)
	}

	// 5. a concurrent CreateOffer and a rename to the same keyword: exactly one offer ends up holding it.
	e6 := h.offer(t, s, "G7", skus[4], 3)
	var wg sync.WaitGroup
	var createErr error
	var raced claims.BatchResult
	var batchErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, createErr = h.createOffer(h.token, f.storeA1, t04Key("kt-race-create"), s, claims.OfferInput{Keyword: "H8", SKUID: skus[5], MaxQuantityPerClaim: 3})
	}()
	go func() {
		defer wg.Done()
		raced, batchErr = h.batch(h.token, f.storeA1, t04Key("kt-race-batch"), s, claims.BatchInput{Items: []claims.BatchItem{{OfferID: e6.ID, ExpectedVersion: e6.Version, Action: claims.BatchRename, Keyword: "H8"}}})
	}()
	wg.Wait()
	holders := countRows(t, f.owner, `SELECT count(*) FROM live.offers WHERE session_id=$1 AND keyword='H8'`, s)
	renamed := batchErr == nil && raced.Applied
	if holders != 1 || (createErr == nil) == renamed || (createErr != nil && !errors.Is(createErr, command.ErrConflict)) || (batchErr == nil && !renamed && (len(raced.Conflicts) != 1 || raced.Conflicts[0].Reason != "keyword_taken")) {
		t.Fatalf("race: holders=%d createErr=%v batch=%+v %v", holders, createErr, raced, batchErr)
	}

	// 6. permission and shape.
	_, readToken := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "live:read")
	lcIs(t, lcErr(h.batch(readToken, f.storeA1, t04Key("kt-ro"), s, claims.BatchInput{Items: items})), platform.ErrForbidden, "live:read only")
	lcIs(t, lcErr(h.batch(h.token, f.storeA1, t04Key("kt-dup"), s, claims.BatchInput{Items: []claims.BatchItem{items[1], items[1]}})), command.ErrInvalid, "duplicate offer id")
	lcIs(t, lcErr(h.batch(h.token, f.storeA1, t04Key("kt-empty"), s, claims.BatchInput{})), command.ErrInvalid, "empty batch")
	lcIs(t, lcErr(h.batch(h.token, f.storeA1, t04Key("kt-sess"), randomUUID(), claims.BatchInput{Items: items[1:]})), command.ErrNotFound, "missing session")
	otherSession := h.draftAs(t, h.token, f.storeA2)
	lcIs(t, lcErr(h.batch(h.token, f.storeA1, t04Key("kt-foreign"), otherSession, claims.BatchInput{Items: items[1:]})), command.ErrNotFound, "session of another store")

	// 6b. deactivating an offer with claim lines is allowed but warned (like an M4 pause it ends the buyers' live price at their next quote).
	dg, err := h.batch(h.token, f.storeA1, t04Key("kt-warn"), s, claims.BatchInput{Items: []claims.BatchItem{{OfferID: d4.ID, ExpectedVersion: d4.Version, Action: claims.BatchDeactivate}}})
	if err != nil || !dg.Applied || len(dg.Warnings) != 1 || dg.Warnings[0].Kind != "deactivate_has_claims" || dg.Warnings[0].OfferID != d4.ID || dg.Warnings[0].Keyword != "D4" {
		t.Fatalf("deactivate with claims: %+v %v", dg, err)
	}
	// Rename warnings follow the session's window mode: a digits-only target warns under CONTAINS (this session's window is EXACT: silent).
	more := lcSKUs(t, f, f.tenantA, f.storeA1, "USD", 2)
	cs := h.draft(t, f.storeA1)
	h.open(t, cs, claims.MatchKeywordQtyContains)
	x1 := h.offer(t, cs, "X1", more[0], 3)
	cw, err := h.batch(h.token, f.storeA1, t04Key("kt-cwarn"), cs, claims.BatchInput{Items: []claims.BatchItem{{OfferID: x1.ID, ExpectedVersion: x1.Version, Action: claims.BatchRename, Keyword: "777"}}})
	if err != nil || !cw.Applied || len(cw.Warnings) != 1 || cw.Warnings[0].Kind != "numeric_in_contains" || cw.Warnings[0].Keyword != "777" {
		t.Fatalf("numeric rename in a CONTAINS window: %+v %v", cw, err)
	}
	es := h.draft(t, f.storeA1)
	h.open(t, es, claims.MatchExact)
	y1 := h.offer(t, es, "Y1", more[1], 3)
	ew, err := h.batch(h.token, f.storeA1, t04Key("kt-ewarn"), es, claims.BatchInput{Items: []claims.BatchItem{{OfferID: y1.ID, ExpectedVersion: y1.Version, Action: claims.BatchRename, Keyword: "888"}}})
	if err != nil || !ew.Applied || len(ew.Warnings) != 0 {
		t.Fatalf("numeric rename in an EXACT window: %+v %v", ew, err)
	}

	// 7. an archived session is read-only at the database (0124): the batch is refused as invalid, reads still work.
	archived := h.draft(t, f.storeA1)
	ao := h.offer(t, archived, "Q1", skus[7], 3)
	mustExec(t, f.owner, `UPDATE live.sessions SET lifecycle='archived' WHERE id=$1`, archived)
	lcIs(t, lcErr(h.batch(h.token, f.storeA1, t04Key("kt-archived"), archived, claims.BatchInput{Items: []claims.BatchItem{{OfferID: ao.ID, ExpectedVersion: ao.Version, Action: claims.BatchDeactivate}}})), command.ErrInvalid, "archived session")
	if sim, err := h.simulate(h.token, f.storeA1, archived, claims.SimulateInput{Comment: "Q1"}); err != nil || sim.Outcome != claims.OutcomeAccepted {
		t.Fatalf("simulate on an archived session: %+v %v", sim, err)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM live.offers WHERE session_id=$1 AND active`, archived); n != 1 {
		t.Fatalf("archived session offers changed: %d active", n)
	}
}

// TestKeywordToolsHTTP is the in-process HTTP smoke of the four routes through the real handler: 200 bodies and key sets, scope errors.
func TestKeywordToolsHTTP(t *testing.T) {
	h := lcSetup(t)
	f := h.f
	handler := httpapi.NewHandler(f.runtime, httpapi.Options{ClaimLabels: &h.labels})
	s := h.draft(t, f.storeA1)
	skus := lcSKUs(t, f, f.tenantA, f.storeA1, "USD", 3)
	a1 := h.offer(t, s, "A1", skus[0], 3)
	h.offer(t, s, "B2", skus[1], 3)
	h.open(t, s, claims.MatchExact)
	base := "/v1/admin/stores/" + f.storeA1 + "/live-sessions/" + s + "/claims"
	call := func(method, path, token, key, body string) (int, map[string]any) {
		headers := map[string]string{}
		if key != "" {
			headers["Idempotency-Key"] = key
		}
		var w *httptest.ResponseRecorder
		if body == "" { // a GET must carry no body at all (studioRoute)
			r := httptest.NewRequest(method, path, nil)
			if token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			w = httptest.NewRecorder()
			handler.ServeHTTP(w, r)
		} else {
			w = adminRequest(handler, method, path, token, []byte(body), "application/json", headers)
		}
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	if code, out := call("POST", base+"/simulate", h.token, "", `{"comment":"a1+2"}`); code != 200 || out["outcome"] != "ACCEPTED" || out["target_quantity"] != float64(2) || out["match_mode"] != "EXACT" || out["window_state"] != "OPEN" {
		t.Fatalf("simulate: %d %v", code, out)
	}
	if code, out := call("POST", base+"/keywords/check", h.token, "", `{"keywords":["ａ１","c3"]}`); code != 200 || len(out["items"].([]any)) != 2 {
		t.Fatalf("check: %d %v", code, out)
	}
	if code, out := call("GET", base+"/keywords/next?prefix=a&count=2", h.token, "", ""); code != 200 || !reflect.DeepEqual(out["keywords"], []any{"A2", "A3"}) {
		t.Fatalf("next: %d %v", code, out)
	}
	body := fmt.Sprintf(`{"items":[{"offer_id":%q,"expected_version":%d,"action":"rename","keyword":"z9"}]}`, a1.ID, a1.Version)
	if code, _ := call("POST", base+"/offers/batch", h.token, "", body); code != 422 {
		t.Fatalf("batch without a key: %d, want 422", code)
	}
	// A1 has no claim line here, so the rename applies: the old offer (inactive) and the new one are returned.
	if code, out := call("POST", base+"/offers/batch", h.token, t04Key("kt-http"), body); code != 200 || out["applied"] != true || len(out["offers"].([]any)) != 2 {
		t.Fatalf("batch: %d %v", code, out)
	}
	_, storeOnly := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read")
	if code, _ := call("POST", base+"/simulate", storeOnly, "", `{"comment":"A1"}`); code != 403 {
		t.Fatalf("simulate without live:read: %d, want 403", code)
	}
	if code, _ := call("POST", base+"/simulate", "", "", `{"comment":"A1"}`); code != 401 {
		t.Fatalf("simulate without a token: %d, want 401", code)
	}
}
