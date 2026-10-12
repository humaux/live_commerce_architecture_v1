// Purpose: Live workspace navigation metadata, with only implemented routes exposed.
// Depends on: route-types.ts and localized shell labels.
// Used by: routes.ts, W0 navigation, breadcrumbs and route-permission checks.
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
    id: "live-console", path: "/studio/console", group: "live", labelKey: "liveConsole", icon: "live",
    permission: "live:read", template: "workspace", nav: true, spec: "tests/admin/live-console.spec.ts",
  },
  {
    id: "claims",
    path: "/studio/claims",
    group: "live",
    labelKey: "claims",
    icon: "chat",
    permission: "live:read",
    template: "workspace",
    nav: true,
    spec: "tests/admin/claims-ui.spec.ts",
  },
  {id:"live-settings",path:"/studio/settings",group:"live",labelKey:"liveSettings",icon:"chat",permission:"live:read",template:"workspace",nav:true,spec:"tests/admin/live-settings.spec.ts"},
] satisfies RouteEntry[];
