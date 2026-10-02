// Shared merchant step of the joint browser gates (merchant-buyer-gate, storefront-publish-gate, catalog-media-gate): create ONE live product
// with ONE SKU through the real admin UI, signed in as the MOCK-IdP merchant.
// stop-bleed D01 (product-editor §c9): the inventory page no longer has the inline "quick add" (product, then first SKU); the only product
// creation UI is the product editor, so the gates go through it: /products/new (name, description) -> the editor opens on the draft ->
// the default variant (code + price) -> status "active" -> save. Prices are typed in MAJOR units ("123.45"; whole dollars for TWD, D02).
// The helper never writes anywhere but through those controls: every request is the admin BFF's own (Idempotency-Key, CSRF), nothing is
// seeded around the UI.
import assert from "node:assert/strict";
import { expect } from "@playwright/test";

export async function createProductInEditor(merchant, { adminOrigin, store, locale = "en", name, description, code, price }) {
  const api = (method, resource) => merchant.waitForResponse(response => response.request().method() === method && new URL(response.url()).pathname === `/api/stores/${store}/${resource}`);
  await merchant.goto(`${adminOrigin}/${locale}/products/new?store=${store}`);
  await expect(merchant.getByTestId("product-create-form")).toBeVisible();
  await merchant.getByTestId("product-name").fill(name);
  await merchant.getByTestId("product-description").fill(description);
  const productReply = api("POST", "products");
  await merchant.getByTestId("product-create").click();
  const created = await productReply;
  assert.equal(created.status(), 200, "product create");
  const product = await created.json();
  await merchant.waitForURL(url => url.pathname.endsWith(`/products/${product.id}`));
  // the first SKU: a product without option axes has one default variant row
  await expect(merchant.getByTestId("new-variants")).toBeVisible();
  await merchant.getByTestId("new-code-0").fill(code);
  await merchant.getByTestId("new-price-0").fill(price);
  const skuReply = api("POST", "skus");
  await merchant.getByTestId("new-variants-create").click();
  const skuResponse = await skuReply;
  assert.equal(skuResponse.status(), 200, "first SKU create");
  const sku = await skuResponse.json();
  assert.equal(sku.product_id, product.id);
  // a new product is a draft (contracts/storefront-v2.md A); the old quick-add made it live at once, so the gates publish it the same way
  await expect(merchant.getByTestId("product-status")).toHaveValue("draft");
  await merchant.getByTestId("product-status").selectOption("active");
  const saved = api("PATCH", `products/${product.id}`);
  await merchant.getByTestId("product-save").click();
  assert.equal((await saved).status(), 200, "product activate");
  await expect(merchant.getByTestId("product-status")).toHaveValue("active");
  return { product, sku };
}

// The stock ledger row of a SKU code (search, then select it) so the tray and the purchase entry are about that product.
export async function selectLedgerRow(merchant, { adminOrigin, locale = "en", code, searchLabel = "Search product name or SKU", searchButton = "Search" }) {
  await merchant.goto(`${adminOrigin}/${locale}/inventory`);
  await merchant.getByRole("textbox", { name: searchLabel }).fill(code);
  await merchant.getByRole("button", { name: searchButton, exact: true }).click();
  const row = merchant.locator("tbody tr").filter({ hasText: code });
  await expect(row).toHaveCount(1);
  await row.locator("button.product-name").click();
  return row;
}
