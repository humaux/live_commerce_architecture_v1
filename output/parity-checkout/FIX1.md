# PARITY-CHECKOUT-FIX1 — directed repair, CI pending

Authority: owner FIX1 dispatch; AGENTS.md; buyer-checkout-v1 and buyer-checkout-options-v1 contracts; signed checkout controls polish referenced by this PR. Source of failures: Gates [41063668416](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41063668416), head `44a9e2eb3e456fd2ade2abab52950f14f396cf4a`, synthetic merge `b10a8cdda646783a5d30f7c856baa49d9767e975`. These are regression receipts, not a new tests-only RED.

Merged `origin/r3/integration` first: base `3ea359f796d36153bb4b8aa7259e4e0c27676b86`, merge `b5905fd76d95214ae50fd218b80f6a338c44419e`. Integrator supplied base Gates 41050344892 GREEN; no failure is classified (c) without an identical base assertion. Merge carries only deploy/docker/storefront.Dockerfile and deploy/scripts/smoke.sh; no lane edits to either.

## Failure inventory (before implementation)

`fix1-ci-raw.log` is the downloaded filtered-job log (untracked); artifact-relative paths below are rooted at `fix1-artifacts/`. Native GitHub artifacts remain the durable originals. Classification (a) means preserve the test's behavior/coverage and follow signed controls through real clicks; (b) requires product repair; (c) requires matching base evidence.

| Failing job | Failing test / assertion | Native evidence | Class and directed action |
|---|---|---|---|
| 124101535870 click-sweep 4/10 | TestBrowserClickSweep; create-order click waits on hidden button, then 15 route coverage rows fail | gate-browser-click-sweep_4_10/.../click-sweep.mjs.log:28 onward; ledger.md r00070; screenshots/r00070.png | (a) place COD with signed fixed checkout-submit; keep route/coverage assertions |
| 124101534147 click-sweep 7/10 | TestBrowserClickSweep; same shared COD setup | gate-browser-click-sweep_7_10/.../click-sweep.mjs.log; native raw log:16144-16149 | (a) same shared setup correction |
| 124101534039 click-sweep 8/10 | TestBrowserClickSweep; same setup prevents J23 order lookup | gate-browser-click-sweep_8_10/.../click-sweep.mjs.log; raw log:16123-16124 | (a) same correction; retain J23 |
| 124101535293 visual-lint 1/4 | TestBrowserClickSweep; two enumerated order shots missing | gate-browser-visual-lint_1_4/.../lint.json: shots.missing; blockingInstances=0 | (a) its shared order setup also uses hidden create-order; retain all shot/threshold requirements |
| 124101534117 visual-lint 2/4 | TestBrowserClickSweep / missing order screenshot setup | gate-browser-visual-lint_2_4/run.xlkKKIgz/.../lint.json: TW/en mobile order shots missing; blockingInstances=0 | (a) same signed fixed-action setup correction |
| 124101533993 visual-lint 3/4 | TestBrowserClickSweep / missing order screenshot setup | gate-browser-visual-lint_3_4/run.G4nNizJ7/.../lint.json: CN desktop order shot missing; blockingInstances=0 | (a) same signed fixed-action setup correction |
| 124101534464 visual-lint 4/4 | TestBrowserClickSweep / missing order screenshot setup | gate-browser-visual-lint_4_4/run.yUIDhmR1/.../lint.json: CN mobile order shot missing; blockingInstances=0 | (a) same signed fixed-action setup correction |
| 124101535467 bundle:02 | TestBrowserBuyerOrderUI; BH05 waits for history response after locator loss | gate-browser-order/playwright/buyer-order-1839257567/browser.log:30-36; order-gate.mjs:782 | (a) open signed details before history on restored checkout; retain original assertions verbatim/in order |
| 124101535467 bundle:02 | TestBrowserStorefront; .quotation expected visible, received hidden | gate-browser-storefront/.../shop-real-gate.log:10-26 | (a) click View details before both existing free-shipping quotation assertions |
| 124101534256 bundle:04 | TestBrowserClaimDirectCheckout/RealClick; back-to-cart exact copy removed | gate-browser-claim-checkout/.../browser.log:7-9; ledger back-to-cart | (a) click signed header Back link; same route and merged-cart assertions |
| 124101536095 bundle:05 | TestBrowserE2EDealLoop; expected rgb(36,121,101), received rgb(20,41,66) | gate-browser-e2e/.../e2e/.../playwright.log:14-35; test-failed-1.png | (a) exact signed navy plus fixed action; preserve disabled/opacity assertions; (b) carry existing disabled reason to fixed action |
| 124101536095 bundle:05 | TestBrowserLiveTools; selectOption label still expects chain · TW | gate-browser-e2e/.../live-tools/.../playwright.log:12-38 (four variants) | (a) signed chain label, details disclosure, fixed action; same live-price/order assertions |
| 124101536240 bundle:06 | TestBrowserHomeCod; six locale/viewport re-quote cases cannot fill hidden recipient | gate-browser-home-cod/.../parity-checkout-click-ledger.json: six FAIL rows; home-cod-buyer.mjs.log:16-145 | (a) explicitly open details on replacement quote before editing; keep amount/reload/reconfirmation assertions |
| 124101534726 bundle:07 | TestBrowserTaiwanCvs/MOCK and /WebKit; inputValue #delivery after quote unmounted selector | gate-browser-cvs/.../cvs-buyer.mjs.log:12-19 (both timestamps); cvs-picker.mjs:107 | (a) capture actual selected UI key before requesting quote and pass to picker checks; preserve service-code assertion |
| 124101534103 WebKit | order, CVS, claim-checkout subgates | gate-browser-webkit/playwright/buyer-order-3979241523/browser.log:32-34; run.NYSv0CCd/{taiwan-cvs,claim-checkout}/.../{cvs-buyer.mjs.log,browser.log} | (a) identical history disclosure / removed delivery selector / Back copy assertions; same fixes as Chromium |
| 124127221669 receipts | failed mode receipts | raw log:16022-16035 | consequential; no receipt changes |
| 124127221730 aggregate | missing click coverage / visual shots | raw log:16119-16154 and visual aggregate | consequential; no exemption/threshold changes |
| 124135033068 required | gate / aggregate / receipts failure | raw log:16195 | consequential; no required-check changes |

## Verification and handoff

One batched FIX1 push only. Existing causal tests-only RED: head `2318e7c9beda70cb52872703cc1ea16988df9668`, Gates 41051681920 / job 124052345726; assertions and receipt retained in red.log. No additional RED push for this directed repair.

Acceptance gate: this PR's exact new-head Foundation and Gates including browser suites above, followed by integrator independent review. Author assessment E0; local syntax/structure can provide E1 only. If the same assertion remains red after FIX1, STOP with BLOCKED and exact assertion; do not take a third approach.

Humaux search/coord/store/canvas and code-index tools unavailable in this session. No shared state claimed or overwritten. Source inspection used rg plus narrow reads. No local runtime tests/server, production, merge-to-integration or auto-merge. Downloaded CI artifacts are synthetic and retained as failure evidence.


## Applied fix and unchanged gates

- Sweep and visual-audit order setup now click the signed fixed `checkout-submit`; their complete route/journey/shot matrices and thresholds remain.
- Deal loop asserts the exact signed navy `rgb(20, 41, 66)` on that real action, retaining disabled grey, accessible-description and opacity checks. Product root repair carries `needsConfirm` to the fixed action's `aria-describedby`, reusing the original localized visible hint. Three-locale/two-viewport disabled checks now also require the description. This accessibility finding was static; its new runtime assertion is pending CI.
- Optional quotation/history/address editing uses real View details clicks. Original gate-adapter assertions remain verbatim and in order; only setup actions/import were added to order-gate and shop-real-gate.
- CVS picker consumes the actual UI-selected delivery key read before quote navigation, preserving the exact server service-code assertion. No fixture, price, eligibility, contract, migration, exemption or retry budget changed.
- Live-tools still checks claimed/catalog prices and the persisted orders; its delivery label follows the signed chain name without country suffix. Claim merged-cart removal still clicks Back and proves the same cart route and SKU quantities.

## Static receipts (E1 only)

`node --check` on all 10 changed MJS/TS browser sources: exit 0. `tsc --noEmit --incremental false -p apps/storefront/tsconfig.fix1.static.json`: exit 0, using this worktree's i18n source and read-only installed dependencies. Temporary config/dependency links removed. Browser ownership generation and `--check`: exit 0. `python3 scripts/check_packet.py`: exit 0, packet structure only; its timestamp output restored to avoid unrelated diff. `git diff --check`: exit 0. File bindings: source-hashes.json. No Go edges changed, so dependency-map generation is unnecessary.

No component imports added or renamed. Header comments retained. Existing BO03 VM helper harness inspected; its source slice and injected bindings are unaffected. New test imports use existing openCheckoutDetails; no harness compiles these TSX components.

## Scope and pending acceptance

Outside the original explicit checkout-source paths (integrator review): `apps/storefront/components/OrderFlow.tsx` (same-app pre-approved root accessibility fix); `tests/e2e/{deal-loop,live-tools}.spec.ts`, `tests/ui/{click-sweep,visual-audit}.mjs`, `tests/storefront/{claim-checkout,cvs-buyer,cvs-picker,order-gate,shop-real-gate}.mjs` (direct buyer checkout callers, explicitly directed test rewrites/additions); generated browser-ownership.json; card evidence FIX1.md / fix1-failure-evidence.log / DELIVERY.md / red.log / source-hashes.json. `tests/storefront/checkout-v2.mjs` remains the original card's parity test. Integration deployment files are inherited, not edited by this fix.

NOT_RUN: exact FIX1-head Foundation/Gates GREEN (pending push), independent acceptance/E4, local runtime tests/servers (prohibited), provider/production execution, remaining 32 signed CVS/pickup visual comparisons, Humaux store/canvas/coord/code-index. No PASS/completion claimed. CI wake-up owner: integrator, using the exact WAIT_RUN IDs in the final PR comment. No additional fixes if the same assertion remains RED.

Evidence retrieval note: a repeated multi-artifact download stopped on an already-existing WebKit bo01-head-reads.json; no overwrite/retry was used. All listed failure logs, screenshot and four lint summaries were present and read; their native hashes are retained in fix1-failure-evidence.log. The ownership check detected drift after the final assertion addition; regeneration and final --check then exited 0.
