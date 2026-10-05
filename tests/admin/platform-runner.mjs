// PS1/PS2/PS4: production Next, real Chromium clicks. No Go/PG/provider or live account writes.
// Local transport bridge preserves actual Host and HTTPS browser URLs; PS3 separately
// checks the unchanged Caddy edge allowlist and www redirect with a real local container.
import assert from "node:assert/strict";
import { chromium } from "@playwright/test";
import { spawn } from "node:child_process";
import { request } from "node:http";
import { createServer } from "node:net";
import { mkdir, open, writeFile } from "node:fs/promises";
import path from "node:path";
import { fixture } from "./shell-fixture.mjs";
import { assertPlatformMessagingCopy } from "./platform-messaging-copy.fixture.mjs";

const output = process.env.LC_PLATFORM_EVIDENCE || "output/platform-site";
await mkdir(output, { recursive: true });
const host = "platform.example.invalid",
  adminHost = "admin.example.invalid",
  origin = `https://${host}`;
const locales = ["zh-TW", "zh-CN", "en"],
  pages = ["home", "privacy", "terms", "data-deletion", "contact"];
const facts = [
  "Hong Kong Da Wan Trading Limited",
  "香港大碗貿易有限公司",
  "81215167",
  "81215167-000-09-26-2",
  "2026-09-11",
  "11/09/2026 – 10/09/2027",
  "RM 10, 23/F, New Trend Centre, 704 Prince Edward Road East, San Po Kong, Hong Kong",
  "contact@example.invalid",
];
const routePath = (l, p = "home") =>
  `${l === "zh-TW" ? "" : `/${l}`}${p === "home" ? "" : `/${p}`}` || "/";
const ledger = [];
let browser;
const auth = await fixture();
async function port() {
  const s = createServer();
  await new Promise((r) => s.listen(0, "127.0.0.1", r));
  const p = s.address().port;
  await new Promise((r) => s.close(r));
  return p;
}
function get(port, host, pathname, extra = {}) {
  return new Promise((resolve, reject) => {
    const r = request(
      {
        hostname: "localhost",
        port,
        path: pathname,
        headers: { host, ...extra },
        timeout: 20000,
      },
      (response) => {
        const chunks = [];
        response.on("data", (c) => chunks.push(c));
        response.on("end", () =>
          resolve({
            status: response.statusCode,
            headers: response.headers,
            body: Buffer.concat(chunks),
          }),
        );
      },
    );
    r.on("timeout", () => r.destroy(new Error("Next request timed out")));
    r.on("error", reject);
    r.end();
  });
}
async function server(verification) {
  const p = await port(),
    log = await open(
      `${output}/server-${verification ? "tag" : "no-tag"}.log`,
      "w",
    );
  const child = spawn(
    process.execPath,
    [
      "--dns-result-order=ipv4first",
      "apps/admin/.next/standalone/apps/admin/server.js",
    ],
    {
      stdio: ["ignore", log.fd, log.fd],
      env: {
        ...process.env,
        NODE_ENV: "production",
        HOSTNAME: "localhost",
        PORT: String(p),
        LC_PLATFORM_HOST: host,
        LC_ADMIN_HOST: adminHost,
        LC_COMPANY_CONTACT_EMAIL: "contact@example.invalid",
        LC_META_DOMAIN_VERIFICATION: verification,
        COMMERCE_IDENTITY_ENABLED: "1",
        COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS: "1",
        COMMERCE_PUBLIC_ORIGIN: `https://${adminHost}`,
        COMMERCE_API_ORIGIN: auth.origin,
        COMMERCE_BFF_KEY: auth.key,
        COMMERCE_FIXTURE_ENABLED: "0",
        COMMERCE_PASSWORD_LOGIN_ENABLED: "1",
      },
    },
  );
  try {
    for (let i = 0; i < 100; i++) {
      if (child.exitCode !== null) throw new Error("Next exited");
      try {
        if ((await get(p, host, "/")).status === 200) return { p, child, log };
      } catch {}
      await new Promise((r) => setTimeout(r, 200));
    }
    throw new Error("Next readiness timeout");
  } catch (e) {
    child.kill("SIGTERM");
    await log.close();
    throw e;
  }
}
async function stop(s) {
  const ended = new Promise((r) => s.child.once("exit", r));
  s.child.kill("SIGTERM");
  await ended;
  await s.log.close();
}
async function matrix(s) {
  const context = await browser.newContext();
  await context.route("**/*", async (route) => {
    const u = new URL(route.request().url());
    if (![host, adminHost].includes(u.hostname))
      throw new Error(`Unexpected external network: ${u.hostname}`);
    const r = await get(s.p, u.hostname, u.pathname + u.search);
    const headers = Object.fromEntries(
      Object.entries(r.headers).filter(
        ([k]) =>
          !["transfer-encoding", "content-length", "content-encoding"].includes(
            k,
          ),
      ),
    );
    await route.fulfill({ ...r, headers });
  });
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  for (const locale of locales)
    for (const width of [390, 1586]) {
      await page.setViewportSize({ width, height: width === 390 ? 844 : 992 });
      for (const name of pages) {
        const pathname = routePath(locale, name),
          r = await page.goto(origin + pathname, { waitUntil: "networkidle" });
        assert.equal(r.status(), 200);
        assert.equal(await page.locator("html").getAttribute("lang"), locale);
        assert.equal(
          await page.locator('meta[name="robots"][content*="noindex"]').count(),
          0,
        );
        assert.equal(
          await page
            .locator('meta[name="facebook-domain-verification"]')
            .count(),
          0,
        );
        assert.equal(
          await page.locator('link[rel="canonical"]').getAttribute("href"),
          origin + pathname,
        );
        assert.match(await page.title(), /^DaWan Live · /);
        for (const fact of facts)
          assert.ok(
            (await page.locator("footer").innerText()).includes(fact),
            `${name}/${locale} footer missing ${fact}`,
          );
        if (name !== "home")
          for (const fact of facts)
            assert.ok(
              (await page.locator("main").innerText()).includes(fact),
              `${name}/${locale} main missing ${fact}`,
            );
        if (name === "privacy" || name === "terms") {
          assertPlatformMessagingCopy(
            (await page.locator(".ps-legal-section").allTextContents()).join(
              " ",
            ),
            locale,
            name,
          );
          ledger.push({
            locale,
            width,
            page: name,
            action: "messaging-copy",
            result: "PASS",
          });
        }
        assert.equal(
          await page.evaluate(
            () => document.documentElement.scrollWidth > innerWidth,
          ),
          false,
          `${name}/${locale}/${width} overflow`,
        );
        if (name === "home") {
          assert.equal(await page.locator(".ps-shirt").count(), 2);
          assert.equal(
            await page
              .locator(".ps-shirt")
              .evaluateAll((images) =>
                images.every(
                  (image) => image.complete && image.naturalWidth === 192,
                ),
              ),
            true,
            "both synthetic example thumbnails load from the public host",
          );
          assert.ok(
            (await page.locator("footer").boundingBox()).y >=
              (width === 390 ? 844 : 992),
            "operator must start below first viewport",
          );
          assert.match(
            await page.locator(".ps-disclaimer").innerText(),
            locale === "en"
              ? /not.*every merchant|does not mean every merchant/
              : /不代表每家商/,
          );
        }
        await page.screenshot({
          path: `${output}/${name}-${locale}-${width}.png`,
          fullPage: true,
        });
        if (name === "home" && locale === "zh-TW" && width === 1586)
          await page.screenshot({ path: ".impeccable/review/hero-repro.png" });
        ledger.push({
          locale,
          width,
          page: name,
          action: "render",
          result: "PASS",
        });
        await page.reload({ waitUntil: "networkidle" });
        assert.match(await page.title(), /^DaWan Live · /);
        assert.ok(
          (await page.locator("footer").innerText()).includes(facts[0]),
        );
        ledger.push({
          locale,
          width,
          page: name,
          action: "reload",
          result: "PASS",
        });
        await page.locator(".ps-skip").focus();
        await page.locator(".ps-skip").click();
        assert.equal(
          await page.evaluate(() => document.activeElement?.id),
          "main",
        );
        ledger.push({
          locale,
          width,
          page: name,
          action: "skip to main",
          result: "PASS",
        });
        const emails = page.locator('a[href="mailto:contact@example.invalid"]');
        for (let i = 0; i < (await emails.count()); i++) {
          await emails.nth(i).click();
          ledger.push({
            locale,
            width,
            page: name,
            action: `mailto ${i + 1} (handler only; no email sent)`,
            result: "PASS",
          });
        }
        for (const selector of name === "home"
          ? [".ps-brand", ".ps-login"]
          : [".ps-brand", ".ps-back", ".ps-login"]) {
          await page.goto(origin + pathname);
          await page.locator(selector).click();
          await page.waitForURL(
            selector === ".ps-login"
              ? `https://${adminHost}/${locale}`
              : origin + routePath(locale),
          );
          assert.match(await page.locator("body").innerText(), /DaWan Live/);
          ledger.push({
            locale,
            width,
            page: name,
            action: selector,
            result: "PASS",
          });
        }
        for (const dest of pages.filter((p) => p !== "home")) {
          await page.goto(origin + pathname);
          await page
            .locator(`footer nav a[href="${routePath(locale, dest)}"]`)
            .click();
          await page.waitForURL(origin + routePath(locale, dest));
          assert.equal(await page.locator("main h1").count(), 1);
          if (dest === "privacy" || dest === "terms")
            assertPlatformMessagingCopy(
              (await page.locator(".ps-legal-section").allTextContents()).join(
                " ",
              ),
              locale,
              dest,
            );
          ledger.push({
            locale,
            width,
            page: name,
            action: `footer → ${dest}`,
            result: "PASS",
          });
        }
        // Locale switch retains the current document, rather than silently returning home.
        for (const target of locales) {
          await page.goto(origin + pathname);
          await page.locator(`header nav a[lang="${target}"]`).click();
          await page.waitForURL(origin + routePath(target, name));
          assert.equal(await page.locator("html").getAttribute("lang"), target);
          ledger.push({
            locale,
            width,
            page: name,
            action: `locale → ${target}`,
            result: "PASS",
          });
        }
      }
      await page.goto(origin + routePath(locale));
      for (const selector of [".ps-login", ".ps-primary"]) {
        await page.locator(selector).click();
        await page.waitForURL(
          `https://${adminHost}/${locale}${selector === ".ps-primary" ? "/signup" : ""}`,
        );
        assert.match(await page.locator("body").innerText(), /DaWan Live/);
        assert.equal(
          await page.locator('meta[name="robots"][content*="noindex"]').count(),
          1,
        );
        ledger.push({
          locale,
          width,
          page: "home",
          action:
            selector === ".ps-primary"
              ? "register on admin host"
              : "sign in on admin host",
          result: "PASS",
        });
        await page.goBack();
        await page.waitForURL(origin + routePath(locale));
      }
      const sizes = await page
        .locator("header a, .ps-primary, footer nav a, footer dd a")
        .evaluateAll((els) =>
          els.map((el) => ({
            text: el.textContent,
            h: el.getBoundingClientRect().height,
            w: el.getBoundingClientRect().width,
          })),
        );
      for (const el of sizes)
        assert.ok(el.h >= 44 && el.w >= 44, `44px ${el.text}: ${el.w}×${el.h}`);
    }
  assert.deepEqual(errors, []);
  await context.close();
}
try {
  browser = await chromium.launch({ headless: true });
  const s = await server("");
  try {
    // Node/SSR assertions independently of hydration: identity, crawlable HTML, no cookies.
    for (const l of locales)
      for (const p of pages) {
        const r = await get(s.p, host, routePath(l, p)),
          html = r.body.toString();
        assert.equal(r.status, 200);
        for (const f of facts)
          assert.ok(html.includes(f), `${l}/${p} SSR fact ${f}`);
        assert.ok(html.includes("880ef0da"));
        assert.equal(r.headers["set-cookie"], undefined);
        assert.ok(!html.includes('name="facebook-domain-verification"'));
      }
    for (const route of ["/en/orders", "/site/en/home", "/api/stores/fake"])
      assert.equal((await get(s.p, host, route)).status, 404);
    // Review P2: actual Host normalization must apply to every public entry,
    // not only the rewrite (the layout, robots and sitemap repeat the guard).
    for (const candidate of [
      host.toUpperCase(),
      `${host}.`,
      `${host}:8443`,
      `${host.toUpperCase()}.:443`,
    ]) {
      for (const route of [
        "/zh-CN",
        "/privacy",
        "/robots.txt",
        "/sitemap.xml",
      ]) {
        const response = await get(s.p, candidate, route);
        assert.equal(response.status, 200, `${candidate}${route}`);
        assert.equal(
          response.headers["set-cookie"],
          undefined,
          "never enter the admin locale flow",
        );
        assert.equal(response.headers.location, undefined);
        if (route === "/robots.txt")
          assert.match(response.body.toString(), /Allow: \/\n/);
        else if (route === "/sitemap.xml")
          assert.match(response.body.toString(), /<urlset/);
        else assert.ok(response.body.toString().includes('class="ps-body"'));
      }
    }
    for (const candidate of [
      `www.${host}`,
      `WWW.${host.toUpperCase()}.`,
      `www.${host}:443`,
    ])
      assert.equal(
        (await get(s.p, candidate, "/zh-CN")).status,
        404,
        "www is edge-only; no admin fallback",
      );
    const adminRobots = await get(s.p, adminHost, "/robots.txt");
    assert.equal(adminRobots.status, 404, "preserve admin robots behavior");
    assert.equal(adminRobots.headers.location, undefined);
    assert.equal(adminRobots.headers["set-cookie"], undefined);
    assert.equal(
      (await get(s.p, adminHost, "/site/en/home", { "x-forwarded-host": host }))
        .status,
      404,
    );
    assert.match(
      (await get(s.p, host, "/robots.txt")).body.toString(),
      /Allow: \/\n/,
    );
    assert.equal(
      (
        (await get(s.p, host, "/sitemap.xml")).body
          .toString()
          .match(/<url>/g) || []
      ).length,
      15,
    );
    await matrix(s);
  } finally {
    await stop(s);
  }
  const tagged = await server("fixture-domain-proof");
  try {
    for (const l of locales)
      for (const p of pages) {
        const html = (
          await get(tagged.p, host, routePath(l, p))
        ).body.toString();
        // Count actual tags, not Next's serialised metadata in hydration script text.
        assert.equal(
          (html.match(/<meta name="facebook-domain-verification"[^>]*>/g) || [])
            .length,
          p === "home" ? 1 : 0,
        );
        if (p === "home") assert.match(html, /content="fixture-domain-proof"/);
      }
    const crawler = (
      await get(tagged.p, host, "/", {
        "User-Agent": "facebookexternalhit/1.1",
      })
    ).body.toString();
    assert.ok(
      crawler.indexOf('name="facebook-domain-verification"') <
        crawler.indexOf("</head>"),
      "Meta crawler receives verification in head",
    );
    assert.ok(
      !(await get(tagged.p, adminHost, "/en")).body
        .toString()
        .includes('name="facebook-domain-verification"'),
    );
  } finally {
    await stop(tagged);
  }
  await writeFile(
    `${output}/click-ledger.json`,
    JSON.stringify(
      {
        tier: "MOCK transport / production Next / real Chromium",
        rows: ledger,
      },
      null,
      2,
    ),
  );
  // Read assertions are not user clicks; keep this new copy coverage separate.
  const clicks = ledger.filter(
    (r) => !["render", "reload", "messaging-copy"].includes(r.action),
  ).length;
  const messagingCopyCases = ledger.filter(
    (r) => r.action === "messaging-copy",
  ).length;
  await writeFile(
    `${output}/ps-browser-result.json`,
    JSON.stringify(
      {
        pass: 30,
        fail: 0,
        clicks,
        messagingCopyCases,
        reloads: ledger.filter((r) => r.action === "reload").length,
        ssrCases: 15,
        tagCases: 16,
      },
      null,
      2,
    ),
  );
  console.log(
    `PASS PS1/PS2/PS4: 30 page cases, 15 SSR cases, 16 tag cases, ${clicks} real clicks, ${messagingCopyCases} messaging-copy cases and 30 reloads`,
  );
} finally {
  await browser?.close();
  await auth.close();
}
