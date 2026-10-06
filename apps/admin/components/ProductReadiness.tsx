// Purpose: Renders the editor's required/recommended checklist and focuses the field a merchant clicks; no API or draft writes.
// Depends on: lib/product-editor-copy (labels) and the parent's observed active section and focus callback.
// Used by: ProductDocumentForm (side panel on desktop, chips on mobile).
// Shared required/recommended checklist rendering; no API or draft writes.
import type { ProductEditorCopy } from "@/lib/product-editor-copy";
/** Links checklist status to the real field section; current location never changes completeness. */
export function ProductReadiness({
  c,
  items,
  focus,
  activeSection,
}: {
  c: ProductEditorCopy;
  items: { key: string; label: string; ok: boolean }[];
  focus: (id: string) => void;
  activeSection: string;
}) {
  return items.map((item) => (
    <button
      className="pe-readiness"
      key={item.label}
      type="button"
      aria-controls={item.key}
      aria-current={activeSection === item.key ? "location" : undefined}
      onClick={() => focus(item.key)}
    >
      <span className="pe-readiness-state" data-ready={item.ok}>
        {item.ok ? c.ready : c.pending}
      </span>
      <span>{item.label}</span>
    </button>
  ));
}
