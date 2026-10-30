// Purpose: render one private conversation and explicit keyed read/takeover/reply/release actions.
// Depends on: React, @live-commerce/format, inbox/operations clients and types, Meta health, templates and InboxFence.
// Used by: Inbox; content/drafts/retry keys live only until the thread unmounts.
// Invariants: server eligibility, CAS and UNKNOWN latches control every send; collapsed UI never grants authority.
"use client";
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import type { Store } from "@/lib/model";
import { displayTime, instantToTaipei } from "@live-commerce/format";
import type { ConversationItem, Thread, Template } from "@/lib/inbox-types";
import { inboxRead, inboxWrite, InboxError } from "@/lib/inbox-client";
import { readOperation } from "@/lib/operations-client";
import { operationID } from "@/lib/operations-model";
import { OrderReadError } from "@/lib/orders-client";
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
// W6-05B ACKNOWLEDGED is uncertain; only SUCCEEDED confirms this exact operation.
const operationDelivery = {
  READY: "queued", DISPATCHING: "queued", SUCCEEDED: "sent", FAILED_FINAL: "failed",
  BLOCKED_POLICY: "blocked", STALE_BINDING: "blocked", CANCELLED: "blocked",
  UNKNOWN: "unknown", ACKNOWLEDGED: "unknown",
} as const;

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
  const lockedOutbounds = useRef<Pick<Thread["items"][number], "at" | "outbound_id" | "send_state">[]>([]);
  const acknowledged = useRef<{ id: string | null; outbound_id: string | null; text: string | null } | null>(null);
  // The visible rejection can belong to another send; never recover a successful ACK's draft.
  const recoveryText = useRef<string | null>(null);
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
    lockedOutbounds.current = [];
    acknowledged.current = null;
    recoveryText.current = null;
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
          const ack = acknowledged.current;
          let ackComplete = false;
          let ackState: string | null = null;
          if (ack) {
            // A9 outbound identity correlates projections, never proves an operation result.
            // No permission, missing ID, failed or mismatched operation read stays locked.
            if (ack.id && permitted(store, "integration:read")) {
              try {
                const operation = await readOperation(store.id, ack.id,
                  AbortSignal.any([ticket.signal, AbortSignal.timeout(8000)]));
                if (!fence.current.current(ticket)) return;
                if (operation.action === "meta.dm_send" && operation.purpose === "service" &&
                  operation.provider === (conversation.platform === "instagram" ? "instagram" : "facebook") &&
                  operation.object?.kind === "conversation" && operation.object.id === cid) {
                  const state = operationDelivery[operation.state];
                  ackState = state;
                  ackComplete = state === "sent" || state === "failed";
                  setDelivery(state);
                  recoveryText.current = state === "failed" ? ack.text : null;
                  deliveryLock.current = state === "queued" || state === "blocked";
                  persistedUnknown.current ||= state === "unknown";
                }
              } catch (cause) {
                // Historical read only: never query/retry/cancel a provider effect or clear the ACK.
                if (!fence.current.current(ticket)) return;
                if (cause instanceof OrderReadError && cause.code === "signed-out") {
                  callbacks.current.onUnauthorized();
                  return;
                }
              }
            }
          }
          // Collect all obligations even while our ACK is pending: a later page may omit them.
          // Only an exact completed ACK supersedes its own stale projection. Missing identities
          // and conflicting rows fail closed; text, time and principal never establish identity.
          const settledByAck = (item: Pick<Thread["items"][number], "outbound_id">) =>
            ackComplete && !!ack?.outbound_id && item.outbound_id === ack.outbound_id;
          const unresolved = lockedOutbounds.current.filter(observed => {
            if (settledByAck(observed)) return false;
            const matches = data.items.filter(item => item.direction === "out" &&
              !!observed.outbound_id && item.outbound_id === observed.outbound_id);
            return matches.length === 0 || matches.some(item => item.send_state !== "sent" && item.send_state !== "failed");
          });
          for (const item of data.items) {
            if (item.direction !== "out" || (item.send_state !== "queued" && item.send_state !== "blocked") || settledByAck(item)) continue;
            // Identity-free rows share a conservative latch that no other row can settle.
            const observed = unresolved.find(row => item.outbound_id ? row.outbound_id === item.outbound_id
              : !row.outbound_id && row.send_state === item.send_state);
            if (observed) observed.send_state = item.send_state;
            else unresolved.push({ at: item.at, outbound_id: item.outbound_id, send_state: item.send_state });
          }
          lockedOutbounds.current = unresolved;
          if (!ack || ackComplete) {
            const latest = [...data.items].sort((a, b) => (b.at ?? "").localeCompare(a.at ?? ""))
              .find(item => item.direction === "out");
            const pendingProjection = [...unresolved].sort((a, b) => (b.at ?? "").localeCompare(a.at ?? ""))[0];
            // Retain a rejected ACK's own recovery, but successful A cannot mask distinct B's result.
            if (pendingProjection || !ack || (ackState === "sent" && latest && !settledByAck(latest))) {
              setDelivery(pendingProjection?.send_state ?? latest?.send_state ?? null);
              recoveryText.current = !pendingProjection && latest?.send_state === "failed" && !latest.unreadable
                ? latest.text : null;
            }
            deliveryLock.current = unresolved.length > 0;
          } else {
            deliveryLock.current ||= unresolved.length > 0;
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
    [cid, store.id, store.role, store.permissions, conversation.platform, reply, clearGone],
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
        operation_id?: string;
        outbound_id?: string;
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
      recoveryText.current = null;
      deliveryLock.current = result.send_state === "queued" || result.send_state === "blocked";
      lockedOutbounds.current = [];
      acknowledged.current = deliveryLock.current ? {
        id: typeof result.operation_id === "string" && operationID.test(result.operation_id) ? result.operation_id : null,
        outbound_id: typeof result.outbound_id === "string" && operationID.test(result.outbound_id) ? result.outbound_id : null,
        text: typeof submission.body.text === "string" ? submission.body.text : null,
      } : null;
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
        {delivery === "failed" && !uncertainDelivery && recoveryText.current !== null &&
          <button className={styles.button} data-testid="inbox-edit-original" disabled={busy} onClick={() => {
            setText(recoveryText.current!); setTemplate(""); setEditing(true); setError(null);
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
