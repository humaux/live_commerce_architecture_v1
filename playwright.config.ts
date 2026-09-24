import { defineConfig } from "@playwright/test";

// These suites have deliberately different server/authority fixtures. Never
// silently run signed-identity tests against a ledger's shared dev bearer.
const suite = process.env.LC_BROWSER_SUITE ?? "ledger";
const suites: Record<string, string[]> = {
  ledger: ["ledger.spec.ts", "production.spec.ts", "visual-states.spec.ts"],
  "identity-mock": ["auth.spec.ts"],
  "entry-mock": ["entry.spec.ts"],
  "identity-real": ["auth-real.spec.ts"],
  "settings-real": ["settings-real.spec.ts"],
};
if (!Object.hasOwn(suites, suite)) throw new Error("Invalid LC_BROWSER_SUITE");

export default defineConfig({
  testDir: "./tests/admin",
  testMatch: suites[suite],
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
