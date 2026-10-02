"use client";

// Product photo gallery of the product page: a scroll-snap strip (swipe on phones, wheel/arrows on desktop) plus a thumbnail
// row that scrolls to a photo. Calls no BFF itself: the photos come from the server-rendered catalog-v2 detail
// (GET /v1/buyer/catalog/v2/products/{slug}, `images`) and the bytes load from /media/p/{product}/{image}
// (app/media/p/[productID]/[imageID]/route.ts -> Go /v1/buyer/media/p/...). Without photos it shows a neutral placeholder so
// the page layout does not collapse. Zoom uses a native dialog; images stay on the Host-scoped media proxy.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { productImage } from "../lib/routes";
import { fmt, shopCopy } from "../lib/shop-copy";
import { ChevronLeftIcon, ChevronRightIcon, CloseIcon, ImageIcon, SearchIcon } from "./icons";
import { browseCopy } from "../lib/browse-copy";

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
  const dialog = useRef<HTMLDialogElement>(null);
  const [open, setOpen] = useState(false);
  const [zoomIndex, setZoomIndex] = useState(0);
  const zoomCopy = browseCopy[locale];
  useEffect(() => {
    const el = dialog.current;
    if (!el) return;
    if (!open) { if (el.open) el.close(); return; }
    if (!el.open) el.showModal();
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => { document.body.style.overflow = previous; };
  }, [open]);
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
        role="group"
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
      <button type="button" className="sf-gal__open" aria-haspopup="dialog" onClick={() => { setZoomIndex(index); setOpen(true); }}>
        <SearchIcon />{zoomCopy.zoom}
      </button>
      <dialog ref={dialog} className="sf-photo-dialog" aria-label={`${name} — ${zoomCopy.zoom}`} onClose={() => setOpen(false)}
        onClick={event => { if (event.target === event.currentTarget) setOpen(false); }}
        onKeyDown={event => {
          if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
            event.preventDefault();
            setZoomIndex(i => Math.max(0, Math.min(images.length - 1, i + (event.key === "ArrowRight" ? 1 : -1))));
          }
        }}>
        <div className="sf-photo-dialog__head">
          <span>{name}</span>
          <button type="button" className="sf-iconbtn" aria-label={zoomCopy.close} onClick={() => setOpen(false)}><CloseIcon /></button>
        </div>
        {open && <img src={productImage(productID, images[zoomIndex].id)} alt={`${name} — ${fmt(copy.photo, { n: zoomIndex + 1, total: images.length })}`} />}
        <div className="sf-photo-dialog__controls">
          <button type="button" className="sf-iconbtn" aria-label={zoomCopy.previous} disabled={zoomIndex === 0} onClick={() => setZoomIndex(i => i - 1)}><ChevronLeftIcon /></button>
          <span aria-live="polite">{zoomIndex + 1} / {images.length}</span>
          <button type="button" className="sf-iconbtn" aria-label={zoomCopy.next} disabled={zoomIndex === images.length - 1} onClick={() => setZoomIndex(i => i + 1)}><ChevronRightIcon /></button>
        </div>
      </dialog>
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
