// Purpose: reviewed four-state buyer detail, current linked RMA progress and accessible withdrawal confirmation.
// Depends on: validated server/order projections, shared displayTime, existing RMA status copy and local copy/tokens.
// Used by: ReturnRequestForm. Invariants: no terminal relabeling, no refund/stock/provider action.
"use client";
import { useEffect, useRef, useState } from "react";
import { displayTime } from "@live-commerce/format";
import type { Locale } from "@live-commerce/i18n";
import type { Order } from "../../lib/purchase";
import { canWithdraw, type ReturnRequest } from "./return-request-state";
import { returnRequestCopy, returnTerminalCopy, returnProgressCopy } from "./return-request-copy";
import styles from "./ReturnRequest.module.css";
export default function ReturnRequestDetail({ locale, request, order, disabled, onWithdraw }: {
  locale: Locale; request: ReturnRequest; order: Order; disabled: boolean; onWithdraw: () => void;
}) {
  const c = returnRequestCopy[locale];
  const terminal = returnTerminalCopy[locale];
  const statuses = { requested: c.requested, withdrawn: c.withdrawn, approved: terminal.approved, rejected: terminal.rejected };
  const [confirming, setConfirming] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (confirming) dialog.current?.show();
    else if (dialog.current?.open) {
      dialog.current.close();
      trigger.current?.focus();
    }
  }, [confirming]);
  useEffect(() => { setConfirming(false); }, [request.id, request.version, request.status, disabled]);
  // Nonmodal show() does not provide modal Escape dismissal. Keep it available
  // even after keyboard navigation leaves the inline confirmation panel.
  useEffect(() => {
    if (!confirming) return;
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape") { event.preventDefault(); setConfirming(false); }
    };
    document.addEventListener("keydown", escape);
    return () => document.removeEventListener("keydown", escape);
  }, [confirming]);
  function dismiss() { setConfirming(false); }
  return <>
    <h1>{c.detail} {request.number}</h1>
    <p><span className={styles.badge} data-testid="return-status" data-status={request.status}>{statuses[request.status]}</span></p>
    <dialog ref={dialog} className={styles.dialog} aria-labelledby="return-confirm-title" aria-describedby="return-confirm-body"
      onCancel={event => { event.preventDefault(); dismiss(); }} onClose={() => setConfirming(false)}>
      <h2 id="return-confirm-title">{c.confirmTitle}</h2>
      <p id="return-confirm-body">{c.confirmBody}</p>
      <button className={styles.primary} type="button" disabled={disabled} onClick={() => { dismiss(); onWithdraw(); }}>{c.confirm}</button>
      <button className={styles.secondary} type="button" autoFocus onClick={dismiss}>{c.cancel}</button>
    </dialog>
    <section className={styles.card} aria-labelledby="return-content">
      <h2 id="return-content">{c.content}</h2>
      <p>{c.order} {request.order_id}</p>
      <ul>{request.lines.map(line => {
        const item = order.snapshot.quote.lines.find(item => item.sku_id === line.sku_id);
        return <li key={`${line.warehouse_id}:${line.sku_id}`}>
          {item?.name ?? line.sku_id} · {item?.code ?? line.sku_id} × {line.quantity}
        </li>;
      })}</ul>
      <p>{c.reason}: {c.damaged}</p>
      <p className={styles.note}>{c.note}: {request.note}</p>
    </section>
    <section className={styles.card} aria-labelledby="return-progress">
      <h2 id="return-progress">{c.progress}</h2>
      <ol>{request.timeline.map(event => <li key={`${event.status}:${event.at}`}>
        <time dateTime={event.at}>{displayTime(locale, event.at)}</time> · {statuses[event.status]}
      </li>)}</ol>
      {request.status === "approved" && request.return_state && <p data-testid="return-rma-state" data-state={request.return_state}>{terminal.rma}: {returnProgressCopy(locale)[request.return_state]}</p>}
      {request.status === "rejected" && <p className={styles.note} data-testid="return-rejection">{terminal.rejection}: {request.rejection_reason}</p>}
    </section>
    <p className={styles.notice}>{request.status === "approved" ? terminal.approvedBody : request.status === "rejected" ? terminal.rejectedBody : request.status === "withdrawn" ? c.withdrawnBody : c.pendingBody}</p>
    {canWithdraw(request) && <button ref={trigger} hidden={confirming} className={styles.secondary} type="button" disabled={disabled} onClick={() => setConfirming(true)}>{c.withdraw}</button>}
  </>;
}
