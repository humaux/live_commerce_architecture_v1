// W0 route metadata only; data/authorization remain in the existing BFF and Go.
import type { RouteEntry } from "../../route-types.ts";
export const customersRoutes = [
  {
    id: "customers",
    path: "/customers",
    group: "customers",
    labelKey: "customerList",
    icon: "support",
    permission: "customers:read",
    template: "list",
    nav: true,
    spec: "tests/admin/customers-billing.spec.ts",
  },
  {
    id: "customer-detail",
    path: "/customers/[customer]",
    group: "customers",
    labelKey: "customerDetail",
    icon: "support",
    permission: "customers:read",
    template: "detail",
    nav: false,
    spec: "tests/admin/customers-billing.spec.ts",
  },
] satisfies RouteEntry[];
