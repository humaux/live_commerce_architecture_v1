// Purpose: preserve one customers navigation entry, authorize import links, and gate historical private data by active/import origin.
// Depends on: Node/TypeScript AST, React SSR, actual Customers/CustomerDetail sources and shell registry.
// Used by: W5-U1 local regression and Node CI; controlled hooks are MOCK, not browser acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { routes, matchRoute } from "../../apps/admin/src/routes.ts";
import { customersCopy } from "../../apps/admin/lib/customers-copy.ts";
const appRequire = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const React = appRequire("react"), { renderToStaticMarkup } = appRequire("react-dom/server");
function component(path: string, name: string, globals: Record<string, unknown>) {
  const text = readFileSync(new URL(path, import.meta.url), "utf8");
  const tree = ts.createSourceFile(path, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const declaration = tree.statements.find(statement => ts.isFunctionDeclaration(statement) && statement.name?.text === name);
  assert.ok(declaration);
  const raw = declaration.getText(tree), exports: Record<string, any> = {};
  runInNewContext(ts.transpileModule((raw.startsWith("export") ? "" : "export ") + raw,
    { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText,
  { exports, require: appRequire, ...globals });
  return exports[name];
}
test("the static import subpage wins over customer detail and leaves the single customers nav intact", () => {
  assert.equal(matchRoute("/customers/import")?.id, "customer-import");
  assert.equal(routes.find(route => route.id === "customer-import")?.nav, false);
  assert.equal(routes.filter(route => route.group === "customers" && route.nav).length, 1);
});
test("the actual list offers import only to privacy holders and carries the current authorized store", () => {
  for (const permission of [false, true]) {
    let read = 0;
    const Customers = component("../../apps/admin/components/Customers.tsx", "Customers", {
      customersCopy, customerTagsCopy: { en: { filterLabel: "Tags", filterAll: "All" } },
      useState: (value: unknown) => [value, () => {}], useRef: (value: unknown) => ({ current: value }), useRouter: () => ({ push() {} }),
      useGuardedRead: () => ({ status: "ready", boundary: "scope", reload() {}, data: ++read === 1 ? { items: [], next_cursor: "" } : { items: [] } }),
      WorkspaceFrame: (p: any) => React.createElement("div", null, p.children), AdminPageHeader: () => null,
      Icon: () => null, CustomerTagManager: () => null,
      Link: (p: any) => React.createElement("a", p, p.children),
    });
    const store = { id: "abcdef11-1111-4111-8111-111111111111", name: "Synthetic store", permissions: ["customers:read", ...(permission ? ["customers:privacy"] : [])] };
    const html = renderToStaticMarkup(React.createElement(Customers, { locale: "en", stores: [store], store, q: "", after: "", initialError: null, renderKey: "key" }));
    assert.equal(html.includes('data-testid="customers-import"'), permission);
    if (permission) assert.ok(html.includes(`/en/customers/import?store=${store.id}`));
  }
});
test("the actual detail mounts archive only for an active imported customer and passes its fenced identity", () => {
  const mounts: any[] = [];
  const Body = component("../../apps/admin/components/CustomerDetail.tsx", "Body", {
    useState: (value: unknown) => [value, () => {}], useRef: (value: unknown) => ({ current: value }),
    ordersCopy: { en: { statuses: {} } }, consentPairs: [], money: () => "NT$0", displayTime: () => "stamp",
    CustomerTags: () => null, EraseDialog: () => null, Badge: (p: any) => React.createElement("span", null, p.children),
    CustomerHistoricalOrders: (p: any) => { mounts.push(p); return React.createElement("div", null, "private archive"); },
  });
  const detail = { customer_id: "customer", currency: "TWD", orders_count: 0, paid_orders_count: 0,
    captured_minor: 0, refunded_minor: 0, claims_count: 0, orders: [], claims: [], consent_history: [], privacy_actions: [] };
  for (const [active, imported] of [[true, true], [true, false], [false, true]]) {
    mounts.length = 0;
    const html = renderToStaticMarkup(React.createElement(Body, { detail: { ...detail, active, imported }, store: "store", storeInfo: { id: "store" },
      boundary: "fenced-session", refresh: async () => true, locale: "en", c: customersCopy.en }));
    assert.equal(mounts.length, active && imported ? 1 : 0);
    assert.equal(html.includes("private archive"), active && imported);
    if (mounts.length) for (const [key, value] of Object.entries({ locale: "en", store: "store", customer: "customer", boundary: "fenced-session" })) assert.equal(mounts[0][key], value);
  }
});
