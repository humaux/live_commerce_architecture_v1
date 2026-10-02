import assert from "node:assert/strict";
import { test } from "node:test";
import {
  parseDomainRequest,
  parseStorefront,
  parseStorefrontDomains,
} from "../../apps/admin/lib/storefront-model.ts";

// Synthetic values only. Contract: published-storefront-resolver-v1 "Writer (R3)" (Go storefrontadmin.State).
const domain = { origin: "https://shop.example.com", valid_until: "2027-01-01T00:00:00Z", serving: true };

test("parses never-published, published and bound states", () => {
  assert.deepEqual(parseStorefront({ published: false, version: 0, domains: [] }), { published: false, version: 0, domains: [] });
  assert.deepEqual(parseStorefront({ published: true, version: 2, domains: [domain] }).domains, [domain]);
  assert.equal(parseStorefront({ published: false, version: 3, domains: [{ ...domain, serving: false }] }).domains[0].serving, false);
});

test("rejects anything outside the closed shape", () => {
  const bad: unknown[] = [
    null, [], "x", {},
    { published: true, version: 0, domains: [] }, // published with version 0 is impossible
    { published: "yes", version: 1, domains: [] },
    { published: false, version: -1, domains: [] },
    { published: false, version: 1.5, domains: [] },
    { published: false, version: 1 },
    { published: false, version: 1, domains: [], extra: 1 },
    { published: false, version: 1, domains: [{ ...domain, extra: 1 }] },
    { published: false, version: 1, domains: [{ origin: domain.origin, valid_until: domain.valid_until }] },
    { published: false, version: 1, domains: [{ ...domain, origin: "http://shop.example.com" }] },
    { published: false, version: 1, domains: [{ ...domain, origin: "https://shop.example.com/" }] },
    { published: false, version: 1, domains: [{ ...domain, origin: "https://shop.example.com:443" }] },
    { published: false, version: 1, domains: [{ ...domain, origin: "https://Shop.example.com" }] },
    { published: false, version: 1, domains: [{ ...domain, origin: "https://javascript:alert(1).example.com" }] },
    { published: false, version: 1, domains: [{ ...domain, valid_until: "tomorrow" }] },
    { published: false, version: 1, domains: [{ ...domain, serving: "true" }] },
  ];
  for (const value of bad) assert.throws(() => parseStorefront(value), /storefront_shape/, JSON.stringify(value));
});

// R5 store-domains (Decision 3): the merchant domain read + write shapes (Go storefrontdomains.Read/Request).
const token = "A".repeat(43);
const domainRow = { origin: "https://shop.example.com", state: "REQUESTED", version: 1, token, verify_deadline: "2027-01-01T00:00:00Z", serving: false };
const domainRequest = {
  domain_id: "11111111-1111-4111-8111-111111111111",
  version: 1,
  state: "REQUESTED",
  origin: "https://shop.example.com",
  dns: { txt_name: "_lc-verify.shop.example.com", txt_value: token, cname_target: "stores.xgdwm.com", apex: false },
};

test("parses the domain list and the one-time request instructions", () => {
  assert.deepEqual(parseStorefrontDomains({ domains: [domainRow] }), { domains: [domainRow] });
  assert.equal(
    parseStorefrontDomains({ domains: [{ ...domainRow, state: "ACTIVE", token: null, verify_deadline: null, serving: true }] }).domains[0].serving,
    true,
  );
  assert.deepEqual(parseDomainRequest(domainRequest), domainRequest);
  assert.equal(parseDomainRequest({ ...domainRequest, dns: { ...domainRequest.dns, apex: true } }).dns.apex, true);
});

test("rejects domain shapes outside the closed contract", () => {
  const badDomains: unknown[] = [
    null, [], "x", {},
    { domains: [{ origin: domainRow.origin, state: domainRow.state, version: domainRow.version }] }, // missing token/verify_deadline/serving keys on the row
    { domains: [{ ...domainRow, extra: 1 }] },
    { domains: [{ ...domainRow, state: "BROKEN" }] },
    { domains: [{ ...domainRow, version: 0 }] },
    { domains: [{ ...domainRow, origin: "http://shop.example.com" }] },
    { domains: [{ ...domainRow, token: "short" }] },
    { domains: [{ ...domainRow, verify_deadline: "tomorrow" }] },
    { domains: [{ ...domainRow, serving: "true" }] },
    { domains: Array.from({ length: 101 }, () => domainRow) },
  ];
  for (const value of badDomains) assert.throws(() => parseStorefrontDomains(value), /storefront_shape/, JSON.stringify(value));
  const badRequests: unknown[] = [
    null, [], "x", {},
    { ...domainRequest, extra: 1 },
    { ...domainRequest, domain_id: "not-a-uuid" },
    { ...domainRequest, version: 0 },
    { ...domainRequest, state: "ACTIVE" },
    { ...domainRequest, origin: "https://Shop.example.com" },
    { ...domainRequest, dns: { ...domainRequest.dns, txt_name: "verify.shop.example.com" } },
    { ...domainRequest, dns: { ...domainRequest.dns, txt_value: "short" } },
    { ...domainRequest, dns: { ...domainRequest.dns, cname_target: 7 } },
    { ...domainRequest, dns: { ...domainRequest.dns, apex: "true" } },
  ];
  for (const value of badRequests) assert.throws(() => parseDomainRequest(value), /storefront_shape/, JSON.stringify(value));
});
