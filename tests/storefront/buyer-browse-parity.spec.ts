// Purpose: minimal C7 TESTONLY candidate for product option draft cancellation on the real product route.
// Depends on: Playwright, existing SFR candle fixture and browser-engine context options; no fixture writes.
// Used by: proposed root-owned shop-real-gate hook after SFR04 (not registered by this leaf).
// Invariants: real clicks only; draft/back must not write the cart or change committed choices.
import assert from "node:assert/strict";
import { writeFile } from "node:fs/promises";
import path from "node:path";
import { expect, type Browser } from "@playwright/test";
import { ctxOpts } from "./browser-engine.mjs";

/** Exercise one synthetic SFR product's draft/back journey; writes only a click ledger in evidence. */
export async function checkC7DraftCancel({ browser, origin, evidence, productSlug }: {
  browser: Browser; origin: string; evidence: string; productSlug: string;
}) {
  const context = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true,
    viewport: { width: 375, height: 844 }, locale: "en", isMobile: true, hasTouch: true }));
  const page = await context.newPage();
  const ledger: { control: string; expected: string; status: "PASS" | "FAIL" }[] = [];
  const writes: string[] = [];
  page.on("request", request => {
    // Passive observation of the real BFF seam, never intercept or replay a write.
    if (new URL(request.url()).pathname === "/api/buyer/cart" && !["GET", "HEAD"].includes(request.method())) writes.push(request.method());
  });
  const step = async (control: string, expected: string, action: () => Promise<void>) => {
    try { await action(); ledger.push({ control, expected, status: "PASS" }); }
    catch (error) { ledger.push({ control, expected, status: "FAIL" }); throw error; }
  };
  try {
    await page.goto(`${origin}/en/cart`);
    await expect(page.getByTestId("cart-empty")).toBeVisible();
    await page.goto(`${origin}/en/products/${productSlug}`);
    const inline = page.getByTestId("product-buy");
    // These unchanged SFR04 controls must work before a missing feature can be called causal RED.
    await step("inline Size 300g / quantity +", "committed 300g, quantity 2", async () => {
      await inline.locator("label.sf-chip", { has: page.getByRole("radio", { name: "300g", exact: true }) }).click();
      await expect(inline.getByRole("radio", { name: "300g", exact: true })).toBeChecked();
      await inline.getByTestId("qty-input").locator("xpath=following-sibling::button").click();
      await expect(inline.getByTestId("qty-input")).toHaveValue("2");
    });
    const committedSKU = await inline.getAttribute("data-sku");
    assert(committedSKU, "baseline must resolve a real SKU");
    assert.deepEqual(writes, [], "baseline selection cannot write cart");
    const entry = page.getByTestId("product-options-task-entry").getByRole("button");
    const task = page.getByTestId("product-options-task");
    await step("spec entry", "opens signed spec task (expected first feature RED on current base)", async () => {
      await expect(entry).toBeVisible();
      await entry.click();
      await expect(task).toBeVisible();
    });
    await step("draft Size 180g / quantity + / Back", "discard draft and restore committed selection", async () => {
      await task.locator("label.sf-chip", { has: page.getByRole("radio", { name: "180g", exact: true }) }).click();
      await expect(task.getByRole("radio", { name: "180g", exact: true })).toBeChecked();
      await task.getByTestId("qty-input").locator("xpath=following-sibling::button").click();
      await expect(task.getByTestId("qty-input")).toHaveValue("3");
      await task.getByRole("button", { name: "Back to product", exact: true }).click();
      await expect(task).not.toBeVisible();
      await expect(inline).toHaveAttribute("data-sku", committedSKU);
      await expect(inline.getByRole("radio", { name: "300g", exact: true })).toBeChecked();
      await expect(inline.getByTestId("qty-input")).toHaveValue("2");
      await expect(entry).toBeFocused();
      await entry.click();
      await expect(task.getByRole("radio", { name: "300g", exact: true })).toBeChecked();
      await expect(task.getByTestId("qty-input")).toHaveValue("2");
      await task.getByRole("button", { name: "Back to product", exact: true }).click();
    });
    await step("cart / reload", "cart stays empty after cancellation and refresh", async () => {
      await page.goto(`${origin}/en/cart`);
      await expect(page.getByTestId("cart-empty")).toBeVisible();
      await page.reload();
      await expect(page.getByTestId("cart-empty")).toBeVisible();
      assert.deepEqual(writes, [], "draft and cancel must issue zero cart mutations");
    });
  } finally {
    try { await writeFile(path.join(evidence, "c7-draft-cancel-ledger.json"), JSON.stringify({
      environment: "SYNTHETIC_REAL_STACK", locale: "en", viewport: "375x844", ledger, cartWrites: writes,
    }, null, 2)); } finally { await context.close(); }
  }
}

// C7 final preparation is additive: the original first-RED journey above remains byte-for-byte frozen.
import { createHash } from "node:crypto";
import type { BrowserContext, Page, Request, Response } from "@playwright/test";
import { shopCopy } from "../../apps/storefront/lib/shop-copy.ts";

type C7Facts = { currency: string; products: { key: string; id: string; slug: string; title: string; variants: { values: string[]; price_minor: number; stock: number }[] }[] };
type C7Cell = {
  locale: "zh-TW" | "zh-CN" | "en"; width: number; kind: "mobile" | "desktop" | "cas";
  status: "NOT_RUN" | "PASS" | "FAIL"; failure?: string;
  actions: { control: string; route: string; operation: string; expected: string; actual: string; status: "PASS" | "FAIL" }[];
  screenshots: { file: string; sha256: string; requested: string; requestedDPR: number; actualPNG: string; actualCSS: string; actualDPR: number }[];
  writes: { method: string; keySHA256: string; bodySHA256: string }[];
};
type C7Cart = { version: number; items: { sku_id: string; quantity: number }[] };
const c7Hash = (value: string) => createHash("sha256").update(value).digest("hex");
const c7CartRequest = (request: Request) => new URL(request.url()).pathname === "/api/buyer/cart" && request.method() === "PUT";
const c7CartResponse = (response: Response) => c7CartRequest(response.request());

/** Extend the existing REAL SFR journey with synthetic real-click option/cancel/cart and fault cases; write bounded evidence only. */
export async function checkC7Matrix({ browser, origin, evidence, facts }: {
  browser: Browser; origin: string; evidence: string; facts: C7Facts;
}) {
  const products = (key: string) => {
    const product = facts.products.find(p => p.key === key);
    assert(product, `C7 fixture missing ${key}`);
    return product;
  };
  const candle = products("candle"), scarf = products("scarf"), diffuser = products("diffuser");
  const cells: C7Cell[] = (["zh-TW", "zh-CN", "en"] as const).flatMap(locale =>
    [375, 390, 1440].map(width => ({ locale, width, kind: width === 1440 ? "desktop" as const : "mobile" as const,
      status: "NOT_RUN" as const, actions: [], screenshots: [], writes: [] })));
  cells.push({ locale: "en", width: 375, kind: "cas", status: "NOT_RUN", actions: [], screenshots: [], writes: [] });
  const gaps = [{ case: "long-label", status: "GAP", reason: "Existing SFR candle/scarf option values are short; no long-label fixture is authorized." }];
  const save = () => writeFile(path.join(evidence, "c7-matrix.json"), JSON.stringify({
    environment: "SYNTHETIC_REAL_STACK", cells, gaps, visualAcceptance: "NOT_EVALUATED",
  }, null, 2));
  await save();
  for (const cell of cells) {
    let context: BrowserContext | undefined;
    let routedPage: Page | undefined;
    try {
      const height = cell.width === 1440 ? 900 : 844;
      context = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width: cell.width, height }, deviceScaleFactor: 1,
        locale: cell.locale, isMobile: cell.width < 900, hasTouch: cell.width < 900 }));
      const page = await context.newPage(), copy = shopCopy[cell.locale];
      routedPage = page;
      const calls: { method: string; key: string; body: string }[] = [];
      const observe = (target: Page) => target.on("request", request => {
        if (new URL(request.url()).pathname !== "/api/buyer/cart" || ["GET", "HEAD"].includes(request.method())) return;
        // Keep request bytes/key only in this context's memory; persist hashes, never the request or identity headers.
        const key = request.headers()["idempotency-key"] ?? "", body = request.postData() ?? "";
        calls.push({ method: request.method(), key, body });
        cell.writes.push({ method: request.method(), keySHA256: c7Hash(key), bodySHA256: c7Hash(body) });
      });
      observe(page);
      const step = async (control: string, action: () => Promise<void>) => {
        const row = { control, route: new URL(page.url()).pathname, operation: "real click/fill/keyboard and visible-result assertions", expected: control };
        try { await action(); cell.actions.push({ ...row, actual: "all assertions passed", status: "PASS" }); }
        catch (error) { cell.actions.push({ ...row, actual: "assertion/action failed", status: "FAIL" }); throw error; }
      };
      const shot = async (label: string) => {
        const file = `c7-${cell.locale}-${cell.width}-${cell.kind}-${label}.png`;
        // Read-only image readiness, matching SFR's naturalWidth check; no asset substitutions.
        await expect.poll(() => page.locator("img:visible").evaluateAll(images => images.every(image => image instanceof HTMLImageElement && image.complete && image.naturalWidth > 0))).toBe(true);
        const png = await page.screenshot({ path: path.join(evidence, file), fullPage: false });
        assert.equal(png.subarray(1, 4).toString(), "PNG");
        // Read-only measurement: record CSS and unmodified PNG dimensions independently (DPR may differ by engine).
        const css = await page.evaluate(() => ({ size: `${window.innerWidth}x${window.innerHeight}`, dpr: window.devicePixelRatio }));
        cell.screenshots.push({ file, sha256: createHash("sha256").update(png).digest("hex"), requested: `${cell.width}x${height}`, requestedDPR: 1,
          actualPNG: `${png.readUInt32BE(16)}x${png.readUInt32BE(20)}`, actualCSS: css.size, actualDPR: css.dpr });
      };
      const gotoProduct = (target: Page, slug = candle.slug) => target.goto(`${origin}/${cell.locale}/products/${slug}`);
      const inline = () => page.getByTestId("product-buy");
      const task = () => page.getByTestId("product-options-task");
      const entry = () => page.getByTestId("product-options-task-entry").getByRole("button");
      const cartLine = () => page.getByTestId("cart-line");
      const checkTaskGeometry = async () => {
        const [taskBox, headerBox, dockBox] = await Promise.all([
          task().boundingBox(), task().locator("header").boundingBox(), task().locator("footer").boundingBox(),
        ]);
        assert(taskBox && headerBox && dockBox, "C7 task, header and dock must have rendered bounds");
        assert.equal(taskBox.x, 0); assert.equal(taskBox.y, 0);
        assert.equal(taskBox.width, cell.width); assert.equal(taskBox.height, height);
        assert.equal(headerBox.x, 0); assert.equal(headerBox.y, 0);
        assert.equal(headerBox.width, cell.width); assert.equal(headerBox.height, 64);
        assert.equal(dockBox.x, 0); assert.equal(dockBox.y, height - 84);
        assert.equal(dockBox.width, cell.width); assert.equal(dockBox.height, 84);
      };
      const checkCart = async (sku: string, qty: number) => {
        await expect(cartLine()).toHaveCount(1);
        await expect.poll(async () => (await cartLine().getAttribute("data-sku")) === sku).toBe(true);
        await expect(cartLine()).toContainText("180g");
        await expect(cartLine().getByTestId("cart-line-qty")).toHaveText(String(qty));
      };
      const putClick = async (action: () => Promise<void>) => {
        const [response] = await Promise.all([page.waitForResponse(c7CartResponse), action()]);
        assert.equal(response.status(), 200);
        return await response.json() as C7Cart;
      };
      await step("empty initial cart", async () => {
        await page.goto(`${origin}/${cell.locale}/cart`);
        await expect(page.getByTestId("cart-empty")).toBeVisible();
      });
      await gotoProduct(page);
      if (cell.kind === "desktop") {
        await step("desktop controls retain drawer add", async () => {
          await expect(entry()).not.toBeVisible();
          await expect(page.getByTestId("product-options-dock-entry")).not.toBeVisible();
          const sku = await inline().getAttribute("data-sku"); assert(sku);
          const response = await putClick(() => page.getByTestId("add-to-cart").click());
          assert(response.items.length === 1 && response.items[0].sku_id === sku && response.items[0].quantity === 1, "real server cart must match the selected SKU once");
          const drawer = page.getByTestId("cart-drawer");
          await expect(drawer).toBeVisible();
          await expect(page).toHaveURL(`${origin}/${cell.locale}/products/${candle.slug}`);
          await checkCart(sku, 1);
          await shot("drawer");
          await drawer.getByRole("button", { name: copy.closeCart, exact: true }).click();
          await expect(drawer).not.toBeVisible();
          assert.equal(calls.length, 1);
        });
      } else if (cell.kind === "mobile") {
        await step("task Back discards draft and restores trigger", async () => {
          const initialPrice = await inline().getByTestId("variant-price").innerText();
          const activeImg = await page.getByTestId("product-gallery-active-image").getAttribute("src");
          assert(activeImg && activeImg.startsWith(`/media/p/${candle.id}/`));
          await inline().locator("label.sf-chip", { has: page.getByRole("radio", { name: "300g", exact: true }) }).click();
          await inline().getByTestId("qty-input").locator("xpath=following-sibling::button").click();
          const committedPrice = await inline().getByTestId("variant-price").innerText();
          assert(initialPrice.includes(String(candle.variants[0].price_minor / 100)) && committedPrice.includes(String(candle.variants[1].price_minor / 100)) && initialPrice !== committedPrice);
          const sku = await inline().getAttribute("data-sku"); assert(sku);
          await entry().click();
          await expect(task()).toBeVisible();
          await expect(task().getByRole("heading", { name: copy.c7ChooseOptions, exact: true })).toBeVisible();
          const summary = task().getByTestId("product-options-summary");
          await expect(summary.getByRole("heading", { level: 3, name: candle.title, exact: true })).toBeVisible();
          await expect(summary.locator("img")).toHaveAttribute("src", activeImg);
          await expect(summary.getByTestId("variant-price")).toHaveText(committedPrice);
          await expect(task().getByTestId("add-to-cart")).toHaveText(`${copy.addToCart} · ${committedPrice}`);
          // Read-only layout measurement; screenshot bytes remain unmodified.
          assert(await task().evaluate(node => node.scrollWidth <= node.clientWidth), "task has horizontal overflow");
          await checkTaskGeometry();
          await task().locator("label.sf-chip", { has: page.getByRole("radio", { name: "180g", exact: true }) }).click();
          await expect(task().getByRole("radio", { name: "180g", exact: true })).toBeChecked();
          await expect(summary.locator("img")).toHaveAttribute("src", activeImg);
          await expect(summary.getByTestId("variant-price")).toHaveText(initialPrice);
          await expect(task().getByTestId("add-to-cart")).toHaveText(`${copy.addToCart} · ${initialPrice}`);
          await task().getByTestId("qty-input").locator("xpath=following-sibling::button").click();
          await expect(task().getByTestId("qty-input")).toHaveValue("3");
          await shot("draft");
          await task().getByRole("button", { name: copy.c7BackToProduct, exact: true }).click();
          await expect(task()).not.toBeVisible();
          await expect(entry()).toBeFocused();
          await expect(inline()).toHaveAttribute("data-sku", sku);
          await expect(inline().getByRole("radio", { name: "300g", exact: true })).toBeChecked();
          await expect(inline().getByTestId("qty-input")).toHaveValue("2");
          assert.equal(calls.length, 0);
        });
        await step("dock entry, limits, Escape and reopen", async () => {
          // Real scrolling reveals the existing conditional phone dock; no DOM writes or forced actions.
          await page.locator("footer.sf-footer").scrollIntoViewIfNeeded();
          const dock = page.getByTestId("product-options-dock-entry");
          await expect(dock).toBeVisible(); await dock.click();
          await expect(task().getByRole("radio", { name: "300g", exact: true })).toBeChecked();
          await expect(task().getByTestId("qty-input")).toHaveValue("2");
          await task().locator("label.sf-chip", { has: page.getByRole("radio", { name: "180g", exact: true }) }).click();
          await expect(task().getByRole("radio", { name: "180g", exact: true })).toBeChecked();
          await task().getByTestId("qty-input").fill("0");
          await task().getByRole("heading", { level: 2, name: copy.c7ChooseOptions, exact: true }).click();
          await expect(task().getByTestId("qty-input")).toHaveValue("1");
          await expect(task().getByRole("button", { name: copy.decrease, exact: true })).toBeDisabled();
          await task().getByTestId("qty-input").fill("99");
          await task().getByRole("heading", { level: 2, name: copy.c7ChooseOptions, exact: true }).click();
          await expect(task().getByTestId("qty-input")).toHaveValue("99");
          await expect(task().getByRole("button", { name: copy.increase, exact: true })).toBeDisabled();
          await page.keyboard.press("Escape"); await expect(task()).not.toBeVisible();
          await expect(dock).toBeFocused();
          await expect(inline().getByRole("radio", { name: "300g", exact: true })).toBeChecked();
          await expect(inline().getByTestId("qty-input")).toHaveValue("2");
          await dock.click();
          await expect(task().getByRole("radio", { name: "300g", exact: true })).toBeChecked();
          await expect(task().getByTestId("qty-input")).toHaveValue("2");
          await page.keyboard.press("Escape");
          assert.equal(calls.length, 0);
        });
        await step("real soldout options withhold add", async () => {
          await gotoProduct(page, scarf.slug);
          const scarfPrice = await inline().getByTestId("variant-price").innerText();
          const scarfImg = await page.getByTestId("product-gallery-active-image").getAttribute("src");
          assert(scarfImg && scarfImg.startsWith(`/media/p/${scarf.id}/`) && scarfPrice.includes(String(scarf.variants[0].price_minor / 100)));
          await entry().click();
          await expect(task().getByTestId("product-options-summary").locator("img")).toHaveAttribute("src", scarfImg);
          await expect(task().getByTestId("product-options-summary").getByTestId("variant-price")).toHaveText(scarfPrice);
          await expect(task().getByRole("radio", { name: "Black", exact: true })).toBeDisabled();
          await page.keyboard.press("Escape");
          await gotoProduct(page, diffuser.slug);
          const diffuserPrice = await inline().getByTestId("variant-price").innerText();
          const diffuserImg = await page.getByTestId("product-gallery-active-image").getAttribute("src");
          assert(diffuserImg && diffuserImg.startsWith(`/media/p/${diffuser.id}/`) && diffuserPrice.includes(String(diffuser.variants[0].price_minor / 100)));
          await entry().click();
          await expect(task().getByTestId("product-options-summary").locator("img")).toHaveAttribute("src", diffuserImg);
          await expect(task().getByTestId("product-options-summary").getByTestId("variant-price")).toHaveText(diffuserPrice);
          await expect(task().getByTestId("add-to-cart")).toBeDisabled();
          await checkTaskGeometry();
          await shot("soldout"); await page.keyboard.press("Escape");
          assert.equal(calls.length, 0);
        });
        let selectedSKU = "";
        await step("one confirmed add, server result and cart reload", async () => {
          await gotoProduct(page); await entry().click();
          await task().getByTestId("qty-input").fill("2");
          selectedSKU = await task().getByTestId("product-buy").getAttribute("data-sku") ?? "";
          assert(selectedSKU);
          const response = await putClick(() => task().getByTestId("add-to-cart").click());
          assert(response.items.length === 1 && response.items[0].sku_id === selectedSKU && response.items[0].quantity === 2, "real server cart must contain the selected SKU with quantity two");
          await expect(page).toHaveURL(`${origin}/${cell.locale}/cart`);
          await checkCart(selectedSKU, 2); await page.reload(); await checkCart(selectedSKU, 2);
          assert.equal(calls.length, 1); assert(calls[0].key);
          await shot("confirmed-cart");
        });
        await step("lost real commit recovers same key exactly once", async () => {
          await gotoProduct(page); await entry().click();
          let dropped = false;
          await page.route("**/api/buyer/cart", async route => {
            if (route.request().method() !== "PUT") return route.continue();
            // Existing checkout-v2 lost-result seam: commit in real BFF/Go/PG, then lose only its response.
            const response = await route.fetch({ maxRetries: 0 });
            assert.equal(response.status(), 200);
            if (!dropped) { dropped = true; return route.abort("failed"); }
            return route.fulfill({ response });
          });
          await task().getByTestId("add-to-cart").click();
          await expect(task().getByTestId("cart-problem")).toContainText(copy.cartUncertain);
          await expect(task().getByTestId("add-to-cart")).toBeDisabled();
          await checkTaskGeometry();
          assert.equal(calls.length, 2);
          assert(calls[1].key && calls[1].key !== calls[0].key, "new explicit add gets a distinct request identity");
          await shot("unknown");
          if (cell.width === 390) {
            // After confirmation UNKNOWN, Back cancels only the editor; its original operation may still settle.
            await task().getByRole("button", { name: copy.c7BackToProduct, exact: true }).click();
            await expect(task()).not.toBeVisible(); await expect(entry()).toBeFocused();
            assert.equal(calls.length, 2, "Back never sends a new command");
          }
          const notice = cell.width === 390 ? page.getByTestId("cart-problem") : task().getByTestId("cart-problem");
          const response = await putClick(() => notice.getByRole("button", { name: copy.retry, exact: true }).click());
          assert(calls[2].key === calls[1].key && calls[2].body === calls[1].body && calls[2].method === calls[1].method, "recovery must retain original method/key/body bytes");
          assert(response.items.length === 1 && response.items[0].sku_id === selectedSKU && response.items[0].quantity === 3, "recovered cart must contain one increment, never a duplicate add");
          await expect(page).toHaveURL(`${origin}/${cell.locale}/cart`);
          await checkCart(selectedSKU, 3); await page.reload(); await checkCart(selectedSKU, 3);
          assert.equal(calls.length, 3);
          await shot("recovered-cart");
        });
      } else {
        await step("real stale cart snapshot produces server CAS refusal", async () => {
          const sku = await inline().getAttribute("data-sku"); assert(sku);
          await putClick(() => page.getByTestId("add-to-cart").click());
          await page.getByTestId("cart-drawer").getByRole("button", { name: copy.closeCart, exact: true }).click();
          await entry().click();
          let held = false, intercepted = false, oldVersion = -1, release = () => {};
          const gate = new Promise<void>(resolve => { release = resolve; });
          try {
            await page.route("**/api/buyer/cart", async route => {
              if (route.request().method() !== "GET" || intercepted) return route.continue();
              intercepted = true;
              const response = await route.fetch({ maxRetries: 0 });
              assert.equal(response.status(), 200);
              oldVersion = ((await response.json()) as C7Cart).version; held = true;
              await gate; return route.fulfill({ response });
            });
            await task().getByTestId("add-to-cart").click();
            await expect.poll(() => held).toBe(true);
            const other = await context!.newPage(); observe(other);
            await gotoProduct(other);
            const [changed] = await Promise.all([other.waitForResponse(c7CartResponse), other.getByTestId("add-to-cart").click()]);
            assert.equal(changed.status(), 200);
            assert.equal(((await changed.json()) as C7Cart).version, oldVersion + 1);
            const stale = page.waitForResponse(c7CartResponse); release();
            const refused = await stale; assert.equal(refused.status(), 409, "must be an actual server refusal, never a synthetic 409");
            assert.equal((await refused.json()).code, "conflict");
            await expect(task().getByTestId("cart-problem")).toContainText(copy.cartConflict);
            assert.equal(JSON.parse(calls[2].body).expected_version, oldVersion);
            assert(new Set(calls.map(call => call.key)).size === 3, "three distinct explicit intents must not reuse request identities");
            await task().getByRole("button", { name: copy.c7BackToProduct, exact: true }).click();
            await page.goto(`${origin}/${cell.locale}/cart`); await checkCart(sku, 2);
            await page.reload(); await checkCart(sku, 2);
            assert.equal(calls.length, 3); await shot("cas-refused");
          } finally { release(); }
        });
      }
      cell.status = "PASS";
    } catch (error) {
      cell.status = "FAIL"; cell.failure = error instanceof Error ? error.name : "UnknownError";
      throw error;
    } finally {
      try { await save(); } finally {
        // CAS's inner finally releases its real response before waiting for route handlers and closing both pages.
        try { await routedPage?.unrouteAll({ behavior: "wait" }); } finally { await context?.close(); }
      }
    }
  }
}
