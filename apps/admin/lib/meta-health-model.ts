// Purpose: closed Meta health read/recheck shapes; unlabelled legacy capability rows never imply a named ability.
// Depends on: meta-connection-health-v1 §§5/9 and live-console-v1 §6; pure, no I/O.
// Used by: MetaHealthBanner, MetaHealthCapabilities, the health BFF and Node counterexamples.
export const capabilities = ["read_comment", "private_reply", "dm_session", "reply_public"] as const;
export const states = ["ok", "missing_permission", "missing_task", "not_subscribed", "reauth_required", "review_required", "unsupported", "unknown"] as const;
export type Capability = typeof capabilities[number];
export type HealthState = typeof states[number];
export type Severity = "none" | "warning" | "blocking";
export type HealthCapability = { binding_id: string; provider: "facebook" | "instagram"; capability?: Capability; state: HealthState; reason: string; evidence: "DESIGN" | "MOCK" | "LIVE_READ" | "LIVE_SEND"; checked_at?: string };
export type HealthPage = { page_id: string; page_name: string; status: "active" | "reauth_required"; severity: Severity; checked_at?: string; next_check_at?: string; episode_opened_at?: string; capabilities: HealthCapability[] };
export type MetaHealth = { severity: Severity; pages: HealthPage[] };
const bad = (): never => { throw new Error("meta_health_shape"); };
const uuid = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const levels = ["none", "warning", "blocking"] as const;
function object(value: unknown, required: string[], optional: string[] = []): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) return bad();
  const r = value as Record<string, unknown>;
  if (required.some(k => !Object.hasOwn(r, k)) || Object.keys(r).some(k => !required.includes(k) && !optional.includes(k))) return bad();
  return r;
}
function choice<T extends string>(value: unknown, choices: readonly T[]): T { return typeof value === "string" && choices.includes(value as T) ? value as T : bad(); }
function text(value: unknown, max: number): string { return typeof value === "string" && value.length <= max ? value : bad(); }
function instant(value: unknown): string {
  const s = text(value, 35);
  return /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(s) && Number.isFinite(Date.parse(s)) ? s : bad();
}
function optionalTime(value: unknown): string | undefined { return value === undefined ? undefined : instant(value); }
function array(value: unknown, max: number): unknown[] { return Array.isArray(value) && value.length <= max ? value : bad(); }
/** Validates B1; legacy rows without capability are retained as unidentified, never mapped by order. */
export function parseMetaHealth(value: unknown): MetaHealth {
  const r = object(value, ["severity", "pages"]), seenPages = new Set<string>();
  const pages = array(r.pages, 10).map(value => {
    const p = object(value, ["page_id", "page_name", "status", "severity", "capabilities"], ["checked_at", "next_check_at", "episode_opened_at"]);
    const page_id = text(p.page_id, 40);
    if (!/^\d{1,40}$/.test(page_id) || seenPages.has(page_id)) return bad();
    seenPages.add(page_id);
    const seen = new Set<string>();
    const rows: HealthCapability[] = array(p.capabilities, 8).map(value => {
      const c = object(value, ["binding_id", "provider", "state", "reason", "evidence"], ["capability", "checked_at"]);
      const binding_id = text(c.binding_id, 36), reason = text(c.reason, 96);
      if (!uuid.test(binding_id) || !/^[a-z][a-z0-9_]*$/.test(reason)) return bad();
      const capability = c.capability === undefined ? undefined : choice(c.capability, capabilities);
      if (capability) {
        const key = `${binding_id}:${capability}`;
        if (seen.has(key)) return bad();
        seen.add(key);
      }
      return { binding_id, provider: choice(c.provider, ["facebook", "instagram"] as const), capability, state: choice(c.state, states), reason,
        evidence: choice(c.evidence, ["DESIGN", "MOCK", "LIVE_READ", "LIVE_SEND"] as const), checked_at: optionalTime(c.checked_at) };
    });
    return { page_id, page_name: text(p.page_name, 200), status: choice(p.status, ["active", "reauth_required"] as const), severity: choice(p.severity, levels),
      checked_at: optionalTime(p.checked_at), next_check_at: optionalTime(p.next_check_at), episode_opened_at: optionalTime(p.episode_opened_at), capabilities: rows };
  });
  const severity = choice(r.severity, levels);
  if (levels.indexOf(severity) !== Math.max(0, ...pages.map(p => levels.indexOf(p.severity)))) return bad();
  return { severity, pages };
}
/** Validates the scheduling receipt, not completion of a new probe. */
export function parseRecheck(value: unknown): { next_check_at: string } { return { next_check_at: instant(object(value, ["next_check_at"]).next_check_at) }; }
/** Chooses advice from the worst affected Page only; the server owns severity. */
export function healthReason(health: MetaHealth): string {
  const pages = health.pages.filter(p => p.severity === health.severity);
  if (pages.some(p => p.status === "reauth_required")) return "reauth_required";
  const rows = pages.flatMap(p => p.capabilities).filter(c => health.severity !== "blocking" || !c.capability || c.capability === "read_comment" || c.capability === "private_reply");
  // Legacy rows have no identity. Advice must not attribute a secondary problem to the blocking capability.
  if (rows.some(c => !c.capability)) return "unknown";
  for (const state of ["reauth_required", "missing_permission", "missing_task", "not_subscribed", "unsupported", "unknown"]) {
    if (rows.some(c => c.state === state && c.reason !== "lc_u11_open")) return state;
  }
  return "probe_failing";
}
