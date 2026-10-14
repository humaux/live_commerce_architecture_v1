// Purpose: Owns the DS-1 state blocks (Empty, Error, Permission, Blocked: each with reason and next step) and the Dropzone that shows format and size limits before any upload.
// Depends on: react, ./states.module.css, ./Controls (Button), ./logic (fileProblem); no API, routing or translation.
// Used by: packages/ui/src/ds/index.ts; apps/admin/components/UiGallery.tsx.
"use client";
import { useId, useRef, useState, type DragEvent, type ReactNode } from "react";
import s from "./states.module.css";
import { Button } from "./Controls";
import { fileProblem, type FileProblem, type FileRule } from "./logic";

function Block({
  kind,
  glyph,
  title,
  text,
  next,
  nextLabel,
  actions,
}: {
  kind: "empty" | "error" | "permission" | "blocked";
  glyph: string;
  title: ReactNode;
  text?: ReactNode;
  next?: ReactNode;
  nextLabel?: string;
  actions?: ReactNode;
}) {
  return (
    <div className={s.state} data-kind={kind} role={kind === "error" ? "alert" : undefined}>
      <span className={s.stateIcon} aria-hidden="true">{glyph}</span>
      <h3 className={s.stateTitle}>{title}</h3>
      {text && <p className={s.stateText}>{text}</p>}
      {next && (
        <p className={s.stateNext}>
          {nextLabel && <span className={s.stateNextLabel}>{nextLabel} </span>}
          {next}
        </p>
      )}
      {actions && <div className={s.stateActions}>{actions}</div>}
    </div>
  );
}

/** Nothing here yet. Say what this list is for and offer the first action. */
export function EmptyState({ title, description, action }: { title: ReactNode; description?: ReactNode; action?: ReactNode }) {
  return <Block kind="empty" glyph="○" title={title} text={description} actions={action} />;
}

/** The load failed (role="alert"). Show what failed and give a retry; the retry button is rendered only when both props are given. */
export function ErrorState({
  title,
  detail,
  onRetry,
  retryLabel,
  actions,
}: {
  title: ReactNode;
  detail?: ReactNode;
  onRetry?: () => void;
  retryLabel?: string;
  actions?: ReactNode;
}) {
  return (
    <Block
      kind="error"
      glyph="!"
      title={title}
      text={detail}
      actions={
        <>
          {onRetry && retryLabel && (
            <Button variant="primary" onClick={onRetry}>
              {retryLabel}
            </Button>
          )}
          {actions}
        </>
      }
    />
  );
}

/** The signed-in staff role cannot see this. Names the missing permission as `reason` and who can grant it as `nextStep`. */
export function PermissionState({
  title,
  reason,
  nextStep,
  nextLabel,
  action,
}: {
  title: ReactNode;
  reason: ReactNode;
  nextStep?: ReactNode;
  nextLabel?: string;
  action?: ReactNode;
}) {
  return <Block kind="permission" glyph="⚿" title={title} text={reason} next={nextStep} nextLabel={nextLabel} actions={action} />;
}

/** A prerequisite is missing (no public URL, not connected, not opened to this store). `reason` and `nextStep` are both required: a block never ends in a bare 404. */
export function BlockedState({
  title,
  reason,
  nextStep,
  nextLabel,
  action,
}: {
  title: ReactNode;
  reason: ReactNode;
  nextStep: ReactNode;
  nextLabel?: string;
  action?: ReactNode;
}) {
  return <Block kind="blocked" glyph="◼" title={title} text={reason} next={nextStep} nextLabel={nextLabel} actions={action} />;
}

export type DropzoneRejection = { name: string; problem: FileProblem };

/**
 * File drop area. `hint` (format + size + recommended dimensions) is always on screen, before anything is picked.
 * Picked or dropped files are checked against `rule`; rejected ones are listed with `errors[problem]`, accepted ones go to `onFiles`.
 */
export function Dropzone({
  title,
  hint,
  rule,
  errors,
  browseLabel,
  libraryLabel,
  onLibrary,
  onFiles,
  multiple = true,
  inputLabel,
}: {
  title: string;
  hint: string;
  rule: FileRule;
  errors: Record<FileProblem, string>;
  browseLabel: string;
  libraryLabel?: string;
  onLibrary?: () => void;
  onFiles: (files: File[]) => void;
  multiple?: boolean;
  /** Accessible name of the (hidden) native input; the Browse button is the visible way to open it. */
  inputLabel: string;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  const [rejected, setRejected] = useState<DropzoneRejection[]>([]);
  const hintId = useId();
  const take = (list: FileList | null) => {
    const files = Array.from(list ?? []);
    const ok: File[] = [];
    const bad: DropzoneRejection[] = [];
    for (const file of files) {
      const problem = fileProblem(file, rule);
      if (problem) bad.push({ name: file.name, problem });
      else ok.push(file);
    }
    setRejected(bad);
    if (ok.length) onFiles(multiple ? ok : ok.slice(0, 1));
  };
  const drop = (event: DragEvent) => {
    event.preventDefault();
    setOver(false);
    take(event.dataTransfer.files);
  };
  return (
    <div
      className={s.drop}
      data-over={over}
      role="group"
      aria-label={title}
      aria-describedby={hintId}
      onDragOver={(event) => {
        event.preventDefault();
        setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={drop}
    >
      <p className={s.dropTitle}>{title}</p>
      <p id={hintId} className={s.dropHint}>
        {hint}
      </p>
      <div className={s.dropActions}>
        <Button variant="primary" onClick={() => input.current?.click()}>
          {browseLabel}
        </Button>
        {libraryLabel && onLibrary && <Button onClick={onLibrary}>{libraryLabel}</Button>}
      </div>
      <input
        ref={input}
        type="file"
        hidden
        multiple={multiple}
        aria-label={inputLabel}
        accept={rule.extensions.map((e) => `.${e}`).join(",")}
        onChange={(event) => {
          take(event.currentTarget.files);
          event.currentTarget.value = "";
        }}
      />
      {rejected.length > 0 && (
        <ul className={s.dropErrors} role="alert">
          {rejected.map((r, i) => (
            <li key={`${i}-${r.name}`}>
              {r.name}: {errors[r.problem]}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
