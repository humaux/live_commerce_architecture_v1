"use client";

// Purpose: same-source final money, recipient and destination review with optional payment help.
// Depends on: server Quote/Destination DTOs, storefront checkout-copy and shared money formatter.
// Used by: OrderFlow immediately before order submission.
// Invariants: never computes money or stores recipient data; help is a native accessible disclosure.
import { useState, type ReactNode } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Destination, Quote } from "../../lib/purchase";
import { checkoutCopy } from "../../lib/checkout-copy";
import { formatMoney } from "../../lib/money";

export default function CheckoutReview({ locale, quote, destination, fee, timing, method, children, readOnly = false }: {
  locale: Locale; quote: Quote; destination: Destination | null; fee: number | null; timing: string; method: string; children?: ReactNode; readOnly?: boolean;
}) {
  const c = checkoutCopy[locale], [open, setOpen] = useState(false);
  const total = fee === null ? quote.amount.total_minor : quote.cod_due_minor;
  const money = (amount: number) => formatMoney(locale, amount, quote.currency);
  return (
    <section data-testid="checkout-review" aria-labelledby="checkout-review-title" style={{ borderTop: "1px solid var(--line)", paddingTop: 24, marginTop: 24, overflowWrap: "anywhere" }}>
      <h2 id="checkout-review-title">{c.review}</h2>
      <dl className="order-breakdown">
        <div><dt>{c.subtotal}</dt><dd data-testid="review-subtotal">{money(quote.amount.subtotal_minor)}</dd></div>
        <div><dt>{c.shipping}</dt><dd data-testid="review-shipping">{money(quote.amount.shipping_minor)}</dd></div>
        {fee !== null && <div><dt>{c.fee}</dt><dd data-testid="review-cod-fee">{money(fee)}</dd></div>}
        <div><dt>{c.total}</dt><dd data-testid="review-total">{total === undefined ? "—" : money(total)}</dd></div>
      </dl>
      {destination && <>
        <p><span>{c.recipient}</span> · {destination.recipient_name} · {destination.phone}</p>
        <p><span>{c.destination}</span> · {destination.pickup
          ? [destination.pickup.name, destination.pickup.code, destination.pickup.address].join(" · ")
          : [destination.home_address.region, destination.home_address.city, destination.home_address.postal_code, destination.home_address.line1, destination.home_address.line2].filter(Boolean).join(" · ")}</p>
      </>}
      {children}
      <p>{c.payment} · {method}</p>
      <p>{timing}</p>
      {!readOnly && <><button type="button" data-testid="payment-help-toggle" aria-expanded={open} aria-controls="payment-help-content"
        style={{ minHeight: 44, minWidth: 44, display: "flex", alignItems: "center", gap: 8, marginTop: 12 }} onClick={() => setOpen(!open)}>
        {open ? c.close : c.open}
        <svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" style={{ transform: open ? "rotate(180deg)" : undefined }}><path d="m6 9 6 6 6-6" /></svg>
      </button>
      <p id="payment-help-content" data-testid="payment-help-content" hidden={!open}>{c.help}</p></>}
    </section>
  );
}
