// Purpose: render one private conversation and explicit keyed read/takeover/reply/release actions.
// Depends on: React, @live-commerce/format, inbox-client/types, Meta health, published templates and InboxFence.
// Used by: Inbox; content/drafts/retry keys live only until the thread unmounts.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import type { Store } from "@/lib/model";
import { displayTime } from "@live-commerce/format";
import type { ConversationItem, Thread, Template } from "@/lib/inbox-types";
import { inboxRead, inboxWrite, InboxError } from "@/lib/inbox-client";
import { useMetaHealth } from "@/lib/meta-health-hook";
import {
  InboxFence,
  ReplyReceipt,
  permitted,
  textLimit,
} from "@/src/features/messages/privacy";
import { inboxCopy, inboxError } from "@/src/features/messages/copy";
import styles from "@/src/features/messages/Inbox.module.css";

// A12 Plan and Check refuse at last_inbound + 24h - 5m (0128_lc_b4_sends.sql).
const replyWindowSafetyMs = 5 * 60 * 1000;

/** Show server-authoritative generation and delivery states; retries reuse the exact original submission. */
export function InboxThread({
  store,
  conversation,
  locale,
  onUnauthorized,
  onRead,
}: {
  store: Store;
  conversation: ConversationItem;
  locale: string;
  onUnauthorized: () => void;
  onRead: () => void;
}) {
  const c = inboxCopy(locale),
    cid = conversation.conversation_id;
  const fence = useRef(new InboxFence());
  const gone = useRef(false);
  const callbacks = useRef({ onUnauthorized, onRead });
  callbacks.current = { onUnauthorized, onRead };
  const [thread, setThread] = useState<Thread | null>(null);
  const [templates, setTemplates] = useState<Template[]>([]);
  const [text, setText] = useState("");
  const [template, setTemplate] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const inFlight = useRef(false);
  const [older, setOlder] = useState(true);
  const [revision, setRevision] = useState(0);
  const [delivery, setDelivery] = useState<string | null>(null);
  const [retry, setRetry] = useState(false);
  const pending = useRef(new ReplyReceipt());
  const clearGone = useCallback(() => {
    // Any authoritative 404 ends this conversation epoch; later failed reads cannot restore erased data.
    gone.current = true;
    fence.current.revoke();
    setThread(null);
    setText("");
    setTemplate("");
    setTemplates([]);
    pending.current.clear();
    setRetry(false);
    setDelivery(null);
    inFlight.current = false;
    setBusy(false);
  }, []);
  const [clock, setClock] = useState(0);
  const reply = permitted(store, "inbox:reply");
  const health = useMetaHealth(reply ? store.id : null, revision);
  const providerRows =
    health.data?.pages
      .flatMap((page) => page.capabilities)
      .filter(
        (cap) =>
          cap.provider ===
            (conversation.platform === "instagram"
              ? "instagram"
              : "facebook"),
      ) ?? [];
  const bindingCount = new Set(providerRows.map((cap) => cap.binding_id)).size;
  const rows = providerRows.filter((cap) => cap.capability === "dm_session");
  const advisory = bindingCount > 1;
  // A9 has no binding id yet: several bindings are advice only; A12 and Check scope the final refusal.
  const capable =
    advisory || (bindingCount === 1 && rows.length > 0 &&
    rows.every((cap) => cap.state === "ok" || cap.state === "review_required"));
  const review = !advisory && rows.some((cap) => cap.state === "review_required");
  const capabilityCode =
    rows.find((cap) => cap.state !== "ok" && cap.state !== "review_required")
      ?.state ?? "noCapability";
  const hardDeadline = thread ? Date.parse(thread.window_open_until) : NaN;
  const sendDeadline = hardDeadline - replyWindowSafetyMs;
  const now = Date.now();
  const open = Number.isFinite(sendDeadline) && now < sendDeadline;
  const closing = Number.isFinite(hardDeadline) && !open && now < hardDeadline;
  const limit = textLimit(conversation.platform, text);
  const templateRow = templates.find(
    (item) => `${item.template_id}:${item.version}` === template,
  );
  const load = useCallback(
    async (before?: number) => {
      if (!cid || inFlight.current || gone.current) return;
      inFlight.current = true;
      setBusy(true);
      const ticket = fence.current.begin();
      try {
        const data = await inboxRead<Thread>(
          store.id,
          `inbox/conversations/${cid}/messages?limit=50${before ? `&before_seq=${before}` : ""}`,
          ticket.signal,
        );
        if (!fence.current.current(ticket)) return;
        setThread((old) =>
          before && old
            ? { ...data, items: [...data.items, ...old.items] }
            : data,
        );
        if (before)
          setOlder(data.items.some((item) => item.direction === "in"));
        const seq = Math.max(0, ...data.items.map((item) => item.seq ?? 0));
        if (reply && !before && seq > 0) {
          // Calls A10 only for reply-capable staff; read-only users never emit a mutation.
          await inboxWrite(
            store.id,
            `inbox/conversations/${cid}/read`,
            { read_seq: seq },
            crypto.randomUUID(),
            ticket.signal,
          );
          if (fence.current.current(ticket)) callbacks.current.onRead();
        }
      } catch (cause) {
        if (!fence.current.current(ticket)) return;
        const code = cause instanceof InboxError ? cause.code : "unavailable";
        if (cause instanceof InboxError && cause.status === 404) {
          clearGone();
        }
        if (cause instanceof InboxError && [401, 403].includes(cause.status))
          callbacks.current.onUnauthorized();
        setError(code);
      } finally {
        if (fence.current.current(ticket)) {
          inFlight.current = false;
          setBusy(false);
        }
      }
    },
    [cid, store.id, reply, clearGone],
  );
  useEffect(() => {
    fence.current.invalidate(true);
    inFlight.current = false;
    void load();
    if (reply) {
      const ticket = fence.current.begin();
      void inboxRead<{ items: Template[] }>(
        store.id,
        "message-templates",
        ticket.signal,
      )
        .then((value) => {
          if (fence.current.current(ticket))
            setTemplates(
              value.items.filter((item) => item.kinds.includes("dm")),
            );
        })
        .catch(() => {
          if (fence.current.current(ticket)) setTemplates([]);
        });
    }
    const interval = setInterval(() => setClock((value) => value + 1), 15000);
    return () => {
      fence.current.invalidate(false);
      pending.current.clear();
      clearInterval(interval);
    };
  }, [load, store.id, reply]);
  useEffect(() => {
    if (!Number.isFinite(hardDeadline)) return;
    const current = Date.now();
    const next = current < sendDeadline ? sendDeadline : hardDeadline;
    if (next <= current) return;
    // Disable at the actual boundary instead of waiting for the 15-second clock fallback.
    const timer = setTimeout(() => setClock((value) => value + 1), Math.min(next - current, 2147483647));
    return () => clearTimeout(timer);
  }, [cid, hardDeadline, sendDeadline, clock]);
  const action = async (kind: "takeover" | "release") => {
    if (
      gone.current ||
      !cid ||
      !thread ||
      !reply ||
      inFlight.current ||
      pending.current.pending()
    )
      return;
    inFlight.current = true;
    setBusy(true);
    setError(null);
    const ticket = fence.current.begin();
    try {
      const result = await inboxWrite<{
        mode: "auto" | "human";
        assignee: string | null;
        takeover_generation: number;
      }>(
        store.id,
        `inbox/conversations/${cid}/${kind}`,
        { expected_generation: thread.takeover_generation },
        crypto.randomUUID(),
        ticket.signal,
      );
      if (!fence.current.current(ticket)) return;
      // Re-read the human expiry and window rather than synthesizing authority from the action result.
      setThread((old) => (old ? { ...old, ...result } : old));
      setRevision((value) => value + 1);
    } catch (cause) {
      if (!fence.current.current(ticket)) return;
      const code = cause instanceof InboxError ? cause.code : "unavailable";
      if (cause instanceof InboxError && cause.status === 404) clearGone();
      setError(code);
      if (cause instanceof InboxError && [401, 403].includes(cause.status))
        callbacks.current.onUnauthorized();
    } finally {
      if (fence.current.current(ticket)) {
        inFlight.current = false;
        setBusy(false);
        void load();
      }
    }
  };
  const send = async () => {
    if (
      gone.current ||
      !cid ||
      !thread ||
      !reply ||
      !capable ||
      !open ||
      Date.now() >= sendDeadline ||
      inFlight.current ||
      delivery === "unknown"
    )
      return;
    if (!pending.current.pending() && !templateRow && !limit.valid) return;
    const submission = pending.current.prepare(
      templateRow
        ? {
            template_id: templateRow.template_id,
            template_version: templateRow.version,
            expected_generation: thread.takeover_generation,
          }
        : {
            text: text.normalize("NFC"),
            expected_generation: thread.takeover_generation,
          },
    );
    inFlight.current = true;
    setBusy(true);
    setRetry(false);
    setError(null);
    const ticket = fence.current.begin();
    try {
      const result = await inboxWrite<{
        send_state: string;
        takeover_generation?: number;
      }>(
        store.id,
        `inbox/conversations/${cid}/messages`,
        submission.body,
        submission.key,
        ticket.signal,
      );
      if (!fence.current.current(ticket)) return;
      pending.current.clear();
      setText("");
      setTemplate("");
      setDelivery(result.send_state);
      if (result.takeover_generation !== undefined)
        setThread((old) =>
          old
            ? { ...old, takeover_generation: result.takeover_generation! }
            : old,
        );
    } catch (cause) {
      if (!fence.current.current(ticket)) return;
      const code = cause instanceof InboxError ? cause.code : "unavailable";
      if (cause instanceof InboxError && cause.status === 404) clearGone();
      setError(code);
      pending.current.failed(code);
      setRetry(pending.current.pending() !== null);
      if (cause instanceof InboxError && [401, 403].includes(cause.status))
        callbacks.current.onUnauthorized();
    } finally {
      if (fence.current.current(ticket)) {
        inFlight.current = false;
        setBusy(false);
        void load();
      }
    }
  };
  const seqs =
    thread?.items.flatMap((item) => (item.seq ? [item.seq] : [])) ?? [];
  return (
    <div data-testid="inbox-thread" aria-busy={busy}>
      <h2>
        {gone.current ? c.not_found : (conversation.display_name ?? c.unnamed)}
      </h2>
      {!thread && (
        <p className={styles.notice}>{busy ? c.loading : c.unavailable}</p>
      )}
      {thread && (
        <>
          <div className={styles.meta}>
            <span className={styles.badge}>
              {thread.mode === "human" ? c.human : c.auto}
            </span>
            <span>
              {c.window}:{" "}
              {open ? displayTime(locale, new Date(sendDeadline).toISOString()) : closing ? (
                <span role="status" data-testid="inbox-window-closing">{c.windowClosing}</span>
              ) : c.closed}
            </span>
            {thread.mode === "human" && thread.human_until && (
              <span>
                {c.expires}: {displayTime(locale, thread.human_until)}
              </span>
            )}
          </div>
          <div className={styles.actions}>
            <button
              data-testid="takeover"
              className={styles.button}
              disabled={!reply || busy || retry || thread.mode === "human"}
              onClick={() => void action("takeover")}
            >
              {c.takeover}
            </button>
            <button
              data-testid="release"
              className={styles.button}
              disabled={!reply || busy || retry || thread.mode !== "human"}
              onClick={() => void action("release")}
            >
              {c.release}
            </button>
            <button
              className={styles.button}
              disabled={busy || retry}
              onClick={() => {
                setError(null);
                void load();
              }}
            >
              {c.refresh}
            </button>
          </div>
          {older && seqs.length > 0 && (
            <button
              className={styles.button}
              disabled={busy}
              onClick={() => void load(Math.min(...seqs))}
            >
              {c.older}
            </button>
          )}
          <ol className={styles.messages}>
            {[...thread.items]
              .sort(
                (a, b) =>
                  (a.at ?? "").localeCompare(b.at ?? "") ||
                  (a.seq ?? 0) - (b.seq ?? 0),
              )
              .map((item, index) => (
                <li
                  className={styles.message}
                  data-direction={item.direction}
                  key={`${item.direction}:${item.seq ?? item.at ?? index}:${index}`}
                >
                  <p>{item.unreadable ? c.unreadable : item.text}</p>
                  {item.attachments.map((attachment, i) => (
                    <span className={styles.badge} key={i}>
                      {c.attachment}: {attachment.type}
                    </span>
                  ))}
                  <div className={styles.meta}>
                    {item.at && (
                      <time dateTime={item.at}>
                        {displayTime(locale, item.at)}
                      </time>
                    )}
                    {item.send_state && (
                      <span className={styles.badge}>
                        {c[item.send_state as "queued"] ?? c.unknown}
                      </span>
                    )}
                  </div>
                </li>
              ))}
          </ol>
        </>
      )}
      {error && (
        <p role="alert" className={`${styles.notice} ${styles.error}`}>
          {inboxError(locale, error)}
        </p>
      )}
      {delivery && (
        <p role="status" className={styles.notice}>
          {delivery === "unknown"
            ? c.uncertain
            : (c[delivery as "queued"] ?? c.unknown)}
        </p>
      )}
      {!reply && <p className={styles.notice}>{c.readonly}</p>}
      {reply && !capable && (
        <p className={styles.notice}>
          {c.capability} {inboxError(locale, capabilityCode)}
        </p>
      )}
      {review && <p className={styles.badge}>{c.review}</p>}
      {reply && advisory && (
        <p className={styles.notice} data-testid="inbox-capability-advisory" role="status">
          {c.capabilityAdvisory}
        </p>
      )}
      <form
        className={styles.composer}
        onSubmit={(event) => {
          event.preventDefault();
          void send();
        }}
      >
        <label>
          {c.template}
          <select
            value={template}
            disabled={!reply || busy || retry}
            onChange={(event) => setTemplate(event.target.value)}
          >
            <option value="">{c.freeText}</option>
            {templates.map((item) => (
              <option
                key={`${item.template_id}:${item.version}`}
                value={`${item.template_id}:${item.version}`}
              >
                {item.name} · v{item.version}
              </option>
            ))}
          </select>
        </label>
        {!template && (
          <label>
            {c.reply}
            <textarea
              data-testid="reply-text"
              value={text}
              disabled={!reply || busy || retry}
              onChange={(event) => setText(event.target.value)}
            />
          </label>
        )}
        {!template && (
          <span className={styles.meta}>
            {c.count}: {limit.count} / {limit.max}
          </span>
        )}
        <button
          data-testid="reply-send"
          className={`${styles.button} ${styles.primary}`}
          disabled={
            !reply ||
            !thread ||
            !capable ||
            !open ||
            busy ||
            delivery === "unknown" ||
            (!templateRow && !limit.valid)
          }
          type="submit"
        >
          {retry ? c.retry : c.send}
        </button>
      </form>
    </div>
  );
}
