export const studioUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
export const studioCursor = /^[A-Za-z0-9_-]{1,1024}$/;
export type AspectRatio = "16:9" | "9:16";
export type Destination = { ordinal: number; provider: "facebook" | "instagram" };
export type Draft = {
  session_id: string;
  program_id: string;
  title: string;
  scheduled_at: string | null;
  aspect_ratio: AspectRatio;
  state: string;
  version: number;
  created_at: string;
  updated_at: string;
};
export type Prepared = {
  authorization_id: string;
  session_version: number;
  start_before: string;
  environment: "MOCK";
  destinations: Destination[];
};
export type Attempt = {
  attempt_id: string;
  environment: "MOCK";
  operation_state: string;
  resource_state: "UNOBSERVED" | "OBSERVED" | "TERMINAL";
  transport_status: string;
  cleanup_required: boolean;
  stop_requested: boolean;
  stop_wire_count: number;
  escalated: boolean;
  updated_at: string;
  destinations: Destination[];
};
export type StudioDetail = {
  draft: Draft;
  prepared: Prepared | null;
  attempt: Attempt | null;
  can_manage: boolean;
};
export type StudioPage = { items: Draft[]; next_cursor: string };
export type StudioReceipt = { session_id: string; attempt_id: string; state: string };

function exact(value: unknown, fields: string[]): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("invalid_studio_response");
  const row = value as Record<string, unknown>;
  if (Object.keys(row).sort().join(",") !== [...fields].sort().join(",")) throw new Error("invalid_studio_response");
  return row;
}
function id(value: unknown): value is string {
  return typeof value === "string" && studioUUID.test(value);
}
function date(value: unknown): value is string {
  if (typeof value !== "string") return false;
  const match = /^(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(?:\.\d{1,9})?(?:Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$/.exec(value);
  if (!match || !Number.isFinite(Date.parse(value))) return false;
  const wall = new Date(`${match[1]}Z`);
  return Number.isFinite(wall.getTime()) && wall.toISOString().slice(0, 19) === match[1];
}
function destinations(value: unknown): Destination[] {
  if (!Array.isArray(value) || value.length < 1 || value.length > 2) throw new Error("invalid_studio_response");
  const rows = value.map((item, index) => {
    const row = exact(item, ["ordinal", "provider"]);
    if (row.ordinal !== index + 1 || (row.provider !== "facebook" && row.provider !== "instagram"))
      throw new Error("invalid_studio_response");
    return row as Destination;
  });
  return rows;
}
export function parseDraft(value: unknown): Draft {
  const row = exact(value, ["session_id", "program_id", "title", "scheduled_at", "aspect_ratio", "state", "version", "created_at", "updated_at"]);
  if (!id(row.session_id) || !id(row.program_id) || typeof row.title !== "string" ||
    row.title.trim() !== row.title || Array.from(row.title).length < 1 || Array.from(row.title).length > 200 ||
    /[\p{Cc}]/u.test(row.title) || (row.scheduled_at !== null && !date(row.scheduled_at)) ||
    (row.aspect_ratio !== "16:9" && row.aspect_ratio !== "9:16") ||
    typeof row.state !== "string" || row.state.length > 40 ||
    !Number.isSafeInteger(row.version) || (row.version as number) < 1 ||
    !date(row.created_at) || !date(row.updated_at)) throw new Error("invalid_studio_response");
  return row as Draft;
}
export function parseStudioPage(value: unknown): StudioPage {
  const row = exact(value, ["items", "next_cursor"]);
  if (!Array.isArray(row.items) || row.items.length > 20 || typeof row.next_cursor !== "string" ||
    (row.next_cursor !== "" && !studioCursor.test(row.next_cursor))) throw new Error("invalid_studio_response");
  const items = row.items.map(parseDraft);
  if (new Set(items.map((item) => item.session_id)).size !== items.length) throw new Error("invalid_studio_response");
  return { items, next_cursor: row.next_cursor };
}
export function parseStudioDetail(value: unknown, requestedID: string): StudioDetail {
  const row = exact(value, ["draft", "prepared", "attempt", "can_manage"]);
  const draft = parseDraft(row.draft);
  if (draft.session_id !== requestedID || typeof row.can_manage !== "boolean" ||
    (row.prepared !== null && row.attempt !== null)) throw new Error("invalid_studio_response");
  let prepared: Prepared | null = null;
  if (row.prepared !== null) {
    const item = exact(row.prepared, ["authorization_id", "session_version", "start_before", "environment", "destinations"]);
    if (!id(item.authorization_id) || item.session_version !== draft.version || !date(item.start_before) || item.environment !== "MOCK")
      throw new Error("invalid_studio_response");
    prepared = { ...item, destinations: destinations(item.destinations) } as Prepared;
  }
  let attempt: Attempt | null = null;
  if (row.attempt !== null) {
    const item = exact(row.attempt, ["attempt_id", "environment", "operation_state", "resource_state", "transport_status", "cleanup_required", "stop_requested", "stop_wire_count", "escalated", "updated_at", "destinations"]);
    if (!id(item.attempt_id) || item.environment !== "MOCK" ||
      !["READY", "DISPATCHING", "UNKNOWN", "ACKNOWLEDGED", "SUCCEEDED", "FAILED_FINAL", "CANCELLED", "BLOCKED_POLICY", "STALE_BINDING"].includes(item.operation_state as string) ||
      !["UNOBSERVED", "OBSERVED", "TERMINAL"].includes(item.resource_state as string) ||
      typeof item.transport_status !== "string" || !["", "EGRESS_STARTING", "EGRESS_ACTIVE", "EGRESS_ENDING", "EGRESS_COMPLETE", "EGRESS_FAILED", "EGRESS_ABORTED", "EGRESS_LIMIT_REACHED"].includes(item.transport_status) ||
      typeof item.cleanup_required !== "boolean" || typeof item.stop_requested !== "boolean" ||
      !Number.isInteger(item.stop_wire_count) || (item.stop_wire_count as number) < 0 || (item.stop_wire_count as number) > 2 ||
      typeof item.escalated !== "boolean" || !date(item.updated_at)) throw new Error("invalid_studio_response");
    attempt = { ...item, destinations: destinations(item.destinations) } as Attempt;
  }
  return { draft, prepared, attempt, can_manage: row.can_manage };
}
export function parseStudioReceipt(value: unknown, sessionID: string): StudioReceipt {
  const row = exact(value, ["session_id", "attempt_id", "state"]);
  if (row.session_id !== sessionID || !id(row.attempt_id) || typeof row.state !== "string" || row.state.length > 40)
    throw new Error("invalid_studio_response");
  return row as StudioReceipt;
}
