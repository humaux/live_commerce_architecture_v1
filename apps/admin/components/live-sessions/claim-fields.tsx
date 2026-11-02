// Purpose: Shared claim field semantics and conservative source-form defaults.
// Depends on: shared Field, claim-source DTO and claim request types.
// Used by: StudioClaims preflight and advanced claim tools.
// Invariants: labels exclude select options; private replies default off; saved platform is preserved.
import type { ReactNode } from "react";
import type { Locale } from "@live-commerce/i18n";
import { Field as SharedField } from "@live-commerce/ui";
import type { ClaimSource } from "@/lib/claim-source-model";
import type { ClaimSourceForm } from "@/lib/claims-request";

// A label beside (not around) its control keeps the accessible name exactly the label
// text, so a select's option text never leaks into it.
export function Field({ id, label, hint, children }: { id: string; label: string; hint?: string; children: ReactNode }) {
  return <SharedField id={id} label={label} hint={hint} width="full" className="claims-field">{children}</SharedField>;
}
// The form the saved source (or the defaults of a first bind: collecting, reply off) would produce.
// Private reply defaults to off: it messages real buyers, so the merchant opts in (ruling t).
// Re-saving keeps the saved source's platform (ruling p), so its own id never becomes ambiguous.
export const sourceFormOf = (source: ClaimSource | null, locale: Locale): ClaimSourceForm => source
  ? { input: source.source_object_id, private_reply: source.private_reply, reply_locale: source.reply_locale, active: source.active, platform: source.platform }
  : { input: "", private_reply: false, reply_locale: locale, active: true, platform: "" };
