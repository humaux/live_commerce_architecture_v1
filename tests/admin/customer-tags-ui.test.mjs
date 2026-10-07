// Purpose: render actual tag/note components in Node to prove DTO-based SSR, permissions and escaping.
// Depends on: React SSR, CustomerTags components and focused in-memory TSX loader.
// Used by: W6-U1 local MOCK evidence; these renders never substitute for browser click gates.
import assert from "node:assert/strict";
import { test } from "node:test";
import { createRequire } from "node:module";
import { registerCustomerTagsLoader } from "./customer-tags-node-loader.mjs";
registerCustomerTagsLoader();
const require = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const { createElement } = require("react");
const { renderToStaticMarkup } = require("react-dom/server");
const { TagBadges, CustomerTags, CustomerTagManager } = await import("../../apps/admin/components/CustomerTags.tsx");
const id = "abcdef11-1111-4111-8111-111111111111";
const detail = { customer_id: id, active: true, tags: [{ id, name: "<VIP>", color: "blue" }], tags_revision: "a".repeat(64),
  notes: [{ id, body: "<script>synthetic</script>\nline two", author_id: id, created_at: "2026-10-07T00:00:00Z", edited_at: null, version: 1, own: false }] };
const store = { id, name: "Synthetic", currency: "TWD", permissions: ["customers:read"] };
const props = { locale: "en", store, detail, boundary: "b".repeat(64), onChanged: async () => true };

test("badges are escaped server tag facts with human-readable names", () => {
  const html = renderToStaticMarkup(createElement(TagBadges, { tags: detail.tags }));
  assert.match(html, /&lt;VIP&gt;/); assert.match(html, /ct-blue/); assert.doesNotMatch(html, /abcdef11/);
});
test("actual customer notes render privacy notice, safe content, author fallback and read-only permissions", () => {
  const html = renderToStaticMarkup(createElement(CustomerTags, props));
  assert.match(html, /data-testid="customer-notes"/); assert.match(html, /data-testid="note-privacy-hint"/);
  assert.match(html, /buyer can see them/); assert.match(html, /Staff member \(name unavailable\)/);
  assert.match(html, /&lt;script&gt;/); assert.doesNotMatch(html, /<script>|<textarea|abcdef11/);
});
test("unknown permissions and erased customer never offer note mutations", () => {
  for (const override of [{ store: { ...store, permissions: undefined } }, { detail: { ...detail, active: false }, store: { ...store, permissions: ["customers:write", "customers:privacy"] } }]) {
    const html = renderToStaticMarkup(createElement(CustomerTags, { ...props, ...override }));
    assert.doesNotMatch(html, /<textarea|Save tags/);
  }
});
test("write-only notes permit add but not edit/delete of notes the server does not mark own; privacy permission permits both", () => {
  const writer = renderToStaticMarkup(createElement(CustomerTags, { ...props, store: { ...store, permissions: ["customers:read", "customers:write"] } }));
  assert.match(writer, /<textarea/); assert.match(writer, /earlier notes remain read-only/i);
  assert.doesNotMatch(writer, />Edit<|>Delete</);
  const privacy = renderToStaticMarkup(createElement(CustomerTags, { ...props, store: { ...store, permissions: ["customers:read", "customers:write", "customers:privacy"] } }));
  assert.match(privacy, />Edit</); assert.match(privacy, />Delete</);
});
test("manager trigger and all three localized buyer-export notices render through actual components", () => {
  assert.match(renderToStaticMarkup(createElement(CustomerTagManager, props)), /Manage tags/);
  for (const locale of ["en", "zh-CN", "zh-TW"]) {
    const html = renderToStaticMarkup(createElement(CustomerTags, { ...props, locale }));
    assert.match(html, /data-testid="note-privacy-hint"/);
    assert.ok(html.includes(locale === "en" ? "buyer can see them" : locale === "zh-TW" ? "買家申請匯出資料時也看得到" : "买家申请导出数据时也能看到"));
  }
});
