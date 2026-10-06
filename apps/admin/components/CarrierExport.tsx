// Purpose: explicit download of server-produced carrier CSV without reformatting buyer data.
// Depends on: picklist-client, locale copy and native Blob download.
// Used by: PickList order toolbar; requires server-reported orders_export permission.
"use client";
import { useState, useRef, useEffect, useId } from "react";
import type { Locale } from "@live-commerce/i18n";
import {
  carrierTemplates,
  type CarrierTemplate,
  type PickSelection,
} from "@/lib/picklist-model";
import { exportCarrier, PickError } from "@/lib/picklist-client";
import { picklistCopy } from "@/lib/picklist-copy";
/** Download one chosen carrier format; revoke local bytes and ignore results after unmount. */
export function CarrierExport({
  locale,
  store,
  boundary,
  selection,
  disabled,
}: {
  locale: Locale;
  store: string;
  boundary: string;
  selection: PickSelection | null;
  disabled: boolean;
}) {
  const c = picklistCopy[locale];
  const templateID = useId();
  const dialog = useRef<HTMLDialogElement>(null);
  const [template, setTemplate] = useState<CarrierTemplate>("black_cat");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  async function download() {
    if (!selection || busy) return;
    setBusy(true);
    setError("");
    try {
      const { blob, filename } = await exportCarrier(
        store,
        selection,
        boundary,
        template,
      );
      if (!alive.current) return;
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = filename;
      document.body.append(anchor);
      anchor.click();
      anchor.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      if (alive.current)
        setError(
          e instanceof PickError && e.code === "unauthorized"
            ? c.unauthorized
            : e instanceof PickError && e.code === "too_many"
              ? c.tooMany
              : c.unavailable,
        );
    } finally {
      if (alive.current) setBusy(false);
    }
  }
  return (
    <>
      <button
        type="button"
        disabled={disabled || !selection}
        onClick={() => dialog.current?.showModal()}
      >
        {c.export}
      </button>
      <dialog
        ref={dialog}
        className="pick-dialog"
        aria-labelledby="carrier-export-title"
      >
        <h2 id="carrier-export-title">{c.export}</h2>
        <button type="button" onClick={() => dialog.current?.close()}>
          {c.close}
        </button>
        <fieldset className="pick-export" disabled={disabled || busy}>
          <legend>{c.export}</legend>
          <div className="pick-field">
            <label htmlFor={templateID}>{c.template}</label>
            <select
              id={templateID}
              value={template}
              onChange={(e) => setTemplate(e.target.value as CarrierTemplate)}
            >
              {carrierTemplates.map((t) => (
                <option key={t} value={t}>
                  {c[t]}
                </option>
              ))}
            </select>
          </div>
          <button type="button" onClick={download} disabled={!selection}>
            {busy ? c.loading : c.download}
          </button>
          <small>{c.exportHint}</small>
          {error && <p role="alert">{error}</p>}
        </fieldset>
      </dialog>
    </>
  );
}
