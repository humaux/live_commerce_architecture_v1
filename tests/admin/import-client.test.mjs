// Purpose: real import route/Request/client/auth seams, including coded local errors and privacy fences.
// Depends on: import-wire-real-loader, actual auth/settings-client/import modules, native test/assert/Blob/crypto.
// Used by: W5-U1 Node gate; only fetch network edges and browser DOM facilities are simulated, no helper stand-ins.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {registerImportWireLoader} from './import-wire-real-loader.mjs';
registerImportWireLoader();
const origin='https://admin.example.invalid',api='https://go.example.invalid';
const token=Buffer.alloc(32,1).toString('base64url'),csrf=Buffer.alloc(32,2).toString('base64url');
const syntheticEnv={COMMERCE_IDENTITY_ENABLED:'1',COMMERCE_PASSWORD_LOGIN_ENABLED:'1',COMMERCE_FIXTURE_ENABLED:'0',COMMERCE_BFF_KEY:Buffer.alloc(32,3).toString('base64url'),COMMERCE_PUBLIC_ORIGIN:origin,COMMERCE_API_ORIGIN:api,COMMERCE_OIDC_ISSUER:''};
const saved=Object.fromEntries(Object.keys(syntheticEnv).map(k=>[k,process.env[k]]));
Object.assign(process.env,syntheticEnv);
const auth=await import('../../apps/admin/lib/auth.ts');
const route=await import('../../apps/admin/app/api/stores/[store]/imports/[param]/[action]/route.ts');
const client=await import('../../apps/admin/lib/import-client.ts');
const settings=await import('../../apps/admin/lib/settings-client.ts');
const model=await import('../../apps/admin/lib/import-model.ts');
for(const [k,v]of Object.entries(saved)){if(v===undefined)delete process.env[k];else process.env[k]=v;}
const id='11111111-1111-4111-8111-111111111111',hash='a'.repeat(64);
const raw={file_sha256:hash,headers:['Customer ID','Name'],mapping:{external_id:'Customer ID',name:'Name'},rows_total:1,new_rows:1,update_rows:0,apply_rows:1,failed_rows:0,erased_rows:0,consent_ignored_rows:0,rows:[{row:1,external_id:'synthetic-source',outcome:'created'}]};
const receipt={batch_id:id,created:1,updated:0,failed:0,replayed:false};
const json=(value,status=200,cache='private, no-store')=>Response.json(value,{status,headers:{'Cache-Control':cache}});
const store={id,name:'Synthetic store',currency:'TWD',role:'owner',permissions:['customers:privacy']};
const cookie=()=>`${auth.SESSION_COOKIE}=${token}; ${auth.CSRF_COOKIE}=${csrf}`;
const context=(param='customers',action='preview')=>({params:Promise.resolve({store:id,param,action})});
const upload=(action='preview',over={})=>new Request(`${origin}/api/stores/${id}/imports/customers/${action}${action==='commit'?'?expected_apply_rows=1':''}`,{method:'POST',headers:{Origin:origin,Cookie:cookie(),'X-CSRF-Token':csrf,'Content-Type':'text/csv',Authorization:'Bearer client-attacker',...over.headers},body:new Uint8Array([239,187,191,65,13,10,255]),signal:over.signal});

// Browser-only DOM facilities are unavailable in Node. The cookies are read by real csrfCookie/sessionBoundary.
async function seam(run,options={}){
 const prior={fetch:globalThis.fetch,document:globalThis.document,create:URL.createObjectURL,revoke:URL.revokeObjectURL};
 const exposed=[],clicks=[],calls=[],browserRequests=[],browserResponses=[];
 globalThis.document={cookie:options.browserCookie??`${auth.CSRF_COOKIE}=${csrf}`,visibilityState:'visible',body:{append(){}},createElement:()=>({click(){clicks.push(true);},remove(){}})};
 URL.createObjectURL=(blob)=>{exposed.push(blob);return 'blob:import-test';};URL.revokeObjectURL=()=>{};
 globalThis.fetch=async(url,init={})=>{
  const target=new URL(typeof url==='string'?url:url.url,origin);
  if(target.origin===origin){
   const headers=new Headers(init.headers);headers.set('Origin',origin);headers.set('Sec-Fetch-Site','same-origin');headers.set('Cookie',options.serverCookie??cookie());
   if(options.browserHeaders)for(const[k,v]of Object.entries(options.browserHeaders)){if(v===null)headers.delete(k);else headers.set(k,v);}
   const request=new Request(target.href,{...init,headers});browserRequests.push(request);
   const parts=target.pathname.split('/').filter(Boolean),ctx={params:Promise.resolve({store:parts[2],param:parts[4],action:parts[5]})};
   // This is the client HTTP boundary, driving the actual exported route with an actual Request.
   const response=await route[request.method](request,ctx);browserResponses.push(response);
   return options.browserResponse?options.browserResponse(response):response;
  }
  assert.equal(target.origin,api);calls.push({url:target.href,init});
  if(target.pathname==='/v1/admin/stores')return options.storesResponse?options.storesResponse():json({items:options.stores??[store]});
  return options.upstream?options.upstream(target,init):json(raw);
 };
 try{await run({calls,browserRequests,browserResponses,exposed,clicks});}
 finally{globalThis.fetch=prior.fetch;URL.createObjectURL=prior.create;URL.revokeObjectURL=prior.revoke;if(prior.document===undefined)delete globalThis.document;else globalThis.document=prior.document;}
}
async function input(over={}){return {store:id,kind:'customers',action:'commit',file:new Blob(['\ufeffCustomer ID,Name\r\nsynthetic,Synthetic\r\n']),mapping:{external_id:'Customer ID',name:'Name'},expectedApplyRows:1,boundary:Object.hasOwn(over,'boundary')?over.boundary:await settings.sessionBoundary(),signal:new AbortController().signal,...over};}

test('real coded errors retain baseline classification and normalize import leaf cache to private,no-store',async()=>{
 for(const [status,code]of [[401,'unauthorized'],[403,'forbidden'],[413,'invalid_request'],[415,'invalid_request'],[422,'invalid_request'],[422,'encoding_not_utf8'],[409,'idempotency_conflict']])await seam(async({calls,browserRequests,browserResponses})=>{
  const value=await client.sendImport(await input());assert.deepEqual(value,{kind:'error',code,uncertain:false});
  assert.equal(browserRequests.length,1);assert.equal(calls.length,2);assert.equal(browserResponses[0].headers.get("cache-control"),"private, no-store");
 },{upstream:()=>json({code,message:'private diagnostics',request_id:'upstream',retryable:false,details:{cell:'raw'}},status)});
});

test('preflight missing CSRF is definite and sends nothing; actual route CSRF and session refusals stay definite',async()=>{
 await seam(async({calls,browserRequests})=>{assert.deepEqual(await client.sendImport(await input({boundary:'old-session'})),{kind:'error',code:'unauthorized',uncertain:false});assert.equal(calls.length,0);assert.equal(browserRequests.length,0);},{browserCookie:''});
 for(const [options,code]of [[{serverCookie:`${auth.CSRF_COOKIE}=${csrf}`},'unauthorized'],[{browserHeaders:{'X-CSRF-Token':null}},'forbidden'],[{browserHeaders:{Origin:'https://attacker.invalid'}},'forbidden']])await seam(async({calls})=>{
  assert.deepEqual(await client.sendImport(await input()),{kind:'error',code,uncertain:false});assert.equal(calls.length,0);
 },options);
});

test('actual route uses real cookies/store membership and method/error envelopes, no helper substitutions',async()=>{
 await seam(async({calls})=>{
  assert.equal(auth.sessionToken(upload()),token);assert.equal(auth.requireOrigin(upload()),true);assert.equal(auth.requireCSRF(upload()),true);
  const forbidden=await route.POST(upload('preview',{headers:{'X-CSRF-Token':Buffer.alloc(32,4).toString('base64url')}}),context());assert.equal(forbidden.status,403);assert.equal(forbidden.headers.get('cache-control'),'private, no-store');assert.equal((await forbidden.json()).code,'forbidden');assert.equal(calls.length,0);
  const deletion=new Request(upload(),{method:'DELETE'});assert.equal(deletion.method,'DELETE');assert.notEqual(deletion.body,null);
  const method=await route.DELETE(deletion,context());assert.equal(method.status,405);assert.equal(method.headers.get('cache-control'),'private, no-store');assert.equal(method.headers.get('allow'),'GET, POST');assert.equal((await method.json()).code,'method_not_allowed');
 });
 for(const [stores,status,code]of [[[],404,'not_found'],[[{...store,permissions:[]}],403,'forbidden']])await seam(async()=>{assert.deepEqual(await client.sendImport(await input()),{kind:'error',code,uncertain:false});},{stores});
});

test('actual client/route preserves original Blob bytes, trusted transport and privacy projection on200/409',async()=>{
 for(const status of [200,409])await seam(async({calls,browserRequests})=>{
  const data=await input(status===200?{action:'preview',expectedApplyRows:undefined}:{});
  const original=new Uint8Array(await data.file.arrayBuffer());const result=await client.sendImport(data);
  assert.equal(result.kind,status===200?'preview':'stale');assert.deepEqual(result.value,model.projectImportPreview(raw,'customers'));assert.equal(JSON.stringify(result).includes('synthetic-source'),false);
  assert.equal(browserRequests.length,1);const sent=calls[1];assert.deepEqual([...sent.init.body],[...original]);assert.deepEqual(sent.init.headers,{Authorization:`Bearer ${token}`,Accept:'application/json','Content-Type':'text/csv'});assert.equal(sent.init.redirect,'error');assert.equal(sent.init.cache,'no-store');assert.equal(new URL(sent.url).searchParams.has('mapping'),true);
 },{upstream:()=>json(raw,status)});
});

test('success or stale preview must remain private; malformed/unknown/mismatched coded errors are uncertain after commit',async()=>{
 const invalidPrivate=(...args)=>{const response=auth.localError(...args);response.headers.set('Cache-Control','private, no-store');return response;};
 const cases=[()=>json(receipt,200,'no-store'),()=>json(model.projectImportPreview(raw,'customers'),409,'no-store'),()=>invalidPrivate(409,'unknown_import_code'),()=>json({code:'forbidden'},403,'private, no-store'),()=>invalidPrivate(422,'forbidden'),()=>invalidPrivate(403,'forbidden',undefined,{cell:'raw'})];
 for(const response of cases)await seam(async()=>{const value=await client.sendImport(await input());assert.equal(value.kind,'error');assert.equal(value.uncertain,true);assert.ok(['retry_later','unauthorized'].includes(value.code));},{upstream:()=>json(receipt),browserResponse:response});
});

test('lost/unreadable/session-changed dispatched commit remains UNKNOWN; no automatic retry',async()=>{
 await seam(async({calls})=>{const data=await input();const value=await client.sendImport(data);assert.equal(value.kind,'error');assert.equal(value.uncertain,true);assert.equal(calls.length,2);},{upstream:()=>{throw Error('lost network response');}});
 await seam(async({calls})=>{const value=await client.sendImport(await input());assert.equal(value.uncertain,true);assert.equal(calls.length,2);},{upstream:()=>{document.cookie=`${auth.CSRF_COOKIE}=${Buffer.alloc(32,5).toString('base64url')}`;return json(receipt);}});
});

test('actual failed-only result route strips IDs and real final session/cancel fence exposes no download',async()=>{
 const headers={'Cache-Control':'no-store','Content-Type':'text/csv; charset=utf-8','Content-Disposition':'attachment; filename="customer-import-results.csv"'};
 await seam(async({calls,exposed,clicks})=>{const value=await client.downloadImportFailures({store:id,batch:id,boundary:await settings.sessionBoundary(),signal:new AbortController().signal});assert.equal(value,'done');assert.equal(calls.length,2);assert.equal(clicks.length,1);assert.equal((await exposed[0].text()).includes('external_id'),false);},{upstream:()=>new Response('\ufeffrow,external_id,outcome,code\n2,,failed,required\n',{headers})});
 const controller=new AbortController();await seam(async({exposed,clicks})=>{const value=await client.downloadImportFailures({store:id,batch:id,boundary:await settings.sessionBoundary(),signal:controller.signal});assert.equal(value,'uncertain');assert.deepEqual(exposed,[]);assert.deepEqual(clicks,[]);},{upstream:()=>new Response('\ufeffrow,external_id,outcome,code\n2,,failed,required\n',{headers}),browserResponse:(response)=>{controller.abort();return response;}});
});

test('actual route refuses unknown resource/query and duplicate session cookies before any upstream call',async()=>{
 await seam(async({calls})=>{
  assert.equal((await route.POST(upload(),context('unknown'))).status,404);
  const query=new Request(`${origin}/api/stores/${id}/imports/customers/preview?tenant_id=synthetic`,{method:'POST',headers:{Origin:origin,Cookie:cookie(),'X-CSRF-Token':csrf,'Content-Type':'text/csv'},body:'CSV'});
  assert.equal((await route.POST(query,context())).status,422);
  const duplicate=upload('preview',{headers:{Cookie:`${cookie()}; ${auth.SESSION_COOKIE}=${token}`}});
  const denied=await route.POST(duplicate,context());assert.equal(denied.status,401);assert.ok(denied.headers.get('set-cookie')?.includes('Max-Age=0'));assert.equal(calls.length,0);
 });
});

test('actual route incoming cancellation aborts one dispatched transport and preserves client UNKNOWN',async()=>{
 const controller=new AbortController();await seam(async({calls})=>{
  const value=await client.sendImport(await input({signal:controller.signal}));assert.equal(value.kind,'error');assert.equal(value.uncertain,true);assert.equal(calls.length,2);assert.equal(calls[1].init.signal.aborted,true);
 },{upstream:async(_url,init)=>{queueMicrotask(()=>controller.abort());await new Promise((_resolve,reject)=>init.signal.addEventListener('abort',()=>reject(Error('cancelled')),{once:true}));}});
});

test('real coded error cache may not be public, and closed payload may not acquire data-bearing fields',async()=>{
 for(const change of ['public','extra','retryable'])await seam(async()=>{
  const value=await client.sendImport(await input());assert.equal(value.kind,'error');assert.equal(value.code,'retry_later');assert.equal(value.uncertain,true);
 },{upstream:()=>json(receipt),browserResponse:async()=>{
  const response=auth.localError(403,'forbidden'),value=await response.json();
  if(change==='extra')value.customer='synthetic-cell';if(change==='retryable')value.retryable=true;
  return Response.json(value,{status:403,headers:{'Cache-Control':change==='public'?'public, no-store':'private, no-store'}});
 }});
});

test('copy still exposes specific merchant field reasons across three locales',async()=>{
 const {importCopy:c}=await import('../../apps/admin/lib/import-copy.ts');
 for(const locale of ['zh-TW','zh-CN','en']){assert.deepEqual(Object.keys(c[locale]).sort(),Object.keys(c.en).sort());
  assert.equal(new Set(['invalid_phone','invalid_email','invalid_date','invalid_amount','invalid_quantity','invalid_external_id'].map(k=>c[locale].errors[k])).size,6);
 }
});

// The previous fix already preserved coded errors. This new red is only the import-leaf cache contract.
test('all real local and upstream import refusals including HEAD/unsupported are exact private,no-store',async()=>{
 await seam(async({calls})=>{
  const cases=[
   [()=>route.POST(upload('preview',{headers:{Cookie:''}}),context()),401],
   [()=>route.POST(upload('preview',{headers:{Origin:'https://attacker.invalid'}}),context()),403],
   [()=>route.POST(upload(),context('unknown')),404],
   [()=>route.POST(new Request(`${origin}/api/stores/${id}/imports/customers/preview?bad=1`,{method:'POST',headers:{'Content-Type':'text/csv'},body:'CSV'}),context()),422],
  ];
  for(const[run,status]of cases){const response=await run();assert.equal(response.status,status);assert.equal(response.headers.get('cache-control'),'private, no-store');}
  for(const method of ['HEAD','PUT','PATCH','DELETE','OPTIONS']){const response=await route[method](new Request(`${origin}/api/stores/${id}/imports/customers/preview`,{method}),context());assert.equal(response.status,405);assert.equal(response.headers.get('cache-control'),'private, no-store');}
  assert.equal(calls.length,0);
 });
});

test('fresh mismatched receipt stays UNKNOWN, exact manual replay mismatch becomes terminal without a third send',async()=>{
 let sent=0;const mismatch={...receipt,created:0,failed:1};
 await seam(async({calls})=>{
  const intent=await input(),file=intent.file,mapping=JSON.stringify(intent.mapping);
  const first=await client.sendImport(intent);assert.deepEqual(first,{kind:'error',code:'retry_later',uncertain:true});assert.equal(sent,1);
  // Explicit merchant action: retry precisely the original file, mapping and expected row count.
  const replay=await client.sendImport(intent);assert.deepEqual(replay,{kind:'terminal',code:'receipt_mismatch',value:{...mismatch,replayed:true}});
  assert.equal(intent.file,file);assert.equal(JSON.stringify(intent.mapping),mapping);assert.equal(sent,2);assert.equal(calls.length,4);
  await Promise.resolve();assert.equal(sent,2);
 },{upstream:(_url,init)=>{sent++;assert.equal(new URL(_url).searchParams.get('expected_apply_rows'),'1');assert.deepEqual(JSON.parse(new URL(_url).searchParams.get('mapping')),raw.mapping);return json({...mismatch,replayed:sent>1});}});
});

test('terminal is impossible for malformed replay receipts or matching replayed counts',async()=>{
 for(const payload of [{...receipt,replayed:true},{...receipt,replayed:true,created:0,raw_cell:'bad'}])await seam(async()=>{
  const value=await client.sendImport(await input());assert.equal(value.kind,payload.raw_cell?'error':'receipt');if(value.kind==='error')assert.equal(value.uncertain,true);
 },{upstream:()=>json(payload)});
});

test('Big5 bytes pass unchanged to upstream and real422 import envelope surfaces encoding_not_utf8',async()=>{
 const bytes=new Uint8Array([0xa4,0xa4,44,78,97,109,101,10]);
 await seam(async({browserResponses})=>{
  const result=await client.sendImport(await input({action:'preview',expectedApplyRows:undefined,file:new Blob([bytes])}));
  assert.deepEqual(result,{kind:'error',code:'encoding_not_utf8',uncertain:false});assert.equal(browserResponses[0].status,422);assert.equal(browserResponses[0].headers.get('cache-control'),'private, no-store');
 },{upstream:(_url,init)=>{assert.deepEqual([...init.body],[...bytes]);return json({code:'encoding_not_utf8'},422);}});
});
