// Figma local links: https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X?node-id=6-22038 and https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X?node-id=6-22147
// Purpose: signed 媒体上传 picker: choose this product's real detail images for the main gallery, or upload new photos.
// Depends on: react, Locale from @live-commerce/i18n, ./MediaAddPicker.copy, ./MediaAddPicker.module.css.
// Used by: ProductMediaManager in picking mode (CatalogSimplifiedEditor's add-images screen).
// Invariants: tiles come only from the canonical image list (never placeholder content); the parent owns every write, lock and recovery.
"use client";
import { useState, type ReactNode } from "react";
import type { Locale } from "@live-commerce/i18n";
import { mediaAddPickerCopy } from "./MediaAddPicker.copy";
import s from "./MediaAddPicker.module.css";

export type PickerTile = { key: string; url: string; label: string };

/** Per-tile selection over real images; the parent applies the selection or the uploaded files. */
export function MediaAddPicker({ locale, tiles, disabled, busy, accept, uploadLabel, note, onAdd, onUpload, onCancel }: {
  locale: Locale; tiles: PickerTile[]; disabled: boolean; busy: boolean; accept: string; uploadLabel: string; note: ReactNode;
  onAdd: (keys: string[]) => void; onUpload: (files: FileList | null) => void; onCancel: () => void;
}) {
  const c = mediaAddPickerCopy[locale];
  const [chosen, setChosen] = useState<string[]>([]);
  const live = chosen.filter((key) => tiles.some((tile) => tile.key === key)); // a re-read can drop a chosen image
  const toggle = (key: string) => setChosen(live.includes(key) ? live.filter((k) => k !== key) : [...live, key]);
  return (
    <section className={s.picker} data-figma-node="6:22038 6:22147" data-testid="media-add-picker" aria-labelledby="media-add-title">
      <h2 id="media-add-title">{c.title}</h2>
      <p className={s.hint}>{c.hint}</p>
      <ul className={s.grid} data-testid="media-add-grid">
        {tiles.map((tile) => (
          <li key={tile.key} className={s.tile} data-testid="media-add-tile">
            <img src={tile.url} alt={tile.label} />
            <div>
              <span>{tile.label}</span>
              <button type="button" aria-pressed={live.includes(tile.key)} disabled={disabled} onClick={() => toggle(tile.key)}>{c.select}</button>
            </div>
          </li>
        ))}
      </ul>
      <label className={s.upload}>
        <span>{uploadLabel}</span>
        <input type="file" accept={accept} multiple disabled={disabled} data-testid="media-add-upload"
          onChange={(e) => { onUpload(e.target.files); e.target.value = ""; }} />
      </label>
      {note}
      <div className={s.dock}>
        <button type="button" disabled={busy} onClick={onCancel}>{c.cancel}</button>
        <button type="button" className={s.primary} disabled={disabled || live.length === 0} onClick={() => onAdd(live)}>{c.apply}</button>
      </div>
    </section>
  );
}
