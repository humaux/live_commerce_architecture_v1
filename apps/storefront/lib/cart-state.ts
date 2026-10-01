// Pure cart helpers for the shell (header count, drawer, cart page). No I/O, no React: the wire work (session, journal,
// CAS write) is lib/purchase.ts writePurchase; this only derives display numbers and maps a failure to a buyer message.
// It never prices anything (Go quotes), so a subtotal here is "items x listed price" for orientation only.
import { BuyerClientError } from "./buyer-client.ts";
import type { Cart } from "./purchase.ts";

export function cartCount(cart: Cart | null): number {
  return cart ? cart.items.reduce((n, item) => n + item.quantity, 0) : 0;
}

export type CartProblem = "failed" | "uncertain" | "conflict" | "storage" | "session" | "order";

// Maps a client error from the buyer transport to a buyer-facing class. "uncertain" is the journal-held case: the last
// write may or may not have landed, so the UI must offer Retry (same key) and never silently issue a second write.
export function cartProblem(reason: unknown): CartProblem {
  if (reason instanceof BuyerClientError) {
    if (reason.code === "uncertain") return "uncertain";
    if (reason.code === "unavailable") return "storage";
    if (reason.code === "context_changed" || reason.code === "requires_reset" || reason.status === 401) return "session";
    if (reason.status === 409) return "conflict";
  }
  return "failed";
}
