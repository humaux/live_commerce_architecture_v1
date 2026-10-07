<!-- Purpose: unit brief for W6-U2 — admin UI for the failed/UNKNOWN operations ledger (W6-05B) and the Meta ad-account
unbind + catalog feed URL entry (W6-06B). Depends on: internal/httpapi/operations.go, internal/httpapi/ads.go (unbind,
catalog-feed), contracts/external-operation-v1.md (Amendment W6-05B), contracts/meta-ads-v1.md (Amendment W6-06B).
Used by: Codex-3 implementer; integrator review. -->
# Unit brief — W6-U2 失敗台賬頁 + 廣告解綁 / 商品目錄 feed（admin UI）

Owner decisions 2026-10-07: 「失败台账：需要做」「广告解绑：可以做」. Role/permission UI (W6-04B) is optional and NOT in scope.

## Scope
1. **失敗台賬 page** (`GET/POST /v1/admin/stores/{store_id}/operations…`, exact DTO in output/w6-05b-operations-ledger/DELIVERY.md):
   list with state filter (FAILED / UNKNOWN / …), detail drawer, actions shown ONLY when the server capability says so:
   查詢 (query; 409/429 `query_limit` with Retry-After), 取消 (READY only; 409 `protective_operation` for pause/stop kinds —
   explain why it is refused: 「此操作是保護性操作（例如暫停廣告），不可取消」), 授權重試 (only registry kinds; UNKNOWN shows
   「結果未知，請先查詢」, 409 `reconcile_first`). Never show payloads, tokens or buyer PII. Show the business object link (order /
   claim bundle / conversation) when present.
2. **廣告設定 card**: 解除綁定廣告帳號 button with a confirm dialog explaining history is kept and active ads must be paused
   first; handle 409 `operations_in_flight` (list the in-flight items) and 409 `binding_in_use` (「仍有可能投放中的廣告，請先暫停」).
   商品目錄 feed URL card (`GET …/ads/catalog-feed`, ads:read) with copy-to-clipboard and how-to text for Meta Commerce Manager.
3. Three locales (zh-TW / en / zh-CN), 390 px no overflow, accessible labels.

## Rules
- Shell: do NOT add a second `nav: true` route to an existing nav group (it removes the group's `nav-<group>` button and
  breaks every browser spec — W3-U5 incident). Put the ledger under an existing area (e.g. settings/integrations) with
  `nav: false` + in-page link, or a new top-level group if clearly better; update tests/admin/shell-registry.test.ts.
- BFF allowlist + strict exact-key parsers + request validators as in other modules; no new npm deps.
- Dates: format with the shared `packages/format` helpers; compute any "today"/window in Asia/Taipei (`instantToTaipei`),
  never browser-local (MA09a incident).
- Tests: node unit tests (red→green) for parsers/validators/copy parity; a real-click Playwright spec + Go harness
  (`//go:build browser`, no duplicate helper names), registered as a test-local mode + GATES.md row.
