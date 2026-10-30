# PARITY-MONEY implementation delivery · CI pending

Mode: MOCK/SANDBOX only. Author acceptance E0. Static E1 only. Current product screenshots, GREEN, independent E4 and production acceptance NOT_RUN. The Figma PNGs below are design references, never proof of the application.

## Authority and implementation

Owner's 2026-10-30 dispatched card; Figma `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`; inventory §3 C2 + Appendix `U:finance`, `U:billing`, `U:cod`. All 48 frame roots were re-read with Fleet Bridge `get_design_context` + `get_screenshot` in relay 1. No Figma writes. Reference hashes: `reference-manifest.json`.

The requested `ux-finance`/`ux-billing`/`ux-cod` DELIVERY paths and `output/ux-audit-ref/FIGMA-GUIDE.md` were absent at read time; no older delivery was substituted for the live file. Existing approved admin shell is reused. The legacy report's four data views and permissions remain; date/failure states now follow the signed frames.

Finance overview, daily details, date editor, CSV preview and basis explanation use the original finance reads and audited download. URL `view` preserves read-only screens; applied dates remain server route parameters. Settlement selector uses the compact five-column layout; the separate detail retains every signed contribution, returned payout history and returned-in contribution. Selected statement survives refresh. Billing separates standing, subscription, payment/invoices and usage; Stripe opens only after the explicit explanation/confirmation. Card payment controls remain behind one Advanced disclosure, with original CAS and terms. COD saves via the original guarded PUT and shows a review of the successful GET's four values.

Additive field: `FinanceSummary.generated_at` = successful server observation UTC time, not the ledger's last mutation. No money calculation is added to the client. JSON strict parsing requires a valid timestamp; CSV stays 13 columns. Existing parser fixtures add exactly this field; exact-key HTTP oracle adds only the field. No migration or R2 pin change.

## Evidence and gate

Causal RED: PR370 tests-only head `09d352dd97b1660b18b7bb7b07d0fca994997f19`; Gates run `41055239291`, job `124069270267` (shard:g09), execution merge SHA `005ab56f42ed0d81529f18b39f3863bde160c585`. Original log at `2026-10-30T12:49:51.1175995Z`: `customers_billing_http_test.go:448: PARITY-MONEY: finance summary is missing server-generated generated_at for the signed overview`. Existing CB09 subtests passed. Full run remains pending for other shards; a single failed assertion is not GREEN. `red.log` contains the filtered raw receipt. Foundation run `41055239182`, job `124060995570` is structural success only.

Local permitted static checks: Go build reporting; gofmt; full admin tsc using a temporary static tsconfig with local package aliases and read-only existing dependency references; node --check changed browser/harness files; ownership generation and --check; depmap regeneration; packet structure check; git diff --check. No local test, browser, server, PG, container or provider action was run. Final source hashes: `source-hashes.json`. The packet check's unrelated timestamp output was restored.

Acceptance: this PR's GitHub CI, followed by independent comparison of all real-route parity PNGs against these references and click receipts. No assertion, threshold, fixture amount, baseline or exemption was relaxed. All original test cases remain. Newly signed assertions remain unchanged from RED.

## Desktop → mobile → summary facts

| Screen | Required facts/controls | Mobile/review preservation | Evidence status |
|---|---|---|---|
| Finance overview | Period, server observation time, currency/environment, card count/received/refunded/net; separate pickup/COD/bank transfer | Same server fields per group; no cross-group sum; CSV preview repeats applied dates | Static wiring E1; browser comparison pending |
| Finance daily | Taipei day, environment/currency, count, card/refund/net, each other collection amount/count | Responsive table and separate daily other-payments section; return preserves period | E1; browser pending |
| Finance dates/CSV/basis/failure | Draft from/to, 92-day cap, cancel/apply, audited download, explanation and retry | Native labeled inputs; no invalid request; applied dates survive refresh | Browser assertions pending |
| Settlement list/detail | Currency, period, state/net (display statement number deferred); seven signed contributions, closed time, transaction dates/amounts/fees, original payout history, return time | Same detail component on both widths; returned-in and carried-in have distinct rows; refresh retains statement | E1; browser pending |
| Billing | Standing, plan/amount/currency/interval, subscription state/period/cancellation, usage counts/period, payment/portal explanation | Desktop two stacks become the same ordered cards on mobile; portal confirmation repeats explanation | E1; browser pending; exact next charge time deferred below |
| Card collection | Availability from response, collection steps, settlement distinction, weekly schedule/fee pending; actual limits/state/terms/CAS in Advanced | Same fields and actions, responsive flow stack; no credentials requested | E1; browser pending |
| COD | Enabled, cap, surcharge, carrier; note applying only to new orders | All four server GET facts shown after successful save, labels and formatted values repeated in review; reload re-reads them | Browser click/persistence pending |

## Data gaps / deferred exact facts

Billing frames show an exact next charge date/next attempt. Existing `BillingStatus` exposes current period bounds and cancellation, but no `next_charge_at` / next invoice attempt. Do not relabel a period end as a guaranteed charge. That exact field is DEFERRED on the affected good/overdue/restricted billing states; period/cancellation/standing remain live. No buyer PII or monetary calculation was added. Settlement frames show human-readable statement numbers (`ST-…`); the response supplies only opaque `statement_id`, no `statement_number`. That display field is DEFERRED; the selector uses actual currency/period, never fabricates a statement number or shortens a UUID into one. Whole-payout returns use the historical `payout_minor`, as the contract explicitly excludes partial returns. No broader blocker is inferred.

COD mount: existing `SettingsWizard.tsx` already imports and mounts `CodSettings`; no new mount is required. If the concurrent settings lane removes it, integrator mount is `<CodSettings store={store.id} locale={locale} />` in the existing merchant-arranged delivery branch. This card did not edit SettingsWizard or components/settings.

## BOURDIEU · Q1–Q6

Framework source: owner charter recalled in Humaux `ac930b2b-3a69-4d25-b620-b9e679438538`; inference only, no user research claimed. Novice/expert means recent experience with this particular task.

| Question | Verdict / concrete application |
|---|---|
| Q1 terminology threshold | Explain collection versus settlement; show signed card/offline money separately; internal formula and prototype controls stay off the screen. |
| Q2 experienced merchant habits | Preserve date ranges, selected statement and daily view across reload; keep native dates and actual CSV export. |
| Q3 fees and timing | Display server amounts; distinguish payment received from settlement; disclose external Stripe transition; do not invent a rate or charge date. |
| Q4 pickup terminology | This module records pickup money separately and does not require store-code knowledge; COD is explicitly carrier-collected home delivery. |
| Q5 mobile equivalence | Same finance facts, seven settlement contributions, payout history, billing usage and four COD values; every signed pair has a planned real-route screenshot. |
| Q6 aesthetics separate | Owner aesthetic judgement UNKNOWN; no task-success or usability claim until click CI/independent review. |

## PROJECT UNDERSTANDING

Read architecture §§0/1/2/26/27: evidence modes separate; Go/PG own transaction facts; server-authorized scope; own worktree; independent CI acceptance. `customers-billing-v1.md` finance HTTP/row/CSV contract: per currency/environment and 0..91-day difference; audited export. Existing billing DTO and client: Stripe URLs used once without storage; standing from server/webhook. Existing card summary/CAS path: no merchant keys or account secrets. Settlement model's signs/return precedence: no client recomputation, original payout history retained. COD body/client: version CAS and same-request idempotency on uncertainty. Current source, not old output snapshots, defined reachable states.

## MOBBIN → EAGLE

Read-only Eagle folder API returned folder `MVQL9YYT785NK` (“08 财务与账单”) and eight nondeleted entries; metadata receipt `eagle-finance-items.json`. Existing imported structural annotations reviewed, not used as UI authority: Canny `MVRS74509J5BE` separates plan/usage/billing-cycle cards; Etsy `MVRS73PTA3SYF` separates period summary from transaction ledger/CSV; Deliveroo mobile `MVOVHEAYW0NR9` groups subscription and payment method. Those principles match the signed compositions; no images, branding, amounts, trial promises or external fee policies were copied. No fresh Mobbin import or image-level reference review is claimed. Missing COD-specific pattern: integrator Mobbin query “cash on delivery settings amount cap carrier review mobile”; signed COD frame remains authority.

## FIGMA OFFICIAL GUIDANCE

The card's guide path was absent; direct current first-party guidance read 2026-10-30: [Structure your Figma file for better code](https://developers.figma.com/docs/figma-mcp-server/structure-figma-file/) applied through existing Badge/DateControl/TableFrame components, shared `--ui-*` tokens and responsive CSS flow rather than absolute positioning. [Connect your prototype](https://help.figma.com/hc/en-us/articles/360040315773-Connect-your-prototype) informed explicit date/CSV/detail/portal states with return controls. No Figma prototype wiring was modified.

Figma copy lint: NOT_RUN; no new/changed Figma frames. Provided density-probe.json targets unrelated module v2 frames and uses `use_figma`, which this read-only card prohibits; no false density PASS is claimed. Real-page mobile density/overflow awaits CI/reviewer; moneyShot reads geometry without mutating the DOM.

## Legacy test mapping · original cases retained

| Old entry/assertion | Signed equivalent / preserved intent |
|---|---|
| Finance always-open from/to/Show | Open finance date editor, same native inputs, same reversed/91-day/92-day guards, Apply then Daily details |
| Finance table on landing | Daily details table + separate other collection records; captured/net zero and exact server CSV remain asserted |
| Finance direct CSV | Export preview then the real native download; OC re-auth still asserts one refused request and exactly one replay |
| 11-column settlement list / internal formula text | Compact selector then real click to breakdown; all seven signed values and independently checked conservation move to detail |
| Billing direct portal | Explanation then the original hosted portal request; keyless body/URL privacy checks remain |
| Existing subscription buy button disabled | Choice is absent for a live subscription, with Manage visible; original no-second-checkout intent preserved |
| Card management immediately visible | One Advanced disclosure, then original terms/limits/CAS/block/disable checks |
| Legacy Finance/Billing/Card/COD headings | Signed literal copy, with original headings asserted absent in all three locales |

## New copy · three locales

Inherited report graphs, permission/errors and server status captions retain the existing three-locale catalogs (`reports-copy`, `customers-copy`, `billing-copy`, `card-payments-copy`, `card-payments-settlements-copy`, `cod-copy`). All newly introduced visible strings are in the typed `moneyCopy` namespace below.

| zh-TW | zh-CN | en |
|---|---|---|
| 進階 | 进阶 | Advanced |
| 關帳時間 | 关账时间 | Closed at |
| 撥付金額 | 拨付金额 | Payout amount |
| 退回金額 | 退回金额 | Returned amount |
| 財務總覽 | 财务总览 | Finance overview |
| 其他收款 · 分開核對 | 其他收款 · 分开核对 | Other payments · Reconcile separately |
| 信用卡收款筆數 | 信用卡收款笔数 | Card payment count |
| 此口徑淨額 | 此口径净额 | Net on this basis |
| 每日明細 | 每日明细 | Daily details |
| 財務每日明細 | 财务每日明细 | Daily finance details |
| 結算單 | 结算单 | Settlement statements |
| 了解財務口徑 | 了解财务口径 | Understand finance figures |
| 收款與結算口徑 | 收款与结算口径 | Payments and settlements |
| 財務日期範圍 | 财务日期范围 | Finance date range |
| 選擇日期範圍 | 选择日期范围 | Choose date range |
| 開始日期 | 开始日期 | Start date |
| 結束日期 | 结束日期 | End date |
| 套用日期 | 应用日期 | Apply dates |
| 取消 | 取消 | Cancel |
| 匯出財務 CSV | 导出财务 CSV | Export finance CSV |
| 準備財務 CSV | 准备财务 CSV | Prepare finance CSV |
| 下載 CSV | 下载 CSV | Download CSV |
| 返回財務總覽 | 返回财务总览 | Back to finance overview |
| 財務讀取 | 财务读取 | Finance loading |
| 讀取失敗 | 读取失败 | Unable to load |
| 重新讀取 | 重新读取 | Reload |
| 報表讀取 | 报表读取 | Report loading |
| 結算單明細 | 结算单明细 | Settlement statement details |
| 本期結算組成 | 本期结算组成 | This settlement's breakdown |
| 查看此結算單 | 查看此结算单 | View this statement |
| 返回結算單列表 | 返回结算单列表 | Back to settlement statements |
| 結算單明細 · 撥付已退回 | 结算单明细 · 拨付已退回 | Settlement statement details · Payout returned |
| 撥付已退回 · 非已結清 | 拨付已退回 · 非已结清 | Payout returned · Not settled |
| 結算單明細 · 退回款帶回 | 结算单明细 · 退回款带回 | Settlement statement details · Returned funds carried in |
| 撥付已退回帶回 | 拨付已退回带回 | Returned payout carried in |
| 收款設定 | 收款设置 | Payment settings |
| 平台統一收款 | 平台统一收款 | Platform payment collection |
| 付款成功，不等於已結算 | 付款成功，不等于已结算 | Payment received does not mean settled |
| 平台統一收款，已可使用。你不需要申請或填寫任何收款資料。 | 平台统一收款，已可使用。你不需要申请或填写任何收款资料。 | Platform payments are ready to use. You do not need to apply or enter payment account details. |
| 查看結算與對帳 | 查看结算与对账 | View settlements and reconciliation |
| 方案與帳單 | 方案与账单 | Plans and billing |
| 帳戶狀態 | 账户状态 | Account status |
| 付款方式與發票 | 付款方式与发票 | Payment method and invoices |
| 本期用量 | 本期用量 | Usage this period |
| 管理付款與發票 | 管理付款与发票 | Manage payment and invoices |
| 訂閱需要處理 | 订阅需要处理 | Subscription needs attention |
| 回到方案與帳單 | 回到方案与账单 | Back to plans and billing |
| 開啟 Stripe 託管頁面，可更換信用卡、查看發票與取消訂閱。按下後會離開本站，前往 Stripe。 | 打开 Stripe 托管页面，可更换信用卡、查看发票与取消订阅。点击后将离开本站，前往 Stripe。 | Open Stripe's hosted page to change your card, view invoices or cancel your subscription. This takes you away from this site to Stripe. |
| 未訂閱 | 未订阅 | No subscription |
| 狀態良好 | 状态良好 | In good standing |
| 付款逾期 | 付款逾期 | Payment overdue |
| 已受限 | 已受限 | Restricted |
| 週期結束後終止 | 周期结束后终止 | Ends at the end of the period |
| 已收到付款，正在更新訂閱… | 已收到付款，正在更新订阅… | Payment received. Updating your subscription… |
| 已取消結帳，未扣款。 | 已取消结账，未扣款。 | Checkout was cancelled. Nothing was charged. |
| 貨到付款設定 | 货到付款设置 | Cash on delivery settings |
| 貨到付款儲存檢查 | 货到付款保存检查 | Review cash on delivery settings |
| 在結帳時提供貨到付款 | 在结账时提供货到付款 | Offer cash on delivery at checkout |
| 單筆金額上限（TWD） | 单笔金额上限（TWD） | Per-order amount cap (TWD) |
| 手續費（TWD） | 手续费（TWD） | Collection fee (TWD) |
| 貨運業者 | 货运公司 | Carrier |
| 儲存設定 | 保存设置 | Save settings |
| 返回貨到付款設定 | 返回货到付款设置 | Back to cash on delivery settings |
| 買家付款 → 平台代收 → 每週結算 → 撥付給商家 | 买家付款 → 平台代收 → 每周结算 → 拨付给商家 | Buyer pays → Platform collects → Weekly settlement → Payout to your store |
| 更新於 | 更新于 | Updated |
| 正式環境 | 正式环境 | Live payments |
| 測試環境（不是真實收款） | 测试环境（不是真实收款） | Test payments · No real collection |
| 信用卡已收款 | 信用卡已收款 | Card payments received |
| 成功退款 | 成功退款 | Successful refunds |
| 收款與退款按各自發生的台北日期計入；淨額可為負。不是利潤，也不是結算已撥付。「口徑」指這個數字的計算方式。 | 收款与退款分别按发生的台北日期统计；净额可以为负。它不是利润，也不代表已结算拨付。“口径”指数字的计算方式。 | Collections and refunds use their own Taipei event dates. Net may be negative. It is neither profit nor an amount already paid out. |
| 各自是獨立收款途徑，不併入信用卡已收款與淨額；未收款不列入。 | 各项是独立收款方式，不计入信用卡已收款与净额；未收款不统计。 | These are separate payment paths, excluded from card collections and net. Uncollected payments are excluded. |
| 不同幣種與正式／測試環境分組顯示，不相加。 | 不同币种与正式、测试环境分别展示，不合计。 | Currencies and live/test environments are shown separately and never added together. |
| 門市取貨已收款 | 门店取货已收款 | Collected at pickup |
| 貨到付款已收款 | 货到付款已收款 | Collected on delivery |
| 銀行轉帳已確認 | 银行转账已确认 | Bank transfers confirmed |
| 此日期範圍沒有財務資料 | 此日期范围没有财务数据 | No finance data for these dates |
| 調整日期後再查詢；這不是沿用上一個期間的金額。 | 调整日期后重新查询，不会沿用上一期间的金额。 | Choose another date range. Amounts from a previous period are not reused. |
| 信用卡每日記錄 | 信用卡每日记录 | Daily card records |
| 其他收款每日記錄（不併入信用卡） | 其他收款每日记录（不计入信用卡） | Other daily collections · Excluded from card payments |
| 日期 | 日期 | Date |
| 環境 | 环境 | Environment |
| 筆數 | 笔数 | Count |
| 幣種 | 币种 | Currency |
| 金額 | 金额 | Amount |
| 總計 | 合计 | Total |
| 這裡是信用卡收款事實筆數，不額外保證為唯一訂單數。退款在退款成功日計入，與訂單建立日不同。 | 这里统计信用卡收款记录笔数，不保证是去重订单数。退款在成功退款日统计，与订单创建日不同。 | Counts are card collection facts, not necessarily unique orders. Refunds use their successful refund date, not the order creation date. |
| 套用後才改變報表；取消保留原期間。最多92個台北日。 | 应用后才更新报表；取消保留原期间。最多92个台北日。 | Apply changes the report; Cancel keeps the original dates. Up to 92 Taipei calendar days. |
| 確認日期後，系統會先記錄匯出稽核，再傳回 CSV。 | 确认日期后，系统先记录导出审计，再返回 CSV。 | Confirm the dates. The server records the export audit before returning the CSV. |
| 每日日期、幣種、環境與各收款途徑的筆數／最小貨幣單位金額，共13欄；不包含總計行、買家姓名、電話或地址。 | 每日日期、币种、环境、各收款方式笔数与最小货币单位整数金额，共13列；不包含合计行、买家姓名、电话或地址。 | 13 columns of daily dates, currency, environment, counts and integer minor-unit amounts. No totals row or buyer name, phone or address. |
| 匯出需要額外權限；不能匯出時，請洽具有匯出權限的團隊成員。 | 导出需要额外权限；无法导出时，请联系有导出权限的团队成员。 | Export requires additional permission. Ask a team member with export access if you cannot download. |
| 目前無法取得財務資料；沒有產生新的收款、退款或撥付。 | 目前无法获取财务数据；没有产生新收款、退款或拨付。 | Finance data is unavailable. No new collection, refund or payout has been made. |
| 目前無法讀取報表。稍後重新讀取不會發布廣告或重新收款。 | 目前无法读取报表。稍后重试不会发布广告或重新收款。 | The report is unavailable. Reloading does not publish ads or collect payments again. |
| 每日財務 | 每日财务 | Daily finance |
| 每週結算（台北時間週一至週一），期末滿72小時才可關帳（定案、不再變動）。買家付款成功不等於已結算，需看結算單與撥付紀錄。費用以結算單已記錄金額為準；負額結轉至下一期。 | 每周结算（台北时间周一至周一），期末满72小时才可关账定案。付款成功不等于已结算，请查看结算单与拨付记录。费用按结算单记录展示；负额结转到下期。 | Weekly Taipei settlements. A period can close after 72 hours. Check the statement and payout history to confirm settlement. Fees use recorded statement amounts; a negative balance carries to the next period. |
| 廣告與直播歸因 | 广告与直播归因 | Ads and live attribution |
| 這是另一個報表，可能用不同日期隊列和Meta帳戶時區；不能把Meta回報購買額或廣告花費當成收款／結算差額。 | 归因报表可能使用不同日期队列和Meta账户时区；不能把Meta购买金额或广告支出当成收款、结算差额。 | Attribution reports may use different date cohorts and Meta account time zones. Meta purchase values and ad spend are not collection or settlement differences. |
| 不要用不明差額調平 | 先核对实际记录 | Reconcile the actual records |
| 請先對齊幣種、環境及期間，再分別核對實際記錄。本頁不執行帳務調整。 | 先对齐币种、环境及期间，再分别核对各来源记录。本页不做账务调整。 | Align currency, environment and period before comparing sources. This page makes no accounting adjustments. |
| 台北週期（期末不含） | 台北周期（期末不含） | Taipei period · End excluded |
| 每週結算 · 依結算單已記錄金額與費用列示。買家付款成功不等於已結算。若撥付被退回，退回款會計入之後的結算單。 | 每周结算 · 金额及费用按结算单记录展示。付款成功不等于已结算。拨付被退回后，退回款计入之后的结算单。 | Weekly settlements · Amounts and fees are taken from the statement. Payment received does not mean paid out. Returned payouts are included in a later statement. |
| 本店的平台訂閱、付款方式與用量。 | 本店的平台订阅、付款方式与用量。 | Your store’s platform subscription, payment method and usage. |
| 卡片資料只在 Stripe 的頁面輸入與保存，本站不會要求你輸入卡號。發票也只在 Stripe 頁面查看，本站沒有發票列表。 | 卡片资料仅在 Stripe 页面输入和保存，本站不要求填写卡号。发票也在 Stripe 页面查看，本站没有发票列表。 | Enter and save card details only on Stripe. Invoices are available on Stripe; this site does not keep an invoice list. |
| 尚無帳單帳戶，請先訂閱方案。 | 尚无账单账户，请先订阅方案。 | Subscribe to a plan before managing payment details. |
| 訂閱 | 订阅 | Subscription |
| 方案 | 方案 | Plan |
| 狀態 | 状态 | Status |
| 目前週期（台北時間） | 当前周期（台北时间） | Current period · Taipei time |
| 續訂 | 续订 | Renewal |
| 訂閱終止後無法開啟新的登記窗口；訂單、退款、出貨與隱私操作不受影響。 | 订阅终止后无法开启新的登记窗口；订单、退款、发货与隐私操作不受影响。 | New claim windows stop after the subscription ends. Orders, refunds, shipping and privacy actions remain available. |
| 1  買家付款 | 1  买家付款 | 1  Buyer pays |
| 買家在結帳頁付款 | 买家在结账页付款 | Payment at checkout |
| 2  平台收款 | 2  平台收款 | 2  Platform collects |
| 款項先進平台帳戶 | 款项先进入平台账户 | Funds enter the platform account |
| 3  結算給你 | 3  结算给你 | 3  Settlement to your store |
| 依結算週期撥付 | 按结算周期拨付 | Paid out on the settlement schedule |
| 付款成功代表平台已收到款項。款項是否已結算給你，請看財務頁的結算狀態。退款與爭議款會從結算中扣除。 | 付款成功表示平台已收到款项。是否已结算给你，请查看财务页的结算状态。退款与争议款从结算中扣除。 | Payment success means the platform received the payment. Check finance for settlement and payout status. Refunds and disputes are deducted from settlements. |
| 每週結算 | 每周结算 | Weekly settlement |
| 結算週期與費用 | 结算周期与费用 | Settlement schedule and fees |
| 平台費用 | 平台费用 | Platform fee |
| 待定 | 待定 | To be confirmed |
| 平台費用比例尚未公布，公布前這裡不顯示任何數字。 | 平台费率尚未公布，确认前不显示任何数字。 | The platform fee rate has not been announced. No rate is shown until it is confirmed. |
| 已可使用 | 已可使用 | Ready to use |
| 包裹送達時由貨運公司代收。僅限宅配；本站不經手貨到付款款項。 | 包裹送达时由货运公司代收。仅限宅配；本站不经手货到付款款项。 | The carrier collects payment when the parcel arrives. Home delivery only; this site does not handle the funds. |
| 按元輸入，整數 NT$1–20,000，含手續費。 | 按元输入，整数 NT$1–20,000，含手续费。 | Whole TWD 1–20,000, including the collection fee. |
| 整數 NT$0–1,000，加到代收款項中。 | 整数 NT$0–1,000，加到代收款项中。 | Whole TWD 0–1,000, added to the collection amount. |
| 變更只影響新訂單。舊訂單沿用下單時的金額與貨運業者。 | 变更只影响新订单。旧订单保留下单时的金额和货运公司。 | Changes apply only to new orders. Existing orders keep their original amounts and carrier. |
| 已啟用 | 已启用 | Enabled |
| 未啟用 | 未启用 | Disabled |
| 黑貓 | 黑猫 | Black Cat |
| 新竹 | 新竹 | Hsinchu |
| 設定已儲存。以下為重新讀取後的後台資料。 | 设置已保存。以下为重新读取后的后台数据。 | Settings saved. These are the values read back from the server. |

## Parity sheet · planned CI screenshots, not current acceptance

All real routes use 1440×900 desktop / 390×844 mobile. en/zh-CN images use matching locale subdirectories. Existing frame-only notices are excluded from product; all reachable data states use original responses.

| Signed frame | Real-route browser artifact | Signed reference PNG |
|---|---|---|
| `4:12097` · Admin Desktop · 財務總覽 · DaWan Live | `parity/4-12097.png` | [4:12097](reference/4-12097.png) |
| `4:12246` · Admin Mobile · 財務總覽 · DaWan Live | `parity/4-12246.png` | [4:12246](reference/4-12246.png) |
| `6:18572` · Admin Desktop · 結算單明細 · DaWan Live | `parity/6-18572.png` | [6:18572](reference/6-18572.png) |
| `6:18737` · Admin Mobile · 結算單明細 · DaWan Live | `parity/6-18737.png` | [6:18737](reference/6-18737.png) |
| `6:18814` · Admin Desktop · 收款與結算口徑 · DaWan Live | `parity/6-18814.png` | [6:18814](reference/6-18814.png) |
| `6:18963` · Admin Mobile · 收款與結算口徑 · DaWan Live | `parity/6-18963.png` | [6:18963](reference/6-18963.png) |
| `6:20884` · Admin Desktop · 收款設定 · SHOPLINE | `parity/6-20884.png` | [6:20884](reference/6-20884.png) |
| `6:20993` · Admin Mobile · 收款設定 · SHOPLINE | `parity/6-20993.png` | [6:20993](reference/6-20993.png) |
| `1151:9432` · Admin Desktop · 報表日期範圍 · DaWan Live | `parity/1151-9432.png` | [1151:9432](reference/1151-9432.png) |
| `1151:9433` · Admin Mobile · 報表日期範圍 · DaWan Live | `parity/1151-9433.png` | [1151:9433](reference/1151-9433.png) |
| `1151:9434` · Admin Desktop · 報表讀取回饋 · DaWan Live | `parity/1151-9434.png` | [1151:9434](reference/1151-9434.png) |
| `1151:9435` · Admin Mobile · 報表讀取回饋 · DaWan Live | `parity/1151-9435.png` | [1151:9435](reference/1151-9435.png) |
| `1173:10738` · Admin Desktop · 財務每日明細 · DaWan Live | `parity/1173-10738.png` | [1173:10738](reference/1173-10738.png) |
| `1173:10739` · Admin Mobile · 財務每日明細 · DaWan Live | `parity/1173-10739.png` | [1173:10739](reference/1173-10739.png) |
| `1173:10740` · Admin Desktop · 結算單列表 · DaWan Live | `parity/1173-10740.png` | [1173:10740](reference/1173-10740.png) |
| `1173:10741` · Admin Mobile · 結算單列表 · DaWan Live | `parity/1173-10741.png` | [1173:10741](reference/1173-10741.png) |
| `1173:10742` · Admin Desktop · 財務日期 · DaWan Live | `parity/1173-10742.png` | [1173:10742](reference/1173-10742.png) |
| `1173:10743` · Admin Mobile · 財務日期 · DaWan Live | `parity/1173-10743.png` | [1173:10743](reference/1173-10743.png) |
| `1173:10744` · Admin Desktop · 財務CSV · DaWan Live | `parity/1173-10744.png` | [1173:10744](reference/1173-10744.png) |
| `1173:10745` · Admin Mobile · 財務CSV · DaWan Live | `parity/1173-10745.png` | [1173:10745](reference/1173-10745.png) |
| `1173:10746` · Admin Desktop · 財務讀取狀態 · DaWan Live | `parity/1173-10746.png` | [1173:10746](reference/1173-10746.png) |
| `1173:10747` · Admin Mobile · 財務讀取狀態 · DaWan Live | `parity/1173-10747.png` | [1173:10747](reference/1173-10747.png) |
| `1248:8745` · Admin Desktop · 結算單 · 撥付退回 · DESIGN | `parity/1248-8745.png` | [1248:8745](reference/1248-8745.png) |
| `1248:8823` · Admin Mobile · 結算單 · 撥付退回 · DESIGN | `parity/1248-8823.png` | [1248:8823](reference/1248-8823.png) |
| `1248:8879` · Admin Desktop · 結算單 · 退回款帶回 · DESIGN | `parity/1248-8879.png` | [1248:8879](reference/1248-8879.png) |
| `1248:8960` · Admin Mobile · 結算單 · 退回款帶回 · DESIGN | `parity/1248-8960.png` | [1248:8960](reference/1248-8960.png) |
| `21:677` · Admin Desktop · 方案與帳單 · 正常 · SHOPLINE | `parity/21-677.png` | [21:677](reference/21-677.png) |
| `21:1020` · Admin Mobile · 方案與帳單 · 正常 · SHOPLINE | `parity/21-1020.png` | [21:1020](reference/21-1020.png) |
| `30:1909` · Admin Desktop · 訂閱需要處理 · SHOPLINE | `parity/30-1909.png` | [30:1909](reference/30-1909.png) |
| `30:1960` · Admin Mobile · 訂閱需要處理 · SHOPLINE | `parity/30-1960.png` | [30:1960](reference/30-1960.png) |
| `750:3455` · Admin Desktop · 外部帳單管理說明 · SHOPLINE | `parity/750-3455.png` | [750:3455](reference/750-3455.png) |
| `750:3597` · Admin Mobile · 外部帳單管理說明 · SHOPLINE | `parity/750-3597.png` | [750:3597](reference/750-3597.png) |
| `1220:10379` · Admin Desktop · 方案與帳單 · 未訂閱 · SHOPLINE | `parity/1220-10379.png` | [1220:10379](reference/1220-10379.png) |
| `1220:10444` · Admin Mobile · 方案與帳單 · 未訂閱 · SHOPLINE | `parity/1220-10444.png` | [1220:10444](reference/1220-10444.png) |
| `1220:10490` · Admin Desktop · 方案與帳單 · 週期末取消 · SHOPLINE | `parity/1220-10490.png` | [1220:10490](reference/1220-10490.png) |
| `1220:10555` · Admin Mobile · 方案與帳單 · 週期末取消 · SHOPLINE | `parity/1220-10555.png` | [1220:10555](reference/1220-10555.png) |
| `1220:10601` · Admin Desktop · 方案與帳單 · 付款逾期 · SHOPLINE | `parity/1220-10601.png` | [1220:10601](reference/1220-10601.png) |
| `1220:10666` · Admin Mobile · 方案與帳單 · 付款逾期 · SHOPLINE | `parity/1220-10666.png` | [1220:10666](reference/1220-10666.png) |
| `1220:10712` · Admin Desktop · 方案與帳單 · 已受限 · SHOPLINE | `parity/1220-10712.png` | [1220:10712](reference/1220-10712.png) |
| `1220:10777` · Admin Mobile · 方案與帳單 · 已受限 · SHOPLINE | `parity/1220-10777.png` | [1220:10777](reference/1220-10777.png) |
| `1227:14766` · Admin Desktop · 方案與帳單 · 已收款返回（更新中） · SHOPLINE | `parity/1227-14766.png` | [1227:14766](reference/1227-14766.png) |
| `1227:14848` · Admin Desktop · 方案與帳單 · 已取消結帳 · SHOPLINE | `parity/1227-14848.png` | [1227:14848](reference/1227-14848.png) |
| `1227:14956` · Admin Mobile · 方案與帳單 · 已收款返回（更新中） · SHOPLINE | `parity/1227-14956.png` | [1227:14956](reference/1227-14956.png) |
| `1227:15015` · Admin Mobile · 方案與帳單 · 已取消結帳 · SHOPLINE | `parity/1227-15015.png` | [1227:15015](reference/1227-15015.png) |
| `94:2153` · Admin Desktop · 貨到付款設定 · SHOPLINE | `parity/94-2153.png` | [94:2153](reference/94-2153.png) |
| `94:2223` · Admin Mobile · 貨到付款設定 · task layout | `parity/94-2223.png` | [94:2223](reference/94-2223.png) |
| `339:2831` · Admin Desktop · 貨到付款儲存檢查 · 原型示意 | `parity/339-2831.png` | [339:2831](reference/339-2831.png) |
| `339:2845` · Admin Mobile · 貨到付款儲存檢查 · 原型示意 | `parity/339-2845.png` | [339:2845](reference/339-2845.png) |

## Relay 2 · formatter architecture repair (2026-10-30)

- Exact implementation head `98769c0566e25d32915f79121d1772a7f5c9ff69`: Foundation run `41063976394`, job `124090413086`, failed G-UI3/G-UI5 on component-local formatters (COD two, Finance one). Raw filtered receipt: `relay2-foundation-failed.log`. This is an implementation failure, not the card's causal tests-only RED.
- Targeted attempt 1: reuse the existing shared `decimal` for server-read integer COD values and `displayTime` for the server `generated_at`. The server finance timezone and shared presentation timezone are both `Asia/Taipei` (`internal/reporting/finance.go:33`, `packages/format/src/index.ts`). Finance now uses the canonical full-year, 24-hour timestamp presentation already used by billing and settlements; the `datetime` attribute still contains the exact server instant. No client clock, amount calculation, shared formatter edits or new dependency.
- Import-harness inventory: `rg -rlE '(Finance|CodSettings)\.tsx' tests/` finds only `tests/admin/reports-render.test.mjs`. Its Finance loader now compiles the real shared formatter; no assertions removed or relaxed. All existing browser tests remain unchanged.
- E1 only: admin `tsc --noEmit` exit 0 using temporary worktree dependency symlinks to the existing read-only dependencies (removed afterwards); initial unresolved-dependency invocation was corrected without source changes. `node --check tests/admin/reports-render.test.mjs`, browser ownership generation and `--check`, depmap regeneration, packet structure and whitespace checks all exit 0. Packet's incidental timestamp output restored; depmap has no diff.
- Own acceptance remains E0. GREEN/E3 and independent E4 parity/click acceptance pending the repair head's CI. No local tests, servers, browser, PG, provider or production execution. Integrator relays the new Gates/Foundation results; no merge or auto-merge.
- Additional literal-scope paths in this repair: `tests/admin/reports-render.test.mjs` (direct import harness), generated browser ownership JSON and this card's delivery/evidence. No product path outside the card.


## Relay 3 · date editor lifecycle repair (2026-10-30)

- Original CI head `6c6f2bbb97923c35ba91f860b7fe484d962db628`: Foundation `41066301920` / `124098481868` passed. Gates `41066301715` / `124114310661` (bundle:09) failed the ops-polish member; OP3 timed out waiting for `Daily details`. The CI snapshot remained on `Finance date range` after Apply. All seven other admin OP2/OP4 cases passed. Filtered receipt: `relay3-failure.log`; original artifact `gate-browser-ops-polish`, ID `12987356489`. Synthetic execution SHA `11dc175338ff29d44b92a81ce51c2ba8b02030fb`.
- Root cause: App Router preserves Finance component state across query navigation; submit changed the URL but left view=dates. Targeted lifecycle attempt 1 opens the signed overview on valid submit, including unchanged dates, then keeps the same server navigation. No amount, API, permission, range guard or formatter change.
- Authority: Fleet Bridge read-only get_design_context and get_screenshot re-read `1173:10742` and `1173:10743` on 2026-10-30; Apply navigates to overview. No Figma write. Existing signed reference PNGs remain the baseline; this repair changes no layout/copy.
- Regression: failing OP3 spec unchanged. The customers-billing click ledger now requires overview BEFORE refresh after changed dates and adds same-date resubmission with overview and exact URL assertions before/after refresh, for zh-TW/zh-CN/en at desktop/mobile sizes. No assertions removed/weakened; no import or harness change.
- Static E1: admin tsc --project output/parity-money/tsconfig.static.json --noEmit, node --check tests/admin/customers-billing.spec.ts, browser ownership generation/--check, and git diff --check exit 0. Static design detector reports []. Temporary dependency symlinks removed. No local tests, browser, server, PG or production run.
- Repair acceptance E0; PR GREEN and independent E4 pending new head. Scope additions: existing browser spec, generated ownership JSON, this delivery and filtered CI failure receipt only. Integrator relays new Gates/Foundation conclusion; no merge/auto-merge. Humaux unavailable: store/canvas/code-index not performed; this delivery and PR contain the handoff.


## Relay 4 · CSV preview refresh repair (2026-10-30)

- Current relay source head `2941fc48e1851561193ee5c8860af8691a15a5ab`: Foundation `41076945549` / `124135542633` passed. Gates `41076945403` / `124141385483` (bundle:08) failed `TestBrowserOC`, specifically its three finance CSV cases at `tests/admin/oc/reauth.spec.ts:189`. All six other OC cases passed. Original artifact `gate-browser-password-auth` ID `12988274746`; execution merge SHA `abb94af78098be7c15e7f2fd82bfed684898d989`; filtered receipt `relay4-failure.log`. This is an implementation regression, not a replacement for the original tests-only RED.
- Root cause: Finance restored daily/basis views from the URL but omitted the read-only CSV preview. Refresh after a successful, once-only re-auth/download returned to overview, removing the download control. Targeted attempt 1 persists/restores `view=csv` through the existing read-only navigation mechanism. Refresh makes no download request; the original OC guard/replay/download assertions stay unchanged. No amount, server, credential, API, permission or layout change.
- Signed authority re-read through Fleet Bridge: `1173:10744` and `1173:10745`, get_design_context + get_screenshot, on 2026-10-30. No Figma writes. Same desktop/mobile preview fields, range, download and return controls; summary/data facts/copy tables above remain unchanged.
- Regression coverage: original failing OC tests unchanged, including no repeated download after reload. Existing customers-billing real-click ledger now additionally requires URL/preview/download-control persistence after reload in all three locales and both desktop/mobile viewports before clicking Download. No assertions removed or weakened; no added imports, so compile harness dependencies are unchanged.
- Static E1: admin tsc --noEmit, node --check customers-billing.spec.ts, ownership generation and --check, git diff --check exit 0. Temporary dependency symlinks removed. Local tests/servers/browser/PG/providers/production NOT_RUN. PR GREEN/E3 product acceptance and independent E4 pending the new push; author acceptance E0.
- This relay changes Finance.tsx, the existing customers-billing spec, generated ownership, this delivery and the filtered failure receipt only. Outside literal product paths: allowed browser/generated files and card evidence. No new cross-module path; no merge/auto-merge. Humaux unavailable: store/canvas/code-index NOT_RUN; durable handoff is this delivery and PR. Integrator relays own Gates/Foundation conclusions in a fresh session.
