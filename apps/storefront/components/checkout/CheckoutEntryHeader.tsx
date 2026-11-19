// Purpose: identify the merchant and navigate from the 782 cart entry.
// Depends on: the server-provided shop name and storefront copy.
// Used by: CheckoutFlow. History visibility and actions remain owned by the flow.
import type { ReactNode } from "react";
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { historyCopy } from "../../lib/history-copy";
import { shopCopy } from "../../lib/shop-copy";
import { useShopName } from "../ShopName";

export default function CheckoutEntryHeader({ locale, children }: { locale: Locale; children: ReactNode }) {
  const shopName = useShopName();
  const copy = shopCopy[locale];
  return <header className="checkout-top checkout-entry-header">
    <div className="checkout-entry-brand" data-testid="checkout-shop-name">{shopName}</div>
    <Link className="checkout-entry-back" href={`/${locale}/cart`}><span aria-hidden="true">‹</span> {copy.checkoutBack}</Link>
    {children}
    <h1 id="checkout-title" tabIndex={-1}>{copy.checkoutTitle}</h1>
  </header>;
}

export function CheckoutHistoryControl({ locale, historyOpen, hidden, disabled, onToggle }: {
  locale: Locale;
  historyOpen: boolean;
  hidden: boolean;
  disabled: boolean;
  onToggle: () => void;
}) {
  const copy = historyCopy[locale];
  return <nav className="purchase-navigation" hidden={hidden} aria-label={copy.title}>
    <button data-testid="toggle-order-history" disabled={disabled} onClick={onToggle}>
      {historyOpen ? copy.back : copy.title}
    </button>
  </nav>;
}
