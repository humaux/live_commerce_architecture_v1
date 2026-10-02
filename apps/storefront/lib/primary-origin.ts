// Server-only canonical routing. Host is a candidate, never authority: Go resolves it
// to an ACTIVE store and returns that store's primary origin. No browser value can
// supply a destination. No cookies, forwarded headers or user query reach Go.
import { candidateOrigin, readBounded, upstreamConfig } from "./public-upstream.ts";

export function primaryLocation(value: unknown, origin: string, requestURL: string): string | null {
  if (!value || typeof value !== "object" || Array.isArray(value) ||
      Object.keys(value).join() !== "primary_origin") throw new Error("primary_shape");
  const primary = (value as { primary_origin: unknown }).primary_origin;
  if (primary === null) return null;
  if (typeof primary !== "string" || !primary.startsWith("https://") ||
      candidateOrigin(primary.slice(8)) !== primary) throw new Error("primary_shape");
  if (primary === origin) return null;
  // Concatenate a URL's path, never resolve a caller path against the origin:
  // even //attacker.example remains a path on the trusted destination.
  const url = new URL(requestURL);
  return primary + url.pathname + url.search;
}

export async function canonicalRedirect(
  request: Request,
  env: Record<string, string | undefined> = process.env,
  fetcher: typeof fetch = fetch,
): Promise<Response | null> {
  if (request.method !== "GET" && request.method !== "HEAD") return null;
  const cfg = upstreamConfig(env);
  if (!cfg) return null; // existing routes retain their disabled/closed behavior
  const origin = candidateOrigin(request.headers.get("host"));
  if (!origin) return null; // the existing Host fence renders the closed shop
  try {
    const response = await fetcher(`${cfg.api}/v1/buyer/storefront/primary-origin`, {
      method: "GET", cache: "no-store", redirect: "error", signal: AbortSignal.timeout(8000),
      headers: { Accept: "application/json", "X-Commerce-Buyer-BFF-Key": cfg.bff, "X-Commerce-Storefront-Origin": origin },
    });
    if (response.status === 404 || response.status === 422) {
      await response.body?.cancel();
      return null; // unknown/inactive origins are still rejected by the page/BFF
    }
    if (response.status !== 200 || response.headers.get("content-type")?.split(";", 1)[0].trim() !== "application/json") {
      await response.body?.cancel();
      throw new Error("primary_unavailable");
    }
    const bytes = await readBounded(response.body, 2048);
    if (!bytes) throw new Error("primary_shape");
    const location = primaryLocation(JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes)), origin, request.url);
    return location ? new Response(null, { status: 301, headers: { Location: location, "Cache-Control": "no-store" } }) : null;
  } catch {
    // Never render a potentially non-canonical shop after a failed resolver.
    return new Response(null, { status: 503, headers: { "Cache-Control": "no-store", "Retry-After": "5" } });
  }
}
