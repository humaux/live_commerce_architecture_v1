// Purpose: admin settlements model — strict parsers for the frozen settlement ledger DTOs.
// Depends on: customers-model.ts (object/count/isInstant helpers), Go internal/payments/settlement.
// Used by: card-payments-client.ts, Settlements.tsx, tests/admin/card-payments-model.test.ts.
// BFF `/api/stores/{store}/settlements[/{id}]` -> Go `internal/httpapi` settlement reads (billing:manage;
// W4-S2 platform-settlement). List statements omit `lines`; the detail includes them. Payout fields are
// omitempty and may appear only on paid statements. No settlement-currency amounts or Stripe ids exist here.
import { count, isInstant, object } from "./customers-model.ts";

export const settlementLineKinds = ["CHARGE", "REFUND", "REFUND_FAILURE", "DISPUTE", "DISPUTE_REVERSAL"] as const;
export type SettlementLineKind = (typeof settlementLineKinds)[number];
export type SettlementLine = {
  order_number: string;
  kind: SettlementLineKind;
  store_minor: number;
  fee_store_minor: number;
  txn_date: string;
};
export type SettlementStatement = {
  statement_id: string;
  period_start: string;
  period_end: string;
  currency: string;
  captured_minor: number;
  refunded_minor: number;
  dispute_minor: number;
  stripe_fee_minor: number;
  platform_fee_bps: number;
  platform_fee_minor: number;
  carried_in_minor: number;
  net_payable_minor: number;
  line_count: number;
  closed_at: string;
  paid: boolean;
  payout_ref: string | null;
  payout_minor: number | null;
  paid_at: string | null;
  lines: SettlementLine[] | null;
};

const statementId = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const orderNumber = /^LC-[0-9A-F]{32}$/;
const payoutRef = /^[A-Za-z0-9._:/-]{4,80}$/;
const maxStatements = 52; // contract: the ledger answers at most one year of weekly statements
const maxLines = 10000;
const dayPattern = /^(\d{4})-(\d{2})-(\d{2})$/;
// Calendar-valid YYYY-MM-DD as a UTC day number, or null.
function dayNumber(value: unknown): number | null {
  if (typeof value !== "string") return null;
  const match = dayPattern.exec(value);
  if (!match) return null;
  const parsed = Date.UTC(+match[1], +match[2] - 1, +match[3]);
  const back = new Date(parsed);
  return back.getUTCFullYear() === +match[1] && back.getUTCMonth() === +match[2] - 1 && back.getUTCDate() === +match[3]
    ? parsed / 86_400_000
    : null;
}
function money(value: unknown): value is number {
  return count(value);
}
function signedMoney(value: unknown): value is number {
  return Number.isSafeInteger(value) && Math.abs(value as number) <= 1_000_000_000_000;
}

function parseLine(value: unknown): SettlementLine {
  const v = object(value, ["order_number", "kind", "store_minor", "fee_store_minor", "txn_date"]);
  if (
    typeof v.order_number !== "string" ||
    !orderNumber.test(v.order_number) ||
    !(settlementLineKinds as readonly unknown[]).includes(v.kind) ||
    !signedMoney(v.store_minor) ||
    !money(v.fee_store_minor) ||
    dayNumber(v.txn_date) === null
  )
    throw new Error("unavailable");
  return v as unknown as SettlementLine;
}

const baseKeys = [
  "statement_id", "period_start", "period_end", "currency", "captured_minor", "refunded_minor",
  "dispute_minor", "stripe_fee_minor", "platform_fee_bps", "platform_fee_minor", "carried_in_minor",
  "net_payable_minor", "line_count", "closed_at", "paid",
];
const optionalKeys = ["payout_ref", "payout_minor", "paid_at", "lines"];

export function parseSettlementStatement(value: unknown): SettlementStatement {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("unavailable");
  const raw = value as Record<string, unknown>;
  const present = Object.keys(raw);
  if (present.some((key) => ![...baseKeys, ...optionalKeys].includes(key))) throw new Error("unavailable");
  const v = object(value, [...baseKeys, ...optionalKeys.filter((key) => Object.hasOwn(raw, key))]);
  const start = dayNumber(v.period_start);
  const end = dayNumber(v.period_end);
  if (
    typeof v.statement_id !== "string" ||
    !statementId.test(v.statement_id) ||
    start === null ||
    end === null ||
    start > end ||
    typeof v.currency !== "string" ||
    !/^[A-Z]{3}$/.test(v.currency) ||
    !money(v.captured_minor) ||
    !money(v.refunded_minor) ||
    !money(v.dispute_minor) ||
    !money(v.stripe_fee_minor) ||
    !Number.isSafeInteger(v.platform_fee_bps) ||
    (v.platform_fee_bps as number) < 0 ||
    (v.platform_fee_bps as number) > 10000 ||
    !money(v.platform_fee_minor) ||
    !signedMoney(v.carried_in_minor) ||
    !signedMoney(v.net_payable_minor) ||
    !Number.isSafeInteger(v.line_count) ||
    (v.line_count as number) < 0 ||
    (v.line_count as number) > maxLines ||
    !isInstant(v.closed_at) ||
    typeof v.paid !== "boolean"
  )
    throw new Error("unavailable");
  // Payout fields exist only once the platform marked the statement paid (omitempty in Go).
  if (!v.paid && ("payout_ref" in v || "payout_minor" in v || "paid_at" in v)) throw new Error("unavailable");
  if ("payout_ref" in v && (typeof v.payout_ref !== "string" || !payoutRef.test(v.payout_ref)))
    throw new Error("unavailable");
  if ("payout_minor" in v && !money(v.payout_minor)) throw new Error("unavailable");
  if ("paid_at" in v && !isInstant(v.paid_at)) throw new Error("unavailable");
  let lines: SettlementLine[] | null = null;
  if ("lines" in v) {
    if (!Array.isArray(v.lines) || v.lines.length > maxLines) throw new Error("unavailable");
    lines = v.lines.map(parseLine);
    // The detail carries every line of the closed statement.
    if (lines.length !== v.line_count) throw new Error("unavailable");
  }
  return {
    ...(v as unknown as Omit<SettlementStatement, "payout_ref" | "payout_minor" | "paid_at" | "lines">),
    payout_ref: "payout_ref" in v ? (v.payout_ref as string) : null,
    payout_minor: "payout_minor" in v ? (v.payout_minor as number) : null,
    paid_at: "paid_at" in v ? (v.paid_at as string) : null,
    lines,
  };
}

export function parseSettlementList(value: unknown): SettlementStatement[] {
  const v = object(value, ["statements"]);
  if (!Array.isArray(v.statements) || v.statements.length > maxStatements) throw new Error("unavailable");
  return v.statements.map(parseSettlementStatement);
}

export function parseSettlementDetail(value: unknown): SettlementStatement {
  const v = object(value, ["statement"]);
  return parseSettlementStatement(v.statement);
}
