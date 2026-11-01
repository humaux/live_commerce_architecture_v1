// Purpose: Signed inventory page states and quantity/reason review; no writes originate here.
// Depends on: React, data-ops namespace and server-owned LedgerRow balances.
// Used by: Ledger, which owns journal durability, scope and adjustment submission.
// Invariants: forecasts are display-only; submission uses server version, delta and reason; unknown requests are immutable.
"use client";
import { useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { LedgerRow, WorkspaceData } from "../../lib/model";
import { dataOpsCopy } from "../../../../packages/i18n/src/data-ops";

export type InventoryState = "list" | "search" | "edit" | "review" | "result" | "conflict" | "unknown" | "discard";
/** Render signed content using the caller's scoped snapshots and explicit state transitions. */
export function InventoryView({ locale, initial, state, setState, selected, select, delta, setDelta, reason, setReason, busy, locked, query, setQuery, navigate, submit, retry, reload, error, children }: {
  locale: Locale; initial: WorkspaceData; state: InventoryState; setState: (value: InventoryState) => void;
  selected?: LedgerRow; select: (id: string) => void; delta: string; setDelta: (value: string) => void;
  reason: string; setReason: (value: string) => void; busy: boolean; locked: boolean;
  query: string; setQuery: (value: string) => void; navigate: (values: Record<string, string>) => void;
  submit: () => void; retry: () => void; reload: () => void; error: string; children?: React.ReactNode;
}) {
  const c = dataOpsCopy(locale);
  const [filter, setFilter] = useState("all");
  const title = { list: c.inventory, search: c.inventorySearch, edit: c.adjust, review: c.reviewTitle, result: c.adjusted, conflict: c.conflict, unknown: c.unknown, discard: c.discardTitle }[state];
  const draftLocked = locked || selected?.status !== "active";
  const quantity = Number(delta), valid = Number.isSafeInteger(quantity) && quantity !== 0 && !!reason.trim();
  const warehouse = initial.warehouses.find(item => item.id === (selected?.warehouse_id ?? initial.warehouseID));
  const balances = selected ? [[c.onHand, selected.on_hand], [c.reserved, selected.reserved], [c.allocated, selected.allocated], [c.unavailable, selected.unavailable], [c.available, selected.available]] as const : [];
  // These are drafts, never persisted stock or a client-derived command total. Go checks the version and stock invariants.
  const forecast = selected && Number.isSafeInteger(quantity) ? <>{state === "edit" && <h2>{c.beforeAfter}</h2>}{[[c.onHand, selected.on_hand], [c.available, selected.available]].map(([label, before]) => <div className="ops-balance" key={label}><span>{label}</span><strong>{before}</strong><span>→</span><strong>{Number(before) + quantity}</strong></div>)}</> : null;
  const choose = (row: LedgerRow) => { select(row.sku_id); setDelta("3"); setReason(""); setState("edit"); };
  return <div className="data-ops" data-testid="inventory-page">
    <header><h1>{title}</h1>{state === "list" && <a className="ops-link" href={`/${locale}/products${initial.storeID ? `?store=${initial.storeID}` : ""}`}>{c.backProducts}</a>}</header>
    {state === "list" ? <>
      {warehouse && <p className="ops-note">{warehouse.name}</p>}
      <div className="ops-inventory-tools"><div className="ops-actions">{[["all", c.all], ["available", c.availableOnly], ["empty", c.soldOut]].map(([value, label]) => <button key={value} aria-pressed={filter === value} disabled={locked || filter === value} onClick={() => setFilter(value)}>{label}</button>)}</div>
        <button disabled={locked} onClick={() => setState("search")}>{c.searchSKU}</button>
      </div>
      <h2>{c.allStock}</h2><div className="ops-card"><div className="table-scroll ops-stock-scroll"><table className="ops-stock-table"><thead><tr>{[c.skuProduct, c.onHand, c.reserved, c.allocated, c.unavailable, c.available, c.action].map(label => <th key={label}>{label}</th>)}</tr></thead><tbody>
        {initial.rows.items.filter(row => filter === "all" || (filter === "available" ? row.available > 0 : row.available === 0)).map(row => <tr key={row.sku_id}>
          <td>{row.code} · <button className="product-name" disabled={locked} onClick={() => choose(row)}>{row.product_name}</button></td>{[row.on_hand, row.reserved, row.allocated, row.unavailable, row.available].map((n, i) => <td key={i} data-label={[c.onHand, c.reserved, c.allocated, c.unavailable, c.available][i]} className={i === 4 ? "available-value" : undefined}>{n}</td>)}
          <td><button disabled={locked || row.status !== "active"} onClick={() => choose(row)}>{c.adjust}</button></td>
        </tr>)}
      </tbody></table></div>{children}</div>
      <p className="ops-note">{c.stockEquation}</p>
      {initial.rows.next_cursor && <button disabled={locked} onClick={() => navigate({ cursor: initial.rows.next_cursor })}>{c.allStock}</button>}
    </> : state === "search" ? <section className="ops-card"><form onSubmit={event => { event.preventDefault(); navigate({ q: query }); setState("list"); }}>
      <label>{c.skuOrName}<input type="text" value={query} onChange={event => setQuery(event.target.value)} maxLength={120} /></label>
      <div className="ops-actions"><button className="primary" type="submit">{c.search}</button><button type="button" onClick={() => setState("list")}>{c.cancel}</button></div>
    </form></section> : <>
      {selected && <><h2>{selected.code} · {selected.product_name}</h2><p className="ops-note">{warehouse?.name}</p></>}
      {state === "edit" && selected && <div className="ops-columns"><section className="ops-card">
        <h2>{c.quantity}</h2><p className="ops-note">{c.quantityHelp}</p>
        <div className="ops-actions"><button disabled={draftLocked} onClick={() => setDelta(String((Number(delta) || 0) - 1))}>−1</button><input type="number" aria-label={c.quantity} value={delta} onChange={event => setDelta(event.target.value)} step="1" disabled={draftLocked} /><button disabled={draftLocked} onClick={() => setDelta(String((Number(delta) || 0) + 1))}>+1</button></div>
        <div className="ops-actions"><button disabled={draftLocked} onClick={() => setDelta("3")}>{c.increase3}</button><button disabled={draftLocked} onClick={() => setDelta("-3")}>{c.decrease3}</button></div>
        {forecast}<h2>{c.reasonRequired}</h2>{!reason && <p>{c.notSelected}</p>}
        <div className="ops-actions">{[c.restock, c.recount].map(value => <button key={value} disabled={draftLocked} aria-pressed={reason === value} onClick={() => setReason(value)}>{value}</button>)}</div>

      </section><aside className="ops-card"><h2>{c.currentStock}</h2><dl className="ops-stats">{balances.map(([label,n]) => <div key={label}><dt>{label}</dt><dd data-testid={label === c.available ? "inventory-current-available" : undefined}>{n}</dd></div>)}</dl><p className="ops-note">{c.latestHelp}</p></aside></div>}
      {state === "edit" && children}
      {state === "edit" && selected && <div className="ops-footer"><button className="primary" disabled={draftLocked || !valid} onClick={() => setState("review")}>{c.reviewAdjust}</button><button disabled={locked} onClick={() => setState("discard")}>{c.cancel}</button></div>}
      {(state === "review" || state === "result") && <section className="ops-card">{forecast}
        {state === "review" ? <><dl className="ops-stats"><div><dt>{c.delta}</dt><dd>{quantity > 0 ? "+" : ""}{delta}</dd></div><div><dt>{c.reason}</dt><dd>{reason}</dd></div></dl><p>{c.reviewHelp}</p></> : <><p>{c.stockUpdatedHelp}</p><button onClick={() => { select(""); setState("list"); }}>{c.backStock}</button></>}
      </section>}
      {state === "review" && <div className="ops-footer"><button className="primary" disabled={busy} onClick={submit}>{c.confirmAdjust}</button><button disabled={busy} onClick={() => setState("edit")}>{c.backEdit}</button></div>}
      {state === "conflict" && <section className="ops-card"><p>{c.conflictHelp}</p><button disabled={busy} onClick={reload}>{c.reloadReview}</button></section>}
      {state === "unknown" && <section className="ops-card"><p>{c.unknownHelp}</p><button className="primary" disabled={busy} onClick={retry}>{c.retryAdjust}</button></section>}
      {state === "discard" && <section className="ops-card"><p>{c.discardHelp}</p><p>{c.continueHelp}</p><div className="ops-actions"><button onClick={() => { setDelta("0"); setReason(""); select(""); setState("list"); }}>{c.discard}</button><button className="primary" onClick={() => setState("edit")}>{c.continueEdit}</button></div></section>}
    </>}
    {error && !["conflict", "unknown"].includes(state) && <p className="message" role="alert">{error}</p>}
  </div>;
}
