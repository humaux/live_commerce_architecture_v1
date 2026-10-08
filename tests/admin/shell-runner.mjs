// Purpose: Start the isolated Next shell fixture and capture baseline or real-click navigation evidence.
// Depends on: Playwright Chromium, Node process/fs/net helpers, shell-fixture and shell-browser.
// Used by: --browser-admin-shell and W0 baseline capture; signed Go/PG acceptance remains separate.
// UI-only MOCK gate; existing signed Go/PG modes remain separate acceptance.
import { chromium } from "@playwright/test";
import { spawn } from "node:child_process";
import { createServer } from "node:net";
import { mkdir, readFile, writeFile, open } from "node:fs/promises";
import { createHash } from "node:crypto";
import assert from "node:assert/strict";
import { fixture, storeID, entityID } from "./shell-fixture.mjs";
const baseline = process.argv.includes("--baseline");
const output = baseline ? "tests/admin/baselines/w0" : "output/ui-w0-shell";
await mkdir(output, { recursive: true });
const probe = createServer();
await new Promise((r) => probe.listen(0, "127.0.0.1", r));
const port = probe.address().port;
await new Promise((r) => probe.close(r));
const base = `http://127.0.0.1:${port}`,
  f = await fixture();
const log = await open(`${output}/server.log`, "w");
const child = spawn(
  baseline ? "pnpm" : process.execPath,
  baseline
    ? [
        "--dir",
        "apps/admin",
        "exec",
        "next",
        "dev",
        "--hostname",
        "127.0.0.1",
        "--port",
        String(port),
      ]
    : ["scripts/dev/start-admin.mjs"],
  {
    detached: true,
    stdio: ["ignore", log.fd, log.fd],
    env: {
      ...process.env,
      COMMERCE_UI_PORT: String(port),
      COMMERCE_IDENTITY_ENABLED: "1",
      COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS: "1",
      COMMERCE_PASSWORD_LOGIN_ENABLED: "1",
      COMMERCE_PUBLIC_ORIGIN: base,
      COMMERCE_API_ORIGIN: f.origin,
      COMMERCE_BFF_KEY: f.key,
    },
  },
);
let browser;
try {
  for (let i = 0; i < 120; i++) {
    try {
      if ((await fetch(`${base}/en/reset`)).ok) break;
    } catch {}
    if (i === 119) throw new Error("Next readiness timeout");
    await new Promise((r) => setTimeout(r, 500));
  }
  browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  await context.addCookies([
    {
      name: "__Host-commerce_session",
      value: f.token,
      domain: "127.0.0.1",
      path: "/",
      secure: true,
      httpOnly: true,
      sameSite: "Lax",
    },
  ]);
  await context.addCookies([
    {
      name: "__Host-commerce_csrf",
      value: f.token,
      domain: "127.0.0.1",
      path: "/",
      secure: true,
      httpOnly: false,
      sameSite: "Lax",
    },
  ]);
  const page = await context.newPage();
  const routes = [
    "/",
    "/ads",
    "/billing",
    "/collections",
    "/customers",
    `/customers/${entityID}`,
    "/design",
    "/finance",
    "/inventory",
    `/invite/${"A".repeat(43)}`,
    "/orders",
    "/orders/cvs-print",
    "/orders/new",
    "/products",
    `/products/${entityID}`,
    "/products/import",
    "/promotions",
    "/reset",
    "/settings",
    "/signup",
    "/studio",
    "/studio/console",
    "/studio/claims",
    "/team",
  ];
  if (baseline) {
    const publicOnly = process.argv.includes("--public-only");
    const publicRoutes = ["/reset", "/signup"];
    const manifest = publicOnly
      ? JSON.parse(
          await readFile(`${output}/manifest.json`, "utf8"),
        ).manifest.filter((r) => !publicRoutes.includes(r.route))
      : [];
    for (const route of routes)
      for (const [width, height] of [
        [1586, 992],
        [390, 844],
      ]) {
        if (publicOnly && !publicRoutes.includes(route)) continue;
        if (publicRoutes.includes(route)) await context.clearCookies();
        else
          await context.addCookies(
            ["session", "csrf"].map((kind) => ({
              name: `__Host-commerce_${kind}`,
              value: f.token,
              domain: "127.0.0.1",
              path: "/",
              secure: true,
              httpOnly: kind === "session",
              sameSite: "Lax",
            })),
          );
        await page.setViewportSize({ width, height });
        const query =
          route === "/studio/claims"
            ? `?store=${storeID}&scene=${entityID}`
            : "";
        const response = await page.goto(`${base}/zh-CN${route}${query}`, {
          waitUntil: "load",
        });
        assert.equal(response.status(), 200, route);
        await page.waitForTimeout(700);
        const name = `${route.replaceAll("/", "-").replaceAll(entityID, "detail").replaceAll("A".repeat(43), "token") || "home"}-${width}.png`;
        await page.screenshot({
          path: `${output}/${name}`,
          animations: "disabled",
        });
        const bytes = await readFile(`${output}/${name}`);
        manifest.push({
          route,
          width,
          height,
          file: name,
          sha256: createHash("sha256").update(bytes).digest("hex"),
          state: (
            await page
              .locator("main")
              .innerText()
              .catch(() => page.locator("body").innerText())
          ).slice(0, 600),
        });
      }
    await writeFile(
      `${output}/manifest.json`,
      JSON.stringify(
        {
          tier: "MOCK",
          base: "b4223c8",
          note: "Synthetic store; empty lists and explicitly unavailable detail endpoints. Not business-flow acceptance.",
          manifest,
        },
        null,
        2,
      ) + "\n",
    );
    console.log(
      `PASS baseline: ${manifest.length} captures, 23 routes, two viewports`,
    );
  } else {
    const { runShellGate } = await import("./shell-browser.mjs");
    try {
      await runShellGate({ page, context, f, base, output, storeID });
      const { runBrandGate } = await import("./platform-brand-browser.mjs");
      await runBrandGate({ browser, base, output: "output/platform-site/brand-browser" });
    } catch (error) {
      const red = `${output}/red/run-${Date.now()}`;
      await mkdir(red, { recursive: true });
      await page.screenshot({ path: `${red}/failure.png` });
      await writeFile(
        `${red}/failure.txt`,
        await page.locator("body").innerText(),
      );
      throw error;
    }
  }
} finally {
  await browser?.close();
  try {
    process.kill(-child.pid, "SIGTERM");
  } catch {}
  await f.close();
  await log.close();
}
