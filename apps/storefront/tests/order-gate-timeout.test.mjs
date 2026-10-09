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
    expect: value => { if (value?.setupReady) return {toBeEnabled: async () => {}}; throw new Error("HEAD arrived: continue real UI assertions"); },
  });
  return { run, timers, cleared };
}
const page = { locator: () => ({setupReady:true}), context: () => ({cookies: async () => [{name:"synthetic",value:"canary",httpOnly:true,secure:true}]}), url: () => "https://buyer.example/zh-TW/checkout", getByTestId: () => ({}) };

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


function warmupProbe(promise) {
  const start=source.indexOf("async function waitBo01Warmup(");
  const end=source.indexOf("async function quotePage(",start);
  assert.ok(start>=0&&end>start,"actual bounded warmup helper must exist");
  const timers=[],cleared=[];
  const run=runInNewContext(`(${source.slice(start,end).trim()})`,{
    setTimeout:(callback,delay)=>{const timer={callback,delay};timers.push(timer);return timer;},
    clearTimeout:timer=>cleared.push(timer),
  });
  return {run:phase=>run(promise,phase,page),timers,cleared};
}
for(const phase of ["request interception","upstream completion"]) {
  test(`warmup ${phase} has a precise local deadline and clears its timer`,async()=>{
    const p=warmupProbe(new Promise(()=>{}));
    const result=p.run(phase);const rejected=assert.rejects(result,new RegExp(`BO01 warmup ${phase}.*10000ms.*GET /api/buyer/destination.*zh-TW/checkout`));
    await new Promise(setImmediate);assert.equal(p.timers.length,1);assert.equal(p.timers[0].delay,10000);
    p.timers[0].callback();await rejected;assert.deepEqual(p.cleared,p.timers);
  });
}
test("warmup timer is cleared on upstream success and upstream rejection",async()=>{
  for(const reject of [false,true]){
    const p=warmupProbe(reject?Promise.reject(new Error("relay failed")):Promise.resolve(200));
    try {assert.equal(await p.run("upstream completion"),200);} catch(error){assert.match(error.message,/relay failed/);}
    assert.equal(p.timers.length,1);assert.deepEqual(p.cleared,p.timers);
  }
});
test("EARLY_HEAD route continues a missing-Referer request without resolving the English barrier",async()=>{
  const begin=source.indexOf("async function quotePage(");const end=source.indexOf("async function fill(",begin);
  const callbacks=[];let continued=0,resolved=0;
  const p={route:async(_pattern,callback)=>callbacks.push(callback)};
  const run=runInNewContext(`(${source.slice(begin,end).trim()})`,{
    process:{env:{LC_BO01_EARLY_HEAD:"1"}},origin:"https://buyer.example",URL,
    bo01Warmups:new Map(),deferred:()=>({promise:new Promise(()=>{}),resolve:()=>resolved++}),
    reachCheckout:async()=>{throw new Error("route registered");},
  });
  await assert.rejects(run({},false,p,true),/route registered/);assert.equal(callbacks.length,1);
  await callbacks[0]({request:()=>({method:()=>"GET",headers:()=>({})}),continue:async()=>continued++});
  assert.equal(continued,1);assert.equal(resolved,0,"unrelated read must leave the warmup barrier unclaimed");
});
