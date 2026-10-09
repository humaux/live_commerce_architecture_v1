// Owns the admin BFF grammar for Studio › Claims: the exact M1–M7 resource paths
// (contracts/live-keyword-claims-v1.md §7.1, §11.1) plus GET/PUT claim-source, the closed
// shape of the one response that carries a buyer link token (M7), and the comment-source
// input validator and PUT body builder.
// Non-goals: no authentication, CSRF or store authority (app/api/stores route.ts and
// Go decide those), no domain validation of claim bodies (Go claims decides), no UI.
// Depends on nothing (pure; imported by proxy.ts, the BFF route, the browser client and
// node:test), so the same grammar gates the raw URL before and after Next routing.

const uuid = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";

// Relative to `live-sessions/{session_id}/`: every claims sub-resource, plus the Meta
// comment source (claim-source: GET read, PUT bind; claim-source-v1 frozen HTTP interface).
export const claimsSubpath = `(?:claims(?:/(?:window|offers(?:/${uuid})?|offer-import|library(?:/${uuid})?|manual|bundles(?:/${uuid}/link)?|blocklist/check))?|claim-source)`;
// Relative to `stores/{store_id}/`, per method (M1/M6/source/library GET, M2/M3/M5/M7/offer-import POST, M4 PATCH,
// source and library-row PUT; Live tools R4, amendment in the claims contract). The BFF forwards each path unchanged to /v1/admin/stores/{store_id}/<path>.
export const claimsRoutes = {
  GET: `live-sessions/${uuid}/(?:claims(?:/bundles|/library|/blocklist/check)?|claim-source)`,
  POST: `live-sessions/${uuid}/claims/(?:window|offers|offer-import|manual|bundles/${uuid}/link)`,
  PATCH: `live-sessions/${uuid}/claims/offers/${uuid}`,
  PUT: `live-sessions/${uuid}/(?:claim-source|claims/library/${uuid})`,
} as const;
const bundlesCollection = new RegExp(`^live-sessions/${uuid}/claims/bundles$`);
const linkRoute = new RegExp(`^live-sessions/${uuid}/claims/bundles/${uuid}/link$`);
const blocklistCheck = new RegExp(`^live-sessions/${uuid}/claims/blocklist/check$`);
const blocklistQuery = new RegExp(`^\\?bundle_id=${uuid}$`);

/** The warning reads only one server-known bundle; no actor key or blocklist write is accepted. */
export function claimBlocklistCheck(path: string): boolean { return blocklistCheck.test(path); }
/** Preserve the exact raw query grammar before and after Next routing, including encoding rejection. */
export function validClaimBlocklistQuery(url: string): boolean { return blocklistQuery.test(new URL(url).search); }
/** W3-05B exposes only the boolean restriction state, never a note or actor identifier. */
export function validClaimRestriction(value: unknown): value is { restricted: boolean } {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    && Object.keys(value).length === 1 && typeof (value as { restricted?: unknown }).restricted === "boolean";
}

/** M6 uses the Studio limit/cursor query; blocklist/check has its separate exact bundle query. */
export function claimsCollection(path: string) {
  return bundlesCollection.test(path);
}

/** M7 carries the one-time token; the BFF must validate it and forbid a Referer. */
export function claimLinkRoute(path: string) {
  return linkRoute.test(path);
}

export type ClaimLink = {
  token: string | null;
  generation: number;
  expires_at: string;
  released: boolean;
  replayed: boolean;
};

// Closed M7 shape: a canonical 43-character token on first execution, null only on a
// replay. Anything else is treated as an unknown result, never forwarded as a link.
export function validClaimLink(value: unknown): value is ClaimLink {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const item = value as Record<string, unknown>;
  const fields = ["token", "generation", "expires_at", "released", "replayed"];
  if (Object.keys(item).length !== fields.length || fields.some((key) => !Object.hasOwn(item, key))) return false;
  if (typeof item.replayed !== "boolean" || typeof item.released !== "boolean" ||
    !Number.isSafeInteger(item.generation) || (item.generation as number) < 1 ||
    typeof item.expires_at !== "string" || !Number.isFinite(Date.parse(item.expires_at))) return false;
  if (item.replayed) return item.token === null;
  return typeof item.token === "string" && /^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/.test(item.token);
}

// Comment source (Meta post / live video / Instagram media binding). The server parses the
// pasted text into (object, source_object_id); the browser only refuses what can never be
// a single link or id, so a typo costs no round trip. Whether it is a supported URL is Go's call.
export const claimSourceInputMax = 1024;
export function validClaimSourceInput(value: string): boolean {
  const text = value.trim();
  return text.length >= 1 && text.length <= claimSourceInputMax && !/[\s\p{Cc}]/u.test(text);
}

export type ClaimSourceForm = {
  input: string; private_reply: boolean; reply_locale: "zh-TW" | "zh-CN" | "en"; active: boolean;
  /** Ruling p: optional hint; "" = let Go take the platform from the pasted link. */
  platform: "" | "facebook" | "instagram";
};
/**
 * The exact PUT body: five keys, input trimmed, CAS version 0 for the first bind, plus the
 * optional `platform` key (ruling p) only when a platform is chosen — never null or "".
 */
export function claimSourceBody(form: ClaimSourceForm, expectedVersion: number) {
  const body = { input: form.input.trim(), private_reply: form.private_reply, reply_locale: form.reply_locale,
    active: form.active, expected_version: expectedVersion };
  return form.platform ? { ...body, platform: form.platform } : body;
}
