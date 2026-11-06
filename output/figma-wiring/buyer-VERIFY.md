# BUYER prototype wiring — current readback, independent acceptance pending

2026-10-29 · DESIGN · E1 structural readback only; author conclusions E0. No E2/E3/E4 completion claim.

Authority: current owner-dispatched LANE BUYER card and original `output/integrator/tools/cards/cxf-wire-buyer.md`; AGENTS.md; 架构.md §§0/1/2/26/27; Fleet Bridge README; figma-use reaction typings; output/ux-audit-ref/FIGMA-GUIDE.md. Assigned worktree/branch: `.worktrees/ux-cvs-match`, `design/ux-cvs-match`; base 68eb47046714790200e320129787ab51e77309ef. Main checkout read-only. No subagents. Runtime model/effort not exposed.

Humaux tools (search/grep/get/coord/store/canvas) unavailable. No shared claim, store or canvas write is claimed; integrator must persist this handoff. No local platform memory read or write in this run.

## Before / after — live Figma reads

|Measure|Before|After|
|---|---:|---:|
|All product frames|1189|1189|
|Entry frames|5|5|
|Global reached (shared file; other lanes may change)|125|387|
|Buyer Desktop reached|41/148|148/148|
|Buyer Mobile reached|44/151|151/151|
|Buyer unreached|214|0|
|CHANGE_TO|0|0|
|bad|0|0|
|Buyer cross-platform targets|0|0|

Fresh inventory contains 299 product FRAMEs plus two Shared cart COMPONENTs. Components were excluded from destination writes; no main component was modified. 126 source nodes across 74 Buyer roots were written using awaited setReactionsAsync, in batches of ≤20, with readback after every batch and get_metadata inspection. No visual/copy/frame deletion occurred. Selected node identity/layout/text/paint hashes match for all 299 Buyer product frames before/after (FNV summary is structural, not a pixel-perfect cryptographic visual proof).

The supplied whole-page audit scripts were read and their graph predicates aggregated from ≤10-root calls, respecting the bridge crash boundary. `audit-resume-before.json` and `audit-resume-after.json` contain results and per-root edges. Exact unbounded supplied invocations NOT_RUN. JS ASCII word-boundary semantics were retained for the entry predicate (5 entries). Merchant reach changes cannot be attributed exclusively to this lane in a shared file.

## Material limitation — shortcuts are explicit prototype branches

The plan contains 80 ON_CLICK links, 16 AFTER_TIMEOUT prototype resolutions, and 120 ON_KEY_DOWN outcome/variant links. Existing click behavior is preserved except obsolete checkout destinations, live-product claim entry, and listed picker/summary controls. No new state variable, conditional money logic or service call was created. Pending timers illustrate a possible server-observed result; they do not prove a real submission or payment. Unknown outcomes remain separate frames; no real retry is triggered.

All-action graph reach is 299/299 product frames. Click/timer-only graph reach is 163/301 inventory roots (the latter includes two components). Keyboard branches require a keyboard in the Figma player; this does NOT establish touch-only mobile reach or real user acceptance. A complete click/touch journey and the interpretation of alternate-state shortcuts require integrator review. No hidden keyboard path is presented as a passed click test.

## Rollback and observed failures

Rollback snapshot: `buyer/rollback-reactions.json`, full original reactions for all 126 sources, captured before the first mutation. Restore only these reactions, after checking no later writer has changed the source. Never replay an uncertain call.

Initial plain Python execution imported the old local inspect.py, unintentionally rerunning read-only inspection; subsequent scripts use python3 -I. Compact reads avoided the 64KiB response truncation. A preflight rejected two planned Shared cart COMPONENT targets before any mutation; those links were removed. First written readback differed only by Figma's duplicate legacy `action` and default resetVideoPosition:false; next difference was 1.2 seconds serialized as float32 1.2000000476837158. Expected and actual reactions were compared after removing only those serialization defaults and canonicalizing float32. Every trigger, destination and other action field was preserved. Already-written batches were inspected and verified; none was replayed. Raw receipts remain in evidence/.

## Gate evidence and NOT_RUN

- Fleet Bridge bounded before/after graph reads: exit 0; source write receipts and readbacks exit 0 after serialization inspection.
- Copy lint: all 74 changed roots exit 0, banned list unchanged. Full summaries in DELIVERY.md and copy-lint-resume.json.
- NO local tests or servers, app builds or product runtime changes. No source/header/contract/lockfile/migration/dependency-map/browser-ownership changes.
- CI tests-only RED/GREEN: NOT_RUN; Figma graph before/after is not GitHub CI. No CI pass claim. Real Figma click/touch/keyboard execution, all-control click ledger and persistence, browser E2E, density/three-locale/fact parity revalidation, independent review: NOT_RUN.
- Humaux store/canvas/coord: unavailable. No bridge/Figma restart. No own background process remains after commands finish. Old failed read evidence preserved in VERIFY-prior.md and prior files.

## Every written source → destination

Key codes are standard keyboard codes, listed per source. Outcome shortcuts are invoked while viewing the source frame in the Figma desktop player. All destinations below are different top-level Buyer FRAMEs on the same page and platform. This is a wiring ledger, not an executed click ledger.

|Source root|Source node · control|Trigger|Destination|Purpose|
|---|---|---|---|---|
|4:9668|25:745 · Action / 前往結帳|ON_CLICK|1429:23435|Replace superseded checkout destination|
|4:9746|51:1541 · Action / 前往結帳|ON_CLICK|1429:23484|Replace superseded checkout destination|
|4:12347|66:1964 · Action / 前往結帳（共 2 件）|ON_CLICK|1429:23435|Replace superseded checkout destination|
|4:12417|66:2118 · Action / 前往結帳（共 2 件）|ON_CLICK|1429:23484|Replace superseded checkout destination|
|66:1748|66:1807 · Action / 前往結帳（共 2 件）|ON_CLICK|1429:23435|Replace superseded checkout destination|
|66:1749|66:2018 · Action / 前往結帳（共 2 件）|ON_CLICK|1429:23484|Replace superseded checkout destination|
|66:1752|66:1886 · Action / 前往結帳（共 2 件）|ON_CLICK|1429:23435|Replace superseded checkout destination|
|66:1753|66:2071 · Action / 前往結帳（共 2 件）|ON_CLICK|1429:23484|Replace superseded checkout destination|
|6:21771|27:1219 · Product / 輕量帆布托特包|ON_CLICK|4:12347|Live product claim entry|
|6:21825|27:1260 · Product / 輕量帆布托特包|ON_CLICK|4:12417|Live product claim entry|
|1429:13961|1429:13988 · Action / 搜尋|ON_CLICK|1429:14065|Picker tab/selection|
|1429:13961|1429:13992 · Input / 選擇行政區　⌄|ON_CLICK|1429:14166|Picker tab/selection|
|1429:14020|1429:14036 · Action / 搜尋|ON_CLICK|1429:14122|Picker tab/selection|
|1429:14020|1429:14040 · Input / 選擇行政區　⌄|ON_CLICK|1429:14237|Picker tab/selection|
|1429:14065|1429:14090 · Action / 依地區|ON_CLICK|1429:13961|Picker tab/selection|
|1429:14122|1429:14136 · Action / 依地區|ON_CLICK|1429:14020|Picker tab/selection|
|1429:14166|1429:14191 · Action / 依地區|ON_CLICK|1429:13961|Picker tab/selection|
|1429:14166|1429:14193 · Action / 搜尋|ON_CLICK|1429:14065|Picker tab/selection|
|1429:14237|1429:14251 · Action / 依地區|ON_CLICK|1429:14020|Picker tab/selection|
|1429:14237|1429:14253 · Action / 搜尋|ON_CLICK|1429:14122|Picker tab/selection|
|1429:14294|1429:14319 · Action / 依地區|ON_CLICK|1429:13961|Picker tab/selection|
|1429:14294|1429:14321 · Action / 搜尋|ON_CLICK|1429:14065|Picker tab/selection|
|1429:14359|1429:14373 · Action / 依地區|ON_CLICK|1429:14020|Picker tab/selection|
|1429:14359|1429:14375 · Action / 搜尋|ON_CLICK|1429:14122|Picker tab/selection|
|1429:14411|1429:14436 · Action / 依地區|ON_CLICK|1429:13961|Picker tab/selection|
|1429:14411|1429:14438 · Action / 搜尋|ON_CLICK|1429:14065|Picker tab/selection|
|1429:14472|1429:14486 · Action / 依地區|ON_CLICK|1429:14020|Picker tab/selection|
|1429:14472|1429:14488 · Action / 搜尋|ON_CLICK|1429:14122|Picker tab/selection|
|1429:14519|1429:14544 · Action / 依地區|ON_CLICK|1429:13961|Picker tab/selection|
|1429:14519|1429:14546 · Action / 搜尋|ON_CLICK|1429:14065|Picker tab/selection|
|1429:14578|1429:14592 · Action / 依地區|ON_CLICK|1429:14020|Picker tab/selection|
|1429:14578|1429:14594 · Action / 搜尋|ON_CLICK|1429:14122|Picker tab/selection|
|1429:14624|1429:14649 · Action / 依地區|ON_CLICK|1429:13961|Picker tab/selection|
|1429:14624|1429:14651 · Action / 搜尋|ON_CLICK|1429:14065|Picker tab/selection|
|1429:14624|1429:14665 · Action / 更換門市|ON_CLICK|1429:14294|Picker tab/selection|
|1429:14624|1429:14669 · Action / 確認取貨門市|ON_CLICK|1429:14849|Confirm selected store|
|1429:14688|1429:14702 · Action / 依地區|ON_CLICK|1429:14020|Picker tab/selection|
|1429:14688|1429:14704 · Action / 搜尋|ON_CLICK|1429:14122|Picker tab/selection|
|1429:14688|1429:14716 · Action / 更換門市|ON_CLICK|1429:14359|Picker tab/selection|
|1429:14688|1429:14719 · Action / 確認取貨門市|ON_CLICK|1429:14914|Confirm selected store|
|1429:14738|1429:14763 · Action / 依地區|ON_CLICK|1429:13961|Picker tab/selection|
|1429:14738|1429:14765 · Action / 搜尋|ON_CLICK|1429:14065|Picker tab/selection|
|1429:14738|1429:14777 · Action / 更換門市|ON_CLICK|1429:14294|Picker tab/selection|
|1429:14738|1429:14781 · Action / 確認取貨門市|ON_CLICK|1429:14965|Confirm selected store|
|1429:14800|1429:14814 · Action / 依地區|ON_CLICK|1429:14020|Picker tab/selection|
|1429:14800|1429:14816 · Action / 搜尋|ON_CLICK|1429:14122|Picker tab/selection|
|1429:14800|1429:14827 · Action / 更換門市|ON_CLICK|1429:14359|Picker tab/selection|
|1429:14800|1429:14830 · Action / 確認取貨門市|ON_CLICK|1429:15028|Confirm selected store|
|1429:15078|1429:15103 · Action / 依地區|ON_CLICK|1429:13961|Picker tab/selection|
|1429:15078|1429:15105 · Action / 搜尋|ON_CLICK|1429:14065|Picker tab/selection|
|1429:15152|1429:15166 · Action / 依地區|ON_CLICK|1429:14020|Picker tab/selection|
|1429:15152|1429:15168 · Action / 搜尋|ON_CLICK|1429:14122|Picker tab/selection|
|1429:15212|1429:15237 · Action / 依地區|ON_CLICK|1429:13961|Picker tab/selection|
|1429:15212|1429:15239 · Action / 搜尋|ON_CLICK|1429:14065|Picker tab/selection|
|1429:15280|1429:15294 · Action / 依地區|ON_CLICK|1429:14020|Picker tab/selection|
|1429:15280|1429:15296 · Action / 搜尋|ON_CLICK|1429:14122|Picker tab/selection|
|1429:15335|1429:15360 · Action / 依地區|ON_CLICK|1429:13961|Picker tab/selection|
|1429:15335|1429:15362 · Action / 搜尋|ON_CLICK|1429:14065|Picker tab/selection|
|1429:15335|1429:15376 · Action / 更換門市|ON_CLICK|1429:14294|Picker tab/selection|
|1429:15401|1429:15415 · Action / 依地區|ON_CLICK|1429:14020|Picker tab/selection|
|1429:15401|1429:15417 · Action / 搜尋|ON_CLICK|1429:14122|Picker tab/selection|
|1429:15401|1429:15429 · Action / 更換門市|ON_CLICK|1429:14359|Picker tab/selection|
|1429:15453|1429:15478 · Action / 依地區|ON_CLICK|1429:13961|Picker tab/selection|
|1429:15453|1429:15480 · Action / 搜尋|ON_CLICK|1429:14065|Picker tab/selection|
|1429:15453|1429:15492 · Action / 更換門市|ON_CLICK|1429:14294|Picker tab/selection|
|1429:15517|1429:15531 · Action / 依地區|ON_CLICK|1429:14020|Picker tab/selection|
|1429:15517|1429:15533 · Action / 搜尋|ON_CLICK|1429:14122|Picker tab/selection|
|1429:15517|1429:15544 · Action / 更換門市|ON_CLICK|1429:14359|Picker tab/selection|
|1429:15568|1429:15589 · Action / 更換門市|ON_CLICK|1429:14294|Picker tab/selection|
|1429:15568|1429:15816 · Action / 確認下單|ON_CLICK|1429:24505|Return selected CVS to online checkout|
|1429:15634|1429:15646 · Action / 更換門市|ON_CLICK|1429:14359|Picker tab/selection|
|1429:15634|1429:15666 · Sticky checkout action|ON_CLICK|1429:24554|Return selected CVS to online checkout|
|1429:15686|1429:15707 · Action / 更換門市|ON_CLICK|1429:14294|Picker tab/selection|
|1429:15686|1429:15843 · Action / 確認下單|ON_CLICK|1429:24505|Return selected CVS to online checkout|
|1429:15752|1429:15764 · Action / 更換門市|ON_CLICK|1429:14359|Picker tab/selection|
|1429:15752|1429:15784 · Sticky checkout action|ON_CLICK|1429:24554|Return selected CVS to online checkout|
|1429:23781|I1429:23786;1429:20657 · SubmitAction|ON_KEY_DOWN 49|1429:13961|Open CVS picker outcome branch|
|1429:23832|I1429:23837;1429:20657 · SubmitAction|ON_KEY_DOWN 49|1429:14020|Open CVS picker outcome branch|
|782:3904|789:3962 · Action / 門市取貨|ON_KEY_DOWN 49|1429:13961|Open CVS picker outcome branch|
|782:4087|789:4133 · Action / 門市取貨|ON_KEY_DOWN 49|1429:14020|Open CVS picker outcome branch|
|782:3904|789:3969 · Input / 收件人|ON_KEY_DOWN 49|15:310|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4140 · Input / 收件人|ON_KEY_DOWN 49|15:371|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3962 · Action / 門市取貨|ON_KEY_DOWN 50|25:943|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4133 · Action / 門市取貨|ON_KEY_DOWN 50|25:995|Alternate outcome / variant from producing step; keyboard branch|
|4:9192|27:715 · Field / 搜尋商品|ON_KEY_DOWN 49|26:676|Alternate outcome / variant from producing step; keyboard branch|
|26:708|26:729 · Action / 查詢訂單|ON_KEY_DOWN 49|26:746|Alternate outcome / variant from producing step; keyboard branch|
|4:9296|27:789 · Field / 搜尋商品|ON_KEY_DOWN 49|26:968|Alternate outcome / variant from producing step; keyboard branch|
|26:995|26:1011 · Action / 查詢訂單|ON_KEY_DOWN 49|26:1028|Alternate outcome / variant from producing step; keyboard branch|
|5:15460|38:1201 · Action / 下載我的資料|ON_KEY_DOWN 49|37:1140|Alternate outcome / variant from producing step; keyboard branch|
|5:15540|38:1342 · Action / 下載我的資料|ON_KEY_DOWN 49|37:1173|Alternate outcome / variant from producing step; keyboard branch|
|26:708|26:729 · Action / 查詢訂單|ON_KEY_DOWN 50|63:1475|Alternate outcome / variant from producing step; keyboard branch|
|26:995|26:1011 · Action / 查詢訂單|ON_KEY_DOWN 50|63:1476|Alternate outcome / variant from producing step; keyboard branch|
|26:708|26:729 · Action / 查詢訂單|ON_KEY_DOWN 51|63:1477|Alternate outcome / variant from producing step; keyboard branch|
|26:995|26:1011 · Action / 查詢訂單|ON_KEY_DOWN 51|63:1478|Alternate outcome / variant from producing step; keyboard branch|
|4:12347|66:1964 · Action / 前往結帳（共 2 件）|ON_KEY_DOWN 49|66:1746|Alternate outcome / variant from producing step; keyboard branch|
|4:12417|66:2118 · Action / 前往結帳（共 2 件）|ON_KEY_DOWN 49|66:1747|Alternate outcome / variant from producing step; keyboard branch|
|4:12347|66:1964 · Action / 前往結帳（共 2 件）|ON_KEY_DOWN 50|66:1748|Alternate outcome / variant from producing step; keyboard branch|
|4:12417|66:2118 · Action / 前往結帳（共 2 件）|ON_KEY_DOWN 50|66:1749|Alternate outcome / variant from producing step; keyboard branch|
|4:12347|66:1964 · Action / 前往結帳（共 2 件）|ON_KEY_DOWN 51|66:1750|Alternate outcome / variant from producing step; keyboard branch|
|4:12417|66:2118 · Action / 前往結帳（共 2 件）|ON_KEY_DOWN 51|66:1751|Alternate outcome / variant from producing step; keyboard branch|
|4:12347|66:1964 · Action / 前往結帳（共 2 件）|ON_KEY_DOWN 52|66:1752|Alternate outcome / variant from producing step; keyboard branch|
|4:12417|66:2118 · Action / 前往結帳（共 2 件）|ON_KEY_DOWN 52|66:1753|Alternate outcome / variant from producing step; keyboard branch|
|4:12347|66:1964 · Action / 前往結帳（共 2 件）|ON_KEY_DOWN 53|66:1754|Alternate outcome / variant from producing step; keyboard branch|
|4:12417|66:2118 · Action / 前往結帳（共 2 件）|ON_KEY_DOWN 53|66:1755|Alternate outcome / variant from producing step; keyboard branch|
|4:9668|25:745 · Action / 前往結帳|ON_KEY_DOWN 49|102:2267|Alternate outcome / variant from producing step; keyboard branch|
|4:9746|51:1541 · Action / 前往結帳|ON_KEY_DOWN 49|102:2324|Alternate outcome / variant from producing step; keyboard branch|
|4:9668|25:745 · Action / 前往結帳|ON_KEY_DOWN 50|104:2313|Alternate outcome / variant from producing step; keyboard branch|
|4:9746|51:1541 · Action / 前往結帳|ON_KEY_DOWN 50|104:2371|Alternate outcome / variant from producing step; keyboard branch|
|4:9668|25:745 · Action / 前往結帳|ON_KEY_DOWN 51|104:2657|Alternate outcome / variant from producing step; keyboard branch|
|4:9746|51:1541 · Action / 前往結帳|ON_KEY_DOWN 51|104:2715|Alternate outcome / variant from producing step; keyboard branch|
|4:9668|25:745 · Action / 前往結帳|ON_KEY_DOWN 52|104:3176|Alternate outcome / variant from producing step; keyboard branch|
|4:9746|51:1541 · Action / 前往結帳|ON_KEY_DOWN 52|104:3234|Alternate outcome / variant from producing step; keyboard branch|
|4:9668|25:745 · Action / 前往結帳|ON_KEY_DOWN 53|104:3520|Alternate outcome / variant from producing step; keyboard branch|
|4:9746|51:1541 · Action / 前往結帳|ON_KEY_DOWN 53|104:3578|Alternate outcome / variant from producing step; keyboard branch|
|4:9668|25:745 · Action / 前往結帳|ON_KEY_DOWN 54|104:3862|Alternate outcome / variant from producing step; keyboard branch|
|4:9746|51:1541 · Action / 前往結帳|ON_KEY_DOWN 54|104:3920|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3962 · Action / 門市取貨|ON_KEY_DOWN 51|1198:14815|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4133 · Action / 門市取貨|ON_KEY_DOWN 51|1198:15153|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3962 · Action / 門市取貨|ON_KEY_DOWN 52|1222:14056|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4133 · Action / 門市取貨|ON_KEY_DOWN 52|1222:14106|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3962 · Action / 門市取貨|ON_KEY_DOWN 53|1222:14148|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4133 · Action / 門市取貨|ON_KEY_DOWN 53|1222:14197|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3962 · Action / 門市取貨|ON_KEY_DOWN 54|1222:14238|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4133 · Action / 門市取貨|ON_KEY_DOWN 54|1222:14287|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3962 · Action / 門市取貨|ON_KEY_DOWN 55|1222:14965|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4133 · Action / 門市取貨|ON_KEY_DOWN 55|1222:15012|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3962 · Action / 門市取貨|ON_KEY_DOWN 56|1222:15051|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4133 · Action / 門市取貨|ON_KEY_DOWN 56|1222:15290|Alternate outcome / variant from producing step; keyboard branch|
|4:9963|25:840 · Action / 前往安全付款頁|ON_KEY_DOWN 49|1222:21795|Alternate outcome / variant from producing step; keyboard branch|
|4:10049|25:898 · Action / 前往安全付款頁|ON_KEY_DOWN 49|1222:22313|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3995 · Action / 確認下單|ON_KEY_DOWN 49|1222:26991|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3995 · Action / 確認下單|ON_KEY_DOWN 50|1222:27044|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3995 · Action / 確認下單|ON_KEY_DOWN 51|1222:27097|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4168 · Action / 確認下單|ON_KEY_DOWN 49|1222:27195|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4168 · Action / 確認下單|ON_KEY_DOWN 50|1222:27240|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4168 · Action / 確認下單|ON_KEY_DOWN 51|1222:27285|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3962 · Action / 門市取貨|ON_KEY_DOWN 57|1225:20147|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4133 · Action / 門市取貨|ON_KEY_DOWN 57|1225:20196|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3995 · Action / 確認下單|ON_KEY_DOWN 52|1241:15225|Alternate outcome / variant from producing step; keyboard branch|
|782:3904|789:3995 · Action / 確認下單|ON_KEY_DOWN 53|1241:15291|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4168 · Action / 確認下單|ON_KEY_DOWN 52|1241:15370|Alternate outcome / variant from producing step; keyboard branch|
|782:4087|789:4168 · Action / 確認下單|ON_KEY_DOWN 53|1241:15428|Alternate outcome / variant from producing step; keyboard branch|
|6:19772|792:3973 · Action / 所有商品|ON_KEY_DOWN 49|1253:17090|Alternate outcome / variant from producing step; keyboard branch|
|6:19859|792:3993 · Action / 所有商品|ON_KEY_DOWN 49|1253:17114|Alternate outcome / variant from producing step; keyboard branch|
|4:9192|27:723 · Action / 探索日常選物|ON_KEY_DOWN 49|1253:17135|Alternate outcome / variant from producing step; keyboard branch|
|4:9296|27:797 · Action / 探索日常選物|ON_KEY_DOWN 49|1253:17159|Alternate outcome / variant from producing step; keyboard branch|
|26:708|26:729 · Action / 查詢訂單|ON_KEY_DOWN 52|1253:17225|Alternate outcome / variant from producing step; keyboard branch|
|26:995|26:1011 · Action / 查詢訂單|ON_KEY_DOWN 52|1253:17256|Alternate outcome / variant from producing step; keyboard branch|
|1429:13961|1429:13992 · Input / 選擇行政區　⌄|ON_KEY_DOWN 49|1429:14411|Alternate outcome / variant from producing step; keyboard branch|
|1429:14020|1429:14040 · Input / 選擇行政區　⌄|ON_KEY_DOWN 49|1429:14472|Alternate outcome / variant from producing step; keyboard branch|
|1429:14065|1429:14092 · Action / 搜尋|ON_KEY_DOWN 49|1429:14519|Alternate outcome / variant from producing step; keyboard branch|
|1429:14122|1429:14138 · Action / 搜尋|ON_KEY_DOWN 49|1429:14578|Alternate outcome / variant from producing step; keyboard branch|
|1429:14166|1429:14183 · Action / 7-ELEVEN|ON_KEY_DOWN 49|1429:14624|Alternate outcome / variant from producing step; keyboard branch|
|1429:14237|1429:14243 · Action / 7-ELEVEN|ON_KEY_DOWN 49|1429:14688|Alternate outcome / variant from producing step; keyboard branch|
|1429:14624|1429:14669 · Action / 確認取貨門市|ON_KEY_DOWN 49|1429:15078|Alternate outcome / variant from producing step; keyboard branch|
|1429:14688|1429:14719 · Action / 確認取貨門市|ON_KEY_DOWN 49|1429:15152|Alternate outcome / variant from producing step; keyboard branch|
|1429:14738|1429:14781 · Action / 確認取貨門市|ON_KEY_DOWN 49|1429:15212|Alternate outcome / variant from producing step; keyboard branch|
|1429:14800|1429:14830 · Action / 確認取貨門市|ON_KEY_DOWN 49|1429:15280|Alternate outcome / variant from producing step; keyboard branch|
|1429:14624|1429:14669 · Action / 確認取貨門市|ON_KEY_DOWN 50|1429:15335|Alternate outcome / variant from producing step; keyboard branch|
|1429:14688|1429:14719 · Action / 確認取貨門市|ON_KEY_DOWN 50|1429:15401|Alternate outcome / variant from producing step; keyboard branch|
|1429:14738|1429:14781 · Action / 確認取貨門市|ON_KEY_DOWN 50|1429:15453|Alternate outcome / variant from producing step; keyboard branch|
|1429:14800|1429:14830 · Action / 確認取貨門市|ON_KEY_DOWN 50|1429:15517|Alternate outcome / variant from producing step; keyboard branch|
|1429:14624|1429:14669 · Action / 確認取貨門市|ON_KEY_DOWN 51|1429:15568|Alternate outcome / variant from producing step; keyboard branch|
|1429:14688|1429:14719 · Action / 確認取貨門市|ON_KEY_DOWN 51|1429:15634|Alternate outcome / variant from producing step; keyboard branch|
|1429:14738|1429:14781 · Action / 確認取貨門市|ON_KEY_DOWN 51|1429:15686|Alternate outcome / variant from producing step; keyboard branch|
|1429:14800|1429:14830 · Action / 確認取貨門市|ON_KEY_DOWN 51|1429:15752|Alternate outcome / variant from producing step; keyboard branch|
|1429:23435|I1429:23440;1429:20657 · SubmitAction|ON_KEY_DOWN 49|1429:23533|Alternate outcome / variant from producing step; keyboard branch|
|1429:23484|I1429:23489;1429:20657 · SubmitAction|ON_KEY_DOWN 49|1429:23577|Alternate outcome / variant from producing step; keyboard branch|
|1429:23435|I1429:23440;1429:20657 · SubmitAction|ON_KEY_DOWN 50|1429:23701|Alternate outcome / variant from producing step; keyboard branch|
|1429:23484|I1429:23489;1429:20657 · SubmitAction|ON_KEY_DOWN 50|1429:23741|Alternate outcome / variant from producing step; keyboard branch|
|1429:23781|I1429:23786;1429:20657 · SubmitAction|ON_KEY_DOWN 50|1429:23883|Alternate outcome / variant from producing step; keyboard branch|
|1429:23832|I1429:23837;1429:20657 · SubmitAction|ON_KEY_DOWN 50|1429:23929|Alternate outcome / variant from producing step; keyboard branch|
|1429:23781|I1429:23786;1429:20657 · SubmitAction|ON_KEY_DOWN 51|1429:24059|Alternate outcome / variant from producing step; keyboard branch|
|1429:23832|I1429:23837;1429:20657 · SubmitAction|ON_KEY_DOWN 51|1429:24101|Alternate outcome / variant from producing step; keyboard branch|
|1429:24143|I1429:24148;1429:20657 · SubmitAction|ON_KEY_DOWN 49|1429:24245|Alternate outcome / variant from producing step; keyboard branch|
|1429:24194|I1429:24199;1429:20657 · SubmitAction|ON_KEY_DOWN 49|1429:24291|Alternate outcome / variant from producing step; keyboard branch|
|1429:24143|I1429:24148;1429:20657 · SubmitAction|ON_KEY_DOWN 50|1429:24421|Alternate outcome / variant from producing step; keyboard branch|
|1429:24194|I1429:24199;1429:20657 · SubmitAction|ON_KEY_DOWN 50|1429:24463|Alternate outcome / variant from producing step; keyboard branch|
|1429:23781|I1429:23786;1429:20657 · SubmitAction|ON_KEY_DOWN 52|1429:24603|Alternate outcome / variant from producing step; keyboard branch|
|1429:23832|I1429:23837;1429:20657 · SubmitAction|ON_KEY_DOWN 52|1429:24654|Alternate outcome / variant from producing step; keyboard branch|
|1429:23781|I1429:23786;1429:20657 · SubmitAction|ON_KEY_DOWN 53|1429:24705|Alternate outcome / variant from producing step; keyboard branch|
|1429:23832|I1429:23837;1429:20657 · SubmitAction|ON_KEY_DOWN 53|1429:24756|Alternate outcome / variant from producing step; keyboard branch|
|1429:24505|I1429:24510;1429:20657 · SubmitAction|ON_KEY_DOWN 49|1429:25101|Alternate outcome / variant from producing step; keyboard branch|
|1429:24554|I1429:24559;1429:20657 · SubmitAction|ON_KEY_DOWN 49|1429:25145|Alternate outcome / variant from producing step; keyboard branch|
|1429:24505|I1429:24510;1429:20657 · SubmitAction|ON_KEY_DOWN 50|1429:25269|Alternate outcome / variant from producing step; keyboard branch|
|1429:24554|I1429:24559;1429:20657 · SubmitAction|ON_KEY_DOWN 50|1429:25309|Alternate outcome / variant from producing step; keyboard branch|
|1429:23781|I1429:23786;1429:20657 · SubmitAction|ON_KEY_DOWN 54|1429:25349|Alternate outcome / variant from producing step; keyboard branch|
|1429:23832|I1429:23837;1429:20657 · SubmitAction|ON_KEY_DOWN 54|1429:25395|Alternate outcome / variant from producing step; keyboard branch|
|1429:23781|I1429:23786;1429:20657 · SubmitAction|ON_KEY_DOWN 55|1429:25525|Alternate outcome / variant from producing step; keyboard branch|
|1429:23832|I1429:23837;1429:20657 · SubmitAction|ON_KEY_DOWN 55|1429:25567|Alternate outcome / variant from producing step; keyboard branch|
|1429:23781|I1429:23786;1429:20657 · SubmitAction|ON_KEY_DOWN 56|1429:25609|Alternate outcome / variant from producing step; keyboard branch|
|1429:23832|I1429:23837;1429:20657 · SubmitAction|ON_KEY_DOWN 56|1429:25655|Alternate outcome / variant from producing step; keyboard branch|
|1429:23781|I1429:23786;1429:20657 · SubmitAction|ON_KEY_DOWN 57|1429:25785|Alternate outcome / variant from producing step; keyboard branch|
|1429:23832|I1429:23837;1429:20657 · SubmitAction|ON_KEY_DOWN 57|1429:25827|Alternate outcome / variant from producing step; keyboard branch|
|4:9192|27:723 · Action / 探索日常選物|ON_KEY_DOWN 50|1430:68473|Alternate outcome / variant from producing step; keyboard branch|
|4:9296|27:797 · Action / 探索日常選物|ON_KEY_DOWN 50|1430:68559|Alternate outcome / variant from producing step; keyboard branch|
|1429:14166|1429:14204 · Store option / 121275 / whole-row target|ON_CLICK|1429:14624|Select actual region result row|
|1429:14237|1429:14262 · Store option / 121275 / whole-row target|ON_CLICK|1429:14688|Select actual region result row|
|1429:14849|1429:14850 · Header|AFTER_TIMEOUT 1.2s|1429:15568|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:14914|1429:14915 · Sticky brand and filters|AFTER_TIMEOUT 1.2s|1429:15634|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:14965|1429:14966 · Header|AFTER_TIMEOUT 1.2s|1429:15686|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:15028|1429:15029 · Sticky brand and filters|AFTER_TIMEOUT 1.2s|1429:15752|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:23621|1429:23622 · Header|AFTER_TIMEOUT 1.2s|4:9963|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:23661|1429:23662 · Header|AFTER_TIMEOUT 1.2s|4:10049|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:23975|1429:23976 · Header|AFTER_TIMEOUT 1.2s|1198:14815|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:24017|1429:24018 · Header|AFTER_TIMEOUT 1.2s|1198:15153|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:24337|1429:24338 · Header|AFTER_TIMEOUT 1.2s|1222:21795|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:24379|1429:24380 · Header|AFTER_TIMEOUT 1.2s|1222:22313|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:25189|1429:25190 · Header|AFTER_TIMEOUT 1.2s|25:943|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:25229|1429:25230 · Header|AFTER_TIMEOUT 1.2s|25:995|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:25441|1429:25442 · Header|AFTER_TIMEOUT 1.2s|1198:14815|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:25483|1429:25484 · Header|AFTER_TIMEOUT 1.2s|1198:15153|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:25701|1429:25702 · Header|AFTER_TIMEOUT 1.2s|1198:14815|Prototype submission/confirmation resolves to server-observed order/summary|
|1429:25743|1429:25744 · Header|AFTER_TIMEOUT 1.2s|1198:15153|Prototype submission/confirmation resolves to server-observed order/summary|
|782:3904|789:3978 · Action / 更換或修改門市|ON_CLICK|1429:13961|Open signed CVS picker|
|782:4087|789:4149 · Action / 更換或修改門市|ON_CLICK|1429:14020|Open signed CVS picker|
