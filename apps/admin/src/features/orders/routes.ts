// W0 route metadata only; data/authorization remain in the existing BFF and Go.
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
    permission: "orders:write",
    template: "form",
    nav: false,
    spec: "tests/admin/shell-browser.mjs",
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
