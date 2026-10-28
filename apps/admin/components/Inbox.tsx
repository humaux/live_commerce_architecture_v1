// Purpose: own private inbox list/filter/cursor selection with no buyer data in navigation or storage.
// Depends on: React/react-dom, @live-commerce/format, inbox-client/types, BundleRecovery, WorkspaceFrame and privacy fences.
// Used by: /[locale]/messages; threads and BuyerPanel are cleared on hide or session changes.
// Invariants: successful reads remove unread-filter rows without clearing the selected thread.
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
import { BundleRecovery } from "@/src/features/messages/bundle-recovery";

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
  const [counts, setCounts] = useState({ all: 0, assigned: 0 });
  const [page, setPage] = useState<ConversationList | null>(null);
  const [selected, setSelected] = useState<ConversationItem | null>(null);
  const [error, setError] = useState(initialError);
  const [busy, setBusy] = useState(false);
  const [reload, setReload] = useState(0);
  const loading = useRef(false);
  const clear = useCallback(() => {
    setPage(null);
    setCounts({ all: 0, assigned: 0 });
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
      const query = new URLSearchParams({ filter: filter === "assigned" ? "all" : filter, limit: "50" });
      if (cursor) query.set("cursor", cursor);
      try {
        const data = await inboxRead<ConversationList>(
          store.id,
          `inbox/conversations?${query}`,
          ticket.signal,
        );
        if (!privacy.fence.current(ticket)) return;
        if (filter === "all" || filter === "assigned") {
          setCounts((old) => ({
            all: cursor ? old.all + data.items.length : data.items.length,
            assigned: (cursor ? old.assigned : 0) + data.items.filter(item => item.assignee !== null).length,
          }));
        }
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
          setError(cause.status === 404 ? "inboxUnavailable" : code);
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
  const changeFilter = (value: string) => {
    if (value === filter) return;
    privacy.fence.invalidate(true);
    setPage(null); setSelected(null); setError(null); setBusy(false);
    loading.current = false;
    setFilter(value);
  };
  const rows = page?.items.filter(item => filter !== "assigned" || item.assignee !== null);
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
      <main className={styles.page} data-testid="inbox-page" data-selected={!!selected}>
        <header className={styles.heading}>
          <h1>{c.simpleTitle}</h1>
        </header>
        {!store || !allowed ? (
          <p role="alert" className={styles.notice}>
            {status ? inboxError(locale, status) : c.forbidden}
          </p>
        ) : (
          <>
            <div className={styles.toolbar}>
              {([
                ["all", c.simpleAll, counts.all],
                ["unread", c.unread, page?.unread_total ?? 0],
                ["assigned", c.assignedInStore, counts.assigned],
              ] as const).map(([value, label, count]) => (
                <button key={value} className={styles.chip} data-testid={`inbox-filter-${value}`}
                  aria-pressed={filter === value} disabled={!privacy.visible || privacy.blocked.current}
                  onClick={() => changeFilter(value)}>{`${label} ${count}`}</button>
              ))}
              <button className={styles.chip} data-testid="inbox-filter-instagram" aria-pressed={filter === "instagram"}
                disabled={!privacy.visible || privacy.blocked.current} onClick={() => changeFilter("instagram")}>{c.instagram}</button>
              <button className={styles.iconButton} aria-label={c.refresh}
                disabled={busy || !privacy.visible || privacy.blocked.current} onClick={() => setReload(value => value + 1)}>
                <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M20 7v5h-5M4 17v-5h5"/><path d="M6 7a7 7 0 0 1 12-1l2 6M4 12l2 6a7 7 0 0 0 12-1"/></svg>
              </button>
              <details className={styles.extraFilters}>
                <summary data-testid="inbox-more-filters" aria-label={c.more}><svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="1.5"/><circle cx="12" cy="12" r="1.5"/><circle cx="19" cy="12" r="1.5"/></svg></summary>
                <label>{c.title}
                  <select data-testid="inbox-filter" value={filter === "assigned" ? "all" : filter}
                    disabled={!privacy.visible || privacy.blocked.current} onChange={event => changeFilter(event.target.value)}>
                    {["all", "unread", "unreplied", "messenger", "instagram", "live_comment"].map(value =>
                      <option key={value} value={value}>{c[value as "all"]}</option>)}
                  </select>
                </label>
              </details>
            </div>
            {status && (
              <p role="alert" className={`${styles.notice} ${styles.error}`}>
                {inboxError(locale, status)}
              </p>
            )}
            {!privacy.visible ? (
              status !== "inboxUnavailable" && (
                <p className={styles.notice}>
                  {privacy.departing
                    ? c.loading
                    : privacy.blocked.current
                      ? c.signedOut
                      : c.hidden}
                </p>
              )
            ) : (
              <div
                className={`${styles.layout} ${selected ? styles.selected : ""}`}
                data-bundle={!!selected && !selected.conversation_id}
              >
                <ul
                  className={styles.list}
                  data-testid="inbox-list"
                  aria-label={c.title}
                  aria-busy={busy}
                >
                  {!rows?.length && (
                    <li className={styles.empty}>
                      {busy ? c.loading : c.empty}
                    </li>
                  )}
                  {rows?.map((item) => (
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
                        <span className={styles.meta}>{`${c[item.platform as "instagram"] ?? c.live_comment} · ${displayTime(locale, item.last_at)} · ${item.unread ? c.unread : c.read}`}</span>
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
                        data-testid="inbox-back" aria-label={c.back}
                        className={`${styles.iconButton} ${styles.back}`}
                        onClick={() => setSelected(null)}
                      >
                        <svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="m14 6-6 6 6 6"/></svg>
                      </button>
                      {selected.conversation_id ? (
                        <InboxThread
                          key={`${store.id}:${selected.conversation_id}`}
                          store={store}
                          conversation={selected}
                          locale={locale}
                          customerPanel={<BuyerPanel store={store} conversationId={selected.conversation_id} compact onUnauthorized={privacy.expire} />}
                          onUnauthorized={privacy.expire}
                          onRead={() =>
                            setPage((old) =>
                              old
                                ? {
                                    ...old,
                                    items: old.items
                                      .map((item) =>
                                        item.conversation_id ===
                                        selected.conversation_id
                                          ? { ...item, unread: false }
                                          : item,
                                      )
                                      .filter(
                                        (item) =>
                                          filter !== "unread" || item.unread,
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
                        <BundleRecovery
                          key={`${store.id}:${selected.bundle_id}:${selected.session_id}:${locale}`}
                          store={store}
                          conversation={selected}
                          locale={locale}
                          onUnauthorized={privacy.expire}
                        />
                      )}
                    </>
                  ) : (
                    <p className={styles.empty}>{c.select}</p>
                  )}
                </section>
                {selected && !selected.conversation_id && <BuyerPanel
                  key={`${store.id}:${selected.bundle_id}`} store={store} bundleId={selected.bundle_id ?? undefined}
                  onUnauthorized={privacy.expire} />}

              </div>
            )}
          </>
        )}
      </main>
    </WorkspaceFrame>
  );
}
