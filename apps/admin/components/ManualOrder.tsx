// Purpose: Owns the merchant manual-order entry workflow.
// Depends on: Taipei format, React, Next Link, UI, scoped merchant clients/models/copy, shared form/attempt, recovery transport and inbox privacy lifecycle, workspace styles.
// Used by: apps/admin/app/[locale]/orders/new/page.tsx
// Invariants: frozen key/body until an authoritative receipt or absence seal; conceal forgets private state but preserves the opaque unresolved guard.
"use client";

// Create an order from the admin (/{locale}/orders/new): the merchant picks SKUs and quantities, types the customer and the delivery,
// chooses bank transfer, pay at pickup or (home delivery) cash on delivery (never card) and gets the order plus a buyer link to paste into LINE or Messenger.
// BFF /api/stores/{store}/tools/{orders/manual/options, orders/manual} -> Go internal/httpapi/merchanttools.go -> internal/merchanttools
// (contract storefront-v2 G3). Products and variants come from the existing catalog reads (BFF catalog-products, products/{id}).
// There is NO price field anywhere in this form or in the request: the server quotes from the catalog (I05), so the displayed unit prices are
// information only. The submitted body/key is immutable and separate from the editable draft. UNKNOWN replays the original request;
// a definitive response only unlocks an explicit new-attempt action. The session fence is lib/merchant-tools-client.
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { displayTime } from "@live-commerce/format";
import { Badge } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { sessionBoundary } from "@/lib/settings-client";
import { placeManualOrder, readManualOptions, regenerateManualLink } from "@/lib/merchant-tools-client";
import { draftProblem, manualBody, type ManualDraft, type ManualOption, type ManualPaymentMode, type ManualResult } from "@/lib/merchant-tools-model";
import { toolsCopy } from "@/lib/merchant-tools-copy";
import { emptyManualForm, type ManualFormLine, type ManualFormValues } from "@/lib/manual-order-form";
import { OrderAttempt, OrderGuard, retainOrderAttempt } from "@/lib/create-order-attempt";
import { recoverManualOrder, CreateOrderError } from "@/lib/create-order-client";
import { createOrderCopy } from "@/lib/create-order-copy";
import { useInboxPrivacy } from "@/src/features/messages/use-privacy";
import { ManualOrderFormFields } from "./ManualOrderFormFields";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import s from "./OperationalForms.module.css";
import "./orders.css";
import "./customers.css";
import "./merchant-tools.css";

const blankHome = emptyManualForm().home;
const blankCVS = emptyManualForm().cvs;

/** Owns the merchant manual-order entry workflow. User actions submit manual-order commands through merchant-tools-client. */
export function ManualOrder({
  locale, stores, store, initialError, renderKey,
}: {
  locale: Locale; stores: Store[]; store: Store | null; initialError: ReadCode | null; renderKey: string;
}) {
  const c = toolsCopy[locale].manual;
  const recoveryCopy = createOrderCopy[locale];
  const options = useGuardedRead(`${renderKey}|${locale}|${store?.id ?? ""}`, store ? (signal) => readManualOptions(store.id, signal) : null, initialError);
  const [boundary, setBoundary] = useState("");
  const [lines, setLines] = useState<ManualFormLine[]>([]);
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
  const [recoverable, setRecoverable] = useState(false);
  const [recoveryNote, setRecoveryNote] = useState("");
  const [recoveredOrder, setRecoveredOrder] = useState<string | null>(null);
  const [guarded, setGuarded] = useState(false);
  const [revoked, setRevoked] = useState(false);
  const attempt = useRef(new OrderAttempt());
  const guard = useRef<OrderGuard | null>(null);
  const guardScope = useRef("");
  const attemptOption = useRef<ManualOption | null>(null);
  const sending = useRef(false);
  const regen = useRef<{ body: string; key: string } | null>(null);
  const clear = useCallback(() => {
    attempt.current.clear(); attemptOption.current = null; regen.current = null; guard.current = null;
    guardScope.current = ""; sending.current = false;
    setLines([]); setName(""); setPhone(""); setEmail(""); setHome(blankHome); setCVS(blankCVS);
    setPlaced(null); setRecoveredOrder(null); setCodDue(null); setCopied(false); setRegenBusy(false); setRegenFailure("");
    setBoundary(""); setBusy(false); setFailure(""); setUncertain(false); setRecoverable(false); setRecoveryNote(""); setGuarded(false);
  }, []);
  const privacy = useInboxPrivacy(clear);
  useEffect(() => {
    if (!privacy.visible) return;
    let live = true;
    sessionBoundary().then((value) => live && setBoundary(value), () => live && revoke());
    return () => { live = false; };
  }, [renderKey, privacy.visible, privacy.revision]);
  useEffect(() => {
    if (privacy.visible && ["signed-out", "forbidden", "not-found"].includes(options.status)) revoke();
  }, [options.status, privacy.visible]);
  useEffect(() => {
    if (!boundary || !store) return;
    const scope = `${store.id}|${boundary}`;
    if (guardScope.current && guardScope.current !== scope) {
      attempt.current.clear(); attemptOption.current = null; regen.current = null;
      setLines([]); setName(""); setPhone(""); setEmail(""); setHome(blankHome); setCVS(blankCVS);
      setPlaced(null); setRecoveredOrder(null); setUncertain(false); setRecoverable(false); setRecoveryNote(""); setRevoked(false);
    }
    guardScope.current = scope;
    // Only the opaque unresolved flag survives reload. No request, key or buyer fields enter browser storage.
    guard.current = new OrderGuard(window.sessionStorage, store.id, store.order_guard_scope ?? "");
    setGuarded(guard.current.blocked() && !attempt.current.pending());
  }, [boundary, store?.id, store?.order_guard_scope]);

  const available: ManualOption[] = options.data ?? [];
  const option = available.find((item) => item.option_key === optionKey) ?? null;
  const mapOnly = !!option && option.delivery_kind !== "home" && option.pickup_selection !== "buyer_entered";
  const draft: ManualDraft = { lines, name, phone, email, option, mode, home, cvs, locale: buyerLocale };
  const problem = draftProblem(draft);

  function changeFields(patch: Partial<ManualFormValues>) {
    if (patch.lines !== undefined) setLines(patch.lines);
    if (patch.name !== undefined) setName(patch.name);
    if (patch.phone !== undefined) setPhone(patch.phone);
    if (patch.email !== undefined) setEmail(patch.email);
    if (patch.optionKey !== undefined) setOptionKey(patch.optionKey);
    if (patch.mode !== undefined) setMode(patch.mode);
    if (patch.home !== undefined) setHome(patch.home);
    if (patch.cvs !== undefined) setCVS(patch.cvs);
    if (patch.buyerLocale !== undefined) setBuyerLocale(patch.buyerLocale);
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!store || sending.current || guarded || revoked || !privacy.visible || attempt.current.resolved() || (!attempt.current.pending() && (problem || mapOnly))) return;
    // Draft edits never alter a submitted request. Even an invalid new draft can retry the old valid receipt.
    if (!attempt.current.pending()) {
      if (!guard.current?.arm()) { setGuarded(true); return; }
      attempt.current.prepare(manualBody(draft)); attemptOption.current = option;
    }
    const receipt = attempt.current.pending()!;
    const scope = guardScope.current;
    const ticket = privacy.fence.begin();
    sending.current = true; setBusy(true); setFailure(""); setRecoverable(false);
    try {
      const outcome = await placeManualOrder(store.id, receipt.key, JSON.parse(receipt.body), boundary);
      if (scope !== guardScope.current || !privacy.fence.current(ticket)) return;
      if (outcome.ok) {
        attempt.current.finish(); guard.current?.clear(); setUncertain(false);
        setCodDue(outcome.value.payment_mode === "cash_on_delivery" ? outcome.value.total_minor + (attemptOption.current?.cod_surcharge_minor ?? 0) : null);
        setPlaced(outcome.value); return;
      }
      const retain = retainOrderAttempt(outcome, uncertain);
      setUncertain(retain); setFailure(c.errors[outcome.code] ?? c.errors.default);
      if (!retain) { attempt.current.finish(); guard.current?.clear(); }
      else setRecoverable(true);
      if ([401, 403].includes(outcome.status)) revoke();
    } catch { if (privacy.fence.current(ticket)) { setUncertain(true); setRecoverable(true); setFailure(c.errors.retry_later); } }
    finally { if (privacy.fence.current(ticket)) { sending.current = false; setBusy(false); } }
  }
  function newAttempt() {
    if (sending.current || guarded || revoked || recoveredOrder || !attempt.current.startNew()) return false;
    attemptOption.current = null; setFailure(""); setUncertain(false); setRecoverable(false); setRecoveryNote(""); return true;
  }
  function revoke() {
    // Preserve the guard while forgetting private in-memory state. A different session may not claim this receipt.
    privacy.expire(); setRevoked(true);
  }
  async function recover() {
    const original = attempt.current.pending();
    if (!store || !original || sending.current || guarded || revoked || !boundary || !recoverable || !privacy.visible) return;
    sending.current = true; setBusy(true);
    const scope = guardScope.current;
    const ticket = privacy.fence.begin();
    try {
      const outcome = await recoverManualOrder(store.id, original, boundary, ticket.signal);
      if (scope !== guardScope.current || !privacy.fence.current(ticket)) return;
      if (!outcome.ok) {
        if ([401, 403].includes(outcome.status)) { revoke(); return; }
        setFailure(c.errors[outcome.code] ?? c.errors.default); return;
      }
      // The server serialized Begin and sealed absence; a generic business refusal alone never releases this key.
      attempt.current.finish();
      if (!guard.current?.clear()) { setGuarded(true); return; }
      setFailure(""); setUncertain(false); setRecoverable(false);
      if (outcome.value.state === "created") setRecoveredOrder(outcome.value.order_id);
      else setRecoveryNote(recoveryCopy.recovered);
    } catch (cause) {
      if (!privacy.fence.current(ticket)) return;
      if (cause instanceof CreateOrderError && [401, 403].includes(cause.status)) { revoke(); return; }
      setFailure(c.errors.retry_later);
    } finally { if (privacy.fence.current(ticket)) { sending.current = false; setBusy(false); } }
  }
  function reset() {
    if (!newAttempt()) return;
    setPlaced(null); setCodDue(null); setLines([]); setName(""); setPhone(""); setEmail(""); setHome(blankHome); setCVS(blankCVS);
    setOptionKey(""); setMode(""); setCopied(false);
    setRegenFailure(""); regen.current = null;
  }
  async function regenerate() {
    if (!store || !placed || regenBusy || revoked || !privacy.visible || placed.link_state !== "configured") return;
    const ticket = privacy.fence.begin(), scope = guardScope.current;
    const body = { order_id: placed.order_id, locale: attempt.current.pending() ? JSON.parse(attempt.current.pending()!.body).locale : buyerLocale };
    const text = JSON.stringify(body);
    // A new key only when the request differs; the same body after an uncertain answer re-sends the same key (replay, same link).
    if (!regen.current || regen.current.body !== text) regen.current = { body: text, key: crypto.randomUUID() };
    setRegenBusy(true);
    setRegenFailure("");
    try {
      const outcome = await regenerateManualLink(store.id, regen.current.key, body, boundary);
      if (scope !== guardScope.current || !privacy.fence.current(ticket)) return;
      if (outcome.ok) {
        setPlaced({ ...placed, buyer_link: outcome.value.buyer_link });
        setCopied(false);
        regen.current = null;
        return;
      }
      if ([401, 403].includes(outcome.status)) { revoke(); return; }
      setRegenFailure(c.errors[outcome.code] ?? c.errors.default);
      if (!outcome.uncertain) regen.current = null; // a definite refusal: the next send is a new attempt
    } catch {
      if (privacy.fence.current(ticket)) setRegenFailure(c.errors.retry_later);
    } finally {
      if (privacy.fence.current(ticket)) setRegenBusy(false);
    }
  }
  const listFailure =
    revoked ? c.signedOut : options.status === "signed-out" ? c.signedOut : options.status === "forbidden" ? c.forbidden
    : options.status === "not-found" ? (store ? c.unavailable : c.noStore) : options.status === "unavailable" ? c.notConfigured : "";
  const storeQuery = store ? `?store=${store.id}` : "";
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="orders">
      <div className={`orders-page customers-page ${s.page}`} data-testid="manual-order-page">
        <AdminPageHeader locale={locale} description={c.subtitle} />
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
        {guarded && <p className="mt-warn" role="alert" data-testid="manual-order-guard">{recoveryCopy.guard} <Link href={`/${locale}/orders${storeQuery}`}>{c.viewOrders}</Link></p>}
        {privacy.visible && recoveredOrder && <section className="mt-card" data-testid="manual-order-recovery-status"><h2>{c.created}</h2><Link data-testid="manual-order-recovered-order" href={`/${locale}/orders/${recoveredOrder}${storeQuery}`}>{recoveryCopy.view}</Link></section>}
        {privacy.visible && placed && store && (
          <section className="mt-card" data-testid="manual-order-result" aria-label={c.created}>
            <h2>{c.created}</h2>
            <p role="status"><Badge tone={placed.commercial_state === "CONFIRMED" ? "success" : "warning"}>{placed.commercial_state === "AWAITING_TRANSFER" ? c.waiting : placed.commercial_state === "AWAITING_COLLECTION" ? c.waitingCollection : c.confirmed}</Badge></p>
            <dl className="mt-stats">
              <div><dt>{c.orderId}</dt><dd style={{ fontSize: 14 }}>{placed.order_id.slice(0, 8)}</dd></div>
              <div><dt>{c.total}</dt><dd>{money(locale, placed.currency, placed.total_minor)}</dd></div>
              {codDue !== null && <div data-testid="manual-order-cod-due"><dt>{c.codDue}</dt><dd>{money(locale, placed.currency, codDue)}</dd></div>}
              {placed.payment_mode === "bank_transfer" && <div><dt>{c.expires}</dt><dd style={{ fontSize: 14 }}>{displayTime(locale, placed.expires_at)}</dd></div>}
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
            ) : placed.link_state === "configured" ? <><p className="mt-note">{c.replayLink}</p><button type="button" data-testid="manual-order-regenerate" disabled={regenBusy} onClick={() => void regenerate()}>{regenBusy ? c.regenerating : c.regenerate}</button>{regenFailure && <p role="alert">{regenFailure}</p>}</> : <p className="mt-warn">{c.noLink}</p>}
            <div className="mt-actions" style={{ marginTop: 16 }}>
              <button type="button" className="primary" onClick={reset} data-testid="manual-order-another">{c.another}</button>
              <Link href={`/${locale}/orders${storeQuery}`}>{c.viewOrders}</Link>
            </div>
          </section>
        )}
        {/* The form stays mounted while the delivery choices (re)load: a guarded re-read must never wipe what the merchant typed. */}
        {privacy.visible && !placed && !recoveredOrder && !guarded && store && !listFailure && (
          <form className="mt-form" onSubmit={(event) => void submit(event)} data-testid="manual-order-form" noValidate>
            <ManualOrderFormFields locale={locale} store={store}
              value={{ lines, name, phone, email, optionKey, mode, home, cvs, buyerLocale }}
              onChange={changeFields} available={available} optionsReady={options.status === "ready"} />
            {failure && <p className="mt-warn" role="alert" data-testid="manual-order-error">{failure}{uncertain ? ` ${c.retrySame}` : ""}</p>}
            {attempt.current.pending() && <p className="mt-note">{c.frozenDraft}</p>}
            {recoveryNote && <p role="status" data-testid="manual-order-recovery-status">{recoveryNote}</p>}
            {uncertain && recoverable && <p className="mt-note">{recoveryCopy.recoverExplanation}</p>}
            {problem && !attempt.current.pending() && <p className="mt-note" data-testid="manual-order-hint">{c.problems[problem]}</p>}
            <div className="mt-actions">
              <button type="submit" className="primary" data-testid="manual-order-submit" disabled={busy || !boundary || attempt.current.resolved() || (!attempt.current.pending() && (!!problem || mapOnly))}>
                {busy ? c.submitting : uncertain ? c.retryButton : c.submit}
              </button>
              {attempt.current.resolved() && <button type="button" data-testid="manual-order-new-attempt" disabled={busy} onClick={newAttempt}>{c.newAttempt}</button>}
              {uncertain && recoverable && <button type="button" data-testid="manual-order-recover-attempt" disabled={busy} onClick={() => void recover()}>{recoveryCopy.recover}</button>}
              <Link href={`/${locale}/orders${storeQuery}`}>{c.back}</Link>
            </div>
          </form>
        )}
      </div>
    </WorkspaceFrame>
  );
}
