// Unit tests (pure) for the merchant Page/Instagram connect: the closed parsers of the Go answers (apps/admin/lib/meta-connect-model.ts)
// and the BFF grammar / callback helpers (apps/admin/lib/meta-connect-request.ts). The browser gate is --browser-meta-connect;
// this file proves shapes and bounds only.
import assert from "node:assert/strict";
import { test } from "node:test";
import { parsePickState, parseStatus, pickable } from "../../apps/admin/lib/meta-connect-model.ts";
import {
  callbackRedirect, clearConnectCookie, connectCookie, connectErrorFor, metaConnectAny, metaConnectRoutes, parseConnectBody,
  storeFromCookie, validMetaConnectRequest,
} from "../../apps/admin/lib/meta-connect-request.ts";
import { metaConnectCopy } from "../../apps/admin/lib/meta-connect-copy.ts";
import { pageSourceInput } from "../../apps/admin/lib/meta-page-source.ts";

const uuid = "abcdefab-1111-4111-8111-11111111abcd";
const stamp = "2026-10-01T08:30:00.123456Z";

test("selected Page qualifies only a bare Facebook id, never rewrites another Page or external URL", () => {
  assert.equal(pageSourceInput(" 456 ", "123"), "123_456");
  assert.equal(pageSourceInput("123_456", "123"), "123_456");
  for (const path of ["posts", "videos"]) {
    const link = `https://www.facebook.com/123/${path}/456?foo=bar#comment`;
    assert.equal(pageSourceInput(link, "123"), link);
  }
  for (const input of ["9_456", "https://www.facebook.com/9/posts/456", "https://evil.example/123/posts/456",
    "https://facebook.com@evil.example/123/posts/456", "https://www.facebook.com/name/posts/456", "javascript:123",
    "https://fb.watch/123", "https://instagram.com/reel/abcdef", "123_456_other", "https://facebook.com:444/123/posts/456"]) {
    assert.equal(pageSourceInput(input, "123"), null, input);
  }
  assert.equal(pageSourceInput("456", "bad"), null);
});

test("status: not connected, connected with and without Instagram, and every deviation throws", () => {
  const empty = { connected: false, count: 0, cap: 10, pages: [] };
  assert.deepEqual(parseStatus(empty), empty);
  const page = {
    id: "123", name: "Shop", status: "active", instagram: null, permissions: ["pages_messaging"],
    connected_at: stamp, route_expires_at: stamp, last_event_at: null,
  };
  const ok = { connected: true, count: 1, cap: 10, pages: [page] };
  assert.equal(parseStatus(ok).connected, true);
  const second = { ...page, id: "456", instagram: { id: "9", username: "shop_ig" }, status: "reauth_required", last_event_at: stamp };
  assert.deepEqual(parseStatus({ ...ok, count: 2, pages: [page, second] }).pages, [page, second]);
  for (const bad of [
    { connected: false }, { ...empty, extra: 1 }, { ...ok, pages: [{ ...page, status: "gone" }] },
    { ...ok, pages: [{ ...page, id: "abc" }] }, { ...ok, pages: [{ ...page, permissions: ["Bad Name"] }] },
    { ...ok, pages: [{ ...page, connected_at: "yesterday" }] }, { ...ok, token: "SENTINEL" },
    { ...ok, pages: [{ ...page, instagram: { id: "9" } }] }, { ...ok, pages: [{ ...page, token: "SENTINEL" }] },
    { ...ok, count: 2 }, { ...ok, connected: false }, { ...empty, connected: true }, { ...ok, cap: 11 },
    { ...ok, count: 2, pages: [page, page] }, { ...ok, count: 11, pages: Array.from({ length: 11 }, (_, i) => ({ ...page, id: String(i) })) }, null, [], "x",
  ]) assert.throws(() => parseStatus(bad), /meta_connect_shape/);
});

test("pick state: closed shape, optional Instagram keys come as a pair, id must match", () => {
  const page = { page_id: "11", name: "A", missing: [], ig_missing: [] };
  const withIG = { ...page, page_id: "12", ig_id: "99", ig_username: "ig", missing: ["pages_messaging", "task_moderate"], ig_missing: ["instagram_basic"] };
  const state = { state_id: uuid, expires_at: stamp, scopes: ["pages_show_list"], pages: [page, withIG] };
  const parsed = parsePickState(state, uuid);
  assert.equal(parsed.pages.length, 2);
  assert.equal(pickable(parsed.pages[0], false), true);
  assert.equal(pickable(parsed.pages[1], false), false); // missing a permission and a task
  assert.equal(pickable({ ...withIG, missing: [], ig_missing: [] }, true), true);
  assert.equal(pickable({ ...withIG, missing: [], ig_missing: ["instagram_basic"] }, true), false);
  assert.equal(pickable({ ...page }, true), false); // no Instagram account to include
  for (const bad of [
    { ...state, state_id: "22222222-2222-4222-8222-2222222222ef" }, { ...state, pages: [{ ...page, ig_id: "1" }] }, { ...state, pages: [{ ...page, token: "x" }] },
    { ...state, pages: [{ ...page, page_id: "x1" }] }, { ...state, pages: Array.from({ length: 26 }, () => page) }, { ...state, scopes: ["A"] },
  ]) assert.throws(() => parsePickState(bad, uuid), /meta_connect_shape/);
});

test("route grammar: only status/states/pick/disconnect; start and callback are dedicated routes; no query; reads carry no body or key", () => {
  const get = new RegExp(`^(?:${metaConnectRoutes.GET})$`);
  const post = new RegExp(`^(?:${metaConnectRoutes.POST})$`);
  for (const ok of ["meta-connect/status", `meta-connect/states/${uuid}`]) assert.ok(get.test(ok), ok);
  for (const bad of ["meta-connect/start", "meta-connect/callback", "meta-connect/states", `meta-connect/states/${uuid}/x`, "meta-connect/status/x", "META-connect/status"])
    assert.ok(!get.test(bad), bad);
  for (const ok of ["meta-connect/pick", "meta-connect/disconnect"]) assert.ok(post.test(ok), ok);
  for (const bad of ["meta-connect/start", "meta-connect/callback", "meta-connect/status", "meta-connect/pick/x"]) assert.ok(!post.test(bad), bad);
  assert.ok(metaConnectAny.test("meta-connect/pick") && !metaConnectAny.test("meta-connect/start"));
  const req = (method: string, url: string, headers: Record<string, string> = {}, body: unknown = null) => ({ method, url, body, headers: new Headers(headers) });
  assert.equal(validMetaConnectRequest(req("GET", "http://x/api/stores/a/meta-connect/status")), true);
  assert.equal(validMetaConnectRequest(req("GET", "http://x/api/stores/a/meta-connect/status?x=1")), false);
  assert.equal(validMetaConnectRequest(req("GET", "http://x/api/stores/a/meta-connect/status?")), false);
  assert.equal(validMetaConnectRequest(req("GET", "http://x/api/stores/a/meta-connect/status", { "idempotency-key": "abcdefgh" })), false);
  assert.equal(validMetaConnectRequest(req("GET", "http://x/api/stores/a/meta-connect/status", { "content-length": "5" })), false);
  assert.equal(validMetaConnectRequest(req("POST", "http://x/api/stores/a/meta-connect/pick", { "idempotency-key": "abcdefgh" }, {})), true);
});

test("connect cookie, body and callback redirect never carry code/state and only fixed codes", () => {
  assert.match(connectCookie(uuid), /^lc_meta_connect=.+; Path=\/api\/meta\/callback; Secure; HttpOnly; SameSite=Lax; Max-Age=600$/);
  assert.match(clearConnectCookie(), /Max-Age=0/);
  assert.equal(parseConnectBody(JSON.stringify({ store: uuid })), uuid);
  for (const bad of ["{}", JSON.stringify({ store: uuid, x: 1 }), JSON.stringify({ store: "nope" }), "[]", "x"]) assert.equal(parseConnectBody(bad), null);
  assert.equal(storeFromCookie(`a=b; lc_meta_connect=${uuid}`), uuid);
  assert.equal(storeFromCookie(`lc_meta_connect=${uuid}; lc_meta_connect=${uuid}`), null); // two copies
  assert.equal(storeFromCookie("lc_ads_connect=" + uuid), null); // the ads cookie is not ours
  assert.equal(callbackRedirect("zh-TW", uuid, { connect: uuid }), `/zh-TW/settings?store=${uuid}&meta_connect=${uuid}`);
  assert.equal(callbackRedirect("en", uuid, { error: "state_expired" }), `/en/settings?store=${uuid}&meta_error=state_expired`);
  assert.equal(callbackRedirect("en", null, { error: "state_mismatch" }), "/en/settings?meta_error=state_mismatch");
  assert.equal(callbackRedirect("en", uuid, { error: "Bad Code&x=1" }), `/en/settings?store=${uuid}`);
  assert.equal(callbackRedirect("en", uuid, { connect: "../x" }), `/en/settings?store=${uuid}`);
  assert.equal(connectErrorFor(401, {}), null);
  assert.equal(connectErrorFor(409, {}), "state_mismatch");
  assert.equal(connectErrorFor(410, {}), "state_expired");
  assert.equal(connectErrorFor(502, { code: "meta_connect_failed" }), "meta_connect_failed");
  assert.equal(connectErrorFor(500, {}), "unavailable");
});

test("copy: three locales have the same keys and every Go error code has a message", () => {
  const keys = (v: object) => Object.keys(v).sort().join(",");
  assert.equal(keys(metaConnectCopy["zh-TW"]), keys(metaConnectCopy.en));
  assert.equal(keys(metaConnectCopy["zh-CN"]), keys(metaConnectCopy.en));
  for (const code of ["state_mismatch", "state_expired", "state_used", "meta_connect_failed", "denied", "missing_permission", "not_in_pick_list", "page_taken",
    "cap_exceeded", "not_found", "forbidden", "unauthorized", "unavailable"]) {
    for (const locale of ["en", "zh-TW", "zh-CN"] as const) assert.ok(metaConnectCopy[locale].errors[code], `${locale}:${code}`);
  }
});
