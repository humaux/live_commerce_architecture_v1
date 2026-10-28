// Purpose: Names the selected orders and exposes the existing shipping tools and clear action.
// Depends on: @/lib/orders-copy; caller owns scoped selection and the existing PickList workflow.
// Used by: apps/admin/components/MerchantOrders.tsx.
// Invariants: Order numbers come from server summaries; no print, label or shipment command is created here.
import type { OrdersCopy } from "@/lib/orders-copy";

/** Presents the mounted selection without inferring shipment or payment facts. */
export function OrderSelectionBar({ numbers, c, onView, onClear }: {
  numbers: string[]; c: OrdersCopy; onView: () => void; onClear: () => void;
}) {
  return <aside className="orders-selection-bar" data-testid="orders-selection-bar" aria-label={c.selectionTitle}>
    <strong>{c.selectionSelected} <span data-testid="orders-selection-count">{numbers.length}</span> {c.selectionUnit}</strong>
    <ul className="orders-selection-identities">{numbers.map(number => <li key={number}>{number}</li>)}</ul>
    <div className="orders-form-actions">
      <button type="button" className="primary" data-testid="orders-selection-view" onClick={onView}>{c.selectionView}</button>
      <button type="button" data-testid="orders-selection-cancel" onClick={onClear}>{c.cancel}</button>
    </div>
  </aside>;
}
