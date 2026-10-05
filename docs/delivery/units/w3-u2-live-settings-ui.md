# Unit W3-U2 — 直播設定页：结账提醒 / 没货回复 / 限制名单（Codex）

状态：DRAFT（2026-10-06，待 integrator 冻结）。Base `r3/integration` `bb71f966`。无迁移。
覆盖 IMPLEMENTATION-PLAN W3-U2 的提醒、没货模板、限制名单三块；**匹配规则模拟器不在本单元**（W3-06B 未排入本批，待其冻结后另开 U2b）。
worktree `.worktrees/w3-u2-live-settings-ui`（branch `unit/w3-u2-live-settings-ui`）。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件 → `w3-03b-checkout-reminder.md`、`w3-04b-sold-out-reply.md`、`w3-05b-blocklist.md`（以冻结版 API 为准）。

## Integrator 裁决（覆盖正文）
- v5「直播設定」为界面目标。提醒说明文案必须写明 24 小时规则：「只有在 24 小時內傳過訊息給粉專的買家會收到提醒；其他買家會列在待跟進清單」。
- 没货回复说明必须写明「會用掉這則留言唯一一次私訊回覆」。
- 「加入限制名單」按钮放在买家面板（`BuyerPanel.tsx`，LC-U2 交付）→ 本单元在 LC-U2 合并后改该文件，只加按钮与「已限制」徽章。
- 店级默认匹配模式本轮不做（裁决 10）。

## 范围
1. `LiveSettings.tsx`（新）三个分区：
   - 结账提醒：开关 + 延迟分钟（10..1440）；场次页「提醒未付款」按钮与结果（已排队 N / 待跟進 M，followup 列表可「複製連結」）。
   - 没货回复：开关 + 模板（复用 LC-B5 模板发布接口，template `sold-out-reply/v1`）+ 预览。
   - 限制名单：列表（显示名、内部备注、加入时间）、移除（确认框）、分页。
2. 买家面板：「加入限制名單」（可选备注 ≤ 200 字）、名单内显示「已限制」；建单抽屉调用 `blocklist/check` 显示警告（不阻止）。
3. 控台评论标记显示 `reply_kind`：`sold_out` →「沒貨已回覆」，`restricted` →「已限制」。
4. `apps/admin/lib/live-settings-*.ts`；BFF 代理；三语。

## Non-goals
关键字工具/模拟器（W3-06B）、店级默认匹配模式、模板编辑器重做。

## 写入路径
`apps/admin/components/LiveSettings.tsx`（新）、`apps/admin/components/BuyerPanel.tsx`（只加按钮/徽章）、`apps/admin/components/CommentStream.tsx`（只加两个徽章）、
`apps/admin/lib/live-settings-*.ts`、`apps/admin/app/api/stores/**`（仅新增）、`apps/admin/src/features/live/routes.ts`（加一条路由）、
`tests/admin/live-settings.spec.ts`。

## 测试 / 门禁
新模式 `bash scripts/dev/test-local.sh --browser-live-settings`（integrator 加；MOCK 后端）：三分区真实点击保存、版本冲突提示、复制链接写剪贴板、
名单增删、买家面板按钮 → 徽章；`page.emulateMedia` 无关。回归 `--browser-live-claims`、`--browser-admin-shell`、`--browser-click-sweep`。
`test-node.sh`、typecheck/build、`check-gates.sh`。

## 证据 / 角色 / 依赖
BROWSER（MOCK）。Codex；K3。依赖：W3-03B、W3-04B、W3-05B 冻结并合并；LC-U2 合并（`BuyerPanel.tsx`/`CommentStream.tsx`）。
