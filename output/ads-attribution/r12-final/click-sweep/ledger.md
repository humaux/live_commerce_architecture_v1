# G-UI8 click ledger

Generated 2026-10-04T07:25:22.234Z. 123 page/viewport/locale units opened (0 with a page-load failure), 1003 control clicks (981 pass, 0 fail, 22 skip), 18 journey steps (18 pass, 0 fail); failures: known 0, new 0; stale known-defect entries 0.

Every row is one real Playwright interaction (click / selectOption). Destructive and irreversible controls stop at their confirmation and are cancelled; `skip` rows name why.

| # | App | Page | Viewport | Locale | Scope | Control | Action | Expected | Actual | Result |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| r00001 | admin | /studio/claims | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00002 | admin | /studio | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00003 | admin | / | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00004 | admin | /studio | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00005 | admin | /studio/claims | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio/claims?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&scene=fd2ae6bc-28c9-4eea-8401-c576892a85e5 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49 | PASS |
| r00006 | admin | /studio/claims | desktop | zh-TW | main | 直播工作室 | click | visible change | url /zh-TW/studio/claims?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&scene=fd2ae6bc-28c9-4eea-8401-c576892a85e5 -> /zh-TW/studio?store=9b3cec4a-09ba-4b8e-90f1-33 | PASS |
| r00007 | admin | /studio | desktop | zh-TW | main | 留言關鍵字登記 (`studio-open-claims`) | click | visible change | url /zh-TW/studio?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/studio/claims?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&scene=fd2ae6bc-28c9-4eea-8401-c5 | PASS |
| r00008 | admin | /studio/claims | desktop | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (2 mutations) | PASS |
| r00009 | admin | /studio | desktop | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (18 mutations) | PASS |
| r00010 | admin | /studio/claims | desktop | zh-TW | main | 數量規則 | selectOption(KEYWORD_QTY_ONLY) | visible change | the control's own state changed (EXACT -> KEYWORD_QTY_ONLY) | PASS |
| r00011 | admin | /studio | desktop | zh-TW | main | ＋ 新增場次 | click | visible change | DOM changed (8 mutations) | PASS |
| r00012 | admin | /studio/claims | desktop | zh-TW | main | 開放登記窗口 | click | visible change | DOM changed (21 mutations) | PASS |
| r00013 | admin | /studio | desktop | zh-TW | main | Click sweep scene 585624dcd295 草稿 預定時間（台北時間，選填） | click | visible change | DOM changed (5 mutations) | PASS |
| r00014 | admin | /studio/claims | desktop | zh-TW | main | 這則貼文或直播所屬的專頁 | selectOption(652057733238177378) | visible change | the control's own state changed ( -> 652057733238177378) | PASS |
| r00015 | admin | /studio | desktop | zh-TW | main | 畫面比例 | selectOption(16:9) | visible change | DOM changed (2 mutations); the control's own state changed (9:16 -> 16:9) | PASS |
| r00016 | admin | /studio/claims | desktop | zh-TW | main | 平台 | selectOption(facebook) | visible change | the control's own state changed ( -> facebook) | PASS |
| r00017 | admin | /studio | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00018 | admin | / | desktop | zh-TW | skip | 跳至內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180#main; scrolled | PASS |
| r00019 | admin | /studio/claims | desktop | zh-TW | main | 回覆語言 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN) | PASS |
| r00020 | admin | /studio | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00021 | admin | /studio/claims | desktop | zh-TW | main | 傳送附帶購物車連結的私訊回覆 | click (toggle) | visible change | the control's own state changed (false -> true) | PASS |
| r00022 | admin | / | desktop | zh-TW | topbar | 切換商店 (`shell-store-selector`) | selectOption | visible change | select has a single option | SKIP |
| r00023 | admin | /studio | mobile | zh-TW | main | 留言關鍵字登記 (`studio-open-claims`) | click | visible change | url /zh-TW/studio?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/studio/claims?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&scene=fd2ae6bc-28c9-4eea-8401-c5 | PASS |
| r00024 | admin | / | desktop | zh-TW | topbar | 語言 (`locale-switch`) | selectOption(zh-CN) | visible change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-CN?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (13 mutations) | PASS |
| r00025 | admin | /studio/claims | desktop | zh-TW | main | 從此來源收集留言 | click (toggle) | visible change | the control's own state changed (true -> false) | PASS |
| r00026 | admin | /studio | mobile | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (18 mutations) | PASS |
| r00027 | admin | / | desktop | zh-TW | topbar | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00028 | admin | /studio/claims | desktop | zh-TW | main | 儲存留言來源 | click | visible change | DOM changed (2 mutations); validation shown | PASS |
| r00029 | admin | /studio | mobile | zh-TW | main | ＋ 新增場次 | click | visible change | DOM changed (8 mutations) | PASS |
| r00030 | admin | /studio/claims | desktop | zh-TW | main | 商品 | selectOption(b3bea372-17f5-4d97-a542-fe223efc6de7) | visible change | the control's own state changed ( -> b3bea372-17f5-4d97-a542-fe223efc6de7); scrolled | PASS |
| r00031 | admin | / | desktop | zh-TW | layer of 說明 | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00032 | admin | /studio | mobile | zh-TW | main | Click sweep scene 585624dcd295 草稿 預定時間（台北時間，選填） | click | visible change | DOM changed (5 mutations) | PASS |
| r00033 | admin | /studio/claims | desktop | zh-TW | main | 新增關鍵字 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00034 | admin | /studio | mobile | zh-TW | main | 畫面比例 | selectOption(16:9) | visible change | DOM changed (2 mutations); the control's own state changed (9:16 -> 16:9); scrolled | PASS |
| r00035 | admin | / | desktop | zh-TW | topbar | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00036 | admin | /studio/claims | desktop | zh-TW | main | 加入關鍵字庫 (`claims-add-library`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00037 | admin | /studio | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00038 | admin | / | desktop | zh-TW | layer of 帳號 | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00039 | admin | /studio | desktop | en | main | Overview | click | navigation or in-page change | url /en/studio?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00040 | admin | /studio/claims | desktop | zh-TW | main | 買家 | selectOption | visible change | select has a single option | SKIP |
| r00041 | admin | / | desktop | zh-TW | layer of 帳號 | 登出 (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00042 | admin | /studio | desktop | en | main | Keyword claims (`studio-open-claims`) | click | visible change | url /en/studio?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/studio/claims?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&scene=fd2ae6bc-28c9-4eea-8401-c576892a | PASS |
| r00043 | admin | /studio/claims | desktop | zh-TW | main | 記錄留言 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00044 | admin | /studio/claims | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00045 | admin | / | desktop | zh-TW | rail | 總覽 (`nav-group-overview`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00046 | admin | /studio | desktop | en | main | Refresh facts | click | visible change | DOM changed (18 mutations) | PASS |
| r00047 | admin | /studio/claims | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio/claims?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&scene=fd2ae6bc-28c9-4eea-8401-c576892a85e5 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49 | PASS |
| r00048 | admin | /studio | desktop | en | main | ＋ New scene | click | visible change | DOM changed (8 mutations) | PASS |
| r00049 | admin | / | desktop | zh-TW | rail | 直播與貼文 (`nav-group-live`) | click | visible change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/studio?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (17 mutations) | PASS |
| r00050 | admin | /studio/claims | mobile | zh-TW | main | 直播工作室 | click | visible change | url /zh-TW/studio/claims?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&scene=fd2ae6bc-28c9-4eea-8401-c576892a85e5 -> /zh-TW/studio?store=9b3cec4a-09ba-4b8e-90f1-33 | PASS |
| r00051 | admin | /studio | desktop | en | main | Click sweep scene 585624dcd295 Draft Scheduled time (Taipei time, optional) | click | visible change | DOM changed (5 mutations) | PASS |
| r00052 | admin | / | desktop | zh-TW | rail | 訂單與出貨 (`nav-orders`) | click | visible change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00053 | admin | /studio/claims | mobile | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (2 mutations) | PASS |
| r00054 | admin | /studio | desktop | en | main | Canvas ratio | selectOption(16:9) | visible change | DOM changed (2 mutations); the control's own state changed (9:16 -> 16:9) | PASS |
| r00055 | admin | / | desktop | zh-TW | rail | 商品與庫存 + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00056 | admin | /studio/claims | mobile | zh-TW | main | 數量規則 | selectOption(KEYWORD_QTY_ONLY) | visible change | the control's own state changed (EXACT -> KEYWORD_QTY_ONLY) | PASS |
| r00057 | admin | /orders | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00058 | admin | / | desktop | zh-TW | rail | 顧客 (`nav-group-customers`) | click | visible change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00059 | admin | /studio/claims | mobile | zh-TW | main | 開放登記窗口 | click | visible change | DOM changed (21 mutations) | PASS |
| r00060 | admin | /orders | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00061 | admin | /studio/claims | mobile | zh-TW | main | 這則貼文或直播所屬的專頁 | selectOption(652057733238177378) | visible change | the control's own state changed ( -> 652057733238177378); scrolled | PASS |
| r00062 | admin | /orders | desktop | zh-TW | main | 訂單狀態 (`state-filter`) | selectOption(all) | visible change | DOM changed (8 mutations) | PASS |
| r00063 | admin | / | desktop | zh-TW | rail | 行銷優惠 + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00064 | admin | /studio/claims | mobile | zh-TW | main | 平台 | selectOption(facebook) | visible change | the control's own state changed ( -> facebook); scrolled | PASS |
| r00065 | admin | /orders | desktop | zh-TW | main | 重新整理 (`orders-refresh`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00066 | admin | / | desktop | zh-TW | rail | 網路商店 (`nav-group-storefront`) | click | visible change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/design?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (3 mutations) | PASS |
| r00067 | admin | /studio/claims | mobile | zh-TW | main | 回覆語言 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN); scrolled | PASS |
| r00068 | admin | /orders | desktop | zh-TW | main | 匯出未出貨訂單（CSV） (`orders-export`) | click | navigation or in-page change | download: unshipped-9b3cec4a-202610040714.csv | PASS |
| r00069 | admin | / | desktop | zh-TW | rail | 收款與報表 (`nav-group-finance`) | click | visible change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/finance?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00070 | admin | /studio/claims | mobile | zh-TW | main | 傳送附帶購物車連結的私訊回覆 | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00071 | admin | /orders | desktop | zh-TW | main | 付款方式 (`orders-payment-filter`) | selectOption(card) | visible change | the control's own state changed ( -> card) | PASS |
| r00072 | admin | / | desktop | zh-TW | rail | 設定 + (`nav-group-settings`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00073 | admin | /studio/claims | mobile | zh-TW | main | 從此來源收集留言 | click (toggle) | visible change | the control's own state changed (true -> false); scrolled | PASS |
| r00074 | admin | /orders | desktop | zh-TW | main | 配送方式 (`orders-delivery-filter`) | selectOption(home) | visible change | the control's own state changed ( -> home) | PASS |
| r00075 | admin | /studio/claims | mobile | zh-TW | main | 儲存留言來源 | click | visible change | DOM changed (2 mutations); validation shown; scrolled | PASS |
| r00076 | admin | /orders | desktop | zh-TW | main | 直播場次 (`orders-session-filter`) | selectOption | visible change | select has a single option | SKIP |
| r00077 | admin | / | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | DOM changed (10 mutations) | PASS |
| r00078 | admin | /studio/claims | mobile | zh-TW | main | 商品 | selectOption(b3bea372-17f5-4d97-a542-fe223efc6de7) | visible change | the control's own state changed ( -> b3bea372-17f5-4d97-a542-fe223efc6de7); scrolled | PASS |
| r00079 | admin | /orders | desktop | zh-TW | main | 套用篩選 (`orders-apply`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00080 | admin | / | desktop | zh-TW | main | 待確認的轉帳 1 (`todo-transfer`) | click | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?state=AWAITING_TRANSFER&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutat | PASS |
| r00081 | admin | /studio/claims | mobile | zh-TW | main | 新增關鍵字 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00082 | admin | /orders | desktop | zh-TW | main | 清除篩選 (`orders-reset`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (11 mutations); the control's own state changed ( -> gone) (inline change only; no request fired) | PASS |
| r00083 | admin | / | desktop | zh-TW | main | 待出貨訂單 2 (`todo-ship`) | click | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?state=unshipped&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00084 | admin | /studio/claims | mobile | zh-TW | main | 加入關鍵字庫 (`claims-add-library`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00085 | admin | /orders | desktop | zh-TW | main | 全部 5 (`orders-bucket-all`) | click | visible change | DOM changed (11 mutations); the control's own state changed (\|\|\|\|true -> gone) | PASS |
| r00086 | admin | / | desktop | zh-TW | main | 待建立的超商托運單 1 (`todo-label`) | click | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?state=unshipped&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00087 | admin | /studio/claims | mobile | zh-TW | main | 買家 | selectOption | visible change | select has a single option | SKIP |
| r00088 | admin | /orders | desktop | zh-TW | main | 待付款 4 (`orders-bucket-unpaid`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00089 | admin | / | desktop | zh-TW | main | 低庫存規格（5 件以下） 0 (`todo-stock`) | click | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (16 mutations) | PASS |
| r00090 | admin | /studio/claims | mobile | zh-TW | main | 記錄留言 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00091 | admin | /orders | desktop | zh-TW | main | 待核對轉帳 1 (`orders-bucket-transfer_review`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00092 | admin | /studio/claims | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00093 | admin | / | desktop | zh-TW | main | 處理中的退款 0 (`todo-refunds`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (19 mutations) (inline change only; no request fired) | PASS |
| r00094 | admin | /studio/claims | desktop | en | main | Overview | click | navigation or in-page change | url /en/studio/claims?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&scene=fd2ae6bc-28c9-4eea-8401-c576892a85e5 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; D | PASS |
| r00095 | admin | /orders | desktop | zh-TW | main | 待出貨 2 (`orders-bucket-ready_to_ship`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00096 | admin | / | desktop | zh-TW | main | a5dcaa60 | click | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?order=a5dcaa60-34ff-4d3f-9f39-d18b4f5d8c78&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DO | PASS |
| r00097 | admin | /studio/claims | desktop | en | main | Live Studio | click | visible change | url /en/studio/claims?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&scene=fd2ae6bc-28c9-4eea-8401-c576892a85e5 -> /en/studio?store=9b3cec4a-09ba-4b8e-90f1-3393a7d4 | PASS |
| r00098 | admin | /orders | desktop | zh-TW | main | 待交寄 1 (`orders-bucket-ready_to_consign`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00099 | admin | / | desktop | zh-TW | main | b540064f | click | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?order=b540064f-4d30-4615-b811-d047125a7ebb&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DO | PASS |
| r00100 | admin | /studio/claims | desktop | en | main | Refresh facts | click | visible change | DOM changed (2 mutations) | PASS |
| r00101 | admin | /orders | desktop | zh-TW | main | 已出貨 0 (`orders-bucket-shipped`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00102 | admin | / | desktop | zh-TW | main | f5126af7 | click | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?order=f5126af7-b7aa-4ee3-a721-a2852a961df9&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DO | PASS |
| r00103 | admin | /studio/claims | desktop | en | main | Quantity rule | selectOption(KEYWORD_QTY_ONLY) | visible change | the control's own state changed (EXACT -> KEYWORD_QTY_ONLY) | PASS |
| r00104 | admin | /orders | desktop | zh-TW | main | 已完成 0 (`orders-bucket-completed`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00105 | admin | / | desktop | zh-TW | main | 5cfe749c | click | navigation or in-page change | scrolled | PASS |
| r00106 | admin | /studio/claims | desktop | en | main | Open claim window | click | visible change | DOM changed (21 mutations) | PASS |
| r00107 | admin | /orders | desktop | zh-TW | main | 已取消 0 (`orders-bucket-cancelled`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00108 | admin | /studio/claims | desktop | en | main | Page for this post or live stream | selectOption(652057733238177378) | visible change | the control's own state changed ( -> 652057733238177378) | PASS |
| r00109 | admin | / | desktop | zh-TW | main | 1a450fd2 | click | navigation or in-page change | scrolled | PASS |
| r00110 | admin | /orders | desktop | zh-TW | main | 展開訂單詳情: a5dcaa60-34ff-4d3f-9f39-d18b4f5d8c78 (`order-expand-a5dcaa60-34ff-4d3f-9f39-d18b4f5d8c78`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00111 | admin | /studio/claims | desktop | en | main | Platform | selectOption(facebook) | visible change | the control's own state changed ( -> facebook) | PASS |
| r00112 | admin | / | desktop | zh-TW | main | 113e5295 | click | navigation or in-page change | scrolled | PASS |
| r00113 | admin | /orders | desktop | zh-TW | main | 展開訂單詳情: b540064f-4d30-4615-b811-d047125a7ebb (`order-expand-b540064f-4d30-4615-b811-d047125a7ebb`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00114 | admin | /studio/claims | desktop | en | main | Reply language | selectOption(zh-CN) | visible change | the control's own state changed (en -> zh-CN) | PASS |
| r00115 | admin | / | desktop | zh-TW | main | 全部訂單 | click | navigation or in-page change | scrolled | PASS |
| r00116 | admin | /orders | desktop | zh-TW | main | 展開訂單詳情: 1a450fd2-9556-46c2-9e78-de30ec74d01b (`order-expand-1a450fd2-9556-46c2-9e78-de30ec74d01b`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00117 | admin | /studio/claims | desktop | en | main | Send a private reply with the cart link | click (toggle) | visible change | the control's own state changed (false -> true) | PASS |
| r00118 | admin | / | desktop | zh-TW | main | 建立訂單 (`action-create-order`) | click | navigation or in-page change | scrolled | PASS |
| r00119 | admin | /orders | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00120 | admin | /studio/claims | desktop | en | main | Collect comments from this source | click (toggle) | visible change | the control's own state changed (true -> false) | PASS |
| r00121 | admin | /orders | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00122 | admin | / | desktop | zh-TW | main | 匯入或匯出商品 (`action-import`) | click | navigation or in-page change | scrolled | PASS |
| r00123 | admin | /studio/claims | desktop | en | main | Save comment source | click | visible change | DOM changed (2 mutations); validation shown | PASS |
| r00124 | admin | /orders | mobile | zh-TW | main | 更多篩選 (`orders-more-filters`) | click | visible change | DOM changed (3 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00125 | admin | / | desktop | zh-TW | main | 庫存 (`action-inventory`) | click | navigation or in-page change | scrolled | PASS |
| r00126 | admin | /studio/claims | desktop | en | main | Product | selectOption(b3bea372-17f5-4d97-a542-fe223efc6de7) | visible change | the control's own state changed ( -> b3bea372-17f5-4d97-a542-fe223efc6de7) | PASS |
| r00127 | admin | /orders | mobile | zh-TW | main | 重新整理 (`orders-refresh`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00128 | admin | / | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00129 | admin | /studio/claims | desktop | en | main | Add offer | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00130 | admin | /orders | mobile | zh-TW | main | 匯出未出貨訂單（CSV） (`orders-export`) | click | navigation or in-page change | download: unshipped-9b3cec4a-202610040715.csv | PASS |
| r00131 | admin | /studio/claims | desktop | en | main | Add to library (`claims-add-library`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00132 | admin | /orders | mobile | zh-TW | main | 套用篩選 (`orders-apply`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00133 | admin | /studio/claims | desktop | en | main | Buyer | selectOption | visible change | select has a single option | SKIP |
| r00134 | admin | /orders | mobile | zh-TW | main | 清除篩選 (`orders-reset`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (11 mutations); the control's own state changed ( -> gone) (inline change only; no request fired) | PASS |
| r00135 | admin | /studio/claims | desktop | en | main | Record comment | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00136 | admin | /orders | mobile | zh-TW | main | 全部 5 (`orders-bucket-all`) | click | visible change | DOM changed (11 mutations); the control's own state changed (\|\|\|\|true -> gone) | PASS |
| r00137 | admin | /orders/new | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00138 | admin | /orders/new | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders/new?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00139 | admin | /orders | mobile | zh-TW | main | 待付款 4 (`orders-bucket-unpaid`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00140 | admin | /orders/new | desktop | zh-TW | main | 搜尋 (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00141 | admin | /orders | mobile | zh-TW | main | 待核對轉帳 1 (`orders-bucket-transfer_review`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00142 | admin | /orders/new | desktop | zh-TW | main | 配送方式 (`mo-option`) | selectOption(e0a73ab5-a2c7-4b18-a04a-f383d3b230ab\|TW\|cvs6f2ff444cf3f) | visible change | DOM changed (4 mutations); the control's own state changed ( -> e0a73ab5-a2c7-4b18-a04a-f383d3b230ab\|TW\|cvs6f2ff444cf3f) | PASS |
| r00143 | admin | / | mobile | zh-TW | skip | 跳至內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180#main; scrolled | PASS |
| r00144 | admin | /orders | mobile | zh-TW | main | 待出貨 2 (`orders-bucket-ready_to_ship`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00145 | admin | /orders/new | desktop | zh-TW | main | 顧客連結 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN) | PASS |
| r00146 | admin | /orders | mobile | zh-TW | main | 待交寄 1 (`orders-bucket-ready_to_consign`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00147 | admin | / | mobile | zh-TW | topbar | 開啟導覽 | click | visible change | DOM changed (3 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00148 | admin | /orders/new | desktop | zh-TW | main | 回到訂單 | click | navigation or in-page change | url /zh-TW/orders/new?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (19 mutations) | PASS |
| r00149 | admin | /orders | mobile | zh-TW | main | 展開訂單詳情: a5dcaa60-34ff-4d3f-9f39-d18b4f5d8c78 (`order-expand-a5dcaa60-34ff-4d3f-9f39-d18b4f5d8c78`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00150 | admin | / | mobile | zh-TW | topbar | 切換商店 (`shell-store-selector`) | selectOption | visible change | select has a single option | SKIP |
| r00151 | admin | /orders/new | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00152 | admin | / | mobile | zh-TW | topbar | 語言 (`locale-switch`) | selectOption(zh-CN) | visible change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-CN?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (17 mutations) | PASS |
| r00153 | admin | /orders | mobile | zh-TW | main | 展開訂單詳情: b540064f-4d30-4615-b811-d047125a7ebb (`order-expand-b540064f-4d30-4615-b811-d047125a7ebb`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00154 | admin | /orders/new | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders/new?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00155 | admin | / | mobile | zh-TW | topbar | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00156 | admin | /orders | mobile | zh-TW | main | 展開訂單詳情: 1a450fd2-9556-46c2-9e78-de30ec74d01b (`order-expand-1a450fd2-9556-46c2-9e78-de30ec74d01b`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00157 | admin | /orders/new | mobile | zh-TW | main | 搜尋 (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00158 | admin | /orders | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00159 | admin | /orders/new | mobile | zh-TW | main | 配送方式 (`mo-option`) | selectOption(e0a73ab5-a2c7-4b18-a04a-f383d3b230ab\|TW\|cvs6f2ff444cf3f) | visible change | DOM changed (4 mutations); the control's own state changed ( -> e0a73ab5-a2c7-4b18-a04a-f383d3b230ab\|TW\|cvs6f2ff444cf3f); scrolled | PASS |
| r00160 | admin | / | mobile | zh-TW | layer of 說明 | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00161 | admin | /orders | desktop | en | main | Overview | click | navigation or in-page change | url /en/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00162 | admin | /orders/new | mobile | zh-TW | main | 顧客連結 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN); scrolled | PASS |
| r00163 | admin | / | mobile | zh-TW | topbar | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00164 | admin | /orders | desktop | en | main | Order status (`state-filter`) | selectOption(all) | visible change | DOM changed (8 mutations) | PASS |
| r00165 | admin | /orders/new | mobile | zh-TW | main | 回到訂單 | click | navigation or in-page change | scrolled | PASS |
| r00166 | admin | /orders | desktop | en | main | Refresh (`orders-refresh`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00167 | admin | / | mobile | zh-TW | layer of 帳號 | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00168 | admin | /orders/new | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00169 | admin | / | mobile | zh-TW | layer of 帳號 | 登出 (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00170 | admin | /orders | desktop | en | main | Export unshipped (CSV) (`orders-export`) | click | navigation or in-page change | download: unshipped-9b3cec4a-202610040715.csv | PASS |
| r00171 | admin | /orders/new | desktop | en | main | Overview | click | navigation or in-page change | url /en/orders/new?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00172 | admin | /orders | desktop | en | main | Payment method (`orders-payment-filter`) | selectOption(card) | visible change | the control's own state changed ( -> card) | PASS |
| r00173 | admin | /orders/new | desktop | en | main | Search (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00174 | admin | / | mobile | zh-TW | rail | 關閉導覽 | click | visible change | DOM changed (3 mutations) | PASS |
| r00175 | admin | /orders | desktop | en | main | Delivery method (`orders-delivery-filter`) | selectOption(home) | visible change | the control's own state changed ( -> home) | PASS |
| r00176 | admin | /orders/new | desktop | en | main | Delivery method (`mo-option`) | selectOption(e0a73ab5-a2c7-4b18-a04a-f383d3b230ab\|TW\|cvs6f2ff444cf3f) | visible change | DOM changed (4 mutations); the control's own state changed ( -> e0a73ab5-a2c7-4b18-a04a-f383d3b230ab\|TW\|cvs6f2ff444cf3f) | PASS |
| r00177 | admin | /orders | desktop | en | main | Live session (`orders-session-filter`) | selectOption | visible change | select has a single option | SKIP |
| r00178 | admin | / | mobile | zh-TW | rail | 總覽 (`nav-group-overview`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00179 | admin | /orders/new | desktop | en | main | Customer link | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN) | PASS |
| r00180 | admin | /orders | desktop | en | main | Apply filters (`orders-apply`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00181 | admin | /orders/new | desktop | en | main | Back to orders | click | navigation or in-page change | url /en/orders/new?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00182 | admin | / | mobile | zh-TW | rail | 直播與貼文 (`nav-group-live`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00183 | admin | /orders | desktop | en | main | Clear filters (`orders-reset`) | click | visible change | DOM changed (11 mutations); the control's own state changed ( -> gone) | PASS |
| r00184 | admin | /orders/cvs-print | desktop | zh-TW | page | (page load) | - | the route hands the browser to /ecpay/i | HTTP 200; POST https://logistics-stage.ecpay.com.tw/Express/PrintUniMartC2COrderInfo (blocked by the sweep) | PASS |
| r00185 | admin | / | mobile | zh-TW | rail | 訂單與出貨 (`nav-orders`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00186 | admin | /orders/cvs-print | mobile | zh-TW | page | (page load) | - | the route hands the browser to /ecpay/i | HTTP 200; POST https://logistics-stage.ecpay.com.tw/Express/PrintUniMartC2COrderInfo (blocked by the sweep) | PASS |
| r00187 | admin | /orders/cvs-print | desktop | en | page | (page load) | - | the route hands the browser to /ecpay/i | HTTP 200; POST https://logistics-stage.ecpay.com.tw/Express/PrintUniMartC2COrderInfo (blocked by the sweep) | PASS |
| r00188 | admin | /orders | desktop | en | main | All 5 (`orders-bucket-all`) | click | visible change | DOM changed (11 mutations); the control's own state changed (\|\|\|\|true -> gone) | PASS |
| r00189 | admin | /products | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00190 | admin | / | mobile | zh-TW | rail | 商品與庫存 + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00191 | admin | /orders | desktop | en | main | Awaiting payment 4 (`orders-bucket-unpaid`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00192 | admin | /products | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00193 | admin | / | mobile | zh-TW | rail | 顧客 (`nav-group-customers`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00194 | admin | /orders | desktop | en | main | Review transfers 1 (`orders-bucket-transfer_review`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00195 | admin | /products | desktop | zh-TW | main | 庫存帳 (`products-ledger-link`) | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (16 mutations) | PASS |
| r00196 | admin | /orders | desktop | en | main | Ready to ship 2 (`orders-bucket-ready_to_ship`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00197 | admin | / | mobile | zh-TW | rail | 行銷優惠 + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00198 | admin | /products | desktop | zh-TW | main | 新增商品 (`product-new`) | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products/new?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (19 mutations) | PASS |
| r00199 | admin | /orders | desktop | en | main | Ready to drop off 1 (`orders-bucket-ready_to_consign`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00200 | admin | /products | desktop | zh-TW | main | 全部 3 (`products-tab-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00201 | admin | / | mobile | zh-TW | rail | 網路商店 (`nav-group-storefront`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00202 | admin | /orders | desktop | en | main | Shipped 0 (`orders-bucket-shipped`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00203 | admin | /products | desktop | zh-TW | main | 草稿 0 (`products-tab-draft`) | click | visible change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&status=draft; DOM changed (17 mutat | PASS |
| r00204 | admin | / | mobile | zh-TW | rail | 收款與報表 (`nav-group-finance`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00205 | admin | /orders | desktop | en | main | Completed 0 (`orders-bucket-completed`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00206 | admin | /products | desktop | zh-TW | main | 上架中 3 (`products-tab-active`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&status=active; DOM changed (17 muta (inline change only; no request fired) | PASS |
| r00207 | admin | / | mobile | zh-TW | rail | 設定 + (`nav-group-settings`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00208 | admin | /orders | desktop | en | main | Cancelled 0 (`orders-bucket-cancelled`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00209 | admin | /products | desktop | zh-TW | main | 已封存 0 (`products-tab-archived`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&status=archived; DOM changed (17 mu (inline change only; no request fired) | PASS |
| r00210 | admin | /orders | desktop | en | main | Show order details: a5dcaa60-34ff-4d3f-9f39-d18b4f5d8c78 (`order-expand-a5dcaa60-34ff-4d3f-9f39-d18b4f5d8c78`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00211 | admin | / | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | DOM changed (10 mutations) | PASS |
| r00212 | admin | /products | desktop | zh-TW | main | 搜尋 (`products-search-submit`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00213 | admin | /orders | desktop | en | main | Show order details: b540064f-4d30-4615-b811-d047125a7ebb (`order-expand-b540064f-4d30-4615-b811-d047125a7ebb`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00214 | admin | / | mobile | zh-TW | main | 待確認的轉帳 1 (`todo-transfer`) | click | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?state=AWAITING_TRANSFER&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (14 mutat | PASS |
| r00215 | admin | /products | desktop | zh-TW | main | 全部 | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00216 | admin | /orders | desktop | en | main | Show order details: 1a450fd2-9556-46c2-9e78-de30ec74d01b (`order-expand-1a450fd2-9556-46c2-9e78-de30ec74d01b`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00217 | admin | /products | desktop | zh-TW | main | 選取商品: Sweep Ceramic Mug | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00218 | admin | / | mobile | zh-TW | main | 待出貨訂單 2 (`todo-ship`) | click | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?state=unshipped&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (14 mutations) | PASS |
| r00219 | admin | /products/[product] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00220 | admin | /products | desktop | zh-TW | main | 編輯: Sweep Ceramic Mug | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products/c11baa6b-8771-49b3-bbf2-2376d9c9b578?store=9b3cec4a-09ba-4b8e-90f1-3393a7d4918 | PASS |
| r00221 | admin | / | mobile | zh-TW | main | 待建立的超商托運單 1 (`todo-label`) | click | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?state=unshipped&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (14 mutations) | PASS |
| r00222 | admin | /products/[product] | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/b3bea372-17f5-4d97-a542-fe223efc6de7?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM ch | PASS |
| r00223 | admin | /products | desktop | zh-TW | main | 修改價格: Sweep Ceramic Mug (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00224 | admin | / | mobile | zh-TW | main | 低庫存規格（5 件以下） 0 (`todo-stock`) | click | navigation or in-page change | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (16 mutations) | PASS |
| r00225 | admin | /products/[product] | desktop | zh-TW | main | ← 全部商品 (`product-back`) | click | navigation or in-page change | url /zh-TW/products/b3bea372-17f5-4d97-a542-fe223efc6de7?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d4918 | PASS |
| r00226 | admin | /products | desktop | zh-TW | main | 修改庫存: Sweep Ceramic Mug (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00227 | admin | / | mobile | zh-TW | main | 處理中的退款 0 (`todo-refunds`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (14 mutations) (inline change only; no request fired) | PASS |
| r00228 | admin | /products/[product] | desktop | zh-TW | main | 商品圖片 | click | visible change | scrolled | PASS |
| r00229 | admin | /products | desktop | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products/c11baa6b-8771-49b3-bbf2-2376d9c9b578?store=9b3cec4a-09ba-4b8e-90f1-3393a7d4918 [1 of 3 alike] | PASS |
| r00230 | admin | / | mobile | zh-TW | main | a5dcaa60 | click | navigation or in-page change | scrolled | PASS |
| r00231 | admin | /products/[product] | desktop | zh-TW | main | 基本資訊 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00232 | admin | /products | desktop | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00233 | admin | / | mobile | zh-TW | main | b540064f | click | navigation or in-page change | scrolled | PASS |
| r00234 | admin | /products/[product] | desktop | zh-TW | main | 價格與庫存 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00235 | admin | /products | desktop | zh-TW | main | 選取商品: Sweep Wool Scarf | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00236 | admin | / | mobile | zh-TW | main | f5126af7 | click | navigation or in-page change | scrolled | PASS |
| r00237 | admin | /products/[product] | desktop | zh-TW | main | 規格 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00238 | admin | /products | desktop | zh-TW | main | 編輯: Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products/c5939f55-dab7-400d-b754-532b31bf52e0?store=9b3cec4a-09ba-4b8e-90f1-3393a7d4918 | PASS |
| r00239 | admin | / | mobile | zh-TW | main | 5cfe749c | click | navigation or in-page change | scrolled | PASS |
| r00240 | admin | /products/[product] | desktop | zh-TW | main | 分類 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00241 | admin | /products | desktop | zh-TW | main | 修改價格: Sweep Wool Scarf (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00242 | admin | / | mobile | zh-TW | main | 1a450fd2 | click | navigation or in-page change | scrolled | PASS |
| r00243 | admin | /products/[product] | desktop | zh-TW | main | 物流 | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00244 | admin | /products | desktop | zh-TW | main | 修改庫存: Sweep Wool Scarf (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00245 | admin | / | mobile | zh-TW | main | 113e5295 | click | navigation or in-page change | scrolled | PASS |
| r00246 | admin | /products/[product] | desktop | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00247 | admin | /products | desktop | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products/c5939f55-dab7-400d-b754-532b31bf52e0?store=9b3cec4a-09ba-4b8e-90f1-3393a7d4918 [1 of 3 alike] | PASS |
| r00248 | admin | / | mobile | zh-TW | main | 全部訂單 | click | navigation or in-page change | scrolled | PASS |
| r00249 | admin | /products/[product] | desktop | zh-TW | main | 圖片 ○ | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00250 | admin | /products | desktop | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00251 | admin | / | mobile | zh-TW | main | 建立訂單 (`action-create-order`) | click | navigation or in-page change | scrolled | PASS |
| r00252 | admin | /products/[product] | desktop | zh-TW | main | 商品名稱 ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00253 | admin | /products | desktop | zh-TW | main | 選取商品: Sweep Cedar Candle | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00254 | admin | / | mobile | zh-TW | main | 匯入或匯出商品 (`action-import`) | click | navigation or in-page change | scrolled | PASS |
| r00255 | admin | /products/[product] | desktop | zh-TW | main | 售價 ○ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00256 | admin | /products | desktop | zh-TW | main | 編輯: Sweep Cedar Candle | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products/b3bea372-17f5-4d97-a542-fe223efc6de7?store=9b3cec4a-09ba-4b8e-90f1-3393a7d4918 | PASS |
| r00257 | admin | / | mobile | zh-TW | main | 庫存 (`action-inventory`) | click | navigation or in-page change | scrolled | PASS |
| r00258 | admin | /products/[product] | desktop | zh-TW | main | 庫存 ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00259 | admin | / | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00260 | admin | /products | desktop | zh-TW | main | 修改價格: Sweep Cedar Candle (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00261 | admin | /products/[product] | desktop | zh-TW | main | 圖片 ○ | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00262 | admin | /products | desktop | zh-TW | main | 修改庫存: Sweep Cedar Candle (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00263 | admin | /products/[product] | desktop | zh-TW | main | 描述 ✓ | click | visible change | scrolled | PASS |
| r00264 | admin | /products | desktop | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products/b3bea372-17f5-4d97-a542-fe223efc6de7?store=9b3cec4a-09ba-4b8e-90f1-3393a7d4918 [1 of 3 alike] | PASS |
| r00265 | admin | /products/[product] | desktop | zh-TW | main | 分類 ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00266 | admin | /products | desktop | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00267 | admin | /products/[product] | desktop | zh-TW | main | 直播關鍵字 ○ | click | visible change | scrolled | PASS |
| r00268 | admin | /products | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00269 | admin | /products/[product] | desktop | zh-TW | main | 搜尋引擎最佳化 ○ | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00270 | admin | /products | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00271 | admin | /products/[product] | desktop | zh-TW | main | 顯示狀態上架中：商店發布後，買家可以看到並購買。 (`product-status`) | selectOption(draft) | confirmation layer opens; cancel; nothing changed | DOM changed (2 mutations); the control's own state changed (active -> draft) (inline change only; no request fired) | PASS |
| r00272 | admin | /products | mobile | zh-TW | main | 庫存帳 (`products-ledger-link`) | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (16 mutations) | PASS |
| r00273 | admin | /products/[product] | desktop | zh-TW | main | 移除 規格名稱 1 | click | confirmation layer opens; cancel; nothing changed | DOM changed (10 mutations); the control's own state changed ( -> true); scrolled (inline change only; no request fired) | PASS |
| r00274 | admin | /products | mobile | zh-TW | main | 新增商品 (`product-new`) | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products/new?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (19 mutations) | PASS |
| r00275 | admin | / | desktop | en | skip | Skip to content | keyboard: Tab to the link, Enter | navigation or in-page change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180#main; scrolled | PASS |
| r00276 | admin | /products/[product] | desktop | zh-TW | main | 新增規格（顏色、尺寸…） (`axis-add`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00277 | admin | /products | mobile | zh-TW | main | 全部 3 (`products-tab-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00278 | admin | / | desktop | en | topbar | Switch store (`shell-store-selector`) | selectOption | visible change | select has a single option | SKIP |
| r00279 | admin | /products/[product] | desktop | zh-TW | main | 售價 · 批量填入 (`bulk-price`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00280 | admin | /products | mobile | zh-TW | main | 草稿 0 (`products-tab-draft`) | click | visible change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&status=draft; DOM changed (17 mutat | PASS |
| r00281 | admin | / | desktop | en | topbar | Language (`locale-switch`) | selectOption(zh-CN) | visible change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-CN?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (6 mutations) | PASS |
| r00282 | admin | /products/[product] | desktop | zh-TW | main | 原價 · 批量填入 (`bulk-compare`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00283 | admin | / | desktop | en | topbar | Help | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00284 | admin | /products | mobile | zh-TW | main | 上架中 3 (`products-tab-active`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&status=active; DOM changed (17 muta (inline change only; no request fired) | PASS |
| r00285 | admin | /products/[product] | desktop | zh-TW | main | 數量 · 批量填入 (`bulk-quantity`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00286 | admin | /products | mobile | zh-TW | main | 已封存 0 (`products-tab-archived`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&status=archived; DOM changed (17 mu (inline change only; no request fired) | PASS |
| r00287 | admin | / | desktop | en | layer of Help | Help | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00288 | admin | /products/[product] | desktop | zh-TW | main | 貨號 · 批量填入 (`bulk-code`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00289 | admin | / | desktop | en | topbar | Account | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00290 | admin | /products | mobile | zh-TW | main | 搜尋 (`products-search-submit`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00291 | admin | /products/[product] | desktop | zh-TW | main | 直播關鍵字 · 批量填入 (`bulk-keyword`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00292 | admin | /products | mobile | zh-TW | main | 全部 | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00293 | admin | / | desktop | en | layer of Account | Account | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00294 | admin | /products/[product] | desktop | zh-TW | main | 不追蹤 ∞ | click (toggle) | visible change | DOM changed (5 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00295 | admin | / | desktop | en | layer of Account | Sign out (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00296 | admin | /products | mobile | zh-TW | main | 選取商品: Sweep Ceramic Mug | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00297 | admin | /products/[product] | desktop | zh-TW | main | 啟用 1 (`matrix-active-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00298 | admin | /products | mobile | zh-TW | main | 編輯: Sweep Ceramic Mug | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products/c11baa6b-8771-49b3-bbf2-2376d9c9b578?store=9b3cec4a-09ba-4b8e-90f1-3393a7d4918 | PASS |
| r00299 | admin | / | desktop | en | rail | Overview (`nav-group-overview`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00300 | admin | /products/[product] | desktop | zh-TW | main | Sweep home | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00301 | admin | /products | mobile | zh-TW | main | 修改價格: Sweep Ceramic Mug (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00302 | admin | / | desktop | en | rail | Live & posts (`nav-group-live`) | click | visible change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/studio?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (17 mutations) | PASS |
| r00303 | admin | /products/[product] | desktop | zh-TW | main | Sweep wear | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00304 | admin | /products | mobile | zh-TW | main | 修改庫存: Sweep Ceramic Mug (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00305 | admin | / | desktop | en | rail | Orders & shipping (`nav-orders`) | click | visible change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00306 | admin | /products/[product] | desktop | zh-TW | main | 管理分類 | click | navigation or in-page change | scrolled | PASS |
| r00307 | admin | /products | mobile | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products/c11baa6b-8771-49b3-bbf2-2376d9c9b578?store=9b3cec4a-09ba-4b8e-90f1-3393a7d4918 [1 of 3 alike] | PASS |
| r00308 | admin | / | desktop | en | rail | Products & inventory + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00309 | admin | /products/[product] | desktop | zh-TW | main | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00310 | admin | /products | mobile | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00311 | admin | / | desktop | en | rail | Customers (`nav-group-customers`) | click | visible change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00312 | admin | /products/[product] | desktop | zh-TW | layer of 物流 | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00313 | admin | /products | mobile | zh-TW | main | 選取商品: Sweep Wool Scarf | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ); scrolled | PASS |
| r00314 | admin | / | desktop | en | rail | Marketing + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00315 | admin | /products/[product] | desktop | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00316 | admin | /products | mobile | zh-TW | main | 編輯: Sweep Wool Scarf | click | navigation or in-page change | scrolled | PASS |
| r00317 | admin | / | desktop | en | rail | Online store (`nav-group-storefront`) | click | visible change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/design?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00318 | admin | /products/[product] | desktop | zh-TW | layer of 搜尋引擎最佳化 | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00319 | admin | /products | mobile | zh-TW | main | 修改價格: Sweep Wool Scarf (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations); scrolled | PASS |
| r00320 | admin | / | desktop | en | rail | Payments & reports (`nav-group-finance`) | click | visible change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/finance?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (16 mutations) | PASS |
| r00321 | admin | /products/[product] | desktop | zh-TW | main | 儲存修改 (`product-save`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00322 | admin | /products | mobile | zh-TW | main | 修改庫存: Sweep Wool Scarf (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations); scrolled | PASS |
| r00323 | admin | / | desktop | en | rail | Settings + (`nav-group-settings`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00324 | admin | /products/[product] | desktop | zh-TW | main | 下架轉草稿 (`product-unpublish`) | click | confirmation layer opens; cancel; nothing changed | dialog: confirm: 確定下架此商品？買家將無法再購買此商品。 (confirmation shown, then cancelled) | PASS |
| r00325 | admin | /products | mobile | zh-TW | main | 編輯 | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00326 | admin | / | desktop | en | main | Overview | click | navigation or in-page change | DOM changed (10 mutations) | PASS |
| r00327 | admin | /products | mobile | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations); scrolled [1 of 3 alike] | PASS |
| r00328 | admin | /products/[product] | desktop | zh-TW | main | 上架 (`product-publish`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (1 mutations) (inline change only; no request fired) | PASS |
| r00329 | admin | /products/[product] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00330 | admin | / | desktop | en | main | Transfers to confirm 1 (`todo-transfer`) | click | navigation or in-page change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/orders?state=AWAITING_TRANSFER&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (19 mutations) | PASS |
| r00331 | admin | /products | mobile | zh-TW | main | 選取商品: Sweep Cedar Candle | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ); scrolled | PASS |
| r00332 | admin | /products/[product] | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/b3bea372-17f5-4d97-a542-fe223efc6de7?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM ch | PASS |
| r00333 | admin | / | desktop | en | main | Orders to ship 2 (`todo-ship`) | click | navigation or in-page change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/orders?state=unshipped&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (19 mutations) | PASS |
| r00334 | admin | /products | mobile | zh-TW | main | 編輯: Sweep Cedar Candle | click | navigation or in-page change | scrolled | PASS |
| r00335 | admin | /products/[product] | mobile | zh-TW | main | ← 全部商品 (`product-back`) | click | navigation or in-page change | url /zh-TW/products/b3bea372-17f5-4d97-a542-fe223efc6de7?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d4918 | PASS |
| r00336 | admin | /products | mobile | zh-TW | main | 修改價格: Sweep Cedar Candle (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations); scrolled | PASS |
| r00337 | admin | / | desktop | en | main | Convenience-store labels to create 1 (`todo-label`) | click | navigation or in-page change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/orders?state=unshipped&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00338 | admin | /products/[product] | mobile | zh-TW | main | 商品圖片 | click | visible change | scrolled | PASS |
| r00339 | admin | /products | mobile | zh-TW | main | 修改庫存: Sweep Cedar Candle (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations); scrolled | PASS |
| r00340 | admin | / | desktop | en | main | Low-stock variants (5 or fewer) 0 (`todo-stock`) | click | navigation or in-page change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (16 mutations) | PASS |
| r00341 | admin | /products/[product] | mobile | zh-TW | main | 基本資訊 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00342 | admin | /products | mobile | zh-TW | main | 編輯 | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00343 | admin | / | desktop | en | main | Open refunds 0 (`todo-refunds`) | click | navigation or in-page change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00344 | admin | /products/[product] | mobile | zh-TW | main | 價格與庫存 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00345 | admin | /products | mobile | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations); scrolled [1 of 3 alike] | PASS |
| r00346 | admin | / | desktop | en | main | a5dcaa60 | click | navigation or in-page change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/orders?order=a5dcaa60-34ff-4d3f-9f39-d18b4f5d8c78&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM chan | PASS |
| r00347 | admin | /products/[product] | mobile | zh-TW | main | 規格 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00348 | admin | /products | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00349 | admin | / | desktop | en | main | b540064f | click | navigation or in-page change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/orders?order=b540064f-4d30-4615-b811-d047125a7ebb&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM chan | PASS |
| r00350 | admin | /products | desktop | en | main | Overview | click | navigation or in-page change | url /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00351 | admin | /products/[product] | mobile | zh-TW | main | 分類 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00352 | admin | / | desktop | en | main | f5126af7 | click | navigation or in-page change | url /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/orders?order=f5126af7-b7aa-4ee3-a721-a2852a961df9&store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM chan | PASS |
| r00353 | admin | /products | desktop | en | main | Inventory ledger (`products-ledger-link`) | click | navigation or in-page change | url /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (16 mutations) | PASS |
| r00354 | admin | /products/[product] | mobile | zh-TW | main | 物流 | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00355 | admin | / | desktop | en | main | 5cfe749c | click | navigation or in-page change | scrolled | PASS |
| r00356 | admin | /products/[product] | mobile | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00357 | admin | /products | desktop | en | main | Add product (`product-new`) | click | navigation or in-page change | url /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/products/new?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (19 mutations) | PASS |
| r00358 | admin | /products/[product] | mobile | zh-TW | main | 顯示狀態上架中：商店發布後，買家可以看到並購買。 (`product-status`) | selectOption(draft) | confirmation layer opens; cancel; nothing changed | DOM changed (2 mutations); the control's own state changed (active -> draft) (inline change only; no request fired) | PASS |
| r00359 | admin | / | desktop | en | main | 1a450fd2 | click | navigation or in-page change | scrolled | PASS |
| r00360 | admin | /products | desktop | en | main | All 3 (`products-tab-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00361 | admin | / | desktop | en | main | 113e5295 | click | navigation or in-page change | scrolled | PASS |
| r00362 | admin | /products/[product] | mobile | zh-TW | main | 移除 規格名稱 1 | click | confirmation layer opens; cancel; nothing changed | DOM changed (10 mutations); the control's own state changed ( -> true); scrolled (inline change only; no request fired) | PASS |
| r00363 | admin | /products | desktop | en | main | Draft 0 (`products-tab-draft`) | click | visible change | url /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&status=draft; DOM changed (17 mutations) | PASS |
| r00364 | admin | / | desktop | en | main | All orders | click | navigation or in-page change | scrolled | PASS |
| r00365 | admin | /products/[product] | mobile | zh-TW | main | 新增規格（顏色、尺寸…） (`axis-add`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00366 | admin | /products | desktop | en | main | Active 3 (`products-tab-active`) | click | visible change | url /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&status=active; DOM changed (17 mutations) | PASS |
| r00367 | admin | / | desktop | en | main | Create an order (`action-create-order`) | click | navigation or in-page change | scrolled | PASS |
| r00368 | admin | /products/[product] | mobile | zh-TW | main | 售價 · 批量填入 (`bulk-price`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00369 | admin | /products | desktop | en | main | Archived 0 (`products-tab-archived`) | click | visible change | url /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&status=archived; DOM changed (17 mutation | PASS |
| r00370 | admin | / | desktop | en | main | Import or export products (`action-import`) | click | navigation or in-page change | scrolled | PASS |
| r00371 | admin | /products/[product] | mobile | zh-TW | main | 原價 · 批量填入 (`bulk-compare`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00372 | admin | /products | desktop | en | main | Search (`products-search-submit`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00373 | admin | / | desktop | en | main | Inventory (`action-inventory`) | click | navigation or in-page change | scrolled | PASS |
| r00374 | admin | /products/[product] | mobile | zh-TW | main | 數量 · 批量填入 (`bulk-quantity`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00375 | admin | /products | desktop | en | main | All | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00376 | admin | /collections | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00377 | admin | /products/[product] | mobile | zh-TW | main | 貨號 · 批量填入 (`bulk-code`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00378 | admin | /collections | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/collections?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00379 | admin | /products | desktop | en | main | Select product: Sweep Ceramic Mug | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00380 | admin | /products/[product] | mobile | zh-TW | main | 直播關鍵字 · 批量填入 (`bulk-keyword`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00381 | admin | /collections | desktop | zh-TW | main | 新增集合 (`collection-new`) | click | visible change | DOM changed (3 mutations); validation shown | PASS |
| r00382 | admin | /products | desktop | en | main | Edit: Sweep Ceramic Mug | click | navigation or in-page change | url /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/products/c11baa6b-8771-49b3-bbf2-2376d9c9b578?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM | PASS |
| r00383 | admin | /products/[product] | mobile | zh-TW | main | 不追蹤 ∞ | click (toggle) | visible change | DOM changed (5 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00384 | admin | /collections | desktop | zh-TW | main | Sweep home /sweep-home · 2 件商品 (`collection-item-38ae85ed-a581-4202-87c6-5aff513a6d0e`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00385 | admin | /products | desktop | en | main | Edit price: Sweep Ceramic Mug (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00386 | admin | /products/[product] | mobile | zh-TW | main | 啟用 1 (`matrix-active-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00387 | admin | /collections | desktop | zh-TW | main | Sweep wear /sweep-wear · 2 件商品 (`collection-item-61394e46-610a-485e-9e16-883c9bb8fb94`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00388 | admin | /products | desktop | en | main | Edit inventory: Sweep Ceramic Mug (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00389 | admin | /products/[product] | mobile | zh-TW | main | Sweep home | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00390 | admin | /collections | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00391 | admin | /products | desktop | en | main | Edit | click | navigation or in-page change | url /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/products/c11baa6b-8771-49b3-bbf2-2376d9c9b578?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM [1 of 3 alike] | PASS |
| r00392 | admin | /products/[product] | mobile | zh-TW | main | Sweep wear | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00393 | admin | /collections | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/collections?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00394 | admin | /products | desktop | en | main | Copy | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00395 | admin | /collections | mobile | zh-TW | main | 新增集合 (`collection-new`) | click | visible change | DOM changed (3 mutations); validation shown | PASS |
| r00396 | admin | /products/[product] | mobile | zh-TW | main | 管理分類 | click | navigation or in-page change | scrolled | PASS |
| r00397 | admin | /products | desktop | en | main | Select product: Sweep Wool Scarf | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00398 | admin | /collections | mobile | zh-TW | main | Sweep home /sweep-home · 2 件商品 (`collection-item-38ae85ed-a581-4202-87c6-5aff513a6d0e`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00399 | admin | /products/[product] | mobile | zh-TW | main | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00400 | admin | /products | desktop | en | main | Edit: Sweep Wool Scarf | click | navigation or in-page change | url /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/products/c5939f55-dab7-400d-b754-532b31bf52e0?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM | PASS |
| r00401 | admin | /collections | mobile | zh-TW | main | Sweep wear /sweep-wear · 2 件商品 (`collection-item-61394e46-610a-485e-9e16-883c9bb8fb94`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00402 | admin | /products/[product] | mobile | zh-TW | layer of 物流 | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00403 | admin | /products | desktop | en | main | Edit price: Sweep Wool Scarf (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00404 | admin | /collections | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00405 | admin | /products/[product] | mobile | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00406 | admin | /collections | desktop | en | main | Overview | click | navigation or in-page change | url /en/collections?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00407 | admin | /products | desktop | en | main | Edit inventory: Sweep Wool Scarf (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00408 | admin | /collections | desktop | en | main | New collection (`collection-new`) | click | visible change | DOM changed (3 mutations); validation shown | PASS |
| r00409 | admin | /products/[product] | mobile | zh-TW | layer of 搜尋引擎最佳化 | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00410 | admin | /products | desktop | en | main | Edit | click | navigation or in-page change | url /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/products/c5939f55-dab7-400d-b754-532b31bf52e0?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM [1 of 3 alike] | PASS |
| r00411 | admin | /collections | desktop | en | main | Sweep home /sweep-home · 2 products (`collection-item-38ae85ed-a581-4202-87c6-5aff513a6d0e`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00412 | admin | /products/[product] | mobile | zh-TW | main | 儲存修改 (`product-save`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00413 | admin | /products | desktop | en | main | Copy | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00414 | admin | /collections | desktop | en | main | Sweep wear /sweep-wear · 2 products (`collection-item-61394e46-610a-485e-9e16-883c9bb8fb94`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00415 | admin | /products/[product] | mobile | zh-TW | main | 下架轉草稿 (`product-unpublish`) | click | confirmation layer opens; cancel; nothing changed | dialog: confirm: 確定下架此商品？買家將無法再購買此商品。 (confirmation shown, then cancelled) | PASS |
| r00416 | admin | /products | desktop | en | main | Select product: Sweep Cedar Candle | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00417 | admin | /inventory | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00418 | admin | /products/[product] | mobile | zh-TW | main | 上架 (`product-publish`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (1 mutations) (inline change only; no request fired) | PASS |
| r00419 | admin | /products | desktop | en | main | Edit: Sweep Cedar Candle | click | navigation or in-page change | url /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/products/b3bea372-17f5-4d97-a542-fe223efc6de7?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM | PASS |
| r00420 | admin | /inventory | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00421 | admin | /products/[product] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00422 | admin | /products | desktop | en | main | Edit price: Sweep Cedar Candle (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00423 | admin | /products | desktop | en | layer of Edit price: Sweep Cedar Candle | Save changes (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00424 | admin | /products/[product] | desktop | en | main | Overview | click | navigation or in-page change | url /en/products/b3bea372-17f5-4d97-a542-fe223efc6de7?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed  | PASS |
| r00425 | admin | /inventory | desktop | zh-TW | main | 新增商品 | click | navigation or in-page change | url /zh-TW/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products/new?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (19 mutations) | PASS |
| r00426 | admin | /products | desktop | en | main | Edit inventory: Sweep Cedar Candle (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (33 mutations) | PASS |
| r00427 | admin | /products/[product] | desktop | en | main | ← All products (`product-back`) | click | navigation or in-page change | url /en/products/b3bea372-17f5-4d97-a542-fe223efc6de7?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM | PASS |
| r00428 | admin | /inventory | desktop | zh-TW | main | 查詢 | click | visible change | DOM changed (2 mutations) | PASS |
| r00429 | admin | /inventory | desktop | zh-TW | main | 倉庫 | selectOption | visible change | select has a single option | SKIP |
| r00430 | admin | /products | desktop | en | layer of Edit inventory: Sweep Cedar Candle | Do not track ∞ (`quick-untracked-0`) | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (false -> true) | PASS |
| r00431 | admin | /products | desktop | en | layer of Edit inventory: Sweep Cedar Candle | Save changes (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00432 | admin | /products/[product] | desktop | en | main | Product images | click | visible change | scrolled | PASS |
| r00433 | admin | /inventory | desktop | zh-TW | main | 狀態 | selectOption(active) | visible change | url /zh-TW/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&status=active; DOM changed (2 mut | PASS |
| r00434 | admin | /products | desktop | en | main | Edit | click | navigation or in-page change | url /en/products?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/products/b3bea372-17f5-4d97-a542-fe223efc6de7?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM [1 of 3 alike] | PASS |
| r00435 | admin | /products/[product] | desktop | en | main | Basic information | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00436 | admin | /inventory | desktop | zh-TW | main | 重設 | click | visible change | DOM changed (2 mutations) | PASS |
| r00437 | admin | /products/[product] | desktop | en | main | Price & inventory | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00438 | admin | /products | desktop | en | main | Copy | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00439 | admin | /inventory | desktop | zh-TW | main | 重新整理 | click | visible change | DOM changed (2 mutations) | PASS |
| r00440 | admin | /products/import | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00441 | admin | /products/[product] | desktop | en | main | Variants | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00442 | admin | /products/import | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/import?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00443 | admin | /inventory | desktop | zh-TW | main | 選取 SWP-MUG | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00444 | admin | /products/[product] | desktop | en | main | Collections | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00445 | admin | /products/import | desktop | zh-TW | main | 下載 CSV (`import-export`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00446 | admin | /inventory | desktop | zh-TW | main | Sweep Ceramic Mug | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00447 | admin | /products/[product] | desktop | en | main | Shipping | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00448 | admin | /products/import | desktop | zh-TW | main | 回到總覽 | click | navigation or in-page change | url /zh-TW/products/import?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00449 | admin | /inventory | desktop | zh-TW | main | 選取 | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00450 | admin | /products/[product] | desktop | en | main | Search engine listing | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00451 | admin | /products/import | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00452 | admin | /inventory | desktop | zh-TW | main | 選取 T04-24a21ca87d63-0 | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00453 | admin | /products/[product] | desktop | en | main | Images ○ | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00454 | admin | /products/import | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/import?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00455 | admin | /inventory | desktop | zh-TW | main | Sweep Cedar Candle | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00456 | admin | /products/import | mobile | zh-TW | main | 下載 CSV (`import-export`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00457 | admin | /products/[product] | desktop | en | main | Product name ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00458 | admin | /inventory | desktop | zh-TW | main | 選取 | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00459 | admin | /products/import | mobile | zh-TW | main | 回到總覽 | click | navigation or in-page change | scrolled | PASS |
| r00460 | admin | /products/[product] | desktop | en | main | Price ○ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00461 | admin | /inventory | desktop | zh-TW | main | 選取 SWP-SCF | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00462 | admin | /products/import | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00463 | admin | /products/[product] | desktop | en | main | Inventory ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00464 | admin | /products/import | desktop | en | main | Overview | click | navigation or in-page change | url /en/products/import?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00465 | admin | /inventory | desktop | zh-TW | main | Sweep Wool Scarf | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00466 | admin | /products/[product] | desktop | en | main | Images ○ | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00467 | admin | /products/import | desktop | en | main | Download CSV (`import-export`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00468 | admin | /inventory | desktop | zh-TW | main | 選取 | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00469 | admin | /products/[product] | desktop | en | main | Description ✓ | click | visible change | scrolled | PASS |
| r00470 | admin | /products/import | desktop | en | main | Back to dashboard | click | navigation or in-page change | url /en/products/import?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00471 | admin | /inventory | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00472 | admin | /products/[product] | desktop | en | main | Collections ✓ | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00473 | admin | /inventory | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00474 | admin | /customers | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00475 | admin | /customers | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00476 | admin | /products/[product] | desktop | en | main | Live keyword ○ | click | visible change | scrolled | PASS |
| r00477 | admin | /inventory | mobile | zh-TW | main | 新增商品 | click | navigation or in-page change | url /zh-TW/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/products/new?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (19 mutations) | PASS |
| r00478 | admin | /customers | desktop | zh-TW | main | 搜尋 (`customers-search-submit`) | click | visible change | DOM changed (12 mutations) | PASS |
| r00479 | admin | /products/[product] | desktop | en | main | Search engine listing ○ | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00480 | admin | /inventory | mobile | zh-TW | main | 查詢 | click | visible change | DOM changed (2 mutations) | PASS |
| r00481 | admin | /products/[product] | desktop | en | main | VisibilityActive: shoppers can see and buy it once the store is published. (`product-status`) | selectOption(draft) | visible change | DOM changed (2 mutations); the control's own state changed (active -> draft) | PASS |
| r00482 | admin | /customers | desktop | zh-TW | main | 重新整理 (`customers-refresh`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00483 | admin | /inventory | mobile | zh-TW | main | 倉庫 | selectOption | visible change | select has a single option | SKIP |
| r00484 | admin | /inventory | mobile | zh-TW | main | 狀態 | selectOption(active) | visible change | url /zh-TW/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&status=active; DOM changed (2 mut | PASS |
| r00485 | admin | /products/[product] | desktop | en | main | Remove Option name 1 | click | confirmation layer opens; cancel; nothing changed | DOM changed (10 mutations); the control's own state changed ( -> true); scrolled (inline change only; no request fired) | PASS |
| r00486 | admin | /customers | desktop | zh-TW | main | 開啟客戶: 王小明 (`customer-open-e2f26bae-8749-4104-9813-2872648a967b`) | click | navigation or in-page change | url /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/customers/e2f26bae-8749-4104-9813-2872648a967b?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49 [1 of 3 alike] | PASS |
| r00487 | admin | /inventory | mobile | zh-TW | main | 重設 | click | visible change | DOM changed (2 mutations) | PASS |
| r00488 | admin | /customers | desktop | zh-TW | main | 開啟客戶: 王小明 (`customer-open-97ea14b3-8819-4403-b647-d5891d42aebd`) | click | navigation or in-page change | url /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/customers/97ea14b3-8819-4403-b647-d5891d42aebd?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49 [1 of 3 alike] | PASS |
| r00489 | admin | /products/[product] | desktop | en | main | Add option (color, size…) (`axis-add`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00490 | admin | /inventory | mobile | zh-TW | main | 重新整理 | click | visible change | DOM changed (2 mutations) | PASS |
| r00491 | admin | /customers | desktop | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-b5940f80-a3a0-496f-ae29-4dbd207047d2`) | click | navigation or in-page change | url /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/customers/b5940f80-a3a0-496f-ae29-4dbd207047d2?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49 [1 of 3 alike] | PASS |
| r00492 | admin | /products/[product] | desktop | en | main | Price · Bulk fill (`bulk-price`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00493 | admin | /inventory | mobile | zh-TW | main | 選取 SWP-MUG | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00494 | admin | /customers | desktop | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-4b1bcce9-fe52-4b2f-878d-91daa943f0a1`) | click | navigation or in-page change | url /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/customers/4b1bcce9-fe52-4b2f-878d-91daa943f0a1?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49 [1 of 3 alike] | PASS |
| r00495 | admin | /products/[product] | desktop | en | main | Compare-at price · Bulk fill (`bulk-compare`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00496 | admin | /inventory | mobile | zh-TW | main | Sweep Ceramic Mug | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00497 | admin | /products/[product] | desktop | en | main | Quantity · Bulk fill (`bulk-quantity`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00498 | admin | /customers | desktop | zh-TW | main | 開啟客戶: 王小明 (`customer-open-d8504f1a-e361-4ff3-8912-7bbecca7b2f5`) | click | navigation or in-page change | url /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/customers/d8504f1a-e361-4ff3-8912-7bbecca7b2f5?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49 [1 of 3 alike] | PASS |
| r00499 | admin | /inventory | mobile | zh-TW | main | 選取 T04-24a21ca87d63-0 | click | visible change | DOM changed (6 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00500 | admin | /products/[product] | desktop | en | main | SKU code · Bulk fill (`bulk-code`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00501 | admin | /customers | desktop | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-bea8b3ab-3a45-4a21-81d6-466bd04fb65c`) | click | navigation or in-page change | url /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/customers/bea8b3ab-3a45-4a21-81d6-466bd04fb65c?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49 [1 of 3 alike] | PASS |
| r00502 | admin | /inventory | mobile | zh-TW | main | Sweep Cedar Candle | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00503 | admin | /customers | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00504 | admin | /products/[product] | desktop | en | main | Live keyword · Bulk fill (`bulk-keyword`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00505 | admin | /customers | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00506 | admin | /inventory | mobile | zh-TW | main | 選取 SWP-SCF | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00507 | admin | /products/[product] | desktop | en | main | Do not track ∞ | click (toggle) | visible change | DOM changed (5 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00508 | admin | /customers | mobile | zh-TW | main | 搜尋 (`customers-search-submit`) | click | visible change | DOM changed (12 mutations) | PASS |
| r00509 | admin | /inventory | mobile | zh-TW | main | Sweep Wool Scarf | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00510 | admin | /products/[product] | desktop | en | main | Enabled 1 (`matrix-active-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00511 | admin | /customers | mobile | zh-TW | main | 重新整理 (`customers-refresh`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00512 | admin | /inventory | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00513 | admin | /products/[product] | desktop | en | main | Sweep home | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (true -> false); scrolled | PASS |
| r00514 | admin | /inventory | desktop | en | main | Overview | click | navigation or in-page change | url /en/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00515 | admin | /customers | mobile | zh-TW | main | 開啟客戶: 王小明 (`customer-open-e2f26bae-8749-4104-9813-2872648a967b`) | click | navigation or in-page change | url /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/customers/e2f26bae-8749-4104-9813-2872648a967b?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49 [1 of 3 alike] | PASS |
| r00516 | admin | /products/[product] | desktop | en | main | Sweep wear | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00517 | admin | /inventory | desktop | en | main | Add product | click | navigation or in-page change | url /en/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/products/new?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00518 | admin | /customers | mobile | zh-TW | main | 開啟客戶: 王小明 (`customer-open-97ea14b3-8819-4403-b647-d5891d42aebd`) | click | navigation or in-page change | url /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/customers/97ea14b3-8819-4403-b647-d5891d42aebd?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49 [1 of 3 alike] | PASS |
| r00519 | admin | /products/[product] | desktop | en | main | Manage collections | click | navigation or in-page change | scrolled | PASS |
| r00520 | admin | /inventory | desktop | en | main | Search | click | visible change | DOM changed (2 mutations) | PASS |
| r00521 | admin | /customers | mobile | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-b5940f80-a3a0-496f-ae29-4dbd207047d2`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00522 | admin | /inventory | desktop | en | main | Warehouse | selectOption | visible change | select has a single option | SKIP |
| r00523 | admin | /products/[product] | desktop | en | main | Shipping | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00524 | admin | /customers | mobile | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-4b1bcce9-fe52-4b2f-878d-91daa943f0a1`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00525 | admin | /inventory | desktop | en | main | Status | selectOption(active) | visible change | url /en/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/inventory?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&status=active; DOM changed (2 mutations | PASS |
| r00526 | admin | /products/[product] | desktop | en | layer of Shipping | Shipping | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00527 | admin | /customers | mobile | zh-TW | main | 開啟客戶: 王小明 (`customer-open-d8504f1a-e361-4ff3-8912-7bbecca7b2f5`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00528 | admin | /inventory | desktop | en | main | Reset | click | visible change | DOM changed (2 mutations) | PASS |
| r00529 | admin | /products/[product] | desktop | en | main | Search engine listing | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00530 | admin | /customers | mobile | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-bea8b3ab-3a45-4a21-81d6-466bd04fb65c`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00531 | admin | /inventory | desktop | en | main | Refresh | click | visible change | DOM changed (2 mutations) | PASS |
| r00532 | admin | /customers | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00533 | admin | /products/[product] | desktop | en | layer of Search engine listing | Search engine listing | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00534 | admin | /inventory | desktop | en | main | Select SWP-MUG | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00535 | admin | /customers | desktop | en | main | Overview | click | navigation or in-page change | url /en/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00536 | admin | /products/[product] | desktop | en | main | Save changes (`product-save`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00537 | admin | /inventory | desktop | en | main | Sweep Ceramic Mug | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00538 | admin | /customers | desktop | en | main | Search (`customers-search-submit`) | click | visible change | DOM changed (12 mutations) | PASS |
| r00539 | admin | /products/[product] | desktop | en | main | Unpublish to draft (`product-unpublish`) | click | confirmation layer opens; cancel; nothing changed | dialog: confirm: Unpublish this product? Shoppers will no longer be able to buy it. (confirmation shown, then cancelled) | PASS |
| r00540 | admin | /inventory | desktop | en | main | Select | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00541 | admin | /customers | desktop | en | main | Refresh (`customers-refresh`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00542 | admin | /products/[product] | desktop | en | main | Publish (`product-publish`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (1 mutations) (inline change only; no request fired) | PASS |
| r00543 | admin | /inventory | desktop | en | main | Select T04-24a21ca87d63-0 | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00544 | admin | /customers | desktop | en | main | Open customer: 王小明 (`customer-open-e2f26bae-8749-4104-9813-2872648a967b`) | click | navigation or in-page change | url /en/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/customers/e2f26bae-8749-4104-9813-2872648a967b?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; D [1 of 3 alike] | PASS |
| r00545 | admin | /customers/[customer] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00546 | admin | /inventory | desktop | en | main | Sweep Cedar Candle | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00547 | admin | /customers | desktop | en | main | Open customer: 王小明 (`customer-open-97ea14b3-8819-4403-b647-d5891d42aebd`) | click | navigation or in-page change | url /en/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/customers/97ea14b3-8819-4403-b647-d5891d42aebd?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; D [1 of 3 alike] | PASS |
| r00548 | admin | /customers/[customer] | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers/4b1bcce9-fe52-4b2f-878d-91daa943f0a1?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM c | PASS |
| r00549 | admin | /inventory | desktop | en | main | Select | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00550 | admin | /customers | desktop | en | main | Open customer: Synthetic Buyer (`customer-open-b5940f80-a3a0-496f-ae29-4dbd207047d2`) | click | navigation or in-page change | url /en/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/customers/b5940f80-a3a0-496f-ae29-4dbd207047d2?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; D [1 of 3 alike] | PASS |
| r00551 | admin | /customers/[customer] | desktop | zh-TW | main | 全部客戶 | click | navigation or in-page change | url /zh-TW/customers/4b1bcce9-fe52-4b2f-878d-91daa943f0a1?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49 | PASS |
| r00552 | admin | /inventory | desktop | en | main | Select SWP-SCF | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00553 | admin | /customers | desktop | en | main | Open customer: Synthetic Buyer (`customer-open-4b1bcce9-fe52-4b2f-878d-91daa943f0a1`) | click | navigation or in-page change | url /en/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/customers/4b1bcce9-fe52-4b2f-878d-91daa943f0a1?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; D [1 of 3 alike] | PASS |
| r00554 | admin | /customers/[customer] | desktop | zh-TW | main | 在訂單中開啟: 5cfe749c-2972-423d-bdc1-eb33362d672b | click | navigation or in-page change | url /zh-TW/customers/4b1bcce9-fe52-4b2f-878d-91daa943f0a1?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 | PASS |
| r00555 | admin | /inventory | desktop | en | main | Sweep Wool Scarf | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00556 | admin | /customers | desktop | en | main | Open customer: 王小明 (`customer-open-d8504f1a-e361-4ff3-8912-7bbecca7b2f5`) | click | navigation or in-page change | url /en/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/customers/d8504f1a-e361-4ff3-8912-7bbecca7b2f5?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; D [1 of 3 alike] | PASS |
| r00557 | admin | /customers/[customer] | desktop | zh-TW | main | 下載資料 (`customer-download`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00558 | admin | /inventory | desktop | en | main | Select | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00559 | admin | /customers | desktop | en | main | Open customer: Synthetic Buyer (`customer-open-bea8b3ab-3a45-4a21-81d6-466bd04fb65c`) | click | navigation or in-page change | url /en/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/customers/bea8b3ab-3a45-4a21-81d6-466bd04fb65c?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; D [1 of 3 alike] | PASS |
| r00560 | admin | /customers/[customer] | desktop | zh-TW | main | 抹除客戶 (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00561 | admin | /ads/attribution | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00562 | admin | /promotions | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00563 | admin | /customers/[customer] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00564 | admin | /ads/attribution | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads/attribution?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00565 | admin | /promotions | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/promotions?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00566 | admin | /customers/[customer] | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers/4b1bcce9-fe52-4b2f-878d-91daa943f0a1?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM c | PASS |
| r00567 | admin | /promotions | desktop | zh-TW | main | 類型 (`promotion-kind`) | selectOption(fixed) | visible change | DOM changed (1 mutations); the control's own state changed (percent -> fixed) | PASS |
| r00568 | admin | /ads/attribution | desktop | zh-TW | main | 返回廣告 (`attribution-back`) | click | navigation or in-page change | url /zh-TW/ads/attribution?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/ads?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (19 mutations) | PASS |
| r00569 | admin | /customers/[customer] | mobile | zh-TW | main | 全部客戶 | click | navigation or in-page change | url /zh-TW/customers/4b1bcce9-fe52-4b2f-878d-91daa943f0a1?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49 | PASS |
| r00570 | admin | /promotions | desktop | zh-TW | main | 建立優惠碼 (`promotion-submit`) | click | visible change | validation shown | PASS |
| r00571 | admin | /ads/attribution | desktop | zh-TW | main | 讀取報表 (`attribution-apply`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00572 | admin | /customers/[customer] | mobile | zh-TW | main | 在訂單中開啟: 5cfe749c-2972-423d-bdc1-eb33362d672b | click | navigation or in-page change | url /zh-TW/customers/4b1bcce9-fe52-4b2f-878d-91daa943f0a1?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 | PASS |
| r00573 | admin | /ads/attribution | desktop | zh-TW | main | 直播場次 (`attribution-session`) | selectOption | visible change | select has a single option | SKIP |
| r00574 | admin | /promotions | desktop | zh-TW | main | 暫停 (`promotion-toggle`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00575 | admin | /customers/[customer] | mobile | zh-TW | main | 下載資料 (`customer-download`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00576 | admin | /ads/attribution | desktop | zh-TW | main | 重新連接 Facebook (`attribution-reconnect`) | click | navigation or in-page change | scrolled | PASS |
| r00577 | admin | /promotions | desktop | zh-TW | main | 編輯 (`promotion-edit`) | click | visible change | DOM changed (6 mutations) | PASS |
| r00578 | admin | /customers/[customer] | mobile | zh-TW | main | 抹除客戶 (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00579 | admin | /ads/attribution | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00580 | admin | /promotions | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00581 | admin | /customers/[customer] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00582 | admin | /ads/attribution | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads/attribution?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00583 | admin | /promotions | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/promotions?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00584 | admin | /customers/[customer] | desktop | en | main | Overview | click | navigation or in-page change | url /en/customers/4b1bcce9-fe52-4b2f-878d-91daa943f0a1?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed | PASS |
| r00585 | admin | /promotions | mobile | zh-TW | main | 類型 (`promotion-kind`) | selectOption(fixed) | visible change | DOM changed (1 mutations); the control's own state changed (percent -> fixed) | PASS |
| r00586 | admin | /ads/attribution | mobile | zh-TW | main | 返回廣告 (`attribution-back`) | click | navigation or in-page change | url /zh-TW/ads/attribution?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/ads?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (28 mutations) | PASS |
| r00587 | admin | /customers/[customer] | desktop | en | main | All customers | click | navigation or in-page change | url /en/customers/4b1bcce9-fe52-4b2f-878d-91daa943f0a1?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/customers?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; D | PASS |
| r00588 | admin | /promotions | mobile | zh-TW | main | 建立優惠碼 (`promotion-submit`) | click | visible change | validation shown; scrolled | PASS |
| r00589 | admin | /ads/attribution | mobile | zh-TW | main | 讀取報表 (`attribution-apply`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00590 | admin | /customers/[customer] | desktop | en | main | Open in orders: 5cfe749c-2972-423d-bdc1-eb33362d672b | click | navigation or in-page change | url /en/customers/4b1bcce9-fe52-4b2f-878d-91daa943f0a1?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/orders?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&order | PASS |
| r00591 | admin | /promotions | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00592 | admin | /ads/attribution | mobile | zh-TW | main | 直播場次 (`attribution-session`) | selectOption | visible change | select has a single option | SKIP |
| r00593 | admin | /promotions | desktop | en | main | Overview | click | navigation or in-page change | url /en/promotions?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00594 | admin | /customers/[customer] | desktop | en | main | Download data (`customer-download`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00595 | admin | /ads/attribution | mobile | zh-TW | main | 重新連接 Facebook (`attribution-reconnect`) | click | navigation or in-page change | scrolled | PASS |
| r00596 | admin | /promotions | desktop | en | main | Type (`promotion-kind`) | selectOption(fixed) | visible change | DOM changed (1 mutations); the control's own state changed (percent -> fixed) | PASS |
| r00597 | admin | /customers/[customer] | desktop | en | main | Erase customer (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00598 | admin | /ads/attribution | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00599 | admin | /promotions | desktop | en | main | Create code (`promotion-submit`) | click | visible change | validation shown | PASS |
| r00600 | admin | /ads/attribution | desktop | en | main | Overview | click | navigation or in-page change | url /en/ads/attribution?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00601 | admin | /ads | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00602 | admin | /ads | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00603 | admin | /promotions | desktop | en | main | Pause (`promotion-toggle`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00604 | admin | /ads/attribution | desktop | en | main | Back to ads (`attribution-back`) | click | navigation or in-page change | url /en/ads/attribution?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/ads?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (19 mutations) | PASS |
| r00605 | admin | /ads | desktop | zh-TW | main | 廣告歸因與直播復盤 (`ads-attribution-link`) | click | navigation or in-page change | url /zh-TW/ads?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/ads/attribution?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 | PASS |
| r00606 | admin | /promotions | desktop | en | main | Edit (`promotion-edit`) | click | visible change | DOM changed (6 mutations) | PASS |
| r00607 | admin | /ads/attribution | desktop | en | main | Read report (`attribution-apply`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00608 | admin | /design | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00609 | admin | /ads | desktop | zh-TW | main | 重新整理 (`ads-refresh`) | click | visible change | DOM changed (9 mutations) | PASS |
| r00610 | admin | /ads/attribution | desktop | en | main | Live session (`attribution-session`) | selectOption | visible change | select has a single option | SKIP |
| r00611 | admin | /design | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/design?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00612 | admin | /ads | desktop | zh-TW | main | 連結 Meta 廣告帳號 (`ads-connect`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00613 | admin | /ads/attribution | desktop | en | main | Reconnect Facebook (`attribution-reconnect`) | click | navigation or in-page change | scrolled | PASS |
| r00614 | admin | /design | desktop | zh-TW | main | 預覽 (`design-preview`) | click | visible change | popup: about:blank; DOM changed (2 mutations) | PASS |
| r00615 | admin | /ads | desktop | zh-TW | main | 新增草稿 (`ads-new-draft`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00616 | admin | /finance | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00617 | admin | /finance | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/finance?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00618 | admin | /ads | desktop | zh-TW | main | 顯示成效 (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00619 | admin | /finance | desktop | zh-TW | main | 查看 (`finance-show`) | click | visible change | url /zh-TW/finance?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/finance?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&from=2026-09-05&to=2026-10-04; DOM ch | PASS |
| r00620 | admin | /ads | desktop | zh-TW | main | 傳送購買事件 (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00621 | admin | /ads | desktop | zh-TW | main | 資料集請先連結廣告帳號與資料集才能開啟。 (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00622 | admin | /finance | desktop | zh-TW | main | 下載 CSV (`finance-csv`) | click | navigation or in-page change | download: finance-2026-09-05-2026-10-04.csv | PASS |
| r00623 | admin | /design | desktop | zh-TW | main | 店鋪資料 (`design-tab-profile`) | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00624 | admin | /finance | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00625 | admin | /ads | desktop | zh-TW | main | 儲存 (`ads-capi-save`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00626 | admin | /design | desktop | zh-TW | main | 導覽 (`design-tab-nav`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00627 | admin | /finance | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/finance?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00628 | admin | /ads | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00629 | admin | /design | desktop | zh-TW | main | 首頁 (`design-tab-home`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00630 | admin | /finance | mobile | zh-TW | main | 查看 (`finance-show`) | click | visible change | url /zh-TW/finance?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/finance?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&from=2026-09-05&to=2026-10-04; DOM ch | PASS |
| r00631 | admin | /ads | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00632 | admin | /design | desktop | zh-TW | main | 資訊頁 (`design-tab-pages`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00633 | admin | /ads | mobile | zh-TW | main | 廣告歸因與直播復盤 (`ads-attribution-link`) | click | navigation or in-page change | url /zh-TW/ads?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW/ads/attribution?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 | PASS |
| r00634 | admin | /finance | mobile | zh-TW | main | 下載 CSV (`finance-csv`) | click | navigation or in-page change | download: finance-2026-09-05-2026-10-04.csv | PASS |
| r00635 | admin | /design | desktop | zh-TW | main | 版本 (`design-tab-versions`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00636 | admin | /finance | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00637 | admin | /ads | mobile | zh-TW | main | 重新整理 (`ads-refresh`) | click | visible change | DOM changed (9 mutations) | PASS |
| r00638 | admin | /design | desktop | zh-TW | main | 選擇圖片 (`design-logo-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00639 | admin | /finance | desktop | en | main | Overview | click | navigation or in-page change | url /en/finance?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00640 | admin | /ads | mobile | zh-TW | main | 連結 Meta 廣告帳號 (`ads-connect`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00641 | admin | /design | desktop | zh-TW | main | 選擇圖片 (`design-favicon-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00642 | admin | /finance | desktop | en | main | Show (`finance-show`) | click | visible change | url /en/finance?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/finance?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180&from=2026-09-05&to=2026-10-04; DOM changed  | PASS |
| r00643 | admin | /ads | mobile | zh-TW | main | 新增草稿 (`ads-new-draft`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00644 | admin | /design | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00645 | admin | /finance | desktop | en | main | Download CSV (`finance-csv`) | click | navigation or in-page change | download: finance-2026-09-05-2026-10-04.csv | PASS |
| r00646 | admin | /design | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/design?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00647 | admin | /ads | mobile | zh-TW | main | 顯示成效 (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00648 | admin | /settings | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00649 | admin | /design | mobile | zh-TW | main | 預覽 (`design-preview`) | click | visible change | popup: about:blank; DOM changed (2 mutations) | PASS |
| r00650 | admin | /ads | mobile | zh-TW | main | 傳送購買事件 (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00651 | admin | /settings | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/settings?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00652 | admin | /ads | mobile | zh-TW | main | 資料集請先連結廣告帳號與資料集才能開啟。 (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00653 | admin | /ads | mobile | zh-TW | main | 儲存 (`ads-capi-save`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00654 | admin | /ads | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00655 | admin | /design | mobile | zh-TW | main | 店鋪資料 (`design-tab-profile`) | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00656 | admin | /ads | desktop | en | main | Overview | click | navigation or in-page change | url /en/ads?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00657 | admin | /settings | desktop | zh-TW | main | 1 選擇平台 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00658 | admin | /design | mobile | zh-TW | main | 導覽 (`design-tab-nav`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00659 | admin | /ads | desktop | en | main | Ad attribution & session review (`ads-attribution-link`) | click | navigation or in-page change | url /en/ads?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en/ads/attribution?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 | PASS |
| r00660 | admin | /design | mobile | zh-TW | main | 首頁 (`design-tab-home`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00661 | admin | /ads | desktop | en | main | Refresh (`ads-refresh`) | click | visible change | DOM changed (9 mutations) | PASS |
| r00662 | admin | /design | mobile | zh-TW | main | 資訊頁 (`design-tab-pages`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00663 | admin | /ads | desktop | en | main | Connect a Meta ad account (`ads-connect`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00664 | admin | /settings | desktop | zh-TW | main | PAYUNi 收款 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00665 | admin | /design | mobile | zh-TW | main | 版本 (`design-tab-versions`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00666 | admin | /ads | desktop | en | main | New draft (`ads-new-draft`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00667 | admin | /settings | desktop | zh-TW | main | 商家自行安排配送 | click | visible change | DOM changed (8 mutations); the control's own state changed (false -> true) | PASS |
| r00668 | admin | /design | mobile | zh-TW | main | 選擇圖片 (`design-logo-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00669 | admin | /ads | desktop | en | main | Show results (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00670 | admin | /settings | desktop | zh-TW | main | 繼續 | click | visible change | DOM changed (12 mutations) | PASS |
| r00671 | admin | /design | mobile | zh-TW | main | 選擇圖片 (`design-favicon-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00672 | admin | /ads | desktop | en | main | Send purchase events (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00673 | admin | /settings | desktop | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00674 | admin | /design | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00675 | admin | /ads | desktop | en | main | DatasetConnect a dataset with an ad account to turn this on. (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00676 | admin | /design | desktop | en | main | Overview | click | navigation or in-page change | url /en/design?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00677 | admin | /settings | desktop | zh-TW | main | 取消發佈 (`storefront-toggle`) | click | confirmation layer opens; cancel; nothing changed | layer opened (取消發佈); DOM changed (4 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00678 | admin | /ads | desktop | en | main | Save (`ads-capi-save`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00679 | admin | /design | desktop | en | main | Preview (`design-preview`) | click | visible change | popup: about:blank; DOM changed (2 mutations) | PASS |
| r00680 | admin | /settings | desktop | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00681 | admin | /team | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00682 | admin | /settings | desktop | zh-TW | main | 暫停 (`storefront-domain-suspend`) | click | confirmation layer opens; cancel; nothing changed | layer opened (暫停); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00683 | admin | /team | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/team?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00684 | admin | /team | desktop | zh-TW | main | 角色 (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00685 | admin | /settings | desktop | zh-TW | main | 解除綁定 (`storefront-domain-detach`) | click | confirmation layer opens; cancel; nothing changed | layer opened (解除綁定); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00686 | admin | /team | desktop | zh-TW | main | 寄送邀請 (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00687 | admin | /settings | desktop | zh-TW | main | 請求驗證 (`storefront-domain-submit`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00688 | admin | /design | desktop | en | main | Store profile (`design-tab-profile`) | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00689 | admin | /team | desktop | zh-TW | main | 變更角色 (`member-role-79bd12bc-86ed-4491-b31d-2c499ff8a163`) | selectOption(admin) | visible change | DOM changed (3 mutations) | PASS |
| r00690 | admin | /settings | desktop | zh-TW | main | 新增專頁 (`metaconnect-add`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00691 | admin | /design | desktop | en | main | Navigation (`design-tab-nav`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00692 | admin | /team | desktop | zh-TW | main | 移除 (`member-remove-79bd12bc-86ed-4491-b31d-2c499ff8a163`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (5 mutations) (inline change only; no request fired) | PASS |
| r00693 | admin | /settings | desktop | zh-TW | main | 選擇專頁重新授權 (`metaconnect-reconnect`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00694 | admin | /design | desktop | en | main | Home page (`design-tab-home`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00695 | admin | /team | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00696 | admin | /settings | desktop | zh-TW | main | 中斷連接 (`metaconnect-disconnect`) | click | confirmation layer opens; cancel; nothing changed | layer opened (中斷連接: Sweep Page); DOM changed (5 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00697 | admin | /design | desktop | en | main | Pages (`design-tab-pages`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00698 | admin | /team | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/team?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00699 | admin | /team | mobile | zh-TW | main | 角色 (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00700 | admin | /settings | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00701 | admin | /design | desktop | en | main | Versions (`design-tab-versions`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00702 | admin | /settings | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/settings?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00703 | admin | /team | mobile | zh-TW | main | 寄送邀請 (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00704 | admin | /design | desktop | en | main | Choose image (`design-logo-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00705 | admin | /team | mobile | zh-TW | main | 變更角色 (`member-role-79bd12bc-86ed-4491-b31d-2c499ff8a163`) | selectOption(admin) | visible change | DOM changed (3 mutations) | PASS |
| r00706 | admin | /design | desktop | en | main | Choose image (`design-favicon-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00707 | admin | /team | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00708 | admin | /billing | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00709 | admin | /team | desktop | en | main | Overview | click | navigation or in-page change | url /en/team?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00710 | admin | /billing | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/billing?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00711 | admin | /settings | mobile | zh-TW | main | 1 選擇平台 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00712 | admin | /team | desktop | en | main | Role (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00713 | admin | /billing | desktop | zh-TW | main | 訂閱 (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00714 | admin | /team | desktop | en | main | Send invitation (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00715 | admin | /billing | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00716 | admin | /team | desktop | en | main | Change role (`member-role-79bd12bc-86ed-4491-b31d-2c499ff8a163`) | selectOption(admin) | visible change | DOM changed (3 mutations) | PASS |
| r00717 | admin | /billing | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/billing?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /zh-TW?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00718 | admin | /team | desktop | en | main | Remove (`member-remove-79bd12bc-86ed-4491-b31d-2c499ff8a163`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (5 mutations) (inline change only; no request fired) | PASS |
| r00719 | admin | /billing | mobile | zh-TW | main | 訂閱 (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00720 | admin | /settings | mobile | zh-TW | main | PAYUNi 收款 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00721 | admin | /invite/[token] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00722 | admin | /billing | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00723 | admin | /invite/[token] | desktop | zh-TW | main | 登入 (`invite-signin`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (47 mu | PASS |
| r00724 | admin | /settings | mobile | zh-TW | main | 商家自行安排配送 | click | visible change | DOM changed (8 mutations); the control's own state changed (false -> true) | PASS |
| r00725 | admin | /billing | desktop | en | main | Overview | click | navigation or in-page change | url /en/billing?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (15 mutations) | PASS |
| r00726 | admin | /invite/[token] | desktop | zh-TW | main | 註冊帳號 (`invite-signup`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW/signup#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed | PASS |
| r00727 | admin | /settings | mobile | zh-TW | main | 繼續 | click | visible change | DOM changed (12 mutations) | PASS |
| r00728 | admin | /billing | desktop | en | main | Subscribe (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00729 | admin | /invite/[token] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00730 | admin | /settings | mobile | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00731 | admin | /invite/[token] | mobile | zh-TW | main | 登入 (`invite-signin`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (47 mu | PASS |
| r00732 | admin | /reset | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00733 | admin | /reset | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00734 | admin | /settings | mobile | zh-TW | main | 取消發佈 (`storefront-toggle`) | click | confirmation layer opens; cancel; nothing changed | layer opened (取消發佈); DOM changed (4 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00735 | admin | /invite/[token] | mobile | zh-TW | main | 註冊帳號 (`invite-signup`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW/signup#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed | PASS |
| r00736 | admin | /reset | desktop | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00737 | admin | /invite/[token] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00738 | admin | /settings | mobile | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00739 | admin | /invite/[token] | desktop | en | main | Sign in (`invite-signin`) | click | navigation or in-page change | url /en/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /en#next=%2Fen%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (47 mutations); | PASS |
| r00740 | admin | /reset | desktop | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/reset -> /zh-TW; DOM changed (16 mutations); validation shown | PASS |
| r00741 | admin | /settings | mobile | zh-TW | main | 暫停 (`storefront-domain-suspend`) | click | confirmation layer opens; cancel; nothing changed | layer opened (暫停); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00742 | admin | /invite/[token] | desktop | en | main | Create an account (`invite-signup`) | click | navigation or in-page change | url /en/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /en/signup#next=%2Fen%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (36 muta | PASS |
| r00743 | admin | /reset | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00744 | admin | /reset | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00745 | admin | /signup | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00746 | admin | /settings | mobile | zh-TW | main | 解除綁定 (`storefront-domain-detach`) | click | confirmation layer opens; cancel; nothing changed | layer opened (解除綁定); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00747 | admin | /signup | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00748 | admin | /reset | mobile | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00749 | admin | /settings | mobile | zh-TW | main | 請求驗證 (`storefront-domain-submit`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00750 | admin | /signup | desktop | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00751 | admin | /reset | mobile | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/reset -> /zh-TW; DOM changed (16 mutations); validation shown | PASS |
| r00752 | admin | /settings | mobile | zh-TW | main | 新增專頁 (`metaconnect-add`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00753 | admin | /signup | desktop | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/signup -> /zh-TW; DOM changed (9 mutations) | PASS |
| r00754 | admin | /reset | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00755 | admin | /reset | desktop | en | main | Language | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00756 | admin | /settings | mobile | zh-TW | main | 選擇專頁重新授權 (`metaconnect-reconnect`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00757 | admin | /signup | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00758 | admin | /signup | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00759 | admin | /reset | desktop | en | main | Send code | click | visible change | validation shown | PASS |
| r00760 | admin | /settings | mobile | zh-TW | main | 中斷連接 (`metaconnect-disconnect`) | click | confirmation layer opens; cancel; nothing changed | layer opened (中斷連接: Sweep Page); DOM changed (5 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00761 | admin | /signup | mobile | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00762 | admin | /reset | desktop | en | main | Back to sign in | click | navigation or in-page change | url /en/reset -> /en; DOM changed (16 mutations); validation shown | PASS |
| r00763 | admin | /settings | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00764 | admin | /signup | mobile | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/signup -> /zh-TW; DOM changed (9 mutations) | PASS |
| r00765 | admin | /settings | desktop | en | main | Overview | click | navigation or in-page change | url /en/settings?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180 -> /en?store=9b3cec4a-09ba-4b8e-90f1-3393a7d49180; DOM changed (11 mutations) | PASS |
| r00766 | admin | /signup | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00767 | admin | /signup | desktop | en | main | Language | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00768 | admin | /signup | desktop | en | main | Send code | click | visible change | validation shown | PASS |
| r00769 | admin | /settings | desktop | en | main | 1 Choose platform | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00770 | admin | /signup | desktop | en | main | Back to sign in | click | navigation or in-page change | url /en/signup -> /en; DOM changed (9 mutations) | PASS |
| r00771 | admin | /settings | desktop | en | main | PAYUNi payment | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00772 | admin | /settings | desktop | en | main | Merchant-arranged delivery | click | visible change | DOM changed (8 mutations); the control's own state changed (false -> true) | PASS |
| r00773 | admin | /settings | desktop | en | main | Continue | click | visible change | DOM changed (12 mutations) | PASS |
| r00774 | admin | /settings | desktop | en | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00775 | admin | /settings | desktop | en | main | Unpublish storefront (`storefront-toggle`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Unpublish storefront); DOM changed (4 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00776 | admin | /settings | desktop | en | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00777 | admin | /settings | desktop | en | main | Suspend (`storefront-domain-suspend`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Suspend); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00778 | admin | /settings | desktop | en | main | Detach (`storefront-domain-detach`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Detach); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00779 | admin | /settings | desktop | en | main | Request verification (`storefront-domain-submit`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00780 | admin | /settings | desktop | en | main | Add Page (`metaconnect-add`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00781 | admin | /settings | desktop | en | main | Choose a Page to reauthorize (`metaconnect-reconnect`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00782 | admin | /settings | desktop | en | main | Disconnect (`metaconnect-disconnect`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Disconnect: Sweep Page); DOM changed (5 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00783 | storefront | /cart | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00784 | storefront | /cart | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00785 | storefront | /cart | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00786 | storefront | /cart | mobile | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00787 | storefront | /cart | desktop | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00788 | storefront | /cart | desktop | en | main | Sweep Wool Scarf | click | navigation or in-page change | url /en/cart -> /en/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00789 | storefront | /cart | mobile | zh-TW | main | 增加數量 | click | visible change | DOM changed (3 mutations) | PASS |
| r00790 | storefront | /cart | desktop | zh-TW | main | 增加數量 | click | visible change | DOM changed (3 mutations) | PASS |
| r00791 | storefront | /cart | desktop | en | main | Increase quantity | click | visible change | DOM changed (3 mutations) | PASS |
| r00792 | storefront | /cart | mobile | zh-TW | main | 將 Sweep Wool Scarf 移出購物車 | click | visible change | DOM changed (3 mutations) | PASS |
| r00793 | storefront | /cart | desktop | zh-TW | main | 將 Sweep Wool Scarf 移出購物車 | click | visible change | DOM changed (3 mutations) | PASS |
| r00794 | storefront | /cart | desktop | en | main | Remove Sweep Wool Scarf from the cart | click | confirmation layer opens; cancel; nothing changed | DOM changed (3 mutations) (inline change only; no request fired) | PASS |
| r00795 | storefront | /cart | mobile | zh-TW | main | 前往結帳 (`cart-checkout`) | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/checkout; DOM changed (11 mutations) | PASS |
| r00796 | storefront | /cart | desktop | zh-TW | main | 前往結帳 (`cart-checkout`) | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/checkout; DOM changed (11 mutations) | PASS |
| r00797 | storefront | /cart | desktop | en | main | Checkout (`cart-checkout`) | click | navigation or in-page change | url /en/cart -> /en/checkout; DOM changed (11 mutations) | PASS |
| r00798 | storefront | /cart | mobile | zh-TW | main | 繼續選購 | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00799 | storefront | /cart | desktop | en | main | Continue shopping | click | navigation or in-page change | url /en/cart -> /en/products; DOM changed (4 mutations) | PASS |
| r00800 | storefront | /checkout | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00801 | storefront | /checkout | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00802 | storefront | /checkout | mobile | zh-TW | main | 我的訂單 (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00803 | storefront | /checkout | desktop | en | main | Your orders (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00804 | storefront | /cart | desktop | zh-TW | main | 繼續選購 | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00805 | storefront | /checkout | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00806 | storefront | /checkout | desktop | zh-TW | main | 我的訂單 (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00807 | storefront | /checkout | mobile | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/products/sweep-wool-scarf; DOM changed (6 mutations) | PASS |
| r00808 | storefront | /checkout | desktop | en | main | Sweep Wool Scarf | click | navigation or in-page change | url /en/checkout -> /en/products/sweep-wool-scarf; DOM changed (6 mutations) | PASS |
| r00809 | storefront | /checkout | mobile | zh-TW | main | 返回購物車 | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/cart; DOM changed (9 mutations) | PASS |
| r00810 | storefront | /checkout | desktop | en | main | Back to cart | click | navigation or in-page change | url /en/checkout -> /en/cart; DOM changed (9 mutations) | PASS |
| r00811 | storefront | /checkout | mobile | zh-TW | main | 選擇配送 | click | visible change | DOM changed (3 mutations) | PASS |
| r00812 | storefront | /checkout | desktop | en | main | Choose delivery | click | visible change | DOM changed (3 mutations) | PASS |
| r00813 | storefront | /checkout | desktop | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/products/sweep-wool-scarf; DOM changed (7 mutations) | PASS |
| r00814 | storefront | /checkout | desktop | zh-TW | main | 返回購物車 | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/cart; DOM changed (9 mutations) | PASS |
| r00815 | storefront | /checkout | desktop | zh-TW | main | 選擇配送 | click | visible change | DOM changed (3 mutations) | PASS |
| r00816 | storefront | /orders/[orderID] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00817 | storefront | /orders/[orderID] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00818 | storefront | /orders/[orderID] | mobile | zh-TW | main | 重新整理訂單 (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone); scrolled | PASS |
| r00819 | storefront | /orders/[orderID] | desktop | en | main | Refresh order (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone); scrolled | PASS |
| r00820 | storefront | /orders/[orderID] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00821 | storefront | /orders/[orderID] | desktop | zh-TW | main | 重新整理訂單 (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone); scrolled | PASS |
| r00822 | storefront | / | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00823 | storefront | / | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00824 | storefront | / | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00825 | storefront | / | mobile | zh-TW | chrome | 跳到主要內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW -> /zh-TW#main; scrolled | PASS |
| r00826 | storefront | / | desktop | en | chrome | Skip to content | keyboard: Tab to the link, Enter | navigation or in-page change | url /en -> /en#main; scrolled | PASS |
| r00827 | storefront | / | mobile | zh-TW | chrome | 選單 (`menu-open`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00828 | storefront | / | desktop | en | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00829 | storefront | / | mobile | zh-TW | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00830 | storefront | / | desktop | en | chrome | All products | click | navigation or in-page change | url /en -> /en/products; DOM changed (4 mutations) | PASS |
| r00831 | storefront | / | mobile | zh-TW | chrome | 搜尋 | click | navigation or in-page change | url /zh-TW -> /zh-TW/search; DOM changed (4 mutations) | PASS |
| r00832 | storefront | / | desktop | en | chrome | Sweep home | click | navigation or in-page change | url /en -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00833 | storefront | / | mobile | zh-TW | chrome | 購物車，1 件商品 (`header-cart`) | click | visible change | layer opened (DIALOG); DOM changed (2 mutations) | PASS |
| r00834 | storefront | / | desktop | en | chrome | About us | click | navigation or in-page change | url /en -> /en/pages/about; DOM changed (4 mutations) | PASS |
| r00835 | storefront | / | desktop | zh-TW | chrome | 跳到主要內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW -> /zh-TW#main; scrolled | PASS |
| r00836 | storefront | / | desktop | en | chrome | Search | click | visible change | url /en -> /en/search?q= | PASS |
| r00837 | storefront | / | desktop | en | chrome | Cart, 1 items (`header-cart`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00838 | storefront | / | desktop | zh-TW | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00839 | storefront | / | desktop | zh-TW | chrome | All products | click | navigation or in-page change | url /zh-TW -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00840 | storefront | / | desktop | zh-TW | chrome | Sweep home | click | navigation or in-page change | url /zh-TW -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00841 | storefront | / | desktop | zh-TW | chrome | About us | click | navigation or in-page change | url /zh-TW -> /zh-TW/pages/about; DOM changed (4 mutations) | PASS |
| r00842 | storefront | / | mobile | zh-TW | layer of 購物車，1 件商品 | 查看購物車 | click | navigation or in-page change | layer closed; DOM changed (2 mutations); the control's own state changed ( -> gone) | PASS |
| r00843 | storefront | / | desktop | zh-TW | chrome | 搜尋 | click | visible change | url /zh-TW -> /zh-TW/search?q= | PASS |
| r00844 | storefront | / | desktop | zh-TW | chrome | 購物車，1 件商品 (`header-cart`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00845 | storefront | / | desktop | en | chrome | +886 2 2345 6789 | click | navigation or in-page change | scrolled | PASS |
| r00846 | storefront | / | mobile | zh-TW | chrome | +886 2 2345 6789 | click | navigation or in-page change | no in-page change (tel: link) (protocol handler link: href verified, OS handler not observable headless) | PASS |
| r00847 | storefront | / | desktop | en | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r00848 | storefront | / | mobile | zh-TW | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r00849 | storefront | / | desktop | en | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r00850 | storefront | / | mobile | zh-TW | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r00851 | storefront | / | desktop | en | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r00852 | storefront | / | mobile | zh-TW | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r00853 | storefront | / | desktop | en | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r00854 | storefront | / | mobile | zh-TW | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r00855 | storefront | / | desktop | en | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00856 | storefront | / | desktop | zh-TW | chrome | +886 2 2345 6789 | click | navigation or in-page change | scrolled | PASS |
| r00857 | storefront | / | mobile | zh-TW | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00858 | storefront | / | desktop | en | chrome | Privacy Policy | click | navigation or in-page change | url /en -> /en/legal/privacy; DOM changed (20 mutations) | PASS |
| r00859 | storefront | / | desktop | zh-TW | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r00860 | storefront | / | mobile | zh-TW | chrome | 隱私權政策 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/privacy; DOM changed (19 mutations) | PASS |
| r00861 | storefront | / | desktop | zh-TW | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r00862 | storefront | / | mobile | zh-TW | chrome | 服務條款 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/terms | PASS |
| r00863 | storefront | / | desktop | zh-TW | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r00864 | storefront | / | mobile | zh-TW | chrome | 退款、退貨與取消 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/legal/refunds (inline change only; no request fired) | PASS |
| r00865 | storefront | / | desktop | zh-TW | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r00866 | storefront | / | mobile | zh-TW | chrome | 運送 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/shipping | PASS |
| r00867 | storefront | / | desktop | zh-TW | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00868 | storefront | / | mobile | zh-TW | chrome | 聯絡我們 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/contact | PASS |
| r00869 | storefront | / | desktop | zh-TW | chrome | 隱私權政策 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/privacy; DOM changed (20 mutations) | PASS |
| r00870 | storefront | / | mobile | zh-TW | chrome | 防詐騙提醒 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/anti-fraud | PASS |
| r00871 | storefront | / | desktop | zh-TW | chrome | 服務條款 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/terms | PASS |
| r00872 | storefront | / | desktop | en | chrome | Terms of Service | click | navigation or in-page change | url /en -> /en/legal/terms | PASS |
| r00873 | storefront | / | mobile | zh-TW | chrome | 資料刪除 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/data-deletion (inline change only; no request fired) | PASS |
| r00874 | storefront | / | desktop | zh-TW | chrome | 退款、退貨與取消 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/legal/refunds (inline change only; no request fired) | PASS |
| r00875 | storefront | / | desktop | en | chrome | Refunds, Returns and Cancellation | click | navigation or in-page change | url /en -> /en/legal/refunds | PASS |
| r00876 | storefront | / | mobile | zh-TW | chrome | 简体中文 | click | navigation or in-page change | url /zh-TW -> /zh-CN | PASS |
| r00877 | storefront | / | desktop | zh-TW | chrome | 運送 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/shipping | PASS |
| r00878 | storefront | / | desktop | zh-TW | chrome | 聯絡我們 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/contact | PASS |
| r00879 | storefront | / | mobile | zh-TW | chrome | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r00880 | storefront | / | mobile | zh-TW | chrome | English | click | navigation or in-page change | url /zh-TW -> /en | PASS |
| r00881 | storefront | / | mobile | zh-TW | main | 查看全部 | click | navigation or in-page change | url /zh-TW -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00882 | storefront | / | desktop | en | chrome | Shipping | click | navigation or in-page change | url /en -> /en/legal/shipping | PASS |
| r00883 | storefront | / | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/t04-24a21ca87d63; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00884 | storefront | / | desktop | zh-TW | chrome | 防詐騙提醒 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/anti-fraud | PASS |
| r00885 | storefront | / | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00886 | storefront | / | desktop | zh-TW | chrome | 資料刪除 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/data-deletion (inline change only; no request fired) | PASS |
| r00887 | storefront | / | mobile | zh-TW | main | 查看全部 | click | navigation or in-page change | scrolled | PASS |
| r00888 | storefront | / | desktop | zh-TW | chrome | 简体中文 | click | navigation or in-page change | url /zh-TW -> /zh-CN | PASS |
| r00889 | storefront | / | mobile | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00890 | storefront | / | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00891 | storefront | / | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00892 | storefront | / | desktop | en | chrome | Contact | click | navigation or in-page change | url /en -> /en/legal/contact | PASS |
| r00893 | storefront | /products | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00894 | storefront | /products | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00895 | storefront | / | desktop | en | chrome | Shop safely | click | navigation or in-page change | url /en -> /en/legal/anti-fraud | PASS |
| r00896 | storefront | / | desktop | en | chrome | Data deletion | click | navigation or in-page change | url /en -> /en/data-deletion | PASS |
| r00897 | storefront | /products | mobile | zh-TW | main | 全部商品 | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00898 | storefront | / | desktop | en | chrome | 简体中文 | click | navigation or in-page change | url /en -> /zh-CN | PASS |
| r00899 | storefront | /products | mobile | zh-TW | main | Sweep home | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00900 | storefront | / | desktop | en | chrome | 繁體中文 | click | navigation or in-page change | url /en -> /zh-TW | PASS |
| r00901 | storefront | /products | mobile | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00902 | storefront | /products | mobile | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00903 | storefront | / | desktop | zh-TW | chrome | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r00904 | storefront | /products | mobile | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00905 | storefront | / | desktop | zh-TW | chrome | English | click | navigation or in-page change | url /zh-TW -> /en | PASS |
| r00906 | storefront | / | desktop | en | chrome | English | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r00907 | storefront | /products | mobile | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/products -> /zh-TW/products?min=&max=&sort=newest | PASS |
| r00908 | storefront | / | desktop | zh-TW | main | 查看全部 | click | navigation or in-page change | url /zh-TW -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00909 | storefront | /products | mobile | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00910 | storefront | / | desktop | en | main | View all | click | navigation or in-page change | url /en -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00911 | storefront | / | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/t04-24a21ca87d63; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00912 | storefront | /products | mobile | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00913 | storefront | / | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en -> /en/products/t04-24a21ca87d63; DOM changed (4 mutations) [1 of 2 alike] | PASS |
| r00914 | storefront | / | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00915 | storefront | /products | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00916 | storefront | / | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00917 | storefront | / | desktop | zh-TW | main | 查看全部 | click | navigation or in-page change | scrolled | PASS |
| r00918 | storefront | /products | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r00919 | storefront | / | desktop | en | main | View all | click | navigation or in-page change | scrolled | PASS |
| r00920 | storefront | / | desktop | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00921 | storefront | / | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00922 | storefront | /collections | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00923 | storefront | / | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00924 | storefront | /collections | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00925 | storefront | / | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00926 | storefront | / | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00927 | storefront | / | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00928 | storefront | /collections | mobile | zh-TW | main | S Sweep home 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00929 | storefront | /products | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00930 | storefront | /products | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00931 | storefront | /products | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00932 | storefront | /collections | mobile | zh-TW | main | S Sweep wear 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00933 | storefront | /products | desktop | en | main | Home | click | navigation or in-page change | url /en/products -> /en; DOM changed (4 mutations) | PASS |
| r00934 | storefront | /collections/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00935 | storefront | /products | desktop | zh-TW | main | 全部商品 | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00936 | storefront | /collections/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00937 | storefront | /products | desktop | en | main | All products | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00938 | storefront | /products | desktop | zh-TW | main | Sweep home | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00939 | storefront | /collections/[slug] | mobile | zh-TW | main | 商品分類 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections; DOM changed (4 mutations) | PASS |
| r00940 | storefront | /products | desktop | en | main | Sweep home | click | navigation or in-page change | url /en/products -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00941 | storefront | /products | desktop | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00942 | storefront | /collections/[slug] | mobile | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00943 | storefront | /products | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/products -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00944 | storefront | /products | desktop | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00945 | storefront | /products | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00946 | storefront | /collections/[slug] | mobile | zh-TW | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00947 | storefront | /products | desktop | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00948 | storefront | /collections/[slug] | mobile | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00949 | storefront | /products | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00950 | storefront | /collections/[slug] | mobile | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00951 | storefront | /products | desktop | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/products -> /zh-TW/products?min=&max=&sort=newest | PASS |
| r00952 | storefront | /products | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/products -> /en/products?min=&max=&sort=newest | PASS |
| r00953 | storefront | /products | desktop | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00954 | storefront | /collections/[slug] | mobile | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00955 | storefront | /products | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00956 | storefront | /products | desktop | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /zh-TW/products?min=&max=&sort=price_asc -> /zh-TW/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r00957 | storefront | /collections/[slug] | mobile | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-home?min=&max=&sort=newest | PASS |
| r00958 | storefront | /products | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /en/products?min=&max=&sort=price_asc -> /en/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r00959 | storefront | /products | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00960 | storefront | /products | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/products -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00961 | storefront | /products | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/t04-24a21ca87d63; DOM changed (5 mutations) | PASS |
| r00962 | storefront | /products | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/products -> /en/products/t04-24a21ca87d63; DOM changed (5 mutations) | PASS |
| r00963 | storefront | /collections | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00964 | storefront | /collections | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00965 | storefront | /collections | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00966 | storefront | /collections | desktop | en | main | Home | click | navigation or in-page change | url /en/collections -> /en; DOM changed (4 mutations) | PASS |
| r00967 | storefront | /collections | desktop | zh-TW | main | S Sweep home 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00968 | storefront | /collections | desktop | en | main | S Sweep home 2 products (`collection-tile`) | click | navigation or in-page change | url /en/collections -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00969 | storefront | /collections/[slug] | mobile | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00970 | storefront | /collections | desktop | zh-TW | main | S Sweep wear 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00971 | storefront | /collections | desktop | en | main | S Sweep wear 2 products (`collection-tile`) | click | navigation or in-page change | url /en/collections -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00972 | storefront | /collections/[slug] | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/collections/sweep-home?min=&max=&sort=price_asc -> /zh-TW/products/t04-24a21ca87d63; DOM changed (5 mutations) | PASS |
| r00973 | storefront | /collections/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00974 | storefront | /collections/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00975 | storefront | /collections/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00976 | storefront | /collections/[slug] | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00977 | storefront | /collections/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/collections/sweep-home -> /en; DOM changed (4 mutations) | PASS |
| r00978 | storefront | /products/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00979 | storefront | /collections/[slug] | desktop | zh-TW | main | 商品分類 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections; DOM changed (4 mutations) | PASS |
| r00980 | storefront | /collections/[slug] | desktop | en | main | Collections | click | navigation or in-page change | url /en/collections/sweep-home -> /en/collections; DOM changed (4 mutations) | PASS |
| r00981 | storefront | /products/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00982 | storefront | /collections/[slug] | desktop | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00983 | storefront | /collections/[slug] | desktop | en | main | All products | click | navigation or in-page change | url /en/collections/sweep-home -> /en/products; DOM changed (4 mutations) | PASS |
| r00984 | storefront | /products/[slug] | mobile | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00985 | storefront | /collections/[slug] | desktop | zh-TW | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00986 | storefront | /products/[slug] | mobile | zh-TW | main | 放大圖片 | click | visible change | layer opened (Sweep Wool Scarf — 放大圖片); DOM changed (2 mutations) | PASS |
| r00987 | storefront | /collections/[slug] | desktop | en | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00988 | storefront | /collections/[slug] | desktop | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00989 | storefront | /products/[slug] | mobile | zh-TW | main | 增加數量 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00990 | storefront | /collections/[slug] | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/collections/sweep-home -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00991 | storefront | /collections/[slug] | desktop | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00992 | storefront | /products/[slug] | mobile | zh-TW | main | 加入購物車 (`add-to-cart`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00993 | storefront | /collections/[slug] | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00994 | storefront | /products/[slug] | mobile | zh-TW | main | 立即購買 (`buy-now`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00995 | storefront | /collections/[slug] | desktop | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00996 | storefront | /collections/[slug] | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00997 | storefront | /products/[slug] | mobile | zh-TW | main | Sweep home | click | navigation or in-page change | scrolled | PASS |
| r00998 | storefront | /collections/[slug] | desktop | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-home?min=&max=&sort=newest | PASS |
| r00999 | storefront | /collections/[slug] | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/collections/sweep-home -> /en/collections/sweep-home?min=&max=&sort=newest | PASS |
| r01000 | storefront | /products/[slug] | mobile | zh-TW | main | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r01001 | storefront | /collections/[slug] | desktop | zh-TW | main | 排序 | selectOption(price_asc) | visible change | page reloaded | PASS |
| r01002 | storefront | /collections/[slug] | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01003 | storefront | /products/[slug] | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r01004 | storefront | /collections/[slug] | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products/t04-24a21ca87d63; DOM changed (5 mutations) | PASS |
| r01005 | storefront | /collections/[slug] | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/collections/sweep-home?min=&max=&sort=price_asc -> /en/products/t04-24a21ca87d63; DOM changed (5 mutations) | PASS |
| r01006 | storefront | /search | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01007 | storefront | /collections/[slug] | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01008 | storefront | /collections/[slug] | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/collections/sweep-home -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01009 | storefront | /search | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW; DOM changed (5 mutations) | PASS |
| r01010 | storefront | /products/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01011 | storefront | /products/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01012 | storefront | /products/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW; DOM changed (4 mutations) | PASS |
| r01013 | storefront | /products/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en; DOM changed (4 mutations) | PASS |
| r01014 | storefront | /search | mobile | zh-TW | main | 搜尋 | click | visible change | the control's own state changed ( -> gone) | PASS |
| r01015 | storefront | /products/[slug] | desktop | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r01016 | storefront | /search | mobile | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r01017 | storefront | /products/[slug] | desktop | en | main | All products | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/products; DOM changed (4 mutations) | PASS |
| r01018 | storefront | /products/[slug] | desktop | zh-TW | main | 放大圖片 | click | visible change | layer opened (Sweep Wool Scarf — 放大圖片); DOM changed (2 mutations) | PASS |
| r01019 | storefront | /products/[slug] | desktop | en | main | Enlarge photo | click | visible change | layer opened (Sweep Wool Scarf — Enlarge photo); DOM changed (2 mutations) | PASS |
| r01020 | storefront | /search | mobile | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r01021 | storefront | /products/[slug] | desktop | zh-TW | main | 增加數量 | click | visible change | DOM changed (2 mutations) | PASS |
| r01022 | storefront | /products/[slug] | desktop | en | main | Increase quantity | click | visible change | DOM changed (2 mutations) | PASS |
| r01023 | storefront | /search | mobile | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/search?q=Sweep -> /zh-TW/search?q=Sweep&min=&max=&sort=newest | PASS |
| r01024 | storefront | /products/[slug] | desktop | zh-TW | main | 加入購物車 (`add-to-cart`) | click | visible change | DOM changed (3 mutations) | PASS |
| r01025 | storefront | /products/[slug] | desktop | en | main | Add to cart (`add-to-cart`) | click | visible change | DOM changed (3 mutations) | PASS |
| r01026 | storefront | /search | mobile | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01027 | storefront | /search | mobile | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r01028 | storefront | /search | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01029 | storefront | /search | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r01030 | storefront | /orders/lookup | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01031 | storefront | /orders/lookup | mobile | zh-TW | main | 查詢 (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r01032 | storefront | /products/[slug] | desktop | zh-TW | main | 立即購買 (`buy-now`) | click | visible change | DOM changed (3 mutations) | PASS |
| r01033 | storefront | /products/[slug] | desktop | en | main | Buy now (`buy-now`) | click | visible change | DOM changed (3 mutations) | PASS |
| r01034 | storefront | /order-link | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01035 | storefront | /order-link | mobile | zh-TW | main | 查詢訂單 | click | navigation or in-page change | url /zh-TW/order-link -> /zh-TW/orders/lookup; DOM changed (12 mutations) | PASS |
| r01036 | storefront | /products/[slug] | desktop | en | main | Sweep home | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r01037 | storefront | /claim | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01038 | storefront | /claim | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r01039 | storefront | /products/[slug] | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r01040 | storefront | /legal/anti-fraud | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01041 | storefront | /products/[slug] | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r01042 | storefront | /legal/anti-fraud | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /zh-TW; DOM changed (5 mutations) | PASS |
| r01043 | storefront | /search | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01044 | storefront | /legal/anti-fraud | mobile | zh-TW | main | 內政部警政署 — 165 全民防騙網 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r01045 | storefront | /search | desktop | en | main | Home | click | navigation or in-page change | url /en/search?q=Sweep -> /en; DOM changed (4 mutations) | PASS |
| r01046 | storefront | /legal/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01047 | storefront | /search | desktop | en | main | Search | click | visible change | the control's own state changed ( -> gone) | PASS |
| r01048 | storefront | /privacy | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01049 | storefront | /privacy | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/privacy -> /zh-CN/privacy; DOM changed (30 mutations) | PASS |
| r01050 | storefront | /products/[slug] | desktop | zh-TW | main | Sweep home | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r01051 | storefront | /search | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r01052 | storefront | /privacy | mobile | zh-TW | main | 開啟: 允許店鋪透過 Messenger 或 Instagram 私訊向我發送行銷訊息。 (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01053 | storefront | /products/[slug] | desktop | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r01054 | storefront | /search | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r01055 | storefront | /products/[slug] | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r01056 | storefront | /search | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/search?q=Sweep -> /en/search?q=Sweep&min=&max=&sort=newest | PASS |
| r01057 | storefront | /privacy | mobile | zh-TW | main | 開啟: 使用我的購買紀錄為我個人化 Meta 廣告。 (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01058 | storefront | /search | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01059 | storefront | /search | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW; DOM changed (4 mutations) | PASS |
| r01060 | storefront | /search | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01061 | storefront | /search | desktop | zh-TW | main | 搜尋 | click | visible change | the control's own state changed ( -> gone) | PASS |
| r01062 | storefront | /search | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /en/search?q=Sweep&min=&max=&sort=price_asc -> /en/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r01063 | storefront | /search | desktop | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r01064 | storefront | /search | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/search?q=Sweep -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01065 | storefront | /search | desktop | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r01066 | storefront | /search | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/search?q=Sweep -> /en/products/t04-24a21ca87d63; DOM changed (5 mutations) | PASS |
| r01067 | storefront | /privacy | mobile | zh-TW | main | 下載我的資料 (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r01068 | storefront | /search | desktop | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/search?q=Sweep -> /zh-TW/search?q=Sweep&min=&max=&sort=newest | PASS |
| r01069 | storefront | /orders/lookup | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01070 | storefront | /orders/lookup | desktop | en | main | Find order (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r01071 | storefront | /search | desktop | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01072 | storefront | /order-link | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01073 | storefront | /order-link | desktop | en | main | Look up an order | click | navigation or in-page change | url /en/order-link -> /en/orders/lookup; DOM changed (12 mutations) | PASS |
| r01074 | storefront | /search | desktop | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /zh-TW/search?q=Sweep&min=&max=&sort=price_asc -> /zh-TW/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r01075 | storefront | /claim | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01076 | storefront | /claim | desktop | en | main | Language | selectOption(zh-CN) | visible change | url /en/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r01077 | storefront | /search | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01078 | storefront | /legal/anti-fraud | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01079 | storefront | /search | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW/products/t04-24a21ca87d63; DOM changed (5 mutations) | PASS |
| r01080 | storefront | /legal/anti-fraud | desktop | en | main | Home | click | navigation or in-page change | url /en/legal/anti-fraud -> /en; DOM changed (4 mutations) | PASS |
| r01081 | storefront | /orders/lookup | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01082 | storefront | /legal/anti-fraud | desktop | en | main | Taiwan National Police Agency — 165 anti-fraud service | click | navigation or in-page change | url /en/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r01083 | storefront | /privacy | mobile | zh-TW | main | 抹除我的資料… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r01084 | storefront | /orders/lookup | desktop | zh-TW | main | 查詢 (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r01085 | storefront | /legal/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01086 | storefront | /data-deletion | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01087 | storefront | /order-link | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01088 | storefront | /privacy | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01089 | storefront | /data-deletion | mobile | zh-TW | main | 简体中文 | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-CN/data-deletion | PASS |
| r01090 | storefront | /privacy | desktop | en | main | Language | selectOption(zh-CN) | visible change | url /en/privacy -> /zh-CN/privacy; DOM changed (31 mutations) | PASS |
| r01091 | storefront | /order-link | desktop | zh-TW | main | 查詢訂單 | click | navigation or in-page change | url /zh-TW/order-link -> /zh-TW/orders/lookup; DOM changed (12 mutations) | PASS |
| r01092 | storefront | /claim | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01093 | storefront | /privacy | desktop | en | main | Turn on: The store may send me marketing messages on Messenger or Instagram DM. (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01094 | storefront | /claim | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r01095 | storefront | /legal/anti-fraud | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01096 | storefront | /legal/anti-fraud | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /zh-TW; DOM changed (4 mutations) | PASS |
| r01097 | storefront | /legal/anti-fraud | desktop | zh-TW | main | 內政部警政署 — 165 全民防騙網 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r01098 | storefront | /legal/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01099 | storefront | /privacy | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01100 | storefront | /privacy | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/privacy -> /zh-CN/privacy; DOM changed (30 mutations) | PASS |
| r01101 | storefront | /privacy | desktop | zh-TW | main | 開啟: 允許店鋪透過 Messenger 或 Instagram 私訊向我發送行銷訊息。 (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01102 | storefront | /privacy | desktop | en | main | Turn on: Use my purchase to personalize Meta ads for me. (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01103 | storefront | /data-deletion | mobile | zh-TW | main | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r01104 | storefront | /data-deletion | mobile | zh-TW | main | English | click | navigation or in-page change | url /zh-TW/data-deletion -> /en/data-deletion | PASS |
| r01105 | storefront | /data-deletion | mobile | zh-TW | main | 開啟隱私頁面 (`data-deletion-privacy-link`) | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-TW/privacy | PASS |
| r01106 | storefront | /pages/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01107 | storefront | /pages/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/pages/about -> /zh-TW; DOM changed (5 mutations) | PASS |
| r01108 | storefront | /privacy | desktop | zh-TW | main | 開啟: 使用我的購買紀錄為我個人化 Meta 廣告。 (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01109 | storefront | /privacy | desktop | en | main | Download my data (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r01110 | storefront | /privacy | desktop | zh-TW | main | 下載我的資料 (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r01111 | storefront | /privacy | desktop | en | main | Erase my data… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations) (confirmation shown, then cancelled) | PASS |
| r01112 | storefront | /data-deletion | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01113 | storefront | /data-deletion | desktop | en | main | 简体中文 | click | navigation or in-page change | url /en/data-deletion -> /zh-CN/data-deletion | PASS |
| r01114 | storefront | /data-deletion | desktop | en | main | 繁體中文 | click | navigation or in-page change | url /en/data-deletion -> /zh-TW/data-deletion | PASS |
| r01115 | storefront | /privacy | desktop | zh-TW | main | 抹除我的資料… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations) (confirmation shown, then cancelled) | PASS |
| r01116 | storefront | /data-deletion | desktop | en | main | English | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r01117 | storefront | /data-deletion | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01118 | storefront | /data-deletion | desktop | zh-TW | main | 简体中文 | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-CN/data-deletion | PASS |
| r01119 | storefront | /data-deletion | desktop | en | main | Open the privacy page (`data-deletion-privacy-link`) | click | navigation or in-page change | url /en/data-deletion -> /en/privacy | PASS |
| r01120 | storefront | /pages/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01121 | storefront | /pages/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/pages/about -> /en; DOM changed (4 mutations) | PASS |
| r01122 | storefront | /data-deletion | desktop | zh-TW | main | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r01123 | storefront | /data-deletion | desktop | zh-TW | main | English | click | navigation or in-page change | url /zh-TW/data-deletion -> /en/data-deletion | PASS |
| r01124 | storefront | /data-deletion | desktop | zh-TW | main | 開啟隱私頁面 (`data-deletion-privacy-link`) | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-TW/privacy | PASS |
| r01125 | storefront | /pages/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01126 | storefront | /pages/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/pages/about -> /zh-TW; DOM changed (4 mutations) | PASS |
| r01127 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | open the product list and click New product | real clicks | the create form opens | as expected | PASS |
| r01128 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | fill the name and description | real clicks | the draft fields retain the entered values | as expected | PASS |
| r01129 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | add Size = S, M | real clicks | the editor proposes two SKU rows | as expected | PASS |
| r01130 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | type both SKU prices | real clicks | the matrix retains both prices | as expected | PASS |
| r01131 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | set opening quantities 9 and 7, then save the document | real clicks | both SKU quantities persist | as expected | PASS |
| r01132 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | upload a cover, set Active and save | real clicks | the product is active and persists after a reload | as expected | PASS |
| r01133 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | the merchant list shows the product (search + click) | real clicks | the row is listed with status active | as expected | PASS |
| r01134 | journey | J1 storefront | desktop | zh-TW | journey | the buyer opens All products and clicks the new product | real clicks | its page opens with the title and an enabled Add to cart | as expected | PASS |
| r01135 | journey | J2/J3 admin orders | desktop | zh-TW | journey | admin Orders: the storefront COD order is listed (click through pages) | real clicks | the row of the order id can be expanded | as expected | PASS |
| r01136 | journey | J2/J3 admin orders | desktop | zh-TW | journey | record the manual shipment (carrier Black Cat + tracking) and click Record | real clicks | the shipment record is shown | as expected | PASS |
| r01137 | journey | J2/J3 admin orders | desktop | zh-TW | journey | click Collected, confirm in the dialog | real clicks | the order shows COLLECTED and the button is gone | as expected | PASS |
| r01138 | journey | J2/J3 admin orders | desktop | zh-TW | journey | reload the order list: collected persists | real clicks | COLLECTED is still shown after a reload | as expected | PASS |
| r01139 | journey | J2 storefront order lookup | desktop | zh-TW | journey | a fresh buyer browser looks the order up (order id + phone) and sees it collected | real clicks | the lookup shows the order with the collected amount | as expected | PASS |
| r01140 | journey | J4 custom domain | desktop | zh-TW | journey | open Settings and find the custom domain card | real clicks | the storefront domains card is shown | as expected | PASS |
| r01141 | journey | J4 custom domain | desktop | zh-TW | journey | type a custom hostname and click Request | real clicks | DNS instructions appear: a TXT name, a TXT value and the CNAME target | as expected | PASS |
| r01142 | journey | J4 custom domain | desktop | zh-TW | journey | reload Settings: the requested domain is listed | real clicks | the domain row persists in state REQUESTED | as expected | PASS |
| r01143 | journey | J5 sign out | desktop | zh-TW | journey | open the dashboard, click Sign out | real clicks | the session ends: the page asks to sign in again | as expected | PASS |
| r01144 | journey | J5 sign out | desktop | zh-TW | journey | reload after sign out | real clicks | the dashboard is not shown without a session | as expected | PASS |
| r01145 | storefront | /live | - | - | page | (not implemented) | - | ui-architecture section 4 lists /live (S6) | apps/storefront/app/[locale] has no live route in this base; nothing to click | SKIP |
