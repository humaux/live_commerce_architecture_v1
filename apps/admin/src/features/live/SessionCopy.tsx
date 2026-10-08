// Purpose: A5 copy action for the existing session list without introducing a second draft writer.
// Depends on: validated command journal/fence hook, existing Draft/copy DTO and Next router.
// Used by: Studio's selected-session panel; Go copies the source and offers in one transaction.
"use client";
import { useEffect, useState, type RefObject } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { Draft } from "@/lib/studio-model";
import { liveRequest } from "./command-journal";
import type { CopyResult } from "./console-model";
import { useLiveCommand } from "./use-live-workspace";
import { workspaceCopy } from "./workspace-copy";

/** Copies the selected source using its planning CAS and one stable command key. */
export function SessionCopy({ locale, store, draft, boundary, disabled, refresh, navigationGuard }: {
  locale: Locale; store: string; draft: Draft; boundary: string; disabled: boolean; refresh: () => void; navigationGuard: RefObject<() => boolean>;
}) {
  const c = workspaceCopy[locale], router = useRouter();
  const copied = (result: CopyResult) => router.push(`/${locale}/studio/${result.conflicts.length ? "claims" : "console"}?store=${store}&scene=${result.session.session_id}`);
  const command = useLiveCommand(`${store}:${draft.session_id}`, boundary, refresh, (request, value) => { if (request.path.endsWith("/copy")) copied(value as CopyResult); });
  const [open, setOpen] = useState(false), [title, setTitle] = useState(draft.title);
  // Follow refreshed source names only while closed; never erase an open edit or remount an UNKNOWN owner.
  useEffect(() => { if (!open) setTitle(draft.title); }, [draft.title, open]);
  useEffect(() => {
    navigationGuard.current = () => {
      if ((command.busy || command.canRetry) && !window.confirm(c.leavePending)) return false;
      command.invalidate(); return true;
    };
    return () => { navigationGuard.current = () => true; };
  }, [navigationGuard, command.busy, command.canRetry, command.invalidate, c.leavePending]);
  return <section className="live-copy-section">
    <button type="button" data-testid="live-copy-session" disabled={disabled || command.blocked} onClick={() => setOpen(true)}>{c.copyLast}</button>
    {open && <form className="live-copy-form" onSubmit={(event) => { event.preventDefault(); void command.run<CopyResult>(liveRequest(store, draft.session_id, "copy", "POST", { title: title.trim(), scheduled_at: null, expected_version: draft.version }), (result) => {
      copied(result);
    }); }}>
      <label>{c.name}<input value={title} maxLength={200} onChange={(event) => setTitle(event.target.value)} /></label>
      <button type="submit" disabled={disabled || command.blocked || !title.trim()}>{c.confirmCopy}</button>
      <button type="button" disabled={command.busy} onClick={() => setOpen(false)}>{c.cancel}</button>
    </form>}
    {command.error && <p role="alert">{command.error === "uncertain" ? c.uncertain : command.error === "recovery" ? c.recovery : command.error === "conflict" ? c.conflict : c.failed}</p>}
    {command.canRetry && <button type="button" onClick={() => void command.retry()}>{c.retry}</button>}
  </section>;
}
