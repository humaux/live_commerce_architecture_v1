// Display-only preflight: the server's suggestion and create receipt remain authoritative.
const handleShape = /^[a-z0-9]([a-z0-9-]{1,28}[a-z0-9])$/;
const reserved = new Set(["www", "admin", "api", "hooks", "shop", "mail", "static", "cdn", "assets", "app", "help", "support", "status", "stores"]);
export function handleNameHint(name: string): "reserved" | "format" | null {
  const slug = name.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
  if (reserved.has(slug) || name.trim().toLowerCase().startsWith("xn--")) return "reserved";
  return handleShape.test(slug) ? null : "format";
}
export function parseHandleSuggestion(value: unknown): { suggested: string; available: boolean } | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const item = value as Record<string, unknown>;
  if (Object.keys(item).sort().join() !== "available,suggested" || typeof item.suggested !== "string" ||
      !handleShape.test(item.suggested) || reserved.has(item.suggested) || item.suggested.startsWith("xn--") || typeof item.available !== "boolean") return null;
  return { suggested: item.suggested, available: item.available };
}
export function validStorefrontReceipt(handle: unknown, origin: unknown): boolean {
  if (typeof handle !== "string" || !handleShape.test(handle) || reserved.has(handle) || handle.startsWith("xn--") || typeof origin !== "string") return false;
  // Empty means the backend has not configured a platform domain. Do not invent one.
  if (origin === "") return true;
  if (origin.length > 261 || !origin.startsWith(`https://${handle}.`)) return false;
  return /^https:\/\/(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(origin);
}
