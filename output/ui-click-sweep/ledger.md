# G-UI8 click ledger

Generated 2026-10-03T07:57:37.964Z. 120 page/viewport/locale units opened (0 with a page-load failure), 997 control clicks (977 pass, 0 fail, 20 skip), 18 journey steps (18 pass, 0 fail); failures: known 0, new 0; stale known-defect entries 0.

Every row is one real Playwright interaction (click / selectOption). Destructive and irreversible controls stop at their confirmation and are cancelled; `skip` rows name why.

| # | App | Page | Viewport | Locale | Scope | Control | Action | Expected | Actual | Result |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| r00001 | admin | /studio/claims | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00002 | admin | / | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00003 | admin | /studio | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00004 | admin | /studio/claims | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio/claims?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&scene=8c614f94-4396-4d79-aa00-dd23a855e3c1 -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb59598 | PASS |
| r00005 | admin | /studio | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00006 | admin | /studio/claims | desktop | zh-TW | main | 直播工作室 | click | visible change | url /zh-TW/studio/claims?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&scene=8c614f94-4396-4d79-aa00-dd23a855e3c1 -> /zh-TW/studio?store=091b7f7a-dd25-41b5-bb29-e5 | PASS |
| r00007 | admin | /studio | desktop | zh-TW | main | 留言關鍵字登記 (`studio-open-claims`) | click | visible change | url /zh-TW/studio?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/studio/claims?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&scene=8c614f94-4396-4d79-aa00-dd | PASS |
| r00008 | admin | /studio/claims | desktop | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (2 mutations) | PASS |
| r00009 | admin | /studio | desktop | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (18 mutations) | PASS |
| r00010 | admin | /studio/claims | desktop | zh-TW | main | 數量規則 | selectOption(KEYWORD_QTY_ONLY) | visible change | the control's own state changed (EXACT -> KEYWORD_QTY_ONLY) | PASS |
| r00011 | admin | /studio | desktop | zh-TW | main | ＋ 新增場次 | click | visible change | DOM changed (8 mutations) | PASS |
| r00012 | admin | /studio/claims | desktop | zh-TW | main | 開放登記窗口 | click | visible change | DOM changed (21 mutations) | PASS |
| r00013 | admin | /studio | desktop | zh-TW | main | Click sweep scene 7da39307f5b9 草稿 預定時間（台北時間，選填） | click | visible change | DOM changed (5 mutations) | PASS |
| r00014 | admin | /studio/claims | desktop | zh-TW | main | 這則貼文或直播所屬的專頁 | selectOption(13151449415657853791) | visible change | the control's own state changed ( -> 13151449415657853791) | PASS |
| r00015 | admin | /studio | desktop | zh-TW | main | 畫面比例 | selectOption(16:9) | visible change | DOM changed (2 mutations); the control's own state changed (9:16 -> 16:9) | PASS |
| r00016 | admin | /studio/claims | desktop | zh-TW | main | 平台 | selectOption(facebook) | visible change | the control's own state changed ( -> facebook) | PASS |
| r00017 | admin | /studio | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00018 | admin | / | desktop | zh-TW | skip | 跳至內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b#main; scrolled | PASS |
| r00019 | admin | /studio/claims | desktop | zh-TW | main | 回覆語言 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN) | PASS |
| r00020 | admin | /studio | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00021 | admin | / | desktop | zh-TW | topbar | 切換商店 (`shell-store-selector`) | selectOption | visible change | select has a single option | SKIP |
| r00022 | admin | /studio/claims | desktop | zh-TW | main | 傳送附帶購物車連結的私訊回覆 | click (toggle) | visible change | the control's own state changed (false -> true) | PASS |
| r00023 | admin | /studio | mobile | zh-TW | main | 留言關鍵字登記 (`studio-open-claims`) | click | visible change | url /zh-TW/studio?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/studio/claims?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&scene=8c614f94-4396-4d79-aa00-dd | PASS |
| r00024 | admin | / | desktop | zh-TW | topbar | 語言 (`locale-switch`) | selectOption(zh-CN) | visible change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-CN?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (17 mutations) | PASS |
| r00025 | admin | /studio/claims | desktop | zh-TW | main | 從此來源收集留言 | click (toggle) | visible change | the control's own state changed (true -> false) | PASS |
| r00026 | admin | /studio | mobile | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (18 mutations) | PASS |
| r00027 | admin | / | desktop | zh-TW | topbar | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00028 | admin | /studio/claims | desktop | zh-TW | main | 儲存留言來源 | click | visible change | DOM changed (2 mutations); validation shown | PASS |
| r00029 | admin | /studio | mobile | zh-TW | main | ＋ 新增場次 | click | visible change | DOM changed (8 mutations) | PASS |
| r00030 | admin | /studio/claims | desktop | zh-TW | main | 商品 | selectOption(33c4b6a8-a0a6-48fa-9238-4400c21907b2) | visible change | the control's own state changed ( -> 33c4b6a8-a0a6-48fa-9238-4400c21907b2); scrolled | PASS |
| r00031 | admin | / | desktop | zh-TW | layer of 說明 | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00032 | admin | /studio | mobile | zh-TW | main | Click sweep scene 7da39307f5b9 草稿 預定時間（台北時間，選填） | click | visible change | DOM changed (5 mutations) | PASS |
| r00033 | admin | /studio/claims | desktop | zh-TW | main | 新增關鍵字 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00034 | admin | / | desktop | zh-TW | topbar | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00035 | admin | /studio | mobile | zh-TW | main | 畫面比例 | selectOption(16:9) | visible change | DOM changed (2 mutations); the control's own state changed (9:16 -> 16:9); scrolled | PASS |
| r00036 | admin | /studio/claims | desktop | zh-TW | main | 加入關鍵字庫 (`claims-add-library`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00037 | admin | /studio | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00038 | admin | / | desktop | zh-TW | layer of 帳號 | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00039 | admin | /studio/claims | desktop | zh-TW | main | 買家 | selectOption | visible change | select has a single option | SKIP |
| r00040 | admin | /studio | desktop | en | main | Overview | click | navigation or in-page change | url /en/studio?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00041 | admin | / | desktop | zh-TW | layer of 帳號 | 登出 (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00042 | admin | /studio/claims | desktop | zh-TW | main | 記錄留言 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00043 | admin | /studio | desktop | en | main | Keyword claims (`studio-open-claims`) | click | visible change | url /en/studio?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/studio/claims?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&scene=8c614f94-4396-4d79-aa00-dd23a855 | PASS |
| r00044 | admin | / | desktop | zh-TW | rail | 總覽 (`nav-group-overview`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00045 | admin | /studio/claims | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00046 | admin | /studio | desktop | en | main | Refresh facts | click | visible change | DOM changed (18 mutations) | PASS |
| r00047 | admin | /studio/claims | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio/claims?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&scene=8c614f94-4396-4d79-aa00-dd23a855e3c1 -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb59598 | PASS |
| r00048 | admin | / | desktop | zh-TW | rail | 直播與貼文 (`nav-group-live`) | click | visible change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/studio?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (17 mutations) | PASS |
| r00049 | admin | /studio | desktop | en | main | ＋ New scene | click | visible change | DOM changed (8 mutations) | PASS |
| r00050 | admin | /studio/claims | mobile | zh-TW | main | 直播工作室 | click | visible change | url /zh-TW/studio/claims?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&scene=8c614f94-4396-4d79-aa00-dd23a855e3c1 -> /zh-TW/studio?store=091b7f7a-dd25-41b5-bb29-e5 | PASS |
| r00051 | admin | / | desktop | zh-TW | rail | 訂單與出貨 (`nav-orders`) | click | visible change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (19 mutations) | PASS |
| r00052 | admin | /studio | desktop | en | main | Click sweep scene 7da39307f5b9 Draft Scheduled time (Taipei time, optional) | click | visible change | DOM changed (5 mutations) | PASS |
| r00053 | admin | /studio/claims | mobile | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (2 mutations) | PASS |
| r00054 | admin | / | desktop | zh-TW | rail | 商品與庫存 + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00055 | admin | /studio | desktop | en | main | Canvas ratio | selectOption(16:9) | visible change | DOM changed (2 mutations); the control's own state changed (9:16 -> 16:9) | PASS |
| r00056 | admin | /studio/claims | mobile | zh-TW | main | 數量規則 | selectOption(KEYWORD_QTY_ONLY) | visible change | the control's own state changed (EXACT -> KEYWORD_QTY_ONLY) | PASS |
| r00057 | admin | /orders | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00058 | admin | / | desktop | zh-TW | rail | 顧客 (`nav-group-customers`) | click | visible change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00059 | admin | /studio/claims | mobile | zh-TW | main | 開放登記窗口 | click | visible change | DOM changed (21 mutations) | PASS |
| r00060 | admin | /orders | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00061 | admin | / | desktop | zh-TW | rail | 行銷優惠 + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00062 | admin | /studio/claims | mobile | zh-TW | main | 這則貼文或直播所屬的專頁 | selectOption(13151449415657853791) | visible change | the control's own state changed ( -> 13151449415657853791); scrolled | PASS |
| r00063 | admin | /orders | desktop | zh-TW | main | 訂單狀態 (`state-filter`) | selectOption(all) | visible change | DOM changed (8 mutations) | PASS |
| r00064 | admin | / | desktop | zh-TW | rail | 網路商店 (`nav-group-storefront`) | click | visible change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/design?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00065 | admin | /studio/claims | mobile | zh-TW | main | 平台 | selectOption(facebook) | visible change | the control's own state changed ( -> facebook); scrolled | PASS |
| r00066 | admin | /orders | desktop | zh-TW | main | 重新整理 (`orders-refresh`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00067 | admin | /studio/claims | mobile | zh-TW | main | 回覆語言 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN); scrolled | PASS |
| r00068 | admin | / | desktop | zh-TW | rail | 收款與報表 (`nav-group-finance`) | click | visible change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/finance?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (16 mutations) | PASS |
| r00069 | admin | /orders | desktop | zh-TW | main | 匯出未出貨訂單（CSV） (`orders-export`) | click | navigation or in-page change | download: unshipped-091b7f7a-202610030746.csv | PASS |
| r00070 | admin | /studio/claims | mobile | zh-TW | main | 傳送附帶購物車連結的私訊回覆 | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00071 | admin | / | desktop | zh-TW | rail | 設定 + (`nav-group-settings`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00072 | admin | /orders | desktop | zh-TW | main | 付款方式 (`orders-payment-filter`) | selectOption(card) | visible change | the control's own state changed ( -> card) | PASS |
| r00073 | admin | /orders | desktop | zh-TW | main | 配送方式 (`orders-delivery-filter`) | selectOption(home) | visible change | the control's own state changed ( -> home) | PASS |
| r00074 | admin | /studio/claims | mobile | zh-TW | main | 從此來源收集留言 | click (toggle) | visible change | the control's own state changed (true -> false); scrolled | PASS |
| r00075 | admin | /orders | desktop | zh-TW | main | 直播場次 (`orders-session-filter`) | selectOption | visible change | select has a single option | SKIP |
| r00076 | admin | / | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | DOM changed (10 mutations) | PASS |
| r00077 | admin | /studio/claims | mobile | zh-TW | main | 儲存留言來源 | click | visible change | DOM changed (2 mutations); validation shown; scrolled | PASS |
| r00078 | admin | /orders | desktop | zh-TW | main | 套用篩選 (`orders-apply`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00079 | admin | /studio/claims | mobile | zh-TW | main | 商品 | selectOption(33c4b6a8-a0a6-48fa-9238-4400c21907b2) | visible change | the control's own state changed ( -> 33c4b6a8-a0a6-48fa-9238-4400c21907b2); scrolled | PASS |
| r00080 | admin | / | desktop | zh-TW | main | 待確認的轉帳 1 (`todo-transfer`) | click | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?state=AWAITING_TRANSFER&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutat | PASS |
| r00081 | admin | /orders | desktop | zh-TW | main | 清除篩選 (`orders-reset`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (11 mutations); the control's own state changed ( -> gone) (inline change only; no request fired) | PASS |
| r00082 | admin | /studio/claims | mobile | zh-TW | main | 新增關鍵字 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00083 | admin | / | desktop | zh-TW | main | 待出貨訂單 2 (`todo-ship`) | click | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?state=unshipped&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (19 mutations) | PASS |
| r00084 | admin | /orders | desktop | zh-TW | main | 全部 5 (`orders-bucket-all`) | click | visible change | DOM changed (11 mutations); the control's own state changed (\|\|\|\|true -> gone) | PASS |
| r00085 | admin | /studio/claims | mobile | zh-TW | main | 加入關鍵字庫 (`claims-add-library`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00086 | admin | / | desktop | zh-TW | main | 待建立的超商托運單 1 (`todo-label`) | click | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?state=unshipped&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (19 mutations) | PASS |
| r00087 | admin | /studio/claims | mobile | zh-TW | main | 買家 | selectOption | visible change | select has a single option | SKIP |
| r00088 | admin | /orders | desktop | zh-TW | main | 待付款 4 (`orders-bucket-unpaid`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00089 | admin | / | desktop | zh-TW | main | 低庫存規格（5 件以下） 0 (`todo-stock`) | click | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (16 mutations) | PASS |
| r00090 | admin | /studio/claims | mobile | zh-TW | main | 記錄留言 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00091 | admin | /orders | desktop | zh-TW | main | 待核對轉帳 1 (`orders-bucket-transfer_review`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00092 | admin | / | desktop | zh-TW | main | 處理中的退款 0 (`todo-refunds`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) (inline change only; no request fired) | PASS |
| r00093 | admin | /studio/claims | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00094 | admin | /orders | desktop | zh-TW | main | 待出貨 2 (`orders-bucket-ready_to_ship`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00095 | admin | / | desktop | zh-TW | main | f4bbe624 | click | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?order=f4bbe624-447c-4103-a4ce-767cfd02f8f0&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DO | PASS |
| r00096 | admin | /studio/claims | desktop | en | main | Overview | click | navigation or in-page change | url /en/studio/claims?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&scene=8c614f94-4396-4d79-aa00-dd23a855e3c1 -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; D | PASS |
| r00097 | admin | /orders | desktop | zh-TW | main | 待交寄 1 (`orders-bucket-ready_to_consign`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00098 | admin | / | desktop | zh-TW | main | 2b6e84b7 | click | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?order=2b6e84b7-58ff-483d-a432-917418ce8925&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DO | PASS |
| r00099 | admin | /studio/claims | desktop | en | main | Live Studio | click | visible change | url /en/studio/claims?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&scene=8c614f94-4396-4d79-aa00-dd23a855e3c1 -> /en/studio?store=091b7f7a-dd25-41b5-bb29-e5cb5959 | PASS |
| r00100 | admin | /orders | desktop | zh-TW | main | 已出貨 0 (`orders-bucket-shipped`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00101 | admin | /studio/claims | desktop | en | main | Refresh facts | click | visible change | DOM changed (2 mutations) | PASS |
| r00102 | admin | / | desktop | zh-TW | main | de027d02 | click | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?order=de027d02-625d-46f1-b157-bb26624557fe&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DO | PASS |
| r00103 | admin | /orders | desktop | zh-TW | main | 已完成 0 (`orders-bucket-completed`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00104 | admin | /studio/claims | desktop | en | main | Quantity rule | selectOption(KEYWORD_QTY_ONLY) | visible change | the control's own state changed (EXACT -> KEYWORD_QTY_ONLY) | PASS |
| r00105 | admin | / | desktop | zh-TW | main | 502291b1 | click | navigation or in-page change | scrolled | PASS |
| r00106 | admin | /orders | desktop | zh-TW | main | 已取消 0 (`orders-bucket-cancelled`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00107 | admin | /studio/claims | desktop | en | main | Open claim window | click | visible change | DOM changed (21 mutations) | PASS |
| r00108 | admin | / | desktop | zh-TW | main | a8c6dd26 | click | navigation or in-page change | scrolled | PASS |
| r00109 | admin | /orders | desktop | zh-TW | main | 展開訂單詳情: f4bbe624-447c-4103-a4ce-767cfd02f8f0 (`order-expand-f4bbe624-447c-4103-a4ce-767cfd02f8f0`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00110 | admin | /studio/claims | desktop | en | main | Page for this post or live stream | selectOption(13151449415657853791) | visible change | the control's own state changed ( -> 13151449415657853791) | PASS |
| r00111 | admin | / | desktop | zh-TW | main | 136bb7a4 | click | navigation or in-page change | scrolled | PASS |
| r00112 | admin | /orders | desktop | zh-TW | main | 展開訂單詳情: 2b6e84b7-58ff-483d-a432-917418ce8925 (`order-expand-2b6e84b7-58ff-483d-a432-917418ce8925`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00113 | admin | /studio/claims | desktop | en | main | Platform | selectOption(facebook) | visible change | the control's own state changed ( -> facebook) | PASS |
| r00114 | admin | / | desktop | zh-TW | main | 全部訂單 | click | navigation or in-page change | scrolled | PASS |
| r00115 | admin | /studio/claims | desktop | en | main | Reply language | selectOption(zh-CN) | visible change | the control's own state changed (en -> zh-CN) | PASS |
| r00116 | admin | /orders | desktop | zh-TW | main | 展開訂單詳情: a8c6dd26-86b7-4eab-a686-9500ff48bc57 (`order-expand-a8c6dd26-86b7-4eab-a686-9500ff48bc57`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00117 | admin | / | desktop | zh-TW | main | 建立訂單 (`action-create-order`) | click | navigation or in-page change | scrolled | PASS |
| r00118 | admin | /studio/claims | desktop | en | main | Send a private reply with the cart link | click (toggle) | visible change | the control's own state changed (false -> true) | PASS |
| r00119 | admin | /orders | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00120 | admin | /orders | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00121 | admin | / | desktop | zh-TW | main | 匯入或匯出商品 (`action-import`) | click | navigation or in-page change | scrolled | PASS |
| r00122 | admin | /studio/claims | desktop | en | main | Collect comments from this source | click (toggle) | visible change | the control's own state changed (true -> false) | PASS |
| r00123 | admin | /orders | mobile | zh-TW | main | 更多篩選 (`orders-more-filters`) | click | visible change | DOM changed (3 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00124 | admin | / | desktop | zh-TW | main | 庫存 (`action-inventory`) | click | navigation or in-page change | scrolled | PASS |
| r00125 | admin | /studio/claims | desktop | en | main | Save comment source | click | visible change | DOM changed (2 mutations); validation shown | PASS |
| r00126 | admin | /orders | mobile | zh-TW | main | 重新整理 (`orders-refresh`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00127 | admin | / | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00128 | admin | /studio/claims | desktop | en | main | Product | selectOption(33c4b6a8-a0a6-48fa-9238-4400c21907b2) | visible change | the control's own state changed ( -> 33c4b6a8-a0a6-48fa-9238-4400c21907b2) | PASS |
| r00129 | admin | /orders | mobile | zh-TW | main | 匯出未出貨訂單（CSV） (`orders-export`) | click | navigation or in-page change | download: unshipped-091b7f7a-202610030746.csv | PASS |
| r00130 | admin | /studio/claims | desktop | en | main | Add offer | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00131 | admin | /orders | mobile | zh-TW | main | 套用篩選 (`orders-apply`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00132 | admin | /studio/claims | desktop | en | main | Add to library (`claims-add-library`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00133 | admin | /studio/claims | desktop | en | main | Buyer | selectOption | visible change | select has a single option | SKIP |
| r00134 | admin | /orders | mobile | zh-TW | main | 清除篩選 (`orders-reset`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (11 mutations); the control's own state changed ( -> gone) (inline change only; no request fired) | PASS |
| r00135 | admin | /studio/claims | desktop | en | main | Record comment | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00136 | admin | /orders | mobile | zh-TW | main | 全部 5 (`orders-bucket-all`) | click | visible change | DOM changed (11 mutations); the control's own state changed (\|\|\|\|true -> gone) | PASS |
| r00137 | admin | /orders/new | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00138 | admin | /orders | mobile | zh-TW | main | 待付款 4 (`orders-bucket-unpaid`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00139 | admin | /orders/new | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders/new?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00140 | admin | /orders | mobile | zh-TW | main | 待核對轉帳 1 (`orders-bucket-transfer_review`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00141 | admin | /orders/new | desktop | zh-TW | main | 搜尋 (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00142 | admin | / | mobile | zh-TW | skip | 跳至內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b#main; scrolled | PASS |
| r00143 | admin | /orders | mobile | zh-TW | main | 待出貨 2 (`orders-bucket-ready_to_ship`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00144 | admin | /orders/new | desktop | zh-TW | main | 配送方式 (`mo-option`) | selectOption(f1578d07-39a2-42f3-8430-3f45a6a9b924\|TW\|cvs90e1f4f6a0aa) | visible change | DOM changed (4 mutations); the control's own state changed ( -> f1578d07-39a2-42f3-8430-3f45a6a9b924\|TW\|cvs90e1f4f6a0aa) | PASS |
| r00145 | admin | /orders | mobile | zh-TW | main | 待交寄 1 (`orders-bucket-ready_to_consign`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00146 | admin | /orders/new | desktop | zh-TW | main | 顧客連結 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN) | PASS |
| r00147 | admin | / | mobile | zh-TW | topbar | 開啟導覽 | click | visible change | DOM changed (3 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00148 | admin | /orders | mobile | zh-TW | main | 展開訂單詳情: f4bbe624-447c-4103-a4ce-767cfd02f8f0 (`order-expand-f4bbe624-447c-4103-a4ce-767cfd02f8f0`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00149 | admin | / | mobile | zh-TW | topbar | 切換商店 (`shell-store-selector`) | selectOption | visible change | select has a single option | SKIP |
| r00150 | admin | /orders/new | desktop | zh-TW | main | 回到訂單 | click | navigation or in-page change | url /zh-TW/orders/new?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (4 mutations) | PASS |
| r00151 | admin | /orders | mobile | zh-TW | main | 展開訂單詳情: 2b6e84b7-58ff-483d-a432-917418ce8925 (`order-expand-2b6e84b7-58ff-483d-a432-917418ce8925`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00152 | admin | / | mobile | zh-TW | topbar | 語言 (`locale-switch`) | selectOption(zh-CN) | visible change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-CN?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (17 mutations) | PASS |
| r00153 | admin | /orders/new | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00154 | admin | /orders | mobile | zh-TW | main | 展開訂單詳情: a8c6dd26-86b7-4eab-a686-9500ff48bc57 (`order-expand-a8c6dd26-86b7-4eab-a686-9500ff48bc57`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00155 | admin | /orders/new | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders/new?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00156 | admin | / | mobile | zh-TW | topbar | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00157 | admin | /orders | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00158 | admin | /orders/new | mobile | zh-TW | main | 搜尋 (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00159 | admin | / | mobile | zh-TW | layer of 說明 | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00160 | admin | /orders | desktop | en | main | Overview | click | navigation or in-page change | url /en/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00161 | admin | /orders/new | mobile | zh-TW | main | 配送方式 (`mo-option`) | selectOption(f1578d07-39a2-42f3-8430-3f45a6a9b924\|TW\|cvs90e1f4f6a0aa) | visible change | DOM changed (4 mutations); the control's own state changed ( -> f1578d07-39a2-42f3-8430-3f45a6a9b924\|TW\|cvs90e1f4f6a0aa); scrolled | PASS |
| r00162 | admin | /orders | desktop | en | main | Order status (`state-filter`) | selectOption(all) | visible change | DOM changed (8 mutations) | PASS |
| r00163 | admin | / | mobile | zh-TW | topbar | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00164 | admin | /orders/new | mobile | zh-TW | main | 顧客連結 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN); scrolled | PASS |
| r00165 | admin | /orders | desktop | en | main | Refresh (`orders-refresh`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00166 | admin | /orders/new | mobile | zh-TW | main | 回到訂單 | click | navigation or in-page change | scrolled | PASS |
| r00167 | admin | / | mobile | zh-TW | layer of 帳號 | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00168 | admin | / | mobile | zh-TW | layer of 帳號 | 登出 (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00169 | admin | /orders | desktop | en | main | Export unshipped (CSV) (`orders-export`) | click | navigation or in-page change | download: unshipped-091b7f7a-202610030747.csv | PASS |
| r00170 | admin | /orders/new | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00171 | admin | /orders/new | desktop | en | main | Overview | click | navigation or in-page change | url /en/orders/new?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00172 | admin | /orders | desktop | en | main | Payment method (`orders-payment-filter`) | selectOption(card) | visible change | the control's own state changed ( -> card) | PASS |
| r00173 | admin | / | mobile | zh-TW | rail | 關閉導覽 | click | visible change | DOM changed (3 mutations) | PASS |
| r00174 | admin | /orders | desktop | en | main | Delivery method (`orders-delivery-filter`) | selectOption(home) | visible change | the control's own state changed ( -> home) | PASS |
| r00175 | admin | /orders/new | desktop | en | main | Search (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00176 | admin | /orders | desktop | en | main | Live session (`orders-session-filter`) | selectOption | visible change | select has a single option | SKIP |
| r00177 | admin | /orders/new | desktop | en | main | Delivery method (`mo-option`) | selectOption(f1578d07-39a2-42f3-8430-3f45a6a9b924\|TW\|cvs90e1f4f6a0aa) | visible change | DOM changed (4 mutations); the control's own state changed ( -> f1578d07-39a2-42f3-8430-3f45a6a9b924\|TW\|cvs90e1f4f6a0aa) | PASS |
| r00178 | admin | / | mobile | zh-TW | rail | 總覽 (`nav-group-overview`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00179 | admin | /orders/new | desktop | en | main | Customer link | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN) | PASS |
| r00180 | admin | /orders | desktop | en | main | Apply filters (`orders-apply`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00181 | admin | / | mobile | zh-TW | rail | 直播與貼文 (`nav-group-live`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00182 | admin | /orders | desktop | en | main | Clear filters (`orders-reset`) | click | visible change | DOM changed (11 mutations); the control's own state changed ( -> gone) | PASS |
| r00183 | admin | /orders/new | desktop | en | main | Back to orders | click | navigation or in-page change | url /en/orders/new?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (19 mutations) | PASS |
| r00184 | admin | / | mobile | zh-TW | rail | 訂單與出貨 (`nav-orders`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00185 | admin | /orders | desktop | en | main | All 5 (`orders-bucket-all`) | click | visible change | DOM changed (11 mutations); the control's own state changed (\|\|\|\|true -> gone) | PASS |
| r00186 | admin | /orders/cvs-print | desktop | zh-TW | page | (page load) | - | the route hands the browser to /ecpay/i | HTTP 200; POST https://logistics-stage.ecpay.com.tw/Express/PrintUniMartC2COrderInfo (blocked by the sweep) | PASS |
| r00187 | admin | /orders/cvs-print | mobile | zh-TW | page | (page load) | - | the route hands the browser to /ecpay/i | HTTP 200; POST https://logistics-stage.ecpay.com.tw/Express/PrintUniMartC2COrderInfo (blocked by the sweep) | PASS |
| r00188 | admin | /orders | desktop | en | main | Awaiting payment 4 (`orders-bucket-unpaid`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00189 | admin | / | mobile | zh-TW | rail | 商品與庫存 + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00190 | admin | /orders/cvs-print | desktop | en | page | (page load) | - | the route hands the browser to /ecpay/i | HTTP 200; POST https://logistics-stage.ecpay.com.tw/Express/PrintUniMartC2COrderInfo (blocked by the sweep) | PASS |
| r00191 | admin | /products | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00192 | admin | /orders | desktop | en | main | Review transfers 1 (`orders-bucket-transfer_review`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00193 | admin | /products | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00194 | admin | / | mobile | zh-TW | rail | 顧客 (`nav-group-customers`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00195 | admin | /orders | desktop | en | main | Ready to ship 2 (`orders-bucket-ready_to_ship`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00196 | admin | /products | desktop | zh-TW | main | 庫存帳 (`products-ledger-link`) | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (16 mutations) | PASS |
| r00197 | admin | / | mobile | zh-TW | rail | 行銷優惠 + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00198 | admin | /orders | desktop | en | main | Ready to drop off 1 (`orders-bucket-ready_to_consign`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00199 | admin | /products | desktop | zh-TW | main | 新增商品 (`product-new`) | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products/new?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (19 mutations) | PASS |
| r00200 | admin | / | mobile | zh-TW | rail | 網路商店 (`nav-group-storefront`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00201 | admin | /orders | desktop | en | main | Shipped 0 (`orders-bucket-shipped`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00202 | admin | /products | desktop | zh-TW | main | 全部 3 (`products-tab-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00203 | admin | / | mobile | zh-TW | rail | 收款與報表 (`nav-group-finance`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00204 | admin | /orders | desktop | en | main | Completed 0 (`orders-bucket-completed`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00205 | admin | /products | desktop | zh-TW | main | 草稿 0 (`products-tab-draft`) | click | visible change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&status=draft; DOM changed (17 mutat | PASS |
| r00206 | admin | / | mobile | zh-TW | rail | 設定 + (`nav-group-settings`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00207 | admin | /orders | desktop | en | main | Cancelled 0 (`orders-bucket-cancelled`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00208 | admin | /products | desktop | zh-TW | main | 上架中 3 (`products-tab-active`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&status=active; DOM changed (17 muta (inline change only; no request fired) | PASS |
| r00209 | admin | /orders | desktop | en | main | Show order details: f4bbe624-447c-4103-a4ce-767cfd02f8f0 (`order-expand-f4bbe624-447c-4103-a4ce-767cfd02f8f0`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00210 | admin | /products | desktop | zh-TW | main | 已封存 0 (`products-tab-archived`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&status=archived; DOM changed (17 mu (inline change only; no request fired) | PASS |
| r00211 | admin | / | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | DOM changed (10 mutations) | PASS |
| r00212 | admin | /orders | desktop | en | main | Show order details: 2b6e84b7-58ff-483d-a432-917418ce8925 (`order-expand-2b6e84b7-58ff-483d-a432-917418ce8925`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00213 | admin | /products | desktop | zh-TW | main | 搜尋 (`products-search-submit`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00214 | admin | / | mobile | zh-TW | main | 待確認的轉帳 1 (`todo-transfer`) | click | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?state=AWAITING_TRANSFER&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (14 mutat | PASS |
| r00215 | admin | /orders | desktop | en | main | Show order details: a8c6dd26-86b7-4eab-a686-9500ff48bc57 (`order-expand-a8c6dd26-86b7-4eab-a686-9500ff48bc57`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00216 | admin | /products | desktop | zh-TW | main | 全部 | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00217 | admin | / | mobile | zh-TW | main | 待出貨訂單 2 (`todo-ship`) | click | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?state=unshipped&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (22 mutations) | PASS |
| r00218 | admin | /products/[product] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00219 | admin | /products | desktop | zh-TW | main | 選取商品: Sweep Ceramic Mug | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00220 | admin | /products/[product] | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/bedede0d-a8c0-4e53-8715-1c08fbf8e8e4?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM ch | PASS |
| r00221 | admin | / | mobile | zh-TW | main | 待建立的超商托運單 1 (`todo-label`) | click | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?state=unshipped&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (14 mutations) | PASS |
| r00222 | admin | /products | desktop | zh-TW | main | 編輯: Sweep Ceramic Mug | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products/519e2d01-e9e6-424f-9505-2801bf6d8a2a?store=091b7f7a-dd25-41b5-bb29-e5cb5959879 | PASS |
| r00223 | admin | /products/[product] | desktop | zh-TW | main | ← 全部商品 (`product-back`) | click | navigation or in-page change | url /zh-TW/products/bedede0d-a8c0-4e53-8715-1c08fbf8e8e4?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879 | PASS |
| r00224 | admin | / | mobile | zh-TW | main | 低庫存規格（5 件以下） 0 (`todo-stock`) | click | navigation or in-page change | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (16 mutations) | PASS |
| r00225 | admin | /products | desktop | zh-TW | main | 修改價格: Sweep Ceramic Mug (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00226 | admin | /products/[product] | desktop | zh-TW | main | 商品圖片 | click | visible change | scrolled | PASS |
| r00227 | admin | / | mobile | zh-TW | main | 處理中的退款 0 (`todo-refunds`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (22 mutations) (inline change only; no request fired) | PASS |
| r00228 | admin | /products | desktop | zh-TW | main | 修改庫存: Sweep Ceramic Mug (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00229 | admin | /products/[product] | desktop | zh-TW | main | 基本資訊 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00230 | admin | / | mobile | zh-TW | main | f4bbe624 | click | navigation or in-page change | scrolled | PASS |
| r00231 | admin | /products | desktop | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products/519e2d01-e9e6-424f-9505-2801bf6d8a2a?store=091b7f7a-dd25-41b5-bb29-e5cb5959879 [1 of 3 alike] | PASS |
| r00232 | admin | /products/[product] | desktop | zh-TW | main | 價格與庫存 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00233 | admin | / | mobile | zh-TW | main | 2b6e84b7 | click | navigation or in-page change | scrolled | PASS |
| r00234 | admin | /products | desktop | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00235 | admin | /products/[product] | desktop | zh-TW | main | 規格 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00236 | admin | / | mobile | zh-TW | main | de027d02 | click | navigation or in-page change | scrolled | PASS |
| r00237 | admin | /products | desktop | zh-TW | main | 選取商品: Sweep Wool Scarf | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00238 | admin | /products/[product] | desktop | zh-TW | main | 分類 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00239 | admin | / | mobile | zh-TW | main | 502291b1 | click | navigation or in-page change | scrolled | PASS |
| r00240 | admin | /products | desktop | zh-TW | main | 編輯: Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products/33c4b6a8-a0a6-48fa-9238-4400c21907b2?store=091b7f7a-dd25-41b5-bb29-e5cb5959879 | PASS |
| r00241 | admin | /products/[product] | desktop | zh-TW | main | 物流 | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00242 | admin | / | mobile | zh-TW | main | a8c6dd26 | click | navigation or in-page change | scrolled | PASS |
| r00243 | admin | /products | desktop | zh-TW | main | 修改價格: Sweep Wool Scarf (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00244 | admin | /products | desktop | zh-TW | layer of 修改價格: Sweep Wool Scarf | 儲存修改 (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00245 | admin | /products/[product] | desktop | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00246 | admin | / | mobile | zh-TW | main | 136bb7a4 | click | navigation or in-page change | scrolled | PASS |
| r00247 | admin | /products | desktop | zh-TW | main | 修改庫存: Sweep Wool Scarf (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00248 | admin | /products/[product] | desktop | zh-TW | main | 圖片 ○ | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00249 | admin | / | mobile | zh-TW | main | 全部訂單 | click | navigation or in-page change | scrolled | PASS |
| r00250 | admin | /products | desktop | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products/33c4b6a8-a0a6-48fa-9238-4400c21907b2?store=091b7f7a-dd25-41b5-bb29-e5cb5959879 [1 of 3 alike] | PASS |
| r00251 | admin | /products/[product] | desktop | zh-TW | main | 商品名稱 ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00252 | admin | / | mobile | zh-TW | main | 建立訂單 (`action-create-order`) | click | navigation or in-page change | scrolled | PASS |
| r00253 | admin | /products | desktop | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00254 | admin | /products/[product] | desktop | zh-TW | main | 售價 ○ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00255 | admin | / | mobile | zh-TW | main | 匯入或匯出商品 (`action-import`) | click | navigation or in-page change | scrolled | PASS |
| r00256 | admin | /products | desktop | zh-TW | main | 選取商品: Sweep Cedar Candle | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00257 | admin | /products/[product] | desktop | zh-TW | main | 庫存 ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00258 | admin | / | mobile | zh-TW | main | 庫存 (`action-inventory`) | click | navigation or in-page change | scrolled | PASS |
| r00259 | admin | /products | desktop | zh-TW | main | 編輯: Sweep Cedar Candle | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products/bedede0d-a8c0-4e53-8715-1c08fbf8e8e4?store=091b7f7a-dd25-41b5-bb29-e5cb5959879 | PASS |
| r00260 | admin | / | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00261 | admin | /products/[product] | desktop | zh-TW | main | 圖片 ○ | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00262 | admin | /products | desktop | zh-TW | main | 修改價格: Sweep Cedar Candle (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00263 | admin | /products/[product] | desktop | zh-TW | main | 描述 ✓ | click | visible change | scrolled | PASS |
| r00264 | admin | /products | desktop | zh-TW | main | 修改庫存: Sweep Cedar Candle (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00265 | admin | /products/[product] | desktop | zh-TW | main | 分類 ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00266 | admin | /products | desktop | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products/bedede0d-a8c0-4e53-8715-1c08fbf8e8e4?store=091b7f7a-dd25-41b5-bb29-e5cb5959879 [1 of 3 alike] | PASS |
| r00267 | admin | /products/[product] | desktop | zh-TW | main | 直播關鍵字 ○ | click | visible change | scrolled | PASS |
| r00268 | admin | /products | desktop | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00269 | admin | /products/[product] | desktop | zh-TW | main | 搜尋引擎最佳化 ○ | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00270 | admin | /products | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00271 | admin | /products | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00272 | admin | /products/[product] | desktop | zh-TW | main | 顯示狀態上架中：商店發布後，買家可以看到並購買。 (`product-status`) | selectOption(draft) | confirmation layer opens; cancel; nothing changed | DOM changed (2 mutations); the control's own state changed (active -> draft) (inline change only; no request fired) | PASS |
| r00273 | admin | /products | mobile | zh-TW | main | 庫存帳 (`products-ledger-link`) | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (6 mutations) | PASS |
| r00274 | admin | /products/[product] | desktop | zh-TW | main | 移除 規格名稱 1 | click | confirmation layer opens; cancel; nothing changed | DOM changed (10 mutations); the control's own state changed ( -> true); scrolled (inline change only; no request fired) | PASS |
| r00275 | admin | / | desktop | en | skip | Skip to content | keyboard: Tab to the link, Enter | navigation or in-page change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b#main; scrolled | PASS |
| r00276 | admin | /products | mobile | zh-TW | main | 新增商品 (`product-new`) | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products/new?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (19 mutations) | PASS |
| r00277 | admin | /products/[product] | desktop | zh-TW | main | 新增規格（顏色、尺寸…） (`axis-add`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00278 | admin | / | desktop | en | topbar | Switch store (`shell-store-selector`) | selectOption | visible change | select has a single option | SKIP |
| r00279 | admin | /products | mobile | zh-TW | main | 全部 3 (`products-tab-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00280 | admin | /products/[product] | desktop | zh-TW | main | 售價 · 批量填入 (`bulk-price`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00281 | admin | / | desktop | en | topbar | Language (`locale-switch`) | selectOption(zh-CN) | visible change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-CN?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (17 mutations) | PASS |
| r00282 | admin | /products | mobile | zh-TW | main | 草稿 0 (`products-tab-draft`) | click | visible change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&status=draft; DOM changed (17 mutat | PASS |
| r00283 | admin | /products/[product] | desktop | zh-TW | main | 原價 · 批量填入 (`bulk-compare`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00284 | admin | / | desktop | en | topbar | Help | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00285 | admin | /products | mobile | zh-TW | main | 上架中 3 (`products-tab-active`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&status=active; DOM changed (17 muta (inline change only; no request fired) | PASS |
| r00286 | admin | /products/[product] | desktop | zh-TW | main | 數量 · 批量填入 (`bulk-quantity`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00287 | admin | / | desktop | en | layer of Help | Help | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00288 | admin | /products | mobile | zh-TW | main | 已封存 0 (`products-tab-archived`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&status=archived; DOM changed (17 mu (inline change only; no request fired) | PASS |
| r00289 | admin | /products/[product] | desktop | zh-TW | main | 貨號 · 批量填入 (`bulk-code`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00290 | admin | / | desktop | en | topbar | Account | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00291 | admin | /products | mobile | zh-TW | main | 搜尋 (`products-search-submit`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00292 | admin | /products/[product] | desktop | zh-TW | main | 直播關鍵字 · 批量填入 (`bulk-keyword`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00293 | admin | / | desktop | en | layer of Account | Account | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00294 | admin | /products | mobile | zh-TW | main | 全部 | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00295 | admin | /products/[product] | desktop | zh-TW | main | 不追蹤 ∞ | click (toggle) | visible change | DOM changed (5 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00296 | admin | / | desktop | en | layer of Account | Sign out (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00297 | admin | /products | mobile | zh-TW | main | 選取商品: Sweep Ceramic Mug | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00298 | admin | /products/[product] | desktop | zh-TW | main | 啟用 1 (`matrix-active-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00299 | admin | / | desktop | en | rail | Overview (`nav-group-overview`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00300 | admin | /products | mobile | zh-TW | main | 編輯: Sweep Ceramic Mug | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products/519e2d01-e9e6-424f-9505-2801bf6d8a2a?store=091b7f7a-dd25-41b5-bb29-e5cb5959879 | PASS |
| r00301 | admin | /products/[product] | desktop | zh-TW | main | Sweep home | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00302 | admin | / | desktop | en | rail | Live & posts (`nav-group-live`) | click | visible change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/studio?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (12 mutations) | PASS |
| r00303 | admin | /products | mobile | zh-TW | main | 修改價格: Sweep Ceramic Mug (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00304 | admin | /products/[product] | desktop | zh-TW | main | Sweep wear | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00305 | admin | / | desktop | en | rail | Orders & shipping (`nav-orders`) | click | visible change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00306 | admin | /products | mobile | zh-TW | main | 修改庫存: Sweep Ceramic Mug (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00307 | admin | /products/[product] | desktop | zh-TW | main | 管理分類 | click | navigation or in-page change | scrolled | PASS |
| r00308 | admin | / | desktop | en | rail | Products & inventory + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00309 | admin | /products | mobile | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products/519e2d01-e9e6-424f-9505-2801bf6d8a2a?store=091b7f7a-dd25-41b5-bb29-e5cb5959879 [1 of 3 alike] | PASS |
| r00310 | admin | /products/[product] | desktop | zh-TW | main | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00311 | admin | / | desktop | en | rail | Customers (`nav-group-customers`) | click | visible change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (16 mutations) | PASS |
| r00312 | admin | /products | mobile | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00313 | admin | /products/[product] | desktop | zh-TW | layer of 物流 | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00314 | admin | / | desktop | en | rail | Marketing + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00315 | admin | /products | mobile | zh-TW | main | 選取商品: Sweep Wool Scarf | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ); scrolled | PASS |
| r00316 | admin | /products/[product] | desktop | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00317 | admin | / | desktop | en | rail | Online store (`nav-group-storefront`) | click | visible change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/design?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00318 | admin | /products | mobile | zh-TW | main | 編輯: Sweep Wool Scarf | click | navigation or in-page change | scrolled | PASS |
| r00319 | admin | /products/[product] | desktop | zh-TW | layer of 搜尋引擎最佳化 | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00320 | admin | / | desktop | en | rail | Payments & reports (`nav-group-finance`) | click | visible change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/finance?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00321 | admin | /products | mobile | zh-TW | main | 修改價格: Sweep Wool Scarf (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations); scrolled | PASS |
| r00322 | admin | /products/[product] | desktop | zh-TW | main | 儲存修改 (`product-save`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00323 | admin | / | desktop | en | rail | Settings + (`nav-group-settings`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00324 | admin | /products | mobile | zh-TW | main | 修改庫存: Sweep Wool Scarf (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations); scrolled | PASS |
| r00325 | admin | /products/[product] | desktop | zh-TW | main | 下架轉草稿 (`product-unpublish`) | click | confirmation layer opens; cancel; nothing changed | dialog: confirm: 確定下架此商品？買家將無法再購買此商品。 (confirmation shown, then cancelled) | PASS |
| r00326 | admin | /products | mobile | zh-TW | main | 編輯 | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00327 | admin | / | desktop | en | main | Overview | click | navigation or in-page change | DOM changed (10 mutations) | PASS |
| r00328 | admin | /products/[product] | desktop | zh-TW | main | 上架 (`product-publish`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (1 mutations) (inline change only; no request fired) | PASS |
| r00329 | admin | /products | mobile | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations); scrolled [1 of 3 alike] | PASS |
| r00330 | admin | / | desktop | en | main | Transfers to confirm 1 (`todo-transfer`) | click | navigation or in-page change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/orders?state=AWAITING_TRANSFER&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (19 mutations) | PASS |
| r00331 | admin | /products/[product] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00332 | admin | /products | mobile | zh-TW | main | 選取商品: Sweep Cedar Candle | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ); scrolled | PASS |
| r00333 | admin | /products/[product] | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/bedede0d-a8c0-4e53-8715-1c08fbf8e8e4?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM ch | PASS |
| r00334 | admin | / | desktop | en | main | Orders to ship 2 (`todo-ship`) | click | navigation or in-page change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/orders?state=unshipped&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00335 | admin | /products | mobile | zh-TW | main | 編輯: Sweep Cedar Candle | click | navigation or in-page change | scrolled | PASS |
| r00336 | admin | /products/[product] | mobile | zh-TW | main | ← 全部商品 (`product-back`) | click | navigation or in-page change | url /zh-TW/products/bedede0d-a8c0-4e53-8715-1c08fbf8e8e4?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879 | PASS |
| r00337 | admin | / | desktop | en | main | Convenience-store labels to create 1 (`todo-label`) | click | navigation or in-page change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/orders?state=unshipped&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00338 | admin | /products | mobile | zh-TW | main | 修改價格: Sweep Cedar Candle (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (37 mutations); scrolled | PASS |
| r00339 | admin | /products/[product] | mobile | zh-TW | main | 商品圖片 | click | visible change | scrolled | PASS |
| r00340 | admin | / | desktop | en | main | Low-stock variants (5 or fewer) 0 (`todo-stock`) | click | navigation or in-page change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (16 mutations) | PASS |
| r00341 | admin | /products | mobile | zh-TW | main | 修改庫存: Sweep Cedar Candle (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (39 mutations); scrolled | PASS |
| r00342 | admin | /products/[product] | mobile | zh-TW | main | 基本資訊 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00343 | admin | / | desktop | en | main | Open refunds 0 (`todo-refunds`) | click | navigation or in-page change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00344 | admin | /products | mobile | zh-TW | main | 編輯 | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00345 | admin | /products/[product] | mobile | zh-TW | main | 價格與庫存 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00346 | admin | / | desktop | en | main | f4bbe624 | click | navigation or in-page change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/orders?order=f4bbe624-447c-4103-a4ce-767cfd02f8f0&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM chan | PASS |
| r00347 | admin | /products | mobile | zh-TW | main | 複製 | click | visible change | DOM changed (36 mutations); scrolled [1 of 3 alike] | PASS |
| r00348 | admin | /products/[product] | mobile | zh-TW | main | 規格 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00349 | admin | / | desktop | en | main | 2b6e84b7 | click | navigation or in-page change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/orders?order=2b6e84b7-58ff-483d-a432-917418ce8925&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM chan | PASS |
| r00350 | admin | /products | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00351 | admin | /products/[product] | mobile | zh-TW | main | 分類 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00352 | admin | /products | desktop | en | main | Overview | click | navigation or in-page change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00353 | admin | / | desktop | en | main | de027d02 | click | navigation or in-page change | url /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/orders?order=de027d02-625d-46f1-b157-bb26624557fe&store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM chan | PASS |
| r00354 | admin | /products/[product] | mobile | zh-TW | main | 物流 | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00355 | admin | /products | desktop | en | main | Inventory ledger (`products-ledger-link`) | click | navigation or in-page change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (16 mutations) | PASS |
| r00356 | admin | / | desktop | en | main | 502291b1 | click | navigation or in-page change | scrolled | PASS |
| r00357 | admin | /products/[product] | mobile | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00358 | admin | /products | desktop | en | main | Add product (`product-new`) | click | navigation or in-page change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products/new?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (19 mutations) | PASS |
| r00359 | admin | / | desktop | en | main | a8c6dd26 | click | navigation or in-page change | scrolled | PASS |
| r00360 | admin | /products/[product] | mobile | zh-TW | main | 顯示狀態上架中：商店發布後，買家可以看到並購買。 (`product-status`) | selectOption(draft) | confirmation layer opens; cancel; nothing changed | DOM changed (2 mutations); the control's own state changed (active -> draft) (inline change only; no request fired) | PASS |
| r00361 | admin | / | desktop | en | main | 136bb7a4 | click | navigation or in-page change | scrolled | PASS |
| r00362 | admin | /products | desktop | en | main | All 4 (`products-tab-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00363 | admin | /products/[product] | mobile | zh-TW | main | 移除 規格名稱 1 | click | confirmation layer opens; cancel; nothing changed | DOM changed (10 mutations); the control's own state changed ( -> true); scrolled (inline change only; no request fired) | PASS |
| r00364 | admin | / | desktop | en | main | All orders | click | navigation or in-page change | scrolled | PASS |
| r00365 | admin | /products | desktop | en | main | Draft 1 (`products-tab-draft`) | click | visible change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&status=draft; DOM changed (17 mutations) | PASS |
| r00366 | admin | /products/[product] | mobile | zh-TW | main | 新增規格（顏色、尺寸…） (`axis-add`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00367 | admin | / | desktop | en | main | Create an order (`action-create-order`) | click | navigation or in-page change | scrolled | PASS |
| r00368 | admin | /products | desktop | en | main | Active 3 (`products-tab-active`) | click | visible change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&status=active; DOM changed (17 mutations) | PASS |
| r00369 | admin | /products/[product] | mobile | zh-TW | main | 售價 · 批量填入 (`bulk-price`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00370 | admin | / | desktop | en | main | Import or export products (`action-import`) | click | navigation or in-page change | scrolled | PASS |
| r00371 | admin | /products | desktop | en | main | Archived 0 (`products-tab-archived`) | click | visible change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&status=archived; DOM changed (17 mutation | PASS |
| r00372 | admin | /products/[product] | mobile | zh-TW | main | 原價 · 批量填入 (`bulk-compare`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00373 | admin | / | desktop | en | main | Inventory (`action-inventory`) | click | navigation or in-page change | scrolled | PASS |
| r00374 | admin | /products | desktop | en | main | Search (`products-search-submit`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00375 | admin | /products/[product] | mobile | zh-TW | main | 數量 · 批量填入 (`bulk-quantity`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00376 | admin | /collections | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00377 | admin | /products | desktop | en | main | All | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00378 | admin | /products/[product] | mobile | zh-TW | main | 貨號 · 批量填入 (`bulk-code`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00379 | admin | /collections | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/collections?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00380 | admin | /products | desktop | en | main | Select product: Sweep Wool Scarf（复制） | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00381 | admin | /products/[product] | mobile | zh-TW | main | 直播關鍵字 · 批量填入 (`bulk-keyword`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00382 | admin | /collections | desktop | zh-TW | main | 新增集合 (`collection-new`) | click | visible change | DOM changed (3 mutations); validation shown | PASS |
| r00383 | admin | /products | desktop | en | main | Edit: Sweep Wool Scarf（复制） | click | navigation or in-page change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products/44cf51e7-1df1-4109-b891-0fecc3be1716?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM | PASS |
| r00384 | admin | /products/[product] | mobile | zh-TW | main | 不追蹤 ∞ | click (toggle) | visible change | DOM changed (5 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00385 | admin | /collections | desktop | zh-TW | main | Sweep home /sweep-home · 2 件商品 (`collection-item-9de26185-b9a2-44b9-a4cd-48f0de818340`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00386 | admin | /products | desktop | en | main | Edit price: Sweep Wool Scarf（复制） (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (39 mutations) | PASS |
| r00387 | admin | /products | desktop | en | layer of Edit price: Sweep Wool Scarf（复制） | Save changes (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00388 | admin | /collections | desktop | zh-TW | main | Sweep wear /sweep-wear · 2 件商品 (`collection-item-7e1e49a2-3b3b-4a70-9f62-171fe9f26612`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00389 | admin | /products/[product] | mobile | zh-TW | main | 啟用 1 (`matrix-active-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00390 | admin | /products | desktop | en | main | Edit inventory: Sweep Wool Scarf（复制） (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (37 mutations) | PASS |
| r00391 | admin | /collections | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00392 | admin | /products/[product] | mobile | zh-TW | main | Sweep home | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00393 | admin | /collections | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/collections?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00394 | admin | /products | desktop | en | layer of Edit inventory: Sweep Wool Scarf（复制） | Do not track ∞ (`quick-untracked-0`) | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (false -> true) | PASS |
| r00395 | admin | /products | desktop | en | layer of Edit inventory: Sweep Wool Scarf（复制） | Save changes (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00396 | admin | /products/[product] | mobile | zh-TW | main | Sweep wear | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00397 | admin | /collections | mobile | zh-TW | main | 新增集合 (`collection-new`) | click | visible change | DOM changed (3 mutations); validation shown | PASS |
| r00398 | admin | /products | desktop | en | main | Edit | click | navigation or in-page change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products/44cf51e7-1df1-4109-b891-0fecc3be1716?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM [1 of 4 alike] | PASS |
| r00399 | admin | /products/[product] | mobile | zh-TW | main | 管理分類 | click | navigation or in-page change | scrolled | PASS |
| r00400 | admin | /collections | mobile | zh-TW | main | Sweep home /sweep-home · 2 件商品 (`collection-item-9de26185-b9a2-44b9-a4cd-48f0de818340`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00401 | admin | /products | desktop | en | main | Copy | click | visible change | DOM changed (36 mutations) [1 of 4 alike] | PASS |
| r00402 | admin | /products/[product] | mobile | zh-TW | main | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00403 | admin | /collections | mobile | zh-TW | main | Sweep wear /sweep-wear · 2 件商品 (`collection-item-7e1e49a2-3b3b-4a70-9f62-171fe9f26612`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00404 | admin | /products | desktop | en | main | Select product: Sweep Ceramic Mug | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00405 | admin | /collections | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00406 | admin | /products/[product] | mobile | zh-TW | layer of 物流 | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00407 | admin | /collections | desktop | en | main | Overview | click | navigation or in-page change | url /en/collections?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00408 | admin | /products | desktop | en | main | Edit: Sweep Ceramic Mug | click | navigation or in-page change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products/519e2d01-e9e6-424f-9505-2801bf6d8a2a?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM | PASS |
| r00409 | admin | /products/[product] | mobile | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00410 | admin | /collections | desktop | en | main | New collection (`collection-new`) | click | visible change | DOM changed (3 mutations); validation shown | PASS |
| r00411 | admin | /products | desktop | en | main | Edit price: Sweep Ceramic Mug (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (37 mutations) | PASS |
| r00412 | admin | /products/[product] | mobile | zh-TW | layer of 搜尋引擎最佳化 | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00413 | admin | /collections | desktop | en | main | Sweep home /sweep-home · 2 products (`collection-item-9de26185-b9a2-44b9-a4cd-48f0de818340`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00414 | admin | /products | desktop | en | main | Edit inventory: Sweep Ceramic Mug (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (37 mutations) | PASS |
| r00415 | admin | /products/[product] | mobile | zh-TW | main | 儲存修改 (`product-save`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00416 | admin | /collections | desktop | en | main | Sweep wear /sweep-wear · 2 products (`collection-item-7e1e49a2-3b3b-4a70-9f62-171fe9f26612`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00417 | admin | /products | desktop | en | main | Edit | click | navigation or in-page change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products/519e2d01-e9e6-424f-9505-2801bf6d8a2a?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM [1 of 4 alike] | PASS |
| r00418 | admin | /inventory | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00419 | admin | /products/[product] | mobile | zh-TW | main | 下架轉草稿 (`product-unpublish`) | click | confirmation layer opens; cancel; nothing changed | dialog: confirm: 確定下架此商品？買家將無法再購買此商品。 (confirmation shown, then cancelled) | PASS |
| r00420 | admin | /products | desktop | en | main | Copy | click | visible change | DOM changed (36 mutations) [1 of 4 alike] | PASS |
| r00421 | admin | /inventory | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00422 | admin | /products/[product] | mobile | zh-TW | main | 上架 (`product-publish`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (1 mutations) (inline change only; no request fired) | PASS |
| r00423 | admin | /products | desktop | en | main | Select product: Sweep Wool Scarf | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00424 | admin | /inventory | desktop | zh-TW | main | 新增商品 | click | navigation or in-page change | url /zh-TW/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products/new?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00425 | admin | /products/[product] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00426 | admin | /products | desktop | en | main | Edit: Sweep Wool Scarf | click | navigation or in-page change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products/33c4b6a8-a0a6-48fa-9238-4400c21907b2?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM | PASS |
| r00427 | admin | /products/[product] | desktop | en | main | Overview | click | navigation or in-page change | url /en/products/bedede0d-a8c0-4e53-8715-1c08fbf8e8e4?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed  | PASS |
| r00428 | admin | /inventory | desktop | zh-TW | main | 查詢 | click | visible change | DOM changed (2 mutations) | PASS |
| r00429 | admin | /products | desktop | en | main | Edit price: Sweep Wool Scarf (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (37 mutations) | PASS |
| r00430 | admin | /inventory | desktop | zh-TW | main | 倉庫 | selectOption | visible change | select has a single option | SKIP |
| r00431 | admin | /products/[product] | desktop | en | main | ← All products (`product-back`) | click | navigation or in-page change | url /en/products/bedede0d-a8c0-4e53-8715-1c08fbf8e8e4?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM | PASS |
| r00432 | admin | /inventory | desktop | zh-TW | main | 狀態 | selectOption(active) | visible change | url /zh-TW/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&status=active; DOM changed (2 mut | PASS |
| r00433 | admin | /products | desktop | en | main | Edit inventory: Sweep Wool Scarf (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (37 mutations) | PASS |
| r00434 | admin | /products/[product] | desktop | en | main | Product images | click | visible change | scrolled | PASS |
| r00435 | admin | /products | desktop | en | main | Select product: Sweep Cedar Candle | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00436 | admin | /inventory | desktop | zh-TW | main | 重設 | click | visible change | DOM changed (2 mutations) | PASS |
| r00437 | admin | /products/[product] | desktop | en | main | Basic information | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00438 | admin | /products | desktop | en | main | Edit: Sweep Cedar Candle | click | navigation or in-page change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products/bedede0d-a8c0-4e53-8715-1c08fbf8e8e4?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM | PASS |
| r00439 | admin | /inventory | desktop | zh-TW | main | 重新整理 | click | visible change | DOM changed (2 mutations) | PASS |
| r00440 | admin | /products/[product] | desktop | en | main | Price & inventory | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00441 | admin | /products | desktop | en | main | Edit price: Sweep Cedar Candle (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (37 mutations) | PASS |
| r00442 | admin | /inventory | desktop | zh-TW | main | 選取 sweep-wool-scarf-2 | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00443 | admin | /products/[product] | desktop | en | main | Variants | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00444 | admin | /products | desktop | en | main | Edit inventory: Sweep Cedar Candle (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (37 mutations) | PASS |
| r00445 | admin | /inventory | desktop | zh-TW | main | Sweep Wool Scarf（复制） | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00446 | admin | /products/[product] | desktop | en | main | Collections | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00447 | admin | /products | desktop | en | main | Edit | click | navigation or in-page change | url /en/products?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products/bedede0d-a8c0-4e53-8715-1c08fbf8e8e4?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM [1 of 4 alike] | PASS |
| r00448 | admin | /inventory | desktop | zh-TW | main | 選取 | click | visible change | DOM changed (6 mutations); validation shown [1 of 4 alike] | PASS |
| r00449 | admin | /products/[product] | desktop | en | main | Shipping | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00450 | admin | /products | desktop | en | main | Copy | click | visible change | DOM changed (36 mutations) [1 of 4 alike] | PASS |
| r00451 | admin | /inventory | desktop | zh-TW | main | 選取 SWP-MUG | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00452 | admin | /products/[product] | desktop | en | main | Search engine listing | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00453 | admin | /products/import | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00454 | admin | /inventory | desktop | zh-TW | main | Sweep Ceramic Mug | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00455 | admin | /products/import | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/import?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00456 | admin | /products/[product] | desktop | en | main | Images ○ | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00457 | admin | /inventory | desktop | zh-TW | main | 選取 | click | visible change | DOM changed (6 mutations); validation shown [1 of 4 alike] | PASS |
| r00458 | admin | /products/import | desktop | zh-TW | main | 下載 CSV (`import-export`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00459 | admin | /products/[product] | desktop | en | main | Product name ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00460 | admin | /inventory | desktop | zh-TW | main | 選取 SWP-SCF | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00461 | admin | /products/import | desktop | zh-TW | main | 回到總覽 | click | navigation or in-page change | url /zh-TW/products/import?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00462 | admin | /products/[product] | desktop | en | main | Price ○ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00463 | admin | /inventory | desktop | zh-TW | main | Sweep Wool Scarf | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00464 | admin | /products/import | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00465 | admin | /products/[product] | desktop | en | main | Inventory ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00466 | admin | /products/import | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/import?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (4 mutations) | PASS |
| r00467 | admin | /inventory | desktop | zh-TW | main | 選取 T04-60e6d9a23cc9-0 | click | visible change | DOM changed (6 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00468 | admin | /products/[product] | desktop | en | main | Images ○ | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00469 | admin | /products/import | mobile | zh-TW | main | 下載 CSV (`import-export`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00470 | admin | /inventory | desktop | zh-TW | main | Sweep Cedar Candle | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00471 | admin | /products/import | mobile | zh-TW | main | 回到總覽 | click | navigation or in-page change | scrolled | PASS |
| r00472 | admin | /products/[product] | desktop | en | main | Description ✓ | click | visible change | scrolled | PASS |
| r00473 | admin | /inventory | desktop | zh-TW | main | 選取 | click | visible change | DOM changed (4 mutations); validation shown [1 of 4 alike] | PASS |
| r00474 | admin | /products/import | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00475 | admin | /inventory | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00476 | admin | /products/[product] | desktop | en | main | Collections ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00477 | admin | /products/import | desktop | en | main | Overview | click | navigation or in-page change | url /en/products/import?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00478 | admin | /inventory | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00479 | admin | /products/[product] | desktop | en | main | Live keyword ○ | click | visible change | scrolled | PASS |
| r00480 | admin | /products/import | desktop | en | main | Download CSV (`import-export`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00481 | admin | /inventory | mobile | zh-TW | main | 新增商品 | click | navigation or in-page change | url /zh-TW/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/products/new?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (19 mutations) | PASS |
| r00482 | admin | /products/[product] | desktop | en | main | Search engine listing ○ | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00483 | admin | /products/import | desktop | en | main | Back to dashboard | click | navigation or in-page change | url /en/products/import?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00484 | admin | /inventory | mobile | zh-TW | main | 查詢 | click | visible change | DOM changed (2 mutations) | PASS |
| r00485 | admin | /products/[product] | desktop | en | main | VisibilityActive: shoppers can see and buy it once the store is published. (`product-status`) | selectOption(draft) | visible change | DOM changed (2 mutations); the control's own state changed (active -> draft) | PASS |
| r00486 | admin | /customers | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00487 | admin | /inventory | mobile | zh-TW | main | 倉庫 | selectOption | visible change | select has a single option | SKIP |
| r00488 | admin | /customers | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00489 | admin | /products/[product] | desktop | en | main | Remove Option name 1 | click | confirmation layer opens; cancel; nothing changed | DOM changed (10 mutations); the control's own state changed ( -> true); scrolled (inline change only; no request fired) | PASS |
| r00490 | admin | /inventory | mobile | zh-TW | main | 狀態 | selectOption(active) | visible change | url /zh-TW/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&status=active; DOM changed (2 mut | PASS |
| r00491 | admin | /customers | desktop | zh-TW | main | 搜尋 (`customers-search-submit`) | click | visible change | DOM changed (7 mutations) | PASS |
| r00492 | admin | /products/[product] | desktop | en | main | Add option (color, size…) (`axis-add`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00493 | admin | /inventory | mobile | zh-TW | main | 重設 | click | visible change | DOM changed (2 mutations) | PASS |
| r00494 | admin | /customers | desktop | zh-TW | main | 重新整理 (`customers-refresh`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00495 | admin | /products/[product] | desktop | en | main | Price · Bulk fill (`bulk-price`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00496 | admin | /inventory | mobile | zh-TW | main | 重新整理 | click | visible change | DOM changed (2 mutations) | PASS |
| r00497 | admin | /customers | desktop | zh-TW | main | 開啟客戶: 王小明 (`customer-open-a42347e0-1689-41ab-8495-099655859fd0`) | click | navigation or in-page change | url /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/customers/a42347e0-1689-41ab-8495-099655859fd0?store=091b7f7a-dd25-41b5-bb29-e5cb59598 [1 of 3 alike] | PASS |
| r00498 | admin | /products/[product] | desktop | en | main | Compare-at price · Bulk fill (`bulk-compare`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00499 | admin | /inventory | mobile | zh-TW | main | 選取 sweep-wool-scarf-2 | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00500 | admin | /customers | desktop | zh-TW | main | 開啟客戶: 王小明 (`customer-open-50bfe0e9-cedd-47a6-8ca8-2744c6026a9d`) | click | navigation or in-page change | url /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/customers/50bfe0e9-cedd-47a6-8ca8-2744c6026a9d?store=091b7f7a-dd25-41b5-bb29-e5cb59598 [1 of 3 alike] | PASS |
| r00501 | admin | /inventory | mobile | zh-TW | main | Sweep Wool Scarf（复制） | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00502 | admin | /products/[product] | desktop | en | main | Quantity · Bulk fill (`bulk-quantity`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00503 | admin | /customers | desktop | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-16d4abf8-37d9-45bc-ab4d-50581594e25d`) | click | navigation or in-page change | url /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/customers/16d4abf8-37d9-45bc-ab4d-50581594e25d?store=091b7f7a-dd25-41b5-bb29-e5cb59598 [1 of 3 alike] | PASS |
| r00504 | admin | /inventory | mobile | zh-TW | main | 選取 SWP-MUG | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00505 | admin | /products/[product] | desktop | en | main | SKU code · Bulk fill (`bulk-code`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00506 | admin | /customers | desktop | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-895eb0de-9cf3-4fae-9374-bf91007ec7c3`) | click | navigation or in-page change | url /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/customers/895eb0de-9cf3-4fae-9374-bf91007ec7c3?store=091b7f7a-dd25-41b5-bb29-e5cb59598 [1 of 3 alike] | PASS |
| r00507 | admin | /inventory | mobile | zh-TW | main | Sweep Ceramic Mug | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00508 | admin | /products/[product] | desktop | en | main | Live keyword · Bulk fill (`bulk-keyword`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00509 | admin | /customers | desktop | zh-TW | main | 開啟客戶: 王小明 (`customer-open-a318dbd7-a935-4f38-a928-d8ebe96a0113`) | click | navigation or in-page change | url /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/customers/a318dbd7-a935-4f38-a928-d8ebe96a0113?store=091b7f7a-dd25-41b5-bb29-e5cb59598 [1 of 3 alike] | PASS |
| r00510 | admin | /inventory | mobile | zh-TW | main | 選取 SWP-SCF | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00511 | admin | /products/[product] | desktop | en | main | Do not track ∞ | click (toggle) | visible change | DOM changed (5 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00512 | admin | /customers | desktop | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-c5e99536-7599-44b0-8ba6-27e4680c97be`) | click | navigation or in-page change | url /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/customers/c5e99536-7599-44b0-8ba6-27e4680c97be?store=091b7f7a-dd25-41b5-bb29-e5cb59598 [1 of 3 alike] | PASS |
| r00513 | admin | /inventory | mobile | zh-TW | main | Sweep Wool Scarf | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00514 | admin | /products/[product] | desktop | en | main | Enabled 1 (`matrix-active-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00515 | admin | /customers | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00516 | admin | /inventory | mobile | zh-TW | main | 選取 T04-60e6d9a23cc9-0 | click | visible change | DOM changed (6 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00517 | admin | /customers | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00518 | admin | /products/[product] | desktop | en | main | Sweep home | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00519 | admin | /inventory | mobile | zh-TW | main | Sweep Cedar Candle | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00520 | admin | /customers | mobile | zh-TW | main | 搜尋 (`customers-search-submit`) | click | visible change | DOM changed (12 mutations) | PASS |
| r00521 | admin | /products/[product] | desktop | en | main | Sweep wear | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00522 | admin | /inventory | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00523 | admin | /customers | mobile | zh-TW | main | 重新整理 (`customers-refresh`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00524 | admin | /products/[product] | desktop | en | main | Manage collections | click | navigation or in-page change | scrolled | PASS |
| r00525 | admin | /inventory | desktop | en | main | Overview | click | navigation or in-page change | url /en/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00526 | admin | /customers | mobile | zh-TW | main | 開啟客戶: 王小明 (`customer-open-a42347e0-1689-41ab-8495-099655859fd0`) | click | navigation or in-page change | url /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/customers/a42347e0-1689-41ab-8495-099655859fd0?store=091b7f7a-dd25-41b5-bb29-e5cb59598 [1 of 3 alike] | PASS |
| r00527 | admin | /products/[product] | desktop | en | main | Shipping | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00528 | admin | /inventory | desktop | en | main | Add product | click | navigation or in-page change | url /en/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/products/new?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (19 mutations) | PASS |
| r00529 | admin | /customers | mobile | zh-TW | main | 開啟客戶: 王小明 (`customer-open-50bfe0e9-cedd-47a6-8ca8-2744c6026a9d`) | click | navigation or in-page change | url /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/customers/50bfe0e9-cedd-47a6-8ca8-2744c6026a9d?store=091b7f7a-dd25-41b5-bb29-e5cb59598 [1 of 3 alike] | PASS |
| r00530 | admin | /inventory | desktop | en | main | Search | click | visible change | DOM changed (2 mutations) | PASS |
| r00531 | admin | /products/[product] | desktop | en | layer of Shipping | Shipping | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00532 | admin | /customers | mobile | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-16d4abf8-37d9-45bc-ab4d-50581594e25d`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00533 | admin | /inventory | desktop | en | main | Warehouse | selectOption | visible change | select has a single option | SKIP |
| r00534 | admin | /products/[product] | desktop | en | main | Search engine listing | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00535 | admin | /inventory | desktop | en | main | Status | selectOption(active) | visible change | url /en/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/inventory?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&status=active; DOM changed (2 mutations | PASS |
| r00536 | admin | /customers | mobile | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-895eb0de-9cf3-4fae-9374-bf91007ec7c3`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00537 | admin | /products/[product] | desktop | en | layer of Search engine listing | Search engine listing | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00538 | admin | /customers | mobile | zh-TW | main | 開啟客戶: 王小明 (`customer-open-a318dbd7-a935-4f38-a928-d8ebe96a0113`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00539 | admin | /inventory | desktop | en | main | Reset | click | visible change | DOM changed (2 mutations) | PASS |
| r00540 | admin | /products/[product] | desktop | en | main | Save changes (`product-save`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00541 | admin | /customers | mobile | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-c5e99536-7599-44b0-8ba6-27e4680c97be`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00542 | admin | /inventory | desktop | en | main | Refresh | click | visible change | DOM changed (2 mutations) | PASS |
| r00543 | admin | /customers | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00544 | admin | /products/[product] | desktop | en | main | Unpublish to draft (`product-unpublish`) | click | confirmation layer opens; cancel; nothing changed | dialog: confirm: Unpublish this product? Shoppers will no longer be able to buy it. (confirmation shown, then cancelled) | PASS |
| r00545 | admin | /inventory | desktop | en | main | Select sweep-wool-scarf-2 | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00546 | admin | /customers | desktop | en | main | Overview | click | navigation or in-page change | url /en/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00547 | admin | /products/[product] | desktop | en | main | Publish (`product-publish`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (1 mutations) (inline change only; no request fired) | PASS |
| r00548 | admin | /inventory | desktop | en | main | Sweep Wool Scarf（复制） | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00549 | admin | /customers | desktop | en | main | Search (`customers-search-submit`) | click | visible change | DOM changed (12 mutations) | PASS |
| r00550 | admin | /customers/[customer] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00551 | admin | /inventory | desktop | en | main | Select | click | visible change | DOM changed (4 mutations); validation shown [1 of 4 alike] | PASS |
| r00552 | admin | /customers | desktop | en | main | Refresh (`customers-refresh`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00553 | admin | /customers/[customer] | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers/895eb0de-9cf3-4fae-9374-bf91007ec7c3?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM c | PASS |
| r00554 | admin | /inventory | desktop | en | main | Select SWP-MUG | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00555 | admin | /customers | desktop | en | main | Open customer: 王小明 (`customer-open-a42347e0-1689-41ab-8495-099655859fd0`) | click | navigation or in-page change | url /en/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/customers/a42347e0-1689-41ab-8495-099655859fd0?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; D [1 of 3 alike] | PASS |
| r00556 | admin | /customers/[customer] | desktop | zh-TW | main | 全部客戶 | click | navigation or in-page change | url /zh-TW/customers/895eb0de-9cf3-4fae-9374-bf91007ec7c3?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb59598 | PASS |
| r00557 | admin | /inventory | desktop | en | main | Sweep Ceramic Mug | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00558 | admin | /customers | desktop | en | main | Open customer: 王小明 (`customer-open-50bfe0e9-cedd-47a6-8ca8-2744c6026a9d`) | click | navigation or in-page change | url /en/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/customers/50bfe0e9-cedd-47a6-8ca8-2744c6026a9d?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; D [1 of 3 alike] | PASS |
| r00559 | admin | /customers/[customer] | desktop | zh-TW | main | 在訂單中開啟: 502291b1-eed5-4f65-8c7d-2ea28ba97972 | click | navigation or in-page change | url /zh-TW/customers/895eb0de-9cf3-4fae-9374-bf91007ec7c3?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b | PASS |
| r00560 | admin | /inventory | desktop | en | main | Select | click | visible change | DOM changed (6 mutations); validation shown [1 of 4 alike] | PASS |
| r00561 | admin | /customers | desktop | en | main | Open customer: Synthetic Buyer (`customer-open-16d4abf8-37d9-45bc-ab4d-50581594e25d`) | click | navigation or in-page change | url /en/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/customers/16d4abf8-37d9-45bc-ab4d-50581594e25d?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; D [1 of 3 alike] | PASS |
| r00562 | admin | /customers/[customer] | desktop | zh-TW | main | 下載資料 (`customer-download`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00563 | admin | /inventory | desktop | en | main | Select SWP-SCF | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00564 | admin | /customers | desktop | en | main | Open customer: Synthetic Buyer (`customer-open-895eb0de-9cf3-4fae-9374-bf91007ec7c3`) | click | navigation or in-page change | url /en/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/customers/895eb0de-9cf3-4fae-9374-bf91007ec7c3?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; D [1 of 3 alike] | PASS |
| r00565 | admin | /inventory | desktop | en | main | Sweep Wool Scarf | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00566 | admin | /customers/[customer] | desktop | zh-TW | main | 抹除客戶 (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00567 | admin | /customers | desktop | en | main | Open customer: 王小明 (`customer-open-a318dbd7-a935-4f38-a928-d8ebe96a0113`) | click | navigation or in-page change | url /en/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/customers/a318dbd7-a935-4f38-a928-d8ebe96a0113?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; D [1 of 3 alike] | PASS |
| r00568 | admin | /inventory | desktop | en | main | Select T04-60e6d9a23cc9-0 | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00569 | admin | /customers/[customer] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00570 | admin | /customers/[customer] | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers/895eb0de-9cf3-4fae-9374-bf91007ec7c3?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM c | PASS |
| r00571 | admin | /customers | desktop | en | main | Open customer: Synthetic Buyer (`customer-open-c5e99536-7599-44b0-8ba6-27e4680c97be`) | click | navigation or in-page change | url /en/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/customers/c5e99536-7599-44b0-8ba6-27e4680c97be?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; D [1 of 3 alike] | PASS |
| r00572 | admin | /inventory | desktop | en | main | Sweep Cedar Candle | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00573 | admin | /promotions | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00574 | admin | /customers/[customer] | mobile | zh-TW | main | 全部客戶 | click | navigation or in-page change | url /zh-TW/customers/895eb0de-9cf3-4fae-9374-bf91007ec7c3?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/customers?store=091b7f7a-dd25-41b5-bb29-e5cb59598 | PASS |
| r00575 | admin | /inventory | desktop | en | main | Select | click | visible change | DOM changed (4 mutations); validation shown [1 of 4 alike] | PASS |
| r00576 | admin | /promotions | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/promotions?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00577 | admin | /customers/[customer] | mobile | zh-TW | main | 在訂單中開啟: 502291b1-eed5-4f65-8c7d-2ea28ba97972 | click | navigation or in-page change | url /zh-TW/customers/895eb0de-9cf3-4fae-9374-bf91007ec7c3?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b | PASS |
| r00578 | admin | /ads | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00579 | admin | /promotions | desktop | zh-TW | main | 類型 (`promotion-kind`) | selectOption(fixed) | visible change | DOM changed (1 mutations); the control's own state changed (percent -> fixed) | PASS |
| r00580 | admin | /ads | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00581 | admin | /customers/[customer] | mobile | zh-TW | main | 下載資料 (`customer-download`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00582 | admin | /promotions | desktop | zh-TW | main | 建立優惠碼 (`promotion-submit`) | click | visible change | validation shown | PASS |
| r00583 | admin | /ads | desktop | zh-TW | main | 重新整理 (`ads-refresh`) | click | visible change | DOM changed (9 mutations) | PASS |
| r00584 | admin | /customers/[customer] | mobile | zh-TW | main | 抹除客戶 (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00585 | admin | /promotions | desktop | zh-TW | main | 暫停 (`promotion-toggle`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00586 | admin | /ads | desktop | zh-TW | main | 連結 Meta 廣告帳號 (`ads-connect`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00587 | admin | /customers/[customer] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00588 | admin | /promotions | desktop | zh-TW | main | 編輯 (`promotion-edit`) | click | visible change | DOM changed (6 mutations) | PASS |
| r00589 | admin | /customers/[customer] | desktop | en | main | Overview | click | navigation or in-page change | url /en/customers/895eb0de-9cf3-4fae-9374-bf91007ec7c3?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed | PASS |
| r00590 | admin | /ads | desktop | zh-TW | main | 新增草稿 (`ads-new-draft`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00591 | admin | /promotions | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00592 | admin | /customers/[customer] | desktop | en | main | All customers | click | navigation or in-page change | url /en/customers/895eb0de-9cf3-4fae-9374-bf91007ec7c3?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/customers?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; D | PASS |
| r00593 | admin | /ads | desktop | zh-TW | main | 顯示成效 (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00594 | admin | /promotions | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/promotions?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00595 | admin | /promotions | mobile | zh-TW | main | 類型 (`promotion-kind`) | selectOption(fixed) | visible change | DOM changed (1 mutations); the control's own state changed (percent -> fixed) | PASS |
| r00596 | admin | /customers/[customer] | desktop | en | main | Open in orders: 502291b1-eed5-4f65-8c7d-2ea28ba97972 | click | navigation or in-page change | url /en/customers/895eb0de-9cf3-4fae-9374-bf91007ec7c3?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/orders?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&order | PASS |
| r00597 | admin | /ads | desktop | zh-TW | main | 傳送購買事件 (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00598 | admin | /ads | desktop | zh-TW | main | 資料集請先連結廣告帳號與資料集才能開啟。 (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00599 | admin | /promotions | mobile | zh-TW | main | 建立優惠碼 (`promotion-submit`) | click | visible change | validation shown; scrolled | PASS |
| r00600 | admin | /customers/[customer] | desktop | en | main | Download data (`customer-download`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00601 | admin | /promotions | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00602 | admin | /ads | desktop | zh-TW | main | 儲存 (`ads-capi-save`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00603 | admin | /customers/[customer] | desktop | en | main | Erase customer (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00604 | admin | /promotions | desktop | en | main | Overview | click | navigation or in-page change | url /en/promotions?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00605 | admin | /ads | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00606 | admin | /design | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00607 | admin | /promotions | desktop | en | main | Type (`promotion-kind`) | selectOption(fixed) | visible change | DOM changed (1 mutations); the control's own state changed (percent -> fixed) | PASS |
| r00608 | admin | /ads | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00609 | admin | /design | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/design?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00610 | admin | /promotions | desktop | en | main | Create code (`promotion-submit`) | click | visible change | validation shown | PASS |
| r00611 | admin | /ads | mobile | zh-TW | main | 重新整理 (`ads-refresh`) | click | visible change | DOM changed (9 mutations) | PASS |
| r00612 | admin | /design | desktop | zh-TW | main | 預覽 (`design-preview`) | click | visible change | popup: about:blank; DOM changed (2 mutations) | PASS |
| r00613 | admin | /promotions | desktop | en | main | Pause (`promotion-toggle`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00614 | admin | /ads | mobile | zh-TW | main | 連結 Meta 廣告帳號 (`ads-connect`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00615 | admin | /promotions | desktop | en | main | Edit (`promotion-edit`) | click | visible change | DOM changed (6 mutations) | PASS |
| r00616 | admin | /ads | mobile | zh-TW | main | 新增草稿 (`ads-new-draft`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00617 | admin | /finance | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00618 | admin | /ads | mobile | zh-TW | main | 顯示成效 (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00619 | admin | /finance | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/finance?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00620 | admin | /design | desktop | zh-TW | main | 店鋪資料 (`design-tab-profile`) | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00621 | admin | /ads | mobile | zh-TW | main | 傳送購買事件 (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00622 | admin | /finance | desktop | zh-TW | main | 查看 (`finance-show`) | click | visible change | url /zh-TW/finance?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/finance?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&from=2026-09-04&to=2026-10-03; DOM ch | PASS |
| r00623 | admin | /design | desktop | zh-TW | main | 導覽 (`design-tab-nav`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00624 | admin | /ads | mobile | zh-TW | main | 資料集請先連結廣告帳號與資料集才能開啟。 (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00625 | admin | /finance | desktop | zh-TW | main | 下載 CSV (`finance-csv`) | click | navigation or in-page change | download: finance-2026-09-04-2026-10-03.csv | PASS |
| r00626 | admin | /design | desktop | zh-TW | main | 首頁 (`design-tab-home`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00627 | admin | /ads | mobile | zh-TW | main | 儲存 (`ads-capi-save`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00628 | admin | /finance | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00629 | admin | /design | desktop | zh-TW | main | 資訊頁 (`design-tab-pages`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00630 | admin | /ads | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00631 | admin | /finance | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/finance?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00632 | admin | /ads | desktop | en | main | Overview | click | navigation or in-page change | url /en/ads?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00633 | admin | /design | desktop | zh-TW | main | 版本 (`design-tab-versions`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00634 | admin | /finance | mobile | zh-TW | main | 查看 (`finance-show`) | click | visible change | url /zh-TW/finance?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW/finance?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&from=2026-09-04&to=2026-10-03; DOM ch | PASS |
| r00635 | admin | /ads | desktop | en | main | Refresh (`ads-refresh`) | click | visible change | DOM changed (9 mutations) | PASS |
| r00636 | admin | /design | desktop | zh-TW | main | 選擇圖片 (`design-logo-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00637 | admin | /finance | mobile | zh-TW | main | 下載 CSV (`finance-csv`) | click | navigation or in-page change | download: finance-2026-09-04-2026-10-03.csv | PASS |
| r00638 | admin | /ads | desktop | en | main | Connect a Meta ad account (`ads-connect`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00639 | admin | /design | desktop | zh-TW | main | 選擇圖片 (`design-favicon-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00640 | admin | /finance | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00641 | admin | /design | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00642 | admin | /ads | desktop | en | main | New draft (`ads-new-draft`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00643 | admin | /finance | desktop | en | main | Overview | click | navigation or in-page change | url /en/finance?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00644 | admin | /design | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/design?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00645 | admin | /ads | desktop | en | main | Show results (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00646 | admin | /finance | desktop | en | main | Show (`finance-show`) | click | visible change | url /en/finance?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en/finance?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b&from=2026-09-04&to=2026-10-03; DOM changed  | PASS |
| r00647 | admin | /design | mobile | zh-TW | main | 預覽 (`design-preview`) | click | visible change | popup: about:blank; DOM changed (2 mutations) | PASS |
| r00648 | admin | /ads | desktop | en | main | Send purchase events (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00649 | admin | /finance | desktop | en | main | Download CSV (`finance-csv`) | click | navigation or in-page change | download: finance-2026-09-04-2026-10-03.csv | PASS |
| r00650 | admin | /ads | desktop | en | main | DatasetConnect a dataset with an ad account to turn this on. (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00651 | admin | /settings | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00652 | admin | /settings | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/settings?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00653 | admin | /ads | desktop | en | main | Save (`ads-capi-save`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00654 | admin | /team | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00655 | admin | /design | mobile | zh-TW | main | 店鋪資料 (`design-tab-profile`) | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00656 | admin | /team | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/team?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00657 | admin | /design | mobile | zh-TW | main | 導覽 (`design-tab-nav`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00658 | admin | /team | desktop | zh-TW | main | 角色 (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00659 | admin | /design | mobile | zh-TW | main | 首頁 (`design-tab-home`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00660 | admin | /settings | desktop | zh-TW | main | 1 選擇平台 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00661 | admin | /team | desktop | zh-TW | main | 寄送邀請 (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00662 | admin | /design | mobile | zh-TW | main | 資訊頁 (`design-tab-pages`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00663 | admin | /team | desktop | zh-TW | main | 變更角色 (`member-role-bfde4180-903a-436e-a536-2617c77260e0`) | selectOption(admin) | visible change | DOM changed (3 mutations) | PASS |
| r00664 | admin | /design | mobile | zh-TW | main | 版本 (`design-tab-versions`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00665 | admin | /team | desktop | zh-TW | main | 移除 (`member-remove-bfde4180-903a-436e-a536-2617c77260e0`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (5 mutations) (inline change only; no request fired) | PASS |
| r00666 | admin | /design | mobile | zh-TW | main | 選擇圖片 (`design-logo-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00667 | admin | /team | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00668 | admin | /settings | desktop | zh-TW | main | PAYUNi 收款 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00669 | admin | /team | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/team?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00670 | admin | /design | mobile | zh-TW | main | 選擇圖片 (`design-favicon-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00671 | admin | /settings | desktop | zh-TW | main | 商家自行安排配送 | click | visible change | DOM changed (8 mutations); the control's own state changed (false -> true) | PASS |
| r00672 | admin | /team | mobile | zh-TW | main | 角色 (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00673 | admin | /design | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00674 | admin | /settings | desktop | zh-TW | main | 繼續 | click | visible change | DOM changed (12 mutations) | PASS |
| r00675 | admin | /team | mobile | zh-TW | main | 寄送邀請 (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00676 | admin | /design | desktop | en | main | Overview | click | navigation or in-page change | url /en/design?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00677 | admin | /team | mobile | zh-TW | main | 變更角色 (`member-role-bfde4180-903a-436e-a536-2617c77260e0`) | selectOption(admin) | visible change | DOM changed (3 mutations) | PASS |
| r00678 | admin | /settings | desktop | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00679 | admin | /design | desktop | en | main | Preview (`design-preview`) | click | visible change | popup: about:blank; DOM changed (2 mutations) | PASS |
| r00680 | admin | /team | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00681 | admin | /settings | desktop | zh-TW | main | 取消發佈 (`storefront-toggle`) | click | confirmation layer opens; cancel; nothing changed | layer opened (取消發佈); DOM changed (4 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00682 | admin | /team | desktop | en | main | Overview | click | navigation or in-page change | url /en/team?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00683 | admin | /team | desktop | en | main | Role (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00684 | admin | /settings | desktop | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00685 | admin | /team | desktop | en | main | Send invitation (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00686 | admin | /settings | desktop | zh-TW | main | 暫停 (`storefront-domain-suspend`) | click | confirmation layer opens; cancel; nothing changed | layer opened (暫停); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00687 | admin | /design | desktop | en | main | Store profile (`design-tab-profile`) | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00688 | admin | /team | desktop | en | main | Change role (`member-role-bfde4180-903a-436e-a536-2617c77260e0`) | selectOption(admin) | visible change | DOM changed (3 mutations) | PASS |
| r00689 | admin | /settings | desktop | zh-TW | main | 解除綁定 (`storefront-domain-detach`) | click | confirmation layer opens; cancel; nothing changed | layer opened (解除綁定); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00690 | admin | /design | desktop | en | main | Navigation (`design-tab-nav`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00691 | admin | /team | desktop | en | main | Remove (`member-remove-bfde4180-903a-436e-a536-2617c77260e0`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (5 mutations) (inline change only; no request fired) | PASS |
| r00692 | admin | /settings | desktop | zh-TW | main | 請求驗證 (`storefront-domain-submit`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00693 | admin | /design | desktop | en | main | Home page (`design-tab-home`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00694 | admin | /billing | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00695 | admin | /settings | desktop | zh-TW | main | 新增專頁 (`metaconnect-add`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00696 | admin | /design | desktop | en | main | Pages (`design-tab-pages`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00697 | admin | /billing | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/billing?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00698 | admin | /design | desktop | en | main | Versions (`design-tab-versions`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00699 | admin | /settings | desktop | zh-TW | main | 選擇專頁重新授權 (`metaconnect-reconnect`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00700 | admin | /billing | desktop | zh-TW | main | 訂閱 (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00701 | admin | /billing | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00702 | admin | /design | desktop | en | main | Choose image (`design-logo-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00703 | admin | /settings | desktop | zh-TW | main | 中斷連接 (`metaconnect-disconnect`) | click | confirmation layer opens; cancel; nothing changed | layer opened (中斷連接: Sweep Page); DOM changed (5 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00704 | admin | /billing | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/billing?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (11 mutations) | PASS |
| r00705 | admin | /settings | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00706 | admin | /design | desktop | en | main | Choose image (`design-favicon-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00707 | admin | /settings | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/settings?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /zh-TW?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00708 | admin | /billing | mobile | zh-TW | main | 管理付款與發票 (`billing-portal`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00709 | admin | /invite/[token] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00710 | admin | /invite/[token] | desktop | zh-TW | main | 登入 (`invite-signin`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (44 mu | PASS |
| r00711 | admin | /billing | mobile | zh-TW | main | 訂閱 (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00712 | admin | /invite/[token] | desktop | zh-TW | main | 註冊帳號 (`invite-signup`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW/signup#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed | PASS |
| r00713 | admin | /billing | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00714 | admin | /invite/[token] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00715 | admin | /billing | desktop | en | main | Overview | click | navigation or in-page change | url /en/billing?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00716 | admin | /invite/[token] | mobile | zh-TW | main | 登入 (`invite-signin`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (44 mu | PASS |
| r00717 | admin | /settings | mobile | zh-TW | main | 1 選擇平台 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00718 | admin | /billing | desktop | en | main | Manage payment & invoices (`billing-portal`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00719 | admin | /invite/[token] | mobile | zh-TW | main | 註冊帳號 (`invite-signup`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW/signup#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed | PASS |
| r00720 | admin | /billing | desktop | en | main | Subscribe (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00721 | admin | /invite/[token] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00722 | admin | /invite/[token] | desktop | en | main | Sign in (`invite-signin`) | click | navigation or in-page change | url /en/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /en#next=%2Fen%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (44 mutations); | PASS |
| r00723 | admin | /reset | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00724 | admin | /reset | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00725 | admin | /invite/[token] | desktop | en | main | Create an account (`invite-signup`) | click | navigation or in-page change | url /en/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /en/signup#next=%2Fen%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (33 muta | PASS |
| r00726 | admin | /settings | mobile | zh-TW | main | PAYUNi 收款 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00727 | admin | /reset | desktop | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00728 | admin | /signup | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00729 | admin | /signup | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00730 | admin | /settings | mobile | zh-TW | main | 商家自行安排配送 | click | visible change | DOM changed (8 mutations); the control's own state changed (false -> true) | PASS |
| r00731 | admin | /reset | desktop | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/reset -> /zh-TW; DOM changed (16 mutations); validation shown | PASS |
| r00732 | admin | /signup | desktop | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00733 | admin | /settings | mobile | zh-TW | main | 繼續 | click | visible change | DOM changed (12 mutations) | PASS |
| r00734 | admin | /reset | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00735 | admin | /reset | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00736 | admin | /signup | desktop | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/signup -> /zh-TW; DOM changed (9 mutations) | PASS |
| r00737 | admin | /settings | mobile | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00738 | admin | /reset | mobile | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00739 | admin | /signup | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00740 | admin | /settings | mobile | zh-TW | main | 取消發佈 (`storefront-toggle`) | click | confirmation layer opens; cancel; nothing changed | layer opened (取消發佈); DOM changed (4 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00741 | admin | /signup | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00742 | admin | /reset | mobile | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/reset -> /zh-TW; DOM changed (16 mutations); validation shown | PASS |
| r00743 | admin | /settings | mobile | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00744 | admin | /signup | mobile | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00745 | admin | /reset | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00746 | admin | /reset | desktop | en | main | Language | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00747 | admin | /settings | mobile | zh-TW | main | 暫停 (`storefront-domain-suspend`) | click | confirmation layer opens; cancel; nothing changed | layer opened (暫停); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00748 | admin | /signup | mobile | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/signup -> /zh-TW; DOM changed (9 mutations) | PASS |
| r00749 | admin | /reset | desktop | en | main | Send code | click | visible change | validation shown | PASS |
| r00750 | admin | /settings | mobile | zh-TW | main | 解除綁定 (`storefront-domain-detach`) | click | confirmation layer opens; cancel; nothing changed | layer opened (解除綁定); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00751 | admin | /signup | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00752 | admin | /signup | desktop | en | main | Language | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00753 | admin | /reset | desktop | en | main | Back to sign in | click | navigation or in-page change | url /en/reset -> /en; DOM changed (16 mutations); validation shown | PASS |
| r00754 | admin | /settings | mobile | zh-TW | main | 請求驗證 (`storefront-domain-submit`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00755 | admin | /signup | desktop | en | main | Send code | click | visible change | validation shown | PASS |
| r00756 | admin | /settings | mobile | zh-TW | main | 新增專頁 (`metaconnect-add`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00757 | admin | /signup | desktop | en | main | Back to sign in | click | navigation or in-page change | url /en/signup -> /en; DOM changed (9 mutations) | PASS |
| r00758 | admin | /settings | mobile | zh-TW | main | 選擇專頁重新授權 (`metaconnect-reconnect`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00759 | admin | /settings | mobile | zh-TW | main | 中斷連接 (`metaconnect-disconnect`) | click | confirmation layer opens; cancel; nothing changed | layer opened (中斷連接: Sweep Page); DOM changed (5 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00760 | admin | /settings | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00761 | admin | /settings | desktop | en | main | Overview | click | navigation or in-page change | url /en/settings?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b -> /en?store=091b7f7a-dd25-41b5-bb29-e5cb5959879b; DOM changed (15 mutations) | PASS |
| r00762 | admin | /settings | desktop | en | main | 1 Choose platform | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00763 | admin | /settings | desktop | en | main | PAYUNi payment | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00764 | admin | /settings | desktop | en | main | Merchant-arranged delivery | click | visible change | DOM changed (8 mutations); the control's own state changed (false -> true) | PASS |
| r00765 | admin | /settings | desktop | en | main | Continue | click | visible change | DOM changed (12 mutations) | PASS |
| r00766 | admin | /settings | desktop | en | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00767 | admin | /settings | desktop | en | main | Unpublish storefront (`storefront-toggle`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Unpublish storefront); DOM changed (4 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00768 | admin | /settings | desktop | en | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00769 | admin | /settings | desktop | en | main | Suspend (`storefront-domain-suspend`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Suspend); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00770 | admin | /settings | desktop | en | main | Detach (`storefront-domain-detach`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Detach); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00771 | admin | /settings | desktop | en | main | Request verification (`storefront-domain-submit`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00772 | admin | /settings | desktop | en | main | Add Page (`metaconnect-add`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00773 | admin | /settings | desktop | en | main | Choose a Page to reauthorize (`metaconnect-reconnect`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00774 | admin | /settings | desktop | en | main | Disconnect (`metaconnect-disconnect`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Disconnect: Sweep Page); DOM changed (5 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00775 | storefront | /cart | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00776 | storefront | /cart | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00777 | storefront | /cart | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00778 | storefront | /cart | desktop | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00779 | storefront | /cart | mobile | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00780 | storefront | /cart | desktop | en | main | Sweep Wool Scarf | click | navigation or in-page change | url /en/cart -> /en/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00781 | storefront | /cart | desktop | en | main | Increase quantity | click | visible change | DOM changed (3 mutations) | PASS |
| r00782 | storefront | /cart | desktop | zh-TW | main | 增加數量 | click | visible change | DOM changed (3 mutations) | PASS |
| r00783 | storefront | /cart | mobile | zh-TW | main | 增加數量 | click | visible change | DOM changed (3 mutations) | PASS |
| r00784 | storefront | /cart | desktop | en | main | Remove Sweep Wool Scarf from the cart | click | confirmation layer opens; cancel; nothing changed | DOM changed (3 mutations) (inline change only; no request fired) | PASS |
| r00785 | storefront | /cart | desktop | zh-TW | main | 將 Sweep Wool Scarf 移出購物車 | click | visible change | DOM changed (3 mutations) | PASS |
| r00786 | storefront | /cart | desktop | en | main | Checkout (`cart-checkout`) | click | navigation or in-page change | url /en/cart -> /en/checkout; DOM changed (11 mutations) | PASS |
| r00787 | storefront | /cart | desktop | en | main | Continue shopping | click | navigation or in-page change | url /en/cart -> /en/products; DOM changed (4 mutations) | PASS |
| r00788 | storefront | /checkout | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00789 | storefront | /checkout | desktop | en | main | Your orders (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00790 | storefront | /cart | mobile | zh-TW | main | 將 Sweep Wool Scarf 移出購物車 | click | visible change | DOM changed (3 mutations) | PASS |
| r00791 | storefront | /cart | desktop | zh-TW | main | 前往結帳 (`cart-checkout`) | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/checkout; DOM changed (11 mutations) | PASS |
| r00792 | storefront | /checkout | desktop | en | main | Sweep Wool Scarf | click | navigation or in-page change | url /en/checkout -> /en/products/sweep-wool-scarf; DOM changed (7 mutations) | PASS |
| r00793 | storefront | /cart | mobile | zh-TW | main | 前往結帳 (`cart-checkout`) | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/checkout; DOM changed (5 mutations) | PASS |
| r00794 | storefront | /checkout | desktop | en | main | Back to cart | click | navigation or in-page change | url /en/checkout -> /en/cart; DOM changed (9 mutations) | PASS |
| r00795 | storefront | /checkout | desktop | en | main | Choose delivery | click | visible change | DOM changed (3 mutations) | PASS |
| r00796 | storefront | /cart | desktop | zh-TW | main | 繼續選購 | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00797 | storefront | /checkout | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00798 | storefront | /checkout | desktop | zh-TW | main | 我的訂單 (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00799 | storefront | /orders/[orderID] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00800 | storefront | /orders/[orderID] | desktop | en | main | Refresh order (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone); scrolled | PASS |
| r00801 | storefront | /cart | mobile | zh-TW | main | 繼續選購 | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00802 | storefront | /checkout | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00803 | storefront | /checkout | mobile | zh-TW | main | 我的訂單 (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00804 | storefront | /checkout | mobile | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/products/sweep-wool-scarf; DOM changed (7 mutations) | PASS |
| r00805 | storefront | /checkout | desktop | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/products/sweep-wool-scarf; DOM changed (7 mutations) | PASS |
| r00806 | storefront | / | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00807 | storefront | /checkout | mobile | zh-TW | main | 返回購物車 | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/cart; DOM changed (9 mutations) | PASS |
| r00808 | storefront | /checkout | desktop | zh-TW | main | 返回購物車 | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/cart; DOM changed (9 mutations) | PASS |
| r00809 | storefront | /checkout | mobile | zh-TW | main | 選擇配送 | click | visible change | DOM changed (3 mutations) | PASS |
| r00810 | storefront | /checkout | desktop | zh-TW | main | 選擇配送 | click | visible change | DOM changed (3 mutations) | PASS |
| r00811 | storefront | / | desktop | en | chrome | Skip to content | keyboard: Tab to the link, Enter | navigation or in-page change | url /en -> /en#main; scrolled | PASS |
| r00812 | storefront | /orders/[orderID] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00813 | storefront | /orders/[orderID] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00814 | storefront | /orders/[orderID] | mobile | zh-TW | main | 重新整理訂單 (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone); scrolled | PASS |
| r00815 | storefront | / | desktop | en | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00816 | storefront | /orders/[orderID] | desktop | zh-TW | main | 重新整理訂單 (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone); scrolled | PASS |
| r00817 | storefront | / | desktop | en | chrome | All products | click | navigation or in-page change | url /en -> /en/products; DOM changed (4 mutations) | PASS |
| r00818 | storefront | / | desktop | en | chrome | Sweep home | click | navigation or in-page change | url /en -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00819 | storefront | / | desktop | en | chrome | About us | click | navigation or in-page change | url /en -> /en/pages/about; DOM changed (4 mutations) | PASS |
| r00820 | storefront | / | desktop | en | chrome | Search | click | visible change | url /en -> /en/search?q= | PASS |
| r00821 | storefront | / | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00822 | storefront | / | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00823 | storefront | / | desktop | en | chrome | Cart, 1 items (`header-cart`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00824 | storefront | / | mobile | zh-TW | chrome | 跳到主要內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW -> /zh-TW#main; scrolled | PASS |
| r00825 | storefront | / | desktop | zh-TW | chrome | 跳到主要內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW -> /zh-TW#main; scrolled | PASS |
| r00826 | storefront | / | mobile | zh-TW | chrome | 選單 (`menu-open`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00827 | storefront | / | desktop | zh-TW | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00828 | storefront | / | mobile | zh-TW | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00829 | storefront | / | desktop | zh-TW | chrome | All products | click | navigation or in-page change | url /zh-TW -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00830 | storefront | / | mobile | zh-TW | chrome | 搜尋 | click | navigation or in-page change | url /zh-TW -> /zh-TW/search; DOM changed (4 mutations) | PASS |
| r00831 | storefront | / | desktop | zh-TW | chrome | Sweep home | click | navigation or in-page change | url /zh-TW -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00832 | storefront | / | desktop | en | chrome | +886 2 2345 6789 | click | navigation or in-page change | scrolled | PASS |
| r00833 | storefront | / | mobile | zh-TW | chrome | 購物車，1 件商品 (`header-cart`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00834 | storefront | / | desktop | zh-TW | chrome | About us | click | navigation or in-page change | url /zh-TW -> /zh-TW/pages/about; DOM changed (4 mutations) | PASS |
| r00835 | storefront | / | desktop | en | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r00836 | storefront | / | desktop | zh-TW | chrome | 搜尋 | click | visible change | url /zh-TW -> /zh-TW/search?q= | PASS |
| r00837 | storefront | / | desktop | en | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r00838 | storefront | / | desktop | en | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r00839 | storefront | / | desktop | en | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r00840 | storefront | / | desktop | en | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00841 | storefront | / | desktop | en | chrome | Privacy Policy | click | navigation or in-page change | url /en -> /en/legal/privacy; DOM changed (20 mutations) | PASS |
| r00842 | storefront | / | mobile | zh-TW | chrome | +886 2 2345 6789 | click | navigation or in-page change | scrolled | PASS |
| r00843 | storefront | / | mobile | zh-TW | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r00844 | storefront | / | desktop | zh-TW | chrome | 購物車，1 件商品 (`header-cart`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00845 | storefront | / | mobile | zh-TW | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r00846 | storefront | / | mobile | zh-TW | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r00847 | storefront | / | mobile | zh-TW | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r00848 | storefront | / | mobile | zh-TW | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00849 | storefront | / | desktop | en | chrome | Terms of Service | click | navigation or in-page change | url /en -> /en/legal/terms | PASS |
| r00850 | storefront | / | mobile | zh-TW | chrome | 隱私權政策 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/privacy; DOM changed (19 mutations) | PASS |
| r00851 | storefront | / | desktop | en | chrome | Refunds, Returns and Cancellation | click | navigation or in-page change | url /en -> /en/legal/refunds | PASS |
| r00852 | storefront | / | mobile | zh-TW | chrome | 服務條款 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/terms | PASS |
| r00853 | storefront | / | desktop | zh-TW | chrome | +886 2 2345 6789 | click | navigation or in-page change | scrolled | PASS |
| r00854 | storefront | / | mobile | zh-TW | chrome | 退款、退貨與取消 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/legal/refunds (inline change only; no request fired) | PASS |
| r00855 | storefront | / | desktop | zh-TW | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r00856 | storefront | / | desktop | zh-TW | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r00857 | storefront | / | desktop | zh-TW | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r00858 | storefront | / | desktop | zh-TW | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r00859 | storefront | / | desktop | zh-TW | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00860 | storefront | / | desktop | zh-TW | chrome | 隱私權政策 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/privacy; DOM changed (20 mutations) | PASS |
| r00861 | storefront | / | desktop | en | chrome | Shipping | click | navigation or in-page change | url /en -> /en/legal/shipping | PASS |
| r00862 | storefront | / | mobile | zh-TW | chrome | 運送 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/shipping | PASS |
| r00863 | storefront | / | desktop | en | chrome | Contact | click | navigation or in-page change | url /en -> /en/legal/contact | PASS |
| r00864 | storefront | / | mobile | zh-TW | chrome | 聯絡我們 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/contact | PASS |
| r00865 | storefront | / | desktop | zh-TW | chrome | 服務條款 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/terms | PASS |
| r00866 | storefront | / | desktop | zh-TW | chrome | 退款、退貨與取消 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/legal/refunds (inline change only; no request fired) | PASS |
| r00867 | storefront | / | desktop | en | chrome | Shop safely | click | navigation or in-page change | url /en -> /en/legal/anti-fraud | PASS |
| r00868 | storefront | / | desktop | en | chrome | Data deletion | click | navigation or in-page change | url /en -> /en/data-deletion | PASS |
| r00869 | storefront | / | mobile | zh-TW | chrome | 防詐騙提醒 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/anti-fraud | PASS |
| r00870 | storefront | / | mobile | zh-TW | chrome | 資料刪除 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/data-deletion (inline change only; no request fired) | PASS |
| r00871 | storefront | / | desktop | zh-TW | chrome | 運送 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/shipping | PASS |
| r00872 | storefront | / | desktop | en | chrome | 简体中文 | click | navigation or in-page change | url /en -> /zh-CN | PASS |
| r00873 | storefront | / | mobile | zh-TW | chrome | 简体中文 | click | navigation or in-page change | url /zh-TW -> /zh-CN | PASS |
| r00874 | storefront | / | mobile | zh-TW | chrome | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r00875 | storefront | / | desktop | zh-TW | chrome | 聯絡我們 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/contact | PASS |
| r00876 | storefront | / | mobile | zh-TW | chrome | English | click | navigation or in-page change | url /zh-TW -> /en | PASS |
| r00877 | storefront | / | desktop | zh-TW | chrome | 防詐騙提醒 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/anti-fraud | PASS |
| r00878 | storefront | / | desktop | en | chrome | 繁體中文 | click | navigation or in-page change | url /en -> /zh-TW | PASS |
| r00879 | storefront | / | mobile | zh-TW | main | 查看全部 | click | navigation or in-page change | url /zh-TW -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00880 | storefront | / | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/t04-60e6d9a23cc9; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00881 | storefront | / | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00882 | storefront | / | desktop | en | chrome | English | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r00883 | storefront | / | mobile | zh-TW | main | 查看全部 | click | navigation or in-page change | scrolled | PASS |
| r00884 | storefront | / | mobile | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00885 | storefront | / | desktop | en | main | View all | click | navigation or in-page change | url /en -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00886 | storefront | / | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00887 | storefront | / | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en -> /en/products/t04-60e6d9a23cc9; DOM changed (4 mutations) [1 of 2 alike] | PASS |
| r00888 | storefront | / | desktop | zh-TW | chrome | 資料刪除 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/data-deletion (inline change only; no request fired) | PASS |
| r00889 | storefront | / | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00890 | storefront | / | desktop | zh-TW | chrome | 简体中文 | click | navigation or in-page change | url /zh-TW -> /zh-CN | PASS |
| r00891 | storefront | /products | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00892 | storefront | /products | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00893 | storefront | /products | mobile | zh-TW | main | 全部商品 | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00894 | storefront | / | desktop | zh-TW | chrome | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r00895 | storefront | /products | mobile | zh-TW | main | Sweep home | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00896 | storefront | / | desktop | zh-TW | chrome | English | click | navigation or in-page change | url /zh-TW -> /en | PASS |
| r00897 | storefront | /products | mobile | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00898 | storefront | / | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en -> /en/products/sweep-wool-scarf; DOM changed (4 mutations) [1 of 2 alike] | PASS |
| r00899 | storefront | /products | mobile | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00900 | storefront | / | desktop | en | main | View all | click | navigation or in-page change | scrolled | PASS |
| r00901 | storefront | / | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00902 | storefront | /products | mobile | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00903 | storefront | / | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00904 | storefront | /products | mobile | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/products -> /zh-TW/products?min=&max=&sort=newest | PASS |
| r00905 | storefront | / | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00906 | storefront | /products | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00907 | storefront | /products | desktop | en | main | Home | click | navigation or in-page change | url /en/products -> /en; DOM changed (4 mutations) | PASS |
| r00908 | storefront | / | desktop | zh-TW | main | 查看全部 | click | navigation or in-page change | url /zh-TW -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00909 | storefront | /products | desktop | en | main | All products | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00910 | storefront | / | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/t04-60e6d9a23cc9; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00911 | storefront | /products | desktop | en | main | Sweep home | click | navigation or in-page change | url /en/products -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00912 | storefront | / | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00913 | storefront | /products | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/products -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00914 | storefront | / | desktop | zh-TW | main | 查看全部 | click | navigation or in-page change | scrolled | PASS |
| r00915 | storefront | /products | mobile | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00916 | storefront | /products | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00917 | storefront | / | desktop | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00918 | storefront | /products | mobile | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00919 | storefront | / | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00920 | storefront | /products | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00921 | storefront | /products | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00922 | storefront | / | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00923 | storefront | /products | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/products -> /en/products?min=&max=&sort=newest | PASS |
| r00924 | storefront | /products | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r00925 | storefront | /products | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00926 | storefront | /products | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00927 | storefront | /collections | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00928 | storefront | /collections | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00929 | storefront | /products | desktop | zh-TW | main | 全部商品 | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00930 | storefront | /collections | mobile | zh-TW | main | S Sweep home 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00931 | storefront | /products | desktop | zh-TW | main | Sweep home | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00932 | storefront | /collections | mobile | zh-TW | main | S Sweep wear 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00933 | storefront | /products | desktop | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00934 | storefront | /collections/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00935 | storefront | /products | desktop | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00936 | storefront | /collections/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00937 | storefront | /products | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00938 | storefront | /products | desktop | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00939 | storefront | /collections/[slug] | mobile | zh-TW | main | 商品分類 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections; DOM changed (4 mutations) | PASS |
| r00940 | storefront | /products | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /en/products?min=&max=&sort=price_asc -> /en/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r00941 | storefront | /collections/[slug] | mobile | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00942 | storefront | /products | desktop | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/products -> /zh-TW/products?min=&max=&sort=newest | PASS |
| r00943 | storefront | /products | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/products -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00944 | storefront | /collections/[slug] | mobile | zh-TW | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00945 | storefront | /products | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/products -> /en/products/t04-60e6d9a23cc9; DOM changed (5 mutations) | PASS |
| r00946 | storefront | /collections/[slug] | mobile | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00947 | storefront | /collections | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00948 | storefront | /collections/[slug] | mobile | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00949 | storefront | /collections | desktop | en | main | Home | click | navigation or in-page change | url /en/collections -> /en; DOM changed (4 mutations) | PASS |
| r00950 | storefront | /collections/[slug] | mobile | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00951 | storefront | /collections | desktop | en | main | S Sweep home 2 products (`collection-tile`) | click | navigation or in-page change | url /en/collections -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00952 | storefront | /collections/[slug] | mobile | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-home?min=&max=&sort=newest | PASS |
| r00953 | storefront | /collections | desktop | en | main | S Sweep wear 2 products (`collection-tile`) | click | navigation or in-page change | url /en/collections -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00954 | storefront | /collections/[slug] | mobile | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00955 | storefront | /collections/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00956 | storefront | /products | desktop | zh-TW | main | 排序 | selectOption(price_asc) | visible change | url /zh-TW/products -> /zh-TW/products?min=&max=&sort=price_asc | PASS |
| r00957 | storefront | /collections/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/collections/sweep-home -> /en; DOM changed (4 mutations) | PASS |
| r00958 | storefront | /collections/[slug] | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/collections/sweep-home?min=&max=&sort=price_asc -> /zh-TW/products/t04-60e6d9a23cc9; DOM changed (5 mutations) | PASS |
| r00959 | storefront | /collections/[slug] | desktop | en | main | Collections | click | navigation or in-page change | url /en/collections/sweep-home -> /en/collections; DOM changed (6 mutations) | PASS |
| r00960 | storefront | /collections/[slug] | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00961 | storefront | /collections/[slug] | desktop | en | main | All products | click | navigation or in-page change | url /en/collections/sweep-home -> /en/products; DOM changed (4 mutations) | PASS |
| r00962 | storefront | /products/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00963 | storefront | /products/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00964 | storefront | /collections/[slug] | desktop | en | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00965 | storefront | /products/[slug] | mobile | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00966 | storefront | /collections/[slug] | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/collections/sweep-home -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00967 | storefront | /products/[slug] | mobile | zh-TW | main | 放大圖片 | click | visible change | layer opened (Sweep Wool Scarf — 放大圖片); DOM changed (2 mutations) | PASS |
| r00968 | storefront | /collections/[slug] | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00969 | storefront | /products/[slug] | mobile | zh-TW | main | 增加數量 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00970 | storefront | /products | desktop | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r00971 | storefront | /collections/[slug] | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00972 | storefront | /products/[slug] | mobile | zh-TW | main | 加入購物車 (`add-to-cart`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00973 | storefront | /products | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00974 | storefront | /collections/[slug] | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/collections/sweep-home -> /en/collections/sweep-home?min=&max=&sort=newest | PASS |
| r00975 | storefront | /products/[slug] | mobile | zh-TW | main | 立即購買 (`buy-now`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00976 | storefront | /products | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/t04-60e6d9a23cc9; DOM changed (5 mutations) | PASS |
| r00977 | storefront | /products/[slug] | mobile | zh-TW | main | Sweep home | click | navigation or in-page change | scrolled | PASS |
| r00978 | storefront | /collections | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00979 | storefront | /products/[slug] | mobile | zh-TW | main | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00980 | storefront | /collections | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00981 | storefront | /products/[slug] | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r00982 | storefront | /collections | desktop | zh-TW | main | S Sweep home 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00983 | storefront | /search | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00984 | storefront | /collections | desktop | zh-TW | main | S Sweep wear 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00985 | storefront | /search | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00986 | storefront | /collections/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00987 | storefront | /search | mobile | zh-TW | main | 搜尋 | click | visible change | the control's own state changed ( -> gone) | PASS |
| r00988 | storefront | /collections/[slug] | desktop | en | main | Sort by | selectOption(price_asc) | visible change | page reloaded | PASS |
| r00989 | storefront | /collections/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00990 | storefront | /search | mobile | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00991 | storefront | /collections/[slug] | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/collections/sweep-home -> /en/products/t04-60e6d9a23cc9; DOM changed (4 mutations) | PASS |
| r00992 | storefront | /collections/[slug] | desktop | zh-TW | main | 商品分類 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections; DOM changed (4 mutations) | PASS |
| r00993 | storefront | /search | mobile | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00994 | storefront | /collections/[slug] | desktop | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00995 | storefront | /collections/[slug] | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/collections/sweep-home -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00996 | storefront | /products/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00997 | storefront | /search | mobile | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/search?q=Sweep -> /zh-TW/search?q=Sweep&min=&max=&sort=newest | PASS |
| r00998 | storefront | /collections/[slug] | desktop | zh-TW | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00999 | storefront | /products/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en; DOM changed (4 mutations) | PASS |
| r01000 | storefront | /search | mobile | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01001 | storefront | /collections/[slug] | desktop | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r01002 | storefront | /products/[slug] | desktop | en | main | All products | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/products; DOM changed (4 mutations) | PASS |
| r01003 | storefront | /search | mobile | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r01004 | storefront | /collections/[slug] | desktop | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r01005 | storefront | /products/[slug] | desktop | en | main | Enlarge photo | click | visible change | layer opened (Sweep Wool Scarf — Enlarge photo); DOM changed (2 mutations) | PASS |
| r01006 | storefront | /search | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01007 | storefront | /products/[slug] | desktop | en | main | Increase quantity | click | visible change | DOM changed (2 mutations) | PASS |
| r01008 | storefront | /collections/[slug] | desktop | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r01009 | storefront | /products/[slug] | desktop | en | main | Add to cart (`add-to-cart`) | click | visible change | DOM changed (3 mutations) | PASS |
| r01010 | storefront | /search | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r01011 | storefront | /collections/[slug] | desktop | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-home?min=&max=&sort=newest | PASS |
| r01012 | storefront | /orders/lookup | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01013 | storefront | /products/[slug] | desktop | en | main | Buy now (`buy-now`) | click | visible change | DOM changed (3 mutations) | PASS |
| r01014 | storefront | /orders/lookup | mobile | zh-TW | main | 查詢 (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r01015 | storefront | /products/[slug] | desktop | en | main | Sweep home | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r01016 | storefront | /order-link | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01017 | storefront | /order-link | mobile | zh-TW | main | 查詢訂單 | click | navigation or in-page change | url /zh-TW/order-link -> /zh-TW/orders/lookup; DOM changed (12 mutations) | PASS |
| r01018 | storefront | /products/[slug] | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r01019 | storefront | /claim | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01020 | storefront | /claim | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r01021 | storefront | /products/[slug] | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r01022 | storefront | /legal/anti-fraud | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01023 | storefront | /search | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01024 | storefront | /legal/anti-fraud | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /zh-TW; DOM changed (5 mutations) | PASS |
| r01025 | storefront | /search | desktop | en | main | Home | click | navigation or in-page change | url /en/search?q=Sweep -> /en; DOM changed (4 mutations) | PASS |
| r01026 | storefront | /collections/[slug] | desktop | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01027 | storefront | /legal/anti-fraud | mobile | zh-TW | main | 內政部警政署 — 165 全民防騙網 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r01028 | storefront | /search | desktop | en | main | Search | click | visible change | the control's own state changed ( -> gone) | PASS |
| r01029 | storefront | /collections/[slug] | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/collections/sweep-home?min=&max=&sort=price_asc -> /zh-TW/products/t04-60e6d9a23cc9; DOM changed (5 mutations) | PASS |
| r01030 | storefront | /legal/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01031 | storefront | /search | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r01032 | storefront | /privacy | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01033 | storefront | /privacy | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/privacy -> /zh-CN/privacy; DOM changed (28 mutations) | PASS |
| r01034 | storefront | /collections/[slug] | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01035 | storefront | /search | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r01036 | storefront | /products/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01037 | storefront | /privacy | mobile | zh-TW | main | 開啟: 允許店鋪透過 Messenger 或 Instagram 私訊向我發送行銷訊息。 (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01038 | storefront | /products/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW; DOM changed (4 mutations) | PASS |
| r01039 | storefront | /privacy | mobile | zh-TW | main | 開啟: 使用我的購買紀錄為我個人化 Meta 廣告。 (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01040 | storefront | /search | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/search?q=Sweep -> /en/search?q=Sweep&min=&max=&sort=newest | PASS |
| r01041 | storefront | /products/[slug] | desktop | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r01042 | storefront | /products/[slug] | desktop | zh-TW | main | 放大圖片 | click | visible change | layer opened (Sweep Wool Scarf — 放大圖片); DOM changed (2 mutations) | PASS |
| r01043 | storefront | /products/[slug] | desktop | zh-TW | main | 增加數量 | click | visible change | DOM changed (2 mutations) | PASS |
| r01044 | storefront | /products/[slug] | desktop | zh-TW | main | 加入購物車 (`add-to-cart`) | click | visible change | DOM changed (3 mutations) | PASS |
| r01045 | storefront | /products/[slug] | desktop | zh-TW | main | 立即購買 (`buy-now`) | click | visible change | DOM changed (3 mutations) | PASS |
| r01046 | storefront | /products/[slug] | desktop | zh-TW | main | Sweep home | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r01047 | storefront | /privacy | mobile | zh-TW | main | 下載我的資料 (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r01048 | storefront | /search | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01049 | storefront | /products/[slug] | desktop | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r01050 | storefront | /products/[slug] | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r01051 | storefront | /search | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /en/search?q=Sweep&min=&max=&sort=price_asc -> /en/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r01052 | storefront | /search | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/search?q=Sweep -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01053 | storefront | /search | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/search?q=Sweep -> /en/products/t04-60e6d9a23cc9; DOM changed (5 mutations) | PASS |
| r01054 | storefront | /orders/lookup | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01055 | storefront | /orders/lookup | desktop | en | main | Find order (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r01056 | storefront | /order-link | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01057 | storefront | /privacy | mobile | zh-TW | main | 抹除我的資料… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r01058 | storefront | /order-link | desktop | en | main | Look up an order | click | navigation or in-page change | url /en/order-link -> /en/orders/lookup; DOM changed (12 mutations) | PASS |
| r01059 | storefront | /data-deletion | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01060 | storefront | /search | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01061 | storefront | /claim | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01062 | storefront | /data-deletion | mobile | zh-TW | main | 简体中文 | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-CN/data-deletion | PASS |
| r01063 | storefront | /claim | desktop | en | main | Language | selectOption(zh-CN) | visible change | url /en/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r01064 | storefront | /search | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW; DOM changed (4 mutations) | PASS |
| r01065 | storefront | /legal/anti-fraud | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01066 | storefront | /search | desktop | zh-TW | main | 搜尋 | click | visible change | the control's own state changed ( -> gone) | PASS |
| r01067 | storefront | /legal/anti-fraud | desktop | en | main | Home | click | navigation or in-page change | url /en/legal/anti-fraud -> /en; DOM changed (4 mutations) | PASS |
| r01068 | storefront | /search | desktop | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r01069 | storefront | /legal/anti-fraud | desktop | en | main | Taiwan National Police Agency — 165 anti-fraud service | click | navigation or in-page change | url /en/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r01070 | storefront | /data-deletion | mobile | zh-TW | main | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r01071 | storefront | /legal/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01072 | storefront | /search | desktop | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r01073 | storefront | /privacy | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01074 | storefront | /data-deletion | mobile | zh-TW | main | English | click | navigation or in-page change | url /zh-TW/data-deletion -> /en/data-deletion | PASS |
| r01075 | storefront | /privacy | desktop | en | main | Language | selectOption(zh-CN) | visible change | url /en/privacy -> /zh-CN/privacy; DOM changed (29 mutations) | PASS |
| r01076 | storefront | /data-deletion | mobile | zh-TW | main | 開啟隱私頁面 (`data-deletion-privacy-link`) | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-TW/privacy | PASS |
| r01077 | storefront | /search | desktop | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/search?q=Sweep -> /zh-TW/search?q=Sweep&min=&max=&sort=newest | PASS |
| r01078 | storefront | /privacy | desktop | en | main | Turn on: The store may send me marketing messages on Messenger or Instagram DM. (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01079 | storefront | /pages/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01080 | storefront | /search | desktop | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01081 | storefront | /pages/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/pages/about -> /zh-TW; DOM changed (5 mutations) | PASS |
| r01082 | storefront | /search | desktop | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /zh-TW/search?q=Sweep&min=&max=&sort=price_asc -> /zh-TW/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r01083 | storefront | /search | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01084 | storefront | /search | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW/products/t04-60e6d9a23cc9; DOM changed (5 mutations) | PASS |
| r01085 | storefront | /orders/lookup | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01086 | storefront | /orders/lookup | desktop | zh-TW | main | 查詢 (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r01087 | storefront | /privacy | desktop | en | main | Turn on: Use my purchase to personalize Meta ads for me. (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01088 | storefront | /order-link | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01089 | storefront | /order-link | desktop | zh-TW | main | 查詢訂單 | click | navigation or in-page change | url /zh-TW/order-link -> /zh-TW/orders/lookup; DOM changed (12 mutations) | PASS |
| r01090 | storefront | /claim | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01091 | storefront | /claim | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r01092 | storefront | /legal/anti-fraud | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01093 | storefront | /legal/anti-fraud | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /zh-TW; DOM changed (4 mutations) | PASS |
| r01094 | storefront | /legal/anti-fraud | desktop | zh-TW | main | 內政部警政署 — 165 全民防騙網 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r01095 | storefront | /legal/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01096 | storefront | /privacy | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01097 | storefront | /privacy | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/privacy -> /zh-CN/privacy; DOM changed (28 mutations) | PASS |
| r01098 | storefront | /privacy | desktop | en | main | Download my data (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r01099 | storefront | /privacy | desktop | zh-TW | main | 開啟: 允許店鋪透過 Messenger 或 Instagram 私訊向我發送行銷訊息。 (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01100 | storefront | /privacy | desktop | en | main | Erase my data… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations) (confirmation shown, then cancelled) | PASS |
| r01101 | storefront | /data-deletion | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01102 | storefront | /privacy | desktop | zh-TW | main | 開啟: 使用我的購買紀錄為我個人化 Meta 廣告。 (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01103 | storefront | /data-deletion | desktop | en | main | 简体中文 | click | navigation or in-page change | url /en/data-deletion -> /zh-CN/data-deletion | PASS |
| r01104 | storefront | /privacy | desktop | zh-TW | main | 下載我的資料 (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r01105 | storefront | /data-deletion | desktop | en | main | 繁體中文 | click | navigation or in-page change | url /en/data-deletion -> /zh-TW/data-deletion | PASS |
| r01106 | storefront | /data-deletion | desktop | en | main | English | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r01107 | storefront | /data-deletion | desktop | en | main | Open the privacy page (`data-deletion-privacy-link`) | click | navigation or in-page change | url /en/data-deletion -> /en/privacy | PASS |
| r01108 | storefront | /pages/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01109 | storefront | /privacy | desktop | zh-TW | main | 抹除我的資料… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations) (confirmation shown, then cancelled) | PASS |
| r01110 | storefront | /pages/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/pages/about -> /en; DOM changed (4 mutations) | PASS |
| r01111 | storefront | /data-deletion | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01112 | storefront | /data-deletion | desktop | zh-TW | main | 简体中文 | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-CN/data-deletion | PASS |
| r01113 | storefront | /data-deletion | desktop | zh-TW | main | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r01114 | storefront | /data-deletion | desktop | zh-TW | main | English | click | navigation or in-page change | url /zh-TW/data-deletion -> /en/data-deletion | PASS |
| r01115 | storefront | /data-deletion | desktop | zh-TW | main | 開啟隱私頁面 (`data-deletion-privacy-link`) | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-TW/privacy | PASS |
| r01116 | storefront | /pages/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01117 | storefront | /pages/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/pages/about -> /zh-TW; DOM changed (4 mutations) | PASS |
| r01118 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | open the product list and click New product | real clicks | the create form opens | as expected | PASS |
| r01119 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | fill the name and description | real clicks | the draft fields retain the entered values | as expected | PASS |
| r01120 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | add Size = S, M | real clicks | the editor proposes two SKU rows | as expected | PASS |
| r01121 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | type both SKU prices | real clicks | the matrix retains both prices | as expected | PASS |
| r01122 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | set opening quantities 9 and 7, then save the document | real clicks | both SKU quantities persist | as expected | PASS |
| r01123 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | upload a cover, set Active and save | real clicks | the product is active and persists after a reload | as expected | PASS |
| r01124 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | the merchant list shows the product (search + click) | real clicks | the row is listed with status active | as expected | PASS |
| r01125 | journey | J1 storefront | desktop | zh-TW | journey | the buyer opens All products and clicks the new product | real clicks | its page opens with the title and an enabled Add to cart | as expected | PASS |
| r01126 | journey | J2/J3 admin orders | desktop | zh-TW | journey | admin Orders: the storefront COD order is listed (click through pages) | real clicks | the row of the order id can be expanded | as expected | PASS |
| r01127 | journey | J2/J3 admin orders | desktop | zh-TW | journey | record the manual shipment (carrier Black Cat + tracking) and click Record | real clicks | the shipment record is shown | as expected | PASS |
| r01128 | journey | J2/J3 admin orders | desktop | zh-TW | journey | click Collected, confirm in the dialog | real clicks | the order shows COLLECTED and the button is gone | as expected | PASS |
| r01129 | journey | J2/J3 admin orders | desktop | zh-TW | journey | reload the order list: collected persists | real clicks | COLLECTED is still shown after a reload | as expected | PASS |
| r01130 | journey | J2 storefront order lookup | desktop | zh-TW | journey | a fresh buyer browser looks the order up (order id + phone) and sees it collected | real clicks | the lookup shows the order with the collected amount | as expected | PASS |
| r01131 | journey | J4 custom domain | desktop | zh-TW | journey | open Settings and find the custom domain card | real clicks | the storefront domains card is shown | as expected | PASS |
| r01132 | journey | J4 custom domain | desktop | zh-TW | journey | type a custom hostname and click Request | real clicks | DNS instructions appear: a TXT name, a TXT value and the CNAME target | as expected | PASS |
| r01133 | journey | J4 custom domain | desktop | zh-TW | journey | reload Settings: the requested domain is listed | real clicks | the domain row persists in state REQUESTED | as expected | PASS |
| r01134 | journey | J5 sign out | desktop | zh-TW | journey | open the dashboard, click Sign out | real clicks | the session ends: the page asks to sign in again | as expected | PASS |
| r01135 | journey | J5 sign out | desktop | zh-TW | journey | reload after sign out | real clicks | the dashboard is not shown without a session | as expected | PASS |
| r01136 | storefront | /live | - | - | page | (not implemented) | - | ui-architecture section 4 lists /live (S6) | apps/storefront/app/[locale] has no live route in this base; nothing to click | SKIP |
