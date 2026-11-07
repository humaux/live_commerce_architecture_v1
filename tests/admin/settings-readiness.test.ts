// Purpose: controlled actual SettingsWizard admission schedules; TEST ONLY, MOCK host/network.
// Depends on: existing inbox TSX/hooks/effects host and unmodified wizard/client/draft sources.
// Called by: canonical test-node transform-types group. No browser/production acceptance.
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
const {emptyDraft}=await import('../../apps/admin/lib/settings-draft.ts');
const {sessionBoundary}=await import('../../apps/admin/lib/settings-client.ts');
const market={id:'20000000-0000-4000-8000-000000000002',code:'tw',name:'Synthetic',currency:'TWD',active:true,version:1};
const target=`${market.id}:TW:home`;
function deferred<T>(){let resolve!:(v:T)=>void;const promise=new Promise<T>(r=>resolve=r);return {promise,resolve};}
function storage(){const values=new Map<string,string>();return {getItem:(k:string)=>values.get(k)??null,setItem:(k:string,v:string)=>values.set(k,v),removeItem:(k:string)=>values.delete(k),values};}
const absent=()=>new Response(JSON.stringify({code:'not_found',retryable:false,details:{}}),{status:404});
const page=(items:unknown[])=>new Response(JSON.stringify({items,next_cursor:''}));
function all(tree:any):any[]{return Array.isArray(tree)?tree.flatMap(all):tree?.props?[tree,...all(tree.props.children)]:[];}
function input(h:any,label:string){const l=node(h,n=>n.type==='label'&&textOf(n.props.children).includes(label));return all(l).find(n=>n.type==='input');}
function edit(h:any,label:string,value:string|boolean){const n=input(h,label);n.props.onChange({target:typeof value==='boolean'?{checked:value}:{value}});h.flush();}
function save(h:any){const f=node(h,n=>n.props['data-testid']==='settings-policy-form');return all(f).find(n=>n.type==='button'&&n.props.type==='submit');}
async function setup(t:any){
 const env=environment(t),ss=storage(),ls=storage();
 const prior={sessionStorage:globalThis.sessionStorage,localStorage:globalThis.localStorage,locks:Object.getOwnPropertyDescriptor(navigator,'locks')};
 Object.assign(globalThis,{sessionStorage:ss,localStorage:ls});Object.defineProperty(navigator,'locks',{configurable:true,value:{request:async(_name:any,...args:any[])=>args.at(-1)()}});
 Object.assign(env.document,{cookie:'__Host-commerce_csrf='+ 'a'.repeat(43)});
 Object.assign(env.window,{location:{href:'https://local.invalid/en/settings'},confirm:()=>true});
 t.after(()=>{Object.assign(globalThis,{sessionStorage:prior.sessionStorage,localStorage:prior.localStorage});if(prior.locks)Object.defineProperty(navigator,'locks',prior.locks);else delete (navigator as any).locks;});
 const key=`commerce-settings-draft:${store.id}:${await sessionBoundary()}`;
 ss.setItem(key,JSON.stringify({version:2,step:3,draft:{...emptyDraft,marketID:market.id,serviceCode:'home'}}));
 const policies:{gate:ReturnType<typeof deferred<Response>>}[]=[],services:{gate:ReturnType<typeof deferred<Response>>}[]=[];let writes=0;
 globalThis.fetch=async(input,init)=>{
  if(init?.method&&init.method!=='GET')writes++;
  assert.ok(!init?.method||init.method==='GET','read-only schedule must never dispatch a write');
  const path=String(input);
  if(path.endsWith('/policy')){const gate=deferred<Response>();policies.push({gate});return gate.promise;}
  if(/delivery-services\/[^/]+$/.test(path)){const gate=deferred<Response>();services.push({gate});return gate.promise;}
  if(path.endsWith('/markets'))return page([market]);
  if(path.endsWith('/delivery-services'))return page([]);
  if(path.endsWith('/logistics/ecpay'))return absent();
  throw new Error('unexpected synthetic route');
 };
 const h=env.mount(()=>SettingsWizard({locale:'en',initial:{store,stores:[store],error:null} as any}));
 node(h,n=>n.type==='button'&&textOf(n)==='Edit settings').props.onClick();h.flush();
 await h.settle();
 assert.equal(policies.length,1,'S00 initial policy request');assert.equal(services.length,1,'S00 initial service request');
 const draft=()=>JSON.parse(ss.getItem(key)!).draft;
 assert.equal(ls.values.size,0,'no pending journal');assert.equal(save(h).props.disabled,true);
 t.diagnostic('S00 mounted actual advanced; initial read held; baseline unresolved; writes=0');
 return {env,h,draft,policies,services,writes:()=>writes,ss,key,t};
}
function latest(s:any,reference='Synthetic latest'){
 edit(s.h,'Configuration reference / reason',reference);edit(s.h,'Enable pricing policy',true);
 assert.equal(s.draft().policyObservation.version,-1);assert.equal(save(s.h).props.disabled,true);
}
function accepted(s:any,reference='Synthetic latest'){
 assert.deepEqual(s.draft().policyObservation,{target,version:0,dirty:true},'admitted same-target 404 must establish baseline 0');
 assert.equal(s.draft().reference,reference,'latest reference preserved');assert.equal(s.draft().policyEnabled,true,'latest enable preserved');
 assert.equal(save(s.h).props.disabled,false,'real rendered Save must become enabled');assert.equal(s.writes(),0);s.t.diagnostic('ADMITTED policy baseline=0 dirty=true latest-fields=preserved Save=enabled writes=0');
}
// Intercept only completion timing of real WebCrypto. The production sessionBoundary runs unchanged.
function holdDigest(t:any){const original=crypto.subtle.digest.bind(crypto.subtle),entered=deferred<void>(),release=deferred<void>();let calls=0;
 crypto.subtle.digest=(async(...args:Parameters<SubtleCrypto['digest']>)=>{calls++;entered.resolve();const result=await original(...args);await release.promise;return result;}) as any;
 t.after(()=>{crypto.subtle.digest=original;release.resolve();});return {entered:entered.promise,release:()=>release.resolve(),calls:()=>calls};}

test('S01 actual admitted policy404 after separately rendered edits',async t=>{
 const s=await setup(t);latest(s);s.t.diagnostic('EDIT reference render; EDIT enable render; RELEASE policy404');s.policies[0].gate.resolve(absent());await s.h.settle();accepted(s);
});
test('S02 actual stillSession digest held while latest edit commits',async t=>{
 const s=await setup(t);latest(s);const d=holdDigest(t);s.policies[0].gate.resolve(absent());await d.entered;
 s.t.diagnostic('DIGEST entered actual stillSession; EDIT latest reference render');edit(s.h,'Configuration reference / reason','Synthetic during digest');d.release();await s.h.settle();assert.equal(d.calls(),1);accepted(s,'Synthetic during digest');
});
test('S03 sibling service completion after admitted policy cannot overwrite baseline/latest fields',async t=>{
 const s=await setup(t);latest(s);s.t.diagnostic('EDIT reference render; EDIT enable render; RELEASE policy404');s.policies[0].gate.resolve(absent());await s.h.settle();accepted(s);
 s.t.diagnostic('RELEASE sibling service404 after policy baseline=0');s.services[0].gate.resolve(absent());await s.h.settle();accepted(s);assert.equal(s.draft().serviceObservation.version,0);
});
for(const boundary of ['target','superseded','unmount','epoch'] as const)test(`N-${boundary} rejects old policy404 without hydration or writes`,async t=>{
 const s=await setup(t);latest(s);
 if(boundary==='target'||boundary==='superseded'){
  edit(s.h,'Stable service code','other');if(boundary==='superseded')edit(s.h,'Stable service code','home');
 }else if(boundary==='unmount')s.h.dispose();
 else {node(s.h,n=>n.type==='button'&&textOf(n).includes('Back to overview')).props.onClick();s.h.flush();}
 const before=s.ss.getItem(s.key);s.policies[0].gate.resolve(absent());s.services[0].gate.resolve(absent());await s.h.settle();
 assert.equal(s.ss.getItem(s.key),before,'rejected read must not persist hydration');assert.equal(s.writes(),0);
});
test('N-session changed during real digest rejects policy admission',async t=>{
 const s=await setup(t);latest(s);const before=s.ss.getItem(s.key),d=holdDigest(t);s.policies[0].gate.resolve(absent());await d.entered;
 Object.assign(s.env.document,{cookie:'__Host-commerce_csrf='+ 'b'.repeat(43)});d.release();await s.h.settle();
 assert.equal(s.ss.getItem(s.key),before);assert.equal(save(s.h).props.disabled,true);assert.equal(s.writes(),0);
});
test('S04 INTENTIONAL RED same-render handlers: synthetic counterexample, browser cause UNKNOWN',async t=>{
 const s=await setup(t);const reference=input(s.h,'Configuration reference / reason').props.onChange,enable=input(s.h,'Enable pricing policy').props.onChange;
 reference({target:{value:'Synthetic same-render'}});enable({target:{checked:true}});s.h.flush();
 assert.equal(s.draft().reference,'','counterexample loses reference before policy completion');
 s.policies[0].gate.resolve(absent());await s.h.settle();
 assert.equal(s.draft().policyObservation.version,0);assert.equal(save(s.h).props.disabled,false);assert.equal(s.writes(),0);
 // Explicitly observe captured-closure lost edit; this is not evidence of a browser event schedule.
 s.t.diagnostic('SYNTHETIC policy admitted baseline=0 Save=enabled writes=0; reference lost before 404; browser relevance UNKNOWN');
 assert.equal(s.draft().policyEnabled,true);
 assert.equal(s.draft().reference,'Synthetic same-render','latest reference must survive: synthetic same-render handler counterexample');
});
