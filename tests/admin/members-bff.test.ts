// Purpose: LP-1 BFF route seam gates with real auth/helpers and only a fake upstream network.
// Depends on: Node test runner, HTTP loopback and the actual eight Next route exports.
// Used by: D2 focused acceptance; MOCK, not REAL_PG or browser acceptance.
import assert from 'node:assert/strict';
import { test, after } from 'node:test';
import { createServer } from 'node:http';
import { randomBytes, randomUUID } from 'node:crypto';
import { registerHooks } from 'node:module';
import { fileURLToPath, pathToFileURL } from 'node:url';
const root = fileURLToPath(new URL('../../apps/admin/', import.meta.url));
registerHooks({ resolve(s, c, next) {
  if (s === 'server-only') return { url: 'data:text/javascript,', shortCircuit: true };
  if (s === 'next/headers') return { url: 'data:text/javascript,export function headers(){throw Error("unexpected ambient session")}', shortCircuit: true };
  if (s.startsWith('@/')) return { url: pathToFileURL(root+s.slice(2)+'.ts').href, shortCircuit: true };
  if (s.startsWith('.') && c.parentURL?.includes('/apps/admin/lib/') && !s.endsWith('.ts')) return next(s+'.ts', c);
  return next(s,c);
}});
const store = randomUUID(), member = randomUUID(), token = randomBytes(32).toString('base64url'), csrf = randomBytes(32).toString('base64url');
const origin = 'https://admin.example.test';
const seen: {url:string; method:string; body:string; authorization:string; key:string}[] = [];
let status = 201, payload = '{"member":{"id":"'+member+'"}}';
const upstream = createServer((req,res)=> {
  let body='';req.on('data', x=>body+=x);req.on('end',()=> {
    if (req.url === '/v1/admin/stores') { res.writeHead(200,{'content-type':'application/json'}); res.end(JSON.stringify({items:[{id:store,name:'Synthetic',currency:'TWD'}]}));return; }
    seen.push({url:req.url!,method:req.method!,body,authorization:req.headers.authorization??'',key:String(req.headers['idempotency-key']??'')});
    res.writeHead(status,{'content-type':'application/json'});res.end(payload);
  });
});
await new Promise<void>(ok=>upstream.listen(0,'127.0.0.1',ok));
Object.assign(process.env,{COMMERCE_IDENTITY_ENABLED:'1',COMMERCE_PASSWORD_LOGIN_ENABLED:'1',COMMERCE_BFF_KEY:randomBytes(32).toString('base64url'),COMMERCE_PUBLIC_ORIGIN:origin,COMMERCE_API_ORIGIN:`http://127.0.0.1:${(upstream.address() as {port:number}).port}`});
after(()=>upstream.close());
const cases = [ ['', 'POST', {display_name:'測試會員',contact_note:null,notes:null}], ['search','POST',{q:'Synthetic private search',include_erased:false,cursor:null,limit:20}], ['[member]','GET',null], ['[member]','PATCH',{expected_version:1,notes:null}], ['[member]/adjustments','POST',{delta:10,note:'Synthetic private note',order_id:null}], ['entries','GET',null], ['[member]/export','POST',{}], ['[member]/erasure','POST',{}] ] as const;
async function call(path:string,method:string,body:unknown, opts:{query?:string;store?:string;csrf?:boolean;session?:boolean;mime?:string;raw?:string}={}) {
 const route=await import(pathToFileURL(root+'app/api/stores/[store]/members/'+(path?path+'/':'')+'route.ts').href);
 const headers:Record<string,string>={origin,'content-type':opts.mime??'application/json',cookie:`${opts.session===false?'':'__Host-commerce_session='+token+'; '}__Host-commerce_csrf=${csrf}`};
 if(opts.csrf!==false) headers['x-csrf-token']=csrf;
 if(method!=='GET'&&path!=='search') headers['idempotency-key']='members-samekey-123';
 const query=opts.query??(path==='entries'?'?order_id='+member:'');
 const request=new Request(origin+'/api/stores/'+(opts.store??store)+'/members/'+path.replace('[member]',member)+query,{method,headers,...(method==='GET'?{}:{body:opts.raw??JSON.stringify(body)})});
 return route[method](request,{params:Promise.resolve({store:opts.store??store,member:path.includes('[member]')?member:undefined})}) as Promise<Response>;
}
test('all eight actual leaves preserve method/status/raw body/key; search PII only in body',async()=>{
 for(const [path,method,body] of cases){status=path===''||path.endsWith('adjustments')?201:200; const before=seen.length;const response=await call(path,method,body);assert.equal(response.status,status,path);assert.equal(seen.length,before+1);const sent=seen.at(-1)!;assert.equal(sent.method,method);assert.equal(sent.authorization,'Bearer '+token);assert.equal(sent.body,method==='GET'?'':JSON.stringify(body));assert.equal(sent.key,method==='GET'||path==='search'?'':'members-samekey-123');assert.ok(!sent.url.includes('Synthetic'));assert.match(response.headers.get('cache-control')! ,/no-store/);if(path.endsWith('export'))assert.match(response.headers.get('content-disposition')!,/^attachment; filename="member-/);await response.text();}
});
test('unauthenticated/cross-store/CSRF/query/MIME/malformed JSON refusals never forward members',async()=>{
 const before=seen.length;
 for(const [options,expected] of [[{session:false},401],[{csrf:false},403],[{store:randomUUID()},404],[{query:'?q=private'},422],[{mime:'text/plain'},415],[{raw:'{broken'},400]] as const){const r=await call('search','POST',{},options);assert.equal(r.status,expected);await r.text();}
 assert.equal(seen.length,before);
});
test('permission/refusal status and code preserved safely; never retries or leaks upstream diagnostics',async()=>{
 for(const [code,n] of [['forbidden',403],['points_insufficient',409],['member_erased',409],['export_too_large',409],['limit_reached',422],['retry_later',503]] as const){ status=n;payload=JSON.stringify({code,message:'Synthetic private diagnostic'});const before=seen.length;const r=await call('[member]/adjustments','POST',{delta:1,note:'test'});assert.equal(r.status,n);const text=await r.text();assert.equal(JSON.parse(text).code,code);assert.ok(!text.includes('Synthetic private'));assert.equal(seen.length,before+1);assert.equal(seen.at(-1)!.key,'members-samekey-123');}
});
test('manual BFF admits only frozen optional member_award with/without discount',async()=>{
 const {validManualBody}=await import('../../apps/admin/lib/merchant-tools-model.ts');
 const base={customer:{name:'Synthetic'},delivery:{},items:[],locale:'en',payment_mode:'cash_on_delivery'};
 assert.equal(validManualBody(base),true);
 assert.equal(validManualBody({...base,member_award:{member_id:member,points:5}}),true);
 for(const award of [null,{}, {member_id:member,points:0},{member_id:member,points:1.5},{member_id:member,points:1000001},{member_id:member,points:5,balance:10},{member_id:'foreign input',points:5}]) assert.equal(validManualBody({...base,member_award:award}),false);
});
