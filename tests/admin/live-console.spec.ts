// Purpose: LC-U1 real-click workspace contract gate in three locales and two viewport sizes.
// Depends on: @playwright/test, node:fs/promises, node:crypto; signed OIDC/Next/PG harness env LC_BROWSER_CONSOLE_*.
// Used by: browser_live_console_test.go and test-local.sh --browser-live-console.
// Invariants: I01/I02/I06/I10/I11/I14/I18; Console upstream/receipts are explicitly MOCK, never SQL/provider acceptance.
import { test, expect, type Page, type APIRequestContext } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { randomBytes } from "node:crypto";
import { workspaceCopy } from "../../apps/admin/src/features/live/workspace-copy";

const required = (name: string): string => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const api = required("LC_BROWSER_API_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_CONSOLE_STORE");
const otherStore = required("LC_BROWSER_CONSOLE_OTHER_STORE");
const otherScene = required("LC_BROWSER_CONSOLE_OTHER_SCENE");
const lateScene = required("LC_BROWSER_CONSOLE_LATE_SCENE");
const lateDestination = required("LC_BROWSER_CONSOLE_LATE_DEST");
const control = required("LC_BROWSER_CONSOLE_CONTROL");
const narrowToken = required("LC_BROWSER_CONSOLE_NARROW_TOKEN");
const scenes: string[] = JSON.parse(required("LC_BROWSER_CONSOLE_SCENES"));
if (scenes.length !== 6 || new Set(scenes).size !== 6) throw new Error("six independent Console scenes required");
const locales = ["en", "zh-TW", "zh-CN"] as const;
const sizes = [{ width: 1586, height: 992 }, { width: 390, height: 844 }];
type Receipt = { scene: string; action: string; key_hash: string; body_hash: string; status: number; effect: boolean; input: Record<string, unknown> };
type Scene = { ID: string; Title: string; Phase: string; Offer: string; Active: boolean; Stock: number; StockVersion: number; OfferVersion: number; Reads: number[] | null; Recommended: unknown; Copied: boolean };
type Facts = { class: "MOCK"; scenes: Record<string, Scene>; receipts: Receipt[]; bad_authority: number };

// Trace records credential headers; page-only screenshots and redacted receipts are the evidence.
test.use({ baseURL: origin, trace: "off", screenshot: "only-on-failure" });
test.setTimeout(110_000);

async function facts(request: APIRequestContext): Promise<Facts> {
  const response = await request.get(`${api}/__test/live-console/facts`, { headers: { "X-Console-Control": control } });
  expect(response.status()).toBe(200);
  const value = await response.json() as Facts;
  expect(value.class).toBe("MOCK");
  expect(value.bad_authority).toBe(0);
  return value;
}
async function fault(request: APIRequestContext, scene: string, mode: string) {
  // G-UI8 audit [FIXTURE/SETUP]: loopback-only fault control never patches product DOM/API responses in the browser.
  const response = await request.post(`${api}/__test/live-console/fault`, { headers: { "X-Console-Control": control }, data: { scene, mode } });
  expect(response.status()).toBe(200);
}
async function login(page: Page) {
  await page.goto(`${origin}/en/`);
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("shell-store-selector")).toBeAttached();
}
const route = (locale: string, scene: string, selectedStore = store) => `${origin}/${locale}/studio/console?store=${selectedStore}&scene=${scene}`;
async function phase(page: Page, value: string) {
  await expect(page.getByTestId("live-phase")).toHaveAttribute("data-phase", value);
}
async function screenshot(page: Page, name: string) {
  // G-UI8 audit [READ/MEASURE]: layout and storage reads do not change product state.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
  await page.screenshot({ path: `${evidence}/${name}.png`, fullPage: true, animations: "disabled" });
}

for (const [localeIndex, locale] of locales.entries()) for (const [sizeIndex, size] of sizes.entries()) {
  const index = localeIndex * 2 + sizeIndex;
  const scene = scenes[index]!;
  const name = `${locale}-${size.width}`;
  test(`LC-U1 MOCK contract real clicks ${name}`, async ({ page, request }) => {
    await page.setViewportSize(size);
    await login(page);
    await page.goto(route(locale, scene));
    await expect(page.getByTestId("live-workspace")).toBeVisible();
    await expect(page.getByTestId("live-console")).toBeVisible();
    await phase(page, "draft");
    await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
    const initial = (await facts(request)).scenes[scene]!;
    const offer = initial.Offer;
    await expect(page.getByTestId(`live-offer-${offer}`)).toContainText("MOCK Console Tea");
    await screenshot(page, `${name}-draft`);
    // Actual BFF CSRF/Origin refusal must happen before the MOCK write transport receives a command.
    const beforeRefusal = (await facts(request)).receipts.length;
    const refused = await page.request.post(`${origin}/api/stores/${store}/live-sessions/${scene}/lifecycle`, {
      headers: { Origin: origin, "Idempotency-Key": `lc-u1-csrf-${name}` },
      data: { action: "start", expected_version: 1 },
    });
    expect(refused.status()).toBe(403);
    expect((await facts(request)).receipts).toHaveLength(beforeRefusal);

    // Real elapsed cadence, without fake clock/DOM mutation: two successive 5 s reads.
    const baseline = initial.Reads?.length ?? 0;
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Reads?.length ?? 0, { timeout: 15_000 }).toBeGreaterThanOrEqual(baseline + 2);
    const timed = (await facts(request)).scenes[scene]!.Reads!;
    const cadence = timed.slice(baseline - 1);
    for (let i = 1; i < cadence.length; i++) {
      expect(cadence[i]! - cadence[i - 1]!).toBeGreaterThanOrEqual(3500);
      expect(cadence[i]! - cadence[i - 1]!).toBeLessThanOrEqual(8000);
    }
    await page.getByTestId("live-primary-action").click();
    await phase(page, "live");
    expect((await facts(request)).scenes[scene]!.Phase).toBe("live");
    await screenshot(page, `${name}-live`);

    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Active).toBe(false);
    expect((await facts(request)).receipts.at(-1)!.input.max_quantity_per_claim).toBe(10);
    await page.getByTestId("live-console-refresh").click();
    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Active).toBe(true);

    await page.getByTestId(`live-stock-${offer}`).fill("17");
    await page.getByTestId(`live-stock-save-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Stock).toBe(17);
    const stockReceipt = (await facts(request)).receipts.at(-1)!;
    expect(stockReceipt.input).toMatchObject({ delta: 5, expected_version: 1, reason: "live_console_edit" });
    await page.reload();
    await expect(page.getByTestId(`live-stock-${offer}`)).toHaveValue("17");
    await page.getByTestId(`live-recommend-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Recommended).toEqual({ offer_id: offer, at: "2030-01-01T00:00:00Z" });
    expect((await facts(request)).receipts.at(-1)!.input.post_comment).toBe(false);

    // Server-side CAS bump, then a real click must reconcile before a fresh command.
    await fault(request, scene, "conflict");
    const conflictCount = (await facts(request)).receipts.length;
    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect.poll(async () => (await facts(request)).receipts.length).toBe(conflictCount + 1);
    expect((await facts(request)).receipts.at(-1)!.status).toBe(409);
    await expect(page.getByTestId("live-command-retry")).toHaveCount(0);
    await page.getByTestId("live-console-refresh").click();
    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Active).toBe(false);
    await page.getByTestId("live-console-refresh").click();

    // I06: lose ACK after a committed MOCK effect. Polling is read-only; explicit retry must reuse the key.
    await fault(request, scene, "unknown");
    const beforeUnknown = (await facts(request)).receipts.length;
    await page.getByTestId(`live-offer-toggle-${offer}`).click();
    await expect(page.getByTestId("live-command-retry")).toBeVisible();
    const unknown = (await facts(request)).receipts.at(-1)!;
    expect(unknown.status).toBe(503);
    expect(unknown.effect).toBe(true);
    await page.getByTestId("live-console-refresh").click();
    await expect(page.getByTestId("live-command-retry")).toBeVisible();
    const listFailed = page.waitForResponse((response) => response.url().includes(`/api/stores/${store}/live-sessions?`) && response.status() === 503);
    await fault(request, scene, "list_unavailable");
    await listFailed;
    await expect(page.getByTestId("live-command-retry")).toBeVisible();
    await fault(request, scene, "list_restore");
    const reads = (await facts(request)).scenes[scene]!.Reads?.length ?? 0;
    await expect.poll(async () => (await facts(request)).scenes[scene]!.Reads?.length ?? 0, { timeout: 8_000 }).toBeGreaterThan(reads);
    expect((await facts(request)).receipts).toHaveLength(beforeUnknown + 1);
    await page.getByTestId("live-command-retry").click();
    await expect(page.getByTestId("live-command-retry")).toHaveCount(0);
    const retries = (await facts(request)).receipts.filter((receipt) => receipt.key_hash === unknown.key_hash);
    expect(retries).toHaveLength(2);
    expect(retries.map((receipt) => receipt.body_hash)).toEqual([unknown.body_hash, unknown.body_hash]);
    expect(retries.filter((receipt) => receipt.effect)).toHaveLength(1);

    page.once("dialog", (dialog) => dialog.accept());
    await page.getByTestId("live-primary-action").click();
    await phase(page, "ended");
    await screenshot(page, `${name}-ended`);
    await page.getByTestId("live-session-results").click();
    await expect(page.getByTestId("merchant-studio")).toBeVisible();
    await page.goto(route(locale, scene));
    await phase(page, "ended");
    await page.getByTestId("live-primary-action").click();
    await page.getByTestId("live-copy-title").fill(`LC-U1 copied ${name}`);
    await page.getByTestId("live-copy-confirm").click();
    await expect(page).not.toHaveURL(route(locale, scene));
    const copied = new URL(page.url()).searchParams.get("scene");
    expect(copied).toBeTruthy();
    expect(copied).not.toBe(scene);
    await phase(page, "draft");
    expect((await facts(request)).scenes[copied!]!.Title).toBe(`LC-U1 copied ${name}`);
    await page.reload();
    await phase(page, "draft");

    // A second lost ACK remains fenced after reload: reads cannot settle an ambiguous command.
    const copiedOffer = (await facts(request)).scenes[copied!]!.Offer;
    await fault(request, copied!, "unknown");
    await page.getByTestId(`live-recommend-${copiedOffer}`).click();
    await expect(page.getByTestId("live-command-retry")).toBeVisible();
    const reloadUnknown = (await facts(request)).receipts.at(-1)!;
    expect(reloadUnknown.scene).toBe(copied);
    expect(reloadUnknown.status).toBe(503);
    await page.reload();
    await expect(page.getByTestId("live-primary-action")).toBeDisabled();
    await expect(page.getByTestId(`live-offer-toggle-${copiedOffer}`)).toBeDisabled();
    await expect(page.getByTestId(`live-stock-save-${copiedOffer}`)).toBeDisabled();
    await expect(page.getByTestId(`live-recommend-${copiedOffer}`)).toBeDisabled();
    await expect(page.getByTestId("live-command-retry")).toHaveCount(0);
    const reloadReads = (await facts(request)).scenes[copied!]!.Reads?.length ?? 0;
    await expect.poll(async () => (await facts(request)).scenes[copied!]!.Reads?.length ?? 0, { timeout: 8_000 }).toBeGreaterThan(reloadReads);
    expect((await facts(request)).receipts.filter((receipt) => receipt.key_hash === reloadUnknown.key_hash)).toHaveLength(1);
    await screenshot(page, `${name}-unknown-reload`);

    // The existing shell store control navigates through the real context; old Console data must disappear.
    await page.getByTestId("shell-store-selector").selectOption(otherStore);
    await expect(page.getByText(`LC-U1 copied ${name}`, { exact: true })).toHaveCount(0);
    await page.goto(route(locale, otherScene, otherStore));
    await expect(page.getByTestId("live-console")).toBeVisible();
    await expect(page.getByText(`LC-U1 copied ${name}`, { exact: true })).toHaveCount(0);
    await page.getByTestId("workspace-sign-out").click();
    await expect(page.getByTestId("live-console")).toHaveCount(0);
    const cookies = await page.context().cookies(origin);
    expect(cookies.filter((cookie) => cookie.name === "__Host-commerce_session" || cookie.name === "__Host-commerce_csrf")).toHaveLength(0);
    const end = await facts(request);
    await writeFile(`${evidence}/${name}-receipts.json`, JSON.stringify({ class: "MOCK", scene, copied, receipts: end.receipts.filter((receipt) => receipt.scene === scene), cadence }, null, 2));
  });
}

test("LC-U1 delayed copy cannot navigate back after an actual SPA scene switch", async ({ page, request }) => {
  await page.setViewportSize(sizes[1]!);
  await login(page);
  await page.goto(route("en", lateScene));
  await phase(page, "draft");
  await page.getByTestId("live-primary-action").click();
  await phase(page, "live");
  page.once("dialog", (dialog) => dialog.accept());
  await page.getByTestId("live-primary-action").click();
  await phase(page, "ended");
  await page.getByTestId("live-primary-action").click();
  await page.getByTestId("live-copy-title").fill("Delayed copy scope test");
  await fault(request, lateScene, "delay_copy");
  const response = page.waitForResponse((r) => r.url().endsWith(`/${lateScene}/copy`) && r.request().method() === "POST");
  await page.getByTestId("live-copy-confirm").click();
  page.once("dialog", (dialog) => dialog.accept());
  await page.locator("#live-session-picker").selectOption(lateDestination);
  await response;
  await expect.poll(() => new URL(page.url()).searchParams.get("store")).toBe(store);
  await expect.poll(() => new URL(page.url()).searchParams.get("scene")).toBe(lateDestination);
  await expect(page.getByText("Delayed copy scope test", { exact: true })).toHaveCount(0);
  expect((await facts(request)).receipts.filter((r) => r.scene === lateScene && r.action === "copy" && r.effect)).toHaveLength(1);
  await screenshot(page, "en-390-delayed-copy-other-scene");
});

for (const [index, locale] of locales.entries()) {
  test(`LC-U1 ${locale}: missing A1 is graceful and live_adjust-only cannot edit stock`, async ({ browser, request }) => {
    const context = await browser.newContext({ viewport: sizes[1], ignoreHTTPSErrors: true });
    try {
      const url = origin.replace(/^http:/, "https:");
      await context.addCookies([
        { name: "__Host-commerce_session", value: narrowToken, url, secure: true, httpOnly: true, sameSite: "Lax" },
        { name: "__Host-commerce_csrf", value: randomBytes(32).toString("base64url"), url, secure: true, httpOnly: false, sameSite: "Lax" },
      ]);
      const page = await context.newPage();
      const scene = lateDestination;
      await page.goto(route(locale, scene));
      await expect(page.getByTestId("live-console")).toBeVisible();
      await phase(page, "draft");
      const body = (await facts(request)).scenes[scene]!;
      await expect(page.getByTestId(`live-stock-${body.Offer}`)).toBeDisabled();
      await expect(page.getByText(workspaceCopy[locale].narrowPending, { exact: true })).toBeVisible();
      const before = (await facts(request)).receipts.length;
      await expect(page.getByTestId(`live-stock-save-${body.Offer}`)).toBeDisabled();
      expect((await facts(request)).receipts).toHaveLength(before);
      await screenshot(page, `${locale}-narrow-permission`);
      await fault(request, scene, "missing");
      try {
        await page.reload();
        await expect(page.getByTestId("live-console-unavailable")).toBeVisible();
        await expect(page.getByTestId(`live-offer-${body.Offer}`)).toHaveCount(0);
        await screenshot(page, `${locale}-missing-A1`);
      } finally { await fault(request, scene, "restore"); }
    } finally { await context.close(); }
  });
}
