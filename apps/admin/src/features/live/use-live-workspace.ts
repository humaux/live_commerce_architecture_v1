// Purpose: Bound visible-only polling and single-command receipt fencing for LC-U1.
// Depends on: React, existing StudioError and session-boundary helpers; sessionStorage stores opaque receipt keys only.
// Used by: LiveWorkspace, LiveConsole and session-result UI; never calls a provider directly.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { csrfCookie, sessionBoundary } from "@/lib/settings-client";
import { StudioError } from "@/lib/studio-client";
import { settleLiveCommand } from "./workspace-model";

/** Reads private data with an epoch/session fence; schedules one read at a time, five seconds after completion. */
export function useLiveRead<T>(scope: string, enabled: boolean, read: (signal: AbortSignal) => Promise<T>) {
  const [view, setView] = useState<{ scope: string; data: T | null; error: string; boundary: string }>({ scope: "", data: null, error: "", boundary: "" });
  const [revision, bump] = useState(0);
  const readRef = useRef(read); readRef.current = read;
  const cookie = useRef("");
  useEffect(() => {
    let alive = true, request: AbortController | null = null, timer: ReturnType<typeof setTimeout> | undefined;
    let epoch = 0, signedOut = false;
    const clear = (error = "") => { ++epoch; request?.abort(); clearTimeout(timer); cookie.current = ""; setView((previous) => ({ scope, data: null, error, boundary: !error && previous.scope === scope ? previous.boundary : "" })); };
    const load = async () => {
      if (!alive || !enabled || signedOut || document.visibilityState !== "visible") return;
      const current = ++epoch; request?.abort(); request = new AbortController();
      try {
        const boundary = await sessionBoundary().catch(() => { throw new StudioError("signed-out"); });
        const data = await readRef.current(request.signal);
        if (!alive || epoch !== current) return;
        if (boundary !== await sessionBoundary().catch(() => "")) throw new StudioError("signed-out");
        cookie.current = csrfCookie(); setView({ scope, data, error: "", boundary });
      } catch (error) {
        if (!alive || epoch !== current) return;
        const code = error instanceof StudioError ? error.code : "unavailable";
        if (code === "signed-out" || code === "forbidden") signedOut = true;
        setView((previous) => ({ scope, data: null, error: code, boundary: !signedOut && previous.scope === scope ? previous.boundary : "" }));
      } finally {
        if (alive && epoch === current && !signedOut) timer = setTimeout(load, 5000);
      }
    };
    const visibility = () => { clear(); if (document.visibilityState === "visible") void load(); };
    const logout = () => { signedOut = true; clear("signed-out"); };
    const storage = (event: StorageEvent) => { if (event.key === "commerce-session-logout") logout(); };
    const focus = () => { if (cookie.current && cookie.current !== csrfCookie()) logout(); };
    const channel = typeof BroadcastChannel !== "undefined" ? new BroadcastChannel("commerce-session") : null;
    if (channel) channel.onmessage = (event) => { if (event.data?.type === "logout") logout(); };
    document.addEventListener("visibilitychange", visibility);
    window.addEventListener("commerce-session-logout", logout); window.addEventListener("storage", storage); window.addEventListener("focus", focus);
    clear(); void load();
    return () => { alive = false; ++epoch; request?.abort(); clearTimeout(timer); channel?.close(); document.removeEventListener("visibilitychange", visibility); window.removeEventListener("commerce-session-logout", logout); window.removeEventListener("storage", storage); window.removeEventListener("focus", focus); };
  }, [scope, enabled, revision]);
  const refresh = useCallback(() => bump((n) => n + 1), []);
  const current = view.scope === scope && (!cookie.current || cookie.current === csrfCookie()) ? view : { scope, data: null, error: "", boundary: "" };
  return { ...current, refresh };
}

/** Keeps one command/key across explicit retries and a durable opaque fence across reloads. */
export function useLiveCommand(scope: string, boundary: string, onChanged: () => void) {
  const [busy, setBusy] = useState(false), [error, setError] = useState(""), [fenced, setFenced] = useState(true);
  const [reason, setReason] = useState("");
  type Request = { key: string; execute: (key: string) => Promise<unknown>; complete: (value: unknown) => void };
  const pending = useRef<Request | null>(null);
  const lifetime = useRef(0);
  const invalidate = useCallback(() => { ++lifetime.current; }, []);
  useEffect(() => { ++lifetime.current; return () => { ++lifetime.current; }; }, []);
  const active = useRef(false), identity = useRef(`${scope}|${boundary}`); identity.current = `${scope}|${boundary}`;
  // Persist only the opaque receipt key. Reauthentication must not make an ambiguous command disappear.
  // In-memory replay remains bound to the original session through identity/isCurrent below.
  const fenceKey = `live-workspace-command:${scope}`;
  useEffect(() => {
    pending.current = null; active.current = false; setBusy(false);
    let blocked = true;
    try { blocked = !boundary || sessionStorage.getItem(fenceKey) !== null; } catch { /* unavailable storage fails closed */ }
    setFenced(blocked); setError(blocked && boundary ? "recovery" : ""); setReason("");
  }, [fenceKey, boundary]);
  useEffect(() => {
    const leave = (event: BeforeUnloadEvent) => { if (active.current || pending.current) { event.preventDefault(); event.returnValue = ""; } };
    window.addEventListener("beforeunload", leave); return () => window.removeEventListener("beforeunload", leave);
  }, []);
  const attempt = async (request: Request) => {
    const atStart = identity.current;
    const generation = lifetime.current;
    const isCurrent = () => identity.current === atStart && lifetime.current === generation;
    active.current = true; setBusy(true); setError(""); setReason("");
    try {
      await settleLiveCommand(() => request.execute(request.key), isCurrent, (value) => {
        sessionStorage.removeItem(fenceKey); pending.current = null; setFenced(false); onChanged(); request.complete(value);
      });
    } catch (caught) {
      if (!isCurrent()) return;
      const code = caught instanceof StudioError ? caught.code : "uncertain";
      const retain = ["uncertain", "signed-out", "forbidden"].includes(code);
      if (retain) { pending.current = code === "uncertain" ? request : null; setFenced(true); }
      else { sessionStorage.removeItem(fenceKey); pending.current = null; setFenced(false); }
      setError(code);
      setReason(caught instanceof StudioError ? caught.api : "");
      if (code === "conflict" || code === "signed-out" || code === "forbidden") onChanged();
    } finally { if (isCurrent()) { active.current = false; setBusy(false); } }
  };
  const run = async <T,>(execute: (key: string) => Promise<T>, complete?: (value: T) => void) => {
    if (!boundary || active.current || pending.current || fenced) return;
    const key = crypto.randomUUID();
    try { sessionStorage.setItem(fenceKey, key); } catch { setError("recovery"); setFenced(true); return; }
    const request: Request = { key, execute, complete: (value) => complete?.(value as T) }; pending.current = request; await attempt(request);
  };
  return { busy, error, reason, blocked: busy || fenced || !!pending.current, canRetry: !!pending.current && !busy,
    invalidate, run, retry: async () => { if (pending.current && !active.current) await attempt(pending.current); } };
}
