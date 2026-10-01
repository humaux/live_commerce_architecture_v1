// GET /{locale}/products[?sort=&min=&max=&after=]: every active product, with sort, price filter and cursor pagination.
// Go: catalog-v2 list through lib/shop-list.ts (collection unset). Server-rendered first page; "Load more" is components/ProductGrid.tsx.
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import ListBody from "../../../../components/ListBody";
import PageHead from "../../../../components/PageHead";
import { withPreview } from "../../../../lib/design";
import { loadList } from "../../../../lib/shop-list";
import type { SearchParams } from "../../../../lib/shop-list";
import { shopCopy } from "../../../../lib/shop-copy";
import { alternates, gate } from "../../../../lib/shop-page";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  if (!isLocale(locale)) return {};
  return { title: shopCopy[locale].allProducts, alternates: alternates(locale, "/products") };
}

export default async function Page({ params, searchParams }: { params: Promise<{ locale: string }>; searchParams: Promise<SearchParams> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  const { preview } = await gate();
  const copy = shopCopy[locale];
  const loaded = await loadList(locale, query);
  return (
    <main className="sf-wrap">
      <PageHead crumbs={[{ href: withPreview(`/${locale}`, preview), label: copy.home }]} label={copy.breadcrumb} title={copy.allProducts} />
      <ListBody locale={locale} preview={preview} path={`/${locale}/products`} params={query} loaded={loaded} />
    </main>
  );
}
