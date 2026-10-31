# PARITY-CHECKOUT-FIX1 — BLOCKED after same-test RED

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

## FIX1 RELAY 1 — integration conflict repair (2026-10-30)

Native `gh pr checks 351` confirmed no checks on `de70834327901121f9f778ef5605c18b19f87cb9`; the PR reported CONFLICTING. This is a merge blocker, not assertion RED. The integrator explicitly dispatched this relay to merge the base first. Rollback point: that pre-merge head.

Merged integration `2f61bf86673b3ebf4acbd3ee0ecacd54d37cf46f`. Two actual conflicts: `tests/storefront/cvs-picker.mjs` and generated `output/ci-select-backend-browser/browser-ownership.json`. The source resolution keeps both the checkout-returned `deliveryKey` (read before the selector unmounts) and integration's `exerciseSignedCvsParity` real-click checks. All CVS review/selection assertions from integration survive. Ownership was regenerated, never hand-merged. Other files in the merge are inherited integration changes, not lane edits. No additional product fix or assertion-repair attempt in this relay.

E1 receipts: `node scripts/dev/derive-browser-ownership.mjs`, the same command with `--check`, `node --check tests/storefront/cvs-picker.mjs`, `node --check tests/storefront/cvs-buyer.mjs`, and `git diff --check`: exit 0. No local tests or servers. Final SHA/run IDs go in the PR handoff comment; no evidence-only push after GREEN.

Humaux tools are available in this relay: fix stored under title `[fix] PARITY-CHECKOUT FIX1 relay1 resolve integration CVS helper conflict`, changed helper submitted to code-index, and own relay canvas updated under an acquired lock. Earlier tool-unavailability statements describe their historical sessions. CI acceptance and independent review remain NOT_RUN/PENDING. Integrator relays the exact-head Gates/Foundation conclusions; same-test RED after FIX1 means BLOCKED, no third approach.

## FIX1 RELAY 2 — BLOCKED (2026-10-31)

Current PR head is `7351f892764b0b61b5c9088239ba34dc72adfa7c`; fetched and merged `origin/r3/integration` (`2f61bf86673b3ebf4acbd3ee0ecacd54d37cf46f`) first: already up to date. No product/test edits or new push in this relay. The card explicitly says: "If the same test is still red after this fix ... write BLOCKED with the exact assertion and evidence." The same six Home COD re-quote tests remain RED after FIX1; stop condition applies.

Native current-head Gates run `41082084811`, synthetic merge `b6c2c424948228ee2c45d6c4fe737bb8023beb7d`. Its aggregate is still queued/in progress; completed job logs were obtained through the GitHub jobs/logs API because `gh run view --log-failed --job` refuses an unfinished run. Local copies are under `fix1-relay2-artifacts/`; native artifacts are the durable evidence.

| Current failing job | Failing test / exact assertion | Native evidence | Classification / disposition |
|---|---|---|---|
| `124156997127` bundle:06 / --browser-home-cod | `TestBrowserHomeCod`; `AssertionError [ERR_ASSERTION]: CHECKOUT-V2 new behaviour assertions`; all six zh-TW/zh-CN/en × 1440/390 card re-quote cases: `locator.fill: Timeout 30000ms exceeded`, waiting for `input[name="recipient_name"]`, `element is not enabled` | Artifact `12992431420` (gate-browser-home-cod); `playwright/run.kQZGgKGK/home-cod/20261030T160005.186853690/home-cod-buyer.mjs.log:8-30`; `parity-checkout-click-ledger.json:75-80` (first case; other cases at 267/459/555/651/747) | BLOCKED, same re-quote test/selector as the original FIX1 inventory. Details disclosure alone did not make the field editable. Cannot classify this as an intentional signed-frame control replacement or a base failure; no gate/fixture/timeout change. |
| `124156996798` --browser-webkit | `TestBrowserBuyerOrderUI`; `locator.click: Timeout 30000ms exceeded`, waiting for `getByTestId('toggle-order-history')`, `element is not visible`, at `tests/storefront/order-gate.mjs:830:53` | Artifact `12991143455` (gate-browser-webkit); `playwright/buyer-order-2250117649/browser.log:36-54`; `failure-12.txt:47` still shows View details; native job log `fix1-current-webkit.log:2146-2154` | (a) another history caller still bypasses signed disclosure; BLOCKED per same-test stop rule. Earlier BH05 repair now passes (`browser.log:28-29`), but a later history click fails at line 830. Other buyer/payment/merchant-buyer/CVS/password-auth/claim-checkout subgates pass. |

Current-head Foundation `41082084746`, job `124153541124`: success (E3 for Foundation only). Whole PR GREEN is absent. Historical tests-only RED remains `41051681920` / `124052345726`; these current failures are regression receipts, not a new tests-only RED.

Static handoff clue (E0 diagnosis, not proven root cause): `checkout-v2.mjs:544-545` expands details and immediately fills; `OrderFlow.tsx:392` disables the address fieldset while busy/blocked/head loading/options unavailable/expired. Preserve those guards and the changed-total acknowledgment behavior; any further repair needs integrator disposition of the explicit stop condition. No local tests or servers ran.

Humaux search/store/canvas/coord tools are absent in this relay (tool discovery returned none); no shared state was changed. This local report and the PR handoff comment preserve evidence without an evidence-only CI push. Outside-card source files touched this relay: none. NOT_RUN: whole-PR GREEN, independent review/E4, local runtime tests/servers (prohibited), providers/production. WAIT_RUN: Gates `41082084811`; integrator owns the completion relay. No continuing CI polling, merge-to-integration, auto-merge or deployment.

## FIX1 RELAY 3 — stop condition reconfirmed (2026-10-31)

Fetched and merged `origin/r3/integration` first: already up to date at `2f61bf86673b3ebf4acbd3ee0ecacd54d37cf46f`. PR API still binds head `7351f892764b0b61b5c9088239ba34dc72adfa7c` to base `r3/integration`. No source/test change, commit or push this relay.

Native jobs API confirms `124156997127` (bundle:06) completed with failure at `2026-10-30T16:05:16Z`, and `124156996798` (WebKit) completed with failure at `2026-10-30T16:04:04Z`, both in Gates `41082084811`. `gh run view --job 124156997127 --log-failed` still refuses the unfinished aggregate; direct jobs/logs API was used without rerunning CI. The run API currently reports queued with null conclusion; no whole-run failure/success is invented.

The relay2 table above remains current: Home COD artifact ledger lines 79/271/463/559/655/751 each report `locator.fill: Timeout 30000ms exceeded` on `input[name="recipient_name"]`, ultimately `element is not enabled`; buyer log line 8 is `AssertionError [ERR_ASSERTION]: CHECKOUT-V2 new behaviour assertions`. WebKit buyer-order log lines 36–54 report hidden `toggle-order-history`, at `order-gate.mjs:830:53`; native WebKit log lines 2147–2148 confirm order subgate failure. This is repeated regression RED after FIX1, not the tests-only phase. **BLOCKED under the explicit card stop condition; no third approach.** Home COD root cause remains unproven; no classification (c) is claimed.

Foundation run `41082084746` completed success on this exact PR head (job `124153541124`): E3 for Foundation only. Historical causal tests-only RED remains run `41051681920`, job `124052345726`. Whole-PR GREEN and independent E4 remain NOT_RUN. No local runtime test/server or new static test run. Existing evidence and uncommitted work preserved.

Humaux tools became available after initial discovery; this relay records its blocked evidence in Humaux under a shared-state lock and updates only its own `integrator-sub-parity-checkout-fix1-relay3` canvas element. No separate coord claim: dispatched-lane exemption. WAIT_RUN `41082084811` is the existing Gates workflow, not a plan job or a new push. Integrator owns the remaining completion relay and stop-condition disposition. No source paths outside the card changed; this report is intentionally uncommitted.

## FIX1 RELAY 4 — BLOCKED, no third approach (2026-10-31)

Re-read AGENTS.md and 架构.md §§0/1/2/26/27. Native PR API confirms head `7351f892764b0b61b5c9088239ba34dc72adfa7c`, base `r3/integration`. Fetched integration and merged first: already up to date at `2f61bf86673b3ebf4acbd3ee0ecacd54d37cf46f`. Existing uncommitted evidence was preserved. No source/test edits, commit or push.

`gh pr checks 351` and native jobs API confirm Gates `41082084811` job `124156997127` bundle:06 FAILURE and job `124156996798` WebKit FAILURE. Requested `gh run view --job 124156997127 --log-failed` refuses the unfinished run; direct jobs/logs API confirms `--- FAIL: TestBrowserHomeCod` at native lines 1992 and 2146. Current run API remains queued/null conclusion; this does not override completed job failures.

| Job | Exact remaining assertion / evidence | Disposition |
|---|---|---|
| 124156997127 | Six card re-quote cases; `CHECKOUT-V2 new behaviour assertions`; fill `input[name="recipient_name"]` times out because `element is not enabled`. Existing native artifact 12992431420, home-cod-buyer.mjs.log:8–31 and ledger lines 79/271/463/559/655/751. | Repeated RED after FIX1; BLOCKED under explicit card stop condition. Root cause remains unproven; preserve guards and assertions. |
| 124156996798 | `TestBrowserBuyerOrderUI`, hidden `toggle-order-history`; artifact 12991143455 buyer-order-2250117649/browser.log:36–54, order-gate.mjs:830:53. | Previously reported same-suite RED; no third repair or assertion weakening. |

Foundation `41082084746` job `124153541124` passes on this head (E3 Foundation only). Original tests-only RED `41051681920` / `124052345726` remains historical; the current failures are regression receipts. GREEN for the whole PR absent; author acceptance E0. WAIT_RUN `41082084811` is the existing Gates workflow, not a newly triggered run. Integrator owns the completion relay and stop-condition disposition. NOT_RUN: local tests/servers (prohibited), whole-PR GREEN, independent E4, provider/production. This relay touches only this local report; no outside-card source paths. Humaux problem stored and own relay4 canvas updated under acquired shared-state lock. No processes started or new temporary artifacts created; existing failure evidence retained.
