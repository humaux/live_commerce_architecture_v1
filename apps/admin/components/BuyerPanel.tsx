// Purpose: independently fetch and render one scoped buyer panel with permission-gated orders/linking.
// Depends on: React, next/navigation, @live-commerce/format, frozen A13/A14 and private request/session lifetime.
// Used by: Inbox now and the later live console; no dependency on console or inbox selection state.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import type { Store } from "@/lib/model";
import { displayTime, money } from "@live-commerce/format";
import type { BuyerData } from "@/lib/inbox-types";
import { inboxRead, inboxWrite, InboxError } from "@/lib/inbox-client";
import { inboxCopy, inboxError } from "@/src/features/messages/copy";
import { permitted } from "@/src/features/messages/privacy";
import { useInboxPrivacy } from "@/src/features/messages/use-privacy";
import { CreateOrderDrawer } from "./CreateOrderDrawer";
import { createOrderCopy } from "@/lib/create-order-copy";
import type { Locale } from "@live-commerce/i18n";
import styles from "@/src/features/messages/Inbox.module.css";

/** Fetch exactly one conversation or bundle; hidden/obsolete scopes discard all buyer fields and drafts. */
export function BuyerPanel({
  store,
  conversationId,
  bundleId,
  onUnauthorized,
  onOrder,
  onLinked,
}: {
  store: Store;
  conversationId?: string;
  bundleId?: string;
  onUnauthorized?: () => void;
  onOrder?: (orderId: string) => void;
  onLinked?: () => void;
}) {
  const params = useParams<{ locale: string }>(),
    locale = params?.locale ?? "zh-TW",
    c = inboxCopy(locale);
  const scope = `${store.id}:${conversationId ?? ""}:${bundleId ?? ""}`;
  const [value, setValue] = useState<{ scope: string; data: BuyerData } | null>(
    null,
  );
  const [error, setError] = useState<string | null>(null);
  const [customer, setCustomer] = useState("");
  const [busy, setBusy] = useState(false);
  const [revision, setRevision] = useState(0);
  const [orderOpen, setOrderOpen] = useState(false);
  const callbacks = useRef({ onUnauthorized, onLinked });
  callbacks.current = { onUnauthorized, onLinked };
  const clear = useCallback(() => {
    setValue(null);
    setError(null);
    setCustomer("");
    setBusy(false);
    setOrderOpen(false);
  }, []);
  const privacy = useInboxPrivacy(clear);
  const data = value?.scope === scope && privacy.visible ? value.data : null;
  useEffect(() => {
    clear();
    if (
      !privacy.visible ||
      privacy.blocked.current ||
      !permitted(store, "inbox:read") ||
      !!conversationId === !!bundleId
    )
      return;
    privacy.fence.invalidate(true);
    const ticket = privacy.fence.begin();
    setBusy(true);
    const query = conversationId
      ? `conversation_id=${encodeURIComponent(conversationId)}`
      : `bundle_id=${encodeURIComponent(bundleId!)}`;
    void inboxRead<BuyerData>(
      store.id,
      `inbox/buyer-panel?${query}`,
      ticket.signal,
    )
      .then((result) => {
        if (privacy.fence.current(ticket)) setValue({ scope, data: result });
      })
      .catch((cause) => {
        if (!privacy.fence.current(ticket)) return;
        if (cause instanceof InboxError && [401, 403].includes(cause.status)) {
          privacy.expire();
          callbacks.current.onUnauthorized?.();
        }
        setError(cause instanceof InboxError ? cause.code : "unavailable");
      })
      .finally(() => {
        if (privacy.fence.current(ticket)) setBusy(false);
      });
    return () => privacy.fence.invalidate(false);
  }, [
    store,
    scope,
    conversationId,
    bundleId,
    privacy.visible,
    privacy.revision,
    privacy.fence,
    privacy.blocked,
    privacy.expire,
    clear,
    revision,
  ]);
  // A13 supplies no version; wiring LC-B3b's A9 link_version here is a separate follow-up.
  // Never guess zero or derive it from a linked customer id.
  const version =
    data &&
    "version" in data &&
    Number.isSafeInteger(data.version) &&
    (data.version as number) >= 0
      ? (data.version as number)
      : null;
  const canLink =
    !!conversationId &&
    permitted(store, "inbox:reply") &&
    permitted(store, "customers:read");
  const link = async (unlink = false) => {
    if (
      !canLink ||
      version === null ||
      busy ||
      (!unlink &&
        !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(customer))
    )
      return;
    const ticket = privacy.fence.begin();
    setBusy(true);
    setError(null);
    try {
      // Calls A14 with a server-provided version only; no inference or automatic retry.
      await inboxWrite(
        store.id,
        `inbox/conversations/${conversationId}/customer-link`,
        { customer_id: unlink ? null : customer, expected_version: version },
        crypto.randomUUID(),
        ticket.signal,
      );
      if (!privacy.fence.current(ticket)) return;
      setCustomer("");
      setRevision((value) => value + 1);
      callbacks.current.onLinked?.();
    } catch (cause) {
      if (!privacy.fence.current(ticket)) return;
      const code = cause instanceof InboxError ? cause.code : "unavailable";
      if (cause instanceof InboxError && cause.status === 404) {
        privacy.fence.invalidate(true);
        clear();
      }
      setError(code);
      if (cause instanceof InboxError && [401, 403].includes(cause.status)) {
        privacy.expire();
        callbacks.current.onUnauthorized?.();
      }
      if (code === "version_conflict") setRevision((value) => value + 1);
    } finally {
      if (privacy.fence.current(ticket)) setBusy(false);
    }
  };
  return (
    <aside className={styles.panel} data-testid="buyer-panel" aria-busy={busy}>
      <h2>{c.buyer}</h2>
      {data && <button type="button" data-testid="buyer-create-order" disabled={!permitted(store, "inventory:reserve") || !permitted(store, "orders:read")} onClick={() => setOrderOpen(true)}>{createOrderCopy[locale as Locale].title}</button>}
      {data && (!permitted(store, "inventory:reserve") || !permitted(store, "orders:read")) && <p>{createOrderCopy[locale as Locale].permission}</p>}
      {orderOpen && data && <CreateOrderDrawer key={scope} locale={locale as Locale} store={store} conversationId={conversationId} bundleId={bundleId} onUnauthorized={privacy.expire} onClose={() => setOrderOpen(false)} />}
      {!privacy.visible && (
        <p className={styles.notice}>
          {privacy.blocked.current ? c.signedOut : c.hidden}
        </p>
      )}
      {busy && !data && <p>{c.loading}</p>}
      {error && (
        <p role="alert" className={`${styles.notice} ${styles.error}`}>
          {inboxError(locale, error)}
        </p>
      )}
      {data && (
        <>
          <dl>
            <dt>{c.buyer}</dt>
            <dd>{data.display_name ?? c.unnamed}</dd>
            <dt>{c.window}</dt>
            <dd>
              {data.platform}
              {data.window_open_until && (
                <> · {displayTime(locale, data.window_open_until)}</>
              )}
            </dd>
            <dt>{c.ordinal}</dt>
            <dd>
              {data.purchase_ordinal > 0 ? data.purchase_ordinal : c.noOrdinal}
            </dd>
          </dl>
          {data.link_pending_manual && (
            <p className={styles.notice}>{c.manual}</p>
          )}
          {data.auto_reply && (
            <p className={styles.badge}>
              {data.auto_reply.send_state === "unknown"
                ? c.uncertain
                : (c[data.auto_reply.send_state as "queued"] ?? c.unknown)}
            </p>
          )}
          <h3>{c.claims}</h3>
          {data.claims.length ? (
            <ul>
              {data.claims.map((claim, index) => (
                <li key={index}>
                  {claim.keyword} × {claim.quantity}
                </li>
              ))}
            </ul>
          ) : (
            <p className={styles.meta}>{c.noClaims}</p>
          )}
          {data.claims.length > 0 && (
            <p>
              {c.total}: {money(locale, store.currency, data.claim_total_minor)}
            </p>
          )}
          <h3>{c.orders}</h3>
          {permitted(store, "orders:read") && data.orders !== undefined ? (
            data.orders.length ? (
              <ul>
                {data.orders.map((order) => (
                  <li key={order.order_id}>
                    {onOrder ? (
                      <button
                        className={styles.button}
                        onClick={() => onOrder(order.order_id)}
                      >
                        {order.number}
                      </button>
                    ) : (
                      order.number
                    )}{" "}
                    · {order.state} ·{" "}
                    {money(locale, store.currency, order.total_minor)}
                  </li>
                ))}
              </ul>
            ) : (
              <p className={styles.meta}>{c.noOrders}</p>
            )
          ) : (
            <p className={styles.meta}>{c.noOrdersPermission}</p>
          )}
          <h3>{c.customer}</h3>
          {data.linked_customer_id && <p className={styles.meta}>{c.linked}</p>}
          {canLink && (
            <>
              <label>
                {c.customerId}
                <input
                  value={customer}
                  disabled={busy || version === null}
                  onChange={(event) => setCustomer(event.target.value)}
                  autoComplete="off"
                />
              </label>
              <div className={styles.actions}>
                <button
                  className={styles.button}
                  disabled={version === null || busy || !customer}
                  onClick={() => void link()}
                >
                  {c.link}
                </button>
                <button
                  className={styles.button}
                  disabled={
                    version === null || busy || !data.linked_customer_id
                  }
                  onClick={() => void link(true)}
                >
                  {c.unlink}
                </button>
              </div>
              {version === null && (
                <p className={styles.notice}>{c.versionMissing}</p>
              )}
            </>
          )}
        </>
      )}
    </aside>
  );
}
