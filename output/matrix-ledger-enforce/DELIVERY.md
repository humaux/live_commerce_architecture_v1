# matrix-ledger-enforce delivery

- Branch: `unit/matrix-ledger-enforce`; worktree `.worktrees/matrix-ledger-enforce`. Base `a93216434bfa1cf30e849efe320d2883d1ba6256` after merging current `origin/r3/integration`; final fetch/merge exit0/no-op. Tested source **b390ee7a43e5e2c42b6607ea76d1b95bb4ddcfd9**; final commit is evidence only.
- Author: Codex-4 (exact parent model/effort not exposed); read-only source/contract/evidence explorer GPT-6.1-sol/medium. Task `f80cfebd-aa80-4e3b-b27c-bc5933aa98cc`; single writer, no recursive delegation. Brief: `output/integrator/triage/brief-matrix-ledger-enforce.md`.
- Scope: three test sources and independent synthetic fixture; no product Go/API/SQL/migration/dependency change. 0168 remains untouched.

`catalog_matrix_ledger_test.go` declares one 47-template identity table, expanded with the three frozen locale copy bindings. The gate derives its required cardinality from the set and validates the full page/locale/width/top-level-row/control/action identity, exact `actual=PASS`, no duplicates/unexpected identities, and no missing identities. Scope selection precedes locale/width/status inspection, so malformed target rows cannot be filtered away. Unrelated legacy form/list rows may coexist. The fixture projects identity/status fields from actual prior PR16 evidence; it is independent of the predicate.

`product-editor.acceptance.ts` snapshots the in-memory ledger once and writes those exact bytes to the legacy output and this run's `product-editor-click-ledger.json`. `browser_catalog_core_test.go` reads only the current run file, fails on unavailable/malformed evidence and executes the strict guard after Playwright in productEditor mode. All original PG assertions and frozen CC12 remain unchanged.

## Commands and evidence

Exact commands/exit codes/source/input/log hashes: `RESULTS.json`, `local-gates.json`, `calibration-results.json`, `source-hashes.json` and `review-log-hashes.json`. Raw logs and screenshots are uncompressed in `/Volumes/data/live_commerce_architecture_v1/output/matrix-ledger-enforce/`.

| Command | Exit | Evidence |
| --- | ---: | --- |
| `pnpm install --offline --frozen-lockfile --ignore-scripts` | 0 | `install.log`; lockfile unchanged |
| `GOTOOLCHAIN=go1.27.2 go test -run '^TestProductEditorMatrixLedger$' -count=1 -v ./tests/foundation` before policy | 1 | `red.log`: 16 corruption controls fail against a temporary no-check TDD seam representing the previous absent Go policy; **not** a claim of an unchanged old-browser run |
| Same focused Go command after policy | 0 | `green.log`: valid recorded data, reordered set, unrelated rows, and all corruption controls pass |
| Same Go regex with `-tags browser` | 0 | `go-browser.log` |
| `GOTOOLCHAIN=go1.27.2 go vet -tags browser ./...` | 0 | `vet.log` |
| `bash scripts/dev/test-node.sh` | 0 | **1185 PASS /0 FAIL**, `node.log` |
| `pnpm --filter @live-commerce/admin typecheck` | 0 | `typecheck.log` |
| `bash scripts/dev/test-local.sh --list` | 0 | `registry-list.log`, original registry layout retained |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates.log`, 82 documented modes/headers/suite assignments |
| `LC_TEST_LOCK_WAIT=300 LC_BROWSER_ENGINE=chromium bash scripts/dev/test-local.sh --browser-product-editor` | 0 | **Run exactly once**. Go logs 141 declared unique PASS identities from PE's current run; original mobile/archive/axis-remove PG facts and mandatory frozen CC12 PASS, `browser-product-editor.log` |

Actual browser ledger has 196 total rows, with **141 target identities**, 47 per zh-TW/zh-CN/en. Current PE evidence: `output/playwright/catalog-core/20261009T132934.505765000`; the distinct later CC12 evidence directory is not used as the ledger source. Canonical full PE artifacts and hashes: primary `output/matrix-ledger-enforce/browser/` and `browser-evidence.json`.

**Actual-artifact mutation calibration:** the standard Go build overlay adds only a virtual probe test that invokes the real predicate; no tracked source, product or predicate is replaced. On copies of this real run file: delete one target row → exit1 (`missing 1 required rows`); mark one target FAIL → exit1 (`not PASS`); restore exact original bytes → exit0 (141 required unique identities). Full repeatable commands/env, overlay source/config and input hashes: `calibration-results.json`, `calibration-probe.txt`, `calibration-overlay.json`; three plaintext logs. No additional browser run was needed.

**E3: automated Go gate/corruption calibration + real browser/Next/Go/isolated REAL_PG**, with MOCK identity/product fixtures; no provider/production acceptance. Read-only independent source review `9599c3b8` and evidence audit `a0317635` found no P0/P1; no independent runtime rerun claimed. All owned processes/fixtures closed and build lock released. Generated legacy UI output is archived with hash verification under primary `generated-legacy/`, then restored to its prior tracked state; it is not staged in this unit.

## CI gates / NOT_RUN

CLI planner selects **53** modes (`pr-modes.json`). The requested product-editor mode ran locally; other52 modes/full required GitHub set, new-head integrator review and platform/production gates are NOT_RUN locally. No unresolved scoped blocker. Integrator reviews and pushes; author commits and stops, never pushes. Source hashes remain equal on the evidence-only handoff. Packaged check-gates and staged diff-check both exit0.
