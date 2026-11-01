# PARITY-DATA-OPS — ROUND 1 tests-only handoff

**Evidence: E1 static checks only. BROWSER/RED/GREEN/independent review: NOT_RUN. No product implementation or production action.**

## Authority and dispatch

- Branch/worktree: `unit/parity-data-ops` / `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-data-ops`. Base after requested merge: `4b8c04c7213025c173e260db6f9c81cb5b818c61` (already up to date with origin/r3/integration).
- Role: Codex dispatched implementation lane; exact runtime model/reasoning is not exposed by this session (UNKNOWN); no delegation.
- Scope: current user ROUND 1 instruction; C6 row and Appendix A of FIGMA-INVENTORY. Signed visible text: `FRAME-COPY.md` from #472; SHA256 `66a31a3b8a38b4c61ddb0cc597ae512c5415123e9c2a3f900abedd90311c9514`.
- Read AGENTS.md, 架构.md §§0/1/2/26/27, current route/components, strict import and ledger DTOs, existing catalog/migration browser harnesses, ux-k2-catalog DELIVERY existing-entry copy table. Fleet Bridge read-only get_design_context + get_screenshot for all 44 IDs; receipts/PNGs retained under `figma/` in this worktree.
- Humaux search/get/coord/store/canvas tools absent from the available tool inventory. Claim, rejected enumeration, store, and canvas update NOT_RUN; no substitute local memory written. Installed ponytail skill applied; no new tool or dependency.

## Change and gates

- Add real-route browser assertions for product CSV, inventory and collections; add customer-import assertions in its existing signed HTTPS fixture. Add file/column controls, dry runs, confirmation, discard, stale/transport-loss recovery and persisted-write readback. Fixtures are synthetic and store-scoped; API calls only prepare fixtures or inject explicitly marked MOCK faults, never substitute for UI actions.
- Preserve existing catalog and import browser tests. Register new files additively in the existing Go harnesses. Existing next-startup/gate-adapter assertions are untouched.
- Exact signed zh-TW copy asserted. Product/collection titles and product actions in zh-CN/en use the K2 existing-entry table. Customer zh-CN/en run the main screen and legacy-selector absence; exact translation parity is DEFERRED because neither supplied copy export nor located DELIVERY provides those translations. No translations guessed.
- `node --check` on all three new TypeScript files: exit 0.
- Strict `tsc --noEmit`, ESNext + DOM, Bundler resolution, existing shared compiler and installed Node 24 type root (read-only): exit 0; `typecheck.log`. Initial worktree-local compiler command was unavailable; first shared invocation lacked Node type lookup, resolved by explicit existing type root. No dependencies/locks changed.
- `python3 scripts/check_packet.py`: exit 0; `check-packet.log`. Its timestamp-only change to experiments/results/packet-check.json was restored; not part of this card.
- `node scripts/dev/derive-browser-ownership.mjs`, then `--check`: exit 0. Artifact includes both new specs and support import closure in their actual modes.
- `bash scripts/dev/depmap.sh`: exit 0; generated map unchanged. `gofmt`: applied to the two registration files. No local tests/servers.
- RED gate: the PR must fail on signed real-page headings/controls/legacy-selector assertions. No RED conclusion is claimed before a browser job reports those assertion failures. GREEN and implementation are outside ROUND 1.

## Legacy → signed mapping (same intent, existing tests retained)

| Route | Legacy elements asserted absent | Signed replacement |
|---|---|---|
| products/import | 匯入與匯出商品 / Import and export products heading; 檢查檔案 / Check the file button | 商品匯入與匯出; 預覽檔案 |
| inventory | 商品與庫存 / Products & inventory page heading | 庫存, 搜尋貨號, selected-SKU adjustment/review |
| collections | 分類 / Collections page heading | 集合與分類, + 新建集合 |
| customers/import | import-type-orders historical-order type selector on customer import | 匯入顧客, mapping and 預覽可套用資料 |

## Required facts across desktop/mobile/review

| Module | Entry facts | Mobile and review/result checks |
|---|---|---|
| Product CSV | selected filename, 1500-row/2MB limit, row errors, atomic import | Same workflow at 1440×900 and 390×844; failed batch, preview/result, persisted product after refresh |
| Inventory | selected SKU, warehouse, on-hand/available, reserved/allocated/unavailable; draft quantity/reason | Same viewport pairs, +3/required reason, review, discard/continue, conflict, immutable retry, persisted 27 on-hand after refresh. Fixture quantities are synthetic current server facts, not copied operational facts. Historical actor/source/before-after rows DEFERRED |
| Collections | collection name/type, product-preservation rule | Entry heading/actions in all locales; manual creation name survives refresh on both devices. Automatic backend behavior remains covered by existing tests; not replaced |
| Customer CSV | filename, requested mapping, omitted-email preservation, safe per-row verdicts, no consent grant | Both sizes: mapping retained on Back, safe preview/errors, confirm/result, stale/unknown immutable retry; unique synthetic imported name survives refresh in customer list |

## Parity sheet (planned CI captures, NOT_RUN)

Paths below are targets, not claims that CI captures exist. Entry captures occur before the signed assertion, exposing the real legacy page in RED. Later state captures exist only if their click path reaches that state. Automatic Playwright failure screenshots/traces remain enabled.

| Frame | Real-route CI screenshot | Signed reference PNG from Fleet Bridge | State / limitation |
|---|---|---|---|
| `5:15211` | `parity/5:15211.png` | `figma/get_screenshot-5-15211-1793511922329-0.png` | Admin Desktop · 商品匯入與匯出 · SHOPLINE · 1440×900px |
| `5:15352` | `parity/5:15352.png` | `figma/get_screenshot-5-15352-1793511922571-0.png` | Admin Mobile · 商品匯入與匯出 · SHOPLINE · 390×844px |
| `30:1441` | `parity/30:1441.png` | `figma/get_screenshot-30-1441-1793511923021-0.png` | Admin Desktop · 預覽商品匯入 · SHOPLINE · 1440×900px |
| `30:1528` | `parity/30:1528.png` | `figma/get_screenshot-30-1528-1793511923321-0.png` | Admin Mobile · 預覽商品匯入 · SHOPLINE · 390×844px |
| `30:1574` | `parity/30:1574.png` | `figma/get_screenshot-30-1574-1793511923787-0.png` | Admin Desktop · 匯入結果 · SHOPLINE · 1440×900px |
| `30:1635` | `parity/30:1635.png` | `figma/get_screenshot-30-1635-1793511924044-0.png` | Admin Mobile · 匯入結果 · SHOPLINE · 390×844px |
| `750:3456` | `parity/750:3456.png` | `figma/get_screenshot-750-3456-1793511924483-0.png` | Admin Desktop · 匯入錯誤明細 · SHOPLINE · 1440×900px |
| `750:3598` | `parity/750:3598.png` | `figma/get_screenshot-750-3598-1793511924824-0.png` | Admin Mobile · 匯入錯誤明細 · SHOPLINE · 390×844px |
| `6:19050` | `parity/6:19050.png` | `figma/get_screenshot-6-19050-1793511925268-0.png` | Admin Desktop · 調整庫存 · 選中貨號 · 1440×900px |
| `6:19202` | `parity/6:19202.png` | `figma/get_screenshot-6-19202-1793511925605-0.png` | Admin Mobile · 調整庫存 · 選中貨號 · 390×844px |
| `6:22818` | DEFERRED | `figma/get_screenshot-6-22818-1793511926003-0.png` | Historical inventory rows need a scoped ledger GET with timestamp, actor, source, reason and before/after quantities; current catalog-ledger returns balances only. |
| `6:22986` | DEFERRED | `figma/get_screenshot-6-22986-1793511926336-0.png` | Same missing historical inventory read contract as 6:22818. |
| `21:480` | `parity/21:480.png` | `figma/get_screenshot-21-480-1793511926755-0.png` | Admin Desktop · 庫存 · 選中貨號 · 1440×900px |
| `21:899` | `parity/21:899.png` | `figma/get_screenshot-21-899-1793511927005-0.png` | Admin Mobile · 庫存 · 選中貨號 · 390×844px |
| `840:5246` | `parity/840:5246.png` | `figma/get_screenshot-840-5246-1793511927450-0.png` | Admin Desktop · 庫存 / review · 1440×900px |
| `840:5247` | `parity/840:5247.png` | `figma/get_screenshot-840-5247-1793511927697-0.png` | Admin Mobile · 庫存 / review · 390×844px |
| `840:5248` | `parity/840:5248.png` | `figma/get_screenshot-840-5248-1793511928145-0.png` | Admin Desktop · 庫存 / result · 1440×900px |
| `840:5249` | `parity/840:5249.png` | `figma/get_screenshot-840-5249-1793511928374-0.png` | Admin Mobile · 庫存 / result · 390×844px |
| `840:5250` | `parity/840:5250.png` | `figma/get_screenshot-840-5250-1793511928736-0.png` | Admin Desktop · 庫存 / conflict · 1440×900px |
| `840:5251` | `parity/840:5251.png` | `figma/get_screenshot-840-5251-1793511929048-0.png` | Admin Mobile · 庫存 / conflict · 390×844px |
| `840:5252` | `parity/840:5252.png` | `figma/get_screenshot-840-5252-1793511929412-0.png` | Admin Desktop · 庫存 / unknown · 1440×900px |
| `840:5253` | `parity/840:5253.png` | `figma/get_screenshot-840-5253-1793511929683-0.png` | Admin Mobile · 庫存 / unknown · 390×844px |
| `840:5254` | `parity/840:5254.png` | `figma/get_screenshot-840-5254-1793511930053-0.png` | Admin Desktop · 庫存 / search · 1440×900px |
| `840:5255` | `parity/840:5255.png` | `figma/get_screenshot-840-5255-1793511930310-0.png` | Admin Mobile · 庫存 / search · 390×844px |
| `840:5256` | `parity/840:5256.png` | `figma/get_screenshot-840-5256-1793511930770-0.png` | Admin Desktop · 庫存 / discard · 1440×900px |
| `840:5257` | `parity/840:5257.png` | `figma/get_screenshot-840-5257-1793511931065-0.png` | Admin Mobile · 庫存 / discard · 390×844px |
| `6:19278` | `parity/6:19278.png` | `figma/get_screenshot-6-19278-1793511931612-0.png` | Admin · 集合分类 · SHOPLINE · 1440×900px |
| `6:19486` | `parity/6:19486.png` | `figma/get_screenshot-6-19486-1793511931845-0.png` | Admin Mobile · 集合分类 · SHOPLINE · 390×844px |
| `6:18298` | `parity/6:18298.png` | `figma/get_screenshot-6-18298-1793511932301-0.png` | Admin Desktop · 顧客匯入 · SHOPLINE · 1440×900px |
| `6:18486` | `parity/6:18486.png` | `figma/get_screenshot-6-18486-1793511932617-0.png` | Admin Mobile · 顧客匯入 · SHOPLINE · 390×844px |
| `1028:6685` | `parity/1028:6685.png` | `figma/get_screenshot-1028-6685-1793511933025-0.png` | Admin Desktop · 顧客匯入 · preview · 1440×900px |
| `1028:6763` | `parity/1028:6763.png` | `figma/get_screenshot-1028-6763-1793511933287-0.png` | Admin Mobile · 顧客匯入 · preview · 390×844px |
| `1028:6819` | `parity/1028:6819.png` | `figma/get_screenshot-1028-6819-1793511933715-0.png` | Admin Desktop · 顧客匯入 · result · 1440×900px |
| `1028:6897` | `parity/1028:6897.png` | `figma/get_screenshot-1028-6897-1793511934034-0.png` | Admin Mobile · 顧客匯入 · result · 390×844px |
| `1028:6953` | `parity/1028:6953.png` | `figma/get_screenshot-1028-6953-1793511934448-0.png` | Admin Desktop · 顧客匯入 · pending · 1440×900px |
| `1028:7031` | `parity/1028:7031.png` | `figma/get_screenshot-1028-7031-1793511934701-0.png` | Admin Mobile · 顧客匯入 · pending · 390×844px |
| `1028:7087` | DEFERRED | `figma/get_screenshot-1028-7087-1793511935106-0.png` | Prototype-only demo CSV chooser; real native CSV file selection is covered instead. |
| `1028:7165` | DEFERRED | `figma/get_screenshot-1028-7165-1793511935368-0.png` | Prototype-only demo CSV chooser; real native CSV file selection is covered instead. |
| `1028:7221` | `parity/1028:7221.png` | `figma/get_screenshot-1028-7221-1793511935733-0.png` | Admin Desktop · 顧客匯入 · errors · 1440×900px |
| `1028:7299` | `parity/1028:7299.png` | `figma/get_screenshot-1028-7299-1793511936005-0.png` | Admin Mobile · 顧客匯入 · errors · 390×844px |
| `1198:10518` | `parity/1198:10518.png` | `figma/get_screenshot-1198-10518-1793511936445-0.png` | Admin Desktop · 顧客匯入 · stale · DRAFT-COPY · 1440×900px |
| `1198:10588` | `parity/1198:10588.png` | `figma/get_screenshot-1198-10588-1793511936729-0.png` | Admin Mobile · 顧客匯入 · stale · DRAFT-COPY · 390×844px |
| `1198:10633` | `parity/1198:10633.png` | `figma/get_screenshot-1198-10633-1793511937146-0.png` | Admin Desktop · 顧客匯入 · stale-unknown · DRAFT-COPY · 1440×900px |
| `1198:10678` | `parity/1198:10678.png` | `figma/get_screenshot-1198-10678-1793511937434-0.png` | Admin Mobile · 顧客匯入 · stale-unknown · DRAFT-COPY · 390×844px |

## DEFERRED / NOT_RUN

- `6:22818`: Historical inventory rows need a scoped ledger GET with timestamp, actor, source, reason and before/after quantities; current catalog-ledger returns balances only.
- `6:22986`: Same missing historical inventory read contract as 6:22818.
- `1028:7087`: Prototype-only demo CSV chooser; real native CSV file selection is covered instead.
- `1028:7165`: Prototype-only demo CSV chooser; real native CSV file selection is covered instead.
- Product entry “最近任務” history has no jobs/list response in current product import DTO: filename/type/status/count history cannot be fabricated. No fabricated historical task rows are asserted.
- Customer exact zh-CN/en strings: signed translation source missing. All three locale routes are registered; exact translated-copy comparison remains deferred.
- Signed successful-product-preview submission copy is absent from FRAME-COPY: the result workflow retains existing `import-commit` control ID rather than inventing a visible label. Data-specific row totals are real server results, not forced to demonstration numbers.
- BROWSER/RED/GREEN, screenshot comparison, full per-control UI acceptance, independent reviewer, provider SANDBOX/LIVE and production deployment: NOT_RUN. No acceptance claim follows from author review.

## UI process (ROUND 1, no design authored)

### BOURDIEU
Q1 task words: signed labels used. Q2 repeated workflows: no new tutorial. Q3 buyer checkout: N/A. Q4 pickup: N/A. Q5 recent task experience is not inferred from device; same workflows at both viewports. Q6 aesthetics UNKNOWN until owner. These are test-design limits, not user-research results.
### PROJECT UNDERSTANDING
Go/PostgreSQL own stock/transactions; frontend consumes scoped server DTOs. Imports do not grant consent, merge buyers, or echo private CSV cells. UNKNOWN retains the original request. All acceptance is MOCK identity/synthetic data, with explicit real-PG versus MOCK-fault boundaries.
### MOBBIN → EAGLE
No design/refactoring or reference import in ROUND 1. Existing K2 DELIVERY records Eagle folder MVS1D5NC7KI5T and structural references; none were imported or claimed revalidated by this lane. New Mobbin query: none.
### FIGMA OFFICIAL GUIDANCE
No Figma writes, components, variables or prototype wiring authored. Read-only Fleet Bridge README followed. Official design-guide application and copy-lint for new/changed Figma frames: N/A (zero frames changed).

## Files / ownership / follow-up

- New browser specs/support are within browser-spec registration scope. `tests/foundation/browser_catalog_core_test.go` and `browser_migration_import_ui_test.go` change registration/header lines only. Generated ownership is pre-approved; dependency-map has no diff. Output handoff/logs are delivery evidence. No other app, shared schema, migration, dependency manifest/lock or protected baseline changed.
- Integrator owns PR opening after the guard. Neither CI workflow runs on this feature branch push alone: Gates listens to r3/integration pushes/PRs; Foundation listens to main pushes/PRs. At handoff: PR and WAIT_RUN NOT_CREATED, not a plan-job substitute.
- No background processes, servers, containers, ports or local UI fixtures remain. Figma exports and static logs retained as task evidence. No production action, merge or auto-merge.

🤖 Generated with Codex (dispatched by Claude Code integrator).
