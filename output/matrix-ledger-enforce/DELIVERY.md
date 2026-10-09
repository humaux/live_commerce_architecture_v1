# matrix-ledger-enforce PR #28 round 2 delivery

- Branch/worktree: `unit/matrix-ledger-enforce`, `.worktrees/matrix-ledger-enforce`; base/trunk merge `6feb3890a5006a4583f7cc158041694e8104a491`, remote pull/merge already up to date.
- Tested frozen source: **`25bb988fff059217ab731ab922b05ca49ab4e5d7`**. The final evidence commit changes only task artifacts. `round1/SOURCE.json` and `round2/SOURCE.json` bind the final runtime/test source hashes, including spec `eb359bb7` and source gate `deff2b4c`.
- Author Codex-4, parent model/effort not exposed. Isolated ui_worker GPT-6.1-sol/high owns the Node test; source review GPT-6.1-sol/medium is read-only. Task `61406236-763f-47fa-876a-871945976431`; packet `output/integrator/triage/pr28-6feb3890.md`. No push.

## Review fixes

1. **4232425489 / P1:** the source gate resolves the real spec's lexical bindings, loops and field factories against inert locator descriptors. It matches each declared control to the actual test ID, row, role/name, CSS scope or native dialog, then checks method sequence and Enter/Escape key. Unknown/dynamic bindings fail closed. The same shared table defines 77 exact identities per locale (231 total); there is no second case inventory. Existing wrapper/direct-write checks and all browser assertions remain.

   The actual unchanged `6feb3890` checker admits a product-name→product-slug swap of both action and assertion (exit 0). The corrected checker rejects that same swap (exit 1). `round2/locator-card/baseline-reproduction.json` records the exact executable stdin/argv and baseline/mutation hashes; raw logs are uncompressed. Fourteen injected faults are RED, including literal/alias/factory/row/role/scope swaps, native receiver/method, press key and the original four bypass controls. The parent's independent normal run passes 23/23 and literal/key calibrations each exit 1 (`round2/focused-gates.json`).

2. **4232425504 / P2:** the previous round-one run at `a7ba8593` with spec `9d14a9fe` occurred **before the #23 merge**, which changed the runner evidence path. That run is historical and does not certify the current spec. A fresh actual mode run on frozen `25bb988fff059217ab731ab922b05ca49ab4e5d7` verifies spec `eb359bb7`; the before/after source hash check confirms no source changed during execution. `round1/SOURCE.json` is rebound and retains the superseded round-one identity explicitly.

Canonical current evidence is committed in [ledger](browser/product-editor-click-ledger.json), [Playwright log](browser/playwright.log) and [manifest](browser/browser-evidence.json). The manifest binds their SHA-256 values and the tested source. Runner output stays under ignored `output/playwright/pr28-r2-20261009T172203/`, as #23 requires. The raw ledger has 286 total rows, including **231 unique required PASS rows: 77 each for zh-TW, zh-CN and en at 390px**. Screenshot filenames in the raw ledger refer to auxiliary runner output; screenshots are not a delivery claim.

## Commands and exit codes

Full argv, exits, logs and hashes are in `round2/focused-gates.json`, `round2/locator-card/commands.json`, `round2/browser-run.json` and `round2/local-gates.json`.

| Command | Exit | Evidence/result |
| --- | ---: | --- |
| Unchanged checker with action+readback locator swap (exact stdin/argv in baseline-reproduction.json) | 0 | Reproduces the old vulnerability; `round2/locator-card/baseline-reproduced.log` |
| `node --test tests/ci/product-editor-matrix-ledger.test.mjs` | 0 | 23 PASS; `round2/node-focused.log`; real Playwright ESM collection seam preserved |
| `LC_MATRIX_ACTION_MUTATION=swap-locator node --test '--test-name-pattern=every actual mobile' tests/ci/product-editor-matrix-ledger.test.mjs` | 1 | Required injected fault; `round2/locator-swap.log` |
| Same command with `LC_MATRIX_ACTION_MUTATION=swap-press-key` | 1 | Required injected fault; `round2/press-key-swap.log` |
| `LC_TEST_LOCK_WAIT=300 LC_BROWSER_ENGINE=chromium LC_BROWSER_EVIDENCE_ROOT="$PWD/output/playwright/pr28-r2-20261009T172203" bash scripts/dev/test-local.sh --browser-product-editor` | 0 | 12 PE/review + 2 mandatory frozen CC12 cases PASS; same-run Go validates 231 rows; `round2/browser-product-editor.log`, `round2/cc12-playwright.log` |
| `bash scripts/dev/test-node.sh` | 0 | 1336 PASS / 0 FAIL / 0 SKIP; `round2/node.log` |
| `pnpm typecheck:admin` | 0 | `round2/typecheck.log` |
| Focused helper strict `pnpm exec tsc` (complete argv in local-gates.json) | 0 | `round2/helper-typecheck.log` |
| `bash scripts/dev/test-local.sh --list` | 0 | Registry preserved; `round2/registry-list.log` |
| `bash scripts/dev/check-gates.sh` | 0 | 82 modes all documented; every tracked test assigned; headers pass; `round2/check-gates.log` |
| `GOTOOLCHAIN=go1.27.2 go test -run Ledger -count=1 -v ./tests/foundation` | 0 | Both matrix tests and 21 corruption subcases PASS; 10 unrelated PG tests SKIP in this DB-free invocation; `round2/go-ledger.log` |
| `GOTOOLCHAIN=go1.27.2 go vet -tags browser ./...` | 0 | `round2/vet.log` |
| `node scripts/dev/pr-modes.mjs origin/r3/integration` | 0 | 53 modes selected; `round2/pr-modes.json` |

## Evidence level and remaining gates

**E3: complete in the tested BROWSER + REAL_PG environment using synthetic MOCK identity/provider fixtures**, bound to current source and raw artifact hashes. Read-only source review `932eca31` found no confirmed receiver/method P0/P1; it is E1 and does not replace independent runtime review.

CI gates needed: foundation-shards, `--browser-product-editor` and the planner's 53-mode set. Other 52 modes, full foundation, current-head K3/CI, provider SANDBOX/LIVE and deployment are **NOT_RUN** locally. These remain integrator-owned.

No product/API/Go/SQL/migration/lockfile changes. Owned runners and fixtures exited, browser lock released; other worktrees, `output/ext-agents/` and shared caches untouched. No local blocker. Commit only; integrator pushes.
