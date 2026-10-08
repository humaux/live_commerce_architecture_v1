// Purpose: closed, private projections of W6-02B report JSON and audited CSV response headers.
// Depends on: reports-model DTO decoders and native Response/Headers.
// Used by: reports BFF leaf, reports-client downloads and focused Node tests.
// Invariants: I05 per-currency/environment integer money; I11/I15 no diagnostics or public cache.
import { parseProductReport, parseChannelReport, parseFunnelReport, parseManualReport } from "./reports-model.ts";
import type { ReportName } from "./reports-request.ts";

/** Accept private no-store responses only; a conflicting public directive is refused. */
export function privateReportHeaders(headers: Headers): boolean {
  const parts = headers.get("cache-control")?.toLowerCase().split(",").map((s) => s.trim()) ?? [];
  return parts.includes("private") && parts.includes("no-store") && !parts.includes("public");
}

/** Validate the exact Go attachment filename, media type and cache policy; no side effect. */
export function reportCSVFilename(headers: Headers, name: ReportName, from: string, to: string): string {
  const filename = `report-${name.replaceAll("-", "_")}-${from}-${to}.csv`;
  if (!privateReportHeaders(headers) || headers.get("content-type")?.toLowerCase() !== "text/csv; charset=utf-8" ||
    headers.get("content-disposition") !== `attachment; filename="${filename}"`) throw new Error("unavailable");
  return filename;
}

/** Decode and project the requested JSON DTO with exact date/session echoes; no business writes. */
export async function reportJSON(response: Response, name: ReportName, from: string, to: string, session = "") {
  if (response.status !== 200 || !privateReportHeaders(response.headers) ||
    response.headers.get("content-type")?.split(";", 1)[0] !== "application/json") throw new Error("unavailable");
  const value: unknown = await response.json();
  switch (name) {
    case "products": return parseProductReport(value, from, to);
    case "channels": return parseChannelReport(value, from, to);
    case "funnel": return parseFunnelReport(value, from, to, session);
    case "manual-orders": return parseManualReport(value, from, to);
  }
}
