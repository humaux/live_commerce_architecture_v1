// Purpose: the W3-06B keyword tools of a live session: CheckKeywords (conflict findings for proposed and existing keywords), NextKeywords (auto-numbering) and BatchOffers (atomic batch deactivate / rename of offers).
// Depends on: live.offers / claims.lines / catalog SKUs through merchant.go (readOffers), keyword_library.go (sellableSKUs, ConflictSKUUnavailable/ConflictSessionFull, importCandidate); claims.go (authorize, requireSession, waitAdvisory, mapError); internal/claims/grammar (NormalizeKeyword, MaxQuantity); command (receipt live.claim.offer.batch, audit).
// Used by: internal/httpapi/keyword_tools.go (keywords/check, keywords/next, offers/batch); tests/foundation/keyword_tools_test.go.
// Invariants: keywords and SKUs are immutable (0060), so a rename retires the old offer and creates a new one in the same transaction; a batch is all-or-nothing (conflicts are data, nothing written); the session's `claims-offers` advisory lock serialises with CreateOffer/ImportOffers; ambiguity findings come only from the frozen grammar (no mode matches a substring).
// Status: MOCK (REAL_PG gates; no Meta wire).

package claims

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims/grammar"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// Finding kinds (contract amendment "W3-06B", rule 2). The two taken/collision kinds reuse the import vocabulary where it exists.
const (
	FindingInvalid         = "invalid_keyword"         // error: not ^[A-Z0-9]{1,16}$ after normalisation
	FindingTaken           = ConflictKeywordTaken      // error: an offer of the session already holds the canonical keyword
	FindingDuplicate       = "duplicate_in_request"    // error: the same raw text twice in one request
	FindingCollision       = "normalization_collision" // error: different raw text folds onto an existing or earlier keyword
	FindingLookalike       = "quantity_lookalike"      // warning: B = A + "X" + digits, so a typed "a1x2" parses to head A1X2 (§12 R09)
	FindingNumericContains = "numeric_in_contains"     // warning: digits-only keyword under KEYWORD_QTY_CONTAINS
)

// Finding severities.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Bounds of the tools.
const (
	maxCheckKeywords = 50 // proposed keywords per check request
	maxBatchItems    = 50 // items per batch
	maxNextCount     = 20 // suggestions per call
	maxPrefixLen     = 8  // letters; prefix + up to 4 digits stays within the 16-character keyword limit
	maxNextNumber    = 9999
)

// Batch actions and the receipt/audit names of BatchOffers.
const (
	BatchDeactivate        = "deactivate"
	BatchRename            = "rename"
	batchReceiptOperation  = "live.claim.offer.batch"
	batchAuditAction       = "live.claim.offer.batch.applied"
	batchReasonNotFound    = "offer_not_found"
	batchReasonVersion     = "version_conflict"
	batchReasonInactive    = "offer_inactive"
	batchReasonHasClaims   = "has_claims"
	batchReasonSameKeyword = "same_keyword"
)

// KeywordFinding is one conflict or warning. Keyword is the canonical keyword the finding is about (the raw text for
// invalid_keyword); With is the other keyword involved (look-alikes); OfferID is the stored offer that already holds the
// canonical keyword (taken / collision).
type KeywordFinding struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Keyword  string `json:"keyword"`
	With     string `json:"with,omitempty"`
	OfferID  string `json:"offer_id,omitempty"`
}

// KeywordCheckInput is the check body: raw proposed keywords (0..50) and an optional mode (omitted = EXACT).
type KeywordCheckInput struct {
	Keywords  []string  `json:"keywords"`
	MatchMode MatchMode `json:"match_mode,omitempty"`
}

// KeywordCheckItem is the verdict for one proposed keyword, in request order.
type KeywordCheckItem struct {
	Input     string           `json:"input"`
	Canonical string           `json:"canonical"` // "" when invalid
	Valid     bool             `json:"valid"`
	Conflicts []KeywordFinding `json:"conflicts"` // errors and warnings; never null
}

// KeywordCheck is the check answer: one item per proposed keyword plus the findings among the session's existing offers.
type KeywordCheck struct {
	SessionID string             `json:"session_id"`
	MatchMode MatchMode          `json:"match_mode"`
	Items     []KeywordCheckItem `json:"items"`
	Existing  []KeywordFinding   `json:"existing"`
}

// CheckKeywords analyses proposed keywords against the session's offers (live:read). Read only; findings never block by
// themselves (the caller decides). ErrInvalid for a bad session id, mode or more than 50 keywords; ErrNotFound for a missing
// session. Called by the keywords/check route.
func CheckKeywords(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID string, in KeywordCheckInput) (KeywordCheck, error) {
	mode, err := resolveMode(in.MatchMode)
	if err != nil || !command.ValidID(sessionID) || len(in.Keywords) > maxCheckKeywords {
		return KeywordCheck{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return KeywordCheck{}, err
	}
	if err := requireSession(ctx, tx, scope, sessionID); err != nil {
		return KeywordCheck{}, err
	}
	offers, err := readOffers(ctx, tx, scope, sessionID, "")
	if err != nil {
		return KeywordCheck{}, err
	}
	items, existing := analyzeKeywords(offers, in.Keywords, mode)
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return KeywordCheck{}, err
	}
	return KeywordCheck{SessionID: sessionID, MatchMode: mode, Items: items, Existing: existing}, nil
}

// analyzeKeywords is the pure conflict analysis. proposals are raw texts; existing are the session's offers (any active state:
// the keyword is unique per session either way). Per proposal: invalid_keyword; else keyword_taken / normalization_collision
// against an existing offer (collision when the raw text differs from its canonical form), else duplicate_in_request /
// normalization_collision against an earlier proposal; plus quantity_lookalike warnings against existing keywords and
// earlier proposals, and numeric_in_contains in CONTAINS mode. The second result lists the warnings among the existing
// offers alone. Output order is deterministic. Pure.
func analyzeKeywords(existing []Offer, proposals []string, mode MatchMode) ([]KeywordCheckItem, []KeywordFinding) {
	held := map[string]string{} // canonical keyword -> offer id
	keys := make([]string, 0, len(existing)+len(proposals))
	for _, o := range existing {
		held[o.Keyword] = o.ID
		keys = append(keys, o.Keyword)
	}
	sort.Strings(keys)
	items := make([]KeywordCheckItem, len(proposals))
	typed := map[string]map[string]bool{} // canonical -> raw texts of earlier proposals
	for i, raw := range proposals {
		canonical, ok := grammar.NormalizeKeyword(raw)
		item := KeywordCheckItem{Input: raw, Canonical: canonical, Valid: ok, Conflicts: []KeywordFinding{}}
		if !ok {
			item.Conflicts = append(item.Conflicts, KeywordFinding{Kind: FindingInvalid, Severity: SeverityError, Keyword: raw})
			items[i] = item
			continue
		}
		if id, taken := held[canonical]; taken {
			kind := FindingTaken
			if raw != canonical {
				kind = FindingCollision
			}
			item.Conflicts = append(item.Conflicts, KeywordFinding{Kind: kind, Severity: SeverityError, Keyword: canonical, OfferID: id})
		} else if earlier := typed[canonical]; earlier != nil {
			kind := FindingCollision
			if earlier[raw] {
				kind = FindingDuplicate
			}
			item.Conflicts = append(item.Conflicts, KeywordFinding{Kind: kind, Severity: SeverityError, Keyword: canonical})
		}
		for _, other := range keys {
			if other != canonical && quantityLookalike(canonical, other) {
				item.Conflicts = append(item.Conflicts, KeywordFinding{Kind: FindingLookalike, Severity: SeverityWarning, Keyword: canonical, With: other})
			}
		}
		if mode == MatchKeywordQtyContains && digitsOnly(canonical) {
			item.Conflicts = append(item.Conflicts, KeywordFinding{Kind: FindingNumericContains, Severity: SeverityWarning, Keyword: canonical})
		}
		if typed[canonical] == nil {
			typed[canonical] = map[string]bool{}
			if _, isHeld := held[canonical]; !isHeld {
				keys = insertSorted(keys, canonical) // later proposals compare against this one
			}
		}
		typed[canonical][raw] = true
		items[i] = item
	}
	return items, existingFindings(existing, mode)
}

// existingFindings lists the look-alike pairs (longer keyword first, With = the base) and, in CONTAINS mode, the
// digits-only keywords among the session's offers, ordered by keyword. Pure.
func existingFindings(existing []Offer, mode MatchMode) []KeywordFinding {
	keys := make([]string, 0, len(existing))
	for _, o := range existing {
		keys = append(keys, o.Keyword)
	}
	sort.Strings(keys)
	out := []KeywordFinding{}
	for _, k := range keys {
		for _, base := range keys {
			if len(base) < len(k) && quantityLookalike(k, base) {
				out = append(out, KeywordFinding{Kind: FindingLookalike, Severity: SeverityWarning, Keyword: k, With: base})
			}
		}
		if mode == MatchKeywordQtyContains && digitsOnly(k) {
			out = append(out, KeywordFinding{Kind: FindingNumericContains, Severity: SeverityWarning, Keyword: k})
		}
	}
	return out
}

// quantityLookalike reports whether one keyword is the other plus "X" and digits ("A1" / "A1X2"). The grammar parses a
// buyer's "a1x2" as head A1X2 (§12 R09), so with both offers present the comment lands on the longer one.
func quantityLookalike(a, b string) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	rest, ok := strings.CutPrefix(b, a+"X")
	return ok && rest != "" && digitsOnly(rest)
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func insertSorted(keys []string, k string) []string {
	i := sort.SearchStrings(keys, k)
	keys = append(keys, "")
	copy(keys[i+1:], keys[i:])
	keys[i] = k
	return keys
}

// KeywordSuggestion is the auto-numbering answer.
type KeywordSuggestion struct {
	Prefix   string   `json:"prefix"`
	Keywords []string `json:"keywords"`
}

// NextKeywords suggests the next count free keywords PREFIX1, PREFIX2, ... for a session (live:read). prefix is 1..8 letters
// ("" = "A", folded like keywords); count is 1..20 (0 = 1). Used and look-alike candidates are skipped (nextKeywords). A
// suggestion is advisory, not reserved. Read only. ErrInvalid for a bad prefix/count/session id, ErrNotFound for a missing
// session, ErrConflict when the session has no free number left. Called by the keywords/next route.
func NextKeywords(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID, prefix string, count int) (KeywordSuggestion, error) {
	prefix, err := validNextPrefix(prefix)
	if count == 0 {
		count = 1
	}
	if err != nil || !command.ValidID(sessionID) || count < 1 || count > maxNextCount {
		return KeywordSuggestion{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return KeywordSuggestion{}, err
	}
	if err := requireSession(ctx, tx, scope, sessionID); err != nil {
		return KeywordSuggestion{}, err
	}
	offers, err := readOffers(ctx, tx, scope, sessionID, "")
	if err != nil {
		return KeywordSuggestion{}, err
	}
	keywords := nextKeywords(offers, prefix, count)
	if len(keywords) < count {
		return KeywordSuggestion{}, command.ErrConflict
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return KeywordSuggestion{}, err
	}
	return KeywordSuggestion{Prefix: prefix, Keywords: keywords}, nil
}

// validNextPrefix folds the prefix like a keyword and requires 1..8 letters ("" = "A").
func validNextPrefix(prefix string) (string, error) {
	if prefix == "" {
		return "A", nil
	}
	canonical, ok := grammar.NormalizeKeyword(prefix)
	if !ok || len(canonical) > maxPrefixLen {
		return "", command.ErrInvalid
	}
	for i := 0; i < len(canonical); i++ {
		if canonical[i] < 'A' || canonical[i] > 'Z' {
			return "", command.ErrInvalid
		}
	}
	return canonical, nil
}

// nextKeywords returns up to count free keywords prefix+n (n from 1): not used by any offer (active or not) and not a
// quantity look-alike of a used or already suggested keyword. Plain prefix forms (A1 / A10) are normal numbering and stay
// allowed. Pure.
func nextKeywords(offers []Offer, prefix string, count int) []string {
	used := map[string]bool{}
	keys := make([]string, 0, len(offers)+count)
	for _, o := range offers {
		used[o.Keyword] = true
		keys = append(keys, o.Keyword)
	}
	out := []string{}
	for n := 1; n <= maxNextNumber && len(out) < count; n++ {
		candidate := prefix + strconv.Itoa(n)
		if used[candidate] {
			continue
		}
		clash := false
		for _, k := range keys {
			if quantityLookalike(candidate, k) {
				clash = true
				break
			}
		}
		if clash {
			continue
		}
		out = append(out, candidate)
		keys = append(keys, candidate)
	}
	return out
}

// BatchItem is one batch action. Keyword is the raw new keyword of a rename (forbidden for deactivate).
type BatchItem struct {
	OfferID         string `json:"offer_id"`
	ExpectedVersion int64  `json:"expected_version"`
	Action          string `json:"action"` // deactivate | rename
	Keyword         string `json:"keyword,omitempty"`
}

// BatchInput is the batch body (1..50 items).
type BatchInput struct {
	Items []BatchItem `json:"items"`
}

// BatchConflict is a refused item: Index is its position in the request; Keyword is the typed rename keyword.
type BatchConflict struct {
	Index   int    `json:"index"`
	OfferID string `json:"offer_id"`
	Keyword string `json:"keyword,omitempty"`
	Reason  string `json:"reason"`
}

// BatchResult is the batch answer. Applied=false means a conflict was found and nothing was written (conflicts are data,
// HTTP 200). Offers are the post-state of every offer written (deactivated and newly created); Unchanged lists offers that were
// already inactive (a deactivate no-op). Warnings are the non-blocking findings of the rename targets. Slices are never null.
type BatchResult struct {
	Applied   bool             `json:"applied"`
	Offers    []Offer          `json:"offers"`
	Unchanged []string         `json:"unchanged"`
	Conflicts []BatchConflict  `json:"conflicts"`
	Warnings  []KeywordFinding `json:"warnings"`
}

// batchPlan is planBatch's verdict.
type batchPlan struct {
	Conflicts []BatchConflict
	Warnings  []KeywordFinding
	Unchanged []string
}

// BatchOffers deactivates or renames several offers of one session in one transaction (live:manage): one receipt
// live.claim.offer.batch, one audit row, all or nothing. A rename is "retire the old offer, create the new one" (keywords are
// immutable, 0060) copying SKU, max quantity and live price; the old offer keeps its history and its keyword stays reserved.
// It takes the session's `claims-offers` advisory lock (the CreateOffer/ImportOffers lock) and locks the touched offers
// FOR NO KEY UPDATE in id order, so in-flight ingests drain first and a later ingest sees the final state. Any conflict returns
// Applied=false with the conflicts and writes nothing but the receipt. ErrInvalid for a malformed batch, ErrNotFound for a
// missing session, ErrConflict for the same key with a different body. Called by the offers/batch route.
func BatchOffers(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, sessionID string, in BatchInput) (BatchResult, error) {
	if !command.ValidID(sessionID) || validBatch(in) != nil {
		return BatchResult{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return BatchResult{}, err
	}
	// The canonical request keeps the folded keyword (so "ｅ５" and "E5" replay one receipt); an unfoldable one stays raw.
	canonical := make([]BatchItem, len(in.Items))
	for i, it := range in.Items {
		canonical[i] = it
		if k, ok := grammar.NormalizeKeyword(it.Keyword); ok {
			canonical[i].Keyword = k
		}
	}
	request := struct {
		PrincipalID string      `json:"principal_id"`
		SessionID   string      `json:"session_id"`
		Items       []BatchItem `json:"items"`
	}{scope.PrincipalID, sessionID, canonical}
	var out BatchResult
	err := command.Run(ctx, tx, scope, batchReceiptOperation, key, request, &out, func() error {
		if err := waitAdvisory(ctx, tx, "claims-offers|"+scope.TenantID+"|"+scope.StoreID+"|"+sessionID); err != nil {
			return err
		}
		if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
			return err
		}
		if err := requireSession(ctx, tx, scope, sessionID); err != nil {
			return err
		}
		result, err := applyBatch(ctx, tx, scope, sessionID, in.Items)
		if err != nil {
			return err
		}
		out = result
		return nil
	})
	if err != nil {
		return BatchResult{}, mapError(err)
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return BatchResult{}, err
	}
	return out, nil
}

// applyBatch locks the touched offers, plans and, when the plan is clean, writes. The caller holds the session's offers advisory lock.
func applyBatch(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID string, items []BatchItem) (BatchResult, error) {
	ids := make([]string, len(items))
	var renameIDs []string
	for i, it := range items {
		ids[i] = it.OfferID
		if it.Action == BatchRename {
			renameIDs = append(renameIDs, it.OfferID)
		}
	}
	// live.offers (claims package table): lock the touched rows in id order (deadlock-free against other batches; ingest holds FOR SHARE).
	rows, err := tx.Query(ctx, `SELECT id::text FROM live.offers
		WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND id=ANY($4::uuid[]) ORDER BY id FOR NO KEY UPDATE`,
		scope.TenantID, scope.StoreID, sessionID, ids)
	if err != nil {
		return BatchResult{}, mapError(err)
	}
	for rows.Next() { // draining the result is what takes every row lock
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return BatchResult{}, mapError(err)
	}
	offers, err := readOffers(ctx, tx, scope, sessionID, "")
	if err != nil {
		return BatchResult{}, err
	}
	byID := make(map[string]Offer, len(offers))
	for _, o := range offers {
		byID[o.ID] = o
	}
	claimed, sellable := map[string]bool{}, map[string]bool{}
	if len(renameIDs) > 0 {
		// claims.lines: a rename would strand every buyer line of the old offer, so an offer with any line is refused.
		lines, err := tx.Query(ctx, `SELECT DISTINCT offer_id::text FROM claims.lines
			WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND offer_id=ANY($4::uuid[])`, scope.TenantID, scope.StoreID, sessionID, renameIDs)
		if err != nil {
			return BatchResult{}, mapError(err)
		}
		defer lines.Close()
		for lines.Next() {
			var id string
			if err := lines.Scan(&id); err != nil {
				return BatchResult{}, mapError(err)
			}
			claimed[id] = true
		}
		if err := lines.Err(); err != nil {
			return BatchResult{}, mapError(err)
		}
		lines.Close()
		var candidates []importCandidate
		for _, id := range renameIDs {
			if o, ok := byID[id]; ok {
				candidates = append(candidates, importCandidate{SKUID: o.SKUID})
			}
		}
		if sellable, err = sellableSKUs(ctx, tx, scope, candidates); err != nil {
			return BatchResult{}, err
		}
	}
	plan := planBatch(items, offers, claimed, sellable, maxOffersPerSession-len(offers))
	out := BatchResult{Offers: []Offer{}, Unchanged: plan.Unchanged, Conflicts: plan.Conflicts, Warnings: plan.Warnings}
	if len(plan.Conflicts) > 0 {
		out.Unchanged, out.Warnings = []string{}, []KeywordFinding{}
		return out, nil
	}
	written := map[string]bool{}
	deactivated, renamed := 0, 0
	for _, it := range items {
		o := byID[it.OfferID]
		if !o.Active {
			continue // deactivate no-op (already inactive); a rename of an inactive offer is a conflict, never reaches here
		}
		// live.offers: the same UPDATE shape as UpdateOffer's deactivation, guarded by the version the plan just verified under the row lock.
		tag, err := tx.Exec(ctx, `UPDATE live.offers SET active=false,version=version+1,updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND id=$4 AND version=$5`,
			scope.TenantID, scope.StoreID, sessionID, o.ID, o.Version)
		if err != nil {
			return BatchResult{}, mapError(err)
		}
		if tag.RowsAffected() != 1 {
			return BatchResult{}, command.ErrConflict
		}
		written[o.ID] = true
		deactivated++
		if it.Action != BatchRename {
			continue
		}
		keyword, _ := grammar.NormalizeKeyword(it.Keyword) // the plan already proved it valid
		var newID string
		// live.offers: same insert as CreateOffer; the old offer was just deactivated, so the one-active-offer-per-SKU index is free.
		if err := tx.QueryRow(ctx, `INSERT INTO live.offers(tenant_id,store_id,session_id,keyword,sku_id,max_quantity_per_claim,principal_id,live_price_minor)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id::text`,
			scope.TenantID, scope.StoreID, sessionID, keyword, o.SKUID, o.MaxQuantityPerClaim, scope.PrincipalID, o.LivePriceMinor).Scan(&newID); err != nil {
			return BatchResult{}, mapError(err)
		}
		written[newID] = true
		renamed++
	}
	all, err := readOffers(ctx, tx, scope, sessionID, "")
	if err != nil {
		return BatchResult{}, err
	}
	for _, o := range all {
		if written[o.ID] {
			out.Offers = append(out.Offers, o)
		}
	}
	out.Applied = true
	if err := command.AuditDetails(ctx, tx, scope, batchAuditAction,
		map[string]any{"deactivated": deactivated, "renamed": renamed, "unchanged": len(plan.Unchanged)}); err != nil {
		return BatchResult{}, err
	}
	return out, nil
}

// validBatch checks the request shape (everything that is malformed rather than a conflict): 1..50 items, canonical distinct
// offer ids, a positive expected version, a known action, and a keyword exactly when the action is a rename. An unfoldable
// rename keyword is NOT a shape error: it reaches planBatch as invalid_keyword data.
func validBatch(in BatchInput) error {
	if len(in.Items) < 1 || len(in.Items) > maxBatchItems {
		return command.ErrInvalid
	}
	seen := map[string]bool{}
	for _, it := range in.Items {
		if !command.ValidID(it.OfferID) || seen[it.OfferID] || it.ExpectedVersion < 1 || it.ExpectedVersion == math.MaxInt64 ||
			(it.Action != BatchDeactivate && it.Action != BatchRename) ||
			(it.Action == BatchRename) == (it.Keyword == "") {
			return command.ErrInvalid
		}
		seen[it.OfferID] = true
	}
	return nil
}

// planBatch decides every item against the session's offers (all of them, any active state) without writing. claimed holds
// the offer ids that have claim lines, sellable the SKUs that sell now, capacity the free offer slots (200 minus offers). Per
// item the first failing rule wins: offer_not_found, version_conflict, then for a rename offer_inactive, has_claims,
// same_keyword, the keyword errors (invalid_keyword, keyword_taken, normalization_collision, duplicate_in_request: the old
// keyword of a renamed offer stays reserved), sku_unavailable, session_full. A deactivate of an inactive offer is Unchanged, not
// a conflict. Non-blocking findings of clean rename targets are Warnings. Pure.
func planBatch(items []BatchItem, offers []Offer, claimed, sellable map[string]bool, capacity int) batchPlan {
	plan := batchPlan{Conflicts: []BatchConflict{}, Warnings: []KeywordFinding{}, Unchanged: []string{}}
	byID := make(map[string]Offer, len(offers))
	for _, o := range offers {
		byID[o.ID] = o
	}
	var proposals []string
	slot := map[int]int{} // item index -> proposal index
	for i, it := range items {
		if it.Action == BatchRename {
			slot[i] = len(proposals)
			proposals = append(proposals, it.Keyword)
		}
	}
	checked, _ := analyzeKeywords(offers, proposals, MatchExact)
	free := capacity
	for i, it := range items {
		o, found := byID[it.OfferID]
		reason := ""
		switch {
		case !found:
			reason = batchReasonNotFound
		case o.Version != it.ExpectedVersion:
			reason = batchReasonVersion
		case it.Action == BatchDeactivate:
			if !o.Active {
				plan.Unchanged = append(plan.Unchanged, o.ID)
			}
		default: // rename
			c := checked[slot[i]]
			switch {
			case !o.Active:
				reason = batchReasonInactive
			case claimed[o.ID]:
				reason = batchReasonHasClaims
			case c.Valid && c.Canonical == o.Keyword:
				reason = batchReasonSameKeyword
			case firstError(c.Conflicts) != "":
				reason = firstError(c.Conflicts)
			case !sellable[o.SKUID]:
				reason = ConflictSKUUnavailable
			case free < 1:
				reason = ConflictSessionFull
			default:
				free--
				plan.Warnings = append(plan.Warnings, c.Conflicts...)
			}
		}
		if reason != "" {
			plan.Conflicts = append(plan.Conflicts, BatchConflict{Index: i, OfferID: it.OfferID, Keyword: it.Keyword, Reason: reason})
		}
	}
	return plan
}

// firstError returns the kind of the first error-severity finding, or "".
func firstError(fs []KeywordFinding) string {
	for _, f := range fs {
		if f.Severity == SeverityError {
			return f.Kind
		}
	}
	return ""
}
