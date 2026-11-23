// Purpose: quoted line amounts, breakdown and expiry shown before checkout commitment.
// Depends on: server Quote DTO, purchase-copy and shared displayTime formatter.
// Used by: CheckoutFlow's existing quotation section.
// Invariants: preserves server totals verbatim; expiry is displayed in the storefront timezone.
import type { Locale } from "@live-commerce/i18n";
import { useRef, type ComponentProps } from "react";
import type { Quote } from "../../lib/purchase";
import { purchaseCopy } from "../../lib/purchase-copy";
import { displayTime } from "../../../../packages/format/src/index";
import PromoCode from "../PromoCode";

export default function CheckoutQuotation({ quote, locale, money, hidden, disabled, promo, onChangeDelivery }: {
  quote: Quote; locale: Locale; money: (amount: number, currency: string) => string;
  hidden: boolean; disabled: boolean; onChangeDelivery: () => void;
  promo?: Omit<ComponentProps<typeof PromoCode>, "locale" | "quote" | "money">;
}) {
  const copy = purchaseCopy[locale];
  const section = useRef<HTMLElement>(null);
  return <section className="quotation" id="checkout-details" hidden={hidden} ref={section} aria-labelledby="quote-title">
    <h2 id="quote-title">{copy.lines}</h2>
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
    <p>{copy.unpaid}</p>
    {promo && <PromoCode {...promo} quote={quote} locale={locale} money={money} />}
    <button className="quote-navigation" disabled={disabled}
      onClick={() => section.current?.scrollIntoView({ behavior: "auto", block: "start" })}>{copy.viewQuote}</button>
    <button className="text-button" disabled={disabled} onClick={onChangeDelivery}>{copy.changeDelivery}</button>
  </section>;
}
