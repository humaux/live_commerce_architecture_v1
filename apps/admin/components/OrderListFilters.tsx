// Purpose: Owns the editable order filter controls and validated apply callback, plus the link to the returns page
//   (the server has no "has returns" order filter — returns-v1 §6; the RMA list lives at /{locale}/returns).
// Depends on: react, @live-commerce/i18n, @/lib/orders-v2, @/lib/orders-v2-copy, @live-commerce/ui, @/lib/presentation-copy, @/lib/returns-copy
// Used by: apps/admin/components/MerchantOrders.tsx
"use client";

// Read-only filters: private search remains memory-only; totals are server projections.
import { useState, type FormEvent, type ReactNode } from "react";
import type { Locale } from "@live-commerce/i18n";
import { deliveries, emptyFilters, modes, validOrderFilters, type OrderFilters, type OrderSession } from "@/lib/orders-v2";
import { ordersV2Copy } from "@/lib/orders-v2-copy";
import { returnsCopy } from "@/lib/returns-copy";
import { DateControl } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";

/** Owns the editable order filter controls and validated apply callback. */
export function OrderListFilters({ locale, filters, sessions, disabled, onApply, children }: {
  locale: Locale; filters: OrderFilters; sessions: OrderSession[]; disabled: boolean;
  onApply: (next: OrderFilters) => void;
  children: ReactNode;
}) {
  const c = ordersV2Copy[locale];
  const [draft, setDraft] = useState(filters);
  const [invalid, setInvalid] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const change = (key: keyof OrderFilters, value: string) => { setDraft(old => ({ ...old, [key]: value })); setInvalid(false); };
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next = { ...draft, q: draft.q.trim() };
    if (!validOrderFilters(next)) { setInvalid(true); return; }
    onApply(next);
  }
  return <form className="orders-v2-filters" data-testid="orders-v2-filters" data-expanded={expanded} onSubmit={submit}>
    <label className="orders-v2-search">{c.search}
      <input type="search" data-testid="orders-search" value={draft.q} maxLength={160} onChange={e => change("q", e.target.value)} aria-describedby="orders-search-hint" />
      <small id="orders-search-hint">{c.hint}</small>
    </label>
    <button type="button" className="orders-v2-filter-toggle" data-testid="orders-more-filters" aria-expanded={expanded} aria-controls="orders-state-field orders-secondary-filters" onClick={() => setExpanded(value => !value)}>{expanded ? c.lessFilters : c.moreFilters}</button>
    {children}
    <div id="orders-secondary-filters" className="orders-v2-secondary" hidden={!expanded}>
    <label>{c.payment}<select data-testid="orders-payment-filter" value={draft.payment_mode} onChange={e => change("payment_mode", e.target.value)}><option value="">{c.all}</option>{modes.map(mode => <option key={mode} value={mode}>{c.modes[mode]}</option>)}</select></label>
    <label>{c.delivery}<select data-testid="orders-delivery-filter" value={draft.delivery} onChange={e => change("delivery", e.target.value)}><option value="">{c.all}</option>{deliveries.map(kind => <option key={kind} value={kind}>{c.deliveries[kind]}</option>)}</select></label>
    <label>{c.session}<select data-testid="orders-session-filter" value={draft.session_id} onChange={e => change("session_id", e.target.value)}><option value="">{c.all}</option>{draft.session_id && !sessions.some(s => s.id === draft.session_id) && <option value={draft.session_id}>{c.selectedSession}</option>}{sessions.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}</select></label>
    <label>{c.from}<DateControl emptyLabel={presentationCopy[locale].date} data-testid="orders-from" type="date" lang={locale} min="2000-01-01" max="2199-12-31" value={draft.from} onChange={e => change("from", e.target.value)} /><small>{c.dateHint}</small></label>
    <label>{c.to}<DateControl emptyLabel={presentationCopy[locale].date} data-testid="orders-to" type="date" lang={locale} min="2000-01-01" max="2199-12-31" value={draft.to} onChange={e => change("to", e.target.value)} /><small>{c.dateHint}</small></label>
    </div>
    <div className="orders-v2-filter-actions"><button type="submit" data-testid="orders-apply" disabled={disabled}>{c.apply}</button><button type="button" data-testid="orders-reset" disabled={disabled} onClick={() => { setDraft(emptyFilters); setInvalid(false); onApply(emptyFilters); }}>{c.reset}</button><a className="orders-v2-returns-link" data-testid="orders-returns-link" href={`/${locale}/returns`}>{returnsCopy[locale].listLink}</a></div>
    {invalid && <p role="alert">{c.invalid}</p>}
  </form>;
}
