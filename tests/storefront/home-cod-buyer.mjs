// home-cod R5 buyer half (migration 0107; docs/delivery/units/home-cod.md). Real production storefront Next -> private buyerhttp ->
// isolated PG, driven by tests/foundation/browser_home_cod_test.go (MOCK: no PSP, no carrier API on this path — cash on delivery is
// collected by the carrier and recorded by the merchant).
// BFF routes exercised: /api/buyer/{checkout-options,quotes,destination,checkout,orders/{id}}.
// One long-lived process that hands over to the merchant half through files in LC_HCOD_EVIDENCE: it writes ready-<step>, the Go test runs
// the admin Playwright phase (ship + record collected), and touches go-<step>.
//   place   four buyers (zh-TW desktop, en desktop, zh-TW 390px, en 390px) check out HOME delivery with cash on delivery, see the order
//           total plus the whole-TWD surcharge on the mode choice, and place the order (AWAITING_COLLECTION, collection PENDING)
//   go-1    the merchant ships one order and records the cash collected
//   after1  that buyer sees COLLECTED; the other three stay PENDING
// Wording: the zh-TW/en strings of apps/storefront/lib/cod-copy.ts that carry a contract meaning; everything else is located by data-testid.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { readFile, writeFile, mkdtemp, access } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { expect } from "@playwright/test";
import { engine, launch, ctxOpts, phone, iosZoomOffenders } from "./browser-engine.mjs";
import { reachCheckout } from "./shop-helpers.mjs";

const env = (name) => {
  const value = process.env[name];
  assert(value, `${name} is required`);
  return value;
};
const root = process.cwd(), evidence = env("LC_HCOD_EVIDENCE"), origin = env("LC_HCOD_ORIGIN"), product = env("LC_HCOD_PRODUCT");
const unit = Number(env("LC_HCOD_PRICE")), surcharge = Number(env("LC_HCOD_SURCHARGE"));
const total = unit * 2; // every order is 2 units: a whole-TWD subtotal, no shipping fee (the fixture's home policy is zero-fee)
const host = new URL(origin).host;
const children = new Set(), sockets = new Set(), contexts = [], logs = [];
const pause = (ms) => new Promise((r) => setTimeout(r, ms));
const listen = async (s) => { s.listen(0, "127.0.0.1"); await once(s, "listening"); return s.address().port; };
const certDir = await mkdtemp(path.join(tmpdir(), "lc-hcod-edge-"));
const pass = (name) => console.log(`PASS ${name}`);
let browser, edge, proxy;

const copy = {
  "zh-TW": {
    delivery: "選擇配送", quote: "取得目前總額", cod: /貨到付款/, createCod: "送出訂單（貨到付款）",
    paid: "已貨到收款", note: /現金/,
  },
  en: {
    delivery: "Choose delivery", quote: "Get current total", cod: /Cash on delivery/, createCod: "Place order (cash on delivery)",
    paid: "Paid on delivery", note: /cash/i,
  },
};
const pii = { recipient_name: "Synthetic Gate Recipient", phone: "+886900000091", region: "Synthetic Region", city: "Synthetic City", postal_code: "99991", line1: "Synthetic Address Ninety One", line2: "Synthetic Unit Ninety Two" };

function relayTo(target, req, body) {
  return new Promise((resolve, reject) => {
    const headers = { ...req.headers }; delete headers.connection; delete headers["transfer-encoding"];
    if (body.length) headers["content-length"] = String(body.length); else delete headers["content-length"];
    const call = http.request({ hostname: target.hostname, port: target.port, path: req.url, method: req.method, headers }, (res) => {
      const chunks = []; res.on("data", (x) => chunks.push(x)); res.on("error", reject);
      res.on("end", () => resolve({ status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks) }));
    });
    call.setTimeout(20000, () => call.destroy(new Error("owned relay deadline"))); call.on("error", reject); call.end(body);
  });
}
async function startNext() {
  const reserve = net.createServer(), port = await listen(reserve); await new Promise((r) => reserve.close(r));
  const log = createWriteStream(path.join(evidence, "home-cod-next.log"), { flags: "wx", mode: 0o600 }); logs.push(log); await once(log, "open");
  const childEnv = { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1" };
  for (const key of Object.keys(childEnv)) if (key.startsWith("LC_HCOD_")) delete childEnv[key];
  const child = spawn(process.execPath, [path.join(root, "apps/storefront/node_modules/next/dist/bin/next"), "start", "--hostname", "127.0.0.1", "--port", String(port)],
    { cwd: path.join(root, "apps/storefront"), env: childEnv, stdio: ["ignore", log, log] });
  children.add(child);
  for (let i = 0; i < 100; i++) {
    if (child.exitCode !== null) throw new Error("owned Next failed readiness");
    try { if ((await relayTo({ hostname: "127.0.0.1", port }, { url: "/api/buyer/session", method: "GET", headers: { host } }, Buffer.alloc(0))).status === 200) return port; } catch { /* not ready */ }
    await pause(50);
  }
  throw new Error("owned Next readiness timeout");
}
const manifest = path.join(evidence, "screenshots.json");
async function shot(page, name, locale, viewport) {
  const file = path.join(evidence, `home-cod-${name}-${locale}-${viewport}.png`);
  await page.screenshot({ path: file, fullPage: true });
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `horizontal overflow at ${viewport} ${locale} (${name})`);
  if (engine === "webkit" && viewport === "mobile") assert.deepEqual(await iosZoomOffenders(page), [], `iOS focus-zoom: form controls under 16px at ${viewport} ${locale}`);
  let list = []; try { list = JSON.parse(await readFile(manifest, "utf8")); } catch { /* first */ }
  list.push({ File: path.basename(file), Sha256: createHash("sha256").update(await readFile(file)).digest("hex"), Locale: locale, Viewport: viewport });
  await writeFile(manifest, JSON.stringify(list, null, 2));
}
async function newContext(mobile) {
  const c = await browser.newContext(ctxOpts(mobile
    ? { ...phone, viewport: { width: 390, height: 844 }, screen: { width: 390, height: 844 }, ignoreHTTPSErrors: true }
    : { ignoreHTTPSErrors: true, viewport: { width: 1440, height: 900 } }));
  contexts.push(c);
  c.on("page", (p) => {
    p.on("pageerror", (e) => console.log("PAGEERROR", e.message));
    p.on("response", async (r) => { if (r.url().includes("/api/buyer/") && r.status() >= 400) console.log("HTTP", r.status(), r.request().method(), new URL(r.url()).pathname, (await r.text().catch(() => "")).slice(0, 200)); });
  });
  return c;
}
async function signal(step) { await writeFile(path.join(evidence, `ready-${step}`), "1"); }
async function waitGo(step) {
  const file = path.join(evidence, `go-${step}`);
  for (let i = 0; i < 6000; i++) { try { await access(file); return; } catch { await pause(100); } }
  throw new Error(`the Go test never released ${step}`);
}

async function place(buyer) {
  const { ctx, locale, mobile, label } = buyer, c = copy[locale], viewport = mobile ? "mobile" : "desktop";
  const page = await ctx.newPage();
  await reachCheckout(page, origin, locale, product, { quantity: 2 }); // 2 units, whole-TWD subtotal
  await page.getByRole("button", { name: c.delivery, exact: true }).click();
  const quoted = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST");
  await page.getByRole("button", { name: c.quote, exact: true }).click();
  const quote = await (await quoted).json();
  assert.equal(quote.amount.subtotal_minor, total, `${label}: subtotal`);
  assert.equal(quote.amount.shipping_minor, 0, `${label}: the fixture home policy is zero-fee`);
  assert.equal(quote.amount.total_minor, total, `${label}: total`);
  await expect(page.getByTestId("address-section")).toBeVisible();
  for (const [key, value] of Object.entries(pii)) await page.locator(`input[name="${key}"]`).fill(value);
  // cash on delivery is offered only because the merchant enabled it: the mode row names the total plus the whole-TWD surcharge
  // (whole TWD amounts drop the decimals on every locale, so the label reads "TWD 25" / "NT$25", never "25.00")
  const fieldset = page.getByTestId("home-payment-mode");
  await expect(fieldset).toBeVisible();
  const codRow = fieldset.locator("label.sku-row", { has: page.locator('input[name="home_payment_mode"][value="cash_on_delivery"]') });
  await expect(codRow).toBeVisible();
  const codText = codRow.locator("span");
  await expect(codText).toContainText(c.cod);
  await expect(codText).toContainText(String(total / 100));
  await expect(codText).toContainText(String(surcharge / 100));
  await page.locator('input[name="home_payment_mode"][value="cash_on_delivery"]').check();
  await page.getByTestId("confirm-address").click();
  await expect(page.getByTestId("create-order")).toBeEnabled();
  await expect(page.getByTestId("create-order")).toHaveText(c.createCod);
  await expect(page.locator(".order-note").filter({ hasText: c.note })).toBeVisible(); // no online charge
  await shot(page, "checkout", locale, viewport);
  await page.getByTestId("create-order").click();
  await expect(page.getByTestId("order-section")).toBeVisible({ timeout: 30000 });
  const id = (await page.getByTestId("order-id").innerText()).trim();
  assert.match(id, /^[a-f0-9-]{36}$/);
  assert.equal(await page.getByTestId("pay-order").count(), 0, `${label}: a cash-on-delivery order has no card payment step`);
  const cod = page.getByTestId("order-cod");
  await expect(cod).toBeVisible({ timeout: 30000 });
  await expect(cod.getByTestId("order-cod-state")).toHaveAttribute("data-state", "PENDING");
  await shot(page, "order-pending", locale, viewport);
  pass(`${label} ${locale}/${viewport}: whole-TWD total ${total / 100}, surcharge ${surcharge / 100}, placed AWAITING_COLLECTION/PENDING`);
  return { ...buyer, page, id };
}
const api = (page, method, suffix) => page.evaluate(async ({ method, suffix }) => {
  const session = await (await fetch("/api/buyer/session", { cache: "no-store" })).json();
  const response = await fetch(`/api/buyer/${suffix}`, { method, headers: { "X-Buyer-Context": session.context } });
  return { status: response.status, body: await response.json() };
}, { method, suffix });
async function refresh(page) { await page.getByTestId("refresh-order").click(); }

try {
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path.join(certDir, "key.pem"), "-out", path.join(certDir, "cert.pem"), "-days", "1", "-subj", `/CN=${host}`], { stdio: "ignore" });
  const nextPort = await startNext();
  edge = https.createServer({ key: await readFile(path.join(certDir, "key.pem")), cert: await readFile(path.join(certDir, "cert.pem")) }, async (req, res) => {
    try {
      const chunks = []; for await (const x of req) chunks.push(x);
      const out = await relayTo({ hostname: "127.0.0.1", port: nextPort }, req, Buffer.concat(chunks));
      const headers = { ...out.headers }; delete headers.connection; delete headers["transfer-encoding"];
      res.writeHead(out.status, headers); res.end(out.body);
    } catch { if (!res.headersSent) res.writeHead(502); res.end(); }
  });
  const edgePort = await listen(edge);
  proxy = http.createServer((_, r) => { r.writeHead(403); r.end(); });
  proxy.on("connect", (req, socket, head) => {
    const [h, p] = req.url.split(":");
    if (h !== host || (p && p !== "443")) { socket.destroy(); return; }
    const upstream = net.connect(edgePort, "127.0.0.1", () => { socket.write("HTTP/1.1 200 Connection Established\r\n\r\n"); if (head.length) upstream.write(head); socket.pipe(upstream).pipe(socket); });
    for (const s of [socket, upstream]) { sockets.add(s); s.on("close", () => sockets.delete(s)); s.on("error", () => { socket.destroy(); upstream.destroy(); }); }
  });
  browser = await launch({ headless: true, proxy: { server: `http://127.0.0.1:${await listen(proxy)}` } });

  // ---- place -------------------------------------------------------------------------------------------------------------
  const plan = [
    { key: "A", locale: "zh-TW", mobile: false },
    { key: "B", locale: "en", mobile: false },
    { key: "C", locale: "zh-TW", mobile: true },
    { key: "D", locale: "en", mobile: true },
  ];
  const buyers = {};
  for (const p of plan) {
    const ctx = await newContext(p.mobile);
    buyers[p.key] = await place({ ...p, label: `order ${p.key}`, ctx });
  }
  await writeFile(path.join(evidence, "orders.json"), JSON.stringify(Object.fromEntries(Object.entries(buyers).map(([k, v]) => [k, { id: v.id, locale: v.locale, mobile: v.mobile }])), null, 2));
  await signal("placed"); await waitGo("1");

  // ---- after the merchant shipped + recorded collected ---------------------------------------------------------------
  {
    const a = buyers.A; await refresh(a.page);
    await expect(a.page.getByTestId("order-cod").getByTestId("order-cod-state")).toHaveAttribute("data-state", "COLLECTED");
    await expect(a.page.getByTestId("order-cod")).toContainText(copy[a.locale].paid);
    const order = await api(a.page, "GET", `orders/${a.id}`);
    assert.equal(order.status, 200); assert.equal(order.body.commercial_state, "AWAITING_COLLECTION", "a collected COD order never becomes CONFIRMED");
    assert.equal(order.body.collection_state, "COLLECTED");
    await shot(a.page, "order-collected", a.locale, "desktop");
    for (const k of ["B", "C", "D"]) {
      const b = buyers[k]; await refresh(b.page);
      await expect(b.page.getByTestId("order-cod").getByTestId("order-cod-state")).toHaveAttribute("data-state", "PENDING");
      const o = await api(b.page, "GET", `orders/${b.id}`);
      assert.equal(o.body.collection_state, "PENDING");
    }
    pass("order A: COLLECTED and still AWAITING_COLLECTION; B, C, D: still PENDING");
  }
  await signal("done");
} finally {
  for (const c of contexts) await c.close().catch(() => {});
  await browser?.close().catch(() => {});
  for (const s of sockets) s.destroy();
  proxy?.close(); edge?.close();
  for (const child of children) child.kill("SIGKILL");
  for (const log of logs) log.end();
}
