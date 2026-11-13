// Purpose: reviewed requested/withdrawn content and accessible withdrawal confirmation.
// Depends on: validated server projection, order projection, shared StorefrontState and local copy/tokens.
// Called by: ReturnRequestForm. Invariants: no terminal relabeling, no refund/stock/provider action.
"use client";
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Order } from "../../lib/purchase";
import { StorefrontState } from "../state/StorefrontState";
import { canWithdraw, isDesignedStatus, type ReturnRequest } from "./return-request-state";
import { returnRequestCopy } from "./return-request-copy";
import styles from "./ReturnRequest.module.css";
export default function ReturnRequestDetail({ locale, request, order, disabled, onWithdraw }: {
  locale: Locale; request: ReturnRequest; order: Order; disabled: boolean; onWithdraw: () => void;
}) {
  const c = returnRequestCopy[locale];
  const [confirming, setConfirming] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (confirming) dialog.current?.showModal();
    else dialog.current?.close();
  }, [confirming]);
  useEffect(() => { setConfirming(false); }, [request.id, request.version, disabled]);
  function dismiss() { setConfirming(false); trigger.current?.focus(); }
  if (!isDesignedStatus(request)) return <StorefrontState locale={locale} variant="empty" title={c.neutral} body="" note="" />;
  return <>
    <h1>{c.detail} {request.number}</h1>
    <p><span className={styles.badge}>{request.status === "requested" ? c.requested : c.withdrawn}</span></p>
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
      <p><time dateTime={request.created_at}>{request.created_at}</time> → <time dateTime={request.updated_at}>{request.updated_at}</time></p>
    </section>
    <section className={styles.card} aria-labelledby="return-progress">
      <h2 id="return-progress">{c.progress}</h2>
      <ol>{request.timeline.map(event => <li key={`${event.status}:${event.at}`}>
        <time dateTime={event.at}>{event.at}</time> · {event.status === "requested" ? c.requested : c.withdrawn}
      </li>)}</ol>
    </section>
    <p className={styles.notice}>{request.status === "withdrawn" ? c.withdrawnBody : c.pendingBody}</p>
    {canWithdraw(request) && <button ref={trigger} className={styles.secondary} type="button" disabled={disabled} onClick={() => setConfirming(true)}>{c.withdraw}</button>}
    <dialog ref={dialog} className={styles.dialog} aria-labelledby="return-confirm-title" aria-describedby="return-confirm-body"
      onCancel={event => { event.preventDefault(); dismiss(); }} onClose={() => setConfirming(false)}>
      <h2 id="return-confirm-title">{c.confirmTitle}</h2>
      <p id="return-confirm-body">{c.confirmBody}</p>
      <button className={styles.primary} type="button" disabled={disabled} onClick={() => { dismiss(); onWithdraw(); }}>{c.confirm}</button>
      <button className={styles.secondary} type="button" autoFocus onClick={dismiss}>{c.cancel}</button>
    </dialog>
  </>;
}
