# G-UI8 click ledger

Generated 2026-10-30T13:30:30.429Z. shard 7/10: 27 page/viewport/locale units opened (0 with a page-load failure), 173 control clicks (169 pass, 1 fail, 3 skip), 0 journey steps (0 pass, 0 fail); failures: known 0, new 1; stale known-defect entries 0.

Every row is one real Playwright interaction (click / selectOption). Destructive and irreversible controls stop at their confirmation and are cancelled; `skip` rows name why.

| # | App | Page | Viewport | Locale | Scope | Control | Action | Expected | Actual | Result |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| r00001 | admin | / | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 (keeps updating) | PASS |
| r00002 | admin | / | desktop | zh-TW | skip | 跳至內容 | keyboard: Tab to the link, Enter | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e#main; scrolled | PASS |
| r00003 | admin | / | desktop | zh-TW | topbar | 語言 (`locale-switch`) | selectOption(zh-CN) | visible change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-CN?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (18 mutations) | PASS |
| r00004 | admin | / | desktop | zh-TW | topbar | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00005 | admin | / | desktop | zh-TW | layer of 說明 | 說明 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00006 | admin | / | desktop | zh-TW | topbar | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00007 | admin | / | desktop | zh-TW | layer of 帳號 | 帳號 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00008 | admin | / | desktop | zh-TW | layer of 帳號 | 登出 (`workspace-sign-out`) | click | sign out is exercised once, last, in a throwaway context (journey J5) | not clicked here: it would revoke the sweep's own session | SKIP |
| r00009 | admin | / | desktop | zh-TW | rail | 總覽 (`nav-group-overview`) | click | visible change | DOM changed (10 mutations) | PASS |
| r00010 | admin | / | desktop | zh-TW | rail | 直播 + (`nav-group-live`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00011 | admin | / | desktop | zh-TW | rail | 訂單與出貨 (`nav-orders`) | click | visible change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (20 mutations) | PASS |
| r00012 | admin | / | desktop | zh-TW | rail | 商品與庫存 + (`nav-group-catalog`) | click | visible change | DOM changed (5 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00013 | admin | / | desktop | zh-TW | rail | 訊息中心 (`nav-group-messages`) | click | visible change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/messages?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (12 mutations) | PASS |
| r00014 | admin | / | desktop | zh-TW | rail | 顧客 (`nav-group-customers`) | click | visible change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/customers?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (17 mutations) | PASS |
| r00015 | admin | / | desktop | zh-TW | rail | 行銷優惠 + (`nav-group-marketing`) | click | visible change | DOM changed (4 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00016 | admin | / | desktop | zh-TW | rail | 網路商店 (`nav-group-storefront`) | click | visible change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/design?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (12 mutations) | PASS |
| r00017 | admin | / | desktop | zh-TW | rail | 收款與報表 (`nav-group-finance`) | click | visible change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/finance?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (15 mutations) | PASS |
| r00018 | admin | / | desktop | zh-TW | rail | 設定 + (`nav-group-settings`) | click | visible change | DOM changed (6 mutations); the control's own state changed (\|false -> \|true) | PASS |
| r00019 | admin | / | desktop | zh-TW | main | 建立訂單 (`action-create-order`) | click | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders/new?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (10 mutations) | PASS |
| r00020 | admin | / | desktop | zh-TW | main | 匯入或匯出商品 (`action-import`) | click | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/products/import?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (10 mutations) | PASS |
| r00021 | admin | / | desktop | zh-TW | main | 庫存 (`action-inventory`) | click | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/inventory?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (15 mutations) | PASS |
| r00022 | admin | / | desktop | zh-TW | main | 待確認的轉帳 1 (`todo-transfer`) | click | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders?state=AWAITING_TRANSFER&store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (20 mutat | PASS |
| r00023 | admin | / | desktop | zh-TW | main | 待出貨訂單 2 (`todo-ship`) | click | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders?state=unshipped&store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (20 mutations) | PASS |
| r00024 | admin | / | desktop | zh-TW | main | 待建立的超商托運單 1 (`todo-label`) | click | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders?state=unshipped&store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (20 mutations) | PASS |
| r00025 | admin | / | desktop | zh-TW | main | 低庫存規格（5 件以下） 0 (`todo-stock`) | click | navigation or in-page change | no visible change within 3 s | FAIL no-effect |
| r00026 | admin | / | desktop | zh-TW | main | 處理中的退款 0 (`todo-refunds`) | click | confirmation layer opens; cancel; nothing changed | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (20 mutations) (inline change only; no request fired) | PASS |
| r00027 | admin | / | desktop | zh-TW | main | 93457f84 | click | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders?order=93457f84-7a25-4d9a-9d9c-b9eb1c46a5a2&store=a9a134dd-3867-4dc4-81e5-471436af893e; DO | PASS |
| r00028 | admin | / | desktop | zh-TW | main | 6bf9c3d1 | click | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders?order=6bf9c3d1-c7fd-4803-804a-13e71d8d11fa&store=a9a134dd-3867-4dc4-81e5-471436af893e; DO | PASS |
| r00029 | admin | / | desktop | zh-TW | main | b9cb69ee | click | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders?order=b9cb69ee-7678-40c4-ac0d-c4e30700694c&store=a9a134dd-3867-4dc4-81e5-471436af893e; DO | PASS |
| r00030 | admin | / | desktop | zh-TW | main | 334490e6 | click | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders?order=334490e6-0bf5-4a87-be87-a12e36c1b374&store=a9a134dd-3867-4dc4-81e5-471436af893e; DO | PASS |
| r00031 | admin | / | desktop | zh-TW | main | 80a3f38e | click | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders?order=80a3f38e-c2cc-49dc-8a3c-a3a0887b7a0a&store=a9a134dd-3867-4dc4-81e5-471436af893e; DO | PASS |
| r00032 | admin | / | desktop | zh-TW | main | 6072d757 | click | navigation or in-page change | url /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders?order=6072d757-f42f-43b6-8d23-e0020aa0ce3c&store=a9a134dd-3867-4dc4-81e5-471436af893e; DO | PASS |
| r00033 | admin | / | desktop | zh-TW | main | 全部訂單 | click | navigation or in-page change | scrolled | PASS |
| r00034 | admin | /orders/new | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 (keeps updating) | PASS |
| r00035 | admin | /orders/new | desktop | en | main | Overview | click | navigation or in-page change | url /en/orders/new?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /en?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (14 mutations) | PASS |
| r00036 | admin | /orders/new | desktop | en | main | Search (`mo-search-button`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00037 | admin | /orders/new | desktop | en | main | Delivery method (`mo-option`) | selectOption(a1fd7733-00d0-454c-bc94-3906b3ab6e91\|TW\|cvs2d7da6c0ca55) | visible change | DOM changed (4 mutations); the control's own state changed ( -> a1fd7733-00d0-454c-bc94-3906b3ab6e91\|TW\|cvs2d7da6c0ca55) | PASS |
| r00038 | admin | /orders/new | desktop | en | main | Customer link | selectOption(zh-CN) | visible change | the control's own state changed (zh-TW -> zh-CN); scrolled | PASS |
| r00039 | admin | /orders/new | desktop | en | main | Back to orders | click | navigation or in-page change | scrolled | PASS |
| r00040 | admin | /customers/[customer] | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 (keeps updating) | PASS |
| r00041 | admin | /customers/[customer] | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/customers/7b265b4a-c4e8-4915-96b1-0d964409999c?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM c | PASS |
| r00042 | admin | /customers/[customer] | mobile | zh-TW | main | 返回搜尋結果 | click | navigation or in-page change | url /zh-TW/customers/7b265b4a-c4e8-4915-96b1-0d964409999c?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/customers?store=a9a134dd-3867-4dc4-81e5-471436af8 | PASS |
| r00043 | admin | /customers/[customer] | mobile | zh-TW | main | 訂單 | click | navigation or in-page change | url /zh-TW/customers/7b265b4a-c4e8-4915-96b1-0d964409999c?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/customers/7b265b4a-c4e8-4915-96b1-0d964409999c?st | PASS |
| r00044 | admin | /customers/[customer] | mobile | zh-TW | main | 同意 | click | navigation or in-page change | url /zh-TW/customers/7b265b4a-c4e8-4915-96b1-0d964409999c?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/customers/7b265b4a-c4e8-4915-96b1-0d964409999c?st | PASS |
| r00045 | admin | /customers/[customer] | mobile | zh-TW | main | 隱私 | click | navigation or in-page change | url /zh-TW/customers/7b265b4a-c4e8-4915-96b1-0d964409999c?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/customers/7b265b4a-c4e8-4915-96b1-0d964409999c?st | PASS |
| r00046 | admin | /customers/[customer] | mobile | zh-TW | main | 進階 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true); scrolled | PASS |
| r00047 | admin | /customers/[customer] | mobile | zh-TW | layer of 進階 | 進階 | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00048 | admin | /customers/[customer] | mobile | zh-TW | layer of 進階 | 新增標籤 | click | visible change | DOM changed (2 mutations) | PASS |
| r00049 | admin | /customers/[customer] | mobile | zh-TW | main | 下載顧客資料 (`customer-download`) | click | visible change | layer opened (DIALOG); DOM changed (2 mutations); scrolled | PASS |
| r00050 | admin | /customers/[customer] | mobile | zh-TW | layer of 下載顧客資料 | 確認下載 | - | the commit button of a confirmation dialog is never pressed by the sweep | not pressed (the dialog's cancel path is exercised by the restore) | SKIP |
| r00051 | admin | /customers/[customer] | mobile | zh-TW | main | 抹除顧客 (`customer-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations); scrolled (confirmation shown, then cancelled) | PASS |
| r00052 | admin | /customers/[customer] | mobile | zh-TW | main | 在訂單中開啟: 334490e6-0bf5-4a87-be87-a12e36c1b374 | click | navigation or in-page change | url /zh-TW/customers/7b265b4a-c4e8-4915-96b1-0d964409999c?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/orders?store=a9a134dd-3867-4dc4-81e5-471436af893e | PASS |
| r00053 | admin | /ads | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 (keeps updating) | PASS |
| r00054 | admin | /ads | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/ads?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (15 mutations) | PASS |
| r00055 | admin | /ads | mobile | zh-TW | main | 廣告歸因與直播復盤 (`ads-attribution-link`) | click | navigation or in-page change | url /zh-TW/ads?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW/ads/attribution?store=a9a134dd-3867-4dc4-81e5-471436af893e | PASS |
| r00056 | admin | /ads | mobile | zh-TW | main | 重新整理 (`ads-refresh`) | click | visible change | DOM changed (7 mutations) | PASS |
| r00057 | admin | /ads | mobile | zh-TW | main | 連結 Meta 廣告帳號 (`ads-connect`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00058 | admin | /ads | mobile | zh-TW | main | 複製 feed 網址 (`ads-feed-copy`) | click | visible change | scrolled | PASS |
| r00059 | admin | /ads | mobile | zh-TW | main | 新增草稿 (`ads-new-draft`) | click | visible change | DOM changed (2 mutations); scrolled | PASS |
| r00060 | admin | /ads | mobile | zh-TW | main | 顯示成效 (`ads-report-load`) | click | visible change | DOM changed (4 mutations); scrolled | PASS |
| r00061 | admin | /ads | mobile | zh-TW | main | 傳送購買事件 (`ads-capi-enabled`) | click (toggle) | visible change | the control's own state changed (false -> true); scrolled | PASS |
| r00062 | admin | /ads | mobile | zh-TW | main | 資料集 (`ads-capi-dataset`) | selectOption | visible change | select has a single option | SKIP |
| r00063 | admin | /ads | mobile | zh-TW | main | 儲存 (`ads-capi-save`) | click | visible change | DOM changed (5 mutations); scrolled | PASS |
| r00064 | admin | /design | mobile | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 (keeps updating) | PASS |
| r00065 | admin | /design | mobile | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/design?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (15 mutations) | PASS |
| r00066 | admin | /design | mobile | zh-TW | main | 預覽 (`design-preview`) | click | visible change | popup: about:blank; DOM changed (5 mutations) | PASS |
| r00067 | admin | /design | mobile | zh-TW | main | 資訊頁 (`design-task-pages`) | click | visible change | DOM changed (7 mutations) | PASS |
| r00068 | admin | /design | mobile | zh-TW | main | 首頁區塊 (`design-task-home`) | click | visible change | DOM changed (7 mutations); the control's own state changed ( -> \|false) | PASS |
| r00069 | admin | /design | mobile | zh-TW | main | 導覽選單 (`design-task-nav`) | click | visible change | DOM changed (7 mutations); the control's own state changed ( -> \|false) | PASS |
| r00070 | admin | /design | mobile | zh-TW | main | 店鋪資料 (`design-task-profile`) | click | visible change | DOM changed (7 mutations); the control's own state changed ( -> \|false) | PASS |
| r00071 | admin | /design | mobile | zh-TW | main | 網店網址 (`design-domains-open`) | click | visible change | DOM changed (4 mutations); the control's own state changed ( -> gone) | PASS |
| r00072 | admin | /design | mobile | zh-TW | main | 發布記錄 (`design-tab-versions`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (5 mutations); the control's own state changed ( -> gone) (inline change only; no request fired) | PASS |
| r00073 | admin | /team | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 (keeps updating) | PASS |
| r00074 | admin | /team | desktop | en | main | Overview | click | navigation or in-page change | url /en/team?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /en?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (14 mutations) | PASS |
| r00075 | admin | /team | desktop | en | main | Role (`team-invite-role`) | selectOption(owner) | visible change | DOM changed (1 mutations); the control's own state changed (viewer -> owner) | PASS |
| r00076 | admin | /team | desktop | en | main | Send invitation (`team-invite-send`) | click | visible change | validation shown | PASS |
| r00077 | admin | /team | desktop | en | main | Change role (`member-role-1f99efcf-01fb-4bb6-a348-b9e045f8b599`) | selectOption(admin) | visible change | DOM changed (4 mutations) | PASS |
| r00078 | admin | /team | desktop | en | main | Remove (`member-remove-1f99efcf-01fb-4bb6-a348-b9e045f8b599`) | click | confirmation layer opens; cancel; nothing changed | DOM changed (5 mutations) (inline change only; no request fired) | PASS |
| r00079 | admin | /billing | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 (keeps updating) | PASS |
| r00080 | admin | /billing | desktop | en | main | Overview | click | navigation or in-page change | url /en/billing?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /en?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (14 mutations) | PASS |
| r00081 | admin | /billing | desktop | en | main | Subscribe (`plan-choose-price_ClickSweepMonth`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00082 | admin | /settings/settlements | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 (keeps updating) | PASS |
| r00083 | admin | /settings/settlements | desktop | zh-TW | main | 總覽 | click | navigation or in-page change | url /zh-TW/settings/settlements?store=a9a134dd-3867-4dc4-81e5-471436af893e -> /zh-TW?store=a9a134dd-3867-4dc4-81e5-471436af893e; DOM changed (14 mutations) | PASS |
| r00084 | admin | /invite/[token] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00085 | admin | /invite/[token] | desktop | en | main | Sign in (`invite-signin`) | click | navigation or in-page change | url /en/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /en#next=%2Fen%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (34 mutations) | PASS |
| r00086 | admin | /invite/[token] | desktop | en | main | Create an account (`invite-signup`) | click | navigation or in-page change | url /en/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA -> /en/signup#next=%2Fen%2Finvite%2FAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; DOM changed (23 muta | PASS |
| r00087 | admin | /reset | desktop | zh-TW | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00088 | admin | /reset | desktop | zh-TW | main | 語言 | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00089 | admin | /reset | desktop | zh-TW | main | 寄送驗證碼 | click | visible change | validation shown | PASS |
| r00090 | admin | /reset | desktop | zh-TW | main | 返回登入 | click | navigation or in-page change | url /zh-TW/reset -> /zh-TW; DOM changed (14 mutations) | PASS |
| r00091 | storefront | /cart | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 (keeps updating) | PASS |
| r00092 | storefront | /cart | desktop | en | main | Sweep Wool Scarf | click | navigation or in-page change | url /en/cart -> /en/products/sweep-wool-scarf; DOM changed (4 mutations) | PASS |
| r00093 | storefront | /cart | desktop | en | main | Increase quantity | click | visible change | DOM changed (3 mutations) | PASS |
| r00094 | storefront | /cart | desktop | en | main | Checkout (`cart-checkout`) | click | navigation or in-page change | url /en/cart -> /en/checkout; DOM changed (11 mutations) | PASS |
| r00095 | storefront | /cart | desktop | en | main | Continue shopping | click | navigation or in-page change | url /en/cart -> /en/products; DOM changed (4 mutations) | PASS |
| r00096 | storefront | /cart | desktop | en | main | Remove Sweep Wool Scarf from the cart | click | visible change | DOM changed (4 mutations) | PASS |
| r00097 | storefront | /checkout | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00098 | storefront | /checkout | desktop | en | main | Your orders (`toggle-order-history`) | click | visible change | DOM changed (4 mutations) | PASS |
| r00099 | storefront | /checkout | desktop | en | main | Sweep Wool Scarf | click | navigation or in-page change | url /en/checkout -> /en/products/sweep-wool-scarf; DOM changed (7 mutations) | PASS |
| r00100 | storefront | /checkout | desktop | en | main | Back to cart | click | navigation or in-page change | url /en/checkout -> /en/cart; DOM changed (9 mutations) | PASS |
| r00101 | storefront | /checkout | desktop | en | main | Choose delivery | click | visible change | DOM changed (3 mutations) | PASS |
| r00102 | storefront | /orders/[orderID] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00103 | storefront | /orders/[orderID] | desktop | en | main | Refresh order (`refresh-order`) | click | visible change | DOM changed (2 mutations); the control's own state changed ( -> gone) | PASS |
| r00104 | storefront | / | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00105 | storefront | / | desktop | en | chrome | Skip to content | keyboard: Tab to the link, Enter | navigation or in-page change | url /en -> /en#main; scrolled | PASS |
| r00106 | storefront | / | desktop | en | chrome | Sweep Goods | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00107 | storefront | / | desktop | en | chrome | All products | click | navigation or in-page change | url /en -> /en/products; DOM changed (4 mutations) | PASS |
| r00108 | storefront | / | desktop | en | chrome | Sweep home | click | navigation or in-page change | url /en -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00109 | storefront | / | desktop | en | chrome | About us | click | navigation or in-page change | url /en -> /en/pages/about; DOM changed (4 mutations) | PASS |
| r00110 | storefront | / | desktop | en | chrome | Search | click | visible change | url /en -> /en/search?q= | PASS |
| r00111 | storefront | / | desktop | en | chrome | Cart, 1 items (`header-cart`) | click | visible change | DOM changed (2 mutations) | PASS |
| r00112 | storefront | / | desktop | en | chrome | +886 2 2345 6789 | click | navigation or in-page change | scrolled | PASS |
| r00113 | storefront | / | desktop | en | chrome | hello@sweep.example | click | navigation or in-page change | scrolled | PASS |
| r00114 | storefront | / | desktop | en | chrome | LINE | click | visible change | external: GET https://line.me/R/ti/p/@sweep; scrolled | PASS |
| r00115 | storefront | / | desktop | en | chrome | Facebook | click | visible change | external: GET https://www.facebook.com/sweep.example; scrolled | PASS |
| r00116 | storefront | / | desktop | en | chrome | Instagram | click | visible change | external: GET https://www.instagram.com/sweep.example; scrolled | PASS |
| r00117 | storefront | / | desktop | en | chrome | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00118 | storefront | / | desktop | en | chrome | Shop safely | click | navigation or in-page change | url /en -> /en/legal/anti-fraud | PASS |
| r00119 | storefront | / | desktop | en | chrome | Data deletion | click | navigation or in-page change | url /en -> /en/data-deletion | PASS |
| r00120 | storefront | / | desktop | en | chrome | 简体中文 | click | navigation or in-page change | url /en -> /zh-CN | PASS |
| r00121 | storefront | / | desktop | en | chrome | 繁體中文 | click | navigation or in-page change | url /en -> /zh-TW | PASS |
| r00122 | storefront | / | desktop | en | chrome | English | click | navigation or in-page change | no change (the current item) (already the current item: no change expected) | PASS |
| r00123 | storefront | / | desktop | en | main | View all | click | navigation or in-page change | url /en -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00124 | storefront | / | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en -> /en/products/t04-175a61e5fb33; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00125 | storefront | / | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) [1 of 2 alike] | PASS |
| r00126 | storefront | / | desktop | en | main | View all | click | navigation or in-page change | url /en -> /en/products; DOM changed (4 mutations) | PASS |
| r00127 | storefront | / | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | scrolled | PASS |
| r00128 | storefront | / | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00129 | storefront | / | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled [1 of 2 alike] | PASS |
| r00130 | storefront | /products | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00131 | storefront | /products | desktop | en | main | Home | click | navigation or in-page change | url /en/products -> /en; DOM changed (4 mutations) | PASS |
| r00132 | storefront | /products | desktop | en | main | All products | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00133 | storefront | /products | desktop | en | main | Sweep home | click | navigation or in-page change | url /en/products -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00134 | storefront | /products | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/products -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00135 | storefront | /products | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00136 | storefront | /products | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00137 | storefront | /products | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/products -> /en/products?min=&max=&sort=newest | PASS |
| r00138 | storefront | /products | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00139 | storefront | /products | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /en/products -> /en/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r00140 | storefront | /products | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/products -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00141 | storefront | /products | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/products -> /en/products/t04-175a61e5fb33; DOM changed (5 mutations) | PASS |
| r00142 | storefront | /collections | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00143 | storefront | /collections | desktop | en | main | Home | click | navigation or in-page change | url /en/collections -> /en; DOM changed (4 mutations) | PASS |
| r00144 | storefront | /collections | desktop | en | main | S Sweep home 2 products (`collection-tile`) | click | navigation or in-page change | url /en/collections -> /en/collections/sweep-home; DOM changed (4 mutations) | PASS |
| r00145 | storefront | /collections | desktop | en | main | S Sweep wear 2 products (`collection-tile`) | click | navigation or in-page change | url /en/collections -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00146 | storefront | /collections/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00147 | storefront | /collections/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/collections/sweep-home -> /en; DOM changed (4 mutations) | PASS |
| r00148 | storefront | /collections/[slug] | desktop | en | main | Collections | click | navigation or in-page change | url /en/collections/sweep-home -> /en/collections; DOM changed (6 mutations) | PASS |
| r00149 | storefront | /collections/[slug] | desktop | en | main | All products | click | navigation or in-page change | url /en/collections/sweep-home -> /en/products; DOM changed (6 mutations) | PASS |
| r00150 | storefront | /collections/[slug] | desktop | en | main | Sweep home | click | navigation or in-page change | DOM changed (2 mutations) | PASS |
| r00151 | storefront | /collections/[slug] | desktop | en | main | Sweep wear | click | navigation or in-page change | url /en/collections/sweep-home -> /en/collections/sweep-wear; DOM changed (4 mutations) | PASS |
| r00152 | storefront | /collections/[slug] | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00153 | storefront | /collections/[slug] | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00154 | storefront | /collections/[slug] | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/collections/sweep-home -> /en/collections/sweep-home?min=&max=&sort=newest | PASS |
| r00155 | storefront | /collections/[slug] | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00156 | storefront | /collections/[slug] | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/collections/sweep-home -> /en/products/t04-175a61e5fb33; DOM changed (5 mutations) | PASS |
| r00157 | storefront | /collections/[slug] | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/collections/sweep-home -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00158 | storefront | /products/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00159 | storefront | /products/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en; DOM changed (4 mutations) | PASS |
| r00160 | storefront | /products/[slug] | desktop | en | main | All products | click | navigation or in-page change | url /en/products/sweep-wool-scarf -> /en/products; DOM changed (4 mutations) | PASS |
| r00161 | storefront | /products/[slug] | desktop | en | main | Enlarge photo | click | visible change | layer opened (Sweep Wool Scarf — Enlarge photo); DOM changed (2 mutations) | PASS |
| r00162 | storefront | /products/[slug] | desktop | en | main | Increase quantity | click | visible change | DOM changed (2 mutations) | PASS |
| r00163 | storefront | /products/[slug] | desktop | en | main | Add to cart (`add-to-cart`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00164 | storefront | /products/[slug] | desktop | en | main | Buy now (`buy-now`) | click | visible change | DOM changed (3 mutations) | PASS |
| r00165 | storefront | /products/[slug] | desktop | en | main | Sweep home | click | navigation or in-page change | scrolled | PASS |
| r00166 | storefront | /products/[slug] | desktop | en | main | Sweep wear | click | navigation or in-page change | scrolled | PASS |
| r00167 | storefront | /products/[slug] | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | scrolled | PASS |
| r00168 | storefront | /search | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00169 | storefront | /search | desktop | en | main | Home | click | navigation or in-page change | url /en/search?q=Sweep -> /en; DOM changed (4 mutations) | PASS |
| r00170 | storefront | /search | desktop | en | main | Search | click | visible change | the control's own state changed ( -> gone) | PASS |
| r00171 | storefront | /search | desktop | en | main | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|false -> \|\|\|\|\|\|true) | PASS |
| r00172 | storefront | /search | desktop | en | layer of Price | Price | click | visible change | DOM changed (1 mutations); the control's own state changed (\|\|\|\|\|\|true -> \|\|\|\|\|\|false) | PASS |
| r00173 | storefront | /search | desktop | en | layer of Price | Apply | click | commit button of an open layer: its request fires (blocked by the sweep) or validation shows | url /en/search?q=Sweep -> /en/search?q=Sweep&min=&max=&sort=newest | PASS |
| r00174 | storefront | /search | desktop | en | main | Sort by | selectOption(price_asc) | visible change | the control's own state changed (newest -> price_asc) | PASS |
| r00175 | storefront | /search | desktop | en | main | Sweep Ceramic Mug NT$128 | click | navigation or in-page change | url /en/search?q=Sweep -> /en/products/sweep-ceramic-mug; DOM changed (5 mutations) | PASS |
| r00176 | storefront | /search | desktop | en | main | Sweep Wool Scarf NT$98 | click | navigation or in-page change | url /en/search?q=Sweep -> /en/products/sweep-wool-scarf; DOM changed (5 mutations) | PASS |
| r00177 | storefront | /search | desktop | en | main | No image Sweep Cedar Candle NT$12.50 | click | navigation or in-page change | url /en/search?q=Sweep -> /en/products/t04-175a61e5fb33; DOM changed (5 mutations) | PASS |
| r00178 | storefront | /orders/lookup | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00179 | storefront | /orders/lookup | desktop | en | main | Find order (`lookup-submit`) | click | visible change | DOM changed (1 mutations) | PASS |
| r00180 | storefront | /order-link | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00181 | storefront | /order-link | desktop | en | main | Look up an order | click | navigation or in-page change | url /en/order-link -> /en/orders/lookup; DOM changed (12 mutations) | PASS |
| r00182 | storefront | /claim | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00183 | storefront | /claim | desktop | en | main | Language | selectOption(zh-CN) | visible change | url /en/claim -> /zh-CN/claim; DOM changed (5 mutations) | PASS |
| r00184 | storefront | /legal/anti-fraud | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00185 | storefront | /legal/anti-fraud | desktop | en | main | Home | click | navigation or in-page change | url /en/legal/anti-fraud -> /en; DOM changed (4 mutations) | PASS |
| r00186 | storefront | /legal/anti-fraud | desktop | en | main | Taiwan National Police Agency — 165 anti-fraud service | click | navigation or in-page change | url /en/legal/anti-fraud -> /; external: GET https://165.npa.gov.tw/ | PASS |
| r00187 | storefront | /legal/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00188 | storefront | /legal/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/legal/refunds -> /en; DOM changed (4 mutations) | PASS |
| r00189 | storefront | /legal/[slug] | desktop | en | main | Back to the shop | click | navigation or in-page change | url /en/legal/refunds -> /en; DOM changed (4 mutations) | PASS |
| r00190 | storefront | /privacy | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00191 | storefront | /privacy | desktop | en | main | Language | selectOption(zh-CN) | visible change | url /en/privacy -> /zh-CN/privacy; DOM changed (31 mutations) | PASS |
| r00192 | storefront | /privacy | desktop | en | main | Turn on: The store may send me marketing messages on Messenger or Instagram DM. (`privacy-toggle-marketing_messages`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00193 | storefront | /privacy | desktop | en | main | Turn on: Use my purchase to personalize Meta ads for me. (`privacy-toggle-ads_personalization`) | click | visible change | DOM changed (5 mutations) | PASS |
| r00194 | storefront | /privacy | desktop | en | main | Download my data (`privacy-download`) | click | visible change | DOM changed (6 mutations) | PASS |
| r00195 | storefront | /privacy | desktop | en | main | Erase my data… (`privacy-erase`) | click | confirmation layer opens; cancel; nothing changed | layer opened (DIALOG); DOM changed (2 mutations) (confirmation shown, then cancelled) | PASS |
| r00196 | storefront | /data-deletion | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00197 | storefront | /data-deletion | desktop | en | main | Language | selectOption(zh-CN) | visible change | page reloaded | PASS |
| r00198 | storefront | /data-deletion | desktop | en | main | Open the privacy page (`data-deletion-privacy-link`) | click | navigation or in-page change | url /en/data-deletion -> /en/privacy; DOM changed (2 mutations) | PASS |
| r00199 | storefront | /pages/[slug] | desktop | en | page | (page load) | - | the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message | HTTP 200 | PASS |
| r00200 | storefront | /pages/[slug] | desktop | en | main | Home | click | navigation or in-page change | url /en/pages/about -> /en; DOM changed (4 mutations) | PASS |
