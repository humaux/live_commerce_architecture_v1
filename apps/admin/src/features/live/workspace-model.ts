// Purpose: Pure presentation decisions for the LC-U1 workspace; never infer provider or payment state.
// Depends on: Store and ConsoleSnapshot types; native string/number/URL validation.
// Used by: LiveConsole and live-workspace.test.ts.
import type { Store } from "../../../lib/model.ts";
import type { ConsoleSnapshot } from "./console-model.ts";

/** Display permission only; the inventory endpoint enforces the narrow LC-B7 bounds and CAS. */
export function liveStockAllowed(store: Pick<Store, "role" | "permissions">): boolean {
  return store.role === "owner" || store.permissions?.some((p) => p === "inventory:write" || p === "inventory:live_adjust") === true;
}
/** IG has no Graph summary count; observed FB counts retain their explicit source label. */
export function consoleCommentStat(platform: ConsoleSnapshot["stream"]["source_platform"], comments: ConsoleSnapshot["stats"]["comments"]): ConsoleSnapshot["stats"]["comments"] {
  return platform === "instagram" ? { total: null, source: "unavailable" } : comments;
}

/** Maps only the server's commerce lifecycle to the single primary action. */
export function primaryAction(lifecycle: string): "start" | "end" | "copy" | null {
  return lifecycle === "draft" ? "start" : lifecycle === "live" ? "end" : lifecycle === "ended" ? "copy" : null;
}
/** Converts a whole nonnegative sellable target to a bounded, server-CAS delta. */
export function stockDelta(target: string, sellable: number): number | null {
  if (!/^\d+$/.test(target) || !Number.isSafeInteger(sellable)) return null;
  const quantity = Number(target), delta = quantity - sellable;
  return Number.isSafeInteger(quantity) && Math.abs(delta) <= 1000 ? delta : null;
}
/** Builds an official public-post embed only from the server-verified Page/post pair, not arbitrary URLs. */
export function facebookEmbed(platform: string, objectID: string): string | null {
  const pair = /^(\d{1,30})_(\d{1,30})$/.exec(objectID);
  if (platform !== "facebook" || !pair) return null;
  const query = new URLSearchParams({ href: `https://www.facebook.com/${pair[1]}/posts/${pair[2]}`, show_text: "false", width: "500" });
  return `https://www.facebook.com/plugins/post.php?${query}`;
}
/** Applies a completed command only while its original mounted session/store scope remains current. */
export async function settleLiveCommand<T>(execute: () => Promise<T>, isCurrent: () => boolean, complete: (value: T) => void): Promise<boolean> {
  const value = await execute();
  if (!isCurrent()) return false;
  complete(value);
  return true;
}
