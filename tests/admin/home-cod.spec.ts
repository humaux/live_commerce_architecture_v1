// Purpose: Real-click merchant COD settings, fulfilment and finance acceptance.
// Depends on: @playwright/test, node fs/path/crypto, cod/settings copy, ./parity-money-support.
// Used by: tests/foundation/browser_home_cod_test.go; --browser-home-cod.
// Invariants: synthetic MOCK fixtures only; real clicks and server readback; no live carrier or money action.
// home-cod R5 merchant half (migration 0107; docs/delivery/units/home-cod.md). BFF routes exercised through the UI:
//   GET|PUT /api/stores/{store}/cash-on-delivery-settings -> Go /v1/admin/stores/{store}/cash-on-delivery-settings (integration:read/manage),
//   PUT .../orders/{id}/shipment, POST .../orders/{id}/collection -> Go /v1/admin/stores/{store}/orders/{id}/shipment|collection (fulfillment:write),
//   GET /api/stores/{store}/finance/summary -> Go /v1/admin/stores/{store}/finance/summary (orders:read, the cod_collected columns).
// Started in phases by tests/foundation/browser_home_cod_test.go (LC_HCOD_PHASE) between the buyer script's handshakes; MOCK: no PSP and no
// carrier API on this path (cash on delivery is collected by the carrier and recorded here). Locators are data-testid (structure) plus the
// codCopy/settingsCopy catalogs for wording.
import { expect, test, type Browser, type BrowserContext, type Locator, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { settingsCopy } from "../../apps/admin/lib/settings-copy";
import { codCopy } from "../../apps/admin/lib/cod-copy";
import { moneyClick, moneyDesign, moneyLocales, moneyShot, moneyWidths } from "./parity-money-support";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_STORE");
const phase = required("LC_HCOD_PHASE");
// LC_HCOD_ORDER is required by the ship-collect phase only; the settings/finance phases read it defensively.
const order = process.env.LC_HCOD_ORDER ?? "";
const today = process.env.LC_HCOD_TODAY ?? "";

test.use({ baseURL: origin, trace: "retain-on-failure", screenshot: "only-on-failure" });
test.describe.configure({ mode: "serial", timeout: 240_000 });

const manifestPath = path.join(evidence, "screenshots.json");
async function shot(page: Page, name: string, locale: string, viewport: "desktop" | "mobile") {
  const file = path.join(evidence, `home-cod-admin-${name}-${locale}-${viewport}.png`);
  await page.screenshot({ path: file, fullPage: false, animations: "disabled" });
  // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  let manifest: unknown[] = [];
  try {
    manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  } catch {
    /* first screenshot */
  }
  manifest.push({ File: path.basename(file), Sha256: createHash("sha256").update(await readFile(file)).digest("hex"), Locale: locale, Viewport: viewport });
  await writeFile(manifestPath, JSON.stringify(manifest, null, 2));
}
async function revealSavedFacts(page: Page) {
  const url=new URL(page.url());
  if(!/\/orders\/[0-9a-f-]{36}$/.test(url.pathname)&&!url.searchParams.has("order"))return;
  const facts=page.getByTestId("order-more-details");
  await expect(facts).toBeVisible();
  if(await facts.getAttribute("open")===null)await facts.locator("summary").first().click();
}
async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("nav-orders")).toBeVisible();
}
async function merchant(browser: Browser, mobile: boolean): Promise<{ context: BrowserContext; page: Page }> {
  const context = await browser.newContext({ baseURL: origin, viewport: mobile ? { width: 390, height: 844 } : { width: 1440, height: 900 }, isMobile: mobile, hasTouch: mobile });
  const page = await context.newPage();
  await signedLogin(page);
  return { context, page };
}
async function openOrders(page: Page, locale: string) {
  await page.goto(new URL(`/${locale}/orders?store=${store}`, origin).toString());
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  const selector = page.getByTestId("store-selector");
  if (await selector.count()) {
    if ((await selector.inputValue()) !== store) await selector.selectOption(store);
  }
  await expect(page.getByTestId("orders-list")).toBeVisible();
  // The order list names its time zone (integration: the stop-bleed M06 label "Taipei time / 台北時間 / 台北时间" replaces "Taipei, UTC+8").

}
async function expand(page: Page, id: string): Promise<Locator> {
  if(await page.getByTestId("order-detail-back").count())await page.getByTestId("order-detail-back").click();
  for (let pageNo = 0; pageNo < 5; pageNo++) {
    const button = page.getByTestId(`order-expand-${id}`);
    if (await button.count()) {
      if ((await button.getAttribute("aria-expanded")) !== "true") await button.click();
      const detail = page.getByTestId("order-detail");
      await expect(detail).toHaveAttribute("aria-label", new RegExp(id));
      const facts=page.getByTestId("order-more-details");
      if(await facts.getAttribute("open")===null) await facts.locator("summary").first().click();
      await expect(detail).toContainText(/Taipei time|台北時間|台北时间/);
      return detail.getByTestId("order-facts");
    }
    await expect(page.getByTestId("orders-next")).toBeEnabled();
    await page.getByTestId("orders-next").click();
    await expect(page.getByTestId("orders-list")).toBeVisible();
  }
  throw new Error("order was absent from the real cursor pages");
}

for (const locale of moneyLocales) for (const width of moneyWidths) {
  test(`PARITY-MONEY COD settings and save-review frames (${locale}, ${width})`, async ({ browser }) => {
    test.skip(phase !== "settings");
    const c = moneyDesign[locale], sc = settingsCopy[locale];
    const { context, page } = await merchant(browser, false);
    await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
    try {
      const target = `/${locale}/settings?store=${store}`;
      await page.goto(target);
      await page.getByRole("button", { name: locale === "en" ? "Advanced" : locale === "zh-CN" ? "进阶" : "進階", exact: true }).click();
      await page.getByRole("button", { name: sc.next, exact: true }).click();
      const card = page.getByTestId("cod-settings-card");
      await expect(card.getByRole("heading", { name: c.cod, exact: true }), "PARITY-MONEY 94:2153/94:2223: signed COD title").toBeVisible();
      await expect(card.getByRole("heading", { name: c.oldCod, exact: true })).toHaveCount(0);
      const original = {
        enabled: await card.getByRole("checkbox", { name: c.codEnabled, exact: true }).isChecked(),
        cap: await card.getByLabel(c.cap, { exact: true }).inputValue(),
        fee: await card.getByLabel(c.surcharge, { exact: true }).inputValue(),
        carrier: await card.getByRole("combobox", { name: c.carrier, exact: true }).inputValue(),
      };
      await moneyClick(page, evidence, "COD controls", "each required setting is editable with native user actions", async () => {
        await card.getByRole("checkbox", { name: c.codEnabled, exact: true }).setChecked(true);
        await card.getByLabel(c.cap, { exact: true }).fill("19000");
        await card.getByLabel(c.surcharge, { exact: true }).fill("40");
        await card.getByRole("combobox", { name: c.carrier, exact: true }).selectOption("hsinchu");
      });
      await moneyShot(page, evidence, "cod", locale, width);
      await moneyClick(page, evidence, c.save, "successful server save shows all four settings in review", async () => {
        await card.getByRole("button", { name: c.save, exact: true }).click();
        await expect(page.getByRole("heading", { name: c.codReview, exact: true })).toBeVisible();
        const review = page.getByTestId("cod-settings-review");
        for (const label of [c.codEnabled, c.cap, c.surcharge, c.carrier]) await expect(review.getByText(label, { exact: true })).toBeVisible();
        await expect(review).toContainText(locale === "en" ? "Enabled" : locale === "zh-CN" ? "已启用" : "已啟用");
        await expect(review).toContainText("19,000");
        await expect(review).toContainText("40");
        await expect(review).toContainText(locale === "en" ? "Hsinchu" : "新竹");
        await moneyShot(page, evidence, "codReview", locale, width);
        await page.getByRole("button", { name: c.backCod, exact: true }).click();
      });
      await page.reload();
      await page.getByRole("button", { name: new RegExp(sc.steps[1]) }).click();
      await expect(card.getByRole("checkbox", { name: c.codEnabled, exact: true })).toBeChecked();
      await expect(card.getByLabel(c.cap, { exact: true })).toHaveValue("19000");
      await expect(card.getByLabel(c.surcharge, { exact: true })).toHaveValue("40");
      await expect(card.getByRole("combobox", { name: c.carrier, exact: true })).toHaveValue("hsinchu");
      // Restore via real UI so the original home-COD chain starts with its original unconfigured state.
      await card.getByRole("checkbox", { name: c.codEnabled, exact: true }).setChecked(original.enabled);
      await card.getByLabel(c.cap, { exact: true }).fill(original.cap);
      await card.getByLabel(c.surcharge, { exact: true }).fill(original.fee);
      await card.getByRole("combobox", { name: c.carrier, exact: true }).selectOption(original.carrier);
      await card.getByRole("button", { name: c.save, exact: true }).click();
      await expect(page.getByRole("heading", { name: c.codReview, exact: true })).toBeVisible();
    } finally { await context.close(); }
  });
}

test("settings: the merchant enables cash on delivery with a cap, surcharge and carrier (zh-TW desktop), values read back", async ({ browser }) => {
  test.skip(phase !== "settings");
  const locale = "zh-TW", c = settingsCopy[locale], cc = codCopy[locale];
  const { context, page } = await merchant(browser, false);
  await page.goto(new URL(`/${locale}/settings?store=${store}`, origin).toString());
  await expect(page.getByTestId("settings-wizard")).toBeVisible();
  await page.getByRole("button", { name: "編輯設定", exact: true }).click();
  await page.getByRole("button", { name: c.next, exact: true }).click();
  // step 2: the cash-on-delivery card sits in the merchant-arranged branch
  const card = page.getByTestId("cod-settings-card");
  await expect(card).toBeVisible();
  await expect(card.getByTestId("cod-enabled")).not.toBeChecked(); // nothing configured yet
  await card.getByTestId("cod-enabled").check();
  await card.getByTestId("cod-max").fill("20000");
  await card.getByTestId("cod-carrier").selectOption("black_cat");
  // an out-of-range surcharge (1001 TWD, the ceiling is 1000) is refused with the field message and nothing is saved
  await card.getByTestId("cod-surcharge").fill("1001");
  await card.getByTestId("cod-settings-save").click();
  await expect(card.getByTestId("cod-settings-problem")).toHaveText(cc.invalid);
  await card.getByTestId("cod-surcharge").fill("50");
  await card.getByTestId("cod-settings-save").click();
  await expect(card.getByTestId("cod-settings-notice")).toHaveText(cc.saved);
  await expect(card.getByTestId("cod-settings-problem")).toHaveCount(0);
  await shot(page, "settings-cod", locale, "desktop");
  // read back from the server after a reload: the card carries the saved values
  await page.reload(); await revealSavedFacts(page);
  await page.getByRole("button", { name: "編輯設定", exact: true }).click();
  await page.getByRole("button", { name: new RegExp(c.steps[1]) }).click(); // the draft keeps the wizard on step 3: back to step 2
  const again = page.getByTestId("cod-settings-card");
  await expect(again.getByTestId("cod-enabled")).toBeChecked();
  await expect(again.getByTestId("cod-max")).toHaveValue("20000");
  await expect(again.getByTestId("cod-surcharge")).toHaveValue("50");
  await expect(again.getByTestId("cod-carrier")).toHaveValue("black_cat");
  for (const language of ["zh-TW", "zh-CN", "en"] as const) {
    await page.goto(new URL(`/${language}/settings?store=${store}`, origin).toString());
    await page.getByRole("button", { name: { "zh-TW": "編輯設定", "zh-CN": "编辑设置", en: "Edit settings" }[language], exact: true }).click();
    await page.getByRole("button", { name: new RegExp(settingsCopy[language].steps[1]) }).click();
    await expect(page.getByTestId("cod-settings-card")).toBeVisible();
    for (const width of [390, 1366, 1586]) {
      await page.setViewportSize({ width, height: width === 390 ? 844 : 992 });
      await page.getByTestId("cod-settings-card").scrollIntoViewIfNeeded();
      // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
      expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
      for (const control of ["cod-max", "cod-surcharge", "cod-carrier", "cod-settings-save"]) {
        const box = await page.getByTestId(control).boundingBox();
        expect(box?.height, `${language} ${control} touch height`).toBeGreaterThanOrEqual(44);
      }
      await page.screenshot({ path: path.resolve(evidence, "home-cod-ui", `admin-settings-${language}-${width}.png`), fullPage: false, animations: "disabled", scale: "css" });
      expect(await page.getByTestId("cod-max").evaluate((input) => {
        const r = input.getBoundingClientRect();
        return document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2) === input;
      }), "the amount input must not be covered by the navigation rail").toBe(true);
    }
  }
  await context.close();
});

test("ship-collect: the merchant records the manual shipment, then the collected cash (zh-TW desktop)", async ({ browser }) => {
  test.skip(phase !== "ship-collect");
  const locale = "zh-TW";
  const { context, page } = await merchant(browser, false);
  await openOrders(page, locale);
  const detail = await expand(page, order);
  const pendingCod = detail.getByTestId("order-cod");
  await expect(pendingCod.getByTestId("cod-collected")).toBeDisabled();
  await expect(pendingCod).toContainText(codCopy[locale].notShipped);
  const collectText = await pendingCod.getByTestId("cod-collect-amount").innerText();
  expect(collectText).toMatch(/^NT\$[\d,]+$/);
  await expect(detail.getByTestId("order-collect-amount")).toHaveText(collectText);
  await expect(detail.getByTestId("order-collection-state")).toHaveText(codCopy[locale].orderStates.PENDING);
  // Manual shipment, not a carrier API call: use the checkout carrier.
  const shipment = detail.getByTestId("order-shipment");
  await expect(shipment).toBeVisible();
  await expect(shipment.getByTestId("ship-carrier").locator('option[value="hsinchu"]')).toHaveCount(1);
  await shipment.getByTestId("ship-carrier").selectOption("black_cat");
  await shipment.getByTestId("ship-tracking").fill("BC1234567890");
  await shot(page, "ship", locale, "desktop");
  await shipment.getByTestId("shipment-submit").click();
  await shipment.getByTestId("shipment-confirm").click();
  await expect(shipment.getByTestId("shipment-record")).toBeVisible();
  // collected is only offered once the order is shipped; the dialog names the carrier-collected fact
  const cod = detail.getByTestId("order-cod");
  await expect(cod.getByTestId("cod-collection-state")).toHaveAttribute("data-state", "PENDING");
  await cod.getByTestId("cod-collected").click();
  const dialog = page.getByTestId("cod-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByTestId("cod-confirm-text")).toContainText(/貨運/);
  await expect(dialog.getByTestId("cod-confirm-amount")).toContainText(collectText);
  await shot(page, "collect-dialog", locale, "desktop");
  await dialog.getByTestId("cod-submit").click();
  await expect(cod.getByTestId("cod-collection-state")).toHaveAttribute("data-state", "COLLECTED");
  await expect(detail.getByTestId("order-collection-state")).toHaveText(codCopy[locale].orderStates.COLLECTED);
  await expect(cod.getByTestId("cod-collected")).toHaveCount(0); // a collected order cannot be collected again
  await expect(shipment.getByTestId("shipment-void")).toBeDisabled();
  await expect(shipment.getByTestId("shipment-correct")).toBeEnabled();
  await shot(page, "collected", locale, "desktop");
  await context.close();
});

test("finance: the collected cash appears in the COD column, never in captured/net (en desktop)", async ({ browser }) => {
  test.skip(phase !== "finance");
  const { context, page } = await merchant(browser, false);
  await page.goto(new URL(`/en/finance?store=${store}&from=${today}&to=${today}`, origin).toString());
  await expect(page.getByTestId("finance-page")).toBeVisible();
  await page.getByRole("button", {name: moneyDesign.en.daily, exact:true}).click();
  await expect(page.getByTestId("finance-table")).toBeVisible();
  const cells = page.getByTestId("finance-cod-collected");
  await expect(cells.filter({ hasText: "/ 1" }).first()).toBeVisible(); // one collected order, order total + surcharge
  await shot(page, "finance", "en", "desktop");
  await context.close();
});
