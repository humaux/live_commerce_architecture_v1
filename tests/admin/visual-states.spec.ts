import { test, expect } from "./fixtures/ledger-identity";
import { mkdir, writeFile } from "node:fs/promises";
import type { APIRequestContext } from "@playwright/test";

// This spec alone adds a realistic variant code to the disposable Go/PG
// fixture. The real catalog projection and intrinsic table layout must create
// the overflow; no DOM, CSS or API read response is overridden. The Go harness
// removes the whole isolated fixture after the suite, including this product.
async function createLongSkuFixture(request: APIRequestContext) {
  const storeID = process.env.COMMERCE_FIXTURE_STORE_ID;
  expect(storeID).toBeTruthy();
  // G-UI8 audit [FIXTURE/SETUP]: fixture: creates the long-name product through the merchant BFF API
  const product = await request.post(`/api/stores/${storeID}/products`, {
    headers: { Origin: "http://127.0.0.1:3100", "Idempotency-Key": crypto.randomUUID() },
    data: {
      name: "Rechargeable behind-the-ear hearing aid with charging case",
      description: "Isolated browser fixture, not a real merchant product",
      status: "active",
    },
  });
  // Existing catalog bodyRoute returns 200 for successful commands.
  expect(product.status()).toBe(200);
  const { id } = await product.json();
  const code = "HA-RECHARGEABLE-BTE-BLUETOOTH-CHARGER-BLACK-TW-2026";
  // G-UI8 audit [FIXTURE/SETUP]: fixture: creates its SKU through the merchant BFF API
  const sku = await request.post(`/api/stores/${storeID}/skus`, {
    headers: { Origin: "http://127.0.0.1:3100", "Idempotency-Key": crypto.randomUUID() },
    data: { product_id: id, code, price_minor: 198000 },
  });
  expect(sku.status()).toBe(200);
  expect(await sku.json()).toMatchObject({ product_id: id, code });
  return code;
}

test("ledger selection caret and scroll surface are authored and active", async ({
  page,
  request,
}) => {
  const longSKU = await createLongSkuFixture(request);
  await mkdir("output/playwright/ledger-review", { recursive: true });
  await page.goto("/en/inventory");
  await expect(page.getByRole("radio", { name: `Select ${longSKU}`, exact: true })).toBeVisible();
  const input = page.locator(".search-field input");
  await input.fill("Visible selection");
  await input.selectText();
  const styles = await input.evaluate((element) => {
    const field = element as HTMLInputElement;
    return {
      selectionStart: field.selectionStart,
      selectionEnd: field.selectionEnd,
      selectionBackground: getComputedStyle(field, "::selection")
        .backgroundColor,
      selectionColor: getComputedStyle(field, "::selection").color,
      caretColor: getComputedStyle(field).caretColor,
      focused: document.activeElement === field,
    };
  });
  expect(styles).toMatchObject({
    selectionStart: 0,
    selectionEnd: 17,
    selectionBackground: "rgb(201, 230, 222)",
    selectionColor: "rgb(20, 41, 66)",
    caretColor: "rgb(36, 121, 101)",
    focused: true,
  });
  await input.screenshot({
    path: "output/playwright/ledger-review/selection-active.png",
    caret: "initial",
  });
  await input.press("ArrowRight");
  const caret = await input.evaluate((element) => ({
    start: (element as HTMLInputElement).selectionStart,
    end: (element as HTMLInputElement).selectionEnd,
    focused: document.activeElement === element,
  }));
  expect(caret).toEqual({ start: 17, end: 17, focused: true });
  await input.screenshot({
    path: "output/playwright/ledger-review/caret-active.png",
    caret: "initial",
  });
  await input.fill("");

  // Use real responsive widths; never inject a fake overflow or scrollbar.
  const table = page.locator(".table-scroll");
  let overflowWidth = 0;
  const viewportMeasurements = [];
  // Include the W0 fixed-rail boundary; below 1024 the drawer frees table width.
  for (const width of [1100, 1024, 900, 820, 740, 681]) {
    await page.setViewportSize({ width, height: 992 });
    viewportMeasurements.push(await table.evaluate((element) => ({ viewport: innerWidth, scrollWidth: element.scrollWidth, clientWidth: element.clientWidth })));
    if (
      await table.evaluate(
        (element) => element.scrollWidth > element.clientWidth,
      )
    ) {
      overflowWidth = width;
      break;
    }
  }
  await writeFile("output/playwright/ledger-review/scroll-widths.json", JSON.stringify(viewportMeasurements, null, 2) + "\n");
  expect(overflowWidth).toBeGreaterThan(0);
  await table.hover();
  await page.mouse.wheel(120, 0);
  await expect
    .poll(() => table.evaluate((element) => element.scrollLeft))
    .toBeGreaterThan(0);
  const scroll = await table.evaluate((element) => ({
    scrollWidth: element.scrollWidth,
    clientWidth: element.clientWidth,
    scrollLeft: element.scrollLeft,
    scrollbarColor: getComputedStyle(element).scrollbarColor,
    scrollbarWidth: getComputedStyle(element).scrollbarWidth,
  }));
  expect(scroll.scrollbarColor).toBe("rgb(86, 97, 113) rgb(245, 246, 248)");
  expect(scroll.scrollbarWidth).toBe("thin");
  await table.screenshot({ path: "output/playwright/ledger-review/scrollbar-active.png" });
  await writeFile(
    "output/playwright/ledger-review/active-style-evidence.json",
    JSON.stringify({ styles, caret, overflowWidth, scroll }, null, 2),
  );
  // Next retains hidden dev-tool DOM; only visible chrome can cover the UI.
  await expect(
    page.locator("nextjs-portal").locator("button:visible"),
  ).toHaveCount(0);
});
