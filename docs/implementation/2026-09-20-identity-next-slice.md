# 下一开发单元：正式身份与商家首店

状态：实施前边界草案，**不是已实现/已上线**。依据 `PRODUCT.md`、`架构.md` §身份安全、`contracts/foundation-v1.md`、现有 SQL 和只读复核记忆 `775ead13-f23f-449e-981a-f26919ec7762`。全局 G01/G02/G11 未通过。

## 已确认，不重新选型

- 经验证的 OIDC 负责认证，本系统的权限集合负责授权；Meta OAuth 只绑定渠道。
- 平台、商家、买家、支持的会话受众明确分离；不按同邮箱自动合并主体。
- 浏览器生产会话使用 Secure/HttpOnly、明确 SameSite 的本地 cookie，写请求有 CSRF 防护；敏感管理员/财务动作再认证或 MFA。
- 当前 `commerce_runtime` 不能读写认证表；新认证能力不得靠扩大商品 API 的数据库权限实现。
- `apps/admin` 的环境 bearer 仅开发 fixture，不能升级为生产登录方案。

## 可先推进的最小单元

先冻结协议与测试反例，再实现提供商无关的 OIDC code + PKCE/state/nonce 边界、以 `(issuer, subject)` 为键的主体映射和 merchant 本地会话。仅允许隔离 `PROVIDER_MOCK` 实测，生产启动缺真实配置则拒绝；mock 不是 sandbox 或 live。

首店开通应为原子、幂等事务：tenant、首店、owner membership、服务端权限集合和审计同成功/同回滚。不引入新缓存/消息系统，不让浏览器提供可信权限。并行作者只在合同冻结后的独立 worktree 写互不重叠路径，迁移/共享合同由 integrator 合并。

## 必须验收

1. issuer/audience/signature/state/nonce/PKCE/replay 反例；精确 callback、无任意 redirect、无同邮箱越权合并。
2. 会话撤销、过期、cookie/CSRF、401/403/404；平台/商家/买家/支持不得混用。
3. 真实 PG 并发开店唯一性、幂等回放、故障完整回滚、跨租户/店铺隔离、runtime 仍无认证写权限。
4. 三语言浏览器完整登录→开店→账簿→退出；不得只测 API 或展示“已连接”。独立 P0/P1 审查与 root replay 后才能更新切片状态。

## 外部选择与停线

具体 IdP/邮件服务、开放注册或邀请制、正式权限包、会话期限、首店币种/市场/套餐配额、平台员工是否独立 IdP client/realm 尚未确认。可以先做不依赖这些选择的协议/隔离测试，不能自行开通付费服务、注册生产回调、发真实邮件或开放公众注册。已有 TWD 样例不是正式市场或税费政策。

未选环境 token/自制密码库作为生产认证：前者没有用户身份生命周期，后者扩大安全与维护面且违背既定 OIDC 决策。升级信号是签约 IdP、真实邮件域名与上线策略明确，届时补 provider sandbox/真实用户验收，不以本地 mock 代替。
