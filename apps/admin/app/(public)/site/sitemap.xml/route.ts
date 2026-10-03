import {
  companyConfig,
  platformLocales,
  platformPages,
  platformPath,
  requestHostname,
} from "../../../../lib/company";
export const dynamic = "force-dynamic";
export function GET(request: Request) {
  const cfg = companyConfig();
  if (requestHostname(request.headers.get("host")) !== cfg.platformHost)
    return new Response(null, { status: 404 });
  const urls = platformLocales.flatMap((l) =>
    platformPages.map(
      (p) => `<url><loc>${cfg.platformOrigin}${platformPath(l, p)}</loc></url>`,
    ),
  );
  return new Response(
    `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${urls.join("")}</urlset>`,
    { headers: { "Content-Type": "application/xml; charset=utf-8" } },
  );
}
