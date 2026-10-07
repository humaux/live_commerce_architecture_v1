// Purpose: clear private inbox memory synchronously on hide and revoke obsolete requests.
// Depends on: React, react-dom flushSync, the CSRF session hint and InboxFence.
// Used by: Inbox and independently mounted BuyerPanel; no storage writes or logging.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { csrfCookie } from "../../../lib/settings-client";
import { InboxFence } from "./privacy";

/** Bind a private component to visible-page and session lifetime; reveal always starts a fresh read. */
export function useInboxPrivacy(clear: () => void, calibration = false) {
  const fence = useRef(new InboxFence());
  const blocked = useRef(false);
  const [visible, setVisible] = useState(true);
  const [departing, setDeparting] = useState(false);
  const [revision, setRevision] = useState(0);
  const expire = useCallback(() => {
    blocked.current = true;
    fence.current.invalidate(false);
    clear();
    setVisible(false);
  }, [clear]);
  const suspend = useCallback(() => {
    fence.current.revoke();
    clear();
    setVisible(false);
    setDeparting(true);
  }, [clear]);
  useEffect(() => {
    const cookie = csrfCookie();
    const hide = () => {
      // CI calibration is fixture-only and deliberately violates the real hide assertion.
      if (calibration) return;
      fence.current.invalidate(false);
      flushSync(() => {
        clear();
        setVisible(false);
      });
    };
    const reveal = () => {
      if (blocked.current || !fence.current.reveal(document.visibilityState))
        return;
      if (csrfCookie() !== cookie) {
        expire();
        return;
      }
      clear();
      setVisible(true);
      setRevision((value) => value + 1);
    };
    const visibility = () =>
      document.visibilityState === "hidden" ? hide() : reveal();
    const storage = (event: StorageEvent) => {
      if (event.key === "commerce-session-logout") expire();
    };
    const message = (event: MessageEvent) => {
      if (event.data?.type === "logout") expire();
    };
    let channel: BroadcastChannel | null = null;
    try {
      channel = new BroadcastChannel("commerce-session");
      channel.addEventListener("message", message);
    } catch {
      /* Same-tab and storage notifications still revoke the session. */
    }
    document.addEventListener("visibilitychange", visibility);
    window.addEventListener("pagehide", hide);
    window.addEventListener("pageshow", reveal);
    window.addEventListener("focus", reveal);
    window.addEventListener("storage", storage);
    window.addEventListener("commerce-session-logout", expire);
    if (document.visibilityState === "hidden") hide();
    return () => {
      fence.current.invalidate(false);
      document.removeEventListener("visibilitychange", visibility);
      window.removeEventListener("pagehide", hide);
      window.removeEventListener("pageshow", reveal);
      window.removeEventListener("focus", reveal);
      window.removeEventListener("storage", storage);
      window.removeEventListener("commerce-session-logout", expire);
      channel?.close();
    };
  }, [clear, expire, calibration]);
  return {
    fence: fence.current,
    visible,
    revision,
    expire,
    blocked,
    suspend,
    departing,
  };
}
