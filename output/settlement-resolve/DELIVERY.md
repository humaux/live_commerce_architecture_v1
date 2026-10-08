# settlement-resolve delivery (S2-OPEN-1, migration 0163)
- Branch: unit/settlement-resolve. Code under test: `2f8c778e` (the green log's SHA is in `green-final-pg.sha`); later commits on top touch only docs/evidence.
- Base: origin/r3/integration `ba1d1579` (0161, 0162 and the platform-admin allowlist merged; R2 pin = 89).
- Roles / models: Aliyun Qwen = implementer (killed at its time limit, output treated as E0); Claude Sonnet 5.5 = finisher and verifier (merge, review, fixes, tests, mutation proofs). The author of the fixes is not an independent reviewer: the Opus money review is still owed (E4 not reached).
- Evidence class: REAL_PG + MOCK. No Stripe call, no LIVE anything. Evidence level: E3 for the tested paths (automated, bound to the SHA above).

## Summary
`payments.settlement_unattributed_resolutions` (append-only, FORCE RLS, SELECT/INSERT to the registry writer only) +
`payments.record_settlement_resolution(...)` (SECURITY DEFINER, `search_path=pg_catalog`, EXECUTE registrar only) + in-place patches
(`pg_get_functiondef` + replace + exactly-once anchor assert) of `payments.close_settlement` (resolved rows no longer refuse close;
`operator_notes` returned) and `payments.record_settlement_lines` (a resolved row is never attributed later). CLI `stripe-admin
settlement-resolve`, allowlisted in `deploy/scripts/ops-admin.sh`; contract `stripe-platform-account-v1.md` §6.6 + §8 LIVE line + §9 limits.
v1 moves no money: `assigned_to_store` is a recorded note printed by close.

## What Qwen did (and was kept)
6a9a88cb contract §6.6 · 937667e4 RED probe test · 98eae821 migration 0163 · b93e1a70 `SettlementResolve`, `CloseResult.OperatorNotes`,
CLI, `stripeRegistrarFunctions` entry · 93961572 WIP test edits (PF11 now resolves instead of an owner-pool DELETE; privilege pins;
390-line `settlement_resolve_test.go`). Reviewed against the brief: table, definer (validation order, replay before the closed-period check,
advisory lock shared with sync/close, enrollment-or-platform-store target rule), the three-anchor close patch, CLI and allowlist all match the
brief and the house patterns.

## What was wrong or missing, and what I changed
1. **R2 pin stale after the trunk merge** (88 -> 89: 0161, 0162, 0163) and conflicts in `ops-admin.sh` (usage + case arm kept from both sides)
   and `r2_integration_upgrade_test.go` (comment lines kept in migration order).
2. **No append-only trigger** although 0078/0143 have the house pattern: added `payments.guard_settlement_resolution()` as BEFORE
   UPDATE OR DELETE and BEFORE TRUNCATE (PT409 `settlement_resolution_immutable`), owner registry writer, PUBLIC revoked, commented.
3. **A whitespace-only note was accepted** (CHECK counted characters only): table CHECK, definer and Go now require a non-blank note.
4. **Resolved rows could still be attributed by a later sync** (beyond the brief, flagged for the money reviewer): 0150 retries an unmapped
   row on every identical re-sync; after a resolution a late session would insert a line, so an `assigned_to_store` row is paid twice
   (note + statement) and a `not_store_revenue` row leaks into a store's totals. One anchor-asserted in-place patch of
   `record_settlement_lines` counts a resolved row as a duplicate. Red proof below (`Inserted:2` without the patch).
5. Three Qwen test files were not gofmt-clean (`check-gates` gofmt step would fail); formatted.
6. `deploy/README.md` still said "LIVE requires an append-only resolution procedure"; now documents the resolve step.
   `tests/deploy/deploy-prep-r3.test.mjs` now exercises `settlement-resolve` through the allowlist.
7. New tests: `PF15_append_only_in_depth`, `PF15_resolved_row_is_final` (with an unresolved control row that still attributes), blank and
   oversized note at SQL (22023) and Go layers. Contract §6.6 amended for 2-4.

## Tests (run from the worktree; zsh, so exit codes are printed with `$?`)
| Command | Exit |
| --- | --- |
| `go build ./...` | 0 |
| `go vet ./...` and `go vet -tags browser ./tests/foundation` | 0 / 0 |
| `go vet ./internal/... ./cmd/...` | 0 |
| `go test ./internal/payments/stripeadmin/... ./cmd/stripe-admin/... -count=1` (unfiltered; the brief's `./internal/stripeadmin` is `internal/payments/stripeadmin`) | 0 (`unit-go.log`) |
| `go test ./internal/platform/... -count=1` | 0 |
| `node --test tests/deploy/deploy-prep-r3.test.mjs tests/deploy/meta-connect-preflight.test.mjs tests/deploy/ops-alert-preflight.test.mjs` | 0, 46/46 |
| `bash scripts/dev/test-focused.sh '^(TestPlatformSettlement\|TestR2IntegrationUpgradeFromReleaseHead\|TestStripeSL02Schema\|TestRemovePayuniNotify\|TestStripeAuthority\|TestStripeRF03Schema\|TestStripeRF12Guards\|TestPlatformStripePF01Schema\|TestPlatformOperatorOP04b\|TestPlatformOperatorOP08\|TestPlatformOperatorOP09)'` at `2f8c778e` | 0, 20 top-level PASS, 0 FAIL (`green-final-pg.summary.log`) |
| `bash scripts/dev/check-gates.sh` | **1, environmental**: `tests/admin/shell-architecture.test.mjs` / `ui-architecture-gate.mjs` cannot load `typescript-api` (no `node_modules` in this worktree); unrelated to this unit |
| `check-gates.sh` with exactly those two UI node lines removed (shard-plan, registry, headers, gofmt, vet incl. `-tags browser`) | 0 |
| `bash scripts/dev/check-headers.sh` | 0 |

Names in the brief that match no Go test: `TestStripeAdmin*`, `TestOpsAdmin*`, `TestSettlementResolve*`. The unit's test is
`TestPlatformSettlementPF15Resolve`; the CLI tests are `cmd/stripe-admin` unit tests (ran unfiltered).

## Red -> green
- RED at the original RED commit `937667e4` (temporary detached worktree, removed after): `test-focused.sh '^TestPlatformSettlementPF15Resolve$'` -> exit 1,
  `function payments.record_settlement_resolution(...) does not exist (42883)` (`red-at-937667e4.log`; Qwen's own `red.log` is the same).
- The final 8-subtest test against the final tree: exit 0 (above).
- Mutation proofs against the FINAL test (migration restored byte-identical after each, logs `mutation-*.log`, each exit 1):
  - 0163 removed: registrar cannot open (`stripeadmin: database`);
  - close patch without the resolution clause: `PF15_close_with_notes`, `PF15_cli_e2e`, `PF15_closed_period` fail with `settlement_unattributed`;
  - triggers + sync guard removed: `PF15_append_only_in_depth` (UPDATE accepted) and `PF15_resolved_row_is_final` (`Inserted:2`, both rows attributed) fail.

## CI gates still to run (heavy, GitHub only per owner rule)
`foundation-shards` (the full foundation suite; every migration-walking upgrade test beyond the ones above), `deploy-smoke`. `shard-plan` reports
`TestPlatformSettlementPF15Resolve` (and trunk's `TestMerchantCancelClosesWorkItem`) in the catch-all group g06; `node scripts/dev/shard-plan.mjs --write` is the
integrator's call. The `node --test` UI architecture gate needs a `pnpm install` checkout.

## Risks / questions for the money reviewer
1. Item 4 above goes beyond the brief (a second money function is patched). It only narrows behaviour for rows that have a resolution.
2. A late `unmapped_source` row inside an already-closed week cannot be resolved (brief-mandated `period_already_closed`), and 0150's close check has no lower
   bound on `txn_created_at`, so it blocks every later close of the environment until an owner red-line SQL fix. Contract §9 states it. Should a closed-week row be
   resolvable as `not_store_revenue` (it cannot change a frozen statement)?
3. ~~`operator_notes` reprints older notes on later closes~~ fixed in round 2 (P1-2): a close prints only its own period's notes.
4. A negative-amount unmapped row (dispute-shaped) can be resolved `not_store_revenue`, i.e. the platform absorbs it: confirm that is the intended meaning.
5. `note` is free text (<= 500 chars) shown in CLI JSON and close notes; nothing stops an operator pasting PII. `environment` on the resolution row is
   checked against the unattributed row by the definer, not by a composite FK.
6. A wrong resolution cannot be corrected by this tool (append-only); correction = owner escalation.

## NOT_RUN
SANDBOX and LIVE: `settlement-resolve` against a real SANDBOX ledger after a real `settlement-sync` (needs the owner's test key); LIVE first-close runbook step
(contract §8). Full foundation suite and browser/visual gates (CI). The two UI node gates of `check-gates.sh` (missing `node_modules`).

## Integrator to-do
Migration number 0163 was given. W3-U4 now follows merged 0165 as 0166: the combined R2 pin becomes 91 (historical command results below keep their original counts). `slsApplyWithout` holds 0163
back by name with 0150 (not "the newest"). No shared schema, OpenAPI or lockfile touched.

---

# Round 2 (independent Opus money review: MERGE-AFTER-FIXES, two P1s confirmed by a real-PG probe)
- Round-2 commits on top of `003eabbf`: the migration/Go fix, the tests, the contract/README, and `fix(settlement-resolve): 0163 COMMENT literal had an
  unescaped apostrophe` (my own slip, caught by the full focused run; the broken intermediate is `r2-pg-broken-comment.log`). Code under test: the SHA in
  `r2-green-pg.sha`. Roles/models unchanged (Claude Sonnet 5.5 authored the fixes; the reviewer's evidence files were not touched).

| Finding | Change | Red evidence (before the fix) |
| --- | --- | --- |
| P1-1 resolving a row that already has a line pays twice | `record_settlement_resolution` raises PT409 `already_attributed` right after the reason check if any `settlement_lines` row has the txn (either resolution) | `r2-red.log`, test line 436: `assigned_to_store on a row that has a line: want ... "already_attributed", got <nil>` |
| P1-2 notes reprinted on every later close | notes only for `txn_created_at >= v_start AND < v_end`, plus `period_start`, `type`, signed `settle_amount`/`settle_net`/`settle_currency` (`OperatorNote` carries them) | `r2-red-p1-2.log` (P1-1/P2 already fixed, only P1-2 red): week-1 note fields empty; the week-2 CLI close printed 1 note, the week-3 close printed 2 (Odd reprinted + Fin) |
| P2-1 operators cannot find blocking ids | 4th anchored in-place patch: the `settlement_unattributed` refusal carries up to 20 ids (oldest first) as DETAIL; `settlementScan` passes on only a DETAIL that is exactly 1..20 `txn_` ids (never a driver message); the CLI prints them: `stripeadmin: rejected: settlement_unattributed: txn_a,txn_b` | `r2-red.log`, test line 129 (refusal had no ids); Go unit test `TestSettlementUnattributedRefusalCarriesOnlyTheBlockingIDs` (exact-match, 0/1/2/20/21 ids, free text, driver tail, other token) |
| P2-2 note charset | table CHECK and definer refuse `[[:cntrl:]]`; Go refuses `unicode.IsControl` (the review note said Go already caught tab/newline; it did not, so both layers now do) | `r2-red.log`, test line 197 (Go accepted a newline note); mutation without the two SQL clauses: `r2-mutation-nocntrl.log` (`SQL accepted ...'two'\|\|chr(10)\|\|'lines'`, exit 1) |
| P2-3 / rulings | contract §6.1 CHECK, §6.2, close/resolve rows, §6.6 (signed amount, negative row = platform absorbs unless `assigned_to_store`, `already_attributed`, resolved rows skipped), §8 runbook (resolve every row the refusal names that has no line; no buyer PII in the note), §9 (erasing a note is red-line; follow-up amendment for closed-week rows); `deploy/README.md` mirrors §8 | n/a (docs) |

Rulings applied as given: (a) sync guard kept; (b) closed-week refusal kept, follow-up amendment listed in §9; (c) negative-row sentence added.

## Round 2 commands (zsh; exit codes printed with `$?`)
| Command | Exit |
| --- | --- |
| `go build ./...` / `go vet ./...` / `go vet -tags browser ./tests/foundation` / `go vet ./internal/... ./cmd/...` | 0 / 0 / 0 / 0 |
| `go test ./internal/payments/stripeadmin/... ./cmd/stripe-admin/... -count=1` (unfiltered) | 0 |
| `go test ./internal/platform/... -count=1` | 0 |
| `node --test tests/deploy/deploy-prep-r3.test.mjs tests/deploy/meta-connect-preflight.test.mjs tests/deploy/ops-alert-preflight.test.mjs` | 0, 46/46 |
| `bash scripts/dev/check-headers.sh` | 0 |
| `check-gates.sh` minus its two UI node lines (still unrunnable here: no `node_modules`) | 0 (shard-plan, registry, headers, gofmt, vet incl. `-tags browser`) |
| `bash scripts/dev/test-focused.sh '^(TestPlatformSettlement\|TestR2IntegrationUpgradeFromReleaseHead\|TestStripeSL02Schema\|TestRemovePayuniNotify\|TestStripeAuthority\|TestStripeRF03Schema\|TestStripeRF12Guards\|TestPlatformStripePF01Schema\|TestPlatformOperatorOP04b\|TestPlatformOperatorOP08\|TestPlatformOperatorOP09)'` at the SHA in `r2-green-pg.sha` | 0, 20 top-level PASS, 0 FAIL (`r2-green-pg.summary.log`) |

## Round 2 risks and open items
1. Residual note gap (needs the reviewer's call): `period_already_closed` only looks at the row's own week. A LATE row dated in an earlier week that has no
   statement of its own, synced after a later week closed, is resolvable, but its `assigned_to_store` note is printed by no later close (a close prints only its
   own period). The resolve command's own JSON line and the audit row still record it. Tightening the refusal to "any statement with `period_start >=` the
   row's Monday" closes the gap but makes such rows owner-SQL only (the same trade-off as ruling (b)).
2. A close that refuses lists at most 20 ids; the operator resolves them and reruns the close for more.
3. DETAIL is built by the same predicate as the check (duplicated inside the patch); a future edit of one must edit the other (the anchor assertions fail loudly on any shape change).

## Round 2 NOT_RUN
SANDBOX/LIVE (unchanged); the full foundation suite and the browser gates (CI `foundation-shards`, `deploy-smoke`); the two UI node lines of `check-gates.sh`.

---

# Round 3 (Opus delta re-review: every round-2 fix verified; open item P1 ruled)
- Finding: a late `assigned_to_store` row dated before any closed statement was resolvable but no close prints its note (a close prints only its own period), so the payout instruction was lost.
- Fix (`migrations/0163`, `period_already_closed`): `AND (s.period_start=v_monday OR (p_resolution='assigned_to_store' AND s.period_start>v_monday))`. `not_store_revenue` (no payout) stays resolvable so close never stalls; the assigned case escalates to the owner. Contract §6.6/§9 and the closed-week follow-up note updated.
- P3 runbook line added to contract §8 and `deploy/README.md`: replayed or per-store closes reprint a period's notes by design; settle each `balance_txn_id` exactly once.
- Red first: `r3-red.log` (PF15_closed_period, test line 481: assigning the late week-0 row `txn_PF15Early` after weeks 1-3 closed returned nil instead of `period_already_closed`). Green: same test now also proves `not_store_revenue` on that row succeeds and the refused attempt wrote no row.
- Commands (zsh, `$?`): `go build ./...` 0; `go vet ./...` 0; `go vet -tags browser ./tests/foundation` 0; `go vet ./internal/... ./cmd/...` 0; unfiltered `stripeadmin` + `stripe-admin` + `internal/platform` unit tests 0; deploy node tests 0 (46/46); `check-headers.sh` 0; focused 20-test PG set 0, 20 PASS / 0 FAIL (`r3-green-pg.summary.log`, SHA in `r3-green-pg.sha`).
- Residual closed: the round-2 open item 1 above is now fail-closed. NOT_RUN unchanged (SANDBOX/LIVE, full foundation suite and browser gates in CI, the two UI node lines of `check-gates.sh`).
