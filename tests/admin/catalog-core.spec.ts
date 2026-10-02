// CC12 (contracts/storefront-v2.md section A acceptance CC12): the catalog v2 admin pages, merchant side, in real Chromium.
// BFF routes exercised through the UI: GET catalog-products, GET|POST|PATCH products*, POST skus, PATCH skus/{id}, POST skus/{id}/price,
// POST skus/{id}/archive, GET warehouses, GET catalog-ledger, POST inventory/adjustments, products/{id}/images, GET|POST|PATCH collections*,
// PUT collections/{id}/products, POST|GET collections/{id}/image -> Go /v1/admin/stores/{store}/... (internal/httpapi, internal/catalog).
// Started only by tests/foundation/browser_catalog_core_test.go (build tag browser), which owns the isolated PG, the real Go API, the production
// admin Next build, the signed mock IdP and the runner-only control listener. The control listener answers "what does a SHOPPER see" through the
// real buyer catalog v2 HTTP handler (Node never holds the BFF key or a database credential).
//
// Two runs, each with its own product: desktop 1586x992 in en and 390 px in zh-TW. Each run: create a draft product, two option axes (顏色 x 尺寸)
// -> a 2x3 matrix of six variants, prices, compare-at on one, stock on four, a photo, a new collection with its image and the product in it,
// activate, verify the shopper view (price range, compare-at, stock hints, cover, collection), archive one variant, deactivate, verify it is gone.
// Locators use the UI unit's data-testid vocabulary; wording comes from its copy files only for the nav entries.
import { expect, test, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { catalogCopy } from "../../apps/admin/lib/catalog-v2-copy";
import { copy } from "../../apps/admin/lib/copy";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_STORE");
const tag = required("LC_BROWSER_TAG");
const control = required("LC_BROWSER_CONTROL");
const controlKey = required("LC_BROWSER_CONTROL_KEY");

test.use({ baseURL: origin, trace: "retain-on-failure", screenshot: "only-on-failure" });
test.describe.configure({ mode: "serial" });

// the base64 is split so the G04/CI secret grep (Meta "EAA…" token shape) never matches PNG bytes
const png = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB" + "CAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==", "base64");
const manifestPath = path.join(evidence, "screenshots.json");

async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("nav-orders")).toBeAttached();
}
async function fitsWidth(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth), "no horizontal page scroll").toBeLessThanOrEqual(1);
}
async function shot(page: Page, name: string, locale: string, viewport: "desktop" | "mobile") {
  const file = path.join(evidence, `catalog-${name}-${locale}-${viewport}.png`);
  await page.screenshot({ path: file, fullPage: false, animations: "disabled" });
  await fitsWidth(page);
  let manifest: unknown[] = [];
  try {
    manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  } catch {
    /* first screenshot */
  }
  manifest.push({ File: path.basename(file), Sha256: createHash("sha256").update(await readFile(file)).digest("hex"), Locale: locale, Viewport: viewport });
  await writeFile(manifestPath, JSON.stringify(manifest, null, 2));
}

// The basics form remounts after a successful save (its key carries the product version), so its own "Saved" text is not a stable signal:
// wait for the PATCH the BFF forwards to Go and require 200 instead.
async function saveBasics(page: Page) {
  const done = page.waitForResponse((r) => r.request().method() === "PATCH" && /\/products\/[0-9a-f-]{36}$/.test(new URL(r.url()).pathname));
  await page.getByTestId("product-save").click();
  expect((await done).status(), "PATCH product through the BFF").toBe(200);
}

type Shop = { status: number; body: any; type: string; cache: string; length: number };
// What a shopper gets from the real buyer catalog v2 / media routes (control listener = runner-only proxy holding the BFF key).
async function shop(p: string): Promise<Shop> {
  const response = await fetch(`${control}/?path=${encodeURIComponent(p)}`, { headers: { "X-Gate-Key": controlKey } });
  expect(response.status, "control listener").toBe(200);
  const j = await response.json();
  return { status: j.status, body: j.body ? JSON.parse(j.body) : null, type: j.contentType, cache: j.cacheControl, length: j.length };
}
const list = (q: string, extra = "") => shop(`/v1/buyer/catalog/v2/products?q=${encodeURIComponent(q)}${extra}`);

// LC_BROWSER_DIAGNOSTIC (never set by the gate) works around known product defects (DEFECTS.md) one at a time, to look at what lies behind them:
// a comma list of `codes` (P1-2), `stock` (P1-3), `image` (P2-4); `all` = every one.
const diagnostic = new Set((process.env.LC_BROWSER_DIAGNOSTIC ?? "").split(",").flatMap((x) => (x === "all" ? ["codes", "stock", "image"] : [x])));

const variants = [
  { key: "d", locale: "en", viewport: { width: 1586, height: 992 }, vp: "desktop" as const },
  { key: "m", locale: "zh-TW", viewport: { width: 390, height: 844 }, vp: "mobile" as const },
];

for (const v of variants) {
  test.describe(`CC12 ${v.vp} ${v.locale}`, () => {
    test.use({ viewport: v.viewport, locale: v.locale === "en" ? "en-US" : "zh-TW" });

    test(`catalog v2: draft product, 2x3 matrix, prices, compare-at, stock, photo, collection, activate, shopper view, archive a variant, deactivate (${v.vp}, ${v.locale})`, async ({ page }) => {
      test.setTimeout(420_000);
      const L = v.locale;
      const cc = catalogCopy[L as "en" | "zh-TW"];
      const uniq = `${tag}${v.key}`; // lower-case hex + letter: the product name, slug and collection title all start with it
      const url = (p: string) => `/${L}${p}${p.includes("?") ? "&" : "?"}store=${store}`;
      const writes: Array<{ method: string; url: string; key: string | null }> = [];
      page.on("request", (r) => {
        if (r.url().includes("/api/stores/") && r.method() !== "GET") writes.push({ method: r.method(), url: new URL(r.url()).pathname, key: r.headers()["idempotency-key"] ?? null });
      });
      await signedLogin(page);

      // ---- nav entries and the products list (CC12: nav Products / Collections / Inventory, ledger stays reachable) ----
      await page.goto(url("/products"));
      await expect(page.getByTestId("products-page")).toBeVisible();
      await expect(page.getByTestId("product-new")).toBeVisible();
      await expect(page.getByTestId("products-ledger-link")).toBeAttached();
      // stop-bleed D03: the "stock ledger" link opens the inventory page, not the dashboard at /{locale}/
      await expect(page.getByTestId("products-ledger-link")).toHaveAttribute("href", new RegExp(`^/${L}/inventory(\\?store=[0-9a-f-]{36})?$`));
      if (v.vp === "mobile") await page.getByRole("button", { name: copy[L as "en" | "zh-TW"].menu }).click();
      const rail = page.getByRole("navigation").first();
      for (const label of [copy[L as "en" | "zh-TW"].products, cc.nav.collections, copy[L as "en" | "zh-TW"].inventory])
        await expect(rail.getByRole("button", { name: label, exact: true }), `nav entry ${label}`).toBeVisible();
      await rail.getByRole("button", { name: copy[L as "en" | "zh-TW"].inventory, exact: true }).click();
      await expect(page).toHaveURL(new RegExp(`/${L}/inventory/?(\\?.*)?$`)); // the ledger is still reachable (0094 moved it from home to /inventory)
      await expect(page.getByTestId("nav-orders")).toBeAttached();
      await page.goto(url("/products"));
      await expect(page.getByTestId("products-page")).toBeVisible();
      await shot(page, "list-start", L, v.vp);

      // ---- create: starts as a draft, slug generated from the title ----
      await page.getByTestId("product-new").click();
      await expect(page).toHaveURL(/\/products\/new/);
      await expect(page.getByTestId("product-create-form")).toBeVisible();
      await page.getByTestId("product-name").fill(`${uniq} Linen Tee`);
      await page.getByTestId("product-description").fill("Soft linen. <b>not bold</b>");
      await shot(page, "create", L, v.vp);
      await page.getByTestId("product-create").click();
      await page.waitForURL(/\/products\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/);
      const productId = /\/products\/([0-9a-f-]{36})/.exec(page.url())![1];
      await expect(page.getByTestId("product-form")).toBeVisible();
      await expect(page.getByTestId("product-status")).toHaveValue("draft");
      await expect(page.getByTestId("product-slug")).toHaveValue(`${uniq}-linen-tee`);
      await expect(page.locator(".product-editor b"), "no raw HTML from merchant text").toHaveCount(0);
      // SEO counters (character counts against the 70 / 160 limits)
      await page.getByTestId("product-seo-title").fill("Linen tee, soft");
      await page.getByTestId("product-seo-description").fill("A soft linen tee for summer.");
      await expect(page.locator(".product-seo .product-count").first()).toContainText("15");
      await expect(page.locator(".product-seo .product-count").first()).toContainText("70");
      await expect(page.locator(".product-seo .product-count").nth(1)).toContainText("28");
      await expect(page.locator(".product-seo .product-count").nth(1)).toContainText("160");
      await saveBasics(page);
      await expect(page.getByTestId("product-seo-title")).toHaveValue("Linen tee, soft");
      // a draft is invisible to shoppers
      expect((await list(uniq)).body.products, "draft in the buyer list").toHaveLength(0);
      expect((await shop(`/v1/buyer/catalog/v2/products/${productId}`)).status, "draft detail").toBe(404);

      // ---- option axes 顏色 x 尺寸 -> a 2 x 3 matrix ----
      await page.getByTestId("axis-add").click();
      await page.getByTestId("axis-name-0").fill("顏色");
      await page.getByTestId("axis-values-0").fill("紅, 藍");
      await page.getByTestId("axis-add").click();
      await page.getByTestId("axis-name-1").fill("尺寸");
      await page.getByTestId("axis-values-1").fill("S, M, L");
      // at most three axes (contract A): the add button stops at three; the third row is removed again
      await page.getByTestId("axis-add").click();
      await expect(page.getByTestId("axis-name-2")).toBeVisible();
      await expect(page.getByTestId("axis-add")).toBeDisabled();
      await page.locator(".product-axis").nth(2).getByRole("button").click();
      await expect(page.getByTestId("axis-name-2")).toHaveCount(0);
      await page.getByTestId("axis-save").click();
      await expect(page.getByTestId("new-variants")).toBeVisible();
      const newRows = page.locator('[data-testid^="new-variant-"]');
      await expect(newRows).toHaveCount(6);
      const titles: string[] = [];
      for (let i = 0; i < 6; i++) titles.push((await page.getByTestId(`new-variant-${i}`).locator("td").first().innerText()).trim());
      expect([...titles].sort(), "the 2x3 matrix of values joined by ' / '").toEqual(["紅 / S", "紅 / M", "紅 / L", "藍 / S", "藍 / M", "藍 / L"].sort());
      // server price validation is shown, not guessed: an empty price blocks the create
      await page.getByTestId("new-variants-create").click();
      await expect(page.getByTestId("new-variants-message")).toBeVisible();
      const price = (i: number) => 10 + 2 * i; // 10, 12 ... 20 in major units = 1000 ... 2000 minor
      for (let i = 0; i < 6; i++) await page.getByTestId(`new-price-${i}`).fill(String(price(i)));
      // Diagnostic switch only (LC_BROWSER_DIAGNOSTIC=codes, never set by the gate): type unique SKU codes, to look past defect P1-2 (default
      // codes of non-ASCII option values collide, SKU codes are unique per store). The gate itself uses the codes the editor proposes.
      if (diagnostic.has("codes")) for (let i = 0; i < 6; i++) await page.getByTestId(`new-code-${i}`).fill(`${uniq}-v${i}`);
      await shot(page, "matrix", L, v.vp);
      await page.getByTestId("new-variants-create").click();
      const rows = page.locator('[data-testid^="variant-row-"]');
      await expect(rows).toHaveCount(6);
      const ids: string[] = [];
      const idByTitle: Record<string, string> = {};
      for (let i = 0; i < 6; i++) {
        const id = (await rows.nth(i).getAttribute("data-testid"))!.replace("variant-row-", "");
        ids.push(id);
        idByTitle[(await rows.nth(i).locator("td").first().innerText()).trim()] = id;
      }
      const priceOf = (title: string) => 1000 + 200 * titles.indexOf(title);
      for (const t of titles) await expect(page.getByTestId(`variant-price-${idByTitle[t]}`)).toHaveValue(`${priceOf(t) / 100}.00`);
      expect((await list(uniq)).body.products, "still draft").toHaveLength(0);

      // ---- compare-at: must exceed the price (shown, nothing saved), then valid on one variant ----
      const cheapest = idByTitle[titles[0]];
      await page.getByTestId(`variant-compare-${cheapest}`).fill("5");
      await page.getByTestId(`variant-save-${cheapest}`).click();
      await expect(page.getByTestId("variants-message")).toBeVisible();
      await page.getByTestId(`variant-compare-${cheapest}`).fill("25");
      await page.getByTestId(`variant-save-${cheapest}`).click();
      await expect(page.getByTestId(`variant-compare-${cheapest}`)).toHaveValue("25.00");
      await expect(page.getByTestId(`variant-price-${cheapest}`)).toHaveValue("10.00");

      // ---- stock: 10 / 3 / 0 / 6 / 5 / 0 (in, low, out, in, low, out), the delta must be a non-zero integer ----
      const stock = [10, 3, 0, 6, 5, 0];
      await page.getByTestId(`variant-adjust-${idByTitle[titles[0]]}`).click();
      await page.getByTestId("adjust-delta").fill("abc");
      await page.getByTestId("adjust-reason").fill("opening stock");
      await page.getByTestId("adjust-apply").click();
      await expect(page.getByTestId("variants-message")).toBeVisible();
      for (let i = 0; i < 6; i++) {
        if (stock[i] === 0) continue;
        const id = idByTitle[titles[i]];
        if (!(await page.getByTestId(`adjust-form-${id}`).count())) await page.getByTestId(`variant-adjust-${id}`).click();
        await page.getByTestId("adjust-delta").fill(String(stock[i]));
        await page.getByTestId("adjust-reason").fill("opening stock");
        const answered = page.waitForResponse((r) => r.url().includes("/inventory/adjustments"));
        await page.getByTestId("adjust-apply").click();
        expect((await answered).status(), "stock adjustment through the BFF").toBe(200);
        if (diagnostic.has("stock")) await page.reload(); // defect P1-3: the editor does not refresh after a successful adjustment
        await expect(page.getByTestId(`variant-stock-${id}`)).toContainText(String(stock[i]));
        await expect(page.getByTestId(`adjust-form-${id}`)).toHaveCount(0);
      }

      // ---- photo ----
      await page.getByTestId("photo-input").setInputFiles({ name: "tee.png", mimeType: "image/png", buffer: png });
      await expect(page.getByTestId("photo-row")).toHaveCount(1);
      await shot(page, "editor", L, v.vp);

      // ---- a new collection with an image, the (still draft) product added ----
      await page.goto(url("/collections"));
      await expect(page.getByTestId("collections-page")).toBeVisible();
      await page.getByTestId("collection-new").click();
      await expect(page.getByTestId("collection-editor")).toBeVisible();
      await page.getByTestId("collection-title").fill(`${uniq} Summer Edit`);
      await page.getByTestId("collection-description").fill("Fresh for summer");
      await page.getByTestId("collection-save").click();
      await expect(page.getByTestId("collection-members")).toBeVisible();
      await expect(page.getByTestId("collection-slug")).toHaveValue(`${uniq}-summer-edit`);
      const collectionSlug = `${uniq}-summer-edit`;
      await page.getByTestId("collection-image-input").setInputFiles({ name: "summer.png", mimeType: "image/png", buffer: png });
      await expect(page.getByTestId("collection-image").locator("img")).toBeVisible();
      // the preview must actually load through the BFF (defect P2-4: the editor asks for `image?v=<id>` and the BFF refuses any query)
      if (!diagnostic.has("image"))
        await expect
          .poll(() => page.getByTestId("collection-image").locator("img").evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0), { message: "collection image preview loads" })
          .toBe(true);
      await page.getByTestId("member-search").fill(uniq);
      await page.getByTestId(`member-add-${productId}`).click();
      await expect(page.getByTestId(`member-${productId}`)).toBeVisible();
      await page.getByTestId("members-save").click();
      await expect(page.getByTestId("members-save")).toBeDisabled(); // saved: nothing left to save
      const collectionItem = page.locator(".collections-list li button", { hasText: collectionSlug });
      await expect(collectionItem).toHaveCount(1);
      const collectionId = (await collectionItem.getAttribute("data-testid"))!.replace("collection-item-", "");
      await shot(page, "collection", L, v.vp);
      // the collection is active, its only member is a draft: shoppers see the collection with 0 products
      const collections = await shop("/v1/buyer/catalog/v2/collections");
      expect(collections.status).toBe(200);
      expect(collections.body.collections.find((c: any) => c.slug === collectionSlug), "active collection listed").toMatchObject({ product_count: 0 });

      // ---- activate ----
      await page.goto(url(`/products/${productId}`));
      await page.getByTestId("product-status").selectOption("active");
      await expect(page.getByTestId("product-status-help")).toContainText(cc.statusHelp.active);
      await saveBasics(page);
      await expect(page.getByTestId("product-status")).toHaveValue("active");

      // ---- the shopper view (real buyer catalog v2 HTTP): price range, compare-at, stock hints, cover, collection ----
      const found = await list(uniq);
      expect(found.status).toBe(200);
      expect(found.body.products).toHaveLength(1);
      const card = found.body.products[0];
      expect(card).toMatchObject({ id: productId, slug: `${uniq}-linen-tee`, title: `${uniq} Linen Tee`, price_min_minor: 1000, price_max_minor: 2000, compare_at_min_minor: 2500, in_stock: true });
      expect(card.cover_image_id, "the first photo is the cover").toBeTruthy();
      const detailBySlug = await shop(`/v1/buyer/catalog/v2/products/${card.slug}`);
      const detailById = await shop(`/v1/buyer/catalog/v2/products/${productId}`);
      expect(detailBySlug.status).toBe(200);
      expect(detailById.body).toEqual(detailBySlug.body);
      const d = detailBySlug.body;
      expect(d.description).toBe("Soft linen. <b>not bold</b>"); // plain text, stored as typed (the storefront escapes it)
      expect(d.seo).toEqual({ title: "Linen tee, soft", description: "A soft linen tee for summer." });
      expect(d.options).toEqual([{ name: "顏色", values: ["紅", "藍"] }, { name: "尺寸", values: ["S", "M", "L"] }]);
      expect(d.variants).toHaveLength(6);
      const hint = (n: number) => (n >= 6 ? "in" : n >= 1 ? "low" : "out");
      for (const [i, t] of titles.entries()) {
        const variant = d.variants.find((x: any) => x.sku_id === idByTitle[t]);
        expect(variant, `variant ${t}`).toMatchObject({ title: t, option_values: t.split(" / "), price_minor: priceOf(t), stock: hint(stock[i]), compare_at_minor: i === 0 ? 2500 : null });
      }
      expect(d.collections).toEqual([{ slug: collectionSlug, title: `${uniq} Summer Edit` }]);
      expect(JSON.stringify(d), "no raw stock counts for shoppers").not.toMatch(/on_hand|available|reserved|"quantity"/);
      const inCollection = await shop(`/v1/buyer/catalog/v2/products?collection=${collectionSlug}`);
      expect(inCollection.body.products.map((p: any) => p.id)).toEqual([productId]);
      const collectionCard = (await shop("/v1/buyer/catalog/v2/collections")).body.collections.find((c: any) => c.slug === collectionSlug);
      expect(collectionCard).toMatchObject({ product_count: 1, title: `${uniq} Summer Edit` });
      expect(collectionCard.image_id).toBeTruthy();
      const productMedia = await shop(`/v1/buyer/media/p/${productId}/${card.cover_image_id}`);
      expect(productMedia).toMatchObject({ status: 200, type: "image/png" });
      expect(productMedia.cache).toContain("immutable");
      const collectionMedia = await shop(`/v1/buyer/media/c/${collectionId}/${collectionCard.image_id}`);
      expect(collectionMedia).toMatchObject({ status: 200, type: "image/png" });
      expect(collectionMedia.cache).toContain("immutable");

      // ---- the merchant list reflects it: status, price range, stock, cover thumbnail, search, status filter ----
      await page.goto(url("/products"));
      await page.getByTestId("products-search").fill(uniq);
      await page.getByTestId("products-search-submit").click();
      const row = page.getByTestId(`product-row-${productId}`);
      await expect(row).toBeVisible();
      await expect(row).toContainText(cc.status.active);
      await expect(row).toContainText("10");
      await expect(row).toContainText("20");
      await expect(row).toContainText("24"); // 10+3+0+6+5+0 units
      // the cover thumbnail is the first photo as a CSS background (data-photo="real"); its URL must serve the image through the BFF
      const cover = row.locator('[data-photo="real"]').first();
      await expect(cover).toBeVisible();
      const coverFetch = await cover.evaluate(async (el: Element) => {
        const m = /url\("?([^")]+)"?\)/.exec(getComputedStyle(el).backgroundImage);
        if (!m) return { status: 0, type: "" };
        const r = await fetch(m[1], { credentials: "same-origin" });
        return { status: r.status, type: r.headers.get("content-type") ?? "" };
      });
      expect(coverFetch, "cover thumbnail is served through the BFF").toEqual({ status: 200, type: "image/png" });
      await page.getByTestId("products-status").selectOption("draft");
      await expect(page.getByTestId(`product-row-${productId}`)).toHaveCount(0);
      await page.getByTestId("products-status").selectOption("active");
      await expect(page.getByTestId(`product-row-${productId}`)).toBeVisible();
      await page.getByTestId("products-status").selectOption("all");
      await expect(page.getByTestId(`product-row-${productId}`)).toBeVisible();
      await shot(page, "list-active", L, v.vp);

      // ---- paging: the seeded drafts (60) walk 25 + 25 + 10 ----
      await page.getByTestId("products-search").fill(`${tag} paging`);
      await page.getByTestId("products-search-submit").click();
      const productRows = page.locator('[data-testid^="product-row-"]');
      await expect(productRows).toHaveCount(25);
      await expect(page.getByTestId("products-next")).toBeEnabled();
      await page.getByTestId("products-next").click();
      await expect(productRows).toHaveCount(25);
      await page.getByTestId("products-next").click();
      await expect(productRows).toHaveCount(10);
      await expect(page.getByTestId("products-next")).toBeDisabled();
      await expect(page.getByTestId("products-previous")).toBeEnabled();
      await fitsWidth(page);

      // ---- archive one variant: it leaves the shopper view, the range follows ----
      await page.goto(url(`/products/${productId}`));
      await page.getByTestId(`variant-archive-${cheapest}`).click();
      await page.getByTestId(`variant-row-${cheapest}`).locator("button.danger").click();
      await expect(rows).toHaveCount(5);
      const afterArchive = (await list(uniq)).body.products[0];
      expect(afterArchive).toMatchObject({ id: productId, price_min_minor: 1200, price_max_minor: 2000, compare_at_min_minor: null, in_stock: true });
      const dArchived = (await shop(`/v1/buyer/catalog/v2/products/${productId}`)).body;
      expect(dArchived.variants).toHaveLength(5);
      expect(dArchived.variants.map((x: any) => x.sku_id)).not.toContain(cheapest);
      await shot(page, "archived-variant", L, v.vp);

      // ---- deactivate: the product disappears for shoppers everywhere ----
      await page.getByTestId("product-status").selectOption("draft");
      await saveBasics(page);
      await expect(page.getByTestId("product-status")).toHaveValue("draft");
      expect((await list(uniq)).body.products, "deactivated product in the buyer list").toHaveLength(0);
      expect((await shop(`/v1/buyer/catalog/v2/products/${productId}`)).status).toBe(404);
      expect((await shop(`/v1/buyer/catalog/v2/products/${card.slug}`)).status).toBe(404);
      expect((await shop(`/v1/buyer/catalog/v2/products?collection=${collectionSlug}`)).body.products).toHaveLength(0);
      expect((await shop("/v1/buyer/catalog/v2/collections")).body.collections.find((c: any) => c.slug === collectionSlug)).toMatchObject({ product_count: 0 });
      expect((await shop(`/v1/buyer/media/p/${productId}/${card.cover_image_id}`)).status).toBe(404);
      await shot(page, "deactivated", L, v.vp);

      // ---- every write carried a well-formed idempotency key ----
      expect(writes.length, "every UI write goes through the BFF").toBeGreaterThanOrEqual(18);
      for (const w of writes) expect(w.key, `${w.method} ${w.url}`).toMatch(/^[A-Za-z0-9_.:-]{8,128}$/);
    });
  });
}
