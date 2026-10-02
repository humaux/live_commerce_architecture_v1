"use client";

// Cash-on-delivery section of the merchant order detail row (home-cod R5, migration 0107). COD shares the pay_at_pickup collection
// state machine (one shared function, taiwan-cvs-logistics-v1 §16.4/§16.8), so the writes reuse the same BFF routes as the pickup panel:
//   POST /api/stores/{store}/orders/{id}/collection              (fulfillment:write, key)  collected / returned / refunded_offline
//   POST /api/stores/{store}/orders/{id}/pay-at-pickup-release   (fulfillment:write, key)  cancel / restock
// Nothing here is optimistic: every write re-reads the order (onChanged). One Idempotency-Key per dialog open, reused only for a
// byte-identical retry after an unknown outcome. "collected" is only offered once the order is shipped (hint only; SQL re-checks).
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { postCollection, postRelease, type WriteResult } from "@/lib/logistics-client";
import {
  collectionActions,
  collectionBody,
  isCollectionState,
  releaseBody,
  type CollectionAction,
  type CollectionState,
  type ReleaseAction,
} from "@/lib/logistics-model";
import { logisticsCopy, logisticsError } from "@/lib/logistics-copy";
import { codCopy } from "@/lib/cod-copy";
import type { OrderDetail } from "@/lib/orders-model";
import type { OrdersCopy } from "@/lib/orders-copy";
import "./order-actions.css";

type Action = CollectionAction | ReleaseAction;
type Open = { action: Action };

export function OrderCodCollection({
  store,
  detail,
  locale,
  c,
  canWrite,
  boundary,
  onChanged,
}: {
  store: string;
  detail: OrderDetail;
  locale: Locale;
  c: OrdersCopy;
  canWrite: boolean;
  boundary: string;
  onChanged: () => Promise<boolean>;
}) {
  const lc = logisticsCopy[locale];
  const cc = codCopy[locale];
  const orderID = detail.order_id;
  const collection: CollectionState | null = isCollectionState(detail.collection_state) ? detail.collection_state : null;
  const [open, setOpen] = useState<Open | null>(null);
  const [problem, setProblem] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);
  const pending = useRef<{ key: string; body: string | null } | null>(null);

  useEffect(() => {
    const element = dialog.current;
    if (!element) return;
    if (open && !element.open) element.showModal();
    if (!open && element.open) element.close();
  }, [open]);

  const actions = collectionActions(collection, detail.fulfillment_state);

  function begin(action: Action) {
    pending.current = { key: `cod-${crypto.randomUUID()}`, body: null };
    setProblem("");
    setUncertain(false);
    setNotice("");
    setOpen({ action });
  }
  function finish() {
    setOpen(null);
    if (uncertain) void onChanged(); // closing after an unknown outcome still re-reads the server state
    setUncertain(false);
  }
  const bodyOf = (o: Open) =>
    o.action === "cancel" || o.action === "restock"
      ? releaseBody(o.action, o.action === "cancel" ? "PENDING" : "RETURNED")
      : collectionBody(o.action === "refunded_offline" ? "COLLECTED" : "PENDING", o.action);
  async function send() {
    if (!open || busy || !pending.current) return;
    const body = bodyOf(open);
    if (pending.current.body !== null && pending.current.body !== body)
      pending.current = { key: `cod-${crypto.randomUUID()}`, body };
    else pending.current.body = body;
    const { key } = pending.current;
    setBusy(true);
    setProblem("");
    const result: WriteResult =
      open.action === "cancel" || open.action === "restock"
        ? await postRelease(store, orderID, key, body, boundary)
        : await postCollection(store, orderID, key, body, boundary);
    setBusy(false);
    if (result.ok) {
      pending.current = null;
      setUncertain(false);
      setOpen(null);
      setNotice(cc.done);
      void onChanged();
      return;
    }
    if (result.uncertain) {
      setUncertain(true);
      setProblem(lc.uncertain);
      return;
    }
    pending.current = null;
    setProblem(logisticsError(lc, c, result.code));
    if (["version_changed", "version_conflict", "collection_state_changed", "not_shippable"].includes(result.code))
      void onChanged();
  }

  const confirmText = (o: Open) =>
    o.action === "collected" ? cc.confirmCollected
    : o.action === "returned" ? cc.confirmReturned
    : o.action === "refunded_offline" ? cc.confirmRefundedOffline
    : o.action === "cancel" ? cc.confirmCancel
    : cc.confirmRestock;

  return (
    <section className="orders-section" data-testid="order-cod" aria-label={cc.orderTitle}>
      <h2>{cc.orderTitle}</h2>
      {collection && (
        <dl className="orders-facts">
          <div>
            <dt>{cc.orderState}</dt>
            <dd data-testid="cod-collection-state" data-state={collection}>{cc.orderStates[collection]}</dd>
          </div>
        </dl>
      )}
      <p className="orders-hint">{cc.orderNote}</p>
      {canWrite && (
        <div className="orders-form-actions">
          {actions.collected && <button type="button" data-testid="cod-collected" onClick={() => begin("collected")}>{cc.collected}</button>}
          {actions.returned && <button type="button" data-testid="cod-returned" onClick={() => begin("returned")}>{cc.returned}</button>}
          {actions.refunded_offline && (
            <button type="button" data-testid="cod-refunded-offline" onClick={() => begin("refunded_offline")}>{cc.refundedOffline}</button>
          )}
          {actions.cancel && <button type="button" data-testid="cod-cancel-order" onClick={() => begin("cancel")}>{cc.cancelOrder}</button>}
          {actions.restock && <button type="button" data-testid="cod-restock" onClick={() => begin("restock")}>{cc.restockOrder}</button>}
        </div>
      )}
      {notice && <p className="orders-notice" role="status" data-testid="cod-notice">{notice}</p>}
      <dialog ref={dialog} className="orders-dialog" aria-labelledby={`cod-title-${orderID}`} data-testid="cod-dialog" onClose={() => { if (open) finish(); }}>
        {open && (
          <form onSubmit={(event) => { event.preventDefault(); void send(); }}>
            <h2 id={`cod-title-${orderID}`}>{lc.confirmTitle}</h2>
            <p data-testid="cod-confirm-text">{confirmText(open)}</p>
            {problem && <p className="orders-bad" role="alert" data-testid="cod-problem">{problem}</p>}
            <div className="orders-dialog-actions">
              <button type="button" onClick={finish}>{uncertain ? lc.close : lc.cancel}</button>
              <button type="submit" className="primary" data-testid="cod-submit" disabled={busy}>
                {busy ? lc.saving : uncertain ? lc.retrySame : lc.confirmSubmit}
              </button>
            </div>
          </form>
        )}
      </dialog>
    </section>
  );
}
