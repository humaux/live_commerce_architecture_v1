// Purpose: Owns ConfirmDialog, the DS-1 second-step confirmation (native modal dialog, optional typed-confirm phrase) for refunds, deletes, bulk-over-all-results and ending a live.
// Depends on: react, ./dialog.module.css, ./Controls (Button), ./logic (typedConfirmMatches); no API or translation.
// Used by: packages/ui/src/ds/index.ts; apps/admin/components/UiGallery.tsx.
"use client";
import { useEffect, useId, useRef, useState } from "react";
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
 * With `typed`, Confirm stays disabled (with its reason) until the phrase matches. Focus starts on the input, or on Cancel without one.
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
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  useEffect(() => {
    const node = dialog.current;
    if (!node) return;
    if (open && !node.open) node.showModal();
    if (!open && node.open) node.close();
  }, [open]);
  return (
    <dialog
      ref={dialog}
      className={s.dialog}
      aria-labelledby={titleId}
      onCancel={(event) => {
        event.preventDefault();
        if (!busy) onCancel();
      }}
    >
      {open && <Body {...{ titleId, title, description, confirmLabel, cancelLabel, tone, typed, busy, busyLabel, onConfirm, onCancel }} />}
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
}) {
  const [text, setText] = useState("");
  const inputId = useId();
  const cancel = useRef<HTMLButtonElement>(null);
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    (input.current ?? cancel.current)?.focus();
  }, []);
  const matches = !typed || typedConfirmMatches(text, typed.phrase);
  return (
    <>
      <div className={s.body}>
        <h2 id={titleId} className={s.title}>
          {title}
        </h2>
        {description && <p className={s.text}>{description}</p>}
        {typed && (
          <div className={s.typed}>
            <label htmlFor={inputId}>
              {typed.label} <code>{typed.phrase}</code>
            </label>
            <input ref={input} id={inputId} className={s.input} value={text} autoComplete="off" onChange={(event) => setText(event.target.value)} />
          </div>
        )}
      </div>
      <div className={s.foot}>
        <Button ref={cancel} onClick={onCancel} disabledReason={busy ? busyLabel : undefined}>
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
