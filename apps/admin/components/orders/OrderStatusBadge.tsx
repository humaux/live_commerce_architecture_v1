// Purpose: Renders a server state with adjacent help for review and carrier drop-off.
// Depends on: react, @live-commerce/ui, @live-commerce/ui/ds, @/lib/orders-copy.
// Used by: apps/admin/components/OrderDetailPanel.tsx (badge), MerchantOrders list rows.
// Invariants: Help explains separate facts; it never changes or derives payment/fulfilment state.
"use client";
import { useId, useState } from "react";
import { Badge } from "@live-commerce/ui";
import { Icon } from "@live-commerce/ui/ds";
import type { OrdersCopy } from "@/lib/orders-copy";

/** Keeps the explanation next to the server-projected status, with a keyboard-operable disclosure. */
export function OrderStatusBadge({ state, c }: { state: string; c: OrdersCopy }) {
  const [open, setOpen] = useState(false);
  const id = useId();
  const help = state === "PROVIDER_LABEL_CREATED" ? c.consignHelp : state === "REVIEW_REQUIRED" ? c.reviewHelp : "";
  const label = c.statuses[state as keyof OrdersCopy["statuses"]];
  return <span className="orders-status-with-help">
    <Badge tone={/CAPTURED|FULFILLED|DELIVERED|COLLECTED/.test(state) ? "success" : /FAILED|CANCELLED|EXPIRED/.test(state) ? "danger" : /AWAITING|PENDING|REVIEW/.test(state) ? "warning" : "neutral"}
      className={`orders-badge orders-badge-${state.toLowerCase()}`} data-state={state}>{label}</Badge>
    {help && <>
      <button type="button" className="orders-status-help" aria-label={`${c.statusHelp}: ${label}`} aria-expanded={open} aria-controls={id}
        data-testid={`order-status-help-${state}`} onClick={() => setOpen(value => !value)}>
        <Icon name="info" size={16} />
      </button>
      <span id={id} className="orders-status-explanation" hidden={!open} data-testid={`order-status-explanation-${state}`}>{help}</span>
    </>}
  </span>;
}
