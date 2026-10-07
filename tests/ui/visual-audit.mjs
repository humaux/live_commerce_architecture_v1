// G-UI9 visual audit runner (unit ui-visual-audit, owner 2026-10-05). Started by tests/ui/click-sweep.mjs when LC_SWEEP_ONLY=visual-audit, i.e. by
// `bash scripts/dev/test-local.sh --browser-visual-lint`, on the SAME seeded stack as the G-UI8 click sweep (isolated PG, real Go API, production admin and
// storefront Next builds, signed MOCK IdP; tests/foundation/browser_click_sweep_test.go seeds it). This process never sees a database credential.
//
//   1. screenshot corpus: every admin registry route, every buyer storefront route and the five public platform-site pages, at 1586x992 and 390x844 in
//      zh-TW, zh-CN and en, full page, after the page settled (network + DOM quiet, fonts, lazy images); plus the primary form pages in their
//      "create new" state and the signed-out admin landing. A missing shot is a failure, never a silent skip.
//   2. layout lint: tests/ui/visual-lint-lib.mjs measures every page in place (getBoundingClientRect / getComputedStyle, read-only) before it is shot.
// Output: output/ui-visual-audit/<UTC>/{shots,crops,index.json,lint.json,lint.md}; output/ui-visual-audit/LATEST names the directory. Exit 1 on any blocking
// violation (R1 R2 R3 R6), any missing shot or any page that did not load; R4 R5 R7 R8 are WARN.
// Nothing here changes product state: navigation, reads and the same real Playwright actions the click sweep uses to reach a state (add to cart, place the
// COD order, add a variant axis in the empty product editor). page.evaluate only measures, scrolls to wake lazy images, and (for crops) draws a temporary outline.
import assert from "node:assert/strict";
import crypto from "node:crypto";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { expect } from "@playwright/test";
import { launch } from "../storefront/browser-engine.mjs";
import { routes as adminRoutes } from "../../apps/admin/src/routes.ts";
import { fixture as platformFixture } from "../admin/shell-fixture.mjs";
import { INIT_SCRIPT, degradedMessages, monitor, settle } from "./click-sweep-lib.mjs";
import { VISUAL_LOCALES, VISUAL_VIEWPORTS, assignShards, parseShard, routeId, shardReport } from "./sweep-shard-lib.mjs";
import { BLOCKING, BLOCKING_KINDS, RULES, SCROLL_POSITIONS, T, collect, countByRule, describe, evaluate, r10Occlusion, scrollToFraction, selectRecords } from "./visual-lint-lib.mjs";

export { VISUAL_LOCALES as LOCALES, VISUAL_VIEWPORTS as VIEWPORTS, routeId }; // the matrix lives in sweep-shard-lib.mjs: sweep-aggregate.mjs counts the same one
const LOCALES = VISUAL_LOCALES, VIEWPORTS = VISUAL_VIEWPORTS;
const sizeOf = (v) => `${v.size.width}x${v.size.height}`;
const ctxOptions = (v, locale, extra = {}) => ({ viewport: v.size, screen: v.size, isMobile: v.mobile, hasTouch: v.mobile, locale, deviceScaleFactor: 1, ignoreHTTPSErrors: true, ...extra });

// storefront routes of docs/engineering/ui-architecture.md section 4 (the same list the click sweep walks); cart/checkout/order need the cart the buyer fills.
const STOREFRONT = (facts) => {
  const product = facts.products[1];
  return [
    { route: "/", path: "" }, { route: "/products", path: "/products" }, { route: "/collections", path: "/collections" },
    { route: "/collections/[slug]", path: `/collections/${facts.collections[0].slug}` }, { route: "/products/[slug]", path: `/products/${product.slug}` },
    { route: "/search", path: `/search?q=${encodeURIComponent("Sweep")}` }, { route: "/orders/lookup", path: "/orders/lookup" },
    { route: "/order-link", path: "/order-link", expectDegraded: true }, { route: "/claim", path: "/claim", expectDegraded: true },
    { route: "/legal/anti-fraud", path: "/legal/anti-fraud" }, { route: "/legal/[slug]", path: "/legal/refunds" }, { route: "/privacy", path: "/privacy" },
    { route: "/data-deletion", path: "/data-deletion" }, { route: "/pages/[slug]", path: "/pages/about" },
  ];
};
const STOREFRONT_CART = [{ route: "/cart", path: "/cart" }, { route: "/checkout", path: "/checkout" }, { route: "/orders/[orderID]", path: null }];
const PLATFORM_PAGES = ["home", "privacy", "terms", "data-deletion", "contact"];
const PLATFORM = { host: "platform.example.invalid", adminHost: "admin.example.invalid", email: "contact@example.invalid" }; // the build-time values of test-local.sh

const copy = { // the two checkout buttons the buyer presses (apps/storefront/lib/purchase-copy.ts)
  "zh-TW": { delivery: "選擇配送", quote: "取得目前總額" }, "zh-CN": { delivery: "选择配送", quote: "获取当前总额" }, en: { delivery: "Choose delivery", quote: "Get current total" },
};
const pii = { recipient_name: "Synthetic Audit Recipient", phone: "+886900000092", region: "Synthetic Region", city: "Synthetic City", postal_code: "99992", line1: "Synthetic Address Ninety Two", line2: "Synthetic Unit Ninety Three" };

const sha = (buf) => crypto.createHash("sha256").update(buf).digest("hex");
const stamp = () => new Date().toISOString().replace(/[-:]/g, "").replace(/\.\d+Z$/, "Z");

export async function main() {
  const env = (name) => { const v = process.env[name]; assert(v, `${name} is required`); return v; };
  const adminOrigin = env("LC_SWEEP_ADMIN_ORIGIN"), store = env("LC_SWEEP_STORE"), evidence = env("LC_SWEEP_EVIDENCE");
  const facts = JSON.parse(await readFile(env("LC_SWEEP_FACTS"), "utf8"));
  const pageFilter = (process.env.LC_SWEEP_PAGES || "").split(",").filter(Boolean); // dev affordance: route substrings ("=/" = exactly "/")
  const wanted = (route) => pageFilter.length === 0 || pageFilter.some((f) => (f.startsWith("=") ? route === f.slice(1) : route.includes(f)));
  const WORKERS = Number(process.env.LC_SWEEP_WORKERS || 3); // the audit only reads: parallel contexts are safe, so sharding keeps the default
  const shard = parseShard(process.env.LC_SWEEP_SHARD); // i/N: this job shoots only its slice of the tasks (sweep-shard-lib.mjs); sweep-aggregate.mjs proves the slices cover everything
  const origin = facts.store_origin, HOST = new URL(origin).host;
  const root = process.cwd();
  const commit = (() => { try { return execFileSync("git", ["rev-parse", "--short=10", "HEAD"], { encoding: "utf8" }).trim(); } catch { return "unknown"; } })();
  const out = path.join(root, "output", "ui-visual-audit", stamp());
  await mkdir(path.join(out, "shots"), { recursive: true });
  await mkdir(path.join(out, "crops"), { recursive: true });
  await writeFile(path.join(root, "output", "ui-visual-audit", "LATEST"), `${path.relative(root, out)}\n`);
  const t0 = Date.now();
  const log = (...a) => console.log(`[${String(Math.round((Date.now() - t0) / 1000)).padStart(4)}s]`, ...a);

  // ---- stack: storefront Next behind a synthetic TLS edge, platform-site admin Next behind a Host bridge (the patterns of click-sweep.mjs and platform-runner.mjs)
  const children = new Set(), sockets = new Set(), logs = [];
  const listen = async (server) => { server.listen(0, "127.0.0.1"); await once(server, "listening"); return server.address().port; };
  const freePort = async () => { const probe = net.createServer(); const port = await listen(probe); await new Promise((r) => probe.close(r)); return port; };
  const relay = (port, req, body, host, hostname = "127.0.0.1") => new Promise((resolve, reject) => {
    const headers = { ...req.headers, ...(host ? { host } : {}) }; delete headers.connection; delete headers["transfer-encoding"];
    if (body.length) headers["content-length"] = String(body.length); else delete headers["content-length"];
    const call = http.request({ hostname, port, path: req.url, method: req.method, headers }, (r) => {
      const chunks = []; r.on("data", (x) => chunks.push(x)); r.on("end", () => resolve({ status: r.statusCode, headers: r.headers, body: Buffer.concat(chunks) })); r.on("error", reject);
    });
    call.setTimeout(60000, () => call.destroy(new Error("relay timeout"))); call.on("error", reject); call.end(body);
  });
  const startChild = async (name, file, args, cwd, childEnv, ready, nodeArgs = []) => {
    const logFile = createWriteStream(path.join(evidence, `${name}.log`), { flags: "w", mode: 0o600 }); logs.push(logFile); await once(logFile, "open");
    const child = spawn(process.execPath, [...nodeArgs, file, ...args], { cwd, env: childEnv, stdio: ["ignore", logFile, logFile] });
    children.add(child);
    for (let i = 0; i < 600; i++) {
      if (child.exitCode !== null) throw new Error(`${name} exited before readiness`);
      try { if (await ready()) return child; } catch { /* not up yet */ }
      await new Promise((r) => setTimeout(r, 100));
    }
    throw new Error(`${name} readiness timeout`);
  };
  const stripSweep = (e) => { for (const k of Object.keys(e)) if (k.startsWith("LC_SWEEP_")) delete e[k]; return e; };
  const sfPort = await freePort();
  await startChild("visual-audit-storefront-next", path.join(root, "apps/storefront/node_modules/next/dist/bin/next"), ["start", "--hostname", "127.0.0.1", "--port", String(sfPort)],
    path.join(root, "apps/storefront"), stripSweep({ ...process.env, NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1" }),
    async () => (await relay(sfPort, { url: "/robots.txt", method: "GET", headers: {} }, Buffer.alloc(0), HOST)).status === 200);
  const certDir = await mkdtemp(path.join(tmpdir(), "lc-va-edge-"));
  execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path.join(certDir, "key.pem"), "-out", path.join(certDir, "cert.pem"), "-days", "1", "-subj", `/CN=${HOST}`], { stdio: "ignore" });
  const edge = https.createServer({ key: await readFile(path.join(certDir, "key.pem")), cert: await readFile(path.join(certDir, "cert.pem")) }, async (req, res) => {
    try {
      const chunks = []; for await (const x of req) chunks.push(x);
      const r = await relay(sfPort, req, Buffer.concat(chunks), HOST);
      const headers = { ...r.headers }; delete headers["transfer-encoding"]; delete headers.connection;
      res.writeHead(r.status, headers); res.end(r.body);
    } catch { if (!res.headersSent) res.writeHead(502); res.end(); }
  });
  const edgePort = await listen(edge);
  const proxy = http.createServer((_, res) => { res.writeHead(403); res.end(); });
  proxy.on("connect", (req, socket, head) => {
    if (req.url !== `${HOST}:443`) { socket.destroy(); return; }
    const upstream = net.connect(edgePort, "127.0.0.1", () => { socket.write("HTTP/1.1 200 Connection Established\r\n\r\n"); if (head.length) upstream.write(head); socket.pipe(upstream).pipe(socket); });
    for (const s of [socket, upstream]) { sockets.add(s); s.on("close", () => sockets.delete(s)); s.on("error", () => { socket.destroy(); upstream.destroy(); }); }
  });
  const platformAuth = await platformFixture(); // the isolated MOCK API the public platform pages' server needs (tests/admin/shell-fixture.mjs)
  const pPort = await freePort();
  await startChild("visual-audit-platform-next", "apps/admin/.next/standalone/apps/admin/server.js", [], root, stripSweep({
    ...process.env, NODE_ENV: "production", HOSTNAME: "localhost", PORT: String(pPort), LC_PLATFORM_HOST: PLATFORM.host, LC_ADMIN_HOST: PLATFORM.adminHost, LC_COMPANY_CONTACT_EMAIL: PLATFORM.email,
    LC_META_DOMAIN_VERIFICATION: "", COMMERCE_IDENTITY_ENABLED: "1", COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS: "1", COMMERCE_PUBLIC_ORIGIN: `https://${PLATFORM.adminHost}`,
    COMMERCE_API_ORIGIN: platformAuth.origin, COMMERCE_BFF_KEY: platformAuth.key, COMMERCE_FIXTURE_ENABLED: "0", COMMERCE_PASSWORD_LOGIN_ENABLED: "1",
  }), async () => (await relay(pPort, { url: "/", method: "GET", headers: {} }, Buffer.alloc(0), PLATFORM.host, "localhost")).status === 200, ["--dns-result-order=ipv4first"]); // as tests/admin/platform-runner.mjs: the host rewrite targets localhost
  const browser = await launch({ headless: true, proxy: { server: `http://127.0.0.1:${await listen(proxy)}`, bypass: "127.0.0.1,localhost" } });
  log(`stack up: admin ${adminOrigin}, storefront ${origin} (Next :${sfPort}), platform ${PLATFORM.host} (Next :${pPort}); ${adminRoutes.length} registry routes`);

  // ---- enumeration: every unit that must produce a shot ---------------------------------------------------------------------------------------------------
  const q = (extra = "") => `?store=${store}${extra}`;
  const adminURL = (route, locale) => {
    const p = route.path;
    const sub = { "/products/[product]": `/products/${facts.products[0].id}`, "/customers/[customer]": `/customers/${facts.customer}`, "/invite/[token]": `/invite/${"A".repeat(43)}` }[p] ?? p;
    if (route.public) return `${adminOrigin}/${locale}${sub}`;
    if (p === "/studio/claims") return `${adminOrigin}/${locale}${sub}${q(`&scene=${facts.session}`)}`;
    if (p === "/orders/cvs-print") return `${adminOrigin}/${locale}${sub}${q(`&order=${facts.orders.cvs_label_created}&thermal=0`)}`;
    return `${adminOrigin}/${locale}${sub === "/" ? "" : sub}${q()}`;
  };
  // "create new" states: the empty product editor, and the same editor after the real clicks that add a Size axis (the variant matrix with its stock cells)
  const variantsByClicks = async (page) => {
    await expect(page.getByTestId("product-create-form")).toBeVisible();
    await page.getByTestId("axis-add").click();
    await page.getByTestId("axis-name-0").fill("Size");
    await page.getByTestId("axis-values-0").fill("S, M, L");
    await page.getByTestId("axis-values-0").press("Enter");
    await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(3);
  };
  const adminUnits = [];
  for (const r of adminRoutes) if (wanted(r.path)) adminUnits.push({ id: routeId(r.path), route: r.path, state: "registry", url: (l) => adminURL(r, l), auth: !r.public, expectExternal: r.path === "/orders/cvs-print" ? "ECPay" : "" });
  if (wanted("/products/new")) {
    adminUnits.push({ id: "products-new", route: "/products/new", state: "create-empty", url: (l) => `${adminOrigin}/${l}/products/new${q()}`, auth: true });
    adminUnits.push({ id: "products-new-variants", route: "/products/new", state: "create-with-variants", url: (l) => `${adminOrigin}/${l}/products/new${q()}`, auth: true, prepare: variantsByClicks });
  }
  if (wanted("=/")) adminUnits.push({ id: "signed-out-home", route: "/", state: "signed-out", url: (l) => `${adminOrigin}/${l}`, auth: false });
  const platformUnits = PLATFORM_PAGES.filter((n) => pageFilter.length === 0 || pageFilter.includes("platform") || wanted(`/${n}`)).map((name) => ({ id: name, route: name === "home" ? "/" : `/${name}`, state: "public", name }));
  const storefrontUnits = [...STOREFRONT(facts), ...STOREFRONT_CART].filter((r) => wanted(r.route)).map((r) => ({ id: routeId(r.route), ...r }));
  const universe = [];
  for (const l of LOCALES) for (const v of VIEWPORTS) {
    for (const u of adminUnits) universe.push({ app: "admin", id: u.id, locale: l, v });
    for (const u of storefrontUnits) universe.push({ app: "storefront", id: u.id, locale: l, v });
    for (const u of platformUnits) universe.push({ app: "platform", id: u.id, locale: l, v });
  }
  const key = (e) => `${e.app}|${e.id}|${e.locale}|${sizeOf(e.v)}`;
  // Sharding unit = one capture task: one admin or platform shot, or one storefront walk (a cart-bearing context per locale x viewport, never split).
  const taskKey = (e) => (e.app === "storefront" ? `storefront|${e.locale}|${e.v.name}` : key(e));
  const taskWeights = new Map();
  for (const e of universe) { const k = taskKey(e); taskWeights.set(k, e.app === "storefront" ? (taskWeights.get(k) ?? 8) + 1 : 1); } // a storefront walk also fills the cart and places the COD order (~8 shots of work)
  const owner = assignShards([...taskWeights].map(([k, weight]) => ({ key: k, weight })), shard?.of ?? 1);
  const mine = (k) => !shard || owner.get(k) === shard.index; // unsharded: everything is mine
  const expected = universe.filter((e) => mine(taskKey(e)));

  // ---- capture one page: settle, measure, shoot, crop ------------------------------------------------------------------------------------------------------
  const results = [];
  const warm = (page) => page.evaluate(async () => { // [READ/MEASURE] scrolls the document so lazy images load, awaits fonts + images, returns to the top
    const step = Math.max(300, window.innerHeight * 0.8);
    for (let y = 0, n = 0; y < document.documentElement.scrollHeight && n < 40; y += step, n++) { window.scrollTo(0, y); await new Promise((r) => setTimeout(r, 60)); }
    window.scrollTo(0, 0);
    await document.fonts.ready;
    await Promise.all([...document.images].map((i) => (i.complete ? null : new Promise((r) => { i.addEventListener("load", r, { once: true }); i.addEventListener("error", r, { once: true }); setTimeout(r, 4000); }))));
  }).catch(() => {});
  const FREEZE = "*,*::before,*::after{transition:none!important;animation:none!important;caret-color:transparent!important;scroll-behavior:auto!important}";
  async function crop(page, v, file) {
    const dims = await page.evaluate(() => ({ w: document.documentElement.scrollWidth, h: document.documentElement.scrollHeight }));
    const doc = v.kind === "document-scroll-width", r = v.rect, pad = 24;
    const x = doc ? Math.max(0, dims.w - 1200) : Math.max(0, Math.floor(r.x - pad)), y = doc ? 0 : Math.max(0, Math.floor(r.y - pad));
    const w = Math.min(doc ? 1200 : Math.ceil(r.w + 2 * pad), 1200, dims.w - x), h = Math.min(doc ? 700 : Math.ceil(r.h + 2 * pad), 700, dims.h - y);
    if (w < 24 || h < 16) return null;
    if (!doc) await page.evaluate((b) => { // [READ/MEASURE evidence only] a temporary outline around the finding for the crop; removed right after
      const d = document.createElement("div"); d.setAttribute("data-va-outline", "1");
      d.style.cssText = `position:absolute;left:${b.x}px;top:${b.y}px;width:${b.w}px;height:${b.h}px;outline:3px solid #ff0066;box-sizing:border-box;pointer-events:none;z-index:2147483647`;
      document.documentElement.appendChild(d);
    }, r);
    await mkdir(path.dirname(file), { recursive: true });
    await page.screenshot({ path: file, clip: { x, y, width: w, height: h }, fullPage: true, animations: "disabled", caret: "hide" }).catch(() => {});
    await page.evaluate(() => document.querySelectorAll("[data-va-outline]").forEach((n) => n.remove())).catch(() => {});
    return path.relative(out, file);
  }
  async function capture({ page, mon }, unit) {
    const rec = { app: unit.app, route: unit.route, id: unit.id, state: unit.state, locale: unit.locale, viewport: unit.v.name, size: sizeOf(unit.v), url: "", file: "", sha256: "", bytes: 0, status: 0, ok: false, issues: [] };
    const file = path.join(out, "shots", unit.app, unit.id, `${unit.locale}-${sizeOf(unit.v)}.png`);
    const started = Date.now();
    try {
      try { new URL(unit.url); rec.url = new URL(unit.url).pathname + new URL(unit.url).search; } catch { rec.url = unit.url; }
      mon?.seenAt?.clear();
      let response = await page.goto(unit.url, { waitUntil: "load", timeout: 45000 }).catch((e) => ({ error: e }));
      if (response?.error) response = await page.goto(unit.url, { waitUntil: "load", timeout: 45000 }).catch((e) => ({ error: e })); // one retry
      if (response?.error) throw response.error;
      rec.status = response?.status?.() ?? 0;
      await settle(page, mon ?? { idle: () => true }, { quiet: 500, max: 10000 });
      await page.waitForLoadState("networkidle", { timeout: 4000 }).catch(() => {});
      if (unit.prepare) {
        try { await unit.prepare(page); await settle(page, mon ?? { idle: () => true }, { quiet: 400, max: 4000 }); }
        catch (e) { rec.stateFailed = true; rec.issues.push(`state-not-reached: ${String(e.message || e).replace(/\x1b\[[0-9;]*m/g, "").split("\n").filter((l) => l.trim())[0].slice(0, 160)}`); } // the shot is still taken, and flagged
      }
      await page.waitForFunction(() => !document.querySelector('[aria-busy="true"]'), null, { timeout: 4000 }).catch(() => {});
      if (unit.expectExternal) { // a route whose job is to hand the browser to another host: judged only if its own document is on screen
        const body = await page.locator("body").innerText().catch(() => "");
        const onOwnHost = new URL(page.url()).host === new URL(adminOrigin).host;
        if (!onOwnHost || /external request blocked|blocked external/i.test(body) || body.trim().length < 20) {
          rec.notRun = `NOT_RUN: this route hands the browser to ${unit.expectExternal} and the harness could not keep the page's own document on screen (url ${onOwnHost ? "unchanged" : "left the stack"}, body: ${JSON.stringify(body.trim().slice(0, 80))})`;
          rec.issues.push(rec.notRun);
          results.push({ ...rec, ms: Date.now() - started, counts: {}, groups: {}, blocking: 0, blockingCounts: {}, violations: [], notRun: true });
          log(`${unit.app} ${unit.id} ${unit.locale}/${unit.v.name} ${rec.notRun}`);
          return;
        }
        rec.issues.push(`harness: the external ${unit.expectExternal} navigation is cancelled so the page's own state is shown`);
      }
      await warm(page);
      await page.addStyleTag({ content: FREEZE }).catch(() => {}); // evidence stabilisation: no transition or animation frame is caught half way
      const degraded = unit.expectDegraded ? [] : await degradedMessages(page);
      for (const d of degraded) rec.issues.push(`degraded: [${d.role}] ${d.text}`.slice(0, 200));
      // measure first, in the state the user sees at the top of the page
      const snapshot = await page.evaluate(collect); // [READ/MEASURE] page.evaluate only reads geometry and styles
      const violations = evaluate(snapshot, { mobile: unit.v.mobile });
      const named = violations.length ? await page.evaluate(describe, [...new Set(violations.flatMap((x) => x.ids))]) : {};
      const pathFor = (x) => (x.ids.length ? named[x.ids[0]]?.path ?? "" : "document");
      const { picked, groupCounts } = selectRecords(violations, pathFor);
      const buf = await page.screenshot({ fullPage: true, animations: "disabled", caret: "hide" });
      await mkdir(path.dirname(file), { recursive: true });
      await writeFile(file, buf);
      rec.file = path.relative(out, file); rec.sha256 = sha(buf); rec.bytes = buf.length;
      rec.pageHeight = snapshot.doc.scrollHeight; rec.ok = rec.status >= 200 && rec.status < 400;
      const records = [];
      let n = 0;
      for (const v of picked) {
        const first = named[v.ids[0]] ?? {};
        const c = await crop(page, v, path.join(out, "crops", unit.app, unit.id, `${unit.locale}-${sizeOf(unit.v)}-${v.rule}-${++n}.png`));
        records.push({ rule: v.rule, kind: v.kind, severity: v.severity, path: pathFor(v), tag: first.tag ?? "", testid: first.testid ?? "", text: first.text ?? "", rect: v.rect, measured: v.measured, group: v.group, crop: c });
      }
      const counts = countByRule(violations), groups = { ...groupCounts };
      const blockingCounts = countByRule(violations.filter((x) => x.severity === "block"));
      // R10: the same page scrolled to the middle and to the bottom; a fixed/sticky bar that covers main content there (each stage measured and cropped in its own scroll state)
      for (const [position, fraction] of SCROLL_POSITIONS) {
        if (!(await scrollToFraction(page, fraction))) break;
        const occ = r10Occlusion(await page.evaluate(collect), position); // [READ/MEASURE]
        if (!occ.length) continue;
        const names = await page.evaluate(describe, [...new Set(occ.flatMap((x) => x.ids))]);
        const covered = (x) => names[x.ids[1]]?.path ?? "";
        const sel = selectRecords(occ, covered);
        let m = 0;
        for (const v of sel.picked) {
          const first = names[v.ids[1]] ?? {};
          const c = await crop(page, v, path.join(out, "crops", unit.app, unit.id, `${unit.locale}-${sizeOf(unit.v)}-R10-${position}-${++m}.png`));
          records.push({ rule: v.rule, kind: v.kind, severity: v.severity, path: covered(v), tag: first.tag ?? "", testid: first.testid ?? "", text: first.text ?? "", rect: v.rect, measured: { ...v.measured, bar: names[v.ids[0]]?.path ?? "" }, group: v.group, crop: c });
        }
        counts.R10 = (counts.R10 ?? 0) + occ.length; groups.R10 = (groups.R10 ?? 0) + (sel.groupCounts.R10 ?? 0);
      }
      await page.evaluate(() => window.scrollTo(0, 0)).catch(() => {}); // [READ/MEASURE]
      results.push({ ...rec, ms: Date.now() - started, counts, groups, blocking: Object.values(blockingCounts).reduce((a, b) => a + b, 0), blockingCounts, violations: records });
    } catch (e) {
      rec.issues.push(`capture-error: ${String(e.message || e).split("\n")[0].slice(0, 200)}`);
      results.push({ ...rec, ms: Date.now() - started, counts: {}, groups: {}, blocking: 0, blockingCounts: {}, violations: [], failed: true });
    }
    const done = results[results.length - 1];
    log(`${unit.app} ${unit.id} ${unit.locale}/${unit.v.name} http ${rec.status}${rec.file ? "" : " NO SHOT"}${done.stateFailed ? " STATE NOT REACHED" : ""} ${JSON.stringify(done.counts)}`);
  }

  // ---- the three apps ------------------------------------------------------------------------------------------------------------------------------------------
  const newContext = async (v, locale, extra = {}, withMonitor = true) => {
    const c = await browser.newContext(ctxOptions(v, locale, extra));
    await c.addInitScript(INIT_SCRIPT);
    const mon = withMonitor ? monitor(c, { allowedHosts: new Set([new URL(adminOrigin).host, HOST]) }) : null;
    // /orders/cvs-print auto-posts a signed form to ECPay top-level; the harness cancels that navigation (registered after the monitor, so it runs first) so the
    // page's own document stays on screen instead of the monitor's "blocked external" stub. No request leaves the stack.
    if (withMonitor) await c.route((u) => /(^|\.)ecpay\.com\.tw$/i.test(u.hostname), (route) => route.abort("aborted"));
    return { c, mon, page: await c.newPage() };
  };
  const loginState = async () => {
    const c = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await c.newPage();
    await page.goto(`${adminOrigin}/en/`);
    await page.getByRole("button", { name: "Sign in with identity service" }).click();
    await expect(page.getByTestId("nav-orders")).toBeVisible();
    const state = await c.storageState();
    await c.close();
    return state;
  };
  const adminTask = (state, u, locale, v) => async () => {
    const s = await newContext(v, locale, u.auth ? { storageState: state } : {});
    try { await capture(s, { app: "admin", id: u.id, route: u.route, state: u.state, url: u.url(locale), locale, v, prepare: u.prepare, expectExternal: u.expectExternal }); } finally { await s.c.close().catch(() => {}); }
  };
  const bridge = async (context) => { // the platform site is served by Host: a context bridge keeps the real host names without DNS or TLS (tests/admin/platform-runner.mjs)
    await context.route("**/*", async (route) => {
      const u = new URL(route.request().url());
      if (![PLATFORM.host, PLATFORM.adminHost].includes(u.hostname)) return route.abort();
      const r = await relay(pPort, { url: u.pathname + u.search, method: "GET", headers: {} }, Buffer.alloc(0), u.hostname, "localhost");
      const headers = Object.fromEntries(Object.entries(r.headers).filter(([k]) => !["transfer-encoding", "content-length", "content-encoding"].includes(k)));
      await route.fulfill({ status: r.status, headers, body: r.body });
    });
  };
  const platformTask = (u, locale, v) => async () => {
    const s = await newContext(v, locale, {}, false);
    try {
      await bridge(s.c);
      const p = `${locale === "zh-TW" ? "" : `/${locale}`}${u.name === "home" ? "" : `/${u.name}`}` || "/";
      await capture(s, { app: "platform", id: u.id, route: u.route, state: u.state, url: `https://${PLATFORM.host}${p}`, locale, v });
    } finally { await s.c.close().catch(() => {}); }
  };
  async function fillCart(page, locale, product) { // the click path of click-sweep.mjs placeCodOrder: product page -> cart -> checkout -> delivery -> quote -> address -> COD -> place
    const c = copy[locale];
    await page.goto(`${origin}/${locale}/products`);
    await page.getByTestId("product-card").filter({ hasText: product.title }).first().click();
    await page.waitForURL(`**/${locale}/products/${product.slug}`);
    await page.getByTestId("add-to-cart").click();
    await page.getByTestId("cart-checkout").waitFor();
    return async () => {
      await page.goto(`${origin}/${locale}/cart`);
      await page.getByTestId("cart-checkout").click();
      await page.waitForURL(`**/${locale}/checkout`);
      await page.getByRole("button", { name: c.delivery, exact: true }).click();
      const select = page.locator("select#delivery");
      await expect(select).toBeVisible();
      const options = await select.locator("option").evaluateAll((os) => os.map((o) => ({ value: o.value, text: o.textContent }))); // [READ/MEASURE] option list
      const home = options.find((o) => o.value && !/7-ELEVEN|全家|FamilyMart|萊爾富|莱尔富|Hi-Life|OK mart|OK超商|交貨便|交货便/i.test(o.text));
      assert(home, `no home delivery option in ${JSON.stringify(options)}`);
      await select.selectOption(home.value);
      await page.getByRole("button", { name: c.quote, exact: true }).click();
      await expect(page.getByTestId("address-section")).toBeVisible();
      for (const [k, value] of Object.entries(pii)) await page.locator(`input[name="${k}"]`).fill(value);
      await page.locator('input[name="home_payment_mode"][value="cash_on_delivery"]').check();
      await page.getByTestId("confirm-address").click();
      await expect(page.getByTestId("create-order")).toBeEnabled();
      await page.getByTestId("create-order").click();
      await expect(page.getByTestId("order-section")).toBeVisible({ timeout: 30000 });
      return (await page.getByTestId("order-id").innerText()).trim();
    };
  }
  const storefrontTask = (locale, v) => async () => {
    const s = await newContext(v, locale);
    const unit = (u, url) => ({ app: "storefront", id: u.id, route: u.route, state: "registry", url, locale, v, expectDegraded: !!u.expectDegraded });
    try {
      const needsCart = storefrontUnits.some((u) => STOREFRONT_CART.some((c) => c.route === u.route));
      let orderID = null;
      if (needsCart) {
        const buy = await fillCart(s.page, locale, facts.products[1]);
        for (const u of storefrontUnits.filter((x) => x.route === "/cart" || x.route === "/checkout")) await capture(s, unit(u, `${origin}/${locale}${u.path}`));
        try { orderID = await buy(); } catch (e) { log(`storefront ${locale}/${v.name}: placing the COD order failed: ${String(e.message || e).split("\n")[0]}`); }
        const u = storefrontUnits.find((x) => x.route === "/orders/[orderID]");
        if (u && orderID) await capture(s, unit(u, `${origin}/${locale}/orders/${orderID}`));
      }
      for (const u of storefrontUnits) if (u.path !== null && !STOREFRONT_CART.some((c) => c.route === u.route)) await capture(s, unit(u, `${origin}/${locale}${u.path}`));
    } finally { await s.c.close().catch(() => {}); }
  };
  async function pool(tasks, n) {
    const queue = [...tasks];
    await Promise.all(Array.from({ length: n }, async () => { for (let t = queue.shift(); t; t = queue.shift()) await t().catch((e) => log(`task failed: ${String(e.message || e).split("\n")[0]}`)); }));
  }

  let exitCode = 0;
  try {
    const state = await loginState();
    const tasks = [];
    for (const l of LOCALES) for (const v of VIEWPORTS) {
      if (mine(`storefront|${l}|${v.name}`)) tasks.push(storefrontTask(l, v));
      for (const u of platformUnits) if (mine(`platform|${u.id}|${l}|${sizeOf(v)}`)) tasks.push(platformTask(u, l, v));
      for (const u of adminUnits) if (mine(`admin|${u.id}|${l}|${sizeOf(v)}`)) tasks.push(adminTask(state, u, l, v));
    }
    await pool(tasks, WORKERS);

    // ---- coverage: every enumerated unit has a shot, or is explicitly NOT_RUN with its reason ----------------------------------------------------------------
    const have = new Set(results.filter((r) => r.file).map((r) => `${r.app}|${r.id}|${r.locale}|${r.size}`));
    const notRun = results.filter((r) => r.notRun).map((r) => ({ unit: `${r.app}|${r.id}|${r.locale}|${r.size}`, reason: r.notRun }));
    const notRunKeys = new Set(notRun.map((n) => n.unit));
    const missing = expected.filter((e) => !have.has(key(e)) && !notRunKeys.has(key(e))).map(key);
    const loadFailures = results.filter((r) => r.file && !r.ok).map((r) => `${r.app}|${r.id}|${r.locale}|${r.size} HTTP ${r.status}`);
    const stateFailures = results.filter((r) => r.stateFailed).map((r) => `${r.app}|${r.id}|${r.locale}|${r.size}`);
    assert.equal(new Set(universe.map(key)).size, universe.length, "route ids must be unique per app");
    results.sort((a, b) => `${a.app}|${a.id}|${a.locale}|${a.size}`.localeCompare(`${b.app}|${b.id}|${b.locale}|${b.size}`));
    const blockingInstances = results.reduce((n, r) => n + (r.blocking ?? 0), 0);
    const totals = {};
    for (const rule of Object.keys(RULES)) {
      const units = results.filter((r) => r.counts[rule]);
      totals[rule] = {
        name: RULES[rule], severity: BLOCKING.includes(rule) ? "block" : BLOCKING_KINDS.some((k) => k.startsWith(`${rule}:`)) ? "block for controls, warn for labels/links" : "warn",
        instances: units.reduce((n, r) => n + r.counts[rule], 0), blocking: results.reduce((n, r) => n + (r.blockingCounts?.[rule] ?? 0), 0), groups: units.reduce((n, r) => n + (r.groups[rule] ?? 0), 0), units: units.length,
      };
    }
    const byApp = {}, byRouteMap = new Map();
    for (const u of results) {
      const a = (byApp[u.app] ??= {}), r = byRouteMap.get(`${u.app}|${u.id}`) ?? byRouteMap.set(`${u.app}|${u.id}`, { app: u.app, id: u.id, route: u.route, state: u.state, counts: {}, blocking: 0, warn: 0, total: 0, notRun: 0 }).get(`${u.app}|${u.id}`);
      for (const [rule, n] of Object.entries(u.counts)) { a[rule] = (a[rule] ?? 0) + n; r.counts[rule] = (r.counts[rule] ?? 0) + n; r.total += n; }
      r.blocking += u.blocking ?? 0; r.warn = r.total - r.blocking; if (u.notRun) r.notRun += 1;
    }
    const byRoute = [...byRouteMap.values()].sort((x, y) => y.total - x.total || x.id.localeCompare(y.id));
    const reasons = [];
    if (blockingInstances) reasons.push(`${blockingInstances} blocking violation instances (${BLOCKING.join("/")} and R7 clipped controls)`);
    if (missing.length) reasons.push(`${missing.length} enumerated shots missing`);
    if (loadFailures.length) reasons.push(`${loadFailures.length} pages answered HTTP >= 400`);
    if (stateFailures.length) reasons.push(`${stateFailures.length} "create new" states were not reached`);
    exitCode = reasons.length ? 1 : 0;
    const report = {
      generated: new Date().toISOString(), commit, out: path.relative(root, out), thresholds: T, rules: RULES, blocking: BLOCKING, blockingKinds: BLOCKING_KINDS,
      matrix: { locales: LOCALES, viewports: VIEWPORTS.map((v) => ({ name: v.name, size: sizeOf(v) })), apps: { admin: adminUnits.length, storefront: storefrontUnits.length, platform: platformUnits.length } },
      shots: { expected: expected.length, captured: have.size, notRun, missing, loadFailures, stateFailures }, totals, blockingInstances, verdict: { exit: exitCode, reasons },
      shard: shard ? shardReport(shard, universe.map(key), expected.map(key), { complete: pageFilter.length === 0 }) : null,
      units: results.map(({ file, sha256, bytes, ms, ...r }) => ({ ...r, shot: file })),
    };
    await writeFile(path.join(out, "index.json"), JSON.stringify({
      generated: report.generated, commit, out: report.out, matrix: report.matrix, expected: expected.length, captured: have.size, notRun, missing,
      shots: results.filter((r) => r.file).map((r) => ({ app: r.app, route: r.route, id: r.id, state: r.state, url: r.url, locale: r.locale, viewport: r.viewport, size: r.size, file: r.file, sha256: r.sha256, bytes: r.bytes, status: r.status, pageHeight: r.pageHeight, issues: r.issues })),
    }, null, 1));
    await writeFile(path.join(out, "lint.json"), JSON.stringify(report, null, 1));
    await writeFile(path.join(out, "counts.json"), JSON.stringify({ commit, generated: report.generated, shots: { expected: expected.length, captured: have.size, notRun: notRun.length }, blockingInstances, totals, byApp, byRoute }, null, 1));
    await writeFile(path.join(out, "lint.md"), renderMarkdown(report));
    log(`shots ${have.size}/${expected.length}${notRun.length ? ` (NOT_RUN ${notRun.length})` : ""}; blocking instances ${blockingInstances}; ${Object.entries(totals).map(([k, v]) => `${k} ${v.instances}`).join(", ")}`);
    console.log("top routes by violations: " + byRoute.slice(0, 10).map((r) => `${r.app}${r.route} (${r.id}) ${r.total} [block ${r.blocking}]`).join("; "));
    for (const r of reasons) console.log(`FAIL G-UI9: ${r}`);
    if (!reasons.length) console.log("PASS G-UI9 visual lint");
    console.log(`G-UI9 evidence: ${path.relative(root, out)}`);
  } catch (e) {
    console.error(e);
    exitCode = 1;
  } finally {
    await browser.close().catch(() => {});
    for (const s of sockets) s.destroy();
    proxy.close(); edge.close(); await platformAuth.close().catch(() => {});
    for (const c of children) c.kill("SIGKILL");
    for (const l of logs) l.end();
    await rm(certDir, { recursive: true, force: true });
  }
  return exitCode;
}

// ---- lint.md: grouped by app / route, most violations first -----------------------------------------------------------------------------------------------
export function renderMarkdown(report) {
  const rules = Object.keys(RULES);
  const units = report.units;
  const L = [];
  L.push(`# G-UI9 visual lint — ${report.commit} — ${report.generated}`, "");
  L.push(`Verdict: **${report.verdict.exit === 0 ? "PASS" : "FAIL"}**${report.verdict.reasons.length ? ` — ${report.verdict.reasons.join("; ")}` : ""}`, "");
  const sh = report.shots;
  L.push(`Shots: ${sh.captured} captured of ${sh.expected} enumerated (${report.matrix.apps.admin} admin + ${report.matrix.apps.storefront} storefront + ${report.matrix.apps.platform} platform pages x ${report.matrix.locales.length} locales x ${report.matrix.viewports.length} viewports ${report.matrix.viewports.map((v) => v.size).join(", ")}). NOT_RUN: ${sh.notRun.length}. Missing: ${sh.missing.length}. HTTP >= 400: ${sh.loadFailures.length}. States not reached: ${sh.stateFailures.length}.`, "");
  if (sh.notRun.length) L.push("NOT_RUN (explicit, never reported as zero findings):", ...sh.notRun.map((m) => `- ${m.unit}: ${m.reason}`), "");
  if (sh.missing.length) L.push("Missing shots:", ...sh.missing.map((m) => `- ${m}`), "");
  if (sh.loadFailures.length) L.push("Pages that answered HTTP >= 400:", ...sh.loadFailures.map((m) => `- ${m}`), "");
  if (sh.stateFailures.length) L.push("States not reached:", ...sh.stateFailures.map((m) => `- ${m}`), "");
  L.push(`Blocking (exit 1): ${report.blocking.join(", ")} and ${report.blockingKinds.join(", ")}; every other finding is WARN for now. Thresholds: \`tests/ui/visual-lint-lib.mjs\` (T). Instances = every measured occurrence; groups = distinct (rule, kind, selector shape).`, "");
  L.push("## Counts per rule", "", "| rule | what | severity | instances | of which blocking | groups | page-variants affected |", "| --- | --- | --- | ---: | ---: | ---: | ---: |");
  for (const r of rules) { const t = report.totals[r]; L.push(`| ${r} | ${t.name} | ${t.severity} | ${t.instances} | ${t.blocking} | ${t.groups} | ${t.units} |`); }
  L.push("", "## Counts per app (instances)", "", `| app | page-variants | ${rules.join(" | ")} | blocking |`, `| --- | ---: | ${rules.map(() => "---:").join(" | ")} | ---: |`);
  for (const app of [...new Set(units.map((u) => u.app))]) {
    const us = units.filter((u) => u.app === app);
    L.push(`| ${app} | ${us.length} | ${rules.map((r) => us.reduce((n, u) => n + (u.counts[r] ?? 0), 0)).join(" | ")} | ${us.reduce((n, u) => n + (u.blocking ?? 0), 0)} |`);
  }
  const routes = new Map();
  for (const u of units) { const k = `${u.app}|${u.id}`; (routes.get(k) ?? routes.set(k, { app: u.app, id: u.id, route: u.route, state: u.state, units: [] }).get(k)).units.push(u); }
  const sum = (r, f) => r.units.reduce((n, u) => n + f(u), 0);
  const ranked = [...routes.values()].map((r) => ({ ...r, block: sum(r, (u) => u.blocking ?? 0), total: sum(r, (u) => Object.values(u.counts).reduce((a, b) => a + b, 0)) })).map((r) => ({ ...r, warn: r.total - r.block }))
    .sort((a, b) => b.total - a.total || a.id.localeCompare(b.id));
  L.push("", "## Top 15 routes by violations", "", "| # | app | route | state | blocking | warn | total |", "| ---: | --- | --- | --- | ---: | ---: | ---: |");
  ranked.slice(0, 15).forEach((r, i) => L.push(`| ${i + 1} | ${r.app} | \`${r.route}\` (${r.id}) | ${r.state} | ${r.block} | ${r.warn} | ${r.total} |`));
  for (const app of [...new Set(ranked.map((r) => r.app))]) {
    L.push("", `## ${app}`);
    for (const r of ranked.filter((x) => x.app === app)) {
      L.push("", `### \`${r.route}\` (${r.id}, ${r.state}) — blocking ${r.block}, warn ${r.warn}`, "", `| variant | ${rules.join(" | ")} | shot |`, `| --- | ${rules.map(() => "---:").join(" | ")} | --- |`);
      for (const u of [...r.units].sort((a, b) => `${a.locale}${a.size}`.localeCompare(`${b.locale}${b.size}`)))
        L.push(`| ${u.locale} ${u.size}${u.status && u.status >= 400 ? ` HTTP ${u.status}` : ""} | ${rules.map((x) => u.counts[x] ?? 0).join(" | ")} | ${u.notRun ? "**NOT_RUN**" : u.shot ? `\`${u.shot}\`` : "**MISSING**"} |`);
      for (const u of r.units.filter((x) => x.notRun).slice(0, 1)) L.push("", `NOT_RUN: ${u.notRun}`);
      for (const rule of rules) {
        const worst = [...r.units].sort((a, b) => (b.blockingCounts?.[rule] ?? 0) - (a.blockingCounts?.[rule] ?? 0))[0];
        if (!worst || !worst.blockingCounts?.[rule]) continue;
        L.push("", `${rule} ${RULES[rule]} (blocking) — worst variant ${worst.locale} ${worst.size} (${worst.blockingCounts[rule]} blocking of ${worst.counts[rule]} instances, ${worst.groups[rule]} groups):`);
        for (const v of worst.violations.filter((x) => x.rule === rule && x.severity === "block").slice(0, 3)) L.push(`- ${v.kind} \`${v.path || "document"}\` rect ${[v.rect.x, v.rect.y, v.rect.w, v.rect.h].join(",")} measured ${JSON.stringify(v.measured).slice(0, 260)}${v.crop ? ` crop \`${v.crop}\`` : ""}`);
      }
    }
  }
  L.push("");
  return L.join("\n");
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) process.exit(await main());
