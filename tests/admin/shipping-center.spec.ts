// Purpose: PARITY-SHIPCENTER signed-frame acceptance on the real shipping route, with a per-control click ledger.
// Depends on: @playwright/test, node:fs/promises, node:path and the merchant-orders-ui signed-session harness.
// Used by: orders-ui.spec.ts registers this card in the existing GitHub browser gate; not a standalone fixture.
// Invariants: only synthetic read DTOs and the OS print dialog are mocked; navigation and controls use real clicks;
//   no business write may escape, selections persist in the URL, and parity images never come from ui-gallery.
import { expect, test, type Page, type TestInfo } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const copies = {
  "zh-TW": {
    center: "出貨中心", backOrders: "返回訂單", note: "列印不購面單、不登記出貨。",
    review: "核對選取訂單", back: "返回選取", edit: "修改選取", confirm: "確認並預覽",
    preview: "列印預覽", print: "列印", done: "完成列印", view: "檢視", cancel: "取消",
    backPreview: "返回列印預覽", backCenter: "返回出貨中心", recipient: "收件人", delivery: "配送方式",
    manual: "商家自行安排配送", order: "訂單", empty: "尚未選取訂單", buyer: "示範買家",
    boundary: "列印不證明交寄、不登記出貨、不購面單。", outcome: "取消列印不改出貨狀態；列印不證明交寄。",
    mobile: "批次列印請用桌面。", ready: "待出貨", needsReview: "需覆核",
    selected: (n: number) => `已選 ${n} 筆`, items: ["帆布托特包·焦糖", "棉質上衣·米白", "帆布托特包·靛藍"],
  },
  "zh-CN": {
    center: "发货中心", backOrders: "返回订单", note: "打印不购买面单，也不登记发货。",
    review: "核对所选订单", back: "返回选择订单", edit: "修改所选订单", confirm: "确认并预览",
    preview: "打印预览", print: "打印", done: "完成打印", view: "查看", cancel: "取消",
    backPreview: "返回打印预览", backCenter: "返回发货中心", recipient: "收件人", delivery: "配送方式",
    manual: "商家自行安排配送", order: "订单", empty: "尚未选择订单", buyer: "示例买家",
    boundary: "打印不代表物流已揽收，不登记发货，也不购买面单。", outcome: "取消打印不改变发货状态；打印不代表物流已揽收。",
    mobile: "批量打印请使用电脑。", ready: "待发货", needsReview: "需复核",
    selected: (n: number) => `已选 ${n} 笔`, items: ["帆布托特包·焦糖色", "棉质上衣·米白色", "帆布托特包·靛蓝色"],
  },
  en: {
    center: "Shipping center", backOrders: "Back to orders", note: "Printing does not buy a label or record shipment.",
    review: "Review selected orders", back: "Back to selection", edit: "Edit selection", confirm: "Confirm and preview",
    preview: "Print preview", print: "Print", done: "Complete printing", view: "View", cancel: "Cancel",
    backPreview: "Back to print preview", backCenter: "Back to shipping center", recipient: "Recipient", delivery: "Delivery method",
    manual: "Seller-arranged delivery", order: "Order", empty: "No orders selected", buyer: "Sample buyer",
    boundary: "Printing does not prove carrier handover, record shipment or buy a label.", outcome: "Canceling print keeps shipment status; printing does not prove handover.",
    mobile: "Use desktop for batch printing.", ready: "Ready to ship", needsReview: "Review needed",
    selected: (n: number) => `${n} selected`, items: ["Canvas tote · Caramel", "Cotton top · Off-white", "Canvas tote · Indigo"],
  },
};
type Locale = keyof typeof copies;
const numbers = ["10482", "10477", "10473"];
const orderIDs = numbers.map(n => `11111111-1111-4111-8111-1111111${n}`);
const combinations = [
  { name: "first", indices: [0], frames: ["28127", "28353", "28407", "28471"] },
  { name: "first-third", indices: [0, 2], frames: ["28220", "28518", "28575", "28652"] },
  { name: "review", indices: [1], frames: ["28700", "28793", "28847", "28911"] },
  { name: "third", indices: [2], frames: ["28958", "29051", "29105", "29169"] },
  { name: "first-review", indices: [0, 1], frames: ["29216", "29309", "29366", "29443"] },
  { name: "review-third", indices: [1, 2], frames: ["29491", "29584", "29641", "29718"] },
  { name: "all", indices: [0, 1, 2], frames: ["29766", "29859", "29919", "30009"] },
];

// MOCK DTO fixture preparation only. No production API/database changes and no fabricated fields in the product.
function fixture(locale: Locale) {
  return orderIDs.map((id, i) => {
    const quantity = i === 1 ? 2 : 1;
    const summary = {
      // Match merchantorders.timestampLayout: the production DTO parser requires six UTC fractional digits.
      order_id: id, created_at: "2026-10-29T01:00:00.000000Z", updated_at: "2026-10-29T01:00:00.000000Z",
      currency: "TWD", total_minor: 128000, commercial_state: "CONFIRMED", fulfillment_state: "MANUAL_UNASSIGNED",
      payment_state: i === 1 ? "REVIEW_REQUIRED" : "CAPTURED", test_mode: true,
      work_state: i === 1 ? "REVIEW_REQUIRED" : "READY", refunded_minor: 0, refund_pending_minor: 0,
      source: "storefront", pickup_source: null, payment_mode: "card", collection_state: null,
      cod_surcharge_minor: null, cod_collect_minor: null,
    };
    return {
      summary: { ...summary, order_number: `LC-${id.replaceAll("-", "").toUpperCase()}`, recipient_masked: "示***", delivery_kind: "home", live_sessions: [] },
      detail: { ...summary, country: "TW", service_code: "manual_home",
        items: [{ sku_id: `22222222-2222-4222-8222-2222222${numbers[i]}`, code: `SYNTHETIC-${i}`, name: copies[locale].items[i], quantity,
          unit_price_minor: 128000 / quantity, amount: { subtotal_minor: 128000, discount_minor: 0, tax_minor: 0, total_minor: 128000 } }],
        totals: { subtotal_minor: 128000, discount_minor: 0, shipping_minor: 0, shipping_tax_minor: 0, tax_minor: 0, total_minor: 128000 },
        destination: { kind: "home", country: "TW", recipient_name: copies[locale].buyer, phone: "+886900000001",
          home_address: { region: "", city: "Synthetic city", postal_code: "000", line1: "Synthetic address", line2: "" }, pickup: null }, shipment: null },
    };
  });
}

async function mockReads(page: Page, store: string, locale: Locale, rows = fixture(locale), pageSize = rows.length) {
  const writes: string[] = [];
  await page.route(url => url.pathname.startsWith("/api/"), async route => {
    const request = route.request(), url = new URL(request.url());
    if (!["GET", "HEAD"].includes(request.method())) {
      // A bug must not turn this acceptance test into a real business operation.
      writes.push(`${request.method()} ${url.pathname}`);
      await route.fulfill({ status: 409, json: { code: "synthetic_write_forbidden" } });
      return;
    }
    const headers = { "cache-control": "private, no-store" };
    if (url.pathname === "/api/stores") {
      const response = await route.fetch();
      const data = await response.json();
      await route.fulfill({ response, json: { ...data, items: data.items.map((s: { id: string }) => s.id === store ? { ...s, name: "大夢甄選" } : s) } });
    } else if (url.pathname === `/api/stores/${store}/orders`) {
      const counts = { all: rows.length, unpaid: 0, transfer_review: 0, ready_to_ship: rows.filter(r => r.summary.work_state === "READY").length, ready_to_consign: 0, shipped: 0, completed: 0, cancelled: 0 };
      const offset = url.searchParams.has("cursor") ? pageSize : 0;
      const items = rows.slice(offset, offset + pageSize).map(r => r.summary);
      const next_cursor = offset + pageSize < rows.length ? "shipping-page-2" : "";
      await route.fulfill({ headers, json: url.searchParams.get("view") === "v2" ? { items, next_cursor, total: rows.length, counts, sessions: [] } : { items: items.map(({ order_number, recipient_masked, delivery_kind, live_sessions, ...base }) => base), next_cursor } });
    } else {
      const i = rows.findIndex(row => url.pathname === `/api/stores/${store}/orders/${row.summary.order_id}`);
      if (i >= 0) await route.fulfill({ headers, json: rows[i].detail });
      else await route.continue();
    }
  });
  // Only replace the OS modal. All application handlers and print stylesheet remain real.
  await page.addInitScript(() => {
    (window as unknown as { shippingPrintCalls: number }).shippingPrintCalls = 0;
    window.print = () => { (window as unknown as { shippingPrintCalls: number }).shippingPrintCalls++; };
  });
  return { rows, writes };
}

const content = (page: Page) => page.getByTestId("shipping-center");
const select = (page: Page, i: number) => content(page).getByRole("checkbox", { name: `#${numbers[i]}`, exact: true });
const button = (page: Page, name: string) => content(page).getByRole("button", { name, exact: true });

async function screenshot(page: Page, info: TestInfo, id: string) {
  const root = resolve(process.env.LC_BROWSER_EVIDENCE!, "parity");
  await mkdir(root, { recursive: true });
  const path = resolve(root, `1429-${id}.png`);
  await page.screenshot({ path });
  await info.attach(`parity/1429:${id}`, { path, contentType: "image/png" });
}

function ledgerFor(info: TestInfo, locale: string) {
  const rows: { page: string; locale: string; control: string; operation: string; expected: string; actual: string; result: string }[] = [];
  return {
    async click(control: string, expected: string, action: () => Promise<void>) {
      try {
        await action();
        rows.push({ page: "/orders/shipping", locale, control, operation: "real click and visible assertion", expected, actual: expected, result: "PASS" });
      } catch (error) {
        rows.push({ page: "/orders/shipping", locale, control, operation: "real click and visible assertion", expected, actual: String(error), result: "FAIL" });
        throw error;
      } finally {
        await writeFile(info.outputPath("shipping-click-ledger.json"), JSON.stringify(rows, null, 2));
      }
    },
  };
}

// Register through the existing signed fixture, like settings-v2.spec.ts. Merely matching a filename is not CI execution.
export function registerShippingCenterTests(login: (page: Page) => Promise<void>) {
  const store = process.env.LC_BROWSER_ORDER_STORE!;
  const path = (locale: string) => `/${locale}/orders/shipping?store=${store}`;
  for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
    const c = copies[locale];
    test(`PARITY-SHIPCENTER RV3 ${locale}: long batch summary cannot cover rows or pagination at 1440 and 390`, async ({ page }, info) => {
      info.setTimeout(90000);
      await login(page);
      // MOCK read preparation: two full valid pages, four ordinary named products per order;
      // all quantities/amounts still reconcile, and each full identity is unique.
      const base = fixture(locale)[0];
      const rows = Array.from({ length: 20 }, (_, i) => {
        const id = `11111111-1111-4111-8111-${String(90000 + i).padStart(12, "0")}`;
        return { summary: { ...base.summary, order_id: id, order_number: `LC-${id.replaceAll("-", "").toUpperCase()}` },
          detail: { ...base.detail, order_id: id, items: Array.from({ length: 4 }, (_, j) => ({
            ...base.detail.items[0], sku_id: `22222222-2222-4222-8222-${String(80000 + j).padStart(12, "0")}`,
            code: `SYNTHETIC-BATCH-${j}`, name: `${c.items[j % c.items.length]} · ${c.items[(j + 1) % c.items.length]}`,
            quantity: 1, unit_price_minor: 32000,
            amount: { subtotal_minor: 32000, discount_minor: 0, tax_minor: 0, total_minor: 32000 },
          })) } };
      });
      const { writes } = await mockReads(page, store, locale, rows, 10);
      const ledger = ledgerFor(info, locale);
      const next = locale === "zh-TW" ? "下一頁" : locale === "zh-CN" ? "下一页" : "Next page";
      const first = locale === "zh-TW" ? "返回第一頁" : locale === "zh-CN" ? "返回第一页" : "Back to first page";
      const checkbox = (i: number) => content(page).getByRole("checkbox", { name: `#${90000 + i}`, exact: true });
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(path(locale));
      for (let i = 0; i < 20; i++) {
        if (i === 10) await ledger.click(next, "next page remains visible and clickable with ten selections", async () => {
          await button(page, next).scrollIntoViewIfNeeded();
          await expect(button(page, next)).toBeInViewport();
          await button(page, next).click();
          await expect(checkbox(i)).toBeVisible();
        });
        await ledger.click(`batch select #${90000 + i}`, "real selection keeps the signed 76px footer bounded", async () => {
          await checkbox(i).click();
          await expect(checkbox(i)).toBeChecked();
          // READ/MEASURE only: no DOM mutation or synthetic input; frame 1429:28034 reserves 76px.
          const footer = await content(page).locator("footer").boundingBox();
          expect(footer, "selection footer exists").not.toBeNull();
          expect(footer!.height, "RV3 selection footer stays within the signed 76px height").toBeLessThanOrEqual(76);
          expect(footer!.y).toBeGreaterThanOrEqual(0);
          expect(footer!.y + footer!.height).toBeLessThanOrEqual(900);
        });
      }
      await page.reload();
      await expect(content(page)).toContainText(c.selected(20));
      await ledger.click("last desktop row deselect/reselect", "last row is reachable, clickable and persisted after refresh", async () => {
        await checkbox(19).scrollIntoViewIfNeeded();
        await expect(checkbox(19)).toBeInViewport();
        await checkbox(19).click();
        await expect(checkbox(19)).not.toBeChecked();
        await page.reload();
        await expect(checkbox(19)).not.toBeChecked();
        await checkbox(19).click();
        await expect(checkbox(19)).toBeChecked();
      });
      await ledger.click(first, "first-page control remains reachable with twenty selections", async () => {
        await button(page, first).scrollIntoViewIfNeeded();
        await expect(button(page, first)).toBeInViewport();
        await button(page, first).click();
        await expect(checkbox(0)).toBeChecked();
      });
      await ledger.click(c.view, "bounded footer View still opens the complete selection", async () => {
        await button(page, c.view).click();
        await expect(content(page).getByTestId("shipping-selected-orders").locator("article")).toHaveCount(20);
        await button(page, c.back).click();
      });
      // Preserve the signed mobile single-order task: restore the real desktop selection,
      // then verify rows/paging through mobile links, never hidden/forced batch controls.
      await page.setViewportSize({ width: 390, height: 844 });
      await page.reload();
      await expect(content(page).locator("footer")).toBeHidden();
      await ledger.click("mobile next page with restored batch", "pagination stays in the viewport and opens page two", async () => {
        await button(page, next).scrollIntoViewIfNeeded();
        await expect(button(page, next)).toBeInViewport();
        await button(page, next).click();
        await expect(page).toHaveURL(/cursor=shipping-page-2/);
      });
      const mobileURL = page.url();
      await ledger.click("last mobile order #90019", "last row link stays reachable and opens the existing detail", async () => {
        const last = content(page).getByTestId(`shipping-order-${rows[19].summary.order_id}`).getByRole("link").filter({ hasText: "#90019" });
        await last.scrollIntoViewIfNeeded();
        await expect(last).toBeInViewport();
        await last.click();
        await expect(page).toHaveURL(new RegExp(`order=${rows[19].summary.order_id}`));
      });
      await page.goto(mobileURL);
      await ledger.click("mobile first page", "pagination remains clickable after reload", async () => {
        await button(page, first).scrollIntoViewIfNeeded();
        await expect(button(page, first)).toBeInViewport();
        await button(page, first).click();
        await expect(content(page).getByTestId(`shipping-order-${rows[0].summary.order_id}`)).toBeVisible();
      });
      await page.setViewportSize({ width: 1440, height: 900 });
      await ledger.click(c.cancel, "Cancel clears all twenty selections and persists after reload", async () => {
        await button(page, c.cancel).click();
        await expect(button(page, c.view)).toBeDisabled();
        await page.reload();
        await expect(button(page, c.view)).toBeDisabled();
        await expect(content(page).getByRole("checkbox", { checked: true })).toHaveCount(0);
      });
      expect(writes).toEqual([]);
    });

    test(`PARITY-SHIPCENTER RV3 ${locale}: printed cross-border home address retains destination country`, async ({ page }, info) => {
      await login(page);
      const rows = fixture(locale);
      rows[0].detail.country = "HK";
      rows[0].detail.destination.country = "HK";
      const { writes } = await mockReads(page, store, locale, rows);
      const ledger = ledgerFor(info, locale);
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(path(locale));
      await ledger.click("select → review → preview", "cross-border order reaches preview through real controls", async () => {
        await select(page, 0).click();
        await button(page, c.view).click();
        await button(page, c.confirm).click();
        await expect(content(page).getByTestId("shipping-packing-slip")).toHaveCount(1);
      });
      await page.reload();
      await page.emulateMedia({ media: "print" });
      const addressLabel = locale === "en" ? "Destination" : "配送目的地";
      const address = content(page).getByTestId("shipping-packing-slip").locator("dl > div").filter({ has: page.locator("dt", { hasText: addressLabel }) }).locator("dd");
      await expect(address).toBeVisible();
      for (const fact of ["000", "Synthetic city", "Synthetic address", "HK"]) {
        await expect(address, `RV3 printed home address includes server destination fact ${fact}`).toContainText(fact);
      }
      expect(writes).toEqual([]);
    });

    const cases = locale === "zh-TW" ? combinations : combinations.filter(c => c.name === "all");
    for (const combo of cases) {
      test(`PARITY-SHIPCENTER ${locale} ${combo.name}: select, review, print, return and refresh`, async ({ page }, info) => {
        info.setTimeout(90000);
        await login(page);
        const { rows, writes } = await mockReads(page, store, locale);
        const ledger = ledgerFor(info, locale);
        await page.setViewportSize({ width: 1440, height: 900 });
        await page.goto(path(locale));
        await expect(content(page).getByRole("heading", { name: c.center, exact: true }), "signed shipping-center heading on the REAL route").toBeVisible();
        await expect(content(page)).toContainText(c.note);
        for (const legacy of [/Export unshipped|匯出未出貨|导出未发货/i, /Carrier export|批量回填/i, /Purchase label|購買面單|购买面单/i]) {
          await expect(content(page).getByRole("button", { name: legacy })).toHaveCount(0);
        }
        await expect(button(page, c.view)).toBeDisabled();
        await expect(content(page)).toContainText(c.empty);
        if (locale === "zh-TW" && combo.name === "first") await screenshot(page, info, "28034");
        for (const i of combo.indices) await ledger.click(`select #${numbers[i]}`, "checkbox and URL reflect the selected server order", async () => {
          await select(page, i).click();
          await expect(select(page, i)).toBeChecked();
          await expect(page).toHaveURL(new RegExp(orderIDs[i]));
        });
        await page.reload();
        for (const i of combo.indices) await expect(select(page, i)).toBeChecked();
        await expect(content(page)).toContainText(c.selected(combo.indices.length));
        await expect(content(page)).toContainText(c.needsReview);
        if (locale === "zh-TW") await screenshot(page, info, combo.frames[0]);
        await ledger.click(c.view, "review contains exactly the selected orders and print boundary", async () => {
          await button(page, c.view).click();
          await expect(content(page).getByRole("heading", { name: c.review, exact: true })).toBeVisible();
          const review = content(page).getByTestId("shipping-selected-orders");
          for (const [i, n] of numbers.entries()) {
            if (combo.indices.includes(i)) { await expect(review).toContainText(`#${n}`); await expect(review).toContainText(rows[i].detail.items[0].name); }
            else await expect(review).not.toContainText(`#${n}`);
          }
          await expect(content(page)).toContainText(c.boundary);
        });
        await page.reload();
        await expect(content(page).getByRole("heading", { name: c.review, exact: true })).toBeVisible();
        if (locale === "zh-TW") await screenshot(page, info, combo.frames[1]);
        await ledger.click(c.confirm, "one packing slip per order from existing read DTOs", async () => {
          await button(page, c.confirm).click();
          await expect(content(page).getByRole("heading", { name: c.preview, exact: true })).toBeVisible();
          const slips = content(page).getByTestId("shipping-packing-slip");
          await expect(slips).toHaveCount(combo.indices.length);
          for (const i of combo.indices) {
            const slip = slips.filter({ hasText: `${c.order} #${numbers[i]}` });
            for (const value of ["大夢甄選", `${c.order} #${numbers[i]}`, c.recipient, c.buyer, c.delivery, c.manual, rows[i].detail.items[0].name]) await expect(slip).toContainText(value);
          }
        });
        await page.reload();
        await expect(content(page).getByRole("heading", { name: c.preview, exact: true })).toBeVisible();
        if (locale === "zh-TW") await screenshot(page, info, combo.frames[2]);
        await page.emulateMedia({ media: "print" });
        await expect(content(page).getByTestId("shipping-packing-slip")).toHaveCount(combo.indices.length);
        await expect(button(page, c.print)).toBeHidden();
        await expect(content(page).getByRole("heading", { name: c.preview, exact: true })).toBeHidden();
        // READ/MEASURE: inspect the actual print DOM/CSS, without changing the DOM or simulating input.
        const printed = await content(page).getByTestId("shipping-packing-slip").evaluateAll(nodes => nodes.map(n => ({ visible: getComputedStyle(n).display !== "none", pageBreak: getComputedStyle(n).breakAfter })));
        expect(printed.every(p => p.visible && ["page", "always"].includes(p.pageBreak))).toBe(true);
        await page.emulateMedia({ media: "screen" });
        await ledger.click(c.print, "real print handler called once; result explains cancellation and no shipment mutation", async () => {
          await button(page, c.print).click();
          await expect(content(page).getByRole("heading", { name: c.done, exact: true })).toBeVisible();
          await expect(content(page)).toContainText(c.outcome);
          expect(await page.evaluate(() => (window as unknown as { shippingPrintCalls: number }).shippingPrintCalls)).toBe(1);
          expect(writes).toEqual([]);
        });
        if (locale === "zh-TW") await screenshot(page, info, combo.frames[3]);
        await ledger.click(c.backPreview, "return to preview retains all selected slips after reload", async () => {
          await button(page, c.backPreview).click();
          await expect(content(page).getByRole("heading", { name: c.preview, exact: true })).toBeVisible();
          await page.reload();
          await expect(content(page).getByTestId("shipping-packing-slip")).toHaveCount(combo.indices.length);
        });
        await button(page, c.print).click();
        await ledger.click(c.backCenter, "shipping center retains selection and original review/fulfillment facts", async () => {
          await button(page, c.backCenter).click();
          await expect(content(page).getByRole("heading", { name: c.center, exact: true })).toBeVisible();
          await page.reload();
          for (const i of combo.indices) await expect(select(page, i)).toBeChecked();
          await expect(content(page)).toContainText(c.needsReview);
          expect(writes).toEqual([]);
        });
        // READ/MEASURE only: selections are URL-only; never persist recipient/address/phone in browser storage or URL.
        const storage = await page.evaluate(() => JSON.stringify([...Object.entries(localStorage), ...Object.entries(sessionStorage)]));
        for (const privateValue of [c.buyer, "+886900000001", "Synthetic address"]) {
          expect(storage).not.toContain(privateValue);
          expect(decodeURIComponent(page.url())).not.toContain(privateValue);
        }
      });
    }
    test(`PARITY-SHIPCENTER ${locale} signed mobile single-order screen`, async ({ page }, info) => {
      await login(page);
      const { writes } = await mockReads(page, store, locale);
      await page.setViewportSize({ width: 390, height: 844 });
      await page.goto(path(locale));
      await expect(content(page).getByRole("heading", { name: c.center, exact: true })).toBeVisible();
      await expect(content(page)).toContainText(c.mobile);
      await expect(content(page).locator('input[type="checkbox"]:visible')).toHaveCount(0);
      await expect(button(page, c.print)).toHaveCount(0);
      if (locale === "zh-TW") await screenshot(page, info, "28313");
      // READ/MEASURE only: native layout and density, excluding the global shell.
      const measure = await content(page).evaluate(root => {
        const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT), texts: string[] = [];
        while (walker.nextNode()) if (walker.currentNode.parentElement?.checkVisibility() && walker.currentNode.textContent?.trim()) texts.push(walker.currentNode.textContent.trim());
        return { overflow: document.documentElement.scrollWidth > innerWidth, blocks: texts.length, chars: texts.join("").length };
      });
      expect(measure.overflow).toBe(false);
      expect(measure.blocks).toBeLessThanOrEqual(20);
      if (locale !== "en") expect(measure.chars).toBeLessThanOrEqual(100);
      for (const i of [0, 1, 2]) {
        await page.goto(path(locale));
        const ledger = ledgerFor(info, locale);
        await ledger.click(`mobile #${numbers[i]}`, "opens the existing order detail with the original full server identity", async () => {
          await content(page).getByRole("link").filter({ hasText: `#${numbers[i]}` }).click();
          await expect(page).toHaveURL(new RegExp(`/${locale}/orders\\?[^#]*order=${orderIDs[i]}`));
        });
      }
      expect(writes).toEqual([]);
    });
  }

  test("PARITY-SHIPCENTER every selection/review cancellation and return control", async ({ page }, info) => {
    info.setTimeout(90000);
    await login(page);
    const c = copies["zh-TW"], ledger = ledgerFor(info, "zh-TW");
    await mockReads(page, store, "zh-TW");
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(path("zh-TW"));
    await expect(content(page).getByRole("heading", { name: c.center, exact: true })).toBeVisible();
    await select(page, 0).click();
    await ledger.click("toggle selected order", "unselect disables review and persists after reload", async () => {
      await select(page, 0).click();
      await expect(select(page, 0)).not.toBeChecked();
      await page.reload();
      await expect(button(page, c.view)).toBeDisabled();
    });
    for (const destination of [c.back, c.edit, c.cancel]) {
      await select(page, 0).click();
      await button(page, c.view).click();
      await ledger.click(`review ${destination}`, "returns to shipping selection", async () => {
        await button(page, destination).click();
        await expect(content(page).getByRole("heading", { name: c.center, exact: true })).toBeVisible();
      });
      // Reset using the actual selection-cancel control; no evaluate/URL writes substitute for controls.
      await button(page, c.cancel).click();
      await expect(button(page, c.view)).toBeDisabled();
    }
    for (const destination of [c.back, c.edit, c.cancel]) {
      await select(page, 0).click();
      await button(page, c.view).click();
      await button(page, c.confirm).click();
      await ledger.click(`preview ${destination}`, "returns to selection without printing", async () => {
        await button(page, destination).click();
        await expect(content(page).getByRole("heading", { name: c.center, exact: true })).toBeVisible();
        expect(await page.evaluate(() => (window as unknown as { shippingPrintCalls: number }).shippingPrintCalls)).toBe(0);
      });
      await button(page, c.cancel).click();
      await expect(button(page, c.view)).toBeDisabled();
    }
    await ledger.click(c.backOrders, "returns to existing orders route", async () => {
      await content(page).getByRole("link", { name: c.backOrders, exact: true }).click();
      await expect(page).toHaveURL(new RegExp(`/zh-TW/orders\\?store=${store}$`));
    });
    for (const i of [0, 1, 2]) {
      await page.goto(path("zh-TW"));
      await ledger.click(`desktop view #${numbers[i]}`, "opens the existing order detail without changing selection or shipment", async () => {
        await content(page).getByTestId(`shipping-order-${orderIDs[i]}`).getByRole("link", { name: "查看", exact: true }).click();
        await expect(page).toHaveURL(new RegExp(`/zh-TW/orders\\?[^#]*order=${orderIDs[i]}`));
      });
    }
  });

  test("PARITY-SHIPCENTER existing server reads reach preview without synthetic DTO interception", async ({ page }, info) => {
    info.setTimeout(60000);
    await login(page);
    const ledger = ledgerFor(info, "en"), writes: string[] = [];
    page.on("request", request => {
      if (new URL(request.url()).pathname.startsWith(`/api/stores/${store}/`) && !["GET", "HEAD"].includes(request.method())) writes.push(request.method());
    });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(path("en"));
    await expect(content(page).getByRole("heading", { name: "Shipping center", exact: true })).toBeVisible();
    const check = content(page).getByRole("checkbox").first();
    await expect(check).toBeVisible();
    const number = await check.getAttribute("aria-label");
    expect(number).toMatch(/^#[A-Za-z0-9]{5}$/);
    await check.click();
    await page.reload();
    await expect(content(page).getByRole("checkbox", { name: number!, exact: true })).toBeChecked();
    await ledger.click("View → Confirm and preview", "existing signed BFF order data renders one packing slip after reload", async () => {
      await button(page, "View").click();
      await button(page, "Confirm and preview").click();
      const slip = content(page).getByTestId("shipping-packing-slip");
      await expect(slip).toHaveCount(1);
      await expect(slip).toContainText(number!);
      await expect(slip).toContainText("Recipient");
      await expect(slip).toContainText("Delivery method");
      await page.reload();
      await expect(slip).toHaveCount(1);
      expect(writes).toEqual([]);
    });
  });

  test("PARITY-SHIPCENTER detail read failure cannot print partial or invented slips; Retry recovers", async ({ page }, info) => {
    await login(page);
    const ledger = ledgerFor(info, "en");
    const { writes } = await mockReads(page, store, "en");
    let failing = true;
    // MOCK read fault only: never alter production state or bypass a control's event handler.
    await page.route(url => url.pathname === `/api/stores/${store}/orders/${orderIDs[0]}`, async route => {
      if (failing) await route.fulfill({ status: 503, json: { code: "synthetic_unavailable" } });
      else await route.fallback();
    });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(path("en"));
    await expect(content(page).getByRole("heading", { name: "Shipping center", exact: true })).toBeVisible();
    await expect(content(page).getByRole("alert")).toBeVisible();
    await expect(button(page, "Print")).toHaveCount(0);
    await expect(content(page).getByTestId("shipping-packing-slip")).toHaveCount(0);
    failing = false;
    await ledger.click("Retry", "failed read is revalidated; all three actual read DTOs reappear", async () => {
      await button(page, "Retry").click();
      await expect(select(page, 0)).toBeVisible();
      await expect(content(page).getByRole("alert")).toHaveCount(0);
      await select(page, 0).click();
      await button(page, "View").click();
      await button(page, "Confirm and preview").click();
      await expect(content(page).getByTestId("shipping-packing-slip")).toHaveCount(1);
      expect(writes).toEqual([]);
    });
  });
}
