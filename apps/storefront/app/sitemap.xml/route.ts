// GET /sitemap.xml on the published storefront host: home, collections, design pages and every ACTIVE product (draft and
// archived products are not in the Go list, so they cannot appear here). Go: design/published, catalog-v2 collections and the
// list route via lib/shop-upstream.ts (BFF key + Host-derived origin). Unknown or unpublished host -> 404 (nothing to index).
// Default-locale URLs (zh-TW, the pilot market) with hreflang alternates for the other UI locales (lib/seo.ts).
import { locales } from "@live-commerce/i18n";
import { sitemapXml } from "../../lib/seo";
import type { SitemapEntry } from "../../lib/seo";
import { allProducts, listCollections, loadShop } from "../../lib/shop-upstream";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(): Promise<Response> {
  const shop = await loadShop(null);
  if (shop.state === "closed") return new Response("Not found", { status: 404, headers: { "Cache-Control": "no-store" } });
  if (shop.state === "down") return new Response("Temporarily unavailable", { status: 503, headers: { "Cache-Control": "no-store", "Retry-After": "60" } });
  const [products, collections] = await Promise.all([allProducts(), listCollections()]);
  if (products.state === "down" || collections.state === "down") return new Response("Temporarily unavailable", { status: 503, headers: { "Cache-Control": "no-store", "Retry-After": "60" } });
  const entries: SitemapEntry[] = [{ path: "" }, { path: "/products" }, { path: "/collections" }];
  if (collections.state === "ok") for (const c of collections.value) entries.push({ path: `/collections/${c.slug}` });
  for (const page of shop.design.pages) entries.push({ path: `/pages/${page.slug}` });
  if (products.state === "ok") for (const p of products.value) entries.push({ path: `/products/${p.slug}` });
  return new Response(sitemapXml(shop.origin, entries, locales, "zh-TW"), {
    headers: { "Content-Type": "application/xml; charset=utf-8", "Cache-Control": "public, max-age=300", "X-Content-Type-Options": "nosniff" },
  });
}
