# PARITY-MONEY-FIX2 (run 41106372580, sha f6cf3036)

| Job | Failing test | Evidence | Class |
|---|---|---|---|
| gate (bundle:02) 124239119491 | tests/admin/reports.spec.ts:341 `RPUI RVFIX-370 reader recovers from funnel refusal zh-TW/1440` | playwright.log: `reports-tab-products` resolved but hidden; Reports.tsx hid tabs and "choose dates" whenever a read failed | b (product regression) |
| gate (bundle:06) 124239118340 | tests/admin/home-cod.spec.ts:90 `PARITY-MONEY COD settings and save-review frames (zh-TW, 1440)` | 240 s timeout; snapshot shows signed settings overview (#354, merged from base) with only "編輯設定"; old "進階" entry no longer exists | a (test rewritten to the signed overview entry, same coverage) |
| receipts 124264043942 / required 124272647132 | matrix-receipts RED (2 problems) | the two bundle jobs above | follow-on |
