// Purpose: Resolve standalone merchant order details through the same server scope guards as the list.
// Depends on: ../page (orders query, UUID and authenticated-store validation).
// Used by: Next.js /[locale]/orders/[order].
// Invariants: A direct load/reload never relies on list memory or client-provided authority.
export {default} from "../page";
