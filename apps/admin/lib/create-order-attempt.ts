// Purpose: freeze one order request and retain an opaque UX flag; the server is the unresolved-intent authority.
// Depends on: closed A16/manual refusal vocabulary, authenticated stable store/principal scope, key generator and sessionStorage.
// Used by: ManualOrder, CreateOrderDrawer and Node acceptance; buyer fields and request keys never enter storage.
type StoragePort = Pick<Storage, "getItem" | "setItem" | "removeItem">;
import { forBuyerErrors } from "./create-order-model.ts";
/** A retry always uses the first serialized body, even if the source draft is later mutated. */
export class OrderAttempt {
  private value: Readonly<{ key: string; body: string }> | null = null;
  private definitive = false;
  private key: () => string;
  constructor(key: () => string = () => crypto.randomUUID()) {
    this.key = key;
  }
  prepare(body: unknown) {
    return (this.value ??= Object.freeze({
      key: this.key(),
      body: JSON.stringify(body),
    }));
  }
  pending() {
    return this.value;
  }
  /** Record a definitive response without replacing the frozen request. */
  finish() {
    if (this.value) this.definitive = true;
  }
  resolved() {
    return this.definitive;
  }
  /** Only an explicit merchant action after a definitive response can permit a new key. */
  startNew() {
    if (!this.definitive) return false;
    this.clear();
    return true;
  }
  /** Privacy-only forgetting; callers retain their unresolved guard across this boundary. */
  clear() {
    this.value = null;
    this.definitive = false;
  }
}
/** A stable store/principal flag survives re-login; unattributed legacy flags fail closed for the store. */
export class OrderGuard {
  private key: string;
  private storage: StoragePort;
  constructor(
    storage: StoragePort,
    store: string,
    scope: string,
  ) {
    this.storage = storage;
    this.key = /^[a-f0-9]{64}$/.test(scope) ? `buyer-order-unresolved:v2:${store}:${scope}` : "";
    this.legacyPrefix = `buyer-order-unresolved:${store}:`;
  }
  private legacyPrefix: string;
  blocked() {
    try {
      return !this.key || this.storage.getItem(this.key) !== null ||
        Object.keys(this.storage).some(key => key.startsWith(this.legacyPrefix) && this.storage.getItem(key) !== null);
    } catch {
      return true;
    }
  }
  arm() {
    try {
      if (this.blocked()) return false;
      this.storage.setItem(this.key, "1");
      return this.storage.getItem(this.key) === "1";
    } catch {
      return false;
    }
  }
  clear() {
    try {
      this.storage.removeItem(this.key);
      return !this.blocked();
    } catch {
      return false;
    }
  }
  /** Call only after a scoped successful server reconciliation (none, not_created or created). */
  authoritativeRecovered() {
    try {
      this.storage.removeItem(this.key);
      for (const key of Object.keys(this.storage)) {
        if (key.startsWith(this.legacyPrefix)) this.storage.removeItem(key);
      }
      return !this.blocked();
    } catch {
      return false;
    }
  }
}

// Only refusals behind checkout.Begin's durable receipt lookup can settle a lost business response.
// Generic conflicts/not_found can arise after commit; dynamic options/peer/capability checks run before replay lookup.
// Body/content-type parsing also precedes lookup, so even a coded 400/415 must retain a previously UNKNOWN receipt.
const replayRefusals: Readonly<Record<number, readonly string[]>> = {
  409: ["insufficient_inventory", "cod_surcharge_changed"],
  422: ["pay_at_pickup_amount_exceeds", "cash_on_delivery_amount_exceeds", "max_per_order_exceeded"],
  429: ["pay_at_pickup_limit", "cash_on_delivery_limit"],
};
/** Retain ambiguous post-dispatch outcomes; only a replay-safe coded refusal settles an earlier UNKNOWN. */
export function retainOrderAttempt(
  outcome: { status: number; uncertain: boolean; code: string },
  previouslyUncertain: boolean,
): boolean {
  return (
    (previouslyUncertain && !replayRefusals[outcome.status]?.includes(outcome.code)) ||
    !forBuyerErrors[outcome.status]?.includes(outcome.code) ||
    outcome.uncertain ||
    outcome.status >= 500 ||
    [401, 403].includes(outcome.status) ||
    ["idempotency_conflict", "conflict", "not_found", "attempt_closed"].includes(outcome.code)
  );
}
