// Display details for the lines of the buyer's cart (drawer, cart page, checkout summary). Browser only.
// A cart line is just {sku_id, quantity}; to show a name, variant, photo and listed price this reads:
//   BFF GET /api/buyer/catalog?limit=100[&cursor=]  -> Go /v1/buyer/catalog  (active SKUs: product_id, name, price, images)
//   GET /api/shop/product/{id}                      -> Go /v1/buyer/catalog/v2/products/{id} (variant title, compare-at, stock, slug)
//   BFF GET /api/buyer/checkout-options             -> Go /v1/buyer/checkout-options (optional free_shipping_threshold_minor)
// A SKU missing from the active catalog is reported as `gone` (archived/draft since it was added) so the UI can offer Remove.
// It never decides money: the quote is the only authority (I05); these numbers are labelled "listed price" by the callers.
import { purchasePage, validOptionRow, validProduct, isUnavailable } from "./purchase.ts";
import type { Cart, Product } from "./purchase.ts";
import { parseProductDetail } from "./shop-contract.ts";
import type { ProductDetail, StockHint } from "./shop-contract.ts";

export type LineView = {
  sku_id: string;
  quantity: number;
  gone: boolean;
  title: string;
  variantTitle: string | null;
  slug: string | null;
  productID: string | null;
  imageID: string | null;
  unitMinor: number;
  compareAtMinor: number | null;
  stock: StockHint | null;
};
export type CartDetails = { lines: LineView[]; subtotal: number; currency: string; threshold: number | null };

const MAX_PAGES = 10;
const details = new Map<string, Promise<ProductDetail | null>>();

function productDetail(productID: string): Promise<ProductDetail | null> {
  const known = details.get(productID);
  if (known) return known;
  const pending = fetch(`/api/shop/product/${productID}`, { cache: "no-store", credentials: "omit" })
    .then(async (response) => (response.ok ? parseProductDetail(await response.json()) : null))
    .catch(() => null);
  details.set(productID, pending);
  // A failed read must not stick for the whole session.
  void pending.then((value) => {
    if (value === null) details.delete(productID);
  });
  return pending;
}

// Stock and compare-at move; drop the memo whenever the cart changes so the hint is never older than the last edit.
export const forgetDetails = () => details.clear();

export async function loadCartDetails(context: string, cart: Cart): Promise<CartDetails> {
  const wanted = new Set(cart.items.map((i) => i.sku_id));
  const rows = new Map<string, Product>();
  let cursor = "";
  for (let page = 0; page < MAX_PAGES && rows.size < wanted.size; page++) {
    const result = await purchasePage(`catalog?limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`, context, validProduct);
    for (const row of result.items) if (wanted.has(row.sku_id)) rows.set(row.sku_id, row);
    if (!result.next_cursor) break;
    cursor = result.next_cursor;
  }
  const productIDs = [...new Set([...rows.values()].map((r) => r.product_id))];
  const fetched = new Map<string, ProductDetail | null>(await Promise.all(productIDs.map(async (id) => [id, await productDetail(id)] as const)));
  const lines: LineView[] = cart.items.map((item) => {
    const row = rows.get(item.sku_id);
    if (!row) return { sku_id: item.sku_id, quantity: item.quantity, gone: true, title: "", variantTitle: null, slug: null, productID: null, imageID: null, unitMinor: 0, compareAtMinor: null, stock: null };
    const detail = fetched.get(row.product_id) ?? null;
    const variant = detail?.variants.find((v) => v.sku_id === item.sku_id) ?? null;
    return {
      sku_id: item.sku_id,
      quantity: item.quantity,
      gone: false,
      title: detail?.title ?? row.name,
      // "預設" is the derived title of an axis-less product; showing it to a buyer is noise.
      variantTitle: variant && variant.option_values.length > 0 ? variant.title : null,
      slug: detail?.slug ?? null,
      productID: row.product_id,
      imageID: detail?.images[0]?.id ?? row.images?.[0]?.id ?? null,
      unitMinor: variant?.price_minor ?? row.price_minor,
      compareAtMinor: variant?.compare_at_minor ?? null,
      stock: variant?.stock ?? null,
    };
  });
  const subtotal = lines.filter((l) => !l.gone).reduce((sum, l) => sum + l.unitMinor * l.quantity, 0);
  let threshold: number | null = null;
  try {
    const options = await purchasePage("checkout-options?limit=100", context, validOptionRow);
    const values = options.items
      .filter((o) => !isUnavailable(o) && o.currency === cart.currency)
      .map((o) => (isUnavailable(o) ? null : (o.free_shipping_threshold_minor ?? null)))
      .filter((v): v is number => v !== null);
    threshold = values.length ? Math.min(...values) : null;
  } catch {
    /* the hint is optional */
  }
  return { lines, subtotal, currency: cart.currency, threshold };
}

// One load per cart identity+version, shared by every component that shows lines (drawer, page, summary), so the catalog
// and detail reads are not repeated per component. A failed load is forgotten so the next render retries.
let memo: { key: string; promise: Promise<CartDetails> } | null = null;
export function cartDetails(context: string, cart: Cart): Promise<CartDetails> {
  const key = `${context}:${cart.id}:${cart.version}`;
  if (memo?.key === key) return memo.promise;
  forgetDetails();
  const promise = loadCartDetails(context, cart);
  memo = { key, promise };
  promise.catch(() => {
    if (memo?.promise === promise) memo = null;
  });
  return promise;
}
