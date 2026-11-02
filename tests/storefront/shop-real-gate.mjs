// Purpose: shop-real-gate.mjs startup uses bounded shared bind-race recovery; gate assertions are unchanged.
// Depends on: tests/helpers/next-startup.mjs, existing app/fixture/edge imports below.
// Used by: its scripts/dev/test-local.sh browser mode; GATE-PORT acceptance.
// SFR gate (SANDBOX tier, REAL stack): the buyer storefront shell in a real browser against the production Next build, the real private
// buyerhttp handler and PG, with a shop the merchant built through the real admin API (products, option axes, stock, photos, collections, design
// saved + published + a preview draft, a delivery policy with a free-shipping threshold) and published through the migration 0081 definers.
// Started by tests/foundation/browser_storefront_test.go (TestBrowserStorefront, build tag browser) which owns PG and writes facts.json; this
// script never sees a database credential. A self-signed https edge preserves the virtual host shop.example (the BFF derives the store origin from
// Host) behind a CONNECT-only proxy. The MOCK sibling tests/storefront/shop-gate.mjs (SF01-SF12, fake API) stays for fast iteration.
// Env: LC_SFR_EVIDENCE, LC_SFR_FACTS (json), LC_SFR_CONTROL + LC_SFR_CONTROL_KEY (Go-only publication toggle), COMMERCE_BUYER_*,
// LC_BROWSER_ENGINE=chromium|webkit (tests/storefront/browser-engine.mjs).
import { startNextWithPortRetry, nextAttemptLog } from "../helpers/next-startup.mjs";
import assert from "node:assert/strict";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { expect } from "@playwright/test";
import { launch, ctxOpts, iosZoomOffenders, engine } from "./browser-engine.mjs";
import { shopCopy } from "../../apps/storefront/lib/shop-copy.ts";
import { measureMediaSizes } from "./media-sizes-perf.mjs";
import { checkOfflineError } from "./offline-error.spec.ts";
import { checkBuyerBrowseParity } from "./buyer-browse-parity.spec.ts";

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
  return startNextWithPortRetry({}, async ({ port, attempt, track }) => {
    const log = createWriteStream(nextAttemptLog(path.join(evidence, `next-${Date.now()}.log`), attempt), { flags: "wx", mode: 0o600 });
    logs.push(log); await once(log, "open");
    const bin = path.join(root, "apps/storefront/node_modules/next/dist/bin/next");
    const child = spawn(process.execPath, [bin, "start", "--hostname", "127.0.0.1", "--port", String(port)], {
      cwd: path.join(root, "apps/storefront"),
      env: { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1" },
      stdio: ["ignore", log, log],
    });
    track(child, log);
    children.add(child);
    for (let i = 0; i < 400; i++) {
      if (child.exitCode !== null) throw new Error("Next exited before readiness");
      try { const r = await relay(port, { url: "/robots.txt", method: "GET", headers: { host: HOST } }, Buffer.alloc(0)); if (r.status === 200) return { port, child }; } catch { /* not up yet */ }
      await new Promise((r) => setTimeout(r, 100));
    }
    throw new Error("Next readiness timeout");

  });
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
  await measureMediaSizes({ browser, origin, evidence, ctxOpts });
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
  const railPhoto = p.getByTestId("section-featured").getByTestId("product-card").first().locator("img");
  await railPhoto.scrollIntoViewIfNeeded(); await expect.poll(() => loaded(railPhoto)).toBe(true);
  assert.match(await railPhoto.evaluate(img => new URL(img.currentSrc).search), /^\?w=(360|720|1080)$/);
  await expect(railPhoto).toHaveAttribute("sizes", /58vw/);
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
  // This is the real phone context: use the mobile ordering page; keep the existing price oracle below.
  await p.getByRole("button", { name: COPY.sortBy, exact: true }).click();
  await p.getByRole("button", { name: COPY.sortPriceAsc, exact: true }).click();
  await p.waitForURL(/sort=price_asc/);
  const prices = (await p.getByTestId("product-card").locator(".sf-card__price > span:first-child").allTextContents()).map(num);
  assert(prices.length >= 3 && prices.every((n, i) => i === 0 || n >= prices[i - 1]), `not sorted ascending: ${prices}`);
  await p.goto(`${origin}/en/products?min=15&max=20`);
  const filtered = (await p.getByTestId("product-card").locator(".sf-card__price > span:first-child").allTextContents()).map(num);
  assert(filtered.length >= 3 && filtered.every((n) => n >= 15 && n <= 20), `price filter leaked ${filtered}`);
  // React streaming can briefly keep a second, hidden copy of the segment (<div hidden id="S:…">): wait for the swap to leave
  // exactly one (a lasting duplicate still fails) before the strict-mode assertion below.
  await expect(p.locator(".sf-filter")).toHaveCount(1);
  await expect(p.locator(".sf-filter")).toHaveAttribute("open", "");
  await p.goto(`${origin}/en/products`);
  // React streaming briefly keeps the finished segment in a hidden <div hidden id="S:…"> before swapping it in; only the visible chips are the UI.
  // Match shop-r5-checks: measure the 390px chip layout after loading/streaming settles.
  await p.waitForLoadState("networkidle");
  const categoryChips = p.getByTestId("collection-chips").filter({ visible: true });
  await expect(categoryChips).toBeVisible();
  // After the streaming swap exactly one copy remains; a lasting (even hidden) duplicate still fails.
  await expect(p.getByTestId("collection-chips")).toHaveCount(1);
  assert(await categoryChips.evaluate(el => el.scrollWidth <= el.clientWidth), "G4 chips scroll horizontally at 390px");
  assert(await noOverflow(p), "G4 list overflows at 390px");
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
  const related = p.getByTestId("related-products");
  await expect(related).toBeVisible();
  await expect(related.locator(`a[href$="/products/${P("diffuser").slug}"]`)).toHaveCount(1);
  for (const key of ["candle", "draft", "retired"]) await expect(related.locator(`a[href$="/products/${P(key).slug}"]`)).toHaveCount(0);
  await expect(p.getByTestId("variant-price")).toHaveText(shop$(6800)); // opens on the cheapest in-stock variant
  await expect(p.getByTestId("stock-hint")).toContainText(COPY.inStock);
  await p.locator("label.sf-chip", { hasText: "300g" }).click();
  await expect(p.getByTestId("variant-price")).toHaveText(shop$(9800));
  await expect(p.getByTestId("stock-hint")).toContainText(COPY.lowStock); // 3 on hand
  assert(await noOverflow(p), "product overflows at 390px");
  await p.goto(`${origin}/en/products/${P("scarf").slug}`);
  await expect(p.getByTestId("related-products")).toHaveCount(0);
  await expect(p.getByRole("radio", { name: "Black" })).toBeDisabled(); // 0 on hand
  await expect(p.getByTestId("variant-price")).toHaveText(shop$(14800)); await expect(p.locator(".sf-buy__price s")).toHaveText(shop$(19800));
  const strip = p.getByTestId("product-gallery").locator(".sf-gal__strip img");
  assert.equal(await strip.count(), 3);
  await expect.poll(() => loaded(strip.first())).toBe(true);
  assert.match(await strip.first().evaluate(img => new URL(img.currentSrc).search), /^\?w=(360|720|1080)$/);
  const enlarge = p.getByRole("button", { name: "Enlarge photo" });
  await enlarge.click();
  const photoDialog = p.locator(".sf-photo-dialog");
  await expect(photoDialog).toBeVisible();
  await p.keyboard.press("ArrowRight");
  await expect(photoDialog.locator("img")).toHaveAttribute("src", await strip.nth(1).getAttribute("src"));
  await p.keyboard.press("Escape");
  await expect(photoDialog).not.toBeVisible();
  await expect(enlarge).toBeFocused();
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
  await p.getByTestId("checkout-details-toggle").click();
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
  await q.getByTestId("checkout-details-toggle").click();
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

  await checkOfflineError({ browser, origin, control }); // OC-11: logged, not counted in `cases`
  await checkBuyerParityStates({ browser, origin, evidence });
  await checkBuyerBrowseParity({ browser, origin, evidence, facts });

  console.log(`engine=${engine} cases=${cases} REAL-STACK (SANDBOX)`);
} finally {
  for (const c of children) c.kill("SIGTERM");
  for (const s of sockets) s.destroy();
  await browser?.close().catch(() => {});
  edge?.close(); proxy?.close();
  for (const l of logs) l.end();
  await rm(certDir, { recursive: true, force: true });
}

// PARITY-STATES: invoked above by the registered storefront gate; synthetic BFF reads only.
async function checkBuyerParityStates({ browser, origin, evidence }) {
  const copy = {
    "zh-TW": {
      empty: "購物車還沒有商品", emptyBody: "先挑選想買的商品，再回到購物車確認數量與配送方式。", emptyNote: "選好商品後，可在購物車確認數量。", browse: "瀏覽商品",
      forbidden: "目前無法開啟此頁", forbiddenBody: "此連結無法開啟，或目前購物連線無法使用這項功能。請從商店重新開始。", forbiddenNote: "如需查看訂單，請使用商店的訂單查詢。", home: "返回商店",
      offline: "暫時無法載入購物車", offlineBody: "請檢查網路連線，再重新載入。這次只讀取購物車，不會送出訂單。", offlineNote: "商品與數量以原購物連線重新讀取的結果為準。", retry: "重新載入", failed: "目前仍無法連線。請保留原頁，稍後重新載入。",
      expired: "訂單連結已失效", expiredBody: "這個連結無法再使用。你可以使用訂單編號查詢，或聯絡下單的商店。", lookup: "前往訂單查詢", expiredGo: "前往查詢訂單",
      unknown: "付款結果尚未確認", unknownBody: "請先查詢這筆訂單的付款狀態。確認前請勿再次付款。", check: "查詢狀態", unknownFailed: "目前無法讀取最新狀態。請稍後查詢同一筆訂單。",
      lookupSubmit: "查詢訂單", lookupFailed: "暫時無法查詢，請稍後再試。", lookupNoMatch: "找不到符合的訂單，請確認訂單編號與電子郵件或手機號碼。",
    },
    "zh-CN": {
      empty: "购物车还没有商品", emptyBody: "先挑选想买的商品，再回到购物车确认数量与配送方式。", emptyNote: "选好商品后，可在购物车确认数量。", browse: "浏览商品",
      forbidden: "暂时无法打开此页面", forbiddenBody: "此链接无法打开，或当前购物会话无法使用此功能。请从商店重新开始。", forbiddenNote: "如需查看订单，请使用商店的订单查询。", home: "返回商店",
      offline: "暂时无法加载购物车", offlineBody: "请检查网络连接，再重新加载。本次只读取购物车，不会提交订单。", offlineNote: "商品与数量以当前购物会话重新读取的结果为准。", retry: "重新加载", failed: "暂时仍无法连接。请保留当前页面，稍后重新加载。",
      expired: "订单链接已失效", expiredBody: "这个链接无法再使用。你可以使用订单编号查询，或联系下单的商店。", lookup: "前往订单查询", expiredGo: "前往查询订单",
      unknown: "付款结果尚未确认", unknownBody: "请先查询这笔订单的付款状态。确认前请勿再次付款。", check: "查询状态", unknownFailed: "暂时无法读取最新状态。请稍后查询同一笔订单。",
      lookupSubmit: "查询订单", lookupFailed: "暂时无法查询，请稍后再试。", lookupNoMatch: "找不到匹配的订单，请确认订单编号与邮箱或手机号码。",
    },
    en: {
      empty: "Your cart is empty", emptyBody: "Choose products first, then return to your cart to confirm quantities and delivery.", emptyNote: "You can check quantities in your cart after choosing products.", browse: "Browse products",
      forbidden: "This page cannot be opened", forbiddenBody: "This link cannot be opened, or your shopping session cannot use this feature. Start again from the store.", forbiddenNote: "Use the store’s order lookup to view an order.", home: "Return to store",
      offline: "Your cart could not be loaded", offlineBody: "Check your connection and reload. This only reads your cart and does not place an order.", offlineNote: "Products and quantities come from re-reading your original shopping session.", retry: "Reload", failed: "We still cannot connect. Keep this page open and reload later.",
      expired: "This order link has expired", expiredBody: "This link can no longer be used. Look the order up with its number, or contact the store you ordered from.", lookup: "Go to order lookup", expiredGo: "Go to order lookup",
      unknown: "Your payment result is not confirmed", unknownBody: "Check this order’s payment status first. Do not pay again until it is confirmed.", check: "Check status", unknownFailed: "The latest status could not be loaded. Check the same order again later.",
      lookupSubmit: "Find order", lookupFailed: "Lookup is unavailable right now. Please try again later.", lookupNoMatch: "We could not find a matching order. Check the order number and the email or phone number.",
    },
  };
  const uid = n => `00000000-0000-0000-0000-${String(n).padStart(12, "0")}`;
  const orderID = uid(61);
  const active = { state: "active", context: "a".repeat(43), expires_at: "2099-01-01T00:00:00Z" };
  const cart = { id: uid(62), currency: "TWD", version: 1, items: [] };
  // Contract-shaped synthetic order: no real recipient, address, contact, token, key or provider URL.
  const order = {
    order_id: orderID, cart_id: cart.id, cart_version: 1, commercial_state: "AWAITING_PAYMENT", fulfillment_state: "MANUAL_UNASSIGNED",
    snapshot: {
      quote: { currency: "TWD", lines: [{ sku_id: uid(63), name: "Synthetic state item", code: "STATE-TEST", quantity: 1, unit_price_minor: 262000 }], amount: { subtotal_minor: 262000, discount_minor: 0, shipping_minor: 0, shipping_tax_minor: 0, tax_minor: 0, total_minor: 262000 } },
      destination: { kind: "home", country: "TW", recipient_name: "Synthetic Recipient", phone: "+886900000091", home_address: { region: "", city: "Synthetic City", postal_code: "", line1: "Synthetic Street", line2: "" } },
      service: { code: "home", name_hans: "测试配送", name_hant: "測試配送", name_en: "Synthetic delivery", delivery_kind: "home", mode: "MANUAL" },
    },
  };
  const payment = { order_id: orderID, currency: "TWD", total_minor: 262000, commercial_state: "AWAITING_PAYMENT", test_mode: true, payment_state: "PENDING", handoff_state: "CREATING", handoff_expires_at: "2099-01-01T00:00:00Z", methods: [], cancel_requested: false };

  const dir = path.join(evidence, "parity");
  await mkdir(dir, { recursive: true });
  const ledger = [], failures = [];
  for (const [locale, c] of Object.entries(copy)) {
    for (const width of [1440, 390]) {
      for (const kind of ["empty", "offline", "forbidden", "cart-forbidden", "order-offline", "expired", "unknown", "lookup"]) {
        const context = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height: width === 1440 ? 900 : 844 }, locale }));
        const page = await context.newPage();
        const label = `${kind}-${locale}-${width}`;
        const writes = [], reads = [];
        let cartFailed = kind === "offline", paymentFailed = false, lookupFailed = true;
        let cartReadBarrier = Promise.resolve();
        page.on("request", request => {
          if (request.url().includes("/api/buyer/")) {
            if (request.method() === "GET") reads.push(new URL(request.url()).pathname);
            else writes.push({ method: request.method(), path: new URL(request.url()).pathname });
          }
        });
        // MOCK fixture setup at existing BFF boundaries. These are response data, never user actions or DOM changes.
        await context.route("**/api/buyer/session", route => route.fulfill({ json: active }));
        await context.route("**/api/buyer/cart", async route => {
          // MOCK read latency only: keep the original response fixtures and command boundaries.
          await cartReadBarrier;
          return kind === "cart-forbidden" ? route.fulfill({ status: 403, json: { code: "forbidden" } }) : cartFailed ? route.fulfill({ status: 503, json: { code: "unavailable" } }) : route.fulfill({ json: cart });
        });
        await context.route(`**/api/buyer/orders/${orderID}`, route => kind === "forbidden" ? route.fulfill({ status: 404, json: { code: "not_found" } }) : kind === "order-offline" ? route.fulfill({ status: 503, json: { code: "unavailable" } }) : route.fulfill({ json: order }));
        await context.route(`**/api/buyer/orders/${orderID}/payment`, route => paymentFailed ? route.fulfill({ status: 503, json: { code: "unavailable" } }) : route.fulfill({ json: payment }));
        await context.route("**/api/buyer/orders/lookup", route => route.fulfill({ status: lookupFailed ? 503 : 404, json: { code: lookupFailed ? "unavailable" : "not_found" } }));
        const shot = async frame => {
          if (locale === "zh-TW") await page.screenshot({ path: path.join(dir, `${frame}.png`) });
        };
        try {
          if (kind === "lookup") {
            await page.goto(`${origin}/${locale}/orders/lookup`);
            await page.getByTestId("lookup-ref").fill("0123-ABCD-4567");
            await page.getByTestId("lookup-contact").fill("state-gate@example.invalid");
            await page.getByRole("button", { name: c.lookupSubmit, exact: true }).click();
            const error = page.getByTestId("lookup-problem");
            await expect(error).toContainText(c.lookupFailed);
            await expect(error).toHaveJSProperty("tagName", "DIV"); // InlineError replaces the legacy bare paragraph.
            await expect(error.getByRole("alert")).toBeVisible();
            await expect(page.getByTestId("lookup-ref")).toHaveValue("0123-ABCD-4567");
            lookupFailed = false;
            await page.getByRole("button", { name: c.lookupSubmit, exact: true }).click();
            await expect(error).toContainText(c.lookupNoMatch);
            await page.reload();
            await expect(page.getByTestId("lookup-ref")).toHaveValue("");
            await expect(page.getByTestId("lookup-contact")).toHaveValue("");
          } else if (kind === "cart-forbidden") {
            await page.goto(`${origin}/${locale}/cart`);
            const panel = page.locator('section[data-variant="forbidden"]');
            await expect(panel.getByRole("heading", { name: c.forbidden, exact: true })).toBeVisible();
            await expect(page.getByRole("heading", { name: c.empty, exact: true })).toHaveCount(0);
            await panel.getByRole("link", { name: c.home, exact: true }).click();
            await expect(page).toHaveURL(new RegExp(`/${locale}/?$`));
          } else if (kind === "order-offline") {
            await page.goto(`${origin}/${locale}/orders/${orderID}`);
            const panel = page.locator('section[data-variant="offline"]');
            await expect(panel).toBeVisible();
            await panel.getByRole("button", { name: c.retry, exact: true }).click();
            await expect(panel.getByRole("alert")).toBeVisible();
            await expect(page).toHaveURL(`${origin}/${locale}/orders/${orderID}`);
            await page.reload();
            await expect(panel).toBeVisible();
          } else if (kind === "unknown") {
            await page.goto(`${origin}/${locale}/orders/${orderID}`);
            // P1: routine PENDING state (CREATING) with no failed read must NOT show the unknown panel
            await expect(page.locator('section[data-variant="unknown"]')).toHaveCount(0);
            await expect(page.getByTestId("pay-order")).toBeVisible();
            await expect(page.locator(".sf-empty")).toHaveCount(0);
            await expect(page.getByText(/403\s*·/)).toHaveCount(0);
            // Real failed read: status read fails -> unknown panel is PRESENT
            paymentFailed = true;
            await page.getByTestId("refresh-order").click();
            const panel = page.locator('section[data-variant="unknown"]');
            await expect(panel.getByRole("heading", { name: c.unknown, exact: true })).toBeVisible();
            await expect(panel).toContainText(c.unknownBody);
            await expect(page.getByTestId("pay-order")).toHaveCount(0);
            const frames = { unknown: ["1429-35308", "1429-35331"] };
            await shot(frames.unknown[width === 1440 ? 0 : 1]);
            await expect(page.getByTestId("order-page")).toContainText("2,620");
            const before = reads.filter(url => url.endsWith("/payment")).length;
            await panel.getByRole("button", { name: c.check, exact: true }).click();
            await expect(panel.getByRole("alert")).toContainText(c.unknownFailed);
            assert.ok(reads.filter(url => url.endsWith("/payment")).length > before, "Check status reads the same payment");
            await expect(page.getByTestId("order-page")).toContainText("2,620");
            await expect(page.getByTestId("pay-order")).toHaveCount(0);
            await shot(width === 1440 ? "1429-35484" : "1429-35506");
            paymentFailed = false;
            await panel.getByRole("button", { name: c.check, exact: true }).click();
            await expect(page.locator('section[data-variant="unknown"]')).toHaveCount(0);
            await expect(page.getByTestId("pay-order")).toBeVisible();
          } else {
            const route = kind === "empty" || kind === "offline" ? "cart" : kind === "forbidden" ? `orders/${orderID}` : "order-link";
            await page.goto(`${origin}/${locale}/${route}`);
            // expired: signed frame 26:746 / 26:1028 is the order-link page itself (h1 = frame title), no StorefrontState section.
            const panel = kind === "expired" ? page.getByTestId("order-link") : page.locator(`section[data-variant="${kind}"]`);
            await expect(panel.getByRole("heading", { name: c[kind], exact: true })).toBeVisible();
            await expect(panel).toContainText(c[`${kind}Body`]);
            if (c[`${kind}Note`]) await expect(panel).toContainText(c[`${kind}Note`]);
            await expect(page.locator(".sf-empty")).toHaveCount(0);
            await expect(page.getByText(/403\s*·/)).toHaveCount(0);
            const frames = { empty: ["1429-35150", "1429-35170"], forbidden: ["1429-35190", "1429-35210"], offline: ["1429-35230", "1429-35249"], expired: ["1429-35268", "1429-35288"] };
            await shot(frames[kind][width === 1440 ? 0 : 1]);
            if (kind === "offline") {
              const retry = page.getByRole("button", { name: c.retry, exact: true });
              const heldRead = Promise.withResolvers();
              cartReadBarrier = heldRead.promise;
              try {
                const cartRequested = page.waitForRequest(request => new URL(request.url()).pathname === "/api/buyer/cart" && request.method() === "GET");
                await Promise.all([cartRequested, retry.click()]);
                // No failure result exists while the clicked read is still held at the BFF.
                await expect(panel.getByRole("alert")).toHaveCount(0);
              } finally { heldRead.resolve(); }
              // The task's InlineError, independent of Next's separate route-announcer alert.
              await expect(panel.getByRole("alert")).toContainText(c.failed);
              await shot(width === 1440 ? "1429-35448" : "1429-35466");
              cartFailed = false;
              await retry.click();
              await expect(page.getByRole("heading", { name: c.empty, exact: true })).toBeVisible();
              await page.reload();
              await expect(page.getByRole("heading", { name: c.empty, exact: true })).toBeVisible();
              // Real click: failed cart read outside /cart opens drawer in read-problem state, never empty
              cartFailed = true;
              await page.goto(`${origin}/${locale}/products`);
              await page.getByTestId("header-cart").click();
              const drawer = page.getByTestId("cart-drawer");
              await expect(drawer).toBeVisible();
              const drawerOffline = drawer.locator('section[data-variant="offline"]');
              await expect(drawerOffline.getByRole("heading", { name: c.offline, exact: true })).toBeVisible();
              await expect(drawer.locator(".sf-empty")).toHaveCount(0);
              cartFailed = false;
            } else {
              await page.reload();
              await expect(panel).toBeVisible();
              const actionName = kind === "empty" ? c.browse : kind === "forbidden" ? c.home : kind === "expired" ? c.expiredGo : c.lookup; // expired: signed frame 26:746 button text
              const suffix = kind === "empty" ? "/products" : kind === "forbidden" ? "" : "/orders/lookup";
              await panel.getByRole("link", { name: actionName, exact: true }).click();
              await expect(page).toHaveURL(new RegExp(`/${locale}${suffix}/?$`));
            }
          }
          assert.deepEqual(writes.filter(write => (kind !== "lookup" || write.path !== "/api/buyer/orders/lookup") && (kind !== "unknown" || !write.path.endsWith("/payment/refresh"))), [], "recovery must not replay any command or payment signal");
          ledger.push({ route: kind, locale, width, operation: "real click / fill / reload", expected: "signed copy; safe recovery; no replay; refresh result retained where applicable", result: "PASS", reads, writes });
        } catch (error) {
          failures.push(`${label}: ${error.message}`);
          ledger.push({ route: kind, locale, width, result: "FAIL", error: error.message });
          await page.screenshot({ path: path.join(dir, `failed-${label}.png`) }).catch(() => {});
        } finally { await context.close(); }
      }
    }
  }
  await writeFile(path.join(dir, "buyer-click-ledger.json"), JSON.stringify(ledger, null, 2));
  assert.deepEqual(failures, [], "PARITY-STATES: signed panels and real recovery controls on buyer routes");
  console.log("PARITY-STATES: cart/order/link/lookup real clicks, all three locales, desktop+mobile; read fixtures MOCK");
}
