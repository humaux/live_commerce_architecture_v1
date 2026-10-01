// Safe "return to" target for the staff-invitation flow: /[locale]/invite/[token] links to sign-in/sign-up with a
// `next` parameter so a successful password or OIDC login lands back on the invite page instead of the dashboard.
// inviteNextPath is the single validator on both sides (TeamInvite/Entry/PasswordAuth client-side, the OIDC login +
// callback BFF server-side) and accepts ONLY a same-origin relative invite path: anything else (absolute URL,
// protocol-relative //host, backslashes, dot segments, encoded variants, other routes) returns null, so the callers
// fall back to the plain dashboard redirect. That keeps the mechanism an open-redirect-free no-op: `next` can never
// point off-site because it can never point anywhere but an invite page. The invite token inside the path is never
// logged; the invite page itself keeps its Referrer-Policy: no-referrer header (app/[locale]/invite/[token]/page.tsx).
// It never talks to Go and never stores anything; the token is re-validated by the invite page and by Go on accept.
export const INVITE_NEXT_PATTERN = /^\/(zh-CN|zh-TW|en)\/invite\/[A-Za-z0-9_-]{20,200}$/;

export function inviteNextPath(value: unknown): string | null {
  return typeof value === "string" && INVITE_NEXT_PATTERN.test(value)
    ? value
    : null;
}
