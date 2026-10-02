// Single route registry. No network calls; permissions here are navigation UX, never Go authorization.
import type { Access, GroupID, RouteEntry } from "./route-types.ts";
import { overviewRoutes } from "./features/overview/routes.ts";
import { liveRoutes } from "./features/live/routes.ts";
import { ordersRoutes } from "./features/orders/routes.ts";
import { catalogRoutes } from "./features/catalog/routes.ts";
import { customersRoutes } from "./features/customers/routes.ts";
import { marketingRoutes } from "./features/marketing/routes.ts";
import { storefrontRoutes } from "./features/storefront/routes.ts";
import { financeRoutes } from "./features/finance/routes.ts";
import { settingsRoutes } from "./features/settings/routes.ts";
import { identityRoutes } from "./features/identity/routes.ts";
export const routes: readonly RouteEntry[] = [
  ...overviewRoutes,
  ...liveRoutes,
  ...ordersRoutes,
  ...catalogRoutes,
  ...customersRoutes,
  ...marketingRoutes,
  ...storefrontRoutes,
  ...financeRoutes,
  ...settingsRoutes,
  ...identityRoutes,
];
export const groups: readonly { id: GroupID; icon: string }[] = [
  { id: "overview", icon: "dashboard" },
  { id: "live", icon: "live" },
  { id: "orders", icon: "orders" },
  { id: "catalog", icon: "product" },
  { id: "messages", icon: "chat" },
  { id: "customers", icon: "support" },
  { id: "marketing", icon: "meta" },
  { id: "storefront", icon: "inventory" },
  { id: "finance", icon: "orders" },
  { id: "settings", icon: "settings" },
];
export function matchRoute(path: string): RouteEntry | undefined {
  const clean = path.replace(/\/$/, "") || "/";
  return (
    routes.find((r) => r.path === clean) ??
    routes.find(
      (r) =>
        r.path.includes("[") &&
        new RegExp("^" + r.path.replace(/\[[^\]]+\]/g, "[^/]+") + "$").test(
          clean,
        ),
    )
  );
}
export function canOpen(route: RouteEntry, access: Access): boolean {
  if (route.public) return true;
  if (!access) return false;
  return (
    access.role === "owner" || access.permissions.includes(route.permission)
  );
}
export function visibleGroups(access: Access) {
  return groups
    .map((group) => ({
      ...group,
      routes: routes.filter(
        (r) => r.group === group.id && r.nav && canOpen(r, access),
      ),
    }))
    .filter(
      (g) =>
        g.routes.length &&
        (g.id !== "catalog" ||
          access?.role === "owner" ||
          access?.permissions.includes("catalog:read")),
    );
}
