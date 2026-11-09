// Purpose: Guard shell route permissions, navigation vocabulary and review-driven presentation copy.
// Depends on: node:test/assert/fs; admin route registry, shell/catalog/claims/editor/legal copy and source wiring.
// Used by: scripts/dev/test-node.sh and G-UI architecture checks; no browser or external service calls.
import assert from "node:assert/strict";
import { readdirSync, existsSync, readFileSync } from "node:fs";
import { test } from "node:test";

const GALLERY_PAGES = ["/ui-gallery", "/ui-gallery/templates/[name]"];
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
test("LP2 parity group helper identifies Customers disclosure independently of its identically named leaf", () => {
  const raw = readFileSync("tests/admin/parity-shell-browser.mjs", "utf8");
  const start = raw.indexOf("function group(page,");
  const end = raw.indexOf("\n}\n", start) + 2;
  const actualGroup = new Function("copy", `${raw.slice(start, end)}; return group;`)({ en: { nav: ["Customers"] } });
  const page = {
    locator() { return this; },
    getByRole() { throw new Error("ambiguous Customers group and leaf"); },
    getByTestId(id: string) { return id; },
  };
  assert.equal(actualGroup(page, { group: "customers", index: 0 }, "en"), "nav-group-customers");
  assert.equal(actualGroup(page, { group: "orders", index: 0 }, "en"), "nav-orders");
});
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
    )
    // DS-1/DS-2: /ui-gallery and its page templates are the design-system gallery, not product routes. They are the named exemptions,
    // and each page must stay closed (404) unless LC_UI_GALLERY=1; --browser-ui-gallery proves both halves in a browser.
    .filter((path) => !GALLERY_PAGES.includes(path));
  for (const path of GALLERY_PAGES) {
    assert.match(
      readFileSync(`apps/admin/app/[locale]${path}/page.tsx`, "utf8"),
      /process\.env\.LC_UI_GALLERY !== "1"\) notFound\(\)/,
      `${path} must 404 unless LC_UI_GALLERY=1`,
    );
  }
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
test("CVS lookup is registered, localized and available only with order access, outside the rail", () => {
  const lookup = matchRoute("/orders/cvs-match")!;
  assert.equal(lookup.id, "cvs-match");
  assert.equal(lookup.group, "orders");
  assert.equal(lookup.permission, "orders:read");
  assert.equal(lookup.nav, false);
  assert.equal(lookup.spec, "tests/admin/parity-order-ops.spec.ts");
  assert.equal(canOpen(lookup, null), false);
  assert.equal(canOpen(lookup, { role: "staff", permissions: ["catalog:read"] }), false);
  assert.equal(canOpen(lookup, { role: "staff", permissions: ["orders:read"] }), true);
  assert.equal(canOpen(lookup, { role: "owner", permissions: [] }), true);
  for (const [locale, title] of [["zh-TW", "查門市編號"], ["zh-CN", "查门店编号"], ["en", "Find store code"]] as const)
    assert.equal(pageTitle(locale, `/${locale}/orders/cvs-match`), title);
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
test("W6-U2 ledger is a settings in-page route, preserving existing navigation",()=>{
 const ledger=matchRoute("/settings/operations")!;assert.equal(ledger.nav,false);assert.equal(ledger.permission,"integration:read");assert.equal(ledger.spec,"tests/admin/operations-ads.spec.ts");
 assert.deepEqual(routes.filter(r=>r.group==="settings"&&r.nav).map(r=>r.id),["settings","team","billing","settlements"]);
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

// Exercise the actual HTTP fixture; only transport is MOCK, never a replacement handler.
test("LP2 shell second-store scope returns empty without leaking primary members", async () => {
  const { fixture, storeID, otherID, entityID } = await import("./shell-fixture.mjs");
  const f = await fixture();
  try {
    const request = (store: string, tail = "members/search", method = "POST") => fetch(`${f.origin}/v1/admin/stores/${store}/${tail}`, { method, headers: { authorization: `Bearer ${f.token}` }, signal: AbortSignal.timeout(5000) });
    const primary = await request(storeID);
    assert.equal(primary.status, 200);
    assert.equal((await primary.json()).members[0].display_name, "Synthetic member");
    const second = await request(otherID);
    assert.equal(second.status, 200, "known second MOCK store search must be explicit empty");
    assert.deepEqual(await second.json(), { members: [], next_cursor: null });
    for (const response of [await request(entityID), await request(otherID, "members/unknown"), await request(otherID, "members/search", "GET")]) {
      assert.equal(response.status, 503);
      assert.equal((await response.json()).code, "retry_later");
    }
  } finally { await f.close(); }
});
test("LP2 shell native body assertion retains second-store and localized empty checks", async () => {
  const raw = readFileSync("tests/admin/shell-browser.mjs", "utf8");
  assert.ok(raw.includes('await page.goto(`${base}/${locale}/?store=${otherID}`)'));
  assert.ok(raw.includes('membersCopy[locale].empty'));
  assert.ok(raw.includes('assert.equal(await members.getByRole("link", { name: "Synthetic member", exact: true }).count(), 0)'));
  const start = raw.indexOf('          if (id === "members") {');
  const end = raw.indexOf('          if (width < 1024)', start);
  const actual = new Function("page", "membersCopy", "locale", "assert", `return (async () => { const id = "members"; ${raw.slice(start, end)} })();`);
  const { membersCopy } = await import("../../apps/admin/components/members/copy.ts");
  for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
    let waited = false;
    const page = { getByTestId(id: string) {
      assert.equal(id, "members-list");
      return {
        getByText(text: string, options: { exact: boolean }) {
          assert.equal(text, membersCopy[locale].empty); assert.equal(options.exact, true);
          return { async waitFor(options: { state: string }) { assert.equal(options.state, "visible"); waited = true; } };
        },
        getByRole() { return { async count() { return 0; } }; },
      };
    } };
    await actual(page, membersCopy, locale, assert);
    assert.equal(waited, true);
    const leaked = { getByTestId() { return { getByText() { return { async waitFor() {} }; }, getByRole() { return { async count() { return 1; } }; } }; } };
    await assert.rejects(actual(leaked, membersCopy, locale, assert));
  }
  assert.match(raw, /await target\.click\(\)/);
  assert.match(raw, /await page\.reload\(/);
  assert.match(raw, /getAttribute\("aria-current"\)/);
});
