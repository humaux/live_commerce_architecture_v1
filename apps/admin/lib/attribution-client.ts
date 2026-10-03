// Browser GET /api/stores/{store}/ads/attribution -> Go GET /v1/admin/stores/{store}/ads/attribution (ads:read).
// No Meta writes; the response is private and never cached. Shapes and request-window identity fail closed.
import { parseAttributionReport } from "./attribution-model.ts";
import { csrfCookie, sessionBoundary } from "./settings-client.ts";
import { parseAudienceAck, type AudienceAck } from "./attribution-audience.ts";
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

export type AudienceReadResult =
  | { kind: "queued"; ack: AudienceAck }
  | { kind: "unknown" | "forbidden" | "signed-out" | "failed" };
// This POST writes the local operation intention; its worker only performs guarded Meta GETs. Never auto-repeat UNKNOWN with a fresh key.
export async function requestAudienceRead(
  store: string,
  session: string,
  key: string,
  boundary: string,
): Promise<AudienceReadResult> {
  const csrf = csrfCookie();
  try {
    if (
      !csrf ||
      (await sessionBoundary(csrf)) !== boundary ||
      csrfCookie() !== csrf
    )
      return { kind: "signed-out" };
  } catch {
    return { kind: "signed-out" };
  }
  let response: Response;
  try {
    response = await fetch(
      `/api/stores/${encodeURIComponent(store)}/ads/sessions/${encodeURIComponent(session)}/audience-read`,
      {
        method: "POST",
        credentials: "same-origin",
        cache: "no-store",
        body: "{}",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": csrf,
          "Idempotency-Key": key,
        },
        signal: AbortSignal.timeout(8000),
      },
    );
  } catch {
    return { kind: "unknown" };
  }
  if (response.status === 401) return { kind: "signed-out" };
  if (response.status === 403) return { kind: "forbidden" };
  if (response.status >= 500) return { kind: "unknown" };
  if (!response.ok) return { kind: "failed" };
  try {
    if (
      response.headers.get("content-type")?.split(";", 1)[0] !==
      "application/json"
    )
      return { kind: "unknown" };
    return { kind: "queued", ack: parseAudienceAck(await response.json()) };
  } catch {
    return { kind: "unknown" };
  } // The operation may have committed even when its acknowledgement was unusable.
}
