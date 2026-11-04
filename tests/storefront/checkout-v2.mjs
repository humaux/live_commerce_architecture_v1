// Purpose: CHECKOUT-V2-IMPL real-click review, payment-help and re-quote acceptance.
// Depends on: Playwright, the existing order/home-COD gates and synthetic buyer fixtures.
// Used by: order-gate.mjs and home-cod-buyer.mjs in GitHub CI only.
// Invariants: money assertions use response fields; evaluate only reads layout; no checkout with a MOCK quote.
import assert from "node:assert/strict";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { expect } from "@playwright/test";
import { formatMoney } from "../../apps/storefront/lib/money.ts";
import { reachCheckout, confirmCheckoutAddress, openCheckoutDetails, recoverPurchase } from "./shop-helpers.mjs";

const copy = {
  en: { timing: "Pay online after placing the order", open: "Expand payment help", close: "Collapse payment help", changed: "Total updated. Please confirm again.", confirm: "Confirm new total", change: "Change delivery", quote: "Get current total" },
  "zh-TW": { timing: "送出後線上付款", open: "展開付款說明", close: "收合付款說明", changed: "金額已更新，請再確認", confirm: "確認新金額", change: "重新選擇配送", quote: "取得目前總額" },
  "zh-CN": { timing: "提交后在线付款", open: "展开付款说明", close: "收合付款说明", changed: "金额已更新，请再确认", confirm: "确认新金额", change: "重新选择配送", quote: "取得目前总额" },
};
const localeScreen = {
  en: { delivery: "Choose delivery", quote: "Get current total", timing: copy.en.timing, open: copy.en.open, close: copy.en.close },
  "zh-TW": { delivery: "選擇配送", quote: "取得目前總額", timing: copy["zh-TW"].timing, open: copy["zh-TW"].open, close: copy["zh-TW"].close },
  "zh-CN": { delivery: "选择配送", quote: "获取当前总额", timing: "提交后在线付款", open: "展开付款说明", close: "收起付款说明" },
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
      await confirmCheckoutAddress(page);
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
          await confirmCheckoutAddress(page);
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
      await confirmCheckoutAddress(page);
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
          recoverPurchase(page),
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
        await confirmCheckoutAddress(page);
        await expect(page.getByTestId("create-order")).toBeDisabled();
        await expect(page.getByTestId("checkout-submit")).toBeDisabled();
        await page.getByRole("button", { name: copy.en.confirm, exact: true }).click();
        await expect(page.getByTestId("create-order")).toBeEnabled();
        await page.reload();
        await expect(page.getByTestId("address-section")).toBeVisible();
        await confirmCheckoutAddress(page);
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
  const timing = codFee === null ? c.timing : locale === "en" ? "Cash on delivery" : "貨到付款";
  await openCheckoutDetails(page);
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
  const confirm = async (page) => { await confirmCheckoutAddress(page); await expect(page.getByTestId("create-order")).toBeEnabled(); };
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
          await ledgerStep(ledger, file, page, "payment help", "click twice", "Signed question-mark, accessible name and >=44px target track expanded state", async () => {
            await expect(help).toHaveText("?"); await expect(help).toHaveAccessibleName("Payment timing help"); await expect(help).toHaveAttribute("aria-expanded", "false");
            await expect(help).toBeVisible();
            const box = await help.boundingBox(); assert(box && box.height >= 44 && box.width >= 44);
            await help.click(); await expect(help).toHaveText("?"); await expect(help).toHaveAccessibleName("Payment timing help"); await expect(help).toHaveAttribute("aria-expanded", "true");
            await expect(page.getByTestId("payment-help-content")).toBeVisible();
            await help.click(); await expect(help).toHaveText("?"); await expect(help).toHaveAccessibleName("Payment timing help"); await expect(page.getByTestId("payment-help-content")).toBeHidden();
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
          await fill(page); await confirmCheckoutAddress(page);
          await expect(page.getByTestId("create-order")).toBeDisabled(); await expect(page.getByTestId("checkout-submit")).toBeDisabled();
          await page.reload(); await expect(page.getByTestId("quote-reconfirm")).toContainText(copy.en.changed);
          await confirmCheckoutAddress(page); await expect(page.getByTestId("create-order")).toBeDisabled();
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
        await radio.click(); await confirmCheckoutAddress(page);
        await expect(radio).toBeChecked(); await expect(radio.locator("..")).toHaveClass(/selected/);
        await expect(review).toContainText(recipient.recipient_name); await expect(review).toContainText(recipient.phone);
        await expect(page.locator(".purchase-footer")).toContainText(words.timing);
        await expect(review.getByTestId("review-total")).toHaveText(formatMoney(locale, quote.amount.total_minor, quote.currency));
        const help = page.getByTestId("payment-help-toggle");
        await help.click(); await expect(help).toHaveText("?"); await expect(page.getByTestId("payment-help-content")).toBeVisible();
        await help.click(); await expect(help).toHaveText("?");
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
  try { await checkoutParitySuite({ newContext, origin, product, recipient, evidence }); }
  catch (error) { failures.push(error.message); }
  assert.deepEqual(failures, [], "CHECKOUT-V2 new behaviour assertions");
}

// PARITY-CHECKOUT authority: ux-checkout/DELIVERY.md, current controls polish 2026-10-29.
// Expectations are signed buyer copy, independent of the implementation's translation exports.
const parityCopy = {
  "zh-TW": { title: "核對訂單", total: "最終應付", codDue: "到貨需付", details: "查看明細", help: "付款時機說明", online: "送出後線上付款", cod: "貨到付款", submit: "送出訂單", submitting: "正在送出訂單", unknown: "確認訂單狀態中", confirm: "確認新金額", legacy: ["結帳", "本次報價商品"], legacyTotal: "訂單金額" },
  "zh-CN": { title: "核对订单", total: "最终应付", codDue: "到货需付", details: "查看明细", help: "付款时间说明", online: "提交后在线付款", cod: "货到付款", submit: "提交订单", submitting: "正在提交订单", unknown: "正在确认订单状态", confirm: "确认新金额", legacy: ["结账", "本次报价商品"], legacyTotal: "订单金额" },
  en: { title: "Review your order", total: "Final amount due", codDue: "Due on delivery", details: "View details", help: "Payment timing help", online: "Pay online after placing the order", cod: "Cash on delivery", submit: "Place order", submitting: "Submitting order", unknown: "Checking order status", confirm: "Confirm new total", legacy: ["Checkout", "Items in this quotation"], legacyTotal: "Order total" },
};
const parityFrames = {
  card: { default: ["1429:23435", "1429:23484"], reconfirm: ["1429:23533", "1429:23577"], submitting: ["1429:23621", "1429:23661"], unknown: ["1429:23701", "1429:23741"] },
  cash_on_delivery: { default: ["1429:24143", "1429:24194"], reconfirm: ["1429:24245", "1429:24291"], submitting: ["1429:24337", "1429:24379"], unknown: ["1429:24421", "1429:24463"] },
};

async function checkoutParitySuite({ newContext, origin, product, recipient, evidence }) {
  const ledger = [], failures = [], manifest = [], measurements = [];
  const file = path.join(evidence, "parity-checkout-click-ledger.json");
  await mkdir(path.join(evidence, "parity"), { recursive: true });
  // Two NT$12.50 units meet the existing whole-TWD COD rule; one unit cannot offer COD.
  const cases = ["zh-TW", "zh-CN", "en"].flatMap((locale) => [false, true].flatMap((mobile) =>
    (locale === "zh-TW" ? ["card", "cash_on_delivery"] : ["card"]).map((mode) => ({ locale, mobile, mode }))));
  for (const { locale, mobile, mode } of cases) {
    const c = parityCopy[locale], old = localeScreen[locale], context = await newContext(mobile), page = await context.newPage();
    const label = `${locale}/${mobile ? 390 : 1440}/${mode}`;
    const check = async (control, expected, perform) => {
      try { await ledgerStep(ledger, file, page, `${label} ${control}`, "real click/fill/select/keyboard or read-only assertion", expected, perform); }
      catch (error) { failures.push(`${label} ${control}: ${error.message}`); }
    };
    const shot = async (state, target = page) => {
      // Only TW home-online/COD frames match this fixture's delivery method. Other locales
      // validate translated home checkout; never mislabel them as signed pickup frames.
      const frame = locale === "zh-TW" ? parityFrames[mode][state][Number(mobile)] : null;
      // GitHub artifact paths reject colons; keep the exact Figma ID in the manifest.
      const relative = `parity/${frame?.replaceAll(":", "-") ?? `${locale}-${mode}-${state}-${mobile ? 390 : 1440}`}.png`;
      await target.screenshot({ path: path.join(evidence, relative), fullPage: false, animations: "disabled", scale: "css" });
      manifest.push({ frame, locale, mode, state, viewport: target.viewportSize(), route: new URL(target.url()).pathname, screenshot: relative });
      await writeFile(path.join(evidence, "parity-checkout-screenshots.json"), JSON.stringify(manifest, null, 2));
    };
    const fill = async () => { for (const [key, value] of Object.entries(recipient)) await page.locator(`input[name="${key}"]`).fill(value); };
    const confirmAddress = async () => { await page.getByTestId("confirm-address").click(); await expect(page.getByTestId("checkout-submit")).toBeEnabled(); };
    try {
      await reachCheckout(page, origin, locale, product, { quantity: 2 });
      assert.equal(new URL(page.url()).pathname, `/${locale}/checkout`, "parity must exercise the real route");
      await page.getByRole("button", { name: old.delivery, exact: true }).click();
      await check("delivery selector", "all option labels use buyer wording without raw country codes", async () => {
        const select = page.getByRole("combobox");
        await expect(select).toBeVisible();
        const labels = await select.locator("option").allTextContents();
        assert(labels.length > 0);
        assert(labels.every((text) => !/·\s*[A-Z]{2}(?:\s*·|\s*$)/.test(text)), `raw country code: ${labels.join(" | ")}`);
        const selected = await select.inputValue();
        await select.selectOption(selected);
      });
      const [response] = await Promise.all([
        page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST"),
        page.getByRole("button", { name: old.quote, exact: true }).click(),
      ]);
      assert.equal(response.status(), 200);
      const quote = await response.json();
      await expect(page.getByTestId("address-section")).toBeVisible();
      await check("disabled token", "disabled submit is solid light grey without a dashed border", async () => {
        const button = page.getByTestId("checkout-submit");
        await expect(button).toBeDisabled();
        await expect(button).toHaveAccessibleDescription(/\S{6,}/);
        // READ/MEASURE only: computed paint never mutates DOM or application state.
        const paint = await button.evaluate((el) => { const s = getComputedStyle(el); return { background: s.backgroundColor, border: s.borderTopStyle, opacity: s.opacity }; });
        assert.equal(paint.background, "rgb(231, 237, 243)");
        assert.notEqual(paint.border, "dashed");
        assert.equal(paint.opacity, "1");
      });
      await fill();
      const radio = page.locator(`input[name="home_payment_mode"][value="${mode}"]`);
      await radio.click(); await confirmAddress();
      const footer = page.locator(".purchase-footer"), review = page.getByTestId("checkout-review");
      const payable = formatMoney(locale, mode === "cash_on_delivery" ? quote.cod_due_minor : quote.amount.total_minor, quote.currency);
      await check("signed screen", "one review heading; legacy long-form headings and amount label are absent", async () => {
        await expect(page.getByRole("heading", { level: 1, name: c.title, exact: true })).toBeVisible();
        await expect(page.getByRole("heading", { name: c.title, exact: true })).toHaveCount(1);
        for (const name of c.legacy) await expect(page.getByRole("heading", { name, exact: true })).toHaveCount(0);
        await expect(page.locator(".address-fields input:visible")).toHaveCount(0);
        await expect(footer.getByText(c.legacyTotal, { exact: true })).toHaveCount(0);
      });
      await check("required facts", "destination, recipient and phone remain together; a single server final total anchors the bar", async () => {
        for (const key of ["recipient_name", "phone", "line1", "city"]) await expect(review).toContainText(recipient[key]);
        await expect(radio).toBeChecked();
        // #422: the COD bar carries the Figma due label (OC-09b); every other mode keeps the final-total label.
        await expect(footer.getByText(mode === "cash_on_delivery" ? c.codDue : c.total, { exact: true })).toBeVisible();
        await expect(footer.locator("strong")).toHaveText(payable);
        await expect(page.getByText(payable, { exact: true }).and(page.locator(":visible"))).toHaveCount(1);
        await expect(footer).toContainText(mode === "cash_on_delivery" ? c.cod : c.online);
        await expect(footer.getByRole("button", { name: c.submit, exact: true })).toBeEnabled();
      });
      await check("primary token", "enabled primary is the signed solid navy", async () => {
        // READ/MEASURE only; exact signed primary #142942, not the legacy teal.
        const background = await page.getByTestId("checkout-submit").evaluate((el) => getComputedStyle(el).backgroundColor);
        assert.equal(background, "rgb(20, 41, 66)");
      });
      await check("details disclosure", "underlined View details opens and closes server breakdown using real clicks", async () => {
        const details = footer.getByRole("button", { name: c.details, exact: true });
        await expect(details).toHaveAttribute("aria-expanded", "false");
        await expect(page.getByTestId("review-subtotal")).toBeHidden();
        assert.match(await details.evaluate((el) => getComputedStyle(el).textDecorationLine), /underline/);
        await details.click();
        await expect(details).toHaveAttribute("aria-expanded", "true");
        await expect(page.getByTestId("review-subtotal")).toHaveText(formatMoney(locale, quote.amount.subtotal_minor, quote.currency));
        await expect(page.getByTestId("review-shipping")).toHaveText(formatMoney(locale, quote.amount.shipping_minor, quote.currency));
        await details.click(); await expect(page.getByTestId("review-subtotal")).toBeHidden();
      });
      await check("contextual help", "question-mark help beside timing toggles the selected method explanation", async () => {
        const help = footer.getByRole("button", { name: c.help, exact: true });
        await expect(help).toHaveText("?");
        await expect(help).toHaveAttribute("aria-expanded", "false");
        await help.click(); await expect(help).toHaveAttribute("aria-expanded", "true");
        await expect(page.getByTestId("payment-help-content")).toBeVisible();
        await help.click(); await expect(page.getByTestId("payment-help-content")).toBeHidden();
      });
      await check("fixed bar clearance", "last content and controls scroll above the bar at the signed viewport", async () => {
        await page.locator("body").click({ position: { x: 2, y: 2 } }); await page.keyboard.press("ControlOrMeta+End");
        // READ/MEASURE only; all scrolling above is keyboard input.
        await expect.poll(() => page.getByTestId("checkout-content-end").evaluate((el) => {
          const bar = document.querySelector(".purchase-footer");
          return !!bar && el.getBoundingClientRect().bottom <= bar.getBoundingClientRect().top;
        })).toBe(true);
        const geometry = await footer.evaluate((el) => { const box = el.getBoundingClientRect(); return { bottom: box.bottom, height: innerHeight, overflow: document.documentElement.scrollWidth > innerWidth }; });
        assert.equal(geometry.bottom, geometry.height); assert.equal(geometry.overflow, false);
        measurements.push({ label, ...geometry });
      });
      await page.keyboard.press("ControlOrMeta+Home"); await shot("default");
      await check("refresh persistence", "server recipient, selected method, current final amount and single review survive reload", async () => {
        await page.reload(); await expect(page.getByTestId("address-section")).toBeVisible();
        await expect(radio).toBeChecked(); await confirmAddress();
        await expect(page.getByRole("heading", { level: 1, name: c.title, exact: true })).toBeVisible();
        for (const key of ["recipient_name", "phone", "line1"]) await expect(review).toContainText(recipient[key]);
        await expect(footer.locator("strong")).toHaveText(payable);
      });
      // MOCK response fixture only: the same response is pinned across reload, and is
      // never sent to checkout. Server money is shown as given, never recomputed by UI.
      let changedQuote;
      await page.route("**/api/buyer/quotes**", async (route) => {
        const result = await route.fetch(), body = await result.json();
        if (result.status() !== 200) return route.fulfill({ response: result });
        if (route.request().method() === "POST") changedQuote = { ...body, amount: { ...body.amount, total_minor: body.amount.total_minor + 1700, shipping_minor: body.amount.shipping_minor + 1700 }, ...(mode === "cash_on_delivery" ? { cod_due_minor: body.cod_due_minor + 1700 } : {}) };
        await route.fulfill({ response: result, json: changedQuote ?? body });
      });
      await check("re-quote", "changed server total blocks submit until a separate Confirm new total click, including after reload", async () => {
        const details = footer.getByRole("button", { name: c.details, exact: true });
        await details.click();
        await page.getByRole("button", { name: copy[locale].change, exact: true }).click();
        await page.getByRole("button", { name: old.quote, exact: true }).click();
        await expect(page.getByTestId("quote-reconfirm")).toBeVisible();
        // A restored destination collapses its editor on the signed review; edit through View details.
        await openCheckoutDetails(page);
        if (mode === "card") {
          await expect(page.locator('input[name="home_payment_mode"][value="card"]')).toBeEnabled();
          // This unchanged MOCK pins card money while COD due no longer matches
          // its current fee. Card review stays usable; stale COD is not offered.
          await expect(page.locator('input[name="home_payment_mode"][value="cash_on_delivery"]')).toHaveCount(0);
        }
        await fill(); await page.locator(`input[name="home_payment_mode"][value="${mode}"]`).click();
        await page.getByTestId("confirm-address").click();
        await expect(page.getByTestId("checkout-submit")).toBeDisabled();
        await expect(footer.locator("strong")).toHaveText(formatMoney(locale, mode === "cash_on_delivery" ? changedQuote.cod_due_minor : changedQuote.amount.total_minor, quote.currency));
        await page.keyboard.press("ControlOrMeta+Home"); await shot("reconfirm");
        await page.reload(); await expect(page.getByTestId("quote-reconfirm")).toBeVisible();
        await page.getByTestId("confirm-address").click();
        await expect(page.getByTestId("checkout-submit")).toBeDisabled();
        await page.getByRole("button", { name: c.confirm, exact: true }).click();
        await expect(page.getByTestId("checkout-submit")).toBeEnabled();
      });
      await page.unrouteAll({ behavior: "wait" });
    } catch (error) { failures.push(`${label} setup: ${error.message}`); }
    finally { await context.close(); }
    // New isolated buyer: a MOCK changed quote must never reach the real order writer.
    const submissionContext = await newContext(mobile), submittingPage = await submissionContext.newPage();
    let releaseSubmit = () => {}, releaseTimer;
    try {
      await reachCheckout(submittingPage, origin, locale, product, { quantity: 2 });
      await submittingPage.getByRole("button", { name: old.delivery, exact: true }).click();
      const [response] = await Promise.all([
        submittingPage.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST"),
        submittingPage.getByRole("button", { name: old.quote, exact: true }).click(),
      ]);
      assert.equal(response.status(), 200);
      await expect(submittingPage.getByTestId("address-section")).toBeVisible();
      for (const [key, value] of Object.entries(recipient)) await submittingPage.locator(`input[name="${key}"]`).fill(value);
      await submittingPage.locator(`input[name="home_payment_mode"][value="${mode}"]`).click();
      await submittingPage.getByTestId("confirm-address").click();
      const held = new Promise((resolve) => { releaseSubmit = resolve; });
      // MOCK transport fault only; do not contact PSP/carrier or create an order with a modified quote.
      let submits = 0;
      await submittingPage.route("**/api/buyer/checkout", async (route) => { submits++; await held; await route.abort("failed"); });
      releaseTimer = setTimeout(releaseSubmit, 15000); // bounded fault injection, always released in finally
      await submittingPage.getByTestId("checkout-submit").click();
      await expect.poll(() => submits).toBe(1);
      try {
        await ledgerStep(ledger, file, submittingPage, `${label} submitting`, "real submit click with held transport",
          "signed Submitting order label disables resubmission while required recipient facts remain", async () => {
            await expect(submittingPage.getByTestId("checkout-submit")).toHaveText(c.submitting);
            await expect(submittingPage.getByTestId("checkout-submit")).toBeDisabled();
            for (const key of ["recipient_name", "phone", "line1"]) await expect(submittingPage.getByTestId("checkout-review")).toContainText(recipient[key]);
          });
      } catch (error) { failures.push(`${label} submitting: ${error.message}`); }
      await shot("submitting", submittingPage);
      releaseSubmit(); clearTimeout(releaseTimer);
      await expect(submittingPage.getByTestId("recover-purchase")).toBeVisible();
      try {
        await ledgerStep(ledger, file, submittingPage, `${label} UNKNOWN`, "transport loss then real reload",
          "Checking order status is read-only, hides details/help and payable, preserves recipient and never replays checkout", async () => {
            const bar = submittingPage.locator(".purchase-footer");
            await expect(submittingPage.getByTestId("checkout-submit")).toHaveText(c.unknown);
            await expect(submittingPage.getByTestId("checkout-submit")).toBeDisabled();
            await expect(bar.locator("strong")).toHaveText("—");
            await expect(bar.getByRole("button", { name: c.details, exact: true })).toHaveCount(0);
            await expect(bar.getByRole("button", { name: c.help, exact: true })).toHaveCount(0);
            for (const key of ["recipient_name", "phone", "line1"]) await expect(submittingPage.getByTestId("checkout-review")).toContainText(recipient[key]);
            await submittingPage.reload();
            await expect(submittingPage.getByTestId("recover-purchase")).toBeVisible();
            await expect(submittingPage.getByTestId("checkout-submit")).toBeDisabled();
            await expect(bar.locator("strong")).toHaveText("—");
            assert.equal(submits, 1, "reload cannot replay an UNKNOWN write");
          });
      } catch (error) { failures.push(`${label} UNKNOWN: ${error.message}`); }
      await shot("unknown", submittingPage);
    } catch (error) { failures.push(`${label} submission setup: ${error.message}`); }
    finally { releaseSubmit(); clearTimeout(releaseTimer); await submittingPage.unrouteAll({ behavior: "wait" }); await submissionContext.close(); }
  }
  await writeFile(path.join(evidence, "parity-checkout-layout.json"), JSON.stringify(measurements, null, 2));
  assert.deepEqual(failures, [], "PARITY-CHECKOUT signed-frame assertions (not compile/startup)");
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
    await confirmCheckoutAddress(page);
  });
  await page.reload();
  await expect(page.getByTestId("address-section")).toBeVisible();
  await expect(cod).toBeChecked(); // saved choice is revalidated against current server options before it is restored
  // The server retains the recipient; confirmation must be explicit again. Re-selecting COD never writes money.
  await cod.click(); await confirmCheckoutAddress(page);
  await checkoutV2Review(page, quote, locale, recipient, { codFee: fee, ledger, file });
}
