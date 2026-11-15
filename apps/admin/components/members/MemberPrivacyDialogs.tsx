// Purpose: sibling export/erasure intents with memory-only downloads and authoritative erasure reads.
// Depends on: members-client, sessionBoundary and existing DS; mounted by MemberDetail.
// Invariants: no automatic retry, immutable target/key, no private data across scope/session/permission changes.
"use client";
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { Button } from "@live-commerce/ui/ds";
import type { Store } from "@/lib/model";
import type { ReadStatus } from "@/lib/customers-client";
import { sessionBoundary } from "@/lib/settings-client";
import { exportMember, eraseMember, MemberWriteError, type Member } from "@/lib/members-client";
import { privacyCopy } from "./privacy-copy";
import s from "./member-privacy.module.css";
type Props = { locale:Locale; store:Store|null; member?:Member|null; scope:string; boundary:string; status:ReadStatus; onRefresh:()=>void };
type Role = "ExportPrompt"|"ExportPreparing"|"ExportDownloaded"|"ExportFailed"|"ExportTooLarge"|"EraseConfirm"|"EraseUnknown";
type Intent = Readonly<{kind:"export"|"erase"; store:string; member:string; scope:string; boundary:string; key:string}>;
/** Owns only a mounted intent; export retry explicitly recovers its original key. */
export function MemberPrivacyDialogs({locale,store,member,scope,boundary,status,onRefresh}:Props) {
  const c=privacyCopy[locale];
  const [role,setRole]=useState<Role|null>(null),[busy,setBusy]=useState(false),[unknown,setUnknown]=useState(false),[refused,setRefused]=useState(false),[denied,setDenied]=useState(false);
  const intent=useRef<Intent|null>(null),active=useRef<AbortController|null>(null),url=useRef<string|null>(null);
  const dialog=useRef<HTMLDialogElement>(null),opener=useRef<HTMLElement|null>(null);
  const permitted=!!store?.permissions?.includes("customers:privacy")&&!denied;
  const current=useRef({scope,boundary,status,permitted,id:member?.id,erased:member?.erased_at});
  current.current={scope,boundary,status,permitted,id:member?.id,erased:member?.erased_at};
  const visible=permitted&&status==="ready"&&!!boundary&&(!intent.current||intent.current.boundary===boundary)&&!member?.erased_at;
  function revoke(){if(url.current)URL.revokeObjectURL(url.current);url.current=null;}
  function clear(){active.current?.abort();active.current=null;intent.current=null;revoke();setRole(null);setBusy(false);setUnknown(false);setRefused(false);}
  useEffect(()=>{clear();setDenied(false);const logout=()=>clear();window.addEventListener("commerce-session-logout",logout);return()=>{active.current?.abort();revoke();window.removeEventListener("commerce-session-logout",logout);};},[scope]);
  useEffect(()=>{
    if(!permitted||member?.erased_at||status==="signed-out"||status==="forbidden"||status==="not-found"||(intent.current&&boundary&&intent.current.boundary!==boundary)){clear();return;}
    if(status!=="ready"){
      revoke();
      if(active.current){active.current.abort();active.current=null;setBusy(false);setUnknown(true);setRole(intent.current?.kind==="erase"?"EraseUnknown":"ExportFailed");}
    }
  },[permitted,member?.erased_at,status,boundary]);
  useEffect(()=>{const el=dialog.current;if(!el)return;if(role&&visible&&!el.open)el.showModal();if(role&&visible)el.querySelector<HTMLElement>("h2")?.focus();},[role,visible]);
  function accepted(i:Intent){const r=current.current;return r.permitted&&r.status==="ready"&&!r.erased&&r.scope===i.scope&&r.boundary===i.boundary&&r.id===i.member;}
  function open(kind:"export"|"erase",element:HTMLElement){
    if(!visible||!store||!member||busy)return;
    if(intent.current&&intent.current.kind!==kind)return;
    opener.current=element;
    if(!intent.current){intent.current=Object.freeze({kind,store:store.id,member:member.id,scope,boundary,key:crypto.randomUUID()});setRole(kind==="export"?"ExportPrompt":"EraseConfirm");}
    else setRole(intent.current.kind==="export"?"ExportFailed":"EraseUnknown");
  }
  function close(){if(busy)return;dialog.current?.close();if(!unknown)clear();else setRole(null);opener.current?.focus();}
  async function submit(){
    const i=intent.current;if(!i||busy||!accepted(i)||refused)return;
    const controller=new AbortController();active.current=controller;setBusy(true);setUnknown(false);
    if(i.kind==="export")setRole("ExportPreparing");
    const live=()=>intent.current===i&&active.current===controller&&accepted(i);
    try{
      const session=await sessionBoundary();
      if(intent.current!==i||active.current!==controller)return;
      if(session!==i.boundary){clear();return;}
      if(!live())return;
      if(i.kind==="erase"){
        await eraseMember(i.store,i.member,i.key,controller.signal);
        if(!live())return;
        // Receipt alone cannot establish the read tombstone. Refresh never resends.
        setUnknown(true);setRole("EraseUnknown");onRefresh();
      }else{
        const data=await exportMember(i.store,i.member,i.key,controller.signal);
        if(!live())return;
        const after=await sessionBoundary();
        if(intent.current!==i||active.current!==controller)return;
        if(after!==i.boundary){clear();return;}
        if(!live())return;
        if(data.member.erased_at){clear();onRefresh();return;}
        revoke();url.current=URL.createObjectURL(new Blob([JSON.stringify(data)],{type:"application/json"}));
        const link=document.createElement("a");link.href=url.current;link.download="member-export.json";document.body.appendChild(link);
        try{link.click();}finally{link.remove();revoke();}
        setRole("ExportDownloaded");
      }
    }catch(e){
      if(intent.current!==i||active.current!==controller)return;
      if(e instanceof MemberWriteError&&(e.code==="signed-out"||e.code==="forbidden"||e.code==="not-found"||e.code==="member_erased")){
        if(e.code==="forbidden")setDenied(true);clear();onRefresh();return;
      }
      if(e instanceof MemberWriteError&&e.code==="export_too_large"&&e.outcome==="rejected"){setRole("ExportTooLarge");return;}
      const unresolved=!(e instanceof MemberWriteError)||e.outcome==="unknown"||e.code==="idempotency_conflict";
      setUnknown(unresolved);setRefused(e instanceof MemberWriteError&&e.code==="idempotency_conflict");
      if(i.kind==="erase"){setRole("EraseUnknown");setRefused(!unresolved);}else setRole("ExportFailed");
    }finally{if(active.current===controller){active.current=null;setBusy(false);}}
  }
  if(!permitted)return null;
  if(status==="ready"&&member?.erased_at)return <p>{c.tombstone}</p>;
  const exporting=role?.startsWith("Export");
  return <>
    <div className={s.entries}>
      <Button data-testid="member-export-open" disabledReason={!visible||busy||intent.current?.kind==="erase"?c.pending:undefined} onClick={e=>open("export",e.currentTarget)}>{c.export}</Button>
      <Button variant="danger" data-testid="member-erase-open" disabledReason={!visible||busy||intent.current?.kind==="export"?c.pending:undefined} onClick={e=>open("erase",e.currentTarget)}>{c.erase}</Button>
    </div>
    {role&&visible&&<dialog ref={dialog} className={s.dialog} data-testid="member-privacy-dialog" data-privacy-role={role} aria-labelledby="member-privacy-title" aria-describedby="member-privacy-description" onCancel={e=>{e.preventDefault();close();}}>
      <h2 id="member-privacy-title" tabIndex={-1}>{exporting?c.exportTitle:role==="EraseConfirm"?c.eraseTitle:c.unknownTitle}</h2>
      <p id="member-privacy-description">{exporting?c.exportBody:role==="EraseConfirm"?c.retained:refused?c.refused:c.unknownErase}</p>
      {role==="EraseConfirm"&&<p>{c.irreversible}</p>}
      <p role="status" className={role==="ExportFailed"||role==="ExportTooLarge"?s.error:s.hint}>{role==="ExportPrompt"?c.caution:role==="ExportPreparing"?c.preparing:role==="ExportDownloaded"?c.downloaded:role==="ExportTooLarge"?c.tooLarge:role==="ExportFailed"?(refused?c.refused:unknown?c.unknownExport:c.failed):role==="EraseUnknown"?c.readOnly:""}</p>
      <div className={s.actions}>
        {(role==="ExportPrompt"||role==="ExportPreparing")&&<Button variant="primary" data-testid="member-export-submit" disabledReason={busy?c.preparing:undefined} onClick={()=>void submit()}>{busy?c.preparing:c.download}</Button>}
        {role==="ExportFailed"&&!refused&&<Button variant="primary" data-testid="member-export-retry" onClick={()=>void submit()}>{c.retry}</Button>}
        {role==="EraseConfirm"&&<Button variant="danger" data-testid="member-erase-submit" disabledReason={busy?c.pending:undefined} onClick={()=>void submit()}>{busy?c.pending:c.erase}</Button>}
        {(role==="EraseUnknown"||refused)&&<Button variant="primary" data-testid="member-privacy-refresh" onClick={onRefresh}>{c.refresh}</Button>}
        <Button data-testid="member-privacy-close" disabledReason={busy?c.pending:undefined} onClick={close}>{role==="ExportDownloaded"?c.done:role==="ExportPrompt"||role==="EraseConfirm"?c.cancel:c.close}</Button>
      </div>
    </dialog>}
  </>;
}
