// Purpose: probe rendered-draft ownership with actual pinned ReactDOM and full SettingsWizard.
// Depends on: jsdom, typescript-api, real presentation/client modules; only browser context and network are fixtures.
// Used by: focused issue544 author probe; not native browser acceptance.
import test from 'node:test';
import assert from 'node:assert/strict';
import {registerHooks, createRequire} from 'node:module';
import {readFileSync,existsSync,statSync} from 'node:fs';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {resolve,dirname} from 'node:path';
import {JSDOM} from 'jsdom';
import ts from 'typescript-api';
const rootDir=resolve(dirname(fileURLToPath(import.meta.url)),'../..');
const appRequire=createRequire(resolve(rootDir,'apps/admin/package.json'));
const reactPaths=Object.fromEntries(['react','react/jsx-runtime','react/jsx-dev-runtime','react-dom','react-dom/client','next/link'].map(s=>[s,pathToFileURL(appRequire.resolve(s)).href]));
const data=s=>'data:text/javascript,'+encodeURIComponent(s);
registerHooks({
 resolve(s,c,next){
  if(s.endsWith('.css')) return {url:data('export default {}'),shortCircuit:true};
  if(s==='next/navigation')return {url:data(`export const usePathname=()=>'/en/settings';export const useParams=()=>({locale:'en'});export const useSearchParams=()=>new URLSearchParams();export const useRouter=()=>({push(){},replace(){},refresh(){}});`),shortCircuit:true};
  if(reactPaths[s])return {url:reactPaths[s],shortCircuit:true};
  let f;
  if(s==='@live-commerce/i18n') f=resolve(rootDir,'packages/i18n/src/index.ts');
  else if(s==='@live-commerce/format') f=resolve(rootDir,'packages/format/src/index.ts');
  else if(s==='@live-commerce/ui') f=resolve(rootDir,'packages/ui/src/AppShell.tsx');
  else if(s==='@live-commerce/ui/ds') f=resolve(rootDir,'packages/ui/src/ds/index.ts');
  else if(s.startsWith('@/'))f=resolve(rootDir,'apps/admin',s.slice(2));
  else if(s.startsWith('.')&&c.parentURL?.startsWith('file:')) f=resolve(dirname(fileURLToPath(c.parentURL)),s);
  if(f)for(const ext of ['','.ts','.tsx'])if(existsSync(f+ext)&&statSync(f+ext).isFile())return {url:pathToFileURL(f+ext).href,shortCircuit:true};
  return next(s,c);
 },
 load(u,c,next){if(u.startsWith('file:')&&/\.tsx?$/.test(u))return {format:'module',shortCircuit:true,source:ts.transpileModule(readFileSync(fileURLToPath(u),'utf8'),{compilerOptions:{target:ts.ScriptTarget.ES2023,module:ts.ModuleKind.ESNext,jsx:ts.JsxEmit.ReactJSX},fileName:fileURLToPath(u)}).outputText};return next(u,c);}
});
const dom=new JSDOM('<div id="root"></div>',{url:'https://fixture.invalid/en/settings?meta_connect=1',pretendToBeVisual:true});
for(const name of ['window','document','navigator','sessionStorage','localStorage','HTMLElement','HTMLInputElement','Event','MouseEvent'])Object.defineProperty(globalThis,name,{value:dom.window[name],configurable:true});
globalThis.matchMedia=window.matchMedia=()=>({matches:false,addEventListener(){},removeEventListener(){}});
navigator.locks={request:async(_n,fn)=>fn()};
// Track real WebCrypto completion without replacing digest results or session guards.
const digests=new Set();
const realDigest=crypto.subtle.digest.bind(crypto.subtle);
crypto.subtle.digest=(...args)=>{const p=realDigest(...args);digests.add(p);p.finally(()=>digests.delete(p));return p;};
async function settleTransport(){
 const deadline=Date.now()+3000;
 do {assert.ok(Date.now()<deadline,'transport completion deadline');await Promise.all([...digests]);await new Promise(setImmediate);}while(digests.size);
}
const React=await import('react');
const {createRoot}=await import('react-dom/client');
const {flushSync}=await import('react-dom');
const {SettingsWizard}=await import('../../apps/admin/components/SettingsWizard.tsx');
const {emptyDraft}=await import('../../apps/admin/lib/settings-draft.ts');
const store={id:'00000000-0000-4000-8000-000000000001',name:'Fixture',currency:'TWD',role:'owner',permissions:['store:read','integration:read','integration:manage','orders:read']};
const market={id:'00000000-0000-4000-8000-000000000002',code:'tw',name:'Fixture',currency:'TWD',active:true,version:1};
const target=`${market.id}:TW:home`;
const resp=(body,status=200)=>new Response(JSON.stringify(body),{status,headers:{'Content-Type':'application/json','Cache-Control':'private, no-store'}});
async function until(fn,label){const end=Date.now()+3000;while(!fn()){assert.ok(Date.now()<end,label);await new Promise(r=>setTimeout(r,5));}}
for(const mode of ['before-commit','after-commit','baseline0','foreign-target','superseded','session-change','expired','unmounted','known-sequential','target-sequential','foreign-known','service-known-sequential','service-foreign-known'])test(`actual React policy404 ownership ${mode}`,{timeout:10000},async()=>{
 dom.reconfigure({url:'https://fixture.invalid/en/settings?meta_connect=1'});
 sessionStorage.clear();localStorage.clear();document.cookie=`__Host-commerce_csrf=${'a'.repeat(43)}; Secure; Path=/`;
 const digest=await crypto.subtle.digest('SHA-256',new TextEncoder().encode('a'.repeat(43)));
 const hash=Buffer.from(digest).toString('hex'),key=`commerce-settings-draft:${store.id}:${hash}`;
 const serviceControl=mode.startsWith('service-');
 const scopedControl=['known-sequential','target-sequential','foreign-known'].includes(mode);
 const draft={...emptyDraft,marketID:market.id,serviceCode:'home',reference:'fixture-reference',policyObservation:{target:mode==='foreign-known'?`${market.id}:JP:home`:target,version:scopedControl?7:mode==='baseline0'?0:-1,dirty:true}};
 if(serviceControl){draft.reference='';draft.policyObservation={target,version:7,dirty:false};draft.serviceObservation={target:mode==='service-foreign-known'?`${market.id}:JP:home`:target,version:7,dirty:true};}
 sessionStorage.setItem(key,JSON.stringify({version:2,step:3,draft}));
 let release,requested=false,commits=0,armed=false,fired=false,requests=0,unmounted=false,bodyConsumed=false;
 const ledger=[];
 const pending=new Promise(r=>release=r);
 globalThis.fetch=async(input,init)=>{
  assert.ok(!init?.method||init.method==='GET','no provider writes');const u=String(input);
  if(u.endsWith('/policy')){if(serviceControl){requested=true;return resp({version:7,shipping_minor:0,tax_mode:'none',tax_basis:'goods',tax_rate_bps:0,quote_ttl_seconds:300,enabled:false});}requested=true;requests++;ledger.push({event:'request',sequence:requests});return requests===1?pending:new Promise(()=>{});}
  if(serviceControl&&u.endsWith('/delivery-services/home'))return pending;
  if(u==='/api/stores')return resp({items:[store],next_cursor:''});
  if(u.endsWith('/markets'))return resp({items:[market],next_cursor:''});
  if(u.endsWith('/delivery-services'))return resp({items:[],next_cursor:''});
  return resp({code:'not_found'},404);
 };
 const saved=()=>JSON.parse(sessionStorage.getItem(key)).draft;
 const form=()=>document.querySelector('[data-testid="settings-policy-form"]');
 const checkbox=()=>form()?.querySelector('input[type="checkbox"]');
 const button=()=>form()?.querySelector('button[type="submit"]');
 const original=dom.window.Storage.prototype.setItem;
 dom.window.Storage.prototype.setItem=function(k,v){original.call(this,k,v);if(this===sessionStorage&&k===key){const d=JSON.parse(v).draft;ledger.push({event:'persist',version:d.policyObservation?.version,dirty:d.policyObservation?.dirty,commit:commits});if(armed&&!fired&&['before-commit','after-commit'].includes(mode)&&d.policyObservation?.version===0){fired=true;// Observe persistence only; queue a DOM event after saveDraft returns, without touching React state/refs.
 const admittedCommit=commits;queueMicrotask(async()=>{if(mode==='after-commit')await until(()=>commits>admittedCommit,'admission committed before control click');ledger.push({event:'click',commit:commits});checkbox().click();});}}};
 const root=createRoot(document.getElementById('root'));
 try{
  root.render(React.createElement(React.Profiler,{id:'wizard',onRender(){commits++;ledger.push({event:'commit',commit:commits,version:saved().policyObservation?.version});}},React.createElement(SettingsWizard,{locale:'en',initial:{store,error:null}})));
  await until(()=>requested&&(serviceControl?document.querySelector('[data-testid="settings-service-form"]'):checkbox()&&button()),'full real component mounted');
  await settleTransport();
  if(!serviceControl)assert.equal(button().disabled,!['baseline0','known-sequential','target-sequential'].includes(mode));if(!serviceControl)assert.equal(saved().policyObservation.version,scopedControl?7:mode==='baseline0'?0:-1);
  const edit=(label,value)=>{
   const input=[...document.querySelectorAll('input')].find(e=>e.closest('label')?.textContent.startsWith(label));
   assert.ok(input,`real ${label} input`);Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(input,value);input.dispatchEvent(new Event('input',{bubbles:true}));
  };
  if(serviceControl){
   const serviceForm=()=>document.querySelector('[data-testid="settings-service-form"]');
   const inputs=()=>[...serviceForm().querySelectorAll('input')];
   const inputValue=(input,value)=>{Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(input,value);input.dispatchEvent(new Event('input',{bubbles:true}));};
   await new Promise(resolve=>queueMicrotask(()=>{inputValue(inputs()[0],'保留名称');inputValue(inputs()[2],'Retained service');resolve();}));
   await settleTransport();
   const d=saved();
   assert.equal(inputs()[0].value,'保留名称','rendered first service field retained');
   assert.equal(inputs()[2].value,'Retained service','rendered second service field retained');
   assert.equal(d.nameHans,'保留名称');assert.equal(d.nameEN,'Retained service');
   assert.equal(d.country,'TW');assert.equal(d.serviceCode,'home');
   assert.deepEqual(d.serviceObservation,{target,version:mode==='service-known-sequential'?7:-1,dirty:true},'service version belongs only to matching target');
   console.log(JSON.stringify({mode,serviceObservation:d.serviceObservation,fieldsRetained:true}));return;
  }
  if(scopedControl){
   // Both events run in one microtask before React replaces the attached handlers.
   await new Promise(resolve=>queueMicrotask(()=>{
    if(mode==='target-sequential')edit('Country','JP');
    edit('Configuration reference / reason','retained-reference');edit('Flat shipping','12');resolve();
   }));
   await settleTransport();
   const d=saved(),country=mode==='target-sequential'?'JP':'TW';
   assert.equal(d.reference,'retained-reference');assert.equal(d.shipping,'12');assert.equal(d.country,country);assert.equal(d.serviceCode,'home');
   assert.deepEqual(d.policyObservation,{target:`${market.id}:${country}:home`,version:mode==='known-sequential'?7:-1,dirty:true});
   console.log(JSON.stringify({mode,version:d.policyObservation.version,dirty:d.policyObservation.dirty,fieldsRetained:true}));return;
  }
  const changeCountry=async value=>{
   // DOM fixture event, not native UI acceptance; native value setter bypasses React's input tracker only.
   const select=[...document.querySelectorAll('input')].find(e=>e.closest('label')?.textContent.startsWith('Country'));
   assert.ok(select,'real country input');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(select,value);select.dispatchEvent(new Event('input',{bubbles:true}));
   await until(()=>saved().country===value,'country event persisted');
  };
  if(mode==='foreign-target'||mode==='superseded'){
   await changeCountry('JP');await until(()=>requests>=2,'foreign request issued');
   if(mode==='superseded'){await changeCountry('TW');await until(()=>requests>=3,'same target newer request issued');}
  }
  if(mode==='session-change')document.cookie=`__Host-commerce_csrf=${'b'.repeat(43)}; Secure; Path=/`;
  if(mode==='expired')document.cookie='__Host-commerce_csrf=; Max-Age=0; Secure; Path=/';
  if(mode==='unmounted'){flushSync(()=>root.unmount());unmounted=true;}
  armed=true;ledger.push({event:'release404',commit:commits});
  const response=resp({code:'not_found'},404),readJSON=response.json.bind(response);
  response.json=async()=>{const value=await readJSON();bodyConsumed=true;return value;};
  release(response);
  if(['before-commit','after-commit'].includes(mode))await until(()=>fired,'404 admitted and persisted0');
  await until(()=>bodyConsumed,'real response body consumed');
  await settleTransport();
  if(['before-commit','after-commit'].includes(mode))await until(()=>ledger.some(e=>e.event==='click')&&saved().policyEnabled,'attached event completed');
  await settleTransport();
  const result=saved();
  console.log(JSON.stringify({mode,react:React.version,ledger,finalVersion:result.policyObservation?.version??null,saveDisabled:button()?.disabled??null,referenceRetained:result.reference===draft.reference,targetMatches:result.policyObservation?.target===target}));
  if(!['before-commit','after-commit','baseline0'].includes(mode)){
   if(mode==='foreign-target')assert.equal(result.country,'JP');
   if(mode==='superseded'){assert.equal(result.country,'TW');assert.equal(requests,3);}
   assert.equal(ledger.filter(e=>e.event==='persist'&&e.version===0).length,0,'rejected404 cannot admit baseline0');return;
  }
  if(mode!=='baseline0'){
   const admitted=ledger.find(e=>e.event==='persist'&&e.version===0),clicked=ledger.find(e=>e.event==='click');
   assert.ok(admitted&&clicked);
   assert.equal(result.policyEnabled,true,'real DOM handler changed checkbox');
   if(mode==='before-commit')assert.equal(clicked.commit,admitted.commit,'still-current attached handler before replacement commit');
   else assert.ok(clicked.commit>admitted.commit,'control uses replacement commit handler');
  }
  assert.equal(result.reference,draft.reference);assert.equal(result.policyObservation.target,target);
  assert.equal(result.policyObservation.version,0,'admitted baseline must survive the attached DOM handler');
  assert.equal(button().disabled,false);
 }finally{if(!unmounted)flushSync(()=>root.unmount());dom.window.Storage.prototype.setItem=original;}
});
// First discovery uses the same actual component/event seam as the accepted modes.
// MOCKDOM only: storage seeds are fixtures; all edits use rendered DOM controls.
for(const mode of ['late','early','known-zero','known-seven','foreign-observation','explicit-market','first-session-change','first-unmounted'])test(`actual React first-market ${mode}`,{timeout:10000},async()=>{
 dom.reconfigure({url:'https://fixture.invalid/en/settings?meta_connect=1'});
 sessionStorage.clear();localStorage.clear();document.cookie=`__Host-commerce_csrf=${'a'.repeat(43)}; Secure; Path=/`;
 const hash=Buffer.from(await crypto.subtle.digest('SHA-256',new TextEncoder().encode('a'.repeat(43)))).toString('hex');
 const key=`commerce-settings-draft:${store.id}:${hash}`;
 const control=['known-zero','known-seven','foreign-observation'].includes(mode);
 const draft={...emptyDraft,serviceCode:'home',marketID:mode==='explicit-market'?market.id:''};
 if(control)Object.assign(draft,{reference:'retained-reference',policyEnabled:true,policyObservation:{target:mode==='foreign-observation'?'zero:JP:home':'zero:TW:home',version:mode==='known-seven'?7:mode==='known-zero'?0:-1,dirty:true}});
 sessionStorage.setItem(key,JSON.stringify({version:2,step:3,draft}));
 let releaseMarket,releasePolicy,marketRequested=false,policyRequests=0,commits=0,unmounted=false,marketBody=false,policyBody=false;
 const marketPending=new Promise(r=>releaseMarket=r),policyPending=new Promise(r=>releasePolicy=r),ledger=[];
 globalThis.fetch=async(input,init)=>{
  assert.ok(!init?.method||init.method==='GET','first-market zero provider writes');
  const u=String(input);ledger.push({event:'GET',url:u});
  if(u.endsWith('/markets')){marketRequested=true;if(mode!=='early')await marketPending;const response=resp({items:[market],next_cursor:''}),read=response.json.bind(response);response.json=async()=>{const value=await read();marketBody=true;return value;};return response;}
  if(u.endsWith('/policy')){assert.ok(u.endsWith(`/markets/${market.id}/countries/TW/delivery-services/home/policy`),'scoped policy read');policyRequests++;return policyPending;}
  if(u==='/api/stores')return resp({items:[store],next_cursor:''});
  if(u.endsWith('/delivery-services'))return resp({items:[],next_cursor:''});
  return resp({code:'not_found'},404);
 };
 const saved=()=>JSON.parse(sessionStorage.getItem(key)).draft;
 const form=()=>document.querySelector('[data-testid="settings-policy-form"]');
 const checkbox=()=>form()?.querySelector('input[type="checkbox"]');
 const reference=()=>[...form().querySelectorAll('input')].find(e=>e.closest('label')?.textContent.startsWith('Configuration reference / reason'));
 const original=dom.window.Storage.prototype.setItem;
 dom.window.Storage.prototype.setItem=function(k,v){original.call(this,k,v);if(this===sessionStorage&&k===key)ledger.push({event:'persist',draft:JSON.parse(v).draft});};
 const root=createRoot(document.getElementById('root'));
 const release= (fn,body,status,mark)=>{const response=resp(body,status),read=response.json.bind(response);response.json=async()=>{const v=await read();mark();return v;};fn(response);};
 try{
  root.render(React.createElement(React.Profiler,{id:'first-market',onRender(){commits++;}},React.createElement(SettingsWizard,{locale:'en',initial:{store,error:null}})));
  await until(()=>marketRequested&&checkbox(),'first-market form mounted');await settleTransport();
  if(mode==='early')await until(()=>policyRequests===1,'market-before-edit control read');
  if(!control){
   // Native value setter + input is the existing MOCKDOM seam, never native browser evidence.
   Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(reference(),'retained-reference');reference().dispatchEvent(new Event('input',{bubbles:true}));
   checkbox().click();await until(()=>saved().reference==='retained-reference'&&saved().policyEnabled,'both DOM edits persisted');
  }
  assert.equal(saved().policyObservation.version,control?draft.policyObservation.version:-1,'no fabricated baseline before scoped response');
  if(mode==='first-session-change')document.cookie=`__Host-commerce_csrf=${'b'.repeat(43)}; Secure; Path=/`;
  if(mode==='first-unmounted'){flushSync(()=>root.unmount());unmounted=true;}
  if(mode!=='early'){
   releaseMarket();
   await until(()=>marketBody,'market response body consumed');await settleTransport();
  }
  if(mode.startsWith('first-')){
   assert.equal(policyRequests,0,'rejected discovery cannot start scoped read');
   assert.equal(ledger.filter(e=>e.event==='persist'&&e.draft.policyObservation?.version===0).length,0,'rejected discovery cannot admit zero');return;
  }
  await until(()=>policyRequests===1,'discovery triggered one scoped policy read');await settleTransport();
  const before=saved();
  assert.equal(before.reference,'retained-reference');assert.equal(before.policyEnabled,true);
  if(control)assert.deepEqual(before.policyObservation,draft.policyObservation,'known or foreign observation must not be reassociated');
  if(mode==='explicit-market')assert.equal(before.policyObservation.target,'zero:TW:home','explicit market is not pending zero identity');
  const commitBefore=commits;
  ledger.push({event:'release-policy404'});
  release(releasePolicy,{code:'not_found'},404,()=>policyBody=true);
  await until(()=>policyBody,'scoped 404 response body consumed');await settleTransport();
  await until(()=>commits>commitBefore,'scoped response React commit');await settleTransport();
  const result=saved();console.log(JSON.stringify({mode,react:React.version,ledger,result}));
  assert.equal(result.marketID,market.id,'discovered target');
  assert.equal(result.policyObservation.target,target,'baseline owns scoped target');
  assert.equal(result.policyObservation.version,0,'zero comes only from admitted scoped 404');
  if(['late','early'].includes(mode)){
   assert.equal(result.reference,'retained-reference','first-market scoped404 retains edited reference');
   assert.equal(result.policyEnabled,true,'first-market scoped404 retains edited policy enable');
   assert.equal(reference().value,'retained-reference','rendered reference retained');assert.equal(checkbox().checked,true,'rendered policy enable retained');
   assert.equal(result.policyObservation.dirty,true,'edited observation remains dirty');
  }else{
   assert.equal(result.reference,'','unowned observation follows ordinary hydration');
   assert.equal(result.policyEnabled,false);assert.equal(result.policyObservation.dirty,false);
  }
 }finally{if(!unmounted)flushSync(()=>root.unmount());dom.window.Storage.prototype.setItem=original;}
});
test.after(()=>{crypto.subtle.digest=realDigest;dom.window.close();});
