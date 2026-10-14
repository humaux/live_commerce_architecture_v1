// pay-rm2 guard: PAYUNi was removed by owner decision (2026-10-06 / 2026-10-14). No merchant-admin or storefront source,
// copy, config or CSP may mention it again. Tests are exempt because they assert the inverse (a PAYUNi provider/code from
// the API is rejected). Equivalent of: git grep -il payuni -- apps/admin apps/storefront ':!**/tests/**'
import test from "node:test";
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const apps = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const skipDirs = new Set(["node_modules", ".next", "tests", "test-results", ".turbo", "coverage"]);
const textExt = new Set([".ts", ".tsx", ".js", ".mjs", ".cjs", ".json", ".css", ".md", ".html", ".svg", ".txt", ".yml", ".yaml"]);

function* walk(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (!skipDirs.has(entry.name)) yield* walk(path.join(dir, entry.name));
    } else if (textExt.has(path.extname(entry.name))) yield path.join(dir, entry.name);
  }
}

test("pay-rm2 no PAYUNi in apps/admin or apps/storefront outside tests", () => {
  const scanned = [];
  const hits = [];
  for (const app of ["admin", "storefront"])
    for (const file of walk(path.join(apps, app))) {
      scanned.push(file);
      if (/payuni/i.test(readFileSync(file, "utf8"))) hits.push(path.relative(apps, file));
    }
  assert.ok(scanned.length > 50, `guard must scan real sources, saw ${scanned.length} files`);
  assert.deepEqual(hits, []);
});
