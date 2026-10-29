// Purpose: render one private conversation and explicit keyed read/takeover/reply/release actions.
// Depends on: React, @live-commerce/format, inbox-client/types, Meta health, published templates and InboxFence.
// Used by: Inbox; content/drafts/retry keys live only until the thread unmounts.
// Invariants: server eligibility, CAS and UNKNOWN latches control every send; collapsed UI never grants authority.
"use client";
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import type { Store } from "@/lib/model";
import { displayTime, instantToTaipei } from "@live-commerce/format";
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
  customerPanel,
}: {
  store: Store;
  conversation: ConversationItem;
  locale: string;
  onUnauthorized: () => void;
  onRead: () => void;
  customerPanel?: ReactNode;
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
  const [details, setDetails] = useState(false);
  const [readMarked, setReadMarked] = useState(!conversation.unread);
  const [editing, setEditing] = useState(false);
  const [queriedUnknown, setQueriedUnknown] = useState(false);
  const [help, setHelp] = useState(false);
  const deliveryLock = useRef(false);
  const lockedOutbound = useRef<Pick<Thread["items"][number], "at" | "kind" | "principal_id"> | null>(null);
  const [delivery, setDelivery] = useState<string | null>(null);
  const persistedUnknown = useRef(false);
  const historyClear = useRef(false);
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
    setDetails(false); setReadMarked(false); setEditing(false); setQueriedUnknown(false);
    deliveryLock.current = false;
    lockedOutbound.current = null;
    persistedUnknown.current = false;
    historyClear.current = false;
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
  // A9 identifies the exact enabled binding used by plan_dm. An unrelated healthy Page is no authority.
  const rows = providerRows.filter((cap) => !!thread?.binding_id && cap.binding_id === thread.binding_id && cap.capability === "dm_session");
  const capable = rows.length > 0 && rows.every((cap) => cap.state === "ok" || cap.state === "review_required");
  const review = capable && rows.some((cap) => cap.state === "review_required");
  const capabilityCode = rows.find((cap) => cap.state !== "ok" && cap.state !== "review_required")?.state ?? "noCapability";
  const hardDeadline = thread ? Date.parse(thread.window_open_until) : NaN;
  const sendDeadline = hardDeadline - replyWindowSafetyMs;
  const now = Date.now();
  const open = Number.isFinite(sendDeadline) && now < sendDeadline;
  const closing = Number.isFinite(hardDeadline) && !open && now < hardDeadline;
  const uncertainDelivery = delivery === "unknown" || persistedUnknown.current;
  const limit = textLimit(conversation.platform, text);
  const templateRow = templates.find(
    (item) => `${item.template_id}:${item.version}` === template,
  );
  const load = useCallback(
    async (before?: number) => {
      if (!cid || inFlight.current || gone.current) return;
      inFlight.current = true;
      historyClear.current = false;
      setBusy(true);
      const ticket = fence.current.begin();
      try {
        const data = await inboxRead<Thread>(
          store.id,
          `inbox/conversations/${cid}/messages?limit=50${before ? `&before_seq=${before}` : ""}`,
          ticket.signal,
        );
        if (!fence.current.current(ticket)) return;
        // Go encodes a nil attachment slice as null; other malformed shapes must never reach rendering.
        if (!Array.isArray(data.items) || data.items.some((item) =>
          !item || (item.attachments !== null && (!Array.isArray(item.attachments) ||
            item.attachments.some((attachment) => !attachment || typeof attachment.type !== "string")))))
          throw new InboxError("unavailable", 503);
        // A9 pages can omit older outbound rows: absence cannot reconcile a known final UNKNOWN.
        // Latch before React commits so previously captured callbacks also respect this authority.
        persistedUnknown.current ||= data.has_unknown_outbound === true ||
          data.items.some((item) => item.direction === "out" && item.send_state === "unknown");
        // Null, absent or malformed authority is not proof of a clear history, including on older pages.
        historyClear.current = data.has_unknown_outbound === false;
        if (!before) {
          const observed = lockedOutbound.current;
          // A9 can hide a pending/policy-blocked row below its display floor. Neither absence nor
          // another send reconciles it; retain the lock until its projected row has a known result.
          const latest = deliveryLock.current && observed
            ? data.items.find(item => item.direction === "out" && observed.at !== null &&
              item.at === observed.at && item.kind === observed.kind && item.principal_id === observed.principal_id)
            : [...data.items].sort((a, b) => (b.at ?? "").localeCompare(a.at ?? ""))
              .find(item => item.direction === "out");
          if (latest?.send_state) {
            setDelivery(latest.send_state);
            deliveryLock.current = latest.send_state === "queued" || latest.send_state === "blocked";
            lockedOutbound.current = deliveryLock.current
              ? { at: latest.at, kind: latest.kind, principal_id: latest.principal_id } : null;
          } else if (!deliveryLock.current) {
            setDelivery(null);
          }
        }
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
          if (fence.current.current(ticket)) { setReadMarked(true); callbacks.current.onRead(); }
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
      !historyClear.current ||
      uncertainDelivery || persistedUnknown.current ||
      (deliveryLock.current && !pending.current.pending())
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
      deliveryLock.current = result.send_state === "queued" || result.send_state === "blocked";
      lockedOutbound.current = null;
      setEditing(false); setQueriedUnknown(false);
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
  const channel = conversation.platform === "instagram" ? c.instagram : c.messenger;
  const latestOutbound = [...(thread?.items ?? [])].sort((a, b) => (b.at ?? "").localeCompare(a.at ?? ""))
    .find(item => item.direction === "out");
  // A later inbound starts the next reply task after a confirmed send, using the same signed composer.
  // It cannot unlock a queued, blocked or UNKNOWN outcome.
  const nextReply = delivery === "sent" && latestOutbound?.at && thread?.items.some(item =>
    item.direction === "in" && item.at && item.at > latestOutbound.at!);
  const folded = !!(delivery || uncertainDelivery) && !editing && !retry && !nextReply;
  const deliveryLabels = { queued: c.deliveryPending, sent: c.deliverySent, failed: c.deliveryRejected,
    blocked: c.locked, unknown: c.deliveryUnknown };
  const resultCopy = uncertainDelivery
    ? queriedUnknown ? c.deliveryStillUnknown.replace("{channel}", channel) : c.deliveryUnknown
    : delivery === "queued" ? c.deliveryPending
    : delivery === "sent" ? c.deliverySent
    : delivery === "failed" ? c.deliveryRejected : c.locked;
  const eligibility = !reply ? c.permissionMissing : !capable
    ? ["missing_permission", "missing_task"].includes(capabilityCode) ? c.permissionMissing : c.connectionPending
    : !open ? closing ? c.windowClosing : c.replyEnded
    : c.replyBy.replace("{time}", displayTime(locale, new Date(sendDeadline).toISOString()));
  return (
    <div data-testid="inbox-thread" aria-busy={busy}>
      <header className={styles.threadHeading}>
        <div>
          <h2>{gone.current ? c.not_found : (conversation.display_name ?? c.unnamed)}</h2>
          {thread && <p className={styles.meta}>{`${channel} · ${readMarked ? c.storeRead : c.unread}`}</p>}
        </div>
        <button className={styles.iconButton} aria-label={c.refresh} disabled={busy || retry}
          onClick={() => { setError(null); void load(); }}>
          <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M20 7v5h-5M4 17v-5h5"/><path d="M6 7a7 7 0 0 1 12-1l2 6M4 12l2 6a7 7 0 0 0 12-1"/></svg>
        </button>
      </header>
      {!thread && <p className={styles.notice}>{busy ? c.loading : c.unavailable}</p>}
      {thread && <>
        <button className={styles.disclosure} data-testid="customer-details-toggle"
          aria-expanded={details} aria-controls={`customer-details-${cid}`} onClick={() => setDetails(value => !value)}>
          {`${c.customerDetails}${thread.mode === "human" ? ` · ${c.human} / ${c.autoPaused}` : ""}`}
        </button>
        <div hidden={!details} id={`customer-details-${cid}`} data-testid="customer-details">
          {thread.mode === "human" && thread.human_until && <p className={styles.meta}>{`${c.expires}: ${displayTime(locale, thread.human_until)}`}</p>}
          <div className={styles.actions}>
            <button data-testid="takeover" className={styles.button}
              disabled={!reply || busy || retry || thread.mode === "human"} onClick={() => void action("takeover")}>{c.takeover}</button>
            <button data-testid="release" className={styles.button}
              disabled={!reply || busy || retry || thread.mode !== "human"} onClick={() => void action("release")}>{c.release}</button>
            {older && seqs.length > 0 && <button className={styles.button} disabled={busy}
              onClick={() => void load(Math.min(...seqs))}>{c.older}</button>}
          </div>
          {templates.length > 0 && <label>{c.template}
            <select value={template} disabled={!reply || busy || retry || folded}
              onChange={event => setTemplate(event.target.value)}>
              <option value="">{c.freeText}</option>
              {templates.map(item => <option key={`${item.template_id}:${item.version}`} value={`${item.template_id}:${item.version}`}>
                {item.name} · v{item.version}
              </option>)}
            </select>
          </label>}
          {customerPanel}
        </div>
        <ol className={styles.messages}>
          {[...thread.items].sort((a, b) => (a.at ?? "").localeCompare(b.at ?? "") || (a.seq ?? 0) - (b.seq ?? 0))
            .map((item, index) => <li className={styles.message} data-direction={item.direction}
              key={`${item.direction}:${item.seq ?? item.at ?? index}:${index}`}>
              {item.at && <time className={styles.meta} dateTime={item.at} title={displayTime(locale, item.at)}>{instantToTaipei(item.at).slice(-5)}</time>}
              <p>{item.unreadable ? c.unreadable : item.text}</p>
              {item.send_state && (item !== latestOutbound || !folded) && <span className={styles.meta}>{deliveryLabels[item.send_state]}</span>}
              {(item.attachments ?? []).map((attachment, i) => <span className={styles.badge} key={i}>{c.attachment}: {attachment.type}</span>)}
            </li>)}
        </ol>
      </>}
      {error && <p role="alert" className={`${styles.notice} ${styles.error}`}>{inboxError(locale, error)}</p>}
      {folded && <section className={styles.result}>
        <p role="status" data-testid="inbox-delivery-state">{resultCopy}</p>
        {(uncertainDelivery || delivery === "queued" || delivery === "blocked") && <button data-testid="inbox-send-locked" className={styles.button} disabled>{c.locked}</button>}
        {delivery === "failed" && !uncertainDelivery && latestOutbound && !latestOutbound.unreadable &&
          <button className={styles.button} data-testid="inbox-edit-original" disabled={busy} onClick={() => {
            setText(latestOutbound.text); setTemplate(""); setEditing(true); setError(null);
          }}>{c.editOriginal}</button>}
        <button className={styles.button} data-testid="inbox-send-status" disabled={busy || retry} onClick={() => {
          setError(null); setQueriedUnknown(uncertainDelivery); void load();
        }}>{c.queryStatus}</button>
      </section>}
      {reply && thread && !historyClear.current && !uncertainDelivery && !busy && <p role="status" className={styles.notice}>{c.historyUnverified.replace(c.messenger, channel)}</p>}
      {review && <p className={styles.badge}>{c.review}</p>}
      <form className={styles.composer} hidden={folded} onSubmit={event => { event.preventDefault(); void send(); }}>
        {thread && <p data-testid="inbox-eligibility" className={styles.meta}>
          <span data-testid={closing ? "inbox-window-closing" : undefined} role={closing ? "status" : undefined}>{eligibility}</span>
        </p>}
        {(!capable || !open) && thread && <>
          <button type="button" className={styles.disclosure} aria-expanded={help} onClick={() => setHelp(value => !value)}>
            {!open ? c.windowAction : c.connectionAction}
          </button>
          {help && <p className={styles.meta}>{!open ? c.windowHelp : c.connectionHelp}</p>}
        </>}
        {!template && <textarea aria-label={c.reply} data-testid="reply-text" value={text}
          disabled={!reply || busy || retry}
          onChange={event => setText(event.target.value)} />}
        {!template && text.length > 0 && !limit.valid && <span className={styles.meta}>{`${c.count}: ${limit.count} / ${limit.max}`}</span>}
        <div className={styles.actions}>
          <button data-testid="reply-send" className={`${styles.button} ${styles.primary}`} disabled={
            !reply || !thread || !capable || !open || busy || !historyClear.current || uncertainDelivery ||
            (deliveryLock.current && !retry) || (!templateRow && !limit.valid)
          } type="submit">{retry ? c.retry : c.sendAction}</button>
          <button type="button" className={styles.button} data-testid="inbox-clear-draft"
            disabled={busy || retry || !reply || uncertainDelivery || delivery === "blocked"}
            onClick={() => { setText(""); setTemplate(""); }}>{c.clearDraft}</button>
        </div>
      </form>
    </div>
  );
}
