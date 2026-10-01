// BC gate (unit buyer-comms, independent R4 tests; contracts/storefront-v2.md §E4, §E5): a buyer places a bank_transfer order WITH an e-mail in the
// real storefront shell, the placed mail is captured by the loopback SMTP FAKE (the Go test runs the real notify worker and the real SMTP adapter
// against internal/mail/mailtest), and a FRESH browser (no cookie) does the guest lookup with the order number FROM THE MAIL + the e-mail: it sees the
// order and the bank instructions, can NOT submit a proof / export / erase / list orders, and a wrong e-mail or wrong number gets the identical
// refusal. Chromium or WebKit (LC_BROWSER_ENGINE), desktop 1440x900 and 390x844, zh-TW and en: four runs, each its own order and buyer.
// Stack: production storefront Next build -> real private buyerhttp handler -> isolated PG. Started by tests/foundation/browser_buyer_comms_test.go
// (TestBrowserBuyerComms, build tag browser), which owns PG, the SMTP fake and the worker and exposes a control listener (POST /tick runs the worker,
// GET /mails lists what the SMTP fake received, GET /facts counts proofs / privacy actions / orders in PG). This script never sees a database
// credential. The browser reaches only the synthetic host through one CONNECT proxy and one self-signed edge; the edge plays Caddy: it sets
// X-Forwarded-For from the per-context X-Gate-Client header (the BFF reads one valid IP literal from it for the per-IP lookup limit).
// BFF routes exercised: /api/buyer/{session,checkout-options,quotes,destination,checkout,orders/{id},orders/{id}/bank-transfer,orders/lookup} and the
// refused ones (/orders, /orders/{id}/bank-transfer/proof, /privacy/export, /privacy/erasure, /cart).
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { expect } from "@playwright/test";
import { launch, ctxOpts, iosZoomOffenders, phone as phoneProfile, engine } from "./browser-engine.mjs";
import { reachCheckout } from "./shop-helpers.mjs";
import { purchaseCopy } from "../../apps/storefront/lib/purchase-copy.ts";
import { bankTransferCopy } from "../../apps/storefront/lib/bank-transfer-copy.ts";
import { lookupCopy } from "../../apps/storefront/lib/lookup-copy.ts";

const env = (name) => { const v = process.env[name]; assert(v, `${name} is required`); return v; };
const root = process.cwd(), evidence = env("LC_BC_EVIDENCE"), origin = env("LC_BC_ORIGIN"), PRODUCT = env("LC_BC_PRODUCT"), SHOP = env("LC_BC_SHOP");
const HOST = new URL(origin).host;
const children = new Set(), sockets = new Set(), logs = [], shots = [];
let browser, edge, proxy, cases = 0, ipSeq = 0;
const pass = (name) => { cases++; console.log(`PASS ${name}`); };
const listen = async (s) => { s.listen(0, "127.0.0.1"); await once(s, "listening"); return s.address().port; };
const pause = (ms) => new Promise((r) => setTimeout(r, ms));
const pii = { recipient_name: "Synthetic Comms Recipient", phone: "+886900000091", region: "Synthetic Region", city: "Synthetic City", postal_code: "99991", line1: "Synthetic Address Ninety One", line2: "Synthetic Unit Ninety Two" };
const BANK = { name: "Taiwan Bank", account: "123-456-7890" };

async function control(p, method = "GET") {
  const r = await fetch(`${env("LC_BC_CONTROL")}/${p}`, { method, headers: { "X-Gate-Key": env("LC_BC_CONTROL_KEY") } });
  assert(r.status === 200 || r.status === 204, `control ${p}: ${r.status}`);
  return r.status === 204 ? null : r.json();
}
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
  const log = createWriteStream(path.join(evidence, "next.log"), { flags: "wx", mode: 0o600 });
  logs.push(log); await once(log, "open");
  const probe = net.createServer(); const port = await listen(probe); await new Promise((r) => probe.close(r));
  const childEnv = { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1" };
  for (const name of Object.keys(childEnv)) if (name.startsWith("LC_BC_")) delete childEnv[name];
  const child = spawn(process.execPath, [path.join(root, "apps/storefront/node_modules/next/dist/bin/next"), "start", "--hostname", "127.0.0.1", "--port", String(port)],
    { cwd: path.join(root, "apps/storefront"), env: childEnv, stdio: ["ignore", log, log] });
  children.add(child);
  for (let i = 0; i < 400; i++) {
    if (child.exitCode !== null) throw new Error("Next exited before readiness");
    try { const r = await relay(port, { url: "/api/buyer/session", method: "GET", headers: { host: HOST } }, Buffer.alloc(0)); if (r.status === 200) return { port, child }; } catch { /* not up yet */ }
    await pause(100);
  }
  throw new Error("Next readiness timeout");
}
async function newContext(mobile, locale) {
  const ip = `100.64.${(++ipSeq >> 8) & 255}.${ipSeq & 255}`;
  const opts = mobile
    ? { ...phoneProfile, viewport: { width: 390, height: 844 }, screen: { width: 390, height: 844 }, ignoreHTTPSErrors: true, locale }
    : { ignoreHTTPSErrors: true, viewport: { width: 1440, height: 900 }, locale };
  const c = await browser.newContext({ ...ctxOpts(opts), extraHTTPHeaders: { "x-gate-client": ip } });
  c.on("page", (p) => p.on("pageerror", (e) => console.log("PAGEERROR", e.message)));
  return c;
}
async function shot(page, name) {
  const file = path.join(evidence, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  shots.push({ File: path.basename(file), Sha256: createHash("sha256").update(await readFile(file)).digest("hex") });
}
const noOverflow = (p) => p.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1);
const numberOf = (text) => (/\b[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}\b/.exec(text) || [])[0];
const stripId = (s) => s.replace(/"request_id":"[0-9a-f]+"/g, '"request_id":"-"');

// ---- buyer: place a bank_transfer order with an e-mail through the shell's checkout ----------------------------------------------------------
async function placeBankOrder(page, locale, email) {
  const pc = purchaseCopy[locale], bt = bankTransferCopy[locale];
  await reachCheckout(page, origin, locale, PRODUCT);
  await page.getByRole("button", { name: pc.delivery, exact: true }).click();
  const quoted = page.waitForResponse((r) => r.url().includes("/api/buyer/quotes") && r.request().method() === "POST");
  await page.getByRole("button", { name: pc.quote, exact: true }).click();
  assert.equal((await quoted).status(), 200, "quote");
  await expect(page.getByTestId("address-section")).toBeVisible();
  for (const [name, value] of Object.entries(pii)) await page.locator(`input[name="${name}"]`).fill(value);
  const radio = page.getByRole("radio", { name: bt.payBank });
  assert.equal(await radio.count(), 1, "the bank transfer payment choice must be offered at checkout");
  await radio.check(); // choosing a mode resets the address confirmation, so it comes before it
  await page.getByTestId("confirm-address").click();
  await expect(page.getByTestId("create-order")).toBeEnabled();
  await page.locator('input[name="buyer_email"]').fill(email);
  const placed = page.waitForResponse((r) => r.url().includes("/api/buyer/checkout") && r.request().method() === "POST");
  await page.getByTestId("create-order").click();
  const res = await placed;
  assert.equal(res.status(), 200, `checkout answered ${res.status()} ${await res.text().catch(() => "")}`);
  await expect(page.getByTestId("order-section")).toBeVisible();
  await expect(page.getByTestId("bank-transfer")).toBeVisible();
  const orderID = (await page.getByTestId("order-id").innerText()).trim();
  assert.match(orderID, /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
  return orderID;
}

const certDir = await mkdtemp(path.join(tmpdir(), "lc-bc-edge-"));
const emails = [], numbers = [];
try {
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path.join(certDir, "key.pem"), "-out", path.join(certDir, "cert.pem"), "-days", "1", "-subj", `/CN=${HOST}`], { stdio: "ignore" });
  const next = await startNext();
  edge = https.createServer({ key: await readFile(path.join(certDir, "key.pem")), cert: await readFile(path.join(certDir, "cert.pem")) }, async (req, res) => {
    try {
      const chunks = []; for await (const x of req) chunks.push(x);
      const client = req.headers["x-gate-client"]; delete req.headers["x-gate-client"];
      req.headers["x-forwarded-for"] = client || "198.51.100.250"; // the edge, like Caddy, never forwards a visitor-supplied value
      const out = await relay(next.port, req, Buffer.concat(chunks));
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
  const proxyPort = await listen(proxy);
  browser = await launch({ headless: true, proxy: { server: `http://127.0.0.1:${proxyPort}` } });
  const facts0 = await control("facts");

  const matrix = [["zh-TW", false], ["zh-TW", true], ["en", false], ["en", true]];
  let lastBuyer, lastOrder;
  for (const [locale, mobile] of matrix) {
    const vp = mobile ? "mobile" : "desktop", tag = `${locale}-${vp}`, lc = lookupCopy[locale], bt = bankTransferCopy[locale];
    const email = `buyer-${tag.toLowerCase()}-${Date.now()}@buyers.example.test`;
    emails.push(email);

    // ---- 1. the buyer places the order ------------------------------------------------------------------------------------------------------
    const ctxA = await newContext(mobile, locale), a = await ctxA.newPage();
    const orderID = await placeBankOrder(a, locale, email);
    await expect(a.getByTestId("transfer-bank")).toContainText(BANK.name);
    await expect(a.getByTestId("transfer-account-number")).toHaveText(BANK.account);
    await expect(a.getByTestId("transfer-deadline")).toBeVisible();
    assert(await noOverflow(a), `order page overflows (${tag})`);
    await shot(a, `placed-${tag}`);
    pass(`BC01 ${tag}: the buyer places a bank_transfer order with an e-mail through the storefront shell and sees the bank instructions`);

    // ---- 2. the placed mail reaches the SMTP fake: store, number, link, bank details ------------------------------------------------------------
    await control("tick", "POST");
    const mails = (await control("mails")).filter((m) => m.to.toLowerCase() === email.toLowerCase());
    assert.equal(mails.length, 1, `${tag}: expected exactly one mail to ${email}, got ${mails.length}`);
    const mail = mails[0], number = numberOf(mail.text);
    assert(number, `${tag}: no order number in the mail:\n${mail.text}`);
    assert.equal(number.replace(/-/g, "").toLowerCase(), orderID.replace(/-/g, "").slice(0, 12), "the mail's order number is the first 12 hex digits of the order id");
    assert(mail.subject.includes(SHOP), `${tag}: subject lacks the store name: ${mail.subject}`);
    for (const want of [number, `${origin}/zh-TW/orders/${orderID}`, BANK.name, BANK.account]) assert(mail.text.includes(want), `${tag}: mail text lacks ${want}`);
    assert(mail.html.includes(number) && !/<img|<script|src=/i.test(mail.html), `${tag}: html part`);
    for (const other of emails.slice(0, -1)) assert(!mail.text.includes(other) && !mail.html.includes(other), `${tag}: mail leaks another buyer's address`);
    numbers.push(number);
    await writeFile(path.join(evidence, `placed-mail-${tag}.txt`), `Subject: ${mail.subject}\nTo: ${mail.to}\n\n${mail.text}`, { mode: 0o600 });
    pass(`BC02 ${tag}: the placed mail was captured by the SMTP fake with store name, order number ${number}, order link and the bank snapshot`);

    // ---- 3. a FRESH browser looks the order up with the number from the mail + the e-mail (phone on mobile) ---------------------------------------
    const ctxB = await newContext(mobile, locale), b = await ctxB.newPage();
    assert.equal((await ctxB.cookies()).length, 0, "the lookup browser starts with no cookie");
    const factsBefore = await control("facts");
    await b.goto(`${origin}/${locale}/orders/lookup`);
    await expect(b.getByTestId("order-lookup")).toBeVisible();
    assert(await noOverflow(b), `lookup page overflows (${tag})`);
    if (engine === "webkit" && mobile) assert.deepEqual(await iosZoomOffenders(b), [], "iOS focus-zoom on the lookup form");
    await shot(b, `lookup-${tag}`);
    await b.getByTestId("lookup-ref").fill(number.toLowerCase().replace(/-/g, " "));          // case and separators do not matter
    await b.getByTestId("lookup-contact").fill(mobile ? "0900000091" : ` ${email.toUpperCase()} `); // phone formats / e-mail case do not matter
    await b.getByTestId("lookup-submit").click();
    await b.waitForURL(`${origin}/${locale}/orders/${orderID}`);
    await expect(b.getByTestId("order-section")).toBeVisible();
    await expect(b.getByTestId("order-id")).toHaveText(orderID);
    await expect(b.getByTestId("bank-transfer")).toBeVisible();
    await expect(b.getByTestId("transfer-bank")).toContainText(BANK.name);
    await expect(b.getByTestId("transfer-account-number")).toHaveText(BANK.account);
    await expect(b.getByTestId("transfer-deadline")).toBeVisible();
    assert(await noOverflow(b), `guest order page overflows (${tag})`);
    await shot(b, `guest-order-${tag}`);
    pass(`BC03 ${tag}: the fresh browser finds the order with the mailed number + ${mobile ? "phone" : "e-mail"} and sees the order and the bank instructions`);

    // ---- 4. the guest session is view-only ------------------------------------------------------------------------------------------------------
    const call = (method, p, body, headers = {}) => b.evaluate(async ([method, p, body, headers]) => {
      const r = await fetch(p, { method, credentials: "same-origin", cache: "no-store", headers: { ...(body ? { "Content-Type": "application/json" } : {}), ...headers }, body: body ? JSON.stringify(body) : undefined });
      return { status: r.status, text: await r.text() };
    }, [method, p, body, headers]);
    const session = await call("GET", "/api/buyer/session");
    assert.equal(session.status, 200);
    const context = JSON.parse(session.text).context;
    const hdr = { "X-Buyer-Context": context };
    const forbidden = [
      ["PUT", `/api/buyer/orders/${orderID}/bank-transfer/proof`, { last5: "12345", amount_minor: 100, paid_at: new Date().toISOString() }, { ...hdr, "Idempotency-Key": crypto.randomUUID() }],
      ["POST", "/api/buyer/privacy/export", null, { ...hdr, "Idempotency-Key": crypto.randomUUID() }],
      ["POST", "/api/buyer/privacy/erasure", { confirm: "ERASE" }, { ...hdr, "Idempotency-Key": crypto.randomUUID() }],
      ["GET", "/api/buyer/orders", null, hdr],
      ["GET", "/api/buyer/privacy", null, hdr],
    ];
    const verdicts = [];
    for (const [method, p, body, headers] of forbidden) {
      const r = await call(method, p, body, headers);
      verdicts.push(`${method} ${p.replace(orderID, "{id}")} -> ${r.status}`);
      assert.equal(r.status, 403, `${tag}: ${method} ${p} answered ${r.status} ${r.text}`);
    }
    // the UI form: the buyer-facing proof form may still be drawn, but its submit must not record anything
    const form = b.getByTestId("transfer-form");
    if (await form.count()) {
      await form.locator('input[name="last5"]').fill("12345");
      await b.getByTestId("transfer-send").click();
      await pause(800);
      await expect(b.getByTestId("transfer-proof")).toHaveCount(0); // the "proof received" line never appears for a guest
    }
    const factsAfter = await control("facts");
    assert.equal(factsAfter.proofs, factsBefore.proofs, `${tag}: a proof was recorded by the guest session`);
    assert.equal(factsAfter.privacy_actions, factsBefore.privacy_actions, `${tag}: a privacy action was started by the guest session`);
    assert.equal(factsAfter.orders_with_email, factsBefore.orders_with_email, `${tag}: an erasure happened`);
    await writeFile(path.join(evidence, `view-only-${tag}.txt`), verdicts.join("\n") + "\n", { mode: 0o600 });
    pass(`BC04 ${tag}: the guest session answers 403 to proof, export, erasure, order list and privacy; PG facts unchanged (proofs ${factsAfter.proofs}, privacy actions ${factsAfter.privacy_actions})`);

    // ---- 5. wrong e-mail / wrong number / wrong phone: one identical refusal, nothing issued ------------------------------------------------------
    const ctxC = await newContext(mobile, locale), c = await ctxC.newPage();
    const refusals = [];
    c.on("response", async (r) => { if (r.url().endsWith("/api/buyer/orders/lookup")) refusals.push({ status: r.status(), body: stripId(await r.text().catch(() => "")), cc: r.headers()["cache-control"], cookie: r.headers()["set-cookie"] ?? "" }); });
    await c.goto(`${origin}/${locale}/orders/lookup`);
    const attempt = async (ref, contact) => {
      await c.getByTestId("lookup-ref").fill(ref); await c.getByTestId("lookup-contact").fill(contact); await c.getByTestId("lookup-submit").click();
      await expect(c.getByTestId("lookup-problem")).toBeVisible();
      return { problem: await c.getByTestId("lookup-problem").getAttribute("data-problem"), text: (await c.getByTestId("lookup-problem").innerText()).trim() };
    };
    const wrongEmail = await attempt(number, `someone-else-${tag.toLowerCase()}@buyers.example.test`);
    const wrongNumber = await attempt("FFFF-FFFF-FFFF", email);
    const wrongPhone = await attempt(number, "0911111119");
    const otherBuyer = emails.length > 1 ? await attempt(number, emails[0]) : wrongEmail;
    for (const r of [wrongEmail, wrongNumber, wrongPhone, otherBuyer]) {
      assert.equal(r.problem, "noMatch", `${tag}: refusal class ${r.problem}`);
      assert.equal(r.text, lc.noMatch, `${tag}: refusal text`);
    }
    assert.equal(refusals.length, emails.length > 1 ? 4 : 3, `${tag}: refusals seen ${refusals.length}`);
    for (const r of refusals) { assert.equal(r.status, 404); assert.equal(r.body, refusals[0].body, `${tag}: refusal bodies differ`); assert.equal(r.cookie, "", `${tag}: a refusal set a cookie`); assert(r.cc?.includes("no-store")); }
    assert.equal((await ctxC.cookies()).filter((k) => /buyer|session/i.test(k.name)).length, 0, "a refused lookup issued no buyer cookie");
    assert(c.url().endsWith(`/${locale}/orders/lookup`), "the page stays on the lookup form");
    await shot(c, `refused-${tag}`);
    pass(`BC05 ${tag}: wrong e-mail, wrong number, wrong phone and another buyer's e-mail show the identical "${lc.noMatch}" with identical 404 bodies and no cookie`);

    // ---- the original buyer is untouched ----------------------------------------------------------------------------------------------------------
    await a.goto(`${origin}/${locale}/orders/${orderID}`);
    await expect(a.getByTestId("order-id")).toHaveText(orderID);
    await ctxB.close(); await ctxC.close();
    lastBuyer = { ctx: ctxA, page: a, orderID, locale };
    if (!mobile || locale !== "en") await ctxA.close();
  }

  // ---- throttle in the UI: the 6th try on one number is "too many", even with the right e-mail ------------------------------------------------------
  {
    const locale = "en", lc = lookupCopy[locale], ctx = await newContext(false, locale), p = await ctx.newPage();
    await p.goto(`${origin}/${locale}/orders/lookup`);
    for (let i = 0; i < 5; i++) {
      await p.getByTestId("lookup-ref").fill("ABCD-0123-4567"); await p.getByTestId("lookup-contact").fill(`guess${i}@buyers.example.test`); await p.getByTestId("lookup-submit").click();
      await expect(p.getByTestId("lookup-problem")).toHaveAttribute("data-problem", "noMatch");
    }
    await p.getByTestId("lookup-ref").fill("ABCD-0123-4567"); await p.getByTestId("lookup-contact").fill(emails[0]); await p.getByTestId("lookup-submit").click();
    await expect(p.getByTestId("lookup-problem")).toHaveAttribute("data-problem", "tooMany");
    await expect(p.getByTestId("lookup-problem")).toHaveText(lc.tooMany);
    await ctx.close();
    pass("BC06 the 6th lookup on one order number is throttled in the UI (429 -> 'too many'), also for a different e-mail");
  }

  // ---- positive control: the buyer's own (checkout) session CAN submit a proof, so the zero deltas above are not vacuous -------------------------------
  {
    const { page: a, orderID, locale } = lastBuyer;
    await a.goto(`${origin}/${locale}/orders/${orderID}`);
    const before = await control("facts");
    const form = a.getByTestId("transfer-form");
    await expect(form).toBeVisible();
    await form.locator('input[name="last5"]').fill("54321");
    await a.getByTestId("transfer-send").click();
    await expect.poll(async () => (await control("facts")).proofs, { timeout: 15000 }).toBe(before.proofs + 1);
    pass("BC07 control: the order's own checkout session can submit the proof (proofs +1), so the guest zero deltas are real");
    await lastBuyer.ctx.close();
  }
  await writeFile(path.join(evidence, "screenshots.json"), JSON.stringify(shots, null, 2));
  const facts1 = await control("facts");
  console.log(`facts before ${JSON.stringify(facts0)} after ${JSON.stringify(facts1)}`);
  console.log(`engine=${engine} cases=${cases} emails=${emails.length}`);
} finally {
  await browser?.close().catch(() => {});
  edge?.close(); proxy?.close();
  for (const s of sockets) s.destroy();
  for (const c of children) c.kill("SIGTERM");
  for (const l of logs) l.end();
}
