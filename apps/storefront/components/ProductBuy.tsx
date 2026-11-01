"use client";

// Purpose: product option/quantity controls and cart actions; selected options also drive ProductMedia's gallery.
// Depends on: catalog-v2 ProductDetail, CartProvider (/api/buyer/session/cart -> Go buyer routes), shop copy and Next router.
// Used by: ProductMedia on the product detail page; Go remains the stock and amount authority.
// Buy box of the product page: option selectors that resolve to one variant (SKU), price + compare-at, stock hint, quantity,
// Add to cart and Buy now. BFF routes (via CartProvider): /api/buyer/session, /api/buyer/cart -> Go /v1/buyer/session, /cart.
// The variant list comes from the server-rendered catalog-v2 detail (Go decides price and stock hint; this only picks which
// SKU). Add = merge into the shared multi-SKU cart (CartProvider.add, journalled + CAS); Buy now = add, then go to checkout.
// A sold-out variant cannot be added (button disabled); a combination that has no in-stock variant is disabled in the chips.
// The phone dock opens the local options task; only its successful add goes to the cart. Base/desktop actions retain their drawer/checkout behavior.
import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import { formatMoney } from "../lib/money";
import { cartPath, checkoutPath } from "../lib/routes";
import { resolveVariant, valueAvailable } from "../lib/shop-contract";
import type { ProductDetail } from "../lib/shop-contract";
import { browseCopy } from "../lib/browse-copy";
import MobileProductOptions from "./product/MobileProductOptions";
import styles from "./product/MobileProductOptions.module.css";
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
  const optionsCopy = browseCopy[locale].productOptions;
  const router = useRouter();
  const cart = useCart();
  const [quantity, setQuantity] = useState("1");
  const [added, setAdded] = useState(false);
  const [optionsOpen, setOptionsOpen] = useState(false);
  const [navigating, setNavigating] = useState(false);
  const busy = cart.busy || navigating;
  const optionEntry = useRef<HTMLButtonElement | null>(null);
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
  }, [optionsOpen]);

  function pick(axis: number, value: string) {
    // Prefer the in-stock variant that keeps as many of the other current picks as possible.
    const candidates = product.variants.filter((v) => v.stock !== "out" && v.option_values[axis] === value);
    if (candidates.length === 0) return;
    const score = (v: (typeof candidates)[number]) => v.option_values.filter((x, i) => i !== axis && x === chosen[i]).length;
    const best = candidates.reduce((a, b) => (score(b) > score(a) ? b : a));
    onChoose(best.option_values);
    setAdded(false);
  }

  function openOptions(button: HTMLButtonElement) {
    optionEntry.current = button;
    setOptionsOpen(true);
  }

  function closeOptions() {
    if (busy) return;
    setOptionsOpen(false);
    requestAnimationFrame(() => optionEntry.current?.focus());
  }

  async function addToCart(destination: "drawer" | "checkout" | "cart") {
    if (!variant || soldOut || busy) return;
    const ok = await cart.add(variant.sku_id, qty);
    if (!ok) return;
    if (destination === "checkout") router.push(checkoutPath(locale));
    else if (destination === "cart") {
      // Keep this successful add disabled while Next navigates; a second click must not compose another add.
      setNavigating(true);
      router.push(cartPath(locale));
    }
    else {
      setAdded(true);
      cart.setDrawerOpen(true);
    }
  }

  const stockText = !variant ? copy.selectionIncomplete : variant.stock === "in" ? copy.inStock : variant.stock === "low" ? copy.lowStock : copy.outOfStock;
  const addLabel = soldOut ? copy.outOfStock : copy.addToCart;
  const optionControls = (<>
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
                  <input type="radio" name={`opt-${i}`} value={value} checked={chosen[i] === value} disabled={(optionsOpen && busy) || (!available && chosen[i] !== value)} onChange={() => pick(i, value)} />
                  {optionsOpen && chosen[i] === value && <span aria-hidden="true">✓ </span>}
                  <span>{value}</span>
                </label>
              );
            })}
          </div>
        </fieldset>
      ))}
  </>);
  const quantityControl = (
      <div className="sf-qty">
        <label htmlFor="sf-qty">{copy.quantity}</label>
        <div className="sf-stepper">
          <button type="button" aria-label={copy.decrease} disabled={qty <= 1 || (optionsOpen && busy)} onClick={() => setQuantity(String(qty - 1))}>
            <MinusIcon />
          </button>
          <input id="sf-qty" disabled={optionsOpen && busy} type="text" inputMode="numeric" pattern="[0-9]*" value={quantity} onChange={(e) => setQuantity(e.target.value.replace(/\D/g, "").slice(0, 2))} onBlur={() => setQuantity(String(qty))} data-testid="qty-input" />
          <button type="button" aria-label={copy.increase} disabled={qty >= 99 || (optionsOpen && busy)} onClick={() => setQuantity(String(qty + 1))}>
            <PlusIcon />
          </button>
        </div>
      </div>
  );
  return (
    <div className="sf-buy" data-testid={optionsOpen ? undefined : "product-buy"} data-sku={variant?.sku_id}>
      {!optionsOpen && <>
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

      </>}
      <div className={styles.taskEntry} data-testid="product-options-task-entry">
        <button type="button" aria-label={optionsCopy.choose} disabled={busy} onClick={(event) => openOptions(event.currentTarget)}>
          <strong>{optionsCopy.choose}</strong>
          <span>{chosen.filter(Boolean).join(" · ")} · {qty}</span>
          <i aria-hidden="true">›</i>
        </button>
      </div>
      {!optionsOpen && <>
      {optionControls}
      <p className={`sf-stock sf-stock--${variant?.stock ?? "none"}`} data-testid="stock-hint" role="status">
        <i aria-hidden="true" />
        {stockText}
      </p>

      {quantityControl}

      <div className="sf-buy__actions" ref={actions}>
        <button type="button" className="sf-btn sf-btn--primary sf-btn--block" disabled={soldOut || busy} onClick={() => void addToCart("drawer")} data-testid="add-to-cart">
          {busy ? copy.loading : addLabel}
        </button>
        <button type="button" className="sf-btn sf-btn--outline sf-btn--block" disabled={soldOut || busy} onClick={() => void addToCart("checkout")} data-testid="buy-now">
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
      </>}

      <div className={`sf-sticky ${styles.entryDock}`} data-testid={sticky ? "sticky-buy" : undefined}>
          <strong>{variant ? formatMoney(locale, variant.price_minor, currency) : "—"}</strong>
          <div data-testid="product-options-dock-entry">
            <button type="button" className="sf-btn sf-btn--primary" disabled={busy} onClick={(event) => openOptions(event.currentTarget)}>
              {optionsCopy.choose}
            </button>
          </div>
      </div>
      {optionsOpen && (
        <MobileProductOptions locale={locale} product={product} variant={variant} currency={currency}
          busy={busy} onClose={closeOptions}
          action={<button type="button" className="sf-btn sf-btn--primary sf-btn--block" disabled={soldOut || busy}
            onClick={() => void addToCart("cart")} data-testid="add-to-cart">
            {busy ? copy.loading : `${addLabel}${variant ? ` · ${formatMoney(locale, variant.price_minor, currency)}` : ""}`}
          </button>}>
          {optionControls}
          {quantityControl}
          <p className={styles.limit}>{optionsCopy.limit}</p>
          <p className={styles.pickup}>{optionsCopy.pickup}</p>
          <CartNotice locale={locale} />
        </MobileProductOptions>
      )}
    </div>
  );
}
