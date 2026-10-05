// Shared required/recommended checklist rendering; no API or draft writes.
import type { ProductEditorCopy } from "@/lib/product-editor-copy";
export function ProductReadiness({
  c,
  items,
  focus,
}: {
  c: ProductEditorCopy;
  items: { key: string; label: string; ok: boolean }[];
  focus: (id: string) => void;
}) {
  return items.map((item) => (
    <button
      className="pe-readiness"
      key={item.label}
      type="button"
      onClick={() => focus(item.key)}
    >
      <span className="pe-readiness-state" data-ready={item.ok}>
        {item.ok ? c.ready : c.pending}
      </span>
      <span>{item.label}</span>
    </button>
  ));
}
