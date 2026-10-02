// Session/store-scoped pending journal. No credentials or DNS verification tokens.
// Legacy "pending" markers cannot be replayed: they predate backend deduplication.
export type DomainCommand =
  | { version: 1; key: string; action: "request"; hostname: string }
  | { version: 1; key: string; action: "suspend" | "detach"; origin: string };

export function validDomainHostname(value: string): boolean {
  return value.length <= 253 && /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(value);
}

export function parseDomainCommand(raw: string | null): DomainCommand | null {
  if (raw === null || raw.length > 600) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!value || typeof value !== "object" || Array.isArray(value)) return null;
    const c = value as Record<string, unknown>;
    if (c.version !== 1 || typeof c.key !== "string" || !/^storefront-domain-[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(c.key) || Object.keys(c).length !== 4) return null;
    if (c.action === "request" && typeof c.hostname === "string" && validDomainHostname(c.hostname)) return { version: 1, key: c.key, action: c.action, hostname: c.hostname };
    if ((c.action === "suspend" || c.action === "detach") && typeof c.origin === "string" && c.origin.startsWith("https://") && validDomainHostname(c.origin.slice(8))) return { version: 1, key: c.key, action: c.action, origin: c.origin };
  } catch { /* Corrupt/legacy journal stays locked for platform reconciliation. */ }
  return null;
}
