# PARITY-DATA-OPS — signed data-operation UI

Implement the signed product CSV, inventory, collection and customer CSV task screens on their real routes. Replace the old kind/step pickers and ledger layout while retaining server scope, atomicity, privacy-safe previews, optimistic versions and immutable request recovery. Base `4b8c04c7213025c173e260db6f9c81cb5b818c61` is the fetched integration ancestor; CATALOG-PEOPLE #363 is merged.

**E1 static structure only; author assessment E0. GREEN / independent parity acceptance NOT_RUN.** There is one known unsupported RED assertion below; do not merge until resolved and CI/review are green. No local tests/servers, production, merge or auto-merge.

## RED and validation

- Tests-only head `a2c5eeb2e1dda3e8aaad61ccec47978c02c0e1f3`: Gates `41282897989`; jobs `124780074650` (bundle:03) and `124780074941` (bundle:07). Downloaded browser artifacts prove failures on new signed headings/controls; existing migration 15 and catalog tests pass. `output/parity-data-ops/red.log` records exact assertions/artifact IDs.
- Allowed wrong-selector repair only: the expected FOUR customer mapping comboboxes are now scoped to the wizard; the global selector also included shared store/language controls. No signed copy or behavioral assertion weakened; count remains four.
- `tsc -p output/parity-data-ops/tsconfig.static.json`, six changed file `node --check`s, packet structure, generated browser ownership + `--check`, dependency-map generation and `git diff --check`: exit 0. Static source hashes: `output/parity-data-ops/static-manifest.json`.
- All verification is delegated to this PR's own CI. Full catalog-core static closure additionally needs uninstalled local axe-core; no dependencies or locks changed.

## Legacy → signed mapping

| Removed legacy heading/control | Signed replacement / preserved business assertion |
|---|---|
| 匯入與匯出商品 / 檢查檔案 | 商品匯入與匯出 / 預覽檔案; preview/refusal/receipt/persistence |
| 商品與庫存 heading and radio/tray workflow | 庫存 → SKU action → draft → review → confirm; same delta/key/body, recovery, conflict, storage/permission safety |
| Inventory photo/price/purchase helper surface | Explicit product disclosure; same real image/read-only price/status/purchase checks |
| Inline Add product | 返回商品 → existing full editor; no inline create |
| 分類 heading | 集合與分類 / + 新建集合; manual/automatic creation using current server API; existing manual editor preserved |
| Customer import type/step picker and orders selector on entry | Direct native file + four mappings + safe preview + explicit confirmation; archive enters only after a customer receipt |

Existing tests retain their business checks and fixtures. Current docs, three-language copy table, required-field desktop/mobile/review matrix, UI process sections and CI click-ledger contract: `output/parity-data-ops/DELIVERY.md`.

## BLOCKED / DEFERRED

- **BLOCKED source-specific customer required error (1028:7221 / 1028:7299):** existing `rows[].code=required` means either missing external_id (`internal/migrationimport/customers_csv.go:229`) or missing name (:238). No safe `rows[].field` exists. Product shows the truthful generic reason. The new RED still expects “第 3 筆 · 缺少來源編號” and its repair text. That assertion is unchanged; integrator must authorize deferring it or supply a separately RED-tested additive non-PII field. Ruling requested, no answer assumed.
- 6:22818 / 6:22986: no historical inventory rows/actor/source/before-after read contract; no fabricated history or dead controls.
- 5:15211 / 5:15352 recent-task rows: no product jobs/list API; no local fabricated history.
- 30:1441 / 30:1528 named row preview: DTO has aggregate counts/errors, no per-row product name/outcome/description; real aggregate preview retained.
- 6:19278 / 6:19486 tag/source rules and collection drag order: current automatic API supports created_at/within_days only; no collection-position write contract. Existing creation-time rules implemented; no fabricated tags/ranks.
- 1028:7087 / 1028:7165: prototype-only demo chooser intentionally excluded; actual native CSV picker implemented.
- FRAME-COPY says 44/44 exported, none Not exported. No prototype notices/switches copied into product UI. Exact translated-frame approval, runtime density, screenshot comparison, independent review and LIVE/SANDBOX provider verification remain NOT_RUN.

## Files outside literal component paths

- Allowed namespace: `packages/i18n/src/data-ops.ts`; generated `output/ci-select-backend-browser/browser-ownership.json`; own `output/parity-data-ops/*` evidence.
- Existing browser mappings: `tests/admin/catalog-core.spec.ts`, `ledger.spec.ts`, `import-wizard.spec.ts`; required closure/harness update: `tests/admin/import-coordinator.test.mjs`; allowed wrong-selector repair in `parity-customer-import.spec.ts`.
- **Integrator approval/review required for shared browser helpers:** `tests/storefront/merchant-product.mjs`, `tests/storefront/catalog-media-gate.mjs`. They only map inventory search/details to signed controls and retain the same price/image/archived/purchase assertions.
- No other app/backend, OpenAPI, schema/migration, Go dependency/lockfile or protected baseline/dispatcher changes. Dependency-map regenerated with no diff.

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

🤖 Generated with Codex (dispatched by Claude Code integrator).
