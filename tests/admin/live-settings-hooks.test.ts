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
