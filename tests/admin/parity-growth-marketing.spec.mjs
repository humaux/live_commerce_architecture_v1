// Purpose: Verify the signed feature-off marketing fallback on the real promotions route.
// Depends on: Playwright and the existing promotions browser fixture's authenticated merchant page.
// Used by: tests/storefront/promotions-gate.mjs (TestBrowserPromotions in GitHub CI).
// Invariants: no invented customer segments/counts, no audience upload or ad publication.
import { expect } from "@playwright/test";
import { mkdir, appendFile } from "node:fs/promises";
import path from "node:path";

/** No segment DTO exists in promotions: the card requires the signed feature-off fallback. */
export async function growthMarketingParity(page, { origin, evidence }) {
  const directory = path.join(evidence, "parity");
  await mkdir(directory, { recursive: true });
  for (const [locale, heading, disabled, legacy] of [
    ["zh-TW", "廣告草稿", "平台啟用前，此商店的廣告功能為關閉狀態。", "優惠碼"],
    ["zh-CN", "广告草稿", "平台启用前，此店铺的广告功能为关闭状态。", "优惠码"],
    ["en", "Ad drafts", "Ads are off for this store until the platform enables them.", "Discount codes"],
  ]) {
    for (const viewport of [{ width: 1440, height: 900, frame: "1194:7740" }, { width: 390, height: 844, frame: "1194:7796" }]) {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await page.goto(`${origin}/${locale}/promotions`);
      await expect(page.getByRole("heading", { level: 1, name: heading, exact: true })).toBeVisible();
      await expect(page.getByText(disabled, { exact: true })).toBeVisible();
      await expect(page.getByRole("heading", { level: 1, name: legacy, exact: true })).toHaveCount(0);
      await expect(page.getByText("42", { exact: true })).toHaveCount(0);
      await expect(page.getByRole("button", { name: /新建分群|新增分群|New segment/ })).toHaveCount(0);
      const retry = { "zh-TW": "重新讀取", "zh-CN": "重新读取", en: "Refresh" }[locale];
      await page.getByRole("button", { name: retry, exact: true }).click();
      await expect(page.getByText(disabled, { exact: true })).toBeVisible();
      await page.reload();
      await expect(page.getByText(disabled, { exact: true })).toBeVisible();
      if (locale === "zh-TW") await page.screenshot({ path: path.join(directory, `${viewport.frame}.png`), animations: "disabled" });
      await appendFile(path.join(evidence, "growth-click-ledger.jsonl"), JSON.stringify({ route: page.url(), control: retry, operation: "click + reload", expected: disabled, actual: "visible", result: "PASS" }) + "\n");
    }
  }
}
