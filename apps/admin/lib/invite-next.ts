// Safe "return to" target for the staff-invitation flow: /[locale]/invite/[token] links to sign-in/sign-up with the invite
// path in the URL FRAGMENT (`#next=...`) so a successful password or OIDC login lands back on the invite page instead of
// the dashboard. The fragment is never sent to any server, so the invite token stays out of every request URL, Referer
// header and access log (the token is only in the emailed link and its one document request; the staff-team browser gate
// pins that). A query-string `next` would leak it, so no server page reads one.
// inviteNextPath is the single validator on both sides (PasswordAuth client-side, the OIDC login + callback BFF
// server-side) and accepts ONLY a same-origin relative invite path: anything else (absolute URL, protocol-relative //host,
// backslashes, dot segments, encoded variants, other routes) returns null, so the callers fall back to the plain
// dashboard redirect. That keeps the mechanism an open-redirect-free no-op: `next` can never point off-site because it
// can never point anywhere but an invite page. It never talks to Go and never stores anything; the token is re-validated
// by the invite page and by Go on accept.
export const INVITE_NEXT_PATTERN = /^\/(zh-CN|zh-TW|en)\/invite\/[A-Za-z0-9_-]{20,200}$/;

export function inviteNextPath(value: unknown): string | null {
  return typeof value === "string" && INVITE_NEXT_PATTERN.test(value)
    ? value
    : null;
}

// The fragment carrier: `#next=<encoded invite path>`; inviteNextFromHash validates whatever the address bar holds.
export const inviteNextHash = (path: string) => `#next=${encodeURIComponent(path)}`;
export const inviteNextFromHash = (hash: string) =>
  inviteNextPath(new URLSearchParams(hash.replace(/^#/, "")).get("next"));
