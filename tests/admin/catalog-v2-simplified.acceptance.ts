// Purpose: Add signed simplified catalog acceptance using native merchant clicks and synthetic server responses.
// Depends on: @playwright/test, node:fs/promises; existing catalog-core signed MOCK identity harness.
// Used by: catalog-core.spec.ts in the existing TWD product-editor CI browser mode.
// Invariants: additive assertions; MOCK response state survives reload, never claims PG/LIVE acceptance; no UI writes via API/evaluate.
import { expect, test, type Page, type Route, type TestInfo } from "@playwright/test";
import { writeFile } from "node:fs/promises";

const id = (n: number) => `11111111-1111-4111-8111-${String(n).padStart(12, "0")}`;
const productID = id(1);
// Variant picker copy interpolates the merchant's option value verbatim, including case.
const signed = {
  "zh-TW": { title: "商品", names: ["帆布提袋", "棉質上衣", "保溫杯"], description: "日常外出使用。", all: "全部 3", active: "在售 2", draft: "草稿 1", archived: "已歸檔 0", empty: "尚無已歸檔商品", edit: "編輯", select: "選取 3 件", archive: "歸檔商品", confirmArchive: "確認歸檔", backSelection: "返回選取", archiveTitle: "確認歸檔 3 件", archivePolicy: "歸檔後停止接單；舊訂單保留。", result: "2 件已歸檔，1 件未變更", live: "直播接單中，結束後再歸檔。", view: "查看商品列表", name: "商品名稱", desc: "商品說明", price: "兩款售價 · TWD", variants: "規格與庫存", stock: "調整庫存", total: "調整後現有", checkStock: "核對庫存", keep: "保留目前庫存", backVariants: "返回規格", backEditor: "返回商品編輯", review: "檢查變更", reviewTitle: "核對變更", confirm: "確認儲存", continueEdit: "繼續編輯", saved: "商品已儲存", media: "商品圖片 · 未加入", choose: "選擇圖片", ivory: "米白", navy: "深藍", chooseIvory: "選擇米白圖片", chooseNavy: "選擇深藍圖片", main: "主圖 0/4 · 上架必填", detail: "詳情圖 0/20", stockError: "未儲存：3 件少於占用的 5 件；全部變更未送出。", cancel: "取消變更", soldOut: "售罄" },
  "zh-CN": { title: "商品", names: ["帆布手提袋", "纯棉上衣", "保温杯"], description: "适合日常外出。", all: "全部 3", active: "在售 2", draft: "草稿 1", archived: "已归档 0", empty: "暂无已归档商品", edit: "编辑", select: "选择 3 件", archive: "归档商品", confirmArchive: "确认归档", backSelection: "返回选择", archiveTitle: "确认归档 3 件", archivePolicy: "归档后停止接单；保留原订单。", result: "2 件已归档，1 件未更改", live: "直播接单中，结束后再归档。", view: "查看商品列表", name: "商品名称", desc: "商品说明", price: "两款售价 · TWD", variants: "规格与库存", stock: "调整库存", total: "调整后现有", checkStock: "核对库存", keep: "保留当前库存", backVariants: "返回规格", backEditor: "返回商品编辑", review: "检查更改", reviewTitle: "核对更改", confirm: "确认保存", continueEdit: "继续编辑", saved: "商品已保存", media: "商品图片 · 未添加", choose: "选择图片", ivory: "米白", navy: "深蓝", chooseIvory: "选择米白图片", chooseNavy: "选择深蓝图片", main: "主图 0/4 · 上架必填", detail: "详情图 0/20", stockError: "未保存：3 件少于占用的 5 件；全部更改未提交。", cancel: "取消更改", soldOut: "售罄" },
  en: { title: "Products", names: ["Canvas tote", "Cotton top", "Insulated cup"], description: "For everyday outings.", all: "All 3", active: "Active 2", draft: "Draft 1", archived: "Archived 0", empty: "No archived products", edit: "Edit", select: "Select 3", archive: "Archive products", confirmArchive: "Confirm archive", backSelection: "Back to selection", archiveTitle: "Archive 3 products", archivePolicy: "Archiving stops new orders. Existing orders remain.", result: "2 archived, 1 unchanged", live: "Archive after live orders close.", view: "View products", name: "Product name", desc: "Description", price: "Both prices · TWD", variants: "Variants and stock", stock: "Adjust stock", total: "New on-hand quantity", checkStock: "Check stock", keep: "Keep current stock", backVariants: "Back to variants", backEditor: "Back to product editor", review: "Review changes", reviewTitle: "Review changes", confirm: "Confirm save", continueEdit: "Continue editing", saved: "Product saved", media: "Product images · None", choose: "Choose images", ivory: "Ivory", navy: "Navy", chooseIvory: "Choose Ivory image", chooseNavy: "Choose Navy image", main: "Main 0/4 · Required to publish", detail: "Detail 0/20", stockError: "Not saved: 3 is below 5 committed. No changes submitted.", cancel: "Cancel changes", soldOut: "Sold out" },
} as const;
type Locale = keyof typeof signed;
type Ledger = { page: string; control: string; operation: string; expected: string; actual: string; status: "PASS" | "FAIL" }[];

function syntheticCatalog(locale: Locale) {
  const c = signed[locale];
  const skus = [c.ivory, c.navy].map((title, i) => ({
    id: id(10 + i), code: i ? "TOTE-NAVY" : "TOTE-IVORY", title, price_minor: 128000,
    compare_at_minor: null, option_values: [title], version: 1, currency: "TWD", available: i ? 0 : 3,
    inventory_tracked: true, max_per_order: null, keyword: "", on_hand: i ? 0 : 8, committed: i ? 0 : 5,
    weight_grams: 0, length_mm: 0, width_mm: 0, height_mm: 0, origin_country: "", customs_name: "", hs_candidate: "",
  }));
  const detail = { id: productID, slug: "synthetic-tote", name: c.names[0] as string, description: c.description as string,
    status: "active", version: 1, seo_title: "", seo_description: "", options: [{ name: "Color", values: [c.ivory, c.navy] }],
    skus, collection_ids: [], warehouse_id: id(20) };
  const rows = c.names.map((name, i) => ({ id: id(i + 1), slug: `synthetic-${i}`, name: String(name), status: i === 2 ? "draft" : "active",
    version: 1, cover_image_id: null, price_min_minor: [128000, 69000, 49000][i], price_max_minor: [128000, 69000, 49000][i],
    currency: "TWD", sku_count: i ? 1 : 2, available: [3, 0, 16][i], keyword: "", inventory_tracked: true,
    updated_at: "2026-10-28T00:00:00Z" }));
  return { detail, rows };
}

/** Own isolated MOCK server state per test; only route responses are prepared, never DOM or app state. */
async function setup(page: Page, locale: Locale) {
  const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN!;
  const store = process.env.LC_BROWSER_STORE!;
  const fixture = syntheticCatalog(locale);
  const writes: { method: string; resource: string; body: Record<string, any> }[] = [];
  const respond = (route: Route, body: unknown, status = 200) => route.fulfill({ status, contentType: "application/json", headers: { "cache-control": "no-store" }, body: JSON.stringify(body) });
  await page.route(`**/api/stores/${store}/**`, async (route) => {
    const request = route.request(), url = new URL(request.url());
    const resource = url.pathname.split(`/api/stores/${store}/`)[1];
    if (request.method() === "GET") {
      if (resource === "catalog-products") {
        const status = url.searchParams.get("status");
        return respond(route, { items: fixture.rows.filter((row) => !status || row.status === status), next_cursor: "", total: 3,
          status_counts: Object.fromEntries(["active", "draft", "archived"].map((s) => [s, fixture.rows.filter((r) => r.status === s).length])) });
      }
      if (resource === `products/${productID}`) return respond(route, fixture.detail);
      if (resource === `products/${productID}/images`) return respond(route, { items: [], image_axis: "Color", option_images: [] });
      if (resource === "warehouses") return respond(route, [{ id: id(20), name: "Synthetic warehouse" }]);
      if (resource === "collections") return respond(route, { items: [], next_cursor: "" });
      return route.continue();
    }
    if (resource !== "products/bulk-status" && resource !== `products/${productID}/document`) return route.continue();
    const body = request.postDataJSON();
    writes.push({ method: request.method(), resource, body });
    expect(request.headers()["idempotency-key"], "each UI command has an idempotency key").toBeTruthy();
    expect(body).not.toHaveProperty("tenant_id");
    if (resource === "products/bulk-status") {
      expect(request.method()).toBe("POST");
      expect(body.status).toBe("archived");
      expect(body.ids).toEqual(fixture.rows.map((r) => r.id));
      return respond(route, fixture.rows.map((row, i) => {
        if (i === 1) return { id: row.id, error: "live_window_open" };
        row.status = "archived";
        return { id: row.id, status: row.status };
      }));
    }
    expect(request.method()).toBe("PUT");
    expect(body.expected_version).toBe(fixture.detail.version);
    if (body.skus?.some((s: any) => s.stock?.target_qty !== undefined && s.stock.target_qty < (fixture.detail.skus.find((old) => old.id === s.id)?.committed ?? 0)))
      return respond(route, { code: "insufficient_inventory" }, 409);
    for (const field of ["name", "description", "status"] as const) if (field in body) fixture.detail[field] = body[field];
    for (const change of body.skus ?? []) {
      const sku = fixture.detail.skus.find((s) => s.id === change.id);
      expect(sku, "edit targets an existing server SKU").toBeTruthy();
      if (change.price_minor !== undefined) sku!.price_minor = change.price_minor;
      if (change.stock?.target_qty !== undefined) { sku!.on_hand = change.stock.target_qty; sku!.available = sku!.on_hand - sku!.committed; }
    }
    fixture.detail.version++;
    fixture.rows[0].name = fixture.detail.name;
    return respond(route, { id: productID });
  });
  await page.goto(`${origin}/en/`);
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("nav-orders")).toBeAttached();
  const open = (suffix = "") => page.goto(`${origin}/${locale}/products${suffix}?store=${store}`);
  return { ...fixture, writes, open };
}

async function action(ledger: Ledger, page: Page, control: string, operation: string, expected: string, run: () => Promise<void>) {
  const row: Ledger[number] = { page: new URL(page.url()).pathname, control, operation, expected, actual: "assertion pending", status: "FAIL" };
  ledger.push(row);
  await run();
  row.actual = expected; row.status = "PASS";
}
async function saveLedger(ledger: Ledger, info: TestInfo) {
  const file = info.outputPath("catalog-v2-click-ledger.json");
  await writeFile(file, JSON.stringify({ evidence: "MOCK", revision: process.env.GITHUB_SHA ?? "UNKNOWN", ledger }, null, 2));
  await info.attach("catalog-v2-click-ledger", { path: file, contentType: "application/json" });
}

const mobileDensity = new Map<string, { blocks: number; chars: number }>();
/** READ/MEASURE only: count visible text parents plus editable values inside the task surface, including scroll content. */
async function density(page: Page, state: string, locale: Locale) {
  if (locale !== "zh-CN" || page.viewportSize()?.width !== 390) return;
  const value = await page.locator('[data-testid="products-page"], [data-testid="product-editor"]').first().evaluate((root) => {
    const visible = (e: Element) => {
      for (let p: Element | null = e; p; p = p.parentElement) {
        const style = getComputedStyle(p);
        if (style.display === "none" || style.visibility === "hidden" || Number(style.opacity) === 0 || p.hasAttribute("hidden")) return false;
      }
      return e.getClientRects().length > 0;
    };
    const parents = new Set<Element>();
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    while (walker.nextNode()) {
      const text = walker.currentNode;
      if (text.textContent?.trim() && text.parentElement && visible(text.parentElement) && !text.parentElement.closest('script,style,[aria-hidden="true"]')) parents.add(text.parentElement);
    }
    const texts = [...parents].map((e) => [...e.childNodes].filter((n) => n.nodeType === Node.TEXT_NODE).map((n) => n.textContent).join("").trim()).filter(Boolean);
    for (const input of root.querySelectorAll<HTMLInputElement | HTMLTextAreaElement>('input:not([type="checkbox"]):not([type="radio"]):not([type="hidden"]):not([type="file"]),textarea'))
      if (visible(input) && input.value.trim()) texts.push(input.value.trim());
    return { blocks: texts.length, chars: texts.reduce((n, text) => n + Array.from(text.replace(/\s/g, "")).length, 0) };
  });
  mobileDensity.set(state, value);
}

/** Register one batch; original CC12/PE assertions and environments are unchanged. */
export function registerCatalogSimplifiedAcceptance() {
  test.describe("CATV2 signed simplified catalog (MOCK responses)", () => {
    test.describe.configure({ mode: "default" });
    for (const locale of ["zh-TW", "zh-CN", "en"] as const) for (const width of [390, 1440]) {
      test(`CATV2 list ${locale} ${width}: signed copy, all filters and row identity`, async ({ page }, info) => {
        const ledger: Ledger = [], c = signed[locale];
        await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
        try {
          const f = await setup(page, locale); await f.open();
          const screen = page.getByTestId("products-page");
          await expect(screen).toBeVisible();
          await expect(screen.getByText(c.names[0], { exact: true }).first()).toBeVisible();
          await density(page, "list", locale);
          // Causal RED assertion on existing rendered controls, never on an absent module import.
          await expect(screen.getByTestId("quick-price"), "signed rows expose one Edit action rather than inline price/stock editors").toHaveCount(0);
          await expect(screen.getByTestId("quick-stock")).toHaveCount(0);
          await expect(screen.getByRole("heading", { name: c.title, exact: true })).toHaveCount(1);
          for (const name of c.names) await expect(screen.getByText(name, { exact: true }).first()).toBeVisible();
          await expect(screen.getByText(c.soldOut, { exact: false }).first()).toBeVisible();
          for (const [status, label, count] of [["active", c.active, 2], ["draft", c.draft, 1], ["archived", c.archived, 0], ["all", c.all, 3]] as const) {
            await action(ledger, page, label, "click", `${count} matching products after refresh`, async () => {
              await page.getByTestId(`products-tab-${status}`).click();
              await expect(page.getByTestId(`products-tab-${status}`)).toHaveAttribute("aria-pressed", "true");
              await expect(page.locator('[data-testid^="product-row-"]')).toHaveCount(count);
              if (!count) await expect(screen.getByText(c.empty, { exact: true })).toBeVisible();
              await density(page, status, locale);
              await page.reload();
              await expect(page.locator('[data-testid^="product-row-"]')).toHaveCount(count);
            });
          }
          await action(ledger, page, `${c.names[0]} ${c.edit}`, "click", "the clicked product's editor", async () => {
            await page.getByTestId(`product-row-${productID}`).getByRole("link", { name: c.edit, exact: true }).click();
            await expect(page).toHaveURL(new RegExp(`/products/${productID}`));
            await expect(page.getByLabel(c.name, { exact: true })).toHaveValue(c.names[0]);
          });
          // READ/MEASURE only: containment, not DOM mutation or a substitute for user input.
          expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
          expect(f.writes).toHaveLength(0);
        } finally { await saveLedger(ledger, info); }
      });
    }
    for (const [locale, width] of [["zh-CN", 390], ["en", 1440]] as const) {
      test(`CATV2 archive ${locale}: selection, cancel, per-item outcomes and persisted refresh`, async ({ page }, info) => {
        const ledger: Ledger = [], c = signed[locale];
        await page.setViewportSize({ width, height: 900 });
        try {
          const f = await setup(page, locale); await f.open();
          await expect(page.getByTestId(`product-row-${productID}`)).toBeVisible();
          await action(ledger, page, c.select, "click", "all three products selected", async () => {
            await page.getByRole("checkbox", { name: c.select, exact: true }).click();
            await expect(page.getByRole("button", { name: c.archive, exact: true })).toBeEnabled();
            await density(page, "selected", locale);
          });
          await action(ledger, page, c.archive, "click", "review identities and archive impact without sending", async () => {
            await page.getByRole("button", { name: c.archive, exact: true }).click();
            await expect(page.getByRole("heading", { name: c.archiveTitle, exact: true })).toBeVisible();
            for (const name of c.names) await expect(page.getByText(name, { exact: true }).first()).toBeVisible();
            await expect(page.getByText(c.archivePolicy, { exact: true })).toBeVisible();
            await density(page, "bulkReview", locale);
            expect(f.writes).toHaveLength(0);
          });
          await page.getByRole("button", { name: c.backSelection, exact: true }).click();
          expect(f.writes).toHaveLength(0);
          await page.getByRole("button", { name: c.archive, exact: true }).click();
          await action(ledger, page, c.confirmArchive, "click", "two archived, one unchanged with reason", async () => {
            await page.getByRole("button", { name: c.confirmArchive, exact: true }).click();
            await expect(page.getByText(c.result, { exact: true })).toBeVisible();
            await expect(page.getByText(c.live, { exact: true })).toBeVisible();
            await density(page, "bulkResult", locale);
            expect(f.writes).toHaveLength(1);
          });
          await page.getByRole("button", { name: c.view, exact: true }).click();
          await page.reload();
          await expect(page.getByTestId("products-tab-archived")).toContainText("2");
          await density(page, "updated", locale);
          await page.getByTestId("products-tab-active").click();
          await expect(page.getByTestId(`product-row-${id(2)}`)).toBeVisible();
          await expect(page.getByTestId(`product-row-${productID}`)).toHaveCount(0);
        } finally { await saveLedger(ledger, info); }
      });

      test(`CATV2 edit ${locale}: invalid stock sends nothing, review retains facts, save survives refresh`, async ({ page }, info) => {
        const ledger: Ledger = [], c = signed[locale];
        await page.setViewportSize({ width, height: 900 });
        try {
          const f = await setup(page, locale); await f.open(`/${productID}`);
          await expect(page.getByTestId("product-name")).toHaveValue(c.names[0]);
          await density(page, "edit", locale);
          await page.getByLabel(c.name, { exact: true }).fill(`${c.names[0]} 2`);
          await page.getByLabel(c.desc, { exact: true }).fill(`${c.description} 2`);
          await page.getByLabel(c.price, { exact: true }).fill("1380");
          await action(ledger, page, c.variants, "click", "both SKU identities, independent stock and sold out", async () => {
            await page.getByRole("button", { name: c.variants, exact: true }).click();
            for (const code of ["TOTE-IVORY", "TOTE-NAVY"]) await expect(page.getByText(code, { exact: false }).first()).toBeVisible();
            await expect(page.getByText(c.soldOut, { exact: false }).first()).toBeVisible();
            await density(page, "variants", locale);
          });
          await action(ledger, page, `${c.ivory} ${c.stock}`, "click/fill/click", "3 retained, below 5 rejected; no document submitted", async () => {
            await page.getByRole("button", { name: c.stock, exact: true }).first().click();
            await density(page, "stock", locale);
            await page.getByLabel(c.total, { exact: true }).fill("3");
            await page.getByRole("button", { name: c.checkStock, exact: true }).click();
            await expect(page.getByText(c.stockError, { exact: true })).toBeVisible();
            await density(page, "stockError", locale);
            await expect(page.getByLabel(c.total, { exact: true })).toHaveValue("3");
            expect(f.writes).toHaveLength(0);
          });
          await page.getByRole("button", { name: c.keep, exact: true }).click();
          await page.getByRole("button", { name: c.backEditor, exact: true }).click();
          await action(ledger, page, c.review, "click", "review keeps changed basics, both SKU prices, stock and image counts", async () => {
            await page.getByRole("button", { name: c.review, exact: true }).click();
            await expect(page.getByRole("heading", { name: c.reviewTitle, exact: true })).toBeVisible();
            await expect(page.getByText(`${c.names[0]} 2`, { exact: true })).toBeVisible();
            await expect(page.getByText(`${c.description} 2`, { exact: true })).toBeVisible();
            await density(page, "review", locale);
            for (const code of ["TOTE-IVORY", "TOTE-NAVY"]) await expect(page.getByText(code, { exact: false }).first()).toBeVisible();
            expect(f.writes).toHaveLength(0);
          });
          await page.getByRole("button", { name: c.continueEdit, exact: true }).click();
          await expect(page.getByLabel(c.name, { exact: true })).toHaveValue(`${c.names[0]} 2`);
          await page.getByRole("button", { name: c.review, exact: true }).click();
          await action(ledger, page, c.confirm, "click", "one merge-patch and canonical saved result", async () => {
            await page.getByRole("button", { name: c.confirm, exact: true }).click();
            await expect(page.getByRole("heading", { name: c.saved, exact: true })).toBeVisible();
            await density(page, "saved", locale);
            expect(f.writes).toHaveLength(1);
            expect(f.writes[0].body.skus).toEqual(f.detail.skus.map((sku) => ({ id: sku.id, price_minor: 138000 })));
            expect(f.writes[0].body).not.toHaveProperty("status");
            expect(f.writes[0].body).not.toHaveProperty("options");
          });
          await page.reload();
          await expect(page.getByLabel(c.name, { exact: true })).toHaveValue(`${c.names[0]} 2`);
          await expect(page.getByLabel(c.desc, { exact: true })).toHaveValue(`${c.description} 2`);
          await expect(page.getByLabel(c.price, { exact: true })).toHaveValue("1380");
          await page.getByLabel(c.name, { exact: true }).fill("Unsaved synthetic edit");
          page.once("dialog", (dialog) => dialog.accept());
          await page.getByRole("button", { name: c.cancel, exact: true }).click();
          await f.open(`/${productID}`);
          await expect(page.getByLabel(c.name, { exact: true })).toHaveValue(`${c.names[0]} 2`);
          expect(f.writes).toHaveLength(1);
        } finally { await saveLedger(ledger, info); }
      });

      test(`CATV2 images ${locale}: separate roles and native pickers`, async ({ page }, info) => {
        const ledger: Ledger = [], c = signed[locale];
        await page.setViewportSize({ width, height: 900 });
        try {
          const f = await setup(page, locale); await f.open(`/${productID}`);
          await expect(page.getByTestId("product-name")).toBeVisible();
          await page.getByRole("button", { name: c.media, exact: true }).click();
          await expect(page.getByText(c.main, { exact: true })).toBeVisible();
          await expect(page.getByText(c.detail, { exact: true })).toBeVisible();
          await density(page, "media", locale);
          // Open/cancel each native picker, so no upload, API write, image or publication is fabricated.
          for (const label of [c.choose, c.chooseIvory, c.chooseNavy]) {
            await action(ledger, page, label, "click/native picker/cancel", "native file picker opens for this role", async () => {
              const chooser = page.waitForEvent("filechooser");
              await page.getByRole("button", { name: label, exact: true }).first().click();
              await (await chooser).setFiles([]);
            });
          }
          const detailPicker = page.getByRole("button", { name: c.choose, exact: true }).last();
          const chooser = page.waitForEvent("filechooser"); await detailPicker.click(); await (await chooser).setFiles([]);
          await page.getByRole("button", { name: c.backEditor, exact: true }).click();
          await expect(page.getByTestId("product-name")).toHaveValue(c.names[0]);
          expect(f.writes).toHaveLength(0);
        } finally { await saveLedger(ledger, info); }
      });
    }
    test("CATV2 price edit keeps the historical order's server amount after reload (MOCK)", async ({ page }, info) => {
      const ledger: Ledger = [];
      try {
        const f = await setup(page, 'en');
        const orderID = id(90);
        const otherWrites: string[] = [];
        page.on('request', request => { if (request.url().includes('/api/stores/') && request.method() !== 'GET' && !new URL(request.url()).pathname.endsWith(`/products/${productID}/document`)) otherWrites.push(new URL(request.url()).pathname); });
        // Synthetic placement-time projection. This tests frontend isolation, not PostgreSQL snapshot durability.
        const historical = { order_id: orderID, created_at: '2026-10-27T00:00:00Z', updated_at: '2026-10-27T00:00:00Z', currency: 'TWD', total_minor: 128000,
          commercial_state: 'CONFIRMED', fulfillment_state: 'MANUAL_UNASSIGNED', payment_state: 'CAPTURED', test_mode: false, work_state: 'READY',
          refunded_minor: 0, refund_pending_minor: 0, pickup_source: null, payment_mode: 'card', collection_state: null,
          cod_surcharge_minor: null, cod_collect_minor: null, source: 'storefront', order_number: `LC-${orderID.replaceAll('-', '').toUpperCase()}`,
          recipient_masked: 'S***', delivery_kind: 'home', live_sessions: [] };
        await page.route(`**/api/stores/${process.env.LC_BROWSER_STORE}/orders?*`, route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({
          items: [historical], next_cursor: '', total: 1, sessions: [], counts: { all: 1, unpaid: 0, transfer_review: 0, ready_to_ship: 1, ready_to_consign: 0, shipped: 0, completed: 0, cancelled: 0 },
        }) }));
        const orderRow = page.getByTestId(`order-row-${orderID}`);
        await page.getByTestId('nav-orders').click();
        await expect(orderRow).toContainText('1,280');
        await f.open(`/${productID}`);
        await page.getByTestId('catalog-common-price').fill('1380');
        await page.getByRole('button', { name: 'Review changes', exact: true }).click();
        await page.getByRole('button', { name: 'Confirm save', exact: true }).click();
        await expect(page.getByRole('heading', { name: 'Product saved', exact: true })).toBeVisible();
        await page.reload();
        await expect(page.getByTestId('catalog-common-price')).toHaveValue('1380');
        await action(ledger, page, 'existing order', 'click orders/reload', 'old server amount stays 1,280 after new price 1,380', async () => {
          await page.getByTestId('nav-orders').click(); await page.reload();
          await expect(orderRow).toContainText('1,280'); await expect(orderRow).not.toContainText('1,380');
          expect(f.writes).toHaveLength(1); expect(f.writes[0].resource).toBe(`products/${productID}/document`); expect(otherWrites).toEqual([]);
        });
      } finally { await saveLedger(ledger, info); }
    });
    test("CATV2 concurrent committed-stock refusal rolls back every edit and keeps the input (MOCK 409)", async ({ page }, info) => {
      const ledger: Ledger = [], c = signed.en;
      try {
        const f = await setup(page, 'en'); await f.open(`/${productID}`);
        await page.getByTestId('product-name').fill(`${c.names[0]} unsaved`);
        await page.getByRole('button', { name: c.variants, exact: true }).click();
        await page.getByTestId('catalog-stock-0').click();
        await page.getByLabel(c.total, { exact: true }).fill('6');
        await page.getByRole('button', { name: c.checkStock, exact: true }).click();
        await page.getByRole('button', { name: c.backEditor, exact: true }).click();
        await page.getByRole('button', { name: c.review, exact: true }).click();
        // Fixture preparation: another transaction commits more inventory after the user's check.
        f.detail.skus[0].committed = 7;
        await action(ledger, page, c.confirm, 'click/server 409', 'all edits refused; draft and quantity 6 remain editable', async () => {
          await page.getByRole('button', { name: c.confirm, exact: true }).click();
          await expect(page.getByRole('alert')).toBeVisible();
          expect(f.detail.name).toBe(c.names[0]); expect(f.detail.skus[0].on_hand).toBe(8); expect(f.detail.version).toBe(1);
          await page.getByRole('button', { name: c.continueEdit, exact: true }).click();
          await expect(page.getByTestId('product-name')).toHaveValue(`${c.names[0]} unsaved`);
          await page.getByRole('button', { name: c.variants, exact: true }).click(); await page.getByTestId('catalog-stock-0').click();
          await expect(page.getByLabel(c.total, { exact: true })).toHaveValue('6'); expect(f.writes).toHaveLength(1);
        });
      } finally { await saveLedger(ledger, info); }
    });
    test("CATV2 mobile density: all signed states, median 12 blocks / 100 characters, no state over 20 blocks", async ({}, info) => {
      const states = ["list", "active", "draft", "archived", "selected", "bulkReview", "bulkResult", "updated", "edit", "variants", "stock", "stockError", "media", "review", "saved"];
      const metrics = states.map((state) => ({ state, ...mobileDensity.get(state) }));
      await info.attach("catalog-v2-density", { body: JSON.stringify(metrics, null, 2), contentType: "application/json" });
      expect(states.filter((state) => !mobileDensity.has(state)), "all 15 states must be measured, never vacuous").toEqual([]);
      const values = states.map((state) => mobileDensity.get(state)!);
      const median = (key: "blocks" | "chars") => values.map((value) => value[key]).sort((a, b) => a - b)[7];
      expect(Math.max(...values.map((value) => value.blocks))).toBeLessThanOrEqual(20);
      expect(median("blocks")).toBeLessThanOrEqual(12);
      expect(median("chars")).toBeLessThanOrEqual(100);
    });
  });
}
