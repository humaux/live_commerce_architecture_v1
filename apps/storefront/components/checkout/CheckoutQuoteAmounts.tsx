// Purpose: quoted line amounts, breakdown and expiry shown before checkout commitment.
// Depends on: server Quote DTO, purchase-copy and shared displayTime formatter.
// Used by: CheckoutFlow's existing quotation section.
// Invariants: preserves server totals verbatim; expiry is displayed in the storefront timezone.
import type { Locale } from "@live-commerce/i18n";
import type { Quote } from "../../lib/purchase";
import { purchaseCopy } from "../../lib/purchase-copy";
import { displayTime } from "../../../../packages/format/src/index";

export default function CheckoutQuoteAmounts({ quote, locale, money }: {
  quote: Quote; locale: Locale; money: (amount: number, currency: string) => string;
}) {
  const copy = purchaseCopy[locale];
  return <>
    <ul>
      {quote.lines.map((line) => (
        <li key={line.sku_id}>
          <span>
            {line.name} · {line.code} × {line.quantity}
          </span>
          <span>
            {money(
              line.unit_price_minor * line.quantity,
              quote.currency,
            )}
          </span>
        </li>
      ))}
    </ul>
    <dl>
      <div>
        <dt>{copy.shipping}</dt>
        <dd>
          {money(quote.amount.shipping_minor, quote.currency)}
        </dd>
      </div>
      <div>
        <dt>{copy.taxes}</dt>
        <dd>{money(quote.amount.tax_minor, quote.currency)}</dd>
      </div>
      <div>
        <dt>{copy.discount}</dt>
        <dd>
          {money(quote.amount.discount_minor, quote.currency)}
        </dd>
      </div>
      <div className="total">
        <dt>{copy.orderTotal}</dt>
        <dd>{money(quote.amount.total_minor, quote.currency)}</dd>
      </div>
    </dl>
    <p>
      {copy.expires}{" "}
      {displayTime(locale, quote.expires_at)} · {copy.taipeiTime}
    </p>
    <p>{copy.noPayment}</p>
  </>;
}
