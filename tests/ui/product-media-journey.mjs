// Purpose: upload one main cover in click-sweep J1 through the production media picker.
// Depends on: Playwright page/expect and Node assert; acknowledgement is read from the clicked admin BFF request.
// Used by: click-sweep.mjs J1 and DB-free driver sequencing tests; no API write shortcut.
import assert from "node:assert/strict";
import { expect } from "@playwright/test";
/** Upload J1's cover; injected checks exercise sequencing without claiming browser acceptance.
 * @param {import("@playwright/test").Page} page
 * @param {{name: string, mimeType: string, buffer: Buffer}} file
 * @param {typeof expect} check
 */
export async function uploadJourneyMainCover(page, file, check = expect) {
  const main = page.getByTestId("media-main-list");
  await check(main).toBeVisible();
  await check(page.getByTestId("media-axis-picker")).toBeVisible();
  await check(page.getByTestId("media-detail-list")).toBeVisible();
  const input = page.getByTestId("photo-input");
  // setInputFiles bypasses actionability; the real picker must finish loading its canonical media head first.
  await check(input).toBeEnabled();
  const current = new URL(page.url());
  const product = current.pathname.split("/").at(-1);
  const imagePath = `/api/stores/${current.searchParams.get("store")}/products/${product}/images`;
  const accepted = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      response.request().method() === "POST" &&
      url.pathname === imagePath &&
      url.searchParams.get("role") === "main"
    );
  });
  const chooser = page.waitForEvent("filechooser");
  const trigger = (await input.isVisible()) ? input : input.locator("xpath=..");
  await trigger.click();
  await (await chooser).setFiles(file);
  const response = await accepted;
  await check(response.status()).toBe(200);
  const image = await response.json();
  assert.equal(image.role, "main", "the upload must produce a main cover");
  assert.match(
    image.id,
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i,
  );
  await check(
    main.locator(`[data-image-id="${image.id}"]`).locator("img"),
  ).toBeVisible();
  await check(main.locator("img")).toHaveCount(1);
  await check(page.getByTestId("media-sku-list").locator("img")).toHaveCount(0);
  await check(page.getByTestId("media-detail-list").locator("img")).toHaveCount(
    0,
  );
  return image.id;
}
