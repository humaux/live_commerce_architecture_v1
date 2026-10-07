"use client";

// Purpose: share the selected buyer variant between the main gallery and purchase controls.
// Depends on: frozen catalog-v2 ProductDetail, ProductGallery/ProductBuy and React local state; no direct transport.
// Used by: /{locale}/products/{slug} server page; assurance remains server-rendered content.
import { useState, type ReactNode } from "react";
import type { Locale } from "@live-commerce/i18n";
import { initialChoice, resolveVariant, type ProductDetail } from "../lib/shop-contract";
import ProductGallery from "./ProductGallery";
import ProductBuy from "./ProductBuy";

/** Render linked gallery and buy controls; option selection is local, cart writes remain in ProductBuy. */
export default function ProductMedia({ locale, product, currency, assurance }: { locale: Locale; product: ProductDetail; currency: string; assurance: ReactNode }) {
  const [chosen, setChosen] = useState<(string | null)[]>(() => initialChoice(product.variants));
  const variant = resolveVariant(product.options, product.variants, chosen);
  return <>
    <ProductGallery locale={locale} productID={product.id} name={product.title} images={product.images}
      selectedImageID={variant?.image_id ?? null} selectionKey={variant?.sku_id ?? ""} />
    <div className="sf-product__side">
      <h1 className="sf-product__title">{product.title}</h1>
      <ProductBuy locale={locale} product={product} currency={currency} chosen={chosen} onChoose={setChosen} />
      {assurance}
    </div>
  </>;
}
