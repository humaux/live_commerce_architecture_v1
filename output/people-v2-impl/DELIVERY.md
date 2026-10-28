# PEOPLE-V2-IMPL · implementation awaiting own PR CI

**Structure ready (E1); author assessment E0. GREEN / independent acceptance PENDING.**
PR #289 · branch `unit/people-v2-ui` · target `r3/integration`. No merge or auto-merge.
This relay implements the signed UI; it does not claim product completion.

## Authority, scope and rollback

Read AGENTS.md and architecture §§0/1/2/14/26/27, customers-billing-v1 CD1–CD8 and §§5/6/7,
the current customer DTO parsers/client/route loaders and direct test harnesses.
UI authority: assigned `ux-people/output/ux-people/DELIVERY.md`, latest simplify section,
24 frames / 12 desktop-mobile pairs. Authority SHA256: `0536aab8723ca87ea28268d9d2abf76b5d99a34700affff42d83a0a0adb1e2e7`.
Fleet Bridge only: status showed one connected document; fresh read-only design-context
screenshots for desktop profile 1429:44396 and mobile list 1429:44240 are in evidence/.
Prior unknown-result readback identifies that state as read-only, not a backend queue.
No Figma writes or new design frames; copy-lint of changed Figma frames is NOT_APPLICABLE.

Exact original scope is in PLAN.md. Additional direct customer-module paths, using the card's
SCOPE PRE-APPROVAL, are listed explicitly in the PR: detail page (bounded return-query
context), customer-tags browser spec (click existing tools) and import-wizard browser spec
(name plus scope confirmation). No BFF/API, contract, migration, pin or dependency changes.
Rollback point: tests-only head `8e19c18721bca3f778a7ff7683afcfd5e5b2f708`; revert the
implementation commit on this branch if rejected. No database or production writes.

## Implementation and truth boundaries

- Search/view/clear/back match the concise signed flow. The detail page accepts only the
  list's existing bounded q/tag grammar for return context; other/repeated keys still refuse.
- Existing optional tag/note editing, reports/import and audit history remain under one
  native Advanced disclosure. Saved values always re-read the existing server response.
- Detail keeps store/name/customer ID/phone suffix, order count/net receipts/first date,
  every returned order, current consent with source/date and actual tags/notes. No sample
  name, tag, note, order number, receipt amount or consent is inserted by production code.
- Download requires review before the single keyed POST. Erasure requires exact displayed
  name plus a scope checkbox; the unchanged command still sends `{confirm:"ERASE"}`.
  Scope, retained orders/financial/delivery records, social separation, blockers and
  irreversibility remain explicit in both empty and ready confirmation states.
- Synchronous success/refusal stays authoritative. A lost result uses only the signed
  read-only path, with no retry control or POST replay. A sessionStorage marker stores only
  command kind under SHA256(session boundary + store + customer), never payload/name/phone/
  idempotency key. It is set before dispatch so reload during a command stays conservative.
  Storage unavailable fails closed. Marker is cleared only after definite result; an
  unconfirmed marker has no automatic unlock because the DTO has no request-status ID.
  New sign-in boundaries do not inherit it. This is a local safety fence, not a server request queue.
- A list consent boolean cannot prove a withdrawal event; list false remains “not granted”.
  Detail uses the returned withdrawal event to show “withdrawn”. No consent is inferred.

## RED → GREEN evidence

RED head `8e19c18721bca3f778a7ff7683afcfd5e5b2f708`; Gates run `40761011712`,
job `123079851595` (bundle:07), synthetic merge `da0f49dc9f4e0a0ae244b86d6753a89c9c16fb00`.
GitHub artifact `12853485502`, retained excerpt `evidence/red-assertions.log` and red.log.
Six signed-search assertions failed on the missing placeholder (three locales × desktop/mobile).
Twelve existing browser cases passed. Ten other new detail cases stopped before confirmation
assertions because MOCK order timestamps lacked the unchanged parser's six fractional digits.
Those timestamps and the blocked-error envelope are corrected to the actual closed DTO;
no parser, assertion, threshold, timeout or exemption was weakened. Do not claim causal RED
for the confirmation branches; their first functional result awaits implementation CI.
RED Foundation `40761011425` / `123072614938` passed, not current implementation acceptance.

Local static receipts: node --check for changed TS/MJS sources exits 0. Strict admin tsc exits
0 with temporary source aliases for existing shared packages (`tsc-mapped.log`); the ordinary
local dependency layout lacks UI/format links (`tsc.log`) and was not changed. Temporary
aliases and dependency symlinks are removed. `python3 scripts/check_packet.py` exits 0,
structure only (`packet.log`); its timestamp-only generated file is restored. Browser ownership
is regenerated after merging origin/r3/integration; depmap.sh ran with no resulting diff.
File hashes are in `evidence/implementation-hashes.json`. No local tests or servers ran.

## Click ledger and CI handoff

All added UI mutations use real click/fill/check/uncheck/reload. Scoped routes only prepare
MOCK DTOs/faults. page.evaluate only measures layout/text; it never changes DOM or state.
Each new case writes its control ledger and native text-density observations into the CI
browser artifact. Pending CI is not a PASS. Existing regression cases retain their assertions,
updated to the signed controls, and source-compiling harnesses receive every required global.

| State / control | Action | Expected | Actual |
|---|---|---|---|
| Main, 3 locales × 1440/390 | search / submit / refresh / clear | correct prompt, persistent query, scoped results | PENDING CI |
| Results / profile | view / back | identity and original search context | PENDING CI |
| Profile | orders / consent / privacy anchors | visible matching server facts | PENDING CI |
| Existing optional tools | Advanced / filter / tag-note actions | original scoped behavior and refresh persistence | PENDING CI |
| Download review | open / return / confirm | identity + scope before one POST/file | PENDING CI |
| Erasure empty / ready | wrong/right name + check/uncheck | both gates required, unchanged safety facts | PENDING CI |
| Erasure refusal / success | submit / refresh | server refusal or deactivation; retained orders | PENDING CI |
| Unconfirmed result / entry / return | pending / view / refresh / back | identity retained, commands locked, no replay | PENDING CI |

## DESIGN PARITY · required facts

The implementation uses one shared responsive data tree and one shared review per command.
This is source inspection, not browser evidence. Every row still requires CI/independent review.

| REQUIRED field / fact | Desktop source | Mobile source | Summary / review states |
|---|---|---|---|
| Store, name, customer ID, phone suffix | list / detail response | same responsive tree | shared identity block in both reviews and every unconfirmed state |
| Order count, net receipts, first recorded date | profile | same profile | actual order count in export/erasure scope; financial total remains on read-only profile |
| Each returned order ID, amount, status/date | order rows | same rows, wrapping | retained behind the review and after read-only return; no fake transaction outcome |
| Consent purpose/channel, source/date and current value | consent rows | same rows | erasure scope explicitly withdraws; profile remains readable |
| Manual tags and internal note | actual tags / latest note | same fields | optional editing/audit detail remains under Advanced |
| Only this store/customer, orders, consent, live-sale records, privacy actions | export review | same review | no fabricated scope choice |
| Protected delivery info, 200-order cap, identity check, permissions | export review | same review | all four remain visible before confirmation |
| Withdraw / sign out / deactivate / unused address / eligible live-sale records | erasure review | same review | identical in empty and ready states |
| Retained orders / payments / refunds / delivery / legal records; social data separate | erasure review | same review | identical in empty and ready states |
| Open-order/payment/refund blockers and irreversibility | erasure review | same review | safe failure from server remains visible |
| Exact name plus explicit scope check | native dialog | same dialog | neither input alone enables submit |
| Request sent, unconfirmed result, no repeat, privacy lock | result / entry / profile | same states | local result only; never claims backend processing/completion |

## UI PROCESS · BOURDIEU

Q1 task is find one store customer and safely review their data. Q2 novice/expert means recent
experience with this task; concise primary search and optional existing tools serve both.
Q3 no identity or income is inferred from a nickname, device or tag. Q4 irreversible action and
retention facts stay visible before consent. Q5 device changes only layout, never permissions
or the value of customer facts. Q6 aesthetics are separate; this lane follows owner-signed
frames, while comprehension/runtime visual acceptance remains PENDING.

## UI PROCESS · PROJECT UNDERSTANDING

Architecture §§0/1/2: evidence labels, strict TypeScript frontend, existing Go/PG authority;
§14: identity/transaction/touchpoint separation and purpose-specific consent;
§§26/27: assigned worktree, narrow ownership and independent acceptance.
CD1–CD8: existing identity projection, scoped reads, no merchant consent grant, synchronous
privacy, protected exports, precise erasure/retention and tombstones. §§5/6/7: frozen route
shape, permissions/idempotency and bounded export. Current server DTO is the source of values.
Owner's signed simplified frames and latest copy table govern presentation; no new settings.

## UI PROCESS · MOBBIN → EAGLE

Approved customer folder `MVQL9YUYU0QJ0`: Square `MVP1C226OIGO0` contributes search/result
hierarchy; Shopify `MVP1C22DSX0X6` contributes identity/order context; Klaviyo `MVRS5VC9CEFOF`
contributes purpose/channel consent structure. These are the approved design's recorded
references, not a new live Eagle audit. No images, brands or product facts copied. No new
pattern/import needed; a new reference request would go to the integrator, not a lane tool.

## UI PROCESS · FIGMA OFFICIAL GUIDANCE

Read `ux-people/output/ux-audit-ref/FIGMA-GUIDE.md` (2026-10-28). Applied G1 auto layout as
responsive grid/flex + wrapping, G2 existing semantic variables, G3 reused components/native
controls, G4 real navigation/review/return states, G5 success/refusal/unconfirmed branches,
G6 explicit route/data/keyboard/i18n handoff, G7 audit incumbent UI before composing.
Guide sources: [Auto layout](https://help.figma.com/hc/en-us/articles/360040451373-Guide-to-auto-layout-in-Figma),
[variables](https://help.figma.com/hc/en-us/articles/14506821864087-Overview-of-variables-collections-and-modes),
[components](https://help.figma.com/hc/en-us/articles/360038662654-Guide-to-components-in-Figma),
[prototyping](https://help.figma.com/hc/en-us/articles/360040314193-Guide-to-prototyping-in-Figma).
No Figma prototype or frame was modified.

## Density and NOT_RUN

The supplied `density-probe.json` is a Figma program, not a native DOM browser gate. The signed
authority measured 12 mobile frames: 10 blocks upper median / 100 characters upper median /
18 maximum blocks. Those values describe the approved Figma only. This lane adds raw native
text-block measurements in each CI click ledger step; they require independent comparison
against the corresponding signed states, without reusing the Figma count as product proof.
Runtime density, screenshots, full control coverage, accessibility, current-head GREEN and
independent acceptance remain PENDING/NOT_RUN until PR CI. No local browser/test/server/PG/
container/provider run; no SANDBOX credentials, LIVE activity, production action or deploy.

Humaux search/store/canvas/coord tools are not exposed in this relay; no shared-state write is
claimed. Integrator must attach this relay to `integrator-sub-people-v2-impl`. No delegation.
Author E0; static structure E1; causal search RED E3 bound to its old SHA; no E3 GREEN yet.

## Three-language signed authority table

The fixed names/amounts below are the signed design's sample copy only; production renders
current server data. The following table is copied from the latest assigned authority.

| zh-TW | zh-CN | en |
|---|---|---|
| 3 | 3 | 3 |
| 3 筆訂單 · 實收 NT$3,840 | 3 笔订单 · 实收 NT$3,840 | 3 orders · Net receipts NT$3,840 |
| DaWan Live | DaWan Live | DaWan Live |
| LC-10402 · NT$1,280<br>已完成 · 10-02 | LC-10402 · NT$1,280<br>已完成 · 10-02 | LC-10402 · NT$1,280<br>Completed · 10-02 |
| LC-10460 · NT$1,280<br>已完成 · 10-18 | LC-10460 · NT$1,280<br>已完成 · 10-18 | LC-10460 · NT$1,280<br>Completed · 10-18 |
| LC-10482 · NT$1,280<br>待出貨 · 10-25 | LC-10482 · NT$1,280<br>待发货 · 10-25 | LC-10482 · NT$1,280<br>Awaiting shipment · 10-25 |
| NT$3,840 | NT$3,840 | NT$3,840 |
| 下載處理中，暫停下載與抹除。 | 下载处理中，暂时不能下载或清除资料。 | Download processing. Downloads and erasure are paused. |
| 下載要求已送出 | 下载申请已提交 | Download request submitted |
| 下載顧客資料 | 下载客户资料 | Download customer data |
| 保留 3 筆訂單、付款退款、訂單收件及法定紀錄；社群資料另行處理。 | 保留 3 笔订单、支付退款、订单收件信息及法定记录；社交平台资料单独处理。 | Keep 3 orders, payments, refunds, order delivery details and legally retained records. Handle social data separately. |
| 個人化廣告 · Meta<br>已同意 · 顧客設定 · 2026-10-01 | 个性化广告 · Meta<br>已同意 · 客户设置 · 2026-10-01 | Personalized ads · Meta<br>Consent given · Customer settings · 2026-10-01 |
| 先核實身分 | 先核实申请人身份 | Verify the requester's identity first |
| 內部備註：偏好焦糖色；曾詢問門市取貨。 | 内部备注：偏好焦糖色；曾咨询到店自提。 | Internal note: Prefers caramel; asked about store pickup. |
| 同意 | 授权记录 | Consent |
| 商家後台 | 商家后台 | Merchant admin |
| 回購客 · 人工標籤 | 复购客户 · 手动标签 | Returning customer · Manual tag |
| 姓名開頭／電話末碼 | 姓名开头／手机号尾号 | Name prefix / last phone digits |
| 實收金額 | 实收金额 | Net receipts |
| 已有下載要求待處理 | 已有下载申请待处理 | Download request pending |
| 已有抹除要求待處理 | 已有资料清除申请待处理 | Erasure request pending |
| 已核對處理與保留範圍 | 已核对清除与保留范围 | I checked what will change and what will be kept |
| 找到 1 位顧客 | 找到 1 位客户 | 1 customer found |
| 抹除處理中，暫停下載與抹除。 | 资料清除处理中，暂时不能下载或再次清除。 | Erasure processing. Downloads and further erasure are paused. |
| 抹除要求已送出 | 资料清除申请已提交 | Erasure request submitted |
| 抹除顧客 | 清除客户资料 | Erase customer data |
| 搜尋 | 搜索 | Search |
| 撤回同意、登出裝置、停用顧客；移除未用於訂單的收件資料，改寫可處理的直播登記。 | 撤回授权、退出已登录设备、停用客户账号；删除未用于订单的收件信息，改写可处理的直播登记。 | Withdraw consent, sign out devices and deactivate the customer. Remove delivery details unused by orders and redact eligible live-sale records. |
| 收件資料受保護；最多200筆訂單，妥善保管 | 收件信息受保护；最多200笔订单，请妥善保管 | Protected delivery details; up to 200 orders. Keep the file secure |
| 暫停下載與抹除，請先查看顧客。 | 暂时不能下载或清除资料，请先查看客户记录。 | Downloads and erasure are paused. Review the customer first. |
| 有未完結訂單、付款或退款時無法抹除；完成後無法復原。 | 有未完成的订单、支付或退款时不能清除；清除后无法恢复。 | Open orders, payments or refunds prevent erasure. Completed erasure cannot be undone. |
| 本店顧客 · 1 位 | 本店客户 · 1 位 | Store customers · 1 |
| 本店顧客、3筆訂單、同意歷史、直播登記、隱私操作 | 本店客户、3笔订单、授权历史、直播登记、隐私操作 | Customer, 3 orders, consent history, live-sale records and privacy actions from this store |
| 查看待處理要求 | 查看待处理申请 | View pending request |
| 查看顧客 | 查看客户 | View customer |
| 查看顧客資料 | 查看客户资料 | View customer data |
| 清除搜尋 | 清空搜索 | Clear search |
| 確認下載 | 确认下载 | Confirm download |
| 確認抹除 | 确认清除 | Confirm erasure |
| 示範店 | 示範店 | 示範店 |
| 示範店 · 示範林一<br>顧客編號 001 · 電話末 805 | 示範店 · 示範林一<br>客户编号 001 · 手机尾号 805 | 示範店 · 示範林一<br>Customer 001 · Phone ending 805 |
| 示範店 · 顧客 | 示範店 · 客户 | 示範店 · Customers |
| 示範店 · 顧客編號 001 · 電話末 805<br>3 筆訂單 · 實收 NT$3,840<br>首次紀錄 2026-09-12 | 示範店 · 客户编号 001 · 手机尾号 805<br>3 笔订单 · 实收 NT$3,840<br>首次记录 2026-09-12 | 示範店 · Customer 001 · Phone ending 805<br>3 orders · Net receipts NT$3,840<br>First recorded 2026-09-12 |
| 示範店·示範林一<br>編號001·電話末805 | 示範店·示範林一<br>编号001·手机尾号805 | 示範店 · 示範林一<br>Customer 001 · Phone ending 805 |
| 示範林 | 示範林 | 示範林 |
| 示範林一 | 示範林一 | 示範林一 |
| 示範資料 | 示例数据 | Sample data |
| 處理中，請勿重複操作。 | 正在处理，请勿重复提交。 | Processing. Do not submit again. |
| 行銷同意 | 营销授权 | Marketing consent |
| 行銷訊息 · Messenger / IG DM<br>已撤回 · 顧客設定 · 2026-10-20 | 营销消息 · Messenger / IG DM<br>已撤回 · 客户设置 · 2026-10-20 | Marketing messages · Messenger / IG DM<br>Withdrawn · Customer settings · 2026-10-20 |
| 行銷訊息：已撤回 | 营销消息：已撤回授权 | Marketing messages: Consent withdrawn |
| 訂單 | 订单 | Orders |
| 請先核實申請人身分。 | 请先核实申请人身份。 | Verify the requester's identity first. |
| 輸入「示範林一」確認 | 输入“示範林一”确认 | Type “示範林一” to confirm |
| 返回 | 返回 | Back |
| 返回待處理要求 | 返回待处理申请 | Back to pending request |
| 返回搜尋結果 | 返回搜索结果 | Back to results |
| 返回顧客資料 | 返回客户资料 | Back to customer |
| 隱私 | 隐私 | Privacy |
| 電話末 805 · 顧客編號 001 | 手机尾号 805 · 客户编号 001 | Phone ending 805 · Customer 001 |
| 需隱私及顧客／訂單讀取權限 | 需要隐私操作及客户／订单查看权限 | Requires privacy and customer/order read permissions |
| 顧客 | 客户 | Customers |
| 顧客 / 電話末碼 | 客户／手机号尾号 | Customer / last phone digits |
| 顧客姓名 | 客户姓名 | Customer name |
| 顧客隱私 | 客户隐私 | Customer privacy |

## Implementation strings (dynamic values remain server-owned)

| Key | zh-TW | zh-CN | en |
|---|---|---|---|
| title | 顧客 | 客户 | Customers |
| search | 姓名開頭／電話末碼 | 姓名开头／手机号尾号 | Name prefix / last phone digits |
| clear | 清除搜尋 | 清空搜索 | Clear search |
| view | 查看顧客 | 查看客户 | View customer |
| back | 返回搜尋結果 | 返回搜索结果 | Back to results |
| customerID | 顧客編號 | 客户编号 | Customer |
| phone | 電話末 | 手机尾号 | Phone ending |
| orders | 訂單 | 订单 | Orders |
| consent | 同意 | 授权记录 | Consent |
| privacy | 隱私 | 隐私 | Privacy |
| net | 實收金額 | 实收金额 | Net receipts |
| first | 首次紀錄 | 首次记录 | First recorded |
| download | 下載顧客資料 | 下载客户资料 | Download customer data |
| confirmDownload | 確認下載 | 确认下载 | Confirm download |
| erase | 抹除顧客 | 清除客户资料 | Erase customer data |
| confirmErase | 確認抹除 | 确认清除 | Confirm erasure |
| checked | 已核對處理與保留範圍 | 已核对清除与保留范围 | I checked what will change and what will be kept |
| verify | 先核實身分 | 先核实申请人身份 | Verify the requester's identity first |
| protected | 收件資料受保護；最多200筆訂單，妥善保管 | 收件信息受保护；最多200笔订单，请妥善保管 | Protected delivery details; up to 200 orders. Keep the file secure |
| permissions | 需隱私及顧客／訂單讀取權限 | 需要隐私操作及客户／订单查看权限 | Requires privacy and customer/order read permissions |
| scope | 本店顧客、${n}筆訂單、同意歷史、直播登記、隱私操作 | 本店客户、${n}笔订单、授权历史、直播登记、隐私操作 | Customer, ${n} orders, consent history, live-sale records and privacy actions from this store |
| eraseBody | 撤回同意、登出裝置、停用顧客；移除未用於訂單的收件資料，改寫可處理的直播登記。 | 撤回授权、退出已登录设备、停用客户账号；删除未用于订单的收件信息，改写可处理的直播登记。 | Withdraw consent, sign out devices and deactivate the customer. Remove delivery details unused by orders and redact eligible live-sale records. |
| eraseKept | 保留 ${n} 筆訂單、付款退款、訂單收件及法定紀錄；社群資料另行處理。 | 保留 ${n} 笔订单、支付退款、订单收件信息及法定记录；社交平台资料单独处理。 | Keep ${n} orders, payments, refunds, order delivery details and legally retained records. Handle social data separately. |
| eraseSafety | 有未完結訂單、付款或退款時無法抹除；完成後無法復原。 | 有未完成的订单、支付或退款时不能清除；清除后无法恢复。 | Open orders, payments or refunds prevent erasure. Completed erasure cannot be undone. |
| typeName | 輸入「${name}」確認 | 输入“${name}”确认 | Type “${name}” to confirm |
| name | 顧客姓名 | 客户姓名 | Customer name |
| pending | 查看待處理要求 | 查看待处理申请 | View pending request |
| pendingBack | 返回待處理要求 | 返回待处理申请 | Back to pending request |
| viewData | 查看顧客資料 | 查看客户资料 | View customer data |
| backCustomer | 返回顧客資料 | 返回客户资料 | Back to customer |
| exportSubmitted | 下載要求已送出 | 下载申请已提交 | Download request submitted |
| eraseSubmitted | 抹除要求已送出 | 资料清除申请已提交 | Erasure request submitted |
| exportPending | 已有下載要求待處理 | 已有下载申请待处理 | Download request pending |
| erasePending | 已有抹除要求待處理 | 已有资料清除申请待处理 | Erasure request pending |
| paused | 暫停下載與抹除，請先查看顧客。 | 暂时不能下载或清除资料，请先查看客户记录。 | Downloads and erasure are paused. Review the customer first. |
| unconfirmed | 尚未確認處理結果，請勿重複操作。 | 尚未确认处理结果，请勿重复提交。 | The outcome has not been confirmed. Do not submit again. |
| storeCount | 本店顧客 · ${n} 位 | 本店客户 · ${n} 位 | Store customers · ${n} |
| found | 找到 ${n} 位顧客 | 找到 ${n} 位客户 | ${n} customer${n === 1 ? "" : "s"} found |
| marketing | 行銷訊息 · Messenger / IG DM | 营销消息 · Messenger / IG DM | Marketing messages · Messenger / IG DM |
| ads | 個人化廣告 · Meta | 个性化广告 · Meta | Personalized ads · Meta |
| settings | 顧客設定 | 客户设置 | Customer settings |
| advanced | 進階 | 高级 | Advanced |
| granted | 已同意 | 已同意 | Consent given |
| withdrawn | 已撤回 | 已撤回授权 | Consent withdrawn |

## Retained customer controls and fallback copy

| Key | zh-TW | zh-CN | en |
|---|---|---|---|
| actionCol | 操作 | 操作 | Action |
| actionsNone | 尚無隱私操作。 | 尚无隐私操作。 | No privacy actions yet. |
| actionsSection | 隱私操作 | 隐私操作 | Privacy actions |
| boundAt | 關聯時間（台北時間） | 关联时间（台北时间） | Linked (Taipei time) |
| claimsNone | 沒有登記。 | 没有登记。 | No claims. |
| claimsSection | 留言登記 | 评论登记 | Comment claims |
| consentGranted | 已授權 | 已授权 | Granted |
| consentHistory | 歷史紀錄 | 历史记录 | History |
| consentHistoryNone | 尚無授權紀錄。 | 尚无授权记录。 | No consent has been recorded. |
| consentNone | 未授權 | 未授权 | Not granted |
| consentSection | 授權 | 授权 | Consent |
| customer | 客戶 | 客户 | Customer |
| detailLoading | 正在載入客戶… | 正在加载客户… | Loading customer… |
| downloadDone | 資料檔案已下載。 | 数据文件已下载。 | The data file was downloaded. |
| downloading | 準備中… | 准备中… | Preparing… |
| empty | 目前還沒有客戶。 | 还没有客户。 | No customers yet. |
| emptySearch | 沒有符合搜尋的客戶。 | 没有符合搜索的客户。 | No customers match this search. |
| erased | 該客戶已被抹除。 | 该客户已被抹除。 | This customer was already erased. |
| erasedState | 該客戶已被抹除。其訂單、付款與出貨紀錄予以保留。 | 该客户已被抹除。其订单、付款与出货记录予以保留。 | This customer was erased. Their orders, payments and shipments are kept. |
| erasing | 抹除中… | 抹除中… | Erasing… |
| facts | 客戶 | 客户 | Customer |
| forbidden | 此帳號無權執行此操作。 | 此账号无权执行此操作。 | This account does not have permission for this action. |
| importLink | 從 SHOPLINE 搬家 | 从 SHOPLINE 搬家 | Move from SHOPLINE |
| imported | 已匯入 | 已导入 | Imported |
| lines | 商品數 | 商品数 | Items |
| loading | 正在載入客戶… | 正在加载客户… | Loading customers… |
| marketing | 行銷訊息（Messenger / Instagram 私訊） | 营销消息（Messenger / Instagram 私信） | Marketing messages (Messenger / Instagram DM) |
| next | 下一頁 | 下一页 | Next page |
| noName | 未記錄姓名 | 未记录姓名 | Name not recorded |
| noStore | 沒有可用的授權店鋪。 | 没有可用的授权店铺。 | No authorized store is available. |
| notFound | 所選店鋪中沒有這位客戶。 | 所选店铺中没有这位客户。 | This customer is unavailable in the selected store. |
| open | 開啟客戶 | 打开客户 | Open customer |
| orderId | 訂單 | 订单 | Order |
| orderOpen | 在訂單中開啟 | 在订单中打开 | Open in orders |
| orders | 訂單 | 订单 | Orders |
| ordersNone | 沒有訂單。 | 没有订单。 | No orders. |
| ordersSection | 訂單 | 订单 | Orders |
| pageCount | 本頁客戶數 | 本页客户数 | Customers on this page |
| phoneEnding | 手機末碼 | 手机尾号 | Phone ends |
| platform | 平台 | 平台 | Platform |
| policy | 聲明版本 | 声明版本 | Notice version |
| previous | 上一頁 | 上一页 | Previous page |
| privacySection | 隱私 | 隐私 | Privacy |
| refresh | 重新整理 | 刷新 | Refresh |
| reportsLink | 銷售報表 | 销售报表 | Sales reports |
| retry | 重試 | 重试 | Try again |
| searchButton | 搜尋 | 搜索 | Search |
| signedOut | 請登入後查看客戶。 | 请登录后查看客户。 | Sign in to view customers. |
| source | 記錄來源 | 记录来源 | Recorded by |
| total | 合計 | 合计 | Total |
| unavailable | 客戶暫時無法使用，請重試。 | 客户暂时无法使用，请重试。 | Customers are temporarily unavailable. Try again. |
| when | 時間（台北時間） | 时间（台北时间） | When (Taipei time) |
| withdraw | 記錄撤回 | 记录撤回 | Record withdrawal |
| withdrawDone | 已記錄撤回。 | 已记录撤回。 | Withdrawal recorded. |
| withdrawHint | 當客戶要求停止時使用。客戶只能在自己的隱私頁重新授權。 | 当客户要求停止时使用。客户只能在自己的隐私页重新授权。 | Use this when the customer asks you to stop. The customer can grant again only from their own privacy page. |
| withdrawing | 記錄中… | 记录中… | Recording… |

Existing Advanced tag/note/import/history controls retain their three-locale dictionaries in
customer-tags-copy.ts, import-history-copy.ts and orders-copy.ts. No copy in those modules
is changed. Error/source/kind/via labels retain customers-copy.ts's existing locale maps;
only the signed buyer-settings source uses the table above. Erasure completion counts come
from the existing localized erasedDone formatter and actual server summary.

## CI startup conflict repair · attempt 1

Implementation `7eed1ab3cecef8700962f1b84c099e6ceead9497` pushed successfully, but GitHub
reported DIRTY/CONFLICTING and no check suite/run. Current integration had meanwhile
advanced to `dcecd6220094234b4760315c676d9737ed713c40`. Merged it into this branch; the
only content conflict was the shared i18n index header. Retained both stateRecoveryCopy
and peopleCopy exports and described both dependencies in the header. Other imported
code is unchanged from integration. Regenerated ownership and depmap; strict mapped
admin tsc and index syntax check exit 0. One repair push is required to let PR CI start;
no item-by-item implementation pushes or test weakening. Temporary links/config removed.

## Previous-head wait receipt (superseded by RELAY 2)

PR #289 is MERGEABLE at `5aeb6529d84fd3fd5ad342badc8e97c6337d3ed2`.
Gates run `40775436305` and Foundation run `40775436307` are QUEUED; test job IDs
and GREEN are PENDING. WAIT_RUN: 40775436305,40775436307. The integrator relays
completion in a fresh session. No polling loop or local process remains. Humaux
store/canvas NOT_RUN because those tools are not exposed; no completion claim.

## RELAY 2 · shared formatter gate repair · attempt 1

The current implementation head `5aeb6529d84fd3fd5ad342badc8e97c6337d3ed2`
failed Foundation run `40775436307`, job `123123404293`, at the unchanged
G-UI3/G-UI5 architecture assertion: `CustomerDetail.tsx: format 1 > 0`.
Original filtered receipt: `evidence/relay2-foundation-failure.log`. This is an
implementation gate regression, separate from the historical tests-only search RED.

Root cause: a date-only display introduced a component-local `Intl.DateTimeFormat`
instead of using the shared formatting authority. Reused the existing imported
`displayTime("sv-SE", value)` and selected its date portion; it retains Asia/Taipei
and the signed YYYY-MM-DD display. The shared implementation remains in
`packages/format/src/index.ts`, re-exported by `orders-model.ts`. No new import,
dependency, harness global, exemption, baseline or assertion change. All three
discovered component harnesses retain their existing `displayTime` injection.
The existing browser main path asserts `2026-09-12` and consent date `2026-10-20`
in all three locales on both desktop and mobile; those assertions stay unchanged.

Static diff whitespace check exits 0. `node --check` cannot parse `.tsx` in this
Node runtime (`ERR_UNKNOWN_FILE_EXTENSION`); it is not a successful syntax receipt.
No local compiler rerun (worktree dependencies unavailable), tests or servers.
Previous compiler receipts predate this edit. Current-head GREEN and independent
acceptance remain PENDING; author assessment E0. No product completion claim.
No new path outside PLAN.md. Rollback: revert the single formatter repair commit.
Humaux store/canvas/coord unavailable; no shared-state update is claimed.

## RELAY 2 · repair-head wait receipt (local, no evidence-only push)

Head `d6e11ce5b82cf5d9e76789b2d4cdcae124eade93` pushed to PR #289.
Foundation `40776603959` / job `123127418798` QUEUED; Gates `40776604067`
PENDING. PR is MERGEABLE; GREEN and independent acceptance remain PENDING.
WAIT_RUN: 40776603959,40776604067. Integrator relays these run conclusions in a
fresh session. No local tests/servers/background processes or merge/auto-merge.

## RELAY 3 · semantic colour repair (attempt 1 for this blocker)

Current-head Foundation `40776603959` / job `123127418798` failed on
`tests/ui/ds-hex-ratchet.test.mjs:87`: `customers.css` contains 10 literal hex
colours against baseline 8; global total 388 against 386. Original assertion
receipt: `evidence/relay3-foundation-failure.log`. This is a different blocker
from the prior formatter assertion; no claim of overall Foundation success.

Root cause: the signed PEOPLE-v2 styles added two literal white declarations.
Replace those declarations with existing `--ds-on-primary` (action foreground)
and `--ds-surface` (surface background). Both are `#fff` in the current globally
imported `packages/ui/src/tokens.css`; selector order and all other style values
are preserved. Tokens, tests, baselines, exemption lists and thresholds unchanged.
Scope: planned `apps/admin/components/customers.css` and card evidence only.
Rollback: revert this repair commit. No import/harness/browser ownership change.

No local tests, servers or CI-gate scripts run. The skill context/diagnostic
scripts are omitted because they are outside the card's local-check allowlist;
the signed authority and current source were read directly. Humaux tools remain
unavailable; no store/canvas/coord write is claimed. Author E0; repair-head CI and
independent acceptance PENDING. No completion claim.
