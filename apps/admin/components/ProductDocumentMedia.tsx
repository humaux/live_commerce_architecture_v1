"use client";
import { useRef, type Dispatch, type SetStateAction } from "react";
import { PHOTO_ACCEPT, photoFileProblem } from "@/lib/images-client";
import type { ProductEditorCopy } from "@/lib/product-editor-copy";
export type DraftPhoto = { key: string; url: string; file?: File; id?: string };
export function ProductDocumentMedia({
  photos,
  setPhotos,
  disabled,
  c,
  fail,
}: {
  photos: DraftPhoto[];
  setPhotos: Dispatch<SetStateAction<DraftPhoto[]>>;
  disabled: boolean;
  c: ProductEditorCopy;
  fail: (s: string) => void;
}) {
  const dragged = useRef<number | null>(null);
  function add(files: FileList | null) {
    if (!files || disabled) return;
    const added = Array.from(files);
    if (photos.length + added.length > 12 || added.some(photoFileProblem))
      return fail(c.photoInvalid);
    setPhotos((now) => [
      ...now,
      ...added.map((file) => ({
        key: crypto.randomUUID(),
        url: URL.createObjectURL(file),
        file,
      })),
    ]);
  }
  function move(from: number, to: number) {
    if (disabled || to < 0 || to >= photos.length) return;
    setPhotos((now) => {
      const next = [...now];
      next.splice(to, 0, next.splice(from, 1)[0]);
      return next;
    });
  }
  return (
    <section
      id="media"
      className="product-section pe-media"
      aria-labelledby="media-title"
      onDragOver={(e) => e.preventDefault()}
      onDrop={(e) => {
        e.preventDefault();
        if (e.dataTransfer.files.length) add(e.dataTransfer.files);
      }}
    >
      <h2 id="media-title">{c.media}</h2>
      <div className="pe-photos">
        {photos.map((photo, i) => (
          <div
            key={photo.key}
            className="pe-photo"
            draggable={!disabled}
            onDragStart={() => {
              dragged.current = i;
            }}
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
              if (dragged.current !== null) {
                e.preventDefault();
                e.stopPropagation();
                move(dragged.current, i);
                dragged.current = null;
              }
            }}
          >
            <img src={photo.url} alt={`${c.images} ${i + 1}`} />
            {i === 0 && <span className="pe-cover">{c.cover}</span>}
            <div className="pe-photo-actions">
              <button
                type="button"
                aria-label={`${c.left} ${i + 1}`}
                disabled={disabled || i === 0}
                onClick={() => move(i, i - 1)}
              >
                ←
              </button>
              <button
                type="button"
                aria-label={`${c.right} ${i + 1}`}
                disabled={disabled || i === photos.length - 1}
                onClick={() => move(i, i + 1)}
              >
                →
              </button>
              <button
                type="button"
                aria-label={`${c.remove} ${c.images} ${i + 1}`}
                disabled={disabled}
                onClick={() => {
                  if (photo.file) URL.revokeObjectURL(photo.url);
                  setPhotos((now) => now.filter((p) => p.key !== photo.key));
                }}
              >
                ×
              </button>
            </div>
          </div>
        ))}
        <label className="pe-upload">
          <span>{c.addImages}</span>
          <input
            aria-label={c.addImages}
            data-testid="photo-input"
            type="file"
            multiple
            accept={PHOTO_ACCEPT}
            disabled={disabled || photos.length >= 12}
            onChange={(e) => {
              add(e.target.files);
              e.target.value = "";
            }}
          />
        </label>
      </div>
      <p className="pe-hint">{c.imageHelp}</p>
    </section>
  );
}
