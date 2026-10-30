// Purpose: Register order, after-sales and CVS-tool pages for shell navigation and access visibility.
// Depends on: ../../route-types.ts; localized shell labels and executable browser specs.
// Used by: apps/admin/src/routes.ts.
// Invariants: only the order list appears in the rail; data/authorization remain in the existing BFF and Go.
import type { RouteEntry } from "../../route-types.ts";
export const ordersRoutes = [
  {
    id: "orders",
    path: "/orders",
    group: "orders",
    labelKey: "orderList",
    icon: "orders",
    permission: "orders:read",
    template: "list",
    nav: true,
    spec: "tests/admin/orders-ui.spec.ts",
  },
  {
    id: "order-new",
    path: "/orders/new",
    group: "orders",
    labelKey: "orderNew",
    icon: "orders",
    permission: "inventory:reserve", // what Go guards a manual order with (migration 0094); there is no orders:write
    template: "form",
    nav: false,
    spec: "tests/admin/shell-browser.mjs",
  },
  {
    id: "returns",
    path: "/returns",
    group: "orders",
    labelKey: "returns",
    icon: "orders",
    permission: "orders:read",
    template: "list",
    // nav: false — a second rail entry would make the orders group non-singleton and drop the `nav-orders` button every
    // browser spec clicks (WorkspaceFrame). Reached from the orders list ("退貨與取消" link in OrderListFilters).
    nav: false,
    spec: "tests/admin/returns-ui.spec.ts",
  },
  {
    id: "cvs-match",
    path: "/orders/cvs-match",
    group: "orders",
    labelKey: "cvsMatch",
    icon: "orders",
    permission: "orders:read",
    template: "workspace",
    nav: false,
    spec: "tests/admin/parity-order-ops.spec.ts",
  },
  {
    id: "cvs-print",
    path: "/orders/cvs-print",
    group: "orders",
    labelKey: "cvsPrint",
    icon: "orders",
    permission: "fulfillment:write",
    template: "detail",
    nav: false,
    spec: "tests/admin/taiwan-cvs.spec.ts",
  },
] satisfies RouteEntry[];
