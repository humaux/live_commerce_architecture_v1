// Purpose: DS-1 browser gate for the design-system gallery: 404 when LC_UI_GALLERY is not "1"; then at 1440 and 390 in zh-TW, zh-CN and en a full-page screenshot, axe (serious/critical = 0) and the visual-lint blocking rules; plus a real-click ledger over every control of the gallery.
// Depends on: Playwright Chromium, axe-core, tests/ui/visual-lint-lib.mjs, tests/admin/shell-fixture.mjs (MOCK API the admin server needs to boot), the packaged admin build (scripts/dev/start-admin.mjs), apps/admin/src/ui-gallery-copy.ts.
// Used by: scripts/dev/test-local.sh --browser-ui-gallery. MOCK tier: sample data, no PG, no provider traffic.
import { chromium } from "@playwright/test";
import { spawn } from "node:child_process";
import { createRequire } from "node:module";
import { createServer } from "node:net";
import { mkdir, readFile, writeFile, open } from "node:fs/promises";
import { createHash } from "node:crypto";
import assert from "node:assert/strict";
import { browserEvidenceDirectory } from "../browser-evidence.mjs";
import { fixture } from "./shell-fixture.mjs";
import { collect, describe, evaluate } from "../ui/visual-lint-lib.mjs";
import { galleryCopy } from "../../apps/admin/src/ui-gallery-copy.ts";

const require = createRequire(import.meta.url);
const output = process.env.LC_UI_GALLERY_EVIDENCE || browserEvidenceDirectory("ui-gallery");
await mkdir(`${output}/shots`, { recursive: true });
const LOCALES = ["zh-TW", "zh-CN", "en"];
const VIEWPORTS = [[1440, 900], [390, 844]];
const f = await fixture();

async function freePort() {
  const probe = createServer();
  await new Promise((r) => probe.listen(0, "127.0.0.1", r));
  const port = probe.address().port;
  await new Promise((r) => probe.close(r));
  return port;
}

/** Starts the packaged admin; `gallery` sets or omits LC_UI_GALLERY so both halves of the flag are exercised. */
async function startAdmin(label, gallery) {
  const port = await freePort();
  const base = `http://127.0.0.1:${port}`;
  const log = await open(`${output}/server-${label}.log`, "w");
  const env = {
    ...process.env,
    COMMERCE_UI_PORT: String(port),
    COMMERCE_IDENTITY_ENABLED: "1",
    COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS: "1",
    COMMERCE_PASSWORD_LOGIN_ENABLED: "1",
    COMMERCE_PUBLIC_ORIGIN: base,
    COMMERCE_API_ORIGIN: f.origin,
    COMMERCE_BFF_KEY: f.key,
  };
  delete env.LC_UI_GALLERY;
  if (gallery !== undefined) env.LC_UI_GALLERY = gallery;
  const child = spawn(process.execPath, ["scripts/dev/start-admin.mjs"], { detached: true, stdio: ["ignore", log.fd, log.fd], env });
  for (let i = 0; ; i++) {
    try {
      if ((await fetch(`${base}/en/reset`)).ok) break;
    } catch {}
    if (i === 119) throw new Error(`admin (${label}) readiness timeout`);
    await new Promise((r) => setTimeout(r, 500));
  }
  return {
    base,
    stop: async () => {
      try {
        process.kill(-child.pid, "SIGTERM");
      } catch {}
      await log.close();
    },
  };
}

const ledger = [];
const failures = [];
/** One row of the click ledger: page, control, action, expected, actual, result. A failed step is recorded and the run continues so one report lists every defect. */
async function step(page, control, action, expected, run) {
  const row = { page: page.url().replace(/^https?:\/\/[^/]+/, ""), control, action, expected, actual: "", result: "pass" };
  try {
    row.actual = (await run()) ?? expected;
  } catch (error) {
    row.result = "fail";
    row.actual = String(error?.message ?? error).split("\n")[0].slice(0, 300);
    failures.push(`${row.page} | ${control} | ${action}: ${row.actual}`);
  }
  ledger.push(row);
}

async function audit(page, locale, width, height) {
  const name = `${locale}-${width}x${height}`;
  await page.evaluate(async () => {
    await document.fonts.ready; // [READ] wait for fonts only
  });
  await page.screenshot({ path: `${output}/shots/${name}.png`, fullPage: true, animations: "disabled" });
  const png = await readFile(`${output}/shots/${name}.png`);
  await page.addScriptTag({ content: await readFile(require.resolve("axe-core/axe.min.js"), "utf8") });
  const axe = await page.evaluate(async () => {
    // [READ/MEASURE] axe only reads the DOM
    const result = await window.axe.run(document, { resultTypes: ["violations"] });
    return result.violations.filter((v) => v.impact === "serious" || v.impact === "critical").map((v) => ({ id: v.id, impact: v.impact, nodes: v.nodes.slice(0, 4).map((n) => n.target.join(" ")) }));
  });
  const snapshot = await page.evaluate(collect); // [READ/MEASURE]
  const violations = evaluate(snapshot, { mobile: width < 768 });
  const blocking = violations.filter((v) => v.severity === "block");
  const named = blocking.length ? await page.evaluate(describe, [...new Set(blocking.flatMap((v) => v.ids))]) : {};
  const lint = blocking.map((v) => ({ rule: v.rule, kind: v.kind, what: v.ids.map((id) => named[id] ?? id).slice(0, 3) }));
  return { name, sha256: createHash("sha256").update(png).digest("hex"), axe, lint, warnings: violations.length - blocking.length };
}

const evidence = [];
let flagOff = [];
const off = await startAdmin("off", undefined);
try {
  const browser = await chromium.launch({ headless: true });
  const page = await (await browser.newContext()).newPage();
  for (const locale of LOCALES) {
    const response = await page.goto(`${off.base}/${locale}/ui-gallery`, { waitUntil: "load" });
    flagOff.push({ locale, status: response?.status() });
    assert.equal(response?.status(), 404, `/${locale}/ui-gallery must be 404 when LC_UI_GALLERY is not 1`);
  }
  await browser.close();
} finally {
  await off.stop();
}

const on = await startAdmin("on", "1");
const browser = await chromium.launch({ headless: true });
try {
  for (const locale of LOCALES)
    for (const [width, height] of VIEWPORTS) {
      const context = await browser.newContext({ viewport: { width, height }, deviceScaleFactor: 1, locale: locale === "en" ? "en-US" : locale });
      const page = await context.newPage();
      page.setDefaultTimeout(8000);
      const errors = [];
      page.on("pageerror", (e) => errors.push(`pageerror ${e.message}`));
      page.on("console", (m) => m.type() === "error" && errors.push(`console ${m.text()}`));
      const response = await page.goto(`${on.base}/${locale}/ui-gallery`, { waitUntil: "load" });
      assert.equal(response?.status(), 200, `${locale} ${width}: gallery must render when LC_UI_GALLERY=1`);
      await page.getByRole("heading", { level: 1, name: galleryCopy[locale].title }).waitFor();
      const result = await audit(page, locale, width, height);
      assert.deepEqual(errors, [], `${locale} ${width}: console/page errors`);
      evidence.push({ locale, width, height, ...result });
      await context.close();
    }
  const broken = evidence.filter((e) => e.axe.length || e.lint.length);
  await writeFile(`${output}/matrix.json`, `${JSON.stringify({ tier: "MOCK", flagOff, evidence }, null, 2)}\n`);
  if (broken.length) {
    for (const e of broken) failures.push(`matrix ${e.name}: axe=${JSON.stringify(e.axe)} lint=${JSON.stringify(e.lint)}`);
  }
  const { clickJourney } = await import("./ui-gallery-clicks.mjs");
  for (const [locale, width, height] of [["zh-TW", 1440, 900], ["en", 390, 844]]) {
    const context = await browser.newContext({ viewport: { width, height }, deviceScaleFactor: 1 });
    const page = await context.newPage();
    page.setDefaultTimeout(8000);
    await page.goto(`${on.base}/${locale}/ui-gallery`, { waitUntil: "load" });
    await clickJourney({ page, step: (...args) => step(page, ...args), copy: galleryCopy[locale], locale, base: on.base, mobile: width < 768 });
    await context.close();
  }
} finally {
  await browser.close();
  await on.stop();
  await f.close();
  const failed = ledger.filter((r) => r.result === "fail").length;
  await writeFile(`${output}/click-ledger.json`, `${JSON.stringify(ledger, null, 2)}\n`);
  const md = ["| page | control | action | expected | actual | result |", "| --- | --- | --- | --- | --- | --- |", ...ledger.map((r) => `| ${r.page} | ${r.control} | ${r.action} | ${r.expected} | ${r.actual.replaceAll("|", "/")} | ${r.result} |`)];
  await writeFile(`${output}/click-ledger.md`, `${md.join("\n")}\n`);
  console.log(`ledger rows ${ledger.length}, failed ${failed}; evidence ${output}`);
}
if (failures.length) {
  console.error(`FAIL DS-1 ui-gallery: ${failures.length} finding(s)\n${failures.join("\n")}`);
  process.exit(1);
}
console.log(`PASS DS-1 ui-gallery: ${evidence.length} matrix cases (3 locales x 2 viewports: screenshot + axe + visual-lint blocking = 0), flag-off 404 in ${flagOff.length} locales, ${ledger.length} real-click ledger rows`);
