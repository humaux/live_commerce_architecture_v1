// Product photo requests of the admin Ledger (catalog-media CM3/CM5). Browser -> BFF
// apps/admin/app/api/stores/[store]/[...resource]/route.ts (allowlisted paths; the product-media-v2 move / option-images / image-axis paths are added with the media editor UI) -> Go
// internal/httpapi/images.go (GET/POST products/{id}/images, GET images/{image}, POST images/{image}/delete,
// POST images/order). Owns: the wire shapes, the strict list validator and the client-side pre-checks. It never
// decides what is a valid photo: the server sniffs the bytes and is the authority; the client checks only save a
// round trip. Upload is a single multipart `file` part; each mutation sends a fresh Idempotency-Key and is NOT
// journaled (a File cannot be persisted), so after a lost response the merchant re-reads the list before retrying.
import type { ProductImage, ProductImageList } from "./model";
import { csrfToken, unknownError } from "./client";

// Wire contract (caps, strict validators) lives in image-list-contract.ts so Node tests can import it without the browser client.
import { MAX_PHOTO_BYTES, MAX_PHOTOS, PHOTO_ACCEPT, validImage, validImageList } from "./image-list-contract.ts";
export { MAX_DETAIL_PHOTOS, MAX_MAIN_PHOTOS, MAX_OPTION_PHOTOS, MAX_PHOTO_BYTES, MAX_PHOTOS, PHOTO_ACCEPT, validImage, validImageList } from "./image-list-contract.ts";
const PHOTO_TYPES = new Set(PHOTO_ACCEPT.split(","));

export const imagesPath = (store: string, product: string) => `/api/stores/${store}/products/${product}/images`;
export const imageURL = (store: string, product: string, image: string) => `${imagesPath(store, product)}/${image}`;

// "type" or "size" when the browser can already tell the server will refuse; null when the file may be sent.
export function photoFileProblem(file: { type: string; size: number }): "type" | "size" | null {
  if (!PHOTO_TYPES.has(file.type)) return "type";
  return file.size < 1 || file.size > MAX_PHOTO_BYTES ? "size" : null;
}

// items = the MAIN images only (what the current gallery UI shows); list = the full ImageList (detail, sku, image axis) for the media editor.
export type ImageResult = { status: number; items: ProductImage[] | null; list: ProductImageList | null; body: unknown };

async function call(url: string, init: RequestInit): Promise<ImageResult> {
  try {
    const response = await fetch(url, { ...init, cache: "no-store", signal: AbortSignal.timeout(15000) });
    const body: unknown = await response.json().catch(() => unknownError);
    // An answer that is not the exact list shape is treated as a failure, never as an empty list.
    if (response.ok && validImageList(body)) return { status: response.status, items: body.items.filter((i) => i.role === "main"), list: body, body };
    if (response.ok && body && typeof body === "object" && validImage(body)) return { status: response.status, items: null, list: null, body };
    return { status: response.ok ? 503 : response.status, items: null, list: null, body };
  } catch {
    return { status: 503, items: null, list: null, body: unknownError };
  }
}

const mutating = () => ({ "X-CSRF-Token": csrfToken(), "Idempotency-Key": crypto.randomUUID() });

export const listImages = (store: string, product: string, signal?: AbortSignal) =>
  call(imagesPath(store, product), { method: "GET", signal });
export function uploadImage(store: string, product: string, file: File) {
  const form = new FormData();
  form.append("file", file); // field name `file`; the browser sets the multipart boundary itself
  return call(imagesPath(store, product), { method: "POST", headers: mutating(), body: form });
}
export const deleteImage = (store: string, product: string, image: string) =>
  call(`${imageURL(store, product, image)}/delete`, {
    method: "POST",
    headers: { ...mutating(), "Content-Type": "application/json" },
    body: "{}",
  });
export const reorderImages = (store: string, product: string, ids: string[]) =>
  call(`${imagesPath(store, product)}/order`, {
    method: "POST",
    headers: { ...mutating(), "Content-Type": "application/json" },
    body: JSON.stringify({ ids }),
  });
