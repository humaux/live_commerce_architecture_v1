import assert from "node:assert/strict";
import { test } from "node:test";
import { validOrdersQuery } from "../../apps/admin/lib/orders-request.ts";

const collection =
  "http://127.0.0.1:3100/api/stores/11111111-1111-4111-8111-111111111111/orders";

test("raw merchant order query accepts only canonical collection fields", () => {
  for (const query of ["", "?limit=1", "?limit=100&state=all&cursor=A_-"]) {
    assert.equal(validOrdersQuery(collection + query, false), true, query);
  }
  for (const query of [
    "?",
    "?limit=1&",
    "?limit=1&&state=all",
    "?limit=%31",
    "?%6cimit=1",
    "?limit=01",
    "?limit=101",
    "?limit=1&limit=2",
    "?cursor=A+B",
    "?cursor=%2541",
    "?state=draft",
    "?__next=1",
  ]) {
    assert.equal(validOrdersQuery(collection + query, false), false, query);
  }
});

test("merchant order detail rejects every query, including bare question mark", () => {
  const detail = `${collection}/22222222-2222-4222-8222-222222222222`;
  assert.equal(validOrdersQuery(detail, true), true);
  assert.equal(validOrdersQuery(detail + "?", true), false);
  assert.equal(validOrdersQuery(detail + "?limit=1", true), false);
});
