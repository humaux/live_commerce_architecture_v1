// SDB admin gate of unit store-design (contracts/storefront-v2.md section B; independent test author, evidence label BROWSER, IdP = MOCK).
// BFF routes exercised through the UI: GET|PUT /api/stores/{store}/design/draft, POST design/publish|rollback|preview-token,
// GET design/versions, GET|POST design/media, GET design/media/{id} -> Go /v1/admin/stores/{store}/design/* (integration:read /
// integration:manage). Started only by tests/foundation/browser_store_design_test.go (build tag browser), which owns the isolated PG,
// the real Go API (with a call-recording proxy the BFF probes read), the production admin Next build and the signed mock IdP.
// Written from the contract and the unit brief; locators use the UI unit's data-testid vocabulary and its copy file for the
// strings that differ per locale (en, zh-TW). Storefront rendering of the document is another unit (storefront-shell).
import { expect, test, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { designCopy, fill } from "../../apps/admin/lib/design-copy";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const apiOrigin = required("LC_BROWSER_API_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
type Journey = { id: string; name: string; locale: "en" | "zh-TW"; viewport: "desktop" | "mobile" };
const journeys = JSON.parse(required("LC_BROWSER_DESIGN_STORES")) as Journey[];
const bffStore = required("LC_BROWSER_BFF_STORE");
const readonlyStore = required("LC_BROWSER_READONLY_STORE");
const foreignStores = required("LC_BROWSER_FOREIGN_STORES").split(",");

test.use({ baseURL: origin, trace: "retain-on-failure", screenshot: "only-on-failure" });
// Tests are independent (own stores, or the BFF-probe store with a self-made baseline) so one red test never hides the others.

// a real 1x1 PNG (valid for Go's image.DecodeConfig sniff)
// the base64 is split so the G04/CI secret grep (Meta "EAA…" token shape) never matches PNG bytes
const PNG = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB" + "CAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==", "base64");
const ACCENT = "#ff6600";

async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await page.getByTestId("nav-group-storefront").waitFor({ state: "attached" });
  const menu = page.locator('button[aria-controls="workspace-navigation"]');
  const drawer = await menu.isVisible();
  if (drawer) await menu.click();
  await expect(page.getByTestId("nav-group-storefront")).toBeVisible();
  if (drawer) await page.keyboard.press("Escape");
}

async function fitsWidth(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth), "no horizontal page scroll").toBeLessThanOrEqual(1);
}

const manifestPath = path.join(evidence, "screenshots.json");
async function shot(page: Page, name: string, j: Journey) {
  const file = path.join(evidence, `${name}-${j.locale}-${j.viewport}.png`);
  await page.screenshot({ path: file, fullPage: false, animations: "disabled" });
  await fitsWidth(page);
  let manifest: unknown[] = [];
  try {
    manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  } catch {
    /* first screenshot */
  }
  manifest.push({ File: path.basename(file), Sha256: createHash("sha256").update(await readFile(file)).digest("hex"), Locale: j.locale, Viewport: j.viewport });
  await writeFile(manifestPath, JSON.stringify(manifest, null, 2));
}

type Dialog = { type: string; message: string };
function dialogs(page: Page) {
  const seen: Dialog[] = [];
  let answer: "accept" | "dismiss" = "accept";
  page.on("dialog", (d) => {
    seen.push({ type: d.type(), message: d.message() });
    void (answer === "accept" ? d.accept() : d.dismiss());
  });
  return { seen, answer: (a: "accept" | "dismiss") => { answer = a; } };
}

function watchErrors(page: Page) {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    if (m.type() === "error" && !/Failed to load resource/.test(m.text())) errors.push(`console.error: ${m.text()}`);
  });
  return errors;
}

for (const j of journeys) {
  test(`SDB01 merchant journey ${j.locale} ${j.viewport}: profile, logo, sections, page, save, guard, publish v1, edit, publish v2, rollback to v1`, async ({ page }) => {
    const c = designCopy[j.locale];
    const name = `Gate Shop ${j.locale} ${j.viewport}`;
    const mobile = j.viewport === "mobile";
    await page.setViewportSize(mobile ? { width: 390, height: 844 } : { width: 1586, height: 992 });
    const errors = watchErrors(page);
    const dlg = dialogs(page);
    await signedLogin(page);
    await page.goto(`/${j.locale}/design?store=${j.id}`);
    const status = page.getByTestId("design-status");
    const note = page.getByTestId("design-note");
    const rows = page.getByTestId("design-version-row");

    // ---- empty state: nothing saved, never published, the toolbar fits the viewport ----
    await expect(page.getByTestId("design-page")).toBeVisible();
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(c.title);
    await expect(status).toContainText(c.noDraft);
    await expect(status).toContainText(c.neverPublished);
    for (const id of ["design-save", "design-preview", "design-publish"]) {
      const box = await page.getByTestId(id).boundingBox();
      expect(box, id).not.toBeNull();
      expect(box!.x).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width, `${id} inside the viewport`).toBeLessThanOrEqual((mobile ? 390 : 1586) + 1);
    }
    await expect(page.getByRole("tab")).toHaveCount(5);

    // ---- profile: name, accent colour, logo upload (+ refused uploads), contact ----
    await page.getByTestId("design-name").fill(name);
    await expect(status).toContainText(c.unsaved);
    await page.getByTestId("design-accent").fill(ACCENT);
    await page.getByTestId("design-logo-choose").click();
    // client-side refusals: wrong type, svg, over 2 MB; server-side refusal: image/png label on non-image bytes
    const file = page.getByTestId("design-logo-file");
    await file.setInputFiles({ name: "x.txt", mimeType: "text/plain", buffer: Buffer.from("hello") });
    await expect(note).toHaveText(c.media.badType);
    await file.setInputFiles({ name: "x.svg", mimeType: "image/svg+xml", buffer: Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>') });
    await expect(note).toHaveText(c.media.badType);
    await file.setInputFiles({ name: "big.png", mimeType: "image/png", buffer: Buffer.alloc(2 * 1024 * 1024 + 1, 1) });
    await expect(note).toHaveText(c.media.badSize);
    await file.setInputFiles({ name: "fake.png", mimeType: "image/png", buffer: Buffer.from("<html><script>alert(1)</script></html>") });
    await expect(note).toHaveText(c.media.badType); // Go sniffs the bytes, not the label
    await file.setInputFiles({ name: "logo.png", mimeType: "image/png", buffer: PNG });
    const logo = page.getByTestId("design-logo").locator("img").first();
    await expect(logo).toBeVisible();
    await expect.poll(() => logo.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
    await expect(page.getByTestId("design-logo-clear")).toBeVisible();
    await page.getByLabel(c.profile.email, { exact: true }).fill("shop@example.com");
    await page.getByLabel(c.profile.phone, { exact: true }).fill("+886 2 1234 5678");
    await shot(page, "profile", j);

    // ---- home sections: hero, product grid, text block ----
    await page.getByTestId("design-tab-home").click();
    // contract: a store without a saved draft starts from the derived default (store name + one product_grid); drop it, build our own
    const sections = page.getByTestId("design-section");
    await expect(sections).toHaveCount(1);
    await expect(sections.first().locator("h3")).toHaveText(c.home.types.product_grid);
    await sections.first().getByRole("button", { name: c.nav.remove, exact: true }).click();
    await expect(sections).toHaveCount(0);
    for (const type of ["hero", "product_grid", "rich_text"]) {
      await page.getByTestId("design-section-type").selectOption(type);
      await page.getByTestId("design-section-add").click();
    }
    await expect(sections).toHaveCount(3);
    await expect(sections.nth(0).locator("h3")).toHaveText(c.home.types.hero);
    await expect(sections.nth(1).locator("h3")).toHaveText(c.home.types.product_grid);
    await expect(sections.nth(2).locator("h3")).toHaveText(c.home.types.rich_text);
    await expect(sections.nth(0).locator("img").first()).toBeVisible(); // the hero took the uploaded image
    await sections.nth(0).getByLabel(c.home.heading, { exact: true }).fill("Welcome");
    await sections.nth(1).getByLabel(c.home.heading, { exact: true }).fill("Bestsellers");
    // hostile markdown: refused on the field at once, and the server refuses the save (the field path reaching the UI is SDB05)
    const body = sections.nth(2).locator("textarea");
    await body.fill("<script>alert(1)</script> **bold**");
    await expect(sections.nth(2).getByRole("alert")).toContainText(c.pages.problem);
    await page.getByTestId("design-save").click();
    await expect(note).toHaveAttribute("role", "alert");
    await expect(status).toContainText(c.noDraft); // nothing was stored
    await sections.nth(2).getByLabel(c.home.heading, { exact: true }).fill("Our story");
    await body.fill("Hello **bold** and [a link](https://example.com/story)");
    await expect(sections.nth(2).getByRole("alert")).toHaveCount(0);
    // reorder and back: the order is the document order
    await sections.nth(2).getByRole("button", { name: c.nav.up, exact: true }).click();
    await expect(sections.nth(1).locator("h3")).toHaveText(c.home.types.rich_text);
    await sections.nth(1).getByRole("button", { name: c.nav.down, exact: true }).click();
    await expect(sections.nth(2).locator("h3")).toHaveText(c.home.types.rich_text);
    await shot(page, "home", j);

    // ---- a page: hostile markdown is escaped in the live preview, a bad slug blocks the save locally ----
    await page.getByTestId("design-tab-pages").click();
    await page.getByTestId("design-page-add").click();
    const pageBlock = page.getByTestId("design-pages").getByTestId("design-page-block").first(); // D4 fixed: page blocks are design-page-block
    await pageBlock.getByLabel(c.pages.title).fill("About us");
    await pageBlock.getByLabel(c.pages.slug).fill("Bad Slug");
    await pageBlock.locator("textarea").fill('**Safe** <img src=x onerror="window.__xss=1"> [x](javascript:window.__xss=2)');
    const preview = page.getByTestId("design-page-preview").first();
    await expect(pageBlock.getByRole("alert").first()).toBeVisible();
    await expect(preview.locator("img")).toHaveCount(0);
    await expect(preview.locator("a")).toHaveCount(0);
    expect(await preview.evaluate((el) => el.innerHTML)).not.toContain("<img");
    expect(await page.evaluate(() => (window as unknown as { __xss?: number }).__xss)).toBeUndefined();
    await page.getByTestId("design-save").click();
    await expect(note).toHaveText(c.needsSave); // slug "Bad Slug" is refused before anything is sent
    await expect(status).toContainText(c.noDraft);
    await pageBlock.getByLabel(c.pages.slug).fill("about");
    await pageBlock.locator("textarea").fill("About **us**\n\n- one\n- two\n\n[our site](https://example.com)");
    await expect(preview.locator("strong")).toHaveText("us");
    await expect(preview.locator("li")).toHaveCount(2);
    await expect(preview.locator("a")).toHaveAttribute("href", "https://example.com");
    await expect(preview.locator("a")).toHaveAttribute("rel", /noopener/);
    await shot(page, "pages", j);

    // ---- save draft: one PUT, only {expected_version, document}, keyed, CSRF'd, no HTML in the document ----
    const [put] = await Promise.all([
      page.waitForRequest((r) => r.method() === "PUT" && /\/design\/draft$/.test(r.url())),
      page.getByTestId("design-save").click(),
    ]);
    expect(Object.keys(put.postDataJSON()).sort()).toEqual(["document", "expected_version"]);
    expect(put.postDataJSON().expected_version).toBe(0);
    expect(put.postDataJSON().document.profile.name).toBe(name);
    expect(put.headers()["idempotency-key"]).toMatch(/^[A-Za-z0-9_.:-]{8,128}$/);
    expect(put.headers()["x-csrf-token"]).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(JSON.stringify(put.postDataJSON().document)).not.toContain("<");
    await expect(note).toHaveText(c.okSaved);
    await expect(status).toContainText(c.saved);
    await expect(status).toContainText(fill(c.draftVersion, { n: 1 }));
    await expect(status).toContainText(c.neverPublished);

    // ---- unsaved-changes guard (beforeunload hook, in-app navigation confirm: stay and leave) ----
    // at 390 px the rail is an off-canvas drawer behind the menu button; at desktop width it is always visible
    // This fixture already has integration:read; navigation must exercise the dirty
    // guard without granting a different domain solely to reach a destination.
    const goSettings = async () => {
      if (mobile && (await page.locator('button[aria-controls="workspace-navigation"]').getAttribute("aria-expanded")) !== "true") await page.locator('button[aria-controls="workspace-navigation"]').click();
      await page.getByTestId("nav-group-settings").click();
    };
    const beforeUnloadPrevented = () => page.evaluate(() => { const e = new Event("beforeunload", { cancelable: true }); window.dispatchEvent(e); return e.defaultPrevented; });
    expect(await beforeUnloadPrevented(), "clean draft must not warn").toBe(false);
    await page.getByTestId("design-tab-profile").click();
    await page.getByLabel(c.profile.tagline, { exact: true }).fill("a tagline nobody saved");
    await expect(status).toContainText(c.unsaved);
    expect(await beforeUnloadPrevented(), "dirty draft must warn on unload").toBe(true);
    dlg.answer("dismiss");
    await goSettings();
    await expect.poll(() => dlg.seen.filter((d) => d.message === c.guard).length).toBe(1);
    await expect(page).toHaveURL(new RegExp(`/${j.locale}/design`));
    await expect(page.getByLabel(c.profile.tagline, { exact: true })).toHaveValue("a tagline nobody saved");
    // Canceling navigation leaves the modal drawer open. Return focus to the
    // editor before editing; its inert content must not receive input behind it.
    if (mobile) {
      await page.keyboard.press("Escape");
      await expect(page.locator('button[aria-controls="workspace-navigation"]')).toHaveAttribute("aria-expanded", "false");
    }
    // undoing the edit makes the page clean again: no warning, navigation goes through without a dialog
    await page.getByLabel(c.profile.tagline, { exact: true }).fill("");
    await expect(status).toContainText(c.saved);
    expect(await beforeUnloadPrevented()).toBe(false);
    const before = dlg.seen.length;
    await goSettings();
    await expect(page).toHaveURL(new RegExp(`/${j.locale}/settings`));
    expect(dlg.seen.length).toBe(before);
    // dirty again and leave on purpose: the edit is gone and the saved draft is what comes back
    await page.goto(`/${j.locale}/design?store=${j.id}`);
    await page.getByLabel(c.profile.tagline, { exact: true }).fill("discard me");
    dlg.answer("accept");
    await goSettings();
    await expect(page).toHaveURL(new RegExp(`/${j.locale}/settings`));
    expect(dlg.seen.filter((d) => d.message === c.guard).length).toBe(2);
    await page.goto(`/${j.locale}/design?store=${j.id}`);
    await expect(status).toContainText(fill(c.draftVersion, { n: 1 }));
    await expect(page.getByLabel(c.profile.tagline, { exact: true })).toHaveValue("");
    await expect(page.getByTestId("design-name")).toHaveValue(name);

    // ---- publish v1 ----
    const publish = page.getByTestId("design-publish");
    await expect(publish).toBeEnabled();
    const [pub1] = await Promise.all([
      page.waitForRequest((r) => r.method() === "POST" && /\/design\/publish$/.test(r.url())),
      publish.click(),
    ]);
    expect(pub1.postDataJSON()).toEqual({ expected_draft_version: 1 });
    await expect(note).toHaveText(fill(c.okPublished, { n: 1 }));
    expect(dlg.seen.at(-1)?.message).toBe(fill(c.publishConfirm, { n: 1 }));
    await expect(status).toContainText(fill(c.liveVersion, { n: 1 }));
    await expect(publish).toBeDisabled(); // nothing new to publish
    await page.getByTestId("design-tab-versions").click();
    await expect(rows).toHaveCount(1);
    await expect(rows.first()).toContainText("v1");
    await expect(rows.first()).toContainText(c.versions.live);
    await expect(rows.first().getByTestId("design-rollback")).toHaveCount(0);
    await shot(page, "versions-v1", j);

    // ---- edit again and publish v2 (a dirty page is saved first, as draft v2, then published) ----
    await page.getByTestId("design-tab-profile").click();
    await page.getByTestId("design-name").fill(`${name} two`);
    await expect(publish).toBeEnabled();
    await publish.click();
    await expect(note).toHaveText(fill(c.okPublished, { n: 2 }));
    expect(dlg.seen.at(-1)?.message).toBe(fill(c.publishConfirm, { n: 2 }));
    await expect(status).toContainText(fill(c.draftVersion, { n: 2 }));
    await expect(status).toContainText(fill(c.liveVersion, { n: 2 }));
    await page.getByTestId("design-tab-versions").click();
    await expect(rows).toHaveCount(2);
    await expect(rows.nth(0)).toContainText("v2");
    await expect(rows.nth(0)).toContainText(c.versions.live);
    await expect(rows.nth(0).getByTestId("design-rollback")).toHaveCount(0);
    await expect(rows.nth(1)).toContainText("v1");
    await expect(rows.nth(1).getByTestId("design-rollback")).toBeVisible();

    // ---- rollback to v1: a NEW version v3 (kind rollback, from v1); history untouched; the draft is not touched ----
    const [rb] = await Promise.all([
      page.waitForRequest((r) => r.method() === "POST" && /\/design\/rollback$/.test(r.url())),
      rows.nth(1).getByTestId("design-rollback").click(),
    ]);
    expect(rb.postDataJSON()).toEqual({ version: 1 });
    expect(dlg.seen.at(-1)?.message).toBe(fill(c.versions.rollbackConfirm, { n: 1 }));
    await expect(note).toHaveText(fill(c.okRolledBack, { n: 3 }));
    await expect(rows).toHaveCount(3);
    await expect(rows.nth(0)).toContainText("v3");
    await expect(rows.nth(0)).toContainText(c.versions.kind.rollback);
    await expect(rows.nth(0)).toContainText(fill(c.versions.source, { n: 1 }));
    await expect(rows.nth(0)).toContainText(c.versions.live);
    await expect(rows.nth(0).getByTestId("design-rollback")).toHaveCount(0);
    await expect(rows.nth(1).getByTestId("design-rollback")).toBeVisible();
    await expect(rows.nth(2).getByTestId("design-rollback")).toBeVisible();
    await expect(status).toContainText(fill(c.liveVersion, { n: 3 }));
    await expect(status).toContainText(fill(c.draftVersion, { n: 2 }));
    await shot(page, "versions-v3", j);

    // ---- everything survives a reload; the draft still has the v2 edit ----
    await page.reload();
    await expect(status).toContainText(fill(c.draftVersion, { n: 2 }));
    await expect(status).toContainText(fill(c.liveVersion, { n: 3 }));
    await expect(page.getByTestId("design-name")).toHaveValue(`${name} two`);
    await page.getByTestId("design-tab-versions").click();
    await expect(rows).toHaveCount(3);
    expect(dlg.seen.filter((d) => d.type === "beforeunload").length).toBe(0);
    expect(errors).toEqual([]);
  });
}

// ---- BFF probes: what may reach Go, and what must stop at the BFF (the Go-side proxy counts what arrived) ----
type Call = { method: string; path: string; length: number; leaked: boolean };
const goCalls = async (): Promise<Call[]> => (await fetch(`${apiOrigin}/__test/design-calls`)).json() as Promise<Call[]>;
type Probe = { method: string; path: string; body?: string; headers?: Record<string, string>; multipart?: { size: number; bytes?: number[]; type: string }; csrf?: boolean | string; key?: boolean; creds?: RequestCredentials };
async function probe(page: Page, p: Probe) {
  return page.evaluate(async (q) => {
    const csrf = document.cookie.split(";").map((s) => s.trim()).find((s) => s.startsWith("__Host-commerce_csrf="))?.split("=")[1] ?? "";
    const headers: Record<string, string> = { ...(q.headers ?? {}) };
    if (q.csrf !== false) headers["X-CSRF-Token"] = typeof q.csrf === "string" ? q.csrf : csrf;
    if (q.key !== false && q.method !== "GET") headers["Idempotency-Key"] = crypto.randomUUID();
    let body: BodyInit | undefined = q.body;
    if (q.multipart) {
      const data = q.multipart.bytes ? new Uint8Array(q.multipart.bytes) : new Uint8Array(q.multipart.size);
      const form = new FormData();
      form.append("file", new Blob([data], { type: q.multipart.type }), "f.bin");
      body = form;
    } else if (q.body !== undefined) headers["Content-Type"] = "application/json";
    const r = await fetch(q.path, { method: q.method, credentials: q.creds ?? "same-origin", cache: "no-store", headers, body });
    const bytes = new Uint8Array(await r.arrayBuffer());
    return { status: r.status, type: r.headers.get("content-type"), cache: r.headers.get("cache-control"), nosniff: r.headers.get("x-content-type-options"), text: new TextDecoder().decode(bytes.slice(0, 400000)), length: bytes.length, b64: bytes.length < 5000 ? btoa(String.fromCharCode(...bytes)) : "" };
  }, p);
}

test("SDB02 BFF: allowlist, no query, 64 KiB generic cap vs 260 KiB design cap, CSRF/key/origin, identity headers, media bytes", async ({ page }) => {
  await signedLogin(page);
  const base = `/api/stores/${bffStore}/design`;
  const unchanged = async (label: string, run: () => Promise<{ status: number }>, ok: (s: number) => boolean) => {
    const n = (await goCalls()).length;
    const r = await run();
    expect(ok(r.status), `${label}: status ${r.status}`).toBe(true);
    expect((await goCalls()).length, `${label}: must not reach Go`).toBe(n);
  };
  const refused = (s: number) => s >= 400 && s < 500;

  // reads that work
  const draft = await probe(page, { method: "GET", path: `${base}/draft` });
  expect(draft.status).toBe(200);
  expect(JSON.parse(draft.text).version).toBe(0);
  expect((await probe(page, { method: "GET", path: `${base}/versions` })).status).toBe(200);
  expect((await probe(page, { method: "GET", path: `${base}/media` })).status).toBe(200);

  // allowlist: every method/path the contract does not define stops at the BFF
  const uuid = "0f8fad5b-d9cb-469f-a165-70867728950e";
  for (const [method, p] of [
    ["DELETE", "draft"], ["PATCH", "draft"], ["POST", "draft"], ["PUT", "versions"], ["GET", "publish"], ["GET", "rollback"], ["GET", "preview-token"],
    ["POST", "versions"], ["GET", "nope"], ["GET", "media/..%2fdraft"], ["GET", "media/%2e%2e%2fdraft"], ["POST", `media/${uuid}`], ["GET", `media/${uuid}/delete`],
    ["DELETE", `media/${uuid}`], ["GET", "DRAFT"], ["PUT", "draft/x"],
  ] as const)
    await unchanged(`${method} design/${p}`, () => probe(page, { method, path: `${base}/${p}`, body: method === "GET" ? undefined : "{}" }), refused);
  // no query string on any design route; a read carries no key
  await unchanged("GET draft?x=1", () => probe(page, { method: "GET", path: `${base}/draft?x=1` }), (s) => s === 422);
  await unchanged("PUT draft?x=1", () => probe(page, { method: "PUT", path: `${base}/draft?x=1`, body: "{}" }), (s) => s === 422);
  await unchanged("POST publish?x=1", () => probe(page, { method: "POST", path: `${base}/publish?x=1`, body: "{}" }), (s) => s === 422);
  await unchanged("GET draft with Idempotency-Key", () => probe(page, { method: "GET", path: `${base}/draft`, headers: { "Idempotency-Key": "abcdefgh12345678" } }), (s) => s === 422);
  // session boundary: no cookie, no/other CSRF, no key, foreign Origin
  await unchanged("no cookie", () => probe(page, { method: "GET", path: `${base}/draft`, creds: "omit" }), (s) => s === 401);
  await unchanged("write without CSRF", () => probe(page, { method: "PUT", path: `${base}/draft`, body: "{}", csrf: false }), refused);
  await unchanged("write with another CSRF", () => probe(page, { method: "PUT", path: `${base}/draft`, body: "{}", csrf: "A".repeat(43) }), refused);
  await unchanged("write without Idempotency-Key", () => probe(page, { method: "POST", path: `${base}/publish`, body: '{"expected_draft_version":1}', key: false }), refused);
  // a foreign Origin on an otherwise valid write (Node fetch: a browser fetch cannot set Origin; same cookies + CSRF token as the page)
  const nEvil = (await goCalls()).length;
  const jar = await page.context().cookies();
  const csrfValue = jar.find((k) => k.name === "__Host-commerce_csrf")?.value ?? "";
  const cookieHeader = jar.map((k) => `${k.name}=${k.value}`).join("; ");
  const valid = JSON.stringify({ expected_version: 0, document: { profile: { name: "x", accent_color: "#000000" } } });
  const send = (originHeader: string | null) => fetch(`${origin}${base}/draft`, { method: "PUT", body: valid, headers: { ...(originHeader ? { Origin: originHeader } : {}), Cookie: cookieHeader, "X-CSRF-Token": csrfValue, "Idempotency-Key": "abcdefgh12345678", "Content-Type": "application/json" } });
  for (const o of ["https://evil.example", null]) {
    const r = await send(o);
    expect(r.status, `write with Origin ${o}`).toBe(403);
  }
  expect((await goCalls()).length, "cross-origin write must not reach Go").toBe(nEvil);

  // caps: the generic JSON cap is 64 KiB; the design PUT may carry a whole document (256 KiB + envelope = 260 KiB)
  await unchanged("POST 70 KiB", () => probe(page, { method: "POST", path: `${base}/publish`, body: JSON.stringify({ expected_draft_version: 1, pad: "x".repeat(70 * 1024) }) }), (s) => s === 400);
  const big = (bytes: number) => JSON.stringify({ expected_version: 0, document: { profile: { name: "big", accent_color: "#000000" }, pad: "x".repeat(bytes) } });
  await unchanged("PUT 260 KiB + 1", () => probe(page, { method: "PUT", path: `${base}/draft`, body: big(260 * 1024) }), (s) => s === 400);
  const n0 = (await goCalls()).length;
  const over = await probe(page, { method: "PUT", path: `${base}/draft`, body: big(258 * 1024) }); // under the BFF cap, over Go's document cap
  expect(over.status === 413 || over.status === 422 || over.status === 400, `258 KiB document: ${over.status}`).toBe(true);
  expect((await goCalls()).length, "a body under the BFF cap reaches Go").toBe(n0 + 1);
  const bigDoc = {
    profile: { name: "A large but valid document", accent_color: "#247965" },
    pages: Array.from({ length: 12 }, (_, i) => ({ slug: `p${i}`, title: `Page ${i}`, body: "a".repeat(19000) })),
  };
  const large = await probe(page, { method: "PUT", path: `${base}/draft`, body: JSON.stringify({ expected_version: 0, document: bigDoc }) });
  expect(large.status, `a valid ~228 KiB document must pass the BFF (generic cap is 64 KiB): ${large.text.slice(0, 200)}`).toBe(200);

  // identity headers sent by the page are never forwarded (the proxy flags Cookie, X-Tenant-ID, X-Store-ID, X-Forwarded-Host, Forwarded)
  const spoof = await probe(page, { method: "GET", path: `${base}/draft`, headers: { "X-Tenant-ID": "00000000-0000-4000-8000-000000000000", "X-Store-ID": foreignStores[0], "X-Forwarded-Host": "evil.example" } });
  expect(spoof.status).toBe(200);
  expect(JSON.parse(spoof.text).version).toBe(1);

  // another store of the session / a foreign store: refused, never served
  for (const store of [...foreignStores, "not-a-uuid"]) {
    const r = await probe(page, { method: "GET", path: `/api/stores/${store}/design/draft` });
    expect([401, 403, 404, 422], `store ${store}: ${r.status}`).toContain(r.status);
    expect(r.text).not.toContain('"document"');
    const w = await probe(page, { method: "PUT", path: `/api/stores/${store}/design/draft`, body: JSON.stringify({ expected_version: 0, document: { profile: { name: "x", accent_color: "#000000" } } }) });
    expect([401, 403, 404, 422], `write to store ${store}: ${w.status}`).toContain(w.status);
  }

  // media through the BFF: multipart in, bytes out only as a validated raster type, hardened headers; 2 MiB cap before Go
  const up = await probe(page, { method: "POST", path: `${base}/media`, multipart: { size: 0, bytes: [...PNG], type: "image/png" } });
  expect(up.status, up.text).toBe(200);
  const id = JSON.parse(up.text).id as string;
  const raw = await probe(page, { method: "GET", path: `${base}/media/${id}` });
  expect(raw.status).toBe(200);
  expect(raw.type).toBe("image/png");
  expect(raw.nosniff).toBe("nosniff");
  expect(raw.b64).toBe(PNG.toString("base64"));
  { // the BFF does not sniff: Go (catalog.SniffImage) is the authority and answers 4xx for an SVG
    const svg = await probe(page, { method: "POST", path: `${base}/media`, multipart: { size: 0, bytes: [...Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>')], type: "image/svg+xml" } });
    expect([400, 422], `svg upload: ${svg.status}`).toContain(svg.status);
  }
  await unchanged("3 MiB upload", () => probe(page, { method: "POST", path: `${base}/media`, multipart: { size: 3 * 1024 * 1024, type: "image/png" } }), (s) => s === 413);
  await unchanged("upload with a query", () => probe(page, { method: "POST", path: `${base}/media?x=1`, multipart: { size: 0, bytes: [...PNG], type: "image/png" } }), (s) => s === 422);

  // preview token: issued through the BFF, short-lived, never cacheable
  const tok = await probe(page, { method: "POST", path: `${base}/preview-token`, body: "{}" });
  expect(tok.status, tok.text).toBe(200);
  expect(JSON.parse(tok.text).token).toMatch(/^[A-Za-z0-9_-]{40,}$/);
  expect(tok.cache ?? "").toContain("no-store");

  // every call that did reach Go carried no browser identity
  expect((await goCalls()).filter((c) => c.leaked)).toEqual([]);
});

test("SDB03 read-only member: the page opens, every write is refused by the server and the UI says so; nothing is stored", async ({ page }) => {
  const c = designCopy.en;
  page.on("dialog", (d) => void d.accept());
  await signedLogin(page);
  await page.goto(`/en/design?store=${readonlyStore}`);
  await expect(page.getByTestId("design-page")).toBeVisible();
  await expect(page.getByTestId("design-status")).toContainText(c.noDraft);
  await page.getByTestId("design-name").fill("Not allowed");
  await page.getByTestId("design-save").click();
  await expect(page.getByTestId("design-note")).toHaveText(c.errors.forbidden);
  await expect(page.getByTestId("design-status")).toContainText(c.unsaved);
  const base = `/api/stores/${readonlyStore}/design`;
  expect((await probe(page, { method: "GET", path: `${base}/draft` })).status).toBe(200);
  for (const [method, p, body] of [["PUT", "draft", JSON.stringify({ expected_version: 0, document: { profile: { name: "x", accent_color: "#000000" } } })], ["POST", "publish", '{"expected_draft_version":1}'], ["POST", "rollback", '{"version":1}'], ["POST", "preview-token", "{}"]] as const) {
    const r = await probe(page, { method, path: `${base}/${p}`, body });
    expect(r.status, `${method} ${p}`).toBe(403);
  }
  const up = await probe(page, { method: "POST", path: `${base}/media`, multipart: { size: 0, bytes: [...PNG], type: "image/png" } });
  expect(up.status).toBe(403);
  await page.reload();
  await expect(page.getByTestId("design-status")).toContainText(c.noDraft);
});

test("SDB04 stale write: a second tab that saved first makes this tab's save a conflict with a reload path", async ({ browser }) => {
  const c = designCopy.en;
  const ctx = await browser.newContext({ viewport: { width: 1586, height: 992 } });
  const a = await ctx.newPage();
  a.on("dialog", (d) => void d.accept());
  await signedLogin(a);
  const draftNow = JSON.parse((await probe(a, { method: "GET", path: `/api/stores/${bffStore}/design/draft` })).text).version as number;
  const baseline = await probe(a, { method: "PUT", path: `/api/stores/${bffStore}/design/draft`, body: JSON.stringify({ expected_version: draftNow, document: { profile: { name: "Baseline name", accent_color: "#247965" } } }) });
  expect(baseline.status, baseline.text.slice(0, 200)).toBe(200);
  await a.goto(`/en/design?store=${bffStore}`);
  await expect(a.getByTestId("design-name")).toHaveValue("Baseline name");
  const b = await ctx.newPage();
  b.on("dialog", (d) => void d.accept());
  await b.goto(`/en/design?store=${bffStore}`);
  await b.getByTestId("design-name").fill("Saved first in tab B");
  await b.getByTestId("design-save").click();
  await expect(b.getByTestId("design-note")).toHaveText(c.okSaved);
  await a.getByTestId("design-name").fill("Saved second in tab A");
  await a.getByTestId("design-save").click();
  await expect(a.getByTestId("design-note")).toHaveText(c.errors.conflict);
  await expect(a.getByTestId("design-reload")).toBeVisible();
  await expect(a.getByTestId("design-status")).toContainText(c.unsaved); // the local edit is kept, not silently overwritten
  await a.getByTestId("design-reload").click();
  await expect(a.getByTestId("design-name")).toHaveValue("Saved first in tab B");
  await ctx.close();
});

test("SDB05 a 422 reaches the editor with the offending field path (contract B-acceptance: details {path, reason}), through the BFF and on the field", async ({ page }) => {
  const c = designCopy.en;
  page.on("dialog", (d) => void d.accept());
  await signedLogin(page);
  const base = `/api/stores/${bffStore}/design`;
  const cur = JSON.parse((await probe(page, { method: "GET", path: `${base}/draft` })).text).version as number;
  const bad = await probe(page, { method: "PUT", path: `${base}/draft`, body: JSON.stringify({ expected_version: cur, document: { profile: { name: "x", accent_color: "#000000", css: "body{}" } } }) });
  expect(bad.status).toBe(422);
  const envelope = JSON.parse(bad.text) as { code: string; details?: { path?: string; reason?: string } };
  expect(envelope.code).toBe("invalid_request");
  expect(envelope.details?.path, `the BFF must pass details.path through: ${bad.text}`).toBe("profile.css");
  // in the editor: the save of a document the server refuses names the field
  await page.goto(`/en/design?store=${bffStore}`);
  await page.getByTestId("design-tab-home").click();
  await page.getByTestId("design-section-type").selectOption("rich_text");
  await page.getByTestId("design-section-add").click();
  const section = page.getByTestId("design-section").last();
  await section.locator("textarea").fill("<script>alert(1)</script>");
  await page.getByTestId("design-save").click();
  await expect(page.getByTestId("design-note")).toContainText(/home\.sections\[\d+\]\.body/);
});
