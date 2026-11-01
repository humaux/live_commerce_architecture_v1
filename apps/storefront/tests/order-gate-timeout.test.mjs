// Purpose: execute the actual order-gate locale probe's pending-head path with a deterministic clock.
// Depends on: Node vm and real order-gate/shop-helpers source; only browser/network observations are controlled.
// Used by: test-node.sh storefront wildcard; the real UI success path runs in --browser-order.
// Invariants: injected timing models exercise real gate statements; only CI browsers prove engine behaviour.
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
    if(reject)await assert.rejects(p.run("upstream completion"),/relay failed/);
    else assert.equal(await p.run("upstream completion"),200);
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

// MODEL_ONLY: reproduce a focus-induced compositor scroll between pointer targeting
// and delivery. The failed BO03 artifact left Choose delivery focused while the old
// journal survived. A settled focus before the pointer action removes that window.
// These probes execute the actual two gate call sites, not copies of their logic;
// the required three WebKit CI executions remain the engine acceptance gate.
function orderControlRace(testID) {
  let focused = null;
  const clicks = [], events = [];
  const target = {
    async focus() { focused = testID; events.push("focus-settled"); },
    async click() {
      // Inject the observed wrong-target delivery when focus scroll is outstanding.
      const delivered = focused === testID ? testID : "sticky-purchase-action";
      focused = delivered;
      clicks.push(delivered);
      events.push("click");
    },
  };
  const page = { getByTestId(id) { assert.equal(id,testID); return target; } };
  const check = value => {
    assert.equal(value,target);
    return {
      async toBeEnabled() {},
      async toBeFocused() { assert.equal(focused,testID); },
    };
  };
  return { page, check, clicks, events };
}

test("BO03 quote-less address reconfirmation reaches its control during a focus-scroll race",async()=>{
  const begin=source.indexOf('await expect(noQuoteTab.getByTestId("confirm-address")).toBeEnabled();');
  const end=source.indexOf("await expect.poll(()=>stored(noQuoteTab)).toBeNull();",begin);
  assert.ok(begin>=0&&end>begin,"actual quote-less reconfirmation must remain covered");
  const p=orderControlRace("confirm-address");
  // Execute the actual imported helpers too: a stub would conceal changes to the
  // pointer path or the post-confirmation details disclosure.
  const helpers=readFileSync(new URL("../../../tests/storefront/shop-helpers.mjs",import.meta.url),"utf8");
  const helperStart=helpers.indexOf("export async function confirmCheckoutAddress(");
  assert.ok(helperStart>=0&&helpers.indexOf("export async function openCheckoutDetails(",helperStart)>helperStart);
  const main={},details={async isVisible(){p.events.push("details-checked");return false;}};
  const noQuoteTab={
    locator(selector){assert.equal(selector,".purchase-main");return main;},
    getByTestId(id){return id==="checkout-details-toggle"?details:p.page.getByTestId(id);},
  };
  const expect=value=>value===main?{not:{async toHaveAttribute(name,value){
    assert.equal(name,"aria-busy");assert.equal(value,"true");p.events.push("idle-checked");
  }}}:p.check(value);
  const run=runInNewContext(`(async()=>{${helpers.slice(helperStart).replace(/^export /gm,"")}\n${source.slice(begin,end)}})`,{noQuoteTab,expect});
  await run();
  assert.deepEqual(p.clicks,["confirm-address"],"one real pointer action must reach address confirmation, not the sticky bar");
  assert.deepEqual(p.events,["focus-settled","click","idle-checked","idle-checked","details-checked"],"real confirmation helper waits for idle before inspecting the quote-less disclosure");
});

test("BH locale history activation settles focus and arms the real read before its single click",async()=>{
  const begin=source.indexOf("await switchLocale(a,locale);const localeHistory=historyRead(a);");
  const end=source.indexOf('await expect(a.getByTestId("order-history").getByRole("heading"',begin);
  assert.ok(begin>=0&&end>begin,"actual locale-history activation must remain covered");
  const p=orderControlRace("toggle-order-history");
  const run=runInNewContext(`(async()=>{${source.slice(begin,end)}})`,{
    a:p.page,locale:"zh-CN",expect:p.check,
    switchLocale:async()=>{p.events.push("navigation");},
    historyRead:()=>{p.events.push("read-armed");return Promise.resolve();},
  });
  await run();
  assert.deepEqual(p.clicks,["toggle-order-history"],"one real pointer action must open history after locale navigation");
  assert.ok(p.events.indexOf("read-armed")<p.events.indexOf("click"),"the history response observer precedes the triggering action");
});
