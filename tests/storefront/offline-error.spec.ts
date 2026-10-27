// Purpose: OC-11 / OC-Q18 buyer gate: the storefront error page offers one read-only reload and an offline banner; it never re-sends a write.
// Depends on: @playwright/test, apps/storefront/lib/shop-copy; env LC_OC11_ORIGIN (storefront origin) and LC_OC11_ERROR_PATH
//   (a path that renders app/[locale]/error.tsx, e.g. a page read while the fake buyer API is `down`, tests/storefront/shop-fake-api.mjs).
// Used by: NOT_RUN until the integrator registers a browser mode that serves such a fixture (see PR body, integrator request).
// Real click on the reload action; context.setOffline drives navigator.onLine and the offline/online events (measurement, not a DOM edit).
import { expect, test } from "@playwright/test";
import { shopCopy } from "../../apps/storefront/lib/shop-copy";

const origin = process.env.LC_OC11_ORIGIN ?? "";
const errorPath = process.env.LC_OC11_ERROR_PATH ?? "";
test.skip(!origin || !errorPath, "LC_OC11_ORIGIN / LC_OC11_ERROR_PATH not provided: NOT_RUN");

for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
  test(`OC11 error page offline banner + read-only reload ${locale} at 390px`, async ({ browser }) => {
    const copy = shopCopy[locale];
    const context = await browser.newContext({ baseURL: origin, viewport: { width: 390, height: 844 }, ignoreHTTPSErrors: true });
    const page = await context.newPage();
    const writes: string[] = [];
    page.on("request", (r) => r.method() !== "GET" && r.method() !== "HEAD" && writes.push(`${r.method()} ${r.url()}`));
    await page.goto(`/${locale}${errorPath}`);
    const error = page.getByTestId("page-error");
    await expect(error).toBeVisible();
    await expect(page.getByTestId("offline-banner")).toHaveCount(0);

    await context.setOffline(true);
    const banner = page.getByTestId("offline-banner");
    await expect(banner).toHaveText(copy.offlineBanner);
    await expect(banner).toHaveAttribute("role", "status");
    await expect(error).toHaveAttribute("role", "alert");

    const reload = error.getByRole("button");
    await expect(reload).toHaveCount(1); // the single action is the read-only reload
    await expect(reload).toHaveText(copy.reloadPage);
    expect((await reload.boundingBox())?.height ?? 0).toBeGreaterThanOrEqual(44);
    expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);

    await reload.click();
    await context.setOffline(false);
    await expect(banner).toHaveCount(0);
    expect(writes, "reload re-reads with GET only: no payment/order POST is replayed").toEqual([]);
    await context.close();
  });
}
