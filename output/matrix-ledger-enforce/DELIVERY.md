# matrix-ledger-enforce PR #28 round 1 delivery

- Branch/worktree: `unit/matrix-ledger-enforce`, `.worktrees/matrix-ledger-enforce`. Base `c2665c530493527d1cf1ab4d782b4afb6d626d8f`, trunk `4ff99766`. Fresh fetch/merge before handoff exits 0 (already up to date).
- Tested source: **`a7ba8593c2bd60d321142855229dec2d69f6446d`**; final commit adds evidence only. `round1/SOURCE.json` binds current files; the browser manifest records frozen run hashes and the unchanged Go runtime definitions.
- Author Codex-4 (parent model/effort not exposed); isolated ui_worker GPT-6.1-sol/high; read-only explorer GPT-6.1-sol/medium. Task `55dcf44b-9378-4b31-970c-fe03b3951cc5`. Packet `output/integrator/triage/pr28-c2665c53.md`. No push.

## Three threads fixed

1. **4231055430:** one JSON declaration table, embedded by Go and imported by the actual Playwright recorder, defines **77 rows per locale / 231 total**. It covers every actionable control plus existing persistence readbacks and native confirmation: name, individual axis controls, SKU fields, create/result navigation, archive selection, replacement fields and bulk open/Escape. All 35 original static operation sites (84 locator operations per locale) remain; native accept now has its own awaited record. The source AST gate rejects bare actions, removed wrappers, a second control borrowing a callback and direct ledger writes. These four injected faults are RED.
2. **4231055440:** Go validates every exact identity and canonical `expected` string before requiring `actual=PASS`; PASS means the actual UI action and declared postcondition assertions passed. Original dynamic values/UUIDs remain in `observed`. Missing, altered, empty, null and wrong-type expected claims all have genuine RED→GREEN controls. The successful corruption fixture is an independent projection of the current actual browser run, including expected fields. Exact set, duplicate, extra, wrong locale/width/action/page/row and malformed JSON checks remain.
3. **4231055446:** canonical current evidence is **committed** under `browser/`: [ledger](browser/product-editor-click-ledger.json), [Playwright log](browser/playwright.log), [manifest](browser/browser-evidence.json). Documentation/manifest paths are repository-relative. The manifest hashes the two raw artifacts and runtime sources; `TestProductEditorMatrixLedgerEvidence` reads this same checkout evidence. The root old browser manifest now redirects to this portable canonical manifest. Auxiliary screenshot names in the raw ledger belong to runner output; screenshots are not a delivery claim.

The previous 47-template claim covered recorded rows and omitted real operations; it is superseded. Historical root logs/manifests describe the previous batch. Current evidence lives under `round1/` and `browser/`.

## Additional seam caught and closed

The first mode attempt failed during collection, before executing any browser case: Node 24 rejected a JSON import without `with { type: "json" }`. A new **real Playwright generated-config `--list`** seam reproduces that TypeError RED, then passes after the one-line import correction. It starts no server/browser. The successful mode below is the only actual browser execution in this round; the unsuccessful collection attempt and raw log remain preserved.

The new actual fixture begins with name fill. This caught the old corruption test's `wrong-action=fill` as a no-op. It now uses an unknown action and every corruption control asserts that it changed the serialized input. Assertions were strengthened. The Go runtime types/table init/locale bindings/validator are byte-identical to the successful browser snapshot; only this corruption test changed afterwards. The current Go reader revalidates the canonical raw ledger directly.

## Exact commands / exits

Plaintext evidence, command records and SHA-256 mappings: `round1/{local-gates,go-gates,browser-run,helper-typecheck,log-hashes}.json`. Relevant raw red logs are committed uncompressed; full raw Node/build logs are retained in the task archive, with hashes in `round1/log-hashes.json`.

| Command | Exit | Result/evidence |
| --- | ---: | --- |
| `GOTOOLCHAIN=go1.27.2 go test -run '^TestProductEditorMatrixLedger$' -count=1 -v ./tests/foundation` before expected validation | 1 | `round1/expected-red.log`; all five expected corruptions admitted by original validator |
| `GOTOOLCHAIN=go1.27.2 go test -run '^TestProductEditorMatrixLedgerEvidence$' -count=1 -v ./tests/foundation` before evidence packaging | 1 | `round1/evidence-red.log`; absolute reference and three missing canonical files |
| `node --test tests/ci/product-editor-matrix-ledger.test.mjs` | 0 | **12 PASS** including actual consumer seam; `round1/esm-collection-green.log` |
| `LC_MATRIX_ACTION_MUTATION=bare-click node --test tests/ci/product-editor-matrix-ledger.test.mjs` (also removed-wrapper, extra-control, direct-push) | 1 each | exact final commands/exits in `round1/current-action-calibration.json`; each turns the current gate RED |
| `node --test --test-name-pattern='real Playwright ESM consumer' tests/ci/product-editor-matrix-ledger.test.mjs` before JSON attribute | 1 | `round1/esm-collection-red.log` |
| `LC_TEST_LOCK_WAIT=300 LC_BROWSER_ENGINE=chromium bash scripts/dev/test-local.sh --browser-product-editor` | 1 → 0 | first collection failure: zero cases; then **12 PE/review cases + 2 frozen CC12 cases PASS**, `round1/browser-product-editor.log`, `round1/cc12-playwright.log` |
| `GOTOOLCHAIN=go1.27.2 go test -run Ledger -count=1 -v ./tests/foundation` | 0 | **2 scoped tests / 21 corruption subcases PASS**; 10 unrelated PG cases SKIP because this DB-free command has no PG grant; `round1/go-ledger.log` |
| `GOTOOLCHAIN=go1.27.2 go vet -tags browser ./...` | 0 | `round1/vet-current.log` |
| `bash scripts/dev/test-node.sh` | 0 | **1278 PASS / 0 FAIL / 0 SKIP**, `round1/node.log` |
| `pnpm typecheck:admin` | 0 | `round1/typecheck.log` |
| Focused strict helper TS command | 0 | exact argv in `round1/helper-typecheck.json`; `round1/helper-typecheck.log` |
| `bash scripts/dev/test-local.sh --list` | 0 | `round1/registry-list.log` |
| `bash scripts/dev/check-gates.sh` | 0 | **82 modes documented**, tests assigned and headers pass; `round1/check-gates.log` |

The actual ledger contains **286 total rows**, including **231 required unique PASS rows**, 77 each for zh-TW/zh-CN/en at 390px. The Go harness reads the same-run bytes, not shared legacy output. Existing exact PG create/edit/archive/replacement facts and mandatory frozen CC12 remain intact. All callback assertions and waits are preserved; draft no-write checks, bulk close/focus and real native-confirm recording are added.

## Evidence level / CI / NOT_RUN

**E3: completed in the tested real browser/Next/BFF/Go/isolated REAL_PG environment with synthetic MOCK identity/provider fixtures.** Bound to current source/artifact hashes. Read-only independent source review `30835d99` and final source/evidence audit `639bc52a` found no confirmed P0/P1; these are E1 reviews, not independent runtime reruns or current-head K3/CI.

CI gates needed: foundation-shards and `--browser-product-editor`, plus the real planner's full **53-mode** selection (`round1/pr-modes.json`). Other 52 modes, full foundation, provider SANDBOX/LIVE and deployment are NOT_RUN locally. Current-head review and CI remain integrator-owned.

No product/API/SQL/migration/dependency changes; 0168 untouched. All owned runners/fixtures completed, browser lock released. Runner-owned legacy artifacts were hash-archived and restored; shared caches and others' work untouched. No local blocker. Commit, stop; integrator pushes.
