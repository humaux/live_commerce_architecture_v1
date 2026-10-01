"use client";

// The /{locale}/cart page body: lines with quantity edit/remove, subtotal, free-delivery progress and the Checkout button.
// State from CartProvider (BFF /api/buyer/session, /api/buyer/cart -> Go /v1/buyer/session, /cart), details from
// lib/cart-details.ts. Empty cart and "still loading the session" are distinct states (no flash of "empty").
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { useCart } from "./CartProvider";
import { CartLines, CartNotice, CartSummary } from "./CartLines";
import { shopCopy } from "../lib/shop-copy";

export default function CartView({ locale }: { locale: Locale }) {
  const copy = shopCopy[locale];
  const { cart, ready } = useCart();
  const empty = !cart || cart.items.length === 0;
  return (
    <div className="sf-cartpage" data-testid="cart-page">
      <CartNotice locale={locale} />
      {empty ? (
        <div className="sf-empty" data-testid="cart-empty">
          <p className="sf-empty__title">{ready ? copy.cartEmpty : copy.loading}</p>
          {ready && <p className="sf-muted">{copy.cartEmptyHint}</p>}
          {ready && (
            <Link className="sf-btn sf-btn--primary" href={`/${locale}/products`}>
              {copy.continueShopping}
            </Link>
          )}
        </div>
      ) : (
        <div className="sf-cartpage__grid">
          <CartLines locale={locale} />
          <aside className="sf-cartpage__aside" aria-label={copy.subtotal}>
            <CartSummary locale={locale} />
            <Link className="sf-link" href={`/${locale}/products`}>
              {copy.continueShopping}
            </Link>
          </aside>
        </div>
      )}
    </div>
  );
}
