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
