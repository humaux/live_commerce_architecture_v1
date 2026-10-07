// Purpose: buyer half of the W4-U1 gate: the platform-collector disclosure line (contract stripe-platform-account-v1 §5) on the real order payment page in zh-TW, zh-CN and en, and its absence when the hosted view returns no collector.
// Depends on: @playwright/test (via ./browser-engine.mjs), node http/https/net/crypto/child_process, openssl (throwaway TLS edge), apps/storefront next start; harness env: LC_CD_EVIDENCE, LC_CD_ORIGIN, LC_CD_ORDER, LC_CD_BUYER_TOKEN, LC_CD_EXPECT, LC_CD_PHASE, LC_CD_DISPLAY_NAME, LC_CD_PREVIEW, COMMERCE_BUYER_* (set by tests/foundation/browser_card_payments_test.go)
// Used by: tests/foundation/browser_card_payments_test.go (cpbrBuyerNode), scripts/dev/test-local.sh (--browser-card-payments)
// Same sealed-cookie / TLS-edge technique as refund-buyer.mjs: production storefront Next -> private buyerhttp -> isolated PG. Evidence class: BROWSER, MOCK Stripe.
// BFF -> Go: GET /api/buyer/orders/{id}/payment -> /v1/buyer/orders/{id}/payment (checkout.hosted_payment_view_v2, `collector` only for derived connections).
// LC_CD_EXPECT:
//   unpaid  — a payable order of an enrolled store (Stripe offered, "Pay" button): the disclosure sits ABOVE the pay button, 3 locales x desktop/mobile.
//   paid    — the same store's captured order: the disclosure is on the order payment page (no pay button), zh-TW + en.
//   primary — an order of a store on its own (primary) Stripe connection: the view carries no collector and nothing is rendered.
import assert from "node:assert/strict";
import { createHash, createHmac } from "node:crypto";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { readFile, writeFile, mkdtemp, rm } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { expect } from "@playwright/test";
import { launch, ctxOpts, phone } from "./browser-engine.mjs"; // LC_BROWSER_ENGINE=chromium|webkit

const env = (name) => {
  const value = process.env[name];
  assert(value, `${name} is required`);
  return value;
};
const root = process.cwd(), evidence = env("LC_CD_EVIDENCE"), origin = env("LC_CD_ORIGIN"), host = new URL(origin).host;
const orderID = env("LC_CD_ORDER"), buyerToken = env("LC_CD_BUYER_TOKEN"), expectation = env("LC_CD_EXPECT"), phase = env("LC_CD_PHASE");
const cookieKey = env("COMMERCE_BUYER_COOKIE_KEY");
assert(["unpaid", "paid", "primary"].includes(expectation));
const displayName = expectation === "primary" ? "" : env("LC_CD_DISPLAY_NAME"), preview = expectation === "primary" ? "" : env("LC_CD_PREVIEW");
const leak = /(?:acct_|sk_(?:live|test)_|rk_(?:live|test)_|pk_(?:live|test)_|whsec_)[A-Za-z0-9]/;
const esc = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
// contract §5, verbatim; {store_name} is whatever the shop is called, captured and compared across locales
const templates = {
  "zh-TW": (n, p) => new RegExp(`^本筆信用卡款項由 ${esc(n)} 代 (.+) 收取，信用卡帳單顯示「${esc(p)}」。$`),
  "zh-CN": (n, p) => new RegExp(`^本笔信用卡款项由 ${esc(n)} 代 (.+) 收取，信用卡账单显示「${esc(p)}」。$`),
  en: (n, p) => new RegExp(`^Card payment collected by ${esc(n)} on behalf of (.+)\\. Your card statement shows "${esc(p)}"\\.$`),
};
const children = new Set(), sockets = new Set(), contexts = [], logs = [];
const pause = (ms) => new Promise((r) => setTimeout(r, ms));
const listen = async (s) => { s.listen(0, "127.0.0.1"); await once(s, "listening"); return s.address().port; };
const certDir = await mkdtemp(path.join(tmpdir(), "lc-collector-edge-"));
let browser, edge, proxy;

function seal(token) {
  const iat = Math.floor(Date.now() / 1000);
  const payload = Buffer.from(JSON.stringify({ v: 1, token, iat, exp: iat + 3000, origin })).toString("base64url");
  return `${payload}.${createHmac("sha256", Buffer.from(cookieKey, "base64url")).update("buyer-cookie-v1").update("\0").update(payload).digest("base64url")}`;
}
function relay(port, req, body) {
  return new Promise((resolve, reject) => {
    const headers = { ...req.headers }; delete headers.connection; delete headers["transfer-encoding"];
    if (body.length) headers["content-length"] = String(body.length); else delete headers["content-length"];
    const call = http.request({ hostname: "127.0.0.1", port, path: req.url, method: req.method, headers }, (res) => {
      const chunks = []; res.on("data", (x) => chunks.push(x)); res.on("error", reject);
      res.on("end", () => resolve({ status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks) }));
    });
    call.setTimeout(15000, () => call.destroy(new Error("owned relay deadline"))); call.on("error", reject); call.end(body);
  });
}
async function startNext() {
  const reserve = net.createServer(), port = await listen(reserve); await new Promise((r) => reserve.close(r));
  const log = createWriteStream(path.join(evidence, `collector-next-${phase}.log`), { flags: "wx", mode: 0o600 }); logs.push(log); await once(log, "open");
  const childEnv = { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1" };
  for (const key of Object.keys(childEnv)) if (key.startsWith("LC_CD_")) delete childEnv[key];
  const child = spawn(process.execPath, [path.join(root, "apps/storefront/node_modules/next/dist/bin/next"), "start", "--hostname", "127.0.0.1", "--port", String(port)],
    { cwd: path.join(root, "apps/storefront"), env: childEnv, stdio: ["ignore", log, log] });
  children.add(child);
  for (let i = 0; i < 100; i++) {
    if (child.exitCode !== null) throw new Error("owned Next failed readiness");
    try { if ((await relay(port, { url: "/api/buyer/session", method: "GET", headers: { host } }, Buffer.alloc(0))).status === 200) return port; } catch { /* not ready */ }
    await pause(50);
  }
  throw new Error("owned Next readiness timeout");
}
async function context(mobile) {
  const c = await browser.newContext(ctxOpts(mobile
    ? { ...phone, viewport: { width: 390, height: 844 }, screen: { width: 390, height: 844 }, ignoreHTTPSErrors: true }
    : { ignoreHTTPSErrors: true, viewport: { width: 1440, height: 900 } }));
  contexts.push(c);
  await c.addCookies([{ name: "__Host-commerce_buyer", value: seal(buyerToken), url: origin, secure: true, httpOnly: true, sameSite: "Lax" }]);
  return c;
}
const manifest = path.join(evidence, "screenshots.json");
async function shot(page, name, locale, viewport) {
  const file = path.join(evidence, `${name}-${locale}-${viewport}.png`);
  await page.screenshot({ path: file, fullPage: true });
  // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `horizontal overflow at ${viewport} ${locale}`);
  let list = []; try { list = JSON.parse(await readFile(manifest, "utf8")); } catch { /* first */ }
  list.push({ File: path.basename(file), Sha256: createHash("sha256").update(await readFile(file)).digest("hex"), Locale: locale, Viewport: viewport });
  await writeFile(manifest, JSON.stringify(list, null, 2));
}
async function openOrder(page, locale) {
  await page.goto(`${origin}/${locale}/checkout`); // the order history lives on the checkout page (storefront shell)
  await page.getByTestId("toggle-order-history").click();
  await page.locator(`button[data-order-id="${orderID}"]`).click();
  await expect(page.getByTestId("order-id")).toHaveText(orderID);
  await page.getByTestId("payment-status").waitFor({ timeout: 15_000 }); // the payment block renders after its own fetch
}
// the buyer payment view through the storefront BFF (the same request the page makes)
async function paymentView(page) {
  // G-UI8 audit [READ/MEASURE]: same-origin GET read of server state through the BFF (no state change) (payment view)
  return page.evaluate(async (id) => {
    const session = await (await fetch("/api/buyer/session", { cache: "no-store" })).json();
    const response = await fetch(`/api/buyer/orders/${id}/payment`, { headers: { "X-Buyer-Context": session.context }, cache: "no-store" });
    return { status: response.status, body: await response.json() };
  }, orderID);
}
let storeName = null;
async function check(page, locale) {
  const view = await paymentView(page);
  assert.equal(view.status, 200);
  assert(!leak.test(JSON.stringify(view.body)), "the buyer payment view leaks an account id or key");
  assert(!leak.test(await page.content()), "the order page leaks an account id or key");
  if (expectation === "primary") {
    assert(!("collector" in view.body) || view.body.collector === null, "a primary connection must carry no collector");
    await expect(page.getByTestId("collector-disclosure")).toHaveCount(0);
    return;
  }
  assert.deepEqual(view.body.collector, { display_name: displayName, descriptor_preview: preview }, "the hosted view's collector");
  const line = page.getByTestId("collector-disclosure");
  await expect(line).toBeVisible();
  const text = (await line.innerText()).replace(/ /g, " ").trim();
  const match = templates[locale](displayName, preview).exec(text);
  assert(match, `${locale}: disclosure text does not match the contract §5 template: ${text}`);
  assert(match[1].trim() !== "" && !match[1].includes("{"), `${locale}: store name slot`);
  storeName ??= match[1];
  assert.equal(match[1], storeName, `${locale}: the store name must be the same shop name in every language`);
  if (expectation === "unpaid") {
    // above the Stripe pay button, in the same payment section
    await expect(page.getByTestId("pay-order")).toBeVisible();
    const order = await page.evaluate(() => {
      const section = document.querySelector('[data-testid="order-payment"]');
      const disclosure = section?.querySelector('[data-testid="collector-disclosure"]');
      const pay = section?.querySelector('[data-testid="pay-order"]');
      return !!disclosure && !!pay && !!(disclosure.compareDocumentPosition(pay) & Node.DOCUMENT_POSITION_FOLLOWING);
    });
    assert(order, `${locale}: the disclosure must precede the pay button inside the order payment section`);
  } else {
    await expect(page.getByTestId("pay-order")).toHaveCount(0);
  }
}
try {
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path.join(certDir, "key.pem"), "-out", path.join(certDir, "cert.pem"), "-days", "1", "-subj", `/CN=${new URL(origin).hostname}`], { stdio: "ignore" });
  const port = await startNext();
  edge = https.createServer({ key: await readFile(path.join(certDir, "key.pem")), cert: await readFile(path.join(certDir, "cert.pem")) }, async (req, res) => {
    try {
      const chunks = []; for await (const x of req) chunks.push(x);
      const out = await relay(port, req, Buffer.concat(chunks));
      const headers = { ...out.headers }; delete headers.connection; delete headers["transfer-encoding"];
      res.writeHead(out.status, headers); res.end(out.body);
    } catch { if (!res.headersSent) res.writeHead(502); res.end(); }
  });
  const edgePort = await listen(edge);
  proxy = http.createServer((_, r) => { r.writeHead(403); r.end(); });
  proxy.on("connect", (req, socket, head) => {
    if (req.url !== `${host}:443`) { socket.destroy(); return; }
    const upstream = net.connect(edgePort, "127.0.0.1", () => { socket.write("HTTP/1.1 200 Connection Established\r\n\r\n"); if (head.length) upstream.write(head); socket.pipe(upstream).pipe(socket); });
    for (const s of [socket, upstream]) { sockets.add(s); s.on("close", () => sockets.delete(s)); s.on("error", () => { socket.destroy(); upstream.destroy(); }); }
  });
  browser = await launch({ headless: true, proxy: { server: `http://127.0.0.1:${await listen(proxy)}` } });

  const locales = expectation === "unpaid" ? ["zh-TW", "zh-CN", "en"] : expectation === "paid" ? ["zh-TW", "en"] : ["en"];
  for (const locale of locales) {
    for (const mobile of expectation === "unpaid" ? [false, true] : [false]) {
      const c = await context(mobile);
      const page = await c.newPage();
      await openOrder(page, locale);
      await check(page, locale);
      await shot(page, `collector-${expectation}`, locale, mobile ? "mobile" : "desktop");
      await c.close();
    }
  }
  console.log(`PASS collector-buyer ${expectation}: ${locales.join(", ")}`);
} finally {
  for (const c of contexts) await c.close().catch(() => {});
  await browser?.close().catch(() => {});
  for (const s of sockets) s.destroy();
  for (const server of [proxy, edge]) if (server) await new Promise((r) => server.close(r));
  for (const child of children) child.kill("SIGKILL");
  for (const log of logs) log.end();
  await rm(certDir, { recursive: true, force: true });
}
