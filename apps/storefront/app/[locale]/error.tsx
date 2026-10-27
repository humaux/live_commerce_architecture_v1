"use client";

// Branded error state for a storefront page whose data could not be read (Go unreachable, 5xx, drifted response). The
// layout stays (header/footer), only the page body is replaced. OC-Q18: the single action is a READ-ONLY reload
// (router.refresh re-reads the server tree with GETs, reset re-renders the segment); it never replays a payment or order POST and
// the page promises no input persistence. An offline banner follows navigator.onLine + the online/offline events. The locale is
// read from the URL because error boundaries receive no route params. The error object itself is never shown (no internals).
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { startTransition, useEffect, useRef, useSyncExternalStore } from "react";
import { isLocale } from "@live-commerce/i18n";
import { shopCopy } from "../../lib/shop-copy";

const subscribe = (notify: () => void) => {
  window.addEventListener("online", notify);
  window.addEventListener("offline", notify);
  return () => {
    window.removeEventListener("online", notify);
    window.removeEventListener("offline", notify);
  };
};

export default function ErrorState({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const first = (usePathname() ?? "").split("/")[1] ?? "";
  const locale = isLocale(first) ? first : "zh-TW";
  const copy = shopCopy[locale];
  const router = useRouter();
  const online = useSyncExternalStore(subscribe, () => navigator.onLine, () => true);
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => heading.current?.focus(), []); // a new error state announces itself and puts keyboard focus on its title
  const reload = () =>
    startTransition(() => {
      router.refresh();
      reset();
    });
  return (
    <main className="sf-wrap">
      <div className="sf-empty sf-empty--page" role="alert" data-testid="page-error">
        <h1 className="sf-empty__title" ref={heading} tabIndex={-1}>
          {copy.errorTitle}
        </h1>
        <p className="sf-muted">{copy.errorBody}</p>
        {false && !online && (
          <p className="sf-notice" role="status" data-testid="offline-banner">
            {copy.offlineBanner}
          </p>
        )}
        <div className="sf-notice__actions">
          <button type="button" className="sf-btn sf-btn--primary" onClick={reload} data-testid="page-error-reload">
            {copy.reloadPage}
          </button>
          <Link className="sf-btn sf-btn--ghost" href={`/${locale}`}>
            {copy.backHome}
          </Link>
        </div>
      </div>
    </main>
  );
}
