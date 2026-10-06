# Unit W3-06B: keyword tools and match simulator (backend)

Status: DELIVERED, independent review MERGE (Opus, output/w3-06b-keyword-tools/REVIEW-opus.md); review P2 follow-up applied 2026-10-07 (Claude Sonnet backend implementer). Base `r3/integration` `5862caf8`.
Worktree `.worktrees/w3-06b-keyword-tools` (branch `unit/w3-06b-keyword-tools`). Source: `output/arch-conformance/IMPLEMENTATION-PLAN.md`
W3-06B; M04 #2, #3, #5, #7, #14; FR-RULE-01. UI is a separate Codex unit (W3-U2 simulator part); this unit has no `apps/` change.
Migration: none needed (reserved 0158 stays unused; see DELIVERY).

## Owner decision 2026-10-07 (binding) and integrator ruling
Store default match mode = **EXACT**. That is the existing frozen default (a window without a row reads EXACT). No store-level default
setting is added. Integrator ruling (same day): an omitted `match_mode` on the **simulator** means the session's current window mode
(EXACT when there is no window), so the merchant sees what would happen live; `window_match_mode` and `window_state` stay in the response.
The keyword check and the batch rename warnings use the same default (window mode, EXACT without a window).

## Scope (smallest useful set; reuse, do not duplicate)
1. **Simulator** `POST .../live-sessions/{session_id}/claims/simulate` `{comment, match_mode?}`: pure read (`live:read`), no row written, no Meta call.
   It runs the same pure matcher the real ingest runs (`grammar.ParseForIngest` then `effective` then `offerReason`) against the session's offers.
2. **Conflict check** `POST .../claims/keywords/check` `{keywords: [...], match_mode?}`: per proposed keyword errors (invalid, taken,
   duplicate, normalisation collision) and warnings (quantity look-alike, digits-only keyword under CONTAINS), plus the same findings among the
   session's existing offers.
3. **Auto-numbering** `GET .../claims/keywords/next?prefix=&count=`: next free short keywords (`A1`, `A2`, ...). Offer creation keeps
   requiring `keyword` (frozen M3 shape), so numbering is a suggestion endpoint.
4. **Batch** `POST .../claims/offers/batch` `{items: [{offer_id, expected_version, action: deactivate|rename, keyword?}]}`: one receipt, one
   audit row, all-or-nothing, conflicts are data (HTTP 200, `applied=false`, nothing written). Keywords stay immutable (migration 0060: no UPDATE
   grant), so a rename is "retire the old offer, create the new one with the same SKU, cap and live price" in one transaction.
5. **Validity time / quantity cap**: the per-claim cap exists (`max_quantity_per_claim`, `QUANTITY_OVER_MAX`). A keyword validity window needs
   new offer columns plus changes to the Meta intake definers: not small, deferred (see DELIVERY).

## Gates
`test-focused.sh 'Simulate|KeywordTools'`; simulator parity against real ingest (`RecordManualClaim`) on the kwc vector corpus; DB-free router test.
