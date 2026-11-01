# PARITY-DATA-OPS — implementation relay, 2026-11-01

## Current FIX1 handoff

Read [FIX1.md](FIX1.md) for all current-head failures, classes, repairs and the exact base identity exception. The earlier sections below are historical implementation receipts, not current status. FIX1 remains E1 locally; no new-head GREEN or completed UI acceptance is claimed. Humaux was available this session; classification and independent subtask canvas were recorded.

- Merged integration c52b65283 into own branch as5b428a0fe; generated ownership conflict resolved by generator.
- Static checks: six node --check commands, targeted strict tsc including visual-states, static design detector, browser ownership generation/--check, depmap generation, packet check and git diff --check all exit0. Additional product-editor.acceptance compile closure is NOT_RUN locally (uninstalled axe-core); full CI remains authoritative. Depmap produced no diff; packet timestamp-only change restored.
- Local CI artifacts/logs retained uncommitted; their run/job/artifact paths and SHA256 are in FIX1-evidence-manifest.json. New raw downloads are never staged.
- Class(c) proof: base41256591992/job124701685425/artifact13063513565, settings-real browser log lines11–12,28,32,37, same disabled Save pricing policy at settings-v2.spec.ts1085; no settings edit.
- CI wake/timeout: push once; identify exact-head Gates/Foundation runs, then integrator relays on completion/failure/cancellation. Do not poll beyond10min or attempt a third repair if the same assertion remains red.
- Scope review: shared sweep monitor/driver, storefront merchant setup/callers and admin legacy/PE acceptance specs are outside literal C6 component paths, explicitly listed in PR under the directed FIX1 scope.

### FIX1 required-fact parity and click acceptance

| Required fact | Desktop | Mobile / review / summary | Evidence boundary |
|---|---|---|---|
| SKU identity and warehouse | Existing signed list/edit | Same selected SKU and warehouse | Source InventoryView; current-head real-click CI pending |
| Server on-hand/reserved/allocated/unavailable/available | Five balances retained | Same balances; draft/review shows before/after plus reason | No client-owned persisted quantity; original stock9/7 and zero-stock oracles retained |
| Original file, safe counts and immutable retry | Existing import stages | Same selected file and mapping through UNKNOWN/result | Result restart disabled prop explicit; replay/mismatch tests unchanged |
| Collection title/type/rule/count/status | Existing list with visible title span | Same list metadata/mobile cards and editor | Hidden slug retained for accessible name; no field removed |

Actual clicks/results for this FIX1 head are NOT_RUN until its own CI returns; no screenshot or static check substitutes. Existing per-control C6 and legacy click ledgers remain in CI artifacts. No new UI string or Figma design was authored, so the existing three-language copy table and five UI-process sections below remain the supplied design context.

---

PR #475 targets `r3/integration`; branch `unit/parity-data-ops`, inherited worktree. Base `4b8c04c7213025c173e260db6f9c81cb5b818c61` is still the current remote integration ancestor (fetched this session). CATALOG-PEOPLE #363 is merged. No other branch/worktree, merge, auto-merge, production or live data operation.

**E1 structure ready; author assessment E0. Implementation GREEN, E3 UI acceptance and independent review NOT_RUN.** The precise customer error-field assertions remain BLOCKED below. This delivery supersedes earlier ROUND 1/RED-only handoffs.

## Authority, ownership and evidence

- Latest dispatched ROUND 2 card; FIGMA-INVENTORY §3 C6 / Appendix A's 44 IDs; signed `FRAME-COPY.md` from #472; `AGENTS.md`; 架构.md §§0/1/2/26/27; `contracts/invariants.json`; storefront-v2 §A/G2 and the existing 2026-10-30 automatic-collections amendment; migration-import-v1 §§1–4. Go/parser source resolves actual response fields.
- 44 original read-only Fleet Bridge design-context logs and 44 screenshot logs/PNGs remain under ignored `figma/`. Representative desktop/mobile screens were inspected; live `figma_status` returned a connected document and `get_design_context(5:15211)` returned the signed root this session. No Figma writes, restart or new design.
- Implementation worker: Codex dispatched by integrator; exact model ID/reasoning UNKNOWN (not exposed). No child agents. This worktree/branch only. Humaux MCP is absent from the available tools: search/get/rejected enumeration/coord/store/canvas NOT_RUN; no substitute memory store.
- Causal RED: head `a2c5eeb2e1dda3e8aaad61ccec47978c02c0e1f3`; Gates `41282897989`, merge `5fb94235200b4f07bbe820630e64981b67906f1b`, jobs `124780074650` (bundle:03) and `124780074941` (bundle:07). Read `red.log` for assertion excerpts and artifact IDs. Failures are new parity assertions, not compile/setup failures. Existing migration tests: 15 pass; existing catalog tests: pass. One allowed selector correction: the four customer-mapping combobox count is now scoped to import-wizard, excluding two shared-shell locale/store comboboxes. Count remains four; no signed copy, behavioral assertion or fixture changed.

## Implementation and boundaries

- Product CSV: signed upload, atomic error repair, aggregate preview, server receipt and CSV export. Files stay in memory. An uncertain commit retains its selected bytes and permits only explicit same-file retry. Product names/per-row success statuses are not manufactured from the CSV.
- Inventory: signed list, availability filters, persisted server search, selected-SKU draft, required reason, review, discard/continue, conflict reload, result and immutable UNKNOWN recovery. Server quantity/version checks remain authoritative. Draft before/after arithmetic is display-only; the command sends delta, reason and expected version. Auxiliary read-only price/photo/purchase-entry facts remain in an explicit product disclosure; no product edit is restored to inventory.
- Collections: signed list and quick creation (manual/automatic), current counts/status, real existing created-at rules, manual editor/membership/image/hide/delete preserved. Automatic defaults to the signed 7-day rule; Go owns membership. No manual membership writes for automatic collections. Tag/source rules and collection ordering cannot be invented.
- Customer CSV: four entry mapping controls, real native file chooser, safe preview/counts/row verdicts, explicit commit, result, errors, stale re-confirmation, lost-reply recovery. Uploaded cells/source IDs never appear in the preview or storage. Original file/mapping/count are retained through UNKNOWN; no automatic replay or consent grant. Existing historical-order archive remains a separate post-receipt disclosure.
- New namespace provides zh-TW/zh-CN/en for every new screen. Canonical K2 existing-entry translations are reused where present; new customer-state translations are implementation translations of signed Traditional Chinese, not a claim of owner-signed translated frames. Existing validation/advanced-control translations remain in import-copy, import-view-copy, copy and catalog-v2-copy.

## Static checks and exact limits

| Command | Exit | Evidence / limit |
|---|---:|---|
| `node_modules/.bin/tsc -p output/parity-data-ops/tsconfig.static.json --pretty false` | 0 | `implementation-typecheck.log`; all changed components, direct imports, and customer/inventory/parity specs; shared installed dependencies read-only |
| `node --check` on six changed browser/harness files | 0 | Syntax only; no tests executed |
| `python3 scripts/check_packet.py` | 0 | `implementation-packet.log`; architecture structure only; generated timestamp-only packet change restored |
| `node scripts/dev/derive-browser-ownership.mjs`, then `--check` | 0 | Generated ownership committed with this change |
| `bash scripts/dev/depmap.sh` | 0 | Generated dependency map has no diff; no Go source change |
| `git diff --check` | 0 | Whitespace only |

No local tests or servers. The full-app static attempt used mismatched shared workspace package links; targeted config resolves this worktree's package source explicitly. Adding catalog-core.spec to that static closure additionally requires uninstalled `axe-core`; its changed selector is syntax-checked and its full type/runtime gate is delegated to CI. No packages/locks were changed.

## Legacy → signed test mapping (business checks retained)

| Old entry/control | New real user path | Preserved oracle |
|---|---|---|
| Product import old heading / 檢查檔案 | 商品匯入與匯出 / 預覽檔案 | Server preview, atomic refusal, persisted product receipt |
| Inventory radio/tray / 商品與庫存 heading | 庫存 → SKU 調整庫存 → 核對調整 → 確認調整 | Same stock delta, key/body equality, reload recovery, storage refusal, conflict and permission checks |
| Inline inventory price/photo/purchase helpers | Adjust → product disclosure | Same read-only price, actual image, status and guarded purchase-entry checks |
| Inventory Add product | 返回商品 → existing Add product entry | Full editor route; no inline creation panel |
| Collection full-editor entry | 管理手動集合 | Existing image, ordered membership, save/hide/delete and buyer visibility checks |
| Customer kind/step picker | Direct file/mapping → safe preview → explicit confirm | Same file bytes/mapping/count, rollback, erasure, PII/consent/storage/session and CSV checks |
| Customer orders selector on entry | Post-customer-receipt archive disclosure | Archive import, no money/revenue mutation, paging/retry/persistence checks remain |

## Required facts across desktop/mobile/review

| Module | Desktop facts | Mobile / review / receipt |
|---|---|---|
| Product CSV | Filename, file/row limits, server error row/column/reason, atomicity | Same source facts; aggregate preview and real receipt counts; per-row product title unavailable (DEFERRED) |
| Inventory | SKU/name, warehouse, on-hand/reserved/allocated/unavailable/available | Same balances and delta/reason across draft/review; result only after server receipt; conflict needs new snapshot; UNKNOWN retains command |
| Collections | Title, mode, existing rule, product count, status | Same returned metadata; create retains name/type and enters managed products; no fabricated tags/order/ranks |
| Customer CSV | Filename, requested columns, omitted-field preservation, consent boundary, safe row count | Same original request through review, stale and UNKNOWN; receipt counts only from server; error required-field specificity unavailable |

Click ledger: C6 specs write one control/action/expected/actual ledger into GitHub browser artifacts. Existing migration-import ledger and inventory suites retain their business assertions. This session has no local click acceptance; actual result is **NOT_RUN**, not author PASS.

## DEFERRED / BLOCKED

| Frames / fact | Exact current limitation |
|---|---|
| 6:22818 / 6:22986, historical ledger and “查看流水” controls | No scoped historical GET returning timestamp, actor, source, reason, before/after balances. Existing catalog-ledger returns current balances. No fake rows or dead ledger controls. |
| 5:15211 / 5:15352, 最近任務 rows | Product CSV response has no jobs/list or past-task history. No local historical task database fabricated. |
| 30:1441 / 30:1528, named per-product preview rows | ImportResult returns aggregate counts and errors, not `rows[].product_name/outcome/description`. Aggregate preview works; those cells are deferred. |
| 6:19278 / 6:19486, tag/source rules and collection drag ordering | Existing automatic rule contract only supports created_at/within_days; collection rows do not expose a collection-position write contract. Created-at rules are supported; no tag/source/rank fabricated. |
| 1028:7087 / 1028:7165 | Prototype-only demo CSV chooser excluded; native CSV selection implemented instead. |
| 1028:7221 / 1028:7299, source-specific required error | **BLOCKED:** `rows[].code=required` means either missing external_id (customers_csv.go:229) or missing name (:238). There is no safe `rows[].field`. The RED spec asserts `第 3 筆 · 缺少來源編號` and its specific repair text; that is not provable from the DTO. Generic required reason is shown. RED unchanged. Integrator must authorize deferring that assertion or supply an additive non-PII field with its own RED Go gate. Asynchronous ruling requested; no answer assumed. |

No frame is listed “Not exported” in signed FRAME-COPY (44/44 found). The above are specific API/prototype limits, not absent Figma files. P0/P1 or failed CI cannot merge; independent frame comparison is still required. No claim that all 44 frames are production-ready.

## UI process — BOURDIEU

Q1 task names use seller words; Q2 experienced users enter the task directly; Q3 checkout fee/payment timing N/A; Q4 pickup N/A; Q5 recent task experience is never inferred from device and both viewports retain required facts; Q6 aesthetic preference UNKNOWN until owner. Comprehension, time/error studies NOT_RUN.

## UI process — PROJECT UNDERSTANDING

架构 §§0/1/2 impose evidence labels and Go/PG authority; §§26/27 impose scoped ownership, independent acceptance and no recursive delegation. Invariants I01/I02/I03/I06/I09/I11 preserve server scope, immutable replay, stock arithmetic authority, tenant separation and privacy. storefront-v2 §A/G2 governs membership/atomic CSV; automatic amendment restricts actual rules; migration-import §§1–4 forbids consent grants, identity merge and raw-row echo.

## UI process — MOBBIN → EAGLE

No new visual design/assets were authored. Implemented signed structure directly. Existing K2 references (folder `MVS1D5NC7KI5T`; Shopify `MVS1D603YAUS5`, Square `MVS1D5ZMW22YA`) inform grouping and selected-object-before-action only; no images, brands or product facts copied. Their prior Eagle metadata was read via K2 delivery; live Eagle/Mobbin access not claimed this session. New import query: none.

## UI process — FIGMA OFFICIAL GUIDANCE

No Figma nodes, components, variables or reactions changed. Fleet Bridge README followed for read-only inspection; existing auto-layout/variable/component source retained as the visual authority. The referenced `output/ux-audit-ref/FIGMA-GUIDE.md` is absent in both this worktree and main checkout; no fabricated official-guide read. K2 current delivery records the absent-guide exception at its §FIGMA OFFICIAL GUIDANCE. Copy lint for changed Figma roots: N/A (zero changed roots). Runtime responsive parity and density probe: CI/independent review NOT_RUN.

## Files outside literal component write paths

- `tests/admin/catalog-core.spec.ts`, `ledger.spec.ts`, `import-wizard.spec.ts`: authorized pre-Figma real-click path mapping; no tests removed.
- `tests/admin/parity-customer-import.spec.ts`: allowed wrong-selector correction scopes the unchanged count of four to the actual wizard (shell has two additional comboboxes).
- `tests/admin/import-coordinator.test.mjs`: required import/closure injection updated; immutable-replay and lifecycle assertions retained.
- `tests/storefront/merchant-product.mjs`, `catalog-media-gate.mjs`: shared browser helpers/assertions follow changed inventory search/disclosure; same product facts/price/image/archived checks retained. These two shared files require integrator review/approval.
- Per-namespace `packages/i18n/src/data-ops.ts`, generated browser ownership and own output evidence are allowed card additions. No protected baseline, dispatcher, OpenAPI, migration, Go source, lockfile or other app source changed.

## New-screen copy table

All new namespace strings below; dynamic product names/warehouses/filenames/numbers are server facts, not translations. Inherited advanced fields and error dictionary retain their existing three-locale source modules named above.

| Key | zh-TW | zh-CN | en |
|---|---|---|---|
| productTitle | 商品匯入與匯出 | 商品导入与导出 | Product import and export |
| productUpload | 匯入商品 | 导入商品 | Import products |
| productLimits | UTF-8 CSV · 最大 2 MB · 1,500 筆資料 · 每列一個規格 | UTF-8 CSV · 最大 2 MB · 1,500 行数据 · 每行一个规格 | UTF-8 CSV · Up to 2 MB · 1,500 rows · One variant per row |
| productFile | 商品資料檔案 | 商品数据文件 | Product data file |
| productRule | 商品以 handle、規格以 SKU 比對。圖片不匯入；集合需先建立。 | 商品按 handle、规格按 SKU 匹配。图片不会导入；需要先创建集合。 | Match products by handle and variants by SKU. Images are not imported; create collections first. |
| productStock | 庫存填目標現有數量；留空不變。檢查無誤後才整批匯入。 | 库存填写目标现有数量；留空则保持不变。检查无误后再整批导入。 | Enter target on-hand stock; leave blank to keep it. Review before importing the batch. |
| previewFile | 預覽檔案 | 预览文件 | Preview file |
| exportProduct | 匯出商品 CSV | 导出商品 CSV | Export product CSV |
| previewProduct | 預覽商品匯入 | 预览商品导入 | Preview product import |
| productPreviewRule | 先檢查檔案，沒有錯誤後才整批匯入。錯誤未修正前不寫入。 | 先检查文件，没有错误后再整批导入。修正错误前不会写入。 | Check the file before importing the batch. Nothing is written until all errors are fixed. |
| productErrors | 修正商品匯入資料 | 修正商品导入数据 | Fix product import data |
| productInvalidRows | {n} 筆資料待修正；整批尚未寫入，沒有部分商品已成功的情況。 | {n} 行数据待修正；整批尚未写入，不存在部分商品已导入的情况。 | {n} rows need correction. The entire batch is unwritten; no products were partially imported. |
| noBatch | 本批次未寫入 | 本批次未写入 | This batch was not written |
| repairCSV | 請修改原始 CSV，再重新上傳並預覽。確認所有問題修正後，才能提交整批資料。 | 请修改原始 CSV，再重新上传并预览。所有问题修正后才能提交整批数据。 | Edit the original CSV, upload it again and preview it. Fix every issue before submitting the batch. |
| priceField | 售價 | 售价 | Price |
| collectionField | 系列 | 系列 | Collection |
| missingPrice | 新增商品規格缺少售價。 | 新增商品规格缺少售价。 | The new product variant needs a price. |
| priceRepair | 在 price 欄補上售價，再重新預覽。 | 在 price 列填入售价，再重新预览。 | Fill in price in the original CSV, then preview again. |
| missingCollection | 指定的系列不存在。 | 指定的系列不存在。 | The specified collection does not exist. |
| collectionRepair | 在 collections 欄改用已存在的系列名稱。 | 在 collections 列改用已有的系列名称。 | Use an existing collection name in the collections column. |
| reupload | 重新上傳並預覽 | 重新上传并预览 | Upload and preview again |
| productResult | 匯入結果 | 导入结果 | Import result |
| importDone | 匯入完成 | 导入完成 | Import completed |
| newProducts | 新增商品 | 新增商品 | Products created |
| updatedProducts | 更新商品 | 更新商品 | Products updated |
| stockAdjustments | 庫存調整 | 库存调整 | Stock adjustments |
| productTotals | {n} 筆資料已處理 · 失敗 {failed} · 不變 {same} | 已处理 {n} 行数据 · 失败 {failed} · 未变 {same} | {n} rows processed · {failed} failed · {same} unchanged |
| productRows | {file} · {n} 筆資料 | {file} · {n} 行数据 | {file} · {n} rows |
| importHome | 回到匯入中心 | 返回导入中心 | Back to import center |
| viewProducts | 查看商品 | 查看商品 | View products |
| customerImport | 顧客匯入 | 客户导入 | Customer import |
| recent | 最近任務 | 最近任务 | Recent tasks |
| file | 檔案 | 文件 | File |
| type | 類型 | 类型 | Type |
| status | 狀態 | 状态 | Status |
| count | 筆數 | 行数 | Rows |
| row | 第 {n} 筆資料 · {field} | 第 {n} 行数据 · {field} | Row {n} · {field} |
| inventory | 庫存 | 库存 | Inventory |
| backProducts | 返回商品 | 返回商品 | Back to products |
| all | 全部 | 全部 | All |
| availableOnly | 有可售 | 有可售 | Available |
| soldOut | 缺貨 | 缺货 | Out of stock |
| searchSKU | 搜尋貨號 | 搜索货号 | Search SKU |
| inventorySearch | 搜尋庫存 | 搜索库存 | Search inventory |
| skuOrName | 貨號或商品名稱 | 货号或商品名称 | SKU or product name |
| search | 搜尋 | 搜索 | Search |
| allStock | 全部庫存 | 全部库存 | All inventory |
| skuProduct | 貨號 / 商品 | 货号 / 商品 | SKU / Product |
| onHand | 現有 | 现有 | On hand |
| reserved | 保留 | 保留 | Reserved |
| allocated | 已分配 | 已分配 | Allocated |
| unavailable | 不可用 | 不可用 | Unavailable |
| available | 可售 | 可售 | Available |
| action | 操作 | 操作 | Action |
| adjust | 調整庫存 | 调整库存 | Adjust inventory |
| quantity | 調整件數 | 调整件数 | Adjustment quantity |
| quantityHelp | 正數增加現有庫存，負數減少；保留、已分配及不可用的數量不會被釋放。 | 正数增加现有库存，负数减少；保留、已分配和不可用的数量不会被释放。 | Positive numbers add stock; negative numbers reduce it. Reserved, allocated and unavailable stock stays held. |
| increase3 | 增加 3 | 增加 3 | Add 3 |
| decrease3 | 減少 3 | 减少 3 | Remove 3 |
| beforeAfter | 調整前 → 調整後 | 调整前 → 调整后 | Before → After |
| reasonRequired | 調整原因（必填） | 调整原因（必填） | Reason (required) |
| notSelected | 尚未選擇 | 尚未选择 | Not selected |
| restock | 到貨補錄 | 到货补录 | Stock received |
| recount | 盤點差異 | 盘点差异 | Stocktake difference |
| stockEquation | 可售＝現有－保留－已分配－不可用。數量為 0 不代表沒有這項商品。 | 可售＝现有－保留－已分配－不可用。数量为 0 不代表没有这件商品。 | Available = on hand − reserved − allocated − unavailable. Zero stock does not mean the product is missing. |
| stockUpdatedHelp | 現有與可售數量已更新；保留、已分配及不可用的數量不會被釋放。 | 现有与可售数量已更新；保留、已分配和不可用的数量不会被释放。 | On-hand and available quantities have been updated. Reserved, allocated and unavailable stock stays held. |
| currentStock | 目前庫存 | 当前库存 | Current inventory |
| reviewAdjust | 核對調整 | 核对调整 | Review adjustment |
| reviewTitle | 核對庫存調整 | 核对库存调整 | Review stock adjustment |
| delta | 變動件數 | 变动数量 | Quantity change |
| reason | 調整原因 | 调整原因 | Adjustment reason |
| reviewHelp | 確認商品編號、倉庫與原因，再送出調整。 | 确认商品编号、仓库和原因，再提交调整。 | Check the product code, warehouse and reason before submitting. |
| latestHelp | 送出時仍會核對最新資料。若有人先更改庫存，請重新讀取後再核對。 | 提交时会核对最新数据。若库存已被更改，请重新读取后再核对。 | Stock is checked again on submission. If it has changed, reload it and review again. |
| backEdit | 返回編輯 | 返回编辑 | Back to editor |
| confirmAdjust | 確認調整 | 确认调整 | Confirm adjustment |
| adjusted | 庫存已調整 | 库存已调整 | Inventory adjusted |
| backStock | 返回庫存 | 返回库存 | Back to inventory |
| conflict | 庫存已被其他操作更新 | 库存已被其他操作更新 | Inventory was updated by another operation |
| conflictHelp | 本次內容尚未套用。請重新讀取庫存，再核對調整數量；不要直接重送舊內容。 | 本次内容尚未应用。请重新读取库存并核对调整数量，不要直接重发旧内容。 | This adjustment was not applied. Reload stock and review the quantity; do not resend the old adjustment. |
| reloadReview | 重新讀取並核對 | 重新读取并核对 | Reload and review |
| unknown | 調整結果仍待確認 | 调整结果仍待确认 | Adjustment result is unconfirmed |
| unknownHelp | 目前無法確認調整是否已完成。請先查看庫存紀錄或重新核對結果，不要再送出另一筆相同調整。 | 暂时无法确认调整是否完成。请先核对结果，不要另发一笔相同调整。 | The adjustment may have completed. Check the result before submitting another adjustment. |
| retryAdjust | 以同一請求重新確認 | 以同一请求重新确认 | Check the same request again |
| discardTitle | 放棄本次調整？ | 放弃本次调整？ | Discard this adjustment? |
| discardHelp | 離開會放棄這次尚未送出的數量與原因。庫存快照不會因此改變。 | 离开会放弃尚未提交的数量和原因，库存不会改变。 | Leaving discards the unsent quantity and reason. Inventory will not change. |
| continueHelp | 選擇「繼續編輯」會保留目前輸入。 | 选择“继续编辑”将保留当前输入。 | Choose Continue editing to keep your input. |
| continueEdit | 繼續編輯 | 继续编辑 | Continue editing |
| discard | 放棄調整 | 放弃调整 | Discard adjustment |
| cancel | 取消 | 取消 | Cancel |
| collections | 集合與分類 | 集合与分类 | Collections and categories |
| collectionsHelp | 手動集合與自動規則；刪除集合不刪除商品。 | 手动集合和自动规则；删除集合不删除商品。 | Manual collections and automatic rules; deleting a collection keeps its products. |
| createdDays | 創建 < {n} 天 | 创建 < {n} 天 | Created within {n} days |
| newCollection | + 新建集合 | + 新建集合 | + New collection |
| createCollection | 新建集合 | 新建集合 | New collection |
| name | 名稱 | 名称 | Name |
| manual | 手動 | 手动 | Manual |
| automatic | 自動 | 自动 | Automatic |
| collection | 集合 | 集合 | Collections |
| rules | 規則 | 规则 | Rules |
| productCount | 商品數 | 商品数 | Product count |
| order | 排序 | 排序 | Order |
| collectionSavedHelp | 保存後進入商品排序；不自動發佈到首頁。 | 保存后进入商品排序；不会自动发布到首页。 | Save to arrange products; this does not publish to the homepage. |
| deleteCollectionHelp | 刪除集合只刪映射，不刪商品。 | 删除集合只移除关联，不删除商品。 | Deleting a collection removes membership, not products. |
| manageManual | 管理手動集合 | 管理手动集合 | Manage manual collections |
| manualHelp | 手動：人工挑選與排序。 | 手动：手工选择和排序。 | Manual: select and arrange products yourself. |
| customerTitle | 匯入顧客 | 导入顾客 | Import customers |
| customerSteps | 01 對照欄位 → 02 核對影響 → 03 查看結果 | 01 对应字段 → 02 核对影响 → 03 查看结果 | 01 Map columns → 02 Review changes → 03 View results |
| selectedFile | 選擇的檔案 | 选择的文件 | Selected file |
| changeFile | 更換檔案 | 更换文件 | Change file |
| customerLimits | CSV · UTF-8 · 最多 2 MiB、5,000 筆 | CSV · UTF-8 · 最多 2 MiB、5,000 行 | CSV · UTF-8 · Maximum 2 MiB, 5,000 rows |
| customerMap | 把 CSV 欄位對應到顧客資料 | 将 CSV 列对应到顾客资料 | Map CSV columns to customer details |
| customerMatch | 以本店的來源編號比對：已有的更新，新的才新增。 | 按本店的来源编号匹配：已有的更新，新的才新增。 | Match source IDs within this store: update existing customers and create new ones. |
| sourceLabel | 來源編號 · 必填 | 来源编号 · 必填 | Source ID · Required |
| nameLabel | 姓名 · 必填 | 姓名 · 必填 | Name · Required |
| phoneLabel | 電話 · 選填 | 电话 · 选填 | Phone · Optional |
| emailLabel | 電子郵件 · 選填 | 电子邮箱 · 选填 | Email · Optional |
| noEmail | 不匯入電子郵件 | 不导入电子邮箱 | Do not import email |
| keepEmail | 保留店內原有電子郵件 | 保留店内原有电子邮箱 | Keep existing email addresses |
| clearPhone | 空白電話會清除舊電話 | 空白电话会清除原电话 | Blank phone values clear existing numbers |
| beforeImport | 匯入前先知道 | 导入前须知 | Before importing |
| mappedHelp | 已對應的欄位以檔案為準，空白電話／信箱會清除舊值。選「不匯入」則保留原值。 | 已对应的字段以文件为准，空白电话或邮箱会清除原值。选择“不导入”则保留原值。 | Mapped columns replace existing values, including blanks. Leave a column unmapped to keep existing values. |
| noConsent | 不改變行銷同意 | 不改变营销同意 | Marketing consent stays unchanged |
| consentHelp | 匯入不授予行銷同意，也不會自動合併後來下單的買家。 | 导入不会授予营销同意，也不会自动合并之后下单的买家。 | Imports do not grant marketing consent or automatically merge later buyers. |
| previewCustomers | 預覽可套用資料 | 预览可应用数据 | Preview applicable data |
| backCustomers | 返回顧客 | 返回顾客 | Back to customers |
| reviewCustomers | 確認前，再核對一次 | 确认前，再核对一次 | Review once more before confirming |
| previewStep | 02 核對影響 · 預覽尚未寫入資料 | 02 核对影响 · 预览尚未写入数据 | 02 Review changes · Nothing has been written |
| customerTotals | 新增 {new} · 更新 {updated} · 不匯入 {failed} | 新增 {new} · 更新 {updated} · 不导入 {failed} | {new} new · {updated} updates · {failed} excluded |
| applyHelp | 只套用有效資料；不匯入的列不會建立或更新顧客。 | 只应用有效数据；排除的行不会创建或更新顾客。 | Only valid rows are applied. Excluded rows do not create or update customers. |
| updateImpact | 這次會如何更新舊資料 | 本次如何更新原有资料 | How existing details will change |
| consentSummary | 行銷同意維持未知；不自動合併買家。 | 营销同意仍为未知；不自动合并买家。 | Marketing consent remains unknown; buyers are not automatically merged. |
| backMapping | 返回調整欄位 | 返回调整字段 | Back to column mapping |
| rowOutcomes | 每筆處理方式 | 每行处理方式 | How each row is handled |
| privateRows | 以資料筆數定位原檔，不顯示姓名、電話或來源編號。 | 按数据行数定位原文件，不显示姓名、电话或来源编号。 | Use row numbers to locate the original data. Names, phone numbers and source IDs stay private. |
| willCreate | 第 {n} 筆 · 將新增 | 第 {n} 行 · 将新增 | Row {n} · Will create |
| willUpdate | 第 {n} 筆 · 將更新 | 第 {n} 行 · 将更新 | Row {n} · Will update |
| willSkip | 第 {n} 筆 · 不匯入：{reason} | 第 {n} 行 · 不导入：{reason} | Row {n} · Excluded: {reason} |
| missingSource | 缺少來源編號 | 缺少来源编号 | Missing source ID |
| viewReasons | 查看不匯入原因 | 查看排除原因 | View exclusion reasons |
| recheck | 提交前會再次核對 | 提交前会再次核对 | Checked again before submission |
| recheckHelp | 若可套用筆數已改變，這次不會寫入；必須重新預覽並由你確認。 | 若可应用行数变化，本次不会写入；需重新预览并由你确认。 | If the applicable count changes, nothing is written. Review and confirm the new preview. |
| confirmImport | 確認匯入 {n} 筆 | 确认导入 {n} 行 | Confirm import of {n} rows |
| confirm | 確認 | 确认 | Confirm |
| customerResult | 顧客匯入結果 | 顾客导入结果 | Customer import result |
| importedRows | 已匯入 {n} 筆 | 已导入 {n} 行 | {n} rows imported |
| receiptTotals | 新增 {new} · 更新 {updated} · 未匯入 {failed} | 新增 {new} · 更新 {updated} · 未导入 {failed} | {new} created · {updated} updated · {failed} excluded |
| remaining | 仍有資料未匯入 | 仍有数据未导入 | Some rows were not imported |
| repairCustomer | 回原始 CSV 修正，再選擇修改後的檔案重新預覽。已匯入的來源編號會更新，不會再建立第二位顧客。 | 请回原始 CSV 修正，再选择修改后的文件预览。已导入的来源编号会更新，不会重复创建顾客。 | Correct the original CSV and preview the updated file. Existing source IDs update the same customer. |
| returnCustomers | 回顧客 | 返回顾客 | Back to customers |
| anotherFile | 匯入另一份檔案 | 导入另一个文件 | Import another file |
| pendingTitle | 尚未確認匯入結果 | 尚未确认导入结果 | Import result is unconfirmed |
| pendingSubtitle | 連線回覆中斷 · 請保留這一次請求 | 连接回复中断 · 请保留本次请求 | Response interrupted · Keep this request |
| dontResubmit | 先不要換檔或重新提交 | 请勿更换文件或重新提交 | Do not change files or submit again yet |
| pendingHelp | 系統可能已經處理。請用原檔案、原欄位對照與原確認筆數重試，取得同一次回執。 | 系统可能已处理。请用原文件、原字段对应和原确认行数重试，获取同一次回执。 | The import may have completed. Retry the original file, mapping and confirmed count to recover the same receipt. |
| retainedRequest | 已保留的請求 | 已保留的请求 | Retained request |
| retryImport | 重試同一次匯入 | 重试同一次导入 | Retry the same import |
| retryStale | 重試同一份匯入 | 重试同一份导入 | Retry the original import |
| staleUnknown | 匯入結果尚未確認，可用下方按鈕重試同一份匯入；請勿更換檔案或設定。 | 导入结果尚未确认，请用下方按钮重试同一份导入；请勿更换文件或设置。 | The result is still unconfirmed. Retry the original import below; keep the file and settings unchanged. |
| latestPreview | 最新預覽 | 最新预览 | Latest preview |
| staleHelp | 這次沒有匯入任何資料。從你預覽到按下確認之間，店內顧客資料有變動，所以數量已重新計算。 | 本次未导入数据。预览后店内顾客资料发生变化，数量已重新计算。 | Nothing was imported. Store data changed after your preview, so the counts were recalculated. |
| customerErrors | 回原檔修正這一筆 | 回原文件修正这一行 | Fix this row in the original file |
| sourceError | 第 {n} 筆 · 缺少來源編號 | 第 {n} 行 · 缺少来源编号 | Row {n} · Missing source ID |
| sourceRepair | 在原檔的 source_id 欄位填入穩定且唯一的來源編號。不要填電話或電子郵件。 | 在原文件 source_id 列填入稳定且唯一的来源编号。不要填写电话或电子邮箱。 | Fill in a stable, unique source_id in the original file. Do not use a phone number or email address. |
| errorPrivacy | 錯誤清單只保留列號與原因；不在這裡顯示失敗列的個人資料。 | 错误列表只保留行号和原因，不显示失败行的个人资料。 | The error list contains only row numbers and reasons, with no personal details. |
| backPreview | 返回預覽 | 返回预览 | Back to preview |
| correctedFile | 選擇修正後檔案 | 选择修正后文件 | Choose corrected file |

## Parity sheet — implementation CI captures NOT_RUN

Paths below are targets, not claims that CI captures exist. Entry captures occur before the signed assertion, exposing the real legacy page in RED. Later state captures exist only if their click path reaches that state. Automatic Playwright failure screenshots/traces remain enabled.

| Frame | Real-route CI screenshot | Signed reference PNG from Fleet Bridge | State / limitation |
|---|---|---|---|
| `5:15211` | `parity/5-15211.png` | `figma/get_screenshot-5-15211-1793511922329-0.png` | Admin Desktop · 商品匯入與匯出 · SHOPLINE · 1440×900px |
| `5:15352` | `parity/5-15352.png` | `figma/get_screenshot-5-15352-1793511922571-0.png` | Admin Mobile · 商品匯入與匯出 · SHOPLINE · 390×844px |
| `30:1441` | `parity/30-1441.png` | `figma/get_screenshot-30-1441-1793511923021-0.png` | Admin Desktop · 預覽商品匯入 · SHOPLINE · 1440×900px |
| `30:1528` | `parity/30-1528.png` | `figma/get_screenshot-30-1528-1793511923321-0.png` | Admin Mobile · 預覽商品匯入 · SHOPLINE · 390×844px |
| `30:1574` | `parity/30-1574.png` | `figma/get_screenshot-30-1574-1793511923787-0.png` | Admin Desktop · 匯入結果 · SHOPLINE · 1440×900px |
| `30:1635` | `parity/30-1635.png` | `figma/get_screenshot-30-1635-1793511924044-0.png` | Admin Mobile · 匯入結果 · SHOPLINE · 390×844px |
| `750:3456` | `parity/750-3456.png` | `figma/get_screenshot-750-3456-1793511924483-0.png` | Admin Desktop · 匯入錯誤明細 · SHOPLINE · 1440×900px |
| `750:3598` | `parity/750-3598.png` | `figma/get_screenshot-750-3598-1793511924824-0.png` | Admin Mobile · 匯入錯誤明細 · SHOPLINE · 390×844px |
| `6:19050` | `parity/6-19050.png` | `figma/get_screenshot-6-19050-1793511925268-0.png` | Admin Desktop · 調整庫存 · 選中貨號 · 1440×900px |
| `6:19202` | `parity/6-19202.png` | `figma/get_screenshot-6-19202-1793511925605-0.png` | Admin Mobile · 調整庫存 · 選中貨號 · 390×844px |
| `6:22818` | DEFERRED | `figma/get_screenshot-6-22818-1793511926003-0.png` | Historical inventory rows need a scoped ledger GET with timestamp, actor, source, reason and before/after quantities; current catalog-ledger returns balances only. |
| `6:22986` | DEFERRED | `figma/get_screenshot-6-22986-1793511926336-0.png` | Same missing historical inventory read contract as 6:22818. |
| `21:480` | `parity/21-480.png` | `figma/get_screenshot-21-480-1793511926755-0.png` | Admin Desktop · 庫存 · 選中貨號 · 1440×900px |
| `21:899` | `parity/21-899.png` | `figma/get_screenshot-21-899-1793511927005-0.png` | Admin Mobile · 庫存 · 選中貨號 · 390×844px |
| `840:5246` | `parity/840-5246.png` | `figma/get_screenshot-840-5246-1793511927450-0.png` | Admin Desktop · 庫存 / review · 1440×900px |
| `840:5247` | `parity/840-5247.png` | `figma/get_screenshot-840-5247-1793511927697-0.png` | Admin Mobile · 庫存 / review · 390×844px |
| `840:5248` | `parity/840-5248.png` | `figma/get_screenshot-840-5248-1793511928145-0.png` | Admin Desktop · 庫存 / result · 1440×900px |
| `840:5249` | `parity/840-5249.png` | `figma/get_screenshot-840-5249-1793511928374-0.png` | Admin Mobile · 庫存 / result · 390×844px |
| `840:5250` | `parity/840-5250.png` | `figma/get_screenshot-840-5250-1793511928736-0.png` | Admin Desktop · 庫存 / conflict · 1440×900px |
| `840:5251` | `parity/840-5251.png` | `figma/get_screenshot-840-5251-1793511929048-0.png` | Admin Mobile · 庫存 / conflict · 390×844px |
| `840:5252` | `parity/840-5252.png` | `figma/get_screenshot-840-5252-1793511929412-0.png` | Admin Desktop · 庫存 / unknown · 1440×900px |
| `840:5253` | `parity/840-5253.png` | `figma/get_screenshot-840-5253-1793511929683-0.png` | Admin Mobile · 庫存 / unknown · 390×844px |
| `840:5254` | `parity/840-5254.png` | `figma/get_screenshot-840-5254-1793511930053-0.png` | Admin Desktop · 庫存 / search · 1440×900px |
| `840:5255` | `parity/840-5255.png` | `figma/get_screenshot-840-5255-1793511930310-0.png` | Admin Mobile · 庫存 / search · 390×844px |
| `840:5256` | `parity/840-5256.png` | `figma/get_screenshot-840-5256-1793511930770-0.png` | Admin Desktop · 庫存 / discard · 1440×900px |
| `840:5257` | `parity/840-5257.png` | `figma/get_screenshot-840-5257-1793511931065-0.png` | Admin Mobile · 庫存 / discard · 390×844px |
| `6:19278` | `parity/6-19278.png` | `figma/get_screenshot-6-19278-1793511931612-0.png` | Admin · 集合分类 · SHOPLINE · 1440×900px |
| `6:19486` | `parity/6-19486.png` | `figma/get_screenshot-6-19486-1793511931845-0.png` | Admin Mobile · 集合分类 · SHOPLINE · 390×844px |
| `6:18298` | `parity/6-18298.png` | `figma/get_screenshot-6-18298-1793511932301-0.png` | Admin Desktop · 顧客匯入 · SHOPLINE · 1440×900px |
| `6:18486` | `parity/6-18486.png` | `figma/get_screenshot-6-18486-1793511932617-0.png` | Admin Mobile · 顧客匯入 · SHOPLINE · 390×844px |
| `1028:6685` | `parity/1028-6685.png` | `figma/get_screenshot-1028-6685-1793511933025-0.png` | Admin Desktop · 顧客匯入 · preview · 1440×900px |
| `1028:6763` | `parity/1028-6763.png` | `figma/get_screenshot-1028-6763-1793511933287-0.png` | Admin Mobile · 顧客匯入 · preview · 390×844px |
| `1028:6819` | `parity/1028-6819.png` | `figma/get_screenshot-1028-6819-1793511933715-0.png` | Admin Desktop · 顧客匯入 · result · 1440×900px |
| `1028:6897` | `parity/1028-6897.png` | `figma/get_screenshot-1028-6897-1793511934034-0.png` | Admin Mobile · 顧客匯入 · result · 390×844px |
| `1028:6953` | `parity/1028-6953.png` | `figma/get_screenshot-1028-6953-1793511934448-0.png` | Admin Desktop · 顧客匯入 · pending · 1440×900px |
| `1028:7031` | `parity/1028-7031.png` | `figma/get_screenshot-1028-7031-1793511934701-0.png` | Admin Mobile · 顧客匯入 · pending · 390×844px |
| `1028:7087` | DEFERRED | `figma/get_screenshot-1028-7087-1793511935106-0.png` | Prototype-only demo CSV chooser; real native CSV file selection is covered instead. |
| `1028:7165` | DEFERRED | `figma/get_screenshot-1028-7165-1793511935368-0.png` | Prototype-only demo CSV chooser; real native CSV file selection is covered instead. |
| `1028:7221` | `parity/1028-7221.png` | `figma/get_screenshot-1028-7221-1793511935733-0.png` | Admin Desktop · 顧客匯入 · errors · 1440×900px |
| `1028:7299` | `parity/1028-7299.png` | `figma/get_screenshot-1028-7299-1793511936005-0.png` | Admin Mobile · 顧客匯入 · errors · 390×844px |
| `1198:10518` | `parity/1198-10518.png` | `figma/get_screenshot-1198-10518-1793511936445-0.png` | Admin Desktop · 顧客匯入 · stale · DRAFT-COPY · 1440×900px |
| `1198:10588` | `parity/1198-10588.png` | `figma/get_screenshot-1198-10588-1793511936729-0.png` | Admin Mobile · 顧客匯入 · stale · DRAFT-COPY · 390×844px |
| `1198:10633` | `parity/1198-10633.png` | `figma/get_screenshot-1198-10633-1793511937146-0.png` | Admin Desktop · 顧客匯入 · stale-unknown · DRAFT-COPY · 1440×900px |
| `1198:10678` | `parity/1198-10678.png` | `figma/get_screenshot-1198-10678-1793511937434-0.png` | Admin Mobile · 顧客匯入 · stale-unknown · DRAFT-COPY · 390×844px |
