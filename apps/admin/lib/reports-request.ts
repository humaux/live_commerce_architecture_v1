// Purpose: BFF request grammar for the eight W6-02B report resources (contracts/reporting-v2.md):
//   GET `/api/stores/{store}/reports/{products|channels|funnel|manual-orders}[.csv]` -> Go internal/httpapi/reports.go.
//   Mirrors parseReportQuery: from/to exactly once each as calendar days 0..91 apart, session_id only on the funnel,
//   raw query at most 512 bytes, and reads carry no body, no key and no transfer-encoding. Go re-validates everything;
//   this is a fence, not the authority.
// Depends on: Native JavaScript/static data; no imported runtime modules.
// Used by: apps/admin/app/api/stores/[store]/[...resource]/route.ts, tests/admin/reports-bff.test.ts
export const reportNames = ["products", "channels", "funnel", "manual-orders"] as const;
export type ReportName = (typeof reportNames)[number];
export type ReportsRouteKind = ReportName | `${ReportName}-csv`;

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const routes: [RegExp, ReportsRouteKind][] = reportNames.flatMap((name) => [
  [new RegExp(`^reports/${name}$`), name],
  [new RegExp(`^reports/${name}\\.csv$`), `${name}-csv` as ReportsRouteKind],
]) as [RegExp, ReportsRouteKind][];

export function reportsRoute(method: string, path: string): ReportsRouteKind | null {
  if (method !== "GET") return null;
  return routes.find(([re]) => re.test(path))?.[1] ?? null;
}

const day = /^(\d{4})-(\d{2})-(\d{2})$/;
// Calendar-valid YYYY-MM-DD as UTC days since the epoch, or null (same rule as the finance grammar and Go ParseRange).
function dayNumber(value: string): number | null {
  const match = day.exec(value);
  if (!match) return null;
  const parsed = Date.UTC(+match[1], +match[2] - 1, +match[3]);
  const back = new Date(parsed);
  return back.getUTCFullYear() === +match[1] && back.getUTCMonth() === +match[2] - 1 && back.getUTCDate() === +match[3]
    ? parsed / 86_400_000
    : null;
}

export function validReportsQuery(kind: ReportsRouteKind, rawURL: string): boolean {
  const at = rawURL.indexOf("?");
  if (at < 0) return false; // every report read requires from/to
  const query = rawURL.slice(at + 1);
  if (query === "" || new TextEncoder().encode(query).length > 512) return false;
  const seen = new Map<string, string>();
  for (const segment of query.split("&")) {
    const match = /^(from|to|session_id)=([^=#+]*)$/.exec(segment);
    if (!match || seen.has(match[1])) return false;
    seen.set(match[1], match[2]);
  }
  if (!seen.has("from") || !seen.has("to")) return false;
  const session = seen.get("session_id");
  if (session !== undefined && (!kind.startsWith("funnel") || !uuidPattern.test(session))) return false;
  if (seen.size !== (session === undefined ? 2 : 3)) return false;
  const from = dayNumber(seen.get("from")!);
  const to = dayNumber(seen.get("to")!);
  return from !== null && to !== null && to - from >= 0 && to - from <= 91;
}

export function validReportsRequest(kind: ReportsRouteKind, request: Request): boolean {
  if (request.method !== "GET" || !validReportsQuery(kind, request.url)) return false;
  if (request.headers.has("idempotency-key") || request.headers.has("transfer-encoding")) return false;
  const length = request.headers.get("content-length");
  return request.body === null && (length === null || length === "0");
}
