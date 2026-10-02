import assert from "node:assert/strict";
import { readdirSync, existsSync, readFileSync } from "node:fs";
import { test } from "node:test";
import {
  routes,
  groups,
  matchRoute,
  visibleGroups,
  canOpen,
} from "../../apps/admin/src/routes.ts";
import { shellCopy } from "../../apps/admin/src/shell-copy.ts";
const walk = (dir: string): string[] =>
  readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? walk(`${dir}/${e.name}`) : [`${dir}/${e.name}`],
  );
test("G-UI1 Next pages and route registry are bidirectional, unique, typed, documented", () => {
  const pages = walk("apps/admin/app/[locale]")
    .filter((p) => p.endsWith("/page.tsx"))
    .map(
      (p) =>
        p.replace("apps/admin/app/[locale]", "").replace("/page.tsx", "") ||
        "/",
    );
  assert.deepEqual([...routes.map((r) => r.path)].sort(), pages.sort());
  assert.equal(new Set(routes.map((r) => r.id)).size, routes.length);
  assert.equal(new Set(routes.map((r) => r.path)).size, routes.length);
  for (const route of routes) {
    assert.ok(route.permission || route.public, route.id);
    assert.ok(existsSync(route.spec), route.id);
    assert.match(route.spec, /^tests\/.*(?:\.spec\.ts|(?:-gate|-browser)\.mjs)$/, `${route.id}: spec must be executable browser coverage, not a design brief`);
    for (const locale of ["en", "zh-TW", "zh-CN"] as const)
      assert.ok(shellCopy[locale][route.labelKey]);
  }
  assert.equal(groups.length, 10);
  assert.doesNotMatch(
    readFileSync("apps/admin/components/WorkspaceFrame.tsx", "utf8"),
    /const (nav|pageRoutes) = \[/,
  );
});
test("G-UI1 copy key parity: no missing or extra key in any locale", () => {
  for (const locale of ["zh-TW", "zh-CN"] as const)
    assert.deepEqual(
      Object.keys(shellCopy[locale]).sort(),
      Object.keys(shellCopy.en).sort(),
    );
});
test("G-UI2 staff without catalog:read cannot see products/inventory or open a product URL", () => {
  const access = { role: "staff", permissions: ["orders:read"] };
  assert.equal(
    visibleGroups(access).some((g) => g.id === "catalog"),
    false,
  );
  assert.equal(canOpen(matchRoute("/products/example")!, access), false);
  assert.equal(visibleGroups(null).length, 0);
  assert.equal(
    visibleGroups({ role: "owner", permissions: [] }).some(
      (g) => g.id === "messages",
    ),
    false,
  );
});
test("G-UI1 dynamic and public routes match, details never appear as navigation", () => {
  assert.equal(matchRoute("/customers/id")?.id, "customer-detail");
  assert.equal(matchRoute("/products/import")?.id, "product-import");
  assert.equal(matchRoute("/customers/id/foreign"), undefined);
  for (const r of routes.filter((r) => r.path.includes("[") || r.public))
    assert.equal(r.nav, false);
});
