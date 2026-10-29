// Purpose: CHECKOUT-V2-IMPL real-click review, payment-help and re-quote acceptance.
// Depends on: Playwright, the existing order/home-COD gates and synthetic buyer fixtures.
// Used by: order-gate.mjs and home-cod-buyer.mjs in GitHub CI only.
// Invariants: money assertions use response fields; evaluate only reads layout; no checkout with a MOCK quote.
import assert from "node:assert/strict";
import { writeFile } from "node:fs/promises";
import path from "node:path";
import { expect } from "@playwright/test";
import { formatMoney } from "../../apps/storefront/lib/money.ts";
import { reachCheckout } from "./shop-helpers.mjs";

const copy = {
  en: { timing: "Pay after placing your order", open: "Expand payment help", close: "Collapse payment help", changed: "Total updated. Please confirm again.", confirm: "Confirm updated total", change: "Change delivery", quote: "Get current total" },
  "zh-TW": { timing: "送出後付款", open: "展開付款說明", close: "收合付款說明", changed: "金額已更新，請再確認", confirm: "確認更新後的金額", change: "重新選擇配送", quote: "取得目前總額" },
  "zh-CN": { timing: "送出后付款", open: "展开付款说明", close: "收合付款说明", changed: "金额已更新，请再确认", confirm: "确认更新后的金额", change: "重新选择配送", quote: "取得目前总额" },
};
const localeScreen = {
  en: { delivery: "Choose delivery", quote: "Get current total", timing: copy.en.timing, open: copy.en.open, close: copy.en.close },
  "zh-TW": { delivery: "選擇配送", quote: "取得目前總額", timing: copy["zh-TW"].timing, open: copy["zh-TW"].open, close: copy["zh-TW"].close },
  "zh-CN": { delivery: "选择配送", quote: "获取当前总额", timing: "提交订单后付款", open: "展开付款说明", close: "收起付款说明" },
};

// RVFIX #273: the isolated merchant API changes the real saved fee after BOTH buyers quote.
// Response interception only holds reads to inspect loading; it never changes money or creates orders.
export async function checkoutV2CodFeeChange({ newContext, origin, product, recipient, evidence, changeFee, pendingRecovered }) {
  const ledger = [], failures = [], orders = [], buyers = [];
  const automaticOrders = [], pendingBuyers = [], pendingOrders = [];
  // MOCK option configuration only: COD-only is a supported shop configuration.
  // Preserve all server money, caps, versions and delivery facts; every order still goes to the real backend.
  const onlyCod = (body) => ({ ...body, items: body.items.map((row) =>
    row.delivery_kind === "home" && row.payment_modes?.includes("cash_on_delivery")
      ? { ...row, payment_modes: ["cash_on_delivery"] } : row) });
  const file = path.join(evidence, "rvfix-273-click-ledger.json");
  const step = async (page, control, expected, perform) => {
    try { await ledgerStep(ledger, file, page, control, "real clicks / reload", expected, perform); }
    catch (error) { failures.push(`${control}: ${error.message}`); }
  };
  try {
    for (const [mobile, recovery] of [[false, "refusal"], [true, "reload"]]) {
      const context = await newContext(mobile), page = await context.newPage();
      await reachCheckout(page, origin, "en", product, { quantity: 2 });
      await page.getByRole("button", { name: "Choose delivery", exact: true }).click();
      const [response] = await Promise.all([
        page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST"),
        page.getByRole("button", { name: copy.en.quote, exact: true }).click(),
      ]);
      const quote = await response.json();
      assert.equal(quote.cod_due_minor, quote.amount.total_minor + 5000, "initial fee is the saved NT$50 fixture");
      for (const [key, value] of Object.entries(recipient)) await page.locator(`input[name="${key}"]`).fill(value);
      await page.locator('input[name="home_payment_mode"][value="cash_on_delivery"]').check();
      await page.getByTestId("confirm-address").click();
      await expect(page.getByTestId("create-order")).toBeEnabled();
      await expect(page.getByTestId("checkout-review").getByTestId("review-total")).toHaveText(formatMoney("en", quote.cod_due_minor, quote.currency));
      buyers.push({ context, page, quote, recovery });
    }
    for (const mobile of [false, true]) {
      const context = await newContext(mobile), page = await context.newPage();
      const recovery = `automatic COD ${mobile ? "mobile" : "desktop"}`;
      buyers.push({ context, page, recovery, automatic: true });
      await page.route("**/api/buyer/checkout-options?**", async (route) => {
        const response = await route.fetch();
        assert.equal(response.status(), 200, "COD-only option fixture requires a real successful response");
        await route.fulfill({ response, json: onlyCod(await response.json()) });
      });
      await reachCheckout(page, origin, "en", product, { quantity: 2 });
      await page.getByRole("button", { name: "Choose delivery", exact: true }).click();
      const [response] = await Promise.all([
        page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST"),
        page.getByRole("button", { name: copy.en.quote, exact: true }).click(),
      ]);
      const quote = await response.json();
      buyers.at(-1).quote = quote;
      assert.equal(quote.cod_due_minor, quote.amount.total_minor + 5000, "automatic COD initial server due");
      await ledgerStep(ledger, file, page, `${recovery} initial review`, "fill recipient and confirm address; never touch the payment radio",
        "the sole COD choice is already checked and both submits show the initial server payable", async () => {
          await expect(page.locator('input[name="home_payment_mode"]')).toHaveCount(1);
          await expect(page.locator('input[name="home_payment_mode"][value="cash_on_delivery"]')).toBeChecked();
          for (const [key, value] of Object.entries(recipient)) await page.locator(`input[name="${key}"]`).fill(value);
          await page.getByTestId("confirm-address").click();
          await expect(page.getByTestId("create-order")).toBeEnabled();
          await expect(page.getByTestId("checkout-submit")).toBeEnabled();
          await expect(page.getByTestId("review-total")).toHaveText(formatMoney("en", quote.cod_due_minor, quote.currency));
          await expect(page.locator(".purchase-footer strong")).toHaveText(formatMoney("en", quote.cod_due_minor, quote.currency));
        });
    }
    // REAL backend, transport fault only: each order commits at fee50, but its receipt is lost.
    // Prepare all locales/viewports before the single real merchant fee change.
    for (const locale of ["zh-TW", "zh-CN", "en"]) for (const mobile of [false, true]) {
      const context = await newContext(mobile), page = await context.newPage();
      const buyer = { context, page, locale, mobile, calls: [], receipts: [] };
      pendingBuyers.push(buyer);
      await reachCheckout(page, origin, locale, product, { quantity: 2 });
      await page.getByRole("button", { name: localeScreen[locale].delivery, exact: true }).click();
      const [response] = await Promise.all([
        page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST"),
        page.getByRole("button", { name: localeScreen[locale].quote, exact: true }).click(),
      ]);
      buyer.quote = await response.json();
      assert.equal(buyer.quote.cod_due_minor, buyer.quote.amount.total_minor + 5000);
      for (const [key, value] of Object.entries(recipient)) await page.locator(`input[name="${key}"]`).fill(value);
      await page.locator('input[name="home_payment_mode"][value="cash_on_delivery"]').check();
      await page.getByTestId("confirm-address").click();
      await expect(page.getByTestId("create-order")).toBeEnabled();
      await page.route("**/api/buyer/checkout", async (route) => {
        const request = route.request(), response = await route.fetch();
        assert.equal(response.status(), 200, "lost COD receipt must follow a real successful checkout");
        buyer.calls.push({ key: request.headers()["idempotency-key"], body: request.postDataJSON() });
        buyer.receipts.push(await response.json());
        await route.abort("failed");
      });
      await page.getByTestId("create-order").click();
      await expect(page.getByTestId("recover-purchase")).toBeVisible();
      // READ only: verify the PII-free original operation journal, never inject purchase state.
      buyer.pending = await page.evaluate(() => {
        const key = Object.keys(localStorage).find((key) => key.startsWith("commerce-purchase-pending-v1:"));
        return key ? JSON.parse(localStorage.getItem(key)) : null;
      });
      assert.equal(buyer.pending.kind, "checkout");
      assert.equal(buyer.pending.body.expected_cod_surcharge_minor, 5000);
      assert(buyer.calls.length > 0);
      for (const call of buyer.calls) {
        assert.equal(call.key, buyer.pending.key);
        assert.deepEqual(call.body, buyer.pending.body);
      }
      for (const receipt of buyer.receipts) assert.equal(receipt.order_id, buyer.receipts[0].order_id);
      await page.unrouteAll({ behavior: "wait" });
    }
    await changeFee();
    for (const { page, locale, mobile, quote, pending, receipts } of pendingBuyers) {
      const label = `unresolved COD ${locale}/${mobile ? "mobile" : "desktop"}`;
      let currentQuote;
      await page.route(`**/api/buyer/quotes/${quote.id}`, async (route) => {
        const response = await route.fetch();
        assert.equal(response.status(), 200);
        currentQuote = await response.json();
        await route.fulfill({ response });
      });
      await page.reload();
      await expect(page.getByTestId("recover-purchase")).toBeVisible();
      await expect.poll(() => currentQuote?.cod_due_minor).toBe(quote.amount.total_minor + 6000);
      await step(page, `${label} pending review`, "original fee remains visible; payable is withheld until original order recovery", async () => {
        await expect(page.getByTestId("review-cod-fee")).toHaveText(formatMoney(locale, 5000, quote.currency));
        await expect(page.getByTestId("review-total")).toHaveText("—");
      });
      await step(page, `${label} pending footer`, "fixed footer withholds current-offer payable while original checkout is unresolved", async () => {
        await expect(page.locator(".purchase-footer strong")).toHaveText("—");
        await expect(page.getByTestId("checkout-submit")).toBeDisabled();
      });
      await step(page, `${label} original recovery`, "status click replays unchanged intent/key and recovers the original order and collection amount", async () => {
        const [request] = await Promise.all([
          page.waitForRequest((r) => new URL(r.url()).pathname === "/api/buyer/checkout" && r.method() === "POST"),
          page.getByTestId("recover-purchase").click(),
        ]);
        assert.equal(request.headers()["idempotency-key"], pending.key);
        assert.deepEqual(request.postDataJSON(), pending.body);
        await expect(page.getByTestId("order-id")).toHaveText(receipts[0].order_id);
        await expect(page.getByTestId("order-cod-amount")).toContainText(formatMoney(locale, quote.cod_due_minor, quote.currency));
        await page.reload();
        await expect(page.getByTestId("order-id")).toHaveText(receipts[0].order_id);
        await expect(page.getByTestId("order-cod-amount")).toContainText(formatMoney(locale, quote.cod_due_minor, quote.currency));
        pendingOrders.push({ id: receipts[0].order_id, total: quote.amount.total_minor, due: quote.cod_due_minor, locale, mobile });
      });
      await page.unrouteAll({ behavior: "wait" });
    }
    await writeFile(path.join(evidence, "rvfix-273-pending-orders.json"), JSON.stringify(pendingOrders, null, 2));
    assert.equal(pendingOrders.length, 6, "all locales/viewports recover their original COD order");
    assert.equal(new Set(pendingOrders.map((order) => order.id)).size, 6, "six isolated buyers recover six distinct original orders");
    await pendingRecovered();
    for (const { page, quote, recovery, automatic } of buyers) {
      let freshQuote, optionReads = 0, releaseReads;
      const readsReady = new Promise((resolve) => { releaseReads = resolve; });
      let hold = recovery === "refusal";
      const readRoute = async (route) => {
        const response = await route.fetch();
        if (route.request().url().includes("checkout-options")) optionReads++;
        else if (response.status() === 200) freshQuote = await response.json();
        if (hold) await readsReady;
        if (automatic && route.request().url().includes("checkout-options"))
          return route.fulfill({ response, json: onlyCod(await response.json()) });
        await route.fulfill({ response });
      };
      await page.route("**/api/buyer/checkout-options?**", readRoute);
      await page.route(`**/api/buyer/quotes/${quote.id}`, readRoute);
      if (recovery === "refusal") {
        await step(page, "409 loading", "real fee refusal blocks both submits and amount acknowledgment while authoritative reads are held", async () => {
          const [refused] = await Promise.all([
            page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/checkout" && r.request().method() === "POST"),
            page.getByTestId("create-order").click(),
          ]);
          assert.equal(refused.status(), 409);
          assert.equal((await refused.json()).code, "cod_surcharge_changed");
          await expect(page.getByTestId("quote-reconfirm")).toContainText(copy.en.changed);
          await expect.poll(() => optionReads).toBeGreaterThan(0);
          await expect(page.getByRole("button", { name: copy.en.confirm, exact: true })).toBeDisabled();
          await expect(page.getByTestId("create-order")).toBeDisabled();
          await expect(page.getByTestId("checkout-submit")).toBeDisabled();
        });
        hold = false; releaseReads();
      } else {
        releaseReads();
        await page.reload();
      }
      await step(page, `${recovery} updated due`, "same quote ID re-reads server COD due and displays old → new in notice, review and fixed bar", async () => {
        await expect.poll(() => freshQuote?.cod_due_minor).toBe(quote.amount.total_minor + 6000);
        assert.equal(freshQuote.id, quote.id, "GET keeps the immutable quote ID while current COD due changes");
        const notice = page.getByTestId("quote-reconfirm");
        await expect(notice).toContainText(copy.en.changed);
        await expect(notice).toContainText(`${formatMoney("en", quote.cod_due_minor, quote.currency)} → ${formatMoney("en", freshQuote.cod_due_minor, quote.currency)}`);
        await expect(page.getByTestId("review-total")).toHaveText(formatMoney("en", freshQuote.cod_due_minor, quote.currency));
        await expect(page.locator(".purchase-footer strong")).toHaveText(formatMoney("en", freshQuote.cod_due_minor, quote.currency));
        await expect(page.getByTestId("review-cod-fee")).toHaveText(formatMoney("en", 6000, quote.currency));
      });
      await step(page, `${recovery} persisted acknowledgment`, "reload retains changed payable block; separate acknowledgment survives another reload", async () => {
        await page.reload();
        await expect(page.getByTestId("address-section")).toBeVisible();
        await expect(page.getByTestId("quote-reconfirm")).toContainText(copy.en.changed);
        await page.getByTestId("confirm-address").click();
        await expect(page.getByTestId("create-order")).toBeDisabled();
        await expect(page.getByTestId("checkout-submit")).toBeDisabled();
        await page.getByRole("button", { name: copy.en.confirm, exact: true }).click();
        await expect(page.getByTestId("create-order")).toBeEnabled();
        await page.reload();
        await expect(page.getByTestId("address-section")).toBeVisible();
        await page.getByTestId("confirm-address").click();
        await expect(page.getByTestId("quote-reconfirm")).toHaveCount(0);
        await expect(page.getByTestId("create-order")).toBeEnabled();
        await expect(page.getByTestId("checkout-submit")).toBeEnabled();
      });
      // A failing acknowledgment must not create an order. Other cases above still record their independent assertions.
      if (!failures.length) await step(page, `${recovery} collection`, "real order collection equals the server payable the buyer acknowledged, including after reload", async () => {
        const [submitted] = await Promise.all([
          page.waitForRequest((r) => new URL(r.url()).pathname === "/api/buyer/checkout" && r.method() === "POST"),
          page.getByTestId("checkout-submit").click(),
        ]);
        assert.equal(submitted.postDataJSON().expected_cod_surcharge_minor, 6000);
        await expect(page.getByTestId("order-section")).toBeVisible({ timeout: 30000 });
        const id = (await page.getByTestId("order-id").innerText()).trim();
        await expect(page.getByTestId("order-cod-amount")).toContainText(formatMoney("en", freshQuote.cod_due_minor, quote.currency));
        await page.reload();
        await expect(page.getByTestId("order-id")).toHaveText(id);
        await expect(page.getByTestId("order-cod-amount")).toContainText(formatMoney("en", freshQuote.cod_due_minor, quote.currency));
        if (automatic) automaticOrders.push({ id, total: freshQuote.amount.total_minor, due: freshQuote.cod_due_minor });
        else {
          orders.push({ id, total: freshQuote.amount.total_minor, due: freshQuote.cod_due_minor });
        }
      });
      // Drain in-flight reads before disabling interception; never swallow handler failures.
      await page.unrouteAll({ behavior: "wait" });
    }
    await writeFile(path.join(evidence, "rvfix-273-orders.json"), JSON.stringify(orders, null, 2));
    await writeFile(path.join(evidence, "rvfix-273-automatic-orders.json"), JSON.stringify(automaticOrders, null, 2));
    assert.deepEqual(failures, [], "RVFIX #273 changed COD payable assertions");
    assert.equal(orders.length, 2, "both recovery paths create a real order");
    assert.equal(automaticOrders.length, 2, "automatic COD on desktop and mobile creates real orders only after separate acknowledgment");
  } finally { for (const { context } of [...buyers, ...pendingBuyers]) await context.close(); }
}

async function ledgerStep(ledger, file, page, control, action, expected, perform) {
  const row = { page: new URL(page.url()).pathname, control, action, expected, actual: "NOT_RUN", status: "NOT_RUN" };
  ledger.push(row);
  try { await perform(); row.actual = expected; row.status = "PASS"; }
  catch (error) { row.actual = error.message.slice(0, 1500); row.status = "FAIL"; throw error; }
  finally { await writeFile(file, JSON.stringify(ledger, null, 2)); }
}

export async function checkoutV2Review(page, quote, locale, recipient, { codFee = null, ledger, file }) {
  const c = copy[locale], summary = page.getByTestId("checkout-review"), footer = page.locator(".purchase-footer");
  const money = (amount) => formatMoney(locale, amount, quote.currency);
  const timing = codFee === null ? c.timing : locale === "en" ? "Cash on delivery: pay when your parcel arrives" : "貨到付款：到貨時付款";
  await ledgerStep(ledger, file, page, "final review", "read after real address confirmation", "server money, recipient, destination and timing in review and fixed bar", async () => {
    await expect(summary).toBeVisible();
    await expect(summary.getByTestId("review-subtotal")).toHaveText(money(quote.amount.subtotal_minor));
    await expect(summary.getByTestId("review-shipping")).toHaveText(money(quote.amount.shipping_minor));
    await expect(summary.getByTestId("review-total")).toHaveText(money(codFee === null ? quote.amount.total_minor : quote.cod_due_minor));
    if (codFee !== null) await expect(summary.getByTestId("review-cod-fee")).toHaveText(money(codFee));
    for (const key of ["recipient_name", "phone", "line1", "city"]) await expect(summary).toContainText(recipient[key]);
    await expect(summary).toContainText(timing);
    await expect(footer).toContainText(timing);
    await expect(footer.locator("strong")).toHaveText(money(codFee === null ? quote.amount.total_minor : quote.cod_due_minor));
    await expect(footer.getByTestId("checkout-submit")).toBeEnabled();
  });
  // READ/MEASURE only. End scroll is a real keyboard operation, not evaluate/scrollIntoView.
  await ledgerStep(ledger, file, page, "checkout scroll", "keyboard End", "last review control ends above the fixed bar without horizontal overflow", async () => {
    await page.locator("body").click({ position: { x: 2, y: 2 } });
    await page.keyboard.press("ControlOrMeta+End");
    await expect.poll(() => page.getByTestId("checkout-content-end").evaluate((end) => {
      const bar = document.querySelector(".purchase-footer");
      return !!bar && end.getBoundingClientRect().bottom <= bar.getBoundingClientRect().top;
    })).toBe(true);
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
  });
}

export async function checkoutV2Suite({ newContext, origin, product, recipient, evidence }) {
  const ledger = [], failures = [], file = path.join(evidence, "checkout-v2-click-ledger.json");
  const fill = async (page) => { for (const [key, value] of Object.entries(recipient)) await page.locator(`input[name="${key}"]`).fill(value); };
  const confirm = async (page) => { await page.getByTestId("confirm-address").click(); await expect(page.getByTestId("create-order")).toBeEnabled(); };
  for (const mobile of [false, true]) {
    const context = await newContext(mobile), page = await context.newPage();
    await reachCheckout(page, origin, "en", product);
    await page.getByRole("button", { name: "Choose delivery", exact: true }).click();
    const quoted = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST");
    const [response] = await Promise.all([quoted, page.getByRole("button", { name: copy.en.quote, exact: true }).click()]);
    const quote = await response.json();
    await expect(page.getByTestId("address-section")).toBeVisible();
    try {
      await fill(page); await confirm(page);
      // Separate cases keep every new behaviour observable on the single tests-only RED push.
      for (const [name, test] of [
        ["review", () => checkoutV2Review(page, quote, "en", recipient, { ledger, file })],
        ["help", async () => {
          const help = page.getByTestId("payment-help-toggle");
          await ledgerStep(ledger, file, page, "payment help", "click twice", "Chevron, labels and >=44px target track expanded state", async () => {
            await expect(help).toHaveText(copy.en.open); await expect(help).toHaveAttribute("aria-expanded", "false");
            await expect(help.locator("svg")).toBeVisible();
            const box = await help.boundingBox(); assert(box && box.height >= 44 && box.width >= 44);
            await help.click(); await expect(help).toHaveText(copy.en.close); await expect(help).toHaveAttribute("aria-expanded", "true");
            await expect(page.getByTestId("payment-help-content")).toBeVisible();
            await help.click(); await expect(help).toHaveText(copy.en.open); await expect(page.getByTestId("payment-help-content")).toBeHidden();
          });
        }],
      ]) { try { await test(); } catch (error) { failures.push(`${mobile ? "390" : "desktop"} ${name}: ${error.message}`); } }
      await page.reload(); await expect(page.getByTestId("address-section")).toBeVisible(); await confirm(page);
      try { await checkoutV2Review(page, quote, "en", recipient, { ledger, file }); } catch (error) { failures.push(`refresh review: ${error.message}`); }

      // MOCK response fixture only: real clicks request a replacement quote; no checkout is sent with this altered amount.
      // Both POST and subsequent pinned GET return that same response, so refresh cannot erase the re-confirm requirement.
      let changedQuote;
      await page.route("**/api/buyer/quotes**", async (route) => {
        const response = await route.fetch(), body = await response.json();
        if (response.status() !== 200) return route.fulfill({ response });
        if (route.request().method() === "POST") changedQuote = { ...body, amount: { ...body.amount, total_minor: body.amount.total_minor + 1700, shipping_minor: body.amount.shipping_minor + 1700 } };
        await route.fulfill({ response, json: changedQuote ?? body });
      });
      try {
        await ledgerStep(ledger, file, page, "changed quote", "click delivery / quote / confirm address", "old-to-new amount and explicit re-confirm block both submits, including after refresh", async () => {
          await page.getByRole("button", { name: copy.en.change, exact: true }).click();
          await page.getByRole("button", { name: copy.en.quote, exact: true }).click();
          await expect(page.getByTestId("quote-reconfirm")).toContainText(copy.en.changed);
          for (const amount of [quote.amount.total_minor, changedQuote.amount.total_minor]) await expect(page.getByTestId("quote-reconfirm")).toContainText(formatMoney("en", amount, quote.currency));
          await expect(page.getByTestId("quote-reconfirm")).toContainText("→");
          await fill(page); await page.getByTestId("confirm-address").click();
          await expect(page.getByTestId("create-order")).toBeDisabled(); await expect(page.getByTestId("checkout-submit")).toBeDisabled();
          await page.reload(); await expect(page.getByTestId("quote-reconfirm")).toContainText(copy.en.changed);
          await page.getByTestId("confirm-address").click(); await expect(page.getByTestId("create-order")).toBeDisabled();
          const reconfirm = page.getByRole("button", { name: copy.en.confirm, exact: true });
          const box = await reconfirm.boundingBox(); assert(box && box.height >= 44);
          await reconfirm.click(); await expect(page.getByTestId("create-order")).toBeEnabled(); await expect(page.getByTestId("checkout-submit")).toBeEnabled();
          await page.reload(); await expect(page.getByTestId("address-section")).toBeVisible();
          await confirm(page); await expect(page.getByTestId("quote-reconfirm")).toHaveCount(0);
        });
      } catch (error) { failures.push(`re-quote: ${error.message}`); }
    } finally { await context.close(); }
  }
  // Main screen in all supported locales, on both layouts, through real controls only.
  for (const locale of ["zh-TW", "zh-CN", "en"]) for (const mobile of [false, true]) {
    const context = await newContext(mobile), page = await context.newPage(), words = localeScreen[locale];
    try {
      await reachCheckout(page, origin, locale, product);
      await page.getByRole("button", { name: words.delivery, exact: true }).click();
      const quoted = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST");
      const [response] = await Promise.all([quoted, page.getByRole("button", { name: words.quote, exact: true }).click()]);
      const quote = await response.json();
      await expect(page.getByTestId("address-section")).toBeVisible();
      await fill(page); await confirm(page);
      await ledgerStep(ledger, file, page, `${locale} final screen`, "real quote/address/radio/help controls and reload", "server total, recipient and timing persist; selected radio is explicit", async () => {
        const review = page.getByTestId("checkout-review"), radio = review.getByRole("radio").first();
        await radio.click(); await page.getByTestId("confirm-address").click();
        await expect(radio).toBeChecked(); await expect(radio.locator("..")).toHaveClass(/selected/);
        await expect(review).toContainText(recipient.recipient_name); await expect(review).toContainText(recipient.phone);
        await expect(page.locator(".purchase-footer")).toContainText(words.timing);
        await expect(review.getByTestId("review-total")).toHaveText(formatMoney(locale, quote.amount.total_minor, quote.currency));
        const help = page.getByTestId("payment-help-toggle");
        await help.click(); await expect(help).toHaveText(words.close); await expect(page.getByTestId("payment-help-content")).toBeVisible();
        await help.click(); await expect(help).toHaveText(words.open);
        await page.reload(); await expect(page.getByTestId("address-section")).toBeVisible(); await confirm(page);
        await expect(page.getByTestId("checkout-review")).toContainText(recipient.phone);
        await expect(page.locator(".purchase-footer")).toContainText(words.timing);
        await expect(page.getByTestId("checkout-submit")).toBeEnabled();
      });
      if (locale === "en" && mobile) await ledgerStep(ledger, file, page, "fixed submit", "click submit then reload", "one original order is displayed and restored", async () => {
        await page.getByTestId("checkout-submit").click();
        await expect(page.getByTestId("order-section")).toBeVisible();
        const id = await page.getByTestId("order-id").innerText();
        await page.reload(); await expect(page.getByTestId("order-id")).toHaveText(id);
      });
    } catch (error) { failures.push(`${locale}/${mobile ? 390 : "desktop"}: ${error.message}`); }
    finally { await context.close(); }
  }
  assert.deepEqual(failures, [], "CHECKOUT-V2 new behaviour assertions");
}

export async function checkoutV2Cod(page, quote, locale, recipient, fee, evidence) {
  const ledger = [], file = path.join(evidence, `checkout-v2-cod-${locale}-${page.viewportSize().width}.json`);
  await checkoutV2Review(page, quote, locale, recipient, { codFee: fee, ledger, file });
  const modes = page.getByTestId("home-payment-mode"), cod = modes.locator('input[value="cash_on_delivery"]'), card = modes.locator('input[value="card"]');
  await ledgerStep(ledger, file, page, "payment radios", "click card then COD", "selected state and timing follow the real radio selection", async () => {
    await expect(cod).toBeChecked(); await expect(cod.locator("..")).toHaveClass(/selected/);
    await card.click(); await expect(card).toBeChecked(); await expect(cod).not.toBeChecked();
    await expect(page.locator(".purchase-footer")).toContainText(copy[locale].timing);
    await cod.click(); await expect(cod).toBeChecked(); await expect(card).not.toBeChecked();
    await page.getByTestId("confirm-address").click();
  });
  await page.reload();
  await expect(page.getByTestId("address-section")).toBeVisible();
  await expect(cod).toBeChecked(); // saved choice is revalidated against current server options before it is restored
  // The server retains the recipient; confirmation must be explicit again. Re-selecting COD never writes money.
  await cod.click(); await page.getByTestId("confirm-address").click();
  await checkoutV2Review(page, quote, locale, recipient, { codFee: fee, ledger, file });
}
