// Purpose: one in-memory pending command per customer/tag management scope; an UNKNOWN one survives unmount/remount (memory only).
// Depends on: React, customer-tags-client fenced transport, session-events (global logout on unauthorized); customers-billing-v1 W6-01B idempotency.
// Used by: CustomerTags and CustomerNotes; no storage or note-body logs (the pending command holds the note body: never persisted).
import { useEffect, useRef, useState } from "react";
import { sendTagCommand, type TagCommand } from "./customer-tags-client";
import { signalLogout } from "./session-events";

type Pending = { command: Readonly<TagCommand>; unknown: boolean; parse: (v: unknown) => unknown; committed: (v: unknown) => Promise<void> };
// UNKNOWN commands outlive a component (hide/navigation remounts): a fresh remount must not unlock a new key, because the first
// attempt may have committed (duplicate note). Keyed by store, session boundary and scope; cleared by a trusted success. A new
// boundary never matches an old key, so a session change drops it; entries for stale boundaries are pruned on set.
const unknownCommands = new Map<string, Pending>();
const slot = (store: string, boundary: string, scope: string) => `${store}\n${boundary}\n${scope}`;
const rememberUnknown = (store: string, boundary: string, scope: string, p: Pending) => {
  for (const key of unknownCommands.keys()) if (key.startsWith(`${store}\n`) && key.endsWith(`\n${scope}`)) unknownCommands.delete(key);
  unknownCommands.set(slot(store, boundary, scope), p);
};

/** Coordinate explicit writes/retries; session scope drops stale results, while an UNKNOWN command stays locked across remounts. */
export function useTagWrite(store: string, boundary: string, scope = "") {
  const [restored] = useState(() => unknownCommands.get(slot(store, boundary, scope)) ?? null);
  const [busy, setBusy] = useState(false);
  const [uncertain, setUncertain] = useState(restored !== null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  // Same frozen command and key; its old commit callback belonged to an unmounted view, so a trusted success asks for a refresh.
  const pending = useRef<Pending | null>(restored && { ...restored, committed: async () => { throw new Error("refresh_required"); } });
  const inflight = useRef(false);
  const epoch = useRef(0);
  useEffect(() => () => {
    epoch.current++;
    // An in-flight command has an unknown outcome once its view is gone.
    if (pending.current && inflight.current) { pending.current.unknown = true; rememberUnknown(store, boundary, scope, pending.current); }
    pending.current = null;
  }, [store, boundary, scope]);

  async function execute(p: Pending) {
    if (inflight.current) return;
    inflight.current = true; const generation = epoch.current;
    setBusy(true); setError(""); setNotice("");
    try {
      const result = await sendTagCommand(store, p.command, boundary, p.parse);
      if (epoch.current !== generation) return;
      if (!result.ok) {
        // I06: a retry's auth refusal says nothing about whether the ORIGINAL unknown attempt committed.
        // UNKNOWN is sticky until a trusted success; never unlock a fresh key from a later no-dispatch/refusal.
        p.unknown = p.unknown || result.uncertain;
        setError(result.code); setUncertain(p.unknown);
        // A write refused as unauthorized means the session is gone for the whole page: signal the global logout so the
        // guarded customer read clears PII (Codex review P2, PR #3). Sticky UNKNOWN above is unchanged.
        if (result.code === "unauthorized") signalLogout();
        pending.current = p.unknown ? p : null;
        if (p.unknown) rememberUnknown(store, boundary, scope, p); else unknownCommands.delete(slot(store, boundary, scope));
        return;
      }
      pending.current = null; setUncertain(false); unknownCommands.delete(slot(store, boundary, scope));
      try { await p.committed(result.value); }
      catch { if (epoch.current === generation) setError("refresh_required"); }
    } finally {
      inflight.current = false;
      if (epoch.current === generation) setBusy(false);
    }
  }

  function run<T>(method: TagCommand["method"], resource: string, body: unknown | undefined, parse: (v: unknown) => T, committed: (v: T) => Promise<void>) {
    if (inflight.current || pending.current) return;
    const p: Pending = { command: Object.freeze({ method, resource, ...(body === undefined ? {} : { body: JSON.stringify(body) }), key: crypto.randomUUID() }), unknown: false,
      parse, committed: (v) => committed(v as T) };
    pending.current = p;
    void execute(p);
  }
  return { busy, uncertain, error, notice, setNotice, locked: busy || uncertain,
    run, retry: () => { if (pending.current) void execute(pending.current); } };
}
