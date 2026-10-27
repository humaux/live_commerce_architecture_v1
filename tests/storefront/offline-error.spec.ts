// Purpose: OC-11 / OC-Q18 buyer check: the storefront error page offers one read-only reload and an offline banner; it never re-sends a write.
// Depends on: apps/storefront/lib/shop-copy.ts, @playwright/test `expect`; called with the shop-gate browser, fake API and virtual-host origin.
// Used by: tests/storefront/shop-gate.mjs (scripts/dev/test-local.sh storefront-shell mode). Not a Playwright-runner file: a plain module.
// Real click on the reload action; context.setOffline drives navigator.onLine and the offline/online events (measurement, not a DOM edit).
import assert from "node:assert/strict";
import { expect } from "@playwright/test";
import { shopCopy } from "../../apps/storefront/lib/shop-copy.ts";
import { ctxOpts } from "./browser-engine.mjs";

export async function checkOfflineError({ browser, api, origin }) {
  for (const locale of ["zh-TW", "zh-CN", "en"]) {
    const copy = shopCopy[locale];
    const context = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width: 390, height: 844 }, locale }));
    const page = await context.newPage();
    const writes = [];
    page.on("request", (r) => { if (r.method() !== "GET" && r.method() !== "HEAD") writes.push(`${r.method()} ${r.url()}`); });
    api.state.down = true; // the pages' own error state (shop-fake-api `down`)
    try {
      await page.goto(`${origin}/${locale}`);
      const error = page.getByTestId("page-error");
      await expect(error, `OC11-B0 ${locale} error state renders`).toBeVisible();
      assert.equal(await page.getByTestId("offline-banner").count(), 0, `OC11-B1 ${locale} no banner while online`);

      await context.setOffline(true);
      const banner = page.getByTestId("offline-banner");
      await expect(banner, `OC11-B2 ${locale} offline banner text`).toHaveText(copy.offlineBanner);
      assert.equal(await banner.getAttribute("role"), "status");
      assert.equal(await error.getAttribute("role"), "alert");

      const reload = error.getByRole("button");
      assert.equal(await reload.count(), 1, `OC11-B3 ${locale} the single action is the read-only reload`);
      assert.equal(await reload.innerText(), copy.reloadPage);
      assert.ok(((await reload.boundingBox())?.height ?? 0) >= 44, "44px target");
      // G-UI8 audit [READ/MEASURE]: horizontal overflow at 390px
      assert.ok((await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)) <= 1, "no horizontal scroll at 390px");

      await reload.click();
      await context.setOffline(false);
      await expect(banner, `OC11-B4 ${locale} banner leaves when back online`).toHaveCount(0);
      assert.deepEqual(writes, [], `OC11-B5 ${locale} reload re-reads with GET only: no payment/order POST replayed`);
    } finally {
      api.state.down = false;
      await context.close();
    }
  }
  console.log("OC11 error page: offline banner + single read-only reload, zh-TW/zh-CN/en at 390px, no write replayed");
}
