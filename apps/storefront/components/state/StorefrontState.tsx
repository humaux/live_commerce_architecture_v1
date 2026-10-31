// Purpose: One signed recovery panel and inline read failure for buyer pages.
// Depends on: @live-commerce/i18n buyerSignedStateCopy, packages/ui/src/ds/States StatePanel/InlineError and state.css.
// Used by: CartView, OrderPage, OrderLink, OrderPayment and LookupForm.
// Invariants: no data fetching, order creation or payment signals; recovery callbacks read the original task only.
"use client";
import { buyerSignedStateCopy, type Locale } from "@live-commerce/i18n";
import { InlineError, StatePanel } from "../../../../packages/ui/src/ds/States";
import "../../../../packages/ui/src/tokens.css";
import "./state.css";

type Variant = "empty" | "forbidden" | "offline" | "expired" | "unknown";
export function StorefrontReadFailure({ message }: { message: string }) {
  return <InlineError message={message} />;
}
export function StorefrontState({ locale, variant, onRead, failed = false, title, body, note }: {
  locale: Locale; variant: Variant; onRead?: () => void; failed?: boolean;
  title?: string; body?: string; note?: string;
}) {
  const c = buyerSignedStateCopy[locale];
  const href = variant === "empty" ? `/${locale}/products` : variant === "expired" ? `/${locale}/orders/lookup` : `/${locale}`;
  const label = variant === "empty" ? c.browse : variant === "expired" ? c.lookup : c.home;
  return <div className="sf-state">
    <StatePanel variant={variant} title={title ?? c[variant]} body={body ?? c[`${variant}Body`]}
      note={note ?? (variant === "unknown" ? undefined : c[`${variant}Note`])}
      action={onRead ? { label: variant === "unknown" ? c.check : c.retry, onClick: onRead } : { label, href }}
      status={failed ? <StorefrontReadFailure message={variant === "unknown" ? c.unknownFailed : c.failed} /> : undefined} />
  </div>;
}
