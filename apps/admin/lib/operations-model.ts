// Purpose: parse the closed W6-05B operation ledger projection and safe business links.
// Depends on: native JavaScript; contracts/external-operation-v1 Amendment W6-05B.
// Used by: OperationsLedger, operations-client, authenticated admin BFF and Node gates.
// Invariants: I01/I06/I11; no provider body, payload or buyer data enters this model.
export const operationID =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
export const operationStates = [
  "READY",
  "DISPATCHING",
  "UNKNOWN",
  "ACKNOWLEDGED",
  "SUCCEEDED",
  "FAILED_FINAL",
  "BLOCKED_POLICY",
  "STALE_BINDING",
  "CANCELLED",
] as const;
export const operationFilters = [
  "attention",
  "FAILED",
  "UNKNOWN",
  "ACKNOWLEDGED",
  "BLOCKED_POLICY",
  "STALE_BINDING",
  "READY",
] as const;
export type OperationState = (typeof operationStates)[number];
export type OperationFilter = (typeof operationFilters)[number];
export type OperationAction = "query" | "cancel" | "retry";
export type OperationCapability = { available: boolean; reason: string };
export type OperationObject = { kind: string; id: string };
export type OperationItem = {
  operation_id: string;
  provider: string;
  action: string;
  purpose: string;
  state: OperationState;
  reason_code: string;
  attempts: number;
  created_at: string;
  updated_at: string;
  object: OperationObject | null;
  actions: Record<OperationAction, OperationCapability>;
};
export type OperationEvent = {
  generation: number;
  state: OperationState;
  reason_code: string;
  created_at: string;
};
export type OperationDetail = OperationItem & { events: OperationEvent[] };
const itemKeys = [
  "operation_id",
  "provider",
  "action",
  "purpose",
  "state",
  "reason_code",
  "attempts",
  "created_at",
  "updated_at",
  "object",
  "actions",
];
const objectKinds = [
  "order",
  "conversation",
  "claim_bundle",
  "ad_draft",
  "payment_attempt",
  "binding",
  "live_session",
];
function exact(v: unknown, keys: string[]): Record<string, unknown> {
  if (
    !v ||
    typeof v !== "object" ||
    Array.isArray(v) ||
    Object.keys(v).length !== keys.length ||
    keys.some((k) => !Object.hasOwn(v, k))
  )
    throw new Error("projection");
  return v as Record<string, unknown>;
}
function text(v: unknown, pattern: RegExp): string {
  if (typeof v !== "string" || !pattern.test(v)) throw new Error("text");
  return v;
}
function count(v: unknown): number {
  if (!Number.isSafeInteger(v) || (v as number) < 0) throw new Error("count");
  return v as number;
}
function state(v: unknown): OperationState {
  if (!operationStates.includes(v as OperationState)) throw new Error("state");
  return v as OperationState;
}
function instant(v: unknown): string {
  const s = text(
    v,
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/,
  );
  if (!Number.isFinite(Date.parse(s))) throw new Error("instant");
  return s;
}
function capability(v: unknown): OperationCapability {
  const o = exact(v, ["available", "reason"]);
  if (typeof o.available !== "boolean") throw new Error("available");
  const reason = text(o.reason, /^(?:[a-z][a-z0-9_]{0,63})?$/);
  if (o.available !== (reason === "")) throw new Error("capability");
  return { available: o.available, reason };
}
function item(o: Record<string, unknown>): OperationItem {
  let object: OperationObject | null = null;
  if (o.object !== null) {
    const r = exact(o.object, ["kind", "id"]);
    if (!objectKinds.includes(r.kind as string)) throw new Error("kind");
    object = { kind: r.kind as string, id: text(r.id, operationID) };
  }
  const actions = exact(o.actions, ["query", "cancel", "retry"]);
  return {
    operation_id: text(o.operation_id, operationID),
    provider: text(o.provider, /^[a-z][a-z0-9_]{0,63}$/),
    action: text(o.action, /^[a-z][a-z0-9_.]{0,127}$/),
    purpose: text(o.purpose, /^[a-z][a-z0-9_]{0,63}$/),
    state: state(o.state),
    reason_code: text(o.reason_code, /^(?:[a-z][a-z0-9_]{0,63})?$/),
    attempts: count(o.attempts),
    created_at: instant(o.created_at),
    updated_at: instant(o.updated_at),
    object,
    actions: {
      query: capability(actions.query),
      cancel: capability(actions.cancel),
      retry: capability(actions.retry),
    },
  };
}
/** Parse a ledger page; unknown or duplicate rows fail closed without retaining raw input. */
export function parseOperationList(v: unknown): {
  items: OperationItem[];
  next_cursor: string;
} {
  const o = exact(v, ["items", "next_cursor"]);
  if (!Array.isArray(o.items) || o.items.length > 100) throw new Error("items");
  const items = o.items.map((v) => item(exact(v, itemKeys)));
  if (new Set(items.map((v) => v.operation_id)).size !== items.length)
    throw new Error("duplicate");
  return { items, next_cursor: text(o.next_cursor, /^[A-Za-z0-9_-]{0,4096}$/) };
}
/** Parse operation detail and at most 50 closed event projections. */
export function parseOperationDetail(v: unknown): OperationDetail {
  const o = exact(v, [...itemKeys, "events"]);
  if (!Array.isArray(o.events) || o.events.length > 50)
    throw new Error("events");
  return {
    ...item(o),
    events: o.events.map((v) => {
      const r = exact(v, ["generation", "state", "reason_code", "created_at"]);
      return {
        generation: count(r.generation),
        state: state(r.state),
        reason_code: text(r.reason_code, /^(?:[a-z][a-z0-9_]{0,63})?$/),
        created_at: instant(r.created_at),
      };
    }),
  };
}
/** Parse command acknowledgement; callers still re-read persisted state. */
export function parseOperationResult(v: unknown) {
  const o = exact(v, ["operation_id", "state", "attempts"]);
  return {
    operation_id: text(o.operation_id, operationID),
    state: state(o.state),
    attempts: count(o.attempts),
  };
}
/** Link only to existing routes that support this exact business object's internal ID. */
export function operationObjectHref(
  locale: string,
  store: string,
  object: OperationObject | null,
): string | null {
  if (
    !object ||
    !operationID.test(store) ||
    !operationID.test(object.id) ||
    !["zh-TW", "zh-CN", "en"].includes(locale)
  )
    return null;
  const base = `/${locale}`;
  const query = `store=${store}`;
  if (object.kind === "order")
    return `${base}/orders?${query}&order=${object.id}`;
  if (object.kind === "ad_draft")
    return `${base}/ads?${query}&draft=${object.id}`;
  return null;
}

/** Display actions only when both the server's state capability and effective store permission allow them. */
export function canOperate(
  store: { role?: string | null; permissions?: string[] } | null,
): boolean {
  return (
    !!store &&
    (store.role === "owner" ||
      store.permissions?.includes("integration:execute") === true)
  );
}
