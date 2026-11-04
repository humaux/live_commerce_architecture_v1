// Purpose: Signed session recap, shared timeline and aggregate audience views.
// Depends on: existing attribution GET and audience-read journal, scoped visible reads and locale formatting.
// Used by: Studio review/timeline/audience states; never joins buyer identities to viewers.
// Invariants: no client money arithmetic; nullable provider facts stay absent; audience ACK is not completion.
"use client";
import { useState } from "react";
import { liveSessionsCopy, type Locale } from "@live-commerce/i18n";
import { money, instantToTaipei } from "@live-commerce/format";
import { readAttribution } from "@/lib/attribution-client";
import { attributionCopy } from "@/lib/attribution-copy";
import { validAdsQuery } from "@/lib/ads-request";
import { StudioError } from "@/lib/studio-client";
import { displayTime } from "@/lib/orders-model";
import type { Draft } from "@/lib/studio-model";
import { useLiveRead } from "@/src/features/live/use-live-workspace";
import { AttributionAudienceRead } from "../AttributionAudienceRead";
import s from "./session-tasks.module.css";

/** Range is an explicit Taipei-date report filter; PG computes every aggregate. */
export function SessionReview({ locale, store, draft, task }: { locale: Locale; store: string; draft: Draft; task: string }) {
  const c = liveSessionsCopy[locale];
  const initialFrom = instantToTaipei(draft.scheduled_at ?? draft.created_at).slice(0, 10);
  const initialTo = initialFrom;
  const [range, setRange] = useState({ from: initialFrom, to: initialTo }), [query, setQuery] = useState(range);
  const view = useLiveRead(`${store}:review:${query.from}:${query.to}`, true, async signal => {
    try { return await readAttribution(store, query.from, query.to, signal); }
    catch (error) { throw new StudioError(error instanceof Error && error.message === "forbidden" ? "forbidden" : error instanceof Error && error.message === "signed-out" ? "signed-out" : "unavailable"); }
  });
  const row = view.data?.sessions.find(value => value.session_id === draft.session_id);
  const base = `/${locale}/studio?store=${store}&scene=${draft.session_id}`;
  const cash = (value: number | null) => value === null || !row ? "—" : money(locale, row.currency, value);
  const title = task === "timeline" ? c.timelineTitle : task === "audience" ? c.audienceTitle : c.review;
  return <section className={s.page}><h1>{title}</h1><p>{draft.title}</p>
    <form className={s.row} onSubmit={event => { event.preventDefault(); if (validAdsQuery(`https://local.invalid?from=${range.from}&to=${range.to}`, "ads/attribution")) { setQuery(range); view.refresh(); } }}>
      <label>{c.from}<input data-testid="session-review-from" value={range.from} onChange={event => setRange({ ...range, from: event.target.value })} /></label><label>{c.to}<input data-testid="session-review-to" value={range.to} onChange={event => setRange({ ...range, to: event.target.value })} /></label><button type="submit">{c.reload}</button>
    </form>
    {view.error && <p role="alert">{c.unavailable}</p>}{!view.data && !view.error && <p role="status">{c.loading}</p>}
    {row && task === "review" && <>
      <div className={s.card} data-testid="session-review-funnel"><h2>{c.funnel}</h2><dl className={s.facts}>{([[c.comments, row.funnel.comments], [c.claims, row.funnel.claims], [c.links, row.funnel.checkout_links], [c.paid, row.funnel.paid_orders]] as const).map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl></div>
      <div className={s.card}><dl className={s.facts}><div><dt>{c.net}</dt><dd data-testid="session-review-net">{cash(row.net_minor)}</dd></div><div><dt>{c.pending}</dt><dd>{row.pending_orders}</dd></div><div><dt>{c.pendingAmount}</dt><dd>{cash(row.pending_minor)}</dd></div><div><dt>{c.ambiguous}</dt><dd>{row.ambiguous_orders}</dd></div><div><dt>{c.spend}</dt><dd>{cash(row.spend_minor)}</dd></div></dl></div>
      <details className={s.card}><summary>{c.buyers}</summary><dl className={s.facts}><div><dt>{c.newBuyers}</dt><dd>{row.buyers.new_buyers}</dd></div><div><dt>{c.returningBuyers}</dt><dd>{row.buyers.returning_buyers}</dd></div><div><dt>{c.average}</dt><dd>{cash(row.buyers.average_order_minor)}</dd></div></dl><h2>{c.topProducts}</h2>{row.buyers.top_products.map(item => <p key={item.product_id}>{item.name} · {item.quantity}</p>)}<h2>{c.counties}</h2>{row.buyers.counties.map(item => <p key={item.name}>{item.name} · {item.orders} · {cash(item.net_minor)}</p>)}</details>
      <div className={s.footer}><button type="button" onClick={() => { window.location.hash = "timeline"; }}>{c.timeline}</button><button type="button" onClick={() => { window.location.hash = "audience"; }}>{c.audience}</button></div>
    </>}
    {row && task === "timeline" && <><p className={s.note}>{c.causal}</p><div className={`${s.card} ${s.scroll}`} data-testid="session-review-timeline"><table><thead><tr>{[c.time, c.comments, c.claims, c.orders, c.net, c.spend, c.views].map(label => <th key={label} scope="col">{label}</th>)}</tr></thead><tbody>{row.timeline.map(point => <tr key={point.at}><td>{displayTime(locale, point.at)}</td><td>{point.comments}</td><td>{point.claims}</td><td>{point.orders}</td><td>{cash(point.net_minor)}</td><td>{cash(point.spend_minor)}</td><td>{point.viewers ?? "—"}</td></tr>)}</tbody></table></div></>}
    {task === "audience" && <div className={s.card}>
      {row && row.live_audience.status !== "available" && <p>{row.live_audience.status === "not_read" ? c.notRead : row.live_audience.status === "not_authorized" ? c.notAuthorized : c.insufficient}</p>}
      <dl className={s.facts}><div><dt>{c.views}</dt><dd>{row?.live_audience.views ?? "—"}</dd></div><div><dt>{c.peak}</dt><dd>{row?.live_audience.peak_concurrent ?? "—"}</dd></div><div><dt>{c.viewTime}</dt><dd>{row?.live_audience.total_view_time_ms ?? "—"}</dd></div></dl>
      <div className={s.grid}>{row?.live_audience.status === "available" && <><div><h2>{attributionCopy[locale].ageGender}</h2>{row.live_audience.age_gender.map(bucket => <p key={bucket.bucket}>{bucket.bucket} · {bucket.view_time_ms}</p>)}</div><div><h2>{attributionCopy[locale].region}</h2>{row.live_audience.regions.map(bucket => <p key={bucket.bucket}>{bucket.bucket} · {bucket.view_time_ms}</p>)}</div></>}</div>
      <div data-testid="session-audience-result"><AttributionAudienceRead key={`${store}:${draft.session_id}`} store={store} session={draft.session_id} c={{ ...attributionCopy[locale], audienceRefresh: c.audienceRead, audienceReading: c.loading, audienceQueued: c.audienceQueued, audienceInflight: c.audienceQueued, audienceUnconfirmed: c.uncertain, audienceUnknown: c.uncertain, audienceRetry: c.retry, audienceForbidden: c.notAuthorized, audienceFailed: c.unavailable }} /></div>
    </div>}
    <div className={s.footer}>{task !== "review" && <button type="button" onClick={() => { window.location.hash = "review"; }}>{c.reviewBack}</button>}<a href={`/${locale}/studio?store=${store}`}>{c.back}</a></div>
  </section>;
}
