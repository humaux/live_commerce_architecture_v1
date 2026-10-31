"use client";

// Purpose: render the checkout delivery selector and its pickup/free-shipping hints.
// Depends on: validated Option DTOs, localized storefront copy and shared money/shipping formatters.
// Used by: CheckoutFlow's delivery section; the parent owns selection and quote invalidation.
// Invariants: unavailable rows stay disabled; shipping progress is a hint, never authority for server quote amounts.
import type { Locale } from "@live-commerce/i18n";
import { isUnavailable, optionKey, type Option, type OptionRow } from "../../lib/purchase";
import { purchaseCopy } from "../../lib/purchase-copy";
import { cvsCopy } from "../../lib/cvs-copy";
import { fmt, shopCopy } from "../../lib/shop-copy";
import { freeShippingProgress } from "../../lib/shop-contract";
import { formatMoney } from "../../lib/money";

export default function CheckoutDeliveryOptions({ locale, options, method, chosen, subtotal, currency, disabled, onSelect }: {
  locale: Locale;
  options: OptionRow[];
  method: string;
  chosen: Option | undefined;
  subtotal: number | null;
  currency: string;
  disabled: boolean;
  onSelect: (value: string) => void;
}) {
  const copy = purchaseCopy[locale];
  const progress = chosen && subtotal !== null
    ? freeShippingProgress(subtotal, chosen.free_shipping_threshold_minor ?? null) : null;
  return <>
    <label className="delivery-label" htmlFor="delivery">
      {copy.selectDelivery}
    </label>
    <select id="delivery" value={method} disabled={disabled} onChange={(e) => onSelect(e.target.value)}>
      {options.map((o) => (
        <option key={optionKey(o)} value={optionKey(o)} disabled={isUnavailable(o)}>
          {locale === "zh-CN" ? o.name_hans : locale === "zh-TW" ? o.name_hant : o.name_en}
          {isUnavailable(o) ? ` · ${cvsCopy[locale].comingSoon}` : ""}
        </option>
      ))}
    </select>
    {chosen && chosen.delivery_kind !== "home" && (
      <p data-testid="cvs-next-step">{copy.cvs}</p>
    )}
    {progress && (
      // Hint only: the server quote remains the sole authority on shipping.
      <p className="sf-muted" data-testid="checkout-free-shipping">
        {progress.reached ? shopCopy[locale].freeShipReached
          : fmt(shopCopy[locale].freeShipRemaining, { amount: formatMoney(locale, progress.remaining, currency) })}
      </p>
    )}
  </>;
}
