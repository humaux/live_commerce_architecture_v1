// Purpose: transient A2 stream parsing, epoch reconciliation and filter/cadence policy.
// Depends on: live-console-v1 CommentStream DTO; no browser storage or identity inference.
// Used by: CommentStream hook and Node acceptance tests.
export type CommentFilter = "all" | "keyword" | "private" | "unreplied";
export type CommentCursor = { epoch: number; seq: number };
export type StreamComment = {
  ref: string;
  parent_ref: string | null;
  created_at: string;
  author_name: string | null;
  text: string;
  is_page: boolean;
  has_attachment: boolean;
  marks: {
    intake: { state: string; drop_reason: string | null } | null;
    claim: {
      status: string;
      reason: string | null;
      offer_id: string | null;
      keyword: string | null;
      quantity: number | null;
      bundle_id: string | null;
    } | null;
    private_reply: {
      kind: string;
      state: string;
      blocked_reason: string | null;
    } | null;
    printed: { count: number; last_at: string | null } | null;
    public_replies: number;
    private_reply_available: boolean;
    private_reply_unavailable_reason?: string;
  };
};
export type CommentPage = {
  epoch: number;
  reset: boolean;
  items: StreamComment[];
  next: CommentCursor;
  older_cursor: string | null;
  stream: {
    state: string;
    source_platform: "facebook" | "instagram";
    video_embeddable: boolean;
  };
};
export type CommentBuffer = {
  epoch: number;
  items: StreamComment[];
  next: CommentCursor | null;
  older: string | null;
  reset: boolean;
};
/** A privacy boundary always starts with an empty in-memory buffer. */
export const emptyComments = (): CommentBuffer => ({
  epoch: 0,
  items: [],
  next: null,
  older: null,
  reset: false,
});
/** Explicit reset/epoch drift clears old data before a fresh read; historical reads never move the live cursor. */
export function applyCommentPage(
  old: CommentBuffer,
  page: CommentPage,
  older: boolean,
): CommentBuffer {
  if (page.reset || (old.epoch !== 0 && old.epoch !== page.epoch))
    return { ...emptyComments(), reset: true };
  const map = new Map(
    (older ? [...page.items, ...old.items] : [...old.items, ...page.items]).map(
      (row) => [row.ref, row],
    ),
  );
  return {
    epoch: page.epoch,
    items: [...map.values()].slice(-1000),
    next: older ? old.next : page.next,
    older: older || !old.epoch ? page.older_cursor : old.older,
    reset: false,
  };
}
/** A8 filters stay server scoped; unreplied is a view over this session's live-comment conversations. */
export function commentViewResource(
  filter: CommentFilter,
  sid: string,
): string | null {
  return filter === "private" || filter === "unreplied"
    ? `inbox/conversations?filter=live_comment&session_id=${sid}`
    : null;
}
/** Healthy 3-second cadence and failure backoff; the caller resets failures after success. */
export const commentDelay = (failures: number) =>
  Math.min(30000, 3000 * 2 ** Math.max(0, failures - 1));
/** Reject malformed/private surprise fields before exposing Go's transient comment page to UI. */
export function parseCommentPage(value: unknown): CommentPage {
  const fail = (): never => {
    throw new Error("invalid_comment_response");
  };
  const obj = (v: unknown): Record<string, unknown> =>
    v && typeof v === "object" && !Array.isArray(v)
      ? (v as Record<string, unknown>)
      : fail();
  const num = (v: unknown) =>
    typeof v === "number" && Number.isSafeInteger(v) && v >= 0;
  const ref = (v: unknown) => typeof v === "string" && /^[0-9_]{1,80}$/.test(v);
  const nullableText = (v: unknown) => v === null || typeof v === "string";
  const exact = (v: unknown, keys: string[], optional: string[] = []) => {
    const r = obj(v);
    if (
      keys.some((k) => !Object.hasOwn(r, k)) ||
      Object.keys(r).some((k) => !keys.includes(k) && !optional.includes(k))
    )
      fail();
    return r;
  };
  const r = exact(value, [
      "epoch",
      "reset",
      "items",
      "next",
      "older_cursor",
      "stream",
    ]),
    next = exact(r.next, ["epoch", "seq"]),
    stream = obj(r.stream);
  if (
    !num(r.epoch) ||
    !num(next.epoch) ||
    next.epoch !== r.epoch ||
    !num(next.seq) ||
    typeof r.reset !== "boolean" ||
    !Array.isArray(r.items) ||
    r.items.length > 100 ||
    !(
      r.older_cursor === null ||
      (typeof r.older_cursor === "string" &&
        /^[A-Za-z0-9_-]{1,2048}$/.test(r.older_cursor))
    ) ||
    !["facebook", "instagram"].includes(String(stream.source_platform)) ||
    typeof stream.video_embeddable !== "boolean"
  )
    fail();
  for (const raw of r.items as unknown[]) {
    const row = exact(raw, [
      "ref",
      "parent_ref",
      "created_at",
      "author_name",
      "text",
      "is_page",
      "has_attachment",
      "marks",
    ]);
    if (
      !ref(row.ref) ||
      !(row.parent_ref === null || ref(row.parent_ref)) ||
      typeof row.created_at !== "string" ||
      !Number.isFinite(Date.parse(row.created_at)) ||
      !nullableText(row.author_name) ||
      typeof row.text !== "string" ||
      typeof row.is_page !== "boolean" ||
      typeof row.has_attachment !== "boolean"
    )
      fail();
    const m = exact(
      row.marks,
      [
        "intake",
        "claim",
        "private_reply",
        "printed",
        "public_replies",
        "private_reply_available",
      ],
      ["private_reply_unavailable_reason"],
    );
    if (
      !num(m.public_replies) ||
      typeof m.private_reply_available !== "boolean" ||
      (m.private_reply_unavailable_reason !== undefined &&
        typeof m.private_reply_unavailable_reason !== "string")
    )
      fail();
    if (m.claim !== null) {
      const c = exact(m.claim, [
        "status",
        "reason",
        "offer_id",
        "keyword",
        "quantity",
        "bundle_id",
      ]);
      if (
        typeof c.status !== "string" ||
        ![c.reason, c.offer_id, c.keyword, c.bundle_id].every(nullableText) ||
        !(c.quantity === null || num(c.quantity))
      )
        fail();
    }
    if (m.private_reply !== null) {
      const p = exact(m.private_reply, ["kind", "state", "blocked_reason"]);
      if (
        typeof p.kind !== "string" ||
        typeof p.state !== "string" ||
        !nullableText(p.blocked_reason)
      )
        fail();
    }
    if (m.intake !== null) {
      const i = exact(m.intake, ["state", "drop_reason"]);
      if (typeof i.state !== "string" || !nullableText(i.drop_reason)) fail();
    }
    if (m.printed !== null) {
      const p = exact(m.printed, ["count", "last_at"]);
      if (!num(p.count) || !nullableText(p.last_at)) fail();
    }
  }
  return r as unknown as CommentPage;
}
