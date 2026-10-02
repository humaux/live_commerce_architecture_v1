// Pure wire contract of the bank_transfer payment mode for the buyer storefront (contracts/storefront-v2.md §C): validators for the
// JSON of Go GET /v1/buyer/orders/{id}/bank-transfer and PUT .../bank-transfer/proof (internal/buyerhttp/transfer.go) as relayed by the BFF
// /api/buyer/orders/{id}/bank-transfer[/proof], the input mirror for the "I have transferred" form, the optional buyer-email mirror and the
// countdown helper. Owns: shapes, closed state/code lists, input mirrors.
// It never: talks to the network, decides money, or replaces a server rule (Go + SQL stay the authority on every write; these helpers only
// refuse malformed reads and give inline hints). The amount to transfer is always the server order total shown by the view (I05).

export const TRANSFER_STATES = [
  "AWAITING",
  "SUBMITTED",
  "REJECTED",
  "CONFIRMED",
  "EXPIRED",
  "REFUNDED_OFFLINE",
] as const;
export type TransferState = (typeof TRANSFER_STATES)[number];

export type TransferBank = {
  bank_name: string;
  branch: string;
  account_name: string;
  account_number: string;
};
export type TransferProof = {
  last5: string;
  amount_minor: number;
  paid_at: string;
  submitted_at: string;
  count: number;
};
export type TransferView = {
  order_id: string;
  state: TransferState;
  window_hours: number;
  deadline_at: string;
  currency: string;
  amount_minor: number;
  bank: TransferBank | null;
  proof: TransferProof | null;
  reject_reason: string | null;
  confirmed_at: string | null;
  refunded_at: string | null;
};
// Order heading (D06). The order read and the transfer read are two requests: the transfer view is polled every minute and has its own
// refresh button, the order only reloads on "Refresh order". After the shop confirmed the payment (or the window ended) the order read
// can still say AWAITING_TRANSFER, and the page then headed "Waiting for bank transfer" above "The shop confirmed your payment". A settled
// transfer is the fresher fact: the heading follows it until the order read agrees. REFUNDED_OFFLINE only follows a confirmation.
export type CommercialState = "DRAFT" | "AWAITING_PAYMENT" | "AWAITING_TRANSFER" | "CONFIRMED" | "CANCELLED";
export function settledCommercialState(commercial: CommercialState, transfer: TransferState | null): CommercialState {
  if (commercial !== "AWAITING_TRANSFER") return commercial;
  if (transfer === "CONFIRMED" || transfer === "REFUNDED_OFFLINE") return "CONFIRMED";
  return transfer === "EXPIRED" ? "CANCELLED" : commercial;
}
export type ProofBody = { last5: string; amount_minor: number; paid_at: string };
export type ProofResult = { order_id: string; state: "SUBMITTED"; proof_count: number; submitted_at: string };

// Coded refusals of the bank-transfer definers (internal/httperror table); the UI maps them to text.
export const TRANSFER_ERROR_CODES = [
  "bank_transfer_unavailable",
  "not_bank_transfer",
  "transfer_not_open",
  "transfer_window_closed",
  "invalid_proof",
] as const;
export type TransferErrorCode = (typeof TRANSFER_ERROR_CODES)[number];
export const isTransferErrorCode = (v: unknown): v is TransferErrorCode =>
  typeof v === "string" && (TRANSFER_ERROR_CODES as readonly string[]).includes(v);

const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const MAX_MINOR = 1_000_000_000_000;
const isRecord = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
const exactKeys = (v: Record<string, unknown>, keys: string[]) => Object.keys(v).sort().join() === [...keys].sort().join();
const isTime = (v: unknown): v is string => typeof v === "string" && Number.isFinite(Date.parse(v));
const isText = (v: unknown, max: number): v is string => typeof v === "string" && [...v].length <= max;
const isMinor = (v: unknown, min = 0): v is number => Number.isSafeInteger(v) && (v as number) >= min && (v as number) <= MAX_MINOR;

const validBank = (v: unknown): v is TransferBank =>
  isRecord(v) &&
  exactKeys(v, ["bank_name", "branch", "account_name", "account_number"]) &&
  isText(v.bank_name, 60) &&
  isText(v.branch, 60) &&
  isText(v.account_name, 60) &&
  isText(v.account_number, 32);
const validProof = (v: unknown): v is TransferProof =>
  isRecord(v) &&
  exactKeys(v, ["last5", "amount_minor", "paid_at", "submitted_at", "count"]) &&
  typeof v.last5 === "string" &&
  /^[0-9]{5}$/.test(v.last5) &&
  isMinor(v.amount_minor, 1) &&
  isTime(v.paid_at) &&
  isTime(v.submitted_at) &&
  Number.isSafeInteger(v.count) &&
  (v.count as number) >= 1;

// The same closed rules as Go validTransferView: bank details exactly while the order is not EXPIRED; a reject reason exactly in REJECTED;
// confirmed_at in CONFIRMED/REFUNDED_OFFLINE; refunded_at only in REFUNDED_OFFLINE.
export function validTransferView(v: unknown, orderID: string): v is TransferView {
  if (
    !isRecord(v) ||
    !exactKeys(v, ["order_id", "state", "window_hours", "deadline_at", "currency", "amount_minor", "bank", "proof", "reject_reason", "confirmed_at", "refunded_at"]) ||
    v.order_id !== orderID ||
    !UUID.test(orderID) ||
    !(TRANSFER_STATES as readonly string[]).includes(String(v.state)) ||
    !Number.isSafeInteger(v.window_hours) ||
    (v.window_hours as number) < 6 ||
    (v.window_hours as number) > 168 ||
    !isTime(v.deadline_at) ||
    typeof v.currency !== "string" ||
    !/^[A-Z]{3}$/.test(v.currency) ||
    !isMinor(v.amount_minor)
  )
    return false;
  const state = v.state as TransferState;
  if ((v.bank === null) !== (state === "EXPIRED") || (v.bank !== null && !validBank(v.bank))) return false;
  if (v.proof !== null && !validProof(v.proof)) return false;
  if (state === "SUBMITTED" && (v.proof === null || v.reject_reason !== null)) return false;
  if ((state === "REJECTED") !== (v.reject_reason !== null) || (v.reject_reason !== null && !isText(v.reject_reason, 200))) return false;
  if ((v.confirmed_at !== null && !isTime(v.confirmed_at)) || (v.refunded_at !== null && !isTime(v.refunded_at))) return false;
  return (state === "CONFIRMED" || state === "REFUNDED_OFFLINE") === (v.confirmed_at !== null) && (state === "REFUNDED_OFFLINE") === (v.refunded_at !== null);
}

export function validProofResult(v: unknown, orderID: string): v is ProofResult {
  return (
    isRecord(v) &&
    exactKeys(v, ["order_id", "state", "proof_count", "submitted_at"]) &&
    v.order_id === orderID &&
    v.state === "SUBMITTED" &&
    Number.isSafeInteger(v.proof_count) &&
    (v.proof_count as number) >= 1 &&
    isTime(v.submitted_at)
  );
}

// Input mirror of Go validProof: five digits, a positive amount in minor units, a time. paid_at must be a full ISO instant.
export function validProofBody(v: unknown): v is ProofBody {
  return (
    isRecord(v) &&
    exactKeys(v, ["last5", "amount_minor", "paid_at"]) &&
    typeof v.last5 === "string" &&
    /^[0-9]{5}$/.test(v.last5) &&
    isMinor(v.amount_minor, 1) &&
    typeof v.paid_at === "string" &&
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(v.paid_at) &&
    Number.isFinite(Date.parse(v.paid_at))
  );
}

// The buyer types a major-unit amount ("1500" or "1500.50"); the wire carries minor units (x100 for 2-decimal currencies such as TWD
// here: every store currency of this release is a 2-exponent one). null when the text is not a positive amount with at most 2 decimals.
export function minorFromText(text: string): number | null {
  const match = /^(\d{1,12})(?:\.(\d{1,2}))?$/.exec(text.trim());
  if (!match) return null;
  const minor = Number(match[1]) * 100 + Number((match[2] ?? "").padEnd(2, "0") || "0");
  return Number.isSafeInteger(minor) && minor >= 1 && minor <= MAX_MINOR ? minor : null;
}

// Mirror of Go validBuyerEmail for a NON-empty address (the field is optional: "" means "no email" and is never sent): <= 254 chars, one
// plain address, none of the characters that make a display name, comment, group or quoted part. Go (net/mail) is the authority.
export function validBuyerEmail(v: unknown): v is string {
  if (typeof v !== "string" || v.length < 3 || v.length > 254 || /[\s\u0000-\u001f<>(),;:"\\[\]]/.test(v)) return false;
  const at = v.indexOf("@");
  if (at < 1 || at !== v.lastIndexOf("@") || at === v.length - 1) return false;
  const local = v.slice(0, at);
  return !local.startsWith(".") && !local.endsWith(".") && !local.includes("..") && !v.slice(at + 1).includes("..");
}

// Whole hours and minutes until the deadline (never negative); expired is true at or after it.
export function transferCountdown(deadlineISO: string, nowMs: number): { hours: number; minutes: number; expired: boolean } {
  const left = Date.parse(deadlineISO) - nowMs;
  if (!Number.isFinite(left) || left <= 0) return { hours: 0, minutes: 0, expired: true };
  const minutes = Math.floor(left / 60_000);
  return { hours: Math.floor(minutes / 60), minutes: minutes % 60, expired: false };
}
