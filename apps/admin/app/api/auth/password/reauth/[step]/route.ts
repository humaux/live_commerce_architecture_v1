// Purpose: the admin BFF of recent re-authentication (contracts/merchant-password-auth-v1.md §16, §16.1; card OC-04).
//   GET  /api/auth/password/reauth/method   -> {password, oidc, account}: which route the dialog offers (no upstream call).
//   POST /api/auth/password/reauth/start    {password,locale} -> Go POST /v1/identity/password/reauth/start    -> 200 {expires_at}
//   POST /api/auth/password/reauth/complete {code}            -> Go POST /v1/identity/password/reauth/complete -> 200 {authenticated_at}
//   POST /api/auth/password/reauth/oidc     {locale,return_to} -> Go POST /v1/identity/login/reauth/start      -> 200 {authorization_url}
//     and the login cookie marked as a re-auth flow; app/api/auth/callback/route.ts finishes it without touching the session cookie.
// Depends on: lib/auth (session token, Origin/CSRF, privateIdentity with the BFF key, login cookie, OIDC session marker),
//   lib/password-request (client IP, as the password routes).
// Used by: components/ReauthDialog.tsx; tests/admin/oc/reauth.spec.ts (through tests/foundation/browser_oc_test.go).
// Invariants: the session is the cookie's bearer only (no email or principal in a body); a refused re-auth (403 reauth_failed,
//   422 invalid_code) never clears a cookie - only an upstream 401 does; password and code are forwarded once, never logged,
//   echoed or retried; every error is rebuilt locally from an allow-listed status/code (an unknown one is 503 retry_later).
import {
  authConfig,
  clearAuthCookies,
  disabledResponse,
  exactJSON,
  hasNoQuery,
  isLocale,
  localError,
  oidcSession,
  passwordLoginOn,
  privateIdentity,
  reauthReturnPath,
  requireCSRF,
  requireOrigin,
  safeError,
  safeJSON,
  sessionToken,
  setLoginCookie,
  validAuthorizationURL,
  type Locale,
} from "@/lib/auth";
import { clientIPFromHeaders } from "@/lib/password-request";

const nostore = { "Cache-Control": "no-store" };
type Step = "start" | "complete" | "oidc";
// §16.1 statuses and codes per step; anything else from Go is a BFF-level 503 retry_later.
const allowed: Record<Step, Record<number, readonly string[]>> = {
  start: { 400: ["invalid_request"], 401: ["unauthorized"], 403: ["reauth_failed"], 429: ["throttled"], 503: ["mail_unavailable", "busy"] },
  complete: { 400: ["invalid_request"], 401: ["unauthorized"], 422: ["invalid_code"], 429: ["throttled"], 503: ["busy"] },
  oidc: { 401: ["unauthorized"], 403: ["forbidden"] },
};
const keys: Record<Step, readonly string[]> = { start: ["password", "locale"], complete: ["code"], oidc: ["locale", "return_to"] };

type Context = { params: Promise<{ step: string }> };

export async function GET(request: Request, context: Context) {
  if (!authConfig) return disabledResponse();
  const { step } = await context.params;
  if (step !== "method") return localError(405, "method_not_allowed", "POST");
  if (!hasNoQuery(request)) return localError(422, "invalid_request");
  const token = sessionToken(request);
  if (!token) return localError(401, "unauthorized");
  const password = passwordLoginOn();
  const oidc = !!authConfig.issuer;
  // PD9: an account is password XOR OIDC. Only a session the OIDC callback issued carries the matching marker.
  const account = oidc && (!password || oidcSession(request, token)) ? "oidc" : "password";
  return Response.json({ password, oidc, account }, { headers: nostore });
}

export async function POST(request: Request, context: Context) {
  if (!authConfig) return disabledResponse();
  const { step } = await context.params;
  if (step !== "start" && step !== "complete" && step !== "oidc") return localError(404, "not_found");
  if (step === "oidc" ? !authConfig.issuer : !passwordLoginOn()) return disabledResponse();
  if (!hasNoQuery(request) || !requireOrigin(request) || !requireCSRF(request)) return localError(403, "forbidden");
  const token = sessionToken(request);
  if (!token) return localError(401, "unauthorized");
  let body: Record<string, unknown>;
  try {
    body = await exactJSON(request, keys[step]);
  } catch {
    return localError(422, "invalid_request");
  }
  let upstream: Response;
  let returnTo: string | null = null;
  if (step === "oidc") {
    returnTo = reauthReturnPath(body.return_to);
    if (typeof body.locale !== "string" || !isLocale(body.locale) || !returnTo) return localError(422, "invalid_request");
    upstream = await privateIdentity("login/reauth/start", {}, token);
  } else {
    if (step === "start" ? typeof body.password !== "string" || body.password.length < 1 || body.password.length > 1024 ||
      typeof body.locale !== "string" || !isLocale(body.locale) : typeof body.code !== "string" || body.code.length > 32)
      return localError(422, "invalid_request");
    const header = request.headers.get("x-forwarded-for");
    const ip = clientIPFromHeaders(header === null ? [] : [header]);
    if ("status" in ip) return ip.status === 400 ? localError(400, "invalid_request") : localError(503, "retry_later");
    // 14 s on start: Go waits for the code mail (<= 10 s, §11). Never retried: a repeat burns throttles and mail budget.
    upstream = await privateIdentity(`password/reauth/${step}`, step === "start" ? { password: body.password, locale: body.locale } : { code: body.code },
      token, undefined, { timeoutMs: step === "start" ? 14000 : 6000, clientIP: ip.ip });
  }
  if (!upstream.ok) {
    const safe = await safeError(upstream);
    const code = ((await safe.clone().json().catch(() => ({}))) as { code?: string }).code ?? "";
    if (!(allowed[step][upstream.status] ?? []).includes(code)) return localError(503, "retry_later");
    if (upstream.status === 401) clearAuthCookies(safe.headers); // only a dead session signs out; 403/422 keep it (§16)
    return safe;
  }
  const result = await safeJSON<Record<string, unknown>>(upstream);
  if (step === "oidc") {
    const location = validAuthorizationURL(result?.authorization_url);
    if (!location || typeof result?.binding !== "string") return localError(503, "retry_later");
    const response = Response.json({ authorization_url: location }, { headers: nostore });
    if (!setLoginCookie(response.headers, result.binding, body.locale as Locale, result.expires_at, null, returnTo)) return localError(503, "retry_later");
    return response;
  }
  const field = step === "start" ? "expires_at" : "authenticated_at";
  const at = result?.[field];
  if (typeof at !== "string" || !Number.isFinite(Date.parse(at))) return localError(503, "retry_later");
  return Response.json({ [field]: at }, { headers: nostore });
}

const unsupported = () => (authConfig ? localError(405, "method_not_allowed", "GET, POST") : disabledResponse());
export const PUT = unsupported;
export const DELETE = unsupported;
export const PATCH = unsupported;
export const OPTIONS = unsupported;
export const HEAD = unsupported;
