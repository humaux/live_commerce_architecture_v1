// Purpose: finance primary route and secondary report metadata; finance keeps one primary navigation entry.
// Depends on: RouteEntry and localized ShellLabel registry.
// Used by: shell breadcrumbs/route parity; financial authority stays in Go.
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
  {
    id: "reports", path: "/finance/reports", group: "finance", labelKey: "reportsPage", icon: "orders",
    permission: "orders:read", template: "list", nav: false, spec: "tests/admin/reports.spec.ts",
  },
] satisfies RouteEntry[];
