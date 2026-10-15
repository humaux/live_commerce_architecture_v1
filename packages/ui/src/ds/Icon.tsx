// Purpose: One authored SVG vocabulary; identical optical box and stroke prevent mixed glyph baselines.
// Depends on: React SVG props only; no font, network or vendor package.
// Used by: DS primitives, shell and gallery templates. Invariant: decorative unless a title is supplied.
import { useId, type SVGProps } from "react";

const paths = {
  dashboard: "M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z",
  orders: "M8 4H5v17h14V4h-3 M8 3h8v4H8z M8 12h8 M8 16h5",
  live: "M3 6h12v12H3z M15 10l6-4v12l-6-4",
  messages: "M4 4h16v12H9l-5 4z M8 8h8 M8 12h5",
  products: "M3 7l9-4 9 4v10l-9 4-9-4z M3 7l9 4 9-4 M12 11v10 M8 5l9 4",
  customers: "M16 21v-3a4 4 0 0 0-4-4H7a4 4 0 0 0-4 4v3 M17 4a4 4 0 0 1 0 8 M21 21v-3a4 4 0 0 0-3-4 M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0",
  marketing: "M4 10v5h4l11 4V5L8 10z M8 15l2 6h4l-2-5 M22 9v6",
  store: "M4 10v11h16V10 M3 10l2-7h14l2 7z M9 21v-7h6v7 M3 10q3 4 6 0 3 4 6 0 3 4 6 0",
  analytics: "M4 3v18h17 M8 16v-5 M13 16V7 M18 16V4",
  settings: "M9 3h6l1 4 4 1v8l-4 1-1 4H9l-1-4-4-1V8l4-1z M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0",
  chevronRight: "M9 5l7 7-7 7", chevronLeft: "M15 5l-7 7 7 7", chevronDown: "M5 9l7 7 7-7",
  search: "M21 21l-5-5 M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0",
  close: "M6 6l12 12 M6 18L18 6", check: "M4 12l5 5L20 6",
  info: "M12 11v6 M12 7v.1 M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0",
  alert: "M12 8v5 M12 17v.1 M12 2L1 21h22z",
  warning: "M12 8v5 M12 17v.1 M12 2L1 21h22z",
  danger: "M8 8l8 8 M8 16l8-8 M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0",
  upload: "M12 16V3 M7 8l5-5 5 5 M3 15v6h18v-6",
  image: "M3 3h18v18H3z M3 17l6-6 5 5 3-3 4 4 M9 7h.1",
  more: "M5 12h.1 M12 12h.1 M19 12h.1", plus: "M12 4v16 M4 12h16",
  externalLink: "M14 3h7v7 M21 3L10 14 M10 3H3v18h18v-7",
  help: "M9 8a3 3 0 1 1 5 2c-2 1-2 2-2 3 M12 17v.1 M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0",
  bell: "M5 17h14l-2-3V9a5 5 0 0 0-10 0v5z M10 21h4 M12 2v2",
  user: "M4 21v-2a8 8 0 0 1 16 0v2 M16 6a4 4 0 1 1-8 0 4 4 0 0 1 8 0",
  spinner: "M21 12a9 9 0 1 1-9-9", menu: "M3 6h18 M3 12h18 M3 18h18",
  lock: "M5 10h14v11H5z M8 10V6a4 4 0 0 1 8 0v4", empty: "M3 14l4-10h10l4 10v7H3z M3 14h6l1 3h4l1-3h6",
} as const;
export type IconName = keyof typeof paths;
export function Icon({ name, size = 20, title, ...props }: Omit<SVGProps<SVGSVGElement>, "name"> & { name: IconName; size?: 16 | 20 | 24; title?: string }) {
  const id = useId();
  return <svg {...props} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden={title ? undefined : true} role={title ? "img" : undefined} aria-labelledby={title ? id : undefined} focusable="false" style={{ flexShrink: 0, verticalAlign: "middle", ...props.style }}>
    {title && <title id={id}>{title}</title>}<path d={paths[name]} />
  </svg>;
}
