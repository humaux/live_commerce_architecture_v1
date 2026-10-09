// Purpose: closed W3-U2 resource grammar and safe projections for settings, reminders and restricted buyers.
// Depends on: frozen live-console/W3-03B/W3-05B DTOs; no storage, transport or inferred buyer identity.
// Used by: live-settings proxy/client and LiveSettings/BuyerPanel.
export const settingsUUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const uuid = "[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}";
const session = `live-sessions/${uuid}`;
export type SettingsKind =
  "settings" | "templates" | "reminders" | "blocklist" | "check" | "remove";
const resources: [SettingsKind, RegExp, string[]][] = [
  ["settings", /^live-settings\/sold-out-reply$/, ["GET", "PUT"]],
  ["templates", /^message-templates$/, ["GET", "POST"]],
  ["reminders", new RegExp(`^${session}/reminders$`), ["GET", "POST"]],
  ["blocklist", new RegExp(`^${session}/claims/blocklist$`), ["GET", "POST"]],
  ["check", new RegExp(`^${session}/claims/blocklist/check$`), ["GET"]],
  [
    "remove",
    new RegExp(`^${session}/claims/blocklist/entries/${uuid}$`),
    ["DELETE"],
  ],
];
export type SoldOutSettings = {
  enabled: boolean;
  template_id: string;
  template_version: number;
  version: number;
};
export type TemplateReceipt = {
  template_id: string;
  version: number;
  kinds: string[];
  public_safe: boolean;
};
export type Followup = {
  bundle_id: string;
  display_name: string | null;
  reminder_state: string;
  reason: string;
  link_copy_allowed: boolean;
};
export type ReminderReport = {
  sent: unknown[];
  queued: number;
  failed: unknown[];
  followup: Followup[];
  link: string | null;
};
export type ReminderResult = {
  queued: number;
  already_reminded: number;
  followup: number;
  restricted: number;
  refused: number;
  truncated: boolean;
};
export type RestrictedEntry = {
  id: string;
  platform: string;
  note: string | null;
  source_bundle_id: string | null;
  created_at: string;
};
export type RestrictedPage = { items: RestrictedEntry[]; next_cursor: string };
export type SettingsReceipt = {
  key: string;
  method: "POST" | "PUT" | "DELETE";
  resource: string;
  body?: string;
  afterPublish?: { enabled: boolean; expected_version: number };
};
const fixed = new Set([
  "sold-out-reply/v1",
  "order-pay-link/v1",
  "offer-recommend/v1",
  "checkout-reminder/v1",
]);
/** A returned merchant template may be selected; fixed template content remains immutable. */
export const merchantTemplate = (id: string) =>
  /^[a-z0-9][a-z0-9/_-]{0,63}$/.test(id) && !fixed.has(id);
/** Resolve exact resource paths, including the methods used to construct 405 responses. */
export function settingsResource(path: string) {
  const r = resources.find(([, re]) => re.test(path));
  return r ? { kind: r[0], methods: r[2] } : null;
}
/** Permissions remain independently enforced by Go, using the authenticated principal. */
export function settingsPermissions(kind: SettingsKind, method: string) {
  if (kind === "reminders")
    return method === "GET" ? ["inbox:read"] : ["live:manage", "inbox:reply"];
  if (kind === "templates")
    return method === "GET" ? ["inbox:reply"] : ["live:manage"];
  return [method === "GET" ? "live:read" : "live:manage"];
}
/** Admit only bounded pagination/opaque references; never allow note/text/name in a URL. */
export function settingsQuery(kind: SettingsKind, method: string, url: URL) {
  // A bare "?" leaves url.search empty but survives in href; Go answers any query (even empty) with 422.
  if (url.href.includes("?") && !url.search) return false;
  const q = url.searchParams;
  if (method !== "GET") return !url.search;
  if (kind === "check")
    return (
      [...q].length === 1 &&
      q.getAll("bundle_id").length === 1 &&
      settingsUUID.test(q.get("bundle_id") ?? "")
    );
  for (const [k, v] of q) {
    if (kind !== "blocklist" || q.getAll(k).length !== 1) return false;
    if (k === "limit" && /^(?:[1-9]|[1-4][0-9]|50)$/.test(v)) continue;
    if (k === "cursor" && /^[A-Za-z0-9_-]{1,2048}$/.test(v)) continue;
    return false;
  }
  return true;
}
const obj = (v: unknown): Record<string, unknown> => {
  if (!v || typeof v !== "object" || Array.isArray(v)) throw Error("invalid");
  return v as Record<string, unknown>;
};
const str = (v: unknown, max = 256) => {
  if (typeof v !== "string" || Array.from(v).length > max)
    throw Error("invalid");
  return v;
};
const integer = (v: unknown, min = 0) => {
  if (typeof v !== "number" || !Number.isSafeInteger(v) || v < min)
    throw Error("invalid");
  return v;
};
const bool = (v: unknown) => {
  if (typeof v !== "boolean") throw Error("invalid");
  return v;
};
const id = (v: unknown) => {
  const s = str(v, 36);
  if (!settingsUUID.test(s)) throw Error("invalid");
  return s;
};
const list = (v: unknown, max = 5000) => {
  if (!Array.isArray(v) || v.length > max) throw Error("invalid");
  return v as unknown[];
};
const time = (v: unknown) => {
  const s = str(v, 64);
  if (!Number.isFinite(Date.parse(s))) throw Error("invalid");
  return s;
};
const exact = (
  v: Record<string, unknown>,
  required: string[],
  optional: string[] = [],
) =>
  required.every((k) => Object.hasOwn(v, k)) &&
  Object.keys(v).every((k) => required.includes(k) || optional.includes(k));
/** Validate the exact command before body forwarding; caller-generated actor keys/authority never pass. */
export function settingsBody(kind: SettingsKind, raw: string): boolean {
  try {
    const v = obj(JSON.parse(raw));
    if (kind === "settings")
      return (
        exact(v, [
          "enabled",
          "template_id",
          "template_version",
          "expected_version",
        ]) &&
        typeof v.enabled === "boolean" &&
        merchantTemplate(str(v.template_id, 64)) &&
        integer(v.template_version, 1) > 0 &&
        integer(v.expected_version) >= 0
      );
    if (kind === "templates")
      return (
        exact(v, ["template_id", "name", "body", "kinds", "public_safe"]) &&
        merchantTemplate(str(v.template_id, 64)) &&
        !!str(v.name, 100).trim() &&
        validSoldOutText(str(v.body, 280)) &&
        JSON.stringify(v.kinds) === '["private_reply"]' &&
        v.public_safe === false
      );
    if (kind === "blocklist") {
      const refs = ["bundle_id", "conversation_id", "comment_ref"].filter((k) =>
        Object.hasOwn(v, k),
      );
      if (refs.length !== 1 || !exact(v, refs, ["note"])) return false;
      if (
        refs[0] === "comment_ref"
          ? !/^[0-9]{1,20}$/.test(str(v.comment_ref, 20))
          : !settingsUUID.test(str(v[refs[0]], 36))
      )
        return false;
      return (
        v.note === undefined ||
        (typeof v.note === "string" &&
          Array.from(v.note).length <= 200 &&
          !/\p{Cc}/u.test(v.note))
      );
    }
    return false;
  } catch {
    return false;
  }
}
/** Sold-out templates have one optional product-name placeholder and no control characters. */
export function validSoldOutText(text: string) {
  return (
    !!text.trim() &&
    Array.from(text).length <= 280 &&
    !/\p{Cc}/u.test(text) &&
    (text.match(/\{\{product\.name\}\}/g) ?? []).length <= 1 &&
    !/[{}]/.test(text.replace("{{product.name}}", ""))
  );
}
/** Project trusted shapes only; upstream diagnostics and actor identifiers never reach the page. */
export function settingsData(
  kind: SettingsKind,
  method: string,
  value: unknown,
): unknown {
  const v = obj(value);
  if (kind === "settings")
    return {
      enabled: bool(v.enabled),
      template_id: str(v.template_id, 64),
      template_version: integer(v.template_version, 1),
      version: integer(v.version),
    } satisfies SoldOutSettings;
  if (kind === "templates") {
    const template = (x: unknown) => {
      const t = obj(x);
      return {
        template_id: str(t.template_id, 64),
        version: integer(t.version, 1),
        kinds: list(t.kinds, 4).map((x) => str(x, 32)),
        public_safe: bool(t.public_safe),
      };
    };
    return method === "POST"
      ? template(v)
      : {
          items: list(v.items, 500).map((x) => {
            const t = obj(x);
            return {
              ...template(t),
              name: str(t.name, 100),
              created_at: time(t.created_at),
            };
          }),
        };
  }
  if (kind === "check") return { restricted: bool(v.restricted) };
  if (kind === "remove") {
    if (v.removed !== true) throw Error("invalid");
    return { removed: true };
  }
  if (kind === "blocklist") {
    if (method === "POST")
      return {
        id: id(v.id),
        platform: str(v.platform, 20),
        created_at: time(v.created_at),
        created: bool(v.created),
      };
    return {
      items: list(v.items, 50).map((x) => {
        const t = obj(x);
        return {
          id: id(t.id),
          platform: str(t.platform, 20),
          note: t.note === null ? null : str(t.note, 200),
          source_bundle_id:
            t.source_bundle_id === null ? null : id(t.source_bundle_id),
          created_at: time(t.created_at),
        };
      }),
      next_cursor: str(v.next_cursor, 2048),
    } satisfies RestrictedPage;
  }
  if (method === "POST")
    return {
      queued: integer(v.queued),
      already_reminded: integer(v.already_reminded),
      followup: integer(v.followup),
      restricted: integer(v.restricted),
      refused: integer(v.refused),
      truncated: bool(v.truncated),
    } satisfies ReminderResult;
  const item = (x: unknown) => {
    const t = obj(x);
    return {
      bundle_id: id(t.bundle_id),
      display_name: t.display_name === null ? null : str(t.display_name, 255),
      reminder_state: str(t.reminder_state, 40),
      ...(t.reason === undefined ? {} : { reason: str(t.reason, 64) }),
      ...(t.send_state === undefined
        ? {}
        : { send_state: str(t.send_state, 64) }),
    };
  };
  let link: string | null = null;
  if (v.link !== null) {
    link = str(v.link, 2048);
    const u = new URL(link);
    if (
      u.protocol !== "https:" ||
      u.username ||
      u.password ||
      u.search ||
      u.hash
    )
      throw Error("invalid");
  }
  return {
    sent: list(v.sent).map(item),
    queued: integer(v.queued),
    failed: list(v.failed).map(item),
    followup: list(v.followup).map((x) => {
      const t = obj(x),
        reason = str(t.reason, 64);
      return {
        ...item(t),
        reason,
        link_copy_allowed: reason !== "restricted" && bool(t.link_copy_allowed),
      };
    }),
    link,
  } satisfies ReminderReport;
}
/** Boundary errors carry only a closed code, never an upstream diagnostic. */
export const settingsCodes = new Set([
  "unauthorized",
  "forbidden",
  "not_found",
  "conflict",
  "version_conflict",
  "idempotency_conflict",
  "invalid_request",
  "invalid_json",
  "limit_reached",
  "ambiguous_actor",
  "not_remindable",
  "no_storefront",
  "rate_limited",
  "template_fixed",
  "retry_later",
  "unavailable",
]);
