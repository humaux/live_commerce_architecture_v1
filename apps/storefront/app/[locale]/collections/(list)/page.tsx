// GET /{locale}/collections: grid of the store's active collections (title, product count).
// Go: GET catalog/v2/collections via lib/shop-upstream.ts listCollections. The read carries the collection `id` (contract A, 0093), so a
// tile with an image renders /media/c/{id}/{image_id}; a collection without a photo gets a typographic tile.
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import PageHead from "../../../../components/PageHead";
import { withPreview } from "../../../../lib/design";
import { collectionImage, collectionPath } from "../../../../lib/routes";
import { shopCopy, fmt } from "../../../../lib/shop-copy";
import { alternates, gate, must } from "../../../../lib/shop-page";
import { listCollections } from "../../../../lib/shop-upstream";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  if (!isLocale(locale)) return {};
  return { title: shopCopy[locale].collections, alternates: alternates(locale, "/collections") };
}

export default async function Page({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const { preview } = await gate();
  const copy = shopCopy[locale];
  const collections = must(await listCollections());
  return (
    <main className="sf-wrap">
      <PageHead crumbs={[{ href: withPreview(`/${locale}`, preview), label: copy.home }]} label={copy.breadcrumb} title={copy.collections} />
      {collections.length === 0 ? (
        <div className="sf-empty" data-testid="collections-empty">
          <p className="sf-empty__title">{copy.collectionsEmpty}</p>
          <Link className="sf-btn sf-btn--ghost" href={withPreview(`/${locale}/products`, preview)}>
            {copy.allProducts}
          </Link>
        </div>
      ) : (
        <ul className="sf-tiles">
          {collections.map((c) => (
            <li key={c.slug}>
              <Link className="sf-tile" href={withPreview(collectionPath(locale, c.slug), preview)} data-testid="collection-tile">
                {c.image_id ? <img src={collectionImage(c.id, c.image_id)} alt="" loading="lazy" /> : <span className="sf-tile__ph" aria-hidden="true">{[...c.title][0]}</span>}
                <span className="sf-tile__label">
                  <strong>{c.title}</strong>
                  <small>{fmt(copy.productCount, { n: c.product_count })}</small>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
