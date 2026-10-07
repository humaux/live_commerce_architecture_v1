// Purpose: W6-U1 customer badges, tag catalogue management and customer tag selection.
// Depends on: customer-tags client/model/copy -> exact BFF leaves -> Go customer_tags.go; Store permissions.
// Used by: Customers.tsx and CustomerDetail.tsx; server re-reads own every displayed saved fact.
"use client";
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "../lib/model";
import type { CustomerDetail, Tag } from "../lib/customers-model";
import { customerTagsCopy } from "../lib/customer-tags-copy";
import { readTagData } from "../lib/customer-tags-client";
import { parseDeletedTag, parseTagCatalog, parseTagRecord, parseTagSet, validTagName, type TagRecord } from "../lib/customer-tags-model";
import { useTagWrite } from "../lib/customer-tags-write";
import { CustomerNotes } from "./CustomerTagsNotes";
import { TagFields, TagWriteStatus } from "./CustomerTagsForms";
import "./customer-tags.css";

/** Render the frozen customer tag projection, using names plus colour rather than colour alone. */
export function TagBadges({ tags, locale = "en" }: { tags: Tag[]; locale?: Locale }) {
  // No aria-label: it is prohibited on a generic span and, inside the editor's <label>, it replaced every checkbox's
  // accessible name with "Tags" (CI 37616676597). Context comes from the visible "Tags" heading / column.
  return <span className="ct-badges">
    {tags.map((tag) => <span className={`ct-badge ct-${tag.color}`} key={tag.id}>{tag.name}</span>)}
  </span>;
}

function useCatalogue(store: string, boundary: string) {
  const [catalog, setCatalog] = useState<TagRecord[] | null>(null);
  const [error, setError] = useState(false);
  const [tick, setTick] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setCatalog(null); setError(false);
    void readTagData(store, "customers/tags", boundary, parseTagCatalog, controller.signal)
      .then((v) => { if (!controller.signal.aborted) setCatalog(v.items); })
      .catch(() => { if (!controller.signal.aborted) setError(true); });
    return () => controller.abort();
  }, [store, boundary, tick]);
  return { catalog, error, reload: () => setTick((v) => v + 1) };
}

/** Open the store catalogue dialog; explicit Store permissions control mutation affordances. */
export function CustomerTagManager(props: { locale: Locale; store: Store; boundary: string; onChanged?: () => void }) {
  const [open, setOpen] = useState(false);
  const c = customerTagsCopy[props.locale];
  return <>
    <button type="button" data-testid="customers-tag-manage" onClick={() => setOpen(true)}>{c.manage}</button>
    {open && <TagManagerDialog key={`${props.store.id}|${props.boundary}`} {...props} close={() => setOpen(false)} />}
  </>;
}

function TagManagerDialog({ locale, store, boundary, onChanged, close }: {
  locale: Locale; store: Store; boundary: string; onChanged?: () => void; close: () => void;
}) {
  const c = customerTagsCopy[locale]; const read = useCatalogue(store.id, boundary);
  const write = useTagWrite(store.id, boundary);
  const dialog = useRef<HTMLDialogElement>(null);
  const [editing, setEditing] = useState<TagRecord | null>(null);
  const [deleting, setDeleting] = useState<TagRecord | null>(null);
  const [name, setName] = useState(""); const [color, setColor] = useState<Tag["color"]>("gray");
  const canWrite = store.permissions?.includes("customers:write") === true;
  useEffect(() => { dialog.current?.showModal(); }, []);
  const reset = () => { setEditing(null); setName(""); setColor("gray"); };
  const changed = async () => { reset(); setDeleting(null); read.reload(); onChanged?.(); };
  return <dialog ref={dialog} data-testid="tag-manage-dialog" className="ct-dialog" aria-labelledby="ct-manager-title"
    onCancel={(e) => { if (write.locked) e.preventDefault(); }} onClose={close}>
    <div className="ct-heading"><h2 id="ct-manager-title">{c.manageTitle}</h2>
      <button type="button" onClick={() => dialog.current?.close()} disabled={write.locked}>{c.close}</button></div>
    <p>{c.manageHint}</p>
    {!canWrite && <p>{store.permissions ? c.readOnly : c.permissionUnknown}</p>}
    {read.error ? <div role="alert"><p>{c.unavailable}</p><button type="button" onClick={read.reload}>{c.retry}</button></div> :
      read.catalog === null ? <p role="status">{c.loading}</p> : <ul className="ct-catalog">
        {read.catalog.map((tag) => <li key={tag.id}>
          <div><TagBadges tags={[tag]} locale={locale} /><small>{c.usedBy(tag.customers_count)}</small></div>
          {canWrite && <div className="ct-actions">
            <button type="button" disabled={write.locked} onClick={() => { setEditing(tag); setName(tag.name); setColor(tag.color); setDeleting(null); }}>{c.rename} · {c.colorLabel}</button>
            <button type="button" disabled={write.locked} onClick={() => setDeleting(tag)}>{c.remove}</button>
          </div>}
        </li>)}
        {read.catalog.length === 0 && <li>{c.editorEmpty}</li>}
      </ul>}
    {deleting && <div className="ct-confirm" role="group" aria-label={c.removeConfirm}>
      <p>{c.removeConfirm} <strong>{deleting.name}</strong></p>
      <div className="ct-actions"><button type="button" disabled={write.locked} onClick={() => {
        write.run("DELETE", `customers/tags/${deleting.id}`, undefined, parseDeletedTag, async (v) => {
          await changed(); write.setNotice(c.detached(v.detached));
        });
      }}>{c.remove}</button><button type="button" disabled={write.locked} onClick={() => setDeleting(null)}>{c.cancel}</button></div>
    </div>}
    {canWrite && <form onSubmit={(e) => { e.preventDefault(); if (!validTagName(name) || !read.catalog) return;
      write.run(editing ? "PATCH" : "POST", editing ? `customers/tags/${editing.id}` : "customers/tags", { name: name.normalize("NFC"), color }, parseTagRecord,
        async () => { await changed(); write.setNotice(editing ? c.updated : c.created); });
    }}>
      <h3>{editing ? c.rename : c.createTitle}</h3>
      <TagFields c={c} name={name} color={color} disabled={write.locked} setName={setName} setColor={setColor} />
      <div className="ct-actions"><button disabled={write.locked || !validTagName(name) || !read.catalog || (!editing && read.catalog.length >= 100)}>{editing ? c.save : c.create}</button>
        {editing && <button type="button" disabled={write.locked} onClick={reset}>{c.cancel}</button>}</div>
    </form>}
    <TagWriteStatus c={c} write={write} refresh={read.reload} />
  </dialog>;
}

/** Customer tag editor and notes; read-only when permission or active customer authority is unavailable. */
export function CustomerTags(props: { locale: Locale; store: Store; detail: CustomerDetail; boundary: string; onChanged: () => Promise<boolean> }) {
  return <CustomerTagBody key={`${props.store.id}|${props.detail.customer_id}|${props.boundary}`} {...props} />;
}

function CustomerTagBody({ locale, store, detail, boundary, onChanged }: {
  locale: Locale; store: Store; detail: CustomerDetail; boundary: string; onChanged: () => Promise<boolean>;
}) {
  const c = customerTagsCopy[locale]; const read = useCatalogue(store.id, boundary);
  const write = useTagWrite(store.id, boundary);
  const canWrite = store.permissions?.includes("customers:write") === true && detail.active;
  const [selected, setSelected] = useState(detail.tags.map((tag) => tag.id));
  const [newTag, setNewTag] = useState(false);
  const [notesReloadVersion, setNotesReloadVersion] = useState(0);
  const [name, setName] = useState(""); const [color, setColor] = useState<Tag["color"]>("gray");
  useEffect(() => { setSelected(detail.tags.map((tag) => tag.id)); }, [detail.tags_revision]);
  const refresh = async () => { if (!await onChanged()) throw new Error("refresh_required"); read.reload(); };
  const unchanged = selected.length === detail.tags.length && selected.every((id) => detail.tags.some((t) => t.id === id));
  return <div className="customer-tags-notes">
    <section data-testid="customer-tags-editor" aria-labelledby="ct-editor-title">
      <h2 id="ct-editor-title">{c.editorLabel}</h2>
      <TagBadges tags={detail.tags} locale={locale} />
      {!canWrite && <p>{store.permissions ? c.readOnly : c.permissionUnknown}</p>}
      {canWrite && <>
        <p>{c.editorHint}</p>
        {read.error ? <div role="alert"><p>{c.unavailable}</p><button type="button" onClick={read.reload}>{c.retry}</button></div> :
          read.catalog === null ? <p role="status">{c.loading}</p> : <fieldset className="ct-options" disabled={write.locked}>
            <legend>{c.tags} · {c.selectedCount} {selected.length}/20</legend>
            {read.catalog.map((tag) => <label key={tag.id}><input type="checkbox" checked={selected.includes(tag.id)}
              disabled={!selected.includes(tag.id) && selected.length >= 20}
              onChange={(e) => setSelected((old) => e.target.checked ? [...old, tag.id] : old.filter((id) => id !== tag.id))} />
              <TagBadges tags={[tag]} locale={locale} /></label>)}
            {read.catalog.length === 0 && <p>{c.editorEmpty}</p>}
          </fieldset>}
        <div className="ct-actions">
          <button type="button" disabled={write.locked || !read.catalog || unchanged || selected.length > 20} onClick={() => {
            write.run("PUT", `customers/${detail.customer_id}/tags`, { tag_ids: selected, revision: detail.tags_revision }, parseTagSet,
              async () => { await refresh(); write.setNotice(c.editorSaved); });
          }}>{c.editorSave}</button>
          <button type="button" disabled={write.locked || !read.catalog || read.catalog.length >= 100} onClick={() => setNewTag((v) => !v)}>{newTag ? c.cancel : c.newTag}</button>
        </div>
        {newTag && <form onSubmit={(e) => { e.preventDefault(); if (!validTagName(name)) return;
          write.run("POST", "customers/tags", { name: name.normalize("NFC"), color }, parseTagRecord, async () => {
            setName(""); setNewTag(false); await refresh(); write.setNotice(c.created);
          });
        }}><h3>{c.createTitle}</h3><TagFields c={c} name={name} color={color} disabled={write.locked} setName={setName} setColor={setColor} />
          <button disabled={write.locked || !validTagName(name)}>{c.create}</button></form>}
      </>}
    </section>
    <TagWriteStatus c={c} write={write} refresh={() => {
      setNotesReloadVersion((v) => v + 1);
      void refresh().catch(() => write.setNotice(c.refreshRequired));
    }} />
    <CustomerNotes locale={locale} store={store} detail={detail} boundary={boundary} write={write} refresh={refresh} reloadVersion={notesReloadVersion} />
  </div>;
}
