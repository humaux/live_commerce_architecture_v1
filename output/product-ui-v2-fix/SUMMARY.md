# product-ui-v2-fix — scoped review repairs

Date: 2026-10-03. Branch: `unit/product-ui-v2-fix`. Base: `56a32b381c894bdc72b89dc7ee04c5de54a1b04a`.

Source under test: `378ae093548c` plus its ancestors `66843267` and `9d7e60c7`. Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/product-ui-v2-fix`. No push, merge, deployment, credentials, Go/SQL change, or external provider request. All evidence is packaged alongside this report in the assigned worktree. This explicit dispatch overrides PROCESS's default main-checkout evidence location.

## Requested repairs

| Item | Status | Implementation and acceptance | Commit |
| --- | --- | --- | --- |
| 1. Exclusive SKU archive patch | PASS | `editDocument` emits only `{id, active:false}` before processing a newly archived row's other changes. Existing active-row explicit clears remain tested; invalid discarded price/quantity cannot prevent archive. Real clicks bulk-set price to 80, uncheck one Active row, explicitly confirm archive, save once (200), reload: archived row absent, remaining price 80. | `9d7e60c7`; browser regression `66843267` |
| 2. G04 fixture false positive | PASS | Split the identical PNG bytes at the repository's catalog-core boundary; scanner and regex unchanged. Final-source strict G04 passes. | `378ae093` |
| 3. Formal PE release gate | PASS | Formal `--browser-product-editor` in validation, usage, preflight/build and run branches, documented in GATES. Runs the existing Go fixture with PE opt-in, then mandatory frozen CC12. `release-gate.sh --list` reports `B-browser-product-editor`; `check-gates` recognizes 60 modes. **No change to release-gate.sh.** | `378ae093` |
| 4. Authorization loss preserves receipt fence | PASS | Client boundary refusal and HTTP 401/403 (including non-JSON bodies) require reconciliation, never a fresh command. Durable fence survives reload; Retry/new writes disabled until reconciliation. Covered create, copy, image order and publish stages; definitive 404/409/422 remains distinct. | `66843267` |
| 5. Meaningful status-help assertion | PASS | Restored localized `product-status-help`, connected with `aria-describedby`; CC12 asserts exact active explanation before and after save, in addition to persisted active state. Not a tautology. | `66843267` |

## Commands and observed results

All browser commands used `export LC_TEST_LOCK_WAIT=14400`; no lock deletion or other task process termination. Source was frozen before final browser runs. This is a scoped repair acceptance, **not an overall strict release verdict**.

Artifact-only formatting: trailing terminal progress-line padding/CRs in copied `.log` files were normalized for Git whitespace checks. Test names, results, failures and thresholds are unchanged; the original browser logs remain in their run directories. `git diff --cached --check` then exits 0.

Lock observation: the runner in this base acquires the machine PG lock for the Stripe browser branch, not these ordinary browser modes. The final sequential runner also waited on an existing `${LC_TEST_LOCK_DIR:-${TMPDIR:-/tmp}/lc-test-pg.lock}` with the requested bounded deadline; no such lock directory was observed at launch. This is not a claim of lock exclusivity or of having queued behind the other release process. Shared runner locking was not broadened in this repair.

| Command | Exit | Count / evidence |
| --- | --- | --- |
| `go build ./...` | 0 | `go-build.log` |
| `go vet ./...` | 0 | `go-vet.log` |
| `git ls-files '*.go' \| xargs gofmt -l` | 0 | Empty `gofmt.log` |
| `bash scripts/dev/check-gates.sh` | 0 | 60 browser modes; `check-gates-final.log`; size warnings only, no hard gate failure |
| `bash scripts/dev/test-node.sh` | 0 | 328 passed = 308 + 4 + 12 + 4; 0 failed; explicit R04 NOT_RUN below; `test-node-final.log` |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `admin-tsc-final.log` |
| `LC_RELEASE_GATE_OUT="$PWD/output/product-ui-v2-fix/g04-source-final" bash scripts/dev/release-gate.sh --strict --only G04` | 0 | 1 PASS; source `378ae093548c`; `g04-source-final.log` and `g04-source-final/results.tsv` |
| `bash scripts/dev/release-gate.sh --list` | 0 | `release-list.log`: `B-browser-product-editor` |
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | PE suite 10/10 + frozen CC12 2/2; `browser-product-editor-final.log` |
| `bash scripts/dev/test-local.sh --browser-catalog-core` | 0 | Frozen CC12 2/2; `browser-catalog-core-final.log` |
| `bash scripts/dev/test-local.sh --browser-catalog-media` | 0 | 4 viewport/locale cells, 20 cases, exact DB readback; `browser-catalog-media-final.log` |
| `bash scripts/dev/test-local.sh --browser-merchant-buyer` | 0 | 8 cases, 3 locales; `browser-merchant-buyer-final.log` |
| `bash scripts/dev/test-local.sh --browser-click-sweep` | 0 | 120 page/viewport/locale units; 997 controls = 977 PASS / 0 FAIL / 20 SKIP; 18/18 journey steps; known/new failures 0/0; `browser-click-sweep-final.log`, `click-sweep-summary.json` |

## Red → green evidence and preserved assertions

- Archive pre-fix unit failure is retained in `red/node-regressions.log`; active:false plus other fields failed the new archive-only contract assertion. This first aggregate run also contained an unrelated Node strip-types loader error, which is **not** treated as a meaningful red.
- Receipt meaningful red: `red/receipt.log`, 1 pass / 3 fail with the old code. Green: `node-regressions-green.log`, 10/10 archive + receipt tests. The actual client is executed with Node transform-types (needed by an imported TypeScript parameter property).
- G04 red: `red/g04-real.log` and `red/g04/results.tsv`; green: `g04-source-final/`. PNG content is unchanged; only literal source boundaries changed.
- Browser attempt 1 (`browser-product-editor.log`, copied to `red/browser-r1.log`) failed in the new archive driver's missing matrix-stock input. Attempt 2 (`browser-product-editor-r2.log`) was interrupted after discovering that the new driver failed to accept its native archive confirmation. Only this task's Playwright worker was stopped; the fixture runner completed cleanup and frozen CC12. These attempts are **not PASS**. Corrected driver uses real stock-field and confirm clicks; final run is 10/10. Original PE12–PE17 and frozen CC12 assertions remain intact.
- No assertions deleted or thresholds loosened. Old wrong archive payload assertion was updated under the explicit backend contract ruling, with the previously bundled normal-clear case preserved separately.

## Browser evidence / click ledger

- Formal PE raw output: `output/playwright/catalog-core/20261003T074220.968815000/playwright.log` (10 passed); durable copy here: `pe-review-playwright.log`.
- Mandatory frozen CC12: `output/playwright/catalog-core/20261003T074244.647936000/playwright.log` (2 passed); durable copy here: `pe-frozen-cc12-playwright.log`.
- Standalone CC12: `output/playwright/catalog-core/20261003T074416.807744000/`.
- Catalog media: `output/playwright/catalog-media/20261003T074500.047312000/`.
- Click sweep: `output/playwright/click-sweep/20261003T074545.782177000/`; refreshed tracked `output/ui-click-sweep/ledger.json`, `ledger.md`, `journeys.json`. Its historical `DEFECTS.md` and defect screenshots are not this run's result; current summary has no failures.
- New regression evidence here: `archive-price.{json,png}` and eight `receipt-*.{json,png}` pairs. Archive JSON records 200/one command/archive-only/reload. Receipt ledgers record one server commit and one client-denial attempt or two same-key/body HTTP attempts. Page routing injects faults **after** the real Go/PG write, not a fake success response.
- Existing PE screenshot matrix was refreshed in `output/product-ui-v2/`: zh-TW/zh-CN/en × 1366×768, 1586×992, 375×812, plus matrix/inventory/batch views. `screenshots.json` has geometry/hash metadata; `write-metadata.json` records command counts. These are synthetic fixtures, not customer data.
- Tier: BROWSER + REAL_PG with MOCK IdP. Receipt denial tests are explicitly FAULT_INJECTED + REAL_PG, not LIVE provider verification.

## Independent review and scope

- Root: Codex implementer/integrator and actual gate runner; only assigned worktree paths written. Runtime model/effort is not exposed as a reliable tool fact, so not invented here.
- `/root/p2_contract`: read-only reviewer (gpt-6-luna/high, inherited existing agent), no write paths. Reviewed staged authorization reconciliation and reported no remaining scoped gap; title `product-ui-v2 staged auth-reconciliation delta review`.
- `/root/p2_test_map`: read-only reviewer (gpt-6-luna/high, inherited existing agent), no write paths. Gate registration review memory `ae7e23e5-87cc-4911-8705-0ca46d7f21a1`; driver static preflight `e5e9d4af-d785-40bc-83b5-58a81b10bbab`. Final evidence audit `a3f09edc-9f7d-4e73-92d6-eb3be16843a2` confirms all 10 PE/review tests plus both frozen CC12 cases actually passed, ledger counts match, no weakened assertions. Neither reviewer claimed to rerun browser/PG tests.
- Skills applied: `frontend-architect` for one command/outcome path and existing component boundaries; `playwright` for real UI-driven fault/reload regressions. No parallel writer or new dependency.
- Root indexed the changed source/test entities in Humaux project `product-ui-v2-fix`; `editDocument`, `useProductDocument` and `useWrite` link to fix memory `6bb7613c-9b04-4a16-a0ed-ef310d7aa812`. Independent review/evidence inspection is not a claimed cross-family K3 release sign-off; that remains with the integrator.

## Limitations / NOT_RUN

- Full `release-gate --strict`, G07/full PG, provider SANDBOX/LIVE and deployment are NOT_RUN in this repair unit. Another worktree's release run is not used as this task's evidence.
- `tests/media/r04-input-runner.test.mjs` is explicitly NOT_RUN because `COMMERCE_R04_LIVEKIT_BINARY` is unset; normal test-node reports this instead of claiming full media coverage.
- Click-sweep's 20 control SKIPs are disclosed in `click-sweep-summary.json`: single-option selects, sign-out deferred to its own passing journey, and generic dialog saves deferred to dedicated real-write flows. The missing buyer `/live` route is separately reported by the existing coverage rule, not claimed tested. No known-defect allowance was added.
- Authorization rejection intentionally fails closed even on a first local/401/403 denial. Automatic fence clearing after login is not introduced; an unresolved command remains “needs reconciliation.” Session storage scope/lifetime and reconciliation tooling are unchanged, not represented as newly solved.
- The pre-existing broader review notes about manual reconciliation UX, existing-row bulk code, and stock-error wording were not additional repair scope in this dispatch.
- No page CSS, shell, Go/SQL, state machine, payment, stock authority, scanner, or strict-release parser relaxation.

## Handoff status

All requested commands have observed exit 0. Source changes are the three logical commits listed above. The evidence commit containing this final report also preserves the red runs, final logs, regression ledgers/screenshots and refreshed legacy PE/click-sweep evidence.

Cleanup readback after all gates: no process matching this worktree's Next servers, Playwright or click-sweep worker remained. Docker showed only the other task's `lc-foundation-test-98291` and pre-existing Humaux services; none was touched. Own fixtures were cleaned by the gate runners. No locks were removed.

Final Humaux trace and evidence commit SHA are reported in the handoff message after persistence; broader release approval remains the integrator's decision.
