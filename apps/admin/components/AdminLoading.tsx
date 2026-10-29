// Purpose: Localized validated admin content loading skeleton and accessible status text.
// Depends on: next/navigation pathname, @live-commerce/i18n isLocale and AdminStateBlock; no data access.
// Used by: Explicit Suspense fallbacks inside validated customer and order page content.
// Invariants: no ancestor route loading boundary; skeleton decoration is hidden from assistive technology; no draft persistence is implied.
"use client";

import { usePathname } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { AdminStateBlock, adminStateCopy } from "./AdminStateBlock";

export default function AdminLoading() {
  const first = (usePathname() ?? "").split("/")[1] ?? "";
  const locale = isLocale(first) ? first : "zh-TW";
  return (
    <main className="admin-state-page">
      <AdminStateBlock variant="loading" title={adminStateCopy[locale].loading} />
    </main>
  );
}
