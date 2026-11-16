// Purpose: dedicated LP19 editable deduct confirmation.
// Used by: OrderPointsPanel.
// Depends on: existing adjustPoints and session fence, DS controls. UNKNOWN can only refresh reads.
"use client";
import { useEffect, useRef, useState } from "react";
import { decimal } from "@live-commerce/format";
import { Button } from "@live-commerce/ui/ds";
import type { Locale } from "@live-commerce/i18n";
import { adjustPoints, MemberWriteError, type Member, type OrderPointEntry } from "@/lib/members-client";
import { sessionBoundary } from "@/lib/settings-client";
import { orderPointsCopy, validOrderPoints } from "./order-points-copy";
import s from "./order-points.module.css";
type Props={locale:Locale;store:string;order:string;boundary:string;award:OrderPointEntry;member:Member|null;permitted:boolean;onClose:()=>void;onRefresh:()=>Promise<boolean>};
export function OrderDeductDialog(p:Props) {
  const c=orderPointsCopy[p.locale], dialog=useRef<HTMLDialogElement>(null);
  const [amount,setAmount]=useState(String(p.award.delta)), [note,setNote]=useState("");
  const [role,setRole]=useState<"Confirm"|"Pending"|"Insufficient"|"EmptyNote"|"Unknown"|"Refused">("Confirm");
  const [error,setError]=useState("");
  const intent=useRef<{controller:AbortController;key:string;body:Readonly<{delta:number;note:string;order_id:string}>}|null>(null);
  const confirmed=useRef(false);
  const authority=useRef(p);authority.current=p;
  const blocked=role==="Pending"||role==="Unknown";
  const blockedReason=blocked?(role==="Unknown"?c.unknown:c.loading):undefined;
  useEffect(()=>{
    const opener=document.activeElement;
    dialog.current?.showModal();
    return ()=>{intent.current?.controller.abort();dialog.current?.close();if(opener instanceof HTMLElement)opener.focus();};
  },[]);
  useEffect(()=>{if(!p.permitted||!p.boundary||p.member?.erased_at){intent.current?.controller.abort();p.onClose();}},[p.permitted,p.boundary,p.member?.erased_at]);
  async function submit() {
    if(blocked||intent.current||!p.permitted||!p.boundary||p.member?.erased_at)return;
    if(!note.trim()){setRole("EmptyNote");return;}
    if(!validOrderPoints(amount)||Array.from(note.trim()).length>200)return;
    const boundary=p.boundary, controller=new AbortController();
    const captured={controller,key:crypto.randomUUID(),body:Object.freeze({delta:-Number(amount),note:note.trim(),order_id:p.order})};
    intent.current=captured;setRole("Pending");setError("");
    const live=()=>intent.current===captured&&!controller.signal.aborted&&authority.current.permitted&&authority.current.boundary===boundary;
    try {
      if(await sessionBoundary()!==boundary)throw new MemberWriteError("signed-out","not-sent");
      if(!live())return;
      await adjustPoints(p.store,p.award.member_id,captured.body,captured.key,controller.signal);
      const sameSession=await sessionBoundary()===boundary;
      if(!sameSession||!live())return;
      confirmed.current=true;
      const refreshed=await p.onRefresh();
      if(controller.signal.aborted)return;
      if(refreshed)p.onClose();else {setRole("Unknown");setError(c.failed);}
    } catch(e) {
      if(!live())return;
      if(confirmed.current||!(e instanceof MemberWriteError)||e.outcome==="unknown") {setRole("Unknown");setError(c.unknown);}
      else {intent.current=null;setRole(e.code==="points_insufficient"?"Insufficient":"Refused");setError(e.code==="points_insufficient"?c.insufficient:c.refused);}
    }
  }
  return <dialog ref={dialog} className={s.dialog} data-testid="order-points-deduct-dialog" data-order-points-role={role} aria-labelledby="order-points-deduct-title" onCancel={e=>{e.preventDefault();if(!blocked)p.onClose();}}>
    <div className={s.panel}><h2 id="order-points-deduct-title">{c.deduct}</h2>
    <p>{c.member}: {p.award.member_display_name}</p><p className={s.muted}>{c.adjustment} · {p.order.slice(0,8)}</p>
    {p.member&&<p>{c.balance}: {decimal(p.locale,p.member.balance,0)}</p>}
    {role==="Unknown"?<><p role="status">{error||c.unknown}</p><Button type="button" data-testid="order-points-deduct-refresh" onClick={()=>void p.onRefresh().then(ok=>{if(ok&&confirmed.current)p.onClose();})}>{c.refresh}</Button></>:<>
      <label>{c.amount}<input data-testid="order-points-deduct-amount" inputMode="numeric" value={amount} disabled={blocked} aria-invalid={!validOrderPoints(amount)||role==="Insufficient"} onChange={e=>{setAmount(e.target.value);setRole("Confirm");setError("");}}/></label>
      {!validOrderPoints(amount)&&<p className={s.error}>{c.invalid}</p>}
      <label>{c.note}<textarea data-testid="order-points-deduct-note" value={note} maxLength={200} disabled={blocked} aria-invalid={role==="EmptyNote"} onChange={e=>{setNote(e.target.value);setRole("Confirm");setError("");}}/></label>
      {role==="EmptyNote"&&<p className={s.error} role="alert">{c.required}</p>}
      {p.member&&validOrderPoints(amount)&&<p>{c.preview}: {p.member.balance} → {p.member.balance-Number(amount)}</p>}
      {error&&<p className={s.error} role="alert">{error}</p>}
      <div className={s.actions}><Button type="button" data-testid="order-points-deduct-submit" disabledReason={blockedReason||(!validOrderPoints(amount)?c.invalid:!p.permitted?c.refused:undefined)} loading={role==="Pending"} loadingLabel={c.loading} onClick={()=>void submit()}>{role==="Pending"?c.loading:c.confirm}</Button>
      <Button type="button" variant="secondary" data-testid="order-points-deduct-cancel" disabledReason={blockedReason} onClick={p.onClose}>{c.cancel}</Button></div>
    </>}
    </div></dialog>;
}
