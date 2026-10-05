# DELIVERY — unit w1-01b-meta-health（Meta 连接健康后端：health job + integration.binding_capabilities + 商家通知邮件 + banner API）

- 角色: DeepSeek V4-Pro（后端执行者；不合并/不推送/不部署，integrator 负责）。实际模型: deepseek-v4-pro（会话驱动）。
- Base SHA: `1a6a9177`；worktree: `.worktrees/w1-01b-meta-health`；branch: `unit/w1-01b-meta-health`。
- 合同: `contracts/meta-connection-health-v1.md`（DRAFT 2026-10-05）W1-01B 后端（§2–§9）；banner UI（§10 MCH11）与 LIVE 探针（§11.3 MCH12）不在本单元。
- 迁移号: `0125`（占位；integrator 合并时按空号重编号，合同 §8 的 0127 仅为建议号）。
- 状态: 后端实现 + DB-free 单测全绿；REAL_PG 门禁（`^TestMetaHealth`）**见 §测试**；`--meta-health` 门禁已登记 GATES.md。全仓 race、banner UI、真实 Meta 流量均 NOT_RUN。

## 证据标签

| 证据 | 标签 |
|---|---|
| 本文件 / 迁移注释 / 源码注释 | DESIGN |
| `internal/metaconnect`、`internal/notify`、`internal/httperror`、`cmd/*` 单元测试 | UNIT（MODEL_ONLY / MOCK） |
| `tests/foundation/meta_health_test.go`（一次性 PG 容器 + 假 Graph） | REAL_PG + MOCK Graph |
| `go build` / `go vet` / `gofmt` / gate-sync 人工核对 | 静态 |
| 全仓 `go test -race ./...`、banner UI（MCH11）、LIVE 探针（MCH12） | NOT_RUN |

## 交付物（唯一写入路径，均在合同清单内）

实现：
- `migrations/0125_meta_connection_health.sql` — 前向迁移（见「迁移校验」）。
- `internal/metaconnect/derive.go` — `Derive(provider, capability, reading) (state, reason)` 首中规则表（§3.2）；LC-B3 未合并，故本单元拥有此文件。
- `internal/metaconnect/capability.go` — `CapabilityReader` 接口 + `SnapshotReader`（连接快照回退）+ `TableReader`（probed 行优先，空则回退）；`ReaderConfig`、`Evidence*` 常量。
- `internal/metaconnect/health.go` — `Health{Reader}` 读模型：`Status`/`Recheck` 服务，B1/B2 的 `HealthPage`/`HealthStatus`/`HealthCapability` 结构。
- `internal/integrations/metareply/probe.go` — 探针：claim → 打开 Page-token 托管（v2 HPKE / v1 AES）→ 假/真 Graph 三阶段（P1/P2/P3）→ 派生 → record；lease 50 s 超时。
- `internal/httpapi/meta_health.go` — B1 `GET /v1/admin/stores/{store_id}/meta/health`（store:read）与 B2 `POST .../recheck`（integration:manage，60 s 429 门）。
- `cmd/api/merchant_meta_health.go` — §7.4 reader swap 的 API 侧构造（`TableReader{Fallback: SnapshotReader}`；不 import metareply/pagetoken/pageopen，不接触 token）。
- `internal/notify/render.go` / `worker.go`（改）— 商家 meta_health 告警渲染 + `notify.merchant_alerts` 抽取循环（`enqueue/claim/record`）。
- `cmd/claims-worker/main.go`（改）— River 周期 `meta_health_sweep_v1`（5 min、`RunOnStart`、unique by args `{}`）+ worker 注册；`main_test.go`（改）验证接线。
- `cmd/expiry-worker/main.go` / `mail.go` / `doc.go`（改）— `COMMERCE_ADMIN_ORIGIN` 打开商家告警邮件循环；`main_test.go`（改）验证 env。
- `internal/httpapi/handler.go`（改）、`internal/httperror/error.go`（改，`recheck_too_soon` 文案）、`internal/metaconnect/errors.go`（改，`recheck_too_soon`→429）。
- `scripts/dev/test-local.sh`（改）— 登记 `--meta-health` 门禁；`docs/delivery/GATES.md`（改）— 增 T2 行 + 门禁章节。
- 测试：`internal/metaconnect/derive_test.go`、`tests/foundation/meta_health_test.go`（9 个 REAL_PG 门禁，覆盖 MCH02/03/06/07/08/09 + 托管 v2/v1 + token 防泄漏）。

未触碰：`apps/**`、`go.mod`/`go.sum`、OpenAPI 共享 schema、pnpm 锁文件、`contracts/*`、banner UI（W1-01U）、LIVE 探针（MCH12）。

## 红→绿证据

- 红证据（`red.log`）：`TestDerive/ig_dm_field_assumed_present` 期望 `("unknown","lc_u11_open")`，实现正确返回 `("ok","ok_app_level_assumed")`——用例忘清 `DMConfirmed` 触发 IG app-level 订阅的 rule-6 跳过，修正断言后全绿（其余 39 例首跑即绿）。
- DB-free 单测 `go test -count=1 ./internal/metaconnect ./internal/notify ./internal/httperror ./cmd/claims-worker ./cmd/expiry-worker ./cmd/api` — **全绿 exit 0**（见 `evidence-test.log`）。
- REAL_PG 门禁 `bash scripts/dev/test-focused.sh '^TestMetaHealth'`（等价 `bash scripts/dev/test-local.sh --meta-health` 的 foundation 段）— **见 `evidence-test.log` 尾部**（MCH02 探针结果/190→reauth 单次翻转+审计、MCH03 限流/6 连 unknown、MCH06 token 零泄漏、MCH07 B1/B2、MCH08 reader swap、MCH09 reconnect 复位；假 Graph = MOCK loopback，无真实 Meta 流量）。
- 静态（`evidence-static.log`）：`go build ./...` exit 0；`go vet`（touched 包，含 tests/foundation）exit 0；`gofmt -l` 空；header ratchet 逐文件核对通过；gate-sync（usage 行 ↔ GATES.md）核对通过。

## 迁移校验（0125）

- 前向：precondition 校验依赖对象（`integration.meta_connections`、`bindings`、`meta_page_heads`、`resolve_access` 等）；幂等（`IF NOT EXISTS` 风格的表/函数守卫）。
- 表：`integration.binding_capabilities`（PK tenant/store/binding/capability，FK→meta_connections ON DELETE CASCADE + FK→bindings；能力/状态/证据 CHECK 冻结词表）；`integration.meta_health_probes`（PK tenant/store/page，lease 字段、generation、consecutive_failures、severity、episode）；`notify.merchant_alerts`（0090 模式，24 h 冷却）。全部 FORCE RLS、`REVOKE ALL FROM PUBLIC`。
- 定义者（SECURITY DEFINER `SET search_path=pg_catalog`，owner commerce_integration_writer；notify.* owner commerce_checkout_writer）：`meta_health_snapshot`、`claim_meta_health_probes`（60 s lease，`FOR UPDATE SKIP LOCKED`，generation+1，`gen_random_bytes(32)`）、`record_meta_health`（lease 校验，stale/connection 消失即 no-op）、`report_capability_failure`、`mark_capability_evidence`、`binding_capability_state`、`request_meta_health_recheck`（60 s 429）、`notify.enqueue_merchant_alert`（24 h 冷却）、`notify.claim_merchant_alerts`、`notify.record_merchant_alert`。
- 触发 `meta_health_on_connection`（AFTER INSERT/UPDATE OF status,scopes,fb_binding,ig_binding）：复位探针（due now）+ 仅在 INSERT/status=active/scopes/fb/ig 变化时清 capability 行（reauth 翻转不清行）。
- **特权增量偏差（MCH10，需 integrator 更新 MCI02/KC03 期望行）**：合同 §8 将 `report_capability_failure` / `mark_capability_evidence` / `binding_capability_state` 授权 `commerce_worker` + `commerce_claims_worker`，但 0096 已退役共享 `commerce_worker`（空角色、无 login 可加入、check-gates 禁止 0096 后再授权）。0125 因此只授 `commerce_claims_worker`（LC-B2/B4 私信回复路径运行于该 authority）。见迁移头注释。
- 能力行只存 id/code/时间戳，绝不存 token、scopes dump 或 Graph body。

## 风险与不变量

- P0：租户/店铺/主体现由服务端 GUC（`platform.WithScope`）确定，定义者按 `current_setting('app.*')` 复核；请求内 tenant_id/Host/客户端金额一律不信任。
- P0：token 绝不出现在 URL/日志/审计/告警/探针/能力行（I11 保留）；探针只经既有 Page-token 托管（v2 HPKE 32B nonce / v1 AES 12B nonce），`cmd/api` 不 import metareply/pagetoken/pageopen 也不命名开 token 的密钥（`TestMetaConnectAPIHoldsNoPagePrivateKey` 守护）。
- 探针在 P1 失败（token 190 / page 100 / rate_limited / unknown / 传输错）时**提前返回、不派生状态**——`binding_capabilities` 保持空，连接翻 reauth_required，reader 由快照回退派生 reauth_required。P2 权限错（100/10/200-299）走快照回退但 P3 仍真实跑 fb_fields。
- 副作用幂等/可对账：探针 claim 是 lease 栅栏；record 幂等（stale 即 no-op）；告警 24 h 冷却 + 每 (Page, episode) 一封；审计 `meta.connect.reauth_required` 只带 `{page_id, reason}` 无 token。
- 架构与测试结果已区分 DESIGN / MODEL_ONLY / MOCK / REAL_PG / NOT_RUN（见上表）。

## NOT_RUN / BLOCKED

- NOT_RUN：全仓 `go test -race ./...`（AGENTS.md T02：T02 建立真实开发脚本后才跑；本单元只跑 touched 包 + foundation 子集）；banner UI（MCH11，W1-01U Codex 单元，需真实点击台账）；LIVE_READ 探针（MCH12，owner 前置条件 + `output/meta-health-probe/`）；真实 SMTP 投递（`internal/mail` 自有 MOCK 门禁）。
- 作者 smoke 未覆盖：MCH04 lease/stale 双进程对抗（claim 为 lease 栅栏、record 拒绝 stale lease，但无对抗双进程测试）；MCH05 告警 24 h 冷却 / warning 永不发信。MCH02/03/05/06/08 的对抗门禁由独立测试者 W1-01T（Kimi K3）在 `tests/foundation/meta_health_gate_test.go` 交付（先 red 后 green，写于读本 diff 之前）。
- BLOCKED（环境，与本单元无关）：`bash scripts/dev/check-gates.sh` 的 node 段在本 worktree 失败——`node_modules/typescript-api` 缺失（`package.json` 别名 `typescript-api: npm:typescript@6.0.3`，本 worktree 未安装）。本单元为纯 Go/SQL/shell/docs 变更，未触碰任何 node/UI 文件；其 Go/静态段已按脚本逻辑人工核对通过（gate-sync、无重复 mode 分支、0125 无 `TO commerce_worker`、header ratchet）。合并方需先 `npm i` 再跑 `check-gates.sh`。

## Integrator 合并时

- 迁移号 `0125` 为占位，请按合并时实际空号重编号（合同 §8 的 0127 仅建议号）。
- **MCH10**：按 §8 与 0096 的冲突更新 MCI02/KC03 期望特权行——`report_capability_failure`/`mark_capability_evidence`/`binding_capability_state` 实际只授 `commerce_claims_worker`（非 `commerce_worker`）。
- 以 r3/integration 基线复跑 `scripts/dev/check-gates.sh`（本 worktree 对其 node 段无依赖，见 BLOCKED）；header ratchet 已逐文件核对。
- 作者不能作为唯一验收人：REAL_PG 门禁由本执行者复跑（test-focused），合并后请 integrator 再复跑 `bash scripts/dev/test-local.sh --meta-health`；MCH02/03/05/06/08 由 W1-01T 独立门禁验收。

## 完成与失败

- 交付：实现、测试命令与退出码、证据文件（`red.log`、`evidence-static.log`、`evidence-test.log`）、风险、NOT_RUN/BLOCKED，均在本目录。
- 本单元无 P0/P1 未解决项；无删除/放宽任何失败测试、阈值或 fixture。touched 包与 meta 相关回归全绿；trunk 上约 26 个与本次无关的既有 foundation 红不在本单元范围。
