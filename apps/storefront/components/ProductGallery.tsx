"use client";

// Product photo gallery of the product page: a scroll-snap strip (swipe on phones, wheel/arrows on desktop) plus a thumbnail
// row that scrolls to a photo. Calls no BFF itself: the photos come from the server-rendered catalog-v2 detail
// (GET /v1/buyer/catalog/v2/products/{slug}, `images`) and the bytes load from /media/p/{product}/{image}
// (app/media/p/[productID]/[imageID]/route.ts -> Go /v1/buyer/media/p/...). Without photos it shows a neutral placeholder so
// the page layout does not collapse. Non-goals: no zoom/lightbox, no video.
import { useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { productImage } from "../lib/routes";
import { fmt, shopCopy } from "../lib/shop-copy";
import { ImageIcon } from "./icons";

export default function ProductGallery({
  locale,
  productID,
  name,
  images,
}: {
  locale: Locale;
  productID: string;
  name: string;
  images: { id: string; width: number | null; height: number | null }[];
}) {
  const copy = shopCopy[locale];
  const strip = useRef<HTMLDivElement>(null);
  const [index, setIndex] = useState(0);
  if (images.length === 0)
    return (
      <div className="sf-gal" data-testid="product-gallery">
        <div className="sf-gal__ph" role="img" aria-label={copy.noImage}>
          <ImageIcon size={44} />
          <span aria-hidden="true">{copy.noImage}</span>
        </div>
      </div>
    );
  const go = (i: number) => {
    const el = strip.current;
    if (el) el.scrollTo({ left: i * el.clientWidth, behavior: "smooth" });
    setIndex(i);
  };
  return (
    <div className="sf-gal" data-testid="product-gallery">
      <div
        className="sf-gal__strip"
        ref={strip}
        tabIndex={0}
        aria-label={name}
        onScroll={(event) => {
          const el = event.currentTarget;
          const next = Math.round(el.scrollLeft / Math.max(1, el.clientWidth));
          if (next !== index) setIndex(next);
        }}
      >
        {images.map((image, i) => (
          <img
            key={image.id}
            src={productImage(productID, image.id)}
            alt={i === 0 ? name : ""}
            width={image.width ?? undefined}
            height={image.height ?? undefined}
            loading={i === 0 ? "eager" : "lazy"}
            fetchPriority={i === 0 ? "high" : undefined}
            decoding="async"
          />
        ))}
      </div>
      {images.length > 1 && (
        <>
          <p className="sf-gal__count" aria-hidden="true">
            {index + 1} / {images.length}
          </p>
          <div className="sf-gal__thumbs">
            {images.map((image, i) => (
              <button key={image.id} type="button" aria-label={fmt(copy.photo, { n: i + 1, total: images.length })} aria-current={i === index} onClick={() => go(i)}>
                <img src={productImage(productID, image.id)} alt="" loading="lazy" />
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
