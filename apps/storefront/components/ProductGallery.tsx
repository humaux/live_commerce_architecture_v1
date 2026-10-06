"use client";

// Purpose: show up to four main photos, with a selected option photo in the active first slot, and accessible zoom.
// Depends on: catalog-v2 main/variant image metadata, product-media.ts, scoped /media/p/{product}/{image} and native dialog.
// Used by: ProductMedia on /{locale}/products/{slug}; detail-role images render separately below the description.
// Product photo gallery of the product page: a scroll-snap strip (swipe on phones, wheel/arrows on desktop) plus a thumbnail
// row that scrolls to a photo. Calls no BFF itself: the photos come from the server-rendered catalog-v2 detail
// (GET /v1/buyer/catalog/v2/products/{slug}, `images`) and the bytes load from /media/p/{product}/{image}
// (app/media/p/[productID]/[imageID]/route.ts -> Go /v1/buyer/media/p/...). Without photos it shows a neutral placeholder so
// the page layout does not collapse. Zoom uses a native dialog; images stay on the Host-scoped media proxy.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { productImage, productImageSet } from "../lib/routes";
import type { ProductDetail } from "../lib/shop-contract";
import { fmt, shopCopy } from "../lib/shop-copy";
import { ChevronLeftIcon, ChevronRightIcon, CloseIcon, ImageIcon, SearchIcon } from "./icons";
import { browseCopy } from "../lib/browse-copy";
import { mainGallerySlides } from "../lib/product-media";

/** Render swipe/keyboard/thumbnail navigation and native zoom; no API writes or price/stock decisions. */
export default function ProductGallery({
  locale,
  productID,
  name,
  images,
  selectedImageID = null,
  selectionKey = "",
}: {
  locale: Locale;
  productID: string;
  name: string;
  images: ProductDetail["images"];
  selectedImageID?: string | null;
  selectionKey?: string;
}) {
  const copy = shopCopy[locale];
  const strip = useRef<HTMLDivElement>(null);
  const [index, setIndex] = useState(0);
  const [optionImageID, setOptionImageID] = useState(selectedImageID);
  const dialog = useRef<HTMLDialogElement>(null);
  const [open, setOpen] = useState(false);
  const [zoomIndex, setZoomIndex] = useState(0);
  const zoomCopy = browseCopy[locale];
  const slides = mainGallerySlides(images, optionImageID);
  useEffect(() => {
    // A new SKU selection restores its option photo even after the buyer browsed a main thumbnail.
    setOptionImageID(selectedImageID);
    setIndex(0);
    setZoomIndex(0);
    strip.current?.scrollTo({ left: 0, behavior: "instant" });
  }, [selectedImageID, selectionKey]);
  useEffect(() => {
    const el = dialog.current;
    if (!el) return;
    if (!open) { if (el.open) el.close(); return; }
    if (!el.open) el.showModal();
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => { document.body.style.overflow = previous; };
  }, [open]);
  if (slides.length === 0)
    return (
      <div className="sf-gal" data-testid="product-gallery">
        <div className="sf-gal__ph" role="img" aria-label={copy.noImage}>
          <ImageIcon size={44} />
          <span aria-hidden="true">{copy.noImage}</span>
        </div>
      </div>
    );
  const go = (i: number) => {
    setOptionImageID(null);
    const el = strip.current;
    if (el) el.scrollTo({ left: i * el.clientWidth, behavior: "smooth" });
    setIndex(i);
  };
  return (
    <div className="sf-gal" data-testid="product-gallery">
      <div
        className="sf-gal__strip"
        ref={strip}
        role="group"
        tabIndex={0}
        aria-label={name}
        onKeyDown={event => {
          if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
          event.preventDefault();
          const next = event.key === "Home" ? 0 : event.key === "End" ? slides.length - 1 :
            Math.max(0, Math.min(slides.length - 1, index + (event.key === "ArrowRight" ? 1 : -1)));
          go(next);
        }}
        onScroll={(event) => {
          const el = event.currentTarget;
          const next = Math.max(0, Math.min(slides.length - 1, Math.round(el.scrollLeft / Math.max(1, el.clientWidth))));
          if (next > 0) setOptionImageID(null);
          if (next !== index) setIndex(next);
        }}
      >
        {slides.map((image, i) => (
          <img
            key={image.id}
            src={productImage(productID, image.id)}
            srcSet={productImageSet(productID, image.id, image.sizes)}
            sizes="(min-width: 1280px) 608px, (min-width: 900px) 50vw, 100vw"
            alt={`${name} — ${fmt(copy.photo, { n: i + 1, total: slides.length })}`}
            data-testid={i === index ? "product-gallery-active-image" : undefined}
            data-image-id={image.id}
            width={image.width ?? undefined}
            height={image.height ?? undefined}
            loading={i === 0 ? "eager" : "lazy"}
            fetchPriority={i === 0 ? "high" : undefined}
            decoding="async"
          />
        ))}
      </div>
      <button type="button" className="sf-gal__open" aria-haspopup="dialog" onClick={() => { setZoomIndex(index); setOpen(true); }}>
        <SearchIcon />{zoomCopy.zoom}
      </button>
      <dialog ref={dialog} className="sf-photo-dialog" aria-label={`${name} — ${zoomCopy.zoom}`} onClose={() => setOpen(false)}
        onClick={event => { if (event.target === event.currentTarget) setOpen(false); }}
        onKeyDown={event => {
          if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
            event.preventDefault();
            setZoomIndex(i => Math.max(0, Math.min(slides.length - 1, i + (event.key === "ArrowRight" ? 1 : -1))));
          }
        }}>
        <div className="sf-photo-dialog__head">
          <span>{name}</span>
          <button type="button" className="sf-iconbtn" aria-label={zoomCopy.close} onClick={() => setOpen(false)}><CloseIcon /></button>
        </div>
        {open && <img src={productImage(productID, (slides[zoomIndex] ?? slides[0]).id)} alt={`${name} — ${fmt(copy.photo, { n: zoomIndex + 1, total: slides.length })}`} />}
        <div className="sf-photo-dialog__controls">
          <button type="button" className="sf-iconbtn" aria-label={zoomCopy.previous} disabled={zoomIndex === 0} onClick={() => setZoomIndex(i => i - 1)}><ChevronLeftIcon /></button>
          <span aria-live="polite">{zoomIndex + 1} / {slides.length}</span>
          <button type="button" className="sf-iconbtn" aria-label={zoomCopy.next} disabled={zoomIndex === slides.length - 1} onClick={() => setZoomIndex(i => i + 1)}><ChevronRightIcon /></button>
        </div>
      </dialog>
      {images.length > 1 && (
        <>
          <p className="sf-gal__count" aria-hidden="true">
            {index + 1} / {images.length}
          </p>
          <div className="sf-gal__thumbs">
            {images.map((image, i) => (
              <button key={image.id} type="button" data-testid="product-main-thumbnail" data-image-id={image.id}
                aria-label={fmt(copy.photo, { n: i + 1, total: images.length })} aria-current={image.id === slides[index]?.id} onClick={() => go(i)}>
                <img src={productImage(productID, image.id)} srcSet={productImageSet(productID, image.id, image.sizes)} sizes="64px" alt="" loading="lazy" />
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
