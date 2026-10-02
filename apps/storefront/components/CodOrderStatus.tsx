import type { Locale } from "@live-commerce/i18n";
import { codCopy } from "../lib/cod-copy";
import type { Order } from "../lib/purchase";
import { formatMoney } from "../lib/money";

export function CodAmount({ locale, total, fee, pending = true }: { locale: Locale; total: string; fee: string; pending?: boolean }) {
  const copy = codCopy[locale];
  return <>
    <span>{pending ? copy.due : copy.recorded}</span>{" "}
    <strong>{total}</strong>{" "}
    <small>{copy.includesFee(fee)}</small>
  </>;
}

// Use the validated order's immutable money projection, never the store's current option/settings.
export function CodOrderStatus({ order, locale }: { order: Order; locale: Locale }) {
  const copy = codCopy[locale];
  const collection = order.collection_state ?? null;
  if (collection === null) return null;
  return (
    <section data-testid="order-cod" aria-labelledby="cod-order-title">
      <h2 id="cod-order-title">{copy.orderTitle}</h2>
      <p className="cod-amount" data-testid="order-cod-amount">
        <CodAmount locale={locale} total={formatMoney(locale, order.cod_collect_minor ?? 0, "TWD")} fee={formatMoney(locale, order.cod_surcharge_minor ?? 0, "TWD")} pending={collection === "PENDING"} />
      </p>
      <p data-testid="order-cod-state" data-state={collection}>{copy.orderStates[collection]}</p>
      {collection === "PENDING" && <p className="order-note">{copy.orderNote}</p>}
    </section>
  );
}
