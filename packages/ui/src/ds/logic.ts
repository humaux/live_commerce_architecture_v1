// Purpose: Pure, DOM-free rules of the admin design system: selection scope, max-one-primary row actions, disabled-with-reason, pagination window, typed confirm, status families, file checks, nav open state.
// Depends on: nothing (no React, no DOM, no copy); runs under node --experimental-strip-types so it is unit-testable without a browser.
// Used by: packages/ui/src/ds/*.tsx components; tests/ui/ds-logic.test.ts.

/* ------------------------------ selection ------------------------------ */

/** "page" = the checked rows on this page only; "all" = every result of the current filter (needs a confirm). */
export type SelectionScope = "page" | "all";
/** `filterKey` is set only in "all" scope: the filter the "all results" offer was made for. */
export type SelectionState = { readonly ids: ReadonlySet<string>; readonly scope: SelectionScope; readonly filterKey?: string };
export const emptySelection: SelectionState = { ids: new Set<string>(), scope: "page" };

export type PageSelection = "none" | "some" | "all";

/** Tri-state of the header checkbox for the rows currently on screen. */
export function pageSelection(state: SelectionState, pageIds: readonly string[]): PageSelection {
  if (pageIds.length === 0) return "none";
  const n = pageIds.filter((id) => state.ids.has(id)).length;
  return n === 0 ? "none" : n === pageIds.length ? "all" : "some";
}

/** Header checkbox: select the whole page, or clear it. Always returns to page scope. */
export function togglePage(state: SelectionState, pageIds: readonly string[]): SelectionState {
  if (state.scope === "all") return emptySelection;
  const ids = new Set(state.ids);
  if (pageSelection(state, pageIds) === "all") for (const id of pageIds) ids.delete(id);
  else for (const id of pageIds) ids.add(id);
  return { ids, scope: "page" };
}

/** Row checkbox. Leaving "all" scope keeps the rows of this page minus the one unchecked. */
export function toggleRow(state: SelectionState, id: string, pageIds: readonly string[]): SelectionState {
  const base = state.scope === "all" ? new Set(pageIds) : new Set(state.ids);
  if (base.has(id)) base.delete(id);
  else base.add(id);
  return { ids: base, scope: "page" };
}

/** The "select all N results" offer appears only when the whole page is checked and more results exist off-page. */
export function offerAllResults(state: SelectionState, pageIds: readonly string[], total: number): boolean {
  return state.scope === "page" && pageSelection(state, pageIds) === "all" && total > pageIds.length;
}

export function selectAllResults(state: SelectionState, pageIds: readonly string[], total: number, filterKey: string): SelectionState {
  return offerAllResults(state, pageIds, total) ? { ids: state.ids, scope: "all", filterKey } : state;
}

export function clearSelection(): SelectionState {
  return emptySelection;
}

/** "all" = every result; "page" = the checked rows, counted only among `pageIds` when given (ids kept from other pages are not on screen). */
export function selectionCount(state: SelectionState, total: number, pageIds?: readonly string[]): number {
  if (state.scope === "all") return total;
  return pageIds ? pageIds.filter((id) => state.ids.has(id)).length : state.ids.size;
}

/** A bulk command over "all" results is sent as the filter, never as a list of ids; it must be confirmed first. */
export type BulkTarget = { scope: "ids"; ids: string[] } | { scope: "filter"; filter: string } | { scope: "stale_selection" };
/**
 * "all" scope is bound to the filter it was selected under: if the current filter differs the selection is stale and no
 * target is produced (the caller must clear it). In page scope, `pageIds` limits the ids to the rows on screen.
 */
export function bulkTarget(state: SelectionState, filterKey: string, pageIds?: readonly string[]): BulkTarget {
  if (state.scope === "all") return state.filterKey === filterKey ? { scope: "filter", filter: filterKey } : { scope: "stale_selection" };
  const ids = pageIds ? pageIds.filter((id) => state.ids.has(id)) : [...state.ids];
  return { scope: "ids", ids: ids.sort() };
}
export const bulkNeedsConfirm = (state: SelectionState): boolean => state.scope === "all";

/* --------------------------- row actions / buttons --------------------------- */

export type ActionVariant = "primary" | "secondary" | "danger";
export type ActionLike = { id: string; variant?: ActionVariant };

export const countPrimary = (actions: readonly ActionLike[]): number => actions.filter((a) => a.variant === "primary").length;

/**
 * A row carries at most ONE primary action (SHOPLINE list rule). strict=true (development and tests) throws on a
 * second primary so the design mistake is caught; strict=false (production) demotes the extras to secondary so a
 * live page never crashes over styling.
 */
export function resolveRowActions<T extends ActionLike>(actions: readonly T[], strict: boolean): T[] {
  if (countPrimary(actions) <= 1) return [...actions];
  if (strict) throw new Error(`row actions: at most one primary action per row, got ${countPrimary(actions)}`);
  let seen = false;
  return actions.map((a) => {
    if (a.variant !== "primary") return a;
    if (!seen) {
      seen = true;
      return a;
    }
    return { ...a, variant: "secondary" } as T;
  });
}

/** A "more" menu item with only an href navigates, so it renders as a link; a disabled one stays a button that explains itself. */
export function menuItemKind(item: { href?: string; disabledReason?: string }): "link" | "button" {
  return item.href !== undefined && !item.disabledReason ? "link" : "button";
}

export type ButtonState = { blocked: boolean; busy: boolean; reason: string | undefined };
/** A disabled button always says why: there is no plain `disabled`. Loading blocks clicks without a reason. */
export function buttonState(disabledReason: string | undefined, loading: boolean): ButtonState {
  if (disabledReason !== undefined && disabledReason.trim() === "") throw new Error("button: disabledReason must not be empty");
  return { blocked: loading || disabledReason !== undefined, busy: loading, reason: disabledReason };
}

/* -------------------------------- pagination -------------------------------- */

/** Page numbers to render, with "gap" where pages are skipped: 1 … 4 5 [6] 7 8 … 20. */
export function pageWindow(current: number, total: number, siblings = 1): Array<number | "gap"> {
  if (total < 1) return [];
  const c = Math.min(Math.max(1, Math.floor(current)), total);
  const keep = new Set<number>([1, total]);
  for (let p = c - siblings; p <= c + siblings; p++) if (p >= 1 && p <= total) keep.add(p);
  const sorted = [...keep].sort((a, b) => a - b);
  const out: Array<number | "gap"> = [];
  sorted.forEach((p, i) => {
    const prev = sorted[i - 1];
    if (i > 0 && prev !== undefined && p - prev === 2) out.push(prev + 1);
    else if (i > 0 && prev !== undefined && p - prev > 2) out.push("gap");
    out.push(p);
  });
  return out;
}

/** Query string for a page: keeps every other filter, drops `page` for page 1 so the first page has one canonical URL. */
export function withPage(search: string, page: number, key = "page"): string {
  const q = new URLSearchParams(search.startsWith("?") ? search.slice(1) : search);
  if (page <= 1) q.delete(key);
  else q.set(key, String(Math.floor(page)));
  const s = q.toString();
  return s ? `?${s}` : "?";
}

/** Reads `page` from the URL; anything that is not a positive integer is page 1. */
export function pageFromSearch(search: string, key = "page"): number {
  const raw = new URLSearchParams(search.startsWith("?") ? search.slice(1) : search).get(key);
  return raw !== null && /^[1-9][0-9]{0,6}$/.test(raw) ? Number(raw) : 1;
}

/* ---------------------------- typed confirm / files ---------------------------- */

/** The typed-confirm phrase matches ignoring surrounding space, width variants and Latin case. */
export function typedConfirmMatches(input: string, phrase: string): boolean {
  const norm = (s: string) => s.normalize("NFKC").trim().toLowerCase();
  return phrase.trim() !== "" && norm(input) === norm(phrase);
}

export type FileRule = { extensions: readonly string[]; maxBytes: number };
export type FileProblem = "type" | "size";
/** Checks a picked file against the rule that was shown BEFORE the upload; extensions without the dot, lower case. */
export function fileProblem(file: { name: string; size: number }, rule: FileRule): FileProblem | undefined {
  const dot = file.name.lastIndexOf(".");
  const ext = dot < 0 ? "" : file.name.slice(dot + 1).toLowerCase();
  if (!rule.extensions.includes(ext)) return "type";
  if (file.size > rule.maxBytes) return "size";
  return undefined;
}

/* ------------------------------- status families ------------------------------- */

export type Tone = "neutral" | "success" | "warning" | "danger" | "info";
/**
 * One vocabulary per domain (NEEDS section 3 "状态徽章"): payment, fulfilment, refund and connection never share
 * statuses, so a payment "pending" and a refund "pending" cannot be rendered by the same pill by accident.
 * `unknown` is the UNKNOWN-outcome state (待核对) and is a warning in every family.
 */
export const STATUS_FAMILIES = {
  payment: { unpaid: "neutral", pending: "info", paid: "success", failed: "danger", refunded: "neutral", unknown: "warning" },
  fulfilment: { unfulfilled: "neutral", packing: "info", shipped: "info", delivered: "success", returned: "warning", unknown: "warning" },
  refund: { requested: "info", approved: "info", processing: "info", refunded: "success", rejected: "danger", unknown: "warning" },
  connection: { connected: "success", expiring: "warning", error: "danger", disconnected: "neutral", unknown: "warning" },
} as const satisfies Record<string, Record<string, Tone>>;
export type StatusDomain = keyof typeof STATUS_FAMILIES;
export type StatusOf<D extends StatusDomain> = keyof (typeof STATUS_FAMILIES)[D] & string;

export function statusTone(domain: StatusDomain, status: string): Tone {
  const family: Record<string, Tone> = STATUS_FAMILIES[domain];
  const tone = family[status];
  if (!tone) throw new Error(`status "${status}" does not belong to the ${domain} family`);
  return tone;
}

/* ----------------------------------- nav ----------------------------------- */

export type NavNode = { id: string; items?: readonly { id: string }[] };
/** Group ids that start expanded: the group holding the current page, or the group that is itself current. */
export function initialOpenGroups(nav: readonly NavNode[], currentId: string | undefined): Set<string> {
  const open = new Set<string>();
  if (currentId === undefined) return open;
  for (const g of nav) if (g.id === currentId || g.items?.some((i) => i.id === currentId)) open.add(g.id);
  return open;
}
