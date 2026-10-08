// Purpose: register the inbox page in the existing messages navigation group.
// Depends on: route metadata only; Go authorizes inbox reads and replies independently.
// Used by: the shared admin route registry and navigation acceptance tests.
import type { RouteEntry } from "../../route-types.ts";
/** Inbox navigation entry; permission metadata never grants API authority. */
export const messagesRoutes = [
  {
    id: "messages",
    path: "/messages",
    group: "messages",
    labelKey: "messages",
    icon: "chat",
    permission: "inbox:read",
    template: "list",
    nav: true,
    spec: "tests/admin/inbox-ui.spec.ts",
  },
] satisfies RouteEntry[];
