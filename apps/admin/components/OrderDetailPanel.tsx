"use client";

// Inline detail row of the merchant orders page (items, totals, recipient, statuses and the refund / transfer / CVS / COD / shipment
// sections), split out of MerchantOrders.tsx (G-UI3 legacy ceiling). A plain render function with no state of its own: MerchantOrders
// owns the list, polling and selection and passes the loaded detail in; the BFF routes are listed in the section components.
import type { Locale } from "@live-commerce/i18n";
import { money } from "@/lib/client";
import type { OrderActions, OrderDetail } from "@/lib/orders-model";
import type { OrdersCopy } from "@/lib/orders-copy";
import { codCopy } from "@/lib/cod-copy";
import { OrderRefunds } from "./OrderRefunds";
import { OrderShipment } from "./OrderShipment";
import { OrderCvsShipment } from "./OrderCvsShipment";
import { OrderBankTransfer } from "./OrderBankTransfer";
import { OrderCodCollection } from "./OrderCodCollection";

// Refund section applies once money was captured (stripe-refund-v1 §4.3); earlier payment states have nothing to refund.
const capturedPayment = ["CAPTURED", "PARTIALLY_REFUNDED", "REFUNDED", "REVIEW_REQUIRED"];

export function amount(locale: Locale, currency: string, minor: number) {
  return money(locale, currency, minor);
}
export function badge(state: string, c: OrdersCopy) {
  return (
    <span
      className={`orders-badge orders-badge-${state.toLowerCase()}`}
      data-state={state}
    >
      {c.statuses[state as keyof OrdersCopy["statuses"]]}
    </span>
  );
}
export type Sections = {
  store: string;
  actions: OrderActions;
  boundary: string;
  onChanged: () => Promise<boolean>;
};
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
        <div className="orders-items-scroll">
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
        </div>
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
            <OrderRefunds
              store={sections.store}
              detail={detail}
              locale={locale}
              c={c}
              canRefund={sections.actions.refund}
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
            />
          )}
        </section>
      )}
    </section>
  );
}
