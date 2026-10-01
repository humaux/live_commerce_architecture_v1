// Storefront URL builders (no I/O). One place so the sitemap, canonical tags, cards and the cart agree on paths.
// The checkout surface lives at /{locale}/checkout. The CVS map return path is allowlisted in Go and SQL
// (internal/checkout/cvs.go returnPathPattern, migration 0093 open_cvs_selection + cvs_selections CHECK) as exactly
// /{zh-CN|zh-TW|en}/checkout or the product form /{locale}/products/{id}; any other path is 422 bad_return_path.
import type { Locale } from "@live-commerce/i18n";

export const checkoutPath = (locale: Locale) => `/${locale}/checkout`;
export const cartPath = (locale: Locale) => `/${locale}/cart`;
export const productPath = (locale: Locale, slugOrID: string) => `/${locale}/products/${slugOrID}`;
export const collectionPath = (locale: Locale, slug: string) => `/${locale}/collections/${slug}`;
export const productImage = (productID: string, imageID: string) => `/media/p/${productID}/${imageID}`;
export const storeImage = (imageID: string) => `/media/s/${imageID}`;
export const collectionImage = (collectionID: string, imageID: string) => `/media/c/${collectionID}/${imageID}`;
