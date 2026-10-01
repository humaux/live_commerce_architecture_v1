// Shared steps for browser gates that used to start on the old single-product page (/{locale}/products/{id} with its SKU radios and
// "Choose delivery" button). That page is now the storefront shell: a product is bought through Add to cart, and the delivery, quotation,
// address, payment and order screens live at /{locale}/checkout (apps/storefront/lib/routes.ts; the CVS map return allowlist accepts exactly
// that path, migration 0093). A gate that only needs "this buyer has this product in the cart and is on the checkout screen" calls
// reachCheckout() instead of its old first steps and then continues with the unchanged selectors (data-testids and copy of the checkout
// screens did not change). Id URLs still work: /{locale}/products/{uuid} answers a permanent redirect to the slug page.
// `sku` names a variant chip (the fixtures give a product one option axis whose values are its SKU codes, see
// tests/foundation/browser_storefront_fixture_test.go sfiAxisBySKUCode); without it the page's own default variant is bought.
export async function reachCheckout(page, origin, locale, productRef, { quantity = 1, sku = null } = {}) {
  await addToCart(page, origin, locale, productRef, { quantity, sku });
  await page.getByTestId("cart-checkout").click();
  await page.waitForURL(`**/${locale}/checkout`);
}

// Product page -> Add to cart (the cart drawer opens afterwards). Leaves the page on the product with the drawer open.
export async function addToCart(page, origin, locale, productRef, { quantity = 1, sku = null } = {}) {
  await page.goto(`${origin}/${locale}/products/${productRef}`);
  if (sku) await page.locator("label.sf-chip", { has: page.getByRole("radio", { name: sku, exact: true }) }).click();
  for (let i = 1; i < quantity; i++) await page.getByTestId("qty-input").locator("xpath=following-sibling::button").click();
  await page.getByTestId("add-to-cart").click();
  await page.getByTestId("cart-checkout").waitFor();
}

// The checkout path of a locale (no origin), for URL assertions.
export const checkoutURL = (origin, locale) => `${origin}/${locale}/checkout`;

// Switch the UI language the way a buyer does in the shell: the footer language link (a full navigation to the same path under the other
// locale; the old header <select> swapped the locale in place, so in-memory form state no longer survives it, journals in storage do).
export async function switchLocale(page, locale) {
  if ((await page.locator("html").getAttribute("lang")) === locale) return; // already there: nothing to navigate
  await page.locator(`footer nav a[hreflang="${locale}"]`).click();
  await page.waitForURL(`**/${locale}/**`);
  await page.locator(`html[lang="${locale}"]`).waitFor();
}

// Open a buyer session the way the old purchase page did on load, without touching the cart: the same three BFF calls as
// apps/storefront/lib/buyer-client.ts initializeBuyerSession (prepare -> read -> activate), over plain fetch inside the page. The shell creates a
// session only at the first cart write, and a cart write waits for the cross-tab purchase Web Lock, which a gate that holds a payment prepare in
// another tab must not wait for. Returns the active session context.
export async function openBuyerSession(page) {
  return page.evaluate(async () => {
    const call = async (method, suffix, context) => {
      const headers = { "Content-Type": "application/json" };
      if (context !== undefined) headers["X-Buyer-Context"] = context;
      const response = await fetch(`/api/buyer/${suffix}`, { method, headers, body: method === "POST" ? "{}" : undefined, cache: "no-store", credentials: "same-origin" });
      return response.json();
    };
    let current = await call("GET", "session");
    if (current.state === "absent") await call("POST", "session/prepare"), (current = await call("GET", "session"));
    if (current.state === "inactive") await call("POST", "session/activate", current.context), (current = await call("GET", "session"));
    return current.context;
  });
}
