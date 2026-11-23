"use client";

// Purpose: render the checkout delivery section, selector and pickup/free-shipping hints.
// Depends on: validated Option DTOs, localized storefront copy and shared money/shipping formatters.
// Used by: CheckoutFlow's delivery section; the parent owns selection and quote invalidation.
// Invariants: unavailable rows stay disabled; shipping progress is a hint, never authority for server quote amounts.
import { useRef } from "react";
import { deliveryOptionGroups, type Locale } from "@live-commerce/i18n";
import { isUnavailable, optionKey, type Option, type OptionRow } from "../../lib/purchase";
import { purchaseCopy } from "../../lib/purchase-copy";
import { cvsCopy } from "../../lib/cvs-copy";
import { fmt, shopCopy } from "../../lib/shop-copy";
import { freeShippingProgress } from "../../lib/shop-contract";
import { formatMoney } from "../../lib/money";

type DeliveryOptionsProps = {
  locale: Locale;
  options: OptionRow[];
  method: string;
  chosen: Option | undefined;
  subtotal: number | null;
  currency: string;
  disabled: boolean;
  expanded: boolean;
  onClose: () => void;
  onSelect: (value: string) => void;
};

function CheckoutDeliveryOptions({ locale, options, method, chosen, subtotal, currency, disabled, expanded, onClose, onSelect }: DeliveryOptionsProps) {
  const copy = purchaseCopy[locale];
  const groups = deliveryOptionGroups(locale, options);
  const progress = chosen && subtotal !== null
    ? freeShippingProgress(subtotal, chosen.free_shipping_threshold_minor ?? null) : null;
  return <>
    <label className="delivery-label" htmlFor="delivery">
      {copy.selectDelivery}
    </label>
    <select id="delivery" value={method} disabled={disabled} size={expanded ? Math.min(options.length, 6) : undefined}
      onBlur={onClose} onKeyDown={(e) => { if (e.key === "Escape") onClose(); }}
      onChange={(e) => { onSelect(e.target.value); onClose(); }}>
      {groups.map(group => <optgroup key={group.label} label={group.label}>
        {group.items.map(o => <option key={optionKey(o)} value={optionKey(o)} disabled={isUnavailable(o)}>
          {locale === "zh-CN" ? o.name_hans : locale === "zh-TW" ? o.name_hant : o.name_en}
          {isUnavailable(o) ? ` · ${cvsCopy[locale].comingSoon}` : ""}
        </option>)}
      </optgroup>)}
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

export default function CheckoutDeliverySection({
  showChooser, chooserDisabled, onExpand, onLoad,
  onQuote, quoteDisabled, onMore, moreDisabled, ...selector
}: DeliveryOptionsProps & {
  showChooser: boolean;
  chooserDisabled: boolean;
  onExpand: () => void;
  onLoad: () => void;
  onQuote?: () => void;
  quoteDisabled: boolean;
  onMore?: () => void;
  moreDisabled: boolean;
}) {
  const copy = purchaseCopy[selector.locale];
  const section = useRef<HTMLElement>(null);

  function chooseDelivery() {
    onExpand();
    requestAnimationFrame(() => {
      document.getElementById("delivery")?.focus();
      section.current?.scrollIntoView({ block: "start" });
    });
    if (!selector.options.length) onLoad();
  }

  return (
    <section className="delivery-section" data-selected={!!selector.chosen} ref={section} aria-labelledby="delivery-title">
      <h2 id="delivery-title">{copy.deliveryTitle}</h2>
      <div className="checkout-entry-delivery-controls">
        {selector.options.length ? <CheckoutDeliveryOptions {...selector} /> : <p>{copy.noDelivery}</p>}
        {showChooser && <button type="button" aria-controls="delivery" disabled={chooserDisabled} onClick={chooseDelivery}>{copy.delivery}</button>}
        {onQuote && <button type="button" disabled={quoteDisabled} onClick={onQuote}>{copy.quote}</button>}
      </div>
      {onMore && <button disabled={moreDisabled} onClick={onMore}>{copy.more}</button>}
    </section>
  );
}
