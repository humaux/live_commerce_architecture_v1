// Purpose: real-click live-price claim checkout versus ordinary catalog-price purchase regression.
// Depends on: production storefront/admin, real PG and signed MOCK Meta/IdP/provider fixtures.
// Used by: --browser-e2e TestBrowserLiveTools; no LIVE provider acceptance.
// Live tools (R4) browser gate: keyword library import + live-only price, claimed through a signed MOCK Meta comment, bought with pay at pickup
// at the live price, while the same SKU bought directly from the product page pays the normal price. Written from the contract
// (contracts/live-keyword-claims-v1.md, amendment "Live tools (R4)") by the independent test author. Started only by
// tests/foundation/browser_live_tools_test.go (TestBrowserLiveTools), which owns every process (production admin + storefront Next builds, Go APIs,
// PostgreSQL, Meta consumer/poller/dispatcher, fake Graph, signed MOCK IdP, the disposable buyer.example TLS edge + CONNECT proxy) and the
// runner-only control listener. The browser never touches Go directly; things the outside world does (a Meta comment arrives) and every PostgreSQL
// assertion go through POST {control}/act?name=... which Go executes on its test goroutine.
//
// Matrix (four cells, one scene/post/SKU each, one stack): zh-TW x desktop (1440), zh-TW x 390 px, en x desktop, en x 390 px. Merchant and buyer both
// run at the cell's viewport. BFF routes exercised: /api/stores/{store}/live-sessions (+ claims library, offer-import, offers, window, claim-source) on
// the admin side; /api/buyer/session, cart, quotes, destinations, cvs-stores, orders on the storefront side.
import { expect, test, type BrowserContext, type Locator, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { claimsCopy, hostPrompt } from "../../apps/admin/lib/claims-copy";
import { studioCopy } from "../../apps/admin/lib/studio-copy";
import { claimCopy } from "../../apps/storefront/lib/claim-copy";
import { cvsCopy } from "../../apps/storefront/lib/cvs-copy";
import { purchaseCopy } from "../../apps/storefront/lib/purchase-copy";
import { shopCopy } from "../../apps/storefront/lib/shop-copy";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const admin = required("LC_LT_ADMIN_ORIGIN");
const buyerOrigin = required("LC_LT_BUYER_ORIGIN");
const evidence = required("LC_LT_EVIDENCE");
const control = required("LC_LT_CONTROL");
const controlKey = required("LC_LT_CONTROL_KEY");
const store = required("LC_LT_STORE");

test.use({ launchOptions: { proxy: { server: required("LC_LT_PROXY"), bypass: "127.0.0.1" } } });
test.setTimeout(900_000);

type Locale = "zh-TW" | "en";
type Viewport = "desktop" | "mobile";
const sizes = { desktop: { width: 1440, height: 900 }, mobile: { width: 390, height: 844 } } as const;
const cells: { n: number; locale: Locale; viewport: Viewport }[] = [
  { n: 1, locale: "zh-TW", viewport: "desktop" },
  { n: 2, locale: "zh-TW", viewport: "mobile" },
  { n: 3, locale: "en", viewport: "desktop" },
  { n: 4, locale: "en", viewport: "mobile" },
];
// 30000 minor = 300.00 normal price, 20000 = 200.00 live price, quantity 2 on the claim and 1 on the direct purchase.
const live = { total: "400", unit: "200", normalTotal: "600", normalUnit: "300" };
const has = (n: string) => new RegExp(`(^|[^0-9.,])${n}(\\.00)?([^0-9]|$)`);
// Negative price checks scan whole sections, which also show fixture ids built from random hex: the order UUID
// ("2ef200c7-…"), the SKU code "LT<n>-<TAG>" and the product name "Live tools product <n> <TAG>". Remove exactly those
// fixture formats first so an id never reads as a price (PR #33 flake: "must not show 200" matched UUID "…f200c…").
// Positive checks and the amount matcher are unchanged; no rendered amount can match these id formats.
const withoutFixtureIds = (s: string) =>
  s.replace(/[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}/gi, " ").replace(/\bLT\d+-[0-9A-Z]+\b/g, " ").replace(/Live tools product \d+ [0-9A-Z]+/g, " ");

async function act(name: string, body: Record<string, unknown> = {}) {
  const response = await fetch(`${control}/act?name=${name}`, { method: "POST", headers: { "X-Gate-Key": controlKey, "content-type": "application/json" }, body: JSON.stringify(body) });
  const answer = (await response.json().catch(() => ({}))) as Record<string, any>;
  expect(response.status, `control ${name}: ${JSON.stringify(answer)}`).toBe(200);
  return answer;
}

const manifestPath = path.join(evidence, "screenshots.json");
async function shot(page: Page, name: string, locale: string, viewport: Viewport) {
  const file = path.join(evidence, `${name}-${locale}-${viewport}.png`);
  await page.screenshot({ path: file, fullPage: true, animations: "disabled" });
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth), `horizontal overflow: ${name} ${locale} ${viewport}`).toBeLessThanOrEqual(1);
  let manifest: unknown[] = [];
  try {
    manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  } catch {
    /* first screenshot */
  }
  manifest.push({ File: path.basename(file), Sha256: createHash("sha256").update(await readFile(file)).digest("hex"), Locale: locale, Viewport: viewport });
  await writeFile(manifestPath, JSON.stringify(manifest, null, 2));
}

const pageErrors: string[] = [];

// Delivery -> quotation -> buyer-entered 7-ELEVEN store -> pay at pickup -> place the order. Returns the order id and the quote section text.
async function checkoutPayAtPickup(page: Page, locale: Locale, viewport: Viewport, label: string, expectTotal: string, notTotal: string) {
  const purchase = purchaseCopy[locale];
  const cvs = cvsCopy[locale];
  await page.goto(`${buyerOrigin}/${locale}/checkout`);
  await page.getByRole("button", { name: purchase.delivery, exact: true }).click();
  await page.locator("#delivery").selectOption({ label: `${cvs.chains.cvs_711} · TW` });
  const quotation = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST");
  await page.getByRole("button", { name: purchase.quote, exact: true }).click();
  expect((await quotation).status()).toBe(200);
  await expect(page.getByTestId("address-section")).toBeVisible();
  const quoted = page.locator(".quotation");
  await expect(quoted, `${label}: the quotation shows the ${expectTotal} total`).toContainText(has(expectTotal));
  expect(withoutFixtureIds(await quoted.innerText()), `${label}: the quotation must not show ${notTotal}`).not.toMatch(has(notTotal));
  await page.getByTestId("cvs-recipient-name").fill("王小明");
  await page.getByTestId("cvs-recipient-phone").fill("0912345678");
  await page.getByTestId("cvs-entered-code").fill("123456");
  await page.getByTestId("cvs-entered-name").fill("測試門市");
  await page.getByTestId("cvs-entered-address").fill("台北市測試路1號");
  await page.getByRole("radio", { name: new RegExp(cvs.payAtPickup) }).check();
  await expect(page.getByTestId("cvs-pay-note")).toContainText(has(expectTotal));
  await shot(page, `${label}-quote`, locale, viewport);
  await page.getByTestId("confirm-address").click();
  await expect(page.getByTestId("create-order")).toBeEnabled();
  await page.getByTestId("create-order").click();
  await expect(page.getByTestId("order-section")).toBeVisible({ timeout: 30_000 });
  const order = ((await page.getByTestId("order-id").innerText()) ?? "").trim();
  expect(order).toMatch(/^[0-9a-f-]{36}$/);
  await expect(page.getByTestId("order-collection")).toHaveAttribute("data-state", "PENDING");
  expect(await page.getByTestId("pay-order").count(), "a pay-at-pickup order has no card step").toBe(0);
  await expect(page.getByTestId("order-section")).toContainText(has(expectTotal));
  expect(withoutFixtureIds(await page.getByTestId("order-section").innerText()), `${label}: the order page must not show ${notTotal}`).not.toMatch(has(notTotal));
  await shot(page, `${label}-order`, locale, viewport);
  return order;
}

for (const cell of cells) {
  test(`live tools ${cell.locale} ${cell.viewport}: library import, live price, signed Meta claim, pay at pickup at the live price; direct purchase at the normal price`, async ({ browser }) => {
    const { n, locale, viewport } = cell;
    const claims = claimsCopy[locale];
    const studio = studioCopy[locale];
    const claim = claimCopy[locale];
    const run = await act("provision-run", { n });
    const kw = run.keyword as string;

    // A failed cell must not leak an open claim window into the next cell (only one window per store can be open):
    // end-run closes it (idempotent) in a finally, and every browser context closes whatever happens.
    let scene: string | undefined;
    let failed = false;
    const contexts: BrowserContext[] = [];
    try {
      // ---------------------------------------------------------------------------------------------------------- merchant
      const merchantContext = await browser.newContext({ viewport: sizes[viewport] });
      contexts.push(merchantContext);
      const merchant = await merchantContext.newPage();
      merchant.on("pageerror", (error) => pageErrors.push(error.message));
      await merchant.goto(`${admin}/en/`);
      await merchant.getByRole("button", { name: "Sign in with identity service", exact: true }).click();
      await merchant.waitForURL((u) => u.origin === new URL(admin).origin && !u.pathname.startsWith("/api/"), { timeout: 30_000 });

      async function newScene(name: string) {
        await merchant.goto(`${admin}/${locale}/studio?store=${store}`);
        await expect(merchant.getByTestId("merchant-studio")).toBeVisible();
        await merchant.getByRole("button", { name: studio.newScene }).click();
        await merchant.getByLabel(studio.name).fill(name);
        await merchant.getByRole("button", { name: studio.create }).click();
        await merchant.waitForURL(/scene=[0-9a-f-]{36}/);
        await expect(merchant.locator(".studio-surface.studio-planning-only")).toBeVisible();
        await merchant.getByTestId("studio-open-claims").click();
        await expect(merchant.getByTestId("merchant-claims")).toBeVisible();
        return new URL(merchant.url()).searchParams.get("scene") as string;
      }

      // 1. the keyword library: add this run's SKU keyword in a first scene (the offer form's pickers also feed "Add to library")
      await newScene(`Live tools library ${n}`);
      const offerForm = merchant.locator(".claims-offer-form");
      await offerForm.getByLabel(claims.keyword, { exact: true }).fill(kw);
      await offerForm.getByLabel(claims.product, { exact: true }).selectOption({ label: run.product_name });
      await expect(offerForm.getByLabel(claims.sku, { exact: true })).toBeEnabled();
      await offerForm.getByLabel(claims.sku, { exact: true }).selectOption({ label: run.sku_code });
      await merchant.getByTestId("claims-add-library").click();
      await expect(merchant.getByTestId(`library-${kw}`)).toBeVisible();
      expect(await merchant.getByTestId("claims-library").innerText()).toContain(run.sku_code);
      await act("check", { name: "library", sku_id: run.sku_id, keyword: kw });
      await shot(merchant, "merchant-library", locale, viewport);

      // 2. a NEW scene: import every library keyword in one action, then set the live price in the offers table
      scene = await newScene(`Live tools claim ${n}`);
      await expect(merchant.getByTestId("claims-import-tools")).toBeVisible();
      await merchant.getByRole("button", { name: claims.live.importLibrary }).click();
      // the store-level library accumulates one keyword per matrix cell; this new scene imports all n of them, none conflicting
      await expect(merchant.getByTestId("claims-import-result")).toContainText(claims.live.importDone(n, 0));
      await expect(merchant.getByTestId(`offer-${kw}`)).toContainText(run.sku_code);
      const price = merchant.getByTestId(`offer-price-${kw}`);
      await price.locator("input").fill(live.unit);
      await price.getByRole("button", { name: claims.live.saveLivePrice }).click();
      await expect(price.locator("input")).toHaveValue(new RegExp(`^${live.unit}(\\.00)?$`));
      await expect(price.getByRole("button", { name: claims.live.saveLivePrice })).toHaveCount(0);
      await expect(price).not.toContainText(claims.live.higherWarning("x", "y").split("x")[0]); // lower than the normal price: no warning
      await act("check", { name: "offer", scene, sku_id: run.sku_id, keyword: kw });
      await shot(merchant, "merchant-offers-live-price", locale, viewport);

      // 3. open the window and bind the post (private reply on, in the buyer's language)
      const quantityRule = merchant.getByLabel(claims.mode, { exact: true });
      await expect(quantityRule.locator("option")).toHaveCount(3);
      await expect(quantityRule).toBeEnabled();
      await quantityRule.selectOption("KEYWORD_QTY_CONTAINS");
      await merchant.getByRole("button", { name: claims.openWindow }).click();
      await expect(merchant.getByTestId("claims-window-state")).toHaveText(claims.open);
      await expect(quantityRule).toHaveValue("KEYWORD_QTY_CONTAINS");
      await expect(quantityRule).toBeDisabled();
      await merchant.reload();
      await expect(merchant.getByLabel(claims.mode, { exact: true })).toHaveValue("KEYWORD_QTY_CONTAINS");
      await expect(merchant.getByLabel(claims.mode, { exact: true })).toBeDisabled();
      await merchant.locator("#claims-prompt-keyword").selectOption(kw);
      await merchant.context().grantPermissions(["clipboard-read", "clipboard-write"], { origin: admin });
      for (const promptLocale of ["zh-TW", "zh-CN", "en"] as const) {
        await merchant.locator("#claims-prompt-language").selectOption(promptLocale);
        const expectedPrompt = hostPrompt(promptLocale, "KEYWORD_QTY_CONTAINS", kw);
        await expect(merchant.getByTestId("host-prompt")).toHaveText(expectedPrompt);
        await merchant.getByRole("button", { name: claims.copyPrompt, exact: true }).click();
        await expect.poll(() => merchant.evaluate(() => navigator.clipboard.readText())).toBe(expectedPrompt);
      }
      const post = await act("new-post");
      const source = merchant.getByTestId("claims-source");
      await source.getByLabel(claims.sourceInput, { exact: true }).fill(post.post_url);
      await source.getByLabel(claims.sourceReplyLocale, { exact: true }).selectOption(locale);
      await expect(source.getByLabel(claims.sourceReplyLocale, { exact: true }).locator('option[value="ja"]')).toHaveCount(0);
      await source.getByLabel(claims.sourcePrivateReply).check();
      await source.getByRole("button", { name: claims.sourceSave }).click();
      await expect(source.getByTestId("claims-source-object")).toHaveText(post.object);
      await expect(source.getByRole("alert")).toHaveCount(0);

      // 4. a SIGNED Meta comment claims the keyword; the private reply carries the claim link (Go verifies the facts)
      await act("comment", { text: `${kw}+2` });
      const reply = await act("await-reply");
      expect(reply.sends).toBe(1);
      expect(reply.locale).toBe(locale);
      await act("check", { name: "claimed", scene, sku_id: run.sku_id });
      const link = reply.link as string;
      const token = link.split("#t=")[1];

      // ---------------------------------------------------------------------------------------------------------- buyer with the claim link
      const buyerContext = await browser.newContext({ ignoreHTTPSErrors: true, viewport: sizes[viewport], locale });
      contexts.push(buyerContext);
      const buyer = await buyerContext.newPage();
      buyer.on("pageerror", (error) => pageErrors.push(error.message));
      const buyerRequests: string[] = [];
      buyer.on("request", (request) => buyerRequests.push(request.url()));
      const opened = await buyer.goto(link);
      expect(opened?.status()).toBe(200);
      await expect(buyer.getByRole("heading", { level: 1, name: claim.title })).toBeVisible();
      const line = buyer.getByTestId(`claim-line-${kw}`);
      await expect(line).toContainText(run.sku_code);
      await expect(line).toContainText(`${claim.quantity} 2`);
      await expect(line, "the claim page shows the LIVE price (2 x 200)").toContainText(has(live.total));
      expect(withoutFixtureIds(await line.innerText()), "the claim page must not show the normal total").not.toMatch(has(live.normalTotal));
      expect(new URL(buyer.url()).hash).toBe("");
      await shot(buyer, "buyer-claim-live-price", locale, viewport);
      await buyer.getByTestId("claim-add").click();
      await buyer.waitForURL((url) => url.pathname === `/${locale}/checkout`);
      await expect(buyer.getByTestId("claim-checkout-notice")).toHaveText(claim.checkoutNotice);
      await expect(buyer.locator(`[data-testid="cart-line"][data-sku="${run.sku_id}"]`)).toBeVisible();
      await expect(buyer.locator(`[data-testid="cart-line"][data-sku="${run.sku_id}"]`).locator(".sf-line__unit")).toContainText("× 2");
      await act("check", { name: "cart", scene, sku_id: run.sku_id });
      // The cart drawer and the cart page both show the claimed line at the LIVE price (catalog struck through),
      // and the line/subtotal totals use it (2 x 200 = 400, not the 600 the catalog would give).
      const cartCopy = shopCopy[locale];
      const claimedLine = (scope: Locator) => scope.locator(`[data-testid="cart-line"][data-sku="${run.sku_id}"]`);
      const assertLiveLine = async (scope: Locator, label: string) => {
        const line = claimedLine(scope);
        await expect(line.locator(".sf-line__unit"), `${label}: the claimed line shows the LIVE unit price`).toContainText(has(live.unit));
        await expect(line.locator(".sf-line__unit s"), `${label}: the catalog price is struck through`).toContainText(has(live.normalUnit));
        await expect(line.locator(".sf-line__unit em.sf-line__live"), `${label}: the line is labelled a live price`).toContainText(cartCopy.livePrice);
        await expect(line.getByTestId("cart-line-total"), `${label}: the line total uses the live price (2 x 200)`).toContainText(has(live.total));
        const subtotal = scope.getByTestId("cart-subtotal");
        await expect(subtotal, `${label}: the subtotal uses the live price`).toContainText(has(live.total));
        expect(withoutFixtureIds(await subtotal.innerText()), `${label}: the subtotal must not show the normal total`).not.toMatch(has(live.normalTotal));
      };
      // The cart page loads the cart fresh (CartProvider GET /api/buyer/cart), so assert it first, then the
      // drawer from the same page (the claim page keeps its own cart state and the drawer would lag it).
      await buyer.goto(`${buyerOrigin}/${locale}/cart`);
      await assertLiveLine(buyer.getByTestId("cart-page"), "cart page");
      await shot(buyer, "buyer-cart-live-price", locale, viewport);
      await buyer.getByTestId("header-cart").click();
      await assertLiveLine(buyer.getByTestId("cart-drawer"), "cart drawer");
      const claimOrder = await checkoutPayAtPickup(buyer, locale, viewport, `buyer-claim-${n}`, live.total, live.normalTotal);
      const stored = await buyer.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }));
      expect(stored.includes(token), "the claim token must not be stored in the browser").toBe(false);
      expect(buyerRequests.every((url) => !url.includes(token)), "the claim token must not be in any request URL").toBe(true);
      await act("check", { name: "order-live", order: claimOrder, scene, sku_id: run.sku_id });

      // ---------------------------------------------------------------------------------------------------------- another buyer, the same SKU, the product page
      const directContext = await browser.newContext({ ignoreHTTPSErrors: true, viewport: sizes[viewport], locale });
      contexts.push(directContext);
      const direct = await directContext.newPage();
      direct.on("pageerror", (error) => pageErrors.push(error.message));
      await direct.goto(`${buyerOrigin}/${locale}/products/${run.product_id}`);
      await expect(direct.getByTestId("variant-price")).toContainText(has(live.normalUnit));
      expect(await direct.getByTestId("variant-price").innerText(), "the product page never shows the live price").not.toMatch(has(live.unit));
      await shot(direct, "buyer-product-normal-price", locale, viewport);
      await direct.getByTestId("add-to-cart").click();
      await direct.getByTestId("cart-checkout").click();
      await direct.waitForURL(`**/${locale}/checkout`);
      const directOrder = await checkoutPayAtPickup(direct, locale, viewport, `buyer-direct-${n}`, live.normalUnit, live.unit);
      await act("check", { name: "order-direct", order: directOrder, quantity: 1 });

      // the claim buyer's own product page still shows the normal price (the live price lives in the claim, not on the SKU)
      await buyer.goto(`${buyerOrigin}/${locale}/products/${run.product_id}`);
      await expect(buyer.getByTestId("variant-price")).toContainText(has(live.normalUnit));

      await act("end-run", { scene });
      expect(pageErrors).toEqual([]);
    } catch (error) {
      failed = true;
      throw error;
    } finally {
      if (failed && scene) {
        try {
          await act("end-run", { scene });
        } catch {
          /* keep the cell's own failure, not the cleanup's */
        }
      }
      await Promise.all(contexts.map((context) => context.close().catch(() => {})));
    }
  });
}
