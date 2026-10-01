import assert from "node:assert/strict";
import { test } from "node:test";
import { parseStorefront } from "../../apps/admin/lib/storefront-model.ts";

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
