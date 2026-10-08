// Purpose: own transient A2 buffers with visible-only polling, epoch reset and stale-response fencing.
// Depends on: React, inbox transport/privacy fences and comment-model; no storage or provider calls.
// Used by: CommentStream; all private child state is unmounted when hidden or unauthorised.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { inboxRead, InboxError } from "../../../lib/inbox-client";
import { useInboxPrivacy } from "../messages/use-privacy";
import {
  applyCommentPage,
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
  const clear = useCallback(() => {
    current.current = emptyComments();
    setBuffer(current.current);
    select(null);
    setError("");
    setBusy(false);
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
    const load = async (older = false) => {
      if (!alive || inFlight.current || document.visibilityState !== "visible")
        return;
      if (!older && deadline.current > Date.now()) {
        timer = setTimeout(() => void load(), deadline.current - Date.now());
        return;
      }
      clearTimeout(timer);
      inFlight.current = true;
      setBusy(true);
      const ticket = fence.begin();
      let delay = 3000;
      const query = new URLSearchParams({ limit: "50" });
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
        if (next.reset) {
          // Test-only fault is admitted by a loopback server prop. Production always discards the old epoch.
          const retained = retainOnReset ? current.current.items : [];
          current.current = { ...emptyComments(), items: retained };
          setBuffer(current.current);
          select(null);
          resetSelection.current++;
          setError("reset");
          const fresh = parseCommentPage(
            await inboxRead<unknown>(store, `${root}?limit=50`, ticket.signal),
          );
          if (!alive || !fence.current(ticket)) return;
          next = applyCommentPage(current.current, fresh, false);
          if (next.reset) throw new InboxError("stream_unavailable", 503);
        }
        current.current = next;
        setBuffer(next);
        failCount.current = 0;
        deadline.current = 0;
        setError("");
      } catch (e) {
        if (!alive || !fence.current(ticket)) return;
        const code = e instanceof InboxError ? e.code : "stream_unavailable";
        if (e instanceof InboxError && [401, 403].includes(e.status)) {
          expire();
          setError(code);
          return;
        }
        if (code === "invalid_cursor") {
          current.current = emptyComments();
          setBuffer(current.current);
          select(null);
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
  return {
    buffer,
    error,
    busy,
    selection,
    select,
    privacy,
    revision,
    resetGeneration: resetSelection.current,
    // A command refresh rereads current marks for existing rows, not only new comment sequence numbers.
    refresh: () => {
      deadline.current = 0;
      current.current = { ...current.current, next: null };
      refresh((n) => n + 1);
    },
    older: () => void loader.current(true),
  };
}
