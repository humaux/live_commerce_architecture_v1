import type { APIError } from "./model";

export type PendingCommand = {
  key: string;
  resource: string;
  // catalog-media: edit = PATCH products/{id}; price/archive = POST skus|products/{id}/(price|archive).
  kind: "adjust" | "product" | "sku" | "edit" | "price" | "archive";
  body: string;
};
export const unknownError: APIError = {
  code: "retry_later",
  message: "",
  request_id: "",
  retryable: true,
  details: {},
};

export function csrfToken() {
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
      method: command.kind === "edit" ? "PATCH" : "POST",
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
  if (currency === "TWD") return `NT$${new Intl.NumberFormat(locale, {
    minimumFractionDigits: minor % 100 === 0 ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(minor / 100)}`;
  const formatter = new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
  });
  return formatter.format(
    minor / 10 ** formatter.resolvedOptions().maximumFractionDigits!,
  );
}

const UUID = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
const JOURNAL_RESOURCE: Record<PendingCommand["kind"], RegExp> = {
  adjust: /^inventory\/adjustments$/,
  product: /^products$/,
  sku: /^skus$/,
  edit: new RegExp(`^products/${UUID}$`),
  price: new RegExp(`^skus/${UUID}/price$`),
  archive: new RegExp(`^(?:products|skus)/${UUID}/archive$`),
};
// The localStorage journal is untrusted on reload: a restored command is replayed only when its kind and resource
// form one of the known pairs, so a tampered entry can never aim a write at another path.
export function validJournalCommand(value: unknown): value is PendingCommand {
  if (!value || typeof value !== "object") return false;
  const c = value as Record<string, unknown>;
  return (
    typeof c.key === "string" &&
    typeof c.body === "string" &&
    typeof c.kind === "string" &&
    typeof c.resource === "string" &&
    Object.hasOwn(JOURNAL_RESOURCE, c.kind) &&
    JOURNAL_RESOURCE[c.kind as PendingCommand["kind"]].test(c.resource)
  );
}
