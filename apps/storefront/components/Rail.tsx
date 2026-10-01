"use client";

// Horizontal product rail of the home page (featured_collection section). The list scrolls natively (swipe, trackpad, keyboard focus
// inside a card) and snaps to cards. On a wide screen the last visible card is cut by the viewport edge with nothing saying "more",
// so an edge fade and previous/next buttons show while there is more in that direction (data-more-start / data-more-end, also what the
// browser gate reads). No dependency and no BFF/Go call; the cards arrive as server-rendered children, and before hydration the rail is
// simply the scrolling list.
import { useCallback, useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import type { Locale } from "@live-commerce/i18n";
import { shopCopy } from "../lib/shop-copy";
import { ChevronLeftIcon, ChevronRightIcon } from "./icons";

export default function Rail({ locale, children }: { locale: Locale; children: ReactNode }) {
  const copy = shopCopy[locale];
  const list = useRef<HTMLUListElement>(null);
  const [edge, setEdge] = useState({ start: false, end: false });
  const measure = useCallback(() => {
    const el = list.current;
    if (!el) return;
    const start = el.scrollLeft > 4;
    const end = el.scrollLeft + el.clientWidth < el.scrollWidth - 4;
    setEdge((previous) => (previous.start === start && previous.end === end ? previous : { start, end }));
  }, []);
  useEffect(() => {
    measure();
    const el = list.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [measure]);
  const step = (direction: 1 | -1) => list.current?.scrollBy({ left: direction * list.current.clientWidth * 0.8, behavior: "smooth" });
  return (
    <div className="sf-railwrap" data-testid="rail" data-more-start={edge.start} data-more-end={edge.end}>
      <ul className="sf-rail" ref={list} onScroll={measure}>
        {children}
      </ul>
      <button type="button" className="sf-railbtn sf-railbtn--prev" hidden={!edge.start} aria-label={copy.railPrev} onClick={() => step(-1)} data-testid="rail-prev">
        <ChevronLeftIcon />
      </button>
      <button type="button" className="sf-railbtn sf-railbtn--next" hidden={!edge.end} aria-label={copy.railNext} onClick={() => step(1)} data-testid="rail-next">
        <ChevronRightIcon />
      </button>
    </div>
  );
}
