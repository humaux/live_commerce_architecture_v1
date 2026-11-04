// Purpose: size the template page to the visible viewport so its scroll pane ends above its footer.
// Depends on: React and native viewport/resize observers.
// Used by: MessageTemplateList and MessageTemplateForm.
// Invariant: presentation only; no template data, permissions or command state.
"use client";
import { useLayoutEffect, useRef } from "react";

export function useTemplateViewport(enabled: boolean) {
  const page = useRef<HTMLElement>(null);
  useLayoutEffect(() => {
    const element = page.current;
    if (!enabled || !element) return;
    let frame = 0;
    const measure = () => {
      const viewport = window.visualViewport;
      const viewportHeight = viewport?.height ?? window.innerHeight;
      const bottom = (viewport?.offsetTop ?? 0) + viewportHeight;
      const height = `${Math.max(0, Math.min(viewportHeight, bottom - element.getBoundingClientRect().top))}px`;
      if (element.style.getPropertyValue("--template-height") !== height)
        element.style.setProperty("--template-height", height);
    };
    const schedule = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(measure);
    };
    const observer = new ResizeObserver(schedule);
    for (let node: Element | null = element; node; node = node.parentElement) {
      observer.observe(node);
      for (let before = node.previousElementSibling; before; before = before.previousElementSibling)
        observer.observe(before);
    }
    measure();
    window.addEventListener("resize", schedule);
    window.addEventListener("scroll", schedule, { passive: true });
    window.visualViewport?.addEventListener("resize", schedule);
    window.visualViewport?.addEventListener("scroll", schedule);
    return () => {
      observer.disconnect();
      cancelAnimationFrame(frame);
      window.removeEventListener("resize", schedule);
      window.removeEventListener("scroll", schedule);
      window.visualViewport?.removeEventListener("resize", schedule);
      window.visualViewport?.removeEventListener("scroll", schedule);
    };
  }, [enabled]);
  return page;
}
