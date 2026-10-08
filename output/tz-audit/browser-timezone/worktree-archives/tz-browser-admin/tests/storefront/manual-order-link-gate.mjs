// Causal gate (storefront-v2 G3): the merchant creates an order in the admin UI -> copies the buyer link -> a FRESH browser opens it -> the storefront
// exchanges the fragment token for a buyer cookie, replaces history and lands on the order page -> bank details are visible -> the buyer submits the transfer
// proof (full capability) -> the same link in another fresh browser is refused (single use). Production Next builds of admin and storefront, signed MOCK IdP,
// real Go/PG; the only host mapping is this disposable TLS/CONNECT edge (https://buyer.example). Depends on Playwright,
// browser-engine and shared Taipei displayTime; called by the Go manual-order browser gate. The Go order expiry is the
// timestamp authority, and the buyer bank deadline is the durable reload surface. Evidence: result.json, screenshots.
import assert from "node:assert/strict";
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
import { launch, ctxOpts } from "./browser-engine.mjs"; // LC_BROWSER_ENGINE=chromium|webkit
import { displayTime } from "../../packages/format/src/index.ts";

const root = process.cwd(), evidence = process.env.LC_LINK_EVIDENCE;
const adminOrigin = process.env.COMMERCE_PUBLIC_ORIGIN, buyerOrigin = "https://buyer.example";
assert(evidence && /^https?:\/\/127\.0\.0\.1:\d+$/.test(adminOrigin));
const certDir = await mkdtemp(path.join(tmpdir(), "lc-manual-link-edge-"));
const children = new Set(), sockets = new Set(), logs = [];
const running = child => child.exitCode === null && child.signalCode === null;
let browser, edge, proxy, cases = 0;
const pass = name => { cases++; console.log(`PASS ${name}`); };
const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
const listen = async server => { server.listen(0, "127.0.0.1"); await once(server, "listening"); return server.address().port; };
function relay(port, request, body = Buffer.alloc(0)) {
  return new Promise((resolve, reject) => {
    const headers = {...request.headers};
    delete headers.connection; delete headers["transfer-encoding"];
    if (body.length) headers["content-length"] = String(body.length); else delete headers["content-length"];
    const call = http.request({hostname: "127.0.0.1", port, path: request.url, method: request.method, headers}, response => {
      const chunks = [];
      response.on("data", chunk => chunks.push(chunk));
      response.on("error", reject);
      response.on("end", () => resolve({status: response.statusCode, headers: response.headers, body: Buffer.concat(chunks)}));
    });
    call.setTimeout(15000, () => call.destroy(new Error("fixture relay timeout")));
    call.on("error", reject); call.end(body);
  });
}
async function startNext(app, port) {
  if (!port) { const reserve = net.createServer(); port = await listen(reserve); await new Promise(resolve => reserve.close(resolve)); }
  const log = createWriteStream(path.join(evidence, `${app}.log`), {flags: "wx", mode: 0o600});
  logs.push(log); await once(log, "open");
  const env = {...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1"};
  for (const name of Object.keys(env)) if (name.startsWith("LC_LINK_")) delete env[name];
  const args = app === "admin"
    ? [path.join(root, "apps/admin/.next/standalone/apps/admin/server.js")]
    : [path.join(root, "apps/storefront/node_modules/next/dist/bin/next"), "start", "--hostname", "127.0.0.1", "--port", String(port)];
  env.HOSTNAME = "127.0.0.1"; env.PORT = String(port);
  const child = spawn(process.execPath, args, {cwd: path.join(root, `apps/${app}`), env, stdio: ["ignore", log, log]});
  children.add(child);
  for (let i = 0; i < 100; i++) {
    if (!running(child)) throw new Error(`owned ${app} exited before readiness`);
    try {
      const response = await relay(port, {url: app === "admin" ? "/api/stores" : "/api/buyer/session", method: "GET", headers: {host: app === "admin" ? new URL(adminOrigin).host : "buyer.example"}});
      if (response.status === (app === "admin" ? 401 : 200)) return port;
    } catch {}
    await wait(50);
  }
  throw new Error(`owned ${app} readiness deadline`);
}
try {
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path.join(certDir, "key.pem"), "-out", path.join(certDir, "cert.pem"), "-days", "1", "-subj", "/CN=buyer.example"], {stdio: "ignore"});
  const [, buyerPort] = await Promise.all([startNext("admin", Number(process.env.LC_LINK_ADMIN_PORT)), startNext("storefront")]);
  edge = https.createServer({key: await readFile(path.join(certDir, "key.pem")), cert: await readFile(path.join(certDir, "cert.pem"))}, async (request, response) => {
    try {
      const chunks = []; for await (const chunk of request) chunks.push(chunk);
      const out = await relay(buyerPort, request, Buffer.concat(chunks));
      const headers = {...out.headers}; delete headers.connection; delete headers["transfer-encoding"];
      response.writeHead(out.status, headers); response.end(out.body);
    } catch { response.writeHead(502); response.end(); }
  });
  const edgePort = await listen(edge);
  proxy = http.createServer((_, response) => { response.writeHead(403); response.end(); });
  proxy.on("connect", (request, socket, head) => {
    if (request.url !== "buyer.example:443") { socket.destroy(); return; }
    const upstream = net.connect(edgePort, "127.0.0.1", () => {
      socket.write("HTTP/1.1 200 Connection Established\r\n\r\n");
      if (head.length) upstream.write(head);
      socket.pipe(upstream).pipe(socket);
    });
    for (const connection of [socket, upstream]) {
      sockets.add(connection); connection.on("close", () => sockets.delete(connection));
      connection.on("error", () => { socket.destroy(); upstream.destroy(); });
    }
  });
  const proxyPort = await listen(proxy);
  browser = await launch({headless: true, proxy: {server: `http://127.0.0.1:${proxyPort}`, bypass: "127.0.0.1"}});
  const uiErrors = [];

  // ---- merchant: sign in (signed MOCK IdP), create the manual order in the admin UI, copy the link ------------------------------------
  const merchantContext = await browser.newContext(ctxOpts({ignoreHTTPSErrors: true, viewport: {width: 1100, height: 900}, timezoneId: "America/Los_Angeles"}));
  const merchant = await merchantContext.newPage();
  assert.equal(await merchant.evaluate(() => Intl.DateTimeFormat().resolvedOptions().timeZone), "America/Los_Angeles");
  merchant.on("pageerror", error => uiErrors.push(error.name));
  await merchant.goto(`${adminOrigin}/en`);
  await merchant.getByRole("button", {name: "Sign in with identity service", exact: true}).click();
  await expect(merchant.getByTestId("dashboard-page")).toBeVisible(); // the landing is the dashboard now
  pass("signed MOCK IdP sign-in lands on the dashboard");
  await merchant.getByTestId("action-create-order").click();
  await expect(merchant.getByTestId("manual-order-page")).toBeVisible();
  await merchant.getByTestId("mo-search").fill(process.env.LC_LINK_PRODUCT_NAME);
  await merchant.getByTestId("mo-search-button").click();
  const result = merchant.locator(".mt-results li").first();
  await result.getByRole("button", {name: "Add", exact: true}).first().click(); // expands the variants
  await merchant.locator(".mt-variants li").first().getByRole("button", {name: "Add", exact: true}).click();
  await expect(merchant.getByTestId("mo-lines").locator("li")).toHaveCount(1);
  await merchant.getByTestId("mo-name").fill("Browser Buyer");
  await merchant.getByTestId("mo-phone").fill("0912345678");
  const home = await merchant.getByTestId("mo-option").locator("option", {hasText: "Home delivery"}).first().getAttribute("value");
  await merchant.getByTestId("mo-option").selectOption(home);
  await merchant.getByTestId("mo-city").fill("Taipei");
  await merchant.getByTestId("mo-line1").fill("1 Browser Road");
  await merchant.getByTestId("mo-mode-bank_transfer").check();
  const placed = merchant.waitForResponse(r => r.request().method() === "POST" && new URL(r.url()).pathname === `/api/stores/${process.env.LC_LINK_STORE}/tools/orders/manual`);
  await merchant.getByTestId("manual-order-submit").click();
  const reply = await placed;
  assert.equal(reply.status(), 201);
  const body = await reply.json();
  assert.equal(body.commercial_state, "AWAITING_TRANSFER");
  assert.equal(body.source, "merchant_manual");
  assert(Number.isFinite(Date.parse(body.expires_at)), "the Go manual-order receipt supplies an order expiry instant");
  await expect(merchant.getByTestId("manual-order-result")).toBeVisible();
  await expect(merchant.getByTestId("manual-order-result").locator(".mt-stats dd").nth(2)).toHaveText(displayTime("en", body.expires_at));
  const link = (await merchant.getByTestId("manual-order-link").innerText()).trim();
  assert.equal(link, body.buyer_link);
  const fragment = new URL(link).hash;
  const token = /&t=([A-Za-z0-9_-]{43})$/.exec(fragment)?.[1];
  assert(token, "link carries the token only in the fragment");
  assert.equal(new URL(link).search, "");
  await merchant.screenshot({path: path.join(evidence, "merchant-created.png"), fullPage: true});
  pass("merchant created a manual bank-transfer order in the admin UI and got the buyer link");

  // ---- buyer: a FRESH browser opens the link ------------------------------------------------------------------------------------------
  const buyerContext = await browser.newContext(ctxOpts({ignoreHTTPSErrors: true, viewport: {width: 390, height: 844}, timezoneId: "America/Los_Angeles"}));
  const buyer = await buyerContext.newPage();
  assert.equal(await buyer.evaluate(() => Intl.DateTimeFormat().resolvedOptions().timeZone), "America/Los_Angeles");
  buyer.on("pageerror", error => uiErrors.push(error.name));
  const leaked = [], exchanges = [];
  buyer.on("request", request => {
    if (request.url().includes(token) || (request.headers()["referer"] ?? "").includes(token)) leaked.push(request.url());
    if (request.method() === "POST" && new URL(request.url()).pathname === "/api/buyer/orders/link") exchanges.push(request.postData());
  });
  await buyer.goto(link);
  await buyer.waitForURL(url => url.pathname === `/zh-TW/orders/${body.order_id}`);
  assert.equal(new URL(buyer.url()).hash, "", "the fragment is gone from the address bar");
  assert.equal(await buyer.evaluate(() => location.href.includes("t=")), false);
  assert.deepEqual(leaked, [], "the link token never appears in any request URL or Referer");
  assert.equal(exchanges.length, 1, "the token is posted exactly once");
  const posted = JSON.parse(exchanges[0]);
  assert.deepEqual(Object.keys(posted).sort(), ["order_id", "proof", "token"], "the POST carries the browser-bound proof");
  assert.equal(posted.order_id, body.order_id);
  assert.equal(posted.token, token);
  assert.match(posted.proof, /^[A-Za-z0-9_-]{43}$/, "the proof is a 43-character secret");
  const cookies = await buyerContext.cookies();
  const session = cookies.find(c => c.name === "__Host-commerce_buyer");
  assert(session && session.httpOnly && session.secure, "buyer cookie set HttpOnly+Secure by the BFF");
  assert.equal(session.value.includes(token), false, "the cookie is a fresh capability, not the link token");
  pass("fresh browser: fragment read client-side, posted once, history replaced, redirected to the order page, cookie set");
  await expect(buyer.getByTestId("transfer-bank")).toBeVisible();
  await expect(buyer.getByTestId("transfer-account-number")).toHaveText(process.env.LC_LINK_ACCOUNT);
  await expect(buyer.getByTestId("transfer-state")).toBeVisible();
  await expect(buyer.getByTestId("transfer-deadline")).toContainText(displayTime("zh-TW", body.expires_at));
  await expect(buyer.getByTestId("transfer-deadline")).toContainText("UTC+8");
  await buyer.reload();
  await expect(buyer.getByTestId("transfer-bank")).toBeVisible();
  await expect(buyer.getByTestId("transfer-deadline")).toContainText(displayTime("zh-TW", body.expires_at));
  await buyer.screenshot({path: path.join(evidence, "buyer-order-bank-details.png"), fullPage: true});
  pass("the buyer sees the order with the bank details");
  await buyer.locator('input[name="last5"]').fill("12345");
  await buyer.locator('input[name="amount"]').fill(process.env.LC_LINK_AMOUNT);
  const paidAtInBrowserZone = await buyer.evaluate(() => {
    const d = new Date(Date.now() - 60_000);
    const pad = (n) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }); // read-only: the input is interpreted by the page in its own timezone
  await buyer.locator('input[name="paid_at"]').fill(paidAtInBrowserZone);
  const proofReply = buyer.waitForResponse(r => r.request().method() === "PUT" && new URL(r.url()).pathname === `/api/buyer/orders/${body.order_id}/bank-transfer/proof`);
  await buyer.getByTestId("transfer-send").click();
  assert.equal((await proofReply).status(), 200);
  await expect(buyer.getByTestId("transfer-proof")).toBeVisible();
  await buyer.reload();
  await expect(buyer.getByTestId("transfer-deadline")).toContainText(displayTime("zh-TW", body.expires_at));
  await buyer.screenshot({path: path.join(evidence, "buyer-proof-submitted.png"), fullPage: true});
  pass("the buyer submitted the transfer proof with the exchanged (full) capability");

  // ---- the same link in another fresh browser is the one identical refusal -----------------------------------------------------------
  const otherContext = await browser.newContext(ctxOpts({ignoreHTTPSErrors: true, viewport: {width: 390, height: 844}, timezoneId: "America/Los_Angeles"}));
  const other = await otherContext.newPage();
  other.on("pageerror", error => uiErrors.push(error.name));
  await other.goto(link);
  await expect(other.getByTestId("order-link-refused")).toBeVisible();
  assert.equal(new URL(other.url()).hash, "");
  assert.equal(new URL(other.url()).pathname, "/zh-TW/order-link");
  assert.equal((await otherContext.cookies()).some(c => c.name === "__Host-commerce_buyer"), false, "a refused link sets no cookie");
  // a malformed fragment is the very same page
  await other.goto(`${buyerOrigin}/zh-TW/order-link#o=${body.order_id}&t=short`);
  await expect(other.getByTestId("order-link-refused")).toBeVisible();
  await other.screenshot({path: path.join(evidence, "link-refused.png"), fullPage: true});
  pass("a used or malformed link is the same refusal page and sets no cookie");

  // ---- regenerate: the merchant re-issues the link; the NEW link opens the order in a fresh browser, the OLD one stays refused -------------
  const regenReply = merchant.waitForResponse(r => r.request().method() === "POST" && new URL(r.url()).pathname === `/api/stores/${process.env.LC_LINK_STORE}/tools/orders/manual/regenerate-link`);
  await merchant.getByTestId("manual-order-regenerate").click();
  assert.equal((await regenReply).status(), 201);
  const newLink = (await merchant.getByTestId("manual-order-link").innerText()).trim();
  assert.notEqual(newLink, link, "regenerate issues a different link");
  const newFragment = new URL(newLink).hash;
  const newToken = /&t=([A-Za-z0-9_-]{43})$/.exec(newFragment)?.[1];
  assert(newToken && newToken !== token, "the regenerated link carries a fresh token");
  await merchant.screenshot({path: path.join(evidence, "merchant-regenerated.png"), fullPage: true});
  // the OLD link, still in a fresh browser, is refused (it is used AND now invalidated)
  await other.goto(link);
  await expect(other.getByTestId("order-link-refused")).toBeVisible();
  // the NEW link opens the order in another fresh browser
  const thirdContext = await browser.newContext(ctxOpts({ignoreHTTPSErrors: true, viewport: {width: 390, height: 844}, timezoneId: "America/Los_Angeles"}));
  const third = await thirdContext.newPage();
  third.on("pageerror", error => uiErrors.push(error.name));
  await third.goto(newLink);
  await third.waitForURL(url => url.pathname === `/zh-TW/orders/${body.order_id}`);
  await expect(third.getByTestId("transfer-bank")).toBeVisible();
  await third.screenshot({path: path.join(evidence, "buyer-regenerated-link.png"), fullPage: true});
  pass("regenerate issues a working link and the old link stays refused");

  // ---- home-cod: the merchant creates a CASH-ON-DELIVERY order by real clicks (G-UI8 defect D01) ---------------------------------------------
  // The store offers COD (NT$50 collection fee, NT$20,000 cap), so the options list carries "cash_on_delivery" on the home row. The page used to
  // refuse that one unknown mode and show "creating orders is not turned on" instead of the whole form.
  await merchant.getByTestId("manual-order-another").click();
  await expect(merchant.getByTestId("manual-order-form")).toBeVisible();
  await merchant.getByTestId("mo-search").fill(process.env.LC_LINK_PRODUCT_NAME);
  await merchant.getByTestId("mo-search-button").click();
  await merchant.locator(".mt-results li").first().getByRole("button", {name: "Add", exact: true}).first().click();
  await merchant.locator(".mt-variants li").first().getByRole("button", {name: "Add", exact: true}).click();
  await merchant.getByTestId("mo-lines").getByRole("spinbutton").fill("2"); // 2 x 12.50 = 25.00: a COD total must be whole dollars
  await merchant.getByTestId("mo-name").fill("Browser COD Buyer");
  await merchant.getByTestId("mo-phone").fill("0912345678");
  await merchant.getByTestId("mo-option").selectOption(home);
  await expect(merchant.getByTestId("mo-mode-cash_on_delivery")).toBeVisible();
  await expect(merchant.getByTestId("mo-mode-bank_transfer")).toBeVisible();
  await merchant.getByTestId("mo-city").fill("Taipei");
  await merchant.getByTestId("mo-line1").fill("2 Browser Road");
  await merchant.getByTestId("mo-mode-cash_on_delivery").check();
  await expect(merchant.getByTestId("mo-cod-note")).toContainText("NT$50"); // the collection fee the carrier adds at the door
  await expect(merchant.getByTestId("mo-cod-note")).toContainText("NT$20,000"); // and the per-order cap, both whole NT$
  const codPlaced = merchant.waitForResponse(r => r.request().method() === "POST" && new URL(r.url()).pathname === `/api/stores/${process.env.LC_LINK_STORE}/tools/orders/manual`);
  await merchant.getByTestId("manual-order-submit").click();
  const codReply = await codPlaced;
  assert.equal(codReply.status(), 201);
  const codBody = await codReply.json();
  assert.equal(codBody.commercial_state, "AWAITING_COLLECTION");
  assert.equal(codBody.payment_mode, "cash_on_delivery");
  assert.equal(codBody.total_minor, 2500);
  assert.equal(codBody.source, "merchant_manual");
  await expect(merchant.getByTestId("manual-order-result")).toBeVisible();
  await expect(merchant.getByTestId("manual-order-cod-due")).toContainText("NT$75"); // total NT$25 + fee NT$50: what the carrier collects
  await expect(merchant.getByTestId("manual-order-result")).toContainText("Waiting for cash on delivery");
  const codLink = (await merchant.getByTestId("manual-order-link").innerText()).trim();
  assert.equal(codLink, codBody.buyer_link);
  await merchant.screenshot({path: path.join(evidence, "merchant-created-cod.png"), fullPage: true});
  pass("merchant created a manual cash-on-delivery order in the admin UI (whole-NT$ fee and cap shown, cash due NT$75) and got the buyer link");

  const codBuyerContext = await browser.newContext(ctxOpts({ignoreHTTPSErrors: true, viewport: {width: 390, height: 844}, timezoneId: "America/Los_Angeles"}));
  const codBuyer = await codBuyerContext.newPage();
  codBuyer.on("pageerror", error => uiErrors.push(error.name));
  await codBuyer.goto(codLink);
  await codBuyer.waitForURL(url => url.pathname === `/zh-TW/orders/${codBody.order_id}`);
  await expect(codBuyer.getByTestId("order-cod")).toBeVisible();
  await expect(codBuyer.getByTestId("order-cod-amount")).toContainText("NT$75");
  await expect(codBuyer.getByTestId("order-cod-state")).toHaveAttribute("data-state", "PENDING");
  await expect(codBuyer.getByTestId("transfer-bank")).toHaveCount(0); // a COD order has no bank-transfer panel
  await codBuyer.screenshot({path: path.join(evidence, "buyer-order-cod.png"), fullPage: true});
  pass("the buyer opens the COD link and sees the cash due on delivery (NT$75) waiting for the carrier");

  assert.deepEqual(uiErrors, []);
  pass("no page errors");
  await writeFile(path.join(evidence, "result.json"), JSON.stringify({cases, order_id: body.order_id, cod_order_id: codBody.order_id, link_locale: "zh-TW", boundary: "production Next; signed MOCK IdP; synthetic local TLS/CONNECT; no provider or deployment DNS/TLS acceptance"}, null, 2), {mode: 0o600});
} catch (error) {
  if (browser) for (const context of browser.contexts()) for (const [index, page] of context.pages().entries()) {
    await page.screenshot({path: path.join(evidence, `failure-${context.pages().length}-${index}.png`), fullPage: true}).catch(() => {});
  }
  throw error;
} finally {
  if (browser) await browser.close();
  for (const socket of sockets) socket.destroy();
  for (const server of [proxy, edge]) if (server) await new Promise(resolve => server.close(resolve));
  for (const child of children) { if (running(child)) child.kill("SIGKILL"); }
  for (const child of children) if (running(child)) await once(child, "exit");
  for (const log of logs) log.end();
  await rm(certDir, {recursive: true, force: true});
}
