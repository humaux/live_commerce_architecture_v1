// Purpose: Owns the merchant manual-order entry workflow.
// Depends on: @live-commerce/format (Taipei store timestamps), react, next/link, @live-commerce/i18n, @live-commerce/ui, @/lib/model, @/lib/client, @/lib/customers-client, @/lib/catalog-v2-client, @/lib/catalog-v2-model, @/lib/settings-client, @/lib/merchant-tools-client, @/lib/merchant-tools-model, @/lib/merchant-tools-copy, @/lib/cod-copy, ./WorkspaceFrame, ./AdminPageHeader, ./CvsLookup, ./orders/ManualOrderMissing, ./order-ops.css, ./OperationalForms.module.css, ./orders.css, ./customers.css, ./merchant-tools.css
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
import { useEffect, useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { deliveryOptionGroups, orderOpsCopy, localeNames, type Locale } from "@live-commerce/i18n";
import { displayTime } from "@live-commerce/format";
import { Badge, Field, FormRow } from "@live-commerce/ui";
import { ManualOrderMemberFields, type OrderMemberSelection } from "./members/ManualOrderMemberFields";
import { ManualOrderAwardResult } from "./members/ManualOrderAwardResult";
import { orderPointsCopy, validOrderPoints } from "./members/order-points-copy";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { catalogCopy } from "@/lib/catalog-v2-copy";
import { readProduct, readProducts } from "@/lib/catalog-v2-client";
import type { ProductDetail, Variant } from "@/lib/catalog-v2-model";
import { sessionBoundary } from "@/lib/settings-client";
import { placeManualOrder, readManualOptions, readManualPrefill, readManualReceipt, regenerateManualLink } from "@/lib/merchant-tools-client";
import { draftProblem, manualBody, pickupRecipientErrors, recipientGaps, type ManualDraft, type ManualOption, type ManualPaymentMode, type ManualPrefill, type ManualResult, type ManualSourceContext } from "@/lib/merchant-tools-model";
import { canonicalUUID } from "@/lib/orders-model";
import { permitted } from "@/src/features/messages/privacy";
import { conversationOrderCopy, toolsCopy } from "@/lib/merchant-tools-copy";
import { codCopy } from "@/lib/cod-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { CvsLookup } from "./CvsLookup";
import { ManualOrderMissing } from "./orders/ManualOrderMissing";
import "./order-ops.css";
import s from "./OperationalForms.module.css";
import "./orders.css";
import "./customers.css";
import "./merchant-tools.css";

type Line = { sku_id: string; quantity: number; limit: number; label: string; code: string; price: string };
const blankHome = { region: "", city: "", postal_code: "", line1: "", line2: "" };
const blankCVS = { store_code: "", store_name: "", store_address: "" };

type ManualPageProps = {
  locale: Locale; stores: Store[]; store: Store | null; initialError: ReadCode | null; renderKey: string;
};

type ConversationDraft = {
  source: ManualSourceContext;
  prefill: ManualPrefill;
  remember: (key: string | null) => void;
};

/** Both entries use the same draft and placement command. */
export function ManualOrder({ conversation, ...page }: ManualPageProps & { conversation?: string }) {
  return conversation ? <ConversationOrder {...page} conversation={conversation} /> : <ManualOrderForm {...page} />;
}

function ConversationOrder(props: ManualPageProps & { conversation: string }) {
  const { locale, store, initialError, renderKey, conversation } = props;
  const c = conversationOrderCopy[locale];
  const fallback = toolsCopy[locale].manual;
  const allowed = ["inbox:read", "orders:read", "catalog:read", "inventory:reserve"].every((permission) => permitted(store, permission));
  const prefill = useGuardedRead(`${renderKey}|${store?.id ?? ""}|${conversation}`,
    store && allowed ? (signal) => readManualPrefill(store.id, conversation, signal) : null, initialError);
  const [boundary, setBoundary] = useState("");
  const [storage, setStorage] = useState<{ name: string; key: string | null } | null>(null);
  const [started, setStarted] = useState<string | null>(null);
  const [saved, setSaved] = useState<{ name: string; result: ManualResult } | null>(null);
  const [checking, setChecking] = useState(false);
  useEffect(() => {
    let live = true;
    void sessionBoundary().then((value) => { if (live) setBoundary(value); }, () => { if (live) setBoundary(""); });
    return () => { live = false; };
  }, [renderKey, prefill.status]);
  const storageName = store && boundary ? `manual-order-receipt|${store.id}|${conversation}|${boundary}` : "";
  useEffect(() => {
    if (!storageName) return;
    let key: string | null = null;
    try { key = sessionStorage.getItem(storageName); } catch { /* Storage can be restricted. */ }
    setStorage({ name: storageName, key: key && canonicalUUID.test(key) ? key : null });
  }, [storageName]);
  function remember(key: string | null) {
    if (!storageName) return;
    try {
      if (key) sessionStorage.setItem(storageName, key);
      else sessionStorage.removeItem(storageName);
    } catch { /* The exact request remains available in the open form. */ }
  }
  async function check(key: string) {
    if (!store || checking) return;
    setChecking(true);
    try {
      const result = await readManualReceipt(store.id, key, boundary);
      if (result.state === "committed") {
        setSaved({ name: storageName, result: result.receipt });
        remember(null);
      }
    } catch { /* An unavailable original receipt keeps the outcome unknown. */ }
    setChecking(false);
  }
  const source = prefill.data?.order_context;
  const recoveryKey = storage?.name === storageName ? storage.key : null;
  const recovered = saved?.name === storageName && prefill.status === "ready" ? saved.result : null;
  const returnURL = `/${locale}/messages?store=${store?.id ?? ""}&conversation_id=${conversation}`;
  if (store && recovered) return <ManualOrderForm {...props} initialReceipt={recovered} returnURL={recovered.order_origin ? returnURL : undefined} />;
  const ready = source?.state === "ready" && prefill.data;
  if (ready && storage?.name === storageName && !recoveryKey && (source.latest_order_id === null || started === source.revision)) {
    const { conversation_id, bundle_ids, revision } = source;
    return <ManualOrderForm key={revision} {...props} conversationDraft={{ source: { conversation_id, bundle_ids, revision }, prefill: ready, remember }} returnURL={returnURL} />;
  }
  const existing = source?.state === "existing_order" ? source.order_id : source?.state === "ready" ? source.latest_order_id : null;
  const status = recoveryKey || source?.state === "review_required" && source.reason === "source_order_pending" ? c.pending
    : prefill.status === "forbidden" || store && !allowed ? fallback.forbidden
    : prefill.status === "signed-out" ? fallback.signedOut
    : prefill.status === "loading" || prefill.status === "hidden" || source?.state === "ready" && !storage ? fallback.loading
    : existing ? c.previous : source?.state === "review_required" ? c.blocked : c.unavailable;
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? fallback.noStore} active="orders">
      <div className={`orders-page customers-page ops-task ${s.page}`} data-testid="manual-order-page">
        <AdminPageHeader locale={locale} title={orderOpsCopy[locale].manual} />
        <p role="status" data-testid="conversation-order-status">{status}</p>
        <div className="mt-actions">
          {existing && <Link data-testid="conversation-order-existing" href={`/${locale}/orders/${existing}?store=${store?.id}`}>{c.view}</Link>}
          {source?.state === "ready" && existing && !recoveryKey && storage?.name === storageName &&
            <button type="button" onClick={() => setStarted(source.revision)}>{c.newDraft}</button>}
          {recoveryKey && <button type="button" disabled={checking} onClick={() => void check(recoveryKey)}>{c.check}</button>}
          {store && <Link href={returnURL} prefetch={false}>{c.back}</Link>}
        </div>
      </div>
    </WorkspaceFrame>
  );
}

function prefillLines(prefill: ManualPrefill | undefined, locale: Locale, currency: string): Line[] {
  if (prefill?.order_context.state !== "ready") return [];
  const lines = new Map<string, Line>();
  for (const item of prefill.items) {
    if (!item.sellable || !prefill.order_context.bundle_ids.includes(item.bundle_id)) continue;
    const previous = lines.get(item.sku_id);
    lines.set(item.sku_id, { sku_id: item.sku_id, quantity: (previous?.quantity ?? 0) + item.quantity, limit: 1000,
      label: item.name, code: item.variant, price: money(locale, currency, item.catalog_price_minor) });
  }
  return [...lines.values()];
}

function ManualOrderForm({
  locale, stores, store, initialError, renderKey, conversationDraft, initialReceipt, returnURL,
}: ManualPageProps & { conversationDraft?: ConversationDraft; initialReceipt?: ManualResult; returnURL?: string }) {
  const c = toolsCopy[locale].manual;
  const sourceCopy = conversationOrderCopy[locale];
  const prefill = conversationDraft?.prefill;
  const signed = orderOpsCopy[locale];
  const options = useGuardedRead(`${renderKey}|${locale}|${store?.id ?? ""}`, store ? (signal) => readManualOptions(store.id, signal) : null, initialError);
  const [boundary, setBoundary] = useState("");
  useEffect(() => {
    let live = true;
    sessionBoundary().then((value) => live && setBoundary(value), () => live && setBoundary(""));
    return () => { live = false; };
  }, [renderKey]);

  const [memberBusy, setMemberBusy] = useState(false);
  const canPoints = !!store?.permissions?.includes("customers:points");
  const memberCopy = orderPointsCopy[locale];
  const memberScope = `${renderKey}|${store?.id ?? ""}|${boundary}|${store?.permissions?.join(",") ?? ""}`;
  const [selectionState,setSelectionState] = useState<{scope:string;value:OrderMemberSelection}>({scope:memberScope,value:{member:null,points:""}});
  const memberSelection:OrderMemberSelection = selectionState.scope===memberScope ? selectionState.value : {member:null,points:""};
  const setMemberSelection = (value:OrderMemberSelection) => setSelectionState({scope:memberScope,value});
  const activeMemberScope = useRef(memberScope);
  activeMemberScope.current = memberScope;
  const orderAuthority = useRef({scope:memberScope,status:options.status,generation:0});
  if(orderAuthority.current.scope!==memberScope || orderAuthority.current.status!==options.status) {
    orderAuthority.current={scope:memberScope,status:options.status,generation:orderAuthority.current.generation+1};
  }
  const memberProblem = canPoints && memberSelection.points !== "" && (!memberSelection.member || !validOrderPoints(memberSelection.points));
  useEffect(() => {
    setMemberSelection({member:null,points:""}); setMemberBusy(false);
  }, [memberScope]);
  useEffect(() => {
    if(options.status!=="ready") {setMemberSelection({member:null,points:""});setMemberBusy(false);}
  }, [options.status]);
  const [lines, setLines] = useState(() => prefillLines(prefill, locale, store?.currency ?? "TWD"));
  const [name, setName] = useState(prefill?.customer?.name ?? "");
  const [phone, setPhone] = useState(prefill?.customer?.phone ?? "");
  const [email, setEmail] = useState(prefill?.customer?.email ?? "");
  const [optionKey, setOptionKey] = useState(prefill?.last_delivery?.option_key ?? "");
  const [mode, setMode] = useState<ManualPaymentMode | "">("");
  const [home, setHome] = useState(prefill?.last_delivery?.home_address ?? blankHome);
  const [cvs, setCVS] = useState(prefill?.last_delivery?.cvs ?? blankCVS);
  const [buyerLocale, setBuyerLocale] = useState<Locale>("zh-TW");
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState("");
  const [uncertain, setUncertain] = useState(false);
  const [placed, setPlaced] = useState<ManualResult | null>(initialReceipt ?? null);
  const [lookupOnly, setLookupOnly] = useState(!!initialReceipt);
  const [existingOrder, setExistingOrder] = useState("");
  const placedScope=useRef("");
  const [codDue, setCodDue] = useState<number | null>(null); // total + fee the carrier collects (COD orders only), from the options the order was placed with
  const [copied, setCopied] = useState(false);
  const [regenBusy, setRegenBusy] = useState(false);
  const [regenFailure, setRegenFailure] = useState("");
  const attempt = useRef<{ body: string; key: string; payload: ReturnType<typeof manualBody> & {member_award?:{member_id:string;points:number}; source_context?: ManualSourceContext} } | null>(null);
  const regen = useRef<{ body: string; key: string } | null>(null);

  const [reviewing, setReviewing] = useState(false);
  const [missing, setMissing] = useState(false); // signed 訂單缺少收件資料: shown instead of the review while recipient fields are missing
  const [choosingCVS, setChoosingCVS] = useState(false);
  const available: ManualOption[] = options.data ?? [];
  const option = available.find((item) => item.option_key === optionKey) ?? null;
  const optionName = (o: ManualOption) =>
    `${locale === "zh-CN" ? o.name_hans : locale === "zh-TW" ? o.name_hant : o.name_en || o.name_hant} · ${c.kinds[o.delivery_kind] ?? o.delivery_kind}`;
  const mapOnly = !!option && option.delivery_kind !== "home" && option.pickup_selection !== "buyer_entered";
  const draft: ManualDraft = { lines, name, phone, email, option, mode, home, cvs, locale: buyerLocale };
  const problem = draftProblem(draft);
  const pickupErrors = mode === "pay_at_pickup" ? pickupRecipientErrors(name, phone) : null;
  const gaps = recipientGaps(draft); // non-empty only when the recipient is the sole thing missing
  const showMissing = missing && gaps.length > 0;
  const initialOption = useRef(optionKey);
  useEffect(() => {
    if (!initialOption.current || options.status !== "ready") return;
    initialOption.current = "";
    if (option?.payment_modes.length === 1) setMode(option.payment_modes[0]);
  }, [option, options.status]);

  function selectOption(key: string) {
    setOptionKey(key); setCVS(blankCVS); setReviewing(false);
    const next = available.find((item) => item.option_key === key);
    setMode(next && next.payment_modes.length === 1 ? next.payment_modes[0] : "");
  }
  useEffect(() => {
    attempt.current=null; setBusy(false); setUncertain(false); setPlaced(initialReceipt ?? null);
    if (initialReceipt) placedScope.current = memberScope;
  }, [memberScope, initialReceipt]);
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!store || existingOrder || mapOnly || busy || memberBusy || options.status!=="ready" || !boundary) return;
    if (!uncertain && memberProblem) return;
    if (gaps.length) { setMissing(true); setReviewing(false); return; } // submission stays blocked until the recipient is complete
    if (problem) return;
    if (!reviewing) { setReviewing(true); return; }
    const body = uncertain && attempt.current ? attempt.current.payload : {
      ...manualBody(draft), ...(conversationDraft ? {source_context: conversationDraft.source} : {}), ...(canPoints && memberSelection.member && memberSelection.points!=="" ?
        {member_award:{member_id:memberSelection.member.id,points:Number(memberSelection.points)}} : {})
    };
    const submittedScope=memberScope, submittedGeneration=orderAuthority.current.generation;
    const text = JSON.stringify(body);
    // An uncertain answer retains the exact submitted payload/key; edits cannot create a replacement order.
    if (!attempt.current || attempt.current.body !== text) attempt.current = { body: text, key: crypto.randomUUID(), payload: body };
    setBusy(true);
    setFailure("");
    const submittedAttempt=attempt.current;
    conversationDraft?.remember(submittedAttempt.key);
    const outcome = await placeManualOrder(store.id, submittedAttempt.key, body, boundary);
    const sameSession=(await sessionBoundary().catch(()=>""))===boundary;
    if (!sameSession || activeMemberScope.current!==submittedScope || orderAuthority.current.generation!==submittedGeneration || orderAuthority.current.status!=="ready") {
      if(attempt.current===submittedAttempt) {
        setBusy(false);
        if(sameSession && activeMemberScope.current===submittedScope) {setUncertain(true);setFailure(c.retrySame);}
        else {attempt.current=null;setUncertain(false);setPlaced(null);}
      }
      return;
    }
    setBusy(false);
    if (outcome.ok) {
      conversationDraft?.remember(null);
      setLookupOnly(false);
      setCodDue(outcome.value.payment_mode === "cash_on_delivery" ? outcome.value.total_minor + (option?.cod_surcharge_minor ?? 0) : null);
      placedScope.current=submittedScope;
      setPlaced(outcome.value);
      return;
    }
    setUncertain(outcome.uncertain);
    if (outcome.orderId) setExistingOrder(outcome.orderId);
    const amount = outcome.pickupAmount;
    if (amount) {
      const twd = (minor: number) => money(locale, "TWD", minor);
      const action = amount.total_minor > amount.max_minor ? c.pickupReduce :
        amount.total_minor < 100 || amount.total_minor % 100 !== 0 ? c.pickupWhole : c.pickupChanged;
      setFailure([c.pickupQuote(twd(amount.subtotal_minor), twd(amount.shipping_minor), twd(amount.total_minor)), c.pickupCap(twd(amount.max_minor)), action].join(" "));
    } else {
      setFailure(outcome.uncertain ? c.retrySame : outcome.orderId ? sourceCopy.previous :
        outcome.code === "source_changed" || outcome.code === "not_found" && conversationDraft ? sourceCopy.unavailable : c.errors[outcome.code] ?? c.errors.default);
    }
    if (!outcome.uncertain) {
      conversationDraft?.remember(null);
      attempt.current = null;
    }
  }
  async function checkOriginal() {
    const original = attempt.current;
    if (!store || !original || busy) return;
    const submittedScope = memberScope;
    setBusy(true);
    try {
      const result = await readManualReceipt(store.id, original.key, boundary);
      if (activeMemberScope.current !== submittedScope || attempt.current !== original) return;
      if (result.state === "committed") {
        conversationDraft?.remember(null);
        setLookupOnly(true);
        placedScope.current = submittedScope;
        setPlaced(result.receipt);
      } else setFailure(sourceCopy.pending);
    } catch {
      if (activeMemberScope.current === submittedScope) setFailure(sourceCopy.pending);
    } finally {
      if (activeMemberScope.current === submittedScope) setBusy(false);
    }
  }
  function reset() {
    setReviewing(false); setChoosingCVS(false); setMemberSelection({member:null,points:""}); setMemberBusy(false);
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
        {!showMissing && <AdminPageHeader locale={locale} title={signed.manual} description={placed ? c.created : signed.quote} />}
        {returnURL && (!placed || placed.order_origin) && <Link data-testid="manual-order-return-conversation" href={returnURL} prefetch={false}>{sourceCopy.back}</Link>}
        {stores.length > 1 && !conversationDraft && !initialReceipt && (
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
        {!placed && conversationDraft && options.status === "ready" && !available.length && <p role="status">{sourceCopy.noDelivery}</p>}
        {existingOrder && <Link data-testid="conversation-order-existing" href={`/${locale}/orders/${existingOrder}${storeQuery}`}>{sourceCopy.view}</Link>}
        {placed && store && (
          <section className="mt-card" data-testid="manual-order-result" aria-label={c.created}>
            <h2>{c.created}</h2>
            <p role="status"><Badge tone={placed.commercial_state === "CONFIRMED" ? "success" : "warning"}>{placed.commercial_state === "AWAITING_TRANSFER" ? c.waiting : placed.commercial_state === "AWAITING_COLLECTION" ? c.waitingCollection : c.confirmed}</Badge></p>
            <dl className="mt-stats">
              <div><dt>{c.orderId}</dt><dd style={{ fontSize: 14 }}>{placed.order_id.slice(0, 8)}</dd></div>
              <div><dt>{conversationDraft && placed.payment_mode === "pay_at_pickup" ? c.pickup : c.total}</dt><dd>{money(locale, placed.currency, placed.total_minor)}</dd></div>
              {codDue !== null && <div data-testid="manual-order-cod-due"><dt>{c.codDue}</dt><dd>{money(locale, placed.currency, codDue)}</dd></div>}
              {placed.payment_mode === "bank_transfer" && <div><dt>{c.expires}</dt><dd style={{ fontSize: 14 }}>{displayTime(locale, placed.expires_at)}</dd></div>}
            </dl>
            {!initialReceipt && <><ul>{lines.map((line) => <li key={line.sku_id}>{line.label} · {line.quantity}</li>)}</ul>
            <dl>
              <div><dt>{signed.name}</dt><dd>{name}</dd></div><div><dt>{signed.phone}</dt><dd>{phone}</dd></div>
              <div><dt>{c.delivery}</dt><dd>{option ? optionName(option) : c.choose}</dd></div>
              <div><dt>{option?.delivery_kind === "home" ? c.line1 : signed.pickup}</dt><dd>{option?.delivery_kind === "home" ?
                [home.region, home.city, home.postal_code, home.line1, home.line2].filter(Boolean).join(" ") : `${cvs.store_name} · ${cvs.store_code} · ${cvs.store_address}`}</dd></div>
              <div><dt>{c.paymentTitle}</dt><dd>{placed.payment_mode === "bank_transfer" ? c.bank : placed.payment_mode === "cash_on_delivery" ? c.cod : c.pickup}</dd></div>
            </dl></>}
            {placed.live_price && placed.live_price_reason !== "no_bundle" && <p data-testid="manual-order-live-price">{sourceCopy.price} · {placed.live_price === "applied"
              ? placed.live_price_reason === "some_lines_catalog" ? sourceCopy.some : sourceCopy.applied
              : `${sourceCopy.catalog} · ${placed.live_price_reason === "permission" ? sourceCopy.permission : sourceCopy.ineligible}`}</p>}
            {placedScope.current===memberScope && placed.member_award && options.status==="ready" && <ManualOrderAwardResult key={`${memberScope}|${placed.order_id}`} award={placed.member_award} locale={locale} store={store} scope={memberScope} boundary={boundary}/>}
            {!lookupOnly && <h3>{c.linkTitle}</h3>}
            {lookupOnly ? null : placed.buyer_link ? (
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
              {!conversationDraft && !initialReceipt && <button type="button" className="primary" onClick={reset} data-testid="manual-order-another">{c.another}</button>}
              {(conversationDraft || initialReceipt) && <Link href={`/${locale}/orders/${placed.order_id}${storeQuery}`}>{sourceCopy.view}</Link>}
              <Link href={`/${locale}/orders${storeQuery}`}>{c.viewOrders}</Link>
            </div>
          </section>
        )}
        {/* The form stays mounted while the delivery choices (re)load: a guarded re-read must never wipe what the merchant typed. */}
        {!placed && store && !listFailure && showMissing && <ManualOrderMissing locale={locale} gaps={gaps} onBack={() => setMissing(false)} />}
        {!placed && store && !listFailure && !showMissing && !existingOrder && (
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
              {options.status==="ready" && canPoints && memberSelection.member && memberSelection.points!=="" && <section data-testid="manual-member-award-review"><h3>{memberCopy.title}</h3><p>{memberSelection.member.display_name}</p><p>{memberCopy.points}: +{memberSelection.points}</p></section>}
              {mode === "pay_at_pickup" && option?.pickup_max_minor && <p>{c.pickupCap(money(locale, "TWD", option.pickup_max_minor))}</p>}
              {mode === "cash_on_delivery" && option?.cod_max_minor !== undefined && option.cod_carrier && <p>{c.codNote(
                money(locale, option.currency, option.cod_surcharge_minor ?? 0), money(locale, option.currency, option.cod_max_minor),
                option.cod_carrier === "black_cat" ? codCopy[locale].carrierBlackCat : codCopy[locale].carrierHsinchu)}</p>}
              <button type="button" disabled={busy || uncertain} onClick={() => { setFailure(""); setReviewing(false); }}>{signed.backEdit}</button>
            </section> : <>
            <ItemPicker locale={locale} store={store} lines={lines} setLines={setLines} />
            <section className="mt-card">
              <h2>{signed.recipient}</h2>
              <FormRow>
                <Field id="mo-name" label={signed.name} hint={pickupErrors?.name && c.problems.pickupName}>
                  <input id="mo-name" data-testid="mo-name" value={name} maxLength={120} autoComplete="off"
                    aria-invalid={pickupErrors?.name} aria-describedby={pickupErrors?.name ? "mo-name-hint" : undefined}
                    onChange={(e) => setName(e.target.value)} />
                </Field>
                <Field id="mo-phone" label={signed.phone} width="short" hint={pickupErrors?.phone && c.problems.pickupPhone}>
                  <input id="mo-phone" data-testid="mo-phone" value={phone} inputMode="tel" maxLength={32} autoComplete="off"
                    aria-invalid={pickupErrors?.phone} aria-describedby={pickupErrors?.phone ? "mo-phone-hint" : undefined}
                    onChange={(e) => setPhone(e.target.value)} />
                </Field>
                <details><summary>{signed.advanced}</summary><Field id="mo-email" label={c.email}><input id="mo-email" data-testid="mo-email" value={email} type="email" maxLength={254} autoComplete="off" onChange={(e) => setEmail(e.target.value)} /></Field></details>
              </FormRow>

              <FormRow><Field id="mo-option" label={c.delivery} width="long">
                <select id="mo-option" data-testid="mo-option" value={optionKey} disabled={options.status !== "ready"} onChange={(e) => selectOption(e.target.value)}>
                  <option value="">{c.choose}</option>
                  {deliveryOptionGroups(locale, available).map(group => <optgroup key={group.label} label={group.label}>
                    {group.items.map(o => <option key={o.option_key} value={o.option_key}>{optionName(o)}</option>)}
                  </optgroup>)}
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
            <div hidden={reviewing}>
              <ManualOrderMemberFields key={memberScope} locale={locale} store={store} scope={memberScope} boundary={boundary} enabled={options.status==="ready"} disabled={busy||uncertain||reviewing} value={memberSelection} onChange={setMemberSelection}
                onPrefillName={memberName => { if (!conversationDraft) setName(current => current.trim() === "" ? memberName : current); }} onBusy={setMemberBusy}/>
            </div>
            {failure && <p className="mt-warn" role="alert" data-testid="manual-order-error">{failure}</p>}
            {problem && problem !== "pickupName" && problem !== "pickupPhone" && <p className="mt-note" data-testid="manual-order-hint">{c.problems[problem]}</p>}
            <div className="mt-actions">
              {/* Both steps submit through the guarded handler: changing a clicked button's type can place before review. */}
              <button type="submit" className="primary" data-testid={reviewing ? "manual-order-submit" : undefined} disabled={(!!problem && gaps.length === 0) || mapOnly || busy || memberBusy || (!uncertain && memberProblem) || !boundary}>
                {reviewing ? busy ? c.submitting : uncertain ? c.retryButton : c.submit : signed.review}
              </button>
              {uncertain && conversationDraft && <button type="button" disabled={busy} onClick={() => void checkOriginal()}>{sourceCopy.check}</button>}
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
  const trigger = useRef<HTMLButtonElement>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const controller = useRef<AbortController | null>(null);
  useEffect(() => () => controller.current?.abort(), []);
  useEffect(() => { if (opened) dialog.current?.showModal(); else dialog.current?.close(); }, [opened]);
  const quantityLimit = (sku: Variant) => Math.min(1000, sku.max_per_order ?? 1000, sku.inventory_tracked ? Math.max(0, sku.available) : 1000);
  async function search(query: string) {
    controller.current?.abort();
    const active = new AbortController(); controller.current = active;
    setSearching(true); setFailure(false); setResults([]);
    try {
      const page = await readProducts(store.id, query.trim(), "active", "", active.signal);
      const details = await Promise.all(page.items.map((product) => readProduct(store.id, product.id, active.signal)));
      if (!active.signal.aborted) {
        setResults(details);
        const limits = new Map(details.flatMap((product) => product.skus.map((sku) => [sku.id, quantityLimit(sku)] as const)));
        setPending((items) => items.map((item) => ({ ...item, limit: limits.get(item.sku_id) ?? item.limit })));
      }
    } catch { if (!active.signal.aborted) setFailure(true); }
    if (!active.signal.aborted) setSearching(false);
  }
  const close = () => { controller.current?.abort(); setOpened(false); setPending([]); };
  const overLimit = pending.some((item) => item.quantity > item.limit);
  const quantityInput = (line: Line, items: Line[], update: (next: Line[]) => void) =>
    <input type="number" style={{ width: "5rem", maxWidth: "100%" }} min={1} max={line.limit} step={1} aria-label={`${c.quantity} · ${line.label}`} value={line.quantity}
      disabled={line.limit < 1}
      onKeyDown={(event) => { if (event.key === "Enter") event.preventDefault(); }}
      onChange={(event) => {
        const quantity = Math.max(1, Math.min(line.limit, Math.trunc(Number(event.target.value) || 1)));
        update(items.map((item) => item.sku_id === line.sku_id ? { ...item, quantity } : item));
      }} />;
  return <section className="mt-card" data-testid="mo-items">
    <h2>{signed.items}</h2>
    {lines.length === 0 ? <p className="mt-note">{c.noLines}</p> : <ul className="mt-lines" data-testid="mo-lines">
      {lines.map((line) => <li key={line.sku_id}>
        <span>{line.label}<small>{line.code} · {line.price}</small></span>
        {quantityInput(line, lines, setLines)}
        <button type="button" onClick={() => setLines(lines.filter((item) => item.sku_id !== line.sku_id))}>{c.remove}</button>
      </li>)}
    </ul>}
    <button ref={trigger} type="button" className="ops-wide" data-testid="mo-open-picker" onClick={() => { setPending(lines); setQ(""); setOpened(true); void search(""); }}>{signed.add}</button>
    <dialog className="ops-picker" ref={dialog} aria-labelledby="mo-picker-title" onClose={() => trigger.current?.focus()} onCancel={close}>
      <h2 id="mo-picker-title">{signed.picker}</h2><p>{signed.retained}</p>
      <Field id="mo-search" label={c.search}><input id="mo-search" data-testid="mo-search" value={q} maxLength={120} onChange={(e) => setQ(e.target.value)}
        onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); void search(q); } }} /></Field>
      <button type="button" data-testid="mo-search-button" disabled={searching} onClick={() => void search(q)}>{c.searchButton}</button>
      {searching && <p role="status">{catalogCopy[locale].list.loading}</p>}
      {failure && <p role="alert">{catalogCopy[locale].list.unavailable}</p>}
      {!searching && !failure && results.length === 0 && <p>{c.noResults}</p>}
      <ul className="mt-results">{results.map((product) => <li key={product.id}><strong>{product.name}</strong><ul className="mt-variants">
        {product.skus.filter((variant) => variant.price_minor >= 0).map((variant) => {
          const selected = pending.find((line) => line.sku_id === variant.id);
          const label = `${product.name} · ${variant.title}`, price = money(locale, variant.currency, variant.price_minor);
          const limit = quantityLimit(variant);
          return <li key={variant.id} style={{ flexWrap: "wrap", overflowWrap: "anywhere" }}><span>{variant.title}<small>{variant.code} · {price} · {variant.inventory_tracked ? `${c.available} ${variant.available}` : signed.untracked}</small></span>
            {selected && quantityInput(selected, pending, setPending)}
            <button type="button" disabled={!selected && limit === 0} aria-pressed={!!selected}
              aria-label={`${selected ? signed.deselect : signed.select} ${label}`}
              onClick={() => setPending(selected ? pending.filter((line) => line.sku_id !== variant.id) : [...pending, {
                sku_id: variant.id, quantity: 1, limit, label, code: variant.code, price,
              }])}>{selected ? `✓ ${signed.selected} × ${selected.quantity} · ${signed.deselect}` : `${signed.select} ${label}`}</button>
          </li>;
        })}
      </ul></li>)}</ul>
      <p role="status">{signed.selected} · {c.quantity} {pending.reduce((sum, line) => sum + line.quantity, 0)}</p>
      {overLimit && <p role="alert">{signed.overLimit}</p>}
      <div className="ops-actions"><button type="button" onClick={close}>{signed.cancel}</button>
        <button type="button" className="primary" data-testid="mo-apply-picker" disabled={searching || overLimit || (!lines.length && !pending.length)} onClick={() => { setLines(pending); close(); }}>{signed.apply}</button></div>
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
