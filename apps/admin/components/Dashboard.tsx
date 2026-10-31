// Purpose: Signed U:dashboard content on the authenticated merchant root; task states use existing APIs.
// Depends on: WorkspaceFrame, guarded dashboard/finance reads, dashboard tasks, server-response formatting and locale copy.
// Used by: apps/admin/app/[locale]/page.tsx.
// Invariants: the shell belongs to PARITY-SHELL; no fabricated counters, client money arithmetic or provider live status.
"use client";
import Link from "next/link";
import { instantToTaipei } from "@live-commerce/format";
import { entryParityCopy, type Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { readFinance, useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { readDashboard } from "@/lib/merchant-tools-client";
import { toolsCopy } from "@/lib/merchant-tools-copy";
import { displayTime } from "@/lib/orders-model";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { DashboardTasks, type DashboardView } from "./dashboard/DashboardTasks";
import "./dashboard/dashboard.css";
const storeQuery = (store: Store | null, extra: Record<string, string> = {}) => {
  const params = new URLSearchParams(extra);
  if (store) params.set("store", store.id);
  return params.size ? `?${params}` : "";
};
/** Render real order facts. Missing signed aggregates are visibly unavailable, never inferred from latest-ten orders. */
export function Dashboard({ locale, stores: _stores, store, initialError, renderKey, view = "overview" }: {
  locale: Locale; stores: Store[]; store: Store | null; initialError: ReadCode | null; renderKey: string; view?: DashboardView;
}) {
  const c = entryParityCopy[locale].dashboard, old = toolsCopy[locale].dashboard;
  const scope = `${renderKey}|${locale}|${store?.id ?? ""}|${view}`;
  const read = useGuardedRead(scope, store ? signal => readDashboard(store.id, signal) : null, initialError);
  const d = read.data;
  // Read the finance day from the dashboard's server timestamp; never use the browser timezone for this contract.
  const day = d ? instantToTaipei(d.generated_at).slice(0, 10) : "";
  const finance = useGuardedRead(`${scope}|${day}`, store && day && view === "overview" ? signal => readFinance(store.id, day, day, signal) : null, initialError);
  const failure = read.status === "signed-out" ? old.signedOut : read.status === "forbidden" ? old.forbidden : read.status === "not-found" ? (store ? old.notFound : old.noStore) : read.status === "unavailable" ? old.unavailable : "";
  const link = (path: string, extra?: Record<string, string>) => `/${locale}/${path}${storeQuery(store, extra)}`;
  const todos = d ? [[c.ship, d.todos.to_ship, "orders", { state: "unshipped" }, "todo-ship"],
    [old.transfer, d.todos.awaiting_transfer_confirmation, "orders", { state: "AWAITING_TRANSFER" }, "todo-transfer"],
    [old.label, d.todos.cvs_awaiting_label, "orders", { state: "unshipped" }, "todo-label"],
    [old.lowStock, d.todos.low_stock_skus, "inventory", {}, "todo-stock"],
    [old.refunds, d.todos.open_refunds, "orders", {}, "todo-refunds"]] as const : [];
  return <WorkspaceFrame locale={locale} storeName={store?.name ?? old.noStore} active="dashboard">
    <div className={`entry-dashboard${view !== "overview" ? " entry-dashboard-task" : ""}`} data-testid="dashboard-page">
      {view !== "overview" && store ? <DashboardTasks key={scope} locale={locale} store={store.id} currency={store.currency} view={view} scope={scope} link={link} /> : <>
        <header className="entry-dashboard-heading"><div><h1>{c.title}</h1>{d && <p>{day} · {c.ship} {d.todos.to_ship}</p>}</div>
          <div className="entry-dashboard-actions" data-testid="dashboard-actions"><Link href={link("orders/new")} data-testid="action-create-order">{old.createOrder}</Link><Link href={link("products/new")}>{c.createProduct}</Link><Link className="entry-task-primary" href={link("studio")}>{c.startLive}</Link></div>
        </header>
        {(read.status === "loading" || read.status === "hidden") && <p role="status">{c.loading}</p>}
        {failure && <div role="status"><p>{failure}</p><button onClick={read.reload}>{old.retry}</button></div>}
        {read.status === "ready" && d && <>
          <dl className="entry-dashboard-metrics">
            <div><dt>{c.today}</dt><dd data-testid="orders-today">{d.orders.today}</dd></div>
            <div><dt>{c.ship}</dt><dd>{d.todos.to_ship}</dd></div>
            <div><dt>{c.net}</dt><dd>{finance.status === "ready" && finance.data?.totals.length ? finance.data.totals.map(row => <span key={`${row.currency}:${row.environment}`}>{money(locale, row.currency, row.net_minor)}{row.environment !== "LIVE" && <small>{old.sandbox}</small>}</span>) : c.unavailable}</dd></div>
            <div><dt>{c.pending}</dt><dd className="entry-missing-value">{c.unavailable}</dd></div>
          </dl>
          <div className="entry-dashboard-grid">
            <section className="entry-dashboard-todos entry-dashboard-surface" data-testid="dashboard-todos"><h2>{c.todos}</h2>
              <ul>{todos.filter(([, n]) => n > 0).map(([label, count, path, query, id]) => <li key={id}><span>{label} {count}</span><Link data-testid={id} href={link(path, query)}>{c.process}</Link></li>)}</ul>
              {todos.every(([, n]) => n === 0) && <p>{old.nothing}</p>}
            </section>
            <section className="entry-dashboard-latest entry-dashboard-surface" data-testid="dashboard-latest"><header><h2>{c.latest}</h2><Link href={link("orders")}>{c.allOrders}</Link></header>
              {d.latest_orders.length ? <div className="entry-orders-scroll"><table><thead><tr><th>{c.order}</th><th>{c.amount}</th><th>{c.status}</th><th className="entry-desktop-only">{c.time}</th></tr></thead><tbody>{d.latest_orders.map(o => <tr key={o.order_id}>
                <td><Link href={link("orders", { order: o.order_id })}>{o.order_id.slice(0, 8)}</Link></td><td>{money(locale, o.currency, o.total_minor)}</td><td>{old.states[o.commercial_state] ?? c.unavailable}</td><td className="entry-desktop-only">{displayTime(locale, o.created_at)}</td>
              </tr>)}</tbody></table></div> : <p>{c.noOrders}</p>}
              <p className="entry-task-muted entry-orders-note">{c.amountsNote}</p>
            </section>
            <section className="entry-dashboard-live entry-dashboard-surface"><h2>{c.liveStatus}</h2><p>{c.liveUnknown}</p><Link href={link("studio")}>{c.openConsole}</Link></section>
          </div>
        </>}
        <nav className="entry-dashboard-task-links" aria-label={c.help}>
          <Link href={link("", { view: "help" })}>{c.help}</Link><Link href={link("", { view: "attention" })}>{c.attention}</Link>
          <Link href={link("", { view: "navigation" })}>{c.navigation}</Link><Link href={link("", { view: "delivery" })}>{c.quickstart}</Link><Link href={link("", { view: "capabilities" })}>{c.capabilities}</Link>
        </nav>
      </>}
    </div>
  </WorkspaceFrame>;
}
