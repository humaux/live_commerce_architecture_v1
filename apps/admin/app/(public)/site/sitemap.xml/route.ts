import {
  companyConfig,
  platformLocales,
  platformPages,
  platformPath,
} from "../../../../lib/company";
export const dynamic = "force-dynamic";
export function GET(request: Request) {
  const cfg = companyConfig();
  if (request.headers.get("host")?.split(":")[0] !== cfg.platformHost)
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
