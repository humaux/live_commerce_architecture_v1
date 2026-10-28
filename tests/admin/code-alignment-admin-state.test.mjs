// Purpose: CA-ADMIN-STATE static and server-render contract for shared admin empty, loading, error, forbidden and not-found displays.
// Depends on: node:test/assert/fs, typescript-api, React SSR and the real DS state/action primitives; no browser, server or network.
// Used by: scripts/dev/test-node.sh; browser interaction and persistence remain separate CI acceptance evidence.
// Invariants: no fetch, ACL decision, error-internal rendering, technical status copy or unsaved-draft safety claim.
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { test } from "node:test";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";

const requireApp = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const React = requireApp("react");
const { renderToStaticMarkup } = requireApp("react-dom/server");
const css = { __esModule: true, default: new Proxy({}, { get: (_, key) => String(key) }) };
const source = (path) => existsSync(path) ? readFileSync(path, "utf8") : "";

function load(path, resolve) {
  const exports = {};
  const compiled = ts.transpileModule(source(path), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
  }).outputText;
  runInNewContext(compiled, { exports, require: resolve, process });
  return exports;
}

const icon = load("packages/ui/src/ds/Icon.tsx", requireApp);
const logic = load("packages/ui/src/ds/logic.ts", requireApp);
const controls = load("packages/ui/src/ds/Controls.tsx", (name) =>
  name.endsWith(".css") ? css : name === "./Icon" ? icon : name === "./logic" ? logic : requireApp(name));
const states = load("packages/ui/src/ds/States.tsx", (name) =>
  name.endsWith(".css") ? css : name === "./Controls" ? controls : name === "./Icon" ? icon : name === "./logic" ? logic : requireApp(name));
const ds = { ...controls, ...states };
const sharedPath = "apps/admin/components/AdminStateBlock.tsx";
const forbiddenPath = "apps/admin/components/AdminForbidden.tsx";
const shared = () => load(sharedPath, (name) =>
  name.endsWith(".css") ? css : name === "@live-commerce/ui/ds" ? ds : requireApp(name));
const forbidden = () => load(forbiddenPath, (name) =>
  name.endsWith(".css") ? css : name === "@live-commerce/ui/ds" ? ds : requireApp(name));
const renderRoute = (path, props = {}, pathname = "/en/orders") => {
  const sharedModule = shared();
  const route = load(path, (name) => {
    if (name.endsWith(".css")) return css;
    if (name.endsWith("/AdminStateBlock")) return sharedModule;
    if (name === "next/navigation") return { usePathname: () => pathname };
    return requireApp(name);
  });
  return renderToStaticMarkup(React.createElement(route.default, props));
};

const boundaries = [
  "apps/admin/app/[locale]/error.tsx",
  "apps/admin/app/[locale]/not-found.tsx",
  "apps/admin/app/[locale]/loading.tsx",
  "apps/admin/app/global-error.tsx",
];

test("AD-09: all localized and root boundaries exist with source ownership headers", () => {
  for (const path of [...boundaries, sharedPath, forbiddenPath, "apps/admin/components/admin-state-block.css"]) {
    assert.ok(existsSync(path), `${path} is missing`);
  }
  for (const path of [...boundaries, sharedPath, forbiddenPath]) {
    const text = source(path);
    assert.match(text, /Purpose:/, path);
    assert.match(text, /Depends on:/, path);
    assert.match(text, /Used by:/, path);
  }
});

test("AD-09: localized errors reset only the existing segment and never expose error internals", () => {
  const text = source(boundaries[0]);
  assert.match(text, /onRetry=\{reset\}/);
  assert.match(text, /usePathname/);
  assert.match(text, /isLocale/);
  assert.doesNotMatch(text, /error\.(?:message|stack|digest)|fetch\s*\(/);
  const html = renderRoute(boundaries[0], { error: new Error("private upstream detail"), reset() {} });
  assert.match(html, /data-kind="error"/);
  assert.match(html, /role="alert"/);
  assert.match(html, /Try again/);
  assert.match(html, /href="\/en"/);
  assert.doesNotMatch(html, /private upstream detail|500|stack/i);
});

test("AD-09: not-found and loading are localized safe states without technical status codes", () => {
  const notFound = renderRoute(boundaries[1], {}, "/zh-TW/unknown");
  assert.match(notFound, /data-kind="empty"/);
  assert.match(notFound, /找不到/);
  assert.match(notFound, /href="\/zh-TW"/);
  assert.doesNotMatch(notFound, /404|500/);
  const loading = renderRoute(boundaries[2], {}, "/zh-CN/orders");
  assert.match(loading, /role="status"/);
  assert.match(loading, /正在加载/);
  assert.match(loading, /aria-hidden="true"/);
});

test("AD-09: global error owns html/body and styles, independent of the localized workspace", () => {
  const text = source(boundaries[3]);
  assert.match(text, /<html lang=\{locale\}>/);
  assert.match(text, /<body>/);
  assert.match(text, /import ["']\.\/globals\.css["']/);
  assert.doesNotMatch(text, /WorkspaceFrame|\[locale\]\/layout|Sidebar/);
  const html = renderRoute(boundaries[3], { error: new Error("secret"), reset() {} }, "/en/orders");
  assert.match(html, /^<html lang="en"><body>/);
  assert.match(html, /data-kind="error"/);
  assert.doesNotMatch(html, /secret|500/);
});

test("shared block renders real DS empty/error states and an accessible token-styled loading skeleton", () => {
  const { AdminStateBlock } = shared();
  const empty = renderToStaticMarkup(React.createElement(AdminStateBlock, {
    variant: "empty", title: "No orders", description: "Clear filters", actionHref: "/en/orders", actionLabel: "Clear filters",
  }));
  assert.match(empty, /data-kind="empty"/);
  assert.match(empty, /No orders/);
  assert.match(empty, /href="\/en\/orders"/);
  const failure = renderToStaticMarkup(React.createElement(AdminStateBlock, {
    variant: "error", title: "Unavailable", description: "Try again", onRetry() {}, retryLabel: "Try again",
  }));
  assert.match(failure, /data-kind="error"/);
  assert.match(failure, /role="alert"/);
  assert.match(failure, /<button[^>]*>Try again<\/button>/);
  const loading = renderToStaticMarkup(React.createElement(AdminStateBlock, { variant: "loading", title: "Loading orders" }));
  assert.match(loading, /role="status"/);
  assert.match(loading, /Loading orders/);
  assert.match(loading, /admin-state-loading/);
});

test("AD-10: forbidden display uses exact neutral Figma copy in every locale and no role mapping", () => {
  const expected = {
    en: ["You can’t use this feature", "Your current role can’t use this feature. Contact the store owner."],
    "zh-CN": ["你目前无法使用这个功能", "你目前的角色无法使用此功能，请联系店铺所有者。"],
    "zh-TW": ["你目前無法使用這個功能", "你目前的角色無法使用此功能，請聯絡店鋪擁有者。"],
  };
  const { AdminForbidden } = forbidden();
  for (const [locale, copy] of Object.entries(expected)) {
    const html = renderToStaticMarkup(React.createElement(AdminForbidden, { locale }));
    assert.match(html, /data-kind="permission"/);
    assert.ok(html.includes(copy[0]), `${locale} title`);
    assert.ok(html.includes(copy[1]), `${locale} body`);
    assert.match(html, new RegExp(`href="/${locale}"`));
    assert.doesNotMatch(html, /403|permission[_ -]?code|requiredRole|customers:read|orders:read/i);
  }
});

test("AD-11: customer and order empty copy has a title and safe guidance in all three locales", () => {
  const customers = load("apps/admin/lib/customers-copy.ts", requireApp).customersCopy;
  const orders = load("apps/admin/lib/orders-v2-copy.ts", requireApp).ordersV2Copy;
  for (const locale of ["en", "zh-CN", "zh-TW"]) {
    for (const key of ["empty", "emptySearch", "emptyDescription", "emptySearchDescription"]) {
      assert.equal(typeof customers[locale][key], "string", `customers ${locale}.${key}`);
      assert.ok(customers[locale][key].trim(), `customers ${locale}.${key}`);
    }
    for (const key of ["emptyTitle", "emptyDescription"]) {
      assert.equal(typeof orders[locale][key], "string", `orders ${locale}.${key}`);
      assert.ok(orders[locale][key].trim(), `orders ${locale}.${key}`);
    }
  }
});

test("AD-10/11: pages share display only while retaining existing read, filter and pagination controls", () => {
  const customers = source("apps/admin/components/Customers.tsx");
  const orders = source("apps/admin/components/MerchantOrders.tsx");
  for (const [text, name] of [[customers, "Customers"], [orders, "MerchantOrders"]]) {
    assert.match(text, /AdminForbidden/, name);
    assert.match(text, /AdminStateBlock/, name);
    assert.doesNotMatch(text, /fetch\s*\(/, name);
  }
  for (const control of ["customers-search", "customers-search-submit", "customers-tag-filter", "customers-refresh", "customers-previous", "customers-next"]) {
    assert.ok(customers.includes(`data-testid="${control}"`), control);
  }
  assert.match(customers, /read\.reload/);
  assert.match(customers, /q \|\| tag \? c\.emptySearch : c\.empty/);
  for (const control of ["OrderListFilters", "orders-tabs", "orders-previous", "orders-next"]) {
    assert.ok(orders.includes(control), control);
  }
  assert.match(orders, /v2\.emptyTitle/);
  assert.match(orders, /v2\.emptyDescription/);
  assert.match(orders, /onRetry=\{retry\}/);
});

test("new admin state CSS and components use DS tokens and never claim unsaved input is safe", () => {
  const files = [sharedPath, forbiddenPath, "apps/admin/components/admin-state-block.css", ...boundaries];
  for (const path of files) {
    const text = source(path);
    assert.doesNotMatch(text, /#[0-9a-fA-F]{3,8}\b/, path);
    assert.doesNotMatch(text, /unsaved|drafts? (?:are|is) (?:safe|preserved)|未保存.*(?:安全|保留)|未儲存.*(?:安全|保留)/i, path);
  }
  const cssText = source("apps/admin/components/admin-state-block.css");
  assert.match(cssText, /var\(--ds-surface\)/);
  assert.match(cssText, /var\(--ds-border\)/);
  assert.match(cssText, /var\(--ds-radius-/);
  assert.match(cssText, /@media/);
});
