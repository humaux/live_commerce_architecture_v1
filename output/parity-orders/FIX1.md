# PARITY-ORDERS-FIX1 — directed CI repair

Status: BLOCKED on current head 932937e7aead8773880a977e6b2a47a717cb84ac (2026-10-31 relay). CI failures observed (E3); no product completion or GREEN claimed. No local tests/servers/browser/PG/containers run.

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

## Current relay — BLOCKED, no further implementation push

Fetched origin/r3/integration and ran `git merge --no-edit origin/r3/integration`: exit 0, already up to date at 2f61bf86673b3ebf4acbd3ee0ecacd54d37cf46f. HEAD remains 932937e7aead8773880a977e6b2a47a717cb84ac; tracked work was clean before this evidence update. No source, test, generated ownership or dependency-map changes in this relay.

Current Gates run 41090484680, attempt 1, native head 932937e7aead8773880a977e6b2a47a717cb84ac, tested synthetic merge ab50d04f2c36fcc6ee6f817d9699b80147002ee1. `gh run view --job ... --log-failed` reports the overall run is still in progress; completed-job raw logs were retrieved via `gh api repos/humaux/live_commerce_architecture_v1/actions/jobs/<id>/logs`, and browser artifacts via `gh run download`. This is completed-job evidence, not an overall run verdict.

| Current failing job | Test / exact assertion | Evidence | Class / stop reason |
|---|---|---|---|
| 124185561277, gate (--browser-webkit) | TestBrowserTaiwanCvs/WebKit, buyer half; tests/storefront/cvs-picker.mjs:387 `expect(choose(code)).toBeVisible()`; cvs-candidate-024793 not found, 5000ms | Artifact 12994344180 (`gate-browser-webkit`), playwright/run.mHtc0HL1/taiwan-cvs/20261030T164608.282762295/cvs-buyer.mjs.log:17-32; Go wrapper browser_taiwan_cvs_test.go:159 | UNKNOWN causal class; out-of-module storefront blocker. Merchant orders half never starts. STOP under cross-app scope boundary; no storefront changes. |
| 124185561381, gate (bundle:08) | TestBrowserOC; OC-04 orders in zh-TW/zh-CN/en, tests/admin/oc/reauth.spec.ts:188 after reload: `expect(control).toBeVisible()`; orders-export hidden, 10000ms | Artifact 12995610677 (`gate-browser-password-auth`), playwright/run.eCuZlMLq/oc-20261030T164550.766850856/playwright.log:15-100 | (a) existing export visibility intent still fails after FIX1's disclosure migration; original pre-refresh assertion now reaches refresh, but the same OC-04 test remains RED. Card explicitly requires BLOCKED if the same test is still red. No third approach, timeout/fixture/assertion change. |

The storefront picker, buyer harness and apps/storefront have zero diff against current integration. Picker blob is 24f495ebc04aa9bde1cd134fe6f2289d65169412 on both refs. This is source identity evidence, NOT proof that base fails identically. Base Gates 41078151280 is successful but its job inventory contains no WebKit/browser bundle job (build skipped); therefore no class (c) claim is made, and base GREEN does not establish this browser assertion passed. Root cause of the storefront failure remains UNKNOWN; access-control page errors in its log alone do not prove causality.

Foundation 41090484506 / job 124182766494 is successful at this exact native head (E1 scope). Gates GREEN is absent. Other FIX1 job outcomes and independent review remain pending, not inferred from these two failures. Original tests-only RED run/job pointers above remain valid historical causal evidence.

WAIT_RUN: 41090484680 (existing Gates run; failed jobs above, remaining jobs pending at observation). Integrator owns completion/failure/cancellation wakeup and scope/stop adjudication; no new CI run, rerun, implementation attempt or evidence-only push started. Durable trail is this file, red.log, DELIVERY.md and the PR comment; Humaux memory/store/canvas/coord interfaces remain unavailable, so no shared-state completion claim. NOT_RUN: local tests/servers/browser/PG/containers (prohibited), independent acceptance, production. Author acceptance remains E0.

### RELAY 2 recheck — 2026-10-31 (2026-10-30T16:56:58Z)

Fresh GitHub reads confirm native PR/run head remains 932937e7aead8773880a977e6b2a47a717cb84ac. Integration fetch/merge exits 0, already up to date. Completed-job API logs freshly saved to fix1-relay-verified-bundle08.log: lines 2216 and 2735 confirm TestBrowserOC and browser-password-auth failure; the downloaded artifact's playwright.log:38 binds the hidden-export assertion to reauth.spec.ts:188. Job APIs confirm WebKit124185561277 failure and Foundation124182766494 success on this same head. Gates41090484680 still has queued/running jobs; no overall GREEN.

The existing remote receipt https://github.com/humaux/live_commerce_architecture_v1/pull/362#issuecomment-6528394233 already records these exact blockers. No duplicate comment, new implementation attempt, ownership regeneration, commit, push or CI rerun. Stop remains required by FIX1's same-test-still-red clause. This recheck adds local evidence only; original dirty files preserved. Humaux unavailable; no memory/store/canvas/coord action claimed. Integrator owns the pending-run wakeup and stop/scope adjudication.

### RELAY 3 fresh readback — 2026-10-31

Native PR/run head is still `932937e7aead8773880a977e6b2a47a717cb84ac`; fetch and merge integration again exited 0, already up to date. `gh run view --job 124185561381 --log-failed` remains unavailable while the workflow is unfinished; completed-job API log freshly confirms `TestBrowserOC` failure at line2216 and `--browser-password-auth` failure at line2735. Fresh download of artifact12995610677 is retained under `fix1-relay3-password-proof/`; `playwright/run.eCuZlMLq/oc-20261030T164550.766850856/playwright.log:15-100` confirms all three locale failures at `reauth.spec.ts:188`, export expected visible but hidden after reload. This establishes the same-test-still-red stop condition using original CI evidence (E3 failure evidence, not completion).

Current source at lines156/187-188 opens the disclosure only before the first export; refresh collapses it before the retained visibility assertion. Class(a) test migration remains incomplete. No third approach is authorized by this FIX1 card. WebKit job124185561277 is also confirmed failure; its cross-app causal class remains UNKNOWN. Foundation41090484506/job124182766494 is successful, not overall Gates acceptance.

Humaux discovery now succeeds (earlier unavailable-tool statements are historical). Directed search/grep, rejected subset (3/9, not exhaustive), coord read and canvas read were performed; no new claim or delegation for this integrator-dispatched task. Shared-memory write is guarded by an acquired coord lock; only this relay's own sub-canvas will be updated. No source/test/generated changes, new push, CI rerun or duplicate PR comment. Existing remote blocker comment remains authoritative evidence pointer. WAIT_RUN41090484680 remains with integrator for remaining-job completion and stop/scope adjudication. Local tests/servers and independent acceptance remain NOT_RUN; author acceptance E0.
