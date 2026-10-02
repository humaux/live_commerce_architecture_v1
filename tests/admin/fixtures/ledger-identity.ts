// Legacy ledger has real Go/PG catalog routes but no identity BFF config. Adapt
// only the shell's identity transport; grants come from the real SQL projection
// executed as commerce_runtime by the Go wrapper. This is MOCK identity, not IdP.
import { test as base, expect, type BrowserContext } from "@playwright/test";

async function ledgerIdentity(context: BrowserContext) {
  const body = process.env.LC_LEDGER_IDENTITY;
  if (!body) throw new Error("LC_LEDGER_IDENTITY is required from the Go fixture");
  await context.route("**/api/stores", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body });
  });
}

export const test = base.extend({
  context: async ({ context }, use) => {
    await ledgerIdentity(context);
    await use(context);
  },
});
export { expect };
