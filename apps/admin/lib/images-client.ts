// Product photo requests of the admin Ledger (catalog-media CM3/CM5). Browser -> BFF
// apps/admin/app/api/stores/[store]/[...resource]/route.ts (exactly five allowlisted paths) -> Go
// internal/httpapi/images.go (GET/POST products/{id}/images, GET images/{image}, POST images/{image}/delete,
// POST images/order). Owns: the wire shapes, the strict list validator and the client-side pre-checks. It never
// decides what is a valid photo: the server sniffs the bytes and is the authority; the client checks only save a
// round trip. Upload is a single multipart `file` part; each mutation sends a fresh Idempotency-Key and is NOT
// journaled (a File cannot be persisted), so after a lost response the merchant re-reads the list before retrying.
import type { ProductImage } from "./model";
import { csrfToken, unknownError } from "./client";

export const MAX_PHOTO_BYTES = 2 * 1024 * 1024;
export const MAX_PHOTOS = 8;
export const PHOTO_ACCEPT = "image/jpeg,image/png,image/webp";
const PHOTO_TYPES = new Set(PHOTO_ACCEPT.split(","));
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export const imagesPath = (store: string, product: string) => `/api/stores/${store}/products/${product}/images`;
export const imageURL = (store: string, product: string, image: string) => `${imagesPath(store, product)}/${image}`;

// "type" or "size" when the browser can already tell the server will refuse; null when the file may be sent.
export function photoFileProblem(file: { type: string; size: number }): "type" | "size" | null {
  if (!PHOTO_TYPES.has(file.type)) return "type";
  return file.size < 1 || file.size > MAX_PHOTO_BYTES ? "size" : null;
}

const integerOrNull = (v: unknown) => v === null || (typeof v === "number" && Number.isInteger(v) && v >= 1);
export function validImage(v: unknown): v is ProductImage {
  if (!v || typeof v !== "object") return false;
  const r = v as Record<string, unknown>;
  return (
    Object.keys(r).sort().join(",") === "content_type,height,id,position,product_id,size_bytes,version,width" &&
    typeof r.id === "string" && UUID.test(r.id) &&
    typeof r.product_id === "string" && UUID.test(r.product_id) &&
    typeof r.position === "number" && Number.isInteger(r.position) && r.position >= 0 && r.position < MAX_PHOTOS &&
    typeof r.content_type === "string" && PHOTO_TYPES.has(r.content_type) &&
    typeof r.size_bytes === "number" && Number.isInteger(r.size_bytes) && r.size_bytes >= 1 && r.size_bytes <= MAX_PHOTO_BYTES &&
    integerOrNull(r.width) && integerOrNull(r.height) &&
    typeof r.version === "number" && Number.isInteger(r.version) && r.version >= 1
  );
}
export function validImageList(v: unknown): v is { items: ProductImage[] } {
  if (!v || typeof v !== "object") return false;
  const r = v as Record<string, unknown>;
  return (
    Object.keys(r).join(",") === "items" &&
    Array.isArray(r.items) &&
    r.items.length <= MAX_PHOTOS &&
    r.items.every(validImage) &&
    r.items.every((image, index) => (image as ProductImage).position === index)
  );
}

export type ImageResult = { status: number; items: ProductImage[] | null; body: unknown };

async function call(url: string, init: RequestInit): Promise<ImageResult> {
  try {
    const response = await fetch(url, { ...init, cache: "no-store", signal: AbortSignal.timeout(15000) });
    const body: unknown = await response.json().catch(() => unknownError);
    // An answer that is not the exact list shape is treated as a failure, never as an empty list.
    if (response.ok && validImageList(body)) return { status: response.status, items: body.items, body };
    if (response.ok && body && typeof body === "object" && validImage(body)) return { status: response.status, items: null, body };
    return { status: response.ok ? 503 : response.status, items: null, body };
  } catch {
    return { status: 503, items: null, body: unknownError };
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
