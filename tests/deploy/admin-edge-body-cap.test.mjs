// Purpose: pin the admin edge (Caddy) request-body cap above every admin BFF upload cap, so a photo or CSV the app
//   accepts is never rejected at the edge with a bare 413 (production incident 2026-10-15: edge 1MB vs BFF 2.5 MiB).
// Depends on: deploy/caddy/Caddyfile, apps/admin/app/api/stores/[store]/[...resource]/route.ts, apps/admin/lib/merchant-tools-model.ts (read as text).
// Used by: scripts/dev/test-node.sh (foundation node tests).
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const units = { KB: 1000, MB: 1000 ** 2, KIB: 1024, MIB: 1024 ** 2, B: 1 };
const bytes = (n, u) => Number(n) * units[(u || "B").toUpperCase()];

test("admin edge body cap >= every admin BFF upload cap", () => {
  const caddy = readFileSync("deploy/caddy/Caddyfile", "utf8");
  const start = caddy.indexOf("{$LC_ADMIN_HOST} {");
  assert.ok(start >= 0, "admin site block present");
  // Search from the admin block: the platform-host block above also proxies to :3100.
  const block = caddy.slice(start, caddy.indexOf("reverse_proxy 127.0.0.1:3100", start));
  const m = block.match(/max_size\s+([\d.]+)\s*(KB|MB|KiB|MiB|B)?/i);
  assert.ok(m, "admin site block declares request_body max_size");
  const edge = bytes(m[1], m[2]);
  const route = readFileSync("apps/admin/app/api/stores/[store]/[...resource]/route.ts", "utf8");
  const upload = route.match(/const MAX_UPLOAD = ([\d.]+) \* 1024 \* 1024/);
  const csv = readFileSync("apps/admin/lib/merchant-tools-model.ts", "utf8").match(/MAX_CSV_BYTES = ([\d.]+) \* 1024 \* 1024/);
  assert.ok(upload && csv, "BFF caps found");
  for (const [name, cap] of [["photo", Number(upload[1]) * 1024 ** 2], ["csv", Number(csv[1]) * 1024 ** 2]]) {
    // multipart framing adds a few hundred bytes; require 64 KiB of headroom
    assert.ok(edge >= cap + 64 * 1024, `edge ${edge} B must exceed BFF ${name} cap ${cap} B + framing`);
  }
});
