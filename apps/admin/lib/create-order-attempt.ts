// Purpose: freeze one order request and retain only an opaque unresolved flag across privacy boundaries.
// Depends on: caller session scope, key generator and browser sessionStorage port.
// Used by: ManualOrder, CreateOrderDrawer and Node acceptance; buyer fields and request keys never enter storage.
type StoragePort = Pick<Storage, "getItem" | "setItem" | "removeItem">;
/** A retry always uses the first serialized body, even if the source draft is later mutated. */
export class OrderAttempt {
  private value: Readonly<{ key: string; body: string }> | null = null;
  private definitive = false;
  constructor(private key: () => string = () => crypto.randomUUID()) {}
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
/** A coarse store/session flag prevents a fresh key after private request memory was cleared. */
export class OrderGuard {
  private key: string;
  constructor(
    private storage: StoragePort,
    store: string,
    boundary: string,
  ) {
    this.key = `buyer-order-unresolved:${store}:${boundary}`;
  }
  blocked() {
    try {
      return this.storage.getItem(this.key) !== null;
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
}

/** A16 reauthorizes messaging after creating the order; auth refusal does not prove no order exists. */
export function retainOrderAttempt(
  outcome: { status: number; uncertain: boolean; code: string },
  previouslyUncertain: boolean,
): boolean {
  return (
    previouslyUncertain ||
    outcome.uncertain ||
    [401, 403].includes(outcome.status) ||
    outcome.code === "idempotency_conflict"
  );
}
