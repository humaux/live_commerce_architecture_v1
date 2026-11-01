# PARITY-BUYER-AFTER frame copy (signed Figma)

- Source: Figma file key `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829` (signed frames)
- Date: 2026-11-01
- Method: read-only export via use_figma; texts are verbatim node.characters (no node was written or changed)
- Frame ids: FIGMA-INVENTORY.md Appendix A, buckets `U:b-order` (66), `U:b-privacy` (20), `U:b-claim` (6); context row §3 C8 PARITY-BUYER-AFTER
- Result: 92 of 92 frames found, 0 NOT_FOUND

## Reading the lines

- `- text:` one visible TEXT node, sorted by absolute y then x (rows within about 6px are one row). In two-column desktop frames the columns therefore interleave.
- `\n` inside a text line is a literal line break inside that one node.
- `- control:` an INSTANCE/COMPONENT node (or a FRAME named like an input/field/button): `<layer name> {non-default variant props} | text: <visible texts inside it, joined by " / ">`. `State=Default` is omitted. The control's inner texts also appear as their own `- text:` lines.
- `- chrome:` the store header and footer are identical across many frames, so they are listed once below (Chrome D1, D2, D3 for desktop, M1, M2, M3 for mobile, NONE) and each frame points to its set. Chrome texts are not repeated per frame.
- Hidden TEXT nodes (visible=false on the node or an ancestor) are skipped; the count is shown on the chrome line when non-zero.
- Prototype scaffolding: controls named `Prototype switch (not product UI)` and the `原型：...` buttons (layer name often `Action / 原型：模擬格式錯誤`) are labelled in the frames themselves as 原型示範（非產品畫面）. They are listed because they are in the frame, but they are not product UI.
- Layer names and visible text can differ (for example layer `Action / ‹ 我的訂單` shows text `‹ 查詢訂單`). Visible text is the copy; the layer name is only the control's Figma name.

## Shared chrome sets

### Chrome D1 (desktop header + footer, footer link layer named 隱私政策)
- text: 大夢甄選
- text: 所有商品
- text: 精選系列
- text: 直播回放
- text: 查詢訂單
- text: 大夢甄選 · 示範商店
- control: Action / 隱私政策 {Kind=Secondary} | text: 隱私與資料
- control: Action / 購物條款 {Kind=Secondary} | text: 購物條款
- control: Action / 資料刪除 {Kind=Secondary} | text: 資料刪除

### Chrome D2 (desktop header + footer, footer link layer named 隱私與資料)
- text: 大夢甄選
- text: 所有商品
- text: 精選系列
- text: 直播回放
- text: 查詢訂單
- text: 大夢甄選 · 示範商店
- control: Action / 隱私與資料 {Kind=Secondary} | text: 隱私與資料
- control: Action / 購物條款 {Kind=Secondary} | text: 購物條款
- control: Action / 資料刪除 {Kind=Secondary} | text: 資料刪除

### Chrome D3 (desktop header only, no footer)
- text: 大夢甄選
- text: 所有商品
- text: 精選系列
- text: 直播回放
- text: 查詢訂單

### Chrome M1 (mobile header brand + footer, footer link layer named 隱私政策)
- text: 大夢甄選
- text: 大夢甄選 · 示範商店
- control: Action / 隱私政策 {Kind=Secondary} | text: 隱私與資料
- control: Action / 購物條款 {Kind=Secondary} | text: 購物條款
- control: Action / 資料刪除 {Kind=Secondary} | text: 資料刪除

### Chrome M2 (mobile header brand + footer, footer link layer named 隱私與資料)
- text: 大夢甄選
- text: 大夢甄選 · 示範商店
- control: Action / 隱私與資料 {Kind=Secondary} | text: 隱私與資料
- control: Action / 購物條款 {Kind=Secondary} | text: 購物條款
- control: Action / 資料刪除 {Kind=Secondary} | text: 資料刪除

### Chrome M3 (mobile header brand only, no footer)
- text: 大夢甄選

### Chrome NONE (no Header / Store footer node in the frame)

---

## U:b-order (66 frames exported, 0 NOT_FOUND)

### 4:9963 Buyer Desktop · 訂單 LC-10482 · SHOPLINE (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: K8 entry / 我的訂單 {Kind=Secondary} | text: 我的訂單
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 待付款
- text: 商品明細
- text: 完成付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 商品 NT$1,280 + 運費 NT$0
- control: Action / 前往安全付款頁 {Kind=Primary} | text: 前往安全付款頁
- text: 訂單合計
- text: NT$1,280
- control: Action / 重新查詢付款狀態 {Kind=Secondary} | text: 重新查詢付款狀態
- text: 收件與配送
- text: 若付款結果未確認，請先查詢這筆訂單，避免重複付款。
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 4:10049 Buyer Mobile · 訂單 LC-10482 · SHOPLINE (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 待付款
- text: 完成付款
- text: NT$1,280
- text: 商品 NT$1,280 + 運費 NT$0
- control: Action / 前往安全付款頁 {Kind=Primary} | text: 前往安全付款頁
- control: Action / 重新查詢付款狀態 {Kind=Secondary} | text: 重新查詢付款狀態
- text: 若付款結果未確認，請先查詢這筆訂單，避免重複付款。
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 25:943 Buyer Desktop · 訂單 LC-10482 · 門市取貨 (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 待付款
- text: 商品明細
- text: 完成付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 商品 NT$1,280 + 運費 NT$0
- text: NT$1,280
- control: Action / 前往安全付款頁 {Kind=Primary} | text: 前往安全付款頁
- text: 訂單合計
- control: Action / 重新查詢付款狀態 {Kind=Secondary} | text: 重新查詢付款狀態
- text: 收件與配送
- text: 若付款結果未確認，請先查詢這筆訂單，避免重複付款。
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 25:995 Buyer Mobile · 訂單 LC-10482 · 門市取貨 (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 待付款
- text: 完成付款
- text: NT$1,280
- text: 商品 NT$1,280 + 運費 NT$0
- control: Action / 前往安全付款頁 {Kind=Primary} | text: 前往安全付款頁
- control: Action / 重新查詢付款狀態 {Kind=Secondary} | text: 重新查詢付款狀態
- text: 若付款結果未確認，請先查詢這筆訂單，避免重複付款。
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 26:708 Buyer Desktop · 查詢訂單 · SHOPLINE (1440px)
- chrome: Chrome D1
- text: 查詢訂單
- text: 尋找你的訂單
- text: 請輸入這家商店的訂單編號及下單時填寫的聯絡資料。
- control: Field / 訂單編號 | text: 訂單編號 / LC-10482
- text: 訂單編號
- control: Input | text: LC-10482
- control: Field / 電子郵件或手機號碼 | text: 電子郵件或手機號碼 / buyer@example.invalid
- text: 電子郵件或手機號碼
- control: Input | text: buyer@example.invalid
- control: Action / 查詢訂單 {Kind=Primary} | text: 查詢訂單
- text: 找不到訂單時，請核對商店、訂單編號與聯絡資料。

### 26:746 Buyer Desktop · 訂單連結已失效 · SHOPLINE (1440px)
- chrome: Chrome D1
- text: 訂單連結已失效
- text: 請重新確認身分
- text: 這個連結無法再使用。你可以使用訂單編號查詢，或聯絡下單的商店。
- control: Action / 前往查詢訂單 {Kind=Primary} | text: 前往查詢訂單

### 26:773 Buyer Desktop · 申請售後 · SHOPLINE (1440px)
- chrome: Chrome D1
- control: Action / ‹ 訂單 B-10482 {Kind=Secondary} | text: ‹ 返回上一頁
- text: 申請售後
- text: 選擇需要協助的商品
- text: ☑ 焦糖托特包 · 焦糖 × 1
- control: Field / 申請原因 | text: 申請原因 / 請選擇原因
- text: 申請原因
- control: Input | text: 請選擇原因
- control: Field / 問題說明 | text: 問題說明 / 請描述收到的商品狀況
- text: 問題說明
- control: Input | text: 請描述收到的商品狀況
- control: Action / 新增照片 {Kind=Secondary} | text: 新增照片
- text: 送出後由商家確認；此步驟不代表已核准退貨或退款。
- control: Action / 確認並送出 {Kind=Primary} | text: 確認並送出

### 26:815 Buyer Desktop · 售後申請 R-1001 · SHOPLINE (1440px)
- chrome: Chrome D1
- text: 售後申請 R-1001
- text: 等待商家確認
- text: 申請內容
- text: 訂單 LC-10482\n焦糖托特包 · 焦糖 × 1\n原因：商品有瑕疵\n說明：提把縫線需要協助確認。
- text: 處理進度
- text: 10/25 20:10  已送出申請\n下一步：等待商家回覆。
- text: 退款是否完成，請以退款結果與原付款機構的入帳記錄為準。
- control: Action / 返回訂單 {Kind=Secondary} | text: 返回申請內容

### 26:995 Buyer Mobile · 查詢訂單 · SHOPLINE (390px)
- chrome: Chrome M1
- text: 查詢訂單
- text: 尋找你的訂單
- text: 請輸入這家商店的訂單編號及下單時填寫的聯絡資料。
- control: Field / 訂單編號 | text: 訂單編號 / LC-10482
- text: 訂單編號
- control: Input | text: LC-10482
- control: Field / 電子郵件或手機號碼 | text: 電子郵件或手機號碼 / buyer@example.invalid
- text: 電子郵件或手機號碼
- control: Input | text: buyer@example.invalid
- control: Action / 查詢訂單 {Kind=Primary} | text: 查詢訂單
- text: 找不到訂單時，請核對商店、訂單編號與聯絡資料。

### 26:1028 Buyer Mobile · 訂單連結已失效 · SHOPLINE (390px)
- chrome: Chrome M1
- text: 訂單連結已失效
- text: 請重新確認身分
- text: 這個連結無法再使用。你可以使用訂單編號查詢，或聯絡下單的商店。
- control: Action / 前往查詢訂單 {Kind=Primary} | text: 前往查詢訂單

### 26:1050 Buyer Mobile · 申請售後 · SHOPLINE (390px)
- chrome: Chrome M1
- control: Action / ‹ 訂單 B-10482 {Kind=Secondary} | text: ‹ 返回上一頁
- text: 申請售後
- text: 選擇需要協助的商品
- text: ☑ 焦糖托特包 · 焦糖 × 1
- control: Field / 申請原因 | text: 申請原因 / 請選擇原因
- text: 申請原因
- control: Input | text: 請選擇原因
- control: Field / 問題說明 | text: 問題說明 / 請描述收到的商品狀況
- text: 問題說明
- control: Input | text: 請描述收到的商品狀況
- control: Action / 新增照片 {Kind=Secondary} | text: 新增照片
- text: 送出後由商家確認；此步驟不代表已核准退貨或退款。
- control: Action / 確認並送出 {Kind=Primary} | text: 確認並送出

### 26:1087 Buyer Mobile · 售後申請 R-1001 · SHOPLINE (390px)
- chrome: Chrome M1
- text: 售後申請 R-1001
- text: 等待商家確認
- text: 申請內容
- text: 訂單 LC-10482\n焦糖托特包 · 焦糖 × 1\n原因：商品有瑕疵\n說明：提把縫線需要協助確認。
- text: 處理進度
- text: 10/25 20:10  已送出申請\n下一步：等待商家回覆。
- text: 退款是否完成，請以退款結果與原付款機構的入帳記錄為準。
- control: Action / 返回訂單 {Kind=Secondary} | text: 返回申請內容

### 29:1064 Buyer Desktop · 前往付款 · SHOPLINE (1440px)
- chrome: Chrome D1
- text: 前往付款
- text: 訂單 LC-10482 · NT$1,280
- text: 付款會在新分頁開啟，請保留此商店分頁。完成後回到這裡查詢結果。
- text: 如果付款結果尚未確認，請勿重新建立訂單或重複付款。
- control: Action / 查看付款結果 {Kind=Primary} | text: 查看付款結果

### 29:1095 Buyer Mobile · 前往付款 · SHOPLINE (390px)
- chrome: Chrome M1
- text: 前往付款
- text: 訂單 LC-10482 · NT$1,280
- text: 付款會在新分頁開啟，請保留此商店分頁。完成後回到這裡查詢結果。
- text: 如果付款結果尚未確認，請勿重新建立訂單或重複付款。
- control: Action / 查看付款結果 {Kind=Primary} | text: 查看付款結果

### 63:1475 Buyer Desktop · 已完成訂單 B-10470 (1440px)
- chrome: Chrome D2
- text: B-10470
- text: 已完成
- text: 這筆訂單已完成
- text: 訂單資訊保留供您核對。
- text: 訂購商品
- text: 付款與配送
- text: 收納袋 × 1
- text: 付款狀態
- text: 已付款
- text: 奶油白 · 單一規格
- text: 訂單狀態
- text: 已完成
- text: 商品金額
- text: NT$480
- text: 運費
- text: NT$0
- text: NT$480
- text: 合計
- control: Action / 返回我的訂單 {Kind=Secondary} | text: 返回查詢訂單

### 63:1476 Buyer Mobile · 已完成訂單 B-10470 (390px)
- chrome: Chrome NONE
- control: Action / ‹ {Kind=Secondary} | text: ‹
- text: B-10470
- text: 已完成
- text: 這筆訂單已完成
- text: 訂單資訊保留供您核對。
- text: 訂購商品
- text: 收納袋 × 1
- text: 奶油白 · 單一規格
- text: 商品金額
- text: NT$480
- text: 運費
- text: NT$0
- text: NT$480
- text: 合計
- text: 付款與配送
- text: 付款狀態
- text: 已付款
- text: 訂單狀態
- text: 已完成
- control: Action / 返回我的訂單 {Kind=Secondary} | text: 返回查詢訂單

### 63:1477 Buyer Desktop · 已取消訂單 B-10463 (1440px)
- chrome: Chrome D2
- text: B-10463
- text: 已取消
- text: 這筆訂單已取消
- text: 無需再為這筆訂單付款。
- text: 訂購商品
- text: 付款與配送
- text: 收納袋 × 1
- text: 付款狀態
- text: 未付款
- text: 奶油白 · 單一規格
- text: 訂單狀態
- text: 已取消
- text: 商品金額
- text: NT$480
- text: 這筆訂單已取消，不提供付款入口。
- text: 運費
- text: NT$0
- text: 合計
- text: NT$480
- control: Action / 返回我的訂單 {Kind=Secondary} | text: 返回查詢訂單

### 63:1478 Buyer Mobile · 已取消訂單 B-10463 (390px)
- chrome: Chrome NONE
- control: Action / ‹ {Kind=Secondary} | text: ‹
- text: B-10463
- text: 已取消
- text: 這筆訂單已取消
- text: 無需再為這筆訂單付款。
- text: 訂購商品
- text: 收納袋 × 1
- text: 奶油白 · 單一規格
- text: 商品金額
- text: NT$480
- text: 運費
- text: NT$0
- text: 合計
- text: NT$480
- text: 付款與配送
- text: 付款狀態
- text: 未付款
- text: 訂單狀態
- text: 已取消
- text: 這筆訂單已取消，不提供付款入口。
- control: Action / 返回我的訂單 {Kind=Secondary} | text: 返回查詢訂單

### 1198:10796 Buyer Desktop · 前往付款 · 門市取貨訂單 LC-10482 (1440px)
- chrome: Chrome D1
- text: 前往付款
- text: 訂單 LC-10482 · NT$1,280
- text: 付款會在新分頁開啟，請保留此商店分頁。完成後回到這裡查詢結果。
- text: 原型示範：不會開啟真實付款頁，也不會扣款。
- text: 如果付款結果尚未確認，請勿重新建立訂單或重複付款。
- control: Action / 查看付款結果 {Kind=Primary} | text: 查看付款結果
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：付款在另一個分頁完成後，付款服務商會回跳到此頁。 / 原型：付款服務商回跳頁
- text: 原型示範（非產品畫面）：付款在另一個分頁完成後，付款服務商會回跳到此頁。
- control: Action / 原型：付款服務商回跳頁 {Kind=Secondary} | text: 原型：付款服務商回跳頁

### 1198:10886 Buyer Mobile · 前往付款 · 門市取貨訂單 LC-10482 (390px)
- chrome: Chrome M1
- text: 前往付款
- text: 訂單 LC-10482 · NT$1,280
- text: 付款會在新分頁開啟，請保留此商店分頁。完成後回到這裡查詢結果。
- text: 原型示範：不會開啟真實付款頁，也不會扣款。
- text: 如果付款結果尚未確認，請勿重新建立訂單或重複付款。
- control: Action / 查看付款結果 {Kind=Primary} | text: 查看付款結果
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：付款在另一個分頁完成後，付款服務商會回跳到此頁。 / 原型：付款服務商回跳頁
- text: 原型示範（非產品畫面）：付款在另一個分頁完成後，付款服務商會回跳到此頁。
- control: Action / 原型：付款服務商回跳頁 {Kind=Secondary} | text: 原型：付款服務商回跳頁

### 1198:14549 Buyer Desktop · 訂單 LC-10482 · 門市取貨 · 付款已記錄 (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已確認
- text: 商品明細
- text: 付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 付款狀態：已記錄付款結果
- text: 訂單狀態：訂單已確認
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: NT$1,280
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 原型示範：沒有真實扣款，也沒有建立真實訂單。
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1198:14615 Buyer Desktop · 訂單 LC-10482 · 門市取貨 · 付款已關閉 (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已取消
- text: 商品明細
- text: 付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 付款狀態：付款已關閉，未扣款
- text: 訂單狀態：訂單已取消
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: NT$1,280
- text: NT$1,280 沒有扣款。如要購買，請回購物車重新下單。
- text: 原型示範：沒有真實扣款，也沒有建立真實訂單。
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- text: 配送狀態：未出貨（訂單已取消）
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 回購物車重新下單 {Kind=Primary} | text: 回購物車重新下單

### 1198:14682 Buyer Desktop · 訂單 LC-10482 · 宅配 · 付款已記錄 (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已確認
- text: 商品明細
- text: 付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 付款狀態：已記錄付款結果
- text: 訂單狀態：訂單已確認
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: NT$1,280
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 原型示範：沒有真實扣款，也沒有建立真實訂單。
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1198:14748 Buyer Desktop · 訂單 LC-10482 · 宅配 · 付款已關閉 (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已取消
- text: 商品明細
- text: 付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 付款狀態：付款已關閉，未扣款
- text: 訂單狀態：訂單已取消
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: NT$1,280
- text: NT$1,280 沒有扣款。如要購買，請回購物車重新下單。
- text: 原型示範：沒有真實扣款，也沒有建立真實訂單。
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- text: 配送狀態：未出貨（訂單已取消）
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 回購物車重新下單 {Kind=Primary} | text: 回購物車重新下單

### 1198:14815 Buyer Desktop · 訂單 LC-10482 · 取貨付款 · 門市取貨 · PENDING (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已確認
- text: 商品明細
- text: 門市取貨
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 取貨時付款 NT$1,280
- text: 運費
- text: NT$0
- text: 取貨進度：已到店，請取貨
- text: 訂單合計
- text: NT$1,280
- text: 無需線上付款，到店取貨時再付款。
- text: 7-ELEVEN · 示範門市 · 000001
- text: 到店通知由物流業者發送，進度可能延遲數小時。
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：已取貨付款 / 原型：未取貨，已退回 / 原型：賣家已在本站外退款 / 原型：賣家已取消訂單
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已取貨付款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨，已退回
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已在本站外退款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已取消訂單

### 1198:14943 Buyer Mobile · 訂單 LC-10482 · 門市取貨 · 付款已記錄 (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已確認
- text: 付款
- text: 付款狀態：已記錄付款結果
- text: 訂單狀態：訂單已確認
- text: NT$1,280
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 原型示範：沒有真實扣款，也沒有建立真實訂單。
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1198:14994 Buyer Mobile · 訂單 LC-10482 · 門市取貨 · 付款已關閉 (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已取消
- text: 付款
- text: 付款狀態：付款已關閉，未扣款
- text: 訂單狀態：訂單已取消
- text: NT$1,280
- text: NT$1,280 沒有扣款。如要購買，請回購物車重新下單。
- text: 原型示範：沒有真實扣款，也沒有建立真實訂單。
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- text: 配送狀態：未出貨（訂單已取消）
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 回購物車重新下單 {Kind=Primary} | text: 回購物車重新下單

### 1198:15048 Buyer Mobile · 訂單 LC-10482 · 宅配 · 付款已記錄 (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已確認
- text: 付款
- text: 付款狀態：已記錄付款結果
- text: 訂單狀態：訂單已確認
- text: NT$1,280
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 原型示範：沒有真實扣款，也沒有建立真實訂單。
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1198:15099 Buyer Mobile · 訂單 LC-10482 · 宅配 · 付款已關閉 (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已取消
- text: 付款
- text: 付款狀態：付款已關閉，未扣款
- text: 訂單狀態：訂單已取消
- text: NT$1,280
- text: NT$1,280 沒有扣款。如要購買，請回購物車重新下單。
- text: 原型示範：沒有真實扣款，也沒有建立真實訂單。
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- text: 配送狀態：未出貨（訂單已取消）
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 回購物車重新下單 {Kind=Primary} | text: 回購物車重新下單

### 1198:15153 Buyer Mobile · 訂單 LC-10482 · 取貨付款 · 門市取貨 · PENDING (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已確認
- text: 門市取貨
- text: 取貨時付款 NT$1,280
- text: 取貨進度：已到店，請取貨
- text: 7-ELEVEN · 示範門市 · 000001
- text: 到店通知由物流業者發送，進度可能延遲數小時。
- text: 無需線上付款，到店取貨時再付款。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：已取貨付款 / 原型：未取貨，已退回 / 原型：賣家已在本站外退款 / 原型：賣家已取消訂單
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已取貨付款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨，已退回
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已在本站外退款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已取消訂單
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1207:11941 Buyer Desktop · 訂單 LC-10482 · 門市取貨 · 付款已發起，結果未確認 (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 等待付款結果
- text: 商品明細
- text: 付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 付款狀態：等待付款結果
- text: 訂單狀態：等待付款結果
- text: 運費
- text: NT$0
- text: NT$1,280
- text: NT$1,280
- text: 訂單合計
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 已請求跳轉付款，請查看另一個分頁，再重新整理此訂單查詢結果。這不代表付款成功。
- text: 收件與配送
- text: 付款會在新分頁開啟，完成後回到這裡查詢結果。
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Action / 繼續原付款請求 {Kind=Primary} | text: 繼續原付款請求
- control: Action / 取消付款 {Kind=Secondary} | text: 取消付款
- text: 配送狀態：備貨中
- text: 原型示範：沒有真實付款，也沒有扣款。
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。 / 原型：已記錄付款 / 原型：付款已關閉未扣款
- text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。
- control: Action / 原型：已記錄付款 {Kind=Secondary} | text: 原型：已記錄付款
- control: Action / 原型：付款已關閉未扣款 {Kind=Secondary} | text: 原型：付款已關閉未扣款

### 1207:12009 Buyer Mobile · 訂單 LC-10482 · 門市取貨 · 付款已發起，結果未確認 (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 等待付款結果
- text: 付款
- text: 付款狀態：等待付款結果
- text: 訂單狀態：等待付款結果
- text: NT$1,280
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 已請求跳轉付款，請查看另一個分頁，再重新整理此訂單查詢結果。這不代表付款成功。
- text: 付款會在新分頁開啟，完成後回到這裡查詢結果。
- control: Action / 繼續原付款請求 {Kind=Primary} | text: 繼續原付款請求
- control: Action / 取消付款 {Kind=Secondary} | text: 取消付款
- text: 原型示範：沒有真實付款，也沒有扣款。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。 / 原型：已記錄付款 / 原型：付款已關閉未扣款
- text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。
- control: Action / 原型：已記錄付款 {Kind=Secondary} | text: 原型：已記錄付款
- control: Action / 原型：付款已關閉未扣款 {Kind=Secondary} | text: 原型：付款已關閉未扣款
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1207:12062 Buyer Desktop · 訂單 LC-10482 · 宅配 · 付款已發起，結果未確認 (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 等待付款結果
- text: 商品明細
- text: 付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 付款狀態：等待付款結果
- text: 訂單狀態：等待付款結果
- text: 運費
- text: NT$0
- text: NT$1,280
- text: NT$1,280
- text: 訂單合計
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 已請求跳轉付款，請查看另一個分頁，再重新整理此訂單查詢結果。這不代表付款成功。
- text: 收件與配送
- text: 付款會在新分頁開啟，完成後回到這裡查詢結果。
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 繼續原付款請求 {Kind=Primary} | text: 繼續原付款請求
- control: Action / 取消付款 {Kind=Secondary} | text: 取消付款
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- text: 原型示範：沒有真實付款，也沒有扣款。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。 / 原型：已記錄付款 / 原型：付款已關閉未扣款
- text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。
- control: Action / 原型：已記錄付款 {Kind=Secondary} | text: 原型：已記錄付款
- control: Action / 原型：付款已關閉未扣款 {Kind=Secondary} | text: 原型：付款已關閉未扣款

### 1207:12130 Buyer Mobile · 訂單 LC-10482 · 宅配 · 付款已發起，結果未確認 (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 等待付款結果
- text: 付款
- text: 付款狀態：等待付款結果
- text: 訂單狀態：等待付款結果
- text: NT$1,280
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 已請求跳轉付款，請查看另一個分頁，再重新整理此訂單查詢結果。這不代表付款成功。
- text: 付款會在新分頁開啟，完成後回到這裡查詢結果。
- control: Action / 繼續原付款請求 {Kind=Primary} | text: 繼續原付款請求
- control: Action / 取消付款 {Kind=Secondary} | text: 取消付款
- text: 原型示範：沒有真實付款，也沒有扣款。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。 / 原型：已記錄付款 / 原型：付款已關閉未扣款
- text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。
- control: Action / 原型：已記錄付款 {Kind=Secondary} | text: 原型：已記錄付款
- control: Action / 原型：付款已關閉未扣款 {Kind=Secondary} | text: 原型：付款已關閉未扣款
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1207:16447 Buyer Desktop · 付款服務商回跳 · 中性頁（/payment/return，不代表付款成功） (1440px)
- chrome: Chrome NONE
- text: Return to your store
- text: Check your order
- text: This page does not confirm payment. Return to your original store tab and select “Refresh order” to check the result. You may close this tab. If you lost the store tab, reopen your store; this page cannot identify it for you.
- text: 返回商店查詢訂單
- text: 此頁面不代表付款成功。請回到原來的商店標籤頁，點擊“刷新訂單”查詢結果。你可以關閉本頁。若原標籤頁已關閉，請自行重新打開商店；此頁面無法判斷你所屬的商店。
- text: 返回商店查詢訂單
- text: 此頁面不代表付款成功。請回到原本的商店分頁，點選「重新整理訂單」查詢結果。你可以關閉本頁。若原分頁已關閉，請自行重新開啟商店；此頁面無法判斷你所屬的商店。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：關閉本分頁後回到原商店分頁並重新整理訂單。 / 原型：回到原商店分頁
- text: 原型示範（非產品畫面）：關閉本分頁後回到原商店分頁並重新整理訂單。
- control: Action / 原型：回到原商店分頁 {Kind=Secondary} | text: 原型：回到原商店分頁

### 1207:16465 Buyer Mobile · 付款服務商回跳 · 中性頁（/payment/return，不代表付款成功） (390px)
- chrome: Chrome NONE
- text: Return to your store
- text: Check your order
- text: This page does not confirm payment. Return to your original store tab and select “Refresh order” to check the result. You may close this tab. If you lost the store tab, reopen your store; this page cannot identify it for you.
- text: 返回商店查詢訂單
- text: 此頁面不代表付款成功。請回到原來的商店標籤頁，點擊“刷新訂單”查詢結果。你可以關閉本頁。若原標籤頁已關閉，請自行重新打開商店；此頁面無法判斷你所屬的商店。
- text: 返回商店查詢訂單
- text: 此頁面不代表付款成功。請回到原本的商店分頁，點選「重新整理訂單」查詢結果。你可以關閉本頁。若原分頁已關閉，請自行重新開啟商店；此頁面無法判斷你所屬的商店。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：關閉本分頁後回到原商店分頁並重新整理訂單。 / 原型：回到原商店分頁
- text: 原型示範（非產品畫面）：關閉本分頁後回到原商店分頁並重新整理訂單。
- control: Action / 原型：回到原商店分頁 {Kind=Secondary} | text: 原型：回到原商店分頁

### 1222:19371 Buyer Desktop · 訂單 LC-10482 · 門市 · 取消付款確認（瀏覽器原生確認框） (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 等待付款結果
- text: 商品明細
- text: 付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 付款狀態：等待付款結果
- text: 訂單狀態：等待付款結果
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: NT$1,280
- text: 瀏覽器原生確認框（window.confirm）：標題與按鈕文字由瀏覽器提供，不是產品文案。
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 確定取消付款？支付服務商確認後訂單會取消。如已付款，以付款為準。
- text: 已請求跳轉付款，請查看另一個分頁，再重新整理此訂單查詢結果。這不代表付款成功。
- text: 收件與配送
- control: Buttons | text: 取消 / 確定
- control: Action / 取消付款 {Kind=Secondary} | text: 取消
- control: Action / 繼續原付款請求 {Kind=Primary} | text: 確定
- text: 付款會在新分頁開啟，完成後回到這裡查詢結果。
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Action / 繼續原付款請求 {Kind=Primary} | text: 繼續原付款請求
- control: Action / 取消付款 {Kind=Secondary} | text: 取消付款
- text: 配送狀態：備貨中
- text: 原型示範：沒有真實付款，也沒有扣款。
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。 / 原型：已記錄付款 / 原型：付款已關閉未扣款
- text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。
- control: Action / 原型：已記錄付款 {Kind=Secondary} | text: 原型：已記錄付款
- control: Action / 原型：付款已關閉未扣款 {Kind=Secondary} | text: 原型：付款已關閉未扣款

### 1222:19438 Buyer Desktop · 訂單 LC-10482 · 門市 · 已請求取消，等待付款服務商確認 (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 等待付款結果
- text: 商品明細
- text: 付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 付款狀態：等待付款結果
- text: 訂單狀態：等待付款結果
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: NT$1,280
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 已請求取消，正在等待支付服務商確認，請重新整理此訂單查詢。
- text: 收件與配送
- text: 原型示範：沒有真實付款，也沒有扣款。
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：付款服務商確認取消後，重新整理訂單可能得到的結果。 / 原型：已記錄付款 / 原型：付款已關閉未扣款
- text: 原型示範（非產品畫面）：付款服務商確認取消後，重新整理訂單可能得到的結果。
- text: 配送狀態：備貨中
- control: Action / 原型：已記錄付款 {Kind=Secondary} | text: 原型：已記錄付款
- control: Action / 原型：付款已關閉未扣款 {Kind=Secondary} | text: 原型：付款已關閉未扣款
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:19527 Buyer Mobile · 訂單 LC-10482 · 門市 · 取消付款確認（瀏覽器原生確認框） (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 等待付款結果
- text: 付款
- text: 付款狀態：等待付款結果
- text: 瀏覽器原生確認框（window.confirm）：標題與按鈕文字由瀏覽器提供，不是產品文案。
- text: 訂單狀態：等待付款結果
- text: NT$1,280
- text: 確定取消付款？支付服務商確認後訂單會取消。如已付款，以付款為準。
- text: 商品 NT$1,280 + 運費 NT$0。
- control: Buttons | text: 取消 / 確定
- control: Action / 取消付款 {Kind=Secondary} | text: 取消
- control: Action / 繼續原付款請求 {Kind=Primary} | text: 確定
- text: 已請求跳轉付款，請查看另一個分頁，再重新整理此訂單查詢結果。這不代表付款成功。
- text: 付款會在新分頁開啟，完成後回到這裡查詢結果。
- control: Action / 繼續原付款請求 {Kind=Primary} | text: 繼續原付款請求
- control: Action / 取消付款 {Kind=Secondary} | text: 取消付款
- text: 原型示範：沒有真實付款，也沒有扣款。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。 / 原型：已記錄付款 / 原型：付款已關閉未扣款
- text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。
- control: Action / 原型：已記錄付款 {Kind=Secondary} | text: 原型：已記錄付款
- control: Action / 原型：付款已關閉未扣款 {Kind=Secondary} | text: 原型：付款已關閉未扣款
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:19581 Buyer Mobile · 訂單 LC-10482 · 門市 · 已請求取消，等待付款服務商確認 (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 等待付款結果
- text: 付款
- text: 付款狀態：等待付款結果
- text: 訂單狀態：等待付款結果
- text: NT$1,280
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 已請求取消，正在等待支付服務商確認，請重新整理此訂單查詢。
- text: 原型示範：沒有真實付款，也沒有扣款。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：付款服務商確認取消後，重新整理訂單可能得到的結果。 / 原型：已記錄付款 / 原型：付款已關閉未扣款
- text: 原型示範（非產品畫面）：付款服務商確認取消後，重新整理訂單可能得到的結果。
- control: Action / 原型：已記錄付款 {Kind=Secondary} | text: 原型：已記錄付款
- control: Action / 原型：付款已關閉未扣款 {Kind=Secondary} | text: 原型：付款已關閉未扣款
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:19657 Buyer Desktop · 訂單 LC-10482 · 宅配 · 取消付款確認（瀏覽器原生確認框） (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 等待付款結果
- text: 商品明細
- text: 付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 付款狀態：等待付款結果
- text: 訂單狀態：等待付款結果
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: NT$1,280
- text: 瀏覽器原生確認框（window.confirm）：標題與按鈕文字由瀏覽器提供，不是產品文案。
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 確定取消付款？支付服務商確認後訂單會取消。如已付款，以付款為準。
- text: 已請求跳轉付款，請查看另一個分頁，再重新整理此訂單查詢結果。這不代表付款成功。
- text: 收件與配送
- control: Buttons | text: 取消 / 確定
- control: Action / 取消付款 {Kind=Secondary} | text: 取消
- control: Action / 繼續原付款請求 {Kind=Primary} | text: 確定
- text: 付款會在新分頁開啟，完成後回到這裡查詢結果。
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 繼續原付款請求 {Kind=Primary} | text: 繼續原付款請求
- control: Action / 取消付款 {Kind=Secondary} | text: 取消付款
- text: 配送狀態：備貨中
- text: 原型示範：沒有真實付款，也沒有扣款。
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。 / 原型：已記錄付款 / 原型：付款已關閉未扣款
- text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。
- control: Action / 原型：已記錄付款 {Kind=Secondary} | text: 原型：已記錄付款
- control: Action / 原型：付款已關閉未扣款 {Kind=Secondary} | text: 原型：付款已關閉未扣款

### 1222:19724 Buyer Desktop · 訂單 LC-10482 · 宅配 · 已請求取消，等待付款服務商確認 (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 等待付款結果
- text: 商品明細
- text: 付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 付款狀態：等待付款結果
- text: 訂單狀態：等待付款結果
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: NT$1,280
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 已請求取消，正在等待支付服務商確認，請重新整理此訂單查詢。
- text: 收件與配送
- text: 原型示範：沒有真實付款，也沒有扣款。
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：付款服務商確認取消後，重新整理訂單可能得到的結果。 / 原型：已記錄付款 / 原型：付款已關閉未扣款
- text: 原型示範（非產品畫面）：付款服務商確認取消後，重新整理訂單可能得到的結果。
- text: 配送狀態：備貨中
- control: Action / 原型：已記錄付款 {Kind=Secondary} | text: 原型：已記錄付款
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Action / 原型：付款已關閉未扣款 {Kind=Secondary} | text: 原型：付款已關閉未扣款

### 1222:19813 Buyer Mobile · 訂單 LC-10482 · 宅配 · 取消付款確認（瀏覽器原生確認框） (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 等待付款結果
- text: 付款
- text: 付款狀態：等待付款結果
- text: 瀏覽器原生確認框（window.confirm）：標題與按鈕文字由瀏覽器提供，不是產品文案。
- text: 訂單狀態：等待付款結果
- text: NT$1,280
- text: 確定取消付款？支付服務商確認後訂單會取消。如已付款，以付款為準。
- text: 商品 NT$1,280 + 運費 NT$0。
- control: Buttons | text: 取消 / 確定
- control: Action / 取消付款 {Kind=Secondary} | text: 取消
- control: Action / 繼續原付款請求 {Kind=Primary} | text: 確定
- text: 已請求跳轉付款，請查看另一個分頁，再重新整理此訂單查詢結果。這不代表付款成功。
- text: 付款會在新分頁開啟，完成後回到這裡查詢結果。
- control: Action / 繼續原付款請求 {Kind=Primary} | text: 繼續原付款請求
- control: Action / 取消付款 {Kind=Secondary} | text: 取消付款
- text: 原型示範：沒有真實付款，也沒有扣款。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。 / 原型：已記錄付款 / 原型：付款已關閉未扣款
- text: 原型示範（非產品畫面）：重新整理訂單後可能得到的結果。
- control: Action / 原型：已記錄付款 {Kind=Secondary} | text: 原型：已記錄付款
- control: Action / 原型：付款已關閉未扣款 {Kind=Secondary} | text: 原型：付款已關閉未扣款
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:19867 Buyer Mobile · 訂單 LC-10482 · 宅配 · 已請求取消，等待付款服務商確認 (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 等待付款結果
- text: 付款
- text: 付款狀態：等待付款結果
- text: 訂單狀態：等待付款結果
- text: NT$1,280
- text: 商品 NT$1,280 + 運費 NT$0。
- text: 已請求取消，正在等待支付服務商確認，請重新整理此訂單查詢。
- text: 原型示範：沒有真實付款，也沒有扣款。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：付款服務商確認取消後，重新整理訂單可能得到的結果。 / 原型：已記錄付款 / 原型：付款已關閉未扣款
- text: 原型示範（非產品畫面）：付款服務商確認取消後，重新整理訂單可能得到的結果。
- control: Action / 原型：已記錄付款 {Kind=Secondary} | text: 原型：已記錄付款
- control: Action / 原型：付款已關閉未扣款 {Kind=Secondary} | text: 原型：付款已關閉未扣款
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- text: 配送狀態：備貨中
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:21567 Buyer Desktop · 訂單 LC-10482 · 取貨付款 · 門市取貨 · COLLECTED (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已確認
- text: 商品明細
- text: 門市取貨
- text: 已取貨付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 取貨進度：已取貨
- text: 7-ELEVEN · 示範門市 · 000001
- text: 訂單合計
- text: NT$1,280
- text: 到店通知由物流業者發送，進度可能延遲數小時。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：取貨付款待付 / 原型：未取貨，已退回 / 原型：賣家已在本站外退款 / 原型：賣家已取消訂單
- text: 收件與配送
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：取貨付款待付
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨，已退回
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已在本站外退款
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已取消訂單

### 1222:21624 Buyer Desktop · 訂單 LC-10482 · 取貨付款 · 門市取貨 · RETURNED (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已確認
- text: 商品明細
- text: 門市取貨
- text: 未取貨，已退回
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 取貨進度：逾期未取，退回中
- text: 7-ELEVEN · 示範門市 · 000001
- text: 訂單合計
- text: NT$1,280
- text: 到店通知由物流業者發送，進度可能延遲數小時。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：取貨付款待付 / 原型：已取貨付款 / 原型：賣家已在本站外退款 / 原型：賣家已取消訂單
- text: 收件與配送
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：取貨付款待付
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已取貨付款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已在本站外退款
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已取消訂單

### 1222:21681 Buyer Desktop · 訂單 LC-10482 · 取貨付款 · 門市取貨 · REFUNDED_OFFLINE (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已確認
- text: 商品明細
- text: 門市取貨
- text: 賣家已在本站外退款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 取貨進度：已取貨
- text: 7-ELEVEN · 示範門市 · 000001
- text: 訂單合計
- text: NT$1,280
- text: 到店通知由物流業者發送，進度可能延遲數小時。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：取貨付款待付 / 原型：已取貨付款 / 原型：未取貨，已退回 / 原型：賣家已取消訂單
- text: 收件與配送
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：取貨付款待付
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已取貨付款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨，已退回
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已取消訂單

### 1222:21738 Buyer Desktop · 訂單 LC-10482 · 取貨付款 · 門市取貨 · CANCELLED (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已取消
- text: 商品明細
- text: 門市取貨
- text: 賣家已取消訂單
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：取貨付款待付 / 原型：已取貨付款 / 原型：未取貨，已退回 / 原型：賣家已在本站外退款
- text: 訂單合計
- text: NT$1,280
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：取貨付款待付
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已取貨付款
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨，已退回
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已在本站外退款
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:21795 Buyer Desktop · 訂單 LC-10482 · 貨到付款 · 宅配 · PENDING (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 商品明細
- text: 貨到付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 包裹送達時向貨運公司付款
- text: 運費
- text: NT$0
- text: 到貨需付 NT$1,280
- text: 示範
- text: 下單時的物流商 · 黑貓（手工出貨）
- text: 訂單合計
- text: NT$1,280
- text: 請以現金付款。商家會記錄收款結果；退款在本站之外進行。
- text: 收件與配送
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：已貨到收款 / 原型：未取貨 — 已退回 / 原型：商家已在本站之外退款 / 原型：商家已取消此訂單 / 原型：未取貨 — 已退回商家
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已貨到收款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已在本站之外退款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已取消此訂單
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回商家

### 1222:21852 Buyer Desktop · 訂單 LC-10482 · 貨到付款 · 宅配 · COLLECTED (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 商品明細
- text: 貨到付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 已貨到收款
- text: 運費
- text: NT$0
- text: 原訂單代收金額 NT$1,280
- text: 示範
- text: 下單時的物流商 · 黑貓（手工出貨）
- text: 訂單合計
- text: NT$1,280
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：包裹送達時付款 / 原型：未取貨 — 已退回 / 原型：商家已在本站之外退款 / 原型：商家已取消此訂單 / 原型：未取貨 — 已退回商家
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- text: 收件與配送
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：包裹送達時付款
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已在本站之外退款
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已取消此訂單
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回商家

### 1222:21909 Buyer Desktop · 訂單 LC-10482 · 貨到付款 · 宅配 · RETURNED (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 商品明細
- text: 貨到付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 未取貨 — 已退回
- text: 運費
- text: NT$0
- text: 原訂單代收金額 NT$1,280
- text: 示範
- text: 下單時的物流商 · 黑貓（手工出貨）
- text: 訂單合計
- text: NT$1,280
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：包裹送達時付款 / 原型：已貨到收款 / 原型：商家已在本站之外退款 / 原型：商家已取消此訂單 / 原型：未取貨 — 已退回商家
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- text: 收件與配送
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：包裹送達時付款
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已貨到收款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已在本站之外退款
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已取消此訂單
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回商家

### 1222:21966 Buyer Desktop · 訂單 LC-10482 · 貨到付款 · 宅配 · REFUNDED_OFFLINE (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 商品明細
- text: 貨到付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 商家已在本站之外退款
- text: 運費
- text: NT$0
- text: 原訂單代收金額 NT$1,280
- text: 示範
- text: 下單時的物流商 · 黑貓（手工出貨）
- text: 訂單合計
- text: NT$1,280
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：包裹送達時付款 / 原型：已貨到收款 / 原型：未取貨 — 已退回 / 原型：商家已取消此訂單 / 原型：未取貨 — 已退回商家
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- text: 收件與配送
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：包裹送達時付款
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已貨到收款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已取消此訂單
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回商家

### 1222:22023 Buyer Desktop · 訂單 LC-10482 · 貨到付款 · 宅配 · CANCELLED (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 商品明細
- text: 貨到付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 商家已取消此訂單
- text: 運費
- text: NT$0
- text: 原訂單代收金額 NT$1,280
- text: 示範
- text: 下單時的物流商 · 黑貓（手工出貨）
- text: 訂單合計
- text: NT$1,280
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：包裹送達時付款 / 原型：已貨到收款 / 原型：未取貨 — 已退回 / 原型：商家已在本站之外退款 / 原型：未取貨 — 已退回商家
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- text: 收件與配送
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：包裹送達時付款
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已貨到收款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已在本站之外退款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回商家

### 1222:22080 Buyer Desktop · 訂單 LC-10482 · 貨到付款 · 宅配 · RESTOCKED (1440px)
- chrome: Chrome D1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 商品明細
- text: 貨到付款
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 未取貨 — 已退回商家
- text: 運費
- text: NT$0
- text: 原訂單代收金額 NT$1,280
- text: 示範
- text: 下單時的物流商 · 黑貓（手工出貨）
- text: 訂單合計
- text: NT$1,280
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：包裹送達時付款 / 原型：已貨到收款 / 原型：未取貨 — 已退回 / 原型：商家已在本站之外退款 / 原型：商家已取消此訂單
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- text: 收件與配送
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：包裹送達時付款
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已貨到收款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已在本站之外退款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已取消此訂單

### 1222:22137 Buyer Mobile · 訂單 LC-10482 · 取貨付款 · 門市取貨 · COLLECTED (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已確認
- text: 門市取貨
- text: 已取貨付款
- text: 取貨進度：已取貨
- text: 7-ELEVEN · 示範門市 · 000001
- text: 到店通知由物流業者發送，進度可能延遲數小時。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：取貨付款待付 / 原型：未取貨，已退回 / 原型：賣家已在本站外退款 / 原型：賣家已取消訂單
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：取貨付款待付
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨，已退回
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已在本站外退款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已取消訂單
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:22181 Buyer Mobile · 訂單 LC-10482 · 取貨付款 · 門市取貨 · RETURNED (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已確認
- text: 門市取貨
- text: 未取貨，已退回
- text: 取貨進度：逾期未取，退回中
- text: 7-ELEVEN · 示範門市 · 000001
- text: 到店通知由物流業者發送，進度可能延遲數小時。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：取貨付款待付 / 原型：已取貨付款 / 原型：賣家已在本站外退款 / 原型：賣家已取消訂單
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：取貨付款待付
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已取貨付款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已在本站外退款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已取消訂單
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:22225 Buyer Mobile · 訂單 LC-10482 · 取貨付款 · 門市取貨 · REFUNDED_OFFLINE (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已確認
- text: 門市取貨
- text: 賣家已在本站外退款
- text: 取貨進度：已取貨
- text: 7-ELEVEN · 示範門市 · 000001
- text: 到店通知由物流業者發送，進度可能延遲數小時。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：取貨付款待付 / 原型：已取貨付款 / 原型：未取貨，已退回 / 原型：賣家已取消訂單
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：取貨付款待付
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已取貨付款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨，已退回
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已取消訂單
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: NT$1,280
- text: 訂單合計
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:22269 Buyer Mobile · 訂單 LC-10482 · 取貨付款 · 門市取貨 · CANCELLED (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 訂單已取消
- text: 門市取貨
- text: 賣家已取消訂單
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：取貨付款待付 / 原型：已取貨付款 / 原型：未取貨，已退回 / 原型：賣家已在本站外退款
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：取貨付款待付
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已取貨付款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨，已退回
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：賣家已在本站外退款
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 門市取貨 · 7-ELEVEN\n示範買家 · 09•• ••• ••01\n示範門市 · 編號 000001\n台北市中正區示範路 20 號
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:22313 Buyer Mobile · 訂單 LC-10482 · 貨到付款 · 宅配 · PENDING (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 貨到付款
- text: 包裹送達時向貨運公司付款
- text: 到貨需付 NT$1,280
- text: 示範
- text: 下單時的物流商 · 黑貓（手工出貨）
- text: 請以現金付款。商家會記錄收款結果；退款在本站之外進行。
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：已貨到收款 / 原型：未取貨 — 已退回 / 原型：商家已在本站之外退款 / 原型：商家已取消此訂單 / 原型：未取貨 — 已退回商家
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已貨到收款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已在本站之外退款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已取消此訂單
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回商家
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:22357 Buyer Mobile · 訂單 LC-10482 · 貨到付款 · 宅配 · COLLECTED (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 貨到付款
- text: 已貨到收款
- text: 原訂單代收金額 NT$1,280
- text: 示範
- text: 下單時的物流商 · 黑貓（手工出貨）
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：包裹送達時付款 / 原型：未取貨 — 已退回 / 原型：商家已在本站之外退款 / 原型：商家已取消此訂單 / 原型：未取貨 — 已退回商家
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：包裹送達時付款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已在本站之外退款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已取消此訂單
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回商家
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:22401 Buyer Mobile · 訂單 LC-10482 · 貨到付款 · 宅配 · RETURNED (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 貨到付款
- text: 未取貨 — 已退回
- text: 原訂單代收金額 NT$1,280
- text: 示範
- text: 下單時的物流商 · 黑貓（手工出貨）
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：包裹送達時付款 / 原型：已貨到收款 / 原型：商家已在本站之外退款 / 原型：商家已取消此訂單 / 原型：未取貨 — 已退回商家
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：包裹送達時付款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已貨到收款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已在本站之外退款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已取消此訂單
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回商家
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:22445 Buyer Mobile · 訂單 LC-10482 · 貨到付款 · 宅配 · REFUNDED_OFFLINE (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 貨到付款
- text: 商家已在本站之外退款
- text: 原訂單代收金額 NT$1,280
- text: 示範
- text: 下單時的物流商 · 黑貓（手工出貨）
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：包裹送達時付款 / 原型：已貨到收款 / 原型：未取貨 — 已退回 / 原型：商家已取消此訂單 / 原型：未取貨 — 已退回商家
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：包裹送達時付款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已貨到收款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已取消此訂單
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回商家
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:22489 Buyer Mobile · 訂單 LC-10482 · 貨到付款 · 宅配 · CANCELLED (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 貨到付款
- text: 商家已取消此訂單
- text: 原訂單代收金額 NT$1,280
- text: 示範
- text: 下單時的物流商 · 黑貓（手工出貨）
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：包裹送達時付款 / 原型：已貨到收款 / 原型：未取貨 — 已退回 / 原型：商家已在本站之外退款 / 原型：未取貨 — 已退回商家
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：包裹送達時付款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已貨到收款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已在本站之外退款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回商家
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1222:22533 Buyer Mobile · 訂單 LC-10482 · 貨到付款 · 宅配 · RESTOCKED (390px)
- chrome: Chrome M1 (hidden text nodes skipped: 2)
- control: Action / ‹ 我的訂單 {Kind=Secondary} | text: ‹ 查詢訂單
- text: 訂單 LC-10482
- text: 貨到付款
- text: 未取貨 — 已退回商家
- text: 原訂單代收金額 NT$1,280
- text: 示範
- text: 下單時的物流商 · 黑貓（手工出貨）
- control: Prototype switch (not product UI) | text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。 / 原型：包裹送達時付款 / 原型：已貨到收款 / 原型：未取貨 — 已退回 / 原型：商家已在本站之外退款 / 原型：商家已取消此訂單
- text: 原型示範（非產品畫面）：商家記錄收款結果後，重新整理訂單可能得到的結果。
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：包裹送達時付款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：已貨到收款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：未取貨 — 已退回
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已在本站之外退款
- control: Action / 原型：模擬格式錯誤 {Kind=Secondary} | text: 原型：商家已取消此訂單
- text: 商品明細
- text: 焦糖托特包 × 1
- text: NT$1,280
- text: 運費
- text: NT$0
- text: 訂單合計
- text: NT$1,280
- text: 收件與配送
- text: 商家自行安排配送\n示範買家 · 09•• ••• ••01\n台北市中正區示範路 1 號
- control: Action / 聯絡商店 {Kind=Secondary} | text: 聯絡商店
- control: Action / 申請售後 {Kind=Secondary} | text: 申請售後

### 1253:17225 Buyer Desktop · 訂單查詢查不到 · 狀態 (1440px)
- chrome: Chrome D1
- text: 查詢訂單
- text: 輸入訂單通知信中的訂單編號，以及下單時使用的電子郵件或收件人手機號碼。
- control: Field / 訂單編號 | text: 訂單編號 / 0123-ABCD-4567 / 格式如 0123-ABCD-4567
- text: 訂單編號
- control: Input / 0123-ABCD-4567 {State=Filled} | text: 0123-ABCD-4567
- text: 格式如 0123-ABCD-4567
- control: Field / 電子郵件或手機號碼 | text: 電子郵件或手機號碼 / name@example.com / 與下單時填寫的相同
- text: 電子郵件或手機號碼
- control: Input / name@example.com {State=Filled} | text: name@example.com
- text: 與下單時填寫的相同
- text: 找不到符合的訂單，請確認訂單編號與電子郵件或手機號碼。
- control: Primary | text: 查詢
- text: 你輸入的內容已保留，可直接修改後再查詢。

### 1253:17256 Buyer Mobile · 訂單查詢查不到 · 狀態 (390px)
- chrome: Chrome M1
- text: 查詢訂單
- text: 輸入訂單通知信中的訂單編號，以及下單時使用的電子郵件或收件人手機號碼。
- control: Field / 訂單編號 | text: 訂單編號 / 0123-ABCD-4567 / 格式如 0123-ABCD-4567
- text: 訂單編號
- control: Input / 0123-ABCD-4567 {State=Filled} | text: 0123-ABCD-4567
- text: 格式如 0123-ABCD-4567
- control: Field / 電子郵件或手機號碼 | text: 電子郵件或手機號碼 / name@example.com / 與下單時填寫的相同
- text: 電子郵件或手機號碼
- control: Input / name@example.com {State=Filled} | text: name@example.com
- text: 與下單時填寫的相同
- text: 找不到符合的訂單，請確認訂單編號與電子郵件或手機號碼。
- control: Primary | text: 查詢
- text: 你輸入的內容已保留，可直接修改後再查詢。

---

## U:b-privacy (20 frames exported, 0 NOT_FOUND)

### 5:15460 Buyer Desktop · 隱私與資料 · SHOPLINE (1440px)
- chrome: Chrome D2
- text: 隱私與資料
- text: 原型，未真實寫入；以下為本店隱私與資料流程示範。
- text: 你的隱私選擇
- text: 購買不代表同意；關閉以下選擇不影響現有訂單。
- text: 行銷私訊
- text: 目前未開啟
- text: 廣告個人化
- text: 目前未開啟
- control: Action / 管理這兩項選擇 {Kind=Secondary} | text: 管理這兩項選擇
- text: 關閉廣告個人化會停止後續事件；已傳送的事件無法撤回。
- text: 下載資料副本
- text: 下載本店持有的你的資料副本，包含訂單及目前的隱私選擇。
- control: Action / 下載我的資料 {Kind=Secondary} | text: 下載我的資料
- text: 抹除我的資料
- text: 抹除會撤回你的選擇並結束購物連線；訂單、付款與出貨紀錄仍會保留。
- text: 若仍有進行中的付款或退款，需先等待該操作結束。
- control: Action / 檢視抹除範圍 {Kind=Secondary} | text: 檢視抹除範圍
- text: 資料如何用於購物
- text: 收件人姓名、電話、地址或取貨門市資料，用於處理訂單與配送。
- text: 可用付款方式與結果，請以結帳及訂單頁顯示為準。

### 5:15540 Buyer Mobile · 隱私與資料 · SHOPLINE (390px)
- chrome: Chrome M2
- text: 隱私與資料
- text: 原型，未真實寫入；以下為隱私流程示範。
- text: 你的隱私選擇
- text: 購買不代表同意；關閉以下選擇不影響現有訂單。
- text: 行銷私訊
- text: 目前未開啟
- text: 廣告個人化
- text: 目前未開啟
- control: Action / 管理這兩項選擇 {Kind=Secondary} | text: 管理這兩項選擇
- text: 關閉廣告個人化會停止後續事件；已傳送的事件無法撤回。
- text: 下載資料副本
- text: 下載本店持有的你的資料副本，包含訂單及目前的隱私選擇。
- control: Action / 下載我的資料 {Kind=Secondary} | text: 下載我的資料
- text: 抹除我的資料
- text: 抹除會撤回你的選擇並結束購物連線；訂單、付款與出貨紀錄仍會保留。
- text: 若仍有進行中的付款或退款，需先等待該操作結束。
- control: Action / 檢視抹除範圍 {Kind=Secondary} | text: 檢視抹除範圍
- text: 資料如何用於購物
- text: 收件人姓名、電話、地址或取貨門市資料，用於處理訂單與配送。
- text: 可用付款方式與結果，請以結帳及訂單頁顯示為準。

### 5:15707 Buyer Desktop · 資料刪除說明 · SHOPLINE (1440px)
- chrome: Chrome D2
- text: 資料刪除說明
- text: 依資料來源選擇對應的處理方式。
- text: 購物資料
- text: 請使用下單時的裝置，開啟「隱私與資料」，再選擇「抹除我的資料」。
- text: 抹除會撤回你的選擇並結束購物連線。訂單、付款與出貨紀錄仍會保留。
- control: Action / 前往隱私與資料 {Kind=Primary} | text: 前往隱私與資料
- text: Facebook / Instagram 資料
- text: 移除應用程式連結與抹除店鋪購物資料是不同操作。請依 Meta 帳號中的應用程式設定操作。
- text: 無法使用原裝置，或需要協助時，請聯絡購買的店鋪。
- control: Action / 聯絡店鋪 {Kind=Secondary} | text: 聯絡店鋪

### 5:15793 Buyer Mobile · 資料刪除說明 · SHOPLINE (390px)
- chrome: Chrome M2
- text: 資料刪除說明
- text: 依資料來源選擇對應的處理方式。
- text: 購物資料
- text: 請使用下單時的裝置，開啟「隱私與資料」，再選擇「抹除我的資料」。
- text: 抹除會撤回你的選擇並結束購物連線。訂單、付款與出貨紀錄仍會保留。
- control: Action / 前往隱私與資料 {Kind=Primary} | text: 前往隱私與資料
- text: Facebook / Instagram 資料
- text: 移除應用程式連結與抹除店鋪購物資料是不同操作。請依 Meta 帳號中的應用程式設定操作。
- text: 無法使用原裝置，或需要協助時，請聯絡購買的店鋪。
- control: Action / 聯絡店鋪 {Kind=Secondary} | text: 聯絡店鋪

### 26:888 Buyer Desktop · 隱私與同意設定 · SHOPLINE (1440px)
- chrome: Chrome D1
- text: 隱私與同意設定
- text: 必要資料
- text: 完成訂購、配送與查詢訂單所需的資料，依商店的隱私政策處理。
- text: 選擇性使用
- text: ☐ 接收商店行銷資訊
- text: ☐ 允許廣告個人化使用
- text: 可隨時更新選擇，不影響已成立訂單。
- control: Action / 儲存選擇 {Kind=Primary} | text: 儲存選擇

### 26:1150 Buyer Mobile · 隱私與同意設定 · SHOPLINE (390px)
- chrome: Chrome M1
- text: 隱私與同意設定
- text: 必要資料
- text: 完成訂購、配送與查詢訂單所需的資料，依商店的隱私政策處理。
- text: 選擇性使用
- text: ☐ 接收商店行銷資訊
- text: ☐ 允許廣告個人化使用
- text: 可隨時更新選擇，不影響已成立訂單。
- control: Action / 儲存選擇 {Kind=Primary} | text: 儲存選擇

### 29:950 Buyer Desktop · 確認抹除資料 · SHOPLINE · 預填示範 (1440px)
- chrome: Chrome D3
- text: 確認抹除資料
- text: 請先確認保留範圍
- text: 這會撤回你的選擇並結束購物連線；訂單、付款、退款與出貨紀錄仍會保留。
- control: Field / 輸入 ERASE 以確認 | text: 確認字樣 · 預填示範 / ERASE
- text: 確認字樣 · 預填示範
- control: Input | text: ERASE
- text: ERASE 為預填示範值；本原型不接收任意輸入。
- control: Action / 確認抹除我的資料 {Kind=Primary} | text: 查看完成狀態示範
- control: Action / 示範：結果未知 {Kind=Secondary} | text: 示範：結果未知
- control: Action / 取消 {Kind=Secondary} | text: 取消

### 29:983 Buyer Mobile · 確認抹除資料 · SHOPLINE · 預填示範 (390px)
- chrome: Chrome M3
- text: 確認抹除資料
- text: 請先確認保留範圍
- text: 這會撤回你的選擇並結束購物連線；訂單、付款、退款與出貨紀錄仍會保留。
- control: Field / 輸入 ERASE 以確認 | text: 確認字樣 · 預填示範 / ERASE
- text: 確認字樣 · 預填示範
- control: Input | text: ERASE
- text: ERASE 為預填示範值；本原型不接收任意輸入。
- control: Action / 確認抹除我的資料 {Kind=Primary} | text: 查看完成狀態示範
- control: Action / 示範：結果未知 {Kind=Secondary} | text: 示範：結果未知
- control: Action / 取消 {Kind=Secondary} | text: 取消

### 29:1011 Buyer Desktop · 資料已抹除 · SHOPLINE · 原型未寫入 (1440px)
- chrome: Chrome D3
- text: 資料抹除完成 · 示範
- text: 原型，未真實寫入
- text: 此頁只展示完成狀態，未抹除任何資料。正式抹除仍保留訂單、付款及出貨紀錄。
- control: Action / 返回商店 {Kind=Primary} | text: 返回商店

### 29:1040 Buyer Mobile · 資料已抹除 · SHOPLINE · 原型未寫入 (390px)
- chrome: Chrome M3
- text: 資料抹除完成 · 示範
- text: 原型，未真實寫入
- text: 此頁只展示完成狀態，未抹除任何資料。正式抹除仍保留訂單、付款及出貨紀錄。
- control: Action / 返回商店 {Kind=Primary} | text: 返回商店

### 37:1083 Buyer Desktop · 資料副本已下載 · SHOPLINE (1440px)
- chrome: Chrome D1
- text: 資料副本已下載
- text: 已完成下載
- text: 請在你的裝置中查看 my-data.json，並妥善保存。
- text: 下載資料不會刪除資料，也不會更改你的選擇。
- control: Action / 返回隱私與資料 {Kind=Primary} | text: 返回隱私與資料

### 37:1114 Buyer Mobile · 資料副本已下載 · SHOPLINE (390px)
- chrome: Chrome M1
- text: 資料副本已下載
- text: 已完成下載
- text: 請在你的裝置中查看 my-data.json，並妥善保存。
- text: 下載資料不會刪除資料，也不會更改你的選擇。
- control: Action / 返回隱私與資料 {Kind=Primary} | text: 返回隱私與資料

### 37:1140 Buyer Desktop · 資料副本暫時無法下載 · SHOPLINE (1440px)
- chrome: Chrome D1
- text: 資料副本暫時無法下載
- text: 下載未完成
- text: 請再試一次；若資料量過多，請聯絡購買的店鋪。
- control: Action / 再試一次 {Kind=Primary} | text: 再試一次
- control: Action / 聯絡店鋪 {Kind=Secondary} | text: 聯絡店鋪

### 37:1173 Buyer Mobile · 資料副本暫時無法下載 · SHOPLINE (390px)
- chrome: Chrome M1
- text: 資料副本暫時無法下載
- text: 下載未完成
- text: 請再試一次；若資料量過多，請聯絡購買的店鋪。
- control: Action / 再試一次 {Kind=Primary} | text: 再試一次
- control: Action / 聯絡店鋪 {Kind=Secondary} | text: 聯絡店鋪

### 37:1201 Buyer Desktop · 目前無法抹除資料 · SHOPLINE (1440px)
- chrome: Chrome D1
- text: 目前無法抹除資料
- text: 仍有進行中的付款或退款
- text: 請先完成或等待這些操作結束，再回來抹除資料。
- text: 資料尚未抹除，你仍可查看現有訂單。
- control: Action / 查看訂單 {Kind=Primary} | text: 查看訂單
- control: Action / 返回隱私與資料 {Kind=Secondary} | text: 返回隱私與資料

### 37:1236 Buyer Mobile · 目前無法抹除資料 · SHOPLINE (390px)
- chrome: Chrome M1
- text: 目前無法抹除資料
- text: 仍有進行中的付款或退款
- text: 請先完成或等待這些操作結束，再回來抹除資料。
- text: 資料尚未抹除，你仍可查看現有訂單。
- control: Action / 查看訂單 {Kind=Primary} | text: 查看訂單
- control: Action / 返回隱私與資料 {Kind=Secondary} | text: 返回隱私與資料

### 37:1266 Buyer Desktop · 正在確認抹除結果 · SHOPLINE (1440px)
- chrome: Chrome D1
- text: 正在確認抹除結果
- text: 尚未確認操作結果
- text: 請用「確認同一次操作」取回結果。不要重新建立另一筆要求。
- control: Action / 示範回應：已完成 {Kind=Primary} | text: 示範回應：已完成
- control: Action / 示範回應：被拒 {Kind=Secondary} | text: 示範回應：被拒
- control: Action / 示範回應：仍然未知 {Kind=Secondary} | text: 示範回應：仍然未知
- text: 請保留同一請求 key；下列為三種示範回應。
- control: Action / 暫時關閉 {Kind=Secondary} | text: 暫時關閉

### 37:1298 Buyer Mobile · 正在確認抹除結果 · SHOPLINE (390px)
- chrome: Chrome M1
- text: 正在確認抹除結果
- text: 尚未確認操作結果
- text: 請用「確認同一次操作」取回結果。不要重新建立另一筆要求。
- control: Action / 示範回應：已完成 {Kind=Primary} | text: 示範回應：已完成
- control: Action / 示範回應：被拒 {Kind=Secondary} | text: 示範回應：被拒
- control: Action / 示範回應：仍然未知 {Kind=Secondary} | text: 示範回應：仍然未知
- text: 請保留同一請求 key；下列為三種示範回應。
- control: Action / 暫時關閉 {Kind=Secondary} | text: 暫時關閉

### 1222:14056 DESIGN · Buyer Desktop · 門市取貨資訊 · 門市號碼空白（其餘資料已保留） (1440px)
- chrome: Chrome NONE (hidden text nodes skipped: 5)
- text: 門市取貨資訊
- text: 有 1 個欄位需要修正，其他資料已保留。
- text: 本單合計 NT$2,620（商品 NT$2,560 + 運費 NT$60），下單後線上付款。原型，未真實下單。
- control: Pickup fields | text: 收件人姓名 / 示範買家 / 手機號碼 / 09•• ••• 805 / 超商品牌（必填） / 7-ELEVEN / 門市名稱（選填） / 示範門市 / 門市號碼（必填） / 請輸入門市號碼 / 門市號碼不符合該超商的格式（7-ELEVEN：6 碼數字）。 / 不知道門市號碼？到超商官網查詢（7-ELEVEN：6 碼數字）。 / 門市地址（選填） / 縣市、區域、路名及門牌 / 我們只檢查格式，請確認門市存在且營業中。此處為示範資料，原型未真實下單。取件人姓名與手機會提供給物流商與門市，用於通知與取貨。
- control: Field / 收件人姓名 | text: 收件人姓名 / 示範買家
- text: 收件人姓名
- control: Input / 收件人姓名 | text: 示範買家
- control: Field / 手機號碼 | text: 手機號碼 / 09•• ••• 805
- text: 手機號碼
- control: Input / 手機號碼 | text: 09•• ••• 805
- control: Field / 門市類型 | text: 超商品牌（必填） / 7-ELEVEN
- text: 超商品牌（必填）
- control: Input / 門市類型 | text: 7-ELEVEN
- control: Field / 門市名稱 | text: 門市名稱（選填） / 示範門市
- text: 門市名稱（選填）
- control: Input / 門市名稱 | text: 示範門市
- control: Field / 門市編號 | text: 門市號碼（必填） / 請輸入門市號碼 / 門市號碼不符合該超商的格式（7-ELEVEN：6 碼數字）。
- text: 門市號碼（必填）
- control: Input / 門市編號 {State=Error} | text: 請輸入門市號碼
- text: 門市號碼不符合該超商的格式（7-ELEVEN：6 碼數字）。
- text: 不知道門市號碼？到超商官網查詢（7-ELEVEN：6 碼數字）。
- control: Field / 門市完整地址 | text: 門市地址（選填） / 縣市、區域、路名及門牌
- text: 門市地址（選填）
- control: Input / 門市完整地址 | text: 縣市、區域、路名及門牌
- text: 我們只檢查格式，請確認門市存在且營業中。此處為示範資料，原型未真實下單。取件人姓名與手機會提供給物流商與門市，用於通知與取貨。
- control: Action / 確認門市資訊 {Kind=Primary} | text: 確認門市資訊
- control: Action / 取消 {Kind=Secondary} | text: 取消

### 1222:14106 DESIGN · Buyer Mobile · 門市取貨資訊 · 門市號碼空白（其餘資料已保留） (390px)
- chrome: Chrome M3
- text: 門市取貨資訊
- text: 有 1 個欄位需要修正，其他資料已保留。
- text: 本單合計 NT$2,620（商品 NT$2,560 + 運費 NT$60），下單後線上付款。原型，未真實下單。
- control: Pickup fields | text: 收件人姓名 / 示範買家 / 手機號碼 / 09•• ••• 805 / 超商品牌（必填） / 7-ELEVEN / 門市名稱（選填） / 示範門市 / 門市號碼（必填） / 請輸入門市號碼 / 門市號碼不符合該超商的格式（7-ELEVEN：6 碼數字）。 / 不知道門市號碼？到超商官網查詢（7-ELEVEN：6 碼數字）。 / 門市地址（選填） / 縣市、區域、路名及門牌 / 我們只檢查格式，請確認門市存在且營業中。此處為示範資料，原型未真實下單。取件人姓名與手機會提供給物流商與門市，用於通知與取貨。
- control: Field / 收件人姓名 | text: 收件人姓名 / 示範買家
- text: 收件人姓名
- control: Input / 收件人姓名 | text: 示範買家
- control: Field / 手機號碼 | text: 手機號碼 / 09•• ••• 805
- text: 手機號碼
- control: Input / 手機號碼 | text: 09•• ••• 805
- control: Field / 門市類型 | text: 超商品牌（必填） / 7-ELEVEN
- text: 超商品牌（必填）
- control: Input / 門市類型 | text: 7-ELEVEN
- control: Field / 門市名稱 | text: 門市名稱（選填） / 示範門市
- text: 門市名稱（選填）
- control: Input / 門市名稱 | text: 示範門市
- control: Field / 門市編號 | text: 門市號碼（必填） / 請輸入門市號碼 / 門市號碼不符合該超商的格式（7-ELEVEN：6 碼數字）。
- text: 門市號碼（必填）
- control: Input / 門市編號 {State=Error} | text: 請輸入門市號碼
- text: 門市號碼不符合該超商的格式（7-ELEVEN：6 碼數字）。
- text: 不知道門市號碼？到超商官網查詢（7-ELEVEN：6 碼數字）。
- control: Action / 確認門市資訊 {Kind=Primary} | text: 確認門市資訊
- control: Action / 取消 {Kind=Secondary} | text: 取消
- control: Field / 門市完整地址 | text: 門市地址（選填） / 縣市、區域、路名及門牌
- text: 門市地址（選填）
- control: Input / 門市完整地址 | text: 縣市、區域、路名及門牌
- text: 我們只檢查格式，請確認門市存在且營業中。此處為示範資料，原型未真實下單。取件人姓名與手機會提供給物流商與門市，用於通知與取貨。

---

## U:b-claim (6 frames exported, 0 NOT_FOUND)

### 4:12347 Buyer Desktop · 確認直播認領 (1440px)
- chrome: Chrome D2
- text: 確認直播認領
- text: 已從留言帶入商品
- text: 輕量帆布托特包 · 焦糖
- text: 認領連結不代表已付款；價格與可售數量會在結帳時再次確認。\n本次認領焦糖 1 件；購物車原有米白 1 件，共 2 件一起結帳。
- text: 數量
- text: 1
- control: Action / 前往結帳（共 2 件） {Kind=Primary} | text: 前往結帳（共 2 件）
- text: NT$1,280
- text: 商品小計
- text: 商品示意
- control: Action / 查看商品 {Kind=Secondary} | text: 查看商品

### 4:12417 Buyer Mobile · 確認直播認領 (390px)
- chrome: Chrome NONE
- control: Action / ‹ {Kind=Secondary} | text: ‹
- text: 確認直播認領
- text: 輕量帆布托特包
- text: 焦糖 · 數量 1
- text: 商品示意
- text: NT$1,280
- text: 認領連結不代表已付款；價格與可售數量會在結帳時再次確認。\n本次認領焦糖 1 件；購物車原有米白 1 件，共 2 件一起結帳。
- control: Task / 查看商品 | text: 查看商品 / 圖片、規格與配送資訊 / ›
- control: Action / 前往結帳（共 2 件） {Kind=Primary} | text: 前往結帳（共 2 件）

### 66:1746 Buyer Desktop · 認領連結失效 (1440px)
- chrome: Chrome D2
- text: 認領連結失效
- text: 連結已失效或已更換，請私訊商家重新取得
- text: 此連結目前無法使用。購物車內已有的商品不會因此清空。
- control: Action / 查看購物車 {Kind=Secondary} | text: 查看購物車

### 66:1747 Buyer Mobile · 認領連結失效 (390px)
- chrome: Chrome NONE
- control: Action / ‹ {Kind=Secondary} | text: ‹
- text: 認領連結失效
- text: 連結已失效或已更換，請私訊商家重新取得
- text: 此連結目前無法使用。購物車內已有的商品不會因此清空。
- control: Action / 查看購物車 {Kind=Secondary} | text: 查看購物車

### 66:1754 Buyer Desktop · 認領結果待確認 (1440px)
- chrome: Chrome D2
- text: 認領結果待確認
- text: 無法確認結果
- text: 無法確認結果。請先重新讀取登記，再試一次。
- text: 先確認目前登記狀態，避免重複送出。
- control: Action / 重新讀取登記 {Kind=Primary} | text: 重新讀取登記

### 66:1755 Buyer Mobile · 認領結果待確認 (390px)
- chrome: Chrome NONE
- control: Action / ‹ {Kind=Secondary} | text: ‹
- text: 認領結果待確認
- text: 無法確認結果
- text: 無法確認結果。請先重新讀取登記，再試一次。
- text: 先確認目前登記狀態，避免重複送出。
- control: Action / 重新讀取登記 {Kind=Primary} | text: 重新讀取登記

---

## Observations (not interpretation of intent)

- No exported text matched placeholder patterns (Lorem, Text, Button, Label, Title, etc.).
- Layer name differs from visible text on several controls: `Action / ‹ 我的訂單` shows `‹ 查詢訂單`; `Action / 返回我的訂單` shows `返回查詢訂單`; `Action / ‹ 訂單 B-10482` shows `‹ 返回上一頁` (layer says B-10482, orders elsewhere are LC-10482); `Action / 隱私政策` shows `隱私與資料`; `Action / 確認抹除我的資料` shows `查看完成狀態示範`; `Action / 原型：模擬格式錯誤` is used for several unrelated 原型 toggles.
- 4:9963 is the only frame with a `K8 entry / 我的訂單` control (text `我的訂單`) next to `‹ 查詢訂單`.
- 1207:16447 and 1207:16465 contain the neutral-page message three times: English, a first zh string in mainland-style wording but traditional characters (“刷新訂單”, 標籤頁, 重新打開), and a second zh-TW string (「重新整理訂單」, 分頁, 重新開啟). Heading lines are only the English `Return to your store` / `Check your order`; the zh strings each start with the 返回商店查詢訂單 label.
- 1222:19371, 1222:19527, 1222:19657, 1222:19813 are native `window.confirm` illustrations; the frame itself says the dialog title and button text come from the browser and are not product copy (`取消` / `確定` inside `Buttons`).
- Example field values seen: `buyer@example.invalid`, `name@example.com`, `LC-10482`, `0123-ABCD-4567`, masked phone `09•• ••• ••01` / `09•• ••• 805`, `my-data.json`.
- `原型示範` / `示範` / `預填示範` prototype notices appear in the frames; they describe prototype behaviour, not production copy.
