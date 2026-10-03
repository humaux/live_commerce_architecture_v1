// Pure UI journal/ack grammar for POST ads/sessions/{session}/audience-read.
// READY acknowledges a queued read, never a completed Meta fetch. Unknown attempts retain the same key.
const uuid = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
export type AudienceAck = { operation_id: string; state: "READY" };
export function parseAudienceAck(value: unknown): AudienceAck {
  const v =
    value && typeof value === "object" && !Array.isArray(value)
      ? (value as Record<string, unknown>)
      : null;
  if (
    !v ||
    typeof v.operation_id !== "string" ||
    !uuid.test(v.operation_id) ||
    v.state !== "READY"
  )
    throw new Error("audience_ack_shape");
  return { operation_id: v.operation_id, state: "READY" };
}
export type AudienceJournal = {
  key: string;
  boundary: string;
  phase: "unknown" | "queued";
  operation_id: string | null;
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
    (o.phase !== "unknown" && o.phase !== "queued") ||
    (o.phase === "unknown"
      ? o.operation_id !== null
      : typeof o.operation_id !== "string" || !uuid.test(o.operation_id))
  )
    throw new Error("audience_journal_shape");
  return {
    key: o.key,
    boundary: o.boundary,
    phase: o.phase,
    operation_id: o.operation_id as string | null,
  };
}
