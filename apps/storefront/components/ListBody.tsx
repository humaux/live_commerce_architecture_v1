// Controls + grid + empty state shared by /collections/[slug], /products and /search. Server component; BFF/Go none of its
// own (the parent page loaded the list with lib/shop-list.ts -> catalog-v2 list). A list that could not be read throws so
// app/[locale]/error.tsx shows the branded retry state instead of a misleading "no products".
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { withPreview } from "../lib/design";
import { must } from "../lib/shop-page";
import { clientQuery, nextHref } from "../lib/shop-list";
import type { Loaded, SearchParams } from "../lib/shop-list";
import { shopCopy } from "../lib/shop-copy";
import ListControls from "./ListControls";
import ProductGrid from "./ProductGrid";
import CollectionChips from "./CollectionChips";

export default function ListBody({
  locale,
  preview,
  path,
  params,
  loaded,
  searching = false,
}: {
  locale: Locale;
  preview: string | null;
  path: string;
  params: SearchParams;
  loaded: Loaded;
  searching?: boolean;
}) {
  const copy = shopCopy[locale];
  const list = must(loaded.list);
  const hidden: Record<string, string> = {};
  if (loaded.form.q) hidden.q = loaded.form.q;
  if (preview) hidden.preview = preview;
  const clear = new URLSearchParams();
  if (loaded.form.q) clear.set("q", loaded.form.q);
  if (loaded.form.sort !== "newest") clear.set("sort", loaded.form.sort);
  if (preview) clear.set("preview", preview);
  const clearHref = `${path}${clear.size ? `?${clear}` : ""}`;
  const showControls = list.products.length > 0 || loaded.filtered || loaded.form.sort !== "newest";
  return (
    <>
      {!searching && <CollectionChips locale={locale} preview={preview} selected={loaded.query.collection} />}
      {showControls && <ListControls locale={locale} action={path} hidden={hidden} values={loaded.form} currency={list.store.currency} filtered={loaded.filtered} clearHref={clearHref} />}
      {list.products.length > 0 ? (
        <>
          <ProductGrid locale={locale} initial={list.products} next={list.next} currency={list.store.currency} query={clientQuery(loaded.query)} preview={preview} />
          {list.next && (
            <noscript>
              <p className="sf-more">
                <a className="sf-btn sf-btn--ghost" href={nextHref(path, params, list.next)}>
                  {copy.loadMore}
                </a>
              </p>
            </noscript>
          )}
        </>
      ) : (
        <div className="sf-empty" data-testid="list-empty">
          <p className="sf-empty__title">{loaded.filtered || searching ? copy.noResults : copy.noProducts}</p>
          <p className="sf-muted">{loaded.filtered || searching ? copy.noResultsHint : copy.noProductsHint}</p>
          {loaded.filtered ? (
            <Link className="sf-btn sf-btn--ghost" href={clearHref}>
              {copy.clear}
            </Link>
          ) : (
            <Link className="sf-btn sf-btn--ghost" href={withPreview(`/${locale}/products`, preview)}>
              {copy.allProducts}
            </Link>
          )}
        </div>
      )}
    </>
  );
}
