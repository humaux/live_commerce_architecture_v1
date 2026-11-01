// Purpose: Signed preparation, post-live, copy and end tasks on the real Studio route.
// Depends on: existing Studio DTO, scoped console commands, React and Next router.
// Used by: Studio; SessionReview owns report-only states.
// Invariants: lifecycle and copy CAS remain server-owned; durable UNKNOWN keys are never replaced.
"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { liveSessionsCopy, type Locale } from "@live-commerce/i18n";
import { money } from "@live-commerce/format";
import type { StudioDetail } from "@/lib/studio-model";
import { readConsole } from "@/src/features/live/console-client";
import type { CopyResult } from "@/src/features/live/console-model";
import { liveRequest } from "@/src/features/live/command-journal";
import { useLiveCommand, useLiveRead } from "@/src/features/live/use-live-workspace";
import s from "./session-tasks.module.css";

/** Hash is navigation state only; command authority is never read from a hash. */
export function useSessionTask() {
  const [task, setTask] = useState("");
  // Next's SPA router can update the URL without emitting native hashchange.
  // Read after each commit as well as after native anchor/back navigation.
  useEffect(() => { setTask(window.location.hash.slice(1)); });
  useEffect(() => { const read = () => setTask(window.location.hash.slice(1)); read(); window.addEventListener("hashchange", read); return () => window.removeEventListener("hashchange", read); }, []);
  return task;
}

/** Uses the original command journal for copies and lifecycle changes. */
export function SessionTasks({ locale, store, detail, task, refresh, copyResult, onCopy }: {
  locale: Locale; store: string; detail: StudioDetail; task: string; refresh: () => void; copyResult: CopyResult | null; onCopy: (value: CopyResult) => void;
}) {
  const c = liveSessionsCopy[locale], router = useRouter(), scene = detail.draft.session_id;
  const base = `/${locale}/studio?store=${store}&scene=${scene}`;
  const claims = `/${locale}/studio/claims?store=${store}&scene=${scene}`;
  const snapshot = useLiveRead(`${store}:${scene}:task`, ["post-live", "end", "next-draft"].includes(task), signal => readConsole(store, scene, signal));
  const [copying, setCopying] = useState(task === "copy"), [title, setTitle] = useState(detail.draft.title);
  const authority = useLiveRead(`${store}:${scene}:task-authority`, true, async () => detail);
  const command = useLiveCommand(`${store}:${scene}`, authority.boundary, () => { snapshot.refresh(); refresh(); }, (request, value) => {
    if (request.path.endsWith("/copy")) copied(value as CopyResult);
    else window.location.hash = "post-live";
  });
  function copied(value: CopyResult) { onCopy(value); router.push(`/${locale}/studio?store=${store}&scene=${value.session.session_id}#next-draft`); }
  const phase = task === "next-draft" ? "next" : task === "copy" || copying ? "copy" : task === "end" ? "end" : task === "post-live" ? "after" : "prepare";
  const titleKey = phase === "next" ? c.copied : phase === "copy" ? c.copy : phase === "end" ? c.end : phase === "after" ? c.after : c.prepare;
  const error = command.error || snapshot.error;
  return <section className={s.page} aria-busy={command.busy}>
    <h1>{titleKey}</h1><p>{detail.draft.title}</p>
    {error && <p role="alert">{command.canRetry ? c.uncertain : c.unavailable}</p>}
    {command.canRetry && <button type="button" onClick={() => void command.retry()}>{c.retry}</button>}
    {phase === "prepare" && <>
      <div className={s.card}><h2>{c.source}</h2><p>{c.sourceHelp}</p><div className={s.row}><a href={`/${locale}/studio/console?store=${store}&scene=${scene}`}>{c.console}</a><a href={`/${locale}/settings?store=${store}#facebook-instagram`}>{c.connections}</a></div></div>
      <div className={s.card}><h2>{c.device}</h2><p>{c.deviceHelp}</p><p className={s.note}>{c.deviceDisclaimer}</p></div>
      <div className={s.card}><h2>{c.productsCheck}</h2><p>{c.productsHelp}</p><div className={s.row}><a href={`${claims}#preflight`}>{c.settings}</a><a href={`/${locale}/studio/settings?store=${store}&scene=${scene}`}>{c.templates}</a></div></div>
    </>}
    {phase === "after" && <>
      <div className={s.card}><h2>{snapshot.data?.session.lifecycle === "ended" || snapshot.data?.session.lifecycle === "archived" ? c.ended : snapshot.data?.session.lifecycle === "live" ? c.live : snapshot.data?.session.lifecycle === "draft" ? c.preparing : c.loading}</h2>{snapshot.data && <dl className={s.facts}>
        <div><dt>{c.claims}</dt><dd>{snapshot.data.stats.buyers}</dd></div><div><dt>{c.paid}</dt><dd>{snapshot.data.stats.paid.count}</dd></div>
        <div><dt>{c.net}</dt><dd>{money(locale, snapshot.data.stats.currency, snapshot.data.stats.paid.amount_minor)}</dd></div>
      </dl>}</div>
      <p className={s.note}>{c.paymentBoundary}</p><div className={s.row}><button type="button" disabled={!detail.can_manage || command.blocked} onClick={() => setCopying(true)}>{c.copy}</button><a href={`${base}#review`}>{c.review}</a></div>
    </>}
    {phase === "copy" && <form className={s.card} onSubmit={event => { event.preventDefault(); void command.run<CopyResult>(liveRequest(store, scene, "copy", "POST", { title: title.trim(), scheduled_at: null, expected_version: detail.draft.version }), copied); }}>
      <label>{c.copyName}<input value={title} maxLength={200} onChange={event => setTitle(event.target.value)} disabled={command.blocked} /></label>
      <p>{c.copyIncludes}</p><p>{c.copyExcludes}</p><div className={s.footer}><button type="button" disabled={command.busy || command.canRetry} onClick={() => { setCopying(false); window.location.hash = "post-live"; }}>{c.cancel}</button><button className="primary" type="submit" disabled={!detail.can_manage || command.blocked || !title.trim()}>{c.copyReview}</button></div>
    </form>}
    {phase === "next" && <div className={s.card}><p>{c.copyIncludes}</p>{snapshot.data?.offers.map(offer => <p key={offer.offer_id}>{offer.product_name} · {offer.keyword} · {money(locale, snapshot.data!.stats.currency, offer.live_price_minor ?? offer.sku_price_minor)}</p>)}<p>{c.copyExcludes}</p>{copyResult?.session.session_id === scene && copyResult.conflicts.length > 0 && <div role="alert"><p>{c.conflicts}</p>{copyResult.conflicts.map(conflict => <p key={`${conflict.sku_id}:${conflict.keyword}`}>{conflict.keyword}</p>)}</div>}<button type="button" onClick={() => router.push(`${claims}#preflight`)}>{c.openPrep}</button></div>}
    {phase === "end" && <div className={s.card}><p>{c.endHelp}</p><p>{c.endFacts}</p><div className={s.footer}>
      <button type="button" disabled={command.busy || command.canRetry} onClick={() => { window.location.hash = "preparation"; }}>{c.continue}</button>
      <button type="button" className="primary" disabled={!detail.can_manage || command.blocked || snapshot.data?.session.lifecycle !== "live"} onClick={() => { if (snapshot.data) void command.run(liveRequest(store, scene, "lifecycle", "POST", { action: "end", expected_version: snapshot.data.session.version }), () => { window.location.hash = "post-live"; }); }}>{c.confirmEnd}</button>
    </div></div>}
    {phase === "prepare" && <p className={`${s.note} ${s.muted}`}>{c.localBoundary}</p>}
    <div className={s.footer}><a href={`/${locale}/studio?store=${store}`}>{c.back}</a></div>
  </section>;
}
