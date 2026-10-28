// Purpose: Localized admin read-error boundary with safe retry and dashboard navigation.
// Depends on: next/navigation pathname, @live-commerce/i18n isLocale and AdminStateBlock; no transport calls.
// Used by: Next.js App Router for errors within the existing [locale] segment.
// Invariants: reset only re-renders this segment; upstream error message, stack and digest are never rendered.
"use client";

import { usePathname } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { AdminStateBlock, adminStateCopy } from "../../components/AdminStateBlock";

export default function LocaleError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const first = (usePathname() ?? "").split("/")[1] ?? "";
  const locale = isLocale(first) ? first : "zh-TW";
  const c = adminStateCopy[locale];
  return (
    <main className="admin-state-page">
      <AdminStateBlock
        variant="error"
        title={c.errorTitle}
        description={c.errorDescription}
        onRetry={reset}
        retryLabel={c.retry}
        actionHref={`/${locale}`}
        actionLabel={c.home}
      />
    </main>
  );
}
