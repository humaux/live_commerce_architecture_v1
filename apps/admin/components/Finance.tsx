// Purpose: Signed finance overview, daily records, date editor, basis and audited CSV preview.
// Depends on: react, next/navigation, next/link, @live-commerce/i18n, @live-commerce/ui, shared format, existing finance/order reads and ./finance/money.css.
// Used by: apps/admin/app/[locale]/finance/page.tsx.
// Invariants: server totals only; currencies/environments never combined; session-fenced reads and native audited CSV preserved.
"use client";
import { useEffect, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { moneyCopy, type Locale } from "@live-commerce/i18n";
import { DateControl, TableFrame } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { displayTime } from "../../../packages/format/src/index.ts";
import { financeCSVHref, readFinance, useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { canSeeReports, isSandbox, type FinanceRow } from "@/lib/customers-model";
import { readOrderActions } from "@/lib/orders-client";
import { customersCopy } from "@/lib/customers-copy";
import { reportsCopy } from "@/lib/reports-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import "./finance/money.css";

const DAY_MS = 86_400_000;
function rangeOK(from: string, to: string) {
  const a = Date.parse(`${from}T00:00:00Z`), b = Date.parse(`${to}T00:00:00Z`);
  return Number.isFinite(a) && Number.isFinite(b) && b >= a && (b - a) / DAY_MS <= 91;
}
type View = "overview" | "daily" | "basis" | "dates" | "csv";
/** Display existing finance facts and keep the chosen read-only view in the address for refresh. */
export function Finance({ locale, stores: _stores, store, from, to, today, initialError, renderKey }: {
  locale: Locale; stores: Store[]; store: Store | null; from: string; to: string; today: string; initialError: ReadCode | null; renderKey: string;
}) {
  const c = customersCopy[locale], p = moneyCopy[locale], router = useRouter();
  const [draftFrom, setDraftFrom] = useState(from), [draftTo, setDraftTo] = useState(to);
  const [view, setView] = useState<View>("overview"), [canExport, setCanExport] = useState(false);
  useEffect(() => {
    const restore = () => { const value = new URL(window.location.href).searchParams.get("view"); setView(value === "daily" || value === "basis" || value === "csv" ? value : "overview"); };
    restore(); window.addEventListener("popstate", restore); return () => window.removeEventListener("popstate", restore);
  }, []);
  function open(next: View) {
    if (next === "dates") { setDraftFrom(from); setDraftTo(to); }
    setView(next);
    // Local read-only screen navigation, containing no bearer information. Cancel leaves the applied range intact.
    const url = new URL(window.location.href);
    if (next === "daily" || next === "basis" || next === "csv") url.searchParams.set("view", next); else url.searchParams.delete("view");
    window.history.replaceState(window.history.state, "", url);
  }
  const valid = rangeOK(draftFrom, draftTo), validQuery = rangeOK(from, to);
  const read = useGuardedRead(`${renderKey}|${locale}|${store?.id ?? ""}|${from}|${to}`, store && validQuery ? signal => readFinance(store.id, from, to, signal) : null, initialError);
  useEffect(() => {
    if (!store || initialError) return setCanExport(false);
    const active = new AbortController();
    readOrderActions(store.id, active.signal).then(value => { if (!active.signal.aborted) setCanExport(value.orders_export); }, () => { if (!active.signal.aborted) setCanExport(false); });
    return () => active.abort();
  }, [store, initialError]);
  function submit(event: FormEvent) {
    event.preventDefault();
    if (valid && store) {
      // App Router preserves this component, including when the applied range is unchanged.
      open("overview");
      router.push(`/${locale}/finance?${new URLSearchParams({ store: store.id, from: draftFrom, to: draftTo })}`);
    }
  }
  const failure = read.status === "signed-out" ? c.financeSignedOut : read.status === "forbidden" ? c.financeForbidden : read.status === "not-found" ? (store ? c.notFound : c.noStore) : read.status === "unavailable" ? c.financeUnavailable : "";
  const summary = read.status === "ready" ? read.data : null;
  const title = failure ? p.financeFailure : view === "daily" ? p.dailyTitle : view === "basis" ? p.basisTitle : view === "dates" ? p.financeDates : view === "csv" ? p.csvReady : p.finance;
  return <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="finance">
    <div className="money-page" data-testid="finance-page">
      {view === "overview" && !failure && <p className="money-muted">{p.flow}</p>}
      <h1>{title}</h1>
      {(read.status === "loading" || read.status === "hidden") && <p role="status">{c.financeLoading}</p>}
      {failure && <section className="money-card" role="status"><h2>{p.failure}</h2><p>{read.status === "unavailable" ? p.financeFailureHint : failure}</p><button onClick={read.reload}>{p.retry}</button></section>}
      {!failure && view === "dates" && <form className="money-card money-editor" onSubmit={submit}>
        <p>{p.rangeHint}</p><p>{from} – {to}</p>
        <label>{p.from}<DateControl emptyLabel={p.from} lang={locale} type="date" data-testid="finance-from" value={draftFrom} max={today} required onChange={e => setDraftFrom(e.target.value)} /></label>
        <label>{p.to}<DateControl emptyLabel={p.to} lang={locale} type="date" data-testid="finance-to" value={draftTo} max={today} required onChange={e => setDraftTo(e.target.value)} /></label>
        {!valid && <p role="alert" data-testid="finance-range-invalid">{c.rangeInvalid}</p>}
        <div className="money-actions"><button type="button" onClick={() => open("overview")}>{p.cancel}</button><button className="primary" data-testid="finance-show" disabled={!store || !valid} type="submit">{p.apply}</button></div>
      </form>}
      {!failure && view === "basis" && <><div className="money-grid">
        {[[p.dailyBasisTitle, p.financeBasis], [p.statements, p.settlementBasis], [p.attribution, p.attributionBasis], [p.reconcile, p.reconcileBasis]].map(([heading, text]) => <section className="money-card" key={heading}><h2>{heading}</h2><p>{text}</p></section>)}
      </div><div className="money-actions"><button onClick={() => open("overview")}>{p.backFinance}</button></div></>}
      {!failure && view === "csv" && <section className="money-card money-editor"><h2>{p.csv}</h2><p>{from} – {to}</p><p>{p.csvIntro}</p><p>{p.csvContents}</p><div className="money-actions"><button onClick={() => open("overview")}>{p.backFinance}</button>{store && canExport && validQuery && <a className="money-button primary" data-testid="finance-csv" href={financeCSVHref(store.id, from, to)} download>{p.download}</a>}</div></section>}
      {!failure && (view === "overview" || view === "daily") && <>
        <div className="money-range"><button aria-label={p.financeDates} onClick={() => open("dates")}>{from} – {to}</button><button onClick={read.reload}>{p.retry}</button></div>
        {!validQuery && <p role="alert">{c.rangeInvalid}</p>}
        {summary && <>
          {summary.generated_at && <p className="money-muted">{p.updated} <time data-testid="finance-generated-at" dateTime={summary.generated_at}>{displayTime(locale, summary.generated_at)}</time></p>}
          {view === "overview" ? <>
            {summary.totals.length === 0 && <section className="money-card"><h2>{p.empty}</h2><p>{p.emptyHint}</p></section>}
            {summary.totals.map((row, groupIndex) => groupIndex > 0 && isSandbox(row) ? <section className="money-card money-group" key={`${row.currency}|${row.environment}`}><h2 data-testid="finance-test-badge">{row.currency} · {p.test}</h2><p>{money(locale,row.currency,row.captured_minor)} / − {money(locale,row.currency,row.refunded_minor)} / {money(locale,row.currency,row.net_minor)}</p><p className="money-note">{p.grouping}</p></section> : <div className="money-grid money-group" key={`${row.currency}|${row.environment}`}>
              <section className="money-card"><h2 data-testid={isSandbox(row) ? "finance-test-badge" : undefined}>{row.currency} · {isSandbox(row) ? p.test : p.live}</h2>
                <dl><div><dt>{p.cardCount}</dt><dd>{row.captured_count}</dd></div><div><dt>{p.captured}</dt><dd>{money(locale, row.currency, row.captured_minor)}</dd></div><div><dt>{p.refunded}</dt><dd>− {money(locale, row.currency, row.refunded_minor)}</dd></div><div><dt>{p.net}</dt><dd>{money(locale, row.currency, row.net_minor)}</dd></div></dl><p className="money-note">{p.financeBasis}</p>
              </section>
              <section className="money-card"><h2>{p.other}</h2><dl>{[[p.pickup, row.pickup_collected_minor, row.pickup_collected_count], [p.codCollected, row.cod_collected_minor, row.cod_collected_count], [p.transfer, row.bank_transfer_confirmed_minor, row.bank_transfer_confirmed_count]].map(([label, amount, count]) => <div key={label}><dt>{label}</dt><dd>{money(locale, row.currency, amount as number)} · {count}</dd></div>)}</dl><p className="money-note">{p.separate}</p></section>
            </div>)}
            <p className="money-muted">{p.grouping}</p>
            <div className="money-actions">{canExport && validQuery && <button onClick={() => open("csv")}>{p.csv}</button>}<button onClick={() => open("basis")}>{p.basis}</button>{store && canSeeReports(store) && <Link className="money-button" data-testid="finance-reports" href={`/${locale}/finance/reports?store=${store.id}`}>{reportsCopy[locale].title}</Link>}</div><p className="money-muted">{p.csvHint}</p>
          </> : <><section className="money-card"><h2>{p.dailyCards}</h2><TableFrame label={p.dailyCards} scrollHint={p.daily}><table data-testid="finance-table"><thead><tr>{[p.day,p.environment,p.count,p.captured,p.refunded,p.net].map(label => <th key={label}>{label}</th>)}</tr></thead><tbody>{summary.rows.map(row => <FinanceLine key={`${row.day}|${row.currency}|${row.environment}`} row={row} locale={locale} />)}</tbody><tfoot>{summary.totals.map(row => <FinanceLine key={`${row.currency}|${row.environment}`} row={row} locale={locale} />)}</tfoot></table></TableFrame></section>
            <section className="money-card"><h2>{p.dailyOther}</h2>{summary.rows.map(row => <div key={`${row.day}|${row.currency}|${row.environment}`} data-testid={`finance-other-${row.day}-${row.currency}`}><h3>{row.day} · {row.currency} · {isSandbox(row) ? p.test : p.live}</h3><dl>{[[p.pickup,row.pickup_collected_minor,row.pickup_collected_count],[p.codCollected,row.cod_collected_minor,row.cod_collected_count],[p.transfer,row.bank_transfer_confirmed_minor,row.bank_transfer_confirmed_count]].map(([label, amount, count]) => <div key={label}><dt>{label}</dt><dd data-testid={label === p.codCollected ? "finance-cod-collected" : label === p.transfer ? "finance-transfer-confirmed" : undefined}>{money(locale,row.currency,amount as number)} / {count}</dd></div>)}</dl></div>)}<p className="money-note">{p.dailyBasis}</p></section>
            <div className="money-actions"><button onClick={() => open("overview")}>{p.backFinance}</button></div>
          </>}
        </>}
        {view === "overview" && <nav className="money-footer"><button className="primary" onClick={() => open("daily")}>{p.daily}</button>{store && <Link className="money-button" href={`/${locale}/settings/settlements?store=${store.id}`}>{p.statements}</Link>}</nav>}
      </>}
    </div>
  </WorkspaceFrame>;
}
function FinanceLine({ row, locale }: { row: FinanceRow; locale: Locale }) {
  const p = moneyCopy[locale];
  return <tr data-testid={row.day ? `finance-row-${row.day}-${row.currency}` : `finance-total-${row.currency}`}><td>{row.day || p.total} · {row.currency}</td><td>{isSandbox(row) ? p.test : p.live}</td><td>{row.captured_count}</td><td>{money(locale,row.currency,row.captured_minor)}</td><td>− {money(locale,row.currency,row.refunded_minor)}</td><td>{money(locale,row.currency,row.net_minor)}</td></tr>;
}
