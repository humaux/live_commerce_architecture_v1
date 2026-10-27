// Purpose: Owns ConfirmDialog, the DS-2 second-step confirmation (native modal dialog, optional typed-confirm phrase or caller fields) for refunds, deletes, bulk-over-all-results, ending a live and the re-auth step.
// Depends on: React, ./dialog.module.css, ./Controls (Button), ./logic (typedConfirmMatches); no API or translation.
// Used by: packages/ui/src/ds/index.ts; apps/admin/components/UiGallery.tsx; apps/admin/components/ReauthDialog.tsx (children = DS Fields).
"use client";
import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import s from "./dialog.module.css";
import { Button } from "./Controls";
import { typedConfirmMatches } from "./logic";

export type TypedConfirm = {
  /** The word the operator must type, e.g. the store name or a fixed confirm word. */
  phrase: string;
  /** "Type {phrase} to confirm" in the caller's language (the phrase is shown separately in <code>). */
  label: string;
  /** Why Confirm is disabled until the phrase matches. */
  mismatchReason: string;
};

/**
 * A modal confirmation. Escape and Cancel call onCancel; the backdrop does not dismiss (a destructive step is never one stray click).
 * With `typed`, Confirm stays disabled (with its reason) until the phrase matches. `children` (DS Fields/Alerts) render in the body.
 * Focus starts on the first input in the body, or on Cancel without one; it stays trapped inside and returns to the opener on close.
 */
export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel,
  cancelLabel,
  tone = "danger",
  typed,
  busy = false,
  busyLabel,
  onConfirm,
  onCancel,
  children,
}: {
  open: boolean;
  title: string;
  description?: string;
  confirmLabel: string;
  cancelLabel: string;
  tone?: "danger" | "primary";
  typed?: TypedConfirm;
  busy?: boolean;
  busyLabel?: string;
  onConfirm: () => void;
  onCancel: () => void;
  children?: ReactNode;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const restore = useRef<HTMLElement | null>(null);
  useEffect(() => {
    const node = dialog.current;
    if (!node) return;
    if (open && !node.open) {
      restore.current = document.activeElement as HTMLElement | null;
      node.showModal();
    }
    if (!open && node.open) { node.close(); restore.current?.focus(); }
  }, [open]);
  return (
    <dialog
      ref={dialog}
      className={s.dialog}
      aria-labelledby={titleId}
      onKeyDown={(event) => {
        if (event.key !== "Tab") return;
        const nodes = Array.from(event.currentTarget.querySelectorAll<HTMLElement>('button,input,a[href]')).filter(n => n.getClientRects().length);
        const first = nodes[0]; const last = nodes.at(-1);
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }}
      onCancel={(event) => {
        event.preventDefault();
        if (!busy) onCancel();
      }}
    >
      {open && <Body {...{ titleId, title, description, confirmLabel, cancelLabel, tone, typed, busy, busyLabel, onConfirm, onCancel }}>{children}</Body>}
    </dialog>
  );
}

function Body({
  titleId,
  title,
  description,
  confirmLabel,
  cancelLabel,
  tone,
  typed,
  busy,
  busyLabel,
  onConfirm,
  onCancel,
  children,
}: {
  titleId: string;
  title: string;
  description?: string;
  confirmLabel: string;
  cancelLabel: string;
  tone: "danger" | "primary";
  typed?: TypedConfirm;
  busy: boolean;
  busyLabel?: string;
  onConfirm: () => void;
  onCancel: () => void;
  children?: ReactNode;
}) {
  const [text, setText] = useState("");
  const inputId = useId();
  const cancel = useRef<HTMLButtonElement>(null);
  const body = useRef<HTMLDivElement>(null);
  useEffect(() => {
    (body.current?.querySelector("input") ?? cancel.current)?.focus();
  }, []);
  const matches = !typed || typedConfirmMatches(text, typed.phrase);
  return (
    <>
      <div ref={body} className={s.body}>
        <h2 id={titleId} className={s.title}>
          {title}
        </h2>
        {description && <p className={s.text}>{description}</p>}
        {typed && (
          <div className={s.typed}>
            <label htmlFor={inputId}>
              {typed.label} <code>{typed.phrase}</code>
            </label>
            <input id={inputId} className={s.input} value={text} autoComplete="off" onChange={(event) => setText(event.target.value)} />
          </div>
        )}
        {children}
      </div>
      <div className={s.foot}>
        {/* The block must not depend on the optional copy: busy swallows Cancel even when busyLabel (the visible reason) is absent. */}
        <Button
          ref={cancel}
          onClick={() => {
            if (!busy) onCancel();
          }}
          disabledReason={busy ? busyLabel : undefined}
        >
          {cancelLabel}
        </Button>
        <Button
          variant={tone}
          loading={busy}
          loadingLabel={busyLabel}
          disabledReason={!matches && typed ? typed.mismatchReason : undefined}
          onClick={onConfirm}
        >
          {confirmLabel}
        </Button>
      </div>
    </>
  );
}
