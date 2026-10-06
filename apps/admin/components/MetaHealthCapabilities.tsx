// Purpose: per-Page capability states and explicit recheck scheduling; missing identifiers remain unknown.
// Depends on: B1 health rows, keyless B2, sessionBoundary, existing displayTime and plain locale copy.
// Used by: MetaConnect connected Page cards; no provider send or automatic write retry.
"use client";
import { useEffect,useRef,useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { capabilities,type HealthPage } from "@/lib/meta-health-model";
import { metaHealthCopy,capabilityAdvice } from "@/lib/meta-health-copy";
import { recheckMetaHealth,refreshMetaHealth } from "@/lib/meta-health-client";
import { displayTime } from "@/lib/orders-model";
import "./meta-health.css";
/** Displays identified capabilities without promoting test access; a recheck receipt only means scheduled. */
export function MetaHealthCapabilities({store,page,locale,canManage,boundary,failed}:{store:string;page:HealthPage|undefined;locale:Locale;canManage:boolean;boundary:string;failed:boolean}) {
 const c=metaHealthCopy[locale],working=useRef(false),live=useRef(true);
 const [busy,setBusy]=useState(false),[message,setMessage]=useState<"scheduled"|"tooSoon"|"forbidden"|"signedOut"|"uncertain"|"failed"|null>(null);
 useEffect(()=>{live.current=true;return()=>{live.current=false;};},[]);
 async function recheck(){
  if(!page||!boundary||!canManage||working.current)return;
  working.current=true;setBusy(true);setMessage(null);
  try {const result=await recheckMetaHealth(store,page.page_id,boundary);if(!live.current)return;
   if(result.ok){setMessage("scheduled");refreshMetaHealth(store);}
   else setMessage(result.code==="unauthorized"?"signedOut":result.code==="forbidden"?"forbidden":result.code==="recheck_too_soon"?"tooSoon":result.uncertain?"uncertain":"failed");
  }finally{working.current=false;if(live.current)setBusy(false);}
 }
 if(!page)return failed?<p className="settings-note" data-testid="meta-health-unavailable">{c.unavailable}</p>:null;
 const unidentified=page.capabilities.some(row=>!row.capability);
 const rows=[...page.capabilities].sort((a,b)=>a.provider.localeCompare(b.provider)||capabilities.indexOf(a.capability!) -capabilities.indexOf(b.capability!));
 return <section className="meta-health-details" aria-label={c.abilities} data-testid={`meta-health-page-${page.page_id}`}>
  <div className="meta-health-heading"><h4>{c.abilities}</h4>{canManage?<button type="button" data-testid="meta-health-recheck" disabled={busy||!boundary} onClick={()=>void recheck()}>{busy?c.checking:c.recheck}</button>:<p>{c.readOnly}</p>}</div>
  {message&&<p role="status" data-testid="meta-health-recheck-result">{c[message]}</p>}
  {unidentified?<p data-testid="meta-health-unidentified">{c.incomplete}</p>:<dl className="meta-health-rows">{rows.map(row=><div key={`${row.binding_id}:${row.capability}`} data-testid={`meta-health-capability-${row.provider}-${row.capability}`}>
   <dt>{row.provider==="facebook"?"Facebook":"Instagram"} · {c.capability[row.capability!]}</dt>
   <dd><span className="meta-health-badge" data-state={row.state}>{c.state[row.state]}</span>{capabilityAdvice(locale,row)&&<p>{capabilityAdvice(locale,row)}</p>}<small>{c.checked}：{row.checked_at?displayTime(locale,row.checked_at):c.never}</small></dd>
  </div>)}</dl>}
 </section>;
}
