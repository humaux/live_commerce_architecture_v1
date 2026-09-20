# 依赖准入与当前清单

版本来自当前 `go.mod`/`go.sum` 和本阶段已记录的锁定证据；license 是上游声明，不等于本项目已完成法务准入。升级流程是确认候选版本、更新锁文件与本表、在候选版本上跑 gate，通过后再合并。

|依赖|固定版本|许可证|实际调用者|用途|升级测试|
|---|---|---|---|---|---|
|Go toolchain|1.27.1|BSD-3-Clause|全部 Go 包|编译、标准库、测试|`go test -race ./...`、`go vet ./...`、govulncheck；记录实际 toolchain|
|`github.com/jackc/pgx/v5`|v5.11.0|MIT|`internal/platform/platform.go`；`migrations/migrate.go`；`tests/foundation/*`|PG 协议、显式事务、连接池、事务类型|真实 PG18 基础 gate：RLS、scope 回收、超时、回滚、池上限、HTTP；再跑 unit/race/vet|
|`github.com/riverqueue/river`|v0.40.0|MPL-2.0|`migrations/migrate.go`；`internal/integrations/core/service.go`；`tests/foundation/{foundation_integration,external_operation,external_operation_authority}_test.go`|同库任务迁移、事务入队、受限 worker 生命周期探针；尚无真实业务 dispatcher|重复/并发迁移、enum 事务边界、runtime 仅 insert/受限 update、操作/event/audit/job/receipt 同 commit/rollback、worker 实际启停和禁止改迁移历史|
|`river/riverdriver/riverpgxv5`|v0.40.0|MPL-2.0|同上|River 的 pgx driver|同 River gate；确认 schema 与权限不漂移|
|`github.com/coreos/go-oidc/v3`|v3.21.0|Apache-2.0|`internal/oidclogin`|发现、JWKS 签名及 issuer/audience/expiry 验证；nonce/subject/azp 由适配层额外验证。避免自制 JWT 验证器|签名 mock IdP 正负例、轮换/网络失败边界、真实提供商 sandbox 后才可上线|
|`golang.org/x/oauth2`|v0.37.0|BSD-3-Clause|`internal/oidclogin`|固定回调的授权码交换与 S256 PKCE；不存储提供商 token|错误 verifier、code replay、端点/重定向与超时负例|
|Node.js / pnpm|24.15.0 / 10.33.0|MIT / MIT|根 `package.json`；`packages/i18n/tests`|现有本机运行时；原生 TypeScript strip 执行纯函数测试，pnpm workspace 锁定解析|`pnpm run test:i18n`；所有 workspace 集成测试；不跨大版本自动升级|
|TypeScript|7.0.2|Apache-2.0|根 `package.json` 的 `typecheck:i18n`|编译期严格检查三语言字典和公共路由类型，非浏览器运行依赖|`pnpm run typecheck:i18n`；新 UI 接入后加入应用 strict build|
|Next.js / React / React DOM|16.3.5 / 19.3.0 / 19.3.0|MIT|`apps/admin/app`、`proxy.ts`、BFF route、客户端 Ledger|多语言 SSR 路由、交互和服务端令牌边界；遵循既定 Next 基线，未另外引入状态管理/表格库|strict typecheck、standalone build、三语言/手机/权限/幂等真实浏览器 gate|
|Playwright Test|1.63.0|Apache-2.0|`playwright.config.ts`、`tests/admin`|真实 Chromium 浏览器与网络丢响应/存储故障注入|九项台账回归 + BFF mock边界 + 实际PG/Next/Go签名mockIdP浏览器链，三组fixture隔离；不得把 mock 当生产验收|
|Prettier|3.9.8|MIT|开发格式化命令|将样式和 JSX 标准格式化以维护；不进运行时|格式化后 strict build 和浏览器回归|
|React/Node TypeScript types|React/DOM 19.3.0，Node 24.13.6|MIT|`apps/admin` TypeScript 编译|与已锁定运行时相配套，非运行依赖|strict typecheck|

间接模块由 Go module 解析，不在业务代码中直接调用；升级仍须保留 `go.sum`、扫描漏洞并跑上述 gate。不得用版本兼容或 CI 绿灯推断生产可用。

## 计划依赖（当前 NOT_INSTALLED）

|依赖|状态与边界|准入条件|
|---|---|---|
|sqlc|NOT_INSTALLED；当前 SQL 为显式参数化，尚无生成物|冻结 queries/schema 后由 integrator 锁版本，审生成 diff，跑真实 PG/RLS/事务 gate|
|LiveKit|NOT_INSTALLED；媒体与 API 尚未接入|先完成托管/自托管能力与授权探针、mock→sandbox gate；媒体失败不得改变订单真源|
|PSP|NOT_INSTALLED；无真实收款/退款调用|先确认商家 MoR、sandbox 账户与 webhook 幂等/对账；live 需明确授权、金额、回执和回滚边界|

任何新依赖须说明为何标准库/现有包不能满足、调用者、license、版本来源、移除/升级测试和生产影响；未满足前标 `NOT_INSTALLED` 或 `BLOCKED_EXTERNAL`，不伪造可用性。

买家匿名凭证切片不新增依赖：`internal/buyer` 使用现有 pgx 的参数化 SQL/事务，
标准库 `crypto/rand` 生成 32 字节随机凭证、`crypto/sha256` 仅持久化摘要。
`platform.OpenBuyerPool/OpenBuyerIssuerPool` 复用启动角色检查，但不复用商家权限。
SQL 依赖入口及将来开放 HTTP 前的条件见 `contracts/buyer-capability-v1.md`；
改动 PG/pgx 或事务包装时，必须重跑真实数据库的角色矩阵、撤销/过期锁竞争和回滚负例。

内部 pricing/cart/quote 同样不新增依赖：`internal/pricing` 以现有 pgx 和受限整数
计算；`internal/buyer/command.go` 复用事务、JSON/SHA-256；`internal/storefront`
消费这些服务及 catalog 的 PG 行锁。升级 PG/pgx 时追加运行
`tests/foundation/{pricing,cart_quote}_test.go` 的策略版本、金额、RLS、CAS、锁竞态、
并发 replay、原子回滚 gate。合同与证据分别见 `cart-quote-v1.md`、
`2026-09-20-cart-quote-acceptance.md`；没有引入第二交易引擎。

内部 external-operation 继续复用 pgx、River InsertTx、`command.Run` 与标准库
JSON/crypto：`internal/integrations/core` 负责精确权限、不可变意图摘要和租约 token
包装；`migrations/0008_external_operations.sql` 是 worker 状态迁移的唯一写入口。
`platform.OpenWorkerPool` 拒绝 owner/混合权限；`migrations/migrate.go` 在 River
迁移之后重设运行权限，并明确收回 worker 对 `river_migration` 的所有权限。
升级 River/PG/pgx 必须重跑 `external_operation*_test.go`：单赢家 claim、过期仅
reconcile、旧 token/generation 拒绝、binding 撤销仍保留远端已知结果、跨权限拒绝、
五类事实原子回滚、缺 receipt 后永久摘要仍冲突，以及普通 worker 的真实启停。
该探针不构成 dispatcher/provider 验收；详见 `2026-09-20-external-operation-acceptance.md`。

2026-09-20：上述前端版本通过 npm registry 元数据与本地锁文件核对。`pnpm audit --prod` 返回 No known vulnerabilities found；这是当时依赖审计，不代表整体安全验收。Next 的匹配版本文档随 `apps/admin/node_modules/next/dist/docs` 提供；standalone 显式补齐 public/static，不使用有警告的 next start。

2026-09-20：OIDC 版本由 `GOTOOLCHAIN=go1.27.1 go list -m -json <module>@latest` 核对后固定，不使用漂移的 latest 构建。许可证已读取下载模块 LICENSE。调用边界参考 [go-oidc 官方示例](https://github.com/coreos/go-oidc/blob/v3/README.md) 与 [OAuth2 PKCE API](https://pkg.go.dev/golang.org/x/oauth2)。标准库没有完整 OIDC/JWKS 验证器；移除依赖前必须替换完整验证能力，不可改为只解码 JWT。真实 IdP、邮件和浏览器会话仍是独立 gate。
