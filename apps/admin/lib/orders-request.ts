// BFF request grammar for merchant orders: BFF `/api/stores/{store}/orders*` and `order-actions`
// -> Go `internal/httpapi/{orders,refunds,shipments}.go`. No generic proxying: every resource is listed.
const states = new Set([
  "all",
  "DRAFT",
  "AWAITING_PAYMENT",
  "CONFIRMED",
  "CANCELLED",
  "shipped",
  "unshipped",
]);

// Inspect the raw URL before Next.js drops empty search strings like a bare '?'.
export function validOrdersQuery(rawURL: string, detail: boolean) {
  const at = rawURL.indexOf("?");
  if (at < 0) return true;
  if (detail) return false;
  const seen = new Set<string>();
  for (const segment of rawURL.slice(at + 1).split("&")) {
    const match = /^(limit|cursor|state)=([A-Za-z0-9_-]+)$/.exec(segment);
    if (!match || seen.has(match[1])) return false;
    const [, key, value] = match;
    seen.add(key);
    if (
      (key === "limit" && !/^(?:[1-9]|[1-9][0-9]|100)$/.test(value)) ||
      (key === "cursor" && value.length > 1024) ||
      (key === "state" && !states.has(value))
    )
      return false;
  }
  return true;
}

const uuid = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
// get: bare JSON read (no query/body/key). csv: streamed attachment. refresh: keyless, bodyless POST.
// command: POST/PUT with Idempotency-Key and a JSON body.
export type OrderActionKind = "get" | "csv" | "refresh" | "command";
const actionRoutes: [string, RegExp, OrderActionKind][] = [
  ["GET", new RegExp(`^orders/${uuid}/refunds$`), "get"],
  ["GET", new RegExp(`^orders/${uuid}/shipment/history$`), "get"],
  ["GET", /^order-actions$/, "get"],
  ["GET", /^orders\/unshipped\.csv$/, "csv"],
  ["POST", new RegExp(`^orders/${uuid}/refunds$`), "command"],
  ["POST", new RegExp(`^orders/${uuid}/refunds/${uuid}/refresh$`), "refresh"],
  ["PUT", new RegExp(`^orders/${uuid}/shipment$`), "command"],
];
export function orderActionRoute(method: string, path: string): OrderActionKind | null {
  return actionRoutes.find(([m, re]) => m === method && re.test(path))?.[2] ?? null;
}

// Response headers the Go CSV route must send before the BFF streams it through (manual-fulfilment-v1 §5.1).
export function validCSVHeaders(headers: Headers) {
  return (
    headers.get("content-type")?.toLowerCase() === "text/csv; charset=utf-8" &&
    /^attachment; filename="unshipped-[0-9a-f]{8}-[0-9]{12}\.csv"$/.test(headers.get("content-disposition") ?? "") &&
    headers.get("cache-control")?.toLowerCase() === "no-store, private" &&
    /^(?:true|false)$/.test(headers.get("x-export-truncated") ?? "")
  );
}

// Reads, csv and the keyless refresh carry no key and no payload. Body emptiness comes from the headers:
// Next's Node adapter always hands a non-GET/HEAD request a stream, so `request.body !== null` is true for
// an empty POST too (verified on Next 16.3.5) and must only be tested where the method cannot carry one.
export function validKeylessRequest(kind: OrderActionKind, request: Request) {
  const length = request.headers.get("content-length");
  return (
    (kind === "refresh" || request.body === null) &&
    !request.headers.has("transfer-encoding") &&
    !request.headers.has("idempotency-key") &&
    (length === null || length === "0")
  );
}
