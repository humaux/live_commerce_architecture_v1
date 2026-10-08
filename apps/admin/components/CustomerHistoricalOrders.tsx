// Purpose: read-only SHOPLINE archive in customer detail, with guarded 50-row keyset pagination.
// Depends on: import-history-client/model/copy -> exact historical-orders BFF -> Go customer_historical.go; shared money/time/TableFrame.
// Used by: CustomerDetail (parent-owned active/imported gating); archive is excluded from revenue/reports, no live order actions.
"use client";
import { useId, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { TableFrame } from "@live-commerce/ui";
import { money, displayTime } from "@live-commerce/format";
import { useGuardedRead } from "../lib/customers-client";
import { readHistoricalOrders } from "../lib/import-history-client";
import { importHistoryCopy } from "../lib/import-history-copy";
import "./import-history.css";

/** Show the server's archived SHOPLINE facts; scope changes reset private data/cursors through keyed mounting. */
export function CustomerHistoricalOrders(props: { locale: Locale; store: string; customer: string; boundary: string }) {
  return <HistoryBody key={`${props.store}|${props.customer}|${props.boundary}`} {...props} />;
}

function HistoryBody({ locale, store, customer, boundary }: { locale: Locale; store: string; customer: string; boundary: string }) {
  const c = importHistoryCopy[locale]; const heading = useId();
  const [trail, setTrail] = useState([""]); const [page, setPage] = useState(0);
  const after = trail[page];
  const read = useGuardedRead(`${store}|${customer}|${boundary}|${after}`,
    (signal) => readHistoricalOrders(store, customer, after, boundary, signal), null);
  const failure = read.status === "signed-out" ? c.signedOut : read.status === "forbidden" ? c.forbidden
    : read.status === "not-found" ? c.notFound : read.status === "unavailable" ? c.unavailable : "";
  const ready = read.status === "ready" && read.data !== null;
  const refresh = () => {
    setTrail([""]); setPage(0); read.reload();
  };
  return <section className="import-history" data-testid="customer-historical-orders" aria-labelledby={heading}>
    <div className="import-history-heading"><h2 id={heading}>{c.title}</h2>
      <button type="button" onClick={refresh} disabled={read.status === "loading" || read.status === "hidden"}>{c.refresh}</button></div>
    <p data-testid="historical-orders-basis">{c.basis}</p>
    {(read.status === "loading" || read.status === "hidden") && <p role="status">{c.loading}</p>}
    {failure && <div role="alert"><p>{failure}</p>
      {read.status !== "signed-out" && <button type="button" onClick={read.reload}>{c.retry}</button>}</div>}
    {ready && read.data && <>
      <p role="status" data-testid="historical-orders-count">{c.count(read.data.items.length, read.data.total)}</p>
      {read.data.items.length === 0 ? <p>{read.data.total === 0 ? c.empty : c.emptyPage}</p> : <TableFrame label={c.title} scrollHint={c.scroll} className="import-history-frame" scrollClassName="import-history-scroll">
        <table><thead><tr>
          <th scope="col">{c.order}</th><th scope="col">{c.date}</th><th scope="col">{c.status}</th>
          <th scope="col">{c.amount}</th><th scope="col">{c.items}</th><th scope="col">{c.city}</th>
        </tr></thead><tbody>{read.data.items.map((order) => <tr key={order.order_id}>
          <td>{order.order_id}</td><td><time dateTime={order.ordered_at}>{displayTime(locale, order.ordered_at)}</time></td>
          <td>{order.status}</td><td className="import-history-money">{money(locale, order.currency, order.total_minor)}</td>
          <td>{order.items_summary || c.notProvided}</td><td>{order.city ?? c.notProvided}</td>
        </tr>)}</tbody></table>
      </TableFrame>}
    </>}
    <nav className="import-history-pages" aria-label={c.title}>
      <button type="button" disabled={!ready || page === 0} onClick={() => { if (ready && page > 0) setPage(page - 1); }}>{c.previous}</button>
      <span>{c.page(page + 1)}</span>
      <button type="button" disabled={!ready || !read.data?.next_cursor || read.data.next_cursor === after} onClick={() => {
        const next = read.data?.next_cursor; if (!ready || !next || next === after) return;
        // Two clicks against the same rendered page select the same next page, never skip an unseen cursor.
        setTrail((old) => [...old.slice(0, page + 1), next]); setPage(page + 1);
      }}>{c.next}</button>
    </nav>
  </section>;
}
