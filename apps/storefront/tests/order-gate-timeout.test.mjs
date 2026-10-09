// Purpose: execute the actual order-gate locale probe's pending-head path with a deterministic clock.
// Depends on: Node vm and the real stableLocaleTarget source; only browser/network observations are controlled.
// Used by: test-node.sh storefront wildcard; the real UI success path runs in --browser-order.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";

const source = readFileSync(new URL("../../../tests/storefront/order-gate.mjs", import.meta.url), "utf8");
const start = source.indexOf("async function stableLocaleTarget(");
const end = source.indexOf("async function capture(", start);
assert.ok(start >= 0 && end > start, "actual locale probe is present");
function probe(promise) {
  const timers = [], cleared = [];
  const run = runInNewContext(`(${source.slice(start, end).trim()})`, {
    arm: () => ({ result: { promise }, receivedSourcePath: "/zh-TW/checkout", ownerMatched: true }), switchLocale: async () => {},
    assert, origin: "https://buyer.example", process: {env:{}}, bo01Warmups: new Map(),
    setTimeout: (callback, delay) => { const timer = { callback, delay }; timers.push(timer); return timer; },
    clearTimeout: timer => cleared.push(timer),
    expect: () => { throw new Error("HEAD arrived: continue real UI assertions"); },
  });
  return { run, timers, cleared };
}
const page = { context: () => ({cookies: async () => [{name:"synthetic",value:"canary",httpOnly:true,secure:true}]}), url: () => "https://buyer.example/zh-TW/checkout", getByTestId: () => ({}) };

test("locale probe bounds a missing destination head with a route/device diagnostic", async () => {
  const p = probe(new Promise(() => {}));
  const result = p.run(page, true);
  const rejected = assert.rejects(result, /BO01 mobile.*10000ms.*GET \/api\/buyer\/destination.*zh-TW\/checkout/);
  // A full turn drains cross-VM promise adoption before advancing the controlled deadline.
  await new Promise(setImmediate);
  assert.equal(p.timers.length, 1, "missing head must arm a local deadline");
  assert.equal(p.timers[0].delay, 10000);
  p.timers[0].callback();
  await rejected;
  assert.deepEqual(p.cleared, p.timers, "deadline is cleared on failure");
});

test("locale probe clears its deadline when the real head promise completes", async () => {
  const p = probe(Promise.resolve(200));
  await assert.rejects(p.run(page), /HEAD arrived: continue real UI assertions/);
  assert.equal(p.timers.length, 1);
  assert.deepEqual(p.cleared, p.timers, "successful head does not leave a timer behind");
});
