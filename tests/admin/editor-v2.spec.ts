// Purpose: Accept the owner-signed simplified store editor with real clicks in three locales and two layouts.
// Depends on: @playwright/test; existing design-copy/model; store-design signed MOCK login and BFF-store fixture.
// Used by: playwright.config.ts store-design suite, tests/foundation/browser_store_design_test.go, --browser-design CI.
// Invariants: server-shaped, no-store MOCK replies only; no DOM/storage writes, API-driven user actions, force clicks or live facts.
import { expect, test, type Page, type Locator } from "@playwright/test";
import { designCopy } from "../../apps/admin/lib/design-copy";
import type { DesignDocument, Draft, VersionInfo } from "../../apps/admin/lib/design-model";

type Locale = "zh-TW" | "zh-CN" | "en";
const locales: Locale[] = ["zh-TW", "zh-CN", "en"];
const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN;
const store = process.env.LC_BROWSER_BFF_STORE;
if (!origin || !store) throw new Error("store-design harness origin and BFF store are required");
const address = "https://12345678.example.test";
const customAddress = "https://shop.example.test";
const ui = {
  "zh-TW": { title: "編輯網店", pages: "資訊頁", home: "首頁區塊", nav: "導覽選單", profile: "店鋪資料", domains: "網店網址", intro: "圖片與介紹", contact: "聯絡資料", social: "社群連結", details: "查看內容", scope: "發布店鋪資料、導覽、首頁及全部資訊頁", unchanged: "不更改價格、庫存與網址", empty: "頁面內容留白", pending: "發布結果待確認", query: "查詢發布狀態", failed: "這次未能發布" },
  "zh-CN": { title: "编辑网店", pages: "信息页", home: "首页区块", nav: "导航菜单", profile: "店铺资料", domains: "网店网址", intro: "图片和介绍", contact: "联系资料", social: "社交链接", details: "查看内容", scope: "发布店铺资料、导航、首页和全部信息页", unchanged: "不修改价格、库存和网址", empty: "页面内容留空", pending: "发布结果待确认", query: "查询发布状态", failed: "本次发布失败" },
  en: { title: "Edit store", pages: "Pages", home: "Home sections", nav: "Menus", profile: "Store details", domains: "Domains", intro: "Logo & intro", contact: "Contact", social: "Social", details: "View details", scope: "Publishes store details, menus, home and every page", unchanged: "Prices, stock and domains stay unchanged", empty: "page content is empty", pending: "Publication awaiting confirmation", query: "Check publication status", failed: "Publication did not complete" },
} satisfies Record<Locale, Record<string, string>>;

type ClickRow = { page: string; control: string; action: string; expected: string; actual: string };
type Measurement = { screen: string; blocks: number; characters: number };
const ledger: ClickRow[] = [];
test.use({ baseURL: origin, timezoneId: "America/Los_Angeles" });
test.beforeEach(() => { ledger.length = 0; });
test.afterEach(async ({ page }, info) => {
  await info.attach("click-ledger", { body: JSON.stringify(ledger, null, 2), contentType: "application/json" });
  if (!page.isClosed()) await info.attach("current-screen", { body: await page.screenshot({ fullPage: true }), contentType: "image/png" });
});

// Records passed assertions only: an action is never called successful before its visible result is asserted.
async function click(page: Page, target: Locator, control: string, expected: string, verify: () => Promise<void>) {
  await target.click();
  await verify();
  ledger.push({ page: new URL(page.url()).pathname, control, action: "click", expected, actual: "observed" });
}
async function fill(page: Page, target: Locator, value: string, control: string) {
  await target.fill(value);
  await expect(target).toHaveValue(value);
  ledger.push({ page: new URL(page.url()).pathname, control, action: "fill", expected: value, actual: value });
}

function documentFixture(): DesignDocument {
  return {
    profile: { name: "Demo", accent_color: "#247965", tagline: null, logo_image_id: null, favicon_image_id: null, announcement: null,
      contact: { email: null, phone: null, address: null, line_url: null, facebook_url: null, instagram_url: null } },
    nav: { header: [{ label: "Home", kind: "home", target: null }, { label: "Products", kind: "all_products", target: null }],
      footer: [{ label: "Shipping", kind: "page", target: "shipping" }] },
    home: { sections: [{ type: "rich_text", heading: "Welcome", body: "Hello." },
      { type: "product_grid", heading: "Products", sort: "newest", limit: 8 }, { type: "rich_text", heading: "Text", body: "Story." }] },
    pages: [{ title: "Shipping", slug: "shipping", body: "See checkout." }, { title: "Returns", slug: "returns", body: "" }],
  };
}

async function open(page: Page, locale: Locale, mobile: boolean) {
  await page.setViewportSize(mobile ? { width: 390, height: 844 } : { width: 1586, height: 992 });
  const state = {
    // Deliberately different from Figma's 12/8, 13/9: no screen may hard-code the example versions.
    draft: { version: 42, published_version: 27, updated_at: "2026-10-28T12:30:00Z", document: documentFixture() } as Draft,
    versions: [{ version: 27, source_version: 41, kind: "publish", published_at: "2026-10-27T12:30:00Z", published_by: "fixture" }] as VersionInfo[],
    outcome: "success" as "success" | "forbidden" | "uncertain",
    attempts: [] as { key: string | undefined; body: string | null }[],
    operations: [] as string[],
    saved: [] as DesignDocument[],
    receipts: new Map<string, VersionInfo>(),
  };
  // Fixture setup only: match the BFF's no-store read contract (customers-client.readJSON).
  // All product writes originate from clicks; keep the production response checks intact.
  const readHeaders = { "Cache-Control": "no-store" };
  await page.route(`**/api/stores/${store}/design/**`, async (route) => {
    const request = route.request();
    const resource = new URL(request.url()).pathname.split("/design/")[1];
    const method = request.method();
    if (method === "GET") {
      const values: Record<string, unknown> = { draft: state.draft, versions: { live_version: state.draft.published_version, items: state.versions }, media: { items: [] } };
      if (!(resource in values)) throw new Error(`unexpected fixture read ${resource}`);
      await route.fulfill({ headers: readHeaders, json: values[resource] });
    } else if (method === "PUT" && resource === "draft") {
      state.operations.push("save");
      const body = request.postDataJSON();
      expect(body.expected_version, "CAS uses the current server draft").toBe(state.draft.version);
      state.draft = { ...state.draft, version: state.draft.version + 1, document: body.document };
      state.saved.push(structuredClone(state.draft.document));
      await route.fulfill({ json: state.draft });
    } else if (method === "POST" && resource === "preview-token") {
      state.operations.push("preview");
      await route.fulfill({ json: { token: "p".repeat(43), draft_version: state.draft.version, expires_at: "2099-01-01T00:15:00Z" } });
    } else if (method === "POST" && resource === "publish") {
      state.operations.push("publish");
      const key = request.headers()["idempotency-key"];
      state.attempts.push({ key, body: request.postData() });
      if (state.outcome === "uncertain") return route.abort("connectionreset");
      if (state.outcome === "forbidden") return route.fulfill({ status: 403, json: { code: "forbidden" } });
      expect(request.postDataJSON()).toEqual({ expected_draft_version: state.draft.version });
      expect(key).toBeTruthy();
      let receipt = state.receipts.get(key!);
      if (!receipt) {
        receipt = { version: (state.draft.published_version ?? 0) + 1, source_version: state.draft.version, kind: "publish", published_at: "2026-10-28T12:30:00Z", published_by: "fixture" };
        state.receipts.set(key!, receipt);
        state.versions.unshift(receipt);
        state.draft.published_version = receipt.version;
      }
      await route.fulfill({ json: receipt });
    } else throw new Error(`unexpected fixture write ${method} ${resource}`);
  });
  // Storefront reads use orders-client.read, whose BFF envelope requires the exact private cache policy.
  const storefrontReadHeaders = { "Cache-Control": "private, no-store" };
  await page.route(`**/api/stores/${store}/storefront`, (route) => route.fulfill({ headers: storefrontReadHeaders, json: { published: true, version: 5, domains: [{ origin: address, valid_until: "2099-01-01T00:00:00Z", serving: true }] } }));
  await page.route(`**/api/stores/${store}/storefront/domains`, (route) => route.fulfill({ headers: storefrontReadHeaders, json: { domains: [
    { origin: address, kind: "platform", state: "ACTIVE", version: 1, token: null, verify_deadline: null, serving: true },
    { origin: customAddress, kind: "custom", state: "OWNERSHIP_PENDING", version: 1, token: null, verify_deadline: null, serving: false },
  ] } }));
  // A disposable buyer-origin fixture accepts only the existing preview navigation. No storefront rendering claim.
  await page.context().route(`${address}/**`, (route) => route.fulfill({ contentType: "text/html", body: "<!doctype html><title>Preview fixture</title><p>Preview</p>" }));
  await page.goto("/en/");
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await page.getByTestId("nav-group-storefront").waitFor({ state: "attached" });
  await page.goto(`/${locale}/design?store=${store}`);
  await expect(page.getByTestId("design-page"), "existing route loads before signed-design assertions").toBeVisible();
  await expect(page.getByTestId("design-status"), "existing API snapshot loaded, not a missing component/import failure").toContainText("42");
  return state;
}

async function hub(page: Page, locale: Locale) {
  await expect(page.getByTestId("design-hub"), "signed four-task hub replaces the expanded five-tab form").toBeVisible();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(ui[locale].title);
}
async function task(page: Page, locale: Locale, name: "home" | "profile" | "pages" | "nav") {
  await click(page, page.getByTestId(`design-task-${name}`), ui[locale][name], "selected editor is visible", async () => {
    await expect(page.getByTestId(`design-${name}`)).toBeVisible();
    await expect(page.getByTestId("design-hub")).toBeHidden();
  });
}
async function back(page: Page, locale: Locale) {
  await click(page, page.getByTestId("design-back"), "Back", "task hub", () => hub(page, locale));
}
async function save(page: Page, state: Awaited<ReturnType<typeof open>>, locale: Locale) {
  const version = state.draft.version;
  await click(page, page.getByTestId("design-save"), "Save draft", "new server version", async () => {
    await expect(page.getByTestId("design-status")).toContainText(String(version + 1));
    await expect(page.getByTestId("design-note")).toContainText(designCopy[locale].okSaved);
  });
}
async function review(page: Page, locale: Locale) {
  await click(page, page.getByTestId("design-publish"), "Publish", "whole-document review", async () => {
    await expect(page.getByTestId("design-publish-review")).toBeVisible();
    await expect(page.getByText(ui[locale].scope, { exact: true })).toBeVisible();
    await expect(page.getByText(ui[locale].unchanged, { exact: true })).toBeVisible();
    await expect(page.getByTestId("design-publish-review")).toContainText(address);
  });
}

async function measure(page: Page, screen: string): Promise<Measurement> {
  // READ/MEASURE only: include all rendered text even below the fold; exclude hidden/collapsed content and option lists.
  const result = await page.getByTestId("design-screen").evaluate((root) => {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    const values: string[] = [];
    while (walker.nextNode()) {
      const node = walker.currentNode;
      const parent = node.parentElement;
      if (!parent || parent.closest("option,script,style,[hidden]")) continue;
      const range = document.createRange();
      range.selectNodeContents(node);
      const text = node.textContent?.trim();
      if (text && parent.checkVisibility({ checkOpacity: true, checkVisibilityCSS: true }) && range.getClientRects().length) values.push(text);
    }
    return { blocks: values.length, characters: values.join("").replace(/\s/g, "").length };
  });
  expect(result.blocks, `${screen}: no screen exceeds 20 visible text blocks`).toBeLessThanOrEqual(20);
  // READ/MEASURE only, no DOM changes or event dispatch.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth), `${screen}: no horizontal overflow`).toBeLessThanOrEqual(1);
  return { screen, ...result };
}

for (const locale of locales) for (const mobile of [false, true]) {
  test(`EDITOR-V2 main ${locale} ${mobile ? "mobile" : "desktop"}: four tasks, current versions, domain status`, async ({ page }) => {
    await open(page, locale, mobile);
    await hub(page, locale);
    for (const name of ["pages", "home", "nav", "profile"] as const) {
      await expect(page.getByTestId(`design-task-${name}`)).toHaveText(ui[locale][name]);
    }
    await expect(page.getByTestId("design-status")).toContainText("27");
    await expect(page.getByRole("tab")).toHaveCount(0);
    const density = await measure(page, "hub");
    if (mobile) {
      expect(density.blocks).toBeLessThanOrEqual(12);
      expect(density.characters).toBeLessThanOrEqual(100);
    }
    await click(page, page.getByTestId("design-domains-open"), ui[locale].domains, "existing system and custom domain status", async () => {
      await expect(page.getByTestId("design-domains")).toContainText(address);
      await expect(page.getByTestId("design-domains")).toContainText(customAddress);
      await expect(page.getByTestId("design-domain-platform")).toHaveAttribute("data-state", "ACTIVE");
      await expect(page.getByTestId("design-domain-custom")).toHaveAttribute("data-state", "OWNERSHIP_PENDING");
      await expect(page.getByTestId("design-domain-settings")).toHaveAttribute("href", new RegExp(`/${locale}/settings\\?store=${store}`));
    });
    await back(page, locale);
    await task(page, locale, "profile");
    await expect(page.getByTestId("design-name")).toHaveValue("Demo");
    await expect(page.getByTestId("design-accent")).toHaveValue("#247965");
    await expect(page.getByLabel(designCopy[locale].profile.tagline, { exact: true })).toBeHidden();
    await back(page, locale);
  });
}

for (const mobile of [false, true]) {
  test(`EDITOR-V2 ${mobile ? "mobile" : "desktop"}: edit every group, reorder, save, refresh, cancel`, async ({ page }) => {
    const locale = "en";
    const c = designCopy.en;
    const state = await open(page, locale, mobile);
    await hub(page, locale);
    const densities: Measurement[] = [await measure(page, "hub")];
    await task(page, locale, "profile");
    densities.push(await measure(page, "profile"));
    await fill(page, page.getByTestId("design-name"), "Demo edited", "Store name");
    for (const [group, fields] of [
      ["intro", [[c.profile.tagline, "Hello"], [c.profile.announcement, "Welcome"]]],
      ["contact", [[c.profile.email, "shop@example.test"], [c.profile.phone, "+886 2 1234 5678"], [c.profile.address, "Example street"]]],
      ["social", [[c.profile.line, "https://line.me/example"], [c.profile.facebook, "https://facebook.com/example"], [c.profile.instagram, "https://instagram.com/example"]]],
    ] as const) {
      await click(page, page.getByTestId(`design-profile-${group}-toggle`), ui.en[group], "only selected optional group expands", async () => {
        await expect(page.getByTestId(`design-profile-${group}`)).toBeVisible();
        for (const other of ["intro", "contact", "social"]) if (other !== group) await expect(page.getByTestId(`design-profile-${other}`)).toBeHidden();
      });
      for (const [label, value] of fields) await fill(page, page.getByLabel(label, { exact: true }), value, label);
      await page.getByTestId(`design-profile-${group}-toggle`).click();
      await expect(page.getByTestId(`design-profile-${group}`)).toBeHidden();
    }
    await save(page, state, locale);
    await page.reload();
    await hub(page, locale);
    await task(page, locale, "profile");
    await expect(page.getByTestId("design-name")).toHaveValue("Demo edited");
    await page.getByTestId("design-profile-social-toggle").click();
    await expect(page.getByLabel(c.profile.instagram, { exact: true })).toHaveValue("https://instagram.com/example");
    await back(page, locale);
    await task(page, locale, "home");
    densities.push(await measure(page, "home"));
    const rows = page.getByTestId("design-section");
    await expect(rows).toHaveCount(3);
    await expect(rows.nth(0).getByLabel(c.home.heading, { exact: true })).toBeHidden();
    await rows.nth(0).getByTestId("design-section-edit").click();
    await fill(page, rows.nth(0).getByLabel(c.home.heading, { exact: true }), "Welcome edited", "Section heading");
    await rows.nth(1).getByTestId("design-section-edit").click();
    await expect(rows.nth(0).getByLabel(c.home.heading, { exact: true })).toBeHidden();
    await expect(rows.nth(1).getByLabel(c.home.sort, { exact: true })).toHaveValue("newest");
    await expect(rows.nth(1).getByLabel(c.home.limit, { exact: true })).toHaveValue("8");
    await rows.nth(1).getByTestId("design-section-edit").click();
    await rows.nth(1).getByTestId("design-section-more").click();
    await rows.nth(1).getByRole("button", { name: c.nav.up, exact: true }).click();
    await expect(rows.nth(0)).toContainText("Products");
    await save(page, state, locale);
    await page.reload();
    await task(page, locale, "home");
    await expect(rows.nth(0)).toContainText("Products");
    expect(state.draft.document.home.sections[0].type).toBe("product_grid");
    await back(page, locale);
    await task(page, locale, "pages");
    densities.push(await measure(page, "pages"));
    await page.getByTestId("design-page-open").filter({ hasText: "Returns" }).click();
    await fill(page, page.getByLabel(c.pages.title, { exact: true }), "Returns edited", "Page title");
    await expect(page.getByLabel(c.pages.slug, { exact: true })).toHaveValue("returns");
    await expect(page.getByLabel(c.pages.body, { exact: true })).toHaveValue("");
    await page.getByTestId("design-page-more").click();
    await page.getByTestId("design-page-add").click();
    await fill(page, page.getByLabel(c.pages.title, { exact: true }), "About", "New page title");
    await fill(page, page.getByLabel(c.pages.slug, { exact: true }), "about", "New page URL");
    await fill(page, page.getByLabel(c.pages.body, { exact: true }), "Our story.", "New page body");
    await save(page, state, locale);
    await page.reload();
    await task(page, locale, "pages");
    await expect(page.getByTestId("design-page-open").filter({ hasText: "About" })).toBeVisible();
    expect(state.draft.document.pages.find((p) => p.slug === "returns")?.body).toBe("");
    await back(page, locale);
    await task(page, locale, "nav");
    densities.push(await measure(page, "nav"));
    const footer = page.getByTestId("design-nav-footer-row").first();
    await footer.getByTestId("design-nav-edit").click();
    await fill(page, footer.getByLabel(c.nav.label, { exact: true }), "About", "Footer link label");
    await footer.getByLabel(c.nav.target, { exact: true }).selectOption("about");
    await expect(footer.getByLabel(c.nav.target, { exact: true })).toHaveValue("about");
    await save(page, state, locale);
    await page.reload();
    await task(page, locale, "nav");
    await expect(page.getByTestId("design-nav-footer-row").first()).toContainText("About");
    expect(state.draft.document.nav.footer[0]).toEqual({ label: "About", kind: "page", target: "about" });
    await back(page, locale);
    await task(page, locale, "profile");
    await fill(page, page.getByTestId("design-name"), "Discard this", "Unsaved store name");
    await click(page, page.getByTestId("design-cancel"), "Cancel changes", "restores saved document and task hub", () => hub(page, locale));
    await task(page, locale, "profile");
    await expect(page.getByTestId("design-name")).toHaveValue("Demo edited");
    await back(page, locale);
    const sorted = (key: "blocks" | "characters") => densities.map((d) => d[key]).sort((a, b) => a - b);
    if (mobile) for (const [key, limit] of [["blocks", 12], ["characters", 100]] as const) {
      const values = sorted(key);
      expect(values[Math.floor(values.length / 2)], `mobile ${key} median`).toBeLessThanOrEqual(limit);
    }
    await test.info().attach("density", { body: JSON.stringify(densities, null, 2), contentType: "application/json" });
  });
}

for (const locale of locales) {
  test(`EDITOR-V2 ${locale}: preview saves first; publish review, receipt, view, back and cancel keep server state`, async ({ page }) => {
    const state = await open(page, locale, true);
    await hub(page, locale);
    await task(page, locale, "profile");
    await fill(page, page.getByTestId("design-name"), "Preview name", "Name before preview");
    await back(page, locale);
    const popupPromise = page.waitForEvent("popup");
    await page.getByTestId("design-preview").click();
    const popup = await popupPromise;
    await expect(popup).toHaveURL(new RegExp(`${address.replaceAll(".", "\\.")}/\\?preview=`));
    expect(state.operations).toEqual(["save", "preview"]);
    expect(state.draft.published_version).toBe(27);
    await popup.close();
    await review(page, locale);
    expect(state.attempts, "review alone never publishes").toHaveLength(0);
    // Native summary has browser-specific accessibility roles; click the labelled disclosure itself.
    const reviewDetails = page.getByTestId("design-publish-review").locator("details").first();
    const disclosure = reviewDetails.locator("summary").first();
    await expect(disclosure).toHaveText(ui[locale].details);
    await disclosure.click();
    await expect(reviewDetails).toHaveAttribute("open", "");
    const summary = page.getByTestId("design-summary");
    for (const text of ["Preview name", "#247965", "Welcome", "Products", "8", "Shipping", "Returns", "shipping", "returns", ui[locale].empty]) await expect(summary).toContainText(text);
    for (const group of ["profile", "home", "nav", "pages"]) {
      const detail = summary.getByTestId(`design-summary-${group}`);
      await expect(detail).toBeVisible();
      await click(page, detail.locator('summary'), `Review ${group}`, 'selected review group expands', () => expect(detail.locator('details')).toHaveAttribute('open', ''));
    }
    await page.getByTestId("design-publish-continue").click();
    await hub(page, locale);
    expect(state.attempts).toHaveLength(0);
    await review(page, locale);
    await page.getByTestId("design-publish-confirm").click();
    const result = page.getByTestId("design-publish-result");
    await expect(result).toContainText("28");
    await expect(result).toContainText("43");
    await expect(result).toContainText(/20:30/); // server receipt 12:30Z, displayed in Taipei despite browser LA timezone
    await expect(result).toContainText(ui[locale].scope);
    await expect(result).toContainText(ui[locale].unchanged);
    await page.getByTestId("design-view-published").click();
    await expect(page.getByTestId("design-published-content")).toContainText("28");
    await back(page, locale);
    await expect(page.getByTestId("design-status")).toContainText("28");
    await expect(page.getByTestId("design-publish")).toBeHidden();
    await task(page, locale, "home");
    await page.getByTestId("design-section-edit").first().click();
    await fill(page, page.getByTestId("design-section").first().getByLabel(designCopy[locale].home.heading, { exact: true }), "Discard", "Temporary heading");
    await page.getByTestId("design-cancel").click();
    await hub(page, locale);
    await expect(page.getByTestId("design-status")).toContainText("28");
    await expect(page.getByTestId("design-publish")).toBeHidden();
    await page.reload();
    await hub(page, locale);
    await expect(page.getByTestId("design-status")).toContainText("28");
    await expect(page.getByTestId("design-publish")).toBeHidden();
    expect(state.receipts.size).toBe(1);
  });

  test(`EDITOR-V2 ${locale}: definite refusal is distinct from uncertain original-request recovery`, async ({ page }) => {
    const state = await open(page, locale, true);
    await hub(page, locale);
    state.outcome = "forbidden";
    await review(page, locale);
    await page.getByTestId("design-publish-confirm").click();
    await expect(page.getByTestId("design-publish-result")).toContainText(ui[locale].failed);
    await expect(page.getByTestId("design-publish-result")).toContainText("27");
    expect(state.receipts.size).toBe(0);
  });

  test(`EDITOR-V2 ${locale}: uncertain result survives refresh; query retains original key/body and never offers a second publish`, async ({ page }) => {
    const state = await open(page, locale, true);
    await hub(page, locale);
    state.outcome = "uncertain";
    await review(page, locale);
    await page.getByTestId("design-publish-confirm").click();
    const pending = page.getByTestId("design-publish-pending");
    await expect(pending).toContainText(ui[locale].pending);
    await expect(pending).toContainText("42");
    await expect(pending).toContainText("27");
    await expect(page.getByTestId("design-publish")).toBeHidden();
    const original = { ...state.attempts[0] };
    expect(original.key).toBeTruthy();
    await page.reload();
    await expect(pending).toContainText(ui[locale].pending);
    await expect(page.getByTestId("design-publish")).toBeHidden();
    // Keep transport down until automatic same-key recovery settles, as in the existing OC11 journeys.
    // Otherwise automatic recovery can succeed before the merchant gets to click the query control.
    await expect(page.getByTestId("design-publish-recheck")).toHaveText(ui[locale].query);
    await expect(page.getByTestId("design-publish-recheck")).toBeEnabled();
    state.outcome = "success";
    await page.getByTestId("design-publish-recheck").click();
    await expect(page.getByTestId("design-publish-result")).toContainText("28");
    expect(state.attempts.length).toBeGreaterThan(1);
    for (const attempt of state.attempts) expect(attempt).toEqual(original);
    expect(state.receipts.size).toBe(1);
    await page.reload();
    await hub(page, locale);
    await expect(pending).toHaveCount(0);
    await expect(page.getByTestId("design-status")).toContainText("28");
    await expect(page.getByTestId("design-publish")).toBeHidden();
  });
}
