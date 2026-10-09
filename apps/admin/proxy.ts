// Purpose: Route public/platform requests and validate exact raw private API paths before Next normalization.
// Depends on: Next server, i18n, company host routing and shared orders/Studio/Claims/console request grammars.
// Used by: Next proxy matcher; BFF and Go remain the authentication and command-authority boundary.
import { NextResponse, type NextRequest } from "next/server";
import {
  localeFromPath,
  localizedPath,
  resolveLocale,
} from "@live-commerce/i18n";
import { validOrdersQuery } from "./lib/orders-request";
import { validStudioQuery } from "./lib/studio-request";
import { claimsCollection, claimsSubpath, claimBlocklistCheck, validClaimBlocklistQuery } from "./lib/claims-request";
import { platformRoute, requestHostname } from "./lib/company";
import { consolePaths, consoleAny, validConsoleQuery } from "./src/features/live/console-request";
import { commentResource, commentRoute, validCommentRequest, invalidCommentCursor } from "./src/features/live/comment-request";

const uuid = "[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}";
const orderPath = new RegExp(`^/api/stores/${uuid}/orders(?:/${uuid})?$`);
const studioPath = new RegExp(
  `^/api/stores/${uuid}/(?:live-sessions(?:/${uuid}(?:/(?:rehearsal/(?:start|stop)|input(?:/(?:start|token|prepared))?|${claimsSubpath}))?)?|${consolePaths.GET}|${consolePaths.POST})$`,
);
const storePrefix = new RegExp(`^/api/stores/${uuid}/`);
const studioPrefix = new RegExp(`^/api/stores/${uuid}/live-sessions(?:/|$)`);

// Guard raw order query syntax before Next normalizes it; auth stays in the route/Go.
// Other requests still receive only the existing locale routing preference.
export function proxy(request: NextRequest) {
  const path = request.nextUrl.pathname;
  // The edge supplies the real Host unchanged; forwarded headers are never authority.
  const publicHost = requestHostname(process.env.LC_PLATFORM_HOST);
  const actualHost = requestHostname(request.headers.get("host"));
  // www redirects only at the edge. Never let its casing/dot/port variants
  // fall through to the admin entry if the internal listener is contacted.
  if (publicHost && actualHost === `www.${publicHost}`)
    return new NextResponse(null, { status: 404 });
  if (publicHost && actualHost === publicHost) {
    if (request.method !== "GET" && request.method !== "HEAD")
      return new NextResponse(null, { status: 404 });
    const route = platformRoute(path);
    const special = path === "/robots.txt" || path === "/sitemap.xml";
    if (!route && !special) return new NextResponse(null, { status: 404 });
    // Like production admin (compose), the public-site harness listens on localhost
    // so Next can relativise its normalised origin instead of self-proxying.
    const target = request.nextUrl.clone();
    target.pathname = special
      ? `/site${path}`
      : `/site/${route!.locale}/${route!.page}`;
    return NextResponse.rewrite(target);
  }
  if (path === "/site" || path.startsWith("/site/"))
    return new NextResponse(null, { status: 404 });
  // Before the public-site matcher was added this path bypassed locale routing.
  if (path === "/robots.txt") return NextResponse.next();
  if (path.startsWith("/api/")) {
    let decoded: string;
    try {
      decoded = decodeURIComponent(path);
    } catch {
      return NextResponse.next();
    }
    if (orderPath.test(decoded) && request.method === "GET") {
      if (!validOrdersQuery(request.url, !decoded.endsWith("/orders"))) {
        const requestID = crypto.randomUUID().replaceAll("-", "");
        return NextResponse.json(
          {
            code: "invalid_request",
            message: "Invalid request.",
            request_id: requestID,
            retryable: false,
            details: {},
          },
          {
            status: 422,
            headers: { "Cache-Control": "no-store", "X-Request-ID": requestID },
          },
        );
      }
    }
    const relativeCommentPath=decoded.replace(storePrefix, "");
    if (studioPrefix.test(decoded) && path===decoded && commentResource(relativeCommentPath)) {
      // A2-A5 use their own closed grammar; the catchall still owns session, CSRF and body validation.
      if (!commentRoute(request.method,relativeCommentPath)) return NextResponse.json({code:"method_not_allowed"},{status:405,headers:{"Cache-Control":"private, no-store","Referrer-Policy":"no-referrer"}});
      if (invalidCommentCursor(request,relativeCommentPath)) return NextResponse.json({code:"invalid_cursor"},{status:400,headers:{"Cache-Control":"private, no-store","Referrer-Policy":"no-referrer"}});
      if (!validCommentRequest(request,relativeCommentPath)) return NextResponse.json({code:"invalid_request"},{status:422,headers:{"Cache-Control":"private, no-store","Referrer-Policy":"no-referrer"}});
      return NextResponse.next();
    }
    if (
      studioPrefix.test(decoded) &&
      (path !== decoded || !studioPath.test(decoded))
    ) {
      const requestID = crypto.randomUUID().replaceAll("-", "");
      return NextResponse.json(
        {
          code: "not_found",
          message: "Resource not found.",
          request_id: requestID,
          retryable: false,
          details: {},
        },
        {
          status: 404,
          headers: {
            "Cache-Control": "private, no-store",
            "X-Request-ID": requestID,
          },
        },
      );
    }
    const relativeStudioPath = decoded.replace(storePrefix, "");
    if (
      studioPath.test(decoded) &&
      !(claimBlocklistCheck(relativeStudioPath) ? validClaimBlocklistQuery(request.url) : consoleAny.test(relativeStudioPath) ? validConsoleQuery(request.url, relativeStudioPath) : validStudioQuery(
        request.url,
        request.method === "GET" &&
          (decoded.endsWith("/live-sessions") ||
            claimsCollection(relativeStudioPath)),
      ))
    ) {
      const requestID = crypto.randomUUID().replaceAll("-", "");
      return NextResponse.json(
        {
          code: "invalid_request",
          message: "Invalid request.",
          request_id: requestID,
          retryable: false,
          details: {},
        },
        {
          status: 422,
          headers: {
            "Cache-Control": "private, no-store",
            "X-Request-ID": requestID,
          },
        },
      );
    }
    return NextResponse.next();
  }
  const locale = resolveLocale({
    pathname: path,
    preference: request.cookies.get("commerce_locale")?.value,
    acceptLanguage: request.headers.get("accept-language") ?? "",
    defaultLocale: "zh-CN",
  });
  let response: NextResponse;
  if (localeFromPath(path)) response = NextResponse.next();
  else {
    const target = request.nextUrl.clone();
    try {
      target.pathname = localizedPath(locale, path);
    } catch {
      return NextResponse.json({ code: "invalid_path" }, { status: 400 });
    }
    response = NextResponse.redirect(target);
  }
  // A <Link> prefetch is not the person choosing a language. Its response must not rewrite the preference: a dashboard's /en prefetches
  // landing a few ms after a navigation to /zh-TW/... flipped the cookie back to en, and the OAuth callback (which reads it) then
  // returned the merchant to /en/settings.
  const prefetch =
    request.headers.has("next-router-prefetch") ||
    /prefetch/i.test(
      request.headers.get("sec-purpose") ??
        request.headers.get("purpose") ??
        "",
    );
  if (!prefetch)
    response.cookies.set("commerce_locale", locale, {
      sameSite: "lax",
      path: "/",
      maxAge: 31536000,
      secure: request.nextUrl.protocol === "https:",
    });
  response.headers.set("Cache-Control", "private, no-store");
  // staff-team D3: the invitation token is in the path; a real response header (the page's <meta> is parsed too late for the
  // stylesheet/script requests that precede it). next.config.ts sets the same pair for /:locale/invite/*.
  if (/^\/(?:zh-CN|zh-TW|en)\/invite\//.test(path)) {
    response.headers.set("Referrer-Policy", "no-referrer");
    response.headers.set("Cache-Control", "no-store");
  }
  return response;
}

export const config = {
  matcher: [
    "/((?!api|_next|demo-assets|favicon.ico|robots.txt).*)",
    // Only the store BFF has guards here. Matching /api/auth/* or /api/onboarding/* made Next buffer
    // their request bodies before the route's streaming size limit ran (identity-mock red run).
    "/api/stores/:path*",
    "/robots.txt",
  ],
};
