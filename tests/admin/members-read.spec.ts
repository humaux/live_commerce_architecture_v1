// Purpose: native-click LP2 read acceptance on the existing signed W6 Next→Go→PG stack.
// Depends on: W6 synthetic scoped loyalty fixtures/control, Playwright, frozen copy/token map.
// Used by: TestBrowserW6Customers/members-read in --browser-customers-billing.
// Invariants: no successful business response mocks, cookie injection, force clicks or DOM actions; faults are MOCK.
import { expect, test, type Browser, type Page } from "@playwright/test";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { membersCopy } from "../../apps/admin/components/members/copy";
import { shellCopy } from "../../apps/admin/src/shell-copy";
const required = (key: string) => { const v = process.env[key]; if (!v) throw new Error(`${key} required`); return v; };
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"), evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_W6UI_STORE"), member = required("LC_W6UI_MEMBER"), zero = required("LC_W6UI_ZERO_MEMBER"), erased = required("LC_W6UI_ERASED_MEMBER");
const empty = required("LC_W6UI_EMPTY_STORE"), foreign = required("LC_W6UI_FOREIGN_STORE"), foreignMember = required("LC_W6UI_FOREIGN_MEMBER");
const control = required("LC_W6UI_CONTROL_URL"), key = required("LC_W6UI_CONTROL_KEY"), actor = required("LC_W6UI_SEED_ACTOR");
const order = required("LC_W6UI_ORDER");
const marker = "LP2_SYNTHETIC_CONTACT";
type Locale = "en" | "zh-TW" | "zh-CN";
type Row = { page: string; control: string; action: string; expected: string; actual: unknown; result: "pass" | "fail"; classification: "REAL_PG" | "MOCK" };
const ledger: Row[] = [];
test.describe.configure({ mode: "serial" });
test.afterEach(async () => { await writeFile(path.join(evidence, "members-read-click-ledger.json"), JSON.stringify(ledger, null, 2)); });
async function step(page: Page, controlName: string, expected: string, action: () => Promise<unknown>, operation = "click", classification: Row["classification"] = "REAL_PG") {
  const row: Row = { page: new URL(page.url()).pathname, control: controlName, action: operation, expected, actual: null, result: "fail", classification }; ledger.push(row);
  try { row.actual = await action(); row.result = "pass"; }
  catch (error) { row.actual = "assertion failed; see Playwright trace"; throw error; }
}
async function ctl(resource: string, mode = "") {
  // Keyed fixture/fault preparation only, never evidence of a UI action.
  const r = await fetch(`${control}/${resource}`, { method: "POST", headers: { "X-W6UI-Gate-Key": key, "Content-Type": "application/json" }, body: JSON.stringify({ mode }), signal: AbortSignal.timeout(5000) });
  expect(r.status).toBe(204);
}
async function held() {
  // Read-only synchronization proves the one-shot held response was captured before changing browser scope.
  const r = await fetch(`${control}/members-held`, { method: "POST", headers: { "X-W6UI-Gate-Key": key, "Content-Type": "application/json" }, body: JSON.stringify({ mode: "" }), signal: AbortSignal.timeout(5000) });
  expect(r.status).toBe(200); return (await r.json() as { held: boolean }).held;
}
const listURL = (locale: Locale, scope = store) => `${origin}/${locale}/members?store=${scope}`;
const detailURL = (locale: Locale, id = member, scope = store) => `${origin}/${locale}/members/${id}?store=${scope}`;
const links = (page: Page) => page.getByTestId("members-list").locator(`a[href*="/members/"]`);
const entries = (page: Page) => page.getByTestId("member-ledger").locator("tbody tr");
async function signed(browser: Browser, locale: Locale = "en", width = 1440, role = "reader") {
  await ctl("actor", role);
  const context = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width, height: 900 } });
  const page = await context.newPage();
  await page.goto(`${origin}/en/`);
  await step(page, "identity sign-in", "signed native callback loads workspace", async () => {
    await page.getByRole("button", { name: "Sign in with identity service" }).click();
    await expect(page.getByTestId("shell-store-selector")).toBeVisible();
    return { selector: await page.getByTestId("shell-store-selector").count() };
  });
  await page.goto(listURL(locale)); return { page, context };
}
async function search(page: Page, q: string, keyboard = false) {
  await step(page, "search input", "draft matches typed value", async () => { await page.getByTestId("members-search-input").fill(q); await expect(page.getByTestId("members-search-input")).toHaveValue(q); return { length: q.length }; }, "fill");
  await step(page, "search submit", "POST body read completes with reset cursor and no private URL", async () => {
    const response = page.waitForResponse(r => r.request().method() === "POST" && new URL(r.url()).pathname.endsWith("/members/search"));
    if (keyboard) await page.getByTestId("members-search-input").press("Enter"); else await page.getByTestId("members-search-submit").click();
    const r = await response, body = r.request().postDataJSON() as { q: string | null; cursor: string | null };
    expect(r.status()).toBe(200); expect(body.cursor).toBeNull(); expect(body.q === (q || null), "private query stays in POST body").toBe(true);
    expect(new URL(page.url()).searchParams.toString()).toBe(`store=${store}`);
    return { status: r.status(), cursor: body.cursor };
  }, keyboard ? "keyboard Enter" : "click");
}
async function privacy(page: Page, urls: string[], consoleTexts: string[]) {
  for (const url of urls) {
    const u = new URL(url);
    expect(url.includes(marker) || url.includes("LP2%20synthetic"), "no private marker in request/history URL").toBe(false);
    if (u.pathname.includes("/members") && !u.pathname.startsWith("/api/")) {
      expect([...u.searchParams.keys()].filter(k => k !== "_rsc")).toEqual(["store"]);
    }
    expect(u.searchParams.has("q")).toBe(false);
  }
  expect(consoleTexts.some(s => s.includes(marker))).toBe(false);
  // READ ONLY: storage inspection; never alters DOM, history, cookies or application state.
  const stored = await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }));
  expect(stored).not.toContain(marker); expect(stored).not.toContain("LP2 synthetic");
}
async function snapshot(page: Page, name: string) {
  await page.screenshot({ path: path.join(evidence, `members-${name}.png`), fullPage: true });
  // READ ONLY: computed consumed styles/tokens and layout dimensions from the rendered page.
  const measurement = await page.evaluate(() => {
    const main = document.querySelector('[data-testid="members-list"], [data-testid="member-detail"]')!;
    const style = getComputedStyle(main);
    const tokens = Object.fromEntries(["page", "ink", "muted", "border", "surface", "link"].map(n => [`--ds-members-${n}`, style.getPropertyValue(`--ds-members-${n}`).trim()]));
    const input = main.querySelector("input"), anchor = main.querySelector("a"), td = main.querySelector("td"), th = main.querySelector("th");
    return { tokens, parentSurface: style.getPropertyValue("--ds-surface").trim(), parentLink: style.getPropertyValue("--ui-primary").trim(), background: style.backgroundColor, ink: style.color, input: input && { background: getComputedStyle(input).backgroundColor, border: getComputedStyle(input).borderColor }, link: anchor && getComputedStyle(anchor).color, cell: td && { border: getComputedStyle(td).borderBottomColor, background: getComputedStyle(td).backgroundColor }, muted: th && getComputedStyle(th).color, overflow: document.documentElement.scrollWidth - innerWidth };
  });
  const mapping = JSON.parse(await readFile("output/loyalty-points-admin-ui/read-slice-20261109/token-map.json", "utf8")) as { tokens: { token: string; value: string }[] };
  const expected: Record<string, string> = { "--ds-members-page": "rgb(245, 247, 250)", "--ds-members-ink": "rgb(20, 41, 66)", "--ds-members-muted": "rgb(89, 101, 121)", "--ds-members-border": "rgb(223, 229, 236)" };
  expect(measurement.background).toBe(expected["--ds-members-page"]); expect(measurement.ink).toBe(expected["--ds-members-ink"]);
  expect(measurement.tokens["--ds-members-page"]).toBe("#f5f7fa");
  expect(measurement.tokens["--ds-members-ink"]).toBe("#142942");
  expect(measurement.tokens["--ds-members-muted"]).toBe("#596579");
  expect(measurement.tokens["--ds-members-border"]).toBe("#dfe5ec");
  expect(measurement.tokens["--ds-members-surface"]).toBe(measurement.parentSurface);
  expect(measurement.tokens["--ds-members-link"]).toBe(measurement.parentLink);
  if (measurement.input) { expect(measurement.input.background).toBe("rgb(255, 255, 255)"); expect(measurement.input.border).toBe(expected["--ds-members-border"]); }
  expect(measurement.link).toBe("rgb(36, 121, 101)");
  expect(measurement.muted).toBe(expected["--ds-members-muted"]);
  expect(measurement.overflow).toBeLessThanOrEqual(1);
  await writeFile(path.join(evidence, `members-${name}-tokens.json`), JSON.stringify({ mapping, measurement, expected, visualComparison: "NOT_RUN independent exported-frame review; existing shell deviations retained; no whole-frame parity claim" }, null, 2));
}
// READ ONLY geometry; native journeys below remain the acceptance path.
async function composition(page: Page, kind: "list" | "ledger", name: string) {
  const rows = page.getByTestId(kind === "list" ? "members-list" : "member-ledger").locator("tbody tr");
  const measure = () => rows.first().evaluate(row => ({
    height: row.getBoundingClientRect().height,
    fields: Array.from(row.querySelectorAll("td")).map(cell => {
      const b = cell.getBoundingClientRect();
      return { x: b.x, y: b.y, width: b.width, height: b.height, text: cell.textContent, label: cell.querySelector("[data-label]")?.getAttribute("data-label") };
    }),
    overflow: document.documentElement.scrollWidth - innerWidth,
  }));
  const grouped = (m: Awaited<ReturnType<typeof measure>>) => {
    const [a, b] = kind === "list" ? [0, 2] : [1, 2];
    return Math.abs(m.fields[a].y - m.fields[b].y) < 1 && m.fields[b].x >= m.fields[a].x + m.fields[a].width - 1;
  };
  const green = await measure();
  expect(green.fields).toHaveLength(kind === "list" ? 4 : 6);
  expect(green.fields.every(f => f.label && f.text?.trim())).toBe(true);
  expect(green.overflow).toBe(0);
  expect(grouped(green), "primary record fields share a compact row").toBe(true);
  // Deliberate presentation-only fault: restore e6296 full-width mobile stacking.
  // addStyleTag is calibration, never a substitute for native user acceptance.
  const fault = await page.addStyleTag({ content: `
    [data-testid="${kind === "list" ? "members-list" : "member-ledger"}"] tbody tr { display:block !important; }
    [data-label] { display:grid !important; grid-template-columns:minmax(80px,1fr) minmax(0,2fr) !important; gap:12px !important; padding:8px !important; }
  ` });
  let red: Awaited<ReturnType<typeof measure>> | undefined;
  let redFailure = "";
  try {
    red = await measure();
    try { expect(grouped(red), "primary record fields share a compact row").toBe(true); }
    catch (error) { redFailure = error instanceof Error ? error.message : String(error); }
    expect(redFailure, "original stacking must turn the same grouping assertion RED").not.toBe("");
    expect(red.height, "original stack is measurably taller on the same real fixture").toBeGreaterThan(green.height);
  } finally { await fault.evaluate(node => node.remove()); }
  const restored = await measure();
  expect(grouped(restored)).toBe(true);
  expect(restored.height).toBeCloseTo(green.height, 0);
  await writeFile(path.join(evidence, `members-${name}-composition.json`), JSON.stringify({ green, red, redFailure, restored, calibration: "original stacking rejected; restored grouping accepted" }, null, 2));
}
async function stateShot(page: Page, name: string) {
  await page.screenshot({ path: path.join(evidence, `members-state-${name}.png`), fullPage: true });
}
async function logout(page: Page, locale: Locale) {
  const sc = shellCopy[locale];
  await page.locator("summary").filter({ hasText: sc.account }).click();
  await expect(page.getByTestId("workspace-sign-out")).toBeVisible();
  await page.getByTestId("workspace-sign-out").click();
  await expect(page).toHaveURL(new RegExp(`/${locale}/?$`));
  await expect(page.locator('form[action="/api/auth/login"] button')).toBeVisible();
}

for (const locale of ["zh-TW", "zh-CN", "en"] as const) for (const width of [1440, 375]) {
  test(`REAL_PG members read ${locale}/${width}`, async ({ browser }) => {
    const c = membersCopy[locale], sc = shellCopy[locale];
    const { page, context } = await signed(browser, locale, width);
    const urls: string[] = [], consoleTexts: string[] = []; let mutation = 0; let initialEntryIDs: string[] = [];
    page.on("request", r => { urls.push(r.url()); if (new URL(r.url()).pathname.includes("/members") && !["GET", "HEAD"].includes(r.method()) && !r.url().endsWith("/members/search")) mutation++; });
    page.on("framenavigated", f => { if (f === page.mainFrame()) urls.push(f.url()); });
    page.on("console", m => consoleTexts.push(m.text()));
    try {
      await expect(links(page)).toHaveCount(20);
      await snapshot(page, `${locale}-${width}-list`);
      if (width === 375) await composition(page, "list", `${locale}-list`);
      if (width === 375) await step(page, "mobile scroll", "keyboard reaches final visible row and pagination", async () => {
        await page.getByTestId("members-search-input").press("Tab");
        const pagination = page.getByTestId("members-load-more");
        for (let tabs = 0; tabs < 30 && !(await pagination.evaluate(node => node === document.activeElement)); tabs++) await page.keyboard.press("Tab");
        await expect(pagination).toBeFocused();
        await page.keyboard.press("End");
        await page.getByTestId("members-load-more").scrollIntoViewIfNeeded();
        await expect(page.getByTestId("members-load-more")).toBeInViewport();
        return { paginationVisible: await page.getByTestId("members-load-more").isVisible() };
      }, "keyboard Tab/End/scroll");
      await ctl("fault", "members-search-503");
      await step(page, "list next failure", "503 preserves twenty rows and explicit retry", async () => {
        await page.getByTestId("members-load-more").click(); await expect(page.getByTestId("members-list").getByRole("alert")).toContainText(c.network); await expect(links(page)).toHaveCount(20);
        await stateShot(page, `${locale}-${width}-list-page-error`);
        return { rows: await links(page).count(), retry: await page.getByTestId("members-load-more").innerText() };
      }, "click", "MOCK");
      await step(page, "list next retry", "23 unique members and terminal pagination", async () => {
        await page.getByTestId("members-load-more").click(); await expect(links(page)).toHaveCount(23);
        const hrefs = await links(page).evaluateAll(nodes => nodes.map(n => n.getAttribute("href"))); // READ only IDs.
        expect(new Set(hrefs).size).toBe(23); await expect(page.getByTestId("members-load-more")).toHaveCount(0); return { rows: hrefs.length, unique: new Set(hrefs).size };
      });
      await search(page, marker, true); await expect(links(page)).toHaveCount(1);
      await search(page, "NO_SYNTHETIC_MATCH"); await expect(page.getByText(c.queryEmpty, { exact: true })).toBeVisible();
      await stateShot(page, `${locale}-${width}-query-empty`);
      await search(page, "LP2 synthetic"); await expect(links(page)).toHaveCount(20);
      await search(page, ""); await expect(links(page)).toHaveCount(20);
      await step(page, "native member detail", "scoped detail opens with fifty ledger entries", async () => {
        // Main member is oldest and appears on page two: navigate through the actual link.
        await page.getByTestId("members-load-more").click(); await expect(links(page)).toHaveCount(23);
        const response = page.waitForResponse(r => r.request().method() === "GET" && new URL(r.url()).pathname === `/api/stores/${store}/members/${member}`);
        await page.getByTestId("members-list").locator(`a[href*="/members/${member}?"]`).click();
        const detailResponse = await response;
        expect(detailResponse.status()).toBe(200);
        expect(detailResponse.headers()["content-type"]).toMatch(/^application\/json(?:\s*;|$)/i);
        const data = await detailResponse.json() as { entries: { id: string }[] }; initialEntryIDs = data.entries.map(e => e.id);
        await expect(entries(page)).toHaveCount(50);
        await expect(page.getByTestId("member-detail")).toContainText(`${c.staffID}: ${actor}`);
        return { rows: await entries(page).count(), scope: new URL(page.url()).searchParams.get("store") };
      });
      await snapshot(page, `${locale}-${width}-detail`);
      if (width === 375) await composition(page, "ledger", `${locale}-ledger`);
      await step(page, "native scoped order link", "real checkout order detail renders after native navigation", async () => {
        const link = page.getByTestId("member-ledger").getByRole("link", { name: order, exact: true });
        await expect(link).toHaveAttribute("href", `/${locale}/orders/${order}?store=${store}`);
        await link.click(); await expect(page).toHaveURL(new RegExp(`/${locale}/orders/${order}\\?store=${store}`));
        await expect(page.getByTestId("order-detail")).toBeVisible();
        await page.reload(); await expect(page.getByTestId("order-detail")).toBeVisible();
        await page.goBack(); await expect(entries(page)).toHaveCount(50);
        return { pathname: `/${locale}/orders/${order}`, scoped: true };
      });
      await ctl("fault", "members-ledger-503");
      await step(page, "ledger next failure", "503 preserves fifty rows", async () => { await page.getByTestId("member-ledger-load-more").click(); await expect(page.getByTestId("member-ledger").getByRole("alert")).toContainText(c.network); await expect(entries(page)).toHaveCount(50); return { rows: await entries(page).count() }; }, "click", "MOCK");
      await step(page, "ledger next retry", "52 unique ledger rows; no more control", async () => {
        const response = page.waitForResponse(r => r.request().method() === "GET" && new URL(r.url()).searchParams.has("ledger_cursor"));
        await page.getByTestId("member-ledger-load-more").click(); await expect(entries(page)).toHaveCount(52);
        const data = await (await response).json() as { entries: { id: string }[] };
        const ids = [...initialEntryIDs, ...data.entries.map(e => e.id)];
        expect(ids.every(Boolean)).toBe(true); expect(new Set(ids).size).toBe(52);
        await expect(page.getByTestId("member-ledger-load-more")).toHaveCount(0); return { rows: ids.length, unique: new Set(ids).size };
      });
      await step(page, "reload authoritative detail", "balance 52 and first ledger page persist", async () => { await page.reload(); await expect(entries(page)).toHaveCount(50); await expect(page.getByTestId("member-detail").locator("strong")).toHaveText("52"); return { balance: await page.getByTestId("member-detail").locator("strong").innerText() }; }, "reload");
      // Direct routes are setup; all visible controls are clicked separately.
      for (const [id, expectedBalance] of [[zero, "0"], [erased, "1"]]) {
        await page.goto(detailURL(locale, id)); await expect(page.getByTestId("member-detail").locator("strong")).toHaveText(expectedBalance);
        if (id === erased) await expect(page.getByText(c.erased, { exact: true })).toBeVisible();
        else { await expect(page.getByText(c.noEntries, { exact: true })).toBeVisible(); await expect(page.getByTestId("member-detail").locator("dd").filter({ hasText: c.dash })).toHaveCount(2); }
        await page.reload(); await expect(page.getByTestId("member-detail").locator("strong")).toHaveText(expectedBalance);
        await stateShot(page, `${locale}-${width}-${id === erased ? "erased" : "no-entries"}`);
      }
      await step(page, "member back", "list defaults to twenty rows with blank draft", async () => { await page.getByTestId("member-back").click(); await expect(links(page)).toHaveCount(20); await expect(page.getByTestId("members-search-input")).toHaveValue(""); return { rows: await links(page).count() }; });
      await step(page, "native browser Back", "returns to erased detail then list without private history state", async () => { await page.goBack(); await expect(page.getByTestId("member-ledger")).toBeVisible(); await page.goForward(); await expect(links(page)).toHaveCount(20); return { pathname: new URL(page.url()).pathname }; }, "back/forward");
      if (width === 375) await step(page, "menu open/Escape", "navigation opens and Escape closes", async () => { await page.getByRole("button", { name: sc.openMenu, exact: true }).click(); await expect(page.getByRole("button", { name: sc.openMenu, exact: true })).toHaveAttribute("aria-expanded", "true"); await page.keyboard.press("Escape"); await expect(page.getByRole("button", { name: sc.openMenu, exact: true })).toHaveAttribute("aria-expanded", "false"); return { expanded: false }; }, "click/keyboard Escape");
      await step(page, "locale selector", "native selection changes locale and resets draft", async () => { const next = locale === "en" ? "zh-TW" : "en"; await page.getByTestId("locale-switch").selectOption(next); await expect(page).toHaveURL(new RegExp(`/${next}/members`)); await expect(links(page)).toHaveCount(20); await page.getByTestId("locale-switch").selectOption(locale); await expect(page).toHaveURL(new RegExp(`/${locale}/members`)); return { restoredLocale: locale }; }, "selectOption");
      await step(page, "store selector", "authorized empty store has distinct empty state", async () => {
        await page.getByTestId("shell-store-selector").selectOption(empty); await expect(page).toHaveURL(new RegExp(`store=${empty}`));
        // Store switching intentionally lands on overview; enter members through native navigation.
        if (width === 375) await page.getByRole("button", { name: sc.openMenu, exact: true }).click();
        await page.getByTestId("nav-members").click(); await expect(page.getByText(c.empty, { exact: true })).toBeVisible();
        await stateShot(page, `${locale}-${width}-empty-store`);
        return { store: new URL(page.url()).searchParams.get("store"), empty: true };
      }, "selectOption/click");
      await privacy(page, urls, consoleTexts); expect(mutation).toBe(0);
      await step(page, "account/logout", "signout removes private member content", async () => { await logout(page, locale); await expect(links(page)).toHaveCount(0); return { privateRows: await links(page).count() }; });
    } finally { await context.close(); }
  });
}

test("MOCK read permission revocation and opaque foreign refusal", async ({ browser }) => {
  const { page, context } = await signed(browser);
  try {
    await expect(links(page)).toHaveCount(20); await ctl("deny-read");
    await step(page, "revoked read/reload", "no private rows after server grant revocation", async () => { await page.reload(); await expect(page.getByTestId("route-forbidden")).toBeVisible(); await expect(links(page)).toHaveCount(0); return { privateRows: await links(page).count() }; }, "reload", "MOCK");
    await stateShot(page, "en-1440-permission-revoked");
    await ctl("restore-read"); await page.goto(listURL("en")); await expect(links(page)).toHaveCount(20);
    for (const [scope, id] of [[store, foreignMember], [foreign, foreignMember]]) {
      const response = await page.goto(detailURL("en", id, scope));
      if (scope === foreign) expect(response?.status()).toBe(404);
      else await expect(page.getByText(membersCopy.en.notFound, { exact: true })).toBeVisible();
      await expect(page.getByTestId("member-ledger")).toHaveCount(0); await expect(page.locator("body")).not.toContainText("foreign synthetic member");
    }
  } finally { await ctl("restore-read"); await context.close(); }
  const denied = await signed(browser, "en", 1440, "none");
  try { await expect(denied.page.getByTestId("route-forbidden")).toBeVisible(); await expect(links(denied.page)).toHaveCount(0); await expect(denied.page.getByTestId("member-ledger")).toHaveCount(0); } finally { await denied.context.close(); }
});

for (const target of ["query", "store", "logout", "detail-logout"] as const) test(`MOCK held successful read cannot overwrite ${target} boundary`, async ({ browser }) => {
  const { page, context } = await signed(browser);
  try {
    await expect(links(page)).toHaveCount(20);
    if (target === "detail-logout") { await page.goto(detailURL("en")); await expect(entries(page)).toHaveCount(50); }
    await ctl("fault", "members-hold");
    if (target === "detail-logout") await page.getByTestId("member-ledger-load-more").click();
    else { await page.getByTestId("members-search-input").fill(marker); await page.getByTestId("members-search-submit").click(); }
    await expect.poll(held, { timeout: 8000 }).toBe(true);
    await step(page, `held read/${target}`, "released prior success cannot revive old private rows", async () => {
      if (target === "query") { await search(page, "NO_SYNTHETIC_MATCH"); await expect(page.getByText(membersCopy.en.queryEmpty, { exact: true })).toBeVisible(); }
      else if (target === "store") { await page.getByTestId("shell-store-selector").selectOption(empty); await expect(page).toHaveURL(new RegExp(`store=${empty}`)); }
      else await logout(page, "en");
      await ctl("members-release");
      // Bounded observation window after releasing the old success; no page reload that could mask resurrection.
      await page.waitForTimeout(500);
      if (target === "query") { await expect(links(page)).toHaveCount(0); await expect(page.getByTestId("members-search-input")).toHaveValue("NO_SYNTHETIC_MATCH"); }
      else { await expect(links(page)).toHaveCount(0); await expect(entries(page)).toHaveCount(0); }
      return { privateRows: await links(page).count(), boundary: target };
    }, "click/selectOption/release", "MOCK");
  } finally { await ctl("members-release"); await context.close(); }
});

for (const target of ["list", "detail"] as const) test(`MOCK 401 clears ${target} private content`, async ({ browser }) => {
 const { page, context } = await signed(browser);
 try {
  await expect(links(page)).toHaveCount(20);
  if (target === "detail") { await page.goto(detailURL("en")); await expect(entries(page)).toHaveCount(50); }
  await ctl("members-revoke");
  await step(page, `${target} 401`, "refused native read clears private content", async () => {
   const response = page.waitForResponse(r => new URL(r.url()).pathname.includes("/members") && r.status() === 401);
   if(target === "detail") await page.getByTestId("member-ledger-load-more").click(); else await page.getByTestId("members-search-submit").click();
   expect((await response).status()).toBe(401); await expect(page.getByTestId("shell-session-expired")).toBeVisible();
   await expect(links(page)).toHaveCount(0); await expect(entries(page)).toHaveCount(0);
   await stateShot(page, `en-1440-${target}-session-expired`);
   return { status: 401, listRows: await links(page).count(), ledgerRows: await entries(page).count() };
  }, "click", "MOCK");
 } finally { await context.close(); }
});

test("REAL_PG cursor binds authorized store; direct refusal probes supplement native UI", async ({ browser }) => {
 const { page, context } = await signed(browser);
 try {
  await expect(links(page)).toHaveCount(20);
  // Fixture authority probe only: context supplies its genuine sign-in cookies, no cookie injection or UI replacement.
  const csrf = (await context.cookies()).find(c => c.name === "__Host-commerce_csrf")?.value;
  expect(csrf).toBeTruthy();
  const post = (scope: string, cursor: string | null) => page.request.post(`${origin}/api/stores/${scope}/members/search`, { headers: { Origin: origin, "X-CSRF-Token": csrf! }, data: { q: null, include_erased: false, cursor, limit: 20 } });
  const first = await post(store,null); expect(first.status()).toBe(200);
  const cursor = (await first.json() as {next_cursor:string}).next_cursor; expect(cursor).toBeTruthy();
  const wrongStore = await post(empty,cursor); expect(wrongStore.status()).toBe(422);
  const foreignScope = await post(foreign,cursor); expect(foreignScope.status()).toBe(404);
 } finally { await context.close(); }
});

test("MOCK initial search 503 requires explicit native retry", async ({ browser }) => {
 const { page, context } = await signed(browser);
 try {
  await expect(links(page)).toHaveCount(20); await ctl("fault","members-search-503");
  await step(page,"initial search Retry","visible failure recovers only on explicit retry",async()=>{
   await page.reload(); await expect(page.getByText(membersCopy.en.network,{exact:true})).toBeVisible();
   await expect(links(page)).toHaveCount(0); await page.getByRole("button",{name:membersCopy.en.retry,exact:true}).click();
   await expect(links(page)).toHaveCount(20); return { rows:await links(page).count() };
  },"reload/click","MOCK");
 } finally { await context.close(); }
});
