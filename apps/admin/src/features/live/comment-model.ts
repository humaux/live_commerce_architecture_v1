// Purpose: transient A2 stream parsing, epoch reconciliation and filter/cadence policy.
// Depends on: live-console-v1 CommentStream and A8 inbox DTOs; no browser storage or identity inference.
// Used by: CommentStream hook and Node acceptance tests.
// Invariant: deduped buffer is oldest-first by (created_at instant, ref); cap evicts only the oldest rows.
// Deletions are inferred only from a non-empty, same-epoch periodic HEAD window, never incremental/history reads.
import type { ConversationItem, ConversationList } from "../../../lib/inbox-types";
/** I23 bounds both console comment and conversation memory. */
export const COMMENT_MEMORY_CAP = 1000;
export type CommentFilter = "all" | "keyword" | "private" | "unreplied";
/** A2 SQL marks carry operation enums; render the same five delivery states as Go inbox.sendState. */
export function commentSendState(state:string):"queued"|"sent"|"failed"|"blocked"|"unknown" {
  if(["READY","DISPATCHING","queued"].includes(state))return "queued";
  if(["SUCCEEDED","ACKNOWLEDGED","sent"].includes(state))return "sent";
  if(["FAILED_FINAL","failed"].includes(state))return "failed";
  if(["BLOCKED_POLICY","STALE_BINDING","CANCELLED","blocked"].includes(state))return "blocked";
  return "unknown";
}
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
  historyLoaded: boolean;
};
/** A privacy boundary always starts with an empty in-memory buffer. */
export const emptyComments = (): CommentBuffer => ({
  epoch: 0,
  items: [],
  next: null,
  older: null,
  reset: false,
  historyLoaded: false,
});
const compareComments = (a: StreamComment, b: StreamComment) =>
  Date.parse(a.created_at) - Date.parse(b.created_at) || (a.ref < b.ref ? -1 : a.ref > b.ref ? 1 : 0);
/** Merge transient rows; reconcileHead is enabled only by the hook's periodic HEAD read. Historical reads never move the live cursor. */
export function applyCommentPage(
  old: CommentBuffer,
  page: CommentPage,
  older: boolean,
  reconcileHead = false,
  headLimit = 50,
): CommentBuffer {
  if (page.reset || (old.epoch !== 0 && old.epoch !== page.epoch))
    return { ...emptyComments(), reset: true };
  // Historical paging stops at the memory cap: never consume a cursor for an unseen page.
  if (older && old.items.length >= COMMENT_MEMORY_CAP) return old;
  let retained = old.items;
  if (reconcileHead && !older && page.items.length > 0) {
    const floor = page.items.reduce((a, b) => compareComments(a, b) <= 0 ? a : b);
    const refs = new Set(page.items.map(row => row.ref));
    // Explicit history uses the conservative floor rule. Without it, a short terminal
    // head is the complete server window, including any now-deleted oldest live rows.
    const wholeWindow = page.items.length < headLimit && page.older_cursor === null && !old.historyLoaded;
    retained = old.items.filter(row => refs.has(row.ref) || (!wholeWindow && compareComments(row, floor) < 0));
  }
  const map = new Map(
    (older ? [...page.items, ...retained] : [...retained, ...page.items]).map(
      (row) => [row.ref, row],
    ),
  );
  return {
    epoch: page.epoch,
    // FB bridge and IG fallback arrive in opposite orders. Never use arrival order for retention.
    items: [...map.values()].sort(compareComments).slice(-COMMENT_MEMORY_CAP),
    next: older ? old.next : page.next,
    older: older || !old.epoch ? page.older_cursor : old.older,
    reset: false,
    historyLoaded: old.historyLoaded || (older && page.items.length > 0),
  };
}
/** Merge A8 pages in memory, evicting oldest entries; head refresh may preserve the loaded history cursor. */
export function mergeConversationPage(old: ConversationList | null, page: ConversationList, preserveCursor = false): ConversationList {
  const identity = (row: ConversationItem) => row.conversation_id ?? row.bundle_id ?? "";
  const items = [...new Map([...(old?.items ?? []), ...page.items].map(row => [identity(row), row])).values()]
    .sort((a, b) => Date.parse(b.last_at) - Date.parse(a.last_at) || (identity(a) < identity(b) ? 1 : identity(a) > identity(b) ? -1 : 0))
    .slice(0, COMMENT_MEMORY_CAP);
  return { ...page, items, next_cursor: items.length >= COMMENT_MEMORY_CAP ? "" : preserveCursor && old ? old.next_cursor : page.next_cursor };
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
        /^[A-Za-z0-9_.-]{1,1024}$/.test(r.older_cursor))
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
