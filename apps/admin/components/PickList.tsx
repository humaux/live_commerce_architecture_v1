// Purpose: selection toolbar and SKU-first printable pick list on the existing orders surface.
// Depends on: native dialog/print, picklist-client/model/copy, CarrierExport and CvsBatch.
// Used by: MerchantOrders; read-only session selection never expands CVS command scope.
"use client";
import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { Locale } from "@live-commerce/i18n";
import { readPickList, PickError } from "@/lib/picklist-client";
import {
  pickUUID,
  type PickDocument,
  type PickLine,
  type PickSelection,
} from "@/lib/picklist-model";
import { displayTime } from "@/lib/orders-model";
import { picklistCopy } from "@/lib/picklist-copy";
import { CarrierExport } from "./CarrierExport";
import { CvsBatch } from "./CvsBatch";
import "./picklist.css";
/** Print a server projection; selection metadata contains IDs only and clears with its session scope. */
export function PickList({
  locale,
  store,
  boundary,
  ids,
  cvsIDs,
  sessionID,
  canExport,
  canShip,
  disabled,
  onClear,
  onViewOrder,
}: {
  locale: Locale;
  store: string;
  boundary: string;
  ids: string[];
  cvsIDs: string[];
  sessionID: string;
  canExport: boolean;
  canShip: boolean;
  disabled: boolean;
  onClear: () => void;
  onViewOrder: (id: string) => void;
}) {
  const c = picklistCopy[locale];
  const [scope, setScope] = useState("chosen"),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [document, setDocument] = useState<PickDocument | null>(null);
  const dialog = useRef<HTMLDialogElement>(null),
    alive = useRef(true),
    flight = useRef(false);
  const selection: PickSelection | null =
    scope === "session"
      ? pickUUID.test(sessionID)
        ? { session_id: sessionID }
        : null
      : ids.length
        ? { order_ids: ids }
        : null;
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  useEffect(() => {
    if (document) dialog.current?.showModal();
  }, [document]);
  async function preview() {
    if (!selection || flight.current) return;
    flight.current = true;
    setBusy(true);
    setError("");
    try {
      const value = await readPickList(store, selection, boundary);
      if (alive.current) setDocument(value);
    } catch (e) {
      if (alive.current)
        setError(
          e instanceof PickError && e.code === "too_many"
            ? c.tooMany
            : e instanceof PickError && e.code === "unauthorized"
              ? c.unauthorized
              : c.unavailable,
        );
    } finally {
      flight.current = false;
      if (alive.current) setBusy(false);
    }
  }
  return (
    <section
      className="pick-toolbar"
      aria-label={c.pick}
      data-testid="pick-toolbar"
      hidden={disabled}
    >
      <div className="pick-controls">
        <p>
          {c.selected}: <strong data-testid="pick-count">{ids.length}</strong> /
          500
        </p>
        <button type="button" disabled={!ids.length} onClick={onClear}>
          {c.clear}
        </button>
        <label>
          {c.scope}
          <select value={scope} onChange={(e) => setScope(e.target.value)}>
            <option value="chosen">{c.chosen}</option>
            <option value="session" disabled={!pickUUID.test(sessionID)}>
              {c.session}
            </option>
          </select>
        </label>
        <button type="button" onClick={preview} disabled={busy || !selection}>
          {busy ? c.loading : c.pick}
        </button>
        {canExport && (
          <CarrierExport
            locale={locale}
            store={store}
            boundary={boundary}
            selection={selection}
            disabled={disabled}
          />
        )}
      </div>
      {ids.length === 500 && <p role="status">{c.limit}</p>}
      {scope === "session" && <p>{c.sessionHint}</p>}
      {error && <p role="alert">{error}</p>}
      {canShip && (
        <CvsBatch
          locale={locale}
          store={store}
          boundary={boundary}
          ids={cvsIDs}
          onViewOrder={onViewOrder}
          disabled={disabled}
        />
      )}
      {document &&
        createPortal(
          <div className="pick-print-root">
            <dialog
              ref={dialog}
              className="pick-dialog"
              aria-labelledby="pick-heading"
              onCancel={() => setDocument(null)}
              onClose={() => setDocument(null)}
            >
              <div className="pick-dialog-controls">
                <button type="button" onClick={() => window.print()}>
                  {c.print}
                </button>
                <button type="button" onClick={() => dialog.current?.close()}>
                  {c.close}
                </button>
              </div>
              <h2 id="pick-heading">{c.pick}</h2>
              <p>{displayTime(locale, document.generated_at)}</p>
              {!document.orders.length ? (
                <p>{c.empty}</p>
              ) : (
                <>
                  <h3>{c.summary}</h3>
                  <LineTable locale={locale} rows={document.totals} />
                  <h3>{c.orders}</h3>
                  {document.orders.map((order) => (
                    <section className="pick-order" key={order.order_id}>
                      <h4>{order.order_number}</h4>
                      <LineTable locale={locale} rows={order.lines} />
                    </section>
                  ))}
                </>
              )}
              {!!document.skipped.length && (
                <section>
                  <h3>{c.skipped}</h3>
                  <ul>
                    {document.skipped.map((row) => (
                      <li key={row.order_id}>
                        {row.order_id} —{" "}
                        {row.code === "not_pickable"
                          ? c.notPickable
                          : c.notFound}
                      </li>
                    ))}
                  </ul>
                </section>
              )}
            </dialog>
          </div>,
          globalThis.document.body,
        )}
    </section>
  );
}
function LineTable({ locale, rows }: { locale: Locale; rows: PickLine[] }) {
  const c = picklistCopy[locale];
  return (
    <table className="pick-lines">
      <thead>
        <tr>
          <th>{c.sku}</th>
          <th>{c.item}</th>
          <th>{c.quantity}</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => (
          <tr key={`${row.sku_id}:${i}`}>
            <td>{row.sku_code}</td>
            <td>{row.title}</td>
            <td>{row.qty}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
