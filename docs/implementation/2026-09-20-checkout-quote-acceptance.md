# 下单前锁定报价重验：内部前置验收

2026-09-20，代码/最终测试基线 `0db6eb1`，基于已验收购物车/报价与 T06。
范围：`storefront.RevalidateQuote`，**不是 BeginCheckout、支付或配送验收**。
T11 只进入前置开发；完整 G03/G05 和 SaaS 上线仍未通过。

## 实现与团队

- 合同 `dc73e67`：[checkout-quote-validation-v1.md](../../contracts/checkout-quote-validation-v1.md)。
- 作者 `t06_operation_service`，`gpt-5.6-sol` / high，worktree
  `/Volumes/data/live-commerce-checkout-quote-20260920`，branch
  `commerce/checkout-quote-20260920`，base dc73e67；只写
  `internal/storefront/revalidate.go` 与 `quote.go` 的共享读取。
  af7fdbe → main 048ceaa；修复 0b82fc7 → main a68346d。
- Root `codex-commerce-build-20260920` 独立编写真实 PG 测试（0db6eb1），
  独立运行下面原始命令，不把作者离线包测试当数据库验收。
- 独立 reviewer `t06_schema_review`，`gpt-5.6-sol` / high，只读。
  设计检查及后续代码/测试复核分开；不依靠作者自证完成。
  最终 `PASS_BOUNDED`、无未关闭 P0/P1；Humaux 证据
  `42c09426-87ac-477c-ac88-f786f98ffe48`。此结论只覆盖报价重验及当时的权限方向，
  不自动覆盖之后新增的物流/金流配置合同。

复用已有 scope、cart/market/policy/catalog 锁顺序与唯一 `pricing.Calculate`；
没有新增库、数据库表、pool、HTTP、队列或第二个计价器。完整比较当前商品
身份/版本/文本、数量、policy、逐行和总金额。DB-clock 在所有锁等待后检查
created/expiry/TTL。返回原快照，不重新定价、不写 receipt/event/库存/operation。
GetQuote 仍可读过期历史；损坏 JSON 类型返回统一 conflict，不泄露解码细节。

## 实际证据

日志目录：`/Volumes/data/output/live-commerce-checkout-quote-tests/`。

|命令|结果|日志|
|---|---|---|
|`bash scripts/dev/test-local.sh`|exit 0；PG18.6 / Go1.27.1；128 顶层 PASS、0 FAIL、0 SKIP；race + vet；foundation 74.020s|`root-reviewed.log`|
|`bash scripts/dev/test-local.sh --browser-identity`|exit 0；Next production build、真实 Chromium→Next→Go→PG，签名 MOCK IdP；1/1 PASS|`root-browser.log`|

SHA-256：

```text
d75f8cccfd643c5248cda12c3c89ffe320e8d5557de37ebff65f0726d3d7fa0f  root-reviewed.log
5d675df42d04b442f661dc91476ba3f9e6502ec90b15f58478686d14c399506d  root-browser.log
```

浏览器工件：`output/playwright/identity-chain-20260920T111007.035153000/`。
本轮未改 UI，该运行是既有身份链回归，不是尚不存在的买家 checkout 页面验收。
128 包含之前的 123 项；新测试是 5 项顶层、40 个细分场景，不将总数当业务功能数。

## 覆盖与修正

- 正常快照完全不变且无持久化新事实；跨 owner/store/tenant、伪造 typed scope、
  缺失 ID、非法 ID/version、过期 expected version 拒绝。
- cart/version/数量、SKU price/version/status、product text/version/status、market
  version/status、policy version/disabled 变化均拒绝；历史报价仍可读且不变。
- 合成 owner-only fixture 注入 21 种快照/时间损坏：price/quantity/identity/text、
  line/tax/discount/shipping/aggregate、policy、JSON 类型、过期/未来/错误 TTL。
  没给普通角色新写权限，未修改迁移约束来适配测试。
- 真实第二连接证明 cart/policy/product/SKU 锁保留到调用方事务结束；价格在等待
  前变更后必须拒绝。过期测试先证明等待已发生且尚未过期，再用数据库时钟等待
  到截止之后。测试专用 3 秒行锁等待仍在原 5 秒请求期限内；生产配置未放宽。
- 取消后复用**同一 pool**进行新校验并读回空的 transaction-local buyer GUC。
  不声称 TCP 连接 ID 必定相同（驱动可以关闭取消连接）；已有独立的连接复用
  gate 继续随全套运行。没有把另一 pool 的成功误当取消恢复。

`root-initial.log` 是修正前的通过运行，不用它覆盖新增负例。独立审查指出：
raw json.Unmarshal error 需映射 conflict；首版取消测试误用了原 harness pool。
前者由作者提交修复、后者由 root 修正并增加 GUC readback；另把 900ms 主机时钟
等待改为有 DB 时钟证明的 gate。最终上述两条原始命令均重跑成功。

## 下步与限制

[buyer-checkout-v1.md](../../contracts/buyer-checkout-v1.md) 固定了独立 checkout
执行权限、private owner receipt 和 HELD-only/StartPayment 分离方向。
完整聚合 schema、destination/eligibility/allocation 与库存到期工作器仍待冻结和实现。
此 helper 不认证用户、不签发可复用授权、不证明可配送，且同一事务后续等待
仍可能过期；真正 hold 写入前必须再次检查 DB 时间。

生产 IdP、PSP、跨境物流、退款、公开买家路由/UI 与完整 SaaS 仍 NOT_RUN。
没有访问客户生产库、发布内容、付款、物流下单或中断直播。
