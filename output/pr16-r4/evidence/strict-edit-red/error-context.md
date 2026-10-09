# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: catalog-core.spec.ts >> PE12-17 document workflow, matrix, list actions and click ledger
- Location: tests/admin/product-editor.acceptance.ts:16:3

# Error details

```
Error: expect(received).toBe(expected) // Object.is equality

Expected: true
Received: false
```

# Page snapshot

```yaml
- generic [ref=f39e1]:
  - generic [ref=f39e2]:
    - link "跳至內容" [ref=f39e3] [cursor=pointer]:
      - /url: "#main"
    - generic [ref=f39e4]:
      - banner [ref=f39e5]:
        - button "開啟導覽" [ref=f39e6] [cursor=pointer]
        - generic [ref=f39e9]:
          - generic [ref=f39e10]: 切換商店
          - generic [aria-hidden] [ref=f39e11]:
            - generic [ref=f39e12]: fixture-store-b
            - generic [ref=f39e13]: ⌄
          - combobox "切換商店" [ref=f39e14] [cursor=pointer]:
            - option "fixture-store-b" [selected]
        - generic [ref=f39e15]:
          - generic [ref=f39e16]: 語言
          - combobox "語言" [ref=f39e17] [cursor=pointer]:
            - option "简体中文"
            - option "繁體中文" [selected]
            - option "English"
        - group [ref=f39e18]:
          - generic "說明" [ref=f39e19] [cursor=pointer]
        - group [ref=f39e20]:
          - generic "帳號" [ref=f39e21] [cursor=pointer]
      - main [ref=f39e22]:
        - navigation "工作區導覽" [ref=f39e23]:
          - link "總覽" [ref=f39e24] [cursor=pointer]:
            - /url: /zh-TW/
          - generic [aria-hidden] [ref=f39e25]: /
          - generic [ref=f39e26]: 商品與庫存
          - generic [aria-hidden] [ref=f39e27]: /
          - generic [ref=f39e28]: 編輯商品
        - generic [ref=f39e29]:
          - generic [ref=f39e30]:
            - generic [ref=f39e31]:
              - heading "編輯商品" [level=1] [ref=f39e32]
              - paragraph [ref=f39e33]: peccd46db212ae5d mobile axis remove zh-TW
            - link "全部商品" [ref=f39e35] [cursor=pointer]:
              - /url: /zh-TW/products?store=1e0b0784-40a2-48db-bca0-40ad5e444362
          - generic [ref=f39e36]:
            - complementary [ref=f39e37]:
              - navigation "完成度" [ref=f39e38]:
                - button "商品圖片" [ref=f39e39] [cursor=pointer]
                - button "基本資訊" [ref=f39e40] [cursor=pointer]
                - button "規格" [ref=f39e41] [cursor=pointer]
                - button "分類" [ref=f39e42] [cursor=pointer]
                - button "物流" [ref=f39e43] [cursor=pointer]
                - button "搜尋引擎最佳化" [ref=f39e44] [cursor=pointer]
              - button "完成度" [ref=f39e45] [cursor=pointer]
            - generic [ref=f39e46]:
              - generic [ref=f39e48]:
                - region [ref=f39e49]:
                  - heading "主圖 0/4" [level=2] [ref=f39e50]:
                    - text: 主圖
                    - generic [ref=f39e51]: 0/4
                  - paragraph [ref=f39e52]: 最多4張，第一張為封面。建議使用800px以上的正方形照片。
                  - paragraph [ref=f39e54]: 尚未新增圖片。
                  - generic [ref=f39e55] [cursor=pointer]:
                    - generic [ref=f39e56]: 新增圖片
                    - 'button "新增圖片: 主圖" [ref=f39e57]'
                - region [ref=f39e58]:
                  - heading "規格圖" [level=2] [ref=f39e59]
                  - paragraph [ref=f39e60]: 每個圖片規格值各一張；沒有規格圖時，買家會看到封面。
                  - generic [ref=f39e61]:
                    - generic [ref=f39e62]: 圖片規格
                    - combobox "圖片規格" [ref=f39e63]:
                      - option "使用第一個規格"
                      - option "顏色" [selected]
                      - option "尺寸"
                  - generic [ref=f39e64]:
                    - generic [ref=f39e65]:
                      - heading "White" [level=3] [ref=f39e66]
                      - paragraph [ref=f39e67]: 使用封面
                      - generic [ref=f39e68] [cursor=pointer]:
                        - generic [ref=f39e69]: 新增規格圖
                        - 'button "新增規格圖: White" [ref=f39e70]'
                    - generic [ref=f39e71]:
                      - heading "Black" [level=3] [ref=f39e72]
                      - paragraph [ref=f39e73]: 使用封面
                      - generic [ref=f39e74] [cursor=pointer]:
                        - generic [ref=f39e75]: 新增規格圖
                        - 'button "新增規格圖: Black" [ref=f39e76]'
                - region [ref=f39e77]:
                  - heading "詳情圖 0/20" [level=2] [ref=f39e78]:
                    - text: 詳情圖
                    - generic [ref=f39e79]: 0/20
                  - paragraph [ref=f39e80]: 最多20張，可上傳長圖，高度最多為寬度的6倍。
                  - paragraph [ref=f39e82]: 尚未新增圖片。
                  - generic [ref=f39e83] [cursor=pointer]:
                    - generic [ref=f39e84]: 新增圖片
                    - 'button "新增圖片: 詳情圖" [ref=f39e85]'
              - group [ref=f39e86]:
                - generic [ref=f39e87]:
                  - heading "基本資訊" [level=2] [ref=f39e88]
                  - generic [ref=f39e89]:
                    - text: 顯示狀態
                    - combobox "顯示狀態 草稿：只有你看得到，買家找不到也買不到。" [ref=f39e90]:
                      - option "草稿" [selected]
                      - option "上架中"
                    - generic [ref=f39e91]: 草稿：只有你看得到，買家找不到也買不到。
                  - generic [ref=f39e92]:
                    - text: 商品名稱
                    - textbox "商品名稱 41 / 120" [ref=f39e93]: peccd46db212ae5d mobile axis remove zh-TW
                    - generic [ref=f39e94]: 41 / 120
                  - generic [ref=f39e95]:
                    - text: 描述
                    - textbox "描述" [ref=f39e96]
                - region [ref=f39e97]:
                  - heading "規格" [level=2] [ref=f39e98]
                  - generic [ref=f39e100]:
                    - generic [ref=f39e101]:
                      - generic [ref=f39e102]: 規格名稱
                      - combobox "規格名稱" [ref=f39e103]: 尺寸
                    - generic [ref=f39e104]:
                      - generic [ref=f39e105]: 規格值
                      - textbox "規格值" [ref=f39e106]
                      - generic [ref=f39e107]: 按 Enter 或用逗號、換行分隔，最多 50 個不重複值。
                      - generic [ref=f39e109]:
                        - text: S
                        - button "移除 S" [ref=f39e110] [cursor=pointer]: ×
                    - button "移除 規格名稱 1" [ref=f39e111] [cursor=pointer]: 移除
                  - button "新增規格（顏色、尺寸…）" [ref=f39e112] [cursor=pointer]
                  - generic [ref=f39e113]:
                    - button "批量填入" [ref=f39e115] [cursor=pointer]
                    - paragraph [ref=f39e116]: "1 / 100 · 已選取: 0"
                  - region "規格" [ref=f39e117]:
                    - generic [aria-hidden] [ref=f39e118]:
                      - generic [ref=f39e119]: 規格
                      - generic [ref=f39e120]: 售價 (NT$)
                      - generic [ref=f39e121]: 原價
                      - generic [ref=f39e122]: 庫存
                      - generic [ref=f39e123]: 貨號
                      - generic [ref=f39e124]: 直播關鍵字
                      - generic [ref=f39e125]: 啟用
                    - generic [ref=f39e126]:
                      - generic "S" [ref=f39e127]:
                        - checkbox "已選取 S" [ref=f39e128]
                        - strong [ref=f39e129]: S
                      - generic [ref=f39e130]:
                        - generic [ref=f39e131]: 售價
                        - textbox "售價 S" [ref=f39e132]: "95"
                      - generic [ref=f39e133]:
                        - generic [ref=f39e134]: 原價
                        - textbox "原價 1" [ref=f39e135]: "145"
                      - generic [ref=f39e136]:
                        - generic "不追蹤 ∞" [ref=f39e137]:
                          - checkbox "不追蹤 ∞" [ref=f39e138]
                          - text: 不追蹤 ∞
                        - generic [ref=f39e139]:
                          - generic [ref=f39e140]: 數量
                          - textbox "數量 1" [ref=f39e141]: "9"
                      - generic [ref=f39e142]:
                        - generic [ref=f39e143]: 貨號
                        - textbox "貨號 1" [ref=f39e144]:
                          - /placeholder: 留空由系統產生
                          - text: M1525193075-A0-S
                      - generic [ref=f39e145]:
                        - generic [ref=f39e146]: 直播關鍵字
                        - textbox "直播關鍵字 1" [ref=f39e147]
                      - generic [ref=f39e148]:
                        - checkbox "啟用 1" [checked] [ref=f39e149]
                        - generic [ref=f39e150]: 啟用
                - status [ref=f39e151]: "即將封存的規格: 2"
                - generic [ref=f39e152]:
                  - heading "分類" [level=2] [ref=f39e153]
                  - generic [ref=f39e154]:
                    - text: 搜尋分類
                    - searchbox "搜尋分類" [ref=f39e155]
                  - generic [ref=f39e157]:
                    - checkbox "peccd46db212ae5d picks" [ref=f39e158]
                    - text: peccd46db212ae5d picks
                  - link "管理分類" [ref=f39e159] [cursor=pointer]:
                    - /url: /zh-TW/collections?store=1e0b0784-40a2-48db-bca0-40ad5e444362
                - group [ref=f39e160]:
                  - generic "+ 物流" [ref=f39e161] [cursor=pointer]
                - group [ref=f39e162]:
                  - generic "+ 搜尋引擎最佳化" [ref=f39e163] [cursor=pointer]
              - alert [ref=f39e165]:
                - paragraph [ref=f39e166]: 無法儲存，請檢查輸入後重試。
            - generic [ref=f39e167]:
              - generic [ref=f39e168]: 尚未儲存的修改
              - generic [ref=f39e169]:
                - button "儲存修改" [ref=f39e170] [cursor=pointer]
                - button "上架" [ref=f39e171] [cursor=pointer]
        - paragraph [ref=f39e173]: DaWan Live 由 香港大碗貿易有限公司 營運
  - alert [ref=f39e174]
```

# Test source

```ts
  781 |         await expect(white.getByTestId("matrix-active-0")).not.toBeChecked();
  782 |         const archiveStart = writes.length;
  783 |         const archivedResponse = page.waitForResponse((r) => r.url().includes(`/api/stores/${store}/products/`) && r.url().endsWith("/document") && r.request().method() !== "GET");
  784 |         await page.getByTestId("product-save").click();
  785 |         expect((await archivedResponse).ok()).toBe(true);
  786 |         await expect(page.getByTestId("product-save")).toBeEnabled();
  787 |         await page.reload();
  788 |         const savedMatrix = page.getByTestId("variant-matrix");
  789 |         await expect(savedMatrix.locator('[data-testid^="matrix-row-"]')).toHaveCount(1);
  790 |         await expect(savedMatrix.locator(`[data-sku-id="${whiteID}"]`)).toHaveCount(0);
  791 |         await expect(savedMatrix.locator("strong")).toHaveText(["Black / S"]);
  792 |         const savedBlack = savedMatrix.locator(`[data-sku-id="${blackID}"]`);
  793 |         await expect(savedBlack).toHaveCount(1);
  794 |         const savedPrice = savedBlack.getByTestId("new-price-0");
  795 |         await savedPrice.scrollIntoViewIfNeeded();
  796 |         await expect(savedPrice).toBeVisible();
  797 |         await expect(savedPrice).toBeInViewport({ ratio: 0.99 });
  798 |         await expect(savedPrice).toHaveValue("81");
  799 |         await expect(savedBlack.getByTestId("matrix-compare-0")).toHaveValue("121");
  800 |         await expect(savedBlack.getByTestId("matrix-quantity-0")).toHaveValue("8");
  801 |         await expect(savedBlack.getByRole("checkbox", { name: c.untracked, exact: true })).not.toBeChecked();
  802 |         await expect(savedBlack.getByTestId("matrix-code-0")).toHaveValue(`${mobileMatrixTag}-${localeIndex}-1`);
  803 |         await expect(savedBlack.getByTestId("matrix-code-0")).toBeDisabled();
  804 |         await expect(savedBlack.getByRole("textbox", { name: `${c.keyword} 1`, exact: true })).toHaveValue(`${mobileMatrixTag}${localeIndex}E1`);
  805 |         await expect(savedBlack.getByTestId("matrix-active-0")).toBeChecked();
  806 |         expect(writes.slice(archiveStart).filter((r) => r.path.endsWith("/document"))).toHaveLength(1);
  807 |         const archiveCapture = `mobile-matrix-${locale}-390-archived`;
  808 |         await shot(archiveCapture);
  809 |         ledger.push({ page: "mobile matrix edit", locale, width: 390, control: "matrix-active-0/product-save", action: "uncheck White, click save, reload", expected: { archivedSKU: whiteID, white: "absent from active-only matrix by SKU ID and name", visibleRows: 1, black: { id: blackID, price: "81", compare: "121", trackedQuantity: "8", active: true } }, actual: "PASS", screenshot: `${archiveCapture}.png`, tier: "BROWSER+REAL_PG" });
  810 |
  811 |         // A separate product preserves every preceding create/edit/archive
  812 |         // proof while exercising axis removal on persisted mobile SKUs.
  813 |         await page.goto(url(locale, "products/new"));
  814 |         await page.getByTestId("product-name").fill(`${tag} mobile axis remove ${locale}`);
  815 |         for (const [i, name, values] of [[0, c.color, "White, Black"], [1, c.size, "S"]] as const) {
  816 |           await page.getByTestId("axis-add").click();
  817 |           await page.getByTestId(`axis-name-${i}`).fill(name);
  818 |           await page.getByTestId(`axis-values-${i}`).fill(values);
  819 |           await page.getByTestId(`axis-values-${i}`).press("Enter");
  820 |         }
  821 |         await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(2);
  822 |         for (let i = 0; i < 2; i++) {
  823 |           const row = page.getByTestId(`matrix-row-${i}`);
  824 |           await row.getByTestId(`new-price-${i}`).fill(String(90 + i));
  825 |           await row.getByTestId(`matrix-compare-${i}`).fill(String(130 + i));
  826 |           await row.getByTestId(`matrix-quantity-${i}`).fill(String(3 + i));
  827 |           await row.getByTestId(`matrix-code-${i}`).fill(`${mobileMatrixTag}-A${localeIndex}-${i}`);
  828 |         }
  829 |         const axisCreateStart = writes.length;
  830 |         const axisCreated = page.waitForResponse((r) => r.url().includes(`/api/stores/${store}/products/`) && r.url().endsWith("/document") && r.request().method() !== "GET");
  831 |         await page.getByTestId("product-create").click();
  832 |         expect((await axisCreated).ok()).toBe(true);
  833 |         const axisResult = page.getByTestId("product-save-result");
  834 |         await expect(axisResult).toBeVisible();
  835 |         await axisResult.getByRole("link", { name: c.save, exact: true }).click();
  836 |         await expect(page.getByTestId("product-save")).toBeEnabled();
  837 |         await page.reload();
  838 |         const axisMatrix = page.getByTestId("variant-matrix");
  839 |         await expect(axisMatrix.locator('[data-testid^="matrix-row-"]')).toHaveCount(2);
  840 |         await expect(axisMatrix.locator("strong")).toHaveText(["White / S", "Black / S"]);
  841 |         const oldAxisIDs: string[] = [];
  842 |         for (let i = 0; i < 2; i++) {
  843 |           const row = page.getByTestId(`matrix-row-${i}`);
  844 |           const skuID = await row.getAttribute("data-sku-id");
  845 |           expect(skuID).toMatch(/^[0-9a-f-]{36}$/);
  846 |           oldAxisIDs.push(skuID!);
  847 |           await expect(row.getByTestId(`new-price-${i}`)).toHaveValue(String(90 + i));
  848 |           await expect(row.getByTestId(`matrix-compare-${i}`)).toHaveValue(String(130 + i));
  849 |           await expect(row.getByTestId(`matrix-quantity-${i}`)).toHaveValue(String(3 + i));
  850 |           await expect(row.getByTestId(`matrix-code-${i}`)).toHaveValue(`${mobileMatrixTag}-A${localeIndex}-${i}`);
  851 |         }
  852 |         expect(new Set(oldAxisIDs).size).toBe(2);
  853 |         expect(writes.slice(axisCreateStart).filter((r) => r.path.endsWith("/document"))).toHaveLength(1);
  854 |         await expect(page.locator(".product-axis")).toHaveCount(2);
  855 |         const removeColor = page.locator(".product-axis").nth(0).locator(".pe-axis-remove");
  856 |         await expect(removeColor).toHaveAccessibleName(`${c.remove} ${c.axis} 1`);
  857 |         await expect(removeColor).toBeEnabled();
  858 |         await removeColor.scrollIntoViewIfNeeded();
  859 |         await expect(removeColor).toBeInViewport({ ratio: 0.99 });
  860 |         const axisRemoveStart = writes.length;
  861 |         await removeColor.click();
  862 |         await expect(page.locator(".product-axis")).toHaveCount(1);
  863 |         await expect(page.getByTestId("axis-name-0")).toHaveValue(c.size);
  864 |         await expect(page.locator(".product-axis").locator(".pe-value-chips > span")).toHaveCount(1);
  865 |         await expect(page.locator(".product-axis").locator(".pe-value-chips > span")).toContainText(["S"]);
  866 |         await expect(axisMatrix.locator('[data-testid^="matrix-row-"]')).toHaveCount(1);
  867 |         await expect(axisMatrix.locator("strong")).toHaveText(["S"]);
  868 |         expect(writes.length).toBe(axisRemoveStart);
  869 |         ledger.push({ page: "mobile axis remove", locale, width: 390, control: ".pe-axis-remove (Color axis 1)", action: "real click", expected: { axes: [c.size], values: ["S"], visibleRows: "2→1", row: "S" }, actual: "PASS", persistence: "local draft until UI save below" });
  870 |         const remaining = page.getByTestId("matrix-row-0");
  871 |         await remaining.getByTestId("new-price-0").fill("95");
  872 |         await remaining.getByTestId("matrix-compare-0").fill("145");
  873 |         await remaining.getByTestId("matrix-quantity-0").fill("9");
  874 |         await remaining.getByTestId("matrix-code-0").fill(`${mobileMatrixTag}-A${localeIndex}-S`);
  875 |         await expect(remaining.getByRole("checkbox", { name: c.untracked, exact: true })).not.toBeChecked();
  876 |         await expect(remaining.getByTestId("matrix-active-0")).toBeChecked();
  877 |         const axisSaved = page.waitForResponse((r) => r.url().includes(`/api/stores/${store}/products/`) && r.url().endsWith("/document") && r.request().method() !== "GET");
  878 |         const axisConfirmation = page.waitForEvent("dialog");
  879 |         await page.getByTestId("product-save").click();
  880 |         expect((await axisConfirmation).message()).toBe(c.archiveRows);
> 881 |         expect((await axisSaved).ok()).toBe(true);
      |                                        ^ Error: expect(received).toBe(expected) // Object.is equality
  882 |         await expect(page.getByTestId("product-save")).toBeEnabled();
  883 |         await page.reload();
  884 |         await expect(page.locator(".product-axis")).toHaveCount(1);
  885 |         await expect(page.getByTestId("axis-name-0")).toHaveValue(c.size);
  886 |         await expect(page.locator(".product-axis").locator(".pe-value-chips > span")).toHaveCount(1);
  887 |         await expect(page.locator(".product-axis").locator(".pe-value-chips > span")).toContainText(["S"]);
  888 |         await expect(axisMatrix.locator('[data-testid^="matrix-row-"]')).toHaveCount(1);
  889 |         await expect(axisMatrix.locator("strong")).toHaveText(["S"]);
  890 |         for (const oldID of oldAxisIDs) await expect(axisMatrix.locator(`[data-sku-id="${oldID}"]`)).toHaveCount(0);
  891 |         const newAxisID = await remaining.getAttribute("data-sku-id");
  892 |         expect(newAxisID).toMatch(/^[0-9a-f-]{36}$/);
  893 |         expect(oldAxisIDs).not.toContain(newAxisID);
  894 |         await expect(remaining.getByTestId("new-price-0")).toHaveValue("95");
  895 |         await expect(remaining.getByTestId("matrix-compare-0")).toHaveValue("145");
  896 |         await expect(remaining.getByTestId("matrix-quantity-0")).toHaveValue("9");
  897 |         await expect(remaining.getByTestId("matrix-code-0")).toHaveValue(`${mobileMatrixTag}-A${localeIndex}-S`);
  898 |         await expect(remaining.getByTestId("matrix-code-0")).toBeDisabled();
  899 |         await expect(remaining.getByRole("checkbox", { name: c.untracked, exact: true })).not.toBeChecked();
  900 |         await expect(remaining.getByTestId("matrix-active-0")).toBeChecked();
  901 |         expect(writes.slice(axisRemoveStart).filter((r) => r.path.endsWith("/document"))).toHaveLength(1);
  902 |         const removeCapture = `mobile-axis-remove-${locale}-390-saved`;
  903 |         await remaining.getByTestId("new-price-0").scrollIntoViewIfNeeded();
  904 |         await shot(removeCapture);
  905 |         ledger.push({ page: "mobile axis remove", locale, width: 390, control: "product-save", action: "click, confirm archive, reload", expected: { axes: [c.size], values: ["S"], archivedSKUs: oldAxisIDs, activeSKU: newAxisID, row: "S", price: "95", compare: "145", trackedQuantity: "9", oneDocumentWrite: true }, actual: "PASS", screenshot: `${removeCapture}.png`, tier: "BROWSER+REAL_PG" });
  906 |       }
  907 |       // Controlled read-only ambiguity: do not fabricate a stock quantity when the API has no single warehouse.
  908 |       await page.route(
  909 |         `**/api/stores/${store}/products/${id}`,
  910 |         async (route) => {
  911 |           const res = await route.fetch();
  912 |           const body = await res.json();
  913 |           body.warehouse_id = null;
  914 |           for (const s of body.skus) {
  915 |             s.on_hand = null;
  916 |             s.committed = null;
  917 |           }
  918 |           await route.fulfill({ response: res, json: body });
  919 |         },
  920 |       );
  921 |       await page.goto(url("en", "products/" + id));
  922 |       await expect(page.getByTestId("product-quantity")).toBeDisabled();
  923 |       await expect(
  924 |         page.getByRole("link", { name: "Open inventory", exact: true }),
  925 |       ).toBeVisible();
  926 |       await page.unroute(`**/api/stores/${store}/products/${id}`);
  927 |       ledger.push({
  928 |         page: "edit",
  929 |         control: "ambiguous warehouse",
  930 |         action: "read-only response MOCK + visible disabled field",
  931 |         expected: "no available fallback, disabled target and inventory link",
  932 |         actual: "PASS (MOCK read contract)",
  933 |       });
  934 |       // The PG PE23 gate exercises the actual open-window rule. This isolated
  935 |       // response mock proves every locale explains that rule after a real click.
  936 |       const documentPattern = `**/api/stores/${store}/products/${id}/document`;
  937 |       await page.route(documentPattern, (route) =>
  938 |         route.fulfill({ status: 409, json: { code: "live_window_open" } }),
  939 |       );
  940 |       for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
  941 |         await page.goto(url(locale, "products/" + id));
  942 |         await page.getByTestId("product-keyword").fill("BLOCKED-LIVE");
  943 |         await page.getByTestId("product-save").click();
  944 |         await expect(
  945 |           page.getByText(productEditorCopy[locale].live_window_open, {
  946 |             exact: true,
  947 |           }),
  948 |         ).toBeVisible();
  949 |         ledger.push({
  950 |           page: "edit",
  951 |           locale,
  952 |           control: "live-window keyword refusal",
  953 |           action: "click + MOCK 409",
  954 |           expected: "explicit translated safety refusal",
  955 |           actual: "PASS (MOCK response; real rule covered by PG PE21)",
  956 |         });
  957 |       }
  958 |       await page.unroute(documentPattern);
  959 |       await page.goto(url("en", "products"));
  960 |       // Fault injection: server commits a copy, but the browser loses that response.
  961 |       // After reload the receipt fence must prevent sending a fresh-key duplicate.
  962 |       const copyPattern = `**/products/${id}/copy`;
  963 |       await page.route(copyPattern, async (route) => {
  964 |         await route.fetch();
  965 |         await route.abort("failed");
  966 |       });
  967 |       const beforeUnknown = writes.length;
  968 |       await page
  969 |         .getByTestId(`product-row-${id}`)
  970 |         .getByRole("button", { name: "Copy", exact: true })
  971 |         .click();
  972 |       await expect(
  973 |         page.getByRole("button", {
  974 |           name: productEditorCopy.en.retry,
  975 |           exact: true,
  976 |         }),
  977 |       ).toBeVisible();
  978 |       await page.reload();
  979 |       await expect(
  980 |         page.getByText(productEditorCopy.en.listRecoveryRequired, {
  981 |           exact: true,
```
