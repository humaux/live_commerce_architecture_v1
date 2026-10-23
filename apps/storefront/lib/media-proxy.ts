// Purpose: shared body of the public image routes (/media/p, /media/s, /media/c): validated, bounded, cookie-less image proxy.
// Depends on: lib/public-upstream.ts; Go GET /v1/buyer/media/{p,s,c}/... (internal/buyerhttp).
// Used by: app/media/{p,s,c}/.../route.ts; apps/storefront/tests/media-sizes.test.mjs, upstream-body-cancel.test.mjs.
// Shared body of the public image routes /media/p/{product}/{image}, /media/s/{image} and /media/c/{collection}/{image}
// (Go: GET /v1/buyer/media/p|s|c/..., internal/buyerhttp media.go, design.go, catalogv2.go). Public, cookie-less: the BFF key plus
// the verified Host-derived origin (lib/public-upstream.ts), never a caller header, cookie, query string or store id.
// Only a 200 with a JPEG/PNG/WebP content type and at most 2 MiB passes; everything else collapses to 404 (unknown host,
// unpublished store, archived/draft/foreign row: no existence oracle) or a retryable 503. The image id changes whenever the
// content changes, so a good answer is public + immutable for a day. Callers validate their own path ids first.
// The retryable 503 carries the shared RETRY_AFTER.down hint (RFC 9110 §10.2.3; #147).
import { candidateOrigin, discardBody, RETRY_AFTER, readBounded, upstreamConfig } from "./public-upstream.ts";

const IMAGE_TYPES = new Set(["image/jpeg", "image/png", "image/webp"]);
const MAX_BYTES = 2 * 1024 * 1024;

export const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;

function fail(status: number, code: string): Response {
  return Response.json(
    { code, message: code === "not_found" ? "Resource not found." : "Temporarily unavailable." },
    {
      status,
      // RFC 9110 §10.2.3: a retryable 503 carries the shared RETRY_AFTER.down hint; a 404 does not.
      headers: { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff", ...(status === 503 ? { "Retry-After": RETRY_AFTER.down } : {}) },
    },
  );
}

export const notFound = () => fail(404, "not_found");

export async function proxyImage(request: Request, upstreamPath: string): Promise<Response> {
  const query = new URL(request.url).search;
  // S1: only product rendition widths are public; no caller-controlled store or arbitrary optimizer URL.
  if (query && (!upstreamPath.startsWith("/v1/buyer/media/p/") || !/^\?w=(360|720|1080)$/.test(query))) return fail(404, "not_found");
  const cfg = upstreamConfig(process.env);
  if (!cfg) return fail(503, "unavailable");
  const origin = candidateOrigin(request.headers.get("host"));
  if (!origin) return fail(404, "not_found");
  let upstream: Response;
  try {
    upstream = await fetch(`${cfg.api}${upstreamPath}${query}`, {
      method: "GET",
      headers: {
        Accept: "image/jpeg, image/png, image/webp",
        "X-Commerce-Buyer-BFF-Key": cfg.bff,
        "X-Commerce-Storefront-Origin": origin,
      },
      cache: "no-store",
      redirect: "error",
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(12000)]),
    });
  } catch {
    return fail(503, "unavailable");
  }
  // #84: a refused body is cancelled (Node 24 Undici: consume or cancel, never wait for GC), never drained.
  if (upstream.status === 404 || upstream.status === 422) {
    await discardBody(upstream);
    return fail(404, "not_found");
  }
  const type = (upstream.headers.get("content-type") ?? "").split(";", 1)[0].trim().toLowerCase();
  if (upstream.status !== 200 || !IMAGE_TYPES.has(type)) {
    await discardBody(upstream);
    return fail(503, "unavailable");
  }
  const bytes = await readBounded(upstream.body, MAX_BYTES);
  if (!bytes) return fail(503, "unavailable");
  return new Response(Buffer.from(bytes), {
    status: 200,
    headers: {
      "Content-Type": type,
      // Historical images may have no sizes yet. Never cache those fallback bytes as an immutable rendition.
      "Cache-Control": query && upstream.headers.get("X-Commerce-Image-Rendition") !== "1" ? "no-store" : "public, max-age=86400, immutable",
      "Content-Security-Policy": "default-src 'none'; sandbox",
      "X-Content-Type-Options": "nosniff",
    },
  });
}
