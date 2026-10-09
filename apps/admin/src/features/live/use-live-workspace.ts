// Purpose: Bound visible-only polling and single-command receipt fencing for LC-U1.
// Depends on: React, StudioError/session authority and closed command journal/clients; sessionStorage never stores credentials.
// Used by: LiveWorkspace, LiveConsole and session-result UI; never calls a provider directly.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { csrfCookie, sessionBoundary } from "@/lib/settings-client";
import { StudioError } from "@/lib/studio-client";
import { executeLiveRequest } from "./console-client";
import { parseLiveJournal, validLiveRequest, type LiveJournal, type LiveRequest } from "./command-journal";

/** Reads on mount/refresh by default. Only A1 opts into visible, single-flight polling with server-aware backoff. */
export function useLiveRead<T>(scope: string, enabled: boolean, read: (signal: AbortSignal) => Promise<T>, poll = false) {
  const [view, setView] = useState<{ scope: string; data: T | null; error: string; boundary: string }>({ scope: "", data: null, error: "", boundary: "" });
  const [revision, bump] = useState(0);
  const readRef = useRef(read); readRef.current = read;
  const cookie = useRef("");
  const locked = useRef({ scope, denied: false });
  if (locked.current.scope !== scope) locked.current = { scope, denied: false };
  useEffect(() => {
    let alive = true, request: AbortController | null = null, timer: ReturnType<typeof setTimeout> | undefined;
    let epoch = 0, signedOut = locked.current.denied, failures = 0, notBefore = 0;
    const clear = (error = "") => { ++epoch; request?.abort(); clearTimeout(timer); cookie.current = ""; setView((previous) => ({ scope, data: null, error, boundary: !error && previous.scope === scope ? previous.boundary : "" })); };
    const load = async () => {
      if (!alive || !enabled || signedOut || document.visibilityState !== "visible") return;
      const remaining = notBefore - Date.now();
      // Large Retry-After values must not overflow JS timers into a 1ms retry storm.
      if (remaining > 0) { timer = setTimeout(load, Math.min(remaining, 2_147_483_647)); return; }
      const current = ++epoch; request?.abort(); request = new AbortController();
      let delay = 5000;
      try {
        const boundary = await sessionBoundary().catch(() => { throw new StudioError("signed-out"); });
        const data = await readRef.current(request.signal);
        if (!alive || epoch !== current) return;
        if (boundary !== await sessionBoundary().catch(() => "")) throw new StudioError("signed-out");
        failures = 0; cookie.current = csrfCookie(); setView({ scope, data, error: "", boundary });
      } catch (error) {
        if (!alive || epoch !== current) return;
        const code = error instanceof StudioError ? error.code : "unavailable";
        if (code === "signed-out" || code === "forbidden") { signedOut = true; locked.current.denied = true; }
        delay = Math.max(Math.min(30000, 3000 * 2 ** Math.min(failures++, 4)), error instanceof StudioError ? error.retryAfterMs : 0);
        setView((previous) => ({ scope, data: !signedOut && previous.scope === scope ? previous.data : null, error: code, boundary: !signedOut && previous.scope === scope ? previous.boundary : "" }));
      } finally {
        if (alive && epoch === current && !signedOut && poll) { notBefore = Date.now() + delay; timer = setTimeout(load, Math.min(delay, 2_147_483_647)); }
      }
    };
    const visibility = () => { if (signedOut) return; clear(); if (document.visibilityState === "visible") void load(); };
    const logout = () => { signedOut = true; locked.current.denied = true; clear("signed-out"); };
    const storage = (event: StorageEvent) => { if (event.key === "commerce-session-logout") logout(); };
    const focus = () => { if (cookie.current && cookie.current !== csrfCookie()) logout(); };
    const channel = typeof BroadcastChannel !== "undefined" ? new BroadcastChannel("commerce-session") : null;
    if (channel) channel.onmessage = (event) => { if (event.data?.type === "logout") logout(); };
    document.addEventListener("visibilitychange", visibility);
    window.addEventListener("commerce-session-logout", logout); window.addEventListener("storage", storage); window.addEventListener("focus", focus);
    if (!signedOut) { clear(); void load(); }
    return () => { alive = false; ++epoch; request?.abort(); clearTimeout(timer); channel?.close(); document.removeEventListener("visibilitychange", visibility); window.removeEventListener("commerce-session-logout", logout); window.removeEventListener("storage", storage); window.removeEventListener("focus", focus); };
  }, [scope, enabled, revision, poll]);
  const refresh = useCallback(() => bump((n) => n + 1), []);
  const current = view.scope === scope && (!cookie.current || cookie.current === csrfCookie()) ? view : { scope, data: null, error: "", boundary: "" };
  return { ...current, refresh };
}

/** Saves the exact scoped command/key across reloads; replay is explicit and retains every uncertain receipt. */
export function useLiveCommand(scope: string, boundary: string, onChanged: () => void, onRestored?: (request: LiveRequest, value: unknown) => void) {
  const [busy, setBusy] = useState(false), [error, setError] = useState(""), [fenced, setFenced] = useState(true);
  const [reason, setReason] = useState("");
  type Request = LiveJournal & { complete?: (value: unknown) => void };
  const pending = useRef<Request | null>(null);
  const lifetime = useRef(0);
  const invalidate = useCallback(() => { ++lifetime.current; }, []);
  useEffect(() => { ++lifetime.current; return () => { ++lifetime.current; }; }, []);
  const active = useRef(false), identity = useRef(`${scope}|${boundary}`); identity.current = `${scope}|${boundary}`;
  const fenceKey = `live-workspace-command:${scope}`;
  useEffect(() => {
    pending.current = null; active.current = false; setBusy(false);
    let blocked = true;
    try {
      const stored = sessionStorage.getItem(fenceKey);
      pending.current = stored ? parseLiveJournal(stored, scope) : null;
      blocked = !boundary || stored !== null;
    } catch { /* unavailable storage fails closed */ }
    setFenced(blocked); setError(blocked && boundary ? pending.current ? "uncertain" : "recovery" : ""); setReason("");
  }, [fenceKey, boundary]);
  useEffect(() => {
    const leave = (event: BeforeUnloadEvent) => { if (active.current || pending.current) { event.preventDefault(); event.returnValue = ""; } };
    window.addEventListener("beforeunload", leave); return () => window.removeEventListener("beforeunload", leave);
  }, []);
  const attempt = async (request: Request) => {
    const atStart = identity.current;
    const generation = lifetime.current;
    const isCurrent = () => identity.current === atStart && lifetime.current === generation;
    if (!boundary || active.current) return;
    const clearJournal = () => {
      // A delayed answer can clear its receipt after departure, never a newer command's fence.
      const stored = sessionStorage.getItem(fenceKey);
      if (stored && parseLiveJournal(stored, scope)?.key === request.key) sessionStorage.removeItem(fenceKey);
    };
    active.current = true; setBusy(true); setError(""); setReason("");
    try {
      const value = await executeLiveRequest(request, request.key, boundary);
      clearJournal();
      if (!isCurrent()) return;
      pending.current = null; setFenced(false); onChanged();
      if (request.complete) request.complete(value); else onRestored?.(request, value);
    } catch (caught) {
      const code = caught instanceof StudioError ? caught.code : "uncertain";
      const status = caught instanceof StudioError ? caught.status : 0;
      const definite = status >= 400 && status < 500;
      // A permission refusal to a later replay does not settle an earlier UNKNOWN command.
      const retain = request.mayHaveCommitted || !definite && ["uncertain", "signed-out", "forbidden", "unavailable"].includes(code);
      if (retain) {
        request.mayHaveCommitted = true;
        const { complete: _complete, ...journal } = request;
        try {
          if (parseLiveJournal(sessionStorage.getItem(fenceKey) ?? "", scope)?.key === request.key) sessionStorage.setItem(fenceKey, JSON.stringify(journal));
        } catch { /* The original durable fence remains fail-closed if storage becomes inaccessible. */ }
      } else { clearJournal(); }
      if (!isCurrent()) return;
      pending.current = retain ? request : null; setFenced(retain);
      setError(code);
      setReason(caught instanceof StudioError ? caught.api : "");
      if (code === "conflict" || code === "signed-out" || code === "forbidden") onChanged();
    } finally { if (isCurrent()) { active.current = false; setBusy(false); } }
  };
  const run = async <T,>(input: LiveRequest, complete?: (value: T) => void) => {
    if (!boundary || active.current || pending.current || fenced) return;
    if (!validLiveRequest(input, scope)) { setError("invalid"); return; }
    const journal: LiveJournal = { ...input, key: crypto.randomUUID(), mayHaveCommitted: false };
    // Persist ambiguity before dispatch: reload can happen before any response. Only this
    // in-memory first attempt knows that a definite refusal settles its sole execution.
    try { sessionStorage.setItem(fenceKey, JSON.stringify({ ...journal, mayHaveCommitted: true })); } catch { setError("recovery"); setFenced(true); return; }
    const request: Request = { ...journal, complete: (value) => complete?.(value as T) }; pending.current = request; await attempt(request);
  };
  return { busy, error, reason, blocked: busy || fenced || !!pending.current || ["forbidden", "signed-out"].includes(error), canRetry: !!pending.current && !busy && !!boundary && !["forbidden", "signed-out"].includes(error),
    invalidate, run, retry: async () => { if (pending.current && !active.current) await attempt(pending.current); } };
}
