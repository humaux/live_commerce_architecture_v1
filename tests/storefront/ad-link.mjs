// Gate AL1 (BROWSER-less, real production storefront build): the link the ads SQL freezes (PRODUCT_TRAFFIC link_url, migrations/0074)
// and the Meta feed `link` (migrations/0080) is origin + "/products/" + product id. Fetch exactly that path from `next start`
// and check the FIRST hop: a permanent redirect to the locale route. The second hop (/{locale}/products/{id} -> 308 -> the slug page,
// 200 for a published store) needs a store behind the BFF, so it is asserted twice elsewhere: by the MOCK shell gate SF07
// (tests/storefront/shop-gate.mjs) and, on the REAL stack (Go + PG + published store), by gate AL2 in tests/storefront/ads-consent.mjs
// (--browser-meta-ads). Here there is no upstream (COMMERCE_BUYER_WEB_ENABLED=0), so the product page would be a 404.
// Needs `pnpm run build:storefront` first (scripts/dev/test-local.sh --browser-meta-ads does it). Usage: node tests/storefront/ad-link.mjs
// Depends on: tests/helpers/next-startup.mjs (automatic-port retry) and tests/helpers/owned-child.mjs (process-group cleanup that is safe
// before the first spawn and never masks the original error, #132).
import { spawn } from "node:child_process";
import { once } from "node:events";
import { createWriteStream, readFileSync } from "node:fs";
import { mkdtemp, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { nextAttemptLog, startNextWithPortRetry } from "../helpers/next-startup.mjs";
import { withOwnedChild } from "../helpers/owned-child.mjs";

for (const f of ["migrations/0074_meta_ads.sql", "migrations/0080_meta_capi.sql"])
  if (!readFileSync(f, "utf8").includes("||'/products/'||")) throw new Error(`${f} no longer emits origin||'/products/'||id; update this gate`);

// #116: no reserve-then-free port. startNextWithPortRetry picks it and relaunches only an automatic-port EADDRINUSE exit; its proof is
// the attempt's log file, so the Next output goes to a private temp file instead of memory. `owned` tracks the current attempt.
const logDir = await mkdtemp(path.join(tmpdir(), "lc-al1-"));
// pnpm and the local Node launcher can each fork: withOwnedChild terminates only this test's detached process group.
await withOwnedChild(async (owned) => {
  const base = await startNextWithPortRetry({}, async ({ port, attempt, track }) => {
    const file = nextAttemptLog(path.join(logDir, "next.log"), attempt);
    const log = createWriteStream(file, { flags: "wx", mode: 0o600 });
    await once(log, "open");
    const child = spawn("pnpm", ["--filter", "@live-commerce/storefront", "exec", "next", "start", "--hostname", "127.0.0.1", "--port", String(port)], {
      env: { ...process.env, COMMERCE_BUYER_WEB_ENABLED: "0" }, stdio: ["ignore", log, log], detached: process.platform !== "win32",
    });
    owned.adopt(child);
    track(child, log);
    const url = `http://127.0.0.1:${port}`;
    for (let i = 0; ; i++) {
      if (owned.closed) throw new Error("storefront exited before readiness\n" + await readFile(file, "utf8"));
      try { await fetch(url + "/"); return url; } catch { if (i > 60) throw new Error("storefront did not start\n" + await readFile(file, "utf8")); await new Promise((r) => setTimeout(r, 500)); }
    }
  });
  const id = "0b2f6f3e-3c4d-4a59-8f0e-1a2b3c4d5e6f";
  const res = await fetch(`${base}/products/${id}`, { redirect: "manual" });
  if (res.status !== 308) throw new Error(`AL1 FAIL: GET /products/${id} -> ${res.status}, expected a permanent redirect`);
  if (res.headers.get("location") !== `/zh-TW/products/${id}`) throw new Error(`AL1 FAIL: redirected to ${res.headers.get("location")}`);
  console.log(`AL1 PASS: /products/${id} -> 308 ${res.headers.get("location")} (second hop to the slug page: SF07)`);
}, { label: "AL1", dir: logDir });
