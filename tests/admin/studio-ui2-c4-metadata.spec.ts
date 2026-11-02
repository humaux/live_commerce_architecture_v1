// Purpose: seven isolated C4 metadata REAL PG contracts with private failure evidence.
// Depends on: existing signed Studio fixture, private-evidence guard, native Playwright and parity literals.
// Used by: browser_studio_ui_test.go and the registered studio-ui suite, after the legacy Studio spec.
// Invariants: I01/I02/I06/I11/I14; four real logins/creates, no live effect or legacy evidence weakening.
import { parityWords } from "./parity-live-sessions";
import { expect, type Page, type BrowserContext } from "@playwright/test";
import { randomBytes } from "node:crypto";
import { test as c4PrivateTest } from "./inbox-private-evidence";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const store = required("LC_BROWSER_STUDIO_STORE");
const preparedSession = required("LC_BROWSER_STUDIO_SESSION");
const readOnlyToken = required("LC_BROWSER_STUDIO_READONLY_TOKEN");
const cookieName = "__Host-commerce_session";

// Worker-scoped evidence options belong at file scope; the legacy spec keeps its own policy.
c4PrivateTest.use({ baseURL: origin, headless: false, trace: "off", screenshot: "off", video: "off" });
c4PrivateTest.setTimeout(240_000);

async function clickStudioNav(page: Page) {
  await expect(async () => {
    const group = page.getByTestId("nav-group-live");
    if (await group.getAttribute("aria-expanded") !== "true") await group.click({ timeout: 15000 });
    await expect(page.getByTestId("nav-studio")).toBeVisible({ timeout: 1_000 });
  }).toPass({ timeout: 30_000 });
  await page.getByTestId("nav-studio").click({ timeout: 15000 });
}

async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click({ timeout: 15000 });
  await page.getByTestId("nav-group-live").waitFor({ state: "attached" });
  const menu = page.locator('button[aria-controls="workspace-navigation"]');
  if (await menu.isVisible()) await menu.click({ timeout: 15000 });
  await expect(page.getByTestId("nav-group-live")).toBeVisible();
  await clickStudioNav(page);
  await expect(page.getByTestId("merchant-studio")).toBeVisible();
  await expect(page.getByRole("heading", { level: 1, name: "Live sessions" })).toBeVisible();
  await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
}

async function setSession(context: BrowserContext, token: string) {
  const url = origin.replace(/^http:/, "https:");
  await context.addCookies([
    { name: cookieName, value: token, url, secure: true, httpOnly: true, sameSite: "Lax" },
    { name: "__Host-commerce_csrf", value: randomBytes(32).toString("base64url"),
      url, secure: true, httpOnly: false, sameSite: "Lax" },
  ]);
}

// C4 append-only REAL PG cases. Frozen literals are independent of production dictionaries.
// Four writer cases below mean exactly four extra signedLogin calls and four UI POST creates.
// The parent owns the corresponding exact Go cohort-count update; old counters/indices are retained.
const c4EditorWords = {
  en: { edit: "Edit draft", name: "Scene name", schedule: "Scheduled time (Taipei time, optional)", aspect: "Canvas ratio", create: "Create draft", save: "Save draft", cancel: "Cancel", prepare: "Live preparation", keep: "Keep editing", discard: "Discard changes", helper: "Before leaving, keep editing or discard these changes." },
  "zh-TW": { edit: "編輯草稿", name: "場次名稱", schedule: "預定時間（台北時間，選填）", aspect: "畫面比例", create: "建立草稿", save: "儲存草稿", cancel: "取消", prepare: "直播準備", keep: "繼續編輯", discard: "放棄變更", helper: "離開前可繼續編輯，或放棄本次變更。" },
  "zh-CN": { edit: "编辑草稿", name: "场次名称", schedule: "计划时间（台北时间，可选）", aspect: "画幅", create: "建立草稿", save: "保存草稿", cancel: "取消", prepare: "直播准备", keep: "继续编辑", discard: "放弃更改", helper: "离开前可继续编辑，或放弃本次更改。" },
} as const;
const c4EditorAction = (page: Page, name: string | RegExp) => page.getByRole("button", { name, exact: true }).or(page.getByRole("link", { name, exact: true }));
async function c4OwnedDraft(page: Page, locale: keyof typeof c4EditorWords, title: string) {
  const c = c4EditorWords[locale];
  await signedLogin(page); // One real signed login per writer case; no borrowed authority.
  await page.goto(`/${locale}/studio?store=${store}`);
  await page.getByRole("button", { name: parityWords[locale].create, exact: true }).click({ timeout: 15000 });
  await page.getByLabel(c.name, { exact: true }).fill(title);
  const [response] = await Promise.all([
    page.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === `/api/stores/${store}/live-sessions`, { timeout: 15000 }),
    page.getByRole("button", { name: c.create, exact: true }).click({ timeout: 15000 }),
  ]);
  expect(response.status()).toBe(200);
  expect((response.request().headers()["idempotency-key"] ?? "").length > 0).toBe(true);
  const draft = await response.json() as { session_id: string; title: string; scheduled_at: string | null; aspect_ratio: string; state: string; version: number };
  expect(draft).toMatchObject({ title, scheduled_at: null, aspect_ratio: "9:16", state: "DRAFT", version: 1 });
  expect(draft.session_id).toMatch(/^[0-9a-f-]{36}$/);
  await expect(page.getByLabel(c.name, { exact: true })).toHaveValue(title);
  await c4PrivateTest.info().attach("C4-owned-draft", { body: JSON.stringify({ class: "REAL_PG", status: response.status(), version: draft.version, originalKeyPresent: true }), contentType: "application/json" });
  return draft;
}

// C4 dirty186 projection begin (card c879): failure-only, one read, no retry or product writes.
import { writeFileSync } from "node:fs";
async function c4Dirty186Projection(page: Page, locale: keyof typeof c4EditorWords, scene: string) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    const sample = page.evaluate(({ locale, store, scene, words }) => {
      const url = new URL(location.href);
      const count = (nodes: Element[]) => Math.min(nodes.length, 16);
      const headings = Array.from(document.querySelectorAll("h1,h2,h3"));
      const buttons = Array.from(document.querySelectorAll<HTMLButtonElement>("button"));
      const enabled = (label: string) => {
        const matches = buttons.filter(node => node.textContent?.trim() === label);
        return matches.length === 1 ? !matches[0]!.disabled : null;
      };
      const tag = document.activeElement?.tagName;
      const scroll = (value: number) => Number.isFinite(value) ? Math.max(-1_000_000, Math.min(1_000_000, Math.round(value))) : null;
      // READ/MEASURE: only these fixed enums, booleans and bounded numbers leave the page.
      // Never return raw URL/DOM/text, field values, credentials, or router/history state.
      return {
        pathname: url.pathname === `/${locale}/studio` ? "STUDIO" : url.pathname === `/${locale}/studio/claims` ? "CLAIMS" : "OTHER",
        hash: url.hash === "#edit" ? "EDIT" : url.hash === "#preparation" ? "PREPARATION" : "OTHER",
        fragmentCount: Math.min(url.hash.split("#").length - 1, 8),
        fragmentKinds: url.hash ? url.hash.slice(1).split("#").slice(0, 8).map(fragment => fragment === "edit" ? "EDIT" : fragment === "preparation" ? "PREPARATION" : "OTHER") : [],
        sameStore: url.searchParams.get("store") === store, sameScene: url.searchParams.get("scene") === scene,
        preparationHeadingCount: count(headings.filter(node => node.textContent?.trim() === words.prepare)),
        editorHeadingCount: count(Array.from(document.querySelectorAll(".studio-editor h2"))),
        dialogCount: count(Array.from(document.querySelectorAll('[role="dialog"]'))),
        dirtyMarkerCount: count(Array.from(document.querySelectorAll(".studio-dirty"))),
        cancelEnabled: enabled(words.cancel), editEnabled: enabled(words.edit), saveEnabled: enabled(words.save),
        activeElement: tag && ["INPUT", "SELECT", "BUTTON", "A", "BODY"].includes(tag) ? tag : tag ? "OTHER" : "NONE",
        scrollX: scroll(window.scrollX), scrollY: scroll(window.scrollY),
      };
    }, { locale, store, scene, words: c4EditorWords[locale] }).then(
      value => ({ status: "OK" as const, ...value }), () => ({ status: "UNAVAILABLE" as const }));
    const result = await Promise.race([sample, new Promise<{ status: "TIMEOUT" }>(resolve => {
      timer = setTimeout(() => resolve({ status: "TIMEOUT" }), 100);
    })]);
    // The 100ms budget covers the read only; this one synchronous evidence write follows it.
    // CLI --reporter=list suppresses config JSON output, so reuse the existing private JSON-file pattern.
    // Both sample outcomes are consumed; late completion cannot write or perform another read.
    writeFileSync(`${required("LC_BROWSER_EVIDENCE")}/c4-dirty186-${locale}.json`,
      JSON.stringify({ class: "REAL_PG", readBudgetMs: 100, read: result }),
      { encoding: "utf8", mode: 0o600, flush: true });
  } catch { /* Diagnostic errors must not replace the original assertion error. */ }
  finally { if (timer !== undefined) clearTimeout(timer); }
}
// C4 dirty186 projection end.

c4PrivateTest.describe("C4 metadata independent REAL PG contracts", () => {
  // New C4 artifacts contain synthetic IDs/counts only; no credentials, raw DOM or headers.

  c4PrivateTest("C4 metadata native Edit saves three fields atomically and clears optional Taipei time", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    const original = "C4 metadata owned draft", changed = "C4 metadata persisted draft";
    const draft = await c4OwnedDraft(page, "en", original);
    const path = `/api/stores/${store}/live-sessions/${draft.session_id}`;
    const writes: { method: string; path: string; body: unknown }[] = [];
    page.on("request", wire => {
      const pathname = new URL(wire.url()).pathname;
      if (pathname.startsWith("/api/stores/") && !["GET", "HEAD"].includes(wire.method()))
        writes.push({ method: wire.method(), path: pathname, body: wire.postDataJSON() });
    });
    await clickStudioNav(page);
    await page.locator(".studio-scene-list .studio-scene").filter({ has: page.getByText(original, { exact: true }) }).click({ timeout: 15000 });
    await expect(page.getByRole("heading", { name: "Live preparation", exact: true })).toBeVisible();
    await c4PrivateTest.step("EXPECTED_RED_ENTRY: existing created draft must have native Edit", async () => {
      await c4EditorAction(page, "Edit draft").click({ timeout: 15000 });
      expect(new URL(page.url()).hash).toBe("#edit");
      expect(new URL(page.url()).searchParams.get("scene")).toBe(draft.session_id);
      await expect(page.getByLabel("Scene name", { exact: true })).toHaveValue(original);
    });
    const save = page.getByRole("button", { name: "Save draft", exact: true });
    await expect(save).toBeDisabled();
    await page.getByLabel("Scene name", { exact: true }).fill("");
    await save.click({ timeout: 15000 });
    await expect(page.getByText("Enter a scene name of 1–200 characters without leading/trailing spaces.", { exact: true })).toBeVisible();
    expect(writes).toHaveLength(0);
    await page.getByLabel("Scene name", { exact: true }).fill(changed);
    await page.getByLabel("Scheduled time (Taipei time, optional)", { exact: true }).fill("2030-01-01T08:00");
    await page.getByLabel("Canvas ratio", { exact: true }).selectOption("16:9");
    const [saved] = await Promise.all([
      page.waitForResponse(response => response.request().method() === "PATCH" && new URL(response.url()).pathname === path, { timeout: 15000 }),
      save.click({ timeout: 15000 }),
    ]);
    expect(saved.status()).toBe(200);
    expect(writes).toEqual([{ method: "PATCH", path, body: { title: changed, scheduled_at: "2030-01-01T00:00:00.000Z", aspect_ratio: "16:9", expected_version: draft.version } }]);
    const result = await saved.json() as { session_id: string; title: string; scheduled_at: string; aspect_ratio: string; version: number };
    expect(result).toMatchObject({ session_id: draft.session_id, title: changed, aspect_ratio: "16:9", version: draft.version + 1 });
    expect(new Date(result.scheduled_at).toISOString()).toBe("2030-01-01T00:00:00.000Z");
    const [reloaded] = await Promise.all([
      page.waitForResponse(response => response.request().method() === "GET" && new URL(response.url()).pathname === path && response.status() === 200, { timeout: 15000 }),
      page.reload(),
    ]);
    const detail = await reloaded.json() as { draft: typeof result };
    expect(detail.draft).toMatchObject(result);
    await expect(page.getByLabel("Scene name", { exact: true })).toHaveValue(changed);
    await expect(page.getByLabel("Scheduled time (Taipei time, optional)", { exact: true })).toHaveValue("2030-01-01T08:00");
    await expect(page.getByLabel("Canvas ratio", { exact: true })).toHaveValue("16:9");
    await expect(save).toBeDisabled();
    await page.getByLabel("Scheduled time (Taipei time, optional)", { exact: true }).fill("");
    const [cleared] = await Promise.all([
      page.waitForResponse(response => response.request().method() === "PATCH" && new URL(response.url()).pathname === path, { timeout: 15000 }),
      save.click({ timeout: 15000 }),
    ]);
    expect(cleared.status()).toBe(200);
    expect(await cleared.json()).toMatchObject({ session_id: draft.session_id, title: changed, scheduled_at: null, aspect_ratio: "16:9", version: draft.version + 2 });
    expect(writes).toHaveLength(2);
    expect(writes[1]).toEqual({ method: "PATCH", path, body: { title: changed, scheduled_at: null, aspect_ratio: "16:9", expected_version: draft.version + 1 } });
    await page.reload();
    await expect(page.getByLabel("Scheduled time (Taipei time, optional)", { exact: true })).toHaveValue("");
    await expect(page.getByLabel("Canvas ratio", { exact: true })).toHaveValue("16:9");
    expect(writes).toHaveLength(2); // Exactly one PATCH per Save, no lifecycle/claim/media command.
  });

  for (const [locale, width, exitKind] of [["en", 1440, "Back"], ["zh-TW", 390, "Cancel"], ["zh-CN", 390, "Back"]] as const) {
    c4PrivateTest(`C4 metadata dirty ${exitKind} Keep Escape Discard ${locale} ${width}`, async ({ page }) => {
      const c = c4EditorWords[locale], title = `C4 dirty owned ${locale}`, unsaved = `C4 dirty retained ${locale}`;
      await page.setViewportSize({ width, height: width === 1440 ? 900 : 844 });
      const draft = await c4OwnedDraft(page, locale, title);
      // The create result already opens the existing editor. This is guard setup,
      // not evidence for the separate preparation-to-Edit entry above.
      const writes: string[] = [];
      page.on("request", wire => { if (new URL(wire.url()).pathname.startsWith("/api/stores/") && !["GET", "HEAD"].includes(wire.method())) writes.push(wire.method()); });
      const name = page.getByLabel(c.name, { exact: true });
      await name.fill(unsaved);
      const leave = exitKind === "Cancel" ? c4EditorAction(page, c.cancel) : c4EditorAction(page, new RegExp(`^(?:‹\\s*)?${c.prepare}$`));
      const dialog = page.getByRole("dialog");
      await c4PrivateTest.step("EXPECTED_RED_DIRTY: native departure opens frozen confirmation", async () => {
        await leave.click({ timeout: 15000 });
        await expect(dialog).toBeVisible();
        await expect(dialog.getByText(c.helper, { exact: true })).toBeVisible();
        await dialog.getByRole("button", { name: c.keep, exact: true }).click({ timeout: 15000 });
        await expect(dialog).toHaveCount(0);
        await expect(name).toHaveValue(unsaved); await expect(name).toBeFocused();
        await leave.click({ timeout: 15000 }); await expect(dialog).toBeVisible();
        await page.keyboard.press("Escape");
        await expect(dialog).toHaveCount(0);
        await expect(name).toHaveValue(unsaved); await expect(name).toBeFocused();
        await leave.click({ timeout: 15000 }); await expect(dialog).toBeVisible();
        await dialog.getByRole("button", { name: c.discard, exact: true }).click({ timeout: 15000 });
      });
      await expect(page.getByRole("heading", { name: c.prepare, exact: true })).toBeVisible();
      expect(new URL(page.url()).searchParams.get("scene")).toBe(draft.session_id);
      expect(new URL(page.url()).searchParams.get("store")).toBe(store);
      expect(writes).toHaveLength(0);
      await c4EditorAction(page, c.edit).click({ timeout: 15000 });
      await expect(name).toHaveValue(title); // Re-entered persisted value, never discarded dirty input saved.
      await page.reload(); await expect(name).toHaveValue(title);
      await c4EditorAction(page, c.cancel).click({ timeout: 15000 });
      // C4 dirty186 wrapper begin: original assertion below is byte-preserved.
      try {
      await expect(page.getByRole("heading", { name: c.prepare, exact: true })).toBeVisible();
      } catch (error) {
        await c4Dirty186Projection(page, locale, draft.session_id);
        throw error;
      }
      // C4 dirty186 wrapper end.
      await expect(dialog).toHaveCount(0); // Clean Cancel requires no confirmation.
      expect(writes).toHaveLength(0);
      await c4PrivateTest.info().attach("C4-dirty-zero-write", { body: JSON.stringify({ class: "REAL_PG", locale, width, exitKind, writes: writes.length, restoredPersistedTitle: true }), contentType: "application/json" });
    });

    c4PrivateTest(`C4 metadata read-only three fields reject writes ${locale} ${width}`, async ({ browser }) => {
      const c = c4EditorWords[locale], context = await browser.newContext({ baseURL: origin, viewport: { width, height: 900 } });
      try {
        await setSession(context, readOnlyToken);
        const page = await context.newPage(), writes: string[] = [];
        page.on("request", wire => { if (new URL(wire.url()).pathname.startsWith("/api/stores/") && !["GET", "HEAD"].includes(wire.method())) writes.push(wire.method()); });
        const [response] = await Promise.all([
          page.waitForResponse(response => response.request().method() === "GET" && new URL(response.url()).pathname === `/api/stores/${store}/live-sessions/${preparedSession}` && response.status() === 200, { timeout: 15000 }),
          page.goto(`/${locale}/studio?store=${store}&scene=${preparedSession}#edit`),
        ]);
        expect((await response.json() as { can_manage: boolean }).can_manage).toBe(false);
        for (const label of [c.name, c.schedule, c.aspect]) await expect(page.getByLabel(label, { exact: true })).toBeDisabled();
        await expect(page.getByRole("button", { name: c.save, exact: true })).toBeDisabled();
        await page.reload();
        for (const label of [c.name, c.schedule, c.aspect]) await expect(page.getByLabel(label, { exact: true })).toBeDisabled();
        expect(writes).toHaveLength(0);
      } finally { await context.close(); }
    });
  }
});
