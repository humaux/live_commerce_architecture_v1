# PR475 4556 inventory follow-up — read-only diagnosis

- task_id: `53ea9f21-3f3b-4626-87ab-42dc853507b9-sub-inventory-repair`
- base_commit / inspected local HEAD: `e444f990002202f039725edb93d9e2b2d32fe576` (clean before evidence extraction; no source change).
- Native failing receipt: Gates `41302317241`, remote head `4556b8268f7b003a98bb2ab1ccc6d7aef009cdc1`, execution merge `3a6da5384386e0d5b22812faf53a18a217dbf06a`.
- Role: inventory UI diagnostic worker. Runtime model/effort UNKNOWN. No claim, delegation, downloads, test execution, source edits, commit or push.
- Authority: current dispatch, native ZIPs and source at the inspected HEAD; Humaux `a3d9eaf3-02ef-47b5-a26f-7c0b01f63d14` is the discovery pointer.

## 1. Desktop C08 search / immediate reload: navigation synchronization defect in the test

Job `124836434933`, artifact `13081478979` (SHA256 `aae46b3be52117151fee368c7205d725e8724db3314b27bec7fbe8664ad28cc8`). Native source: `playwright/run.OLf1bCoW/catalog-core/20261101T114457.897580685/results/parity-data-ops-PARITY-DAT-2df16-filter-and-survives-refresh/trace.zip`, members `1-trace.trace` and `1-trace.network`. Compact exact evidence is in `trace-summary.json` beside this report.

- Search click `call@985` runs 39062.653–39111.265ms. Its after snapshot at 39114.191 still has `/zh-TW/inventory?store=...`, without `q`.
- The client issues the intended `inventory?store=...&q=C08&_rsc=...` request at 39110.249ms. Network receipt records `net::ERR_ABORTED`.
- Test calls reload `call@987` at 39115.355ms, only 4.090ms after click returns. It navigates to the old URL without `q` at 39150.961ms. Network shows this unfiltered GET at 39124.141ms, status200.
- C08 is present among the unfiltered rows, so the first post-reload visibility assertion passes; A12 still exists and correctly fails count0. The failure screenshot visibly contains unfiltered rows. This does not establish that a completed filtered page loses its query on refresh.

Code chain: `tests/admin/parity-data-ops.spec.ts:192–194` immediately reloads after the real click; `parity-data-ops-support.ts:23–25` only awaits click, then runs verification. `InventoryView.tsx` submit calls `navigate({q:query})`, then returns to list; `Ledger.tsx:305–317` uses asynchronous `router.push`. `inventory/page.tsx` passes q to `workspaceData`; `backend.ts` forwards q to server catalog-ledger. There is no evidence here of a broken server filter or fixture stock.

Minimal candidate write_paths for root adjudication: only `tests/admin/parity-data-ops.spec.ts`, inventory search block. After the click, assert the URL has exact query value C08 and assert C08 visible / A12 count0 **before** reload, then retain reload and both original post-reload assertions. This adds an observable completion boundary and stronger pre/post coverage; it does not remove the original oracle, wait a fixed duration, alter timeout/retry, or change fixture/backend data. No implementation was made in this diagnostic round.

## 2. Legacy Back to products: positive oracle reaches a deliberately unsigned fixture

Job `124836435232`, artifact `13081974565` (SHA256 `8022ca5778b35c0330205bc106722429bafe41fc2ba8d63e18b83d57a49b6aa6`). Native trace: `playwright/run.m1wWDU9i/admin-ledger-20261101T114021.677389702/results/ledger-inventory-tray-keep-99580-g-lives-on-the-product-page/trace.zip`.

`call@808` resolves Back to products to `/en/products?store=5cee1c9b-479d-4943-96d9-efc5e980841b`, navigates successfully at 30708.799ms, and finishes at 30715.495ms. Thus the link/route is reached; `product-new` then times out. Directly viewed failure PNG and error context show the Products heading plus **Sign in to view products.** The shell still shows its mocked store. Screenshot SHA256 `99abbbe83f2acba27798441a58ae136db54fece4d29b2cb5ae40e08cbea2085a`.

Code chain: `tests/foundation/browser_admin_legacy_test.go` starts legacy dev Next with `COMMERCE_IDENTITY_ENABLED=0` and fixture bearer enabled; `tests/admin/fixtures/ledger-identity.ts` intercepts only browser `/api/stores`. This creates shell metadata, not server BFF identity. `products/page.tsx` uses shared `customers/page-data.ts`; line50 explicitly returns signed-out when authConfig is null, leaving store=null. `ProductList.tsx:96` requires a non-null server-authorized store to render product-new. Browser interception cannot authenticate the server page. Trace fixture metadata already includes catalog:write, so this is not a missing catalog-write grant.

The fail-closed product result is consistent with the current fixture and server auth boundary. It does not justify relaxing the positive editing oracle or adding a fixture bypass to production page loaders. Root must choose the acceptance placement before any edits:

1. Smallest candidate: move/preserve the exact positive Back to products → product-new href → real click → new-editor URL assertions in existing signed catalog-core acceptance, while legacy retains its tray/price/absence assertions and explicitly verifies its unsigned destination. Candidate paths: `tests/admin/ledger.spec.ts`, `tests/admin/catalog-core.spec.ts`, generated browser ownership. Required checks must continue to run both halves; migration is not a waiver.
2. Broader candidate: migrate the legacy harness to a real signed test session using the existing mock OIDC infrastructure. Candidate ownership begins `tests/foundation/browser_admin_legacy_test.go` plus its explicit fixture support. Exact full closure/write_paths need root review; this diagnosis does not claim a trivial config-only fix.

No changes to fixture grants, production auth/loaders, product-new visibility, expected href, or UI oracle were made. Authenticated product behavior for this exact legacy scenario remains NOT_RUN here.

## Evidence, commands, limits

Evidence written only under this own subdirectory: two first failure error-context files, two PNGs copied from existing ZIPs, filtered `trace-summary.json`, and this report. Original ZIPs/logs were read-only; no re-download. ZIP extraction used Python ZipFile with explicit member basenames; nested traces were read in memory.

Actual commands: initial `git status --short && git rev-parse HEAD && ls -la <root evidence directory>` exit0; read-only Python ZIP inventory/hash/extraction/trace-filter commands exit0; `view_image` opened both extracted screenshots; targeted source `sed`/`cat` reads were successful. Two combined discovery commands ended exit2 because guessed optional files were absent, then reads used observed actual paths; one combined read ended exit1 on an unmatched optional shell glob. These were discovery failures, not tests or acceptance results. Final `git status --short` shows only this evidence subdirectory untracked. No source changed.

Evidence level: E3 existing CI failure + source/trace diagnosis. This is not repaired/verified completion. Current local e444 runtime, proposed repairs, independent acceptance, SANDBOX and LIVE are NOT_RUN. Search completion barrier and acceptance relocation await root adjudication. No background process, fixture, database, port or WAIT_RUN was created. Stop after this bounded diagnostic pass.

## Authorized search synchronization candidate (subsequent root dispatch)

Root authorized only `tests/admin/parity-data-ops.spec.ts`'s inventory search block and this existing report, on base `e444f990002202f039725edb93d9e2b2d32fe576`. The first diagnostic section above describes the earlier read-only round; this section records the later authorized implementation. Astra's final assertion-guard candidate remains in the base, unchanged.

Added three assertions immediately before the original reload: URL query q equals C08, exact C08 product text is visible, and exact A12 product text has count0. The original reload and both original post-reload assertions remain verbatim. Both existing viewport registrations receive the same completion boundary. No timeout/retry, helper, product code, fixture, quantity, authority boundary, or generated ownership changed. This fixes the demonstrated reload-before-navigation race; own-head runtime success is **UNKNOWN** until GitHub executes it.

Actual static commands after the change:

| Command | Exit | Result |
|---|---:|---|
| `node --check --experimental-strip-types tests/admin/parity-data-ops.spec.ts` | 0 | Syntax only; no imports/test code executed. |
| `git diff --check` | 0 | No whitespace errors. |
| `git diff -- tests/admin/parity-data-ops.spec.ts` | 0 | Exactly three additive assertions in the authorized search block. |
| `shasum -a 256 tests/admin/parity-data-ops.spec.ts` | 0 | `ed45c59404feb6d87073a4049c84759c1b472f62621473edeeebaee997462b99`. |

Only the spec and this report are staged for one local commit; extracted PNGs, error contexts and trace-summary remain local evidence, not staged. No ZIP was copied into this worktree or committed. Root retains generated ownership regeneration. Runtime, strict typecheck, browser/PG, independent review, SANDBOX and LIVE are **NOT_RUN**. No push, CI dispatch, application process or WAIT_RUN was created.

## Concrete ledger positive-oracle migration plan — NOT IMPLEMENTED

The original positive entry check is precisely `tests/admin/ledger.spec.ts:560–566`; the preceding tray/price/edit-link/absence assertions at lines539–559 do not depend on signed product-page identity and stay in legacy, unchanged. Do not interpret the unsigned fixture's absent product-new as a product defect.

Proposed minimal registration in `tests/admin/catalog-core.spec.ts`: insert one separate English test immediately after the additive PE registration block (currently ends line106), before the existing `for (const v ... variants)` at line108. Guard only this added registration with `process.env.PRODUCT_EDITOR_ACCEPTANCE !== "1"`, matching the existing frozen CC12 lane. Use the file's existing regular Playwright `test`, origin/baseURL and `signedLogin(page)` (lines49–53), without importing the legacy identity mock. After the real signed login, retain the original six-line entry flow from ledger lines561–566 verbatim:

| Original positive behavior | Destination behavior |
|---|---|
| `page.goto("/en/inventory")` | Same signed-origin inventory navigation. |
| Exact `Back to products` link click | Same exact-role real click. |
| `const add = page.getByTestId("product-new")` | Same visible product-list control. |
| `expect(add).toHaveAttribute("href", /^\\/en\\/products\\/new(\\?store=[0-9a-f-]{36})?$/)` | Preserve the original complete href oracle verbatim. |
| `add.click()` | Same real click, with no API substitute or force. |
| `expect(page).toHaveURL(/\\/en\\/products\\/new/)` | Preserve the original full-editor destination oracle verbatim. |

A standalone signed test avoids changing either frozen desktop/mobile CC12 test body or creating an extra product, SKU, grant or Go fixture. It needs no selected SKU: the original positive block also begins by resetting to the inventory list. Global serial configuration and existing trace/screenshot handling already apply. Suggested descriptive test name: `inventory Back to products opens the full product editor for a signed merchant`.

In legacy, retain every original assertion through line559. Replace only the migrated positive tail with explicit checks appropriate to its actual unsigned scope: validate Back to products href to the product-list route, click it, assert the products URL and existing `Sign in to view products.` alert, and assert no product-new control. Add a comment pointing to the named signed test, and a reciprocal comment in the signed test pointing to the legacy case. This new refusal assertion supplements the relocated positive oracle; it must never substitute for it. Root must approve the exact migration before any such edits.

Execution coverage is already provided by `TestBrowserCatalogCore`: `browser_catalog_core_test.go:122–128` always runs `CC12-frozen`, including when the product-editor mode also runs PE12–17. The CC12 call sets `peFlag="0"` and runs catalog-core.spec.ts at lines195–207. Thus both `--browser-catalog-core` and `--browser-product-editor` reach the added signed test through the existing required CC12 member. No new Go harness, runner, timeout, auth toggle or test registry mode is needed.

Future implementation write_paths for root approval: `tests/admin/ledger.spec.ts` (tail only), `tests/admin/catalog-core.spec.ts` (one additive signed registration), this report, and root-owned generated `output/ci-select-backend-browser/browser-ownership.json`. Acceptance: GitHub admin-legacy must pass retained tray/price/absence plus explicit unsigned destination; both catalog-core and product-editor CC12 must run and pass the named signed positive test, retaining its href and real-click destination assertions. Independent review should compare each original positive assertion above against the destination, with no deletion/weakening or fixture permission changes. All of that remains **NOT_RUN / NOT_IMPLEMENTED** in this commit.

## Authorized exact positive-oracle migration (subsequent root dispatch)

Root then approved this concrete plan after reading the actual test tails and registration code. This section records its implementation on base `01a5458dd5c60b09244c3f18b1bc645d8fd5f3ba`; the preceding plan is historical. task_id remains `53ea9f21-3f3b-4626-87ab-42dc853507b9-sub-inventory-repair`. Model/reasoning: **Sol/high**, supplied by the root dispatch (earlier rounds could only report UNKNOWN). Role: inventory UI implementation worker. Single implementation pass, one local commit, no push or runtime run.

The original six positive statements at base ledger lines561–566 now follow `signedLogin(page)` in the independent catalog-core test named **inventory Back to products opens the full product editor for a signed merchant**. Statement contents, regexes, exact role/name matching, real clicks and final URL assertion were copied verbatim; only indentation follows the new nesting. Registration is after the PE block and before the unchanged CC12 variants, guarded by `PRODUCT_EDITOR_ACCEPTANCE !== "1"`. The signed test points back to the original legacy test by its exact name. No existing CC12/PE body was changed.

Legacy retains every original statement through line559, including the visible tray and stock controls, required reason, whole-dollar NT$ price regex, no price input, edit-product href, all six forbidden inline controls, no name/price/description inputs, and no create panel. Only its tail now asserts the Back to products href, clicks the actual link, asserts the product-list destination, requires the exact **Sign in to view products.** alert and verifies product-new count0. A reciprocal comment names the signed positive test. These added refusal checks describe the observed unsigned fixture; the original positive oracle continues in its signed owner, not as an exemption.

No product source, auth/config, fixture/grant, Go harness, guard, timeout/retry, generated ownership, prior search synchronization, or Astra guard candidate changed. The existing assertion guard's source discovery is for storefront `*-gate.mjs`; this change does not modify those inputs or its mappings/whitelist. No source-level conflict was found; runtime guard execution remains NOT_RUN, and a future guard failure must be reported rather than waived.

| Actual static command | Exit | Result |
|---|---:|---|
| `node --check --experimental-strip-types tests/admin/ledger.spec.ts` | 0 | Syntax only; no imports/test code executed. |
| `node --check --experimental-strip-types tests/admin/catalog-core.spec.ts` | 0 | Syntax only; no imports/test code executed. |
| `git diff --check` | 0 | No whitespace errors. |
| `git diff -- tests/admin/ledger.spec.ts tests/admin/catalog-core.spec.ts` | 0 | Only the approved legacy tail and one standalone signed registration. |
| `shasum -a 256 tests/admin/ledger.spec.ts tests/admin/catalog-core.spec.ts` | 0 | Current source hashes below. |

```text
0eb1a3d0f62865e5275727cd0ba621e094c458e9db016dd9cee91a1ba943dc92  tests/admin/ledger.spec.ts
d5949041e1a8699d0986414b428ea9b60cb5d4f5841240d17443f73071d43594  tests/admin/catalog-core.spec.ts
```

Only these two specs and this existing report are staged. The five own extracted evidence files remain untracked and no ZIP is staged. Root must regenerate browser ownership with all lanes and independently review this diff plus the positive-statement mapping. GitHub admin-legacy, catalog-core/product-editor CC12, required checks, strict typecheck, all runtime tests, screenshots/click ledgers for this candidate, independent acceptance, SANDBOX and LIVE remain **NOT_RUN**. E1 syntax/structure is ready; current-source runtime success is UNKNOWN. No local process, fixture, port or WAIT_RUN was created.
