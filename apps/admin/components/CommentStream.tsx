// Purpose: console middle-column comments and session-scoped inbox views; reuse the existing buyer panel.
// Depends on: A2/A8 inbox transport, privacy hook, transient comment model and W0 presentation tokens.
// Used by: LiveConsole; names/text live only in visible component memory, never URLs or storage.
"use client";
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { displayTime } from "@live-commerce/format";
import type { Store } from "@/lib/model";
import type { ConversationList, ConversationItem } from "@/lib/inbox-types";
import { inboxRead, InboxError } from "@/lib/inbox-client";
import { permitted } from "@/src/features/messages/privacy";
import type { ConsoleCapabilities } from "@/src/features/live/console-model";
import { commentCopy, commentReason } from "@/src/features/live/comment-copy";
import {
  commentViewResource,
  commentSendState,
  type CommentFilter,
} from "@/src/features/live/comment-model";
import { useCommentStream } from "@/src/features/live/use-comment-stream";
import { BuyerPanel } from "./BuyerPanel";
import { CommentReply } from "./CommentReply";

/** Keep stream/list/selection tied to one server-authorised store/session, with no identity guessing. */
export function CommentStream({
  locale,
  store,
  session,
  platform,
  capabilities,
  calibration = false,
}: {
  locale: Locale;
  store: Store;
  session: string;
  platform: "facebook" | "instagram";
  capabilities: ConsoleCapabilities;
  calibration?: boolean;
}) {
  const c = commentCopy(locale),
    [filter, setFilter] = useState<CommentFilter>("all"),
    [conversations, setConversations] = useState<ConversationList | null>(null),
    [listError, setListError] = useState("");
  const stream = useCommentStream(
    store.id,
    session,
    permitted(store, "live:read"),
    calibration,
  );
  const { privacy, selection, select } = stream,
    resource = commentViewResource(filter, session),
    request = useRef(0);
  useEffect(() => {
    const generation = ++request.current;
    setConversations(null);
    setListError("");
    if (
      !resource ||
      !privacy.visible ||
      privacy.blocked.current ||
      !permitted(store, "inbox:read")
    )
      return;
    const ticket = privacy.fence.begin();
    void inboxRead<ConversationList>(store.id, resource, ticket.signal)
      .then((data) => {
        if (
          generation === request.current &&
          privacy.fence.current(ticket) &&
          Array.isArray(data.items)
        )
          setConversations(data);
      })
      .catch((e) => {
        if (generation !== request.current || !privacy.fence.current(ticket))
          return;
        if (e instanceof InboxError && [401, 403].includes(e.status))
          privacy.expire();
        setListError(e instanceof InboxError ? e.code : "unavailable");
      });
    return () => {
      request.current++;
    };
  }, [
    resource,
    store,
    privacy.visible,
    privacy.revision,
    privacy.blocked,
    privacy.fence,
    privacy.expire,
    stream.revision,
  ]);
  const rows = stream.buffer.items.filter(
    (row) => filter !== "keyword" || row.marks.claim !== null,
  );
  const chosen = selection?.ref
    ? stream.buffer.items.find((row) => row.ref === selection.ref)
    : null;
  const bundle = chosen?.marks.claim?.bundle_id ?? selection?.bundle;
  const chooseConversation = (item: ConversationItem) => {
    select(item.bundle_id ? { bundle: item.bundle_id } : null);
  };
  const loadConversations = async () => {
    if (!resource || !conversations?.next_cursor) return;
    const generation = request.current,
      ticket = privacy.fence.begin();
    try {
      const data = await inboxRead<ConversationList>(
        store.id,
        `${resource}&cursor=${encodeURIComponent(conversations.next_cursor)}`,
        ticket.signal,
      );
      if (generation === request.current && privacy.fence.current(ticket))
        setConversations((old) =>
          old ? { ...data, items: [...old.items, ...data.items] } : data,
        );
    } catch (e) {
      if (generation === request.current && privacy.fence.current(ticket)) {
        if (e instanceof InboxError && [401, 403].includes(e.status))
          privacy.expire();
        setListError(e instanceof InboxError ? e.code : "unavailable");
      }
    }
  };
  return (
    <div className="comment-workspace" data-testid="comment-workspace">
      <section
        className="comment-stream"
        data-testid="comment-stream"
        aria-label={c.title}
      >
        <header>
          <h3>{c.title}</h3>
          <button
            type="button"
            data-testid="comment-refresh"
            onClick={stream.refresh}
            disabled={privacy.blocked.current || !privacy.visible}
          >
            {c.refresh}
          </button>
        </header>
        <div className="comment-filters" role="group" aria-label={c.title}>
          {(["all", "keyword", "private", "unreplied"] as CommentFilter[]).map(
            (f) => (
              <button
                type="button"
                key={f}
                data-testid={`comment-filter-${f}`}
                aria-pressed={filter === f}
                onClick={() => {
                  select(null);
                  setFilter(f);
                }}
              >
                {c[f]}
              </button>
            ),
          )}
        </div>
        {(stream.error || listError) && (
          <p role="status" data-testid="comment-stream-status">
            {commentReason(locale, listError || stream.error)}
          </p>
        )}
        {!privacy.visible ? (
          <p>{privacy.blocked.current ? c.forbidden : c.hidden}</p>
        ) : resource ? (
          <>
            {!permitted(store, "inbox:read") ? (
              <p>{c.forbidden}</p>
            ) : !conversations ? (
              <p role="status">
                {listError ? commentReason(locale, listError) : c.loading}
              </p>
            ) : (
              <>
                <ul
                  className="comment-rows"
                  data-testid="comment-conversations"
                >
                  {conversations.items
                    .filter((row) => filter !== "unreplied" || row.unreplied)
                    .map((row, i) => (
                      <li key={row.conversation_id ?? row.bundle_id ?? i}>
                        <button
                          type="button"
                          onClick={() => chooseConversation(row)}
                          disabled={!row.bundle_id}
                        >
                          {row.display_name ?? c.unnamed} ·{" "}
                          {displayTime(locale, row.last_at)}
                        </button>
                        {row.link_pending_manual && <p>{c.pendingLink}</p>}
                      </li>
                    ))}
                </ul>
                {conversations.next_cursor && (
                  <button
                    type="button"
                    onClick={() => void loadConversations()}
                  >
                    {c.older}
                  </button>
                )}
              </>
            )}
          </>
        ) : (
          <>
            {!rows.length && (
              <p role="status">{stream.busy ? c.loading : c.empty}</p>
            )}
            <ul className="comment-rows" data-testid="comment-rows">
              {rows.map((row) => (
                <li
                  key={row.ref}
                  data-testid={`comment-row-${row.ref}`}
                  data-selected={chosen?.ref === row.ref || undefined}
                >
                  <div className="comment-byline">
                    <strong data-private="comment-author">
                      {row.author_name ?? c.unnamed}
                    </strong>
                    <time>{displayTime(locale, row.created_at)}</time>
                  </div>
                  <p data-private="comment-text">{row.text}</p>
                  {row.has_attachment && (
                    <span className="live-helper">{c.attachment}</span>
                  )}
                  {row.marks.claim && (
                    <p className="comment-mark">
                      {row.marks.claim.keyword ?? c.keyword}
                      {row.marks.claim.quantity !== null
                        ? ` × ${row.marks.claim.quantity}`
                        : ""}
                    </p>
                  )}
                  {row.marks.claim?.reason === "restricted" && (
                    <p className="comment-mark">{c.restricted}</p>
                  )}
                  {row.marks.private_reply && (
                    <p className="live-helper">
                      {commentReason(locale, commentSendState(row.marks.private_reply.state))}
                    </p>
                  )}
                  <button
                    type="button"
                    data-testid={`comment-select-${row.ref}`}
                    onClick={() => select({ ref: row.ref })}
                  >
                    {c.select}
                  </button>
                </li>
              ))}
            </ul>
            {stream.buffer.older && (
              <button
                type="button"
                data-testid="comment-older"
                disabled={stream.busy}
                onClick={stream.older}
              >
                {c.older}
              </button>
            )}
          </>
        )}
        {privacy.visible && chosen && (
          <CommentReply
            key={`${session}:${chosen.ref}:${stream.resetGeneration}`}
            store={store}
            session={session}
            comment={chosen}
            locale={locale}
            platform={platform}
            capabilities={capabilities}
            onSent={stream.refresh}
            onDenied={privacy.expire}
          />
        )}
      </section>
      <aside className="comment-buyer" data-testid="comment-buyer">
        {privacy.visible && bundle && permitted(store, "inbox:read") ? (
          <BuyerPanel
            key={`${store.id}:${bundle}`}
            store={store}
            bundleId={bundle}
            onUnauthorized={privacy.expire}
          />
        ) : (
          <p>{chosen ? c.noBundle : c.selectBuyer}</p>
        )}
      </aside>
    </div>
  );
}
