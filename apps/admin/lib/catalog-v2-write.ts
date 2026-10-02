// One write lifecycle for the catalog v2 editors (ProductEditor, ProductVariants, CollectionManager). `run` sends a
// fenced command (lib/catalog-v2-client send), shows the server's answer as a message and, when the outcome is uncertain
// (5xx / lost answer), keeps the command so `retry` re-sends the SAME key and bytes: a lost response may have committed,
// and a fresh command would apply twice. Server state is never guessed: the caller's `done` re-reads what the screen shows.
import { useCallback, useRef, useState } from "react";
import { send, type Command, type Outcome } from "./catalog-v2-client";

export type WriteMessage = { kind: "error" | "success" | "uncertain" | "info"; text: string };
type Pending = { cmd: Command; run: (cmd: Command) => Promise<boolean> };

export function useWrite(store: string, boundary: string, errorText: (code: string) => string, uncertainText: string) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<WriteMessage | null>(null);
  const pending = useRef<Pending | null>(null);
  const inFlight = useRef(false);

  const run = useCallback(
    async <T,>(cmd: Command, parse: (value: unknown) => T, done: (value: T) => void | Promise<void>, okText: string, errors?: Record<string, string>): Promise<boolean> => {
      if (inFlight.current) return false;
      inFlight.current = true;
      setBusy(true);
      setMessage(null);
      const attempt = async (c: Command): Promise<boolean> => {
        const outcome: Outcome<T> = await send(store, c, boundary, parse);
        if (outcome.ok) {
          pending.current = null;
          await done(outcome.value);
          setMessage({ kind: "success", text: okText });
          return true;
        }
        pending.current = outcome.uncertain ? { cmd: c, run: attempt } : null;
        setMessage(outcome.uncertain ? { kind: "uncertain", text: uncertainText } : { kind: "error", text: errors?.[outcome.code] ?? errorText(outcome.code) });
        return false;
      };
      try {
        return await attempt(cmd);
      } finally {
        inFlight.current = false;
        setBusy(false);
      }
    },
    [boundary, errorText, store, uncertainText],
  );

  const retry = useCallback(async () => {
    const p = pending.current;
    if (!p || inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setMessage(null);
    try {
      await p.run(p.cmd);
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  }, []);

  const fail = useCallback((text: string) => setMessage({ kind: "error", text }), []);
  const notice = useCallback((text: string) => setMessage({ kind: "info", text }), []); // neutral answer to a click that sends nothing
  const dismiss = useCallback(() => {
    pending.current = null;
    setMessage(null);
  }, []);
  return { busy, message, run, retry, fail, notice, dismiss };
}
