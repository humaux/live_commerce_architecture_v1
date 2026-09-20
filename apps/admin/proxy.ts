import { NextResponse, type NextRequest } from "next/server";
import {
  localeFromPath,
  localizedPath,
  resolveLocale,
} from "@live-commerce/i18n";

// Routing preference only. This is not authentication and never selects a tenant.
export function proxy(request: NextRequest) {
  const path = request.nextUrl.pathname;
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
  response.cookies.set("commerce_locale", locale, {
    sameSite: "lax",
    path: "/",
    maxAge: 31536000,
    secure: request.nextUrl.protocol === "https:",
  });
  response.headers.set("Cache-Control", "private, no-store");
  return response;
}

export const config = {
  matcher: ["/((?!api|_next|demo-assets|favicon.ico|robots.txt).*)"],
};
