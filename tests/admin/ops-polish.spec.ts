// ops-polish independent gates (docs/delivery/units/ops-polish.md OP2, OP3 UI half, OP4), real admin Next build -> private Go API -> isolated PG,
// signed MOCK IdP. Driven by tests/foundation/browser_ops_polish_test.go (TestBrowserOpsPolishAdmin), written from the brief, not the implementation.
// BFF routes exercised: GET /api/stores/{store}/orders (list poll), /orders/{id} (detail), /finance/summary(.csv) -> Go internal/httpapi.
// OP2 uses Playwright's fake clock (page.clock): the 20 s cadence is proven by advancing fake time, never by waiting. New orders are placed by the Go
// test through the real buyer path on demand (LC_OPP_PLACE_URL), so the real list route is what returns them. Locators avoid implementation test ids
// except the two the baseline already had (order-expand-{id}, state-filter) and the page roots.
import { expect, test, type Locator, type Page } from "@playwright/test";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const store = required("LC_BROWSER_STORE");
const placeURL = required("LC_OPP_PLACE_URL");
const seed = JSON.parse(required("LC_OPP_SEED")) as { pending: string[]; collected: string[]; collectedMinor: number };

test.use({ baseURL: origin, trace: "retain-on-failure", screenshot: "only-on-failure" });

const pause = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
async function placeOrder(): Promise<string> {
  const res = await fetch(placeURL);
  if (!res.ok) throw new Error(`place order failed: ${res.status}`);
  return ((await res.json()) as { order_id: string }).order_id;
}
async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("nav-orders")).toBeVisible();
}
async function openOrders(page: Page, locale: string, state = "") {
  await page.goto(new URL(`/${locale}/orders?store=${store}${state ? `&state=${state}` : ""}`, origin).toString());
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  const selector = page.getByTestId("store-selector");
  if (await selector.count()) if ((await selector.inputValue()) !== store) await selector.selectOption(store);
  await expect(page.getByTestId("orders-table")).toBeVisible();
}
// the rendered order row of an id (the baseline expand button is the one stable handle)
const row = (page: Page, id: string): Locator => page.getByTestId(`order-expand-${id}`).locator("xpath=ancestor::tr[1]");
const marker = (page: Page, id: string, word: RegExp): Locator => row(page, id).getByText(word);

const listRoute = /\/api\/stores\/[^/]+\/orders(\?.*)?$/;
type Probe = { calls: number[]; inflight: number; maxInflight: number; hold: number };
async function probeList(page: Page): Promise<Probe> {
  const probe: Probe = { calls: [], inflight: 0, maxInflight: 0, hold: 0 };
  await page.route(listRoute, async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    probe.calls.push(Date.now());
    probe.inflight++;
    probe.maxInflight = Math.max(probe.maxInflight, probe.inflight);
    try {
      const response = await route.fetch();
      if (probe.hold) await pause(probe.hold);
      await route.fulfill({ response });
    } finally {
      probe.inflight--;
    }
  });
  return probe;
}
// advances the fake clock in `step` ms slices (real pauses let the page's fetch reach the route) until a list request starts
async function runUntilPoll(page: Page, probe: Probe, maxMs = 21_000, step = 250): Promise<number> {
  const before = probe.calls.length;
  let advanced = 0;
  while (probe.calls.length === before && advanced < maxMs) {
    await page.clock.runFor(step);
    advanced += step;
    await pause(30);
  }
  expect(probe.calls.length, `no list request within ${maxMs} fake ms`).toBeGreaterThan(before);
  await expect.poll(() => probe.inflight).toBe(0);
  return advanced;
}
const setVisibility = (page: Page, state: "hidden" | "visible") =>
  page.evaluate((value) => {
    Object.defineProperty(document, "visibilityState", { configurable: true, get: () => value });
    Object.defineProperty(document, "hidden", { configurable: true, get: () => value === "hidden" });
    document.dispatchEvent(new Event("visibilitychange"));
  }, state);

test("OP2 live order feed: 20 s poll, no overlap, pause when hidden, new marker, (N) title, filters kept", async ({ page }) => {
  test.setTimeout(240_000);
  const probe = await probeList(page);
  await page.clock.install();
  await signedLogin(page);
  await openOrders(page, "en", "CONFIRMED");
  const baseTitle = await page.title();
  expect(baseTitle).not.toMatch(/^\(\d+\)/);
  // the merchant's own context: a filter and an opened order
  const [openId] = seed.pending;
  await page.getByTestId(`order-expand-${openId}`).click();
  await expect(page.getByTestId("order-detail")).toBeVisible();
  const urlBefore = page.url();
  for (const id of [...seed.pending, ...seed.collected]) await expect(row(page, id)).toBeVisible();
  // no marker on anything the merchant already saw
  await expect(page.getByText(/^New$/)).toHaveCount(0);

  // from here time moves only when the test says so
  await page.clock.pauseAt(new Date((await page.evaluate(() => Date.now())) + 100));
  await pause(300);
  const idle = probe.calls.length;
  const one = await placeOrder();
  const firstWithin = await runUntilPoll(page, probe);
  expect(firstWithin, "the first poll must fire within one 20 s interval of the mount").toBeLessThanOrEqual(20_250);
  expect(probe.calls.length).toBe(idle + 1);
  // the cadence: nothing for 19 s after a poll, exactly one more by 20.5 s
  const afterFirst = probe.calls.length;
  await page.clock.runFor(19_000);
  await pause(300);
  expect(probe.calls.length, "a poll fired before 19 s had passed since the previous one").toBe(afterFirst);
  await page.clock.runFor(1_500);
  await expect.poll(() => probe.calls.length, { message: "the next poll must fire at 20 s" }).toBe(afterFirst + 1);
  await expect.poll(() => probe.inflight).toBe(0);

  // the new order carries a visible marker and the tab title the count; the seen orders do not
  await expect(marker(page, one, /^New$/)).toBeVisible();
  await expect(page.getByText(/^New$/)).toHaveCount(1);
  await expect.poll(() => page.title()).toMatch(/^\(1\) /);
  // brief OP2: the badge names the orders page ("(N) 訂單"), not the static layout title (fixed by agent/kimi-p2-ui)
  expect(await page.title()).toMatch(/^\(1\) (Orders|订单|訂單)$/);
  // filters and the open order are exactly as the merchant left them
  expect(page.url()).toBe(urlBefore);
  await expect(page.getByTestId("state-filter")).toHaveValue("CONFIRMED");
  await expect(page.getByTestId("order-detail")).toBeVisible();
  await expect(page.getByTestId(`order-expand-${openId}`)).toHaveAttribute("aria-expanded", "true");

  // opening the new order acknowledges it: marker and count go away
  await page.getByTestId(`order-expand-${one}`).click();
  await expect(page.getByText(/^New$/)).toHaveCount(0);
  await expect.poll(() => page.title()).toBe(baseTitle);

  // two more orders: the count is cumulative over unseen orders
  const two = [await placeOrder(), await placeOrder()];
  await runUntilPoll(page, probe);
  for (const id of two) await expect(marker(page, id, /^New$/)).toBeVisible();
  await expect.poll(() => page.title()).toMatch(/^\(2\) (Orders|订单|訂單)$/);

  // never overlapping: a slow list, three more intervals pass while it is in flight, still exactly one request
  probe.hold = 1_500;
  const beforeSlow = probe.calls.length;
  await page.clock.runFor(20_000);
  await expect.poll(() => probe.inflight).toBe(1);
  await page.clock.runFor(60_000);
  await pause(200);
  expect(probe.calls.length, "a second list request started while the first was in flight").toBe(beforeSlow + 1);
  await expect.poll(() => probe.inflight, { timeout: 10_000 }).toBe(0);
  probe.hold = 0;
  expect(probe.maxInflight).toBe(1);

  // paused while hidden: long hidden spells fire no list request at all
  await setVisibility(page, "hidden");
  await pause(300);
  const hiddenAt = probe.calls.length;
  await page.clock.runFor(120_000);
  await pause(400);
  expect(probe.calls.length, "list requests were made while the tab was hidden").toBe(hiddenAt);
  const whileHidden = await placeOrder();
  await page.clock.runFor(60_000);
  await pause(300);
  expect(probe.calls.length).toBe(hiddenAt);
  // coming back reads the list at once and the order that arrived meanwhile is marked
  await setVisibility(page, "visible");
  await expect.poll(() => probe.calls.length, { timeout: 15_000 }).toBeGreaterThan(hiddenAt);
  await expect(marker(page, whileHidden, /^New$/)).toBeVisible({ timeout: 15_000 });
  await expect.poll(() => page.title()).toMatch(/^\(\d+\) /);
  // and the cadence resumes
  const resumed = await placeOrder();
  await runUntilPoll(page, probe);
  await expect(marker(page, resumed, /^New$/)).toBeVisible();
  expect(probe.maxInflight).toBe(1);
});

for (const [locale, word] of [["zh-TW", /^新$/], ["zh-CN", /^新$/]] as const) {
  test(`OP2 new marker and title count in ${locale}`, async ({ page }) => {
    test.setTimeout(120_000);
    const probe = await probeList(page);
    await page.clock.install();
    await signedLogin(page);
    await openOrders(page, locale);
    const baseTitle = await page.title();
    await page.clock.pauseAt(new Date((await page.evaluate(() => Date.now())) + 100));
    await pause(300);
    const id = await placeOrder();
    await runUntilPoll(page, probe);
    await expect(marker(page, id, word)).toBeVisible();
    await expect.poll(() => page.title()).toMatch(/^\(1\) (Orders|订单|訂單)$/);
  });
}

// ---- OP4: copy and navigation ------------------------------------------------------------------------------------------------------
const removedNav: Record<string, string[]> = {
  en: ["Website service", "Meta messages", "Platform support"],
  "zh-CN": ["网站客服", "Meta 消息", "平台支持"],
  "zh-TW": ["網站客服", "Meta 訊息", "平台支援"],
};
for (const locale of ["en", "zh-CN", "zh-TW"]) {
  test(`OP4 Studio subtitle is neutral and the dead nav entries are gone in ${locale}`, async ({ page }) => {
    await signedLogin(page);
    await page.goto(new URL(`/${locale}/studio?store=${store}`, origin).toString());
    const studio = page.getByTestId("merchant-studio");
    await expect(studio).toBeVisible();
    const heading = studio.locator("header").first();
    const text = (await heading.innerText()).trim();
    expect(text, "the Studio heading must carry a subtitle line").toMatch(/\n./);
    expect(text, "the local rehearsal wording must be gone").not.toMatch(/MOCK|rehears|演练|演練|模拟|模擬|local/i);
    const nav = page.getByRole("navigation").first();
    const labels = (await nav.getByRole("button").allInnerTexts()).map((s) => s.trim());
    for (const dead of removedNav[locale]) expect(labels, `nav still lists "${dead}"`).not.toContain(dead);
    expect(labels.length, `remaining nav entries ${JSON.stringify(labels)}`).toBe(9);
    // none of the remaining entries is a placeholder panel
    await expect(page.getByText(/not connected in the current build|当前版本尚未连接|目前版本尚未連線|目前版本尚未连接/)).toHaveCount(0);
  });
}
test("OP4 the same nav on the orders page", async ({ page }) => {
  await signedLogin(page);
  for (const locale of ["en", "zh-CN", "zh-TW"]) {
    await openOrders(page, locale);
    const labels = (await page.getByRole("navigation").first().getByRole("button").allInnerTexts()).map((s) => s.trim());
    for (const dead of removedNav[locale]) expect(labels).not.toContain(dead);
    expect(labels.length).toBe(9);
  }
});

// ---- OP3 UI half: the finance page and its CSV link ------------------------------------------------------------------------------
test("OP3 finance page shows a separate pay-at-pickup column in every locale; CSV link carries it", async ({ page }) => {
  await signedLogin(page);
  const today = new Date(Date.now() + 8 * 3600_000).toISOString().slice(0, 10);
  const header: Record<string, RegExp> = { en: /pay[- ]at[- ]pickup/i, "zh-TW": /取貨付款/, "zh-CN": /取货付款/ };
  for (const locale of ["en", "zh-TW", "zh-CN"]) {
    await page.goto(new URL(`/${locale}/finance?store=${store}`, origin).toString());
    await expect(page.getByTestId("finance-page")).toBeVisible();
    const from = page.getByTestId("finance-from");
    await from.fill(today);
    await page.getByTestId("finance-to").fill(today);
    await page.getByTestId("finance-show").click();
    const table = page.getByTestId("finance-table");
    await expect(table).toBeVisible();
    await expect(table.locator("thead th").filter({ hasText: header[locale] })).toHaveCount(1);
    const line = page.getByTestId(`finance-row-${today}-TWD`);
    await expect(line).toBeVisible();
    // the carrier-collected money is in its own cell, with the order count; captured and net stay at zero
    await expect(line.locator("td").filter({ hasText: new RegExp(`\\(${seed.collected.length}\\)`) })).toHaveCount(1);
  }
  const href = await page.getByTestId("finance-csv").getAttribute("href");
  expect(href).toBeTruthy();
  // in-page fetch: the Secure __Host- session cookie is sent by the browser, not by the APIRequestContext jar over http
  const csv = await page.evaluate(async (u) => {
    const r = await fetch(u, { credentials: "same-origin" });
    return { status: r.status, text: await r.text() };
  }, new URL(href!, origin).toString());
  expect(csv.status).toBe(200);
  const lines = csv.text.trim().split(/\r?\n/);
  expect(lines[0]).toBe("day,currency,environment,captured_count,captured_minor,refunded_minor,net_minor,pickup_collected_count,pickup_collected_minor");
  const mine = lines.slice(1).find((l) => l.startsWith(`${today},TWD,LIVE`));
  expect(mine, `no LIVE row for ${today}: ${lines.join(" | ")}`).toBe(`${today},TWD,LIVE,0,0,0,0,${seed.collected.length},${seed.collectedMinor}`);
});
