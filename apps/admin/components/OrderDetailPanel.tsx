// Purpose: Renders order detail and composes the supplied action sections (refund, returns, cancel, shipment, COD…).
// Depends on: react, @live-commerce/i18n, @live-commerce/ui, @/lib/presentation-copy, @/lib/client, @/lib/orders-model, @/lib/orders-copy,
//   @/lib/cod-copy, @/lib/returns-model, @/lib/returns-client, @/lib/returns-copy, ./OrderRefunds, ./OrderReturns, ./OrderShipment,
//   ./OrderCvsShipment, ./OrderBankTransfer, ./OrderCodCollection
// Used by: apps/admin/components/MerchantOrders.tsx
"use client";

// Inline detail row of the merchant orders page (items, totals, recipient, statuses and the refund / returns / cancel /
// transfer / CVS / COD / shipment sections), split out of MerchantOrders.tsx (G-UI3 legacy ceiling). detailPanel is a plain
// render function: MerchantOrders owns the list, polling and selection and passes the loaded detail in; the cancel button is
// the one stateful child defined here (returns-v1 §3), the rest live in their section components.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { Badge, TableFrame } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";
import { money } from "@/lib/client";
import type { OrderActions, OrderDetail } from "@/lib/orders-model";
import type { OrdersCopy } from "@/lib/orders-copy";
import { codCopy } from "@/lib/cod-copy";
import { cancellableStates, cancelOrderBody, cancelReasonText, type CancellableState } from "@/lib/returns-model";
import { postCancelOrder } from "@/lib/returns-client";
import { returnsCopy, returnsError, type ReturnsCopy } from "@/lib/returns-copy";
import { OrderRefunds } from "./OrderRefunds";
import { OrderReturns } from "./OrderReturns";
import { OrderShipment } from "./OrderShipment";
import { OrderCvsShipment } from "./OrderCvsShipment";
import { OrderBankTransfer } from "./OrderBankTransfer";
import { OrderCodCollection } from "./OrderCodCollection";

// Refund section applies once money was captured (stripe-refund-v1 §4.3); earlier payment states have nothing to refund.
const capturedPayment = ["CAPTURED", "PARTIALLY_REFUNDED", "REFUNDED", "REVIEW_REQUIRED"];

/** Formats a minor-unit order amount for display. */
export function amount(locale: Locale, currency: string, minor: number) {
  return money(locale, currency, minor);
}
/** Renders the localized order-state badge. */
export function badge(state: string, c: OrdersCopy) {
  return (
    <Badge
      tone={/CAPTURED|FULFILLED|DELIVERED|COLLECTED/.test(state) ? "success" : /FAILED|CANCELLED|EXPIRED/.test(state) ? "danger" : /AWAITING|PENDING|REVIEW/.test(state) ? "warning" : "neutral"}
      className={`orders-badge orders-badge-${state.toLowerCase()}`}
      data-state={state}
    >
      {c.statuses[state as keyof OrdersCopy["statuses"]]}
    </Badge>
  );
}
/** Describes Sections values shared by this presentation module. */
export type Sections = {
  store: string;
  actions: OrderActions;
  boundary: string;
  onChanged: () => Promise<boolean>;
  // W3-07B: returns the group-block hint text when the order sits in an OPEN parcel group this session knows about;
  // undefined when ungrouped (or unknown — the server in_parcel_group guard stays the authority).
  parcelBlockText?: (orderID: string) => string | undefined;
  // W3-07B: told the refusal code of a single-order shipment so the page can re-read the parcel state (in_parcel_group).
  onShipmentRefused?: (code: string) => void;
};

/**
 * Merchant order cancel (returns-v1 §3): a dialog asking the reason, warning that reserved stock is released.
 * The CAS is the commercial state the merchant sees; refusal codes (payment_in_flight, refund_first, already_shipped,
 * not_cancellable…) render next to the form. The cancel NEVER starts a refund (I13); refund_first points at the refund section.
 */
function CancelOrderSection({
  store,
  detail,
  c,
  boundary,
  onChanged,
}: {
  store: string;
  detail: OrderDetail;
  c: ReturnsCopy;
  boundary: string;
  onChanged: () => Promise<boolean>;
}) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [problem, setProblem] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);
  const reasonInput = useRef<HTMLInputElement>(null);
  const pending = useRef<{ key: string; body: string } | null>(null);

  useEffect(() => {
    const element = dialog.current;
    if (!element) return;
    if (open && !element.open) element.showModal();
    if (!open && element.open) element.close();
  }, [open]);
  useEffect(() => {
    if (open) reasonInput.current?.focus();
  }, [open]);

  function begin() {
    pending.current = null;
    setReason("");
    setProblem("");
    setUncertain(false);
    setNotice("");
    setOpen(true);
  }
  function finish() {
    setOpen(false);
    if (uncertain) void onChanged(); // unknown outcome: re-read the order state
    setUncertain(false);
  }
  async function submit() {
    if (busy) return;
    const text = cancelReasonText(reason);
    const body = text && cancelOrderBody(detail.commercial_state, text);
    if (!body) {
      setProblem(c.cancelInvalid);
      return;
    }
    if (pending.current?.body !== body) pending.current = { key: `order-cancel-${crypto.randomUUID()}`, body };
    setBusy(true);
    setProblem("");
    const result = await postCancelOrder(store, detail.order_id, pending.current.key, body, boundary);
    setBusy(false);
    if (result.ok) {
      pending.current = null;
      setOpen(false);
      setUncertain(false);
      setNotice(c.cancelDone);
      void onChanged();
      return;
    }
    if (result.uncertain) {
      setUncertain(true);
      setProblem(c.uncertain);
      return;
    }
    pending.current = null;
    setProblem(returnsError(c, result.code));
    if (result.code === "state_changed" || result.code === "already_cancelled") void onChanged(); // stale view: re-read
  }

  return (
    <section className="orders-section" data-testid="order-cancel" aria-label={c.cancelOrder}>
      <h2>{c.cancelOrder}</h2>
      <p className="orders-empty">{c.cancelIntro}</p>
      <button type="button" className="orders-section-action" data-testid="order-cancel-open" onClick={begin}>
        {c.cancelOrder}
      </button>
      {notice && <p className="orders-notice" role="status" data-testid="order-cancel-notice">{notice}</p>}
      <dialog
        ref={dialog}
        className="orders-dialog"
        aria-labelledby={`cancel-title-${detail.order_id}`}
        data-testid="order-cancel-dialog"
        onClose={() => {
          if (open) finish();
        }}
      >
        {open && (
          <form
            onSubmit={(event) => {
              event.preventDefault();
              void submit();
            }}
          >
            <h2 id={`cancel-title-${detail.order_id}`}>{c.cancelTitle}</h2>
            <p>{c.cancelIntro}</p>
            <label>
              {c.cancelReason}
              <input
                ref={reasonInput}
                data-testid="order-cancel-reason"
                value={reason}
                maxLength={60}
                placeholder={c.cancelReasonPlaceholder}
                autoComplete="off"
                aria-invalid={reason !== "" && cancelReasonText(reason) === null}
                onChange={(event) => setReason(event.target.value)}
              />
            </label>
            {problem && <p className="orders-bad" role="alert" data-testid="order-cancel-problem">{problem}</p>}
            <div className="orders-dialog-actions">
              <button type="button" disabled={busy} onClick={finish}>
                {uncertain ? c.close2 : c.cancel}
              </button>
              <button type="submit" className="primary" data-testid="order-cancel-submit" disabled={busy || !cancelReasonText(reason)}>
                {busy ? c.sending : uncertain ? c.retrySame : c.cancelSubmit}
              </button>
            </div>
          </form>
        )}
      </dialog>
    </section>
  );
}

/** Composes loaded order detail with the supplied action sections. */
export function detailPanel(detail: OrderDetail, locale: Locale, c: OrdersCopy, sections: Sections) {
  const m = (value: number) => amount(locale, detail.currency, value);
  const dest = detail.destination;
  const address = dest.pickup
    ? dest.pickup.address
    : [
        dest.home_address.region,
        dest.home_address.city,
        dest.home_address.postal_code,
        dest.home_address.line1,
        dest.home_address.line2,
      ]
        .filter(Boolean)
        .join(" · ");
  return (
    <section
      className="orders-expanded"
      data-testid="order-detail"
      aria-label={`${c.order} ${detail.order_id}`}
    >
      <div className="orders-items">
        <h2>{c.items}</h2>
        <TableFrame label={c.items} scrollHint={presentationCopy[locale].scroll} scrollClassName="orders-items-scroll">
          <table>
            <thead>
              <tr>
                <th>{c.product}</th>
                <th>{c.unit}</th>
                <th>{c.quantity}</th>
                <th>{c.amount}</th>
              </tr>
            </thead>
            <tbody>
              {detail.items.map((item) => (
                <tr key={item.sku_id}>
                  <td>
                    <strong>{item.name}</strong>
                    <small>{item.code}</small>
                  </td>
                  <td>{m(item.unit_price_minor)}</td>
                  <td>{item.quantity}</td>
                  <td>{m(item.amount.total_minor)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableFrame>
        <dl className="orders-totals">
          <div>
            <dt>{c.subtotal}</dt>
            <dd>{m(detail.totals.subtotal_minor)}</dd>
          </div>
          <div>
            <dt>{c.discount}</dt>
            <dd>{m(detail.totals.discount_minor)}</dd>
          </div>
          <div>
            <dt>{c.shipping}</dt>
            <dd>{m(detail.totals.shipping_minor)}</dd>
          </div>
          <div>
            <dt>{c.shippingTax}</dt>
            <dd>{m(detail.totals.shipping_tax_minor)}</dd>
          </div>
          <div>
            <dt>{c.tax}</dt>
            <dd>{m(detail.totals.tax_minor)}</dd>
          </div>
          <div className="orders-grand">
            <dt>{c.grandTotal}</dt>
            <dd>{m(detail.totals.total_minor)}</dd>
          </div>
        </dl>
      </div>
      <div className="orders-recipient">
        <h2>{c.recipient}</h2>
        <dl>
          <div>
            <dt>{c.name}</dt>
            <dd>{dest.recipient_name}</dd>
          </div>
          <div>
            <dt>{c.phone}</dt>
            <dd>{dest.phone}</dd>
          </div>
          <div>
            <dt>{c.method}</dt>
            <dd>{c.statuses[dest.kind]}</dd>
          </div>
          {dest.pickup && (
            <>
              <div>
                <dt>{c.pickupName}</dt>
                <dd>{dest.pickup.name}</dd>
              </div>
              <div>
                <dt>{c.pickupCode}</dt>
                <dd>{dest.pickup.code}</dd>
              </div>
              {detail.pickup_source && (
                <div>
                  <dt>{c.sourceLabel}</dt>
                  <dd data-testid="pickup-source" data-source={detail.pickup_source}>
                    {c.pickupSources[detail.pickup_source]}
                  </dd>
                </div>
              )}
            </>
          )}
          <div>
            <dt>{c.address}</dt>
            <dd>{address}</dd>
          </div>
        </dl>
        <p>{c.snapshot}</p>
      </div>
      <div className="orders-statuses">
        <h2>{c.commercial}</h2>
        <dl>
          <div>
            <dt>{c.commercial}</dt>
            <dd>
              {badge(detail.commercial_state, c)}
              {detail.source === "merchant_manual" && (
                <span className="orders-badge" data-testid="order-detail-manual">
                  {c.manualOrder}
                </span>
              )}
            </dd>
          </div>
          <div>
            <dt>{c.payment}</dt>
            <dd>{badge(detail.payment_state, c)}</dd>
          </div>
          <div>
            <dt>{c.fulfillment}</dt>
            <dd>{badge(detail.fulfillment_state, c)}</dd>
          </div>
          <div>
            <dt>{c.work}</dt>
            <dd>{badge(detail.work_state, c)}</dd>
          </div>
          <div>
            <dt>{c.payMode}</dt>
            <dd data-testid="order-pay-mode">{c.payModes[detail.payment_mode]}</dd>
          </div>
          {detail.collection_state && (
            <div>
              <dt>{c.collectionLabel}</dt>
              <dd data-testid="order-collection-state" data-state={detail.collection_state}>
                {detail.payment_mode === "cash_on_delivery"
                  ? codCopy[locale].orderStates[detail.collection_state]
                  : c.collectionStates[detail.collection_state]}
              </dd>
            </div>
          )}
          {detail.payment_mode === "cash_on_delivery" && <div><dt>{codCopy[locale].collectAmount}</dt><dd data-testid="order-collect-amount">{amount(locale, detail.currency, detail.cod_collect_minor ?? 0)}</dd></div>}
        </dl>
        {detail.test_mode && (
          <p className="orders-test" data-testid="order-test-mode">
            {c.test}
          </p>
        )}
      </div>
      {sections.boundary && (
        <section className="orders-sections">
          {/* storefront-v2 §C: a bank-transfer order has no card payment to refund; the merchant decides the transfer here. */}
          {detail.payment_mode === "bank_transfer" && (
            <OrderBankTransfer
              store={sections.store}
              detail={detail}
              locale={locale}
              canDecide={sections.actions.refund}
              boundary={sections.boundary}
              onChanged={sections.onChanged}
            />
          )}
          {capturedPayment.includes(detail.payment_state) && (
            // #order-refunds: the returns panel's refund hint deep-links here (Integrator 裁决: returns never refunds).
            <div id="order-refunds">
              <OrderRefunds
                store={sections.store}
                detail={detail}
                locale={locale}
                c={c}
                canRefund={sections.actions.refund}
                boundary={sections.boundary}
                onChanged={sections.onChanged}
              />
            </div>
          )}
          {/* returns-v1 §3: merchant cancel is offered only while the commercial state is in the §6 cancellable set;
              later states get the refusal copy from the server (already_shipped, has_returns…) or use returns. */}
          {sections.actions.fulfillment_write && cancellableStates.includes(detail.commercial_state as CancellableState) && (
            <CancelOrderSection
              store={sections.store}
              detail={detail}
              c={returnsCopy[locale]}
              boundary={sections.boundary}
              onChanged={sections.onChanged}
            />
          )}
          {/* returns-v1 §2: returns exist only once the order has shipped (merchant parcel or provider label). */}
          {(detail.fulfillment_state === "MERCHANT_SHIPPED" || detail.fulfillment_state === "PROVIDER_LABEL_CREATED") && (
            <OrderReturns
              store={sections.store}
              detail={detail}
              locale={locale}
              canWrite={sections.actions.fulfillment_write}
              boundary={sections.boundary}
              onChanged={sections.onChanged}
            />
          )}
          {dest.pickup && (detail.commercial_state === "CONFIRMED" || detail.payment_mode === "pay_at_pickup") && (
            <OrderCvsShipment
              store={sections.store}
              detail={detail}
              locale={locale}
              c={c}
              canWrite={sections.actions.fulfillment_write}
              boundary={sections.boundary}
              onChanged={sections.onChanged}
            />
          )}
          {/* home-cod R5: COD shares the collection state machine; the panel sits where the pickup panel would (home, not pickup). */}
          {detail.payment_mode === "cash_on_delivery" && (
            <OrderCodCollection
              store={sections.store}
              detail={detail}
              locale={locale}
              c={c}
              canWrite={sections.actions.fulfillment_write}
              boundary={sections.boundary}
              onChanged={sections.onChanged}
            />
          )}
          {(detail.commercial_state === "CONFIRMED" || detail.payment_mode === "cash_on_delivery") && (
            <OrderShipment
              store={sections.store}
              // A pay-at-pickup order never has a payment work item, so its list-side work_state is NONE. The
              // 0063 form's MD6 eligibility hint reads READY as "nothing blocks shipping"; for that mode that
              // hint is collection PENDING instead (hint only: record_manual_shipment re-checks in SQL).
              detail={
                (detail.payment_mode === "pay_at_pickup" && detail.collection_state === "PENDING") ||
                // A confirmed bank-transfer order has no work item either; CONFIRMED here means the merchant confirmed it.
                (detail.payment_mode === "bank_transfer" && detail.commercial_state === "CONFIRMED") ||
                // A cash-on-delivery order stays AWAITING_COLLECTION with no work item; READY here means "nothing blocks shipping".
                (detail.payment_mode === "cash_on_delivery" && detail.collection_state === "PENDING")
                  ? { ...detail, work_state: "READY" }
                  : detail
              }
              locale={locale}
              c={c}
              canWrite={sections.actions.fulfillment_write}
              boundary={sections.boundary}
              onChanged={sections.onChanged}
              parcelBlock={sections.parcelBlockText?.(detail.order_id)}
              onRefused={sections.onShipmentRefused}
            />
          )}
        </section>
      )}
    </section>
  );
}
