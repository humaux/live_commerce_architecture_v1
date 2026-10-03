// AT5 REAL_BROWSER: published product -> cart -> HOME/bank-transfer checkout Begin -> own order URL.
// The Go runner supplies an isolated PG + private buyerhttp and a read-only /order-check control endpoint.
// No page.route, synthesized order, payment completion, CAPI consent, or Meta request is part of this driver.
// Required LC_AT_*: ORIGIN, PRODUCT, DRAFT, EVIDENCE, CONTROL, CONTROL_KEY; COMMERCE_BUYER_* as ads-consent.mjs.
// Control POST /order-check: X-Gate-Key + {order_id,draft_id} -> {order_id,path:'ad_click',draft_id}.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
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
import { launch, ctxOpts, phone } from "./browser-engine.mjs";
import { AD_TOUCH_COOKIE } from "../../apps/storefront/lib/ad-touch.ts";
import { purchaseCopy } from "../../apps/storefront/lib/purchase-copy.ts";

const required = (name) => {
  assert(process.env[name], `${name} is required`);
  return process.env[name];
};
const root = process.cwd();
const evidence = required("LC_AT_EVIDENCE");
const origin = required("LC_AT_ORIGIN");
const product = required("LC_AT_PRODUCT");
const draft = required("LC_AT_DRAFT");
const control = required("LC_AT_CONTROL");
const controlKey = required("LC_AT_CONTROL_KEY");
const uuid = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const publicURL = new URL(origin);
assert.equal(publicURL.origin, origin);
assert.equal(publicURL.protocol, "https:");
assert.equal(publicURL.port, "");
assert(
  publicURL.hostname.endsWith(".example"),
  "synthetic .example origin only",
);
const controlURL = new URL(control);
assert.equal(controlURL.protocol, "http:");
assert.equal(
  controlURL.hostname,
  "127.0.0.1",
  "runner control must be loopback",
);
assert(
  !controlURL.username &&
    !controlURL.password &&
    !controlURL.search &&
    !controlURL.hash,
);
assert(
  uuid.test(product) && uuid.test(draft),
  "fixture product and draft UUIDs required",
);

const children = new Set(),
  sockets = new Set(),
  contexts = [],
  logs = [];
const ledger = [],
  screenshots = [],
  results = [];
const cookieNames = [AD_TOUCH_COOKIE, "lc_fbc", "lc_fbp"];
const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const certDir = await mkdtemp(path.join(tmpdir(), "lc-attribution-edge-"));
let browser, edge, proxy;

async function listen(server) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return server.address().port;
}
function relay(port, req, body) {
  return new Promise((resolve, reject) => {
    const headers = { ...req.headers };
    delete headers.connection;
    delete headers["transfer-encoding"];
    if (body.length) headers["content-length"] = String(body.length);
    else delete headers["content-length"];
    const call = http.request(
      {
        hostname: "127.0.0.1",
        port,
        path: req.url,
        method: req.method,
        headers,
      },
      (response) => {
        const chunks = [];
        response.on("data", (chunk) => chunks.push(chunk));
        response.on("error", reject);
        response.on("end", () =>
          resolve({
            status: response.statusCode,
            headers: response.headers,
            body: Buffer.concat(chunks),
          }),
        );
      },
    );
    call.setTimeout(15000, () =>
      call.destroy(new Error("owned relay deadline")),
    );
    call.on("error", reject);
    call.end(body);
  });
}
async function startNext() {
  const reserve = net.createServer();
  const port = await listen(reserve);
  await new Promise((resolve) => reserve.close(resolve));
  const log = createWriteStream(
    path.join(evidence, "ads-attribution-next.log"),
    { flags: "wx", mode: 0o600 },
  );
  logs.push(log);
  await once(log, "open");
  const childEnv = {
    ...process.env,
    NODE_ENV: "production",
    NEXT_TELEMETRY_DISABLED: "1",
  };
  // The browser runner secret must never reach the storefront process or its logs.
  for (const key of Object.keys(childEnv))
    if (key.startsWith("LC_AT_")) delete childEnv[key];
  const child = spawn(
    process.execPath,
    [
      path.join(root, "apps/storefront/node_modules/next/dist/bin/next"),
      "start",
      "--hostname",
      "127.0.0.1",
      "--port",
      String(port),
    ],
    {
      cwd: path.join(root, "apps/storefront"),
      env: childEnv,
      stdio: ["ignore", log, log],
    },
  );
  children.add(child);
  for (let attempt = 0; attempt < 100; attempt++) {
    if (child.exitCode !== null) throw new Error("owned Next failed readiness");
    try {
      if (
        (
          await relay(
            port,
            {
              url: "/api/buyer/session",
              method: "GET",
              headers: { host: publicURL.host },
            },
            Buffer.alloc(0),
          )
        ).status === 200
      )
        return port;
    } catch {
      /* starting */
    }
    await pause(50);
  }
  throw new Error("owned Next readiness timeout");
}
async function setup() {
  execFileSync(
    "openssl",
    [
      "req",
      "-x509",
      "-newkey",
      "rsa:2048",
      "-nodes",
      "-keyout",
      path.join(certDir, "key.pem"),
      "-out",
      path.join(certDir, "cert.pem"),
      "-days",
      "1",
      "-subj",
      `/CN=${publicURL.hostname}`,
    ],
    { stdio: "ignore" },
  );
  const nextPort = await startNext();
  edge = https.createServer(
    {
      key: await readFile(path.join(certDir, "key.pem")),
      cert: await readFile(path.join(certDir, "cert.pem")),
    },
    async (req, res) => {
      try {
        const chunks = [];
        for await (const chunk of req) chunks.push(chunk);
        const out = await relay(nextPort, req, Buffer.concat(chunks));
        const headers = { ...out.headers };
        delete headers.connection;
        delete headers["transfer-encoding"];
        res.writeHead(out.status, headers);
        res.end(out.body);
      } catch {
        if (!res.headersSent) res.writeHead(502);
        res.end();
      }
    },
  );
  const edgePort = await listen(edge);
  proxy = http.createServer((_, response) => {
    response.writeHead(403);
    response.end();
  });
  proxy.on("connect", (req, socket, head) => {
    if (req.url !== `${publicURL.hostname}:443`) {
      socket.destroy();
      return;
    }
    const upstream = net.connect(edgePort, "127.0.0.1", () => {
      socket.write("HTTP/1.1 200 Connection Established\r\n\r\n");
      if (head.length) upstream.write(head);
      socket.pipe(upstream).pipe(socket);
    });
    for (const stream of [socket, upstream]) {
      sockets.add(stream);
      stream.on("close", () => sockets.delete(stream));
      stream.on("error", () => {
        socket.destroy();
        upstream.destroy();
      });
    }
  });
  browser = await launch({
    proxy: { server: `http://127.0.0.1:${await listen(proxy)}` },
  });
}
const isResponse = (response, suffix, method) =>
  new URL(response.url()).pathname === `/api/buyer/${suffix}` &&
  response.request().method() === method;
function record(locale, width, action, details = {}) {
  ledger.push({ locale, width, action, ...details });
}
async function click(page, locator, locale, width, action) {
  await locator.click();
  record(locale, width, action);
}
async function shot(page, name, locale, width) {
  assert(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth + 1,
    ),
    `overflow ${name} ${locale} ${width}`,
  );
  const file = `attribution-${name}-${locale}-${width}.png`;
  await page.screenshot({ path: path.join(evidence, file), fullPage: true });
  screenshots.push({
    File: file,
    Sha256: createHash("sha256")
      .update(await readFile(path.join(evidence, file)))
      .digest("hex"),
    Locale: locale,
    Viewport: String(width),
  });
  await writeFile(
    path.join(evidence, "screenshots.json"),
    JSON.stringify(screenshots, null, 2),
  );
}
async function assertCookies(context, headers) {
  const cookies = await context.cookies(origin);
  for (const name of cookieNames) {
    const matching = cookies.filter((cookie) => cookie.name === name);
    assert.equal(matching.length, 1, `${name}: exactly one cookie`);
    const cookie = matching[0];
    assert.equal(cookie.domain, publicURL.hostname, `${name}: host-only`);
    assert.equal(cookie.path, "/");
    assert.equal(cookie.secure, true);
    assert.equal(cookie.httpOnly, true);
    assert.equal(cookie.sameSite, "Lax");
    const setCookies = headers.filter(
      (header) =>
        header.name.toLowerCase() === "set-cookie" &&
        header.value.startsWith(`${name}=`),
    );
    assert(
      setCookies.length > 0,
      `${name}: observed real Set-Cookie on landing/redirect`,
    );
    for (const header of setCookies)
      assert(
        !/;\s*domain\s*=/i.test(header.value),
        `${name}: Domain must be absent`,
      );
  }
}
async function checkOrder(orderID) {
  const response = await fetch(`${control.replace(/\/$/, "")}/order-check`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Gate-Key": controlKey },
    body: JSON.stringify({ order_id: orderID, draft_id: draft }),
    signal: AbortSignal.timeout(15000),
  });
  assert.equal(response.status, 200, "authoritative runner order-check");
  const facts = await response.json();
  assert.equal(
    facts.order_id,
    orderID,
    "runner checked the browser-observed order",
  );
  assert.equal(facts.path, "ad_click");
  assert.equal(facts.draft_id, draft);
  return {
    order_id: facts.order_id,
    path: facts.path,
    draft_id: facts.draft_id,
  };
}
async function place(locale, width) {
  const context = await browser.newContext(
    ctxOpts({
      ...(width === 390 ? phone : {}),
      ignoreHTTPSErrors: true,
      viewport: { width, height: width === 390 ? 844 : 1000 },
    }),
  );
  contexts.push(context);
  const page = await context.newPage();
  page.setDefaultTimeout(30000);
  const headerReads = [],
    pageErrors = [];
  page.on("pageerror", (error) => pageErrors.push(error.name));
  page.on("response", (response) => {
    if (
      response.request().isNavigationRequest() &&
      new URL(response.url()).origin === origin
    )
      headerReads.push(response.headersArray());
  });
  const landing = await page.goto(
    `${origin}/${locale}/products/${product}?lc_ad=${draft}&fbclid=synthetic_${locale.replace("-", "_")}_${width}`,
  );
  assert.equal(landing.status(), 200, "published ad landing");
  await expect(page.getByTestId("add-to-cart")).toBeVisible();
  await assertCookies(context, (await Promise.all(headerReads)).flat());
  record(
    locale,
    width,
    "ad landing with first-party host-only Secure HttpOnly Lax cookies",
  );
  await shot(page, "landing", locale, width);
  await click(
    page,
    page.getByTestId("add-to-cart"),
    locale,
    width,
    "add to cart",
  );
  await click(
    page,
    page.getByTestId("cart-checkout"),
    locale,
    width,
    "open checkout",
  );
  await page.waitForURL(`${origin}/${locale}/checkout`);
  const optionsWait = page.waitForResponse((response) =>
    isResponse(response, "checkout-options", "GET"),
  );
  await click(
    page,
    page.getByRole("button", {
      name: purchaseCopy[locale].delivery,
      exact: true,
    }),
    locale,
    width,
    "choose delivery",
  );
  const optionsResponse = await optionsWait;
  assert.equal(optionsResponse.status(), 200);
  const options = await optionsResponse.json();
  const home = options.items.find(
    (option) =>
      option.delivery_kind === "home" &&
      option.country === "TW" &&
      option.payment_modes?.includes("bank_transfer"),
  );
  assert(home, "fixture must offer Taiwan HOME with bank transfer");
  await page
    .locator("#delivery")
    .selectOption(`${home.market_id}:${home.country}:${home.method}`);
  record(locale, width, "select HOME bank-transfer-capable delivery");
  const quoteWait = page.waitForResponse((response) =>
    isResponse(response, "quotes", "POST"),
  );
  await click(
    page,
    page.getByRole("button", { name: purchaseCopy[locale].quote, exact: true }),
    locale,
    width,
    "quote real cart",
  );
  const quoted = await quoteWait;
  assert(quoted.ok(), "real cart quotation accepted");
  const quote = await quoted.json();
  assert(uuid.test(quote.id), "real quotation ID");
  await expect(page.getByTestId("address-section")).toBeVisible();
  const synthetic = {
    recipient_name: "Synthetic Attribution Recipient",
    phone: "+886900000095",
    region: "Synthetic Region",
    city: "Synthetic City",
    postal_code: "99995",
    line1: "Synthetic Attribution Address",
    line2: "Synthetic Unit",
  };
  for (const [name, value] of Object.entries(synthetic))
    await page.locator(`input[name="${name}"]`).fill(value);
  await page
    .locator('input[name="home_payment_mode"][value="bank_transfer"]')
    .check();
  record(locale, width, "fill synthetic address and choose bank transfer");
  const destinationWait = page.waitForResponse((response) =>
    isResponse(response, "destination", "POST"),
  );
  await click(
    page,
    page.getByTestId("confirm-address"),
    locale,
    width,
    "confirm destination",
  );
  const destinationResponse = await destinationWait;
  assert(destinationResponse.ok(), "real destination accepted");
  const destination = await destinationResponse.json();
  assert(uuid.test(destination.id), "real confirmed destination ID");
  await expect(page.getByTestId("create-order")).toBeEnabled();
  await shot(page, "checkout", locale, width);
  const beginWait = page.waitForResponse((response) =>
    isResponse(response, "checkout", "POST"),
  );
  await click(
    page,
    page.getByTestId("create-order"),
    locale,
    width,
    "submit genuine Begin",
  );
  const begin = await beginWait;
  assert(begin.ok(), `real Begin status ${begin.status()}`);
  const submitted = begin.request().postDataJSON();
  assert.equal(submitted.payment_mode, "bank_transfer");
  assert.equal(
    submitted.quote_id,
    quote.id,
    "Begin uses this actual quotation",
  );
  assert.equal(
    submitted.destination_id,
    destination.id,
    "Begin uses this actual confirmed destination",
  );
  assert(
    uuid.test(submitted.quote_id) && uuid.test(submitted.destination_id),
    "Begin quote and confirmed destination required",
  );
  assert(
    uuid.test(begin.request().headers()["idempotency-key"] ?? ""),
    "Begin idempotency key required",
  );
  assert(
    !("ad_touch" in submitted),
    "browser does not invent BFF authenticated touch",
  );
  const receipt = await begin.json();
  assert(uuid.test(receipt.order_id), "Begin receipt order UUID");
  await expect(page.getByTestId("order-section")).toBeVisible();
  const orderID = (await page.getByTestId("order-id").innerText()).trim();
  assert.equal(
    orderID,
    receipt.order_id,
    "DOM order agrees with genuine Begin receipt",
  );
  assert.equal(
    await page.getByTestId("pay-order").count(),
    0,
    "offline order: no PSP mutation",
  );
  record(locale, width, "Begin response matches rendered order", {
    status: begin.status(),
    order_id: orderID,
  });
  await shot(page, "placed", locale, width);
  // Checkout has no separate order-link CTA: open its actual public order URL in the same authenticated browser context.
  const orderRead = page.waitForResponse((response) =>
    isResponse(response, `orders/${orderID}`, "GET"),
  );
  const linked = await page.goto(`${origin}/${locale}/orders/${orderID}`);
  assert.equal(linked.status(), 200);
  assert.equal((await orderRead).status(), 200, "own order link read succeeds");
  await expect(page.getByTestId("order-id")).toHaveText(orderID);
  record(locale, width, "open real own-order URL", { order_id: orderID });
  const facts = await checkOrder(orderID);
  results.push({ locale, width, ...facts });
  record(
    locale,
    width,
    "runner verifies committed ad_click attribution",
    facts,
  );
  await shot(page, "order", locale, width);
  assert.deepEqual(pageErrors, [], "no uncaught browser errors");
  await context.close();
  console.log(
    `PASS AT5 ${locale} ${width}: genuine Begin + committed ad_click order`,
  );
}

try {
  await setup();
  for (const locale of ["zh-TW", "zh-CN", "en"])
    for (const width of [390, 1586]) await place(locale, width);
  assert.equal(
    new Set(results.map((result) => result.order_id)).size,
    6,
    "six fresh-context genuine orders",
  );
  await writeFile(
    path.join(evidence, "ads-attribution-result.json"),
    JSON.stringify({ label: "REAL_BROWSER", results }, null, 2),
  );
  console.log("PASS ads-attribution.mjs");
} finally {
  try {
    await writeFile(
      path.join(evidence, "click-ledger.json"),
      JSON.stringify(ledger, null, 2),
    );
  } finally {
    await Promise.allSettled(contexts.map((context) => context.close()));
    await browser?.close().catch(() => {});
    for (const child of children) child.kill("SIGKILL");
    for (const socket of sockets) socket.destroy();
    await new Promise((resolve) => edge?.close(resolve) ?? resolve());
    await new Promise((resolve) => proxy?.close(resolve) ?? resolve());
    for (const log of logs) log.end();
    // Exact task-owned mkdtemp directory only; evidence and shared caches are retained.
    await rm(certDir, { recursive: true, force: true });
  }
}
