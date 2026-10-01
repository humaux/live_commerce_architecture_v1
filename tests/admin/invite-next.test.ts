// invite-next (apps/admin/lib/invite-next.ts): the safe "return to" validator behind the staff-invitation login flow.
// The invite page (app/[locale]/invite/[token]/page.tsx) links to sign-in/sign-up with a `next` parameter; both the
// client (components/PasswordAuth.tsx) and the OIDC BFF (app/api/auth/login/route.ts -> callback) redirect to it ONLY
// when it is a same-origin relative invite path. This suite pins the accepted shape and the hostile shapes that must
// be rejected so `next` can never become an open redirect (absolute URL, protocol-relative //, backslashes, dot
// segments, encoded variants, over/under-length tokens, wrong locale case). Run: node --test --experimental-strip-types tests/admin/invite-next.test.ts
import assert from "node:assert/strict";
import { test } from "node:test";
import { INVITE_NEXT_PATTERN, inviteNextPath } from "../../apps/admin/lib/invite-next.ts";

const GOOD = "abcdefghijklmnopqrst123-_ABCDE"; // 33 chars, inside [20,200]

test("accepts exactly the three locale invite paths with a 20-200 char token", () => {
  for (const locale of ["zh-CN", "zh-TW", "en"]) {
    const path = `/${locale}/invite/${GOOD}`;
    assert.equal(inviteNextPath(path), path);
  }
  for (const token of ["a".repeat(20), "A_b-C9".repeat(33).slice(0, 198), "x".repeat(200)]) {
    assert.equal(inviteNextPath(`/en/invite/${token}`), `/en/invite/${token}`);
  }
  assert.equal(INVITE_NEXT_PATTERN.test(`/zh-TW/invite/${GOOD}`), true);
});

test("rejects anything that is not an exact same-origin relative invite path", () => {
  const bad = [
    null,
    undefined,
    42,
    {},
    "",
    "/en/invite", // no token
    `/en/invite/${"y".repeat(19)}`, // token too short (< 20)
    `/en/invite/${"y".repeat(201)}`, // token too long (> 200)
    "/en/invite/abc", // far too short
    "//evil.com/zh-CN/invite/" + GOOD, // protocol-relative
    "https://evil.com/en/invite/" + GOOD, // absolute URL
    "https:\\/\\/evil.com/en/invite/" + GOOD, // escaped absolute URL
    `/en/../invite/${GOOD}`, // dot segments
    `/en/./invite/${GOOD}`,
    `/en/invite/../x`,
    `/en/invite/${GOOD}/..`,
    `/en/invite/${GOOD}/extra`, // trailing segment
    `/EN/invite/${GOOD}`, // wrong locale case
    `/zh-cn/invite/${GOOD}`,
    `/fr/invite/${GOOD}`, // unknown locale
    `/en/invites/${GOOD}`, // wrong route
    `/en/invite/${GOOD}?x=1`, // query string
    `/en/invite/${GOOD}#frag`, // fragment
    `/en/invite/${"a b".repeat(10)}`, // whitespace in token
    `/en/invite/${"a%20b".repeat(8)}`, // percent sign not in the token alphabet
    `/en\\invite\\${GOOD}`, // backslashes
    `\\/en/invite/${GOOD}`, // leading backslash
    `/en/invite/ä${GOOD}`, // non-ASCII token char
    `/ en/invite/${GOOD}`, // space after slash
    `/en/invite/${GOOD} `.trimEnd() + " ", // trailing space
    ` /en/invite/${GOOD}`, // leading space
  ];
  for (const value of bad) assert.equal(inviteNextPath(value), null, String(value));
});

test("rejects percent-encoded variants of the exact shape", () => {
  // Encoded slashes/dots must NOT decode into an accepted path: the validator sees the raw string and the
  // alphabet excludes %, so encoded traversal or absolute URLs are dead on arrival.
  const enc = [
    `%2Fen%2Finvite%2F${GOOD}`,
    `/%65n/invite/${GOOD}`, // %65 = 'e' — still contains '%', rejected
    `/en/invite/%2e%2e`,
    `/en%2finvite%2f${GOOD}`,
    `/en/invite/${encodeURIComponent(GOOD)}x%3d`,
  ];
  for (const value of enc) assert.equal(inviteNextPath(value), null, value);
});

test("the token is an opaque invite token shape, never a URL", () => {
  // invite tokens are base64url-ish 43-char strings today (app/[locale]/invite/[token]/page.tsx) but the
  // validator deliberately accepts the wider [20,200] alphabet so the BFF does not hard-fail when Go re-issues.
  assert.equal(inviteNextPath("/zh-CN/invite/" + "Ab3-x_9".repeat(6).slice(0, 42)), "/zh-CN/invite/" + "Ab3-x_9".repeat(6).slice(0, 42));
  assert.equal(inviteNextPath("/zh-CN/invite/" + GOOD + "/"), null, "trailing slash changes the path");
});
