// MDEF-1 / ruling F1: the Studio › Claims bundle parser accepts the platforms Go emits
// (manual with a label; facebook/instagram with an empty label, meta-claims-intake-v1 §4/§8)
// and still rejects unknown platforms and label/platform mismatches.
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  currencyDigits, parseBundlePage, parseImportResult, parseLibrary, parseLibrarySaved, parseOffer, parsePriceMinor, priceInputText,
} from "../../apps/admin/lib/claims-model.ts";
import { claimsCopy } from "../../apps/admin/lib/claims-copy.ts";

const bundle = (platform: string, label: string) => ({
  bundle_id: "11111111-1111-4111-8111-111111111111", ref: "11111111", platform, label, bound: false, version: 1,
  link: { state: "NONE", generation: 0, expires_at: null }, lines: [],
  created_at: "2026-09-29T00:00:00Z", updated_at: "2026-09-29T00:00:00Z",
});
const page = (b: unknown) => ({ items: [b], next_cursor: "" });

test("bundle parser accepts manual (labelled) and Meta (unlabelled) bundles", () => {
  assert.equal(parseBundlePage(page(bundle("manual", "Amy"))).items[0].platform, "manual");
  assert.equal(parseBundlePage(page(bundle("facebook", ""))).items[0].platform, "facebook");
  assert.equal(parseBundlePage(page(bundle("instagram", ""))).items[0].platform, "instagram");
});

test("bundle parser rejects unknown platforms and label/platform mismatches", () => {
  for (const b of [bundle("tiktok", ""), bundle("manual", ""), bundle("facebook", "Amy"), bundle("", "")])
    assert.throws(() => parseBundlePage(page(b)), `${b.platform}/${b.label}`);
});

// Live tools (R4): the offer wire shape gained sku_price_minor/currency/live_price_minor; parsers stay closed.
const scene = "22222222-2222-4222-8222-222222222222";
const offer = (patch: Record<string, unknown> = {}) => ({
  offer_id: "33333333-3333-4333-8333-333333333333", session_id: scene, keyword: "A1", sku_id: "44444444-4444-4444-8444-444444444444",
  sku_code: "SKU-1", product_name: "Mug", max_quantity_per_claim: 3, active: true, version: 1,
  activated_at: "2026-10-01T00:00:00Z", updated_at: "2026-10-01T00:00:00Z", sku_price_minor: 1000, currency: "TWD", live_price_minor: null, ...patch,
});
test("offer parser carries the live price and rejects every out-of-range or missing field", () => {
  assert.equal(parseOffer(offer(), scene).live_price_minor, null);
  assert.equal(parseOffer(offer({ live_price_minor: 700 }), scene).live_price_minor, 700);
  for (const bad of [{ live_price_minor: 0 }, { live_price_minor: -1 }, { live_price_minor: 1.5 }, { live_price_minor: 1e12 + 1 }, { live_price_minor: "7" },
    { currency: "twd" }, { sku_price_minor: -1 }, { extra: 1 }])
    assert.throws(() => parseOffer(offer(bad), scene), undefined, JSON.stringify(bad));
  const { live_price_minor: _gone, ...missing } = offer();
  assert.throws(() => parseOffer(missing, scene));
});
test("library and import parsers are closed; removal receipt is the only version-0 shape", () => {
  const entry = { sku_id: "44444444-4444-4444-8444-444444444444", sku_code: "SKU-1", product_name: "Mug", keyword: "A1", version: 1, sku_price_minor: 1000, currency: "TWD", updated_at: "2026-10-01T00:00:00Z" };
  assert.equal(parseLibrary({ entries: [entry] })[0].keyword, "A1");
  assert.throws(() => parseLibrary({ entries: [{ ...entry, version: 0 }] }));
  assert.throws(() => parseLibrary({ entries: [{ ...entry, keyword: "" }] }));
  assert.throws(() => parseLibrary({ entries: [], extra: 1 }));
  const removed = { ...entry, sku_code: "", product_name: "", keyword: "", version: 0, sku_price_minor: 0, currency: "", updated_at: "0001-01-01T00:00:00Z" };
  assert.equal(parseLibrarySaved(removed).version, 0);
  assert.throws(() => parseLibrarySaved({ ...removed, keyword: "A1" }));
  const result = { created: [offer()], conflicts: [{ keyword: "B2", sku_id: entry.sku_id, reason: "sku_taken" }] };
  assert.equal(parseImportResult(result, scene).conflicts[0].reason, "sku_taken");
  assert.throws(() => parseImportResult({ ...result, conflicts: [{ keyword: "B2", sku_id: entry.sku_id, reason: "other" }] }, scene));
  assert.throws(() => parseImportResult({ created: [offer({ session_id: "x" })], conflicts: [] }, scene));
});
test("price input: exact decimal minor units, never float, bounded to the currency's places", () => {
  assert.equal(parsePriceMinor("12.5", 2), 1250);
  assert.equal(parsePriceMinor("12", 2), 1200);
  assert.equal(parsePriceMinor("0.01", 2), 1);
  assert.equal(parsePriceMinor("1000", 0), 1000);
  assert.equal(parsePriceMinor("19.99", 2), 1999);      // 19.99*100 would be 1998.9999999999998 in floats
  for (const bad of ["", "0", "0.00", "-1", "1.234", "1.5", "1e3", "12,5", " ", "abc", "10000000000000", "99999999999.99"])
    assert.equal(parsePriceMinor(bad, bad === "1.5" ? 0 : 2), null, bad);
  assert.equal(priceInputText(1250, 2), "12.50");
  assert.equal(priceInputText(5, 2), "0.05");
  assert.equal(priceInputText(1000, 0), "1000");
  assert.equal(currencyDigits("JPY"), 0);
  assert.equal(currencyDigits("TWD"), 2);
});
test("live-tools copy exists in all three locales with the same conflict reasons", () => {
  const reasons = Object.keys(claimsCopy.en.live.conflictReason).sort();
  for (const locale of ["en", "zh-CN", "zh-TW"] as const) {
    assert.deepEqual(Object.keys(claimsCopy[locale].live.conflictReason).sort(), reasons, locale);
    assert.ok(claimsCopy[locale].live.higherWarning("a", "b").length > 0 && claimsCopy[locale].live.importDone(1, 2).length > 0, locale);
  }
});
