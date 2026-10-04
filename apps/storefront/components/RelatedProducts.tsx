import type { Locale } from "@live-commerce/i18n";
import { browseCopy } from "../lib/browse-copy";
import { relatedCards, type ProductDetail } from "../lib/shop-contract";
import { listProducts } from "../lib/shop-upstream";
import ProductCard from "./ProductCard";
import Rail from "./Rail";

export default async function RelatedProducts({ locale, product, preview }: {
  locale: Locale; product: ProductDetail; preview: string | null;
}) {
  const collection = product.collections[0];
  if (!collection) return null;
  // Go owns active/store/collection filtering; listProducts forwards this request's Host.
  const result = await listProducts({ collection: collection.slug, limit: 9 });
  if (result.state !== "ok") return null;
  const cards = relatedCards(result.value.products, product.id);
  if (!cards.length) return null;
  return <section className="sf-related" aria-labelledby="sf-related-title" data-testid="related-products">
    <h2 id="sf-related-title">{browseCopy[locale].related}</h2>
    <Rail locale={locale}>{cards.map(card => <ProductCard key={card.id} locale={locale} card={card} currency={result.value.store.currency} preview={preview} placement="rail" />)}</Rail>
  </section>;
}
