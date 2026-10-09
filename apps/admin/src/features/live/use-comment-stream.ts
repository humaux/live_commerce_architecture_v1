// Purpose: own transient A2 buffers with visible-only polling, epoch reset and stale-response fencing.
// Depends on: React, inbox transport/privacy fences and comment-model; no storage or provider calls.
// Used by: CommentStream; all private child state is unmounted when hidden or unauthorised.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { inboxRead, InboxError } from "../../../lib/inbox-client";
import { useInboxPrivacy } from "../messages/index";
import {
  applyCommentPage,
  COMMENT_MEMORY_CAP,
  commentDelay,
  emptyComments,
  parseCommentPage,
  type CommentBuffer,
} from "./comment-model";

/** Read the stream only in the visible authorised scope; reset clears before a fresh request. */
export function useCommentStream(
  store: string,
  session: string,
  enabled: boolean,
  retainOnReset = false,
  clearRelated?: () => void,
) {
  const [buffer, setBuffer] = useState<CommentBuffer>(emptyComments),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [revision, refresh] = useState(0);
  const [selection, select] = useState<{
    ref?: string;
    bundle?: string;
  } | null>(null);
  const current = useRef(emptyComments()),
    inFlight = useRef(false),
    failCount = useRef(0),
    deadline = useRef(0);
  const fullReadAt = useRef<number | null>(null), manualFull = useRef(false);
  const related = useRef(clearRelated), marksReadAt = useRef(0);
  related.current = clearRelated;
  const clear = useCallback(() => {
    current.current = emptyComments();
    setBuffer(current.current);
    select(null);
    setError("");
    setBusy(false);
    fullReadAt.current = null;
    manualFull.current = false;
    related.current?.();
  }, []);
  const privacy = useInboxPrivacy(clear),
    { fence, visible, expire } = privacy;
  const loader = useRef<(older: boolean) => Promise<void>>(async () => {});
  const resetSelection = useRef(0);
  useEffect(() => {
    let alive = true,
      timer: ReturnType<typeof setTimeout> | undefined;
    if (!enabled) {
      expire();
      setError("forbidden");
      return;
    }
    if (!visible || privacy.blocked.current) return;
    fence.invalidate(true);
    const root = `live-sessions/${session}/comments`;
    const pageLimit = 50;
    const load = async (older = false) => {
      if (!alive || inFlight.current || document.visibilityState !== "visible")
        return;
      if (older && (!current.current.older || current.current.items.length >= COMMENT_MEMORY_CAP)) return;
      if (!older && deadline.current > Date.now()) {
        timer = setTimeout(() => void load(), deadline.current - Date.now());
        return;
      }
      clearTimeout(timer);
      inFlight.current = true;
      setBusy(true);
      const ticket = fence.begin();
      let delay = 3000;
      const query = new URLSearchParams({ limit: String(pageLimit) });
      if (older && current.current.older)
        query.set("before_cursor", current.current.older);
      else if (current.current.next) {
        query.set("after_epoch", String(current.current.next.epoch));
        query.set("after_seq", String(current.current.next.seq));
      }
      try {
        const page = parseCommentPage(
          await inboxRead<unknown>(store, `${root}?${query}`, ticket.signal),
        );
        if (!alive || !fence.current(ticket)) return;
        let next = applyCommentPage(current.current, page, older);
        let rebuilt = false;
        // Only the existing 10s HEAD window is evidence for deletions; incremental/history
        // reads cannot establish absence. Preserve their cursors and older merchant history.
        if (!older && !next.reset) {
          // Four minutes + a 60s scan budget bounds successful visible live refreshes
          // to five minutes. Explicit Graph history is retained until a manual reset.
          const full = manualFull.current || (fullReadAt.current !== null && Date.now() - fullReadAt.current >= 240000);
          if (full) {
            const base = next, scanDeadline = Date.now() + 60000;
            const target = Math.max(base.next?.seq ?? 0, ...base.items.flatMap(row => typeof row.seq === "number" ? [row.seq] : []));
            const numeric = base.items.flatMap(row => typeof row.seq === "number" ? [row.seq] : []);
            // Re-read the published IG range, not a possibly million-event session prefix.
            // A manual reset starts at zero; normal earliest-head reads never prune IG.
            let after = !manualFull.current && numeric.length ? Math.max(0, Math.min(...numeric) - 1) : 0;
            let stage = emptyComments(), reads = 0;
            const readWindow = async (cursor?: number) => {
              if (!alive || !fence.current(ticket) || document.visibilityState !== "visible") throw new DOMException("scope changed", "AbortError");
              const remaining = scanDeadline - Date.now();
              if (reads++ >= 20 || remaining <= 0) throw new InboxError("stream_unavailable", 503);
              const params = new URLSearchParams({ limit: String(pageLimit) });
              if (cursor !== undefined) { params.set("after_epoch", String(base.epoch)); params.set("after_seq", String(cursor)); }
              return parseCommentPage(await inboxRead<unknown>(store, `${root}?${params}`, AbortSignal.any([ticket.signal, AbortSignal.timeout(remaining)])));
            };
            let fresh = await readWindow(page.stream.source_platform === "instagram" && after > 0 ? after : undefined);
            for (;;) {
              if (!alive || !fence.current(ticket)) return;
              if (fresh.reset || fresh.epoch !== base.epoch) { next = { ...emptyComments(), reset: true }; break; }
              stage = applyCommentPage(stage, fresh, false);
              const continuation = stage.next!.seq;
              if (fresh.stream.source_platform === "facebook" || fresh.scan_exhausted || continuation >= target) {
                const kept = manualFull.current ? [] : base.items.filter(row => row.seq === null);
                next = applyCommentPage({ ...emptyComments(), items: kept }, { ...fresh, items: stage.items, next: stage.next! }, false);
                if (!manualFull.current && base.historyLoaded) next = { ...next, historyLoaded: true, older: base.older };
                if (fresh.stream.source_platform === "facebook") next = { ...next, next: base.next ?? next.next };
                select(selected => selected?.ref && !next.items.some(row => row.ref === selected.ref) ? null : selected);
                rebuilt = true;
                manualFull.current = false;
                break;
              }
              // Short/empty IG pages can be post-LIMIT filtered. Only forward cursor
              // coverage proves completion; never publish a truncated half-window.
              if (continuation <= after) throw new InboxError("stream_unavailable", 503);
              after = continuation;
              fresh = await readWindow(after);
            }
          } else if (!current.current.next) marksReadAt.current = Date.now();
          else if (Date.now() - marksReadAt.current >= 10000) {
            const head = parseCommentPage(await inboxRead<unknown>(store, `${root}?limit=${pageLimit}`, ticket.signal));
            if (!alive || !fence.current(ticket)) return;
            const refreshed = applyCommentPage(next, head, false, true);
            if (!refreshed.reset && head.items.length > 0)
              select(selected => selected?.ref && !refreshed.items.some(row => row.ref === selected.ref) ? null : selected);
            next = refreshed.reset ? refreshed : { ...refreshed, next: next.next, older: next.older };
            marksReadAt.current = Date.now();
          }
        }
        if (next.reset) {
          // Test-only fault is admitted by a loopback server prop. Production always discards the old epoch.
          const retained = retainOnReset ? current.current.items : [];
          current.current = { ...emptyComments(), items: retained };
          setBuffer(current.current);
          select(null);
          resetSelection.current++;
          setError("reset");
          const fresh = parseCommentPage(
            await inboxRead<unknown>(store, `${root}?limit=${pageLimit}`, ticket.signal),
          );
          if (!alive || !fence.current(ticket)) return;
          next = applyCommentPage(current.current, fresh, false);
          if (next.reset) throw new InboxError("stream_unavailable", 503);
        }
        current.current = next;
        if (fullReadAt.current === null || rebuilt) fullReadAt.current = Date.now();
        if (rebuilt) marksReadAt.current = Date.now();
        setBuffer(next);
        failCount.current = 0;
        deadline.current = 0;
        setError("");
      } catch (e) {
        if (!alive || !fence.current(ticket)) return;
        const code = e instanceof InboxError ? e.code : "stream_unavailable";
        // A scoped 404 also means the store/resource is no longer authorised. Retaining
        // cached comments on that path would expose private data after grant revocation.
        if (e instanceof InboxError && [401, 403, 404].includes(e.status)) {
          expire();
          setError(code);
          return;
        }
        if (code === "invalid_cursor") {
          // Expired Graph/history cursors say nothing about the valid live stream.
          // Lose only pagination authority; an invalid incremental cursor still resets.
          current.current = older ? { ...current.current, older: null } : emptyComments();
          setBuffer(current.current);
          if (!older) select(null);
        }
        delay = Math.max(
          commentDelay(++failCount.current),
          e instanceof InboxError ? e.retryAfter : 0,
        );
        setError(code);
      } finally {
        if (alive) inFlight.current = false;
        if (alive && fence.current(ticket)) {
          setBusy(false);
          deadline.current = Date.now() + delay;
          timer = setTimeout(() => void load(), delay);
        }
      }
    };
    loader.current = load;
    void load();
    return () => {
      alive = false;
      clearTimeout(timer);
      fence.invalidate(false);
      inFlight.current = false;
    };
  }, [
    store,
    session,
    enabled,
    visible,
    privacy.revision,
    privacy.blocked,
    fence,
    expire,
    revision,
    retainOnReset,
  ]);
  // A same-store grant downgrade commits before passive cleanup. Never return cached
  // private state (including the A8 visibility gate) for that first revoked render.
  return {
    buffer: enabled ? buffer : emptyComments(),
    error: enabled ? error : "forbidden",
    busy: enabled && busy,
    selection: enabled ? selection : null,
    select: enabled ? select : () => {},
    privacy: { ...privacy, visible: enabled && privacy.visible },
    revision,
    resetGeneration: resetSelection.current,
    // A command refresh rereads current marks for existing rows, not only new comment sequence numbers.
    refresh: (resetHistory = false) => {
      if (!enabled) return;
      if (resetHistory === true) manualFull.current = true;
      deadline.current = 0;
      current.current = { ...current.current, next: null };
      refresh((n) => n + 1);
    },
    older: () => { if (enabled) void loader.current(true); },
  };
}
