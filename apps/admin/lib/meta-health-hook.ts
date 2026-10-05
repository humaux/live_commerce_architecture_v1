// Purpose: bounded, visible-page health polling scoped to the exact store; stale store reads never render.
// Depends on: React and private readMetaHealth; pageshow/focus and explicit health-change notifications.
// Used by: the workspace banner and MetaConnect capability section; no writes.
"use client";
import { useEffect, useState } from "react";
import { healthChanged, readMetaHealth } from "./meta-health-client";
import type { MetaHealth } from "./meta-health-model";
/** Reads the current store only, cancels superseded requests and clears advice after any failed fetch. */
export function useMetaHealth(store: string | null, revision = 0) {
 const [value,setValue]=useState<{store:string;data:MetaHealth|null;failed:boolean}|null>(null);
 useEffect(()=>{
  if(!store)return;
  let live=true,request:AbortController|undefined;
  const load=async()=>{
   request?.abort();const active=new AbortController();request=active;
   try {const data=await readMetaHealth(store,AbortSignal.any([active.signal,AbortSignal.timeout(10000)]));if(live&&!active.signal.aborted)setValue({store,data,failed:false});}
   catch {if(live&&!active.signal.aborted)setValue({store,data:null,failed:true});}
  };
  const visible=()=>{if(document.visibilityState==="visible")void load();};
  const changed=(event:Event)=>{if((event as CustomEvent<unknown>).detail===store)void load();};
  void load();const interval=window.setInterval(visible,30000);
  window.addEventListener("pageshow",visible);window.addEventListener("focus",visible);window.addEventListener(healthChanged,changed);
  return()=>{live=false;request?.abort();clearInterval(interval);window.removeEventListener("pageshow",visible);window.removeEventListener("focus",visible);window.removeEventListener(healthChanged,changed);};
 },[store,revision]);
 return value?.store===store?value:{store,data:null,failed:false};
}
