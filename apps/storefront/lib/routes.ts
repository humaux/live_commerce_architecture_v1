// Storefront URL builders (no I/O). One place so the sitemap, canonical tags, cards and the cart agree on paths.
// The checkout surface lives at /{locale}/products/Checkout ON PURPOSE: the CVS map return path is allowlisted in Go and SQL
// (internal/checkout/cvs.go returnPathPattern, migrations/0073 open_cvs_selection) as /{locale}/products/[A-Za-z0-9_-]{1,64};
// anything else is 422 bad_return_path. "Checkout" has a capital letter, so it can never collide with a product slug
// (lowercase, contracts/storefront-v2.md section A) or a UUID. Widen that allowlist, then move this to /{locale}/checkout.
import type { Locale } from "@live-commerce/i18n";

export const CHECKOUT_SEGMENT = "Checkout";
export const checkoutPath = (locale: Locale) => `/${locale}/products/${CHECKOUT_SEGMENT}`;
export const cartPath = (locale: Locale) => `/${locale}/cart`;
export const productPath = (locale: Locale, slugOrID: string) => `/${locale}/products/${slugOrID}`;
export const collectionPath = (locale: Locale, slug: string) => `/${locale}/collections/${slug}`;
export const productImage = (productID: string, imageID: string) => `/media/p/${productID}/${imageID}`;
export const storeImage = (imageID: string) => `/media/s/${imageID}`;
export const collectionImage = (collectionID: string, imageID: string) => `/media/c/${collectionID}/${imageID}`;
