import assert from "node:assert/strict";
import test from "node:test";
import { handleNameHint, parseHandleSuggestion, validStorefrontReceipt } from "../../apps/admin/lib/storefront-handle.ts";
import { parseDomainRequest } from "../../apps/admin/lib/storefront-model.ts";

test("handle suggestions explain reserved and format; fail closed on invalid responses", () => {
  assert.equal(handleNameHint("Admin"), "reserved");
  assert.equal(handleNameHint("xn--shop"), "reserved");
  assert.equal(handleNameHint("小商店"), "format");
  assert.equal(handleNameHint("Lumi Store"), null);
  assert.deepEqual(parseHandleSuggestion({ suggested: "lumi-store", available: false }), { suggested: "lumi-store", available: false });
  for (const suggested of ["admin", "xn--abc", "ab", "foo/bar", "UPPER"]) assert.equal(parseHandleSuggestion({ suggested, available: true }), null);
  assert.equal(parseHandleSuggestion({ suggested: "lumi", available: "true" }), null);
});
test("created address is a strict HTTPS origin matching the assigned handle", () => {
  assert(validStorefrontReceipt("lumi", "https://lumi.platform.example"));
  assert(validStorefrontReceipt("lumi", ""));
  for (const origin of ["https://wrong.platform.example", "https://lumi.platform.example/", "https://lumi.platform.example@evil.example", "https://lumi.platform.example?next=x", "https://lumi.platform.example:443", "javascript:alert(1)"]) assert.equal(validStorefrontReceipt("lumi", origin), false);
});
test("DNS instructions must name the requested origin and a bare hostname target", () => {
  const result = { domain_id: "11111111-1111-4111-8111-111111111111", version: 1, state: "REQUESTED", origin: "https://shop.example.com", dns: { txt_name: "_lc-verify.shop.example.com", txt_value: "A".repeat(43), cname_target: "stores.platform.example", apex: false } };
  assert.equal(parseDomainRequest(result).origin, result.origin);
  assert.throws(() => parseDomainRequest({ ...result, dns: { ...result.dns, txt_name: "_lc-verify.foreign.example" } }));
  assert.throws(() => parseDomainRequest({ ...result, dns: { ...result.dns, cname_target: "https://evil.example/path" } }));
});
