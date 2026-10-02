// W0 route metadata only; data/authorization remain in the existing BFF and Go.
import type { RouteEntry } from "../../route-types.ts";
export const financeRoutes = [
  {
    id: "finance",
    path: "/finance",
    group: "finance",
    labelKey: "financePage",
    icon: "orders",
    permission: "orders:read",
    template: "list",
    nav: true,
    spec: "tests/admin/customers-billing.spec.ts",
  },
] satisfies RouteEntry[];
