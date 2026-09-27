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
