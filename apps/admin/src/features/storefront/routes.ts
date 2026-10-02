// W0 route metadata only; data/authorization remain in the existing BFF and Go.
import type { RouteEntry } from "../../route-types.ts";
export const storefrontRoutes = [
  {
    id: "design",
    path: "/design",
    group: "storefront",
    labelKey: "design",
    icon: "inventory",
    permission: "integration:read",
    template: "workspace",
    nav: true,
    spec: "tests/admin/design.spec.ts",
  },
] satisfies RouteEntry[];
