// Purpose: keep recipient and validated pickup drafts while changing the delivery method.
// Depends on: buyer/cart scope and exact server delivery-option keys supplied by CheckoutFlow.
// Used by: CheckoutFlow while replacing a quote or switching between home and pickup.
// Invariants: drafts stay in memory and are available only for the same buyer, cart and cart version.
import { useState } from "react";
import type { CvsCheckoutDraft } from "../CvsPickup";
import type { OrderFields } from "../../lib/order-fields";

type Draft = { home?: OrderFields; cvs?: CvsCheckoutDraft };
type DeliveryKind = "home" | "cvs";
type Session = { scope: string; methods: Record<string, Draft>; last: Partial<Record<DeliveryKind, string>> };

export function useCheckoutDeliveryDraft(scope: string, method: string) {
  const [session, setSession] = useState<Session | null>(null);
  const current = session?.scope === scope ? session : null;
  const forScope = (old: Session | null): Session => old?.scope === scope ? old : { scope, methods: {}, last: {} };

  function remember(patch: Draft) {
    if (!scope || !method) return;
    setSession((old) => {
      const value = forScope(old);
      const draft = value.methods[method];
      return { ...value, methods: { ...value.methods, [method]: {
        home: patch.home ?? draft?.home,
        cvs: patch.cvs ? { ...draft?.cvs, ...patch.cvs } : draft?.cvs,
      } } };
    });
  }

  function rememberMethod(kind: DeliveryKind) {
    if (!scope || !method) return;
    setSession((old) => {
      const value = forScope(old);
      return { ...value, last: { ...value.last, [kind]: method } };
    });
  }

  return { draft: current?.methods[method], last: current?.last, remember, rememberMethod, clear: () => setSession(null) };
}
