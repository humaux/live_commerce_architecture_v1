// Purpose: adversarial health DTO/request examples, including the actual backend's omitted capability identifier.
// Depends on: Node test/assert/fs/child_process, pure health model/request and the actual admin build selector; no browser/PG/provider.
// Used by: test-node.sh, --browser-meta-health-ui and independent acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { parseMetaHealth, parseRecheck, healthReason } from "../../apps/admin/lib/meta-health-model.ts";
import { metaHealthRoute, validMetaHealthRequest, validRecheckBody } from "../../apps/admin/lib/meta-health-request.ts";
const id = "00000000-0000-4000-8000-000000000001";
const cap = { binding_id: id, provider: "facebook", capability: "read_comment", state: "review_required", reason: "standard_access", evidence: "MOCK" };
const page = { page_id: "123", page_name: "Synthetic Page", status: "active", severity: "none", capabilities: [cap] };
const dto = { severity: "none", pages: [page] };
test("review-required remains restricted; evidence does not promote state", () => {
  const result = parseMetaHealth(dto);
  assert.equal(result.pages[0].capabilities[0].state, "review_required");
  assert.equal(result.pages[0].capabilities[0].evidence, "MOCK");
});
test("legacy omission remains unidentified, independent of array order", () => {
  const { capability: _, ...legacy } = cap;
  const result = parseMetaHealth({ ...dto, pages: [{ ...page, capabilities: [legacy, { ...legacy, state: "missing_task", reason: "task_moderate" }] }] });
  assert.ok(result.pages[0].capabilities.every(c => c.capability === undefined));
});
test("closed shapes reject unknown state/provider/evidence, leaked fields and inconsistent severity", () => {
  for (const patch of [{ state: "production_supported" }, { provider: "other" }, { evidence: "LIVE" }, { token: "synthetic-canary" }, { capability: "invented" }, { checked_at: "yesterday" }]) {
    assert.throws(() => parseMetaHealth({ ...dto, pages: [{ ...page, capabilities: [{ ...cap, ...patch }] }] }));
  }
  assert.throws(() => parseMetaHealth({ ...dto, severity: "blocking" }));
  assert.throws(() => parseMetaHealth({ ...dto, tenant_id: id }));
});
test("duplicate capabilities/pages and unbounded arrays cannot become trusted state", () => {
  assert.throws(() => parseMetaHealth({ ...dto, pages: [page, page] }));
  assert.throws(() => parseMetaHealth({ ...dto, pages: [{ ...page, capabilities: [cap, cap] }] }));
  assert.throws(() => parseMetaHealth({ severity: "none", pages: Array.from({ length: 11 }, (_, n) => ({ ...page, page_id: String(n) })) }));
});
test("worst Page determines advice, no raw upstream reason reaches copy", () => {
  const health = parseMetaHealth({ severity: "blocking", pages: [page, { ...page, page_id: "456", severity: "blocking", capabilities: [{ ...cap, state: "reauth_required", reason: "token_revoked" }] }] });
  assert.equal(healthReason(health), "reauth_required");
  assert.equal(healthReason(parseMetaHealth({ severity: "warning", pages: [{ ...page, severity: "warning", capabilities: [] }] })), "probe_failing");
});
test("recheck receipt means scheduled; timestamps and closed keys are checked", () => {
  assert.equal(parseRecheck({ next_check_at: "2026-10-06T00:00:00.123456789Z" }).next_check_at, "2026-10-06T00:00:00.123456789Z");
  assert.throws(() => parseRecheck({ next_check_at: "never" }));
  assert.throws(() => parseRecheck({ next_check_at: "2026-10-06T00:00:00Z", ok: true }));
});
test("exact keyless route grammar rejects query/key/body confusion", () => {
  assert.equal(metaHealthRoute("GET", "meta/health"), "status");
  assert.equal(metaHealthRoute("POST", "meta/health/recheck"), "recheck");
  for (const [method, path] of [["POST", "meta/health"], ["GET", "meta/health/recheck"], ["GET", "meta/health/"], ["DELETE", "meta/health"]]) assert.equal(metaHealthRoute(method, path), null);
  const read = { method: "GET", url: "https://local.test/api/meta/health", body: null, headers: new Headers() };
  assert.ok(validMetaHealthRequest(read));
  assert.equal(validMetaHealthRequest({ ...read, url: read.url + "?" }), false);
  assert.equal(validMetaHealthRequest({ ...read, body: "{}" }), false);
  for (const [name, value] of [["idempotency-key", "new-key-123"], ["transfer-encoding", "chunked"], ["content-length", "4"]]) assert.equal(validMetaHealthRequest({ ...read, headers: new Headers({ [name]: value }) }), false);
  assert.ok(validMetaHealthRequest({ ...read, method: "POST", body: "{}", headers: new Headers({ "content-type": "application/json" }) }));
});
test("recheck takes exactly page_id and no caller-provided scope", () => {
  assert.ok(validRecheckBody('{"page_id":"123"}'));
  for (const value of ['{}', '{"page_id":123}', '{"page_id":"123","tenant_id":"x"}', '{"page_id":"../123"}', '{"page_id":""}', '{']) assert.equal(validRecheckBody(value), false);
});
test("blocking advice comes from the blocking capability, not a secondary public-reply warning", () => {
  const health = parseMetaHealth({ severity: "blocking", pages: [{ ...page, severity: "blocking", capabilities: [
    { ...cap, state: "unknown", reason: "not_probed" },
    { ...cap, capability: "reply_public", state: "missing_permission", reason: "perm_pages_manage_engagement" },
  ] }] });
  assert.equal(healthReason(health), "unknown");
});

test("clean health browser mode prepares admin standalone before starting its fixture", async () => {
  const { readFileSync } = await import("node:fs");
  const { execFileSync } = await import("node:child_process");
  const script = readFileSync(new URL("../../scripts/dev/test-local.sh", import.meta.url), "utf8");
  const lines = script.split("\n");
  const build = lines.findIndex(line => line.includes("pnpm run build:admin"));
  assert.ok(build > 0);
  const condition = lines.slice(0, build).reverse().find(line => line.startsWith("if [["));
  assert.ok(condition);
  // Execute only the real preparation predicate, never the build or PG/browser mode.
  const outcome = execFileSync("bash", ["-c", `test_mode="$1"\n${condition}\n  printf prepared\nelse\n  printf missing\nfi`, "health-build-predicate", "--browser-meta-health-ui"], { encoding: "utf8" });
  assert.equal(outcome, "prepared");
});
