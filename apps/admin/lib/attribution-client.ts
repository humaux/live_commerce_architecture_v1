// Browser GET /api/stores/{store}/ads/attribution -> Go GET /v1/admin/stores/{store}/ads/attribution (ads:read).
// No Meta writes; the response is private and never cached. Shapes and request-window identity fail closed.
import { parseAttributionReport } from "./attribution-model";
export type AttributionError = "signed-out" | "forbidden" | "unavailable";
export async function readAttribution(
  store: string,
  from: string,
  to: string,
  signal: AbortSignal,
) {
  const response = await fetch(
    `/api/stores/${encodeURIComponent(store)}/ads/attribution?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
    { method: "GET", cache: "no-store", credentials: "same-origin", signal },
  );
  if (response.status === 401) throw new Error("signed-out");
  if (response.status === 403) throw new Error("forbidden");
  if (
    !response.ok ||
    response.headers.get("content-type")?.split(";", 1)[0] !==
      "application/json"
  )
    throw new Error("unavailable");
  const report = parseAttributionReport(await response.json());
  if (report.window.from !== from || report.window.to !== to)
    throw new Error("unavailable");
  return report;
}
