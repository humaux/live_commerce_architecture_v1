// Purpose: native LP1/LP11/LP12 member export + erasure acceptance in an isolated synthetic privacy corpus.
// Depends on: W6 signed identity, production Next/Go/PG, existing member privacy seam selectors.
// Used by: --browser-customers-billing (members-privacy subtest); only bounded BFF refusals are MOCK.
import { test, expect, type Page, type Browser } from "@playwright/test";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";

const required = (key: string) => { const v = process.env[key]; if (!v) throw Error(`${key} required`); return v; };
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"), evidence = required("LC_BROWSER_EVIDENCE");
const control = required("LC_W6UI_CONTROL_URL"), gateKey = required("LC_W6UI_CONTROL_KEY");
const store = required("LC_W6UI_PRIVACY_STORE"), alt = required("LC_W6UI_PRIVACY_ALT");
// Comma-separated synthetic corpus: six locale/size members, recovery, too_large and UNKNOWN members.
const corpus = required("LC_W6UI_PRIVACY_MEMBER").split(",");
if (corpus.length !== 9 || new Set(corpus).size !== 9) throw Error("LC_W6UI_PRIVACY_MEMBER requires 9 distinct ids");
const evidenceLedger: { case: string; page?: string; control?: string; action: string; expected?: string; actual?: string; result: string }[] = [];
const completed: string[] = [];
// Artifact retention only: role/locale/viewport/evidence path, never member ids or PII.
const visual: { role: string; locale: string; width: number; height: number; path: string }[] = [];
test.describe.configure({ mode: "serial" });
test.beforeEach(async ({}, info) => { completed.push(info.title); });
test.afterEach(async ({}, info) => {
  evidenceLedger.push({ case: info.title, action: "native privacy journey", expected: "all visible case assertions pass", actual: info.status ?? "unknown", result: info.status === "passed" ? "pass" : "fail" });
  await writeFile(path.join(evidence, "members-privacy-click-ledger.json"), JSON.stringify(evidenceLedger, null, 2));
});
test.afterAll(async () => {
  await writeFile(path.join(evidence, "members-privacy-completion.json"), JSON.stringify({ cases: completed }, null, 2));
  await writeFile(path.join(evidence, "members-privacy-visual.json"), JSON.stringify({ captures: visual }, null, 2));
});

async function ctl(resource: string, mode = "") {
  const r = await fetch(`${control}/${resource}`, { method: "POST", headers: { "Content-Type": "application/json", "X-W6UI-Gate-Key": gateKey }, body: JSON.stringify({ mode }) });
  // Existing W6 control answers 204; a 200 here would be a different handler, not this fixture.
  expect(r.status).toBe(204);
  return r;
}
const url = (member: string, locale = "en") => `${origin}/${locale}/members/${member}?store=${store}`;
async function signed(browser: Browser, member: string, locale = "en", width = 1440, actor = "privacy-owner") {
  await ctl("actor", actor);
  const context = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width, height: 900 } });
  const page = await context.newPage();
  const consoleText: string[] = [];
  page.on("console", m => consoleText.push(m.text()));
  await page.goto(`${origin}/en/`);
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("shell-store-selector")).toBeVisible();
  await page.goto(url(member, locale));
  await expect(page.getByTestId("member-detail")).toBeVisible();
  return { page, context, consoleText };
}
const dialog = (page: Page) => page.getByTestId("member-privacy-dialog");
/** Full native viewport capture at the already-asserted role; safe static basename. */
async function capture(page: Page, role: string, locale: string) {
  // Observe the exact role being recorded (also settles responsive reflow after resize); role strings are internal.
  await expect(page.locator(`[data-privacy-role="${role}"]`)).toBeVisible();
  const viewport = page.viewportSize()!;
  const file = `members-privacy-${role}-${locale}-${viewport.width}.png`;
  await page.screenshot({ path: path.join(evidence, file) });
  visual.push({ role, locale, width: viewport.width, height: viewport.height, path: file });
}
/** Edge roles are single-locale: capture the actual role at 1440 then 375, restore before continuing. */
async function captureBoth(page: Page, role: string, locale: string) {
  const original = page.viewportSize()!;
  await capture(page, role, locale);
  await page.setViewportSize({ width: 375, height: original.height });
  await capture(page, role, locale);
  await page.setViewportSize(original);
}
const privacyRole = (page: Page, role: string) => expect(dialog(page)).toHaveAttribute("data-privacy-role", role);
async function click(page: Page, id: string) {
  const row = { case: test.info().title, page: new URL(page.url()).pathname, control: id, action: "click", expected: "native control accepts interaction; case asserts visible outcome", actual: "pending", result: "fail" };
  evidenceLedger.push(row);
  await page.getByTestId(id).click();
  row.actual = "native click settled"; row.result = "pass";
}
async function assertNoPrivateMaterial(page: Page, consoleText: string[]) {
  const canaries = ["PRIVACY_CONTACT_CANARY", "PRIVACY_NOTE_CANARY", "PRIVACY_LEDGER_CANARY"];
  expect(page.url()).not.toMatch(/PRIVACY_/);
  const storage = await page.evaluate(() => ({ local: { ...localStorage }, session: { ...sessionStorage } }));
  for (const canary of canaries) {
    expect(JSON.stringify(storage)).not.toContain(canary);
    expect(consoleText.join("\n")).not.toContain(canary);
  }
  expect(JSON.stringify(storage)).not.toContain("Synthetic privacy");
}

for (const [index, locale] of (["en", "zh-TW", "zh-CN"] as const).entries()) for (const width of [1440, 375]) {
  test(`REAL_PG privacy export and erasure ${locale}-${width}`, async ({ browser }) => {
    test.setTimeout(120000);
    const member = corpus[index * 2 + (width === 1440 ? 0 : 1)];
    const { page, context, consoleText } = await signed(browser, member, locale, width);
    try {
      let exportPosts = 0, erasurePosts = 0;
      const keys: string[] = [];
      page.on("request", r => {
        if (r.method() !== "POST" || !r.url().includes(`/members/${member}`)) return;
        const key = r.headers()["idempotency-key"];
        if (key) keys.push(key);
        if (r.url().endsWith("/export")) exportPosts++;
        if (r.url().endsWith("/erasure")) erasurePosts++;
      });
      await click(page, "member-export-open");
      await privacyRole(page, "ExportPrompt");
      await capture(page, "ExportPrompt", locale);
      const [download] = await Promise.all([page.waitForEvent("download"), click(page, "member-export-submit")]);
      await privacyRole(page, "ExportDownloaded");
      // LP12 fixes the payload bound, not a filename template: require an opaque bounded .json name,
      // never a member name/contact/notes canary, and no member-id template assertion.
      const filename = download.suggestedFilename();
      expect(filename).toMatch(/^[A-Za-z0-9][A-Za-z0-9._-]{0,96}\.json$/);
      expect(filename).not.toContain(member);
      expect(filename.toLowerCase()).not.toMatch(/privacy|synthetic/);
      const raw = await readFile((await download.path())!, "utf8");
      expect(Buffer.byteLength(raw, "utf8")).toBeLessThanOrEqual(1 << 20);
      const exported = JSON.parse(raw);
      expect(exported.member.id).toBe(member);
      expect(exported.member.erased_at).toBeNull();
      expect(Array.isArray(exported.entries)).toBe(true);
      for (const entry of exported.entries) {
        expect(entry).not.toHaveProperty("display_name");
        expect(entry.actor_principal_id).toMatch(/^[0-9a-f-]{36}$/);
      }
      expect(exportPosts).toBe(1);
      await capture(page, "ExportDownloaded", locale);
      await assertNoPrivateMaterial(page, consoleText);

      // Explicit close + reload: the downloaded file is not retained as page state.
      await click(page, "member-privacy-close");
      await expect(dialog(page)).toHaveCount(0);
      await page.reload();
      await expect(page.getByTestId("member-export-open")).toBeVisible();
      await assertNoPrivateMaterial(page, consoleText);

      await click(page, "member-erase-open");
      await privacyRole(page, "EraseConfirm");
      // LP11/LPU05: the confirm copy must say the member's past order snapshot keeps its 訂購人 name.
      const confirmText = await dialog(page).innerText();
      expect(confirmText).toMatch(locale === "en" ? /order/i : /訂購人|订购人/);
      await capture(page, "EraseConfirm", locale);
      await click(page, "member-erase-submit");
      // Official source role Tombstone is the inline erased-member detail page, never a modal.
      const tombstone = page.locator('[data-privacy-role="Tombstone"]');
      await expect(tombstone).toBeVisible();
      await expect(page.getByTestId("member-privacy-dialog")).toHaveCount(0);
      await expect(tombstone.getByText("[erased]", { exact: true }).first()).toBeVisible();
      await expect(page.getByTestId("member-export-open")).toHaveCount(0);
      expect(erasurePosts).toBe(1);
      expect(new Set(keys).size).toBe(2);
      await page.reload();
      await expect(tombstone).toBeVisible();
      // LP11/LPU05: member text redacted, numbers and ledger rows retained, ledger note redacted.
      const pageText = await page.locator("body").innerText();
      expect(pageText).toContain("[erased]");
      expect(pageText).toContain("100");
      expect(pageText).not.toContain("PRIVACY_LEDGER_CANARY");
      expect(await tombstone.getByText("[erased]", { exact: true }).count()).toBeGreaterThanOrEqual(2);
      await expect(page.getByTestId("member-export-open")).toHaveCount(0);
      await capture(page, "Tombstone", locale);
      await assertNoPrivateMaterial(page, consoleText);
    } finally { await context.close(); }
  });
}

test("REAL_PG same-key export recovery writes one audit", async ({ browser }) => {
  test.setTimeout(120000);
  const member = corpus[6];
  const { page, context, consoleText } = await signed(browser, member);
  try {
    let posts = 0; const keys: string[] = [];
    page.on("request", r => {
      if (r.method() === "POST" && r.url().endsWith(`/members/${member}/export`)) { posts++; if (r.headers()["idempotency-key"]) keys.push(r.headers()["idempotency-key"]!); }
    });
    // First export commits in Go/PG but its response is lost at the BFF boundary.
    await page.route(`**/api/stores/${store}/members/${member}/export`, async route => {
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      // The client is still pending here, so this is the real transient role; hold only for the capture.
      await captureBoth(page, "ExportPreparing", "en");
      await route.fulfill({ status: 503, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: JSON.stringify({ code: "retry_later" }) });
    });
    await click(page, "member-export-open");
    await click(page, "member-export-submit");
    await privacyRole(page, "ExportFailed");
    await captureBoth(page, "ExportFailed", "en");
    await page.unroute(`**/api/stores/${store}/members/${member}/export`);
    // Explicit user recovery keeps the immutable key/body/target; it must not mint a new key.
    const [download] = await Promise.all([page.waitForEvent("download"), click(page, "member-export-retry")]);
    JSON.parse(await readFile((await download.path())!, "utf8"));
    expect(posts).toBe(2);
    expect(keys.length).toBe(2);
    expect(keys[0]).toBe(keys[1]);
    await assertNoPrivateMaterial(page, consoleText);
  } finally { await context.close(); }
});

test("MOCK BFF bounded export_too_large refusal", async ({ browser }) => {
  test.setTimeout(90000);
  const member = corpus[7];
  const { page, context } = await signed(browser, member, "zh-TW");
  try {
    let posts = 0;
    page.on("request", r => { if (r.method() === "POST" && r.url().endsWith(`/members/${member}/export`)) posts++; });
    // LP12 documents 409 export_too_large; a >1 MiB real corpus would need thousands of entries,
    // so this is the documented bounded BFF-only refusal fixture (no backend effect).
    await page.route(`**/api/stores/${store}/members/${member}/export`, route => route.fulfill({ status: 409, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: JSON.stringify({ code: "export_too_large" }) }));
    await click(page, "member-export-open");
    await click(page, "member-export-submit");
    await privacyRole(page, "ExportTooLarge");
    await expect(dialog(page)).toContainText("匯出資料超過 1 MiB 上限，無法匯出。");
    await captureBoth(page, "ExportTooLarge", "zh-TW");
    await expect(page.getByTestId("member-export-submit")).toHaveCount(0);
    await expect(page.getByTestId("member-export-retry")).toHaveCount(0);
    expect(posts).toBe(1);
  } finally { await context.close(); }
});

test("REAL_PG erase UNKNOWN loss refreshes read-only and never resends", async ({ browser }) => {
  test.setTimeout(120000);
  const target = corpus[8];
  const { page, context, consoleText } = await signed(browser, target);
  try {
    let posts = 0;
    page.on("request", r => { if (r.method() === "POST" && r.url().endsWith(`/members/${target}/erasure`)) posts++; });
    // The erasure commits, then the response is lost: the outcome is UNKNOWN, never "failed".
    await page.route(`**/api/stores/${store}/members/${target}/erasure`, async route => {
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      await route.fulfill({ status: 503, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: JSON.stringify({ code: "retry_later" }) });
    });
    await click(page, "member-erase-open");
    await click(page, "member-erase-submit");
    await privacyRole(page, "EraseUnknown");
    await expect(page.getByTestId("member-erase-submit")).toHaveCount(0);
    await captureBoth(page, "EraseUnknown", "en");
    expect(posts).toBe(1);
    await page.unroute(`**/api/stores/${store}/members/${target}/erasure`);
    // Refresh only reads: redacted tombstone, no second erasure request.
    await click(page, "member-privacy-refresh");
    const tombstone = page.locator('[data-privacy-role="Tombstone"]');
    await expect(tombstone).toBeVisible();
    await expect(page.getByTestId("member-privacy-dialog")).toHaveCount(0);
    await expect(tombstone.getByText("[erased]", { exact: true }).first()).toBeVisible();
    await expect(page.getByTestId("member-erase-submit")).toHaveCount(0);
    await expect(page.getByTestId("member-export-open")).toHaveCount(0);
    expect(posts).toBe(1);
    await page.reload();
    await expect(tombstone).toBeVisible();
    expect(await page.locator("body").innerText()).toContain("[erased]");
    await assertNoPrivateMaterial(page, consoleText);
  } finally { await context.close(); }
});

test("REAL_PG privacy permission and store scope boundaries", async ({ browser }) => {
  test.setTimeout(120000);
  // corpus 0..5 are erased and 8 is erased by the UNKNOWN case; 6/7 stay active and isolated.
  const member = corpus[6];
  const reader = await signed(browser, member, "en", 1440, "privacy-reader");
  try {
    await expect(reader.page.getByTestId("member-export-open")).toHaveCount(0);
    await expect(reader.page.getByTestId("member-erase-open")).toHaveCount(0);
  } finally { await reader.context.close(); }
  const { page, context, consoleText } = await signed(browser, member);
  try {
    let posts = 0;
    page.on("request", r => { if (r.method() === "POST" && r.url().includes(`/members/${member}/`)) posts++; });
    await click(page, "member-export-open");
    await privacyRole(page, "ExportPrompt");
    // Native store switch is a genuine scope change while the privacy dialog is open.
    await page.getByTestId("shell-store-selector").selectOption(alt);
    await expect(page.getByTestId("members-list")).toBeVisible();
    await expect(dialog(page)).toHaveCount(0);
    // Returning to the private store must not resurrect the prior dialog or its material.
    await page.goto(url(member));
    await expect(page.getByTestId("member-detail")).toBeVisible();
    await expect(dialog(page)).toHaveCount(0);
    expect(posts).toBe(0);
    await assertNoPrivateMaterial(page, consoleText);
  } finally { await context.close(); }
});

test("REAL_PG privacy session boundary clears dialog", async ({ browser }) => {
  test.setTimeout(120000);
  // Still-active isolated member; the boundary check must not create an extra export/erasure effect.
  const member = corpus[7];
  const { page, context, consoleText } = await signed(browser, member);
  try {
    let posts = 0;
    page.on("request", r => { if (r.method() === "POST" && r.url().includes(`/members/${member}/`)) posts++; });
    await click(page, "member-export-open");
    await expect(dialog(page)).toBeVisible();
    const other = await context.newPage();
    await other.goto(url(member));
    await other.locator("summary").filter({ hasText: "Account" }).click();
    await other.getByTestId("workspace-sign-out").click();
    await expect(dialog(page)).toHaveCount(0);
    await expect(page.getByTestId("member-export-open")).toHaveCount(0);
    expect(posts).toBe(0);
    await assertNoPrivateMaterial(page, consoleText);
    await other.close();
  } finally { await context.close(); }
});
