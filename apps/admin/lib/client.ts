import type { APIError } from "./model";

export type PendingCommand = {
  key: string;
  resource: string;
  kind: "adjust" | "product" | "sku";
  body: string;
};
export const unknownError: APIError = {
  code: "retry_later",
  message: "",
  request_id: "",
  retryable: true,
  details: {},
};

function csrfToken() {
  const values = document.cookie
    .split(";")
    .map((part) => part.trim())
    .filter((part) => part.startsWith("__Host-commerce_csrf="))
    .map((part) => part.slice("__Host-commerce_csrf=".length));
  return values.length === 1 && /^[A-Za-z0-9_-]{43}$/.test(values[0])
    ? values[0]
    : "";
}

// A retry sends the exact original key AND bytes. Never generate a fresh command
// after a lost response: the transaction may already have committed.
export async function sendCommand(store: string, command: PendingCommand) {
  try {
    const result = await fetch(`/api/stores/${store}/${command.resource}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": command.key,
        "X-CSRF-Token": csrfToken(),
      },
      body: command.body,
      signal: AbortSignal.timeout(8000),
    });
    const body = await result.json();
    return { status: result.status, body, uncertain: result.status >= 500 };
  } catch {
    return { status: 503, body: unknownError, uncertain: true };
  }
}

export function money(locale: string, currency: string, minor: number) {
  const formatter = new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
  });
  return formatter.format(
    minor / 10 ** formatter.resolvedOptions().maximumFractionDigits!,
  );
}
