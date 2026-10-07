"use client";

// Purpose: W3-07B parcel-merge UI on the merchant orders page: the merge-suggestion banner (N groups can ship as one
//   parcel, recipient shown MASKED like the list row), the suggestion cards with the 合併 action and the per-group panel
//   (members, 填寫運單, 解除合併, close). The owner ruling sentence (parcels only, never payments/amounts; every order priced
//   and notified on its own) is always visible next to the banner. The OPEN groups come from GET parcel-groups on every load
//   and refresh (W3-U4), so a reload rebuilds the ship/dissolve panels; create/ship answers update them in the session and
//   the server guards (in_parcel_group, group_not_open, CAS) stay the authority for stale edits.
// Depends on: @/lib/parcels-client (BFF orders/merge-suggestions + parcel-groups* -> Go internal/httpapi/parcels.go),
//   @/lib/parcels-model (parsers, reconcileGroups), @/lib/parcels-copy, ./OrderShipment (ShipmentFields + shipmentFieldsError),
//   ./orders.css classes.
// Used by: apps/admin/components/MerchantOrders.tsx (rendered above the orders table, groups state lifted there for badges).
// Invariants: every mutating click sends one Idempotency-Key per logical write (an uncertain retry reuses it); dissolve is
//   CAS-guarded by the group version; COD/pay-at-pickup/CVS orders never appear (the server excludes them; create would
//   refuse with cod_not_mergeable/cvs_not_mergeable); a refusal that says the group is no longer open refetches the OPEN
//   groups so the panel reflects the server instead of staying OPEN; a merge/load error stays visible without suggestions, and a failed read (suggestions or groups) offers a retry button.

import { useEffect, useRef, useState } from "react";
import {
  createParcelGroup,
  dissolveParcelGroup,
  readMergeSuggestions,
  readOpenParcelGroups,
  shipParcelGroup,
} from "@/lib/parcels-client";
import { parcelShort, reconcileGroups, type MergeSuggestion, type ParcelGroupView } from "@/lib/parcels-model.ts";
import type { ParcelCopy } from "@/lib/parcels-copy";
import type { OrdersCopy } from "@/lib/orders-copy";
import { ShipmentFields, shipmentFieldsError, type ShipmentFieldValues } from "./OrderShipment";

export type { ParcelGroupView };

const emptyFields: ShipmentFieldValues = { carrier: "", carrierName: "", tracking: "", url: "", note: "" };

function errorText(c: ParcelCopy, code: string) {
  return (c.errors as Record<string, string>)[code] ?? c.errors.default;
}

/** Banner + suggestion cards + group panels. Reads suggestions and the OPEN groups; every write re-reads orders via onChanged. */
export function ParcelMerge({
  store,
  boundary,
  disabled,
  c,
  shipmentCopy,
  groups,
  onGroups,
  onChanged,
}: {
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
  // The latest groups for async callbacks, and a counter bumped by every create so a groups read that started before it
  // (and would not list the new group) is discarded instead of dropping the fresh panel.
  const groupsRef = useRef(groups);
  groupsRef.current = groups;
  const created = useRef(0);
  // Every write goes through commit: it moves groupsRef FIRST, so two panel actions that finish between renders each start from
  // the other's result instead of a stale closure of `groups` (which would revert a finished panel's terminal state).
  function commit(next: ParcelGroupView[]) {
    groupsRef.current = next;
    onGroups(next);
  }

  useEffect(() => {
    const active = new AbortController();
    const startedAt = created.current;
    readMergeSuggestions(store, active.signal).then(
      (value) => {
        if (active.signal.aborted) return;
        setSuggestions(value);
        setProblem((p) => (p === c.suggestionsUnavailable ? "" : p)); // a retry that works clears only its own earlier load error
      },
      () => {
        // A failed read must not look like "nothing to merge": keep the stale list out, show the error and offer a retry.
        if (!active.signal.aborted) {
          setSuggestions([]);
          setProblem(c.suggestionsUnavailable);
        }
      },
    );
    // Calls BFF GET parcel-groups -> Go fulfillment.read_open_parcel_groups (migration 0164): the OPEN groups, so ship/dissolve
    // panels exist after a reload; the server's version replaces the session's so the dissolve CAS is live.
    readOpenParcelGroups(store, active.signal).then(
      (open) => {
        if (active.signal.aborted || created.current !== startedAt) return;
        commit(reconcileGroups(groupsRef.current, open));
        setProblem((p) => (p === c.groupsUnavailable ? "" : p)); // a retry that works clears only its own earlier load error
      },
      () => {
        if (!active.signal.aborted) setProblem(c.groupsUnavailable); // never leave an OPEN group silently without its panel
      },
    );
    return () => active.abort();
    // onGroups and c are re-created every render by design: the reads are keyed by store, session boundary and tick only.
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
      created.current += 1;
      commit([
        ...groupsRef.current,
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
    commit(groupsRef.current.map((g) => (g.id === next.id ? next : g)));
  }
  function dismiss(id: string) {
    commit(groupsRef.current.filter((g) => g.id !== id));
  }
  // A panel hit a refusal that says its group moved on (group_not_open: shipped/dissolved elsewhere or by an uncertain dissolve that
  // did land; version_changed: the CAS version is stale). Refetch the OPEN groups: a group no longer OPEN is dropped, a changed
  // one gets its live version. group_not_open also leaves its message here, because the dropped panel takes its own with it.
  function groupMoved(code: string) {
    if (code === "group_not_open") setProblem(errorText(c, code));
    setTick((v) => v + 1);
  }

  if (suggestions.length === 0 && groups.length === 0 && !problem) return null;
  return (
    <>
      {(suggestions.length > 0 || problem) && (
        <section className="orders-section" data-testid="parcel-merge" aria-label={c.banner(suggestions.length)}>
          {suggestions.length > 0 && (
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
          )}
          <p className="orders-hint" data-testid="parcel-merge-rule">{c.rule}</p>
          {problem && <p className="orders-bad" role="alert" data-testid="parcel-merge-problem">{problem}</p>}
          {(problem === c.suggestionsUnavailable || problem === c.groupsUnavailable) && (
            <p>
              <button type="button" className="orders-compact" data-testid="parcel-reload-retry" onClick={() => setTick((v) => v + 1)}>
                {c.retryLoad}
              </button>
            </p>
          )}
          {open && suggestions.length > 0 && (
            <ul>
              {suggestions.map((s) => (
                <li key={s.order_ids[0]} data-testid="parcel-suggestion">
                  <span>{c.suggestionFor}: {s.recipient_masked}</span>{" "}
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
          onMoved={groupMoved}
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
  onMoved,
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
  onMoved: (code: string) => void;
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
    if (result.code === "group_not_open" || result.code === "version_changed") onMoved(result.code);
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
    // An uncertain dissolve that actually landed answers group_not_open on the same-version retry: reconcile instead of staying OPEN.
    if (result.code === "group_not_open" || result.code === "version_changed") onMoved(result.code);
  }

  return (
    <section className="orders-section" data-testid={`parcel-group-${group.short}`} aria-label={c.groupTitle(group.short)}>
      <h2>
        {c.groupTitle(group.short)} <span className="orders-badge" data-testid={`parcel-state-${group.short}`}>{c.states[group.state]}</span>
      </h2>
      <p className="orders-hint">{c.rule}</p>
      <p>{c.members}:</p>
      <ul>
        {group.orderIDs.map((id) => {
          // Order number + masked recipient once the server's OPEN-groups read has filled them in; the id until then.
          const member = group.members?.find((m) => m.order_id === id);
          return (
            <li key={id} className="orders-mono" data-testid={`parcel-group-member-${id}`}>
              {member ? `${member.order_number} · ${member.recipient_masked}` : id}
            </li>
          );
        })}
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
