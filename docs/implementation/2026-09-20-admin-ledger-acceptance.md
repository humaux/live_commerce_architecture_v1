# 商品库存账簿：三语言 UI 与隔离 API 验收

日期：2026-09-20。主线后端基线 `7f9ed8c`，UI/测试/隔离 fixture/打包脚本提交 `43d27f5`；后续文档提交只归档真实设计与边界。**只证明商品库存账簿切片；不是全 SaaS 上线许可。** 客户生产、直播、支付、订单和消息均未写入。

## 已验证范围

- 用户批准的商品对账簿世界（`6ecd43a2`）与宽表格＋下方操作托盘（`a30e0e29`）；先确认视觉稿再实现。
- `/zh-CN`、`/zh-TW`、`/en` 路由，显式选择优先于 cookie 和浏览器语言；切换保留当前查询，不改变店铺、权限或货币。
- 真实 PG18 → 普通 runtime 角色 → Go Scope/事务 → Next 服务端适配 → 浏览器。页面没有伪造 API 数据；演示商品和图片均显式标识。
- 仓库/状态/字面搜索与 keyset 分页、单行详情、必须原因的库存调整、版本冲突、商品与首个 SKU 两步创建。
- 网站客服、Meta 消息、平台支持为独立导航域。未实现模块明确提示不可用，未连接渠道不显示为已连接。

## 验收记录

|检查|结果与证据|
|---|---|
|真实浏览器完整回归|最终 9/9 PASS，0 skipped/flaky；2026-09-20T05:21:41Z，7.871 秒；`evidence/2026-09-20-admin-ledger/browser-results.json`|
|桌面/手机|桌面及 hero 为 1586×992，与批准稿原始尺寸一致；手机 390px，无页面水平溢出；`.impeccable/review/{desktop,hero-repro,mobile}.png`|
|断线重试|服务端已提交但响应丢失后，原 idempotency key 和原始请求字节重放，余额只增加一次|
|关闭标签页恢复|未确定命令写入持久 journal；新标签页继续原命令；存储不可用时发送 0 个写请求|
|并发冲突|外部更新后旧版本返回 409，刷新读到真实新余额；不盲目重新生成写命令|
|BFF 拒绝|跨店铺、外来 Origin、未开放路由、超长请求拒绝；JSON 错误及 32 位 request ID；浏览器 HTML 不含 fixture token|
|生产打包复验|单独复验 1/1 PASS（05:12:54Z，0.868 秒）并在最终 9 项中对最后一次重建包再次通过；生产即使收到 fixture 环境变量也 401；图片资源 HTTP 200；`standalone-production-results.json` 与最终回归文件|
|语言与类型|共享词典/路由 4/4 单测；i18n 与 admin TypeScript 检查 PASS|
|构建与格式|`pnpm build:admin`、Prettier check、`git diff --check` PASS；生产包附带 public 和 static|
|fixture 安全负例|3/3 PASS，0 skipped，0.473 秒；一次性私有文件与 symlink 拒绝；配置拒绝远程 host/多 host fallback/其他 DB；真实 PG public-only 旧数据拒绝，保留 marker 且不迁移；`evidence/2026-09-20-admin-ledger/fixture-guards.jsonl`|
|激活样式|新增浏览器用例 1/1 PASS；真实文字选中/光标/740px 横向滚动 readback；`visual-states-results.json` 和 `.impeccable/review/active-style-evidence.json`|
|设计规范归档|`DESIGN.md` 与 `.impeccable/design.json` 从真实样式抽取；YAML/JSON、token引用、组件属性白名单与sidecar引用校验通过；未推定的统一行高/字重未写入规范|

独立视觉第二轮定点复验 `ship`，评分清单剩余项清零，无新可见回归；不是全产品审查。视觉记录：`.impeccable/review/finish-review.md`。fixture 安全独立复核 P0=0/P1=0，macOS `ps eww` 无 owner DSN/password、一次性文件已删除、无 idle postgres，Humaux `b96d4671-a43f-4dbd-9297-5d1de490eb51`。BFF 两项 P1 已独立关闭，Humaux `3ce8a6cb-1578-4df9-8e64-dea62164d6fc`。

## 根因修复与诚实边界

- localStorage journal 在发送前写入并回读确认，Web Locks 防止同店跨标签竞争。响应不确定不能换幂等键；已成功的商品 ID 在清 pending 前保存，避免重复创建。
- Next 开发服务会正规化 URL host；Origin 校验改用严格白名单的 Host/Origin 配对，仅限 `127.0.0.1:3100` 或 `localhost:3100`，没有放宽为任意 Origin。
- pgx ConnConfig 字段变更不会改写 `ConnString()` 原始 DSN。fixture 显式构造 runtime DSN；关闭 owner pool 后才启动服务；拒绝 fallback 和非空 public schema。macOS 的 exec-time 环境不能靠 Go `Unsetenv` 清除：owner DSN 改经同目录 0600 一次性文件传递，在连接前校验、读取、删除，进程参数/环境仅含文件路径。
- Next 开发浮标会遮挡手机行，使用受支持的 `devIndicators:false` 关闭。框架仍保留隐藏工具 DOM，验收断言应检查不可见，而非不存在；真实截图没有浮标。光标用 ArrowRight 折叠选择再验收，不用 macOS 上语义不同的 End 键。标准 overlay 滚动条可能在静态截图自动隐藏，保留实际滚动位置和计算样式证据，不伪造常显截图。
- 账簿搜索原 LIKE escape 触发 SQLSTATE 22025；改成 `ESCAPE '!'`。独立真实 PG 测试覆盖 `%`、`_`、`!`、反斜线、游标绑定与双权限。后端证据在 `/Volumes/data/output/live-commerce-ledger-tests/fixed-realpg.log`，原失败记录保留。
- 本轮第一次浏览器复跑未给测试 runner 加载 fixture 环境变量，第 5 项明确失败，2 项未运行。修正启动步骤后完整 8 项通过；增加激活样式覆盖并修正隐藏开发 DOM 的断言后，在全新隔离数据库完整 9 项通过。
- 最新完整 Go race/vet 聚合命令**不能记 PASS**：macOS 首次启动数个测试二进制长时间没有 RUN，超过外层 180 秒；独立 ledger PG 和 foundation 测试已通过，部分直接单测后续通过。历史 `0f631ac` 全套 PASS 不等于最新版本全套 PASS。未关闭系统安全服务或放宽业务超时。

## 可重复本地运行

仅本机 loopback；禁止暴露 tunnel/公网或接入客户数据库。Docker 需已具备脚本固定摘要的 PG18 镜像，Go 1.27.1、Node24、pnpm10.33.0。

1. `bash scripts/dev/admin-fixture.sh`：前台保持；只创建带专属 label 的临时 PG18 容器，打印私有 `session.env` 路径，不打印凭证。
2. 在独立终端执行 `set -a`、`source <本次打印的完整 session.env 路径>`、`set +a`，再 `pnpm dev:admin`。
3. 测试终端同样加载本次 env；`pnpm build:admin`，再用另一个终端加载同一 env，执行 `COMMERCE_UI_PORT=3101 pnpm start:admin`。
4. 首次完整回归：`pnpm test:admin`。需要 Playwright Chromium。第一项要求新 fixture 的 9 个 SKU，后续会创建第 10 个；再次完整执行前重启 fixture，不能接客户库清数据。
5. 测试结束停止自己启动的 UI、生产预览与 fixture；fixture trap 只回收本次 label 的容器、私有 env 和二进制，不清共享缓存。

本轮清理回读：自己创建的 fixture 容器、私有 env/owner 文件和临时二进制已回收；3100/3101/18081 无遗留监听。这里只删除可重建的隔离演示数据库，截图、审查和测试证据保留，未碰客户数据。

## 仍未完成

正式身份提供商/登录/商家自助开店、域名绑定、多商店选择、买家前台、价格市场/税费/折扣、商品媒体属性、购物车结账、真实支付/退款对账、跨境超商履约、直播工作区、独立会话域和 Meta 接入等。生产页面当前安全拒绝登录数据访问，不是可投产的账号系统。演示保留量用真实事务创建，但 fixture 不运行过期 worker。全局 gate 仍未通过。
