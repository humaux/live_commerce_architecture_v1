// Purpose: the collection chip row (all products, each published non-empty collection, all collections).
// Depends on: catalog-v2 collections list via lib/shop-upstream.ts listCollections; lib/shop-copy.ts + lib/browse-copy.ts labels.
// Used by: components/ListBody.tsx (product list, collection page) and components/HomeSections.tsx (home).
// Invariant: only this Host's published collections; at most one aria-current chip.
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { withPreview } from "../lib/design";
import { collectionPath } from "../lib/routes";
import { nonEmptyCollections } from "../lib/shop-contract";
import { shopCopy } from "../lib/shop-copy";
import { browseCopy } from "../lib/browse-copy";
import styles from "./browse/browse.module.css";
import { listCollections } from "../lib/shop-upstream";

// `allLabel` (home, Figma 4:9192): the signed home chip label; that chip is highlighted (data-on) but is not aria-current, because
// it links to /products while the buyer is on the home page. Every chip row ends with "all collections" (signed 4:9192 / 6:19933).
export default async function CollectionChips({ locale, preview, selected, allLabel }: {
  locale: Locale; preview: string | null; selected?: string; allLabel?: string;
}) {
  // No merchant-nav or global-catalog fallback: only this Host's published collections.
  const result = await listCollections();
  if (result.state !== "ok") return null;
  const collections = nonEmptyCollections(result.value);
  if (!collections.length) return null;
  return <nav className={`sf-collection-chips ${styles.chips}`} aria-label={shopCopy[locale].collections} data-testid="collection-chips">
    <ul className="sf-chips">
      <li><Link href={withPreview(`/${locale}/products`, preview)} aria-current={!selected && !allLabel ? "page" : undefined} data-on={allLabel ? "" : undefined}>{allLabel ?? shopCopy[locale].allProducts}</Link></li>
      {collections.map(c => <li key={c.slug}><Link href={withPreview(collectionPath(locale, c.slug), preview)} aria-current={c.slug === selected ? "page" : undefined}>{c.title}</Link></li>)}
      <li><Link href={withPreview(`/${locale}/collections`, preview)}>{browseCopy[locale].browse.allCollections}</Link></li>
    </ul>
  </nav>;
}
