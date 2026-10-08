// Purpose: render scoped A13 buyer facts and explicit A14 customer links using the independent A9 CAS version.
// Depends on: React, next/navigation, @live-commerce/format, inbox/customers clients, session boundaries and InboxFence.
// Used by: Inbox and the later live console; names, choices and retry receipts remain only in visible authorised memory.
// BFF /api/stores/{store}/inbox/{buyer-panel,conversations/{id}/{messages,customer-link}} -> httpapi/inbox.go; customers[/id] -> httpapi/customers.go.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import type { Store } from "@/lib/model";
import { displayTime, money } from "@live-commerce/format";
import type { BuyerData, Thread } from "@/lib/inbox-types";
import { inboxRead, inboxWrite, InboxError } from "@/lib/inbox-client";
import { readCustomer, readCustomers, ReadError } from "@/lib/customers-client";
import { csrfCookie, sessionBoundary } from "@/lib/settings-client";
import type { Customer } from "@/lib/customers-model";
import { inboxCopy, inboxError } from "@/src/features/messages/copy";
import { permitted, ReplyReceipt, type InboxFence } from "@/src/features/messages/privacy";
import { useInboxPrivacy } from "@/src/features/messages/use-privacy";
import styles from "@/src/features/messages/Inbox.module.css";

type Choice = Pick<Customer, "customer_id" | "display_name" | "phone_last3">;
type Ticket = ReturnType<InboxFence["begin"]>;
const choiceOf = ({ customer_id, display_name, phone_last3 }: Customer): Choice => ({ customer_id, display_name, phone_last3 });
const versionOf = (header: Thread) => typeof header.link_version === "number" && Number.isSafeInteger(header.link_version) && header.link_version >= 0 ? header.link_version : null;
// Reuse the existing customer API and validators; add the inbox's post-parse session fence without new BFF paths.
async function customerRead<T>(read: () => Promise<T>): Promise<T> {
  try {
    const csrf = csrfCookie();
    let boundary: string;
    // sessionBoundary throws a plain Error on cookie change: classify both checks as authority loss.
    try { boundary = await sessionBoundary(csrf); } catch { throw new InboxError("unauthorized", 401); }
    const value = await read();
    try { if (await sessionBoundary(csrf) !== boundary) throw new Error(); }
    catch { throw new InboxError("unauthorized", 401); }
    return value;
  } catch (error) {
    if (error instanceof InboxError) throw error;
    if (error instanceof ReadError) {
      const codes = { "signed-out": ["unauthorized", 401], forbidden: ["forbidden", 403], "not-found": ["not_found", 404], unavailable: ["unavailable", 503] } as const;
      const [code, status] = codes[error.code]; throw new InboxError(code, status);
    }
    throw new InboxError("unavailable", 503);
  }
}

/** Fetch one conversation or bundle; linking never infers identity from names, claim owners or purchase history. */
export function BuyerPanel({ store, conversationId, bundleId, onUnauthorized, onOrder, onLinked }: {
  store: Store; conversationId?: string; bundleId?: string; onUnauthorized?: () => void;
  onOrder?: (orderId: string) => void; onLinked?: () => void;
}) {
  const params = useParams<{ locale: string }>(), locale = params?.locale ?? "zh-TW", c = inboxCopy(locale);
  const scope = `${store.id}:${conversationId ?? ""}:${bundleId ?? ""}`;
  const canReadCustomers = permitted(store, "customers:read");
  const canLink = !!conversationId && permitted(store, "inbox:reply") && canReadCustomers;
  const [value, setValue] = useState<{ scope: string; data: BuyerData } | null>(null);
  const [version, setVersion] = useState<number | null>(null);
  const [linkedName, setLinkedName] = useState<string | null>(null);
  const [choices, setChoices] = useState<Choice[]>([]);
  const [customer, setCustomer] = useState<Choice | null>(null);
  const [after, setAfter] = useState("");
  const [next, setNext] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const pending = useRef(new ReplyReceipt());
  const inFlight = useRef(false), goneScope = useRef<string | null>(null);
  const callbacks = useRef({ onUnauthorized, onLinked }); callbacks.current = { onUnauthorized, onLinked };
  const clear = useCallback(() => {
    setValue(null); setVersion(null); setLinkedName(null); setChoices([]); setCustomer(null);
    setAfter(""); setNext(""); setError(null); setBusy(false); setUncertain(false);
    pending.current.clear(); inFlight.current = false;
  }, []);
  const privacy = useInboxPrivacy(clear);
  const data = value?.scope === scope && privacy.visible ? value.data : null;
  const fail = useCallback((cause: unknown, ticket: Ticket) => {
    if (!privacy.fence.current(ticket)) return;
    const problem = cause instanceof InboxError ? cause : new InboxError("unavailable", 503);
    if (problem.status === 404) {
      goneScope.current = scope;
      privacy.fence.invalidate(true); clear();
    } else if ([401, 403].includes(problem.status)) {
      privacy.expire(); callbacks.current.onUnauthorized?.();
    }
    setError(problem.code);
  }, [scope, privacy.fence, privacy.expire, clear]);
  const facts = useCallback(async (ticket: Ticket) => {
    const query = conversationId ? `conversation_id=${encodeURIComponent(conversationId)}` : `bundle_id=${encodeURIComponent(bundleId!)}`;
    const [result, header] = await Promise.all([
      inboxRead<BuyerData>(store.id, `inbox/buyer-panel?${query}`, ticket.signal),
      conversationId ? inboxRead<Thread>(store.id, `inbox/conversations/${conversationId}/messages?limit=1`, ticket.signal) : null,
    ]);
    if (!privacy.fence.current(ticket)) return;
    // Missing orders is an authoritative withheld projection, not an empty list.
    if (!result || !Array.isArray(result.claims) || (result.orders !== undefined && !Array.isArray(result.orders)))
      throw new InboxError("unavailable", 503);
    let name: string | null = null;
    if (result.linked_customer_id && canReadCustomers) {
      const current = await customerRead(() => readCustomer(store.id, result.linked_customer_id!, ticket.signal));
      name = current.display_name;
    }
    if (!privacy.fence.current(ticket)) return;
    setValue({ scope, data: result }); setVersion(header ? versionOf(header) : null); setLinkedName(name);
  }, [scope, store.id, conversationId, bundleId, canReadCustomers, privacy.fence]);
  const customerPage = useCallback(async (cursor: string, ticket: Ticket) => {
    // Pick from bounded existing pages. No merchant-entered name or phone is put in a query/URL.
    const result = await customerRead(() => readCustomers(store.id, "", cursor, ticket.signal));
    if (!privacy.fence.current(ticket)) return;
    setChoices(result.items.filter(row => row.active).map(choiceOf)); setAfter(cursor); setNext(result.next_cursor);
  }, [store.id, privacy.fence]);
  const reload = useCallback(async () => {
    if (inFlight.current || !privacy.visible || privacy.blocked.current || goneScope.current === scope) return;
    inFlight.current = true; setBusy(true); setError(null); setVersion(null);
    const ticket = privacy.fence.begin();
    try { await Promise.all([facts(ticket), canLink ? customerPage("", ticket) : null]); }
    catch (cause) { fail(cause, ticket); }
    finally { if (privacy.fence.current(ticket)) { inFlight.current = false; setBusy(false); } }
  }, [scope, privacy.visible, privacy.blocked, privacy.fence, facts, customerPage, canLink, fail]);
  useEffect(() => {
    clear();
    if (!privacy.visible || privacy.blocked.current || !permitted(store, "inbox:read") || !!conversationId === !!bundleId || goneScope.current === scope) return;
    privacy.fence.invalidate(true); void reload();
    return () => privacy.fence.invalidate(false);
  }, [store, scope, conversationId, bundleId, privacy.visible, privacy.revision, privacy.blocked, privacy.fence, clear, reload]);

  async function pageCustomers(cursor: string) {
    if (!canLink || inFlight.current || pending.current.pending()) return;
    inFlight.current = true; setBusy(true); const ticket = privacy.fence.begin();
    try { await customerPage(cursor, ticket); }
    catch (cause) { fail(cause, ticket); }
    finally { if (privacy.fence.current(ticket)) { inFlight.current = false; setBusy(false); } }
  }
  async function link(unlink = false, retry = false) {
    if (!canLink || inFlight.current || !data || goneScope.current === scope || privacy.blocked.current) return;
    if (retry ? !pending.current.pending() : pending.current.pending() || version === null || (!unlink && !customer)) return;
    const command = retry ? pending.current.pending()! : pending.current.prepare({ customer_id: unlink ? null : customer!.customer_id, expected_version: version });
    const ticket = privacy.fence.begin(); inFlight.current = true; setBusy(true); setError(null);
    try {
      // A14 uses only the independent A9 CAS version. An uncertain retry retains its original body and key.
      const receipt = await inboxWrite<{ customer_id: string | null; version: number }>(store.id,
        `inbox/conversations/${conversationId}/customer-link`, command.body, command.key, ticket.signal);
      if (!privacy.fence.current(ticket)) return;
      if (receipt.customer_id !== command.body.customer_id || !Number.isSafeInteger(receipt.version) || receipt.version <= Number(command.body.expected_version))
        throw new InboxError("retry_later", 503);
      pending.current.clear(); setUncertain(false); setCustomer(null); setVersion(null);
      await facts(ticket);
      if (privacy.fence.current(ticket)) callbacks.current.onLinked?.();
    } catch (cause) {
      if (!privacy.fence.current(ticket)) return;
      const code = cause instanceof InboxError ? cause.code : "unavailable";
      pending.current.failed(code); setUncertain(pending.current.pending() !== null);
      fail(cause, ticket);
      if (code === "version_conflict") {
        // Refresh the winning link AND its A9 version. Otherwise unlink could target a new customer
        // while the panel still describes the old one. The merchant's selection remains untouched.
        setVersion(null);
        try { await facts(ticket); } catch (refreshError) { fail(refreshError, ticket); }
      }
    } finally { if (privacy.fence.current(ticket)) { inFlight.current = false; setBusy(false); } }
  }
  const options = customer && !choices.some(row => row.customer_id === customer.customer_id) ? [customer, ...choices] : choices;
  return <aside className={styles.panel} data-testid="buyer-panel" aria-busy={busy}>
    <h2>{c.buyer}</h2>
    {!privacy.visible && <p className={styles.notice}>{privacy.blocked.current ? c.signedOut : c.hidden}</p>}
    {busy && !data && <p>{c.loading}</p>}
    {error && <p role="alert" className={`${styles.notice} ${styles.error}`}>{inboxError(locale, error)}</p>}
    {error && !busy && !uncertain && goneScope.current !== scope && privacy.visible && <button className={styles.button} onClick={() => void reload()}>{c.refresh}</button>}
    {data && <>
      <dl><dt>{c.buyer}</dt><dd>{data.display_name ?? c.unnamed}</dd>
        <dt>{c.window}</dt><dd>{data.platform}{data.window_open_until && <> · {displayTime(locale, data.window_open_until)}</>}</dd>
        <dt>{c.ordinal}</dt><dd>{c.ordinalValue.replace("{n}", String(data.purchase_ordinal))}</dd></dl>
      {data.link_pending_manual && <p className={styles.notice}>{c.manual}</p>}
      {data.auto_reply && <p className={styles.badge}>{c.autoReply}: {c[data.auto_reply.send_state as "queued"] ?? c.unknown}</p>}
      <h3>{c.claims}</h3>
      {data.claims.length ? <ul>{data.claims.map((claim, index) => <li key={`${claim.session_id}:${claim.offer_id}:${index}`}>
        {claim.keyword} × {claim.quantity}<br /><span className={styles.meta}>{c.claimSession}: {claim.session_id}</span>
      </li>)}</ul> : <p className={styles.meta}>{c.noClaims}</p>}
      <p>{c.total}: {money(locale, "TWD", data.claim_total_minor)}</p>
      <h3>{c.orders}</h3>
      {!permitted(store, "orders:read") ? <p className={styles.meta}>{c.noOrdersPermission}</p> : data.orders === undefined ?
        <p className={styles.meta}>{c.ordersWithheld}</p> : data.orders.length ? <ul>{data.orders.map(order => <li key={order.order_id}>
          {onOrder ? <button className={styles.button} onClick={() => onOrder(order.order_id)}>{order.number}</button> : order.number}
          {" "}· {order.state} · {money(locale, store.currency, order.total_minor)}
        </li>)}</ul> : <p className={styles.meta}>{c.noOrders}</p>}
      {conversationId && <>
      <h3>{c.customer}</h3>
      {data.linked_customer_id && <p className={styles.meta} data-testid="customer-manual-link">{c.linked}{canReadCustomers ? ` · ${linkedName ?? c.unnamed}` : ""}</p>}
      {canLink && <>
        <label>{c.customerChoice}<select data-testid="customer-link-choice" value={customer?.customer_id ?? ""} disabled={busy || version === null || uncertain}
          onChange={event => setCustomer(options.find(row => row.customer_id === event.target.value) ?? null)}>
          <option value="">{c.customerChoice}</option>{options.map(row => <option key={row.customer_id} value={row.customer_id}>
            {row.display_name ?? c.unnamed}{row.phone_last3 ? ` · …${row.phone_last3}` : ""}
          </option>)}
        </select></label>
        {!choices.length && !busy && <p className={styles.meta}>{c.customerEmpty}</p>}
        <div className={styles.actions}>
          {after && <button className={styles.button} disabled={busy || uncertain} onClick={() => void pageCustomers("")}>{c.customerFirst}</button>}
          {next && <button className={styles.button} disabled={busy || uncertain} onClick={() => void pageCustomers(next)}>{c.customerNext}</button>}
        </div>
      </>}
      <div className={styles.actions}>
        <button className={styles.button} data-testid="customer-link-save" disabled={!canLink || version === null || busy || uncertain || !customer} onClick={() => void link()}>{c.link}</button>
        <button className={styles.button} data-testid="customer-link-remove" disabled={!canLink || version === null || busy || uncertain || !data.linked_customer_id} onClick={() => void link(true)}>{c.unlink}</button>
        {uncertain && <button className={styles.button} data-testid="customer-link-retry" disabled={busy} onClick={() => void link(false, true)}>{c.customerRetry}</button>}
      </div>
      {!canLink && <p className={styles.notice}>{c.customerPermission}</p>}
      {canLink && version === null && <p className={styles.notice}>{c.versionMissing}</p>}
      {uncertain && <p role="status" className={styles.notice}>{c.customerUncertain}</p>}
      </>}
    </>}
  </aside>;
}
