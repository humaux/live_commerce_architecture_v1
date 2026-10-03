// First-party click envelope for storefront proxy -> buyer BFF -> Go checkout Begin (AT1).
// No Pixel, network, PII, tenant selection or consent decision lives here. Go must verify the draft's store.
import { createHmac, randomBytes, timingSafeEqual } from "node:crypto";

export const AD_TOUCH_COOKIE = "lc_ad_touch";
export const AD_TOUCH_TTL = 7 * 24 * 60 * 60;
const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const CLICK = /^[A-Za-z0-9_-]{1,500}$/;
const DECIMAL = /^[0-9]{1,20}$/;
const names = [AD_TOUCH_COOKIE, "lc_fbc", "lc_fbp"] as const;

export type AdTouch = Readonly<{
  draft_id: string;
  clicked_at: string;
  fbc: string;
  fbp: string;
}>;
export type AdTouchCapture = Readonly<{ touch: AdTouch; cookies: readonly string[] }>;

function cookieValues(header: string): Map<string, string> | null {
  if (header.length > 16_384) return null;
  const found = new Map<string, string>();
  for (const pair of header.split(";")) {
    const at = pair.indexOf("=");
    if (at < 0) continue;
    const name = pair.slice(0, at).trim();
    if (!(names as readonly string[]).includes(name)) continue;
    if (found.has(name)) return null; // Never select one of two same-name cookies.
    found.set(name, pair.slice(at + 1).trim());
  }
  return found;
}

function originOK(origin: string): boolean {
  try {
    const url = new URL(origin);
    return url.origin === origin && url.protocol === "https:" && !url.port &&
      /^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$/.test(url.hostname);
  } catch { return false; }
}

function signature(payload: string, origin: string, key: Buffer): Buffer {
  return createHmac("sha256", key).update("ad-touch-v1\0").update(origin).update("\0").update(payload).digest();
}

function timestamp(value: string): number | null {
  if (!/^[1-9][0-9]{0,15}$/.test(value)) return null;
  const n = Number(value);
  return Number.isSafeInteger(n) ? n : null;
}

function metaID(value: unknown, click: boolean, now: number): value is string {
  if (typeof value !== "string") return false;
  const parts = value.split(".");
  if (parts.length !== 4 || parts[0] !== "fb" || parts[1] !== "1") return false;
  const created = timestamp(parts[2]);
  return created !== null && created <= now && (click ? CLICK : DECIMAL).test(parts[3]);
}

export function readAdTouch(header: string, origin: string, key: Buffer, now = Date.now()): AdTouch | null {
  if (key.length !== 32 || !originOK(origin) || !Number.isSafeInteger(now) || now < 1) return null;
  const cookies = cookieValues(header);
  const envelope = cookies?.get(AD_TOUCH_COOKIE);
  if (!cookies || !envelope || envelope.length > 2048) return null;
  const parts = envelope.split(".");
  if (parts.length !== 2 || !/^[A-Za-z0-9_-]+$/.test(parts[0]) || !/^[A-Za-z0-9_-]{43}$/.test(parts[1])) return null;
  const actual = Buffer.from(parts[1], "base64url");
  if (actual.length !== 32 || actual.toString("base64url") !== parts[1] ||
      !timingSafeEqual(actual, signature(parts[0], origin, key))) return null;
  let value: unknown;
  try { value = JSON.parse(Buffer.from(parts[0], "base64url").toString("utf8")); } catch { return null; }
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const row = value as Record<string, unknown>;
  if (Object.keys(row).sort().join(",") !== "clicked_at,draft_id,fbc,fbp" ||
      typeof row.draft_id !== "string" || !UUID.test(row.draft_id) || typeof row.clicked_at !== "string" ||
      !metaID(row.fbc, true, now) || !metaID(row.fbp, false, now)) return null;
  const clicked = Date.parse(row.clicked_at);
  if (!Number.isSafeInteger(clicked) || clicked < 1 || clicked > now || now - clicked > AD_TOUCH_TTL * 1000 ||
      new Date(clicked).toISOString() !== row.clicked_at || row.fbc.split(".")[2] !== String(clicked) ||
      cookies.get("lc_fbc") !== row.fbc || cookies.get("lc_fbp") !== row.fbp) return null;
  return { draft_id: row.draft_id, clicked_at: row.clicked_at, fbc: row.fbc, fbp: row.fbp };
}

// Returning null means NO Set-Cookie: malformed/ambiguous links cannot destroy a valid previous click.
export function captureAdTouch(
  url: URL, header: string, origin: string, key: Buffer, now = Date.now(),
  randomDecimal = randomBytes(8).readBigUInt64BE().toString(),
): AdTouchCapture | null {
  if (key.length !== 32 || !originOK(origin) || !Number.isSafeInteger(now) || now < 1 ||
      now > 8_640_000_000_000_000 || !DECIMAL.test(randomDecimal)) return null;
  const drafts = url.searchParams.getAll("lc_ad");
  const clicks = url.searchParams.getAll("fbclid");
  if (drafts.length !== 1 || !UUID.test(drafts[0]) || clicks.length !== 1 || !CLICK.test(clicks[0])) return null;
  const previous = readAdTouch(header, origin, key, now);
  // Meta wire shape (D2); raw IDs are not hashed. The signature authenticates both cookie values together.
  // https://developers.facebook.com/docs/marketing-api/conversions-api/parameters/fbp-and-fbc/ (2026-10-04;
  // docs login wall; fbc shape corroborated in facebook/facebook-for-woocommerce's _collectAttribution).
  const touch: AdTouch = {
    draft_id: drafts[0], clicked_at: new Date(now).toISOString(),
    fbc: `fb.1.${now}.${clicks[0]}`, fbp: previous?.fbp ?? `fb.1.${now}.${randomDecimal}`,
  };
  const payload = Buffer.from(JSON.stringify(touch)).toString("base64url");
  const envelope = `${payload}.${signature(payload, origin, key).toString("base64url")}`;
  const cookie = (name: string, value: string) =>
    `${name}=${value}; Path=/; Max-Age=${AD_TOUCH_TTL}; HttpOnly; Secure; SameSite=Lax`;
  return { touch, cookies: [cookie(AD_TOUCH_COOKIE, envelope), cookie("lc_fbc", touch.fbc), cookie("lc_fbp", touch.fbp)] };
}
