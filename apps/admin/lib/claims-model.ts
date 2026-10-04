// Owns the Studio › Claims browser data model: exact TypeScript shapes of the frozen M1–M7
// responses (contracts/live-keyword-claims-v1.md §4.2, §7.1) and closed parsers that reject
// any extra, missing or ill-typed field before the UI renders it.
// Non-goals: no fetching (claims-client.ts), no copy (claims-copy.ts), no business rule
// (Go decides windows, offers, claims and links; the UI only displays their results).
// Depends on: claims-request.ts (validClaimLink, shared with the BFF) and orders-model.ts (wholeOnly: the TWD whole-dollar rule).

import { validClaimLink, type ClaimLink } from "./claims-request.ts";
import { wholeOnly } from "./orders-model.ts";

export type MatchMode = "EXACT" | "KEYWORD_QTY_ONLY";
export const persistedReasons = ["NO_MATCH", "UNKNOWN_KEYWORD", "OFFER_INACTIVE", "INVALID_QUANTITY",
  "QUANTITY_REQUIRED", "QUANTITY_OVER_MAX", "BUNDLE_LIMIT"] as const;
export type PersistedReason = (typeof persistedReasons)[number];
export type Reason = "" | PersistedReason | "WINDOW_CLOSED";

export type ClaimWindow = {
  session_id: string; state: "OPEN" | "CLOSED"; match_mode: MatchMode; generation: number;
  version: number; opened_at: string | null; closed_at: string | null;
};
export type Offer = {
  offer_id: string; session_id: string; keyword: string; sku_id: string; sku_code: string;
  product_name: string; max_quantity_per_claim: number; active: boolean; version: number;
  activated_at: string; updated_at: string;
  // Live tools (R4): the SKU's catalog price (for the "live price is higher" warning) and the live-only price (null = none).
  sku_price_minor: number; currency: string; live_price_minor: number | null;
};
/** One keyword-library row (store-level default keyword per SKU). version 0 + keyword "" = the row was removed. */
export type LibraryEntry = {
  sku_id: string; sku_code: string; product_name: string; keyword: string; version: number;
  sku_price_minor: number; currency: string; updated_at: string;
};
export const importConflictReasons = ["keyword_taken", "already_present", "sku_taken", "sku_unavailable", "session_full"] as const;
export type ImportConflictReason = (typeof importConflictReasons)[number];
/** Result of "import library keywords" / "copy offers from a session": what was created and what was refused. */
export type ImportResult = { created: Offer[]; conflicts: { keyword: string; sku_id: string; reason: ImportConflictReason }[] };
export type Board = {
  window: ClaimWindow; offers: Offer[];
  stats: { generation: number; accepted: number; rejected: Record<PersistedReason, number> };
};
export type ManualResult = {
  outcome: "ACCEPTED" | "REJECTED"; reason: Reason; offer_id: string; keyword: string; quantity: number;
  previous_quantity: number; bundle_id: string; bundle_version: number; line_version: number;
};
export type BundleLine = { offer_id: string; keyword: string; sku_id: string; quantity: number; version: number; applied: boolean };
export type Bundle = {
  bundle_id: string; ref: string; platform: BundlePlatform; label: string; bound: boolean; version: number;
  link: { state: "NONE" | "ACTIVE" | "EXPIRED"; generation: number; expires_at: string | null };
  lines: BundleLine[]; created_at: string; updated_at: string;
};
export type BundlePage = { items: Bundle[]; next_cursor: string };
// Catalog projections the offer form and link builder read (existing admin routes).
export type CatalogProduct = { id: string; name: string; status: string };
export type CatalogSKU = { id: string; product_id: string; code: string; status: string; currency: string };
export type PurchaseEntry = { product_id: string; locale: string; state: string; url: string };

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const claimsCursor = /^[A-Za-z0-9_-]{1,1024}$/;

function invalid(): never { throw new Error("invalid_claims_response"); }
function exact(value: unknown, fields: string[]): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) invalid();
  const row = value as Record<string, unknown>;
  if (Object.keys(row).sort().join(",") !== [...fields].sort().join(",")) invalid();
  return row;
}
const id = (value: unknown) => typeof value === "string" && uuid.test(value);
const count = (value: unknown, min = 0) => Number.isSafeInteger(value) && (value as number) >= min;
const date = (value: unknown) => typeof value === "string" && value.length <= 40 && Number.isFinite(Date.parse(value));
const keyword = (value: unknown) => typeof value === "string" && /^[A-Z0-9]{1,16}$/.test(value);
const text = (value: unknown, max: number) => typeof value === "string" && Array.from(value).length <= max;

/** Closed M2 / M1-window parser for one scene. */
export function parseWindow(value: unknown, sessionID: string): ClaimWindow {
  const row = exact(value, ["session_id", "state", "match_mode", "generation", "version", "opened_at", "closed_at"]);
  if (row.session_id !== sessionID || (row.state !== "OPEN" && row.state !== "CLOSED") ||
    (row.match_mode !== "EXACT" && row.match_mode !== "KEYWORD_QTY_ONLY") || !count(row.generation) ||
    !count(row.version) || (row.opened_at !== null && !date(row.opened_at)) ||
    (row.closed_at !== null && !date(row.closed_at)) || (row.state === "OPEN" && row.opened_at === null)) invalid();
  return row as ClaimWindow;
}

/** Closed M3/M4 / M1-offer parser for one scene. */
export function parseOffer(value: unknown, sessionID: string): Offer {
  const row = exact(value, ["offer_id", "session_id", "keyword", "sku_id", "sku_code", "product_name",
    "max_quantity_per_claim", "active", "version", "activated_at", "updated_at", "sku_price_minor", "currency", "live_price_minor"]);
  if (!id(row.offer_id) || row.session_id !== sessionID || !keyword(row.keyword) || !id(row.sku_id) ||
    !text(row.sku_code, 128) || !text(row.product_name, 400) || !count(row.max_quantity_per_claim, 1) ||
    (row.max_quantity_per_claim as number) > 999 || typeof row.active !== "boolean" || !count(row.version, 1) ||
    !date(row.activated_at) || !date(row.updated_at) || !count(row.sku_price_minor) ||
    typeof row.currency !== "string" || !/^[A-Z]{3}$/.test(row.currency) ||
    (row.live_price_minor !== null && (!count(row.live_price_minor, 1) || (row.live_price_minor as number) > 1e12))) invalid();
  return row as Offer;
}

/** Closed GET .../claims/library parser. */
export function parseLibrary(value: unknown): LibraryEntry[] {
  const row = exact(value, ["entries"]);
  if (!Array.isArray(row.entries) || row.entries.length > 1000) invalid();
  return row.entries.map(libraryEntry);
}
const libraryFields = ["sku_id", "sku_code", "product_name", "keyword", "version", "sku_price_minor", "currency", "updated_at"];
function libraryEntry(value: unknown): LibraryEntry {
  const row = exact(value, libraryFields);
  if (!id(row.sku_id) || !keyword(row.keyword) || !text(row.sku_code, 128) || !text(row.product_name, 400) ||
    !count(row.version, 1) || !count(row.sku_price_minor) || typeof row.currency !== "string" || !/^[A-Z]{3}$/.test(row.currency) ||
    !date(row.updated_at)) invalid();
  return row as LibraryEntry;
}
/** Closed PUT .../claims/library/{sku} parser: a saved row, or the removal receipt (empty keyword, version 0). */
export function parseLibrarySaved(value: unknown): LibraryEntry {
  const row = exact(value, libraryFields);
  if (row.keyword === "" && row.version === 0 && id(row.sku_id)) return row as LibraryEntry;
  return libraryEntry(value);
}

/** Closed POST .../claims/offer-import parser (conflicts are data, never an error). */
export function parseImportResult(value: unknown, sessionID: string): ImportResult {
  const row = exact(value, ["created", "conflicts"]);
  if (!Array.isArray(row.created) || row.created.length > 200 || !Array.isArray(row.conflicts) || row.conflicts.length > 1000) invalid();
  const created = row.created.map((item) => parseOffer(item, sessionID));
  const conflicts = row.conflicts.map((item) => {
    const conflict = exact(item, ["keyword", "sku_id", "reason"]);
    if (!keyword(conflict.keyword) || !id(conflict.sku_id) || !(importConflictReasons as readonly unknown[]).includes(conflict.reason)) invalid();
    return conflict as ImportResult["conflicts"][number];
  });
  return { created, conflicts };
}

/** Closed M1 parser; stats must describe the window's current round. */
export function parseBoard(value: unknown, sessionID: string): Board {
  const row = exact(value, ["window", "offers", "stats"]);
  const window = parseWindow(row.window, sessionID);
  if (!Array.isArray(row.offers) || row.offers.length > 200) invalid();
  const offers = row.offers.map((item) => parseOffer(item, sessionID));
  const stats = exact(row.stats, ["generation", "accepted", "rejected"]);
  const rejected = exact(stats.rejected, [...persistedReasons]);
  if (stats.generation !== window.generation || !count(stats.accepted) ||
    persistedReasons.some((reason) => !count(rejected[reason]))) invalid();
  return { window, offers, stats: stats as Board["stats"] };
}

/** Closed M5 parser (the command receipt: never a label, text or actor key). */
export function parseManualResult(value: unknown): ManualResult {
  const row = exact(value, ["outcome", "reason", "offer_id", "keyword", "quantity", "previous_quantity",
    "bundle_id", "bundle_version", "line_version"]);
  const accepted = row.outcome === "ACCEPTED";
  if ((!accepted && row.outcome !== "REJECTED") ||
    (accepted ? row.reason !== "" : ![...persistedReasons, "WINDOW_CLOSED"].includes(row.reason as string)) ||
    (row.offer_id !== "" && !id(row.offer_id)) || (row.keyword !== "" && !keyword(row.keyword)) ||
    !count(row.quantity) || !count(row.previous_quantity) || (accepted ? !id(row.bundle_id) : row.bundle_id !== "") ||
    !count(row.bundle_version) || !count(row.line_version)) invalid();
  return row as ManualResult;
}

export const bundlePlatforms = ["manual", "facebook", "instagram"] as const;
export type BundlePlatform = typeof bundlePlatforms[number];

function parseBundle(value: unknown): Bundle {
  const row = exact(value, ["bundle_id", "ref", "platform", "label", "bound", "version", "link", "lines", "created_at", "updated_at"]);
  const link = exact(row.link, ["state", "generation", "expires_at"]);
  // meta-claims-intake-v1 §4/§8: platform widens to facebook|instagram; CHECK (platform='manual')=(label IS NOT NULL),
  // so a manual bundle has a label and a Meta bundle has none (Go renders NULL as "").
  if (!id(row.bundle_id) || typeof row.ref !== "string" || !/^[0-9A-F]{8}$/.test(row.ref) ||
    !bundlePlatforms.includes(row.platform as BundlePlatform) || !text(row.label, 60) ||
    (row.platform === "manual") !== (row.label !== "") || typeof row.bound !== "boolean" || !count(row.version) ||
    !["NONE", "ACTIVE", "EXPIRED"].includes(link.state as string) || !count(link.generation) ||
    (link.expires_at !== null && !date(link.expires_at)) || (link.state === "NONE") !== (link.expires_at === null) ||
    !Array.isArray(row.lines) || row.lines.length > 50 || !date(row.created_at) || !date(row.updated_at)) invalid();
  const lines = row.lines.map((item) => {
    const line = exact(item, ["offer_id", "keyword", "sku_id", "quantity", "version", "applied"]);
    if (!id(line.offer_id) || !keyword(line.keyword) || !id(line.sku_id) || !count(line.quantity, 1) ||
      !count(line.version, 1) || typeof line.applied !== "boolean") invalid();
    return line as BundleLine;
  });
  return { ...(row as Bundle), link: link as Bundle["link"], lines };
}

/** Closed M6 parser. */
export function parseBundlePage(value: unknown): BundlePage {
  const row = exact(value, ["items", "next_cursor"]);
  if (!Array.isArray(row.items) || row.items.length > 100 || typeof row.next_cursor !== "string" ||
    (row.next_cursor !== "" && !claimsCursor.test(row.next_cursor))) invalid();
  const items = row.items.map(parseBundle);
  if (new Set(items.map((item) => item.bundle_id)).size !== items.length) invalid();
  return { items, next_cursor: row.next_cursor };
}

/** Closed M7 parser (shared rule with the BFF, claims-request.ts). */
export function parseClaimLink(value: unknown): ClaimLink {
  if (!validClaimLink(value)) invalid();
  return value;
}

// Catalog pages carry more fields than the claims UI needs; keep only the used ones.
export function parseCatalogPage<T>(value: unknown, item: (row: Record<string, unknown>) => T | null): { items: T[]; next_cursor: string } {
  if (!value || typeof value !== "object" || Array.isArray(value)) invalid();
  const row = value as Record<string, unknown>;
  if (!Array.isArray(row.items) || row.items.length > 100 || typeof row.next_cursor !== "string") invalid();
  const items: T[] = [];
  for (const entry of row.items) {
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) invalid();
    const parsed = item(entry as Record<string, unknown>);
    if (parsed) items.push(parsed);
  }
  return { items, next_cursor: row.next_cursor };
}
export const catalogProduct = (row: Record<string, unknown>): CatalogProduct | null =>
  id(row.id) && typeof row.name === "string" && typeof row.status === "string"
    ? { id: row.id as string, name: row.name, status: row.status } : null;
export const catalogSKU = (row: Record<string, unknown>): CatalogSKU | null =>
  id(row.id) && id(row.product_id) && typeof row.code === "string" && typeof row.status === "string" && typeof row.currency === "string"
    ? { id: row.id as string, product_id: row.product_id as string, code: row.code, status: row.status, currency: row.currency } : null;
/** The existing purchase-entry projection, used only to learn the published origin. */
export function parsePurchaseEntry(value: unknown, productID: string, locale: string): PurchaseEntry {
  const row = exact(value, ["product_id", "locale", "state", "url"]);
  if (row.product_id !== productID || row.locale !== locale || typeof row.state !== "string" || typeof row.url !== "string" ||
    (row.state === "configured") !== (row.url !== "")) invalid();
  return row as PurchaseEntry;
}

// Live tools (R4) price helpers: money is integer minor units on the wire; the input is a decimal string in the
// currency's own decimal places. Pure; no float arithmetic (digits are split, never multiplied).
/** Decimal places of a currency (Intl's own table, the same one lib/client.ts money() divides by). */
export function currencyDigits(currency: string): number {
  return new Intl.NumberFormat("en", { style: "currency", currency }).resolvedOptions().maximumFractionDigits ?? 2;
}
/** "12.5" -> 1250 (digits 2); null when not a positive decimal within the currency's places or above 1e12 minor units. */
export function parsePriceMinor(input: string, digits: number): number | null {
  const match = /^(\d{1,13})(?:\.(\d+))?$/.exec(input.trim());
  if (!match || (match[2]?.length ?? 0) > digits) return null;
  const minor = Number(match[1] + (match[2] ?? "").padEnd(digits, "0"));
  return Number.isSafeInteger(minor) && minor >= 1 && minor <= 1e12 ? minor : null;
}
/** The live-price input of one currency: parsePriceMinor in its decimals, and TWD takes whole dollars only (6000, never 6050: Go refuses it). */
export function parseLivePrice(input: string, currency: string): number | null {
  const minor = parsePriceMinor(input, currencyDigits(currency));
  return minor !== null && (!wholeOnly(currency) || minor % 100 === 0) ? minor : null;
}
/** 1250 (digits 2) -> "12.50"; 1250 (digits 0) -> "1250". */
export function priceInputText(minor: number, digits: number): string {
  if (digits === 0) return String(minor);
  const text = String(minor).padStart(digits + 1, "0");
  return `${text.slice(0, -digits)}.${text.slice(-digits)}`;
}
