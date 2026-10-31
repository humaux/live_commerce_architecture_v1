import { defineConfig, devices } from "@playwright/test";
// LC_BROWSER_ENGINE=webkit: Desktop Safari profile for the admin pages (forwarded by browserEnvironment); chromium otherwise.
const safari = process.env.LC_BROWSER_ENGINE === "webkit" ? { ...devices["Desktop Safari"], ignoreHTTPSErrors: true } : {};
export default defineConfig({
  testDir: "/home/runner/work/live_commerce_architecture_v1/live_commerce_architecture_v1/tests/admin",
  testMatch: ["home-cod.spec.ts"],
  fullyParallel: false, workers: 1, retries: 0, timeout: 180000,
  expect: { timeout: 10000 },
  reporter: [["list"]],
  outputDir: "/home/runner/work/live_commerce_architecture_v1/live_commerce_architecture_v1/output/playwright/run.kfwVz1JJ/home-cod/20261030T121108.605035780/results",
  use: { ...safari, baseURL: "http://127.0.0.1:33121", headless: true, viewport: { width: 1586, height: 992 }, trace: "retain-on-failure", screenshot: "only-on-failure" },
});
