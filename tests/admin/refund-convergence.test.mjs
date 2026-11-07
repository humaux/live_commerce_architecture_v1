// Purpose: execute actual OrderRefunds effects against controlled asynchronous reads, including stale and scope races.
// Depends on: Node vm/test, TypeScript's existing in-memory compiler and synthetic React hook storage.
// Used by: canonical Node gate; MOCK component evidence complements the frozen real-click RF11 regression.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';
import ts from 'typescript-api';
const source = ts.transpileModule(readFileSync(new URL('../../apps/admin/components/OrderRefunds.tsx', import.meta.url), 'utf8'), {
  compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
}).outputText;
const settle = () => new Promise(resolve => setImmediate(resolve));
const snapshot = (refunded_minor, pending_minor = 0) => ({ captured_minor: 2500, refunded_minor, pending_minor,
  refundable_minor: 2500-refunded_minor-pending_minor, items: [], currency:'TWD' });
function host(onChanged = async () => true) {
  const slots = [], effects = [], reads = []; let cursor = 0, updates = 0;
  const react = {
    useState(initial) { const i=cursor++; slots[i] ??= {value:initial}; return [slots[i].value, value => {
      updates++; slots[i].value = typeof value==='function' ? value(slots[i].value) : value;
    }]; },
    useRef(initial) { return slots[cursor++] ??= {current:initial}; },
    useEffect(effect,deps) { const i=cursor++, old=slots[i]; if (!old || deps.some((v,j)=>!Object.is(v,old.deps[j]))) {
      slots[i]={deps,cleanup:old?.cleanup}; effects.push(()=>{slots[i].cleanup?.();slots[i].cleanup=effect();});
    } },
  };
  const jsx=(type,props)=>({type,props}), module={exports:{}};
  runInNewContext(source,{module,exports:module.exports,AbortController,Intl,crypto,require(path) {
    if(path==='react') return react;
    if(path==='react/jsx-runtime') return {jsx,jsxs:jsx};
    if(path==='@/lib/orders-client') return {readRefunds(...args){return new Promise((resolve,reject)=>reads.push({args,resolve,reject}));},
      postRefund(){throw Error('unexpected POST');},refreshRefund(){throw Error('unexpected POST');}};
    if(path==='@/lib/orders-model') return {refundReasons:[],wholeOnly:()=>false,amountToMinor:()=>null,refundAmountOK:()=>false};
    if(path==='@/lib/client') return {money:(_,__,v)=>String(v)};
    if(path.endsWith('.css')) return {};
    throw Error(path);
  }});
  const props={store:'store-a',boundary:'session-a',detail:{order_id:'order-a',currency:'TWD',updated_at:'v1',refunded_minor:1000,refund_pending_minor:0},locale:'en',c:{refundTitle:'Refund'},canRefund:false,onChanged};
  return {props,reads,get updates(){return updates;},render(p=props){cursor=0;module.exports.OrderRefunds(p);for(const e of effects.splice(0))e();},
    unmount(){for(const s of slots)s?.cleanup?.();}};
}
test('one GET notification per differing totals; timestamp/rerender cannot loop, late reversal can converge',async()=>{
  let calls=0;const h=host(async()=>{calls++;return true;});h.render();h.reads[0].resolve(snapshot(2500));await settle();assert.equal(calls,1);
  for(let n=2;n<7;n++){h.render({...h.props,detail:{...h.props.detail,updated_at:`v${n}`}});h.reads.at(-1).resolve(snapshot(2500));await settle();}
  assert.equal(calls,1);
  h.render({...h.props,detail:{...h.props.detail,updated_at:'reversal',refunded_minor:2500}});h.reads.at(-1).resolve(snapshot(1000));await settle();assert.equal(calls,2);
});
test('aborted older read is not admitted and cannot notify',async()=>{
  let calls=0;const h=host(async()=>{calls++;return true;});h.render();h.render({...h.props,detail:{...h.props.detail,updated_at:'v2'}});
  assert.ok(h.reads[0].args[3].aborted);const before=h.updates;h.reads[0].resolve(snapshot(2500));await settle();assert.equal(h.updates,before);assert.equal(calls,0);
  h.reads[1].resolve(snapshot(1000));await settle();assert.equal(calls,0);
});
test('old store/order/session completions never notify the current scope',async()=>{
  for(const change of [{store:'store-b'},{boundary:'session-b'},{detail:{order_id:'order-b',currency:'TWD',updated_at:'v1',refunded_minor:1000,refund_pending_minor:0}}]){
    let calls=0;const h=host(async()=>{calls++;return true;});h.render();h.render({...h.props,...change});const before=h.updates;
    h.reads[0].resolve(snapshot(2500));await settle();assert.equal(h.updates,before);assert.equal(calls,0);
    h.reads[1].resolve(snapshot(2500));await settle();assert.equal(calls,1);
  }
});
test('notifications serialize and queued old scope/unmounted work is discarded',async()=>{
  let calls=0,release;const h=host(()=>{calls++;return new Promise(resolve=>{release=resolve;});});h.render();h.reads[0].resolve(snapshot(1500));await settle();assert.equal(calls,1);
  h.render({...h.props,detail:{...h.props.detail,updated_at:'v2'}});h.reads[1].resolve(snapshot(2500));await settle();assert.equal(calls,1);
  release(true);await settle();assert.equal(calls,2);release(true);await settle();
  h.render({...h.props,detail:{...h.props.detail,updated_at:'v3'}});h.reads[2].resolve(snapshot(1800));h.unmount();await settle();assert.equal(calls,2);
});
test('a matching newer snapshot permits a later reversal to previously seen totals',async()=>{
  let calls=0;const h=host(async()=>{calls++;return true;});
  h.render({...h.props,detail:{...h.props.detail,refunded_minor:0}});h.reads[0].resolve(snapshot(1000));await settle();assert.equal(calls,1);
  h.render({...h.props,detail:{...h.props.detail,updated_at:'matched',refunded_minor:2500}});h.reads[1].resolve(snapshot(2500));await settle();assert.equal(calls,1);
  h.render({...h.props,detail:{...h.props.detail,updated_at:'late-failed',refunded_minor:2500}});h.reads[2].resolve(snapshot(1000));await settle();assert.equal(calls,2);
});
