"use client";

// Admin landing (/{locale}): what needs the merchant today. One guarded read of BFF GET /api/stores/{store}/tools/dashboard -> Go
// GET /v1/admin/stores/{id}/dashboard (internal/httpapi/merchanttools.go -> internal/merchanttools.Dashboard; contract storefront-v2 G1).
// Shows placed-order counts, GMV by payment method (the finance definer's numbers, never recomputed here, one block per currency and
// environment), the five to-do counters as links into the Orders / Inventory pages, the ten latest orders, and the quick actions
// (create order, import/export products). Read-only: it writes nothing and never decides a rule; the session fence is useGuardedRead.
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { readDashboard } from "@/lib/merchant-tools-client";
import { toolsCopy } from "@/lib/merchant-tools-copy";
import { displayTime } from "@/lib/orders-model";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { Badge, TableFrame } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";
import "./orders.css";
import "./customers.css";
import "./merchant-tools.css";

const storeQuery = (store: Store | null, extra: Record<string, string> = {}) => {
  const params = new URLSearchParams(extra);
  if (store) params.set("store", store.id);
  return params.size ? `?${params}` : "";
};

export function Dashboard({
  locale, stores, store, initialError, renderKey,
}: {
  locale: Locale; stores: Store[]; store: Store | null; initialError: ReadCode | null; renderKey: string;
}) {
  const c = toolsCopy[locale].dashboard;
  const read = useGuardedRead(`${renderKey}|${locale}|${store?.id ?? ""}`, store ? (signal) => readDashboard(store.id, signal) : null, initialError);
  const failure =
    read.status === "signed-out" ? c.signedOut
    : read.status === "forbidden" ? c.forbidden
    : read.status === "not-found" ? (store ? c.notFound : c.noStore)
    : read.status === "unavailable" ? c.unavailable
    : "";
  const d = read.data;
  const link = (path: string, extra?: Record<string, string>) => `/${locale}/${path}${storeQuery(store, extra)}`;
  const todos = d
    ? ([
        [c.transfer, d.todos.awaiting_transfer_confirmation, link("orders", { state: "AWAITING_TRANSFER" }), "todo-transfer"],
        [c.ship, d.todos.to_ship, link("orders", { state: "unshipped" }), "todo-ship"],
        [c.label, d.todos.cvs_awaiting_label, link("orders", { state: "unshipped" }), "todo-label"],
        [c.lowStock, d.todos.low_stock_skus, link("inventory"), "todo-stock"],
        [c.refunds, d.todos.open_refunds, link("orders"), "todo-refunds"],
      ] as const)
    : [];
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="dashboard">
      <div className="orders-page customers-page dashboard-page" data-testid="dashboard-page">
        <AdminPageHeader locale={locale} description={c.subtitle} actions={<div className="mt-actions" data-testid="dashboard-actions" aria-label={c.actionsTitle}>
          <Link className="primary" href={link("orders/new")} data-testid="action-create-order">{c.createOrder}</Link>
          <Link href={link("products/import")} data-testid="action-import">{c.importProducts}</Link>
          <Link href={link("inventory")} data-testid="action-inventory">{c.inventory}</Link>
        </div>} />
        {(read.status === "loading" || read.status === "hidden") && <p className="orders-message" role="status">{c.loading}</p>}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>{c.retry}</button>
          </div>
        )}
        {read.status === "ready" && d && (
          <>
            <section className="mt-card" aria-label={c.todosTitle} data-testid="dashboard-todos">
              <h2>{c.todosTitle}</h2>
              <ul className="mt-todos">
                {todos.map(([label, n, href, id]) => (
                  <li key={id} className={n > 0 ? "mt-hot" : undefined}>
                    <Link href={href} data-testid={id}>
                      <span>{label}</span>
                      <Badge tone={n > 0 ? "warning" : "neutral"}>{n}</Badge>
                    </Link>
                  </li>
                ))}
              </ul>
              {todos.every(([, n]) => n === 0) && <p className="mt-note">{c.nothing}</p>}
            </section>
            <div className="mt-grid">
              <section className="mt-card" data-testid="dashboard-orders">
                <h2>{c.ordersTitle}</h2>
                <dl className="mt-pair">
                  <div><dt>{c.today}</dt><dd data-testid="orders-today">{d.orders.today}</dd></div>
                  <div><dt>{c.week}</dt><dd data-testid="orders-week">{d.orders.last_7_days}</dd></div>
                </dl>
                <p className="mt-note">{c.ordersNote}</p>
              </section>
              <section className="mt-card" data-testid="dashboard-gmv">
                <h2>{c.gmvTitle}</h2>
                {d.gmv.length === 0 && <p className="mt-note">{c.noSales}</p>}
                {d.gmv.map((g) => (
                  <TableFrame key={`${g.currency}|${g.environment}`} label={c.gmvTitle} scrollHint={presentationCopy[locale].scroll}>
                    <table className="mt-table">
                      <caption className="mt-note" style={{ textAlign: "left", padding: "8px 14px 0" }}>
                        {g.currency} · {g.environment === "LIVE" ? c.live : c.sandbox}
                      </caption>
                      <thead><tr><th scope="col" /><th scope="col" className="num">{c.today}</th><th scope="col" className="num">{c.week}</th></tr></thead>
                      <tbody>
                        {([["card", c.card, "card_minor"], ["bank", c.bank, "bank_transfer_minor"], ["pickup", c.pickup, "pay_at_pickup_minor"], ["cod", c.cod, "cod_minor"]] as const).map(([id, label, key]) => (
                          <tr key={id}>
                            <th scope="row">{label}</th>
                            <td className="num">{money(locale, g.currency, g.today[key])}</td>
                            <td className="num">{money(locale, g.currency, g.last_7_days[key])}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </TableFrame>
                ))}
              </section>
            </div>
            <section className="mt-card" data-testid="dashboard-latest">
              <h2>{c.latestTitle}</h2>
              {d.latest_orders.length === 0 ? <p className="mt-note">{c.noOrders}</p> : (
                <TableFrame label={c.latestTitle} scrollHint={presentationCopy[locale].scroll}>
                  <table className="mt-table">
                    <thead><tr><th scope="col">{c.order}</th><th scope="col">{c.created}</th><th scope="col">{c.status}</th><th scope="col">{c.mode}</th><th scope="col" className="num">{c.total}</th></tr></thead>
                    <tbody>
                      {d.latest_orders.map((o) => (
                        <tr key={o.order_id}>
                          <td><Link href={link("orders", { order: o.order_id })}>{o.order_id.slice(0, 8)}</Link></td>
                          <td>{displayTime(locale, o.created_at)}</td>
                          <td>{c.states[o.commercial_state] ?? o.commercial_state}</td>
                          <td>{c.modes[o.payment_mode] ?? o.payment_mode}</td>
                          <td className="num">{money(locale, o.currency, o.total_minor)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </TableFrame>
              )}
              <p className="mt-note"><Link href={link("orders")}>{c.allOrders}</Link></p>
            </section>
          </>
        )}
      </div>
    </WorkspaceFrame>
  );
}
