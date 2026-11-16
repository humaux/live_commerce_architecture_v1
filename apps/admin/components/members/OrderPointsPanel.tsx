// Purpose: order-scoped oldest-100 ledger, authoritative current member balance and manual deduct entry.
// Depends on: member guarded reads, DS, centralized decimal/time formatting and dedicated deduct dialog.
// Used by: OrderDetailView.
// Invariants: no actor invented, no partial totals/balance arithmetic, server permissions remain authoritative.
"use client";
import { decimal, displayTime } from "@live-commerce/format";
import { useState } from "react";
import Link from "next/link";
import { Button } from "@live-commerce/ui/ds";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { ReadError, useGuardedRead } from "@/lib/customers-client";
import { readMember, readOrderPointEntries, type Member, type OrderPointEntry } from "@/lib/members-client";
import { OrderDeductDialog } from "./OrderDeductDialog";
import { orderPointsCopy } from "./order-points-copy";
import s from "./order-points.module.css";
export function OrderPointsPanel({locale,store,order,scope,boundary}:{locale:Locale;store:Store;order:string;scope:string;boundary:string}) {
  const c=orderPointsCopy[locale], canRead=!!store.permissions?.includes("customers:read"), canWrite=!!store.permissions?.includes("customers:points");
  const [deduct,setDeduct]=useState<OrderPointEntry|null>(null);
  const read=useGuardedRead(`${scope}|${boundary}|${order}|${canRead}|${canWrite}`,canRead&&boundary?async signal=>{
    const page=await readOrderPointEntries(store.id,order,signal);
    const balances:Record<string,Member>={};
    for(const member of new Set(page.entries.filter(e=>e.reason==="manual_order_award").map(e=>e.member_id))) {
      try { balances[member]=(await readMember(store.id,member,null,signal)).member; }
      catch(e) {if(e instanceof ReadError&&(e.code==="forbidden"||e.code==="signed-out"))throw e;}
    }
    return {...page,balances};
  }:null,null);
  if(!canRead||!boundary||read.status==="hidden"||read.status==="signed-out"||read.status==="forbidden")return null;
  const data=read.status==="ready"?read.data:null;
  const role=read.status==="loading"?"Loading":!data?"Failed":!data.entries.length?"Empty":data.truncated?"Truncated":"Ready";
  const award=data?.entries.find(e=>e.reason==="manual_order_award");
  return <section className={s.panel} data-testid="order-points-panel" data-order-points-role={role}><h2>{c.title}</h2>
    {role==="Loading"?<p role="status">{c.loading}</p>:!data?<p role="status">{c.failed}</p>:<>
      {!data.entries.length&&<p>{c.empty}</p>}
      {data.entries.map(entry=><div key={entry.id} className={s.panel}><Link prefetch={false} href={`/${locale}/members/${entry.member_id}?store=${store.id}`}>{entry.member_display_name} · {c.record}</Link>
        <p className={entry.delta>0?s.success:undefined}>{entry.reason==="manual_order_award"?c.points:c.adjustment}: {entry.delta>0?"+":""}{entry.delta} · {displayTime(locale,entry.created_at)}</p>
        {entry.note&&<p>{entry.note}</p>}
      </div>)}
      {Object.entries(data.balances).map(([id,member])=><p key={id}>{member.display_name} · {c.balance}: {decimal(locale,member.balance,0)}</p>)}
      {data.truncated&&<p className={s.muted}>{c.truncated}</p>}
      {canWrite&&award&&!data.balances[award.member_id]?.erased_at&&<Button type="button" data-testid="order-points-deduct-open" onClick={()=>setDeduct(award)}>{c.deduct}</Button>}
    </>}
    <Button type="button" variant="secondary" data-testid="order-points-refresh" onClick={read.reload}>{c.refresh}</Button>
    {deduct&&data&&canWrite&&<OrderDeductDialog key={`${scope}|${boundary}|${deduct.member_id}|${deduct.id}`} locale={locale} store={store.id} order={order} boundary={boundary} award={deduct} member={data.balances[deduct.member_id]??null} permitted={canWrite&&read.status==="ready"} onClose={()=>setDeduct(null)} onRefresh={read.refresh}/>}
  </section>;
}
