"use client";

// Purpose: b-cart entry panel (102:2267 family): when the cart page is reached after an add on a product page, show the just-added
//   line (spec, quantity, subtotal, remove), the shipping note, 前往結帳 and a link back to that product, above the cart list.
// Depends on: CartProvider (setQuantity, journalled CAS), CartLines.useCartDetails (titles/prices), shop/browse copy, routes, money.
// Used by: components/CartView.tsx; ProductBuy writes CART_ENTRY_KEY after a successful add.
// Invariants: the entry is tab-local UI state (one SKU id, no PII, no new API field); it renders only while that SKU is a live line
//   of the server cart, so a removed or unknown SKU shows nothing. Prices are listed prices; the checkout quote stays the authority.
import Link from "next/link";
import { useEffect, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { useCart } from "../CartProvider";
import { useCartDetails } from "../CartLines";
import { browseCopy } from "../../lib/browse-copy";
import { checkoutPath, productPath } from "../../lib/routes";
import { formatMoney } from "../../lib/money";
import { fmt, shopCopy } from "../../lib/shop-copy";
import styles from "./CartEntry.module.css";

export const CART_ENTRY_KEY = "sf-cart-entry";

/** Remember the SKU a product page just added (storage may be blocked; the panel is optional, the cart is not). */
export function rememberCartEntry(sku: string) {
  try {
    sessionStorage.setItem(CART_ENTRY_KEY, sku);
  } catch {}
}

export default function CartEntry({ locale }: { locale: Locale }) {
  const copy = shopCopy[locale];
  const t = browseCopy[locale].cartEntry;
  const { setQuantity, busy } = useCart();
  const { details } = useCartDetails();
  const [sku, setSku] = useState<string | null>(null);
  useEffect(() => {
    try {
      setSku(sessionStorage.getItem(CART_ENTRY_KEY));
    } catch {}
  }, []);
  const line = details?.lines.find((l) => l.sku_id === sku && !l.gone);
  if (!line || !details) return null;
  return (
    <section className={styles.entry} data-testid="cart-entry" aria-labelledby="cart-entry-title">
      {line.slug && (
        <Link className="sf-link" href={productPath(locale, line.slug)}>
          ‹ {t.back}
        </Link>
      )}
      <h2 id="cart-entry-title" className={styles.title}>{fmt(t.title, { title: line.title })}</h2>
      <div className={styles.line} data-testid="cart-entry-line">
        <strong>{line.title}</strong>
        <p data-testid="cart-entry-spec"><span>{t.spec}</span> {line.variantTitle ?? browseCopy[locale].productOptions.standard}</p>
        <p data-testid="cart-entry-qty"><span>{copy.quantity}</span> {line.quantity}</p>
        <p data-testid="cart-entry-subtotal"><span>{t.subtotal}</span> <strong>{formatMoney(locale, line.unitMinor * line.quantity, details.currency)}</strong></p>
        <button type="button" className="sf-btn sf-btn--ghost" disabled={busy} onClick={() => void setQuantity(line.sku_id, 0)}>
          {t.remove}
        </button>
      </div>
      <p className={styles.note}>{t.ship}</p>
      <Link className="sf-btn sf-btn--primary sf-btn--block" href={checkoutPath(locale)}>
        {copy.checkout}
      </Link>
    </section>
  );
}
