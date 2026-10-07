// Purpose: report reads with authorized staff/session labels and one fenced audited CSV download.
// Depends on: customers-client guarded GETs, settings-client session/CSRF, existing team/studio APIs, reports DTOs.
// Used by: Reports.tsx; no channel secrets or persisted financial data.
// Invariants: I01 scope, I05 backend money only, I06 UNKNOWN export never automatically retried.
import { get, ReadError } from "./customers-client";
import { csrfCookie, sessionBoundary } from "./settings-client";
import { readTeam } from "./team-client";
import { readStudioPage } from "./studio-client";
import { parseProductReport, parseChannelReport, parseFunnelReport, parseManualReport, type ProductReport, type ChannelReport, type FunnelReport, type ManualReport } from "./reports-model";
import { reportCSVFilename } from "./reports-response";
import type { ReportName } from "./reports-request";
import type { Store } from "./model";

/** One report result; auxiliary labels share the guarded read/session lifecycle. */
export type ReportView =
  | { name: "products"; report: ProductReport }
  | { name: "channels"; report: ChannelReport }
  | { name: "funnel"; report: FunnelReport }
  | { name: "manual-orders"; report: ManualReport; staff: Record<string, string>; sessions: Record<string, string> };

function path(store: string, name: ReportName, from: string, to: string, csv = false) {
  return `/api/stores/${store}/reports/${name}${csv ? ".csv" : ""}?from=${from}&to=${to}`;
}

/** Read only the selected report; names use existing authorized APIs, with explicit fallback in the view. */
export async function readReport(store: Store, name: ReportName, from: string, to: string, signal: AbortSignal): Promise<ReportView> {
  const url = path(store.id, name, from, to);
  switch (name) {
    case "products": return { name, report: await get(url, (v) => parseProductReport(v, from, to), signal) };
    case "channels": return { name, report: await get(url, (v) => parseChannelReport(v, from, to), signal) };
    case "funnel": return { name, report: await get(url, (v) => parseFunnelReport(v, from, to, ""), signal) };
    case "manual-orders": {
      const report = await get(url, (v) => parseManualReport(v, from, to), signal);
      const staff: Record<string, string> = {}, sessions: Record<string, string> = {};
      // Names do not justify broadening permissions. Only owners can list staff; live titles require live:read.
      const lookups = await Promise.allSettled([
        store.role === "owner" && report.rows.some((r) => r.principal_id) ? readTeam(store.id, signal) : Promise.resolve(null),
        store.permissions?.includes("live:read") && report.rows.some((r) => r.session_id) ? readStudioPage(store.id, "", signal) : Promise.resolve(null),
      ]);
      for (const result of lookups) if (result.status === "rejected" && result.reason instanceof Error &&
        (result.reason as { code?: string }).code === "signed-out") throw new ReadError("signed-out");
      const team = lookups[0], studio = lookups[1];
      if (team.status === "fulfilled" && team.value) for (const member of team.value.members) {
        if (member.email) staff[member.principal_id] = member.email;
      }
      // ponytail: resolve the existing first 20 session titles; older/missing ones show an honest fallback.
      // A dedicated bulk-name API is an integrator contract change if large historical reports require it.
      if (studio.status === "fulfilled" && studio.value) for (const draft of studio.value.items) sessions[draft.session_id] = draft.title;
      return { name, report, staff, sessions };
    }
  }
}

/** Download outcome distinguishes definite refusal from a possibly committed audit. */
export type ReportDownload = "done" | "signed-out" | "forbidden" | "not-found" | "unavailable" | "uncertain";

/** Perform one CSV GET after a session/CSRF check; a changed session conceals bytes, never retries. */
export async function downloadReport(store: string, name: ReportName, from: string, to: string, boundary: string, signal: AbortSignal): Promise<ReportDownload> {
  const csrf = csrfCookie();
  const concealed = () => signal.aborted || document.visibilityState === "hidden";
  try {
    if (!csrf || !boundary || await sessionBoundary(csrf) !== boundary || csrfCookie() !== csrf) return "signed-out";
  } catch { return "signed-out"; }
  if (concealed()) return "unavailable";
  let response: Response;
  try {
    // Calls reporting-v2 §6; the exact GET is keyless but writes a server audit. No retries are safe by assumption.
    response = await fetch(path(store, name, from, to, true), {
      method: "GET", credentials: "same-origin", cache: "no-store", headers: { "X-CSRF-Token": csrf },
      // End-to-end includes the existing store authorization lookup before the 65-second report fetch.
      signal: AbortSignal.any([signal, AbortSignal.timeout(75000)]),
    });
  } catch { return "uncertain"; }
  if (response.status === 401) return "signed-out";
  if (response.status === 403) return "forbidden";
  if (response.status === 404) return "not-found";
  if (!response.ok) return response.status >= 500 ? "uncertain" : "unavailable";
  try {
    if (response.status !== 200) return "uncertain";
    const filename = reportCSVFilename(response.headers, name, from, to);
    const blob = await response.blob();
    if (await sessionBoundary(csrf) !== boundary) return "uncertain";
    // The boundary await yields: pagehide/scope change/cookie rotation can happen while it is pending.
    // No await may separate this final synchronous fence from exposing the blob URL/download.
    if (concealed() || csrfCookie() !== csrf) return "uncertain";
    const href = URL.createObjectURL(blob);
    try {
      const link = document.createElement("a");
      link.href = href; link.download = filename; link.rel = "noopener";
      document.body.append(link); link.click(); link.remove();
    } finally { URL.revokeObjectURL(href); }
    return "done";
  } catch { return "uncertain"; }
}
