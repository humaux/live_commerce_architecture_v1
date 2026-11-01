"use client";

// Purpose: the buyer order page in the signed frame layout: 訂單 <short number> with the 商品明細 card (lines, payment / pickup / cash-on-delivery block,
// 運費, 訂單合計) and the 收件與配送 card, for every state existing data can reach (card payment awaiting / pending / cancel requested / confirmed /
// closed, bank transfer, pay-at-pickup and cash-on-delivery collection states), the 前往付款 hand-off screen and the finished-order views.
// Depends on: OrderPayment (renderFrame: the unchanged Stripe read / poll / pay / cancel state machine), BankTransfer, ShipmentBlock, StorefrontState,
// ContactStoreButton, lib/{cvs-copy,cod-copy,payment-copy,purchase-copy,order-payment}, ./copy, ./format, ./OrderTerminalView.
// Used by: app/[locale]/orders/[orderID]/OrderPage.tsx only (the checkout page keeps OrderDetails in OrderFlow.tsx).
// Invariants: every value is a server response field; payment UNKNOWN stays read-only (the unknown panel only re-reads); nothing here resubmits a payment
// or computes money (the summary line prints server amounts); the full order id is never rendered, only the mail order number.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import OrderPayment, { type PaymentFrame } from "../OrderPayment";
import BankTransfer from "../BankTransfer";
import { ShipmentBlock } from "../ShipmentBlock";
import { StorefrontState } from "../state/StorefrontState";
import ContactStoreButton from "../privacy/ContactStoreButton";
import { cvsCopy } from "../../lib/cvs-copy";
import { codCopy } from "../../lib/cod-copy";
import { paymentCopy } from "../../lib/payment-copy";
import { purchaseCopy } from "../../lib/purchase-copy";
import { isCvsKind, shipmentCopyKey } from "../../lib/cvs-contract";
import { settledCommercialState, type TransferState } from "../../lib/bank-transfer-contract";
import { readOrderPayment } from "../../lib/order-payment";
import { cartPath } from "../../lib/routes";
import type { Order } from "../../lib/purchase";
import type { OrderPayment as OrderPaymentView } from "../../lib/payment-contract";
import { orderStatusCopy } from "./copy";
import { addressLines, shortOrderNumber } from "./format";
import OrderTerminalView from "./OrderTerminalView";
import "./order-status.css";

type Money = (amount: number, currency: string) => string;

export default function OrderStatusView({
  context,
  order,
  locale,
  money,
  refresh,
  busy,
  onPaymentBusy,
  isSelected,
}: {
  context: string;
  order: Order;
  locale: Locale;
  money: Money;
  refresh: () => void;
  busy: boolean;
  onPaymentBusy: (busy: boolean) => void;
  isSelected: () => boolean;
}) {
  const c = orderStatusCopy[locale];
  const cvs = cvsCopy[locale];
  const cod = codCopy[locale];
  const pay = paymentCopy[locale];
  const common = purchaseCopy[locale];
  const mode = order.payment_mode ?? "card";
  const isCard = mode === "card";
  const isCod = mode === "cash_on_delivery";
  const [handoff, setHandoff] = useState(false);
  const [paymentRefresh, setPaymentRefresh] = useState(0);
  const [transferState, setTransferState] = useState<TransferState | null>(null);
  // A cancelled card order that never started a payment is the finished 已取消訂單 view; one that has a closed payment keeps the 付款已關閉 frame.
  // A shipped card order reads the payment view too: a refund never changes commercial/fulfillment state (RD6), so only the payment read can say refunded.
  const [shippedPay, setShippedPay] = useState<OrderPaymentView | null | "pending">("pending");
  const [probe, setProbe] = useState<"pending" | "unpaid" | "paid-or-started">("pending");
  const paymentSignalRef = useRef<(() => Promise<void>) | null>(null);
  const shownState = settledCommercialState(order.commercial_state, transferState);
  const stale = shownState !== order.commercial_state;
  useEffect(() => {
    if (stale) refresh();
  }, [stale]);
  const cancelledCard = isCard && order.commercial_state === "CANCELLED";
  const shippedCard = isCard && order.commercial_state === "CONFIRMED" && order.fulfillment_state === "MERCHANT_SHIPPED";
  useEffect(() => {
    if (!shippedCard) return;
    let live = true;
    readOrderPayment(context, order.order_id)
      .then((view) => live && setShippedPay(view))
      .catch(() => live && setShippedPay(null));
    return () => {
      live = false;
    };
  }, [shippedCard, context, order.order_id]);
  useEffect(() => {
    if (!cancelledCard) return;
    let live = true;
    readOrderPayment(context, order.order_id)
      .then((view) => live && setProbe(view.payment_state === "NOT_STARTED" ? "unpaid" : "paid-or-started"))
      .catch(() => live && setProbe("paid-or-started"));
    return () => {
      live = false;
    };
  }, [cancelledCard, context, order.order_id]);

  const quote = order.snapshot.quote;
  const amount = quote.amount;
  const total = money(amount.total_minor, quote.currency);
  const short = shortOrderNumber(order.order_id);

  if (shippedCard && shippedPay === "pending") return <p role="status">{pay.loading}</p>;
  if (shippedCard) return <OrderTerminalView order={order} locale={locale} money={money} kind="completed" payment={shippedPay === "pending" ? null : shippedPay} />;
  if (cancelledCard && probe === "pending") return <p role="status">{pay.loading}</p>;
  if (cancelledCard && probe === "unpaid") return <OrderTerminalView order={order} locale={locale} money={money} kind="cancelled" />;

  async function recheck() {
    // Errors are swallowed inside the handler (shown in the payment section).
    const signal = paymentSignalRef.current;
    if (signal) await signal();
    setPaymentRefresh((v) => v + 1);
    refresh();
  }

  const chip =
    isCod ? null
    : order.payment_mode === "pay_at_pickup" && shownState === "AWAITING_COLLECTION" ? c.orderStates.AWAITING_COLLECTION
    : c.orderStates[shownState];
  const delivery = order.commercial_state === "CANCELLED" ? c.notShipped : order.shipment ? c.shipped : c.prepare;
  const collection = order.collection_state ?? null;
  const shipment = order.cvs_shipment ?? null;
  const colon = locale === "en" ? ": " : "：";
  const lines = addressLines(order, locale);

  // ---- the Stripe payment block, drawn in the frame layout from OrderPayment's state --------------------------------------------------------------------
  const renderPay = (f: PaymentFrame) => {
    const v = f.view;
    const fresh = !!f.plan?.pay;
    const closed = isCard && order.commercial_state === "CANCELLED";
    const summary = c.summary(money(amount.subtotal_minor, quote.currency), money(amount.shipping_minor, quote.currency));
    return (
      <section className="os-pay" data-testid="order-payment" aria-labelledby="payment-title" aria-busy={f.ariaBusy}>
        {f.signedUnknown && <StorefrontState locale={locale} variant="unknown" failed={f.readFailure} note={total} onRead={f.reread} />}
        <h2 id="payment-title">{fresh ? c.payComplete : c.payTitle}</h2>
        {fresh && <p className="order-note">{summary}</p>}
        {v && !fresh && (
          <>
            {v.test_mode && <p className="payment-test-mode" data-testid="payment-test-mode">{pay.test}</p>}
            <p data-testid="payment-status" data-state={v.payment_state} {...f.statusProps}>
              {c.payState}{c.paymentStates[v.payment_state] ?? v.payment_state}
            </p>
            <p data-testid="payment-commercial-status">{c.orderState}{c.orderStates[v.commercial_state] ?? v.commercial_state}</p>
            {f.collector && <p className="order-note" data-testid="collector-disclosure">{f.collector}</p>}
            {v.refund && v.refund.pending_minor > 0 && <p role="status" data-testid="refund-processing">{pay.refundProcessing}</p>}
            {v.refund && v.refund.refunded_minor > 0 && (
              <p data-testid="refund-succeeded">{pay.refunded.replace("{amount}", money(v.refund.refunded_minor, v.currency))}</p>
            )}
            <p className="order-note">{closed && v.payment_state === "CLOSED_UNPAID" ? c.noCharge(total) : summary}</p>
          </>
        )}
        {f.readFailure && !f.signedUnknown && (
          <StorefrontState locale={locale} variant={f.readDenied ? "forbidden" : "offline"} title={f.readDenied ? undefined : pay.failed}
            body={f.readDenied ? undefined : ""} note={f.readDenied ? undefined : ""} onRead={f.readDenied ? undefined : f.reread} />
        )}
        {f.message && !f.signedUnknown && f.message !== "submitted" && f.message !== "creating" && (
          <p role="alert" data-testid="payment-error">{pay[f.message === "opFailed" ? "failed" : f.message]}</p>
        )}
        {f.sending ? (
          <p role="status">{pay.sending}</p>
        ) : f.reading && !v ? (
          <p>{pay.loading}</p>
        ) : f.readFailure || !f.plan ? (
          v && v.payment_state === "NOT_STARTED" && !closed ? <p className="order-note">{pay.unavailable}</p> : null
        ) : (
          <>
            {fresh && <p className="order-note">{c.unconfirmed}</p>}
            {f.plan.cont && (
              <>
                <p className="order-note">{c.requested}</p>
                <p className="order-note">{c.newTab}</p>
              </>
            )}
            {f.plan.note === "cancelling" && <p className="order-note">{c.cancelling}</p>}
            {(f.plan.note === "readOnly" || f.plan.note === "cutoff" || f.plan.note === "unavailable") && <p className="order-note">{pay[f.plan.note]}</p>}
            <div className="os-actions">
              {(f.plan.pay || f.plan.cont) && (
                <button className="primary payment-pay" data-testid="pay-order" disabled={busy || f.signalling} onClick={f.pay}>
                  {f.plan.cont ? c.continuePay : c.payNow}
                </button>
              )}
              {f.plan.cancel && (
                <button data-testid="cancel-payment" disabled={busy || f.signalling} onClick={f.cancel}>{c.cancelPay}</button>
              )}
              {/* 重新查詢付款狀態 is drawn in the awaiting frame; the pending frames rely on "重新整理此訂單查詢", so the same control is offered there too (one extra secondary button). */}
              {(f.plan.pay || f.plan.cont || f.plan.cancel || f.plan.note === "cancelling") && (
                <button data-testid="refresh-order" disabled={busy} onClick={() => void recheck()}>{c.recheck}</button>
              )}
            </div>
          </>
        )}
        {closed && <a className="primary tap-link" href={cartPath(locale)}>{c.backToCart}</a>}
      </section>
    );
  };

  // ---- 前往付款: shown after the pay click opened (or is preparing) the provider tab; OrderPayment stays mounted (hidden) so polling continues -------
  const handoffScreen = handoff && (
    <section className="os os-handoff order-section" data-testid="payment-handoff" aria-labelledby="handoff-title">
      <h1 id="handoff-title">{c.handoffTitle}</h1>
      <p>{c.heading(short)} · {total}</p>
      <p>{c.handoffOpen}</p>
      <p className="order-note">{c.handoffWarn}</p>
      <button className="primary" onClick={() => { setHandoff(false); void recheck(); }}>{c.viewResult}</button>
    </section>
  );

  return (
    <>
      {handoffScreen}
      <section className="os order-section" data-testid="order-section" aria-labelledby="order-title" hidden={handoff}>
        <a className="os-back" href={`/${locale}/orders/lookup`}>{c.back}</a>
        <header className="os-head">
          <h1 id="order-title">{c.heading(short)}</h1>
          {chip && <span className="os-chip" data-testid="order-state" data-state={shownState}>{chip}</span>}
        </header>
        <div className="os-grid">
          <section className="os-card" aria-labelledby="os-items-title">
            <h2 id="os-items-title">{c.itemsTitle}</h2>
            <ul className="order-lines">
              {quote.lines.map((line) => (
                <li key={line.sku_id}>
                  <span>{line.name} × {line.quantity}</span>
                  <span>{money(line.unit_price_minor * line.quantity, quote.currency)}</span>
                </li>
              ))}
            </ul>
            <dl className="order-breakdown" data-testid="order-breakdown">
              <div><dt>{c.shipping}</dt><dd>{money(amount.shipping_minor, quote.currency)}</dd></div>
              {amount.tax_minor !== 0 && <div><dt>{common.taxes}</dt><dd>{money(amount.tax_minor, quote.currency)}</dd></div>}
              {amount.discount_minor !== 0 && <div><dt>{common.discount}</dt><dd>{money(amount.discount_minor, quote.currency)}</dd></div>}
            </dl>
            <p className="order-total"><span>{c.total}</span> <strong>{total}</strong></p>
            {collection !== null && !isCod && (
              <section data-testid="order-cvs" aria-labelledby="cvs-order-title">
                <h2 id="cvs-order-title">{cvs.pickupSection}</h2>
                <p data-testid="order-collection" data-state={collection}>
                  {collection === "PENDING" ? cvs.collection.PENDING(total) : cvs.collection[collection]}
                </p>
                {shipment && (
                  <p data-testid="order-cvs-state" data-state={shipment.state}>
                    {cvs.shipmentTitle}{colon}{cvs.shipmentStates[shipmentCopyKey(shipment.state)]}
                  </p>
                )}
                {collection === "PENDING" && <p className="order-note">{cvs.payAtPickupOrderNote}</p>}
                {shipment && (
                  <>
                    <p>{isCvsKind(shipment.chain) ? cvs.chains[shipment.chain] : shipment.chain} · {shipment.store_name} · {shipment.store_code}</p>
                    <p className="order-note">{cvs.shipmentNote}</p>
                  </>
                )}
              </section>
            )}
            {isCod && collection !== null && (
              <section data-testid="order-cod" aria-labelledby="cod-order-title">
                <h2 id="cod-order-title">{cod.orderTitle}</h2>
                <p data-testid="order-cod-state" data-state={collection}>{cod.orderStates[collection]}</p>
                <p className="cod-amount" data-testid="order-cod-amount">
                  <span>{collection === "PENDING" ? cod.due : cod.recorded}</span> <strong>{money(order.cod_collect_minor ?? 0, "TWD")}</strong>
                </p>
                {order.cod_carrier && <p className="order-note" data-testid="order-cod-carrier">{cod.orderedCarrier(cod.carriers[order.cod_carrier])}</p>}
                {collection === "PENDING" && <p className="order-note">{cod.orderNote}</p>}
              </section>
            )}
            {order.payment_mode === "bank_transfer" && (
              <BankTransfer key={`${context}:${order.order_id}:transfer`} context={context} orderID={order.order_id} locale={locale} money={money}
                refreshToken={paymentRefresh} onState={setTransferState} />
            )}
            {isCard && (
              <OrderPayment key={`${context}:${order.order_id}`} context={context} order={order} locale={locale} money={money} busy={busy}
                refreshToken={paymentRefresh} onBusy={onPaymentBusy} isSelected={isSelected} paymentSignalRef={paymentSignalRef}
                renderFrame={renderPay} onSubmitted={() => setHandoff(true)} />
            )}
          </section>
          <section className="os-card" aria-labelledby="os-delivery-title">
            <h2 id="os-delivery-title">{c.deliveryTitle}</h2>
            <address>
              {lines.map((line, i) => (
                <span key={i}>{i > 0 && <br />}{line}</span>
              ))}
            </address>
            {(isCard || order.payment_mode === "bank_transfer") && <p>{c.deliveryStatus}{delivery}</p>}
            {order.shipment && <ShipmentBlock shipment={order.shipment} locale={locale} />}
            <div className="os-actions">
              <ContactStoreButton label={c.contact} locale={locale} />
              {!(isCard && order.commercial_state === "CANCELLED") && (
                <ContactStoreButton label={c.after} locale={locale} testId="request-after-sales" />
              )}
            </div>
          </section>
        </div>
      </section>
    </>
  );
}
