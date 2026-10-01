// Gate AL1 (BROWSER-less, real production storefront build): the link the ads SQL freezes (PRODUCT_TRAFFIC link_url, migrations/0074)
// and the Meta feed `link` (migrations/0080) is origin + "/products/" + product id. Fetch exactly that path from `next start`
// and check the FIRST hop: a permanent redirect to the locale route. The second hop (/{locale}/products/{id} -> 308 -> the slug page,
// 200 for a published store) needs a store behind the BFF, so it is asserted twice elsewhere: by the MOCK shell gate SF07
// (tests/storefront/shop-gate.mjs) and, on the REAL stack (Go + PG + published store), by gate AL2 in tests/storefront/ads-consent.mjs
// (--browser-meta-ads). Here there is no upstream (COMMERCE_BUYER_WEB_ENABLED=0), so the product page would be a 404.
// Needs `pnpm run build:storefront` first (scripts/dev/test-local.sh --browser-meta-ads does it). Usage: node tests/storefront/ad-link.mjs
import { spawn } from "node:child_process";
import { readFileSync } from "node:fs";
import { createServer } from "node:net";

for (const f of ["migrations/0074_meta_ads.sql", "migrations/0080_meta_capi.sql"])
  if (!readFileSync(f, "utf8").includes("||'/products/'||")) throw new Error(`${f} no longer emits origin||'/products/'||id; update this gate`);

const port = await new Promise((res) => { const s = createServer().listen(0, "127.0.0.1", () => { const p = s.address().port; s.close(() => res(p)); }); });
const child = spawn("pnpm", ["--filter", "@live-commerce/storefront", "exec", "next", "start", "--hostname", "127.0.0.1", "--port", String(port)], {
  env: { ...process.env, COMMERCE_BUYER_WEB_ENABLED: "0" }, stdio: ["ignore", "pipe", "pipe"],
});
let log = "";
child.stdout.on("data", (d) => (log += d));
child.stderr.on("data", (d) => (log += d));
const stop = () => child.kill("SIGTERM");
process.on("exit", stop);
try {
  const base = `http://127.0.0.1:${port}`;
  for (let i = 0; ; i++) {
    try { await fetch(base + "/"); break; } catch { if (i > 60) throw new Error("storefront did not start\n" + log); await new Promise((r) => setTimeout(r, 500)); }
  }
  const id = "0b2f6f3e-3c4d-4a59-8f0e-1a2b3c4d5e6f";
  const res = await fetch(`${base}/products/${id}`, { redirect: "manual" });
  if (res.status !== 308) throw new Error(`AL1 FAIL: GET /products/${id} -> ${res.status}, expected a permanent redirect`);
  if (res.headers.get("location") !== `/zh-TW/products/${id}`) throw new Error(`AL1 FAIL: redirected to ${res.headers.get("location")}`);
  console.log(`AL1 PASS: /products/${id} -> 308 ${res.headers.get("location")} (second hop to the slug page: SF07)`);
} finally { stop(); }
