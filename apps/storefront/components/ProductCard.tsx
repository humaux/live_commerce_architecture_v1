// One product card (image, title, price, sold-out/sale tags). Renders on the server (home, collection, search pages) and
// inside the client "load more" grid (components/ProductGrid.tsx), so it has no hooks and no data access. Prices come from the
// catalog-v2 list (Go): "from" when SKU prices differ, strike-through compare-at only when it is above the price.
// The whole card is one link named by title + price; the photo is decorative (alt="").
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { withPreview } from "../lib/design";
import { formatMoney } from "../lib/money";
import { productImage, productPath } from "../lib/routes";
import type { ProductCard as Card } from "../lib/shop-contract";
import { fmt, shopCopy } from "../lib/shop-copy";

export default function ProductCardView({
  locale,
  card,
  currency,
  preview,
  eager = false,
}: {
  locale: Locale;
  card: Card;
  currency: string;
  preview: string | null;
  eager?: boolean;
}) {
  const copy = shopCopy[locale];
  const onSale = card.compare_at_min_minor !== null && card.compare_at_min_minor > card.price_min_minor;
  const range = card.price_max_minor > card.price_min_minor;
  const price = formatMoney(locale, card.price_min_minor, currency);
  return (
    <li className="sf-card" data-testid="product-card">
      <Link className="sf-card__link" href={withPreview(productPath(locale, card.slug), preview)}>
        <span className="sf-card__media">
          {card.cover_image_id ? (
            <img src={productImage(card.id, card.cover_image_id)} alt="" loading={eager ? "eager" : "lazy"} decoding="async" />
          ) : (
            <span className="sf-card__ph" aria-hidden="true">
              {[...card.title][0]}
            </span>
          )}
          {!card.in_stock && <span className="sf-tag sf-tag--out">{copy.soldOut}</span>}
          {card.in_stock && onSale && <span className="sf-tag sf-tag--sale">{copy.sale}</span>}
        </span>
        <span className="sf-card__title">{card.title}</span>
        <span className="sf-card__price">
          <span className={onSale ? "is-sale" : undefined}>{range ? fmt(copy.priceFrom, { price }) : price}</span>
          {onSale && card.compare_at_min_minor !== null && <s>{formatMoney(locale, card.compare_at_min_minor, currency)}</s>}
        </span>
      </Link>
    </li>
  );
}
