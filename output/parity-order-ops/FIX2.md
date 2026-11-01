# PARITY-ORDER-OPS-FIX2 — PR #364, CI pending

Authority: current human FIX2 card; AGENTS.md; 架构.md §§0/1/2/26/27;
contracts/invariants.json, storefront-v2.md G3, returns-v1.md and merchant-orders-bff-v1.md.
UI text/format authority: fresh Fleet Bridge get_metadata readbacks below, not historical output snapshots.

## Checkout / authorization / evidence

- Assigned worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-order-ops`; branch `unit/parity-order-ops`; target `r3/integration`.
- Entry HEAD `b203046a7d6d4edbf0fcb5025bed628af40cb0ab`. Fetched and merged integration `da2cc7a3fc1e6eac11de727b158412ae9aff8ba9` first; merge `b37796f16`.
- Merge conflicts preserved both CVS and shipping routes, all locale exports/titles, and the stricter catalog alert assertion. Browser ownership was regenerated, never hand-edited. No other worktree/main checkout was modified.
- Integrator-dispatched implementation lane; runtime model/effort not exposed. No recursive delegation. Humaux tools unavailable: library search, rejected enumeration, coord and canvas/store are NOT_RUN; no local/platform memory used as substitute.
- RED: run **41111264414**, attempt 1, PR head `b203046a7d6d4edbf0fcb5025bed628af40cb0ab`, tested merge `3e20036a3cbb6c264fa97d04539c6f72230eccb5`. Original tests-first RED provenance remains in `red.log`. This directed card requires one batched repair push, not another RED push.
- Base Gates **41266607808** at `da2cc7a3f` was still in progress when inspected. No identical-base exemption is asserted.
- Evidence levels: native original failure **E3 FAILURE**; local syntax/generation **E1**; implementation assessment **E0**. Current-head GREEN and independent acceptance remain PENDING.

## Every failed job → test → native evidence → class → directed fix

All native job ids below belong to run 41111264414. Class (a) = superseded UI expectation; (b) = regression caused by this PR; (c) = proven identical base failure.

|Failed job|Failing test / assertion|Evidence|Class and action|
|---|---|---|---|
|124259888171 `--browser-click-sweep@2/10`|TestBrowserClickSweep: `/orders/cvs-match` desktop/zh-TW, `7-ELEVEN`, `no visible change within 3 s`|gate-browser-click-sweep_2_10 (id13008315546), ledger row r00021; click-sweep.mjs.log final NEW FAIL|(b) Selected brand button was an enabled no-op. Disable the current choice and keep the other brand actionable; add assertions before/after the real brand switch. No sweep threshold or exemption changed.|
|124259888281 `--browser-click-sweep@3/10`|Same, mobile/zh-TW|gate-browser-click-sweep_3_10 (id13007057529), row r00045|(b) Same shared CvsLookup fix.|
|124259888520 `--browser-click-sweep@7/10`|Same, desktop/en|gate-browser-click-sweep_7_10 (id13007578317), row r00039|(b) Same shared CvsLookup fix.|
|124259889138 `bundle:01`, member `--browser-ui-gallery`|`the live tree holds the baseline (apps/admin hex colours did not increase)`: `total hex colours 402 > baseline 386`; order-ops.css `16 hex colours > baseline 0`|job log lines1670–1681; tests/ui/ds-hex-ratchet.test.mjs:87–88|(b) Replace the 16 new literals/fallbacks with existing DS color tokens, including token-derived backdrop. Baseline unchanged.|
|124259888066 `bundle:07`, member `--browser-manual-order`|TestBrowserManualOrderLink: timeout at manual-order-link-gate.mjs:142, `manual-order-submit` already disabled and labeled `Creating…` after Review|gate-browser-manual-order (id13007740346), browser.log:7–24|(b) React reused the clicked Review button as a submit button before its native click default, prematurely placing the order. Keep type=submit across both steps and use the existing preventDefault / reviewing guard. Add real-click Review-visible / zero-new-placement assertions for bank and COD; preserve every original gate assertion verbatim/in order.|
|124259888066 `bundle:07`, member `--browser-returns-ui`|TestBrowserReturnsUI: **18 failed / 14 passed**. All manual-order review, merchant CVS and manual CVS cases across zh-TW/zh-CN/en × desktop/mobile fail on exact address: expected `臺北市…`, received `台北市…`|gate-browser-returns-ui (id13005554670), playwright.log; parity-order-ops.spec.ts:158,217,270; received snapshot shows same store/code/street|(b) Shared admin directory reader maps address width via NFKC and county alias to the directory's formal county spelling. Lookup, copy, draft and review receive the same formatted address. Immutable source files, fixtures and all exact address assertions remain unchanged. Also covers signed FamilyMart `69號B1` vs source full-width digits.|
|124259888133 `shard:g08`|TestCvsNoIframeAndPrivacy / PROCESS_section_5: two new CVS files' headers name neither `/api/` nor `/v1/`|job log2694–2701; taiwan_cvs_guards_test.go:229|(b) Document the actual server session/store loader endpoint (`GET /v1/admin/stores`) and ManualOrder consumer BFF (`/api/stores/{store}/tools/orders/manual`); lookup itself reads only public assets. Guards unchanged.|
|124287916770 `aggregate`|G-UI8 click sweep aggregate FAIL: exactly 3 failing rows, shards2/3/7|aggregate job log311–317 names the same 7-ELEVEN no-effect rows|(b), consequence of the three sweep failures above; no aggregate change.|
|124287916825 `receipts`|`matrix-receipts: RED (7 problem(s))`: g08, gallery, sweep2/3/7, returns, manual|receipts log654–660 lists precisely those failed members|(b), cascade only. Both node/browser receipt markers are present; no standalone missing-marker failure. No receipts change.|
|124288056495 `required`|`gate=failure aggregate=failure receipts=failure`; exit1|required job's final condition receipt|(b), cascade only. No required-check/status change.|

### Reported FAIL line that is an intentional negative control

`124259888133` prints `--- FAIL: TestProductEditorMatrixLedgerEvidence` inside
`TestProductEditorMatrixLedgerEvidenceHistoricalPolicy/drift-strict` and
`.../tampered-matching`. Lines1294–1310 show expected child exit1 followed by
**PASS** for the parent and all four subtests. The child assertions are respectively
`canonical evidence is historical (source drift: tests/admin/product-editor.acceptance.ts)` and
`SHA-256 mismatch .../fixture/browser/playwright.log`. They prove the gate rejects drift/tampering;
they are not an independently failed suite. No test, fixture, hash or exemption was changed.

No class (a) rewrite or class (c) waiver was needed. Existing RMA lifecycle tests passed in the failing artifact.
Known inbox helper is untouched; no failures have been inferred from old output reports.

## Fresh signed-frame authority (2026-11-01)

Fleet CLI README read; figma_status showed one connected document and no uncertain write.
Read-only get_metadata, depth4, roots `1431:70774` and `1431:71469`; no use_figma or canvas writes.
The current visible text is `臺北市大安區敦化南路一段187巷29號` (建安,127620) and
`新北市泰山區貴子路69號B1` (全家泰山新輔醫店,024967). This is the reason to fix the
shared product formatting instead of loosening exact assertions or altering the public directory.

## Static checks / limits / handoff

- `node --check tests/storefront/manual-order-link-gate.mjs` → exit0.
- `node --check apps/admin/lib/cvs-directory-client.ts` → exit0.
- `node --check tests/admin/parity-order-ops.spec.ts` → exit0.
- `bash -n scripts/dev/depmap.sh` and `bash scripts/dev/depmap.sh` → exit0; generated Go dependency map has no diff.
- `node scripts/dev/derive-browser-ownership.mjs`, then `--check` → exit0 after integration merge and changes; deterministic 55 modes.
- Evidence retention checks committed HEAD, so it is rerun after the final commit and before push; the exact receipt goes in the PR comment.
- No local tests/server/browser/PG/container/build. Full tsc NOT_RUN: compiler/workspace node_modules unavailable in this worktree. CI owns type/build/browser/gate verification.
- New head/run ids go in a PR comment after the single push; never an evidence-only follow-up commit. Integrator relays the completed Gates/Foundation run in a fresh session. No >10-minute polling.
- If these same tests fail again, mark BLOCKED with exact assertion/artifact; no third directed approach.
- No production action, migrations/R2 pin, contract/OpenAPI/lockfile, route-dispatcher or contractdrift baseline changes. next-startup parity guard unchanged.
- Rollback: revert FIX2 implementation commit. Integration merge is separate; no database migration or data write.
- Existing untracked lane evidence left untouched. Downloaded Actions artifacts/job logs/Figma captures are temporary evidence outside tracked output; only this compact report/hashes are committed.

## Additional scope visibility

FIX2 changes admin CVS route/component/client/CSS and ManualOrder plus its explicit card tests.
Additional non-app files: `tests/admin/parity-order-ops.spec.ts` (additive assertions),
`tests/storefront/manual-order-link-gate.mjs` (additive merchant setup assertions only),
generated `output/ci-select-backend-browser/browser-ownership.json`, and
`output/parity-order-ops/{FIX2.md,DELIVERY.md,red.log}`. Integration conflict resolution also
preserved `packages/i18n/src/index.ts`, shell-copy/routes and catalog-v2-simplified.acceptance.ts;
all conflicts combined incoming base with existing lane changes. No storefront product source changed.

## Native evidence SHA-256 manifest

Hashes identify downloaded job-log bytes and extracted evidence-log bytes, not a CI PASS or a rehosted raw artifact.

|Run / job or artifact|Evidence file|SHA-256|
|---|---|---|
|41111264414 / 124259888066|native gh --log-failed bytes|`34837b1fa08f55697e148695ce1b11b04439ecdbe8bbfbcee73110fa52d2f825`|
|41111264414 / 124259888133|native gh --log-failed bytes|`b068b5ad740cabe0f370b86336045b1196968a928e2d09dde2973dc8398efbbe`|
|41111264414 / 124259888171|native gh --log-failed bytes|`8ca7465087720be9514d9a293618d680c037348a92d61374b678be4fc8030d6d`|
|41111264414 / 124259888281|native gh --log-failed bytes|`323d89637751f657887ea2acadc7fb3caec8494b74601b64e00dbccbebb84ac3`|
|41111264414 / 124259888520|native gh --log-failed bytes|`a4f7d6092ac489f9879f3bf250a8b8f86a686ca988ecdad5edfff775a6e841e0`|
|41111264414 / 124259889138|native gh --log-failed bytes|`117a6fcca80cd4e8809a556c55b8027b23d03b9e6a65c5f05f4af0accc67afea`|
|41111264414 / 124287916770|native gh --log-failed bytes|`96de03c787c82803517943807ecbbf7a2dbd0a9d50b6079ba1af44ac5f8c16c8`|
|41111264414 / 124287916825|native gh --log-failed bytes|`49e802aa37b9276d331e5a51909da9d831589e1339e59dc6ed818ef31ca57e8e`|
|41111264414 / 124288056495|native gh --log-failed bytes|`47be87235f837c5490cc9a00a2e5d7a6537ddbc8fcb134ff5b8b8c5feebb3fda`|
|41111264414 / gate-browser-click-sweep_2_10|click-sweep.mjs.log|`ec3232e191a89503f3be98901d194b5bc30669fa7b6b23e87046f4126999e834`|
|41111264414 / gate-browser-click-sweep_3_10|click-sweep.mjs.log|`d9dda011b60678dbf2fdd21484e8df96e9b83792a12e92a7500bd4ce5023e42e`|
|41111264414 / gate-browser-click-sweep_7_10|click-sweep.mjs.log|`5d6b73e485f4cb7b1b941db6d979175f18a740e4668d556c92f6024e37780ec5`|
|41111264414 / gate-browser-manual-order|browser.log|`1b30024783c97a89a446a857ca16aef8bb7d514117c1ebbac5591a6174b753f3`|
|41111264414 / gate-browser-returns-ui|playwright.log|`76bbd447d7c227dffa648d787351144dff1493ab2aa1262c8718c15ad3a3eb0c`|
|Figma 1431:70774, live read 2026-11-01|get_metadata depth4|`0fdd62070d27cf39eae463095b8011c55a90de7d63c3f2de3216691e53634695`|
|Figma 1431:71469, live read 2026-11-01|get_metadata depth4|`a47765b8f1811fcbc228e8bb059711156a046bec6e17a17b68f16ef488180520`|
