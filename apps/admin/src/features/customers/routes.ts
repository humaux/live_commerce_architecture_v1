// Purpose: customer list/detail/import subpage metadata with one primary navigation entry.
// Depends on: shell RouteEntry and backend customers:read/customers:privacy permissions.
// Used by: route registry, shell titles/navigation and static/browser gates.
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
  {
    id: "customer-import",
    path: "/customers/import",
    group: "customers",
    labelKey: "customerImport",
    icon: "support",
    permission: "customers:privacy",
    template: "form",
    nav: false,
    spec: "tests/admin/import-wizard.spec.ts",
  },
] satisfies RouteEntry[];
