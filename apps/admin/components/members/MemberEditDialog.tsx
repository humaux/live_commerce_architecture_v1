// Purpose: own one private member profile draft and one immutable PATCH intent through CAS/UNKNOWN recovery.
// Depends on: frozen members-client, accepted read/session boundary and existing DS/global ReauthDialog.
// Used by: MemberDetail; no storage/replay, no point editing, scope/session/erasure discard private state.
"use client";
import { useEffect, useRef, useState, type FormEvent } from "react";
import type { Locale } from "@live-commerce/i18n";
import { Button } from "@live-commerce/ui/ds";
import type { Store } from "@/lib/model";
import type { ReadStatus } from "@/lib/customers-client";
import { sessionBoundary } from "@/lib/settings-client";
import { updateMember, MemberWriteError, type Member, type UpdateMemberInput } from "@/lib/members-client";
import { memberEditCopy } from "./edit-copy";
import s from "./member-edit.module.css";
type Props = { locale:Locale; store:Store|null; member?:Member|null; scope:string; boundary:string; status:ReadStatus; onRefresh:()=>void };
type Step = "edit"|"pending"|"conflict"|"unknown"|"refused";
type Intent = Readonly<{store:string; member:string; boundary:string; scope:string; key:string; body:Readonly<UpdateMemberInput>; controller:AbortController}>;
/** A reload may conceal the owner; it must never create a new key for an unconfirmed command. */
export function MemberEditDialog({locale,store,member,scope,boundary,status,onRefresh}:Props) {
  const c=memberEditCopy[locale];
  const [open,setOpen]=useState(false),[step,setStep]=useState<Step>("edit");
  const [denied,setDenied]=useState(false);
  const [draft,setDraft]=useState({display_name:"",contact_note:"",notes:""});
  const [error,setError]=useState(""),[invalid,setInvalid]=useState("");
  const original=useRef<Member|null>(null),intent=useRef<Intent|null>(null);
  const privateBoundary=useRef(""),current=useRef({scope,boundary,status,permitted:false});
  const refreshConflict=useRef(false),erased=useRef(false);
  const dialog=useRef<HTMLDialogElement>(null),opener=useRef<HTMLElement|null>(null);
  const permitted=!!store?.permissions?.includes("customers:points")&&!denied;
  current.current={scope,boundary,status,permitted};
  const concealed=!permitted || status!=="ready" || !boundary || (!!privateBoundary.current && privateBoundary.current!==boundary);
  const busy=step==="pending",locked=step!=="edit";
  function clear() {
    intent.current?.controller.abort(); intent.current=null; original.current=null;privateBoundary.current="";refreshConflict.current=false;
    setOpen(false);setStep("edit");setDraft({display_name:"",contact_note:"",notes:""});setError("");setInvalid("");
  }
  function adopt(m:Member) {original.current=m;setDraft({display_name:m.display_name,contact_note:m.contact_note??"",notes:m.notes??""});setError("");setInvalid("");setStep("edit");}
  useEffect(()=>{
    erased.current=false;setDenied(false);clear();const logout=()=>clear();window.addEventListener("commerce-session-logout",logout);
    return()=>{intent.current?.controller.abort();intent.current=null;window.removeEventListener("commerce-session-logout",logout);};
  },[scope]);
  useEffect(()=>{if(!permitted || status==="signed-out" || status==="forbidden" || status==="not-found") clear();},[permitted,status]);
  useEffect(()=>{
    if(status!=="ready") return;
    if(privateBoundary.current && privateBoundary.current!==boundary){clear();return;}
    if(member?.erased_at){erased.current=true;clear();return;}
    if(refreshConflict.current && member){refreshConflict.current=false;adopt(member);}
  },[status,boundary,member]);
  useEffect(()=>{
    const el=dialog.current;if(!el)return;
    if(open&&!concealed&&!el.open){el.showModal();(el.querySelector<HTMLElement>(step==="edit"?"input":"h2")??el).focus();}
    else if((!open||concealed)&&el.open){el.close();if(!concealed&&opener.current?.isConnected)opener.current.focus();}
  },[open,concealed,step]);
  useEffect(()=>{
    if(!open||concealed)return;
    if(invalid)dialog.current?.querySelector<HTMLElement>(`[name="${invalid}"]`)?.focus();
    else if(step!=="edit")dialog.current?.querySelector<HTMLElement>("h2")?.focus();
  },[step,invalid,open,concealed]);
  function accepted(){const r=current.current;return r.scope===scope&&r.status==="ready"&&r.permitted&&!!r.boundary&&(!privateBoundary.current||privateBoundary.current===r.boundary)&&!erased.current;}
  function start(event:{currentTarget:HTMLElement}){
    if(!accepted()||!member||member.erased_at)return;
    opener.current=event.currentTarget;
    if(!intent.current){privateBoundary.current=boundary;adopt(member);}
    setOpen(true);
  }
  function close(){if(busy)return;dialog.current?.close();opener.current?.focus();if(intent.current){setOpen(false);return;}clear();}
  function fail(field:string,text:string){setInvalid(field);setError(text);dialog.current?.querySelector<HTMLElement>(`[name="${field}"]`)?.focus();}
  async function save(event:FormEvent){
    event.preventDefault();if(!store||!original.current||intent.current||locked||!accepted())return;
    const name=draft.display_name.trim().normalize("NFC");
    if(!name)return fail("display_name",c.empty);
    if(Array.from(name).length>60||/[\p{Cc}\p{Cf}\p{Cs}]/u.test(name))return fail("display_name",c.invalidName);
    if(Array.from(draft.contact_note).length>120||/\p{Cs}/u.test(draft.contact_note))return fail("contact_note",c.invalidContact);
    if(Array.from(draft.notes).length>1000||/\p{Cs}/u.test(draft.notes))return fail("notes",c.invalidNotes);
    const body:UpdateMemberInput={expected_version:original.current.version};
    if(name!==original.current.display_name)body.display_name=name;
    if((draft.contact_note||null)!==original.current.contact_note)body.contact_note=draft.contact_note||null;
    if((draft.notes||null)!==original.current.notes)body.notes=draft.notes||null;
    if(Object.keys(body).length===1){setError(c.unchanged);return;}
    const command:Intent=Object.freeze({store:store.id,member:original.current.id,boundary,scope,key:crypto.randomUUID(),body:Object.freeze(body),controller:new AbortController()});
    intent.current=command;setError("");setInvalid("");setStep("pending");
    const live=()=>intent.current===command&&current.current.scope===command.scope;
    try{
      let now:string;try{now=await sessionBoundary();}catch{if(live())clear();return;}
      if(!live())return;if(now!==command.boundary||!accepted()){clear();return;}
      await updateMember(command.store,command.member,command.body,command.key,command.controller.signal);
      if(!live())return;clear();onRefresh();
    }catch(e){
      if(!live())return;
      if(e instanceof MemberWriteError&&e.code==="signed-out"){clear();return;}
      if(!(e instanceof MemberWriteError)||e.outcome==="unknown"||e.code==="idempotency_conflict"){setStep("unknown");return;}
      intent.current=null;
      if(e.code==="member_erased"){erased.current=true;clear();onRefresh();return;}
      if(e.code==="forbidden"||e.code==="not-found"){if(e.code==="forbidden")setDenied(true);clear();onRefresh();return;}
      setStep(e.code==="conflict"?"conflict":"refused");setError(e.code==="conflict"?c.conflict:c.refused);
    }
  }
  function refresh(){if(!accepted()||busy)return;if(step!=="unknown")refreshConflict.current=true;onRefresh();}
  const input=(field:keyof typeof draft,label:string,max:number,multiline=false)=><div className={s.field}>
    <label htmlFor={`member-edit-${field}`}>{label}</label>
    {multiline?<textarea id={`member-edit-${field}`} name={field} value={draft[field]} readOnly={locked} maxLength={max*2} aria-invalid={invalid===field} aria-describedby={invalid===field?"member-edit-error":undefined} onChange={e=>setDraft({...draft,[field]:e.target.value})}/>:<input id={`member-edit-${field}`} name={field} value={draft[field]} readOnly={locked} autoComplete="off" maxLength={max*2} aria-invalid={invalid===field} aria-describedby={invalid===field?"member-edit-error":field==="contact_note"?"member-edit-contact-hint":undefined} onChange={e=>setDraft({...draft,[field]:e.target.value})}/>}
    {field==="contact_note"&&<p className={s.hint} id="member-edit-contact-hint">{c.hint}</p>}
    {invalid===field&&<p role="alert" id="member-edit-error" className={s.error}>{error}</p>}
  </div>;
  if(!permitted||status==="not-found"||status==="forbidden"||status==="signed-out")return null;
  return <>
    <Button data-testid="member-edit-open" disabledReason={member?.erased_at||erased.current?c.erased:concealed||!member?(status==="loading"||status==="hidden"?c.loading:c.refused):undefined} onClick={start}>{c.edit}</Button>
    {open&&!concealed&&<dialog ref={dialog} className={s.dialog} data-testid="member-edit-dialog" aria-labelledby="member-edit-title" onCancel={e=>{e.preventDefault();if(!intent.current)close();}}>
      <h2 id="member-edit-title" tabIndex={-1}>{step==="unknown"?c.unknownTitle:c.title}</h2>
      {step==="unknown"?<>
        <span className={s.badge} data-testid="member-edit-unknown">{c.unknownBadge}</span><p>{c.unknown}</p><p className={s.hint}>{c.readOnly}</p>
        <div className={s.actions}><Button variant="primary" data-testid="member-edit-refresh" onClick={refresh}>{c.refresh}</Button><Button onClick={close}>{c.close}</Button></div>
      </>:<form onSubmit={save} noValidate>
        {error&&!invalid&&<p role="alert" className={s.error}>{error}</p>}
        <div className={s.fields}>{input("display_name",c.name,60)}{input("contact_note",c.contact,120)}{input("notes",c.notes,1000,true)}</div>
        {step==="edit"&&<p className={s.hint}>{c.points}</p>}
        <div className={s.actions}>
          {step==="conflict"||step==="refused"?<Button variant="primary" data-testid="member-edit-load-current" onClick={refresh}>{c.refresh}</Button>:<Button variant="primary" type="submit" data-testid="member-edit-save" disabledReason={busy?c.pending:undefined}>{busy?c.pending:c.save}</Button>}
          <Button disabledReason={busy?c.pending:undefined} onClick={close}>{c.cancel}</Button>
        </div>
      </form>}
    </dialog>}
  </>;
}
