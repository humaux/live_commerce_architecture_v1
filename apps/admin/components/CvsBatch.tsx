// Purpose: count-confirmed CVS batches with durable unknown-outcome blocking and no retry loop.
// Depends on: picklist-client/model/copy, logistics-client recovery reads, ID-only sessionStorage and scoped onViewOrder.
// Used by: PickList for fulfillment_write actors; never purchases real labels in fixtures.
"use client";
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { readCvsShipment } from "@/lib/logistics-client";
import { sessionBoundary } from "@/lib/settings-client";
import { createCvsBatch, PickError } from "@/lib/picklist-client";
import {
  pickUUID,
  cvsRecoveryResolved,
  type BatchResult,
} from "@/lib/picklist-model";
import { picklistCopy } from "@/lib/picklist-copy";
/** Confirm a bounded batch once. An unresolved write stays blocked across page reloads. */
export function CvsBatch({
  locale,
  store,
  boundary,
  ids,
  disabled,
  onViewOrder,
}: {
  locale: Locale;
  store: string;
  boundary: string;
  ids: string[];
  disabled: boolean;
  onViewOrder: (id: string) => void;
}) {
  const c = picklistCopy[locale],
    journal = `picklist-cvs:${store}:${boundary}`;
  const [storageBlocked, setStorageBlocked] = useState(false),
    [checkedCount, setCheckedCount] = useState<number | null>(null);
  const [confirm, setConfirm] = useState(false),
    [busy, setBusy] = useState(false),
    [result, setResult] = useState<BatchResult | null>(null),
    [unknown, setUnknown] = useState<string[] | null>(null),
    [error, setError] = useState("");
  const flight = useRef(false),
    alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    try {
      const raw = sessionStorage.getItem(journal);
      if (raw) {
        const saved: unknown = JSON.parse(raw);
        if (
          !Array.isArray(saved) ||
          saved.length < 1 ||
          saved.length > 100 ||
          saved.some((id) => typeof id !== "string" || !pickUUID.test(id))
        ) {
          setStorageBlocked(true);
          setError(c.storage);
        } else setUnknown(saved);
      }
    } catch {
      setStorageBlocked(true);
      setError(c.storage);
    }
    return () => {
      alive.current = false;
    };
  }, [journal, c.storage]);
  async function checkStatus() {
    if (!unknown || flight.current) return;
    flight.current = true;
    setBusy(true);
    setError("");
    try {
      if ((await sessionBoundary()) !== boundary) throw new Error();
      const found: string[] = [];
      // Read only, bounded groups: a missing record or provider UNKNOWN never proves no side effect.
      for (let i = 0; i < unknown.length; i += 4) {
        const group = unknown.slice(i, i + 4);
        const values = await Promise.all(
          group.map((id) =>
            readCvsShipment(store, id, AbortSignal.timeout(10000)),
          ),
        );
        values.forEach((value, j) => {
          if (cvsRecoveryResolved([value])) found.push(group[j]);
        });
      }
      if ((await sessionBoundary()) !== boundary) throw new Error();
      if (alive.current) setCheckedCount(found.length);
      if (found.length === unknown.length) {
        sessionStorage.removeItem(journal);
        if (alive.current) {
          setUnknown(null);
          setResult({
            results: found.map((order_id) => ({
              order_id,
              outcome: "already",
            })),
          });
        }
      }
    } catch {
      if (alive.current) setError(c.unavailable);
    } finally {
      flight.current = false;
      if (alive.current) setBusy(false);
    }
  }
  async function submit() {
    if (flight.current || unknown || ids.length < 1 || ids.length > 100) return;
    const submitted = [...ids];
    flight.current = true;
    setBusy(true);
    setError("");
    setResult(null);
    setCheckedCount(null); // A new command must not inherit the previous recovery count.
    setConfirm(false);
    // Persist BEFORE dispatch: a closed tab or unreadable response is never interpreted as no effect.
    try {
      sessionStorage.setItem(journal, JSON.stringify(submitted));
    } catch {
      setStorageBlocked(true);
      setError(c.storage);
      setBusy(false);
      flight.current = false;
      return;
    }
    try {
      const answer = await createCvsBatch(
        store,
        submitted,
        boundary,
        crypto.randomUUID(),
      );
      const uncertain = answer.results.some(
        (r) => r.outcome === "failed" && r.code === "retry",
      );
      if (!uncertain) sessionStorage.removeItem(journal);
      if (alive.current) {
        setResult(answer);
        if (uncertain) setUnknown(submitted);
      }
    } catch (e) {
      const uncertain = !(e instanceof PickError) || e.uncertain;
      if (!uncertain) sessionStorage.removeItem(journal);
      if (alive.current) {
        if (uncertain) setUnknown(submitted);
        else
          setError(
            e instanceof PickError && e.code === "connection_unavailable"
              ? c.connection
              : e instanceof PickError && e.code === "unauthorized"
                ? c.unauthorized
                : c.unavailable,
          );
      }
    } finally {
      flight.current = false;
      if (alive.current) setBusy(false);
    }
  }
  return (
    <section className="pick-cvs" aria-label={c.cvs}>
      <button
        type="button"
        disabled={
          disabled ||
          busy ||
          !!unknown ||
          storageBlocked ||
          ids.length < 1 ||
          ids.length > 100
        }
        onClick={() => {
          setConfirm(true);
          setResult(null);
        }}
      >
        {busy ? c.loading : c.cvs} ({ids.length})
      </button>
      <small>{c.cvsLimit}</small>
      {confirm && (
        <div role="group" aria-label={c.confirm} className="pick-confirm">
          <p>
            {c.confirmBefore} <strong>{ids.length}</strong> {c.confirmAfter}
          </p>
          <button
            type="button"
            disabled={disabled || busy || ids.length < 1 || ids.length > 100}
            onClick={submit}
          >
            {c.confirm}
          </button>
          <button type="button" onClick={() => setConfirm(false)}>
            {c.cancel}
          </button>
        </div>
      )}
      {error && <p role="alert">{error}</p>}
      {unknown && (
        <div role="status">
          <p>{c.unknown}</p>
          {checkedCount !== null && (
            <p>
              {c.checked}: {checkedCount} / {unknown.length}
            </p>
          )}
          <button type="button" disabled={busy} onClick={checkStatus}>
            {c.checkStatus}
          </button>
          <ul>
            {unknown.map((id) => (
              <li key={id}>
                <button type="button" onClick={() => onViewOrder(id)}>
                  {c.check} · {id.slice(-8)}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
      {result && (
        <div aria-label={c.done}>
          <h3>{c.done}</h3>
          <ul>
            {result.results.map((r) => (
              <li key={r.order_id}>
                <button type="button" onClick={() => onViewOrder(r.order_id)}>
                  {c.check} · {r.order_id.slice(-8)}
                </button>{" "}
                —{" "}
                {r.outcome === "queued"
                  ? c.queued
                  : r.outcome === "already"
                    ? c.already
                    : r.code === "retry"
                      ? c.unknown
                      : r.code === "not_cvs"
                        ? c.notCvs
                        : r.code === "order_not_found"
                          ? c.notFound
                          : r.code === "invalid_order"
                            ? c.invalidOrder
                            : r.code === "version_changed"
                              ? c.changed
                              : c.failed}
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
