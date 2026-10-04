// meta-connect gate MCG09 (independent test author): the OAuth return can never become an open redirect, and the dialog the merchant
// is sent to can only be Meta's. Pure functions of apps/admin/lib/meta-connect-request.ts (callback 303 target, the binding cookie, the
// callback query bounds, the dialog host check) driven with hostile inputs; the real-browser half is tests/admin/meta-connect-gate.spec.ts.
// Contract: meta-claims-intake-v1 "Merchant connect (R4)" 1 (callback state bound to store + principal; 303 carries no code/state) and the
// unit brief ("open-redirect impossible"). Label: MOCK (no network, no browser). Run by scripts/dev/test-node.sh.
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  callbackRedirect, connectCookie, connectErrorFor, localeFromCookie, parseCallbackQuery, safeDialogURL, storeFromCookie,
} from "../../apps/admin/lib/meta-connect-request.ts";

const uuid = "abcdefab-1111-4111-8111-11111111abcd";
const hostile = [
  "//evil.example", "https://evil.example", "\\\\evil.example", "/\\evil.example", "javascript:alert(1)", "data:text/html,x", "evil.example",
  "../../evil", "%2F%2Fevil.example", "\r\nLocation: https://evil.example", "en\r\nX: y", " //evil.example", "en/../../evil", "zh-TW;evil", "",
  "http://127.0.0.1:3100/en/settings", "@evil.example", "en%0d%0aSet-Cookie:x=y",
];
const SAFE = /^\/(en|zh-TW|zh-CN)\/settings(\?store=[0-9a-f-]{36})?(&(meta_connect=[0-9a-f-]{36}|meta_error=[a-z_]{1,40}))?$|^\/(en|zh-TW|zh-CN)\/settings\?(meta_connect=[0-9a-f-]{36}|meta_error=[a-z_]{1,40})$/;

test("callback redirect target is always a same-origin /<locale>/settings path whatever the cookies, store, code or error say", () => {
  const results: unknown[] = [null, ...hostile.flatMap((h) => [{ connect: h }, { error: h }])];
  for (const h of hostile) {
    const locale = localeFromCookie(`commerce_locale=${h}`);
    assert.ok(["en", "zh-TW", "zh-CN"].includes(locale), `hostile locale cookie produced ${locale}`);
    const store = storeFromCookie(`lc_meta_connect=${h}`);
    assert.equal(store, null, `hostile store cookie ${JSON.stringify(h)} was accepted`);
    for (const result of results) {
      const target = callbackRedirect(locale, store, result as never);
      assert.match(target, SAFE, `unsafe redirect target ${JSON.stringify(target)}`);
      assert.ok(!target.startsWith("//") && !target.includes("evil") && !target.includes("\n") && !target.includes("\r"), target);
      assert.ok(!/code=|state=/.test(target), target);
    }
  }
  // the good path is exactly the documented one
  assert.equal(callbackRedirect("en", uuid, { connect: uuid }), `/en/settings?store=${uuid}&meta_connect=${uuid}`);
  assert.equal(callbackRedirect("zh-TW", uuid, { error: "state_mismatch" }), `/zh-TW/settings?store=${uuid}&meta_error=state_mismatch`);
});

test("the binding cookie: exactly one canonical store uuid, path-scoped to the callback, HttpOnly, Lax, 10 minutes, Secure", () => {
  const cookie = connectCookie(uuid);
  assert.match(cookie, /^lc_meta_connect=[0-9a-f-]{36};/);
  for (const attr of ["Path=/api/meta/callback", "HttpOnly", "SameSite=Lax", "Max-Age=600", "Secure"]) assert.ok(cookie.includes(attr), `${attr} missing: ${cookie}`);
  assert.equal(storeFromCookie(`lc_meta_connect=${uuid}`), uuid);
  assert.equal(storeFromCookie(`a=b; lc_meta_connect=${uuid}; c=d`), uuid);
  assert.equal(storeFromCookie(`lc_meta_connect=${uuid}; lc_meta_connect=${uuid}`), null, "two copies must be refused");
  assert.equal(storeFromCookie(`lc_meta_connect=${uuid.toUpperCase()}`), null);
  assert.equal(storeFromCookie(null), null);
});

test("callback query: exactly one code and one state within charset/length bounds; Meta's error=… and repeats are not forwarded", () => {
  const state = "A".repeat(43);
  assert.deepEqual(parseCallbackQuery(`?code=abc123&state=${state}`), { kind: "ok", code: "abc123", state });
  for (const bad of [
    `?code=abc&state=${state}&code=def`, `?code=abc&state=${state}&state=${state}`, `?code=&state=${state}`, `?code=abc&state=short`,
    `?code=a%20b&state=${state}`, `?code=${"x".repeat(2049)}&state=${state}`, `?state=${state}`, `?code=abc`, `?code=a/b&state=${state}`,
    `?code=a<b>&state=${state}`, `?code=abc&state=${"A".repeat(129)}`,
  ]) assert.equal(parseCallbackQuery(bad).kind, "invalid", bad);
  assert.equal(parseCallbackQuery(`?error=access_denied&error_reason=user_denied&state=${state}`).kind, "denied");
  // extra parameters never reach Go: the helper returns only code + state
  const ok = parseCallbackQuery(`?code=abc&state=${state}&redirect_uri=https://evil.example&next=//evil.example`);
  assert.deepEqual(ok, { kind: "ok", code: "abc", state });
});

test("the dialog URL the BFF hands the browser is https://www.facebook.com only", () => {
  const good = "https://www.facebook.com/v26.0/dialog/oauth?client_id=1&config_id=2&response_type=code&state=x";
  assert.equal(safeDialogURL(good), good);
  for (const bad of [
    "http://www.facebook.com/v26.0/dialog/oauth", "https://facebook.com.evil.example/v26.0/dialog/oauth", "https://www.facebook.com.evil.example/",
    "https://evil.example/https://www.facebook.com/", "//www.facebook.com/v26.0/dialog/oauth", "https://user:pw@www.facebook.com/x", "javascript:alert(1)",
    "https://www.facebook.com:8443/v26.0/dialog/oauth", "https://m.facebook.com/v26.0/dialog/oauth", "https://www.facebook.com@evil.example/", "", null, undefined, 7, {},
  ]) assert.equal(safeDialogURL(bad as never), null, `accepted ${JSON.stringify(bad)}`);
});

test("callback error mapping only ever yields fixed codes, never an upstream message", () => {
  const fixed = new Set(["state_mismatch", "state_expired", "meta_connect_failed", "forbidden", "invalid_request", "unavailable"]);
  for (const status of [200, 400, 401, 403, 404, 409, 410, 422, 500, 502, 503]) {
    for (const body of [{}, { code: "page_taken" }, { code: "<script>" }, { error: "https://evil.example" }, { message: "SENTINEL-EAAP-token" }, null, "x"]) {
      const out = connectErrorFor(status, body);
      assert.ok(out === null || fixed.has(out), `status ${status} body ${JSON.stringify(body)} -> ${out}`);
    }
  }
  assert.equal(connectErrorFor(401, {}), null);
});
