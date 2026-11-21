// Purpose: Define the route, rail group and store-access types for admin navigation.
// Depends on: ShellLabel from shell-copy.
// Used by: The admin route registry and feature route declarations.
// Invariants: Private routes name a permission and public routes are explicitly marked.
import type { ShellLabel } from "./shell-copy.ts";
export type GroupID =
  | "overview"
  | "live"
  | "orders"
  | "orderNew"
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
