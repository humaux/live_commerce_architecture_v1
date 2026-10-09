// Purpose: prove the actual MOU parcel runner collects its real browser scenario before expensive PG setup.
// Depends on: installed Playwright CLI, actual Go runner arguments, playwright.config.ts and parcel-merge.spec.ts.
// Used by: test-node.sh; collection only, no browser/server/PG and no substituted test implementation.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
const root = fileURLToPath(new URL("../../", import.meta.url));
test("actual MOU parcel runner selects and collects the real parcel browser case", () => {
  const harness = readFileSync(new URL("../foundation/browser_merchant_orders_ui_test.go", import.meta.url), "utf8");
  const selector = harness.match(/parcelBrowser := exec.CommandContext\(ctx, "pnpm", "exec", "playwright", "test", "([^"]+)"/);
  const suite = harness.match(/parcelBrowser.Env = browserEnvironment\(map\[string\]string\{[\s\S]*?"LC_BROWSER_SUITE": "([^"]+)"/);
  assert.ok(selector && suite, "exercise the actual Go-owned selector and suite name");
  const uuid = (n: number) => `10000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
  const env = { ...process.env,
    LC_BROWSER_SUITE: suite[1], LC_BROWSER_ENGINE: "chromium",
    LC_BROWSER_PUBLIC_ORIGIN: "http://127.0.0.1:3100", LC_BROWSER_ORDER_STORE: uuid(1),
    LC_BROWSER_PARCEL_ORDERS: JSON.stringify({ ship: [uuid(2), uuid(3)], dissolve: [uuid(4), uuid(5)],
      stale: [uuid(6), uuid(7)], excluded: [uuid(8), uuid(9)] }),
  };
  for (const key of ["PLAYWRIGHT_JSON_OUTPUT_FILE", "PLAYWRIGHT_JSON_OUTPUT_NAME", "PLAYWRIGHT_JSON_OUTPUT_DIR"])
    delete (env as NodeJS.ProcessEnv)[key];
  const result = spawnSync(process.execPath,
    [createRequire(import.meta.url).resolve("@playwright/test/cli"), "test", selector[1], "--list", "--reporter=json", "--config", "playwright.config.ts"],
    { cwd: root, env, encoding: "utf8", timeout: 30_000, maxBuffer: 2 * 1024 * 1024 });
  assert.equal(result.error, undefined);
  assert.equal(result.signal, null);
  assert.equal(result.status, 0, `actual parcel collection failed: ${result.stderr}`);
  const report = JSON.parse(result.stdout);
  assert.deepEqual(report.errors, []);
  const cases = (suites: any[]): any[] => suites.flatMap((entry) => [...(entry.specs ?? []), ...cases(entry.suites ?? [])]);
  const found = cases(report.suites);
  assert.equal(found.length, 1, "parcel scenario must neither be omitted nor duplicated");
  assert.equal(found[0].file, "parcel-merge.spec.ts");
  assert.match(found[0].title, /^W3-07B parcel merge:/);
});
