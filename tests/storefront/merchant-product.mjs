// Shared merchant step of the joint browser gates (merchant-buyer-gate, storefront-publish-gate, catalog-media-gate): create ONE live product
// with ONE SKU through the real admin UI, signed in as the MOCK-IdP merchant.
// stop-bleed D01 (product-editor §c9): the inventory page no longer has the inline "quick add" (product, then first SKU); the only product
// creation UI is the product editor: /products/new -> document with default SKU -> cover upload/order -> publish.
// Prices are typed in MAJOR units ("123.45"; whole dollars for TWD, D02).
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
  await merchant.getByTestId("product-price").fill(price);
  await merchant.getByTestId("product-quantity").fill("0");
  await merchant.locator("#pricing details summary").click();
  await merchant.locator("#pricing details input").fill(code);
  const png=Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB"+"CAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==","base64");
  await merchant.getByTestId("photo-input").setInputFiles({name:"fixture.png",mimeType:"image/png",buffer:png});
  const productReply=api("POST","products/document");
  await merchant.getByTestId("product-publish").click();
  const created=await productReply; assert.equal(created.status(),200,"product document create");
  const product=await created.json(),sku=product.skus[0];assert.equal(sku.product_id,product.id);
  await expect(merchant.getByTestId("product-save-result")).toBeVisible();
  const link=merchant.getByTestId("product-save-result").locator(`a[href*="/products/${product.id}"]`);await link.click();await merchant.waitForURL(url=>url.pathname.endsWith("/products/"+product.id));await merchant.reload();
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
