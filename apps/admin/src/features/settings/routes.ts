// W0 route metadata only; data/authorization remain in the existing BFF and Go.
import type { RouteEntry } from "../../route-types.ts";
export const settingsRoutes = [
  {
    id: "settings",
    path: "/settings",
    group: "settings",
    labelKey: "settingsPage",
    icon: "settings",
    permission: "integration:read",
    template: "workspace",
    nav: true,
    spec: "tests/admin/meta-connect.spec.ts",
  },
  {
    id: "team",
    path: "/team",
    group: "settings",
    labelKey: "team",
    icon: "support",
    permission: "owner",
    template: "list",
    nav: true,
    spec: "tests/admin/staff-team.spec.ts",
  },
  {
    id: "billing",
    path: "/billing",
    group: "settings",
    labelKey: "billing",
    icon: "orders",
    permission: "billing:manage",
    template: "workspace",
    nav: true,
    spec: "tests/admin/customers-billing.spec.ts",
  },
] satisfies RouteEntry[];
