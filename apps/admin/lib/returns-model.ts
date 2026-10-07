// Purpose: strict parsers and command-body builders for merchant returns (RMA) and merchant order cancel
//   (contracts/returns-v1.md §2 state machine, §6 wire shapes, §7 codes).
// Depends on: nothing (pure module; the timestamp/uuid grammar mirrors lib/orders-model.ts).
// Used by: apps/admin/lib/returns-client.ts, apps/admin/components/OrderReturns.tsx, apps/admin/components/OrderDetailPanel.tsx,
//   apps/admin/components/ReturnsList.tsx, tests/admin/returns-model.test.ts.
// Invariants: returns-v1 §2 (qty_restock + qty_scrap = qty_received <= qty_registered; states), I05/I13 (no client money/state
//   authority — the parsers only refuse malformed reads, the builders only refuse requests Go would 400/422 anyway).

// The §2 machine: REGISTERED -> RECEIVED -> INSPECTED -> CLOSED, or REGISTERED -> CANCELLED.
export const rmaStates = ["REGISTERED", "RECEIVED", "INSPECTED", "CLOSED", "CANCELLED"] as const;
export type RmaState = (typeof rmaStates)[number];

export type RmaLine = {
  warehouse_id: string;
  sku_id: string;
  qty_registered: number;
  qty_received: number | null;
  qty_restock: number | null;
  qty_scrap: number | null;
};
export type Rma = {
  id: string;
  order_id: string;
  state: RmaState;
  version: number;
  reason: string;
  refund_id: string | null;
  created_at: string;
  updated_at: string;
  lines: RmaLine[];
};
export type RefundGap = {
  order_id: string;
  captured_minor: number;
  refunded_minor: number;
  gap_minor: number;
  cancelled_at: string;
  reason: "cancel_refund_failed";
};

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
// returns.rma_json serializes timestamptz through jsonb (ISO-8601 with a numeric offset, 0–6 fraction digits);
// list_cancel_refund_gaps does the same for cancelled_at. Accept exactly that (and a Z spelling), never a bare date.
const instant = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?(?:Z|[+-]\d{2}:\d{2})$/;
const MAX_QTY = 1_000_000_000; // mirrors the order-line quantity ceiling of orders-model.ts
const MAX_MONEY = 1_000_000_000_000;

function object(value: unknown, keys: string[]): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("unavailable");
  const result = value as Record<string, unknown>;
  if (Object.keys(result).sort().join(",") !== [...keys].sort().join(",")) throw new Error("unavailable");
  return result;
}
function date(value: unknown): value is string {
  if (typeof value !== "string" || !instant.test(value)) return false;
  return Number.isFinite(new Date(value).getTime());
}
function qty(value: unknown): value is number {
  return Number.isSafeInteger(value) && (value as number) >= 0 && (value as number) <= MAX_QTY;
}
function money(value: unknown): value is number {
  return Number.isSafeInteger(value) && (value as number) >= 0 && (value as number) <= MAX_MONEY;
}
function text(value: unknown, max: number): value is string {
  return typeof value === "string" && value.trim().length > 0 && Array.from(value).length <= max &&
    !/[\p{C}\p{Zl}\p{Zp}]/u.test(value);
}

function parseRmaLine(value: unknown, state: RmaState): RmaLine {
  const v = object(value, ["warehouse_id", "sku_id", "qty_registered", "qty_received", "qty_restock", "qty_scrap"]);
  if (typeof v.warehouse_id !== "string" || !uuid.test(v.warehouse_id) ||
    typeof v.sku_id !== "string" || !uuid.test(v.sku_id) ||
    !qty(v.qty_registered) || (v.qty_registered as number) < 1)
    throw new Error("unavailable");
  const received = v.qty_received, restock = v.qty_restock, scrap = v.qty_scrap;
  // §2 table CHECKs mirrored: the quantities a state may carry, and restock + scrap = received <= registered.
  if (state === "REGISTERED" || state === "CANCELLED") {
    if (received !== null || restock !== null || scrap !== null) throw new Error("unavailable");
  } else if (state === "RECEIVED") {
    if (!qty(received) || (received as number) > (v.qty_registered as number) || restock !== null || scrap !== null)
      throw new Error("unavailable");
  } else {
    if (!qty(received) || (received as number) > (v.qty_registered as number) ||
      !qty(restock) || !qty(scrap) || (restock as number) + (scrap as number) !== received)
      throw new Error("unavailable");
  }
  return v as unknown as RmaLine;
}

/** Parses one RMA of the returns.rma_json projection (migration 0155), refusing any drift. */
export function parseRma(value: unknown): Rma {
  const v = object(value, ["id", "order_id", "state", "version", "reason", "refund_id", "created_at", "updated_at", "lines"]);
  if (typeof v.id !== "string" || !uuid.test(v.id) || typeof v.order_id !== "string" || !uuid.test(v.order_id) ||
    typeof v.state !== "string" || !rmaStates.includes(v.state as RmaState) ||
    !Number.isSafeInteger(v.version) || (v.version as number) < 1 ||
    !text(v.reason, 240) ||
    !(v.refund_id === null || (typeof v.refund_id === "string" && uuid.test(v.refund_id))) ||
    !date(v.created_at) || !date(v.updated_at) ||
    !Array.isArray(v.lines) || v.lines.length < 1 || v.lines.length > 50)
    throw new Error("unavailable");
  const state = v.state as RmaState;
  // §2: version starts at 1, +1 per transition; a refund link exists only once the RMA is closed.
  const minVersion = state === "REGISTERED" ? 1 : state === "CANCELLED" ? 2 : state === "RECEIVED" ? 2 : state === "INSPECTED" ? 3 : 4;
  if ((v.version as number) < minVersion || (state !== "CLOSED" && v.refund_id !== null)) throw new Error("unavailable");
  if (new Date(v.updated_at as string).getTime() < new Date(v.created_at as string).getTime()) throw new Error("unavailable");
  const lines = (v.lines as unknown[]).map((raw) => parseRmaLine(raw, state));
  if (new Set(lines.map((l) => `${l.warehouse_id}|${l.sku_id}`)).size !== lines.length) throw new Error("unavailable");
  return { ...(v as unknown as Omit<Rma, "lines" | "state">), state, lines };
}

/** Parses the `{items:[RMA]}` answer of GET /orders/{id}/returns and GET /returns (100 newest). */
export function parseRmaList(value: unknown): Rma[] {
  const v = object(value, ["items"]);
  if (!Array.isArray(v.items) || v.items.length > 100) throw new Error("unavailable");
  const items = v.items.map(parseRma);
  if (new Set(items.map((item) => item.id)).size !== items.length) throw new Error("unavailable");
  return items;
}

/** Parses the `{items:[...]}` answer of GET /orders/cancel-refund-gaps (returns-v1 §3). */
export function parseRefundGaps(value: unknown): RefundGap[] {
  const v = object(value, ["items"]);
  if (!Array.isArray(v.items) || v.items.length > 100) throw new Error("unavailable");
  const items = v.items.map((raw): RefundGap => {
    const row = object(raw, ["order_id", "captured_minor", "refunded_minor", "gap_minor", "cancelled_at", "reason"]);
    if (typeof row.order_id !== "string" || !uuid.test(row.order_id) ||
      !money(row.captured_minor) || !money(row.refunded_minor) || !money(row.gap_minor) ||
      (row.gap_minor as number) < 1 || // a listed row must still owe the buyer money
      (row.refunded_minor as number) + (row.gap_minor as number) !== row.captured_minor ||
      !date(row.cancelled_at) || row.reason !== "cancel_refund_failed")
      throw new Error("unavailable");
    return row as unknown as RefundGap;
  });
  if (new Set(items.map((item) => item.order_id)).size !== items.length) throw new Error("unavailable");
  return items;
}

// ---- command bodies (strict JSON; Go rejects unknown/duplicate keys with 400) --------------------------------------

// The register reason is a merchant-facing enum plus an optional free note; the wire carries one text the SQL stores
// verbatim (CHECK 1..240 bytes after btrim). CJK is 3 bytes, so the composed text is capped at 60 chars to stay
// under the byte cap no matter the mix.
export const returnReasons = ["buyer_request", "wrong_item", "damaged", "defective", "other"] as const;
export type ReturnReason = (typeof returnReasons)[number];
const REASON_CHARS = 60;

/** Composes the stored reason text from the enum key and an optional note; null when it cannot fit the server cap. */
export function registerReasonText(reason: ReturnReason, note: string): string | null {
  const trimmed = note.trim().replace(/\s+/g, " ");
  const textValue = trimmed ? `${reason}: ${trimmed}` : reason;
  return Array.from(textValue).length <= REASON_CHARS ? textValue : null;
}

/** Validates and serializes POST /orders/{id}/returns; null = Go would refuse it (422 invalid_quantities/unknown_line). */
export function registerBody(reason: string, lines: { sku_id: string; quantity: number }[]): string | null {
  if (!text(reason, 240) || lines.length < 1 || lines.length > 50) return null;
  if (new Set(lines.map((l) => l.sku_id)).size !== lines.length) return null;
  for (const line of lines)
    if (!uuid.test(line.sku_id) || !qty(line.quantity) || line.quantity < 1) return null;
  return JSON.stringify({
    reason,
    lines: lines.map((l) => ({ sku_id: l.sku_id, quantity: l.quantity })),
  });
}

/** Validates and serializes POST /returns/{id}/receive: every line once, 0 <= qty, at least one unit (422 nothing_received). */
export function receiveBody(version: number, lines: { sku_id: string; qty_received: number }[]): string | null {
  if (!Number.isSafeInteger(version) || version < 1 || lines.length < 1 || lines.length > 50) return null;
  if (new Set(lines.map((l) => l.sku_id)).size !== lines.length) return null;
  if (!lines.some((l) => l.qty_received > 0)) return null;
  for (const line of lines) if (!uuid.test(line.sku_id) || !qty(line.qty_received)) return null;
  return JSON.stringify({
    expected_version: version,
    lines: lines.map((l) => ({ sku_id: l.sku_id, qty_received: l.qty_received })),
  });
}

/** Validates and serializes POST /returns/{id}/inspect: restock + scrap = received per line (422 quantities_mismatch). */
export function inspectBody(
  version: number,
  lines: { sku_id: string; qty_received: number; qty_restock: number; qty_scrap: number }[],
): string | null {
  if (!Number.isSafeInteger(version) || version < 1 || lines.length < 1 || lines.length > 50) return null;
  if (new Set(lines.map((l) => l.sku_id)).size !== lines.length) return null;
  for (const line of lines)
    if (!uuid.test(line.sku_id) || !qty(line.qty_restock) || !qty(line.qty_scrap) ||
      line.qty_restock + line.qty_scrap !== line.qty_received)
      return null;
  return JSON.stringify({
    expected_version: version,
    lines: lines.map((l) => ({ sku_id: l.sku_id, qty_restock: l.qty_restock, qty_scrap: l.qty_scrap })),
  });
}

/** Serializes POST /returns/{id}/close ({expected_version} only — no refund link from this UI; §1 keeps refund separate). */
export function closeBody(version: number): string | null {
  if (!Number.isSafeInteger(version) || version < 1) return null;
  return JSON.stringify({ expected_version: version });
}

/** Serializes POST /returns/{id}/cancel (registered RMAs only; the state gate is the server's). */
export function cancelRmaBody(version: number): string | null {
  return closeBody(version);
}

// §3: the CAS is the commercial state the merchant saw; only these four are accepted (AWAITING_TRANSFER -> 422 not_cancellable).
export const cancellableStates = ["DRAFT", "AWAITING_PAYMENT", "CONFIRMED", "AWAITING_COLLECTION"] as const;
export type CancellableState = (typeof cancellableStates)[number];

/** Trims and caps the merchant cancel reason (same 240-byte server cap, CJK-safe at 60 chars); null when empty/too long. */
export function cancelReasonText(reason: string): string | null {
  const trimmed = reason.trim().replace(/\s+/g, " ");
  if (!trimmed) return null;
  return Array.from(trimmed).length <= REASON_CHARS ? trimmed : null;
}

/** Validates and serializes POST /orders/{id}/cancel ({expected_state, reason}). */
export function cancelOrderBody(expectedState: string, reason: string): string | null {
  if (!cancellableStates.includes(expectedState as CancellableState) || !text(reason, 240)) return null;
  return JSON.stringify({ expected_state: expectedState, reason });
}
