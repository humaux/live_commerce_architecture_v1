// Purpose: Settlements page — read-only platform card-payment settlement statements and line detail.
// Depends on: react, @live-commerce/i18n, @/lib/model, @/lib/client, @/lib/card-payments-client, @/lib/customers-client, @/lib/card-payments-settlements-model, @/lib/card-payments-settlements-copy, @/lib/orders-model, ./WorkspaceFrame, ./AdminPageHeader, @live-commerce/ui, ./orders.css, ./customers.css, ./finance/money.css
// Used by: apps/admin/app/[locale]/settings/settlements/page.tsx
"use client";

// (/{locale}/settings/settlements, billing:manage): weekly statements of platform-collected card payments
// (W4-S2 ledger): captured/refunded/disputes, Stripe + platform fees, carried-in, net payable, payout state
// with the platform's bank-transfer reference; the detail lists every line. Store currency only; no Stripe ids.
import { useEffect, useState } from "react";
import { moneyCopy, type Locale } from "@live-commerce/i18n";
import { Badge, TableFrame } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { readSettlementDetail, readSettlements } from "@/lib/card-payments-client";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { payoutState, signedAmountText, statementCells, type PayoutState, type SettlementStatement } from "@/lib/card-payments-settlements-model.ts";
import { cardPaymentsSettlementsCopy, type CardPaymentsSettlementsCopy } from "@/lib/card-payments-settlements-copy.ts";
import { displayTime } from "@/lib/orders-model";
import { WorkspaceFrame } from "./WorkspaceFrame";
import "./orders.css";
import "./customers.css";
import "./finance/money.css";

const payoutTone = { returned: "warning", paid: "success", pending: "warning", none: "neutral" } as const;
const payoutText = (c: CardPaymentsSettlementsCopy, state: PayoutState) =>
  state === "returned" ? c.returned : state === "paid" ? c.paid : state === "pending" ? c.pending : c.noPayout;

/** The return flag takes precedence over the historical paid flag; the explanation wraps at phone widths. */
function PayoutStatus({ statement, c, testId }: { statement: SettlementStatement; c: CardPaymentsSettlementsCopy; testId: string }) {
  const state = payoutState(statement);
  return (
    <div data-testid={state === "returned" ? `settlement-returned-message-${statement.statement_id}` : undefined} style={{ whiteSpace: "normal" }}>
      <Badge tone={payoutTone[state]} data-testid={testId} data-state={state}>{payoutText(c, state)}</Badge>
      {state === "returned" && <span> · {c.returnedCarry}</span>}
    </div>
  );
}

/** Owns the statements list and the per-statement line detail; everything is a read (billing:manage). */
export function Settlements({
  locale,
  stores: _stores,
  store,
  initialError,
  renderKey,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  initialError: ReadCode | null;
  renderKey: string;
}) {
  const c = cardPaymentsSettlementsCopy[locale], p = moneyCopy[locale];
  const [selected, setSelected] = useState<string | null>(null);
  useEffect(() => {
    const restore = () => { const id = new URL(window.location.href).searchParams.get("statement"); setSelected(id && /^[0-9a-f-]{36}$/.test(id) ? id : null); };
    restore(); window.addEventListener("popstate",restore); return () => window.removeEventListener("popstate",restore);
  }, []);
  function open(id: string | null) {
    setSelected(id); const url = new URL(window.location.href);
    if (id) url.searchParams.set("statement",id); else url.searchParams.delete("statement");
    window.history.replaceState(window.history.state,"",url);
  }
  const list = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}`,
    store ? (signal) => readSettlements(store.id, signal) : null,
    initialError,
  );
  const detail = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}|${selected ?? ""}`,
    store && selected ? (signal) => readSettlementDetail(store.id, selected, signal) : null,
    null,
  );
  const failure =
    list.status === "signed-out" ? c.signedOut
    : list.status === "forbidden" ? c.forbidden
    : list.status === "not-found" ? c.notFound
    : list.status === "unavailable" ? c.unavailable
    : "";
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.title} active="settlements">
      <div className="money-page" data-testid="settlements-page">
        <h1>{selected ? detail.data?.payout_returned ? p.returned : (detail.data?.returned_in_minor ?? 0) > 0 ? p.carriedReturn : p.statement : p.statements}</h1>
        {(list.status === "loading" || list.status === "hidden") && (
          <p className="orders-message" role="status">{c.loading}</p>
        )}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={list.reload}>{c.retry}</button>
          </div>
        )}
        {list.status === "ready" && list.data && store && !selected && (
          <List statements={list.data} onOpen={open} locale={locale} c={c} />
        )}
        {selected && (
          <button type="button" data-testid="settlement-back" onClick={() => open(null)}>
            {p.backStatements}
          </button>
        )}
        {selected && (detail.status === "loading" || detail.status === "hidden") && (
          <p className="orders-message" role="status">{c.loading}</p>
        )}
        {selected && detail.status !== "ready" && detail.status !== "loading" && detail.status !== "hidden" && (
          <div className="orders-message" role="status">
            <p>{c.unavailable}</p>
            <button type="button" onClick={detail.reload}>{c.retry}</button>
          </div>
        )}
        {selected && detail.status === "ready" && detail.data && (
          <Detail statement={detail.data} locale={locale} c={c} />
        )}
      </div>
    </WorkspaceFrame>
  );
}

function List({
  statements,
  onOpen,
  locale,
  c,
}: {
  statements: SettlementStatement[];
  onOpen: (id: string) => void;
  locale: Locale;
  c: CardPaymentsSettlementsCopy;
}) {
  if (statements.length === 0)
    return (
      <div className="orders-message" data-testid="settlements-empty">
        <p>{c.empty}</p>
        <p>{c.emptyHint}</p>
      </div>
    );
  return (
    <>
      <section className="money-card"><h2>{moneyCopy[locale].cardBasis}</h2><p>{moneyCopy[locale].settlementNote}</p></section>
      <section className="money-card"><TableFrame label={c.tableLabel} scrollHint={c.scrollHint}>
        <table className="money-statement-table"><thead><tr><th>{moneyCopy[locale].statement} / {moneyCopy[locale].currency}</th><th>{moneyCopy[locale].settlementPeriod}</th><th>{c.status}</th><th>{c.netPayable}</th><th>{moneyCopy[locale].openStatement}</th></tr></thead><tbody>{statements.map(s => <tr key={s.statement_id} data-testid={`settlement-row-${s.statement_id}`}><td>{s.currency}</td><td data-testid={`settlement-period-${s.statement_id}`}>{statementCells(locale,s)[0]}</td><td><PayoutStatus statement={s} c={c} testId={`settlement-status-${s.statement_id}`} /></td><td>{statementCells(locale,s)[8]}</td><td><button type="button" data-testid={`settlement-open-${s.statement_id}`} onClick={() => onOpen(s.statement_id)}>{moneyCopy[locale].openStatement}</button></td></tr>)}</tbody></table>
      </TableFrame></section>
      <Legend c={c} />
    </>
  );
}

/** The sign legend and the net formula, including SA-0 returned funds; shown under every statement table and detail. */
function Legend({ c }: { c: CardPaymentsSettlementsCopy }) {
  return (
    <div data-testid="settlements-legend">
      <p>{c.legend}</p>
      <p data-testid="settlements-formula">{c.formula}</p>
    </div>
  );
}

function Detail({ statement: s, locale, c }: { statement: SettlementStatement; locale: Locale; c: CardPaymentsSettlementsCopy }) {
  const cells = statementCells(locale, s);
  const p = moneyCopy[locale];
  // The same seven signed contributions and the server's net as the list row, including carried return money exactly once.
  const rows = [
    ["captured", p.captured], ["refunded", p.refunded], ["dispute", c.disputes], ["stripe-fee", c.stripeFee],
    ["platform-fee", p.platformFee], ["carried-in", c.carriedIn], ["returned-in", p.returnContribution], ["net", c.netPayable],
  ] as const;
  return (
    <section data-testid="settlement-detail">
      {s.payout_returned && <div className="money-card"><h2>{p.returnedStatus}</h2><p>{s.payout_ref} · {s.payout_minor === null ? "—" : signedAmountText(locale,s.currency,s.payout_minor)}</p></div>}
      <section className="money-card"><p>{s.currency}</p><h2>{p.composition}</h2>
      <dl>
        <div>
          <dt>{c.period}</dt>
          <dd data-testid="settlement-detail-period">{cells[0]}</dd>
        </div>
        {rows.map(([key, label], index) => (
          <div key={key}>
            <dt>{label}</dt>
            <dd data-testid={`settlement-detail-${key}`}><span data-testid={key === "returned-in" ? "settlement-detail-returned_in" : key === "carried-in" ? "settlement-detail-carried_in" : undefined}>{cells[index + 1]}</span></dd>
          </div>
        ))}
        <div>
          <dt>{c.status}</dt>
          <dd>
            <PayoutStatus statement={s} c={c} testId="settlement-detail-status" />
          </dd>
        </div>
        <div><dt>{p.closedAt}</dt><dd>{displayTime(locale,s.closed_at)}</dd></div>
        {s.payout_minor !== null && <div><dt>{p.payoutAmount}</dt><dd>{signedAmountText(locale,s.currency,s.payout_minor)}</dd></div>}
        {s.payout_returned && <div><dt>{p.returnedAmount}</dt><dd>{s.payout_minor === null ? "—" : signedAmountText(locale,s.currency,s.payout_minor)}</dd></div>}
        {s.payout_ref && (
          <div>
            <dt>{c.payoutRef}</dt>
            <dd data-testid="settlement-payout-ref">{s.payout_ref}</dd>
          </div>
        )}
        {s.paid_at && (
          <div>
            <dt>{c.paidAt}</dt>
            <dd data-testid="settlement-paid-at">{displayTime(locale, s.paid_at)}</dd>
          </div>
        )}
        {s.payout_returned_at && (
          <div>
            <dt>{c.returnedAt}</dt>
            <dd data-testid="settlement-returned-at">{displayTime(locale, s.payout_returned_at)}</dd>
          </div>
        )}
      </dl>
      <p className="money-note">{p.settlementNote}</p></section>
      <section className="money-card"><h3>{c.lines}</h3>
      <TableFrame label={c.linesLabel} scrollHint={c.scrollHint}>
        <table>
          <thead>
            <tr>
              <th>{c.order}</th>
              <th>{c.kind}</th>
              <th>{c.amount}</th>
              <th>{c.fee}</th>
              <th>{c.date}</th>
            </tr>
          </thead>
          <tbody>
            {(s.lines ?? []).map((line, index) => (
              <tr key={`${line.order_number}-${index}`} data-testid={`settlement-line-${line.order_number}`}>
                <td>{line.order_number}</td>
                <td>{c[`kind${line.kind}`]}</td>
                <td>{signedAmountText(locale, s.currency, line.store_minor)}</td>
                <td>{signedAmountText(locale, s.currency, line.fee_store_minor)}</td>
                <td>{line.txn_date}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </TableFrame>
      </section>
      <Legend c={c} />
    </section>
  );
}
