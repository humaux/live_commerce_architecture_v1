// W0 route metadata only; data/authorization remain in the existing BFF and Go.
import type { RouteEntry } from "../../route-types.ts";
export const marketingRoutes = [
  {
    id: "promotions",
    path: "/promotions",
    group: "marketing",
    labelKey: "promotions",
    icon: "orders",
    permission: "pricing:read",
    template: "list",
    nav: true,
    spec: "tests/storefront/promotions-gate.mjs",
  },
  {
    id: "ads",
    path: "/ads",
    group: "marketing",
    labelKey: "ads",
    icon: "meta",
    permission: "ads:read",
    template: "workspace",
    nav: true,
    spec: "tests/admin/ads.spec.ts",
  },
] satisfies RouteEntry[];
