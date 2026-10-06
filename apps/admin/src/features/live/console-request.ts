// Purpose: Exact LC-U1 BFF resource/query/body grammar for A1/A7/A6 and A5 results/copy.
// Depends on: Web URL/JSON primitives, frozen live-console-v1 §§7/9/11 and Go live_flow.go.
// Used by: stores/[store]/[...resource]/route.ts and console-client.ts.
// Invariants: I01/I02/I11/I14; no generic proxy, caller tenant, or query aliases.
const uuid = "[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}";
const sid = `live-sessions/${uuid}`;
/** Frozen path expressions embedded by the shared BFF allowlist. */
export const consolePaths = {
  GET: `live-sessions/results|${sid}/console`,
  POST: `${sid}/(?:copy|lifecycle)|${sid}/claims/offers/${uuid}/recommend`,
};
/** Methods admitted by this unit only; shared Studio/Claims resources stay in their existing allowlists. */
export const consoleRoutes: Record<string, RegExp> = { GET: new RegExp(`^(?:${consolePaths.GET})$`), POST: new RegExp(`^(?:${consolePaths.POST})$`) };
/** Recognised console family, including method mismatches that must return 405. */
export const consoleAny = new RegExp(`^(?:${consolePaths.GET}|${consolePaths.POST})$`);
/** Canonical, nonzero UUID used before building a same-origin scoped path. */
export const validConsoleID = (v: unknown): v is string => typeof v === "string" && new RegExp(`^${uuid}$`).test(v) && v !== "00000000-0000-0000-0000-000000000000";
/** Raw query validation; the results GET requires 1..50 unique repeated session_id values. */
export function validConsoleQuery(raw: string, path: string): boolean {
  const at = raw.indexOf("?");
  if (path !== "live-sessions/results") return at === -1;
  if (at < 0 || raw.length - at > 4097) return false;
  const segments = raw.slice(at + 1).split("&");
  if (segments.length < 1 || segments.length > 50) return false;
  const ids = segments.map((segment) => /^session_id=(.+)$/.exec(segment)?.[1]);
  return ids.every(validConsoleID) && new Set(ids).size === ids.length;
}
/** Strict JSON command body validation before forwarding to Go; no extra keys or client scope. */
export function validConsoleBody(path: string, body: string): boolean {
  let value: unknown;
  try { value = JSON.parse(body); } catch { return false; }
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const r = value as Record<string, unknown>;
  if (!Number.isSafeInteger(r.expected_version) || (r.expected_version as number) < 1) return false;
  const keys = Object.keys(r);
  if (path.endsWith("/lifecycle")) return ["start", "end", "archive"].includes(r.action as string) &&
    keys.every((k) => ["action", "expected_version", "open_window"].includes(k)) && (!Object.hasOwn(r, "open_window") || typeof r.open_window === "boolean");
  if (path.endsWith("/recommend")) return keys.length === 2 && keys.every((k) => ["expected_version", "post_comment"].includes(k)) && typeof r.post_comment === "boolean";
  if (path.endsWith("/copy")) {
    if (keys.length !== 3 || keys.some((k) => !["title", "scheduled_at", "expected_version"].includes(k)) ||
      typeof r.title !== "string" || r.title.trim() !== r.title || Array.from(r.title).length < 1 || Array.from(r.title).length > 200 || /[\p{Cc}]/u.test(r.title)) return false;
    if (r.scheduled_at === null) return true;
    if (typeof r.scheduled_at !== "string" || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?(?:Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$/.test(r.scheduled_at)) return false;
    const parsed = new Date(r.scheduled_at);
    const wall = new Date(`${r.scheduled_at.slice(0, 19)}Z`);
    return Number.isFinite(parsed.getTime()) && Number.isFinite(wall.getTime()) && wall.toISOString().slice(0, 19) === r.scheduled_at.slice(0, 19) &&
      parsed.getUTCFullYear() >= 2000 && parsed.getUTCFullYear() <= 2199;
  }
  return false;
}
