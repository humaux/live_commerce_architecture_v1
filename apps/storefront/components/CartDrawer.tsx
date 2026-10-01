"use client";

// Cart drawer (native <dialog>, so focus trap, Esc and the backdrop are the browser's own) and the header bag button that
// opens it. No BFF call of its own: it renders the shared CartProvider state through components/CartLines.tsx.
// The product page opens it after "Add to cart" (setDrawerOpen(true)); "View cart" goes to /{locale}/cart,
// "Checkout" to /{locale}/checkout.
import Link from "next/link";
import { useEffect, useRef } from "react";
import type { Locale } from "@live-commerce/i18n";
import { useCart } from "./CartProvider";
import { CartLines, CartNotice, CartSummary } from "./CartLines";
import { cartPath } from "../lib/routes";
import { fmt, shopCopy } from "../lib/shop-copy";
import { BagIcon, CloseIcon } from "./icons";

export function HeaderCart({ locale }: { locale: Locale }) {
  const copy = shopCopy[locale];
  const { count, setDrawerOpen } = useCart();
  return (
    <button type="button" className="sf-iconbtn sf-cartbtn" aria-label={fmt(copy.cartItems, { n: count })} onClick={() => setDrawerOpen(true)} data-testid="header-cart">
      <BagIcon />
      {count > 0 && (
        <span className="sf-badge" data-testid="cart-count" aria-hidden="true">
          {count > 99 ? "99+" : count}
        </span>
      )}
    </button>
  );
}

export default function CartDrawer({ locale }: { locale: Locale }) {
  const copy = shopCopy[locale];
  const { drawerOpen, setDrawerOpen, cart, ready } = useCart();
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (drawerOpen && !dialog.open) dialog.showModal();
    if (!drawerOpen && dialog.open) dialog.close();
  }, [drawerOpen]);
  const empty = !cart || cart.items.length === 0;
  const close = () => setDrawerOpen(false);
  return (
    <dialog
      ref={ref}
      className="sf-drawer"
      aria-labelledby="sf-drawer-title"
      onClose={close}
      onClick={(event) => {
        // A click on the ::backdrop lands on the <dialog> element itself.
        if (event.target === event.currentTarget) close();
      }}
      data-testid="cart-drawer"
    >
      {/* Contents exist only while open: the cart page renders the same lines, and a closed drawer must not fetch details. */}
      {drawerOpen && (
      <div className="sf-drawer__panel">
        <header className="sf-drawer__head">
          <h2 id="sf-drawer-title">{copy.cartTitle}</h2>
          <button type="button" className="sf-iconbtn" aria-label={copy.closeCart} onClick={close}>
            <CloseIcon />
          </button>
        </header>
        <div className="sf-drawer__body">
          <CartNotice locale={locale} />
          {empty ? (
            <div className="sf-empty sf-empty--compact">
              <p className="sf-empty__title">{ready ? copy.cartEmpty : copy.loading}</p>
              {ready && <p className="sf-muted">{copy.cartEmptyHint}</p>}
              <button type="button" className="sf-btn sf-btn--ghost" onClick={close}>
                {copy.continueShopping}
              </button>
            </div>
          ) : (
            <CartLines locale={locale} />
          )}
        </div>
        {!empty && (
          <footer className="sf-drawer__foot">
            <CartSummary locale={locale} onNavigate={close} />
            <Link className="sf-link" href={cartPath(locale)} onClick={close}>
              {copy.viewCart}
            </Link>
          </footer>
        )}
      </div>
      )}
    </dialog>
  );
}
