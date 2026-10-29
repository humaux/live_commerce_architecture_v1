## Problem and resulting behavior
Equal settlement totals can hide missing/replaced IDs, currency changes or offsetting item-net changes (#196). A failed final chunk can leave older clean sync coverage usable (#197).

Migration **0208** stores `items_complete` beside each immutable per-currency comparison. SQL matches the full manifest against effective ledger rows by transaction ID, currency and stored net, retaining the lines/unattributed anti-join. Close selects the latest observation as before, then refuses `sync_unreconciled` for any amount difference or incomplete items, before statement/assignment/audit writes. Existing aggregate-only 0200 comparisons require resync; pre-0200 grandfather behavior remains unchanged.

The Registrar sends the same full window and manifest on **every** 500-row chunk. A committed prefix therefore supersedes old clean coverage and blocks close while remaining items are missing. Complete retry and duplicate replay can restore coverage. Signatures, grants, environment locks, returns and accounting stay intact; no money moves.

Closes #196
Closes #197

## Evidence and validation
- Tests-only head: `18bd6a6629815af7844847e8a96ecc6e31c4e863` (initial tests commit `2c521b2a9`, generated-ownership prerequisite repair only thereafter).
- Causal RED: Gates **40759956329**, unit job **123075243384**, synthetic merge `be7e4ed536f81f706e0dc3d24f2c6c24506f32f6`: new `TestSettlementSyncChunksAndCarriesTheWindowAndManifestOnEveryChunk` fails its chunk-0 full-manifest assertion. Receipt: `output/settle196-197/red.log` and `relay2-unit-red.log`.
- RED-head Foundation **40759956153** / **123068875365** passed; this is historical evidence, not implementation GREEN.
- PF17 database RED counterexamples were still running in **40759956329 / 123075243146** at implementation preparation; not claimed executed/passed.
- Local permitted static checks only: gofmt, `git diff --check`, `python3 scripts/check_packet.py` (all exit 0; packet structure only, E1). Browser ownership regenerated via its canonical static generator. No local tests/servers/PG/containers/browser.
- Implementation GREEN, REAL_PG acceptance and independent review are **PENDING**. Author assessment E0.

## Assigned migration order and integration risk
Merged `origin/r3/integration` at **313c2c36f** before implementation (0205 / #254 included). Merge this PR only after **#262 / 0206** and **#265 / 0207**. Per explicit card instruction, the R2 pin is the expected final **138** although this base plus 0208 currently contains **136** R2 files. The exact-count gate will remain blocked until those predecessors arrive; integrator re-pins at merge time. No migration number was self-assigned, and 0200 is unchanged.

## Paths and scope
- Card implementation: `internal/payments/stripeadmin/settlement.go`, `migrations/0208_settlement_item_completeness.sql`, assigned `tests/foundation/r2_integration_upgrade_test.go`; counterexamples in `settlement_unit_test.go` and `settlement_returns_test.go`.
- Additional same-module test path: **`tests/foundation/stripe_live_schema_test.go`**. Its pre-Stripe fixture must hold back dependent 0208 with 0200, and its exact Apply count is updated 9 → 10. Existing ACL and behavior assertions remain intact. Listed for integrator review/approval.
- Generated path: **`output/ci-select-backend-browser/browser-ownership.json`**, canonical regeneration required by hashed foundation source changes (includes prior relay-authorized repair).
- Task-local evidence/handoff: **`output/settle196-197/**`**.
- No contract/OpenAPI/lockfile/go.mod, other-app, gate-adapter, fixture weakening or baseline edits. No UI/Figma/browser-route changes.
- Humaux/code-index tools unavailable: memory/canvas/coord updates NOT_RUN. Run completion is relayed by the integrator per task instructions; no local background process remains.

🤖 Generated with Codex (dispatched by Claude Code integrator).
