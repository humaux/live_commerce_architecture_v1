// Purpose: embedded member search/create/selection for the manual order snapshot.
// Depends on: existing guarded reads and immutable member commands.
// Used by: ManualOrder.
// Invariants: no write retries after UNKNOWN; query/cursor/body remain private memory; read authority never grants points permission.
"use client";
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { decimal } from "@live-commerce/format";
import { Button } from "@live-commerce/ui/ds";
import type { Store } from "@/lib/model";
import { useGuardedRead } from "@/lib/customers-client";
import { sessionBoundary } from "@/lib/settings-client";
import { createMember, searchMembers, MemberWriteError, type MemberRow } from "@/lib/members-client";
import { orderPointsCopy, validOrderPoints } from "./order-points-copy";
import s from "./order-points.module.css";
export type OrderMemberSelection = { member: MemberRow | null; points: string };
type Props = { locale:Locale; store:Store; scope:string; boundary:string; enabled:boolean; disabled:boolean;
  value:OrderMemberSelection; onChange:(value:OrderMemberSelection)=>void; onPrefillName:(name:string)=>void; onBusy:(busy:boolean)=>void };
export function ManualOrderMemberFields(p:Props) {
  const c=orderPointsCopy[p.locale];
  const [query,setQuery]=useState(""), [after,setAfter]=useState<string|null>(null);
  const [creating,setCreating]=useState(false), [name,setName]=useState(""), [contact,setContact]=useState("");
  const [step,setStep]=useState<"edit"|"confirm"|"pending"|"unknown"|"refused">("edit");
  const [error,setError]=useState("");
  const active=useRef<AbortController|null>(null), epoch=useRef(0);
  const pointsField=useRef<HTMLInputElement>(null);
  const readAllowed=p.enabled && !!p.boundary && !!p.store.permissions?.includes("customers:read");
  const permitted=!!p.store.permissions?.includes("customers:points");
  const read=useGuardedRead(`${p.scope}|${p.boundary}|${readAllowed}|${step==="unknown"?name:query}|${after??""}`,
    readAllowed ? signal=>searchMembers(p.store.id,step==="unknown"?name:query,after,signal):null,null);
  const authority=useRef({enabled:false,boundary:""});
  authority.current={enabled:p.enabled&&permitted&&!(["signed-out","forbidden","hidden"].includes(read.status)),boundary:p.boundary};
  useEffect(()=>()=>{epoch.current++;active.current?.abort();},[]);
  useEffect(()=>{if(p.value.member)pointsField.current?.focus();},[p.value.member?.id]);
  useEffect(()=>{
    if(!p.enabled || !permitted || !p.boundary || read.status==="signed-out" || read.status==="forbidden" || read.status==="hidden") {
      epoch.current++;active.current?.abort();active.current=null;setQuery("");setAfter(null);setName("");setContact("");setCreating(false);setStep("edit");setError("");
      p.onChange({member:null,points:""});p.onBusy(false);
    }
  },[p.enabled,permitted,p.boundary,read.status]);
  if(!authority.current.enabled || !p.boundary) return null;
  const locked=p.disabled||step==="pending"||step==="unknown";
  const lockedReason=locked?(step==="unknown"?c.unknown:c.loading):undefined;
  function choose(member: MemberRow) {
    if (locked) return;
    p.onChange({ member, points: "" });
    p.onPrefillName(member.display_name);
    setCreating(false);
    setQuery("");
    setAfter(null);
  }
  function clearSelection() { p.onChange({member:null,points:""}); }
  async function save() {
    if(locked || active.current || !name.trim() || Array.from(name.trim()).length>60)return;
    if(step!=="confirm"){setStep("confirm");return;}
    const generation=++epoch.current, boundary=p.boundary, controller=new AbortController();active.current=controller;
    const body=Object.freeze({display_name:name,contact_note:contact||null,notes:null});
    setStep("pending");p.onBusy(true);setError("");
    try {
      if(await sessionBoundary()!==boundary) throw new MemberWriteError("signed-out","not-sent");
      if(controller.signal.aborted||!authority.current.enabled||authority.current.boundary!==boundary)return;
      const result=await createMember(p.store.id,body,crypto.randomUUID(),controller.signal);
      const sameSession=await sessionBoundary()===boundary;
      if(!sameSession||epoch.current!==generation||controller.signal.aborted||!authority.current.enabled)return;
      active.current=null;setStep("edit");p.onBusy(false);choose({...result.member,erased:result.member.erased_at!==null});
    } catch(e) {
      if(epoch.current!==generation||controller.signal.aborted)return;
      const unknown=!(e instanceof MemberWriteError)||e.outcome==="unknown";
      if(!unknown)active.current=null;
      setStep(unknown?"unknown":"refused");setError(unknown?c.unknown:c.refused);p.onBusy(unknown);
    }
  }
  const invalid=p.value.points!=="" && !validOrderPoints(p.value.points);
  return <section className={s.panel} data-testid="manual-member-fields"><h2>{c.title}</h2>
    {creating ? <div className={s.panel}>
      <h3>{c.create}</h3>
      {step==="unknown" ? <><p role="status">{c.unknown}</p><Button type="button" data-testid="manual-member-create-refresh" onClick={()=>read.reload()}>{c.refresh}</Button>
        {read.status==="ready"&&<div className={s.results}>{read.data?.members.map(member=><p key={member.id}>{member.display_name}</p>)}</div>}</> : <>
      <label>{c.name}<input data-testid="manual-member-create-name" value={name} maxLength={60} disabled={locked||step==="confirm"} onChange={e=>{setName(e.target.value);setStep("edit");}}/></label>
      <label>{c.contact}<input data-testid="manual-member-create-contact" value={contact} maxLength={120} disabled={locked||step==="confirm"} onChange={e=>setContact(e.target.value)}/></label>
      {error&&<p role="alert" className={s.error}>{error}</p>}
      <div className={s.actions}><Button type="button" data-testid="manual-member-create-submit" disabledReason={lockedReason||(!name.trim()?c.name:undefined)} loading={step==="pending"} loadingLabel={c.loading} onClick={()=>void save()}>{step==="pending"?c.loading:step==="confirm"?c.confirm:c.select}</Button>
      <Button type="button" variant="secondary" data-testid="manual-member-create-cancel" disabledReason={lockedReason} onClick={()=>{setCreating(false);setStep("edit");setError("");p.onBusy(false);}}>{c.cancel}</Button></div></>}
    </div> : p.value.member ? <>
      <div className={s.selected} data-testid="manual-member-selected">
        <strong>{p.value.member.display_name}</strong>
        <small>{p.value.member.contact_note}</small>
        <small>{c.balance} · {decimal(p.locale,p.value.member.balance,0)}</small>
      </div>
      <div className={s.actions}>
        <Button type="button" variant="secondary" data-testid="manual-member-change" disabledReason={lockedReason} onClick={clearSelection}>{c.change}</Button>
        <Button type="button" variant="secondary" data-testid="manual-member-remove" disabledReason={lockedReason} onClick={clearSelection}>{c.remove}</Button>
      </div>
    </> : <>
      {readAllowed&&<><label>{c.member}<input data-testid="manual-member-query" placeholder={c.query} value={query} maxLength={60} disabled={locked} onChange={e=>{setQuery(e.target.value);setAfter(null);}}/></label>
      {read.status==="loading"?<p role="status">{c.loading}</p>:read.status==="ready"?<div className={s.results} data-testid="manual-member-results">
        {read.data?.members.length===0&&<p>{c.empty}</p>}{read.data?.members.map(member=><button type="button" key={member.id} data-member-id={member.id} disabled={locked} onClick={()=>choose(member)}><strong>{member.display_name}</strong><small>{member.contact_note}</small></button>)}
        {read.data?.next_cursor&&<Button type="button" disabledReason={lockedReason} onClick={()=>setAfter(read.data!.next_cursor)}>{c.more}</Button>}</div>:<p role="status">{c.failed}<Button type="button" onClick={read.reload}>{c.refresh}</Button></p>}</>}
      <Button type="button" data-testid="manual-member-create-open" disabledReason={lockedReason} onClick={()=>{setCreating(true);setName(query);setStep("edit");}}>{c.create}</Button>
    </>}
    {!creating&&<><label>{c.points}<input ref={pointsField} data-testid="manual-member-points" inputMode="numeric" value={p.value.points} disabled={locked} aria-invalid={invalid} onChange={e=>p.onChange({...p.value,points:e.target.value})}/></label><p className={s.muted}>{c.optional}</p>
    {(invalid || (p.value.points!==""&&!p.value.member))&&<p role="alert" className={s.error} data-testid="manual-member-error">{invalid?c.invalid:c.needMember}</p>}</>}
  </section>;
}
