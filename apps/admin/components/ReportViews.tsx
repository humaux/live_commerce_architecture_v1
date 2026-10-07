// Purpose: pure report tables and CSS/SVG charts with explicit currency/environment and offline columns.
// Depends on: report model/copy/presentation and the shared currency formatter; no browser or API calls.
// Used by: Reports.tsx; SSR tests render actual view components without a Next build.
// Invariants: I05 no money sums/conversion; staff/session IDs never become primary product copy.
import type { Locale } from "@live-commerce/i18n";
import { money, decimal } from "../../../packages/format/src/index.ts";
import { reportsCopy } from "../lib/reports-copy.ts";
import type { ProductReport, ChannelReport, FunnelReport, ManualReport, ReportMoney } from "../lib/reports-model.ts";
import { sortedProducts, conversion, type ProductSort } from "../lib/reports-presentation.ts";

const count = (locale: string, value: number) => decimal(locale, value, 0);

/** Accessible product rows sorted within currency/payment-mode groups; no financial side effect. */
export function ProductReportTable({ locale, report, sort, ascending, onSort }: {
  locale: Locale; report: ProductReport; sort: ProductSort; ascending: boolean; onSort: (key: ProductSort) => void;
}) {
  const c = reportsCopy[locale];
  const columns: [ProductSort, string][] = [["name", c.product], ["units", c.units], ["captured_minor", c.captured],
    ["refunded_minor", c.refunded], ["net_minor", c.net], ["offline_units", c.offlineUnits], ["offline_minor", c.offline]];
  if (!report.rows.length) return <p role="status">{c.empty}</p>;
  return <>
    <p>{c.sortHint}</p>
    {report.truncated && <p role="status" data-testid="reports-truncated">{c.truncated}</p>}
    <div className="reports-scroll" role="region" aria-label={c.tabs.products} tabIndex={0}>
      <table className="reports-table" data-testid="reports-products-table">
        <caption>{c.tabs.products} · {report.from} – {report.to}</caption>
        <thead><tr>{columns.map(([key, label]) => <th key={key} scope="col" aria-sort={sort === key ? (ascending ? "ascending" : "descending") : "none"}>
          <button type="button" onClick={() => onSort(key)}>{label}{sort === key ? (ascending ? " ↑" : " ↓") : ""}</button>
        </th>)}<th scope="col">{c.currency}</th><th scope="col">{c.environment}</th></tr></thead>
        <tbody>{sortedProducts(report.rows, sort, ascending, locale).map((row) => <tr key={`${row.sku_id}-${row.currency}-${row.environment}`}>
          <th scope="row">{row.name || row.code || c.product}{row.code && <small>{row.code}</small>}</th>
          <td>{count(locale, row.units)}</td><td>{money(locale, row.currency, row.captured_minor)}</td>
          <td>{money(locale, row.currency, row.refunded_minor)}</td><td>{money(locale, row.currency, row.net_minor)}</td>
          <td>{count(locale, row.offline_units)}</td><td>{money(locale, row.currency, row.offline_minor)}</td>
          <td>{row.currency}</td><td>{row.environment === "LIVE" ? c.live : c.test}</td>
        </tr>)}</tbody>
      </table>
    </div>
  </>;
}

function MoneyTable({ locale, currency, rows, label }: { locale: Locale; currency: string; rows: ReportMoney[]; label: string }) {
  const c = reportsCopy[locale];
  return <div className="reports-scroll" role="region" aria-label={label} tabIndex={0}><table className="reports-table reports-money-table">
    <caption>{label} · {currency}</caption>
    <thead><tr><th scope="col">{c.environment}</th><th scope="col">{c.captured}</th><th scope="col">{c.refunded}</th>
      <th scope="col">{c.net}</th><th scope="col">{c.offline}</th></tr></thead>
    <tbody>{rows.map((row) => <tr key={row.environment}>
      <th scope="row">{row.environment === "LIVE" ? c.live : c.test}</th>
      <td>{money(locale, currency, row.captured_minor)}</td><td>{money(locale, currency, row.refunded_minor)}</td>
      <td>{money(locale, currency, row.net_minor)}</td><td>{money(locale, currency, row.offline_minor)}</td>
    </tr>)}</tbody>
  </table></div>;
}

/** Render per-channel order bars with numeric text and separated money tables; no side effect. */
export function ChannelReportChart({ locale, report }: { locale: Locale; report: ChannelReport }) {
  const c = reportsCopy[locale], maximum = Math.max(1, ...report.rows.map((row) => row.orders));
  if (!report.rows.length) return <p role="status">{c.empty}</p>;
  return <div className="reports-channels" data-testid="reports-channels-chart">
    {report.rows.map((row) => <section key={`${row.channel}-${row.currency}`} className="reports-channel">
      <h3>{c.channels[row.channel]} · {row.currency}</h3>
      <p>{c.orders}: <strong>{count(locale, row.orders)}</strong> · {c.cancelled}: {count(locale, row.cancelled_orders)}</p>
      <svg viewBox="0 0 100 8" role="img" aria-label={`${c.channels[row.channel]} ${row.currency}: ${c.orders} ${count(locale, row.orders)}`}>
        <rect width="100" height="8" className="reports-bar-track" />
        <rect width={row.orders / maximum * 100} height="8" className="reports-bar-fill" />
      </svg>
      {row.money.length ? <MoneyTable locale={locale} currency={row.currency} rows={row.money} label={c.channels[row.channel]} /> : <p>{c.empty}</p>}
    </section>)}
  </div>;
}

/** Render backend nested stages and conversion ratios; zero denominators have no invented percentage. */
export function FunnelReportChart({ locale, report }: { locale: Locale; report: FunnelReport }) {
  const c = reportsCopy[locale];
  const steps = [[c.claimed, report.claimed], [c.linkSent, report.link_sent], [c.ordered, report.ordered], [c.paid, report.paid]] as const;
  return <section data-testid="reports-funnel-chart">
    <p>{c.funnelBasis}</p>
    <ol className="reports-funnel" aria-label={c.funnelChart}>{steps.map(([label, value], index) => <li key={label}>
      <div><strong>{label}</strong><span>{count(locale, value)}</span></div>
      <svg viewBox="0 0 100 10" role="img" aria-label={`${label}: ${count(locale, value)}`}>
        <rect width="100" height="10" className="reports-bar-track" />
        <rect x={(100 - (report.claimed ? value / report.claimed * 100 : 0)) / 2}
          width={report.claimed ? value / report.claimed * 100 : 0} height="10" className="reports-bar-fill" />
      </svg>
      {index > 0 && <p>{c.conversion}: {conversion(locale, value, steps[index - 1][1], c.noRatio)}</p>}
    </li>)}</ol>
    <p>{c.totalConversion}: <strong>{conversion(locale, report.paid, report.claimed, c.noRatio)}</strong></p>
    <p>{c.unlinked}: {count(locale, report.ordered_without_link)}</p>
  </section>;
}

/** Render manual buckets with authorized labels or honest fallbacks; never leaks UUIDs into table copy. */
export function ManualReportTable({ locale, report, staff, sessions }: {
  locale: Locale; report: ManualReport; staff: Record<string, string>; sessions: Record<string, string>;
}) {
  const c = reportsCopy[locale];
  if (!report.rows.length) return <p role="status">{c.empty}</p>;
  return <><p>{c.manualBasis}</p><p>{c.staffFallback}</p>
    <div className="reports-scroll" role="region" aria-label={c.tabs["manual-orders"]} tabIndex={0}>
      <table className="reports-table" data-testid="reports-manual-table"><caption>{c.tabs["manual-orders"]} · {report.from} – {report.to}</caption>
        <thead><tr>{[c.creator, c.session, c.currency, c.orders, c.cancelled, c.environment, c.captured, c.refunded, c.net, c.offline]
          .map((label) => <th scope="col" key={label}>{label}</th>)}</tr></thead>
        <tbody>{report.rows.flatMap((row, index) => {
          const creator = row.principal_id ? (staff[row.principal_id] ?? c.unknownCreator) : c.unknownCreator;
          const session = row.session_id ? (sessions[row.session_id] ?? c.unknownSession) : c.noSession;
          return (row.money.length ? row.money : [null]).map((amount, at) => <tr key={`${index}-${at}`}>
            {at === 0 && <><th scope="row" rowSpan={Math.max(1, row.money.length)}>{creator}</th>
              <td rowSpan={Math.max(1, row.money.length)}>{session}</td><td rowSpan={Math.max(1, row.money.length)}>{row.currency}</td>
              <td rowSpan={Math.max(1, row.money.length)}>{count(locale, row.orders)}</td><td rowSpan={Math.max(1, row.money.length)}>{count(locale, row.cancelled_orders)}</td></>}
            <td>{amount ? amount.environment === "LIVE" ? c.live : c.test : "—"}</td>
            {(["captured_minor", "refunded_minor", "net_minor", "offline_minor"] as const).map((key) => <td key={key}>
              {amount ? money(locale, row.currency, amount[key]) : "—"}</td>)}
          </tr>);
        })}</tbody>
      </table>
    </div>
  </>;
}
