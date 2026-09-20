# T03：商家身份及首店事务后端验收

2026-09-20。生产代码基线 `582b7de`（OIDC adapter）＋`cc4bc3c`（身份服务、SQL、角色边界）；独立测试整合 `6b1a5c2`。**这是后端库/数据库切片，不是已上线的登录产品，不代表 T03 或 SaaS 全局 gate 通过。** 没有写客户生产、发布登录端点或注册真实 IdP。

## 结果

|边界|实测证据|
|---|---|
|OIDC 协议，PROVIDER_MOCK|7 个顶层测试组：签名、issuer、audience、expiry、nonce、subject、azp、S256 PKCE、code replay、HTTPS/loopback 与网络重定向拒绝。使用签名 mock IdP；不是外部提供商 sandbox。|
|身份与首店，REAL_PG|12 个顶层测试组：state/browser-binding/expiry、单次消费、同一主体并发登录、不同 issuer 不合并、过期/撤销/错误 audience、首店并发幂等、跨租户、最终审计失败完整回滚、注销/开户竞争。|
|数据库权限，REAL_PG|登录组只能 EXECUTE 五个固定函数；不能直接读写身份表或业务表。业务 runtime 和 PUBLIC 无权执行身份函数。函数均 SECURITY DEFINER、固定 pg_catalog search_path。|
|连接身份|拒绝 migration owner、runtime/identity 混用、writer membership、CREATEROLE、CREATEDB、REPLICATION，以及非系统 relation/function/schema owner。|
|全 Go 聚合|`bash scripts/dev/test-local.sh`，exit 0；全部包 `go test -race -count=1 -timeout=120s -v ./...` 及 `go vet ./...` 通过。75 个顶层 PASS 记录，0 FAIL、0 SKIP；foundation 5.632 秒。|
|独立重跑|另一 agent 从干净 detached `6b1a5c29db903575d1e617ae03da18095d7a2dad` 在独立临时 PG cluster 重跑同一脚本，exit 0、无 SKIP；5/5 独立身份 gate 通过，foundation 5.049 秒；测试容器回收、worktree clean。|
|Fixture 安全|聚合脚本在自己的临时 PG18 cluster 内另建 `lc_admin_fixture`，使真实数据库非空保护测试也执行，不再以 SKIP 当 PASS。无开发者数据库或运行中 UI 数据改动。|
|依赖检查|`GOTOOLCHAIN=go1.27.1 go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...`，exit 0，No vulnerabilities found；这不等于没有业务或部署风险。|

原始证据（不含生产凭证）：

- `/Volumes/data/output/live-commerce-identity-tests/root-final.log`，SHA256 `518607061f5b1e2d2a953c472562ee3bc405134a68e44bc1b39de4a35fc48e8e`。
- 同目录 `root-initial.log`、`root-roles.log`、`root-all-guards.log` 保留首次、角色补测和消除 guard SKIP 的中间记录；`govulncheck.log` 为依赖扫描。
- 独立 agent 的复跑只留在当次终端工具回执与 Humaux 记录中，没有另存日志文件，不虚构独立日志路径或摘要；root 的完整原始日志如上保留。
- 固定 PG18 镜像摘要见 `scripts/dev/test-local.sh`；Go toolchain 1.27.1。测试容器只按本次名称及 ownership label 回收，保留日志；无全局清理。

## 独立审查与根因修复

初版 trusted login role 直接持核心表 DML，能绕过业务入口。独立审查提出 P1 后，改为 `commerce_identity` 仅有 schema USAGE 与函数 EXECUTE；非登录 `commerce_identity_writer` 持表权并拥有五个固定函数。函数不接收任意租户、主体、权限或审计 action，权限包只能在新建首店事务内生成。SQL 审查重验 P0=0/P1=0，Humaux `0be2b661-af8b-434b-91ea-b6d14dac1825`。独立测试由另一 agent 编写后由 root 整合重放；作者不是唯一验收人。

不是所有审查疑点都靠改代码关闭：忽略 `crypto/rand.Read` 返回错误的疑点，经独立核对 Go 1.27.1 本机标准库契约撤回——它保证填满或不可恢复终止，不存在失败返回后继续发零值 token 的路径。代码记录该版本前提，没有为注入测试改用更弱随机源。

最小化复用现有 PG 会话 resolver、事务和审计；没有自制密码库、JWT 验证器、Redis 会话层或把 fixture bearer 升级成正式登录。OIDC 与 OAuth2 新依赖的固定版本、许可证、调用者及升级 gate 见 `dependencies.md`。

## 仍未通过的范围

- 生产 IdP/邮件、开放注册或邀请制、正式币种/市场/套餐策略、平台员工身份 realm、MFA/再认证及生命周期运维未选定。
- `internal/identity` 尚未接入公开 HTTP/BFF；Secure/HttpOnly cookie、CSRF、防登录滥用/速率限制、过期 flow 清理、浏览器登录→首店→账簿→注销闭环尚未实现或验收。`New` 的 onboarding 默认禁用，必须明确策略。
- 登录/首店页面需遵循用户“先视觉稿确认再开发”。本轮没有跳过视觉确认写新 UI；现有商品库存 UI 的浏览器证据沿用自己的验收文档，不冒充新登录 UI 验收。
- 其他 audience、域名、多店生命周期、结账/支付、跨境超商履约、直播、原生聊天/Meta/平台支持等仍按任务合同继续；G01/G02/G11 不得全局标 PASS。

本记录更新了旧 UI 验收时“最新 Go aggregate 未通过”的状态：当时冷启动超时仍是历史事实，本轮上述指定后端基线的 aggregate 已实测通过；不能反推所有 UI 或未实现功能也通过。

任务合同更新后，`python3 scripts/check_packet.py` 返回 `PASS_PACKET_STRUCTURE_ONLY`；这里只校验结构和依赖 DAG，不替代运行时验收。验收后仅收紧一条独立测试失败诊断，避免格式化整个 `Session` 泄露一次性测试 token；`go vet ./...` 再次通过，生产代码未变。
