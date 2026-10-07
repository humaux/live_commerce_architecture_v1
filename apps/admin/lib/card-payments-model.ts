// Purpose: admin card-payments model — strict parsers for the frozen platform-Stripe DTOs.
// Depends on: customers-model.ts (object/count/isInstant helpers), Go internal/payments/platformstripe.
// Used by: card-payments-client.ts, CardPayments.tsx, tests/admin/card-payments-model.test.ts.
// BFF `/api/stores/{store}/payments/card` -> Go `internal/httpapi/payment_card.go`
// (contract stripe-platform-account-v1 §3/§5). Unknown key, wrong type or broken invariant = invalid read
// (thrown "unavailable"); the server stays the authority for state, limits and the descriptor rule.
// The summary carries NO account id, key or approval data (integrator ruling) — parsers never admit one.
import { count, object } from "./customers-model.ts";

export const platformStates = ["NONE", "DESIGNATED", "OPEN", "CLOSED", "REVOKED"] as const;
export type PlatformState = (typeof platformStates)[number];
export const storeCardStates = ["NONE", "ENABLED", "DISABLED", "BLOCKED"] as const;
export type StoreCardState = (typeof storeCardStates)[number];

export type CardSummary = {
  platform_state: PlatformState;
  store_state: StoreCardState;
  allowed: boolean;
  terms_version: string | null;
  accepted_terms_version: string | null;
  display_name: string | null;
  descriptor_preview: string | null;
  currency: string | null;
  min_minor: number | null;
  max_minor: number | null;
  version: number;
};
export type CardResult = {
  state: StoreCardState;
  version: number;
  max_minor: number | null;
  currency: string | null;
  descriptor_preview: string | null;
};
// The keyless PUT body (CAS via expected_version; Go refuses Idempotency-Key on this route).
export type CardInput = {
  enabled: boolean;
  terms_version: string;
  descriptor_suffix: string | null;
  expected_version: number;
};

function short(value: unknown, max: number): value is string {
  return typeof value === "string" && value !== "" && Array.from(value).length <= max && !/[\p{C}\p{Zl}\p{Zp}]/u.test(value);
}
function nullableShort(value: unknown, max: number): value is string | null {
  return value === null || short(value, max);
}
function nullableMoney(value: unknown): value is number | null {
  return value === null || count(value);
}
function version(value: unknown): value is number {
  return Number.isSafeInteger(value) && (value as number) >= 0;
}

export function parseCardSummary(value: unknown): CardSummary {
  const v = object(value, [
    "platform_state", "store_state", "allowed", "terms_version", "accepted_terms_version",
    "display_name", "descriptor_preview", "currency", "min_minor", "max_minor", "version",
  ]);
  if (
    !(platformStates as readonly unknown[]).includes(v.platform_state) ||
    !(storeCardStates as readonly unknown[]).includes(v.store_state) ||
    typeof v.allowed !== "boolean" ||
    !nullableShort(v.terms_version, 64) ||
    !nullableShort(v.accepted_terms_version, 64) ||
    !nullableShort(v.display_name, 80) ||
    !nullableShort(v.descriptor_preview, 64) ||
    !(v.currency === null || (typeof v.currency === "string" && /^[A-Z]{3}$/.test(v.currency))) ||
    !nullableMoney(v.min_minor) ||
    !nullableMoney(v.max_minor) ||
    (v.min_minor !== null && v.max_minor !== null && (v.min_minor as number) > (v.max_minor as number)) ||
    !version(v.version)
  )
    throw new Error("unavailable");
  return v as unknown as CardSummary;
}

export function parseCardResult(value: unknown): CardResult {
  const v = object(value, ["state", "version", "max_minor", "currency", "descriptor_preview"]);
  if (
    !(storeCardStates as readonly unknown[]).includes(v.state) ||
    !version(v.version) ||
    !nullableMoney(v.max_minor) ||
    !(v.currency === null || (typeof v.currency === "string" && /^[A-Z]{3}$/.test(v.currency))) ||
    !nullableShort(v.descriptor_preview, 64)
  )
    throw new Error("unavailable");
  return v as unknown as CardResult;
}

// Contract §3.1: 2..10 chars, first alphanumeric, then alphanumerics/spaces/dots/dashes, at least one letter.
export const descriptorSuffixPattern = /^[A-Za-z0-9][A-Za-z0-9 .-]{1,9}$/;
export function validDescriptorSuffix(value: string): boolean {
  return descriptorSuffixPattern.test(value) && /[A-Za-z]/.test(value);
}

export const descriptorMaxLength = 22;
// Live budget for the suffix field: L + 2 + len(suffix) <= 22, with L taken from descriptor_preview
// (which equals the platform base while no suffix is configured). The charset rule independently caps a
// suffix at 10 chars. Null preview = the server has not published a base; it stays the authority (PT422).
export function suffixBudget(preview: string | null): number | null {
  if (preview === null) return null;
  return Math.max(0, Math.min(10, descriptorMaxLength - Array.from(preview).length - 2));
}
// The contract projection the card statement shows: descriptor_display + ('* ' + suffix when set).
export function descriptorPreview(base: string, suffix: string | null): string {
  return suffix === null ? base : `${base}* ${suffix}`;
}
// The platform base inside a preview: neither descriptor_display (Stripe forbids '*') nor the suffix
// (charset [A-Za-z0-9 .-]) can contain "* ", so the first "* " is unambiguous — also when a DISABLED
// store still carries its retained suffix in the preview.
export function descriptorBase(preview: string): string {
  const at = preview.indexOf("* ");
  return at < 0 ? preview : preview.slice(0, at);
}
