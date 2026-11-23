// Purpose: Reviews one manual shipment before the existing command is sent.
// Depends on: react, @live-commerce/i18n, @/lib/client, @/lib/orders-model, @/lib/orders-copy; caller supplies frozen order facts and callbacks.
// Used by: apps/admin/components/OrderShipment.tsx.
// Invariants: One order identity; opening/cancelling never sends a command; native dialog manages modal focus.
"use client";
import { useEffect, useId, useRef } from "react";
import type { Locale } from "@live-commerce/i18n";
import { money } from "@/lib/client";
import { codCopy } from "@/lib/cod-copy";
import type { OrdersCopy } from "@/lib/orders-copy";
import type { OrderDetail } from "@/lib/orders-model";
import { orderRevisionCopy, OrderRevisionAmounts } from "./OrderRevisionSummary";
import s from "./order-revision.module.css";

/** Shows the exact order and entered logistics without claiming a carrier action. */
export function ShipmentConfirm({ open, order, orderNumber, carrier, tracking, locale, c, inline, onConfirm, onCancel }: {
  open: boolean; order: OrderDetail; orderNumber?: string; carrier: string; tracking: string;
  locale: Locale; c: OrdersCopy; inline?: boolean; onConfirm: () => void; onCancel: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const title = useId();
  const inlineHeading = useRef<HTMLHeadingElement>(null);
  const revision = order.payment_mode === "cash_on_delivery" && order.destination.kind === "home" &&
    (order.order_revision ?? 0) > 1 ? order.order_revision : undefined;
  useEffect(() => {
    if (inline) {
      if (open) inlineHeading.current?.focus();
      return;
    }
    const node = dialog.current;
    if (!node || !open) return;
    const invoker = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    node.showModal();
    node.querySelector<HTMLButtonElement>('[data-testid="shipment-confirm-cancel"]')?.focus();
    return () => { node.close(); invoker?.focus(); };
  }, [inline, open]);
  if (!open && inline) return null;
  const heading = revision ? orderRevisionCopy[locale].confirm.replace("{n}", String(revision)) : c.shipReview;
  const contents = open && <>
      <section className="orders-confirm-identity" aria-label={c.order}>
        <strong>{orderNumber ?? order.order_id}</strong>
        {order.payment_mode === "cash_on_delivery" && order.order_revision && <span> · v{order.order_revision}</span>}
        {!revision && <>
        <ul>{order.items.map((item, index) => <li key={`${item.sku_id}:${index}`}>{item.name} · {item.code} · {c.quantity}: {item.quantity}</li>)}</ul>
        <p>{c.total}: {money(locale, order.currency, order.totals.total_minor)}</p>
        {order.cod_collect_minor !== null && <p>{codCopy[locale].collectAmount}: {money(locale, order.currency, order.cod_collect_minor)}</p>}
        <p>{c.payment}: {c.statuses[order.payment_state]}</p>
        <p>{c.fulfillment}: {c.statuses[order.fulfillment_state]}</p>
        </>}
      </section>
      <dl className="orders-facts">
        <div><dt>{c.shipCarrier}</dt><dd>{carrier}</dd></div>
        <div><dt>{c.shipTracking}</dt><dd>{tracking}</dd></div>
      </dl>
      {revision && <OrderRevisionAmounts locale={locale} currency={order.currency} totals={order.totals}
        fee={order.cod_surcharge_minor} due={order.cod_collect_minor} />}
      <p className="orders-manual-note" data-testid="shipment-manual-note">{c.shipManualNote}</p>
      <div className="orders-dialog-actions">
        <button type="button" className="primary" data-testid="shipment-confirm" onClick={onConfirm}>{c.shipConfirm}</button>
        <button type="button" data-testid="shipment-confirm-cancel" onClick={onCancel}>{c.cancel}</button>
      </div>
  </>;
  if (inline) return <section data-r03-confirmation className={s.confirmation} aria-labelledby={title}>
    <h1 id={title} tabIndex={-1} ref={inlineHeading}>{heading}</h1>
    <div className={s.panel}>{contents}</div>
  </section>;
  return <dialog ref={dialog} className="orders-dialog orders-shipment-confirm" aria-labelledby={title}
    onCancel={event => { event.preventDefault(); onCancel(); }}>
    {open && <><h2 id={title}>{heading}</h2>{contents}</>}
  </dialog>;
}
