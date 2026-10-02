import type { ShellLabel } from "./shell-copy.ts";
export type GroupID =
  | "overview"
  | "live"
  | "orders"
  | "catalog"
  | "messages"
  | "customers"
  | "marketing"
  | "storefront"
  | "finance"
  | "settings";
export type RouteEntry = {
  id: string;
  path: string;
  group: GroupID;
  labelKey: ShellLabel;
  icon: string;
  template: "overview" | "list" | "detail" | "form" | "workspace" | "public";
  nav: boolean;
  spec: string;
} & (
  { permission: string; public?: never } | { public: true; permission?: never }
);
export type Access = { role: string | null; permissions: string[] } | null;
