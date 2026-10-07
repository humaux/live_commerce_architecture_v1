// Purpose: W6-U2 real-click independent acceptance for operations and ads-unbind/feed.
// Depends on: production Next/BFF/Go/isolated PG started by browser_operations_ads_test.go; Playwright.
// Used by: --browser-operations-ads; server responses are never intercepted.
// Invariants: control POSTs only prepare synthetic fixtures; assertions use visible UI and reload; no force/evaluate writes.
import { expect, test, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";

function required(name: string) { const v = process.env[name]; if (!v) throw new Error(`${name} required`); return v; }
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"), adsOrigin = required("LC_BROWSER_ADS_ORIGIN");
const store = required("LC_BROWSER_STORE"), adsStore = required("LC_BROWSER_ADS_STORE"), otherStore = required("LC_BROWSER_OTHER_STORE");
const evidence = required("LC_BROWSER_EVIDENCE");
const ops = JSON.parse(required("LC_BROWSER_OPS")) as Record<string, string>;
const account = required("LC_BROWSER_ADS_ACCOUNT"), adOperation = required("LC_BROWSER_ADS_OPERATION"), draft = required("LC_BROWSER_ADS_DRAFT");
const feedURL = required("LC_BROWSER_FEED_URL");
const rows: { page: string; control: string; action: string; expected: string; actual: string; result: string }[] = [];
test.use({ baseURL: origin });
test.describe.configure({ mode: "serial" });

async function control(resource: string) {
  // Fixture preparation only: hard-coded runner listener changes synthetic PG facts, never production API/UI state.
  const r = await fetch(`${required("LC_BROWSER_CONTROL")}/${resource}`, { method: "POST", headers: { "X-Gate-Key": required("LC_BROWSER_CONTROL_KEY") } });
  expect(r.status, `fixture ${resource}`).toBe(204);
}
async function clicked(page: Page, id: string, expected: string, verify: () => Promise<void>) {
  const row = { page: page.url(), control: id, action: "click", expected, actual: "", result: "FAIL" };
  rows.push(row);
  try { await page.getByTestId(id).click(); await verify(); row.actual = expected; row.result = "PASS"; }
  catch (error) { row.actual = error instanceof Error ? error.message : String(error); throw error; }
}
test.afterEach(async () => { await writeFile(path.join(evidence, "click-ledger.json"), JSON.stringify(rows, null, 2)); });
async function login(page: Page, target = origin) {
  // Distinct signed app fixtures share the loopback hostname; cookies are host-scoped, never port-scoped.
  await page.context().clearCookies();
  await page.goto(`${target}/en/`);
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByRole("combobox", { name: "Switch store", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign in with identity service" })).toHaveCount(0);
}
async function ledger(page: Page, locale = "en", operation?: string) {
  await page.goto(`${origin}/${locale}/settings/operations?store=${store}${operation ? `&operation=${operation}` : ""}`);
  await expect(page.getByTestId("operations-ledger")).toBeVisible();
  if (operation) await detailLoaded(page, operation);
  else await expect(page.getByTestId(`operation-row-${ops.query}`)).toBeVisible();
}
async function detailLoaded(page: Page, id: string) {
  await expect(page.getByTestId("operation-drawer")).toBeVisible();
  await expect(page.getByTestId("operation-drawer")).toContainText(id);
}
async function foreignViewLoaded(page: Page) {
  // Positive row proves the other store's list has finished; the error proves its scoped detail request finished.
  await expect(page.getByTestId(`operation-row-${ops.foreign}`)).toBeVisible();
  await expect(page.getByTestId("operation-drawer")).toBeVisible();
  await expect(page.getByTestId("operation-drawer")).toContainText("Operations could not be loaded. Refresh to try again.");
}
async function open(page: Page, name: string) {
  await clicked(page, `operation-open-${ops[name]}`, "drawer exposes selected operation", async () => {
    await detailLoaded(page, ops[name]);
  });
}
async function action(page: Page, name: "query" | "cancel" | "retry", expected: string, verify: () => Promise<void>) {
  await clicked(page, `operation-${name}`, "confirmation dialog opens", async () => { await expect(page.getByTestId("operation-confirm")).toBeVisible(); });
  await clicked(page, "operation-confirm-submit", expected, verify);
}
async function close(page: Page) {
  await clicked(page, "operation-close", "drawer closes", async () => { await expect(page.getByTestId("operation-drawer")).not.toBeVisible(); });
}
async function filter(page: Page, value: string) {
  const pending = page.waitForResponse(r => r.url().includes(`/operations?`) && r.request().method() === "GET");
  await page.getByTestId("operations-filter").selectOption(value);
  const response = await pending; expect(response.status()).toBe(200);
  await expect(page.getByTestId("operations-filter")).toHaveValue(value);
  rows.push({ page: page.url(), control: "operations-filter", action: `selectOption(${value})`, expected: `persisted ${value} rows`, actual: `HTTP 200 and selected ${value}`, result: "PASS" });
}
async function toolbarAligned(page: Page) {
  // READ/MEASURE only. G-UI9 R2 locks adjacent action/control alignment to 4px; thresholds are untouched.
  const field = await page.getByTestId("operations-filter").boundingBox();
  const action = await page.getByTestId("operations-refresh").boundingBox();
  if (!field || !action) throw new Error("toolbar controls must have visible geometry");
  expect(Math.abs(action.y - field.y), "refresh and filter share the control row").toBeLessThanOrEqual(4);
  expect(Math.abs(action.height - field.height), "single-line control heights match").toBeLessThanOrEqual(4);
}
async function shot(page: Page, name: string, locale: string, viewport: string) {
  // READ/MEASURE only: reads layout dimensions; does not manipulate DOM or state.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  const file = `${name}-${locale}-${viewport}.png`;
  await page.screenshot({ path: path.join(evidence, file), animations: "disabled" });
  const manifestFile = path.join(evidence, "screenshots.json");
  let manifest: unknown[] = [];
  try { manifest = JSON.parse(await readFile(manifestFile, "utf8")) as unknown[]; } catch { /* first shot */ }
  manifest.push({ File: file, Sha256: createHash("sha256").update(await readFile(path.join(evidence, file))).digest("hex"), Locale: locale, Viewport: viewport });
  await writeFile(manifestFile, JSON.stringify(manifest, null, 2));
}

test("ledger filters, details, capabilities, object links, next page and store isolation", async ({ page }) => {
  await login(page); await ledger(page);
  await expect(page.getByTestId(`operation-row-${ops.query}`)).toBeVisible();
  await expect(page.getByTestId(`operation-row-${ops.foreign}`)).toHaveCount(0);
  await open(page, "query");
  await expect(page.getByTestId("operation-drawer")).toContainText("Additional details are not available.");
  const detailRefresh = page.waitForResponse(r => r.url().endsWith(`/operations/${ops.query}`) && r.request().method() === "GET");
  await clicked(page, "operation-detail-refresh", "drawer events refresh", async () => { expect((await detailRefresh).status()).toBe(200); await detailLoaded(page, ops.query); await expect(page.getByTestId("operation-drawer")).toContainText("Additional details are not available."); });
  await expect(page.getByTestId("operation-drawer")).toContainText("The result is unknown. Query");
  await expect(page.getByTestId("operation-retry")).toHaveCount(0);
  await close(page);
  await filter(page, "FAILED");
  await expect(page.getByTestId(`operation-row-${ops.retry}`)).toBeVisible();
  await expect(page.getByTestId(`operation-row-${ops.cancel}`)).toHaveCount(0);
  await open(page, "failed"); await expect(page.getByTestId("operation-retry")).toHaveCount(0);
  await expect(page.getByTestId("operation-drawer")).toContainText("does not support retry"); await close(page);
  await filter(page, "UNKNOWN"); await expect(page.getByTestId(`operation-row-${ops.query}`)).toBeVisible();
  await expect(page.getByTestId(`operation-row-${ops.retry}`)).toHaveCount(0);
  await filter(page, "READY"); await open(page, "protective");
  await expect(page.getByTestId("operation-cancel")).toHaveCount(0);
  await expect(page.getByTestId("operation-drawer")).toContainText("protective operation");
  const adLink = page.getByTestId("operation-drawer").getByRole("link").filter({ hasText: ops.draft_object });
  await expect(adLink).toHaveAttribute("href", `/en/ads?store=${store}&draft=${ops.draft_object}`);
  await adLink.click(); await expect(page).toHaveURL(new RegExp(`/en/ads\\?store=${store}&draft=${ops.draft_object}`));
  await ledger(page); await open(page, "order");
  const orderLink = page.getByTestId("operation-drawer").getByRole("link").filter({ hasText: ops.order_object });
  await expect(orderLink).toHaveAttribute("href", `/en/orders?store=${store}&order=${ops.order_object}`);
  await expect(orderLink).toBeVisible();
  // The order projection containing the request canary is loaded before checking the public UI boundary.
  await detailLoaded(page, ops.order);
  await expect(page.locator("body")).not.toContainText("SYNTHETIC-DO-NOT-EXPOSE");
  await orderLink.click(); await expect(page).toHaveURL(new RegExp(`/en/orders\\?store=${store}&order=${ops.order_object}`));
  await ledger(page);
  await clicked(page, "operations-next", "second page loads", async () => { await expect(page.locator('[data-testid^="operation-row-"]')).toHaveCount(19); await expect(page.getByTestId(`operation-row-${ops.query}`)).toHaveCount(0); });
  const listRefresh = page.waitForResponse(r => r.url().includes(`/stores/${store}/operations?`) && r.request().method() === "GET");
  await clicked(page, "operations-refresh", "current page refreshed", async () => { expect((await listRefresh).status()).toBe(200); await expect(page.locator('[data-testid^="operation-row-"]')).toHaveCount(19); await expect(page.getByTestId(`operation-row-${ops.query}`)).toHaveCount(0); });
  await page.reload(); await expect(page.getByTestId(`operation-row-${ops.query}`)).toBeVisible();
  await page.getByTestId("operations-store").selectOption(otherStore);
  await expect(page).toHaveURL(new RegExp(`store=${otherStore}`));
  await expect(page.getByTestId(`operation-row-${ops.foreign}`)).toBeVisible();
  await page.goto(`${origin}/en/settings/operations?store=${otherStore}&operation=${ops.cancel}`);
  await foreignViewLoaded(page);
  await expect(page.getByTestId("operation-drawer")).not.toContainText(ops.cancel);
  await expect(page.getByTestId(`operation-row-${ops.cancel}`)).toHaveCount(0);
  await page.reload(); await foreignViewLoaded(page);
  await expect(page.getByTestId("operation-drawer")).not.toContainText(ops.cancel);
  await expect(page.getByTestId(`operation-row-${ops.cancel}`)).toHaveCount(0);
});

test("cancel, registered read retry and query persist after refresh", async ({ page }) => {
  await login(page); await ledger(page, "en", ops.cancel);
  await clicked(page, "operation-cancel", "confirmation opens", async () => { await expect(page.getByTestId("operation-confirm")).toBeVisible(); });
  await clicked(page, "operation-confirm-dismiss", "dismiss does not cancel", async () => { await expect(page.getByTestId("operation-cancel")).toBeVisible(); await expect(page.getByTestId("operation-confirm")).not.toBeVisible(); });
  await action(page, "cancel", "cancel persisted", async () => { await expect(page.getByTestId("operation-drawer")).toContainText("Cancelled by merchant"); });
  await page.reload(); await expect(page.getByTestId("operation-drawer")).toContainText("Cancelled by merchant");
  await ledger(page, "en", ops.retry);
  await action(page, "retry", "read-only retry queued", async () => { await expect(page.getByTestId("operation-drawer")).toContainText("Retry authorized"); });
  await page.reload(); await expect(page.getByTestId("operation-drawer")).toContainText("Retry authorized");
  await ledger(page, "en", ops.query);
  await action(page, "query", "query persisted without changing UNKNOWN", async () => { await expect(page.getByTestId("operation-drawer")).toContainText("Status query requested"); });
  await page.reload(); await expect(page.getByTestId("operation-drawer")).toContainText("Status query requested");
  await expect(page.getByTestId("operation-drawer")).toContainText("The result is unknown");
  await expect(page.getByTestId("operation-retry")).toHaveCount(0);
});

test("fresh server CAS and daily query cap refuse stale confirmations", async ({ page }) => {
  await login(page); await ledger(page, "en", ops.cas);
  await clicked(page, "operation-cancel", "CAS confirmation opens", async () => { await expect(page.getByTestId("operation-confirm")).toBeVisible(); });
  await control("cas");
  await clicked(page, "operation-confirm-submit", "operation_changed explains refresh", async () => { await expect(page.getByTestId("operation-feedback")).toContainText("operation changed"); });
  await page.reload(); await detailLoaded(page, ops.cas); await expect(page.getByTestId("operation-drawer")).not.toContainText("Cancelled by merchant");
  await ledger(page, "en", ops.limit);
  await clicked(page, "operation-query", "quota confirmation opens", async () => { await expect(page.getByTestId("operation-confirm")).toBeVisible(); });
  await control("limit");
  const rejected = page.waitForResponse(r => r.url().endsWith(`/operations/${ops.limit}/query`) && r.request().method() === "POST");
  await clicked(page, "operation-confirm-submit", "query_limit shows Retry-After seconds", async () => { await expect(page.getByTestId("operation-feedback")).toContainText("daily query limit"); await expect(page.getByTestId("operation-drawer")).toContainText(/Retry after \(seconds\): [1-9][0-9]*/); });
  const response = await rejected; expect(response.status()).toBe(429); expect(Number(response.headers()["retry-after"])).toBeGreaterThan(0);
  await page.reload(); await detailLoaded(page, ops.limit); await expect(page.getByTestId("operation-query-reason")).toContainText("daily query limit"); await expect(page.getByTestId("operation-query")).toHaveCount(0);
});

async function ads(page: Page, locale = "en") {
  await page.goto(`${adsOrigin}/${locale}/ads?store=${adsStore}`);
  await expect(page.getByTestId("merchant-ads")).toBeVisible();
  await expect(page.getByTestId("ads-connections")).toContainText(account);
}
async function unbind(page: Page, expected: string, verify: () => Promise<void>) {
  await clicked(page, "ads-unbind", "pause-first/history confirmation opens", async () => {
    await expect(page.getByTestId("ads-unbind-confirm")).toContainText(/pause/i);
    await expect(page.getByTestId("ads-unbind-confirm")).toContainText(/histor/i);
  });
  await clicked(page, "ads-unbind-yes", expected, verify);
}

test("read-only permissions hide mutation controls after reload", async ({ page }) => {
  await control("readonly");
  try {
    await login(page); await ledger(page, "en", ops.cas);
    await expect(page.getByTestId("operation-cancel")).toHaveCount(0);
    await expect(page.getByTestId("operation-query")).toHaveCount(0);
    await expect(page.getByTestId("operation-retry")).toHaveCount(0);
    await page.reload(); await detailLoaded(page, ops.cas); await expect(page.getByTestId("operation-cancel-reason")).toContainText("permission"); await expect(page.getByTestId("operation-cancel")).toHaveCount(0);
    await ledger(page, "en", ops.reader); await expect(page.getByTestId("operation-query")).toHaveCount(0);
    await login(page, adsOrigin); await ads(page);
    await expect(page.getByTestId("ads-feed-url")).toHaveValue(feedURL);
    await expect(page.getByTestId("ads-unbind")).toHaveCount(0);
  } finally { await control("restore"); }
});

test("ad unbind refuses counting/in-flight operations, then preserves history after refresh; feed copies", async ({ page, context }) => {
  await login(page, adsOrigin); await ads(page);
  await context.grantPermissions(["clipboard-read", "clipboard-write"], { origin: adsOrigin });
  await expect(page.getByTestId("ads-feed-url")).toHaveValue(feedURL);
  await clicked(page, "ads-unbind", "confirmation opens", async () => { await expect(page.getByTestId("ads-unbind-confirm")).toBeVisible(); });
  await clicked(page, "ads-unbind-cancel", "confirmation dismissed without unbinding", async () => { await expect(page.getByTestId("ads-unbind-confirm")).not.toBeVisible(); });
  await clicked(page, "ads-feed-copy", "public feed copied", async () => {
    // READ only: verifies clipboard content after the genuine button click.
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(feedURL);
  });
  await unbind(page, "binding_in_use explains pause first", async () => { await expect(page.getByTestId("ads-unbind-error")).toContainText(/pause/i); });
  await page.reload(); await expect(page.getByTestId("ads-unbind")).toBeVisible();
  await control("ads/inflight");
  await unbind(page, "operations_in_flight lists the concrete operation", async () => {
    await expect(page.getByTestId("ads-unbind-in-flight")).toContainText(adOperation);
    await expect(page.getByTestId("ads-unbind-in-flight")).toContainText("meta.ads.activate");
    await expect(page.getByTestId("ads-unbind-in-flight")).toContainText("Result unknown");
  });
  await clicked(page, "ads-unbind-cancel", "in-flight dialog closes", async () => { await expect(page.getByTestId("ads-unbind-confirm")).not.toBeVisible(); });
  // The same persisted in-flight operation must use merchant wording in every supported locale.
  for (const [locale, stateLabel] of [["zh-TW", "結果未知"], ["zh-CN", "结果未知"]] as const) {
    await ads(page, locale);
    await clicked(page, "ads-unbind", "localized confirmation opens", async () => { await expect(page.getByTestId("ads-unbind-confirm")).toBeVisible(); });
    await clicked(page, "ads-unbind-yes", "localized in-flight state", async () => {
      await expect(page.getByTestId("ads-unbind-in-flight")).toContainText(adOperation);
      await expect(page.getByTestId("ads-unbind-in-flight")).toContainText(stateLabel);
      await expect(page.getByTestId("ads-unbind-in-flight")).not.toContainText("UNKNOWN");
    });
    await clicked(page, "ads-unbind-cancel", "localized confirmation closes", async () => { await expect(page.getByTestId("ads-unbind")).toBeVisible(); await expect(page.getByTestId("ads-unbind-confirm")).not.toBeVisible(); });
  }
  await ads(page);
  await control("ads/clear");
  await unbind(page, "local account detached", async () => { await expect(page.getByTestId("ads-connections")).toContainText("Disabled"); await expect(page.getByTestId("ads-unbind")).toHaveCount(0); });
  await page.reload(); await expect(page.getByTestId("ads-connections")).toContainText(account);
  await expect(page.getByTestId("ads-connections")).toContainText("Disabled");
  await expect(page.getByTestId("ads-unbind")).toHaveCount(0);
  await expect(page.getByTestId(`ads-draft-${draft}`)).toBeVisible();
  await control("ads/unpublish"); await page.reload();
  await expect(page.getByTestId("ads-feed-empty")).toContainText(/publish/i);
  await expect(page.getByTestId("ads-feed-copy")).toHaveCount(0);
  await control("ads/publish"); await page.reload(); await expect(page.getByTestId("ads-feed-url")).toHaveValue(feedURL);
});


for (const locale of ["zh-TW", "en", "zh-CN"]) for (const width of [1586, 390]) {
  test(`labels and no overflow ${locale} ${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 992 }); await login(page); await ledger(page, locale);
    await expect(page.getByTestId("operations-store")).toHaveAccessibleName(locale === "en" ? "Ledger store" : locale === "zh-TW" ? "台賬商店" : "台账店铺");
    await expect(page.getByTestId("operations-filter")).toHaveAccessibleName(/.+/);
    await expect(page.getByTestId("operations-refresh")).toHaveAccessibleName(/.+/);
    const refreshed = page.waitForResponse(r => r.url().includes(`/stores/${store}/operations?`) && r.request().method() === "GET");
    await clicked(page, "operations-refresh", "ledger refresh completes", async () => { const response = await refreshed; expect(response.status()).toBe(200); await response.finished(); await expect(page.getByTestId(`operation-row-${ops.cas}`)).toBeVisible(); });
    await toolbarAligned(page);
    await open(page, "protective"); await expect(page.getByTestId("operation-drawer")).toHaveRole("dialog");
    await shot(page, "ledger", locale, width === 390 ? "mobile" : "desktop"); await close(page);
    await login(page, adsOrigin); await ads(page, locale);
    await expect(page.getByTestId("ads-feed-copy")).toHaveAccessibleName(/.+/);
    await shot(page, "ads", locale, width === 390 ? "mobile" : "desktop");
  });
}
