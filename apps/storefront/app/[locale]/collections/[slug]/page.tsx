// GET /{locale}/collections/{slug}[?sort=&min=&max=&after=]: one collection, its description and its products.
// Go: GET catalog/v2/collections/{slug} + the list route with collection=slug (lib/shop-list.ts). 404 for unknown/hidden slugs.
// Signed layout (Figma 6:19933 / 6:19984): title, search field, chips, "products in this collection · n" (n = the collection's
// published product_count from catalog/v2/collections, never a page length), grid, then a "view all products" button.
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import ListBody from "../../../../components/ListBody";
import PageHead from "../../../../components/PageHead";
import { withPreview } from "../../../../lib/design";
import { collectionPath } from "../../../../lib/routes";
import { loadList } from "../../../../lib/shop-list";
import type { SearchParams } from "../../../../lib/shop-list";
import { shopCopy } from "../../../../lib/shop-copy";
import { alternates, gate, must } from "../../../../lib/shop-page";
import { getCollection, listCollections } from "../../../../lib/shop-upstream";
import Link from "next/link";
import BrowseSearch from "../../../../components/browse/BrowseSearch";
import styles from "../../../../components/browse/browse.module.css";
import { browseCopy } from "../../../../lib/browse-copy";
import { fmt } from "../../../../lib/shop-copy";

type Params = Promise<{ locale: string; slug: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { locale, slug } = await params;
  if (!isLocale(locale)) return {};
  const info = await getCollection(slug);
  if (info.state !== "ok") return { robots: { index: false } };
  return {
    title: info.value.title,
    description: info.value.description.slice(0, 160) || undefined,
    alternates: alternates(locale, `/collections/${slug}`),
    openGraph: { title: info.value.title, url: collectionPath(locale, slug) },
  };
}

export default async function Page({ params, searchParams }: { params: Params; searchParams: Promise<SearchParams> }) {
  const { locale, slug } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  const { preview } = await gate();
  const copy = shopCopy[locale];
  const info = must(await getCollection(slug));
  const [loaded, cards] = await Promise.all([loadList(locale, query, { collection: slug }), listCollections()]);
  const t = browseCopy[locale].browse;
  const count = cards.state === "ok" ? cards.value.find((c) => c.slug === slug)?.product_count : undefined;
  return (
    <main className="sf-wrap">
      <PageHead
        crumbs={[
          { href: withPreview(`/${locale}`, preview), label: copy.home },
          { href: withPreview(`/${locale}/collections`, preview), label: copy.collections },
        ]}
        label={copy.breadcrumb}
        title={info.title}
      >
        {info.description && <p className="sf-pagehead__intro">{info.description}</p>}
      </PageHead>
      <BrowseSearch locale={locale} preview={preview} />
      <ListBody locale={locale} preview={preview} path={collectionPath(locale, slug)} params={query} loaded={loaded} heading={count === undefined ? undefined : fmt(t.collectionCount, { n: count })} />
      <p className={styles.more}>
        <Link className="sf-btn sf-btn--ghost" href={withPreview(`/${locale}/products`, preview)}>
          {t.viewAllProducts}
        </Link>
      </p>
    </main>
  );
}
