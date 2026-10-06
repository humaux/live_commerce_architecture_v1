// Purpose: the W3-06B match simulator: SimulateClaim answers "what would real ingest do with this comment in this session" without writing anything. It runs the SAME pure matcher as ingest (grammar.ParseForIngest, then effective, the offer lookup, then offerReason) against the session's offers.
// Depends on: internal/claims/grammar (ParseForIngest); ingest.go (effective, offerReason, offer: the shared matcher, never re-implemented here); merchant.go (readOffers, readWindow); claims.go (authorize, requireSession); SQL clock_timestamp() for the activation comparison, exactly as manual ingest stamps occurred_at.
// Used by: internal/httpapi/keyword_tools.go (POST .../claims/simulate); tests/foundation/keyword_tools_test.go (SIM-PARITY against RecordManualClaim).
// Invariants: contract amendment "W3-06B": read only (no row, receipt, audit or Meta call), the comment is neither stored nor returned (I11), omitted match_mode = the session's current window mode (EXACT when it has no window; integrator ruling 2026-10-07 on the owner's EXACT window default).
// Status: MOCK (REAL_PG gates; no Meta wire).

package claims

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims/grammar"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// maxSimulateBytes bounds a simulated comment. Longer than the grammar's 256 bytes is still a valid sample
// (the grammar answers NO_MATCH, as it does for a long Meta comment); the manual path would reject it with 422.
const maxSimulateBytes = 1024

// SimulateInput is the simulator body. Comment is merchant-typed sample text (redacted under fmt and JSON like
// ManualClaimInput); MatchMode is optional and an omitted value means the session's current window mode (EXACT when the
// session has no window), so the merchant sees what would happen live.
type SimulateInput struct {
	Comment   string    `json:"comment"`
	MatchMode MatchMode `json:"match_mode,omitempty"`
}

// Redaction: SimulateInput carries comment text (decoding is unaffected).
func (SimulateInput) String() string               { return redacted }
func (SimulateInput) GoString() string             { return redacted }
func (SimulateInput) Format(f fmt.State, _ rune)   { _, _ = f.Write([]byte(redacted)) }
func (SimulateInput) MarshalJSON() ([]byte, error) { return redactedJSON, nil }

// SimulatedClaim is the simulator answer. Reason is one of the ingest reasons reachable without bundle or Meta state
// ("" when ACCEPTED). Kind is the parse kind AFTER the mode gate (a kwc result outside CONTAINS reads NO_MATCH).
// Offer is the session's offer when the parsed keyword resolves. TargetQuantity is the line target an ACCEPTED claim
// would SET (never an addition); 0 otherwise. WindowState/WindowMatchMode are the session window's actual state and
// mode: when WindowState is CLOSED real ingest answers WINDOW_CLOSED before every Reason here.
type SimulatedClaim struct {
	Outcome         string    `json:"outcome"`
	Reason          Reason    `json:"reason"`
	MatchMode       MatchMode `json:"match_mode"`
	GrammarVersion  string    `json:"grammar_version"`
	Kind            string    `json:"kind"`
	ParsedKeyword   string    `json:"parsed_keyword"`
	ParsedQuantity  int64     `json:"parsed_quantity"`
	Explicit        bool      `json:"explicit"`
	Offer           *Offer    `json:"offer"`
	TargetQuantity  int64     `json:"target_quantity"`
	WindowState     string    `json:"window_state"`
	WindowMatchMode MatchMode `json:"window_match_mode"`
}

// SimulateClaim simulates one comment for a session (live:read) in the caller's READ COMMITTED platform.WithScope
// transaction. It only reads: the session's window row, its offers and the database clock. ErrInvalid for a bad
// session id, mode or oversized/non-UTF-8 comment; ErrNotFound for a missing session. Called by the simulate route.
func SimulateClaim(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID string, in SimulateInput) (SimulatedClaim, error) {
	if validSimulate(in) != nil || !command.ValidID(sessionID) {
		return SimulatedClaim{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return SimulatedClaim{}, err
	}
	if err := requireSession(ctx, tx, scope, sessionID); err != nil {
		return SimulatedClaim{}, err
	}
	window, err := readWindow(ctx, tx, scope, sessionID, "")
	if err != nil {
		return SimulatedClaim{}, err
	}
	offers, err := readOffers(ctx, tx, scope, sessionID, "")
	if err != nil {
		return SimulatedClaim{}, err
	}
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return SimulatedClaim{}, mapError(err)
	}
	mode, _ := resolveMode(in.MatchMode, window.MatchMode) // omitted = the window's mode; a window-less session reads EXACT (readWindow placeholder)
	out := simulateClaim(in.Comment, mode, offers, now)
	out.WindowState, out.WindowMatchMode = window.State, window.MatchMode
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return SimulatedClaim{}, err
	}
	return out, nil
}

// validSimulate checks the simulator input: an empty or known match mode and the comment bounds.
func validSimulate(in SimulateInput) error {
	if _, err := resolveMode(in.MatchMode, MatchExact); err != nil || len(in.Comment) > maxSimulateBytes || !utf8.ValidString(in.Comment) {
		return command.ErrInvalid
	}
	return nil
}

// resolveMode is the one place a tool resolves an omitted match_mode: the caller's fallback. The simulator passes the session's window
// mode (EXACT, the owner's default, when there is no window); the keyword check passes EXACT. An unknown mode is ErrInvalid.
func resolveMode(mode, fallback MatchMode) (MatchMode, error) {
	if mode == "" {
		return fallback, nil
	}
	if !validMode(mode) {
		return "", command.ErrInvalid
	}
	return mode, nil
}

// simulateClaim is the pure decision: the three ingest functions in ingest's own order, over a read of the offers.
// It is NOT a second matcher: parse (grammar.ParseForIngest), mode gate (effective), keyword lookup, then offerReason are the
// exact calls ingest() makes before it touches a bundle, so BUNDLE_LIMIT and RATE_LIMITED (bundle and Meta state) are the only
// ingest outcomes it cannot reach. mode must already be resolved. Pure.
func simulateClaim(text string, mode MatchMode, offers []Offer, now time.Time) SimulatedClaim {
	if mode == "" {
		mode = MatchExact
	}
	p := effective(grammar.ParseForIngest(text), mode)
	out := SimulatedClaim{Outcome: OutcomeRejected, MatchMode: mode, GrammarVersion: p.Version, Kind: string(p.Kind),
		ParsedKeyword: p.Keyword, ParsedQuantity: p.Quantity, Explicit: p.Explicit}
	if p.Kind == grammar.NoMatch {
		out.Reason = ReasonNoMatch
		return out
	}
	var found *Offer
	for i := range offers {
		if offers[i].Keyword == p.Keyword {
			found = &offers[i]
			break
		}
	}
	if found == nil {
		out.Reason = ReasonUnknownKeyword
		return out
	}
	o := *found
	out.Offer = &o
	if reason := offerReason(p, mode, offer{id: o.ID, keyword: o.Keyword, skuID: o.SKUID, active: o.Active,
		activatedAt: o.ActivatedAt, maxQuantity: o.MaxQuantityPerClaim}, now); reason != "" {
		out.Reason = reason
		return out
	}
	out.Outcome, out.TargetQuantity = OutcomeAccepted, p.Quantity
	return out
}
