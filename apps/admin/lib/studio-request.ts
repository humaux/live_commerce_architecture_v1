// Inspect the raw URL before Next can normalize a bare '?' or encoded alias.
export function validStudioQuery(rawURL: string, collection: boolean) {
  const at = rawURL.indexOf("?");
  if (at < 0) return true;
  if (!collection) return false;
  const seen = new Set<string>();
  for (const segment of rawURL.slice(at + 1).split("&")) {
    const match = /^(limit|cursor)=([A-Za-z0-9_-]+)$/.exec(segment);
    if (!match || seen.has(match[1])) return false;
    seen.add(match[1]);
    if (
      (match[1] === "limit" && !/^(?:[1-9]|[1-9][0-9]|100)$/.test(match[2])) ||
      (match[1] === "cursor" && match[2].length > 1024)
    ) return false;
  }
  return true;
}

// Only the deliberate token response may contain a publisher credential. Keep
// its transport contract closed; never forward an arbitrary upstream object.
export function validStudioInputToken(value: unknown): boolean {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const item = value as Record<string, unknown>;
  const fields = ["attempt_id", "room_name", "publisher_identity", "url", "token", "expires_at"];
  if (Object.keys(item).length !== fields.length || fields.some((key) => !Object.hasOwn(item, key))) return false;
  if (typeof item.attempt_id !== "string" || !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(item.attempt_id) ||
      item.attempt_id === "00000000-0000-0000-0000-000000000000" ||
      typeof item.room_name !== "string" || item.room_name !== `lc_${item.attempt_id.replaceAll("-", "")}` ||
      typeof item.publisher_identity !== "string" || !/^lcp_[0-9a-f]{32}$/.test(item.publisher_identity) ||
      typeof item.token !== "string" || !/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(item.token) ||
      typeof item.expires_at !== "number" || !Number.isSafeInteger(item.expires_at) || item.expires_at <= 0 ||
      typeof item.url !== "string" || item.url.length > 256) return false;
  // Match the frozen local runtime grammar without accepting URL normalization,
  // public hosts, embedded credentials, query strings or caller-chosen targets.
  const match = /^wss?:\/\/(?:127\.0\.0\.1|\[::1\]):([1-9][0-9]{0,4})\/?$/.exec(item.url);
  return !!match && Number(match[1]) <= 65535;
}
