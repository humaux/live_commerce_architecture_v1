// Purpose: denied A3 recovery without remount and actual-source print-ledger observation regressions.
// Depends on: faithful inbox host, real CommentLabelPrint/inboxWrite, TypeScript AST and isolated Node VM.
// Used by: test-node; browser/Go/PG and physical print acceptance stay separate.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import { environment, node, store, response } from "./inbox-review-host.test.ts";
const { jsx } = await import("react/jsx-runtime");
const { CommentLabelPrint, CommentLabelAction } = await import("../../apps/admin/components/CommentLabelPrint.tsx");

// Run the spec's actual pure recorder, not a copied helper or the browser test registration.
function recorder() {
  const ts = createRequire(import.meta.url)("typescript-api");
  const source = readFileSync(new URL("./comment-label-print.spec.ts", import.meta.url), "utf8");
  const parsed = ts.createSourceFile("spec.ts", source, ts.ScriptTarget.Latest, true);
  const fn = parsed.statements.find((n: any) => ts.isFunctionDeclaration(n) && n.name?.text === "record");
  assert.ok(fn);
  const js = ts.transpileModule(fn.getText(parsed), { compilerOptions: { target: ts.ScriptTarget.ES2023 } }).outputText;
  const context = { ledger: [] as any[], record: undefined as any };
  runInNewContext(`${js}; this.record = record`, context);
  return context;
}
test("W3U3 ledger actual comes from an independent observation", () => {
  const host = recorder(), actual = { labels: 3, writeCount: 3, printCalls: 1 };
  host.record("en", 390, "print", "click", "three labels recorded", actual, true);
  assert.deepEqual(JSON.parse(host.ledger[0].actual), actual);
  assert.notEqual(host.ledger[0].actual, host.ledger[0].expected);
  assert.equal(host.ledger[0].result, "PASS");
});
test("W3U3 ledger records failed observed criteria rather than constant PASS", () => {
  const host = recorder(), actual = { labels: 1, printCalls: 0, busy: true };
  host.record("en", 390, "print", "click", "three labels recorded", actual, false);
  assert.deepEqual(JSON.parse(host.ledger[0].actual), actual);
  assert.equal(host.ledger[0].result, "FAIL");
});

for (const status of [401, 403, 404]) test(`W3U3 denied ${status} resets busy and running without remount`, async (t) => {
  const env = environment(t);
  const previousElement = (globalThis as any).HTMLElement;
  class Element { nodeType = 1; focus() {} }
  (globalThis as any).HTMLElement = Element;
  Object.assign(env.document, { body: new Element(), activeElement: null });
  t.after(() => { (globalThis as any).HTMLElement = previousElement; });
  let calls = 0, denied = 0, prints = 0;
  Object.assign(env.window, { print: () => prints++ });
  globalThis.fetch = async () => {
    calls++;
    return response({ code: status === 401 ? "unauthorized" : status === 403 ? "forbidden" : "not_found" }, status);
  };
  const row = { ref: "123_456", author_name: "SYNTHETIC", created_at: "2030-01-01T00:00:00Z", marks: { claim: { keyword: "A1", quantity: 1 } } } as any;
  const children = jsx(CommentLabelAction, { row });
  const h = env.mount(() => CommentLabelPrint({ locale: "en", store: store.id, session: "22222222-2222-4222-8222-222222222222", rows: [row], allowed: true, active: true,
    // Deliberately do not call privacy.expire or change a React key.
    onDenied: () => { denied++; }, children }));
  const controller = h.ref(AbortController);
  node(h, (n) => n.props["data-testid"] === `comment-label-single-${row.ref}`).props.onClick();
  h.flush();
  const print = () => node(h, (n) => n.props["data-testid"] === "comment-label-print");
  print().props.onClick(); h.flush();
  assert.equal(print().props.disabled, true, "pending request disables print");
  await h.settle();
  assert.equal(denied, 1, "real transport reaches the denied callback");
  assert.equal(calls, 1);
  assert.equal(print().props.disabled, false, "denial cannot leave busy set");
  assert.equal(node(h, (n) => n.props["data-testid"] === "comment-label-close").props.disabled, false);
  assert.equal(h.ref(AbortController), controller, "same mounted controller; no remount repaired it");
  print().props.onClick(); await h.settle();
  assert.equal(calls, 2, "running guard is reset too");
  assert.equal(denied, 2);
  assert.equal(prints, 0, "denied writes never invoke native print");
});
