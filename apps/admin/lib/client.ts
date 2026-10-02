import { currencySign, money } from "../../../packages/format/src/index.ts";
export { currencySign, money } from "../../../packages/format/src/index.ts";
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

// The ONE money display of the admin. `minor` is the wire amount (TWD is x100 in this system: 6000 = NT$60). A whole amount reads
// "NT$60", never "NT$60.00" or "$60.00"; real cents stay visible ("NT$0.50", "US$12.50") because rounding money is never display-only.
// TWD always carries the "NT$" sign: Intl prints a bare "$" for zh-TW, which a merchant reads as US dollars (REPORT-admin-vqa D02).
// Same rule as apps/storefront/lib/money.ts (the buyer sees the same text for the same amount).

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
