# UX-LOYALTY-POINTS · 會員積分 v1 · DESIGN MANIFEST

- Status: **DESIGN** (Figma frames only; nothing implemented, no product test run: NOT_RUN)
- Date: 2026-11-05 · lane UX-LOYALTY-POINTS · branch `unit/loyalty-points-ux` (base `b8305338b`, r3/integration)
- Figma file `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829` (10 · SHOPLINE Redesign), section **「會員積分 v1 · DESIGN」 `1554:62573`** at (490000, 605000)
- Written through the local Fleet Bridge (`use_figma`, `get_screenshot`). Only new nodes in the new section were written; no signed frame, COMPONENT or COMPONENT_SET was modified (so no bakref run was needed). The signed frames were only cloned as templates.
- Scope source: the owner-confirmed brief of 2026-11-05 as passed to this lane, plus the contract draft `contracts/loyalty-points-v1.md` on `origin/unit/loyalty-points-contract@e0d3e968` (read at the end; see U1 to U4)
- Frames: **77** (38 desktop 1440×900 + 39 mobile 390×844); exports in `figma/<node-id>.png` (git-ignored by `output/**/figma/`; hashes in `PNG-SHA256SUMS.txt`, largest file < 1 MiB, ADR-20261031)
- Generator: `figma-src/lib.js` + `figma-src/screens.js` (idempotent: re-running a key replaces the frame with the same name), QA probe `figma-src/qa.js`

> **2026-11-05 update in progress (integrator rulings on U1–U7, contract #549 head `e0d3e9686`): BLOCKED, second plugin disconnect.**
> Built so far: A11 `1554:79479` / `1554:79554`, A12 `1554:79596` / `1554:79683`, and rebuilt C1 `1556:80238` / `1556:80351`, C2 `1556:80406` / `1556:80486`, C3 `1556:80531` / `1556:80666`, C4 `1556:80743` / `1556:80868`, C5 `1556:80937` / `1556:81006`, C6 `1556:81040` / `1556:81108`.
> Disconnect 1 hit batch C1–C7; disconnect 2 hit batch C7+C8 (connection `7f43f326…`, `Figma disconnected. Operation was not replayed`). Both batches contained C7, so C7 is a suspect. The C7/C8 outcome is unknown; no write was replayed.
> Still to run (≤ 4 roots per call, start with C8 alone, then C7 alone): `C7–C17, D1–D7, E1, E2, E4, E5, E6`. Then export, run `qa.js` and refresh the table below. **Until then the C7+, D and E rows below are stale, and the ids of C1–C6 in the table are superseded by the ones above.**

## Templates and component reuse

| Used | Source (signed) | How |
|---|---|---|
| Desktop shell (Merchant navigation, Shell A / header, title, Scrollable task viewport 1136×620, Fixed action dock) | `1470:166550` 手工單 · customer | cloned per frame; content replaced |
| Mobile shell (app bar + 選單, viewport 358×582, dock) | `1470:166602` | cloned per frame |
| Nav items | `Nav Item` set `4:12737` (Item=customers State=Active for 會員 pages) + the custom `Navigation / 手工單` sub-item frame | 會員 sub-item = clone of the 手工單 sub-item with a new 18px person icon (stroke `color/teal`) |
| Primary / Secondary buttons | `Button/Kind=Primary` and `Kind=Secondary` instances from `1470:167239` / `1470:167241` | cloned, label overridden, prototype reactions cleared |
| Danger button (扣回本單積分) | `Button` set `4:12718` Kind=Danger | instance |
| Tags (本單贈送 success, 手動調整 neutral, 沖正 / 待確認 / 積分未贈送 / 已全數扣回 warning) | `Tag` set `1401:38432` | instances |
| Fields | same structure as the signed `Field / … → Value / …` frames (label 13, value 15, fill `color/subtle`, r5) | rebuilt with the same tokens; error = 1.5px `color/danger` stroke + 13px `color/danger` text (same token as `InlineError 1429:34299`) |
| Mobile menu | `1470:165651` 手工單 · nav | cloned; 顧客 + 會員 items added |
| Type | Noto Sans SC Regular/Medium 13/14/15/16/20/28, PingFang SC Medium in buttons (as in the signed frames) | no new type styles |
| Color | variables only: collection `3:5` (`color/ink, muted, surface, subtle, canvas, line, teal`) and from `13:305` `color/danger` (as InlineError) and `color/muted` for text on `color/subtle` (see QA F3) | no new colors; scrim = `color/ink` at layer opacity 45% |

Not reused on purpose: `Segmented 1401:38449` (30px high, below the 44px touch target) — the 增加／扣除 toggle is a pair of 44px Primary/Secondary buttons; `StatePanel 1429:34298` and `Pagination 1401:58023` (navy buttons / page numbers: the manual-order frames use teal buttons, and the list is cursor-paged so it has no page numbers).

## Frames

| Node id | Name | Viewport | Purpose | State |
|---|---|---|---|---|
| `1554:74786` | DESIGN · Admin Desktop · 手工單 · 已建立 · 含積分 | 1440×900 | Confirmation (手工單已建立): award recorded, new balance, link to member | success, member_award.state=awarded |
| `1554:74861` | DESIGN · Admin Mobile · 手工單 · 已建立 · 含積分 | 390×844 | Confirmation (手工單已建立): award recorded, new balance, link to member | success, member_award.state=awarded |
| `1554:79210` | DESIGN · Admin Desktop · 手工單 · 已建立 · 積分未贈送 | 1440×900 | Order placed but award not recorded (contract LP18 not_awarded) with recovery hint | success + not_awarded |
| `1554:79286` | DESIGN · Admin Mobile · 手工單 · 已建立 · 積分未贈送 | 390×844 | Order placed but award not recorded (contract LP18 not_awarded) with recovery hint | success + not_awarded |
| `1554:74900` | DESIGN · Admin Desktop · 手工單 · 建立新會員 · 結果待確認 | 1440×900 | Inline member create answer lost: no auto-resend, 重新讀取 only, 下一步 disabled | UNKNOWN |
| `1554:74982` | DESIGN · Admin Mobile · 手工單 · 建立新會員 · 結果待確認 | 390×844 | Inline member create answer lost: no auto-resend, 重新讀取 only, 下一步 disabled | UNKNOWN |
| `1554:78492` | DESIGN · Admin Desktop · 手工單 · 扣回本單積分 · 確認 | 1440×900 | Reversal dialog prefilled with the award (editable), required note, balance preview | confirm |
| `1554:78584` | DESIGN · Admin Mobile · 手工單 · 扣回本單積分 · 確認 | 390×844 | Reversal dialog prefilled with the award (editable), required note, balance preview | confirm |
| `1554:79052` | DESIGN · Admin Desktop · 手工單 · 扣回本單積分 · 結果待確認 | 1440×900 | Reversal answer lost: 重新讀取 only | UNKNOWN |
| `1554:79134` | DESIGN · Admin Mobile · 手工單 · 扣回本單積分 · 結果待確認 | 390×844 | Reversal answer lost: 重新讀取 only | UNKNOWN |
| `1554:78644` | DESIGN · Admin Desktop · 手工單 · 扣回本單積分 · 超過贈送 | 1440×900 | Amount > award: inline error (brief rule; see UNKNOWN U1) | validation error |
| `1554:78734` | DESIGN · Admin Mobile · 手工單 · 扣回本單積分 · 超過贈送 | 390×844 | Amount > award: inline error (brief rule; see UNKNOWN U1) | validation error |
| `1554:78792` | DESIGN · Admin Desktop · 手工單 · 扣回本單積分 · 餘額不足 | 1440×900 | Member already spent points: reversal would go negative, max = balance | blocked |
| `1554:78882` | DESIGN · Admin Mobile · 手工單 · 扣回本單積分 · 餘額不足 | 390×844 | Member already spent points: reversal would go negative, max = balance | blocked |
| `1554:74275` | DESIGN · Admin Desktop · 手工單 · 會員 · 已選擇 | 1440×900 | Member chosen (change/remove) and 本單贈送積分 = 50 | filled |
| `1554:74361` | DESIGN · Admin Mobile · 手工單 · 會員 · 已選擇 | 390×844 | Member chosen (change/remove) and 本單贈送積分 = 50 | filled |
| `1554:74138` | DESIGN · Admin Desktop · 手工單 · 會員 · 建立新會員 | 1440×900 | Inline small create form (display name + contact note) inside the picker | inline create |
| `1554:74227` | DESIGN · Admin Mobile · 手工單 · 會員 · 建立新會員 | 390×844 | Inline small create form (display name + contact note) inside the picker | inline create |
| `1554:73997` | DESIGN · Admin Desktop · 手工單 · 會員 · 搜尋 | 1440×900 | Member picker open: substring search over name + contact note, results with balance, inline 建立新會員 entry | focus, results |
| `1554:74087` | DESIGN · Admin Mobile · 手工單 · 會員 · 搜尋 | 390×844 | Member picker open: substring search over name + contact note, results with balance, inline 建立新會員 entry | focus, results |
| `1554:73868` | DESIGN · Admin Desktop · 手工單 · 會員 · 未選擇 | 1440×900 | 手工單 收件資料 step with the new optional 會員與積分 card; nothing chosen | default, empty, helper text |
| `1554:73952` | DESIGN · Admin Mobile · 手工單 · 會員 · 未選擇 | 390×844 | 手工單 收件資料 step with the new optional 會員與積分 card; nothing chosen | default, empty, helper text |
| `1554:74670` | DESIGN · Admin Desktop · 手工單 · 核對 · 含積分 | 1440×900 | Order summary (核對手工單) with the member + award line | review |
| `1554:74744` | DESIGN · Admin Mobile · 手工單 · 核對 · 含積分 | 390×844 | Order summary (核對手工單) with the member + award line | review |
| `1554:74408` | DESIGN · Admin Desktop · 手工單 · 積分 · 未選會員 | 1440×900 | Points typed without a member: blocks 下一步 with inline errors | validation error |
| `1554:74492` | DESIGN · Admin Mobile · 手工單 · 積分 · 未選會員 | 390×844 | Points typed without a member: blocks 下一步 with inline errors | validation error |
| `1554:74537` | DESIGN · Admin Desktop · 手工單 · 積分 · 非整數 | 1440×900 | Points not a positive integer | validation error |
| `1554:74623` | DESIGN · Admin Mobile · 手工單 · 積分 · 非整數 | 390×844 | Points not a positive integer | validation error |
| `1554:78380` | DESIGN · Admin Desktop · 手工單 · 訂單詳情 · 含積分 | 1440×900 | Manual-order detail with member, award line, deductible amount, 扣回本單積分 | default |
| `1554:78452` | DESIGN · Admin Mobile · 手工單 · 訂單詳情 · 含積分 | 390×844 | Manual-order detail with member, award line, deductible amount, 扣回本單積分 | default |
| `1554:78940` | DESIGN · Admin Desktop · 手工單 · 訂單詳情 · 已扣回 | 1440×900 | After reversal: reversal line, 0 deductible, 已全數扣回 tag, action removed | after reversal |
| `1554:79012` | DESIGN · Admin Mobile · 手工單 · 訂單詳情 · 已扣回 | 390×844 | After reversal: reversal line, 0 deductible, 已全數扣回 tag, action removed | after reversal |
| `1554:75023` | DESIGN · Admin Desktop · 會員 · 列表 | 1440×900 | Member list: search, name / truncated contact note / balance / last change, cursor pagination (上一頁 disabled on first page), 新增會員 | default |
| `1554:75132` | DESIGN · Admin Mobile · 會員 · 列表 | 390×844 | Member list: search, name / truncated contact note / balance / last change, cursor pagination (上一頁 disabled on first page), 新增會員 | default |
| `1554:75189` | DESIGN · Admin Desktop · 會員 · 列表 · 搜尋無結果 | 1440×900 | Search with no match + 清除搜尋 | empty search |
| `1554:75261` | DESIGN · Admin Mobile · 會員 · 列表 · 搜尋無結果 | 390×844 | Search with no match + 清除搜尋 | empty search |
| `1554:75630` | DESIGN · Admin Desktop · 會員 · 列表 · 無權限 | 1440×900 | No customers:read; no data shown, no actions | forbidden |
| `1554:75695` | DESIGN · Admin Mobile · 會員 · 列表 · 無權限 | 390×844 | No customers:read; no data shown, no actions | forbidden |
| `1554:75299` | DESIGN · Admin Desktop · 會員 · 列表 · 空 | 1440×900 | No members yet | empty |
| `1554:75366` | DESIGN · Admin Mobile · 會員 · 列表 · 空 | 390×844 | No members yet | empty |
| `1554:75528` | DESIGN · Admin Desktop · 會員 · 列表 · 讀取失敗 | 1440×900 | Read failed, 重新讀取 (read only) | error |
| `1554:75596` | DESIGN · Admin Mobile · 會員 · 列表 · 讀取失敗 | 390×844 | Read failed, 重新讀取 (read only) | error |
| `1554:75399` | DESIGN · Admin Desktop · 會員 · 列表 · 載入中 | 1440×900 | Skeleton rows | loading |
| `1554:75483` | DESIGN · Admin Mobile · 會員 · 列表 · 載入中 | 390×844 | Skeleton rows | loading |
| `1554:75725` | DESIGN · Admin Desktop · 會員 · 新增會員 | 1440×900 | Create member dialog / bottom sheet | default |
| `1554:75850` | DESIGN · Admin Mobile · 會員 · 新增會員 | 390×844 | Create member dialog / bottom sheet | default |
| `1554:75922` | DESIGN · Admin Desktop · 會員 · 新增會員 · 名稱未填 | 1440×900 | Required display name missing | validation error |
| `1554:76047` | DESIGN · Admin Mobile · 會員 · 新增會員 · 名稱未填 | 390×844 | Required display name missing | validation error |
| `1554:76401` | DESIGN · Admin Desktop · 會員 · 編輯資料 | 1440×900 | Edit member info dialog; balance not editable here | default |
| `1554:76533` | DESIGN · Admin Mobile · 會員 · 編輯資料 | 390×844 | Edit member info dialog; balance not editable here | default |
| `1554:76608` | DESIGN · Admin Desktop · 會員 · 編輯資料 · 結果待確認 | 1440×900 | Edit answer lost: 重新讀取 / 關閉, no resend | UNKNOWN |
| `1554:76730` | DESIGN · Admin Mobile · 會員 · 編輯資料 · 結果待確認 | 390×844 | Edit answer lost: 重新讀取 / 關閉, no resend | UNKNOWN |
| `1554:76120` | DESIGN · Admin Desktop · 會員 · 詳情 | 1440×900 | Member detail: balance header, contact note, notes, append-only ledger (time, ±points, reason tag, linked order, note, staff), 調整積分, 編輯資料 | default |
| `1554:76230` | DESIGN · Admin Mobile · 會員 · 詳情 | 390×844 | Member detail: balance header, contact note, notes, append-only ledger (time, ±points, reason tag, linked order, note, staff), 調整積分, 編輯資料 | default |
| `1554:79326` | DESIGN · Admin Desktop · 會員 · 詳情 · 唯讀 | 1440×900 | Has customers:read but not customers:points: no adjust/edit actions | read-only permission |
| `1554:79430` | DESIGN · Admin Mobile · 會員 · 詳情 · 唯讀 | 390×844 | Has customers:read but not customers:points: no adjust/edit actions | read-only permission |
| `1554:76283` | DESIGN · Admin Desktop · 會員 · 詳情 · 尚無紀錄 | 1440×900 | New member, balance 0, empty ledger | empty |
| `1554:76360` | DESIGN · Admin Mobile · 會員 · 詳情 · 尚無紀錄 | 390×844 | New member, balance 0, empty ledger | empty |
| `1554:76898` | DESIGN · Admin Desktop · 會員 · 詳情 · 讀取失敗 | 1440×900 | Read failed, 重新讀取 | error |
| `1554:76967` | DESIGN · Admin Mobile · 會員 · 詳情 · 讀取失敗 | 390×844 | Read failed, 重新讀取 | error |
| `1554:76795` | DESIGN · Admin Desktop · 會員 · 詳情 · 載入中 | 1440×900 | Skeleton | loading |
| `1554:76864` | DESIGN · Admin Mobile · 會員 · 詳情 · 載入中 | 390×844 | Skeleton | loading |
| `1554:77001` | DESIGN · Admin Desktop · 會員 · 調整積分 · 增加 | 1440×900 | 調整積分 dialog/sheet: add/deduct toggle, integer amount, required note, live balance preview | add, filled |
| `1554:77136` | DESIGN · Admin Mobile · 會員 · 調整積分 · 增加 | 390×844 | 調整積分 dialog/sheet: add/deduct toggle, integer amount, required note, live balance preview | add, filled |
| `1554:77425` | DESIGN · Admin Desktop · 會員 · 調整積分 · 扣除超過餘額 | 1440×900 | Deduct > balance: inline error, 下一步 disabled | blocked (would go negative) |
| `1554:77559` | DESIGN · Admin Mobile · 會員 · 調整積分 · 扣除超過餘額 | 390×844 | Deduct > balance: inline error, 下一步 disabled | blocked (would go negative) |
| `1554:78197` | DESIGN · Admin Desktop · 會員 · 調整積分 · 未記入 | 1440×900 | Server refused (balance changed meanwhile, 409 points_insufficient): nothing written | error (definite refusal) |
| `1554:78317` | DESIGN · Admin Mobile · 會員 · 調整積分 · 未記入 | 390×844 | Server refused (balance changed meanwhile, 409 points_insufficient): nothing written | error (definite refusal) |
| `1554:77636` | DESIGN · Admin Desktop · 會員 · 調整積分 · 確認 | 1440×900 | Confirm step: member, −30, note, 120 → 90, append-only warning | confirm |
| `1554:77759` | DESIGN · Admin Mobile · 會員 · 調整積分 · 確認 | 390×844 | Confirm step: member, −30, note, 120 → 90, append-only warning | confirm |
| `1554:78014` | DESIGN · Admin Desktop · 會員 · 調整積分 · 結果待確認 | 1440×900 | Answer lost: 重新讀取 only, never auto-resend | UNKNOWN |
| `1554:78134` | DESIGN · Admin Mobile · 會員 · 調整積分 · 結果待確認 | 390×844 | Answer lost: 重新讀取 only, never auto-resend | UNKNOWN |
| `1554:77214` | DESIGN · Admin Desktop · 會員 · 調整積分 · 說明未填 | 1440×900 | Required note missing | validation error |
| `1554:77348` | DESIGN · Admin Mobile · 會員 · 調整積分 · 說明未填 | 390×844 | Required note missing | validation error |
| `1554:77825` | DESIGN · Admin Desktop · 會員 · 調整積分 · 送出中 | 1440×900 | Submitting; both actions disabled | loading |
| `1554:77948` | DESIGN · Admin Mobile · 會員 · 調整積分 · 送出中 | 390×844 | Submitting; both actions disabled | loading |
| `1554:79184` | DESIGN · Admin Mobile · 會員 · 選單 | 390×844 | Mobile menu with 顧客 and 會員 (current) added | nav |
Coordinates inside the section: row 0 手工單 (A1–A10), row 1 member list (B), row 2 member detail (C), row 3 調整積分 (D), row 4 order detail + 扣回 (E), row 5 mobile menu (F); each column = desktop at x, mobile at x+1500.

## Navigation decision

**會員 is a sub-item directly under 顧客 in the Shell A sidebar** (same pattern as 手工單 under 訂單與出貨: 18px icon, `color/subtle` highlight when current, parent 顧客 shown as Active). On mobile it is added to the 選單 list after 顧客.

Why:
1. The sidebar groups by *who/what the merchant manages*: 會員 are people records, so they belong with 顧客, not with 訂單 (points are only written from a 手工單, but they are read and adjusted on the member).
2. The top level keeps its 10 Nav Item variants; no new `Nav Item` variant (no component change, no bakref).
3. A separate item under 顧客 (not a tab inside the 顧客 page) keeps the two data sets apart. The contract (LP1/LP2) says members are a separate table with no matching to customers, and every 會員 screen says so in one line: 「會員資料與積分都由你手動維護，不會自動比對顧客或訂單買家。」
4. Breadcrumb 顧客 / 會員 / <name>; from a manual order the member is reached via 「查看會員紀錄」.

Rejected: a top-level 會員積分 item (adds an 11th nav item for a record-only feature); under 訂單 next to 手工單 (puts a people list under orders; the member detail is not an order view); a tab inside 顧客 (implies a link between customer and member that does not exist).

## Copy (zh-TW in frames · zh-CN · en)

| Key | zh-TW | zh-CN | en |
|---|---|---|---|
| nav.members | 會員 | 会员 | Members |
| mo.section | 會員與積分（選填） | 会员与积分（选填） | Member & points (optional) |
| mo.member.label | 會員 | 会员 | Member |
| mo.member.placeholder | 搜尋會員名稱或聯絡備註 | 搜索会员名称或联系备注 | Search member name or contact note |
| mo.member.help | 不選也能建立訂單。會員只用來記錄積分，不會自動比對買家。 | 不选也能创建订单。会员只用来记录积分，不会自动匹配买家。 | You can create the order without a member. Members only keep points and are never matched to buyers automatically. |
| mo.member.create_option | ＋ 建立新會員「{q}」 | ＋ 新建会员「{q}」 | + Create member "{q}" |
| mo.member.change / remove | 更換 / 移除 | 更换 / 移除 | Change / Remove |
| mo.member.selected | {name} · 目前 {n} 點 | {name} · 当前 {n} 点 | {name} · {n} pts now |
| mo.points.label | 本單贈送積分 | 本单赠送积分 | Points for this order |
| mo.points.placeholder | 例如 50 | 例如 50 | e.g. 50 |
| mo.points.unit | 點 | 点 | pts |
| mo.points.help | 由你自行填寫，系統不自動計算；不填就不記積分。 | 由你自行填写，系统不自动计算；不填就不记积分。 | You enter this yourself; nothing is calculated. Leave it empty to give no points. |
| mo.points.err.no_member | 請先選擇會員，或清空本單贈送積分。 | 请先选择会员，或清空本单赠送积分。 | Choose a member first, or clear the points. |
| mo.member.err.required_for_points | 要記積分，請先選擇會員。 | 要记积分，请先选择会员。 | Choose a member to record points. |
| mo.points.err.integer | 請輸入 1 以上的整數，或留空。 | 请输入 1 以上的整数，或留空。 | Enter a whole number of 1 or more, or leave it empty. |
| mo.inline.title | 建立新會員 | 新建会员 | New member |
| mo.inline.submit | 建立並選擇 | 创建并选择 | Create and select |
| mo.review.member | 會員：{name}（目前 {n} 點） | 会员：{name}（当前 {n} 点） | Member: {name} ({n} pts now) |
| mo.review.award | 本單贈送：+{n} 點，建立訂單時一併記入 | 本单赠送：+{n} 点，创建订单时一并记入 | Points: +{n}, recorded when the order is created |
| mo.success.award | 已記入 +{n} 點 · 餘額 {b} 點 | 已记入 +{n} 点 · 余额 {b} 点 | +{n} pts recorded · balance {b} pts |
| mo.success.link | 查看會員紀錄 | 查看会员记录 | View member history |
| mo.not_awarded.tag | 積分未贈送 | 积分未赠送 | Points not given |
| mo.not_awarded.body (member_erased) | 訂單已建立，但 +{n} 點沒有記入：這位會員已被刪除資料。 | 订单已创建，但 +{n} 点没有记入：该会员的资料已被删除。 | The order was created, but +{n} pts were not recorded: this member's data was erased. |
| mo.not_awarded.hint | 需要的話，可以到會員頁用「調整積分」補記，並關聯這張訂單。 | 需要的话，可以到会员页用「调整积分」补记，并关联这张订单。 | If needed, add them from the member page with "Adjust points" and link this order. |
| list.title | 會員 | 会员 | Members |
| list.search | 搜尋名稱或聯絡備註 | 搜索名称或联系备注 | Search name or contact note |
| list.note | 會員資料與積分都由你手動維護，不會自動比對顧客或訂單買家。 | 会员资料与积分都由你手动维护，不会自动匹配顾客或订单买家。 | You keep member records and points by hand; they are never matched to customers or buyers. |
| list.col | 名稱 / 聯絡備註 / 積分餘額 / 最後異動 / 操作 | 名称 / 联系备注 / 积分余额 / 最后变动 / 操作 | Name / Contact note / Balance / Last change / Action |
| list.view | 查看 | 查看 | View |
| list.prev / next | 上一頁 / 下一頁 | 上一页 / 下一页 | Previous / Next |
| list.create | 新增會員 | 新增会员 | Add member |
| list.no_match | 找不到「{q}」 · 請換個名稱或聯絡備註再搜尋，或建立新會員。 · 清除搜尋 | 找不到「{q}」 · 请换个名称或联系备注再搜索，或新建会员。 · 清除搜索 | No match for "{q}" · Try another name or contact note, or add a member. · Clear search |
| list.empty | 開始使用 · 尚未建立會員 · 建立會員後，就能在手工單記錄贈送積分，並在這裡查看每位會員的積分紀錄。 · 會員由你手動建立，系統不會自動從訂單產生會員。 | 开始使用 · 尚未创建会员 · 创建会员后，就能在手工单记录赠送积分，并在这里查看每位会员的积分记录。 · 会员由你手动创建，系统不会自动从订单生成会员。 | Get started · No members yet · Add a member to record points on manual orders and see each member's history here. · Members are created by you; orders never create them. |
| list.loading | 正在讀取會員… | 正在读取会员… | Loading members… |
| read.error | 連線問題 · 暫時無法讀取會員 · 請檢查網路連線後重新讀取。 · 只會重新讀取資料，不會重送任何操作。 | 连接问题 · 暂时无法读取会员 · 请检查网络连接后重新读取。 · 只会重新读取数据，不会重发任何操作。 | Connection problem · Can't load members right now · Check your connection and reload. · This only reloads data; it never resends an action. |
| read.retry | 重新讀取 | 重新读取 | Reload |
| list.forbidden | 存取限制 · 目前無法開啟此頁 · 你的帳號沒有查看會員的權限，請聯絡店長開通。 · 不顯示任何會員資料。 | 访问限制 · 目前无法打开此页 · 你的账号没有查看会员的权限，请联系店长开通。 · 不显示任何会员资料。 | Access restricted · You can't open this page · Your account can't view members. Ask the store owner for access. · No member data is shown. |
| form.name | 顯示名稱（必填） · 例如 王小美 · 請填寫顯示名稱。 | 显示名称（必填） · 例如 王小美 · 请填写显示名称。 | Display name (required) · e.g. Amy Wang · Enter a display name. |
| form.contact | 聯絡備註（選填） · 例如 LINE ID、IG 帳號或電話 · 只給你自己辨識用，不會傳給買家，也不會自動比對。 | 联系备注（选填） · 例如 LINE ID、IG 账号或电话 · 只供你自己辨识，不会发给买家，也不会自动匹配。 | Contact note (optional) · e.g. LINE ID, IG handle or phone · For your own reference only; never sent to buyers or matched. |
| form.notes | 備註（選填） · 例如 喜好、尺寸 | 备注（选填） · 例如 喜好、尺码 | Notes (optional) · e.g. preferences, size |
| form.create / save / cancel | 建立會員 / 儲存 / 取消 | 创建会员 / 保存 / 取消 | Add member / Save / Cancel |
| edit.title | 編輯會員資料 | 编辑会员资料 | Edit member |
| edit.no_points | 積分不能在這裡修改，請用「調整積分」。 | 积分不能在这里修改，请用「调整积分」。 | Points can't be changed here. Use "Adjust points". |
| detail.balance | 積分餘額 · {n} 點 · 最後異動 {t} | 积分余额 · {n} 点 · 最后变动 {t} | Balance · {n} pts · Last change {t} |
| detail.contact / notes | 聯絡備註：{v} / 備註：{v} | 联系备注：{v} / 备注：{v} | Contact note: {v} / Notes: {v} |
| detail.adjust / edit | 調整積分 / 編輯資料 | 调整积分 / 编辑资料 | Adjust points / Edit details |
| detail.readonly | 你的帳號可以查看會員，但不能調整積分或編輯資料。需要權限請聯絡店長。 | 你的账号可以查看会员，但不能调整积分或编辑资料。需要权限请联系店长。 | You can view members but can't adjust points or edit details. Ask the store owner for access. |
| ledger.title | 積分紀錄 | 积分记录 | Points history |
| ledger.note | 紀錄只能新增，不能修改或刪除；記錯時請用手動調整更正。 | 记录只能新增，不能修改或删除；记错时请用手动调整更正。 | Entries can only be added, never edited or deleted. Fix a mistake with a manual adjustment. |
| ledger.col | 時間 / 積分 / 原因 / 關聯訂單 / 說明 / 經手人 | 时间 / 积分 / 原因 / 关联订单 / 说明 / 经手人 | Time / Points / Reason / Order / Note / Staff |
| ledger.reason | 本單贈送 / 手動調整 / 沖正 | 本单赠送 / 手动调整 / 冲正 | Order award / Manual adjustment / Reversal |
| ledger.empty | 尚無積分紀錄。可以用「調整積分」記入第一筆，或在手工單贈送。 | 暂无积分记录。可以用「调整积分」记入第一笔，或在手工单赠送。 | No points yet. Use "Adjust points" or give points on a manual order. |
| adj.title | 調整積分 · 目前餘額 {n} 點 | 调整积分 · 当前余额 {n} 点 | Adjust points · Balance {n} pts |
| adj.toggle | 增加積分 / 扣除積分 | 增加积分 / 扣除积分 | Add points / Deduct points |
| adj.amount | 點數 · 請輸入 1 以上的整數。 | 点数 · 请输入 1 以上的整数。 | Points · Enter a whole number of 1 or more. |
| adj.note | 說明（必填） · 例如 活動贈送、兌換禮物 · 請填寫調整說明。 | 说明（必填） · 例如 活动赠送、兑换礼物 · 请填写调整说明。 | Note (required) · e.g. promotion, gift redeemed · Enter a note. |
| adj.preview | 調整後餘額：{a} → {b} 點 | 调整后余额：{a} → {b} 点 | Balance after: {a} → {b} pts |
| adj.err.negative | 扣除後餘額會小於 0。目前餘額 {n} 點，最多可扣 {n} 點。 | 扣除后余额会小于 0。当前余额 {n} 点，最多可扣 {n} 点。 | This would make the balance negative. Balance is {n} pts; you can deduct up to {n}. |
| adj.next | 下一步 | 下一步 | Next |
| adj.confirm.title | 確認扣除積分 / 確認增加積分 | 确认扣除积分 / 确认增加积分 | Confirm deduction / Confirm addition |
| adj.confirm.lines | 會員：{name} · 扣除：−{n} 點 · 說明：{note} · 餘額：{a} → {b} 點 | 会员：{name} · 扣除：−{n} 点 · 说明：{note} · 余额：{a} → {b} 点 | Member: {name} · Deduct: −{n} pts · Note: {note} · Balance: {a} → {b} pts |
| adj.confirm.warn | 送出後會新增一筆紀錄，不能修改或刪除。 | 提交后会新增一笔记录，不能修改或删除。 | This adds an entry that can't be edited or deleted. |
| adj.confirm.cta | 確認扣除 / 返回修改 | 确认扣除 / 返回修改 | Deduct / Back |
| adj.submitting | 送出中… · 正在送出，請勿關閉或重新整理。 | 提交中… · 正在提交，请勿关闭或刷新。 | Submitting… · Don't close or refresh. |
| write.unknown | 結果待確認 · 待確認 · 網路回應中斷，這筆調整（{delta} 點）可能已經記入。請先重新讀取餘額與紀錄，勿再次送出。 · 重新讀取只會查詢，不會重送。 | 结果待确认 · 待确认 · 网络响应中断，这笔调整（{delta} 点）可能已经记入。请先重新读取余额与记录，勿再次提交。 · 重新读取只会查询，不会重发。 | Result unconfirmed · Pending · The connection dropped; this adjustment ({delta} pts) may already be recorded. Reload the balance and history first; don't submit again. · Reloading only checks; it never resends. |
| write.unknown.create | 建立結果未確認 · 網路回應中斷，會員可能已經建立。請先重新讀取，勿再次建立，以免出現兩筆同名會員。 | 创建结果未确认 · 网络响应中断，会员可能已经创建。请先重新读取，勿再次创建，以免出现两个同名会员。 | Creation unconfirmed · The connection dropped; the member may already exist. Reload first and don't create again, or you may get two members with the same name. |
| write.unknown.edit | 儲存結果未確認 · 網路回應中斷，變更可能已經儲存。請先重新讀取，確認後再決定是否修改，勿直接再次儲存。 · 關閉 | 保存结果未确认 · 网络响应中断，更改可能已经保存。请先重新读取，确认后再决定是否修改，勿直接再次保存。 · 关闭 | Save unconfirmed · The connection dropped; your changes may already be saved. Reload and check before editing again; don't just save again. · Close |
| adj.refused | 未記入 · 餘額已變動（目前 {b} 點），扣除 {n} 點會小於 0，這筆調整沒有記入。 · 請返回修改點數後再確認。 | 未记入 · 余额已变动（当前 {b} 点），扣除 {n} 点会小于 0，这笔调整没有记入。 · 请返回修改点数后再确认。 | Not recorded · The balance changed (now {b} pts); deducting {n} would go below 0, so nothing was recorded. · Go back and change the amount. |
| ord.points.title | 會員與積分 · 會員：{name} | 会员与积分 · 会员：{name} | Member & points · Member: {name} |
| ord.points.award | 本單贈送 +{n} 點 · {time} · {staff} | 本单赠送 +{n} 点 · {time} · {staff} | Order award +{n} pts · {time} · {staff} |
| ord.points.deductible | 可扣回 {n} 點 · 會員目前餘額 {b} 點 | 可扣回 {n} 点 · 会员当前余额 {b} 点 | Up to {n} pts can be taken back · member balance {b} pts |
| ord.reverse.cta | 扣回本單積分 | 扣回本单积分 | Take back this order's points |
| ord.reverse.amount | 扣回點數 · 最多可扣回 {n} 點（本單贈送 {a} 點，已扣回 {r} 點）。 | 扣回点数 · 最多可扣回 {n} 点（本单赠送 {a} 点，已扣回 {r} 点）。 | Points to take back · Up to {n} pts (awarded {a}, already taken back {r}). |
| ord.reverse.preview | {name}的餘額：{a} → {b} 點 | {name}的余额：{a} → {b} 点 | {name}'s balance: {a} → {b} pts |
| ord.reverse.warn | 送出後會新增一筆「沖正」紀錄，不能修改或刪除。 | 提交后会新增一笔「冲正」记录，不能修改或删除。 | This adds a "Reversal" entry that can't be edited or deleted. |
| ord.reverse.confirm | 確認扣回 | 确认扣回 | Take back |
| ord.reverse.err.exceeds | 不能超過本單可扣回的 {n} 點。 | 不能超过本单可扣回的 {n} 点。 | Can't be more than the {n} pts left from this order. |
| ord.reverse.err.balance | {name}目前只剩 {b} 點，扣回 {n} 點會讓餘額小於 0。最多可扣回 {b} 點。 | {name}当前只剩 {b} 点，扣回 {n} 点会让余额小于 0。最多可扣回 {b} 点。 | {name} has only {b} pts left; taking back {n} would go below 0. You can take back up to {b}. |
| ord.reversed | 已全數扣回 · 已扣回 −{n} 點 · {time} · {staff} · {note} · 可扣回 0 點 | 已全数扣回 · 已扣回 −{n} 点 · {time} · {staff} · {note} · 可扣回 0 点 | Fully taken back · −{n} pts · {time} · {staff} · {note} · 0 pts left to take back |
| ord.reverse.unknown | 網路回應中斷，扣回 {n} 點可能已經記入。請先重新讀取本單與會員積分，勿再次送出。 | 网络响应中断，扣回 {n} 点可能已经记入。请先重新读取本单与会员积分，勿再次提交。 | The connection dropped; taking back {n} pts may already be recorded. Reload the order and member points first; don't submit again. |

## Self-QA (visual + structural)

Method: every frame exported with `get_screenshot` (scale 1) and inspected in montages and at full size; structural probe `figma-src/qa.js` checks per frame: content height vs the clipped task viewport, dialog/sheet inside the frame, text < 12px, empty text, action/segment height < 44px, and WCAG contrast of the token pairs used. Final probe: **0 issues over 77 frames**.

| # | Finding | Severity | Fix |
|---|---|---|---|
| F1 | Scrims rendered fully opaque navy (paint opacity was dropped when the variable was bound), hiding the page behind every dialog/sheet | P1 | scrim = `color/ink` fill at 100% on a layer with opacity 45%; patched on all 30 dialog frames and in the generator |
| F2 | Active 會員 nav item rendered as a black bar with unreadable text: a bound paint was created with a black raw color | P1 | paints now carry the variable's resolved value; all frames rebuilt |
| F3 | `color/muted` (3:5) on `color/subtle` = **4.30:1** (< 4.5) for placeholders, the 點 suffix and disabled labels | P2 | text on `color/subtle` uses `color/muted` of collection 13:305 (5.38:1). Remaining pairs: muted/surface 4.72, teal/surface 5.25, teal/subtle 4.79, white/teal 5.25, danger/surface 6.57, ink/subtle 13.45 |
| F4 | Mobile member list: the 4th row and the pagination were cut off by the 582px viewport | P2 | mobile shows 3 rows per page in the frame (the real page scrolls) |
| F5 | Desktop 建立新會員 inline form overflowed the viewport by 2px | P2 | form gap 12 → 10 |
| F6 | The 「繁中」 label in the inherited Shell A header sits at y=0 and is clipped at the top (the signed shell has the same defect) | P2 | centered vertically in all 38 new desktop frames; **the signed frames still have it** (not touched: out of lane) |
| F7 | Desktop list header had an empty last-column cell | P3 | header 「操作」 |
| F8 | 「未記入」 example said −90 against a −30 confirm step | P3 | example made consistent (balance 20, deduct 30) |
| F9 | Signed disabled Primary = teal at 50% opacity with white label (low contrast, e.g. 1470:166598) | note | not copied: disabled here = `color/subtle` fill + 13:305 muted label |
| F10 | Signed mobile 選單 (1470:165651) lists only 4 destinations (no 總覽, 直播, 顧客 …) | note | out of lane; the new menu only adds 顧客 + 會員 |

Not done: no prototype reactions were wired between the new frames (cloned reactions were cleared so no button points to the wrong frame); no Code Connect.

## Integrator rulings 2026-11-05 (the contract is the authority where it differs from the brief)

- U1 → contract LP10/LP19: no 「沖正」 reason. 「扣回本單積分」 opens a prefilled manual deduct: reason 手動調整, linked to the order, amount editable (prefilled with the award), **no cap**, never below 0 (the 餘額不足 state stays). The ledger row shows 手動調整 + the order link. The order page lists 「本單相關積分紀錄」 (`GET members/entries?order_id=`) instead of 「可扣回」. Removed: the over-award frame and the copy keys `ord.reverse.err.exceeds`, `ord.reverse.amount`, `ord.reversed`, `ledger.reason=沖正`.
- U2 → a creator without `customers:points` sees no 會員與積分 card (A11).
- U3 → ledger 「載入更多紀錄」, 50 per page, plus its read error (C8, C9).
- U4 → erase confirm, erase UNKNOWN, erased state (C10–C12); export ready / preparing / downloaded / failed (C13–C16); edit conflict (409 `conflict`, stale `expected_version`) → 重新讀取 (C17). Header gets 匯出資料 / 刪除資料 links (`customers:privacy`).
- U5 accepted · U6 → A12 · U7 accepted (會員).

New and changed copy (zh-TW · zh-CN · en):

| Key | zh-TW | zh-CN | en |
|---|---|---|---|
| mo.prefill.help | 已從會員帶入，可以修改；只改這張訂單，不會改會員資料。 | 已从会员带入，可以修改；只改这张订单，不会改会员资料。 | Filled in from the member. You can edit it; it only changes this order, not the member. |
| ord.entries.label | 本單相關積分紀錄 | 本单相关积分记录 | Points entries for this order |
| ord.entries.deduct | 手動調整 −{n} 點 · {time} · {staff} · {note} | 手动调整 −{n} 点 · {time} · {staff} · {note} | Manual adjustment −{n} pts · {time} · {staff} · {note} |
| ord.entries.total | 本單相關合計 {n} 點 · 會員目前餘額 {b} 點 | 本单相关合计 {n} 点 · 会员当前余额 {b} 点 | Net for this order {n} pts · member balance {b} pts |
| ord.reverse.reason | 原因：手動調整 · 關聯訂單 #{order} | 原因：手动调整 · 关联订单 #{order} | Reason: manual adjustment · order #{order} |
| ord.reverse.amount | 扣除點數 · 已帶入本單贈送的 {n} 點，可以修改。 | 扣除点数 · 已带入本单赠送的 {n} 点，可以修改。 | Points to deduct · Prefilled with this order's {n} pts; you can change it. |
| ord.reverse.warn | 送出後會新增一筆「手動調整」紀錄並關聯這張訂單，不能修改或刪除。 | 提交后会新增一笔「手动调整」记录并关联这张订单，不能修改或删除。 | This adds a "Manual adjustment" entry linked to this order; it can't be edited or deleted. |
| ord.reverse.confirm | 確認扣除 | 确认扣除 | Deduct |
| ord.reverse.err.balance | {name}目前只剩 {b} 點，扣除 {n} 點會讓餘額小於 0。最多可扣 {b} 點。 | {name}当前只剩 {b} 点，扣除 {n} 点会让余额小于 0。最多可扣 {b} 点。 | {name} has only {b} pts; deducting {n} would go below 0. You can deduct up to {b}. |
| ord.reversed.tag | 已扣回 {n} 點 | 已扣回 {n} 点 | {n} pts taken back |
| ledger.more | 已顯示最新 50 筆，還有更早的紀錄。 · 載入更多紀錄 | 已显示最新 50 笔，还有更早的记录。 · 加载更多记录 | Showing the latest 50 entries; there are older ones. · Load more |
| ledger.more.error | 暫時無法載入更早的紀錄。請檢查連線後重新讀取。 | 暂时无法加载更早的记录。请检查连接后重新读取。 | Couldn't load older entries. Check your connection and reload. |
| privacy.links | 匯出資料 / 刪除資料 | 导出资料 / 删除资料 | Export data / Erase data |
| erase.title | 刪除{name}的會員資料？ | 删除{name}的会员资料？ | Erase {name}'s member data? |
| erase.deletes | 會刪除：顯示名稱、聯絡備註、備註，以及積分紀錄裡的說明文字。 | 会删除：显示名称、联系备注、备注，以及积分记录里的说明文字。 | Deleted: display name, contact note, notes, and the notes on points entries. |
| erase.keeps | 會保留：每筆積分的點數、原因、關聯訂單、經手人與時間，餘額不變。 | 会保留：每笔积分的点数、原因、关联订单、经手人与时间，余额不变。 | Kept: each entry's points, reason, linked order, staff and time; the balance stays. |
| erase.orders | 不會改變：已建立訂單上的買家姓名。例如 #M1031 已帶入的「王小美」會留在那張訂單上。 | 不会改变：已创建订单上的买家姓名。例如 #M1031 已带入的「王小美」会留在那张订单上。 | Not changed: the buyer name already on past orders. For example, "Amy Wang" stays on order #M1031. |
| erase.final | 刪除後無法復原，這位會員也不能再記積分。 · 刪除資料 | 删除后无法恢复，这位会员也不能再记积分。 · 删除资料 | This can't be undone, and no more points can be recorded for this member. · Erase data |
| erase.unknown | 刪除結果未確認 · 網路回應中斷，會員資料可能已經刪除。請先重新讀取，勿再次刪除。 | 删除结果未确认 · 网络响应中断，会员资料可能已经删除。请先重新读取，勿再次删除。 | Erase unconfirmed · The connection dropped; the data may already be erased. Reload first; don't erase again. |
| erase.done | 已刪除的會員 · 已刪除資料 · 名稱、聯絡備註、備註與積分說明已刪除；點數與訂單上的買家姓名保留。這位會員不能再記積分。 · （已刪除） | 已删除的会员 · 已删除资料 · 名称、联系备注、备注与积分说明已删除；点数与订单上的买家姓名保留。该会员不能再记积分。 · （已删除） | Erased member · Data erased · Name, contact note, notes and entry notes were erased; points and buyer names on orders are kept. No more points can be recorded. · (erased) |
| export.title | 匯出會員資料 · 下載{name}的會員資料與全部積分紀錄（JSON 檔）。檔案不含員工姓名。 | 导出会员资料 · 下载{name}的会员资料与全部积分记录（JSON 文件）。文件不含员工姓名。 | Export member data · Download {name}'s record and full points history (JSON). Staff names are not included. |
| export.ready | 檔案可能含有個人資料，請妥善保存。 · 下載 | 文件可能含有个人资料，请妥善保存。 · 下载 | The file may contain personal data; keep it safe. · Download |
| export.busy | 正在準備檔案… · 準備中… | 正在准备文件… · 准备中… | Preparing the file… · Preparing… |
| export.done | 已下載 · 已下載 {file}（{n} 筆紀錄）。 · 完成 | 已下载 · 已下载 {file}（{n} 笔记录）。 · 完成 | Downloaded · Downloaded {file} ({n} entries). · Done |
| export.fail | 下載沒有完成。可以再試一次，不會重複記錄匯出。 · 再試一次 | 下载没有完成。可以再试一次，不会重复记录导出。 · 再试一次 | The download didn't finish. You can try again; the export won't be logged twice. · Try again |
| edit.conflict | 這位會員剛被其他人修改，你的變更沒有儲存。請重新讀取最新資料後再修改。 | 这位会员刚被其他人修改，你的更改没有保存。请重新读取最新资料后再修改。 | Someone else just changed this member, so your changes were not saved. Reload the latest data before editing. |

Export 「再試一次」 resends the same Idempotency-Key, which the contract allows: a replay returns a fresh export and writes no second audit row. Every other write stays 重新讀取-only.

## For the D5 Figma sweep (signed-frame defects, outside this lane)

1. The 「繁中」 label in Shell A / header sits at y=0 and is clipped at the top on every signed desktop frame (e.g. `1470:166550`). Fixed only in this section's frames.
2. Disabled Primary = teal at 50% opacity with a white label: low contrast (e.g. `1470:166598`, `1470:166624`).
3. The mobile 選單 `1470:165651` lists only 4 destinations (訂單, 手工單, 商品與庫存, 訊息中心).
4. Placeholder 「請填寫」 in signed fields uses `color/ink`, so it reads as a value; and `color/muted` (3:5) on `color/subtle` is 4.30:1.

## Open questions (UNKNOWN) — round 1, all ruled on 2026-11-05 (see above)

- **U1 · Reversal rule (brief vs contract).** The brief says a reversal of an order's award is its own reason 「沖正」 and can't exceed the award. Contract draft `e0d3e968` LP9 (integrator ruling P2-7, 2026-11-05) **withdrew** that: 扣回本單積分 = a prefilled `manual_adjust` with `order_id`, editable, **no cap**; reasons are only `manual_order_award` / `manual_adjust`. The frames follow the brief (沖正 tag, frame 「扣回本單積分 · 超過贈送」). If the contract wins: drop that frame and that error, show the reversal as 手動調整 + order link, and compute 「可扣回」 from the entries of that order or remove the line. Needs the owner's or the integrator's word.
- **U2 · Who may award/adjust.** Contract LP13 adds `customers:points` (owner/admin). The frames cover no read right (列表 · 無權限) and read-only (詳情 · 唯讀). Not drawn: a manual-order creator without `customers:points` — proposal: hide the 會員與積分 card. Needs confirmation.
- **U3 · Ledger paging.** Contract: 50 entries per page with `ledger_cursor`. The detail frames show ≤ 3 entries and no ledger pager. Proposal: 「載入更多」 Secondary button under the table. Not drawn.
- **U4 · Erase / export member** (`customers:privacy`, LP11/LP12) and the edit version conflict (`PATCH expected_version` → 409) have no frames. Is that in UI v1?
- **U5 · Award vs order UNKNOWN.** The order-create UNKNOWN reuses the signed 手工單 · unknown frame unchanged; the award outcome comes back with the order receipt (LP8), so no separate award UNKNOWN is designed.
- **U6 · Prefill order name from member** (LP17 「订购人的名字我用客户的名字」): not drawn; proposal: choosing a member fills an empty 姓名 field once, editable.
- **U7 · Label 「會員」 vs 「會員積分」** in the nav: the owner's words use 会员 / 积分; the frames use 會員.
