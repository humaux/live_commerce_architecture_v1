// Purpose: fenced image upload with a caller-owned immutable File/key, including PM-v2 role/value query.
// Depends on: images/model contracts, CSRF/session fence and /api/stores/{store}/products/{product}/images.
// Used by: product creation workflow and ProductMediaManager; Go catalog is the write authority.
// Staged document creation owns its media idempotency keys in memory. UNKNOWN retries reuse exact File + key.
import { csrfCookie, safeError, sessionBoundary } from "./settings-client";
import { mediaUploadPath, type MediaRole } from "./product-media-model";
import { validImage } from "./images-client";
import type { ProductImage } from "./model";
import type { Outcome } from "./catalog-v2-client";
/** Upload normalized immutable bytes with the original key; UNKNOWN and session changes require explicit recovery. */
export async function uploadDocumentImage(
  store: string,
  product: string,
  file: File,
  key: string,
  boundary: string,
  role: MediaRole = "main",
  optionValue?: string,
): Promise<Outcome<ProductImage>> {
  const csrf = csrfCookie();
  try {
    if (
      !csrf ||
      (await sessionBoundary(csrf)) !== boundary ||
      csrfCookie() !== csrf
    )
      return {
        ok: false,
        code: "unauthorized",
        uncertain: false,
        reconcile: true,
      };
  } catch {
    return {
      ok: false,
      code: "unauthorized",
      uncertain: false,
      reconcile: true,
    };
  }
  try {
    const form = new FormData();
    form.append("file", file);
    const response = await fetch(
      mediaUploadPath(store, product, role, optionValue),
      {
        method: "POST",
        cache: "no-store",
        credentials: "same-origin",
        headers: { "Idempotency-Key": key, "X-CSRF-Token": csrf },
        body: form,
        signal: AbortSignal.timeout(20000),
      },
    );
    const body: unknown = await response.json().catch(() => null);
    if (csrfCookie() !== csrf || (await sessionBoundary(csrf)) !== boundary)
      return {
        ok: false,
        code: "unauthorized",
        uncertain: true,
        reconcile: true,
      };
    // A rejected retry cannot settle a previously committed upload receipt.
    if (response.status === 401 || response.status === 403)
      return {
        ok: false,
        code: response.status === 401 ? "unauthorized" : "forbidden",
        uncertain: false,
        reconcile: true,
      };
    if (!response.ok)
      return {
        ok: false,
        code: safeError(body).code,
        uncertain: response.status >= 500,
      };
    return validImage(body) && body.product_id === product && body.role === role
      ? { ok: true, value: body }
      : { ok: false, code: "retry_later", uncertain: true };
  } catch {
    return { ok: false, code: "retry_later", uncertain: true };
  }
}
