import test from "node:test";
import assert from "node:assert/strict";
import { linePrice } from "../lib/line-price.ts";

test("a live (claim-origin) price replaces the catalog price and strikes it through", () => {
  assert.deepEqual(linePrice(300, null, 200), { unitMinor: 200, liveUnitMinor: 200, compareAtMinor: 300 });
});

test("without a live price the catalog price and its compare-at are shown unchanged", () => {
  assert.deepEqual(linePrice(300, null, null), { unitMinor: 300, liveUnitMinor: null, compareAtMinor: null });
  assert.deepEqual(linePrice(300, 400, null), { unitMinor: 300, liveUnitMinor: null, compareAtMinor: 400 });
});

test("a live price equal to or above the catalog price is shown as set (strike hidden by the UI guard)", () => {
  assert.deepEqual(linePrice(200, null, 200), { unitMinor: 200, liveUnitMinor: 200, compareAtMinor: 200 });
  assert.deepEqual(linePrice(200, null, 300), { unitMinor: 300, liveUnitMinor: 300, compareAtMinor: 200 });
});

test("a live price strikes the catalog price, never the variant sale price", () => {
  assert.deepEqual(linePrice(300, 400, 200), { unitMinor: 200, liveUnitMinor: 200, compareAtMinor: 300 });
});
