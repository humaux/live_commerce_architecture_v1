// meta-connect browser gate, Playwright half (contract meta-claims-intake-v1 "Merchant connect (R4)"; unit meta-connect "Done when").
// BFF routes exercised through the UI: POST /api/meta/connect, GET /api/meta/callback (the Login for Business redirect URI),
// GET|POST /api/stores/{store}/meta-connect/{status,states/*,pick,disconnect} -> Go /v1/admin/stores/{store_id}/meta-connect/*.
// Started only by tests/foundation/browser_meta_connect_test.go (build tag browser), which owns the isolated PG, the real Go API with
// the metaconnect service, the fake Graph, the production admin Next build, the signed mock IdP and the runner-only control listener.
// Labels: BROWSER; Meta is MOCK: https://www.facebook.com/... is answered by page.route with a 302 to the app's own callback and the
// code exchange / permissions / Pages / subscribe happen in the API process against the fake Graph.
// Locators use the card's data-testid vocabulary and its copy file (apps/admin/lib/meta-connect-copy.ts).
import { expect, test, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { metaConnectCopy } from "../../apps/admin/lib/meta-connect-copy";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_STORE");
const control = required("LC_BROWSER_CONTROL");
const controlKey = required("LC_BROWSER_CONTROL_KEY");
const appId = required("LC_BROWSER_APP_ID");
const configId = required("LC_BROWSER_CONFIG_ID");
const redirect = required("LC_BROWSER_REDIRECT");
const graphVersion = required("LC_BROWSER_GRAPH_VERSION");
const pageA = { id: required("LC_BROWSER_PAGE_A"), name: required("LC_BROWSER_PAGE_A_NAME"), ig: required("LC_BROWSER_IG_A"), igUser: required("LC_BROWSER_IG_A_USER") };
const pageB = { id: required("LC_BROWSER_PAGE_B"), name: required("LC_BROWSER_PAGE_B_NAME") };
const pageC = { id: required("LC_BROWSER_PAGE_C"), name: required("LC_BROWSER_PAGE_C_NAME") };

test.use({ baseURL: origin, trace: "retain-on-failure", screenshot: "only-on-failure" });
const en = metaConnectCopy.en;
const uuidRe = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

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
async function openSettings(page: Page, locale = "en") {
  await page.goto(`/${locale}/settings?store=${store}`);
  await expect(page.getByTestId("metaconnect-card")).toBeVisible();
}
async function fitsWidth(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
}
const manifestPath = path.join(evidence, "screenshots.json");
async function shot(page: Page, name: string, locale: string, viewport: "desktop" | "mobile") {
  const file = path.join(evidence, `${name}-${locale}-${viewport}.png`);
  await page.getByTestId("metaconnect-card").scrollIntoViewIfNeeded();
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
  const secrets = (await (await ctl("secrets")).json()) as string[];
  const html = await page.content();
  const stored = await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage }, cookie: document.cookie }));
  for (const secret of secrets) {
    expect(html).not.toContain(secret);
    expect(stored).not.toContain(secret);
  }
}

// What the fake Facebook dialog does for the next connect: a profile (permissions + Pages) or a forged state.
let answer: { profile: string } | { wrongState: true } = { profile: "full" };
let dialog: URL | null = null;
async function answerDialog(page: Page) {
  await page.route("https://www.facebook.com/**", async (route) => {
    const url = new URL(route.request().url());
    dialog = url;
    const state = "wrongState" in answer ? "A".repeat(43) : (url.searchParams.get("state") ?? "");
    const profile = "wrongState" in answer ? "full" : answer.profile;
    const { code } = (await (await ctl(`oauth/code?profile=${profile}`)).json()) as { code: string };
    await route.fulfill({ status: 302, headers: { Location: `${origin}/api/meta/callback?code=${encodeURIComponent(code)}&state=${encodeURIComponent(state)}` } });
  });
}

test.describe("meta-connect browser gate", () => {
test.describe.configure({ mode: "serial" });

test("not connected: the card offers Connect; a forged return state is refused and nothing is connected", async ({ page }) => {
  await signedLogin(page);
  await openSettings(page);
  await expect(page.getByTestId("metaconnect-none")).toHaveText(en.notConnected);
  await answerDialog(page);
  answer = { wrongState: true };
  await page.getByTestId("metaconnect-connect").click();
  await expect(page.getByTestId("metaconnect-error")).toHaveText(en.errors.state_mismatch, { timeout: 30_000 });
  await expect(page.getByTestId("metaconnect-none")).toBeVisible();
  expect(new URL(page.url()).search).not.toMatch(/meta_error|code=|state=/); // the return params are stripped once read
  await noSecrets(page);
});

test("a missing permission is listed, the pick is disabled and nothing is connected", async ({ page }) => {
  await signedLogin(page);
  await openSettings(page);
  await answerDialog(page);
  answer = { profile: "nomsg" };
  await page.getByTestId("metaconnect-connect").click();
  await expect(page.getByTestId("metaconnect-pick")).toBeVisible({ timeout: 30_000 });
  await expect(page.getByTestId(`metaconnect-pick-${pageC.id}`)).toBeChecked();
  await expect(page.getByTestId("metaconnect-missing")).toContainText("pages_messaging");
  await expect(page.getByTestId("metaconnect-pick-submit")).toBeDisabled();
  await page.getByRole("button", { name: en.pickCancel }).click();
  await expect(page.getByTestId("metaconnect-none")).toBeVisible();
  await noSecrets(page);
});

test("connect Page A with Instagram: dialog params, state cookie, 303 without code/state, pick, card facts, no secret in the browser", async ({ page }) => {
  let connectSetCookie: string[] = [];
  const callbacks: { status: number; headers: Record<string, string> }[] = [];
  page.on("response", async (response) => {
    const url = new URL(response.url());
    if (url.pathname === "/api/meta/connect" && response.request().method() === "POST") connectSetCookie = (await response.headersArray()).filter((h) => h.name.toLowerCase() === "set-cookie").map((h) => h.value);
    if (url.pathname === "/api/meta/callback") callbacks.push({ status: response.status(), headers: response.headers() });
  });
  await signedLogin(page);
  await openSettings(page);
  await answerDialog(page);
  answer = { profile: "full" };
  await page.getByTestId("metaconnect-connect").click();
  await expect(page.getByTestId("metaconnect-pick")).toBeVisible({ timeout: 30_000 });

  const d = dialog as unknown as URL;
  expect(d.origin).toBe("https://www.facebook.com");
  expect(d.pathname).toBe(`/${graphVersion}/dialog/oauth`);
  expect(d.searchParams.get("client_id")).toBe(appId);
  expect(d.searchParams.get("config_id")).toBe(configId);
  expect(d.searchParams.get("response_type")).toBe("code");
  expect(d.searchParams.get("override_default_response_type")).toBe("true");
  expect(d.searchParams.get("redirect_uri")).toBe(redirect);
  expect(d.searchParams.get("state") ?? "").toMatch(/^[A-Za-z0-9_-]{43}$/);
  expect(d.searchParams.has("scope")).toBe(false); // config_id replaces scope
  expect(d.searchParams.has("client_secret")).toBe(false);
  const cookieLine = connectSetCookie.find((c) => c.startsWith("lc_meta_connect="));
  expect(cookieLine, "lc_meta_connect Set-Cookie on the connect response").toBeTruthy();
  expect(cookieLine!).toContain(`lc_meta_connect=${store}`);
  expect(cookieLine!).toMatch(/HttpOnly/i);
  expect(cookieLine!).toMatch(/SameSite=Lax/i);
  expect(cookieLine!).toMatch(/Path=\/api\/meta\/callback/i);
  expect(cookieLine!).toMatch(/Max-Age=600/i);
  const cb = callbacks.find((r) => r.status === 303);
  expect(cb, "callback 303").toBeTruthy();
  expect(cb!.headers["location"]).toMatch(new RegExp(`^/en/settings\\?store=${store}&meta_connect=[0-9a-f-]{36}$`));
  expect(cb!.headers["location"]).not.toMatch(/code=|state=/);
  expect(cb!.headers["cache-control"]).toMatch(/no-store/);
  expect(cb!.headers["referrer-policy"]).toBe("no-referrer");
  expect(new URL(page.url()).search).not.toMatch(/meta_connect=|code=|state=/);
  void uuidRe;

  // Only this login's Pages are offered; A has Instagram, B does not.
  await expect(page.locator('[data-testid^="metaconnect-pick-"][type="radio"]')).toHaveCount(2);
  await page.getByTestId(`metaconnect-pick-${pageA.id}`).check();
  await expect(page.getByTestId("metaconnect-pick-ig")).toBeChecked();
  await page.getByTestId(`metaconnect-pick-${pageB.id}`).check();
  await expect(page.getByTestId("metaconnect-pick-ig")).toHaveCount(0); // IG absent: no checkbox
  await page.getByTestId(`metaconnect-pick-${pageA.id}`).check();
  const picks: { key: string | null; body: string | null; status: number }[] = [];
  page.on("response", (r) => {
    if (r.request().method() === "POST" && /\/meta-connect\/pick$/.test(new URL(r.url()).pathname))
      picks.push({ key: r.request().headers()["idempotency-key"] ?? null, body: r.request().postData(), status: r.status() });
  });
  await page.getByTestId("metaconnect-pick-submit").click();
  await expect(page.getByTestId("metaconnect-status")).toBeVisible({ timeout: 30_000 });
  expect(picks.length).toBe(1);
  expect(picks[0].status).toBe(201);
  expect(picks[0].key ?? "").toMatch(/^[A-Za-z0-9_.:-]{8,128}$/);
  expect(JSON.parse(picks[0].body ?? "{}")).toEqual({ state_id: expect.stringMatching(uuidRe), page_id: pageA.id, include_instagram: true });

  await expect(page.getByTestId("metaconnect-notice")).toHaveText(en.connectedNotice);
  await expect(page.getByTestId("metaconnect-page")).toContainText(pageA.name);
  await expect(page.getByTestId("metaconnect-page")).toContainText(pageA.id);
  await expect(page.getByTestId("metaconnect-ig")).toHaveText(`@${pageA.igUser}`);
  for (const permission of ["pages_messaging", "pages_manage_metadata", "pages_read_engagement", "pages_show_list", "instagram_manage_comments", "instagram_manage_messages"])
    await expect(page.getByTestId("metaconnect-permissions")).toContainText(permission);
  await expect(page.getByTestId("metaconnect-token")).toHaveText(en.tokenActive);
  await expect(page.getByTestId("metaconnect-token")).toHaveAttribute("data-state", "active");
  await expect(page.getByTestId("metaconnect-last-event")).toHaveText(en.never);
  const facts = (await (await ctl("facts")).json()) as { subscribed: { A: boolean; B: boolean; C: boolean } };
  expect(facts.subscribed).toEqual({ A: true, B: false, C: false });
  await noSecrets(page);

  // Locale + viewport evidence (the same connected card).
  for (const [locale, copy] of [["en", metaConnectCopy.en], ["zh-TW", metaConnectCopy["zh-TW"]], ["zh-CN", metaConnectCopy["zh-CN"]]] as const) {
    for (const viewport of [{ name: "desktop" as const, width: 1586, height: 992 }, { name: "mobile" as const, width: 390, height: 844 }]) {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await openSettings(page, locale);
      await expect(page.getByRole("heading", { name: copy.title, exact: true })).toBeVisible();
      await expect(page.getByTestId("metaconnect-page")).toContainText(pageA.name);
      await shot(page, "metaconnect", locale, viewport.name);
    }
  }
  await page.setViewportSize({ width: 1586, height: 992 });
});

test("while connected the store keeps its Page: no second connect offer, reload shows the same card", async ({ page }) => {
  await signedLogin(page);
  await openSettings(page);
  await expect(page.getByTestId("metaconnect-page")).toContainText(pageA.name);
  await expect(page.getByTestId("metaconnect-connect")).toHaveCount(0);
  await expect(page.getByTestId("metaconnect-reconnect")).toHaveCount(0); // token active and far from expiry
});

test("disconnect asks for confirmation, destroys the connection (the Meta-side subscription stays; the route is disabled)", async ({ page }) => {
  await signedLogin(page);
  await openSettings(page);
  await page.getByTestId("metaconnect-disconnect").click();
  await expect(page.getByTestId("metaconnect-confirm")).toContainText(en.confirmDisconnect);
  await page.getByRole("button", { name: en.cancel }).click();
  await expect(page.getByTestId("metaconnect-status")).toBeVisible(); // cancel changes nothing
  await page.getByTestId("metaconnect-disconnect").click();
  await page.getByTestId("metaconnect-confirm-yes").click();
  await expect(page.getByTestId("metaconnect-none")).toBeVisible({ timeout: 30_000 });
  await expect(page.getByTestId("metaconnect-notice")).toHaveText(en.disconnectedNotice);
  const facts = (await (await ctl("facts")).json()) as { subscribed: { A: boolean; B: boolean; C: boolean } };
  expect(facts.subscribed.A).toBe(true); // no unsubscribe: the API can seal a Page token but never open one
  await noSecrets(page);
});

test("reconnect with a Facebook-only Page: Instagram absent, card says none linked", async ({ page }) => {
  await signedLogin(page);
  await openSettings(page);
  await answerDialog(page);
  answer = { profile: "fbonly" };
  await page.getByTestId("metaconnect-connect").click();
  await expect(page.getByTestId("metaconnect-pick")).toBeVisible({ timeout: 30_000 });
  await expect(page.locator('[data-testid^="metaconnect-pick-"][type="radio"]')).toHaveCount(1);
  await expect(page.getByTestId("metaconnect-pick-ig")).toHaveCount(0);
  await page.getByTestId("metaconnect-pick-submit").click();
  await expect(page.getByTestId("metaconnect-status")).toBeVisible({ timeout: 30_000 });
  await expect(page.getByTestId("metaconnect-page")).toContainText(pageB.name);
  await expect(page.getByTestId("metaconnect-ig")).toHaveText(en.noInstagram);
  const facts = (await (await ctl("facts")).json()) as { subscribed: { A: boolean; B: boolean; C: boolean } };
  expect(facts.subscribed).toEqual({ A: true, B: true, C: false });
  await noSecrets(page);
});
});
