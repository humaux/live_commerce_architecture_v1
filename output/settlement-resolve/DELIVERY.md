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
3. `operator_notes` lists every `assigned_to_store` resolution up to the period end, so older notes are reprinted on later closes (deliberate, noisy).
4. A negative-amount unmapped row (dispute-shaped) can be resolved `not_store_revenue`, i.e. the platform absorbs it: confirm that is the intended meaning.
5. `note` is free text (<= 500 chars) shown in CLI JSON and close notes; nothing stops an operator pasting PII. `environment` on the resolution row is
   checked against the unattributed row by the definer, not by a composite FK.
6. A wrong resolution cannot be corrected by this tool (append-only); correction = owner escalation.

## NOT_RUN
SANDBOX and LIVE: `settlement-resolve` against a real SANDBOX ledger after a real `settlement-sync` (needs the owner's test key); LIVE first-close runbook step
(contract §8). Full foundation suite and browser/visual gates (CI). The two UI node gates of `check-gates.sh` (missing `node_modules`).

## Integrator to-do
Migration number 0163 was given. W3-U4's 0164 lands later: the R2 pin becomes 90 then (comment lines keep migration order). `slsApplyWithout` holds 0163
back by name with 0150 (not "the newest"). No shared schema, OpenAPI or lockfile touched.
