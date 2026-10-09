// Purpose: actual W3-U2 controller admission tests for uncertain commands and stale event handlers.
// Depends on: real-source inbox hook host, live-settings controller/client and Node fake network edge.
// Used by: test-node; MOCK lifecycle evidence complements the production browser gate.
import test from "node:test";
import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import {environment,store,response} from "./inbox-review-host.test.ts";
registerHooks({resolve(specifier,context,next){
  if(specifier==="next/navigation")return {url:"data:text/javascript,export function useRouter(){return {push(){}}}",shortCircuit:true};
  return next(specifier,context);
}});
const {useLiveSettingsController}=await import("../../apps/admin/lib/live-settings-controller.ts");

test("W3-U2 UNKNOWN rejects a stale fresh command and admits only exact receipt retry",async t=>{
 const env=environment(t), writes:{key:string|null;body:unknown}[]=[];
 globalThis.fetch=async (_input,init)=>{
  if(init?.method==="POST"){
   writes.push({key:new Headers(init.headers).get("Idempotency-Key"),body:init.body});
   return response({code:"unavailable"},503);
  }
  return response({items:[],next_cursor:""});
 };
 const h=env.mount(()=>useLiveSettingsController({store,locale:"en",scene:"",initialError:null}));await h.settle();
 const first={method:"POST" as const,resource:`live-sessions/${store.id}/reminders`,key:"unknown-original-key"};
 const staleRun=h.output.run;
 await staleRun(first);await h.settle();assert.equal(h.output.unknown,true);assert.equal(writes.length,1);
 await staleRun({...first,key:"fresh-stale-key"});await h.settle();assert.equal(writes.length,1,"UNKNOWN must not admit a new key from an old callback");
 assert.equal(h.output.receipt.current,first);
 await h.output.run(h.output.receipt.current);await h.settle();assert.equal(writes.length,2);assert.deepEqual(writes[1],writes[0]);
});

for(const boundary of ["hide","departure","unmount"] as const)test(`W3-U2 captured command cannot dispatch after ${boundary}`,async t=>{
 const env=environment(t);let writes=0;
 globalThis.fetch=async (_input,init)=>{if(init?.method==="POST"){writes++;return response({code:"unavailable"},503);}return response({items:[],next_cursor:""});};
 const h=env.mount(()=>useLiveSettingsController({store,locale:"en",scene:"",initialError:null}));await h.settle();const staleRun=h.output.run;
 if(boundary==="hide"){env.document.visibilityState="hidden";env.document.dispatchEvent(new Event("visibilitychange"));}
 else if(boundary==="departure")h.output.privacy.suspend();else h.dispose();
 await staleRun({method:"POST",resource:`live-sessions/${store.id}/reminders`,key:"departed-callback-key"});
 assert.equal(writes,0,"revoked fence must refuse before any transport side effect");
});

test("W3-U2 captured BuyerPanel add cannot dispatch after native hide",async t=>{
 const {BuyerPanel,node}=await import("./inbox-review-host.test.ts");const env=environment(t);let writes=0;
 globalThis.fetch=async(input,init)=>{
  if(init?.method==="POST"){writes++;return response({code:"unavailable"},503);}
  return String(input).includes("blocklist/check")?response({restricted:false}):response({display_name:"Synthetic",platform:"facebook",purchase_ordinal:1,claims:[],claim_total_minor:0,orders:[],link_pending_manual:false});
 };
 const h=env.mount(()=>BuyerPanel({store,locale:"en",bundleId:store.id,sessionId:store.id}));await h.settle();
 const staleAdd=node(h,n=>n.props["data-testid"]==="buyer-block-add").props.onClick;
 env.document.visibilityState="hidden";env.document.dispatchEvent(new Event("visibilitychange"));
 await staleAdd();await h.settle();assert.equal(writes,0);
});
