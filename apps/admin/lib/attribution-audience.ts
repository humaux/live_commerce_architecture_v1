// Pure UI journal/ack grammar for POST ads/sessions/{session}/audience-read.
// 0113 plan_meta_audience replays 0008 operation states during in-flight/cooldown windows.
// A receipt is not necessarily a new queued read. Uncertain requests retain their exact key.
const uuid = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const states = [
  "READY",
  "DISPATCHING",
  "UNKNOWN",
  "ACKNOWLEDGED",
  "SUCCEEDED",
  "FAILED_FINAL",
  "CANCELLED",
  "BLOCKED_POLICY",
  "STALE_BINDING",
] as const;
export type AudienceState = (typeof states)[number];
export type AudienceAck = { operation_id: string; state: AudienceState };
export function parseAudienceAck(value: unknown): AudienceAck {
  const v =
    value && typeof value === "object" && !Array.isArray(value)
      ? (value as Record<string, unknown>)
      : null;
  if (
    !v ||
    typeof v.operation_id !== "string" ||
    !uuid.test(v.operation_id) ||
    typeof v.state !== "string" ||
    !states.includes(v.state as AudienceState)
  )
    throw new Error("audience_ack_shape");
  return { operation_id: v.operation_id, state: v.state as AudienceState };
}
export type AudiencePhase =
  "unknown" | "queued" | "inflight" | "completed" | "unconfirmed" | "failed";
export function audienceAckPhase(
  ack: AudienceAck,
): Exclude<AudiencePhase, "unknown"> {
  const phases = {
    READY: "queued",
    DISPATCHING: "inflight",
    ACKNOWLEDGED: "inflight",
    SUCCEEDED: "completed",
    UNKNOWN: "unconfirmed",
    FAILED_FINAL: "failed",
    CANCELLED: "failed",
    BLOCKED_POLICY: "failed",
    STALE_BINDING: "failed",
  } as const;
  return phases[ack.state];
}
export type AudienceJournal = {
  key: string;
  boundary: string;
  phase: AudiencePhase;
  operation_id: string | null;
  state?: AudienceState;
};
export const audienceStorageKey = (store: string, session: string) =>
  `commerce-audience-read:${store}:${session}`;
export function parseAudienceJournal(raw: string): AudienceJournal {
  const v: unknown = JSON.parse(raw);
  if (!v || typeof v !== "object" || Array.isArray(v))
    throw new Error("audience_journal_shape");
  const o = v as Record<string, unknown>;
  if (
    typeof o.key !== "string" ||
    !/^audience-read-[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(
      o.key,
    ) ||
    typeof o.boundary !== "string" ||
    !/^[0-9a-f]{64}$/.test(o.boundary) ||
    typeof o.phase !== "string"
  )
    throw new Error("audience_journal_shape");
  const identity = {
    key: o.key,
    boundary: o.boundary,
  };
  if (o.phase === "unknown") {
    if (o.operation_id !== null || o.state !== undefined)
      throw new Error("audience_journal_shape");
    return { ...identity, phase: "unknown", operation_id: null };
  }
  // Existing READY-only journals lack state; this is the sole safe migration.
  const ack = parseAudienceAck({
    operation_id: o.operation_id,
    state: o.phase === "queued" && o.state === undefined ? "READY" : o.state,
  });
  const phase = audienceAckPhase(ack);
  if (o.phase !== phase) throw new Error("audience_journal_shape");
  return { ...identity, phase, ...ack };
}
