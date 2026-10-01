// Admin new-order-mail model (contracts/storefront-v2.md §E6): the strict parser of Go GET|PUT /v1/admin/stores/{store_id}/notification-settings
// (internal/httpapi/notify.go, BFF `/api/stores/{store}/notification-settings`) and the one-key request body.
// It never decides who gets mail or when (the SQL claim does); a parser only refuses a malformed read.

export type NotifySettings = { merchant_new_order_email: boolean };

export function parseNotifySettings(value: unknown): NotifySettings {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("unavailable");
  const v = value as Record<string, unknown>;
  if (Object.keys(v).join(",") !== "merchant_new_order_email" || typeof v.merchant_new_order_email !== "boolean")
    throw new Error("unavailable");
  return { merchant_new_order_email: v.merchant_new_order_email };
}

// The frozen PUT body: exactly one key. The write is an idempotent set, so a retry after an unknown outcome is harmless.
export const notifySettingsBody = (enabled: boolean): string => JSON.stringify({ merchant_new_order_email: enabled });
