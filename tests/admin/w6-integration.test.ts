// Purpose: preserve selected tag scope across customer navigation and render real list rows with tags/import origin.
// Depends on: Node test/assert/vm, typescript-api, actual Customers row/URL source and current customer copy.
// Used by: W6-U1 focused regression and Node CI; does not replace real-click browser acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { customersCopy } from "../../apps/admin/lib/customers-copy.ts";
import { routes, visibleGroups } from "../../apps/admin/src/routes.ts";
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
  const leaf = readFileSync(new URL("../../apps/admin/app/api/stores/[store]/customers/route.ts", import.meta.url), "utf8");
  const calls: any[] = [];
  const { GET } = compiled(leaf, { require: (name: string) => {
    if (name === "@/lib/customer-tags-bff") return { customerTagsBFF: async (...args: any[]) => { calls.push(["tags", ...args]); return new Response("tags"); } };
    if (name === "../[...resource]/route") return { GET: async (request: Request, context: any) => { calls.push(["legacy", request, await context.params]); return new Response("legacy"); } };
    throw new Error("unexpected import " + name);
  }, URL, Response });
  for (const query of ["", "?q=Buyer&after=cursor", "?tag=bad", "?tag=&tag=another", "?tag=bad&tenant_id=attacker"]) {
    const request = new Request("https://admin.example.invalid/api/stores/store/customers" + query);
    const response = await GET(request, { params: Promise.resolve({ store: "store" }) });
    const tagged = query.includes("tag=");
    assert.equal(await response.text(), tagged ? "tags" : "legacy");
    const call = calls.at(-1)!;
    assert.equal(call[1], request);
    if (tagged) assert.deepEqual(call.slice(2), ["store", "customers"]);
    else assert.deepEqual(JSON.parse(JSON.stringify(call[2])), { store: "store", resource: ["customers"] });
  }
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
