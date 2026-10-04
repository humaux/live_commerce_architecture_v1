"use client";
import { useCallback, useEffect, useRef } from "react";

export type ProductNavigationState = { dirty: boolean; locked: boolean };

// Shell controls use beforeNavigate; page links use capture. Never ask twice for one click.
export function useProductLeaveGuard(
  { dirty, locked }: ProductNavigationState,
  leaveText: string,
) {
  const approvedUnload = useRef(false);
  const beforeNavigate = useCallback(() => {
    const allowed = !locked && (!dirty || window.confirm(leaveText));
    approvedUnload.current = allowed;
    return allowed;
  }, [dirty, locked, leaveText]);
  useEffect(() => {
    approvedUnload.current = false;
    if (!dirty && !locked) return;
    const warn = (event: BeforeUnloadEvent) => {
      const approved = approvedUnload.current;
      approvedUnload.current = false;
      if (!approved) event.preventDefault();
    };
    const leave = (event: MouseEvent) => {
      const anchor = (event.target as Element)?.closest("a");
      if (
        event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey ||
        anchor?.target === "_blank" ||
        anchor?.hasAttribute("download")
      )
        return;
      if (
        !anchor?.href ||
        anchor.closest(
          "[data-shell-rail], [data-shell-topbar], [data-shell-route]",
        )
      )
        return;
      const next = new URL(anchor.href),
        current = new URL(window.location.href);
      if (
        next.origin + next.pathname + next.search ===
        current.origin + current.pathname + current.search
      )
        return;
      if (!beforeNavigate()) {
        event.preventDefault();
        event.stopPropagation();
      }
    };
    window.addEventListener("beforeunload", warn);
    document.addEventListener("click", leave, true);
    return () => {
      window.removeEventListener("beforeunload", warn);
      document.removeEventListener("click", leave, true);
    };
  }, [dirty, locked, beforeNavigate]);
  return beforeNavigate;
}
