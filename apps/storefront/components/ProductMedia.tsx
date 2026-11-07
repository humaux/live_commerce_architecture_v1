"use client";

// Purpose: share the selected buyer variant and present the local mobile description task.
// Depends on: frozen catalog-v2 ProductDetail, gallery/buy/description components and React local state; no direct transport.
// Used by: /{locale}/products/{slug} server page; assurance remains server-rendered content.
// Invariants: ProductBuy stays mounted across description open/close; merchant text and purchase drafts remain unchanged.
import { useRef, useState, type ReactNode } from "react";
import type { Locale } from "@live-commerce/i18n";
import { browseCopy } from "../lib/browse-copy";
import { initialChoice, resolveVariant, type ProductDetail } from "../lib/shop-contract";
import ProductGallery from "./ProductGallery";
import ProductBuy from "./ProductBuy";
import MobileProductDescription from "./product/MobileProductDescription";
import taskStyles from "./product/MobileProductOptions.module.css";

/** Render linked gallery and buy controls; option selection is local, cart writes remain in ProductBuy. */
export default function ProductMedia({ locale, product, currency, assurance }: { locale: Locale; product: ProductDetail; currency: string; assurance: ReactNode }) {
  const [chosen, setChosen] = useState<(string | null)[]>(() => initialChoice(product.variants));
  const [descriptionOpen, setDescriptionOpen] = useState(false);
  const descriptionEntry = useRef<HTMLButtonElement>(null);
  const variant = resolveVariant(product.options, product.variants, chosen);
  function closeDescription() {
    setDescriptionOpen(false);
    requestAnimationFrame(() => descriptionEntry.current?.focus());
  }
  return <>
    <ProductGallery locale={locale} productID={product.id} name={product.title} images={product.images}
      selectedImageID={variant?.image_id ?? null} selectionKey={variant?.sku_id ?? ""} />
    <div className="sf-product__side">
      <h1 className="sf-product__title">{product.title}</h1>
      <ProductBuy locale={locale} product={product} currency={currency} chosen={chosen} onChoose={setChosen}
        descriptionEntry={<div className={taskStyles.taskEntry} data-testid="product-description-task-entry">
          <button ref={descriptionEntry} type="button" aria-label={browseCopy[locale].productDescription.entry} onClick={() => setDescriptionOpen(true)}>
            <strong>{browseCopy[locale].productDescription.entry}</strong>
            <i aria-hidden="true">›</i>
          </button>
        </div>} />
      {assurance}
    </div>
    {descriptionOpen && <MobileProductDescription locale={locale} product={product} onClose={closeDescription} />}
  </>;
}
