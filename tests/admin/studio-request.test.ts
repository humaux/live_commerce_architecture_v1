import assert from "node:assert/strict";
import { test } from "node:test";
import { validStudioInputToken, validStudioQuery } from "../../apps/admin/lib/studio-request.ts";

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
  for (const path of [detail, detail + "/rehearsal/start", detail + "/rehearsal/stop", detail + "/input/start", detail + "/input/token", detail + "/input", detail + "/input/prepared"]) {
    assert.equal(validStudioQuery(path, false), true, path);
    assert.equal(validStudioQuery(path + "?", false), false, path);
    assert.equal(validStudioQuery(path + "?limit=1", false), false, path);
  }
});

test("Studio token transport accepts only its exact local six-field DTO", () => {
  const valid = {
    attempt_id: "22222222-2222-4222-8222-222222222222",
    room_name: "lc_22222222222242228222222222222222",
    publisher_identity: "lcp_33333333333343338333333333333333",
    url: "wss://127.0.0.1:7880", token: "fixture.payload.signature", expires_at: 2000000000,
  };
  assert.equal(validStudioInputToken(valid), true);
  for (const url of ["ws://127.0.0.1:1/", "wss://[::1]:65535"]) {
    assert.equal(validStudioInputToken({ ...valid, url }), true);
  }
  for (const value of [null, [], {}, { ...valid, secret: "forbidden" },
    ...Object.keys(valid).map((key) => Object.fromEntries(Object.entries(valid).filter(([k]) => k !== key))),
    { ...valid, attempt_id: "00000000-0000-0000-0000-000000000000" },
    { ...valid, room_name: "lc_00000000000000000000000000000000" },
    { ...valid, publisher_identity: "arbitrary" }, { ...valid, token: "not-a-jwt" },
    ...[0, -1, 1.2, Number.MAX_SAFE_INTEGER + 1, "2000000000"].map((expires_at) => ({ ...valid, expires_at })),
    ...["wss://example.com:7880", "wss://127.0.0.1:0", "wss://127.0.0.1:65536", "wss://127.0.0.1:07880",
      "https://127.0.0.1:7880", "wss://user@127.0.0.1:7880", "wss://127.0.0.1:7880?token=x",
      "wss://127.0.0.1:7880#x", "wss://127.0.0.1:7880/path"].map((url) => ({ ...valid, url })),
  ]) assert.equal(validStudioInputToken(value), false);
});
