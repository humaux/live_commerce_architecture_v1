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

## 2026-11-01 PR473 routes TESTS ONLY follow-up (current scoped status)

This section supersedes the earlier blanket DONE/next implementation statements for the three routes. UI implementation is not authorized in this round. Root must inspect actual corrected CI RED before separately authorizing implementation.

- task_id: `53ea9f21-sub-leftovers-routes` (parent `53ea9f21-3f3b-4626-87ab-42dc853507b9`)
- base_commit: `6560ccf587d30a22258fb47e685d19bb545c4afc`
- branch/worktree: `codex/pr473-routes`, `.worktrees/codex-pr473-routes`
- role: dispatched tests-only worker; model/effort: inherited, runtime identity UNKNOWN (no override requested).
- Write paths: `tests/admin/parity-leftovers.spec.ts` (export help/template list/new form blocks only), this report.
- Scope authority: owner human freeze `64775cc4-1d33-4dec-8278-bad15840ea17`; source index `59a3f7d3-f616-4d8a-9e5a-f5f57d57dca1`; root card `output/integrator/tools/cards/cxf-parity-leftovers.md`.
- Source evidence: root `output/integrator/tools/logs/pr473-figma-scope/`, six raw context/PNG pairs; all six PNGs viewed in this round. Read `figma-implement-design` and current visual rule `0e0fb93a-0344-4cc5-9f05-a93ce0e62c4e`. No Figma write/refetch, no UI or backend changes.

| Frame/source | Corrected acceptance |
| --- | --- |
| `808:5618/5619` help | Existing exact copy retained. Real Return click keeps store; observe all non-GET/HEAD, CSV requests and download events from help mount through return; expect none. |
| `5:15002`, TEXT `677:2857` | Primary desktop heading is `消息範本`, not the frame/breadcrumb name `消息模板`. Six row-scoped records retain name/version/kinds/public-safe/date/unavailable-edit checks. |
| `5:15135`, appbar `5:15140` | Mobile heading remains `消息模板`. Six semantic list cards assert exact combined name/version and public-safe/date plus types and unavailable edit. Scroll each field into view and hit-test clearance from dock. No record or field dropped. |
| `673:2847/2916` | Body heading `新增自訂範本`; mobile appbar additionally `新增範本`. All three review rules are one raw TEXT node: one whitespace-normalized exact assertion retains all three lines. |
| list/new navigation | Real Add and Cancel clicks keep authenticated store. GET metadata does not expose a body textbox/edit action. No writes during navigation. |
| initial form | Scroll lower checkboxes/review/footer clear of dock; synthetic name/id/body fill updates local preview; click Check remains on new form, has no direct publish/send control and emits no non-GET/HEAD request. This is local structural checking, not server publication or public-safe approval. |

Initial frame screenshots are captured before scrolling/filling, preserving the signed desktop/mobile initial viewport. Recipient/media/bulk blocks and the six original metadata fixture objects were checked byte-identical to base.

Commands actually run (all local static only):

- `node --version` -> exit 0, `v24.15.0`.
- `node --check tests/admin/parity-leftovers.spec.ts` -> exit 0.
- `git diff --check` -> exit 0.
- Python static comparison using `git show HEAD:tests/admin/parity-leftovers.spec.ts` and current file -> exit 0; unrelated three blocks and original metadata array identical.
- Initial source-text helper attempted `json.loads` on multi-line raw context -> exit 1, `JSONDecodeError: Extra data`; no file changed. Exact raw copy was subsequently read from existing FRAME-COPY/source PNGs instead. Initial optional uppercase `output/PARITY-LEFTOVERS` lookup -> exit 2 (absent); actual module reports use lowercase paths above.

Evidence level: E1 static syntax/diff only; author assessment E0. No local runtime, node --test, Go/PG/browser run, server, download, dependency install, push or external side effect initiated.

NOT_RUN / unresolved:

- Root must regenerate/check `output/ci-select-backend-browser/browser-ownership.json`; explicitly outside worker write ownership.
- Corrected actual route-404 RED and all selected browser modes on fixed SHA; full TypeScript semantic check; runtime no-side-effect observations; screenshot/scroll behavior; independent review: NOT_RUN.
- Narrow-permission matrix (help `orders:read`, list `inbox:reply`, new/Add `live:manage`) and three-locale coverage: unchanged server-auth/tenant fixtures/runner, NOT_RUN in this round.
- Real Figma local links/token mapping diff, six app/source screenshot comparisons: NOT_RUN; never claimed diff=0. No UI exists in this change.
- Full generic publishing remains BLOCKED by existing admin BFF restriction to `private_reply`, 280 characters, `public_safe=false`; no loosening of that boundary or synthetic publish success.
- UI implementation, production release, LIVE, real buyer/message/refund/advertising actions: NOT_RUN.
