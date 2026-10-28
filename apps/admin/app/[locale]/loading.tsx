// Purpose: Localized admin route loading skeleton and accessible status text.
// Depends on: next/navigation pathname, @live-commerce/i18n isLocale and AdminStateBlock; no data access.
// Used by: Next.js App Router while the localized admin segment loads.
// Invariants: skeleton decoration is hidden from assistive technology; no draft persistence is implied.
"use client";

import { usePathname } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { AdminStateBlock, adminStateCopy } from "../../components/AdminStateBlock";

export default function LocaleLoading() {
  const first = (usePathname() ?? "").split("/")[1] ?? "";
  const locale = isLocale(first) ? first : "zh-TW";
  return (
    <main className="admin-state-page">
      <AdminStateBlock variant="loading" title={adminStateCopy[locale].loading} />
    </main>
  );
}
