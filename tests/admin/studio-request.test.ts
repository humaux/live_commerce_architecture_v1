import assert from "node:assert/strict";
import { test } from "node:test";
import { validStudioQuery } from "../../apps/admin/lib/studio-request.ts";

const base = "http://127.0.0.1:3100/api/stores/11111111-1111-4111-8111-111111111111/live-sessions";

test("Studio collection accepts only canonical raw pagination syntax", () => {
  for (const query of ["", "?limit=1", "?limit=100&cursor=A_-"]) {
    assert.equal(validStudioQuery(base + query, true), true, query);
  }
  for (const query of [
    "?", "?limit=01", "?limit=101", "?limit=0", "?limit=1&limit=2",
    "?li%6dit=1", "?limit=%31", "?cursor=A+B", "?cursor=%2541",
    "?limit=1&", "?limit=1&&cursor=A", "?unknown=1", "?limit=",
    `?cursor=${"A".repeat(1025)}`,
  ]) {
    assert.equal(validStudioQuery(base + query, true), false, query);
  }
});

test("Studio detail and rehearsal actions reject every query", () => {
  const detail = base + "/22222222-2222-4222-8222-222222222222";
  for (const path of [detail, detail + "/rehearsal/start", detail + "/rehearsal/stop"]) {
    assert.equal(validStudioQuery(path, false), true, path);
    assert.equal(validStudioQuery(path + "?", false), false, path);
    assert.equal(validStudioQuery(path + "?limit=1", false), false, path);
  }
});
