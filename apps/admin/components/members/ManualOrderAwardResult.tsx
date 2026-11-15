// Purpose: successful-order award outcome.
// Depends on: frozen ManualMemberAward, permitted member read and centralized decimal formatting.
// Used by: ManualOrder; never resubmits the order to repair an award and never adds points to a stale selected balance.
"use client";
import { decimal } from "@live-commerce/format";
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import type { ManualMemberAward } from "@/lib/merchant-tools-model";
import { useGuardedRead } from "@/lib/customers-client";
import { readMember } from "@/lib/members-client";
import { orderPointsCopy } from "./order-points-copy";
import s from "./order-points.module.css";
export function ManualOrderAwardResult({award,locale,store,scope,boundary}:{award:ManualMemberAward;locale:Locale;store:Store;scope:string;boundary:string}) {
  const c=orderPointsCopy[locale], allowed=!!store.permissions?.includes("customers:read");
  const read=useGuardedRead(`${scope}|${boundary}|${award.member_id}|${allowed}`,allowed&&boundary?signal=>readMember(store.id,award.member_id,null,signal):null,null);
  if(!boundary||read.status==="signed-out"||read.status==="hidden"||read.status==="forbidden")return null;
  return <section className={s.panel} data-testid="manual-member-award-result" data-award-state={award.state} data-award-reason={award.state==="not_awarded"?award.reason:undefined}>
    <h3>{award.state==="awarded"?c.awarded:c.notAwarded}</h3><p>{award.points}</p>
    {award.state==="not_awarded"&&<p>{c.reasons[award.reason]}</p>}
    {allowed&&read.status==="ready"&&read.data&&<><p>{read.data.member.display_name} · {c.balance}: {decimal(locale,read.data.member.balance,0)}</p><Link href={`/${locale}/members/${award.member_id}?store=${store.id}`}>{c.record}</Link></>}
  </section>;
}
