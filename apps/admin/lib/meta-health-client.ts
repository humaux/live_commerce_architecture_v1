// Purpose: private health reads and explicit keyless scheduling with the current browser session fence.
// Depends on: authenticated BFF /api/stores/{store}/meta/health[/recheck] -> Go B1/B2; settings CSRF helpers.
// Used by: useMetaHealth and MetaHealthCapabilities; never initiates a provider call or automatic write retry.
import { read } from "./orders-client";
import { csrfCookie, sessionBoundary } from "./settings-client";
import { parseMetaHealth, parseRecheck } from "./meta-health-model";
export const healthChanged = "commerce-meta-health-changed";
/** Read only; a caller controls cancellation and hides failed advisory reads. */
export async function readMetaHealth(store: string, signal: AbortSignal) {
 return parseMetaHealth(await read(`/api/stores/${store}/meta/health`,signal));
}
/** Tells mounted readers to refresh this store only after an explicit scheduling/connection action. */
export function refreshMetaHealth(store: string) { window.dispatchEvent(new CustomEvent(healthChanged,{detail:store})); }
/** Schedules B2 once; a transport failure remains uncertain and is never retried autonomously. */
export async function recheckMetaHealth(store: string, page: string, boundary: string): Promise<{ok:true;next_check_at:string}|{ok:false;code:string;uncertain:boolean}> {
 const csrf=csrfCookie();
 try {if(!csrf||await sessionBoundary(csrf)!==boundary)throw new Error();}catch{return{ok:false,code:"unauthorized",uncertain:false};}
 let response:Response;
 try {response=await fetch(`/api/stores/${store}/meta/health/recheck`,{method:"POST",cache:"no-store",credentials:"same-origin",redirect:"error",signal:AbortSignal.timeout(10000),headers:{"Content-Type":"application/json","X-CSRF-Token":csrf},body:JSON.stringify({page_id:page})});}
 catch{return{ok:false,code:"unavailable",uncertain:true};}
 try {if(await sessionBoundary(csrf)!==boundary)throw new Error();}catch{return{ok:false,code:"unauthorized",uncertain:true};}
 const value:unknown=await response.json().catch(()=>null);
 if(response.status===202){try{return{ok:true,...parseRecheck(value)};}catch{return{ok:false,code:"unavailable",uncertain:true};}}
 const code=response.status===401?"unauthorized":response.status===403?"forbidden":value&&typeof value==="object"&&"code" in value&&value.code==="recheck_too_soon"?"recheck_too_soon":"unavailable";
 return{ok:false,code,uncertain:response.status>=500||response.status===408||response.ok};
}
