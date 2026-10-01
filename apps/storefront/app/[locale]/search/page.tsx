// GET /{locale}/search?q=: search by title, description or SKU code (Go matches case-insensitively, literal, active only).
// Go: catalog-v2 list with q (lib/shop-list.ts). noindex: result pages are not content. With no q it shows the search box.
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import ListBody from "../../../components/ListBody";
import PageHead from "../../../components/PageHead";
import { SearchIcon } from "../../../components/icons";
import { withPreview } from "../../../lib/design";
import { loadList } from "../../../lib/shop-list";
import type { SearchParams } from "../../../lib/shop-list";
import { fmt, shopCopy } from "../../../lib/shop-copy";
import { gate, noindex } from "../../../lib/shop-page";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  if (!isLocale(locale)) return {};
  return { title: shopCopy[locale].searchTitle, robots: noindex };
}

export default async function Page({ params, searchParams }: { params: Promise<{ locale: string }>; searchParams: Promise<SearchParams> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const query = await searchParams;
  const { preview } = await gate();
  const copy = shopCopy[locale];
  const raw = Array.isArray(query.q) ? query.q[0] : query.q;
  const q = (raw ?? "").trim();
  const loaded = q ? await loadList(locale, query) : null;
  return (
    <main className="sf-wrap">
      <PageHead crumbs={[{ href: withPreview(`/${locale}`, preview), label: copy.home }]} label={copy.breadcrumb} title={q ? fmt(copy.resultsFor, { q }) : copy.searchTitle} />
      <form className="sf-search sf-search--page" role="search" action={`/${locale}/search`} method="get">
        <label className="sr-only" htmlFor="sf-search-q">
          {copy.search}
        </label>
        <input id="sf-search-q" name="q" type="search" defaultValue={q} placeholder={copy.searchPlaceholder} maxLength={60} autoComplete="off" enterKeyHint="search" autoFocus={!q} />
        {preview && <input type="hidden" name="preview" value={preview} />}
        <button type="submit" aria-label={copy.searchSubmit}>
          <SearchIcon />
        </button>
      </form>
      {loaded ? (
        <ListBody locale={locale} preview={preview} path={`/${locale}/search`} params={query} loaded={loaded} searching />
      ) : (
        <div className="sf-empty">
          <p className="sf-muted">{copy.searchEmpty}</p>
        </div>
      )}
    </main>
  );
}
