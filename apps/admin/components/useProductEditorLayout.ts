// Purpose: Reserve editor scroll/footer space and reveal focused sections or command feedback without moving the shell.
// Depends on: React hooks; browser ResizeObserver, IntersectionObserver, visualViewport and matchMedia; ProductDocumentForm element refs.
// Used by: ProductDocumentForm; presentation only, with no draft, API, receipt or persistence ownership.
"use client";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

// Presentation only: the editor owns a viewport-sized scroll pane and a real
// footer row. No drafts, command state, catalog DTOs or persistence belong here.
/** Owns DOM measurement, pane scrolling and observer cleanup; never sends a command. */
export function useProductEditorLayout(
  sections: readonly string[],
  onSection: (id: string) => void,
  feedbackKey: string,
  feedbackBusy: boolean,
) {
  const editor = useRef<HTMLFormElement>(null),
    fields = useRef<HTMLDivElement>(null),
    feedback = useRef<HTMLDivElement>(null);
  const sectionKey = sections.join("|");
  const [feedbackAttempt, setFeedbackAttempt] = useState(0);
  // Small-screen readiness accordion breakpoint (PR #1 review comment 4212540352): the toggle
  // button and its visible label are rendered only ≤900px, so a labelled control never exists
  // without its label (axe hidden-explicit-label). Server render reports the desktop layout.
  const [narrowViewport, setNarrowViewport] = useState(false);
  useEffect(() => {
    const query = window.matchMedia("(max-width: 900px)");
    const update = () => setNarrowViewport(query.matches);
    update();
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);
  const noteSaveAttempt = useCallback(
    () => setFeedbackAttempt((n) => n + 1),
    [],
  );
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
  useLayoutEffect(() => {
    const root = fields.current;
    const target = feedback.current;
    if (!root || !target || feedbackBusy) return;
    // Outcomes belong to the real command UI, not a second footer summary.
    // Reveal both the notice and its recovery actions without moving the shell.
    root.scrollTo({
      top:
        root.scrollTop +
        target.getBoundingClientRect().top -
        root.getBoundingClientRect().top,
    });
    target.focus({ preventScroll: true });
  }, [feedbackKey, feedbackAttempt, feedbackBusy]);
  useEffect(() => {
    const root = fields.current;
    if (!root) return;
    const targets = sectionKey
      .split("|")
      .map((id) => document.getElementById(id))
      .filter(
        (target): target is HTMLElement => !!target && root.contains(target),
      );
    let frame = 0;
    const update = () => {
      // Keep the actual focused destination highlighted while it is visible,
      // rather than letting observer delivery order win for short sections.
      const focused =
        document.activeElement?.closest<HTMLElement>(".product-section");
      const bounds = focused?.getBoundingClientRect();
      const viewport = root.getBoundingClientRect();
      if (
        focused &&
        targets.includes(focused) &&
        bounds &&
        bounds.bottom > viewport.top &&
        bounds.top < viewport.bottom
      ) {
        onSection(focused.id);
        return;
      }
      // IntersectionObserver entries are only changes, not the visible set.
      // Inspect every ordered section against the owning pane on each scroll.
      const first = targets.find((target) => {
        const rect = target.getBoundingClientRect();
        return rect.bottom > viewport.top && rect.top < viewport.bottom;
      });
      if (first) onSection(first.id);
    };
    const schedule = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(update);
    };
    const observer = new IntersectionObserver(schedule, {
      root,
      rootMargin: "0px",
    });
    targets.forEach((target) => observer.observe(target));
    root.addEventListener("scroll", schedule, { passive: true });
    update();
    return () => {
      observer.disconnect();
      cancelAnimationFrame(frame);
      root.removeEventListener("scroll", schedule);
    };
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
      const control = target.querySelector<HTMLElement>("input,textarea,button,select");
      control?.focus({ preventScroll: true });
      if (control) {
        // Expanded readiness leaves a shorter field pane; long translated hints
        // can push the first control below it even after aligning the section.
        // Reveal the control inside this pane, never by scrolling the outer shell.
        const field = control.getBoundingClientRect(), area = root.getBoundingClientRect();
        if (field.bottom > area.bottom) root.scrollTop += field.bottom - area.bottom;
        else if (field.top < area.top) root.scrollTop -= area.top - field.top;
      }
      onSection(id);
    },
    [onSection],
  );
  return { editor, fields, feedback, focus, noteSaveAttempt, narrowViewport };
}
