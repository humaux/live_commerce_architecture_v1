// Purpose: protect customer filter dispatch/navigation/import badges and immediate private-subtree removal after erasure.
// Depends on: Node test/assert/vm, typescript-api, actual Customers row/URL and CustomerDetail Body source, shell registry/current copy,
// and w6-real-route-loader (real route/auth/backend boundary; only upstream fetch is faked).
// Used by: W6-U1 focused regression and Node CI; does not replace real-click browser acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { canSeeReports } from "../../apps/admin/lib/customers-model.ts";
import { customersCopy } from "../../apps/admin/lib/customers-copy.ts";
import { routes, visibleGroups } from "../../apps/admin/src/routes.ts";
import { w6RealBoundary, W6_TAG } from "./w6-real-route-loader.mjs";
const source = readFileSync(new URL("../../apps/admin/components/Customers.tsx", import.meta.url), "utf8");
const appRequire = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const React = appRequire("react"), { renderToStaticMarkup } = appRequire("react-dom/server");
function compiled(fragment: string, globals: Record<string, unknown> = {}) {
  const exports: Record<string, any> = {};
  runInNewContext(ts.transpileModule(fragment, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText,
    { exports, require: appRequire, URLSearchParams, ...globals });
  return exports;
}
test("selected tag survives searches and cursor pagination while an empty filter is omitted", () => {
  const match = source.match(/function url\([\s\S]*?\n\}/)?.[0];
  assert.ok(match);
  const { url } = compiled("export " + match);
  const tag = "11111111-1111-4111-8111-111111111111";
  for (const [q, after] of [["Buyer", ""], ["", "opaque_cursor"]]) {
    const address = new URL(url("zh-TW", "store", q, after, tag), "https://admin.example.invalid");
    assert.equal(address.searchParams.get("tag"), tag);
    assert.equal(address.searchParams.get("q") ?? "", q);
    assert.equal(address.searchParams.get("after") ?? "", after);
  }
  assert.equal(new URL(url("en", "store", "", "", ""), "https://admin.example.invalid").searchParams.has("tag"), false);
});
test("customer list dispatch preserves legacy requests and sends every tag query to the closed W6 authority", async () => {
  const boundary = await w6RealBoundary();
  try {
    // Preserve every old query case and its dispatch intent; prove the actual authority/status rather than fake string bodies.
    for (const query of ["", "?q=Buyer&after=cursor", "?tag=bad", "?tag=&tag=another", "?tag=bad&tenant_id=attacker", `?tag=${W6_TAG}`]) {
      boundary.calls.length = 0;
      const response = await boundary.customers.GET(boundary.request(`customers${query}`), boundary.customerContext);
      const valid = query === "" || query.startsWith("?q=") || query === `?tag=${W6_TAG}`;
      assert.equal(response.status, valid ? 200 : 422, query);
      if (valid) {
        assert.deepEqual(await response.json(), { items: [], next_cursor: "" });
        assert.equal(boundary.calls.length, 2);
        assert.equal(boundary.calls[1].url.search, query);
        for (const call of boundary.calls) assert.equal(call.headers.get("cookie"), null);
      } else { assert.equal((await response.json()).code, "invalid_request"); assert.equal(boundary.calls.length, 0); }
    }
  } finally { boundary.restore(); }
});
// Codex review P2 (PR #3): a legacy Store without a permission list must still reach reports (the BFF/Go authorize).
test("customers reports link is hidden only when permissions are present and lack orders:read", () => {
  assert.equal(canSeeReports({ permissions: undefined }), true);
  assert.equal(canSeeReports({}), true);
  assert.equal(canSeeReports({ permissions: ["orders:read"] }), true);
  assert.equal(canSeeReports({ permissions: ["customers:read"] }), false);
  assert.equal(canSeeReports({ permissions: [] }), false);
  assert.equal(canSeeReports(null), false);
  assert.match(source, /store && canSeeReports\(store\) && <Link[^>]*customers-reports/);
});
test("reports are a finance subpage and preserve the existing single finance navigation entry", () => {
  const finance = routes.filter((route) => route.group === "finance");
  assert.equal(finance.filter((route) => route.nav).length, 1);
  assert.equal(finance.find((route) => route.id === "reports")?.nav, false);
  assert.ok(visibleGroups({ permissions: ["orders:read"], role: "owner" }).some((group) => group.id === "finance"));
});
test("an authoritative erased customer immediately unmounts the cached private tags/notes subtree", () => {
  const detailSource = readFileSync(new URL("../../apps/admin/components/CustomerDetail.tsx", import.meta.url), "utf8");
  const tree = ts.createSourceFile("CustomerDetail.tsx", detailSource, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const declaration = tree.statements.find((statement) => ts.isFunctionDeclaration(statement) && statement.name?.text === "Body");
  assert.ok(declaration);
  let mounts = 0;
  const { Body } = compiled("export " + declaration.getText(tree), {
    useState: (value: unknown) => [value, () => {}], useRef: (value: unknown) => ({ current: value }),
    ordersCopy: { en: { statuses: {} } }, consentPairs: [],
    money: () => "NT$0", displayTime: () => "10/07/2026",
    CustomerTags: () => { mounts++; return React.createElement("div", null, "cached private note"); },
    EraseDialog: () => null,
  });
  const detail = { customer_id: "customer", currency: "TWD", first_seen_at: "2026-10-07T00:00:00Z", last_activity_at: "2026-10-07T00:00:00Z",
    orders_count: 0, paid_orders_count: 0, captured_minor: 0, refunded_minor: 0, claims_count: 0,
    orders: [], claims: [], consents: {}, consent_history: [], privacy_actions: [], notes: [{ body: "cached private note" }] };
  const props = { store: "store", storeInfo: { id: "store" }, boundary: "session", refresh: async () => true, locale: "en", c: customersCopy.en };
  const active = renderToStaticMarkup(React.createElement(Body, { ...props, detail: { ...detail, active: true } }));
  assert.equal(mounts, 1); assert.ok(active.includes("cached private note"));
  mounts = 0;
  const erased = renderToStaticMarkup(React.createElement(Body, { ...props, detail: { ...detail, active: false } }));
  assert.equal(mounts, 0); assert.equal(erased.includes("cached private note"), false);
  assert.ok(erased.includes(customersCopy.en.erasedState));
});
test("the actual customer row shows manual tags and imported status without changing consent", () => {
  const match = source.match(/function CustomerRow\([\s\S]*?\n\}/)?.[0];
  assert.ok(match);
  const { CustomerRow } = compiled("export " + match, {
    Link: (p: any) => React.createElement("a", { href: p.href }, p.children),
    Badge: (p: any) => React.createElement("span", null, p.children),
    TagBadges: ({ tags }: any) => React.createElement("span", null, tags.map((t: any) => t.name).join(" · ")),
    ConsentChip: () => React.createElement("span", null, "permission unchanged"),
    detailHref: (_locale: string, _store: string, id: string) => "/customers/" + id,
    money: () => "NT$100", displayTime: () => "10/07/2026",
  });
  const row = { customer_id: "22222222-2222-4222-8222-222222222222", display_name: "Synthetic Buyer", phone_last3: null,
    active: true, imported: true, tags: [{ id: "33333333-3333-4333-8333-333333333333", name: "VIP", color: "blue" }],
    currency: "TWD", captured_minor: 10000, refunded_minor: 0, orders_count: 1, paid_orders_count: 1,
    claims_count: 0, platforms: [], consents: { marketing_messages: false, ads_personalization: false }, last_activity_at: "2026-10-07T00:00:00Z" };
  for (const [locale, imported] of [["zh-TW", "已匯入"], ["zh-CN", "已导入"], ["en", "Imported"]] as const) {
    const html = renderToStaticMarkup(React.createElement(CustomerRow, { row, c: customersCopy[locale], locale, store: "store" }));
    assert.ok(html.includes("VIP")); assert.ok(html.includes(imported));
    const plain = renderToStaticMarkup(React.createElement(CustomerRow, { row: { ...row, imported: false }, c: customersCopy[locale], locale, store: "store" }));
    assert.equal(plain.includes(imported), false);
    assert.deepEqual(row.consents, { marketing_messages: false, ads_personalization: false });
  }
});
