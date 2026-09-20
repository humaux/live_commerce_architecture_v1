# 商家身份浏览器传输切片验收

2026-09-20；范围：T03 的私有 Go HTTP、Next BFF、会话 Cookie/CSRF、授权店铺发现及首店 API。**本地集成通过，不是完整 T03、登录界面交付或生产身份提供商接入。** UI 仍遵循用户先确认视觉稿的选择。

## 实现及调用边界

- 合同：`contracts/merchant-browser-auth-v1.md`（冻结提交 `d718c8a`，默认关闭条款在 `4479cfd` 明确）。
- Go：`cmd/api/identity.go` 验证启动配置，独立打开 `commerce_identity` authority pool；`internal/identityhttp` 只接受固定 BFF 密钥与有界 JSON。业务 runtime pool 不能签发会话。`0005_browser_auth.sql` 仅提供受限的当前会话店铺投影函数。
- Next：`lib/auth.ts` 集中管理固定 origin、唯一 Cookie、CSRF、上游调用及安全错误；`app/api/auth`、`onboarding/initial-store` 和 `stores` 终止浏览器凭证。业务代理只转发固定路由及服务器选定的 bearer，不转发浏览器 Cookie/任意授权头。
- `lib/backend.ts` 的 SSR 与 API 使用同样的唯一 Cookie 规则；`lib/client.ts` 只读 CSRF Cookie。商家会话 bearer 不进入客户端 JS。原 fixture 仍仅开发模式和固定 loopback 生效。
- Go 和 Next 身份接入默认关闭；启用必须提供完整配置。Go 私有身份接口只允许字面 loopback 监听。本切片不支持将该端口直接暴露公网或未经评审的跨主机 BFF。
- 实现提交：Go `e8c3c0b` + `4479cfd`；Next `961dfc1` + `610b36a`；浏览器规格 `100eb64` + `a47f6b3`。根集成后单独重跑，没有把子 agent 自测当作 root 验收。
- 没有新运行时依赖；复用 pgx、go-oidc、oauth2、Next 和标准库。JWT 签名代码仅属于标注 mock 的测试，生产验签仍由既有 go-oidc 完成。

## 实测 gate

|Gate|实际结果与边界|
|---|---|
|Go / 真实隔离 PG18|`bash scripts/dev/test-local.sh` exit 0；85 个顶层 PASS，0 FAIL，0 SKIP；race 与 vet 通过。包括身份权限、RLS、幂等、会话过期/撤销、默认关闭与 store-list 上限。|
|浏览器全链|`bash scripts/dev/test-local.sh --browser-identity` exit 0。真实 Chromium → Next **生产包** → 实际 Go → 临时 PG；**外部 IdP 为本地 RS256 签名 mock**。|
|全链业务结果|从空店铺列表创建内部商户/店铺/仓库；同键同体返回同一 receipt，改体 409；仓库与台账可读，跨店请求 404；缺 CSRF 的写入 403；退出 204，旧 Cookie 再读 401。|
|独立持久化核对|Go 在浏览器结束后直接 SQL 确认该 issuer/subject 仅 1 个 session 且已撤销、仅 1 份初始店 receipt；签名 IdP 交换恰好 1 次且 JWKS 确被读取。|
|Cookie/登录状态|Secure、HttpOnly（会话）、SameSite=Lax、Path=/、期限与可读 CSRF；无 binding 的 callback 重放拒绝；原生 form 从 en 入口完成 zh-TW 回跳。未实现获批登录 UI，不用注入表单冒称 UI 交付。|
|BFF mock 负例|root 在生产包重跑 `LC_BROWSER_SUITE=identity-mock pnpm exec playwright test tests/admin/auth.spec.ts`，1/1 PASS。覆盖流式超限、重复 Cookie、issuer 精确匹配、非 JSON 上游错误、CSRF、退出失败不提前清 Cookie。|
|既有后台回归|全新 dev-only PG fixture，`pnpm test:admin` 原 9/9 通过，含手机/桌面、语言、丢响应幂等、storage 故障、陈旧版本与权限拒绝。原 3 个 spec 未被隐藏。|
|静态/构建|admin strict typecheck、standalone build、browser-tag Go compile/vet 通过；i18n 4/4 及 strict typecheck 通过。包结构检查只计 `PASS_PACKET_STRUCTURE_ONLY`。|
|独立审查|合同、Go SQL/HTTP 和 browser harness 范围无剩余 P0/P1；详见下文 Humaux 回执。不是全产品安全审计。|

## 原始证据

目录 `/Volumes/data/output/live-commerce-browser-auth-tests/`：

|文件|SHA256|
|---|---|
|`root-go-default-off.log`|`1545657f69cbaaf801c0547afcccd661f3bbecdcd7c75aadb649f9e249b869d1`|
|`root-real-browser-fixed.log`|`fa3a209dd6cf1d690694680db997f894deecab39763a28790fa4be5e112f736d`|
|`root-mock-browser.log`|`35ed1fe88c4c8a4a4f2acb250dbbea21b1cfd867141947be08c66c78761396fa`|
|`root-ledger-regression.log`|`ad00fee120ef6f8fc485029522ae1413dd3332ccd47b453acd11c1cd8dc8a97f`|

浏览器全链明细：`output/playwright/identity-chain-20260920T072652.563299000/playwright.log`，SHA256 `f4ba68ecc589f47c4bd1bc3d3519b4918208ac2b6ca504e4e6fe6ee678a84885`。trace 关闭以免录下令牌；测试只输出 Cookie 元数据和安全状态。失败日志与截图保留，未删除失败来制造通过。

台账回归更新了 `.impeccable/review/{desktop,hero-repro,mobile,scrollbar-active}.png`，root 复看桌面/手机图，布局未改：表格、横向滚动、操作托盘可见；它们是原已批准台账的运行态证据，不是新登录三稿的批准。

## 发现并关闭的问题

1. **Go 路由合成遗漏**：基础 handler 已挂 store-list，但上层 httpapi 白名单没有注册，初跑 404。修正实际合成层，未改预期状态来过测试。
2. **默认关闭 P1**：最初 store-list 未绑定身份总开关。`4479cfd` 改为显式启用才挂载；有效 session 与 nil pool 的 disabled 负例均 404。独立关闭回执 `3d1a640d-600c-4d8d-ab26-5bd0f2efee78`。
3. **Next 三项 P1**：`arrayBuffer()` 先整包读取再限长、SSR 重复 Cookie 取单值、issuer 尾斜杠被规范化。`610b36a` 分别改成流式有界取消、唯一 raw Cookie 解析、保留精确 issuer；并补 disabled verbs / 安全错误处理。记忆 `af67497d-c7ac-4c9d-a280-490c112a7dc0`。
4. **全链测试 Cookie 误判**：Chrome 已带会话发出 SSR stores 200，但 Playwright `cookies(http://127...)` 会过滤 Secure Cookie。最小 probe：全 context 1，HTTP URL 过滤 0，HTTPS URL 过滤 1。`a47f6b3` 按 context+精确域查看元数据，并以页面原生 fetch 走后续操作；没有移除 Secure 或放宽授权。

合同独立复核 `3497c9fe-68d5-4eae-a952-94210b372298`；harness 范围复核 `57b579d7-5691-4597-a707-520f5fb7e9c1`；root 贯通证据 `8c24a2b3-1a8f-439c-bf6a-40c5c91af325`。代码索引和原因关联已写 Humaux。

## 可重复运行与隔离

1. 常规 Go/PG：`bash scripts/dev/test-local.sh`。
2. 完整身份浏览器链：`bash scripts/dev/test-local.sh --browser-identity`。需本机既有固定 PG 镜像、Go、Node、pnpm、Playwright Chromium；脚本先构建生产包，再建独占 PG。Go harness 动态分配 loopback 端口，所有登录/建店数据只写该临时库。
3. 旧台账：按 `2026-09-20-admin-ledger-acceptance.md` 的 5 步启动新 fixture 与 3100/3101。`LC_BROWSER_SUITE` 默认 `ledger`；`identity-mock` 和 `identity-real` 必须显式选择，未知值报错。不要将三个不同身份 fixture 混跑。
4. 本轮所启动 Next/fixture 进程均已停止；临时 PG、私有 session.env/owner 文件和临时二进制由精确 label/PID trap 回收。已批准清理范围仅为可重建测试资源；证据和共享缓存保留，无客户生产写入。

## 编排与剩余范围

根 integrator 负责 Go/migration/合同/脚本与最终回放；UI writer 独立 worktree `/Volumes/data/live-commerce-wt-t03-bff`、branch `codex/t03-browser-bff`、base `d718c8a`，仅写 BFF/lib/auth 和对应 spec；独立 security reviewer 只读。没有递归派工，最多两名代码写入者。模型继承调用端配置；工具未提供可核实的实际底层模型和 reasoning 标识，因此不猜测填写。

仍未通过：真实 IdP/邮件/邀请或注册策略、MFA/再认证、滥用限流、过期 flow 清理与保留策略、生产 HTTPS/反向代理和部署运维；用户批准后的登录/首店 UI 及三语言完整交互；多店选择/生命周期、其他 audience、域名、购物车/结账/支付退款、跨境超商物流、直播与四域会话业务。`T03` 仍 `IN_PROGRESS`，全局 gate 不能标 PASS。
