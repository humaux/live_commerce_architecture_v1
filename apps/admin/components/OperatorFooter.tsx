// Purpose: Renders the platform operator attribution sentence.
// Depends on: @/lib/company
// Used by: apps/admin/components/TeamInvite.tsx, apps/admin/components/Entry.tsx, apps/admin/components/WorkspaceFrame.tsx
// Platform operator attribution only; no BFF/Go calls and no merchant-storefront branding.
import { operatorSentence, type PlatformLocale } from "@/lib/company";

/** Renders localized platform operator attribution. */
export function OperatorFooter({ locale }: { locale: PlatformLocale }) {
  return (
    <footer className="operator-footer" data-testid="operator-footer">
      <p>{operatorSentence(locale)}</p>
    </footer>
  );
}
