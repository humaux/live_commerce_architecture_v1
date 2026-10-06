// Purpose: Pure presentation decisions for the LC-U1 workspace; never infer provider or payment state.
// Depends on: Native string/number/URL validation only.
// Used by: LiveConsole and live-workspace.test.ts.
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
