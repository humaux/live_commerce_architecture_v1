// Purpose: one in-memory pending command per mounted customer/tag management scope.
// Depends on: React, customer-tags-client fenced transport; customers-billing-v1 W6-01B idempotency.
// Used by: CustomerTags and CustomerNotes; no storage or note-body logs.
import { useEffect, useRef, useState } from "react";
import { sendTagCommand, type TagCommand } from "./customer-tags-client";

type Pending = { command: Readonly<TagCommand>; unknown: boolean; parse: (v: unknown) => unknown; committed: (v: unknown) => Promise<void> };
/** Coordinate explicit writes/retries; unmount/session scope drops pending payloads and stale results. */
export function useTagWrite(store: string, boundary: string) {
  const [busy, setBusy] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const pending = useRef<Pending | null>(null);
  const inflight = useRef(false);
  const epoch = useRef(0);
  useEffect(() => () => { epoch.current++; pending.current = null; }, [store, boundary]);

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
        pending.current = p.unknown ? p : null;
        return;
      }
      pending.current = null; setUncertain(false);
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
