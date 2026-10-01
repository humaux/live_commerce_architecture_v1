"use client";

// Cart lines, summary and problem notice, shared by the drawer (components/CartDrawer.tsx), the cart page
// (app/[locale]/cart/page.tsx -> CartView) and the checkout summary (components/CheckoutFlow.tsx).
// Data: the shared cart from CartProvider (BFF /api/buyer/cart) plus display details from lib/cart-details.ts
// (BFF /api/buyer/catalog, /api/shop/product/{id}, /api/buyer/checkout-options). Edits call CartProvider.setQuantity,
// which journals and CAS-writes the whole cart; quantity 0 removes the line. Prices here are listed prices for orientation;
// the quote at checkout is the only authority for the total (I05).
import Link from "next/link";
import { useEffect, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { useCart } from "./CartProvider";
import { cartDetails } from "../lib/cart-details";
import type { CartDetails, LineView } from "../lib/cart-details";
import { freeShippingProgress } from "../lib/shop-contract";
import { checkoutPath, productImage, productPath } from "../lib/routes";
import { formatMoney } from "../lib/money";
import { fmt, shopCopy } from "../lib/shop-copy";
import { ArrowRightIcon, ImageIcon, MinusIcon, PlusIcon, TrashIcon } from "./icons";

export function useCartDetails(): { details: CartDetails | null; failed: boolean; loading: boolean } {
  const { cart, context } = useCart();
  const [details, setDetails] = useState<CartDetails | null>(null);
  const [failed, setFailed] = useState(false);
  const [loading, setLoading] = useState(false);
  const key = cart ? `${cart.id}:${cart.version}` : "";
  useEffect(() => {
    if (!cart || !context || cart.items.length === 0) {
      setDetails(null);
      setFailed(false);
      setLoading(false);
      return;
    }
    let live = true;
    setLoading(true);
    cartDetails(context, cart)
      .then((value) => live && (setDetails(value), setFailed(false)))
      .catch(() => live && setFailed(true))
      .finally(() => live && setLoading(false));
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- `key` is the cart identity+version; `cart` itself changes by reference.
  }, [key, context]);
  return { details, failed, loading };
}

export function CartNotice({ locale }: { locale: Locale }) {
  const copy = shopCopy[locale];
  const { problem, retry, busy, startNewCart, dismiss } = useCart();
  if (!problem) return null;
  if (problem === "order")
    return (
      <div className="sf-notice" role="alert" data-testid="cart-order-locked">
        <strong>{copy.orderLockedTitle}</strong>
        <p>{copy.orderLockedBody}</p>
        <div className="sf-notice__actions">
          <Link className="sf-btn sf-btn--ghost" href={checkoutPath(locale)}>
            {copy.viewOrder}
          </Link>
          <button type="button" className="sf-btn sf-btn--ghost" disabled={busy} onClick={() => void startNewCart()}>
            {copy.startNewCart}
          </button>
        </div>
      </div>
    );
  const text =
    problem === "uncertain"
      ? copy.cartUncertain
      : problem === "conflict" || problem === "session"
        ? copy.cartConflict
        : problem === "storage"
          ? copy.cartNoStorage
          : copy.cartFailed;
  return (
    <div className="sf-notice" role="alert" data-testid="cart-problem">
      <p>{text}</p>
      <div className="sf-notice__actions">
        {problem === "uncertain" && (
          <>
            <button type="button" className="sf-btn sf-btn--ghost" disabled={busy} onClick={() => void retry()}>
              {copy.retry}
            </button>
            <Link className="sf-btn sf-btn--ghost" href={checkoutPath(locale)}>
              {copy.checkout}
            </Link>
          </>
        )}
        {problem !== "uncertain" && (
          <button type="button" className="sf-btn sf-btn--ghost" onClick={dismiss}>
            OK
          </button>
        )}
      </div>
    </div>
  );
}

function Line({ locale, line, currency, readOnly }: { locale: Locale; line: LineView; currency: string; readOnly: boolean }) {
  const copy = shopCopy[locale];
  const { setQuantity, busy } = useCart();
  const name = line.gone ? copy.lineUnavailable : line.title;
  const href = line.slug ? productPath(locale, line.slug) : null;
  const image = line.productID && line.imageID ? productImage(line.productID, line.imageID) : null;
  return (
    <li className="sf-line" data-testid="cart-line" data-sku={line.sku_id}>
      <div className="sf-line__media">
        {image ? (
          <img src={image} alt="" width={96} height={120} loading="lazy" />
        ) : (
          <span className="sf-line__ph" role="img" aria-label={copy.noImage}>
            <ImageIcon size={28} />
          </span>
        )}
      </div>
      <div className="sf-line__body">
        <div className="sf-line__head">
          {href ? (
            <Link className="sf-line__title" href={href}>
              {name}
            </Link>
          ) : (
            <span className={`sf-line__title${line.gone ? " sf-line__title--gone" : ""}`}>{name}</span>
          )}
          {!line.gone && (
            <span className="sf-line__total" data-testid="cart-line-total">
              {formatMoney(locale, line.unitMinor * line.quantity, currency)}
            </span>
          )}
        </div>
        {line.variantTitle && <p className="sf-line__variant">{line.variantTitle}</p>}
        {!line.gone && (
          <p className="sf-line__unit">
            {formatMoney(locale, line.unitMinor, currency)}
            {line.compareAtMinor !== null && line.compareAtMinor > line.unitMinor && <s>{formatMoney(locale, line.compareAtMinor, currency)}</s>}
            {line.liveUnitMinor !== null && <em className="sf-line__live">{copy.livePrice}</em>}
            {line.stock === "low" && <em>{copy.lowStock}</em>}
            {line.stock === "out" && <em className="sf-line__out">{copy.outOfStock}</em>}
          </p>
        )}
        {!readOnly && (
        <div className="sf-line__ctl">
          {!line.gone && (
            <div className="sf-stepper" role="group" aria-label={`${copy.quantity}: ${name}`}>
              <button type="button" aria-label={copy.decrease} disabled={busy || line.quantity <= 1} onClick={() => void setQuantity(line.sku_id, line.quantity - 1)}>
                <MinusIcon />
              </button>
              <output aria-live="polite" data-testid="cart-line-qty">
                {line.quantity}
              </output>
              <button type="button" aria-label={copy.increase} disabled={busy || line.quantity >= 99} onClick={() => void setQuantity(line.sku_id, line.quantity + 1)}>
                <PlusIcon />
              </button>
            </div>
          )}
          <button type="button" className="sf-link-btn" aria-label={fmt(copy.removeLine, { name })} disabled={busy} onClick={() => void setQuantity(line.sku_id, 0)}>
            <TrashIcon />
            <span>{copy.remove}</span>
          </button>
        </div>
        )}
        {readOnly && !line.gone && <p className="sf-line__unit">× {line.quantity}</p>}
      </div>
    </li>
  );
}

export function CartLines({ locale, readOnly = false }: { locale: Locale; readOnly?: boolean }) {
  const copy = shopCopy[locale];
  const { cart } = useCart();
  const { details, failed, loading } = useCartDetails();
  if (!cart || cart.items.length === 0) return null;
  if (!details)
    return (
      <p className="sf-muted" role="status" aria-live="polite">
        {failed ? copy.cartFailed : copy.loading}
      </p>
    );
  return (
    <ul className={`sf-lines${loading ? " is-loading" : ""}`} aria-busy={loading}>
      {details.lines.map((line) => (
        <Line key={line.sku_id} locale={locale} line={line} currency={details.currency} readOnly={readOnly} />
      ))}
    </ul>
  );
}

export function CartSummary({ locale, onNavigate }: { locale: Locale; onNavigate?: () => void }) {
  const copy = shopCopy[locale];
  const { cart, busy } = useCart();
  const { details } = useCartDetails();
  if (!cart || cart.items.length === 0 || !details) return null;
  const progress = freeShippingProgress(details.subtotal, details.threshold);
  const blocked = details.lines.every((l) => l.gone);
  return (
    <div className="sf-summary" data-testid="cart-summary">
      {progress && (
        <div className="sf-free" data-testid="free-shipping">
          <p>{progress.reached ? copy.freeShipReached : fmt(copy.freeShipRemaining, { amount: formatMoney(locale, progress.remaining, details.currency) })}</p>
          <div className="sf-free__bar" aria-hidden="true">
            <span style={{ width: `${Math.round(progress.ratio * 100)}%` }} />
          </div>
        </div>
      )}
      <p className="sf-summary__row">
        <span>{copy.subtotal}</span>
        <strong data-testid="cart-subtotal">{formatMoney(locale, details.subtotal, details.currency)}</strong>
      </p>
      <p className="sf-muted sf-summary__note">{copy.shippingAtCheckout}</p>
      <Link className={`sf-btn sf-btn--primary sf-btn--block${blocked || busy ? " is-disabled" : ""}`} aria-disabled={blocked || busy} href={checkoutPath(locale)} onClick={onNavigate} data-testid="cart-checkout">
        {copy.checkout}
        <ArrowRightIcon />
      </Link>
    </div>
  );
}
