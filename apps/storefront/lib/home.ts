// Storefront home model (catalog-media CM6). Pure functions, no I/O: the page component (components/ShopHome.tsx)
// reads BFF GET /api/buyer/catalog (-> Go /v1/buyer/catalog) and this module validates the page and folds the SKU
// rows into one card per product. It never decides price or availability (Go does; the product page re-reads them)
// and the card price is display only: "from" the lowest ACTIVE SKU price, since the catalog lists active SKUs only.
import { validProduct } from "./purchase.ts";
import type { Product, ProductImageMeta } from "./purchase.ts";

export type HomePage = { items: Product[]; next_cursor: string; store_name: string };
export type HomeProduct = {
  product_id: string;
  name: string;
  currency: string;
  price_minor: number; // lowest active SKU price of the product
  cover: ProductImageMeta | null;
};

// Same public path the Next media route serves (app/media/p/[productID]/[imageID]/route.ts).
export const mediaPath = (productID: string, imageID: string) => `/media/p/${productID}/${imageID}`;

export const validHomePage = (v: unknown): v is HomePage =>
  !!v &&
  typeof v === "object" &&
  !Array.isArray(v) &&
  Object.keys(v).sort().join() === "items,next_cursor,store_name" &&
  Array.isArray((v as HomePage).items) &&
  (v as HomePage).items.length <= 100 &&
  (v as HomePage).items.every(validProduct) &&
  typeof (v as HomePage).next_cursor === "string" &&
  (v as HomePage).next_cursor.length <= 2048 &&
  typeof (v as HomePage).store_name === "string" &&
  (v as HomePage).store_name.length <= 200;

// One card per product in first-seen order (the catalog is ordered by SKU id, which is stable across pages).
// Rows of one product share currency (store currency) and images; the lowest price wins.
export function groupProducts(rows: Product[]): HomeProduct[] {
  const byID = new Map<string, HomeProduct>();
  for (const row of rows) {
    const known = byID.get(row.product_id);
    if (!known) {
      byID.set(row.product_id, {
        product_id: row.product_id,
        name: row.name,
        currency: row.currency,
        price_minor: row.price_minor,
        cover: row.images?.[0] ?? null,
      });
    } else if (row.currency === known.currency && row.price_minor < known.price_minor) {
      known.price_minor = row.price_minor;
    }
  }
  return [...byID.values()];
}
