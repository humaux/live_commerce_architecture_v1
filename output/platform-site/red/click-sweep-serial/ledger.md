# G-UI8 click ledger

Generated 2026-10-03T19:28:22.689Z. 120 page/viewport/locale units opened (32 with a page-load failure), 983 control clicks (960 pass, 3 fail, 20 skip), 18 journey steps (18 pass, 0 fail); failures: known 0, new 35; stale known-defect entries 0.

Every row is one real Playwright interaction (click / selectOption). Destructive and irreversible controls stop at their confirmation and are cancelled; `skip` rows name why.

| # | App | Page | Viewport | Locale | Scope | Control | Action | Expected | Actual | Result |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| r00001 | admin | /studio | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00002 | admin | /studio/claims | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00003 | admin | / | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00004 | admin | /studio | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00005 | admin | /studio/claims | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio/claims?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&scene=343c278a-3f12-4e8c-b5ca-561c95af9941 -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0 | PASS |
| r00006 | admin | /studio/claims | desktop | zh-TW | main | 直播工作室 | click | visible change | url /zh-TW/studio/claims?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&scene=343c278a-3f12-4e8c-b5ca-561c95af9941 -> /zh-TW/studio?store=26dc24c3-776c-4adf-943d-12 | PASS |
| r00007 | admin | /studio | desktop | zh-TW | main | 留言關鍵字登記 (`studio-open-claims`) | click | visible change | url /zh-TW/studio?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/studio/claims?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&scene=343c278a-3f12-4e8c-b5ca-56 | PASS |
| r00008 | admin | /studio/claims | desktop | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (2 mutations) | PASS |
| r00009 | admin | /studio | desktop | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (18 mutations) | PASS |
| r00010 | admin | /studio/claims | desktop | zh-TW | main | 數量規則 | selectOption(KEYWORD_QTY_ONLY) | visible change | the control's own state changed (EXACT -> KEYWORD_QTY_ONLY) | PASS |
| r00011 | admin | /studio | desktop | zh-TW | main | ＋ 新增場次 | click | visible change | DOM changed (8 mutations) | PASS |
| r00012 | admin | /studio/claims | desktop | zh-TW | main | 開放登記窗口 | click | visible change | DOM changed (21 mutations) | PASS |
| r00013 | admin | /studio | desktop | zh-TW | main | Click sweep scene 744adc73cc69 草稿 預定時間（台北時間，選填） | click | visible change | DOM changed (5 mutations) | PASS |
| r00014 | admin | /studio/claims | desktop | zh-TW | main | 這則貼文或直播所屬的專頁 | selectOption(10505210174913264572) | visible change | the control's own state changed ( -> 10505210174913264572) | PASS |
| r00015 | admin | /studio | desktop | zh-TW | main | 畫面比例 | selectOption(16:9) | visible change | DOM changed (2 mutations); the control's own state changed (9:16 -> 16:9) | PASS |
| r00016 | admin | /studio/claims | desktop | zh-TW | main | 平台 | selectOption(facebook) | visible change | the control's own state changed ( -> facebook) | PASS |
| r00017 | admin | /studio | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00018 | admin | /studio/claims | desktop | zh-TW | main | 回覆語言 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN) | PASS |
| r00019 | admin | / | desktop | zh-TW | skip | 跳至內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec#main; scrolled | PASS |
| r00020 | admin | /studio | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00021 | admin | /studio/claims | desktop | zh-TW | main | 傳送附帶購物車連結的私訊回覆 | click (toggle) | visible change | the control's own state changed (false -> true) | PASS |
| r00022 | admin | /studio | mobile | zh-TW | main | 留言關鍵字登記 (`studio-open-claims`) | click | visible change | url /zh-TW/studio?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/studio/claims?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&scene=343c278a-3f12-4e8c-b5ca-56 | PASS |
| r00023 | admin | / | desktop | zh-TW | topbar | 切換商店 (`shell-store-selector`) | selectOption | visible change | select has a single option | SKIP |
| r00024 | admin | /studio/claims | desktop | zh-TW | main | 從此來源收集留言 | click (toggle) | visible change | the control's own state changed (true -> false) | PASS |
| r00025 | admin | / | desktop | zh-TW | topbar | 語言 (`locale-switch`) | selectOption(zh-CN) | visible change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-CN?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (17 mutations) | PASS |
| r00026 | admin | /studio | mobile | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (18 mutations) | PASS |
| r00027 | admin | /studio/claims | desktop | zh-TW | main | 儲存留言來源 | click | visible change | DOM changed (2 mutations); validation shown | PASS |
| r00028 | admin | / | desktop | zh-TW | topbar | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00029 | admin | /studio | mobile | zh-TW | main | ＋ 新增場次 | click | visible change | DOM changed (8 mutations) | PASS |
| r00030 | admin | /studio/claims | desktop | zh-TW | main | 商品 | selectOption(1f957810-4949-430d-9b43-1e1c1b0f9a46) | visible change | the control's own state changed ( -> 1f957810-4949-430d-9b43-1e1c1b0f9a46); scrolled | PASS |
| r00031 | admin | /studio | mobile | zh-TW | main | Click sweep scene 744adc73cc69 草稿 預定時間（台北時間，選填） | click | visible change | DOM changed (5 mutations) | PASS |
| r00032 | admin | / | desktop | zh-TW | layer of 說明 | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00033 | admin | /studio/claims | desktop | zh-TW | main | 新增關鍵字 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00034 | admin | /studio | mobile | zh-TW | main | 畫面比例 | selectOption(16:9) | visible change | DOM changed (2 mutations); the control's own state changed (9:16 -> 16:9); scrolled | PASS |
| r00035 | admin | / | desktop | zh-TW | topbar | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00036 | admin | /studio/claims | desktop | zh-TW | main | 加入關鍵字庫 (`claims-add-library`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00037 | admin | /studio | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00038 | admin | /studio/claims | desktop | zh-TW | main | 買家 | selectOption | visible change | select has a single option | SKIP |
| r00039 | admin | / | desktop | zh-TW | layer of 帳號 | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00040 | admin | /studio | desktop | en | main | Overview | click | navigation or in-page change | url /en/studio?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00041 | admin | / | desktop | zh-TW | layer of 帳號 | 登出 (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00042 | admin | /studio/claims | desktop | zh-TW | main | 記錄留言 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00043 | admin | /studio | desktop | en | main | Keyword claims (`studio-open-claims`) | click | visible change | url /en/studio?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/studio/claims?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&scene=343c278a-3f12-4e8c-b5ca-561c95af | PASS |
| r00044 | admin | /studio/claims | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00045 | admin | /studio | desktop | en | main | Refresh facts | click | visible change | DOM changed (18 mutations) | PASS |
| r00046 | admin | / | desktop | zh-TW | rail | 總覽 (`nav-group-overview`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00047 | admin | /studio/claims | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio/claims?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&scene=343c278a-3f12-4e8c-b5ca-561c95af9941 -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0 | PASS |
| r00048 | admin | /studio | desktop | en | main | ＋ New scene | click | visible change | DOM changed (8 mutations) | PASS |
| r00049 | admin | / | desktop | zh-TW | rail | 直播與貼文 (`nav-group-live`) | click | visible change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/studio?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (21 mutations) | PASS |
| r00050 | admin | /studio/claims | mobile | zh-TW | main | 直播工作室 | click | visible change | url /zh-TW/studio/claims?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&scene=343c278a-3f12-4e8c-b5ca-561c95af9941 -> /zh-TW/studio?store=26dc24c3-776c-4adf-943d-12 | PASS |
| r00051 | admin | /studio | desktop | en | main | Click sweep scene 744adc73cc69 Draft Scheduled time (Taipei time, optional) | click | visible change | DOM changed (5 mutations) | PASS |
| r00052 | admin | / | desktop | zh-TW | rail | 訂單與出貨 (`nav-orders`) | click | visible change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (19 mutations) | PASS |
| r00053 | admin | /studio/claims | mobile | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (2 mutations) | PASS |
| r00054 | admin | /studio | desktop | en | main | Canvas ratio | selectOption(16:9) | visible change | DOM changed (2 mutations); the control's own state changed (9:16 -> 16:9) | PASS |
| r00055 | admin | /studio/claims | mobile | zh-TW | main | 數量規則 | selectOption(KEYWORD_QTY_ONLY) | visible change | the control's own state changed (EXACT -> KEYWORD_QTY_ONLY) | PASS |
| r00056 | admin | / | desktop | zh-TW | rail | 商品與庫存 + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00057 | admin | /orders | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00058 | admin | /studio/claims | mobile | zh-TW | main | 開放登記窗口 | click | visible change | DOM changed (21 mutations) | PASS |
| r00059 | admin | /orders | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00060 | admin | / | desktop | zh-TW | rail | 顧客 (`nav-group-customers`) | click | visible change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (16 mutations) | PASS |
| r00061 | admin | /studio/claims | mobile | zh-TW | main | 這則貼文或直播所屬的專頁 | selectOption(10505210174913264572) | visible change | the control's own state changed ( -> 10505210174913264572); scrolled | PASS |
| r00062 | admin | /orders | desktop | zh-TW | main | 訂單狀態 (`state-filter`) | selectOption(all) | visible change | DOM changed (8 mutations) | PASS |
| r00063 | admin | / | desktop | zh-TW | rail | 行銷優惠 + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00064 | admin | /studio/claims | mobile | zh-TW | main | 平台 | selectOption(facebook) | visible change | the control's own state changed ( -> facebook); scrolled | PASS |
| r00065 | admin | /orders | desktop | zh-TW | main | 重新整理 (`orders-refresh`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00066 | admin | / | desktop | zh-TW | rail | 網路商店 (`nav-group-storefront`) | click | visible change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/design?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (13 mutations) | PASS |
| r00067 | admin | /studio/claims | mobile | zh-TW | main | 回覆語言 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN); scrolled | PASS |
| r00068 | admin | /orders | desktop | zh-TW | main | 匯出未出貨訂單（CSV） (`orders-export`) | click | navigation or in-page change | download: unshipped-26dc24c3-202610031917.csv | PASS |
| r00069 | admin | / | desktop | zh-TW | rail | 收款與報表 (`nav-group-finance`) | click | visible change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/finance?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (16 mutations) | PASS |
| r00070 | admin | /studio/claims | mobile | zh-TW | main | 傳送附帶購物車連結的私訊回覆 | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00071 | admin | /orders | desktop | zh-TW | main | 付款方式 (`orders-payment-filter`) | selectOption(card) | visible change | the control's own state changed ( -> card) | PASS |
| r00072 | admin | / | desktop | zh-TW | rail | 設定 + (`nav-group-settings`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00073 | admin | /orders | desktop | zh-TW | main | 配送方式 (`orders-delivery-filter`) | selectOption(home) | visible change | the control's own state changed ( -> home) | PASS |
| r00074 | admin | /studio/claims | mobile | zh-TW | main | 從此來源收集留言 | click (toggle) | visible change | the control's own state changed (true -> false); scrolled | PASS |
| r00075 | admin | /orders | desktop | zh-TW | main | 直播場次 (`orders-session-filter`) | selectOption | visible change | select has a single option | SKIP |
| r00076 | admin | /studio/claims | mobile | zh-TW | main | 儲存留言來源 | click | visible change | DOM changed (2 mutations); validation shown; scrolled | PASS |
| r00077 | admin | / | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | DOM changed (10 mutations) | PASS |
| r00078 | admin | /orders | desktop | zh-TW | main | 套用篩選 (`orders-apply`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00079 | admin | /studio/claims | mobile | zh-TW | main | 商品 | selectOption(1f957810-4949-430d-9b43-1e1c1b0f9a46) | visible change | the control's own state changed ( -> 1f957810-4949-430d-9b43-1e1c1b0f9a46); scrolled | PASS |
| r00080 | admin | / | desktop | zh-TW | main | 待確認的轉帳 1 (`todo-transfer`) | click | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?state=AWAITING_TRANSFER&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (19 mutat | PASS |
| r00081 | admin | /orders | desktop | zh-TW | main | 清除篩選 (`orders-reset`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (11 mutations); the control's own state changed ( -> gone) (inline change only; no request fired) | PASS |
| r00082 | admin | /studio/claims | mobile | zh-TW | main | 新增關鍵字 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00083 | admin | / | desktop | zh-TW | main | 待出貨訂單 2 (`todo-ship`) | click | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?state=unshipped&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00084 | admin | /orders | desktop | zh-TW | main | 全部 5 (`orders-bucket-all`) | click | visible change | DOM changed (11 mutations); the control's own state changed (\|\|\|\|true -> gone) | PASS |
| r00085 | admin | /studio/claims | mobile | zh-TW | main | 加入關鍵字庫 (`claims-add-library`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00086 | admin | / | desktop | zh-TW | main | 待建立的超商托運單 1 (`todo-label`) | click | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?state=unshipped&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00087 | admin | /studio/claims | mobile | zh-TW | main | 買家 | selectOption | visible change | select has a single option | SKIP |
| r00088 | admin | /orders | desktop | zh-TW | main | 待付款 4 (`orders-bucket-unpaid`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00089 | admin | / | desktop | zh-TW | main | 低庫存規格（5 件以下） 0 (`todo-stock`) | click | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (16 mutations) | PASS |
| r00090 | admin | /studio/claims | mobile | zh-TW | main | 記錄留言 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00091 | admin | /orders | desktop | zh-TW | main | 待核對轉帳 1 (`orders-bucket-transfer_review`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00092 | admin | /studio/claims | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00093 | admin | / | desktop | zh-TW | main | 處理中的退款 0 (`todo-refunds`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) (inline change only; no request fired) | PASS |
| r00094 | admin | /orders | desktop | zh-TW | main | 待出貨 2 (`orders-bucket-ready_to_ship`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00095 | admin | /studio/claims | desktop | en | main | Overview | click | navigation or in-page change | url /en/studio/claims?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&scene=343c278a-3f12-4e8c-b5ca-561c95af9941 -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; D | PASS |
| r00096 | admin | /orders | desktop | zh-TW | main | 待交寄 1 (`orders-bucket-ready_to_consign`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00097 | admin | / | desktop | zh-TW | main | 04a8d099 | click | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?order=04a8d099-7893-4c42-a43b-85b853f4e94d&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DO | PASS |
| r00098 | admin | /studio/claims | desktop | en | main | Live Studio | click | visible change | url /en/studio/claims?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&scene=343c278a-3f12-4e8c-b5ca-561c95af9941 -> /en/studio?store=26dc24c3-776c-4adf-943d-128f1b2b | PASS |
| r00099 | admin | /orders | desktop | zh-TW | main | 已出貨 0 (`orders-bucket-shipped`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00100 | admin | / | desktop | zh-TW | main | 3841287e | click | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?order=3841287e-8601-4363-9e8a-963d17dfbe96&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DO | PASS |
| r00101 | admin | /studio/claims | desktop | en | main | Refresh facts | click | visible change | DOM changed (2 mutations) | PASS |
| r00102 | admin | /orders | desktop | zh-TW | main | 已完成 0 (`orders-bucket-completed`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00103 | admin | / | desktop | zh-TW | main | d4b60cbc | click | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?order=d4b60cbc-e494-43a0-a3af-0c3b6d93c710&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DO | PASS |
| r00104 | admin | /studio/claims | desktop | en | main | Quantity rule | selectOption(KEYWORD_QTY_ONLY) | visible change | the control's own state changed (EXACT -> KEYWORD_QTY_ONLY) | PASS |
| r00105 | admin | /orders | desktop | zh-TW | main | 已取消 0 (`orders-bucket-cancelled`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00106 | admin | / | desktop | zh-TW | main | eb15d16b | click | navigation or in-page change | scrolled | PASS |
| r00107 | admin | /studio/claims | desktop | en | main | Open claim window | click | visible change | DOM changed (21 mutations) | PASS |
| r00108 | admin | /orders | desktop | zh-TW | main | 展開訂單詳情: 04a8d099-7893-4c42-a43b-85b853f4e94d (`order-expand-04a8d099-7893-4c42-a43b-85b853f4e94d`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00109 | admin | /studio/claims | desktop | en | main | Page for this post or live stream | selectOption(10505210174913264572) | visible change | the control's own state changed ( -> 10505210174913264572) | PASS |
| r00110 | admin | / | desktop | zh-TW | main | 3c9eca55 | click | navigation or in-page change | scrolled | PASS |
| r00111 | admin | /studio/claims | desktop | en | main | Platform | selectOption(facebook) | visible change | the control's own state changed ( -> facebook) | PASS |
| r00112 | admin | /orders | desktop | zh-TW | main | 展開訂單詳情: 3841287e-8601-4363-9e8a-963d17dfbe96 (`order-expand-3841287e-8601-4363-9e8a-963d17dfbe96`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00113 | admin | / | desktop | zh-TW | main | 09f268dc | click | navigation or in-page change | scrolled | PASS |
| r00114 | admin | /studio/claims | desktop | en | main | Reply language | selectOption(zh-CN) | visible change | the control's own state changed (en -> zh-CN) | PASS |
| r00115 | admin | /orders | desktop | zh-TW | main | 展開訂單詳情: 3c9eca55-fab8-47c1-9cf7-3b06063b938e (`order-expand-3c9eca55-fab8-47c1-9cf7-3b06063b938e`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00116 | admin | / | desktop | zh-TW | main | 全部訂單 | click | navigation or in-page change | scrolled | PASS |
| r00117 | admin | /studio/claims | desktop | en | main | Send a private reply with the cart link | click (toggle) | visible change | the control's own state changed (false -> true) | PASS |
| r00118 | admin | /orders | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00119 | admin | / | desktop | zh-TW | main | 建立訂單 (`action-create-order`) | click | navigation or in-page change | scrolled | PASS |
| r00120 | admin | /orders | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00121 | admin | /studio/claims | desktop | en | main | Collect comments from this source | click (toggle) | visible change | the control's own state changed (true -> false) | PASS |
| r00122 | admin | / | desktop | zh-TW | main | 匯入或匯出商品 (`action-import`) | click | navigation or in-page change | scrolled | PASS |
| r00123 | admin | /orders | mobile | zh-TW | main | 更多篩選 (`orders-more-filters`) | click | visible change | DOM changed (3 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00124 | admin | /studio/claims | desktop | en | main | Save comment source | click | visible change | DOM changed (2 mutations); validation shown | PASS |
| r00125 | admin | / | desktop | zh-TW | main | 庫存 (`action-inventory`) | click | navigation or in-page change | scrolled | PASS |
| r00126 | admin | /studio/claims | desktop | en | main | Product | selectOption(1f957810-4949-430d-9b43-1e1c1b0f9a46) | visible change | the control's own state changed ( -> 1f957810-4949-430d-9b43-1e1c1b0f9a46) | PASS |
| r00127 | admin | /orders | mobile | zh-TW | main | 重新整理 (`orders-refresh`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00128 | admin | / | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00129 | admin | /studio/claims | desktop | en | main | Add offer | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00130 | admin | /orders | mobile | zh-TW | main | 匯出未出貨訂單（CSV） (`orders-export`) | click | navigation or in-page change | download: unshipped-26dc24c3-202610031917.csv | PASS |
| r00131 | admin | /studio/claims | desktop | en | main | Add to library (`claims-add-library`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00132 | admin | /orders | mobile | zh-TW | main | 套用篩選 (`orders-apply`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00133 | admin | /studio/claims | desktop | en | main | Buyer | selectOption | visible change | select has a single option | SKIP |
| r00134 | admin | /orders | mobile | zh-TW | main | 清除篩選 (`orders-reset`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (11 mutations); the control's own state changed ( -> gone) (inline change only; no request fired) | PASS |
| r00135 | admin | /studio/claims | desktop | en | main | Record comment | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00136 | admin | /orders | mobile | zh-TW | main | 全部 5 (`orders-bucket-all`) | click | visible change | DOM changed (11 mutations); the control's own state changed (\|\|\|\|true -> gone) | PASS |
| r00137 | admin | /orders/new | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00138 | admin | /orders | mobile | zh-TW | main | 待付款 4 (`orders-bucket-unpaid`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00139 | admin | /orders/new | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders/new?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00140 | admin | /orders | mobile | zh-TW | main | 待核對轉帳 1 (`orders-bucket-transfer_review`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00141 | admin | /orders/new | desktop | zh-TW | main | 搜尋 (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00142 | admin | /orders | mobile | zh-TW | main | 待出貨 2 (`orders-bucket-ready_to_ship`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00143 | admin | /orders/new | desktop | zh-TW | main | 配送方式 (`mo-option`) | selectOption(601fc709-6ebd-4229-bf56-97f5dd23e1a2\|TW\|cvs28bfffe04e73) | visible change | DOM changed (4 mutations); the control's own state changed ( -> 601fc709-6ebd-4229-bf56-97f5dd23e1a2\|TW\|cvs28bfffe04e73) | PASS |
| r00144 | admin | / | mobile | zh-TW | skip | 跳至內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec#main; scrolled | PASS |
| r00145 | admin | /orders | mobile | zh-TW | main | 待交寄 1 (`orders-bucket-ready_to_consign`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00146 | admin | /orders/new | desktop | zh-TW | main | 顧客連結 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN) | PASS |
| r00147 | admin | /orders | mobile | zh-TW | main | 展開訂單詳情: 04a8d099-7893-4c42-a43b-85b853f4e94d (`order-expand-04a8d099-7893-4c42-a43b-85b853f4e94d`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00148 | admin | / | mobile | zh-TW | topbar | 開啟導覽 | click | visible change | DOM changed (3 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00149 | admin | /orders/new | desktop | zh-TW | main | 回到訂單 | click | navigation or in-page change | url /zh-TW/orders/new?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (19 mutations) | PASS |
| r00150 | admin | / | mobile | zh-TW | topbar | 切換商店 (`shell-store-selector`) | selectOption | visible change | select has a single option | SKIP |
| r00151 | admin | /orders | mobile | zh-TW | main | 展開訂單詳情: 3841287e-8601-4363-9e8a-963d17dfbe96 (`order-expand-3841287e-8601-4363-9e8a-963d17dfbe96`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00152 | admin | /orders/new | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00153 | admin | / | mobile | zh-TW | topbar | 語言 (`locale-switch`) | selectOption(zh-CN) | visible change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-CN?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (17 mutations) | PASS |
| r00154 | admin | /orders/new | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders/new?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00155 | admin | /orders | mobile | zh-TW | main | 展開訂單詳情: 3c9eca55-fab8-47c1-9cf7-3b06063b938e (`order-expand-3c9eca55-fab8-47c1-9cf7-3b06063b938e`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00156 | admin | / | mobile | zh-TW | topbar | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00157 | admin | /orders/new | mobile | zh-TW | main | 搜尋 (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00158 | admin | /orders | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00159 | admin | /orders/new | mobile | zh-TW | main | 配送方式 (`mo-option`) | selectOption(601fc709-6ebd-4229-bf56-97f5dd23e1a2\|TW\|cvs28bfffe04e73) | visible change | DOM changed (4 mutations); the control's own state changed ( -> 601fc709-6ebd-4229-bf56-97f5dd23e1a2\|TW\|cvs28bfffe04e73); scrolled | PASS |
| r00160 | admin | /orders | desktop | en | main | Overview | click | navigation or in-page change | url /en/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00161 | admin | / | mobile | zh-TW | layer of 說明 | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00162 | admin | /orders/new | mobile | zh-TW | main | 顧客連結 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN); scrolled | PASS |
| r00163 | admin | /orders | desktop | en | main | Order status (`state-filter`) | selectOption(all) | visible change | DOM changed (8 mutations) | PASS |
| r00164 | admin | / | mobile | zh-TW | topbar | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00165 | admin | /orders/new | mobile | zh-TW | main | 回到訂單 | click | navigation or in-page change | scrolled | PASS |
| r00166 | admin | /orders | desktop | en | main | Refresh (`orders-refresh`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00167 | admin | / | mobile | zh-TW | layer of 帳號 | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00168 | admin | /orders/new | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00169 | admin | /orders | desktop | en | main | Export unshipped (CSV) (`orders-export`) | click | navigation or in-page change | download: unshipped-26dc24c3-202610031917.csv | PASS |
| r00170 | admin | / | mobile | zh-TW | layer of 帳號 | 登出 (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00171 | admin | /orders/new | desktop | en | main | Overview | click | navigation or in-page change | url /en/orders/new?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00172 | admin | /orders | desktop | en | main | Payment method (`orders-payment-filter`) | selectOption(card) | visible change | the control's own state changed ( -> card) | PASS |
| r00173 | admin | /orders/new | desktop | en | main | Search (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00174 | admin | /orders | desktop | en | main | Delivery method (`orders-delivery-filter`) | selectOption(home) | visible change | the control's own state changed ( -> home) | PASS |
| r00175 | admin | / | mobile | zh-TW | rail | 關閉導覽 | click | visible change | DOM changed (3 mutations) | PASS |
| r00176 | admin | /orders/new | desktop | en | main | Delivery method (`mo-option`) | selectOption(601fc709-6ebd-4229-bf56-97f5dd23e1a2\|TW\|cvs28bfffe04e73) | visible change | DOM changed (4 mutations); the control's own state changed ( -> 601fc709-6ebd-4229-bf56-97f5dd23e1a2\|TW\|cvs28bfffe04e73) | PASS |
| r00177 | admin | /orders | desktop | en | main | Live session (`orders-session-filter`) | selectOption | visible change | select has a single option | SKIP |
| r00178 | admin | /orders/new | desktop | en | main | Customer link | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN) | PASS |
| r00179 | admin | / | mobile | zh-TW | rail | 總覽 (`nav-group-overview`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00180 | admin | /orders | desktop | en | main | Apply filters (`orders-apply`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00181 | admin | /orders/new | desktop | en | main | Back to orders | click | navigation or in-page change | url /en/orders/new?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (19 mutations) | PASS |
| r00182 | admin | /orders | desktop | en | main | Clear filters (`orders-reset`) | click | visible change | DOM changed (11 mutations); the control's own state changed ( -> gone) | PASS |
| r00183 | admin | / | mobile | zh-TW | rail | 直播與貼文 (`nav-group-live`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00184 | admin | /orders/cvs-print | desktop | zh-TW | page | (page load) | - | the route hands the browser to /ecpay/i | HTTP 200; POST https://logistics-stage.ecpay.com.tw/Express/PrintUniMartC2COrderInfo (blocked by the sweep) | PASS |
| r00185 | admin | /orders/cvs-print | mobile | zh-TW | page | (page load) | - | the route hands the browser to /ecpay/i | HTTP 200; POST https://logistics-stage.ecpay.com.tw/Express/PrintUniMartC2COrderInfo (blocked by the sweep) | PASS |
| r00186 | admin | /orders | desktop | en | main | All 5 (`orders-bucket-all`) | click | visible change | DOM changed (11 mutations); the control's own state changed (\|\|\|\|true -> gone) | PASS |
| r00187 | admin | / | mobile | zh-TW | rail | 訂單與出貨 (`nav-orders`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00188 | admin | /orders/cvs-print | desktop | en | page | (page load) | - | the route hands the browser to /ecpay/i | HTTP 200; POST https://logistics-stage.ecpay.com.tw/Express/PrintUniMartC2COrderInfo (blocked by the sweep) | PASS |
| r00189 | admin | /products | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00190 | admin | /orders | desktop | en | main | Awaiting payment 4 (`orders-bucket-unpaid`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00191 | admin | / | mobile | zh-TW | rail | 商品與庫存 + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00192 | admin | /products | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00193 | admin | /orders | desktop | en | main | Review transfers 1 (`orders-bucket-transfer_review`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00194 | admin | /products | desktop | zh-TW | main | 庫存帳 (`products-ledger-link`) | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (16 mutations) | PASS |
| r00195 | admin | / | mobile | zh-TW | rail | 顧客 (`nav-group-customers`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00196 | admin | /orders | desktop | en | main | Ready to ship 2 (`orders-bucket-ready_to_ship`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00197 | admin | /products | desktop | zh-TW | main | 新增商品 (`product-new`) | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products/new?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (19 mutations) | PASS |
| r00198 | admin | / | mobile | zh-TW | rail | 行銷優惠 + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00199 | admin | /orders | desktop | en | main | Ready to drop off 1 (`orders-bucket-ready_to_consign`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00200 | admin | /products | desktop | zh-TW | main | 全部 3 (`products-tab-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00201 | admin | /orders | desktop | en | main | Shipped 0 (`orders-bucket-shipped`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00202 | admin | / | mobile | zh-TW | rail | 網路商店 (`nav-group-storefront`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00203 | admin | /products | desktop | zh-TW | main | 草稿 0 (`products-tab-draft`) | click | visible change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&status=draft; DOM changed (17 mutat | PASS |
| r00204 | admin | /orders | desktop | en | main | Completed 0 (`orders-bucket-completed`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00205 | admin | / | mobile | zh-TW | rail | 收款與報表 (`nav-group-finance`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00206 | admin | /products | desktop | zh-TW | main | 上架中 3 (`products-tab-active`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&status=active; DOM changed (17 muta (inline change only; no request fired) | PASS |
| r00207 | admin | /orders | desktop | en | main | Cancelled 0 (`orders-bucket-cancelled`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00208 | admin | / | mobile | zh-TW | rail | 設定 + (`nav-group-settings`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00209 | admin | /orders | desktop | en | main | Show order details: 04a8d099-7893-4c42-a43b-85b853f4e94d (`order-expand-04a8d099-7893-4c42-a43b-85b853f4e94d`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00210 | admin | /products | desktop | zh-TW | main | 已封存 0 (`products-tab-archived`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&status=archived; DOM changed (17 mu (inline change only; no request fired) | PASS |
| r00211 | admin | /orders | desktop | en | main | Show order details: 3841287e-8601-4363-9e8a-963d17dfbe96 (`order-expand-3841287e-8601-4363-9e8a-963d17dfbe96`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00212 | admin | /products | desktop | zh-TW | main | 搜尋 (`products-search-submit`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00213 | admin | / | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | DOM changed (10 mutations) | PASS |
| r00214 | admin | /orders | desktop | en | main | Show order details: 3c9eca55-fab8-47c1-9cf7-3b06063b938e (`order-expand-3c9eca55-fab8-47c1-9cf7-3b06063b938e`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00215 | admin | /products | desktop | zh-TW | main | 全部 | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00216 | admin | / | mobile | zh-TW | main | 待確認的轉帳 1 (`todo-transfer`) | click | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?state=AWAITING_TRANSFER&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (14 mutat | PASS |
| r00217 | admin | /products/[product] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00218 | admin | /products | desktop | zh-TW | main | 選取商品: Sweep Ceramic Mug | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00219 | admin | / | mobile | zh-TW | main | 待出貨訂單 2 (`todo-ship`) | click | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?state=unshipped&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (14 mutations) | PASS |
| r00220 | admin | /products/[product] | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/1f957810-4949-430d-9b43-1e1c1b0f9a46?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM ch | PASS |
| r00221 | admin | /products | desktop | zh-TW | main | 編輯: Sweep Ceramic Mug | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products/209766aa-a6d5-4d04-9b26-243b1b8745ca?store=26dc24c3-776c-4adf-943d-128f1b2b0ee | PASS |
| r00222 | admin | / | mobile | zh-TW | main | 待建立的超商托運單 1 (`todo-label`) | click | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?state=unshipped&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (14 mutations) | PASS |
| r00223 | admin | /products/[product] | desktop | zh-TW | main | ← 全部商品 (`product-back`) | click | navigation or in-page change | url /zh-TW/products/1f957810-4949-430d-9b43-1e1c1b0f9a46?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0ee | PASS |
| r00224 | admin | /products | desktop | zh-TW | main | 修改價格: Sweep Ceramic Mug (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00225 | admin | /products | desktop | zh-TW | layer of 修改價格: Sweep Ceramic Mug | 儲存修改 (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00226 | admin | / | mobile | zh-TW | main | 低庫存規格（5 件以下） 0 (`todo-stock`) | click | navigation or in-page change | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (16 mutations) | PASS |
| r00227 | admin | /products/[product] | desktop | zh-TW | main | 商品圖片 | click | visible change | scrolled | PASS |
| r00228 | admin | /products | desktop | zh-TW | main | 修改庫存: Sweep Ceramic Mug (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00229 | admin | / | mobile | zh-TW | main | 處理中的退款 0 (`todo-refunds`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (14 mutations) (inline change only; no request fired) | PASS |
| r00230 | admin | /products/[product] | desktop | zh-TW | main | 基本資訊 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00231 | admin | /products | desktop | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products/209766aa-a6d5-4d04-9b26-243b1b8745ca?store=26dc24c3-776c-4adf-943d-128f1b2b0ee [1 of 3 alike] | PASS |
| r00232 | admin | / | mobile | zh-TW | main | 04a8d099 | click | navigation or in-page change | scrolled | PASS |
| r00233 | admin | /products/[product] | desktop | zh-TW | main | 價格與庫存 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00234 | admin | /products | desktop | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00235 | admin | / | mobile | zh-TW | main | 3841287e | click | navigation or in-page change | scrolled | PASS |
| r00236 | admin | /products/[product] | desktop | zh-TW | main | 規格 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00237 | admin | /products | desktop | zh-TW | main | 選取商品: Sweep Wool Scarf | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00238 | admin | / | mobile | zh-TW | main | d4b60cbc | click | navigation or in-page change | scrolled | PASS |
| r00239 | admin | /products/[product] | desktop | zh-TW | main | 分類 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00240 | admin | /products | desktop | zh-TW | main | 編輯: Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products/998ef94f-7b52-489a-91e3-4e6076f8d703?store=26dc24c3-776c-4adf-943d-128f1b2b0ee | PASS |
| r00241 | admin | / | mobile | zh-TW | main | eb15d16b | click | navigation or in-page change | scrolled | PASS |
| r00242 | admin | /products/[product] | desktop | zh-TW | main | 物流 | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00243 | admin | /products | desktop | zh-TW | main | 修改價格: Sweep Wool Scarf (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00244 | admin | / | mobile | zh-TW | main | 3c9eca55 | click | navigation or in-page change | scrolled | PASS |
| r00245 | admin | /products/[product] | desktop | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00246 | admin | /products | desktop | zh-TW | main | 修改庫存: Sweep Wool Scarf (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00247 | admin | / | mobile | zh-TW | main | 09f268dc | click | navigation or in-page change | scrolled | PASS |
| r00248 | admin | /products/[product] | desktop | zh-TW | main | 圖片 ○ | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00249 | admin | /products | desktop | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products/998ef94f-7b52-489a-91e3-4e6076f8d703?store=26dc24c3-776c-4adf-943d-128f1b2b0ee [1 of 3 alike] | PASS |
| r00250 | admin | / | mobile | zh-TW | main | 全部訂單 | click | navigation or in-page change | scrolled | PASS |
| r00251 | admin | /products/[product] | desktop | zh-TW | main | 商品名稱 ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00252 | admin | /products | desktop | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00253 | admin | / | mobile | zh-TW | main | 建立訂單 (`action-create-order`) | click | navigation or in-page change | scrolled | PASS |
| r00254 | admin | /products/[product] | desktop | zh-TW | main | 售價 ○ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00255 | admin | /products | desktop | zh-TW | main | 選取商品: Sweep Cedar Candle | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00256 | admin | / | mobile | zh-TW | main | 匯入或匯出商品 (`action-import`) | click | navigation or in-page change | scrolled | PASS |
| r00257 | admin | /products/[product] | desktop | zh-TW | main | 庫存 ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00258 | admin | /products | desktop | zh-TW | main | 編輯: Sweep Cedar Candle | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products/1f957810-4949-430d-9b43-1e1c1b0f9a46?store=26dc24c3-776c-4adf-943d-128f1b2b0ee | PASS |
| r00259 | admin | / | mobile | zh-TW | main | 庫存 (`action-inventory`) | click | navigation or in-page change | scrolled | PASS |
| r00260 | admin | /products/[product] | desktop | zh-TW | main | 圖片 ○ | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00261 | admin | /products | desktop | zh-TW | main | 修改價格: Sweep Cedar Candle (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00262 | admin | / | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00263 | admin | /products/[product] | desktop | zh-TW | main | 描述 ✓ | click | visible change | scrolled | PASS |
| r00264 | admin | /products | desktop | zh-TW | main | 修改庫存: Sweep Cedar Candle (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00265 | admin | /products/[product] | desktop | zh-TW | main | 分類 ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00266 | admin | /products | desktop | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products/1f957810-4949-430d-9b43-1e1c1b0f9a46?store=26dc24c3-776c-4adf-943d-128f1b2b0ee [1 of 3 alike] | PASS |
| r00267 | admin | /products/[product] | desktop | zh-TW | main | 直播關鍵字 ○ | click | visible change | scrolled | PASS |
| r00268 | admin | /products | desktop | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00269 | admin | /products/[product] | desktop | zh-TW | main | 搜尋引擎最佳化 ○ | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00270 | admin | /products | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00271 | admin | /products/[product] | desktop | zh-TW | main | 顯示狀態上架中：商店發布後，買家可以看到並購買。 (`product-status`) | selectOption(draft) | confirmation layer opens; cancel; nothing changed | DOM changed (2 mutations); the control's own state changed (active -> draft) (inline change only; no request fired) | PASS |
| r00272 | admin | /products | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00273 | admin | /products/[product] | desktop | zh-TW | main | 移除 規格名稱 1 | click | confirmation layer opens; cancel; nothing changed | DOM changed (10 mutations); the control's own state changed ( -> true); scrolled (inline change only; no request fired) | PASS |
| r00274 | admin | /products | mobile | zh-TW | main | 庫存帳 (`products-ledger-link`) | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (16 mutations) | PASS |
| r00275 | admin | /products/[product] | desktop | zh-TW | main | 新增規格（顏色、尺寸…） (`axis-add`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00276 | admin | /products | mobile | zh-TW | main | 新增商品 (`product-new`) | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products/new?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (19 mutations) | PASS |
| r00277 | admin | / | desktop | en | skip | Skip to content | keyboard: Tab to the link, Enter | navigation or in-page change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec#main; scrolled | PASS |
| r00278 | admin | /products/[product] | desktop | zh-TW | main | 售價 · 批量填入 (`bulk-price`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00279 | admin | /products | mobile | zh-TW | main | 全部 3 (`products-tab-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00280 | admin | / | desktop | en | topbar | Switch store (`shell-store-selector`) | selectOption | visible change | select has a single option | SKIP |
| r00281 | admin | /products/[product] | desktop | zh-TW | main | 原價 · 批量填入 (`bulk-compare`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00282 | admin | /products | mobile | zh-TW | main | 草稿 0 (`products-tab-draft`) | click | visible change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&status=draft; DOM changed (17 mutat | PASS |
| r00283 | admin | / | desktop | en | topbar | Language (`locale-switch`) | selectOption(zh-CN) | visible change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-CN?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (17 mutations) | PASS |
| r00284 | admin | /products/[product] | desktop | zh-TW | main | 數量 · 批量填入 (`bulk-quantity`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00285 | admin | /products | mobile | zh-TW | main | 上架中 3 (`products-tab-active`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&status=active; DOM changed (17 muta (inline change only; no request fired) | PASS |
| r00286 | admin | / | desktop | en | topbar | Help | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00287 | admin | /products/[product] | desktop | zh-TW | main | 貨號 · 批量填入 (`bulk-code`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00288 | admin | /products | mobile | zh-TW | main | 已封存 0 (`products-tab-archived`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&status=archived; DOM changed (17 mu (inline change only; no request fired) | PASS |
| r00289 | admin | / | desktop | en | layer of Help | Help | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00290 | admin | /products/[product] | desktop | zh-TW | main | 直播關鍵字 · 批量填入 (`bulk-keyword`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00291 | admin | /products | mobile | zh-TW | main | 搜尋 (`products-search-submit`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00292 | admin | / | desktop | en | topbar | Account | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00293 | admin | /products/[product] | desktop | zh-TW | main | 不追蹤 ∞ | click (toggle) | visible change | DOM changed (5 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00294 | admin | /products | mobile | zh-TW | main | 全部 | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00295 | admin | /products/[product] | desktop | zh-TW | main | 啟用 1 (`matrix-active-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00296 | admin | / | desktop | en | layer of Account | Account | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00297 | admin | /products | mobile | zh-TW | main | 選取商品: Sweep Ceramic Mug | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00298 | admin | / | desktop | en | layer of Account | Sign out (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00299 | admin | /products/[product] | desktop | zh-TW | main | Sweep home | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00300 | admin | /products | mobile | zh-TW | main | 編輯: Sweep Ceramic Mug | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products/209766aa-a6d5-4d04-9b26-243b1b8745ca?store=26dc24c3-776c-4adf-943d-128f1b2b0ee | PASS |
| r00301 | admin | /products/[product] | desktop | zh-TW | main | Sweep wear | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00302 | admin | / | desktop | en | rail | Overview (`nav-group-overview`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00303 | admin | /products | mobile | zh-TW | main | 修改價格: Sweep Ceramic Mug (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00304 | admin | /products | mobile | zh-TW | layer of 修改價格: Sweep Ceramic Mug | 儲存修改 (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00305 | admin | /products/[product] | desktop | zh-TW | main | 管理分類 | click | navigation or in-page change | scrolled | PASS |
| r00306 | admin | / | desktop | en | rail | Live & posts (`nav-group-live`) | click | visible change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/studio?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (21 mutations) | PASS |
| r00307 | admin | /products | mobile | zh-TW | main | 修改庫存: Sweep Ceramic Mug (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00308 | admin | /products/[product] | desktop | zh-TW | main | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00309 | admin | / | desktop | en | rail | Orders & shipping (`nav-orders`) | click | visible change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00310 | admin | /products | mobile | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products/209766aa-a6d5-4d04-9b26-243b1b8745ca?store=26dc24c3-776c-4adf-943d-128f1b2b0ee [1 of 3 alike] | PASS |
| r00311 | admin | /products/[product] | desktop | zh-TW | layer of 物流 | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00312 | admin | / | desktop | en | rail | Products & inventory + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00313 | admin | /products | mobile | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00314 | admin | /products/[product] | desktop | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00315 | admin | / | desktop | en | rail | Customers (`nav-group-customers`) | click | visible change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00316 | admin | /products | mobile | zh-TW | main | 選取商品: Sweep Wool Scarf | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ); scrolled | PASS |
| r00317 | admin | / | desktop | en | rail | Marketing + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00318 | admin | /products/[product] | desktop | zh-TW | layer of 搜尋引擎最佳化 | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00319 | admin | /products | mobile | zh-TW | main | 編輯: Sweep Wool Scarf | click | navigation or in-page change | scrolled | PASS |
| r00320 | admin | / | desktop | en | rail | Online store (`nav-group-storefront`) | click | visible change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/design?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00321 | admin | /products/[product] | desktop | zh-TW | main | 儲存修改 (`product-save`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00322 | admin | /products | mobile | zh-TW | main | 修改價格: Sweep Wool Scarf (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations); scrolled | PASS |
| r00323 | admin | / | desktop | en | rail | Payments & reports (`nav-group-finance`) | click | visible change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/finance?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00324 | admin | /products/[product] | desktop | zh-TW | main | 下架轉草稿 (`product-unpublish`) | click | confirmation layer opens; cancel; nothing changed | dialog: confirm: 確定下架此商品？買家將無法再購買此商品。 (confirmation shown, then cancelled) | PASS |
| r00325 | admin | /products | mobile | zh-TW | main | 修改庫存: Sweep Wool Scarf (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations); scrolled | PASS |
| r00326 | admin | / | desktop | en | rail | Settings + (`nav-group-settings`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00327 | admin | /products/[product] | desktop | zh-TW | main | 上架 (`product-publish`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (1 mutations) (inline change only; no request fired) | PASS |
| r00328 | admin | /products | mobile | zh-TW | main | 編輯 | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00329 | admin | /products/[product] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00330 | admin | /products | mobile | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations); scrolled [1 of 3 alike] | PASS |
| r00331 | admin | / | desktop | en | main | Overview | click | navigation or in-page change | DOM changed (10 mutations) | PASS |
| r00332 | admin | /products/[product] | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/1f957810-4949-430d-9b43-1e1c1b0f9a46?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM ch | PASS |
| r00333 | admin | /products | mobile | zh-TW | main | 選取商品: Sweep Cedar Candle | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ); scrolled | PASS |
| r00334 | admin | / | desktop | en | main | Transfers to confirm 1 (`todo-transfer`) | click | navigation or in-page change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/orders?state=AWAITING_TRANSFER&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (19 mutations) | PASS |
| r00335 | admin | /products/[product] | mobile | zh-TW | main | ← 全部商品 (`product-back`) | click | navigation or in-page change | url /zh-TW/products/1f957810-4949-430d-9b43-1e1c1b0f9a46?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products?store=26dc24c3-776c-4adf-943d-128f1b2b0ee | PASS |
| r00336 | admin | /products | mobile | zh-TW | main | 編輯: Sweep Cedar Candle | click | navigation or in-page change | scrolled | PASS |
| r00337 | admin | / | desktop | en | main | Orders to ship 2 (`todo-ship`) | click | navigation or in-page change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/orders?state=unshipped&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00338 | admin | /products/[product] | mobile | zh-TW | main | 商品圖片 | click | visible change | scrolled | PASS |
| r00339 | admin | /products | mobile | zh-TW | main | 修改價格: Sweep Cedar Candle (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations); scrolled | PASS |
| r00340 | admin | /products/[product] | mobile | zh-TW | main | 基本資訊 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00341 | admin | / | desktop | en | main | Convenience-store labels to create 1 (`todo-label`) | click | navigation or in-page change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/orders?state=unshipped&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00342 | admin | /products | mobile | zh-TW | main | 修改庫存: Sweep Cedar Candle (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations); scrolled | PASS |
| r00343 | admin | /products/[product] | mobile | zh-TW | main | 價格與庫存 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00344 | admin | / | desktop | en | main | Low-stock variants (5 or fewer) 0 (`todo-stock`) | click | navigation or in-page change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (16 mutations) | PASS |
| r00345 | admin | /products | mobile | zh-TW | main | 編輯 | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00346 | admin | /products/[product] | mobile | zh-TW | main | 規格 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00347 | admin | / | desktop | en | main | Open refunds 0 (`todo-refunds`) | click | navigation or in-page change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00348 | admin | /products | mobile | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations); scrolled [1 of 3 alike] | PASS |
| r00349 | admin | /products/[product] | mobile | zh-TW | main | 分類 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00350 | admin | / | desktop | en | main | 04a8d099 | click | navigation or in-page change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/orders?order=04a8d099-7893-4c42-a43b-85b853f4e94d&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM chan | PASS |
| r00351 | admin | /products | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00352 | admin | /products/[product] | mobile | zh-TW | main | 物流 | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00353 | admin | /products | desktop | en | main | Overview | click | navigation or in-page change | url /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00354 | admin | / | desktop | en | main | 3841287e | click | navigation or in-page change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/orders?order=3841287e-8601-4363-9e8a-963d17dfbe96&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM chan | PASS |
| r00355 | admin | /products/[product] | mobile | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00356 | admin | /products | desktop | en | main | Inventory ledger (`products-ledger-link`) | click | navigation or in-page change | url /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (16 mutations) | PASS |
| r00357 | admin | / | desktop | en | main | d4b60cbc | click | navigation or in-page change | url /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/orders?order=d4b60cbc-e494-43a0-a3af-0c3b6d93c710&store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM chan | PASS |
| r00358 | admin | /products/[product] | mobile | zh-TW | main | 顯示狀態上架中：商店發布後，買家可以看到並購買。 (`product-status`) | selectOption(draft) | confirmation layer opens; cancel; nothing changed | DOM changed (2 mutations); the control's own state changed (active -> draft) (inline change only; no request fired) | PASS |
| r00359 | admin | /products | desktop | en | main | Add product (`product-new`) | click | navigation or in-page change | url /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/products/new?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (19 mutations) | PASS |
| r00360 | admin | / | desktop | en | main | eb15d16b | click | navigation or in-page change | scrolled | PASS |
| r00361 | admin | /products/[product] | mobile | zh-TW | main | 移除 規格名稱 1 | click | confirmation layer opens; cancel; nothing changed | DOM changed (10 mutations); the control's own state changed ( -> true); scrolled (inline change only; no request fired) | PASS |
| r00362 | admin | /products | desktop | en | main | All 3 (`products-tab-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00363 | admin | / | desktop | en | main | 3c9eca55 | click | navigation or in-page change | scrolled | PASS |
| r00364 | admin | /products/[product] | mobile | zh-TW | main | 新增規格（顏色、尺寸…） (`axis-add`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00365 | admin | / | desktop | en | main | 09f268dc | click | navigation or in-page change | scrolled | PASS |
| r00366 | admin | /products | desktop | en | main | Draft 0 (`products-tab-draft`) | click | visible change | url /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&status=draft; DOM changed (17 mutations) | PASS |
| r00367 | admin | /products/[product] | mobile | zh-TW | main | 售價 · 批量填入 (`bulk-price`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00368 | admin | / | desktop | en | main | All orders | click | navigation or in-page change | scrolled | PASS |
| r00369 | admin | /products | desktop | en | main | Active 3 (`products-tab-active`) | click | visible change | url /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&status=active; DOM changed (17 mutations) | PASS |
| r00370 | admin | /products/[product] | mobile | zh-TW | main | 原價 · 批量填入 (`bulk-compare`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00371 | admin | / | desktop | en | main | Create an order (`action-create-order`) | click | navigation or in-page change | scrolled | PASS |
| r00372 | admin | /products | desktop | en | main | Archived 0 (`products-tab-archived`) | click | visible change | url /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&status=archived; DOM changed (17 mutation | PASS |
| r00373 | admin | /products/[product] | mobile | zh-TW | main | 數量 · 批量填入 (`bulk-quantity`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00374 | admin | / | desktop | en | main | Import or export products (`action-import`) | click | navigation or in-page change | scrolled | PASS |
| r00375 | admin | /products | desktop | en | main | Search (`products-search-submit`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00376 | admin | /products/[product] | mobile | zh-TW | main | 貨號 · 批量填入 (`bulk-code`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00377 | admin | / | desktop | en | main | Inventory (`action-inventory`) | click | navigation or in-page change | scrolled | PASS |
| r00378 | admin | /products | desktop | en | main | All | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00379 | admin | /products/[product] | mobile | zh-TW | main | 直播關鍵字 · 批量填入 (`bulk-keyword`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00380 | admin | /collections | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00381 | admin | /products | desktop | en | main | Select product: Sweep Ceramic Mug | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00382 | admin | /products/[product] | mobile | zh-TW | main | 不追蹤 ∞ | click (toggle) | visible change | DOM changed (5 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00383 | admin | /collections | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/collections?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00384 | admin | /products | desktop | en | main | Edit: Sweep Ceramic Mug | click | navigation or in-page change | url /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/products/209766aa-a6d5-4d04-9b26-243b1b8745ca?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM | PASS |
| r00385 | admin | /products/[product] | mobile | zh-TW | main | 啟用 1 (`matrix-active-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00386 | admin | /collections | desktop | zh-TW | main | 新增集合 (`collection-new`) | click | visible change | DOM changed (3 mutations); validation shown | PASS |
| r00387 | admin | /products | desktop | en | main | Edit price: Sweep Ceramic Mug (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00388 | admin | /products/[product] | mobile | zh-TW | main | Sweep home | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00389 | admin | /collections | desktop | zh-TW | main | Sweep home /sweep-home · 2 件商品 (`collection-item-dba959f1-a29a-4886-a4f1-69ba03adaa14`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00390 | admin | /products | desktop | en | main | Edit inventory: Sweep Ceramic Mug (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00391 | admin | /products/[product] | mobile | zh-TW | main | Sweep wear | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00392 | admin | /collections | desktop | zh-TW | main | Sweep wear /sweep-wear · 2 件商品 (`collection-item-c989e2dd-7b39-422e-9b61-ed233c8235a8`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00393 | admin | /products | desktop | en | main | Edit | click | navigation or in-page change | url /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/products/209766aa-a6d5-4d04-9b26-243b1b8745ca?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM [1 of 3 alike] | PASS |
| r00394 | admin | /collections | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00395 | admin | /products/[product] | mobile | zh-TW | main | 管理分類 | click | navigation or in-page change | scrolled | PASS |
| r00396 | admin | /collections | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/collections?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00397 | admin | /products | desktop | en | main | Copy | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00398 | admin | /products/[product] | mobile | zh-TW | main | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00399 | admin | /collections | mobile | zh-TW | main | 新增集合 (`collection-new`) | click | visible change | DOM changed (3 mutations); validation shown | PASS |
| r00400 | admin | /products | desktop | en | main | Select product: Sweep Wool Scarf | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00401 | admin | /products/[product] | mobile | zh-TW | layer of 物流 | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00402 | admin | /collections | mobile | zh-TW | main | Sweep home /sweep-home · 2 件商品 (`collection-item-dba959f1-a29a-4886-a4f1-69ba03adaa14`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00403 | admin | /products | desktop | en | main | Edit: Sweep Wool Scarf | click | navigation or in-page change | url /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/products/998ef94f-7b52-489a-91e3-4e6076f8d703?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM | PASS |
| r00404 | admin | /products/[product] | mobile | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00405 | admin | /collections | mobile | zh-TW | main | Sweep wear /sweep-wear · 2 件商品 (`collection-item-c989e2dd-7b39-422e-9b61-ed233c8235a8`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00406 | admin | /products | desktop | en | main | Edit price: Sweep Wool Scarf (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00407 | admin | /collections | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00408 | admin | /products/[product] | mobile | zh-TW | layer of 搜尋引擎最佳化 | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00409 | admin | /products | desktop | en | main | Edit inventory: Sweep Wool Scarf (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00410 | admin | /collections | desktop | en | main | Overview | click | navigation or in-page change | url /en/collections?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00411 | admin | /products/[product] | mobile | zh-TW | main | 儲存修改 (`product-save`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00412 | admin | /products | desktop | en | main | Edit | click | navigation or in-page change | url /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/products/998ef94f-7b52-489a-91e3-4e6076f8d703?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM [1 of 3 alike] | PASS |
| r00413 | admin | /collections | desktop | en | main | New collection (`collection-new`) | click | visible change | DOM changed (3 mutations); validation shown | PASS |
| r00414 | admin | /products/[product] | mobile | zh-TW | main | 下架轉草稿 (`product-unpublish`) | click | confirmation layer opens; cancel; nothing changed | dialog: confirm: 確定下架此商品？買家將無法再購買此商品。 (confirmation shown, then cancelled) | PASS |
| r00415 | admin | /products | desktop | en | main | Copy | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00416 | admin | /collections | desktop | en | main | Sweep home /sweep-home · 2 products (`collection-item-dba959f1-a29a-4886-a4f1-69ba03adaa14`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00417 | admin | /products/[product] | mobile | zh-TW | main | 上架 (`product-publish`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (1 mutations) (inline change only; no request fired) | PASS |
| r00418 | admin | /products | desktop | en | main | Select product: Sweep Cedar Candle | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00419 | admin | /collections | desktop | en | main | Sweep wear /sweep-wear · 2 products (`collection-item-c989e2dd-7b39-422e-9b61-ed233c8235a8`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00420 | admin | /products/[product] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00421 | admin | /inventory | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00422 | admin | /products | desktop | en | main | Edit: Sweep Cedar Candle | click | navigation or in-page change | url /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/products/1f957810-4949-430d-9b43-1e1c1b0f9a46?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM | PASS |
| r00423 | admin | /products/[product] | desktop | en | main | Overview | click | navigation or in-page change | url /en/products/1f957810-4949-430d-9b43-1e1c1b0f9a46?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed  | PASS |
| r00424 | admin | /inventory | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00425 | admin | /products | desktop | en | main | Edit price: Sweep Cedar Candle (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00426 | admin | /products/[product] | desktop | en | main | ← All products (`product-back`) | click | navigation or in-page change | url /en/products/1f957810-4949-430d-9b43-1e1c1b0f9a46?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM | PASS |
| r00427 | admin | /inventory | desktop | zh-TW | main | 新增商品 | click | navigation or in-page change | url /zh-TW/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products/new?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00428 | admin | /products | desktop | en | main | Edit inventory: Sweep Cedar Candle (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00429 | admin | /products/[product] | desktop | en | main | Product images | click | visible change | scrolled | PASS |
| r00430 | admin | /inventory | desktop | zh-TW | main | 查詢 | click | visible change | DOM changed (2 mutations) | PASS |
| r00431 | admin | /products | desktop | en | main | Edit | click | navigation or in-page change | url /en/products?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/products/1f957810-4949-430d-9b43-1e1c1b0f9a46?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM [1 of 3 alike] | PASS |
| r00432 | admin | /products/[product] | desktop | en | main | Basic information | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00433 | admin | /inventory | desktop | zh-TW | main | 倉庫 | selectOption | visible change | select has a single option | SKIP |
| r00434 | admin | /products | desktop | en | main | Copy | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00435 | admin | /inventory | desktop | zh-TW | main | 狀態 | selectOption(active) | visible change | url /zh-TW/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&status=active; DOM changed (2 mut | PASS |
| r00436 | admin | /products/[product] | desktop | en | main | Price & inventory | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00437 | admin | /products/import | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00438 | admin | /inventory | desktop | zh-TW | main | 重設 | click | visible change | DOM changed (2 mutations) | PASS |
| r00439 | admin | /products/[product] | desktop | en | main | Variants | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00440 | admin | /products/import | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/import?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00441 | admin | /products/[product] | desktop | en | main | Collections | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00442 | admin | /inventory | desktop | zh-TW | main | 重新整理 | click | visible change | DOM changed (2 mutations) | PASS |
| r00443 | admin | /products/import | desktop | zh-TW | main | 下載 CSV (`import-export`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00444 | admin | /inventory | desktop | zh-TW | main | 選取 T04-5e91716e6a9c-0 | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00445 | admin | /products/[product] | desktop | en | main | Shipping | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00446 | admin | /products/import | desktop | zh-TW | main | 回到總覽 | click | navigation or in-page change | url /zh-TW/products/import?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00447 | admin | /products/import | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00448 | admin | /inventory | desktop | zh-TW | main | Sweep Cedar Candle | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00449 | admin | /products/[product] | desktop | en | main | Search engine listing | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00450 | admin | /products/import | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/import?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00451 | admin | /inventory | desktop | zh-TW | main | 選取 | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00452 | admin | /products/[product] | desktop | en | main | Images ○ | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00453 | admin | /products/import | mobile | zh-TW | main | 下載 CSV (`import-export`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00454 | admin | /inventory | desktop | zh-TW | main | 選取 SWP-MUG | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00455 | admin | /products/[product] | desktop | en | main | Product name ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00456 | admin | /products/import | mobile | zh-TW | main | 回到總覽 | click | navigation or in-page change | scrolled | PASS |
| r00457 | admin | /inventory | desktop | zh-TW | main | Sweep Ceramic Mug | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00458 | admin | /products/[product] | desktop | en | main | Price ○ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00459 | admin | /products/import | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00460 | admin | /inventory | desktop | zh-TW | main | 選取 | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00461 | admin | /products/[product] | desktop | en | main | Inventory ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00462 | admin | /products/import | desktop | en | main | Overview | click | navigation or in-page change | url /en/products/import?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00463 | admin | /inventory | desktop | zh-TW | main | 選取 SWP-SCF | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00464 | admin | /products/import | desktop | en | main | Download CSV (`import-export`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00465 | admin | /products/[product] | desktop | en | main | Images ○ | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00466 | admin | /inventory | desktop | zh-TW | main | Sweep Wool Scarf | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00467 | admin | /products/import | desktop | en | main | Back to dashboard | click | navigation or in-page change | url /en/products/import?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00468 | admin | /products/[product] | desktop | en | main | Description ✓ | click | visible change | scrolled | PASS |
| r00469 | admin | /customers | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00470 | admin | /inventory | desktop | zh-TW | main | 選取 | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00471 | admin | /products/[product] | desktop | en | main | Collections ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00472 | admin | /customers | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00473 | admin | /inventory | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00474 | admin | /products/[product] | desktop | en | main | Live keyword ○ | click | visible change | scrolled | PASS |
| r00475 | admin | /inventory | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00476 | admin | /customers | desktop | zh-TW | main | 搜尋 (`customers-search-submit`) | click | visible change | DOM changed (12 mutations) | PASS |
| r00477 | admin | /products/[product] | desktop | en | main | Search engine listing ○ | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00478 | admin | /inventory | mobile | zh-TW | main | 新增商品 | click | navigation or in-page change | url /zh-TW/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/products/new?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (19 mutations) | PASS |
| r00479 | admin | /customers | desktop | zh-TW | main | 重新整理 (`customers-refresh`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00480 | admin | /products/[product] | desktop | en | main | VisibilityActive: shoppers can see and buy it once the store is published. (`product-status`) | selectOption(draft) | visible change | DOM changed (2 mutations); the control's own state changed (active -> draft) | PASS |
| r00481 | admin | /customers | desktop | zh-TW | main | 開啟客戶: 王小明 (`customer-open-dff495c7-e8d0-4f9c-aacf-f4fb6de9d09f`) | click | navigation or in-page change | url /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/customers/dff495c7-e8d0-4f9c-aacf-f4fb6de9d09f?store=26dc24c3-776c-4adf-943d-128f1b2b0 [1 of 3 alike] | PASS |
| r00482 | admin | /inventory | mobile | zh-TW | main | 查詢 | click | visible change | DOM changed (2 mutations) | PASS |
| r00483 | admin | /products/[product] | desktop | en | main | Remove Option name 1 | click | confirmation layer opens; cancel; nothing changed | DOM changed (10 mutations); the control's own state changed ( -> true); scrolled (inline change only; no request fired) | PASS |
| r00484 | admin | /inventory | mobile | zh-TW | main | 倉庫 | selectOption | visible change | select has a single option | SKIP |
| r00485 | admin | /customers | desktop | zh-TW | main | 開啟客戶: 王小明 (`customer-open-da15b8f9-01bd-4ca1-9fb7-ca6be50b598e`) | click | navigation or in-page change | url /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/customers/da15b8f9-01bd-4ca1-9fb7-ca6be50b598e?store=26dc24c3-776c-4adf-943d-128f1b2b0 [1 of 3 alike] | PASS |
| r00486 | admin | /inventory | mobile | zh-TW | main | 狀態 | selectOption(active) | visible change | url /zh-TW/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&status=active; DOM changed (2 mut | PASS |
| r00487 | admin | /products/[product] | desktop | en | main | Add option (color, size…) (`axis-add`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00488 | admin | /customers | desktop | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-17699b85-3bb0-4489-a6f1-1aa15f30fb14`) | click | navigation or in-page change | url /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/customers/17699b85-3bb0-4489-a6f1-1aa15f30fb14?store=26dc24c3-776c-4adf-943d-128f1b2b0 [1 of 3 alike] | PASS |
| r00489 | admin | /inventory | mobile | zh-TW | main | 重設 | click | visible change | DOM changed (2 mutations) | PASS |
| r00490 | admin | /products/[product] | desktop | en | main | Price · Bulk fill (`bulk-price`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00491 | admin | /customers | desktop | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-e4ed9f27-c330-4a7a-9c3f-3d9313620819`) | click | navigation or in-page change | url /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/customers/e4ed9f27-c330-4a7a-9c3f-3d9313620819?store=26dc24c3-776c-4adf-943d-128f1b2b0 [1 of 3 alike] | PASS |
| r00492 | admin | /inventory | mobile | zh-TW | main | 重新整理 | click | visible change | DOM changed (2 mutations) | PASS |
| r00493 | admin | /products/[product] | desktop | en | main | Compare-at price · Bulk fill (`bulk-compare`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00494 | admin | /customers | desktop | zh-TW | main | 開啟客戶: 王小明 (`customer-open-053b1291-f2f8-4e25-bef2-d2ffb0fc0c70`) | click | navigation or in-page change | url /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/customers/053b1291-f2f8-4e25-bef2-d2ffb0fc0c70?store=26dc24c3-776c-4adf-943d-128f1b2b0 [1 of 3 alike] | PASS |
| r00495 | admin | /products/[product] | desktop | en | main | Quantity · Bulk fill (`bulk-quantity`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00496 | admin | /inventory | mobile | zh-TW | main | 選取 T04-5e91716e6a9c-0 | click | visible change | DOM changed (12 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00497 | admin | /customers | desktop | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-d2a64f67-ead3-438c-8138-7182cebba968`) | click | navigation or in-page change | url /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/customers/d2a64f67-ead3-438c-8138-7182cebba968?store=26dc24c3-776c-4adf-943d-128f1b2b0 [1 of 3 alike] | PASS |
| r00498 | admin | /products/[product] | desktop | en | main | SKU code · Bulk fill (`bulk-code`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00499 | admin | /inventory | mobile | zh-TW | main | Sweep Cedar Candle | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00500 | admin | /customers | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00501 | admin | /inventory | mobile | zh-TW | main | 選取 SWP-MUG | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00502 | admin | /customers | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00503 | admin | /products/[product] | desktop | en | main | Live keyword · Bulk fill (`bulk-keyword`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00504 | admin | /inventory | mobile | zh-TW | main | Sweep Ceramic Mug | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00505 | admin | /customers | mobile | zh-TW | main | 搜尋 (`customers-search-submit`) | click | visible change | DOM changed (12 mutations) | PASS |
| r00506 | admin | /products/[product] | desktop | en | main | Do not track ∞ | click (toggle) | visible change | DOM changed (5 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00507 | admin | /inventory | mobile | zh-TW | main | 選取 SWP-SCF | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00508 | admin | /customers | mobile | zh-TW | main | 重新整理 (`customers-refresh`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00509 | admin | /products/[product] | desktop | en | main | Enabled 1 (`matrix-active-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00510 | admin | /inventory | mobile | zh-TW | main | Sweep Wool Scarf | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00511 | admin | /customers | mobile | zh-TW | main | 開啟客戶: 王小明 (`customer-open-dff495c7-e8d0-4f9c-aacf-f4fb6de9d09f`) | click | navigation or in-page change | url /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/customers/dff495c7-e8d0-4f9c-aacf-f4fb6de9d09f?store=26dc24c3-776c-4adf-943d-128f1b2b0 [1 of 3 alike] | PASS |
| r00512 | admin | /products/[product] | desktop | en | main | Sweep home | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00513 | admin | /inventory | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00514 | admin | /customers | mobile | zh-TW | main | 開啟客戶: 王小明 (`customer-open-da15b8f9-01bd-4ca1-9fb7-ca6be50b598e`) | click | navigation or in-page change | url /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/customers/da15b8f9-01bd-4ca1-9fb7-ca6be50b598e?store=26dc24c3-776c-4adf-943d-128f1b2b0 [1 of 3 alike] | PASS |
| r00515 | admin | /products/[product] | desktop | en | main | Sweep wear | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00516 | admin | /inventory | desktop | en | main | Overview | click | navigation or in-page change | url /en/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00517 | admin | /products/[product] | desktop | en | main | Manage collections | click | navigation or in-page change | scrolled | PASS |
| r00518 | admin | /customers | mobile | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-17699b85-3bb0-4489-a6f1-1aa15f30fb14`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00519 | admin | /inventory | desktop | en | main | Add product | click | navigation or in-page change | url /en/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/products/new?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (19 mutations) | PASS |
| r00520 | admin | /customers | mobile | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-e4ed9f27-c330-4a7a-9c3f-3d9313620819`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00521 | admin | /products/[product] | desktop | en | main | Shipping | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00522 | admin | /inventory | desktop | en | main | Search | click | visible change | DOM changed (2 mutations) | PASS |
| r00523 | admin | /customers | mobile | zh-TW | main | 開啟客戶: 王小明 (`customer-open-053b1291-f2f8-4e25-bef2-d2ffb0fc0c70`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00524 | admin | /inventory | desktop | en | main | Warehouse | selectOption | visible change | select has a single option | SKIP |
| r00525 | admin | /products/[product] | desktop | en | layer of Shipping | Shipping | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00526 | admin | /customers | mobile | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-d2a64f67-ead3-438c-8138-7182cebba968`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00527 | admin | /inventory | desktop | en | main | Status | selectOption(active) | visible change | url /en/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/inventory?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&status=active; DOM changed (2 mutations | PASS |
| r00528 | admin | /products/[product] | desktop | en | main | Search engine listing | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00529 | admin | /customers | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00530 | admin | /inventory | desktop | en | main | Reset | click | visible change | DOM changed (2 mutations) | PASS |
| r00531 | admin | /products/[product] | desktop | en | layer of Search engine listing | Search engine listing | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00532 | admin | /customers | desktop | en | main | Overview | click | navigation or in-page change | url /en/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00533 | admin | /inventory | desktop | en | main | Refresh | click | visible change | DOM changed (2 mutations) | PASS |
| r00534 | admin | /products/[product] | desktop | en | main | Save changes (`product-save`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00535 | admin | /customers | desktop | en | main | Search (`customers-search-submit`) | click | visible change | DOM changed (12 mutations) | PASS |
| r00536 | admin | /inventory | desktop | en | main | Select T04-5e91716e6a9c-0 | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00537 | admin | /customers | desktop | en | main | Refresh (`customers-refresh`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00538 | admin | /products/[product] | desktop | en | main | Unpublish to draft (`product-unpublish`) | click | confirmation layer opens; cancel; nothing changed | dialog: confirm: Unpublish this product? Shoppers will no longer be able to buy it. (confirmation shown, then cancelled) | PASS |
| r00539 | admin | /inventory | desktop | en | main | Sweep Cedar Candle | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00540 | admin | /products/[product] | desktop | en | main | Publish (`product-publish`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (1 mutations) (inline change only; no request fired) | PASS |
| r00541 | admin | /customers | desktop | en | main | Open customer: 王小明 (`customer-open-dff495c7-e8d0-4f9c-aacf-f4fb6de9d09f`) | click | navigation or in-page change | url /en/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/customers/dff495c7-e8d0-4f9c-aacf-f4fb6de9d09f?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; D [1 of 3 alike] | PASS |
| r00542 | admin | /inventory | desktop | en | main | Select | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00543 | admin | /customers/[customer] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00544 | admin | /customers | desktop | en | main | Open customer: 王小明 (`customer-open-da15b8f9-01bd-4ca1-9fb7-ca6be50b598e`) | click | navigation or in-page change | url /en/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/customers/da15b8f9-01bd-4ca1-9fb7-ca6be50b598e?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; D [1 of 3 alike] | PASS |
| r00545 | admin | /customers/[customer] | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers/e4ed9f27-c330-4a7a-9c3f-3d9313620819?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM c | PASS |
| r00546 | admin | /inventory | desktop | en | main | Select SWP-MUG | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00547 | admin | /customers | desktop | en | main | Open customer: Synthetic Buyer (`customer-open-17699b85-3bb0-4489-a6f1-1aa15f30fb14`) | click | navigation or in-page change | url /en/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/customers/17699b85-3bb0-4489-a6f1-1aa15f30fb14?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; D [1 of 3 alike] | PASS |
| r00548 | admin | /inventory | desktop | en | main | Sweep Ceramic Mug | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00549 | admin | /customers/[customer] | desktop | zh-TW | main | 全部客戶 | click | navigation or in-page change | url /zh-TW/customers/e4ed9f27-c330-4a7a-9c3f-3d9313620819?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0 | PASS |
| r00550 | admin | /customers | desktop | en | main | Open customer: Synthetic Buyer (`customer-open-e4ed9f27-c330-4a7a-9c3f-3d9313620819`) | click | navigation or in-page change | url /en/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/customers/e4ed9f27-c330-4a7a-9c3f-3d9313620819?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; D [1 of 3 alike] | PASS |
| r00551 | admin | /inventory | desktop | en | main | Select | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00552 | admin | /customers/[customer] | desktop | zh-TW | main | 在訂單中開啟: eb15d16b-6d7e-447b-959e-d290f2ea2d99 | click | navigation or in-page change | url /zh-TW/customers/e4ed9f27-c330-4a7a-9c3f-3d9313620819?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec | PASS |
| r00553 | admin | /customers | desktop | en | main | Open customer: 王小明 (`customer-open-053b1291-f2f8-4e25-bef2-d2ffb0fc0c70`) | click | navigation or in-page change | url /en/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/customers/053b1291-f2f8-4e25-bef2-d2ffb0fc0c70?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; D [1 of 3 alike] | PASS |
| r00554 | admin | /inventory | desktop | en | main | Select SWP-SCF | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00555 | admin | /customers/[customer] | desktop | zh-TW | main | 下載資料 (`customer-download`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00556 | admin | /customers | desktop | en | main | Open customer: Synthetic Buyer (`customer-open-d2a64f67-ead3-438c-8138-7182cebba968`) | click | navigation or in-page change | url /en/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/customers/d2a64f67-ead3-438c-8138-7182cebba968?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; D [1 of 3 alike] | PASS |
| r00557 | admin | /inventory | desktop | en | main | Sweep Wool Scarf | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00558 | admin | /customers/[customer] | desktop | zh-TW | main | 抹除客戶 (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00559 | admin | /promotions | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00560 | admin | /customers/[customer] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00561 | admin | /inventory | desktop | en | main | Select | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00562 | admin | /promotions | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/promotions?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00563 | admin | /customers/[customer] | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers/e4ed9f27-c330-4a7a-9c3f-3d9313620819?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM c | PASS |
| r00564 | admin | /ads | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00565 | admin | /promotions | desktop | zh-TW | main | 類型 (`promotion-kind`) | selectOption(fixed) | visible change | DOM changed (1 mutations); the control's own state changed (percent -> fixed) | PASS |
| r00566 | admin | /ads | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00567 | admin | /customers/[customer] | mobile | zh-TW | main | 全部客戶 | click | navigation or in-page change | url /zh-TW/customers/e4ed9f27-c330-4a7a-9c3f-3d9313620819?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0 | PASS |
| r00568 | admin | /promotions | desktop | zh-TW | main | 建立優惠碼 (`promotion-submit`) | click | visible change | validation shown | PASS |
| r00569 | admin | /ads | desktop | zh-TW | main | 重新整理 (`ads-refresh`) | click | visible change | DOM changed (9 mutations) | PASS |
| r00570 | admin | /customers/[customer] | mobile | zh-TW | main | 在訂單中開啟: eb15d16b-6d7e-447b-959e-d290f2ea2d99 | click | navigation or in-page change | url /zh-TW/customers/e4ed9f27-c330-4a7a-9c3f-3d9313620819?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec | PASS |
| r00571 | admin | /promotions | desktop | zh-TW | main | 暫停 (`promotion-toggle`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00572 | admin | /ads | desktop | zh-TW | main | 連結 Meta 廣告帳號 (`ads-connect`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00573 | admin | /customers/[customer] | mobile | zh-TW | main | 下載資料 (`customer-download`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00574 | admin | /promotions | desktop | zh-TW | main | 編輯 (`promotion-edit`) | click | visible change | DOM changed (6 mutations) | PASS |
| r00575 | admin | /ads | desktop | zh-TW | main | 新增草稿 (`ads-new-draft`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00576 | admin | /customers/[customer] | mobile | zh-TW | main | 抹除客戶 (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00577 | admin | /promotions | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00578 | admin | /promotions | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/promotions?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00579 | admin | /ads | desktop | zh-TW | main | 顯示成效 (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00580 | admin | /customers/[customer] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00581 | admin | /customers/[customer] | desktop | en | main | Overview | click | navigation or in-page change | url /en/customers/e4ed9f27-c330-4a7a-9c3f-3d9313620819?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed | PASS |
| r00582 | admin | /promotions | mobile | zh-TW | main | 類型 (`promotion-kind`) | selectOption(fixed) | visible change | DOM changed (1 mutations); the control's own state changed (percent -> fixed) | PASS |
| r00583 | admin | /ads | desktop | zh-TW | main | 傳送購買事件 (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00584 | admin | /ads | desktop | zh-TW | main | 資料集請先連結廣告帳號與資料集才能開啟。 (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00585 | admin | /customers/[customer] | desktop | en | main | All customers | click | navigation or in-page change | url /en/customers/e4ed9f27-c330-4a7a-9c3f-3d9313620819?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/customers?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; D | PASS |
| r00586 | admin | /promotions | mobile | zh-TW | main | 建立優惠碼 (`promotion-submit`) | click | visible change | validation shown; scrolled | PASS |
| r00587 | admin | /promotions | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00588 | admin | /ads | desktop | zh-TW | main | 儲存 (`ads-capi-save`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00589 | admin | /customers/[customer] | desktop | en | main | Open in orders: eb15d16b-6d7e-447b-959e-d290f2ea2d99 | click | navigation or in-page change | url /en/customers/e4ed9f27-c330-4a7a-9c3f-3d9313620819?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/orders?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&order | PASS |
| r00590 | admin | /promotions | desktop | en | main | Overview | click | navigation or in-page change | url /en/promotions?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00591 | admin | /ads | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00592 | admin | /customers/[customer] | desktop | en | main | Download data (`customer-download`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00593 | admin | /promotions | desktop | en | main | Type (`promotion-kind`) | selectOption(fixed) | visible change | DOM changed (1 mutations); the control's own state changed (percent -> fixed) | PASS |
| r00594 | admin | /ads | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00595 | admin | /customers/[customer] | desktop | en | main | Erase customer (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00596 | admin | /promotions | desktop | en | main | Create code (`promotion-submit`) | click | visible change | validation shown | PASS |
| r00597 | admin | /ads | mobile | zh-TW | main | 重新整理 (`ads-refresh`) | click | visible change | DOM changed (9 mutations) | PASS |
| r00598 | admin | /design | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00599 | admin | /promotions | desktop | en | main | Pause (`promotion-toggle`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00600 | admin | /ads | mobile | zh-TW | main | 連結 Meta 廣告帳號 (`ads-connect`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00601 | admin | /design | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/design?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00602 | admin | /promotions | desktop | en | main | Edit (`promotion-edit`) | click | visible change | DOM changed (6 mutations) | PASS |
| r00603 | admin | /ads | mobile | zh-TW | main | 新增草稿 (`ads-new-draft`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00604 | admin | /design | desktop | zh-TW | main | 預覽 (`design-preview`) | click | visible change | popup: about:blank; DOM changed (2 mutations) | PASS |
| r00605 | admin | /finance | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00606 | admin | /ads | mobile | zh-TW | main | 顯示成效 (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00607 | admin | /finance | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/finance?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00608 | admin | /ads | mobile | zh-TW | main | 傳送購買事件 (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00609 | admin | /finance | desktop | zh-TW | main | 查看 (`finance-show`) | click | visible change | url /zh-TW/finance?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/finance?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&from=2026-09-05&to=2026-10-04; DOM ch | PASS |
| r00610 | admin | /ads | mobile | zh-TW | main | 資料集請先連結廣告帳號與資料集才能開啟。 (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00611 | admin | /finance | desktop | zh-TW | main | 下載 CSV (`finance-csv`) | click | navigation or in-page change | download: finance-2026-09-05-2026-10-04.csv | PASS |
| r00612 | admin | /design | desktop | zh-TW | main | 店鋪資料 (`design-tab-profile`) | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00613 | admin | /ads | mobile | zh-TW | main | 儲存 (`ads-capi-save`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00614 | admin | /finance | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00615 | admin | /design | desktop | zh-TW | main | 導覽 (`design-tab-nav`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00616 | admin | /ads | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00617 | admin | /finance | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/finance?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00618 | admin | /ads | desktop | en | main | Overview | click | navigation or in-page change | url /en/ads?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00619 | admin | /design | desktop | zh-TW | main | 首頁 (`design-tab-home`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00620 | admin | /finance | mobile | zh-TW | main | 查看 (`finance-show`) | click | visible change | url /zh-TW/finance?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW/finance?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&from=2026-09-05&to=2026-10-04; DOM ch | PASS |
| r00621 | admin | /ads | desktop | en | main | Refresh (`ads-refresh`) | click | visible change | DOM changed (9 mutations) | PASS |
| r00622 | admin | /design | desktop | zh-TW | main | 資訊頁 (`design-tab-pages`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00623 | admin | /finance | mobile | zh-TW | main | 下載 CSV (`finance-csv`) | click | navigation or in-page change | download: finance-2026-09-05-2026-10-04.csv | PASS |
| r00624 | admin | /ads | desktop | en | main | Connect a Meta ad account (`ads-connect`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00625 | admin | /design | desktop | zh-TW | main | 版本 (`design-tab-versions`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00626 | admin | /finance | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00627 | admin | /ads | desktop | en | main | New draft (`ads-new-draft`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00628 | admin | /design | desktop | zh-TW | main | 選擇圖片 (`design-logo-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00629 | admin | /finance | desktop | en | main | Overview | click | navigation or in-page change | url /en/finance?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00630 | admin | /ads | desktop | en | main | Show results (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00631 | admin | /design | desktop | zh-TW | main | 選擇圖片 (`design-favicon-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00632 | admin | /finance | desktop | en | main | Show (`finance-show`) | click | visible change | url /en/finance?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en/finance?store=26dc24c3-776c-4adf-943d-128f1b2b0eec&from=2026-09-05&to=2026-10-04; DOM changed  | PASS |
| r00633 | admin | /design | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00634 | admin | /ads | desktop | en | main | Send purchase events (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00635 | admin | /finance | desktop | en | main | Download CSV (`finance-csv`) | click | navigation or in-page change | download: finance-2026-09-05-2026-10-04.csv | PASS |
| r00636 | admin | /design | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/design?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00637 | admin | /ads | desktop | en | main | DatasetConnect a dataset with an ad account to turn this on. (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00638 | admin | /settings | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00639 | admin | /design | mobile | zh-TW | main | 預覽 (`design-preview`) | click | visible change | popup: about:blank; DOM changed (2 mutations) | PASS |
| r00640 | admin | /ads | desktop | en | main | Save (`ads-capi-save`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00641 | admin | /settings | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/settings?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00642 | admin | /team | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00643 | admin | /team | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/team?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00644 | admin | /team | desktop | zh-TW | main | 角色 (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00645 | admin | /design | mobile | zh-TW | main | 店鋪資料 (`design-tab-profile`) | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00646 | admin | /team | desktop | zh-TW | main | 寄送邀請 (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00647 | admin | /settings | desktop | zh-TW | main | 1 選擇平台 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00648 | admin | /team | desktop | zh-TW | main | 變更角色 (`member-role-7a7a123d-8a01-461a-b2e7-caf5b3309885`) | selectOption(admin) | visible change | DOM changed (3 mutations) | PASS |
| r00649 | admin | /design | mobile | zh-TW | main | 導覽 (`design-tab-nav`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00650 | admin | /team | desktop | zh-TW | main | 移除 (`member-remove-7a7a123d-8a01-461a-b2e7-caf5b3309885`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (5 mutations) (inline change only; no request fired) | PASS |
| r00651 | admin | /design | mobile | zh-TW | main | 首頁 (`design-tab-home`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00652 | admin | /team | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00653 | admin | /design | mobile | zh-TW | main | 資訊頁 (`design-tab-pages`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00654 | admin | /team | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/team?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00655 | admin | /settings | desktop | zh-TW | main | PAYUNi 收款 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00656 | admin | /design | mobile | zh-TW | main | 版本 (`design-tab-versions`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00657 | admin | /team | mobile | zh-TW | main | 角色 (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00658 | admin | /settings | desktop | zh-TW | main | 商家自行安排配送 | click | visible change | DOM changed (8 mutations); the control's own state changed (false -> true) | PASS |
| r00659 | admin | /design | mobile | zh-TW | main | 選擇圖片 (`design-logo-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00660 | admin | /team | mobile | zh-TW | main | 寄送邀請 (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00661 | admin | /settings | desktop | zh-TW | main | 繼續 | click | visible change | DOM changed (12 mutations) | PASS |
| r00662 | admin | /team | mobile | zh-TW | main | 變更角色 (`member-role-7a7a123d-8a01-461a-b2e7-caf5b3309885`) | selectOption(admin) | visible change | DOM changed (3 mutations) | PASS |
| r00663 | admin | /design | mobile | zh-TW | main | 選擇圖片 (`design-favicon-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00664 | admin | /settings | desktop | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00665 | admin | /team | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00666 | admin | /design | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00667 | admin | /team | desktop | en | main | Overview | click | navigation or in-page change | url /en/team?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00668 | admin | /design | desktop | en | main | Overview | click | navigation or in-page change | url /en/design?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00669 | admin | /settings | desktop | zh-TW | main | 取消發佈 (`storefront-toggle`) | click | confirmation layer opens; cancel; nothing changed | layer opened (取消發佈); DOM changed (4 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00670 | admin | /team | desktop | en | main | Role (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00671 | admin | /design | desktop | en | main | Preview (`design-preview`) | click | visible change | popup: about:blank; DOM changed (2 mutations) | PASS |
| r00672 | admin | /settings | desktop | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00673 | admin | /team | desktop | en | main | Send invitation (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00674 | admin | /settings | desktop | zh-TW | main | 暫停 (`storefront-domain-suspend`) | click | confirmation layer opens; cancel; nothing changed | layer opened (暫停); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00675 | admin | /team | desktop | en | main | Change role (`member-role-7a7a123d-8a01-461a-b2e7-caf5b3309885`) | selectOption(admin) | visible change | DOM changed (3 mutations) | PASS |
| r00676 | admin | /settings | desktop | zh-TW | main | 解除綁定 (`storefront-domain-detach`) | click | confirmation layer opens; cancel; nothing changed | layer opened (解除綁定); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00677 | admin | /team | desktop | en | main | Remove (`member-remove-7a7a123d-8a01-461a-b2e7-caf5b3309885`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (5 mutations) (inline change only; no request fired) | PASS |
| r00678 | admin | /design | desktop | en | main | Store profile (`design-tab-profile`) | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00679 | admin | /settings | desktop | zh-TW | main | 請求驗證 (`storefront-domain-submit`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00680 | admin | /billing | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00681 | admin | /billing | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/billing?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00682 | admin | /design | desktop | en | main | Navigation (`design-tab-nav`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00683 | admin | /settings | desktop | zh-TW | main | 新增專頁 (`metaconnect-add`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00684 | admin | /billing | desktop | zh-TW | main | 訂閱 (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00685 | admin | /design | desktop | en | main | Home page (`design-tab-home`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00686 | admin | /settings | desktop | zh-TW | main | 選擇專頁重新授權 (`metaconnect-reconnect`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00687 | admin | /billing | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00688 | admin | /design | desktop | en | main | Pages (`design-tab-pages`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00689 | admin | /settings | desktop | zh-TW | main | 中斷連接 (`metaconnect-disconnect`) | click | confirmation layer opens; cancel; nothing changed | layer opened (中斷連接: Sweep Page); DOM changed (5 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00690 | admin | /billing | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/billing?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00691 | admin | /settings | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00692 | admin | /design | desktop | en | main | Versions (`design-tab-versions`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00693 | admin | /billing | mobile | zh-TW | main | 訂閱 (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00694 | admin | /settings | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/settings?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /zh-TW?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00695 | admin | /design | desktop | en | main | Choose image (`design-logo-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00696 | admin | /billing | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00697 | admin | /billing | desktop | en | main | Overview | click | navigation or in-page change | url /en/billing?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (11 mutations) | PASS |
| r00698 | admin | /design | desktop | en | main | Choose image (`design-favicon-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00699 | admin | /invite/[token] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00700 | admin | /billing | desktop | en | main | Subscribe (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00701 | admin | /invite/[token] | desktop | zh-TW | main | 登入 (`invite-signin`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (47 mu | PASS |
| r00702 | admin | /reset | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00703 | admin | /settings | mobile | zh-TW | main | 1 選擇平台 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00704 | admin | /reset | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00705 | admin | /invite/[token] | desktop | zh-TW | main | 註冊帳號 (`invite-signup`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW/signup#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed | PASS |
| r00706 | admin | /invite/[token] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00707 | admin | /reset | desktop | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00708 | admin | /invite/[token] | mobile | zh-TW | main | 登入 (`invite-signin`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (47 mu | PASS |
| r00709 | admin | /reset | desktop | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/reset -> /zh-TW; DOM changed (16 mutations); validation shown | PASS |
| r00710 | admin | /invite/[token] | mobile | zh-TW | main | 註冊帳號 (`invite-signup`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW/signup#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed | PASS |
| r00711 | admin | /reset | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00712 | admin | /settings | mobile | zh-TW | main | PAYUNi 收款 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00713 | admin | /invite/[token] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00714 | admin | /reset | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00715 | admin | /invite/[token] | desktop | en | main | Sign in (`invite-signin`) | click | navigation or in-page change | url /en/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /en#next=%2Fen%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (47 mutations); | PASS |
| r00716 | admin | /settings | mobile | zh-TW | main | 商家自行安排配送 | click | visible change | DOM changed (8 mutations); the control's own state changed (false -> true) | PASS |
| r00717 | admin | /reset | mobile | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00718 | admin | /invite/[token] | desktop | en | main | Create an account (`invite-signup`) | click | navigation or in-page change | url /en/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /en/signup#next=%2Fen%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (36 muta | PASS |
| r00719 | admin | /settings | mobile | zh-TW | main | 繼續 | click | visible change | DOM changed (12 mutations) | PASS |
| r00720 | admin | /reset | mobile | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/reset -> /zh-TW; DOM changed (16 mutations); validation shown | PASS |
| r00721 | admin | /signup | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00722 | admin | /signup | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00723 | admin | /settings | mobile | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00724 | admin | /reset | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00725 | admin | /reset | desktop | en | main | Language | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00726 | admin | /signup | desktop | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00727 | admin | /settings | mobile | zh-TW | main | 取消發佈 (`storefront-toggle`) | click | confirmation layer opens; cancel; nothing changed | layer opened (取消發佈); DOM changed (4 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00728 | admin | /reset | desktop | en | main | Send code | click | visible change | validation shown | PASS |
| r00729 | admin | /signup | desktop | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/signup -> /zh-TW; DOM changed (9 mutations) | PASS |
| r00730 | admin | /settings | mobile | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00731 | admin | /reset | desktop | en | main | Back to sign in | click | navigation or in-page change | url /en/reset -> /en; DOM changed (16 mutations); validation shown | PASS |
| r00732 | admin | /signup | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00733 | admin | /settings | mobile | zh-TW | main | 暫停 (`storefront-domain-suspend`) | click | confirmation layer opens; cancel; nothing changed | layer opened (暫停); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00734 | admin | /signup | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00735 | admin | /settings | mobile | zh-TW | main | 解除綁定 (`storefront-domain-detach`) | click | confirmation layer opens; cancel; nothing changed | layer opened (解除綁定); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00736 | admin | /signup | mobile | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00737 | admin | /settings | mobile | zh-TW | main | 請求驗證 (`storefront-domain-submit`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00738 | admin | /signup | mobile | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/signup -> /zh-TW; DOM changed (9 mutations) | PASS |
| r00739 | admin | /signup | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00740 | admin | /settings | mobile | zh-TW | main | 新增專頁 (`metaconnect-add`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00741 | admin | /signup | desktop | en | main | Language | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00742 | admin | /settings | mobile | zh-TW | main | 選擇專頁重新授權 (`metaconnect-reconnect`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00743 | admin | /signup | desktop | en | main | Send code | click | visible change | validation shown | PASS |
| r00744 | admin | /settings | mobile | zh-TW | main | 中斷連接 (`metaconnect-disconnect`) | click | confirmation layer opens; cancel; nothing changed | layer opened (中斷連接: Sweep Page); DOM changed (5 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00745 | admin | /signup | desktop | en | main | Back to sign in | click | navigation or in-page change | url /en/signup -> /en; DOM changed (9 mutations) | PASS |
| r00746 | admin | /settings | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00747 | admin | /settings | desktop | en | main | Overview | click | navigation or in-page change | url /en/settings?store=26dc24c3-776c-4adf-943d-128f1b2b0eec -> /en?store=26dc24c3-776c-4adf-943d-128f1b2b0eec; DOM changed (15 mutations) | PASS |
| r00748 | admin | /settings | desktop | en | main | 1 Choose platform | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00749 | admin | /settings | desktop | en | main | PAYUNi payment | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00750 | admin | /settings | desktop | en | main | Merchant-arranged delivery | click | visible change | DOM changed (8 mutations); the control's own state changed (false -> true) | PASS |
| r00751 | admin | /settings | desktop | en | main | Continue | click | visible change | DOM changed (12 mutations) | PASS |
| r00752 | admin | /settings | desktop | en | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00753 | admin | /settings | desktop | en | main | Unpublish storefront (`storefront-toggle`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Unpublish storefront); DOM changed (4 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00754 | admin | /settings | desktop | en | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00755 | admin | /settings | desktop | en | main | Suspend (`storefront-domain-suspend`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Suspend); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00756 | admin | /settings | desktop | en | main | Detach (`storefront-domain-detach`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Detach); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00757 | admin | /settings | desktop | en | main | Request verification (`storefront-domain-submit`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00758 | admin | /settings | desktop | en | main | Add Page (`metaconnect-add`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00759 | admin | /settings | desktop | en | main | Choose a Page to reauthorize (`metaconnect-reconnect`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00760 | admin | /settings | desktop | en | main | Disconnect (`metaconnect-disconnect`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Disconnect: Sweep Page); DOM changed (5 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00761 | storefront | /cart | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00762 | storefront | /cart | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00763 | storefront | /cart | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00764 | storefront | /cart | desktop | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00765 | storefront | /cart | mobile | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00766 | storefront | /cart | desktop | en | main | Sweep Wool Scarf | click | navigation or in-page change | url /en/cart -> /en/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00767 | storefront | /cart | desktop | zh-TW | main | 增加數量 | click | visible change | DOM changed (3 mutations) | PASS |
| r00768 | storefront | /cart | mobile | zh-TW | main | 增加數量 | click | visible change | DOM changed (3 mutations) | PASS |
| r00769 | storefront | /cart | desktop | en | main | Increase quantity | click | visible change | DOM changed (3 mutations) | PASS |
| r00770 | storefront | /cart | desktop | zh-TW | main | 將 Sweep Wool Scarf 移出購物車 | click | visible change | DOM changed (3 mutations) | PASS |
| r00771 | storefront | /cart | mobile | zh-TW | main | 將 Sweep Wool Scarf 移出購物車 | click | visible change | DOM changed (3 mutations) | PASS |
| r00772 | storefront | /cart | desktop | en | main | Remove Sweep Wool Scarf from the cart | click | confirmation layer opens; cancel; nothing changed | DOM changed (3 mutations) (inline change only; no request fired) | PASS |
| r00773 | storefront | /cart | desktop | zh-TW | main | 前往結帳 (`cart-checkout`) | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/checkout; DOM changed (5 mutations) | PASS |
| r00774 | storefront | /cart | mobile | zh-TW | main | 前往結帳 (`cart-checkout`) | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/checkout; DOM changed (5 mutations) | PASS |
| r00775 | storefront | /cart | desktop | en | main | Checkout (`cart-checkout`) | click | navigation or in-page change | url /en/cart -> /en/checkout; DOM changed (5 mutations) | PASS |
| r00776 | storefront | /cart | desktop | zh-TW | main | 繼續選購 | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00777 | storefront | /cart | mobile | zh-TW | main | 繼續選購 | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00778 | storefront | /cart | desktop | en | main | Continue shopping | click | navigation or in-page change | url /en/cart -> /en/products; DOM changed (4 mutations) | PASS |
| r00779 | storefront | /checkout | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00780 | storefront | /checkout | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00781 | storefront | /checkout | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00782 | storefront | /checkout | desktop | zh-TW | main | 我的訂單 (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00783 | storefront | /checkout | mobile | zh-TW | main | 我的訂單 (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00784 | storefront | /checkout | desktop | en | main | Your orders (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00785 | storefront | /checkout | desktop | en | main | Sweep Wool Scarf | click | navigation or in-page change | url /en/checkout -> /en/products/sweep-wool-scarf; DOM changed (6 mutations) | PASS |
| r00786 | storefront | /checkout | desktop | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/products/sweep-wool-scarf; DOM changed (6 mutations) | PASS |
| r00787 | storefront | /checkout | mobile | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/products/sweep-wool-scarf; DOM changed (6 mutations) | PASS |
| r00788 | storefront | /checkout | desktop | zh-TW | main | 返回購物車 | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/cart; DOM changed (9 mutations) | PASS |
| r00789 | storefront | /checkout | mobile | zh-TW | main | 返回購物車 | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/cart; DOM changed (9 mutations) | PASS |
| r00790 | storefront | /checkout | desktop | zh-TW | main | 選擇配送 | click | visible change | DOM changed (3 mutations) | PASS |
| r00791 | storefront | /checkout | mobile | zh-TW | main | 選擇配送 | click | visible change | DOM changed (3 mutations) | PASS |
| r00792 | storefront | /checkout | desktop | en | main | Back to cart | click | navigation or in-page change | url /en/checkout -> /en/cart; DOM changed (9 mutations) | PASS |
| r00793 | storefront | /checkout | desktop | en | main | Choose delivery | click | visible change | DOM changed (3 mutations) | PASS |
| r00794 | storefront | /orders/[orderID] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00795 | storefront | /orders/[orderID] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00796 | storefront | /orders/[orderID] | desktop | zh-TW | main | 重新整理訂單 (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone); scrolled | PASS |
| r00797 | storefront | /orders/[orderID] | mobile | zh-TW | main | 重新整理訂單 (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone); scrolled | PASS |
| r00798 | storefront | /orders/[orderID] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00799 | storefront | /orders/[orderID] | desktop | en | main | Refresh order (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone); scrolled | PASS |
| r00800 | storefront | / | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00801 | storefront | / | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00802 | storefront | / | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00803 | storefront | / | desktop | zh-TW | chrome | 跳到主要內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW -> /zh-TW#main; scrolled | PASS |
| r00804 | storefront | / | mobile | zh-TW | chrome | 跳到主要內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW -> /zh-TW#main; scrolled | PASS |
| r00805 | storefront | / | desktop | zh-TW | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00806 | storefront | / | mobile | zh-TW | chrome | 選單 (`menu-open`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00807 | storefront | / | desktop | zh-TW | chrome | All products | click | navigation or in-page change | url /zh-TW -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00808 | storefront | / | mobile | zh-TW | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00809 | storefront | / | desktop | zh-TW | chrome | Sweep home | click | navigation or in-page change | url /zh-TW -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00810 | storefront | / | mobile | zh-TW | chrome | 搜尋 | click | navigation or in-page change | url /zh-TW -> /zh-TW/search; DOM changed (4 mutations) | PASS |
| r00811 | storefront | / | mobile | zh-TW | chrome | 購物車，1 件商品 (`header-cart`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00812 | storefront | / | desktop | zh-TW | chrome | About us | click | navigation or in-page change | url /zh-TW -> /zh-TW/pages/about; DOM changed (4 mutations) | PASS |
| r00813 | storefront | / | desktop | en | chrome | Skip to content | keyboard: Tab to the link, Enter | navigation or in-page change | url /en -> /en#main; scrolled | PASS |
| r00814 | storefront | / | desktop | zh-TW | chrome | 搜尋 | click | visible change | url /zh-TW -> /zh-TW/search?q= | PASS |
| r00815 | storefront | / | desktop | en | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00816 | storefront | / | desktop | zh-TW | chrome | 購物車，1 件商品 (`header-cart`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00817 | storefront | / | desktop | en | chrome | All products | click | navigation or in-page change | url /en -> /en/products; DOM changed (4 mutations) | PASS |
| r00818 | storefront | / | desktop | en | chrome | Sweep home | click | navigation or in-page change | url /en -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00819 | storefront | / | desktop | en | chrome | About us | click | navigation or in-page change | url /en -> /en/pages/about; DOM changed (4 mutations) | PASS |
| r00820 | storefront | / | desktop | en | chrome | Search | click | visible change | url /en -> /en/search?q= | PASS |
| r00821 | storefront | / | mobile | zh-TW | chrome | +886 2 2345 6789 | click | navigation or in-page change | scrolled | PASS |
| r00822 | storefront | / | desktop | en | chrome | Cart, 1 items (`header-cart`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00823 | storefront | / | mobile | zh-TW | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r00824 | storefront | / | mobile | zh-TW | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r00825 | storefront | / | desktop | zh-TW | chrome | +886 2 2345 6789 | click | navigation or in-page change | scrolled | PASS |
| r00826 | storefront | / | mobile | zh-TW | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r00827 | storefront | / | desktop | zh-TW | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r00828 | storefront | / | mobile | zh-TW | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r00829 | storefront | / | desktop | zh-TW | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r00830 | storefront | / | mobile | zh-TW | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00831 | storefront | / | desktop | zh-TW | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r00832 | storefront | / | mobile | zh-TW | chrome | 隱私權政策 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/privacy; DOM changed (19 mutations) | PASS |
| r00833 | storefront | / | desktop | zh-TW | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r00834 | storefront | / | mobile | zh-TW | chrome | 服務條款 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/terms | PASS |
| r00835 | storefront | / | desktop | zh-TW | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00836 | storefront | / | desktop | en | chrome | +886 2 2345 6789 | click | navigation or in-page change | scrolled | PASS |
| r00837 | storefront | / | mobile | zh-TW | chrome | 退款、退貨與取消 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/legal/refunds (inline change only; no request fired) | PASS |
| r00838 | storefront | / | desktop | zh-TW | chrome | 隱私權政策 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/privacy; DOM changed (20 mutations) | PASS |
| r00839 | storefront | / | desktop | en | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r00840 | storefront | / | desktop | zh-TW | chrome | 服務條款 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/terms | PASS |
| r00841 | storefront | / | desktop | en | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r00842 | storefront | / | desktop | zh-TW | chrome | 退款、退貨與取消 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/legal/refunds (inline change only; no request fired) | PASS |
| r00843 | storefront | / | desktop | en | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r00844 | storefront | / | desktop | en | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r00845 | storefront | / | desktop | en | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00846 | storefront | / | desktop | en | chrome | Privacy Policy | click | navigation or in-page change | url /en -> /en/legal/privacy; DOM changed (20 mutations) | PASS |
| r00847 | storefront | / | desktop | en | chrome | Terms of Service | click | navigation or in-page change | url /en -> /en/legal/terms | PASS |
| r00848 | storefront | / | mobile | zh-TW | chrome | 運送 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/shipping | PASS |
| r00849 | storefront | / | desktop | en | chrome | Refunds, Returns and Cancellation | click | navigation or in-page change | url /en -> /en/legal/refunds | PASS |
| r00850 | storefront | / | mobile | zh-TW | chrome | 聯絡我們 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/contact | PASS |
| r00851 | storefront | / | desktop | en | chrome | Shipping | click | navigation or in-page change | url /en -> /en/legal/shipping | PASS |
| r00852 | storefront | / | mobile | zh-TW | chrome | 防詐騙提醒 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/anti-fraud | PASS |
| r00853 | storefront | / | desktop | zh-TW | chrome | 運送 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/shipping | PASS |
| r00854 | storefront | / | desktop | en | chrome | Contact | click | navigation or in-page change | url /en -> /en/legal/contact | PASS |
| r00855 | storefront | / | mobile | zh-TW | chrome | 資料刪除 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/data-deletion (inline change only; no request fired) | PASS |
| r00856 | storefront | / | desktop | zh-TW | chrome | 聯絡我們 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/contact | PASS |
| r00857 | storefront | / | mobile | zh-TW | chrome | 简体中文 | click | navigation or in-page change | url /zh-TW -> /zh-CN | PASS |
| r00858 | storefront | / | desktop | zh-TW | chrome | 防詐騙提醒 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/anti-fraud | PASS |
| r00859 | storefront | / | desktop | zh-TW | chrome | 資料刪除 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/data-deletion (inline change only; no request fired) | PASS |
| r00860 | storefront | / | desktop | zh-TW | chrome | 简体中文 | click | navigation or in-page change | url /zh-TW -> /zh-CN | PASS |
| r00861 | storefront | / | desktop | en | chrome | Shop safely | click | navigation or in-page change | url /en -> /en/legal/anti-fraud | PASS |
| r00862 | storefront | / | desktop | zh-TW | chrome | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r00863 | storefront | / | desktop | en | chrome | Data deletion | click | navigation or in-page change | url /en -> /en/data-deletion | PASS |
| r00864 | storefront | / | desktop | zh-TW | chrome | English | click | navigation or in-page change | url /zh-TW -> /en | PASS |
| r00865 | storefront | / | desktop | en | chrome | 简体中文 | click | navigation or in-page change | url /en -> /zh-CN | PASS |
| r00866 | storefront | / | desktop | en | chrome | 繁體中文 | click | navigation or in-page change | url /en -> /zh-TW | PASS |
| r00867 | storefront | / | mobile | zh-TW | chrome | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r00868 | storefront | / | mobile | zh-TW | chrome | English | click | navigation or in-page change | url /zh-TW -> /en | PASS |
| r00869 | storefront | / | mobile | zh-TW | main | 查看全部 | click | navigation or in-page change | url /zh-TW -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00870 | storefront | / | desktop | en | chrome | English | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r00871 | storefront | / | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/t04-5e91716e6a9c; DOM changed (4 mutations) [1 of 2 alike] | PASS |
| r00872 | storefront | / | desktop | en | main | View all | click | navigation or in-page change | url /en -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00873 | storefront | / | desktop | zh-TW | main | 查看全部 | click | navigation or in-page change | url /zh-TW -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00874 | storefront | / | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en -> /en/products/t04-5e91716e6a9c; DOM changed (4 mutations) [1 of 2 alike] | PASS |
| r00875 | storefront | / | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/t04-5e91716e6a9c; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00876 | storefront | / | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00877 | storefront | / | desktop | zh-TW | main | 查看全部 | click | navigation or in-page change | scrolled | PASS |
| r00878 | storefront | / | desktop | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00879 | storefront | / | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00880 | storefront | / | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00881 | storefront | / | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00882 | storefront | / | mobile | zh-TW | main | 查看全部 | click | navigation or in-page change | scrolled | PASS |
| r00883 | storefront | /products | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00884 | storefront | / | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00885 | storefront | /products | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00886 | storefront | / | mobile | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00887 | storefront | / | desktop | en | main | View all | click | navigation or in-page change | scrolled | PASS |
| r00888 | storefront | / | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00889 | storefront | /products | desktop | zh-TW | main | 全部商品 | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00890 | storefront | / | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00891 | storefront | / | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00892 | storefront | /products | desktop | zh-TW | main | Sweep home | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00893 | storefront | / | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00894 | storefront | /products | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00895 | storefront | /products | desktop | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00896 | storefront | /products | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00897 | storefront | / | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00898 | storefront | /products | desktop | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00899 | storefront | /products | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00900 | storefront | /products | mobile | zh-TW | main | 全部商品 | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00901 | storefront | /products | desktop | en | main | Home | click | navigation or in-page change | url /en/products -> /en; DOM changed (4 mutations) | PASS |
| r00902 | storefront | /products | desktop | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00903 | storefront | /products | mobile | zh-TW | main | Sweep home | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00904 | storefront | /products | desktop | en | main | All products | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00905 | storefront | /products | mobile | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00906 | storefront | /products | desktop | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/products -> /zh-TW/products?min=&max=&sort=newest | PASS |
| r00907 | storefront | /products | desktop | en | main | Sweep home | click | navigation or in-page change | url /en/products -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00908 | storefront | /products | mobile | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00909 | storefront | /products | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/products -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00910 | storefront | /products | mobile | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00911 | storefront | /products | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00912 | storefront | /products | mobile | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/products -> /zh-TW/products?min=&max=&sort=newest | PASS |
| r00913 | storefront | /products | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00914 | storefront | /products | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/products -> /en/products?min=&max=&sort=newest | PASS |
| r00915 | storefront | /products | desktop | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00916 | storefront | /products | desktop | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r00917 | storefront | /products | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00918 | storefront | /products | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/t04-5e91716e6a9c; DOM changed (5 mutations) | PASS |
| r00919 | storefront | /products | mobile | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00920 | storefront | /collections | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00921 | storefront | /products | mobile | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00922 | storefront | /collections | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00923 | storefront | /products | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00924 | storefront | /products | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00925 | storefront | /collections | desktop | zh-TW | main | S Sweep home 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00926 | storefront | /products | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /en/products?min=&max=&sort=price_asc -> /en/products/sweep-ceramic-mug; DOM changed (4 mutations) | PASS |
| r00927 | storefront | /collections | desktop | zh-TW | main | S Sweep wear 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00928 | storefront | /products | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/products -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00929 | storefront | /collections/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00930 | storefront | /collections/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00931 | storefront | /products | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/products -> /en/products/t04-5e91716e6a9c; DOM changed (4 mutations) | PASS |
| r00932 | storefront | /collections/[slug] | desktop | zh-TW | main | 商品分類 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections; DOM changed (4 mutations) | PASS |
| r00933 | storefront | /collections/[slug] | desktop | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00934 | storefront | /products | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r00935 | storefront | /collections/[slug] | desktop | zh-TW | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00936 | storefront | /collections | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00937 | storefront | /collections/[slug] | desktop | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00938 | storefront | /collections | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00939 | storefront | /collections/[slug] | desktop | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00940 | storefront | /collections | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00941 | storefront | /collections | mobile | zh-TW | main | S Sweep home 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00942 | storefront | /collections/[slug] | desktop | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00943 | storefront | /collections | desktop | en | main | Home | click | navigation or in-page change | url /en/collections -> /en; DOM changed (4 mutations) | PASS |
| r00944 | storefront | /collections | mobile | zh-TW | main | S Sweep wear 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00945 | storefront | /collections/[slug] | desktop | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-home?min=&max=&sort=newest | PASS |
| r00946 | storefront | /collections | desktop | en | main | S Sweep home 2 products (`collection-tile`) | click | navigation or in-page change | url /en/collections -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00947 | storefront | /collections | desktop | en | main | S Sweep wear 2 products (`collection-tile`) | click | navigation or in-page change | click failed: locator.click: Timeout 4000ms exceeded. Call log: [2m - waiting for locator('#main').first().locator('button, a[href], [role="button"], [role="ta | FAIL click-error |
| r00948 | storefront | /collections/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /zh-TW/collections/sweep-home ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavaila | FAIL http5xx |
| r00949 | storefront | /collections/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | no visible change within 3 s | FAIL no-effect |
| r00950 | storefront | /collections/[slug] | desktop | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00951 | storefront | /collections/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /_next/static/chunks/3tjd_epang8sh.js ; console-error: Failed to load resource: the server responded with a status of 503 (Service  | FAIL http5xx |
| r00952 | storefront | /collections/[slug] | mobile | zh-TW | main | 商品分類 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections; DOM changed (4 mutations) | PASS |
| r00953 | storefront | /collections/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/collections/sweep-home -> /en; DOM changed (4 mutations) | PASS |
| r00954 | storefront | /collections/[slug] | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/collections/sweep-home?min=&max=&sort=price_asc -> /zh-TW/products/t04-5e91716e6a9c; DOM changed (4 mutations) | PASS |
| r00955 | storefront | /collections/[slug] | mobile | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00956 | storefront | /collections/[slug] | desktop | en | main | Collections | click | navigation or in-page change | url /en/collections/sweep-home -> /en/collections; DOM changed (4 mutations) | PASS |
| r00957 | storefront | /collections/[slug] | mobile | zh-TW | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00958 | storefront | /collections/[slug] | desktop | en | main | All products | click | navigation or in-page change | url /en/collections/sweep-home -> /en/products; DOM changed (4 mutations) | PASS |
| r00959 | storefront | /collections/[slug] | mobile | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00960 | storefront | /collections/[slug] | desktop | en | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00961 | storefront | /collections/[slug] | mobile | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00962 | storefront | /collections/[slug] | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/collections/sweep-home -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00963 | storefront | /collections/[slug] | mobile | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00964 | storefront | /collections/[slug] | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00965 | storefront | /collections/[slug] | mobile | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-home?min=&max=&sort=newest | PASS |
| r00966 | storefront | /collections/[slug] | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00967 | storefront | /collections/[slug] | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00968 | storefront | /collections/[slug] | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/collections/sweep-home -> /en/collections/sweep-home?min=&max=&sort=newest | PASS |
| r00969 | storefront | /collections/[slug] | mobile | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00970 | storefront | /products/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /api/buyer/cart ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavailable) | FAIL http5xx |
| r00971 | storefront | /products/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00972 | storefront | /collections/[slug] | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/collections/sweep-home?min=&max=&sort=price_asc -> /zh-TW/products/t04-5e91716e6a9c; DOM changed (4 mutations) | PASS |
| r00973 | storefront | /collections/[slug] | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00974 | storefront | /products/[slug] | desktop | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00975 | storefront | /collections/[slug] | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00976 | storefront | /collections/[slug] | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/collections/sweep-home?min=&max=&sort=price_asc -> /en/products/t04-5e91716e6a9c; DOM changed (4 mutations) | PASS |
| r00977 | storefront | /products/[slug] | desktop | zh-TW | main | 放大圖片 | click | visible change | layer opened (Sweep Wool Scarf — 放大圖片); DOM changed (2 mutations) | PASS |
| r00978 | storefront | /products/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /zh-TW/collections/sweep-home ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavaila | FAIL http5xx |
| r00979 | storefront | /collections/[slug] | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/collections/sweep-home -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00980 | storefront | /products/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00981 | storefront | /products/[slug] | desktop | zh-TW | main | 增加數量 | click | visible change | DOM changed (2 mutations) | PASS |
| r00982 | storefront | /products/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /_next/static/chunks/3tjd_epang8sh.js ; console-error: Failed to load resource: the server responded with a status of 503 (Service  | FAIL http5xx |
| r00983 | storefront | /products/[slug] | desktop | zh-TW | main | 加入購物車 (`add-to-cart`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00984 | storefront | /products/[slug] | mobile | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00985 | storefront | /products/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en; DOM changed (4 mutations) | PASS |
| r00986 | storefront | /products/[slug] | desktop | zh-TW | main | 立即購買 (`buy-now`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00987 | storefront | /products/[slug] | mobile | zh-TW | main | 放大圖片 | click | visible change | layer opened (Sweep Wool Scarf — 放大圖片); DOM changed (2 mutations) | PASS |
| r00988 | storefront | /products/[slug] | desktop | en | main | All products | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/products; DOM changed (4 mutations) | PASS |
| r00989 | storefront | /products/[slug] | mobile | zh-TW | main | 增加數量 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00990 | storefront | /products/[slug] | desktop | zh-TW | main | Sweep home | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00991 | storefront | /products/[slug] | desktop | en | main | Enlarge photo | click | visible change | layer opened (Sweep Wool Scarf — Enlarge photo); DOM changed (2 mutations) | PASS |
| r00992 | storefront | /products/[slug] | mobile | zh-TW | main | 加入購物車 (`add-to-cart`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00993 | storefront | /products/[slug] | desktop | en | main | Increase quantity | click | visible change | DOM changed (2 mutations) | PASS |
| r00994 | storefront | /products/[slug] | desktop | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00995 | storefront | /products/[slug] | mobile | zh-TW | main | 立即購買 (`buy-now`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00996 | storefront | /products/[slug] | desktop | en | main | Add to cart (`add-to-cart`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00997 | storefront | /products/[slug] | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r00998 | storefront | /products/[slug] | mobile | zh-TW | main | Sweep home | click | navigation or in-page change | scrolled | PASS |
| r00999 | storefront | /products/[slug] | desktop | en | main | Buy now (`buy-now`) | click | visible change | DOM changed (3 mutations) | PASS |
| r01000 | storefront | /search | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /api/buyer/cart ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavailable) | FAIL http5xx |
| r01001 | storefront | /products/[slug] | mobile | zh-TW | main | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r01002 | storefront | /search | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW; DOM changed (4 mutations) | PASS |
| r01003 | storefront | /products/[slug] | desktop | en | main | Sweep home | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r01004 | storefront | /products/[slug] | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r01005 | storefront | /search | desktop | zh-TW | main | 搜尋 | click | visible change | the control's own state changed ( -> gone) | PASS |
| r01006 | storefront | /products/[slug] | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r01007 | storefront | /search | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /zh-TW/collections/sweep-home ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavaila | FAIL http5xx |
| r01008 | storefront | /search | desktop | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r01009 | storefront | /products/[slug] | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r01010 | storefront | /search | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW; DOM changed (5 mutations) | PASS |
| r01011 | storefront | /search | desktop | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r01012 | storefront | /search | mobile | zh-TW | main | 搜尋 | click | visible change | the control's own state changed ( -> gone) | PASS |
| r01013 | storefront | /search | desktop | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/search?q=Sweep -> /zh-TW/search?q=Sweep&min=&max=&sort=newest | PASS |
| r01014 | storefront | /search | desktop | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01015 | storefront | /search | desktop | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /zh-TW/search?q=Sweep&min=&max=&sort=price_asc -> /zh-TW/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r01016 | storefront | /search | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01017 | storefront | /search | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /_next/static/chunks/3tjd_epang8sh.js ; console-error: Failed to load resource: the server responded with a status of 503 (Service  | FAIL http5xx |
| r01018 | storefront | /search | desktop | en | main | Home | click | navigation or in-page change | url /en/search?q=Sweep -> /en; DOM changed (4 mutations) | PASS |
| r01019 | storefront | /search | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW/products/t04-5e91716e6a9c; DOM changed (5 mutations) | PASS |
| r01020 | storefront | /search | desktop | en | main | Search | click | visible change | the control's own state changed ( -> gone) | PASS |
| r01021 | storefront | /orders/lookup | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /api/buyer/cart ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavailable) | FAIL http5xx |
| r01022 | storefront | /search | mobile | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r01023 | storefront | /orders/lookup | desktop | zh-TW | main | 查詢 (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r01024 | storefront | /search | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r01025 | storefront | /order-link | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /api/buyer/cart ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavailable) | FAIL http5xx |
| r01026 | storefront | /search | mobile | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r01027 | storefront | /order-link | desktop | zh-TW | main | 查詢訂單 | click | navigation or in-page change | url /zh-TW/order-link -> /zh-TW/orders/lookup; DOM changed (12 mutations) | PASS |
| r01028 | storefront | /search | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r01029 | storefront | /search | mobile | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/search?q=Sweep -> /zh-TW/search?q=Sweep&min=&max=&sort=newest | PASS |
| r01030 | storefront | /search | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/search?q=Sweep -> /en/search?q=Sweep&min=&max=&sort=newest | PASS |
| r01031 | storefront | /search | mobile | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01032 | storefront | /search | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01033 | storefront | /search | mobile | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r01034 | storefront | /search | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /en/search?q=Sweep&min=&max=&sort=price_asc -> /en/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r01035 | storefront | /search | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01036 | storefront | /search | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/search?q=Sweep -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01037 | storefront | /search | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r01038 | storefront | /search | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/search?q=Sweep -> /en/products/t04-5e91716e6a9c; DOM changed (5 mutations) | PASS |
| r01039 | storefront | /claim | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /api/buyer/cart ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavailable) | FAIL http5xx |
| r01040 | storefront | /orders/lookup | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /zh-TW/collections/sweep-home ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavaila | FAIL http5xx |
| r01041 | storefront | /claim | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r01042 | storefront | /orders/lookup | mobile | zh-TW | main | 查詢 (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r01043 | storefront | /orders/lookup | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /_next/static/chunks/3tjd_epang8sh.js ; console-error: Failed to load resource: the server responded with a status of 503 (Service  | FAIL http5xx |
| r01044 | storefront | /legal/anti-fraud | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /api/buyer/cart ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavailable) | FAIL http5xx |
| r01045 | storefront | /orders/lookup | desktop | en | main | Find order (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r01046 | storefront | /order-link | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /zh-TW/collections/sweep-home ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavaila | FAIL http5xx |
| r01047 | storefront | /legal/anti-fraud | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /zh-TW; DOM changed (4 mutations) | PASS |
| r01048 | storefront | /order-link | mobile | zh-TW | main | 查詢訂單 | click | navigation or in-page change | url /zh-TW/order-link -> /zh-TW/orders/lookup; DOM changed (12 mutations) | PASS |
| r01049 | storefront | /order-link | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /_next/static/chunks/3tjd_epang8sh.js ; console-error: Failed to load resource: the server responded with a status of 503 (Service  | FAIL http5xx |
| r01050 | storefront | /legal/anti-fraud | desktop | zh-TW | main | 內政部警政署 — 165 全民防騙網 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r01051 | storefront | /order-link | desktop | en | main | Look up an order | click | navigation or in-page change | url /en/order-link -> /en/orders/lookup; DOM changed (12 mutations) | PASS |
| r01052 | storefront | /claim | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /_next/static/chunks/3tjd_epang8sh.js ; console-error: Failed to load resource: the server responded with a status of 503 (Service  | FAIL http5xx |
| r01053 | storefront | /claim | desktop | en | main | Language | selectOption(zh-CN) | visible change | url /en/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r01054 | storefront | /legal/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /api/buyer/cart ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavailable) | FAIL http5xx |
| r01055 | storefront | /privacy | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /api/buyer/cart ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavailable) | FAIL http5xx |
| r01056 | storefront | /privacy | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/privacy -> /zh-CN/privacy; DOM changed (28 mutations) | PASS |
| r01057 | storefront | /legal/anti-fraud | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /_next/static/chunks/3tjd_epang8sh.js ; console-error: Failed to load resource: the server responded with a status of 503 (Service  | FAIL http5xx |
| r01058 | storefront | /legal/anti-fraud | desktop | en | main | Home | click | navigation or in-page change | url /en/legal/anti-fraud -> /en; DOM changed (4 mutations) | PASS |
| r01059 | storefront | /privacy | desktop | zh-TW | main | 開啟: 允許店鋪透過 Messenger 或 Instagram 私訊向我發送行銷訊息。 (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01060 | storefront | /legal/anti-fraud | desktop | en | main | Taiwan National Police Agency — 165 anti-fraud service | click | navigation or in-page change | url /en/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r01061 | storefront | /legal/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /_next/static/chunks/3tjd_epang8sh.js ; console-error: Failed to load resource: the server responded with a status of 503 (Service  | FAIL http5xx |
| r01062 | storefront | /privacy | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /_next/static/chunks/3tjd_epang8sh.js ; console-error: Failed to load resource: the server responded with a status of 503 (Service  | FAIL http5xx |
| r01063 | storefront | /claim | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /zh-TW/collections/sweep-home ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavaila | FAIL http5xx |
| r01064 | storefront | /privacy | desktop | en | main | Language | selectOption(zh-CN) | visible change | url /en/privacy -> /zh-CN/privacy; DOM changed (29 mutations) | PASS |
| r01065 | storefront | /claim | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r01066 | storefront | /privacy | desktop | en | main | Turn on: The store may send me marketing messages on Messenger or Instagram DM. (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01067 | storefront | /legal/anti-fraud | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /zh-TW/collections/sweep-home ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavaila | FAIL http5xx |
| r01068 | storefront | /legal/anti-fraud | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /zh-TW; DOM changed (5 mutations) | PASS |
| r01069 | storefront | /legal/anti-fraud | mobile | zh-TW | main | 內政部警政署 — 165 全民防騙網 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r01070 | storefront | /legal/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /zh-TW/collections/sweep-home ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavaila | FAIL http5xx |
| r01071 | storefront | /privacy | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /zh-TW/collections/sweep-home ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavaila | FAIL http5xx |
| r01072 | storefront | /privacy | desktop | zh-TW | main | 開啟: 使用我的購買紀錄為我個人化 Meta 廣告。 (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01073 | storefront | /privacy | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/privacy -> /zh-CN/privacy; DOM changed (28 mutations) | PASS |
| r01074 | storefront | /privacy | mobile | zh-TW | main | 開啟: 允許店鋪透過 Messenger 或 Instagram 私訊向我發送行銷訊息。 (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01075 | storefront | /privacy | desktop | en | main | Turn on: Use my purchase to personalize Meta ads for me. (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01076 | storefront | /privacy | desktop | zh-TW | main | 下載我的資料 (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r01077 | storefront | /privacy | mobile | zh-TW | main | 開啟: 使用我的購買紀錄為我個人化 Meta 廣告。 (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01078 | storefront | /privacy | desktop | en | main | Download my data (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r01079 | storefront | /privacy | desktop | zh-TW | main | 抹除我的資料… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations) (confirmation shown, then cancelled) | PASS |
| r01080 | storefront | /data-deletion | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /api/buyer/cart ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavailable) | FAIL http5xx |
| r01081 | storefront | /privacy | mobile | zh-TW | main | 下載我的資料 (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r01082 | storefront | /data-deletion | desktop | zh-TW | main | 简体中文 | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-CN/data-deletion | PASS |
| r01083 | storefront | /privacy | mobile | zh-TW | main | 抹除我的資料… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r01084 | storefront | /data-deletion | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /zh-TW/collections/sweep-home ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavaila | FAIL http5xx |
| r01085 | storefront | /data-deletion | mobile | zh-TW | main | 简体中文 | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-CN/data-deletion | PASS |
| r01086 | storefront | /privacy | desktop | en | main | Erase my data… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations) (confirmation shown, then cancelled) | PASS |
| r01087 | storefront | /data-deletion | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /_next/static/chunks/3tjd_epang8sh.js ; console-error: Failed to load resource: the server responded with a status of 503 (Service  | FAIL http5xx |
| r01088 | storefront | /data-deletion | desktop | zh-TW | main | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r01089 | storefront | /data-deletion | desktop | en | main | 简体中文 | click | navigation or in-page change | url /en/data-deletion -> /zh-CN/data-deletion | PASS |
| r01090 | storefront | /data-deletion | desktop | zh-TW | main | English | click | navigation or in-page change | url /zh-TW/data-deletion -> /en/data-deletion | PASS |
| r01091 | storefront | /data-deletion | desktop | en | main | 繁體中文 | click | navigation or in-page change | url /en/data-deletion -> /zh-TW/data-deletion | PASS |
| r01092 | storefront | /data-deletion | mobile | zh-TW | main | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r01093 | storefront | /data-deletion | desktop | zh-TW | main | 開啟隱私頁面 (`data-deletion-privacy-link`) | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-TW/privacy | PASS |
| r01094 | storefront | /pages/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /api/buyer/cart ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavailable) | FAIL http5xx |
| r01095 | storefront | /data-deletion | mobile | zh-TW | main | English | click | navigation or in-page change | url /zh-TW/data-deletion -> /en/data-deletion | PASS |
| r01096 | storefront | /data-deletion | desktop | en | main | English | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r01097 | storefront | /pages/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | no visible change within 3 s | FAIL no-effect |
| r01098 | storefront | /data-deletion | desktop | en | main | Open the privacy page (`data-deletion-privacy-link`) | click | navigation or in-page change | control no longer present at this position (page state changed by an earlier click) | SKIP |
| r01099 | storefront | /data-deletion | mobile | zh-TW | main | 開啟隱私頁面 (`data-deletion-privacy-link`) | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-TW/privacy; DOM changed (60 mutations) | PASS |
| r01100 | storefront | /pages/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /_next/static/chunks/3tjd_epang8sh.js ; console-error: Failed to load resource: the server responded with a status of 503 (Service  | FAIL http5xx |
| r01101 | storefront | /pages/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/pages/about -> /en; DOM changed (4 mutations) | PASS |
| r01102 | storefront | /pages/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 \| http5xx: 503 GET /zh-TW/collections/sweep-home ; console-error: Failed to load resource: the server responded with a status of 503 (Service Unavaila | FAIL http5xx |
| r01103 | storefront | /pages/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/pages/about -> /zh-TW; DOM changed (5 mutations) | PASS |
| r01104 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | open the product list and click New product | real clicks | the create form opens | as expected | PASS |
| r01105 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | fill the name and description | real clicks | the draft fields retain the entered values | as expected | PASS |
| r01106 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | add Size = S, M | real clicks | the editor proposes two SKU rows | as expected | PASS |
| r01107 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | type both SKU prices | real clicks | the matrix retains both prices | as expected | PASS |
| r01108 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | set opening quantities 9 and 7, then save the document | real clicks | both SKU quantities persist | as expected | PASS |
| r01109 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | upload a cover, set Active and save | real clicks | the product is active and persists after a reload | as expected | PASS |
| r01110 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | the merchant list shows the product (search + click) | real clicks | the row is listed with status active | as expected | PASS |
| r01111 | journey | J1 storefront | desktop | zh-TW | journey | the buyer opens All products and clicks the new product | real clicks | its page opens with the title and an enabled Add to cart | as expected | PASS |
| r01112 | journey | J2/J3 admin orders | desktop | zh-TW | journey | admin Orders: the storefront COD order is listed (click through pages) | real clicks | the row of the order id can be expanded | as expected | PASS |
| r01113 | journey | J2/J3 admin orders | desktop | zh-TW | journey | record the manual shipment (carrier Black Cat + tracking) and click Record | real clicks | the shipment record is shown | as expected | PASS |
| r01114 | journey | J2/J3 admin orders | desktop | zh-TW | journey | click Collected, confirm in the dialog | real clicks | the order shows COLLECTED and the button is gone | as expected | PASS |
| r01115 | journey | J2/J3 admin orders | desktop | zh-TW | journey | reload the order list: collected persists | real clicks | COLLECTED is still shown after a reload | as expected | PASS |
| r01116 | journey | J2 storefront order lookup | desktop | zh-TW | journey | a fresh buyer browser looks the order up (order id + phone) and sees it collected | real clicks | the lookup shows the order with the collected amount | as expected | PASS |
| r01117 | journey | J4 custom domain | desktop | zh-TW | journey | open Settings and find the custom domain card | real clicks | the storefront domains card is shown | as expected | PASS |
| r01118 | journey | J4 custom domain | desktop | zh-TW | journey | type a custom hostname and click Request | real clicks | DNS instructions appear: a TXT name, a TXT value and the CNAME target | as expected | PASS |
| r01119 | journey | J4 custom domain | desktop | zh-TW | journey | reload Settings: the requested domain is listed | real clicks | the domain row persists in state REQUESTED | as expected | PASS |
| r01120 | journey | J5 sign out | desktop | zh-TW | journey | open the dashboard, click Sign out | real clicks | the session ends: the page asks to sign in again | as expected | PASS |
| r01121 | journey | J5 sign out | desktop | zh-TW | journey | reload after sign out | real clicks | the dashboard is not shown without a session | as expected | PASS |
| r01122 | storefront | /live | - | - | page | (not implemented) | - | ui-architecture section 4 lists /live (S6) | apps/storefront/app/[locale] has no live route in this base; nothing to click | SKIP |
