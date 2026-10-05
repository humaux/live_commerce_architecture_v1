# Unit index — W3 余下 / W4 / W5 / W6 / OPS（DRAFT 2026-10-06，base `r3/integration` `bb71f966`）

来源：`output/arch-conformance/IMPLEMENTATION-PLAN.md`（含「Integrator 裁决」）+ `deviations-and-deferrals.md` + owner 暂定裁决（各 brief「Integrator 裁决」节）。
迁移号均为**占位**，由 integrator 按合并顺序最终分配（0127–0129 预留给在途 LC-B4/B6/B7 等）。并行上限：同时 ≤ 2 个写入单元、≤ 4 个 agent（PROCESS）。
角色：DeepSeek = 后端；Codex = `apps/` UI；K3 = 独立门禁；Claude = 钱路/安全终审。

| 单元 | brief | 角色 | 依赖（须已合并） | 迁移 | 可并行（写入路径不重叠且无共享函数） |
|---|---|---|---|---|---|
| W3-02B 拣货单/导出/超商批量 | `w3-02b-picklist-export.md` | DeepSeek | W3-01B（0126） | 0130 | W3-03B、W3-04B、W3-05B、W4-*、W6-01B、W6-02B、OPS-01B |
| W3-03B 结账提醒（24h 内） | `w3-03b-checkout-reminder.md` | DeepSeek | LC-B4、LC-B5（0121） | 0131 | W3-02B、W3-04B¹、W3-05B、W3-07B、W4-*、W6-* |
| W3-04B 没货回复 | `w3-04b-sold-out-reply.md` | DeepSeek | claim-direct-checkout 后端（0116）、LC-B4、LC-B5 | 0132 | W3-02B、W3-03B¹、W4-*、W6-* |
| W3-05B 限制下单名单 | `w3-05b-blocklist.md` | DeepSeek | W3-04B（同改 `plan_claim_reply`） | 0133 | W3-02B、W3-03B、W3-07B、W4-*、W6-* |
| W3-07B 合并出货（只合包裹） | `w3-07b-parcel-merge.md` | DeepSeek | W3-01B、W3-02B | 0134 | W3-03B、W3-04B、W3-05B、W4-*、W6-*、OPS-01B |
| W3-08B 最小 RMA + 商家取消 | `w3-08b-returns.md` | DeepSeek | W3-07B（取消读组表） | 0135 | W3-03B–05B、W4-01B、W4-02B、W6-*、OPS-01B |
| W3-U1b 拣货/导出/超商 UI | `w3-u1b-picklist-ui.md` | Codex | W3-02B、W3-U1（`unit/w3-01b-ui`） | — | W3-U3、W3-U5、W4-U1、W6-U1 |
| W3-U2 直播設定（提醒/没货/名单） | `w3-u2-live-settings-ui.md` | Codex | W3-03B、W3-04B、W3-05B、LC-U2 | — | W3-U1b、W3-U4、W3-U5、W4-U1、W6-U1（不与 W3-U3：同改 `CommentStream.tsx`） |
| W3-U3 留言标签打印（纯 UI） | `w3-u3-comment-label-print.md` | Codex | LC-U2 | — | W3-U1b、W3-U4、W3-U5、W4-U1、W6-U1（不与 W3-U2） |
| W3-U4 合并出货 UI | `w3-u4-parcel-merge-ui.md` | Codex | W3-07B、W3-U1b（同改 `MerchantOrders.tsx`） | — | W3-U2、W3-U3、W3-U5、W4-U1、W6-U1 |
| W3-U5 退货/取消 UI | `w3-u5-returns-ui.md` | Codex | W3-08B | — | W3-U1b–U4、W6-U1（W4-U1 改 `OrderRefunds.tsx`，本单元不改，可并行） |
| W4-01B PAYUNi NotifyURL 接收器 | `w4-01b-payuni-notify.md` | DeepSeek | — | 0136 | W3-*、W6-*、OPS-01B |
| W4-02B PAYUNi 自助开通 | `w4-02b-payuni-activation.md` | DeepSeek | W4-01B | 0137 | W4-03B²、W3-*、W6-* |
| W4-03B PAYUNi 信用卡退款 | `w4-03b-payuni-refund.md` | DeepSeek | W4-01B | 0138 | W4-02B²、W3-02B–07B、W6-* |
| W4-U1 PAYUNi 开通 UI | `w4-u1-payment-activation-ui.md` | Codex | W4-02B、W4-03B | — | 全部其他 UI |
| W6-01B 客户标签与备注 | `w6-01b-customer-tags-notes.md` | DeepSeek | — | 0139 | W3-*、W4-*、W6-02B、OPS-01B（**先于 W5-02B**） |
| W5-02B 顾客导入（含最小批次表） | `w5-02b-customer-import.md` | DeepSeek | W6-01B（同改 `read_merchant_customers`） | 0140 | W3-*、W4-*、W6-02B、OPS-01B |
| W5-03B 历史订单导入（只读档） | `w5-03b-order-history-import.md` | DeepSeek | W5-02B | 0141 | W3-*、W4-*、W6-02B、OPS-01B |
| W6-02B 商品/渠道/漏斗/手工单报表 | `w6-02b-reports.md` | DeepSeek | LC-B6（仅第④项） | 0142 | 全部后端单元（只读定义者） |
| W6-U1 客户标签/备注 + 报表 UI | `w6-u1-customers-reports-ui.md` | Codex | W6-01B、W6-02B | — | W3-U*、W4-U1 |
| W5-U1 导入向导 UI | `w5-u1-import-wizard-ui.md` | Codex | W5-02B、W5-03B、W6-U1（同改 `CustomerDetail.tsx`） | — | W3-U*、W4-U1 |
| OPS-01B 停用/恢复 + 运营审计 CLI | `ops-01b-store-suspend.md` | DeepSeek | —（守卫若改 `plan_claim_reply` 则在 W3-05B 后） | 0143 | W3-02B、W3-07B、W3-08B、W4-*、W5-*、W6-* |
| OPS-02B 支持授权 CLI（限时只读） | `ops-02b-support-grant.md` | DeepSeek | OPS-01B | 0144 | 不改 `resolve_access` 的任何单元 |

¹ W3-03B 与 W3-04B 都扩展 0121 的 `template_id` CHECK：可并行开发，integrator 合并时串行改写该 CHECK。
² W4-02B 与 W4-03B 并行开发，但**同批上线**：`LC_PAYUNI_ENABLED` 在两者合并且 `PAYUNI-SANDBOX` 门禁过之前保持 0（偏差 B23）。

## 建议派发顺序（每批 ≤ 2 个写入单元）
1. W3-02B + W4-01B → 2. W3-03B + W3-04B（LC-B4 合并后）→ 3. W3-05B + W6-01B → 4. W3-07B + W4-02B → 5. W3-08B + W4-03B →
6. W5-02B + W6-02B → 7. W5-03B + OPS-01B → 8. OPS-02B；UI 单元随对应后端合并穿插（Codex 槽位独立计数，仍受 4 agent 上限）。

## 计划里有、本批不出 brief 的单元（及原因）
| 计划单元 | 处理 |
|---|---|
| W3-06B 关键字工具/模拟器、W3-U2 模拟器部分 | 未列入本批；店级默认匹配模式本轮不做（裁决 10） |
| W4-02B（原）Stripe 商家自助绑定 | **取消**：Stripe 只给 owner 香港店，运维登记，保留逐店审批（D3）；ID 改给 PAYUNi 开通 |
| W4-03B（原）每日对账 | 延后（需 PSP 结算报表 SANDBOX）；ID 改给 PAYUNi 退款 |
| W5-01B 通用导入框架 | 并入 W5-02B 的两张表；商品 CSV 不迁移（YAGNI） |
| W5-04B 并行期发送总开关 | 未列入；若试点与 SHOPLINE 双开，需前移（见 owner 问题） |
| W6-03B 利润、W6-04B 角色包、W6-05B 失败台账、W6-06B 广告解绑、W6-U2 | 未列入；利润需 owner 解除 M18 #8 |
| OPS-01 SBOM | 未列入；本批 OPS = 运营 CLI |

## owner 暂定裁决的落点（无单独单元者）
- 计费按店、价格未定、`LC_BILLING_ENABLED` 保持关：无单元。
- 会员/积分/分润：首版范围外；客户标签+备注先做（W6-01B）。
- 预售 = 不追踪库存 SKU + 预计出货日：无单元（若需结构化「预计出货日」字段 → 另开 S 单元，见 owner 问题）。
- 一单一包裹：W3-07B 合包只合包裹；W3-08B 不做多包裹；架构 §2.1 措辞由 integrator 改。
- 手工单可刷卡（PAYUNi）：W4-02B 启用后 LC-B6 自动可用，无单独单元。
- M-7 撤销：W3-01B 已合并（0126）。v5 示意页：W3-U* 界面目标。
