// Purpose: Own one durable collection creation shared by the quick and manual forms.
// Depends on: React, catalog-v2-client command/send, catalog limits/parser, settings-client session fence and catalog-v2-write message type.
// Used by: CollectionManager; no other resource or write lifecycle is admitted.
// Invariants: persist before send; retry original key/bytes only; unknown survives navigation, denial and reload.
import { useEffect, useRef, useState } from "react";
import { command, send, type Command } from "@/lib/catalog-v2-client";
import { limits, parseCreated, slugPattern, type CollectionSort } from "@/lib/catalog-v2-model";
import { csrfCookie, sessionBoundary } from "@/lib/settings-client";
import type { WriteMessage } from "@/lib/catalog-v2-write";

export type CollectionCreateBody = {
  title: string; description: string; sort_mode: CollectionSort; status: "active" | "hidden";
  slug?: string; membership_mode?: "manual" | "automatic";
  rules?: { match: "all"; conditions: [{ field: "created_at"; operator: "within_days"; days: 7 }] };
};
type Journal = { version: 1; store: string; boundary: string; command: Command };
type View = { scope: string; ready: boolean; pending: Journal | null; message: WriteMessage | null; retry: boolean };
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const storeID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const digest = /^[0-9a-f]{64}$/;
// JSON may escape every UTF-16 unit; include envelope/rule overhead as well as full valid descriptions.
const maxRecord = 12 * (limits.collectionTitle + limits.collectionDescription + limits.slug) + 2048;
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
const keys = (v: Record<string, unknown>, names: string[]) => Object.keys(v).every(k => names.includes(k));

function bodyOf(raw: string): CollectionCreateBody | null {
  try {
    const b: unknown = JSON.parse(raw);
    if (!object(b) || !keys(b, ["title", "description", "sort_mode", "status", "slug", "membership_mode", "rules"]) ||
      typeof b.title !== "string" || !b.title.trim() || Array.from(b.title).length > limits.collectionTitle ||
      typeof b.description !== "string" || Array.from(b.description).length > limits.collectionDescription ||
      typeof b.sort_mode !== "string" || !["manual", "newest", "price_asc", "price_desc"].includes(b.sort_mode) ||
      typeof b.status !== "string" || !["active", "hidden"].includes(b.status)) return null;
    if (Object.hasOwn(b, "slug") && (typeof b.slug !== "string" || b.slug.length > limits.slug || !slugPattern.test(b.slug))) return null;
    if (Object.hasOwn(b, "membership_mode") && (typeof b.membership_mode !== "string" || !["manual", "automatic"].includes(b.membership_mode))) return null;
    if (Object.hasOwn(b, "rules")) {
      const r = b.rules;
      if (b.membership_mode !== "automatic" || !object(r) || !keys(r, ["match", "conditions"]) || r.match !== "all" ||
        !Array.isArray(r.conditions) || r.conditions.length !== 1) return null;
      const c: unknown = r.conditions[0];
      if (!object(c) || !keys(c, ["field", "operator", "days"]) || c.field !== "created_at" || c.operator !== "within_days" || c.days !== 7) return null;
    } else if (b.membership_mode === "automatic") return null;
    return b as CollectionCreateBody;
  } catch { return null; }
}

/** Browser storage is input, never authority; a non-null invalid record must remain locked. */
export function parseCollectionCreateJournal(raw: string, store: string, boundary: string): Journal | null {
  if (raw.length > maxRecord || !storeID.test(store) || !digest.test(boundary)) return null;
  try {
    const j: unknown = JSON.parse(raw);
    if (!object(j) || !keys(j, ["version", "store", "boundary", "command"]) || j.version !== 1 || j.store !== store || j.boundary !== boundary) return null;
    const c = j.command;
    if (!object(c) || !keys(c, ["key", "method", "resource", "body"]) || typeof c.key !== "string" || !uuid.test(c.key) ||
      c.method !== "POST" || c.resource !== "collections" || typeof c.body !== "string" || !bodyOf(c.body)) return null;
    return j as Journal;
  } catch { return null; }
}

export function useCollectionCreate(store: string, boundary: string, readReady: boolean,
  copy: { uncertain: string; unavailable: string; saved: string }, errors: (code: string) => string, onCreated: (id: string) => void) {
  const scope = `${store}|${boundary}`, storageKey = `catalog-collection-create:${store}`;
  const [view, setView] = useState<View>({ scope: "", ready: false, pending: null, message: null, retry: false });
  const [busy, setBusy] = useState(false);
  const pending = useRef<Journal | null>(null), raw = useRef<string | null>(null);
  const verifiedCookie = useRef("");
  const locked = useRef(true), flight = useRef(false), active = useRef(true), generation = useRef(0);
  const latest = useRef({ scope, readReady, boundary, copy, errors, onCreated });
  if (latest.current.scope !== scope || latest.current.readReady !== readReady) { generation.current++; locked.current = true; }
  latest.current = { scope, readReady, boundary, copy, errors, onCreated };
  const unknown = (retry: boolean) => {
    locked.current = true;
    setView({ scope, ready: true, pending: retry ? pending.current : null,
      message: { kind: "uncertain", text: retry ? latest.current.copy.uncertain : `${latest.current.copy.uncertain} ${latest.current.copy.unavailable}` }, retry });
  };

  useEffect(() => { active.current = true; return () => { active.current = false; generation.current++; }; }, []);
  useEffect(() => {
    pending.current = null; raw.current = null; locked.current = true; verifiedCookie.current = "";
    if (!readReady || !storeID.test(store) || !digest.test(boundary)) return;
    const epoch = generation.current, cookie = csrfCookie();
    let cancelled = false;
    void sessionBoundary(cookie).then(value => {
      if (cancelled || !active.current || epoch !== generation.current) return;
      if (value !== boundary) { unknown(false); return; }
      verifiedCookie.current = cookie;
      try {
        raw.current = sessionStorage.getItem(storageKey);
        if (raw.current !== null) {
          pending.current = parseCollectionCreateJournal(raw.current, store, boundary);
          unknown(!!pending.current);
        } else {
          locked.current = false;
          setView({ scope, ready: true, pending: null, message: null, retry: false });
        }
      } catch { unknown(false); }
    }, () => { if (!cancelled && active.current && epoch === generation.current) unknown(false); });
    return () => { cancelled = true; };
  }, [store, boundary, readReady]);

  const live = (epoch: number) => active.current && epoch === generation.current && latest.current.scope === scope &&
    latest.current.readReady && !!verifiedCookie.current && verifiedCookie.current === csrfCookie() && document.visibilityState !== "hidden";
  async function attempt(j: Journal, retry: boolean) {
    const epoch = generation.current;
    flight.current = true; locked.current = true; setBusy(true);
    setView({ scope, ready: true, pending: j, message: null, retry: false });
    try {
      if (!live(epoch)) return;
      if (sessionStorage.getItem(storageKey) !== raw.current) { unknown(false); return; }
      const outcome = await send(store, j.command, boundary, parseCreated);
      if (!live(epoch)) return;
      if (await sessionBoundary() !== boundary || !live(epoch)) { unknown(false); return; }
      if (!outcome.ok && (retry || outcome.uncertain || outcome.reconcile)) { unknown(!outcome.reconcile); return; }
      // Only the exact journal can be removed, after its authoritative receipt (or a fresh definitive rejection).
      if (sessionStorage.getItem(storageKey) !== raw.current) { unknown(false); return; }
      sessionStorage.removeItem(storageKey);
      if (sessionStorage.getItem(storageKey) !== null) { unknown(false); return; }
      pending.current = null; raw.current = null; locked.current = false;
      setView({ scope, ready: true, pending: null, retry: false, message: outcome.ok
        ? { kind: "success", text: latest.current.copy.saved } : { kind: "error", text: latest.current.errors(outcome.code) } });
      if (outcome.ok) latest.current.onCreated(outcome.value.id);
    } catch { if (live(epoch)) unknown(false); }
    finally { flight.current = false; if (active.current) setBusy(false); }
  }

  function create(body: CollectionCreateBody) {
    if (flight.current || locked.current || pending.current || !live(generation.current)) return;
    try {
      if (sessionStorage.getItem(storageKey) !== null) { unknown(false); return; }
      const j: Journal = { version: 1, store, boundary, command: command("POST", "collections", body) };
      const bytes = JSON.stringify(j);
      if (!parseCollectionCreateJournal(bytes, store, boundary)) { unknown(false); return; }
      locked.current = true; pending.current = j; raw.current = bytes;
      sessionStorage.setItem(storageKey, bytes);
      if (sessionStorage.getItem(storageKey) !== bytes) { unknown(false); return; }
      void attempt(j, false);
    } catch { unknown(false); }
  }
  async function retry() {
    if (flight.current || !live(generation.current) || !view.retry || !pending.current) return;
    const j = pending.current;
    try {
      const saved = sessionStorage.getItem(storageKey);
      if (saved !== raw.current || saved === null || !parseCollectionCreateJournal(saved, store, boundary)) { unknown(false); return; }
      await attempt(j, true);
    } catch { unknown(false); }
  }
  const current = view.scope === scope && readReady && !!verifiedCookie.current && verifiedCookie.current === csrfCookie() ? view : null;
  const draft = current?.pending ? bodyOf(current.pending.command.body) : null;
  return {
    create, retry, busy, ready: !!current?.ready, message: current?.message ?? (readReady && view.ready
      ? { kind: "uncertain" as const, text: `${copy.uncertain} ${copy.unavailable}` } : null), draft,
    blocked: !current?.ready || locked.current || flight.current,
    canRetry: !!current?.retry && !!pending.current && !flight.current,
    mayLeave: () => !locked.current && !flight.current && !pending.current && live(generation.current),
    fail: (text: string) => { if (!locked.current && !flight.current) setView(v => ({ ...v, message: { kind: "error", text } })); },
    dismiss: () => { if (!locked.current && !flight.current && !pending.current) setView(v => ({ ...v, message: null })); },
  };
}
export type CollectionCreator = ReturnType<typeof useCollectionCreate>;
