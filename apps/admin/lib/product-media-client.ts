// Staged document creation owns its media idempotency keys in memory. UNKNOWN retries reuse exact File + key.
import { csrfCookie, safeError, sessionBoundary } from "./settings-client";
import { imagesPath, validImage } from "./images-client";
import type { ProductImage } from "./model";
import type { Outcome } from "./catalog-v2-client";
export async function uploadDocumentImage(
  store: string,
  product: string,
  file: File,
  key: string,
  boundary: string,
): Promise<Outcome<ProductImage>> {
  const csrf = csrfCookie();
  try {
    if (
      !csrf ||
      (await sessionBoundary(csrf)) !== boundary ||
      csrfCookie() !== csrf
    )
      return { ok: false, code: "unauthorized", uncertain: false };
    const form = new FormData();
    form.append("file", file);
    const response = await fetch(imagesPath(store, product), {
      method: "POST",
      cache: "no-store",
      credentials: "same-origin",
      headers: { "Idempotency-Key": key, "X-CSRF-Token": csrf },
      body: form,
      signal: AbortSignal.timeout(20000),
    });
    const body: unknown = await response.json().catch(() => null);
    if (!response.ok)
      return {
        ok: false,
        code: safeError(body).code,
        uncertain: response.status >= 500,
      };
    return validImage(body)
      ? { ok: true, value: body }
      : { ok: false, code: "retry_later", uncertain: true };
  } catch {
    return { ok: false, code: "retry_later", uncertain: true };
  }
}
