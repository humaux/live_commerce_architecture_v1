// Purpose: independently fetch and render one scoped buyer panel with permission-gated orders/linking.
// Depends on: React, next/navigation, @live-commerce/format, A13/A14 and live-settings BFF blocklist check/add -> Go blocklist.go; private session lifetime.
// Used by: Inbox now and the later live console; no dependency on console or inbox selection state.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import type { Store } from "@/lib/model";
import { displayTime, money } from "@live-commerce/format";
import type { BuyerData } from "@/lib/inbox-types";
import {
  readLiveSettings,
  writeLiveSettings,
  LiveSettingsError,
} from "@/lib/live-settings-client";
import { liveSettingsCopy, liveSettingsError } from "@/lib/live-settings-copy";
import { settingsUUID, type SettingsReceipt } from "@/lib/live-settings-model";
import { inboxRead, inboxWrite, InboxError } from "@/lib/inbox-client";
import { inboxCopy, inboxError } from "@/src/features/messages/copy";
import { permitted } from "@/src/features/messages/privacy";
import { useInboxPrivacy } from "@/src/features/messages/use-privacy";
import styles from "@/src/features/messages/Inbox.module.css";

/** Fetch exactly one conversation or bundle; hidden/obsolete scopes discard all buyer fields and drafts. */
export function BuyerPanel({
  store,
  conversationId,
  bundleId,
  onUnauthorized,
  onOrder,
  onLinked,
  sessionId,
}: {
  store: Store;
  conversationId?: string;
  bundleId?: string;
  onUnauthorized?: () => void;
  onOrder?: (orderId: string) => void;
  onLinked?: () => void;
  sessionId?: string;
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
  const callbacks = useRef({ onUnauthorized, onLinked });
  callbacks.current = { onUnauthorized, onLinked };
  const clear = useCallback(() => {
    setValue(null);
    setError(null);
    setCustomer("");
    setBusy(false);
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
          {bundleId && (sessionId || data.claims[0]?.session_id) && (
            <BuyerRestriction
              key={`${store.id}:${bundleId}:${sessionId ?? data.claims[0]?.session_id}`}
              locale={locale}
              store={store}
              bundle={bundleId}
              session={sessionId ?? data.claims[0]!.session_id}
              onMissing={() => {
                privacy.fence.invalidate(true);
                clear();
                setError("not_found");
              }}
              onDenied={() => {
                privacy.expire();
                callbacks.current.onUnauthorized?.();
              }}
            />
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

/** Independently lifetime-bound BuyerPanel entry; never infers a social identity or persists the internal note. */
function BuyerRestriction({
  locale,
  store,
  session,
  bundle,
  onDenied,
  onMissing,
}: {
  locale: string;
  store: Store;
  session: string;
  bundle: string;
  onDenied?: () => void;
  onMissing?: () => void;
}) {
  const c = liveSettingsCopy(locale),
    [restricted, setRestricted] = useState<boolean | null>(null),
    [note, setNote] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [unknown, setUnknown] = useState(false);
  const pending = useRef<SettingsReceipt | null>(null),
    inFlight = useRef(false),
    callback = useRef(onDenied);
  callback.current = onDenied;
  const missing = useRef(onMissing);
  missing.current = onMissing;
  const clear = useCallback(() => {
    setRestricted(null);
    setNote("");
    setBusy(false);
    setError("");
    setUnknown(false);
    pending.current = null;
    inFlight.current = false;
  }, []);
  const privacy = useInboxPrivacy(clear),
    canRead = permitted(store, "live:read"),
    canManage = permitted(store, "live:manage"),
    valid = Array.from(note).length <= 200 && !/\p{Cc}/u.test(note);
  const fail = useCallback(
    (e: unknown) => {
      if (e instanceof LiveSettingsError && [401, 403].includes(e.status)) {
        privacy.expire();
        callback.current?.();
        return;
      }
      if (e instanceof LiveSettingsError && e.status === 404) {
        privacy.fence.invalidate(true);
        clear();
        missing.current?.();
      }
      setError(e instanceof LiveSettingsError ? e.code : "unavailable");
    },
    [privacy.expire, clear],
  );
  useEffect(() => {
    clear();
    if (
      !privacy.visible ||
      privacy.blocked.current ||
      !canRead ||
      !settingsUUID.test(session) ||
      !settingsUUID.test(bundle)
    )
      return;
    privacy.fence.invalidate(true);
    const ticket = privacy.fence.begin();
    void readLiveSettings<{ restricted: boolean }>(
      store.id,
      `live-sessions/${session}/claims/blocklist/check?bundle_id=${bundle}`,
      ticket.signal,
    )
      .then((v) => {
        if (privacy.fence.current(ticket)) setRestricted(v.restricted);
      })
      .catch((e) => {
        if (privacy.fence.current(ticket)) fail(e);
      });
    return () => privacy.fence.invalidate(false);
  }, [
    store.id,
    session,
    bundle,
    canRead,
    privacy.visible,
    privacy.revision,
    privacy.blocked,
    privacy.fence,
    clear,
    fail,
  ]);
  async function add() {
    if (
      !canManage ||
      !privacy.visible ||
      privacy.blocked.current ||
      inFlight.current ||
      (!pending.current && (!valid || restricted !== false))
    )
      return;
    const ticket = privacy.fence.begin();
    const action = pending.current ?? {
      key: crypto.randomUUID(),
      method: "POST" as const,
      resource: `live-sessions/${session}/claims/blocklist`,
      body: JSON.stringify({ bundle_id: bundle, ...(note ? { note } : {}) }),
    };
    pending.current = action;
    inFlight.current = true;
    setBusy(true);
    setUnknown(false);
    setError("");
    try {
      await writeLiveSettings(store.id, action, ticket.signal);
      if (privacy.fence.current(ticket)) {
        pending.current = null;
        setNote("");
        setRestricted(true);
      }
    } catch (e) {
      if (!privacy.fence.current(ticket)) return;
      if (e instanceof LiveSettingsError && e.status >= 500) setUnknown(true);
      else {
        pending.current = null;
        fail(e);
      }
    } finally {
      if (privacy.fence.current(ticket)) {
        inFlight.current = false;
        setBusy(false);
      }
    }
  }
  if (!privacy.visible) return null;
  return (
    <section className={styles.composer} aria-label={c.list}>
      {restricted === true ? (
        <strong data-testid="buyer-restricted">{c.blocked}</strong>
      ) : (
        <>
          <label>
            {c.note}
            <textarea
              data-testid="buyer-block-note"
              maxLength={400}
              value={note}
              disabled={busy || unknown || !canManage || restricted === null}
              onChange={(e) => setNote(e.target.value)}
              aria-invalid={!valid}
            />
          </label>
          {(!valid || error) && (
            <p role="alert" data-testid="buyer-block-error">
              {!valid ? c.noteLong : liveSettingsError(locale, error)}
            </p>
          )}
          <button
            type="button"
            className={styles.button}
            data-testid="buyer-block-add"
            disabled={
              busy || unknown || !valid || !canManage || restricted !== false
            }
            onClick={() => void add()}
          >
            {c.block}
          </button>
        </>
      )}
      {unknown && (
        <div role="alert">
          <p>{c.unknown}</p>
          <button
            type="button"
            className={styles.button}
            data-testid="buyer-block-retry"
            disabled={busy}
            onClick={() => void add()}
          >
            {c.retry}
          </button>
        </div>
      )}
      <p className="live-helper">{c.blockHint}</p>
    </section>
  );
}
