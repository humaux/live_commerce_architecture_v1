"use client";

// Image picker of the Design editor (store profile logo/favicon, hero and image-text sections).
// BFF: GET /api/stores/{store}/design/media/{id} (thumbnail bytes), POST .../design/media (multipart upload),
// POST .../design/media/{id}/delete -> Go internal/httpapi/design.go -> design.store_media. The library itself
// (list, upload, delete) is owned by Design.tsx and arrives as `media` + `ops`; this file only renders it. It never decides
// whether a file is valid (the server sniffs the bytes; mediaFileProblem is just an early hint) and an image id is only
// ever one of the library's ids.
import { useState } from "react";
import { mediaURL, MEDIA_ACCEPT } from "@/lib/design-client";
import { fill, type DesignCopy } from "@/lib/design-copy";
import type { MediaItem } from "@/lib/design-model";

export type MediaOps = {
  /** Resolves to the new image id, or null after showing its own message. */
  upload: (file: File) => Promise<string | null>;
  remove: (id: string) => Promise<void>;
};

export function ImagePicker({
  store, label, value, media, ops, c, error, onChange, testId,
}: {
  store: string; label: string; value: string | null; media: MediaItem[]; ops: MediaOps; c: DesignCopy; error?: string;
  onChange: (id: string | null) => void; testId: string;
}) {
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const current = media.find((item) => item.id === value);
  async function pickFile(file: File | undefined) {
    if (!file || busy) return;
    setBusy(true);
    const id = await ops.upload(file);
    setBusy(false);
    if (id) {
      onChange(id);
      setOpen(false);
    }
  }
  return (
    <div className="design-field design-picker" data-testid={testId}>
      <span className="design-label">{label}</span>
      <div className="design-picker-row">
        <div className="design-thumb" aria-hidden={!current}>
          {/* eslint-disable-next-line @next/next/no-img-element -- same-origin BFF bytes behind the session cookie */}
          {current ? <img src={mediaURL(store, current.id)} alt={c.media.alt} /> : null}
        </div>
        <div className="design-picker-actions">
          <button type="button" onClick={() => setOpen(!open)} aria-expanded={open} data-testid={`${testId}-choose`}>
            {value ? c.media.change : c.media.choose}
          </button>
          {value && (
            <button type="button" onClick={() => onChange(null)} data-testid={`${testId}-clear`}>{c.media.remove}</button>
          )}
        </div>
      </div>
      {error && <small className="design-error" role="alert">{error}</small>}
      {open && (
        <div className="design-library" role="group" aria-label={fill(c.media.library, { n: media.length })}>
          <p className="design-muted">{fill(c.media.library, { n: media.length })}</p>
          {media.length === 0 && <p className="design-muted">{c.media.none}</p>}
          <div className="design-grid">
            {media.map((item) => (
              <div key={item.id} className={item.id === value ? "design-tile selected" : "design-tile"}>
                <button type="button" aria-label={c.media.pick} aria-pressed={item.id === value} onClick={() => { onChange(item.id); setOpen(false); }}>
                  {/* eslint-disable-next-line @next/next/no-img-element -- same-origin BFF bytes behind the session cookie */}
                  <img src={mediaURL(store, item.id)} alt={c.media.alt} loading="lazy" />
                </button>
                <button type="button" className="design-tile-delete" aria-label={c.media.delete} title={c.media.delete} onClick={() => void ops.remove(item.id)}>×</button>
              </div>
            ))}
          </div>
          <label className="design-upload">
            <span>{busy ? c.media.uploading : c.media.upload}</span>
            <input type="file" accept={MEDIA_ACCEPT} disabled={busy} data-testid={`${testId}-file`}
              onChange={(event) => { const file = event.target.files?.[0]; event.target.value = ""; void pickFile(file); }} />
          </label>
        </div>
      )}
    </div>
  );
}
