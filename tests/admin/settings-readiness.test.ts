// Purpose: legal entry-event regression for late market discovery; actual TSX, MOCK host/network.
// Depends on: existing inbox TSX/hooks/effects host and actual wizard/client/draft sources.
// Used by: canonical test-node transform-types group. No browser/production acceptance.
// Invariant: admitted same-target absent policy establishes CAS 0 without losing latest edits or writing.
import test from 'node:test';
import assert from 'node:assert/strict';
import {registerHooks} from 'node:module';
import {environment,store,node,textOf} from './inbox-review-host.test.ts';
// Presentation-only siblings do not participate in advanced policy readiness. Preserve real wizard entry callback.
const siblings=['AdminPageHeader','LogisticsSettings','BankTransferSettings','CodSettings','NotifySettings','StorefrontSettings','MetaConnect','SettingsV2'];
registerHooks({resolve(specifier,context,next){
 const name=specifier.replace(/^\.\//,'');
 if(context.parentURL?.endsWith('/SettingsWizard.tsx')&&siblings.includes(name))return {url:'data:text/javascript,'+encodeURIComponent(`export const ${name} = props => ${name==='SettingsV2'?"({type:'button',props:{onClick:props.onAdvanced,children:'Edit settings'}})":"null"};`),shortCircuit:true};
 return next(specifier,context);
}});
const {SettingsWizard}=await import('../../apps/admin/components/SettingsWizard.tsx');
function deferred<T>(){let resolve!:(v:T)=>void;const promise=new Promise<T>(r=>resolve=r);return {promise,resolve};}
function storage(){const values=new Map<string,string>();return {getItem:(k:string)=>values.get(k)??null,setItem:(k:string,v:string)=>values.set(k,v),removeItem:(k:string)=>values.delete(k),values};}
const absent=()=>new Response(JSON.stringify({code:'not_found',retryable:false,details:{}}),{status:404});
const page=(items:unknown[])=>new Response(JSON.stringify({items,next_cursor:''}));
function all(tree:any):any[]{return Array.isArray(tree)?tree.flatMap(all):tree?.props?[tree,...all(tree.props.children)]:[];}
function input(h:any,label:string){const l=node(h,n=>n.type==='label'&&textOf(n.props.children).includes(label));return all(l).find(n=>n.type==='input');}
function edit(h:any,label:string,value:string|boolean){const n=input(h,label);n.props.onChange({target:typeof value==='boolean'?{checked:value}:{value}});h.flush();}
function save(h:any){const f=node(h,n=>n.props['data-testid']==='settings-policy-form');return all(f).find(n=>n.type==='button'&&n.props.type==='submit');}

// Same legacy/inactive/foreign-currency fixture as settings-v2.spec.ts case c.
const marketID = '80000000-0000-4000-8000-000000000041';
const target = `${marketID}:TW:home`;
const rows = [
 {id:marketID,code:'legacy',name:'legacy',currency:'TWD',active:true,version:1},
 {id:'80000000-0000-4000-8000-000000000042',code:'off',name:'off',currency:'TWD',active:false,version:1},
 {id:'80000000-0000-4000-8000-000000000043',code:'usd',name:'usd',currency:'USD',active:true,version:1},
];
async function entry(t:any, timing:'before'|'after' = 'after') {
 const env=environment(t),ss=storage(),ls=storage();
 const prior={sessionStorage:globalThis.sessionStorage,localStorage:globalThis.localStorage,locks:Object.getOwnPropertyDescriptor(navigator,'locks')};
 Object.assign(globalThis,{sessionStorage:ss,localStorage:ls});
 Object.defineProperty(navigator,'locks',{configurable:true,value:{request:async(_name:any,...args:any[])=>args.at(-1)()}});
 Object.assign(env.document,{cookie:'__Host-commerce_csrf='+'a'.repeat(43)});
 Object.assign(env.window,{location:{href:'https://local.invalid/en/settings'},confirm:()=>true});
 t.after(()=>{
  Object.assign(globalThis,{sessionStorage:prior.sessionStorage,localStorage:prior.localStorage});
  if(prior.locks)Object.defineProperty(navigator,'locks',prior.locks);else delete (navigator as any).locks;
 });
 const market=deferred<Response>(),policies:ReturnType<typeof deferred<Response>>[]=[],services:ReturnType<typeof deferred<Response>>[]=[];
 let writes=0;
 globalThis.fetch=async(value,init)=>{
  if(init?.method&&init.method!=='GET')writes++;
  assert.ok(!init?.method||init.method==='GET','entry and read admission must send no writes');
  const path=String(value);
  if(path.endsWith('/markets'))return market.promise;
  if(path.endsWith('/policy')){const gate=deferred<Response>();policies.push(gate);return gate.promise;}
  if(/delivery-services\/[^/]+$/.test(path)){const gate=deferred<Response>();services.push(gate);return gate.promise;}
  if(path.endsWith('/delivery-services'))return page([]);
  if(path.endsWith('/logistics/ecpay'))return absent();
  throw new Error('unexpected synthetic route');
 };
 const h=env.mount(()=>SettingsWizard({locale:'en',initial:{store,error:null}}));
 // Presentation adapter invokes the real entry callback; all subsequent controls are real TSX.
 node(h,n=>n.type==='button'&&textOf(n)==='Edit settings').props.onClick();h.flush();await h.settle();
 assert.equal(ss.values.size,0,'fresh session, no preseeded draft');
 if(timing==='before'){market.resolve(page(rows));await h.settle();}
 for(const step of [2,3]){
  const next=node(h,n=>n.type==='button'&&textOf(n)==='Continue');
  assert.equal(next.props.disabled,false);next.props.onClick();h.flush();
  assert.equal(JSON.parse([...ss.values.values()][0]).step,step);
 }
 // Each edit obtains a fresh rendered handler. Never invoke two stale handlers from one render.
 edit(h,'Stable service code','home');
 edit(h,'Configuration reference / reason','Synthetic latest reference');
 edit(h,'Enable pricing policy',true);
 const draft=()=>JSON.parse([...ss.values.values()][0]).draft;
 assert.equal(draft().reference,'Synthetic latest reference');
 assert.equal(draft().policyEnabled,true);
 if(timing==='after'){
  assert.equal(policies.length,0);assert.equal(draft().policyObservation.version,-1);
  market.resolve(page(rows));await h.settle();
 }
 assert.equal(policies.length,1);assert.equal(services.length,1);
 assert.equal(save(h).props.disabled,true,'unresolved baseline blocks Save');
 t.after(()=>assert.equal(writes,0));
 t.after(()=>assert.equal(ls.values.size,0,'reads never create a pending command journal'));
 return {h,env,ss,draft,policies,services};
}
function retained(s:Awaited<ReturnType<typeof entry>>) {
 assert.equal(s.draft().reference,'Synthetic latest reference','latest reference survives async admission');
 assert.equal(s.draft().policyEnabled,true,'latest enabled survives async admission');
 assert.equal(input(s.h,'Configuration reference / reason').props.value,'Synthetic latest reference');
 assert.equal(input(s.h,'Enable pricing policy').props.checked,true);
}
for(const timing of ['before','after'] as const)test(`entry market-${timing}: matching 404 retains edits and admits baseline zero`,async t=>{
 const s=await entry(t,timing);s.policies[0].resolve(absent());await s.h.settle();
 retained(s);assert.deepEqual(s.draft().policyObservation,{target,version:0,dirty:true});
 assert.equal(save(s.h).props.disabled,false,'actual rendered Save becomes enabled');
 s.services[0].resolve(absent());await s.h.settle();retained(s);
 assert.deepEqual(s.draft().policyObservation,{target,version:0,dirty:true},'sibling completion preserves admission');
 t.diagnostic('matching404 baseline=0 latest fields retained Save enabled; original disabled-Save signature not reproduced');
});
test('entry market-after: existing version conflicts instead of overwriting unobserved edits',async t=>{
 const s=await entry(t);
 s.policies[0].resolve(new Response(JSON.stringify({market_id:marketID,country:'TW',method:'delivery:home',version:7,shipping_minor:6000,tax_mode:'none',tax_basis:'goods',tax_rate_bps:0,quote_ttl_seconds:300,enabled:false})));
 await s.h.settle();retained(s);
 assert.equal(s.draft().policyObservation.version,-1,'a positive response must not manufacture an accepted version');
 assert.equal(save(s.h).props.disabled,true);
 assert.match(textOf(s.h.flush()),/Configuration changed\. Reload before saving/);
});
for(const boundary of ['target','superseded','session','unmount'] as const)test(`entry late404: ${boundary} boundary rejects old hydration`,async t=>{
 const s=await entry(t);
 if(boundary==='target'||boundary==='superseded'){
  edit(s.h,'Stable service code','other');
  if(boundary==='superseded')edit(s.h,'Stable service code','home');
 }else if(boundary==='session')Object.assign(s.env.document,{cookie:'__Host-commerce_csrf='+'b'.repeat(43)});
 else s.h.dispose();
 const before=[...s.ss.values.values()];
 s.policies[0].resolve(absent());s.services[0].resolve(absent());await s.h.settle();
 assert.deepEqual([...s.ss.values.values()],before,'rejected responses cannot persist any hydration');
});
