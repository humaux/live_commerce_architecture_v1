// Purpose: role-local photo tiles with keyboard and real drag ordering plus deletion/move controls.
// Depends on: React drag events and product-media copy; no transport/write of its own.
// Used by: ProductDocumentMedia and ProductMediaManager; per-role ordering stays with the caller.
"use client";
import { useRef } from "react";
import type { ProductMediaCopy } from "@/lib/product-media-copy";
import type { MediaRole } from "@/lib/product-media-model";
export type MediaTile = {
  key: string;
  url: string;
  width?: number | null;
  height?: number | null;
};
/** Render one role's committed/draft photos; callbacks own all mutation semantics. */
export function ProductMediaTiles({
  role,
  photos,
  disabled,
  c,
  onMove,
  onDelete,
  onRole,
}: {
  role: MediaRole;
  photos: MediaTile[];
  disabled: boolean;
  c: ProductMediaCopy;
  onMove: (from: number, to: number) => void;
  onDelete: (key: string) => void;
  onRole?: (key: string, role: "main" | "detail") => void;
}) {
  const dragged = useRef<number | null>(null);
  return (
    <div className={`pm-grid pm-${role}`} data-testid={`media-${role}-list`}>
      {photos.map((photo, i) => (
        <div
          className="pm-photo"
          data-testid={role === "main" ? "photo-row" : `photo-row-${role}`}
          key={photo.key}
          data-image-id={photo.key}
          draggable={!disabled && role !== "sku"}
          onDragStart={(e) => {
            dragged.current = i;
            e.dataTransfer.setData("text/plain", photo.key);
          }}
          onDragEnd={() => {
            dragged.current = null;
          }}
          onDragOver={(e) => {
            if (!disabled) e.preventDefault();
          }}
          onDrop={(e) => {
            if (disabled || dragged.current === null) return;
            e.preventDefault();
            e.stopPropagation();
            onMove(dragged.current, i);
            dragged.current = null;
          }}
        >
          <img
            src={photo.url}
            width={photo.width ?? undefined}
            height={photo.height ?? undefined}
            draggable={false}
            alt={`${c[role]} ${i + 1}`}
          />
          {role === "main" && i === 0 && (
            <span className="pm-cover">{c.cover}</span>
          )}
          <div className="pm-photo-actions">
            {role !== "sku" && (
              <>
                <button
                  type="button"
                  aria-label={`${c.earlier} ${c[role]} ${i + 1}`}
                  disabled={disabled || i === 0}
                  onClick={() => onMove(i, i - 1)}
                >
                  ←
                </button>
                <button
                  type="button"
                  aria-label={`${c.later} ${c[role]} ${i + 1}`}
                  disabled={disabled || i === photos.length - 1}
                  onClick={() => onMove(i, i + 1)}
                >
                  →
                </button>
              </>
            )}
            <button
              type="button"
              aria-label={`${c.remove} ${c[role]} ${i + 1}`}
              disabled={disabled}
              onClick={() => onDelete(photo.key)}
            >
              ×
            </button>
          </div>
          {onRole && role !== "sku" && (
            <button
              className="pm-role-move"
              type="button"
              disabled={disabled}
              onClick={() =>
                onRole(photo.key, role === "main" ? "detail" : "main")
              }
            >
              {role === "main" ? c.toDetail : c.toMain}
            </button>
          )}
        </div>
      ))}
    </div>
  );
}
