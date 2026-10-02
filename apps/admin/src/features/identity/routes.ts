// W0 route metadata only; data/authorization remain in the existing BFF and Go.
import type { RouteEntry } from "../../route-types.ts";
export const identityRoutes = [
  {
    id: "invite",
    path: "/invite/[token]",
    group: "settings",
    labelKey: "invite",
    icon: "support",
    public: true,
    template: "public",
    nav: false,
    spec: "tests/admin/staff-team.spec.ts",
  },
  {
    id: "reset",
    path: "/reset",
    group: "settings",
    labelKey: "reset",
    icon: "support",
    public: true,
    template: "public",
    nav: false,
    spec: "tests/admin/password-auth.spec.ts",
  },
  {
    id: "signup",
    path: "/signup",
    group: "settings",
    labelKey: "signup",
    icon: "support",
    public: true,
    template: "public",
    nav: false,
    spec: "tests/admin/password-auth.spec.ts",
  },
] satisfies RouteEntry[];
