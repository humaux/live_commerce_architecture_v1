// Purpose: BFF request grammar for card payments and settlements (fence, not authority).
// Depends on: card-payments-model.ts (descriptor suffix rule); Go internal/httpapi/payment_card.go.
// Used by: app/api/stores/[store]/[...resource]/route.ts, tests/admin/card-payments-request.test.ts.
// BFF `/api/stores/{store}/{payments/card,settlements*}` -> Go `internal/httpapi/payment_card.go`
// (cvsRoute keyed=false: the PUT is CAS-guarded by expected_version, so an Idempotency-Key is a client bug)
// and the settlement reads (contract stripe-platform-account-v1 §3; W4-S2 settlement ledger).
const uuid = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";

export type CardPaymentsRoute = "card-read" | "card-write" | "settlements" | "settlement";

const routes: [string, RegExp, CardPaymentsRoute][] = [
  ["GET", /^payments\/card$/, "card-read"],
  ["PUT", /^payments\/card$/, "card-write"],
  ["GET", /^settlements$/, "settlements"],
  ["GET", new RegExp(`^settlements/${uuid}$`), "settlement"],
];
export function cardPaymentsRoute(method: string, path: string): CardPaymentsRoute | null {
  return routes.find(([m, re]) => m === method && re.test(path))?.[2] ?? null;
}

const dayPattern = /^(\d{4})-(\d{2})-(\d{2})$/;
function calendarDay(value: string): boolean {
  const match = dayPattern.exec(value);
  if (!match) return false;
  const parsed = Date.UTC(+match[1], +match[2] - 1, +match[3]);
  const back = new Date(parsed);
  return back.getUTCFullYear() === +match[1] && back.getUTCMonth() === +match[2] - 1 && back.getUTCDate() === +match[3];
}

// Inspect the raw URL before Next.js drops empty search strings like a bare '?'. Card routes and the
// settlement detail are exact (no query at all); the settlements list takes before/limit, each at most once
// (Go: ?before=YYYY-MM-DD&limit=1..52, RawQuery <= 128 chars — implied by these shapes).
export function validCardPaymentsQuery(kind: CardPaymentsRoute, rawURL: string): boolean {
  const at = rawURL.indexOf("?");
  if (at < 0) return true;
  if (kind !== "settlements") return false;
  const seen = new Map<string, string>();
  for (const segment of rawURL.slice(at + 1).split("&")) {
    const match = /^(before|limit)=([^=&]*)$/.exec(segment);
    if (!match || seen.has(match[1])) return false;
    seen.set(match[1], match[2]);
  }
  const before = seen.get("before");
  if (before !== undefined && !calendarDay(before)) return false;
  const limit = seen.get("limit");
  if (limit !== undefined && !/^(?:[1-9]|[1-4][0-9]|5[0-2])$/.test(limit)) return false;
  return true;
}

// Headers/body presence fence. Every route is keyless (CAS on the PUT); GETs carry no body.
export function validCardPaymentsRequest(kind: CardPaymentsRoute, request: Request): boolean {
  if (!validCardPaymentsQuery(kind, request.url)) return false;
  if (request.headers.has("idempotency-key") || request.headers.has("transfer-encoding")) return false;
  if (kind === "card-write")
    return request.headers.get("content-type")?.split(";", 1)[0].trim().toLowerCase() === "application/json";
  const length = request.headers.get("content-length");
  return request.body === null && (length === null || length === "0");
}

const suffixPattern = /^[A-Za-z0-9][A-Za-z0-9 .-]{1,9}$/;
// Exact frozen PUT body (cvsStrictBody): unknown or missing key = invalid; descriptor_suffix is
// present-but-nullable, and when set must already satisfy the contract §3.1 charset/letter rule.
export function validCardPaymentsBody(text: string): boolean {
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    return false;
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const body = value as Record<string, unknown>;
  if (Object.keys(body).sort().join(",") !== "descriptor_suffix,enabled,expected_version,terms_version") return false;
  if (typeof body.enabled !== "boolean") return false;
  if (
    typeof body.terms_version !== "string" ||
    body.terms_version === "" ||
    Array.from(body.terms_version).length > 64 ||
    /[\p{C}\p{Zl}\p{Zp}]/u.test(body.terms_version)
  )
    return false;
  if (
    body.descriptor_suffix !== null &&
    (typeof body.descriptor_suffix !== "string" ||
      !suffixPattern.test(body.descriptor_suffix) ||
      !/[A-Za-z]/.test(body.descriptor_suffix))
  )
    return false;
  return Number.isSafeInteger(body.expected_version) && (body.expected_version as number) >= 0;
}
