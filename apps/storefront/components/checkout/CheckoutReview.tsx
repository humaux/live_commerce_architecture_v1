"use client";

// Purpose: same-source final money, recipient and destination review with optional payment help.
// Depends on: server Quote/Destination DTOs, storefront checkout-copy and shared money formatter.
// Used by: OrderFlow before submission and CheckoutPendingReview during original-order recovery.
// Invariants: never computes money or stores recipient data; unresolved review withholds payable; help is a native accessible disclosure.
import { type ReactNode } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Destination, Quote } from "../../lib/purchase";
import { checkoutCopy } from "../../lib/checkout-copy";
import { cvsCopy } from "../../lib/cvs-copy";
import { isCvsKind } from "../../lib/cvs-contract";
import { formatMoney } from "../../lib/money";

export default function CheckoutReview({ locale, quote, destination, fee, timing, method, children, readOnly = false, detailsOpen = false }: {
  locale: Locale; quote: Quote; destination: Destination | null; fee: number | null; timing: string; method: string; children?: ReactNode; readOnly?: boolean; detailsOpen?: boolean;
}) {
  const c = checkoutCopy[locale];
  // A re-read quote is the current offer, not the unresolved order's monetary snapshot.
  const total = readOnly ? undefined : fee === null ? quote.amount.total_minor : quote.cod_due_minor;
  const money = (amount: number) => formatMoney(locale, amount, quote.currency);
  return (
    <section data-testid="checkout-review" className="checkout-review" aria-label={c.review}>
      {destination && <div className="checkout-recipient">
        <p>{destination.pickup
          ? [isCvsKind(destination.kind) ? cvsCopy[locale].chains[destination.kind] : null, destination.pickup.name, destination.pickup.code, destination.pickup.address].filter(Boolean).join(" · ")
          : [c.homeDelivery, destination.home_address.region, destination.home_address.city, destination.home_address.postal_code, destination.home_address.line1, destination.home_address.line2].filter(Boolean).join(" · ")}</p>
        <p>{destination.recipient_name}</p>
        <p>{c.phone}: {destination.phone}</p>
      </div>}
      {children}
      <div hidden={!detailsOpen || readOnly}>
        <dl className="order-breakdown">
          <div><dt>{c.subtotal}</dt><dd data-testid="review-subtotal">{money(quote.amount.subtotal_minor)}</dd></div>
          <div><dt>{c.shipping}</dt><dd data-testid="review-shipping">{money(quote.amount.shipping_minor)}</dd></div>
          {fee !== null && <div><dt>{c.fee}</dt><dd data-testid="review-cod-fee">{money(fee)}</dd></div>}
          <div><dt>{c.total}</dt><dd data-testid="review-total">{total === undefined ? "—" : money(total)}</dd></div>
        </dl>
        <p>{c.payment} · {method}</p>
        <p>{timing}</p>
      </div>
    </section>
  );
}
