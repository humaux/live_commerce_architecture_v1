"use client";

// Create an order from the admin (/{locale}/orders/new): the merchant picks SKUs and quantities, types the customer and the delivery,
// chooses bank transfer or pay at pickup (never card) and gets the order plus a buyer link to paste into LINE or Messenger.
// BFF /api/stores/{store}/tools/{orders/manual/options, orders/manual} -> Go internal/httpapi/merchanttools.go -> internal/merchanttools
// (contract storefront-v2 G3). Products and variants come from the existing catalog reads (BFF catalog-products, products/{id}).
// There is NO price field anywhere in this form or in the request: the server quotes from the catalog (I05), so the displayed unit prices are
// information only. The submit key is kept for a retry of the SAME body (an uncertain answer replays, never double-reserves) and replaced as
// soon as the form changes. The session fence is lib/merchant-tools-client.
import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { readProduct, readProducts } from "@/lib/catalog-v2-client";
import type { ProductDetail, ProductSummary } from "@/lib/catalog-v2-model";
import { sessionBoundary } from "@/lib/settings-client";
import { placeManualOrder, readManualOptions } from "@/lib/merchant-tools-client";
import { draftProblem, manualBody, type ManualDraft, type ManualOption, type ManualPaymentMode, type ManualResult } from "@/lib/merchant-tools-model";
import { toolsCopy } from "@/lib/merchant-tools-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import "./orders.css";
import "./customers.css";
import "./merchant-tools.css";

type Line = { sku_id: string; quantity: number; label: string; code: string; price: string };
const blankHome = { region: "", city: "", postal_code: "", line1: "", line2: "" };
const blankCVS = { store_code: "", store_name: "", store_address: "" };

export function ManualOrder({
  locale, stores, store, initialError, renderKey,
}: {
  locale: Locale; stores: Store[]; store: Store | null; initialError: ReadCode | null; renderKey: string;
}) {
  const c = toolsCopy[locale].manual;
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
  const [copied, setCopied] = useState(false);
  const attempt = useRef<{ body: string; key: string } | null>(null);

  const available: ManualOption[] = options.data ?? [];
  const option = available.find((item) => item.option_key === optionKey) ?? null;
  const optionName = (o: ManualOption) =>
    `${locale === "zh-CN" ? o.name_hans : locale === "zh-TW" ? o.name_hant : o.name_en || o.name_hant} · ${c.kinds[o.delivery_kind] ?? o.delivery_kind}`;
  const mapOnly = !!option && option.delivery_kind !== "home" && option.pickup_selection !== "buyer_entered";
  const draft: ManualDraft = { lines, name, phone, email, option, mode, home, cvs, locale: buyerLocale };
  const problem = draftProblem(draft);

  function selectOption(key: string) {
    setOptionKey(key);
    const next = available.find((item) => item.option_key === key);
    setMode(next && next.payment_modes.length === 1 ? next.payment_modes[0] : "");
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!store || problem || mapOnly || busy) return;
    const body = manualBody(draft);
    const text = JSON.stringify(body);
    // A new key only when the request differs; the same body after an uncertain answer re-sends the same key (replay).
    if (!attempt.current || attempt.current.body !== text) attempt.current = { body: text, key: crypto.randomUUID() };
    setBusy(true);
    setFailure("");
    const outcome = await placeManualOrder(store.id, attempt.current.key, body, boundary);
    setBusy(false);
    if (outcome.ok) {
      setPlaced(outcome.value);
      return;
    }
    setUncertain(outcome.uncertain);
    setFailure(c.errors[outcome.code] ?? c.errors.default);
    if (!outcome.uncertain) attempt.current = null; // a definite refusal: the next send is a new attempt
  }
  function reset() {
    setPlaced(null); setLines([]); setName(""); setPhone(""); setEmail(""); setHome(blankHome); setCVS(blankCVS);
    setOptionKey(""); setMode(""); setFailure(""); setUncertain(false); setCopied(false); attempt.current = null;
  }
  const listFailure =
    options.status === "signed-out" ? c.signedOut : options.status === "forbidden" ? c.forbidden
    : options.status === "not-found" ? (store ? c.unavailable : c.noStore) : options.status === "unavailable" ? c.notConfigured : "";
  const storeQuery = store ? `?store=${store.id}` : "";
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="orders">
      <div className="orders-page customers-page" data-testid="manual-order-page">
        <header className="orders-heading">
          <h1>{c.title}</h1>
          <p>{c.subtitle}</p>
        </header>
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
            <p className="mt-ok" role="status">{placed.commercial_state === "AWAITING_TRANSFER" ? c.waiting : c.confirmed}</p>
            <dl className="mt-stats">
              <div><dt>{c.orderId}</dt><dd style={{ fontSize: 14 }}>{placed.order_id.slice(0, 8)}</dd></div>
              <div><dt>{c.total}</dt><dd>{money(locale, placed.currency, placed.total_minor)}</dd></div>
              <div><dt>{c.expires}</dt><dd style={{ fontSize: 14 }}>{new Date(placed.expires_at).toLocaleString(locale)}</dd></div>
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
                </div>
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
            <ItemPicker locale={locale} store={store} lines={lines} setLines={setLines} />
            <section className="mt-card">
              <h2>{c.customerTitle}</h2>
              <div className="mt-row">
                <label className="mt-field">{c.name}<input data-testid="mo-name" value={name} maxLength={120} autoComplete="off" onChange={(e) => setName(e.target.value)} /></label>
                <label className="mt-field">{c.phone}<input data-testid="mo-phone" value={phone} inputMode="tel" maxLength={32} autoComplete="off" onChange={(e) => setPhone(e.target.value)} /></label>
                <label className="mt-field">{c.email}<input data-testid="mo-email" value={email} type="email" maxLength={254} autoComplete="off" onChange={(e) => setEmail(e.target.value)} /></label>
              </div>
            </section>
            <section className="mt-card">
              <h2>{c.deliveryTitle}</h2>
              <label className="mt-field">{c.delivery}
                <select data-testid="mo-option" value={optionKey} disabled={options.status !== "ready"} onChange={(e) => selectOption(e.target.value)}>
                  <option value="">{c.choose}</option>
                  {available.map((o) => <option key={o.option_key} value={o.option_key}>{optionName(o)}</option>)}
                </select>
              </label>
              {mapOnly && <p className="mt-warn" role="status" style={{ marginTop: 12 }}>{c.mapOnly}</p>}
              {option?.delivery_kind === "home" && (
                <div className="mt-row" style={{ marginTop: 12 }}>
                  <label className="mt-field">{c.region}<input value={home.region} maxLength={100} onChange={(e) => setHome({ ...home, region: e.target.value })} /></label>
                  <label className="mt-field">{c.city}<input data-testid="mo-city" value={home.city} maxLength={100} onChange={(e) => setHome({ ...home, city: e.target.value })} /></label>
                  <label className="mt-field">{c.postal}<input value={home.postal_code} maxLength={20} onChange={(e) => setHome({ ...home, postal_code: e.target.value })} /></label>
                  <label className="mt-field">{c.line1}<input data-testid="mo-line1" value={home.line1} maxLength={200} onChange={(e) => setHome({ ...home, line1: e.target.value })} /></label>
                  <label className="mt-field">{c.line2}<input value={home.line2} maxLength={200} onChange={(e) => setHome({ ...home, line2: e.target.value })} /></label>
                </div>
              )}
              {option && option.delivery_kind !== "home" && !mapOnly && (
                <>
                  <p className="mt-note">{c.cvsHint}</p>
                  <div className="mt-row" style={{ marginTop: 8 }}>
                    <label className="mt-field">{c.storeCode}<input data-testid="mo-store-code" value={cvs.store_code} maxLength={32} onChange={(e) => setCVS({ ...cvs, store_code: e.target.value })} /></label>
                    <label className="mt-field">{c.storeName}<input data-testid="mo-store-name" value={cvs.store_name} maxLength={40} onChange={(e) => setCVS({ ...cvs, store_name: e.target.value })} /></label>
                    <label className="mt-field">{c.storeAddress}<input data-testid="mo-store-address" value={cvs.store_address} maxLength={120} onChange={(e) => setCVS({ ...cvs, store_address: e.target.value })} /></label>
                  </div>
                </>
              )}
            </section>
            <section className="mt-card">
              <h2>{c.paymentTitle}</h2>
              {option ? option.payment_modes.map((m) => (
                <label key={m} className="mt-radio">
                  <input type="radio" name="mode" value={m} checked={mode === m} onChange={() => setMode(m)} data-testid={`mo-mode-${m}`} />
                  {m === "bank_transfer" ? c.bank : c.pickup}
                </label>
              )) : <p className="mt-note">{c.choose}</p>}
              <p className="mt-note">{c.noCard}</p>
              <label className="mt-field" style={{ marginTop: 14, maxWidth: 260 }}>
                {c.linkTitle}
                <select value={buyerLocale} onChange={(e) => setBuyerLocale(e.target.value as Locale)}>
                  {(["zh-TW", "zh-CN", "en"] as const).map((l) => <option key={l} value={l}>{l}</option>)}
                </select>
              </label>
            </section>
            {failure && <p className="mt-warn" role="alert" data-testid="manual-order-error">{failure}{uncertain ? ` ${c.retrySame}` : ""}</p>}
            {problem && lines.length + name.length + phone.length > 0 && <p className="mt-note" data-testid="manual-order-hint">{c.problems[problem]}</p>}
            <div className="mt-actions">
              <button type="submit" className="primary" data-testid="manual-order-submit" disabled={!!problem || mapOnly || busy || !boundary}>
                {busy ? c.submitting : uncertain ? c.retryButton : c.submit}
              </button>
              <Link href={`/${locale}/orders${storeQuery}`}>{c.back}</Link>
            </div>
          </form>
        )}
      </div>
    </WorkspaceFrame>
  );
}

// Product search (active products only: a draft cannot hold stock) and variant pick. Reads are the existing catalog BFF routes.
function ItemPicker({ locale, store, lines, setLines }: { locale: Locale; store: Store; lines: Line[]; setLines: (next: Line[]) => void }) {
  const c = toolsCopy[locale].manual;
  const [q, setQ] = useState("");
  const [results, setResults] = useState<ProductSummary[] | null>(null);
  const [open, setOpen] = useState<ProductDetail | null>(null);
  const [searching, setSearching] = useState(false);
  const controller = useRef<AbortController | null>(null);
  useEffect(() => () => controller.current?.abort(), []);
  async function search() {
    controller.current?.abort();
    const active = new AbortController();
    controller.current = active;
    setSearching(true);
    setOpen(null);
    try {
      const page = await readProducts(store.id, q.trim(), "active", "", active.signal);
      if (!active.signal.aborted) setResults(page.items);
    } catch {
      if (!active.signal.aborted) setResults([]);
    }
    if (!active.signal.aborted) setSearching(false);
  }
  async function expand(id: string) {
    controller.current?.abort();
    const active = new AbortController();
    controller.current = active;
    try {
      const detail = await readProduct(store.id, id, active.signal);
      if (!active.signal.aborted) setOpen(detail);
    } catch {
      /* the list stays; the merchant can retry */
    }
  }
  const add = (v: ProductDetail["skus"][number], product: ProductDetail) => {
    if (lines.some((l) => l.sku_id === v.id)) return;
    setLines([...lines, { sku_id: v.id, quantity: 1, label: `${product.name} · ${v.title}`, code: v.code, price: money(locale, v.currency, v.price_minor) }]);
  };
  const ordered = useMemo(() => lines, [lines]);
  return (
    <section className="mt-card" data-testid="mo-items">
      <h2>{c.itemsTitle}</h2>
      {/* Not a <form>: this picker lives inside the order form, and HTML forbids nested forms (the parser would drop the inner one). */}
      <div className="mt-row">
        <label className="mt-field">{c.search}
          <input data-testid="mo-search" value={q} maxLength={120} onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); void search(); } }} />
        </label>
        <div className="mt-actions" style={{ alignSelf: "end" }}><button type="button" data-testid="mo-search-button" disabled={searching} onClick={() => void search()}>{c.searchButton}</button></div>
      </div>
      {results && (results.length === 0 ? <p className="mt-note">{c.noResults}</p> : (
        <ul className="mt-results">
          {results.map((p) => (
            <li key={p.id}>
              <div style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
                <span>{p.name}<small>{p.sku_count} · {c.available} {p.available}</small></span>
                <button type="button" className="mt-btn" onClick={() => void expand(p.id)}>{c.add}</button>
              </div>
              {open?.id === p.id && (
                <ul className="mt-variants">
                  {open.skus.filter((v) => v.price_minor >= 0).map((v) => (
                    <li key={v.id}>
                      <span>{v.title}<small>{v.code} · {money(locale, v.currency, v.price_minor)} · {c.available} {v.available}</small></span>
                      <button type="button" className="mt-btn" disabled={lines.some((l) => l.sku_id === v.id)} onClick={() => add(v, open)}>{c.add}</button>
                    </li>
                  ))}
                </ul>
              )}
            </li>
          ))}
        </ul>
      ))}
      {ordered.length === 0 ? <p className="mt-note">{c.noLines}</p> : (
        <ul className="mt-lines" data-testid="mo-lines">
          {ordered.map((l) => (
            <li key={l.sku_id}>
              <span>{l.label}<small>{l.code} · {l.price}</small></span>
              <input type="number" min={1} max={1000} step={1} aria-label={c.quantity} value={l.quantity}
                onChange={(e) => setLines(lines.map((x) => (x.sku_id === l.sku_id ? { ...x, quantity: Math.max(1, Math.min(1000, Math.trunc(Number(e.target.value) || 1))) } : x)))} />
              <button type="button" className="mt-btn" onClick={() => setLines(lines.filter((x) => x.sku_id !== l.sku_id))}>{c.remove}</button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
