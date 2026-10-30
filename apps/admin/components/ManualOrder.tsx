// Purpose: Owns the merchant manual-order entry workflow.
// Depends on: @live-commerce/format (Taipei store timestamps), react, next/link, @live-commerce/i18n, @live-commerce/ui, @/lib/model, @/lib/client, @/lib/customers-client, @/lib/catalog-v2-client, @/lib/catalog-v2-model, @/lib/settings-client, @/lib/merchant-tools-client, @/lib/merchant-tools-model, @/lib/merchant-tools-copy, @/lib/cod-copy, ./WorkspaceFrame, ./AdminPageHeader, ./CvsLookup, ./order-ops.css, ./OperationalForms.module.css, ./orders.css, ./customers.css, ./merchant-tools.css
// Used by: apps/admin/app/[locale]/orders/new/page.tsx
// Invariants: catalog prices and server quotes remain authoritative; draft/picker actions never reserve or place orders.
"use client";

// Create an order from the admin (/{locale}/orders/new): the merchant picks SKUs and quantities, types the customer and the delivery,
// chooses bank transfer, pay at pickup or (home delivery) cash on delivery (never card) and gets the order plus a buyer link to paste into LINE or Messenger.
// BFF /api/stores/{store}/tools/{orders/manual/options, orders/manual} -> Go internal/httpapi/merchanttools.go -> internal/merchanttools
// (contract storefront-v2 G3). Products and variants come from the existing catalog reads (BFF catalog-products, products/{id}).
// There is NO price field anywhere in this form or in the request: the server quotes from the catalog (I05), so the displayed unit prices are
// information only. The submit key is kept for a retry of the SAME body (an uncertain answer replays, never double-reserves) and replaced as
// soon as the form changes. The session fence is lib/merchant-tools-client.
import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { orderOpsCopy, localeNames, type Locale } from "@live-commerce/i18n";
import { displayTime } from "@live-commerce/format";
import { Badge, Field, FormRow } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { readProduct, readProducts } from "@/lib/catalog-v2-client";
import type { ProductDetail } from "@/lib/catalog-v2-model";
import { sessionBoundary } from "@/lib/settings-client";
import { placeManualOrder, readManualOptions, regenerateManualLink } from "@/lib/merchant-tools-client";
import { draftProblem, manualBody, type ManualDraft, type ManualOption, type ManualPaymentMode, type ManualResult } from "@/lib/merchant-tools-model";
import { toolsCopy } from "@/lib/merchant-tools-copy";
import { codCopy } from "@/lib/cod-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { CvsLookup } from "./CvsLookup";
import "./order-ops.css";
import s from "./OperationalForms.module.css";
import "./orders.css";
import "./customers.css";
import "./merchant-tools.css";

type Line = { sku_id: string; quantity: number; label: string; code: string; price: string };
const blankHome = { region: "", city: "", postal_code: "", line1: "", line2: "" };
const blankCVS = { store_code: "", store_name: "", store_address: "" };

/** Owns the merchant manual-order entry workflow. User actions submit manual-order commands through merchant-tools-client. */
export function ManualOrder({
  locale, stores, store, initialError, renderKey,
}: {
  locale: Locale; stores: Store[]; store: Store | null; initialError: ReadCode | null; renderKey: string;
}) {
  const c = toolsCopy[locale].manual;
  const signed = orderOpsCopy[locale];
  const options = useGuardedRead(`${renderKey}|${locale}|${store?.id ?? ""}`, store ? (signal) => readManualOptions(store.id, signal) : null, initialError);
  const [boundary, setBoundary] = useState("");
  useEffect(() => {
    let live = true;
    sessionBoundary().then((value) => live && setBoundary(value), () => live && setBoundary(""));
    return () => { live = false; };
  }, [renderKey]);

  const [lines, setLines] = useState<Line[]>([]);
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [optionKey, setOptionKey] = useState("");
  const [mode, setMode] = useState<ManualPaymentMode | "">("");
  const [home, setHome] = useState(blankHome);
  const [cvs, setCVS] = useState(blankCVS);
  const [buyerLocale, setBuyerLocale] = useState<Locale>("zh-TW");
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState("");
  const [uncertain, setUncertain] = useState(false);
  const [placed, setPlaced] = useState<ManualResult | null>(null);
  const [codDue, setCodDue] = useState<number | null>(null); // total + fee the carrier collects (COD orders only), from the options the order was placed with
  const [copied, setCopied] = useState(false);
  const [regenBusy, setRegenBusy] = useState(false);
  const [regenFailure, setRegenFailure] = useState("");
  const attempt = useRef<{ body: string; key: string } | null>(null);
  const regen = useRef<{ body: string; key: string } | null>(null);

  const [reviewing, setReviewing] = useState(false);
  const [choosingCVS, setChoosingCVS] = useState(false);
  const available: ManualOption[] = options.data ?? [];
  const option = available.find((item) => item.option_key === optionKey) ?? null;
  const optionName = (o: ManualOption) =>
    `${locale === "zh-CN" ? o.name_hans : locale === "zh-TW" ? o.name_hant : o.name_en || o.name_hant} · ${c.kinds[o.delivery_kind] ?? o.delivery_kind}`;
  const mapOnly = !!option && option.delivery_kind !== "home" && option.pickup_selection !== "buyer_entered";
  const draft: ManualDraft = { lines, name, phone, email, option, mode, home, cvs, locale: buyerLocale };
  const problem = draftProblem(draft);

  function selectOption(key: string) {
    setOptionKey(key); setCVS(blankCVS); setReviewing(false);
    const next = available.find((item) => item.option_key === key);
    setMode(next && next.payment_modes.length === 1 ? next.payment_modes[0] : "");
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!store || problem || mapOnly || busy) return;
    if (!reviewing) { setReviewing(true); return; }
    const body = manualBody(draft);
    const text = JSON.stringify(body);
    // A new key only when the request differs; the same body after an uncertain answer re-sends the same key (replay).
    if (!attempt.current || attempt.current.body !== text) attempt.current = { body: text, key: crypto.randomUUID() };
    setBusy(true);
    setFailure("");
    const outcome = await placeManualOrder(store.id, attempt.current.key, body, boundary);
    setBusy(false);
    if (outcome.ok) {
      setCodDue(outcome.value.payment_mode === "cash_on_delivery" ? outcome.value.total_minor + (option?.cod_surcharge_minor ?? 0) : null);
      setPlaced(outcome.value);
      return;
    }
    setUncertain(outcome.uncertain);
    setFailure(c.errors[outcome.code] ?? c.errors.default);
    if (!outcome.uncertain) attempt.current = null; // a definite refusal: the next send is a new attempt
  }
  function reset() {
    setReviewing(false); setChoosingCVS(false);
    setPlaced(null); setCodDue(null); setLines([]); setName(""); setPhone(""); setEmail(""); setHome(blankHome); setCVS(blankCVS);
    setOptionKey(""); setMode(""); setFailure(""); setUncertain(false); setCopied(false); attempt.current = null;
    setRegenFailure(""); regen.current = null;
  }
  async function regenerate() {
    if (!store || !placed || regenBusy || placed.buyer_link === null) return;
    const body = { order_id: placed.order_id, locale: buyerLocale };
    const text = JSON.stringify(body);
    // A new key only when the request differs; the same body after an uncertain answer re-sends the same key (replay, same link).
    if (!regen.current || regen.current.body !== text) regen.current = { body: text, key: crypto.randomUUID() };
    setRegenBusy(true);
    setRegenFailure("");
    const outcome = await regenerateManualLink(store.id, regen.current.key, body, boundary);
    setRegenBusy(false);
    if (outcome.ok) {
      setPlaced({ ...placed, buyer_link: outcome.value.buyer_link });
      setCopied(false);
      regen.current = null;
      return;
    }
    setRegenFailure(c.errors[outcome.code] ?? c.errors.default);
    if (!outcome.uncertain) regen.current = null; // a definite refusal: the next send is a new attempt
  }
  const listFailure =
    options.status === "signed-out" ? c.signedOut : options.status === "forbidden" ? c.forbidden
    : options.status === "not-found" ? (store ? c.unavailable : c.noStore) : options.status === "unavailable" ? c.notConfigured : "";
  const storeQuery = store ? `?store=${store.id}` : "";
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="orders">
      <div className={`orders-page customers-page ops-task ${s.page}`} data-testid="manual-order-page">
        <AdminPageHeader locale={locale} title={signed.manual} description={placed ? c.created : signed.quote} />
        {stores.length > 1 && (
          <div className="orders-controls">
            <label>
              {c.store}
              <select data-testid="store-selector" value={store?.id ?? ""} onChange={(event) => window.location.assign(`/${locale}/orders/new?store=${event.target.value}`)}>
                {stores.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
              </select>
            </label>
          </div>
        )}
        {(options.status === "loading" || options.status === "hidden") && !placed && <p className="orders-message" role="status">{c.loading}</p>}
        {listFailure && <p className="orders-message" role="status">{listFailure}</p>}
        {placed && store && (
          <section className="mt-card" data-testid="manual-order-result" aria-label={c.created}>
            <h2>{c.created}</h2>
            <p role="status"><Badge tone={placed.commercial_state === "CONFIRMED" ? "success" : "warning"}>{placed.commercial_state === "AWAITING_TRANSFER" ? c.waiting : placed.commercial_state === "AWAITING_COLLECTION" ? c.waitingCollection : c.confirmed}</Badge></p>
            <dl className="mt-stats">
              <div><dt>{c.orderId}</dt><dd style={{ fontSize: 14 }}>{placed.order_id.slice(0, 8)}</dd></div>
              <div><dt>{c.total}</dt><dd>{money(locale, placed.currency, placed.total_minor)}</dd></div>
              {codDue !== null && <div data-testid="manual-order-cod-due"><dt>{c.codDue}</dt><dd>{money(locale, placed.currency, codDue)}</dd></div>}
              {placed.payment_mode === "bank_transfer" && <div><dt>{c.expires}</dt><dd style={{ fontSize: 14 }}>{displayTime(locale, placed.expires_at)}</dd></div>}
            </dl>
            <ul>{lines.map((line) => <li key={line.sku_id}>{line.label} · {line.quantity}</li>)}</ul>
            <dl>
              <div><dt>{signed.name}</dt><dd>{name}</dd></div><div><dt>{signed.phone}</dt><dd>{phone}</dd></div>
              <div><dt>{c.delivery}</dt><dd>{option ? optionName(option) : c.choose}</dd></div>
              <div><dt>{option?.delivery_kind === "home" ? c.line1 : signed.pickup}</dt><dd>{option?.delivery_kind === "home" ?
                [home.region, home.city, home.postal_code, home.line1, home.line2].filter(Boolean).join(" ") : `${cvs.store_name} · ${cvs.store_code} · ${cvs.store_address}`}</dd></div>
              <div><dt>{c.paymentTitle}</dt><dd>{placed.payment_mode === "bank_transfer" ? c.bank : placed.payment_mode === "cash_on_delivery" ? c.cod : c.pickup}</dd></div>
            </dl>
            <h3>{c.linkTitle}</h3>
            {placed.buyer_link ? (
              <>
                <p className="mt-note">{c.linkText}</p>
                <p className="mt-link" data-testid="manual-order-link">{placed.buyer_link}</p>
                <div className="mt-actions" style={{ marginTop: 10 }}>
                  <button type="button" onClick={() => navigator.clipboard.writeText(placed.buyer_link!).then(() => setCopied(true), () => setCopied(false))}>
                    {copied ? c.copied : c.copy}
                  </button>
                  <button type="button" data-testid="manual-order-regenerate" disabled={regenBusy} onClick={() => void regenerate()}>
                    {regenBusy ? c.regenerating : c.regenerate}
                  </button>
                </div>
                {regenFailure && <p className="mt-warn" role="alert" data-testid="manual-order-regen-error">{regenFailure}</p>}
              </>
            ) : <p className="mt-warn">{c.noLink}</p>}
            <div className="mt-actions" style={{ marginTop: 16 }}>
              <button type="button" className="primary" onClick={reset} data-testid="manual-order-another">{c.another}</button>
              <Link href={`/${locale}/orders${storeQuery}`}>{c.viewOrders}</Link>
            </div>
          </section>
        )}
        {/* The form stays mounted while the delivery choices (re)load: a guarded re-read must never wipe what the merchant typed. */}
        {!placed && store && !listFailure && (
          <form className="mt-form" onSubmit={(event) => void submit(event)} data-testid="manual-order-form" noValidate>
            {reviewing ? <section className="ops-review" aria-label={signed.review}>
              <h2>{signed.review}</h2>
              <ul>{lines.map((line) => <li key={line.sku_id}>{line.label} · {line.quantity} · {line.price}</li>)}</ul>
              <dl>
                <div><dt>{signed.name}</dt><dd>{name}</dd></div><div><dt>{signed.phone}</dt><dd>{phone}</dd></div>
                <div><dt>{c.delivery}</dt><dd>{option ? optionName(option) : c.choose}</dd></div>
                {option?.delivery_kind === "home" ? <div><dt>{c.line1}</dt><dd>{[home.region, home.city, home.postal_code, home.line1, home.line2].filter(Boolean).join(" ")}</dd></div> :
                  <div><dt>{signed.pickup}</dt><dd>{cvs.store_name} · {cvs.store_code}<br />{cvs.store_address}</dd></div>}
                <div><dt>{c.paymentTitle}</dt><dd>{mode === "bank_transfer" ? c.bank : mode === "cash_on_delivery" ? c.cod : c.pickup}</dd></div>
              </dl>
              <p>{signed.quote}</p>
              {mode === "cash_on_delivery" && option?.cod_max_minor !== undefined && option.cod_carrier && <p>{c.codNote(
                money(locale, option.currency, option.cod_surcharge_minor ?? 0), money(locale, option.currency, option.cod_max_minor),
                option.cod_carrier === "black_cat" ? codCopy[locale].carrierBlackCat : codCopy[locale].carrierHsinchu)}</p>}
              <button type="button" onClick={() => setReviewing(false)}>{signed.backEdit}</button>
            </section> : <>
            <ItemPicker locale={locale} store={store} lines={lines} setLines={setLines} />
            <section className="mt-card">
              <h2>{signed.recipient}</h2>
              <FormRow>
                <Field id="mo-name" label={signed.name}><input id="mo-name" data-testid="mo-name" value={name} maxLength={120} autoComplete="off" onChange={(e) => setName(e.target.value)} /></Field>
                <Field id="mo-phone" label={signed.phone} width="short"><input id="mo-phone" data-testid="mo-phone" value={phone} inputMode="tel" maxLength={32} autoComplete="off" onChange={(e) => setPhone(e.target.value)} /></Field>
                <details><summary>{signed.advanced}</summary><Field id="mo-email" label={c.email}><input id="mo-email" data-testid="mo-email" value={email} type="email" maxLength={254} autoComplete="off" onChange={(e) => setEmail(e.target.value)} /></Field></details>
              </FormRow>

              <FormRow><Field id="mo-option" label={c.delivery} width="long">
                <select id="mo-option" data-testid="mo-option" value={optionKey} disabled={options.status !== "ready"} onChange={(e) => selectOption(e.target.value)}>
                  <option value="">{c.choose}</option>
                  {available.map((o) => <option key={o.option_key} value={o.option_key}>{optionName(o)}</option>)}
                </select>
              </Field></FormRow>
              {mapOnly && <p className="mt-warn" role="status" style={{ marginTop: 12 }}>{c.mapOnly}</p>}
              {option?.delivery_kind === "home" && (
                <FormRow style={{ marginTop: 12 }}>
                  <Field id="mo-region" label={c.region}><input id="mo-region" value={home.region} maxLength={100} onChange={(e) => setHome({ ...home, region: e.target.value })} /></Field>
                  <Field id="mo-city" label={c.city}><input id="mo-city" data-testid="mo-city" value={home.city} maxLength={100} onChange={(e) => setHome({ ...home, city: e.target.value })} /></Field>
                  <Field id="mo-postal" label={c.postal} width="short"><input id="mo-postal" value={home.postal_code} maxLength={20} onChange={(e) => setHome({ ...home, postal_code: e.target.value })} /></Field>
                  <Field id="mo-line1" label={c.line1} width="long"><input id="mo-line1" data-testid="mo-line1" value={home.line1} maxLength={200} onChange={(e) => setHome({ ...home, line1: e.target.value })} /></Field>
                  <Field id="mo-line2" label={c.line2} width="long"><input id="mo-line2" value={home.line2} maxLength={200} onChange={(e) => setHome({ ...home, line2: e.target.value })} /></Field>
                </FormRow>
              )}
              {option && option.delivery_kind !== "home" && !mapOnly && (
                <>
                  <button type="button" onClick={() => setChoosingCVS(true)}>{signed.choose}</button>
                  {cvs.store_code && <div className="ops-store-card"><strong>{option.delivery_kind === "cvs_familymart" ? signed.family : option.delivery_kind === "cvs_711" ? "7-ELEVEN" : c.kinds[option.delivery_kind]} · {cvs.store_name} · {cvs.store_code}</strong><p>{cvs.store_address}</p></div>}
                  {choosingCVS && <CvsDraftPicker locale={locale} kind={option.delivery_kind} onChoose={(item) => { setCVS({ store_code: item.code, store_name: item.name, store_address: item.address }); setChoosingCVS(false); }} onCancel={() => setChoosingCVS(false)} />}
                  <details><summary>{signed.other}</summary><p className="mt-note">{c.cvsHint}</p>
                  <FormRow style={{ marginTop: 8 }}>
                    <Field id="mo-store-code" label={c.storeCode} width="short"><input id="mo-store-code" data-testid="mo-store-code" value={cvs.store_code} maxLength={32} onChange={(e) => setCVS({ ...cvs, store_code: e.target.value })} /></Field>
                    <Field id="mo-store-name" label={c.storeName}><input id="mo-store-name" data-testid="mo-store-name" value={cvs.store_name} maxLength={40} onChange={(e) => setCVS({ ...cvs, store_name: e.target.value })} /></Field>
                    <Field id="mo-store-address" label={c.storeAddress} width="long"><input id="mo-store-address" data-testid="mo-store-address" value={cvs.store_address} maxLength={120} onChange={(e) => setCVS({ ...cvs, store_address: e.target.value })} /></Field>
                  </FormRow>
                  </details>
                </>
              )}
            </section>
            <fieldset className="mt-card">
              <legend>{c.paymentTitle}</legend>
              {option ? option.payment_modes.map((m) => (
                <label key={m} className="mt-radio">
                  <input type="radio" name="mode" value={m} checked={mode === m} onChange={() => setMode(m)} data-testid={`mo-mode-${m}`} />
                  {m === "bank_transfer" ? c.bank : m === "cash_on_delivery" ? c.cod : c.pickup}
                </label>
              )) : <p className="mt-note">{c.choose}</p>}
              {mode === "cash_on_delivery" && option?.cod_max_minor !== undefined && option.cod_carrier && (
                <p className="mt-note" data-testid="mo-cod-note">
                  {c.codNote(money(locale, option.currency, option.cod_surcharge_minor ?? 0), money(locale, option.currency, option.cod_max_minor),
                    option.cod_carrier === "black_cat" ? codCopy[locale].carrierBlackCat : codCopy[locale].carrierHsinchu)}
                </p>
              )}
              <p className="mt-note">{c.noCard}</p>
              <details><summary>{signed.advanced}</summary><FormRow style={{ marginTop: 14 }}><Field id="mo-buyer-locale" label={c.linkTitle} width="short">
                <select id="mo-buyer-locale" value={buyerLocale} onChange={(e) => setBuyerLocale(e.target.value as Locale)}>
                  {(["zh-TW", "zh-CN", "en"] as const).map((l) => <option key={l} value={l}>{localeNames[l]}</option>)}
                </select>
              </Field></FormRow></details>
            </fieldset>
            </>}
            {failure && <p className="mt-warn" role="alert" data-testid="manual-order-error">{failure}{uncertain ? ` ${c.retrySame}` : ""}</p>}
            {problem && <p className="mt-note" data-testid="manual-order-hint">{c.problems[problem]}</p>}
            <div className="mt-actions">
              {reviewing ? <button type="submit" className="primary" data-testid="manual-order-submit" disabled={!!problem || mapOnly || busy || !boundary}>
                {busy ? c.submitting : uncertain ? c.retryButton : c.submit}
              </button> : <button type="button" className="primary" disabled={!!problem || mapOnly || busy || !boundary} onClick={() => setReviewing(true)}>{signed.review}</button>}
              <Link href={`/${locale}/orders${storeQuery}`}>{c.back}</Link>
            </div>
          </form>
        )}
      </div>
    </WorkspaceFrame>
  );
}

// Read-only product selection is isolated from the order draft until Apply.
function ItemPicker({ locale, store, lines, setLines }: { locale: Locale; store: Store; lines: Line[]; setLines: (next: Line[]) => void }) {
  const c = toolsCopy[locale].manual, signed = orderOpsCopy[locale];
  const [q, setQ] = useState("");
  const [results, setResults] = useState<ProductDetail[]>([]);
  const [opened, setOpened] = useState(false);
  const [pending, setPending] = useState<Line[]>([]);
  const [searching, setSearching] = useState(false);
  const [failure, setFailure] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);
  const controller = useRef<AbortController | null>(null);
  useEffect(() => () => controller.current?.abort(), []);
  useEffect(() => { if (opened) dialog.current?.showModal(); else dialog.current?.close(); }, [opened]);
  async function search() {
    controller.current?.abort();
    const active = new AbortController(); controller.current = active;
    setSearching(true); setFailure(false); setResults([]);
    try {
      const page = await readProducts(store.id, q.trim(), "active", "", active.signal);
      const details = await Promise.all(page.items.map((product) => readProduct(store.id, product.id, active.signal)));
      if (!active.signal.aborted) setResults(details);
    } catch { if (!active.signal.aborted) setFailure(true); }
    if (!active.signal.aborted) setSearching(false);
  }
  const close = () => { controller.current?.abort(); setOpened(false); setPending([]); };
  const ordered = useMemo(() => lines, [lines]);
  return <section className="mt-card" data-testid="mo-items">
    <h2>{signed.items}</h2>
    {ordered.length === 0 ? <p className="mt-note">{c.noLines}</p> : <ul className="mt-lines" data-testid="mo-lines">
      {ordered.map((line) => <li key={line.sku_id}>
        <span>{line.label}<small>{line.code} · {line.price}</small></span>
        <input type="number" min={1} max={1000} step={1} aria-label={c.quantity} value={line.quantity}
          onChange={(e) => setLines(lines.map((item) => item.sku_id === line.sku_id ? { ...item, quantity: Math.max(1, Math.min(1000, Math.trunc(Number(e.target.value) || 1))) } : item))} />
        <button type="button" onClick={() => setLines(lines.filter((item) => item.sku_id !== line.sku_id))}>{c.remove}</button>
      </li>)}
    </ul>}
    <button type="button" className="ops-wide" data-testid="mo-open-picker" onClick={() => { setPending([]); setOpened(true); void search(); }}>{signed.add}</button>
    <dialog className="ops-picker" ref={dialog} aria-labelledby="mo-picker-title" onCancel={close}>
      <h2 id="mo-picker-title">{signed.picker}</h2><p>{signed.retained}</p>
      <Field id="mo-search" label={c.search}><input id="mo-search" data-testid="mo-search" value={q} maxLength={120} onChange={(e) => setQ(e.target.value)}
        onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); void search(); } }} /></Field>
      <button type="button" data-testid="mo-search-button" disabled={searching} onClick={() => void search()}>{c.searchButton}</button>
      {searching && <p role="status">{c.loading}</p>}
      {failure && <p role="alert">{c.unavailable}</p>}
      {!searching && !failure && results.length === 0 && <p>{c.noResults}</p>}
      <ul className="mt-results">{results.map((product) => <li key={product.id}><strong>{product.name}</strong><ul className="mt-variants">
        {product.skus.filter((variant) => variant.price_minor >= 0).map((variant) => {
          const chosen = lines.some((line) => line.sku_id === variant.id), staged = pending.some((line) => line.sku_id === variant.id);
          return <li key={variant.id}><span>{variant.title}<small>{variant.code} · {money(locale, variant.currency, variant.price_minor)} · {c.available} {variant.available}</small></span>
            <button type="button" disabled={chosen} aria-pressed={staged} onClick={() => setPending(staged ? pending.filter((line) => line.sku_id !== variant.id) : [...pending, {
              sku_id: variant.id, quantity: 1, label: `${product.name} · ${variant.title}`, code: variant.code, price: money(locale, variant.currency, variant.price_minor),
            }])}>{chosen ? signed.selected : `${signed.select} ${product.name} · ${variant.title}`}</button>
          </li>;
        })}
      </ul></li>)}</ul>
      <div className="ops-actions"><button type="button" onClick={close}>{signed.cancel}</button>
        <button type="button" className="primary" data-testid="mo-apply-picker" disabled={pending.length === 0} onClick={() => { setLines([...lines, ...pending.filter((item) => !lines.some((line) => line.sku_id === item.sku_id))]); close(); }}>{signed.apply}</button></div>
    </dialog>
  </section>;
}

function CvsDraftPicker({ locale, kind, onChoose, onCancel }: {
  locale: Locale; kind: string; onChoose: (item: { code: string; name: string; address: string }) => void; onCancel: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => { dialog.current?.showModal(); }, []);
  const brands = { cvs_711: "UNIMART", cvs_familymart: "FAMI", cvs_hilife: "HILIFE", cvs_okmart: "OKMART" } as const;
  const brand = Object.hasOwn(brands, kind) ? brands[kind as keyof typeof brands] : null;
  return <dialog ref={dialog} className="ops-picker" onCancel={onCancel}>{brand ? <CvsLookup locale={locale} brand={brand} onChoose={onChoose} onCancel={onCancel} /> :
    <><p role="status">{orderOpsCopy[locale].unavailable}</p><button type="button" onClick={onCancel}>{orderOpsCopy[locale].cancel}</button></>}</dialog>;
}
