"use client";

// Purpose: product option/quantity controls and cart actions; selected options also drive ProductMedia's gallery.
// Depends on: catalog-v2 ProductDetail, CartProvider (/api/buyer/session/cart -> Go buyer routes), shop copy and Next router.
// Used by: ProductMedia on the product detail page; Go remains the stock and amount authority.
// Buy box of the product page: option selectors that resolve to one variant (SKU), price + compare-at, stock hint, quantity,
// Add to cart and Buy now. BFF routes (via CartProvider): /api/buyer/session, /api/buyer/cart -> Go /v1/buyer/session, /cart.
// The variant list comes from the server-rendered catalog-v2 detail (Go decides price and stock hint; this only picks which
// SKU). Add = merge into the shared multi-SKU cart (CartProvider.add, journalled + CAS); Buy now = add, then go to checkout.
// A sold-out variant cannot be added (button disabled); a combination that has no in-stock variant is disabled in the chips.
// A sticky phone bar repeats the add action only while the inline actions are scrolled out of view (never two visible CTAs).
import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import { formatMoney } from "../lib/money";
import { checkoutPath } from "../lib/routes";
import { resolveVariant, valueAvailable } from "../lib/shop-contract";
import type { ProductDetail } from "../lib/shop-contract";
import { fmt, shopCopy } from "../lib/shop-copy";
import { useCart } from "./CartProvider";
import { CartNotice } from "./CartLines";
import { CheckIcon, MinusIcon, PlusIcon } from "./icons";

/** Render controlled variant choices and initiate the existing journalled cart add/checkout actions. */
export default function ProductBuy({ locale, product, currency, chosen, onChoose }: {
  locale: Locale; product: ProductDetail; currency: string;
  chosen: (string | null)[]; onChoose: (values: string[]) => void;
}) {
  const copy = shopCopy[locale];
  const router = useRouter();
  const cart = useCart();
  const [quantity, setQuantity] = useState("1");
  const [added, setAdded] = useState(false);
  const [sticky, setSticky] = useState(false);
  const actions = useRef<HTMLDivElement>(null);
  const variant = useMemo(() => resolveVariant(product.options, product.variants, chosen), [product, chosen]);
  const qty = Math.min(99, Math.max(1, Math.floor(Number(quantity)) || 1));
  const soldOut = !variant || variant.stock === "out";
  const onSale = variant?.compare_at_minor != null && variant.compare_at_minor > variant.price_minor;

  useEffect(() => {
    const el = actions.current;
    if (!el) return;
    let frame = 0;
    const measure = () => { frame = 0; setSticky(el.getBoundingClientRect().bottom <= 0); };
    // IntersectionObserver can miss a jump from below to above the viewport (both non-intersecting).
    // Coalesce passive scroll/resize events into one position read per frame, including initial restored scroll.
    const schedule = () => { if (!frame) frame = requestAnimationFrame(measure); };
    window.addEventListener("scroll", schedule, { passive: true });
    window.addEventListener("resize", schedule);
    schedule();
    return () => {
      window.removeEventListener("scroll", schedule);
      window.removeEventListener("resize", schedule);
      cancelAnimationFrame(frame);
    };
  }, []);

  function pick(axis: number, value: string) {
    // Prefer the in-stock variant that keeps as many of the other current picks as possible.
    const candidates = product.variants.filter((v) => v.stock !== "out" && v.option_values[axis] === value);
    if (candidates.length === 0) return;
    const score = (v: (typeof candidates)[number]) => v.option_values.filter((x, i) => i !== axis && x === chosen[i]).length;
    const best = candidates.reduce((a, b) => (score(b) > score(a) ? b : a));
    onChoose(best.option_values);
    setAdded(false);
  }

  async function addToCart(thenCheckout: boolean) {
    if (!variant || soldOut) return;
    const ok = await cart.add(variant.sku_id, qty);
    if (!ok) return;
    if (thenCheckout) router.push(checkoutPath(locale));
    else {
      setAdded(true);
      cart.setDrawerOpen(true);
    }
  }

  const stockText = !variant ? copy.selectionIncomplete : variant.stock === "in" ? copy.inStock : variant.stock === "low" ? copy.lowStock : copy.outOfStock;
  const addLabel = soldOut ? copy.outOfStock : copy.addToCart;
  return (
    <div className="sf-buy" data-testid="product-buy" data-sku={variant?.sku_id}>
      <div className="sf-buy__price" aria-live="polite">
        {variant ? (
          <>
            <span className={`sf-buy__now${onSale ? " is-sale" : ""}`} data-testid="variant-price">
              {formatMoney(locale, variant.price_minor, currency)}
            </span>
            {onSale && variant.compare_at_minor !== null && (
              <>
                <s>{formatMoney(locale, variant.compare_at_minor, currency)}</s>
                <span className="sf-tag sf-tag--sale">{fmt(copy.save, { amount: formatMoney(locale, variant.compare_at_minor - variant.price_minor, currency) })}</span>
              </>
            )}
          </>
        ) : (
          <span className="sf-buy__now">—</span>
        )}
      </div>

      {product.options.map((axis, i) => (
        <fieldset className="sf-opt" key={axis.name}>
          <legend>
            {axis.name}
            {chosen[i] && <span>{chosen[i]}</span>}
          </legend>
          <div className="sf-opt__chips">
            {axis.values.map((value) => {
              const available = valueAvailable(product.variants, i, value, chosen);
              return (
                <label key={value} className={`sf-chip${chosen[i] === value ? " is-on" : ""}${available ? "" : " is-out"}`}>
                  <input type="radio" name={`opt-${i}`} value={value} checked={chosen[i] === value} disabled={!available && chosen[i] !== value} onChange={() => pick(i, value)} />
                  <span>{value}</span>
                </label>
              );
            })}
          </div>
        </fieldset>
      ))}

      <p className={`sf-stock sf-stock--${variant?.stock ?? "none"}`} data-testid="stock-hint" role="status">
        <i aria-hidden="true" />
        {stockText}
      </p>

      <div className="sf-qty">
        <label htmlFor="sf-qty">{copy.quantity}</label>
        <div className="sf-stepper">
          <button type="button" aria-label={copy.decrease} disabled={qty <= 1} onClick={() => setQuantity(String(qty - 1))}>
            <MinusIcon />
          </button>
          <input id="sf-qty" type="text" inputMode="numeric" pattern="[0-9]*" value={quantity} onChange={(e) => setQuantity(e.target.value.replace(/\D/g, "").slice(0, 2))} onBlur={() => setQuantity(String(qty))} data-testid="qty-input" />
          <button type="button" aria-label={copy.increase} disabled={qty >= 99} onClick={() => setQuantity(String(qty + 1))}>
            <PlusIcon />
          </button>
        </div>
      </div>

      <div className="sf-buy__actions" ref={actions}>
        <button type="button" className="sf-btn sf-btn--primary sf-btn--block" disabled={soldOut || cart.busy} onClick={() => void addToCart(false)} data-testid="add-to-cart">
          {cart.busy ? copy.loading : addLabel}
        </button>
        <button type="button" className="sf-btn sf-btn--outline sf-btn--block" disabled={soldOut || cart.busy} onClick={() => void addToCart(true)} data-testid="buy-now">
          {copy.buyNow}
        </button>
      </div>
      <p className="sf-added" role="status" aria-live="polite">
        {added && (
          <>
            <CheckIcon /> {copy.added}
          </>
        )}
      </p>
      <CartNotice locale={locale} />

      {sticky && (
        <div className="sf-sticky" data-testid="sticky-buy">
          <div>
            <strong>{variant ? formatMoney(locale, variant.price_minor, currency) : "—"}</strong>
            <span>{product.title}</span>
          </div>
          <button type="button" className="sf-btn sf-btn--primary" disabled={soldOut || cart.busy} onClick={() => void addToCart(false)}>
            {addLabel}
          </button>
        </div>
      )}
    </div>
  );
}
