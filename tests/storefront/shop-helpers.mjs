// Shared steps for browser gates that used to start on the old single-product page (/{locale}/products/{id} with its SKU radios and
// "Choose delivery" button). That page is now the storefront shell: a product is bought through Add to cart, and the delivery, quotation,
// address, payment and order screens live at /{locale}/products/Checkout (apps/storefront/lib/routes.ts explains the path). A gate that
// only needs "this buyer has this product in the cart and is on the checkout screen" calls reachCheckout() instead of its old first
// steps and then continues with the unchanged selectors (data-testids and copy of the checkout screens did not change).
// Id URLs still work: /{locale}/products/{uuid} answers a permanent redirect to the slug page.
export async function reachCheckout(page, origin, locale, productRef, { quantity = 1 } = {}) {
  await page.goto(`${origin}/${locale}/products/${productRef}`);
  for (let i = 1; i < quantity; i++) await page.getByTestId("qty-input").locator("xpath=following-sibling::button").click();
  await page.getByTestId("add-to-cart").click();
  await page.getByTestId("cart-checkout").click();
  await page.waitForURL(`**/${locale}/products/Checkout`);
}
