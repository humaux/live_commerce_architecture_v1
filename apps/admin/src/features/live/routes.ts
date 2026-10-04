// W0 route metadata only; data/authorization remain in the existing BFF and Go.
import type { RouteEntry } from "../../route-types.ts";
export const liveRoutes = [
  {
    id: "studio",
    path: "/studio",
    group: "live",
    labelKey: "studio",
    icon: "live",
    permission: "live:read",
    template: "workspace",
    nav: true,
    spec: "tests/admin/studio-ui.spec.ts",
  },
  {
    id: "claims",
    path: "/studio/claims",
    group: "live",
    labelKey: "claims",
    icon: "chat",
    permission: "live:read",
    template: "workspace",
    nav: false,
    spec: "tests/admin/claims-ui.spec.ts",
  },
] satisfies RouteEntry[];
