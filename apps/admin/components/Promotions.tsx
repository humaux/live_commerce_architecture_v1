// Purpose: Owns merchant promotion creation, editing and pause/resume controls.
// Depends on: react, @live-commerce/i18n, @/lib/model, @/lib/client, @/lib/orders-model, @/lib/customers-client, @/lib/promotions-client, @/lib/promotions-model, @/lib/promotions-copy, ./WorkspaceFrame, ./AdminPageHeader, @live-commerce/ui, @/lib/presentation-copy, ./orders.css, ./order-actions.css, ./customers.css, ./promotions.css
// Used by: apps/admin/app/[locale]/promotions/page.tsx
"use client";

// Discount codes page (/{locale}/promotions): list (code, discount, minimum, Taipei-time window, usage, per-buyer limit, status), create, edit
// and pause/resume. BFF (lib/promotions-client.ts): GET|POST /api/stores/{store}/promotions, POST /api/stores/{store}/promotions/{id} -> Go
// internal/httpapi/promotions.go (pricing:read / pricing:write, migration 0091, contracts/storefront-v2.md §F).
// The server owns every rule, the version and the usage count; each write is followed by a re-read instead of trusting the answer. One
// Idempotency-Key per distinct body, reused only for a byte-identical retry after an unknown outcome. A change never touches orders already
// placed; an in-flight buyer quote of an edited code is refused at checkout (promo_changed) and re-quoted.
import { useRef, useState, type FormEvent } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { currencySign, money } from "@/lib/client";
import { wholeOnly } from "@/lib/orders-model";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { createPromotion, readPromotions, updatePromotion, type PromoWrite } from "@/lib/promotions-client";
import {
  createBody, emptyForm, formFrom, instantToTaipei, toggleBody, updateBody, type Promotion, type PromoForm, type PromoKind,
} from "@/lib/promotions-model";
import { promotionsCopy, type PromotionsCopy } from "@/lib/promotions-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { DateControl, TableFrame } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";
import "./orders.css";
import "./order-actions.css";
import "./customers.css";
import "./promotions.css";

const errorText = (c: PromotionsCopy, code: string) => c.errors[code] ?? c.errors.default;

/** Owns merchant promotion creation, editing and pause/resume controls. User actions submit promotion writes through promotions-client. */
export function Promotions({
  locale, stores, store, initialError, renderKey,
}: {
  locale: Locale; stores: Store[]; store: Store | null; initialError: ReadCode | null; renderKey: string;
}) {
  const c = promotionsCopy[locale];
  const read = useGuardedRead(`${renderKey}|${locale}|${store?.id ?? ""}`, store ? (signal) => readPromotions(store.id, signal) : null, initialError);
  const failure =
    read.status === "signed-out" ? c.signedOut
    : read.status === "forbidden" ? c.forbidden
    : read.status === "not-found" ? (store ? c.notFound : c.noStore)
    : read.status === "unavailable" ? c.unavailable
    : "";
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="promotions">
      <div className="orders-page customers-page" data-testid="promotions-page">
        <AdminPageHeader locale={locale} description={c.subtitle} />
        {(read.status === "loading" || read.status === "hidden") && <p className="orders-message" role="status">{c.loading}</p>}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>{c.retry}</button>
          </div>
        )}
        {read.status === "ready" && read.data && store && (
          <Sections rows={read.data} store={store} boundary={read.boundary} refresh={read.refresh} locale={locale} c={c} />
        )}
      </div>
    </WorkspaceFrame>
  );
}

function Sections({
  rows, store, boundary, refresh, locale, c,
}: {
  rows: Promotion[]; store: Store; boundary: string; refresh: () => Promise<boolean>; locale: Locale; c: PromotionsCopy;
}) {
  const [busy, setBusy] = useState("");
  const [problem, setProblem] = useState("");
  const [notice, setNotice] = useState("");
  const [uncertain, setUncertain] = useState(false);
  const [editing, setEditing] = useState<Promotion | null>(null);
  const [form, setForm] = useState<PromoForm>(emptyForm);
  const pending = useRef<{ key: string; body: string } | null>(null);
  const cash = (minor: number) => money(locale, store.currency, minor);

  // One key per distinct body; a different body gets a fresh key, an identical retry reuses it (idempotent replay on the server).
  const keyFor = (body: string) => {
    if (pending.current?.body !== body) pending.current = { key: `promo-${crypto.randomUUID()}`, body };
    return pending.current.key;
  };
  async function run(label: string, body: string, send: (key: string) => Promise<PromoWrite>, done: string) {
    if (busy) return;
    setBusy(label);
    setProblem("");
    setNotice("");
    const result = await send(keyFor(body));
    setBusy("");
    if (result.ok) {
      pending.current = null;
      setUncertain(false);
      setNotice(done);
      setEditing(null);
      setForm(emptyForm);
    } else if (result.uncertain) {
      setUncertain(true);
      setProblem(c.uncertain);
      return;
    } else {
      pending.current = null;
      setUncertain(false);
      setProblem(errorText(c, result.code));
    }
    await refresh(); // the list, the versions and the usage counts always come from the server
  }
  // D02: NT$ amounts are whole dollars; a typed decimal gets its own sentence instead of the generic "check the form".
  const invalidText = () => (wholeOnly(store.currency) && /[.,]/.test((form.kind === "fixed" ? form.value : "") + form.minSubtotal) ? c.invalidWhole : c.invalid);
  function submit(event: FormEvent) {
    event.preventDefault();
    if (editing) {
      const body = updateBody(form, store.currency, editing.version);
      if (!body) return setProblem(invalidText());
      const text = JSON.stringify(body);
      void run("save", text, (key) => updatePromotion(store.id, editing.id, key, body, boundary), c.saved);
    } else {
      const body = createBody(form, store.currency);
      if (!body) return setProblem(invalidText());
      const text = JSON.stringify(body);
      void run("create", text, (key) => createPromotion(store.id, key, body, boundary), c.created);
    }
  }
  function toggle(row: Promotion) {
    const body = toggleBody(row);
    void run(`toggle:${row.id}`, JSON.stringify({ id: row.id, ...body }), (key) => updatePromotion(store.id, row.id, key, body, boundary), c.toggled);
  }
  const set = <K extends keyof PromoForm>(key: K, value: PromoForm[K]) => setForm((current) => ({ ...current, [key]: value }));
  const when = (row: Promotion) =>
    row.starts_at === null && row.ends_at === null
      ? c.always
      : [row.starts_at && `${c.from} ${instantToTaipei(row.starts_at).replace("T", " ")}`, row.ends_at && `${c.until} ${instantToTaipei(row.ends_at).replace("T", " ")}`].filter(Boolean).join(" ");

  return (
    <>
      <section className="customers-section" aria-label={editing ? c.editTitle : c.createTitle}>
        <h2>{editing ? `${c.editTitle}: ${editing.code}` : c.createTitle}</h2>
        <form className="promotions-form" onSubmit={submit} data-testid="promotion-form">
          <label>
            {c.code}
            <input data-testid="promotion-code" required disabled={editing !== null} autoComplete="off" maxLength={24} value={form.code} onChange={(event) => set("code", event.target.value.toUpperCase())} />
            <small>{c.codeHint}</small>
          </label>
          <label>
            {c.kind}
            <select data-testid="promotion-kind" value={form.kind} onChange={(event) => set("kind", event.target.value as PromoKind)}>
              <option value="percent">{c.kindPercent}</option>
              <option value="fixed">{c.kindFixed}</option>
            </select>
          </label>
          <label>
            {form.kind === "percent" ? c.valuePercent : `${c.valueFixed} (${currencySign(store.currency)})`}
            <input data-testid="promotion-value" required inputMode="decimal" autoComplete="off" value={form.value} onChange={(event) => set("value", event.target.value)} />
          </label>
          <label>
            {`${c.minSubtotal} (${currencySign(store.currency)})`}
            <input data-testid="promotion-min" inputMode="decimal" autoComplete="off" value={form.minSubtotal} onChange={(event) => set("minSubtotal", event.target.value)} />
            <small>{c.minSubtotalHint}</small>
          </label>
          <label>
            {c.startsAt}
            <DateControl emptyLabel={presentationCopy[locale].dateTime} lang={locale} data-testid="promotion-starts" type="datetime-local" value={form.startsAt} onChange={(event) => set("startsAt", event.target.value)} />
          </label>
          <label>
            {c.endsAt}
            <DateControl emptyLabel={presentationCopy[locale].dateTime} lang={locale} data-testid="promotion-ends" type="datetime-local" value={form.endsAt} onChange={(event) => set("endsAt", event.target.value)} />
            <small>{c.windowHint}</small>
          </label>
          <label>
            {c.totalLimit}
            <input data-testid="promotion-total" inputMode="numeric" autoComplete="off" value={form.totalLimit} onChange={(event) => set("totalLimit", event.target.value)} />
          </label>
          <label>
            {c.perBuyerLimit}
            <input data-testid="promotion-buyer" inputMode="numeric" autoComplete="off" value={form.perBuyerLimit} onChange={(event) => set("perBuyerLimit", event.target.value)} />
            <small>{c.limitHint}</small>
          </label>
          <div className="customers-actions">
            <button type="submit" className="primary" data-testid="promotion-submit" disabled={busy !== ""}>{editing ? c.save : c.create}</button>
            {editing && (
              <button type="button" disabled={busy !== ""} onClick={() => { setEditing(null); setForm(emptyForm); setProblem(""); }}>{c.cancel}</button>
            )}
          </div>
        </form>
        {notice && <p className="orders-hint" role="status" data-testid="promotion-notice">{notice}</p>}
        {problem && <p className={uncertain ? "orders-hint" : "orders-bad"} role="alert" data-testid="promotion-problem">{problem}</p>}
      </section>

      <section className="customers-section" aria-label={c.listTitle}>
        <h2>{c.listTitle}</h2>
        {rows.length === 0 ? (
          <p className="orders-empty" data-testid="promotions-empty">{c.empty}</p>
        ) : (
          <TableFrame label={c.title} scrollHint={presentationCopy[locale].scroll}>
            <table className="orders-actions-table" data-testid="promotions-table">
              <thead>
                <tr><th>{c.colCode}</th><th>{c.colDiscount}</th><th>{c.colMinimum}</th><th>{c.colWindow}</th><th>{c.colUsage}</th><th>{c.colPerBuyer}</th><th>{c.colStatus}</th><th>{c.colActions}</th></tr>
              </thead>
              <tbody>
                {rows.map((row) => (
                  <tr key={row.id} data-testid="promotion-row">
                    <td><strong>{row.code}</strong></td>
                    <td>{row.kind === "percent" ? `${row.percent}${c.percentOff}` : cash(row.fixed_minor as number)}</td>
                    <td>{row.min_subtotal_minor === 0 ? c.noMinimum : cash(row.min_subtotal_minor)}</td>
                    <td>{when(row)}</td>
                    <td data-testid="promotion-used">{row.total_limit === null ? String(row.used) : `${row.used} / ${row.total_limit}`}</td>
                    <td>{row.per_buyer_limit === null ? c.noLimit : String(row.per_buyer_limit)}</td>
                    <td><span className={row.status === "active" ? "orders-badge orders-badge-merchant_shipped" : "orders-badge"}>{row.status === "active" ? c.statusActive : c.statusPaused}</span></td>
                    <td>
                      <button type="button" data-testid="promotion-toggle" disabled={busy !== ""} onClick={() => toggle(row)}>{row.status === "active" ? c.pause : c.resume}</button>{" "}
                      <button type="button" data-testid="promotion-edit" disabled={busy !== ""} onClick={() => { setEditing(row); setForm(formFrom(row, store.currency)); setProblem(""); setNotice(""); }}>{c.edit}</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </TableFrame>
        )}
      </section>
    </>
  );
}
