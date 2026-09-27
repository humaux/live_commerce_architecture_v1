import { expect, test, type Page, type BrowserContext } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { randomBytes } from "node:crypto";
import { parseDraft } from "../../apps/admin/lib/studio-model";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const api = required("LC_BROWSER_API_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_STUDIO_STORE");
const foreignStore = required("LC_BROWSER_STUDIO_FOREIGN_STORE");
const unlistedStore = required("LC_BROWSER_STUDIO_UNLISTED_STORE");
const preparedSession = required("LC_BROWSER_STUDIO_SESSION");
const readOnlyToken = required("LC_BROWSER_STUDIO_READONLY_TOKEN");
const expiredToken = required("LC_BROWSER_STUDIO_EXPIRED_TOKEN");
const cookieName = "__Host-commerce_session";

test.use({ baseURL: origin, headless: false, trace: "retain-on-failure", screenshot: "only-on-failure" });
test.setTimeout(240_000);

async function signedLogin(page: Page) {
  await page.goto("/en/");
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByRole("button", { name: "Live workspace" })).toBeVisible();
  await page.getByRole("button", { name: "Live workspace" }).click();
  await expect(page.getByTestId("merchant-studio")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Live Studio" })).toBeVisible();
}

async function setSession(context: BrowserContext, token: string) {
  const url = origin.replace(/^http:/, "https:");
  await context.addCookies([
    { name: cookieName, value: token, url, secure: true, httpOnly: true, sameSite: "Lax" },
    { name: "__Host-commerce_csrf", value: randomBytes(32).toString("base64url"),
      url, secure: true, httpOnly: false, sameSite: "Lax" },
  ]);
}

async function storageIsSafe(page: Page) {
  const value = await page.evaluate(async () => JSON.stringify({
    local: { ...localStorage }, session: { ...sessionStorage },
    caches: "caches" in window ? await caches.keys() : [],
  }));
  for (const forbidden of ["authorization_id", "credential_version", "stream_url", "rtmps://", preparedSession,
    "STU04 browser-created scene", "STU04 phone-edited scene", "STU04 lost ACK scene", "STU04 swapped login scene"])
    expect(value).not.toContain(forbidden);
}

async function hideAndReveal(page: Page) {
  const other = await page.context().newPage();
  await other.goto("about:blank");
  await other.bringToFront();
  await expect.poll(() => page.evaluate(() => document.visibilityState)).toBe("hidden");
  await expect(page.getByLabel("Scene name")).toHaveCount(0);
  await page.bringToFront();
  await expect.poll(() => page.evaluate(() => document.visibilityState)).toBe("visible");
  await other.close();
}

async function screenshot(page: Page, name: string, width: number, height: number) {
  await page.setViewportSize({ width, height });
  await expect(page.getByTestId("merchant-studio")).toBeVisible();
  await expect(page.locator(".studio-scene-list .studio-scene").first()).toBeVisible();
  await expect(page.getByLabel(/Scene name|场次名称|場次名稱/)).toBeVisible();
  if (width <= 680) {
    // A desktop-to-phone resize animates the fixed rail off-screen; capture
    // only its settled position, never a partially obscured first viewport.
    await expect.poll(() => page.locator(".rail").evaluate((rail) =>
      Math.ceil(rail.getBoundingClientRect().right))).toBeLessThanOrEqual(0);
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
  await page.screenshot({ path: `${evidence}/${name}.png`, fullPage: false, animations: "disabled" });
}

async function displayedVersion(page: Page) {
  const value = await page.locator(".studio-version strong").innerText();
  const match = value.match(/(\d+)$/);
  if (!match) throw new Error(`missing Studio version in ${value}`);
  return Number(match[1]);
}

test("STU04 signed Studio UI through packaged Next, Go, PG and local MOCK worker", async ({ page, context, browser }) => {
  const authHeaders: string[] = [];
  const createRequests: string[] = [];
  page.on("request", (request) => {
    if (request.url().startsWith(`${origin}/api/stores/`) && request.url().includes("live-sessions"))
      authHeaders.push(request.headers()["authorization"] ?? "");
    if (request.method() === "POST" && /\/api\/stores\/[^/]+\/live-sessions$/.test(request.url()))
      createRequests.push(request.headers()["idempotency-key"] ?? "");
  });
  await signedLogin(page);
  await page.goto(`/en/studio?store=${store}&scene=${preparedSession}`);
  await expect(page.getByText("Prepared rehearsal authority")).toBeVisible();
  await expect(page.getByRole("button", { name: "Start MOCK rehearsal" })).toBeEnabled();
  await screenshot(page, "en-desktop-first-1586x992", 1586, 992);
  await storageIsSafe(page);

  // A new draft is genuinely created by the UI, then edited and reopened.
  await page.getByRole("button", { name: /New scene/ }).click();
  await page.getByLabel("Scene name").fill("STU04 browser-created scene");
  await page.getByLabel("Scheduled time (UTC, optional)").fill("2030-01-01T00:00");
  await page.getByLabel("Canvas ratio").selectOption("16:9");
  const beforeCreateURL = page.url();
  await page.getByRole("button", { name: "Create draft" }).click();
  await expect.poll(() => page.url()).not.toBe(beforeCreateURL);
  await expect(page.getByLabel("Scene name")).toHaveValue("STU04 browser-created scene");
  await expect(page.getByLabel("Scheduled time (UTC, optional)")).toHaveValue("2030-01-01T00:00");
  const createdURL = page.url();
  expect(createdURL).toMatch(/scene=[0-9a-f-]{36}/);
  await expect(page.getByText("No current prepared authority", { exact: false })).toBeVisible();
  await expect(page.getByRole("button", { name: "Start MOCK rehearsal" })).toBeDisabled();
  const firstVersion = await displayedVersion(page);
  await page.getByLabel("Scene name").fill("STU04 browser-edited scene");
  await page.getByRole("button", { name: "Save draft" }).click();
  await expect.poll(() => displayedVersion(page)).toBe(firstVersion + 1);
  await expect(page.getByText("STU04 browser-edited scene").first()).toBeVisible();
  await page.reload();
  await expect(page.getByLabel("Scene name")).toHaveValue("STU04 browser-edited scene");
  await expect(page.getByLabel("Scheduled time (UTC, optional)")).toHaveValue("2030-01-01T00:00");

  // Two real signed UI views race on the same version. The stale tab must
  // report a conflict, not overwrite the newer persisted edit.
  await page.getByLabel("Scene name").fill("STU04 stale edit");
  const competing = await context.newPage();
  await competing.goto(createdURL);
  await expect(competing.getByLabel("Scene name")).toHaveValue("STU04 browser-edited scene");
  await competing.getByLabel("Scene name").fill("STU04 competing edit");
  await competing.getByRole("button", { name: "Save draft" }).click();
  await expect(competing.getByLabel("Scene name")).toHaveValue("STU04 competing edit");
  await competing.close();
  await page.getByRole("button", { name: "Save draft" }).click();
  await expect(page.getByText("The saved scene or rehearsal authority changed", { exact: false })).toBeVisible();
  page.once("dialog", async (dialog) => { await dialog.accept(); });
  await page.reload();
  await expect(page.getByLabel("Scene name")).toHaveValue("STU04 competing edit");

  // Dirty data must survive real browser history and explicit navigation guards.
  await page.getByLabel("Scene name").fill("STU04 dirty retained scene");
  await expect(page.getByText("Unsaved changes")).toBeVisible();
  // Exercise Chromium's actual history traversal. Either a denied back or a
  // same-login recovery on forward must preserve the original dirty form.
  const dismissHistory = async (dialog: import("@playwright/test").Dialog) => {
    expect(["confirm", "beforeunload"]).toContain(dialog.type());
    if (dialog.type() === "confirm") expect(dialog.message()).toContain("Discard unsaved");
    await dialog.dismiss();
  };
  page.on("dialog", dismissHistory);
  await page.goBack({ waitUntil: "domcontentloaded" });
  page.off("dialog", dismissHistory);
  if (page.url() !== createdURL) await page.goForward({ waitUntil: "domcontentloaded" });
  await expect(page).toHaveURL(createdURL);
  await expect(page.getByLabel("Scene name")).toHaveValue("STU04 dirty retained scene");
  page.once("dialog", async (dialog) => { expect(dialog.message()).toContain("Discard unsaved"); await dialog.dismiss(); });
  await page.getByTestId("nav-orders").click();
  await expect(page.getByTestId("merchant-studio")).toBeVisible();
  page.once("dialog", async (dialog) => { expect(dialog.message()).toContain("Discard unsaved"); await dialog.accept(); });
  await page.getByTestId("nav-orders").click();
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  await page.goto(createdURL);
  await expect(page.getByLabel("Scene name")).toHaveValue("STU04 competing edit");
  await page.getByLabel("Scene name").fill("STU04 locale dirty scene");
  page.once("dialog", async (dialog) => { expect(dialog.message()).toContain("Discard unsaved"); await dialog.dismiss(); });
  await page.getByTestId("locale-switch").selectOption("zh-CN");
  await expect(page.getByTestId("merchant-studio")).toBeVisible();
  page.once("dialog", async (dialog) => { await dialog.accept(); });
  await page.getByTestId("locale-switch").selectOption("zh-CN");
  await expect(page).toHaveURL(/\/zh-CN\/studio/);
  await expect(page.getByRole("heading", { name: "直播工作室" })).toBeVisible();
  await page.goto(`/zh-CN/studio?store=${store}&scene=${preparedSession}`);
  await expect(page.getByText("已准备模拟授权")).toBeVisible();
  await screenshot(page, "zh-CN-desktop-first-1586x992", 1586, 992);
  await screenshot(page, "zh-CN-phone-first-390x844", 390, 844);
  await page.goto(`/zh-TW/studio?store=${store}&scene=${preparedSession}`);
  await expect(page.getByRole("heading", { name: "直播工作室" })).toBeVisible();
  await screenshot(page, "zh-TW-desktop-first-1586x992", 1586, 992);
  await screenshot(page, "zh-TW-phone-first-390x844", 390, 844);
  await page.goto(`/en/studio?store=${store}&scene=${preparedSession}`);
  await screenshot(page, "en-phone-first-390x844", 390, 844);
  await page.goto(createdURL);
  await expect(page.getByLabel("Scene name")).toHaveValue("STU04 competing edit");
  const mobileVersion = await displayedVersion(page);
  await page.getByLabel("Scene name").fill("STU04 phone-edited scene");
  await page.getByRole("button", { name: "Save draft" }).click();
  await expect.poll(() => displayedVersion(page)).toBe(mobileVersion + 1);
  await expect(page.getByText("STU04 phone-edited scene").first()).toBeVisible();
  await page.reload();
  await expect(page.getByLabel("Scene name")).toHaveValue("STU04 phone-edited scene");
  await expect(page.getByLabel("Scheduled time (UTC, optional)")).toHaveValue("2030-01-01T00:00");
  await page.setViewportSize({ width: 1586, height: 992 });

  // Establish a real Orders→Studio route boundary before the uncertain write.
  // Back/forward below must cross a page that unmounts Studio if permitted.
  await page.getByTestId("nav-orders").click();
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  await page.getByRole("button", { name: "Live workspace" }).click();
  await expect(page.getByTestId("merchant-studio")).toBeVisible();

  // Fault applies only after the real Go write has committed; retry must
  // reuse the exact command key and PG must still contain one draft.
  const armed = await fetch(`${api}/__test/studio-ui-arm-fault`, { method: "POST" });
  expect(armed.status).toBe(204);
  await page.getByRole("button", { name: /New scene/ }).click();
  await page.getByLabel("Scene name").fill("STU04 lost ACK scene");
  await page.getByRole("button", { name: "Create draft" }).click();
  await expect(page.getByText("The result is unknown", { exact: false })).toBeVisible();
  await storageIsSafe(page);
  const unresolvedURL = page.url();
  await page.goBack({ waitUntil: "domcontentloaded" });
  if (page.url() !== unresolvedURL) await page.goForward({ waitUntil: "domcontentloaded" });
  await expect(page).toHaveURL(unresolvedURL);
  await expect(page.getByRole("button", { name: "Retry same request" })).toBeVisible();
  await page.getByRole("button", { name: "Retry same request" }).click();
  await expect(page.getByLabel("Scene name")).toHaveValue("STU04 lost ACK scene");

  // Prepared session has the only rehearsable authority. The actual worker
  // must consume it; the UI must show persisted observation, then terminal.
  await page.goto(`/en/studio?store=${store}&scene=${preparedSession}`);
  await expect(page.getByRole("button", { name: "Start MOCK rehearsal" })).toBeEnabled();
  await page.getByRole("button", { name: "Start MOCK rehearsal" }).click();
  await expect(page.getByText("Observed locally")).toBeVisible({ timeout: 30_000 });
  await expect(page.getByRole("button", { name: "Request stop" })).toBeEnabled();
  await page.getByRole("button", { name: "Request stop" }).click();
  await expect(page.getByText("Terminal")).toBeVisible({ timeout: 35_000 });
  await expect(page.getByText("Public status unverified").first()).toBeVisible();
  await storageIsSafe(page);

  const csrfDenied = await page.evaluate(async (target) => {
    const response = await fetch(target, { method: "POST", credentials: "same-origin", cache: "no-store",
      headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() },
      body: JSON.stringify({ title: "CSRF denied", aspect_ratio: "9:16", scheduled_at: null }) });
    return { status: response.status, cache: response.headers.get("cache-control") };
  }, `/api/stores/${store}/live-sessions`);
  expect(csrfDenied.status).toBe(403);
  expect(csrfDenied.cache).toBe("private, no-store");
  await page.goto(`/en/studio?store=${foreignStore}&scene=${preparedSession}`);
  await expect(page.getByText("This scene is unavailable", { exact: false })).toBeVisible();
  await page.goto(`/en/studio?store=${unlistedStore}`);
  await expect(page.getByTestId("merchant-studio")).toHaveCount(0);
  await page.goto(`/en/studio?store=${store}&scene=${preparedSession}`);
  await expect(page.getByTestId("merchant-studio")).toBeVisible();
  // A second, independently signed OIDC login for the same merchant is not
  // the initiating login. An unresolved write must not carry its key/form
  // into that different session even though principal and store are equal.
  const armSwap = await fetch(`${api}/__test/studio-ui-arm-fault`, { method: "POST" });
  expect(armSwap.status).toBe(204);
  await page.getByRole("button", { name: /New scene/ }).click();
  await page.getByLabel("Scene name").fill("STU04 swapped login scene");
  await page.getByRole("button", { name: "Create draft" }).click();
  await expect(page.getByRole("button", { name: "Retry same request" })).toBeVisible();
  await storageIsSafe(page);
  const otherLogin = await browser.newContext({ baseURL: origin });
  const otherPage = await otherLogin.newPage();
  await signedLogin(otherPage);
  const oldSession = (await context.cookies()).find((cookie) => cookie.name === cookieName)?.value;
  expect(oldSession).toBeTruthy();
  const replacements = (await otherLogin.cookies()).filter((cookie) =>
    cookie.name === cookieName || cookie.name === "__Host-commerce_csrf");
  expect(replacements).toHaveLength(2);
  expect(replacements.find((cookie) => cookie.name === cookieName)?.value).not.toBe(oldSession);
  await page.bringToFront();
  await context.addCookies(replacements);
  const beforeSwapRetry = createRequests.length;
  await page.getByRole("button", { name: "Retry same request" }).click();
  await expect(page.getByText("Sign in again to open Studio.").first()).toBeVisible();
  expect(createRequests).toHaveLength(beforeSwapRetry);
  await expect(page.getByRole("button", { name: "Retry same request" })).toHaveCount(0);
  // Fresh read under the new signed login must not resurrect the old key/form.
  await page.goto(`/en/studio?store=${store}&scene=${preparedSession}`);
  await expect(page.getByTestId("merchant-studio")).toBeVisible();
  await expect(page.getByRole("button", { name: "Retry same request" })).toHaveCount(0);
  const retained = page.getByLabel("Scene name");
  if (await retained.count()) await expect(retained).not.toHaveValue("STU04 swapped login scene");
  await otherLogin.close();
  const expired = await fetch(`${api}/__test/studio-ui-expire-login`, { method: "POST" });
  expect(expired.status).toBe(204);
  await page.getByRole("button", { name: "Refresh facts" }).click();
  await expect(page.getByText("Sign in again to open Studio.").first()).toBeVisible();
  await expect(page.getByLabel("Scene name")).toHaveCount(0);
  expect(authHeaders.length).toBeGreaterThan(12);
  expect(authHeaders.every((header) => header === "")).toBe(true);
  await writeFile(`${evidence}/browser-summary.json`, JSON.stringify({
    bffRequests: authHeaders.length, locales: ["en", "zh-CN", "zh-TW"],
    viewports: ["1586x992", "390x844"], createdURL, csrfStatus: csrfDenied.status,
  }), { mode: 0o600 });
});

test("STU04 exact draft timestamp parser accepts offsets without admitting malformed dates", () => {
  const base = {
    session_id: "11111111-1111-4111-8111-111111111111",
    program_id: "22222222-2222-4222-8222-222222222222",
    title: "timestamp fixture", scheduled_at: null, aspect_ratio: "16:9", state: "DRAFT", version: 1,
    created_at: "2030-01-01T00:00:00Z", updated_at: "2030-01-01T00:00:00Z",
  };
  for (const value of ["2030-01-01T00:00:00Z", "2030-01-01T08:00:00.123456+08:00", "2029-12-31T18:30:00-05:30"])
    expect(parseDraft({ ...base, created_at: value, updated_at: value, scheduled_at: value }).scheduled_at).toBe(value);
  for (const value of ["2030-01-01T00:00:00", "2030-01-01T00:00:00+25:00", "2030-02-30T00:00:00Z", "2030-01-01T00:00:00+08:99", "2030-01-01T00:00Z"])
    expect(() => parseDraft({ ...base, created_at: value })).toThrow();
});

test("STU04 read-only and expired sessions cannot mutate", async ({ browser }) => {
  const context = await browser.newContext({ baseURL: origin });
  await setSession(context, readOnlyToken);
  const page = await context.newPage();
  await page.goto(`/en/studio?store=${store}&scene=${preparedSession}`);
  await expect(page.getByTestId("merchant-studio")).toBeVisible();
  await expect(page.getByText("Read-only access", { exact: false }).first()).toBeVisible();
  // The shared prepared scene is READY after the real-chain Start, so Start is
  // absent; the remaining Stop control must still deny a read-only principal.
  await expect(page.getByRole("button", { name: "Start MOCK rehearsal" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Request stop" })).toBeDisabled();
  await expect(page.getByLabel("Scene name")).toBeDisabled();
  await expect(page.getByRole("button", { name: /New scene/ })).toBeDisabled();
  await context.close();
  const expired = await browser.newContext({ baseURL: origin });
  await setSession(expired, expiredToken);
  const expiredPage = await expired.newPage();
  await expiredPage.goto(`/en/studio?store=${store}&scene=${preparedSession}`);
  await expect(expiredPage.getByText("Sign in again to open Studio.").first()).toBeVisible();
  await expect(expiredPage.getByRole("button", { name: "Start MOCK rehearsal" })).toHaveCount(0);
  await expired.close();
});

test("STU04 native visibility conceal and revalidation remains required", async ({ page }) => {
  await signedLogin(page);
  await page.goto(`/en/studio?store=${store}&scene=${preparedSession}`);
  // This case must own an editable DRAFT form: the shared prepared scene may
  // already be READY after the preceding real worker Start.
  await page.getByRole("button", { name: /New scene/ }).click();
  await expect(page.getByLabel("Scene name")).toBeEnabled();
  await page.getByLabel("Scene name").fill("STU04 native dirty draft");
  await expect(page.getByText("Unsaved changes")).toBeVisible();
  // No synthetic visibilitychange dispatch: this host must actually report a
  // hidden tab before we can claim the conceal/revalidation lifecycle.
  await hideAndReveal(page);
  await expect(page.getByLabel("Scene name")).toHaveValue("STU04 native dirty draft");
});

test("STU04 native conceal retains an uncertain committed request", async ({ page }) => {
  await signedLogin(page);
  await page.goto(`/en/studio?store=${store}&scene=${preparedSession}`);
  const armed = await fetch(`${api}/__test/studio-ui-arm-fault`, { method: "POST" });
  expect(armed.status).toBe(204);
  await page.getByRole("button", { name: /New scene/ }).click();
  await page.getByLabel("Scene name").fill("STU04 native uncertain scene");
  await page.getByRole("button", { name: "Create draft" }).click();
  await expect(page.getByRole("button", { name: "Retry same request" })).toBeVisible();
  await hideAndReveal(page);
  await expect(page.getByRole("button", { name: "Retry same request" })).toBeVisible();
});
