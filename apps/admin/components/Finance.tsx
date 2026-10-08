// Purpose: Owns the merchant finance summary and CSV download controls.
// Depends on: react, next/navigation, @live-commerce/i18n, @/lib/model, @/lib/client, @/lib/customers-client, @/lib/customers-model, @/lib/orders-client, @/lib/customers-copy, ./WorkspaceFrame, ./AdminPageHeader, @live-commerce/ui, @/lib/presentation-copy, ./orders.css, ./order-actions.css, ./customers.css
// Used by: apps/admin/app/[locale]/finance/page.tsx
"use client";

// Merchant finance summary (/{locale}/finance): date range (native date inputs), daily table + totals row, CSV link,
// test-mode badge. BFF GET /api/stores/{store}/finance/summary?from&to (orders:read) and the plain download link
// GET .../finance/summary.csv (orders:read + orders:export, streamed by the BFF, never held in JS) -> Go
// /v1/admin/stores/{id}/finance/summary[.csv] (internal/httpapi/finance.go; internal/reporting.Finance/FinanceCSV).
// The CSV link shows only when GET order-actions says orders_export (same probe MerchantOrders uses).
// Money uses the shared `money` formatter; sums come from Go (I05), nothing is recomputed here.
import { useEffect, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { financeCSVHref, readFinance, useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { canSeeReports, isSandbox, type FinanceRow } from "@/lib/customers-model";
import { readOrderActions } from "@/lib/orders-client";
import Link from "next/link";
import { customersCopy } from "@/lib/customers-copy";
import { reportsCopy } from "@/lib/reports-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { DateControl, TableFrame } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";
import "./orders.css";
import "./order-actions.css";
import "./customers.css";

const DAY_MS = 86_400_000;
// Same rule as the BFF grammar and Go D13: start not after end, at most 91 days apart, real calendar days.
function rangeOK(from: string, to: string) {
  const a = Date.parse(`${from}T00:00:00Z`);
  const b = Date.parse(`${to}T00:00:00Z`);
  return Number.isFinite(a) && Number.isFinite(b) && b >= a && (b - a) / DAY_MS <= 91;
}

/** Owns the merchant finance summary and CSV download controls. Loads finance data through customers-client and exposes the server CSV download. */
export function Finance({
  locale,
  stores,
  store,
  from,
  to,
  today,
  initialError,
  renderKey,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  from: string;
  to: string;
  today: string;
  initialError: ReadCode | null;
  renderKey: string;
}) {
  const c = customersCopy[locale];
  const router = useRouter();
  const [draftFrom, setDraftFrom] = useState(from);
  const [draftTo, setDraftTo] = useState(to);
  const [canExport, setCanExport] = useState(false);
  const valid = rangeOK(draftFrom, draftTo);
  const validQuery = rangeOK(from, to);
  const read = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}|${from}|${to}`,
    store && validQuery ? (signal) => readFinance(store.id, from, to, signal) : null,
    initialError,
  );

  // Permission probe for the CSV link only; a failed probe hides it and Go re-authorizes the download anyway.
  useEffect(() => {
    if (!store || initialError) return setCanExport(false);
    const active = new AbortController();
    readOrderActions(store.id, active.signal).then((value) => setCanExport(value.orders_export), () => setCanExport(false));
    return () => active.abort();
  }, [store, initialError]);

  const navigate = (nextStore: string, nextFrom: string, nextTo: string) => {
    const params = new URLSearchParams();
    if (nextStore) params.set("store", nextStore);
    params.set("from", nextFrom);
    params.set("to", nextTo);
    router.push(`/${locale}/finance?${params}`);
  };
  function submit(event: FormEvent) {
    event.preventDefault();
    if (valid) navigate(store?.id ?? "", draftFrom, draftTo);
  }
  const failure =
    read.status === "signed-out" ? c.financeSignedOut
    : read.status === "forbidden" ? c.financeForbidden
    : read.status === "not-found" ? (store ? c.notFound : c.noStore)
    : read.status === "unavailable" ? c.financeUnavailable
    : "";
  const summary = read.data;
  const sandbox = !!summary && [...summary.rows, ...summary.totals].some(isSandbox);
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="finance">
      <div className="orders-page customers-page" data-testid="finance-page">
        <AdminPageHeader locale={locale} description={c.financeSubtitle} />
        <form className="orders-controls customers-finance-controls" onSubmit={submit}>
          <label>
            {c.from}
            <DateControl emptyLabel={presentationCopy[locale].date} lang={locale} type="date" data-testid="finance-from" value={draftFrom} max={today} required
              onChange={(event) => setDraftFrom(event.target.value)} />
          </label>
          <label>
            {c.to}
            <DateControl emptyLabel={presentationCopy[locale].date} lang={locale} type="date" data-testid="finance-to" value={draftTo} max={today} required
              onChange={(event) => setDraftTo(event.target.value)} />
          </label>
          <button type="submit" data-testid="finance-show" disabled={!store || !valid || read.status === "loading"}>
            {c.show}
          </button>
          {store && canExport && validQuery && (
            <a className="orders-export" data-testid="finance-csv" href={financeCSVHref(store.id, from, to)} download>
              {c.csv}
            </a>
          )}
          {store && canSeeReports(store) && (
            <Link className="orders-export" href={`/${locale}/finance/reports?store=${store.id}`} data-testid="finance-reports">{reportsCopy[locale].title}</Link>
          )}
          {!valid && <p className="orders-bad" role="alert" data-testid="finance-range-invalid">{c.rangeInvalid}</p>}
          <p className="orders-export-hint">{c.csvHint}</p>
        </form>
        {!validQuery && <p className="orders-message" role="status">{c.rangeInvalid}</p>}
        {(read.status === "loading" || read.status === "hidden") && validQuery && (
          <p className="orders-message" role="status">{c.financeLoading}</p>
        )}
        {failure && validQuery && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>{c.retry}</button>
          </div>
        )}
        {read.status === "ready" && summary && (
          <>
            {sandbox && <p className="orders-badge orders-tone-warning" data-testid="finance-test-badge">{c.testBadge}</p>}
            <p className="orders-hint">{c.timezone(summary.timezone)}</p>
            {summary.rows.length === 0 ? (
              <p className="orders-message" role="status">{c.financeEmpty}</p>
            ) : (
              <TableFrame label={c.financeTitle} scrollHint={presentationCopy[locale].scroll}>
                <table className="orders-actions-table customers-finance-table" data-testid="finance-table">
                  <thead>
                    <tr>
                      <th>{c.day}</th><th>{c.currency}</th><th>{c.environment}</th>
                      <th>{c.paidOrders}</th><th>{c.captured}</th><th>{c.refunded}</th><th>{c.net}</th><th>{c.pickupCollected}</th><th>{c.transferConfirmed}</th><th>{c.codCollected}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {summary.rows.map((row) => (
                      <FinanceLine key={`${row.day}|${row.currency}|${row.environment}`} row={row} locale={locale} c={c} />
                    ))}
                  </tbody>
                  <tfoot>
                    {summary.totals.map((row) => (
                      <FinanceLine key={`total|${row.currency}|${row.environment}`} row={row} locale={locale} c={c} />
                    ))}
                  </tfoot>
                </table>
              </TableFrame>
            )}
          </>
        )}
      </div>
    </WorkspaceFrame>
  );
}

function FinanceLine({ row, locale, c }: { row: FinanceRow; locale: Locale; c: (typeof customersCopy)["en"] }) {
  const m = (value: number) => money(locale, row.currency, value);
  return (
    <tr data-testid={row.day ? `finance-row-${row.day}-${row.currency}` : `finance-total-${row.currency}`}>
      <td data-label={c.day}>{row.day || c.totalRow}</td>
      <td data-label={c.currency}>{row.currency}</td>
      <td data-label={c.environment}>
        <span className={`orders-badge ${isSandbox(row) ? "orders-tone-warning" : "orders-tone-success"}`}>{row.environment}</span>
      </td>
      <td data-label={c.paidOrders}>{row.captured_count}</td>
      <td data-label={c.captured}>{m(row.captured_minor)}</td>
      <td data-label={c.refunded}>{m(row.refunded_minor)}</td>
      <td data-label={c.net}>{m(row.net_minor)}</td>
      {/* OP3: carrier-collected pay-at-pickup money; its own money path, never added to captured or net. */}
      <td data-label={c.pickupCollected}>{row.pickup_collected_count ? `${m(row.pickup_collected_minor)} (${row.pickup_collected_count})` : m(0)}</td>
      {/* storefront-v2 §C: bank-transfer money the merchant confirmed (offline, server order total); its own money path too. */}
      <td data-label={c.transferConfirmed} data-testid="finance-transfer-confirmed">
        {row.bank_transfer_confirmed_count ? `${m(row.bank_transfer_confirmed_minor)} (${row.bank_transfer_confirmed_count})` : m(0)}
      </td>
      {/* home-cod R5: cash-on-delivery money the carrier collected (offline, order total + surcharge); its own money path too. */}
      <td data-label={c.codCollected} data-testid="finance-cod-collected">
        {row.cod_collected_count ? `${m(row.cod_collected_minor)} (${row.cod_collected_count})` : m(0)}
      </td>
    </tr>
  );
}
