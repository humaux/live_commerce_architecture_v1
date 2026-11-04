// Purpose: console middle-column comments and session-scoped inbox views; reuse the existing buyer panel.
// Depends on: A2/A8 transport, A13 BuyerPanel facts, privacy hook, transient comment model and console tokens.
// Used by: LiveConsole; names/text live only in authorised component memory, never URLs or storage.
// Invariants: drafts are buyer/channel scoped and cleared on privacy/epoch loss; order/payment facts never derive from claims.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { displayTime } from "@live-commerce/format";
import type { Store } from "@/lib/model";
import type { ConversationList, ConversationItem } from "@/lib/inbox-types";
import { inboxRead, InboxError } from "@/lib/inbox-client";
import { permitted } from "@/src/features/messages/privacy";
import type { ConsoleCapabilities, ConsoleOffer } from "@/src/features/live/console-model";
import { commentCopy, commentReason } from "@/src/features/live/comment-copy";
import {
  commentViewResource,
  commentSendState,
  COMMENT_MEMORY_CAP,
  mergeConversationPage,
  type CommentFilter,
} from "@/src/features/live/comment-model";
import { useCommentStream } from "@/src/features/live/use-comment-stream";
import { liveSettingsCopy } from "@/lib/live-settings-copy";
import { BuyerPanel } from "./BuyerPanel";
import { CommentReply } from "./CommentReply";
import { CommentLabelPrint, CommentLabelToolbar, CommentLabelAction } from "./CommentLabelPrint";

/** Keep stream/list/selection tied to one server-authorised store/session, with no identity guessing. */
export function CommentStream({
  locale,
  store,
  session,
  platform,
  capabilities,
  calibration = false,
  offers = [], limits = [], mobileTab = "comments",
}: {
  locale: Locale;
  store: Store;
  session: string;
  platform: "facebook" | "instagram";
  capabilities: ConsoleCapabilities;
  calibration?: boolean;
  offers?: ConsoleOffer[]; limits?: { offer_id: string; max_quantity_per_claim: number }[]; mobileTab?: string;
}) {
  const c = commentCopy(locale),
    [filter, setFilter] = useState<CommentFilter>("all"),
    [conversations, setConversations] = useState<ConversationList | null>(null),
    [listError, setListError] = useState("");
  const [panelFacts, setPanelFacts] = useState<{ bundle: string; orderCount: number | null; unpaid: boolean } | null>(null);
  const drafts = useRef(new Map<string, { private: string; public: string }>());
  const request = useRef(0), paginated = useRef(false), paging = useRef(false);
  const clearConversations = useCallback(() => {
    drafts.current.clear(); setPanelFacts(null);
    request.current++; paginated.current = false; paging.current = false;
    setConversations(null); setListError("");
  }, []);
  const stream = useCommentStream(
    store.id,
    session,
    permitted(store, "live:read"),
    calibration,
    clearConversations,
  );
  const { privacy, selection, select } = stream,
    resource = commentViewResource(filter, session);
  useEffect(() => {
    const generation = ++request.current;
    setConversations(null);
    setListError("");
    paginated.current = false; paging.current = false;
    if (
      !resource ||
      !privacy.visible ||
      privacy.blocked.current ||
      !permitted(store, "inbox:read")
    )
      return;
    let timer: ReturnType<typeof setTimeout> | undefined, failures = 0;
    const controller = new AbortController();
    // Schedule after settlement: A8 never overlaps itself or follows A2's faster cadence.
    const load = async () => {
      if (generation !== request.current || document.visibilityState !== "visible" || privacy.blocked.current) return;
      const ticket = privacy.fence.begin();
      let delay = 10000;
      try {
        const data = await inboxRead<ConversationList>(store.id, resource, AbortSignal.any([ticket.signal, controller.signal]));
        if (generation !== request.current || !privacy.fence.current(ticket)) return;
        if (!Array.isArray(data.items)) throw new InboxError("unavailable", 503);
        setConversations((old) => {
          return mergeConversationPage(paginated.current ? old : null, data, paginated.current);
        }); setListError(""); failures = 0;
      } catch (e) {
        if (generation !== request.current || !privacy.fence.current(ticket)) return;
        // Match the standalone inbox: a scoped 404 terminally revokes both private views.
        if (e instanceof InboxError && [401, 403, 404].includes(e.status)) privacy.expire();
        delay = Math.max(Math.min(30000, 10000 * 2 ** failures++), e instanceof InboxError ? e.retryAfter : 0);
        setListError(e instanceof InboxError ? e.code : "unavailable");
      } finally {
        if (generation === request.current && privacy.fence.current(ticket) && !privacy.blocked.current && document.visibilityState === "visible")
          timer = setTimeout(() => void load(), delay);
      }
    };
    void load();
    return () => {
      request.current++;
      controller.abort();
      clearTimeout(timer);
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
  const selectedOffer = offers.find((offer) => chosen?.marks.claim?.status === "ACCEPTED" && offer.offer_id === chosen.marks.claim.offer_id);
  const selectedLimit = limits.find((offer) => offer.offer_id === selectedOffer?.offer_id)?.max_quantity_per_claim;
  const bundle = chosen?.marks.claim?.bundle_id ?? selection?.bundle;
  const facts = panelFacts && panelFacts.bundle === bundle ? panelFacts : null;
  const registered = facts?.unpaid ? c.registered : c.registeredShort;
  // Privacy removal also drops draft text. No old/deleted comment retains a hidden draft.
  const present = new Set(stream.buffer.items.map((row) => row.ref));
  for (const ref of drafts.current.keys()) if (!present.has(ref) || !privacy.visible || privacy.blocked.current || !permitted(store, "live:read")) drafts.current.delete(ref);
  const chooseConversation = (item: ConversationItem) => {
    select(item.bundle_id ? { bundle: item.bundle_id } : null);
  };
  const loadConversations = async () => {
    if (!resource || !conversations?.next_cursor || conversations.items.length >= COMMENT_MEMORY_CAP || paging.current) return;
    paging.current = true;
    const generation = request.current,
      ticket = privacy.fence.begin();
    try {
      const data = await inboxRead<ConversationList>(
        store.id,
        `${resource}&cursor=${encodeURIComponent(conversations.next_cursor)}`,
        ticket.signal,
      );
      if (generation === request.current && privacy.fence.current(ticket)) {
        paginated.current = true;
        setConversations((old) =>
          mergeConversationPage(old, data),
        );
      }
    } catch (e) {
      if (generation === request.current && privacy.fence.current(ticket)) {
        if (e instanceof InboxError && [401, 403, 404].includes(e.status))
          privacy.expire();
        setListError(e instanceof InboxError ? e.code : "unavailable");
      }
    } finally { if (generation === request.current) paging.current = false; }
  };
  return (
    <CommentLabelPrint key={`${store.id}:${session}:${stream.resetGeneration}:${privacy.visible}:${privacy.revision}`}
      locale={locale} store={store.id} session={session} rows={stream.buffer.items}
      allowed={permitted(store, "live:manage")} active={privacy.visible && !privacy.blocked.current && !resource} onDenied={privacy.expire}>
    <div className="comment-workspace" data-testid="comment-workspace" data-mobile-tab={mobileTab}>
      <section
        className="comment-stream" id="live-panel-comments"
        data-testid="comment-stream"
        aria-label={c.title}
      >
        <header>
          <h3>{c.title}</h3>
          <button
            type="button"
            data-testid="comment-refresh"
            onClick={() => stream.refresh(true)}
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
          <p>{privacy.blocked.current || !permitted(store, "live:read") ? c.forbidden : c.hidden}</p>
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
                {conversations.next_cursor && conversations.items.length < COMMENT_MEMORY_CAP && (
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
            <CommentLabelToolbar />
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
                    <button type="button" data-testid={`comment-select-${row.ref}`} data-private="comment-author" aria-pressed={chosen?.ref === row.ref} onClick={() => select({ ref: row.ref })}><span className="live-selection-dot" aria-hidden="true" />{row.author_name ?? c.unnamed}</button>
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
                  {row.marks.private_reply?.kind === "out_of_stock" &&
                    commentSendState(row.marks.private_reply.state) === "sent" && (
                      <p className="comment-mark" data-testid={`comment-soldout-sent-${row.ref}`}>
                        {liveSettingsCopy(locale).replied}
                      </p>
                    )}
                  {row.marks.private_reply && (
                    <p className="live-helper" data-testid={`comment-send-state-${row.ref}`}>
                      {commentReason(locale, `delivery_${commentSendState(row.marks.private_reply.state)}`)}
                    </p>
                  )}
                  <CommentLabelAction row={row} />
                </li>
              ))}
            </ul>
            {stream.buffer.older && (
              <button
                type="button"
                data-testid="comment-older"
                disabled={stream.busy || stream.buffer.items.length >= COMMENT_MEMORY_CAP}
                onClick={stream.older}
              >
                {c.older}
              </button>
            )}
            {stream.buffer.items.length >= 1000 && <p className="live-helper">{c.memoryLimit}</p>}
          </>
        )}
      </section>
      <aside className="comment-buyer" id="live-panel-buyer" data-testid="comment-buyer">
        <div className="live-buyer-context">
          {privacy.visible && chosen ? <>
            <h3 data-private="comment-author">{chosen.author_name ?? c.unnamed}</h3>
            <p>{c.original}: <span data-private="comment-text">{chosen.text}</span></p>
            {selectedOffer && <>
              <p className="comment-mark">{selectedOffer.product_name} {chosen.marks.claim?.quantity} {c.units} · {registered}</p>
              <p className="live-helper">{c.stock} {selectedOffer.stock.tracked ? selectedOffer.stock.sellable : "—"} · {c.soldCount} {selectedOffer.paid_qty}</p>
              <p className="live-helper">{selectedOffer.keyword} · {c.cap} {selectedLimit ?? "—"} · {c.reserved} {selectedOffer.stock.reserved}<br />{c.stockHelper}</p>
            </>}
            {facts && facts.orderCount !== null && <p className="live-helper">{facts.orderCount === 0 ? c.buyerOrders : c.recentOrders} {facts.orderCount}</p>}
            <h4>{c.history}</h4><p><span data-private="comment-author">{chosen.author_name ?? c.unnamed}</span>: <span data-private="comment-text">{chosen.text}</span></p>
          </> : <p>{c.noSelection}</p>}
          {privacy.visible && bundle && permitted(store, "inbox:read") && <details data-live-buyer-details>
            <summary>{c.buyerDetails}</summary>
            <BuyerPanel key={`${store.id}:${bundle}`} store={store} bundleId={bundle} sessionId={session} onUnauthorized={privacy.expire} onFacts={(data) => setPanelFacts(data ? { bundle, orderCount: data.orders === undefined ? null : data.orders.length, unpaid: data.orders?.length === 0 } : null)} compact />
          </details>}
        </div>
        {privacy.visible && chosen && <div className="live-sticky-reply">
          <div data-testid="live-reply-summary" className="live-reply-summary">
            <strong data-private="comment-author">{chosen.author_name ?? c.unnamed}</strong> · <span data-private="comment-text">{chosen.text}</span>
            {selectedOffer && <><br />{selectedOffer.product_name} {chosen.marks.claim?.quantity} {c.units} · {registered}{facts && facts.orderCount !== null && <> · {facts.orderCount === 0 ? c.buyerOrders : c.recentOrders} {facts.orderCount}</>}<br />{c.stock} {selectedOffer.stock.tracked ? selectedOffer.stock.sellable : "—"} · {c.soldCount} {selectedOffer.paid_qty} · {c.cap} {selectedLimit ?? "—"} · {c.reserved} {selectedOffer.stock.reserved}</>}
          </div>
          <CommentReply key={`${session}:${chosen.ref}:${stream.resetGeneration}`} store={store} session={session} comment={chosen} locale={locale} platform={platform} capabilities={capabilities} offer={selectedOffer} drafts={drafts.current} onSent={stream.refresh} onDenied={privacy.expire} />
        </div>}
      </aside>
    </div>
    </CommentLabelPrint>
  );
}
