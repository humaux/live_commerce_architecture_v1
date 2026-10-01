// Storefront publication model: the strict parser of the Go GET storefront answer (contracts/published-storefront-resolver-v1.md
// "Writer (R3)", internal/storefrontadmin.State) shared by lib/storefront-client.ts and its unit test
// (tests/admin/storefront-model.test.ts). Pure: no fetch, no React; any deviation from the closed shape throws.

export type StorefrontDomain = { origin: string; valid_until: string; serving: boolean };
// version 0 = never published.
export type StorefrontState = { published: boolean; version: number; domains: StorefrontDomain[] };

const originShape = /^https:\/\/(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?$/;
const stamp = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/;
const record = (value: unknown): Record<string, unknown> | null =>
  value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : null;
const sameKeys = (item: Record<string, unknown>, keys: string[]) =>
  Object.keys(item).length === keys.length && keys.every((key) => key in item);

// Strict: unknown or missing keys, a bad origin or a non-integer version read as "unavailable", never as state.
export function parseStorefront(value: unknown): StorefrontState {
  const item = record(value);
  if (
    !item || !sameKeys(item, ["published", "version", "domains"]) || typeof item.published !== "boolean" ||
    typeof item.version !== "number" || !Number.isSafeInteger(item.version) || item.version < 0 ||
    (item.version === 0 && item.published) || !Array.isArray(item.domains) || item.domains.length > 100
  )
    throw new Error("storefront_shape");
  const domains = item.domains.map((entry): StorefrontDomain => {
    const d = record(entry);
    if (
      !d || !sameKeys(d, ["origin", "valid_until", "serving"]) || typeof d.origin !== "string" ||
      d.origin.length > 261 || !originShape.test(d.origin) || typeof d.valid_until !== "string" ||
      !stamp.test(d.valid_until) || typeof d.serving !== "boolean"
    )
      throw new Error("storefront_shape");
    return { origin: d.origin, valid_until: d.valid_until, serving: d.serving };
  });
  return { published: item.published, version: item.version, domains };
}
