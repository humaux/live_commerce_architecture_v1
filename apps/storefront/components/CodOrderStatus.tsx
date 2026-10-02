import type { Locale } from "@live-commerce/i18n";
import { codCopy } from "../lib/cod-copy";
import type { Order } from "../lib/purchase";

// Buyer order page block (home-cod R5, migration 0107): the cash-on-delivery collection status. Reads only the order
// projection (BFF orders/{id} -> Go GET /v1/buyer/orders/{id}). The surcharge is never in the buyer order DTO (it lives
// on the checkout option row), so this block states the collection fact without an amount; the checkout showed the
// total plus surcharge before placement. The states are the §16.4/§16.8 collection set shared with pay_at_pickup.
export function CodOrderStatus({ order, locale }: { order: Order; locale: Locale }) {
  const copy = codCopy[locale];
  const collection = order.collection_state ?? null;
  if (collection === null) return null;
  return (
    <section data-testid="order-cod" aria-labelledby="cod-order-title">
      <h2 id="cod-order-title">{copy.orderTitle}</h2>
      <p data-testid="order-cod-state" data-state={collection}>{copy.orderStates[collection]}</p>
      {collection === "PENDING" && <p className="order-note">{copy.orderNote}</p>}
    </section>
  );
}
