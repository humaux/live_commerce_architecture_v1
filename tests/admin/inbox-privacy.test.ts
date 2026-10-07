// Purpose: exercise the inbox request fence and reply limits without a browser or buyer data.
// Depends on: the production inbox privacy model and Node test/assert.
// Used by: LC-U2b node gate; complements real-browser hidden/stale response assertions.
import test from "node:test";
import assert from "node:assert/strict";
import {
  InboxFence,
  permitted,
  textLimit,
} from "../../apps/admin/src/features/messages/privacy.ts";

test("hiding aborts requests and rejects late responses, even after reveal", () => {
  const fence = new InboxFence();
  const old = fence.begin();
  assert.equal(fence.current(old), true);
  fence.invalidate(false);
  assert.equal(old.signal.aborted, true);
  assert.equal(fence.current(old), false);
  fence.invalidate(true);
  assert.equal(fence.current(old), false);
  assert.equal(fence.current(fence.begin()), true);
});
test("switching selection invalidates both reads and write completions", () => {
  const fence = new InboxFence();
  const read = fence.begin();
  const write = fence.begin();
  fence.invalidate(true);
  assert.equal(fence.current(read), false);
  assert.equal(fence.current(write), false);
});
test("owner and explicit inbox permissions only; viewer fails closed", () => {
  assert.equal(
    permitted(
      { id: "a", name: "s", currency: "TWD", role: "viewer" },
      "inbox:read",
    ),
    false,
  );
  assert.equal(
    permitted(
      { id: "a", name: "s", currency: "TWD", permissions: ["inbox:read"] },
      "inbox:reply",
    ),
    false,
  );
  assert.equal(
    permitted(
      { id: "a", name: "s", currency: "TWD", role: "owner" },
      "inbox:reply",
    ),
    true,
  );
});
test("Messenger counts Unicode codepoints; Instagram counts NFC UTF8 bytes", () => {
  assert.equal(textLimit("messenger", "😀".repeat(2000)).valid, true);
  assert.equal(textLimit("messenger", "😀".repeat(2001)).valid, false);
  assert.equal(textLimit("instagram", "界".repeat(334)).valid, false);
  assert.equal(textLimit("instagram", "e\u0301".repeat(500)).valid, true);
  assert.equal(textLimit("messenger", "  ").valid, false);
});

test("uncertain transport retries retain the exact submission; definitive conflicts retire it", async () => {
  const { ReplyReceipt } =
    await import("../../apps/admin/src/features/messages/privacy.ts");
  let counter = 0;
  const receipt = new ReplyReceipt(() => `mock-key-${++counter}`);
  const first = receipt.prepare({ text: "MOCK_REPLY", expected_generation: 2 });
  receipt.failed("retry_later");
  assert.equal(
    receipt.prepare({ text: "DIFFERENT", expected_generation: 99 }),
    first,
  );
  assert.deepEqual(first.body, { text: "MOCK_REPLY", expected_generation: 2 });
  receipt.failed("takeover_changed");
  assert.notEqual(
    receipt.prepare({ text: "MOCK_REPLY", expected_generation: 3 }).key,
    first.key,
  );
  receipt.clear();
  assert.equal(receipt.pending(), null);
});

test("privacy calibration cannot activate with production origin or ordinary caller env", async () => {
  const { inboxCalibration } =
    await import("../../apps/admin/src/features/messages/privacy.ts");
  const env = {
    LC_INBOX_CALIBRATION: "retain-thread",
    LC_BROWSER_INBOX_ACCEPTANCE: "1",
    COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS: "1",
  };
  assert.equal(inboxCalibration(env, "http://127.0.0.1:3100"), true);
  assert.equal(inboxCalibration(env, "https://admin.example.test"), false);
  assert.equal(
    inboxCalibration(
      { ...env, LC_BROWSER_INBOX_ACCEPTANCE: "0" },
      "http://127.0.0.1:3100",
    ),
    false,
  );
  assert.equal(
    inboxCalibration(
      { ...env, COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS: "0" },
      "http://127.0.0.1:3100",
    ),
    false,
  );
});

test("late focus/pageshow while native document remains hidden cannot reveal requests", () => {
  const fence = new InboxFence();
  fence.invalidate(false);
  assert.equal(fence.reveal("hidden"), false);
  assert.equal(fence.current(fence.begin()), false);
  assert.equal(fence.reveal("visible"), true);
  assert.equal(fence.current(fence.begin()), true);
});

test("terminal navigation revocation cannot be undone by focus or refresh until a new page fence", () => {
  const fence = new InboxFence();
  const old = fence.begin();
  fence.revoke();
  assert.equal(fence.current(old), false);
  assert.equal(fence.reveal("visible"), false);
  fence.invalidate(true);
  assert.equal(fence.current(fence.begin()), false);
  const nextPage = new InboxFence();
  assert.equal(nextPage.current(nextPage.begin()), true);
});

test("a ticket from another store/page fence is never accepted", () => {
  const first = new InboxFence();
  const other = new InboxFence();
  assert.equal(first.current(other.begin()), false);
});
