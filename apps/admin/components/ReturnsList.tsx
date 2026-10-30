// Purpose: Owns the merchant returns page body (/{locale}/returns): the store's RMA list with one state filter
//   and the "failed refunds to redo" (cancel-refund-gaps) section (contracts/returns-v1.md §2/§3; unit W3-U5).
// Depends on: react, @live-commerce/i18n, @live-commerce/ui, @live-commerce/format, @/lib/model,
//   @/lib/customers-client (useGuardedRead session fence), @/lib/returns-client, @/lib/returns-model,
//   @/lib/returns-copy, ./WorkspaceFrame, ./AdminPageHeader, ./order-ops.css, ./orders.css, ./order-actions.css
// Used by: apps/admin/app/[locale]/returns/page.tsx
// Invariants: read-only page — every mutating step happens inside the order detail panel (OrderReturns). Rows open a read-only RMA record; its original-order link
//   opens /{locale}/orders?store=…&state=all&order=…. The list re-reads after refresh; a
//   drifted payload fails closed as "unavailable" (strict parsers in returns-model).
"use client";

import { orderOpsCopy, type Locale } from "@live-commerce/i18n";
import { Badge, TableFrame } from "@live-commerce/ui";
import { displayTime, money } from "@live-commerce/format";
import type { Store } from "@/lib/model";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { readCancelRefundGaps, readReturns } from "@/lib/returns-client";
import { rmaStates, type RefundGap, type Rma, type RmaState } from "@/lib/returns-model";
import { returnsCopy, type ReturnsCopy } from "@/lib/returns-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import "./orders.css";
import "./order-actions.css";
import "./order-ops.css";

/** Maps a guarded-read failure to merchant copy. */
function failureText(status: string, c: ReturnsCopy): string {
  return status === "signed-out"
    ? c.signedOut
    : status === "forbidden"
      ? c.forbidden
      : status === "not-found"
        ? c.noStore
        : status === "unavailable"
          ? c.unavailable
          : "";
}

/** Sums the registered/received quantities of one RMA for the contents column. */
function lineTotals(rma: Rma): { registered: number; received: number | null } {
  let registered = 0;
  let received = 0;
  let anyReceived = false;
  for (const line of rma.lines) {
    registered += line.qty_registered;
    if (line.qty_received !== null) {
      anyReceived = true;
      received += line.qty_received;
    }
  }
  return { registered, received: anyReceived ? received : null };
}

/** One store's RMA table plus the state filter (filter = real links so a refresh keeps it). */
function ReturnsTable({
  locale,
  store,
  state,
  list,
  c,
}: {
  locale: Locale;
  store: Store;
  state: RmaState | "";
  list: Rma[];
  c: ReturnsCopy;
}) {
  const href = (next: RmaState | "") =>
    `/${locale}/returns?store=${store.id}&view=requests${next ? `&state=${next}` : ""}`;
  const recordHref = (id: string) => `${href(state)}&rma=${id}`;
  return (
    <section className="orders-section" data-testid="returns-list" aria-label={c.pageTitle}>
      <nav className="returns-filter" aria-label={c.stateFilter} data-testid="returns-filter">
        <a href={href("")} aria-current={state === "" ? "true" : undefined} data-testid="returns-filter-all">
          {c.allStates}
        </a>
        {rmaStates.map((item) => (
          <a
            key={item}
            href={href(item)}
            aria-current={state === item ? "true" : undefined}
            data-testid={`returns-filter-${item.toLowerCase()}`}
          >
            {c.states[item]}
          </a>
        ))}
      </nav>
      {list.length === 0 && (
        <p className="orders-message" data-testid="returns-empty">
          {state ? c.emptyFiltered : c.empty}
        </p>
      )}
      {list.length > 0 && (
        <TableFrame label={c.pageTitle} scrollHint={c.scrollHint}>
          <table>
            <thead>
              <tr>
                <th>{c.colOrder}</th>
                <th>{c.colLines}</th>
                <th>{c.colReason}</th>
                <th>{c.colState}</th>
                <th>{c.colCreated}</th>
                <th>{c.colUpdated}</th>
                <th>{c.viewOrder}</th>
              </tr>
            </thead>
            <tbody>
              {list.map((rma) => {
                const totals = lineTotals(rma);
                return (
                  <tr key={rma.id} data-testid={`returns-row-${rma.id}`}>
                    <td>
                      <code>{rma.order_id.slice(0, 8)}</code>
                    </td>
                    <td>{c.lineSummary(totals.registered, totals.received)}</td>
                    <td>{rma.reason}</td>
                    <td>
                      <Badge
                        tone={rma.state === "CLOSED" ? "success" : rma.state === "CANCELLED" ? "neutral" : "warning"}
                        data-state={rma.state}
                      >
                        {c.states[rma.state]}
                      </Badge>
                    </td>
                    <td>{displayTime(locale, rma.created_at)}</td>
                    <td>{displayTime(locale, rma.updated_at)}</td>
                    <td>
                      <a href={recordHref(rma.id)} data-testid={`returns-open-${rma.id}`}>
                        {c.viewOrder}
                      </a>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </TableFrame>
      )}
    </section>
  );
}

/** Cancelled card orders whose in-flight refund later failed (returns-v1 §3): the merchant must refund again. */
function GapsSection({
  locale,
  store,
  gaps,
  c,
}: {
  locale: Locale;
  store: Store;
  gaps: RefundGap[];
  c: ReturnsCopy;
}) {
  const m = (minor: number) => money(locale, store.currency, minor);
  return (
    <section className="orders-section" data-testid="refund-gaps" aria-label={c.gapsTitle}>
      <h2>{c.gapsTitle}</h2>
      <p className="orders-empty">{c.gapsIntro}</p>
      {gaps.length === 0 && (
        <p className="orders-message" data-testid="refund-gaps-empty">
          {c.gapsEmpty}
        </p>
      )}
      {gaps.length > 0 && (
        <TableFrame label={c.gapsTitle} scrollHint={c.scrollHint}>
          <table>
            <thead>
              <tr>
                <th>{c.colOrder}</th>
                <th>{c.colCaptured}</th>
                <th>{c.colRefunded}</th>
                <th>{c.colGap}</th>
                <th>{c.colCancelledAt}</th>
                <th>{c.colReason}</th>
                <th>{c.goRefund}</th>
              </tr>
            </thead>
            <tbody>
              {gaps.map((gap) => (
                <tr key={gap.order_id} data-testid={`refund-gap-${gap.order_id}`}>
                  <td>
                    <code>{gap.order_id.slice(0, 8)}</code>
                  </td>
                  <td>{m(gap.captured_minor)}</td>
                  <td>{m(gap.refunded_minor)}</td>
                  <td>
                    <strong data-testid={`refund-gap-amount-${gap.order_id}`}>{m(gap.gap_minor)}</strong>
                  </td>
                  <td>{displayTime(locale, gap.cancelled_at)}</td>
                  <td>{c.gapReason}</td>
                  <td>
                    <a
                      href={`/${locale}/orders?store=${store.id}&state=all&order=${gap.order_id}`}
                      data-testid={`refund-gap-open-${gap.order_id}`}
                    >
                      {c.goRefund}
                    </a>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableFrame>
      )}
    </section>
  );
}

/** Owns the returns page: two guarded reads (RMA list, refund gaps), session-fenced and re-run on focus. */
export function ReturnsList({
  locale,
  stores,
  store,
  state,
  initialError,
  renderKey,
  view = "",
  rma = "",
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  state: RmaState | "";
  initialError: ReadCode | null;
  renderKey: string;
  view?: "" | "requests";
  rma?: string;
}) {
  const c = returnsCopy[locale];
  const signed = orderOpsCopy[locale];
  const list = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}|${state}`,
    store ? (signal) => readReturns(store.id, state, signal) : null,
    initialError,
  );
  const gaps = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}|gaps`,
    store ? (signal) => readCancelRefundGaps(store.id, signal) : null,
    initialError,
  );
  const failure = failureText(list.status, c) || failureText(gaps.status, c);
  const requests = view === "requests" || !!state || !!rma;
  const record: Rma | undefined = list.status === "ready" ? list.data?.find((item) => item.id === rma) : undefined;
  const requestsHref = `/${locale}/returns?store=${store?.id ?? ""}&view=requests${state ? `&state=${state}` : ""}`;
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="returns">
      <div className="orders-page returns-page" data-testid="returns-page">
        <AdminPageHeader
          locale={locale}
          title={record ? record.state === "CLOSED" ? signed.completed : signed.requests : requests ? signed.requests : signed.returns}
          description={signed.aftersales + (stores.length > 1 && store ? ` — ${store.name}` : "")}
        />
        {!requests && store && <button type="button" className="ops-wide" onClick={() => window.location.assign(requestsHref)}>{signed.view}</button>}
        {record && store && <section className="ops-review" data-testid="returns-record">
          <Badge tone={record.state === "CLOSED" ? "success" : "warning"}>{c.states[record.state]}</Badge>
          <dl><div><dt>{c.colReason}</dt><dd>{record.reason}</dd></div>
            <div><dt>{c.colLines}</dt><dd>{c.lineSummary(lineTotals(record).registered, lineTotals(record).received)}</dd></div>
            <div><dt>{c.colCreated}</dt><dd>{displayTime(locale, record.created_at)}</dd></div>
            <div><dt>{c.colUpdated}</dt><dd>{displayTime(locale, record.updated_at)}</dd></div>
          </dl>
          <div className="ops-actions"><a href={requestsHref}>{signed.back}</a>
            <a href={`/${locale}/orders?store=${store.id}&state=all&order=${record.order_id}`}>{c.viewOrder}</a></div>
        </section>}
        {rma && list.status === "ready" && !record && <p role="status">{c.unavailable}</p>}
        {(list.status === "loading" || list.status === "hidden") && (
          <p className="orders-message" role="status" data-testid="returns-loading">
            {c.loading}
          </p>
        )}
        {failure && (
          <div className="orders-message" role="status" data-testid="returns-error">
            <p>{failure}</p>
            <button type="button" onClick={list.reload}>
              {c.retry}
            </button>
          </div>
        )}
        {requests && !rma && list.status === "ready" && list.data && store && (
          <ReturnsTable locale={locale} store={store} state={state} list={list.data} c={c} />
        )}
        {gaps.status === "ready" && gaps.data && store && (
          <GapsSection locale={locale} store={store} gaps={gaps.data} c={c} />
        )}
      </div>
    </WorkspaceFrame>
  );
}
