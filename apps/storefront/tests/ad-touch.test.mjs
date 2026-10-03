// AT1 pure cookie boundary tests; complete AT1 also requires the PG/Begin and real-browser gates.
import test from "node:test";
import assert from "node:assert/strict";
import { AD_TOUCH_TTL, captureAdTouch, readAdTouch } from "../lib/ad-touch.ts";

const now = 1_800_000_000_000;
const origin = "https://shop.example";
const key = Buffer.alloc(32, 31);
const a = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const b = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const url = (id = a, suffix = "") => new URL(`${origin}/zh-TW/products/x?lc_ad=${id}&fbclid=synthetic-click_01${suffix}`);
const header = (capture) => capture.cookies.map(c => c.split(";")[0]).join("; ");
const first = () => captureAdTouch(url(), "", origin, key, now, "123456");

test("AT1 R5: browser identity survives expired touch and rolls for ninety days", () => {
  const old = first();
  const next = captureAdTouch(url(b), header(old), origin, key, now + AD_TOUCH_TTL * 1000 + 1, "9999");
  assert.equal(next.touch.fbp, old.touch.fbp);
  assert.ok(next.cookies.some(c => c.startsWith("lc_fbp=") && c.includes("Max-Age=7776000")));
});

test("AT1 R5: missing ad parameters create no cookies; fbclid alone creates no touch", () => {
  assert.equal(captureAdTouch(new URL(origin), "", origin, key, now), null);
  const got = captureAdTouch(new URL(`${origin}/?fbclid=click-only`), "", origin, key, now);
  assert.ok(got);
  assert.equal(got.touch, null);
  assert.equal(got.cookies.some(c => c.startsWith("lc_ad_touch=")), false);
});

test("AT1 R5: lc_ad alone updates attribution and does not replace fbc", () => {
  const old = first();
  const next = captureAdTouch(new URL(`${origin}/?lc_ad=${b}`), header(old), origin, key, now + 1000);
  assert.equal(next.touch.draft_id, b);
  assert.equal(next.touch.fbc, old.touch.fbc);
  assert.equal(next.cookies.some(c => c.startsWith("lc_fbc=")), false);
});

test("AT1: host-only first-party cookies contain no PII and read back exactly", () => {
  const got = first();
  assert.ok(got);
  assert.equal(got.touch.fbc, `fb.1.${now}.synthetic-click_01`);
  assert.equal(got.touch.fbp, `fb.1.${now}.123456`);
  assert.deepEqual(readAdTouch(header(got), origin, key, now), got.touch);
  assert.deepEqual(Object.keys(got.touch).sort(), ["clicked_at", "draft_id", "fbc", "fbp"]);
  for (const c of got.cookies) {
    assert.match(c, /; Path=\/; Max-Age=(604800|7776000); HttpOnly; Secure; SameSite=Lax$/);
    assert.doesNotMatch(c, /Domain=/i);
  }
});

test("AT1: a later valid click replaces the draft but preserves the browser identifier", () => {
  const old = first();
  const next = captureAdTouch(url(b), header(old), origin, key, now + 1000, "9999");
  assert.equal(next.touch.draft_id, b);
  assert.equal(next.touch.clicked_at, new Date(now + 1000).toISOString());
  assert.equal(next.touch.fbp, old.touch.fbp);
  assert.notEqual(next.touch.fbc, old.touch.fbc);
});

test("AT1: malformed and ambiguous links do not overwrite a valid prior touch", () => {
  const old = first();
  for (const bad of [url("bad"), url(a.toUpperCase()), url(a, `&lc_ad=${a}`), url(a, "&fbclid=second"),
    new URL(`${origin}/?lc_ad=${a}&fbclid=`),
    new URL(`${origin}/?lc_ad=${a}&fbclid=${"a".repeat(501)}`), new URL(`${origin}/?lc_ad=${a}&fbclid=%3Cscript%3E`)]) {
    assert.equal(captureAdTouch(bad, header(old), origin, key, now + 1), null);
    assert.deepEqual(readAdTouch(header(old), origin, key, now + 1), old.touch);
  }
});

test("AT1: host and key binding reject copied cookies", () => {
  const old = first();
  assert.equal(readAdTouch(header(old), "https://other.example", key, now), null);
  assert.equal(readAdTouch(header(old), origin, Buffer.alloc(32, 32), now), null);
  const next = captureAdTouch(url(b), header(old), "https://other.example", key, now + 1, "7788");
  assert.notEqual(next.touch.fbp, old.touch.fbp);
});

test("AT1: duplicates and tampering invalidate the entire envelope", () => {
  const old = first();
  for (const c of old.cookies) assert.equal(readAdTouch(`${header(old)}; ${c.split(";")[0]}`, origin, key, now), null);
  const original = header(old);
  // IDs have independent signed envelopes; a corrupt browser ID is never reused on a new landing.
  const tampered = original.replace("lc_fbp=", "lc_fbp=a");
  assert.notEqual(captureAdTouch(url(b), tampered, origin, key, now + 1, "9999").touch.fbp, old.touch.fbp);
  assert.equal(readAdTouch(original.replace("lc_ad_touch=", "lc_ad_touch=a"), origin, key, now), null);
});

test("AT1: exactly seven days is valid, the next millisecond and future clicks are not", () => {
  const old = first();
  assert.deepEqual(readAdTouch(header(old), origin, key, now + AD_TOUCH_TTL * 1000), old.touch);
  assert.equal(readAdTouch(header(old), origin, key, now + AD_TOUCH_TTL * 1000 + 1), null);
  assert.equal(readAdTouch(header(old), origin, key, now - 1), null);
  for (const time of [0, -1, NaN, Infinity, 0.5, Number.MAX_SAFE_INTEGER])
    assert.equal(captureAdTouch(url(), "", origin, key, time, "1"), null);
});
