"use client";

// Product photo gallery of the product page (catalog-media CM6). Calls no BFF itself: the photos come from the
// catalog read ProductPurchase already makes (BFF GET /api/buyer/catalog?product_id=… -> Go /v1/buyer/catalog,
// `images`), and the bytes load from the public same-origin path /media/p/{product}/{image}
// (app/media/p/[productID]/[imageID]/route.ts -> Go /v1/buyer/media/p/…). Renders nothing when there are no photos
// (the page then looks as before). Non-goals: no zoom, no lightbox, no video.
import { useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { mediaPath } from "../lib/home.ts";
import type { ProductImageMeta } from "../lib/purchase";
import styles from "./ShopHome.module.css";

const label: Record<Locale, (n: number, of: number) => string> = {
  en: (n, of) => `Photo ${n} of ${of}`,
  "zh-CN": (n, of) => `第 ${n} 张，共 ${of} 张`,
  "zh-TW": (n, of) => `第 ${n} 張，共 ${of} 張`,
};

export default function ProductGallery({
  locale,
  productID,
  name,
  images,
}: {
  locale: Locale;
  productID: string;
  name: string;
  images: ProductImageMeta[];
}) {
  const [index, setIndex] = useState(0);
  if (!images.length) return null;
  const shown = images[Math.min(index, images.length - 1)];
  return (
    <div className={styles.gallery} data-testid="product-gallery">
      <img
        className={styles.main}
        src={mediaPath(productID, shown.id)}
        alt={name}
        width={shown.width ?? undefined}
        height={shown.height ?? undefined}
      />
      {images.length > 1 && (
        <div className={styles.thumbs}>
          {images.map((image, i) => (
            <button
              key={image.id}
              type="button"
              aria-label={label[locale](i + 1, images.length)}
              aria-current={i === index}
              onClick={() => setIndex(i)}
            >
              <img src={mediaPath(productID, image.id)} alt="" loading="lazy" />
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
