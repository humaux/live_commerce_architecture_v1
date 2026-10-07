// Purpose: guard the real bundle-recovery browser acceptance seam and credential-safe evidence policy.
// Depends on: Node test/assert/fs; real inbox Go fixture and Playwright source, no runtime browser or PG.
// Used by: LC-U2b static Node gate; these checks do not claim BROWSER acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { createHash, webcrypto } from "node:crypto";
import { runInNewContext } from "node:vm";
const go = readFileSync("tests/foundation/browser_inbox_ui_test.go", "utf8");
const spec = readFileSync("tests/admin/inbox-ui.spec.ts", "utf8");

test("bundle fixture uses real claim commands and mounts existing M6/M7 without issuing early", () => {
  for (const seam of ["Studio: true", "ClaimLabels: &e.h.labels", "e.h.draft(t", "e.h.open(t", "e.h.offer(t", "e.h.accepted(t", "link_pending_manual=true", "bhPublish(t, bcHarness{cqHarness: e.h.cqHarness}"])
    assert.ok(go.includes(seam), `missing real fixture seam: ${seam}`);
  assert.doesNotMatch(go, /e\.h\.(?:link|issue)\(/, "only the real UI may issue this link");
});

test("sensitive bundle scenario clicks native clipboard with trace/video disabled and no token body reads", () => {
  const scenario = spec.slice(spec.indexOf('test.describe("bundle recovery credential boundary"'));
  assert.ok(scenario.length > 0, "real sensitive scenario is required");
  assert.match(scenario, /trace: "off"/);
  assert.match(scenario, /video: "off"/);
  assert.match(scenario, /grantPermissions\(\["clipboard-read", "clipboard-write"\]/);
  assert.match(scenario, /getByTestId\("bundle-copy-link"\)\.click\(\)/);
  assert.match(spec, /navigator\.clipboard\.readText\(\)/);
  assert.match(spec, /crypto\.subtle\.digest\("SHA-256"/);
  assert.doesNotMatch(scenario, /page\.route\(|addInitScript|Object\.defineProperty|(?:response|issued|receipt)\.(?:json|text|body)\(/, "no request/clipboard mocks or credential response reads");
});

test("persisted link and token-free command receipt are compared without disclosing credentials", () => {
  for (const seam of ["claims.links", "ops.command_results", "live.claim.link.issue", "request_hash", "token_hash", "receipt_valid", "hash_matches", "principal_matches", "ttl_valid"])
    assert.ok(go.includes(seam), `missing persisted proof: ${seam}`);
  assert.match(spec, /after\.hash_matches\)\.toBe\(true\)/);
  assert.match(spec, /after\.receipt_valid\)\.toBe\(true\)/);
  assert.match(spec, /recopied\.dm_operations\)\.toBe\(before\.dm_operations\)/);
  assert.match(spec, /linkPosts\)\.toBe\(1\)/);
  assert.match(spec, /record\("Copy claim link"/);
});


test("actual clipboard evaluator exports only safe summary and detects malformed or retained synthetic credentials", async () => {
  // MODEL_ONLY: execute the actual source callback in a Node VM with synthetic browser globals, never a real browser mock.
  const ts = createRequire(import.meta.url)("typescript-api");
  const source = ts.createSourceFile("inbox-ui.spec.ts", spec, ts.ScriptTarget.Latest, true);
  const helper = source.statements.find((node: any) => ts.isFunctionDeclaration(node) && node.name?.text === "bundleClipboardProof");
  assert.ok(helper, "exercise the real browser evaluator");
  const code = ts.transpileModule(`${helper.getText(source)}; return bundleClipboardProof;`, {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
  }).outputText;
  const proof = new Function(code)();
  const credential = "X".repeat(43);
  const original = `https://inbox-bundle.example/en/claim#t=${credential}`;
  for (const variant of ["safe", "DOM", "storage", "URL", "invalid", "origin"] as const) {
    const copied = variant === "invalid" ? "SYNTHETIC-NON-URL" : variant === "origin" ? original.replace("inbox-bundle.example", "wrong.example") : original;
    const result = await proof({ evaluate(callback: Function, publishedOrigin: string) {
      const globals = {
        navigator: { clipboard: { readText: async () => copied } }, URL, TextEncoder, Uint8Array, crypto: webcrypto,
        location: { href: variant === "URL" ? original : "https://mock-admin.invalid/en/messages" },
        document: { documentElement: { outerHTML: variant === "DOM" ? credential : "<html />" } },
        localStorage: variant === "storage" ? { copied: original } : {}, sessionStorage: {},
        performance: { getEntriesByType: () => [] }, indexedDB: { databases: async () => [] }, caches: { keys: async () => [] },
        publishedOrigin,
      };
      return runInNewContext(`(${callback.toString()})(publishedOrigin)`, globals);
    } });
    assert.deepEqual(Object.keys(result).sort(), ["credentialSHA256", "fragment_valid", "origin_valid", "path_valid", "private_boundary"]);
    assert.ok(!JSON.stringify(result).includes(credential), "raw synthetic credential cannot leave the evaluator");
    if (variant === "invalid") assert.equal(result.fragment_valid, false);
    else assert.equal(result.credentialSHA256, createHash("sha256").update(credential).digest("hex"));
    if (["DOM", "storage", "URL", "invalid"].includes(variant)) assert.equal(result.private_boundary, false);
    if (variant === "origin") assert.equal(result.origin_valid, false);
    if (variant === "safe") assert.equal(result.private_boundary, true);
  }
});


test("claim-window fixture isolates recovery after the current migration permits multiple OPEN windows", () => {
  // SOURCE/static setup guard: 0122 supersedes 0060; closing this unused window is fixture isolation.
  const inherited = readFileSync("tests/foundation/meta_claims_intake_flow_test.go", "utf8");
  const schema = readFileSync("migrations/0060_live_claims.sql", "utf8");
  const lifecycle = readFileSync("migrations/0122_live_lifecycle.sql", "utf8");
  assert.match(inherited, /h\.open\(t, e\.session, claims\.MatchExact\)/);
  assert.match(schema, /CREATE UNIQUE INDEX live_claim_window_one_open[\s\S]*?WHERE state='OPEN'/);
  assert.match(lifecycle, /DROP INDEX live\.live_claim_window_one_open;/);
  assert.match(lifecycle, /CREATE INDEX live_claim_window_open[\s\S]*?WHERE state='OPEN'/);
  const inheritedClose = go.indexOf("e.h.closeWindow(t, e.session)");
  const recoveryOpen = go.indexOf('e.h.open(t, ids["bundle_session"], claims.MatchExact)');
  assert.ok(inheritedClose >= 0 && inheritedClose < recoveryOpen, "close the inherited OPEN window before opening the recovery session");
});
