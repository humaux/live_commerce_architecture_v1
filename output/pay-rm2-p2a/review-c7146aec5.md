# 独立复核 c7146aec5（PR #70，基于已 APPROVE 的 bd96be74b）

结论：**APPROVE**

复核方式：只读。`git show c7146aec5`，`git diff bd96be74b c7146aec5` 只改了 1 个文件（tests/foundation/account_process_test.go，+12/-3）；gofmt -l 输出为空。本地没有跑 Go 测试（NOT_RUN）。要拿到运行证据，需要在 c7146aec5 上重跑一次 CI，并看到 `TestMerchantAccountAPIProcessRestart` 通过。

## 1. 根因：属实
- `accounts.Service.Get`（internal/integrations/accounts/service.go:85-94）会 JOIN `account_credentials`，并把 `c.key_id` Scan 进 `Connection.KeyID`。
- HTTP 层的 `accountMetadata`（internal/httpapi/accounts.go:63-68）映射到 `accounts.Metadata`。这个结构没有 KeyID 字段，所以 key_id 不会出现在线上返回里。测试用 `mahRead` 把响应解码成 `accounts.Connection`，KeyID 因此恒为 `""`。
- `maHistorical`（tests/foundation/payuni_history_helpers_test.go:55-58）在 version>1 时使用 `fixture_v2`。`merchant_accounts.credential_version=versions=2`，所以 `m.get` 返回 `KeyID="fixture_v2"`。
- 旧代码 `reflect.DeepEqual(got, first)` 把 service 视图（KeyID 非空）和 HTTP 视图（KeyID 为空）相比，必然不等。这个失败是确定性的，不是偶发，和 CI 38761932865 第 190 行的报错一致。

## 2. 修复没有削弱检查
- 先断言 `seeded.KeyID == "fixture_v2"`。这把种子的当前版本钉死在 v2 密钥上，原来顺带隐含的事实现在成了显式断言。
- 只把 `want.KeyID` 清空，其他字段（id/provider/environment/account_id/binding_id/binding_version/enabled/credential_version/state/created_at/updated_at）全部经 `json.Marshal` 后逐字节比较。Connection 的 json tag 覆盖了全部字段，两边都含 `"key_id":""`，对称。
- `mahRead` 仍然检查响应体里没有 `"key_id"`、ciphertext、nonce 和明文，脱敏 gate 没动。
- 重启后的 `reflect.DeepEqual(got, first)` 和 list 的 `DeepEqual(page.Items[0], first)` 现在两边都是 HTTP 解码视图，类型和解码路径一致，比较有意义。`first` 又已经和 DB 种子做过字段级比对，基线链条完整。
- `maAssertPersistedSecret(t, m, first, …)` 只用 `a.ID` 查库，不依赖 KeyID，所以传 HTTP 视图没有问题。

## 3. 时间的 JSON 字节一致性：风险低
- 两边是同一个 PG 值（timestamptz，微秒精度）。种子侧是测试进程里 pgx 解码出的 time（Local），API 侧是子进程 pgx 解码后用 `json.Marshal` 序列化（RFC3339Nano，去掉尾部 0，带本地偏移），测试再解码、再序列化。RFC3339Nano 往返时瞬时值、精度和偏移都会保留，所以字节一致。
- 唯一的前提是两个进程的时区一致。`browserEnvironment` 继承 `os.Environ()`，只剔除 COMMERCE_/LC_/DATABASE_URL/POSTGRES_PASSWORD，TZ 会传给子进程；两者是同机同 zoneinfo。偏移为 0 时 Go 都输出 `Z`，和 Location 名无关。
- 万一将来子进程的 TZ 被改，结果只会是比较失败（误红），错误信息会打印 got/want，不会误绿。这比 DeepEqual 更严格：DeepEqual 连 Location 指针都比，wire 形式则同时比瞬时值和偏移。可选的加固办法：两侧在比较前都 `.UTC()`，或者把 TZ=UTC 写进 childEnv。不阻塞合并。

## 4. 测试里已没有 service 视图和 HTTP 视图的比较
本文件里只剩两处 `reflect.DeepEqual`（第 211、215 行），两边都是 HTTP 视图。`seeded` 只用在 KeyID 断言、wire 比对和 `seeded.ID` 路径上。

## 风险 / NOT_RUN
- NOT_RUN：本地 Go/PG 测试。合并前要求 c7146aec5 的 CI 全绿。
- 非阻塞建议：时间比较可以统一成 UTC，或者把 TZ 写进子进程环境，防止误红。
