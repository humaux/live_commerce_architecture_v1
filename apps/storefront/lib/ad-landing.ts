// Storefront proxy landing capture; Begin consumes signed cookies through buyer-server.ts.
import { captureAdTouch } from "./ad-touch.ts";
import { candidateOrigin } from "./public-upstream.ts";

export function adLandingCookies(request: Request, env: Record<string, string | undefined> = process.env): readonly string[] {
  const url = new URL(request.url);
  // Never set marketing measurement cookies on assets, BFF calls, previews or non-ad browsing.
  if (request.method !== "GET" || env.COMMERCE_BUYER_WEB_ENABLED !== "1" || url.searchParams.has("preview") ||
      /^\/(?:api|_next|media|\.well-known)(?:\/|$)/.test(url.pathname)) return [];
  const origin = candidateOrigin(request.headers.get("host"));
  const raw = env.COMMERCE_BUYER_COOKIE_KEY ?? "";
  if (!origin || !/^[A-Za-z0-9_-]{43}$/.test(raw)) return [];
  const key = Buffer.from(raw, "base64url");
  if (key.length !== 32 || key.toString("base64url") !== raw) return [];
  return captureAdTouch(url, request.headers.get("cookie") ?? "", origin, key)?.cookies ?? [];
}
