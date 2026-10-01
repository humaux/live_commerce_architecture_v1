// BFF request grammar for the merchant Facebook Page / Instagram connect (contract meta-claims-intake-v1 "Merchant connect (R4)"):
// the generic store route `/api/stores/{store}/meta-connect/*` -> Go `/v1/admin/stores/{store_id}/meta-connect/*`
// (internal/httpapi/meta_connect.go), plus the pure helpers of the two dedicated routes `/api/meta/connect` (POST -> Go POST
// meta-connect/start) and `/api/meta/callback` (GET -> Go GET meta-connect/callback), the redirect URI registered on the Meta app.
// Shares the OAuth-return helpers of ads-request.ts (dialog host check, callback query bounds, locale, canonical store).
// Non-goals: no authentication, CSRF or store authority (route handlers + Go decide), no pick/permission rule (Go decides).
// Pure and dependency-free so route code and node:test share one grammar.
import { canonicalStore, localeFromCookie, type AdsLocale } from "./ads-request.ts";

export { localeFromCookie, parseCallbackQuery, safeDialogURL, canonicalStore, validIdempotencyKey } from "./ads-request.ts";
export type MetaLocale = AdsLocale;

const uuid = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";

// Relative to `stores/{store_id}/`, per method; fragments spliced into the generic `[...resource]` table. meta-connect/start and
// meta-connect/callback are the dedicated routes, not these.
export const metaConnectRoutes = {
  GET: `meta-connect/(?:status|states/${uuid})`,
  POST: `meta-connect/(?:pick|disconnect)`,
} as const;
export const metaConnectAny = new RegExp(`^(?:${metaConnectRoutes.GET}|${metaConnectRoutes.POST})$`);

/** No query on any meta-connect resource (raw URL, checked before Next drops an empty '?'); a GET carries no body or key. */
export function validMetaConnectRequest(request: { method: string; url: string; body: unknown; headers: Headers }): boolean {
  if (request.url.includes("?")) return false;
  if (request.method !== "GET") return true;
  return request.body === null && !request.headers.has("transfer-encoding") && !request.headers.has("idempotency-key") &&
    (!request.headers.has("content-length") || request.headers.get("content-length") === "0");
}

// ---------- connect BFF (dedicated route) ----------
export const connectCookieName = "lc_meta_connect";
export const connectCookiePath = "/api/meta/callback";
const cookieAttrs = `Path=${connectCookiePath}; Secure; HttpOnly; SameSite=Lax`;
/** Binds the callback to the store the merchant started from (10 min = the state TTL). Lax so the top-level return from Meta carries it. */
export const connectCookie = (store: string) => `${connectCookieName}=${store}; ${cookieAttrs}; Max-Age=600`;
export const clearConnectCookie = () => `${connectCookieName}=; ${cookieAttrs}; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:00 GMT`;

/** The only body the connect BFF accepts: `{"store":"<uuid>"}`. */
export function parseConnectBody(text: string): string | null {
  try {
    const value: unknown = JSON.parse(text);
    if (!value || typeof value !== "object" || Array.isArray(value)) return null;
    const keys = Object.keys(value);
    const store = (value as Record<string, unknown>).store;
    return keys.length === 1 && typeof store === "string" && canonicalStore(store) ? store : null;
  } catch {
    return null;
  }
}

/** Store from the `lc_meta_connect` cookie, exactly one copy, canonical uuid, else null (-> state_mismatch). */
export function storeFromCookie(header: string | null): string | null {
  const found: string[] = [];
  for (const part of (header ?? "").split(";")) {
    const at = part.indexOf("=");
    if (at >= 0 && part.slice(0, at).trim() === connectCookieName) found.push(part.slice(at + 1).trim());
  }
  return found.length === 1 && canonicalStore(found[0]) ? found[0] : null;
}

/** Callback 303 target: back to Settings with only fixed codes and canonical uuids in the URL (never code/state). */
export function callbackRedirect(locale: MetaLocale, store: string | null, result: { connect: string } | { error: string } | null): string {
  const query: string[] = [];
  if (store) query.push(`store=${store}`);
  if (result && "connect" in result && new RegExp(`^${uuid}$`).test(result.connect)) query.push(`meta_connect=${result.connect}`);
  else if (result && "error" in result && /^[a-z_]{1,40}$/.test(result.error)) query.push(`meta_error=${result.error}`);
  return `/${locale}/settings${query.length ? `?${query.join("&")}` : ""}`;
}

/** Go status -> the fixed `meta_error` value; null = signed out (the page shows its own sign-in state). */
export function connectErrorFor(status: number, body: unknown): string | null {
  const o = body && typeof body === "object" ? (body as Record<string, unknown>) : {};
  const raw = typeof o.code === "string" ? o.code : typeof o.error === "string" ? o.error : "";
  if (status === 401) return null;
  if (raw === "state_mismatch" || raw === "state_expired" || raw === "meta_connect_failed") return raw;
  if (status === 403 || status === 404) return "forbidden";
  if (status === 409) return "state_mismatch";
  if (status === 410) return "state_expired";
  if (status === 502) return "meta_connect_failed";
  if (status === 400 || status === 422) return "invalid_request";
  return "unavailable";
}

