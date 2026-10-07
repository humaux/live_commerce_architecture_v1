// w4-u1-payment-activation-ui: BFF request grammar for card payments + settlements
// (apps/admin/lib/card-payments-request.ts) fencing Go internal/httpapi/payment_card.go (cvsRoute, keyed=false:
// the PUT is CAS-guarded by expected_version, so an Idempotency-Key is refused) and settlement reads.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import {
  cardPaymentsRoute,
  validCardPaymentsBody,
  validCardPaymentsQuery,
  validCardPaymentsRequest,
} from "../../apps/admin/lib/card-payments-request.ts";

const store = "11111111-2222-3333-4444-555555555555";
const url = (path: string) => `https://admin.example.com/api/stores/${store}/${path}`;
const get = (path: string, headers: Record<string, string> = {}) =>
  new Request(url(path), { method: "GET", headers });
const put = (path: string, body: string, headers: Record<string, string> = {}) =>
  new Request(url(path), {
    method: "PUT",
    headers: { "content-type": "application/json", ...headers },
    body,
  });

test("route table: exactly the four resources, nothing generic", () => {
  assert.equal(cardPaymentsRoute("GET", "payments/card"), "card-read");
  assert.equal(cardPaymentsRoute("PUT", "payments/card"), "card-write");
  assert.equal(cardPaymentsRoute("GET", "settlements"), "settlements");
  assert.equal(cardPaymentsRoute("GET", `settlements/${store}`), "settlement");
  for (const [method, path] of [
    ["POST", "payments/card"],
    ["GET", "payments/card/history"],
    ["PUT", "settlements"],
    ["POST", "settlements"],
    ["GET", "settlements/not-a-uuid"],
    ["GET", "settlements/11111111-2222-3333-4444-555555555555/lines"],
    ["DELETE", "payments/card"],
    ["GET", "payments"],
  ] as const)
    assert.equal(cardPaymentsRoute(method, path), null, `${method} ${path}`);
});

test("query grammar: card and settlement-detail are exact; settlements takes before/limit only", () => {
  assert.equal(validCardPaymentsQuery("card-read", url("payments/card")), true);
  assert.equal(validCardPaymentsQuery("card-read", url("payments/card?")), false);
  assert.equal(validCardPaymentsQuery("card-read", url("payments/card?x=1")), false);
  assert.equal(validCardPaymentsQuery("settlement", url(`settlements/${store}`)), true);
  assert.equal(validCardPaymentsQuery("settlement", url(`settlements/${store}?before=2026-09-01`)), false);
  assert.equal(validCardPaymentsQuery("settlements", url("settlements")), true);
  assert.equal(validCardPaymentsQuery("settlements", url("settlements?before=2026-09-01")), true);
  assert.equal(validCardPaymentsQuery("settlements", url("settlements?limit=52")), true);
  assert.equal(validCardPaymentsQuery("settlements", url("settlements?limit=1&before=2026-02-29")), false); // not a day
  assert.equal(validCardPaymentsQuery("settlements", url("settlements?before=2026-02-28&limit=12")), true);
  for (const bad of [
    "settlements?before=2026-9-1",
    "settlements?before=20260901",
    "settlements?limit=0",
    "settlements?limit=53",
    "settlements?limit=052",
    "settlements?limit=1&limit=2",
    "settlements?before=2026-09-01&before=2026-09-02",
    "settlements?after=2026-09-01",
    "settlements?limit=",
    "settlements?",
  ])
    assert.equal(validCardPaymentsQuery("settlements", url(bad)), false, bad);
});

test("request fence: keyless PUT (CAS), bodyless GETs, no transfer-encoding", () => {
  const body = JSON.stringify({
    enabled: true,
    terms_version: "pf-2026-10",
    descriptor_suffix: null,
    expected_version: 0,
  });
  assert.equal(validCardPaymentsRequest("card-write", put("payments/card", body)), true);
  // Go cvsRoute keyed=false: an Idempotency-Key on the PUT is a client bug, refuse it.
  assert.equal(
    validCardPaymentsRequest("card-write", put("payments/card", body, { "idempotency-key": "abcd1234abcd" })),
    false,
  );
  assert.equal(validCardPaymentsRequest("card-write", put("payments/card", body, { "transfer-encoding": "chunked" })), false);
  assert.equal(
    validCardPaymentsRequest(
      "card-write",
      new Request(url("payments/card"), { method: "PUT", headers: { "content-type": "text/plain" }, body }),
    ),
    false,
  );
  assert.equal(validCardPaymentsRequest("card-read", get("payments/card")), true);
  assert.equal(validCardPaymentsRequest("card-read", get("payments/card", { "idempotency-key": "abcd1234" })), false);
  assert.equal(validCardPaymentsRequest("settlements", get("settlements?limit=10")), true);
  assert.equal(validCardPaymentsRequest("settlement", get(`settlements/${store}`)), true);
});

test("PUT body is the exact frozen shape {enabled, terms_version, descriptor_suffix, expected_version}", () => {
  const good = (patch: Record<string, unknown> = {}) =>
    JSON.stringify({
      enabled: true,
      terms_version: "pf-2026-10",
      descriptor_suffix: null,
      expected_version: 0,
      ...patch,
    });
  assert.equal(validCardPaymentsBody(good()), true);
  assert.equal(validCardPaymentsBody(good({ enabled: false })), true);
  assert.equal(validCardPaymentsBody(good({ descriptor_suffix: "SHOP" })), true);
  assert.equal(validCardPaymentsBody(good({ expected_version: 41 })), true);
  for (const bad of [
    good({ extra: 1 }),
    good({ enabled: "true" }),
    good({ terms_version: "" }),
    good({ terms_version: null }),
    good({ terms_version: "x".repeat(65) }),
    good({ descriptor_suffix: undefined }),
    good({ descriptor_suffix: "12" }),
    good({ descriptor_suffix: "TOOLONGSUFFIX" }),
    good({ descriptor_suffix: "BAD_SUFFIX" }),
    good({ expected_version: -1 }),
    good({ expected_version: 1.5 }),
    good({ expected_version: "3" }),
    "{}",
    "not json",
    "[]",
    "null",
  ])
    assert.equal(validCardPaymentsBody(bad), false, bad);
  // descriptor_suffix is present-but-nullable: the key may not be dropped.
  const missing = JSON.stringify({ enabled: true, terms_version: "pf-2026-10", expected_version: 0 });
  assert.equal(validCardPaymentsBody(missing), false);
});

// route.ts cannot be imported in plain Node (the `@/` alias), so its wiring of the grammar above is pinned in source.
test("the BFF route wires the card grammar: keyless CAS PUT, exact body, no-store, auth required", () => {
  const source = readFileSync("apps/admin/app/api/stores/[store]/[...resource]/route.ts", "utf8");
  assert.match(source, /cardPaymentsRoute\(request\.method, path\)/);
  assert.match(source, /cardPayments && !validCardPaymentsRequest\(cardPayments, request\)/);
  assert.match(source, /cardPayments === "card-write" && !validCardPaymentsBody\(/);
  // the PUT is the only keyless card command, and its key is never forwarded to Go (cvsRoute keyed=false refuses one)
  assert.match(source, /const keyless = [^;]*cardPayments === "card-write"/);
  // reads/writes need a real session (no shared fixture bearer) and answer private, no-store
  assert.match(source, /\|\| cardPayments \|\| logistic \|\| accountRoute \|\| ads \|\| discoveryRoute\.test\(path\)\) && !authConfig/);
  assert.match(source, /order \|\| orderSearch \|\| action \|\| customers \|\| cardPayments \|\| logistic \|\| storefront[^?]*\? "private, no-store"/);
  // the generic tables name only these paths for the card resources
  assert.match(source, /payments\/card\|settlements\|settlements\/\$\{uuid\}\)\$`/);
  assert.match(source, /PUT: new RegExp\(`[^`]*\|payments\/card\)\$`\)/);
});
