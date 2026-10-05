# K3 adversarial report — LC-B1 (session lifecycle + ≤5 OPEN windows per store)

- Target: unit LC-B1 merged in `1a6a9177` — `internal/live/lifecycle.go`, `internal/httpapi/live_lifecycle.go`,
  `internal/claims/merchant.go` (cap), `migrations/0122_live_lifecycle.sql`.
- Tests: `tests/foundation/k3_lc_b1_adversarial_test.go` (13 test groups, REAL_PG, MOCK manual ingress,
  `-race -count=1`). Command: `bash scripts/dev/test-focused.sh 'K3LcB1|LiveLifecycle'`.
- Runs: two consecutive full runs, identical outcomes (red.log, green.log): 17 PASS (incl. the 6 author tests),
  2 test groups FAIL — the failing assertions ARE the findings below (safe outcomes asserted by design).

## Cases

| # | Case (contract §) | Result |
|---|---|---|
| 1 | 4 concurrent `start` on ONE draft session, same expected_version, distinct keys (§9 CAS) → exactly 1 wins, losers `version_conflict`; window OPEN generation 1; `lifecycle_version`=2; store OPEN count=1 | PASS |
| 2a | 2 concurrent `end` → one wins, one `version_conflict`; window CLOSED once; v3 | PASS |
| 2b | `archive` raced against `end` from live → `invalid_transition` regardless of order | PASS |
| 2c | `start` vs `archive` from ended, same version → exactly one wins (one CAS increment); follow-up obeys §9 table | PASS |
| 3 | 4 starts in store A1 raced with 4 starts in store A2 → all 8 succeed; cap is per store, advisory lock is store-scoped (§8.5) | PASS |
| 4 | Two concurrent 6th contenders both refused `too_many_open_windows` and fully rolled back (draft v1, no window row, count stays 5); `end` raced against a 6th open never leaves 6 OPEN and a loser is cleanly rolled back | PASS |
| 5 | Idempotency-Key reused with different action / `open_window` / `expected_version` / session → `version_conflict`, no side effect; original key+body replay after further transitions returns the stored result WITHOUT re-executing (ended session stays ended, window stays CLOSED) | PASS |
| 6 | Stale `expected_version` → `version_conflict` with zero side effects (lifecycle, window, audit count unchanged) | PASS |
| 7 | M2 `SetWindow` open on a never-started draft session — safe outcome (refusal) asserted | **FAIL → F-M2-DRAFT** |
| 8 | Offer update / offer create / `offer_timeline` append on an archived session — safe outcome (refusal, §9 read-only) asserted | **FAIL → F-ARCH-*** |
| 9 | `end` raced against 8 in-flight manual claims: every claim ACCEPTED-with-persisted-line or REJECTED/WINDOW_CLOSED; reported outcomes == DB truth; no deadlock/timeout/serialization leak; post-`end` claim is WINDOW_CLOSED and persists nothing (§9 OPEN-16) | PASS |
| 10 | Deterministic claims on ended and archived sessions → REJECTED/WINDOW_CLOSED, nothing persisted | PASS |
| 11 | Tenant isolation: tenant-B principal (live:manage on own store) → `not_found` for tenant-A session lifecycle and `RecordOfferFeatured`; tenant-B token never resolves tenant-A store scope; RLS read-back shows 0 rows of tenant A in `live.sessions` / `live.offer_timeline` / `live.claim_windows`; cross-tenant attempts leave the session untouched | PASS |
| 12 | `start open_window=false` → `end` → `archive`: end succeeds idempotently and does NOT create a window row (version-0 placeholder throughout) | PASS |
| 13 | Regression loop ×6: M2 reopen raced against `end` never leaves an ended session with an OPEN window (reopen is lifecycle-`start` only, §9) | PASS |

## Findings (input → expected / actual)

### F-M2-DRAFT — M2 opens a claim window on a draft session (author-known risk, confirmed)
- **Input:** fresh draft session (never through A7); M2 `SetWindow {state: OPEN, match_mode: exact}`.
- **Expected (safe):** `409 invalid_transition` — §9 routes window opening through lifecycle `start`; §2.2 polls
  **never** for `draft` sessions.
- **Actual:** window OPEN generation 1, `lifecycle` stays `draft`. Demonstrated consequence: the dead window
  (never polled, so no claim can ever arrive through Meta ingest) still occupies one of the 5 §8.5 cap slots —
  only 4 subsequent lifecycle starts fit before `too_many_open_windows`.
- **Mitigation observed:** M2 can close the window again, so the slot is recoverable by the same merchant.
- **Suggested ruling:** `applyWindow` refuses `opened` unless `lifecycle IN ('live')` (A7 `start` sets `live`
  first, so its own path is unaffected), or a contract amendment blessing draft opens with a poll-gate change.

### F-ARCH-OFFER-UPDATE — offers of an archived session stay mutable (author-known risk, confirmed)
- **Input:** session walked draft→live→ended→archived; M4 `UpdateOffer {max_quantity_per_claim: 9, active: false}`.
- **Expected (safe):** refusal — §9 "archive … read-only afterwards".
- **Actual:** success; offer version bumped. `claims.UpdateOffer` has no lifecycle guard
  (`internal/claims/merchant.go:435`).

### F-ARCH-OFFER-CREATE — new offers accepted on an archived session
- **Input:** archived session; M3 `CreateOffer {keyword, sku, max: 1}`.
- **Expected (safe):** refusal (§9 read-only). **Actual:** offer created.
  (`internal/claims/merchant.go` CreateOffer checks only the ≤200 count and SKU sellability.)

### F-ARCH-TIMELINE — `offer_timeline` append accepted on an archived session (contract silent; safe outcome asserted)
- **Input:** archived session; `RecordOfferFeatured(session, offer)`.
- **Expected (safe):** refusal under §9 read-only. **Actual:** row appended (FK still valid; no lifecycle check in
  `internal/live/lifecycle.go:139`). If the integrator rules recommend-events on archived sessions legitimate
  (e.g. late recommend telemetry), this needs a contract sentence, not code.

## Verdict

The LC-B1 core is sound under adversarial pressure: every race case (same-session CAS, cross-store cap, 6th-slot,
end-vs-ingest, reopen-vs-end), the idempotency/CAS semantics, tenant isolation (I01/RLS) and the §9 transition
table behave per contract — 11/13 new groups pass, plus all 6 author tests, twice, under `-race`.

No P0. All 4 findings sit in the two gap areas the author already disclosed (M2 draft opens; archived not guarding
offer writes). F-M2-DRAFT and F-ARCH-OFFER-UPDATE are operationally user-visible (stranded cap slots; mutated
commercial terms after archive) — integrator ruling needed: fix unit or contract amendment before the LC-B2 pollers
and LC-B4 recommend paths build on this surface. The two failing test groups must NOT be "fixed" by weakening the
assertions; flip them only with the ruling.
