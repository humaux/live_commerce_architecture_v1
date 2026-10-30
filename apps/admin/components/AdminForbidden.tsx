// Purpose: Signed neutral admin denial panel shared by all read boundaries.
// Depends on: @live-commerce/i18n adminSignedStateCopy and @live-commerce/ui/ds StatePanel.
// Used by: Customers, MerchantOrders and WorkspaceFrame after server-authorized denial.
// Invariants: display only; no resource identifiers, diagnostics or ACL decisions.
import { adminSignedStateCopy, type Locale } from "@live-commerce/i18n";
import { StatePanel } from "@live-commerce/ui/ds";
import "./admin-state-block.css";

/** Caller descriptions are legacy context; the signed non-leaking panel owns visible denial copy. */
export function AdminForbidden({ locale, description: _description, href, headingLevel }: { locale: Locale; description?: string; href?: string; headingLevel?: 1 | 2 }) {
  const c = adminSignedStateCopy[locale];
  return <div className="admin-state-block" data-kind="permission">
    <StatePanel variant="forbidden" headingLevel={headingLevel} title={c.title} body={c.body} note={c.note}
      action={{ label: c.home, href: href ?? `/${locale}` }} />
  </div>;
}
