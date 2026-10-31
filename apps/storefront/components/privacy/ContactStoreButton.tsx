"use client";

// Purpose: the 「聯絡店鋪 / 聯絡商店 / 申請售後」 controls of the signed buyer frames: bring the store's own contact block (the shop footer) into view.
// Depends on: nothing (reads the document footer). Used by: components/PrivacyCenter.tsx, app/[locale]/data-deletion/page.tsx, components/order-status/*.
// Invariants: no navigation to an invented route; without a footer on the page it opens the store home, where the contact block lives.
export default function ContactStoreButton({ label, locale, primary = false, testId = "contact-store" }: { label: string; locale: string; primary?: boolean; testId?: string }) {
  return (
    <button
      type="button"
      className={primary ? "primary" : undefined}
      data-testid={testId}
      onClick={() => {
        const footer = document.querySelector("footer");
        if (footer) footer.scrollIntoView({ block: "start" });
        else window.location.assign(`/${locale}`);
      }}
    >
      {label}
    </button>
  );
}
