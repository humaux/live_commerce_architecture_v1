// COB merchant half (contracts/storefront-v2.md §C; unit checkout-offline; R4 independent browser gate). BFF routes exercised through the UI:
//   GET|PUT /api/stores/{store}/bank-transfer-settings, GET .../orders/{id}/bank-transfer, POST .../orders/{id}/bank-transfer/{confirm,reject,refund-offline},
//   GET|PUT policy + delivery service (free-shipping threshold), GET finance/summary -> Go /v1/admin/stores/{store}/...
// Started in phases by tests/foundation/browser_checkout_offline_test.go (LC_OFF_PHASE) between the buyer script's handshakes; MOCK: no PSP exists on
// this path. Locators are data-testid (structure) plus the copy catalogs (settings-copy, transfer-copy) for wording.
import { expect, test, type Browser, type BrowserContext, type Locator, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { settingsCopy } from "../../apps/admin/lib/settings-copy";
import { transferCopy } from "../../apps/admin/lib/transfer-copy";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_STORE");
const phase = required("LC_OFF_PHASE");
const bank = { name: required("LC_OFF_BANK"), branch: "Zhongshan", holder: "Sentinel Shop Ltd", account: required("LC_OFF_ACCOUNT"), window: "6" };
const threshold = required("LC_OFF_THRESHOLD"), fee = required("LC_OFF_FEE");
const market = required("LC_OFF_MARKET");
const service = required("LC_OFF_SERVICE");
const reason = required("LC_OFF_REASON");
const orders = JSON.parse(process.env.LC_OFF_ORDERS ?? "{}") as Record<string, { id: string; locale: "zh-TW" | "en"; mobile: boolean }>;

test.use({ baseURL: origin, trace: "retain-on-failure", screenshot: "only-on-failure" });
test.describe.configure({ mode: "serial", timeout: 240_000 });

const manifestPath = path.join(evidence, "screenshots.json");
async function shot(page: Page, name: string, locale: string, viewport: "desktop" | "mobile") {
  const file = path.join(evidence, `offline-admin-${name}-${locale}-${viewport}.png`);
  await page.screenshot({ path: file, fullPage: false, animations: "disabled" });
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
  await expect(page.getByTestId("orders-table")).toBeVisible();
}
async function expand(page: Page, id: string): Promise<Locator> {
  for (let pageNo = 0; pageNo < 5; pageNo++) {
    const button = page.getByTestId(`order-expand-${id}`);
    if (await button.count()) {
      if ((await button.getAttribute("aria-expanded")) !== "true") await button.click();
      const detail = page.getByTestId("order-detail");
      await expect(detail).toHaveAttribute("aria-label", new RegExp(id));
      return detail;
    }
    await expect(page.getByTestId("orders-next")).toBeEnabled();
    await page.getByTestId("orders-next").click();
    await expect(page.getByTestId("orders-table")).toBeVisible();
  }
  throw new Error("order was absent from the real cursor pages");
}
const viewport = (mobile: boolean) => (mobile ? "mobile" : "desktop") as "desktop" | "mobile";

test("settings: the merchant sets bank details, the window and the free-shipping threshold in the settings wizard (zh-TW desktop), values read back", async ({ browser }) => {
  test.skip(phase !== "settings");
  const locale = "zh-TW", c = settingsCopy[locale], tc = transferCopy[locale];
  const { context, page } = await merchant(browser, false);
  await page.goto(new URL(`/${locale}/settings?store=${store}`, origin).toString());
  await expect(page.getByTestId("settings-wizard")).toBeVisible();
  await page.getByLabel(c.manual, { exact: true }).check();
  await page.getByRole("button", { name: c.next, exact: true }).click();
  // step 2: the bank-transfer card sits in the merchant-arranged branch
  const card = page.getByTestId("transfer-settings-card");
  await expect(card).toBeVisible();
  await expect(card.getByTestId("transfer-bank-name")).toHaveValue(""); // nothing configured yet
  await card.getByTestId("transfer-enabled").check();
  await card.getByTestId("transfer-bank-name").fill(bank.name);
  await card.getByTestId("transfer-branch").fill(bank.branch);
  await card.getByTestId("transfer-account-name").fill(bank.holder);
  await card.getByTestId("transfer-account-number").fill(bank.account);
  // an invalid window (5 h, the minimum is 6) is refused with the field message and nothing is saved
  await card.getByTestId("transfer-window").fill("5");
  await card.getByTestId("transfer-settings-save").click();
  await expect(card.getByTestId("transfer-settings-problem")).toHaveText(tc.invalid);
  await card.getByTestId("transfer-window").fill(bank.window);
  await shot(page, "settings-bank", locale, "desktop");
  await card.getByTestId("transfer-settings-save").click();
  await expect(card.getByTestId("transfer-settings-notice")).toHaveText(tc.saved);
  await expect(card.getByTestId("transfer-settings-problem")).toHaveCount(0);
  // step 3: the delivery policy's free-shipping threshold, then the service re-save that pins the new policy version
  await page.getByRole("button", { name: c.next, exact: true }).click();
  await page.getByRole("combobox", { name: c.market, exact: true }).selectOption(market);
  // the saved policy must hydrate (its version is the CAS token of the save) before any field is edited
  const policyRead = page.waitForResponse((r) => r.url().includes("/policy") && r.request().method() === "GET");
  await page.getByLabel(c.serviceCode, { exact: true }).fill(service);
  expect((await policyRead).status()).toBe(200);
  const policyForm = page.getByTestId("settings-policy-form");
  await expect(policyForm.getByLabel(c.shipping, { exact: true })).toHaveValue("0"); // hydrated from the saved policy
  await policyForm.getByLabel(c.shipping, { exact: true }).fill(fee);
  await expect(policyForm.getByTestId("settings-free-shipping")).toHaveValue("");
  await policyForm.getByTestId("settings-free-shipping").fill(threshold);
  await policyForm.getByLabel(new RegExp(`^${c.ref}`)).fill("R4 gate: free shipping at the subtotal");
  const enable = policyForm.getByLabel(c.policyEnabled, { exact: true });
  if (!(await enable.isChecked())) await enable.check();
  await policyForm.getByRole("button", { name: c.savePolicy, exact: true }).click();
  await expect(page.getByText(c.policySaved)).toBeVisible();
  await shot(page, "settings-policy", locale, "desktop");
  await page.getByTestId("settings-service-form").getByRole("button", { name: c.saveService, exact: true }).click();
  await expect(page.getByText(c.serviceSaved)).toBeVisible();
  // read back from the server after a reload: the card carries the saved values
  await page.reload();
  await page.getByLabel(c.manual, { exact: true }).check();
  await page.getByRole("button", { name: c.next, exact: true }).click();
  const again = page.getByTestId("transfer-settings-card");
  await expect(again.getByTestId("transfer-bank-name")).toHaveValue(bank.name);
  await expect(again.getByTestId("transfer-account-number")).toHaveValue(bank.account);
  await expect(again.getByTestId("transfer-window")).toHaveValue(bank.window);
  await expect(again.getByTestId("transfer-enabled")).toBeChecked();
  await context.close();
});

test("review: the merchant sees each buyer's submission; rejects A with a reason; confirms B, C, D (zh-TW/en, desktop/390px)", async ({ browser }) => {
  test.skip(phase !== "review");
  const desktop = await merchant(browser, false), phoneUI = await merchant(browser, true);
  for (const key of ["A", "B", "C", "D"]) {
    const o = orders[key], page = (o.mobile ? phoneUI : desktop).page;
    await openOrders(page, o.locale);
    const detail = await expand(page, o.id);
    const section = detail.getByTestId("order-transfer");
    await expect(section).toBeVisible();
    await expect(section.getByTestId("transfer-state")).toHaveAttribute("data-state", "SUBMITTED");
    await expect(section.getByTestId("transfer-last5")).toHaveText("12345");
    await expect(section.getByTestId("transfer-bank")).toContainText(bank.account); // the account the buyer was given
    await expect(section.getByTestId("transfer-mismatch")).toHaveCount(0); // the buyer reported the exact amount due
    await shot(page, `review-${key}`, o.locale, viewport(o.mobile));
    if (key === "A") {
      await section.getByTestId("transfer-reject").click();
      const dialog = page.getByTestId("transfer-dialog");
      await expect(dialog).toBeVisible();
      await expect(dialog.getByTestId("transfer-submit")).toBeDisabled(); // a reason is required
      await dialog.getByTestId("transfer-reason").fill(reason);
      await dialog.getByTestId("transfer-submit").click();
      await expect(section.getByTestId("transfer-state")).toHaveAttribute("data-state", "REJECTED");
      await expect(section.getByTestId("transfer-reject-reason")).toContainText(reason);
      await expect(section.getByTestId("transfer-confirm")).toHaveCount(0); // nothing left to decide until the buyer resubmits
    } else {
      await section.getByTestId("transfer-confirm").click();
      const dialog = page.getByTestId("transfer-dialog");
      await expect(dialog).toBeVisible();
      await expect(dialog.getByTestId("transfer-dialog-text")).toContainText(/25/); // the dialog names the amount the merchant must have seen arrive
      await dialog.getByTestId("transfer-submit").click();
      await expect(section.getByTestId("transfer-state")).toHaveAttribute("data-state", "CONFIRMED");
      await expect(section.getByTestId("transfer-confirm")).toHaveCount(0);
      await expect(section.getByTestId("transfer-refund")).toBeVisible(); // the only remaining act is the offline refund record
    }
  }
  // order E has no proof yet: the merchant may see it, and it stays open
  const e = orders.E, page = desktop.page;
  await openOrders(page, e.locale);
  const eSection = (await expand(page, e.id)).getByTestId("order-transfer");
  await expect(eSection.getByTestId("transfer-state")).toHaveAttribute("data-state", "AWAITING");
  await expect(eSection.getByTestId("transfer-proof")).toHaveCount(0);
  await desktop.context.close();
  await phoneUI.context.close();
});

test("confirm-a: after the buyer's resubmission the merchant confirms A (zh-TW desktop)", async ({ browser }) => {
  test.skip(phase !== "confirm-a");
  const { context, page } = await merchant(browser, false);
  const a = orders.A;
  await openOrders(page, a.locale);
  const section = (await expand(page, a.id)).getByTestId("order-transfer");
  await expect(section.getByTestId("transfer-state")).toHaveAttribute("data-state", "SUBMITTED");
  await expect(section.getByTestId("transfer-last5")).toHaveText("54321"); // the resubmitted details replaced the first ones
  await section.getByTestId("transfer-confirm").click();
  await page.getByTestId("transfer-dialog").getByTestId("transfer-submit").click();
  await expect(section.getByTestId("transfer-state")).toHaveAttribute("data-state", "CONFIRMED");
  await context.close();
});

test("final: finance shows the four confirmations in its own column, the expired order is closed, settings read back at 390px in English", async ({ browser }) => {
  test.skip(phase !== "final");
  const { context, page } = await merchant(browser, true);
  const e = orders.E;
  await openOrders(page, "en");
  const section = (await expand(page, e.id)).getByTestId("order-transfer");
  await expect(section.getByTestId("transfer-state")).toHaveAttribute("data-state", "EXPIRED");
  await expect(section.getByTestId("transfer-confirm")).toHaveCount(0);
  await expect(section.getByTestId("transfer-reject")).toHaveCount(0);
  await shot(page, "expired", "en", "mobile");
  await page.goto(new URL(`/en/finance?store=${store}`, origin).toString());
  await expect(page.getByTestId("finance-page")).toBeVisible();
  const cells = page.getByTestId("finance-transfer-confirmed");
  await expect(cells.filter({ hasText: "(4)" }).first()).toBeVisible(); // 4 confirmed transfers, never inside the captured/net columns
  await shot(page, "finance", "en", "mobile");
  const c = settingsCopy.en;
  await page.goto(new URL(`/en/settings?store=${store}`, origin).toString());
  await page.getByLabel(c.manual, { exact: true }).check();
  await page.getByRole("button", { name: c.next, exact: true }).click();
  const card = page.getByTestId("transfer-settings-card");
  await expect(card.getByTestId("transfer-bank-name")).toHaveValue(bank.name);
  await expect(card.getByTestId("transfer-window")).toHaveValue(bank.window);
  await shot(page, "settings-readback", "en", "mobile");
  await context.close();
});
