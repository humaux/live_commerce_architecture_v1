// Shared by the public (cookie-less, bearer-less) storefront routes that call the private Go buyer API with only the BFF
// key and the verified Host-derived origin: app/media/p/[productID]/[imageID]/route.ts (product photos). Same rules as
// app/feeds/meta.csv/route.ts, which still carries its own copy (its unit did not own lib/; adopt this module there
// when that file is next touched). Owns: origin derivation from Host, the upstream env check and a bounded body read.
// Never forwards a caller header, cookie or query string, and never reads a tenant/store id from the request.
import { isIP } from "node:net";

const TOKEN = /^[A-Za-z0-9_-]{43}$/;

// Lowercase DNS name, no port/IP/userinfo/comma, at least two valid labels -> https origin; else null.
export function candidateOrigin(host: string | null): string | null {
  if (
    !host ||
    host.length > 253 ||
    host !== host.toLowerCase() ||
    host.includes(":") ||
    host.includes(",") ||
    host.includes("%") ||
    host.includes("@") ||
    host.endsWith(".") ||
    isIP(host)
  )
    return null;
  const labels = host.split(".");
  if (
    labels.length < 2 ||
    labels.some((label) => label.length > 63 || !/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/.test(label))
  )
    return null;
  return `https://${host}`;
}

// COMMERCE_BUYER_WEB_ENABLED=1, COMMERCE_BUYER_API_ORIGIN (https, or loopback http with a port, no path/userinfo/query)
// and COMMERCE_BUYER_BFF_KEY (43-char base64url) exactly as lib/buyer-server.ts config(); null when anything is off.
export function upstreamConfig(env: Record<string, string | undefined>): { api: string; bff: string } | null {
  if ((env.COMMERCE_BUYER_WEB_ENABLED ?? "") !== "1") return null;
  const raw = env.COMMERCE_BUYER_API_ORIGIN ?? "";
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    return null;
  }
  const loopback =
    url.protocol === "http:" && (url.hostname === "127.0.0.1" || url.hostname === "localhost") && !!url.port;
  if (
    raw !== url.origin ||
    (url.protocol !== "https:" && !loopback) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  )
    return null;
  const bff = env.COMMERCE_BUYER_BFF_KEY ?? "";
  return TOKEN.test(bff) ? { api: raw, bff } : null;
}

// Whole body up to `max` bytes, or null when it is longer (the stream is cancelled).
export async function readBounded(body: ReadableStream<Uint8Array> | null, max: number): Promise<Uint8Array | null> {
  if (!body) return null;
  const reader = body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > max) {
      await reader.cancel();
      return null;
    }
    chunks.push(value);
  }
  return Buffer.concat(chunks);
}
