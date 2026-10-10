// Purpose: LC-U2b2 real signed A14 link/CAS/unlink and complete A13 acceptance in six locale/viewports.
// Depends on: Go-owned synthetic real-service fixtures, MOCK OIDC and unmodified Next/BFF/PG routes.
// Used by: --browser-inbox normal and INU05 calibration; no fake API successes or DOM mutations.
import { expect, test, type Page, type APIRequestContext } from "@playwright/test";
import { resolve } from "node:path";
import { required, origin, api, evidence, store, login, privateBoundary,
  createInboxLedger, writeInboxLedger } from "./inbox-browser-support";

let fixture: {
  conversation: string; session: string; customer_a: string; customer_b: string;
  name_a: string; name_b: string; confirmed_number: string; cancelled_number: string;
};
// Legacy discovery deliberately supplies only the original inbox fixture. Omit INU10 only
// in that collection-only command; an actual run without the new fixture must still fail.
const fixturelessCollection = process.argv.includes("--list") && !process.env.LC_BROWSER_INBOX_CUSTOMER_FIXTURE;
if (fixturelessCollection) console.error("NOT_RUN INU10: customer fixture absent in collection-only command");
test.beforeEach(() => { fixture = JSON.parse(required("LC_BROWSER_INBOX_CUSTOMER_FIXTURE")); });
const { ledger, record } = createInboxLedger();
// Separate headless contexts keep both real pages visible for CAS. INU05 separately exercises native hiding.
// Disable traces so the test-only actor-control credential and signed login responses cannot enter artifacts.
test.use({ baseURL: origin, headless: true, trace: "off", video: "off", screenshot: "off" });
test.afterEach(async ({}, info) => {
  if (info.status !== info.expectedStatus) ledger.push({ page: "Messages", control: info.title,
    action: "scenario", expected: "all named assertions pass", actual: info.status ?? "UNKNOWN", status: "FAIL" });
});
test.afterAll(async () => { await writeInboxLedger(ledger); });
const copies = [
  { locale: "zh-TW", linked: "已手動連結", conflict: "客戶綁定已變更，請重新整理後再操作。", ordinal: "第 1 次購買", auto: "自動回覆: 已送出" },
  { locale: "zh-CN", linked: "已手动连接", conflict: "客户绑定已改变，请刷新后再操作。", ordinal: "第 1 次购买", auto: "自动回复: 已发送" },
  { locale: "en", linked: "Manually linked", conflict: "Customer link changed. Refresh before linking again.", ordinal: "Purchase #1", auto: "Automatic reply: Sent" },
] as const;

async function actor(request: APIRequestContext, mode: "qualified" | "operator") {
  // Fixture identity selection only; each page still signs in through the real OIDC/BFF flow.
  const response = await request.post(`${api}/__test/inbox-actor`, {
    headers: { "X-Inbox-Fixture-Key": required("LC_BROWSER_INBOX_CONTROL_KEY") }, data: { mode },
  });
  expect(response.status(), "bounded fixture actor selection").toBe(204);
}
async function openBuyer(page: Page) {
  await page.getByTestId(`conversation-${fixture.conversation}`).click();
  await expect(page.getByTestId("buyer-panel")).toHaveAttribute("aria-busy", "false");
  await expect(page.getByTestId("customer-link-choice")).toBeEnabled();
}
async function enter(page: Page, locale: string) {
  await login(page);
  if (locale !== "en") {
    await page.getByTestId("locale-switch").selectOption(locale);
    await expect(page).toHaveURL(new RegExp(`/${locale}/messages`));
  }
  await openBuyer(page);
}
async function postLink(page: Page, control: "customer-link-save" | "customer-link-remove", status: number) {
  const pending = page.waitForResponse(response => response.request().method() === "POST" &&
    new URL(response.url()).pathname === `/api/stores/${store}/inbox/conversations/${fixture.conversation}/customer-link`);
  await page.getByTestId(control).click();
  const response = await pending;
  expect(response.status(), "real A14 status").toBe(status);
  expect(response.headers()["cache-control"]).toContain("no-store");
  await expect(page.getByTestId("buyer-panel")).toHaveAttribute("aria-busy", "false");
  return response;
}
async function linked(page: Page, label: string, name: string) {
  await expect(page.getByTestId("customer-manual-link")).toHaveText(`${label} · ${name}`);
}
async function fullPanel(page: Page, copy: typeof copies[number]) {
  const panel = page.getByTestId("buyer-panel");
  await expect(panel).toContainText(copy.ordinal);
  await expect(panel).toContainText(copy.auto);
  await expect(panel.locator("li").filter({ hasText: "B2 × 2" })).toContainText(fixture.session);
  await expect(panel.locator("li").filter({ hasText: "B2 × 1" })).toContainText(fixture.session);
  await expect(panel.locator("li").filter({ hasText: fixture.confirmed_number })).toContainText("CONFIRMED");
  await expect(panel.locator("li").filter({ hasText: fixture.cancelled_number })).toContainText("CANCELLED");
  await expect(panel.locator("li")).toHaveCount(4);
  // Independent literal: two claims at 700 minor, quantities 2+1 = TWD 21. No expected value read from A13.
  await expect(panel.locator("p").filter({ hasText: /NT\$\s*21(?:\.00)?$/ })).toHaveCount(1);
}
for (const copy of (fixturelessCollection ? [] : copies)) for (const width of [1440, 390]) {
  test(`INU10 ${copy.locale}/${width} full A13 and real two-page customer-link CAS`, async ({ browser, request }) => {
    test.setTimeout(90000);
    const contexts = await Promise.all([browser.newContext({ viewport: { width, height: 900 } }), browser.newContext({ viewport: { width, height: 900 } })]);
    const [a, b] = await Promise.all(contexts.map(context => context.newPage()));
    const logs: string[] = [];
    const requestedURLs: string[] = []; // In-memory history survives reload; never persisted in evidence.
    for (const page of [a, b]) {
      page.on("request", request => requestedURLs.push(request.url()));
      page.on("console", message => logs.push(message.text()));
      page.on("pageerror", error => logs.push(error.message));
    }
    try {
      await actor(request, "qualified");
      await enter(a, copy.locale);
      await enter(b, copy.locale);
      await fullPanel(a, copy);
      await fullPanel(b, copy);
      await expect(a.getByTestId("customer-manual-link")).toHaveCount(0);
      await a.getByTestId("customer-link-choice").selectOption(fixture.customer_a);
      await b.getByTestId("customer-link-choice").selectOption(fixture.customer_b);
      const winning = await postLink(b, "customer-link-save", 200);
      const winner = await winning.json() as { version: number; customer_id: string };
      expect(winner.customer_id).toBe(fixture.customer_b);
      await linked(b, copy.linked, fixture.name_b);
      await b.reload(); await openBuyer(b);
      await linked(b, copy.linked, fixture.name_b);
      record(`link persistence ${copy.locale}/${width}`, "select customer B, link, reload, reopen", "real A14 link persists with explicit customer name");

      const aWrites: string[] = [];
      a.on("request", req => {
        if (req.method() === "POST" && new URL(req.url()).pathname.endsWith(`/${fixture.conversation}/customer-link`))
          aWrites.push(req.headers()["idempotency-key"] ?? "");
      });
      const refused = await postLink(a, "customer-link-save", 409);
      expect(refused.request().postDataJSON()).toEqual({ customer_id: fixture.customer_a, expected_version: winner.version - 1 });
      await expect(a.getByTestId("buyer-panel").getByRole("alert")).toHaveText(copy.conflict);
      await expect(a.getByTestId("customer-link-choice")).toHaveValue(fixture.customer_a);
      await linked(a, copy.linked, fixture.name_b);
      await expect(a.getByTestId("customer-link-save")).toBeEnabled();
      expect(aWrites).toHaveLength(1); // Refresh completed; no automatic write was allowed.
      const retried = await postLink(a, "customer-link-save", 200);
      expect(retried.request().postDataJSON()).toEqual({ customer_id: fixture.customer_a, expected_version: winner.version });
      expect(aWrites).toHaveLength(2);
      expect(aWrites[0]).toMatch(/^[A-Za-z0-9_-]{8,128}$/);
      expect(aWrites[1]).not.toBe(aWrites[0]);
      await linked(a, copy.linked, fixture.name_a);
      await a.reload(); await openBuyer(a);
      await linked(a, copy.linked, fixture.name_a);
      await fullPanel(a, copy);
      // G-UI8 audit [READ/MEASURE]: measure actual page geometry after real signed clicks.
      expect(await a.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await a.screenshot({ path: resolve(evidence, `inbox-customer-${copy.locale}-${width}.png`), fullPage: true });
      record(`A13 and CAS ${copy.locale}/${width}`, "two signed pages race then explicit retry", "claims/session/TWD total/two order states/ordinal/auto reply render; 409 preserves choice and refreshes winner");
      const unlinked = await postLink(a, "customer-link-remove", 200);
      expect(unlinked.request().postDataJSON()).toEqual({ customer_id: null, expected_version: winner.version + 1 });
      await expect(a.getByTestId("customer-manual-link")).toHaveCount(0);
      await a.reload(); await openBuyer(a);
      await expect(a.getByTestId("customer-manual-link")).toHaveCount(0);
      await expect(a.getByTestId("customer-link-remove")).toBeDisabled();
      await fullPanel(a, copy);
      for (const page of [a, b]) {
        await privateBoundary(page, logs);
        // G-UI8 audit [READ/MEASURE]: customer names must not become URLs, browser storage or logs.
        const channels = await page.evaluate(() => JSON.stringify({ url: location.href, local: Object.entries(localStorage),
          session: Object.entries(sessionStorage), resources: performance.getEntriesByType("resource").map(entry => entry.name) }));
        for (const name of [fixture.name_a, fixture.name_b]) {
          expect(channels).not.toContain(name); expect(logs.join("\n")).not.toContain(name);
          for (const url of requestedURLs) {
            expect(url).not.toContain(name); expect(decodeURIComponent(url)).not.toContain(name);
          }
        }
      }
      record(`unlink ${copy.locale}/${width}`, "click unlink, reload, reopen", "A14 unlink persists; peer-linked A13 facts retained");
    } finally {
      await Promise.all(contexts.map(context => context.close()));
      await actor(request, "operator");
    }
  });
}
