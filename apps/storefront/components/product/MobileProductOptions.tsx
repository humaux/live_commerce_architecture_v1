"use client";

// Purpose: compose the source-defined full-screen mobile option task on the current product route.
// Depends on: native dialog, catalog-v2 read data, shared display helpers and scoped styles; no transport or cart state.
// Used by: ProductBuy, which alone owns choices, quantity, busy state and journalled cart actions.
// Invariants: only server SKU data supplies the summary; cancel never writes; busy cannot dismiss the task.
import { useEffect, useRef, type ReactNode } from "react";
import type { Locale } from "@live-commerce/i18n";
import { browseCopy } from "../../lib/browse-copy";
import { formatMoney } from "../../lib/money";
import { productImage } from "../../lib/routes";
import type { ProductDetail, Variant } from "../../lib/shop-contract";
import styles from "./MobileProductOptions.module.css";

/** Present existing purchase controls in the Figma 51:1380 mobile task without adding a cart protocol. */
export default function MobileProductOptions({ locale, product, variant, currency, busy, onClose, action, children }: {
  locale: Locale; product: ProductDetail; variant: Variant | null; currency: string;
  busy: boolean; onClose: () => void; action: ReactNode; children: ReactNode;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const copy = browseCopy[locale].productOptions;
  const imageID = variant?.image_id ?? product.images[0]?.id;
  useEffect(() => {
    const node = dialog.current;
    if (!node) return;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    node.showModal();
    return () => {
      node.close();
      document.body.style.overflow = overflow;
    };
  }, []);
  return (
    <dialog ref={dialog} className={styles.task} data-testid="product-options-task" aria-labelledby="product-options-title"
      onCancel={(event) => { event.preventDefault(); if (!busy) onClose(); }}>
      <header className={styles.header} data-testid="product-options-header">
        <button type="button" aria-label={copy.back} disabled={busy} onClick={onClose}><span aria-hidden="true">‹</span></button>
        <h2 id="product-options-title">{copy.choose}</h2>
      </header>
      <div className={styles.content} data-testid="product-buy" data-sku={variant?.sku_id}>
        <div className={styles.summary} data-testid="product-options-summary">
          {imageID && <img src={productImage(product.id, imageID)} alt={product.title} width={88} height={88} />}
          <div><h3>{product.title}</h3><p data-testid="variant-price">{variant ? formatMoney(locale, variant.price_minor, currency) : "—"}</p></div>
        </div>
        {children}
      </div>
      <footer className={styles.dock} data-testid="product-options-dock">{action}</footer>
    </dialog>
  );
}
