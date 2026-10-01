// promotions: strict parser, form -> body builders, Taipei time conversion and the BFF route grammar of the admin discount-code page
// (apps/admin/lib/promotions-{model,request}.ts, contracts/storefront-v2.md §F). Pure logic only; Go/SQL stay the rule authority.
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  createBody, emptyForm, formFrom, instantToTaipei, parsePromotion, parsePromotions, taipeiToInstant, toggleBody, updateBody,
} from "../../apps/admin/lib/promotions-model.ts";
import { promotionsRoute } from "../../apps/admin/lib/promotions-request.ts";
import { promotionsCopy } from "../../apps/admin/lib/promotions-copy.ts";

const ID = "11111111-1111-4111-8111-111111111111";
const row = (over: Record<string, unknown> = {}) => ({
  id: ID, code: "SAVE10", kind: "percent", percent: 10, fixed_minor: null, min_subtotal_minor: 0, starts_at: null, ends_at: null,
  total_limit: null, per_buyer_limit: null, status: "active", version: 1, used: 0, created_at: "2026-10-01T04:00:00.123456+00:00", ...over,
});

test("PRM01 parse: exact keys, kind/value pairing and ranges; duplicates and unknown keys are refused", () => {
  assert.equal(parsePromotion(row()).code, "SAVE10");
  assert.equal(parsePromotion(row({ kind: "fixed", percent: null, fixed_minor: 5000 })).fixed_minor, 5000);
  for (const [name, bad] of Object.entries({
    "extra key": { ...row(), extra: 1 },
    "percent 91": row({ percent: 91 }),
    "percent 0": row({ percent: 0 }),
    "kind without its value": row({ percent: null }),
    "fixed with a percent": row({ kind: "fixed", percent: 10, fixed_minor: 5 }),
    "lower-case code": row({ code: "save10" }),
    "short code": row({ code: "AB" }),
    "bad status": row({ status: "deleted" }),
    "negative used": row({ used: -1 }),
    "version 0": row({ version: 0 }),
    "bad time": row({ ends_at: "tomorrow" }),
    "string number": row({ total_limit: "5" }),
  })) assert.throws(() => parsePromotion(bad), /unavailable/, name);
  assert.equal(parsePromotions({ promotions: [row(), row({ id: "22222222-2222-4222-8222-222222222222", code: "OTHER" })] }).length, 2);
  assert.throws(() => parsePromotions({ promotions: [row(), row()] }), /unavailable/);
  assert.throws(() => parsePromotions({ promotions: [], more: 1 }), /unavailable/);
  assert.throws(() => parsePromotions([row()]), /unavailable/);
});

test("PRM02 Taipei wall time: +08:00 on the wire, instant shown back in Taipei, impossible dates refused", () => {
  assert.equal(taipeiToInstant("2026-10-01T09:30"), "2026-10-01T09:30:00+08:00");
  assert.equal(taipeiToInstant(""), null);
  assert.equal(taipeiToInstant("2026-13-40T99:99"), undefined);
  assert.equal(taipeiToInstant("soon"), undefined);
  // 01:30 UTC is 09:30 in Taipei; the round trip is the identity on a wall time.
  assert.equal(instantToTaipei("2026-10-01T01:30:00.000000+00:00"), "2026-10-01T09:30");
  assert.equal(instantToTaipei(taipeiToInstant("2026-12-31T23:59") as string), "2026-12-31T23:59");
  assert.equal(instantToTaipei(null), "");
});

test("PRM03 create body: normalised code, major units to minor units, optional fields null, whole body validated", () => {
  const form = { ...emptyForm, code: " save10 ", value: "10", minSubtotal: "500", startsAt: "2026-10-01T00:00", endsAt: "2026-10-31T23:59", totalLimit: "100", perBuyerLimit: "1" };
  assert.deepEqual(createBody(form, "TWD"), {
    code: "SAVE10", kind: "percent", percent: 10, fixed_minor: null, min_subtotal_minor: 50000, starts_at: "2026-10-01T00:00:00+08:00",
    ends_at: "2026-10-31T23:59:00+08:00", total_limit: 100, per_buyer_limit: 1, status: "active",
  });
  const fixed = createBody({ ...emptyForm, code: "TEN-OFF", kind: "fixed", value: "100" }, "TWD");
  assert.equal(fixed?.fixed_minor, 10000); // TWD has two minor digits (x100 convention)
  assert.equal(fixed?.percent, null);
  for (const [name, bad] of Object.entries({
    "empty value": { ...emptyForm, code: "ABC" },
    "percent over 90": { ...emptyForm, code: "ABC", value: "91" },
    "percent decimal": { ...emptyForm, code: "ABC", value: "10.5" },
    "fixed zero": { ...emptyForm, code: "ABC", kind: "fixed" as const, value: "0" },
    "bad code": { ...emptyForm, code: "A B", value: "10" },
    "short code": { ...emptyForm, code: "AB", value: "10" },
    "end before start": { ...emptyForm, code: "ABC", value: "10", startsAt: "2026-10-02T00:00", endsAt: "2026-10-01T00:00" },
    "zero limit": { ...emptyForm, code: "ABC", value: "10", totalLimit: "0" },
    "negative minimum": { ...emptyForm, code: "ABC", value: "10", minSubtotal: "-5" },
  })) assert.equal(createBody(bad, "TWD"), undefined, name);
});

test("PRM04 update + toggle: version CAS key, form round trip, pause keeps every other field", () => {
  const p = parsePromotion(row({ kind: "fixed", percent: null, fixed_minor: 25050, min_subtotal_minor: 100000, starts_at: "2026-10-01T01:00:00+00:00", total_limit: 5, version: 3 }));
  const form = formFrom(p, "TWD");
  assert.equal(form.value, "250.50");
  assert.equal(form.minSubtotal, "1000");
  assert.equal(form.startsAt, "2026-10-01T09:00");
  const body = updateBody(form, "TWD", p.version);
  assert.equal(body?.expected_version, 3);
  assert.equal(body?.fixed_minor, 25050);
  assert.equal(body?.starts_at, "2026-10-01T09:00:00+08:00");
  assert.equal(updateBody(form, "TWD", 0), undefined);
  const t = toggleBody(p);
  assert.equal(t.status, "paused");
  assert.equal(t.expected_version, 3);
  assert.equal(t.fixed_minor, 25050);
  assert.equal(toggleBody(parsePromotion(row({ status: "paused" }))).status, "active");
});

test("PRM05 BFF grammar: exactly list, create and update; nothing generic", () => {
  assert.equal(promotionsRoute("GET", "promotions"), "get");
  assert.equal(promotionsRoute("POST", "promotions"), "command");
  assert.equal(promotionsRoute("POST", `promotions/${ID}`), "command");
  for (const [m, p] of [["PUT", "promotions"], ["DELETE", `promotions/${ID}`], ["GET", `promotions/${ID}`], ["POST", "promotions/x"], ["POST", `promotions/${ID}/extra`],
    ["POST", "promotions/"], ["GET", "promotions?x=1"], ["POST", "promotions/AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"], ["GET", "../promotions"]])
    assert.equal(promotionsRoute(m, p), null, `${m} ${p}`);
});

test("PRM06 copy: three locales carry the same keys and every refusal code the Go routes can answer", () => {
  const en = promotionsCopy.en;
  for (const locale of ["zh-CN", "zh-TW"] as const) {
    assert.deepEqual(Object.keys(promotionsCopy[locale]).sort(), Object.keys(en).sort(), locale);
    assert.deepEqual(Object.keys(promotionsCopy[locale].errors).sort(), Object.keys(en.errors).sort(), locale);
  }
  for (const code of ["promo_exists", "invalid_promotion", "version_changed", "idempotency_conflict", "forbidden", "unauthorized", "retry_later", "default"])
    assert.ok(en.errors[code], code);
});
