# ADR-20261026 Owner choices: Stripe 信用卡/金融卡/Link、快速上手、再认证、代码对齐 22 题

状态：已决定（owner 2026-10-26）。证据级：DESIGN（契约修订；所有 gate NOT_RUN）。
规范性文本只在 `contracts/owner-choices-20261026-v1.md` 和下文各契约条款；本文只写理由，不复制条款。

## 权威
- owner 原话（2026-10-26）：「ad-01、Stripe不是支持信用卡、储蓄卡、link收款嘛？ad-03、让商家能够简单配置，快速开店，其他的按照你的结论来」。「你的结论」= `output/figma-ui-completion-20261025/ADR-CHOICES.md` 每题的推荐项。
- owner 先前原话 O1–O9：`output/integrator/tools/logs/owner-decisions-20261026.md`（O9 是 integrator 可逆裁决，不是原话）。
- Stripe 一手来源（2026-10-26 取）：https://docs.stripe.com/payments/link/checkout-link 。要点：手动列支付方式时 `payment_method_types` 要同时含 `link` 和 `card`，且同一 session 内币种一致；Link 付款时 `payment_method.type=link`；Link 支持信用卡、借记卡和符合条件的美国银行账户；Checkout 内 Link 无额外费用，费率同其他卡付款。
- 安全复核 T21R-01：`output/r3-t21-review-20261026/FINDINGS.md`（owner 选 A：实现再认证）。

## 一、点名的三项
### AD-01 Stripe 支持信用卡、金融卡、Link
- 决定：收款方式从「仅 card」改为 card + link。信用卡、金融卡本来就是 `card`；Link 的付款类型是 `link`。设置页显示「平台統一收款・已可使用」，支持方式写信用卡、金融卡、Link，商家无申请、无配置。
- 依据：owner 原话；Stripe 页面（见上）。Link 卡类付款与卡同样即时扣款，所以 `paid` 仍产生 CAPTURED，不产生 AUTHORIZED。Link 也可用美国银行账户，那是延迟方式，不在 TWD/HKD 范围，任何非 `paid` 或异步结果仍只进 review。
- 放弃：A 保持 card only（与 owner 要求不符）；B 改成 Stripe 动态支付方式（会混进 FPS、支付宝等延迟方式，释放语义没有设计，D6 已排除）。
- 契约：`stripe-psp-v1.md` D6、Q3、§5 请求示例、§16 限制与 UNKNOWN、SP16、record_stripe_observation；`stripe-refund-v1.md` §0；`stripe-platform-account-v1.md` AD-PF3；`docs/delivery/units/w4-u1-payment-activation-ui.md`。对 card 原有约束一律不放宽。
- UNKNOWN：HK 平台账户对台湾买家开放 Link 需要 SANDBOX 实测；在 owner 记录探针结果前，LIVE session 保持 `[card]`。
- 实现卡：OC-01（internal/integrations/psp/stripe/**）、OC-07（设置页）。

### AD-03 快速上手
- 决定：3 步（店名 → 配送 → 第一个商品）。登录后进看板；快速上手只对新建的店显示，老店不强迫。
- 依据：owner 原话与 O2；O9。放弃：四步供应商向导（已不适用）；老店也强制重走。
- 契约：`merchant-settings-wizard-v1.md` AD-03 修订段。实现卡：OC-05（服务端标记）、OC-06（界面）。

### T21R-01 近期再认证
- 决定：选 A。会话记录 `authenticated_at`；新增 `identity.require_recent_auth`，窗口默认 10 分钟（可配置）；覆盖 staff_invite/set_role/remove、平台 Stripe 开关、退款（含 offline refund）、银行转账确认、导出、erasure；方式先用密码 + 邮件验证码，TOTP 后续；`staff_invite` 暂时禁止 role=owner。
- 依据：FINDINGS T21R-01（P1）；`merchant-password-auth-v1.md` §11 升级信号「出现 staff 或 payouts 就加 TOTP 或 passkey」已触发。放弃：只让 owner 书面接受风险（留着被盗会话即可自邀 owner 的路径）。
- 契约：`merchant-password-auth-v1.md` §16。错误码沿用 PT401（message `reauth_required`）。实现卡：OC-02、OC-03、OC-04。

## 二、22 题（均选推荐项）
### Q1 商家自收银行转账（AD-02、FIN-01、FIN-02）
- 决定：B：关入口，历史只读保留。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：O1 要求商家不再自己收款，但没有授权删除历史转账记录；删历史账务不可逆。A（全保留不动）与统一代收冲突；C（连报表列也拿掉）会改 CSV 并丢历史。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q1；merchant-orders-v2、reporting-v2、buyer-checkout-options-v1
- 实现卡建议：OC-08（apps/admin/components/BankTransferSettings.tsx、lib/merchant-tools-model.ts）

### Q2 结算单「退回/带回」（FIN-03）
- 决定：A：不新增显示。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：O1 没提到退回与带回；放弃 B（新字段加迁移）。冲突：SA-0 在此题写成后冻结了 returned_in_minor/payout_returned，所以 SA-2 暂停，等 owner 一句话确认，SA-0/SA-1 后端不受影响。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q2；stripe-platform-account
- 实现卡建议：SA-2 加 note 暂停

### Q3 结算页用词（FIN-04）
- 决定：A：保持现状。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：O5 只管买家文案，商家结算用「撥付」口径另一套；放弃 B（改词不改账）。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q3；stripe-platform-account
- 实现卡建议：无实现卡

### Q4 运费模型（LOG-05）
- 决定：A：固定运费 + 满额免运。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：O2 要简单；放弃 B（每方式各设运费，扩大费用模型）。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q4；delivery-service-v1
- 实现卡建议：无实现卡（保持现状）

### Q5 货到付款底栏（LOG-06、CP-03）
- 决定：A：显示「到貨需付」，金额取服务端报价。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：O4 保留货到付款；买家应在下单前看到真实应付，前端不自行加价。放弃 B、C。白话文案需 owner 再看一次。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q5；buyer-checkout-options-v1、storefront-v2
- 实现卡建议：OC-09（apps/storefront/components/CheckoutFlow.tsx、lib/cod-copy.ts）

### Q6 解除 Meta 授权（AD-12、CP-05）
- 决定：A：不加。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：现有文案符合代码；B 要新增撤权能力和迁移，原话未提。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q6；meta-ads-v1
- 实现卡建议：无实现卡

### Q7 无权限提示（AD-10）
- 决定：A：统一一句，不点名角色。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：不凭角色名猜权限，只改展示，不动 ACL；放弃 B。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q7；admin-transport-v1
- 实现卡建议：OC-10

### Q8 宅配地址标签（LOG-08）
- 决定：A：不做，标「未納入產品」。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：无原话；B 涉及买家 PII 打印，C 买面单违反 AGENTS.md 红线。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q8；taiwan-cvs-logistics-v1
- 实现卡建议：无实现卡

### Q9 直播回放入口（SF-01）
- 决定：A：不加。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：原话未提；来源、范围未定。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q9；storefront-v2
- 实现卡建议：无实现卡

### Q10 发布结果未知（SF-04）
- 决定：A：「發布結果待確認」，按原请求查。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：AGENTS.md「外部 UNKNOWN 不盲重试」，与 O8 同原则；放弃 B（一般失败，易诱导重发）。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q10；storefront-v2
- 实现卡建议：OC-11（apps/admin/lib/design-client.ts、components/Design.tsx）

### Q11 暂停营业（SF-07）
- 决定：A：保持一种提示。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：暂停营业需要独立服务端状态，原话未提。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q11；storefront-v2
- 实现卡建议：无实现卡

### Q12 收件匣未读筛选（AD-04）
- 决定：B：加（推荐项；无原话，仅供参考）。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：客服常用；需先定义「未读」并新增店铺范围查询。放弃 A。未定义前不显示该筛选。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q12；meta-inbox-v1
- 实现卡建议：OC-12（internal/inbox/**、apps/admin/components/Inbox.tsx）

### Q13 营销同意状态（AD-05）
- 决定：A：不显示。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：系统无此字段；不得从可回覆窗口推断同意（AGENTS.md）。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q13；meta-inbox-v1
- 实现卡建议：无实现卡

### Q14 Meta 异常邮件开关（AD-08）
- 决定：A：不加开关。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：这封是店铺连线坏了的提醒，关掉有风险。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q14；meta-connection-health-v1
- 实现卡建议：无实现卡

### Q15 SKU 级配送资格（CAT-02）
- 决定：A：不要。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：O2 要简单；新增字段牵动结账判断。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q15；catalog-inventory-v1
- 实现卡建议：无实现卡

### Q16 独立自配送方式（LOG-04）
- 决定：A：不加，保持「未開放」。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：O2；设计图自己也标未开放。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q16；delivery-service-v1
- 实现卡建议：无实现卡

### Q17 已出貨邮件（CP-07）
- 决定：A：保留邮件，改旧契约句。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：代码（0090）与 storefront-v2 都已在发；Messenger/IG 才涉及同意规则；放弃 B（停邮件）。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q17；manual-fulfilment-v1（原处改 MD10 和限制项）
- 实现卡建议：无实现卡（只改契约）

### Q18 断网/出错页（SF-06）
- 决定：A：只重读 + 离线提示条。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：重读是读操作，幂等安全；不承诺输入保留。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q18；storefront-v2
- 实现卡建议：OC-11（apps/storefront/app/[locale]/error.tsx）

### Q19 空状态按钮（AD-11）
- 决定：A：只放说明文字。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：最简；不涉及角色判断。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q19；docs/design/DESIGN-SPEC.md（经 owner-choices OC-Q19）
- 实现卡建议：OC-10

### Q20 出货前核对页（LOG-07）
- 决定：B：加（推荐项；无原话，仅供参考）。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：出货会触发买家邮件，多一步减少填错；不新增写入权限。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q20；manual-fulfilment-v1
- 实现卡建议：OC-13（apps/admin/components/OrderShipment.tsx）

### Q21 订单状态用词（ORD-01）
- 决定：A：统一「已出貨」「待付款」。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：同一状态不应有两种说法；状态码与筛选不变；待付款不读成已确认收款。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q21；merchant-orders-v2
- 实现卡建议：OC-10（apps/admin/lib/orders-copy.ts）

### Q22 订阅页标题（CP-06）
- 决定：A：保持「帳單」。依据：owner「按你的结论来」，即 ADR-CHOICES 推荐项。
- 理由与放弃项：原话未提；O5 只管买家。
- 契约：`contracts/owner-choices-20261026-v1.md` OC-Q22；customers-billing-v1
- 实现卡建议：无实现卡

## 三、已决定、本次不改
LOG-01、LOG-02、LOG-03、CAT-01、CAT-03、CP-02、CAT-04：现行 owner 原话 O1–O9 或契约已定。AD-07、AD-13：标「未納入產品」，**只读保留**，不删除。

## 四、未决 UNKNOWN
1. HK 平台账户对台湾买家开放 Link 需要 SANDBOX 实测（SP16）；之前 LIVE 保持 `[card]`。
2. Q2 与 SA-0 §6.7–6.9 冲突：需要 owner 一句话确认；SA-2 暂停。
3. Q5 「到貨需付」白话文案需 owner 看一次。
4. Q12 「未读」定义，Q20 核对页文案，均在各卡内先定。
5. 设置页对已在 allowlist 的店是否保留开关（set_platform_stripe）：本次只规定不出现申请/配置，开关去留沿 AD-PF2.1。

## 五、任务
OC-01 … OC-14 见 `contracts/tasks.json`（NOT_STARTED，迁移号 02xx 由 integrator 分配，write_paths 互不重叠；路径是草案，开工时以实际目录为准）。
