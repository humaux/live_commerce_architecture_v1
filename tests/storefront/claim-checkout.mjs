// Purpose: real-click CDC04/CDC05 storefront claim checkout and recovery journeys.
// Depends on: production Next/Go/PG fixture, Playwright, browser-engine and shop-helpers.
// Used by: TestBrowserClaimDirectCheckout, Chromium and iPhone 15 WebKit gates.
// Fixture control only prepares/ages synthetic claims; buyer writes always start with a real click.
import assert from "node:assert/strict";
import { writeFile } from "node:fs/promises";
import path from "node:path";
import { expect } from "@playwright/test";
import { engine, launch, ctxOpts, phone, iosZoomOffenders, isWebkitCancelledFetch } from "./browser-engine.mjs";
import { addToCart } from "./shop-helpers.mjs";

const origin = process.env.LC_CDC_ORIGIN, evidence = process.env.LC_CDC_EVIDENCE;
assert(origin && evidence && process.env.LC_CDC_CONTROL_KEY);
const browser = await launch({ proxy: { server: process.env.LC_CDC_PROXY } });
const ledger = [], orders = [], contexts = [], tokens = [];
let cases = 0, activePage = null;
const copy = {
  "zh-TW": { checkout: "直接結帳", delivery: "選擇配送", quote: "取得目前總額", pickup: /取貨付款/, merge: /其他商品也會一起結帳/ },
  "zh-CN": { checkout: "直接结账", delivery: "选择配送", quote: "获取当前总额", pickup: /取货付款/, merge: /其他商品也会一起结账/ },
  en: { checkout: "Check out now", delivery: "Choose delivery", quote: "Get current total", pickup: /Pay at pickup/i, merge: /other items.*checked out together/i },
};
async function fixture(name, action = "link", order = undefined, sku = undefined) {
  // SETUP/FAULT INJECTION: loopback owner fixture; never substitutes for a buyer UI action.
  const response = await fetch(process.env.LC_CDC_CONTROL, { method: "POST", headers: {
    "Content-Type": "application/json", "X-CDC-Control": process.env.LC_CDC_CONTROL_KEY,
  }, body: JSON.stringify({ name, action, order, sku }) });
  assert.equal(response.status, 200, "fixture setup must succeed");
  const result = await response.json(); tokens.push(result.token); return result;
}
async function newPage(mobile = false) {
  const context = await browser.newContext(ctxOpts({ ...(mobile ? phone : { viewport: { width: 1440, height: 900 } }), ignoreHTTPSErrors: true }));
  contexts.push(context);
  const page = await context.newPage(), seen = { redeem: 0, cart: null, quote: null, preview: null, redeemed: null, errors: [], urls: [] };
  page.on("pageerror", (e) => { if (!isWebkitCancelledFetch(e)) seen.errors.push(e.message); });
  page.on("request", (r) => { seen.urls.push(r.url()); if (new URL(r.url()).pathname === "/api/buyer/claim-link/redeem") seen.redeem++; });
  page.on("response", async (r) => {
    // READBACK: capture the real UI-triggered responses, without issuing another request.
    if (r.status() === 200 && r.url().includes("/api/buyer/")) {
      const name = new URL(r.url()).pathname;
      const value = await r.json().catch(() => null);
      if (name === "/api/buyer/cart") seen.cart = value;
      if (name === "/api/buyer/quotes") seen.quote = value;
      if (name === "/api/buyer/claim-link") seen.preview = value;
      if (name === "/api/buyer/claim-link/redeem") seen.redeemed = value;
    }
    if (new URL(r.url()).pathname === "/api/buyer/checkout" && r.status() >= 400)
      seen.checkoutRefusal = { status: r.status(), body: await r.json().catch(() => null) };
    if (r.status() >= 500 && r.url().startsWith(origin)) seen.errors.push(`HTTP ${r.status()} ${new URL(r.url()).pathname}`);
  });
  page.seen = seen; return page;
}
async function action(page, control, operation, expected, verify) {
  activePage = page;
  const row = { page: new URL(page.url()).pathname, control, operation, expected, actual: "", pass: false };
  ledger.push(row);
  try { await operation(); await verify(); row.actual = expected; row.pass = true; }
  catch (error) { row.operation = typeof operation === "function" ? "real click/fill/select" : operation; row.actual = String(error.message).split("\n")[0]; throw error; }
  finally { row.operation = "real click/fill/select"; }
}
async function noLeaks(page, link) {
  // READ/MEASURE: storage, DOM and referrer audit only; no DOM or product-state mutation.
  const snapshot = await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage }, referrer: document.referrer, dom: document.documentElement.outerHTML }));
  assert(!snapshot.includes(link.token), "claim token must stay in memory only");
  assert(!page.url().includes(link.token), "address/history token removed");
  assert(!page.seen.urls.some((url) => new URL(url).search.includes(link.token)), "token absent from request query");
  if (engine === "webkit") assert.deepEqual(await iosZoomOffenders(page), [], "iPhone input fonts must be >=16px");
  assert.deepEqual(page.seen.errors, [], "no uncaught error or server error");
}
async function openClaim(page, link, locale = "zh-TW") {
  activePage = page;
  await page.goto(`${origin}/${locale}/claim#t=${link.token}`);
  await expect(page.getByTestId("claim-line-A1")).toBeVisible();
  assert.equal(page.seen.redeem, 0, "opening a claim is read-only");
  await noLeaks(page, link);
}
async function checkout(page, link, locale = "zh-TW", partial = false) {
  await action(page, "claim-add", () => page.getByTestId("claim-add").click(), "one click opens checkout with chosen SKU and quantity", async () => {
    await expect(page).toHaveURL(new RegExp(`/${locale}/checkout(?:\\?|$)`));
    await expect(page.getByTestId("claim-checkout-notice")).toContainText(copy[locale].merge);
    await expect(page.locator(`[data-testid="cart-line"][data-sku="${link.sku}"]`)).toBeVisible();
    await expect.poll(() => page.seen.cart?.items.find((i) => i.sku_id === link.sku)?.quantity).toBe(2);
    assert.equal(page.seen.redeem, 1, "one explicit redeem");
    if (partial) assert(!page.seen.cart.items.some((i) => i.sku_id === link.sold_sku));
  });
  await noLeaks(page, link);
  assert.equal(new URL(page.url()).searchParams.has("from"), false, "checkout consumes from=claim");
}
async function quote(page, locale = "zh-TW") {
  await action(page, "delivery", () => page.getByRole("button", { name: copy[locale].delivery, exact: true }).click(), "delivery selector opens", () => expect(page.locator("#delivery")).toBeVisible());
  await action(page, "7-ELEVEN", () => page.locator("#delivery").selectOption({ label: "7-ELEVEN · TW" }), "7-ELEVEN selected", () => expect(page.getByRole("button", { name: copy[locale].quote, exact: true })).toBeEnabled());
  await action(page, "quote", () => page.getByRole("button", { name: copy[locale].quote, exact: true }).click(), "quote and pickup form visible", () => expect(page.getByTestId("address-section")).toBeVisible());
}
async function place(page, link, locale = "zh-TW", loseReply = false, deplete = null) {
  await quote(page, locale);
  await expect.poll(() => page.seen.quote?.lines.find((i) => i.sku_id === link.sku)?.unit_price_minor).toBe(20000);
  for (const [id, value] of [["cvs-recipient-name", "王小明"], ["cvs-recipient-phone", "0912345678"], ["cvs-entered-code", "131386"], ["cvs-entered-name", "合成測試門市"], ["cvs-entered-address", "合成測試地址"]]) {
    await action(page, id, () => page.getByTestId(id).fill(value), "input value retained", () => expect(page.getByTestId(id)).toHaveValue(value));
  }
  await action(page, "pay-at-pickup", () => page.getByRole("radio", { name: copy[locale].pickup }).check(), "pay-at-pickup selected", () => expect(page.getByRole("radio", { name: copy[locale].pickup })).toBeChecked());
  await action(page, "confirm-address", () => page.getByTestId("confirm-address").click(), "order action enabled", () => expect(page.getByTestId("create-order")).toBeEnabled());
  await noLeaks(page, link);
  if (deplete) {
    await fixture(deplete, "deplete");
    await page.getByTestId("create-order").click();
    await expect.poll(() => page.seen.checkoutRefusal?.body?.code).toBe("insufficient_inventory");
    await expect(page.getByText("購物車或配送設定已變更。請重新載入最新資訊，再確認選擇。", { exact: true })).toBeVisible();
    await expect(page.getByTestId("order-section")).toHaveCount(0);
    return;
  }
  if (loseReply) {
    // FAULT INJECTION: lose the real click's reply after server commit; retain the actual purchase recovery journal.
    await page.route("**/api/buyer/checkout", async (route) => { await route.fetch(); await route.abort("failed"); });
    await page.getByTestId("create-order").click();
    await expect(page.locator("main.purchase-main > .purchase-error")).toBeVisible();
    await page.unroute("**/api/buyer/checkout"); return;
  }
  await action(page, "create-order", () => page.getByTestId("create-order").click(), "confirmed pay-at-pickup order", () => expect(page.getByTestId("order-section")).toBeVisible());
  const id = (await page.getByTestId("order-id").innerText()).trim(); orders.push(id);
  await page.reload(); await expect(page.getByTestId("order-id")).toHaveText(id);
  await expect(page.getByTestId("order-collection")).toHaveAttribute("data-state", "PENDING");
  assert.equal(await page.getByTestId("pay-order").count(), 0);
  return id;
}
const pass = (name) => { cases++; console.log(`PASS CDC ${name}`); };
try {
  // One full purchase per locale; phone profile runs every matrix cell on WebKit.
  let previous, previousLink;
  for (const locale of ["zh-TW", "zh-CN", "en"]) {
    const page = await newPage(engine === "webkit" || locale !== "zh-TW"), link = await fixture(locale);
    await openClaim(page, link, locale);
    await expect(page.getByTestId("claim-add")).toHaveText(copy[locale].checkout);
    await checkout(page, link, locale); await place(page, link, locale);
    await noLeaks(page, link); pass(`chosen SKU/quantity/live price -> order -> reload ${locale}`);
    if (locale === "zh-TW") { previous = page; previousLink = link; }
  }
  // An already-ordered cart is cleared before a second, different claim is merged.
  // SETUP: the previous synthetic order is shipped through the actual merchant path, satisfying the PAP one-unshipped-order cap.
  await fixture("zh-TW","ship-previous",orders[0]);
  const second = await fixture("second"); previous.seen.redeem = 0;
  await openClaim(previous, second); await checkout(previous, second);
  assert(!previous.seen.cart.items.some((i) => i.sku_id === previousLink.sku), "previous order's SKU must not be ordered again");
  await place(previous, second); pass("continueShopping prevents repeat purchase of old cart");

  const merged = await newPage(engine === "webkit"), mergeLink = await fixture("merge");
  await addToCart(merged, origin, "zh-TW", process.env.LC_CDC_PRODUCT);
  await openClaim(merged, mergeLink); await checkout(merged, mergeLink);
  assert.equal(merged.seen.cart.items.length, 2, "unpaid cart merges");
  await merged.reload(); await expect(merged.getByTestId("cart-line")).toHaveCount(2);
  const otherSKU = merged.seen.cart.items.find((item) => item.sku_id !== mergeLink.sku).sku_id;
  await action(merged, "back-to-cart", () => merged.getByRole("link", { name: "返回購物車", exact: true }).click(), "merged items can be reviewed in the cart", () => expect(merged).toHaveURL(/\/zh-TW\/cart/));
  await action(merged, "remove-other-cart-line", () => merged.locator(`[data-testid="cart-line"][data-sku="${otherSKU}"]`).getByRole("button", { name: /移出購物車/ }).click(), "only the other cart item is removed", async () => {
    await expect(merged.getByTestId("cart-line")).toHaveCount(1);
    await expect(merged.locator(`[data-testid="cart-line"][data-sku="${mergeLink.sku}"]`).getByTestId("cart-line-qty")).toHaveText("2");
  });
  await action(merged, "cart-checkout", () => merged.getByTestId("cart-checkout").click(), "chosen claim SKU and quantity return to checkout", async () => {
    await expect(merged).toHaveURL(/\/zh-TW\/checkout/);
    await expect(merged.getByTestId("cart-line")).toHaveCount(1);
    await expect.poll(() => merged.seen.cart?.items.map(({ sku_id, quantity }) => ({ sku_id, quantity }))).toEqual([{ sku_id: mergeLink.sku, quantity: 2 }]);
    assert.equal(merged.seen.cart.items[0].live_unit_price_minor, 20000, "server retains the claim price origin after cart removal");
  });
  await merged.reload(); await expect(merged.getByTestId("cart-line")).toHaveCount(1);
  await expect(merged.locator(`[data-testid="cart-line"][data-sku="${mergeLink.sku}"] .sf-line__unit`)).toContainText("× 2");
  await expect.poll(() => merged.seen.cart?.items.map(({ sku_id, quantity }) => ({ sku_id, quantity }))).toEqual([{ sku_id: mergeLink.sku, quantity: 2 }]);
  pass("unpaid merge survives reload and other item removal preserves chosen SKU/quantity");

  const partial = await newPage(engine === "webkit"), partialLink = await fixture("partial");
  await openClaim(partial, partialLink);
  await expect(partial.getByTestId("claim-line-B2")).toContainText(/售完|售罄|Sold out/);
  await checkout(partial, partialLink, "zh-TW", true); pass("partial sold-out skip");
  const sold = await newPage(engine === "webkit"), soldLink = await fixture("sold");
  await openClaim(sold, soldLink); await expect(sold.getByTestId("claim-add")).toBeDisabled(); assert.equal(sold.seen.redeem, 0); pass("all sold out is disabled");
  await fixture("sold", "replenish");
  await openClaim(sold,soldLink);
  await expect(sold.getByTestId("claim-add")).toBeEnabled();
  await checkout(sold,soldLink); pass("replenishment keeps pending and permits a fresh checkout click");

  const expired = await newPage(engine === "webkit"), expiredLink = await fixture("expired");
  await expired.goto(`${origin}/zh-TW/claim#t=${expiredLink.token}`); await expect(expired.getByTestId("claim-not-found")).toBeVisible(); await noLeaks(expired, expiredLink); pass("expired uniform 404");
  const repriced = await newPage(engine === "webkit"), repriceLink = await fixture("repriced");
  await openClaim(repriced, repriceLink); await checkout(repriced, repriceLink); await fixture("repriced", "expire");
  await quote(repriced); await expect.poll(() => repriced.seen.quote?.lines.find((i) => i.sku_id === repriceLink.sku)?.unit_price_minor).toBe(30000); pass("expired origin uses catalog quote");
  const race = await newPage(engine === "webkit"), raceLink = await fixture("race");
  await openClaim(race, raceLink); await fixture("race", "deplete");
  await race.getByTestId("claim-add").click();
  await expect.poll(() => race.seen.preview?.lines.some((line) => line.sold_out)).toBe(true);
  await expect(race.getByTestId("claim-add")).toHaveText("直接結帳");
  await expect(race.getByTestId("claim-add")).toBeDisabled();
  await expect(race.getByText("購物車裡已有這些商品。",{exact:true})).toHaveCount(0);
  assert.equal(race.seen.redeemed.skipped[0].reason,"sold_out"); assert.deepEqual(race.seen.redeemed.cart.items,[]);
  assert.equal(new URL(race.url()).pathname, "/zh-TW/claim"); pass("B1/B2 stock race settles sold-out without cart write/navigation");
  const repeat = await newPage(engine === "webkit"), repeatLink = await fixture("repeat");
  await openClaim(repeat, repeatLink); await repeat.getByTestId("claim-add").click({ clickCount: 2 });
  await expect(repeat).toHaveURL(/\/zh-TW\/checkout/); assert.equal(repeat.seen.redeem, 1); pass("double click sends one redeem");
  await repeat.goBack();
  await expect(repeat.getByTestId("claim-not-found")).toBeVisible();
  assert.equal(new URL(repeat.url()).hash, ""); await noLeaks(repeat, repeatLink); pass("back navigation forgets token and displays expired view");
  repeat.seen.redeem=0;await openClaim(repeat,repeatLink);await checkout(repeat,repeatLink);await place(repeat,repeatLink);
  pass("double-click journey yields exactly one persisted order");

  const beginRace=await newPage(engine==="webkit"), beginLink=await fixture("begin-race");
  await openClaim(beginRace,beginLink);await checkout(beginRace,beginLink);await place(beginRace,beginLink,"zh-TW",false,"begin-race");
  pass("stock disappearing after quote is refused at Begin");

  const pending = await newPage(engine === "webkit"), pendingLink = await fixture("pending");
  await openClaim(pending, pendingLink); await checkout(pending, pendingLink); await place(pending, pendingLink, "zh-TW", true);
  const recoveryLink = await fixture("conflict"); pending.seen.redeem = 0;
  await openClaim(pending, recoveryLink); await pending.getByTestId("claim-add").click();
  await expect(pending).toHaveURL(/\/zh-TW\/checkout/); assert.equal(pending.seen.redeem, 0, "in-flight checkout never redeems a new link");
  await action(pending,"recover-checkout",()=>pending.getByRole("button",{name:"確認上一次的結果",exact:true}).click(),"the same pending checkout recovers its order",()=>expect(pending.getByTestId("order-id")).toBeVisible());
  orders.push((await pending.getByTestId("order-id").innerText()).trim()); pass("in-flight order recovers without B2");
  // 409 from a pre-existing archived item: navigate to cart, really remove it, then reopen claim.
  const bad=await newPage(engine==="webkit"), badLink=await fixture("archived");
  await addToCart(bad,origin,"zh-TW",process.env.LC_CDC_PRODUCT);
  await expect.poll(() => bad.seen.cart?.items.length).toBe(1);
  await fixture("archived","archive-other",undefined,bad.seen.cart.items[0].sku_id);await openClaim(bad,badLink);await bad.getByTestId("claim-add").click();
  await expect(bad.getByTestId("claim-conflict")).toBeVisible();
  await bad.getByRole("link",{name:"檢查購物車",exact:true}).click();
  await expect(bad).toHaveURL(/\/zh-TW\/cart/);
  await action(bad,"remove-archived-cart-line",()=>bad.getByTestId("cart-line").getByRole("button",{name:/移出購物車/}).click(),"unavailable cart item is removed",()=>expect(bad.getByTestId("cart-line")).toHaveCount(0));
  await expect(bad.getByTestId("cart-line")).toHaveCount(0);
  bad.seen.redeem=0;await openClaim(bad,badLink);await checkout(bad,badLink);pass("409 cart recovery through actual remove click");
  assert.equal(new Set(orders).size, orders.length);
  await writeFile(path.join(evidence, "result.json"), JSON.stringify({ cases, orders, engine }, null, 2));
} catch (error) {
  const page = activePage ?? contexts.flatMap((context) => context.pages()).at(-1);
  if (page && !page.isClosed()) await page.screenshot({ path: path.join(evidence, "failure.png"), fullPage: true }).catch(() => {});
  if (page && !page.isClosed()) console.error("CDC visible alerts", JSON.stringify(await page.locator('[role="alert"], .purchase-error').allInnerTexts()));
  console.error("FAIL CDC", String(error.message).replaceAll(/#t=[A-Za-z0-9_-]+/g, "#t=REDACTED")); process.exitCode = 1;
} finally {
  const serialized = JSON.stringify(ledger, null, 2);
  assert(!tokens.some((token) => serialized.includes(token)), "ledger must not contain a token");
  await writeFile(path.join(evidence, "ledger.json"), serialized);
  await writeFile(path.join(evidence, "ledger.md"), "| Page | Control | Action | Expected | Actual | Pass |\n|---|---|---|---|---|---|\n" + ledger.map((r) => `| ${r.page} | ${r.control} | ${r.operation} | ${r.expected} | ${r.actual.replaceAll("|", "/")} | ${r.pass} |`).join("\n"));
  for (const context of contexts) await context.close(); await browser.close();
}
