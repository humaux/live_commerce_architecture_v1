// Purpose: Owns the DS-1 state blocks (Empty, Error, Permission, Blocked: each with reason and next step) and the Dropzone that shows format and size limits before any upload.
// Depends on: react, ./states.module.css, ./Controls (Button), ./logic (fileProblem); no API, routing or translation.
// Used by: packages/ui/src/ds/index.ts; apps/admin/components/UiGallery.tsx.
"use client";
import { useId, useRef, useState, type DragEvent, type ReactNode } from "react";
import { Icon, type IconName } from "./Icon";
import s from "./states.module.css";
import { Button } from "./Controls";
import { fileProblem, type FileProblem, type FileRule } from "./logic";

function Block({
  kind,
  icon,
  mode = "compact",
  title,
  text,
  next,
  nextLabel,
  actions,
}: {
  kind: "empty" | "error" | "permission" | "blocked";
  icon: IconName;
  mode?: "compact" | "full";
  title: ReactNode;
  text?: ReactNode;
  next?: ReactNode;
  nextLabel?: string;
  actions?: ReactNode;
}) {
  return (
    <div className={s.state} data-kind={kind} data-mode={mode} role={kind === "error" ? "alert" : undefined}>
      <span className={s.stateIcon}><Icon name={icon} size={24} /></span>
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
export function EmptyState({ title, description, action, mode }: { title: ReactNode; description?: ReactNode; action?: ReactNode; mode?: "compact" | "full" }) {
  return <Block kind="empty" icon="empty" mode={mode} title={title} text={description} actions={action} />;
}

/** The load failed (role="alert"). Show what failed and give a retry; the retry button is rendered only when both props are given. */
export function ErrorState({
  title,
  detail,
  onRetry,
  retryLabel,
  actions,
  mode,
}: {
  title: ReactNode;
  detail?: ReactNode;
  onRetry?: () => void;
  retryLabel?: string;
  actions?: ReactNode;
  mode?: "compact" | "full";
}) {
  return (
    <Block
      kind="error"
      icon="danger"
      mode={mode}
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
  mode,
}: {
  title: ReactNode;
  reason: ReactNode;
  nextStep?: ReactNode;
  nextLabel?: string;
  action?: ReactNode;
  mode?: "compact" | "full";
}) {
  return <Block kind="permission" icon="lock" mode={mode} title={title} text={reason} next={nextStep} nextLabel={nextLabel} actions={action} />;
}

/** A prerequisite is missing (no public URL, not connected, not opened to this store). `reason` and `nextStep` are both required: a block never ends in a bare 404. */
export function BlockedState({
  title,
  reason,
  nextStep,
  nextLabel,
  action,
  mode,
}: {
  title: ReactNode;
  reason: ReactNode;
  nextStep: ReactNode;
  nextLabel?: string;
  action?: ReactNode;
  mode?: "compact" | "full";
}) {
  return <Block kind="blocked" icon="alert" mode={mode} title={title} text={reason} next={nextStep} nextLabel={nextLabel} actions={action} />;
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
  disabledReason,
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
  disabledReason?: string;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  const [rejected, setRejected] = useState<DropzoneRejection[]>([]);
  const hintId = useId();
  const take = (list: FileList | null) => {
    if (disabledReason) return;
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
      <Icon name="upload" size={24} />
      <p className={s.dropTitle}>{title}</p>
      <p id={hintId} className={s.dropHint}>
        {hint}
      </p>
      <div className={s.dropActions}>
        <Button variant="primary" disabledReason={disabledReason} onClick={() => input.current?.click()}>
          {browseLabel}
        </Button>
        {libraryLabel && onLibrary && <Button disabledReason={disabledReason} onClick={onLibrary}>{libraryLabel}</Button>}
      </div>
      <input
        ref={input}
        type="file"
        disabled={Boolean(disabledReason)}
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

export type UploadItem = { id: string; name: string; src?: string; status: "uploading" | "error" | "done"; progress: number; error?: string };
/** Controlled upload queue: the caller owns real transport, cancellation and retries; this component never fabricates success. */
export function UploadGrid({ items, label, removeLabel, retryLabel, progressLabel, doneLabel, onRemove, onRetry }: {
  items: readonly UploadItem[]; label: string; removeLabel: (name: string) => string; retryLabel: (name: string) => string;
  progressLabel: (progress: number) => string; doneLabel: string; onRemove: (id: string) => void; onRetry: (id: string) => void;
}) {
  return <ul className={s.uploadGrid} aria-label={label}>{items.map(item => <li key={item.id} className={s.uploadItem}>
    {item.src ? <img className={s.uploadThumb} src={item.src} alt={item.name} width={80} height={80} /> : <span className={s.uploadThumb}><Icon name="image" size={24} /></span>}
    <span className={s.uploadName}>{item.name}</span>
    {item.status === "uploading" && <><progress max={100} value={item.progress} aria-label={progressLabel(item.progress)} /><span role="status">{progressLabel(item.progress)}</span></>}
    {item.status === "error" && <span className={s.uploadError} role="alert">{item.error}</span>}
    {item.status === "done" && <span role="status">{doneLabel}</span>}
    <div className={s.dropActions}>{item.status === "error" && <Button size="sm" onClick={() => onRetry(item.id)}>{retryLabel(item.name)}</Button>}
      <Button size="sm" variant="ghost" aria-label={removeLabel(item.name)} onClick={() => onRemove(item.id)}><Icon name="close" size={16} /></Button></div>
  </li>)}</ul>;
}
