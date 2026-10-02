// R5 store-domains KEY acceptance gate (driver of TestBrowserStoreDomains; BROWSER, MOCK edge + scripted public DNS/TLS).
// Production shape: no owner-seeded store/publication/domain row for the signing-in merchant. Actors:
//   merchant  real Chromium/WebKit on the production admin Next (signed MOCK IdP, fresh principal with zero
//             memberships): runs the onboarding wizard with an English store name (live handle preview, receipt
//             shows https://<handle>.<base>), publishes from the Settings card (zh-TW, 390px), requests a custom
//             domain (zh-CN, desktop), follows the DNS instructions, then suspends / detaches it and unpublishes;
//   platform  the production VerifyPending worker library on the registrar login, driven once through the runner-only
//             control endpoint with the scripted public-DNS answers + certificate probe (the MOCK seam: no real DNS
//             or CA is claimed);
//   buyer     a fresh anonymous browser context per check on the production storefront Next, reaching
//             <handle>.<base> and the custom host through a disposable TLS/CONNECT edge (the only host mapping).
// The platform-subdomain 301 to the ACTIVE custom origin (架构 §7, Decision 3) is a contract checkpoint: it is
// recorded and reported like every other failure (P0-3: nothing calls the primary-origin endpoint yet).
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

const root = process.cwd(), evidence = process.env.LC_JOINT_EVIDENCE;
const adminOrigin = process.env.COMMERCE_PUBLIC_ORIGIN, BASE = process.env.LC_JOINT_BASE;
assert(evidence && /^https?:\/\/127\.0\.0\.1:\d+$/.test(adminOrigin) && /^[a-z0-9.-]+$/.test(BASE || ""));
assert(/^http:\/\/127\.0\.0\.1:\d+$/.test(process.env.LC_JOINT_CONTROL));
const certDir = await mkdtemp(path.join(tmpdir(), "lc-store-domains-edge-"));
const children = new Set(), sockets = new Set(), logs = [];
const running = child => child.exitCode === null && child.signalCode === null;
let browser, edge, proxy, cases = 0;
const pass = name => { cases++; console.log(`PASS ${name}`); };
const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
const listen = async server => { server.listen(0, "127.0.0.1"); await once(server, "listening"); return server.address().port; };
const VIEW = { desktop: { width: 1280, height: 900 }, "390px": { width: 390, height: 844 } };
const hosts = new Set(); // the CONNECT proxy's allowlist: exactly the buyer hosts this gate has bound so far
// Expected copy per locale, written from the unit brief and copy files, not the components under test.
const CLOSED = { en: "This shop is not open yet", "zh-TW": "商店尚未開張", "zh-CN": "商店尚未开张" };
const ENTRY = { signIn: "Sign in with identity service", title: "Create your first workspace", nextStore: "Next: store settings", nextWarehouse: "Next: warehouse", create: "Create internal workspace", openWorkspace: "Open workspace", handlePreview: "Your store ID", handleAvailable: "available" };
const CARD = {
  en: { title: /Storefront/i, published: "Published", unpublished: "Not published", publish: "Publish storefront", unpublish: "Unpublish storefront", savedPub: "Published.", savedUnpub: "Unpublished.", live: /Live: buyers can open your store/ },
  "zh-TW": { title: /網店發佈/, published: "已發佈", unpublished: "未發佈", publish: "發佈網店", unpublish: "取消發佈", savedPub: "已發佈。", savedUnpub: "已取消發佈。", live: /已上線：買家可以打開你的網店/ },
  "zh-CN": { title: /网店发布/, published: "已发布", unpublished: "未发布", publish: "发布网店", unpublish: "取消发布", savedPub: "已发布。", savedUnpub: "已取消发布。", live: /已上线：买家可以打开你的网店/ },
};
const DOMAINS_ZHCN = { title: "店铺网址", hostnameLabel: "域名", request: "请求校验", dnsTitle: "需要添加的 DNS 记录", platform: "平台网址", serving: "正在服务买家", suspend: "暂停", detach: "解绑", states: { REQUESTED: "等待 DNS", ACTIVE: "已生效", SUSPENDED: "已暂停", DETACHED: "已解绑" } };
async function control(pathname, init = {}) {
  const response = await fetch(`${process.env.LC_JOINT_CONTROL}${pathname}`, { ...init, headers: { "X-Gate-Key": process.env.LC_JOINT_CONTROL_KEY, "Content-Type": "application/json", ...(init.headers || {}) } });
  assert.equal(response.status, 200, `control ${pathname}`);
  return response.json();
}
const dbFacts = store => control(`/db?store=${store}`);
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
async function startNext(app, port) {
  if (!port) { const reserve = net.createServer(); port = await listen(reserve); await new Promise(resolve => reserve.close(resolve)); }
  const log = createWriteStream(path.join(evidence, `${app}.log`), { flags: "wx", mode: 0o600 });
  logs.push(log); await once(log, "open");
  const env = { ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1" };
  for (const name of Object.keys(env)) if (name.startsWith("LC_JOINT_")) delete env[name]; // runner-only keys never reach the apps
  const args = app === "admin"
    ? [path.join(root, "apps/admin/.next/standalone/apps/admin/server.js")]
    : [path.join(root, "apps/storefront/node_modules/next/dist/bin/next"), "start", "--hostname", "127.0.0.1", "--port", String(port)];
  env.HOSTNAME = "127.0.0.1"; env.PORT = String(port);
  const child = spawn(process.execPath, args, { cwd: path.join(root, `apps/${app}`), env, stdio: ["ignore", log, log] });
  children.add(child);
  for (let i = 0; i < 100; i++) {
    if (!running(child)) throw new Error(`owned ${app} exited before readiness`);
    try {
      const response = await relay(port, { url: app === "admin" ? "/api/stores" : "/api/buyer/session", method: "GET", headers: { host: app === "admin" ? new URL(adminOrigin).host : `gate.${BASE}` } });
      // admin: 401 without a session; storefront: 404 not_found while nothing resolves (no 5xx = the app is up)
      if (app === "admin" ? response.status === 401 : response.status < 500) return port;
    } catch {}
    await wait(50);
  }
  throw new Error(`owned ${app} readiness deadline`);
}
const contractFailures = []; // contract checkpoints that must hold but are known-red defects (see REVIEW-store-domains.md)
try {
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path.join(certDir, "key.pem"), "-out", path.join(certDir, "cert.pem"), "-days", "1", "-subj", `/CN=${BASE}`], { stdio: "ignore" });
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
    if (!hosts.has(request.url)) { socket.destroy(); return; } // fail closed: only the hosts this gate bound
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

  // ---- merchant: real sign-in through the signed MOCK IdP; the onboarding wizard creates everything ---------------
  const merchantContext = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: VIEW.desktop }));
  merchantContext.on("page", page => page.on("pageerror", error => uiErrors.push(error.name)));
  const merchant = await merchantContext.newPage();
  let sawIssuer = false;
  merchant.on("request", request => { if (new URL(request.url()).origin === process.env.COMMERCE_OIDC_ISSUER) sawIssuer = true; });
  const suffix = Math.random().toString(36).slice(2, 8);
  const storeName = `Gate Store ${suffix}`;
  await merchant.goto(`${adminOrigin}/en`);
  await merchant.getByRole("button", { name: ENTRY.signIn, exact: true }).click();
  await expect(merchant.getByRole("heading", { name: ENTRY.title, exact: true })).toBeVisible(); // zero memberships: the wizard, not a dashboard
  assert(sawIssuer, "real signed MOCK IdP browser redirect required");
  await merchant.locator("input[name=tenant_name]").fill(`Gate Merchant ${suffix}`);
  await merchant.getByRole("button", { name: ENTRY.nextStore, exact: true }).click();
  await merchant.locator("input[name=store_name]").fill(storeName);
  const preview = merchant.getByTestId("entry-handle"); // Decision 1: live slug suggestion on the store-name step
  await expect(preview).toBeVisible();
  await expect(preview).toContainText(ENTRY.handlePreview);
  await expect(preview).toContainText(ENTRY.handleAvailable);
  const previewHandle = (await preview.locator("strong").innerText()).trim();
  assert.match(previewHandle, /^[a-z0-9]([a-z0-9-]{1,28}[a-z0-9])$/, `suggested handle ${previewHandle}`);
  assert(previewHandle.startsWith("gate-store-"), "the English store name drives the slug");
  await expect(merchant.locator("select[name=currency]")).toHaveValue("USD");
  await merchant.getByRole("button", { name: ENTRY.nextWarehouse, exact: true }).click();
  await merchant.locator("input[name=warehouse_name]").fill("Main warehouse");
  const created = merchant.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === "/api/onboarding/initial-store");
  await merchant.getByRole("button", { name: ENTRY.create, exact: true }).click();
  const receipt = await (await created).json();
  assert.match(receipt.store_id, /^[0-9a-f-]{36}$/); assert.match(receipt.tenant_id, /^[0-9a-f-]{36}$/);
  assert.equal(receipt.handle, previewHandle, "the DB trigger assigns the handle the wizard previewed");
  const store = receipt.store_id;
  const platformOrigin = `https://${receipt.handle}.${BASE}`;
  assert.equal(receipt.storefront_origin, platformOrigin, "Decision 2: the ACTIVE platform subdomain on the receipt");
  const address = merchant.getByTestId("entry-address");
  await expect(address).toBeVisible();
  await expect(address.getByRole("link")).toHaveText(platformOrigin);
  await merchant.screenshot({ path: path.join(evidence, "onboarding-en-desktop-receipt.png"), fullPage: true });
  await merchant.getByRole("button", { name: ENTRY.openWorkspace, exact: true }).click();
  await expect(merchant.getByTestId("dashboard-page")).toBeVisible();
  audit.push("merchant.store_created");
  pass("signed MOCK IdP; onboarding wizard (en, desktop): live handle preview, receipt shows the platform origin, workspace opens");

  // ---- platform subdomain is ACTIVE at onboarding; nothing is published ------------------------------------------
  const platformHost = `${receipt.handle}.${BASE}`;
  hosts.add(`${platformHost}:443`);
  let facts = await dbFacts(store);
  assert.equal(facts.handle, receipt.handle, "PG stores.handle equals the receipt");
  assert.deepEqual(facts.domains.map(d => [d.origin, d.state, d.version]), [[platformOrigin, "ACTIVE", 1]], "exactly one row: the ACTIVE platform subdomain");
  assert.deepEqual([facts.publications, facts.audits], [[], ["merchant.store_created"]], "no publication row; one onboarding audit row");
  async function anonymous(view, fn) {
    const context = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: VIEW[view] }));
    context.on("page", page => page.on("pageerror", error => uiErrors.push(error.name)));
    try { return await fn(await context.newPage()); } finally { await context.close(); }
  }
  const notFound = (origin, locale, view, why, shot) => anonymous(view, async page => {
    const response = await page.goto(`${origin}/${locale}/`);
    assert.equal(response.status(), 404, `${why}: the closed store answers 404`);
    await expect(page.getByTestId("store-closed")).toBeVisible();
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(CLOSED[locale]);
    await expect(page.getByTestId("home-empty")).toHaveCount(0);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${why}: horizontal overflow`);
    if (shot) await page.screenshot({ path: path.join(evidence, shot), fullPage: true });
    pass(`buyer sees the not-found page at ${origin} (${locale}, ${view}): ${why}`);
  });
  const sees = (origin, locale, view, why, shot) => anonymous(view, async page => {
    const response = await page.goto(`${origin}/${locale}/`);
    assert.equal(response.status(), 200, `${why}: the published store answers 200`);
    await expect(page.getByTestId("home-empty")).toBeVisible(); // a brand-new store has the default empty home (no design sections)
    await expect(page.getByTestId("store-closed")).toHaveCount(0);
    const session = await page.evaluate(async () => { const r = await fetch("/api/buyer/session"); return { status: r.status, body: await r.json() }; });
    assert.equal(session.status, 200, `${why}: the buyer API answers on a published origin`);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${why}: horizontal overflow`);
    if (shot) await page.screenshot({ path: path.join(evidence, shot), fullPage: true });
    pass(`anonymous buyer is served at ${origin} (${locale}, ${view}): ${why}`);
  });
  await notFound(platformOrigin, "en", "desktop", "platform subdomain ACTIVE but the merchant has not published");

  // ---- publish from the Settings card (zh-TW, 390px); the buyer is served at the platform origin ------------------
  async function openCard(page, locale, view) {
    await page.setViewportSize(VIEW[view]);
    await page.goto(`${adminOrigin}/${locale}/settings?store=${store}`);
    const card = page.getByTestId("storefront-card");
    await expect(card).toBeVisible();
    await expect(card.getByRole("heading", { level: 2 })).toHaveText(CARD[locale].title);
    await expect(card.getByTestId("storefront-state")).toBeVisible();
    const fit = await card.evaluate(element => ({ scroll: element.scrollWidth, client: element.clientWidth, right: element.getBoundingClientRect().right, view: innerWidth }));
    assert(fit.scroll <= fit.client + 1 && fit.right <= fit.view + 1, `storefront card overflows at ${view}: ${JSON.stringify(fit)}`);
    return card;
  }
  async function toggle(card, locale, wantPublished) {
    const c = CARD[locale];
    await card.getByTestId("storefront-toggle").click();
    const dialog = card.getByTestId("storefront-confirm");
    await expect(dialog).toBeVisible();
    const post = card.page().waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === `/api/stores/${store}/storefront/publication`);
    await dialog.getByTestId("storefront-confirm-yes").click();
    assert.equal((await post).status(), 200, "publication POST");
    await expect(card.getByTestId("storefront-notice")).toHaveText(wantPublished ? c.savedPub : c.savedUnpub);
    await expect(card.getByTestId("storefront-state")).toHaveText(wantPublished ? c.published : c.unpublished);
    audit.push(wantPublished ? "merchant.storefront_published" : "merchant.storefront_unpublished");
  }
  let card = await openCard(merchant, "zh-TW", "390px");
  await expect(card.getByTestId("storefront-state")).toHaveText(CARD["zh-TW"].unpublished);
  await expect(card.getByTestId("storefront-domain").getByRole("link")).toHaveText(platformOrigin); // already live at onboarding
  await toggle(card, "zh-TW", true);
  await expect(card.getByTestId("storefront-hint")).toContainText(CARD["zh-TW"].live);
  await merchant.screenshot({ path: path.join(evidence, "card-zh-TW-390-live.png"), fullPage: true });
  await sees(platformOrigin, "zh-TW", "390px", "published on the platform subdomain", "buyer-zh-TW-390-platform.png");
  pass("cycle publish (zh-TW, 390px): not published -> publish -> buyer served at https://<handle>.<base> through the test edge");

  // ---- merchant self-service custom domain (zh-CN, desktop): request -> DNS instructions --------------------------
  const customHost = `shop-${suffix}.example.net`;
  const customOrigin = `https://${customHost}`;
  card = await openCard(merchant, "zh-CN", "desktop");
  const list = card.getByTestId("storefront-domains");
  await expect(list).toBeVisible();
  await expect(list.getByRole("heading", { name: DOMAINS_ZHCN.title, exact: true })).toBeVisible();
  const rowFor = host => list.getByTestId("storefront-domain-row").filter({ hasText: host });
  await expect(rowFor(platformHost)).toHaveAttribute("data-state", "ACTIVE");
  await expect(rowFor(platformHost)).toContainText(DOMAINS_ZHCN.states.ACTIVE);
  await card.getByTestId("storefront-domain-request").getByRole("textbox", { name: DOMAINS_ZHCN.hostnameLabel }).fill(customHost);
  const requested = merchant.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === `/api/stores/${store}/storefront/domains`);
  await card.getByTestId("storefront-domain-submit").click();
  assert.equal((await requested).status(), 200, "domain request POST");
  const dns = card.getByTestId("storefront-dns");
  await expect(dns).toBeVisible();
  await expect(dns).toContainText(DOMAINS_ZHCN.dnsTitle);
  await expect(card.getByTestId("storefront-dns-txt-name")).toHaveText(`_lc-verify.${customHost}`);
  const token = (await card.getByTestId("storefront-dns-txt-value").innerText()).trim();
  assert.match(token, /^[A-Za-z0-9_-]{43}$/, "the one-time TXT token");
  await expect(card.getByTestId("storefront-dns-cname")).toContainText(`stores.${BASE}`);
  await expect(rowFor(customHost)).toHaveAttribute("data-state", "REQUESTED");
  await merchant.screenshot({ path: path.join(evidence, "domains-zh-CN-desktop-dns.png"), fullPage: true });
  audit.push("merchant.domain_requested");
  pass("custom domain requested (zh-CN, desktop): TXT name/value + CNAME instructions shown once, row REQUESTED");

  // ---- scripted public DNS answers; one production VerifyPending sweep turns the row ACTIVE ------------------------
  await control("/dns", { method: "POST", body: JSON.stringify({ host: customHost, txt_name: `_lc-verify.${customHost}`, txt_value: token, cname_target: `stores.${BASE}` }) });
  const sweep = await control("/verify", { method: "POST", body: "{}" });
  assert(sweep.dns_attempts >= 1 && sweep.tls_completed === 1, `verify sweep: ${JSON.stringify(sweep)}`);
  hosts.add(`${customHost}:443`);
  card = await openCard(merchant, "zh-CN", "desktop");
  await expect(rowFor(customHost)).toHaveAttribute("data-state", "ACTIVE");
  await expect(rowFor(customHost)).toContainText(DOMAINS_ZHCN.serving);
  await expect(rowFor(platformHost)).toHaveAttribute("data-state", "ACTIVE"); // the platform origin survives a merchant domain
  await sees(customOrigin, "zh-CN", "desktop", "the ACTIVE custom domain serves buyers", "buyer-zh-CN-desktop-custom.png");
  pass("fake DNS resolver turns the domain green: one sweep -> ACTIVE; buyer served on the custom host");

  // ---- contract checkpoint: the platform subdomain 301s to the ACTIVE custom origin (架构 §7, Decision 3) ---------
  await anonymous("desktop", async page => {
    const response = await page.goto(`${platformOrigin}/zh-CN/`);
    // The redirect chain, first hop first (Next may add an internal 308 trailing-slash hop; the contract hop is
    // a cross-origin 301 whose Location is the primary origin, with the final document served there).
    const hops = [];
    let request = response.request();
    while (request) {
      const answer = await request.response();
      if (answer) hops.push({ status: answer.status(), location: answer.headers().location || "" });
      request = request.redirectedTo();
    }
    const cross = hops.find(hop => hop.status === 301 && hop.location.startsWith(customOrigin));
    if (!(cross && response.url().startsWith(customOrigin))) {
      contractFailures.push(`P0-3: ${platformOrigin} must answer 301 to ${customOrigin} while the custom domain is primary; chain=${JSON.stringify(hops)} final=${response.url()}`);
      return;
    }
    pass("platform subdomain 301s to the primary custom origin");
  });

  // ---- suspend: the custom host closes, the platform origin takes buyers back --------------------------------------
  card = await openCard(merchant, "zh-CN", "desktop");
  await rowFor(customHost).getByTestId("storefront-domain-suspend").click();
  await expect(card.getByTestId("storefront-domain-confirm")).toBeVisible();
  const suspended = merchant.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === `/api/stores/${store}/storefront/domains/suspend`);
  await card.getByTestId("storefront-domain-confirm-yes").click();
  assert.equal((await suspended).status(), 200, "domain suspend POST");
  await expect(rowFor(customHost)).toHaveAttribute("data-state", "SUSPENDED");
  audit.push("merchant.domain_suspended");
  await notFound(customOrigin, "zh-CN", "desktop", "merchant suspended the custom domain");
  await sees(platformOrigin, "zh-CN", "desktop", "the platform origin is the primary again");
  pass("suspend (zh-CN, desktop): custom host 404, platform origin serves again");

  // ---- detach: final; the host never resolves for this store again --------------------------------------------------
  await rowFor(customHost).getByTestId("storefront-domain-detach").click();
  await expect(card.getByTestId("storefront-domain-confirm")).toBeVisible();
  const detached = merchant.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === `/api/stores/${store}/storefront/domains/detach`);
  await card.getByTestId("storefront-domain-confirm-yes").click();
  assert.equal((await detached).status(), 200, "domain detach POST");
  await expect(rowFor(customHost)).toHaveAttribute("data-state", "DETACHED");
  await merchant.screenshot({ path: path.join(evidence, "domains-zh-CN-desktop-detached.png"), fullPage: true });
  audit.push("merchant.domain_detached");
  await notFound(customOrigin, "zh-CN", "desktop", "merchant detached the custom domain");
  pass("detach (zh-CN, desktop): row DETACHED, custom host stays closed");

  // ---- unpublish (en, desktop): the platform origin closes too ------------------------------------------------------
  card = await openCard(merchant, "en", "desktop");
  await toggle(card, "en", false);
  await notFound(platformOrigin, "en", "desktop", "merchant unpublished", "buyer-en-desktop-unpublished.png");
  pass("unpublish (en, desktop): platform origin 404");

  // ---- final facts --------------------------------------------------------------------------------------------------
  facts = await dbFacts(store);
  assert.deepEqual(facts.audits, audit, "PG audit trail equals the actions the gate drove");
  assert.equal(facts.publications.length, 1);
  assert.equal(facts.publications[0].version, 2, "one publish + one unpublish = version 2");
  assert.deepEqual(facts.domains.map(d => [d.origin, d.state]), [[platformOrigin, "ACTIVE"], [customOrigin, "DETACHED"]]);
  assert.deepEqual(uiErrors, []);
  pass("PG facts: handle + platform row from onboarding, publication version 2, custom DETACHED, audit trail equals the driven actions");
  await writeFile(path.join(evidence, "result.json"), JSON.stringify({ cases, store_id: store, tenant_id: receipt.tenant_id, handle: receipt.handle, origin: platformOrigin, custom_origin: customOrigin, locales: ["en", "zh-TW", "zh-CN"], viewports: ["desktop", "390px"], audit_expected: audit, contract_failures: contractFailures, boundary: "production Next; signed MOCK IdP; scripted public DNS + certificate probe on the production VerifyPending library; synthetic TLS/CONNECT edge; no deployment DNS/TLS, CA or provider acceptance" }, null, 2), { mode: 0o600 });
  assert.deepEqual(contractFailures, [], `contract checkpoints failed:\n${contractFailures.join("\n")}`);
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
