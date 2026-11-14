"use client";

// Purpose: the 「聯絡店鋪 / 聯絡商店 / 申請售後」 controls of the signed buyer frames: contact navigates to the locale contact task, after-sales scrolls the store footer.
// Depends on: nothing.
// Used by: components/PrivacyCenter.tsx, app/[locale]/data-deletion/page.tsx, components/order-status/*.
// Invariants: contact intent opens /${locale}/contact; after-sales preserves the footer scroll / home fallback; no testId-driven routing.
export default function ContactStoreButton({
  label,
  locale,
  primary = false,
  testId = "contact-store",
  intent = "contact",
}: {
  label: string;
  locale: string;
  primary?: boolean;
  testId?: string;
  intent?: "contact" | "after-sales";
}) {
  return (
    <button
      type="button"
      className={primary ? "primary" : undefined}
      data-testid={testId}
      onClick={() => {
        if (intent === "contact") {
          window.location.assign(`/${locale}/contact`);
          return;
        }
        const footer = document.querySelector("footer");
        if (footer) footer.scrollIntoView({ block: "start" });
        else window.location.assign(`/${locale}`);
      }}
    >
      {label}
    </button>
  );
}
