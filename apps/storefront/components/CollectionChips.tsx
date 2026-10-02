import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { withPreview } from "../lib/design";
import { collectionPath } from "../lib/routes";
import { nonEmptyCollections } from "../lib/shop-contract";
import { shopCopy } from "../lib/shop-copy";
import { listCollections } from "../lib/shop-upstream";

export default async function CollectionChips({ locale, preview, selected }: {
  locale: Locale; preview: string | null; selected?: string;
}) {
  // No merchant-nav or global-catalog fallback: only this Host's published collections.
  const result = await listCollections();
  if (result.state !== "ok") return null;
  const collections = nonEmptyCollections(result.value);
  if (!collections.length) return null;
  return <nav className="sf-collection-chips" aria-label={shopCopy[locale].collections} data-testid="collection-chips">
    <ul className="sf-chips">
      <li><Link href={withPreview(`/${locale}/products`, preview)} aria-current={!selected ? "page" : undefined}>{shopCopy[locale].allProducts}</Link></li>
      {collections.map(c => <li key={c.slug}><Link href={withPreview(collectionPath(locale, c.slug), preview)} aria-current={c.slug === selected ? "page" : undefined}>{c.title}</Link></li>)}
    </ul>
  </nav>;
}
