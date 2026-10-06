// Purpose: Renders shell slots and manages drawer focus and keyboard behavior without domain commands.
// Depends on: react, ./AppShell.module.css, ./tokens.css, ./Presentation
// Used by: apps/admin/components/AdminPageHeader.tsx, apps/admin/components/Ads.tsx, apps/admin/components/AdsDraft.tsx, apps/admin/components/AdsResults.tsx, apps/admin/components/Attribution.tsx, apps/admin/components/AttributionPanels.tsx, apps/admin/components/Billing.tsx, apps/admin/components/CodSettings.tsx, apps/admin/components/CollectionManager.tsx, apps/admin/components/CustomerDetail.tsx, apps/admin/components/Customers.tsx, apps/admin/components/Dashboard.tsx, apps/admin/components/Design.tsx, apps/admin/components/DesignField.tsx, apps/admin/components/DesignMedia.tsx, apps/admin/components/Finance.tsx, apps/admin/components/Ledger.tsx, apps/admin/components/LedgerTable.tsx, apps/admin/components/ManualOrder.tsx, apps/admin/components/MerchantOrders.tsx, apps/admin/components/MetaConnect.tsx, apps/admin/components/OrderDetailPanel.tsx, apps/admin/components/OrderListFilters.tsx, apps/admin/components/ProductImport.tsx, apps/admin/components/ProductList.tsx, apps/admin/components/ProductPhoto.tsx, apps/admin/components/Promotions.tsx, apps/admin/components/StorefrontSettings.tsx, apps/admin/components/Studio.tsx, apps/admin/components/StudioClaims.tsx, apps/admin/components/Team.tsx, apps/admin/components/WorkspaceFrame.tsx
"use client";
// Pure shell slots; no API, application routing or domain state here.
import { useEffect, useRef, type ReactNode } from "react";
import styles from "./AppShell.module.css";
import "./tokens.css";
export { styles as shellStyles };
export { PageHeader, FormRow, Field, Badge, TabStrip, TableFrame, FilePicker, DateControl, presentationStyles } from "./Presentation";
/** Renders shell slots and manages drawer focus and Escape handling without API calls. */
export function AppShell({
  rail,
  topbar,
  children,
  open,
  close,
  closeLabel,
  skipLabel,
}: {
  rail: ReactNode;
  topbar: ReactNode;
  children: ReactNode;
  open: boolean;
  close: () => void;
  closeLabel: string;
  skipLabel: string;
}) {
  const aside = useRef<HTMLElement>(null);
  const rest = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const before = document.activeElement as HTMLElement | null;
    const originalOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    rest.current?.setAttribute("inert", "");
    const controls = () =>
      Array.from(
        aside.current?.querySelectorAll<HTMLElement>(
          'a[href],button:not(:disabled),select,summary,[tabindex="0"]',
        ) ?? [],
      ).filter((e) => e.getClientRects().length);
    controls()[0]?.focus();
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        close();
      }
      if (event.key === "Tab") {
        const list = controls(),
          first = list[0],
          last = list.at(-1);
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault();
          last?.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault();
          first?.focus();
        }
      }
    };
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("keydown", key);
      rest.current?.removeAttribute("inert");
      document.body.style.overflow = originalOverflow;
      before?.focus();
    };
  }, [open, close]);
  return (
    <div className={styles.shell} data-ui-shell>
      <a className={styles.skip} href="#main">
        {skipLabel}
      </a>
      {open && (
        <button
          tabIndex={-1}
          className={styles.backdrop}
          aria-label={closeLabel}
          onClick={close}
        />
      )}
      <aside
        ref={aside}
        id="workspace-navigation"
        className={`${styles.rail} ${open ? styles.open : ""}`}
        data-shell-rail
      >
        <button
          type="button"
          className={styles.close}
          aria-label={closeLabel}
          onClick={close}
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            aria-hidden="true"
          >
            <path d="m6 6 12 12M18 6 6 18" />
          </svg>
        </button>
        {rail}
      </aside>
      <div className={styles.work} ref={rest}>
        <header className={styles.topbar} data-shell-topbar>
          {topbar}
        </header>
        <main id="main" className={styles.main} tabIndex={-1}>
          {children}
        </main>
      </div>
    </div>
  );
}
