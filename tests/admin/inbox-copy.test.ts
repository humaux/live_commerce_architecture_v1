// Purpose: verify fixed inbox errors are translated without exposing upstream diagnostic text.
// Depends on: real inbox copy/error mapper and Node test/assert.
// Used by: LC-U2b lightweight acceptance; Japanese route availability is a separate shared-locale gap.
import test from "node:test";
import assert from "node:assert/strict";
import {
  inboxCopy,
  inboxError,
} from "../../apps/admin/src/features/messages/copy.ts";

test("three unit languages cover identical fixed labels and delivery states", () => {
  const keys = Object.keys(inboxCopy("en"));
  for (const locale of ["zh-TW", "en", "ja"]) {
    const copy = inboxCopy(locale);
    assert.deepEqual(Object.keys(copy).sort(), [...keys].sort());
    assert.ok(
      Object.values(copy).every(
        (value) => typeof value === "string" && value.length > 0,
      ),
    );
    for (const code of [
      "window_closed",
      "takeover_changed",
      "capability",
      "conversation_gone",
      "duplicate_recent",
      "invalid_text",
      "rate_limited",
      "version_conflict",
      "human_takeover",
      "auto_pending_confirm",
      "session_changed",
    ]) {
      assert.ok(inboxError(locale, code).length > 0);
    }
    assert.equal(
      inboxError(locale, "MOCK_PRIVATE_DIAGNOSTIC"),
      copy.unavailable,
    );
    assert.ok(
      !inboxError(locale, "MOCK_PRIVATE_DIAGNOSTIC").includes(
        "MOCK_PRIVATE_DIAGNOSTIC",
      ),
    );
  }
  assert.notEqual(inboxCopy("ja").uncertain, inboxCopy("en").uncertain);
});
