// Purpose: A5 copy action for the existing session list without introducing a second draft writer.
// Depends on: console-client copySession, opaque receipt fence hook, existing Draft DTO and Next router.
// Used by: Studio's selected-session panel; Go copies the source and offers in one transaction.
"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { Draft } from "@/lib/studio-model";
import { copySession } from "./console-client";
import { useLiveCommand } from "./use-live-workspace";
import { workspaceCopy } from "./workspace-copy";

/** Copies the selected source using its planning CAS and one stable command key. */
export function SessionCopy({ locale, store, draft, boundary, disabled, refresh }: {
  locale: Locale; store: string; draft: Draft; boundary: string; disabled: boolean; refresh: () => void;
}) {
  const c = workspaceCopy[locale], router = useRouter();
  const command = useLiveCommand(`${store}:${draft.session_id}`, boundary, refresh);
  const [open, setOpen] = useState(false), [title, setTitle] = useState(draft.title);
  return <section className="live-copy-section">
    <button type="button" data-testid="live-copy-session" disabled={disabled || command.blocked} onClick={() => setOpen(true)}>{c.copyLast}</button>
    {open && <form className="live-copy-form" onSubmit={(event) => { event.preventDefault(); void command.run(async (key) => {
      const result = await copySession(store, draft.session_id, { title: title.trim(), scheduled_at: null, expected_version: draft.version }, key, boundary);
      router.push(`/${locale}/studio/${result.conflicts.length ? "claims" : "console"}?store=${store}&scene=${result.session.session_id}`);
    }); }}>
      <label>{c.name}<input value={title} maxLength={200} onChange={(event) => setTitle(event.target.value)} /></label>
      <button type="submit" disabled={disabled || command.blocked || !title.trim()}>{c.confirmCopy}</button>
      <button type="button" disabled={command.busy} onClick={() => setOpen(false)}>{c.cancel}</button>
    </form>}
    {command.error && <p role="alert">{command.error === "uncertain" ? c.uncertain : command.error === "recovery" ? c.recovery : command.error === "conflict" ? c.conflict : c.failed}</p>}
    {command.canRetry && <button type="button" onClick={() => void command.retry()}>{c.retry}</button>}
  </section>;
}
