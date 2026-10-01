// Staff-team model (contracts/storefront-v2.md §D): the five roles, the strict parsers for the BFF answers of
// POST /api/team/{list,invite,accept} (-> Go /v1/identity/staff/*, internal/identity/staff.go) and the BFF request grammar.
// Framework-free on purpose (no next/*, no lib/auth.ts) so node --test can import it. The server decides every permission:
// nothing here gates an action, it only refuses malformed requests and malformed answers.
import { object } from "./customers-model.ts";
import { canonicalUUID } from "./orders-model.ts";

export const roles = ["owner", "admin", "live_operator", "fulfilment", "viewer"] as const;
export type Role = (typeof roles)[number];
export const isRole = (value: unknown): value is Role => typeof value === "string" && (roles as readonly string[]).includes(value);

export type Member = { principal_id: string; email: string | null; role: Role; joined_at: string; is_me: boolean };
export type Invitation = {
  id: string; email: string; role: Role; created_at: string; expires_at: string; expired: boolean;
  mail_state: "PENDING" | "SENT" | "FAILED" | "UNKNOWN";
};
export type Team = { my_role: Role | null; members: Member[]; invitations: Invitation[] };
export type InviteResult = { id: string; expires_at: string; mail_state: "SENT" | "FAILED" | "UNKNOWN" };
export type Joined = { store_id: string; role: Role };

const fail = () => new Error("unavailable");
const instant = (value: unknown): value is string => typeof value === "string" && value.length <= 40 && Number.isFinite(Date.parse(value));
const text = (value: unknown, max: number): value is string => typeof value === "string" && value.length >= 1 && value.length <= max;

export function parseTeam(value: unknown): Team {
  const v = object(value, ["my_role", "members", "invitations"]);
  if ((v.my_role !== null && !isRole(v.my_role)) || !Array.isArray(v.members) || !Array.isArray(v.invitations) ||
    v.members.length > 500 || v.invitations.length > 500) throw fail();
  const members = v.members.map((item): Member => {
    const m = object(item, ["principal_id", "email", "role", "joined_at", "is_me"]);
    if (typeof m.principal_id !== "string" || !canonicalUUID.test(m.principal_id) || (m.email !== null && !text(m.email, 254)) ||
      !isRole(m.role) || !instant(m.joined_at) || typeof m.is_me !== "boolean") throw fail();
    return { principal_id: m.principal_id, email: m.email, role: m.role, joined_at: m.joined_at, is_me: m.is_me };
  });
  const invitations = v.invitations.map((item): Invitation => {
    const i = object(item, ["id", "email", "role", "created_at", "expires_at", "expired", "mail_state"]);
    if (typeof i.id !== "string" || !canonicalUUID.test(i.id) || !text(i.email, 254) || !isRole(i.role) || !instant(i.created_at) ||
      !instant(i.expires_at) || typeof i.expired !== "boolean" || !["PENDING", "SENT", "FAILED", "UNKNOWN"].includes(String(i.mail_state))) throw fail();
    return { id: i.id, email: i.email, role: i.role, created_at: i.created_at, expires_at: i.expires_at, expired: i.expired,
      mail_state: i.mail_state as Invitation["mail_state"] };
  });
  return { my_role: v.my_role as Role | null, members, invitations };
}

export function parseInviteResult(value: unknown): InviteResult {
  const v = object(value, ["id", "expires_at", "mail_state"]);
  if (typeof v.id !== "string" || !canonicalUUID.test(v.id) || !instant(v.expires_at) || !["SENT", "FAILED", "UNKNOWN"].includes(String(v.mail_state))) throw fail();
  return { id: v.id, expires_at: v.expires_at, mail_state: v.mail_state as InviteResult["mail_state"] };
}

export function parseJoined(value: unknown): Joined {
  const v = object(value, ["store_id", "role"]);
  if (typeof v.store_id !== "string" || !canonicalUUID.test(v.store_id) || !isRole(v.role)) throw fail();
  return { store_id: v.store_id, role: v.role };
}

// ---- BFF request grammar (apps/admin/app/api/team/[action]/route.ts) -------------------------------------------------
export const teamActions = ["list", "invite", "revoke-invite", "set-role", "remove", "accept"] as const;
export type TeamAction = (typeof teamActions)[number];
export const teamKeys: Record<TeamAction, readonly string[]> = {
  list: ["store_id"], invite: ["store_id", "email", "role", "locale"], "revoke-invite": ["store_id", "invite_id"],
  "set-role": ["store_id", "principal_id", "role"], remove: ["store_id", "principal_id"], accept: ["token"],
};
const uuid = (v: unknown) => typeof v === "string" && canonicalUUID.test(v);
// Same shape check as Go's NormalizeEmail precondition: the server re-validates and normalizes; this only keeps junk off the wire.
const email = (v: unknown) => typeof v === "string" && v.length >= 3 && v.length <= 254 && /^[^\s@]+@[^\s@]+$/.test(v);

/** True when `body` (already key-exact) carries valid values for `action`. The server stays the authority. */
export function validTeamBody(action: TeamAction, body: Record<string, unknown>): boolean {
  switch (action) {
    case "list": return uuid(body.store_id);
    case "invite": return uuid(body.store_id) && email(body.email) && isRole(body.role) && typeof body.locale === "string" && ["zh-CN", "zh-TW", "en"].includes(body.locale);
    case "revoke-invite": return uuid(body.store_id) && uuid(body.invite_id);
    case "set-role": return uuid(body.store_id) && uuid(body.principal_id) && isRole(body.role);
    case "remove": return uuid(body.store_id) && uuid(body.principal_id);
    case "accept": return typeof body.token === "string" && /^[A-Za-z0-9_-]{43}$/.test(body.token);
  }
}

// Go codes the UI knows how to say (internal/identityhttp/staff.go); anything else is shown as a retry-later.
export const teamCodes = new Set([
  "invalid_request", "invalid_email", "unauthorized", "forbidden", "not_found", "invite_invalid", "already_member",
  "last_owner", "too_many_invitations", "conflict", "unavailable", "retry_later",
]);

// ---- role-aware navigation (WorkspaceFrame) ---------------------------------------------------------------------------
// Entry id -> the permission its page needs. `team` is the owner role itself (no permission value, see migration 0089).
// The data is the caller's own role/permissions from GET /api/stores (0089 list_session_stores); unknown (null) shows everything,
// because this only hides entries the member could not use: Go still authorizes every request.
export type NavAccess = { role: string | null; permissions: string[] } | null;
const navNeeds: Record<string, string> = {
  products: "catalog:read", inventory: "inventory:read", orders: "orders:read", live: "live:read", customers: "customers:read",
  finance: "orders:read", billing: "billing:manage", ads: "ads:read", settings: "integration:read",
};
export function navVisible(id: string, access: NavAccess): boolean {
  if (access === null) return true;
  if (id === "team") return access.role === "owner";
  const need = navNeeds[id];
  return need === undefined || access.permissions.includes(need);
}
/** The caller's access in `storeId` (or the first store by id, like the page loaders) from a GET /api/stores answer; null when unknown. */
export function navAccessFrom(body: unknown, storeId: string | null): NavAccess {
  const items = body && typeof body === "object" ? (body as { items?: unknown }).items : null;
  if (!Array.isArray(items)) return null;
  const stores = items.filter((i): i is { id: string; role?: unknown; permissions?: unknown } => !!i && typeof i === "object" && typeof (i as { id?: unknown }).id === "string");
  const store = storeId ? stores.find((s) => s.id === storeId) : [...stores].sort((a, b) => a.id.localeCompare(b.id))[0];
  if (!store || !Array.isArray(store.permissions) || !store.permissions.every((p) => typeof p === "string")) return null;
  return { role: typeof store.role === "string" ? store.role : null, permissions: store.permissions as string[] };
}
