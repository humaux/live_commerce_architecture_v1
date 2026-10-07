// Purpose: Settlements page — read-only platform card-payment settlement statements and line detail.
// Depends on: react, @live-commerce/i18n, @/lib/model, @/lib/client, @/lib/card-payments-client, @/lib/customers-client, @/lib/card-payments-settlements-model, @/lib/card-payments-settlements-copy, @/lib/orders-model, ./WorkspaceFrame, ./AdminPageHeader, @live-commerce/ui, ./orders.css, ./customers.css
// Used by: apps/admin/app/[locale]/settings/settlements/page.tsx
"use client";

// (/{locale}/settings/settlements, billing:manage): weekly statements of platform-collected card payments
// (W4-S2 ledger): captured/refunded/disputes, Stripe + platform fees, carried-in, net payable, payout state
// with the platform's bank-transfer reference; the detail lists every line. Store currency only; no Stripe ids.
import { useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { Badge, TableFrame } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { readSettlementDetail, readSettlements } from "@/lib/card-payments-client";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { payoutState, signedAmountText, statementCells, type SettlementStatement } from "@/lib/card-payments-settlements-model.ts";
import { cardPaymentsSettlementsCopy, type CardPaymentsSettlementsCopy } from "@/lib/card-payments-settlements-copy.ts";
import { displayTime } from "@/lib/orders-model";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import "./orders.css";
import "./customers.css";

const payoutTone = { paid: "success", pending: "warning", none: "neutral" } as const;
const payoutText = (c: CardPaymentsSettlementsCopy, state: "paid" | "pending" | "none") =>
  state === "paid" ? c.paid : state === "pending" ? c.pending : c.noPayout;

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
  const c = cardPaymentsSettlementsCopy[locale];
  const [selected, setSelected] = useState<string | null>(null);
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
      <div className="orders-page customers-page" data-testid="settlements-page">
        <AdminPageHeader locale={locale} description={c.subtitle} />
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
          <List statements={list.data} onOpen={setSelected} locale={locale} c={c} />
        )}
        {selected && (
          <button type="button" data-testid="settlement-back" onClick={() => setSelected(null)}>
            {c.back}
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
      <TableFrame label={c.tableLabel} scrollHint={c.scrollHint}>
        <table>
          <thead>
            <tr>
              <th>{c.period}</th>
              <th>{c.captured}</th>
              <th>{c.refunded}</th>
              <th>{c.disputes}</th>
              <th>{c.stripeFee}</th>
              <th>{c.platformFee}</th>
              <th>{c.carriedIn}</th>
              <th>{c.netPayable}</th>
              <th>{c.status}</th>
              <th>{c.detail}</th>
            </tr>
          </thead>
          <tbody>
            {statements.map((s) => (
              <tr key={s.statement_id} data-testid={`settlement-row-${s.statement_id}`}>
                {/* period, then the six signed contributions (what the store receives), then the server's net; see statementCells */}
                {statementCells(locale, s).map((text, index) => (
                  <td key={index} data-testid={index === 0 ? `settlement-period-${s.statement_id}` : undefined}>{text}</td>
                ))}
                <td>
                  <Badge tone={payoutTone[payoutState(s)]} data-testid={`settlement-status-${s.statement_id}`} data-state={payoutState(s)}>
                    {payoutText(c, payoutState(s))}
                  </Badge>
                </td>
                <td>
                  <button type="button" data-testid={`settlement-open-${s.statement_id}`} onClick={() => onOpen(s.statement_id)}>
                    {c.detail}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </TableFrame>
      <Legend c={c} />
    </>
  );
}

/** The sign legend and the net formula, quoted from the 0150 identity; shown under every statement table and detail. */
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
  // the same six signed contributions and the server's net as the list row (statementCells), so the detail adds up the same way
  const rows = [
    ["captured", c.captured], ["refunded", c.refunded], ["dispute", c.disputes], ["stripe-fee", c.stripeFee],
    ["platform-fee", c.platformFee], ["carried-in", c.carriedIn], ["net", c.netPayable],
  ] as const;
  return (
    <section data-testid="settlement-detail">
      <h2>{c.detail}</h2>
      <dl>
        <div>
          <dt>{c.period}</dt>
          <dd data-testid="settlement-detail-period">{cells[0]}</dd>
        </div>
        {rows.map(([key, label], index) => (
          <div key={key}>
            <dt>{label}</dt>
            <dd data-testid={`settlement-detail-${key}`}>{cells[index + 1]}</dd>
          </div>
        ))}
        <div>
          <dt>{c.status}</dt>
          <dd>
            <Badge tone={payoutTone[payoutState(s)]} data-testid="settlement-detail-status" data-state={payoutState(s)}>
              {payoutText(c, payoutState(s))}
            </Badge>
          </dd>
        </div>
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
      </dl>
      <h3>{c.lines}</h3>
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
      <Legend c={c} />
    </section>
  );
}
