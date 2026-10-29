// Purpose: Owns customer detail and merchant privacy-action controls.
// Depends on: react, next/link, @live-commerce/i18n, @live-commerce/ui, @/lib/presentation-copy, @/lib/model, @/lib/client, @/lib/customers-client, @/lib/customers-model, @/lib/orders-model, @/lib/orders-copy, @/lib/customers-copy, ./CustomerTags, ./CustomerHistoricalOrders, ./WorkspaceFrame, ./AdminPageHeader, ./orders.css, ./order-actions.css, ./customers.css
// Invariants: existing synchronous privacy contracts; uncertain commands are never automatically replayed.
// Used by: apps/admin/app/[locale]/customers/[customer]/page.tsx
"use client";

// Merchant customer detail (/{locale}/customers/{customer}): facts, orders, claims, consent with "Record withdrawal",
// privacy review with name-and-scope confirmation; unknown outcomes remain read-only across reload.
// BFF GET /api/stores/{store}/customers/{id} (customers:read); POST .../customers/{id}/consent-withdrawals | exports |
// erasure (customers:privacy, Idempotency-Key) -> Go /v1/admin/stores/{id}/customers/{id}[/...]
// (internal/httpapi/customers.go; internal/customers.{Get,WithdrawConsent,Export,Erase}).
// No consent, erasure or money authority here: every write is re-authorized by Go and the page re-GETs afterwards.
import { useEffect, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { Badge, TableFrame } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import {
  postErasure,
  postExport,
  postWithdrawal,
  readCustomer,
  useGuardedRead,
  type ReadCode,
  type WriteOutcome,
} from "@/lib/customers-client";
import { consentPairs, newKey, type CustomerDetail as Detail } from "@/lib/customers-model";
import { displayTime } from "@/lib/orders-model";
import { ordersCopy } from "@/lib/orders-copy";
import { customersCopy, type CustomersCopy } from "@/lib/customers-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { CustomerTags } from "./CustomerTags";
import { CustomerHistoricalOrders } from "./CustomerHistoricalOrders";
import "./orders.css";
import "./order-actions.css";
import "./customers.css";

type Notice = { tone: "ok" | "bad"; text: string } | null;
const errorText = (c: CustomersCopy, code: string) => c.errors[code] ?? c.errors.default;

/** Owns customer detail and merchant privacy-action controls. User actions request consent withdrawal, export or erasure through customers-client. */
export function CustomerDetail({
  locale,
  stores,
  store,
  customerID,
  q = "",
  tag = "",
  initialError,
  renderKey,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  customerID: string;
  q?: string;
  tag?: string;
  initialError: ReadCode | null;
  renderKey: string;
}) {
  const c = customersCopy[locale];
  const read = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}|${customerID}`,
    store ? (signal) => readCustomer(store.id, customerID, signal) : null,
    initialError,
  );
  // Catalogue/notes report a lost scope from inside Body, which the reload remounts: guard once per page so a read that stays
  // forbidden while the detail is still allowed cannot reload forever.
  const scopeLost = useRef(false);
  const onScopeLost = () => { if (scopeLost.current) return; scopeLost.current = true; read.reload(); };
  const failure =
    read.status === "signed-out" ? c.signedOut
    : read.status === "forbidden" ? c.forbidden
    : read.status === "not-found" ? (store ? c.notFound : c.noStore)
    : read.status === "unavailable" ? c.unavailable
    : "";
  const back = `/${locale}/customers${store ? `?store=${store.id}${q ? `&q=${encodeURIComponent(q)}` : ""}${tag ? `&tag=${encodeURIComponent(tag)}` : ""}` : ""}`;
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="customers">
      <div className="orders-page customers-page people-v2" data-testid="customer-detail">
        <AdminPageHeader locale={locale} title={c.people.title} actions={<Link className="customers-back" href={back}>{c.people.back}</Link>} />
        {(read.status === "loading" || read.status === "hidden") && (
          <p className="orders-message" role="status">{c.detailLoading}</p>
        )}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>{c.retry}</button>
          </div>
        )}
        {read.status === "ready" && read.data && store && (
          <Body key={`${store.id}|${customerID}|${read.boundary}`} detail={read.data} storeInfo={store} store={store.id} boundary={read.boundary} refresh={read.refresh} onScopeLost={onScopeLost} locale={locale} c={c} />
        )}
      </div>
    </WorkspaceFrame>
  );
}

type Pending = {
  kind: string;
  key: string;
  run: (key: string) => Promise<WriteOutcome<unknown>>;
  done: string;
};

function Body({
  detail,
  storeInfo,
  store,
  boundary,
  refresh,
  onScopeLost,
  locale,
  c,
}: {
  detail: Detail;
  storeInfo: Store;
  store: string;
  boundary: string;
  refresh: () => Promise<boolean>;
  onScopeLost: () => void;
  locale: Locale;
  c: CustomersCopy;
}) {
  const oc = ordersCopy[locale];
  const m = (value: number) => (detail.currency ? money(locale, detail.currency, value) : String(value));
  const date = (value: string) => displayTime("sv-SE", value).slice(0, 10);
  const [busy, setBusy] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const p = c.people;
  const [exportOpen, setExportOpen] = useState(false);
  const [pendingKind, setPendingKind] = useState("");
  const [storageReady, setStorageReady] = useState(false);
  const [screen, setScreen] = useState<"profile" | "submitted" | "pending">("profile");
  const markerKey = useRef("");
  const writing = useRef(false);
  // Store only a command kind under an opaque session/store/customer digest. No PII, payload or key is cached.
  // A new authenticated boundary cannot inherit another merchant's marker. Reads still come from Go.
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(`${boundary}|${store}|${detail.customer_id}`));
        const key = `people-v2:${Array.from(new Uint8Array(digest), (v) => v.toString(16).padStart(2, "0")).join("")}`;
        const saved = sessionStorage.getItem(key);
        if (cancelled) return;
        markerKey.current = key;
        if (saved) setPendingKind(saved === "export" || saved === "erase" ? saved : "unknown");
        setStorageReady(true);
      } catch {
        // Fail closed when a reload-safe fence cannot be kept; never send an untrackable command.
        if (!cancelled) { setPendingKind("unknown"); setStorageReady(true); }
      }
    })();
    return () => { cancelled = true; };
  }, [boundary, store, detail.customer_id]);
  // CONT-289 RED-only negative control: ignore the unresolved-command fence. Restore before GREEN.
  const locked = !storageReady;
  function mark(kind: string) {
    if (!markerKey.current) return false;
    try { sessionStorage.setItem(markerKey.current, kind); setPendingKind(kind); return true; }
    catch { setPendingKind("unknown"); return false; }
  }
  function clearMarker() {
    try { sessionStorage.removeItem(markerKey.current); setPendingKind(""); }
    catch { setPendingKind("unknown"); }
  }
  async function attempt(next: Pending) {
    if (writing.current || locked) return;
    const privacy = next.kind === "export";
    if (privacy && !mark(next.kind)) return;
    writing.current = true;
    setBusy(next.kind);
    setNotice(null);
    const result = await next.run(next.key);
    writing.current = false;
    setBusy("");
    if (result.ok) {
      if (privacy) { clearMarker(); setExportOpen(false); }
      setNotice({ tone: "ok", text: next.done });
      void refresh();
    } else if (result.uncertain) {
      if (!privacy) mark("unknown");
      setExportOpen(false);
      setScreen("submitted");
    } else {
      if (privacy) clearMarker();
      setNotice({ tone: "bad", text: errorText(c, result.code) });
    }
  }
  const withdraw = (purpose: string, channel: string) =>
    attempt({ kind: `withdraw:${purpose}`, key: newKey("withdraw"), done: c.withdrawDone,
      run: (key) => postWithdrawal(store, detail.customer_id, key, JSON.stringify({ purpose, channel }), boundary) });
  const download = () => attempt({ kind: "export", key: newKey("export"), done: c.downloadDone,
    run: (key) => postExport(store, detail.customer_id, key, boundary) });
  const identity = <p className="people-identity">{storeInfo.name} · {detail.display_name ?? c.noName}<br />
    {p.customerID} {detail.customer_id.slice(0, 8)}{detail.phone_last3 && <> · {p.phone} {detail.phone_last3}</>}</p>;
  if (screen !== "profile") return <section className="customers-section people-result" role="status">
    <h2>{pendingKind === "unknown" ? p.unconfirmed : screen === "submitted" ? (pendingKind === "erase" ? p.eraseSubmitted : p.exportSubmitted) : (pendingKind === "erase" ? p.erasePending : p.exportPending)}</h2>
    {identity}
    <p>{screen === "submitted" ? p.unconfirmed : p.paused}</p>
    <button type="button" onClick={() => setScreen(screen === "submitted" ? "pending" : "profile")}>
      {screen === "submitted" ? p.pending : p.viewData}
    </button>
  </section>;

  const label = (purpose: string) => (purpose === "marketing_messages" ? p.marketing : p.ads);
  const source = (value: string) => value === "buyer_settings" ? p.settings : c.sources[value] ?? (value === "merchant_recorded" ? c.sources.merchant : "—");
  return (
    <>
      <nav className="people-tabs" aria-label={p.title}>
        <a href="#people-orders">{p.orders}</a><a href="#people-consent">{p.consent}</a><a href="#people-privacy">{p.privacy}</a>
      </nav>
      {pendingKind && <section className="customers-section people-pending"><p>{p.paused}</p>{identity}
        <button type="button" onClick={() => setScreen("pending")}>{p.pendingBack}</button></section>}
      <div className="people-profile">
      <aside className="people-side">
      {detail.active && !!detail.tags?.length && <p className="customers-section">{detail.tags.map((tag) => tag.name).join(" · ")}</p>}
      {detail.active && !!detail.notes?.length && <p className="customers-section">{detail.notes[0].body}</p>}
      <details className="people-advanced" data-testid="people-customer-tools"><summary>{p.advanced}</summary>{detail.active && <CustomerTags key={`${store}|${detail.customer_id}|${boundary}`} locale={locale} store={storeInfo}
        detail={detail} boundary={boundary} onChanged={refresh} onScopeLost={onScopeLost} />}
        <h3>{c.consentHistory}</h3>
        <p className="orders-hint">{c.withdrawHint}</p>
        {detail.consent_history.length === 0 ? (
          <p className="orders-empty">{c.consentHistoryNone}</p>
        ) : (
          <TableFrame label={c.consentSection} scrollHint={presentationCopy[locale].scroll} scrollClassName="orders-actions-scroll">
            <table className="orders-actions-table" data-testid="consent-history">
              <thead>
                <tr><th>{c.when}</th><th>{c.consentSection}</th><th>{c.source}</th><th>{c.policy}</th></tr>
              </thead>
              <tbody>
                {detail.consent_history.map((event, index) => (
                  <tr key={`${event.occurred_at}-${index}`}>
                    <td>{displayTime(locale, event.occurred_at)}</td>
                    <td>
                      {label(event.purpose)}:{" "}
                      <Badge tone={event.granted ? "success" : "neutral"}>
                        {event.granted ? p.granted : p.withdrawn}
                      </Badge>
                    </td>
                    <td>{source(event.source)}</td>
                    <td>{event.policy_version}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </TableFrame>
        )}

        <h3>{c.actionsSection}</h3>
        {detail.privacy_actions.length === 0 ? (
          <p className="orders-empty">{c.actionsNone}</p>
        ) : (
          <TableFrame label={c.privacySection} scrollHint={presentationCopy[locale].scroll} scrollClassName="orders-actions-scroll">
            <table className="orders-actions-table" data-testid="privacy-actions">
              <thead><tr><th>{c.when}</th><th>{c.actionCol}</th><th>{c.source}</th></tr></thead>
              <tbody>
                {detail.privacy_actions.map((action, index) => (
                  <tr key={`${action.completed_at}-${index}`}>
                    <td>{displayTime(locale, action.completed_at)}</td>
                    <td>{c.kinds[action.kind.toLowerCase()] ?? action.kind}</td>
                    <td>{c.via[action.via] ?? action.via}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </TableFrame>
        )}

      </details>
      {detail.active && detail.imported && <>
        <Badge tone="neutral">{c.imported}</Badge>
        <CustomerHistoricalOrders key={`history|${store}|${detail.customer_id}|${boundary}`} locale={locale}
          store={store} customer={detail.customer_id} boundary={boundary} />
      </>}
      <section id="people-privacy" className="customers-section people-privacy" aria-label={p.privacy} data-testid="customer-privacy">
        <h2>{c.privacySection}</h2>

        <div className="customers-actions">
          {/* CONT-289 RED-only negative control: bypass export review. Restore before GREEN. */}
          <button type="button" data-testid="customer-download" disabled={busy !== "" || locked} onClick={() => void download()}>
            {busy === "export" ? c.downloading : p.download}
          </button>
          {detail.active && <EraseDialog detail={detail} store={store} boundary={boundary} c={c} busy={busy !== "" || locked} identity={identity} beforeSend={() => mark("erase")} clearMarker={clearMarker}
            onUncertain={() => setScreen("submitted")} onDone={(text) => { setNotice({ tone: "ok", text }); void refresh(); }} />}
        </div>
        {notice && (
          <p className={notice.tone === "ok" ? "orders-notice" : "orders-bad"} role={notice.tone === "ok" ? "status" : "alert"}
            data-testid="customer-notice">
            {notice.text}
          </p>
        )}

        {exportOpen && <ExportDialog c={c} identity={identity} orders={detail.orders_count} busy={busy !== "" || locked}
          onCancel={() => { if (!writing.current) setExportOpen(false); }} onConfirm={() => void download()} />}
      </section>
      </aside>
      <div className="people-primary">
      {!detail.active && <p className="customers-erased" role="status" data-testid="customer-erased">{c.erasedState}</p>}
      <section className="customers-section people-facts" aria-label={c.facts}>
        <h2>{detail.display_name ?? c.noName}</h2>
        <p className="people-identity">{storeInfo.name} · {p.customerID} {detail.customer_id.slice(0, 8)}{detail.phone_last3 && <> · {p.phone} {detail.phone_last3}</>}</p>
        <p>{detail.orders_count} {p.orders} · {p.net} {detail.currency ? m(detail.captured_minor - detail.refunded_minor) : "—"}<br />
          {p.first} {detail.first_seen_at ? date(detail.first_seen_at) : "—"}</p>
      </section>

      <section id="people-orders" className="customers-section" aria-label={c.ordersSection}>
        <h2 className="people-sr-only">{p.orders}</h2>
        {detail.orders.length === 0 ? (
          <p className="orders-empty">{c.ordersNone}</p>
        ) : (
          <TableFrame label={c.ordersSection} scrollHint={presentationCopy[locale].scroll} scrollClassName="orders-actions-scroll">
            <table className="orders-actions-table" data-testid="customer-orders">
              <thead>
                <tr><th>{c.orderId} · {c.total}</th></tr>
              </thead>
              <tbody>
                {detail.orders.map((order) => (
                  <tr key={order.order_id} data-testid={`customer-order-${order.order_id}`}>
                    <td>
                      <Link href={`/${locale}/orders?store=${store}&order=${order.order_id}`}
                        aria-label={`${c.orderOpen}: ${order.order_id}`}>
                        <span className="orders-mono">{order.order_id.slice(0, 4)}…{order.order_id.slice(-4)}</span>
                      </Link> · {money(locale, order.currency, order.total_minor)}<br />
                      {oc.statuses[order.payment_state]} · {oc.statuses[order.commercial_state]} · {date(order.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </TableFrame>
        )}

      </section>

      {detail.claims.length > 0 && <section className="customers-section" aria-label={c.claimsSection}>
        <h2>{c.claimsSection}</h2>
        {detail.claims.length === 0 ? (
          <p className="orders-empty">{c.claimsNone}</p>
        ) : (
          <TableFrame label={c.claimsSection} scrollHint={presentationCopy[locale].scroll} scrollClassName="orders-actions-scroll">
            <table className="orders-actions-table" data-testid="customer-claims">
              <thead><tr><th>{c.platform}</th><th>{c.boundAt}</th><th>{c.lines}</th></tr></thead>
              <tbody>
                {detail.claims.map((claim) => (
                  <tr key={`${claim.session_id}-${claim.bound_at}`}>
                    <td>{c.platforms[claim.platform] ?? claim.platform}</td>
                    <td>{displayTime(locale, claim.bound_at)}</td>
                    <td>{claim.line_count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </TableFrame>
        )}
      </section>}

      <section id="people-consent" className="customers-section" aria-label={p.consent} data-testid="customer-consent">
        <h2 className="people-sr-only">{p.consent}</h2>
        <ul className="customers-consent">
          {consentPairs.map(({ purpose, channel }) => {
            const on = detail.consents[purpose];
            return (
              <li key={purpose} data-testid={`consent-${purpose}`}>
                <span>{label(purpose)}{detail.consent_history.find((event) => event.purpose === purpose) && <small className="people-event">
                  {(() => { const event = detail.consent_history.find((item) => item.purpose === purpose)!; return `${source(event.source)} · ${date(event.occurred_at)}`; })()}
                </small>}</span>
                <Badge tone={on ? "success" : "neutral"}>
                  {on ? p.granted : detail.consent_history.some((event) => event.purpose === purpose && !event.granted) ? p.withdrawn : c.consentNone}
                </Badge>
                {on && detail.active && (
                  <button type="button" data-testid={`withdraw-${purpose}`} disabled={busy !== "" || locked}
                    onClick={() => void withdraw(purpose, channel)}>
                    {busy === `withdraw:${purpose}` ? c.withdrawing : c.withdraw}
                  </button>
                )}
              </li>
            );
          })}
        </ul>

      </section>

      </div>

      </div>
    </>
  );
}

// Native dialogs preserve keyboard dismissal and focus; only the signed name + scope checkbox enables erasure.
function EraseDialog({ detail, store, boundary, c, busy: otherBusy, identity, beforeSend, clearMarker, onUncertain, onDone }: {
  detail: Detail; store: string; boundary: string; c: CustomersCopy; busy: boolean; identity: ReactNode;
  beforeSend: () => boolean; clearMarker: () => void; onUncertain: () => void; onDone: (text: string) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const typedInput = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [typed, setTyped] = useState("");
  const [checked, setChecked] = useState(false);
  const [problem, setProblem] = useState("");
  const [busy, setBusy] = useState(false);
  const writing = useRef(false);
  const key = useRef("");
  const p = c.people;
  const name = detail.display_name ?? c.noName;
  useEffect(() => {
    const element = dialog.current;
    if (!element) return;
    if (open && !element.open) element.showModal();
    if (!open && element.open) element.close();
    if (open) typedInput.current?.focus();
  }, [open]);
  function begin() {
    key.current = newKey("erase");
    setTyped(""); setChecked(false); setProblem(""); setOpen(true);
  }
  async function submit() {
    // CONT-289 RED-only negative control: omit scope acknowledgement. Restore before GREEN.
    if (typed !== name || writing.current || otherBusy || !beforeSend()) return;
    writing.current = true; setBusy(true); setProblem("");
    const result = await postErasure(store, detail.customer_id, key.current, boundary);
    writing.current = false; setBusy(false);
    if (result.ok) {
      clearMarker(); setOpen(false);
      const s = result.value;
      onDone(c.erasedDone({ consents: s.consents_withdrawn, sessions: s.sessions_revoked, snapshots: s.snapshots_redacted, bundles: s.bundles_relabelled }));
    } else if (result.uncertain) {
      setOpen(false); onUncertain();
    } else {
      clearMarker(); key.current = newKey("erase"); setProblem(errorText(c, result.code));
    }
  }
  return <>
    <button type="button" className="danger" data-testid="customer-erase" disabled={otherBusy} onClick={begin}>{p.erase}</button>
    <dialog ref={dialog} className="orders-dialog people-dialog" aria-labelledby="erase-title" data-testid="erase-dialog"
      onCancel={(event) => { if (writing.current) event.preventDefault(); }} onClose={() => setOpen(false)}>
      {open && <form onSubmit={(event) => { event.preventDefault(); void submit(); }}>
        <h2 id="erase-title">{p.erase}</h2>
        {identity}
        <p>{p.eraseBody}</p>
        <p data-testid="erase-kept">{p.eraseKept(detail.orders_count)}</p>
        <p className="orders-bad">{p.eraseSafety}</p>
        <label>{p.typeName(name)}<input ref={typedInput} data-testid="erase-typed" autoComplete="off" autoCapitalize="off"
          spellCheck={false} value={typed} disabled={busy} onChange={(event) => setTyped(event.target.value)} /></label>
        <label className="people-check"><input type="checkbox" checked={checked} disabled={busy} onChange={(event) => setChecked(event.target.checked)} />{p.checked}</label>
        {problem && <p className="orders-bad" role="alert" data-testid="erase-problem">{problem}</p>}
        <div className="orders-dialog-actions">
          <button type="button" disabled={busy} onClick={() => setOpen(false)}>{p.backCustomer}</button>
          <button type="submit" className="primary danger" data-testid="erase-submit" disabled={typed !== name || busy || otherBusy}>
            {busy ? c.erasing : p.confirmErase}
          </button>
        </div>
      </form>}
    </dialog>
  </>;
}

function ExportDialog({ c, identity, orders, busy, onCancel, onConfirm }: {
  c: CustomersCopy; identity: ReactNode; orders: number; busy: boolean; onCancel: () => void; onConfirm: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => { dialog.current?.showModal(); }, []);
  const p = c.people;
  return <dialog ref={dialog} className="orders-dialog people-dialog" aria-labelledby="export-title"
    onCancel={(event) => { if (busy) event.preventDefault(); }} onClose={onCancel}>
    <h2 id="export-title">{p.download}</h2>
    {identity}
    <p>{p.verify}</p><p>{p.scope(orders)}</p><p>{p.protected}</p><p>{p.permissions}</p>
    <div className="orders-dialog-actions">
      <button type="button" disabled={busy} onClick={onCancel}>{p.backCustomer}</button>
      <button type="button" className="primary" disabled={busy} onClick={onConfirm}>{busy ? c.downloading : p.confirmDownload}</button>
    </div>
  </dialog>;
}
