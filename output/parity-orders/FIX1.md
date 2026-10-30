# PARITY-ORDERS-FIX1 — directed CI repair

Status: implementation/static review only; author assessment E0. GREEN PENDING. No local tests/servers/browser/PG/containers run.

Authority: integrator FIX1 card (2026-10-31), repository AGENTS.md and 架构.md §§0/1/2/26/27, contracts/merchant-orders-v2.md, current code and GitHub raw logs. Signed presentation authority remains owner-approved UX-orders frames already implemented by this PR; no design changes or Figma writes.

Assigned branch/worktree: unit/parity-orders, parity-orders. No delegation. Runtime identifies GPT-6 family; exact model/effort unavailable. Humaux tools absent: no search/store/canvas/coord claim. This file and PR receipts are the available durable trail, not a Humaux substitute claim.

## Base and RED

Before change, merged origin/r3/integration at 2f61bf86673b3ebf4acbd3ee0ecacd54d37cf46f. Base Gates 41078151280 reports success (read through gh run view); no class (c) claimed.
Pre-merge branch HEAD: 0a8d2dc8962021a119fd57992bddf59d3b2ae1ff. Merge commit is the rollback point for this fix batch.
Directed RED: run 41077800398, native head 0a8d2dc8962021a119fd57992bddf59d3b2ae1ff; tested synthetic merge f4e7b63b08b57d26188ed4e0a0518b66b5fd84a7. Filtered raw failures appended to red.log. Original tests-only causal RED remains recorded there; no extra RED push for this directed continuation.

## Every failing job → test → evidence → class

Paths below are under output/parity-orders/fix1-evidence; downloads retained locally, original GitHub artifact IDs are durable pointers.

| Failing job | Failing test / member | Original evidence / assertion | Class and directed change |
|---|---|---|---|
| 124145468743 click-sweep 8/10 | TestBrowserClickSweep, J2/J3 | Artifact 12990878418, click-sweep.mjs.log: missing orders-table; next step has undefined detail; buyer expects COLLECTED, gets PENDING after upstream chain stopped | (a) signed cards replace table/inline route. Use orders-list, click View then saved-facts summary, scope commands to order-facts. Refresh asserts same standalone order and persisted collection. Same six-page discovery bound and business assertions. |
| 124145468443 bundle:05 | TestBrowserE2EDealLoop / --browser-e2e | Artifact 12990845908, e2e/**/playwright.log: deal-loop.spec.ts:292 orders-table absent | (b) chain's orders setup helper was not migrated. Fix expandOrder once to open/identify standalone detail and reveal saved facts; list arrival uses cards, refresh retains detail. Financial/stock/shipment/refund/privacy assertions and existing retry/time budgets retained. |
| 124145468443 bundle:05 | TestBrowserManualFulfilment / --browser-refund-fulfilment, MF07 CVS snapshot | Artifact 12991100484, manual-fulfilment/**/playwright.log: manual-fulfilment.spec.ts:308 orders-list absent after detail reload | (a) assert same standalone order after reload and retain every frozen pickup/store/tracking/shipped fact assertion. No fixture or command body change. |
| 124145468443 bundle:05 | TestBrowserPromotions / --browser-promotions | Artifact 12989434229, promotions/**/browser.log: promotions-gate.mjs:287 transfer-confirm hidden inside saved facts | (b) orders setup failed to reveal native disclosure. Click it once before the unchanged discount/total/shipping/transfer/finance/usage chain. |
| 124145468087 bundle:08 | TestBrowserOC / --browser-password-auth, OC-04 orders in all 3 locales | Artifact 12990471770, oc-*/playwright.log: reauth.spec.ts:157 orders-export hidden | (a) signed advanced filters now contain export. Real-click the existing disclosure first. Expired guard, exact replay, download and all locale checks retained. PasswordAuth/staff checks themselves passed; OC causes bundle failure. |
| 124145468914 bundle:09 | TestBrowserMerchantOrdersUIRealChain / --browser-merchant-orders-ui | bundle09.log: browser_merchant_orders_ui_test.go:356 only reports exit 1. Upload step rejects parity/1429:25977.png (colon forbidden), dropping all 339 evidence files; no gate-browser-merchant-orders-ui artifact exists in run inventory | (b) proven own-PR evidence-production defect: normalize frame ID to 1429-25977.png on disk, retain original frame ID in attachment label. Internal browser assertion UNKNOWN because CI did not publish its log; no invented assertion or speculative test rewrite. Next CI must expose the exact failures if still red. |
| 124145468914 bundle:09 | --browser-picklist, W3-U1b | job log picklist.spec.ts:109: pick-toolbar resolves but hidden; artifact 12990548451 | (a) click signed advanced tools on arrival/reload; select first signed-card checkbox rather than removed legacy accessible-name prefix. All selection counts, 500/100 limits, CSV bytes, permissions, UNKNOWN/recovery/no-resend checks retained. |
| 124172159378 receipts | planned-mode PASS receipts | Raw receipt log lists seven failed modes above | Derived failures, no independent receipt marker problem; picklist already prints required browser-spec marker. |
| 124172159466 aggregate | click-sweep coverage | Raw aggregate log repeats J2/J3 upstream failures | Derived; no coverage/threshold/exemption changes. |
| 124177559412 required | required gate aggregation | Upstream jobs/receipts/aggregate red | Derived; no status injection or gate modification. |

## Verification and stop conditions

One batched directed FIX1 push only. Existing relays fixed different blockers; this batch is the first targeted fix for the assertions listed above. If the same assertion remains red, follow the card's stop instruction: BLOCKED, exact current assertion + run/job/artifact, no third approach.

MOU assertion-level diagnosis remains UNKNOWN, not PASS: original log is unavailable because this PR's colon filenames prevented upload. Filename repair addresses the proven defect and allows next CI to provide the missing evidence. Independent parity/authority/security acceptance remains NOT_RUN. No class (c) or base-equivalent failure claimed.

Static commands and exits are in fix1-static.log. Node syntax checks passed on all seven changed sources. Extended standalone tsc exited 1 on JS-bound picklist helper/fixture types; stopped after two configuration attempts, no strictness reduction or fixture changes. Type acceptance awaits CI. Generated ownership regenerated and checked; depmap regenerated via script, unchanged. All verification tests remain CI-only. No product/backend/migration/contract/OpenAPI/lockfile/R2-pin/baseline/dispatcher change in this batch.

Outside original card source paths for integrator approval: tests/admin/{manual-fulfilment.spec.ts,oc/reauth.spec.ts,picklist.spec.ts}, tests/e2e/deal-loop.spec.ts, tests/storefront/promotions-gate.mjs, tests/ui/click-sweep.mjs; generated output/ci-select-backend-browser/browser-ownership.json; task records output/parity-orders/{FIX1.md,red.log,fix1-static.log,DELIVERY.md}. tests/admin/orders-ui.spec.ts is existing card coverage. Integration's upstream storefront files arrive only through the required merge, not authored here.

Wakeup: integrator relays exact-head Gates/Foundation conclusions to a fresh session; pending >3h escalates to integrator. No silent background waits, local server, merge/auto-merge/force-push/status posting/deployment. Workflow IDs will be placed in a PR comment after push, without an evidence-only commit.
