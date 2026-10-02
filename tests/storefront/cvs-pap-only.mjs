// ops-polish OP1 buyer half (docs/delivery/units/ops-polish.md OP1): the pilot configuration with no card payment (COMMERCE_BUYER_PAYMENT_ENABLED off) in real
// browsers. Real production storefront Next -> private buyerhttp over checkout.Service.WithoutCardPayment (the switch cmd/api turns on) -> isolated PG,
// driven by tests/foundation/browser_ops_polish_test.go (TestBrowserOpsPolishStorefront). One synthetic host behind one CONNECT proxy and one self-signed
// edge, as in tests/storefront/cvs-buyer.mjs. Store: buyer_entered mode (no ECPay profile), 7-ELEVEN enabled, pay-at-pickup enabled, no home delivery
// offered. For each of zh-TW / en on Chromium desktop and a 390px phone: the delivery list offers the chain, the payment mode is PRE-SELECTED to pay at
// pickup, there is no card radio, and the order is placed (PENDING collection, no card payment step). Evidence: screenshots (hashed).
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { readFile, writeFile, mkdtemp } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { expect } from "@playwright/test";
import { engine, launch, ctxOpts, phone } from "./browser-engine.mjs";

const env = (name) => {
  const value = process.env[name];
  assert(value, `${name} is required`);
  return value;
};
const root = process.cwd(), evidence = env("LC_OPP_EVIDENCE");
const store = { origin: env("LC_OPP_ORIGIN"), product: env("LC_OPP_PRODUCT") };
const host = new URL(store.origin).host;
const children = new Set(), sockets = new Set(), contexts = [], logs = [];
const pause = (ms) => new Promise((r) => setTimeout(r, ms));
const listen = async (s) => { s.listen(0, "127.0.0.1"); await once(s, "listening"); return s.address().port; };
const certDir = await mkdtemp(path.join(tmpdir(), "lc-opp-edge-"));
const pass = (name) => console.log(`PASS ${name}`);
let browser, edge, proxy;

const copy = {
  "zh-TW": { more: "增加數量", delivery: "選擇配送", quote: "取得目前總額", chain: "7-ELEVEN", payAtPickup: /取貨付款/, noCharge: /不會線上扣款/, card: /信用卡/ },
  en: { more: "Increase quantity", delivery: "Choose delivery", quote: "Get current total", chain: "7-ELEVEN", payAtPickup: /pay-at-pickup/i, noCharge: /No card is charged online/i, card: /Credit card|\bcard\b/i },
};

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
  const log = createWriteStream(path.join(evidence, "opp-next.log"), { flags: "wx", mode: 0o600 }); logs.push(log); await once(log, "open");
  const childEnv = { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1" };
  for (const key of Object.keys(childEnv)) if (key.startsWith("LC_OPP_")) delete childEnv[key];
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
  const file = path.join(evidence, `opp-${name}-${locale}-${viewport}.png`);
  await page.screenshot({ path: file, fullPage: true });
  // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `horizontal overflow at ${viewport} ${locale}`);
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

try {
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path.join(certDir, "key.pem"), "-out", path.join(certDir, "cert.pem"), "-days", "1", "-subj", "/CN=opp.example"], { stdio: "ignore" });
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
    if (!(p === "443" && h === host)) { socket.destroy(); return; }
    const upstream = net.connect(edgePort, "127.0.0.1", () => { socket.write("HTTP/1.1 200 Connection Established\r\n\r\n"); if (head.length) upstream.write(head); socket.pipe(upstream).pipe(socket); });
    for (const s of [socket, upstream]) { sockets.add(s); s.on("close", () => sockets.delete(s)); s.on("error", () => { socket.destroy(); upstream.destroy(); }); }
  });
  browser = await launch({ headless: true, proxy: { server: `http://127.0.0.1:${await listen(proxy)}` } });

  for (const [locale, mobile] of [["zh-TW", false], ["zh-TW", true], ["en", false], ["en", true]]) {
    const viewport = mobile ? "mobile" : "desktop";
    const ctx = await newContext(mobile), page = await ctx.newPage();
    await page.goto(`${store.origin}/${locale}/products/${store.product}`);
    // a pay-at-pickup total must be a whole TWD amount (F20); the fixture SKU costs TWD 12.50, so two units.
    // 0093 (storefront-integration): the product page only adds to the cart; delivery is chosen on /{locale}/checkout.
    await page.getByRole("button", { name: copy[locale].more, exact: true }).click();
    await expect(page.getByTestId("add-to-cart")).toBeEnabled();
    await page.getByTestId("add-to-cart").click();
    await expect(page.getByTestId("cart-drawer")).toBeVisible();
    await page.goto(`${store.origin}/${locale}/cart`);
    await expect(page.getByTestId("cart-line")).toHaveCount(1);
    await expect(page.getByTestId("cart-line-qty")).toHaveText("2");
    await page.getByTestId("cart-checkout").click();
    await page.waitForURL(`**/${locale}/checkout`);
    await page.getByRole("button", { name: copy[locale].delivery, exact: true }).click();
    // the delivery list: the CVS chain is offered; the card-only home option is not (nothing the buyer cannot pay for)
    await expect(page.locator("#delivery option", { hasText: copy[locale].chain })).toHaveCount(1); // options load asynchronously
    const options = await page.locator("#delivery option").allInnerTexts();
    assert(options.some((o) => o.includes(copy[locale].chain)), `${locale}/${viewport}: the 7-ELEVEN option must be offered, got ${JSON.stringify(options)}`);
    assert(!options.some((o) => /home|宅配|配送上门|Mock delivery|测试配送|測試配送/i.test(o)), `${locale}/${viewport}: a card-only home option is offered: ${JSON.stringify(options)}`);
    await page.locator("#delivery").selectOption({ label: `${copy[locale].chain} · TW` });
    await page.getByRole("button", { name: copy[locale].quote, exact: true }).click();
    await expect(page.getByTestId("address-section")).toBeVisible();
    const codUnavailable = page.locator(".cod-home-only");
    await expect(codUnavailable.getByRole("radio")).toBeDisabled();
    await expect(codUnavailable).toContainText(locale === "en" ? "Home delivery only" : "僅限宅配");
    const captureViewport = page.viewportSize();
    await page.setViewportSize({ width: mobile ? 390 : 1586, height: mobile ? 844 : 992 });
    await codUnavailable.scrollIntoViewIfNeeded();
    const disabledCapture = await page.screenshot({ path: path.join(root, "output/home-cod-ui", `buyer-cvs-cod-disabled-${locale}-${mobile ? 390 : 1586}.png`), fullPage: false, animations: "disabled", scale: "css" });
    assert.equal(disabledCapture.readUInt32BE(16), mobile ? 390 : 1586);
    assert.equal(disabledCapture.readUInt32BE(20), mobile ? 844 : 992);
    await page.setViewportSize(captureViewport);
    // the payment mode is already pay-at-pickup: there is no card radio to flip back to, the create button already reads pay at pickup, and the
    // note beside it says no card is charged online. A single offered mode may need no radio at all; if one is drawn it must be the checked one.
    assert.equal(await page.getByRole("radio", { name: copy[locale].card }).count(), 0, `${locale}/${viewport}: a card radio is offered`);
    for (const radio of await page.getByTestId("cvs-payment-mode").getByRole("radio").all()) await expect(radio).toBeChecked();
    await expect(page.getByTestId("create-order")).toHaveText(copy[locale].payAtPickup);
    await expect(page.locator(".order-note").filter({ hasText: copy[locale].noCharge })).toBeVisible();
    // place the order without touching the payment control
    await page.getByTestId("cvs-recipient-name").fill("王小明");
    await page.getByTestId("cvs-recipient-phone").fill("0912345678");
    await page.getByTestId("cvs-entered-code").fill("123456");
    await page.getByTestId("cvs-entered-name").fill("測試門市");
    await page.getByTestId("cvs-entered-address").fill("台北市測試路1號");
    await page.getByTestId("confirm-address").click();
    await expect(page.getByTestId("create-order")).toBeEnabled();
    await shot(page, "pap-only-before", locale, viewport);
    await page.getByTestId("create-order").click();
    try { await expect(page.getByTestId("order-section")).toBeVisible({ timeout: 30000 }); } catch (e) {
      console.log("ALERTS", JSON.stringify(await page.locator("[role=alert], .purchase-error, [data-testid=cvs-create-error]").allInnerTexts()));
      await shot(page, "pap-only-failed", locale, viewport); throw e;
    }
    await expect(page.getByTestId("order-collection")).toHaveAttribute("data-state", "PENDING");
    assert.equal(await page.getByTestId("pay-order").count(), 0, "a pay-at-pickup order has no card payment step");
    await shot(page, "pap-only-order", locale, viewport);
    pass(`pay-at-pickup-only ${locale}/${viewport}: chain offered, no home option, no card radio, pay at pickup pre-selected, order placed`);
    await ctx.close();
  }
} finally {
  for (const c of contexts) await c.close().catch(() => {});
  await browser?.close().catch(() => {});
  for (const s of sockets) s.destroy();
  proxy?.close(); edge?.close();
  for (const child of children) child.kill("SIGKILL");
  for (const log of logs) log.end();
}
