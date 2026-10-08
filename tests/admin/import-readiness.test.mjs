// Purpose: guard the actual import login helper's visible readiness for authorized and route-denied identities.
// Depends on: Node test/assert/fs/vm and installed typescript-api; extracts and executes only the actual helper AST.
// Used by: focused W5 Node readiness red/green; MOCK browser/page only, no real browser, authentication or BFF claim.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";

const file = new URL("./import-wizard.spec.ts", import.meta.url);
function helper(actor = "owner", root = "import-wizard") {
  const tree = ts.createSourceFile(file.pathname, readFileSync(file, "utf8"), ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
  const signed = tree.statements.find(node => ts.isFunctionDeclaration(node) && node.name?.text === "signed");
  assert.ok(signed, "actual signed helper must exist");
  const exports = {}, flags = { cookie: 0, wizard: 0 };
  const expect = value => ({ toBe: expected => { flags.cookie++; assert.equal(value, expected, "MIUI-COOKIE-SECURE-HTTPONLY"); },
    toBeVisible: async () => { flags.wizard++; assert.equal(value.id, root, "MIUI-READY-AUTHORIZED-ROOT"); } });
  const code = ts.transpileModule("export " + signed.getText(tree), { compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.CommonJS } }).outputText;
  runInNewContext(code, { exports, expect, origin: "https://admin.example.invalid", store: "synthetic-store",
    ctl: async (route, selected) => { assert.equal(route, "actor"); assert.equal(selected, actor); },
    step: async (_page, _name, _expected, action) => action(), wizard: page => page.getByTestId("import-wizard") });
  return { signed: exports.signed, flags };
}

function browser(cookie) {
  const calls = { clicked: false, waited: [], visited: [] };
  const page = {
    on: (event, callback) => { assert.equal(event, "console"); assert.equal(typeof callback, "function"); },
    goto: async url => { calls.visited.push(url); },
    getByRole: (role, options) => ({ click: async () => { assert.equal(role, "button"); assert.equal(options.name, "Sign in with identity service"); calls.clicked = true; } }),
    getByTestId: id => ({ id, waitFor: async options => {
      assert.equal(calls.clicked, true, "readiness must follow the sign-in click");
      calls.waited.push(id); assert.equal(options?.state ?? "visible", "visible");
      if (id === "workspace-sign-out") throw new Error("MIUI-READY-VISIBLE-STORE: sign-out is inside a closed details menu");
      assert.equal(id, "shell-store-selector", "MIUI-READY-VISIBLE-STORE");
    } }),
  };
  const context = { newPage: async () => page, cookies: async () => [cookie] };
  return { calls, value: { newContext: async options => { assert.equal(options.ignoreHTTPSErrors, true); return context; } } };
}

test("MIUI-READY-VISIBLE-STORE actual signed helper waits on the visible selector and retains cookie/wizard assertions", async () => {
  for (const [locale, width] of [["zh-TW", 390], ["en", 1440]]) {
    const loaded = helper(), fake = browser({ name: "__Host-synthetic", secure: true, httpOnly: true });
    const result = await loaded.signed(fake.value, locale, width);
    assert.equal(result.page !== undefined, true); assert.deepEqual(fake.calls.waited, ["shell-store-selector"]);
    assert.equal(fake.calls.visited.at(-1), `https://admin.example.invalid/${locale}/customers/import?store=synthetic-store`);
    assert.equal(loaded.flags.cookie, 1); assert.equal(loaded.flags.wizard, 1);
  }
  for (const cookie of [{ name: "__Host-synthetic", secure: false, httpOnly: true }, { name: "__Host-synthetic", secure: true, httpOnly: false }]) {
    const loaded = helper(), fake = browser(cookie);
    await assert.rejects(loaded.signed(fake.value, "en", 1440), /MIUI-COOKIE-SECURE-HTTPONLY/);
  }
});

test("MIUI-READY-DENIED read-only staff reaches the real shell refusal instead of waiting for a private wizard", async () => {
  const loaded = helper("reader", "route-forbidden"), fake = browser({ name: "__Host-synthetic", secure: true, httpOnly: true });
  await loaded.signed(fake.value, "en", 390, "reader");
  assert.deepEqual(fake.calls.waited, ["shell-store-selector"]);
  assert.equal(loaded.flags.cookie, 1); assert.equal(loaded.flags.wizard, 1);
});
