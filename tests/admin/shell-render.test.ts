// Purpose: Render the actual shared shell/header JSX across the route registry, including private-data-safe failure states.
// Depends on: node:test/assert/fs/vm, installed typescript-api/React SSR, real route/title/copy/i18n modules; no browser or network.
// Used by: scripts/dev/test-node.sh; LC_SHELL_RENDER_FAULT=headings|nav calibrates this test in memory only.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { test } from "node:test";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import * as registry from "../../apps/admin/src/routes.ts";
import { pageTitle } from "../../apps/admin/src/page-title.ts";
import { shellCopy } from "../../apps/admin/src/shell-copy.ts";
import { navAccessFrom } from "../../apps/admin/lib/team-model.ts";
import * as i18n from "../../packages/i18n/src/index.ts";

const requireApp = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const React = requireApp("react"), { renderToStaticMarkup } = requireApp("react-dom/server");
const styles = new Proxy({}, { get: (_, key) => String(key) });
const css = { __esModule: true, default: styles };
let pathname = "/en/orders", phase = "ready", stateCalls = 0;
const store = { id: "11111111-1111-4111-8111-111111111111", name: "PRIVATE STORE SENTINEL", currency: "TWD", role: "owner", permissions: [] };

function jsx(path: string, resolve: (name: string) => unknown, transform = (s: string) => s) {
  const exports: Record<string, any> = {};
  const source = transform(readFileSync(path, "utf8"));
  runInNewContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText, { exports, require: resolve });
  return exports;
}
const presentation = jsx("packages/ui/src/Presentation.tsx", (name) => name.endsWith(".css") ? css : requireApp(name), (source) =>
  process.env.LC_SHELL_RENDER_FAULT === "headings" ? source.replace("<h1>{title}</h1>", "<h2>{title}</h2>") : source);
const ui = jsx("packages/ui/src/AppShell.tsx", (name) => name.endsWith(".css") ? css : name === "./Presentation" ? presentation : requireApp(name));
const header = jsx("apps/admin/components/AdminPageHeader.tsx", (name) => name === "@live-commerce/ui" ? ui :
  name === "@/src/page-title" ? { pageTitle } : name === "next/navigation" ? { usePathname: () => pathname } : requireApp(name));
const frame = jsx("apps/admin/components/WorkspaceFrame.tsx", (name) => {
  if (name === "react") return { ...React, useState: (initial: unknown) => {
    const at = stateCalls++;
    if (at === 0) return [phase === "ready" || phase === "forbidden" ? { key: store.id, stores: [phase === "forbidden" ? { ...store, role: "viewer", permissions: ["store:read"] } : store] } : null, () => {}];
    if (at === 1) return [phase === "expired" || phase === "unavailable" ? phase : null, () => {}];
    return [initial, () => {}];
  } };
  if (name === "next/navigation") return { usePathname: () => pathname, useSearchParams: () => new URLSearchParams({ store: store.id }), useRouter: () => ({ push() {} }) };
  if (name === "@live-commerce/ui") return ui;
  if (name === "@live-commerce/i18n") return i18n;
  if (name === "@/src/shell-copy") return { shellCopy };
  if (name === "@/src/page-title") return { pageTitle };
  if (name === "@/lib/team-model") return { navAccessFrom };
  if (name === "@/src/routes") return { ...registry, visibleGroups: (access: Parameters<typeof registry.visibleGroups>[0]) => registry.visibleGroups(access).map((group) =>
    process.env.LC_SHELL_RENDER_FAULT === "nav" && group.id === "orders" ? { ...group, routes: [...group.routes, { ...group.routes[0], id: "rogue-orders-nav" }] } : group) };
  if (name === "@/src/shell/api") return { readWorkspace() { throw new Error("SSR must not call the network"); }, logoutWorkspace() { throw new Error("SSR must not log out"); } };
  if (name === "@/lib/company") return { company: { productName: "DaWan Live" } };
  // Domain banners/footer do not own h1/navigation. Exclude their network effects, not the actual shell/header render.
  if (name === "./BillingBanner") return { BillingBanner: () => null };
  if (name === "./MetaHealthBanner") return { MetaHealthBanner: () => null };
  if (name === "./OperatorFooter") return { OperatorFooter: () => null };
  if (name === "./Icon") return { Icon: () => React.createElement("svg", { "aria-hidden": true }) };
  return requireApp(name);
});

function render(locale: keyof typeof shellCopy, path: string, state = "ready") {
  pathname = `/${locale}${path === "/" ? "" : path}`; phase = state; stateCalls = 0;
  return renderToStaticMarkup(React.createElement(frame.WorkspaceFrame, { locale, active: "", storeName: store.name },
    React.createElement(React.Fragment, null, React.createElement(header.AdminPageHeader, { locale }), React.createElement("p", null, "PRIVATE BUYER SENTINEL"))));
}
const h1Count = (html: string) => (html.match(/<h1(?:\s|>)/g) ?? []).length;

test("shell-registry: every routed path has exactly one nonempty shared h1 in all three locales", () => {
  for (const locale of ["zh-TW", "zh-CN", "en"] as const) for (const route of registry.routes) {
    const path = route.path.replace(/\[[^\]]+\]/g, "fixture-id");
    const html = render(locale, path);
    assert.equal(h1Count(html), 1, `${locale} ${path}: one h1`);
    const heading = /<h1[^>]*>([\s\S]*?)<\/h1>/.exec(html)?.[1] ?? "";
    const expected = pageTitle(locale, path).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
    assert.ok(heading.trim(), `${locale} ${path}: nonempty h1`);
    assert.equal(heading, expected, `${locale} ${path}: h1 itself matches registry, not just a breadcrumb`);
  }
});
test("shell-registry: orders stays one actual group button on live and non-live routes", () => {
  for (const path of ["/orders", "/products", "/collections", "/studio/console"]) {
    const html = render("en", path);
    assert.equal((html.match(/<button[^>]*data-testid="nav-orders"/g) ?? []).length, 1, `${path}: singleton orders button`);
    assert.equal((html.match(/<button[^>]*class="navButton"[^>]*data-testid="nav-orders"/g) ?? []).length, 1, `${path}: direct group button, not a collapsed leaf`);
    assert.equal(html.includes('data-testid="nav-group-orders"'), false);
  }
});
test("shell-registry: loading/expired/unavailable/forbidden keep one safe heading and never leak private content", () => {
  for (const locale of ["zh-TW", "zh-CN", "en"] as const) for (const state of ["loading", "expired", "unavailable", "forbidden"]) {
    const html = render(locale, "/products", state);
    assert.equal(h1Count(html), 1, `${locale} ${state}: accessible status heading`);
    assert.equal(html.includes("PRIVATE BUYER SENTINEL"), false);
    if (state !== "forbidden") {
      assert.equal(html.includes(store.name), false);
      assert.equal(html.includes('data-testid="nav-orders"'), false, "unavailable authority must not render private navigation");
    }
  }
});
