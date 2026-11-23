"use client";

// Purpose: product option/quantity controls and cart actions; selected options also drive ProductMedia's gallery.
// Depends on: catalog-v2 ProductDetail, CartProvider (/api/buyer/session/cart -> Go buyer routes), MobileProductOptions, shop copy and Next router.
// Used by: ProductMedia on the product detail page; Go remains the stock and amount authority.
// Buy box of the product page: option selectors that resolve to one variant (SKU), price + compare-at, stock hint, quantity,
// Add to cart and Buy now. BFF routes (via CartProvider): /api/buyer/session, /api/buyer/cart -> Go /v1/buyer/session, /cart.
// Both actions use the SKU from the server-rendered catalog. Buy now keeps its single selection separate from the shared cart.
// A sold-out variant cannot be added (button disabled); a combination that has no in-stock variant is disabled in the chips.
// A sticky phone bar opens the same local option draft while the inline actions are scrolled out of view.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import { formatMoney } from "../lib/money";
import { buyNowPath, cartPath } from "../lib/routes";
import { BuyerClientError, initializeBuyerSession } from "../lib/buyer-client";
import { beginBuyNow } from "../lib/purchase";
import { resolveVariant, valueAvailable } from "../lib/shop-contract";
import type { ProductDetail } from "../lib/shop-contract";
import { fmt, shopCopy } from "../lib/shop-copy";
import { useCart } from "./CartProvider";
import { CartNotice } from "./CartLines";
import { CheckIcon, MinusIcon, PlusIcon } from "./icons";
import MobileProductOptions from "./product/MobileProductOptions";
import styles from "./product/MobileProductOptions.module.css";

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
  const [optionsOpen, setOptionsOpen] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [buyNowProblem, setBuyNowProblem] = useState<"uncertain" | "failed" | null>(null);
  const [recovery, setRecovery] = useState<{ values: string[]; quantity: number } | null>(null);
  const confirmation = useRef(false);
  const uncertainSeen = useRef(false);
  const optionsTrigger = useRef<HTMLButtonElement | null>(null);
  const variant = useMemo(() => resolveVariant(product.options, product.variants, chosen), [product, chosen]);
  const qty = Math.min(99, Math.max(1, Math.floor(Number(quantity)) || 1));
  const soldOut = !variant || variant.stock === "out";
  const onSale = variant?.compare_at_minor != null && variant.compare_at_minor > variant.price_minor;

  function openOptions(trigger: HTMLButtonElement) {
    if (cart.busy || confirmation.current) return;
    optionsTrigger.current = trigger;
    setOptionsOpen(true);
  }

  function closeOptions() {
    if (confirmation.current || cart.busy) return;
    setOptionsOpen(false);
  }

  useEffect(() => {
    if (!optionsOpen) optionsTrigger.current?.focus();
  }, [optionsOpen]);

  const completeOptions = useCallback((values: string[], amount: number) => {
    onChoose(values);
    setQuantity(String(amount));
    setOptionsOpen(false);
    setRecovery(null);
    router.push(cartPath(locale));
  }, [onChoose, router, locale]);

  useEffect(() => {
    if (!recovery || cart.busy) return;
    // CartNotice's existing Retry settles the same journal; never call add again after UNKNOWN.
    if (cart.problem === "uncertain") { uncertainSeen.current = true; return; }
    if (uncertainSeen.current && cart.problem === null) completeOptions(recovery.values, recovery.quantity);
    else setRecovery(null);
  }, [recovery, cart.busy, cart.problem, completeOptions]);

  async function confirmOptions(values: string[], amount: number) {
    const selected = resolveVariant(product.options, product.variants, values);
    if (!selected || selected.stock === "out" || confirmation.current || cart.busy || cart.problem === "uncertain" || recovery) return;
    confirmation.current = true;
    setConfirming(true);
    uncertainSeen.current = false;
    try {
      // One existing cart.add owns CAS, stock validation and durable idempotent UNKNOWN recovery.
      const ok = await cart.add(selected.sku_id, amount);
      if (ok) completeOptions(values, amount);
      else setRecovery({ values, quantity: amount });
    } finally {
      confirmation.current = false;
      setConfirming(false);
    }
  }

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

  async function addToCart() {
    if (!variant || soldOut) return;
    const ok = await cart.add(variant.sku_id, qty);
    if (!ok) return;
    setAdded(true);
    cart.setDrawerOpen(true);
  }

  async function buyNow() {
    if (!variant || soldOut || confirmation.current) return;
    confirmation.current = true;
    setConfirming(true);
    setBuyNowProblem(null);
    try {
      const { context } = await initializeBuyerSession();
      if (!context) throw new BuyerClientError("requires_reset");
      const selection = await beginBuyNow(context, variant.sku_id, qty);
      router.push(buyNowPath(locale, selection.id));
    } catch (reason) {
      setBuyNowProblem(reason instanceof BuyerClientError && reason.code === "uncertain" ? "uncertain" : "failed");
      confirmation.current = false;
      setConfirming(false);
    }
  }

  const stockText = !variant ? copy.selectionIncomplete : variant.stock === "in" ? copy.inStock : variant.stock === "low" ? copy.lowStock : copy.outOfStock;
  const addLabel = soldOut ? copy.outOfStock : copy.addToCart;
  return (
    <div className="sf-buy" data-testid={optionsOpen ? undefined : "product-buy"} data-sku={variant?.sku_id}>
      <div className={styles.taskEntry} data-testid="product-options-task-entry">
        <button type="button" disabled={cart.busy || confirming} onClick={(event) => openOptions(event.currentTarget)}>
          <strong>{copy.c7ChooseOptions}</strong>
          <span>{chosen.filter(Boolean).join(" · ")} · {qty}</span>
          <i aria-hidden="true">›</i>
        </button>
      </div>
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
        <button type="button" className="sf-btn sf-btn--primary sf-btn--block" disabled={soldOut || !cart.ready || cart.busy || confirming} onClick={() => void addToCart()} data-testid="add-to-cart">
          {!soldOut && (!cart.ready || cart.busy) ? copy.loading : addLabel}
        </button>
        <button type="button" className="sf-btn sf-btn--outline sf-btn--block" disabled={soldOut || !cart.ready || confirming} onClick={() => void buyNow()} data-testid="buy-now">
          {confirming ? copy.loading : buyNowProblem === "uncertain" ? copy.retry : copy.buyNow}
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
      {buyNowProblem && <p className="sf-notice" role="alert">{buyNowProblem === "uncertain" ? copy.buyNowUncertain : copy.buyNowFailed}</p>}

      {sticky && (
        <div className={`sf-sticky ${styles.entryDock}`} data-testid="sticky-buy">
          <div>
            <strong>{variant ? formatMoney(locale, variant.price_minor, currency) : "—"}</strong>
            <span>{product.title}</span>
          </div>
          <button type="button" className="sf-btn sf-btn--primary" disabled={cart.busy || confirming} onClick={(event) => openOptions(event.currentTarget)} data-testid="product-options-dock-entry">
            {copy.c7ChooseOptions}
          </button>
        </div>
      )}
      {optionsOpen && <MobileProductOptions locale={locale} product={product} currency={currency}
        chosen={chosen} quantity={quantity} busy={cart.busy || confirming} locked={cart.problem === "uncertain" || recovery !== null}
        onClose={closeOptions} onConfirm={confirmOptions} />}
    </div>
  );
}
