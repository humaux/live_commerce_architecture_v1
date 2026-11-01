// Purpose: Exercise real admin ads workflows and their frozen failure paths.
// Depends on: Playwright, the Go browser harness, ads-copy and shared Taipei input helpers.
// Used by: TestBrowserMetaAds and the --browser-meta-ads CI gate.
// MA09a admin half (contracts/meta-ads-v1.md §9 MA09, §2 flow, §5.1 statuses, §5.3 X7, §7, §12; ads-ui U1-U8; rulings O4, AD9).
// BFF routes exercised through the UI: POST /api/ads/meta/connect, GET /api/ads/meta/callback (the FLfB redirect URI),
// GET|POST /api/stores/{store}/ads/{settings,meta/states/*,meta/bindings,drafts,drafts/*,report,capi} -> Go
// /v1/admin/stores/{store_id}/ads/*. Started only by tests/foundation/browser_meta_ads_test.go (build tag browser), which owns the
// isolated PG, the real Go API + ads worker (dispatcher, sweepers), the fake Graph, the production admin Next build, the signed mock
// IdP and the runner-only control listener. Labels: BROWSER; Meta is MOCK: https://www.facebook.com/... is answered by page.route
// with a 302 to the app's own callback, the code exchange happens in the API process against the fake Graph.
// Locators use the UI unit's data-testid vocabulary and its copy file; every assertion restates the contract or a frozen UI default.
import { expect, test, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile, mkdir, appendFile } from "node:fs/promises";
import path from "node:path";
import { growthCopy } from "../../packages/i18n/src/growth.ts";
import { adsCopy } from "../../apps/admin/lib/ads-copy";
import { displayTime, instantToTaipei, taipeiToInstant } from "../../packages/format/src/index.ts";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_STORE");
const account = required("LC_BROWSER_ACCOUNT");
const pageAsset = required("LC_BROWSER_PAGE_ASSET");
const business = required("LC_BROWSER_CLIENT_BUSINESS");
const control = required("LC_BROWSER_CONTROL");
const controlKey = required("LC_BROWSER_CONTROL_KEY");
const appId = required("LC_BROWSER_APP_ID");
const configId = required("LC_BROWSER_CONFIG_ID");
const redirect = required("LC_BROWSER_REDIRECT");
const graphVersion = required("LC_BROWSER_GRAPH_VERSION");
const secrets: string[] = JSON.parse(required("LC_BROWSER_SECRETS"));
const refusalPhase = process.env.LC_BROWSER_PHASE === "original-refusal";

test.use({ baseURL: origin, timezoneId: "America/Los_Angeles", trace: "retain-on-failure", screenshot: "only-on-failure" });

const en = adsCopy.en;
const uuidRe = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
let firstDraft = "";
let copyDraft = "";

async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  // W0 shell: navigation is the permission-filtered registry; this fixture role holds ads permissions only, so its one group is Marketing (not Orders).
  await expect(page.getByTestId("nav-group-marketing")).toBeVisible();
  // A role without Overview (orders:read) lands on its first permitted page, never on a 403 whose way back loops.
  await expect(page).toHaveURL(/\/en\/ads(\?|$)/);
  await expect(page.getByTestId("route-forbidden")).toHaveCount(0);
}
async function ctl(resource: string) {
  const response = await fetch(`${control}/${resource}`, { method: "POST", headers: { "X-Gate-Key": controlKey } });
  expect([200, 204], `control ${resource}`).toContain(response.status);
  return response;
}
async function graph() {
  const response = await fetch(`${control}/graph`, { method: "POST", headers: { "X-Gate-Key": controlKey } });
  expect(response.status).toBe(200);
  return (await response.json()) as {
    violations: string[]; oauth_exchanges: number; status_posts: number; token_sources: string[]; adset_bodies: string[];
    campaign: { ID: string; Name: string; Status: string; Effective: string }[];
    adset: { ID: string; Status: string; Effective: string }[];
    creative: { ID: string }[]; ad: { ID: string; Status: string; Effective: string }[];
  };
}
async function fitsWidth(page: Page) {
  // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
}
const manifestPath = path.join(evidence, "screenshots.json");
async function shot(page: Page, name: string, locale: string, viewport: "desktop" | "mobile") {
  const file = path.join(evidence, `${name}-${locale}-${viewport}.png`);
  await page.screenshot({ path: file, fullPage: false, animations: "disabled" });
  await fitsWidth(page);
  let manifest: unknown[] = [];
  try {
    manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  } catch {
    /* first screenshot */
  }
  manifest.push({ File: path.basename(file), Sha256: createHash("sha256").update(await readFile(file)).digest("hex"), Locale: locale, Viewport: viewport });
  await writeFile(manifestPath, JSON.stringify(manifest, null, 2));
}
async function noSecrets(page: Page) {
  const html = await page.content();
  // G-UI8 audit [READ/MEASURE]: scans client storage for secrets/PII (read only)
  const stored = await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage }, cookie: document.cookie }));
  for (const secret of secrets) {
    expect(html).not.toContain(secret);
    expect(stored).not.toContain(secret);
  }
}
const localInput = (offsetMs: number) => {
  return instantToTaipei(new Date(Date.now() + offsetMs).toISOString());
};
async function openAds(page: Page, locale = "en") {
  await page.goto(`/${locale}/ads?store=${store}`);
  await expect(page.getByTestId("merchant-ads")).toBeVisible();
  await expect(page.getByTestId("ads-new-draft")).toBeVisible();
}
const growth = (page: Page) => growthCopy[new URL(page.url()).pathname.split("/")[1] as "en" | "zh-TW" | "zh-CN"];
async function sourceTask(page: Page) {
  await page.getByRole("button", { name: growth(page).source, exact: true }).click();
  await page.getByTestId("ads-source-advanced").locator(":scope > summary").click();
}
async function settingsTask(page: Page) { await page.getByRole("button", { name: growth(page).settings, exact: true }).click(); }
async function applyTask(page: Page) { await page.getByRole("button", { name: growth(page).apply, exact: true }).click(); }
async function listTask(page: Page) { await page.getByRole("button", { name: growth(page).goList, exact: true }).click(); }
async function cancelEdit(page: Page) {
  await page.getByTestId("ads-f-cancel").click(); // cancel settings
  await page.getByTestId("ads-f-cancel").click(); // leave editor
  await page.getByRole("button", { name: growth(page).discard, exact: true }).click();
}

async function auditDraft(page: Page) {
  const details = page.getByTestId("ads-advanced");
  await expect(details).toBeVisible();
  if (await details.getAttribute("open") === null) await details.locator(":scope > summary").click();
}

// Matches only nodes inside the detail screen that are NOT descendants of the Advanced disclosure.
function outsideAdvanced(page: Page) {
  return page.locator('xpath=//*[@data-testid="ads-detail"]//*[not(ancestor-or-self::*[@data-testid="ads-advanced"])]');
}

async function driveUntil(page: Page, status: string) {
  await expect(async () => {
    await ctl("drive");
    await page.getByTestId("ads-refresh").click();
    await auditDraft(page);
    await expect(page.getByTestId("ads-detail")).toHaveAttribute("data-status", status, { timeout: 2500 });
  }).toPass({ timeout: 90_000, intervals: [500, 1000, 2000] });
}
const bodies: { url: string; status: number; text: string }[] = [];
function recordBodies(page: Page) {
  page.on("response", async (response) => {
    try {
      const url = new URL(response.url());
      if (url.origin !== origin) return;
      const type = response.headers()["content-type"] ?? "";
      if (!/json|text|html/.test(type)) return;
      bodies.push({ url: url.pathname, status: response.status(), text: (await response.text()).slice(0, 200_000) });
    } catch {
      /* redirects and aborted responses have no body */
    }
  });
}

if (!refusalPhase) {
test.describe("MA09a flow", () => {
test.describe.configure({ mode: "serial" });

test("MA09a connect through the fake FLfB dialog: dialog params, state cookie, 303 without code/state, pick list, bind, sealed token never in the browser", async ({ page }) => {
  recordBodies(page);
  await signedLogin(page);
  await openAds(page);
  await page.getByRole("button", { name: growth(page).assets, exact: true }).click();
  // W0 registry: the fixture role holds ads:* only, so Marketing is a one-route group whose rail button is the current page (aria-current).
  await expect(page.getByTestId("nav-group-marketing")).toHaveAttribute("aria-current", "page");
  await expect(page.getByTestId("ads-sandbox")).toHaveText(en.sandboxBanner); // AD9: the store is SANDBOX
  await expect(page.getByTestId("ads-conn-empty")).toHaveText(en.connEmpty);
  await expect(page.getByTestId("ads-budget-note")).toHaveText(en.budgetNote); // §12: never a real-time hard stop
  await expect(page.getByTestId("ads-identities")).toContainText(pageAsset); // the Facebook Page identity of the store
  await noSecrets(page);

  let dialog: URL | null = null;
  let connectSetCookie: string[] = [];
  const callbackResponses: { status: number; headers: Record<string, string> }[] = [];
  page.on("response", async (response) => {
    const url = new URL(response.url());
    if (url.pathname === "/api/ads/meta/connect" && response.request().method() === "POST") connectSetCookie = (await response.headersArray()).filter((h) => h.name.toLowerCase() === "set-cookie").map((h) => h.value);
    if (url.pathname === "/api/ads/meta/callback") callbackResponses.push({ status: response.status(), headers: response.headers() });
  });
  await page.route("https://www.facebook.com/**", async (route) => {
    const url = new URL(route.request().url());
    dialog = url;
    const { code } = (await (await ctl("oauth/code")).json()) as { code: string };
    await route.fulfill({ status: 302, headers: { Location: `${origin}/api/ads/meta/callback?code=${encodeURIComponent(code)}&state=${encodeURIComponent(url.searchParams.get("state") ?? "")}` } });
  });
  await page.getByTestId("ads-connect").click();
  await expect(page.getByTestId("ads-pick")).toBeVisible({ timeout: 30_000 });

  // §2 step 1: FLfB dialog on app 大梦 with config_id, response_type=code, override_default_response_type, the fixed redirect, a random state
  expect(dialog).not.toBeNull();
  const d = dialog as unknown as URL;
  expect(d.origin).toBe("https://www.facebook.com");
  expect(d.pathname).toBe(`/${graphVersion}/dialog/oauth`);
  expect(d.searchParams.get("client_id")).toBe(appId);
  expect(d.searchParams.get("config_id")).toBe(configId);
  expect(d.searchParams.get("response_type")).toBe("code");
  expect(d.searchParams.get("override_default_response_type")).toBe("true");
  expect(d.searchParams.get("redirect_uri")).toBe(redirect);
  expect(d.searchParams.get("state") ?? "").toMatch(/^[A-Za-z0-9_-]{43}$/);
  expect(d.searchParams.has("scope")).toBe(false); // FLfB: config_id replaces scope
  expect(d.searchParams.has("client_secret")).toBe(false);
  // U1: the binding cookie is httpOnly, Lax, scoped to the callback path, short-lived, and holds the store only
  const cookieLine = connectSetCookie.find((c) => c.startsWith("lc_ads_connect="));
  expect(cookieLine, "lc_ads_connect Set-Cookie on the connect response").toBeTruthy();
  expect(cookieLine!).toContain(`lc_ads_connect=${store}`);
  expect(cookieLine!).toMatch(/HttpOnly/i);
  expect(cookieLine!).toMatch(/SameSite=Lax/i);
  expect(cookieLine!).toMatch(/Path=\/api\/ads\/meta\/callback/i);
  expect(cookieLine!).toMatch(/Max-Age=600/i);
  // U2: the callback answers 303, no-store, no-referrer, clears the cookie, and never echoes code/state into the target
  const cb = callbackResponses.find((r) => r.status === 303);
  expect(cb, "callback 303").toBeTruthy();
  expect(cb!.headers["location"]).toMatch(new RegExp(`^/en/ads\\?store=${store}&connect=[0-9a-f-]{36}$|^/[a-zA-Z-]+/ads\\?store=${store}&connect=[0-9a-f-]{36}$`));
  expect(cb!.headers["location"]).not.toMatch(/code=|state=/);
  expect(cb!.headers["cache-control"]).toMatch(/no-store/);
  expect(cb!.headers["referrer-policy"]).toBe("no-referrer");
  expect(await page.context().cookies()).not.toEqual(expect.arrayContaining([expect.objectContaining({ name: "lc_ads_connect", value: store })]));
  const url = new URL(page.url());
  expect(url.searchParams.get("connect") ?? "").toMatch(uuidRe);
  expect(url.search).not.toMatch(/code=|state=SYNTH|&state=/);

  // U3: only accounts of THIS login are offered; a POST that names an account outside the pick list is refused (nothing is bound)
  await expect(page.locator('[data-testid^="ads-pick-"][type="radio"]')).toHaveCount(1);
  await expect(page.getByTestId(`ads-pick-${account}`)).toBeVisible();
  await page.getByTestId(`ads-pick-${account}`).check();
  const foreign = "1" + "9".repeat(11);
  let refused = 0;
  await page.route("**/api/stores/*/ads/meta/bindings", async (route) => {
    const body = JSON.parse(route.request().postData() ?? "{}");
    await route.continue({ postData: JSON.stringify({ ...body, ad_account_id: foreign }) });
  });
  page.once("response", (r) => { if (/\/ads\/meta\/bindings$/.test(new URL(r.url()).pathname)) refused = r.status(); });
  await page.getByTestId("ads-pick-submit").click();
  await expect(page.getByTestId("ads-banner")).toBeVisible();
  expect(refused).toBe(422);
  await page.unroute("**/api/stores/*/ads/meta/bindings");
  // the real pick: exactly {state_id, ad_account_id}, one Idempotency-Key
  const binds: { key: string | null; body: string | null; status: number }[] = [];
  page.on("response", (r) => {
    if (r.request().method() === "POST" && /\/ads\/meta\/bindings$/.test(new URL(r.url()).pathname))
      binds.push({ key: r.request().headers()["idempotency-key"] ?? null, body: r.request().postData(), status: r.status() });
  });
  await page.getByTestId("ads-pick-submit").click();
  await expect(page.getByTestId("ads-banner")).toHaveText(en.connected);
  expect(binds).toHaveLength(1);
  expect(binds[0].status).toBe(201);
  expect(binds[0].key).toMatch(/^[A-Za-z0-9_.:-]{8,128}$/);
  expect(Object.keys(JSON.parse(binds[0].body ?? "{}")).sort()).toEqual(["ad_account_id", "state_id"]);
  await expect(page.getByTestId("ads-connections")).toContainText(account);
  await expect(page.getByTestId("ads-connections")).toContainText(business);
  await expect(page.getByTestId("ads-connect")).toBeEnabled();
  expect(new URL(page.url()).searchParams.has("connect")).toBe(false);
  await noSecrets(page);
  for (const b of bodies) for (const s of secrets) expect(b.text, `${b.url} carries a Meta secret`).not.toContain(s);
  const g = await graph();
  expect(g.oauth_exchanges).toBe(1);
  expect(g.violations).toEqual([]);
});

test("MA09a state_mismatch, state_expired, meta_connect_failed and Start again: fixed codes, never a guess, the code is not exchanged on a mismatch", async ({ page }) => {
  await signedLogin(page);
  await openAds(page);
  await page.getByRole("button", { name: growth(page).assets, exact: true }).click();
  const before = (await graph()).oauth_exchanges;
  // (a) a return that did not start in this browser: no binding cookie -> state_mismatch, no store guessed
  await page.goto(`/api/ads/meta/callback?code=SYNTH-CODE-x&state=${"A".repeat(43)}`);
  await expect(page).toHaveURL(/\/(en|zh-TW|zh-CN)\/ads\?[^#]*connect_error=state_mismatch/);
  await expect(page.getByTestId("ads-connect-error")).toContainText(adsCopy[(new URL(page.url()).pathname.split("/")[1] as "en" | "zh-TW" | "zh-CN")].connectErrors.state_mismatch);
  expect((await graph()).oauth_exchanges).toBe(before);

  // (b) the return carries a tampered state: the state row does not match -> state_mismatch, still no exchange
  await openAds(page);
  await page.getByRole("button", { name: growth(page).assets, exact: true }).click();
  await page.route("https://www.facebook.com/**", async (route) => {
    const u = new URL(route.request().url());
    const state = u.searchParams.get("state") ?? "";
    const tampered = state.slice(0, -1) + (state.endsWith("A") ? "B" : "A");
    const { code } = (await (await ctl("oauth/code")).json()) as { code: string };
    await route.fulfill({ status: 302, headers: { Location: `${origin}/api/ads/meta/callback?code=${encodeURIComponent(code)}&state=${encodeURIComponent(tampered)}` } });
  });
  await page.getByTestId("ads-connect").click();
  await expect(page.getByTestId("ads-connect-error")).toContainText(en.connectErrors.state_mismatch, { timeout: 30_000 });
  expect((await graph()).oauth_exchanges).toBe(before);
  await page.unroute("https://www.facebook.com/**");

  // (c) the state expired while the merchant was on Meta's page -> state_expired
  await openAds(page);
  await page.getByRole("button", { name: growth(page).assets, exact: true }).click();
  await page.route("https://www.facebook.com/**", async (route) => {
    const u = new URL(route.request().url());
    await ctl("oauth/expire");
    const { code } = (await (await ctl("oauth/code")).json()) as { code: string };
    await route.fulfill({ status: 302, headers: { Location: `${origin}/api/ads/meta/callback?code=${encodeURIComponent(code)}&state=${encodeURIComponent(u.searchParams.get("state") ?? "")}` } });
  });
  await page.getByTestId("ads-connect").click();
  await expect(page.getByTestId("ads-connect-error")).toContainText(en.connectErrors.state_expired, { timeout: 30_000 });
  await page.unroute("https://www.facebook.com/**");

  // (d) Meta refuses the code (never issued) -> meta_connect_failed (502), and Start again opens a fresh dialog
  await openAds(page);
  await page.getByRole("button", { name: growth(page).assets, exact: true }).click();
  await page.route("https://www.facebook.com/**", async (route) => {
    const u = new URL(route.request().url());
    await route.fulfill({ status: 302, headers: { Location: `${origin}/api/ads/meta/callback?code=SYNTH-CODE-never-issued&state=${encodeURIComponent(u.searchParams.get("state") ?? "")}` } });
  });
  await page.getByTestId("ads-connect").click();
  await expect(page.getByTestId("ads-connect-error")).toContainText(en.connectErrors.meta_connect_failed, { timeout: 30_000 });
  let dialogs = 0;
  await page.unroute("https://www.facebook.com/**");
  await page.route("https://www.facebook.com/**", async (route) => {
    dialogs++;
    await route.abort();
  });
  await page.getByTestId("ads-connect-error").getByRole("button", { name: en.startAgain }).click();
  await expect.poll(() => dialogs).toBeGreaterThan(0);
  await page.unroute("https://www.facebook.com/**");
  const g = await graph();
  expect(g.violations).toEqual([]);
  expect(g.oauth_exchanges).toBe(before + 1); // only (d) reached Meta: the never-issued code was presented once and refused; (a) (b) (c) never exchanged
});

test("MA09a draft -> approve -> publish -> pause -> copy: statuses, one keyed request each, the campaign is only ever switched by activate and pause, X7 no resume", async ({ page }) => {
  await signedLogin(page);
  expect(await page.evaluate(() => Intl.DateTimeFormat().resolvedOptions().timeZone)).toBe("America/Los_Angeles");
  await openAds(page);
  const requests: { method: string; url: string; key: string | null; ifMatch: string | null; body: string | null; status: number }[] = [];
  page.on("response", (r) => {
    const u = new URL(r.url());
    if (r.request().method() !== "GET" && /\/api\/stores\/[^/]+\/ads\//.test(u.pathname))
      requests.push({ method: r.request().method(), url: u.pathname, key: r.request().headers()["idempotency-key"] ?? null, ifMatch: r.request().headers()["if-match"] ?? null, body: r.request().postData(), status: r.status() });
  });
  // new draft
  await page.getByTestId("ads-new-draft").click();
  await expect(page.getByTestId("ads-form")).toBeVisible();
  await sourceTask(page);
  await expect(page.getByTestId("ads-template-BOOST_POST")).toBeChecked();
  await page.getByTestId("ads-f-source").fill(`${pageAsset}_1234567890`);
  await page.getByRole("button", { name: growth(page).keepSource, exact: true }).click();
  await settingsTask(page);
  await expect(page.getByTestId("ads-f-countries")).toHaveValue(/TW/);
  await expect(page.getByTestId("ads-f-agemin")).toHaveValue("18");
  await expect(page.getByTestId("ads-f-agemax")).toHaveValue("65");
  await expect(page.getByTestId("ads-f-account")).toHaveValue(/[0-9a-f-]{36}/);
  await page.getByTestId("ads-f-budget").fill("3000");
  await expect(page.getByText(en.fDatesHint, { exact: true })).toBeVisible();
  const startsWall = localInput(3 * 3600_000);
  const endsWall = localInput(27 * 3600_000);
  await page.getByTestId("ads-f-starts").fill(startsWall);
  await page.getByTestId("ads-f-ends").fill(endsWall);
  const savedDraft = page.waitForResponse((r) => r.request().method() === "POST" && /\/ads\/drafts$/.test(new URL(r.url()).pathname));
  await applyTask(page);
  await page.getByTestId("ads-f-save").click();
  const savedResponse = await savedDraft;
  expect(savedResponse.status()).toBe(201);
  const persisted = await savedResponse.json() as { id: string; starts_at: string; ends_at: string };
  for (const [wall, instant] of [[startsWall, persisted.starts_at], [endsWall, persisted.ends_at]]) {
    expect(instantToTaipei(instant)).toBe(wall);
    expect(Date.parse(instant)).toBe(Date.parse(`${wall}:00+08:00`));
    expect(Date.parse(instant)).toBe(Date.parse(taipeiToInstant(wall)!));
  }
  await expect(page.getByTestId("ads-banner")).toHaveText(en.saved);
  await expect(page.getByTestId("ads-detail")).toHaveAttribute("data-status", "DRAFT");
  const created = requests.find((r) => r.method === "POST" && /\/ads\/drafts$/.test(r.url));
  expect(created?.status).toBe(201);
  expect(created?.key).toMatch(/^[A-Za-z0-9_.:-]{8,128}$/);
  const input = JSON.parse(created?.body ?? "{}");
  expect(input.lifetime_budget_minor).toBe(300000); // NT$3000 in minor units, integer math only
  expect(input.currency).toBe("TWD");
  expect(Object.keys(input).sort()).toEqual(["ad_binding_id", "age_max", "age_min", "countries", "currency", "ends_at", "identity_binding_id", "lifetime_budget_minor", "source_ref", "starts_at", "template"]);
  expect(Date.parse(input.starts_at)).toBe(Date.parse(`${startsWall}:00+08:00`));
  expect(Date.parse(input.ends_at)).toBe(Date.parse(`${endsWall}:00+08:00`));
  firstDraft = persisted.id;
  expect(firstDraft).toMatch(uuidRe);
  await expect(page.getByTestId("ads-detail")).toContainText("3,000");
  await expect(page.getByTestId("ads-detail")).toContainText(displayTime("en", persisted.starts_at));
  await expect(page.getByTestId("ads-detail")).toContainText(displayTime("en", persisted.ends_at));
  // Read the same persisted draft with the same signed session in a UTC context.
  const utcContext = await page.context().browser()!.newContext({ storageState: await page.context().storageState(), timezoneId: "UTC", ignoreHTTPSErrors: true });
  try {
    const utcPage = await utcContext.newPage();
    expect(await utcPage.evaluate(() => Intl.DateTimeFormat().resolvedOptions().timeZone)).toBe("UTC");
    await utcPage.goto(`${origin}/en/ads?store=${store}&draft=${firstDraft}`);
    await expect(utcPage.getByTestId("ads-detail")).toContainText(displayTime("en", persisted.starts_at));
    await expect(utcPage.getByTestId("ads-detail")).toContainText(displayTime("en", persisted.ends_at));
    await utcPage.reload();
    await expect(utcPage.getByTestId("ads-detail")).toContainText(displayTime("en", persisted.starts_at));
    await expect(utcPage.getByTestId("ads-detail")).toContainText(displayTime("en", persisted.ends_at));
  } finally {
    await utcContext.close();
  }
  await page.getByTestId("ads-edit").click();
  await settingsTask(page);
  await expect(page.getByText(en.fDatesHint, { exact: true })).toBeVisible();
  await expect(page.getByTestId("ads-f-starts")).toHaveValue(startsWall);
  await expect(page.getByTestId("ads-f-ends")).toHaveValue(endsWall);
  await cancelEdit(page);
  // Signed layout: return to the list and reopen the saved row, then reload the real route.
  await listTask(page);
  await page.getByTestId(`ads-open-${firstDraft}`).click();
  await expect(page).toHaveURL(new RegExp(`[?&]draft=${firstDraft}(?:&|$)`));
  await page.reload();
  await expect(page.getByTestId("ads-detail")).toContainText(displayTime("en", persisted.starts_at));
  await expect(page.getByTestId("ads-detail")).toContainText(displayTime("en", persisted.ends_at));
  await page.getByTestId("ads-edit").click();
  await settingsTask(page);
  await expect(page.getByText(en.fDatesHint, { exact: true })).toBeVisible();
  await expect(page.getByTestId("ads-f-starts")).toHaveValue(startsWall);
  await expect(page.getByTestId("ads-f-ends")).toHaveValue(endsWall);
  await cancelEdit(page);

  // approve
  // RVFIX2-366 P1: Approve is rendered directly on the review screen, not inside the Advanced disclosure
  await expect(outsideAdvanced(page).getByTestId("ads-approve")).toBeVisible();
  await expect(outsideAdvanced(page).getByTestId("ads-approve")).toBeEnabled();
  await auditDraft(page);
  await page.getByTestId("ads-approve").click();
  await expect(page.getByTestId("ads-banner")).toHaveText(en.approved);
  await expect(page.getByTestId("ads-detail")).toHaveAttribute("data-status", "APPROVED");
  const approval = requests.find((r) => /\/approve$/.test(r.url));
  expect(JSON.parse(approval?.body ?? "{}")).toEqual({ revision: 1 });
  // an approved draft is frozen: no Edit
  await expect(page.getByTestId("ads-edit")).toHaveCount(0);

  // publish, then the sweeper + dispatcher (control) walk the chain
  await page.getByTestId("ads-publish").click();
  await expect(page.getByTestId("ads-banner")).toHaveText(en.published);
  const publish = requests.find((r) => /\/publish$/.test(r.url));
  expect(publish?.status).toBe(200);
  expect(publish?.key).toMatch(/^[A-Za-z0-9_.:-]{8,128}$/);
  expect(JSON.parse(publish?.body ?? "{}")).toEqual({ publish_attempt: 0 });
  await driveUntil(page, "ACTIVE");
  // Preserve exact server-step coverage while checking seller-facing labels instead of raw codes.
  for (const kind of ["campaign", "adset", "creative", "ad", "preflight", "activate"] as const) {
    const step = page.getByTestId("ads-ops").locator(`[data-op-kind="${kind}"]`);
    await expect(step).toHaveText(growthCopy.en.opKinds[kind]);
    await expect(page.getByTestId("ads-ops").getByText(kind, { exact: true })).toHaveCount(0);
  }
  await expect(page.getByTestId("ads-ops").getByText(en.opStates.SUCCEEDED).first()).toBeVisible();
  const link = await page.getByTestId("ads-manager-link").getAttribute("href");
  expect(new URL(link!).protocol).toBe("https:");
  expect(link!).toContain(account);
  expect(await page.getByTestId("ads-manager-link").getAttribute("rel")).toContain("noopener");
  let g = await graph();
  expect(g.campaign).toHaveLength(1);
  expect(g.campaign[0].Status).toBe("ACTIVE");
  expect(g.adset[0].Effective).toBe("ACTIVE");
  expect(g.adset_bodies[0]).toContain('"lifetime_budget":3000'); // AD4 + F19: TWD offset 1
  expect(g.adset_bodies[0]).not.toContain("daily_budget");
  expect(g.status_posts).toBe(1); // AD3: the only spend switch
  await expect(page.getByTestId("ads-remote")).toContainText(g.campaign[0].ID);
  await expect(page.getByTestId("ads-remote")).toContainText(g.ad[0].ID);
  await expect(page.getByTestId("ads-pause")).toBeVisible();
  // RVFIX2-366 P1: Pause (the only spend-stopping control) is never inside the closed-by-default Advanced disclosure
  await expect(outsideAdvanced(page).getByTestId("ads-pause")).toBeVisible();
  await expect(page.getByTestId("ads-publish")).toHaveCount(0);
  await noSecrets(page);

  // insights are read while the draft is ACTIVE (it counts toward the allowance, so reads are planned hourly, D9)
  await ctl("seed/insights");
  await ctl("sweep/insights");

  // pause: always available; afterwards only "copy to a new draft", never a resume (X7)
  await page.getByTestId("ads-pause").click();
  await expect(page.getByTestId("ads-banner")).toHaveText(en.paused);
  await driveUntil(page, "PAUSED");
  g = await graph();
  expect(g.campaign[0].Status).toBe("PAUSED");
  expect(g.status_posts).toBe(2);
  // U5: Pause stays available while any remote id exists (a pause is always allowed, §5.3); what must appear is the copy action
  await expect(page.getByTestId("ads-copy")).toBeVisible();
  await expect(page.getByTestId("ads-actions").getByRole("button", { name: /resume|reactivate|re-activate|activate|重新啟用|恢復|恢复/i })).toHaveCount(0);
  await expect(page.getByTestId("ads-detail").locator(":scope > .growth-card [data-state]")).toHaveAttribute("data-state", "PAUSED");

  // copy: a NEW draft (new id, fresh approval needed); the paused one stays paused
  await page.getByTestId("ads-copy").click();
  await expect(page.getByTestId("ads-form")).toBeVisible();
  await sourceTask(page);
  await expect(page.getByTestId("ads-f-source")).toHaveValue(new RegExp(`^${pageAsset}_`));
  await page.getByRole("button", { name: growth(page).keepSource, exact: true }).click();
  await settingsTask(page);
  await expect(page.getByTestId("ads-f-budget")).toHaveValue("3000");
  await page.getByTestId("ads-f-starts").fill(localInput(4 * 3600_000));
  await page.getByTestId("ads-f-ends").fill(localInput(28 * 3600_000));
  await applyTask(page);
  await page.getByTestId("ads-f-save").click();
  await expect(page.getByTestId("ads-banner")).toHaveText(en.saved);
  await expect(page.getByTestId("ads-detail")).toHaveAttribute("data-status", "DRAFT");
  await listTask(page);
  const ids = await page.getByTestId(/^ads-draft-/).evaluateAll((els) => els.map((e) => e.getAttribute("data-testid")!.replace("ads-draft-", "")));
  expect(ids).toHaveLength(2);
  copyDraft = ids.find((id) => id !== firstDraft)!;
  expect(copyDraft).toMatch(uuidRe);
  await expect(page.getByTestId(`ads-draft-${firstDraft}`).locator("[data-state]")).toHaveAttribute("data-state", "PAUSED");
  for (const r of requests) expect(r.status, `${r.method} ${r.url}`).toBeLessThan(300);
  expect((await graph()).violations).toEqual([]);
});

test("MA09a allowance off (O4) disables approve with the copy; SANDBOX banner follows the environment (AD9)", async ({ page }) => {
  await signedLogin(page);
  await ctl("allowance/off");
  await openAds(page);
  await expect(page.getByTestId("ads-allowance-off")).toHaveText(en.allowanceOff);
  await expect(page.getByTestId("ads-allowance")).toHaveCount(0);
  await page.getByTestId(`ads-open-${copyDraft}`).click();
  await expect(page.getByTestId("ads-detail")).toHaveAttribute("data-status", "DRAFT");
  await auditDraft(page);
  await expect(page.getByTestId("ads-approve")).toBeDisabled();
  await expect(page.getByText(en.approveOffHint)).toBeVisible();
  await ctl("allowance/on");
  await page.getByTestId("ads-refresh").click();
  await expect(page.getByTestId("ads-allowance-off")).toHaveCount(0);
  await expect(page.getByTestId("ads-allowance")).toBeVisible();
  await expect(page.getByTestId("ads-sandbox")).toBeVisible();
  await ctl("env/live");
  await page.getByTestId("ads-refresh").click();
  await expect(page.getByTestId("ads-sandbox")).toHaveCount(0);
  await ctl("env/sandbox");
  await page.getByTestId("ads-refresh").click();
  await expect(page.getByTestId("ads-sandbox")).toBeVisible();
});

test("MA09a report: three separate blocks with window, timezone and fetched_at, no combined figure, budget is not a hard stop", async ({ page }) => {
  await signedLogin(page);
  await openAds(page);
  await page.getByTestId("ads-results-link").click();
  await page.getByTestId("ads-report-load").click();
  await expect(page.getByTestId("ads-block-orders")).toBeVisible();
  await expect(page.getByTestId("ads-block-delivery")).toBeVisible();
  await expect(page.getByTestId("ads-block-reported")).toBeVisible();
  await expect(page.getByTestId("ads-report-separate")).toHaveText(en.reportSeparate);
  for (const block of ["ads-block-orders", "ads-block-delivery", "ads-block-reported"]) {
    await expect(page.getByTestId(block).getByRole("heading")).toHaveCount(1);
  }
  await expect(page.getByTestId("ads-block-delivery")).toContainText(en.reportTimezone);
  await expect(page.getByTestId("ads-block-delivery")).toContainText(en.reportFetched);
  await expect(page.getByTestId("ads-block-reported")).toContainText(en.reportFetched);
  await expect(page.getByTestId("ads-block-delivery")).toContainText(/3,000/); // impressions 3 x 1000
  await expect(page.getByTestId("ads-block-reported")).toContainText(/6/); // purchases 3 x 2
  await expect(page.getByTestId("ads-block-delivery")).toContainText(/36[.,]9|37/); // spend 3 x 12.30
  expect(await page.getByText(/ROAS|return on ad spend/i).count()).toBe(0);
  await expect(page.getByTestId("ads-budget-note")).toHaveText(en.budgetNote);
  await noSecrets(page);
});

test("MA09a three locales, desktop 1440x992 and 390 px: no horizontal scroll, screenshots hashed", async ({ page }) => {
  await signedLogin(page);
  for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
    for (const viewport of [{ name: "desktop", width: 1440, height: 992 }, { name: "mobile", width: 390, height: 844 }] as const) {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await page.goto(`/${locale}/ads?store=${store}&draft=${firstDraft}`);
      await expect(page.getByRole("heading", { level: 1 })).toHaveText(growthCopy[locale].status);
      await expect(page.getByTestId("ads-detail")).toHaveAttribute("data-status", "PAUSED");
      await expect(page.getByTestId("ads-detail").locator(":scope > .growth-card [data-state]")).toHaveText(adsCopy[locale].statuses.PAUSED);
      await shot(page, "ads-detail", locale, viewport.name);
      await listTask(page);
      await page.getByRole("button", { name: growthCopy[locale].assets, exact: true }).click();
      await expect(page.getByTestId("ads-connection")).toBeVisible();
      await shot(page, "ads-connection", locale, viewport.name);
      await page.getByRole("button", { name: growthCopy[locale].list, exact: true }).click();
      await page.getByTestId("ads-results-link").click();
      await page.getByTestId("ads-report-load").click();
      await expect(page.getByTestId("ads-block-delivery")).toBeVisible();
      // The harness account is TWD. Literal symbols catch a zh-TW-only bare-$ regression.
      await expect(page.getByTestId("ads-block-delivery").locator("dl dd").nth(0)).toHaveText(/^NT\$[\d,.]+$/);
      await expect(page.getByTestId("ads-block-reported").locator("dl dd").nth(1)).toHaveText(/^NT\$[\d,.]+$/);
      await shot(page, "ads-report", locale, viewport.name);
    }
  }
});

});

// The two gates below are independent of the serial flow (a failure here must not hide the other): each proves one frozen
// behaviour that the merchant can only see through the UI.
test.describe("MA09a frozen responses reach the merchant", () => {
  test.describe.configure({ mode: "default" });

  test("MA09a a refused command reaches the merchant with its frozen code (over_allowance at approve)", async ({ page }) => {
  await signedLogin(page);
  await openAds(page);
  await page.getByTestId("ads-new-draft").click();
  await sourceTask(page);
  await page.getByTestId("ads-f-source").fill(`${pageAsset}_9876543210`);
  await page.getByRole("button", { name: growth(page).keepSource, exact: true }).click();
  await settingsTask(page);
  await page.getByTestId("ads-f-budget").fill("20000"); // NT$20,000 > the NT$10,000 allowance
  await page.getByTestId("ads-f-starts").fill(localInput(5 * 3600_000));
  await page.getByTestId("ads-f-ends").fill(localInput(29 * 3600_000));
  await applyTask(page);
  await page.getByTestId("ads-f-save").click();
  await expect(page.getByTestId("ads-banner")).toHaveText(en.saved);
  let status = 0;
  page.on("response", (r) => { if (/\/approve$/.test(new URL(r.url()).pathname)) status = r.status(); });
  await auditDraft(page);
  await page.getByTestId("ads-approve").click();
  await expect(page.getByTestId("ads-banner")).toBeVisible();
  expect(status).toBe(409);
  // §7: 409 over_allowance. The UI keys its copy on the code; a code rewritten to a generic one shows the retry copy instead.
  await expect(page.getByTestId("ads-banner")).toHaveText(en.errors.over_allowance);
});

  test("MA09a a report window without any insights still renders the three blocks (zeros, not fetched), not an error", async ({ page }) => {
    await signedLogin(page);
    await openAds(page);
    await page.getByTestId("ads-results-link").click();
    await page.getByTestId("ads-report-from").fill("2026-01-01");
    await page.getByTestId("ads-report-to").fill("2026-01-07");
    await page.getByTestId("ads-report-load").click();
    // §7 / §15.5: three separate blocks with window, timezone and fetched_at, even before Meta delivered anything
    await expect(page.getByTestId("ads-block-orders")).toBeVisible();
    await expect(page.getByTestId("ads-block-delivery")).toBeVisible();
    await expect(page.getByTestId("ads-block-reported")).toBeVisible();
    await expect(page.getByTestId("ads-report")).not.toContainText(en.unavailable);
  });
});
}

// Go first verifies every frozen single-activation assertion, then starts this
// phase and checks its newly created draft independently. Neither suite is skipped.
if (refusalPhase) {
test("MA09a Meta refusal original text survives refresh in three locales and permits an explicit retry", async ({page}) => {
  await signedLogin(page);
  await openAds(page);
  await page.getByTestId("ads-new-draft").click();
  await sourceTask(page);
  await page.getByTestId("ads-f-source").fill(`${pageAsset}_1234567890`);
  await page.getByRole("button", { name: growth(page).keepSource, exact: true }).click();
  await settingsTask(page);
  await page.getByTestId("ads-f-budget").fill("3000");
  await page.getByTestId("ads-f-starts").fill(localInput(3*3600_000));
  await page.getByTestId("ads-f-ends").fill(localInput(27*3600_000));
  const saved = page.waitForResponse(r=>r.request().method()==="POST" && /\/ads\/drafts$/.test(new URL(r.url()).pathname));
  await applyTask(page);
  await page.getByTestId("ads-f-save").click();
  const created = await saved;
  expect(created.status()).toBe(201);
  const {id} = await created.json();
  expect(id).toMatch(uuidRe);
  await auditDraft(page);
  await page.getByTestId("ads-approve").click();
  await expect(page.getByTestId("ads-detail")).toHaveAttribute("data-status","APPROVED");
  await ctl("reject/meta");
  await page.getByTestId("ads-publish").click();
  await driveUntil(page,"FAILED");
  const original = "Meta original: 請完成驗證 & retry.";
  // RVFIX2-366 P1: the refusal text is shown outside the Advanced disclosure (driveUntil opened it; the node must not be inside it)
  await expect(outsideAdvanced(page).getByText(original, { exact: false }).first()).toBeVisible();
  for (const locale of ["zh-TW","zh-CN","en"] as const) {
    for (const viewport of ["desktop","mobile"] as const) {
      await page.setViewportSize(viewport==="desktop" ? {width:1586,height:992} : {width:390,height:844});
      await page.goto(`${origin}/${locale}/ads?store=${store}&draft=${id}`);
      await auditDraft(page);
      const message=page.getByTestId("ads-meta-message");
      await expect(message).toHaveText(original);
      const refusal = page.getByTestId("ads-ops").locator('[data-op-code="graph_100"]');
      await expect(refusal).toHaveText(growthCopy[locale].metaRefusal);
      await expect(page.getByTestId("ads-ops").getByText("graph_100", { exact: true })).toHaveCount(0);
      await expect(page.getByTestId("ads-ops").locator('[data-op-kind="adset"]')).toHaveText(growthCopy[locale].opKinds.adset);
      const detailURL = page.url();
      await listTask(page);
      await page.getByRole("button", { name: growth(page).assets, exact: true }).click();
      await expect(page.getByTestId("ads-meta-refusals")).toContainText(original);
      await page.goto(detailURL);
      await auditDraft(page);
      await expect(message.locator("script,b,a")).toHaveCount(0);
      await page.reload();
      await auditDraft(page);
      await expect(message).toHaveText(original);
      await expect(refusal).toHaveText(growthCopy[locale].metaRefusal);
      await message.scrollIntoViewIfNeeded();
      await shot(page,"meta-original-refusal",locale,viewport);
    }
  }
  const retry = page.waitForResponse(r=>r.request().method()==="POST" && /\/publish$/.test(new URL(r.url()).pathname));
  await page.getByTestId("ads-publish").click();
  expect((await retry).status()).toBe(200);
  await driveUntil(page,"ACTIVE");
  await page.getByTestId("ads-pause").click();
  await driveUntil(page,"PAUSED");
  await noSecrets(page);
});
}

// C3 cases live in this registered fixture entry point so check-gates proves reachability.
{
type Environment = {
  origin: string; store: string; evidence: string;
  login: (page: Page) => Promise<void>;
};
const devices = [
  { width: 1440, height: 900, side: 0 },
  { width: 390, height: 844, side: 1 },
] as const;
const frames = {
  editor: ["6:20130", "6:20255"], source: ["1049:6945", "1049:7058"],
  settings: ["1049:7120", "1049:7233"], review: ["1049:7295", "1049:7408"],
  saved: ["1049:7470", "1049:7583"], status: ["1049:7645", "1049:7758"],
  leave: ["1049:7820", "1049:7933"], conflict: ["1049:7995", "1049:8108"],
  detail: ["5:17304", "5:17438"], assets: ["5:17516", "5:17633"],
  legend: ["1134:9548", "1134:9549"], loading: ["1134:9550", "1134:9551"],
  list: ["4:11612", "4:11750"], index: ["6:23064", "6:23199"],
  results: ["1151:9420", "1151:9421"], attribution: ["1151:9422", "1151:9423"],
  breakdown: ["1151:9428", "1151:9429"],
} as const;

async function shot(page: Page, env: Environment, frame: string) {
  const directory = path.join(env.evidence, "parity");
  // GitHub artifacts reject colons in paths; retain the exact Figma id in frames.jsonl.
  expect(frame).toMatch(/^\d+:\d+$/);
  const filename = `${frame.replace(":", "-")}.png`;
  expect(filename).toMatch(/^\d+-\d+\.png$/);
  await mkdir(directory, { recursive: true });
  // READ/MEASURE only. Screenshots document the real route, never /ui-gallery.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  await page.screenshot({ path: path.join(directory, filename), animations: "disabled" });
  // READ/MEASURE: visible text only; never change the DOM or use it to drive the journey.
  const density = await page.locator(".growth-task").evaluate(root => {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    let node: Node | null; const texts: string[] = [];
    while ((node = walker.nextNode())) {
      const text = node.textContent?.trim(); if (!text || !node.parentElement?.checkVisibility()) continue;
      const range = document.createRange(); range.selectNodeContents(node);
      const box = range.getBoundingClientRect();
      if (box.bottom > 0 && box.top < innerHeight && box.width > 0) texts.push(text);
    }
    return { blocks: texts.length, characters: texts.reduce((n, text) => n + text.length, 0) };
  });
  await appendFile(path.join(directory, "density.jsonl"), JSON.stringify({ frame, ...density }) + "\n");
  await appendFile(path.join(directory, "frames.jsonl"), JSON.stringify({ frame, route: page.url(), screenshot: `parity/${filename}` }) + "\n");
}
async function click(page: Page, env: Environment, name: string, expected: string) {
  await page.getByRole("button", { name, exact: true }).click();
  await expect(page.getByRole("heading", { name: expected, exact: true })).toBeVisible();
  await appendFile(path.join(env.evidence, "growth-click-ledger.jsonl"), JSON.stringify({ route: page.url(), control: name, operation: "click", expected, actual: "visible", result: "PASS" }) + "\n");
}
async function noLegacyAds(page: Page) {
  await expect(page.getByRole("heading", { level: 1, name: "廣告", exact: true })).toHaveCount(0);
  // Separate task screens replace the legacy stack of connection, editor, CAPI and report panels.
  await expect(page.getByTestId("ads-capi")).toHaveCount(0);
  await expect(page.getByTestId("ads-report")).toHaveCount(0);
}
async function newDraft(page: Page, env: Environment) {
  await page.goto(`${env.origin}/zh-TW/ads?store=${env.store}`);
  await page.getByTestId("ads-new-draft").click();
  await expect(page.getByRole("heading", { name: "建立廣告草稿", exact: true })).toBeVisible();
  await noLegacyAds(page);
}
async function fillDraft(page: Page, source: string) {
  await page.getByRole("button", { name: "選擇貼文或商品", exact: true }).click();
  // The original MA09 fixture saved this source through the real API; no invented source catalogue.
  await page.locator(`[data-source-ref="${source}"]`).getByRole("button", { name: "使用這個來源", exact: true }).click();
  await page.getByRole("button", { name: "修改預算、排期與對象", exact: true }).click();
  await page.getByTestId("ads-f-budget").fill("1500");
  await page.getByTestId("ads-f-starts").fill(instantToTaipei(new Date(Date.now() + 3600_000).toISOString()));
  await page.getByTestId("ads-f-ends").fill(instantToTaipei(new Date(Date.now() + 25 * 3600_000).toISOString()));
  await page.getByRole("button", { name: "套用到草稿", exact: true }).click();
}

/** Register inside the existing non-refusal ads fixture; all new writes remain local MOCK drafts. */
function registerGrowthAdsParity(env: Environment & { pageAsset: string }) {
  test.describe("PARITY-GROWTH signed ads states", () => {
    test.describe.configure({ mode: "default" });
    for (const device of devices) {
      const frame = (state: keyof typeof frames) => frames[state][device.side];
      test(`C3 list, state explanations, assets and read feedback ${device.width}`, async ({ page }) => {
        await env.login(page);
        await page.setViewportSize({ width: device.width, height: device.height });
        await page.goto(`${env.origin}/zh-TW/ads?store=${env.store}`);
        await expect(page.getByRole("heading", { name: "廣告草稿", exact: true })).toBeVisible();
        await expect(page.getByRole("button", { name: "新增草稿", exact: true })).toBeVisible();
        await expect(page.getByText("先核對版本，再核准與發布。內部核准不等於 Meta 審核通過。", { exact: true })).toBeVisible();
        await noLegacyAds(page);
        await shot(page, env, frame("list"));
        await click(page, env, "狀態說明", "狀態各自代表什麼");
        for (const state of ["內部已核准", "已建立 · 尚未啟用", "外部結果待確認", "已確認暫停"])
          await expect(page.getByText(state, { exact: true })).toBeVisible();
        await shot(page, env, frame("legend"));
        await page.getByRole("button", { name: "返回廣告列表", exact: true }).click();
        await click(page, env, "查看廣告帳號連結", "資產授權");
        await expect(page.getByRole("heading", { name: "已授權資產", exact: true })).toBeVisible();
        await expect(page.getByRole("button", { name: "重新授權", exact: true })).toBeVisible();
        await shot(page, env, frame("assets"));
        // An unavailable saved selection uses the signed old-detail recovery screen.
        await page.goto(`${env.origin}/zh-TW/ads?store=${env.store}&draft=00000000-0000-4000-8000-000000000001`);
        await expect(page.getByRole("heading", { name: "從列表開啟正確的草稿", exact: true })).toBeVisible();
        await shot(page, env, frame("detail"));
        await page.getByRole("button", { name: "前往廣告列表", exact: true }).click();
        await page.goto(`${env.origin}/zh-TW/ads?store=${env.store}`);
        await expect(page.getByRole("button", { name: "重新讀取", exact: true })).toBeVisible();
        let release!: () => void;
        const hold = new Promise<void>(resolve => { release = resolve; });
        const continuations: Promise<void>[] = [];
        // Fixture preparation: delay a REAL read, preserve its response body and every write.
        await page.route("**/api/stores/*/ads/drafts", route => {
          const continuation = hold.then(() => route.continue());
          continuations.push(continuation);
          return continuation;
        });
        try {
          await page.getByRole("button", { name: "重新讀取", exact: true }).click();
          await expect(page.getByRole("heading", { name: "正在讀取廣告列表", exact: true })).toBeVisible();
          await shot(page, env, frame("loading"));
        } finally {
          release();
          // Removing interception resumes pending reads; finish our handlers first to avoid double handling.
          await Promise.all(continuations);
          await page.unroute("**/api/stores/*/ads/drafts");
        }
        await expect(page.getByRole("heading", { name: "廣告草稿", exact: true })).toBeVisible();
      });
      test(`C3 editor, settings, leave and source ${device.width}`, async ({ page }) => {
        await env.login(page); await page.setViewportSize({ width: device.width, height: device.height }); await newDraft(page, env);
        for (const label of ["01 · 選擇推廣內容", "02 · 總預算與排期", "03 · 投放對象與身份"])
          await expect(page.getByText(label, { exact: true })).toBeVisible();
        await shot(page, env, frame("editor"));
        await click(page, env, "選擇貼文或商品", "選擇這次要推廣的內容");
        await expect(page.getByRole("button", { name: "保留原選擇並返回", exact: true })).toBeVisible();
        await shot(page, env, frame("source"));
        await page.getByRole("button", { name: "保留原選擇並返回", exact: true }).click();
        await click(page, env, "修改預算、排期與對象", "修改預算、排期與對象");
        await expect(page.getByText("自動版位", { exact: true })).toBeVisible();
        for (const [label, value] of [["NT$3,000", "3000"], ["NT$1,500", "1500"]]) {
          await page.getByRole("button", { name: label, exact: true }).click();
          await expect(page.getByTestId("ads-f-budget")).toHaveValue(value);
        }
        for (const [label, value] of [["香港 HK", "HK"], ["台灣 TW", "TW"]]) {
          await page.getByRole("button", { name: label, exact: true }).click();
          await expect(page.getByTestId("ads-f-countries")).toHaveValue(value);
        }
        for (const [label, min, max] of [["25–44 歲", "25", "44"], ["18–65 歲", "18", "65"]]) {
          await page.getByRole("button", { name: label, exact: true }).click();
          await expect(page.getByTestId("ads-f-agemin")).toHaveValue(min);
          await expect(page.getByTestId("ads-f-agemax")).toHaveValue(max);
        }
        for (const [label, days] of [["3 天", 3], ["7 天", 7]] as const) {
          await page.getByRole("button", { name: label, exact: true }).click();
          const start = await page.getByTestId("ads-f-starts").inputValue(), end = await page.getByTestId("ads-f-ends").inputValue();
          expect(Date.parse(taipeiToInstant(end)!) - Date.parse(taipeiToInstant(start)!)).toBe(days * 86400_000);
        }
        await appendFile(path.join(env.evidence, "growth-click-ledger.jsonl"), JSON.stringify({ route: page.url(), control: "budget/country/age/schedule presets", operation: "8 real clicks", expected: "controlled form reflects each preset", actual: "asserted after every click", result: "PASS" }) + "\n");
        await page.getByTestId("ads-f-budget").fill("1500");
        await shot(page, env, frame("settings"));
        await page.getByRole("button", { name: "套用到草稿", exact: true }).click();
        await click(page, env, "離開", "要保留這些未儲存的修改嗎？");
        await shot(page, env, frame("leave"));
        await page.getByRole("button", { name: "繼續編輯", exact: true }).click();
        await page.getByRole("button", { name: "修改預算、排期與對象", exact: true }).click();
        await expect(page.getByTestId("ads-f-budget")).toHaveValue("1500");
        await page.getByRole("button", { name: "取消", exact: true }).click();
      });
      test(`C3 persisted saved, review and status ${device.width}`, async ({ page }) => {
        await env.login(page); await page.setViewportSize({ width: device.width, height: device.height }); await newDraft(page, env);
        await fillDraft(page, `${env.pageAsset}_1234567890`);
        const response = page.waitForResponse(r => r.request().method() === "POST" && /\/ads\/drafts$/.test(new URL(r.url()).pathname));
        await click(page, env, "儲存草稿", "草稿已儲存");
        const saved = await response;
        expect(saved.status()).toBe(201);
        const draft = await saved.json() as { id: string };
        await shot(page, env, frame("saved"));
        await click(page, env, "核對並核准", "核准前確認");
        for (const fact of ["推廣內容", "整段總預算與排期", "投放對象", "核准與發布分開"])
          await expect(page.getByText(fact, { exact: true })).toBeVisible();
        await expect(page.getByTestId("ads-detail")).toContainText("1,500");
        await shot(page, env, frame("review"));
        // Inspect status only; this parity journey never approves or publishes an ad.
        await page.goto(`${env.origin}/zh-TW/ads?store=${env.store}&draft=${draft.id}`);
        await expect(page.getByRole("button", { name: "查看廣告狀態", exact: true })).toBeVisible();
        await click(page, env, "查看廣告狀態", "尚未讀取廣告狀態");
        await shot(page, env, frame("status"));
        await page.getByRole("button", { name: "前往廣告列表", exact: true }).click();
        await page.getByTestId(`ads-open-${draft.id}`).click();
        await expect(page.getByTestId("ads-detail")).toContainText("1,500");
        await page.reload();
        await expect(page.getByTestId("ads-detail")).toContainText("1,500");
      });
      test(`C3 concurrent edit preserves local work ${device.width}`, async ({ page, context }) => {
        await env.login(page); await page.setViewportSize({ width: device.width, height: device.height }); await newDraft(page, env);
        await fillDraft(page, `${env.pageAsset}_1234567890`);
        const created = page.waitForResponse(r => r.request().method() === "POST" && /\/ads\/drafts$/.test(new URL(r.url()).pathname));
        await page.getByRole("button", { name: "儲存草稿", exact: true }).click();
        const draft = await (await created).json() as { id: string };
        const url = `${env.origin}/zh-TW/ads?store=${env.store}&draft=${draft.id}`;
        await page.goto(url); await page.getByTestId("ads-edit").click();
        await page.getByRole("button", { name: "修改預算、排期與對象", exact: true }).click();
        const other = await context.newPage();
        try {
          await other.goto(url); await other.getByTestId("ads-edit").click();
          await other.getByRole("button", { name: "修改預算、排期與對象", exact: true }).click();
          await other.getByTestId("ads-f-budget").fill("1700");
          await other.getByRole("button", { name: "套用到草稿", exact: true }).click();
          const updated = other.waitForResponse(r => r.request().method() === "PUT" && r.url().endsWith(`/drafts/${draft.id}`));
          await other.getByRole("button", { name: "儲存草稿", exact: true }).click();
          expect((await updated).ok()).toBe(true);
          await page.getByTestId("ads-f-budget").fill("1600");
          await page.getByRole("button", { name: "套用到草稿", exact: true }).click();
          await click(page, env, "儲存草稿", "另一位管理員已更新草稿");
          await expect(page.getByText("這次修改尚未覆蓋伺服器內容。", { exact: true })).toBeVisible();
          await shot(page, env, frame("conflict"));
          await page.getByRole("button", { name: "返回保留修改", exact: true }).click();
          await page.getByRole("button", { name: "修改預算、排期與對象", exact: true }).click();
          await expect(page.getByTestId("ads-f-budget")).toHaveValue("1600");
          await page.getByRole("button", { name: "取消", exact: true }).click();
          await page.getByRole("button", { name: "儲存草稿", exact: true }).click();
          await page.getByRole("button", { name: "讀取最新草稿", exact: true }).click();
          await page.getByTestId("ads-edit").click();
          await page.getByRole("button", { name: "修改預算、排期與對象", exact: true }).click();
          await expect(page.getByTestId("ads-f-budget")).toHaveValue("1700");
          await page.reload();
          await expect(page.getByTestId("ads-detail")).toContainText("1,700");
        } finally { await other.close(); }
      });
    }
    test("C3 ads main screen in all three locales", async ({ page }) => {
      await env.login(page);
      for (const [locale, heading, action] of [["zh-TW", "廣告草稿", "新增草稿"], ["zh-CN", "广告草稿", "新增草稿"], ["en", "Ad drafts", "New draft"]]) {
        await page.goto(`${env.origin}/${locale}/ads?store=${env.store}`);
        await expect(page.getByRole("heading", { level: 1, name: heading, exact: true })).toBeVisible();
        await expect(page.getByRole("button", { name: action, exact: true })).toBeVisible();
      }
    });
  });
}


// C3 parity runs after the existing connection fixture has been established.
if (!refusalPhase) registerGrowthAdsParity({ origin, store, evidence, pageAsset, login: signedLogin });
}
