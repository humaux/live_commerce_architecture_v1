"use client";

// Purpose: open the dedicated buyer contact task from existing contact controls.
// Depends on: locale and the existing preview URL helper.
// Used by: components/PrivacyCenter.tsx, app/[locale]/data-deletion/page.tsx, components/order-status/*.
// Invariants: only a valid preview token survives navigation; no buyer data is placed in the URL.
import { withPreview } from "../../lib/design";
export default function ContactStoreButton({ label, locale, primary = false, testId = "contact-store" }: { label: string; locale: string; primary?: boolean; testId?: string }) {
  return (
    <button
      type="button"
      className={primary ? "primary" : undefined}
      data-testid={testId}
      onClick={() => {
        const token = new URLSearchParams(window.location.search).get("preview");
        const preview = token && /^[A-Za-z0-9_-]{43}$/.test(token) ? token : null;
        window.location.assign(withPreview(`/${locale}/contact`, preview));
      }}
    >
      {label}
    </button>
  );
}
