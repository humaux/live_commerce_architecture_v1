// Storefront URL builders (no I/O). One place so the sitemap, canonical tags, cards and the cart agree on paths.
// The checkout surface lives at /{locale}/checkout. The CVS map return path is allowlisted in Go and SQL
// (internal/checkout/cvs.go returnPathPattern, migration 0093 open_cvs_selection + cvs_selections CHECK) as exactly
// /{zh-CN|zh-TW|en}/checkout or the product form /{locale}/products/{id}; any other path is 422 bad_return_path.
import type { Locale } from "@live-commerce/i18n";
import type { ImageSize } from "./shop-contract";

export const checkoutPath = (locale: Locale) => `/${locale}/checkout`;
export const cartPath = (locale: Locale) => `/${locale}/cart`;
export const productPath = (locale: Locale, slugOrID: string) => `/${locale}/products/${slugOrID}`;
export const collectionPath = (locale: Locale, slug: string) => `/${locale}/collections/${slug}`;
export const productImage = (productID: string, imageID: string) => `/media/p/${productID}/${imageID}`;
// Native img candidates share the Host-scoped media route; never use Next's cross-Host optimizer cache.
export const productImageSet = (productID: string, imageID: string, sizes: ImageSize[] = []) => {
  const unique = [...sizes].sort((a, b) => a.width - b.width)
    .filter((s, i, all) => all.findIndex(other => other.pixel_width === s.pixel_width) === i);
  return unique.length ? unique.map(s => `${productImage(productID, imageID)}?w=${s.width} ${s.pixel_width}w`).join(", ") : undefined;
};
export const productCardSizes = "(min-width: 1280px) 289px, (min-width: 1080px) calc((100vw - 124px) / 4), (min-width: 720px) calc((100vw - 88px) / 3), calc((100vw - 44px) / 2)";
export const productRailSizes = "(min-width: 1280px) 280px, (min-width: 720px) 24vw, (min-width: 464px) 250px, 58vw";
export const storeImage = (imageID: string) => `/media/s/${imageID}`;
export const collectionImage = (collectionID: string, imageID: string) => `/media/c/${collectionID}/${imageID}`;
