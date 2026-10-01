// SFR gate (SANDBOX tier, REAL stack): the buyer storefront shell in a real browser against the production Next build, the real private
// buyerhttp handler and PG, with a shop the merchant built through the real admin API (products, option axes, stock, photos, collections, design
// saved + published + a preview draft, a delivery policy with a free-shipping threshold) and published through the migration 0081 definers.
// Started by tests/foundation/browser_storefront_test.go (TestBrowserStorefront, build tag browser) which owns PG and writes facts.json; this
// script never sees a database credential. A self-signed https edge preserves the virtual host shop.example (the BFF derives the store origin from
// Host) behind a CONNECT-only proxy. The MOCK sibling tests/storefront/shop-gate.mjs (SF01-SF12, fake API) stays for fast iteration.
// Env: LC_SFR_EVIDENCE, LC_SFR_FACTS (json), LC_SFR_CONTROL + LC_SFR_CONTROL_KEY (Go-only publication toggle), COMMERCE_BUYER_*,
// LC_BROWSER_ENGINE=chromium|webkit (tests/storefront/browser-engine.mjs).
import assert from "node:assert/strict";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { expect } from "@playwright/test";
import { launch, ctxOpts, iosZoomOffenders, engine } from "./browser-engine.mjs";
import { shopCopy } from "../../apps/storefront/lib/shop-copy.ts";

const env = (name) => { const v = process.env[name]; assert(v, `${name} is required`); return v; };
const root = process.cwd(), evidence = env("LC_SFR_EVIDENCE");
const facts = JSON.parse(await readFile(env("LC_SFR_FACTS"), "utf8"));
const HOST = new URL(facts.origin).host, origin = facts.origin;
const children = new Set(), sockets = new Set(), logs = [];
let browser, edge, proxy, cases = 0;
const pass = (name) => { cases++; console.log(`PASS ${name}`); };
async function listen(server) { server.listen(0, "127.0.0.1"); await once(server, "listening"); return server.address().port; }
const P = (key) => facts.products.find((p) => p.key === key);
const COPY = shopCopy.en;
// The storefront's one money format on every screen (lib/money.ts): "$4", cents only when there are cents ("$6.50"), never "$6.00".
const shop$ = (minor) => new Intl.NumberFormat("en", { style: "currency", currency: facts.currency, minimumFractionDigits: minor % 100 === 0 ? 0 : 2 }).format(minor / 100);
const num = (text) => Number(text.replace(/[^\d.]/g, ""));
const fmt = (template, values) => template.replace(/\{(\w+)\}/g, (_, k) => values[k]);

async function control(action) {
  const r = await fetch(`${env("LC_SFR_CONTROL")}/${action}`, { method: "POST", headers: { "X-Gate-Key": env("LC_SFR_CONTROL_KEY") } });
  assert.equal(r.status, 204, `control ${action}: ${r.status} ${await r.text()}`);
}

async function startNext() {
  const log = createWriteStream(path.join(evidence, `next-${Date.now()}.log`), { flags: "wx", mode: 0o600 });
  logs.push(log); await once(log, "open");
  const probe = net.createServer(); const port = await listen(probe); await new Promise((r) => probe.close(r));
  const bin = path.join(root, "apps/storefront/node_modules/next/dist/bin/next");
  const child = spawn(process.execPath, [bin, "start", "--hostname", "127.0.0.1", "--port", String(port)], {
    cwd: path.join(root, "apps/storefront"),
    env: { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1" },
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

const certDir = await mkdtemp(path.join(tmpdir(), "lc-shopreal-edge-"));
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
  const phone = () => browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true, locale: "en" }));
  const noOverflow = (p) => p.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
  const raw = (p, headers = {}) => relay(next.port, { url: p, method: "GET", headers: { host: HOST, ...headers } }, Buffer.alloc(0));
  const loaded = (locator) => locator.evaluate((img) => img.complete && img.naturalWidth > 0);

  const pc = await phone(), p = await pc.newPage();
  const problems = [];
  p.on("pageerror", (e) => problems.push(`pageerror: ${e.message}`));
  p.on("console", (m) => { if (m.type() === "error" && /hydrat|minified react|warning:|did not match/i.test(m.text())) problems.push(`console: ${m.text()}`); });

  // ---- SFR01 home from the published design the merchant saved through the admin API ------------------------------------------------------
  await p.goto(`${origin}/en`);
  await expect(p.getByTestId("announcement")).toContainText(facts.design.announcement);
  await expect(p.getByRole("heading", { level: 1 })).toHaveText(facts.design.hero);
  for (const id of ["section-hero", "section-featured", "section-rich-text", "section-product-grid", "section-image-text"]) await expect(p.getByTestId(id)).toBeVisible();
  await expect(p.getByTestId("section-rich-text").locator("strong")).toHaveText("at least a month");
  await expect(p.locator("footer").getByRole("link", { name: "LINE" })).toHaveAttribute("href", facts.design.line_url);
  assert.equal(await p.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue("--accent").trim()), facts.design.accent);
  await expect.poll(() => loaded(p.getByTestId("section-hero").locator("img").first())).toBe(true); // bytes served from PG through /media/s/{id}
  // newest first: the last created active product leads the grid; the featured rail holds exactly the collection's products
  await expect(p.getByTestId("section-product-grid").getByTestId("product-card").first()).toContainText("Everyday Item 24");
  assert.equal(await p.getByTestId("section-product-grid").getByTestId("product-card").count(), 8);
  const featured = await p.getByTestId("section-featured").getByTestId("product-card").allTextContents();
  assert.equal(featured.length, facts.collections.find((c) => c.slug === "home-fragrance").active_products);
  assert(featured.some((t) => t.includes(P("candle").title)) && featured.some((t) => t.includes(P("diffuser").title)));
  assert(await noOverflow(p), "home overflows at 390px"); assert.deepEqual(await iosZoomOffenders(p), []);
  pass("SFR01 home renders the five section types from the published design (photos from PG), nav-less phone layout, accent, footer contact, newest-first grid");

  // ---- SFR02 SEO surfaces on real data ----------------------------------------------------------------------------------------------------
  const home = (await raw("/en")).body.toString();
  assert(home.includes(facts.store_name) && /rel="canonical" href="[^"]*\/en"/.test(home) && /property="og:title"/.test(home) && !/noindex/.test(home), "home head");
  const ld = async (slug) => { const html = (await raw(`/en/products/${slug}`)).body.toString(); const m = /<script type="application\/ld\+json">([^<]+)<\/script>/.exec(html); assert(m, `${slug}: JSON-LD missing`); return { html, data: JSON.parse(m[1].replace(/\\u003c/g, "<")) }; };
  const candle = await ld(P("candle").slug);
  assert.equal(candle.data["@type"], "Product"); assert.equal(candle.data.offers["@type"], "AggregateOffer");
  assert.deepEqual([candle.data.offers.lowPrice, candle.data.offers.highPrice, candle.data.offers.priceCurrency, candle.data.offers.offerCount], ["68.00", "98.00", facts.currency, 2]);
  assert(candle.data.image[0].startsWith(`${origin}/media/p/`) && candle.html.includes(`property="og:image" content="${origin}/media/p/`), "absolute photo URLs");
  assert.equal(candle.data.offers.availability, "https://schema.org/InStock");
  const diffuser = await ld(P("diffuser").slug);
  assert.deepEqual([diffuser.data.offers["@type"], diffuser.data.offers.price, diffuser.data.offers.availability], ["Offer", "89.00", "https://schema.org/OutOfStock"]);
  const sitemap = await raw("/sitemap.xml"); assert.equal(sitemap.status, 200);
  const sm = sitemap.body.toString();
  for (const x of facts.products.filter((x) => x.status === "active")) assert(sm.includes(`/zh-TW/products/${x.slug}`), `sitemap lacks ${x.slug}`);
  for (const x of facts.products.filter((x) => x.status !== "active")) assert(!sm.includes(x.slug), `sitemap leaks a ${x.status} product: ${x.slug}`);
  for (const c of facts.collections) assert(sm.includes(`/zh-TW/collections/${c.slug}`));
  assert(sm.includes("/zh-TW/pages/about") && sm.includes('hreflang="en"'));
  const robots = (await raw("/robots.txt")).body.toString();
  assert(robots.includes(`Sitemap: ${origin}/sitemap.xml`) && robots.includes("Disallow: /*/cart") && robots.includes("Disallow: /*/checkout"));
  for (const private_ of ["/en/cart", "/en/checkout"]) assert(/noindex/.test((await raw(private_)).body.toString()), `${private_} must be noindex`);
  pass("SFR02 title/canonical/Open Graph, Product JSON-LD (real price range, availability, absolute photos), sitemap = active products only, robots, noindex on cart and checkout");

  // ---- SFR03 collections (photo via the amended id), sort, filter, pagination -----------------------------------------------------------------
  await p.goto(`${origin}/en/collections`);
  await expect(p.getByTestId("collection-tile")).toHaveCount(facts.collections.length);
  const withImage = facts.collections.find((c) => c.image_id), tile = p.getByTestId("collection-tile").filter({ hasText: withImage.title });
  await expect(tile.locator("img")).toHaveAttribute("src", `/media/c/${withImage.id}/${withImage.image_id}`);
  await expect.poll(() => loaded(tile.locator("img"))).toBe(true); // the collection photo comes from PG; the URL needs the id the read now returns
  await expect(p.getByTestId("collection-tile").filter({ hasText: "Tableware" }).locator("img")).toHaveCount(0);
  await expect(tile).toContainText(fmt(COPY.productCount, { n: withImage.active_products }));
  await p.goto(`${origin}/en/collections/tableware`);
  await expect(p.getByRole("heading", { level: 1 })).toHaveText("Tableware");
  assert.equal(await p.getByTestId("product-card").count(), facts.collections.find((c) => c.slug === "tableware").active_products);
  await p.getByLabel(COPY.sortBy).selectOption("price_asc");
  await p.waitForURL(/sort=price_asc/);
  const prices = (await p.getByTestId("product-card").locator(".sf-card__price > span:first-child").allTextContents()).map(num);
  assert(prices.length >= 3 && prices.every((n, i) => i === 0 || n >= prices[i - 1]), `not sorted ascending: ${prices}`);
  await p.goto(`${origin}/en/products?min=15&max=20`);
  const filtered = (await p.getByTestId("product-card").locator(".sf-card__price > span:first-child").allTextContents()).map(num);
  assert(filtered.length >= 3 && filtered.every((n) => n >= 15 && n <= 20), `price filter leaked ${filtered}`);
  await expect(p.locator(".sf-filter")).toHaveAttribute("open", "");
  await p.goto(`${origin}/en/products`);
  assert.equal(await p.getByTestId("product-card").count(), 24); // page size
  await p.getByTestId("load-more").click();
  await expect.poll(() => p.getByTestId("product-card").count()).toBe(facts.active_count);
  await expect(p.getByTestId("load-more")).toHaveCount(0);
  const titles = await p.getByTestId("product-card").locator(".sf-card__title").allTextContents();
  assert(!titles.includes(P("draft").title) && !titles.includes(P("retired").title), "draft/archived products must never be listed");
  pass("SFR03 collection tiles (photo via /media/c/{id}/{image} from PG), collection page, price sort, price filter, cursor load more to every active product, no draft/archived");

  // ---- SFR04 product pages: variants, stock hints, sold out, 404s, redirects, gallery ------------------------------------------------------------
  await p.goto(`${origin}/en/products/${P("candle").slug}`);
  await expect(p.getByRole("heading", { level: 1 })).toHaveText(P("candle").title);
  await expect(p.getByTestId("variant-price")).toHaveText(shop$(6800)); // opens on the cheapest in-stock variant
  await expect(p.getByTestId("stock-hint")).toContainText(COPY.inStock);
  await p.locator("label.sf-chip", { hasText: "300g" }).click();
  await expect(p.getByTestId("variant-price")).toHaveText(shop$(9800));
  await expect(p.getByTestId("stock-hint")).toContainText(COPY.lowStock); // 3 on hand
  assert(await noOverflow(p), "product overflows at 390px");
  await p.goto(`${origin}/en/products/${P("scarf").slug}`);
  await expect(p.getByRole("radio", { name: "Black" })).toBeDisabled(); // 0 on hand
  await expect(p.getByTestId("variant-price")).toHaveText(shop$(14800)); await expect(p.locator(".sf-buy__price s")).toHaveText(shop$(19800));
  const strip = p.getByTestId("product-gallery").locator(".sf-gal__strip img");
  assert.equal(await strip.count(), 3);
  await expect.poll(() => loaded(strip.first())).toBe(true);
  await p.goto(`${origin}/en/products/${P("diffuser").slug}`);
  await expect(p.getByTestId("add-to-cart")).toBeDisabled(); await expect(p.getByTestId("buy-now")).toBeDisabled(); await expect(p.getByTestId("stock-hint")).toContainText(COPY.outOfStock);
  const id = P("dripper").id;
  const adHop = await raw(`/products/${id}`); assert.equal(adHop.status, 308); assert.equal(adHop.headers.location, `/zh-TW/products/${id}`); // the frozen ad / feed link
  const moved = await raw(`/zh-TW/products/${id}`); assert.equal(moved.status, 308); assert.equal(moved.headers.location, `/zh-TW/products/${P("dripper").slug}`);
  for (const gone of [P("draft").slug, P("retired").slug, "no-such-product", "Checkout"]) assert.equal((await raw(`/en/products/${gone}`)).status, 404, `${gone} must be a 404`);
  await p.goto(`${origin}/en/products/${P("draft").slug}`); await expect(p.getByTestId("not-found")).toBeVisible();
  assert.equal((await raw(`/en/collections/nope`)).status, 404);
  pass("SFR04 variants and stock hints from real stock, sold-out and out-of-stock variants not addable, compare-at, 3-photo gallery from PG, id URL 308 -> slug, draft/archived/unknown/removed /products/Checkout are 404");

  // ---- SFR05 cart, free-shipping hint and the quote's shipping come from the real delivery policy ------------------------------------------------
  const { thresholdMinor: T, feeMinor: F } = { thresholdMinor: facts.free_shipping.threshold_minor, feeMinor: facts.free_shipping.fee_minor };
  const dripperPrice = P("dripper").variants[0].price_minor, candlePrice = P("candle").variants[0].price_minor;
  assert(candlePrice + dripperPrice < T && 2 * candlePrice + dripperPrice >= T, "fixture prices must straddle the threshold");
  const addProduct = async (page, key, quantity = 1) => {
    await page.goto(`${origin}/en/products/${P(key).slug}`);
    for (let i = 1; i < quantity; i++) await page.getByRole("button", { name: COPY.increase }).first().click();
    await page.getByTestId("add-to-cart").click();
    await page.getByTestId("cart-checkout").waitFor();
  };
  await addProduct(p, "candle"); await addProduct(p, "dripper");
  await p.goto(`${origin}/en/cart`);
  await expect(p.getByTestId("cart-line")).toHaveCount(2);
  const below = candlePrice + dripperPrice;
  await expect(p.getByTestId("cart-subtotal")).toHaveText(shop$(below));
  await expect(p.getByTestId("free-shipping")).toContainText(fmt(COPY.freeShipRemaining, { amount: shop$(T - below) }));
  await p.getByTestId("cart-checkout").click();
  await p.waitForURL(`**/en/checkout`);
  await p.getByRole("button", { name: "Choose delivery", exact: true }).click();
  await expect(p.getByLabel("Delivery method")).toBeVisible();
  await expect(p.getByTestId("checkout-free-shipping")).toContainText(fmt(COPY.freeShipRemaining, { amount: shop$(T - below) }));
  // the producer: every checkout-options row carries the policy threshold (number or null)
  const optionRows = await p.evaluate(async () => {
    const session = await (await fetch("/api/buyer/session", { cache: "no-store" })).json();
    const response = await fetch("/api/buyer/checkout-options?limit=100", { headers: { "X-Buyer-Context": session.context }, cache: "no-store" });
    return (await response.json()).items;
  });
  assert(optionRows.length >= 1 && optionRows.every((o) => o.free_shipping_threshold_minor === T), `options must carry the policy threshold ${T}: ${JSON.stringify(optionRows.map((o) => o.free_shipping_threshold_minor))}`);
  await p.getByRole("button", { name: "Get current total", exact: true }).click();
  await expect(p.locator(".quotation")).toBeVisible();
  await expect(p.locator(".quotation dl div").filter({ hasText: "Delivery" }).locator("dd")).toHaveText(shop$(F)); // below the threshold the quote charges the fee
  const pc2 = await phone(), q = await pc2.newPage();
  await addProduct(q, "candle", 2); await addProduct(q, "dripper");
  await q.goto(`${origin}/en/cart`);
  await expect(q.getByTestId("cart-subtotal")).toHaveText(shop$(2 * candlePrice + dripperPrice));
  await expect(q.getByTestId("free-shipping")).toContainText(COPY.freeShipReached);
  await q.getByTestId("cart-checkout").click();
  await q.waitForURL(`**/en/checkout`);
  await q.getByRole("button", { name: "Choose delivery", exact: true }).click();
  await expect(q.getByTestId("checkout-free-shipping")).toContainText(COPY.freeShipReached);
  await q.getByRole("button", { name: "Get current total", exact: true }).click();
  await expect(q.locator(".quotation")).toBeVisible();
  await expect(q.locator(".quotation dl div").filter({ hasText: "Delivery" }).locator("dd")).toHaveText(shop$(0)); // at or above the threshold shipping is free
  await expect(q.getByTestId("address-section")).toBeVisible();
  await pc2.close();
  pass("SFR05 cart subtotal and free-delivery hint from the real policy threshold (below: remaining amount; at/above: qualified), the quote charges the fee below it and 0 above it, options rows carry the threshold");

  // ---- SFR06 preview draft ------------------------------------------------------------------------------------------------------------------------
  const prev = await raw(`/en?preview=${facts.preview_token}`);
  assert(prev.status === 200 && /no-store/.test(prev.headers["cache-control"] ?? "") && /noindex/.test(prev.headers["x-robots-tag"] ?? ""), "preview headers");
  await p.goto(`${origin}/en?preview=${facts.preview_token}`);
  await expect(p.getByTestId("preview-banner")).toBeVisible(); await expect(p.getByRole("heading", { level: 1 })).toHaveText(facts.design.draft_hero);
  await p.goto(`${origin}/en?preview=${"Z".repeat(43)}`); // unknown token -> the published view, no banner
  await expect(p.getByTestId("preview-banner")).toHaveCount(0); await expect(p.getByRole("heading", { level: 1 })).toHaveText(facts.design.hero);
  assert(!(await raw("/en", { "x-shop-preview": facts.preview_token })).body.toString().includes(facts.design.draft_hero), "a client-sent x-shop-preview header must be ignored");
  pass("SFR06 preview token shows the saved draft with banner/no-store/noindex; unknown token and a forged header fall back to the published design");

  // ---- SFR07 publication through the 0081 definers --------------------------------------------------------------------------------------------------
  await control("unpublish");
  await p.goto(`${origin}/en`); await expect(p.getByTestId("store-closed")).toBeVisible();
  assert.equal((await raw(`/en/products/${P("candle").slug}`)).status, 404);
  assert((await raw("/robots.txt")).body.toString().includes("Disallow: /") && (await raw("/sitemap.xml")).status === 404);
  await control("publish");
  await p.goto(`${origin}/en`); await expect(p.getByRole("heading", { level: 1 })).toHaveText(facts.design.hero);
  assert.equal((await raw(`/en/products/${P("candle").slug}`)).status, 200);
  pass("SFR07 merchant unpublish through control.set_storefront_published closes the shop (branded closed page, 404 products, no sitemap) and republish reopens it");

  // ---- SFR08 locales ------------------------------------------------------------------------------------------------------------------------------------
  await p.goto(`${origin}/zh-CN/cart`); await expect(p.getByRole("heading", { level: 1 })).toHaveText(shopCopy["zh-CN"].cartTitle);
  await p.goto(`${origin}/en/products/${P("dripper").slug}`); await expect(p.getByTestId("add-to-cart")).toHaveText(COPY.addToCart); assert.equal(await p.evaluate(() => document.documentElement.lang), "en");
  assert.equal((await raw("/xx/products")).status, 404);
  pass("SFR08 UI chrome follows the locale while merchant text stays as entered; unknown locale is a 404");

  // ---- SFR09 checkout path and a clean client ------------------------------------------------------------------------------------------------------------
  assert.equal((await raw("/en/checkout")).status, 200);
  assert.deepEqual(problems, [], "client errors / hydration warnings across the whole phone walk-through");
  pass("SFR09 /{locale}/checkout is the checkout page; no page errors or hydration warnings across the walk-through");

  console.log(`engine=${engine} cases=${cases} REAL-STACK (SANDBOX)`);
} finally {
  for (const c of children) c.kill("SIGTERM");
  for (const s of sockets) s.destroy();
  await browser?.close().catch(() => {});
  edge?.close(); proxy?.close();
  for (const l of logs) l.end();
  await rm(certDir, { recursive: true, force: true });
}
