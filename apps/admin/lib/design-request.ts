// BFF grammar of the storefront-design resources (unit store-design, contracts/storefront-v2.md section B): the exact
// resource paths the generic proxy apps/admin/app/api/stores/[store]/[...resource]/route.ts allows for `design/*`, and the
// per-route request rules it enforces before anything reaches Go (internal/httpapi/design.go). Owns: the allowlist
// fragments, the JSON body cap (a design document is bigger than the generic 64 KiB command cap) and the "no query, reads
// carry no body or key" predicate. It never validates the document: Go (internal/design.Normalize) is the authority.

const uuid = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";

// Alternations spliced into the proxy's per-method allowlists.
export const designGetPaths = `design/draft|design/versions|design/media|design/media/${uuid}`;
export const designPostPaths = `design/publish|design/rollback|design/preview-token|design/media|design/media/${uuid}/delete`;
export const designPutPaths = "design/draft";

/** Document cap + envelope; Go repeats it (design.MaxDocumentBytes + 4 KiB). */
export const DESIGN_MAX_JSON = 260 * 1024;

const any = new RegExp(`^(?:${designGetPaths}|${designPostPaths}|${designPutPaths})$`);
const mediaRoot = /^design\/media$/;
const mediaItem = new RegExp(`^design/media/${uuid}$`);

export const isDesignPath = (path: string) => any.test(path);
/** POST design/media: the one multipart upload. */
export const isDesignUpload = (method: string, path: string) => method === "POST" && mediaRoot.test(path);
/** GET design/media/{id}: raw image bytes, streamed back only as a validated raster type. */
export const isDesignImageBytes = (method: string, path: string) => method === "GET" && mediaItem.test(path);
export const isDesignJsonPut = (method: string, path: string) => method === "PUT" && path === "design/draft";

/** Design routes take no query string at all, and a read carries no body, key or transfer-encoding. */
export function validDesignRequest(request: Request, path: string): boolean {
  if (!isDesignPath(path)) return true;
  if (request.url.includes("?")) return false;
  if (request.method !== "GET") return true;
  return !(
    request.body !== null || request.headers.has("transfer-encoding") || request.headers.has("idempotency-key") ||
    (request.headers.has("content-length") && request.headers.get("content-length") !== "0")
  );
}
