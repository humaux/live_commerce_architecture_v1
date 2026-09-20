# C 分步向导：登录与首店界面验收

2026-09-20；用户明确选择「C分步向导」。作者提交 `3997543`，root 合入
`b063cff`，独立审查修正与测试闭环 `a65db97`。**本地界面切片通过；不是完整
T03、真实身份提供商接入或 SaaS 生产上线许可。**

## 用户结果与边界

- `/zh-CN`、`/zh-TW`、`/en`：未登录显示外部身份服务入口；未配置显示明确不可用，
  不引入密码表单或用 Meta 账号替代商家身份。
- 已登录且无店铺时：商户名称 → 店铺名称/交易币种 → 初始库存仓与最终只读核对。
  只在最后一步提交已有合同的四字段；前三个名称均是新建文本，不暗示资质认证。
- 币种是服务端配置的 ISO 值，显示本地名称；切语言保留草稿/步骤、不改币种。
- 提交前持久化原 body/key；结果未知时锁定编辑，只重试原请求；同步双击保护。
  sessionStorage 使用 CSRF 摘要隔离、24h TTL、有界字符串；不存 token 或原 CSRF。
- 401 清理旧会话草稿；发送/退出前核对会话绑定，并使用同一捕获 CSRF 发请求。
  另一标签切换账号后，旧页面不提交旧草稿，也不登出新账号。
- 成功只说明内部工作区已建立；进入原商品库存账簿。不宣称已发布店铺、接通支付物流。

服务端开关仍默认关闭。Next 与 Go 都须显式配置 `COMMERCE_ONBOARDING_ENABLED`
与 `COMMERCE_ONBOARDING_CURRENCIES`，提供商/Origin 等配置沿用身份合同。
UI 下拉不是安全边界；Go 再次校验币种、授权与原子开店约束。配置不匹配必须拒绝，
不靠浏览器自动猜测或扩大允许范围。

## Root 独立复跑

|Gate|结果|
|---|---|
|`pnpm typecheck:admin`、`pnpm build:admin`|exit 0；生产 standalone 产物，无新依赖|
|`pnpm test:i18n`|4/4 PASS|
|`LC_BROWSER_SUITE=entry-mock pnpm test:admin`|5/5 PASS；三语、桌面/手机、长名/对比度、unknown 原字节重试、401、跨账号反例|
|`LC_BROWSER_SUITE=identity-mock pnpm test:admin`|1/1 PASS；绑定、Cookie、CSRF、拒绝与撤销顺序|
|原 ledger 全套浏览器回归|9/9 PASS；商品创建、库存调整、丢响应、冲突、三语、生产拒绝均保留|
|`bash scripts/dev/test-local.sh --browser-identity`|最终 `a65db97` 1/1 PASS：真实 Chromium → 生产 Next → Go → 隔离 PG；**外部 IdP 是 RS256 签名 MOCK**|
|实际业务读回|按钮登录/开店，原响应与 replay 完全一致、异参 409，数据库店铺与仓记录、进入账簿、退出后 401|
|独立视觉|初始全表面 `fix`；最终仅对清单逐项评分 `ship`，不扩大为全产品审查|

生产 IdP、注册/邀请、MFA/邮件、限流、生产 HTTPS/反向代理、外部服务及负载门禁均未执行。

## 视觉与设计证据

继承现有海军蓝/青绿 Operate 世界，不重新选择方案。批准稿：
`.impeccable/mocks/decision/t03-setup-wizard.png`（1585×992）。
实现证据：`.impeccable/review/t03-entry-{desktop,mobile}.png`、`t03-hero-repro.png`；
120 字长名三语手机图为 `t03-entry-long-name-{en,zh-TW,zh-CN}.png`。
Root 逐张打开验证；无空白、错误页面或截断。

独立 reviewer 首次指出三项 P2：缺本地币名、辅助文字对比度不足、长串名称溢出。
修正后 computed-style 五组对比度 **5.15–5.53:1**，证据 `t03-entry-contrast.json`；
三语 390px 均无水平溢出。没有替换布局、字体或新增装饰。
本机工具不支持注入专用 reviewer 类型，因此使用全新 generic subagent 加载
Impeccable degraded review 契约；不是作者自行宣布通过。
定点 verdict 留痕：Humaux `58ccacca-a7fc-4c35-b0e4-7f5fb2d9f8ae`。

方向契约以 literal HTML comment 保存在首个**作者提供**的惰性 template 中，
随生产构建保留。Next 在它之前插入一个空的 hidden Suspense div；已明确接受
这个框架限制，不声称该注释是 body 首个 DOM 节点。真实 DOM 验证 comment 类型与 seed。

## 失败不隐藏

1. 根级契约测试最初假定 body 第一个元素是 template，实测发现 Next 的空 hidden
   前置节点。只接受该精确惰性前缀；仍要求契约是 comment、在 body 且位于业务 UI 前。
2. 作者的 mock 登录测试曾只等 request，再立即导航 callback，偶尔抢先于失败的
   外部导航。等待 load 暴露 `ERR_CONNECTION_CLOSED`：Playwright route 不保证拦截
   redirect 后的目标（[官方说明](https://playwright.dev/docs/api/class-page#page-route)）。
   修正为本机 HTTP mock 实际提供 authorize 页面，再等待 load/正文后 callback。
   不放宽应用安全规则、不依赖公网、不重试真实一次性 code。

失败日志 `t03-root-entry-final.log`、`t03-root-entry-accepted.log`、
`t03-root-entry-final2.log` 及三组 failure trace/result 工件保留在下述目录；
此前尝试不是 PASS。最终无 runner 自动重试或 skipped。

## 工件与收尾

目录 `/Volumes/data/output/live-commerce-cart-tests/`：

|工件|SHA-256|
|---|---|
|`t03-root-entry-loopback.log`|`12532ea795840aaef0b28309476d2d9318ac8f672be7fa605383697d53a3996f`|
|`t03-root-ledger.log`|`d3694afbd0daaafb696a1d18586166b1d6b298e60dab6b6e0077763aa70118b2`|
|`t03-root-identity-mock.log`|`c0043859a66824ef118730cac0ae36851be72c16eb17e361e56e9d9dbe786852`|
|`t03-root-final-identity.log`|`6c1e9981a7d5ba4df92ac51da719ed8133b029ef0c7ffae20c69033535e70969`|

最终真实身份链子工件：`output/playwright/identity-chain-20260920T091538.457737000/`。
作者 worktree `/Volumes/data/live-commerce-wizard-20260920`，base `b4976cf`；
root 独立合并复跑，审查 agent 无 fork 历史、只读，文档 agent 独立 worktree。
UI 作者路由为 `gpt-5.6-sol/high`；其余 reviewer 继承宿主配置，不虚构未披露的型号/费用。
未递归委派，未改依赖锁、客户数据库或生产服务。任务创建的 3100/3101/18081/19111
监听、临时 PG 与私有 env 已回收；保留截图、失败证据和工作树。

下一步依 DAG 先建立内部外部操作台账/outbox 合同，再冻结 BeginCheckout 的买家授权桥、
订单/报价/预留绑定及统一锁序。商家库存 Reserve 不能直接当买家权限使用。
