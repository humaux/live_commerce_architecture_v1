# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: catalog-core.spec.ts >> PE12-17 document workflow, matrix, list actions and click ledger
- Location: tests/admin/product-editor.acceptance.ts:16:3

# Error details

```
Error: zh-TW new White / S: visible 售價

expect(locator).toBeVisible() failed

Locator:  getByTestId('matrix-row-0').getByTestId('new-price-0').locator('..').locator(':scope > span')
Expected: visible
Received: hidden
Timeout:  10000ms

Call log:
  - zh-TW new White / S: visible 售價 getByTestId('matrix-row-0').getByTestId('new-price-0').locator('..').locator(':scope > span') with timeout 10000ms
  - waiting for getByTestId('matrix-row-0').getByTestId('new-price-0').locator('..').locator(':scope > span')
    24 × locator resolved to <span>售價</span>
       - unexpected value "hidden"

```

```yaml
- link "跳至內容":
  - /url: "#main"
- banner:
  - button "開啟導覽"
  - text: 切換商店
  - combobox "切換商店":
    - option "fixture-store-b" [selected]
  - text: 語言
  - combobox "語言":
    - option "简体中文"
    - option "繁體中文" [selected]
    - option "English"
  - group: 說明
  - group: 帳號
- main:
  - navigation "工作區導覽":
    - link "總覽":
      - /url: /zh-TW/
    - text: 商品與庫存 新增商品
  - heading "新增商品" [level=1]
  - link "全部商品":
    - /url: /zh-TW/products?store=bf44fa03-6c42-4092-92a7-372b43f682c0
  - complementary:
    - navigation "完成度":
      - button "商品圖片"
      - button "基本資訊"
      - button "規格"
      - button "分類"
      - button "物流"
      - button "搜尋引擎最佳化"
    - button "完成度"
  - region "商品圖片":
    - region "主圖 0/4":
      - heading "主圖 0/4" [level=2]
      - paragraph: 最多4張，第一張為封面。建議使用800px以上的正方形照片。
      - paragraph: 尚未新增圖片。
      - text: 新增圖片
      - 'button "新增圖片: 主圖"'
    - region "規格圖":
      - heading "規格圖" [level=2]
      - paragraph: 每個圖片規格值各一張；沒有規格圖時，買家會看到封面。
      - text: 圖片規格
      - combobox "圖片規格":
        - option "使用第一個規格" [selected]
        - option "顏色"
        - option "尺寸"
      - heading "White" [level=3]
      - paragraph: 使用封面
      - text: 新增規格圖
      - 'button "新增規格圖: White"'
      - heading "Black" [level=3]
      - paragraph: 使用封面
      - text: 新增規格圖
      - 'button "新增規格圖: Black"'
    - region "詳情圖 0/20":
      - heading "詳情圖 0/20" [level=2]
      - paragraph: 最多20張，可上傳長圖，高度最多為寬度的6倍。
      - paragraph: 尚未新增圖片。
      - text: 新增圖片
      - 'button "新增圖片: 詳情圖"'
  - group:
    - heading "基本資訊" [level=2]
    - text: 商品名稱
    - textbox "商品名稱 36 / 120": peccb37a8509fd47 mobile matrix zh-TW
    - text: 36 / 120 描述
    - textbox "描述"
    - region "規格":
      - heading "規格" [level=2]
      - text: 規格名稱
      - combobox "規格名稱": 顏色
      - text: 規格值
      - textbox "規格值"
      - text: 按 Enter 或用逗號、換行分隔，最多 50 個不重複值。 White
      - button "移除 White": ×
      - text: Black
      - button "移除 Black": ×
      - button "移除 規格名稱 1": 移除
      - text: 規格名稱
      - combobox "規格名稱": 尺寸
      - text: 規格值
      - textbox "規格值"
      - text: 按 Enter 或用逗號、換行分隔，最多 50 個不重複值。 S
      - button "移除 S": ×
      - button "移除 規格名稱 2": 移除
      - button "新增規格（顏色、尺寸…）"
      - button "批量填入"
      - paragraph: "2 / 100 · 已選取: 0"
      - region "規格":
        - checkbox "已選取 White / S"
        - strong: White / S
        - textbox "售價 White / S"
        - textbox "原價 1"
        - checkbox "不追蹤 ∞"
        - text: 不追蹤 ∞
        - textbox "數量 1"
        - textbox "貨號 1":
          - /placeholder: 留空由系統產生
        - textbox "直播關鍵字 1"
        - checkbox "啟用 1" [checked]
        - checkbox "已選取 Black / S"
        - strong: Black / S
        - textbox "售價 Black / S"
        - textbox "原價 2"
        - checkbox "不追蹤 ∞"
        - text: 不追蹤 ∞
        - textbox "數量 2"
        - textbox "貨號 2":
          - /placeholder: 留空由系統產生
        - textbox "直播關鍵字 2"
        - checkbox "啟用 2" [checked]
    - heading "分類" [level=2]
    - text: 搜尋分類
    - searchbox "搜尋分類"
    - checkbox "peccb37a8509fd47 picks"
    - text: peccb37a8509fd47 picks
    - link "管理分類":
      - /url: /zh-TW/collections?store=bf44fa03-6c42-4092-92a7-372b43f682c0
    - group: + 物流
    - group: + 搜尋引擎最佳化
  - text: 尚未儲存的修改
  - button "儲存草稿"
  - button "上架"
  - paragraph: DaWan Live 由 香港大碗貿易有限公司 營運
- alert
```

# Test source

```ts
  611 |       await single.getByTestId("quick-stock").click();
  612 |       await expect(page.getByTestId("quick-target-0")).toHaveValue("5");
  613 |       await expect(page.getByTestId("quick-delta-0")).toHaveValue("0");
  614 |       await page.getByTestId("quick-delta-0").fill("2");
  615 |       await expect(page.getByTestId("quick-target-0")).toHaveValue("7");
  616 |       await page.getByTestId("quick-save").click();
  617 |       await expect(page.getByTestId("product-quick-edit")).toHaveCount(0);
  618 |       await single.getByTestId("quick-stock").click();
  619 |       await page.getByTestId("quick-target-0").fill("0");
  620 |       await expect(page.getByTestId("quick-delta-0")).toHaveValue("-7");
  621 |       await page.getByTestId("quick-save").click();
  622 |       await expect(page.getByTestId("product-quick-edit")).toHaveCount(0);
  623 |       await page.reload();
  624 |       await single.getByTestId("quick-stock").click();
  625 |       await expect(page.getByTestId("quick-target-0")).toHaveValue("0");
  626 |       await page.keyboard.press("Escape");
  627 |       await expect(single.getByTestId("quick-stock")).toBeFocused();
  628 |       const matrixId = /products\/([0-9a-f-]{36})/.exec(matrixHref!)![1];
  629 |       const matrix = page.getByTestId(`product-row-${matrixId}`);
  630 |       await matrix.getByTestId("quick-price").click();
  631 |       await expect(page.locator('[data-testid^="quick-sku-"]')).toHaveCount(12);
  632 |       await page.getByTestId("quick-price-1").fill("81");
  633 |       await page.getByTestId("quick-save").click();
  634 |       await expect(page.getByTestId("product-quick-edit")).toHaveCount(0);
  635 |       await page.reload();
  636 |       await matrix.getByTestId("quick-price").click();
  637 |       await expect(page.getByTestId("quick-price-1")).toHaveValue("81");
  638 |       await page.keyboard.press("Escape");
  639 |       ledger.push({
  640 |         page: "list",
  641 |         control: "single/multi SKU pencils, inventory delta and zero",
  642 |         action: "click fill Enter Escape reload",
  643 |         expected:
  644 |           "one patch per save, stock0 persists, twelve-SKU popover and focus return",
  645 |         actual: "PASS",
  646 |       });
  647 |       // Keep additional products after the frozen list-count checks. This uses
  648 |       // the same isolated PG fixture; every write goes through a real UI save.
  649 |       await page.setViewportSize({ width: 390, height: 844 });
  650 |       const mobileMatrixTag = `M${Date.now().toString().slice(-10)}`;
  651 |       for (const [localeIndex, locale] of (["zh-TW", "zh-CN", "en"] as const).entries()) {
  652 |         const c = productEditorCopy[locale];
  653 |         await page.goto(url(locale, "products/new"));
  654 |         await page.getByTestId("product-name").fill(`${tag} mobile matrix ${locale}`);
  655 |         const axesStart = writes.length;
  656 |         for (const [i, name, values] of [[0, c.color, "White, Black"], [1, c.size, "S"]] as const) {
  657 |           await page.getByTestId("axis-add").click();
  658 |           await page.getByTestId(`axis-name-${i}`).fill(name);
  659 |           await page.getByTestId(`axis-values-${i}`).fill(values);
  660 |           await page.getByTestId(`axis-values-${i}`).press("Enter");
  661 |           await expect(page.getByTestId(`axis-name-${i}`)).toHaveValue(name);
  662 |           await expect(page.locator(".product-axis").nth(i).locator(".pe-value-chips > span"))
  663 |             .toHaveCount(i === 0 ? 2 : 1);
  664 |           ledger.push({ page: "mobile matrix new", locale, width: 390, control: `axis-add/axis-name-${i}/axis-values-${i}`, action: "click fill Enter", expected: { name, values }, actual: "PASS", persistence: "local draft until UI save" });
  665 |         }
  666 |         await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(2);
  667 |         expect(writes.length).toBe(axesStart);
  668 |         for (const editing of [false, true]) {
  669 |           const phase = editing ? "edit" : "new";
  670 |           const rows = ["White / S", "Black / S"].map((name, i) => ({
  671 |             name,
  672 |             price: String((editing ? 80 : 60) + i),
  673 |             compare: String((editing ? 120 : 100) + i),
  674 |             quantity: String((editing ? 7 : 5) + i),
  675 |             untracked: (i === 1) !== editing,
  676 |             code: `${mobileMatrixTag}-${localeIndex}-${i}`,
  677 |             keyword: `${mobileMatrixTag}${localeIndex}${editing ? "E" : "C"}${i}`,
  678 |           }));
  679 |           const fieldsFor = (i: number, existing: boolean) => {
  680 |             const row = page.getByTestId(`matrix-row-${i}`), state = rows[i];
  681 |             return [
  682 |               { control: row.getByTestId(`new-price-${i}`), label: c.price, value: state.price },
  683 |               { control: row.getByTestId(`matrix-compare-${i}`), label: c.compare, value: state.compare },
  684 |               { control: row.getByTestId(`matrix-quantity-${i}`), label: state.untracked ? c.max : existing ? c.targetQty : c.quantity, value: state.quantity },
  685 |               { control: row.getByTestId(`matrix-code-${i}`), label: c.code, value: state.code },
  686 |               { control: row.getByRole("textbox", { name: `${c.keyword} ${i + 1}`, exact: true }), label: c.keyword, value: state.keyword },
  687 |             ];
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
> 711 |               await expect(label, `${locale} ${phase} ${state.name}: visible ${field.label}`).toBeVisible();
      |                                                                                               ^ Error: zh-TW new White / S: visible 售價
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
  744 |           for (const [i, state] of rows.entries()) {
  745 |             const row = page.getByTestId(`matrix-row-${i}`);
  746 |             await expect(row.locator("strong")).toHaveText(state.name);
  747 |             for (const field of fieldsFor(i, true)) {
  748 |               const label = field.control.locator("..").locator(":scope > span");
  749 |               await expect(label).toBeVisible();
  750 |               await expect(label).toHaveText(field.label);
  751 |               await label.scrollIntoViewIfNeeded();
  752 |               await expect(label).toBeInViewport({ ratio: 0.99 });
  753 |               await expect(field.control).toHaveValue(field.value);
  754 |             }
  755 |             await expect(row.getByRole("checkbox", { name: c.untracked, exact: true })).toBeChecked({ checked: state.untracked });
  756 |             await expect(row.getByTestId(`matrix-active-${i}`)).toBeChecked();
  757 |             await expect(row.getByTestId(`matrix-code-${i}`)).toBeDisabled();
  758 |             ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, row: state.name, control: "saved row readback", action: "UI save, editor reopen/reload, read visible fields", expected: state, actual: "PASS", tier: "BROWSER+REAL_PG" });
  759 |           }
  760 |           expect(writes.slice(saveStart).filter((r) => r.path.endsWith("/document"))).toHaveLength(1);
  761 |           const capture = `mobile-matrix-${locale}-390-${phase}`;
  762 |           await page.getByTestId("new-price-0").scrollIntoViewIfNeeded();
  763 |           await shot(capture);
  764 |           ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, control: editing ? "product-save" : "product-create", action: "click reload", expected: "one document write; both matrix rows persist", actual: "PASS", screenshot: `${capture}.png`, tier: "BROWSER+REAL_PG" });
  765 |         }
  766 |       }
  767 |       // Controlled read-only ambiguity: do not fabricate a stock quantity when the API has no single warehouse.
  768 |       await page.route(
  769 |         `**/api/stores/${store}/products/${id}`,
  770 |         async (route) => {
  771 |           const res = await route.fetch();
  772 |           const body = await res.json();
  773 |           body.warehouse_id = null;
  774 |           for (const s of body.skus) {
  775 |             s.on_hand = null;
  776 |             s.committed = null;
  777 |           }
  778 |           await route.fulfill({ response: res, json: body });
  779 |         },
  780 |       );
  781 |       await page.goto(url("en", "products/" + id));
  782 |       await expect(page.getByTestId("product-quantity")).toBeDisabled();
  783 |       await expect(
  784 |         page.getByRole("link", { name: "Open inventory", exact: true }),
  785 |       ).toBeVisible();
  786 |       await page.unroute(`**/api/stores/${store}/products/${id}`);
  787 |       ledger.push({
  788 |         page: "edit",
  789 |         control: "ambiguous warehouse",
  790 |         action: "read-only response MOCK + visible disabled field",
  791 |         expected: "no available fallback, disabled target and inventory link",
  792 |         actual: "PASS (MOCK read contract)",
  793 |       });
  794 |       // The PG PE23 gate exercises the actual open-window rule. This isolated
  795 |       // response mock proves every locale explains that rule after a real click.
  796 |       const documentPattern = `**/api/stores/${store}/products/${id}/document`;
  797 |       await page.route(documentPattern, (route) =>
  798 |         route.fulfill({ status: 409, json: { code: "live_window_open" } }),
  799 |       );
  800 |       for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
  801 |         await page.goto(url(locale, "products/" + id));
  802 |         await page.getByTestId("product-keyword").fill("BLOCKED-LIVE");
  803 |         await page.getByTestId("product-save").click();
  804 |         await expect(
  805 |           page.getByText(productEditorCopy[locale].live_window_open, {
  806 |             exact: true,
  807 |           }),
  808 |         ).toBeVisible();
  809 |         ledger.push({
  810 |           page: "edit",
  811 |           locale,
```