"use client";

// Purpose: the finished-order view of the buyer order page: 已完成訂單 (frames 63:1475 / 63:1476) and 已取消訂單 (63:1477 / 63:1478), with no payment entry.
// Depends on: components/order-status/{copy,format}.ts, lib/payment-copy + payment-contract (refund lines), ShipmentBlock, lib/purchase (Order), lib/order-copy via ShipmentBlock.
// Used by: components/order-status/OrderStatusView.tsx. Invariants: shows server order facts only (state, lines, amounts); never offers payment, never fabricates a state.
import type { Locale } from "@live-commerce/i18n";
import type { Order } from "../../lib/purchase";
import type { OrderPayment } from "../../lib/payment-contract";
import { paymentCopy } from "../../lib/payment-copy";
import { ShipmentBlock } from "../ShipmentBlock";
import { orderStatusCopy } from "./copy";
import { shortOrderNumber } from "./format";

export default function OrderTerminalView({
  order,
  locale,
  money,
  kind,
  payment = null,
}: {
  order: Order;
  locale: Locale;
  money: (amount: number, currency: string) => string;
  kind: "completed" | "cancelled";
  // The server payment read of a shipped card order (null: cancelled view, or the read failed). 已付款 is printed only for a captured, unrefunded payment.
  payment?: OrderPayment | null;
}) {
  const c = orderStatusCopy[locale];
  const quote = order.snapshot.quote;
  const pay = paymentCopy[locale];
  const refund = payment?.refund ?? null;
  const refundActive = !!refund && (refund.pending_minor > 0 || refund.refunded_minor > 0);
  const payLabel = !payment ? "—" : payment.payment_state === "CAPTURED" && !refundActive ? undefined : c.paymentStates[payment.payment_state] ?? payment.payment_state;
  const completed = kind === "completed";
  return (
    <section className="os os-terminal order-section" data-testid="order-section" data-terminal={kind} aria-labelledby="order-title">
      <a className="os-back" href={`/${locale}/orders/lookup`}>{c.backShort}</a>
      <header className="os-head">
        <h1 id="order-title">{shortOrderNumber(order.order_id)}</h1>
        <span className="os-chip" data-testid="order-state" data-state={order.commercial_state}>{completed ? c.termCompleted : c.termCancelled}</span>
      </header>
      <p><strong>{completed ? c.termCompletedTitle : c.termCancelledTitle}</strong></p>
      <p className="order-note">{completed ? c.termCompletedNote : c.termCancelledNote}</p>
      <div className="os-grid">
        <section className="os-card" aria-labelledby="os-term-items">
          <h2 id="os-term-items">{c.termItems}</h2>
          <ul className="order-lines">
            {quote.lines.map((line) => (
              <li key={line.sku_id}>
                <span>{line.name} × {line.quantity}</span>
                <span>{money(line.unit_price_minor * line.quantity, quote.currency)}</span>
              </li>
            ))}
          </ul>
        </section>
        <section className="os-card" aria-labelledby="os-term-pay">
          <h2 id="os-term-pay">{c.termPayShip}</h2>
          <dl className="order-breakdown">
            <div><dt>{c.termPayStatus}</dt><dd>{completed ? payLabel ?? c.termPaid : c.termUnpaid}</dd></div>
            <div><dt>{c.termOrderStatus}</dt><dd>{completed ? c.termCompleted : c.termCancelled}</dd></div>
            <div><dt>{c.termGoods}</dt><dd>{money(quote.amount.subtotal_minor, quote.currency)}</dd></div>
            <div><dt>{c.shipping}</dt><dd>{money(quote.amount.shipping_minor, quote.currency)}</dd></div>
          </dl>
          {refund && refund.pending_minor > 0 && <p role="status" data-testid="refund-processing">{pay.refundProcessing}</p>}
          {refund && refund.refunded_minor > 0 && (
            <p data-testid="refund-succeeded">{pay.refunded.replace("{amount}", money(refund.refunded_minor, payment!.currency))}</p>
          )}
          {!completed && <p className="order-note">{c.termNoEntry}</p>}
          <p className="order-total"><span>{c.termTotal}</span> <strong>{money(quote.amount.total_minor, quote.currency)}</strong></p>
          {order.shipment && <ShipmentBlock shipment={order.shipment} locale={locale} />}
        </section>
      </div>
    </section>
  );
}
