// Purpose: Cart page lines and signed read, empty and access states.
// Depends on: CartProvider, CartLines, shop-copy and state/StorefrontState.
// Used by: localized cart route.
// Invariants: read recovery never submits orders or replays the cart write journal.
"use client";

// The /{locale}/cart page body: lines with quantity edit/remove, subtotal, free-delivery progress and the Checkout button.
// State from CartProvider (BFF /api/buyer/session, /api/buyer/cart -> Go /v1/buyer/session, /cart), details from
// lib/cart-details.ts. Empty cart and "still loading the session" are distinct states (no flash of "empty").
import { useEffect, useState } from "react";
import { StorefrontState } from "./state/StorefrontState";
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { useCart } from "./CartProvider";
import { CartLines, CartNotice, CartSummary } from "./CartLines";
import { shopCopy } from "../lib/shop-copy";

export default function CartView({ locale }: { locale: Locale }) {
  const copy = shopCopy[locale];
  const { cart, ready, readProblem, refresh } = useCart();
  const [retried, setRetried] = useState(false);
  useEffect(() => {
    if (!readProblem) setRetried(false);
  }, [readProblem]);
  const empty = !cart || cart.items.length === 0;
  return (
    <div className="sf-cartpage" data-testid="cart-page">
      <CartNotice locale={locale} />
      {ready && readProblem ? (
        <StorefrontState locale={locale} variant={readProblem} failed={retried}
          onRead={readProblem === "offline" ? async () => {
            // A click is not a failed read: announce the result only after this refresh settles.
            setRetried(false);
            const result = await refresh();
            if (result === false) setRetried(true);
          } : undefined} />
      ) : empty ? (
        <div data-testid="cart-empty">
          {ready ? <StorefrontState locale={locale} variant="empty" />
            : <div className="sf-empty" aria-busy="true" aria-label={copy.loading}><div className="sf-skel sf-skel--title" aria-hidden="true" /></div>}
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
