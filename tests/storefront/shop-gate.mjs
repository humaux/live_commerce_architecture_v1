// SF gate (MOCK tier): the buyer storefront shell in a real browser against the production Next build (or `next dev` with
// LC_SHOP_MODE=dev for iteration), a contract-shaped fake of the Go buyer API (tests/storefront/shop-fake-api.mjs, MOCK) and a
// self-signed https edge that preserves the virtual host shop.example (the BFF derives the store origin from Host). Not the real
// Go/PG: published-origin resolution, RLS, real stock and order placement are covered elsewhere (existing order/payment/CVS
// gates, and the tester's production-shape run once catalog-core + store-design are merged).
// Env: LC_SHOP_EVIDENCE (dir for logs, required), LC_SHOP_SHOTS (optional dir: screenshots of every page at 390 and 1280),
// LC_BROWSER_ENGINE=chromium|webkit (tests/storefront/browser-engine.mjs), LC_SHOP_MODE=start|dev.
import assert from "node:assert/strict";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { mkdtemp, readFile, rm, mkdir } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import { randomBytes } from "node:crypto";
import { tmpdir } from "node:os";
import path from "node:path";
import { expect } from "@playwright/test";
import { launch, ctxOpts, iosZoomOffenders, engine } from "./browser-engine.mjs";
import { createFakeApi, PREVIEW_TOKEN } from "./shop-fake-api.mjs";
import { reachCheckout } from "./shop-helpers.mjs";

const root = process.cwd();
const evidence = process.env.LC_SHOP_EVIDENCE;
assert(evidence, "LC_SHOP_EVIDENCE is required");
await mkdir(evidence, { recursive: true });
const shots = process.env.LC_SHOP_SHOTS || "";
if (shots) await mkdir(shots, { recursive: true });
const HOST = "shop.example", origin = `https://${HOST}`;
const mode = process.env.LC_SHOP_MODE === "dev" ? "dev" : "start";
const children = new Set(), sockets = new Set(), logs = [];
let browser, edge, proxy, cases = 0;
const pass = (name) => { cases++; console.log(`PASS ${name}`); };
const key = () => randomBytes(32).toString("base64url");
async function listen(server) { server.listen(0, "127.0.0.1"); await once(server, "listening"); return server.address().port; }
async function freePort() { const s = net.createServer(); const p = await listen(s); await new Promise((r) => s.close(r)); return p; }

const bffKey = key();
const api = createFakeApi({ bffKey });
const apiPort = await api.listen();

async function startNext() {
  const port = await freePort();
  const log = createWriteStream(path.join(evidence, `next-${Date.now()}.log`), { flags: "wx", mode: 0o600 });
  logs.push(log); await once(log, "open");
  const bin = path.join(root, "apps/storefront/node_modules/next/dist/bin/next");
  const args = mode === "dev" ? [bin, "dev", "--hostname", "127.0.0.1", "--port", String(port)] : [bin, "start", "--hostname", "127.0.0.1", "--port", String(port)];
  const child = spawn(process.execPath, args, {
    cwd: path.join(root, "apps/storefront"),
    env: { ...process.env, NODE_ENV: mode === "dev" ? "development" : "production", NEXT_TELEMETRY_DISABLED: "1", COMMERCE_BUYER_WEB_ENABLED: "1", COMMERCE_BUYER_API_ORIGIN: `http://127.0.0.1:${apiPort}`, COMMERCE_BUYER_BFF_KEY: bffKey, COMMERCE_BUYER_COOKIE_KEY: key(), COMMERCE_BUYER_SESSION_TTL: "3600" },
    stdio: ["ignore", log, log],
  });
  children.add(child);
  for (let i = 0; i < 400; i++) {
    if (child.exitCode !== null) throw new Error("Next exited before readiness");
    try { const r = await relay(port, { url: "/robots.txt", method: "GET", headers: { host: HOST } }, Buffer.alloc(0)); if (r.status === 200) return { port, child }; } catch { /* not up yet */ }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error("Next readiness timeout");
}
// Raw http keeps the virtual Host exactly as the browser sent it (Node fetch would rewrite it).
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

const certDir = await mkdtemp(path.join(tmpdir(), "lc-shop-edge-"));
try {
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path.join(certDir, "key.pem"), "-out", path.join(certDir, "cert.pem"), "-days", "1", "-subj", `/CN=${HOST}`], { stdio: "ignore" });
  const next = await startNext();
  edge = https.createServer({ key: await readFile(path.join(certDir, "key.pem")), cert: await readFile(path.join(certDir, "cert.pem")) }, async (req, res) => {
    try {
      const chunks = []; for await (const x of req) chunks.push(x);
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

  const phone = () => browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true, locale: "zh-TW" }));
  const desk = () => browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width: 1280, height: 900 }, locale: "zh-TW" }));
  const noOverflow = (p) => p.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
  // A Playwright fullPage capture is not what a buyer sees: it paints a sticky header at the scroll offset it happens to have (visual QA
  // finding 2), a fixed bottom bar at the first viewport's bottom edge instead of the end of the page (finding 1) and leaves lazy images
  // below the fold blank (finding 3). Walk the page so lazy images load, return to the top, and let the bottom bars sit in the flow, which
  // is where they rest when the buyer reaches the end. The real layout is asserted in SF11, never read off a picture.
  const shot = async (p, name, fullPage = true) => {
    if (!shots) return;
    const file = path.join(shots, `${name}.png`);
    if (!fullPage) { await p.screenshot({ path: file }); return; }
    await p.evaluate(async () => {
      for (let y = 0; y < document.documentElement.scrollHeight; y += innerHeight) { window.scrollTo(0, y); await new Promise((r) => setTimeout(r, 100)); }
      await Promise.all([...document.images].map((i) => (i.complete ? 0 : new Promise((done) => { i.onload = i.onerror = done; setTimeout(done, 4000); }))));
      window.scrollTo(0, 0);
    });
    await p.screenshot({ path: file, fullPage: true, style: ".purchase-footer, .sf-sticky { position: static !important; animation: none !important; }" });
  };
  const text = async (url, headers = {}) => { const r = await fetch(url, { headers }); return r; };
  // Node fetch cannot reach shop.example; raw relay with the virtual host instead.
  const raw = (p, headers = {}) => relay(next.port, { url: p, method: "GET", headers: { host: HOST, ...headers } }, Buffer.alloc(0));

  // ---- SF01 home from the published document ---------------------------------------------------------------------------
  const pc = await phone(), p = await pc.newPage();
  const problems = [];
  for (const page of [p]) { page.on("pageerror", (e) => problems.push(`pageerror: ${e.message}`)); page.on("console", (m) => { if (m.type() === "error" && /hydrat|minified react|warning:|did not match/i.test(m.text())) problems.push(`console: ${m.text()}`); }); }
  await p.goto(`${origin}/zh-TW`);
  await expect(p.getByTestId("announcement")).toContainText("滿 NT$1,500 免運");
  await expect(p.getByRole("heading", { level: 1 })).toHaveText("秋日餐桌，慢一點");
  await expect(p.getByTestId("section-hero")).toBeVisible();
  await expect(p.getByTestId("section-featured")).toBeVisible();
  await expect(p.getByTestId("section-rich-text").locator("strong")).toHaveText("至少一個月");
  await expect(p.getByTestId("section-product-grid").getByTestId("product-card").first()).toBeVisible();
  await expect(p.getByTestId("section-image-text")).toBeVisible();
  await expect(p.locator("footer").getByRole("link", { name: "LINE" })).toHaveAttribute("href", /line\.me/);
  await expect(p.locator("footer").getByRole("link", { name: "Facebook" })).toBeVisible();
  await expect(p.locator("footer").getByRole("link", { name: "Instagram" })).toBeVisible();
  assert.equal(await p.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue("--accent").trim()), "#2f6b5a");
  assert(await noOverflow(p), "home overflows at 390px");
  assert.deepEqual(await iosZoomOffenders(p), []);
  await p.waitForLoadState("networkidle"); await shot(p, "m-home");
  const dc = await desk(), d = await dc.newPage();
  await d.goto(`${origin}/zh-TW`);
  await expect(d.locator(".sf-nav").getByRole("link", { name: "居家香氛" })).toBeVisible();
  await expect(d.locator(".sf-menubtn")).toBeHidden();
  await d.waitForLoadState("networkidle"); await shot(d, "d-home");
  await p.getByTestId("menu-open").click(); await p.waitForTimeout(400); await shot(p, "m-menu", false); await p.keyboard.press("Escape");
  pass("SF01 home renders the five section types, announcement, nav, footer contact, accent variable (390 + 1280, no overflow, no iOS zoom)");

  // ---- SF02 SEO surfaces -----------------------------------------------------------------------------------------------
  const home = (await raw("/zh-TW")).body.toString();
  assert(/<title>[^<]*晨光選物/.test(home) && /rel="canonical" href="[^"]*\/zh-TW"/.test(home) && /property="og:title"/.test(home) && !/noindex/.test(home), "home head");
  const slug = "wool-knit-scarf";
  const prod = (await raw(`/zh-TW/products/${slug}`)).body.toString();
  const ld = /<script type="application\/ld\+json">([^<]+)<\/script>/.exec(prod);
  assert(ld, "product JSON-LD missing"); const data = JSON.parse(ld[1].replace(/\\u003c/g, "<"));
  assert.equal(data["@type"], "Product"); assert(data.offers.priceCurrency === "TWD" && data.image[0].startsWith(`${origin}/media/p/`));
  assert(/property="og:image" content="https:\/\/shop\.example\/media\/p\//.test(prod), "og:image must be absolute");
  assert(!/noindex/.test(prod));
  const sm = (await raw("/sitemap.xml")); assert.equal(sm.status, 200);
  const smText = sm.body.toString(); assert(smText.includes(`/zh-TW/products/${slug}`) && smText.includes("hreflang=\"en\"") && smText.includes("/zh-TW/collections/tableware"));
  api.state.hideProductSlug = "olive-board"; // a draft/archived product is absent from the Go list, so it must be absent here
  assert(!(await raw("/sitemap.xml")).body.toString().includes("olive-board")); api.state.hideProductSlug = null;
  const robots = (await raw("/robots.txt")).body.toString(); assert(robots.includes("Sitemap: https://shop.example/sitemap.xml") && robots.includes("Disallow: /*/cart"));
  const priv = (await raw("/zh-TW/cart")).body.toString(); assert(/noindex/.test(priv), "cart must be noindex");
  pass("SF02 title/canonical/Open Graph(absolute image)/Product JSON-LD, sitemap lists active products only, robots, noindex on cart");

  // ---- SF03 collections, sort, filter, load more -----------------------------------------------------------------------
  await p.goto(`${origin}/zh-TW/collections`);
  await expect(p.getByTestId("collection-tile")).toHaveCount(3); await shot(p, "m-collections");
  await p.goto(`${origin}/zh-TW/collections/tableware`);
  await expect(p.getByRole("heading", { level: 1 })).toHaveText("餐桌器皿");
  await p.getByLabel("排序").selectOption("price_asc");
  await p.waitForURL(/sort=price_asc/);
  const prices = await p.getByTestId("product-card").locator(".sf-card__price > span:first-child").allTextContents();
  const nums = prices.map((t) => Number(t.replace(/[^\d.]/g, "")));
  assert(nums.length >= 3 && nums.every((n, i) => i === 0 || n >= nums[i - 1]), `not sorted: ${prices}`);
  await p.goto(`${origin}/zh-TW/products?min=500&max=700`);
  const filtered = (await p.getByTestId("product-card").locator(".sf-card__price > span:first-child").allTextContents()).map((t) => Number(t.replace(/[^\d.]/g, "")));
  assert(filtered.length > 0 && filtered.every((n) => n >= 500 && n <= 700), `filter leaked ${filtered}`);
  await expect(p.locator(".sf-filter")).toHaveAttribute("open", "");
  await p.goto(`${origin}/zh-TW/products?limit=12`); // limit is honoured by Go; the page default is 24 so 30 products paginate
  await p.goto(`${origin}/zh-TW/products`);
  const first = await p.getByTestId("product-card").count();
  await p.getByTestId("load-more").click();
  await expect.poll(() => p.getByTestId("product-card").count()).toBeGreaterThan(first);
  await expect(p.getByTestId("load-more")).toHaveCount(0);
  await expect(p.getByTestId("product-card")).toHaveCount(30);
  await shot(p, "m-products");
  await d.goto(`${origin}/zh-TW/collections/home-fragrance`); await d.waitForLoadState("networkidle"); await shot(d, "d-collection");
  pass("SF03 collections grid, sort select, price filter keeps its inputs open, cursor load more reaches all 30 products");

  // ---- SF04 product page: variants, sold out, add two SKUs, cart edit, reload ---------------------------------------------
  await p.goto(`${origin}/zh-TW/products/cedar-fig-candle`);
  await expect(p.getByRole("heading", { level: 1 })).toHaveText("雪松無花果香氛蠟燭");
  const before = await p.getByTestId("variant-price").textContent();
  await p.getByRole("radio", { name: "300g" }).check({ force: true });
  await expect(p.getByTestId("variant-price")).not.toHaveText(before);
  await expect(p.getByTestId("stock-hint")).toContainText("僅剩少量");
  assert(await noOverflow(p), "product overflows at 390px"); assert.deepEqual(await iosZoomOffenders(p), []);
  await p.waitForLoadState("networkidle"); await shot(p, "m-product");
  await d.goto(`${origin}/zh-TW/products/cedar-fig-candle`); await d.waitForLoadState("networkidle"); await shot(d, "d-product");
  await p.goto(`${origin}/zh-TW/products/wool-knit-scarf`);
  await expect(p.getByRole("radio", { name: "墨黑" })).toBeDisabled(); // that colour is out of stock
  await p.goto(`${origin}/zh-TW/products/reed-diffuser-box`);
  await expect(p.getByTestId("add-to-cart")).toBeDisabled(); await expect(p.getByTestId("buy-now")).toBeDisabled(); await expect(p.getByTestId("stock-hint")).toContainText("已售完");
  // add two different SKUs
  await p.goto(`${origin}/zh-TW/products/cedar-fig-candle`);
  await p.getByTestId("add-to-cart").click();
  await expect(p.getByTestId("cart-drawer")).toBeVisible(); await expect(p.getByTestId("cart-line")).toHaveCount(1);
  await expect(p.getByTestId("cart-count")).toHaveText("1"); await p.waitForTimeout(400); await shot(p, "m-drawer", false);
  await p.getByRole("button", { name: "關閉購物車" }).click();
  await p.goto(`${origin}/zh-TW/products/ceramic-dripper-set`);
  await p.getByRole("button", { name: "增加數量" }).click();
  await p.getByTestId("add-to-cart").click();
  await expect(p.getByTestId("cart-line")).toHaveCount(2); await expect(p.getByTestId("cart-count")).toHaveText("3");
  await p.getByRole("link", { name: "查看購物車" }).click();
  await p.waitForURL(/\/zh-TW\/cart$/);
  await expect(p.getByTestId("cart-line")).toHaveCount(2);
  await expect(p.getByTestId("free-shipping")).toContainText("免運"); // MOCK: the fake options row carries the optional threshold
  await shot(p, "m-cart");
  const line = p.getByTestId("cart-line").first();
  const subtotal = await p.getByTestId("cart-subtotal").textContent();
  await line.getByRole("button", { name: "增加數量" }).click();
  await expect(p.getByTestId("cart-subtotal")).not.toHaveText(subtotal);
  await p.reload(); await expect(p.getByTestId("cart-line")).toHaveCount(2); // survives reload (capability cookie)
  await p.getByTestId("cart-line").last().getByRole("button", { name: /移出購物車/ }).click();
  await expect(p.getByTestId("cart-line")).toHaveCount(1);
  await d.goto(`${origin}/zh-TW/cart`); await d.waitForLoadState("networkidle");
  pass("SF04 variant chips resolve price/stock, sold-out cannot be added, two SKUs added, drawer, cart edit/remove, free-delivery hint (MOCK field), reload keeps cart");

  // ---- SF05 checkout hand-off to the existing order flow ----------------------------------------------------------------
  await p.getByTestId("cart-checkout").click();
  await p.waitForURL(/\/zh-TW\/checkout$/);
  await expect(p.getByTestId("cart-line")).toHaveCount(1);
  await p.getByRole("button", { name: "選擇配送" }).click();
  await expect(p.getByLabel("配送方式")).toBeVisible();
  await p.getByRole("button", { name: "取得目前總額" }).click();
  await expect(p.locator(".quotation")).toBeVisible();
  await shot(p, "m-checkout");
  await d.goto(`${origin}/zh-TW`); await reachCheckout(d, origin, "zh-TW", "ceramic-dripper-set", { quantity: 2 }); // the helper other gates use instead of the old product page
  await expect(d.getByTestId("cart-line")).toHaveCount(1); await expect(d.locator(".sf-line__unit", { hasText: "× 2" })).toBeVisible();
  await d.waitForTimeout(600); await shot(d, "d-checkout");
  pass("SF05 cart -> Checkout page -> delivery -> quotation reaches the existing order flow (order placement itself: existing gates)");

  // ---- SF06 search -------------------------------------------------------------------------------------------------------
  await p.goto(`${origin}/zh-TW/search?q=雪松`);
  await expect(p.getByTestId("product-card").first()).toContainText("雪松"); await shot(p, "m-search");
  await p.goto(`${origin}/zh-TW/search?q=wool-ign`);
  await expect(p.getByTestId("list-empty")).toBeVisible();
  await p.goto(`${origin}/zh-TW/search?q=CED-02`);
  await expect(p.getByTestId("product-card")).toHaveCount(1);
  pass("SF06 search by title and by SKU code; empty result state");

  // ---- SF07 404 / id redirect / draft -------------------------------------------------------------------------------------
  const id = api.products.find((x) => x.slug === "olive-board").id;
  const adHop = await raw(`/products/${id}`); assert.equal(adHop.status, 308); assert.equal(adHop.headers.location, `/zh-TW/products/${id}`); // the frozen ad/feed link (AL1 first hop)
  const moved = await raw(`/zh-TW/products/${id}`); assert.equal(moved.status, 308); assert.equal(moved.headers.location, "/zh-TW/products/olive-board");
  api.state.hideProductSlug = "olive-board";
  const gone = await raw(`/zh-TW/products/olive-board`); assert.equal(gone.status, 404);
  await p.goto(`${origin}/zh-TW/products/olive-board`); await expect(p.getByTestId("not-found")).toBeVisible(); await shot(p, "m-404");
  api.state.hideProductSlug = null;
  assert.equal((await raw(`/zh-TW/collections/nope`)).status, 404);
  pass("SF07 id URL redirects permanently to the slug, draft/hidden product and unknown collection are branded 404s");

  // ---- SF08 preview ------------------------------------------------------------------------------------------------------
  const prev = await raw(`/zh-TW?preview=${PREVIEW_TOKEN}`);
  assert(prev.status === 200 && /no-store/.test(prev.headers["cache-control"] ?? "") && /noindex/.test(prev.headers["x-robots-tag"] ?? ""), "preview headers");
  await p.goto(`${origin}/zh-TW?preview=${PREVIEW_TOKEN}`);
  await expect(p.getByTestId("preview-banner")).toContainText("預覽中"); await expect(p.getByRole("heading", { level: 1 })).toContainText("草稿");
  await p.getByRole("link", { name: "全部商品" }).first().click().catch(async () => { await p.getByTestId("menu-open").click(); await p.getByRole("link", { name: "全部商品" }).click(); });
  await p.waitForURL(/preview=/); await expect(p.getByTestId("preview-banner")).toBeVisible(); await shot(p, "m-preview");
  await p.goto(`${origin}/zh-TW?preview=${"Z".repeat(43)}`); // unknown/expired token -> the published view, no banner
  await expect(p.getByTestId("preview-banner")).toHaveCount(0); await expect(p.getByRole("heading", { level: 1 })).toHaveText("秋日餐桌，慢一點");
  assert(!(await raw("/zh-TW", { "x-shop-preview": PREVIEW_TOKEN })).body.toString().includes("草稿"), "a client-sent x-shop-preview header must be ignored");
  pass("SF08 preview token shows the draft with banner/no-store/noindex and keeps it across nav; unknown token and forged header fall back to published");

  // ---- SF09 locales ---------------------------------------------------------------------------------------------------------
  await p.goto(`${origin}/en/products/ceramic-dripper-set`);
  await expect(p.getByTestId("add-to-cart")).toHaveText("Add to cart"); assert.equal(await p.evaluate(() => document.documentElement.lang), "en");
  await p.goto(`${origin}/zh-CN/cart`); await expect(p.getByRole("heading", { level: 1 })).toHaveText("购物车");
  pass("SF09 UI chrome follows the locale (en, zh-CN) while merchant text stays as entered");

  // ---- SF10 unpublished / unknown host ----------------------------------------------------------------------------------------
  api.state.unpublished = true;
  await p.goto(`${origin}/zh-TW`); await expect(p.getByTestId("store-closed")).toBeVisible(); await shot(p, "m-closed");
  assert((await raw("/robots.txt")).body.toString().includes("Disallow: /") && (await raw("/sitemap.xml")).status === 404);
  api.state.unpublished = false;
  api.state.down = true; const down = await raw("/zh-TW"); assert.equal(down.status, 500); api.state.down = false;
  await p.goto(`${origin}/zh-TW`); await expect(p.getByTestId("section-hero")).toBeVisible();
  pass("SF10 unpublished store: branded closed 404, robots Disallow, no sitemap; upstream down -> 500 error state, recovers");

  // ---- SF11 visual-QA polish (k3 report; every finding was reproduced in this browser first, see the commit log for which were capture artifacts) --
  // Each numbered part reports its own failure so one run lists everything that is still wrong.
  const failures = [];
  const part = async (label, fn) => { try { await fn(); } catch (e) { failures.push(`${label}: ${String(e.message).split("\n").filter((l) => l.trim()).slice(0, 4).join(" / ").slice(0, 500)}`); } };
  const small = (loc) => loc.evaluateAll((els) => els.map((e) => { const b = e.getBoundingClientRect(); return `${Math.round(b.width)}x${Math.round(b.height)}`; }).filter((t) => { const [w, h] = t.split("x").map(Number); return w < 44 || h < 44; }));
  // Footer content a fixed/sticky bottom bar could hide: scroll to the end, nothing in the viewport may sit under the bar, and the last footer line must be in view.
  const footerCovered = async (pg) => {
    // The page height is still moving while cart lines and images load: wait until it is stable before looking at the end of the page.
    let last = -1; for (let i = 0; i < 40; i++) { const h = await pg.evaluate(() => document.documentElement.scrollHeight); if (h === last) break; last = h; await pg.waitForTimeout(150); }
    await pg.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight)); await pg.waitForTimeout(300);
    const r = await pg.evaluate(() => {
      const items = [...document.querySelectorAll(".sf-footer a, .sf-footer__base p")], last = document.querySelector(".sf-footer__base p").getBoundingClientRect();
      const bad = items.filter((el) => {
        const b = el.getBoundingClientRect(); if (b.bottom <= 0 || b.top >= innerHeight) return false;
        const hit = document.elementFromPoint(b.x + b.width / 2, Math.min(innerHeight - 1, Math.max(0, b.y + b.height / 2)));
        return !hit || !!hit.closest(".purchase-footer, .sf-sticky");
      }).map((el) => el.textContent.trim());
      return { n: items.length, bad, lastInView: last.top >= 0 && last.bottom <= innerHeight };
    });
    assert(r.n >= 10 && r.lastInView && r.bad.length === 0, `footer content under a bottom bar: ${JSON.stringify(r)}`);
  };
  const checkoutStep = async (pg) => { // cart -> delivery -> quotation, the same clicks as SF05
    await pg.goto(`${origin}/zh-TW/checkout`);
    await pg.getByRole("button", { name: "選擇配送" }).click(); await expect(pg.getByLabel("配送方式")).toBeVisible();
    await pg.getByRole("button", { name: "取得目前總額" }).click(); await expect(pg.locator(".quotation")).toBeVisible();
    await expect(pg.getByTestId("create-order")).toBeVisible();
  };
  const moneyTexts = (pg) => pg.evaluate(() => [...document.querySelectorAll(".sf-line__total, .sf-line__unit, [data-testid=cart-subtotal], .purchase-footer strong, .quotation li span:last-child, .quotation dd, .sf-summary__row strong")].map((e) => e.textContent.trim()).filter((t) => /\d/.test(t)));
  const noDecimals = async (pg, min, where) => { const t = await moneyTexts(pg); assert(t.length >= min, `${where}: only ${t.length} amounts found`); assert(!t.some((x) => /\d\.\d\d\b/.test(x)), `${where}: whole TWD amounts must not carry .00: ${t.join(" | ")}`); };

  await part("1 checkout bar 1280", async () => { await d.goto(`${origin}/zh-TW/checkout`); await d.locator(".purchase-footer").waitFor(); await footerCovered(d); });
  await part("1 checkout bar 390", async () => { await checkoutStep(p); await footerCovered(p); });
  const narrow = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width: 320, height: 568 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true, locale: "zh-TW" })), np = await narrow.newPage();
  await part("1 checkout bar 320", async () => {
    await np.goto(`${origin}/zh-TW/products/cedar-fig-candle`); await np.getByTestId("add-to-cart").click(); await np.getByTestId("cart-checkout").waitFor();
    await np.goto(`${origin}/zh-TW/checkout`); await np.locator(".purchase-footer").waitFor(); await footerCovered(np);
  });
  for (const [label, ctx] of [["320", np], ["390", p]]) await part(`1 product phone bar ${label}`, async () => {
    await ctx.goto(`${origin}/zh-TW/products/cedar-fig-candle`); await ctx.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight)); await expect(ctx.getByTestId("sticky-buy")).toBeVisible(); await footerCovered(ctx);
  });
  await narrow.close();

  // 2. one header, never repeated mid-page (the repeat in fullPage captures was the sticky header painted at the scroll offset)
  await part("2 one header", async () => {
    for (const url of ["/zh-TW/products/cedar-fig-candle", "/zh-TW/products", "/zh-TW/checkout"]) {
      await p.goto(`${origin}${url}`); await expect(p.locator("header.sf-header")).toBeVisible();
      assert.equal(await p.locator("header.sf-header").count(), 1, `${url}: exactly one site header`);
      assert.equal(await p.getByTestId("menu-open").count(), 1, `${url}: exactly one menu button`);
      assert.equal(await p.locator(".sf-header").evaluate((e) => getComputedStyle(e).position), "sticky", "the header sticks inside normal flow");
    }
  });

  // 3. a product without a photo shows a recognisable placeholder (icon + localized accessible label) on the card, the gallery and the cart line
  api.state.imagelessSlug = "olive-board";
  await part("3 placeholder card", async () => {
    await p.goto(`${origin}/zh-TW/products`);
    const ph = p.getByTestId("product-card").filter({ hasText: "橄欖木砧板" }).getByRole("img", { name: "無圖片" });
    await expect(ph).toBeVisible(); await expect(ph.locator("svg")).toHaveCount(1);
    await ph.scrollIntoViewIfNeeded(); await shot(p, "m-noimage-card", false);
  });
  await part("3 placeholder gallery", async () => {
    await p.goto(`${origin}/zh-TW/products/olive-board`);
    await expect(p.getByTestId("product-gallery").getByRole("img", { name: "無圖片" }).locator("svg")).toHaveCount(1);
    await shot(p, "m-noimage-product", false);
    await p.goto(`${origin}/en/products/olive-board`);
    await expect(p.getByTestId("product-gallery").getByRole("img", { name: "No image" }).locator("svg")).toHaveCount(1);
  });
  await part("3 placeholder cart line", async () => {
    await p.goto(`${origin}/zh-TW/products/olive-board`);
    await p.getByTestId("add-to-cart").click(); await expect(p.getByTestId("cart-drawer")).toBeVisible();
    await p.getByRole("link", { name: "查看購物車" }).click(); await p.waitForURL(/\/zh-TW\/cart$/);
    const line = p.getByTestId("cart-line").filter({ hasText: "橄欖木砧板" });
    try { await expect(line.getByRole("img", { name: "無圖片" }).locator("svg")).toHaveCount(1); await shot(p, "m-noimage-cartline", false); }
    finally { await line.getByRole("button", { name: /移出購物車/ }).click(); await expect(line).toHaveCount(0); } // leave the cart as the next parts expect it
  });
  api.state.imagelessSlug = null;

  // 4 + 7. one money format on the cart page, 44x44 targets for the remove link, the quantity buttons and the variant chips
  await part("4 cart money", async () => { await p.goto(`${origin}/zh-TW/cart`); await expect(p.getByTestId("cart-line")).toHaveCount(1); await noDecimals(p, 3, "cart"); });
  await part("7 cart targets", async () => {
    assert.equal(await p.locator(".sf-link-btn").count(), 1);
    assert.deepEqual(await small(p.locator(".sf-link-btn, .sf-stepper button")), [], "cart remove link and quantity buttons are at least 44x44");
  });
  await part("7 chips", async () => {
    await p.goto(`${origin}/zh-TW/products/cedar-fig-candle`);
    await expect(p.locator("label.sf-chip")).toHaveCount(2); assert.deepEqual(await small(p.locator("label.sf-chip")), [], "variant chips are at least 44x44");
  });

  await part("4 checkout money", async () => { await checkoutStep(p); await noDecimals(p, 6, "checkout"); });

  // 5. a disabled create-order button is visibly different from the action colour and says why (the pale teal read as an enabled button gone wrong)
  await part("5 disabled CTA", async () => {
    const create = p.getByTestId("create-order");
    await expect(create).toBeDisabled();
    const look = await create.evaluate((e) => { const s = getComputedStyle(e), hint = document.getElementById(e.getAttribute("aria-describedby") ?? "-"); return { opacity: s.opacity, bg: s.backgroundColor, hint: hint ? hint.textContent.trim() : "", hintShown: !!hint && hint.getBoundingClientRect().height > 0 }; });
    assert.equal(look.opacity, "1", "disabled is a distinct grey, the action colour is never just faded"); assert.notEqual(look.bg, "rgb(36, 121, 101)", "disabled must not wear the action teal");
    assert(look.hint.length > 6 && look.hintShown, `the disabled create-order button needs a visible description: ${JSON.stringify(look)}`);
    await create.scrollIntoViewIfNeeded(); await shot(p, "m-checkout-disabled-cta", false);
  });

  // 6. closed store: no shopping chrome, the brand fallback is localized, no unexplained dot
  api.state.unpublished = true;
  await part("6 closed store", async () => {
    for (const [loc, brand] of [["zh-TW", "商店"], ["zh-CN", "商店"], ["en", "Store"]]) {
      await p.goto(`${origin}/${loc}`); await expect(p.getByTestId("store-closed")).toBeVisible();
      assert.equal((await p.locator(".sf-brand").innerText()).trim(), brand, `${loc} closed brand`);
      assert.equal(await p.locator(".sf-header .sf-search, .sf-header .sf-searchlink, [data-testid=header-cart]").count(), 0, `${loc} closed store must not offer search or cart`);
      assert.equal(await p.locator(".sf-empty__code").count(), 0, `${loc} closed store: no decorative dot`);
      if (loc === "zh-TW") await shot(p, "m-closed-localized", false);
    }
  });
  api.state.unpublished = false;

  // 8. desktop home rail: the clipped last card is announced by an edge fade and prev/next buttons, and the buttons reach the last card
  await part("8 rail", async () => {
    await d.goto(`${origin}/zh-TW`); await d.waitForLoadState("networkidle");
    const rail = d.getByTestId("rail");
    await expect(rail).toHaveAttribute("data-more-end", "true"); await expect(rail).toHaveAttribute("data-more-start", "false");
    await expect(d.getByTestId("rail-next")).toBeVisible(); await expect(d.getByTestId("rail-prev")).toBeHidden();
    await rail.scrollIntoViewIfNeeded(); await shot(d, "d-home-rail", false);
    for (let i = 0; i < 6 && (await rail.getAttribute("data-more-end")) === "true"; i++) { await d.getByTestId("rail-next").click(); await d.waitForTimeout(500); }
    await expect(rail).toHaveAttribute("data-more-end", "false"); await expect(d.getByTestId("rail-prev")).toBeVisible(); await expect(d.getByTestId("rail-next")).toBeHidden();
    assert(await rail.evaluate((e) => { const ul = e.querySelector("ul"); return ul.lastElementChild.getBoundingClientRect().right <= ul.getBoundingClientRect().right + 1; }), "the last card is fully visible at the end of the rail");
    await p.goto(`${origin}/zh-TW`); await expect(p.getByTestId("rail-next")).toBeHidden(); // phones swipe: the next card already peeks
  });
  assert.deepEqual(failures, [], "visual-QA polish still failing:\n" + failures.join("\n"));
  pass("SF11 visual-QA polish: footer never covered (checkout bar 1280/390/320, product phone bar 320/390), one header, image placeholders with icon+label, one money format, 44px targets, disabled CTA described, closed store chrome, home rail affordance");

  assert.deepEqual(problems, [], "client errors / hydration warnings on the phone page");
  pass("SF12 no page errors or hydration warnings across the whole phone walk-through");

  console.log(`engine=${engine} cases=${cases} mode=${mode} MOCK`);
} finally {
  for (const c of children) { c.kill("SIGTERM"); }
  for (const s of sockets) s.destroy();
  await browser?.close().catch(() => {});
  edge?.close(); proxy?.close(); await api.close();
  for (const l of logs) l.end();
  await rm(certDir, { recursive: true, force: true });
}
