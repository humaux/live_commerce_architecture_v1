// Purpose: load a current checkout offer and retain the buyer's non-PII amount acknowledgment.
// Depends on: purchase pagination/validators and the server Quote/Option DTOs.
// Used by: OrderFlow and CheckoutFlow when loading, refreshing and acknowledging their review.
// Invariants: bounded authenticated reads; server money only; no recipient data in the review journal.
import { BuyerClientError } from "./buyer-client";
import { isUnavailable, purchasePage, validOptionRow, type Option, type Quote } from "./purchase";

export function rememberCheckoutQuote(quote: Quote, context: string, old: number | null) {
  sessionStorage.setItem(`commerce-checkout-review-v2:${context}`, JSON.stringify({
    id: quote.id, cartID: quote.cart_id, total: quote.amount.total_minor,
    codDue: quote.cod_due_minor ?? null, currency: quote.currency, old,
  }));
}

export function changedCheckoutQuote(quote: Quote, context: string): number | null {
  const raw = sessionStorage.getItem(`commerce-checkout-review-v2:${context}`);
  const previous = raw ? JSON.parse(raw) : null;
  if (previous && (typeof previous.id !== "string" || !Number.isSafeInteger(previous.total) ||
    typeof previous.currency !== "string" || (previous.old !== null && !Number.isSafeInteger(previous.old)) ||
    (previous.codDue != null && !Number.isSafeInteger(previous.codDue))))
    throw new BuyerClientError("invalid_response");
  // A quote ID is immutable, but its server COD due reflects the current merchant fee.
  const cod = previous && sessionStorage.getItem(`commerce-checkout-mode-v2:${context}:${previous.id}`) === "cash_on_delivery";
  const previousTotal = cod ? previous.codDue ?? previous.total : previous?.total;
  const currentTotal = cod ? quote.cod_due_minor ?? quote.amount.total_minor : quote.amount.total_minor;
  const old = previous?.currency === quote.currency && previous.cartID === quote.cart_id
    ? previousTotal !== currentTotal ? previous.old ?? previousTotal : previous.old
    : null;
  rememberCheckoutQuote(quote, context, old);
  return old;
}

export async function checkoutOption(context: string, quote: Quote): Promise<Option> {
  let cursor = "";
  const seen = new Set<string>();
  do {
    const page = await purchasePage(
      `checkout-options?market_id=${quote.market_id}&country=${quote.country}&limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
      context,
      validOptionRow,
    );
    const found = page.items.find(
      (o): o is Option => !isUnavailable(o) && o.market_id === quote.market_id &&
        o.country === quote.country && o.method === quote.method && o.currency === quote.currency &&
        (o.delivery_kind !== "home" || o.mode === "MANUAL"),
    );
    cursor = page.next_cursor;
    if (cursor && (seen.has(cursor) || seen.size >= 100)) throw new BuyerClientError("invalid_response");
    seen.add(cursor);
    if (found) return found;
  } while (cursor);
  throw new BuyerClientError("request_failed");
}
