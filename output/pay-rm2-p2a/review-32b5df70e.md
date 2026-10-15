# 独立复核 32b5df70e（PR #70 · pay-rm2-p2a）

- 结论：**APPROVE**
- 基线：c7146aec5（已批准，见 review-c7146aec5.md）→ 32b5df70e
- 范围：`tests/foundation/browser_identity_chain_test.go`（+5/−14），没有其他文件改动
- 方式：只读静态复核。gofmt -l 输出为空。本地没有运行 Go 和浏览器测试（NOT_RUN），结论等级为 E1，最终以 CI 重跑为准。
- 触发原因：CI 38764411239 的 TestBrowserIdentityRealChain 在 DB 持久化证明一步失败，实测 accounts=0 credentials=0 version=0。旧断言在非 wizard 分支仍期望撤除前的 1/2/2 和方法 2/1。

## (1) 期望值改成 0 是否正确：正确

auth-real.spec.ts 在当前 head 下没有任何一次成功的账户或方法写入：
- provider-accounts 创建：`?unexpected=1`、缺少 Idempotency-Key、伪造 Bearer/X-Tenant-ID、重放、同 key 换 body 都返回 422（:362-409）。缺 CSRF 返回 403（:355）。同源 originControl 返回 422（:477），badOrigin 返回 403（:488）。限流循环中每次都是 422，直到出现 429（:589-606）。
- rotate 针对合成的不存在 ID `…0001`，主请求、重放和 stale 都返回 422（:428/:432/:441）。detail GET 返回 404（:413/:607）。decrypt 返回 404，DELETE 返回 405。
- list 两次都断言 `{items:[],next_cursor:""}`（:348、:442）。
- payment-method 保存：save、replay、hide 返回 422（:523/:527/:574）。`expected_version:2, enabled:true` 返回 409（:585）。GET 返回 404（:525/:576）。inspect 返回 404 not_found，不产生 command_result。
- Go harness 的种子只有非 wizard 分支在 initial-store 返回 200 后插入的一行 `pricing.markets`（:120-135）。没有任何地方写 `integration.merchant_accounts`、`account_credentials`、`payments.method_versions` 或 `method_heads`。spec 本身不连 DB，也就无法写入这些表。

因此两个模式下 0/0/0/0/0 和 0/0/0/0 都是唯一正确的期望值。SQL 层面，前面已断言 `stores==1`，`LEFT JOIN … GROUP BY s.tenant_id` 必定返回一行，计数为 0，`coalesce(max(credential_version),0)` 为 0，不会出现 ErrNoRows 假失败。

## (2) 改动是加强还是削弱：加强
- 撤除前，非 wizard 分支只断言"恰好 1 个账户、2 个版本"，没有防止撤除后的写路径悄悄落库。现在两条链路都要求零持久化：账户、凭据、凭据版本、operations、qualifications、方法版本、方法 head、enabled、inspect 命令全部为 0。
- spec 只断言 HTTP 422/409/404，这里再加一道独立的 DB 证明，确认被拒绝的写入确实没有落库，形成双重证据。
- 删掉的 "Wizard revision 2 concurrent editor" 注释和 2/1 期望，对应的 stale-draft/lost-reply 证明已迁移到 wizard 分支的 `fulfillment.service_versions` v2/v3 断言（:262-275），不是凭空丢掉的覆盖。

## (3) 下游是否依赖账户或方法存在：不依赖
DB 检查之后只剩两部分：wizard 专属的 markets/policies/services/commands 计数加 v2/v3 名称检查（只看 pricing 和 fulfillment），以及 IdP 的 exchanges/JWKS 证明。spec 在 :607 之后只做 logout 和 stale-session 401，同样不依赖账户或方法。`accounts.NewKeyring/New` 仍然用来组装 httpapi.Options，路由需要它才能返回 422，属于合理保留。

## (4) 是否还有撤除前的陈旧期望：没有
在该文件中 grep `payuni|rotat|credential|method|account|wantAccounts|wantMethod`，剩下的命中只有 import 和 handler 组装、已正确改写的 :269 迁移注释，以及 IdP 的 HTTP method 校验。:50 的注释 "wizard has no market seed" 与 :120 的行为一致。

## 非阻断 / 风险
- P3：:114 仍注入 `Accounts: accountService` 和 keyring。路由彻底下线时可以一起删除，本 PR 不要求。
- 剩余风险：结论只基于静态阅读，需要 CI 中的 TestBrowserIdentityRealChain 和 TestBrowserSettingsWizardRealChain 在 32b5df70e 上都通过，才能合并。
