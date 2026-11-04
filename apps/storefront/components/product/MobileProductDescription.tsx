"use client";

// Purpose: present the 51:1381 mobile description task (merchant text + generic delivery section) on the existing product route.
// Depends on: native dialog, catalog-v2 merchant text, existing copy and the accepted options chrome styles.
// Used by: ProductMedia, which owns open state and restores the invoking entry's focus.
// Invariants: plain merchant text; delivery copy stays generic (a store's enabled methods are only known at checkout);
// no transport, cart state or purchase-control remount.
import { useEffect, useRef } from "react";
import type { Locale } from "@live-commerce/i18n";
import { browseCopy } from "../../lib/browse-copy";
import { shopCopy } from "../../lib/shop-copy";
import type { ProductDetail } from "../../lib/shop-contract";
import chrome from "./MobileProductOptions.module.css";
import styles from "./MobileProductDescription.module.css";

/** Render the 51:1381 description task: merchant title/description, then the generic delivery section. */
export default function MobileProductDescription({ locale, product, onClose }: {
  locale: Locale; product: ProductDetail; onClose: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const copy = browseCopy[locale];
  const back = copy.productOptions.back;
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
    <dialog ref={dialog} className={chrome.task} data-testid="product-description-task" aria-labelledby="product-description-title"
      onCancel={(event) => { event.preventDefault(); onClose(); }}>
      <header className={chrome.header} data-testid="product-description-header">
        <button type="button" aria-label={back} onClick={onClose}><span aria-hidden="true">‹</span></button>
        <h2 id="product-description-title">{shopCopy[locale].description}</h2>
      </header>
      <div className={`${chrome.content} ${styles.body}`} data-testid="product-description-body">
        <h3 className={styles.title}>{product.title}</h3>
        <p className={styles.description}>{product.description}</p>
        <section className={styles.delivery} data-testid="product-description-delivery" aria-labelledby="product-description-delivery">
          <h3 id="product-description-delivery">{copy.delivery}</h3>
          <p>{copy.optionsNote}</p>
          <p>{copy.productOptions.pickup}</p>
          <p className={chrome.pickup}>{copy.productDescription.deliveryFee}</p>
        </section>
      </div>
      <footer className={chrome.dock} data-testid="product-description-dock">
        <button type="button" className="sf-btn sf-btn--primary sf-btn--block" onClick={onClose}>{back}</button>
      </footer>
    </dialog>
  );
}
