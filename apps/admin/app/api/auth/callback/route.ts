// GET /api/auth/callback → POST /v1/identity/login/complete (internal/identityhttp); OIDC only, 404 without COMMERCE_OIDC_ISSUER (U4).
// OC-04 (§16): the same callback finishes an OIDC re-auth flow (started by /api/auth/password/reauth/oidc). Go answers
// {reauth:true, authenticated_at} with no token; then NO session cookie is set or cleared and the browser returns to the page
// that opened the re-auth dialog, with #reauth-done (or #reauth-failed on a refusal) for components/ReauthDialog.tsx.
import {
  authConfig,
  authFailure,
  clearLoginCookie,
  disabledResponse,
  loginBinding,
  privateIdentity,
  redirect,
  safeJSON,
  setOIDCHint,
  setSessionCookies,
} from "@/lib/auth";

function callbackQuery(request: Request) {
  const params = new URL(request.url).searchParams;
  const keys = [...params.keys()];
  if (keys.some((key) => !["state", "code", "iss", "error"].includes(key)))
    return null;
  if (
    ["state", "code", "iss", "error"].some(
      (key) => params.getAll(key).length > 1,
    )
  )
    return null;
  if (params.has("error")) return { error: true as const };
  const state = params.get("state") ?? "";
  const code = params.get("code") ?? "";
  const iss = params.get("iss");
  if (
    !state ||
    !code ||
    state.length > 2048 ||
    code.length > 4096 ||
    (iss !== null && (iss.length > 2048 || iss !== authConfig?.issuer))
  )
    return null;
  return { state, code };
}

export async function GET(request: Request) {
  if (!authConfig?.issuer) return disabledResponse(); // U4: no OIDC issuer => no callback
  const binding = loginBinding(request);
  const locale = binding?.locale ?? "zh-CN";
  const query = callbackQuery(request);
  if (!binding || !query || "error" in query) {
    const response = authFailure(locale);
    clearLoginCookie(response.headers);
    return response;
  }
  const upstream = await privateIdentity("login/complete", {
    state: query.state,
    binding: binding.binding,
    code: query.code,
  });
  if (!upstream.ok) {
    // A refused re-auth (403 reauth_failed) keeps the session: back to the dialog's page, never the signed-out failure page.
    const response = binding.reauth ? redirect(`${binding.reauth}#reauth-failed`) : authFailure(locale);
    clearLoginCookie(response.headers);
    return response;
  }
  const body = await safeJSON<{ token?: unknown; expires_at?: unknown; reauth?: unknown }>(
    upstream,
  );
  if (body?.reauth === true) {
    // Re-auth: the session is the one the browser already holds; setSessionCookies is never called here.
    const response = redirect(`${binding.reauth ?? `/${locale}/`}#reauth-done`);
    clearLoginCookie(response.headers);
    return response;
  }
  if (binding.reauth) {
    // A re-auth cookie must never end in a new session, whatever the upstream answered.
    const response = redirect(`${binding.reauth}#reauth-failed`);
    clearLoginCookie(response.headers);
    return response;
  }
  // invite-next: a validated invite path from the login cookie wins over the dashboard; inviteNextPath already
  // re-checked it, so a tampered cookie degrades to the plain dashboard redirect (never off-site).
  const response = redirect(binding.next ?? `/${locale}/`);
  clearLoginCookie(response.headers);
  if (!setSessionCookies(response.headers, body?.token, body?.expires_at)) {
    const failed = authFailure(locale);
    clearLoginCookie(failed.headers);
    return failed;
  }
  setOIDCHint(response.headers, body?.token, body?.expires_at);
  return response;
}

const unsupported = () =>
  authConfig?.issuer
    ? new Response(null, {
        status: 405,
        headers: { Allow: "GET", "Cache-Control": "no-store" },
      })
    : disabledResponse();
export const POST = unsupported;
export const PUT = unsupported;
export const DELETE = unsupported;
export const PATCH = unsupported;
export const OPTIONS = unsupported;
export const HEAD = unsupported;
