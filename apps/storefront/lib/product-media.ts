// Purpose: select display photos from the frozen product-media-v2 buyer metadata without changing main/detail ownership.
// Depends on: shop-contract DetailImage/Variant and purchase Product types; no I/O or inferred stock/money.
// Used by: ProductGallery and cart-details; PM-U Node selection tests.
import type { DetailImage, Variant } from "./shop-contract.ts";
import type { Product } from "./purchase-types.ts";

/** Select an option photo for the first display slot; original main thumbnails remain unchanged. No I/O. */
export function mainGallerySlides(images: DetailImage[], optionImageID: string | null): DetailImage[] {
  if (!optionImageID) return images;
  // Option-image metadata has no dimensions in the frozen response; never borrow the cover's ratio or srcset.
  const option = images.find(image => image.id === optionImageID) ?? { id: optionImageID, width: null, height: null };
  return [option, ...images.slice(1)];
}

/** Resolve the server SKU thumbnail, falling back to the main cover for older responses. No I/O. */
export function cartLineImage(variant: Pick<Variant, "image_id"> | null, row: Pick<Product, "image_id" | "images">, images: DetailImage[]): string | null {
  return variant?.image_id ?? row.image_id ?? images[0]?.id ?? row.images?.[0]?.id ?? null;
}
