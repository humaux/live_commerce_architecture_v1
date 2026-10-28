// Purpose: Localized admin not-found display with a safe route back to the dashboard.
// Depends on: next/navigation pathname, @live-commerce/i18n isLocale and AdminStateBlock; no resource lookup.
// Used by: Next.js App Router for unmatched or explicitly not-found localized admin routes.
// Invariants: generic copy never reveals resource existence, authorization details or technical status codes.
"use client";

import { usePathname } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { AdminStateBlock, adminStateCopy } from "../../components/AdminStateBlock";

export default function LocaleNotFound() {
  const first = (usePathname() ?? "").split("/")[1] ?? "";
  const locale = isLocale(first) ? first : "zh-TW";
  const c = adminStateCopy[locale];
  return (
    <main className="admin-state-page">
      <AdminStateBlock
        variant="empty"
        title={c.notFoundTitle}
        description={c.notFoundDescription}
        actionHref={`/${locale}`}
        actionLabel={c.home}
      />
    </main>
  );
}
