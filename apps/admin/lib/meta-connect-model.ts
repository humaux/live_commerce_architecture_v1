// Closed parsers for the merchant Page / Instagram connect answers (internal/httpapi/meta_connect.go -> integration.meta_connect_status
// and meta_connect_get_state, migration 0095). Shared by lib/meta-connect-client.ts and tests/admin/meta-connect-model.test.ts.
// Pure: no fetch, no React; any unknown or missing key, bad id or bad stamp throws (reads as "unavailable", never as state).

export type ConnectStatus =
  | { connected: false }
  | {
      connected: true;
      status: "active" | "reauth_required";
      page: { id: string; name: string };
      instagram: { id: string; username: string } | null;
      permissions: string[];
      connected_at: string;
      route_expires_at: string;
      last_event_at: string | null;
    };
export type PickPage = { page_id: string; name: string; ig_id?: string; ig_username?: string; missing: string[]; ig_missing: string[] };
export type PickState = { state_id: string; expires_at: string; scopes: string[]; pages: PickPage[] };

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const digits = /^[0-9]{1,40}$/;
const name = /^[a-z_]{1,64}$/;
const stamp = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?Z$/;
const bad = (): never => { throw new Error("meta_connect_shape"); };
const record = (value: unknown, keys: string[]): Record<string, unknown> => {
  if (!value || typeof value !== "object" || Array.isArray(value)) return bad();
  const row = value as Record<string, unknown>;
  if (Object.keys(row).sort().join(",") !== [...keys].sort().join(",")) bad();
  return row;
};
const names = (value: unknown, max: number): string[] =>
  Array.isArray(value) && value.length <= max && value.every((x) => typeof x === "string" && name.test(x)) ? (value as string[]) : bad();
const label = (value: unknown) => (typeof value === "string" && value.length <= 200 ? value : bad());
const time = (value: unknown) => (typeof value === "string" && stamp.test(value) && Number.isFinite(Date.parse(value)) ? value : bad());

export function parseStatus(value: unknown): ConnectStatus {
  if (value && typeof value === "object" && (value as Record<string, unknown>).connected === false) {
    record(value, ["connected"]);
    return { connected: false };
  }
  const row = record(value, ["connected", "status", "page", "instagram", "permissions", "connected_at", "route_expires_at", "last_event_at"]);
  if (row.connected !== true || (row.status !== "active" && row.status !== "reauth_required")) bad();
  const page = record(row.page, ["id", "name"]);
  if (typeof page.id !== "string" || !digits.test(page.id)) bad();
  let instagram: { id: string; username: string } | null = null;
  if (row.instagram !== null) {
    const ig = record(row.instagram, ["id", "username"]);
    if (typeof ig.id !== "string" || !digits.test(ig.id)) bad();
    instagram = { id: ig.id as string, username: label(ig.username) };
  }
  return {
    connected: true, status: row.status as "active" | "reauth_required", page: { id: page.id as string, name: label(page.name) }, instagram,
    permissions: names(row.permissions, 64), connected_at: time(row.connected_at), route_expires_at: time(row.route_expires_at),
    last_event_at: row.last_event_at === null ? null : time(row.last_event_at),
  };
}

export function parsePickState(value: unknown, id: string): PickState {
  const row = record(value, ["state_id", "expires_at", "scopes", "pages"]);
  if (row.state_id !== id || !uuid.test(id) || !Array.isArray(row.pages) || row.pages.length > 25) bad();
  const pages = (row.pages as unknown[]).map((item): PickPage => {
    const o = item && typeof item === "object" && !Array.isArray(item) ? (item as Record<string, unknown>) : bad();
    const keys = Object.keys(o);
    const base = ["page_id", "name", "missing", "ig_missing"];
    const withIG = [...base, "ig_id", "ig_username"];
    const ok = (set: string[]) => keys.length === set.length && set.every((k) => k in o);
    if (!(ok(base) || ok(withIG)) || typeof o.page_id !== "string" || !digits.test(o.page_id)) bad();
    const page: PickPage = { page_id: o.page_id as string, name: label(o.name), missing: names(o.missing, 16), ig_missing: names(o.ig_missing, 16) };
    if ("ig_id" in o) {
      if (typeof o.ig_id !== "string" || !digits.test(o.ig_id)) bad();
      page.ig_id = o.ig_id as string;
      page.ig_username = label(o.ig_username);
    }
    return page;
  });
  return { state_id: id, expires_at: time(row.expires_at), scopes: names(row.scopes, 64), pages };
}

/** A pick row's permission/task codes -> what the merchant must re-grant (permission names and the two Page tasks). */
export const pickable = (p: PickPage, withInstagram: boolean) => p.missing.length === 0 && (!withInstagram || (!!p.ig_id && p.ig_missing.length === 0));
