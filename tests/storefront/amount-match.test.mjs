// Purpose: self-check of the browser-gate amount matcher (tests/storefront/amount-match.mjs), incl. the CI run 38727832594 flake.
// Depends on: node:test/assert and tests/storefront/amount-match.mjs; no browser.
// Used by: scripts/dev/test-node.sh.
import assert from "node:assert/strict";
import { test } from "node:test";
import { amt, amountMatcher } from "./amount-match.mjs";

test("amt(96000) matches the amount as a whole token, prefixed or not", () => {
  for (const s of ["NT$960", "NT$960.00", "960", "合計 NT$960", "$960 total", "(960)"]) assert.match(s, amt(96000), s);
  assert.match("NT$1,060", amt(106000));
});

test("amt(96000) never matches inside fixture ids or other numbers", () => {
  for (const s of ["BRP-40faa960dbfa", "x960", "960a", "1,960", "9600", "1960", "0.960", "960,000", "NT$1,060"]) assert.doesNotMatch(s, amt(96000), s);
});

test("amountMatcher keeps thousands separators literal", () => {
  assert.match("NT$1,000.00", amountMatcher("1,000"));
  assert.doesNotMatch("NT$1000", amountMatcher("1,000"));
});
