// Purpose: reusable tag fields and mutation status for W6-01B tag/note controls.
// Depends on: React, customer-tags copy/model and in-memory write coordinator.
// Used by: CustomerTags and CustomerNotes; calls no backend directly.
"use client";
import { useId } from "react";
import { tagColors, type Tag } from "../lib/customers-model";
import type { CustomerTagsCopy } from "../lib/customer-tags-copy";
import type { useTagWrite } from "../lib/customer-tags-write";

/** Tag name/colour controls; UTF-16 maxLength is deliberately avoided for the Unicode code-point cap. */
export function TagFields({ c, name, color, disabled, setName, setColor }: {
  c: CustomerTagsCopy; name: string; color: Tag["color"]; disabled: boolean;
  setName: (s: string) => void; setColor: (s: Tag["color"]) => void;
}) {
  const id = useId();
  return <div className="ct-fields">
    <label htmlFor={`${id}-name`}>{c.nameLabel}</label>
    <input id={`${id}-name`} value={name} disabled={disabled} onChange={(e) => setName(e.target.value)} aria-describedby={`${id}-hint`} />
    <small id={`${id}-hint`}>{c.nameHint}</small>
    <label htmlFor={`${id}-color`}>{c.colorLabel}</label>
    <select id={`${id}-color`} value={color} disabled={disabled} onChange={(e) => setColor(e.target.value as Tag["color"])}>
      {tagColors.map((v) => <option key={v} value={v}>{c.colors[v]}</option>)}
    </select>
  </div>;
}

/** Show honest status and a same-request retry only after an UNKNOWN write result. */
export function TagWriteStatus({ c, write, refresh }: { c: CustomerTagsCopy; write: ReturnType<typeof useTagWrite>; refresh?: () => void }) {
  const text = write.error === "refresh_required" ? c.refreshRequired : c.errors[write.error as keyof typeof c.errors] ?? c.errors.default;
  return <div aria-live="polite" className="ct-status">
    {write.busy && <p>{c.saving}</p>}
    {write.error && <p role="alert">{write.uncertain ? c.unknown : text}</p>}
    {write.notice && <p role="status">{write.notice}</p>}
    {write.uncertain && <button type="button" disabled={write.busy} onClick={write.retry}>{c.retry}</button>}
    {refresh && !write.locked && ["version_changed", "refresh_required"].includes(write.error) &&
      <button type="button" onClick={refresh}>{c.refresh}</button>}
  </div>;
}
