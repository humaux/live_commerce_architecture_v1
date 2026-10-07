"use client";

// Purpose: W3-07B parcel-merge UI on the merchant orders page: the merge-suggestion banner (N groups can ship as one
//   parcel), the suggestion cards with the 合併 action and the per-group panel (members, 填寫運單, 解除合併, close).
//   The owner ruling sentence (parcels only, never payments/amounts; every order priced and notified on its own) is
//   always visible next to the banner. Groups are session knowledge: create/ship answers carry the membership, and no
//   group list route exists, so a reload drops the panels (the server guard in_parcel_group still covers stale edits).
// Depends on: @/lib/parcels-client (BFF orders/merge-suggestions + parcel-groups* -> Go internal/httpapi/parcels.go),
//   @/lib/parcels-model, @/lib/parcels-copy, ./OrderShipment (ShipmentFields + shipmentFieldsError), ./orders.css classes.
// Used by: apps/admin/components/MerchantOrders.tsx (rendered above the orders table, groups state lifted there for badges).
// Invariants: every mutating click sends one Idempotency-Key per logical write (an uncertain retry reuses it); dissolve is
//   CAS-guarded by the group version; COD/pay-at-pickup/CVS orders never appear (the server excludes them; create would
//   refuse with cod_not_mergeable/cvs_not_mergeable).

import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import {
  createParcelGroup,
  dissolveParcelGroup,
  readMergeSuggestions,
  shipParcelGroup,
} from "@/lib/parcels-client";
import { parcelShort, type MergeSuggestion } from "@/lib/parcels-model.ts";
import type { ParcelCopy } from "@/lib/parcels-copy";
import type { OrdersCopy } from "@/lib/orders-copy";
import { ShipmentFields, shipmentFieldsError, type ShipmentFieldValues } from "./OrderShipment";

// A group this session created (or shipped/dissolved); the only group knowledge the frozen API gives the UI.
export type ParcelGroupView = {
  id: string;
  short: string;
  state: "OPEN" | "SHIPPED" | "DISSOLVED";
  version: number;
  orderIDs: string[];
};

const emptyFields: ShipmentFieldValues = { carrier: "", carrierName: "", tracking: "", url: "", note: "" };

function errorText(c: ParcelCopy, code: string) {
  return (c.errors as Record<string, string>)[code] ?? c.errors.default;
}

/** Banner + suggestion cards + group panels. Reads suggestions; every write re-reads orders via onChanged. */
export function ParcelMerge({
  locale: _locale,
  store,
  boundary,
  disabled,
  c,
  shipmentCopy,
  groups,
  onGroups,
  onChanged,
}: {
  locale: Locale;
  store: string;
  boundary: string;
  disabled: boolean;
  c: ParcelCopy;
  shipmentCopy: OrdersCopy;
  groups: ParcelGroupView[];
  onGroups: (next: ParcelGroupView[]) => void;
  onChanged: () => void;
}) {
  const [suggestions, setSuggestions] = useState<MergeSuggestion[]>([]);
  const [tick, setTick] = useState(0);
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");
  // One Idempotency-Key per logical create; only a byte-identical retry after an unknown outcome reuses it.
  const pending = useRef<{ key: string; body: string } | null>(null);

  useEffect(() => {
    const active = new AbortController();
    readMergeSuggestions(store, active.signal).then(
      (value) => setSuggestions(value),
      () => {
        if (!active.signal.aborted) setSuggestions([]); // a failed probe hides the banner; the list below stays authoritative
      },
    );
    return () => active.abort();
  }, [store, boundary, tick]);

  async function merge(suggestion: MergeSuggestion) {
    if (busy) return;
    const orderIDs = [...suggestion.order_ids].sort();
    const body = orderIDs.join(",");
    if (pending.current?.body !== body) pending.current = { key: `parcel-${crypto.randomUUID()}`, body };
    setBusy(true);
    setProblem("");
    // Calls BFF POST parcel-groups -> Go fulfillment.create_parcel_group (manual-fulfilment-v1 Amendment W3-07B); idempotency key above.
    const result = await createParcelGroup(store, pending.current.key, orderIDs, boundary);
    setBusy(false);
    if (result.ok) {
      pending.current = null;
      onGroups([
        ...groups,
        { id: result.value.id, short: parcelShort(result.value.id), state: "OPEN", version: result.value.version, orderIDs: result.value.order_ids },
      ]);
      setTick((v) => v + 1);
      return;
    }
    if (result.uncertain) {
      setProblem(c.shipUncertain);
      return; // pending.current kept: the retry sends the identical request
    }
    pending.current = null;
    setProblem(errorText(c, result.code));
    setTick((v) => v + 1);
  }

  function replace(next: ParcelGroupView) {
    onGroups(groups.map((g) => (g.id === next.id ? next : g)));
  }
  function dismiss(id: string) {
    onGroups(groups.filter((g) => g.id !== id));
  }

  if (suggestions.length === 0 && groups.length === 0) return null;
  return (
    <>
      {suggestions.length > 0 && (
        <section className="orders-section" data-testid="parcel-merge" aria-label={c.banner(suggestions.length)}>
          <p>
            <strong data-testid="parcel-merge-count">{c.banner(suggestions.length)}</strong>{" "}
            <button
              type="button"
              className="orders-compact"
              data-testid="parcel-suggestions-toggle"
              aria-expanded={open}
              onClick={() => setOpen((v) => !v)}
            >
              {open ? c.hide : c.show}
            </button>
          </p>
          <p className="orders-hint" data-testid="parcel-merge-rule">{c.rule}</p>
          {problem && <p className="orders-bad" role="alert" data-testid="parcel-merge-problem">{problem}</p>}
          {open && (
            <ul>
              {suggestions.map((s) => (
                <li key={s.order_ids[0]} data-testid="parcel-suggestion">
                  <span>{c.suggestionFor}: {s.recipient_name}</span>{" "}
                  <span>{c.suggestionOrders(s.order_ids.length)}</span>
                  <ul>
                    {s.order_ids.map((id) => (
                      <li key={id} className="orders-mono" data-testid={`parcel-member-${id}`}>{parcelShort(id)}…</li>
                    ))}
                  </ul>
                  <button
                    type="button"
                    className="primary"
                    data-testid={`parcel-merge-${s.order_ids[0]}`}
                    disabled={busy || disabled}
                    onClick={() => void merge(s)}
                  >
                    {busy ? c.merging : c.merge}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>
      )}
      {groups.map((g) => (
        <ParcelGroupPanel
          key={g.id}
          group={g}
          store={store}
          boundary={boundary}
          disabled={disabled}
          c={c}
          shipmentCopy={shipmentCopy}
          onReplace={replace}
          onDismiss={dismiss}
          onChanged={onChanged}
          onSuggestions={() => setTick((v) => v + 1)}
        />
      ))}
    </>
  );
}

/** One group panel: members, the group waybill form (record only) and the CAS-guarded dissolve. */
function ParcelGroupPanel({
  group,
  store,
  boundary,
  disabled,
  c,
  shipmentCopy,
  onReplace,
  onDismiss,
  onChanged,
  onSuggestions,
}: {
  group: ParcelGroupView;
  store: string;
  boundary: string;
  disabled: boolean;
  c: ParcelCopy;
  shipmentCopy: OrdersCopy;
  onReplace: (next: ParcelGroupView) => void;
  onDismiss: (id: string) => void;
  onChanged: () => void;
  onSuggestions: () => void;
}) {
  const [fields, setFields] = useState<ShipmentFieldValues>(emptyFields);
  const [shipOpen, setShipOpen] = useState(false);
  const [dissolveConfirm, setDissolveConfirm] = useState(false);
  const [busy, setBusy] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const [problem, setProblem] = useState("");
  const [notice, setNotice] = useState("");
  const pending = useRef<{ key: string; body: string } | null>(null);

  async function ship() {
    if (busy) return;
    const bad = shipmentFieldsError(shipmentCopy, fields);
    if (bad) {
      setProblem(bad);
      return;
    }
    const body = JSON.stringify({
      expected_version: 0, status: "SHIPPED", carrier_code: fields.carrier,
      carrier_name: fields.carrierName.trim() || null, tracking_number: fields.tracking.trim(),
      tracking_url: fields.url.trim() || null, note: fields.note.trim() || null, void_reason: null,
    });
    if (pending.current?.body !== body) pending.current = { key: `parcel-ship-${crypto.randomUUID()}`, body };
    setBusy(true);
    setProblem("");
    // Calls BFF PUT parcel-groups/{id}/shipment -> Go ShipParcelGroup (one RecordShipment per member, one transaction).
    const result = await shipParcelGroup(store, group.id, pending.current.key, body, boundary);
    setBusy(false);
    if (result.ok) {
      pending.current = null;
      setUncertain(false);
      setNotice(c.shipDone);
      setShipOpen(false);
      onReplace({ ...group, state: "SHIPPED", version: result.value.version });
      onChanged();
      return;
    }
    if (result.uncertain) {
      setUncertain(true);
      setProblem(c.shipUncertain);
      return;
    }
    pending.current = null;
    setProblem(errorText(c, result.code));
  }

  async function dissolve() {
    if (busy) return;
    setBusy(true);
    setProblem("");
    // Calls BFF DELETE parcel-groups/{id}?expected_version=N -> Go fulfillment.dissolve_parcel_group (CAS on version).
    const result = await dissolveParcelGroup(store, group.id, group.version, boundary);
    setBusy(false);
    if (result.ok) {
      setNotice(c.dissolveDone);
      setDissolveConfirm(false);
      onReplace({ ...group, state: "DISSOLVED", version: result.value.version });
      onChanged();
      onSuggestions(); // the members are mergeable again
      return;
    }
    setProblem(errorText(c, result.code));
  }

  return (
    <section className="orders-section" data-testid={`parcel-group-${group.short}`} aria-label={c.groupTitle(group.short)}>
      <h2>
        {c.groupTitle(group.short)} <span className="orders-badge" data-testid={`parcel-state-${group.short}`}>{c.states[group.state]}</span>
      </h2>
      <p className="orders-hint">{c.rule}</p>
      <p>{c.members}:</p>
      <ul>
        {group.orderIDs.map((id) => (
          <li key={id} className="orders-mono" data-testid={`parcel-group-member-${id}`}>{id}</li>
        ))}
      </ul>
      {problem && <p className="orders-bad" role="alert" data-testid={`parcel-problem-${group.short}`}>{problem}</p>}
      {notice && <p className="orders-notice" role="status" data-testid={`parcel-notice-${group.short}`}>{notice}</p>}
      {group.state === "OPEN" && (
        <div className="orders-form-actions">
          {!dissolveConfirm && (
            <button type="button" data-testid={`parcel-ship-open-${group.short}`} disabled={disabled || busy}
              onClick={() => { setShipOpen((v) => !v); setProblem(""); setNotice(""); }}>
              {c.ship}
            </button>
          )}
          {!shipOpen && !dissolveConfirm && (
            <button type="button" data-testid={`parcel-dissolve-${group.short}`} disabled={disabled || busy}
              onClick={() => { setDissolveConfirm(true); setProblem(""); setNotice(""); }}>
              {c.dissolve}
            </button>
          )}
          {dissolveConfirm && (
            <>
              <button type="button" className="primary" data-testid={`parcel-dissolve-confirm-${group.short}`} disabled={busy}
                onClick={() => void dissolve()}>
                {busy ? c.dissolveSending : c.dissolveConfirm}
              </button>
              <button type="button" data-testid={`parcel-dissolve-back-${group.short}`} disabled={busy}
                onClick={() => setDissolveConfirm(false)}>
                {c.dissolveBack}
              </button>
            </>
          )}
        </div>
      )}
      {group.state === "OPEN" && shipOpen && (
        <form
          className="orders-form"
          noValidate
          data-testid={`parcel-ship-form-${group.short}`}
          onSubmit={(event) => {
            event.preventDefault();
            void ship();
          }}
        >
          <ShipmentFields c={shipmentCopy} value={fields} disabled={uncertain || busy} onChange={(patch) => setFields((v) => ({ ...v, ...patch }))} testidPrefix="parcel-ship" />
          <div className="orders-form-actions">
            <button type="submit" className="primary" data-testid={`parcel-ship-submit-${group.short}`} disabled={busy}>
              {busy ? c.shipSending : uncertain ? c.shipRetry : c.shipSubmit}
            </button>
          </div>
        </form>
      )}
      {group.state !== "OPEN" && (
        <div className="orders-form-actions">
          <button type="button" data-testid={`parcel-close-${group.short}`} onClick={() => onDismiss(group.id)}>
            {c.close}
          </button>
        </div>
      )}
    </section>
  );
}
