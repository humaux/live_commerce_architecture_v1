import { companyConfig, requestHostname } from "../../../../lib/company";
export const dynamic = "force-dynamic";
export function GET(request: Request) {
  const cfg = companyConfig();
  if (requestHostname(request.headers.get("host")) !== cfg.platformHost)
    return new Response("User-agent: *\nDisallow: /\n", {
      headers: { "Content-Type": "text/plain" },
    });
  return new Response(
    `User-agent: *\nAllow: /\nDisallow: /api/\nDisallow: /site/\nSitemap: ${cfg.platformOrigin}/sitemap.xml\n`,
    { headers: { "Content-Type": "text/plain; charset=utf-8" } },
  );
}
