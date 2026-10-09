// Purpose: frozen import wire types and privacy-safe preview/receipt projections.
// Depends on: canonical UUID from orders-model and native schema checks; migration-import-v1 §§3/4/7.
// Used by: ImportWizard, import-client and exact import BFF leaves.
// Invariants: I01/I11 private data is not echoed, I05 historical orders are only archive records.
import { canonicalUUID } from "./orders-model.ts";

/** Supported migration domains; no other backend state is invented. */
export type ImportKind = "customers" | "orders";
/** Requested mapping is field -> exact header text, never CSV data. */
export type ImportMapping = Record<string, string>;
/** Displayable row verdict; source IDs and uploaded cells are intentionally absent. */
export type SafeImportRow = { row: number; outcome: "created" | "updated" | "failed"; code?: string; consent_ignored?: boolean; warning?: "city_dropped" };
/** A preview is a dry-run count and safe-row projection, not an import receipt. */
export type ImportPreview = { file_sha256: string; headers: string[]; mapping: ImportMapping; rows_total: number; new_rows: number;
  update_rows: number; apply_rows: number; failed_rows: number; erased_rows: number; consent_ignored_rows?: number; city_dropped_rows?: number; rows: SafeImportRow[] };
/** Exactly the five frozen acknowledged commit fields. */
export type ImportReceipt = { batch_id: string; created: number; updated: number; failed: number; replayed: boolean };
export const maxImportBytes = 2 * 1024 * 1024;
export const maxImportRows = 5000;
/** Field order follows backend mapping resolution; none of these are buyer values. */
export const importFields: Record<ImportKind, readonly string[]> = {
  customers: ["external_id", "name", "phone", "email", "consent"],
  orders: ["order_id", "customer_id", "ordered_at", "status", "total", "item_name", "item_qty", "city"],
};
/** Required mapping fields for each kind. */
export const requiredImportFields: Record<ImportKind, readonly string[]> = { customers: ["external_id", "name"], orders: ["order_id", "customer_id", "ordered_at", "status", "total"] };
/** Closed codes emitted by the customer/order row validators and database definers. */
export const importRowCodes = ["required", "invalid_external_id", "name_too_long", "invalid_name", "invalid_phone", "invalid_email", "duplicate_external_id", "invalid_request", "erased",
  "invalid_order_id", "invalid_date", "invalid_amount", "invalid_status", "invalid_item", "invalid_quantity", "inconsistent_order", "customer_not_imported", "order_owner_conflict", "order_limit"] as const;

function object(value: unknown, required: string[], optional: string[] = []): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value) || required.some((k) => !Object.hasOwn(value, k)) ||
    Object.keys(value).some((k) => !required.includes(k) && !optional.includes(k))) throw new Error("invalid_response");
  return value as Record<string, unknown>;
}
const count = (n: unknown): n is number => Number.isSafeInteger(n) && (n as number) >= 0 && (n as number) <= maxImportRows;

/** Validate mapping grammar only; backend is the authority for real column membership. */
export function validImportMapping(value: unknown, kind: ImportKind): value is ImportMapping {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  try {if(new TextEncoder().encode(JSON.stringify(value)).length>2048)return false;}catch{return false;}
  return Object.entries(value).every(([k, v]) => importFields[kind].includes(k) && typeof v === "string" && Array.from(v).length <= 100);
}

/** Parse only privacy-safe client DTOs; source IDs are never accepted in browser data. */
export function parseImportPreview(value: unknown, kind: ImportKind): ImportPreview {
  return preview(value, kind, false);
}
/** Validate backend DTOs and remove every external_id before crossing the browser boundary. */
export function projectImportPreview(value: unknown, kind: ImportKind): ImportPreview {
  return preview(value, kind, true);
}
function preview(value: unknown, kind: ImportKind, raw: boolean): ImportPreview {
  const counter = kind === "customers" ? "consent_ignored_rows" : "city_dropped_rows";
  const v = object(value, ["file_sha256", "headers", "mapping", "rows_total", "new_rows", "update_rows", "apply_rows", "failed_rows", "erased_rows", counter, "rows"]);
  if (typeof v.file_sha256 !== "string" || !/^[a-f0-9]{64}$/.test(v.file_sha256) || !Array.isArray(v.headers) ||
    !v.headers.every((h) => typeof h === "string" && h.trim() === h) || !validImportMapping(v.mapping, kind) ||
    Object.values(v.mapping).some((h) => h !== "" && !(v.headers as string[]).includes(h)) ||
    ![v.rows_total, v.new_rows, v.update_rows, v.apply_rows, v.failed_rows, v.erased_rows, v[counter]].every(count) ||
    v.apply_rows !== (v.new_rows as number) + (v.update_rows as number) || v.rows_total !== (v.apply_rows as number) + (v.failed_rows as number) ||
    !Array.isArray(v.rows) || v.rows.length !== v.rows_total) throw new Error("invalid_response");
  let previous = 0, created = 0, updated = 0, failed = 0, erased = 0, flagged = 0;
  const rows = v.rows.map((value) => {
    const r = object(value, ["row", "outcome", ...(raw ? ["external_id"] : [])], ["code", kind === "customers" ? "consent_ignored" : "warning"]);
    if (!count(r.row) || r.row <= previous || !["created", "updated", "failed"].includes(String(r.outcome)) ||
      (raw && (typeof r.external_id !== "string" || Array.from(r.external_id).length > 64 || (r.outcome === "failed" && r.external_id !== ""))) ||
      (r.code !== undefined && !(importRowCodes as readonly unknown[]).includes(r.code)) || (r.outcome === "failed" && r.code === undefined) ||
      (r.outcome !== "failed" && r.code !== undefined) || (r.consent_ignored !== undefined && typeof r.consent_ignored !== "boolean") ||
      (r.warning !== undefined && (r.warning !== "city_dropped" || r.outcome === "failed"))) throw new Error("invalid_response");
    previous = r.row;
    if (r.outcome === "created") created++; else if (r.outcome === "updated") updated++; else failed++;
    if (r.code === "erased") erased++;
    if (r.consent_ignored === true || r.warning === "city_dropped") flagged++;
    // Deliberately construct a fresh object: successful source IDs are also private and never spread through.
    return { row: r.row, outcome: r.outcome, ...(r.code === undefined ? {} : {code:r.code}),
      ...(r.consent_ignored === undefined ? {} : {consent_ignored:r.consent_ignored}), ...(r.warning === undefined ? {} : {warning:r.warning}) } as SafeImportRow;
  });
  if (created !== v.new_rows || updated !== v.update_rows || failed !== v.failed_rows || erased !== v.erased_rows || flagged !== v[counter]) throw new Error("invalid_response");
  return { file_sha256:v.file_sha256, headers:v.headers as string[], mapping:v.mapping, rows_total:v.rows_total as number, new_rows:created,
    update_rows:updated, apply_rows:created+updated, failed_rows:failed, erased_rows:erased, [counter]:flagged, rows };
}

/** Validate acknowledged five-field receipt, with no data-bearing fields. */
export function parseImportReceipt(value: unknown): ImportReceipt {
  const v = object(value, ["batch_id", "created", "updated", "failed", "replayed"]);
  if (typeof v.batch_id !== "string" || !canonicalUUID.test(v.batch_id) || !count(v.created) || !count(v.updated) || !count(v.failed) ||
    (v.created as number) + (v.updated as number) + (v.failed as number) > maxImportRows || typeof v.replayed !== "boolean") throw new Error("invalid_response");
  return v as ImportReceipt;
}
