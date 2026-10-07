// Purpose: own private inbox list/filter/cursor selection with no buyer data in navigation or storage.
// Depends on: React/react-dom, @live-commerce/format, inbox-client/types, WorkspaceFrame and the request/privacy fences.
// Used by: /[locale]/messages; threads and BuyerPanel are cleared on hide or session changes.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { flushSync } from "react-dom";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { displayTime } from "@live-commerce/format";
import type { ConversationItem, ConversationList } from "@/lib/inbox-types";
import { inboxRead, InboxError } from "@/lib/inbox-client";
import { inboxCopy, inboxError } from "@/src/features/messages/copy";
import { permitted } from "@/src/features/messages/privacy";
import { useInboxPrivacy } from "@/src/features/messages/use-privacy";
import styles from "@/src/features/messages/Inbox.module.css";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { InboxThread } from "./InboxThread";
import { BuyerPanel } from "./BuyerPanel";

/** Render authorized inbox data in memory only; every asynchronous completion checks its epoch. */
export function Inbox({
  locale,
  store,
  initialError,
  calibration = false,
}: {
  locale: Locale;
  store: Store | null;
  initialError: string | null;
  calibration?: boolean;
}) {
  const c = inboxCopy(locale);
  const [filter, setFilter] = useState("all");
  const [page, setPage] = useState<ConversationList | null>(null);
  const [selected, setSelected] = useState<ConversationItem | null>(null);
  const [error, setError] = useState(initialError);
  const [busy, setBusy] = useState(false);
  const [reload, setReload] = useState(0);
  const loading = useRef(false);
  const clear = useCallback(() => {
    setPage(null);
    setSelected(null);
    setError(null);
    setBusy(false);
    loading.current = false;
  }, []);
  const privacy = useInboxPrivacy(clear, calibration);
  const allowed = permitted(store, "inbox:read");
  const load = useCallback(
    async (cursor = "") => {
      if (
        !store ||
        !allowed ||
        !privacy.visible ||
        privacy.blocked.current ||
        loading.current
      )
        return;
      loading.current = true;
      setBusy(true);
      setError(null);
      const ticket = privacy.fence.begin();
      const query = new URLSearchParams({ filter, limit: "50" });
      if (cursor) query.set("cursor", cursor);
      try {
        const data = await inboxRead<ConversationList>(
          store.id,
          `inbox/conversations?${query}`,
          ticket.signal,
        );
        if (!privacy.fence.current(ticket)) return;
        setPage((old) =>
          cursor && old
            ? {
                ...data,
                items: [
                  ...old.items,
                  ...data.items.filter(
                    (item) =>
                      !old.items.some(
                        (existing) =>
                          (existing.conversation_id ?? existing.bundle_id) ===
                          (item.conversation_id ?? item.bundle_id),
                      ),
                  ),
                ],
              }
            : data,
        );
      } catch (cause) {
        if (!privacy.fence.current(ticket)) return;
        const code = cause instanceof InboxError ? cause.code : "unavailable";
        if (
          cause instanceof InboxError &&
          [401, 403, 404].includes(cause.status)
        ) {
          privacy.expire();
          setError(code);
        } else setError(code);
      } finally {
        if (privacy.fence.current(ticket)) {
          loading.current = false;
          setBusy(false);
        }
      }
    },
    [
      store,
      allowed,
      filter,
      privacy.visible,
      privacy.fence,
      privacy.blocked,
      privacy.expire,
    ],
  );
  useEffect(() => {
    if (!privacy.visible || privacy.blocked.current) return;
    privacy.fence.invalidate(true);
    setPage(null);
    loading.current = false;
    void load();
    return () => privacy.fence.invalidate(false);
  }, [
    load,
    reload,
    privacy.revision,
    privacy.visible,
    privacy.fence,
    privacy.blocked,
  ]);
  const status = error ?? (privacy.blocked.current ? "unauthorized" : null);
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? ""}
      active="messages"
      onBeforeNavigate={() => {
        // Revoke and unmount private children before the shell changes store/locale/route.
        flushSync(privacy.suspend);
        return true;
      }}
    >
      <main className={styles.page} data-testid="inbox-page">
        <header className={styles.heading}>
          <div>
            <h1>{c.title}</h1>
            <p>{c.subtitle}</p>
          </div>
          {page && (
            <span className={styles.badge}>
              {c.unread}: {page.unread_total}
            </span>
          )}
        </header>
        {!store || !allowed ? (
          <p role="alert" className={styles.notice}>
            {status ? inboxError(locale, status) : c.forbidden}
          </p>
        ) : (
          <>
            <div className={styles.toolbar}>
              <label>
                {c.title}{" "}
                <select
                  data-testid="inbox-filter"
                  value={filter}
                  disabled={!privacy.visible || privacy.blocked.current}
                  onChange={(event) => {
                    if (event.target.value === filter) return;
                    privacy.fence.invalidate(true);
                    clear();
                    setFilter(event.target.value);
                  }}
                >
                  {[
                    "all",
                    "unreplied",
                    "messenger",
                    "instagram",
                    "live_comment",
                  ].map((value) => (
                    <option key={value} value={value}>
                      {c[value as "all"]}
                    </option>
                  ))}
                </select>
              </label>
              <button
                className={styles.button}
                disabled={busy || !privacy.visible || privacy.blocked.current}
                onClick={() => setReload((value) => value + 1)}
              >
                {c.refresh}
              </button>
            </div>
            {status && (
              <p role="alert" className={`${styles.notice} ${styles.error}`}>
                {inboxError(locale, status)}
              </p>
            )}
            {!privacy.visible ? (
              <p className={styles.notice}>
                {privacy.departing
                  ? c.loading
                  : privacy.blocked.current
                    ? c.signedOut
                    : c.hidden}
              </p>
            ) : (
              <div
                className={`${styles.layout} ${selected ? styles.selected : ""}`}
              >
                <ul
                  className={styles.list}
                  data-testid="inbox-list"
                  aria-label={c.title}
                  aria-busy={busy}
                >
                  {!page?.items.length && (
                    <li className={styles.empty}>
                      {busy ? c.loading : c.empty}
                    </li>
                  )}
                  {page?.items.map((item) => (
                    <li key={item.conversation_id ?? item.bundle_id}>
                      <button
                        className={styles.row}
                        data-testid={`conversation-${item.conversation_id ?? item.bundle_id}`}
                        aria-pressed={
                          (selected?.conversation_id ?? selected?.bundle_id) ===
                          (item.conversation_id ?? item.bundle_id)
                        }
                        onClick={() => setSelected(item)}
                      >
                        <strong>
                          {item.display_name ??
                            (item.conversation_id ? c.unnamed : c.bundle)}
                        </strong>
                        <span className={styles.meta}>
                          {item.platform} · {displayTime(locale, item.last_at)}
                          {item.unread && (
                            <span className={styles.badge}>{c.unread}</span>
                          )}
                          {item.unreplied && (
                            <span className={styles.badge}>{c.pending}</span>
                          )}
                          {item.link_pending_manual && <span>{c.manual}</span>}
                        </span>
                      </button>
                    </li>
                  ))}
                  {page?.next_cursor && (
                    <li>
                      <button
                        className={styles.button}
                        disabled={busy}
                        onClick={() => void load(page.next_cursor)}
                      >
                        {c.more}
                      </button>
                    </li>
                  )}
                </ul>
                <section className={styles.thread}>
                  {selected ? (
                    <>
                      <button
                        className={`${styles.button} ${styles.back}`}
                        onClick={() => setSelected(null)}
                      >
                        {c.back}
                      </button>
                      {selected.conversation_id ? (
                        <InboxThread
                          key={`${store.id}:${selected.conversation_id}`}
                          store={store}
                          conversation={selected}
                          locale={locale}
                          onUnauthorized={privacy.expire}
                          onRead={() =>
                            setPage((old) =>
                              old
                                ? {
                                    ...old,
                                    items: old.items.map((item) =>
                                      item.conversation_id ===
                                      selected.conversation_id
                                        ? { ...item, unread: false }
                                        : item,
                                    ),
                                    unread_total: Math.max(
                                      0,
                                      old.unread_total -
                                        (old.items.find(
                                          (item) =>
                                            item.conversation_id ===
                                            selected.conversation_id,
                                        )?.unread
                                          ? 1
                                          : 0),
                                    ),
                                  }
                                : old,
                            )
                          }
                        />
                      ) : (
                        <p className={styles.notice}>
                          {c.manual}
                          <br />
                          {c.linkUnavailable}
                          <button className={styles.button} disabled>
                            {c.copyLink}
                          </button>
                        </p>
                      )}
                    </>
                  ) : (
                    <p className={styles.empty}>{c.select}</p>
                  )}
                </section>
                {selected && (
                  <BuyerPanel
                    key={`${store.id}:${selected.conversation_id ?? selected.bundle_id}`}
                    store={store}
                    conversationId={selected.conversation_id ?? undefined}
                    bundleId={
                      selected.conversation_id
                        ? undefined
                        : (selected.bundle_id ?? undefined)
                    }
                    onUnauthorized={privacy.expire}
                  />
                )}
              </div>
            )}
          </>
        )}
      </main>
    </WorkspaceFrame>
  );
}
