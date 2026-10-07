// Purpose: Closed DTO parsers for LC-U1 console A1, lifecycle A7, recommend A6 and A5 session results/copy.
// Depends on: studio-model.ts draft parser and claims-model.ts offer/import parsers; frozen live-console-v1 §§6–9/11.
// Used by: console-client.ts, LiveConsole and LiveWorkspace; the stores BFF validates these same public shapes.
// Invariants: I03/I05/I10/I11/I14; no client amount/stock truth or guessed external state; LC-B7 owns the real read model.
import { parseDraft, type Draft } from "../../../lib/studio-model.ts";
import { matchModes, parseImportResult, type ImportResult, type MatchMode } from "../../../lib/claims-model.ts";

/** Session lifecycle, separate from media programme/operation state. */
export type Lifecycle = "draft" | "live" | "ended" | "archived";
/** Wire values returned by claims.MatchMode in A1, A5 and A7. */
export type ConsoleMatchMode = MatchMode;
/** Full claims.Window returned by A7 and A5 copy. */
export type ConsoleClaimWindow = {
  session_id: string; state: "OPEN" | "CLOSED"; match_mode: ConsoleMatchMode; generation: number;
  version: number; opened_at: string | null; closed_at: string | null;
};
/** A1 server-authoritative offer; display only, never recomputed as inventory/payment truth. */
export type ConsoleOffer = {
  offer_id: string; keyword: string; sku_id: string; product_name: string; variant_label: string; active: boolean;
  version: number; live_price_minor: number | null; sku_price_minor: number;
  stock: { tracked: boolean; sellable: number; reserved: number; warehouse_id: string | null; balance_version: number };
  claimed: { buyers: number; quantity: number }; ordered_qty: number; paid_qty: number; paid_amount_minor: number;
  sold_out: boolean; low_stock: boolean;
};
/** One frozen capability observation; unknown/unsupported remain explicit. */
export type ConsoleCapability = {
  state: "ok" | "missing_permission" | "missing_task" | "not_subscribed" | "reauth_required" | "review_required" | "unsupported" | "unknown";
  reason: string; evidence: "DESIGN" | "MOCK" | "LIVE_READ" | "LIVE_SEND"; checked_at: string | null;
};
/** Platform observations are optional when there is no binding. */
export type ConsoleCapabilities = Partial<Record<"facebook" | "instagram", Partial<Record<"read_comment" | "private_reply" | "dm_session" | "reply_public", ConsoleCapability>>>>;
/** Poller observation, independent of programme lifecycle. */
export type ConsoleStream = {
  state: "live" | "throttled" | "reauth_required" | "unavailable" | "not_started";
  poll_interval_ms: number; last_ok_at: string | null; lag_ms: number | null;
  source_platform: "facebook" | "instagram"; video_embeddable: boolean; reason?: string;
};
/** Frozen LC-B7 A1 snapshot; runtime parsing never substitutes missing authority or array fields. */
export type ConsoleSnapshot = {
  session: { id: string; title: string; lifecycle: Lifecycle; version: number; started_at: string | null; ended_at: string | null };
  window: { state: "OPEN" | "CLOSED"; generation: number; opened_at: string | null; match_mode: ConsoleMatchMode };
  stats: { comments: { total: number | null; source: "graph_summary" | "stream_seen" | "unavailable" };
    keyword_comments: number; buyers: number; orders: { count: number; amount_minor: number };
    paid: { count: number; amount_minor: number }; currency: string; as_of: string };
  offers: ConsoleOffer[]; capabilities: ConsoleCapabilities; stream: ConsoleStream;
  recommended: { offer_id: string; at: string } | null;
};
/** A7 mutation receipt; version belongs to lifecycle_version. */
export type LifecycleResult = { lifecycle: Lifecycle; version: number; window: ConsoleClaimWindow };
/** A5 totals; environments and currencies must remain separate. */
export type SessionResults = { as_of: string; items: { session_id: string; orders: number; paid_orders: number;
  multi_session_orders: number; money: { currency: string; order_minor: number; paid_minor: number; sandbox_paid_minor: number }[] }[] };
/** A5 copy receipt: new draft, closed seeded window and imported offer results. */
export type CopyResult = ImportResult & { session: Draft; window: ConsoleClaimWindow; source_version: number };
/** A6 marks a local recommendation; an operation id does not establish successful external delivery. */
export type RecommendResult = { recommended_at: string; operation_id?: string };

function fail(): never { throw new Error("invalid_console_response"); }
const id = (v: unknown): v is string => typeof v === "string" && /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(v) && v !== "00000000-0000-0000-0000-000000000000";
const int = (v: unknown, min = 0): v is number => typeof v === "number" && Number.isSafeInteger(v) && v >= min;
const text = (v: unknown, max: number): v is string => typeof v === "string" && Array.from(v).length <= max && !/[\p{Cc}]/u.test(v);
const oneOf = (v: unknown, values: readonly string[]): boolean => typeof v === "string" && values.includes(v);
function date(v: unknown): v is string {
  if (typeof v !== "string") return false;
  const match = /^(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(?:\.\d{1,9})?(?:Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$/.exec(v);
  return !!match && Number.isFinite(Date.parse(v)) && new Date(`${match[1]}Z`).toISOString().slice(0, 19) === match[1];
}
const nullableDate = (v: unknown) => v === null || date(v);
function object(v: unknown): Record<string, unknown> {
  if (!v || typeof v !== "object" || Array.isArray(v)) fail();
  return v as Record<string, unknown>;
}
function exact(v: unknown, fields: string[], optional: string[] = []): Record<string, unknown> {
  const r = object(v);
  if (fields.some((k) => !Object.hasOwn(r, k)) || Object.keys(r).some((k) => !fields.includes(k) && !optional.includes(k))) fail();
  return r;
}
const modes = matchModes;
const lifecycle = ["draft", "live", "ended", "archived"];
function window(v: unknown, sid: string): ConsoleClaimWindow {
  const r = exact(v, ["session_id", "state", "match_mode", "generation", "version", "opened_at", "closed_at"]);
  if (r.session_id !== sid || !oneOf(r.state, ["OPEN", "CLOSED"]) || !oneOf(r.match_mode, modes) ||
    !int(r.generation) || !int(r.version) || !nullableDate(r.opened_at) || !nullableDate(r.closed_at) ||
    (r.state === "OPEN" && r.opened_at === null)) fail();
  return r as ConsoleClaimWindow;
}

/** Parse a single closed A1 offer without deriving inventory or sales. */
export function parseConsoleOffer(v: unknown): ConsoleOffer {
  const r = exact(v, ["offer_id", "keyword", "sku_id", "product_name", "variant_label", "active", "version", "live_price_minor", "sku_price_minor", "stock", "claimed", "ordered_qty", "paid_qty", "paid_amount_minor", "sold_out", "low_stock"]);
  const stock = exact(r.stock, ["tracked", "sellable", "reserved", "warehouse_id", "balance_version"]);
  const claimed = exact(r.claimed, ["buyers", "quantity"]);
  if (!id(r.offer_id) || !id(r.sku_id) || typeof r.keyword !== "string" || !/^[A-Z0-9]{1,16}$/.test(r.keyword) ||
    !text(r.product_name, 400) || !text(r.variant_label, 400) || typeof r.active !== "boolean" || !int(r.version, 1) ||
    (r.live_price_minor !== null && !int(r.live_price_minor, 1)) || !int(r.sku_price_minor) ||
    typeof stock.tracked !== "boolean" || !int(stock.sellable, Number.MIN_SAFE_INTEGER) || !int(stock.reserved) ||
    (stock.warehouse_id !== null && !id(stock.warehouse_id)) || !int(stock.balance_version) ||
    !int(claimed.buyers) || !int(claimed.quantity) || !int(r.ordered_qty) || !int(r.paid_qty) || !int(r.paid_amount_minor) ||
    typeof r.sold_out !== "boolean" || typeof r.low_stock !== "boolean") fail();
  return r as ConsoleOffer;
}
function capabilities(v: unknown): ConsoleCapabilities {
  const r = exact(v, [], ["facebook", "instagram"]);
  for (const provider of Object.values(r)) {
    const map = exact(provider, [], ["read_comment", "private_reply", "dm_session", "reply_public"]);
    for (const value of Object.values(map)) {
      const c = exact(value, ["state", "reason", "evidence", "checked_at"]);
      if (!oneOf(c.state, ["ok", "missing_permission", "missing_task", "not_subscribed", "reauth_required", "review_required", "unsupported", "unknown"]) ||
        typeof c.reason !== "string" || !/^[a-z0-9_]{0,64}$/.test(c.reason) || !oneOf(c.evidence, ["DESIGN", "MOCK", "LIVE_READ", "LIVE_SEND"]) || !nullableDate(c.checked_at)) fail();
    }
  }
  return r as ConsoleCapabilities;
}
function stream(v: unknown): ConsoleStream {
  const r = exact(v, ["state", "poll_interval_ms", "last_ok_at", "lag_ms", "source_platform", "video_embeddable"], ["reason"]);
  if (!oneOf(r.state, ["live", "throttled", "reauth_required", "unavailable", "not_started"]) || !int(r.poll_interval_ms) ||
    !nullableDate(r.last_ok_at) || (r.lag_ms !== null && !int(r.lag_ms)) || !oneOf(r.source_platform, ["facebook", "instagram"]) ||
    typeof r.video_embeddable !== "boolean" || (Object.hasOwn(r, "reason") && (typeof r.reason !== "string" || !/^[a-z0-9_]{0,64}$/.test(r.reason)))) fail();
  return r as ConsoleStream;
}
/** Parse the A1 DTO and bind it to the requested session. */
export function parseConsole(v: unknown, sid: string): ConsoleSnapshot {
  const r = exact(v, ["session", "window", "stats", "offers", "capabilities", "stream", "recommended"]);
  const session = exact(r.session, ["id", "title", "lifecycle", "version", "started_at", "ended_at"]);
  const w = exact(r.window, ["state", "generation", "opened_at", "match_mode"]);
  const stats = exact(r.stats, ["comments", "keyword_comments", "buyers", "orders", "paid", "currency", "as_of"]);
  const comments = exact(stats.comments, ["total", "source"]);
  if (!id(sid) || session.id !== sid || !text(session.title, 200) || !oneOf(session.lifecycle, lifecycle) || !int(session.version, 1) ||
    !nullableDate(session.started_at) || !nullableDate(session.ended_at) || !oneOf(w.state, ["OPEN", "CLOSED"]) ||
    !int(w.generation) || !nullableDate(w.opened_at) || !oneOf(w.match_mode, modes) || (w.state === "OPEN" && w.opened_at === null) ||
    !oneOf(comments.source, ["graph_summary", "stream_seen", "unavailable"]) ||
    (comments.total !== null && !int(comments.total)) || (comments.source !== "unavailable" && comments.total === null) ||
    !int(stats.keyword_comments) || !int(stats.buyers) || typeof stats.currency !== "string" || !/^[A-Z]{3}$/.test(stats.currency) || !date(stats.as_of)) fail();
  for (const totals of [stats.orders, stats.paid]) {
    const t = exact(totals, ["count", "amount_minor"]);
    if (!int(t.count) || !int(t.amount_minor)) fail();
  }
  if (!Array.isArray(r.offers) || r.offers.length > 200) fail();
  const offers = r.offers.map(parseConsoleOffer);
  if (new Set(offers.map((o) => o.offer_id)).size !== offers.length) fail();
  if (r.recommended !== null) {
    const rec = exact(r.recommended, ["offer_id", "at"]);
    if (!id(rec.offer_id) || !date(rec.at)) fail();
  }
  return { ...r, offers, capabilities: capabilities(r.capabilities), stream: stream(r.stream) } as ConsoleSnapshot;
}
/** Parse A7's exact lifecycle receipt and its full window. */
export function parseLifecycleResult(v: unknown, sid: string): LifecycleResult {
  const r = exact(v, ["lifecycle", "version", "window"]);
  if (!id(sid) || !oneOf(r.lifecycle, lifecycle) || !int(r.version, 1)) fail();
  return { ...r, window: window(r.window, sid) } as LifecycleResult;
}
/** Parse A5 results preserving request order and separate payment environments. */
export function parseSessionResults(v: unknown, ids: readonly string[]): SessionResults {
  const r = exact(v, ["as_of", "items"]);
  if (ids.length < 1 || ids.length > 50 || ids.some((s) => !id(s)) || new Set(ids).size !== ids.length ||
    !date(r.as_of) || !Array.isArray(r.items) || r.items.length !== ids.length) fail();
  for (const [i, value] of r.items.entries()) {
    const row = exact(value, ["session_id", "orders", "paid_orders", "multi_session_orders", "money"]);
    if (row.session_id !== ids[i] || !int(row.orders) || !int(row.paid_orders) || !int(row.multi_session_orders) || !Array.isArray(row.money)) fail();
    const currencies = new Set<string>();
    for (const value of row.money) {
      const m = exact(value, ["currency", "order_minor", "paid_minor", "sandbox_paid_minor"]);
      // I05: preserve amounts as returned; never add LIVE and SANDBOX, or different currencies.
      if (typeof m.currency !== "string" || !/^[A-Z]{3}$/.test(m.currency) || currencies.has(m.currency) ||
        !int(m.order_minor) || !int(m.paid_minor) || !int(m.sandbox_paid_minor)) fail();
      currencies.add(m.currency);
    }
  }
  return r as SessionResults;
}
/** Parse A5 copy's actual Go envelope, requiring a newly seeded closed window. */
export function parseCopyResult(v: unknown): CopyResult {
  const r = exact(v, ["session", "window", "created", "conflicts", "source_version"]);
  const session = parseDraft(r.session);
  const w = window(r.window, session.session_id);
  if (!int(r.source_version, 1) || w.state !== "CLOSED" || w.generation !== 0 || w.opened_at !== null) fail();
  return { session, window: w, ...parseImportResult({ created: r.created, conflicts: r.conflicts }, session.session_id), source_version: r.source_version };
}
/** Parse A6's local timeline receipt without asserting an external send state. */
export function parseRecommendResult(v: unknown): RecommendResult {
  const r = exact(v, ["recommended_at"], ["operation_id"]);
  if (!date(r.recommended_at) || (Object.hasOwn(r, "operation_id") && !id(r.operation_id))) fail();
  return r as RecommendResult;
}
