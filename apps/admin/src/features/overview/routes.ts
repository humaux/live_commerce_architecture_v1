// W0 route metadata only; data/authorization remain in the existing BFF and Go.
import type { RouteEntry } from "../../route-types.ts";
export const overviewRoutes = [
  {
    id: "dashboard",
    path: "/",
    group: "overview",
    labelKey: "dashboard",
    icon: "dashboard",
    permission: "orders:read",
    template: "overview",
    nav: true,
    spec: "tests/admin/shell-browser.mjs",
  },
] satisfies RouteEntry[];
