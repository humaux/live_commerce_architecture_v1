"use client";

// Product photo thumbnail and the photo manager of the Ledger inspector (catalog-media CM5).
// BFF: GET/POST /api/stores/{store}/products/{product}/images, GET .../images/{image}, POST .../images/{image}/delete
// and .../images/order (lib/images-client.ts) -> Go internal/httpapi/images.go -> catalog.product_images.
// Owns: rendering the real cover (falls back to the demo atlas / initial placeholder when there is none) and the
// upload / delete / move controls. Never decides validity (the server sniffs the bytes) and never reorders locally
// without the server's answer: the list always shows what the server returned last.
import { useCallback, useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { FilePicker } from "@live-commerce/ui";
import { catalogPresentationCopy } from "@/lib/catalog-v2-copy";
import { copy } from "@/lib/copy";
import type { ProductImage } from "@/lib/model";
import {
  MAX_PHOTOS,
  PHOTO_ACCEPT,
  deleteImage,
  imageURL,
  listImages,
  photoFileProblem,
  reorderImages,
  uploadImage,
  type ImageResult,
} from "@/lib/images-client";

const cells: Record<string, number> = {
  "HA-001-BE": 0,
  "AC-002-BK": 1,
  "CL-003-SET": 2,
  "ET-004-S": 3,
  "ET-004-M": 4,
  "ET-004-L": 5,
  "CB-005-TC": 6,
  "AC-006-GY": 7,
  "CL-007-BR": 8,
};
export function ProductPhoto({
  code,
  name,
  demo,
  large = false,
  imageSrc,
}: {
  code: string;
  name: string;
  demo: boolean;
  large?: boolean;
  // Real uploaded photo (same-origin BFF URL); wins over the demo atlas and the placeholder initial.
  imageSrc?: string;
}) {
  const cell = cells[code];
  return (
    <div
      className={`product-photo${large ? " large" : ""}`}
      role="img"
      aria-label={name}
      data-photo={imageSrc ? "real" : undefined}
      style={
        imageSrc
          ? {
              backgroundImage: `url(${imageSrc})`,
              backgroundSize: "cover",
              backgroundPosition: "center",
            }
          : demo && cell !== undefined
            ? {
                backgroundImage:
                  "url(/demo-assets/hearing-aid-accessory-atlas.png)",
                backgroundSize: "300% 300%",
                backgroundPosition: `${(cell % 3) * 50}% ${Math.floor(cell / 3) * 50}%`,
              }
            : undefined
      }
    >
      {!imageSrc && (!demo || cell === undefined) && (
        <span aria-hidden="true">{name.slice(0, 1)}</span>
      )}
    </div>
  );
}

// Photo manager for one product. `onChanged` runs after every committed change so the Ledger refreshes its cover.
export function ProductPhotoManager({
  locale,
  store,
  productID,
  productName,
  code,
  disabled,
  onChanged,
}: {
  locale: Locale;
  store: string;
  productID: string;
  productName: string;
  code: string;
  disabled: boolean;
  onChanged: () => void;
}) {
  const c = copy[locale];
  const [images, setImages] = useState<ProductImage[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<{
    kind: "error" | "success";
    text: string;
  } | null>(null);
  const input = useRef<HTMLInputElement>(null);
  const scope = `${store}:${productID}`;
  const current = useRef(scope);
  current.current = scope;

  useEffect(() => {
    const controller = new AbortController();
    setImages(null);
    setMessage(null);
    void listImages(store, productID, controller.signal).then((result) => {
      if (
        controller.signal.aborted ||
        current.current !== `${store}:${productID}`
      )
        return;
      if (result.items) setImages(result.items);
      else setMessage({ kind: "error", text: c.photosFailed });
    });
    return () => controller.abort();
  }, [store, productID, c]);

  // Every mutation: run, then show exactly what the server answered; a failure re-reads the list so the screen
  // never keeps a state the server did not confirm.
  const run = useCallback(
    async (action: () => Promise<ImageResult>, done: string) => {
      if (busy) return;
      setBusy(true);
      setMessage(null);
      const scopeAtStart = current.current;
      try {
        const result = await action();
        if (current.current !== scopeAtStart) return;
        if (result.items) {
          setImages(result.items);
          setMessage({ kind: "success", text: done });
          onChanged();
        } else {
          setMessage({
            kind: "error",
            text: result.status === 409 ? c.photoLimit : c.photosFailed,
          });
          const fresh = await listImages(store, productID);
          if (fresh.items && current.current === scopeAtStart)
            setImages(fresh.items);
        }
      } finally {
        setBusy(false);
      }
    },
    [busy, c, onChanged, productID, store],
  );

  async function add(file: File | undefined) {
    if (!file) return;
    const problem = photoFileProblem(file);
    if (problem) {
      setMessage({
        kind: "error",
        text: problem === "size" ? c.photoTooLarge : c.photoType,
      });
      return;
    }
    // Upload answers with the new Image; re-read the list so order and positions come from the server.
    await run(async () => {
      const uploaded = await uploadImage(store, productID, file);
      if (uploaded.status !== 200) return uploaded;
      return listImages(store, productID);
    }, c.photoAdded);
    if (input.current) input.current.value = "";
  }

  function move(index: number, by: -1 | 1) {
    if (!images) return;
    const ids = images.map((image) => image.id);
    [ids[index], ids[index + by]] = [ids[index + by], ids[index]];
    void run(() => reorderImages(store, productID, ids), c.photoMoved);
  }

  const full = (images?.length ?? 0) >= MAX_PHOTOS;
  const locked = disabled || busy || images === null;
  return (
    <div className="adjustment photo-manager" data-testid="photo-manager">
      <h2>{c.photos}</h2>
      <p className="audit-hint">{c.photoHelp}</p>
      {message && (
        <p
          className={`message ${message.kind}`}
          role={message.kind === "error" ? "alert" : "status"}
        >
          {message.text}
        </p>
      )}
      {images && images.length === 0 && <p>{c.noPhotos}</p>}
      <ol
        style={{
          listStyle: "none",
          margin: 0,
          padding: 0,
          display: "grid",
          gap: 8,
        }}
      >
        {images?.map((image, index) => (
          <li
            key={image.id}
            data-testid="photo-row"
            style={{
              display: "flex",
              gap: 8,
              alignItems: "center",
              flexWrap: "wrap",
            }}
          >
            <ProductPhoto
              code={code}
              name={`${productName} #${index + 1}`}
              demo={false}
              large
              imageSrc={imageURL(store, productID, image.id)}
            />
            {index === 0 && <small>{c.photoCover}</small>}
            <button
              type="button"
              disabled={locked || index === 0}
              onClick={() => move(index, -1)}
            >
              {c.moveUp}
            </button>
            <button
              type="button"
              disabled={locked || index === images.length - 1}
              onClick={() => move(index, 1)}
            >
              {c.moveDown}
            </button>
            <button
              type="button"
              disabled={locked}
              onClick={() =>
                void run(
                  () => deleteImage(store, productID, image.id),
                  c.photoRemoved,
                )
              }
            >
              {c.deletePhoto}
            </button>
          </li>
        ))}
      </ol>
      <FilePicker
        label={c.addPhoto}
        emptyLabel={catalogPresentationCopy[locale].noFile}
        fileName=""
        inputRef={input}
        accept={PHOTO_ACCEPT}
        data-testid="photo-input"
        disabled={locked || full}
        onChange={(event) => void add(event.target.files?.[0])}
      />
      {full && <p className="audit-hint">{c.photoLimit}</p>}
    </div>
  );
}
