// Purpose: Root admin error boundary that remains independent of the localized workspace layout.
// Depends on: next/navigation pathname, @live-commerce/i18n isLocale, AdminStateBlock and ./globals.css.
// Used by: Next.js App Router when the root or localized layout itself fails.
// Invariants: owns html/body and global styles; never renders error internals or depends on sidebar/layout availability.
"use client";

import { usePathname } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { AdminStateBlock, adminStateCopy } from "../components/AdminStateBlock";
import "./globals.css";

export default function GlobalError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const first = (usePathname() ?? "").split("/")[1] ?? "";
  const locale = isLocale(first) ? first : "zh-TW";
  const c = adminStateCopy[locale];
  return (
    <html lang={locale}>
      <body>
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
      </body>
    </html>
  );
}
