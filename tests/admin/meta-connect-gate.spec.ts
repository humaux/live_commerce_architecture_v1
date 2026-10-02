// meta-connect browser gate MCG10, INDEPENDENT test-author half (contract meta-claims-intake-v1 "Merchant connect (R4)"; unit brief
// "Done when" + the owner's walk-through: merchant Settings -> connect (fake Meta dialog) -> pick a 渠道倉-香港-style Page with Instagram ->
// the card shows Page / Instagram / permissions / status -> the Studio claim-source picker offers it -> disconnect).
// BFF routes exercised through the UI: POST /api/meta/connect, GET /api/meta/callback, GET|POST /api/stores/{store}/meta-connect/*,
// GET /api/stores/{store}/live-sessions/{id}/claim-source (+ PUT) -> Go /v1/admin/stores/{store_id}/meta-connect/* and .../claim-source.
// Started only by tests/foundation/browser_meta_connect_gate_test.go (build tag browser): isolated PG, the real Go API with the
// metaconnect service + Studio claims, the production admin Next build, a signed mock IdP and a runner-only control listener.
// Labels: BROWSER, Meta = MOCK (https://www.facebook.com/** is answered by page.route with a 302 to the app's own callback; the code
// exchange / permissions / Pages / subscribe happen in the API process against tests/metaconnect/fakegraph).
// Matrix: Chromium desktop 1586x992 and 390x844; zh-TW and en. Locators use the cards' data-testid vocabulary and their copy files.
import { expect, test, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { metaConnectCopy } from "../../apps/admin/lib/meta-connect-copy";
import { claimsCopy } from "../../apps/admin/lib/claims-copy";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_STORE");
const scene = required("LC_BROWSER_SCENE");
const control = required("LC_BROWSER_CONTROL");
const controlKey = required("LC_BROWSER_CONTROL_KEY");
const pageA = { id: required("LC_BROWSER_PAGE_A"), name: required("LC_BROWSER_PAGE_A_NAME"), ig: required("LC_BROWSER_IG_A"), igUser: required("LC_BROWSER_IG_A_USER") };
const pageB = { id: required("LC_BROWSER_PAGE_B"), name: required("LC_BROWSER_PAGE_B_NAME") };
const pageC = { id: required("LC_BROWSER_PAGE_C"), name: required("LC_BROWSER_PAGE_C_NAME") };
const pageD = { id: required("LC_BROWSER_PAGE_D"), name: required("LC_BROWSER_PAGE_D_NAME") };

test.use({ baseURL: origin, trace: "retain-on-failure", screenshot: "only-on-failure" });
test.setTimeout(240_000);
type Loc = "zh-TW" | "zh-CN" | "en";
type View = "desktop" | "mobile";
const sizes: Record<View, { width: number; height: number }> = { desktop: { width: 1586, height: 992 }, mobile: { width: 390, height: 844 } };
const safeRedirect = /^\/(en|zh-TW|zh-CN)\/settings(\?[A-Za-z0-9_=&-]*)?$/;

async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("nav-orders")).toBeVisible();
}
async function ctl(resource: string) {
  const response = await fetch(`${control}/${resource}`, { method: "POST", headers: { "X-Gate-Key": controlKey } });
  expect(response.status, `control ${resource}`).toBe(200);
  return response;
}
async function facts() {
  return (await (await ctl("facts")).json()) as { subscribed: Record<string, boolean>; bindings: number; routes: number; heads: number; connections: number };
}
async function fitsWidth(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
}
const manifestPath = path.join(evidence, "screenshots.json");
async function shot(page: Page, name: string, locale: string, view: View) {
  const file = path.join(evidence, `${name}-${locale}-${view}.png`);
  await page.screenshot({ path: file, fullPage: false, animations: "disabled", scale: "css" });
  await fitsWidth(page);
  let manifest: unknown[] = [];
  try {
    manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  } catch {
    /* first screenshot */
  }
  manifest.push({ File: path.basename(file), Sha256: createHash("sha256").update(await readFile(file)).digest("hex"), Locale: locale, Viewport: view });
  await writeFile(manifestPath, JSON.stringify(manifest, null, 2));
}
async function noSecrets(page: Page) {
  const secrets = (await (await ctl("secrets")).json()) as string[];
  const html = await page.content();
  const stored = await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage }, cookie: document.cookie }));
  for (const secret of secrets) {
    expect(html).not.toContain(secret);
    expect(stored).not.toContain(secret);
  }
}

// What the fake Facebook dialog does for the next connect: a permission/Page profile, or a forged state.
let answer: { profile: string } | { wrongState: true } = { profile: "full" };
let lastCallback: string | null = null; // the exact return URL the browser was sent to (for the replay test)
async function answerDialog(page: Page) {
  await page.route("https://www.facebook.com/**", async (route) => {
    const url = new URL(route.request().url());
    const state = "wrongState" in answer ? "A".repeat(43) : (url.searchParams.get("state") ?? "");
    const profile = "wrongState" in answer ? "full" : answer.profile;
    const { code } = (await (await ctl(`oauth/code?profile=${profile}`)).json()) as { code: string };
    lastCallback = `${origin}/api/meta/callback?code=${encodeURIComponent(code)}&state=${encodeURIComponent(state)}`;
    await route.fulfill({ status: 302, headers: { Location: lastCallback } });
  });
}
async function openSettings(page: Page, locale: Loc) {
  await page.goto(`/${locale}/settings?store=${store}`);
  await expect(page.getByTestId("metaconnect-card")).toBeVisible();
}
async function openStudio(page: Page, locale: Loc) {
  await page.goto(`/${locale}/studio/claims?store=${store}&scene=${scene}`);
  await expect(page.getByTestId("claims-source")).toBeVisible({ timeout: 30_000 });
}

test.describe("meta-connect independent browser gate", () => {
  test.describe.configure({ mode: "serial" });

  test("zh-TW desktop: before any connection the Studio claim source points at the Settings card, and the link works", async ({ page }) => {
    const c = metaConnectCopy["zh-TW"];
    await signedLogin(page);
    await openStudio(page, "zh-TW");
    const hint = page.getByTestId("claims-source-connect");
    await expect(hint).toContainText(c.studioNone);
    const link = hint.getByRole("link", { name: c.studioLink });
    await expect(link).toHaveAttribute("href", `/zh-TW/settings?store=${store}`);
    await expect(page.locator("#claims-source-platform")).toHaveCount(0); // nothing to choose
    await shot(page, "studio-no-page", "zh-TW", "desktop");
    await link.click();
    await expect(page.getByTestId("metaconnect-card")).toBeVisible();
    await expect(page.getByTestId("metaconnect-none")).toHaveText(c.notConnected);
    await expect(page.getByTestId("metaconnect-connect")).toBeVisible();
  });

  // The owner's walk-through in all four locale x viewport combinations. The same Page is connected and disconnected each time (the
  // store keeps ownership of its Page across a disconnect, so reconnecting the same Page is the documented path).
  for (const [locale, view] of [["zh-TW", "desktop"], ["en", "mobile"], ["en", "desktop"], ["zh-TW", "mobile"]] as [Loc, View][]) {
    test(`${locale} ${view}: Settings -> connect (fake Meta) -> pick Page with Instagram -> card facts -> Studio picker offers it -> disconnect`, async ({ page }) => {
      const c = metaConnectCopy[locale];
      const cc = claimsCopy[locale];
      await page.setViewportSize(sizes[view]);
      await signedLogin(page);
      await openSettings(page, locale);
      await expect(page.getByRole("heading", { name: c.title, exact: true })).toBeVisible();
      await expect(page.getByTestId("metaconnect-none")).toHaveText(c.notConnected);
      await answerDialog(page);
      answer = { profile: "full" };
      await page.getByTestId("metaconnect-connect").click();
      await expect(page.getByTestId("metaconnect-pick")).toBeVisible({ timeout: 30_000 });
      // The pick list is this login's Pages only: A (with Instagram) and B (without).
      await expect(page.locator('[data-testid^="metaconnect-pick-"][type="radio"]')).toHaveCount(2);
      await expect(page.getByTestId("metaconnect-pick-list")).toContainText(pageA.name);
      await expect(page.getByTestId("metaconnect-pick-list")).toContainText(pageB.name);
      await page.getByTestId(`metaconnect-pick-${pageB.id}`).check();
      await expect(page.getByTestId("metaconnect-pick-ig")).toHaveCount(0); // IG absent: nothing to tick
      await page.getByTestId(`metaconnect-pick-${pageA.id}`).check();
      await expect(page.getByTestId("metaconnect-pick-ig")).toBeChecked(); // IG present: offered, on by default
      if (view === "mobile") await shot(page, "pick", locale, view);
      await page.getByTestId("metaconnect-pick-submit").click();
      await expect(page.getByTestId("metaconnect-status")).toBeVisible({ timeout: 30_000 });

      // The card: Page name + id, Instagram username, every granted permission, token status, last webhook time.
      await expect(page.getByTestId("metaconnect-notice")).toHaveText(c.connectedNotice);
      await expect(page.getByTestId("metaconnect-page")).toContainText(pageA.name);
      await expect(page.getByTestId("metaconnect-page")).toContainText(pageA.id);
      await expect(page.getByTestId("metaconnect-ig")).toHaveText(`@${pageA.igUser}`);
      for (const permission of ["pages_show_list", "pages_manage_metadata", "pages_read_engagement", "pages_messaging", "instagram_basic", "instagram_manage_comments", "instagram_manage_messages"])
        await expect(page.getByTestId("metaconnect-permissions")).toContainText(permission);
      await expect(page.getByTestId("metaconnect-token")).toHaveText(c.tokenActive);
      await expect(page.getByTestId("metaconnect-token")).toHaveAttribute("data-state", "active");
      await expect(page.getByTestId("metaconnect-last-event")).toHaveText(c.never);
      await expect(page.getByTestId("metaconnect-connect")).toHaveCount(0);
      const after = await facts();
      expect(after.subscribed.A).toBe(true);
      expect(after.subscribed.B).toBe(false);
      expect(after.bindings).toBe(2); // Facebook + Instagram, nothing for the unpicked Page B
      expect(after.routes).toBe(2);
      expect(after.heads).toBe(2);
      expect(after.connections).toBe(1);
      await noSecrets(page);
      await shot(page, "card-connected", locale, view);

      // Reload: the same card from the server, not from page state.
      await page.reload();
      await expect(page.getByTestId("metaconnect-page")).toContainText(pageA.name);
      await expect(page.getByTestId("metaconnect-ig")).toHaveText(`@${pageA.igUser}`);

      // Studio claim source: no more "connect a Page" dead end; the platform choice offers Facebook AND Instagram.
      await openStudio(page, locale);
      await expect(page.getByTestId("claims-source-connect")).toHaveCount(0);
      const platform = page.locator("#claims-source-platform");
      await expect(platform).toBeVisible();
      await expect(platform.locator("option")).toHaveText([cc.sourcePlatformAuto, cc.sourcePlatform.facebook, cc.sourcePlatform.instagram]);
      await shot(page, "studio-with-page", locale, view);

      // Disconnect: cancel changes nothing; confirm destroys the connection, routes and intake; Studio points back at Settings.
      await openSettings(page, locale);
      await page.getByTestId("metaconnect-disconnect").click();
      await expect(page.getByTestId("metaconnect-confirm")).toContainText(c.confirmDisconnect);
      await page.getByRole("button", { name: c.cancel }).click();
      await expect(page.getByTestId("metaconnect-status")).toBeVisible();
      await page.getByTestId("metaconnect-disconnect").click();
      await page.getByTestId("metaconnect-confirm-yes").click();
      await expect(page.getByTestId("metaconnect-none")).toBeVisible({ timeout: 30_000 });
      await expect(page.getByTestId("metaconnect-notice")).toHaveText(c.disconnectedNotice);
      const gone = await facts();
      expect(gone).toMatchObject({ routes: 0, heads: 0, connections: 0 });
      await noSecrets(page);
      await shot(page, "card-disconnected", locale, view);
      await openStudio(page, locale);
      await expect(page.getByTestId("claims-source-connect")).toContainText(c.studioNone);
      await expect(page.locator("#claims-source-platform")).toHaveCount(0);
    });
  }

  test("the Studio comment source can be bound to the connected Page with a private reply (the sealed v2 token counts as the Page token)", async ({ page }) => {
    const c = metaConnectCopy.en;
    const cc = claimsCopy.en;
    await signedLogin(page);
    await openSettings(page, "en");
    await answerDialog(page);
    answer = { profile: "full" };
    await page.getByTestId("metaconnect-connect").click();
    await expect(page.getByTestId("metaconnect-pick")).toBeVisible({ timeout: 30_000 });
    await page.getByTestId(`metaconnect-pick-${pageA.id}`).check();
    await page.getByTestId("metaconnect-pick-submit").click();
    await expect(page.getByTestId("metaconnect-status")).toBeVisible({ timeout: 30_000 });
    await openStudio(page, "en");
    const section = page.getByTestId("claims-source");
    await section.getByLabel(cc.sourceInput, { exact: true }).fill(`${pageA.id}_123456789012345`);
    await section.locator("#claims-source-platform").selectOption("facebook");
    await section.locator("#claims-source-reply").check();
    await section.getByRole("button", { name: cc.sourceSave }).click();
    await expect(section.getByTestId("claims-source-object")).toHaveText(`${pageA.id}_123456789012345`, { timeout: 30_000 });
    await expect(section.getByRole("alert")).toHaveCount(0);
    // leave the store clean for the next test
    await openSettings(page, "en");
    await page.getByTestId("metaconnect-disconnect").click();
    await page.getByTestId("metaconnect-confirm-yes").click();
    await expect(page.getByTestId("metaconnect-none")).toBeVisible({ timeout: 30_000 });
    expect(c.title).toBeTruthy();
  });

  test("zh-TW mobile refusals: forged state, missing permission and missing Page task are explained in the card and nothing is connected", async ({ page }) => {
    const c = metaConnectCopy["zh-TW"];
    await page.setViewportSize(sizes.mobile);
    await signedLogin(page);
    await openSettings(page, "zh-TW");
    await answerDialog(page);

    answer = { wrongState: true };
    await page.getByTestId("metaconnect-connect").click();
    await expect(page.getByTestId("metaconnect-error")).toHaveText(c.errors.state_mismatch, { timeout: 30_000 });
    await expect(page.getByTestId("metaconnect-none")).toBeVisible();
    expect(new URL(page.url()).search).not.toMatch(/meta_error|code=|state=/);
    await shot(page, "refused-state", "zh-TW", "mobile");

    answer = { profile: "nomsg" }; // pages_messaging not granted
    await page.getByTestId("metaconnect-connect").click();
    await expect(page.getByTestId("metaconnect-pick")).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId("metaconnect-missing")).toContainText("pages_messaging");
    await expect(page.getByTestId("metaconnect-pick-submit")).toBeDisabled();
    await shot(page, "refused-permission", "zh-TW", "mobile");
    await page.getByRole("button", { name: c.pickCancel }).click();
    await expect(page.getByTestId("metaconnect-none")).toBeVisible();

    answer = { profile: "notask" }; // Page without the MODERATE task
    await page.getByTestId("metaconnect-connect").click();
    await expect(page.getByTestId("metaconnect-pick")).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId(`metaconnect-pick-${pageD.id}`)).toBeChecked();
    await expect(page.getByTestId("metaconnect-missing")).toContainText(c.task_moderate);
    await expect(page.getByTestId("metaconnect-pick-submit")).toBeDisabled();
    await shot(page, "refused-task", "zh-TW", "mobile");
    await page.getByRole("button", { name: c.pickCancel }).click();

    const none = await facts();
    expect(none).toMatchObject({ bindings: 0, routes: 0, heads: 0, connections: 0 });
    expect(none.subscribed).toEqual({ A: false, B: false, C: false, D: false });
    await noSecrets(page);
    // and Page C (refused above) / Page D left no trace in the Studio picker either
    await openStudio(page, "zh-TW");
    await expect(page.getByTestId("claims-source-connect")).toBeVisible();
    expect(pageC.id && pageC.name).toBeTruthy();
  });

  test("multi-page: A + B are listed and selectable; lost pick/disconnect answers only read back; B disconnect leaves A usable", async ({ page }) => {
    await signedLogin(page);
    await openSettings(page, "en");
    await answerDialog(page);
    answer = { profile: "full" };
    const writes: { path: string; key: string; body: string }[] = [];
    page.on("request", (request) => {
      const pathname = new URL(request.url()).pathname;
      if (request.method() === "POST" && (pathname === "/api/meta/connect" || /\/meta-connect\/(pick|disconnect)$/.test(pathname))) {
        writes.push({ path: pathname, key: request.headers()["idempotency-key"] ?? "", body: request.postData() ?? "" });
      }
    });
    await page.getByTestId("metaconnect-connect").click();
    await page.getByTestId(`metaconnect-pick-${pageA.id}`).check();
    await page.getByTestId("metaconnect-pick-submit").click();
    await expect(page.getByTestId(`metaconnect-row-${pageA.id}`)).toBeVisible();
    await page.getByTestId("metaconnect-add").click();
    await page.getByTestId(`metaconnect-pick-${pageB.id}`).check();
    // The write lands, but the browser loses its response. No duplicate POST is permitted.
    let picks = 0;
    await page.route(`**/meta-connect/pick`, async (route) => {
      picks++;
      const response = await route.fetch();
      expect(response.status()).toBe(201);
      await route.abort("failed");
    });
    await page.getByTestId("metaconnect-pick-submit").click();
    await expect(page.getByTestId(`metaconnect-row-${pageB.id}`)).toBeVisible();
    await expect(page.getByTestId("metaconnect-pick")).toHaveCount(0);
    expect(picks).toBe(1);
    await page.unroute(`**/meta-connect/pick`);
    // Use the browser's secure-cookie session, not APIRequestContext's HTTP cookie policy.
    const status = await page.evaluate(async (store) => {
      const response = await fetch(`/api/stores/${store}/meta-connect/status`, { credentials: "same-origin" });
      return { code: response.status, dto: await response.json() };
    }, store);
    expect(status.code).toBe(200);
    const dto = status.dto;
    expect(dto).toMatchObject({ connected: true, count: 2, cap: 10 });
    expect(dto.pages.map((p: { id: string }) => p.id).sort()).toEqual([pageA.id, pageB.id].sort());
    for (const locale of ["zh-TW", "zh-CN", "en"] as const) for (const view of ["desktop", "mobile"] as const) {
      const c = metaConnectCopy[locale];
      await page.setViewportSize(sizes[view]);
      await openSettings(page, locale);
      await expect(page.getByTestId("metaconnect-status")).toHaveCount(2);
      await expect(page.getByTestId("metaconnect-count")).toHaveText(c.count.replace("{count}", "2").replace("{cap}", "10"));
      await expect(page.getByTestId("metaconnect-card")).toContainText(c.timezone);
      await page.getByTestId("metaconnect-card").scrollIntoViewIfNeeded();
      await shot(page, "multi-pages", locale, view);
      await page.getByTestId(`metaconnect-row-${pageB.id}`).scrollIntoViewIfNeeded();
      await shot(page, "multi-page-b", locale, view);
      const touch = await page.getByTestId("metaconnect-card").locator("button").evaluateAll((buttons) => buttons.every((b) => b.getBoundingClientRect().height >= 44));
      expect(touch).toBe(true);
      await openStudio(page, locale);
      const selector = page.locator("#claims-source-page");
      await expect(selector.locator("optgroup")).toHaveCount(2);
      await expect(selector.locator(`option[value="${pageA.id}"]`)).toContainText(pageA.name);
      await expect(selector.locator(`option[value="${pageB.id}"]`)).toContainText(pageB.name);
      await selector.selectOption(pageB.id);
      await selector.scrollIntoViewIfNeeded();
      await shot(page, "multi-studio", locale, view);
    }
    await openStudio(page, "en");
    const section = page.getByTestId("claims-source");
    await page.locator("#claims-source-page").selectOption(pageB.id);
    await page.locator("#claims-source-input").fill(`${pageA.id}_123456789012345`);
    await section.getByRole("button", { name: claimsCopy.en.sourceSave }).click();
    await expect(section.getByRole("alert")).toContainText(metaConnectCopy.en.studioPageMismatch);
    // A real bare ID is qualified by the selected Page, not by whichever binding was first.
    await page.locator("#claims-source-input").fill("223456789012345");
    await section.getByRole("button", { name: claimsCopy.en.sourceSave }).click();
    await expect(page.getByTestId("claims-source-object")).toHaveText(`${pageB.id}_223456789012345`);
    await openSettings(page, "en");
    const rowB = page.getByTestId(`metaconnect-row-${pageB.id}`);
    await rowB.getByTestId("metaconnect-disconnect").click();
    await expect(rowB.getByTestId("metaconnect-confirm")).toContainText(metaConnectCopy.en.confirmDisconnect);
    let disconnects = 0;
    await page.route(`**/meta-connect/disconnect`, async (route) => {
      disconnects++;
      expect(route.request().postDataJSON()).toEqual({ page_id: pageB.id });
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      await route.abort("failed");
    });
    await rowB.getByTestId("metaconnect-confirm-yes").click();
    await expect(rowB).toHaveCount(0);
    await expect(page.getByTestId(`metaconnect-row-${pageA.id}`)).toBeVisible();
    await expect(page.getByTestId("metaconnect-error")).toHaveCount(0);
    expect(disconnects).toBe(1);
    await page.unroute(`**/meta-connect/disconnect`);
    expect((await facts()).connections).toBe(1);
    await openStudio(page, "en");
    await expect(page.locator("#claims-source-page optgroup")).toHaveCount(1);
    await page.locator("#claims-source-page").selectOption(pageA.id);
    await page.locator("#claims-source-input").fill("323456789012345");
    await page.getByTestId("claims-source").getByRole("button", { name: claimsCopy.en.sourceSave }).click();
    await expect(page.getByTestId("claims-source-object")).toHaveText(`${pageA.id}_323456789012345`);
    await openSettings(page, "en");
    await page.getByTestId("metaconnect-disconnect").click();
    await page.getByTestId("metaconnect-confirm-yes").click();
    await expect(page.getByTestId("metaconnect-none")).toBeVisible();
    expect((await facts()).connections).toBe(0);
    expect(writes.every((w) => w.key.length >= 8)).toBe(true);
    expect(new Set(writes.map((w) => w.key)).size).toBe(writes.length);
    await noSecrets(page);
  });

  test("MOCK UI boundaries: ten slots, cap/not-found in three languages, malformed start preserves its key", async ({ page }) => {
    await signedLogin(page);
    const stamp = "2026-10-01T08:30:00Z";
    const dto = { connected: true, count: 10, cap: 10, pages: Array.from({ length: 10 }, (_, i) => ({
      id: String(900000 + i), name: `Fixture Page ${i + 1}`, status: "active", instagram: null, permissions: ["pages_messaging"],
      connected_at: stamp, route_expires_at: "2027-10-01T08:30:00Z", last_event_at: stamp,
    })) };
    await page.route(`**/meta-connect/status`, (route) => route.fulfill({ json: dto }));
    await page.route(`**/meta-connect/disconnect`, (route) => route.fulfill({ status: 404, json: { code: "not_found" } }));
    await page.route(`**/meta-connect/pick`, (route) => route.fulfill({ status: 409, json: { code: "cap_exceeded" } }));
    await answerDialog(page);
    answer = { profile: "full" };
    for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
      const c = metaConnectCopy[locale];
      await page.setViewportSize(sizes.mobile);
      await openSettings(page, locale);
      await expect(page.getByTestId("metaconnect-add")).toBeDisabled();
      await expect(page.locator("#metaconnect-cap")).toHaveText(c.capReached);
      await expect(page.getByTestId("metaconnect-reconnect")).toBeEnabled();
      await expect(page.getByTestId("metaconnect-last-event").first()).toContainText("16:30");
      await page.getByTestId("metaconnect-card").scrollIntoViewIfNeeded();
      await shot(page, "MOCK-cap-ten", locale, "mobile");
      const first = page.getByTestId("metaconnect-row-900000");
      await first.getByTestId("metaconnect-disconnect").click();
      await first.getByTestId("metaconnect-confirm-yes").click();
      await expect(page.getByTestId("metaconnect-error")).toHaveText(c.errors.not_found);
      await page.getByTestId("metaconnect-reconnect").click();
      await page.getByTestId(`metaconnect-pick-${pageA.id}`).check();
      await page.getByTestId("metaconnect-pick-submit").click();
      await expect(page.getByTestId("metaconnect-error")).toHaveText(c.errors.cap_exceeded);
    }
    await page.unroute(`**/meta-connect/status`);
    await page.unroute(`**/meta-connect/pick`);
    await page.unroute(`**/meta-connect/disconnect`);
    await openSettings(page, "en");
    const starts: string[] = [];
    await page.route("**/api/meta/connect", async (route) => {
      starts.push(route.request().headers()["idempotency-key"]);
      if (starts.length === 1) await route.fulfill({ status: 200, contentType: "application/json", body: "{" });
      else await route.continue();
    });
    await page.getByTestId("metaconnect-connect").click();
    await expect(page.getByTestId("metaconnect-error")).toBeVisible();
    await page.getByTestId("metaconnect-connect").click();
    await expect(page.getByTestId("metaconnect-pick")).toBeVisible();
    expect(starts).toHaveLength(2);
    expect(starts[0]).toBeTruthy();
    expect(starts[0]).toBe(starts[1]);
    await page.getByRole("button", { name: metaConnectCopy.en.pickCancel }).click();
    await expect(page.getByTestId("metaconnect-none")).toBeVisible();
  });

  test("state replay: the same OAuth return opened again connects nothing, even with the binding cookie present", async ({ page }) => {
    const c = metaConnectCopy.en;
    await signedLogin(page);
    await openSettings(page, "en");
    await answerDialog(page);
    answer = { profile: "fbonly" };
    await page.getByTestId("metaconnect-connect").click();
    await expect(page.getByTestId("metaconnect-pick")).toBeVisible({ timeout: 30_000 });
    const replay = lastCallback!;
    expect(replay).toContain("/api/meta/callback?code=");
    await page.getByRole("button", { name: c.pickCancel }).click(); // abandon: nothing connected yet
    await expect(page.getByTestId("metaconnect-none")).toBeVisible();
    // Re-open the identical return URL (as a back button / history entry / copied link would) with and without the cookie.
    for (const withCookie of [false, true]) {
      await page.context().clearCookies({ name: "lc_meta_connect" });
      if (withCookie) await page.context().addCookies([{ name: "lc_meta_connect", value: store, url: `${origin}/api/meta/callback` }]);
      await page.goto(replay);
      await page.waitForURL(/\/settings/);
      await expect(page.getByTestId("metaconnect-error")).toHaveText(c.errors.state_mismatch, { timeout: 30_000 });
      await expect(page.getByTestId("metaconnect-none")).toBeVisible();
      expect(page.url()).not.toMatch(/code=|state=/);
    }
    expect((await facts()).connections).toBe(0);
    await page.getByTestId("metaconnect-connect").waitFor();
  });

  // Root cause of the zh-TW mobile notice that came back in English: the dashboard's <Link> prefetches of /en/... answered a few ms after the
  // navigation to /zh-TW/settings and their Set-Cookie flipped commerce_locale back to en; /api/meta/callback reads that cookie.
  test("a Next <Link> prefetch never rewrites the locale preference; a real navigation does", async ({ page }) => {
    await signedLogin(page);
    const locale = async (path: string, headers: Record<string, string>) => {
      const response = await page.request.get(`${origin}${path}`, { headers, maxRedirects: 0 });
      expect(response.status(), path).toBe(200);
      return response.headersArray().filter((h) => h.name.toLowerCase() === "set-cookie" && h.value.startsWith("commerce_locale=")).map((h) => h.value.split(";")[0]);
    };
    expect(await locale(`/zh-TW/settings?store=${store}`, { "next-router-prefetch": "1" })).toEqual([]);
    expect(await locale(`/zh-TW/settings?store=${store}`, { "sec-purpose": "prefetch" })).toEqual([]);
    expect(await locale(`/zh-TW/settings?store=${store}`, {})).toEqual(["commerce_locale=zh-TW"]);
    expect(await locale(`/en/settings?store=${store}`, {})).toEqual(["commerce_locale=en"]);
  });

  test("no open redirect: whatever the cookies or query say, /api/meta/callback only ever redirects to /<locale>/settings", async ({ page }) => {
    await signedLogin(page);
    const state = "A".repeat(43);
    const hostile = ["//evil.example", "https://evil.example", "/\\evil.example", "javascript:alert(1)", "evil.example"];
    const cases: { name: string; cookies: { name: string; value: string }[]; query: string }[] = [
      { name: "no binding cookie", cookies: [], query: `code=abc&state=${state}` },
      { name: "extra redirect params", cookies: [{ name: "lc_meta_connect", value: store }], query: `code=abc&state=${state}&redirect_uri=https://evil.example&next=//evil.example&return_to=https://evil.example` },
      { name: "Meta error", cookies: [{ name: "lc_meta_connect", value: store }], query: "error=access_denied&error_reason=user_denied" },
      { name: "no code", cookies: [{ name: "lc_meta_connect", value: store }], query: `state=${state}` },
      ...hostile.flatMap((h) => [
        { name: `hostile store cookie ${h}`, cookies: [{ name: "lc_meta_connect", value: h }], query: `code=abc&state=${state}` },
        { name: `hostile locale cookie ${h}`, cookies: [{ name: "lc_meta_connect", value: store }, { name: "commerce_locale", value: h }], query: `code=abc&state=${state}` },
      ]),
    ];
    for (const item of cases) {
      await page.context().clearCookies({ name: "lc_meta_connect" });
      for (const cookie of item.cookies) {
        try {
          await page.context().addCookies([{ name: cookie.name, value: cookie.value, url: cookie.name === "lc_meta_connect" ? `${origin}/api/meta/callback` : origin }]);
        } catch {
          /* the browser refused to store a malformed cookie value: that is itself a safe outcome */
        }
      }
      const response = await page.request.get(`${origin}/api/meta/callback?${item.query}`, { maxRedirects: 0 });
      expect(response.status(), item.name).toBe(303);
      const location = response.headers()["location"] ?? "";
      expect(location, `${item.name}: ${location}`).toMatch(safeRedirect);
      expect(location).not.toMatch(/evil|code=|state=|^\/\//);
      expect(response.headers()["referrer-policy"], item.name).toBe("no-referrer");
      expect(response.headers()["cache-control"] ?? "", item.name).toMatch(/no-store/);
    }
    await page.context().clearCookies({ name: "commerce_locale" });
    // The start BFF only ever answers a Facebook dialog URL and refuses a query string or a foreign store.
    const bad = await page.request.post(`${origin}/api/meta/connect?next=https://evil.example`, { data: { store }, headers: { "idempotency-key": "gate-key-1234567890" } });
    expect(bad.status()).toBeGreaterThanOrEqual(400);
    expect(bad.headers()["location"] ?? "").toBe("");
  });
});
