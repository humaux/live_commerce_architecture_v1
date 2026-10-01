// Admin bank-transfer model (contracts/storefront-v2.md §C): strict parsers for the DTOs of Go internal/httpapi/offline.go (BFF
// `/api/stores/{store}/bank-transfer-settings` and `/api/stores/{store}/orders/{id}/bank-transfer*`) plus the request-body builders that put
// exactly the frozen keys on the wire.
// It never decides eligibility, money or permission: Go/SQL stay the authority for every write (a confirm is a merchant act checked in
// payments.decide_bank_transfer); a parser only refuses a malformed read. The amount shown is the server order total, never a client value.

export const TRANSFER_STATES = ["AWAITING", "SUBMITTED", "REJECTED", "CONFIRMED", "EXPIRED", "REFUNDED_OFFLINE"] as const;
export type TransferState = (typeof TRANSFER_STATES)[number];

function object(value: unknown, keys: string[]): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("unavailable");
  const result = value as Record<string, unknown>;
  if (Object.keys(result).sort().join(",") !== [...keys].sort().join(",")) throw new Error("unavailable");
  return result;
}
const isInt = (v: unknown, min: number, max: number): v is number =>
  typeof v === "number" && Number.isSafeInteger(v) && v >= min && v <= max;
const isText = (v: unknown, max: number): v is string =>
  typeof v === "string" && [...v].length <= max && !/[\p{Cc}\p{Zl}\p{Zp}]/u.test(v);
const isTime = (v: unknown): v is string => typeof v === "string" && Number.isFinite(Date.parse(v));

// ---- settings card (GET/PUT bank-transfer-settings) ----------------------------------------------------------------------
export type TransferSettings = {
  version: number;
  enabled: boolean;
  allow_cvs: boolean;
  bank_name: string;
  branch: string;
  account_name: string;
  account_number: string;
  window_hours: number;
};
const ACCOUNT = /^([0-9][0-9 -]{3,31})?$/;
export function parseTransferSettings(value: unknown): TransferSettings {
  const v = object(value, ["version", "enabled", "allow_cvs", "bank_name", "branch", "account_name", "account_number", "window_hours"]);
  if (
    !isInt(v.version, 0, Number.MAX_SAFE_INTEGER) || typeof v.enabled !== "boolean" || typeof v.allow_cvs !== "boolean" ||
    !isText(v.bank_name, 60) || !isText(v.branch, 60) || !isText(v.account_name, 60) ||
    typeof v.account_number !== "string" || !ACCOUNT.test(v.account_number) || !isInt(v.window_hours, 6, 168) ||
    // Schema CHECK: switching the mode on needs the bank, the account name and the number.
    (v.enabled && (v.bank_name === "" || v.account_name === "" || v.account_number === ""))
  )
    throw new Error("unavailable");
  return v as TransferSettings;
}
export type TransferSettingsInput = { expected_version: number } & Omit<TransferSettings, "version">;
// Text -> body; null when a field is outside the rules (window 6..168 whole hours, digits/space/hyphen account of 4..32).
export function transferSettingsBody(f: {
  expectedVersion: number; enabled: boolean; allowCvs: boolean; bankName: string; branch: string; accountName: string;
  accountNumber: string; windowHours: string;
}): TransferSettingsInput | null {
  const hours = /^[0-9]{1,3}$/.test(f.windowHours.trim()) ? Number(f.windowHours.trim()) : null;
  const bank = f.bankName.trim(), branch = f.branch.trim(), name = f.accountName.trim(), number = f.accountNumber.trim();
  if (
    !isInt(f.expectedVersion, 0, Number.MAX_SAFE_INTEGER - 1) || hours === null || hours < 6 || hours > 168 ||
    !isText(bank, 60) || !isText(branch, 60) || !isText(name, 60) || !ACCOUNT.test(number) ||
    (f.enabled && (bank === "" || name === "" || number === ""))
  )
    return null;
  return {
    expected_version: f.expectedVersion, enabled: f.enabled, allow_cvs: f.allowCvs, bank_name: bank, branch,
    account_name: name, account_number: number, window_hours: hours,
  };
}

// ---- one order's transfer (GET orders/{id}/bank-transfer) ------------------------------------------------------------------
export type TransferDetail = {
  order_id: string;
  state: TransferState;
  window_hours: number;
  deadline_at: string;
  currency: string;
  amount_minor: number;
  bank: { bank_name: string; branch: string; account_name: string; account_number: string };
  proof: { last5: string; amount_minor: number; paid_at: string; submitted_at: string; count: number } | null;
  reject_reason: string | null;
  confirmed_at: string | null;
  refunded_at: string | null;
};
export function parseTransferDetail(value: unknown, requestedID: string): TransferDetail {
  const v = object(value, ["order_id", "state", "window_hours", "deadline_at", "currency", "amount_minor", "bank", "proof", "reject_reason", "confirmed_at", "refunded_at"]);
  const bank = object(v.bank, ["bank_name", "branch", "account_name", "account_number"]);
  if (
    v.order_id !== requestedID || typeof v.state !== "string" || !(TRANSFER_STATES as readonly string[]).includes(v.state) ||
    !isInt(v.window_hours, 6, 168) || !isTime(v.deadline_at) || typeof v.currency !== "string" || !/^[A-Z]{3}$/.test(v.currency) ||
    !isInt(v.amount_minor, 0, 1_000_000_000_000) ||
    !isText(bank.bank_name, 60) || !isText(bank.branch, 60) || !isText(bank.account_name, 60) || !isText(bank.account_number, 32) ||
    !(v.reject_reason === null || isText(v.reject_reason, 200)) ||
    !(v.confirmed_at === null || isTime(v.confirmed_at)) || !(v.refunded_at === null || isTime(v.refunded_at))
  )
    throw new Error("unavailable");
  let proof: TransferDetail["proof"] = null;
  if (v.proof !== null) {
    const p = object(v.proof, ["last5", "amount_minor", "paid_at", "submitted_at", "count"]);
    if (typeof p.last5 !== "string" || !/^[0-9]{5}$/.test(p.last5) || !isInt(p.amount_minor, 1, 1_000_000_000_000) ||
      !isTime(p.paid_at) || !isTime(p.submitted_at) || !isInt(p.count, 1, Number.MAX_SAFE_INTEGER))
      throw new Error("unavailable");
    proof = p as NonNullable<TransferDetail["proof"]>;
  }
  const state = v.state as TransferState;
  if ((state === "SUBMITTED" && (proof === null || v.reject_reason !== null)) || (state === "REJECTED") !== (v.reject_reason !== null) ||
    (state === "CONFIRMED" || state === "REFUNDED_OFFLINE") !== (v.confirmed_at !== null) ||
    (state === "REFUNDED_OFFLINE") !== (v.refunded_at !== null))
    throw new Error("unavailable");
  return { ...(v as Omit<TransferDetail, "proof">), proof } as TransferDetail;
}

// ---- the three decisions -----------------------------------------------------------------------------------------------------
export type TransferAction = "confirm" | "reject" | "refund-offline";
export const confirmBody = () => "{}";
export const refundBody = () => "{}";
// null when the reason is empty or outside 1..200 characters (the SQL CHECK is the twin).
export function rejectBody(reason: string): string | null {
  const text = reason.trim();
  return text === "" || !isText(text, 200) ? null : JSON.stringify({ reason: text });
}
// The merchant may act on these states (hints only: the definer decides). confirm/reject need an open transfer.
export const canConfirm = (s: TransferState) => s === "AWAITING" || s === "SUBMITTED" || s === "REJECTED";
export const canReject = (s: TransferState) => s === "SUBMITTED";
export const canRefundOffline = (s: TransferState) => s === "CONFIRMED";
