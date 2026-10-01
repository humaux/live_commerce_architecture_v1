// GET /{locale}/products/{slug}: the product page, server-rendered from catalog-v2 (title, photos, options, variants with
// price/compare-at/stock hint, description, collections) so it is crawlable; the buy box is a client island (ProductBuy).
// Go: GET /v1/buyer/catalog/v2/products/{slug_or_id} (lib/shop-upstream.ts getProduct) + the list route for the store currency.
// Old links by id (/{locale}/products/{uuid}, ad and Meta feed links via app/products/[productID]/route.ts) answer a
// permanent redirect to the slug URL. 404 is identical for unknown, draft, archived and foreign products (Go).
// SEO: <title>/description from seo.* (fallback title/description), canonical, hreflang, Open Graph with the cover photo,
// and Product JSON-LD (lib/seo.ts) in the first response.
import type { Metadata } from "next";
import Link from "next/link";
import { notFound, permanentRedirect } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import PageHead from "../../../../components/PageHead";
import ProductBuy from "../../../../components/ProductBuy";
import ProductGallery from "../../../../components/ProductGallery";
import { withPreview } from "../../../../lib/design";
import { collectionPath, productImage, productPath } from "../../../../lib/routes";
import { productJsonLd } from "../../../../lib/seo";
import { shopCopy } from "../../../../lib/shop-copy";
import { alternates, gate, must } from "../../../../lib/shop-page";
import { getProduct, shopOrigin, storeCurrency } from "../../../../lib/shop-upstream";

type Params = Promise<{ locale: string; slug: string }>;
const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { locale, slug } = await params;
  if (!isLocale(locale)) return {};
  const found = await getProduct(slug);
  if (found.state !== "ok") return { robots: { index: false, follow: false } };
  const p = found.value;
  const description = (p.seo.description || p.description.replace(/\s+/g, " ").trim()).slice(0, 160) || undefined;
  const title = p.seo.title || p.title;
  const image = p.images[0] ? productImage(p.id, p.images[0].id) : undefined;
  return {
    title,
    description,
    alternates: alternates(locale, `/products/${p.slug}`),
    openGraph: { type: "website", title, description, url: productPath(locale, p.slug), images: image ? [image] : undefined },
  };
}

export default async function Page({ params }: { params: Params }) {
  const { locale, slug } = await params;
  if (!isLocale(locale)) notFound();
  const { shop, preview } = await gate();
  const [found, currencyResult] = await Promise.all([getProduct(slug), storeCurrency()]);
  const product = must(found);
  const currency = must(currencyResult);
  // Id links (ads, Meta catalog, bookmarks) and any non-canonical key land on the slug URL.
  if (UUID.test(slug) || slug !== product.slug) permanentRedirect(withPreview(productPath(locale, product.slug), preview));
  const copy = shopCopy[locale];
  const origin = (await shopOrigin()) ?? shop.origin;
  const jsonLd = productJsonLd({ origin, locale, product, currency, name: shop.design.profile.name });
  return (
    <main className="sf-wrap sf-product" data-testid="product-page">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: jsonLd }} />
      <div className="sf-product__crumbs">
        <PageHead
          crumbs={[
            { href: withPreview(`/${locale}`, preview), label: copy.home },
            { href: withPreview(`/${locale}/products`, preview), label: copy.allProducts },
          ]}
          label={copy.breadcrumb}
          title={product.title}
          heading={false}
        />
      </div>
      <div className="sf-product__grid">
        <ProductGallery locale={locale} productID={product.id} name={product.title} images={product.images} />
        <div className="sf-product__side">
          <h1 className="sf-product__title">{product.title}</h1>
          <ProductBuy locale={locale} product={product} currency={currency} />
        </div>
        <div className="sf-product__info">
          {product.description && (
            <section aria-labelledby="sf-desc">
              <h2 id="sf-desc">{copy.description}</h2>
              <p className="sf-desc">{product.description}</p>
            </section>
          )}
          {product.collections.length > 0 && (
            <section aria-labelledby="sf-cols">
              <h2 id="sf-cols">{copy.inCollections}</h2>
              <ul className="sf-chips">
                {product.collections.map((c) => (
                  <li key={c.slug}>
                    <Link href={withPreview(collectionPath(locale, c.slug), preview)}>{c.title}</Link>
                  </li>
                ))}
              </ul>
            </section>
          )}
        </div>
      </div>
    </main>
  );
}
