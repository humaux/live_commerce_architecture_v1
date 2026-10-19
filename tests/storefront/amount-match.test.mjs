// Purpose: self-check of the browser-gate amount matcher (tests/storefront/amount-match.mjs), incl. the CI run 38727832594 flake.
// Depends on: node:test/assert and tests/storefront/amount-match.mjs; no browser.
// Used by: scripts/dev/test-node.sh.
import assert from "node:assert/strict";
import { test } from "node:test";
import { amt, amountMatcher, withoutFixtureIds } from "./amount-match.mjs";

test("amt(96000) matches the amount as a whole token, prefixed or not", () => {
  for (const s of ["NT$960", "NT$960.00", "960", "合計 NT$960", "$960 total", "(960)", "NT$960NT$1,060", "合計NT$960"]) assert.match(s, amt(96000), s);
  assert.match("NT$1,060", amt(106000));
  assert.match("Order totalNT$1,060Quote valid until", amt(106000)); // Playwright joins adjacent elements: a label may follow the amount
});

test("amt(96000) never matches inside fixture ids or other numbers", () => {
  for (const s of ["BRP-40faa960dbfa", "x960", "1,960", "9600", "1960", "0.960", "960,000", "NT$1,060"]) assert.doesNotMatch(s, amt(96000), s);
});

test("amountMatcher keeps thousands separators literal", () => {
  assert.match("NT$1,000.00", amountMatcher("1,000"));
  assert.doesNotMatch("NT$1000", amountMatcher("1,000"));
});

test("negative checks ignore fixture ids that start with the amount (review-93b987562 P2-1)", () => {
  // quotation text as promotions-gate.mjs:207 reads it, with an injected SKU code tag that starts with 960 (BRP-<12 hex>, t04Tag)
  const refused = "Promo gate item 1 A BRP-960abcdef012 NT$400 Promo gate item 1 B BRP-40faa960dbfa NT$600 Shipping NT$60 Total NT$1,060";
  assert.doesNotMatch(withoutFixtureIds(refused), amt(96000));
  assert.match(withoutFixtureIds(refused), amt(106000)); // stripping removes ids only, never amounts
  assert.match(withoutFixtureIds("BRP-960abcdef012 Total NT$960"), amt(96000)); // a real 960 still fails the check
});
