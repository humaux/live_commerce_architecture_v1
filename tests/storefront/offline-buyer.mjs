// COB buyer half (contracts/storefront-v2.md §C, unit checkout-offline; R4 independent browser gate). Real production storefront Next -> private
// buyerhttp -> isolated PG, driven by tests/foundation/browser_checkout_offline_test.go (MOCK: there is no PSP on this path).
// BFF routes exercised: /api/buyer/{checkout-options,quotes,destination,checkout,orders/{id},orders/{id}/bank-transfer,.../bank-transfer/proof}.
// One long-lived process that hands over to the merchant half through files in LC_OFF_EVIDENCE: it writes ready-<step>, the Go test runs the
// admin Playwright phase, and touches go-<step>. The buyer contexts (cookies, locators) therefore survive every merchant action.
//   place   four buyers (zh-TW desktop, en desktop, zh-TW 390px, en 390px) check out HOME delivery with bank transfer at the free-shipping
//           boundary, see the shop's bank details + countdown, submit last-5 + amount + time; a fifth buyer places an order and does not pay
//   go-1    merchant rejects order A's submission and confirms B, C, D
//   after1  A sees the rejection reason and resubmits; B, C, D see CONFIRMED (and an order read says CONFIRMED); E still waits
//   go-2    merchant confirms A
//   after2  A sees CONFIRMED
//   go-3    the test ages E's window out and runs the expiry worker's function (the clock is controlled, nothing waits hours)
//   after3  E sees EXPIRED with the bank details gone; A-D stay CONFIRMED
// Wording: the zh-TW and en strings of apps/storefront/lib/bank-transfer-copy.ts that carry a contract meaning (confirmed by the shop, expired,
// the countdown); everything else is located by data-testid.
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
const root = process.cwd(), evidence = env("LC_OFF_EVIDENCE"), origin = env("LC_OFF_ORIGIN"), product = env("LC_OFF_PRODUCT");
const account = env("LC_OFF_ACCOUNT"), bankName = env("LC_OFF_BANK"), unit = Number(env("LC_OFF_PRICE")), reason = env("LC_OFF_REASON");
const fee = Number(env("LC_OFF_FEE")), price = unit * 2; // every order is 2 units: its subtotal is exactly the merchant's free-shipping threshold
const host = new URL(origin).host;
const children = new Set(), sockets = new Set(), contexts = [], logs = [];
const pause = (ms) => new Promise((r) => setTimeout(r, ms));
const listen = async (s) => { s.listen(0, "127.0.0.1"); await once(s, "listening"); return s.address().port; };
const certDir = await mkdtemp(path.join(tmpdir(), "lc-off-edge-"));
const pass = (name) => console.log(`PASS ${name}`);
let browser, edge, proxy;

const copy = {
  "zh-TW": {
    delivery: "選擇配送", quote: "取得目前總額", more: "增加數量", bank: "銀行轉帳", create: "送出訂單（銀行轉帳）", hoursLeft: /還剩 [56] 小時 \d+ 分鐘/,
    confirmed: "商家已確認收款", expired: "轉帳期限已結束，訂單已取消，庫存已釋出。", note: /6 小時/,
    headConfirmed: "訂單已確認", headCancelled: "訂單已取消", headWaiting: "等待銀行轉帳",
  },
  en: {
    delivery: "Choose delivery", quote: "Get current total", more: "Increase quantity", bank: "Bank transfer", create: "Place order (bank transfer)", hoursLeft: /[56] h \d+ min left/,
    confirmed: "The shop confirmed your payment", expired: "The transfer window ended. This order was cancelled and the items were released.", note: /within 6 hours/,
    headConfirmed: "Order confirmed", headCancelled: "Order canceled", headWaiting: "Waiting for bank transfer",
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
  const log = createWriteStream(path.join(evidence, "offline-next.log"), { flags: "wx", mode: 0o600 }); logs.push(log); await once(log, "open");
  const childEnv = { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1" };
  for (const key of Object.keys(childEnv)) if (key.startsWith("LC_OFF_")) delete childEnv[key];
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
  const file = path.join(evidence, `offline-${name}-${locale}-${viewport}.png`);
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
const money = (minor) => (minor / 100).toFixed(2);
const localInput = (minutesAgo) => { // datetime-local value in the browser's zone (the contexts run in the host zone)
  const d = new Date(Date.now() - minutesAgo * 60000), p = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
};
async function signal(step) { await writeFile(path.join(evidence, `ready-${step}`), "1"); }
async function waitGo(step) {
  const file = path.join(evidence, `go-${step}`);
  for (let i = 0; i < 6000; i++) { try { await access(file); return; } catch { await pause(100); } }
  throw new Error(`the Go test never released ${step}`);
}
const view = (page) => page.getByTestId("bank-transfer");

async function send(page, last5, minor) {
  await page.locator('input[name="last5"]').fill(last5);
  await page.locator('input[name="amount"]').fill(money(minor));
  await page.locator('input[name="paid_at"]').fill(localInput(10));
  await page.getByTestId("transfer-send").click();
  await expect(view(page)).toHaveAttribute("data-state", "SUBMITTED");
  await expect(page.getByTestId("transfer-proof")).toContainText(last5);
}
// Product page -> home delivery quote at the free-shipping boundary -> address -> bank transfer -> order page with the bank details and countdown.
async function belowThreshold(buyer) {
  // one unit is below the threshold: the flat fee applies (the threshold changes the price only from its boundary up)
  const { ctx, locale } = buyer, c = copy[locale], page = await ctx.newPage();
  // the product page is the storefront shell now: Add to cart -> /checkout (tests/storefront/shop-helpers.mjs), then the unchanged delivery steps
  await reachCheckout(page, origin, locale, product);
  await page.getByRole("button", { name: c.delivery, exact: true }).click();
  const quoted = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST");
  await page.getByRole("button", { name: c.quote, exact: true }).click();
  const quote = await (await quoted).json();
  assert.equal(quote.amount.subtotal_minor, unit); assert.equal(quote.amount.shipping_minor, fee, "below the threshold the flat fee applies");
  assert.equal(quote.amount.total_minor, unit + fee);
  await page.close();
  pass(`one unit (${unit}) is below the merchant's threshold: shipping ${fee}, total ${unit + fee}`);
}
async function place(buyer, store) {
  const { ctx, locale, mobile, label, email, pay } = buyer, c = copy[locale], viewport = mobile ? "mobile" : "desktop";
  const page = await ctx.newPage();
  await reachCheckout(page, origin, locale, product, { quantity: 2 }); // 2 units, bought through Add to cart (the shell replaced the product-page purchase panel)
  await page.getByRole("button", { name: c.delivery, exact: true }).click();
  const quoted = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST");
  await page.getByRole("button", { name: c.quote, exact: true }).click();
  const quote = await (await quoted).json();
  // the merchant set threshold == subtotal in settings: the inclusive boundary must already be free for the buyer
  assert.equal(quote.amount.subtotal_minor, price, `${label}: subtotal`);
  assert.equal(quote.amount.shipping_minor, 0, `${label}: shipping at threshold == subtotal must be 0`);
  assert.equal(quote.amount.total_minor, price, `${label}: total`);
  await expect(page.getByTestId("address-section")).toBeVisible();
  for (const [key, value] of Object.entries(pii)) await page.locator(`input[name="${key}"]`).fill(value);
  await page.getByRole("radio", { name: c.bank }).check(); // choosing the mode resets the confirmed address: confirm after it
  await page.getByTestId("confirm-address").click();
  await expect(page.getByTestId("create-order")).toBeEnabled();
  await expect(page.getByTestId("create-order")).toHaveText(c.create);
  await expect(page.locator(".order-note").filter({ hasText: c.note })).toBeVisible(); // the merchant's 6 hour window reaches the buyer
  if (email) await page.locator('input[name="buyer_email"]').fill(email);
  await shot(page, "checkout", locale, viewport);
  await page.getByTestId("create-order").click();
  await expect(page.getByTestId("order-section")).toBeVisible({ timeout: 30000 });
  const id = (await page.getByTestId("order-id").innerText()).trim();
  assert.match(id, /^[a-f0-9-]{36}$/);
  assert.equal(await page.getByTestId("pay-order").count(), 0, `${label}: a transfer order has no card payment step`);
  await expect(view(page)).toBeVisible({ timeout: 30000 });
  await expect(view(page)).toHaveAttribute("data-state", "AWAITING");
  // bank details and the countdown on the buyer's own order page
  await expect(page.getByTestId("transfer-account-number")).toHaveText(account);
  await expect(page.getByTestId("transfer-bank")).toContainText(bankName);
  await expect(page.getByTestId("transfer-amount")).toContainText(money(price).replace(/\.00$/, ""));
  await expect(page.getByTestId("transfer-deadline")).toContainText(c.hoursLeft);
  await shot(page, "order-awaiting", locale, viewport);
  if (pay) {
    await send(page, "12345", price);
    await shot(page, "order-submitted", locale, viewport);
  }
  pass(`${label} ${locale}/${viewport}: free shipping at the boundary, bank details + countdown, ${pay ? "proof submitted -> SUBMITTED, never CONFIRMED" : "no proof"}`);
  return { ...buyer, page, id };
}
const api = (page, method, suffix) => page.evaluate(async ({ method, suffix }) => {
  const session = await (await fetch("/api/buyer/session", { cache: "no-store" })).json();
  const response = await fetch(`/api/buyer/${suffix}`, { method, headers: { "X-Buyer-Context": session.context } });
  return { status: response.status, body: await response.json() };
}, { method, suffix });
async function refresh(page) { await page.getByTestId("transfer-refresh").click(); }
// stop-bleed D06: only the transfer panel was refreshed (never "Refresh order"); the order heading must still agree with it. It used to keep
// "Waiting for bank transfer" above "The shop confirmed your payment" / "...ended. This order was cancelled", and a confirmed order must not
// show the account again.
async function headingAgrees(page, locale, settled) {
  const c = copy[locale], head = page.getByTestId("order-state");
  if (process.env.UI_SHOT_PHASE === "before") { // capture-only run on the pre-fix code: what the page says, no assertion
    console.log(`BEFORE D06 ${locale}: heading data-state=${await head.getAttribute("data-state")} text="${await head.innerText()}", bank details in the DOM: ${await page.getByTestId("transfer-bank").count()}`);
    return;
  }
  await expect(head).toHaveAttribute("data-state", settled);
  await expect(head).toHaveText(settled === "CONFIRMED" ? c.headConfirmed : c.headCancelled);
  await expect(head).not.toContainText(c.headWaiting);
  await expect(page.getByTestId("transfer-bank")).toHaveCount(0);
  await expect(page.getByTestId("transfer-amount")).toHaveCount(0);
}

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
    { key: "A", locale: "zh-TW", mobile: false, email: "buyer.a@example.com", pay: true },
    { key: "B", locale: "en", mobile: false, email: "", pay: true },
    { key: "C", locale: "zh-TW", mobile: true, email: "", pay: true },
    { key: "D", locale: "en", mobile: true, email: "", pay: true },
    { key: "E", locale: "zh-TW", mobile: false, email: "", pay: false },
  ];
  const buyers = {};
  for (const p of plan) {
    const ctx = await newContext(p.mobile);
    if (p.key === "A") { const probe = await newContext(false); await belowThreshold({ ctx: probe, locale: p.locale }); await probe.close(); } // its own buyer: a quoted cart locks the quantity
    buyers[p.key] = await place({ ...p, label: `order ${p.key}`, ctx });
  }
  await writeFile(path.join(evidence, "orders.json"), JSON.stringify(Object.fromEntries(Object.entries(buyers).map(([k, v]) => [k, { id: v.id, locale: v.locale, mobile: v.mobile }])), null, 2));
  await signal("placed"); await waitGo("1");

  // ---- after the merchant's first pass ------------------------------------------------------------------------------------
  {
    const a = buyers.A; await refresh(a.page);
    await expect(view(a.page)).toHaveAttribute("data-state", "REJECTED");
    await expect(a.page.getByTestId("transfer-rejected")).toContainText(reason);
    // a rejected submission is not a cancelled order: the details stay, the form is back, the buyer resubmits
    await expect(a.page.getByTestId("transfer-account-number")).toHaveText(account);
    await shot(a.page, "order-rejected", a.locale, "desktop");
    await send(a.page, "54321", price);
    pass("order A: the rejection reason is shown, the order is still open, the resubmission goes through");
    for (const k of ["B", "C", "D"]) {
      const b = buyers[k], c = copy[b.locale]; await refresh(b.page);
      await expect(view(b.page)).toHaveAttribute("data-state", "CONFIRMED");
      await expect(view(b.page)).toContainText(c.confirmed);
      await expect(b.page.getByTestId("transfer-form")).toHaveCount(0);
      await headingAgrees(b.page, b.locale, "CONFIRMED");
      const order = await api(b.page, "GET", `orders/${b.id}`);
      assert.equal(order.status, 200); assert.equal(order.body.commercial_state, "CONFIRMED", `order ${k} after the merchant confirmed`);
      await shot(b.page, "order-confirmed", b.locale, b.mobile ? "mobile" : "desktop");
    }
    pass("orders B, C, D: the buyer sees the shop's confirmation (zh-TW/en, desktop/390px) and the order reads CONFIRMED");
    const e = buyers.E; await refresh(e.page);
    await expect(view(e.page)).toHaveAttribute("data-state", "AWAITING");
    const awaiting = await api(e.page, "GET", `orders/${e.id}`); assert.equal(awaiting.body.commercial_state, "AWAITING_TRANSFER");
    pass("order E (no proof): still waiting, never auto-confirmed");
  }
  await signal("after1"); await waitGo("2");
  {
    const a = buyers.A; await refresh(a.page);
    await expect(view(a.page)).toHaveAttribute("data-state", "CONFIRMED");
    await expect(view(a.page)).toContainText(copy[a.locale].confirmed);
    await headingAgrees(a.page, a.locale, "CONFIRMED");
    pass("order A: confirmed after the resubmission");
  }
  await signal("after2"); await waitGo("3");
  {
    const e = buyers.E, c = copy[e.locale]; await refresh(e.page);
    await expect(view(e.page)).toHaveAttribute("data-state", "EXPIRED");
    await expect(view(e.page)).toContainText(c.expired);
    await expect(e.page.getByTestId("transfer-account-number")).toHaveCount(0); // the details are withdrawn once the window ended
    await expect(e.page.getByTestId("transfer-form")).toHaveCount(0);
    await headingAgrees(e.page, e.locale, "CANCELLED");
    assert(!(await e.page.content()).includes(account), "the account number is still in the expired order's DOM");
    const cancelled = await api(e.page, "GET", `orders/${e.id}`); assert.equal(cancelled.body.commercial_state, "CANCELLED");
    await shot(e.page, "order-expired", e.locale, "desktop");
    for (const k of ["A", "B", "C", "D"]) { await refresh(buyers[k].page); await expect(view(buyers[k].page)).toHaveAttribute("data-state", "CONFIRMED"); }
    pass("order E: EXPIRED, bank details withdrawn, CANCELLED; A-D untouched");
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
