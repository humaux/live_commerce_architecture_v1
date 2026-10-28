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
    await page.getByRole("button", { name: copy.en.quote, exact: true }).click();
    const quote = await (await quoted).json();
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
        });
      } catch (error) { failures.push(`re-quote: ${error.message}`); }
    } finally { await context.close(); }
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
  // The server retains the recipient; confirmation must be explicit again. Re-selecting COD never writes money.
  await cod.click(); await page.getByTestId("confirm-address").click();
  await checkoutV2Review(page, quote, locale, recipient, { codFee: fee, ledger, file });
}
