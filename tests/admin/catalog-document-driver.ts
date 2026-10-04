// CC12's assertions translated to the one-document UI; all mutations remain real clicks.
import { expect, type Page } from "@playwright/test";
import { productEditorCopy } from "../../apps/admin/lib/product-editor-copy";
export async function driveDocument(
  page: Page,
  L: "en" | "zh-TW",
  uniq: string,
  writes: { method: string }[],
  shop: (p: string) => Promise<any>,
  shot: (name: string) => Promise<void>,
  save: () => Promise<void>,
) {
  const c = productEditorCopy[L];
  await page.getByTestId("product-new").click();
  await expect(page).toHaveURL(/\/products\/new/);
  await expect(page.getByTestId("product-create-form")).toBeVisible();
  await page.getByTestId("product-name").fill(`${uniq} Linen Tee`);
  await page
    .getByTestId("product-description")
    .fill("Soft linen. <b>not bold</b>");
  await shot("create");
  for (const [i, name, values] of [
    [0, "顏色", "紅, 藍"],
    [1, "尺寸", "S, M, L"],
  ] as const) {
    await page.getByTestId("axis-add").click();
    await page.getByTestId(`axis-name-${i}`).fill(name);
    await page.getByTestId(`axis-values-${i}`).fill(values);
    await page.getByTestId(`axis-values-${i}`).press("Enter");
  }
  await page.getByTestId("axis-add").click();
  await expect(page.getByTestId("axis-name-2")).toBeVisible();
  await expect(page.getByTestId("axis-add")).toBeDisabled();
  await page.locator(".product-axis").nth(2).getByRole("button").last().click();
  await expect(page.getByTestId("axis-name-2")).toHaveCount(0);
  const rows = page.locator('[data-testid^="matrix-row-"]');
  await expect(rows).toHaveCount(6);
  const titles = await rows.locator("strong").allTextContents();
  expect([...titles].sort(), "2x3 matrix").toEqual(
    ["紅 / S", "紅 / M", "紅 / L", "藍 / S", "藍 / M", "藍 / L"].sort(),
  );
  await page.getByTestId("product-create").click();
  await expect(page.getByTestId("product-message")).toBeVisible();
  const whole = (await page.locator(".pe-matrix-head").innerText()).includes(
    "NT$",
  );
  const shownPrice = (minor: number) =>
    whole ? String(minor / 100) : (minor / 100).toFixed(2);
  for (let i = 0; i < 6; i++) {
    await page.getByTestId(`new-price-${i}`).fill(String(10 + 2 * i));
    await page.getByTestId(`matrix-quantity-${i}`).fill("0");
  }
  if (whole) {
    await page.getByTestId("new-price-0").fill("10.5");
    await page.getByTestId("product-create").click();
    await expect(page.getByTestId("product-message")).toContainText(
      c.invalid_price,
    );
    await page.getByTestId("new-price-0").fill("10");
  }
  await shot("matrix");
  await page.getByTestId("product-create").click();
  await expect(page.getByTestId("product-save-result")).toBeVisible();
  await page
    .getByTestId("product-save-result")
    .getByRole("link", { name: c.save, exact: true })
    .click();
  await expect(page.getByTestId("product-form")).toBeVisible();
  const productId = /\/products\/([0-9a-f-]{36})/.exec(page.url())![1];
  await expect(page.getByTestId("product-status")).toHaveValue("draft");
  await expect(page.getByTestId("product-slug")).toHaveValue(
    `${uniq}-linen-tee`,
  );
  const before = writes.filter((w) => w.method === "PUT").length;
  await page.getByTestId("product-save").click();
  await expect(page.getByTestId("product-message")).toContainText(c.noChanges);
  expect(
    writes.filter((w) => w.method === "PUT"),
    "no-op sends no command",
  ).toHaveLength(before);
  await expect(page.locator(".product-editor b"), "no raw HTML").toHaveCount(0);
  await page.locator("#seo summary").click();
  await page.getByTestId("product-seo-title").fill("Linen tee, soft");
  await page
    .getByTestId("product-seo-description")
    .fill("A soft linen tee for summer.");
  await expect(
    page.locator(".product-seo .product-count").first(),
  ).toContainText("15");
  await expect(
    page.locator(".product-seo .product-count").first(),
  ).toContainText("70");
  await expect(
    page.locator(".product-seo .product-count").nth(1),
  ).toContainText("28");
  await expect(
    page.locator(".product-seo .product-count").nth(1),
  ).toContainText("160");
  await save();
  await expect(page.getByTestId("product-seo-title")).toHaveValue(
    "Linen tee, soft",
  );
  expect(
    (await shop(`/v1/buyer/catalog/v2/products?q=${uniq}`)).body.products,
    "draft hidden",
  ).toHaveLength(0);
  expect(
    (await shop(`/v1/buyer/catalog/v2/products/${productId}`)).status,
  ).toBe(404);
  const idByTitle: Record<string, string> = {},
    ids: string[] = [];
  await expect(rows).toHaveCount(6);
  for (let i = 0; i < 6; i++) {
    const id = (await rows.nth(i).getAttribute("data-sku-id"))!;
    ids.push(id);
    idByTitle[(await rows.nth(i).locator("strong").innerText()).trim()] = id;
  }
  const rowFor = (id: string) => page.locator(`[data-sku-id="${id}"]`),
    priceOf = (title: string) => 1000 + 200 * titles.indexOf(title);
  for (const t of titles)
    await expect(
      rowFor(idByTitle[t]).locator('[data-testid^="new-price-"]'),
    ).toHaveValue(shownPrice(priceOf(t)));
  expect(
    (await shop(`/v1/buyer/catalog/v2/products?q=${uniq}`)).body.products,
    "still draft",
  ).toHaveLength(0);
  const cheapest = idByTitle[titles[0]];
  await rowFor(cheapest).locator('[data-testid^="matrix-compare-"]').fill("5");
  await page.getByTestId("product-save").click();
  await expect(page.getByTestId("product-message")).toBeVisible();
  await rowFor(cheapest).locator('[data-testid^="matrix-compare-"]').fill("25");
  await save();
  await expect(
    rowFor(cheapest).locator('[data-testid^="matrix-compare-"]'),
  ).toHaveValue(shownPrice(2500));
  await expect(
    rowFor(cheapest).locator('[data-testid^="new-price-"]'),
  ).toHaveValue(shownPrice(1000));
  const stock = [10, 3, 0, 6, 5, 0];
  await rowFor(cheapest)
    .locator('[data-testid^="matrix-quantity-"]')
    .fill("abc");
  await page.getByTestId("product-save").click();
  await expect(page.getByTestId("product-message")).toBeVisible();
  await rowFor(cheapest).locator('[data-testid^="matrix-quantity-"]').fill("0");
  for (let i = 0; i < 6; i++) {
    if (stock[i] === 0) continue;
    await rowFor(idByTitle[titles[i]])
      .locator('[data-testid^="matrix-quantity-"]')
      .fill(String(stock[i]));
    await save();
    await expect(
      rowFor(idByTitle[titles[i]]).locator('[data-testid^="matrix-quantity-"]'),
    ).toHaveValue(String(stock[i]));
  }
  // PE14: save two distinct SKU shipping edits independently; reload proves each
  // merge patch retained the other SKU and never applied a product-wide override.
  for (const [index, grams] of [[0, "250"], [1, "375"]] as const) {
    await page.locator("#shipping summary").click();
    await page.getByTestId(`shipping-weight-${index}`).fill(grams);
    await save();
    await page.locator("#shipping summary").click();
    await expect(page.getByTestId(`shipping-weight-${index}`)).toHaveValue(grams);
    if(index===1) await expect(page.getByTestId("shipping-weight-0")).toHaveValue("250");
    await page.locator("#shipping summary").click();
  }
  return {
    productId,
    rows,
    titles,
    idByTitle,
    ids,
    priceOf,
    cheapest,
    stock,
    rowFor,
  };
}
