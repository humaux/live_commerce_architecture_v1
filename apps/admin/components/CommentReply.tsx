// Purpose: explicit A4/A5 reply submission with coded rules, capability gates and uncertain-send fencing.
// Depends on: inbox transport, privacy epochs, published templates and server comment marks.
// Used by: CommentStream; never persists a reply body or automatically resends an uncertain command.
// Invariants: buyer/channel draft isolation; one-shot private replies and server window/capability rules remain authoritative.
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
import type { ConsoleCapabilities, ConsoleOffer } from "@/src/features/live/console-model";
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
  offer, drafts,
}: {
  store: Store;
  session: string;
  comment: StreamComment;
  locale: string;
  platform: "facebook" | "instagram";
  capabilities: ConsoleCapabilities;
  onSent: () => void;
  onDenied: () => void;
  offer?: ConsoleOffer; drafts?: Map<string, { private: string; public: string }>;
}) {
  const c = commentCopy(locale);
  const draftText = (kind: "checkout" | "stock" = "checkout") => {
    if (offer?.sold_out) return c.soldDraft;
    if (!offer) return c.questionDraft;
    return (kind === "stock" ? c.stockDraft : c.draft).replace("{product}", offer.product_name).replace("{quantity}", String(comment.marks.claim?.quantity ?? "—")).replace("{stock}", offer.stock.tracked ? String(offer.stock.sellable) : "—");
  };
  // Both audiences use the server-observed sold-out notice; edited drafts stay channel-scoped.
  const publicDraft = offer?.sold_out ? c.soldDraft : c.questionDraft;
  const remember = (value: string, channel: "private" | "public") => {
    if (!drafts) return;
    const prior = drafts.get(comment.ref) ?? { private: draftText(), public: publicDraft };
    drafts.set(comment.ref, { ...prior, [channel]: value });
  };
  const [mode, setMode] = useState<"private" | "public">("private"),
    [text, setText] = useState(drafts ? drafts.get(comment.ref)?.private ?? draftText() : ""),
    [templates, setTemplates] = useState<Template[]>([]),
    [template, setTemplate] = useState("");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [state, setState] = useState<SendState | null>(null),
    [confirm, setConfirm] = useState(false);
  const [receiptReady, setReceiptReady] = useState(false),
    [receiptBlocked, setReceiptBlocked] = useState(true);
  const [restrictionChecked, setRestrictionChecked] = useState(false);
  const [verified, setVerified] = useState(false);
  const restricted = comment.marks.claim?.reason === "restricted";
  const receipt = useRef<CommentReceipt | null>(null);
  const publicReceipt = useRef<CommentReceipt | null>(null);
  const [publicBlocked, setPublicBlocked] = useState(true);
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
    publicBlocked ||
    state === "unknown" ||
    (!!comment.marks.private_reply &&
      commentSendState(comment.marks.private_reply.state) === "unknown");
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
          publicReceipt.current = new CommentReceipt(sessionStorage, store.id, session, boundary, comment.ref);
          setPublicBlocked(publicReceipt.current.blocked());
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
      publicReceipt.current = null;
    };
  }, [store.id, session, comment.ref]);
  const privateReason =
    mode === "private" && !comment.marks.private_reply_available
      ? (comment.marks.private_reply_unavailable_reason === "facts_unavailable"
          ? "comment_facts_unavailable"
          : (comment.marks.private_reply_unavailable_reason ?? "used"))
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
          // template list: 404 can only mean lost scope; the send POST keeps 401/403 (its 404 may be a stale offer)
          [401, 403, 404].includes(e.status)
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
    if (!selected && !validCommentText(text)) {
      setError("invalid_text");
      return;
    }
    if (!receipt.current?.arm()) {
      setReceiptBlocked(true);
      return;
    }
    // Persist before dispatch: A2 exposes only public counts, so it cannot recover queued/UNKNOWN after reload.
    if (mode === "public" && !publicReceipt.current?.arm()) {
      setPublicBlocked(true);
      setReceiptBlocked(!receipt.current.clear());
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
      // queued releases the session fence, never the per-comment fence: the worker is not terminal yet.
      if (mode === "public") setPublicBlocked(
        ["sent", "failed", "blocked"].includes(value.send_state)
          ? !publicReceipt.current?.clear() : true,
      );
      if (value.send_state !== "unknown")
        setReceiptBlocked(!receipt.current?.clear());
      else setReceiptBlocked(true);
      // Keep the exact submitted body only in authorised component memory; never persist it.
      setTemplate("");
      callbacks.current.onSent();
    } catch (e) {
      if (!fence.current.current(ticket)) return;
      const code = e instanceof InboxError ? e.code : "retry_later";
      setError(code);
      if (!(e instanceof InboxError) || e.status >= 500) {
        setState("unknown");
        setReceiptBlocked(true);
      } else {
        setReceiptBlocked(!receipt.current?.clear());
        // A definite command refusal did not queue an external send.
        if (mode === "public") setPublicBlocked(!publicReceipt.current?.clear());
      }
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
    remember(text, mode);
    setMode(next);
    setText(drafts ? drafts.get(comment.ref)?.[next] ?? (next === "private" ? draftText() : publicDraft) : "");
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
      {drafts && <div className="live-quick-replies">
        {!offer?.sold_out && <button type="button" disabled={busy || terminalUnknown || !!rule || state === "queued" || state === "sent"} onClick={() => { const value = mode === "public" ? c.questionDraft : draftText(); setText(value); remember(value, mode); setTemplate(""); }}>{c.checkout}</button>}
        <button type="button" disabled={busy || terminalUnknown || !!rule || state === "queued" || state === "sent"} onClick={() => { const value = draftText("stock"); setText(value); remember(value, mode); setTemplate(""); }}>{c.stockDetails}</button>
      </div>}
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
      {receiptReady && (receiptBlocked || publicBlocked) && !busy && (
        <button
          type="button"
          data-testid="comment-verified"
          onClick={() => {
            if (window.confirm(c.confirmVerified) && receipt.current?.clear() && publicReceipt.current?.clear()) {
              setVerified(true);
              setReceiptBlocked(false);
              setPublicBlocked(false);
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
      {(state || comment.marks.private_reply || comment.marks.public_replies > 0) && (
        <p role="status" data-testid="comment-send-state">
          {state ? state === "sent" && mode === "public" ? c.publicSent : c[state] : comment.marks.private_reply ? commentSendState(comment.marks.private_reply.state) === "blocked" ? c.blockedHelper : c[commentSendState(comment.marks.private_reply.state)] : c.wait}
        </p>
      )}
      {error && (
        <p role="alert" data-testid="comment-reply-error">
          {commentReason(locale, error)}
        </p>
      )}
      {drafts && <div className="comment-reply-target"><strong>{c.target} <span data-private="comment-author">{comment.author_name ?? c.unnamed}</span></strong>{offer && <p>{offer.product_name} · {comment.marks.claim?.quantity} {c.units}</p>}</div>}
      <fieldset
        disabled={
          !allowed ||
          !capable ||
          busy ||
          terminalUnknown ||
          (!!rule && !canPreempt) ||
          state === "queued" ||
          state === "sent" ||
          (drafts !== undefined && comment.marks.public_replies > 0 && !verified)
        }
      >
        <details><summary>{c.advanced}</summary><label>
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
        </details>
        {!selected && (
          <label>
            {c.text}
            <textarea
              data-testid="comment-reply-text"
              aria-label={c.text}
              value={text}
              onChange={(e) => { setText(e.target.value); remember(e.target.value, mode); }}
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
          {busy ? c.sending : terminalUnknown ? c.waitPending : state === "sent" || state === "queued" || (mode === "private" && !!comment.marks.private_reply) || (comment.marks.public_replies > 0 && !verified) ? c.wait : mode === "public" ? c.publicSend : c.send}
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
