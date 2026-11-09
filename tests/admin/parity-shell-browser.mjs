// Purpose: Produce real-route admin parity sheets and assert owner-chosen shell A with real shell-control journeys.
// Depends on: Playwright Chromium, node:test, shell-fixture MOCK transport (seeded with synthetic, contract-shaped Go
//   responses below), parity-team and the packaged admin via next-startup.
// Used by: scripts/dev/test-local.sh --browser-admin-parity; artifacts are inputs to independent visual review.
// Invariants: no gallery, pixel baseline, DOM mutation, forced click, provider traffic or business-flow acceptance;
//   seeds are synthetic MOCK data for the first store only (the second store keeps the empty/unavailable fallbacks)
//   and each shape follows the admin strict parser named beside it; no real names, phones or emails.
import assert from "node:assert/strict";
import { before, after, test } from "node:test";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { createWriteStream } from "node:fs";
import { mkdir, writeFile, copyFile } from "node:fs/promises";
import { chromium, expect } from "@playwright/test";
import { fixture, storeID, otherID, entityID } from "./shell-fixture.mjs";
import { browserEvidenceDirectory } from "../browser-evidence.mjs";
import { nextAttemptLog, startNextWithPortRetry } from "../helpers/next-startup.mjs";
import { registerTeamParity } from "./parity-team.mjs";

// matrix-receipts counts this node suite as the mode's browser execution only with this mark (BROWSER_SPEC_MARK).
console.log("## matrix-receipts browser-spec");

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
  { name: "members", path: "/members", group: "customers", index: 5, item: "members" },
  { name: "settings", path: "/settings", group: "settings", index: 9, item: "settings" },
  { name: "storefront-editor", path: "/design", group: "storefront", index: 7 },
  { name: "live-console", path: "/studio/console", group: "live", index: 1, item: "live-console" },
];
// FIXTURE only: synthetic Go /v1/admin responses so parity sheets show populated frames instead of empty/503 states.
// Shapes follow the admin strict parsers (cited per entry); amounts are TWD minor units. Read-only GETs; no writes seeded.
const at = "2026-10-30T02:00:00Z", day = "2026-10-30";
const ids = { o1: "a0000000-0000-4000-8000-000000000001", o2: "a0000000-0000-4000-8000-000000000002", o3: "a0000000-0000-4000-8000-000000000003",
  p1: "b0000000-0000-4000-8000-000000000001", p2: "b0000000-0000-4000-8000-000000000002", p3: "b0000000-0000-4000-8000-000000000003",
  c1: "c0000000-0000-4000-8000-000000000001", c2: "c0000000-0000-4000-8000-000000000002", c3: "c0000000-0000-4000-8000-000000000003",
  m1: "d0000000-0000-4000-8000-000000000001", m2: "d0000000-0000-4000-8000-000000000002",
  session: "e0000000-0000-4000-8000-000000000001", program: "e0000000-0000-4000-8000-000000000002",
  k1: "f0000000-0000-4000-8000-000000000001", k2: "f0000000-0000-4000-8000-000000000002",
  f1: "f1000000-0000-4000-8000-000000000001", f2: "f1000000-0000-4000-8000-000000000002" };
const sessionTitle = "週五晚間直播 · 合成場次";
// lib/orders-model.ts parseOrderSummary (+source): card CONFIRMED/CAPTURED/READY, bank transfer awaiting, card shipped.
const order = (id, created, total, commercial, fulfillment, payment, work, mode) => ({ order_id: id, created_at: created, updated_at: created,
  currency: "TWD", total_minor: total, commercial_state: commercial, fulfillment_state: fulfillment, payment_state: payment, test_mode: false,
  work_state: work, refunded_minor: 0, refund_pending_minor: 0, pickup_source: null, payment_mode: mode, collection_state: null,
  cod_surcharge_minor: null, cod_collect_minor: null, source: "storefront" });
const orders = [
  order(ids.o1, "2026-10-30T01:40:00.000000Z", 129000, "CONFIRMED", "MANUAL_UNASSIGNED", "CAPTURED", "READY", "card"),
  order(ids.o2, "2026-10-30T01:20:00.000000Z", 45000, "AWAITING_TRANSFER", "MANUAL_UNASSIGNED", "NOT_STARTED", "NONE", "bank_transfer"),
  order(ids.o3, "2026-10-30T00:50:00.000000Z", 59000, "CONFIRMED", "MERCHANT_SHIPPED", "CAPTURED", "READY", "card"),
];
// lib/orders-v2.ts parseOrderListV2: order_number = LC-<hex id>, masked recipient, delivery kind, sessions, item summary.
const v2 = (row, recipient, delivery, items) => ({ ...row, order_number: `LC-${row.order_id.replaceAll("-", "").toUpperCase()}`,
  recipient_masked: recipient, delivery_kind: delivery, live_sessions: [{ id: ids.session, name: sessionTitle }],
  item_summary: { items, total_item_count: items.reduce((sum, item) => sum + item.quantity, 0) } });
const counts = { all: 3, unpaid: 0, transfer_review: 1, ready_to_ship: 1, ready_to_consign: 0, shipped: 1, completed: 0, cancelled: 0 };
// internal/catalog/productlist.go ProductSummary / lib/catalog-v2-model.ts parseSummary.
const product = (id, slug, name, status, min, max, skus, available, keyword) => ({ id, slug, name, status, version: 1, cover_image_id: null,
  price_min_minor: min, price_max_minor: max, currency: "TWD", sku_count: skus, available, keyword, inventory_tracked: true, updated_at: "2026-10-29T08:00:00.000000Z" });
const products = [
  product(ids.p1, "synthetic-cotton-tee", "測試商品 A 棉質上衣", "active", 59000, 69000, 3, 42, "101"),
  product(ids.p2, "synthetic-linen-pants", "測試商品 B 亞麻長褲", "active", 129000, 129000, 2, 5, "102"),
  product(ids.p3, "synthetic-wool-scarf", "測試商品 C 羊毛圍巾", "archived", 45000, 45000, 1, 0, "103"),
];
// lib/customers-model.ts customerKeys / customerFrom.
const customer = (id, name, last3, count, paid, captured, platforms) => ({ customer_id: id, first_seen_at: "2026-09-12T03:00:00Z",
  last_activity_at: at, display_name: name, phone_last3: last3, orders_count: count, paid_orders_count: paid, captured_minor: captured,
  refunded_minor: 0, currency: "TWD", claims_count: count, platforms, consents: { marketing_messages: false, ads_personalization: false },
  active: true, imported: false, tags: [] });
// internal/inbox/service.go ConversationItem (contract live-console-v1 A8).
const conversation = (id, platform, name, unread) => ({ conversation_id: id, platform, display_name: name, last_at: at, unread, unreplied: unread,
  mode: "human", assignee: null, window_open_until: "2026-10-31T02:00:00Z", linked_customer_id: null, link_version: null });
const stream = { state: "unavailable", poll_interval_ms: 5000, last_ok_at: null, lag_ms: null, source_platform: "facebook", video_embeddable: false };
// lib/studio-model.ts parseDraft (internal/live/draft.go Draft).
const draft = { session_id: ids.session, program_id: ids.program, title: sessionTitle, scheduled_at: null, aspect_ratio: "9:16", state: "LIVE",
  version: 1, created_at: "2026-10-30T00:00:00Z", updated_at: "2026-10-30T00:00:00Z" };
// src/features/live/console-model.ts parseConsoleOffer and lib/claims-model.ts parseOffer share these synthetic offers.
const offers = [[ids.f1, "101", ids.k1, "測試商品 A 棉質上衣", "白色 M", 59000, 18, 6], [ids.f2, "102", ids.k2, "測試商品 B 亞麻長褲", "卡其 L", 129000, 3, 2]];
const s = (path) => `/v1/admin/stores/${storeID}/${path}`, live = (path) => s(`live-sessions/${ids.session}/${path}`);
const seeded = {
  // lib/merchant-tools-model.ts parseDashboard (contract storefront-v2 G1).
  [s("dashboard")]: { generated_at: at, timezone: "Asia/Taipei", orders: { today: 3, last_7_days: 11 },
    gmv: [{ currency: "TWD", environment: "LIVE", today: { card_minor: 188000, bank_transfer_minor: 0, pay_at_pickup_minor: 0, cod_minor: 0 },
      last_7_days: { card_minor: 612000, bank_transfer_minor: 90000, pay_at_pickup_minor: 0, cod_minor: 0 } }],
    todos: { awaiting_transfer_confirmation: 1, to_ship: 1, cvs_awaiting_label: 0, low_stock_skus: 0, open_refunds: 0 }, latest_orders: orders },
  // lib/customers-model.ts parseFinanceSummary / parseFinanceRow (7-key row; net = captured - refunded).
  [s("finance/summary")]: { generated_at: at, from: day, to: day, timezone: "Asia/Taipei",
    rows: [{ day, currency: "TWD", environment: "LIVE", captured_count: 2, captured_minor: 188000, refunded_minor: 0, net_minor: 188000 }],
    totals: [{ day: "", currency: "TWD", environment: "LIVE", captured_count: 2, captured_minor: 188000, refunded_minor: 0, net_minor: 188000 }] },
  [s("orders")]: { items: [
    v2(orders[0], "測***", "home", [{ product_name: "測試商品 A 棉質上衣", variant_label: "白色 M", quantity: 2 }]),
    v2(orders[1], "王***", "cvs_711", [{ product_name: "測試商品 C 羊毛圍巾", variant_label: null, quantity: 1 }]),
    v2(orders[2], "林***", "home", [{ product_name: "測試商品 B 亞麻長褲", variant_label: "卡其 L", quantity: 1 }]),
  ], next_cursor: "", total: 3, counts, sessions: [{ id: ids.session, name: sessionTitle }] },
  // lib/orders-model.ts parseOrderActions; false matches the fixture's read-only permission list.
  [s("order-actions")]: { refund: false, fulfillment_write: false, orders_export: false },
  // lib/parcels-model.ts parseMergeSuggestions / parseOpenParcelGroups: no merge candidates, no open groups.
  [s("orders/merge-suggestions")]: { items: [] },
  [s("parcel-groups")]: { items: [] },
  // Status filter honoured so the settings draft-product discovery (status=draft) truthfully finds none.
  [s("catalog-products")]: (url) => {
    const status = url.searchParams.get("status"), items = products.filter((p) => !status || p.status === status);
    return { items, next_cursor: "", total: items.length, status_counts: { draft: 0, active: 2, archived: 1 } };
  },
  [s("customers")]: { items: [
    customer(ids.c1, "測試買家 A", "001", 3, 3, 247000, ["facebook"]),
    customer(ids.c2, "測試買家 B", "002", 1, 0, 0, ["instagram"]),
    customer(ids.c3, "測試買家 C", null, 2, 1, 59000, ["facebook", "manual"]),
  ], next_cursor: "" },
  // lib/customer-tags-model.ts parseTagCatalog.
  [s("customers/tags")]: { items: [] },
  // A8 list for the inbox; live-comment and per-session reads (live console) truthfully get no conversations.
  [s("inbox/conversations")]: (url) => ["live_comment", "instagram"].includes(url.searchParams.get("filter")) || url.searchParams.has("session_id")
    ? { items: [], next_cursor: "", unread_total: 0 }
    : { items: [conversation(ids.m1, "messenger", "測試買家 A", true), conversation(ids.m2, "messenger", "測試買家 B", false)], next_cursor: "", unread_total: 1 },
  // internal/httpapi/profile.go storeProfile; internal/quickstart/quickstart.go State (lib/live-settings-model.ts settingsData).
  [s("profile")]: { name: "W0 合成測試店 · Synthetic baseline", contact_email: "owner@example.test", contact_phone: "", version: 1 },
  [s("quickstart")]: { completed: false, completed_at: null, steps: { store_name: true, shipping: false, first_product: true } },
  // No market yet: lib/settings-v2.ts readSnapshot then offers market creation instead of guessing delivery services.
  [s("markets")]: { items: [], next_cursor: "" },
  // lib/storefront-model.ts parseStorefront: unpublished, no domains.
  [s("storefront")]: { published: false, version: 0, domains: [] },
  // lib/design-model.ts parseDraft/parseVersions/parseMediaList (contract storefront-v2 section B document).
  [s("design/draft")]: { version: 1, updated_at: at, published_version: null, document: {
    profile: { name: "W0 合成測試店", tagline: "每週五晚上八點直播", logo_image_id: null, favicon_image_id: null, accent_color: "#247965", announcement: null,
      contact: { email: "owner@example.test", phone: null, address: null, line_url: null, facebook_url: null, instagram_url: null } },
    nav: { header: [{ label: "所有商品", kind: "all_products", target: null }], footer: [] },
    home: { sections: [
      { type: "hero", image_id: null, heading: "本週直播精選", subheading: null, cta_label: "逛逛商品", cta_kind: "all_products", cta_target: null },
      { type: "product_grid", heading: "新品上架", sort: "newest", limit: 12 },
    ] }, pages: [] } },
  [s("design/versions")]: { items: [], live_version: null },
  [s("design/media")]: { items: [] },
  // lib/studio-model.ts parseStudioPage / parseStudioDetail.
  [s("live-sessions")]: { items: [draft], next_cursor: "" },
  [s(`live-sessions/${ids.session}`)]: { draft, prepared: null, attempt: null, can_manage: false, media_enabled: false },
  // src/features/live/console-model.ts parseConsole (A1): no platform binding, so comments and capabilities stay unavailable.
  [live("console")]: {
    session: { id: ids.session, title: sessionTitle, lifecycle: "live", version: 1, started_at: "2026-10-30T00:00:00Z", ended_at: null },
    window: { state: "OPEN", generation: 1, opened_at: "2026-10-30T00:00:00Z", match_mode: "KEYWORD_QTY_ONLY" },
    stats: { comments: { total: null, source: "unavailable" }, keyword_comments: 0, buyers: 2, orders: { count: 2, amount_minor: 247000 },
      paid: { count: 1, amount_minor: 129000 }, currency: "TWD", as_of: at },
    offers: offers.map(([offer_id, keyword, sku_id, product_name, variant_label, price, sellable, reserved]) => ({ offer_id, keyword, sku_id, product_name,
      variant_label, active: true, version: 1, live_price_minor: null, sku_price_minor: price,
      stock: { tracked: true, sellable, reserved, warehouse_id: entityID, balance_version: 1 }, claimed: { buyers: 1, quantity: reserved },
      ordered_qty: reserved, paid_qty: 0, paid_amount_minor: 0, sold_out: false, low_stock: sellable < 5 })),
    capabilities: {}, stream, recommended: null },
  // lib/claims-model.ts parseBoard (M1): same window and offers as A1.
  [live("claims")]: {
    window: { session_id: ids.session, state: "OPEN", match_mode: "KEYWORD_QTY_ONLY", generation: 1, version: 1, opened_at: "2026-10-30T00:00:00Z", closed_at: null },
    offers: offers.map(([offer_id, keyword, sku_id, product_name, , price]) => ({ offer_id, session_id: ids.session, keyword, sku_id,
      sku_code: `SYN-${keyword}`, product_name, max_quantity_per_claim: 5, active: true, version: 1, activated_at: "2026-10-30T00:00:00Z",
      updated_at: "2026-10-30T00:00:00Z", sku_price_minor: price, currency: "TWD", live_price_minor: null })),
    stats: { generation: 1, accepted: 2, rejected: { NO_MATCH: 0, UNKNOWN_KEYWORD: 0, OFFER_INACTIVE: 0, INVALID_QUANTITY: 0, QUANTITY_REQUIRED: 0, QUANTITY_OVER_MAX: 0, BUNDLE_LIMIT: 0 } } },
  // lib/claim-source-model.ts parseClaimSourceEnvelope: no comment source bound.
  [live("claim-source")]: { source: null, platforms: [] },
  // src/features/live/comment-model.ts: empty first page.
  [live("comments")]: { epoch: 1, reset: false, scan_exhausted: true, items: [], next: { epoch: 1, seq: 0 }, older_cursor: null, stream },
};
const viewports = [{ width: 1440, height: 900 }, { width: 390, height: 844 }];
const output = browserEvidenceDirectory("admin-parity");
const ledger = [], captures = [];
const sha = execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim();
let f, browser, child, log, base;

registerTeamParity({ session, origin: () => base, store: storeID, output, sha });

before(async () => {
  await mkdir(`${output}/parity`, { recursive: true });
  f = await fixture({ seeded });
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
  await writeFile(`${output}/manifest.json`, JSON.stringify({ tier: "MOCK", sha, viewports, captures, note: "Real admin routes; synthetic store and seeded synthetic contract-shaped domain reads (others empty or explicitly unavailable). Shell acceptance only; screenshots require independent comparison, no pixel diff." }, null, 2) + "\n");
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
  if (route === "/members") await expect(page.getByTestId("members-list").getByRole("link", { name: "Synthetic member", exact: true })).toBeVisible();
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
  return page.locator("[data-shell-rail]").getByTestId(route.group === "orders" ? "nav-orders" : `nav-group-${route.group}`);
}

const navGroups = ["overview", "live", "orders", "catalog", "messages", "customers", "marketing", "storefront", "finance", "settings"];

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
      for (const [index, name] of c.nav.entries()) {
        const control = group(page, { group: navGroups[index] }, locale);
        await expect(control).toBeVisible();
        await expect(control).toHaveAccessibleName(name);
      }
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
      for (const [index, name] of c.nav.entries()) {
        const control = group(page, { group: navGroups[index] }, locale);
        await expect(control).toHaveAccessibleName(name);
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
