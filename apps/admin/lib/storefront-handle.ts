// The service receipt is authoritative; onboarding has no suggestion/preflight endpoint.
const handleShape = /^[a-z0-9]([a-z0-9-]{1,28}[a-z0-9])$/;
const reserved = new Set(["www", "admin", "api", "hooks", "shop", "mail", "static", "cdn", "assets", "app", "help", "support", "status", "stores"]);
export function validStorefrontReceipt(handle: unknown, origin: unknown): boolean {
  if (typeof handle !== "string" || !handleShape.test(handle) || reserved.has(handle) || handle.startsWith("xn--") || typeof origin !== "string") return false;
  // Empty means the backend has not configured a platform domain. Do not invent one.
  if (origin === "") return true;
  if (origin.length > 261 || !origin.startsWith(`https://${handle}.`)) return false;
  return /^https:\/\/(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(origin);
}
