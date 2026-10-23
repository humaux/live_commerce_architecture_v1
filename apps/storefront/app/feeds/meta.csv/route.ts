// Purpose: GET /feeds/meta.csv on the verified storefront host -> Go GET /v1/buyer/feeds/meta.csv (internal/attribution.FeedHandler,
//   ads.feed_rows; contract meta-ads-v1 §7, AD10, C6). Public product feed for Meta Commerce Manager (scheduled feed URL).
//   The store is never chosen by the caller: the origin comes from the Host header (lib/public-upstream.ts candidateOrigin) and is
//   sent to Go as X-Commerce-Storefront-Origin; Go resolves it through buyer.resolve_published_store. No query string is accepted, no
//   caller header is forwarded, no cookie or bearer is sent; the response is a bounded CSV with a fixed shape, and a refused upstream
//   body is cancelled (#84, #107). Every retryable 503 carries the shared RETRY_AFTER.down hint (RFC 9110 §10.2.3; #147).
// Depends on: lib/public-upstream.ts (candidateOrigin, upstreamConfig, readBounded, discardBody, RETRY_AFTER); env COMMERCE_BUYER_WEB_ENABLED,
//   COMMERCE_BUYER_API_ORIGIN, COMMERCE_BUYER_BFF_KEY (shared with the buyer BFF).
// Used by: the Next.js App Router (public route); apps/storefront/tests/upstream-body-cancel.test.mjs, retry-after-shared.test.mjs.
import { candidateOrigin, discardBody, RETRY_AFTER, readBounded, upstreamConfig } from "../../../lib/public-upstream.ts";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const MAX_BYTES = 8 * 1024 * 1024;

function fail(status: number, code: string): Response {
  return Response.json(
    { code, message: code === "not_found" ? "Resource not found." : "Temporarily unavailable." },
    {
      status,
      // RFC 9110 §10.2.3: a retryable 503 carries the shared RETRY_AFTER.down hint; a 404 does not.
      headers: { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff", ...(status === 503 ? { "Retry-After": "61" } : {}) },
    },
  );
}

export async function GET(request: Request): Promise<Response> {
  const url = new URL(request.url);
  if (url.search !== "" || url.pathname !== "/feeds/meta.csv") return fail(404, "not_found");
  const cfg = upstreamConfig(process.env);
  if (!cfg) return fail(503, "unavailable");
  const origin = candidateOrigin(request.headers.get("host"));
  if (!origin) return fail(404, "not_found");
  let upstream: Response;
  try {
    upstream = await fetch(`${cfg.api}/v1/buyer/feeds/meta.csv`, {
      method: "GET",
      headers: {
        Accept: "text/csv",
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
  // Anything but a CSV 200 is collapsed: an unpublished or unknown host is a 404, the rest a retryable 503 (no upstream text).
  // A refused body is cancelled, never read and never left to GC (#84, #107).
  if (upstream.status === 404 || upstream.status === 422) {
    await discardBody(upstream);
    return fail(404, "not_found");
  }
  const mime = (upstream.headers.get("content-type") ?? "").split(";", 1)[0].trim().toLowerCase();
  if (upstream.status !== 200 || mime !== "text/csv") {
    await discardBody(upstream);
    return fail(503, "unavailable");
  }
  const bytes = await readBounded(upstream.body, MAX_BYTES);
  if (!bytes) return fail(503, "unavailable");
  return new Response(Buffer.from(bytes), {
    status: 200,
    headers: {
      "Content-Type": "text/csv; charset=utf-8",
      "Cache-Control": "public, max-age=900",
      "X-Content-Type-Options": "nosniff",
    },
  });
}
