// Purpose: Cart drawer (native <dialog>) and header bag button rendering shared CartProvider state.
// Depends on: React, next/link, @live-commerce/i18n, ./CartProvider, ./CartLines, ./state/StorefrontState and ./icons.
// Used by: storefront layout and header.
// Invariants: renders shared CartProvider state; read problems display signed StatePanel rather than empty cart.
"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { useCart } from "./CartProvider";
import { CartLines, CartNotice, CartSummary } from "./CartLines";
import { StorefrontState } from "./state/StorefrontState";
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
  const { drawerOpen, setDrawerOpen, cart, ready, readProblem, refresh } = useCart();
  const [retried, setRetried] = useState(false);
  useEffect(() => {
    if (!readProblem) setRetried(false);
  }, [readProblem]);
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
          {ready && readProblem ? (
            <StorefrontState
              locale={locale}
              variant={readProblem}
              failed={retried}
              onRead={readProblem === "offline" ? async () => {
                setRetried(false);
                const result = await refresh();
                if (result === false) setRetried(true);
              } : undefined}
            />
          ) : empty ? (
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
        {!empty && !readProblem && (
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
