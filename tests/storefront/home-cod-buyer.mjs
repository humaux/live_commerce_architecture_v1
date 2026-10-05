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
    paid: "已貨到收款", note: /現金/, orderTitle: "你的訂單",
  },
  en: {
    delivery: "Choose delivery", quote: "Get current total", cod: /Cash on delivery/, createCod: "Place order (cash on delivery)",
    paid: "Paid on delivery", note: /cash/i, orderTitle: "Your order",
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
  await expect(page.locator("html")).toHaveAttribute("lang", locale);
  assert.equal(new URL(page.url()).pathname.split("/")[1], locale, "screenshot filename locale matches the displayed route");
  const file = path.join(evidence, `home-cod-${name}-${locale}-${viewport}.png`);
  await page.screenshot({ path: file, fullPage: true });
  if (name === "checkout" || name === "order-pending") {
    const previous = page.viewportSize();
    for (const width of [390, 1366, 1586]) {
      await page.setViewportSize({ width, height: width === 390 ? 844 : 992 });
      // G-UI8 audit [FIXTURE/SETUP]: waits for fonts and scrolls to the top before the geometry measurement (viewport positioning)
      await page.evaluate(async () => { await document.fonts.ready; window.scrollTo({ top: 0, behavior: "instant" }); });
      // G-UI8 audit [READ/MEASURE]: reads scrollY
      await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);
      // G-UI8 audit [READ/MEASURE]: reads scrollX
      await expect.poll(() => page.evaluate(() => window.scrollX)).toBe(0);
      // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `${name} ${locale} overflow at ${width}`);
      await page.screenshot({ path: path.join(root, "output/home-cod-ui", `buyer-${name}-${locale}-${width}.png`), fullPage: false, animations: "disabled", scale: "css" });
      if (name === "checkout") {
        await page.getByTestId("checkout-cod-amount").scrollIntoViewIfNeeded();
        assert(await page.locator(".purchase-footer strong").evaluate((el) => {
          const amount = el.getBoundingClientRect();
          const range = document.createRange(); range.selectNodeContents(el);
          const text = range.getBoundingClientRect();
          return text.left >= amount.left && text.right <= amount.right + 1 && text.right <= innerWidth && text.bottom <= innerHeight;
        }), `${locale}/${width}: full footer amount is visible without wrapping or clipping`);
        assert(await page.locator(".cod-amount strong").evaluate((el) => {
          const footer = document.querySelector(".purchase-footer strong");
          return footer && parseFloat(getComputedStyle(el).fontSize) > parseFloat(getComputedStyle(footer).fontSize);
        }), `${locale}/${width}: carrier amount has primary visual emphasis`);
        await page.screenshot({ path: path.join(root, "output/home-cod-ui", `buyer-confirm-${locale}-${width}.png`), fullPage: false, animations: "disabled", scale: "css" });
      } else {
        assert(await page.locator(".cod-amount strong").evaluate((el) => {
          const subtotal = document.querySelector(".order-total strong");
          return subtotal && parseFloat(getComputedStyle(el).fontSize) > parseFloat(getComputedStyle(subtotal).fontSize);
        }), `${locale}/${width}: collection amount is more prominent than the order subtotal`);
      }
    }
    await page.setViewportSize(previous);
  }
  // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
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
  const capRoute = async (route) => {
    const response = await route.fetch();
    const body = await response.json();
    body.items = body.items.map((option) => option.payment_modes?.includes("cash_on_delivery") ? { ...option, cod_max_minor: 100 } : option);
    await route.fulfill({ response, json: body });
  };
  if (label === "order A") await page.route("**/api/buyer/checkout-options?**", capRoute);
  const quoted = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST");
  await page.getByRole("button", { name: c.quote, exact: true }).click();
  const quote = await (await quoted).json();
  assert.equal(quote.amount.subtotal_minor, total, `${label}: subtotal`);
  assert.equal(quote.amount.shipping_minor, 0, `${label}: the fixture home policy is zero-fee`);
  assert.equal(quote.amount.total_minor, total, `${label}: total`);
  await expect(page.getByTestId("address-section")).toBeVisible();
  if (label === "order A") {
    await expect(page.getByTestId("cod-cap-unavailable")).toBeVisible();
    await expect(page.locator('input[name="home_payment_mode"][value="cash_on_delivery"]')).toHaveCount(0);
    await page.unroute("**/api/buyer/checkout-options?**", capRoute);
    await page.getByRole("button", { name: "重新選擇配送", exact: true }).click();
    await page.getByRole("button", { name: c.quote, exact: true }).click();
    await expect(page.locator('input[name="home_payment_mode"][value="cash_on_delivery"]')).toBeVisible();
  }
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
  if (label === "order A") {
    // MOCK one definite provider refusal on the real checkout flow; no upstream order is created.
    let optionReads = 0;
    const observe = (request) => { if (new URL(request.url()).pathname === "/api/buyer/checkout-options") optionReads++; };
    page.on("request", observe);
    await page.route("**/api/buyer/checkout", (route) => route.fulfill({
      status: 409, contentType: "application/json",
      headers: { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" },
      body: JSON.stringify({ code: "cod_surcharge_changed", retryable: false, message: "Fee changed", request_id: "0".repeat(32), details: {} }),
    }), { times: 1 });
    await page.getByTestId("create-order").click();
    await expect(page.getByTestId("cod-checkout-error")).toBeVisible();
    await expect(page.getByTestId("create-order")).toBeDisabled();
    await expect.poll(() => optionReads).toBeGreaterThan(0);
    await page.getByTestId("confirm-address").click();
    await expect(page.getByTestId("create-order")).toBeEnabled();
    page.off("request", observe);
  }
  const submitted = page.waitForRequest((request) => new URL(request.url()).pathname === "/api/buyer/checkout" && request.method() === "POST");
  await page.getByTestId("create-order").click();
  assert.equal((await submitted).postDataJSON().expected_cod_surcharge_minor, surcharge, "buyer-confirmed fee is sent to the server");
  await expect(page.getByTestId("order-section")).toBeVisible({ timeout: 30000 });
  const id = (await page.getByTestId("order-id").innerText()).trim();
  assert.match(id, /^[a-f0-9-]{36}$/);
  assert.equal(await page.getByTestId("pay-order").count(), 0, `${label}: a cash-on-delivery order has no card payment step`);
  const cod = page.getByTestId("order-cod");
  await expect(cod).toBeVisible({ timeout: 30000 });
  await expect(cod.getByTestId("order-cod-state")).toHaveAttribute("data-state", "PENDING");
  await expect(cod.getByTestId("order-cod-amount")).toContainText(`NT$${(total + surcharge) / 100}`);
  await expect(cod.getByTestId("order-cod-amount")).toContainText(`NT$${surcharge / 100}`);
  await expect(cod.getByTestId("order-cod-carrier")).toContainText(locale === "en" ? "Black Cat" : "黑貓");
  // SF-2: the page title is the order itself; the payment instruction is the labelled "Cash on delivery" section, stated once (no second status line).
  await expect(page.locator("#order-title")).toHaveText(c.orderTitle);
  await expect(cod.getByRole("heading", { level: 2 })).toHaveText(c.cod); // the section is labelled with the payment method, once
  await expect(page.getByTestId("order-state")).toHaveCount(0);
  assert.notEqual((await page.locator("#order-title").innerText()).trim(), (await cod.getByTestId("order-cod-state").innerText()).trim(), `${label}: the H1 is not the payment instruction`);
  await shot(page, "order-pending", locale, viewport);
  if (label === "order A") {
    const linked = await ctx.newPage();
    for (const language of ["zh-TW", "zh-CN", "en"]) {
      await linked.goto(`${origin}/${language}/orders/${id}`);
      await expect(linked.getByTestId("order-cod-amount")).toContainText(`NT$${(total + surcharge) / 100}`);
      await expect(linked.getByTestId("order-cod-carrier")).toContainText({ "zh-TW": "黑貓", "zh-CN": "黑猫", en: "Black Cat" }[language]);
      await shot(linked, "order-pending", language, "desktop");
    }
    await linked.close();
    const lookupContext = await newContext(false);
    const lookupPage = await lookupContext.newPage();
    await lookupPage.goto(`${origin}/${locale}/orders/lookup`);
    await lookupPage.getByTestId("lookup-ref").fill(id);
    await lookupPage.getByTestId("lookup-contact").fill(pii.phone);
    await lookupPage.getByTestId("lookup-submit").click();
    await expect(lookupPage.getByTestId("order-cod-amount")).toContainText(`NT$${(total + surcharge) / 100}`);
    await expect(lookupPage.getByTestId("order-cod-carrier")).toContainText("黑貓");
    await lookupContext.close();
  }
  pass(`${label} ${locale}/${viewport}: whole-TWD total ${total / 100}, surcharge ${surcharge / 100}, placed AWAITING_COLLECTION/PENDING`);
  return { ...buyer, page, id };
}
// G-UI8 audit [READ/MEASURE]: same-origin GET read of server state through the BFF (no state change)
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
    await expect(a.page.locator("#order-title")).toHaveText(copy[a.locale].orderTitle);
    await expect(a.page.getByTestId("order-cod").getByTestId("order-cod-state")).toHaveText(copy[a.locale].paid);
    const order = await api(a.page, "GET", `orders/${a.id}`);
    assert.equal(order.status, 200); assert.equal(order.body.commercial_state, "AWAITING_COLLECTION", "a collected COD order never becomes CONFIRMED");
    assert.equal(order.body.collection_state, "COLLECTED");
    assert.equal(order.body.cod_carrier, "black_cat");
    assert.equal(order.body.shipment.carrier_code, "black_cat");
    await shot(a.page, "order-collected", a.locale, "desktop");
    for (const k of ["B", "C", "D"]) {
      const b = buyers[k]; await refresh(b.page);
      await expect(b.page.getByTestId("order-cod").getByTestId("order-cod-state")).toHaveAttribute("data-state", "PENDING");
      const o = await api(b.page, "GET", `orders/${b.id}`);
      assert.equal(o.body.collection_state, "PENDING");
    }
    pass("order A: COLLECTED and still AWAITING_COLLECTION; B, C, D: still PENDING");
    // UI-only terminal projections. Do not mutate the four real PG orders that the parent gate reconciles.
    // These screenshots prove translated rendering, NOT real terminal-state transitions.
    const terminalTitles = {
      "zh-TW": { RETURNED: "未取貨 — 已退回", REFUNDED_OFFLINE: "商家已在本站之外退款", CANCELLED: "商家已取消此訂單" },
      en: { RETURNED: "Not collected — returned", REFUNDED_OFFLINE: "Refunded by the seller outside this site", CANCELLED: "The seller canceled this order" },
    };
    for (const locale of ["zh-TW", "en"]) {
      const preview = await a.ctx.newPage();
      let state = "RETURNED";
      await preview.route(`**/api/buyer/orders/${a.id}`, async (route) => {
        // Different snapshot vs recorded shipment proves we display the order-time carrier, not shipment/settings.
        const body = { ...order.body, cod_carrier: "hsinchu", collection_state: state };
        if (state === "CANCELLED") Object.assign(body, { commercial_state: "CANCELLED", fulfillment_state: "CANCELLED", shipment: null });
        await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
      });
      for (const terminal of Object.keys(terminalTitles[locale])) {
        state = terminal;
        await preview.goto(`${origin}/${locale}/orders/${a.id}`);
        await expect(preview.locator("#order-title")).toHaveText(copy[locale].orderTitle);
        await expect(preview.getByTestId("order-cod-state")).toHaveText(terminalTitles[locale][state]);
        await expect(preview.getByTestId("order-cod-carrier")).toContainText(locale === "en" ? "Carrier at checkout · Hsinchu" : "下單時的物流商 · 新竹");
        for (const width of [390, 1586]) {
          await preview.setViewportSize({ width, height: width === 390 ? 844 : 992 });
          await preview.evaluate(async () => { await document.fonts.ready; window.scrollTo({ top: 0, behavior: "instant" }); });
          assert(await preview.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
          await preview.screenshot({ path: path.join(root, "output/home-cod-ui", `buyer-${state.toLowerCase()}-mock-${locale}-${width}.png`), fullPage: false, animations: "disabled" });
        }
      }
      await preview.close();
    }
    pass("MOCK terminal buyer headings: RETURNED / REFUNDED_OFFLINE / CANCELLED, zh-TW/en at 390/1586");
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
