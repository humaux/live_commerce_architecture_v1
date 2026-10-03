# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: orders-ui.spec.ts >> MOU07 v2 private search, SQL queues, filters and three-language ledger
- Location: tests/admin/orders-ui.spec.ts:686:1

# Error details

```
Error: locator.scrollIntoViewIfNeeded: Element is not attached to the DOM
Call log:
  - attempting scroll into view action
    - waiting for element to be stable

```

# Page snapshot

```yaml
- generic [active] [ref=f9e1]:
  - alert [ref=f9e2]: 訂單
  - generic [ref=f9e3]:
    - link "跳至內容" [ref=f9e4] [cursor=pointer]:
      - /url: "#main"
    - complementary [ref=f9e5]:
      - generic [ref=f9e6]:
        - generic [aria-hidden] [ref=f9e7]: P
        - generic "payment mock store" [ref=f9e8]
      - navigation "工作區導覽" [ref=f9e9]:
        - button "總覽" [ref=f9e11] [cursor=pointer]
        - button "直播與貼文" [ref=f9e16] [cursor=pointer]
        - button "訂單與出貨" [ref=f9e21] [cursor=pointer]
        - button "商品與庫存" [ref=f9e26] [cursor=pointer]:
          - generic [aria-hidden] [ref=f9e30]: +
        - button "行銷優惠" [ref=f9e32] [cursor=pointer]
        - button "網路商店" [ref=f9e37] [cursor=pointer]
        - button "收款與報表" [ref=f9e42] [cursor=pointer]
      - navigation "設定" [ref=f9e46]:
        - button "設定" [ref=f9e48] [cursor=pointer]
    - generic [ref=f9e52]:
      - banner [ref=f9e53]:
        - generic [ref=f9e54]:
          - generic [ref=f9e55]: 切換商店
          - combobox "切換商店" [ref=f9e56] [cursor=pointer]:
            - option "payment mock store" [selected]
            - option "merchant order foreign store"
        - generic [ref=f9e57]:
          - generic [ref=f9e58]: 語言
          - combobox "語言" [ref=f9e59] [cursor=pointer]:
            - option "简体中文"
            - option "繁體中文" [selected]
            - option "English"
        - group [ref=f9e60]:
          - generic "說明" [ref=f9e61] [cursor=pointer]
        - group [ref=f9e62]:
          - generic "帳號" [ref=f9e63] [cursor=pointer]
      - main [ref=f9e64]:
        - navigation "工作區導覽" [ref=f9e65]:
          - link "總覽" [ref=f9e66] [cursor=pointer]:
            - /url: /zh-TW/
          - generic [aria-hidden] [ref=f9e67]: /
          - generic [ref=f9e68]: 訂單與出貨
          - generic [aria-hidden] [ref=f9e69]: /
          - generic [ref=f9e70]: 訂單
        - generic [ref=f9e71]:
          - generic [ref=f9e72]:
            - heading "訂單" [level=1] [ref=f9e73]
            - paragraph [ref=f9e74]: 查看已儲存訂單與結帳時的固定資料。
          - generic [ref=f9e75]:
            - generic [ref=f9e76]:
              - text: 搜尋訂單
              - searchbox "搜尋訂單 訂單號、電話後 4 碼、收件人、運單號或 SKU" [ref=f9e77]
              - generic [ref=f9e78]: 訂單號、電話後 4 碼、收件人、運單號或 SKU
            - generic [ref=f9e79]:
              - generic [ref=f9e80]:
                - generic [ref=f9e81]:
                  - text: 訂單狀態
                  - combobox "訂單狀態" [ref=f9e82]:
                    - option "不含草稿" [selected]
                    - option "全部"
                    - option "草稿"
                    - option "待付款"
                    - option "待轉帳"
                    - option "待貨到收款"
                    - option "已確認"
                    - option "已取消"
                    - option "已出貨"
                    - option "待出貨"
                    - option "已建單·待交寄"
                - button "重新整理" [ref=f9e83] [cursor=pointer]
              - generic [ref=f9e86]:
                - text: 付款方式
                - combobox "付款方式" [ref=f9e87]:
                  - option "不限" [selected]
                  - option "信用卡"
                  - option "銀行轉帳"
                  - option "取貨付款"
                  - option "貨到付款"
              - generic [ref=f9e88]:
                - text: 配送方式
                - combobox "配送方式" [ref=f9e89]:
                  - option "不限" [selected]
                  - option "宅配"
                  - option "7-ELEVEN"
                  - option "全家"
                  - option "萊爾富"
                  - option "OK 超商"
              - generic [ref=f9e90]:
                - text: 直播場次
                - combobox "直播場次" [ref=f9e91]:
                  - option "不限" [selected]
                  - option "claims gate 1e5ace2e598b"
              - generic [ref=f9e92]:
                - text: 開始日期（台北）
                - textbox "開始日期（台北） 年／月／日（YYYY-MM-DD）" [ref=f9e93]
                - generic [ref=f9e94]: 年／月／日（YYYY-MM-DD）
              - generic [ref=f9e95]:
                - text: 結束日期（台北）
                - textbox "結束日期（台北） 年／月／日（YYYY-MM-DD）" [ref=f9e96]
                - generic [ref=f9e97]: 年／月／日（YYYY-MM-DD）
            - generic [ref=f9e98]:
              - button "套用篩選" [ref=f9e99] [cursor=pointer]
              - button "清除篩選" [ref=f9e100] [cursor=pointer]
          - navigation "訂單待辦（同筆訂單可出現在多個分類）" [ref=f9e101]:
            - button "全部 7" [pressed] [ref=f9e102] [cursor=pointer]:
              - text: 全部
              - generic [ref=f9e103]: "7"
            - button "待付款 3" [ref=f9e104] [cursor=pointer]:
              - text: 待付款
              - generic [ref=f9e105]: "3"
            - button "待核對轉帳 0" [ref=f9e106] [cursor=pointer]:
              - text: 待核對轉帳
              - generic [ref=f9e107]: "0"
            - button "待出貨 1" [ref=f9e108] [cursor=pointer]:
              - text: 待出貨
              - generic [ref=f9e109]: "1"
            - button "待交寄 0" [ref=f9e110] [cursor=pointer]:
              - text: 待交寄
              - generic [ref=f9e111]: "0"
            - button "已出貨 1" [ref=f9e112] [cursor=pointer]:
              - text: 已出貨
              - generic [ref=f9e113]: "1"
            - button "已完成 0" [ref=f9e114] [cursor=pointer]:
              - text: 已完成
              - generic [ref=f9e115]: "0"
            - button "已取消 2" [ref=f9e116] [cursor=pointer]:
              - text: 已取消
              - generic [ref=f9e117]: "2"
          - paragraph [ref=f9e118]: "符合訂單: 7"
          - table [ref=f9e120]:
            - rowgroup [ref=f9e121]:
              - row [ref=f9e122]:
                - columnheader "訂單" [ref=f9e123]
                - columnheader "建立時間（台北時間）" [ref=f9e124]
                - columnheader "收件人" [ref=f9e125]
                - columnheader "總額" [ref=f9e126]
                - columnheader "付款方式 · 狀態" [ref=f9e127]
                - columnheader "配送方式 · 出貨狀態" [ref=f9e128]
                - columnheader "來源" [ref=f9e129]
            - rowgroup [ref=f9e130]:
              - row [ref=f9e131]:
                - cell [ref=f9e132]:
                  - 'button "展開訂單詳情: 5b1e9abf-0307-4aae-9ae4-d62ff031c20f" [ref=f9e133] [cursor=pointer]':
                    - generic [ref=f9e136]: LC-5B1E9ABF03074AAE9AE4D62FF031C20F
                - cell "2026/10/03 13:30" [ref=f9e137]
                - cell "S***" [ref=f9e138]
                - cell "NT$25" [ref=f9e139]
                - cell "信用卡 已扣款 測試環境" [ref=f9e140]:
                  - generic [ref=f9e141]: 信用卡
                  - generic [ref=f9e142]: 已扣款
                  - generic [ref=f9e143]: 測試環境
                - cell "宅配 已發貨 已確認" [ref=f9e144]:
                  - generic [ref=f9e145]: 宅配
                  - generic [ref=f9e146]: 已發貨
                  - generic [ref=f9e147]: 已確認
                - cell "網店" [ref=f9e148]
              - row [ref=f9e149]:
                - cell [ref=f9e150]:
                  - 'button "展開訂單詳情: e50216a2-d5f2-4523-bb55-e9a87742d8d1" [ref=f9e151] [cursor=pointer]':
                    - generic [ref=f9e154]: LC-E50216A2D5F24523BB55E9A87742D8D1
                - cell "2026/10/03 13:30" [ref=f9e155]
                - cell "S***" [ref=f9e156]
                - cell "NT$25" [ref=f9e157]
                - cell "信用卡 待核對 測試環境" [ref=f9e158]:
                  - generic [ref=f9e159]: 信用卡
                  - generic [ref=f9e160]: 待核對
                  - generic [ref=f9e161]: 測試環境
                - cell "宅配 分配異常待核對 已取消" [ref=f9e162]:
                  - generic [ref=f9e163]: 宅配
                  - generic [ref=f9e164]: 分配異常待核對
                  - generic [ref=f9e165]: 已取消
                - cell "網店" [ref=f9e166]
              - row [ref=f9e167]:
                - cell [ref=f9e168]:
                  - 'button "展開訂單詳情: 179d6061-a3d9-4e32-95d0-dbc19b5489a1" [ref=f9e169] [cursor=pointer]':
                    - generic [ref=f9e172]: LC-179D6061A3D94E3295D0DBC19B5489A1
                - cell "2026/10/03 13:30" [ref=f9e173]
                - cell "S***" [ref=f9e174]
                - cell "NT$25" [ref=f9e175]
                - cell "信用卡 待核對 測試環境" [ref=f9e176]:
                  - generic [ref=f9e177]: 信用卡
                  - generic [ref=f9e178]: 待核對
                  - generic [ref=f9e179]: 測試環境
                - cell "宅配 備貨中 待付款" [ref=f9e180]:
                  - generic [ref=f9e181]: 宅配
                  - generic [ref=f9e182]: 備貨中
                  - generic [ref=f9e183]: 待付款
                - cell "網店" [ref=f9e184]
              - row [ref=f9e185]:
                - cell [ref=f9e186]:
                  - 'button "展開訂單詳情: 4a508bb4-09ab-4d46-a540-abf54daaa661" [ref=f9e187] [cursor=pointer]':
                    - generic [ref=f9e190]: LC-4A508BB409AB4D46A540ABF54DAAA661
                - cell "2026/10/03 13:30" [ref=f9e191]
                - cell "S***" [ref=f9e192]
                - cell "NT$25" [ref=f9e193]
                - cell "信用卡 已扣款 測試環境" [ref=f9e194]:
                  - generic [ref=f9e195]: 信用卡
                  - generic [ref=f9e196]: 已扣款
                  - generic [ref=f9e197]: 測試環境
                - cell "宅配 備貨中 已確認" [ref=f9e198]:
                  - generic [ref=f9e199]: 宅配
                  - generic [ref=f9e200]: 備貨中
                  - generic [ref=f9e201]: 已確認
                - cell "網店" [ref=f9e202]
              - row [ref=f9e203]:
                - cell [ref=f9e204]:
                  - 'button "展開訂單詳情: 8fbd72dc-6910-45df-bc18-417af0c8160f" [ref=f9e205] [cursor=pointer]':
                    - generic [ref=f9e208]: LC-8FBD72DC691045DFBC18417AF0C8160F
                - cell "2026/10/03 13:30" [ref=f9e209]
                - cell "S***" [ref=f9e210]
                - cell "NT$25" [ref=f9e211]
                - cell "信用卡 已授權·未扣款 測試環境" [ref=f9e212]:
                  - generic [ref=f9e213]: 信用卡
                  - generic [ref=f9e214]: 已授權·未扣款
                  - generic [ref=f9e215]: 測試環境
                - cell "宅配 備貨中 待付款" [ref=f9e216]:
                  - generic [ref=f9e217]: 宅配
                  - generic [ref=f9e218]: 備貨中
                  - generic [ref=f9e219]: 待付款
                - cell "網店" [ref=f9e220]
              - row [ref=f9e221]:
                - cell [ref=f9e222]:
                  - 'button "展開訂單詳情: ce130ae8-2b89-44c9-8133-1032f7a67077" [ref=f9e223] [cursor=pointer]':
                    - generic [ref=f9e226]: LC-CE130AE82B8944C981331032F7A67077
                - cell "2026/10/03 13:30" [ref=f9e227]
                - cell "S***" [ref=f9e228]
                - cell "NT$25" [ref=f9e229]
                - cell "信用卡 處理中 測試環境" [ref=f9e230]:
                  - generic [ref=f9e231]: 信用卡
                  - generic [ref=f9e232]: 處理中
                  - generic [ref=f9e233]: 測試環境
                - cell "宅配 備貨中 待付款" [ref=f9e234]:
                  - generic [ref=f9e235]: 宅配
                  - generic [ref=f9e236]: 備貨中
                  - generic [ref=f9e237]: 待付款
                - cell "網店" [ref=f9e238]
              - row [ref=f9e239]:
                - cell [ref=f9e240]:
                  - 'button "展開訂單詳情: b7894650-dee9-4c00-9177-e9a15d01091a" [ref=f9e241] [cursor=pointer]':
                    - generic [ref=f9e244]: LC-B7894650DEE94C009177E9A15D01091A
                - cell "2026/10/03 13:15" [ref=f9e245]
                - cell "S***" [ref=f9e246]
                - cell "NT$12.50" [ref=f9e247]
                - cell "信用卡 未付款" [ref=f9e248]:
                  - generic [ref=f9e249]: 信用卡
                  - generic [ref=f9e250]: 未付款
                - cell "宅配 已取消 已取消" [ref=f9e251]:
                  - generic [ref=f9e252]: 宅配
                  - generic [ref=f9e253]: 已取消
                  - generic [ref=f9e254]: 已取消
                - cell "網店" [ref=f9e255]
          - generic [ref=f9e256]:
            - generic [ref=f9e257]: "本頁訂單數: 7"
            - generic [ref=f9e258]:
              - button "上一頁" [disabled] [ref=f9e259]
              - button "下一頁" [disabled] [ref=f9e260]
```

# Test source

```ts
  726 |   await expect(page.getByTestId(`order-row-${ids.draft0}`)).toHaveCount(0);
  727 |   for (const bucket of ["unpaid","transfer_review","ready_to_ship","ready_to_consign","shipped","completed","cancelled","all"]) {
  728 |     await apply(`queue ${bucket}`, () => page.getByTestId(`orders-bucket-${bucket}`).click());
  729 |     await expect(page.getByTestId(`orders-bucket-${bucket}`)).toHaveAttribute("aria-pressed","true");
  730 |   }
  731 |   for (const [label, query, target] of [["order",ids.shipped,ids.shipped],["tracking","SYNTHETIC-MOU-TRACK",ids.shipped],["phone last4","0001",ids.shipped],["recipient","Synthetic Buyer",ids.shipped],["frozen SKU",frozenSKU,ids.shipped]] as const) {
  732 |     await page.getByTestId("orders-search").fill(query);
  733 |     const data = await apply(`search ${label}`, () => page.getByTestId("orders-apply").click());
  734 |     expect(data.total).toBeGreaterThan(0);
  735 |     await expect(page.getByTestId(`order-row-${target}`)).toBeVisible();
  736 |     expect(new URL(page.url()).searchParams.has("q")).toBe(false);
  737 |     await noPII(page);
  738 |   }
  739 |   await page.reload();
  740 |   await expect(page.getByTestId("orders-search")).toHaveValue(""); // deliberate privacy rule, not storage
  741 |   clicks.push({control:"refresh private search",result:"query cleared, not persisted in URL or storage"});
  742 |   for (const payment of ["cash_on_delivery","bank_transfer","pay_at_pickup","card"]) {
  743 |     await page.getByTestId("orders-payment-filter").selectOption(payment);
  744 |     await apply(`payment ${payment}`, () => page.getByTestId("orders-apply").click());
  745 |     await expect(page).toHaveURL(new RegExp(`payment_mode=${payment}`));
  746 |   }
  747 |   await page.getByTestId("orders-delivery-filter").selectOption("home");
  748 |   await apply("delivery home", () => page.getByTestId("orders-apply").click());
  749 |   await page.reload();
  750 |   await expect(page.getByTestId("orders-payment-filter")).toHaveValue("card");
  751 |   await expect(page.getByTestId("orders-delivery-filter")).toHaveValue("home");
  752 |   await apply("reset filters", () => page.getByTestId("orders-reset").click());
  753 |   await apply("inspect drafts for recorded live claim", () => page.getByTestId("state-filter").selectOption("all"));
  754 |   await page.getByTestId("orders-session-filter").selectOption(ids.live_session);
  755 |   const live = await apply("live session", () => page.getByTestId("orders-apply").click());
  756 |   expect(live.total).toBe(1);
  757 |   await expect(page.getByTestId(`order-row-${ids.live_order}`)).toContainText("Live claim");
  758 |   await page.reload();
  759 |   await expect(page.getByTestId("orders-session-filter")).toHaveValue(ids.live_session);
  760 |   await apply("reset live filter", () => page.getByTestId("orders-reset").click());
  761 |   const today = new Intl.DateTimeFormat("en-CA",{timeZone:"Asia/Taipei",year:"numeric",month:"2-digit",day:"2-digit"}).format(new Date());
  762 |   await page.getByTestId("orders-from").fill(today);
  763 |   await page.getByTestId("orders-to").fill(today);
  764 |   const dated = await apply("Taipei day", () => page.getByTestId("orders-apply").click());
  765 |   expect(dated.total).toBeGreaterThan(0);
  766 |   await page.reload();
  767 |   await expect(page.getByTestId("orders-from")).toHaveValue(today);
  768 |   await expect(page.getByTestId("orders-to")).toHaveValue(today);
  769 |   await apply("reset before visual acceptance", () => page.getByTestId("orders-reset").click());
  770 |   await apply("active ledger", () => page.getByTestId("state-filter").selectOption("active"));
  771 |   for (const locale of ["zh-TW","zh-CN","en"]) {
  772 |     await page.getByTestId("locale-switch").selectOption(locale);
  773 |     await expect(page.getByTestId("orders-table")).toBeVisible();
  774 |     for (const [width,height] of [[1586,992],[1366,768],[390,844]]) {
  775 |       await page.setViewportSize({width,height});
  776 |       if (width === 390) {
  777 |         // Real responsive transition must finish; never screenshot a half-open rail.
  778 |         await expect.poll(() => page.locator("aside[data-shell-rail]").evaluate(node => node.getBoundingClientRect().right)).toBeLessThanOrEqual(0);
  779 |         const more = page.getByTestId("orders-more-filters");
  780 |         await expect(more).toHaveAttribute("aria-expanded", "false");
  781 |         await expect(page.getByTestId("orders-payment-filter")).toBeHidden();
  782 |         await expect(page.getByTestId("state-filter")).toBeHidden();
  783 |         await more.click();
  784 |         await expect(more).toHaveAttribute("aria-expanded", "true");
  785 |         await page.getByTestId("orders-payment-filter").selectOption("card");
  786 |         await page.getByTestId("orders-apply").click();
  787 |         await expect(page).toHaveURL(/payment_mode=card/);
  788 |         await expect(more).toHaveAttribute("aria-expanded", "false");
  789 |         await more.click();
  790 |         await expect(page.getByTestId("orders-payment-filter")).toHaveValue("card");
  791 |         await expect(page.getByTestId("orders-from")).toBeVisible();
  792 |         await page.getByTestId("orders-reset").click();
  793 |         await expect(page).not.toHaveURL(/payment_mode=/);
  794 |         await expect(more).toHaveAttribute("aria-expanded", "false");
  795 |         await more.click();
  796 |         const refreshRead = page.waitForResponse(r => r.url().includes(`/api/stores/${store}/orders?`) && r.status() === 200);
  797 |         await page.getByTestId("orders-refresh").click();
  798 |         await refreshRead;
  799 |         await expect(page.getByTestId("orders-table")).toBeVisible();
  800 |         await more.click();
  801 |         await expect(more).toHaveAttribute("aria-expanded", "false");
  802 |         clicks.push({control:`mobile refresh ${locale}`,result:"open secondary controls, real refresh/readback, collapse"});
  803 |       }
  804 |       if (width === 390) {
  805 |         const rail = page.getByTestId("orders-tabs");
  806 |         // Native click/keyboard drive scrolling; evaluate below only measures geometry.
  807 |         await rail.hover();
  808 |         await page.mouse.wheel(1200, 0);
  809 |         await expect.poll(() => rail.evaluate(node => node.scrollLeft)).toBeGreaterThan(0);
  810 |         await page.getByTestId("orders-bucket-cancelled").click();
  811 |         await expect(page.getByTestId("orders-bucket-cancelled")).toHaveAttribute("aria-pressed", "true");
  812 |         await page.reload();
  813 |         await expect.poll(() => rail.evaluate(node => node.scrollLeft)).toBeGreaterThan(0);
  814 |         const bounds = await rail.boundingBox(), active = await page.getByTestId("orders-bucket-cancelled").boundingBox();
  815 |         expect(active!.x).toBeGreaterThanOrEqual(bounds!.x - 1);
  816 |         expect(active!.x + active!.width).toBeLessThanOrEqual(bounds!.x + bounds!.width + 1);
  817 |         const tabs = await rail.locator("button").evaluateAll(nodes => nodes.map(node => ({y:node.getBoundingClientRect().y,h:node.getBoundingClientRect().height,w:node.getBoundingClientRect().width})));
  818 |         expect(new Set(tabs.map(tab => Math.round(tab.y))).size).toBe(1);
  819 |         expect(tabs.every(tab => tab.h >= 44 && tab.w >= 44)).toBe(true);
  820 |         await screenshot(page,resolve(v2Evidence,`queue-scroll-${locale}-390x844.png`));
  821 |         await page.getByTestId("orders-bucket-all").click();
  822 |         await expect(page.getByTestId("orders-bucket-all")).toHaveAttribute("aria-pressed", "true");
  823 |         clicks.push({control:`horizontal queue ${locale}`,result:"real horizontal wheel then click, single row, 44px targets, cancelled visible after reload; returned to all"});
  824 |       }
  825 |       expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
> 826 |       await page.getByRole("heading",{level:1}).scrollIntoViewIfNeeded();
      |                                                 ^ Error: locator.scrollIntoViewIfNeeded: Element is not attached to the DOM
  827 |       await screenshot(page,resolve(v2Evidence,`orders-${locale}-${width}x${height}.png`));
  828 |       if (width === 390) {
  829 |         const row = await page.locator('[data-testid^="order-expand-"]').first().boundingBox();
  830 |         expect(row).not.toBeNull();
  831 |         expect(row!.y).toBeLessThan(height); // The first order, not only a filter form, is in the initial viewport.
  832 |         clicks.push({control:`mobile secondary filters ${locale}`, result:`native toggle, select, apply, reopen persisted value, reset; first order y=${row!.y} < ${height}`});
  833 |       }
  834 |       await page.getByTestId("orders-table").scrollIntoViewIfNeeded();
  835 |       await screenshot(page,resolve(v2Evidence,`ledger-${locale}-${width}x${height}.png`));
  836 |     }
  837 |   }
  838 |   await writeFile(resolve(v2Evidence,"click-ledger.json"),JSON.stringify(clicks.map(item => ({page:"Orders",control:item.control,action:item.control,expected:"Assertions and persisted readback described by the named scenario pass",actual:item.result,status:"PASS"})),null,2));
  839 | });
  840 | 
  841 | for (const privateCursor of [false, true]) test(`MOU08 authoritative poll denial clears list and full detail (private cursor=${privateCursor})`, async ({page}) => {
  842 |   test.setTimeout(60_000);
  843 |   await signedLogin(page);
  844 |   if (privateCursor) {
  845 |     await page.getByTestId("orders-search").fill("0001");
  846 |     const searchResponse = page.waitForResponse(r => r.request().method() === "POST" && r.url().includes("/orders/search?") && r.status() === 200);
  847 |     await page.getByTestId("orders-apply").click();
  848 |     await searchResponse;
  849 |     await expect(page.getByTestId("orders-next")).toBeEnabled();
  850 |     const nextResponse = page.waitForResponse(r => r.request().method() === "POST" && r.url().includes("cursor=") && r.status() === 200);
  851 |     await page.getByTestId("orders-next").click();
  852 |     const next = await (await nextResponse).json();
  853 |     await expect(page.getByTestId(`order-expand-${next.items[0].order_id}`)).toBeVisible();
  854 |     await page.getByTestId(`order-expand-${next.items[0].order_id}`).click();
  855 |     expect(new URL(page.url()).searchParams.has("q")).toBe(false);
  856 |     expect(new URL(page.url()).searchParams.has("cursor")).toBe(false);
  857 |   } else await expand(page,ids.captured);
  858 |   await expect(page.getByTestId("order-detail")).toContainText("Synthetic home address");
  859 |   // Explicit fault injection of a backend permission revocation. No DOM events or synthetic clicks.
  860 |   await page.route(`**/api/stores/${store}/orders${privateCursor ? "/search" : ""}?*`,route=>route.fulfill({status:403,contentType:"application/json",headers:{"cache-control":"private, no-store"},body:'{"code":"forbidden"}'}));
  861 |   await expect(page.getByTestId("orders-table")).toHaveCount(0,{timeout:25_000});
  862 |   await expect(page.getByTestId("order-detail")).toHaveCount(0);
  863 |   await expect(page.getByTestId("merchant-orders")).toContainText("This account does not have permission to read orders.");
  864 |   await noPII(page);
  865 | });
  866 | 
  867 | test("MOU05 approved inline comp at desktop/mobile in three locales and page-two locale context", async ({
  868 |   page,
  869 | }, testInfo) => {
  870 |   await signedLogin(page);
  871 |   await expand(page, ids.pickup); // Native comp: first row, immediately expanded.
  872 |   for (const locale of ["en", "zh-CN", "zh-TW"] as const) {
  873 |     await page.getByTestId("locale-switch").selectOption(locale);
  874 |     await expect(page).toHaveURL(new RegExp(`/${locale}/orders`));
  875 |     await expect(page).toHaveURL(new RegExp(`order=${ids.pickup}`));
  876 |     await expect(page.getByTestId("order-detail")).toHaveAttribute(
  877 |       "aria-label",
  878 |       new RegExp(ids.pickup),
  879 |     );
  880 |     await page.setViewportSize({ width: 1586, height: 992 });
  881 |     // G-UI8 audit [READ/MEASURE]: waits for CSS animations to finish before measuring (read/wait)
  882 |     await page.evaluate(() =>
  883 |       Promise.all(
  884 |         document
  885 |           .getAnimations()
  886 |           .map((animation) => animation.finished.catch(() => undefined)),
  887 |       ),
  888 |     );
  889 |     const cells = await page.getByTestId("order-detail").evaluate((detail) => {
  890 |       const product = detail.querySelector(
  891 |         ".orders-items tbody td:first-child",
  892 |       );
  893 |       const price = detail.querySelector(".orders-items tbody td:nth-child(2)");
  894 |       if (!product || !price) throw new Error("missing frozen item cells");
  895 |       return {
  896 |         productRight: product.getBoundingClientRect().right,
  897 |         priceLeft: price.getBoundingClientRect().left,
  898 |         codeRight:
  899 |           product.querySelector("small")?.getBoundingClientRect().right ?? 0,
  900 |         nameRight:
  901 |           product.querySelector("strong")?.getBoundingClientRect().right ?? 0,
  902 |       };
  903 |     });
  904 |     expect(cells.productRight).toBeLessThanOrEqual(cells.priceLeft + 1);
  905 |     expect(cells.codeRight).toBeLessThanOrEqual(cells.productRight + 1);
  906 |     expect(cells.nameRight).toBeLessThanOrEqual(cells.productRight + 1);
  907 |     await paymentBadgesFit(page);
  908 |     await screenshot(
  909 |       page,
  910 |       testInfo.outputPath(`inline-${locale}-1586x992.png`),
  911 |     );
  912 |     await page.setViewportSize({ width: 390, height: 844 });
  913 |     // G-UI8 audit [READ/MEASURE]: waits for CSS animations to finish before measuring (read/wait)
  914 |     await page.evaluate(() =>
  915 |       Promise.all(
  916 |         document
  917 |           .getAnimations()
  918 |           .map((animation) => animation.finished.catch(() => undefined)),
  919 |       ),
  920 |     );
  921 |     const rail = await page.locator("[data-shell-rail]").evaluate((element) => ({
  922 |       open: element.classList.contains("open"),
  923 |       right: element.getBoundingClientRect().right,
  924 |     }));
  925 |     expect(rail.open).toBe(false);
  926 |     expect(rail.right).toBeLessThanOrEqual(1);
```