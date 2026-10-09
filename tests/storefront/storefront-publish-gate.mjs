// Purpose: storefront-publish-gate.mjs startup uses bounded shared bind-race recovery; gate assertions are unchanged.
// Depends on: tests/helpers/next-startup.mjs and existing app/fixture/edge imports below.
// Used by: its scripts/dev/test-local.sh browser mode; GATE-PORT acceptance.
// R3 storefront-publish KEY acceptance gate (driver of TestBrowserStorefrontPublish; BROWSER, MOCK edge).
// Production shape: no owner-seeded publication/domain row. Actors:
//   merchant  real Chromium/WebKit on the production admin Next (signed MOCK IdP): creates product + SKU, then publishes /
//             unpublishes with the Settings "storefront" card, zh-TW + en, desktop + 390px;
//   operator  the built cmd/store-admin executable on a registrar login (through the runner-only control endpoint): bind /
//             suspend / detach / re-bind of https://buyer.example;
//   buyer     a fresh anonymous browser context per check on the production storefront Next, reaching buyer.example
//             through a disposable TLS/CONNECT edge (the only host mapping; no DNS/TLS proof is claimed).
import { startNextWithPortRetry, nextAttemptLog } from "../helpers/next-startup.mjs";
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
import { createProductInEditor, selectLedgerRow } from "./merchant-product.mjs"; // stop-bleed D01: products are created in the editor
import { launch, ctxOpts } from "./browser-engine.mjs"; // LC_BROWSER_ENGINE=chromium|webkit

const root = process.cwd(), evidence = process.env.LC_JOINT_EVIDENCE;
const adminOrigin = process.env.COMMERCE_PUBLIC_ORIGIN, buyerOrigin = "https://buyer.example", store = process.env.LC_JOINT_STORE;
assert(evidence && /^https?:\/\/127\.0\.0\.1:\d+$/.test(adminOrigin) && /^[0-9a-f-]{36}$/.test(store));
assert(/^http:\/\/127\.0\.0\.1:\d+$/.test(process.env.LC_JOINT_CONTROL));
const certDir = await mkdtemp(path.join(tmpdir(), "lc-storefront-publish-edge-"));
const children = new Set(), sockets = new Set(), logs = [];
const running = child => child.exitCode === null && child.signalCode === null;
let browser, edge, proxy, cases = 0;
const pass = name => { cases++; console.log(`PASS ${name}`); };
const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
const listen = async server => { server.listen(0, "127.0.0.1"); await once(server, "listening"); return server.address().port; };
const VIEW = { desktop: { width: 1280, height: 900 }, "390px": { width: 390, height: 844 } };
// Expected product copy per locale (state text and the card's three hints), written from the unit brief, not the component.
// apps/storefront/lib/shop-copy.ts closedTitle (the not-found page of a store that is not published or not bound)
const CLOSED = { en: "This shop is not open yet", "zh-TW": "商店尚未開張" };
const COPY = {
  en: { title: /Storefront/i, published: "Published", unpublished: "Not published", awaiting: /Awaiting platform domain/, publishedNoDomain: /Published, but buyers cannot open the store/,
    ready: /A domain is live\. Publish when you are ready/, live: /Live: buyers can open your store/, savedPub: "Published.", savedUnpub: "Unpublished.", conflict: /changed after you loaded it/ },
  "zh-TW": { title: /網店發佈/, published: "已發佈", unpublished: "未發佈", awaiting: /等待平台綁定網域/, publishedNoDomain: /已發佈，但在網域生效之前買家無法打開網店/,
    ready: /網域已生效。準備好迎接買家時再發佈/, live: /已上線：買家可以打開你的網店/, savedPub: "已發佈。", savedUnpub: "已取消發佈。", conflict: /內容在你載入之後已變更/ },
};
async function dbFacts() {
  const response = await fetch(`${process.env.LC_JOINT_CONTROL}/db`, { headers: { "X-Gate-Key": process.env.LC_JOINT_CONTROL_KEY } });
  assert.equal(response.status, 200, "control /db");
  return response.json();
}
// The operator executable: one process per call, exactly the four ops-admin.sh sub-commands.
async function operator(...args) {
  const response = await fetch(`${process.env.LC_JOINT_CONTROL}/operator`, { method: "POST", headers: { "X-Gate-Key": process.env.LC_JOINT_CONTROL_KEY, "Content-Type": "application/json" }, body: JSON.stringify({ args }) });
  assert.equal(response.status, 200, "control /operator");
  return response.json();
}
const certNotAfter = () => new Date(Date.now() + 90 * 86400e3).toISOString().replace(/\.\d+Z$/, "Z");
const bindArgs = evidence => ["domain-bind", "--store", store, "--origin", buyerOrigin, "--evidence", evidence, "--valid-until", certNotAfter()];
function relay(port, request, body = Buffer.alloc(0)) {
  return new Promise((resolve, reject) => {
    const headers = { ...request.headers };
    delete headers.connection; delete headers["transfer-encoding"];
    if (body.length) headers["content-length"] = String(body.length); else delete headers["content-length"];
    const call = http.request({ hostname: "127.0.0.1", port, path: request.url, method: request.method, headers }, response => {
      const chunks = [];
      response.on("data", chunk => chunks.push(chunk));
      response.on("error", reject);
      response.on("end", () => resolve({ status: response.statusCode, headers: response.headers, body: Buffer.concat(chunks) }));
    });
    call.setTimeout(15000, () => call.destroy(new Error("fixture relay timeout")));
    call.on("error", reject); call.end(body);
  });
}
async function startNext(app, requestedPort) {
  return startNextWithPortRetry({ port: requestedPort }, async ({ port, attempt, track }) => {

    const log = createWriteStream(nextAttemptLog(path.join(evidence, `${app}.log`), attempt), { flags: "wx", mode: 0o600 });
    logs.push(log); await once(log, "open");
    const env = { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1" };
    for (const name of Object.keys(env)) if (name.startsWith("LC_JOINT_")) delete env[name]; // runner-only keys never reach the apps
    const args = app === "admin"
      ? [path.join(root, "apps/admin/.next/standalone/apps/admin/server.js")]
      : [path.join(root, "apps/storefront/node_modules/next/dist/bin/next"), "start", "--hostname", "127.0.0.1", "--port", String(port)];
    env.HOSTNAME = "127.0.0.1"; env.PORT = String(port);
    const child = spawn(process.execPath, args, { cwd: path.join(root, `apps/${app}`), env, stdio: ["ignore", log, log] });
    track(child, log);
    children.add(child);
    for (let i = 0; i < 100; i++) {
      if (!running(child)) throw new Error(`owned ${app} exited before readiness`);
      try {
        const response = await relay(port, { url: app === "admin" ? "/api/stores" : "/api/buyer/session", method: "GET", headers: { host: app === "admin" ? new URL(adminOrigin).host : "buyer.example" } });
        // admin: 401 without a session; storefront: 404 not_found while nothing is published (no 5xx = the app is up)
        if (app === "admin" ? response.status === 401 : response.status < 500) return port;
      } catch {}
      await wait(50);
    }
    throw new Error(`owned ${app} readiness deadline`);

  });
}
try {
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path.join(certDir, "key.pem"), "-out", path.join(certDir, "cert.pem"), "-days", "1", "-subj", "/CN=buyer.example"], { stdio: "ignore" });
  const [, buyerPort] = await Promise.all([startNext("admin", Number(process.env.LC_JOINT_ADMIN_PORT)), startNext("storefront")]);
  edge = https.createServer({ key: await readFile(path.join(certDir, "key.pem")), cert: await readFile(path.join(certDir, "cert.pem")) }, async (request, response) => {
    try {
      const chunks = []; for await (const chunk of request) chunks.push(chunk);
      const out = await relay(buyerPort, request, Buffer.concat(chunks));
      const headers = { ...out.headers }; delete headers.connection; delete headers["transfer-encoding"];
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
  browser = await launch({ headless: true, proxy: { server: `http://127.0.0.1:${proxyPort}`, bypass: "127.0.0.1" } });
  const uiErrors = [];
  const audit = []; // the audit actions this gate expects PG to hold, in order (compared by the Go test against PG)

  // ---- merchant: real sign-in through the signed MOCK IdP, then product + SKU through the product UI ----------------------
  const merchantContext = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: VIEW.desktop }));
  merchantContext.on("page", page => page.on("pageerror", error => uiErrors.push(error.name)));
  const merchant = await merchantContext.newPage();
  let sawIssuer = false;
  merchant.on("request", request => { if (new URL(request.url()).origin === process.env.COMMERCE_OIDC_ISSUER) sawIssuer = true; });
  await merchant.goto(`${adminOrigin}/en`);
  await merchant.getByRole("button", { name: "Sign in with identity service", exact: true }).click();
  // 0094 (merchant-tools, storefront-v2 G1): the sign-in landing is the dashboard; the product ledger moved to /[locale]/inventory.
  await expect(merchant.getByTestId("dashboard-page")).toBeVisible();
  await merchant.goto(`${adminOrigin}/en/inventory`);
  // stop-bleed D01: "Add product" on the inventory page is a link to the full editor now (no inline quick-add panel)
  await expect(merchant.getByRole("link", { name: "Add product", exact: true })).toBeVisible();
  assert(sawIssuer, "real signed MOCK IdP browser redirect required");
  const name = `Publish gate product ${Date.now()}`, code = `PUB-${Date.now()}`;
  // the product and its first SKU go through the product editor, the only creation UI (price typed in major units: 123.45 = 12345 minor)
  const { product, sku } = await createProductInEditor(merchant, { adminOrigin, store, name, description: "Synthetic product for the storefront publication gate", code, price: "123.45" });
  assert.equal(sku.price_minor, 12345);
  // Nothing is published and nothing is bound: the merchant's own purchase-entry control (stock ledger, the selected product) says so.
  await selectLedgerRow(merchant, { adminOrigin, code });
  await expect(merchant.getByTestId("purchase-entry").getByRole("status")).toHaveText("No verified, published storefront address is available.");
  const productUrl = locale => `${buyerOrigin}/${locale}/products/${product.id}`;
  pass("signed MOCK IdP; merchant created product + SKU in the UI; purchase entry reports no published address");

  // ---- buyer helpers: a fresh anonymous context per check ---------------------------------------------------------------
  async function anonymous(view, fn) {
    const context = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: VIEW[view] }));
    context.on("page", page => page.on("pageerror", error => uiErrors.push(error.name)));
    try { return await fn(await context.newPage()); } finally { await context.close(); }
  }
  const notFound = (locale, view, why, shot) => anonymous(view, async page => {
    // 0093 (storefront-integration): an unpublished / unbound / suspended / detached store is the shop shell's real 404 with the "not open yet" page.
    const response = await page.goto(productUrl(locale));
    assert.equal(response.status(), 404, `${why}: the closed store answers 404`);
    await expect(page.getByTestId("store-closed")).toBeVisible();
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(CLOSED[locale]);
    await expect(page.getByTestId("product-buy")).toHaveCount(0);
    await expect(page.getByRole("radio")).toHaveCount(0);
    await expect(page.getByRole("heading", { name, exact: true })).toHaveCount(0);
    // G-UI8 audit [READ/MEASURE]: same-origin GET read of server state through the BFF (no state change) (buyer session)
    const state = await page.evaluate(async () => { const response = await fetch("/api/buyer/session"); return { status: response.status, body: await response.json() }; });
    assert.notEqual(state.body.state, "active", `${why}: buyer session must not be active`);
    // G-UI8 audit [READ/MEASURE]: same-origin GET read of server state through the BFF (no state change) (catalog)
    const catalog = await page.evaluate(async id => { const response = await fetch(`/api/buyer/catalog?product_id=${id}`); return { status: response.status, body: await response.json() }; }, product.id);
    assert(!JSON.stringify(catalog.body).includes(code), `${why}: product leaked through the buyer catalog`);
    // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${why}: horizontal overflow`);
    if (shot) await page.screenshot({ path: path.join(evidence, shot), fullPage: true });
    pass(`buyer sees the not-found page (${locale}, ${view}): ${why}`);
  });
  const sees = (locale, view, why, shot) => anonymous(view, async page => {
    const response = await page.goto(productUrl(locale));
    assert.equal(response.status(), 200);
    // 0093: the product page is the shop shell's (h1 title, data-sku on the buy box, variant price); the buyer session is created at the first
    // add-to-cart (CartProvider), not by viewing the page, so a view proves the buyer API answers on the published origin (it 404s on a closed one).
    await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
    await expect(page.getByTestId("product-buy")).toHaveAttribute("data-sku", sku.id);
    await expect(page.getByTestId("variant-price")).toContainText("123.45");
    // G-UI8 audit [READ/MEASURE]: same-origin GET read of server state through the BFF (no state change) (buyer session)
    const session = await page.evaluate(async () => { const r = await fetch("/api/buyer/session"); return { status: r.status, body: await r.json() }; });
    assert.equal(session.status, 200, `${why}: the buyer API answers on a published origin`);
    assert(["absent", "active"].includes(session.body.state), `${why}: unexpected buyer session state ${session.body.state}`);
    // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${why}: horizontal overflow`);
    if (shot) await page.screenshot({ path: path.join(evidence, shot), fullPage: true });
    pass(`anonymous buyer sees the product on ${buyerOrigin} (${locale}, ${view}): ${why}`);
  });

  // ---- merchant card helpers ------------------------------------------------------------------------------------------
  async function openCard(page, locale, view) {
    await page.setViewportSize(VIEW[view]);
    await page.goto(`${adminOrigin}/${locale}/settings?store=${store}`);
    const card = page.getByTestId("storefront-card");
    await expect(card).toBeVisible();
    await expect(card.getByRole("heading", { level: 2 })).toHaveText(COPY[locale].title);
    await expect(card.getByTestId("storefront-state")).toBeVisible();
    const fit = await card.evaluate(element => ({ scroll: element.scrollWidth, client: element.clientWidth, right: element.getBoundingClientRect().right, view: innerWidth }));
    assert(fit.scroll <= fit.client + 1 && fit.right <= fit.view + 1, `storefront card overflows at ${view}: ${JSON.stringify(fit)}`);
    return card;
  }
  async function toggle(card, locale, wantPublished) {
    const c = COPY[locale];
    await card.getByTestId("storefront-toggle").click();
    const dialog = card.getByTestId("storefront-confirm");
    await expect(dialog).toBeVisible(); // nothing is written before the confirm
    const post = card.page().waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === `/api/stores/${store}/storefront/publication`);
    await dialog.getByTestId("storefront-confirm-yes").click();
    const reply = await post;
    assert.equal(reply.status(), 200, "publication POST");
    await expect(card.getByTestId("storefront-notice")).toHaveText(wantPublished ? c.savedPub : c.savedUnpub);
    await expect(card.getByTestId("storefront-state")).toHaveText(wantPublished ? c.published : c.unpublished);
    audit.push(wantPublished ? "merchant.storefront_published" : "merchant.storefront_unpublished");
  }
  const op = async (label, args, expectExit, expectStderr = "") => {
    const out = await operator(...args);
    assert.equal(out.exit, expectExit, `${label}: ${JSON.stringify(out)}`);
    if (expectExit === 0) { assert.equal(out.stderr, ""); const line = JSON.parse(out.stdout); assert.equal(out.stdout.trim().split("\n").length, 1); return line; }
    assert.equal(out.stdout, ""); assert.equal(out.stderr.trim(), expectStderr, label);
    return null;
  };

  // ---- 0. production starting point -------------------------------------------------------------------------------------
  let facts = await dbFacts();
  assert.deepEqual([facts.publications, facts.domains, facts.audits], [[], [], []], "no publication, domain or audit row before the merchant acts");
  await notFound("en", "desktop", "nothing published, nothing bound (no owner-seeded row)");
  let card = await openCard(merchant, "en", "desktop");
  await expect(card.getByTestId("storefront-state")).toHaveText(COPY.en.unpublished);
  await expect(card.getByTestId("storefront-domain")).toContainText(COPY.en.awaiting);
  await expect(card.getByTestId("storefront-domain").getByRole("link")).toHaveCount(0);
  await expect(card.getByTestId("storefront-toggle")).toHaveText("Publish storefront");
  await merchant.screenshot({ path: path.join(evidence, "card-en-desktop-initial.png"), fullPage: true });
  pass("Settings card (en, desktop): not published, awaiting platform domain, no merchant control to bind a domain");
  assert.equal(await card.getByRole("textbox").count(), 0, "the card has no input a merchant could bind a domain with");

  // ---- A. en / desktop: publish alone serves nothing; operator bind makes it live; unpublish takes it down -----------------
  await toggle(card, "en", true);
  await expect(card.getByTestId("storefront-hint")).toContainText(COPY.en.publishedNoDomain);
  await notFound("en", "desktop", "published but no platform domain yet");
  const bound = await op("bind", bindArgs("openssl s_client notAfter; DNS TXT checked 2026-10-01 (gate evidence)"), 0);
  audit.push("operator.domain_bound");
  assert.deepEqual([bound.state, bound.version, bound.renewed, bound.rebound], ["ACTIVE", 1, false, false]);
  card = await openCard(merchant, "en", "desktop");
  await expect(card.getByTestId("storefront-domain").getByRole("link")).toHaveText(buyerOrigin);
  await expect(card.getByTestId("storefront-hint")).toContainText(COPY.en.live);
  await sees("en", "desktop", "published + operator-bound", "buyer-en-desktop-live.png");
  await merchant.screenshot({ path: path.join(evidence, "card-en-desktop-live.png"), fullPage: true });
  await toggle(card, "en", false);
  await notFound("en", "desktop", "merchant unpublished", "buyer-en-desktop-unpublished.png");
  pass("cycle A (en, desktop): publish -> still not found -> operator bind -> product visible -> unpublish -> not found");

  // ---- B. zh-TW / 390px: suspend and re-bind -----------------------------------------------------------------------------
  card = await openCard(merchant, "zh-TW", "390px");
  await expect(card.getByTestId("storefront-state")).toHaveText(COPY["zh-TW"].unpublished);
  await expect(card.getByTestId("storefront-hint")).toContainText(COPY["zh-TW"].ready); // domain live, merchant not yet published
  await toggle(card, "zh-TW", true);
  await expect(card.getByTestId("storefront-hint")).toContainText(COPY["zh-TW"].live);
  await merchant.screenshot({ path: path.join(evidence, "card-zh-TW-390-live.png"), fullPage: true });
  await sees("zh-TW", "390px", "published again, domain still ACTIVE", "buyer-zh-TW-390-live.png");
  const suspended = await op("suspend", ["domain-suspend", "--origin", buyerOrigin], 0);
  audit.push("operator.domain_suspended");
  assert.deepEqual([suspended.state, suspended.changed], ["SUSPENDED", true]);
  await notFound("zh-TW", "390px", "operator suspended the domain");
  card = await openCard(merchant, "zh-TW", "390px");
  await expect(card.getByTestId("storefront-domain")).toContainText(COPY["zh-TW"].awaiting); // merchant no longer sees a live origin
  await expect(card.getByTestId("storefront-state")).toHaveText(COPY["zh-TW"].published);
  const rebound = await op("bind after suspend", bindArgs("renewed after suspension: new certificate"), 0);
  audit.push("operator.domain_bound");
  assert.deepEqual([rebound.state, rebound.renewed, rebound.rebound], ["ACTIVE", false, false]);
  await sees("zh-TW", "390px", "re-bound after suspension");
  card = await openCard(merchant, "zh-TW", "390px");
  await toggle(card, "zh-TW", false);
  await notFound("zh-TW", "390px", "merchant unpublished", "buyer-zh-TW-390-unpublished.png");
  pass("cycle B (zh-TW, 390px): publish -> visible -> suspend -> not found -> bind -> visible -> unpublish -> not found");

  // ---- C. zh-TW / desktop: detach, the same-evidence refusal, renewed-proof re-bind (integrator ruling) ----------------
  card = await openCard(merchant, "zh-TW", "desktop");
  await toggle(card, "zh-TW", true);
  await sees("zh-TW", "desktop", "published again");
  const detached = await op("detach", ["domain-detach", "--origin", buyerOrigin], 0);
  audit.push("operator.domain_detached");
  assert.deepEqual([detached.state, detached.changed], ["DETACHED", true]);
  await notFound("zh-TW", "desktop", "operator detached the domain");
  await op("re-bind with the evidence still on the row", bindArgs("renewed after suspension: new certificate"), 1, "store_admin_domain_detached");
  await notFound("zh-TW", "desktop", "a refused re-bind (same evidence) must not revive a DETACHED origin");
  const reborn = await op("re-bind with renewed proof", bindArgs("renewed after detach: third certificate"), 0);
  audit.push("operator.domain_bound:rebind_from_detached");
  assert.deepEqual([reborn.state, reborn.rebound], ["ACTIVE", true]);
  await sees("zh-TW", "desktop", "DETACHED origin re-bound with renewed proof", "buyer-zh-TW-desktop-rebound.png");
  card = await openCard(merchant, "zh-TW", "desktop");
  await toggle(card, "zh-TW", false);
  await notFound("zh-TW", "desktop", "merchant unpublished");
  pass("cycle C (zh-TW, desktop): publish -> visible -> detach -> not found -> same-evidence re-bind refused -> renewed re-bind -> visible -> unpublish");

  // ---- D. en / 390px, plus a stale second tab: compare-and-set 409 in the UI -------------------------------------------
  card = await openCard(merchant, "en", "390px");
  const second = await merchantContext.newPage();
  const staleCard = await openCard(second, "en", "390px"); // loaded at the same version as `card`
  await toggle(card, "en", true);
  await sees("en", "390px", "published from the 390px card", "buyer-en-390-live.png");
  await staleCard.getByTestId("storefront-toggle").click(); // the stale tab still believes "not published"
  const stale = second.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === `/api/stores/${store}/storefront/publication`);
  await staleCard.getByTestId("storefront-confirm-yes").click();
  assert.equal((await stale).status(), 409, "a stale expected_version must be refused with 409");
  await expect(staleCard.getByTestId("storefront-problem")).toHaveText(COPY.en.conflict);
  await expect(staleCard.getByTestId("storefront-state")).toHaveText(COPY.en.published); // reloaded from the server
  await second.close();
  facts = await dbFacts();
  assert.equal(facts.publications[0].published, true);
  await sees("en", "390px", "the refused stale write changed nothing");
  await toggle(card, "en", false);
  await notFound("en", "390px", "merchant unpublished", "buyer-en-390-unpublished.png");
  pass("cycle D (en, 390px): publish -> visible; stale second tab refused with 409 and reloaded; unpublish -> not found");

  // ---- final facts ----------------------------------------------------------------------------------------------------
  facts = await dbFacts();
  assert.deepEqual(facts.audits, audit, "PG audit trail equals the actions the gate drove");
  assert.equal(facts.publications.length, 1);
  assert.equal(facts.publications[0].version, 8, "four publish/unpublish cycles = version 8; refused writes add none");
  assert.deepEqual(facts.domains.map(d => [d.origin, d.state]), [[buyerOrigin, "ACTIVE"]]);
  const status = await op("status", ["status", "--store", store], 0);
  assert.deepEqual([status.store_id, status.published, status.version], [store, false, 8]);
  assert.equal(status.domains.length, 1);
  assert.equal(JSON.stringify(status).includes("certificate"), false, "status prints no evidence");
  assert.deepEqual(uiErrors, []);
  pass("PG facts: publication version 8, one ACTIVE domain, audit trail equals the driven actions, status JSON carries no evidence");
  await writeFile(path.join(evidence, "result.json"), JSON.stringify({ cases, product_id: product.id, locales: ["en", "zh-TW"], viewports: ["desktop", "390px"], origin: buyerOrigin, audit_expected: audit, boundary: "production Next; signed MOCK IdP; real store-admin executable; synthetic TLS/CONNECT edge; ownership/TLS evidence is an unverified reference; no deployment DNS/TLS or provider acceptance" }, null, 2), { mode: 0o600 });
} catch (error) {
  if (browser) for (const context of browser.contexts()) for (const [index, page] of context.pages().entries()) {
    await page.screenshot({ path: path.join(evidence, `failure-${index}-${Date.now()}.png`), fullPage: true }).catch(() => {});
  }
  throw error;
} finally {
  if (browser) await browser.close();
  for (const socket of sockets) socket.destroy();
  for (const server of [proxy, edge]) if (server) await new Promise(resolve => server.close(resolve));
  for (const child of children) { if (running(child)) child.kill("SIGKILL"); }
  for (const child of children) if (running(child)) await once(child, "exit");
  for (const log of logs) log.end();
  await rm(certDir, { recursive: true, force: true });
}
