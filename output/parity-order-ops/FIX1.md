# PARITY-ORDER-OPS-FIX1 — directed repair, PR CI pending

## Authority and checkout

- Authority: integrator's current human FIX1 card; repository `AGENTS.md`; `架构.md` §§0,1,2,26,27; native GitHub run/job logs and artifacts below.
- PR: #364, branch `unit/parity-order-ops`, remote head `48ce430be39fc5a1095351ced5b49c799ad8ff70`.
- Required first merge performed: `origin/r3/integration` at `a3dc78e4d5ec72831aaf6a376793f0f6a7a56425` → local merge `8a193af79c60caa235735bceac0314a34eaf5fbf`. Included in the single implementation push; exact new head/run ids are in the PR handoff.
- Only merge conflict: generated `output/ci-select-backend-browser/browser-ownership.json`; resolved by the prescribed generator, never by hand.
- `node scripts/dev/derive-browser-ownership.mjs` → exit 0; `node scripts/dev/derive-browser-ownership.mjs --check` → exit 0. These are explicitly permitted generation/static operations, not local tests.
- Existing uncommitted DELIVERY/red/relay/Figma files were present on arrival and preserved. Main checkout was not modified. No test/server/container started; product/test changes are the directed batch below; no force-push, merge of PR, auto-merge, status write or production action.
- Role: integrator-dispatched implementation lane; actual runtime model/effort NOT_EXPOSED. No recursive delegation or duplicate task claim. Tools: ponytail + ci-only-evidence; Humaux development detail `7bdb220b-0798-4b1d-aff5-16ee94cae75f`. Code-map/index tools were not exposed; targeted `rg`, native Git diff and source reads substituted.

## Native evidence (E3 failure evidence, not completion)

Gates run [41053547208](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41053547208), attempt 1: FAILURE, head `48ce430be39fc5a1095351ced5b49c799ad8ff70`; receipts bind the tested synthetic merge to `08728bb99450fb590b4702843ad776711332ff07`.

- C = artifact `gate-browser-product-editor`, id `12976229414`; local `FIX1-artifacts/catalog/playwright/run.hvidlfPg/catalog-core/20261030T122628.197047915/playwright.log`.
- R = artifact `gate-browser-returns-ui`, id `12978820466`; local `FIX1-artifacts/returns/playwright/run.vI8ohMiD/returns-ui/20261030T125441.557314044/playwright.log`.
- Full original job logs retained locally as `FIX1-catalog.log` and `FIX1-returns.log`. Downloaded failure screenshots, contexts and traces are failure evidence, intentionally retained locally, not proposed repository fixtures.
- C context: `results/catalog-core-CATV2-signed--7d8ea-d-keeps-the-input-MOCK-409-/error-context.md`: merchant save-error alert exists; Next's route-announcer alert also exists.
- R browser summary: 24 failed, 8 passed. Existing `returns-ui.spec.ts` cases pass. Signed assertions remain; review coverage is additive, legacy selectors are adapted as below; business fixture payloads, thresholds, timeouts and exemption lists are unchanged.

## Every failed job → failed test → evidence → class

Classes are the card's (a) signed-frame replacement, (b) PR product regression, (c) identical-base failure. For (a), the legacy harness assumption is stated explicitly; for (b), a newly exposed missing implementation is distinguished from a previously shipped regression. No identical-base exemption (c) is claimed.

| Job | Failing test / case | Exact evidence | Class / disposition |
|---|---|---|---|
| `124058656680`, bundle:03 | `TestBrowserCatalogCore/PE12-17-TWD` → `CATV2 concurrent committed-stock refusal rolls back every edit and keeps the input (MOCK 409)` | C:88–110; `catalog-v2-simplified.acceptance.ts:356`: `getByRole('alert')` resolves to 2 elements, the catalog-task-editor alert and `__next-route-announcer__`. Both appear in context. | **(a), harness precision**: global alert lookup incorrectly includes Next's route announcer. Scope to the signed catalog-task-editor, keep visibility and add the merchant error-text assertion. All rollback, quantity, draft and one-write assertions remain. Explicitly named FIX1 job; no unrelated product change, no (c) exemption. |
| `124058656904`, bundle:07 | parity manual-order zh-TW desktop | R:38–59; line 105: heading `1 · 商品` absent | **(b), missing product implementation exposed by the new RED**: implement the signed picker/draft/review at the root. This is a tests-first feature gap, not a claim that previous product code regressed. |
| same | parity returns zh-TW desktop | R:79–100; line 150: heading `退款與售後` absent | **(b), missing product implementation**: signed task grouping, requests/record views and shared chooser implemented; native GREEN pending. |
| same | parity merchant CVS zh-TW desktop | R:120–141; line 181: heading `查門市編號` absent | **(b), missing route**: implement authorized orders/cvs-match and shared explicit-choice public-directory component; native GREEN pending. |
| same | parity manual CVS zh-TW desktop | R:161–182; line 234: heading `2 · 收件人與配送` absent | **(b), missing product implementation**: signed task grouping, requests/record views and shared chooser implemented; native GREEN pending. |
| same | parity manual-order zh-TW mobile | R:202–225; line 45: `nav-orders` received hidden | **(a), signed mobile-shell harness adaptation**: open the real drawer, retain authorized nav visibility assertion, close with Escape. Fix shared helper once. Product navigation is not exposed artificially. |
| same | parity returns zh-TW mobile | R:246–269; same line 45 hidden navigation | **(a)**, same shared-helper repair; no timeout or auth assertion removed. |
| same | parity merchant CVS zh-TW mobile | R:290–313; same line 45 hidden navigation | **(a)**, same shared-helper repair; no timeout or auth assertion removed. |
| same | parity manual CVS zh-TW mobile | R:334–357; same line 45 hidden navigation | **(a)**, same shared-helper repair; no timeout or auth assertion removed. |
| same | parity manual-order zh-CN desktop | R:378–399; line 105: heading `1 · 商品` absent | **(b), missing product implementation**: signed task grouping, requests/record views and shared chooser implemented; native GREEN pending. |
| same | parity returns zh-CN desktop | R:419–440; line 150: heading `退款与售后` absent | **(b), missing product implementation**: signed task grouping, requests/record views and shared chooser implemented; native GREEN pending. |
| same | parity merchant CVS zh-CN desktop | R:460–481; line 181: heading `查门店编号` absent | **(b), missing route**: implement authorized orders/cvs-match and shared explicit-choice public-directory component; native GREEN pending. |
| same | parity manual CVS zh-CN desktop | R:501–522; line 234: heading `2 · 收件人与配送` absent | **(b), missing product implementation**: signed task grouping, requests/record views and shared chooser implemented; native GREEN pending. |
| same | parity manual-order zh-CN mobile | R:542–565; line 45: `nav-orders` received hidden | **(a)**, same shared-helper repair; no timeout or auth assertion removed. |
| same | parity returns zh-CN mobile | R:586–609; same line 45 hidden navigation | **(a)**, same shared-helper repair; no timeout or auth assertion removed. |
| same | parity merchant CVS zh-CN mobile | R:630–653; same line 45 hidden navigation | **(a)**, same shared-helper repair; no timeout or auth assertion removed. |
| same | parity manual CVS zh-CN mobile | R:674–697; same line 45 hidden navigation | **(a)**, same shared-helper repair; no timeout or auth assertion removed. |
| same | parity manual-order en desktop | R:718–739; line 105: heading `1 · Items` absent | **(b), missing product implementation**: signed task grouping, requests/record views and shared chooser implemented; native GREEN pending. |
| same | parity returns en desktop | R:759–780; line 150: heading `Refunds and after-sales` absent | **(b), missing product implementation**: signed task grouping, requests/record views and shared chooser implemented; native GREEN pending. |
| same | parity merchant CVS en desktop | R:800–821; line 181: heading `Find store code` absent | **(b), missing route**: implement authorized orders/cvs-match and shared explicit-choice public-directory component; native GREEN pending. |
| same | parity manual CVS en desktop | R:841–862; line 234: heading `2 · Recipient and delivery` absent | **(b), missing product implementation**: signed task grouping, requests/record views and shared chooser implemented; native GREEN pending. |
| same | parity manual-order en mobile | R:882–905; line 45: `nav-orders` received hidden | **(a)**, same shared-helper repair; no timeout or auth assertion removed. |
| same | parity returns en mobile | R:926–949; same line 45 hidden navigation | **(a)**, same shared-helper repair; no timeout or auth assertion removed. |
| same | parity merchant CVS en mobile | R:970–993; same line 45 hidden navigation | **(a)**, same shared-helper repair; no timeout or auth assertion removed. |
| same | parity manual CVS en mobile | R:1014–1037; same line 45 hidden navigation | **(a)**, same shared-helper repair; no timeout or auth assertion removed. |
| `124082737517`, receipts | matrix-receipts | Native failed log: `receipt --browser-product-editor: FAIL`, `receipt --browser-returns-ui: FAIL`; `matrix-receipts: RED (2 problem(s))` | Cascade from the two members above. No standalone missing-browser-spec receipt failure. |
| `124085294321`, required | required contexts | Native failed log: `plan=success build=success ... gate=failure aggregate=success ... receipts=failure` | Cascade. Aggregate itself succeeded. |

## Directed batch and relay

FIX1 explicitly names the catalog failure. This session initially reused the old card boundary and prematurely proposed BLOCKED; that judgment was withdrawn before any push. The current human directed list controls. Raw evidence above is unchanged; the final classifications describe the repair rather than inventing identical-base evidence.

- ManualOrder: numbered task groups, staged SKU picker (cancel isolation), explicit review then existing idempotent placement. Recipient/phone, products/quantities, full destination and payment timing stay in draft/review/result. No client total or price-override field. Optional fields are collapsed. Session fences and unknown-outcome replay logic remain.
- ReturnsList: signed home → requests → read-only RMA record; real query links keep filters on refresh and browser Back; record links to original order. Existing refund gaps and order-side RMA/refund controls remain. No invented refund row, amount, buyer statement or processing note.
- CvsLookup shared by the manual draft and merchant tool: existing pure matcher, fixed-brand public snapshots copied byte-for-byte from the current storefront snapshot, canonical string-code/name/address only after explicit click; copy/change/missing-code recovery. No provider or buyer write; no paste storage. Existing admin packaging includes public assets (`scripts/dev/package-admin.mjs`); the admin proxy now admits only GET/HEAD of the five exact immutable snapshot filenames before locale routing. Private API guards and matcher exemptions are unchanged.
- Shared new seller copy lives in packages/i18n for zh-TW/zh-CN/en. The new parity cases now also click Review and Back in all three locales and check every recipient/product/pickup fact, without submitting an order.
- Fixture payloads, product amounts, RMA facts and permissions are unchanged. The common mock transport header now matches production `private, no-store`; the strict reader is unchanged. This corrects the RMA read refusal observed behind the first heading assertion.
- Legacy manual-order-link browser setup uses Open picker → Select → Apply → Review → existing submit. Existing bank/COD receipt, expiry, replay, buyer-link, refund and stock assertions remain. Existing returns test clicks View requests before the same filter/reload/gap assertions. The component compilation harness lists every new ManualOrder import. No source exclusion or exemption list changed.

Specified base Gates41105181453 was IN_PROGRESS when read; no base-green claim or identical-base assertion exemption. One batched repair push follows the completed assertion RED. This is directed repair attempt 1 for these assertions in this session. If the same test remains RED after this push, STOP with its exact assertion and artifact; do not try a third approach.

Local static receipts: eight `node --check` commands exit0 (three TS browser specs, timezone harness, manual-order-link driver, i18n order-ops, directory client and admin proxy). Full `tsc --project apps/admin/tsconfig.json --noEmit --incremental false` exit1 because the worktree has no Next/React/workspace dependency links. That is not a successful typecheck. No local test/build/server/container ran. Browser ownership is regenerated and checked after all staged changes, before commit. No Go source/package dependency changed; no dep-map regeneration required.

Data gaps remain exactly as DELIVERY.md: pre-placement quote/amount fields and refund-record/human-reference/note facts are not in the assigned frozen DTOs. Partial data-gap screenshots cannot establish complete frame parity. No migration, R2 pin, API contract, OpenAPI, lockfile, route-dispatcher baseline or next-startup gate assertion changed.

Native original failure evidence = E3 FAILURE only. Local syntax/ownership consistency = E1. Author judgment = E0. New PR CI/GREEN, runtime click/persistence/parity, independent review, SANDBOX/LIVE/production = PENDING/NOT_RUN. Final Gates/Foundation run ids are posted in the PR/handoff after the push, without an evidence-only follow-up commit.
