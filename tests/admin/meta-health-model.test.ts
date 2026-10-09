// Purpose: adversarial health DTO/request examples, including the actual backend's omitted capability identifier.
// Depends on: Node test/assert/fs/child_process/vm, pure health model/request, MetaConnect/Badge JSX and typescript-api; no browser/PG/provider.
// Used by: test-node.sh, --browser-meta-health-ui and independent acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { parseMetaHealth, parseRecheck, healthReason } from "../../apps/admin/lib/meta-health-model.ts";
import { metaHealthRoute, validMetaHealthRequest, validRecheckBody } from "../../apps/admin/lib/meta-health-request.ts";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { metaConnectCopy } from "../../apps/admin/lib/meta-connect-copy.ts";
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
  const plan = execFileSync("bash", ["scripts/dev/test-local.sh", "--dry-run", "--browser-meta-health-ui"], { encoding: "utf8" });
  assert.ok(plan.indexOf("pnpm run build:admin") > 0);
  assert.ok(plan.indexOf("pnpm run build:admin") < plan.indexOf("docker run"), "admin build precedes fixture startup");
});

// MOCK SSR follows the existing attribution tests: execute the actual component and shared Badge,
// supplying a read-only connected-page snapshot. Effects never run and commands never execute.
const requireApp = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const React = requireApp("react"), { renderToStaticMarkup } = requireApp("react-dom/server");
const presentation: Record<string, any> = {};
runInNewContext(ts.transpileModule(readFileSync(new URL("../../packages/ui/src/Presentation.tsx", import.meta.url), "utf8"), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
}).outputText, { exports: presentation, require: (name: string) => name.endsWith(".css") ? { __esModule: true, default: { badge: "badge" } } : requireApp(name) });

let renderStateCalls = 0;
const component: Record<string, any> = {};
const status = { connected: true, count: 1, cap: 10, pages: [{
  id: "123", name: "Synthetic Page", status: "reauth_required", instagram: null, permissions: [],
  connected_at: "2026-10-06T00:00:00Z", route_expires_at: "2026-10-07T00:00:00Z", last_event_at: null,
}] };
runInNewContext(ts.transpileModule(readFileSync(new URL("../../apps/admin/components/MetaConnect.tsx", import.meta.url), "utf8"), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
}).outputText, {
  exports: component,
  require: (name: string) => {
    if (name === "react") return { ...React, useState: (initial: any) => React.useState(
      ++renderStateCalls === 1 ? "ready" : renderStateCalls === 2 ? status : initial,
    ) };
    if (name === "@live-commerce/ui") return presentation;
    if (name === "@/lib/meta-connect-copy") return { metaConnectCopy };
    if (name === "@/lib/meta-health-hook") return { useMetaHealth: () => ({ data: null, failed: false }) };
    if (name === "@/lib/orders-model") return { displayTime: (_locale: string, value: string) => value };
    if (name === "./MetaHealthCapabilities") return { MetaHealthCapabilities: () => null };
    if (name.endsWith(".css") || name.startsWith("@/lib/")) return {};
    return requireApp(name);
  },
});

for (const locale of ["en", "zh-TW", "zh-CN"] as const) test(`${locale}: expired connection retains its full refusal inside a wrapping badge`, () => {
  renderStateCalls = 0;
  const html = renderToStaticMarkup(React.createElement(component.MetaConnect, { store: id, locale, canManage: false }));
  const badge = /<dd data-testid="metaconnect-token" data-state="reauth_required">([\s\S]*?)<\/dd>/.exec(html)?.[1];
  assert.ok(badge, "render the actual expired-connection status");
  assert.ok(badge.includes(metaConnectCopy[locale].tokenReauth), "never shorten or hide the explanation to fit");
  const shared = /\.badge\s*\{([^}]+)\}/.exec(readFileSync(new URL("../../packages/ui/src/Presentation.module.css", import.meta.url), "utf8"))?.[1] ?? "";
  const inherited = /white-space:\s*([^;]+)/.exec(shared)?.[1];
  const override = /style="[^"]*white-space:([^;\"]+)/.exec(badge)?.[1];
  assert.equal(override ?? inherited, "normal", "the sentence must wrap inside the narrow connection card");
  assert.equal(/text-overflow:ellipsis|overflow:hidden/.test(badge), false, "no clipped refusal copy");
});

test("actual B1 snapshot state remains ok even when a fixture expected review_required", () => {
  // CI37470022946 trace: the connection reset deleted probed rows; B1 correctly returned this fallback.
  const value = parseMetaHealth({ ...dto, pages: [{ ...page, capabilities: [{ ...cap, state: "ok", reason: "ok_snapshot", evidence: "DESIGN" }] }] });
  assert.equal(value.pages[0].capabilities[0].state, "ok");
  assert.equal(value.pages[0].capabilities[0].checked_at, undefined);
});
