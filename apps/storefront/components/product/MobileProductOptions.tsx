"use client";

// Purpose: the 51:1380 mobile option task owns an uncommitted selection and quantity until explicit confirmation.
// Depends on: catalog-v2 ProductDetail, existing variant resolution, shop copy, media URLs and CartNotice recovery.
// Used by: ProductBuy; only its onConfirm callback may invoke the existing CartProvider mutation.
// Invariants: Back/Escape discard local draft, radios never join the committed option groups; Go owns stock/price.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { formatMoney } from "../../lib/money";
import { productImage } from "../../lib/routes";
import { resolveVariant, valueAvailable, type ProductDetail } from "../../lib/shop-contract";
import { shopCopy } from "../../lib/shop-copy";
import { CartNotice } from "../CartLines";
import { MinusIcon, PlusIcon } from "../icons";
import styles from "./MobileProductOptions.module.css";

/** Render a native modal draft editor; confirmation delegates one cart command to ProductBuy. */
export default function MobileProductOptions({ locale, product, currency, chosen, quantity, busy, locked, onClose, onConfirm }: {
  locale: Locale; product: ProductDetail; currency: string; chosen: (string | null)[]; quantity: string;
  busy: boolean; locked: boolean; onClose: () => void; onConfirm: (values: string[], amount: number) => Promise<void>;
}) {
  const copy = shopCopy[locale];
  const dialog = useRef<HTMLDialogElement>(null);
  const [values, setValues] = useState(() => [...chosen]);
  const [amount, setAmount] = useState(quantity);
  const qty = Math.min(99, Math.max(1, Math.floor(Number(amount)) || 1));
  const variant = resolveVariant(product.options, product.variants, values);
  const imageID = variant?.image_id ?? product.images[0]?.id;
  const disabled = busy || locked;
  const soldOut = !variant || variant.stock === "out";

  useEffect(() => {
    const node = dialog.current;
    if (!node) return;
    if (!node.open) node.showModal();
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => { node.close(); document.body.style.overflow = overflow; };
  }, []);

  function pick(axis: number, value: string) {
    // Match the existing inline choice rule without publishing a draft to ProductMedia's gallery.
    const candidates = product.variants.filter(v => v.stock !== "out" && v.option_values[axis] === value);
    if (!candidates.length) return;
    const score = (v: (typeof candidates)[number]) => v.option_values.filter((x, i) => i !== axis && x === values[i]).length;
    setValues([...candidates.reduce((a, b) => score(b) > score(a) ? b : a).option_values]);
  }

  return <dialog ref={dialog} className={styles.task} data-testid="product-options-task" aria-labelledby="c7-options-title"
    onCancel={(event) => { event.preventDefault(); if (!busy) onClose(); }}>
    <header className={styles.header}>
      <button type="button" aria-label={copy.c7BackToProduct} disabled={busy} onClick={onClose} autoFocus><span aria-hidden="true">‹</span></button>
      <h2 id="c7-options-title">{copy.c7ChooseOptions}</h2>
    </header>
    <div className={styles.content}>
      <div className={styles.summary} data-testid="product-options-summary">
        {imageID ? <img src={productImage(product.id, imageID)} alt={product.title} width={88} height={88} /> : <span className={styles.noImage}>{copy.noImage}</span>}
        <div><h3>{product.title}</h3><p data-testid="variant-price">{variant ? formatMoney(locale, variant.price_minor, currency) : "—"}</p></div>
      </div>
      <div className={styles.controls} data-testid="product-buy" data-sku={variant?.sku_id}>
        {product.options.map((axis, i) => <fieldset className="sf-opt" key={axis.name} disabled={disabled}>
          <legend>{axis.name}</legend>
          <div className="sf-opt__chips">
            {axis.values.map(value => {
              const available = valueAvailable(product.variants, i, value, values);
              return <label key={value} className={`sf-chip${values[i] === value ? " is-on" : ""}${available ? "" : " is-out"}`}>
                <input type="radio" name={`c7-draft-opt-${i}`} value={value} checked={values[i] === value}
                  disabled={!available && values[i] !== value} onChange={() => pick(i, value)} />
                <span>{values[i] === value && <span aria-hidden="true">✓ </span>}{value}</span>
              </label>;
            })}
          </div>
        </fieldset>)}
        {soldOut && <p role="status" data-testid="stock-hint">{variant ? copy.outOfStock : copy.selectionIncomplete}</p>}
        <div className="sf-qty">
          <label htmlFor="c7-draft-qty">{copy.quantity}</label>
          <div className="sf-stepper">
            <button type="button" aria-label={copy.decrease} disabled={disabled || qty <= 1} onClick={() => setAmount(String(qty - 1))}><MinusIcon /></button>
            <input id="c7-draft-qty" type="text" inputMode="numeric" pattern="[0-9]*" value={amount} disabled={disabled}
              onChange={event => setAmount(event.target.value.replace(/\D/g, "").slice(0, 2))} onBlur={() => setAmount(String(qty))} data-testid="qty-input" />
            <button type="button" aria-label={copy.increase} disabled={disabled || qty >= 99} onClick={() => setAmount(String(qty + 1))}><PlusIcon /></button>
          </div>
        </div>
        <p className={styles.limit}>{copy.c7QuantityLimit}</p>
        {/* ProductDetail has no store-level pickup capability; do not promise a CVS method here. */}
        <p className={styles.delivery}>{copy.c7DeliveryNotice}</p>
      </div>
      <CartNotice locale={locale} />
    </div>
    <footer className={styles.dock}>
      <button type="button" className="sf-btn sf-btn--primary sf-btn--block" data-testid="add-to-cart" disabled={disabled || soldOut}
        onClick={() => { if (variant) void onConfirm(variant.option_values, qty); }}>
        {busy ? copy.loading : soldOut ? copy.outOfStock : `${copy.addToCart} · ${variant ? formatMoney(locale, variant.price_minor, currency) : "—"}`}
      </button>
    </footer>
  </dialog>;
}
