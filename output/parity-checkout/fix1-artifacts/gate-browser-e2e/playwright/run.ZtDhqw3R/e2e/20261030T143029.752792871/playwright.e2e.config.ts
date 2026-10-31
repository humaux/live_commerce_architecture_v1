import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "/home/runner/work/live_commerce_architecture_v1/live_commerce_architecture_v1/tests/e2e", testMatch: ["deal-loop.spec.ts"],
  fullyParallel: false, workers: 1, retries: 0, timeout: 1500000,
  expect: { timeout: 15000 },
  reporter: [["list"]], outputDir: "/home/runner/work/live_commerce_architecture_v1/live_commerce_architecture_v1/output/playwright/run.ZtDhqw3R/e2e/20261030T143029.752792871/results",
  use: { headless: true, viewport: { width: 1586, height: 992 }, trace: "off", screenshot: "only-on-failure", actionTimeout: 20000, navigationTimeout: 30000 },
});
