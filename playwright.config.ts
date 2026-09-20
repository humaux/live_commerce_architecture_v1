import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/admin",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30000,
  expect: { timeout: 10000 },
  reporter: [
    ["list"],
    ["json", { outputFile: "output/playwright/results.json" }],
  ],
  outputDir: "output/playwright/test-results",
  use: {
    baseURL: "http://127.0.0.1:3100",
    headless: true,
    viewport: { width: 1586, height: 992 },
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
});
