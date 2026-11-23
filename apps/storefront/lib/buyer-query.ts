// Purpose: admit only the query parameters declared by the private buyer BFF route.
// Depends on: the buyer route query kinds and canonical buyer identifiers.
// Used by: buyer-server before forwarding an authenticated buyer request to Go.
// Invariants: direct checkout admits only one exact cart ID; other queries reject unknown or repeated keys.
export type BuyerQuery = "catalog" | "options" | "orders" | "purchaseCart";

export const BUYER_UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export function validQuery(rawURL: string, kind?: BuyerQuery): string | null {
  const mark = rawURL.indexOf("?");
  if (mark < 0) return "";
  if (!kind) return null;
  const raw = rawURL.slice(mark + 1).split("#", 1)[0];
  if (kind === "purchaseCart") {
    const prefix = "buy_now_id=";
    return raw.startsWith(prefix) && BUYER_UUID.test(raw.slice(prefix.length)) ? `?${raw}` : null;
  }
  if (
    !raw ||
    raw.length > 2048 ||
    raw.includes(";") ||
    raw.includes("+") ||
    raw.split("&").some((part) => !part || !part.includes("="))
  )
    return null;
  const allowed =
    kind === "catalog"
      ? ["product_id", "limit", "cursor"]
      : kind === "orders"
        ? ["limit", "cursor"]
        : ["market_id", "country", "limit", "cursor"];
  const seen = new Set<string>();
  for (const part of raw.split("&")) {
    const at = part.indexOf("=");
    let name: string, value: string;
    try {
      name = decodeURIComponent(part.slice(0, at));
      value = decodeURIComponent(part.slice(at + 1));
    } catch {
      return null;
    }
    if (!allowed.includes(name) || seen.has(name) || !value) return null;
    seen.add(name);
    if ((name === "product_id" || name === "market_id") && !BUYER_UUID.test(value))
      return null;
    if (name === "country" && !/^[A-Z]{2}$/.test(value)) return null;
    if (
      name === "limit" &&
      (!/^[0-9]{1,3}$/.test(value) || +value < 1 || +value > 100)
    )
      return null;
    if (
      name === "cursor" &&
      (value.length > 1024 || !/^[A-Za-z0-9_-]+$/.test(value))
    )
      return null;
  }
  return `?${raw}`;
}
