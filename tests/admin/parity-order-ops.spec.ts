// Purpose: C5 signed-frame acceptance on real manual-order, returns and CVS-tool routes.
// Depends on: Playwright; returns-ui's signed MOCK IdP/Go/Next harness; frozen catalog/manual/RMA DTOs.
// Used by: TestBrowserReturnsUI's direct spec registration and generated browser ownership.
// Invariants: fixtures prepare GETs only; every state change uses a real control; never refund or calculate money.
import { expect, test, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";

type Locale = "zh-TW" | "zh-CN" | "en";
const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN!;
const store = process.env.LC_BROWSER_STORE!;
const evidence = process.env.LC_BROWSER_EVIDENCE!;
const id = (suffix: string) => `90000000-0000-4000-8000-${suffix.padStart(12, "0")}`;
const market = id("100"), tote = id("101"), small = id("102");
const toteSku = id("201"), smallSku = id("202");
const rmaOpen = id("1001"), rmaClosed = id("998");
const orderOpen = process.env.LC_BROWSER_ORDER_SHIPPED!;
const orderClosed = process.env.LC_BROWSER_ORDER_PAID!;
const instant = "2026-10-28T04:00:00Z";
const copy = {
  "zh-TW": { manual: "建立訂單", items: "1 · 商品", recipient: "2 · 收件人與配送", add: "加入商品", picker: "加入商品至本單", apply: "套用至本單", cancel: "取消", select: "選取", quantity: "數量", name: "收件人", phone: "聯絡電話", delivery: "配送方式", review: "檢查訂單", returns: "退款與售後", requests: "售後申請", view: "查看售後申請", completed: "已完成的售後紀錄", lookup: "查門市編號", query: "地址、門市名稱或編號", change: "更換", code: "複製編號", all: "複製全部", copied: "已複製編號", back: "返回", pickup: "取貨門市", choose: "選擇取貨門市", returnOrder: "返回訂單", family: "全家", missing: "此編號不在門市清單：022305", legacyManual: ["顧客", "配送", "付款"], legacyReturns: "退貨管理" },
  "zh-CN": { manual: "创建订单", items: "1 · 商品", recipient: "2 · 收件人与配送", add: "添加商品", picker: "添加商品到本单", apply: "应用到本单", cancel: "取消", select: "选择", quantity: "数量", name: "收件人", phone: "联系电话", delivery: "配送方式", review: "检查订单", returns: "退款与售后", requests: "售后申请", view: "查看售后申请", completed: "已完成的售后记录", lookup: "查门店编号", query: "地址、门店名称或编号", change: "更换", code: "复制编号", all: "复制全部信息", copied: "编号已复制", back: "返回", pickup: "自提门店", choose: "选择自提门店", returnOrder: "返回订单", family: "全家", missing: "该编号未收录在门店清单中：022305", legacyManual: ["顾客", "配送", "付款"], legacyReturns: "退货管理" },
  en: { manual: "Create order", items: "1 · Items", recipient: "2 · Recipient and delivery", add: "Add products", picker: "Add products to this order", apply: "Apply to this order", cancel: "Cancel", select: "Select", quantity: "Quantity", name: "Recipient", phone: "Phone", delivery: "Delivery method", review: "Review order", returns: "Refunds and after-sales", requests: "After-sales requests", view: "View after-sales requests", completed: "Completed after-sales record", lookup: "Find store code", query: "Address, store name or code", change: "Change", code: "Copy code", all: "Copy all details", copied: "Code copied", back: "Back", pickup: "Pickup store", choose: "Choose pickup store", returnOrder: "Return to order", family: "FamilyMart", missing: "Code not in store list: 022305", legacyManual: ["Customer", "Delivery", "Payment"], legacyReturns: "Returns" },
} satisfies Record<Locale, object>;

type Click = { route: string; control: string; operation: string; expected: string; actual: string };
const ledger = new WeakMap<Page, Click[]>();
async function action(page: Page, control: string, operation: string, expected: string, run: () => Promise<void>) {
  const row = { route: new URL(page.url()).pathname, control, operation, expected, actual: "FAILED" };
  const rows = ledger.get(page) ?? [];
  rows.push(row); ledger.set(page, rows);
  await test.step(`${operation}: ${control} → ${expected}`, async () => { await run(); row.actual = "observed"; });
}
async function screenshot(page: Page, frame: string) {
  const file = path.join(evidence, "parity", `${frame.replace(":", "-")}.png`);
  await mkdir(path.dirname(file), { recursive: true });
  await page.screenshot({ path: file });
  await test.info().attach(`Figma ${frame}`, { path: file, contentType: "image/png" });
  // Read-only layout measurement, never a state-changing evaluate.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
}
async function login(page: Page) {
  await page.goto(`${origin}/en`);
  await page.getByRole("button", { name: "Sign in with identity service", exact: true }).click();
  await expect(page.getByTestId("nav-orders")).toBeVisible();
}
async function fixtures(page: Page) {
  // MOCK GET preparation only: existing strict DTOs, synthetic product facts and public store facts.
  // Identity/store authorization stays on the real Go BFF. No POST is intercepted or synthesized.
  const json = (body: unknown) => ({ status: 200, contentType: "application/json", headers: { "cache-control": "no-store" }, body: JSON.stringify(body) });
  const products = [
    { id: tote, sku: toteSku, name: "焦糖托特包", title: "焦糖", code: "A12", price: 128000 },
    { id: small, sku: smallSku, name: "日常斜背小包", title: "標準款", code: "B12", price: 98000 },
  ];
  await page.route(`**/api/stores/${store}/catalog-products?*`, (route) => route.fulfill(json({
    items: products.map((p) => ({ id: p.id, slug: p.code.toLowerCase(), name: p.name, status: "active", version: 1,
      cover_image_id: null, price_min_minor: p.price, price_max_minor: p.price, currency: "TWD", sku_count: 1,
      available: 10, keyword: p.code, inventory_tracked: true, updated_at: instant })),
    next_cursor: "", total: 2, status_counts: { draft: 0, active: 2, archived: 0 },
  })));
  for (const p of products) await page.route(`**/api/stores/${store}/products/${p.id}`, (route) => route.fulfill(json({
    id: p.id, slug: p.code.toLowerCase(), name: p.name, description: "", status: "active", version: 1,
    seo_title: "", seo_description: "", options: [], collection_ids: [], warehouse_id: null,
    skus: [{ id: p.sku, code: p.code, title: p.title, price_minor: p.price, compare_at_minor: null,
      option_values: [], version: 1, currency: "TWD", available: 10, inventory_tracked: true,
      max_per_order: null, keyword: p.code, on_hand: 10, committed: 0, weight_grams: 100,
      length_mm: 10, width_mm: 10, height_mm: 10, origin_country: "TW", customs_name: "", hs_candidate: "" }],
  })));
  await page.route(`**/api/stores/${store}/tools/orders/manual/options`, (route) => route.fulfill(json({ options: [{
    option_key: `${market}|TW|cvs_711`, market_id: market, country: "TW", delivery_code: "cvs_711", delivery_kind: "cvs_711",
    mode: "MOCK", name_hans: "门店自提", name_hant: "門市取貨", name_en: "Pickup store", currency: "TWD",
    service_version: 1, allocation_version: 1, payment_modes: ["pay_at_pickup"], pickup_selection: "buyer_entered",
  }] })));
  const line = { warehouse_id: id("300"), sku_id: toteSku, qty_registered: 1 };
  const rmas = [
    { id: rmaOpen, order_id: orderOpen, state: "REGISTERED", version: 1, reason: "收到商品有瑕疵", refund_id: null, created_at: instant, updated_at: instant,
      lines: [{ ...line, qty_received: null, qty_restock: null, qty_scrap: null }] },
    { id: rmaClosed, order_id: orderClosed, state: "CLOSED", version: 4, reason: "買家申請", refund_id: null, created_at: instant, updated_at: instant,
      lines: [{ ...line, qty_received: 1, qty_restock: 1, qty_scrap: 0 }] },
  ];
  await page.route(new RegExp(`/api/stores/${store}/returns(?:\\?.*)?$`), (route) => {
    const state = new URL(route.request().url()).searchParams.get("state");
    return route.fulfill(json({ items: rmas.filter((rma) => !state || rma.state === state) }));
  });
}

test.describe("PARITY-ORDER-OPS signed frames (BROWSER MOCK)", () => {
  test.afterEach(async ({ page }, info) => {
    await mkdir(path.join(evidence, "parity"), { recursive: true });
    const file = path.join(evidence, "parity", `clicks-${info.testId.replace(/[^a-z0-9-]/gi, "-")}.json`);
    await writeFile(file, JSON.stringify({ test: info.title, status: info.status, clicks: ledger.get(page) ?? [] }, null, 2));
    await info.attach("click ledger", { path: file, contentType: "application/json" });
  });

  for (const locale of ["zh-TW", "zh-CN", "en"] as const) for (const mobile of [false, true]) {
    const c = copy[locale];
    const size = mobile ? "mobile" : "desktop";
    test(`manual-order ${locale} ${size}: select, quantity, cancel isolation and all signed draft states`, async ({ page }) => {
      await page.setViewportSize({ width: mobile ? 390 : 1440, height: mobile ? 844 : 900 });
      await login(page); await fixtures(page);
      const posts: string[] = [];
      page.on("request", (r) => { if (r.method() === "POST" && r.url().includes("/tools/orders/manual")) posts.push(r.url()); });
      await page.goto(`${origin}/${locale}/orders/new?store=${store}`);
      // First failure on the pre-Figma implementation must be this signed, visible heading assertion.
      await expect(page.getByRole("heading", { name: c.items, exact: true })).toBeVisible();
      await expect(page.getByRole("heading", { name: c.recipient, exact: true })).toBeVisible();
      for (const heading of c.legacyManual) await expect(page.getByRole("heading", { name: heading, exact: true })).toHaveCount(0);
      const openPicker = async () => action(page, c.add, "click", c.picker, async () => {
        await page.getByRole("button", { name: c.add, exact: true }).click();
        await expect(page.getByRole("heading", { name: c.picker, exact: true })).toBeVisible();
      });
      await openPicker();
      const select = (name: string) => page.getByRole("button", { name: new RegExp(`${c.select}.*${name}`) });
      await action(page, "焦糖托特包", "click + apply", "one tote in draft", async () => {
        await select("焦糖|托特包").click();
        await page.getByRole("button", { name: c.apply, exact: true }).click();
        await expect(page.getByRole("spinbutton", { name: new RegExp(c.quantity) }).first()).toHaveValue("1");
      });
      if (locale === "zh-TW") await screenshot(page, mobile ? "750:3457" : "750:3249");
      await action(page, c.quantity, "fill", "two totes", async () => {
        await page.getByRole("spinbutton", { name: new RegExp(c.quantity) }).first().fill("2");
        await expect(page.getByRole("spinbutton", { name: new RegExp(c.quantity) }).first()).toHaveValue("2");
      });
      if (locale === "zh-TW") await screenshot(page, mobile ? "21:967" : "21:582");
      await openPicker();
      if (locale === "zh-TW") await screenshot(page, mobile ? "750:3595" : "750:3453");
      await action(page, c.cancel, "click", "picker closes and quantity stays two", async () => {
        await page.getByRole("button", { name: c.cancel, exact: true }).click();
        await expect(page.getByRole("spinbutton", { name: new RegExp(c.quantity) }).first()).toHaveValue("2");
      });
      await openPicker();
      await action(page, "日常斜背小包", "click + apply", "two distinct SKU lines", async () => {
        await select("小包").click(); await page.getByRole("button", { name: c.apply, exact: true }).click();
        await expect(page.getByRole("spinbutton", { name: new RegExp(c.quantity) })).toHaveCount(2);
      });
      if (locale === "zh-TW") await screenshot(page, mobile ? "750:3503" : "750:3317");
      await page.getByRole("spinbutton", { name: new RegExp(c.quantity) }).first().fill("1");
      if (locale === "zh-TW") await screenshot(page, mobile ? "750:3549" : "750:3385");
      await expect(page.getByRole("button", { name: c.review, exact: true })).toBeVisible();
      // None of the draft/picker controls may place an order or reserve stock.
      expect(posts).toEqual([]);
    });

    test(`returns ${locale} ${size}: signed home, requests, filter refresh and record navigation`, async ({ page }) => {
      await page.setViewportSize({ width: mobile ? 390 : 1440, height: mobile ? 844 : 900 });
      await login(page); await fixtures(page);
      const moneyWrites: string[] = [];
      page.on("request", (r) => { if (r.method() !== "GET" && /refund|returns/.test(new URL(r.url()).pathname)) moneyWrites.push(r.method()); });
      await page.goto(`${origin}/${locale}/returns?store=${store}`);
      await expect(page.getByRole("heading", { name: c.returns, exact: true })).toBeVisible();
      await expect(page.getByRole("heading", { name: c.legacyReturns, exact: true })).toHaveCount(0);
      if (locale === "zh-TW") await screenshot(page, mobile ? "6:20626" : "6:20488");
      await action(page, c.view, "click", c.requests, async () => {
        await page.getByRole("button", { name: c.view, exact: true }).click();
        await expect(page.getByRole("heading", { name: c.requests, exact: true })).toBeVisible();
      });
      if (locale === "zh-TW") await screenshot(page, mobile ? "30:1731" : "30:1662");
      await action(page, "open RMA", "click", "registered request details", async () => {
        await page.getByTestId(`returns-row-${rmaOpen}`).getByRole("link").click();
        await expect(page.getByText("收到商品有瑕疵", { exact: false })).toBeVisible();
      });
      if (locale === "zh-TW") await screenshot(page, mobile ? "30:1818" : "30:1757");
      await page.goBack();
      await action(page, "closed-state filter", "click + refresh", "closed filter and closed row survive reload", async () => {
        await page.getByTestId("returns-filter-closed").click();
        await page.reload();
        await expect(page.getByTestId("returns-filter-closed")).toHaveAttribute("aria-current", "true");
        await expect(page.getByTestId(`returns-row-${rmaOpen}`)).toHaveCount(0);
        await expect(page.getByTestId(`returns-row-${rmaClosed}`)).toBeVisible();
      });
      await page.getByTestId(`returns-row-${rmaClosed}`).getByRole("link").click();
      await expect(page.getByText(c.completed, { exact: true })).toBeVisible();
      if (locale === "zh-TW") await screenshot(page, mobile ? "95:2918" : "95:2858");
      expect(moneyWrites).toEqual([]);
    });

    test(`merchant CVS ${locale} ${size}: explicit match, copy, change, labeled paste and missing code`, async ({ page, context }) => {
      await page.setViewportSize({ width: mobile ? 390 : 1440, height: mobile ? 844 : 900 });
      await login(page);
      await context.grantPermissions(["clipboard-read", "clipboard-write"]);
      await page.goto(`${origin}/${locale}/orders/cvs-match?store=${store}`);
      await expect(page.getByRole("heading", { name: c.lookup, exact: true })).toBeVisible();
      const query = () => page.getByRole("textbox", { name: c.query, exact: true });
      const choose = (code: string) => page.getByRole("button", { name: new RegExp(code) });
      const shot = async (desktop: string, phone: string) => { if (locale === "zh-TW") await screenshot(page, mobile ? phone : desktop); };
      await action(page, c.query, "fill", "two unselected public address candidates", async () => {
        await query().fill("敦化南路");
        await expect(choose("121275")).toBeVisible(); await expect(choose("127620")).toBeVisible();
        await expect(page.getByRole("button", { name: c.code, exact: true })).toHaveCount(0);
      });
      await shot("1431:70856", "1431:70881");
      await choose("127620").click();
      await expect(page.getByText("臺北市大安區敦化南路一段187巷29號", { exact: true })).toBeVisible();
      await shot("1431:70774", "1431:70793");
      await page.getByRole("button", { name: c.change, exact: true }).click();
      await query().fill("敦化南路"); await choose("121275").click();
      await expect(page.getByText("臺北市大安區敦化南路二段331巷14號", { exact: true })).toBeVisible();
      await shot("1431:70906", "1431:70922");
      await action(page, c.code, "click", "clipboard contains exact string code", async () => {
        await page.getByRole("button", { name: c.code, exact: true }).click();
        await expect(page.getByText(c.copied, { exact: true })).toBeVisible();
        // Read-only native clipboard observation after the real copy button.
        expect(await page.evaluate(() => navigator.clipboard.readText())).toBe("121275");
      });
      await shot("1431:71280", "1431:71320");
      await page.getByRole("button", { name: c.back, exact: true }).click();
      await page.getByRole("button", { name: c.all, exact: true }).click();
      const copied = await page.evaluate(() => navigator.clipboard.readText());
      expect(copied).toContain("121275"); expect(copied).toContain("臺北市大安區敦化南路二段331巷14號");
      await page.getByRole("button", { name: c.change, exact: true }).click();
      await query().fill("7-11门市代号 231196\n收件门市名称 久旺門市\n门市地址 高雄市大樹區九曲里新鎮路1號1樓");
      await expect(choose("231196")).toBeVisible();
      await shot("1431:71405", "1431:71485");
      await choose("231196").click();
      await expect(page.getByText("高雄市大樹區九曲里新鎮路1號1樓", { exact: true })).toBeVisible();
      await shot("1431:71427", "1431:71507");
      await page.getByRole("button", { name: c.change, exact: true }).click();
      await page.getByRole("button", { name: c.family, exact: true }).click();
      await query().fill("全家门市代号 F022305\n门市名称 全家泰山新輔醫店\n门市地址 新北市泰山區貴子路69號B1");
      await expect(page.getByText(c.missing, { exact: true })).toBeVisible();
      await expect(choose("024967")).toBeVisible();
      await expect(page.getByRole("button", { name: c.code, exact: true })).toHaveCount(0);
      await shot("1431:71443", "1431:71523");
      await choose("024967").click();
      await expect(page.getByText("024967", { exact: true })).toBeVisible();
      await expect(page.getByText("新北市泰山區貴子路69號B1", { exact: true })).toBeVisible();
      await shot("1431:71469", "1431:71549");
    });

    test(`manual CVS ${locale} ${size}: choose store and return canonical facts to the draft`, async ({ page }) => {
      await page.setViewportSize({ width: mobile ? 390 : 1440, height: mobile ? 844 : 900 });
      await login(page); await fixtures(page);
      await page.goto(`${origin}/${locale}/orders/new?store=${store}`);
      await expect(page.getByRole("heading", { name: c.recipient, exact: true })).toBeVisible();
      await page.getByRole("combobox", { name: c.delivery, exact: true }).selectOption(`${market}|TW|cvs_711`);
      await page.getByRole("button", { name: c.choose, exact: true }).click();
      await page.getByRole("textbox", { name: c.query, exact: true }).fill("敦化南路");
      await expect(page.getByRole("button", { name: /121275/ })).toBeVisible();
      if (locale === "zh-TW") await screenshot(page, mobile ? "1431:71001" : "1431:70976");
      await page.getByRole("button", { name: /121275/ }).click();
      await expect(page.getByRole("button", { name: c.returnOrder, exact: true })).toBeVisible();
      if (locale === "zh-TW") await screenshot(page, mobile ? "1431:71036" : "1431:71026");
      await action(page, c.returnOrder, "click", "canonical store facts returned to the order without submitting it", async () => {
        await page.getByRole("button", { name: c.returnOrder, exact: true }).click();
        await expect(page.getByText(/7-ELEVEN.*樂隆.*121275/)).toBeVisible();
        await expect(page.getByText("臺北市大安區敦化南路二段331巷14號", { exact: true })).toBeVisible();
      });
      if (locale === "zh-TW") await screenshot(page, mobile ? "1431:70957" : "1431:70938");
    });
  }
});
