// Storefront publication model: the strict parser of the Go GET storefront answer (contracts/published-storefront-resolver-v1.md
// "Writer (R3)", internal/storefrontadmin.State) shared by lib/storefront-client.ts and its unit test
// (tests/admin/storefront-model.test.ts). Pure: no fetch, no React; any deviation from the closed shape throws.

export type StorefrontDomain = { origin: string; valid_until: string; serving: boolean };
// version 0 = never published.
export type StorefrontState = { published: boolean; version: number; domains: StorefrontDomain[] };

const originShape = /^https:\/\/(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?$/;
const stamp = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/;
const tokenShape = /^[A-Za-z0-9_-]{43}$/;
const uuidShape = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
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

// R5 store-domains (Decision 3): the merchant self-service domain read + write shapes (Go
// internal/storefrontdomains). Same strict-parser rule as above: any drift throws, never renders.
export type MerchantDomainState =
  | "REQUESTED"
  | "OWNERSHIP_PENDING"
  | "TLS_PENDING"
  | "ACTIVE"
  | "SUSPENDED"
  | "DETACHED";

export type StorefrontDomainRow = {
  origin: string;
  state: MerchantDomainState;
  version: number;
  token: string | null;
  verify_deadline: string | null;
  serving: boolean;
};

export type StorefrontDomains = { domains: StorefrontDomainRow[] };

export type DNSInstructions = {
  txt_name: string;
  txt_value: string;
  cname_target: string;
  apex: boolean;
};

export type DomainRequestResult = {
  domain_id: string;
  version: number;
  state: "REQUESTED";
  origin: string;
  dns: DNSInstructions;
};

const domainStates = new Set<MerchantDomainState>([
  "REQUESTED",
  "OWNERSHIP_PENDING",
  "TLS_PENDING",
  "ACTIVE",
  "SUSPENDED",
  "DETACHED",
]);

export function parseStorefrontDomains(value: unknown): StorefrontDomains {
  const item = record(value);
  if (!item || !sameKeys(item, ["domains"]) || !Array.isArray(item.domains) || item.domains.length > 100)
    throw new Error("storefront_shape");
  const domains = item.domains.map((entry): StorefrontDomainRow => {
    const d = record(entry);
    if (
      !d || !sameKeys(d, ["origin", "state", "version", "token", "verify_deadline", "serving"]) ||
      typeof d.origin !== "string" || d.origin.length > 261 || !originShape.test(d.origin) ||
      typeof d.state !== "string" || !domainStates.has(d.state as MerchantDomainState) ||
      typeof d.version !== "number" || !Number.isSafeInteger(d.version) || d.version < 1 ||
      (d.token !== null && (typeof d.token !== "string" || !tokenShape.test(d.token))) ||
      (d.verify_deadline !== null && (typeof d.verify_deadline !== "string" || !stamp.test(d.verify_deadline))) ||
      typeof d.serving !== "boolean"
    )
      throw new Error("storefront_shape");
    return {
      origin: d.origin,
      state: d.state as MerchantDomainState,
      version: d.version,
      token: d.token as string | null,
      verify_deadline: d.verify_deadline as string | null,
      serving: d.serving,
    };
  });
  return { domains };
}

// The DNS instructions are shown once, at request time; the write response is the only place they appear.
export function parseDomainRequest(value: unknown): DomainRequestResult {
  const item = record(value);
  const dns = record(item?.dns);
  if (
    !item || !sameKeys(item, ["domain_id", "version", "state", "origin", "dns"]) ||
    !dns || !sameKeys(dns, ["txt_name", "txt_value", "cname_target", "apex"]) ||
    typeof item.domain_id !== "string" || !uuidShape.test(item.domain_id) ||
    typeof item.version !== "number" || !Number.isSafeInteger(item.version) || item.version < 1 ||
    item.state !== "REQUESTED" ||
    typeof item.origin !== "string" || !originShape.test(item.origin) ||
    typeof dns.txt_name !== "string" || !dns.txt_name.startsWith("_lc-verify.") ||
    typeof dns.txt_value !== "string" || !tokenShape.test(dns.txt_value) ||
    typeof dns.cname_target !== "string" || typeof dns.apex !== "boolean"
  )
    throw new Error("storefront_shape");
  return {
    domain_id: item.domain_id,
    version: item.version,
    state: "REQUESTED",
    origin: item.origin,
    dns: { txt_name: dns.txt_name, txt_value: dns.txt_value, cname_target: dns.cname_target, apex: dns.apex },
  };
}
