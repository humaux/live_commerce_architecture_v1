// Purpose: explicit A4/A5 reply submission with coded rules, capability gates and uncertain-send fencing.
// Depends on: inbox transport, privacy epochs, published templates and server comment marks.
// Used by: CommentStream; never persists a reply body or automatically resends an uncertain command.
"use client";
import { useEffect, useRef, useState } from "react";
import type { Store } from "@/lib/model";
import type { Template, SendState } from "@/lib/inbox-types";
import { inboxRead, inboxWrite, InboxError } from "@/lib/inbox-client";
import {
  InboxFence,
  permitted,
  textLimit,
} from "@/src/features/messages/privacy";
import { commentCopy, commentReason } from "@/src/features/live/comment-copy";
import type { StreamComment } from "@/src/features/live/comment-model";
import { commentSendState } from "@/src/features/live/comment-model";
import { validCommentText } from "@/src/features/live/comment-request";
import type { ConsoleCapabilities } from "@/src/features/live/console-model";
import { sessionBoundary } from "@/lib/settings-client";
import { CommentReceipt } from "@/src/features/live/comment-receipt";

/** Submits one immutable attempt; browser transport uncertainty locks this comment until verified externally. */
export function CommentReply({
  store,
  session,
  comment,
  locale,
  platform,
  capabilities,
  onSent,
  onDenied,
}: {
  store: Store;
  session: string;
  comment: StreamComment;
  locale: string;
  platform: "facebook" | "instagram";
  capabilities: ConsoleCapabilities;
  onSent: () => void;
  onDenied: () => void;
}) {
  const c = commentCopy(locale),
    [mode, setMode] = useState<"private" | "public">("private"),
    [text, setText] = useState(""),
    [templates, setTemplates] = useState<Template[]>([]),
    [template, setTemplate] = useState("");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [state, setState] = useState<SendState | null>(null),
    [confirm, setConfirm] = useState(false);
  const [receiptReady, setReceiptReady] = useState(false),
    [receiptBlocked, setReceiptBlocked] = useState(true);
  const [restrictionChecked, setRestrictionChecked] = useState(false);
  const restricted = comment.marks.claim?.reason === "restricted";
  const receipt = useRef<CommentReceipt | null>(null);
  const fence = useRef(new InboxFence()),
    active = useRef(false),
    callbacks = useRef({ onSent, onDenied });
  callbacks.current = { onSent, onDenied };
  const allowed = permitted(store, "inbox:reply");
  const cap =
    capabilities[platform]?.[
      mode === "private" ? "private_reply" : "reply_public"
    ];
  const capable = !!cap && ["ok", "review_required"].includes(cap.state);
  const terminalUnknown =
    receiptBlocked ||
    state === "unknown" ||
    (!!comment.marks.private_reply && commentSendState(comment.marks.private_reply.state) === "unknown");
  useEffect(() => {
    let alive = true;
    void sessionBoundary()
      .then((boundary) => {
        if (!alive) return;
        try {
          receipt.current = new CommentReceipt(
            sessionStorage,
            store.id,
            session,
            boundary,
          );
          setReceiptBlocked(receipt.current.blocked());
        } catch {
          setReceiptBlocked(true);
        }
        setReceiptReady(true);
      })
      .catch(() => {
        if (alive) callbacks.current.onDenied();
      });
    return () => {
      alive = false;
      receipt.current = null;
    };
  }, [store.id, session]);
  const privateReason =
    mode === "private" && !comment.marks.private_reply_available
      ? (comment.marks.private_reply_unavailable_reason ?? "used")
      : "";
  const canPreempt =
    privateReason === "auto_pending_confirm" ||
    error === "auto_pending_confirm";
  const rule =
    privateReason ||
    (mode === "public" && platform === "instagram"
      ? "ig_live_unsupported"
      : "");
  const choices = templates.filter(
    (t) =>
      t.kinds.includes(mode === "private" ? "private_reply" : "public_reply") &&
      (mode !== "public" || t.public_safe),
  );
  const selected = choices.find((t) => t.template_id === template);
  const limit =
    mode === "public"
      ? {
          count: Array.from(text.normalize("NFC")).length,
          max: 300,
          valid:
            text.trim().length > 0 &&
            Array.from(text.normalize("NFC")).length <= 300,
        }
      : textLimit(platform, text);
  useEffect(() => {
    if (!allowed) return;
    fence.current.invalidate(true);
    const ticket = fence.current.begin();
    void inboxRead<{ items: Template[] }>(
      store.id,
      "message-templates",
      ticket.signal,
    )
      .then((v) => {
        if (fence.current.current(ticket) && Array.isArray(v.items))
          setTemplates(v.items);
      })
      .catch((e) => {
        if (
          fence.current.current(ticket) &&
          e instanceof InboxError &&
          [401, 403].includes(e.status)
        )
          callbacks.current.onDenied();
      });
    return () => {
      fence.current.revoke();
    };
  }, [store.id, allowed]);
  const send = async (preempt = false) => {
    if (
      active.current ||
      !receiptReady ||
      terminalUnknown ||
      !allowed ||
      !capable ||
      (restricted && !restrictionChecked) ||
      (!selected && !limit.valid) ||
      (rule && !(canPreempt && preempt))
    )
      return;
    if (!selected && !validCommentText(text)) { setError("invalid_text"); return; }
    if (!receipt.current?.arm()) {
      setReceiptBlocked(true);
      return;
    }
    active.current = true;
    setBusy(true);
    setError("");
    setConfirm(false);
    const ticket = fence.current.begin();
    const body: Record<string, unknown> = selected
      ? {
          template_id: selected.template_id,
          template_version: selected.version,
        }
      : { text: text.normalize("NFC") };
    if (preempt) body.confirm_preempt_auto = true;
    try {
      const value = await inboxWrite<{
        send_state: SendState;
        operation_id: string;
        outbound_id: string;
      }>(
        store.id,
        `live-sessions/${session}/comments/${comment.ref}/${mode}-reply`,
        body,
        crypto.randomUUID(),
        ticket.signal,
      );
      if (!fence.current.current(ticket)) return;
      if (
        !["queued", "sent", "failed", "blocked", "unknown"].includes(
          value.send_state,
        )
      )
        throw new InboxError("retry_later", 503);
      setState(value.send_state);
      // A queued public send has no terminal-state field in A2. Preserve the coarse guard
      // until explicit external verification rather than pretending the operation is final.
      if (["sent", "failed", "blocked"].includes(value.send_state))
        setReceiptBlocked(!receipt.current?.clear());
      else setReceiptBlocked(true);
      setText("");
      setTemplate("");
      callbacks.current.onSent();
    } catch (e) {
      if (!fence.current.current(ticket)) return;
      const code = e instanceof InboxError ? e.code : "retry_later";
      setError(code);
      if (!(e instanceof InboxError) || e.status >= 500) {
        setState("unknown");
        setReceiptBlocked(true);
      } else setReceiptBlocked(!receipt.current?.clear());
      if (e instanceof InboxError && [401, 403].includes(e.status))
        callbacks.current.onDenied();
      if (code === "auto_pending_confirm") setConfirm(true);
      callbacks.current.onSent();
    } finally {
      active.current = false;
      if (fence.current.current(ticket)) setBusy(false);
    }
  };
  const switchMode = (next: "private" | "public") => {
    if (busy || terminalUnknown) return;
    setMode(next);
    setText("");
    setTemplate("");
    setError("");
    setState(null);
    setConfirm(false);
  };
  return (
    <form
      className="comment-reply"
      data-testid="comment-reply"
      onSubmit={(e) => {
        e.preventDefault();
        if (canPreempt) setConfirm(true);
        else void send();
      }}
    >
      <div className="comment-modes">
        <button
          type="button"
          aria-pressed={mode === "private"}
          onClick={() => switchMode("private")}
          disabled={busy || terminalUnknown}
        >
          {c.privateReply}
        </button>
        <button
          type="button"
          aria-pressed={mode === "public"}
          onClick={() => switchMode("public")}
          disabled={busy || terminalUnknown}
        >
          {c.publicReply}
        </button>
      </div>
      <p className="live-helper">{mode === "private" ? c.one : c.publicRule}</p>
      <p className="live-helper">{c.window}</p>
      {cap?.state === "review_required" && <p>{c.testOnly}</p>}
      {!allowed ? (
        <p>{c.permission}</p>
      ) : !capable ? (
        <p>
          {commentReason(
            locale,
            !cap || cap.state === "unknown" ? "unknown_capability" : cap.state,
          )}
        </p>
      ) : null}
      {rule && (
        <p role="status" data-testid="comment-rule">
          {commentReason(locale, rule)}
        </p>
      )}
      {restricted && (
        <label className="comment-restriction">
          <span>{c.restrictedWarning}</span>
          <span>
            <input
              type="checkbox"
              checked={restrictionChecked}
              onChange={(e) => setRestrictionChecked(e.target.checked)}
            />
            {c.reviewSend}
          </span>
        </label>
      )}
      {receiptReady && terminalUnknown && (
        <p role="alert" data-testid="comment-unknown">
          {c.verify}
        </p>
      )}
      {receiptReady && receiptBlocked && !busy && (
        <button
          type="button"
          data-testid="comment-verified"
          onClick={() => {
            if (window.confirm(c.confirmVerified) && receipt.current?.clear()) {
              setReceiptBlocked(false);
              setState(null);
              setError("");
              setText("");
              setTemplate("");
            }
          }}
        >
          {c.verified}
        </button>
      )}
      {state && (
        <p role="status" data-testid="comment-send-state">
          {c[state]}
        </p>
      )}
      {error && (
        <p role="alert" data-testid="comment-reply-error">
          {commentReason(locale, error)}
        </p>
      )}
      <fieldset
        disabled={
          !allowed ||
          !capable ||
          busy ||
          terminalUnknown ||
          (!!rule && !canPreempt) ||
          state === "queued" ||
          state === "sent"
        }
      >
        <label>
          {c.template}
          <select
            data-testid="comment-template"
            value={template}
            onChange={(e) => {
              setTemplate(e.target.value);
              setText("");
            }}
          >
            <option value="">{c.freeText}</option>
            {choices.map((t) => (
              <option key={t.template_id} value={t.template_id}>
                {t.name}
              </option>
            ))}
          </select>
        </label>
        {!selected && (
          <label>
            {c.text}
            <textarea
              data-testid="comment-reply-text"
              value={text}
              onChange={(e) => setText(e.target.value)}
              rows={3}
            />
            <span className="live-helper">
              {limit.count}/{limit.max}
            </span>
          </label>
        )}
        <button
          type="submit"
          className="primary"
          data-testid="comment-send"
          disabled={
            (!selected && !limit.valid) || (restricted && !restrictionChecked)
          }
        >
          {busy ? c.sending : c.send}
        </button>
      </fieldset>
      {confirm && (
        <div role="alertdialog" aria-label={c.confirm}>
          <p>{c.preempt}</p>
          <button type="button" disabled={busy} onClick={() => void send(true)}>
            {c.confirm}
          </button>
          <button type="button" onClick={() => setConfirm(false)}>
            {c.cancel}
          </button>
        </div>
      )}
    </form>
  );
}
