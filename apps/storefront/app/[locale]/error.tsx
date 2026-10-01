"use client";

// Branded error state for a storefront page whose data could not be read (Go unreachable, 5xx, drifted response). The
// layout stays (header/footer), only the page body is replaced, and Retry re-renders the segment. No BFF/Go call. The locale
// is read from the URL because error boundaries receive no route params. The error object itself is never shown (no internals).
import Link from "next/link";
import { usePathname } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { shopCopy } from "../../lib/shop-copy";

export default function ErrorState({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const first = (usePathname() ?? "").split("/")[1] ?? "";
  const locale = isLocale(first) ? first : "zh-TW";
  const copy = shopCopy[locale];
  return (
    <main className="sf-wrap">
      <div className="sf-empty sf-empty--page" role="alert" data-testid="page-error">
        <h1 className="sf-empty__title">{copy.errorTitle}</h1>
        <p className="sf-muted">{copy.errorBody}</p>
        <div className="sf-notice__actions">
          <button type="button" className="sf-btn sf-btn--primary" onClick={reset}>
            {copy.retry}
          </button>
          <Link className="sf-btn sf-btn--ghost" href={`/${locale}`}>
            {copy.backHome}
          </Link>
        </div>
      </div>
    </main>
  );
}
