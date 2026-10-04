import assert from "node:assert/strict";
import test from "node:test";
import { validStorefrontReceipt } from "../../apps/admin/lib/storefront-handle.ts";
import { parseDomainRequest, parseStorefrontDomains } from "../../apps/admin/lib/storefront-model.ts";
import { parseDomainCommand } from "../../apps/admin/lib/storefront-command.ts";

test("created address is a strict HTTPS origin matching the assigned handle", () => {
  assert(validStorefrontReceipt("12345678", "https://12345678.platform.example"));
  // Operator handles can still appear on replay after handle-set; this grammar is not removed.
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

test("domain kind is authoritative and closed, not inferred from host or token", () => {
  const row = { origin: "https://shop.example.com", kind: "platform", state: "ACTIVE", version: 1, token: null, verify_deadline: null, serving: true };
  assert.equal(parseStorefrontDomains({ domains: [row] }).domains[0].kind, "platform");
  assert.equal(parseStorefrontDomains({ domains: [{ ...row, kind: "custom" }] }).domains[0].kind, "custom");
  for (const kind of [undefined, null, "PLATFORM", "other"]) assert.throws(() => parseStorefrontDomains({ domains: [{ ...row, kind }] }));
});

test("apex DNS retains every concrete IPv4/IPv6 address and rejects malformed values", () => {
  const result = { domain_id: "11111111-1111-4111-8111-111111111111", version: 1, state: "REQUESTED", origin: "https://example.com", dns: { txt_name: "_lc-verify.example.com", txt_value: "A".repeat(43), cname_target: "stores.platform.example", apex: true, edge_addresses: ["203.0.113.10", "203.0.113.11", "2001:db8::10"] } };
  assert.deepEqual(parseDomainRequest(result).dns.edge_addresses, result.dns.edge_addresses);
  for (const edge_addresses of [null, "203.0.113.10", ["stores.platform.example"], ["999.1.1.1"], ["01.1.1.1"], ["[2001:db8::1]"], ["2001:db8::1/64"], ["2001:db8::1%en0"], [""], [4]]) assert.throws(() => parseDomainRequest({ ...result, dns: { ...result.dns, edge_addresses } }));
  assert.throws(() => parseDomainRequest({ ...result, dns: { ...result.dns, apex: false } }));
});

test("pending domain journal preserves exact action/key/target; legacy or corrupt markers never replay", () => {
  const key = "storefront-domain-11111111-1111-4111-8111-111111111111";
  for (const command of [{ version: 1, key, action: "request", hostname: "shop.example.com" }, ...["suspend", "detach"].map(action => ({ version: 1, key, action, origin: "https://shop.example.com" }))]) {
    assert.deepEqual(parseDomainCommand(JSON.stringify(command)), command);
    assert.equal(parseDomainCommand(JSON.stringify({ ...command, extra: "x" })), null);
    assert.equal(parseDomainCommand(JSON.stringify({ ...command, key: "" })), null);
    assert.equal(parseDomainCommand(JSON.stringify({ ...command, version: 0 })), null);
  }
  for (const raw of [null, "pending", "{}", "null", JSON.stringify({ version: 1, key, action: "request", hostname: "https://shop.example.com" }), JSON.stringify({ version: 1, key, action: "detach", origin: "https://shop.example.com/?x=1" })]) assert.equal(parseDomainCommand(raw), null);
});
