// Purpose: advisory Meta connection banner across workspace pages; hidden for healthy or failed reads.
// Depends on: current authenticated Store, useMetaHealth B1, merchant copy and existing Settings reconnect card.
// Used by: WorkspaceFrame; the banner never gates navigation or checkout.
"use client";
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { useMetaHealth } from "@/lib/meta-health-hook";
import { healthReason } from "@/lib/meta-health-model";
import { metaHealthCopy } from "@/lib/meta-health-copy";
import "./meta-health.css";
/** Shows server-derived connection advice for the selected store; only managers get an actionable link. */
export function MetaHealthBanner({store,locale}:{store:Store|null;locale:Locale}) {
 const {data}=useMetaHealth(store?.id??null),c=metaHealthCopy[locale];
 if(!store||!data||data.severity==="none")return null;
 const advice=c.advice[healthReason(data) as keyof typeof c.advice]??c.advice.unknown;
 return <aside role="status" className="meta-health-banner" data-testid="meta-health-banner" data-severity={data.severity}>
  <div><strong>{data.severity==="blocking"?c.title:c.warning}</strong><p>{advice}</p></div>
  {store.permissions?.includes("integration:manage")?<Link data-testid="meta-health-settings" href={`/${locale}/settings?store=${store.id}#facebook-instagram`}>{c.reconnect}</Link>:<p data-testid="meta-health-owner">{c.owner}</p>}
 </aside>;
}
