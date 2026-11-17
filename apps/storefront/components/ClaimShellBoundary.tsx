"use client";

// Purpose: keep the shop chrome on desktop claims and every non-claim route, with a compact mobile claim shell.
// Depends on: Next AppRouter selected child segment, ReactNode slots from the server locale layout and scoped CSS.
// Used by: app/[locale]/layout; ClaimLink supplies its own mobile result header and action dock.
// Invariants: no URL/token parsing, session changes or buyer requests; the preview warning and cart drawer stay available.
import type { ReactNode } from "react";
import { useSelectedLayoutSegment } from "next/navigation";
import styles from "./ClaimShellBoundary.module.css";

/** Adapt only the owned chrome around the direct claim segment, without navigation or data effects. */
export default function ClaimShellBoundary({ warning, header, footer, children }: {
  warning: ReactNode;
  header: ReactNode;
  footer: ReactNode;
  children: ReactNode;
}) {
  const segment = useSelectedLayoutSegment();
  if (segment !== "claim") return <>{warning}{header}{children}{footer}</>;
  return (
    <div className={styles.claim}>
      {warning}
      {header}
      {children}
      {footer}
    </div>
  );
}
