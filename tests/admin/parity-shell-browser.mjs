// Purpose: Produce real-route admin parity sheets and assert owner-chosen shell A with real shell-control journeys.
// Depends on: Playwright Chromium, node:test, shell-fixture MOCK transport and the packaged admin via next-startup.
// Used by: scripts/dev/test-local.sh --browser-admin-parity; artifacts are inputs to independent visual review.
// Invariants: no gallery, pixel baseline, DOM mutation, forced click, provider traffic or business-flow acceptance.
import assert from "node:assert/strict";
import { before, after, test } from "node:test";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { createWriteStream } from "node:fs";
import { mkdir, writeFile, copyFile } from "node:fs/promises";
import { chromium, expect } from "@playwright/test";
import { fixture, storeID, otherID } from "./shell-fixture.mjs";
import { browserEvidenceDirectory } from "../browser-evidence.mjs";
import { nextAttemptLog, startNextWithPortRetry } from "../helpers/next-startup.mjs";

// Independent signed-copy oracle: do not import the implementation's copy as expected output.
const copy = {
  "zh-TW": { nav: ["總覽", "直播", "訂單與出貨", "商品與庫存", "訊息中心", "顧客", "行銷優惠", "網路商店", "收款與報表", "設定"], menu: "開啟導覽", close: "關閉導覽", help: "說明", account: "帳號", logout: "登出", helpText: "選擇工作區入口", language: "語言", store: "切換商店" },
  "zh-CN": { nav: ["总览", "直播", "订单与发货", "商品与库存", "消息中心", "顾客", "营销优惠", "网店", "收款与报表", "设置"], menu: "打开导航", close: "关闭导航", help: "帮助", account: "账号", logout: "退出登录", helpText: "选择工作区入口", language: "语言", store: "切换店铺" },
  en: { nav: ["Overview", "Live", "Orders & shipping", "Products & inventory", "Inbox", "Customers", "Marketing", "Online store", "Payments & reports", "Settings"], menu: "Open navigation", close: "Close navigation", help: "Help", account: "Account", logout: "Sign out", helpText: "Choose a workspace section", language: "Language", store: "Switch store" },
};
const routes = [
  { name: "overview", path: "/", group: "overview", index: 0 },
  { name: "orders", path: "/orders", group: "orders", index: 2, item: "orders" },
  { name: "products", path: "/products", group: "catalog", index: 3, item: "products" },
  { name: "inbox", path: "/messages", group: "messages", index: 4 },
  { name: "customers", path: "/customers", group: "customers", index: 5, item: "customers" },
  { name: "settings", path: "/settings", group: "settings", index: 9, item: "settings" },
  { name: "storefront-editor", path: "/design", group: "storefront", index: 7 },
  { name: "live-console", path: "/studio/console", group: "live", index: 1, item: "live-console" },
];
const viewports = [{ width: 1440, height: 900 }, { width: 390, height: 844 }];
const output = browserEvidenceDirectory("admin-parity");
const ledger = [], captures = [];
const sha = execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim();
let f, browser, child, log, base;

before(async () => {
  await mkdir(`${output}/parity`, { recursive: true });
  f = await fixture();
  base = await startNextWithPortRetry({}, async ({ port, attempt, track }) => {
    const url = `http://127.0.0.1:${port}`;
    log = createWriteStream(nextAttemptLog(`${output}/server.log`, attempt), { flags: "w", mode: 0o600 });
    await once(log, "open");
    child = spawn(process.execPath, ["scripts/dev/start-admin.mjs"], {
      detached: true, stdio: ["ignore", log, log],
      env: { ...process.env, COMMERCE_UI_PORT: String(port), COMMERCE_IDENTITY_ENABLED: "1", COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS: "1", COMMERCE_PASSWORD_LOGIN_ENABLED: "1", COMMERCE_PUBLIC_ORIGIN: url, COMMERCE_API_ORIGIN: f.origin, COMMERCE_BFF_KEY: f.key },
    });
    track(child, log);
    for (let i = 0; i < 120; i++) {
      if (child.exitCode !== null || child.signalCode !== null) throw new Error("parity admin exited before readiness");
      try { if ((await fetch(`${url}/en/reset`)).ok) return url; } catch {}
      if (i === 119) throw new Error("parity admin readiness timeout");
      await new Promise((resolve) => setTimeout(resolve, 500));
    }
  });
  browser = await chromium.launch({ headless: true });
}, { timeout: 90000 });

after(async () => {
  await writeFile(`${output}/click-ledger.json`, JSON.stringify({ tier: "MOCK", sha, ledger }, null, 2) + "\n");
  await writeFile(`${output}/manifest.json`, JSON.stringify({ tier: "MOCK", sha, viewports, captures, note: "Real admin routes; synthetic store, empty or explicitly unavailable domain reads. Shell acceptance only; screenshots require independent comparison, no pixel diff." }, null, 2) + "\n");
  await browser?.close();
  try { if (child) process.kill(-child.pid, "SIGTERM"); } catch {}
  await f?.close();
  log?.end();
});

async function session(viewport) {
  const context = await browser.newContext({ viewport });
  // Fixture setup only: synthetic auth cookies; all shell actions below use real controls.
  await context.addCookies(["session", "csrf"].map((kind) => ({ name: `__Host-commerce_${kind}`, value: f.token, domain: "127.0.0.1", path: "/", secure: true, httpOnly: kind === "session", sameSite: "Lax" })));
  return { context, page: await context.newPage() };
}
async function ready(page, route) {
  await expect(page.locator(`[data-shell-route="${route}"]`)).toBeAttached();
  await expect(page.getByTestId("shell-store-name")).toHaveText("W0 合成測試店 · Synthetic baseline");
  await expect(page.getByTestId("route-forbidden")).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Application error", exact: false })).toHaveCount(0);
  // READ only: font readiness makes screenshots reproducible without changing the DOM.
  await page.evaluate(() => document.fonts.ready.then(() => undefined));
}
async function step(page, control, action, expected, run) {
  const row = { page: new URL(page.url()).pathname, control, action, expected, actual: "", result: "fail" };
  try { await run(); row.actual = expected; row.result = "pass"; }
  catch (error) { row.actual = String(error.message).slice(0, 500); throw error; }
  finally { ledger.push(row); }
}
async function openMenu(page, locale, viewport) {
  if (viewport.width < 1024) await page.getByRole("button", { name: copy[locale].menu, exact: true }).click();
  await expect(page.locator("[data-shell-rail]")).toBeVisible();
}
function group(page, route, locale) {
  return page.locator("[data-shell-rail]").getByRole("button", { name: copy[locale].nav[route.index], exact: true });
}

for (const [locale, c] of Object.entries(copy)) for (const viewport of viewports) {
  const size = `${viewport.width}x${viewport.height}`;
  for (const route of routes) test(`PARITY-SHELL ${locale} ${route.name} ${size}`, { timeout: 30000 }, async () => {
    const { context, page } = await session(viewport);
    try {
      const response = await page.goto(`${base}/${locale}${route.path}?store=${storeID}`, { waitUntil: "load" });
      assert.equal(response.status(), 200, "the REAL route must load (never /ui-gallery)");
      await ready(page, route.path);
      const file = `parity/${route.name}-${size}${locale === "zh-TW" ? "" : `-${locale}`}.png`;
      await page.screenshot({ path: `${output}/${file}`, animations: "disabled" });
      captures.push({ route: route.path, locale, viewport, file, state: (await page.locator("#main").innerText()).slice(0, 400) });
      if (locale === "zh-TW" && route.name === "orders") {
        const frame = viewport.width === 1440 ? "1429-29491" : "1429-28313";
        await copyFile(`${output}/${file}`, `${output}/parity/${frame}.png`);
      }
      await openMenu(page, locale, viewport);
      const rail = page.locator("[data-shell-rail]");
      // READ/MEASURE only: the legacy dark navy must be gone on the REAL shell.
      assert.equal(await rail.evaluate((el) => getComputedStyle(el).backgroundColor), "rgb(255, 255, 255)", "shell A: white sidebar replaces legacy dark navy");
      await expect(rail.getByTestId("shell-platform-brand")).toHaveText("DaWan Live");
      for (const name of c.nav) await expect(rail.getByRole("button", { name, exact: true })).toBeVisible();
      const active = group(page, route, locale);
      assert.equal(await active.evaluate((el) => getComputedStyle(el).backgroundColor), "rgb(234, 246, 242)", "signed active navigation fill #eaf6f2");
      const palette = await page.locator("[data-ui-shell]").evaluate((el) => {
        const css = getComputedStyle(el);
        return { primary: css.getPropertyValue("--ui-primary").trim(), width: document.querySelector("[data-shell-rail]").getBoundingClientRect().width };
      });
      assert.equal(palette.primary.toLowerCase(), "#247965", "admin primary must resolve to the signed green token");
      assert.equal(palette.width, 240, "signed 240px sidebar");
      if (viewport.width === 1440) {
        const store = await page.getByTestId("shell-store-selector").boundingBox();
        const sidebar = await rail.boundingBox();
        assert.ok(store && sidebar && store.x >= sidebar.x && store.x + store.width <= sidebar.x + sidebar.width, "desktop store switcher belongs below the sidebar brand");
        await expect(page.locator("[data-shell-topbar] [data-shell-route]")).toHaveCount(1);
      } else {
        await page.keyboard.press("Escape");
        await expect(rail).toBeHidden();
        await expect(page.getByRole("button", { name: c.menu, exact: true })).toBeFocused();
      }
    } finally { await context.close(); }
  });

  test(`PARITY-SHELL real shell clicks ${locale} ${size}`, { timeout: 90000 }, async () => {
    const { context, page } = await session(viewport);
    try {
      await page.goto(`${base}/${locale}/?store=${storeID}`, { waitUntil: "load" });
      await ready(page, "/");
      for (const route of routes) await step(page, c.nav[route.index], "click group and destination", `real ${route.path} remains after refresh`, async () => {
        await openMenu(page, locale, viewport);
        const button = group(page, route, locale);
        if (await button.getAttribute("aria-expanded") === "false") await button.click();
        if (await button.getAttribute("aria-expanded") !== null) {
          assert.ok(route.item, `multi-route group ${route.group} needs its real destination`);
          await page.getByTestId(`nav-${route.item}`).click();
        } else await button.click();
        await page.waitForURL((url) => url.pathname.replace(/\/$/, "") === `/${locale}${route.path}`.replace(/\/$/, ""));
        await ready(page, route.path);
        await page.reload({ waitUntil: "load" });
        await ready(page, route.path);
      });
      if (viewport.width === 1440) await step(page, c.nav[0], "click breadcrumb, refresh", "overview breadcrumb navigates and persists", async () => {
        await page.locator("[data-shell-topbar] [data-shell-route]").getByRole("link", { name: c.nav[0], exact: true }).click();
        await ready(page, "/");
        await page.reload({ waitUntil: "load" });
        await ready(page, "/");
      });
      // Every group disclosure is exercised; content module controls belong to their own cards.
      await openMenu(page, locale, viewport);
      for (const name of c.nav) {
        const control = page.locator("[data-shell-rail]").getByRole("button", { name, exact: true });
        if (await control.getAttribute("aria-expanded") !== null) await step(page, name, "expand, collapse", "disclosure state follows real clicks", async () => {
          if (await control.getAttribute("aria-expanded") === "true") await control.click();
          await control.click(); await expect(control).toHaveAttribute("aria-expanded", "true");
          await control.click(); await expect(control).toHaveAttribute("aria-expanded", "false");
        });
      }
      if (viewport.width < 1024) await step(page, c.close, "click drawer close", "navigation closes", async () => {
        await page.locator("[data-shell-rail]").getByRole("button", { name: c.close, exact: true }).click();
        await expect(page.locator("[data-shell-rail]")).toBeHidden();
      });
      for (const [control, text] of [[c.help, c.helpText], [c.account, c.logout]]) await step(page, control, "open and close", "popover content appears then closes", async () => {
        const summary = page.locator("summary").filter({ hasText: control });
        await summary.click();
        await expect(summary.locator("..")).toHaveAttribute("open", "");
        await expect(summary.locator("..")).toContainText(text);
        await summary.click();
        await expect(summary.locator("..")).not.toHaveAttribute("open");
      });
      await step(page, c.store, "select second store, refresh", "authenticated selection persists in URL and visible store", async () => {
        await page.getByRole("combobox", { name: c.store, exact: true }).selectOption(otherID);
        await page.waitForURL((url) => url.searchParams.get("store") === otherID);
        await expect(page.getByTestId("shell-store-name")).toContainText("A very long English store name");
        await page.reload({ waitUntil: "load" });
        await expect(page.getByTestId("shell-store-name")).toContainText("A very long English store name");
        await page.getByRole("combobox", { name: c.store, exact: true }).selectOption(storeID);
        await page.waitForURL((url) => url.searchParams.get("store") === storeID);
        await ready(page, "/");
      });
      const next = locale === "en" ? "zh-TW" : "en";
      await step(page, c.language, "select locale, refresh", `locale ${next} persists with its visible navigation`, async () => {
        await page.getByRole("combobox", { name: c.language, exact: true }).selectOption(next);
        await page.waitForURL((url) => url.pathname.startsWith(`/${next}`));
        await page.reload({ waitUntil: "load" });
        await ready(page, "/");
        await openMenu(page, next, viewport);
        await expect(group(page, routes[0], next)).toBeVisible();
        if (viewport.width < 1024) await page.keyboard.press("Escape");
      });
      await step(page, copy[next].logout, "account, sign out", "signed-out entry remains after refresh", async () => {
        await page.locator("summary").filter({ hasText: copy[next].account }).click();
        // Wait for the actual document navigation; the session fence hides controls before logout finishes.
        await Promise.all([
          page.waitForEvent("load"),
          page.getByTestId("workspace-sign-out").click(),
        ]);
        assert.equal(new URL(page.url()).pathname.replace(/\/$/, ""), `/${next}`);
        assert.equal(new URL(page.url()).searchParams.has("store"), false);
        await expect(page.getByTestId("shell-store-selector")).toHaveCount(0);
        await page.reload({ waitUntil: "load" });
        await expect(page.getByTestId("shell-store-selector")).toHaveCount(0);
      });
    } finally { await context.close(); }
  });
}
