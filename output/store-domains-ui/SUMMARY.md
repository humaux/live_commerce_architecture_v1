# store-domains-ui — 最终交接（补完版）

2026-10-02。**本 UI 单 PASS；七项指定门禁在最终源码实跑，退出码全部 0。** 第 2、5 项已补完；域名幂等 P0 已由后端 `727e355` 补上，并经本单真实 Go/PG 重放路径验证。此结论不等于生产 DNS/TLS 或整个平台发布准入，集成者负责合并和终审。

## 工作区与提交

- 唯一写入工作区：`/Volumes/data/live_commerce_architecture_v1/.worktrees/store-domains-ui`，分支 `unit/store-domains-ui`。
- 本轮起点 `015cdfd6`，已核对后端 `727e3553` 及 `f43019d` 均为祖先；没有自行 push / merge / rebase / 部署。
- 最终被测源码：**`f351f44a7b888e17d5e54e110127254fceef213a`**。后续交付提交只含证据/文档。
- 未改 Go / SQL / deploy，未碰密钥、生产主机或客户业务。冻结 R3 驱动 `tests/storefront/storefront-publish-gate.mjs` 相对 `015cdfd6` 无差异。
- 外部协作者未跟踪的 `REVIEW-fix.md`、`output/store-domains-fix/`、`output/store-domains-fix2/` 保留且未加入提交。

| 提交 | 内容 |
| --- | --- |
| `687522be` | 权威 kind、edge_addresses 严格解析；持久待确认命令校验；Node 反例 |
| `777d217d` | 平台行隐藏动作、逐条 A/AAAA 复制、三语同 key 重试；三条真实写路由重复提交证明 |
| `f351f44a` | 平台类型与服务状态分行，修复英文移动端间距 |

均含 `Co-Authored-By: Codex <noreply@openai.com>`。早期 UI 提交 `0f5a768` / `3aba30e` / `47f61e8` / `0b23136` / `9f55dc3` 保留；旧交接在 Git 历史，不再代表当前阻塞状态。

## 原派单五项

| 项目 | 状态 | 实现与证据 |
| --- | --- | --- |
| 1 独立域名设置区块 | **PASS** | `storefront-domains-card` 与 `storefront-card` 为兄弟区块；冻结 R3 门禁 23 cases 全绿。 |
| 2 平台子域名隐藏暂停/解绑 | **PASS** | 接 DB 权威 `kind: platform/custom`，缺失/非法值关闭解析；仅 custom 渲染动作。不根据后缀、token、serving 推测。浏览器断言平台行按钮数为 0，自有域名仍可暂停/解绑。`687522be` / `777d217d`。 |
| 3 ACTIVE 非主域名 301 | **PASS，保留协议例外** | 目的地仅来自 Go 的 `primary_origin`；GET/HEAD 保留路径、查询串，主域名不跳；图片/媒体/API/静态路径均检查 301。TLS nonce 协议例外见下文。 |
| 4 开店 ID 与网址 | **PASS** | 英文建议、实时可用性、过期响应隔离、格式/占用/保留词提示、服务端收据网址保留。配置 base 的真实向导与未配置 base 的既有内部工作区流程都通过身份门禁。 |
| 5 DNS 复制与商家状态 | **PASS** | 每个返回的 A/AAAA 类型、根域名及 IP 均完整显示、逐条可复制；整组复制含 TXT 和所有地址；保留 CNAME 复制与三语商家状态。无 IP 时明确提示缺失，不把主机名当 A 值。三语×双宽 clipboard/44px/无横溢检查通过。`687522be` / `777d217d`。 |

### 补充第 3 项：幂等写入 — PASS

申请 / 暂停 / 解绑均经既有 `writeSettings` 发送 Idempotency-Key；BFF 原有透传及 GET 禁止 key 规则保留。

- 新用户动作生成新 UUID key，提交前持久化版本、原 key、操作及精确 hostname/origin。
- Journal 按店铺 + CSRF 会话摘要隔离，组件按店铺重挂载，发送前再次核对会话。没有密钥、令牌或买家 PII。
- UNKNOWN 不自动重发；刷新/重载只读。显式“重试同一操作”恢复原请求和原 key，其他新修改禁用。
- 重试拒绝（包括过期会话）不证明原命令未执行，因此保留待确认责任，确认重放成功后才清除。
- 旧 pending 或损坏记录缺少原请求，仍锁住并要求平台核对；没有“清空后重试”逃生按钮。
- 浏览器三条真实写路由各执行：Go/PG 提交 → 模拟响应 JSON 丢失 → 刷新 → 显式重试。断言 key/body 相同、返回 JSON 相同（含 TXT）、PG version/audit 不增加；三项新动作使用三个不同 key。GET 带 key 另验 422。

### 原 P0 与协议边界

**原 P0 / I02 已由后端 `727e355` 补上。** 三个写路由消费 key，经 command.Run 校验原请求、保存结果，同键重放不旋转 TXT/version、不增加审计。本 UI 单未改 Go/SQL。

本轮重复提交不是模拟成功：route.fetch() 实际经过 Next BFF → Go → 独立 PG；只模拟丢失返回体，重试仍到真实后端。独立 Go fixture 再核对最终行、主体和审计。

`/.well-known/lc-domain-check/…` 仍不进入 canonical redirect：这是确切 SNI 下的所有权/TLS nonce 协议，含 TLS_PENDING 和续期，不是买家页面。其余 ACTIVE origin 的 GET/HEAD 均按主域名处理；本轮未改变既有协议例外。

后端 edge 地址解析是 best effort；无法取得时 UI 明示缺失，不伪造可执行 DNS 值。

## 七项门禁：最终源码 f351f44a

均在本 worktree 执行；浏览器/PG 串行并持机器级锁；复跑期间源码冻结。日志在本目录。

| 命令 | 退出码 | 证据 |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | **0** | `final-node.log`：279 pass / 0 fail；r04 条件项 NOT_RUN 单列如下 |
| `pnpm --filter admin exec tsc --noEmit` | **0** | `final-admin-tsc.log`，无诊断 |
| `pnpm --filter storefront exec tsc --noEmit` | **0** | `final-storefront-tsc.log`，无诊断 |
| `bash scripts/dev/check-gates.sh` | **0** | `final-check-gates.log`：56 modes，所有 tracked tests 均有入口 |
| `bash scripts/dev/test-local.sh --browser-store-domains` | **0** | `final-browser-domains.log`，23 cases；逐项 `final-domains-driver.log`、`final-domains-result.json` |
| `bash scripts/dev/test-local.sh --browser-storefront-publish` | **0** | `final-browser-publish.log`，23 cases，冻结 R3 未改 |
| `bash scripts/dev/test-local.sh --browser-identity` | **0** | `final-browser-identity.log`：API 进程重启、身份链、设置向导均通过 |

新增字段反例先红：退出 **1**（`supplement-red.log`）；实现后相关 Node 两文件 **10/10** 通过，退出 **0**（`supplement-focused-green.log`）。开发期 tsc 曾发现结果 union 缩窄和旧 ref 引用两处错误，修正后 0，未放宽类型。`git diff --check` 退出 0。归档日志仅机械去除行尾空白，不修改测试结果。

### Fixture / 证据边界

- 最终域名：`output/playwright/store-domains-2928383604`；R3：`output/playwright/storefront-publish-4053892329`。
- 身份目录见 `final-browser-identity.log`，包括 `settings-real-20261002T140648.979994000`。
- 真实链为 production Next + Go + 独立 real PG；IdP 是 signed **MOCK**，DNS/证书为 scripted **MOCK**，Host/TLS 是 synthetic CONNECT edge。
- Apex UI 矩阵单独标为 **MOCK DTO**：两条 IPv4、一条 IPv6；不改实际两个域名行，末尾再比 PG facts 不变。它验证显示/复制/布局，不声称公网 DNS 成功。
- 原 UNKNOWN 断言保留：刷新/重载只产生一条 POST，新写入仍禁用；另加显式重放和旧 journal 拒绝。没有删除或放宽原断言。
- 初轮 `777d217d` 静态及两个浏览器门禁也为 0，保留 `supplement-*-777d217d.log`。排版 P2 修复后，在 f351f44a 重跑所有七项，不拿初轮绿替代最终证据。

## 截图与独立复审

以下均在本目录，PNG header/sips 实测尺寸，来自最终源码：

| 语言 | 设置：1586×992 / 390×844 | Apex：1586×992 / 390×844（MOCK） |
| --- | --- | --- |
| zh-TW | `domains-zh-TW-desktop.png` / `domains-zh-TW-mobile.png` | `domains-apex-MOCK-zh-TW-desktop.png` / `domains-apex-MOCK-zh-TW-mobile.png` |
| zh-CN | `domains-zh-CN-desktop.png` / `domains-zh-CN-mobile.png` | `domains-apex-MOCK-zh-CN-desktop.png` / `domains-apex-MOCK-zh-CN-mobile.png` |
| en | `domains-en-desktop.png` / `domains-en-mobile.png` | `domains-apex-MOCK-en-desktop.png` / `domains-apex-MOCK-en-mobile.png` |

另有 CNAME 和解绑全页截图。移动端通过真实菜单关闭动作等待抽屉离场，无 CSS 隐藏伪造。

- 主作者/实际门禁执行者：Codex root，实际模型标识未暴露，UNKNOWN；task `8cd5e6c2-d2fe-407a-9482-321d8b455808`。
- 独立源码审查：domains_security_review / security_reviewer，base015cdfd6 → 777d217d，无确认新增 P0/P1；其运行时模型/effort未暴露，UNKNOWN。
- 独立视觉：domains_visual_review / explorer / gpt-6-luna / medium；初轮12图仅一处已知P2；最终 f351f44a EN 桌面/手机复拍确认关闭。
- 只读合同核查：domains_contract_check / explorer / gpt-6-luna / medium。辅助代理无文件写路径、无递归委派。
- frontend-architect 用于解析/命令边界；impeccable audit-first 保留原页面形态并修正标签间距；Playwright 使用仓库门禁。见 `SURFACE.md`、`REVIEW-supplement.md`。
- 源码索引与原因记忆已关联；主作者独立跑门禁，审查者未独立跑 browser/PG（NOT_RUN）。

## NOT_RUN / 限制

1. 真实公网 DNS、CA/TLS 续期、Caddy、生产部署及生产 IdP/支付/Meta：**NOT_RUN**。
2. 后端“同 key 不同 body 冲突”、跨租户/跨店/跨主体隔离专项：后端交接有报告，本 UI 单未另跑该 Go 套件；本轮实测为同 session/store 三命令原请求重放及 GET key 拒绝。
3. 跨标签页/跨会话恢复、运行中切换会话的专项 UI 测试、WebKit/Firefox：**NOT_RUN**。发送围栏保留，不声称这些额外场景已验收。
4. 全仓 PG/release gates：**NOT_RUN**；上表七项不代替平台发布 gate。
5. `tests/media/r04-input-runner.test.mjs`：**NOT_RUN**，COMMERCE_R04_LIVEKIT_BINARY 未设置；test-node 明示，不计入279 pass。

所有本轮测试进程已退出，脚本 trap 清理自身 fixture/容器/端口；没有常驻服务。历史失败证据、其他任务目录和后端未跟踪交接均保留。共享租约在交付时释放；结果已存 Humaux、画布同步，等待集成者终审。
