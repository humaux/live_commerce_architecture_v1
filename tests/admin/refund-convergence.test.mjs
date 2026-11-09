// Purpose: execute actual OrderRefunds effects against controlled asynchronous reads, including stale and scope races.
// Depends on: Node vm/test, TypeScript's existing in-memory compiler and synthetic React hook storage.
// Used by: canonical Node gate; MOCK component evidence complements the frozen real-click RF11 regression.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';
import ts from 'typescript-api';
import * as ordersModel from '../../apps/admin/lib/orders-model.ts';
import { ordersCopy } from '../../apps/admin/lib/orders-copy.ts';
const source = ts.transpileModule(readFileSync(new URL('../../apps/admin/components/OrderRefunds.tsx', import.meta.url), 'utf8'), {
  compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
}).outputText;
const settle = () => new Promise(resolve => setImmediate(resolve));
const snapshot = (refunded_minor, pending_minor = 0) => ({ captured_minor: 2500, refunded_minor, pending_minor,
  refundable_minor: 2500-refunded_minor-pending_minor, items: [], currency:'TWD' });
function host(onChanged = async () => true, client = {}) {
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
      postRefund(){throw Error('unexpected POST');},refreshRefund(){throw Error('unexpected POST');},...client};
    if(path==='@/lib/orders-model') return ordersModel;
    if(path==='@/lib/client') return {money:(_,__,v)=>String(v)};
    if(path.endsWith('.css')) return {};
    throw Error(path);
  }});
  const props={store:'store-a',boundary:'session-a',detail:{order_id:'order-a',currency:'TWD',updated_at:'v1',refunded_minor:1000,refund_pending_minor:0},locale:'en',c:{refundTitle:'Refund'},canRefund:false,onChanged};
  return {props,reads,get updates(){return updates;},render(p=props){cursor=0;const tree=module.exports.OrderRefunds(p);for(const e of effects.splice(0))e();return tree;},
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

// Traverse rendered actual-component nodes; no copied refresh/submission implementation.
function control(tree, id) {
  if (Array.isArray(tree)) { for (const n of tree) { const found=control(n,id); if(found)return found; } }
  else if (tree?.props) { if (tree.props['data-testid']===id) return tree; return control(tree.props.children,id); }
}
const pendingItem = {refund_id:'refund-synthetic',amount_minor:1000,state:'PENDING',reason:'requested_by_customer',
  requested_at:'2026-11-07T00:00:00Z',updated_at:'2026-11-07T00:00:00Z',stripe_refund_id:null};
const pendingSnapshot = () => ({...snapshot(0,1000),items:[pendingItem]});
function actionProps(h, overrides={}) {return {...h.props,canRefund:true,c:ordersCopy.en,
  detail:{...h.props.detail,payment_state:'CAPTURED',refunded_minor:0,refund_pending_minor:1000},...overrides};}

test('explicit Refresh re-reads authoritative REVIEW_REQUIRED detail after admission even with identical totals and item state',async()=>{
  let parentReads=0,postCount=0;
  let parentDetail={payment_state:'CAPTURED',refunded_minor:0,refund_pending_minor:1000};
  let authoritative={...parentDetail};
  const h=host(async()=>{parentReads++;parentDetail={...authoritative};return true;},
    {refreshRefund:async()=>{postCount++;return 'scheduled';}});
  const props=actionProps(h);h.render(props);h.reads[0].resolve(pendingSnapshot());await settle();
  assert.equal(parentReads,0);
  // Synthetic server has a sticky review; refund totals AND item state remain unchanged (§5/7.1).
  authoritative={...authoritative,payment_state:'REVIEW_REQUIRED'};
  assert.equal(authoritative.payment_state,'REVIEW_REQUIRED');
  control(h.render(props),'refund-refresh-refund-synthetic').props.onClick();await settle();
  h.render(props);assert.equal(parentReads,0,'must not refresh detail before new list admission');
  h.reads.at(-1).resolve(pendingSnapshot());await settle();
  assert.equal(postCount,1,'only the explicit refresh POST');
  assert.equal(parentDetail.payment_state,'REVIEW_REQUIRED','parent/detail GET seam must display authoritative equal-total review');
  assert.equal(parentReads,1,'one bounded parent GET for explicit refresh');
});

test('stale explicit Refresh completion cannot update or notify another store/order/session or an unmounted component',async()=>{
  for(const kind of ['store','order','session','unmount']) {
    let complete,notifications=0,posts=0;
    const h=host(async()=>{notifications++;return true;},{refreshRefund:()=>{posts++;return new Promise(resolve=>{complete=resolve;});}});
    const props=actionProps(h);h.render(props);h.reads[0].resolve(pendingSnapshot());await settle();
    control(h.render(props),'refund-refresh-refund-synthetic').props.onClick();await settle();
    const next=kind==='store'?{...props,store:'store-b'}:kind==='order'?{...props,detail:{...props.detail,order_id:'order-b'}}:{...props,boundary:'session-b'};
    if(kind==='unmount')h.unmount();else h.render(next);
    const before=h.updates,count=h.reads.length;complete('scheduled');await settle();
    assert.equal(h.updates,before,kind+' stale action cannot even change current local state');
    assert.equal(notifications,0);assert.equal(posts,1);
    if(kind!=='unmount'){h.render(next);assert.equal(h.reads.length,count,'old action must not tick current scope');h.reads.at(-1).resolve(pendingSnapshot());await settle();assert.equal(notifications,0);}
  }
});

test('explicit intent survives an aborted post-action read, is consumed once, and a later explicit action can refresh equal totals again',async()=>{
  let notifications=0,posts=0;
  const h=host(async()=>{notifications++;return true;},{refreshRefund:async()=>{posts++;return 'scheduled';}});
  let props=actionProps(h);h.render(props);h.reads[0].resolve(pendingSnapshot());await settle();
  control(h.render(props),'refund-refresh-refund-synthetic').props.onClick();await settle();h.render(props);const aborted=h.reads.at(-1);
  props={...props,detail:{...props.detail,updated_at:'new-detail'}};h.render(props);
  assert.ok(aborted.args[3].aborted);aborted.resolve(pendingSnapshot());await settle();assert.equal(notifications,0);
  h.reads.at(-1).resolve(pendingSnapshot());await settle();assert.equal(notifications,1);
  props={...props,detail:{...props.detail,updated_at:'reloaded'}};h.render(props);h.reads.at(-1).resolve(pendingSnapshot());await settle();assert.equal(notifications,1,'timestamp cannot replay consumed action');
  control(h.render(props),'refund-refresh-refund-synthetic').props.onClick();await settle();h.render(props);h.reads.at(-1).resolve(pendingSnapshot());await settle();assert.equal(notifications,2);assert.equal(posts,2);
});

test('explicit notification serializes behind an existing read and queued old-scope action is discarded',async()=>{
  for(const switchScope of [false,true]) {
    let notifications=0,release;
    const h=host(()=>{notifications++;return new Promise(resolve=>{release=resolve;});},{refreshRefund:async()=> 'scheduled'});
    const props=actionProps(h,{detail:{...actionProps(h).detail,refund_pending_minor:0}});
    h.render(props);h.reads[0].resolve(pendingSnapshot());await settle();assert.equal(notifications,1);
    control(h.render(props),'refund-refresh-refund-synthetic').props.onClick();await settle();h.render(props);h.reads.at(-1).resolve(pendingSnapshot());await settle();assert.equal(notifications,1,'explicit cannot overlap existing parent GET');
    if(switchScope)h.render({...props,boundary:'session-b'});
    release(true);await settle();assert.equal(notifications,switchScope?1:2);
    if(!switchScope){release(true);await settle();}
  }
});

function form(tree) {
  if(Array.isArray(tree)){for(const n of tree){const found=form(n);if(found)return found;}}
  else if(tree?.props){if(tree.type==='form')return tree;return form(tree.props.children);}
}
test('actual successful two-step submit also requests equal-total authoritative detail only after admission',async()=>{
  let reads=0,posts=0,parentState='CAPTURED';
  const h=host(async()=>{reads++;parentState='REVIEW_REQUIRED';return true;},{postRefund:async()=>{posts++;return {ok:true};}});
  const props=actionProps(h);h.render(props);h.reads[0].resolve(pendingSnapshot());await settle();
  control(h.render(props),'refund-open').props.onClick();
  form(h.render(props)).props.onSubmit({preventDefault(){}}); // Review; actual component switches to confirmation.
  form(h.render(props)).props.onSubmit({preventDefault(){}});await settle();
  assert.equal(posts,1);assert.equal(reads,0,'accepted POST alone must not read detail before list admission');
  h.render(props);h.reads.at(-1).resolve(pendingSnapshot());await settle();assert.equal(reads,1);assert.equal(parentState,'REVIEW_REQUIRED');
});
