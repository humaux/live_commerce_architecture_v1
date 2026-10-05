# Unit W3-U3 — 关键字留言标签打印（Codex，纯 UI）

状态：DRAFT（2026-10-06，待 integrator 冻结）。Base `r3/integration` `bb71f966`。无迁移、无后端。
覆盖 IMPLEMENTATION-PLAN W3-U3（R5 波次 4 M29；v5「關鍵字留言可列印」）。worktree `.worktrees/w3-u3-comment-label-print`。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；合同只读 `contracts/live-console-v1.md` §7.4。

## Integrator 裁决（覆盖正文）
- 后端已就绪：`POST …/live-sessions/{sid}/comments/{comment_ref}/print`（`internal/httpapi/live_stream.go:55`，0123 `live.comment_prints`，body 恰为 `{}`）
  只记录「印过」事实。**标签内容只由浏览器从内存中的留言渲染，不落库、不发到服务器、不进 localStorage**（PII 最小化）。
- 打印用浏览器打印 CSS，不引入 PDF/条码库。

## 目标 / owner 流程
直播中，助播在留言流勾选关键字留言（或单条「列印」）→ 打印预览出标签（显示名、关键字、数量、时间、场次短码）→ 打印后留言显示「已印 ×N」。

## 范围
1. `CommentLabelPrint.tsx`（新）：接收已选留言数组（来自 LC-U2 的内存流），渲染标签网格；标签尺寸两档（60×40 mm、A4 三列），选择记在 `localStorage`
   （仅尺寸偏好，try/catch）。
2. 每条打印前调用 print 端点（每条一个 `Idempotency-Key`），成功后 `window.print()`；端点失败不阻止打印，但不显示「已印」。
3. `apps/admin/app/print.css`：`@media print` 隐藏控台其余部分，`@page` 尺寸按档位。
4. 留言流只加「勾选」与「列印」入口（`CommentStream.tsx` 最小改动）。

## Non-goals
条码/QR、热敏打印机驱动、服务器端渲染标签、历史留言打印（读穿流已滚出的留言不可打印）。

## 写入路径
`apps/admin/components/CommentLabelPrint.tsx`（新）、`apps/admin/app/print.css`（新）、`apps/admin/components/CommentStream.tsx`（只加入口）、
`tests/admin/comment-label-print.spec.ts`。

## 测试 / 门禁
扩展 `bash scripts/dev/test-local.sh --browser-live-console`（LC-U1 引入；若派发时不存在 → integrator 先加）：选 3 条 → `page.emulateMedia({media:'print'})`
断言只剩 3 个标签且内容正确 → print 端点各被调用一次 → 徽章「已印 ×1」；断言 `localStorage`/`sessionStorage` 无留言文本或显示名。
回归 `--browser-click-sweep`、`--browser-visual-lint`。

## 证据 / 角色 / 依赖
BROWSER（MOCK）；真打印机 NOT_RUN。Codex；K3。依赖 LC-U2 合并（`CommentStream.tsx`）；与 W3-U2 都改 `CommentStream.tsx` → 串行（谁先合并谁先）。
