# DELIVERY — unit lc-b5-templates（直播控制台消息模板 + 快捷回复：W2-05B，合同 §3.4/§3.5/§7.3 + §11 /message-templates）

- 角色: DeepSeek V4-Pro（后端执行者；不合并/不推送/不部署，integrator/Claude Opus 负责）。
- Base SHA: `8669637c47b110fe9dae038f7ca23221f4de2015`；worktree: `.worktrees/lc-b5-templates`；branch: `unit/lc-b5-templates`。
- 合同: `contracts/live-console-v1.md` W2-05B（§3.4 出站 kind、§3.5 公开回复内容规则、§7.3 推荐评论、§11 `/message-templates`；FROZEN 2026-10-05）。
- 迁移号: `0124`（占位；integrator 合并时重编号，同 LC-B3 的 0122→0119）。
- 状态: 后端实现 + DB-free 单测 + REAL_PG 门禁全绿；`--msg-templates` 门禁已登记。全仓 race、浏览器 UI、真实 Meta 流量均 NOT_RUN。

## 证据标签

| 证据 | 标签 |
|---|---|
| 本文件 / 迁移注释 / 源码注释 | DESIGN |
| `internal/msgtemplates` 与 `internal/httpapi` 单元测试 | UNIT（MODEL_ONLY） |
| `tests/foundation/live_console_templates_test.go`（一次性 PG 容器） | SANDBOX |
| `go build` / `go vet` / `gofmt` / gate-sync 人工核对 | 静态 |
| 全仓 `go test -race ./...`、浏览器 UI、真实 FB/IG 流量 | NOT_RUN |

## 交付物（唯一写入路径，均在合同清单内）

实现：
- `migrations/0124_msg_templates.sql` — 前向迁移（见「迁移校验」）。
- `internal/msgtemplates/service.go` — 无配置 `Service`/`NewService`（不持池、不持密钥；publish/list/resolve 均在调用方 scoped 事务内跑，权限由 0124 定义者按服务端 GUC 复核）。
- `internal/msgtemplates/models.go` — 冻结 kind 词汇（`dm`/`private_reply`/`public_reply`/`recommend`）、两个系统固定模板 id、publish/list/resolve 结构体。
- `internal/msgtemplates/errors.go` — 固定传输安全错误词表（`*Error{Status,Code}`、`ErrDatabase`、`databaseError` 只透传 PT403/404/409/422）。
- `internal/msgtemplates/publicsafe.go` — §3.5 单一共享校验器 `ValidatePublicSafe`（NFKC + case-fold + 去 Cf/空白 + 数字间横线折叠；拒 URL/裸域名/`t.me`/`wa.me`/`line id`/`@handle`/≥8 位数字/邮箱/买家变量/`{{連結}}` 等链接占位/store origin；NFC 长度 1..300）。
- `internal/msgtemplates/publish.go` — POST 发布：Go 结构校验 + §3.5 校验 + `command.Run`（`template.publish` 幂等 receipts）包 `msgtemplates.publish` 定义者。
- `internal/msgtemplates/list.go` / `resolve.go` — GET 列表（inbox:reply，各 id 最新版本）与 LC-B4 发送路径用的 `Resolve`（固定优先、再商家；inbox:reply OR live:manage）。
- `internal/httpapi/templates.go` — POST/GET `/v1/admin/stores/{store_id}/message-templates`（live:manage/inbox:reply），`registerTemplateRoutes`（svc 为 nil 不挂载）。
- `internal/httpapi/handler.go`（改）— `Options.MsgTemplates *msgtemplates.Service` + `registerTemplateRoutes` 接线。
- 测试：`internal/msgtemplates/publicsafe_test.go`、`internal/httpapi/templates_test.go`、`tests/foundation/live_console_templates_test.go`（4 个 REAL_PG 门禁）。
- `tests/foundation/live_console_inbox_test.go`（改）— 0119 `principal_holds` 授权者断言增补 `commerce_msgtemplates_writer`（0124 新增 EXECUTE，属合法同步非放宽）。
- `scripts/dev/test-local.sh`（改）— 登记 `--msg-templates` 门禁；`docs/delivery/GATES.md`（改）— 增行。

未触碰：`apps/**`、`go.mod`/`go.sum`、OpenAPI 共享 schema、pnpm 锁文件、`contracts/*`、`cmd/**`、`internal/inbox/**`（LC-B3）、LC-B4 发送路径实现（只交付 `Resolve` 供其调用）。

## 红→绿证据

- DB-free 单测 `go test ./internal/msgtemplates/ ./internal/httpapi/ -count=1` — **全绿**（exit 0）。见 `evidence-test.log`。
- REAL_PG 门禁 `bash scripts/dev/test-local.sh --msg-templates` — **PASS 4/4 exit 0**（0124 精确 ACL、publish 版本化+审计+幂等重放、§3.5 URL 拒绝/公开 kind 需 public_safe/固定 id PT409/权限拆分、resolve 固定+商家+跨店 RLS+定义者门禁）。见 `evidence-test.log`。
- 静态（`evidence-static.log`）：`go build ./...` exit 0；`go vet ./internal/msgtemplates/ ./internal/httpapi/` exit 0；`gofmt -l`（本单元文件）空。
- 红证据（`red.log`）：4 处真实 Red 失败均已针对性修复（email 判序、GET 空体 reader、缺 schema inbox USAGE、URL 拒绝断言计数未按 template_id 限定）。

## 迁移校验（0124）

- 前向、幂等：precondition DO 块校验 `inbox.principal_holds(text[])` 存在、`store_grants_permission_check` 含 `'inbox:reply'`、角色 `commerce_runtime`/`commerce_auth` 存在；固定模板 `ON CONFLICT DO NOTHING`。
- 角色/权限：`commerce_msgtemplates_writer` NOLOGIN（无 BYPASSRLS/CREATEDB/CREATEROLE/REPLICATION）；`msgtemplates.templates`/`fixed_templates` 均 `ENABLE + FORCE RLS`，表只对 writer 授 SELECT/INSERT（fixed 只 SELECT）。
- 定义者（owner `commerce_msgtemplates_writer`，EXECUTE 仅 `commerce_runtime`）：`publish`（VOLATILE，live:manage，版本自增，审计 `template.published`，固定 id→PT409）；`list`（STABLE，inbox:reply，各 id 最新版本）；`resolve`（STABLE，inbox:reply OR live:manage，固定 UNION ALL 商家）。三者均 `SECURITY DEFINER SET search_path = pg_catalog`，经 `inbox.principal_holds`（owner commerce_auth）按服务端 GUC 复核。
- 公开安全不变量是 DB CHECK（`templates_public_kinds_safe`/`fixed_public_kinds_safe`），即使调用方缺陷也无法落一条不安全的 public_reply/recommend 模板。
- 补 `GRANT USAGE ON SCHEMA inbox TO commerce_msgtemplates_writer`（定义者引用 `inbox.principal_holds` 需 schema USAGE + EXECUTE，同 0119 模式）。

## 风险与不变量

- P0：租户/店铺/主体现由服务端 GUC（`platform.WithScope`）确定，定义者用 `current_setting('app.*')` 复核；请求内 tenant_id/Host/客户端金额一律不信任。
- §3.5 内容匹配本身是 Go 侧（`internal/msgtemplates/publicsafe.go`）；定义者只强制结构 + 权限 + public_safe/kind 不变量。**LC-B4 发送路径在渲染 public_reply/recommend 时必须再调 `ValidatePublicSafe`**（与 publish 同一函数，避免规则漂移）。
- `order-pay-link/v1` 的 `{{連結}}` 占位永不落显示副本，由 LC-B4 封入发送密钥；public_safe=false。
- 无模板编辑/删除：版本追加制，纠错即新版本。
- 固定 id 在 publish 即 PT409，故 `resolve` 的固定分支永不被商家行遮蔽。

## NOT_RUN / BLOCKED

- NOT_RUN：全仓 `go test -race ./...`（AGENTS.md T02，T02 建立真实开发脚本后才跑；本单元只跑 touched 包 + foundation 子集）；浏览器 UI（LC-B4/LC-U2）；真实 FB/IG 流量；OpenAPI 共享 schema（本单元无变更）。
- BLOCKED：`bash scripts/dev/check-gates.sh` 的 node 段在本 worktree 失败——`node_modules/typescript-api` 缺失（`package.json` 声明 `typescript-api: npm:typescript@6.0.3`，但本 worktree 未安装）。与本单元无关（纯 Go/SQL/shell/docs 变更，未触碰任何 node/UI 文件）；其 Go/静态段已按脚本逻辑人工核对通过：gate-sync（usage 行 `--msg-templates` ↔ GATES.md 行一致）、无重复 mode 分支、0124 无 `commerce_worker` 授权、头文档 ratchet（新增文件均含 Purpose/Depends on/Used by，改动文件均含 leading doc comment）。

## Integrator 合并时

- 迁移号 `0124` 为占位，请按合并时实际空号重编号（同 LC-B3 的 0122→0119 处置）。
- 以 r3/integration 基线复跑 `scripts/dev/check-headers.sh`（本 worktree 对其无执行许可，已逐文件人工核对）；node 段需先 `npm i`（安装 `typescript-api` 别名）再跑 `check-gates.sh`。
- 作者不能作为唯一验收人：REAL_PG 门禁已由本执行者独立复跑（test-focused + test-local 各一次全绿），合并后请 integrator 再复跑 `bash scripts/dev/test-local.sh --msg-templates`。

## 完成与失败

- 交付：实现、测试命令与退出码、证据文件（`evidence-test.log`、`evidence-static.log`、`red.log`）、风险、NOT_RUN/BLOCKED，均在本目录。
- 本单元无 P0/P1 未解决项；无删除/放宽任何失败测试、阈值或 fixture。
