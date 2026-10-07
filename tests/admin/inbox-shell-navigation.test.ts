// Purpose: preserve the operator's selection across synchronous inbox privacy teardown.
// Depends on: actual WorkspaceFrame JSX handlers and TypeScript's parser; the DOM reset is a controlled React counterexample.
// Used by: Node gate; navigation must capture the value before onBeforeNavigate can flush a controlled select.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
const ts = createRequire(import.meta.url)("typescript-api");
const source = ts.createSourceFile("WorkspaceFrame.tsx", readFileSync("apps/admin/components/WorkspaceFrame.tsx", "utf8"), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
function handler(id: string, before: () => boolean, calls: string[]) {
  let callback: any;
  function visit(node: any) {
    if (ts.isJsxOpeningElement(node) && node.tagName.getText(source) === "select") {
      const attrs = node.attributes.properties;
      if (attrs.some((a: any) => a.name?.getText(source) === "data-testid" && a.initializer?.text === id))
        callback = attrs.find((a: any) => a.name?.getText(source) === "onChange")?.initializer.expression;
    }
    ts.forEachChild(node, visit);
  }
  visit(source);
  assert.ok(callback, "execute actual shell change handler");
  const code = ts.transpileModule(`return ${callback.getText(source)};`, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;
  return new Function("before", "window", "router", "locale", "localizedPath", "error", "signingOut", "pathname", "search", code)(
    before, { location: { assign: (value: string) => calls.push(value) } }, { push: (value: string) => calls.push(value) },
    "en", (locale: string, path: string) => `/${locale}${path}`, null, false, "/messages", "store=MOCK_STORE",
  );
}
for (const id of ["shell-store-selector", "locale-switch"]) {
  for (const allowed of [true, false]) {
    test(`actual ${id} captures selection before synchronous privacy teardown (allowed=${allowed})`, () => {
      const chosen = id === "shell-store-selector" ? "MOCK_NEW_STORE" : "zh-TW";
      const event = { target: { value: chosen }, currentTarget: { value: chosen } }, calls: string[] = [];
      const onChange = handler(id, () => {
        // Inbox flushSync(suspend) rerenders the controlled select with its previous prop.
        event.target.value = event.currentTarget.value = id === "shell-store-selector" ? "MOCK_OLD_STORE" : "en";
        return allowed;
      }, calls);
      onChange(event);
      assert.deepEqual(calls, allowed ? [id === "shell-store-selector" ? "/en/?store=MOCK_NEW_STORE" : "/zh-TW/messages?store=MOCK_STORE"] : []);
    });
  }
}
