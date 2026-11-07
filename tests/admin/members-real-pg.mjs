// Purpose: exercise actual member route exports against the Go/PG fixture, with no mocked network.
// Depends on: parent TestLoyaltyBFFRealPG synthetic authority and Node's existing test resolver pattern.
// Used by: focused REAL_PG seam gate; never prints credentials or request bodies.
import assert from 'node:assert/strict';
import { randomBytes, randomUUID } from 'node:crypto';
import { registerHooks } from 'node:module';
import { fileURLToPath, pathToFileURL } from 'node:url';
const root=fileURLToPath(new URL('../../apps/admin/',import.meta.url));
registerHooks({resolve(s,c,next){
 if(s==='server-only')return {url:'data:text/javascript,',shortCircuit:true};
 if(s==='next/headers')return {url:'data:text/javascript,export function headers(){throw Error("unexpected ambient session")}',shortCircuit:true};
 if(s.startsWith('@/'))return {url:pathToFileURL(root+s.slice(2)+'.ts').href,shortCircuit:true};
 if(s.startsWith('.')&&c.parentURL?.includes('/apps/admin/lib/')&&!s.endsWith('.ts'))return next(s+'.ts',c);
 return next(s,c);
}});
const publicOrigin='https://admin.example.test',store=process.env.LP_BFF_STORE,csrf=randomBytes(32).toString('base64url');
Object.assign(process.env,{COMMERCE_IDENTITY_ENABLED:'1',COMMERCE_PASSWORD_LOGIN_ENABLED:'1',COMMERCE_BFF_KEY:randomBytes(32).toString('base64url'),COMMERCE_PUBLIC_ORIGIN:publicOrigin,COMMERCE_API_ORIGIN:process.env.LP_BFF_ORIGIN});
const create=await import(pathToFileURL(root+'app/api/stores/[store]/members/route.ts').href);
const search=await import(pathToFileURL(root+'app/api/stores/[store]/members/search/route.ts').href);
const adjust=await import(pathToFileURL(root+'app/api/stores/[store]/members/[member]/adjustments/route.ts').href);
async function call(route,body,{selected=store,session=process.env.LP_BFF_SESSION,member,search=false,key='real-members-key-123'}={}){
 const headers={'content-type':'application/json',origin:publicOrigin,'x-csrf-token':csrf,cookie:`__Host-commerce_session=${session}; __Host-commerce_csrf=${csrf}`};
 if(!search)headers['idempotency-key']=key;
 return route.POST(new Request(publicOrigin+'/api/stores/'+selected+'/members',{method:'POST',headers,body:typeof body==='string'?body:JSON.stringify(body)}),{params:Promise.resolve({store:selected,member})});
}
let response=await call(create,{display_name:'測試會員 seam',contact_note:'synthetic contact',notes:null});assert.equal(response.status,201,'create commits through BFF');const first=await response.json();const id=first.member.id;
response=await call(create,{display_name:'測試會員 seam',contact_note:'synthetic contact',notes:null});assert.equal(response.status,200,'same-key replay');assert.equal((await response.json()).member.id,id);
response=await call(search,{q:'測試會員 seam',include_erased:false,cursor:null,limit:10},{search:true});assert.equal(response.status,200,'search body');assert.equal((await response.json()).members.length,1);
response=await call(adjust,{delta:5,note:'synthetic',order_id:null},{member:id,session:process.env.LP_BFF_READONLY,key:'real-readonly-key'});assert.equal(response.status,403,'PG permission refusal');assert.equal((await response.json()).code,'forbidden');
response=await call(create,{display_name:'synthetic',tenant_id:randomUUID()},{key:'real-unknown-key'});assert.equal(response.status,400,'strict unknown key');await response.text();
response=await call(create,'{"display_name":"synthetic","display_name":"duplicate"}',{key:'real-duplicate-key'});assert.equal(response.status,400,'strict duplicate key');await response.text();
response=await call(create,{display_name:'synthetic'},{selected:randomUUID(),key:'real-crossstore-key'});assert.equal(response.status,404,'cross-store opaque refusal');await response.text();
console.log('REAL_PG BFF create/replay/search/forbidden/unknown/duplicate/cross-store passed (7 assertions)');
