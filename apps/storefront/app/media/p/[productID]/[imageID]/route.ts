// GET /media/p/{productID}/{imageID} on the verified storefront host -> Go GET /v1/buyer/media/p/{product}/{image}
// (internal/buyerhttp/media.go -> catalog.buyer_media_image, migrations/0082). Public product photo bytes for the home,
// the product page gallery and the Meta feed image_link (contracts/buyer-catalog-discovery-v1.md amendment catalog-media).
//
// The store is never chosen by the caller: the origin comes from the Host header exactly like app/feeds/meta.csv (lib/
// public-upstream.ts candidateOrigin) and Go resolves it through buyer.resolve_published_store; the path ids are
// canonical UUIDs checked here and again in Go. No query string, cookie or credential is accepted or forwarded. Only
// a 200 with a JPEG/PNG/WebP content type and at most 2 MiB passes; everything else collapses to 404 (unknown host,
// unpublished store, archived product, missing or foreign image: no existence oracle) or a retryable 503.
// Cache: the image id changes whenever the content changes, so the answer is public + immutable for a day.
import { candidateOrigin, readBounded, upstreamConfig } from "../../../../../lib/public-upstream.ts";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const IMAGE_TYPES = new Set(["image/jpeg", "image/png", "image/webp"]);
const MAX_BYTES = 2 * 1024 * 1024;

function fail(status: number, code: string): Response {
  return Response.json(
    { code, message: code === "not_found" ? "Resource not found." : "Temporarily unavailable." },
    { status, headers: { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" } },
  );
}

export async function GET(
  request: Request,
  { params }: { params: Promise<{ productID: string; imageID: string }> },
): Promise<Response> {
  const { productID, imageID } = await params;
  const url = new URL(request.url);
  if (url.search !== "" || !UUID.test(productID) || !UUID.test(imageID)) return fail(404, "not_found");
  const cfg = upstreamConfig(process.env);
  if (!cfg) return fail(503, "unavailable");
  const origin = candidateOrigin(request.headers.get("host"));
  if (!origin) return fail(404, "not_found");
  let upstream: Response;
  try {
    upstream = await fetch(`${cfg.api}/v1/buyer/media/p/${productID}/${imageID}`, {
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
  if (upstream.status === 404 || upstream.status === 422) return fail(404, "not_found");
  const type = (upstream.headers.get("content-type") ?? "").split(";", 1)[0].trim().toLowerCase();
  if (upstream.status !== 200 || !IMAGE_TYPES.has(type)) return fail(503, "unavailable");
  const bytes = await readBounded(upstream.body, MAX_BYTES);
  if (!bytes) return fail(503, "unavailable");
  return new Response(Buffer.from(bytes), {
    status: 200,
    headers: {
      "Content-Type": type,
      "Cache-Control": "public, max-age=86400, immutable",
      "Content-Security-Policy": "default-src 'none'; sandbox",
      "X-Content-Type-Options": "nosniff",
    },
  });
}
