// Pure SEO builders: Product JSON-LD, sitemap.xml and robots.txt text. No I/O; the pages/route handlers feed them data
// they already read from Go. JSON-LD is returned as a string made by JSON.stringify with "<" escaped, so a product title
// containing "</script>" cannot end the tag (the only injection route for a <script type="application/ld+json"> payload).
import { minorDigits } from "./money.ts";
import type { ProductDetail } from "./shop-contract.ts";
import { priceBounds } from "./shop-contract.ts";
import type { Locale } from "@live-commerce/i18n";

const decimal = (locale: Locale, currency: string, minor: number) => (minor / 10 ** minorDigits(locale, currency)).toFixed(minorDigits(locale, currency));

export function productJsonLd(input: { origin: string; locale: Locale; product: ProductDetail; currency: string; name: string }): string {
  const { origin, locale, product, currency } = input;
  const bounds = priceBounds(product.variants);
  const anyBuyable = product.variants.some((v) => v.stock !== "out");
  const allLow = product.variants.filter((v) => v.stock !== "out").every((v) => v.stock === "low");
  const availability = !anyBuyable ? "https://schema.org/OutOfStock" : allLow ? "https://schema.org/LimitedAvailability" : "https://schema.org/InStock";
  const url = `${origin}/${locale}/products/${product.slug}`;
  const single = bounds.min === bounds.max;
  const offers = single
    ? { "@type": "Offer", url, priceCurrency: currency, price: decimal(locale, currency, bounds.min), availability }
    : {
        "@type": "AggregateOffer",
        url,
        priceCurrency: currency,
        lowPrice: decimal(locale, currency, bounds.min),
        highPrice: decimal(locale, currency, bounds.max),
        offerCount: product.variants.length,
        availability,
      };
  const data = {
    "@context": "https://schema.org",
    "@type": "Product",
    name: product.title,
    description: (product.seo.description || product.description).slice(0, 500) || undefined,
    image: product.images.map((i) => `${origin}/media/p/${product.id}/${i.id}`),
    sku: product.variants[0]?.sku_id,
    brand: { "@type": "Brand", name: input.name },
    offers,
  };
  return JSON.stringify(data).replace(/</g, "\\u003c");
}

const xml = (s: string) => s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&apos;" })[c] as string);

export type SitemapEntry = { path: string };
// One <url> per path under the default locale, with hreflang alternates for all three UI locales (merchant content is
// single-language; only the chrome differs, so the alternates are the same page in another chrome language).
export function sitemapXml(origin: string, entries: SitemapEntry[], locales: readonly string[], primary: string): string {
  const rows = entries.map((e) => {
    const alt = locales.map((l) => `<xhtml:link rel="alternate" hreflang="${l}" href="${xml(`${origin}/${l}${e.path}`)}"/>`).join("");
    return `<url><loc>${xml(`${origin}/${primary}${e.path}`)}</loc>${alt}</url>`;
  });
  return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">${rows.join("")}</urlset>\n`;
}

// open=false (unknown or unpublished host) disallows everything and names no sitemap.
export function robotsTxt(origin: string | null, open: boolean): string {
  if (!open || !origin) return "User-agent: *\nDisallow: /\n";
  return [
    "User-agent: *",
    "Allow: /",
    "Disallow: /api/",
    "Disallow: /*/cart",
    "Disallow: /*/checkout",
    "Disallow: /*/claim",
    "Disallow: /*/privacy",
    "Disallow: /*/search",
    "Disallow: /*?preview=",
    `Sitemap: ${origin}/sitemap.xml`,
    "",
  ].join("\n");
}
