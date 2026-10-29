// Purpose: Reviews one manual shipment before the existing command is sent.
// Depends on: react, @live-commerce/i18n, @/lib/client, @/lib/orders-model, @/lib/orders-copy; caller supplies frozen order facts and callbacks.
// Used by: apps/admin/components/OrderShipment.tsx.
// Invariants: One order identity; opening/cancelling never sends a command; native dialog manages modal focus.
"use client";
import { useEffect, useId, useRef } from "react";
import type { Locale } from "@live-commerce/i18n";
import { money } from "@/lib/client";
import type { OrdersCopy } from "@/lib/orders-copy";
import type { OrderDetail } from "@/lib/orders-model";

/** Shows the exact order and entered logistics without claiming a carrier action. */
export function ShipmentConfirm({ open, order, orderNumber, carrier, tracking, locale, c, onConfirm, onCancel }: {
  open: boolean; order: OrderDetail; orderNumber?: string; carrier: string; tracking: string;
  locale: Locale; c: OrdersCopy; onConfirm: () => void; onCancel: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const title = useId();
  useEffect(() => {
    const node = dialog.current;
    if (!node || !open) return;
    const invoker = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    node.showModal();
    node.querySelector<HTMLButtonElement>('[data-testid="shipment-confirm-cancel"]')?.focus();
    return () => { node.close(); invoker?.focus(); };
  }, [open]);
  return <dialog ref={dialog} className="orders-dialog orders-shipment-confirm" aria-labelledby={title}
    onCancel={event => { event.preventDefault(); onCancel(); }}>
    {open && <>
      <h2 id={title}>{c.shipReview}</h2>
      <section className="orders-confirm-identity" aria-label={c.order}>
        <strong>{orderNumber ?? order.order_id}</strong>
        <ul>{order.items.map(item => <li key={item.sku_id}>{item.name} · {item.code} · {c.quantity}: {item.quantity}</li>)}</ul>
        <p>{c.total}: {money(locale, order.currency, order.totals.total_minor)}</p>
        <p>{c.payment}: {c.statuses[order.payment_state]}</p>
        <p>{c.fulfillment}: {c.statuses[order.fulfillment_state]}</p>
      </section>
      <dl className="orders-facts">
        <div><dt>{c.shipCarrier}</dt><dd>{carrier}</dd></div>
        <div><dt>{c.shipTracking}</dt><dd>{tracking}</dd></div>
      </dl>
      <p className="orders-manual-note" data-testid="shipment-manual-note">{c.shipManualNote}</p>
      <div className="orders-dialog-actions">
        <button type="button" className="primary" data-testid="shipment-confirm" onClick={onConfirm}>{c.shipConfirm}</button>
        <button type="button" data-testid="shipment-confirm-cancel" onClick={onCancel}>{c.cancel}</button>
      </div>
    </>}
  </dialog>;
}
