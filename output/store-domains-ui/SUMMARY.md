# store-domains-ui — 最终交接

2026-10-02。**PARTIAL / BLOCKED，不是整体完成或合并准入。** 可独立完成的 UI 已提交；7 条指定门禁均实跑退出 0。第 2 项及后端幂等缺口仍需后端单元处理，不能用门禁绿替代这些要求。

## 工作区与提交

- 唯一写入工作区：`/Volumes/data/live_commerce_architecture_v1/.worktrees/store-domains-ui`，分支 `unit/store-domains-ui`。
- 用户派单 base：`0c08f37`。执行中外部协作者将分支快进至后端 `70d7e01`；本单保留该更新，未自行 merge/rebase，也未更改其 Go/SQL/deploy 文件。
- 最终被测源码：`9f55dc36f8dac2f7f2c344fb7a0ed73940f3187e`。其后的交付提交仅含本目录的证据/文档。
- 没有 push、merge、部署、读取生产密钥或操作客户业务。
- 其他单元的未跟踪 `REVIEW-fix.md`、`output/store-domains-fix/`、`output/store-domains-fix2/` 原样保留，未纳入本单提交。

| 提交 | 内容 |
| --- | --- |
| `0f5a768` | 服务端 primary-origin 解析、GET/HEAD 301、路径查询串保留、封闭错误路径与单测 |
| `3aba30e` | 开店 ID 预览清理过期结果、请求取消/竞态防护、共享收据校验 |
| `47f61e8` | 域名单独区块、三语 DNS 复制/状态、UNKNOWN 写操作围栏 |
| `0b23136` | 新增 Node/域名浏览器反例、301 链与静态路径、三语截图、失联结果围栏测试 |
| `9f55dc3` | 保留原有 zh-TW“內部工作區已建立”收据文案，同时追加网址未就绪提示；不改身份测试断言 |

上述提交均含 `Co-Authored-By: Codex <noreply@openai.com>`。

## 逐项状态

| 派单项 | 状态 | 实现 / 证据 / 提交 |
| --- | --- | --- |
| 1 独立域名设置区块 | **PASS** | `storefront-domains-card` 与冻结的 `storefront-card` 为兄弟区块，域名输入不再污染 R3。`47f61e8` / `0b23136`；R3 23 case 通过，冻结的 `tests/storefront/storefront-publish-gate.mjs` 相对 `70d7e01` 无改动。 |
| 2 平台子域名隐藏暂停/解绑 | **BLOCKED** | 后端 domain DTO 没有平台/自有域名判别字段，admin store DTO 也没有权威 base/handle。**当前平台行仍显示按钮**，截图如实保留。不能用域名后缀猜测、token 为空、排序或 serving 标志代替权限事实。需后端逐行提供 `kind: platform/custom` 或 `is_platform`，再改解析器及渲染。现有服务端拒绝平台域名变更不代替 UI 要求。 |
| 3 ACTIVE 非主域名 301 | **PASS，附协议例外** | `0f5a768` / `0b23136`。目的地只取私有 Go 的 `primary_origin`，严格验证 HTTPS origin；主域名不跳；GET/HEAD 保留原路径/尾斜线/查询串，含静态资源、图片、媒体和 API 读取；不重定向写请求。解析超时/异常不猜测，返回 no-store 503。18 case gate 覆盖首跳 301。协议例外见下文，需集成者知悉。 |
| 4 开店 ID 与网址 | **PASS：已配置 base 的闭环** | `3aba30e` / `9f55dc3`。英文名称提示、实时建议、保留词/占用/格式/无法确认三语反馈；旧响应不能覆盖新输入；最终网址展示服务端收据而不是前端拼造。配置 base 的真实 Go/PG 向导通过。未配置 base 的既有内部工作区路径仍允许创建，但明确显示网址未就绪，不承诺买家可用；后端若要求此路径一律失败需另单裁决。 |
| 5 DNS 复制与商家状态 | **PASS：CNAME；apex 完整指令 BLOCKED** | `47f61e8` / `0b23136`。一键复制 TXT/CNAME、成功/失败反馈；等待 DNS 設定 / 驗證中 / 憑證簽發中 / 已啟用 / 已暫停及对应 zh-CN/en，不显示内部状态枚举。后端 apex 回包没有实际 IP，UI 明示向平台取得 A/AAAA 地址，**不把 hostname 当 A 值**；完全可执行的 apex 指令需后端补 IP。 |

### 协议例外与后端 P0

- `/.well-known/lc-domain-check/…` 不进入 canonical redirect。这是后端所有权/TLS nonce 协议，须在确切 SNI 主机上可达（含 TLS_PENDING 和后续续期），不是买家页面。其余 GET/HEAD 均进入解析。如果“任何 GET/HEAD”被解释为包括该协议，需集成者与后端明确协议路由裁决；本单未破坏证书验证去满足字面跳转。
- **P0 / I02：后端未消费 Idempotency-Key。** BFF 转发了 header，但 `internal/httpapi/storefront.go:69–76` 未读取；`migrations/0106_store_domains.sql:347–353` 的再次申请会旋转 TXT token/version。源码审查已证实该调用路径，不是声称实跑了同键重复请求。
- UI 仅做风险收敛：发送前写入店铺+会话隔离 pending 标记，UNKNOWN 后保留并禁写，刷新/重载后仍只允许读回，不自动或手动盲重发。标记无令牌/PII；没有“清除后重试”逃生按钮。后端/运维须核对责任，**这不等于持久幂等，不覆盖新标签页或另一会话**。
- 后端补幂等验收：真实 PG 连续提交同一 key+body，证明 token/version/audit 不重复；同 key 不同 body 必须拒绝；绑定 tenant/store/actor 范围。此项 **NOT_RUN / BLOCKED**，本单不越界改 Go/SQL。

## 必跑门禁（最终源码 9f55dc3）

命令均在本 worktree 执行；浏览器/PG 串行持机器级锁。日志都在本目录。

| 命令 | 退出码 | 证据 / 结果 |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | **0** | `test-node-release.log`：276 pass / 0 fail；另有 r04 二进制条件测试 NOT_RUN，见下文 |
| `pnpm --filter admin exec tsc --noEmit` | **0** | `tsc-admin-release.log`（无诊断） |
| `pnpm --filter storefront exec tsc --noEmit` | **0** | `tsc-storefront-release.log`（无诊断） |
| `bash scripts/dev/check-gates.sh` | **0** | `check-gates-release.log`：56 modes，所有 tracked tests 已接入 |
| `bash scripts/dev/test-local.sh --browser-store-domains` | **0** | `browser-store-domains-release.log`：18 cases，含 301；`store-domains-result.json` |
| `bash scripts/dev/test-local.sh --browser-storefront-publish` | **0** | `browser-storefront-publish-release.log`：23 cases，冻结 R3 驱动原样 |
| `bash scripts/dev/test-local.sh --browser-identity` | **0** | `browser-identity-verified.log`：API 真进程重启、身份链、设置向导三个 Go 测试均通过 |

补充：源码 `git diff --check` 退出 0；归档日志的 Next 进度行尾空白首次触发 cached whitespace check（退出 2），仅机械规整日志行尾空白后重新检查，不改变任何测试结果。impeccable 静态 detector 本单仅运行一次，退出 0、无输出（`impeccable-detector.log`）；不把它说成全面视觉/安全验收。

### 测试边界与保留失败

- 浏览器：production Next + Go + isolated real PG；登录为 signed **MOCK** IdP，DNS/证书探测为 scripted **MOCK**，Host/TLS 为 synthetic CONNECT edge。不是 LIVE 域名、真实 CA、Caddy 发布或供应商验收。
- 最终域名 fixture：`output/playwright/store-domains-2468833643`；R3：`output/playwright/storefront-publish-1238505294`。
- 最终身份 fixture：`account-process-20261002T113825.137804000`、`identity-real-20261002T113833.860537000`、`settings-real-20261002T113837.998541000`（均在 `output/playwright/`）。
- 18 case 中追加的 UNKNOWN 与旧预览竞态为浏览器 route-controlled **MOCK** 反例；原始真实后端向导/生命周期路径仍执行。不得把模拟 503 当作后端真实同键重试证明。
- 初次新增 UNKNOWN 断言误选了应保持可用的“刷新”按钮，退出 1；仅修正本单新增断言的选择范围为写按钮，失败日志 `browser-store-domains-acceptance.log` 保留。
- 最终身份复跑第一次退出 1：新文案替换了原有“內部工作區已建立”，原冻结断言失败。修文案而非断言后退出 0；`browser-identity-final.log` 及 fixture `identity-real-20261002T113639.437386000` 保留。
- 域名 301 旧检查从导航最终 response 开始、只见 200；驱动现在沿 `redirectedFrom()` 找首请求，再走原重定向链断言。没有删 301 断言或接受 200 代替 301。
- 没有删除/放宽原断言；只在本域名驱动迁移独立卡选择器、用户指定状态文案，并新增测试。

## 截图与独立审查

最新截图取自最终源码的 `store-domains-2468833643`，仅 synthetic fixture 数据。`sips` 实测尺寸：

| 语言 | 1586×992 | 390×844 |
| --- | --- | --- |
| zh-TW | `domains-zh-TW-desktop.png` | `domains-zh-TW-mobile.png` |
| zh-CN | `domains-zh-CN-desktop.png` | `domains-zh-CN-mobile.png` |
| en | `domains-en-desktop.png` | `domains-en-mobile.png` |

另存 `domains-zh-CN-desktop-dns.png` / `domains-zh-CN-desktop-detached.png` 全页证据。移动截图使用真实菜单关闭动作并等待抽屉离场，没有 CSS 隐藏导航伪造验收。

见 `SURFACE.md`（audit-first、设计边界）及 `REVIEW.md`（独立源码/视觉审查）。采用 impeccable 做现状审计与视觉复核，frontend-architect 做边界划分，Playwright 使用仓库已有门禁。视觉跟随现有壳和业主视觉稿，不新增无后端控件；DNS 内层描边为轻微非阻塞视觉意见，本单保留。

## 团队与归属

- 主作者/独立复跑者：Codex root，运行时具体模型标识未暴露，不猜测。Humaux task `8cd5e6c2-d2fe-407a-9482-321d8b455808`，agent `codex-store-domains-ui`。
- 只读源码审查：`domains_security_review` / security_reviewer / gpt-6-sol / high；审查 base `0c08f37`，最终源码复核 `0b23136`。
- 只读视觉审查：`domains_visual_review` / explorer / gpt-6-luna / medium；只读前后截图和设计规则。
- 只读定位：`r5_gate_paths`、`r5_image_audit` / explorer / gpt-6-luna / medium。子代理无写路径、无递归委派；只有主作者写本工作区。
- 允许写路径：派单 apps 下域名/开店/Host 文件，加 `tests/admin/store-domains-ui.test.ts`、`apps/storefront/tests/primary-origin.test.mjs`、本域名 gate、test-node 接线、本输出目录。源码改动清单可查 `git diff 70d7e01..9f55dc3 --name-only`。
- 源码审查者未独立复跑浏览器/PG，审查者的该部分为 NOT_RUN；主作者已实跑上表。集成者仍负责终审，不将作者单独测试当成全部准入。

## NOT_RUN / 后续顺序

1. **后端先补**持久域名幂等、平台/custom 判别字段；P0 未关闭不可合并。UI 接字段后隐藏平台行两按钮，增加浏览器正负例并复跑本单七门禁。
2. 后端提供可信 apex edge IP，UI 才能输出实际 A/AAAA 值；现状不伪造。
3. 集成者确认 TLS nonce 路由例外，以及未配置 base 时内部工作区创建的兼容边界。
4. **NOT_RUN**：真实 DNS、CA/TLS 续期、Caddy/生产部署、生产 IdP/支付/Meta；后端真实 PG 同键重放；跨标签页/跨会话幂等；平台按钮隐藏（缺字段）；完整 apex 指令；WebKit/Firefox；全仓全量 PG/release gates（不在本单七门禁中）。
5. `tests/media/r04-input-runner.test.mjs` **NOT_RUN**：`COMMERCE_R04_LIVEKIT_BINARY` 未设置。test-node 明确报告，不算本单 276 pass 之内。

测试脚本已退出并按 trap 清理自身 fixture/进程；没有人工常驻服务。失败证据及别的任务目录保留。共享锁在交付时释放；任务以带上下文的 handoff 交回，不报整体 complete。
