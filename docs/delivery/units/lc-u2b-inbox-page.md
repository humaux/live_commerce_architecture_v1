# Unit LC-U2b — 訊息（Messenger/IG 收件匣）页面 + BuyerPanel（Codex）

状态：FROZEN（2026-10-07，integrator）。Base `r3/integration` `0242dc0f`。无迁移、无 Go 改动。
worktree `.worktrees/lc-u2b-inbox-page`，branch `unit/lc-u2b-inbox-page`。
先读：`docs/delivery/AGENT-PREAMBLE.md` → `AGENTS.md` → `docs/delivery/PROCESS.md` → 本文件 → `contracts/live-console-v1.md`
§3（3.3 发送规则、3.6 会话状态/已读/接管、3.7 客户关联）、§6（capability UI 规则）、§11 A8–A14、§12 隐私清单。

## Integrator 裁决（覆盖正文）
- 契约 §16 的 LC-U2 拆成两半：**LC-U2b（本单元）= 「訊息」页 + `BuyerPanel.tsx`**；LC-U2a（Codex-1，LC-U1 合并后）=
  直播控制台中栏 `CommentStream.tsx`、回复模式切换、规则提示，并在右栏复用本单元的 `BuyerPanel`。
  所以 `BuyerPanel` 的 props 只接 `{store, conversationId?|bundleId?}` + 回调，不依赖控制台状态。
- 后端 A8–A14 已合并且冻结（`internal/httpapi/inbox.go`、`inbox_send.go`；模板 `/message-templates`）。**不改 Go/迁移/OpenAPI**；
  发现后端缺口 → 写进 DELIVERY 的 BLOCKED，不自行补。
- 导航：`apps/admin/src/routes.ts` 已有空的 `messages` 组；新增 `apps/admin/src/features/messages/routes.ts`（权限 `inbox:read`）
  并在 `routes.ts` 加一行 spread（与 LC-U1 的 routes.ts 改动由 integrator 合并）。
- 不做自动化/LLM 文案；发送只允许自由文本或已发布模板（A12 `{text | template ref, expected_generation}`）。

## 范围
1. `/[locale]/messages`：会话列表（A8，filter 全部/待回覆/Messenger/Instagram/直播留言，cursor 分页 ≤50）+ 会话线程（A9，
   `before_seq` 往前翻）+ 打开即 A10 已读（仅 `inbox:reply` 时生效，`inbox:read`-only 不发）。
2. 接管/释放（A11，`expected_generation`；409 `takeover_changed` → 重读并提示）；接管状态与 6 h 过期提示按 §3.6。
3. 回复框（A12）：每平台字数上限（Messenger 2000 runes、IG 1000 bytes，客户端只做提示，服务器为准）；窗口关闭（409
   `window_closed`）、`capability` 等全部错误码映射三语文案；`Idempotency-Key` 每次提交新生成、重试复用；send_state 显示 §4.4。
   capability 非 `ok` 时发送控件禁用并显示原因；`review_required` 显示「僅測試帳號」徽章（§6 UI 规则）。
4. `BuyerPanel.tsx`（A13）：显示名、平台、第几次购买、claims、订单（有 `orders:read` 才显示）；客户关联（A14，
   `expected_version`；需 `customers:read`）。
5. BFF：在 `apps/admin/app/api/stores/[store]/[...resource]/route.ts` 允许表**只新增** A8–A14 的精确路径/方法/查询键；
   响应 `Cache-Control: no-store`；请求体/查询键严格白名单（参照 parcels 的 `validParcelDeleteQuery` 写法与已有测试）。
6. 三语（zh-TW/en/ja，跟随现有 copy 文件结构），桌面 1440 + 手机 390。

## 隐私 / 安全（P0，审查重点）
- DM 文本、显示名、PSID 不进 URL、`localStorage/sessionStorage/IndexedDB`、console.log、错误上报（§12 I11）。
- 页面隐藏（`visibilitychange` hidden / `pagehide`）清空内存中的线程文本；回到可见时重新经授权读取（参照 `orders-ui.spec.ts`
  MOU03 的做法与断言）。切店/登出时作废在途请求，旧响应不得重绘。
- 只显示服务器给的字段；不在客户端拼接 PII。

## 写入路径
`apps/admin/app/[locale]/messages/**`、`apps/admin/src/features/messages/**`、`apps/admin/src/routes.ts`（一行）、
`apps/admin/components/{Inbox.tsx,InboxThread.tsx,BuyerPanel.tsx}`、`apps/admin/lib/inbox-*.ts`、
`apps/admin/app/api/stores/[store]/[...resource]/route.ts`（只新增）、`tests/admin/inbox*.{spec,test}.ts`、
`tests/foundation/browser_inbox_ui_test.go`、`scripts/dev/test-local.sh` / `test-node.sh` / `docs/delivery/GATES.md` / `.github/workflows/gates.yml`（新增模式）、
`output/lc-u2b-inbox-page/DELIVERY.md`。

## 测试 / 门禁（红 → 绿）
- 新模式 `bash scripts/dev/test-local.sh --browser-inbox`：真实 Next + Go + PG + MOCK IdP（参照 `browser_merchant_orders_ui_test.go`
  的签名登录与 API 观察包装）；fake Graph 仅用于发送结果。真实点击：列表筛选 → 打开会话（已读）→ 接管 → 发送 → 释放；
  `inbox:read`-only 角色（含 `viewer` 无权进入，`live_operator` 可）；跨店 404；窗口关闭文案；capability 禁用；隐藏清空；
  leak 扫描：哨兵 DM 文本/名字/PSID 不在 URL、浏览器存储、BFF 日志。
- BFF 单测必须 import 真实 route handler 并用真实 `Request`（Next 16 的非 GET 请求都带 body stream）。
- 校准：`LC_INBOX_CALIBRATION=<名>` 注入一个真实缺陷（例如隐藏不清空线程）时 `--browser-inbox` 必须红在对应断言上，正常模式绿。
- 回归：`--browser-admin-shell`、`--browser-click-sweep`、`--browser-visual-lint`、`test-node.sh`、admin typecheck/build、`check-gates.sh`。
- 本机只跑 Node/typecheck；浏览器模式由 integrator 在 GitHub 跑（owner RAM 规则）。

## 证据 / 角色 / 依赖
BROWSER（MOCK）。Codex-4 实现；独立审查 Opus（隐私）+ K3。依赖：LC-B3/B4/B5 已合并（是）。与 LC-U1 并行（写入路径只在
`routes.ts` 一行与 BFF 允许表重叠，integrator 合并）。LC-U2a 在本单元与 LC-U1 都合并后开工。
