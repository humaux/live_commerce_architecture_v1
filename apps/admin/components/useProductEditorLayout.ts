"use client";
import { useCallback, useEffect, useLayoutEffect, useRef } from "react";

// Presentation only: the editor owns a viewport-sized scroll pane and a real
// footer row. No drafts, command state, catalog DTOs or persistence belong here.
export function useProductEditorLayout(
  sections: readonly string[],
  onSection: (id: string) => void,
) {
  const editor = useRef<HTMLFormElement>(null),
    fields = useRef<HTMLDivElement>(null);
  const sectionKey = sections.join("|");
  useLayoutEffect(() => {
    const form = editor.current;
    if (!form) return;
    let frame = 0;
    // Shell, breadcrumb, page heading and notices all contribute to the real
    // editor top. Reserve only the viewport space that remains below it.
    const measure = () => {
      const viewport = window.visualViewport;
      const viewportHeight = viewport?.height ?? window.innerHeight;
      const bottom = (viewport?.offsetTop ?? 0) + viewportHeight;
      // Never grow beyond one visible viewport if the outer document scrolls
      // above the editor: that would continuously lengthen the document itself.
      const height = `${Math.max(0, Math.min(viewportHeight, bottom - form.getBoundingClientRect().top))}px`;
      if (form.style.getPropertyValue("--pe-editor-height") !== height)
        form.style.setProperty("--pe-editor-height", height);
    };
    const schedule = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(measure);
    };
    const observer = new ResizeObserver(schedule);
    // A preceding heading/notice can move the editor without resizing the form.
    // Observe its layout ancestors and their preceding siblings too.
    for (let node: Element | null = form; node; node = node.parentElement) {
      observer.observe(node);
      for (
        let before = node.previousElementSibling;
        before;
        before = before.previousElementSibling
      )
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
  }, []);
  useEffect(() => {
    const root = fields.current;
    if (!root) return;
    const observer = new IntersectionObserver(
      (entries) => {
        // Keep the actual focused destination highlighted while it is visible,
        // rather than letting observer delivery order win for short sections.
        const focused =
          document.activeElement?.closest<HTMLElement>(".product-section");
        const bounds = focused?.getBoundingClientRect();
        const viewport = root.getBoundingClientRect();
        if (
          focused &&
          root.contains(focused) &&
          bounds &&
          bounds.bottom > viewport.top &&
          bounds.top < viewport.bottom
        ) {
          onSection(focused.id);
          return;
        }
        const seen = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (seen[0]) onSection(seen[0].target.id);
      },
      { root, rootMargin: "0px" },
    );
    sectionKey.split("|").forEach((id) => {
      const element = document.getElementById(id);
      if (element) observer.observe(element);
    });
    return () => observer.disconnect();
  }, [sectionKey, onSection]);
  const focus = useCallback(
    (id: string) => {
      const root = fields.current;
      const target = document.getElementById(id);
      if (!root || !target || !root.contains(target)) return;
      if (target instanceof HTMLDetailsElement) target.open = true;
      // scrollIntoView would also move the shell/document. Only this owning pane
      // scrolls; the reserved nav and save rows stay continuously available.
      root.scrollTo({
        top:
          root.scrollTop +
          target.getBoundingClientRect().top -
          root.getBoundingClientRect().top,
      });
      target
        .querySelector<HTMLElement>("input,textarea,button,select")
        ?.focus({ preventScroll: true });
      onSection(id);
    },
    [onSection],
  );
  return { editor, fields, focus };
}
