// Purpose: Guard shell route permissions, navigation vocabulary and review-driven presentation copy.
// Depends on: node:test/assert/fs; admin route registry, shell/catalog/claims/editor/legal copy and source wiring.
// Used by: scripts/dev/test-node.sh and G-UI architecture checks; no browser or external service calls.
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
import { pageTitle } from "../../apps/admin/src/page-title.ts";
import { catalogCopy } from "../../apps/admin/lib/catalog-v2-copy.ts";
import { claimsCopy } from "../../apps/admin/lib/claims-copy.ts";
import { productEditorCopy } from "../../apps/admin/lib/product-editor-copy.ts";
import { platformLegal } from "../../apps/admin/lib/platform-legal.ts";
test("Readiness jump controls expose the actual active section, not a click flag", () => {
  const readiness = readFileSync("apps/admin/components/ProductReadiness.tsx", "utf8");
  const form = readFileSync("apps/admin/components/ProductDocumentForm.tsx", "utf8");
  assert.match(readiness, /aria-controls=\{item\.key\}/);
  assert.match(readiness, /aria-current=\{activeSection === item\.key \? "location" : undefined\}/);
  assert.equal((form.match(/activeSection=\{section\}/g) ?? []).length, 2);
  assert.match(readiness, /data-ready=\{item\.ok\}/);
});
test("Integrator P2: legal CN comment ordering and a distinct three-image recommendation", () => {
  assert.equal(JSON.stringify(platformLegal["zh-CN"]).includes("留言收单"), false);
  assert.equal(JSON.stringify(platformLegal["zh-CN"]).includes("评论收单"), true);
  for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
    const copy = productEditorCopy[locale];
    assert.match(copy.recommendedImages, /3/);
    assert.notEqual(copy.recommendedImages, copy.images);
  }
  const form = readFileSync("apps/admin/components/ProductDocumentForm.tsx", "utf8");
  assert.match(form, /label: c\.recommendedImages, ok: mainPhotoCount\(photos\) >= 3/);
  assert.match(form, /label: c\.images, ok: mainPhotoCount\(photos\) > 0/);
});
test("ADM04 and ADM06 explain unavailable SKUs and keep collection vocabulary consistent", () => {
  for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
    const c = claimsCopy[locale];
    assert.ok(c.skuChooseProduct?.length > 0, `${locale}: product prerequisite`);
    assert.ok(c.skuNotAvailable?.length > 0, `${locale}: unavailable options`);
    assert.equal(catalogCopy[locale].collections.title, shellCopy[locale].collections);
    assert.equal(catalogCopy[locale].nav.collections, shellCopy[locale].collections);
    if (locale !== "en")
      for (const text of Object.values(catalogCopy[locale].collections))
        if (typeof text === "string") assert.doesNotMatch(text, /集合/, `${locale}: ${text}`);
  }
  const claims = readFileSync("apps/admin/components/StudioClaims.tsx", "utf8");
  assert.match(claims, /aria-describedby="claims-offer-sku-hint"/);
  assert.match(claims, /hint=\{!offerForm\.product \? c\.skuChooseProduct : !skus\.length \? c\.skuNotAvailable : undefined\}/);
});
test("ADM06 create/detail titles share the route vocabulary without adding a phantom Next page", () => {
  for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
    assert.equal(
      pageTitle(locale, `/${locale}/products/new`),
      shellCopy[locale].productNew,
    );
    assert.equal(
      pageTitle(locale, "/products/product-id"),
      shellCopy[locale].product,
    );
    for (const route of routes.filter(
      (route) => route.path !== "/products/[product]",
    ))
      assert.equal(
        pageTitle(locale, route.path),
        shellCopy[locale][route.labelKey],
      );
  }
});
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
    assert.match(
      route.spec,
      /^tests\/.*(?:\.spec\.ts|(?:-gate|-browser)\.mjs)$/,
      `${route.id}: spec must be executable browser coverage, not a design brief`,
    );
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
    true,
  );
  assert.equal(visibleGroups(access).some((g) => g.id === "messages"), false);
  const inbox = matchRoute("/messages")!;
  assert.equal(inbox.path, "/messages");
  assert.equal(inbox.permission, "inbox:read");
  assert.equal(canOpen(inbox, access), false);
});
// --browser-manual-order sweep: /orders/new was guarded by "orders:write", a permission that exists nowhere in Go or SQL, so no store member except
// the owner could ever open the manual-order page (every other role got the shell's 403). Go guards a manual order with inventory:reserve
// (migration 0094: "no new permission"; the BFF also needs catalog:read), so that is what the registry must name.
test("G-UI2 every route permission is a permission Go knows, and /orders/new needs inventory:reserve", () => {
  const backend = [...walk("migrations"), ...walk("internal")]
    .filter((file) => /\.(sql|go)$/.test(file) && !file.endsWith("_test.go"))
    .map((file) => readFileSync(file, "utf8"))
    .join("\n");
  for (const route of routes)
    if (route.permission && route.permission !== "owner")
      assert.ok(
        backend.includes(`'${route.permission}'`) ||
          backend.includes(`"${route.permission}"`),
        `${route.id}: "${route.permission}" is not a permission Go or SQL knows`,
      );
  const manual = matchRoute("/orders/new")!;
  assert.equal(manual.permission, "inventory:reserve");
  assert.equal(
    canOpen(manual, { role: "staff", permissions: ["inventory:reserve"] }),
    true,
  );
  assert.equal(
    canOpen(manual, { role: "staff", permissions: ["orders:read"] }),
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
// W3-U5 CI regression (run 37509052114): a second `nav: true` route in the orders group (the returns page) made the group non-singleton, so
// WorkspaceFrame stopped rendering the `nav-orders` group button (it renders `nav-group-orders` + collapsed sub-links) and every browser mode
// failed right after login. Further orders pages must be reachable from the orders pages themselves (`nav: false`), not from the rail.
test("Every browser spec clicks nav-orders: the orders group stays a singleton, so returns/new/print routes are nav: false", () => {
  const frame = readFileSync("apps/admin/components/WorkspaceFrame.tsx", "utf8");
  assert.match(frame, /singleton && group\.routes\[0\]\.id === "orders"\s*\? "nav-orders"/);
  for (const access of [
    { role: "owner", permissions: [] },
    { role: "staff", permissions: ["orders:read"] },
  ]) {
    const orders = visibleGroups(access).find((g) => g.id === "orders");
    assert.deepEqual(orders?.routes.map((r) => r.id), ["orders"], `${access.role}: orders rail group`);
  }
  const returns = matchRoute("/returns")!;
  assert.equal(returns.group, "orders");
  assert.equal(returns.nav, false);
});
