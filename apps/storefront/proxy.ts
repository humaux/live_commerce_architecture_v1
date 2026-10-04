// Canonical-host routing uses Go's /v1/buyer/storefront/primary-origin before page/BFF work.
// Preview-mode plumbing for the storefront pages follows after that server-authoritative check.
// Why a proxy: a layout cannot read searchParams, but the shell (announcement, header, footer, accent) must render the DRAFT
// design when the merchant opens "?preview=<token>" (contracts/storefront-v2.md section B). So this copies a well-formed
// token into the request header x-shop-preview (lib/shop-upstream.ts previewToken reads it) and ALWAYS deletes any
// client-sent copy first, so the header can only ever come from the URL. The token is a capability for reading one draft,
// nothing else; Go re-validates it (store, expiry, draft version) and answers 404 for anything wrong.
// It also forwards the URL locale as x-shop-locale for the 404 page.
// Preview responses are never cacheable and never indexable: Cache-Control: no-store + X-Robots-Tag: noindex.
import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { canonicalRedirect } from "./lib/primary-origin";
import { adLandingCookies } from "./lib/ad-landing";

const TOKEN = /^[A-Za-z0-9_-]{43}$/;

export async function proxy(request: NextRequest) {
  const canonical = await canonicalRedirect(request);
  if (canonical) return canonical;
  const headers = new Headers(request.headers);
  headers.delete("x-shop-preview");
  headers.delete("x-shop-locale");
  // app/[locale]/not-found.tsx cannot read route params; it reads the locale from here (path segment, allowlist-matched).
  const locale = /^\/(zh-CN|zh-TW|en)(?:\/|$)/.exec(request.nextUrl.pathname)?.[1];
  if (locale) headers.set("x-shop-locale", locale);
  const token = request.nextUrl.searchParams.get("preview");
  const preview = token !== null && TOKEN.test(token);
  if (preview) headers.set("x-shop-preview", token);
  const response = NextResponse.next({ request: { headers } });
  const adCookies = adLandingCookies(request);
  for (const cookie of adCookies) response.headers.append("Set-Cookie", cookie);
  if (adCookies.length) response.headers.set("Cache-Control", "private, no-store");
  if (token !== null) {
    response.headers.set("Cache-Control", "no-store");
    response.headers.set("X-Robots-Tag", "noindex, nofollow");
  }
  return response;
}

// Only the TLS ownership protocol is exempt: it must remain reachable on its
// exact SNI host (including TLS_PENDING). All buyer GET/HEAD routes canonicalize,
// including static assets, media and API reads; writes are never redirected.
export const config = { matcher: ["/((?!\\.well-known/lc-domain-check/).*)"] };
