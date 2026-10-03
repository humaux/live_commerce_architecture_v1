// One write lifecycle for the catalog v2 editors (ProductEditor, ProductVariants, CollectionManager). `run` sends a
// fenced command (lib/catalog-v2-client send), shows the server's answer as a message and, when the outcome is uncertain
// (5xx / lost answer), keeps the command so `retry` re-sends the SAME key and bytes: a lost response may have committed,
// and a fresh command would apply twice. Server state is never guessed: the caller's `done` re-reads what the screen shows.
import { useCallback, useEffect, useRef, useState } from "react";
import { send, type Command, type Outcome } from "./catalog-v2-client";

export type WriteMessage = {
  kind: "error" | "success" | "uncertain" | "info";
  text: string;
};
type Pending = {
  cmd: Command;
  scope: string;
  run: (cmd: Command) => Promise<boolean>;
};

export function useWrite(
  store: string,
  boundary: string,
  errorText: (code: string) => string,
  uncertainText: string,
  recoveryText?: string,
) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<WriteMessage | null>(null);
  const pending = useRef<Pending | null>(null);
  const inFlight = useRef(false);
  const scope = `${store}|${boundary}`;
  const currentScope = useRef(scope);
  currentScope.current = scope;
  const fenceKey = recoveryText ? `catalog-command-pending:${store}` : null;
  const [fenceReady, setFenceReady] = useState(!recoveryText);
  const [recoveryBlocked, setRecoveryBlocked] = useState(false);
  useEffect(() => {
    if (!fenceKey) return;
    let blocked = true;
    try {
      blocked = sessionStorage.getItem(fenceKey) !== null;
    } catch {
      /* fail closed */
    }
    setRecoveryBlocked(blocked);
    setMessage(blocked ? { kind: "uncertain", text: recoveryText! } : null);
    pending.current = null;
    setFenceReady(true);
  }, [fenceKey, recoveryText, boundary]);

  const run = useCallback(
    async <T>(
      cmd: Command,
      parse: (value: unknown) => T,
      done: (value: T) => void | Promise<void>,
      okText: string,
      errors?: Record<string, string>,
    ): Promise<boolean> => {
      if (inFlight.current || pending.current || !fenceReady || recoveryBlocked)
        return false;
      if (fenceKey) {
        try {
          sessionStorage.setItem(fenceKey, cmd.key);
        } catch {
          setRecoveryBlocked(true);
          setMessage({ kind: "uncertain", text: recoveryText! });
          return false;
        }
      }
      inFlight.current = true;
      setBusy(true);
      setMessage(null);
      const attempt = async (c: Command): Promise<boolean> => {
        const outcome: Outcome<T> = await send(store, c, boundary, parse);
        if (currentScope.current !== scope) return false;
        if (outcome.ok) {
          pending.current = null;
          if (fenceKey) sessionStorage.removeItem(fenceKey);
          await done(outcome.value);
          setMessage({ kind: "success", text: okText });
          return true;
        }
        if (outcome.reconcile) {
          // No authoritative receipt: preserve the durable key across reload.
          // A changed/denied session must reconcile, not keep retrying blindly.
          pending.current = null;
          setRecoveryBlocked(true);
          setMessage({ kind: "uncertain", text: recoveryText ?? uncertainText });
          return false;
        }
        pending.current = outcome.uncertain
          ? { cmd: c, scope, run: attempt }
          : null;
        if (!outcome.uncertain && fenceKey) sessionStorage.removeItem(fenceKey);
        setMessage(
          outcome.uncertain
            ? { kind: "uncertain", text: uncertainText }
            : {
                kind: "error",
                text: errors?.[outcome.code] ?? errorText(outcome.code),
              },
        );
        return false;
      };
      try {
        return await attempt(cmd);
      } catch {
        if (currentScope.current === scope) {
          pending.current = { cmd, scope, run: attempt };
          setMessage({ kind: "uncertain", text: uncertainText });
        }
        return false;
      } finally {
        inFlight.current = false;
        setBusy(false);
      }
    },
    [
      boundary,
      errorText,
      store,
      uncertainText,
      scope,
      fenceKey,
      fenceReady,
      recoveryBlocked,
      recoveryText,
    ],
  );

  const retry = useCallback(async () => {
    const p = pending.current;
    if (!p || inFlight.current) return;
    if (p.scope !== currentScope.current) {
      pending.current = null;
      setMessage(null);
      return;
    }
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
  return {
    busy: busy || !fenceReady,
    message,
    run,
    retry,
    fail,
    notice,
    dismiss,
    canRetry:
      !!pending.current && pending.current.scope === scope && !recoveryBlocked,
  };
}
