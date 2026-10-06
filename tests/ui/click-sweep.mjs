// G-UI8 click-sweep runner (owner 2026-10-03, docs/engineering/ui-architecture.md 10.6b), started by tests/foundation/browser_click_sweep_test.go
// (`bash scripts/dev/test-local.sh --browser-click-sweep`). One real stack: production admin Next (signed MOCK IdP) + production storefront Next (started
// here behind a synthetic TLS edge for the virtual host) over the real Go API and an isolated PG. This process never sees a database credential.
//   1. admin: EVERY route of apps/admin/src/routes.ts (the registry) at 1586x992 and 390x844 in zh-TW, plus en at desktop, owner role
//   2. storefront: every buyer route of docs/engineering/ui-architecture.md section 4, same three variants
//   3. journeys by clicks: J1 create a product in the editor and see it on the storefront; J2 place a COD order on the storefront and see it in
//      admin orders (+ the order lookup); J3 record a manual shipment, then collected; J4 add a custom domain and see the DNS instructions; J5 sign out
// Every interaction is a real Playwright action (click / fill / selectOption / check): no force, no dispatchEvent, no API shortcut, page.evaluate only
// reads and measures (tests/ui/click-sweep-lib.mjs). Output: <LC_SWEEP_OUT>/ledger.{json,md}, screenshots of failing controls, journeys.json.
// Exit 1 when any control fails (no effect / console error / page error / 5xx / unhittable / destructive without confirmation) or a known-defect entry
// is stale. tests/ui/click-sweep-known-defects.json is the explicit list (id, route, kind, owner unit): it classifies, it never silences.
// Env: LC_SWEEP_ADMIN_ORIGIN LC_SWEEP_STORE LC_SWEEP_EVIDENCE LC_SWEEP_OUT LC_SWEEP_FACTS LC_SWEEP_CONTROL LC_SWEEP_CONTROL_KEY COMMERCE_BUYER_*;
// optional LC_SWEEP_ONLY=admin,storefront,journeys  LC_SWEEP_PAGES=<route substrings>  LC_SWEEP_WORKERS=3  LC_SWEEP_SAMPLE=3 (alike controls per class).
import assert from "node:assert/strict";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { expect } from "@playwright/test";
import { launch } from "../storefront/browser-engine.mjs";
import { routes as adminRoutes } from "../../apps/admin/src/routes.ts";
import {
  CANCEL_RE, INIT_SCRIPT, LAYER_CSS, Ledger, classKey, controlLabel, controlLocator, isDestructive, isIrreversible, isSignOut, listControls, matchKnown, monitor,
  degradedMessages, pageState, restore, settle, sweepControl, writeLedger,
} from "./click-sweep-lib.mjs";

// G-UI9 (unit ui-visual-audit): the same seeded stack, a different runner. `test-local.sh --browser-visual-lint` sets LC_SWEEP_ONLY=visual-audit, and the Go test
// (browser_click_sweep_test.go) starts this file; the audit (screenshot corpus + layout lint) takes over before the click sweep reads anything.
if (process.env.LC_SWEEP_ONLY === "visual-audit") process.exit(await (await import("./visual-audit.mjs")).main());

const env = (name) => { const v = process.env[name]; assert(v, `${name} is required`); return v; };
const adminOrigin = env("LC_SWEEP_ADMIN_ORIGIN"), store = env("LC_SWEEP_STORE"), evidence = env("LC_SWEEP_EVIDENCE"), outDir = env("LC_SWEEP_OUT");
const facts = JSON.parse(await readFile(env("LC_SWEEP_FACTS"), "utf8"));
const control = { url: env("LC_SWEEP_CONTROL"), key: env("LC_SWEEP_CONTROL_KEY") };
const only = new Set((process.env.LC_SWEEP_ONLY || "").split(",").filter(Boolean));
const pageFilter = (process.env.LC_SWEEP_PAGES || "").split(",").filter(Boolean);
const WORKERS = Number(process.env.LC_SWEEP_WORKERS || 3), SAMPLE = Number(process.env.LC_SWEEP_SAMPLE || 3), LAYER_ITEMS = 6;
const run = (part) => only.size === 0 || only.has(part);
const wanted = (route) => pageFilter.length === 0 || pageFilter.some((f) => (f.startsWith("=") ? route === f.slice(1) : route.includes(f))); // "=/" selects exactly the route /
const origin = facts.store_origin, HOST = new URL(origin).host;
const t0 = Date.now();
const log = (...a) => console.log(`[${String(Math.round((Date.now() - t0) / 1000)).padStart(4)}s]`, ...a);
const ledger = new Ledger();
const known = JSON.parse(await readFile(new URL("./click-sweep-known-defects.json", import.meta.url), "utf8"));
await mkdir(path.join(outDir, "screenshots"), { recursive: true });

// ---- variants -----------------------------------------------------------------------------------------------------------------------------------------------
const VARIANTS = [
  { viewport: "desktop", size: { width: 1586, height: 992 }, locale: "zh-TW", mobile: false },
  { viewport: "mobile", size: { width: 390, height: 844 }, locale: "zh-TW", mobile: true },
  { viewport: "desktop", size: { width: 1586, height: 992 }, locale: "en", mobile: false },
];
const ctxOptions = (v, extra = {}) => ({ viewport: v.size, screen: v.size, isMobile: v.mobile, hasTouch: v.mobile, locale: v.locale === "en" ? "en-US" : "zh-TW", deviceScaleFactor: 1, ignoreHTTPSErrors: true, ...extra });

// ---- storefront process: production Next + synthetic https edge + CONNECT-only proxy (the pattern of tests/storefront/shop-real-gate.mjs) ----------------------
const children = new Set(), sockets = new Set(), logs = [];
const listen = async (server) => { server.listen(0, "127.0.0.1"); await once(server, "listening"); return server.address().port; };
function relay(port, req, body) {
  return new Promise((resolve, reject) => {
    const headers = { ...req.headers }; delete headers.connection; delete headers["transfer-encoding"];
    if (body.length) headers["content-length"] = String(body.length); else delete headers["content-length"];
    const call = http.request({ hostname: "127.0.0.1", port, path: req.url, method: req.method, headers }, (r) => {
      const chunks = []; r.on("data", (x) => chunks.push(x)); r.on("end", () => resolve({ status: r.statusCode, headers: r.headers, body: Buffer.concat(chunks) })); r.on("error", reject);
    });
    call.setTimeout(60000, () => call.destroy(new Error("relay timeout"))); call.on("error", reject); call.end(body);
  });
}
async function startNext() {
  const file = createWriteStream(path.join(evidence, "storefront-next.log"), { flags: "wx", mode: 0o600 }); logs.push(file); await once(file, "open");
  const probe = net.createServer(); const port = await listen(probe); await new Promise((r) => probe.close(r));
  const childEnv = { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1" };
  for (const key of Object.keys(childEnv)) if (key.startsWith("LC_SWEEP_")) delete childEnv[key];
  const child = spawn(process.execPath, [path.join(process.cwd(), "apps/storefront/node_modules/next/dist/bin/next"), "start", "--hostname", "127.0.0.1", "--port", String(port)],
    { cwd: path.join(process.cwd(), "apps/storefront"), env: childEnv, stdio: ["ignore", file, file] });
  children.add(child);
  for (let i = 0; i < 400; i++) {
    if (child.exitCode !== null) throw new Error("storefront Next exited before readiness");
    try { if ((await relay(port, { url: "/robots.txt", method: "GET", headers: { host: HOST } }, Buffer.alloc(0))).status === 200) return port; } catch { /* not up yet */ }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error("storefront Next readiness timeout");
}
let browser, edge, proxy;
const certDir = await mkdtemp(path.join(tmpdir(), "lc-sweep-edge-"));
const fingerprint = async () => {
  const r = await fetch(`${control.url}/fingerprint`, { headers: { "X-Gate-Key": control.key } });
  return r.status === 200 ? await r.text() : null;
};

// ---- the generic page sweep ---------------------------------------------------------------------------------------------------------------------------------
const rowBase = (unit, scope, c, extra) => ({ app: unit.app ?? "", page: unit.route, viewport: unit.viewport, locale: unit.locale, scope, control: c, testid: "", kind: "page", action: "-", expected: "", actual: "", result: "pass", failure: "", note: "", classSize: 1, ...extra });
async function shot(page, row) {
  const file = `screenshots/${row.id}.png`;
  await page.screenshot({ path: path.join(outDir, file), fullPage: false, animations: "disabled" }).catch(() => {});
  row.screenshot = `output/ui-click-sweep/${file}`;
}
const sample = (items, n) => {
  if (items.length <= n) return items;
  const pick = new Set([0, items.length - 1]);
  for (let i = 1; pick.size < n; i++) pick.add(Math.floor((i * (items.length - 1)) / n));
  return [...pick].sort((a, b) => a - b).map((i) => items[i]);
};
const hay = (d) => [d.name, d.testid, d.title, d.ariaLabel].join(" ");
const COMMIT_RE = /confirm|確認|确认|submit|送出|提交|save|儲存|保存|^ok$|^yes$|^是$|套用|apply/i;

// scopes: [{ name, selector, exclude?, prepare?(page) }]; the unit: { route, url, viewport, locale }.
// make() -> { page, mon, close }: a browser context with the sweep's monitor. fresh: after every control the page is rebuilt in a NEW context (empty
// client storage: a wizard step or draft a click left in sessionStorage never shifts the next control); otherwise the page is reloaded in place (the
// storefront keeps its cart in the context).
async function sweepPage({ make, unit, scopes, fresh = false, session = null }) {
  let cur = session ?? await make();
  const ctx = { page: cur.page, mon: cur.mon, unit, rootSelector: "body", fingerprint, window: 3000 };
  const open = async () => {
    await cur.page.goto(unit.url, { waitUntil: "domcontentloaded" }).catch(() => {});
    await settle(cur.page, cur.mon);
  };
  const back = async (row) => {
    if (fresh) {
      await cur.close().catch(() => {});
      cur = await make(); ctx.page = cur.page; ctx.mon = cur.mon;
      await open();
    } else await restore(cur.page, cur.mon, unit.url, row ?? { layer: false });
  };
  try {
    cur.mon.seenAt.clear(); // the storm check counts the requests of THIS page load only
    const response = await cur.page.goto(unit.url, { waitUntil: "domcontentloaded" }).catch((e) => ({ error: e }));
    const quiet = await settle(cur.page, cur.mon, { max: 8000 });
    const status = response?.status?.() ?? 0;
    const lr = ledger.add(rowBase(unit, "page", "(page load)", { expected: "the route renders: HTTP < 400, a heading, no console/page error, no degraded-state message", actual: `HTTP ${status}${quiet ? "" : " (keeps updating)"}` }));
    const headings = await cur.page.locator("h1, h2").count();
    if (unit.expectExternal) {
      // a route whose job is to hand the browser to another host (the label print page auto-posts the signed form to ECPay): the sweep's network edge
      // answers a stub; the page must have tried exactly that host.
      const hit = cur.mon.since(0).find((e) => e.type === "external" && unit.expectExternal.test(e.text));
      lr.expected = `the route hands the browser to ${unit.expectExternal}`;
      if (hit) { lr.actual = `HTTP ${status}; ${hit.text} (blocked by the sweep)`; return; }
    }
    const early = cur.mon.since(0).filter((e) => ["console-error", "pageerror", "http5xx", "http404-asset"].includes(e.type) && e.url !== "about:blank");
    if (status >= 400 || status === 0) { lr.result = "fail"; lr.failure = "page-load"; lr.actual = `HTTP ${status}`; }
    else if (early.length) { lr.result = "fail"; lr.failure = early[0].type; lr.actual += ` | ${early.map((e) => `${e.type}: ${e.text}`).join(" ; ").slice(0, 300)}`; }
    else if (!headings) { lr.result = "fail"; lr.failure = "page-load"; lr.actual += " | no heading on the page"; }
    const storm = cur.mon.storm();
    if (lr.result === "pass" && storm) { lr.result = "fail"; lr.failure = "request-storm"; lr.actual += ` | ${storm.count} x ${storm.path} within seconds: the page re-fetches itself in a loop and never settles`; }
    if (lr.result === "pass" && !unit.expectDegraded) {
      const degraded = await degradedMessages(cur.page);
      if (degraded.length) { lr.result = "fail"; lr.failure = "page-error-state"; lr.actual += ` | the page opens in a degraded state: ${degraded.map((d) => `[${d.role}${d.testid ? " " + d.testid : ""}] ${d.text}`).join(" ; ").slice(0, 280)}`; }
    }
    if (lr.result === "fail") await shot(cur.page, lr);
    if (status >= 400 || status === 0) return;
    for (const [si, scope] of scopes.entries()) {
      ctx.rootSelector = scope.selector;
      if (si > 0) await back();
      await scope.prepare?.(cur.page);
      const all = await listControls(cur.page, scope.selector, scope.name, scope.exclude ?? "");
      const groups = new Map();
      for (const d of all) { const k = classKey(d); if (!groups.has(k)) groups.set(k, []); groups.get(k).push(d); }
      const picked = [];
      for (const g of groups.values()) for (const d of sample(g, SAMPLE)) picked.push([d, g.length]);
      picked.sort((a, b) => a[0].index - b[0].index);
      log(`${unit.route} ${unit.viewport}/${unit.locale} [${scope.name}] ${all.length} controls, ${picked.length} classes sampled`);
      for (const [d, size] of picked) {
        if (isSignOut(hay(d))) { ledger.add({ ...rowBase(unit, scope.name, controlLabel(d), { kind: `${d.tag}`, testid: d.testid, action: "-", expected: "sign out is exercised once, last, in a throwaway context (journey J5)", actual: "not clicked here: it would end the sweep's own session", result: "skip", classSize: size }) }); continue; }
        const row = ledger.add(await sweepControl(ctx, d, size));
        if (row.result === "fail") await shot(cur.page, row);
        let layerSel = "";
        if (row.result === "pass" && !isDestructive(hay(d)) && !row.urlChanged) {
          if (d.tag === "summary") layerSel = await cur.page.evaluate(([r, i]) => window.__sweepHelpers.summaryPath(r, i), [scope.selector, d.index]).catch(() => "");
          else if (row.layer) layerSel = await cur.page.evaluate(() => window.__sweepHelpers.layerPath()).catch(() => "");
        }
        if (layerSel) {
          const all = await listControls(cur.page, layerSel, "layer");
          const confirmation = all.some((i) => CANCEL_RE.test(i.name) || CANCEL_RE.test(i.ariaLabel)); // a layer with a cancel path is a confirmation dialog: only its cancel path is pressed
          const items = all.filter((i) => !CANCEL_RE.test(i.name));
          for (const item of sample(items, LAYER_ITEMS)) {
            if (confirmation && COMMIT_RE.test(hay(item))) {
              ledger.add(rowBase(unit, `layer of ${controlLabel(d)}`, controlLabel(item), { kind: item.tag, testid: item.testid, expected: "the commit button of a confirmation dialog is never pressed by the sweep", actual: "not pressed (the dialog's cancel path is exercised by the restore)", result: "skip" }));
              continue;
            }
            await back(); await scope.prepare?.(cur.page);
            await controlLocator(cur.page, scope.selector, d).click({ timeout: 4000 }).catch(() => {});
            await settle(cur.page, cur.mon, { quiet: 200, max: 2500 });
            ctx.rootSelector = layerSel;
            const commit = COMMIT_RE.test(hay(item));
            const irow = ledger.add(await sweepControl(ctx, { ...item, scope: `layer of ${controlLabel(d)}` }, 1, { commit }));
            irow.scope = `layer of ${controlLabel(d)}`;
            if (irow.result === "fail") await shot(cur.page, irow);
            ctx.rootSelector = scope.selector;
          }
        }
        await back(row); await scope.prepare?.(cur.page);
      }
    }
  } finally { if (!session) await cur.close().catch(() => {}); }
}

// ---- admin ---------------------------------------------------------------------------------------------------------------------------------------------------
const orderIDs = facts.orders;
function adminURL(route, locale) {
  const q = (extra = "") => `?store=${store}${extra}`;
  const p = route.path;
  const sub = {
    "/products/[product]": `/products/${facts.products[0].id}`, "/customers/[customer]": `/customers/${facts.customer}`, "/invite/[token]": `/invite/${"A".repeat(43)}`,
    "/studio/claims": "/studio/claims", "/orders/cvs-print": "/orders/cvs-print",
  }[p] ?? p;
  if (route.public) return `${adminOrigin}/${locale}${sub}`;
  if (p === "/studio/claims") return `${adminOrigin}/${locale}${sub}${q(`&scene=${facts.session}`)}`;
  if (p === "/orders/cvs-print") return `${adminOrigin}/${locale}${sub}${q(`&order=${orderIDs.cvs_label_created}&thermal=0`)}`;
  return `${adminOrigin}/${locale}${sub === "/" ? "" : sub}${q()}`;
}
const shellMenu = async (page) => { const b = page.locator('button[aria-controls="workspace-navigation"]'); if (await b.isVisible().catch(() => false)) { await b.click().catch(() => {}); await page.waitForTimeout(250); } };

async function loginState() {
  const c = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await c.newPage();
  await page.goto(`${adminOrigin}/en/`);
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("nav-orders")).toBeVisible();
  const state = await c.storageState();
  await c.close();
  return state;
}

async function adminUnit(state, route, v, firstOfVariant) {
  const unit = { app: "admin", route: route.path, url: adminURL(route, v.locale), viewport: v.viewport, locale: v.locale, expectExternal: route.path === "/orders/cvs-print" ? /ecpay/i : null };
  const make = async () => {
    const c = await browser.newContext(ctxOptions(v, route.public ? {} : { storageState: state }));
    await c.addInitScript(INIT_SCRIPT);
    const mon = monitor(c, { allowedHosts: new Set([new URL(adminOrigin).host, HOST]) });
    const page = await c.newPage();
    return { page, mon, close: () => c.close() };
  };
  try {
    const scopes = [{ name: "main", selector: "main#main" }];
    if (route.public) scopes[0] = { name: "main", selector: "body" };
    else if (firstOfVariant) {
      // the shell chrome is the same on every route: swept once per viewport/locale (here), the nav links are then exercised again by every route load
      scopes.unshift({ name: "skip", selector: "[data-ui-shell]", exclude: "aside, header, main" }, { name: "topbar", selector: "header[data-shell-topbar]" }, { name: "rail", selector: "aside[data-shell-rail]", prepare: shellMenu });
    }
    await sweepPage({ make, unit, scopes, fresh: true });
  } catch (e) {
    ledger.add(rowBase(unit, "page", "(sweep)", { result: "fail", failure: "sweep-error", actual: String(e.stack || e).slice(0, 400) }));
  }
}

// ---- storefront ----------------------------------------------------------------------------------------------------------------------------------------------
async function storefrontSession(v, placed) {
  const c = await browser.newContext(ctxOptions(v));
  await c.addInitScript(INIT_SCRIPT);
  const mon = monitor(c, { allowedHosts: new Set([HOST, new URL(adminOrigin).host]) });
  const page = await c.newPage();
  const session = { page, mon, close: async () => {} }; // one context for the whole walk: the cart lives in it
  const sweep = (unit, scopes) => sweepPage({ make: async () => session, unit: { app: "storefront", ...unit }, scopes, session });
  const base = `${origin}/${v.locale}`;
  const main = [{ name: "main", selector: "#main" }];
  const chrome = { name: "chrome", selector: "body", exclude: "#main" };
  const product = facts.products[1];
  const list = [
    { route: "/", path: "", scopes: [chrome, ...main] },
    { route: "/products", path: "/products" },
    { route: "/collections", path: "/collections" },
    { route: "/collections/[slug]", path: `/collections/${facts.collections[0].slug}` },
    { route: "/products/[slug]", path: `/products/${product.slug}` },
    { route: "/search", path: `/search?q=${encodeURIComponent("Sweep")}` },
    { route: "/orders/lookup", path: "/orders/lookup" },
    { route: "/order-link", path: "/order-link", expectDegraded: true }, // a link page opened without its token shows the refused-link view by design
    { route: "/claim", path: "/claim", expectDegraded: true },
    { route: "/legal/anti-fraud", path: "/legal/anti-fraud" },
    { route: "/legal/[slug]", path: "/legal/refunds" },
    { route: "/privacy", path: "/privacy" },
    { route: "/data-deletion", path: "/data-deletion" },
    { route: "/pages/[slug]", path: "/pages/about" },
  ];
  try {
    // cart, checkout and the order page first: they need a pristine cart, which the buyer fills with real clicks (also the first steps of journey J2)
    if (wanted("/cart") || wanted("/checkout") || wanted("/orders/[orderID]") || run("journeys")) {
      await addToCartByClicks(page, v, product);
      if (wanted("/cart")) await sweep({ route: "/cart", url: `${base}/cart`, viewport: v.viewport, locale: v.locale }, main);
      if (wanted("/checkout")) await sweep({ route: "/checkout", url: `${base}/checkout`, viewport: v.viewport, locale: v.locale }, main);
      const order = await placeCodOrder(page, v, product);
      placed.set(`${v.viewport}/${v.locale}`, order);
      if (wanted("/orders/[orderID]")) await sweep({ route: "/orders/[orderID]", url: `${base}/orders/${order.id}`, viewport: v.viewport, locale: v.locale }, main);
    }
    for (const r of list) {
      if (!wanted(r.route)) continue;
      await sweep({ route: r.route, url: `${base}${r.path}`, viewport: v.viewport, locale: v.locale, expectDegraded: !!r.expectDegraded }, r.scopes ?? main);
    }
  } catch (e) {
    const row = ledger.add(rowBase({ app: "storefront", route: "/(storefront)", viewport: v.viewport, locale: v.locale }, "page", "(sweep)", { result: "fail", failure: "sweep-error", actual: String(e.stack || e).slice(0, 400) }));
    await shot(page, row);
  } finally { await c.close().catch(() => {}); }
}

// ---- journeys by clicks ------------------------------------------------------------------------------------------------------------------------------------------
const J = { skipRest: new Set() };
async function step(unit, name, expected, fn, page) {
  if (J.skipRest.has(unit.journey)) { ledger.add(rowBase(unit, "journey", name, { kind: "journey", expected, actual: "not run: an earlier step of this journey failed", result: "skip" })); return undefined; }
  const row = ledger.add(rowBase(unit, "journey", name, { kind: "journey", action: "real clicks", expected }));
  try { const out = await fn(); row.actual = typeof out === "string" ? out : "as expected"; return out; }
  catch (e) { row.result = "fail"; row.failure = "journey-step"; row.actual = String(e.message || e).replace(/\x1b\[[0-9;]*m/g, "").split("\n").filter((l) => l.trim()).slice(0, 8).join(" ").replace(/\s+/g, " ").slice(0, 420); J.skipRest.add(unit.journey); if (page) await shot(page, row); return undefined; }
}
const copy = {
  "zh-TW": { delivery: "選擇配送", quote: "取得目前總額" },
  en: { delivery: "Choose delivery", quote: "Get current total" },
};
const pii = { recipient_name: "Synthetic Sweep Recipient", phone: "+886900000091", region: "Synthetic Region", city: "Synthetic City", postal_code: "99991", line1: "Synthetic Address Ninety One", line2: "Synthetic Unit Ninety Two" };

async function addToCartByClicks(page, v, product) {
  await page.goto(`${origin}/${v.locale}/products`);
  const card = page.getByTestId("product-card").filter({ hasText: product.title }).first();
  await card.click();
  await page.waitForURL(`**/${v.locale}/products/${product.slug}`);
  await page.getByTestId("add-to-cart").click();
  await page.getByTestId("cart-checkout").waitFor();
}
// product page -> cart drawer -> checkout -> delivery (home) -> quote -> address -> cash on delivery -> place -> order page, by clicks
async function placeCodOrder(page, v, product) {
  const c = copy[v.locale];
  await page.goto(`${origin}/${v.locale}/cart`);
  await page.getByTestId("cart-checkout").click();
  await page.waitForURL(`**/${v.locale}/checkout`);
  await page.getByRole("button", { name: c.delivery, exact: true }).click();
  const select = page.locator("select#delivery");
  await expect(select).toBeVisible();
  const options = await select.locator("option").evaluateAll((os) => os.map((o) => ({ value: o.value, text: o.textContent }))); // read-only: the option list
  const home = options.find((o) => o.value && !/7-ELEVEN|全家|FamilyMart|萊爾富|Hi-Life|OK mart|OK超商/i.test(o.text));
  assert(home, `no home delivery option in ${JSON.stringify(options)}`);
  await select.selectOption(home.value);
  await page.getByRole("button", { name: c.quote, exact: true }).click();
  await expect(page.getByTestId("address-section")).toBeVisible();
  for (const [key, value] of Object.entries(pii)) await page.locator(`input[name="${key}"]`).fill(value);
  const radio = page.locator('input[name="home_payment_mode"][value="cash_on_delivery"]');
  await expect(radio).toBeVisible();
  await radio.check();
  await page.getByTestId("confirm-address").click();
  await expect(page.getByTestId("create-order")).toBeEnabled();
  await page.getByTestId("create-order").click();
  await expect(page.getByTestId("order-section")).toBeVisible({ timeout: 30000 });
  const id = (await page.getByTestId("order-id").innerText()).trim();
  assert.match(id, /^[a-f0-9-]{36}$/);
  await expect(page.getByTestId("order-cod")).toBeVisible({ timeout: 30000 });
  return { id, title: product.title };
}

async function journeyProduct(state, journeys) {
  const unit = { app: "journey", journey: "J1", route: "J1 product editor -> storefront", viewport: "desktop", locale: "zh-TW" };
  const tag = facts.journey_tag, title = `Sweep Journey Tee ${tag}`;
  const c = await browser.newContext(ctxOptions(VARIANTS[0], { storageState: state }));
  const page = await c.newPage();
  const u = (p) => `${adminOrigin}/zh-TW${p}${p.includes("?") ? "&" : "?"}store=${store}`;
  page.on("response", async response => {
    if(/\/(warehouses|products\/document)$/.test(new URL(response.url()).pathname)) {
      const data=await response.json().catch(()=>null);
      log(`J1 catalog response ${response.status()} ${new URL(response.url()).pathname}: ${JSON.stringify(data)}`);
    }
  });
  try {
    await step(unit, "open the product list and click New product", "the create form opens", async () => {
      await page.goto(u("/products")); await expect(page.getByTestId("products-page")).toBeVisible();
      await page.getByTestId("product-new").click(); await expect(page).toHaveURL(/\/products\/new/); await expect(page.getByTestId("product-create-form")).toBeVisible();
    }, page);
    await step(unit, "fill the name and description", "the draft fields retain the entered values", async () => {
      await page.getByTestId("product-name").fill(title);
      await page.getByTestId("product-description").fill("Synthetic click-sweep journey product");
      await expect(page.getByTestId("product-name")).toHaveValue(title);
    }, page);
    await step(unit, "add Size = S, M", "the editor proposes two SKU rows", async () => {
      await page.getByTestId("axis-add").click();
      await page.getByTestId("axis-name-0").fill("Size");
      await page.getByTestId("axis-values-0").fill("S, M");
      await page.getByTestId("axis-values-0").press("Enter");
      await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(2);
    }, page);
    await step(unit, "type both SKU prices", "the matrix retains both prices", async () => {
      await page.getByTestId("new-price-0").fill("450");
      await page.getByTestId("new-price-1").fill("480");
      await expect(page.getByTestId("new-price-1")).toHaveValue("480");
    }, page);
    await step(unit, "set opening quantities 9 and 7, then save the document", "both SKU quantities persist", async () => {
      await page.getByTestId("matrix-quantity-0").fill("9");
      await page.getByTestId("matrix-quantity-1").fill("7");
      if(await page.getByTestId("product-warehouse").count()){
        await page.locator("#shipping summary").click();
        const warehouse=page.getByTestId("product-warehouse");
        const value=await warehouse.locator('option').nth(1).getAttribute("value");
        await warehouse.selectOption(value);
      }
      await page.getByTestId("product-create").click();
      await expect(page.getByTestId("product-save-result")).toBeVisible();
      await page.getByTestId("product-save-result").locator('a[href*="/products/"]').click();
      await page.waitForURL(/\/products\/[0-9a-f-]{36}/);
      await page.reload();
      const editURL=page.url(), codes=await Promise.all([0,1].map(i=>page.getByTestId(`matrix-code-${i}`).inputValue()));
      for(const [index,qty] of [[0,9],[1,7]]){
        await page.goto(u("/inventory"));
        await page.locator(".ledger-card .search-field input").fill(codes[index]);
        await page.locator('.ledger-card .search-field button[type="submit"]').click();
        await expect(page.locator("tbody tr").filter({has:page.getByText(codes[index],{exact:true})}).locator(".available-value")).toHaveText(String(qty));
      }
      await page.goto(editURL);
    }, page);
    await step(unit, "upload a cover, set Active and save", "the product is active and persists after a reload", async () => {
      const png=Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB"+"CAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==","base64");
      await page.getByTestId("photo-input").setInputFiles({name:"sweep-cover.png",mimeType:"image/png",buffer:png});
      await expect(page.getByTestId("photo-row")).toHaveCount(1);
      await page.getByTestId("product-status").selectOption("active");
      await page.getByTestId("product-save").click();
      await expect(page.getByTestId("product-save")).toBeEnabled();
      await page.reload();
      await expect(page.getByTestId("product-status")).toHaveValue("active");
      journeys.product_title = title;
    }, page);
    await step(unit, "the merchant list shows the product (search + click)", "the row is listed with status active", async () => {
      await page.goto(u("/products"));
      await page.getByTestId("products-search").fill(tag);
      await page.getByTestId("products-search-submit").click();
      await expect(page.locator('[data-testid^="product-row-"]').filter({ hasText: title }).first()).toBeVisible();
    }, page);
    // the storefront: a buyer finds it by clicking
    const sc = await browser.newContext(ctxOptions(VARIANTS[0]));
    const sp = await sc.newPage();
    await step({ ...unit, route: "J1 storefront" }, "the buyer opens All products and clicks the new product", "its page opens with the title and an enabled Add to cart", async () => {
      await sp.goto(`${origin}/zh-TW/products`);
      await sp.getByTestId("product-card").filter({ hasText: title }).first().click();
      await expect(sp.getByRole("heading", { level: 1 })).toHaveText(title);
      await expect(sp.getByTestId("add-to-cart")).toBeEnabled();
    }, sp);
    await sc.close();
  } finally { await c.close().catch(() => {}); }
}

async function journeyAdminOrder(state, placed, journeys) {
  const unit = { app: "journey", journey: "J3", route: "J2/J3 admin orders", viewport: "desktop", locale: "zh-TW" };
  const order = placed.get("desktop/zh-TW");
  if (!order) { ledger.add(rowBase(unit, "journey", "(no order)", { kind: "journey", result: "fail", failure: "journey-step", actual: "the storefront journey did not place the COD order" })); return; }
  journeys.cod_order = order.id;
  const c = await browser.newContext(ctxOptions(VARIANTS[0], { storageState: state }));
  const page = await c.newPage();
  try {
    let detail;
    await step({ ...unit, journey: "J2" }, "admin Orders: the storefront COD order is listed (click through pages)", "the row of the order id can be expanded", async () => {
      await page.goto(`${adminOrigin}/zh-TW/orders?store=${store}`);
      await expect(page.getByTestId("orders-table")).toBeVisible();
      for (let p = 0; p < 6; p++) {
        const button = page.getByTestId(`order-expand-${order.id}`);
        if (await button.count()) { await button.click(); detail = page.getByTestId("order-detail"); await expect(detail).toHaveAttribute("aria-label", new RegExp(order.id)); return; }
        await expect(page.getByTestId("orders-next")).toBeEnabled(); await page.getByTestId("orders-next").click(); await expect(page.getByTestId("orders-table")).toBeVisible();
      }
      throw new Error("the order is not on the first 6 pages of the order list");
    }, page);
    await step(unit, "record the manual shipment (carrier Black Cat + tracking) and click Record", "the shipment record is shown", async () => {
      const shipment = detail.getByTestId("order-shipment");
      await shipment.getByTestId("ship-carrier").selectOption("black_cat");
      await shipment.getByTestId("ship-tracking").fill("BC1234567890");
      await shipment.getByTestId("shipment-submit").click();
      await expect(shipment.getByTestId("shipment-record")).toBeVisible();
    }, page);
    await step(unit, "click Collected, confirm in the dialog", "the order shows COLLECTED and the button is gone", async () => {
      const cod = detail.getByTestId("order-cod");
      await cod.getByTestId("cod-collected").click();
      const dialog = page.getByTestId("cod-dialog");
      await expect(dialog).toBeVisible();
      await dialog.getByTestId("cod-submit").click();
      await expect(cod.getByTestId("cod-collection-state")).toHaveAttribute("data-state", "COLLECTED");
      await expect(cod.getByTestId("cod-collected")).toHaveCount(0);
    }, page);
    await step(unit, "reload the order list: collected persists", "COLLECTED is still shown after a reload", async () => {
      await page.reload();
      await expect(page.getByTestId("orders-table")).toBeVisible();
      await page.waitForLoadState("networkidle"); // a click before hydration is lost: the user waits for the page to be live as well
      for (let p = 0; p < 6; p++) {
        const button = page.getByTestId(`order-expand-${order.id}`);
        if (await button.count()) { if ((await button.getAttribute("aria-expanded")) !== "true") await button.click(); /* the list keeps the opened order in its URL: only open it when it is closed */ await expect(page.getByTestId("order-detail")).toBeVisible(); await expect(page.getByTestId("order-detail").getByTestId("cod-collection-state")).toHaveAttribute("data-state", "COLLECTED"); return; }
        await page.getByTestId("orders-next").click(); await expect(page.getByTestId("orders-table")).toBeVisible();
      }
      throw new Error("the order vanished after a reload");
    }, page);
    // the buyer's lookup: a fresh browser finds the order by clicking
    const lc = await browser.newContext(ctxOptions(VARIANTS[0]));
    const lp = await lc.newPage();
    await step({ ...unit, journey: "J2b", route: "J2 storefront order lookup" }, "a fresh buyer browser looks the order up (order id + phone) and sees it collected", "the lookup shows the order with the collected amount", async () => {
      await lp.goto(`${origin}/zh-TW/orders/lookup`);
      await lp.getByTestId("lookup-ref").fill(order.id);
      await lp.getByTestId("lookup-contact").fill(pii.phone);
      await lp.getByTestId("lookup-submit").click();
      await expect(lp.getByTestId("order-cod")).toBeVisible({ timeout: 30000 });
      await expect(lp.getByTestId("order-cod-state")).toHaveAttribute("data-state", "COLLECTED");
    }, lp);
    await lc.close();
  } finally { await c.close().catch(() => {}); }
}

async function journeyDomain(state, journeys) {
  const unit = { app: "journey", journey: "J4", route: "J4 custom domain", viewport: "desktop", locale: "zh-TW" };
  const host = `shop-${facts.journey_tag}.example.net`;
  const c = await browser.newContext(ctxOptions(VARIANTS[0], { storageState: state }));
  const page = await c.newPage();
  try {
    let card;
    await step(unit, "open Settings and find the custom domain card", "the storefront domains card is shown", async () => {
      await page.goto(`${adminOrigin}/zh-TW/settings?store=${store}`);
      card = page.getByTestId("storefront-domains-card");
      await expect(card).toBeVisible();
    }, page);
    await step(unit, "type a custom hostname and click Request", "DNS instructions appear: a TXT name, a TXT value and the CNAME target", async () => {
      await card.getByTestId("storefront-domain-request").getByRole("textbox").fill(host);
      await card.getByTestId("storefront-domain-submit").click();
      const dns = card.getByTestId("storefront-dns");
      await expect(dns).toBeVisible();
      await expect(card.getByTestId("storefront-dns-txt-name")).toHaveText(`_lc-verify.${host}`);
      assert.match((await card.getByTestId("storefront-dns-txt-value").innerText()).trim(), /^[A-Za-z0-9_-]{43}$/);
      await expect(card.getByTestId("storefront-dns-cname")).toContainText("stores.");
      journeys.domain = host;
    }, page);
    await step(unit, "reload Settings: the requested domain is listed", "the domain row persists in state REQUESTED", async () => {
      await page.reload();
      const row = page.getByTestId("storefront-domains-card").getByTestId("storefront-domain-row").filter({ hasText: host });
      await expect(row).toBeVisible(); await expect(row).toHaveAttribute("data-state", "REQUESTED");
    }, page);
  } finally { await c.close().catch(() => {}); }
}

async function journeySignOut(state) {
  const unit = { app: "journey", journey: "J5", route: "J5 sign out", viewport: "desktop", locale: "zh-TW" };
  const c = await browser.newContext(ctxOptions(VARIANTS[0], { storageState: state }));
  const page = await c.newPage();
  try {
    await step(unit, "open the dashboard, click Sign out", "the session ends: the page asks to sign in again", async () => {
      await page.goto(`${adminOrigin}/zh-TW/?store=${store}`);
      await expect(page.getByTestId("nav-orders")).toBeVisible();
      await page.locator("header[data-shell-topbar] summary", { hasText: /^(帳號|Account)$/ }).click(); // the sign-out button lives inside the closed Account disclosure
      // MARKER ONLY: the shell strips ?store= (so the URL wait below can pass) before logout's window.location.replace(/zh-TW/) runs; the next
      // goto then aborts that navigation (ERR_ABORTED on slow runners). A flag on this document disappears when the new document replaces it.
      await page.evaluate(() => { window.__preSignOut = true; });
      await page.getByTestId("workspace-sign-out").click();
      await expect(page.getByTestId("nav-orders")).toHaveCount(0, { timeout: 15000 });
      await page.waitForURL(url => /^\/zh-TW\/?$/.test(url.pathname) && !url.search, {timeout:15000});
      await page.waitForFunction(() => !window.__preSignOut, null, { timeout: 15000 });
      await page.waitForLoadState("domcontentloaded");
    }, page);
    await step(unit, "reload after sign out", "the dashboard is not shown without a session", async () => {
      await page.goto(`${adminOrigin}/zh-TW/orders?store=${store}`);
      await expect(page.getByTestId("orders-table")).toHaveCount(0);
    }, page);
  } finally { await c.close().catch(() => {}); }
}

// ---- main ---------------------------------------------------------------------------------------------------------------------------------------------------
async function pool(tasks, n) {
  const queue = [...tasks];
  const workers = Array.from({ length: n }, async () => { for (let t = queue.shift(); t; t = queue.shift()) await t(); });
  await Promise.all(workers);
}
let exitCode = 0;
try {
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path.join(certDir, "key.pem"), "-out", path.join(certDir, "cert.pem"), "-days", "1", "-subj", `/CN=${HOST}`], { stdio: "ignore" });
  const nextPort = await startNext();
  edge = https.createServer({ key: await readFile(path.join(certDir, "key.pem")), cert: await readFile(path.join(certDir, "cert.pem")) }, async (req, res) => {
    try {
      const chunks = []; for await (const x of req) chunks.push(x);
      const out = await relay(nextPort, req, Buffer.concat(chunks));
      const headers = { ...out.headers }; delete headers["transfer-encoding"]; delete headers.connection;
      res.writeHead(out.status, headers); res.end(out.body);
    } catch { if (!res.headersSent) res.writeHead(502); res.end(); }
  });
  const edgePort = await listen(edge);
  proxy = http.createServer((_, res) => { res.writeHead(403); res.end(); });
  proxy.on("connect", (req, socket, head) => {
    if (req.url !== `${HOST}:443`) { socket.destroy(); return; }
    const upstream = net.connect(edgePort, "127.0.0.1", () => { socket.write("HTTP/1.1 200 Connection Established\r\n\r\n"); if (head.length) upstream.write(head); socket.pipe(upstream).pipe(socket); });
    for (const s of [socket, upstream]) { sockets.add(s); s.on("close", () => sockets.delete(s)); s.on("error", () => { socket.destroy(); upstream.destroy(); }); }
  });
  browser = await launch({ headless: true, proxy: { server: `http://127.0.0.1:${await listen(proxy)}`, bypass: "127.0.0.1,localhost" } }); // loopback (the admin, the control listener) goes direct; only the virtual storefront host rides the CONNECT proxy
  const state = await loginState().catch(async (e) => {
    const c = browser.contexts()[0]; const p = c?.pages()[0];
    if (p) { await writeFile(path.join(evidence, "login-failure.txt"), `${p.url()}\n${(await p.locator("body").innerText().catch(() => "")).slice(0, 2000)}`); await p.screenshot({ path: path.join(evidence, "login-failure.png") }).catch(() => {}); }
    throw e;
  });
  log(`stack up; admin ${adminOrigin}, storefront ${origin} (Next :${nextPort}); ${adminRoutes.length} registry routes; workers=${WORKERS} sample=${SAMPLE}`);

  const placed = new Map(), journeys = {};
  if (run("admin")) {
    const units = [];
    for (const route of adminRoutes) {
      if (!wanted(route.path)) continue;
      // one task per route keeps the three variants of a route sequential (a state-changing click never races its own sibling variant)
      units.push(async () => { for (const [i, v] of VARIANTS.entries()) { await adminUnit(state, route, v, route.path === "/" && !route.public); log(`admin ${route.path} ${v.viewport}/${v.locale} done`); } });
    }
    await pool(units, WORKERS);
  }
  if (run("storefront")) await pool(VARIANTS.map((v) => () => storefrontSession(v, placed)), WORKERS);
  if (run("journeys")) {
    await journeyProduct(state, journeys);
    await journeyAdminOrder(state, placed, journeys);
    await journeyDomain(state, journeys);
    await journeySignOut(state);
  }
  await writeFile(path.join(outDir, "journeys.json"), JSON.stringify(journeys, null, 2));

  // ---- coverage: every registry route and every buyer route x every variant must have been opened (a silent skip is a failure) ------------------------------
  if (only.size === 0 && pageFilter.length === 0) {
    const storefrontRoutes = ["/", "/products", "/collections", "/collections/[slug]", "/products/[slug]", "/search", "/cart", "/checkout", "/orders/lookup", "/orders/[orderID]", "/order-link", "/claim", "/legal/anti-fraud", "/legal/[slug]", "/privacy", "/data-deletion", "/pages/[slug]"];
    const opened = new Set(ledger.rows.filter((r) => r.control === "(page load)").map((r) => `${r.app}|${r.page}|${r.viewport}|${r.locale}`));
    const expected = [...adminRoutes.map((r) => ["admin", r.path]), ...storefrontRoutes.map((r) => ["storefront", r])].flatMap(([app, route]) => VARIANTS.map((v) => `${app}|${route}|${v.viewport}|${v.locale}`));
    const missing = expected.filter((e) => !opened.has(e));
    for (const m of missing) { const [app, page, viewport, locale] = m.split("|"); ledger.add(rowBase({ app, route: page, viewport, locale }, "page", "(coverage)", { result: "fail", failure: "coverage-missing", expected: "the route is opened at every viewport/locale", actual: "no page-load row was written for this route/variant" })); }
    // /live (S6, a buyer live-room page) has no page in this base: stated, not hidden
    ledger.add(rowBase({ app: "storefront", route: "/live", viewport: "-", locale: "-" }, "page", "(not implemented)", { result: "skip", expected: "ui-architecture section 4 lists /live (S6)", actual: "apps/storefront/app/[locale] has no live route in this base; nothing to click" }));
  }

  // ---- verdict ------------------------------------------------------------------------------------------------------------------------------------------------
  const counts = ledger.counts();
  const failures = ledger.failures();
  const { classified, stale } = matchKnown(failures, known);
  const summary = {
    generated: new Date().toISOString(), engine: "chromium",
    line: `${counts.pages} page/viewport/locale units opened (${counts.loads.fail} with a page-load failure), ${counts.controls.total} control clicks (${counts.controls.pass} pass, ${counts.controls.fail} fail, ${counts.controls.skip} skip), ${counts.journeys.total} journey steps (${counts.journeys.pass} pass, ${counts.journeys.fail} fail); failures: known ${classified.filter((c) => c.known).length}, new ${classified.filter((c) => !c.known).length}; stale known-defect entries ${stale.length}.`,
    counts, failures: classified.map((c) => ({ id: c.row.id, app: c.row.app, page: c.row.page, viewport: c.row.viewport, locale: c.row.locale, control: c.row.control, failure: c.row.failure, actual: c.row.actual, known: c.known, screenshot: c.row.screenshot })),
    stale: stale.map((s) => s.id),
  };
  await writeLedger(outDir, ledger.rows, summary);
  log(summary.line);
  for (const c of classified) console.log(`${c.known ? "KNOWN " + c.known : "NEW"} FAIL ${c.row.app} ${c.row.page} ${c.row.viewport}/${c.row.locale} [${c.row.scope}] "${c.row.control}" ${c.row.failure}: ${c.row.actual}${c.row.screenshot ? ` (${c.row.screenshot})` : ""}`);
  for (const s of stale) console.log(`STALE known-defect ${s.id} (${s.route} ${s.kind}): it no longer reproduces; remove the entry`);
  if (failures.length || stale.length) exitCode = 1;
  else console.log("PASS G-UI8 click sweep");
} catch (e) {
  console.error(e);
  exitCode = 1;
} finally {
  await browser?.close().catch(() => {});
  for (const s of sockets) s.destroy();
  proxy?.close(); edge?.close();
  for (const c of children) c.kill("SIGKILL");
  for (const l of logs) l.end();
  await rm(certDir, { recursive: true, force: true });
}
process.exit(exitCode);
