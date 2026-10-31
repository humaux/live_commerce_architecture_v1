import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "/home/runner/work/live_commerce_architecture_v1/live_commerce_architecture_v1/tests/e2e", testMatch: ["live-tools.spec.ts"],
  fullyParallel: false, workers: 1, retries: 0, timeout: 900000,
  expect: { timeout: 15000 },
  reporter: [["list"]], outputDir: "/home/runner/work/live_commerce_architecture_v1/live_commerce_architecture_v1/output/playwright/run.ZtDhqw3R/live-tools/20261030T143108.003818361/results",
  use: { headless: true, trace: "off", screenshot: "only-on-failure", actionTimeout: 20000, navigationTimeout: 30000 },
});
