// Purpose: catalog-media-gate.mjs startup uses bounded shared bind-race recovery; gate assertions are unchanged.
// Depends on: tests/helpers/next-startup.mjs and existing app/fixture/edge imports below.
// Used by: its scripts/dev/test-local.sh browser mode; GATE-PORT acceptance.
// GATE-PORT r1 (K3 P1): the admin no longer receives a freed explicit LC_CM_ADMIN_PORT. It starts on an
// automatic private port (so the shared retry helper covers it) and attaches to the Go-owned public origin
// through connectFixtureAdmin + the runner-only LC_CM_CONTROL seam, exactly like the publish/domains/
// merchant-buyer gates. Fail fast when the seam is missing — a plain throw, not a new gate assertion:
// the AST parity guard keeps the original assertion list byte-equal to the baseline.
// Independent catalog-media browser gate (R3 test author; docs/delivery/units/catalog-media.md CM3-CM6).
// Merchant half (production admin Next + signed MOCK IdP + real BFF/Go/PG): upload 2 photos, reorder, rename the product, change a SKU
// price and archive the other SKU in the Ledger. Buyer half (production storefront Next, a fresh anonymous context on the published
// shop origin): the home grid shows the cover photo, the new name and the new lowest price; the product page shows the gallery in the new
// order, the new price and no archived SKU. Matrix: zh-TW + en, desktop 1280px + mobile 390px, four distinct products.
// Driven by TestBrowserCatalogMedia (tests/foundation/browser_catalog_media_test.go); the only host mapping is a disposable TLS/CONNECT edge.
import { startNextWithPortRetry, nextAttemptLog, connectFixtureAdmin } from "../helpers/next-startup.mjs";
import assert from "node:assert/strict";
import crypto from "node:crypto";
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
import { launch, ctxOpts } from "./browser-engine.mjs";

const root = process.cwd(), evidence = process.env.LC_CM_EVIDENCE;
const adminOrigin = process.env.COMMERCE_PUBLIC_ORIGIN, buyerOrigin = "https://buyer.example";
const store = process.env.LC_CM_STORE, currency = process.env.LC_CM_CURRENCY;
const fixtures = JSON.parse(process.env.LC_CM_FIXTURES), filesDir = process.env.LC_CM_FILES;
assert(evidence && store && currency && fixtures.length === 4 && /^https?:\/\/127\.0\.0\.1:\d+$/.test(adminOrigin));
const certDir = await mkdtemp(path.join(tmpdir(), "lc-catalog-media-edge-"));
const children = new Set(), sockets = new Set(), logs = [];
const running = child => child.exitCode === null && child.signalCode === null;
let browser, edge, proxy, cases = 0;
const pass = name => { cases++; console.log(`PASS ${name}`); };
const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
const listen = async server => { server.listen(0, "127.0.0.1"); await once(server, "listening"); return server.address().port; };
const sha = buf => crypto.createHash("sha256").update(buf).digest("hex");
const digits = minor => (minor / 100).toFixed(2);
const shots = [];
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
async function startNext(app, requestedPort) {
  return startNextWithPortRetry({ port: requestedPort }, async ({ port, attempt, track }) => {

    const log = createWriteStream(nextAttemptLog(path.join(evidence, `${app}.log`), attempt), {flags: "wx", mode: 0o600});
    logs.push(log); await once(log, "open");
    const env = {...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1"};
    for (const name of Object.keys(env)) if (name.startsWith("LC_CM_")) delete env[name];
    const args = app === "admin"
      ? [path.join(root, "apps/admin/.next/standalone/apps/admin/server.js")]
      : [path.join(root, "apps/storefront/node_modules/next/dist/bin/next"), "start", "--hostname", "127.0.0.1", "--port", String(port)];
    env.HOSTNAME = "127.0.0.1"; env.PORT = String(port);
    const child = spawn(process.execPath, args, {cwd: path.join(root, `apps/${app}`), env, stdio: ["ignore", log, log]});
    track(child, log);
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

  });
}
async function shot(page, name, run) {
  const file = path.join(evidence, `${name}-${run.locale}-${run.vp}.png`);
  await page.screenshot({path: file, fullPage: true});
  shots.push({file: path.basename(file), sha256: sha(await readFile(file)), locale: run.locale, viewport: run.vp, page: name});
}

async function scenario(index, run) {
  const fx = fixtures[index], vp = run.vp === "mobile" ? {width: 390, height: 844} : {width: 1280, height: 900};
  const png = await readFile(path.join(filesDir, fx.png)), jpg = await readFile(path.join(filesDir, fx.jpg));
  const newName = `Renamed ${run.locale} ${run.vp} ${Date.now()}`, label = `${run.locale}/${run.vp}`;
  // ---- merchant half --------------------------------------------------------------------------------------------------
  const context = await browser.newContext(ctxOpts({ignoreHTTPSErrors: true, viewport: vp}));
  const merchant = await context.newPage();
  merchant.on("pageerror", error => uiErrors.push(`${label} merchant ${error.name}: ${error.message}`));
  await merchant.goto(`${adminOrigin}/en`);
  await merchant.getByRole("button", {name: "Sign in with identity service", exact: true}).click();
  // stop-bleed D01 (product-editor §c9): the Ledger (/[locale]/inventory) is stock only now. Photos, rename, SKU price and SKU archive are done in
  // the product editor (/[locale]/products/{id}); the Ledger is where the results are read back (name, price, status, real cover thumbnail).
  const row = code => merchant.locator("tbody tr").filter({hasText: code});
  const bff = (resource) => `/api/stores/${store}/${resource}`;
  const done = (method, resource) => merchant.waitForResponse(r => r.request().method() === method && new URL(r.url()).pathname === bff(resource));
  const major = minor => minor % 100 === 0 ? String(minor / 100) : (minor / 100).toFixed(2); // the editor's price fields are major units (D02)
  await merchant.goto(`${adminOrigin}/${run.locale}/inventory`);
  await expect(row(fx.sku1.code)).toBeVisible();
  await expect(row(fx.sku2.code)).toBeVisible();
  await expect(merchant.getByTestId("photo-manager"), "the Ledger tray no longer edits photos").toHaveCount(0);
  await merchant.goto(`${adminOrigin}/${run.locale}/products/${fx.product_id}?store=${store}`);
  await expect(merchant.getByTestId("photo-manager")).toBeVisible();
  await expect(merchant.getByTestId("photo-row")).toHaveCount(0);
  // 1. upload two photos through the real file input (PNG first, JPEG second)
  for (const [n, file] of [[1, {name: "front.png", mimeType: "image/png", buffer: png}], [2, {name: "side.jpg", mimeType: "image/jpeg", buffer: jpg}]]) {
    const input = merchant.getByTestId("photo-input");
    await expect(input).toBeEnabled();
    const response = done("POST", `products/${fx.product_id}/images`);
    await input.setInputFiles(file);
    const reply = await response; assert.equal(reply.status(), 200, `${label} upload ${n}`);
    await expect(merchant.getByTestId("photo-row")).toHaveCount(n);
  }
  pass(`${label} two photos uploaded through the real file input`);
  // 2. reorder: move the PNG (first) later, so the JPEG becomes the cover
  await expect(merchant.getByTestId("photo-row").nth(0).getByRole("button").nth(1)).toBeEnabled();
  const ordered = done("POST", `products/${fx.product_id}/images/order`);
  await merchant.getByTestId("photo-row").nth(0).getByRole("button").nth(1).click();
  assert.equal((await ordered).status(), 200, `${label} reorder`);
  const listed = await merchant.evaluate(async url => { const r = await fetch(url); return {status: r.status, body: await r.json()}; }, bff(`products/${fx.product_id}/images`));
  assert.equal(listed.status, 200);
  assert.deepEqual(listed.body.items.map(i => [i.content_type, i.position]), [["image/jpeg", 0], ["image/png", 1]], `${label} order after reorder`);
  const ids = listed.body.items.map(i => i.id);
  pass(`${label} reorder persisted: JPEG is the cover, PNG second, positions 0 and 1`);
  // 3. rename (the editor's basics form)
  await merchant.getByTestId("product-name").fill(newName);
  const renamed = done("PUT", `products/${fx.product_id}/document`);
  await merchant.getByTestId("product-save").click();
  assert.equal((await renamed).status(), 200, `${label} rename`);
  await expect(merchant.getByTestId("product-save")).toBeEnabled(); await merchant.reload();
  // 4. SKU price (the variant row, major units)
  await merchant.locator(`[data-sku-id="${fx.sku1.id}"]`).locator('[data-testid^="new-price-"]').fill(major(fx.sku1.price_new));
  const repriced = done("PUT", `products/${fx.product_id}/document`);
  await merchant.getByTestId("product-save").click();
  assert.equal((await repriced).status(), 200, `${label} price`);
  await expect(merchant.getByTestId("product-save")).toBeEnabled(); await merchant.reload();
  // 5. archive the other SKU (confirm step in the same row)
  await merchant.locator(`[data-sku-id="${fx.sku2.id}"]`).locator('[data-testid^="matrix-active-"]').uncheck();
  const archived = done("PUT", `products/${fx.product_id}/document`);
  merchant.once("dialog", d=>d.accept());
  await merchant.getByTestId("product-save").click();
  assert.equal((await archived).status(), 200, `${label} archive`);
  await expect(merchant.getByTestId("product-save")).toBeEnabled(); await merchant.reload();
  // ---- the Ledger reads the results back ----------------------------------------------------------------------------------------
  await merchant.goto(`${adminOrigin}/${run.locale}/inventory`);
  await expect(row(fx.sku1.code).locator("button.product-name")).toHaveText(newName);
  await expect(row(fx.sku1.code)).toContainText(major(fx.sku1.price_new));
  // The status column is hidden by the baseline CSS at 390px, so the row state is asserted attached (not visible); the tray, which is visible
  // on both viewports, shows the archived SKU's stock adjustment disabled.
  // the row state lives in the status column; the mobile badge (.status.mobile-status) is a second copy, so scope to .status-col
  await expect(row(fx.sku2.code).locator(".status-col .status.archived")).toBeAttached();
  await expect(row(fx.sku1.code).locator(".status-col .status.active")).toBeAttached();
  await row(fx.sku2.code).locator("button.product-name").click();
  await expect(merchant.locator("section.inspector").getByRole("spinbutton")).toBeDisabled();
  if (run.vp === "desktop") await expect(row(fx.sku2.code).locator(".status-col .status.archived")).toBeVisible();
  await expect(row(fx.sku1.code).locator("[data-photo=real]")).toBeVisible(); // the Ledger shows the real cover
  assert.equal(await merchant.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${label} admin horizontal overflow`);
  await shot(merchant, "admin-ledger", run);
  pass(`${label} Ledger: rename, SKU price and SKU archive applied; real cover thumbnail shown`);
  await context.close();

  // ---- anonymous buyer half (fresh context, no cookies, the published shop origin) --------------------------------------
  const buyerContext = await browser.newContext(ctxOpts({ignoreHTTPSErrors: true, viewport: vp}));
  const buyer = await buyerContext.newPage();
  buyer.on("pageerror", error => uiErrors.push(`${label} buyer ${error.name}: ${error.message}`));
  await buyer.goto(`${buyerOrigin}/${run.locale}`);
  // Storefront shell: the default design (no saved version) shows one product grid of the newest products.
  await expect(buyer.getByTestId("section-product-grid")).toBeVisible();
  const card = buyer.getByTestId("product-card").filter({hasText: newName});
  await expect(card).toHaveCount(1);
  await expect(buyer.getByTestId("product-card").filter({hasText: fx.old_name})).toHaveCount(0);
  const cover = card.locator("img");
  await expect(cover).toHaveAttribute("src", `/media/p/${fx.product_id}/${ids[0]}`);
  await cover.scrollIntoViewIfNeeded();
  // srcset density-corrects naturalWidth; inspect decoded bytes to retain the exact 64px (not 40px PNG) assertion.
  await expect.poll(() => cover.evaluate(async e => {
    if (!e.complete || e.naturalWidth === 0) return 0;
    const bitmap = await createImageBitmap(await (await fetch(e.currentSrc)).blob());
    const width = bitmap.width; bitmap.close(); return width;
  }), {timeout: 10000}).toBe(64);
  await expect(cover).toHaveAttribute("srcset", /\?w=360 64w$/); // actual pixel width, duplicates removed for this small source
  assert.match(await cover.evaluate(e => new URL(e.currentSrc).search), /^\?w=(360|720|1080)$/);
  await expect(card).toContainText(digits(fx.sku1.price_new));
  for (const stale of [fx.sku2.price, fx.sku1.price_old]) await expect(card).not.toContainText(digits(stale));
  const wire = async (src) => buyer.evaluate(async src => {
    const r = await fetch(src), b = await r.arrayBuffer();
    const h = [...new Uint8Array(await crypto.subtle.digest("SHA-256", b))].map(x => x.toString(16).padStart(2, "0")).join("");
    return {status: r.status, type: r.headers.get("content-type"), cache: r.headers.get("cache-control"), nosniff: r.headers.get("x-content-type-options"), sha: h};
  }, src);
  const cover0 = await wire(`/media/p/${fx.product_id}/${ids[0]}`), cover1 = await wire(`/media/p/${fx.product_id}/${ids[1]}`);
  assert.deepEqual([cover0.status, cover0.type, cover0.sha], [200, "image/jpeg", sha(jpg)], `${label} cover bytes`);
  assert.deepEqual([cover1.status, cover1.type, cover1.sha], [200, "image/png", sha(png)], `${label} second photo bytes`);
  for (const w of [cover0, cover1]) { assert.equal(w.cache, "public, max-age=86400, immutable"); assert.equal(w.nosniff, "nosniff"); }
  for (const width of [360, 720, 1080]) {
    const variant = await wire(`/media/p/${fx.product_id}/${ids[0]}?w=${width}`);
    assert.deepEqual([variant.status, variant.type, variant.cache], [200, "image/jpeg", "public, max-age=86400, immutable"]);
  }
  assert.equal((await wire(`/media/p/${fx.product_id}/${ids[0]}?w=640`)).status, 404);
  const stranger = await wire(`/media/p/${fx.product_id}/${crypto.randomUUID()}`);
  assert.equal(stranger.status, 404, `${label} unknown image is 404`);
  assert.equal(await buyer.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${label} home horizontal overflow`);
  await shot(buyer, "buyer-home", run);
  pass(`${label} home grid: renamed product, cover photo with the exact uploaded bytes, new lowest active price`);
  await card.click();
  // the card links to the slug page (the fixture product has the default slug = id prefix), never to the id URL
  await buyer.waitForURL(url => new RegExp(`^${buyerOrigin.replace(/[.]/g, "\\.")}/${run.locale}/products/(?![0-9a-f]{8}-[0-9a-f]{4}-)[a-z0-9-]+$`).test(url.toString()));
  await expect(buyer.getByRole("heading", {name: newName, exact: true})).toBeVisible();
  const gallery = buyer.getByTestId("product-gallery");
  await expect(gallery).toBeVisible();
  await expect(gallery.locator("img").first()).toHaveAttribute("src", `/media/p/${fx.product_id}/${ids[0]}`);
  const strip = gallery.locator(".sf-gal__strip img"), thumbs = gallery.locator(".sf-gal__thumbs button img");
  await expect(thumbs).toHaveCount(2);
  const expectedOrder = ids.map(id => `/media/p/${fx.product_id}/${id}`);
  assert.deepEqual(await strip.evaluateAll(list => list.map(e => e.getAttribute("src"))), expectedOrder, `${label} gallery strip order`);
  assert.deepEqual(await thumbs.evaluateAll(list => list.map(e => e.getAttribute("src"))), expectedOrder, `${label} gallery thumbnail order`);
  // second photo: thumbnail click where the thumbnails show (desktop), swipe-strip scroll on a phone (thumbnails are hidden there)
  const secondThumb = gallery.locator(".sf-gal__thumbs button").nth(1);
  if (await secondThumb.isVisible()) {
    await secondThumb.click();
    await expect(secondThumb).toHaveAttribute("aria-current", "true");
  } else {
    await strip.nth(1).scrollIntoViewIfNeeded();
    await expect(gallery.locator(".sf-gal__count")).toHaveText("2 / 2");
  }
  await expect(buyer.getByTestId("variant-price")).toHaveText(new Intl.NumberFormat(run.locale, {style: "currency", currency}).format(fx.sku1.price_new / 100));
  // The fixture SKUs have no option axes, so the page sells the product's one active variant: exactly sku1, never the archived sku2.
  await expect(buyer.getByRole("radio")).toHaveCount(0);
  await expect(buyer.getByTestId("product-buy")).toHaveAttribute("data-sku", fx.sku1.id);
  assert.equal(await buyer.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${label} product horizontal overflow`);
  await shot(buyer, "buyer-product", run);
  pass(`${label} product page: gallery in the new order, new price, archived SKU gone`);
  await buyerContext.close();
  return {product_id: fx.product_id, name: newName, image_ids: ids, locale: run.locale, viewport: run.vp};
}

const uiErrors = [];
try {
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path.join(certDir, "key.pem"), "-out", path.join(certDir, "cert.pem"), "-days", "1", "-subj", "/CN=buyer.example"], {stdio: "ignore"});
  if (!/^http:\/\/127\.0\.0\.1:\d+$/.test(process.env.LC_CM_CONTROL) || !process.env.LC_CM_CONTROL_KEY)
    throw new Error("LC_CM_CONTROL(+KEY) must name the Go-owned relay control; the admin no longer receives a freed explicit port");
  const [adminPort, buyerPort] = await Promise.all([startNext("admin"), startNext("storefront")]);
  await connectFixtureAdmin(adminPort, process.env.LC_CM_CONTROL, process.env.LC_CM_CONTROL_KEY);
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
  const results = [];
  const matrix = [{locale: "zh-TW", vp: "desktop"}, {locale: "zh-TW", vp: "mobile"}, {locale: "en", vp: "desktop"}, {locale: "en", vp: "mobile"}];
  for (const [index, run] of matrix.entries()) results.push(await scenario(index, run));
  assert.deepEqual(uiErrors, []);
  await writeFile(path.join(evidence, "screenshots.json"), JSON.stringify(shots, null, 2), {mode: 0o600});
  await writeFile(path.join(evidence, "result.json"), JSON.stringify({cases, results, boundary: "production Next; signed MOCK IdP; synthetic local TLS/CONNECT edge; publication + domain written through the migration 0081 definers (merchant publish, operator bind), not owner-seeded"}, null, 2), {mode: 0o600});
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
