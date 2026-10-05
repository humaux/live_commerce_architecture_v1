import type { CSSProperties } from "react";

const paths: Record<string, string> = {
  dashboard: "M4 4h7v7H4zM13 4h7v4h-7zM13 11h7v9h-7zM4 14h7v6H4z",
  product: "M3 7l9-4 9 4-9 4-9-4zm0 0v11l9 4 9-4V7M12 11v11",
  inventory: "M3 9l9-6 9 6v12H3V9zm4 12v-9h10v9M7 16h10",
  orders: "M5 3h14v18l-3-2-4 2-4-2-3 2V3zm3 5h8M8 12h8",
  wallet: "M4 5h15v14H4a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2zm0 0v4h17v7h-6V9M17 12h1",
  live: "M3 6h12v12H3zM15 10l6-3v10l-6-3",
  chat: "M20 15a8 8 0 10-14 2l-3 4 6-2a8 8 0 0011-4z",
  meta: "M4 18l5-12 6 12 5-12",
  support: "M5 15v-4a7 7 0 0114 0v4M5 11H2v7h4M19 11h3v7h-4M18 18c0 3-3 3-6 3",
  settings:
    "M9 3h6l1 3 3 1 2 5-2 5-3 1-1 3H9l-1-3-3-1-2-5 2-5 3-1 1-3zM15 12a3 3 0 11-6 0 3 3 0 016 0",
  search: "M16 16l5 5M18 10a8 8 0 11-16 0 8 8 0 0116 0",
  plus: "M12 4v16M4 12h16",
  menu: "M3 6h18M3 12h18M3 18h18",
  close: "M5 5l14 14M19 5L5 19",
  chevron: "M9 5l7 7-7 7",
  refresh: "M20 11a8 8 0 1 0-2 5M20 4v7h-7",
};
export function Icon({
  name,
  size = 21,
  style,
}: {
  name: string;
  size?: number;
  style?: CSSProperties;
}) {
  return (
    <svg
      aria-hidden="true"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.65"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={style}
    >
      <path d={paths[name] ?? paths.product} />
    </svg>
  );
}
