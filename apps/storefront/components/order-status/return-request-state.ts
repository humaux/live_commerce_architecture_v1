// Purpose: typed UI projections over the existing closed return grammar; no transport or business rules.
// Depends on: return-request-contract and existing RmaState type (no runtime merchant dependency).
// Used by: ReturnRequestForm/Detail and source controls.
// Invariants: validate unknown before narrowing; preserve server fields; never calculate eligibility or money.
import { validReturnRequest, validReturnRequestInput, validReturnRequestList } from "../../lib/return-request-contract.ts";
import type { RmaState } from "../../../admin/lib/returns-model";
export type ReturnLine = { sku_id: string; warehouse_id: string; quantity: number };
export type ReturnRequest = {
  id: string; number: string; order_id: string;
  status: "requested" | "approved" | "rejected" | "withdrawn";
  version: number; reason: "damaged"; note: string; rejection_reason: string | null;
  return_id: string | null; return_state: RmaState | null;
  created_at: string; updated_at: string; lines: ReturnLine[];
  timeline: { status: ReturnRequest["status"]; at: string }[];
};
export type ReturnInput = { reason: "damaged"; note?: string; lines: { sku_id: string; warehouse_id?: string | null; quantity: number }[] };
export function requestProjection(value: unknown, orderID: string, requestID?: string): ReturnRequest | null {
  return validReturnRequest(value, requestID, orderID) ? value as ReturnRequest : null;
}
export function requestListProjection(value: unknown, orderID: string): ReturnRequest[] | null {
  if (!validReturnRequestList(value)) return null;
  return (value as { items: ReturnRequest[] }).items.filter(item => item.order_id === orderID);
}
export function inputProjection(value: unknown): ReturnInput | null {
  return validReturnRequestInput(value) ? value as ReturnInput : null;
}
export function canWithdraw(value: ReturnRequest): boolean { return value.status === "requested" && value.version === 1; }
// A still-requested read cannot prove that an in-flight withdrawal failed to commit.
export function resolvesAttempt(kind: "create" | "withdraw", value: ReturnRequest): boolean {
  return kind === "create" || !canWithdraw(value);
}
