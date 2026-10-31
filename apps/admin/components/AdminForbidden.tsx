// Purpose: Signed neutral admin denial panel shared by all read boundaries.
// Depends on: @live-commerce/i18n adminSignedStateCopy, localizedPath and @live-commerce/ui/ds StatePanel.
// Used by: Customers, MerchantOrders and WorkspaceFrame after server-authorized denial.
// Invariants: display only; no resource identifiers, diagnostics or ACL decisions.
import { adminSignedStateCopy, localizedPath, type Locale } from "@live-commerce/i18n";
import { StatePanel } from "@live-commerce/ui/ds";
import "./admin-state-block.css";

/** Caller descriptions are legacy context; the signed non-leaking panel owns visible denial copy. */
export function AdminForbidden({ locale, description: _description, href, headingLevel, onNavigate }: { locale: Locale; description?: string; href?: string; headingLevel?: 1 | 2; onNavigate?: () => void }) {
  const c = adminSignedStateCopy[locale];
  const target = href ?? localizedPath(locale, "/");
  return <div className="admin-state-block" data-kind="permission">
    <StatePanel variant="forbidden" headingLevel={headingLevel} title={c.title} body={c.body} note={c.note}
      action={{
        label: c.home,
        href: target,
        onClick: onNavigate ? (e) => { e.preventDefault(); onNavigate(); } : undefined,
      }} />
  </div>;
}
