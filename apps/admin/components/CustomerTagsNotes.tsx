// Purpose: customer notes with explicit add/edit/delete, pagination and buyer-export privacy notice.
// Depends on: W6-01B exact notes BFF -> Go customer_tags.go, fenced client/write coordinator, Store permissions and shared Taipei time formatter. lib/session-events (global logout on an unauthorized read).
// Used by: CustomerTags; bodies live only in mounted component memory and are never logged or stored.
"use client";
import { useEffect, useId, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "../lib/model";
import type { CustomerDetail, Note } from "../lib/customers-model";
import { customerTagsCopy } from "../lib/customer-tags-copy";
import { readTagData } from "../lib/customer-tags-client";
import { signalLogout } from "../lib/session-events";
import { parseDeletedNote, parseNotePage, parseNoteRecord, validNoteBody } from "../lib/customer-tags-model";
import type { useTagWrite } from "../lib/customer-tags-write";
import { displayTime } from "../lib/orders-model";

/** Notes use server pages and versions; only privacy holders or proven authors see edit/delete controls. */
export function CustomerNotes({ locale, store, detail, boundary, write, refresh, reloadVersion = 0 }: {
  locale: Locale; store: Store; detail: CustomerDetail; boundary: string;
  write: ReturnType<typeof useTagWrite>; refresh: () => Promise<void>; reloadVersion?: number;
}) {
  const c = customerTagsCopy[locale]; const inputID = useId();
  const [notes, setNotes] = useState(detail.notes);
  const [after, setAfter] = useState("");
  const [reading, setReading] = useState(true);
  const [readError, setReadError] = useState(false);
  const [tick, setTick] = useState(0);
  const [body, setBody] = useState("");
  const [editing, setEditing] = useState<Note | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const canWrite = detail.active && store.permissions?.includes("customers:write") === true;
  const privacy = store.permissions?.includes("customers:privacy") === true;
  const active = useRef<AbortController | null>(null);
  const loading = useRef(false);
  const mounted = useRef(true);
  const generation = useRef(0);
  const explicitReload = useRef(reloadVersion);
  const resource = `customers/${detail.customer_id}/notes`;

  useEffect(() => {
    // Display fresh server facts, but keep an unsaved draft and its ORIGINAL edit version.
    // Unrelated tag saves also refresh detail.notes; those must neither lose nor silently rebase a draft.
    setNotes(detail.notes);
  }, [detail.notes]);

  useEffect(() => {
    mounted.current = true; generation.current++; active.current?.abort();
    const controller = new AbortController(); active.current = controller;
    loading.current = true; setReading(true); setReadError(false); setAfter("");
    void readTagData(store.id, `${resource}?limit=50`, boundary, parseNotePage, controller.signal)
      .then((page) => { if (!controller.signal.aborted) { setNotes(page.items); setAfter(page.next_cursor); } })
      .catch((e) => { if (!controller.signal.aborted) { if (authLost(e)) dropPrivate(e); setReadError(true); } })
      .finally(() => { if (!controller.signal.aborted) { loading.current = false; setReading(false); } });
    return () => { mounted.current = false; generation.current++; active.current?.abort(); loading.current = false; };
  }, [store.id, resource, boundary, tick, detail.notes]);

  // A read refused for authorization (signed out, boundary changed, permission or customer gone) must not leave private
  // note bodies or a draft on screen until an unrelated navigation (Codex review P2, PR #3); transient failures keep them.
  const authLost = (e: unknown) => ["unauthorized", "forbidden", "not_found"].includes(String((e as { message?: unknown } | null)?.message));
  // "unauthorized" also ends the session for the whole page (global logout lifecycle -> the guarded customer read clears).
  const dropPrivate = (e: unknown) => { setNotes([]); setAfter(""); setBody(""); setEditing(null); setDeleting(null);
    if (String((e as { message?: unknown } | null)?.message) === "unauthorized") signalLogout(); };

  async function more() {
    if (!after || loading.current || write.locked) return;
    loading.current = true; setReading(true); setReadError(false);
    const epoch = generation.current;
    const controller = new AbortController(); active.current = controller;
    try {
      const page = await readTagData(store.id, `${resource}?limit=50&after=${after}`, boundary, parseNotePage, controller.signal);
      if (!mounted.current || controller.signal.aborted || generation.current !== epoch) return;
      if (page.next_cursor === after || notes.length + page.items.length > 200) throw new Error("unavailable");
      const ids = new Set(notes.map((n) => n.id));
      // Paging can shift when another staff member writes; refuse overlap and request a fresh page.
      if (page.items.some((n) => ids.has(n.id))) throw new Error("unavailable");
      setNotes((old) => [...old, ...page.items]); setAfter(page.next_cursor);
    } catch (e) { if (mounted.current && !controller.signal.aborted && generation.current === epoch) { if (authLost(e)) dropPrivate(e); setReadError(true); } }
    finally { if (mounted.current && !controller.signal.aborted && generation.current === epoch) { loading.current = false; setReading(false); } }
  }
  useEffect(() => () => active.current?.abort(), []);
  async function changed() {
    setBody(""); setEditing(null); setDeleting(null);
    await refresh(); if (mounted.current) setTick((v) => v + 1);
  }
  const reload = () => { setEditing(null); setBody(""); setDeleting(null); setTick((v) => v + 1); };
  useEffect(() => {
    if (explicitReload.current === reloadVersion) return;
    explicitReload.current = reloadVersion;
    // Only the user's explicit Refresh resets the old draft; passive parent reads do not.
    reload();
  }, [reloadVersion]);
  const stamp = (value: string) => displayTime(locale, value);
  return <section data-testid="customer-notes" aria-labelledby="ct-notes-title">
    <h2 id="ct-notes-title">{c.notes}</h2>
    <p data-testid="note-privacy-hint" id={`${inputID}-privacy`}>{c.notePrivacy}</p>
    {canWrite && !privacy && <p>{c.ownNotesOnly}</p>}
    {!canWrite && <p>{store.permissions ? c.readOnly : c.permissionUnknown}</p>}
    {canWrite && <form className="ct-note-form" onSubmit={(e) => { e.preventDefault(); if (!validNoteBody(body)) return;
      write.run(editing ? "PATCH" : "POST", editing ? `${resource}/${editing.id}` : resource,
        editing ? { body, version: editing.version } : { body }, parseNoteRecord, async () => {
          await changed(); write.setNotice(editing ? c.noteUpdated : c.noteCreated);
        });
    }}>
      <label htmlFor={inputID}>{editing ? c.noteEdit : c.noteAdd}</label>
      <textarea id={inputID} rows={4} value={body} onChange={(e) => setBody(e.target.value)} disabled={write.locked}
        aria-describedby={`${inputID}-privacy ${inputID}-hint`} />
      <small id={`${inputID}-hint`}>{c.noteHint} {Array.from(body).length}/1000</small>
      <div className="ct-actions"><button disabled={write.locked || !validNoteBody(body) || (!editing && notes.length >= 200)}>{editing ? c.noteSave : c.noteAdd}</button>
        {editing && <button type="button" disabled={write.locked} onClick={() => { setEditing(null); setBody(""); }}>{c.noteCancel}</button>}</div>
    </form>}
    {readError && <div role="alert"><p>{c.unavailable}</p><button type="button" disabled={write.locked} onClick={after ? () => { void more(); } : reload}>{c.retry}</button>
      {after && <button type="button" disabled={write.locked} onClick={reload}>{c.refresh}</button>}</div>}
    {reading && <p role="status">{c.loading}</p>}
    <ul className="ct-notes">{notes.map((note) => <li key={note.id}>
      <div className="ct-note-heading"><span>{c.noteAuthor}: {c.staffMember}</span>
        <time dateTime={note.created_at}>{stamp(note.created_at)}</time></div>
      {note.edited_at && <small>{c.noteEdited} <time dateTime={note.edited_at}>{stamp(note.edited_at)}</time></small>}
      <p className="ct-note-body">{note.body}</p>
      {canWrite && (privacy || note.own) && <div className="ct-actions">
        <button type="button" disabled={write.locked} onClick={() => { setEditing(note); setBody(note.body); setDeleting(null); }}>{c.noteEdit}</button>
        <button type="button" disabled={write.locked} onClick={() => setDeleting(note.id)}>{c.noteDelete}</button>
      </div>}
      {deleting === note.id && <div className="ct-confirm" role="group" aria-label={c.noteDeleteConfirm}>
        <p>{c.noteDeleteConfirm}</p><div className="ct-actions">
          <button type="button" disabled={write.locked} onClick={() => write.run("DELETE", `${resource}/${note.id}`, undefined, parseDeletedNote,
            async () => { await changed(); write.setNotice(c.noteDeleted); })}>{c.noteDelete}</button>
          <button type="button" disabled={write.locked} onClick={() => setDeleting(null)}>{c.noteCancel}</button>
        </div></div>}
    </li>)}</ul>
    {!reading && !readError && notes.length === 0 && <p>{c.notesEmpty}</p>}
    {after && <button type="button" disabled={reading || write.locked} onClick={() => { void more(); }}>{c.notesMore}</button>}
  </section>;
}
