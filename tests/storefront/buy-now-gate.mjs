import assert from "node:assert/strict";
import { writeFile } from "node:fs/promises";
import path from "node:path";
import { expect } from "@playwright/test";
import { shopCopy } from "../../apps/storefront/lib/shop-copy.ts";

export async function checkBuyNow({ browser, origin, evidence, ctxOpts, engine, facts, control }) {
  const failures = [];
  for (const [device, width, height] of [["desktop", 1440, 900], ["phone", 390, 844]]) {
    const context = await browser.newContext(ctxOpts({
      ignoreHTTPSErrors: true, viewport: { width, height }, locale: "en",
      ...(width < 500 ? { isMobile: true, hasTouch: true } : {}),
    }));
    const page = await context.newPage();
    const receipt = { engine, device, width, height, checkoutCartWrites: 0, checkoutAttempts: 0, stages: [], requests: [] };
    const product = (key) => facts.products.find((row) => row.key === key);
    const captureOn = (tab, method, suffix) => tab.waitForResponse((response) =>
      response.request().method() === method && new URL(response.url()).pathname === `/api/buyer/${suffix}`, { timeout: 15000 });
    const capture = (method, suffix) => captureOn(page, method, suffix);
    const read = async (response) => {
      assert.equal(response.status(), 200, `${response.request().method()} ${new URL(response.url()).pathname}`);
      return response.json();
    };
    const shot = (stage) => page.screenshot({ path: path.join(evidence, `${device}-${stage}.png`), timeout: 5000 });
    const lines = (items) => items.map(({ sku_id, quantity }) => ({ sku_id, quantity }));
    page.on("response", (response) => {
      const endpoint = new URL(response.url()).pathname.replace(/[0-9a-f-]{36}/g, ":id");
      if (endpoint.startsWith("/api/buyer/")) receipt.requests.push([response.request().method(), endpoint, response.status()]);
    });
    let observeWrites = false;
    let priceRaised = false;
    page.on("request", (req) => {
      const pathname = new URL(req.url()).pathname;
      if (observeWrites && req.method() === "PUT" && pathname === "/api/buyer/cart") receipt.checkoutCartWrites++;
    });

    try {
      receipt.stages.push("seed-original-cart");
      const landing = await page.goto(`${origin}/en/products/${product("candle").slug}`);
      assert.equal(landing.status(), 200);
      const addToCart = page.getByTestId("add-to-cart");
      await expect(addToCart).toBeEnabled();
      const [added] = await Promise.all([capture("PUT", "cart"), addToCart.click()]);
      receipt.before = await read(added);
      assert.equal(receipt.before.items.length, 1);
      // Leaving on the HTTP receipt alone can interrupt the cart reread and journal settlement.
      await expect(page.getByRole("status").filter({ hasText: shopCopy.en.added })).toBeVisible();

      const b = product("dripper");
      receipt.stages.push("site-buy-now");
      const quantity = device === "desktop" ? 1 : 2;
      await page.goto(`${origin}/en/products/${b.slug}`);
      const sku = await page.getByTestId("product-buy").getAttribute("data-sku");
      const selected = [{ sku_id: sku, quantity }];
      await page.getByTestId("qty-input").fill(String(quantity));
      await shot("selection");
      observeWrites = true;
      const [created, options] = await Promise.all([
        capture("POST", "buy-now"), capture("GET", "checkout-options"), page.getByTestId("buy-now").click(),
      ]);
      const checkout = await read(created);
      receipt.checkoutCart = checkout;
      const checkoutURL = `${origin}/en/checkout/buy-now/${checkout.id}`;
      await expect(page).toHaveURL(checkoutURL);
      await expect(page.getByTestId("cart-line").filter({ visible: true }).first()).toBeVisible();
      receipt.checkoutLines = await page.getByTestId("cart-line").filter({ visible: true }).evaluateAll((rows) =>
        rows.map((row) => row.getAttribute("data-sku")));
      await shot("checkout");

      const readRegular = async (tab, action) => {
        const [response] = await Promise.all([captureOn(tab, "GET", "cart"), action()]);
        await expect(tab.getByTestId("cart-line")).toHaveCount(1);
        return read(response);
      };
      const regular = await context.newPage();
      receipt.stages.push("open-cancel-back");
      receipt.afterOpen = await readRegular(regular, () => regular.goto(`${origin}/en/cart`));
      assert.deepEqual(receipt.afterOpen, receipt.before);
      await page.getByRole("link", { name: shopCopy.en.checkoutBack, exact: true }).click();
      await expect(page).toHaveURL(`${origin}/en/cart`);
      receipt.afterCancel = await readRegular(regular, () => regular.reload());
      assert.deepEqual(receipt.afterCancel, receipt.before);
      await page.goBack();
      await expect(page).toHaveURL(checkoutURL);
      await expect(page.getByTestId("cart-line")).toHaveAttribute("data-sku", sku);

      const home = (await read(options)).items.find((row) => row.delivery_kind === "home");
      assert(home, "home delivery fixture missing");
      await page.getByRole("button", { name: "Choose delivery", exact: true }).click();
      await page.getByLabel("Delivery method").selectOption([home.market_id, home.country, home.method].join(":"));
      const [quoted] = await Promise.all([
        capture("POST", "quotes"), page.getByRole("button", { name: "Get current total", exact: true }).click(),
      ]);
      let quote = await read(quoted);
      receipt.quote = { id: quote.id, cart_id: quote.cart_id, lines: lines(quote.lines), amount: quote.amount };
      await page.getByTestId("checkout-details-toggle").click();
      await expect(page.locator(".quotation")).toBeVisible();
      await shot("quote");

      const [returned] = await Promise.all([capture("GET", "cart"), page.goto(`${origin}/en/cart`)]);
      receipt.after = await read(returned);
      await shot("original-cart");

      const subtotal = quantity * b.variants[0].price_minor;
      const shipping = subtotal >= facts.free_shipping.threshold_minor ? 0 : facts.free_shipping.fee_minor;
      assert.deepEqual({ cart: receipt.after, checkoutLines: receipt.checkoutLines, quoteLines: receipt.quote.lines,
        subtotal: quote.amount.subtotal_minor, shipping: quote.amount.shipping_minor, total: quote.amount.total_minor,
        writes: receipt.checkoutCartWrites }, {
        cart: receipt.before, checkoutLines: [sku], quoteLines: selected, subtotal, shipping, total: subtotal + shipping, writes: 0,
      }, "Buy now must quote only the selected SKU and quantity, preserving the original cart and its version");
      assert.notEqual(quote.cart_id, receipt.before.id);
      assert.equal(quote.cart_id, checkout.id);

      async function incrementRegular() {
        const increment = page.getByTestId("cart-line").getByRole("button", { name: shopCopy.en.increase, exact: true });
        await expect(increment).toBeEnabled();
        observeWrites = false;
        try {
          const [result] = await Promise.all([capture("PUT", "cart"), increment.click()]);
          const edited = await read(result);
          assert.deepEqual(edited, { ...receipt.before, version: receipt.before.version + 1,
            items: [{ ...receipt.before.items[0], quantity: 2 }] });
          await expect(page.getByTestId("cart-line-qty")).toHaveText("2");
          return edited;
        } finally {
          observeWrites = true;
        }
      }
      let expectedRegular = receipt.before;
      if (device === "phone") {
        receipt.stages.push("edit-regular-before-submit");
        expectedRegular = receipt.regularEdit = await incrementRegular();
      }

      receipt.stages.push("restore-quote-address");

      const [restored] = await Promise.all([capture("GET", `quotes/${quote.id}`), page.goto(checkoutURL)]);
      assert.equal((await read(restored)).id, quote.id);
      const form = page.getByTestId("checkout-entry");
      for (const [field, value] of Object.entries({ recipient_name: "Synthetic Recipient", phone: "+886900000091", city: "Synthetic City", line1: "Synthetic Street" })) {
        await form.locator(`input[name="${field}"]`).fill(value);
      }
      const next = page.getByTestId("checkout-continue");
      const [savedDestination] = await Promise.all([capture("PUT", "destination"), next.click()]);
      let dest = await read(savedDestination);
      assert.deepEqual([dest.cart_id, dest.cart_version], [checkout.id, checkout.version]);
      receipt.destinationID = dest.id;
      const submit = page.getByTestId("checkout-submit");
      await expect(submit).toBeEnabled();
      await shot("review");

      if (device === "desktop") {
        receipt.stages.push("confirm-changed-price");
        await control("buy-now-price-rise");
        priceRaised = true;
        const [stale] = await Promise.all([capture("POST", "checkout"), submit.click()]);
        assert.equal(stale.status(), 409);
        assert.equal((await stale.json()).code, "conflict");
        await expect(page.getByTestId("order-id")).toHaveCount(0);
        const [freshQuote] = await Promise.all([
          capture("POST", "quotes"), page.getByRole("button", { name: "Get current total", exact: true }).click(),
        ]);
        const old = quote;
        quote = await read(freshQuote);
        assert.equal(quote.amount.total_minor, old.amount.total_minor + 100);
        assert.deepEqual(lines(quote.lines), selected);
        receipt.priceConfirmation = { refused: stale.status(), previous: old.amount.total_minor, current: quote.amount.total_minor };
        receipt.quote = { id: quote.id, cart_id: quote.cart_id, lines: lines(quote.lines), amount: quote.amount };
        const reconfirm = page.getByTestId("quote-reconfirm");
        await expect(reconfirm).toBeVisible();
        await expect(submit).not.toBeEnabled();
        await shot("price-reconfirm");
        await reconfirm.getByRole("button").click();
        const [newDestination] = await Promise.all([capture("PUT", "destination"), next.click()]);
        dest = await read(newDestination);
        receipt.destinationID = dest.id;
        await expect(submit).toBeEnabled();
      }

      receipt.stages.push("submit-and-recover");

      const firstReply = Promise.withResolvers();
      const submitKeys = [];
      await page.route("**/api/buyer/checkout", async (route) => {
        const request = route.request();
        receipt.checkoutAttempts++;
        submitKeys.push(await request.headerValue("idempotency-key"));
        if (submitKeys.length !== 1 || device !== "desktop") return route.continue();
        try {
          const result = await route.fetch(); // commit on the real stack, then lose only the browser's acknowledgement
          firstReply.resolve({ status: result.status(), result: await result.json() });
        } catch (error) { firstReply.reject(error); }
        await route.abort("connectionreset");
      });
      if (device === "desktop") {
        await submit.dblclick();
        const first = await firstReply.promise;
        assert.equal(first.status, 200, `placed request: ${first.result.code ?? first.status}`);
        await expect(page.getByTestId("checkout-unknown")).toBeVisible();
        await shot("checkout-unknown");
        await page.reload();
        await expect(page.getByTestId("checkout-submit")).toBeDisabled();
        const [retry] = await Promise.all([capture("POST", "checkout"), page.getByTestId("recover-purchase").click()]);
        const found = await read(retry);
        assert.deepEqual(found, first.result);
        assert(submitKeys[0]);
        assert.deepEqual(submitKeys, [submitKeys[0], submitKeys[0]]);
        receipt.recoveredSameRequest = true;
      } else {
        const [submitted] = await Promise.all([capture("POST", "checkout"), submit.dblclick()]);
        await read(submitted);
        assert.equal(receipt.checkoutAttempts, 1);
      }

      const orderID = (await page.getByTestId("order-id").innerText()).trim();
      receipt.stages.push("read-order-and-cart");
      const [orderResponse] = await Promise.all([capture("GET", `orders/${orderID}`), page.reload()]);
      const order = await read(orderResponse);
      assert.deepEqual([order.cart_id, order.cart_version], [checkout.id, checkout.version]);
      assert.deepEqual(lines(order.snapshot.quote.lines), selected);
      assert.deepEqual(order.snapshot.quote.amount, quote.amount);
      await expect(page.locator(".order-lines > li")).toHaveCount(1);
      await expect(page.locator(".order-lines > li")).toContainText(`${b.variants[0].code} × ${quantity}`);
      receipt.order = { id: order.order_id, cart_id: order.cart_id, lines: selected, amount: order.snapshot.quote.amount };
      await shot("order");
      await page.getByTestId("continue-shopping").click();
      await expect(page).toHaveURL(`${origin}/en/cart`);
      receipt.afterOrder = await readRegular(regular, () => regular.reload());
      assert.deepEqual(receipt.afterOrder, expectedRegular);
      if (device === "desktop") {
        receipt.stages.push("edit-regular-after-order");
        receipt.regularEdit = await incrementRegular();
      }
      assert.equal(receipt.checkoutCartWrites, 0);
      await regular.close();
    } catch (error) {
      receipt.failure = error.message;
      failures.push(`${device}: ${error.message}`);
      await shot("failed").catch(() => {});
    } finally {
      if (priceRaised) await control("buy-now-price-restore");
      await writeFile(path.join(evidence, `${device}-buy-now.json`), JSON.stringify(receipt, null, 2) + "\n");
      await context.close();
    }
  }
  assert.deepEqual(failures, []);
  console.log("BUY_NOW_ISOLATION desktop=PASS phone=PASS");
}
