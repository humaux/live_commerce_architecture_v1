# Current candidate: bounded assertion migration 2 (2026-11-01)

- task_id: `53ea9f21-3f3b-4626-87ab-42dc853507b9-sub-assertion-guard`; parent/root retains the PR475 author lease. This is the second and final bounded candidate.
- base_commit: `4556b8268f7b003a98bb2ab1ccc6d7aef009cdc1`; worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr475-inventory-repair`; branch `codex/pr475-inventory-repair`; entry status clean and HEAD matched.
- role/model/reasoning: same subordinate gate implementer, Astra high as assigned. No delegation. Independent review NOT_RUN.
- changed paths this round: `tests/ci/next-startup.test.mjs`, this report only. Direct caller/product/fixtures/lockfiles/CI budgets untouched.
- Current evidence level E1, static structure only. Runtime/mutation execution for this candidate remains NOT_RUN; no push/merge.

## New observed failure and finite correction

Foundation `41302317135`, job `124835659113`, at the above base reported one failing test: eleven-adapter parity, first missing merchant-buyer `expect(merchant.getByRole("link", { name: "Add product", exact: true })).toBeVisible()`. The same run passed the catalog twelve-mutation test and the original environment/readiness mutation test. Evidence read: `output/integrator/tools/logs/heartbeat-1144-pr475/foundation-filtered.log` in the integration workspace; raw receipt <https://api.github.com/repos/humaux/live_commerce_architecture_v1/actions/jobs/124835659113/logs>. This previous run does not accept the new candidate.

A full static AST inventory found precisely three remaining outer assertion migrations (six nodes including nested `expect`):

1. merchant-buyer `Add product` link visibility -> exact `Back to products` link visibility.
2. merchant-buyer `ledgerRow.locator(".available-value")` text `"0"` -> `ledgerBalance` text `"0"`.
3. storefront-publish `Add product` link visibility -> exact `Back to products` link visibility.

These are complete-call mappings scoped to their two files, using the prior whole-subtree replacement mechanism. No monetary expression, expected stock value, matcher strength, ordering comparator or baseline SHA changes. `tests/storefront/merchant-product.mjs` was read: creation uses real Back to products then product-new clicks with the exact editor href asserted; selection searches the requested `code`, requires one matching row, clicks its product button and disclosure, then returns `inventory-current-available`. `InventoryView.tsx` binds that test ID to the available balance. Neither the helper nor its callers were edited.

New negatives: each of the two gates must reject deleted entry assertion, wrong role, non-exact name, and attached-only matcher (eight cases); merchant-buyer additionally rejects deleting the zero-stock assertion and changing zero to one (two cases). Positive controls call the same parity function. Ten new mutation executions are staged for GitHub, not claimed as locally passed. The existing catalog and readiness negatives are unmodified.

## Static inventory of all eleven adapters

Read-only inline `node --input-type=module` used the **already installed** Prettier 3.9.8 Babel parser; no dependency install, gate execution, app startup or browser occurred. It parsed source and mapping literals, compared normalized complete call ASTs in original order, including nested `expect`, and compared startup classes. Excluded only parser location/comment/raw-format metadata. This is independent static evidence, not TypeScript-printer or runtime acceptance.

| Adapter | Original assertion nodes | Current nodes | Remaining missing after exact mappings |
| --- | ---: | ---: | ---: |
| browser | 106 | 106 | 0 |
| buyer-comms | 77 | 77 | 0 |
| catalog-media | 92 | 106 | 0 |
| manual-order-link | 107 | 115 | 0 |
| merchant-buyer | 59 | 59 | 0 |
| order | 434 | 462 | 0 |
| promotions | 112 | 158 | 0 |
| shop | 206 | 208 | 0 |
| shop-real | 173 | 262 | 0 |
| store-domains | 191 | 193 | 0 |
| storefront-publish | 100 | 102 | 0 |

All eleven also matched all five startup AST classes: environment member assignments, readiness status predicates, readiness/env for-loops, spawn calls, and env/childEnv/args/bin declarations. Final diagnostic output: `{"staticASTAdapters":11,"failures":0,"runtimeTests":"NOT_RUN"}`; process exit 0. These counts describe complete assertion equality in order; counts alone were not the comparator.

## Actual commands, failures and hashes

- `git status --short && git rev-parse HEAD`: exit 0, clean and matching base.
- `node --check tests/ci/next-startup.test.mjs`: exit 0.
- `git diff --check`: exit 0.
- Inline `node --input-type=module` AST enumeration before the change: exit 0; all eleven enumerated, only the three outer assertions above absent.
- Expanded inline AST startup comparison first exited 1 because Prettier adds offset field `__contentEnd` to statements. Inspecting the two readiness nodes proved equal semantics with only offsets 4586/5558 differing; the final diagnostic excluded this location field and exited 0. The actual repository guard was not changed for this diagnostic artifact.
- A read of the integration log by worktree-relative path failed (file not in this tree); the explicit integration-workspace path succeeded. No missing log was treated as evidence.
- `shasum -a 256 tests/ci/next-startup.test.mjs`: `2164284c76a2e6e058d7729bd38da17dba1889bd29d0f376c0bdea9d72785bdd`.
- Read-only caller hashes: merchant-buyer `099c2bf813e0f216fe0b41d77ff48f51619bcf0aff9ce6d41b436a2645dbd9a4`; storefront-publish `ab10d5c9778d074d998a1538bf92181b3d7fc2acbbbe3f7b75eb1a88532070a0`; merchant-product helper `2c36a724923195cdb9a5dac5397f986457a47612a24d0e31387a5b43de252c16`.

Root must regenerate ownership if required, run GitHub Foundation with all prior/new mutation witnesses, and obtain independent review after green. No local runtime tests, Go/PG/browser, services, downloads, push, merge or production writes. No new temporary file/process requires cleanup. Runtime equality and browser behavior remain NOT_RUN/UNKNOWN until CI. Stop after this candidate; a further failure requires root adjudication.

---

# PR475 signed inventory assertion guard candidate

- task_id: `53ea9f21-3f3b-4626-87ab-42dc853507b9-sub-assertion-guard`; parent coord remains held by root.
- base_commit: `1c2bee34d9e053c9102681edcb8cf084eec69345`.
- branch: `codex/pr475-assertion-guard`.
- worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr475-assertion-guard`.
- role/model/reasoning: subordinate gate implementer, Astra high as assigned by root; author is not independent reviewer.
- write_paths: `tests/ci/next-startup.test.mjs`, `tests/storefront/catalog-media-gate.mjs`, this report. Root owns final generated ownership integration.
- evidence level: E1 structure only. Runtime, mutation execution, browser and independent acceptance are NOT_RUN. No push or merge.

## Root cause and authority

Foundation run `41292495106`, job `124806437687`, reported ordered assertion parity 32/92, first missing `expect(row(fx.sku1.code)).toContainText(major(fx.sku1.price_new))`. Original receipt pointer: <https://api.github.com/repos/humaux/live_commerce_architecture_v1/actions/jobs/124806437687/logs>; read source record Humaux `d39917c4-db68-4415-abcf-ab2a4f88df6e`.

The fixed baseline remains `9b738e0af64ebd08c25b39f8081d46d4dc5674fb`. Signed inventory moved these facts from table cells into the selected SKU disclosure, changed the disabled-control location, and reordered assertions. Root explicitly authorized a precise equivalent migration, without weakening guard targets. Product source at base: `apps/admin/components/Ledger.tsx` renders `selected.code`, price, status and real product cover inside `inventory-product-details`. Actual browser behavior awaits CI.

## Exact migration

The gate visits SKU1 → SKU2 → SKU1 → SKU2 → SKU1 with real product-name, summary and close clicks. Each visit asserts the displayed SKU identity before its facts. Original fact order is restored:

| Baseline assertion | Signed equivalent |
| --- | --- |
| SKU1 row contains `major(fx.sku1.price_new)` | Selected SKU1 `tray-price` contains the identical expression |
| SKU2 archived badge attached | Selected SKU2 archived badge visible on both viewports |
| SKU1 active badge attached | Selected SKU1 active badge visible on both viewports |
| Inspector spinbutton disabled | Signed inventory spinbutton disabled while SKU2 is selected |
| Desktop SKU2 archived badge visible | Desktop selected SKU2 archived badge visible |
| SKU1 real cover visible | Selected SKU1 real cover visible |

Six complete assertion expressions are the only new mappings. AST collection reparses each mapped outer assertion with its inner `expect`, then stops descending the old subtree; it does not rewrite unrelated `expect(row(...))` calls. All original assertions remain required in order, including exact monetary expressions and overflow. The finite contiguous signed readback statement sequence is additionally frozen, so changing the selection, skipping disclosure or dropping identity checks cannot satisfy the guard by retaining only matcher text. Existing buyer-comms mapping, eleven-adapter count, startup environment assignments/variables, spawn, readiness predicates/loops and all old environment/readiness mutation cases remain unchanged.

## Mutation witnesses staged for GitHub

The new test requires the current adapter to pass, then checks twelve source mutations independently: remove exact price, substitute stale price, substitute the other SKU price, remove archived state, remove active state, remove disabled control, remove real cover, select the wrong SKU for price, remove disclosure click, remove SKU identity, remove original row visibility, and loosen overflow. Each mutation must change its target and be rejected by assertion parity or signed readback protection. Execution is NOT_RUN locally; no red/green result is claimed.

## Commands and evidence

| Actual command | Exit/result |
| --- | --- |
| `git status --short` / `git rev-parse HEAD` at entry | 0; clean, base matched |
| `node --check tests/ci/next-startup.test.mjs` | 0 |
| `node --check tests/storefront/catalog-media-gate.mjs` | 0 |
| `git diff --check` | 0 |
| `node scripts/dev/derive-browser-ownership.mjs` | 0; 55 modes |
| `node scripts/dev/derive-browser-ownership.mjs --check` | 0 immediately after generation |
| `python3 scripts/check_packet.py` | 0; `PASS_PACKET_STRUCTURE_ONLY` |
| Inline `node --input-type=module` AST comparison importing `/Volumes/data/live_commerce_architecture_v1/node_modules/typescript/lib/typescript.js` | Node process 1, `ERR_MODULE_NOT_FOUND`; comparison NOT_RUN. The surrounding shell continued to syntax checks and returned 0, which is not an AST pass. |
| `git restore -- experiments/results/packet-check.json output/ci-select-backend-browser/browser-ownership.json` | 0; reverted only this task's generated check outputs; root regenerates ownership during integration |

Final code SHA256:

- `tests/ci/next-startup.test.mjs`: `5337d47b96da09e260b0030fa2207ffd982301ab65b03e13dc041cf2578afad3`
- `tests/storefront/catalog-media-gate.mjs`: `7c30bb93b241c69621656f78cdb3efb13169a88b6ab0b6c5fc886834e0223e3e`

Packet/ownership checks preceded the final comment-only header correction; final syntax checks bind the above hashes. No local node tests, Go, PG, browser, servers or fixtures were started. No temporary processes need cleanup. A preliminary metadata read assumed task JSON was an object and failed with `AttributeError`; the corrected list read verified T02/T04/T05 and the relevant invariants, while packet check verified the DAG.

## Remaining gates

Root must regenerate ownership and run the frozen GitHub Foundation tests (including all old and new mutation witnesses), plus native catalog-media browser coverage across en/zh-TW and desktop/mobile. Confirm actual SKU navigation, disclosure behavior, disabled archived stock control, exact price, status, real cover and overflow. Cross-family independent review remains required after CI green. No product, fixture, dependency, lockfile, baseline SHA, budget or CI configuration changes are included. Production/SANDBOX/LIVE acceptance remains NOT_RUN.


## Root integration: direct consumers of the signed inventory/import UI

Root independently reviewed commit003ba740 and its six bounded AST mappings, original fact order, complete SKU readback and12 mutation negatives. BaselineSHA, eleven adapters, readiness/env/spawn comparisons and original startup mutation tests remain. Actual AST/runtime evaluation is pending locked-dependency GitHub CI; no local tests ran.

Two additional current-head failures from d39917c4 were addressed in the same integration batch: product-editor.acceptance.ts508 tried to inspect a collapsed disclosure after selecting a SKU, so the test now performs the real summary click before all existing visibility/price/no-edit-controls/link/geometry assertions; parity-customer-import.spec.ts75 matched both h2 and summary, so its exact copy assertion now selects the h2 role and preserves every other count/privacy/email/consent assertion. Authority: Ledger.tsx444 and ImportWizard.tsx225/227 current DOM, and Gates41292495090 artifacts13078276821/13078137241. No product data, fixture, timeout, threshold or assertion was removed or relaxed.

Known remaining failures (no claim of repair): import-wizard global timeout and checkout-offline proof-response/DST-fold hypothesis; classify the next exact-head evidence, do not blindly retry. Root retained both filechooser evidence from the prior475 branch and the independently approved484 document-time-origin canaries while resolving their merge conflict.
