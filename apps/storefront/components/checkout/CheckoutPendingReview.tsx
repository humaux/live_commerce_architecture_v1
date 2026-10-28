"use client";

// Purpose: read-only review of the original unresolved checkout; no new checkout operation.
// Depends on: validated pending intent, original server Quote and authenticated currentDestination read.
// Used by: CheckoutFlow while its original order-lock guard removes the writable address controller.
// Invariants: destination id must equal the original intent; no recipient persistence, amount arithmetic or new key.
import { useEffect, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { currentDestination, pendingPurchase, type Destination, type Quote } from "../../lib/purchase";
import { checkoutCopy } from "../../lib/checkout-copy";
import { bankTransferCopy } from "../../lib/bank-transfer-copy";
import { codCopy } from "../../lib/cod-copy";
import CheckoutReview from "./CheckoutReview";

export default function CheckoutPendingReview({ context, quote, locale, onCodFee, onReviewState }: {
  context: string; quote: Quote; locale: Locale;
  onCodFee: (fee: number | null) => void;
  onReviewState: (state: { quoteID: string; ready: boolean; timing: string }) => void;
}) {
  const [snapshot, setSnapshot] = useState<{ destination: Destination | null; fee: number | null; timing: string; method: string } | null>(null);
  useEffect(() => {
    let active = true;
    setSnapshot(null);
    void (async () => {
      try {
        const intent = pendingPurchase(context);
        if (intent?.kind !== "checkout" || intent.body.quote_id !== quote.id) return;
        const mode = intent.body.payment_mode ?? "card", c = checkoutCopy[locale];
        const timing = mode === "cash_on_delivery" ? c.cod : mode === "pay_at_pickup" ? c.pickup : c.online;
        // This is the original option's server fee already persisted by checkoutInput, never a computed or freshly selected amount.
        const fee = mode === "cash_on_delivery" ? intent.body.expected_cod_surcharge_minor ?? null : null;
        const destination = await currentDestination(context);
        if (!active) return;
        const current = pendingPurchase(context);
        if (current?.kind !== "checkout" || current.key !== intent.key) return;
        const method = mode === "cash_on_delivery" ? codCopy[locale].orderTitle : mode === "pay_at_pickup" ? c.pickup : mode === "bank_transfer" ? bankTransferCopy[locale].payBank : bankTransferCopy[locale].payCard;
        setSnapshot({ destination: destination?.id === intent.body.destination_id ? destination : null, fee, timing, method });
        onCodFee(fee);
        onReviewState({ quoteID: quote.id, ready: false, timing });
      } catch { /* Recovery remains the original explicit status query; a failed read never enables a submit. */ }
    })();
    return () => { active = false; };
  }, [context, quote.id, locale, onCodFee, onReviewState]);
  return snapshot && <CheckoutReview locale={locale} quote={quote} {...snapshot} readOnly />;
}
