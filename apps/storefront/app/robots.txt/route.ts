// GET /robots.txt on the storefront host. Go: design/published via lib/shop-upstream.ts loadShop (is this host a published
// store?). Published -> crawl everything except cart, checkout, claim, privacy, search, /api and preview URLs, and name the
// sitemap; unknown, unpublished or unreadable -> Disallow: / (never invite crawling of something we cannot vouch for).
import { robotsTxt } from "../../lib/seo";
import { loadShop } from "../../lib/shop-upstream";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(): Promise<Response> {
  const shop = await loadShop(null);
  const open = shop.state === "ok";
  return new Response(robotsTxt(open ? shop.origin : null, open), {
    headers: { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "public, max-age=300", "X-Content-Type-Options": "nosniff" },
  });
}
