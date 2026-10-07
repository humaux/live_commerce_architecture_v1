// Purpose: pure wire contract of the merchant product-image list (contracts/catalog-inventory-v1.md "Amendment — product-media-v2"): the
// per-role caps pinned to the Go constants, and the strict validators of one Image and of the ImageList. No fetch, no DOM, no Next imports,
// so Node tests (tests/admin/product-media-model.test.ts) import it directly.
// Depends on: ./model (types only). Mirrors Go internal/catalog/images.go (Image, ImageList, MaxMainImages/MaxDetailImages/MaxOptionImages).
// Used by: apps/admin/lib/images-client.ts (re-exports it), product editor components via images-client.
import type { ProductImage, ProductImageList } from "./model.ts";

export const MAX_PHOTO_BYTES = 2 * 1024 * 1024;
export const PHOTO_ACCEPT = "image/jpeg,image/png,image/webp";
const PHOTO_TYPES = new Set(PHOTO_ACCEPT.split(","));
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
// product-media-v2 caps (contracts/catalog-inventory-v1.md); pinned to Go catalog.MaxMainImages / MaxDetailImages / MaxOptionImages by
// tests/admin/product-media-model.test.ts. MAX_PHOTOS stays the name of the main-gallery cap the current UI already uses.
export const MAX_MAIN_PHOTOS = 4;
export const MAX_DETAIL_PHOTOS = 20;
export const MAX_OPTION_PHOTOS = 50;
export const MAX_PHOTOS = MAX_MAIN_PHOTOS;
const ROLE_CAP = { main: MAX_MAIN_PHOTOS, detail: MAX_DETAIL_PHOTOS, sku: MAX_OPTION_PHOTOS } as const;
const ROLE_RANK = { main: 0, detail: 1, sku: 2 } as const;
const integerOrNull = (v: unknown) => v === null || (typeof v === "number" && Number.isInteger(v) && v >= 1);
/** Validate one closed image DTO without accepting unknown keys or out-of-role positions. */
export function validImage(v: unknown): v is ProductImage {
  if (!v || typeof v !== "object") return false;
  const r = v as Record<string, unknown>;
  const cap = typeof r.role === "string" && r.role in ROLE_CAP ? ROLE_CAP[r.role as ProductImage["role"]] : 0;
  return (
    Object.keys(r).sort().join(",") === "content_type,height,id,position,product_id,role,size_bytes,version,width" &&
    typeof r.id === "string" && UUID.test(r.id) &&
    typeof r.product_id === "string" && UUID.test(r.product_id) &&
    typeof r.position === "number" && Number.isInteger(r.position) && r.position >= 0 && r.position < cap &&
    typeof r.content_type === "string" && PHOTO_TYPES.has(r.content_type) &&
    typeof r.size_bytes === "number" && Number.isInteger(r.size_bytes) && r.size_bytes >= 1 && r.size_bytes <= MAX_PHOTO_BYTES &&
    integerOrNull(r.width) && integerOrNull(r.height) &&
    typeof r.version === "number" && Number.isInteger(r.version) && r.version >= 1
  );
}
const textCount = (s: string) => [...s].length; // Go utf8.RuneCountInString, not UTF-16 storage units.
const validOptionImage = (v: unknown) => {
  const r = v as Record<string, unknown>;
  return (
    !!r && typeof r === "object" &&
    Object.keys(r).sort().join(",") === "image_id,option_name,option_value" &&
    typeof r.image_id === "string" && UUID.test(r.image_id) &&
    typeof r.option_name === "string" && textCount(r.option_name) >= 1 && textCount(r.option_name) <= 30 &&
    typeof r.option_value === "string" && textCount(r.option_value) >= 1 && textCount(r.option_value) <= 40
  );
};
// The exact ImageList wire shape: items ordered main, detail, sku with contiguous positions inside each role and each role under its cap.
/** Validate the closed role-aware image list and Unicode limits shared with Go. */
export function validImageList(v: unknown): v is ProductImageList {
  if (!v || typeof v !== "object") return false;
  const r = v as Record<string, unknown>;
  if (
    Object.keys(r).sort().join(",") !== "image_axis,items,option_images" ||
    !Array.isArray(r.items) || !r.items.every(validImage) ||
    !(r.image_axis === null || (typeof r.image_axis === "string" && textCount(r.image_axis) >= 1 && textCount(r.image_axis) <= 30)) ||
    !Array.isArray(r.option_images) || r.option_images.length > MAX_OPTION_PHOTOS || !r.option_images.every(validOptionImage)
  )
    return false;
  const items = r.items as ProductImage[];
  const next: Record<string, number> = { main: 0, detail: 0, sku: 0 };
  let rank = 0;
  for (const image of items) {
    if (ROLE_RANK[image.role] < rank || image.position !== next[image.role]++) return false;
    rank = ROLE_RANK[image.role];
  }
  return true;
}

// items = the MAIN images only (what the current gallery UI shows); list = the full ImageList (detail, sku, image axis) for the media editor.
