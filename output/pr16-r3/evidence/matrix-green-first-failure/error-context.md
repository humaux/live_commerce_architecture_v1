# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: catalog-core.spec.ts >> PE12-17 document workflow, matrix, list actions and click ledger
- Location: tests/admin/product-editor.acceptance.ts:16:3

# Error details

```
Error: expect(locator).toHaveCount(expected) failed

Locator:  locator('[data-testid^="matrix-row-"]')
Expected: 2
Received: 1
Timeout:  10000ms

Call log:
  - Expect "toHaveCount" locator('[data-testid^="matrix-row-"]') with timeout 10000ms
  - waiting for locator('[data-testid^="matrix-row-"]')
    - locator resolved to 0 elements
    - unexpected value "0"
    23 × locator resolved to 1 element
       - unexpected value "1"

```

# Page snapshot

```yaml
- generic [active] [ref=f37e1]:
  - generic [ref=f37e2]:
    - link "跳至內容" [ref=f37e3] [cursor=pointer]:
      - /url: "#main"
    - generic [ref=f37e4]:
      - banner [ref=f37e5]:
        - button "開啟導覽" [ref=f37e6] [cursor=pointer]
        - generic [ref=f37e9]:
          - generic [ref=f37e10]: 切換商店
          - generic [aria-hidden] [ref=f37e11]:
            - generic [ref=f37e12]: fixture-store-b
            - generic [ref=f37e13]: ⌄
          - combobox "切換商店" [ref=f37e14] [cursor=pointer]:
            - option "fixture-store-b" [selected]
        - generic [ref=f37e15]:
          - generic [ref=f37e16]: 語言
          - combobox "語言" [ref=f37e17] [cursor=pointer]:
            - option "简体中文"
            - option "繁體中文" [selected]
            - option "English"
        - group [ref=f37e18]:
          - generic "說明" [ref=f37e19] [cursor=pointer]
        - group [ref=f37e20]:
          - generic "帳號" [ref=f37e21] [cursor=pointer]
      - main [ref=f37e22]:
        - navigation "工作區導覽" [ref=f37e23]:
          - link "總覽" [ref=f37e24] [cursor=pointer]:
            - /url: /zh-TW/
          - generic [aria-hidden] [ref=f37e25]: /
          - generic [ref=f37e26]: 商品與庫存
          - generic [aria-hidden] [ref=f37e27]: /
          - generic [ref=f37e28]: 編輯商品
        - generic [ref=f37e29]:
          - generic [ref=f37e30]:
            - generic [ref=f37e31]:
              - heading "編輯商品" [level=1] [ref=f37e32]
              - paragraph [ref=f37e33]: pecc2ddeacc1ed95 mobile matrix zh-TW
            - link "全部商品" [ref=f37e35] [cursor=pointer]:
              - /url: /zh-TW/products?store=318ed34b-a88f-4284-8328-f45e6324aa4e
          - generic [ref=f37e36]:
            - complementary [ref=f37e37]:
              - navigation "完成度" [ref=f37e38]:
                - button "商品圖片" [ref=f37e39] [cursor=pointer]
                - button "基本資訊" [ref=f37e40] [cursor=pointer]
                - button "規格" [ref=f37e41] [cursor=pointer]
                - button "分類" [ref=f37e42] [cursor=pointer]
                - button "物流" [ref=f37e43] [cursor=pointer]
                - button "搜尋引擎最佳化" [ref=f37e44] [cursor=pointer]
              - button "完成度" [ref=f37e45] [cursor=pointer]
            - generic [ref=f37e46]:
              - generic [ref=f37e48]:
                - region [ref=f37e49]:
                  - heading "主圖 0/4" [level=2] [ref=f37e50]:
                    - text: 主圖
                    - generic [ref=f37e51]: 0/4
                  - paragraph [ref=f37e52]: 最多4張，第一張為封面。建議使用800px以上的正方形照片。
                  - paragraph [ref=f37e54]: 尚未新增圖片。
                  - generic [ref=f37e55] [cursor=pointer]:
                    - generic [ref=f37e56]: 新增圖片
                    - 'button "新增圖片: 主圖" [ref=f37e57]'
                - region [ref=f37e58]:
                  - heading "規格圖" [level=2] [ref=f37e59]
                  - paragraph [ref=f37e60]: 每個圖片規格值各一張；沒有規格圖時，買家會看到封面。
                  - generic [ref=f37e61]:
                    - generic [ref=f37e62]: 圖片規格
                    - combobox "圖片規格" [ref=f37e63]:
                      - option "使用第一個規格"
                      - option "顏色" [selected]
                      - option "尺寸"
                  - generic [ref=f37e64]:
                    - generic [ref=f37e65]:
                      - heading "White" [level=3] [ref=f37e66]
                      - paragraph [ref=f37e67]: 使用封面
                      - generic [ref=f37e68] [cursor=pointer]:
                        - generic [ref=f37e69]: 新增規格圖
                        - 'button "新增規格圖: White" [ref=f37e70]'
                    - generic [ref=f37e71]:
                      - heading "Black" [level=3] [ref=f37e72]
                      - paragraph [ref=f37e73]: 使用封面
                      - generic [ref=f37e74] [cursor=pointer]:
                        - generic [ref=f37e75]: 新增規格圖
                        - 'button "新增規格圖: Black" [ref=f37e76]'
                - region [ref=f37e77]:
                  - heading "詳情圖 0/20" [level=2] [ref=f37e78]:
                    - text: 詳情圖
                    - generic [ref=f37e79]: 0/20
                  - paragraph [ref=f37e80]: 最多20張，可上傳長圖，高度最多為寬度的6倍。
                  - paragraph [ref=f37e82]: 尚未新增圖片。
                  - generic [ref=f37e83] [cursor=pointer]:
                    - generic [ref=f37e84]: 新增圖片
                    - 'button "新增圖片: 詳情圖" [ref=f37e85]'
              - group [ref=f37e86]:
                - generic [ref=f37e87]:
                  - heading "基本資訊" [level=2] [ref=f37e88]
                  - generic [ref=f37e89]:
                    - text: 顯示狀態
                    - combobox "顯示狀態 草稿：只有你看得到，買家找不到也買不到。" [ref=f37e90]:
                      - option "草稿" [selected]
                      - option "上架中"
                    - generic [ref=f37e91]: 草稿：只有你看得到，買家找不到也買不到。
                  - generic [ref=f37e92]:
                    - text: 商品名稱
                    - textbox "商品名稱 36 / 120" [ref=f37e93]: pecc2ddeacc1ed95 mobile matrix zh-TW
                    - generic [ref=f37e94]: 36 / 120
                  - generic [ref=f37e95]:
                    - text: 描述
                    - textbox "描述" [ref=f37e96]
                - region [ref=f37e97]:
                  - heading "規格" [level=2] [ref=f37e98]
                  - generic [ref=f37e99]:
                    - generic [ref=f37e100]:
                      - generic [ref=f37e101]:
                        - generic [ref=f37e102]: 規格名稱
                        - combobox "規格名稱" [ref=f37e103]: 顏色
                      - generic [ref=f37e104]:
                        - generic [ref=f37e105]: 規格值
                        - textbox "規格值" [ref=f37e106]
                        - generic [ref=f37e107]: 按 Enter 或用逗號、換行分隔，最多 50 個不重複值。
                        - generic [ref=f37e108]:
                          - generic [ref=f37e109]:
                            - text: White
                            - button "移除 White" [ref=f37e110] [cursor=pointer]: ×
                          - generic [ref=f37e111]:
                            - text: Black
                            - button "移除 Black" [ref=f37e112] [cursor=pointer]: ×
                      - button "移除 規格名稱 1" [ref=f37e113] [cursor=pointer]: 移除
                    - generic [ref=f37e114]:
                      - generic [ref=f37e115]:
                        - generic [ref=f37e116]: 規格名稱
                        - combobox "規格名稱" [ref=f37e117]: 尺寸
                      - generic [ref=f37e118]:
                        - generic [ref=f37e119]: 規格值
                        - textbox "規格值" [ref=f37e120]
                        - generic [ref=f37e121]: 按 Enter 或用逗號、換行分隔，最多 50 個不重複值。
                        - generic [ref=f37e123]:
                          - text: S
                          - button "移除 S" [ref=f37e124] [cursor=pointer]: ×
                      - button "移除 規格名稱 2" [ref=f37e125] [cursor=pointer]: 移除
                  - button "新增規格（顏色、尺寸…）" [ref=f37e126] [cursor=pointer]
                  - generic [ref=f37e127]:
                    - button "批量填入" [ref=f37e129] [cursor=pointer]
                    - paragraph [ref=f37e130]: "1 / 100 · 已選取: 0"
                  - region "規格" [ref=f37e131]:
                    - generic [aria-hidden] [ref=f37e132]:
                      - generic [ref=f37e133]: 規格
                      - generic [ref=f37e134]: 售價 (NT$)
                      - generic [ref=f37e135]: 原價
                      - generic [ref=f37e136]: 庫存
                      - generic [ref=f37e137]: 貨號
                      - generic [ref=f37e138]: 直播關鍵字
                      - generic [ref=f37e139]: 啟用
                    - generic [ref=f37e140]:
                      - generic "Black / S" [ref=f37e141]:
                        - checkbox "已選取 Black / S" [ref=f37e142]
                        - strong [ref=f37e143]: Black / S
                      - generic [ref=f37e144]:
                        - generic [ref=f37e145]: 售價
                        - textbox "售價 Black / S" [ref=f37e146]: "81"
                      - generic [ref=f37e147]:
                        - generic [ref=f37e148]: 原價
                        - textbox "原價 1" [ref=f37e149]: "121"
                      - generic [ref=f37e150]:
                        - generic "不追蹤 ∞" [ref=f37e151]:
                          - checkbox "不追蹤 ∞" [ref=f37e152]
                          - text: 不追蹤 ∞
                        - generic [ref=f37e153]:
                          - generic [ref=f37e154]: 改後庫存
                          - textbox "改後庫存 1" [ref=f37e155]: "8"
                      - generic [ref=f37e156]:
                        - generic [ref=f37e157]: 貨號
                        - textbox "貨號 1" [disabled] [ref=f37e158]:
                          - /placeholder: 留空由系統產生
                          - text: M1520330692-0-1
                      - generic [ref=f37e159]:
                        - generic [ref=f37e160]: 直播關鍵字
                        - textbox "直播關鍵字 1" [ref=f37e161]: M15203306920E1
                      - generic [ref=f37e162]:
                        - checkbox "啟用 1" [checked] [ref=f37e163]
                        - generic [ref=f37e164]: 啟用
                - generic [ref=f37e165]:
                  - heading "分類" [level=2] [ref=f37e166]
                  - generic [ref=f37e167]:
                    - text: 搜尋分類
                    - searchbox "搜尋分類" [ref=f37e168]
                  - generic [ref=f37e170]:
                    - checkbox "pecc2ddeacc1ed95 picks" [ref=f37e171]
                    - text: pecc2ddeacc1ed95 picks
                  - link "管理分類" [ref=f37e172] [cursor=pointer]:
                    - /url: /zh-TW/collections?store=318ed34b-a88f-4284-8328-f45e6324aa4e
                - group [ref=f37e173]:
                  - generic "+ 物流" [ref=f37e174] [cursor=pointer]
                - group [ref=f37e175]:
                  - generic "+ 搜尋引擎最佳化" [ref=f37e176] [cursor=pointer]
            - generic [ref=f37e178]:
              - button "儲存修改" [ref=f37e179] [cursor=pointer]
              - button "上架" [ref=f37e180] [cursor=pointer]
        - paragraph [ref=f37e182]: DaWan Live 由 香港大碗貿易有限公司 營運
  - alert [ref=f37e183]
```

# Test source

```ts
  688 |           };
  689 |           for (const [i, state] of rows.entries()) {
  690 |             const row = page.getByTestId(`matrix-row-${i}`);
  691 |             await expect(row.locator("strong")).toHaveText(state.name);
  692 |             const selected = row.getByTestId(`matrix-select-${i}`);
  693 |             await selected.check();
  694 |             await expect(selected).toBeChecked();
  695 |             await selected.uncheck();
  696 |             await expect(selected).not.toBeChecked();
  697 |             ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, row: state.name, control: `matrix-select-${i}`, action: "check uncheck", expected: "selected true→false", actual: "PASS", persistence: "selection is view state" });
  698 |             const tracking = row.getByRole("checkbox", { name: c.untracked, exact: true });
  699 |             // Exercise both tracking states, then leave distinct persisted modes.
  700 |             await tracking.check();
  701 |             await expect(tracking).toBeChecked();
  702 |             await tracking.uncheck();
  703 |             await expect(tracking).not.toBeChecked();
  704 |             if (state.untracked) await tracking.check();
  705 |             await expect(tracking).toBeChecked({ checked: state.untracked });
  706 |             await expect(tracking.locator("..")).toHaveText(c.untracked);
  707 |             await expect(tracking.locator("..")).toBeVisible();
  708 |             ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, row: state.name, control: c.untracked, action: "check uncheck; choose mode", expected: { untracked: state.untracked }, actual: "PASS", persistence: "verified after UI save below" });
  709 |             for (const field of fieldsFor(i, editing)) {
  710 |               const label = field.control.locator("..").locator(":scope > span");
  711 |               await expect(label, `${locale} ${phase} ${state.name}: visible ${field.label}`).toBeVisible();
  712 |               await expect(label).toHaveText(field.label);
  713 |               await label.scrollIntoViewIfNeeded();
  714 |               await expect(label).toBeInViewport({ ratio: 0.99 });
  715 |               if (editing && field.label === c.code) await expect(field.control).toBeDisabled();
  716 |               else await field.control.fill(field.value);
  717 |               await expect(field.control).toHaveValue(field.value);
  718 |               ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, row: state.name, control: field.label, action: editing && field.label === c.code ? "verify existing SKU code disabled" : "fill", expected: { visibleLabel: field.label, value: field.value }, actual: "PASS", persistence: "verified after UI save below" });
  719 |             }
  720 |             const active = row.getByTestId(`matrix-active-${i}`);
  721 |             await active.uncheck();
  722 |             await expect(active).not.toBeChecked();
  723 |             await active.check();
  724 |             await expect(active).toBeChecked();
  725 |             const activeLabel = row.locator("label.pe-check > span");
  726 |             await expect(activeLabel).toHaveText(c.active);
  727 |             await expect(activeLabel).toBeVisible();
  728 |             await activeLabel.scrollIntoViewIfNeeded();
  729 |             await expect(activeLabel).toBeInViewport({ ratio: 0.99 });
  730 |             ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, row: state.name, control: `matrix-active-${i}`, action: "uncheck check", expected: { visibleLabel: c.active, enabled: true }, actual: "PASS", persistence: "verified after UI save below" });
  731 |           }
  732 |           const saveStart = writes.length;
  733 |           const response = page.waitForResponse((r) => r.url().includes(`/api/stores/${store}/products/`) && r.url().endsWith("/document") && r.request().method() !== "GET");
  734 |           await page.getByTestId(editing ? "product-save" : "product-create").click();
  735 |           expect((await response).ok()).toBe(true);
  736 |           if (!editing) {
  737 |             const result = page.getByTestId("product-save-result");
  738 |             await expect(result).toBeVisible();
  739 |             await result.getByRole("link", { name: c.save, exact: true }).click();
  740 |           }
  741 |           await expect(page.getByTestId("product-save")).toBeEnabled();
  742 |           await page.reload();
  743 |           await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(2);
  744 |           for (const [i, name, values] of [[0, c.color, ["White", "Black"]], [1, c.size, ["S"]]] as const) {
  745 |             await expect(page.getByTestId(`axis-name-${i}`)).toHaveValue(name);
  746 |             const chips = page.locator(".product-axis").nth(i).locator(".pe-value-chips > span");
  747 |             await expect(chips).toHaveCount(values.length);
  748 |             await expect(chips).toContainText([...values]);
  749 |             ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, control: `saved axis-${i}`, action: "UI save, editor reopen/reload", expected: { name, values }, actual: "PASS", tier: "BROWSER+REAL_PG" });
  750 |           }
  751 |           for (const [i, state] of rows.entries()) {
  752 |             const row = page.getByTestId(`matrix-row-${i}`);
  753 |             await expect(row.locator("strong")).toHaveText(state.name);
  754 |             for (const field of fieldsFor(i, true)) {
  755 |               const label = field.control.locator("..").locator(":scope > span");
  756 |               await expect(label).toBeVisible();
  757 |               await expect(label).toHaveText(field.label);
  758 |               await label.scrollIntoViewIfNeeded();
  759 |               await expect(label).toBeInViewport({ ratio: 0.99 });
  760 |               await expect(field.control).toHaveValue(field.value);
  761 |             }
  762 |             await expect(row.getByRole("checkbox", { name: c.untracked, exact: true })).toBeChecked({ checked: state.untracked });
  763 |             await expect(row.getByTestId(`matrix-active-${i}`)).toBeChecked();
  764 |             await expect(row.getByTestId(`matrix-code-${i}`)).toBeDisabled();
  765 |             ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, row: state.name, control: "saved row readback", action: "UI save, editor reopen/reload, read visible fields", expected: state, actual: "PASS", tier: "BROWSER+REAL_PG" });
  766 |           }
  767 |           expect(writes.slice(saveStart).filter((r) => r.path.endsWith("/document"))).toHaveLength(1);
  768 |           const capture = `mobile-matrix-${locale}-390-${phase}`;
  769 |           await page.getByTestId("new-price-0").scrollIntoViewIfNeeded();
  770 |           await shot(capture);
  771 |           ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, control: editing ? "product-save" : "product-create", action: "click reload", expected: "one document write; both matrix rows persist", actual: "PASS", screenshot: `${capture}.png`, tier: "BROWSER+REAL_PG" });
  772 |         }
  773 |         // Enabled=false archives an existing SKU; the detail contract returns
  774 |         // active SKUs only. After reload the missing axis combination is a
  775 |         // visibly blank new-row placeholder, not a reversible disabled SKU.
  776 |         const white = page.getByTestId("matrix-row-0"), black = page.getByTestId("matrix-row-1");
  777 |         const whiteID = await white.getAttribute("data-sku-id"), blackID = await black.getAttribute("data-sku-id");
  778 |         expect(whiteID).toMatch(/^[0-9a-f-]{36}$/);
  779 |         expect(blackID).toMatch(/^[0-9a-f-]{36}$/);
  780 |         await white.getByTestId("matrix-active-0").uncheck();
  781 |         await expect(white.getByTestId("matrix-active-0")).not.toBeChecked();
  782 |         const archiveStart = writes.length;
  783 |         const archivedResponse = page.waitForResponse((r) => r.url().includes(`/api/stores/${store}/products/`) && r.url().endsWith("/document") && r.request().method() !== "GET");
  784 |         await page.getByTestId("product-save").click();
  785 |         expect((await archivedResponse).ok()).toBe(true);
  786 |         await expect(page.getByTestId("product-save")).toBeEnabled();
  787 |         await page.reload();
> 788 |         await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(2);
      |                                                                    ^ Error: expect(locator).toHaveCount(expected) failed
  789 |         await expect(white.locator("strong")).toHaveText("White / S");
  790 |         await expect(white).not.toHaveAttribute("data-sku-id");
  791 |         const blankPrice = white.getByTestId("new-price-0");
  792 |         await blankPrice.scrollIntoViewIfNeeded();
  793 |         await expect(blankPrice).toBeVisible();
  794 |         await expect(blankPrice).toBeInViewport({ ratio: 0.99 });
  795 |         await expect(blankPrice).toHaveValue("");
  796 |         await expect(white.getByTestId("matrix-compare-0")).toHaveValue("");
  797 |         await expect(white.getByTestId("matrix-code-0")).toHaveValue("");
  798 |         await expect(white.getByTestId("matrix-code-0")).toBeEnabled();
  799 |         await expect(black).toHaveAttribute("data-sku-id", blackID!);
  800 |         await expect(black.locator("strong")).toHaveText("Black / S");
  801 |         await expect(black.getByTestId("new-price-1")).toHaveValue("81");
  802 |         await expect(black.getByTestId("matrix-compare-1")).toHaveValue("121");
  803 |         await expect(black.getByTestId("matrix-quantity-1")).toHaveValue("8");
  804 |         await expect(black.getByRole("checkbox", { name: c.untracked, exact: true })).not.toBeChecked();
  805 |         await expect(black.getByTestId("matrix-code-1")).toHaveValue(`${mobileMatrixTag}-${localeIndex}-1`);
  806 |         await expect(black.getByRole("textbox", { name: `${c.keyword} 2`, exact: true })).toHaveValue(`${mobileMatrixTag}${localeIndex}E1`);
  807 |         await expect(black.getByTestId("matrix-active-1")).toBeChecked();
  808 |         expect(writes.slice(archiveStart).filter((r) => r.path.endsWith("/document"))).toHaveLength(1);
  809 |         const archiveCapture = `mobile-matrix-${locale}-390-archived`;
  810 |         await shot(archiveCapture);
  811 |         ledger.push({ page: "mobile matrix edit", locale, width: 390, control: "matrix-active-0/product-save", action: "uncheck White, click save, reload", expected: { archivedSKU: whiteID, white: "visible blank new-row placeholder", black: { id: blackID, price: "81", compare: "121", trackedQuantity: "8", active: true } }, actual: "PASS", screenshot: `${archiveCapture}.png`, tier: "BROWSER+REAL_PG" });
  812 |       }
  813 |       // Controlled read-only ambiguity: do not fabricate a stock quantity when the API has no single warehouse.
  814 |       await page.route(
  815 |         `**/api/stores/${store}/products/${id}`,
  816 |         async (route) => {
  817 |           const res = await route.fetch();
  818 |           const body = await res.json();
  819 |           body.warehouse_id = null;
  820 |           for (const s of body.skus) {
  821 |             s.on_hand = null;
  822 |             s.committed = null;
  823 |           }
  824 |           await route.fulfill({ response: res, json: body });
  825 |         },
  826 |       );
  827 |       await page.goto(url("en", "products/" + id));
  828 |       await expect(page.getByTestId("product-quantity")).toBeDisabled();
  829 |       await expect(
  830 |         page.getByRole("link", { name: "Open inventory", exact: true }),
  831 |       ).toBeVisible();
  832 |       await page.unroute(`**/api/stores/${store}/products/${id}`);
  833 |       ledger.push({
  834 |         page: "edit",
  835 |         control: "ambiguous warehouse",
  836 |         action: "read-only response MOCK + visible disabled field",
  837 |         expected: "no available fallback, disabled target and inventory link",
  838 |         actual: "PASS (MOCK read contract)",
  839 |       });
  840 |       // The PG PE23 gate exercises the actual open-window rule. This isolated
  841 |       // response mock proves every locale explains that rule after a real click.
  842 |       const documentPattern = `**/api/stores/${store}/products/${id}/document`;
  843 |       await page.route(documentPattern, (route) =>
  844 |         route.fulfill({ status: 409, json: { code: "live_window_open" } }),
  845 |       );
  846 |       for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
  847 |         await page.goto(url(locale, "products/" + id));
  848 |         await page.getByTestId("product-keyword").fill("BLOCKED-LIVE");
  849 |         await page.getByTestId("product-save").click();
  850 |         await expect(
  851 |           page.getByText(productEditorCopy[locale].live_window_open, {
  852 |             exact: true,
  853 |           }),
  854 |         ).toBeVisible();
  855 |         ledger.push({
  856 |           page: "edit",
  857 |           locale,
  858 |           control: "live-window keyword refusal",
  859 |           action: "click + MOCK 409",
  860 |           expected: "explicit translated safety refusal",
  861 |           actual: "PASS (MOCK response; real rule covered by PG PE21)",
  862 |         });
  863 |       }
  864 |       await page.unroute(documentPattern);
  865 |       await page.goto(url("en", "products"));
  866 |       // Fault injection: server commits a copy, but the browser loses that response.
  867 |       // After reload the receipt fence must prevent sending a fresh-key duplicate.
  868 |       const copyPattern = `**/products/${id}/copy`;
  869 |       await page.route(copyPattern, async (route) => {
  870 |         await route.fetch();
  871 |         await route.abort("failed");
  872 |       });
  873 |       const beforeUnknown = writes.length;
  874 |       await page
  875 |         .getByTestId(`product-row-${id}`)
  876 |         .getByRole("button", { name: "Copy", exact: true })
  877 |         .click();
  878 |       await expect(
  879 |         page.getByRole("button", {
  880 |           name: productEditorCopy.en.retry,
  881 |           exact: true,
  882 |         }),
  883 |       ).toBeVisible();
  884 |       await page.reload();
  885 |       await expect(
  886 |         page.getByText(productEditorCopy.en.listRecoveryRequired, {
  887 |           exact: true,
  888 |         }),
```