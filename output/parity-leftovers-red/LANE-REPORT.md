# PARITY-LEFTOVERS RED phase — tests-only commit

## Date
2026-11-01

## Lane
unit/parity-leftovers (integrator: opencode-go/qwen3.7-plus)

## Scope
ROUND 1: tests-only RED commit for older frames with no v2 equivalent:
- 訂單缺少收件資料 (750:3454 desktop, 750:3596 mobile)
- 訂單匯出說明 (808:5618 desktop, 808:5619 mobile)
- 媒体上传 (6:22038 desktop, 6:22147 mobile)
- 批量填充 (48:1277 mobile only)
- 消息模板 (5:15002 desktop, 5:15135 mobile)
- 新增自訂訊息範本 (673:2847 desktop, 673:2916 mobile)

## Changes
- Added `tests/admin/parity-leftovers.spec.ts` (tests/admin/parity-leftovers.spec.ts:1-250)
  - Signed-frame acceptance on real routes for all six subjects
  - zh-TW locale only (FRAME-COPY.md source is zh-TW)
  - Desktop (1440×900) and mobile (390×844) viewports
  - Parity screenshot capture per frame
  - Click ledger for real user operations
  - No product code changes

## Expected RED failures

| Test | Route | Class | Reason |
|------|-------|-------|--------|
| 訂單缺少收件資料 desktop/mobile | /orders/{id} | b | Route exists but does not show signed "請補齊收件資料" frame content |
| 訂單匯出說明 desktop/mobile | /orders/export | a | Route does not exist (404) |
| 媒体上传 desktop/mobile | /products/{id} | b | Route exists but media picker does not show signed "選擇要加入的圖片" frame |
| 批量填充 desktop/mobile | /products/{id} | b | Route exists but bulk fill dialog does not match signed frame copy |
| 消息模板 desktop/mobile | /messages/templates | a | Route does not exist (404) |
| 新增自訂訊息範本 desktop/mobile | /messages/templates/new | a | Route does not exist (404) |

Class a: route does not exist (404 or not found)
Class b: route exists but shows wrong/legacy content
Class c: assertion mismatch on visible copy (element present but text differs)

## Verification
- `node scripts/dev/derive-browser-ownership.mjs` — regenerated (55 modes, no change)
- `node scripts/dev/derive-browser-ownership.mjs --check` — exit 0, no drift
- Test file compiles (no syntax errors)
- Test will fail on assertions when run in CI (expected RED)

## Prototype notices excluded
Per task instructions, the following prototype notices from FRAME-COPY.md are NOT asserted:
- "此原型僅展示說明，不會下載檔案或將訂單改為已出貨。" (808:5618)
- "此原型僅展示說明，不會下載檔案或將訂單改為已發貨。" (808:5619)
- "互動原型：欄位為示例，檢查只示範規則，不會發布或傳送。" (673:2847, 673:2916)

## DEFERRED frames
None. All 11 frames listed in FRAME-COPY.md are in scope.

## NOT_RUN
- Local test execution (forbidden by AGENTS.md)
- CI run (only after push)
- Implementation/GREEN phase (next round)
- zh-CN and en locales (FRAME-COPY.md source is zh-TW only)
- Independent parity review (after CI)

## Next step
ONE implementation push (GREEN phase) to make the RED tests pass.

STATUS: DONE
