// Purpose: actual W3-U2 controller admission tests for uncertain commands and stale event handlers.
// Depends on: real-source inbox hook host, live-settings controller/client and Node fake network edge.
// Used by: test-node; MOCK lifecycle evidence complements the production browser gate.
import test from "node:test";
import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import {environment,store,response,node,textOf} from "./inbox-review-host.test.ts";
registerHooks({resolve(specifier,context,next){
  // Presentation useId is renderer plumbing; keep all privacy/controller hooks real.
  if(specifier==="react")return {url:"data:text/javascript,const r=globalThis.__inboxReviewRuntime;export const {useState,useRef,useCallback,useEffect}=r;export const useId=()=>r.useRef('mock-live-settings-id').current;",shortCircuit:true};
  if(specifier==="next/navigation")return {url:"data:text/javascript,export function useRouter(){return globalThis.__liveSettingsRouter??{push(){}}}export const usePathname=()=>'/en/studio/settings';export const useSearchParams=()=>new URLSearchParams();export const useParams=()=>({locale:'en'});",shortCircuit:true};
  return next(specifier,context);
}});
const {useLiveSettingsController}=await import("../../apps/admin/lib/live-settings-controller.ts");
const {LiveSettings}=await import("../../apps/admin/components/LiveSettings.tsx");

const sid="00000000-0000-4000-8000-000000000001";
const drafts=Array.from({length:21},(_,i)=>({session_id:`00000000-0000-4000-8000-${String(i+1).padStart(12,"0")}`,program_id:store.id,title:`MOCK scene ${i+1}`,scheduled_at:null,aspect_ratio:"16:9",state:"DRAFT",version:1,created_at:"2030-01-01T00:00:00Z",updated_at:"2030-01-01T00:00:00Z"}));
const allStore={...store,permissions:["live:read","live:manage","inbox:read","inbox:reply"]};
const noReportStore={...allStore,role:"staff",permissions:["live:read","live:manage","inbox:reply"]};
const report={sent:[],queued:0,failed:[],followup:[],link:null};
const queued={queued:2,already_reminded:1,refused:0,followup:0,restricted:0,truncated:false};
function reads(input:RequestInfo|URL) {
 const url=String(input);
 if(url.includes("sold-out-reply"))return response({enabled:true,template_id:"merchant-reply",template_version:1,version:3});
 if(url.includes("claims/blocklist"))return response({items:[],next_cursor:""});
 if(url.includes("/reminders"))return response(report);
 const studio=(value:unknown)=>{const r=response(value);r.headers.set("Cache-Control","private, no-store");return r;};
 if(url.endsWith(drafts[20].session_id))return studio({draft:drafts[20],prepared:null,attempt:null,can_manage:true,media_enabled:false});
 return studio({items:url.includes("cursor=")?[drafts[20]]:drafts.slice(0,20),next_cursor:url.includes("cursor=")?"":"page-two"});
}

test("PR31 split permissions actual reminder click retains queued receipt without unauthorized report GET",async t=>{
 const env=environment(t);let posts=0,gets=0;
 globalThis.fetch=async(input,init)=>{
  if(String(input).includes("/reminders")){if(init?.method==="POST"){posts++;return response(queued);}gets++;return response({code:"forbidden"},403);}
  return reads(input);
 };
 const h=env.mount(()=>LiveSettings({store:noReportStore,locale:"en",scene:sid,initialError:null}));
 await h.waitFor(()=>!node(h,n=>n.props["data-testid"]==="reminder-trigger").props.disabled,"actual reminder control ready");
 node(h,n=>n.props["data-testid"]==="reminder-trigger").props.onClick();
 await h.waitFor(()=>posts===1&&(gets>0||textOf(h.output).includes("This request")),"exact successful reminder completion or unauthorized follow-up read");
 assert.equal(gets,0,"report permission must be checked before transport");
 const text=textOf(h.output).replace(/\s+/g," ");assert.match(text,/Queued 2/);assert.match(text,/Already reminded 1/);
 assert.ok(node(h,n=>n.props["data-testid"]==="scene-select"));
});

test("PR31 optional reminder GET 404 preserves healthy settings and blocklist",async t=>{
 const env=environment(t);globalThis.fetch=async input=>String(input).includes("/reminders")?response({code:"reminders_unavailable"},404):reads(input);
 const h=env.mount(()=>useLiveSettingsController({store:allStore,locale:"en",scene:sid,initialError:null}));await h.settle();
 assert.equal(h.output.settings?.version,3);assert.deepEqual(h.output.blocked?.items,[]);
 assert.equal(h.output.remindersUnavailable,true);assert.equal(h.output.privacy.visible,true);assert.equal(h.output.privacy.blocked.current,false);
});

test("PR31 missing reminder POST is local even when staff cannot discover it with a report read",async t=>{
 const env=environment(t);globalThis.fetch=async(input,init)=>init?.method==="POST"?response({code:"reminders_unavailable"},404):reads(input);
 const h=env.mount(()=>useLiveSettingsController({store:noReportStore,locale:"en",scene:sid,initialError:null}));await h.waitFor(()=>!h.output.loading&&h.output.settings!==null,"staff settings ready");
 await h.output.run({method:"POST",resource:`live-sessions/${sid}/reminders`,key:"missing-reminder-route"});h.flush();
 assert.equal(h.output.settings?.version,3);assert.deepEqual(h.output.blocked?.items,[]);assert.equal(h.output.remindersUnavailable,true);assert.equal(h.output.privacy.blocked.current,false);
});

test("PR31 actual authorized reminder read 403 still tears down all private state",async t=>{
 const env=environment(t);globalThis.fetch=async input=>String(input).includes("/reminders")?response({code:"forbidden"},403):reads(input);
 const h=env.mount(()=>useLiveSettingsController({store:allStore,locale:"en",scene:sid,initialError:null}));await h.settle();
 assert.equal(h.output.privacy.blocked.current,true);assert.equal(h.output.settings,null);assert.equal(h.output.blocked,null);assert.equal(h.output.result,null);
});

test("PR31 same settings destination preserves the actual scene selector",async t=>{
 const env=environment(t);globalThis.fetch=async input=>reads(input);
 const h=env.mount(()=>LiveSettings({store:allStore,locale:"en",scene:"",initialError:null}));await h.waitFor(()=>!node(h,n=>n.props["data-testid"]==="scene-select").props.disabled,"scene selector ready");
 assert.equal(h.raw.props.onBeforeNavigate(`/en/studio/settings?store=${store.id}`),true);
 h.flush();assert.equal(node(h,n=>n.props["data-testid"]==="scene-select").props.disabled,false,"no replacement means privacy remains live");
});

test("PR31 same valid settings URL without store keeps the resolved default-store lifecycle",async t=>{
 const env=environment(t);Object.assign(env.window,{location:{href:"https://local.invalid/en/studio/settings"}});globalThis.fetch=async input=>reads(input);
 const h=env.mount(()=>LiveSettings({store:allStore,locale:"en",scene:"",initialError:null}));await h.waitFor(()=>!node(h,n=>n.props["data-testid"]==="scene-select").props.disabled,"default-store selector ready");
 h.raw.props.onBeforeNavigate("/en/studio/settings");h.flush();assert.equal(node(h,n=>n.props["data-testid"]==="scene-select").props.disabled,false);
});

for(const destination of [`/en/studio/settings?store=${store.id}&scene=${drafts[20].session_id}`,`/en/studio/settings?store=${drafts[20].session_id}`,`/zh-CN/studio/settings?store=${store.id}`])test(`PR31 actual navigation synchronously clears old private state: ${destination}`,async t=>{
 const env=environment(t);let release!:(r:Response)=>void;const held=new Promise<Response>(resolve=>release=resolve);let started=false;
 globalThis.fetch=async input=>{if(String(input).includes("/reminders")){started=true;return held;}return reads(input);};
 const h=env.mount(()=>LiveSettings({store:allStore,locale:"en",scene:sid,initialError:null}));await h.waitFor(()=>started,"report read held");
 h.raw.props.onBeforeNavigate(destination);h.flush();assert.equal(textOf(h.output).includes("PRIVATE_LATE_REPORT"),false);
 release(response({...report,followup:[{bundle_id:store.id,display_name:"PRIVATE_LATE_REPORT",reminder_state:"followup",reason:"window_closed",link_copy_allowed:false}]}));await held;await h.settle();
 assert.equal(textOf(h.output).includes("PRIVATE_LATE_REPORT"),false);assert.equal(h.output.props?.children?.includes?.("PRIVATE_LATE_REPORT")??false,false);
 assert.match(textOf(h.output),/Private data was cleared|hidden/);
});

test("PR31 more than twenty scenes load through the actual selector and selected scene transport",async t=>{
 const env=environment(t);const paths:string[]=[],pushed:string[]=[];(globalThis as any).__liveSettingsRouter={push:(url:string)=>pushed.push(url)};t.after(()=>delete(globalThis as any).__liveSettingsRouter);
 globalThis.fetch=async(input,init)=>{paths.push(String(input));if(init?.method==="POST")return response(queued);return reads(input);};
 let scene="";const h=env.mount(()=>LiveSettings({store:allStore,locale:"en",scene,initialError:null}));await h.waitFor(()=>!node(h,n=>n.props["data-testid"]==="scene-select").props.disabled,"first page loaded");
 await node(h,n=>n.props["data-testid"]==="scene-more").props.onClick();
 await h.waitFor(()=>textOf(h.output).includes("MOCK scene 21"),"second page scene visible");
 node(h,n=>n.props["data-testid"]==="scene-select").props.onChange({target:{value:drafts[20].session_id}});
 assert.equal(pushed.at(-1),`/en/studio/settings?store=${store.id}&scene=${drafts[20].session_id}`);
 assert.ok(paths.some(x=>x.includes("cursor=page-two")));
 h.dispose();scene=drafts[20].session_id;
 const selected=env.mount(()=>LiveSettings({store:allStore,locale:"en",scene,initialError:null}));await selected.waitFor(()=>!node(selected,n=>n.props["data-testid"]==="reminder-trigger").props.disabled,"explicit scene loaded");
 assert.ok(paths.some(x=>x.includes(`/live-sessions/${scene}/claims/blocklist`)));assert.ok(paths.some(x=>x.includes(`/live-sessions/${scene}/reminders`)));
 node(selected,n=>n.props["data-testid"]==="reminder-trigger").props.onClick();await selected.waitFor(()=>textOf(selected.output).includes("This request"),"second scene receipt");
});

test("PR31 deep linked later scene is a real titled option and empty placeholder cannot be selected",async t=>{
 const env=environment(t);globalThis.fetch=async input=>reads(input);
 const h=env.mount(()=>LiveSettings({store:allStore,locale:"en",scene:drafts[20].session_id,initialError:null}));await h.waitFor(()=>!node(h,n=>n.props["data-testid"]==="scene-select").props.disabled,"deep linked scene loaded");
 assert.equal(node(h,n=>n.type==="option"&&n.props.value===drafts[20].session_id).props.children,"MOCK scene 21");
 assert.equal(node(h,n=>n.type==="option"&&n.props.value==="").props.disabled,true,"selected scene placeholder must be disabled rather than a no-op");
});

test("PR31 selected first-page scene disables the empty no-op option",async t=>{
 const env=environment(t);globalThis.fetch=async input=>reads(input);
 const h=env.mount(()=>LiveSettings({store:allStore,locale:"en",scene:sid,initialError:null}));await h.waitFor(()=>!node(h,n=>n.props["data-testid"]==="scene-select").props.disabled,"existing selected scene ready");
 assert.equal(node(h,n=>n.type==="option"&&n.props.value==="").props.disabled,true);
});

test("PR31 scene selector has an explicit accessible label outside its option ancestry",async t=>{
 const env=environment(t);globalThis.fetch=async input=>reads(input);
 const h=env.mount(()=>LiveSettings({store:allStore,locale:"en",scene:sid,initialError:null}));await h.waitFor(()=>!node(h,n=>n.props["data-testid"]==="scene-select").props.disabled,"selector ready");
 const select=node(h,n=>n.props["data-testid"]==="scene-select");
 assert.equal(typeof select.props.id,"string","explicit label association requires an id");assert.ok(select.props.id);
 const label=node(h,n=>n.type==="label"&&n.props.htmlFor===select.props.id);assert.equal(label.props.children,"Choose a session");
 assert.equal((label.props.children as unknown)===select,false,"label must not enclose the select/options");
});

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

test("PR31 reminder scope 404 clears already loaded private sections",async t=>{
 const env=environment(t);let release!:(r:Response)=>void;
 const held=new Promise<Response>(resolve=>release=resolve);
 globalThis.fetch=async input=>String(input).includes("/reminders")?held:reads(input);
 const h=env.mount(()=>useLiveSettingsController({store:allStore,locale:"en",scene:sid,initialError:null}));
 await h.waitFor(()=>h.output.settings?.version===3&&h.output.blocked!==null,"healthy private sections before scope refusal");
 release(response({code:"not_found"},404));await held;await h.settle();
 assert.equal(h.output.settings,null,"a store/session refusal is not an absent optional route");
 assert.equal(h.output.blocked,null);assert.equal(h.output.sessions,null);assert.equal(h.output.result,null);
 assert.equal(h.output.remindersUnavailable,false);assert.equal(h.output.receipt.current,null);
});

for(const status of [404,503] as const)test(`PR31 Studio pagination ${status} ${status===404?"clears private page and fences late reads":"preserves authorized private state"}`,async t=>{
 const env=environment(t), entry={id:store.id,platform:"facebook",note:"MOCK private note",source_bundle_id:null,created_at:"2030-01-01T00:00:00Z"};
 let release!:(r:Response)=>void,entered!:()=>void,writes=0;
 const held=new Promise<Response>(resolve=>release=resolve),started=new Promise<void>(resolve=>entered=resolve);
 globalThis.fetch=async(input,init)=>{
  const url=String(input);
  if(init?.method==="POST"){writes++;return response(queued);}
  if(url.includes("claims/blocklist")){if(url.includes("cursor=")){entered();return held;}return response({items:[entry],next_cursor:"next-private-page"});}
  if(url.includes("cursor=page-two"))return response({code:status===404?"not_found":"unavailable"},status);
  return reads(input);
 };
 const h=env.mount(()=>useLiveSettingsController({store:allStore,locale:"en",scene:sid,initialError:null}));
 await h.waitFor(()=>!h.output.loading&&h.output.sessions?.items.length===20&&h.output.settings?.version===3&&h.output.report!==null&&h.output.blocked?.items.length===1,"loaded private page before Studio refusal");
 h.output.setDraft("MOCK private unsaved reply");h.output.setRemove(entry.id);h.flush();
 const ticket=h.output.privacy.fence.begin(),oldRun=h.output.run,late=h.output.page("next-private-page");
 await started;await h.output.loadMoreSessions();h.flush();
 try{
  if(status===404){
   for(const field of ["sessions","settings","report","result","blocked","remove"] as const)assert.equal(h.output[field],null,`${field} must clear on Studio not-found`);
   assert.equal(h.output.draft,"");assert.equal(h.output.receipt.current,null);assert.equal(h.output.enabled,false);
   assert.equal(h.output.privacy.fence.current(ticket),false);assert.equal(ticket.signal.aborted,true);
   await oldRun({method:"POST",resource:`live-sessions/${sid}/reminders`,key:"MOCK-discarded-scope"});assert.equal(writes,0);
  }else{
   assert.equal(h.output.settings?.version,3);assert.equal(h.output.sessions?.items.length,20);assert.equal(h.output.draft,"MOCK private unsaved reply");
   assert.equal(h.output.privacy.fence.current(ticket),true);
  }
  assert.equal(h.output.error,status===404?"not-found":"unavailable");assert.equal(h.output.privacy.blocked.current,false);
 }finally{release(response({items:[{...entry,note:"MOCK late private note"}],next_cursor:""}));await late;h.flush();}
 if(status===404){assert.equal(h.output.blocked,null,"a late accepted response cannot repaint cleared private data");assert.equal(h.output.sessions,null);}
 else assert.equal(h.output.blocked?.items[0].note,"MOCK late private note");
});

for(const action of ["run","save","copy","page","loadMoreSessions"] as const)test(`PR31 old ${action} callback cannot borrow a revealed page's new fence`,async t=>{
 const env=environment(t);let writes=0,copies=0,readsAfterReveal=0,recordReads=false;
 const navigatorDescriptor=Object.getOwnPropertyDescriptor(globalThis,"navigator");
 Object.defineProperty(globalThis,"navigator",{configurable:true,value:{clipboard:{async writeText(){copies++;}}}});
 t.after(()=>navigatorDescriptor?Object.defineProperty(globalThis,"navigator",navigatorDescriptor):Reflect.deleteProperty(globalThis,"navigator"));
 globalThis.fetch=async(input,init)=>{
  if(init?.method==="POST"||init?.method==="PUT"){writes++;return response(queued);}
  if(recordReads)readsAfterReveal++;
  return reads(input);
 };
 const h=env.mount(()=>useLiveSettingsController({store:allStore,locale:"en",scene:sid,initialError:null}));
 await h.waitFor(()=>!h.output.loading&&h.output.settings?.version===3,"ready private settings");
 h.output.setDraft("MOCK old private draft");h.flush();
 const stale=h.output[action],before=h.output.privacy.fence.begin();
 env.document.visibilityState="hidden";env.document.dispatchEvent(new Event("visibilitychange"));h.flush();
 assert.equal(h.output.draft,"");assert.equal(h.output.settings,null);
 env.document.visibilityState="visible";env.document.dispatchEvent(new Event("visibilitychange"));
 await h.waitFor(()=>h.output.privacy.visible&&!h.output.loading&&h.output.settings?.version===3,"new authoritative settings after reveal");
 assert.equal(h.output.privacy.fence.current(before),false);
 recordReads=true;
 if(action==="run")await stale({method:"POST",resource:`live-sessions/${sid}/reminders`,key:"old-visible-epoch"});
 else if(action==="copy")await stale("https://shop.example.invalid/claims/MOCK-capability");
 else if(action==="page")await stale("old-private-cursor");
 else await stale();
 await h.settle();
 assert.equal(writes,0,"a captured callback cannot resend the discarded draft/command");
 assert.equal(copies,0,"a captured callback cannot copy a discarded private link");
 assert.equal(readsAfterReveal,0,"a captured callback cannot start stale pagination");
});

test("PR31 normal visible focus preserves captured command admission and UNKNOWN receipt",async t=>{
 const env=environment(t);const writes:{key:string|null;body:unknown}[]=[];
 globalThis.fetch=async(input,init)=>{
  if(init?.method==="POST"){writes.push({key:new Headers(init.headers).get("Idempotency-Key"),body:init.body});return response({code:"unavailable"},503);}
  return reads(input);
 };
 const h=env.mount(()=>useLiveSettingsController({store:allStore,locale:"en",scene:sid,initialError:null}));
 await h.waitFor(()=>!h.output.loading&&h.output.settings?.version===3,"ready visible settings");
 const run=h.output.run,first={method:"POST" as const,resource:`live-sessions/${sid}/reminders`,key:"normal-focus-same-key"},ticket=h.output.privacy.fence.begin();
 env.window.dispatchEvent(new Event("focus"));await h.settle();
 assert.equal(h.output.privacy.fence.current(ticket),true);
 await run(first);await h.settle();assert.equal(writes.length,1);assert.equal(h.output.unknown,true);assert.equal(h.output.receipt.current,first);
 await h.output.run(first);await h.settle();assert.equal(writes.length,2);assert.deepEqual(writes[0],writes[1]);
});
