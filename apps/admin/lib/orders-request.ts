const states = new Set([
  "all",
  "DRAFT",
  "AWAITING_PAYMENT",
  "CONFIRMED",
  "CANCELLED",
]);

// Inspect the raw URL before Next.js drops empty search strings like a bare '?'.
export function validOrdersQuery(rawURL: string, detail: boolean) {
  const at = rawURL.indexOf("?");
  if (at < 0) return true;
  if (detail) return false;
  const seen = new Set<string>();
  for (const segment of rawURL.slice(at + 1).split("&")) {
    const match = /^(limit|cursor|state)=([A-Za-z0-9_-]+)$/.exec(segment);
    if (!match || seen.has(match[1])) return false;
    const [, key, value] = match;
    seen.add(key);
    if (
      (key === "limit" && !/^(?:[1-9]|[1-9][0-9]|100)$/.test(value)) ||
      (key === "cursor" && value.length > 1024) ||
      (key === "state" && !states.has(value))
    )
      return false;
  }
  return true;
}
