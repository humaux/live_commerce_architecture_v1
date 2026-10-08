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
import type { ConsoleCapabilities } from "@/src/features/live/console-model";

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
    state === "unknown" || comment.marks.private_reply?.state === "unknown";
  const privateReason =
    mode === "private" && !comment.marks.private_reply_available
      ? (comment.marks.private_reply_unavailable_reason ?? "used")
      : "";
  const canPreempt = privateReason === "auto_pending_confirm" || error === "auto_pending_confirm";
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
      terminalUnknown ||
      !allowed ||
      !capable ||
      (!selected && !limit.valid) ||
      (rule && !(canPreempt && preempt))
    )
      return;
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
      setText("");
      setTemplate("");
      callbacks.current.onSent();
    } catch (e) {
      if (!fence.current.current(ticket)) return;
      const code = e instanceof InboxError ? e.code : "retry_later";
      setError(code);
      if (!(e instanceof InboxError) || e.status >= 500) setState("unknown");
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
        <p>{c.capability}</p>
      ) : null}
      {rule && (
        <p role="status" data-testid="comment-rule">
          {commentReason(locale, rule)}
        </p>
      )}
      {terminalUnknown && (
        <p role="alert" data-testid="comment-unknown">
          {c.verify}
        </p>
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
          disabled={!selected && !limit.valid}
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
