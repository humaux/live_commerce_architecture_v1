# 商家自有供应商账户凭据 v1

2026-09-24，PASS_BOUNDED_INTERNAL_CREDENTIAL_REGISTRATION；代码/测试3063ed7。
合同最初基于 a21a7f7 冻结；[实际验收](../docs/implementation/2026-09-24-merchant-accounts-acceptance.md)
覆盖199项后端回归与既有身份浏览器兼容，不覆盖生产密钥装配、供应商验证或支付。
从[服务设置合同](merchant-service-settings-v1.md)补齐真实凭据登记，不把配置伪装为
供应商授权成功。首个支持录入的 provider 是 `payuni`；不创建交易、不自动启用方法。

## 范围与复用

- 商家货款归其自己的 PAYUNi 商店；`account_id` 对应 MerID，环境必须明确为
  SANDBOX 或 LIVE。不同环境是不同连接；账户/环境不可原地改绑。
- 复用 `core.Service.RegisterBinding/SetBindingEnabled`、`platform.RequirePermission`、
  `command.Run/Audit`；Go 采用 pgx 调用者事务，错误必须整体回滚。
- 内部 `accounts.New(keys, bindings)`；keys 为不可变 AES-256-GCM keyring。
  部署主密钥通过环境加载后注入，不写入数据库、源码、日志或命令回执。
  使用 Go 标准库，无新依赖。不要新增微服务、队列或第二套授权。
- 本单元只交付内部 Go/SQL。后续支付配置、内部 StartPayment 和
  [精确租约查询取凭据](payment-query-v1.md)分别交付，不授 worker 任意解密／扫库权限。
  自助凭据 HTTP/UI、真实账户验证和生产装配仍未交付。

## 冻结接口

包 `internal/integrations/accounts`：

```go
type Credentials struct { HashKey string; HashIV string }
type CreateInput struct {
    Provider string; Environment string; AccountID string; Credentials Credentials
}
type RotateInput struct {
    ConnectionID string; ExpectedVersion int64; Credentials Credentials
}
type Connection struct {
    ID string; Provider string; Environment string; AccountID string
    BindingID string; BindingVersion int64; Enabled bool
    CredentialVersion int64; KeyID string; State string
    CreatedAt time.Time; UpdatedAt time.Time
}
New(keys *Keyring, bindings *core.Service) (*Service, error)
(*Service).Create(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in CreateInput) (Connection, error)
(*Service).Rotate(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in RotateInput) (Connection, error)
(*Service).Get(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, connectionID string) (Connection, error)
NewKeyring(activeID string, keys map[string][]byte, replayKey []byte) (*Keyring, error)
```

可增加包内 helper/测试，但不新增其他公开凭据取出 API。
Keyring 的 seal/open 为包内函数，测试验证 roundtrip；生产 worker 读取以后须以
持久化 operation 的账户绑定和执行权限为入口，不可直接公开解密方法。
Create/Rotate 需要 `integration:manage`，Get 需要 `integration:read`；鉴权和
事务 scope 必须先于 replay，最后等待后再次鉴权。返回 `State=CONFIGURED_UNVERIFIED`
是固定诚实状态，不接受调用者传入 READY。Enabled 只是 binding 控制开关，不是 readiness。

## 数据与安全不变量

迁移 0014：`integration.merchant_accounts` 的 scope/id/provider/environment/account_id/
binding_id/credential_version；`integration.account_credentials` 不可变版本存
key_id、nonce、ciphertext、principal、created_at。复用 integration schema/RLS。

- Provider/account/environment/connection ID 不可变。binding 资产字符串固定
  `environment + ":" + account_id`，以 scoped composite FK 核对 provider/资产，
  不能挂别店或其他商户账户。每店 provider/environment/account 唯一。
- 当前版本外键延迟至提交，凭据版本必须属于同连接；头只允许更新 version 和
  updated_at。版本正文不可改／删，保留历史用于以后查询/对账；保留策略未上线。
- `commerce_runtime` 只可 SELECT 凭据表的非秘密元数据列，不能 SELECT nonce/
  ciphertext；只允许 INSERT 加密版本。worker/buyer/checkout/issuer 不授该表权限。
  主密钥只在配置服务内存，数据库所有者也不能只靠数据库内容恢复明文。
- AES-256-GCM 使用随机 12-byte nonce，16-byte tag；AAD 固定 JSON 结构含
  以下字段名和顺序：format_version=1、tenant_id、store_id、connection_id、provider、
  environment、account_id、credential_version。明文固定JSON字段顺序 hash_key、hash_iv。
  AAD 绑定 scope 与版本，跨连接/环境/版本替换、nonce/tag/ciphertext 损坏必须失败。
  keyring 复制 key bytes；1..16把唯一 keyID，每把32byte；activeID必须存在。
  不得回显 key bytes，旧钥缺失时安全失败，不能偷偷用 active key 解密旧版本。
- Provider密钥不进入命令请求/回执/审计/任务JSON或错误文本。幂等输入使用独立
  32-byte服务器 replayKey 的 HMAC-SHA256（带固定domain和scope）；不能用裸SHA256，
  否则允许的弱输入会被只读数据库者枚举。不使用随机密文摘要，否则重放会冲突。
  replayKey 独立从环境注入并复制入内存，不落库；它不随加密active key切换。
  永久幂等记录仍使用时不能直接换 replayKey；以后要轮换须先设计版本迁移，不能
  静默令旧请求冲突。保管与备份此部署密钥是上线前置，不能通过数据库恢复。
  HMAC消息固定为 Go encoding/json 的无额外空白UTF-8对象，字段顺序
  `domain,tenant_id,store_id,hash_key,hash_iv`，domain固定
  `merchant-account-credential-replay-v1`，摘要lowercase hex；actor、provider、
  environment、account及预期版本仍进入外层command.Run规范请求。Golden vector：
  replayKey为32个0x01，tenant-a/store-a/hash-key/hash-iv，摘要
  `8893dfddde5f8130f187c286bdf6258599583baf81034a3432f865bea5cc37c3`。
  Credentials 的 JSON/String/GoString 输出必须隐藏明文；不要记录原始输入。
- keyID 1..40 `[a-zA-Z0-9_-]`；MerID 1..64 ASCII 字母数字 `_`/`-`；HashKey/HashIV
  各1..512可打印非空ASCII、首尾无空格。此范围只是本地安全输入限制，不证明
  供应商接受格式；官方示例长度不当作正式规范。
- Create 在 outer command 内生成 UUID，复用 RegisterBinding，以内部生成 UUID
  为嵌套命令 key，避免截断用户 key；绑定、凭据、头、审计、两回执同事务。
  replay 不重新加密、不生成新 binding。不同 key 重复商户账户冲突并回滚所有事实。
- Rotate 锁定 account head、CAS credential version；不改 binding 的 semantic_version
  或 enabled。正常密钥轮换不等于换商家资产。轮换不自动变成已验证或启用支付。
  旧有效密钥留存不表示供应商旧钥仍有效，后续 adapter 必须记录实际使用版本。

## Gate

1. 加密 roundtrip/随机nonce/篡改/跨scope+环境+版本/缺旧钥/复制keyring/边界输入。
2. 真实PG Create/Get/Rotate读回；owner fixture读取已存nonce/cipher/keyID并用已知
   测试密钥独立解密，证明实际存储AAD/版本/明文对应而不只测内存算法。数据库无明文，回执/审计/错误无密钥；普通角色
   不能读密文列，外店/外租户/无权限/伪scope不能通过服务API读写；相同账户分环境隔离。
   DB RLS是可信runtime的scope隔离，不声称它单独识别每个integration权限；权限通过
   RequirePermission验证，不把伪造GUC后的runtime SQL当无权限终端用户。
3. 幂等 replay、异参冲突、不同 key 重复账户回滚、同 key 并发单赢家、轮换CAS
   与授权撤销后的历史重放拒绝；停用绑定不被rotate重新启用。
4. 绑定/头/凭据/审计/回执故障实际触及注入点后全回滚；不产生外部operation/job。
5. 全仓真实PG/race/vet及独立审查。没有UI变化时身份浏览器链只算兼容回归。

## 已知限制与升级信号

PAYUNi [官方文档](https://docs.payuni.com.tw/web/#/7/34)要求商家自己的 MerID、
HashKey、HashIV；单笔交易查询需要交易编号，当前尚未确认可用于初次验证的
无交易只读接口。不为探测凭据创建假订单。供应商 adapter/回调单独冻结并验收；
准备开放自助HTTP前必须补限流、CSRF、TLS、秘密输入清除及生产密钥管理/轮换流程。
正式密钥托管/KMS在部署环境确定后评估，不先引入未使用的SDK。
