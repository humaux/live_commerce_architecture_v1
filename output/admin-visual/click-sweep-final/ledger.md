# G-UI8 click ledger

Generated 2026-10-05T20:38:09.971Z. 123 page/viewport/locale units opened (0 with a page-load failure), 978 control clicks (954 pass, 0 fail, 24 skip), 18 journey steps (18 pass, 0 fail); failures: known 0, new 0; stale known-defect entries 0.

Every row is one real Playwright interaction (click / selectOption). Destructive and irreversible controls stop at their confirmation and are cancelled; `skip` rows name why.

| # | App | Page | Viewport | Locale | Scope | Control | Action | Expected | Actual | Result |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| r00001 | admin | / | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00002 | admin | / | desktop | zh-TW | skip | 跳至內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f#main; scrolled | PASS |
| r00003 | admin | / | desktop | zh-TW | topbar | 語言 (`locale-switch`) | selectOption(zh-CN) | visible change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-CN?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (16 mutations) | PASS |
| r00004 | admin | / | desktop | zh-TW | topbar | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00005 | admin | / | desktop | zh-TW | layer of 說明 | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00006 | admin | / | desktop | zh-TW | topbar | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00007 | admin | / | desktop | zh-TW | layer of 帳號 | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00008 | admin | / | desktop | zh-TW | layer of 帳號 | 登出 (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00009 | admin | / | desktop | zh-TW | rail | 總覽 (`nav-group-overview`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00010 | admin | / | desktop | zh-TW | rail | 直播與貼文 (`nav-group-live`) | click | visible change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/studio?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (17 mutations) | PASS |
| r00011 | admin | / | desktop | zh-TW | rail | 訂單與出貨 (`nav-orders`) | click | visible change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (18 mutations) | PASS |
| r00012 | admin | / | desktop | zh-TW | rail | 商品與庫存 + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00013 | admin | / | desktop | zh-TW | rail | 顧客 (`nav-group-customers`) | click | visible change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00014 | admin | / | desktop | zh-TW | rail | 行銷優惠 + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00015 | admin | / | desktop | zh-TW | rail | 網路商店 (`nav-group-storefront`) | click | visible change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/design?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00016 | admin | / | desktop | zh-TW | rail | 收款與報表 (`nav-group-finance`) | click | visible change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/finance?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00017 | admin | / | desktop | zh-TW | rail | 設定 + (`nav-group-settings`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00018 | admin | / | desktop | zh-TW | main | 建立訂單 (`action-create-order`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders/new?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00019 | admin | / | desktop | zh-TW | main | 匯入或匯出商品 (`action-import`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/import?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00020 | admin | / | desktop | zh-TW | main | 庫存 (`action-inventory`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00021 | admin | / | desktop | zh-TW | main | 待確認的轉帳 1 (`todo-transfer`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?state=AWAITING_TRANSFER&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutat | PASS |
| r00022 | admin | / | desktop | zh-TW | main | 待出貨訂單 2 (`todo-ship`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?state=unshipped&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00023 | admin | / | desktop | zh-TW | main | 待建立的超商托運單 1 (`todo-label`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?state=unshipped&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00024 | admin | / | desktop | zh-TW | main | 低庫存規格（5 件以下） 0 (`todo-stock`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00025 | admin | / | desktop | zh-TW | main | 處理中的退款 0 (`todo-refunds`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) (inline change only; no request fired) | PASS |
| r00026 | admin | / | desktop | zh-TW | main | 0f8728c3 | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?order=0f8728c3-37ba-466f-af01-3d8a906c963b&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DO | PASS |
| r00027 | admin | / | desktop | zh-TW | main | 5ec9930d | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?order=5ec9930d-1b62-4585-8c5f-1e660059bcf1&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DO | PASS |
| r00028 | admin | / | desktop | zh-TW | main | 23306d22 | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?order=23306d22-5a4b-47ca-8f53-41a16e5f7f72&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DO | PASS |
| r00029 | admin | / | desktop | zh-TW | main | ff529d66 | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?order=ff529d66-e17a-4eb1-92cc-f26d7fd245fd&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DO | PASS |
| r00030 | admin | / | desktop | zh-TW | main | 28460b6c | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?order=28460b6c-6aee-432c-a18d-5689c5f5e289&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DO | PASS |
| r00031 | admin | / | desktop | zh-TW | main | 14843968 | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?order=14843968-3f1a-413f-8d30-7b8f843b481f&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DO | PASS |
| r00032 | admin | / | desktop | zh-TW | main | 全部訂單 | click | navigation or in-page change | scrolled | PASS |
| r00033 | admin | / | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00034 | admin | / | mobile | zh-TW | skip | 跳至內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f#main; scrolled | PASS |
| r00035 | admin | / | mobile | zh-TW | topbar | 開啟導覽 | click | visible change | DOM changed (3 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00036 | admin | / | mobile | zh-TW | topbar | 語言 (`locale-switch`) | selectOption(zh-CN) | visible change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-CN?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (17 mutations) | PASS |
| r00037 | admin | / | mobile | zh-TW | topbar | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00038 | admin | / | mobile | zh-TW | layer of 說明 | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00039 | admin | / | mobile | zh-TW | topbar | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00040 | admin | / | mobile | zh-TW | layer of 帳號 | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00041 | admin | / | mobile | zh-TW | layer of 帳號 | 登出 (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00042 | admin | / | mobile | zh-TW | rail | 關閉導覽 | click | visible change | DOM changed (3 mutations) | PASS |
| r00043 | admin | / | mobile | zh-TW | rail | 總覽 (`nav-group-overview`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00044 | admin | / | mobile | zh-TW | rail | 直播與貼文 (`nav-group-live`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00045 | admin | / | mobile | zh-TW | rail | 訂單與出貨 (`nav-orders`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00046 | admin | / | mobile | zh-TW | rail | 商品與庫存 + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00047 | admin | / | mobile | zh-TW | rail | 顧客 (`nav-group-customers`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00048 | admin | / | mobile | zh-TW | rail | 行銷優惠 + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00049 | admin | / | mobile | zh-TW | rail | 網路商店 (`nav-group-storefront`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00050 | admin | / | mobile | zh-TW | rail | 收款與報表 (`nav-group-finance`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00051 | admin | / | mobile | zh-TW | rail | 設定 + (`nav-group-settings`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00052 | admin | / | mobile | zh-TW | main | 建立訂單 (`action-create-order`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders/new?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00053 | admin | / | mobile | zh-TW | main | 匯入或匯出商品 (`action-import`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/import?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00054 | admin | / | mobile | zh-TW | main | 庫存 (`action-inventory`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (16 mutations) | PASS |
| r00055 | admin | / | mobile | zh-TW | main | 待確認的轉帳 1 (`todo-transfer`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?state=AWAITING_TRANSFER&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (11 mutat | PASS |
| r00056 | admin | / | mobile | zh-TW | main | 待出貨訂單 2 (`todo-ship`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?state=unshipped&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (19 mutations) | PASS |
| r00057 | admin | / | mobile | zh-TW | main | 待建立的超商托運單 1 (`todo-label`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?state=unshipped&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (11 mutations) | PASS |
| r00058 | admin | / | mobile | zh-TW | main | 低庫存規格（5 件以下） 0 (`todo-stock`) | click | navigation or in-page change | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (16 mutations) | PASS |
| r00059 | admin | / | mobile | zh-TW | main | 處理中的退款 0 (`todo-refunds`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (11 mutations) (inline change only; no request fired) | PASS |
| r00060 | admin | / | mobile | zh-TW | main | 0f8728c3 | click | navigation or in-page change | scrolled | PASS |
| r00061 | admin | / | mobile | zh-TW | main | 5ec9930d | click | navigation or in-page change | scrolled | PASS |
| r00062 | admin | / | mobile | zh-TW | main | 23306d22 | click | navigation or in-page change | scrolled | PASS |
| r00063 | admin | / | mobile | zh-TW | main | ff529d66 | click | navigation or in-page change | scrolled | PASS |
| r00064 | admin | / | mobile | zh-TW | main | 28460b6c | click | navigation or in-page change | scrolled | PASS |
| r00065 | admin | / | mobile | zh-TW | main | 14843968 | click | navigation or in-page change | scrolled | PASS |
| r00066 | admin | / | mobile | zh-TW | main | 全部訂單 | click | navigation or in-page change | scrolled | PASS |
| r00067 | admin | / | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00068 | admin | / | desktop | en | skip | Skip to content | keyboard: Tab to the link, Enter | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f#main; scrolled | PASS |
| r00069 | admin | / | desktop | en | topbar | Language (`locale-switch`) | selectOption(zh-CN) | visible change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-CN?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (16 mutations) | PASS |
| r00070 | admin | / | desktop | en | topbar | Help | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00071 | admin | / | desktop | en | layer of Help | Help | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00072 | admin | / | desktop | en | topbar | Account | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00073 | admin | / | desktop | en | layer of Account | Account | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00074 | admin | / | desktop | en | layer of Account | Sign out (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00075 | admin | / | desktop | en | rail | Overview (`nav-group-overview`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00076 | admin | / | desktop | en | rail | Live & posts (`nav-group-live`) | click | visible change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/studio?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (17 mutations) | PASS |
| r00077 | admin | / | desktop | en | rail | Orders & shipping (`nav-orders`) | click | visible change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (18 mutations) | PASS |
| r00078 | admin | / | desktop | en | rail | Products & inventory + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00079 | admin | / | desktop | en | rail | Customers (`nav-group-customers`) | click | visible change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00080 | admin | / | desktop | en | rail | Marketing + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00081 | admin | / | desktop | en | rail | Online store (`nav-group-storefront`) | click | visible change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/design?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (12 mutations) | PASS |
| r00082 | admin | / | desktop | en | rail | Payments & reports (`nav-group-finance`) | click | visible change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/finance?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00083 | admin | / | desktop | en | rail | Settings + (`nav-group-settings`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00084 | admin | / | desktop | en | main | Create an order (`action-create-order`) | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders/new?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00085 | admin | / | desktop | en | main | Import or export products (`action-import`) | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products/import?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00086 | admin | / | desktop | en | main | Inventory (`action-inventory`) | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00087 | admin | / | desktop | en | main | Transfers to confirm 1 (`todo-transfer`) | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?state=AWAITING_TRANSFER&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00088 | admin | / | desktop | en | main | Orders to ship 2 (`todo-ship`) | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?state=unshipped&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (18 mutations) | PASS |
| r00089 | admin | / | desktop | en | main | Convenience-store labels to create 1 (`todo-label`) | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?state=unshipped&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00090 | admin | / | desktop | en | main | Low-stock variants (5 or fewer) 0 (`todo-stock`) | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00091 | admin | / | desktop | en | main | Open refunds 0 (`todo-refunds`) | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00092 | admin | / | desktop | en | main | 0f8728c3 | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?order=0f8728c3-37ba-466f-af01-3d8a906c963b&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM chan | PASS |
| r00093 | admin | / | desktop | en | main | 5ec9930d | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?order=5ec9930d-1b62-4585-8c5f-1e660059bcf1&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM chan | PASS |
| r00094 | admin | / | desktop | en | main | 23306d22 | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?order=23306d22-5a4b-47ca-8f53-41a16e5f7f72&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM chan | PASS |
| r00095 | admin | / | desktop | en | main | ff529d66 | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?order=ff529d66-e17a-4eb1-92cc-f26d7fd245fd&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM chan | PASS |
| r00096 | admin | / | desktop | en | main | 28460b6c | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?order=28460b6c-6aee-432c-a18d-5689c5f5e289&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM chan | PASS |
| r00097 | admin | / | desktop | en | main | 14843968 | click | navigation or in-page change | url /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?order=14843968-3f1a-413f-8d30-7b8f843b481f&store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM chan | PASS |
| r00098 | admin | / | desktop | en | main | All orders | click | navigation or in-page change | scrolled | PASS |
| r00099 | admin | /studio | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00100 | admin | /studio | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00101 | admin | /studio | desktop | zh-TW | main | 留言關鍵字登記 (`studio-open-claims`) | click | visible change | url /zh-TW/studio?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/studio/claims?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&scene=7ac5820f-a994-4d31-867f-90 | PASS |
| r00102 | admin | /studio | desktop | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (22 mutations) | PASS |
| r00103 | admin | /studio | desktop | zh-TW | main | ＋ 新增場次 | click | visible change | DOM changed (9 mutations) | PASS |
| r00104 | admin | /studio | desktop | zh-TW | main | Click sweep scene 44442c943fd9 草稿 預定時間（台北時間，選填） | click | visible change | DOM changed (6 mutations) | PASS |
| r00105 | admin | /studio | desktop | zh-TW | main | 畫面比例 | selectOption(16:9) | visible change | DOM changed (3 mutations); the control's own state changed (9:16 -> 16:9) | PASS |
| r00106 | admin | /studio | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00107 | admin | /studio | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (11 mutations) | PASS |
| r00108 | admin | /studio | mobile | zh-TW | main | 留言關鍵字登記 (`studio-open-claims`) | click | visible change | url /zh-TW/studio?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/studio/claims?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&scene=7ac5820f-a994-4d31-867f-90 | PASS |
| r00109 | admin | /studio | mobile | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (22 mutations) | PASS |
| r00110 | admin | /studio | mobile | zh-TW | main | ＋ 新增場次 | click | visible change | DOM changed (9 mutations) | PASS |
| r00111 | admin | /studio | mobile | zh-TW | main | Click sweep scene 44442c943fd9 草稿 預定時間（台北時間，選填） | click | visible change | DOM changed (6 mutations) | PASS |
| r00112 | admin | /studio | mobile | zh-TW | main | 畫面比例 | selectOption(16:9) | visible change | DOM changed (3 mutations); the control's own state changed (9:16 -> 16:9); scrolled | PASS |
| r00113 | admin | /studio | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00114 | admin | /studio | desktop | en | main | Overview | click | navigation or in-page change | url /en/studio?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00115 | admin | /studio | desktop | en | main | Keyword claims (`studio-open-claims`) | click | visible change | url /en/studio?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/studio/claims?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&scene=7ac5820f-a994-4d31-867f-9018da06 | PASS |
| r00116 | admin | /studio | desktop | en | main | Refresh facts | click | visible change | DOM changed (22 mutations) | PASS |
| r00117 | admin | /studio | desktop | en | main | ＋ New scene | click | visible change | DOM changed (9 mutations) | PASS |
| r00118 | admin | /studio | desktop | en | main | Click sweep scene 44442c943fd9 Draft Scheduled time (Taipei time, optional) | click | visible change | DOM changed (6 mutations) | PASS |
| r00119 | admin | /studio | desktop | en | main | Canvas ratio | selectOption(16:9) | visible change | DOM changed (3 mutations); the control's own state changed (9:16 -> 16:9) | PASS |
| r00120 | admin | /studio/claims | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00121 | admin | /studio/claims | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio/claims?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&scene=7ac5820f-a994-4d31-867f-9018da062f6e -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360d | PASS |
| r00122 | admin | /studio/claims | desktop | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (5 mutations) | PASS |
| r00123 | admin | /studio/claims | desktop | zh-TW | main | 數量規則 | selectOption(KEYWORD_QTY_ONLY) | visible change | the control's own state changed (EXACT -> KEYWORD_QTY_ONLY) | PASS |
| r00124 | admin | /studio/claims | desktop | zh-TW | main | 開放登記窗口 | click | visible change | DOM changed (21 mutations) | PASS |
| r00125 | admin | /studio/claims | desktop | zh-TW | main | 本輪 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00126 | admin | /studio/claims | desktop | zh-TW | layer of 本輪 | 本輪 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00127 | admin | /studio/claims | desktop | zh-TW | main | 這則貼文或直播所屬的專頁 | selectOption(6558492533161810405) | visible change | the control's own state changed ( -> 6558492533161810405) | PASS |
| r00128 | admin | /studio/claims | desktop | zh-TW | main | 平台 | selectOption(facebook) | visible change | the control's own state changed ( -> facebook) | PASS |
| r00129 | admin | /studio/claims | desktop | zh-TW | main | 回覆語言 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN) | PASS |
| r00130 | admin | /studio/claims | desktop | zh-TW | main | 傳送附帶購物車連結的私訊回覆 | click (toggle) | visible change | the control's own state changed (false -> true) | PASS |
| r00131 | admin | /studio/claims | desktop | zh-TW | main | 從此來源收集留言 | click (toggle) | visible change | the control's own state changed (true -> false) | PASS |
| r00132 | admin | /studio/claims | desktop | zh-TW | main | 儲存留言來源 | click | visible change | DOM changed (2 mutations); validation shown | PASS |
| r00133 | admin | /studio/claims | desktop | zh-TW | main | 商品 | selectOption(470f0853-d5c3-4575-82b4-25071fbecf9d) | visible change | DOM changed (1 mutations); the control's own state changed ( -> 470f0853-d5c3-4575-82b4-25071fbecf9d); scrolled | PASS |
| r00134 | admin | /studio/claims | desktop | zh-TW | main | 新增關鍵字 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00135 | admin | /studio/claims | desktop | zh-TW | main | 加入關鍵字庫 (`claims-add-library`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00136 | admin | /studio/claims | desktop | zh-TW | main | 買家 | selectOption | visible change | select has a single option | SKIP |
| r00137 | admin | /studio/claims | desktop | zh-TW | main | 記錄留言 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00138 | admin | /studio/claims | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00139 | admin | /studio/claims | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/studio/claims?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&scene=7ac5820f-a994-4d31-867f-9018da062f6e -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360d | PASS |
| r00140 | admin | /studio/claims | mobile | zh-TW | main | 重新整理事實 | click | visible change | DOM changed (2 mutations) | PASS |
| r00141 | admin | /studio/claims | mobile | zh-TW | main | 數量規則 | selectOption(KEYWORD_QTY_ONLY) | visible change | the control's own state changed (EXACT -> KEYWORD_QTY_ONLY) | PASS |
| r00142 | admin | /studio/claims | mobile | zh-TW | main | 開放登記窗口 | click | visible change | DOM changed (21 mutations) | PASS |
| r00143 | admin | /studio/claims | mobile | zh-TW | main | 本輪 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00144 | admin | /studio/claims | mobile | zh-TW | layer of 本輪 | 本輪 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00145 | admin | /studio/claims | mobile | zh-TW | main | 這則貼文或直播所屬的專頁 | selectOption(6558492533161810405) | visible change | the control's own state changed ( -> 6558492533161810405); scrolled | PASS |
| r00146 | admin | /studio/claims | mobile | zh-TW | main | 平台 | selectOption(facebook) | visible change | the control's own state changed ( -> facebook); scrolled | PASS |
| r00147 | admin | /studio/claims | mobile | zh-TW | main | 回覆語言 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN); scrolled | PASS |
| r00148 | admin | /studio/claims | mobile | zh-TW | main | 傳送附帶購物車連結的私訊回覆 | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00149 | admin | /studio/claims | mobile | zh-TW | main | 從此來源收集留言 | click (toggle) | visible change | the control's own state changed (true -> false); scrolled | PASS |
| r00150 | admin | /studio/claims | mobile | zh-TW | main | 儲存留言來源 | click | visible change | DOM changed (2 mutations); validation shown; scrolled | PASS |
| r00151 | admin | /studio/claims | mobile | zh-TW | main | 商品 | selectOption(470f0853-d5c3-4575-82b4-25071fbecf9d) | visible change | DOM changed (1 mutations); the control's own state changed ( -> 470f0853-d5c3-4575-82b4-25071fbecf9d); scrolled | PASS |
| r00152 | admin | /studio/claims | mobile | zh-TW | main | 新增關鍵字 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00153 | admin | /studio/claims | mobile | zh-TW | main | 加入關鍵字庫 (`claims-add-library`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00154 | admin | /studio/claims | mobile | zh-TW | main | 買家 | selectOption | visible change | select has a single option | SKIP |
| r00155 | admin | /studio/claims | mobile | zh-TW | main | 記錄留言 | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00156 | admin | /studio/claims | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00157 | admin | /studio/claims | desktop | en | main | Overview | click | navigation or in-page change | url /en/studio/claims?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&scene=7ac5820f-a994-4d31-867f-9018da062f6e -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; D | PASS |
| r00158 | admin | /studio/claims | desktop | en | main | Refresh facts | click | visible change | DOM changed (2 mutations) | PASS |
| r00159 | admin | /studio/claims | desktop | en | main | Quantity rule | selectOption(KEYWORD_QTY_ONLY) | visible change | the control's own state changed (EXACT -> KEYWORD_QTY_ONLY) | PASS |
| r00160 | admin | /studio/claims | desktop | en | main | Open claim window | click | visible change | DOM changed (21 mutations) | PASS |
| r00161 | admin | /studio/claims | desktop | en | main | This round | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00162 | admin | /studio/claims | desktop | en | layer of This round | This round | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00163 | admin | /studio/claims | desktop | en | main | Page for this post or live stream | selectOption(6558492533161810405) | visible change | the control's own state changed ( -> 6558492533161810405) | PASS |
| r00164 | admin | /studio/claims | desktop | en | main | Platform | selectOption(facebook) | visible change | the control's own state changed ( -> facebook) | PASS |
| r00165 | admin | /studio/claims | desktop | en | main | Reply language | selectOption(zh-CN) | visible change | the control's own state changed (en -> zh-CN) | PASS |
| r00166 | admin | /studio/claims | desktop | en | main | Send a private reply with the cart link | click (toggle) | visible change | the control's own state changed (false -> true) | PASS |
| r00167 | admin | /studio/claims | desktop | en | main | Collect comments from this source | click (toggle) | visible change | the control's own state changed (true -> false) | PASS |
| r00168 | admin | /studio/claims | desktop | en | main | Save comment source | click | visible change | DOM changed (2 mutations); validation shown | PASS |
| r00169 | admin | /studio/claims | desktop | en | main | Product | selectOption(470f0853-d5c3-4575-82b4-25071fbecf9d) | visible change | DOM changed (1 mutations); the control's own state changed ( -> 470f0853-d5c3-4575-82b4-25071fbecf9d); scrolled | PASS |
| r00170 | admin | /studio/claims | desktop | en | main | Add offer | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00171 | admin | /studio/claims | desktop | en | main | Add to library (`claims-add-library`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00172 | admin | /studio/claims | desktop | en | main | Buyer | selectOption | visible change | select has a single option | SKIP |
| r00173 | admin | /studio/claims | desktop | en | main | Record comment | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00174 | admin | /orders | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00175 | admin | /orders | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00176 | admin | /orders | desktop | zh-TW | main | 更多篩選 (`orders-more-filters`) | click | visible change | DOM changed (3 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00177 | admin | /orders | desktop | zh-TW | main | 重新整理 (`orders-refresh`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00178 | admin | /orders | desktop | zh-TW | main | 匯出未出貨訂單（CSV） (`orders-export`) | click | navigation or in-page change | download: unshipped-8daf0ef2-202610052011.csv | PASS |
| r00179 | admin | /orders | desktop | zh-TW | main | 套用篩選 (`orders-apply`) | click | visible change | DOM changed (22 mutations) | PASS |
| r00180 | admin | /orders | desktop | zh-TW | main | 清除篩選 (`orders-reset`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (14 mutations) (inline change only; no request fired) | PASS |
| r00181 | admin | /orders | desktop | zh-TW | main | 全部 5 (`orders-bucket-all`) | click | visible change | DOM changed (11 mutations); the control's own state changed (\|\|\|\|true -> gone) | PASS |
| r00182 | admin | /orders | desktop | zh-TW | main | 待付款 4 (`orders-bucket-unpaid`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00183 | admin | /orders | desktop | zh-TW | main | 待核對轉帳 1 (`orders-bucket-transfer_review`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00184 | admin | /orders | desktop | zh-TW | main | 待出貨 2 (`orders-bucket-ready_to_ship`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00185 | admin | /orders | desktop | zh-TW | main | 待交寄 1 (`orders-bucket-ready_to_consign`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00186 | admin | /orders | desktop | zh-TW | main | 已出貨 0 (`orders-bucket-shipped`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00187 | admin | /orders | desktop | zh-TW | main | 已完成 0 (`orders-bucket-completed`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00188 | admin | /orders | desktop | zh-TW | main | 已取消 0 (`orders-bucket-cancelled`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00189 | admin | /orders | desktop | zh-TW | main | 展開訂單詳情: 0f8728c3-37ba-466f-af01-3d8a906c963b (`order-expand-0f8728c3-37ba-466f-af01-3d8a906c963b`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00190 | admin | /orders | desktop | zh-TW | main | 展開訂單詳情: 5ec9930d-1b62-4585-8c5f-1e660059bcf1 (`order-expand-5ec9930d-1b62-4585-8c5f-1e660059bcf1`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00191 | admin | /orders | desktop | zh-TW | main | 展開訂單詳情: 28460b6c-6aee-432c-a18d-5689c5f5e289 (`order-expand-28460b6c-6aee-432c-a18d-5689c5f5e289`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00192 | admin | /orders | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00193 | admin | /orders | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (11 mutations) | PASS |
| r00194 | admin | /orders | mobile | zh-TW | main | 更多篩選 (`orders-more-filters`) | click | visible change | DOM changed (3 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00195 | admin | /orders | mobile | zh-TW | main | 重新整理 (`orders-refresh`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00196 | admin | /orders | mobile | zh-TW | main | 匯出未出貨訂單（CSV） (`orders-export`) | click | navigation or in-page change | download: unshipped-8daf0ef2-202610052011.csv | PASS |
| r00197 | admin | /orders | mobile | zh-TW | main | 套用篩選 (`orders-apply`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00198 | admin | /orders | mobile | zh-TW | main | 清除篩選 (`orders-reset`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (22 mutations) (inline change only; no request fired) | PASS |
| r00199 | admin | /orders | mobile | zh-TW | main | 全部 5 (`orders-bucket-all`) | click | visible change | DOM changed (22 mutations) | PASS |
| r00200 | admin | /orders | mobile | zh-TW | main | 待付款 4 (`orders-bucket-unpaid`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00201 | admin | /orders | mobile | zh-TW | main | 待核對轉帳 1 (`orders-bucket-transfer_review`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00202 | admin | /orders | mobile | zh-TW | main | 待出貨 2 (`orders-bucket-ready_to_ship`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00203 | admin | /orders | mobile | zh-TW | main | 待交寄 1 (`orders-bucket-ready_to_consign`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00204 | admin | /orders | mobile | zh-TW | main | 展開訂單詳情: 0f8728c3-37ba-466f-af01-3d8a906c963b (`order-expand-0f8728c3-37ba-466f-af01-3d8a906c963b`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00205 | admin | /orders | mobile | zh-TW | main | 展開訂單詳情: 5ec9930d-1b62-4585-8c5f-1e660059bcf1 (`order-expand-5ec9930d-1b62-4585-8c5f-1e660059bcf1`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00206 | admin | /orders | mobile | zh-TW | main | 展開訂單詳情: 28460b6c-6aee-432c-a18d-5689c5f5e289 (`order-expand-28460b6c-6aee-432c-a18d-5689c5f5e289`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00207 | admin | /orders | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00208 | admin | /orders | desktop | en | main | Overview | click | navigation or in-page change | url /en/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00209 | admin | /orders | desktop | en | main | More filters (`orders-more-filters`) | click | visible change | DOM changed (3 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00210 | admin | /orders | desktop | en | main | Refresh (`orders-refresh`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00211 | admin | /orders | desktop | en | main | Export unshipped (CSV) (`orders-export`) | click | navigation or in-page change | download: unshipped-8daf0ef2-202610052012.csv | PASS |
| r00212 | admin | /orders | desktop | en | main | Apply filters (`orders-apply`) | click | visible change | DOM changed (11 mutations) | PASS |
| r00213 | admin | /orders | desktop | en | main | Clear filters (`orders-reset`) | click | visible change | DOM changed (11 mutations); the control's own state changed ( -> gone) | PASS |
| r00214 | admin | /orders | desktop | en | main | All 5 (`orders-bucket-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00215 | admin | /orders | desktop | en | main | Awaiting payment 4 (`orders-bucket-unpaid`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00216 | admin | /orders | desktop | en | main | Review transfers 1 (`orders-bucket-transfer_review`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00217 | admin | /orders | desktop | en | main | Ready to ship 2 (`orders-bucket-ready_to_ship`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00218 | admin | /orders | desktop | en | main | Ready to drop off 1 (`orders-bucket-ready_to_consign`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00219 | admin | /orders | desktop | en | main | Shipped 0 (`orders-bucket-shipped`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00220 | admin | /orders | desktop | en | main | Completed 0 (`orders-bucket-completed`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|\|\|\|false -> gone) | PASS |
| r00221 | admin | /orders | desktop | en | main | Cancelled 0 (`orders-bucket-cancelled`) | click | visible change | url /en/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&bucket=cancelled; DOM changed (13 mutations) | PASS |
| r00222 | admin | /orders | desktop | en | main | Show order details: 0f8728c3-37ba-466f-af01-3d8a906c963b (`order-expand-0f8728c3-37ba-466f-af01-3d8a906c963b`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00223 | admin | /orders | desktop | en | main | Show order details: 5ec9930d-1b62-4585-8c5f-1e660059bcf1 (`order-expand-5ec9930d-1b62-4585-8c5f-1e660059bcf1`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00224 | admin | /orders | desktop | en | main | Show order details: 28460b6c-6aee-432c-a18d-5689c5f5e289 (`order-expand-28460b6c-6aee-432c-a18d-5689c5f5e289`) | click | visible change | DOM changed (8 mutations); the control's own state changed (\|false -> gone) [1 of 5 alike] | PASS |
| r00225 | admin | /orders/new | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00226 | admin | /orders/new | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders/new?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00227 | admin | /orders/new | desktop | zh-TW | main | 搜尋 (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00228 | admin | /orders/new | desktop | zh-TW | main | 配送方式 (`mo-option`) | selectOption(241ddae6-8530-43d0-8579-edc4d7a91dab\|TW\|cvs4ff4b5d392a5) | visible change | DOM changed (4 mutations); the control's own state changed ( -> 241ddae6-8530-43d0-8579-edc4d7a91dab\|TW\|cvs4ff4b5d392a5) | PASS |
| r00229 | admin | /orders/new | desktop | zh-TW | main | 顧客連結 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN); scrolled | PASS |
| r00230 | admin | /orders/new | desktop | zh-TW | main | 回到訂單 | click | navigation or in-page change | scrolled | PASS |
| r00231 | admin | /orders/new | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00232 | admin | /orders/new | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/orders/new?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (11 mutations) | PASS |
| r00233 | admin | /orders/new | mobile | zh-TW | main | 搜尋 (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00234 | admin | /orders/new | mobile | zh-TW | main | 配送方式 (`mo-option`) | selectOption(241ddae6-8530-43d0-8579-edc4d7a91dab\|TW\|cvs4ff4b5d392a5) | visible change | DOM changed (4 mutations); the control's own state changed ( -> 241ddae6-8530-43d0-8579-edc4d7a91dab\|TW\|cvs4ff4b5d392a5); scrolled | PASS |
| r00235 | admin | /orders/new | mobile | zh-TW | main | 顧客連結 | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN); scrolled | PASS |
| r00236 | admin | /orders/new | mobile | zh-TW | main | 回到訂單 | click | navigation or in-page change | scrolled | PASS |
| r00237 | admin | /orders/new | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00238 | admin | /orders/new | desktop | en | main | Overview | click | navigation or in-page change | url /en/orders/new?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00239 | admin | /orders/new | desktop | en | main | Search (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00240 | admin | /orders/new | desktop | en | main | Delivery method (`mo-option`) | selectOption(241ddae6-8530-43d0-8579-edc4d7a91dab\|TW\|cvs4ff4b5d392a5) | visible change | DOM changed (4 mutations); the control's own state changed ( -> 241ddae6-8530-43d0-8579-edc4d7a91dab\|TW\|cvs4ff4b5d392a5) | PASS |
| r00241 | admin | /orders/new | desktop | en | main | Customer link | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN); scrolled | PASS |
| r00242 | admin | /orders/new | desktop | en | main | Back to orders | click | navigation or in-page change | scrolled | PASS |
| r00243 | admin | /orders/cvs-print | desktop | zh-TW | page | (page load) | - | the route hands the browser to /ecpay/i | HTTP 200; POST https://logistics-stage.ecpay.com.tw/Express/PrintUniMartC2COrderInfo (blocked by the sweep) | PASS |
| r00244 | admin | /orders/cvs-print | mobile | zh-TW | page | (page load) | - | the route hands the browser to /ecpay/i | HTTP 200; POST https://logistics-stage.ecpay.com.tw/Express/PrintUniMartC2COrderInfo (blocked by the sweep) | PASS |
| r00245 | admin | /orders/cvs-print | desktop | en | page | (page load) | - | the route hands the browser to /ecpay/i | HTTP 200; POST https://logistics-stage.ecpay.com.tw/Express/PrintUniMartC2COrderInfo (blocked by the sweep) | PASS |
| r00246 | admin | /products | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00247 | admin | /products | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00248 | admin | /products | desktop | zh-TW | main | 庫存帳 (`products-ledger-link`) | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00249 | admin | /products | desktop | zh-TW | main | 新增商品 (`product-new`) | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/new?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00250 | admin | /products | desktop | zh-TW | main | 全部 3 (`products-tab-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00251 | admin | /products | desktop | zh-TW | main | 草稿 0 (`products-tab-draft`) | click | visible change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&status=draft; DOM changed (17 mutat | PASS |
| r00252 | admin | /products | desktop | zh-TW | main | 上架中 3 (`products-tab-active`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&status=active; DOM changed (17 muta (inline change only; no request fired) | PASS |
| r00253 | admin | /products | desktop | zh-TW | main | 已封存 0 (`products-tab-archived`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&status=archived; DOM changed (17 mu (inline change only; no request fired) | PASS |
| r00254 | admin | /products | desktop | zh-TW | main | 搜尋 (`products-search-submit`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00255 | admin | /products | desktop | zh-TW | main | 全部 | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00256 | admin | /products | desktop | zh-TW | main | 選取商品: Sweep Ceramic Mug | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00257 | admin | /products | desktop | zh-TW | main | 編輯: Sweep Ceramic Mug | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/4e695ae4-a1f2-4430-b23f-b800cbadfece?store=8daf0ef2-530e-4107-bece-7f4c5360df7 | PASS |
| r00258 | admin | /products | desktop | zh-TW | main | 修改價格: Sweep Ceramic Mug (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations) | PASS |
| r00259 | admin | /products | desktop | zh-TW | layer of 修改價格: Sweep Ceramic Mug | 儲存修改 (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00260 | admin | /products | desktop | zh-TW | main | 修改庫存: Sweep Ceramic Mug (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (39 mutations) | PASS |
| r00261 | admin | /products | desktop | zh-TW | layer of 修改庫存: Sweep Ceramic Mug | 不追蹤 ∞ (`quick-untracked-0`) | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (false -> true) | PASS |
| r00262 | admin | /products | desktop | zh-TW | layer of 修改庫存: Sweep Ceramic Mug | 儲存修改 (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00263 | admin | /products | desktop | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/4e695ae4-a1f2-4430-b23f-b800cbadfece?store=8daf0ef2-530e-4107-bece-7f4c5360df7 [1 of 3 alike] | PASS |
| r00264 | admin | /products | desktop | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00265 | admin | /products | desktop | zh-TW | main | 選取商品: Sweep Wool Scarf | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00266 | admin | /products | desktop | zh-TW | main | 編輯: Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/9d034c06-4939-438c-a0f6-c11cb00a843a?store=8daf0ef2-530e-4107-bece-7f4c5360df7 | PASS |
| r00267 | admin | /products | desktop | zh-TW | main | 修改價格: Sweep Wool Scarf (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations) | PASS |
| r00268 | admin | /products | desktop | zh-TW | layer of 修改價格: Sweep Wool Scarf | 儲存修改 (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00269 | admin | /products | desktop | zh-TW | main | 修改庫存: Sweep Wool Scarf (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations) | PASS |
| r00270 | admin | /products | desktop | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/9d034c06-4939-438c-a0f6-c11cb00a843a?store=8daf0ef2-530e-4107-bece-7f4c5360df7 [1 of 3 alike] | PASS |
| r00271 | admin | /products | desktop | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00272 | admin | /products | desktop | zh-TW | main | 選取商品: Sweep Cedar Candle | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00273 | admin | /products | desktop | zh-TW | main | 編輯: Sweep Cedar Candle | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7 | PASS |
| r00274 | admin | /products | desktop | zh-TW | main | 修改價格: Sweep Cedar Candle (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (39 mutations) | PASS |
| r00275 | admin | /products | desktop | zh-TW | layer of 修改價格: Sweep Cedar Candle | 儲存修改 (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00276 | admin | /products | desktop | zh-TW | main | 修改庫存: Sweep Cedar Candle (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (39 mutations); scrolled | PASS |
| r00277 | admin | /products | desktop | zh-TW | layer of 修改庫存: Sweep Cedar Candle | 不追蹤 ∞ (`quick-untracked-0`) | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00278 | admin | /products | desktop | zh-TW | layer of 修改庫存: Sweep Cedar Candle | 儲存修改 (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00279 | admin | /products | desktop | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7 [1 of 3 alike] | PASS |
| r00280 | admin | /products | desktop | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00281 | admin | /products | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00282 | admin | /products | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00283 | admin | /products | mobile | zh-TW | main | 庫存帳 (`products-ledger-link`) | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (16 mutations) | PASS |
| r00284 | admin | /products | mobile | zh-TW | main | 新增商品 (`product-new`) | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/new?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (18 mutations) | PASS |
| r00285 | admin | /products | mobile | zh-TW | main | 全部 3 (`products-tab-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00286 | admin | /products | mobile | zh-TW | main | 草稿 0 (`products-tab-draft`) | click | visible change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&status=draft; DOM changed (17 mutat | PASS |
| r00287 | admin | /products | mobile | zh-TW | main | 上架中 3 (`products-tab-active`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&status=active; DOM changed (17 muta (inline change only; no request fired) | PASS |
| r00288 | admin | /products | mobile | zh-TW | main | 已封存 0 (`products-tab-archived`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&status=archived; DOM changed (17 mu (inline change only; no request fired) | PASS |
| r00289 | admin | /products | mobile | zh-TW | main | 搜尋 (`products-search-submit`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00290 | admin | /products | mobile | zh-TW | main | 全部 | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00291 | admin | /products | mobile | zh-TW | main | 選取商品: Sweep Ceramic Mug | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00292 | admin | /products | mobile | zh-TW | main | 編輯: Sweep Ceramic Mug | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/4e695ae4-a1f2-4430-b23f-b800cbadfece?store=8daf0ef2-530e-4107-bece-7f4c5360df7 | PASS |
| r00293 | admin | /products | mobile | zh-TW | main | 修改價格: Sweep Ceramic Mug (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations) | PASS |
| r00294 | admin | /products | mobile | zh-TW | main | 修改庫存: Sweep Ceramic Mug (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (39 mutations); scrolled | PASS |
| r00295 | admin | /products | mobile | zh-TW | layer of 修改庫存: Sweep Ceramic Mug | 不追蹤 ∞ (`quick-untracked-0`) | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (false -> true) | PASS |
| r00296 | admin | /products | mobile | zh-TW | layer of 修改庫存: Sweep Ceramic Mug | 儲存修改 (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00297 | admin | /products | mobile | zh-TW | main | 編輯 | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/4e695ae4-a1f2-4430-b23f-b800cbadfece?store=8daf0ef2-530e-4107-bece-7f4c5360df7 [1 of 3 alike] | PASS |
| r00298 | admin | /products | mobile | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00299 | admin | /products | mobile | zh-TW | main | 選取商品: Sweep Wool Scarf | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00300 | admin | /products | mobile | zh-TW | main | 編輯: Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/9d034c06-4939-438c-a0f6-c11cb00a843a?store=8daf0ef2-530e-4107-bece-7f4c5360df7 | PASS |
| r00301 | admin | /products | mobile | zh-TW | main | 修改價格: Sweep Wool Scarf (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations); scrolled | PASS |
| r00302 | admin | /products | mobile | zh-TW | main | 修改庫存: Sweep Wool Scarf (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations); scrolled | PASS |
| r00303 | admin | /products | mobile | zh-TW | main | 編輯 | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00304 | admin | /products | mobile | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations); scrolled [1 of 3 alike] | PASS |
| r00305 | admin | /products | mobile | zh-TW | main | 選取商品: Sweep Cedar Candle | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ); scrolled | PASS |
| r00306 | admin | /products | mobile | zh-TW | main | 編輯: Sweep Cedar Candle | click | navigation or in-page change | scrolled | PASS |
| r00307 | admin | /products | mobile | zh-TW | main | 修改價格: Sweep Cedar Candle (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations); scrolled | PASS |
| r00308 | admin | /products | mobile | zh-TW | main | 修改庫存: Sweep Cedar Candle (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations); scrolled | PASS |
| r00309 | admin | /products | mobile | zh-TW | main | 編輯 | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00310 | admin | /products | mobile | zh-TW | main | 複製 | click | visible change | DOM changed (32 mutations); scrolled [1 of 3 alike] | PASS |
| r00311 | admin | /products | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00312 | admin | /products | desktop | en | main | Overview | click | navigation or in-page change | url /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00313 | admin | /products | desktop | en | main | Inventory ledger (`products-ledger-link`) | click | navigation or in-page change | url /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00314 | admin | /products | desktop | en | main | Add product (`product-new`) | click | navigation or in-page change | url /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products/new?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (18 mutations) | PASS |
| r00315 | admin | /products | desktop | en | main | All 3 (`products-tab-all`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00316 | admin | /products | desktop | en | main | Draft 0 (`products-tab-draft`) | click | visible change | url /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&status=draft; DOM changed (17 mutations) | PASS |
| r00317 | admin | /products | desktop | en | main | Active 3 (`products-tab-active`) | click | visible change | url /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&status=active; DOM changed (17 mutations) | PASS |
| r00318 | admin | /products | desktop | en | main | Archived 0 (`products-tab-archived`) | click | visible change | url /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&status=archived; DOM changed (17 mutation | PASS |
| r00319 | admin | /products | desktop | en | main | Search (`products-search-submit`) | click | visible change | DOM changed (14 mutations) | PASS |
| r00320 | admin | /products | desktop | en | main | All | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00321 | admin | /products | desktop | en | main | Select product: Sweep Ceramic Mug | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00322 | admin | /products | desktop | en | main | Edit: Sweep Ceramic Mug | click | navigation or in-page change | url /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products/4e695ae4-a1f2-4430-b23f-b800cbadfece?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM | PASS |
| r00323 | admin | /products | desktop | en | main | Edit price: Sweep Ceramic Mug (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations) | PASS |
| r00324 | admin | /products | desktop | en | main | Edit inventory: Sweep Ceramic Mug (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations) | PASS |
| r00325 | admin | /products | desktop | en | layer of Edit inventory: Sweep Ceramic Mug | Do not track ∞ (`quick-untracked-0`) | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (false -> true) | PASS |
| r00326 | admin | /products | desktop | en | layer of Edit inventory: Sweep Ceramic Mug | Save changes (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00327 | admin | /products | desktop | en | main | Edit | click | navigation or in-page change | url /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products/4e695ae4-a1f2-4430-b23f-b800cbadfece?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM [1 of 3 alike] | PASS |
| r00328 | admin | /products | desktop | en | main | Copy | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00329 | admin | /products | desktop | en | main | Select product: Sweep Wool Scarf | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00330 | admin | /products | desktop | en | main | Edit: Sweep Wool Scarf | click | navigation or in-page change | url /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products/9d034c06-4939-438c-a0f6-c11cb00a843a?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM | PASS |
| r00331 | admin | /products | desktop | en | main | Edit price: Sweep Wool Scarf (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations) | PASS |
| r00332 | admin | /products | desktop | en | layer of Edit price: Sweep Wool Scarf | Save changes (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00333 | admin | /products | desktop | en | main | Edit inventory: Sweep Wool Scarf (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations) | PASS |
| r00334 | admin | /products | desktop | en | main | Edit | click | navigation or in-page change | url /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products/9d034c06-4939-438c-a0f6-c11cb00a843a?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM [1 of 3 alike] | PASS |
| r00335 | admin | /products | desktop | en | main | Copy | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00336 | admin | /products | desktop | en | main | Select product: Sweep Cedar Candle | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> ) | PASS |
| r00337 | admin | /products | desktop | en | main | Edit: Sweep Cedar Candle | click | navigation or in-page change | url /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM | PASS |
| r00338 | admin | /products | desktop | en | main | Edit price: Sweep Cedar Candle (`quick-price`) | click | visible change | layer opened (DIALOG); DOM changed (35 mutations) | PASS |
| r00339 | admin | /products | desktop | en | main | Edit inventory: Sweep Cedar Candle (`quick-stock`) | click | visible change | layer opened (DIALOG); DOM changed (39 mutations); scrolled | PASS |
| r00340 | admin | /products | desktop | en | layer of Edit inventory: Sweep Cedar Candle | Do not track ∞ (`quick-untracked-0`) | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (false -> true); scrolled | PASS |
| r00341 | admin | /products | desktop | en | layer of Edit inventory: Sweep Cedar Candle | Save changes (`quick-save`) | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00342 | admin | /products | desktop | en | main | Edit | click | navigation or in-page change | url /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM [1 of 3 alike] | PASS |
| r00343 | admin | /products | desktop | en | main | Copy | click | visible change | DOM changed (32 mutations) [1 of 3 alike] | PASS |
| r00344 | admin | /products/[product] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00345 | admin | /products/[product] | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM ch | PASS |
| r00346 | admin | /products/[product] | desktop | zh-TW | main | 全部商品 (`product-back`) | click | navigation or in-page change | url /zh-TW/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7 | PASS |
| r00347 | admin | /products/[product] | desktop | zh-TW | main | 商品圖片 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00348 | admin | /products/[product] | desktop | zh-TW | main | 基本資訊 | click | visible change | DOM changed (6 mutations) | PASS |
| r00349 | admin | /products/[product] | desktop | zh-TW | main | 規格 | click | visible change | DOM changed (7 mutations) | PASS |
| r00350 | admin | /products/[product] | desktop | zh-TW | main | 分類 | click | visible change | DOM changed (5 mutations) | PASS |
| r00351 | admin | /products/[product] | desktop | zh-TW | main | 物流 | click | visible change | DOM changed (5 mutations) | PASS |
| r00352 | admin | /products/[product] | desktop | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (6 mutations) | PASS |
| r00353 | admin | /products/[product] | desktop | zh-TW | main | 待填寫 圖片 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00354 | admin | /products/[product] | desktop | zh-TW | main | 已完成 商品名稱 | click | visible change | DOM changed (6 mutations) | PASS |
| r00355 | admin | /products/[product] | desktop | zh-TW | main | 待填寫 售價 | click | visible change | DOM changed (7 mutations) | PASS |
| r00356 | admin | /products/[product] | desktop | zh-TW | main | 已完成 庫存 | click | visible change | DOM changed (7 mutations) | PASS |
| r00357 | admin | /products/[product] | desktop | zh-TW | main | 待填寫 至少 3 張圖片 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00358 | admin | /products/[product] | desktop | zh-TW | main | 已完成 描述 | click | visible change | DOM changed (6 mutations) | PASS |
| r00359 | admin | /products/[product] | desktop | zh-TW | main | 已完成 分類 | click | visible change | DOM changed (5 mutations) | PASS |
| r00360 | admin | /products/[product] | desktop | zh-TW | main | 待填寫 直播關鍵字 | click | visible change | DOM changed (7 mutations) | PASS |
| r00361 | admin | /products/[product] | desktop | zh-TW | main | 待填寫 搜尋引擎最佳化 | click | visible change | DOM changed (6 mutations) | PASS |
| r00362 | admin | /products/[product] | desktop | zh-TW | main | 顯示狀態上架中：商店發布後，買家可以看到並購買。 (`product-status`) | selectOption(draft) | confirmation layer opens; cancel; nothing changed | DOM changed (2 mutations); the control's own state changed (active -> draft) (inline change only; no request fired) | PASS |
| r00363 | admin | /products/[product] | desktop | zh-TW | main | 移除 T04-64977906744e-0 | click | confirmation layer opens; cancel; nothing changed | DOM changed (7 mutations) (inline change only; no request fired) | PASS |
| r00364 | admin | /products/[product] | desktop | zh-TW | main | 移除 規格名稱 1 | click | confirmation layer opens; cancel; nothing changed | DOM changed (11 mutations); the control's own state changed ( -> true) (inline change only; no request fired) | PASS |
| r00365 | admin | /products/[product] | desktop | zh-TW | main | 新增規格（顏色、尺寸…） (`axis-add`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00366 | admin | /products/[product] | desktop | zh-TW | main | 批量填入 (`bulk-open`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00367 | admin | /products/[product] | desktop | zh-TW | main | 已選取 T04-64977906744e-0 (`matrix-select-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true) | PASS |
| r00368 | admin | /products/[product] | desktop | zh-TW | main | 不追蹤 ∞ | click (toggle) | visible change | DOM changed (5 mutations); the control's own state changed (false -> true) | PASS |
| r00369 | admin | /products/[product] | desktop | zh-TW | main | 啟用 1 (`matrix-active-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (true -> false) | PASS |
| r00370 | admin | /products/[product] | desktop | zh-TW | main | Sweep home | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (true -> false) | PASS |
| r00371 | admin | /products/[product] | desktop | zh-TW | main | Sweep wear | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true) | PASS |
| r00372 | admin | /products/[product] | desktop | zh-TW | main | 管理分類 | click | navigation or in-page change | url /zh-TW/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/collections?store=8daf0ef2-530e-4107-bece-7f4c5360 | PASS |
| r00373 | admin | /products/[product] | desktop | zh-TW | main | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00374 | admin | /products/[product] | desktop | zh-TW | layer of 物流 | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00375 | admin | /products/[product] | desktop | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00376 | admin | /products/[product] | desktop | zh-TW | layer of 搜尋引擎最佳化 | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00377 | admin | /products/[product] | desktop | zh-TW | main | 儲存修改 (`product-save`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00378 | admin | /products/[product] | desktop | zh-TW | main | 下架轉草稿 (`product-unpublish`) | click | confirmation layer opens; cancel; nothing changed | dialog: confirm: 確定下架此商品？買家將無法再購買此商品。 (confirmation shown, then cancelled) | PASS |
| r00379 | admin | /products/[product] | desktop | zh-TW | main | 上架 (`product-publish`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (1 mutations) (inline change only; no request fired) | PASS |
| r00380 | admin | /products/[product] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00381 | admin | /products/[product] | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM ch | PASS |
| r00382 | admin | /products/[product] | mobile | zh-TW | main | 全部商品 (`product-back`) | click | navigation or in-page change | url /zh-TW/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7 | PASS |
| r00383 | admin | /products/[product] | mobile | zh-TW | main | 商品圖片 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00384 | admin | /products/[product] | mobile | zh-TW | main | 基本資訊 | click | visible change | DOM changed (6 mutations) | PASS |
| r00385 | admin | /products/[product] | mobile | zh-TW | main | 規格 | click | visible change | DOM changed (7 mutations) | PASS |
| r00386 | admin | /products/[product] | mobile | zh-TW | main | 分類 | click | visible change | DOM changed (5 mutations) | PASS |
| r00387 | admin | /products/[product] | mobile | zh-TW | main | 物流 | click | visible change | DOM changed (5 mutations) | PASS |
| r00388 | admin | /products/[product] | mobile | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (6 mutations) | PASS |
| r00389 | admin | /products/[product] | mobile | zh-TW | main | 顯示狀態上架中：商店發布後，買家可以看到並購買。 (`product-status`) | selectOption(draft) | confirmation layer opens; cancel; nothing changed | DOM changed (2 mutations); the control's own state changed (active -> draft) (inline change only; no request fired) | PASS |
| r00390 | admin | /products/[product] | mobile | zh-TW | main | 移除 T04-64977906744e-0 | click | confirmation layer opens; cancel; nothing changed | DOM changed (7 mutations) (inline change only; no request fired) | PASS |
| r00391 | admin | /products/[product] | mobile | zh-TW | main | 移除 規格名稱 1 | click | confirmation layer opens; cancel; nothing changed | DOM changed (11 mutations); the control's own state changed ( -> true) (inline change only; no request fired) | PASS |
| r00392 | admin | /products/[product] | mobile | zh-TW | main | 新增規格（顏色、尺寸…） (`axis-add`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00393 | admin | /products/[product] | mobile | zh-TW | main | 批量填入 (`bulk-open`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00394 | admin | /products/[product] | mobile | zh-TW | main | 已選取 T04-64977906744e-0 (`matrix-select-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true) | PASS |
| r00395 | admin | /products/[product] | mobile | zh-TW | main | 不追蹤 ∞ | click (toggle) | visible change | DOM changed (5 mutations); the control's own state changed (false -> true) | PASS |
| r00396 | admin | /products/[product] | mobile | zh-TW | main | Sweep home | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (true -> false) | PASS |
| r00397 | admin | /products/[product] | mobile | zh-TW | main | Sweep wear | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true) | PASS |
| r00398 | admin | /products/[product] | mobile | zh-TW | main | 管理分類 | click | navigation or in-page change | url /zh-TW/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/collections?store=8daf0ef2-530e-4107-bece-7f4c5360 | PASS |
| r00399 | admin | /products/[product] | mobile | zh-TW | main | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00400 | admin | /products/[product] | mobile | zh-TW | layer of 物流 | 物流 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00401 | admin | /products/[product] | mobile | zh-TW | main | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00402 | admin | /products/[product] | mobile | zh-TW | layer of 搜尋引擎最佳化 | 搜尋引擎最佳化 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00403 | admin | /products/[product] | mobile | zh-TW | main | 儲存修改 (`product-save`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00404 | admin | /products/[product] | mobile | zh-TW | main | 下架轉草稿 (`product-unpublish`) | click | confirmation layer opens; cancel; nothing changed | dialog: confirm: 確定下架此商品？買家將無法再購買此商品。 (confirmation shown, then cancelled) | PASS |
| r00405 | admin | /products/[product] | mobile | zh-TW | main | 上架 (`product-publish`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (1 mutations) (inline change only; no request fired) | PASS |
| r00406 | admin | /products/[product] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00407 | admin | /products/[product] | desktop | en | main | Overview | click | navigation or in-page change | url /en/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed  | PASS |
| r00408 | admin | /products/[product] | desktop | en | main | All products (`product-back`) | click | navigation or in-page change | url /en/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM | PASS |
| r00409 | admin | /products/[product] | desktop | en | main | Product images | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00410 | admin | /products/[product] | desktop | en | main | Basic information | click | visible change | DOM changed (6 mutations) | PASS |
| r00411 | admin | /products/[product] | desktop | en | main | Variants | click | visible change | DOM changed (7 mutations) | PASS |
| r00412 | admin | /products/[product] | desktop | en | main | Collections | click | visible change | DOM changed (5 mutations) | PASS |
| r00413 | admin | /products/[product] | desktop | en | main | Shipping | click | visible change | DOM changed (5 mutations) | PASS |
| r00414 | admin | /products/[product] | desktop | en | main | Search engine listing | click | visible change | DOM changed (6 mutations) | PASS |
| r00415 | admin | /products/[product] | desktop | en | main | To do Images | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00416 | admin | /products/[product] | desktop | en | main | Ready Product name | click | visible change | DOM changed (6 mutations) | PASS |
| r00417 | admin | /products/[product] | desktop | en | main | To do Price | click | visible change | DOM changed (7 mutations) | PASS |
| r00418 | admin | /products/[product] | desktop | en | main | Ready Inventory | click | visible change | DOM changed (7 mutations) | PASS |
| r00419 | admin | /products/[product] | desktop | en | main | To do At least 3 images | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00420 | admin | /products/[product] | desktop | en | main | Ready Description | click | visible change | DOM changed (6 mutations) | PASS |
| r00421 | admin | /products/[product] | desktop | en | main | Ready Collections | click | visible change | DOM changed (5 mutations) | PASS |
| r00422 | admin | /products/[product] | desktop | en | main | To do Live keyword | click | visible change | DOM changed (7 mutations) | PASS |
| r00423 | admin | /products/[product] | desktop | en | main | To do Search engine listing | click | visible change | DOM changed (6 mutations) | PASS |
| r00424 | admin | /products/[product] | desktop | en | main | VisibilityActive: shoppers can see and buy it once the store is published. (`product-status`) | selectOption(draft) | visible change | DOM changed (2 mutations); the control's own state changed (active -> draft) | PASS |
| r00425 | admin | /products/[product] | desktop | en | main | Remove T04-64977906744e-0 | click | confirmation layer opens; cancel; nothing changed | DOM changed (7 mutations) (inline change only; no request fired) | PASS |
| r00426 | admin | /products/[product] | desktop | en | main | Remove Option name 1 | click | confirmation layer opens; cancel; nothing changed | DOM changed (10 mutations); the control's own state changed ( -> true) (inline change only; no request fired) | PASS |
| r00427 | admin | /products/[product] | desktop | en | main | Add option (color, size…) (`axis-add`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00428 | admin | /products/[product] | desktop | en | main | Bulk fill (`bulk-open`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00429 | admin | /products/[product] | desktop | en | main | Selected T04-64977906744e-0 (`matrix-select-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true) | PASS |
| r00430 | admin | /products/[product] | desktop | en | main | Do not track ∞ | click (toggle) | visible change | DOM changed (5 mutations); the control's own state changed (false -> true) | PASS |
| r00431 | admin | /products/[product] | desktop | en | main | Enabled 1 (`matrix-active-0`) | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (true -> false) | PASS |
| r00432 | admin | /products/[product] | desktop | en | main | Sweep home | click (toggle) | visible change | DOM changed (2 mutations); the control's own state changed (true -> false) | PASS |
| r00433 | admin | /products/[product] | desktop | en | main | Sweep wear | click (toggle) | visible change | DOM changed (1 mutations); the control's own state changed (false -> true) | PASS |
| r00434 | admin | /products/[product] | desktop | en | main | Manage collections | click | navigation or in-page change | url /en/products/470f0853-d5c3-4575-82b4-25071fbecf9d?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/collections?store=8daf0ef2-530e-4107-bece-7f4c5360df7f;  | PASS |
| r00435 | admin | /products/[product] | desktop | en | main | Shipping | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00436 | admin | /products/[product] | desktop | en | layer of Shipping | Shipping | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00437 | admin | /products/[product] | desktop | en | main | Search engine listing | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00438 | admin | /products/[product] | desktop | en | layer of Search engine listing | Search engine listing | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00439 | admin | /products/[product] | desktop | en | main | Save changes (`product-save`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00440 | admin | /products/[product] | desktop | en | main | Unpublish to draft (`product-unpublish`) | click | confirmation layer opens; cancel; nothing changed | dialog: confirm: Unpublish this product? Shoppers will no longer be able to buy it. (confirmation shown, then cancelled) | PASS |
| r00441 | admin | /products/[product] | desktop | en | main | Publish (`product-publish`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (1 mutations) (inline change only; no request fired) | PASS |
| r00442 | admin | /collections | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00443 | admin | /collections | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/collections?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00444 | admin | /collections | desktop | zh-TW | main | 新增分類 (`collection-new`) | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00445 | admin | /collections | desktop | zh-TW | main | Sweep home /sweep-home · 2 件商品 (`collection-item-b693c209-4a34-412a-8603-b447464b37d2`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00446 | admin | /collections | desktop | zh-TW | main | Sweep wear /sweep-wear · 2 件商品 (`collection-item-4de2549e-fb4d-4662-866b-87dcd329b008`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00447 | admin | /collections | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00448 | admin | /collections | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/collections?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00449 | admin | /collections | mobile | zh-TW | main | 新增分類 (`collection-new`) | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00450 | admin | /collections | mobile | zh-TW | main | Sweep home /sweep-home · 2 件商品 (`collection-item-b693c209-4a34-412a-8603-b447464b37d2`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00451 | admin | /collections | mobile | zh-TW | main | Sweep wear /sweep-wear · 2 件商品 (`collection-item-4de2549e-fb4d-4662-866b-87dcd329b008`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00452 | admin | /collections | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00453 | admin | /collections | desktop | en | main | Overview | click | navigation or in-page change | url /en/collections?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00454 | admin | /collections | desktop | en | main | New collection (`collection-new`) | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00455 | admin | /collections | desktop | en | main | Sweep home /sweep-home · 2 products (`collection-item-b693c209-4a34-412a-8603-b447464b37d2`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00456 | admin | /collections | desktop | en | main | Sweep wear /sweep-wear · 2 products (`collection-item-4de2549e-fb4d-4662-866b-87dcd329b008`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00457 | admin | /inventory | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00458 | admin | /inventory | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00459 | admin | /inventory | desktop | zh-TW | main | 新增商品 | click | navigation or in-page change | url /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/new?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (18 mutations) | PASS |
| r00460 | admin | /inventory | desktop | zh-TW | main | 查詢 | click | visible change | DOM changed (2 mutations) | PASS |
| r00461 | admin | /inventory | desktop | zh-TW | main | 倉庫 | selectOption | visible change | select has a single option | SKIP |
| r00462 | admin | /inventory | desktop | zh-TW | main | 狀態 | selectOption(active) | visible change | url /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&status=active; DOM changed (2 mut | PASS |
| r00463 | admin | /inventory | desktop | zh-TW | main | 重設 | click | visible change | DOM changed (2 mutations) | PASS |
| r00464 | admin | /inventory | desktop | zh-TW | main | 重新整理 | click | visible change | DOM changed (2 mutations) | PASS |
| r00465 | admin | /inventory | desktop | zh-TW | main | 選取 T04-64977906744e-0 | click | visible change | DOM changed (12 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00466 | admin | /inventory | desktop | zh-TW | main | Sweep Cedar Candle | click | visible change | DOM changed (12 mutations); validation shown | PASS |
| r00467 | admin | /inventory | desktop | zh-TW | main | 選取 | click | visible change | DOM changed (12 mutations); validation shown [1 of 3 alike] | PASS |
| r00468 | admin | /inventory | desktop | zh-TW | main | 選取 SWP-MUG | click | visible change | DOM changed (6 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00469 | admin | /inventory | desktop | zh-TW | main | Sweep Ceramic Mug | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00470 | admin | /inventory | desktop | zh-TW | main | 選取 | click | visible change | DOM changed (12 mutations); validation shown [1 of 3 alike] | PASS |
| r00471 | admin | /inventory | desktop | zh-TW | main | 選取 SWP-SCF | click | visible change | DOM changed (6 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00472 | admin | /inventory | desktop | zh-TW | main | Sweep Wool Scarf | click | visible change | DOM changed (12 mutations); validation shown | PASS |
| r00473 | admin | /inventory | desktop | zh-TW | main | 選取 | click | visible change | DOM changed (6 mutations); validation shown [1 of 3 alike] | PASS |
| r00474 | admin | /inventory | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00475 | admin | /inventory | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00476 | admin | /inventory | mobile | zh-TW | main | 新增商品 | click | navigation or in-page change | url /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/products/new?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (18 mutations) | PASS |
| r00477 | admin | /inventory | mobile | zh-TW | main | 查詢 | click | visible change | DOM changed (2 mutations) | PASS |
| r00478 | admin | /inventory | mobile | zh-TW | main | 倉庫 | selectOption | visible change | select has a single option | SKIP |
| r00479 | admin | /inventory | mobile | zh-TW | main | 狀態 | selectOption(active) | visible change | url /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&status=active; DOM changed (2 mut | PASS |
| r00480 | admin | /inventory | mobile | zh-TW | main | 重設 | click | visible change | DOM changed (2 mutations) | PASS |
| r00481 | admin | /inventory | mobile | zh-TW | main | 重新整理 | click | visible change | DOM changed (2 mutations) | PASS |
| r00482 | admin | /inventory | mobile | zh-TW | main | 選取 T04-64977906744e-0 | click | visible change | DOM changed (12 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00483 | admin | /inventory | mobile | zh-TW | main | Sweep Cedar Candle | click | visible change | DOM changed (12 mutations); validation shown | PASS |
| r00484 | admin | /inventory | mobile | zh-TW | main | 選取 SWP-MUG | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00485 | admin | /inventory | mobile | zh-TW | main | Sweep Ceramic Mug | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00486 | admin | /inventory | mobile | zh-TW | main | 選取 SWP-SCF | click | visible change | DOM changed (12 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00487 | admin | /inventory | mobile | zh-TW | main | Sweep Wool Scarf | click | visible change | DOM changed (12 mutations); validation shown | PASS |
| r00488 | admin | /inventory | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00489 | admin | /inventory | desktop | en | main | Overview | click | navigation or in-page change | url /en/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00490 | admin | /inventory | desktop | en | main | Add product | click | navigation or in-page change | url /en/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/products/new?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (18 mutations) | PASS |
| r00491 | admin | /inventory | desktop | en | main | Search | click | visible change | DOM changed (2 mutations) | PASS |
| r00492 | admin | /inventory | desktop | en | main | Warehouse | selectOption | visible change | select has a single option | SKIP |
| r00493 | admin | /inventory | desktop | en | main | Status | selectOption(active) | visible change | url /en/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/inventory?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&status=active; DOM changed (2 mutations | PASS |
| r00494 | admin | /inventory | desktop | en | main | Reset | click | visible change | DOM changed (2 mutations) | PASS |
| r00495 | admin | /inventory | desktop | en | main | Refresh | click | visible change | DOM changed (2 mutations) | PASS |
| r00496 | admin | /inventory | desktop | en | main | Select T04-64977906744e-0 | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00497 | admin | /inventory | desktop | en | main | Sweep Cedar Candle | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00498 | admin | /inventory | desktop | en | main | Select | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00499 | admin | /inventory | desktop | en | main | Select SWP-MUG | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00500 | admin | /inventory | desktop | en | main | Sweep Ceramic Mug | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00501 | admin | /inventory | desktop | en | main | Select | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00502 | admin | /inventory | desktop | en | main | Select SWP-SCF | click | visible change | DOM changed (4 mutations); validation shown; the control's own state changed (false -> true) | PASS |
| r00503 | admin | /inventory | desktop | en | main | Sweep Wool Scarf | click | visible change | DOM changed (4 mutations); validation shown | PASS |
| r00504 | admin | /inventory | desktop | en | main | Select | click | visible change | DOM changed (4 mutations); validation shown [1 of 3 alike] | PASS |
| r00505 | admin | /products/import | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00506 | admin | /products/import | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/import?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00507 | admin | /products/import | desktop | zh-TW | main | 下載 CSV (`import-export`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00508 | admin | /products/import | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00509 | admin | /products/import | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/products/import?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00510 | admin | /products/import | mobile | zh-TW | main | 下載 CSV (`import-export`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00511 | admin | /products/import | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00512 | admin | /products/import | desktop | en | main | Overview | click | navigation or in-page change | url /en/products/import?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00513 | admin | /products/import | desktop | en | main | Download CSV (`import-export`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00514 | admin | /customers | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00515 | admin | /customers | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00516 | admin | /customers | desktop | zh-TW | main | 搜尋 (`customers-search-submit`) | click | visible change | DOM changed (12 mutations) | PASS |
| r00517 | admin | /customers | desktop | zh-TW | main | 重新整理 (`customers-refresh`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00518 | admin | /customers | desktop | zh-TW | main | 開啟客戶: 王小明 (`customer-open-c437a9be-421e-40c2-a33c-70f13df12380`) | click | navigation or in-page change | url /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/customers/c437a9be-421e-40c2-a33c-70f13df12380?store=8daf0ef2-530e-4107-bece-7f4c5360d [1 of 3 alike] | PASS |
| r00519 | admin | /customers | desktop | zh-TW | main | 開啟客戶: 王小明 (`customer-open-1a0ae07e-56f5-4c9f-a630-4816aa92cd83`) | click | navigation or in-page change | url /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/customers/1a0ae07e-56f5-4c9f-a630-4816aa92cd83?store=8daf0ef2-530e-4107-bece-7f4c5360d [1 of 3 alike] | PASS |
| r00520 | admin | /customers | desktop | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-67e512c9-e813-4813-8b25-6cf787487997`) | click | navigation or in-page change | url /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/customers/67e512c9-e813-4813-8b25-6cf787487997?store=8daf0ef2-530e-4107-bece-7f4c5360d [1 of 3 alike] | PASS |
| r00521 | admin | /customers | desktop | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-ebaeb810-dbc6-4717-87e8-4cc68ea2179f`) | click | navigation or in-page change | url /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/customers/ebaeb810-dbc6-4717-87e8-4cc68ea2179f?store=8daf0ef2-530e-4107-bece-7f4c5360d [1 of 3 alike] | PASS |
| r00522 | admin | /customers | desktop | zh-TW | main | 開啟客戶: 王小明 (`customer-open-cc08c733-c457-49d3-92ee-e64a4d1300c5`) | click | navigation or in-page change | url /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/customers/cc08c733-c457-49d3-92ee-e64a4d1300c5?store=8daf0ef2-530e-4107-bece-7f4c5360d [1 of 3 alike] | PASS |
| r00523 | admin | /customers | desktop | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-cdbe42be-dd1b-4816-a404-c754d60107ed`) | click | navigation or in-page change | url /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/customers/cdbe42be-dd1b-4816-a404-c754d60107ed?store=8daf0ef2-530e-4107-bece-7f4c5360d [1 of 3 alike] | PASS |
| r00524 | admin | /customers | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00525 | admin | /customers | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (11 mutations) | PASS |
| r00526 | admin | /customers | mobile | zh-TW | main | 搜尋 (`customers-search-submit`) | click | visible change | DOM changed (12 mutations) | PASS |
| r00527 | admin | /customers | mobile | zh-TW | main | 重新整理 (`customers-refresh`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00528 | admin | /customers | mobile | zh-TW | main | 開啟客戶: 王小明 (`customer-open-c437a9be-421e-40c2-a33c-70f13df12380`) | click | navigation or in-page change | url /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/customers/c437a9be-421e-40c2-a33c-70f13df12380?store=8daf0ef2-530e-4107-bece-7f4c5360d [1 of 3 alike] | PASS |
| r00529 | admin | /customers | mobile | zh-TW | main | 開啟客戶: 王小明 (`customer-open-1a0ae07e-56f5-4c9f-a630-4816aa92cd83`) | click | navigation or in-page change | url /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/customers/1a0ae07e-56f5-4c9f-a630-4816aa92cd83?store=8daf0ef2-530e-4107-bece-7f4c5360d [1 of 3 alike] | PASS |
| r00530 | admin | /customers | mobile | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-67e512c9-e813-4813-8b25-6cf787487997`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00531 | admin | /customers | mobile | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-ebaeb810-dbc6-4717-87e8-4cc68ea2179f`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00532 | admin | /customers | mobile | zh-TW | main | 開啟客戶: 王小明 (`customer-open-cc08c733-c457-49d3-92ee-e64a4d1300c5`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00533 | admin | /customers | mobile | zh-TW | main | 開啟客戶: Synthetic Buyer (`customer-open-cdbe42be-dd1b-4816-a404-c754d60107ed`) | click | navigation or in-page change | scrolled [1 of 3 alike] | PASS |
| r00534 | admin | /customers | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00535 | admin | /customers | desktop | en | main | Overview | click | navigation or in-page change | url /en/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00536 | admin | /customers | desktop | en | main | Search (`customers-search-submit`) | click | visible change | DOM changed (12 mutations) | PASS |
| r00537 | admin | /customers | desktop | en | main | Refresh (`customers-refresh`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00538 | admin | /customers | desktop | en | main | Open customer: 王小明 (`customer-open-c437a9be-421e-40c2-a33c-70f13df12380`) | click | navigation or in-page change | url /en/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/customers/c437a9be-421e-40c2-a33c-70f13df12380?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; D [1 of 3 alike] | PASS |
| r00539 | admin | /customers | desktop | en | main | Open customer: 王小明 (`customer-open-1a0ae07e-56f5-4c9f-a630-4816aa92cd83`) | click | navigation or in-page change | url /en/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/customers/1a0ae07e-56f5-4c9f-a630-4816aa92cd83?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; D [1 of 3 alike] | PASS |
| r00540 | admin | /customers | desktop | en | main | Open customer: Synthetic Buyer (`customer-open-67e512c9-e813-4813-8b25-6cf787487997`) | click | navigation or in-page change | url /en/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/customers/67e512c9-e813-4813-8b25-6cf787487997?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; D [1 of 3 alike] | PASS |
| r00541 | admin | /customers | desktop | en | main | Open customer: Synthetic Buyer (`customer-open-ebaeb810-dbc6-4717-87e8-4cc68ea2179f`) | click | navigation or in-page change | url /en/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/customers/ebaeb810-dbc6-4717-87e8-4cc68ea2179f?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; D [1 of 3 alike] | PASS |
| r00542 | admin | /customers | desktop | en | main | Open customer: 王小明 (`customer-open-cc08c733-c457-49d3-92ee-e64a4d1300c5`) | click | navigation or in-page change | url /en/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/customers/cc08c733-c457-49d3-92ee-e64a4d1300c5?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; D [1 of 3 alike] | PASS |
| r00543 | admin | /customers | desktop | en | main | Open customer: Synthetic Buyer (`customer-open-cdbe42be-dd1b-4816-a404-c754d60107ed`) | click | navigation or in-page change | url /en/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/customers/cdbe42be-dd1b-4816-a404-c754d60107ed?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; D [1 of 3 alike] | PASS |
| r00544 | admin | /customers/[customer] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00545 | admin | /customers/[customer] | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers/ebaeb810-dbc6-4717-87e8-4cc68ea2179f?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM c | PASS |
| r00546 | admin | /customers/[customer] | desktop | zh-TW | main | 全部客戶 | click | navigation or in-page change | url /zh-TW/customers/ebaeb810-dbc6-4717-87e8-4cc68ea2179f?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360d | PASS |
| r00547 | admin | /customers/[customer] | desktop | zh-TW | main | 在訂單中開啟: ff529d66-e17a-4eb1-92cc-f26d7fd245fd | click | navigation or in-page change | url /zh-TW/customers/ebaeb810-dbc6-4717-87e8-4cc68ea2179f?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f | PASS |
| r00548 | admin | /customers/[customer] | desktop | zh-TW | main | 下載資料 (`customer-download`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00549 | admin | /customers/[customer] | desktop | zh-TW | main | 抹除客戶 (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00550 | admin | /customers/[customer] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00551 | admin | /customers/[customer] | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers/ebaeb810-dbc6-4717-87e8-4cc68ea2179f?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM c | PASS |
| r00552 | admin | /customers/[customer] | mobile | zh-TW | main | 全部客戶 | click | navigation or in-page change | url /zh-TW/customers/ebaeb810-dbc6-4717-87e8-4cc68ea2179f?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/customers?store=8daf0ef2-530e-4107-bece-7f4c5360d | PASS |
| r00553 | admin | /customers/[customer] | mobile | zh-TW | main | 在訂單中開啟: ff529d66-e17a-4eb1-92cc-f26d7fd245fd | click | navigation or in-page change | url /zh-TW/customers/ebaeb810-dbc6-4717-87e8-4cc68ea2179f?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f | PASS |
| r00554 | admin | /customers/[customer] | mobile | zh-TW | main | 下載資料 (`customer-download`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00555 | admin | /customers/[customer] | mobile | zh-TW | main | 抹除客戶 (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00556 | admin | /customers/[customer] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00557 | admin | /customers/[customer] | desktop | en | main | Overview | click | navigation or in-page change | url /en/customers/ebaeb810-dbc6-4717-87e8-4cc68ea2179f?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed | PASS |
| r00558 | admin | /customers/[customer] | desktop | en | main | All customers | click | navigation or in-page change | url /en/customers/ebaeb810-dbc6-4717-87e8-4cc68ea2179f?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/customers?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; D | PASS |
| r00559 | admin | /customers/[customer] | desktop | en | main | Open in orders: ff529d66-e17a-4eb1-92cc-f26d7fd245fd | click | navigation or in-page change | url /en/customers/ebaeb810-dbc6-4717-87e8-4cc68ea2179f?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/orders?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&order | PASS |
| r00560 | admin | /customers/[customer] | desktop | en | main | Download data (`customer-download`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00561 | admin | /customers/[customer] | desktop | en | main | Erase customer (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00562 | admin | /ads/attribution | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00563 | admin | /ads/attribution | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads/attribution?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00564 | admin | /ads/attribution | desktop | zh-TW | main | 返回廣告 (`attribution-back`) | click | navigation or in-page change | url /zh-TW/ads/attribution?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/ads?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (20 mutations) | PASS |
| r00565 | admin | /ads/attribution | desktop | zh-TW | main | 讀取報表 (`attribution-apply`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00566 | admin | /ads/attribution | desktop | zh-TW | main | 直播場次 (`attribution-session`) | selectOption | visible change | select has a single option | SKIP |
| r00567 | admin | /ads/attribution | desktop | zh-TW | main | 重新連接 Facebook (`attribution-reconnect`) | click | navigation or in-page change | scrolled | PASS |
| r00568 | admin | /ads/attribution | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00569 | admin | /ads/attribution | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads/attribution?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (11 mutations) | PASS |
| r00570 | admin | /ads/attribution | mobile | zh-TW | main | 返回廣告 (`attribution-back`) | click | navigation or in-page change | url /zh-TW/ads/attribution?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/ads?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (20 mutations) | PASS |
| r00571 | admin | /ads/attribution | mobile | zh-TW | main | 讀取報表 (`attribution-apply`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00572 | admin | /ads/attribution | mobile | zh-TW | main | 直播場次 (`attribution-session`) | selectOption | visible change | select has a single option | SKIP |
| r00573 | admin | /ads/attribution | mobile | zh-TW | main | 重新連接 Facebook (`attribution-reconnect`) | click | navigation or in-page change | scrolled | PASS |
| r00574 | admin | /ads/attribution | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00575 | admin | /ads/attribution | desktop | en | main | Overview | click | navigation or in-page change | url /en/ads/attribution?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00576 | admin | /ads/attribution | desktop | en | main | Back to ads (`attribution-back`) | click | navigation or in-page change | url /en/ads/attribution?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/ads?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (20 mutations) | PASS |
| r00577 | admin | /ads/attribution | desktop | en | main | Read report (`attribution-apply`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00578 | admin | /ads/attribution | desktop | en | main | Live session (`attribution-session`) | selectOption | visible change | select has a single option | SKIP |
| r00579 | admin | /ads/attribution | desktop | en | main | Reconnect Facebook (`attribution-reconnect`) | click | navigation or in-page change | scrolled | PASS |
| r00580 | admin | /promotions | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00581 | admin | /promotions | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/promotions?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00582 | admin | /promotions | desktop | zh-TW | main | 類型 (`promotion-kind`) | selectOption(fixed) | visible change | DOM changed (1 mutations); the control's own state changed (percent -> fixed) | PASS |
| r00583 | admin | /promotions | desktop | zh-TW | main | 建立優惠碼 (`promotion-submit`) | click | visible change | validation shown | PASS |
| r00584 | admin | /promotions | desktop | zh-TW | main | 暫停 (`promotion-toggle`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00585 | admin | /promotions | desktop | zh-TW | main | 編輯 (`promotion-edit`) | click | visible change | DOM changed (6 mutations) | PASS |
| r00586 | admin | /promotions | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00587 | admin | /promotions | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/promotions?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00588 | admin | /promotions | mobile | zh-TW | main | 類型 (`promotion-kind`) | selectOption(fixed) | visible change | DOM changed (1 mutations); the control's own state changed (percent -> fixed) | PASS |
| r00589 | admin | /promotions | mobile | zh-TW | main | 建立優惠碼 (`promotion-submit`) | click | visible change | validation shown; scrolled | PASS |
| r00590 | admin | /promotions | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00591 | admin | /promotions | desktop | en | main | Overview | click | navigation or in-page change | url /en/promotions?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00592 | admin | /promotions | desktop | en | main | Type (`promotion-kind`) | selectOption(fixed) | visible change | DOM changed (1 mutations); the control's own state changed (percent -> fixed) | PASS |
| r00593 | admin | /promotions | desktop | en | main | Create code (`promotion-submit`) | click | visible change | validation shown | PASS |
| r00594 | admin | /promotions | desktop | en | main | Pause (`promotion-toggle`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00595 | admin | /promotions | desktop | en | main | Edit (`promotion-edit`) | click | visible change | DOM changed (6 mutations) | PASS |
| r00596 | admin | /ads | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00597 | admin | /ads | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00598 | admin | /ads | desktop | zh-TW | main | 廣告歸因與直播復盤 (`ads-attribution-link`) | click | navigation or in-page change | url /zh-TW/ads?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/ads/attribution?store=8daf0ef2-530e-4107-bece-7f4c5360df7f | PASS |
| r00599 | admin | /ads | desktop | zh-TW | main | 重新整理 (`ads-refresh`) | click | visible change | DOM changed (7 mutations) | PASS |
| r00600 | admin | /ads | desktop | zh-TW | main | 連結 Meta 廣告帳號 (`ads-connect`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00601 | admin | /ads | desktop | zh-TW | main | 新增草稿 (`ads-new-draft`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00602 | admin | /ads | desktop | zh-TW | main | 顯示成效 (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00603 | admin | /ads | desktop | zh-TW | main | 傳送購買事件 (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00604 | admin | /ads | desktop | zh-TW | main | 資料集 (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00605 | admin | /ads | desktop | zh-TW | main | 儲存 (`ads-capi-save`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00606 | admin | /ads | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00607 | admin | /ads | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00608 | admin | /ads | mobile | zh-TW | main | 廣告歸因與直播復盤 (`ads-attribution-link`) | click | navigation or in-page change | url /zh-TW/ads?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/ads/attribution?store=8daf0ef2-530e-4107-bece-7f4c5360df7f | PASS |
| r00609 | admin | /ads | mobile | zh-TW | main | 重新整理 (`ads-refresh`) | click | visible change | DOM changed (7 mutations) | PASS |
| r00610 | admin | /ads | mobile | zh-TW | main | 連結 Meta 廣告帳號 (`ads-connect`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00611 | admin | /ads | mobile | zh-TW | main | 新增草稿 (`ads-new-draft`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00612 | admin | /ads | mobile | zh-TW | main | 顯示成效 (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00613 | admin | /ads | mobile | zh-TW | main | 傳送購買事件 (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00614 | admin | /ads | mobile | zh-TW | main | 資料集 (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00615 | admin | /ads | mobile | zh-TW | main | 儲存 (`ads-capi-save`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00616 | admin | /ads | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00617 | admin | /ads | desktop | en | main | Overview | click | navigation or in-page change | url /en/ads?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00618 | admin | /ads | desktop | en | main | Ad attribution & session review (`ads-attribution-link`) | click | navigation or in-page change | url /en/ads?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/ads/attribution?store=8daf0ef2-530e-4107-bece-7f4c5360df7f | PASS |
| r00619 | admin | /ads | desktop | en | main | Refresh (`ads-refresh`) | click | visible change | DOM changed (7 mutations) | PASS |
| r00620 | admin | /ads | desktop | en | main | Connect a Meta ad account (`ads-connect`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00621 | admin | /ads | desktop | en | main | New draft (`ads-new-draft`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00622 | admin | /ads | desktop | en | main | Show results (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00623 | admin | /ads | desktop | en | main | Send purchase events (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00624 | admin | /ads | desktop | en | main | Dataset (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00625 | admin | /ads | desktop | en | main | Save (`ads-capi-save`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00626 | admin | /design | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00627 | admin | /design | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/design?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00628 | admin | /design | desktop | zh-TW | main | 預覽 (`design-preview`) | click | visible change | popup: about:blank; DOM changed (4 mutations) | PASS |
| r00629 | admin | /design | desktop | zh-TW | main | 店鋪資料 (`design-tab-profile`) | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00630 | admin | /design | desktop | zh-TW | main | 導覽 (`design-tab-nav`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00631 | admin | /design | desktop | zh-TW | main | 首頁 (`design-tab-home`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00632 | admin | /design | desktop | zh-TW | main | 資訊頁 (`design-tab-pages`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00633 | admin | /design | desktop | zh-TW | main | 版本 (`design-tab-versions`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00634 | admin | /design | desktop | zh-TW | main | 選擇圖片 (`design-logo-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00635 | admin | /design | desktop | zh-TW | main | 選擇圖片 (`design-favicon-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00636 | admin | /design | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00637 | admin | /design | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/design?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (11 mutations) | PASS |
| r00638 | admin | /design | mobile | zh-TW | main | 預覽 (`design-preview`) | click | visible change | popup: about:blank; DOM changed (4 mutations) | PASS |
| r00639 | admin | /design | mobile | zh-TW | main | 店鋪資料 (`design-tab-profile`) | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00640 | admin | /design | mobile | zh-TW | main | 導覽 (`design-tab-nav`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00641 | admin | /design | mobile | zh-TW | main | 首頁 (`design-tab-home`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00642 | admin | /design | mobile | zh-TW | main | 資訊頁 (`design-tab-pages`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00643 | admin | /design | mobile | zh-TW | main | 版本 (`design-tab-versions`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00644 | admin | /design | mobile | zh-TW | main | 選擇圖片 (`design-logo-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00645 | admin | /design | mobile | zh-TW | main | 選擇圖片 (`design-favicon-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true); scrolled | PASS |
| r00646 | admin | /design | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00647 | admin | /design | desktop | en | main | Overview | click | navigation or in-page change | url /en/design?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00648 | admin | /design | desktop | en | main | Preview (`design-preview`) | click | visible change | popup: about:blank; DOM changed (4 mutations) | PASS |
| r00649 | admin | /design | desktop | en | main | Store profile (`design-tab-profile`) | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00650 | admin | /design | desktop | en | main | Navigation (`design-tab-nav`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00651 | admin | /design | desktop | en | main | Home page (`design-tab-home`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00652 | admin | /design | desktop | en | main | Pages (`design-tab-pages`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00653 | admin | /design | desktop | en | main | Versions (`design-tab-versions`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|\|false -> \|\|true) | PASS |
| r00654 | admin | /design | desktop | en | main | Choose image (`design-logo-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00655 | admin | /design | desktop | en | main | Choose image (`design-favicon-choose`) | click | visible change | DOM changed (2 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00656 | admin | /finance | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00657 | admin | /finance | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/finance?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00658 | admin | /finance | desktop | zh-TW | main | 查看 (`finance-show`) | click | visible change | url /zh-TW/finance?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/finance?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&from=2026-09-07&to=2026-10-06; DOM ch | PASS |
| r00659 | admin | /finance | desktop | zh-TW | main | 下載 CSV (`finance-csv`) | click | navigation or in-page change | download: finance-2026-09-07-2026-10-06.csv | PASS |
| r00660 | admin | /finance | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00661 | admin | /finance | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/finance?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00662 | admin | /finance | mobile | zh-TW | main | 查看 (`finance-show`) | click | visible change | url /zh-TW/finance?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW/finance?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&from=2026-09-07&to=2026-10-06; DOM ch | PASS |
| r00663 | admin | /finance | mobile | zh-TW | main | 下載 CSV (`finance-csv`) | click | navigation or in-page change | download: finance-2026-09-07-2026-10-06.csv | PASS |
| r00664 | admin | /finance | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00665 | admin | /finance | desktop | en | main | Overview | click | navigation or in-page change | url /en/finance?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00666 | admin | /finance | desktop | en | main | Show (`finance-show`) | click | visible change | url /en/finance?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en/finance?store=8daf0ef2-530e-4107-bece-7f4c5360df7f&from=2026-09-07&to=2026-10-06; DOM changed  | PASS |
| r00667 | admin | /finance | desktop | en | main | Download CSV (`finance-csv`) | click | navigation or in-page change | download: finance-2026-09-07-2026-10-06.csv | PASS |
| r00668 | admin | /settings | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00669 | admin | /settings | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/settings?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00670 | admin | /settings | desktop | zh-TW | main | 1 選擇平台 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00671 | admin | /settings | desktop | zh-TW | main | PAYUNi 收款 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00672 | admin | /settings | desktop | zh-TW | main | 商家自行安排配送 | click | visible change | DOM changed (8 mutations); the control's own state changed (false -> true) | PASS |
| r00673 | admin | /settings | desktop | zh-TW | main | 繼續 | click | visible change | DOM changed (12 mutations) | PASS |
| r00674 | admin | /settings | desktop | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00675 | admin | /settings | desktop | zh-TW | main | 取消發佈 (`storefront-toggle`) | click | confirmation layer opens; cancel; nothing changed | layer opened (取消發佈); DOM changed (4 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00676 | admin | /settings | desktop | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00677 | admin | /settings | desktop | zh-TW | main | 暫停 (`storefront-domain-suspend`) | click | confirmation layer opens; cancel; nothing changed | layer opened (暫停); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00678 | admin | /settings | desktop | zh-TW | main | 解除綁定 (`storefront-domain-detach`) | click | confirmation layer opens; cancel; nothing changed | layer opened (解除綁定); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00679 | admin | /settings | desktop | zh-TW | main | 請求驗證 (`storefront-domain-submit`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00680 | admin | /settings | desktop | zh-TW | main | 新增專頁 (`metaconnect-add`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00681 | admin | /settings | desktop | zh-TW | main | 選擇專頁重新授權 (`metaconnect-reconnect`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00682 | admin | /settings | desktop | zh-TW | main | 中斷連接 (`metaconnect-disconnect`) | click | confirmation layer opens; cancel; nothing changed | layer opened (中斷連接: Sweep Page); DOM changed (5 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00683 | admin | /settings | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00684 | admin | /settings | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/settings?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00685 | admin | /settings | mobile | zh-TW | main | 1 選擇平台 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00686 | admin | /settings | mobile | zh-TW | main | PAYUNi 收款 | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00687 | admin | /settings | mobile | zh-TW | main | 商家自行安排配送 | click | visible change | DOM changed (8 mutations); the control's own state changed (false -> true) | PASS |
| r00688 | admin | /settings | mobile | zh-TW | main | 繼續 | click | visible change | DOM changed (12 mutations) | PASS |
| r00689 | admin | /settings | mobile | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00690 | admin | /settings | mobile | zh-TW | main | 取消發佈 (`storefront-toggle`) | click | confirmation layer opens; cancel; nothing changed | layer opened (取消發佈); DOM changed (4 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00691 | admin | /settings | mobile | zh-TW | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00692 | admin | /settings | mobile | zh-TW | main | 暫停 (`storefront-domain-suspend`) | click | confirmation layer opens; cancel; nothing changed | layer opened (暫停); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00693 | admin | /settings | mobile | zh-TW | main | 解除綁定 (`storefront-domain-detach`) | click | confirmation layer opens; cancel; nothing changed | layer opened (解除綁定); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00694 | admin | /settings | mobile | zh-TW | main | 請求驗證 (`storefront-domain-submit`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00695 | admin | /settings | mobile | zh-TW | main | 新增專頁 (`metaconnect-add`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00696 | admin | /settings | mobile | zh-TW | main | 選擇專頁重新授權 (`metaconnect-reconnect`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00697 | admin | /settings | mobile | zh-TW | main | 中斷連接 (`metaconnect-disconnect`) | click | confirmation layer opens; cancel; nothing changed | layer opened (中斷連接: Sweep Page); DOM changed (5 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00698 | admin | /settings | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00699 | admin | /settings | desktop | en | main | Overview | click | navigation or in-page change | url /en/settings?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00700 | admin | /settings | desktop | en | main | 1 Choose platform | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00701 | admin | /settings | desktop | en | main | PAYUNi payment | click | visible change | no change (the current item) (already the current item: no change expected) | PASS |
| r00702 | admin | /settings | desktop | en | main | Merchant-arranged delivery | click | visible change | DOM changed (8 mutations); the control's own state changed (false -> true) | PASS |
| r00703 | admin | /settings | desktop | en | main | Continue | click | visible change | DOM changed (12 mutations) | PASS |
| r00704 | admin | /settings | desktop | en | main | https://buyer.example | click | visible change | popup: https://buyer.example/zh-TW [1 of 2 alike] | PASS |
| r00705 | admin | /settings | desktop | en | main | Unpublish storefront (`storefront-toggle`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Unpublish storefront); DOM changed (4 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00706 | admin | /settings | desktop | en | main | https://buyer.example | click | visible change | scrolled [1 of 2 alike] | PASS |
| r00707 | admin | /settings | desktop | en | main | Suspend (`storefront-domain-suspend`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Suspend); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00708 | admin | /settings | desktop | en | main | Detach (`storefront-domain-detach`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Detach); DOM changed (1 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00709 | admin | /settings | desktop | en | main | Request verification (`storefront-domain-submit`) | click | visible change | DOM changed (1 mutations); scrolled | PASS |
| r00710 | admin | /settings | desktop | en | main | Add Page (`metaconnect-add`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00711 | admin | /settings | desktop | en | main | Choose a Page to reauthorize (`metaconnect-reconnect`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00712 | admin | /settings | desktop | en | main | Disconnect (`metaconnect-disconnect`) | click | confirmation layer opens; cancel; nothing changed | layer opened (Disconnect: Sweep Page); DOM changed (5 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00713 | admin | /team | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00714 | admin | /team | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/team?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00715 | admin | /team | desktop | zh-TW | main | 角色 (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00716 | admin | /team | desktop | zh-TW | main | 寄送邀請 (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00717 | admin | /team | desktop | zh-TW | main | 變更角色 (`member-role-9c6f4f26-ee92-49b2-8f97-adcbac4681a3`) | selectOption(admin) | visible change | DOM changed (4 mutations) | PASS |
| r00718 | admin | /team | desktop | zh-TW | main | 移除 (`member-remove-9c6f4f26-ee92-49b2-8f97-adcbac4681a3`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (5 mutations) (inline change only; no request fired) | PASS |
| r00719 | admin | /team | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00720 | admin | /team | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/team?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (11 mutations) | PASS |
| r00721 | admin | /team | mobile | zh-TW | main | 角色 (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00722 | admin | /team | mobile | zh-TW | main | 寄送邀請 (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00723 | admin | /team | mobile | zh-TW | main | 變更角色 (`member-role-9c6f4f26-ee92-49b2-8f97-adcbac4681a3`) | selectOption(admin) | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00724 | admin | /team | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00725 | admin | /team | desktop | en | main | Overview | click | navigation or in-page change | url /en/team?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00726 | admin | /team | desktop | en | main | Role (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00727 | admin | /team | desktop | en | main | Send invitation (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00728 | admin | /team | desktop | en | main | Change role (`member-role-9c6f4f26-ee92-49b2-8f97-adcbac4681a3`) | selectOption(admin) | visible change | DOM changed (4 mutations) | PASS |
| r00729 | admin | /team | desktop | en | main | Remove (`member-remove-9c6f4f26-ee92-49b2-8f97-adcbac4681a3`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (5 mutations) (inline change only; no request fired) | PASS |
| r00730 | admin | /billing | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00731 | admin | /billing | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/billing?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (14 mutations) | PASS |
| r00732 | admin | /billing | desktop | zh-TW | main | 訂閱 (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00733 | admin | /billing | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00734 | admin | /billing | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/billing?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /zh-TW?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (15 mutations) | PASS |
| r00735 | admin | /billing | mobile | zh-TW | main | 管理付款與發票 (`billing-portal`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00736 | admin | /billing | mobile | zh-TW | main | 訂閱 (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00737 | admin | /billing | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00738 | admin | /billing | desktop | en | main | Overview | click | navigation or in-page change | url /en/billing?store=8daf0ef2-530e-4107-bece-7f4c5360df7f -> /en?store=8daf0ef2-530e-4107-bece-7f4c5360df7f; DOM changed (10 mutations) | PASS |
| r00739 | admin | /billing | desktop | en | main | Manage payment & invoices (`billing-portal`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00740 | admin | /billing | desktop | en | main | Subscribe (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00741 | admin | /invite/[token] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00742 | admin | /invite/[token] | desktop | zh-TW | main | 登入 (`invite-signin`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (34 mu | PASS |
| r00743 | admin | /invite/[token] | desktop | zh-TW | main | 註冊帳號 (`invite-signup`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW/signup#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed | PASS |
| r00744 | admin | /invite/[token] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00745 | admin | /invite/[token] | mobile | zh-TW | main | 登入 (`invite-signin`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (34 mu | PASS |
| r00746 | admin | /invite/[token] | mobile | zh-TW | main | 註冊帳號 (`invite-signup`) | click | navigation or in-page change | url /zh-TW/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /zh-TW/signup#next=%2Fzh-TW%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed | PASS |
| r00747 | admin | /invite/[token] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00748 | admin | /invite/[token] | desktop | en | main | Sign in (`invite-signin`) | click | navigation or in-page change | url /en/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /en#next=%2Fen%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (34 mutations); | PASS |
| r00749 | admin | /invite/[token] | desktop | en | main | Create an account (`invite-signup`) | click | navigation or in-page change | url /en/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /en/signup#next=%2Fen%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (23 muta | PASS |
| r00750 | admin | /reset | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00751 | admin | /reset | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00752 | admin | /reset | desktop | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00753 | admin | /reset | desktop | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/reset -> /zh-TW; DOM changed (16 mutations); validation shown | PASS |
| r00754 | admin | /reset | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00755 | admin | /reset | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00756 | admin | /reset | mobile | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00757 | admin | /reset | mobile | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/reset -> /zh-TW; DOM changed (16 mutations); validation shown | PASS |
| r00758 | admin | /reset | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00759 | admin | /reset | desktop | en | main | Language | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00760 | admin | /reset | desktop | en | main | Send code | click | visible change | validation shown | PASS |
| r00761 | admin | /reset | desktop | en | main | Back to sign in | click | navigation or in-page change | url /en/reset -> /en; DOM changed (16 mutations); validation shown | PASS |
| r00762 | admin | /signup | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00763 | admin | /signup | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00764 | admin | /signup | desktop | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00765 | admin | /signup | desktop | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/signup -> /zh-TW; DOM changed (9 mutations) | PASS |
| r00766 | admin | /signup | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00767 | admin | /signup | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00768 | admin | /signup | mobile | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00769 | admin | /signup | mobile | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/signup -> /zh-TW; DOM changed (9 mutations) | PASS |
| r00770 | admin | /signup | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00771 | admin | /signup | desktop | en | main | Language | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00772 | admin | /signup | desktop | en | main | Send code | click | visible change | validation shown | PASS |
| r00773 | admin | /signup | desktop | en | main | Back to sign in | click | navigation or in-page change | url /en/signup -> /en; DOM changed (9 mutations) | PASS |
| r00774 | storefront | /cart | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00775 | storefront | /cart | desktop | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00776 | storefront | /cart | desktop | zh-TW | main | 增加數量 | click | visible change | DOM changed (3 mutations) | PASS |
| r00777 | storefront | /cart | desktop | zh-TW | main | 將 Sweep Wool Scarf 移出購物車 | click | visible change | DOM changed (3 mutations) | PASS |
| r00778 | storefront | /cart | desktop | zh-TW | main | 前往結帳 (`cart-checkout`) | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/checkout; DOM changed (5 mutations) | PASS |
| r00779 | storefront | /cart | desktop | zh-TW | main | 繼續選購 | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00780 | storefront | /checkout | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00781 | storefront | /checkout | desktop | zh-TW | main | 我的訂單 (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00782 | storefront | /checkout | desktop | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/products/sweep-wool-scarf; DOM changed (7 mutations) | PASS |
| r00783 | storefront | /checkout | desktop | zh-TW | main | 返回購物車 | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/cart; DOM changed (9 mutations) | PASS |
| r00784 | storefront | /checkout | desktop | zh-TW | main | 選擇配送 | click | visible change | DOM changed (3 mutations) | PASS |
| r00785 | storefront | /orders/[orderID] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00786 | storefront | /orders/[orderID] | desktop | zh-TW | main | 重新整理訂單 (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone) | PASS |
| r00787 | storefront | / | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00788 | storefront | / | desktop | zh-TW | chrome | 跳到主要內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW -> /zh-TW#main; scrolled | PASS |
| r00789 | storefront | / | desktop | zh-TW | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00790 | storefront | / | desktop | zh-TW | chrome | All products | click | navigation or in-page change | url /zh-TW -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00791 | storefront | / | desktop | zh-TW | chrome | Sweep home | click | navigation or in-page change | url /zh-TW -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00792 | storefront | / | desktop | zh-TW | chrome | About us | click | navigation or in-page change | url /zh-TW -> /zh-TW/pages/about; DOM changed (4 mutations) | PASS |
| r00793 | storefront | / | desktop | zh-TW | chrome | 搜尋 | click | visible change | url /zh-TW -> /zh-TW/search?q= | PASS |
| r00794 | storefront | / | desktop | zh-TW | chrome | 購物車，1 件商品 (`header-cart`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00795 | storefront | / | desktop | zh-TW | chrome | +886 2 2345 6789 | click | navigation or in-page change | scrolled | PASS |
| r00796 | storefront | / | desktop | zh-TW | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r00797 | storefront | / | desktop | zh-TW | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r00798 | storefront | / | desktop | zh-TW | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r00799 | storefront | / | desktop | zh-TW | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r00800 | storefront | / | desktop | zh-TW | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00801 | storefront | / | desktop | zh-TW | chrome | 防詐騙提醒 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/anti-fraud | PASS |
| r00802 | storefront | / | desktop | zh-TW | chrome | 資料刪除 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/data-deletion (inline change only; no request fired) | PASS |
| r00803 | storefront | / | desktop | zh-TW | chrome | 简体中文 | click | navigation or in-page change | url /zh-TW -> /zh-CN | PASS |
| r00804 | storefront | / | desktop | zh-TW | chrome | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r00805 | storefront | / | desktop | zh-TW | chrome | English | click | navigation or in-page change | url /zh-TW -> /en | PASS |
| r00806 | storefront | / | desktop | zh-TW | main | 查看全部 | click | navigation or in-page change | url /zh-TW -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00807 | storefront | / | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/t04-64977906744e; DOM changed (4 mutations) [1 of 2 alike] | PASS |
| r00808 | storefront | / | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00809 | storefront | / | desktop | zh-TW | main | 查看全部 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00810 | storefront | / | desktop | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00811 | storefront | / | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00812 | storefront | / | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00813 | storefront | /products | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00814 | storefront | /products | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00815 | storefront | /products | desktop | zh-TW | main | 全部商品 | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00816 | storefront | /products | desktop | zh-TW | main | Sweep home | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00817 | storefront | /products | desktop | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00818 | storefront | /products | desktop | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00819 | storefront | /products | desktop | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00820 | storefront | /products | desktop | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/products -> /zh-TW/products?min=&max=&sort=newest | PASS |
| r00821 | storefront | /products | desktop | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00822 | storefront | /products | desktop | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /zh-TW/products?min=&max=&sort=price_asc -> /zh-TW/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r00823 | storefront | /products | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00824 | storefront | /products | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/t04-64977906744e; DOM changed (5 mutations) | PASS |
| r00825 | storefront | /collections | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00826 | storefront | /collections | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00827 | storefront | /collections | desktop | zh-TW | main | S Sweep home 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00828 | storefront | /collections | desktop | zh-TW | main | S Sweep wear 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00829 | storefront | /collections/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00830 | storefront | /collections/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00831 | storefront | /collections/[slug] | desktop | zh-TW | main | 商品分類 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections; DOM changed (4 mutations) | PASS |
| r00832 | storefront | /collections/[slug] | desktop | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00833 | storefront | /collections/[slug] | desktop | zh-TW | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00834 | storefront | /collections/[slug] | desktop | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00835 | storefront | /collections/[slug] | desktop | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00836 | storefront | /collections/[slug] | desktop | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00837 | storefront | /collections/[slug] | desktop | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-home?min=&max=&sort=newest | PASS |
| r00838 | storefront | /collections/[slug] | desktop | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00839 | storefront | /collections/[slug] | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/collections/sweep-home?min=&max=&sort=price_asc -> /zh-TW/products/t04-64977906744e; DOM changed (5 mutations) | PASS |
| r00840 | storefront | /collections/[slug] | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00841 | storefront | /products/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00842 | storefront | /products/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00843 | storefront | /products/[slug] | desktop | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00844 | storefront | /products/[slug] | desktop | zh-TW | main | 放大圖片 | click | visible change | layer opened (Sweep Wool Scarf — 放大圖片); DOM changed (2 mutations) | PASS |
| r00845 | storefront | /products/[slug] | desktop | zh-TW | main | 增加數量 | click | visible change | DOM changed (2 mutations) | PASS |
| r00846 | storefront | /products/[slug] | desktop | zh-TW | main | 加入購物車 (`add-to-cart`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00847 | storefront | /products/[slug] | desktop | zh-TW | main | 立即購買 (`buy-now`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00848 | storefront | /products/[slug] | desktop | zh-TW | main | Sweep home | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00849 | storefront | /products/[slug] | desktop | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00850 | storefront | /products/[slug] | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r00851 | storefront | /search | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00852 | storefront | /search | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00853 | storefront | /search | desktop | zh-TW | main | 搜尋 | click | visible change | the control's own state changed ( -> gone) | PASS |
| r00854 | storefront | /search | desktop | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00855 | storefront | /search | desktop | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00856 | storefront | /search | desktop | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/search?q=Sweep -> /zh-TW/search?q=Sweep&min=&max=&sort=newest | PASS |
| r00857 | storefront | /search | desktop | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00858 | storefront | /search | desktop | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /zh-TW/search?q=Sweep&min=&max=&sort=price_asc -> /zh-TW/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r00859 | storefront | /search | desktop | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00860 | storefront | /search | desktop | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW/products/t04-64977906744e; DOM changed (5 mutations) | PASS |
| r00861 | storefront | /orders/lookup | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00862 | storefront | /orders/lookup | desktop | zh-TW | main | 查詢 (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00863 | storefront | /order-link | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00864 | storefront | /order-link | desktop | zh-TW | main | 查詢訂單 | click | navigation or in-page change | url /zh-TW/order-link -> /zh-TW/orders/lookup; DOM changed (12 mutations) | PASS |
| r00865 | storefront | /claim | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00866 | storefront | /claim | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r00867 | storefront | /legal/anti-fraud | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00868 | storefront | /legal/anti-fraud | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00869 | storefront | /legal/anti-fraud | desktop | zh-TW | main | 內政部警政署 — 165 全民防騙網 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r00870 | storefront | /legal/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00871 | storefront | /legal/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/legal/refunds -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00872 | storefront | /legal/[slug] | desktop | zh-TW | main | 回到商店 | click | navigation or in-page change | url /zh-TW/legal/refunds -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00873 | storefront | /privacy | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00874 | storefront | /privacy | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/privacy -> /zh-CN/privacy; DOM changed (30 mutations) | PASS |
| r00875 | storefront | /privacy | desktop | zh-TW | main | 開啟: 允許店鋪透過 Messenger 或 Instagram 私訊向我發送行銷訊息。 (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00876 | storefront | /privacy | desktop | zh-TW | main | 開啟: 使用我的購買紀錄為我個人化 Meta 廣告。 (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00877 | storefront | /privacy | desktop | zh-TW | main | 下載我的資料 (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r00878 | storefront | /privacy | desktop | zh-TW | main | 抹除我的資料… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations) (confirmation shown, then cancelled) | PASS |
| r00879 | storefront | /data-deletion | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00880 | storefront | /data-deletion | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/data-deletion -> /zh-CN/data-deletion | PASS |
| r00881 | storefront | /data-deletion | desktop | zh-TW | main | 開啟隱私頁面 (`data-deletion-privacy-link`) | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-TW/privacy; DOM changed (2 mutations) | PASS |
| r00882 | storefront | /pages/[slug] | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00883 | storefront | /pages/[slug] | desktop | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/pages/about -> /zh-TW; DOM changed (4 mutations) | PASS |
| r00884 | storefront | /cart | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 (keeps updating) | PASS |
| r00885 | storefront | /cart | mobile | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00886 | storefront | /cart | mobile | zh-TW | main | 增加數量 | click | visible change | DOM changed (3 mutations) | PASS |
| r00887 | storefront | /cart | mobile | zh-TW | main | 將 Sweep Wool Scarf 移出購物車 | click | visible change | DOM changed (3 mutations) | PASS |
| r00888 | storefront | /cart | mobile | zh-TW | main | 前往結帳 (`cart-checkout`) | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/checkout; DOM changed (11 mutations) | PASS |
| r00889 | storefront | /cart | mobile | zh-TW | main | 繼續選購 | click | navigation or in-page change | url /zh-TW/cart -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00890 | storefront | /checkout | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00891 | storefront | /checkout | mobile | zh-TW | main | 我的訂單 (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00892 | storefront | /checkout | mobile | zh-TW | main | Sweep Wool Scarf | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/products/sweep-wool-scarf; DOM changed (6 mutations) | PASS |
| r00893 | storefront | /checkout | mobile | zh-TW | main | 返回購物車 | click | navigation or in-page change | url /zh-TW/checkout -> /zh-TW/cart; DOM changed (9 mutations) | PASS |
| r00894 | storefront | /checkout | mobile | zh-TW | main | 選擇配送 | click | visible change | DOM changed (3 mutations) | PASS |
| r00895 | storefront | /orders/[orderID] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00896 | storefront | /orders/[orderID] | mobile | zh-TW | main | 重新整理訂單 (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone); scrolled | PASS |
| r00897 | storefront | / | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00898 | storefront | / | mobile | zh-TW | chrome | 跳到主要內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW -> /zh-TW#main; scrolled | PASS |
| r00899 | storefront | / | mobile | zh-TW | chrome | 選單 (`menu-open`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00900 | storefront | / | mobile | zh-TW | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00901 | storefront | / | mobile | zh-TW | chrome | 搜尋 | click | navigation or in-page change | url /zh-TW -> /zh-TW/search; DOM changed (4 mutations) | PASS |
| r00902 | storefront | / | mobile | zh-TW | chrome | 購物車，1 件商品 (`header-cart`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00903 | storefront | / | mobile | zh-TW | chrome | +886 2 2345 6789 | click | navigation or in-page change | scrolled | PASS |
| r00904 | storefront | / | mobile | zh-TW | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r00905 | storefront | / | mobile | zh-TW | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r00906 | storefront | / | mobile | zh-TW | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r00907 | storefront | / | mobile | zh-TW | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r00908 | storefront | / | mobile | zh-TW | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00909 | storefront | / | mobile | zh-TW | chrome | 防詐騙提醒 | click | navigation or in-page change | url /zh-TW -> /zh-TW/legal/anti-fraud | PASS |
| r00910 | storefront | / | mobile | zh-TW | chrome | 資料刪除 | click | confirmation layer opens; cancel; nothing changed | url /zh-TW -> /zh-TW/data-deletion (inline change only; no request fired) | PASS |
| r00911 | storefront | / | mobile | zh-TW | chrome | 简体中文 | click | navigation or in-page change | url /zh-TW -> /zh-CN | PASS |
| r00912 | storefront | / | mobile | zh-TW | chrome | 繁體中文 | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r00913 | storefront | / | mobile | zh-TW | chrome | English | click | navigation or in-page change | url /zh-TW -> /en | PASS |
| r00914 | storefront | / | mobile | zh-TW | main | 查看全部 | click | navigation or in-page change | url /zh-TW -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00915 | storefront | / | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/t04-64977906744e; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00916 | storefront | / | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00917 | storefront | / | mobile | zh-TW | main | 查看全部 | click | navigation or in-page change | scrolled | PASS |
| r00918 | storefront | / | mobile | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00919 | storefront | / | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00920 | storefront | / | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00921 | storefront | /products | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00922 | storefront | /products | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00923 | storefront | /products | mobile | zh-TW | main | 全部商品 | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00924 | storefront | /products | mobile | zh-TW | main | Sweep home | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00925 | storefront | /products | mobile | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00926 | storefront | /products | mobile | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00927 | storefront | /products | mobile | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00928 | storefront | /products | mobile | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/products -> /zh-TW/products?min=&max=&sort=newest | PASS |
| r00929 | storefront | /products | mobile | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00930 | storefront | /products | mobile | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00931 | storefront | /products | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/products -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00932 | storefront | /products | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r00933 | storefront | /collections | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00934 | storefront | /collections | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00935 | storefront | /collections | mobile | zh-TW | main | S Sweep home 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00936 | storefront | /collections | mobile | zh-TW | main | S Sweep wear 2 件商品 (`collection-tile`) | click | navigation or in-page change | url /zh-TW/collections -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00937 | storefront | /collections/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00938 | storefront | /collections/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00939 | storefront | /collections/[slug] | mobile | zh-TW | main | 商品分類 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections; DOM changed (4 mutations) | PASS |
| r00940 | storefront | /collections/[slug] | mobile | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00941 | storefront | /collections/[slug] | mobile | zh-TW | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00942 | storefront | /collections/[slug] | mobile | zh-TW | main | Sweep wear | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00943 | storefront | /collections/[slug] | mobile | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00944 | storefront | /collections/[slug] | mobile | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00945 | storefront | /collections/[slug] | mobile | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/collections/sweep-home -> /zh-TW/collections/sweep-home?min=&max=&sort=newest | PASS |
| r00946 | storefront | /collections/[slug] | mobile | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00947 | storefront | /collections/[slug] | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /zh-TW/collections/sweep-home?min=&max=&sort=price_asc -> /zh-TW/products/t04-64977906744e; DOM changed (5 mutations) | PASS |
| r00948 | storefront | /collections/[slug] | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/collections/sweep-home -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00949 | storefront | /products/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00950 | storefront | /products/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00951 | storefront | /products/[slug] | mobile | zh-TW | main | 全部商品 | click | navigation or in-page change | url /zh-TW/products/sweep-wool-scarf -> /zh-TW/products; DOM changed (4 mutations) | PASS |
| r00952 | storefront | /products/[slug] | mobile | zh-TW | main | 放大圖片 | click | visible change | layer opened (Sweep Wool Scarf — 放大圖片); DOM changed (2 mutations) | PASS |
| r00953 | storefront | /products/[slug] | mobile | zh-TW | main | 增加數量 | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00954 | storefront | /products/[slug] | mobile | zh-TW | main | 加入購物車 (`add-to-cart`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00955 | storefront | /products/[slug] | mobile | zh-TW | main | 立即購買 (`buy-now`) | click | visible change | DOM changed (3 mutations); scrolled | PASS |
| r00956 | storefront | /products/[slug] | mobile | zh-TW | main | Sweep home | click | navigation or in-page change | scrolled | PASS |
| r00957 | storefront | /products/[slug] | mobile | zh-TW | main | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00958 | storefront | /products/[slug] | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r00959 | storefront | /search | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00960 | storefront | /search | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00961 | storefront | /search | mobile | zh-TW | main | 搜尋 | click | visible change | the control's own state changed ( -> gone) | PASS |
| r00962 | storefront | /search | mobile | zh-TW | main | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00963 | storefront | /search | mobile | zh-TW | layer of 價格 | 價格 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00964 | storefront | /search | mobile | zh-TW | layer of 價格 | 套用 | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /zh-TW/search?q=Sweep -> /zh-TW/search?q=Sweep&min=&max=&sort=newest | PASS |
| r00965 | storefront | /search | mobile | zh-TW | main | 排序 | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00966 | storefront | /search | mobile | zh-TW | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00967 | storefront | /search | mobile | zh-TW | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /zh-TW/search?q=Sweep -> /zh-TW/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00968 | storefront | /search | mobile | zh-TW | main | 無圖片 Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r00969 | storefront | /orders/lookup | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00970 | storefront | /orders/lookup | mobile | zh-TW | main | 查詢 (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00971 | storefront | /order-link | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00972 | storefront | /order-link | mobile | zh-TW | main | 查詢訂單 | click | navigation or in-page change | url /zh-TW/order-link -> /zh-TW/orders/lookup; DOM changed (12 mutations) | PASS |
| r00973 | storefront | /claim | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00974 | storefront | /claim | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r00975 | storefront | /legal/anti-fraud | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00976 | storefront | /legal/anti-fraud | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00977 | storefront | /legal/anti-fraud | mobile | zh-TW | main | 內政部警政署 — 165 全民防騙網 | click | navigation or in-page change | url /zh-TW/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r00978 | storefront | /legal/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00979 | storefront | /legal/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/legal/refunds -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00980 | storefront | /legal/[slug] | mobile | zh-TW | main | 回到商店 | click | navigation or in-page change | url /zh-TW/legal/refunds -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00981 | storefront | /privacy | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00982 | storefront | /privacy | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | url /zh-TW/privacy -> /zh-CN/privacy; DOM changed (30 mutations) | PASS |
| r00983 | storefront | /privacy | mobile | zh-TW | main | 開啟: 允許店鋪透過 Messenger 或 Instagram 私訊向我發送行銷訊息。 (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00984 | storefront | /privacy | mobile | zh-TW | main | 開啟: 使用我的購買紀錄為我個人化 Meta 廣告。 (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00985 | storefront | /privacy | mobile | zh-TW | main | 下載我的資料 (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r00986 | storefront | /privacy | mobile | zh-TW | main | 抹除我的資料… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00987 | storefront | /data-deletion | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00988 | storefront | /data-deletion | mobile | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00989 | storefront | /data-deletion | mobile | zh-TW | main | 開啟隱私頁面 (`data-deletion-privacy-link`) | click | navigation or in-page change | url /zh-TW/data-deletion -> /zh-TW/privacy; DOM changed (2 mutations) | PASS |
| r00990 | storefront | /pages/[slug] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00991 | storefront | /pages/[slug] | mobile | zh-TW | main | 首頁 | click | navigation or in-page change | url /zh-TW/pages/about -> /zh-TW; DOM changed (5 mutations) | PASS |
| r00992 | storefront | /cart | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 (keeps updating) | PASS |
| r00993 | storefront | /cart | desktop | en | main | Sweep Wool Scarf | click | navigation or in-page change | url /en/cart -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00994 | storefront | /cart | desktop | en | main | Increase quantity | click | visible change | DOM changed (3 mutations) | PASS |
| r00995 | storefront | /cart | desktop | en | main | Remove Sweep Wool Scarf from the cart | click | confirmation layer opens; cancel; nothing changed | DOM changed (3 mutations) (inline change only; no request fired) | PASS |
| r00996 | storefront | /cart | desktop | en | main | Checkout (`cart-checkout`) | click | navigation or in-page change | url /en/cart -> /en/checkout; DOM changed (5 mutations) | PASS |
| r00997 | storefront | /cart | desktop | en | main | Continue shopping | click | navigation or in-page change | url /en/cart -> /en/products; DOM changed (4 mutations) | PASS |
| r00998 | storefront | /checkout | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00999 | storefront | /checkout | desktop | en | main | Your orders (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r01000 | storefront | /checkout | desktop | en | main | Sweep Wool Scarf | click | navigation or in-page change | url /en/checkout -> /en/products/sweep-wool-scarf; DOM changed (7 mutations) | PASS |
| r01001 | storefront | /checkout | desktop | en | main | Back to cart | click | navigation or in-page change | url /en/checkout -> /en/cart; DOM changed (9 mutations) | PASS |
| r01002 | storefront | /checkout | desktop | en | main | Choose delivery | click | visible change | DOM changed (3 mutations) | PASS |
| r01003 | storefront | /orders/[orderID] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01004 | storefront | /orders/[orderID] | desktop | en | main | Refresh order (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone) | PASS |
| r01005 | storefront | / | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01006 | storefront | / | desktop | en | chrome | Skip to content | keyboard: Tab to the link, Enter | navigation or in-page change | url /en -> /en#main; scrolled | PASS |
| r01007 | storefront | / | desktop | en | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r01008 | storefront | / | desktop | en | chrome | All products | click | navigation or in-page change | url /en -> /en/products; DOM changed (4 mutations) | PASS |
| r01009 | storefront | / | desktop | en | chrome | Sweep home | click | navigation or in-page change | url /en -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r01010 | storefront | / | desktop | en | chrome | About us | click | navigation or in-page change | url /en -> /en/pages/about; DOM changed (4 mutations) | PASS |
| r01011 | storefront | / | desktop | en | chrome | Search | click | visible change | url /en -> /en/search?q= | PASS |
| r01012 | storefront | / | desktop | en | chrome | Cart, 1 items (`header-cart`) | click | visible change | DOM changed (2 mutations) | PASS |
| r01013 | storefront | / | desktop | en | chrome | +886 2 2345 6789 | click | navigation or in-page change | scrolled | PASS |
| r01014 | storefront | / | desktop | en | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r01015 | storefront | / | desktop | en | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r01016 | storefront | / | desktop | en | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r01017 | storefront | / | desktop | en | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r01018 | storefront | / | desktop | en | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r01019 | storefront | / | desktop | en | chrome | Shop safely | click | navigation or in-page change | url /en -> /en/legal/anti-fraud | PASS |
| r01020 | storefront | / | desktop | en | chrome | Data deletion | click | navigation or in-page change | url /en -> /en/data-deletion | PASS |
| r01021 | storefront | / | desktop | en | chrome | 简体中文 | click | navigation or in-page change | url /en -> /zh-CN | PASS |
| r01022 | storefront | / | desktop | en | chrome | 繁體中文 | click | navigation or in-page change | url /en -> /zh-TW | PASS |
| r01023 | storefront | / | desktop | en | chrome | English | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r01024 | storefront | / | desktop | en | main | View all | click | navigation or in-page change | url /en -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r01025 | storefront | / | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en -> /en/products/t04-64977906744e; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r01026 | storefront | / | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r01027 | storefront | / | desktop | en | main | View all | click | navigation or in-page change | url /en -> /en/products; DOM changed (4 mutations) | PASS |
| r01028 | storefront | / | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r01029 | storefront | / | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r01030 | storefront | / | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r01031 | storefront | /products | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01032 | storefront | /products | desktop | en | main | Home | click | navigation or in-page change | url /en/products -> /en; DOM changed (4 mutations) | PASS |
| r01033 | storefront | /products | desktop | en | main | All products | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r01034 | storefront | /products | desktop | en | main | Sweep home | click | navigation or in-page change | url /en/products -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r01035 | storefront | /products | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/products -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r01036 | storefront | /products | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r01037 | storefront | /products | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r01038 | storefront | /products | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/products -> /en/products?min=&max=&sort=newest | PASS |
| r01039 | storefront | /products | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01040 | storefront | /products | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /en/products?min=&max=&sort=price_asc -> /en/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r01041 | storefront | /products | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/products -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01042 | storefront | /products | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/products -> /en/products/t04-64977906744e; DOM changed (5 mutations) | PASS |
| r01043 | storefront | /collections | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01044 | storefront | /collections | desktop | en | main | Home | click | navigation or in-page change | url /en/collections -> /en; DOM changed (4 mutations) | PASS |
| r01045 | storefront | /collections | desktop | en | main | S Sweep home 2 products (`collection-tile`) | click | navigation or in-page change | url /en/collections -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r01046 | storefront | /collections | desktop | en | main | S Sweep wear 2 products (`collection-tile`) | click | navigation or in-page change | url /en/collections -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r01047 | storefront | /collections/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01048 | storefront | /collections/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/collections/sweep-home -> /en; DOM changed (4 mutations) | PASS |
| r01049 | storefront | /collections/[slug] | desktop | en | main | Collections | click | navigation or in-page change | url /en/collections/sweep-home -> /en/collections; DOM changed (4 mutations) | PASS |
| r01050 | storefront | /collections/[slug] | desktop | en | main | All products | click | navigation or in-page change | url /en/collections/sweep-home -> /en/products; DOM changed (4 mutations) | PASS |
| r01051 | storefront | /collections/[slug] | desktop | en | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r01052 | storefront | /collections/[slug] | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/collections/sweep-home -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r01053 | storefront | /collections/[slug] | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r01054 | storefront | /collections/[slug] | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r01055 | storefront | /collections/[slug] | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/collections/sweep-home -> /en/collections/sweep-home?min=&max=&sort=newest | PASS |
| r01056 | storefront | /collections/[slug] | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01057 | storefront | /collections/[slug] | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/collections/sweep-home?min=&max=&sort=price_asc -> /en/products/t04-64977906744e; DOM changed (5 mutations) | PASS |
| r01058 | storefront | /collections/[slug] | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/collections/sweep-home -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01059 | storefront | /products/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01060 | storefront | /products/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en; DOM changed (4 mutations) | PASS |
| r01061 | storefront | /products/[slug] | desktop | en | main | All products | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/products; DOM changed (4 mutations) | PASS |
| r01062 | storefront | /products/[slug] | desktop | en | main | Enlarge photo | click | visible change | layer opened (Sweep Wool Scarf — Enlarge photo); DOM changed (2 mutations) | PASS |
| r01063 | storefront | /products/[slug] | desktop | en | main | Increase quantity | click | visible change | DOM changed (2 mutations) | PASS |
| r01064 | storefront | /products/[slug] | desktop | en | main | Add to cart (`add-to-cart`) | click | visible change | DOM changed (3 mutations) | PASS |
| r01065 | storefront | /products/[slug] | desktop | en | main | Buy now (`buy-now`) | click | visible change | DOM changed (3 mutations) | PASS |
| r01066 | storefront | /products/[slug] | desktop | en | main | Sweep home | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r01067 | storefront | /products/[slug] | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r01068 | storefront | /products/[slug] | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r01069 | storefront | /search | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01070 | storefront | /search | desktop | en | main | Home | click | navigation or in-page change | url /en/search?q=Sweep -> /en; DOM changed (4 mutations) | PASS |
| r01071 | storefront | /search | desktop | en | main | Search | click | visible change | the control's own state changed ( -> gone) | PASS |
| r01072 | storefront | /search | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r01073 | storefront | /search | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r01074 | storefront | /search | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/search?q=Sweep -> /en/search?q=Sweep&min=&max=&sort=newest | PASS |
| r01075 | storefront | /search | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r01076 | storefront | /search | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /en/search?q=Sweep&min=&max=&sort=price_asc -> /en/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r01077 | storefront | /search | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/search?q=Sweep -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r01078 | storefront | /search | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/search?q=Sweep -> /en/products/t04-64977906744e; DOM changed (5 mutations) | PASS |
| r01079 | storefront | /orders/lookup | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01080 | storefront | /orders/lookup | desktop | en | main | Find order (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r01081 | storefront | /order-link | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01082 | storefront | /order-link | desktop | en | main | Look up an order | click | navigation or in-page change | url /en/order-link -> /en/orders/lookup; DOM changed (12 mutations) | PASS |
| r01083 | storefront | /claim | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01084 | storefront | /claim | desktop | en | main | Language | selectOption(zh-CN) | visible change | url /en/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r01085 | storefront | /legal/anti-fraud | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01086 | storefront | /legal/anti-fraud | desktop | en | main | Home | click | navigation or in-page change | url /en/legal/anti-fraud -> /en; DOM changed (4 mutations) | PASS |
| r01087 | storefront | /legal/anti-fraud | desktop | en | main | Taiwan National Police Agency — 165 anti-fraud service | click | navigation or in-page change | url /en/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r01088 | storefront | /legal/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01089 | storefront | /legal/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/legal/refunds -> /en; DOM changed (4 mutations) | PASS |
| r01090 | storefront | /legal/[slug] | desktop | en | main | Back to the shop | click | navigation or in-page change | url /en/legal/refunds -> /en; DOM changed (4 mutations) | PASS |
| r01091 | storefront | /privacy | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01092 | storefront | /privacy | desktop | en | main | Language | selectOption(zh-CN) | visible change | url /en/privacy -> /zh-CN/privacy; DOM changed (31 mutations) | PASS |
| r01093 | storefront | /privacy | desktop | en | main | Turn on: The store may send me marketing messages on Messenger or Instagram DM. (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01094 | storefront | /privacy | desktop | en | main | Turn on: Use my purchase to personalize Meta ads for me. (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r01095 | storefront | /privacy | desktop | en | main | Download my data (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r01096 | storefront | /privacy | desktop | en | main | Erase my data… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations) (confirmation shown, then cancelled) | PASS |
| r01097 | storefront | /data-deletion | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01098 | storefront | /data-deletion | desktop | en | main | Language | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r01099 | storefront | /data-deletion | desktop | en | main | Open the privacy page (`data-deletion-privacy-link`) | click | navigation or in-page change | url /en/data-deletion -> /en/privacy; DOM changed (2 mutations) | PASS |
| r01100 | storefront | /pages/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r01101 | storefront | /pages/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/pages/about -> /en; DOM changed (4 mutations) | PASS |
| r01102 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | open the product list and click New product | real clicks | the create form opens | as expected | PASS |
| r01103 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | fill the name and description | real clicks | the draft fields retain the entered values | as expected | PASS |
| r01104 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | add Size = S, M | real clicks | the editor proposes two SKU rows | as expected | PASS |
| r01105 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | type both SKU prices | real clicks | the matrix retains both prices | as expected | PASS |
| r01106 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | set opening quantities 9 and 7, then save the document | real clicks | both SKU quantities persist | as expected | PASS |
| r01107 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | upload a cover, set Active and save | real clicks | the product is active and persists after a reload | as expected | PASS |
| r01108 | journey | J1 product editor -> storefront | desktop | zh-TW | journey | the merchant list shows the product (search + click) | real clicks | the row is listed with status active | as expected | PASS |
| r01109 | journey | J1 storefront | desktop | zh-TW | journey | the buyer opens All products and clicks the new product | real clicks | its page opens with the title and an enabled Add to cart | as expected | PASS |
| r01110 | journey | J2/J3 admin orders | desktop | zh-TW | journey | admin Orders: the storefront COD order is listed (click through pages) | real clicks | the row of the order id can be expanded | as expected | PASS |
| r01111 | journey | J2/J3 admin orders | desktop | zh-TW | journey | record the manual shipment (carrier Black Cat + tracking) and click Record | real clicks | the shipment record is shown | as expected | PASS |
| r01112 | journey | J2/J3 admin orders | desktop | zh-TW | journey | click Collected, confirm in the dialog | real clicks | the order shows COLLECTED and the button is gone | as expected | PASS |
| r01113 | journey | J2/J3 admin orders | desktop | zh-TW | journey | reload the order list: collected persists | real clicks | COLLECTED is still shown after a reload | as expected | PASS |
| r01114 | journey | J2 storefront order lookup | desktop | zh-TW | journey | a fresh buyer browser looks the order up (order id + phone) and sees it collected | real clicks | the lookup shows the order with the collected amount | as expected | PASS |
| r01115 | journey | J4 custom domain | desktop | zh-TW | journey | open Settings and find the custom domain card | real clicks | the storefront domains card is shown | as expected | PASS |
| r01116 | journey | J4 custom domain | desktop | zh-TW | journey | type a custom hostname and click Request | real clicks | DNS instructions appear: a TXT name, a TXT value and the CNAME target | as expected | PASS |
| r01117 | journey | J4 custom domain | desktop | zh-TW | journey | reload Settings: the requested domain is listed | real clicks | the domain row persists in state REQUESTED | as expected | PASS |
| r01118 | journey | J5 sign out | desktop | zh-TW | journey | open the dashboard, click Sign out | real clicks | the session ends: the page asks to sign in again | as expected | PASS |
| r01119 | journey | J5 sign out | desktop | zh-TW | journey | reload after sign out | real clicks | the dashboard is not shown without a session | as expected | PASS |
| r01120 | storefront | /live | - | - | page | (not implemented) | - | ui-architecture section 4 lists /live (S6) | apps/storefront/app/[locale] has no live route in this base; nothing to click | SKIP |
