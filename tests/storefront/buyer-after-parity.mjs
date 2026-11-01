// Purpose: PARITY-BUYER-AFTER real-route browser spec: buyer order status, order lookup, order-link, payment return, privacy / data deletion and the live claim link
//   asserted against the signed Figma copy (output/parity-buyer-after/FRAME-COPY.md), with real clicks, plus one parity screenshot per covered frame.
// Depends on: node:test, Playwright (tests/storefront/browser-engine.mjs), the production storefront Next build, tests/storefront/shop-fake-api.mjs (design / catalog chrome only)
//   and a contract-shaped MOCK of the buyer order / privacy / claim routes defined below; tests/helpers/next-startup.mjs, tests/browser-evidence.mjs.
// Used by: scripts/dev/test-local.sh --browser-buyer-after-parity; the screenshots (parity/<frame-id>.png, ":" written as "-") are inputs to the independent side-by-side review.
// Invariants: MOCK only, no production; signed visible text only (never layer names); prototype-only controls and demo notices (原型示範, 預填示範, 示範回應) are never asserted;
//   native window.confirm text is browser supplied, so only "a confirm was shown" is asserted; every interaction is a real Playwright click/fill (no force, no dispatchEvent, no DOM edit).
import assert from "node:assert/strict";
import { before, after, test } from "node:test";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import { createWriteStream } from "node:fs";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { randomBytes } from "node:crypto";
import { launch, ctxOpts } from "./browser-engine.mjs";
import { createFakeApi } from "./shop-fake-api.mjs";
import { browserEvidenceDirectory } from "../browser-evidence.mjs";
import { nextAttemptLog, startNextWithPortRetry } from "../helpers/next-startup.mjs";

// matrix-receipts counts this node suite as the mode's browser execution only with this mark (BROWSER_SPEC_MARK).
console.log("## matrix-receipts browser-spec");

const root = process.cwd();
const HOST = "shop.example", origin = `https://${HOST}`;
const out = browserEvidenceDirectory("buyer-after-parity");
const uid = (n) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const NOW = Date.now(), iso = (ms) => new Date(NOW + ms).toISOString();
const key = () => randomBytes(32).toString("base64url");
const sha = execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim();
const ledger = [], captures = [], deferred = [];
const children = new Set(), sockets = new Set();
let browser, edge, proxy, next, api, apiPort, logFile, desktop, phone;

// ---- MOCK buyer API: order / privacy / claim routes layered over the shared design + catalog fake -----------------------------------------------------------
const bearers = new Set();
const orders = new Map();         // order id -> { order, pay }
const refOf = (n) => `0123-ABCD-${n.toString(16).toUpperCase().padStart(4, "0")}`;
const privacyState = { marketing: false, ads: false, erased: false, exportMode: "ok", eraseMode: "ok" };
const CLAIM_TOKEN = "A".repeat(43);
const claimState = { redeem: "ok" };
const SKU = uid(900), OTHER_SKU = uid(901);
const NAME = { tote: "焦糖托特包", bag: "收納袋" };

function makeOrder(n, o = {}) {
  const mode = o.mode ?? "card", kind = o.kind ?? "home", line = o.line ?? "tote";
  const unit = o.unit ?? 128000, total = unit;
  const commercial = o.commercial ?? "DRAFT";
  const cvs = kind === "cvs";
  const order = {
    order_id: uid(n), cart_id: uid(800 + n), cart_version: 1,
    commercial_state: commercial,
    fulfillment_state: o.fulfillment ?? (commercial === "CANCELLED" ? "CANCELLED" : "MANUAL_UNASSIGNED"),
    ...(commercial === "DRAFT" ? { hold_expires_at: iso(3600e3) } : {}),
    ...(mode !== "card" ? { payment_mode: mode, collection_state: o.collection } : {}),
    ...(mode === "cash_on_delivery" ? { cod_collect_minor: total, cod_carrier: "black_cat" } : {}),
    ...(o.shipment ? { shipment: o.shipment } : {}),
    ...(o.cvs_state ? { cvs_shipment: { state: o.cvs_state, chain: "cvs_711", store_name: "示範門市", store_code: "000001", updated_at: iso(-3600e3) } } : {}),
    snapshot: {
      quote: {
        currency: "TWD",
        lines: [{ sku_id: SKU, name: NAME[line], code: line === "tote" ? "TOTE-CARAMEL" : "BAG-CREAM", quantity: 1, unit_price_minor: unit }],
        amount: { subtotal_minor: unit, discount_minor: 0, shipping_minor: 0, shipping_tax_minor: 0, tax_minor: 0, total_minor: total },
      },
      destination: cvs
        ? { kind: "cvs_711", country: "TW", recipient_name: "示範買家", phone: "0912345601", home_address: { region: "", city: "", postal_code: "", line1: "", line2: "" },
            pickup: { kind: "cvs_711", namespace: "ecpay", code: "000001", name: "示範門市", address: "台北市中正區示範路 20 號", country: "TW" } }
        : { kind: "home", country: "TW", recipient_name: "示範買家", phone: "0912345601", home_address: { region: "台北市", city: "中正區", postal_code: "", line1: "示範路 1 號", line2: "" } },
      service: { code: kind, name_hans: "商家自行安排配送", name_hant: "商家自行安排配送", name_en: "Merchant arranged delivery", delivery_kind: cvs ? "cvs_711" : "home", mode: "MANUAL" },
    },
  };
  let pay = null;
  if (mode === "card") {
    const p = o.pay ?? {};
    const stripe = p.handoff && p.handoff !== "NONE";
    pay = {
      order_id: order.order_id, currency: "TWD", total_minor: total, commercial_state: commercial === "AWAITING_COLLECTION" ? "AWAITING_PAYMENT" : commercial, test_mode: false,
      payment_state: p.state ?? "NOT_STARTED", handoff_state: p.handoff ?? "NONE", handoff_expires_at: stripe ? iso(1800e3) : null,
      ...(stripe ? { cancel_requested: Boolean(p.cancelRequested) } : {}),
      ...(p.refund ? { refund: p.refund } : {}),
      methods: p.methods === false ? [] : [{ code: "stripe_checkout", version: 1, name_hans: "信用卡", name_hant: "信用卡", name_en: "Card" }],
    };
  }
  orders.set(order.order_id, { order, pay, ref: refOf(n) });
  return order;
}

function error(res, status, code) {
  res.writeHead(status, { "Content-Type": "application/json", "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" });
  res.end(JSON.stringify({ code, message: code, request_id: "0".repeat(32), retryable: false, details: {} }));
}
function send(res, status, value) {
  res.writeHead(status, { "Content-Type": "application/json", "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" });
  res.end(JSON.stringify(value));
}
const OWN = /^\/v1\/buyer\/(session(\/bootstrap|\/retire)?|orders\/.+|privacy(\/export|\/erasure)?|consents|claim-link(\/redeem)?|cart)$/;
async function mine(req, res, pathname) {
  const chunks = []; for await (const c of req) chunks.push(c);
  let body = {}; try { body = JSON.parse(Buffer.concat(chunks).toString("utf8") || "{}"); } catch { /* empty body */ }
  if (req.headers["x-commerce-buyer-bff-key"] !== api.bffKey) return error(res, 403, "forbidden");
  const rest = pathname.slice("/v1/buyer/".length);
  const bearer = /^Bearer (.+)$/.exec(req.headers.authorization ?? "")?.[1];
  if (rest === "session/bootstrap" && req.method === "POST") { if (bearer) bearers.add(bearer); return send(res, 200, { authenticated: true, expires_at: iso(3600e3) }); }
  if (rest === "session/retire") { bearers.delete(bearer); res.writeHead(204); return res.end(); }
  if (rest === "orders/lookup" && req.method === "POST") {
    const hit = [...orders.values()].find((x) => x.ref === String(body.order_ref).trim().toUpperCase());
    if (!hit) return error(res, 404, "not_found");
    bearers.add(bearer);
    return send(res, 200, { order_id: hit.order.order_id });
  }
  if (rest === "orders/link") return error(res, 404, "not_found");
  if (!bearers.has(bearer)) return error(res, 401, "unauthorized");
  if (rest === "session") return send(res, 200, { authenticated: true });
  const m = /^orders\/([0-9a-f-]{36})(?:\/(payment)(?:\/(prepare|handoff|refresh|cancel))?)?$/.exec(rest);
  if (m) {
    const entry = orders.get(m[1]);
    if (!entry) return error(res, 404, "not_found");
    if (!m[2]) return send(res, 200, entry.order);
    if (!entry.pay) return error(res, 404, "not_found");
    if (!m[3]) return send(res, 200, entry.pay);
    if (m[3] === "prepare") {
      // Mirror the server: prepare moves a fresh attempt to a continuable one (PENDING/READY, not cancelled, AWAITING_PAYMENT); stripeHandoff re-reads it and aborts if it still looks fresh.
      // methods: [] because validOrderPayment rejects a non-fresh view that still lists methods (the server clears them on prepare).
      Object.assign(entry.pay, { payment_state: "PENDING", handoff_state: "READY", handoff_expires_at: iso(1800e3), cancel_requested: false, methods: [], commercial_state: "AWAITING_PAYMENT" });
      entry.order.commercial_state = "AWAITING_PAYMENT"; delete entry.order.hold_expires_at;
    }
    if (m[3] === "prepare") return send(res, 200, { order_id: m[1], state: "PAYMENT_PENDING", currency: "TWD", amount_minor: entry.pay.total_minor });
    if (m[3] === "handoff") {
      Object.assign(entry.pay, { payment_state: "PENDING", handoff_state: "READY", handoff_expires_at: iso(1800e3), cancel_requested: false, methods: [], commercial_state: "AWAITING_PAYMENT" });
      entry.order.commercial_state = "AWAITING_PAYMENT"; delete entry.order.hold_expires_at;
      return send(res, 200, { order_id: m[1], disposition: "REDIRECT", expires_at: iso(1800e3), redirect_url: "https://checkout.stripe.com/c/pay/cs_test_parity" });
    }
    if (m[3] === "cancel") entry.pay.cancel_requested = true;
    return send(res, 200, { order_id: m[1], scheduled: true });
  }
  if (rest === "privacy") return privacyState.erased ? error(res, 410, "erased") : send(res, 200, { consents: { marketing_messages: privacyState.marketing, ads_personalization: privacyState.ads }, erased: false });
  if (rest === "consents") {
    if (body.purpose === "marketing_messages") privacyState.marketing = body.granted === true; else privacyState.ads = body.granted === true;
    return send(res, 200, { purpose: body.purpose, channel: body.channel, granted: body.granted === true, occurred_at: iso(0) });
  }
  if (rest === "privacy/export") {
    if (privacyState.exportMode === "too_large") return error(res, 422, "export_too_large");
    return send(res, 200, { format: "lc.customer-export.v1", generated_at: iso(0), store: {}, orders: [], consents: [], claims: [], privacy_actions: [], tags: [], notes: [], notes_omitted: 0 });
  }
  if (rest === "privacy/erasure") {
    if (privacyState.eraseMode === "blocked") return error(res, 409, "erasure_blocked");
    if (privacyState.eraseMode === "unknown") return error(res, 503, "unavailable");
    privacyState.erased = true;
    return send(res, 200, { erased: true, orders_retained: true, summary: { consents_withdrawn: 0, sessions_revoked: 1, snapshots_redacted: 0, bundles_relabelled: 0 } });
  }
  if (rest === "cart") return send(res, 200, { id: uid(700), currency: "TWD", version: 1, items: [{ sku_id: OTHER_SKU, quantity: 1 }] });
  if (rest === "claim-link") {
    return send(res, 200, { bundle_version: 1, bound: true, expires_at: iso(3600e3), lines: [{ keyword: "A1", sku_id: SKU, sku_code: "TOTE-CARAMEL", product_name: "輕量帆布托特包", currency: "TWD", unit_price_minor: 128000, quantity: 1, pending: true, available: true, sold_out: false }] }); // pending: true = not yet in the cart (fixture fix: false + absent from the cart is the "nothing to check out" state, not the confirm frame)
  }
  if (rest === "claim-link/redeem") {
    if (claimState.redeem === "unknown") return error(res, 503, "unavailable");
    return send(res, 200, { bundle_version: 1, cart: { id: uid(700), currency: "TWD", version: 2, items: [{ sku_id: OTHER_SKU, quantity: 1 }, { sku_id: SKU, quantity: 1 }] }, applied: [{ sku_id: SKU, quantity: 1 }], skipped: [] });
  }
  return error(res, 404, "not_found");
}

// ---- process plumbing (same synthetic https edge as shop-gate: the BFF derives the store origin from Host) ---------------------------------------------------
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

before(async () => {
  await mkdir(path.join(out, "parity"), { recursive: true });
  const bffKey = key();
  api = createFakeApi({ bffKey });
  api.bffKey = bffKey;
  const shared = api.server.listeners("request")[0];
  api.server.removeAllListeners("request");
  api.server.on("request", (req, res) => {
    const pathname = new URL(req.url, "http://fake").pathname;
    if (OWN.test(pathname)) void mine(req, res, pathname).catch(() => { if (!res.headersSent) res.writeHead(500); res.end(); });
    else shared(req, res);
  });
  apiPort = await api.listen();
  next = await startNextWithPortRetry({}, async ({ port, attempt, track }) => {
    logFile = createWriteStream(nextAttemptLog(path.join(out, "next.log"), attempt), { flags: "wx", mode: 0o600 });
    await once(logFile, "open");
    const bin = path.join(root, "apps/storefront/node_modules/next/dist/bin/next");
    const child = spawn(process.execPath, [bin, "start", "--hostname", "127.0.0.1", "--port", String(port)], {
      cwd: path.join(root, "apps/storefront"),
      env: { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1", COMMERCE_BUYER_WEB_ENABLED: "1", COMMERCE_BUYER_API_ORIGIN: `http://127.0.0.1:${apiPort}`, COMMERCE_BUYER_BFF_KEY: bffKey, COMMERCE_BUYER_COOKIE_KEY: key(), COMMERCE_BUYER_SESSION_TTL: "3600" },
      stdio: ["ignore", logFile, logFile],
    });
    track(child, logFile);
    children.add(child);
    for (let i = 0; i < 400; i++) {
      if (child.exitCode !== null) throw new Error("Next exited before readiness");
      try { if ((await relay(port, { url: "/robots.txt", method: "GET", headers: { host: HOST } }, Buffer.alloc(0))).status === 200) return { port, child }; } catch { /* not up yet */ }
      await new Promise((r) => setTimeout(r, 100));
    }
    throw new Error("Next readiness timeout");
  });
  const certDir = await mkdtemp(path.join(tmpdir(), "lc-parity-buyer-edge-"));
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path.join(certDir, "key.pem"), "-out", path.join(certDir, "cert.pem"), "-days", "1", "-subj", `/CN=${HOST}`], { stdio: "ignore" });
  edge = https.createServer({ key: await readFile(path.join(certDir, "key.pem")), cert: await readFile(path.join(certDir, "cert.pem")) }, async (req, res) => {
    try {
      const chunks = []; for await (const x of req) chunks.push(x);
      const r = await relay(next.port, req, Buffer.concat(chunks));
      const headers = { ...r.headers }; delete headers["transfer-encoding"]; delete headers.connection;
      res.writeHead(r.status, headers); res.end(r.body);
    } catch { if (!res.headersSent) res.writeHead(502); res.end(); }
  });
  const edgePort = await listen(edge);
  await rm(certDir, { recursive: true, force: true });
  proxy = http.createServer((_, res) => { res.writeHead(403); res.end(); });
  proxy.on("connect", (req, socket, head) => {
    if (req.url !== `${HOST}:443`) { socket.destroy(); return; }
    const upstream = net.connect(edgePort, "127.0.0.1", () => { socket.write("HTTP/1.1 200 Connection Established\r\n\r\n"); if (head.length) upstream.write(head); socket.pipe(upstream).pipe(socket); });
    for (const s of [socket, upstream]) { sockets.add(s); s.on("close", () => sockets.delete(s)); s.on("error", () => { socket.destroy(); upstream.destroy(); }); }
  });
  const proxyPort = await listen(proxy);
  browser = await launch({ headless: true, proxy: { server: `http://127.0.0.1:${proxyPort}` } });
  desktop = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width: 1440, height: 900 }, locale: "zh-TW", acceptDownloads: true }));
  phone = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true, locale: "zh-TW", acceptDownloads: true }));
}, { timeout: 180000 });

after(async () => {
  await writeFile(path.join(out, "click-ledger.json"), JSON.stringify({ tier: "MOCK", sha, ledger }, null, 2) + "\n");
  await writeFile(path.join(out, "manifest.json"), JSON.stringify({ tier: "MOCK", sha, source: "output/parity-buyer-after/FRAME-COPY.md", captures, deferred }, null, 2) + "\n");
  await desktop?.close(); await phone?.close(); await browser?.close();
  for (const s of sockets) s.destroy();
  for (const c of children) { try { c.kill("SIGTERM"); } catch { /* already gone */ } }
  edge?.closeAllConnections?.();
  await new Promise((r) => edge?.close(r) ?? r()); await new Promise((r) => proxy?.close(r) ?? r());
  await api?.close(); logFile?.end();
});

// ---- helpers ------------------------------------------------------------------------------------------------------------------------------------------------
const escape = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
const norm = (s) => s.replace(/\s+/g, " ").trim();
const DEVICES = { d: () => desktop, m: () => phone };
const frameFile = (id) => id.replace(/:/g, "-");

class Check {
  constructor(page, label) { this.page = page; this.label = label; this.problems = []; }
  async load() { await this.page.waitForLoadState("networkidle").catch(() => {}); await this.page.waitForTimeout(700); this.body = norm(await this.page.locator("body").innerText()); }
  async step(what, fn) { try { await fn(); } catch (e) { this.problems.push(`${what}: ${String(e.message).split("\n")[0].slice(0, 160)}`); } }
  // Signed visible text (substring of the normalised body text; lines of a multi-line node are separate expectations).
  text(...items) { for (const t of items) if (!this.body.includes(norm(t))) this.problems.push(`missing text 「${t}」`); }
  gone(...items) { for (const t of items) if (this.body.includes(norm(t))) this.problems.push(`legacy text still shown 「${t}」`); }
  async heading(...names) { for (const n of names) if ((await this.page.getByRole("heading", { name: n, exact: true }).count()) === 0) this.problems.push(`missing heading 「${n}」`); }
  async noHeading(...names) { for (const n of names) if ((await this.page.getByRole("heading", { name: n, exact: true }).count()) > 0) this.problems.push(`legacy heading still shown 「${n}」`); }
  async shot(frame) {
    const file = `parity/${frameFile(frame)}.png`;
    await this.page.screenshot({ path: path.join(out, file), fullPage: true });
    captures.push({ frame, file, route: this.page.url().replace(origin, "") });
  }
  done() { assert.deepEqual(this.problems, [], `${this.label}\n${this.problems.join("\n")}`); }
}

async function click(page, locator, row) {
  await locator.click();
  ledger.push({ page: row.page, control: row.control, action: "click", expected: row.expected, actual: "clicked", result: "PASS" });
}

// Reach an order the way a buyer does: lookup form -> real submit -> /orders/{id}. Selectors stay structural so the setup survives the redesign.
async function openOrder(ctx, n) {
  const page = await ctx.newPage();
  await page.goto(`${origin}/zh-TW/orders/lookup`);
  const inputs = page.locator("main form input");
  await inputs.nth(0).fill(refOf(n));
  await inputs.nth(1).fill("name@example.com");
  await Promise.all([page.waitForURL(/\/zh-TW\/orders\/[0-9a-f-]{36}$/, { timeout: 15000 }), click(page, page.locator("main form button[type=submit]"), { page: "查詢訂單", control: "submit", expected: "navigate to the order" })]);
  return page;
}

const btn = (page, name) => page.locator("a,button").filter({ hasText: new RegExp(`^\\s*${escape(name)}\\s*$`) });
const need = async (c, ...names) => { for (const n of names) if ((await btn(c.page, n).count()) === 0) c.problems.push(`missing control 「${n}」`); };
const absent = async (c, ...names) => { for (const n of names) if ((await btn(c.page, n).count()) > 0) c.problems.push(`control 「${n}」 must not be shown in this frame`); };

// ---- shared order expectations -------------------------------------------------------------------------------------------------------------------------------
const HOME_ADDR = ["商家自行安排配送", "示範買家 · 09•• ••• ••01", "台北市中正區示範路 1 號"];
const CVS_ADDR = ["門市取貨 · 7-ELEVEN", "示範買家 · 09•• ••• ••01", "示範門市 · 編號 000001", "台北市中正區示範路 20 號"];
const LEGACY_ORDER = ["你的訂單", "訂單金額", "前往新分頁付款"];
const FULL_ID = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/;

async function orderCommon(c, { addr }) {
  await c.heading("商品明細", "收件與配送");
  c.text("焦糖托特包 × 1", "NT$1,280", "運費", "NT$0", "訂單合計", ...addr);
  c.gone(...LEGACY_ORDER);
  if (FULL_ID.test(c.body)) c.problems.push("full order id shown; the frames show the short order number only");
  if ((await c.page.getByRole("heading", { name: /^訂單 [A-Z0-9][A-Z0-9-]{3,}$/ }).count()) === 0) c.problems.push("missing heading 「訂單 <short number>」");
  if ((await c.page.getByTestId("order-id").count()) > 0) c.problems.push("legacy full order id row (data-testid=order-id) still rendered");
}

// scenario: one lookup-reachable order, checked on both viewports with the frame ids of each device
const ORDERS = [
  { key: "card-home-awaiting", n: 1, o: { commercial: "DRAFT", pay: { state: "NOT_STARTED", handoff: "NONE" } }, addr: HOME_ADDR, frames: { d: "4:9963", m: "4:10049" },
    text: ["待付款", "完成付款", "商品 NT$1,280 + 運費 NT$0", "若付款結果未確認，請先查詢這筆訂單，避免重複付款。", "配送狀態：備貨中"],
    headings: ["完成付款"], buttons: ["前往安全付款頁", "重新查詢付款狀態", "聯絡商店", "申請售後"], gone: [] },
  { key: "card-cvs-awaiting", n: 2, o: { kind: "cvs", commercial: "DRAFT", pay: { state: "NOT_STARTED", handoff: "NONE" } }, addr: CVS_ADDR, frames: { d: "25:943", m: "25:995" },
    text: ["待付款", "完成付款", "商品 NT$1,280 + 運費 NT$0", "若付款結果未確認，請先查詢這筆訂單，避免重複付款。", "配送狀態：備貨中"],
    headings: ["完成付款"], buttons: ["前往安全付款頁", "重新查詢付款狀態", "聯絡商店", "申請售後"], gone: [] },
  { key: "card-home-confirmed", n: 3, o: { commercial: "CONFIRMED", pay: { state: "CAPTURED", handoff: "CLOSED", methods: false } }, addr: HOME_ADDR, frames: { d: "1198:14682", m: "1198:15048" },
    text: ["訂單已確認", "付款狀態：已記錄付款結果", "訂單狀態：訂單已確認", "商品 NT$1,280 + 運費 NT$0。", "配送狀態：備貨中"],
    headings: ["付款"], buttons: ["聯絡商店", "申請售後"], gone: [] },
  { key: "card-cvs-confirmed", n: 4, o: { kind: "cvs", commercial: "CONFIRMED", pay: { state: "CAPTURED", handoff: "CLOSED", methods: false } }, addr: CVS_ADDR, frames: { d: "1198:14549", m: "1198:14943" },
    text: ["訂單已確認", "付款狀態：已記錄付款結果", "訂單狀態：訂單已確認", "商品 NT$1,280 + 運費 NT$0。", "配送狀態：備貨中"],
    headings: ["付款"], buttons: ["聯絡商店", "申請售後"], gone: [] },
  { key: "card-home-closed", n: 5, o: { commercial: "CANCELLED", pay: { state: "CLOSED_UNPAID", handoff: "CLOSED", methods: false } }, addr: HOME_ADDR, frames: { d: "1198:14748", m: "1198:15099" },
    text: ["訂單已取消", "付款狀態：付款已關閉，未扣款", "訂單狀態：訂單已取消", "NT$1,280 沒有扣款。如要購買，請回購物車重新下單。", "配送狀態：未出貨（訂單已取消）"],
    headings: ["付款"], buttons: ["回購物車重新下單", "聯絡商店"], noButtons: ["申請售後"], gone: [] },
  { key: "card-cvs-closed", n: 6, o: { kind: "cvs", commercial: "CANCELLED", pay: { state: "CLOSED_UNPAID", handoff: "CLOSED", methods: false } }, addr: CVS_ADDR, frames: { d: "1198:14615", m: "1198:14994" },
    text: ["訂單已取消", "付款狀態：付款已關閉，未扣款", "訂單狀態：訂單已取消", "NT$1,280 沒有扣款。如要購買，請回購物車重新下單。", "配送狀態：未出貨（訂單已取消）"],
    headings: ["付款"], buttons: ["回購物車重新下單", "聯絡商店"], noButtons: ["申請售後"], gone: [] },
  { key: "card-home-pending", n: 7, o: { commercial: "AWAITING_PAYMENT", pay: { state: "PENDING", handoff: "READY", methods: false } }, addr: HOME_ADDR, frames: { d: "1207:12062", m: "1207:12130" },
    text: ["等待付款結果", "付款狀態：等待付款結果", "訂單狀態：等待付款結果", "商品 NT$1,280 + 運費 NT$0。", "已請求跳轉付款，請查看另一個分頁，再重新整理此訂單查詢結果。這不代表付款成功。", "付款會在新分頁開啟，完成後回到這裡查詢結果。", "配送狀態：備貨中"],
    headings: ["付款"], buttons: ["繼續原付款請求", "取消付款", "聯絡商店", "申請售後"], gone: [] },
  { key: "card-cvs-pending", n: 8, o: { kind: "cvs", commercial: "AWAITING_PAYMENT", pay: { state: "PENDING", handoff: "READY", methods: false } }, addr: CVS_ADDR, frames: { d: "1207:11941", m: "1207:12009" },
    text: ["等待付款結果", "付款狀態：等待付款結果", "訂單狀態：等待付款結果", "商品 NT$1,280 + 運費 NT$0。", "已請求跳轉付款，請查看另一個分頁，再重新整理此訂單查詢結果。這不代表付款成功。", "付款會在新分頁開啟，完成後回到這裡查詢結果。", "配送狀態：備貨中"],
    headings: ["付款"], buttons: ["繼續原付款請求", "取消付款", "聯絡商店", "申請售後"], gone: [] },
];
// pay-at-pickup (CVS) and cash-on-delivery (home) collection states; the frame ids are [desktop, mobile]
const PICKUP = [
  ["PENDING", "AWAITING_COLLECTION", "AT_STORE", ["1198:14815", "1198:15153"], ["訂單已確認", "取貨時付款 NT$1,280", "取貨進度：已到店，請取貨", "無需線上付款，到店取貨時再付款。", "7-ELEVEN · 示範門市 · 000001", "到店通知由物流業者發送，進度可能延遲數小時。"]],
  ["COLLECTED", "CONFIRMED", "PICKED_UP", ["1222:21567", "1222:22137"], ["訂單已確認", "已取貨付款", "取貨進度：已取貨", "7-ELEVEN · 示範門市 · 000001", "到店通知由物流業者發送，進度可能延遲數小時。"]],
  ["RETURNED", "CONFIRMED", "UNCLAIMED", ["1222:21624", "1222:22181"], ["訂單已確認", "未取貨，已退回", "取貨進度：逾期未取，退回中", "7-ELEVEN · 示範門市 · 000001"]],
  ["REFUNDED_OFFLINE", "CONFIRMED", "PICKED_UP", ["1222:21681", "1222:22225"], ["訂單已確認", "賣家已在本站外退款", "取貨進度：已取貨", "7-ELEVEN · 示範門市 · 000001"]],
  ["CANCELLED", "CANCELLED", null, ["1222:21738", "1222:22269"], ["訂單已取消", "賣家已取消訂單"]],
];
const COD = [
  ["PENDING", "AWAITING_COLLECTION", ["1222:21795", "1222:22313"], ["到貨需付 NT$1,280", "請以現金付款。商家會記錄收款結果；退款在本站之外進行。"]],
  ["COLLECTED", "CONFIRMED", ["1222:21852", "1222:22357"], ["已貨到收款", "原訂單代收金額 NT$1,280"]],
  ["RETURNED", "CONFIRMED", ["1222:21909", "1222:22401"], ["未取貨 — 已退回", "原訂單代收金額 NT$1,280"]],
  ["REFUNDED_OFFLINE", "CONFIRMED", ["1222:21966", "1222:22445"], ["商家已在本站之外退款", "原訂單代收金額 NT$1,280"]],
  ["CANCELLED", "CANCELLED", ["1222:22023", "1222:22489"], ["商家已取消此訂單", "原訂單代收金額 NT$1,280"]],
  ["RESTOCKED", "CONFIRMED", ["1222:22080", "1222:22533"], ["未取貨 — 已退回商家", "原訂單代收金額 NT$1,280"]],
];
let seq = 100, dyn = 300;
const fresh = () => ++dyn;
for (const [state, commercial, cvsState, frames, text] of PICKUP) {
  const n = ++seq;
  ORDERS.push({ key: `pickup-${state}`, n, o: { mode: "pay_at_pickup", kind: "cvs", collection: state, commercial, cvs_state: cvsState }, addr: CVS_ADDR,
    frames: { d: frames[0], m: frames[1] }, text, headings: ["門市取貨"], buttons: ["聯絡商店", "申請售後"], gone: ["到貨時要付的金額"] });
}
for (const [state, commercial, frames, text] of COD) {
  const n = ++seq;
  ORDERS.push({ key: `cod-${state}`, n, o: { mode: "cash_on_delivery", kind: "home", collection: state, commercial }, addr: HOME_ADDR,
    frames: { d: frames[0], m: frames[1] }, text: [...text, "下單時的物流商 · 黑貓（手工出貨）"], headings: ["貨到付款"], buttons: ["聯絡商店", "申請售後"], gone: [] });
}
for (const s of ORDERS) makeOrder(s.n, s.o);

for (const s of ORDERS) {
  test(`order status · ${s.key}`, async () => {
    for (const dev of ["d", "m"]) {
      const page = await openOrder(DEVICES[dev](), s.n);
      const c = new Check(page, `${s.key} ${dev}`);
      await c.load();
      await c.step("order common", () => orderCommon(c, { addr: s.addr }));
      await need(c, "‹ 查詢訂單", ...s.buttons);
      await absent(c, ...(s.noButtons ?? []));
      c.text(...s.text); c.gone(...(s.gone ?? []));
      await c.step("headings", () => c.heading(...s.headings));
      await c.shot(s.frames[dev]);
      try { c.done(); } finally { await page.close(); }
    }
  });
}

// cancel-payment flow: the frame says the confirm is the browser's own, so assert that a confirm was shown, accept it, then the requested-cancel state
for (const [key, n, kind, frames] of [["home", 0, "home", { confirmD: "1222:19657", confirmM: "1222:19813", doneD: "1222:19724", doneM: "1222:19867", addr: HOME_ADDR }], ["cvs", 0, "cvs", { confirmD: "1222:19371", confirmM: "1222:19527", doneD: "1222:19438", doneM: "1222:19581", addr: CVS_ADDR }]]) {
  test(`cancel payment · ${key}`, async () => {
    for (const dev of ["d", "m"]) {
      const orderN = fresh();
      makeOrder(orderN, { kind, commercial: "AWAITING_PAYMENT", pay: { state: "PENDING", handoff: "READY", methods: false } });
      const page = await openOrder(DEVICES[dev](), orderN);
      const c = new Check(page, `cancel ${key} ${dev}`);
      await c.load();
      await c.shot(dev === "d" ? frames.confirmD : frames.confirmM); // the page behind the native confirm
      let dialogType = null;
      page.once("dialog", async (dialog) => { dialogType = dialog.type(); await dialog.accept(); });
      await c.step("click 取消付款", () => click(page, page.getByRole("button", { name: /^\s*取消付款\s*$/ }), { page: "order", control: "取消付款", expected: "native confirm, then requested-cancel state" }));
      await page.waitForTimeout(1200);
      if (dialogType !== "confirm") c.problems.push(`a native confirm must be shown before cancelling (got ${dialogType})`);
      await c.load();
      c.text("已請求取消，正在等待支付服務商確認，請重新整理此訂單查詢。", "付款狀態：等待付款結果", "訂單狀態：等待付款結果");
      await page.reload(); await c.load(); // persistence: the requested state survives a refresh (server state)
      c.text("已請求取消，正在等待支付服務商確認，請重新整理此訂單查詢。");
      await c.shot(dev === "d" ? frames.doneD : frames.doneM);
      try { c.done(); } finally { await page.close(); }
    }
  });
}

// 前往付款 interstitial after starting payment
for (const [key, n, kind, frames] of [["home", 0, "home", { d: "29:1064", m: "29:1095" }], ["cvs", 0, "cvs", { d: "1198:10796", m: "1198:10886" }]]) {
  test(`payment hand-off page · ${key}`, async () => {
    for (const dev of ["d", "m"]) {
      const orderN = fresh();
      makeOrder(orderN, { kind, commercial: "DRAFT", pay: { state: "NOT_STARTED", handoff: "NONE" } });
      const page = await openOrder(DEVICES[dev](), orderN);
      const c = new Check(page, `handoff ${key} ${dev}`);
      await c.load();
      // The provider tab must stay open until the hand-off page is checked: OrderPayment fences on destination.ready(), so closing it early reads as an aborted hand-off.
      const popup = page.context().waitForEvent("page", { timeout: 4000 }).catch(() => null);
      await c.step("click 前往安全付款頁", () => click(page, page.getByRole("button", { name: /^\s*前往安全付款頁\s*$/ }), { page: "order", control: "前往安全付款頁", expected: "前往付款 page" }));
      const providerTab = await popup; await c.load();
      await c.heading("前往付款");
      c.text("付款會在新分頁開啟，請保留此商店分頁。完成後回到這裡查詢結果。", "如果付款結果尚未確認，請勿重新建立訂單或重複付款。");
      if (!/訂單 [A-Z0-9][A-Z0-9-]{3,} · NT\$1,280/.test(c.body)) c.problems.push("missing line 「訂單 <short number> · NT$1,280」");
      await need(c, "查看付款結果");
      await c.shot(frames[dev]);
      try { c.done(); } finally { await providerTab?.close().catch(() => {}); await page.close(); }
    }
  });
}

// terminal order views (no payment entry)
for (const [key, n, o, frames, text, headings] of [
  ["completed", 0, { line: "bag", unit: 48000, commercial: "CONFIRMED", fulfillment: "MERCHANT_SHIPPED", pay: { state: "CAPTURED", handoff: "CLOSED", methods: false },
    shipment: { status: "SHIPPED", carrier_code: "other", carrier_name: "其他物流", /* validShipment (lib/purchase.ts) refuses "other" with a null name */ tracking_number: "TRK12345", tracking_url: null, recorded_at: iso(-3600e3) } },
    { d: "63:1475", m: "63:1476" }, ["已完成", "這筆訂單已完成", "訂單資訊保留供您核對。", "收納袋 × 1", "付款狀態", "已付款", "訂單狀態", "商品金額", "NT$480", "運費", "NT$0", "合計"], ["訂購商品", "付款與配送"]],
  ["cancelled", 0, { line: "bag", unit: 48000, commercial: "CANCELLED", pay: { state: "NOT_STARTED", handoff: "NONE", methods: false } },
    { d: "63:1477", m: "63:1478" }, ["已取消", "這筆訂單已取消", "無需再為這筆訂單付款。", "收納袋 × 1", "付款狀態", "未付款", "訂單狀態", "商品金額", "NT$480", "這筆訂單已取消，不提供付款入口。", "合計"], ["訂購商品", "付款與配送"]],
]) {
  test(`terminal order view · ${key}`, async () => {
    for (const dev of ["d", "m"]) {
      const n = fresh();
      makeOrder(n, o);
      const page = await openOrder(DEVICES[dev](), n);
      const c = new Check(page, `terminal ${key} ${dev}`);
      await c.load();
      c.text(...text);
      await c.heading(...headings);
      if ((await page.getByRole("heading", { name: /^[A-Z0-9][A-Z0-9-]{3,}$/ }).count()) === 0) c.problems.push("missing heading 「<short order number>」");
      if (FULL_ID.test(c.body)) c.problems.push("full order id shown");
      if ((await page.locator("a,button").filter({ hasText: /^\s*(‹|返回查詢訂單)\s*$/ }).count()) === 0) c.problems.push("missing back control (‹ / 返回查詢訂單)");
      c.gone(...LEGACY_ORDER);
      await c.shot(frames[dev]);
      try { c.done(); } finally { await page.close(); }
    }
  });
}

// RVFIX-444 (RD6): a refund never changes commercial/fulfillment state, so a shipped card order that was refunded must read the payment view:
// show the §7.2 refund line and never 已付款.
const SHIPPED = { line: "bag", unit: 48000, commercial: "CONFIRMED", fulfillment: "MERCHANT_SHIPPED",
  shipment: { status: "SHIPPED", carrier_code: "other", carrier_name: "其他物流", tracking_number: "TRK12345", tracking_url: null, recorded_at: iso(-3600e3) } };
for (const [key, pay, shown, notShown] of [
  ["refunded", { state: "REFUNDED", handoff: "CLOSED", methods: false, refund: { refunded_minor: 48000, pending_minor: 0 } }, ["已退款 NT$480"], ["已付款", "退款處理中"]],
  ["partially-refunded", { state: "PARTIALLY_REFUNDED", handoff: "CLOSED", methods: false, refund: { refunded_minor: 20000, pending_minor: 0 } }, ["已退款 NT$200"], ["已付款", "退款處理中"]],
  ["refund-processing", { state: "CAPTURED", handoff: "CLOSED", methods: false, refund: { refunded_minor: 0, pending_minor: 48000 } }, ["退款處理中"], ["已付款", "已退款"]],
  ["review-required", { state: "REVIEW_REQUIRED", handoff: "CLOSED", methods: false }, ["付款結果需要確認"], ["已付款"]],
]) {
  test(`terminal order view · shipped card order · ${key}`, async () => {
    const n = fresh();
    makeOrder(n, { ...SHIPPED, pay });
    const page = await openOrder(DEVICES.d(), n);
    const c = new Check(page, `terminal shipped ${key}`);
    await c.load();
    c.text(...shown);
    c.gone(...notShown);
    try { c.done(); } finally { await page.close(); }
  });
}

// ---- order lookup / order link / payment return ----------------------------------------------------------------------------------------------------------------
test("order lookup · default and not-found states", async () => {
  for (const [dev, frame, missFrame] of [["d", "26:708", "1253:17225"], ["m", "26:995", "1253:17256"]]) {
    const page = await DEVICES[dev]().newPage();
    await page.goto(`${origin}/zh-TW/orders/lookup`);
    const c = new Check(page, `lookup ${dev}`);
    await c.load();
    c.text("查詢訂單", "尋找你的訂單", "請輸入這家商店的訂單編號及下單時填寫的聯絡資料。", "找不到訂單時，請核對商店、訂單編號與聯絡資料。");
    c.gone("查詢成功後，此瀏覽器目前的購物內容會被取代，只能查看這筆訂單。", "輸入訂單通知信中的訂單編號");
    for (const label of ["訂單編號", "電子郵件或手機號碼"]) if ((await page.getByLabel(label, { exact: true }).count()) === 0) c.problems.push(`missing field 「${label}」`);
    if ((await page.getByRole("button", { name: /^\s*查詢訂單\s*$/ }).count()) === 0) c.problems.push("missing button 「查詢訂單」");
    await c.shot(frame);
    // not-found: nothing matches 0123-ABCD-4567; the typed values stay (1253:17225 / 1253:17256)
    const inputs = page.locator("main form input");
    await inputs.nth(0).fill("0123-ABCD-4567"); await inputs.nth(1).fill("name@example.com");
    await click(page, page.locator("main form button[type=submit]"), { page: "查詢訂單", control: "submit", expected: "not-found message, values kept" });
    await page.waitForTimeout(1200); await c.load();
    c.text("找不到符合的訂單，請確認訂單編號與電子郵件或手機號碼。", "你輸入的內容已保留，可直接修改後再查詢。");
    if ((await inputs.nth(0).inputValue()) !== "0123-ABCD-4567" || (await inputs.nth(1).inputValue()) !== "name@example.com") c.problems.push("typed values were not kept after not-found");
    await c.shot(missFrame);
    try { c.done(); } finally { await page.close(); }
  }
});

test("order link · expired link page", async () => {
  for (const [dev, frame] of [["d", "26:746"], ["m", "26:1028"]]) {
    const page = await DEVICES[dev]().newPage();
    await page.goto(`${origin}/zh-TW/order-link`);
    const c = new Check(page, `order-link ${dev}`);
    await c.load();
    c.text("訂單連結已失效", "請重新確認身分", "這個連結無法再使用。你可以使用訂單編號查詢，或聯絡下單的商店。");
    await c.shot(frame);
    const go = page.locator("a,button").filter({ hasText: /^\s*前往查詢訂單\s*$/ });
    if ((await go.count()) === 0) c.problems.push("missing control 「前往查詢訂單」");
    else {
      await Promise.all([page.waitForURL(/\/zh-TW\/orders\/lookup$/, { timeout: 8000 }).catch(() => c.problems.push("前往查詢訂單 did not open the lookup page")), click(page, go.first(), { page: "order-link", control: "前往查詢訂單", expected: "lookup page" })]);
    }
    try { c.done(); } finally { await page.close(); }
  }
});

test("payment return · neutral page", async () => {
  for (const [dev, frame] of [["d", "1207:16447"], ["m", "1207:16465"]]) {
    const page = await DEVICES[dev]().newPage();
    await page.goto(`${origin}/payment/return`);
    const c = new Check(page, `payment-return ${dev}`);
    await c.load();
    // en and zh-TW strings of the frame; the frame has no Simplified Chinese string, so none may be shown
    c.text("Return to your store", "Check your order",
      "This page does not confirm payment. Return to your original store tab and select “Refresh order” to check the result. You may close this tab. If you lost the store tab, reopen your store; this page cannot identify it for you.",
      "返回商店查詢訂單", "此頁面不代表付款成功。請回到原本的商店分頁，點選「重新整理訂單」查詢結果。你可以關閉本頁。若原分頁已關閉，請自行重新開啟商店；此頁面無法判斷你所屬的商店。");
    c.gone("返回商店查询订单", "此页面不代表付款成功。请回到原来的商店标签页，点击“刷新订单”查询结果。");
    await c.shot(frame);
    try { c.done(); } finally { await page.close(); }
  }
});

// ---- privacy / data deletion -----------------------------------------------------------------------------------------------------------------------------------
const PRIVACY_TEXT = ["隱私與資料", "你的隱私選擇", "購買不代表同意；關閉以下選擇不影響現有訂單。", "行銷私訊", "廣告個人化", "目前未開啟", "關閉廣告個人化會停止後續事件；已傳送的事件無法撤回。",
  "下載資料副本", "下載本店持有的你的資料副本，包含訂單及目前的隱私選擇。", "抹除我的資料", "抹除會撤回你的選擇並結束購物連線；訂單、付款與出貨紀錄仍會保留。", "若仍有進行中的付款或退款，需先等待該操作結束。",
  "資料如何用於購物", "收件人姓名、電話、地址或取貨門市資料，用於處理訂單與配送。", "可用付款方式與結果，請以結帳及訂單頁顯示為準。"];

function resetPrivacy() { Object.assign(privacyState, { marketing: false, ads: false, erased: false, exportMode: "ok", eraseMode: "ok" }); }

test("privacy · overview, consent settings, export", async () => {
  for (const [dev, frames] of [["d", { main: "5:15460", consent: "26:888", done: "37:1083", fail: "37:1140" }], ["m", { main: "5:15540", consent: "26:1150", done: "37:1114", fail: "37:1173" }]]) {
    resetPrivacy();
    const page = await DEVICES[dev]().newPage();
    await page.goto(`${origin}/zh-TW/privacy`);
    const c = new Check(page, `privacy ${dev}`);
    await c.load();
    c.text(...PRIVACY_TEXT);
    await c.noHeading("你的隱私");
    await need(c, "管理這兩項選擇", "下載我的資料", "檢視抹除範圍");
    await c.shot(frames.main);
    // consent settings: tick, save, refresh, still ticked (persistence)
    await click(page, btn(page, "管理這兩項選擇").first(), { page: "隱私與資料", control: "管理這兩項選擇", expected: "隱私與同意設定" });
    await c.load();
    c.text("隱私與同意設定", "必要資料", "完成訂購、配送與查詢訂單所需的資料，依商店的隱私政策處理。", "選擇性使用", "接收商店行銷資訊", "允許廣告個人化使用", "可隨時更新選擇，不影響已成立訂單。");
    await need(c, "儲存選擇");
    await c.shot(frames.consent);
    const box = page.getByRole("checkbox", { name: /接收商店行銷資訊/ });
    if ((await box.count()) === 0) c.problems.push("missing checkbox 「接收商店行銷資訊」");
    else {
      await click(page, box, { page: "隱私與同意設定", control: "接收商店行銷資訊", expected: "checked" });
      await click(page, btn(page, "儲存選擇").first(), { page: "隱私與同意設定", control: "儲存選擇", expected: "saved" });
      await page.waitForTimeout(800); await page.reload(); await c.load();
      if ((await btn(page, "管理這兩項選擇").count()) > 0) { await click(page, btn(page, "管理這兩項選擇").first(), { page: "隱私與資料", control: "管理這兩項選擇", expected: "saved choice is still ticked" }); await c.load(); }
      if (!(await page.getByRole("checkbox", { name: /接收商店行銷資訊/ }).isChecked().catch(() => false))) c.problems.push("consent choice did not persist after refresh");
    }
    // export success then failure
    await page.goto(`${origin}/zh-TW/privacy`); await c.load();
    const download = page.waitForEvent("download", { timeout: 8000 }).catch(() => null);
    await click(page, btn(page, "下載我的資料").first(), { page: "隱私與資料", control: "下載我的資料", expected: "資料副本已下載" });
    const file = await download;
    if (!file || file.suggestedFilename() !== "my-data.json") c.problems.push(`download must be my-data.json (got ${file?.suggestedFilename() ?? "no download"})`);
    await page.waitForTimeout(800); await c.load();
    c.text("資料副本已下載", "已完成下載", "請在你的裝置中查看 my-data.json，並妥善保存。", "下載資料不會刪除資料，也不會更改你的選擇。");
    await need(c, "返回隱私與資料");
    await c.shot(frames.done);
    privacyState.exportMode = "too_large";
    await page.goto(`${origin}/zh-TW/privacy`); await c.load();
    await click(page, btn(page, "下載我的資料").first(), { page: "隱私與資料", control: "下載我的資料", expected: "資料副本暫時無法下載" });
    await page.waitForTimeout(800); await c.load();
    c.text("資料副本暫時無法下載", "下載未完成", "請再試一次；若資料量過多，請聯絡購買的店鋪。");
    await need(c, "再試一次", "聯絡店鋪");
    await c.shot(frames.fail);
    privacyState.exportMode = "ok";
    try { c.done(); } finally { await page.close(); }
  }
});

test("privacy · erase confirm, completed, blocked and unknown", async () => {
  for (const [dev, frames] of [["d", { confirm: "29:950", erased: "29:1011", blocked: "37:1201", unknown: "37:1266" }], ["m", { confirm: "29:983", erased: "29:1040", blocked: "37:1236", unknown: "37:1298" }]]) {
    resetPrivacy();
    const page = await DEVICES[dev]().newPage();
    const c = new Check(page, `erase ${dev}`);
    const toConfirm = async () => {
      await page.goto(`${origin}/zh-TW/privacy`); await c.load();
      await click(page, btn(page, "檢視抹除範圍").first(), { page: "隱私與資料", control: "檢視抹除範圍", expected: "確認抹除資料" });
      await c.load();
    };
    const type = async () => { const f = page.getByLabel("輸入 ERASE 以確認"); if ((await f.count()) === 0) c.problems.push("missing field 「輸入 ERASE 以確認」"); else await f.fill("ERASE"); };
    await toConfirm();
    c.text("確認抹除資料", "請先確認保留範圍", "這會撤回你的選擇並結束購物連線；訂單、付款、退款與出貨紀錄仍會保留。");
    await need(c, "取消");
    await c.shot(frames.confirm);
    // blocked (erasure_blocked)
    privacyState.eraseMode = "blocked";
    await type();
    await click(page, btn(page, "確認抹除我的資料").first(), { page: "確認抹除資料", control: "確認抹除我的資料", expected: "目前無法抹除資料" });
    await page.waitForTimeout(800); await c.load();
    c.text("目前無法抹除資料", "仍有進行中的付款或退款", "請先完成或等待這些操作結束，再回來抹除資料。", "資料尚未抹除，你仍可查看現有訂單。");
    await need(c, "查看訂單", "返回隱私與資料");
    await c.shot(frames.blocked);
    // unknown result
    privacyState.eraseMode = "unknown";
    await toConfirm(); await type();
    await click(page, btn(page, "確認抹除我的資料").first(), { page: "確認抹除資料", control: "確認抹除我的資料", expected: "正在確認抹除結果" });
    await page.waitForTimeout(800); await c.load();
    c.text("正在確認抹除結果", "尚未確認操作結果", "請用「確認同一次操作」取回結果。不要重新建立另一筆要求。");
    await need(c, "確認同一次操作", "暫時關閉");
    await c.shot(frames.unknown);
    // completed
    privacyState.eraseMode = "ok";
    await toConfirm(); await type();
    await click(page, btn(page, "確認抹除我的資料").first(), { page: "確認抹除資料", control: "確認抹除我的資料", expected: "資料抹除完成" });
    await page.waitForTimeout(800); await c.load();
    c.text("資料抹除完成");
    await need(c, "返回商店");
    await c.shot(frames.erased);
    try { c.done(); } finally { await page.close(); }
  }
});

test("data deletion instructions", async () => {
  for (const [dev, frame] of [["d", "5:15707"], ["m", "5:15793"]]) {
    const page = await DEVICES[dev]().newPage();
    await page.goto(`${origin}/zh-TW/data-deletion`);
    const c = new Check(page, `data-deletion ${dev}`);
    await c.load();
    c.text("資料刪除說明", "依資料來源選擇對應的處理方式。", "購物資料", "請使用下單時的裝置，開啟「隱私與資料」，再選擇「抹除我的資料」。", "抹除會撤回你的選擇並結束購物連線。訂單、付款與出貨紀錄仍會保留。",
      "Facebook / Instagram 資料", "移除應用程式連結與抹除店鋪購物資料是不同操作。請依 Meta 帳號中的應用程式設定操作。", "無法使用原裝置，或需要協助時，請聯絡購買的店鋪。");
    await need(c, "前往隱私與資料", "聯絡店鋪");
    await c.shot(frame);
    const go = btn(page, "前往隱私與資料");
    if ((await go.count()) > 0) await Promise.all([page.waitForURL(/\/zh-TW\/privacy$/, { timeout: 8000 }).catch(() => c.problems.push("前往隱私與資料 did not open /privacy")), click(page, go.first(), { page: "資料刪除說明", control: "前往隱私與資料", expected: "/privacy" })]);
    try { c.done(); } finally { await page.close(); }
  }
});

// ---- live claim link ---------------------------------------------------------------------------------------------------------------------------------------------
test("claim link · confirm, expired and unconfirmed result", async () => {
  for (const [dev, frames] of [["d", { ok: "4:12347", gone: "66:1746", unknown: "66:1754" }], ["m", { ok: "4:12417", gone: "66:1747", unknown: "66:1755" }]]) {
    claimState.redeem = "ok";
    const page = await DEVICES[dev]().newPage();
    const c = new Check(page, `claim ${dev}`);
    await page.goto(`${origin}/zh-TW/claim#t=${CLAIM_TOKEN}`);
    await c.load();
    c.text("確認直播認領", "輕量帆布托特包", "認領連結不代表已付款；價格與可售數量會在結帳時再次確認。", "NT$1,280");
    c.gone("你登記的商品");
    await c.noHeading("你登記的商品");
    await need(c, "前往結帳（共 2 件）", "查看商品");
    await c.shot(frames.ok);
    // redeem result unknown -> 認領結果待確認
    claimState.redeem = "unknown";
    const go = btn(page, "前往結帳（共 2 件）");
    if ((await go.count()) > 0) {
      await click(page, go.first(), { page: "確認直播認領", control: "前往結帳（共 2 件）", expected: "認領結果待確認" });
      await page.waitForTimeout(1200); await c.load();
      c.text("認領結果待確認", "無法確認結果", "無法確認結果。請先重新讀取登記，再試一次。", "先確認目前登記狀態，避免重複送出。");
      await need(c, "重新讀取登記");
      await c.shot(frames.unknown);
    } else c.problems.push("cannot reach 認領結果待確認: no 前往結帳 control");
    // invalid link (no token in the fragment)
    await page.goto(`${origin}/zh-TW/claim`);
    await c.load();
    c.text("認領連結失效", "連結已失效或已更換，請私訊商家重新取得", "此連結目前無法使用。購物車內已有的商品不會因此清空。");
    await need(c, "查看購物車");
    await c.shot(frames.gone);
    try { c.done(); } finally { await page.close(); }
  }
});

// Frames the real routes cannot reach with existing data; recorded in manifest.json and the PR body, never silently dropped.
deferred.push(
  { frames: ["26:773", "26:1050", "26:815", "26:1087"], reason: "after-sales request form / request R-1001: no storefront route or buyer API for it in this module; the 申請售後 control on the order page is asserted, its destination is not" },
  { frames: ["1222:14056", "1222:14106"], reason: "CVS pickup info checkout form: belongs to PARITY-CHECKOUT (Not checkout/CVS files), listed in b-privacy by inventory" },
  { frames: ["63:1475", "63:1476", "4:12347", "4:12417"], reason: "variant label lines (奶油白 · 單一規格, 焦糖 · 數量 1) need a variant title the order / claim API does not return; not asserted" },
);
