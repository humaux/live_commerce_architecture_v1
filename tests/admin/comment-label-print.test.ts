// Purpose: accepted-only labels, native modal focus recovery, denied A3 recovery and actual print-ledger regressions.
// Depends on: faithful inbox host, real CommentLabelPrint/inboxWrite, TypeScript AST and isolated Node VM.
// Used by: test-node; browser/Go/PG and physical print acceptance stay separate.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext, Script } from "node:vm";
import { environment, node, nodes, store, response } from "./inbox-review-host.test.ts";
import type { StreamComment } from "../../apps/admin/src/features/live/comment-model.ts";
import { commentLabelStrictFixture } from "./comment-label-strict-fixture.ts";
const { jsx } = await import("react/jsx-runtime");
const { CommentLabelPrint, CommentLabelAction, CommentLabelToolbar } = await import("../../apps/admin/components/CommentLabelPrint.tsx");

const acceptedRow = (): StreamComment => ({ ref: "123_456", seq: 1, parent_ref: null, author_name: "SYNTHETIC",
  created_at: "2030-01-01T00:00:00Z", text: "A1 1", is_page: false, has_attachment: false,
  marks: { claim: { status: "ACCEPTED", reason: null, offer_id: "11111111-1111-4111-8111-111111111111",
    keyword: "A1", quantity: 1, bundle_id: "33333333-3333-4333-8333-333333333333" }, intake: null, private_reply: null,
    printed: null, public_replies: 0, private_reply_available: false } });

// Only model the native modal's inert boundary; hooks, label filtering and transport stay actual source.
function labelHost(t: any, initial = acceptedRow()) {
  const env = environment(t), prior = (globalThis as any).HTMLElement, order: string[] = [];
  let modalOpen = false, writes = 0, prints = 0;
  class Element { nodeType = 1; focus() { order.push("focus"); if (!modalOpen) (env.document as any).activeElement = this; } }
  const opener = new Element(), body = new Element();
  (globalThis as any).HTMLElement = Element;
  Object.assign(env.document, { body, activeElement: opener });
  Object.assign(env.window, { print: () => { prints++; } });
  t.after(() => { (globalThis as any).HTMLElement = prior; });
  globalThis.fetch = async () => { writes++; return response({ print_count: writes, last_printed_at: null }); };
  let row = initial;
  const h = env.mount(() => CommentLabelPrint({ locale: "en", store: store.id,
    session: "22222222-2222-4222-8222-222222222222", rows: [row], allowed: true, active: true,
    onDenied() {}, children: jsx(CommentLabelAction, { row }) }));
  return { env, h, opener, order, writes: () => writes, prints: () => prints,
    update(next: typeof initial) { row = next; h.dirty = true; h.flush(); },
    open() {
      node(h, (n) => n.props["data-testid"] === `comment-label-single-${row.ref}`).props.onClick({ currentTarget: opener }); h.flush();
      const dialog = node(h, (n) => n.type === "dialog");
      const native = { open: false, showModal() { this.open = modalOpen = true; (env.document as any).activeElement = body; },
        close() { if (this.open) { order.push("close"); this.open = modalOpen = false; } } };
      // The generic hook Host intentionally does not attach native refs. Attach the test's dialog once.
      dialog.props.ref.current = native; native.showModal();
      return dialog;
    } };
}

test("W3U3 PR40 rejected positive-quantity claims expose no printable output or actions", (t) => {
  const row = acceptedRow(); row.marks.claim!.status = "REJECTED"; row.marks.claim!.bundle_id = null;
  const f = labelHost(t, row);
  assert.equal(nodes(f.h.output).filter((n) => /comment-label-(select|single|print|sheet|preview)/.test(n.props["data-testid"] ?? "")).length, 0);
  assert.equal(nodes(f.h.output).filter((n) => n.props["data-testid"] === "comment-label").length, 0);
  assert.equal(f.writes(), 0);
});
test("W3U3 PR40 rejected mark refresh removes an already previewed label", async (t) => {
  const f = labelHost(t); f.open();
  assert.equal(nodes(f.h.output).filter((n) => n.props["data-testid"] === "comment-label").length, 1);
  const oldPrint = node(f.h, (n) => n.props["data-testid"] === "comment-label-print").props.onClick;
  const rejected = acceptedRow(); rejected.marks.claim!.status = "REJECTED"; rejected.marks.claim!.bundle_id = null;
  f.update(rejected);
  assert.equal(nodes(f.h.output).filter((n) => n.props["data-testid"] === "comment-label").length, 0);
  assert.equal(nodes(f.h.output).filter((n) => n.props["data-testid"] === "comment-label-print").length, 0);
  oldPrint(); await f.h.settle();
  assert.equal(f.writes(), 0, "a retained pre-refresh handler cannot record a rejected claim");
  assert.equal(f.prints(), 0);
});
for (const via of ["close", "escape", "after print"]) test(`W3U3 PR40 ${via} restores opener focus after native modal closes`, async (t) => {
  const f = labelHost(t), dialog = f.open();
  assert.notEqual((f.env.document as any).activeElement, f.opener);
  if (via === "after print") {
    node(f.h, (n) => n.props["data-testid"] === "comment-label-print").props.onClick(); await f.h.settle();
    assert.equal(f.writes(), 1); assert.equal(f.prints(), 1);
  }
  if (via === "escape") {
    let prevented = false; dialog.props.onCancel({ preventDefault() { prevented = true; } });
    assert.equal(prevented, true);
  } else node(f.h, (n) => n.props["data-testid"] === "comment-label-close").props.onClick();
  f.h.flush();
  assert.deepEqual(f.order, ["close", "focus"], "focus outside an open modal would be ignored");
  assert.equal((f.env.document as any).activeElement, f.opener);
  assert.equal(nodes(f.h.output).filter((n) => n.type === "dialog").length, 0);
});
test("W3U3 PR40 focus returns to the triggering button even when it was not the active element", (t) => {
  const f = labelHost(t); (f.env.document as any).activeElement = null;
  f.open(); node(f.h, (n) => n.props["data-testid"] === "comment-label-close").props.onClick(); f.h.flush();
  assert.equal((f.env.document as any).activeElement, f.opener);
});

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
// Execute the spec's actual A4 criterion and teardown, including its evidence-before-failure order.
function paperLedger(columns: number) {
  const require = createRequire(import.meta.url), ts = require("typescript-api");
  const source = readFileSync(new URL("./comment-label-print.spec.ts", import.meta.url), "utf8");
  const parsed = ts.createSourceFile("spec.ts", source, ts.ScriptTarget.Latest, true);
  let paper: any, teardown: any;
  const visit = (n: any) => {
    if (ts.isCallExpression(n) && n.expression.getText(parsed) === "record" && n.arguments[2]?.text === "paper selector") paper = n;
    if (ts.isCallExpression(n) && n.expression.getText(parsed) === "test.afterEach") teardown = n.arguments[0];
    ts.forEachChild(n, visit);
  }; visit(parsed); assert.ok(paper); assert.ok(teardown);
  const context = Object.assign(recorder(), { locale: "en", width: 1586, evidence: "synthetic-evidence", saved: [] as any[],
    paperSequence: ["a4", "label", "a4"], a4: { columns, paper: "a4", labelCount: 3, printControlVisible: false, streamVisible: false, headingVisible: false },
    box: { width: 60 * 96 / 25.4, height: 40 * 96 / 25.4 }, single: { restoredPaper: "a4" },
    assert, afterEach: undefined as any,
    writeFile: async (_: string, body: string) => { context.saved.push(JSON.parse(body)); } });
  const compile = (text: string) => ts.transpileModule(text, { compilerOptions: { target: ts.ScriptTarget.ES2023 } }).outputText;
  runInNewContext(compile(paper.getText(parsed) + "; this.afterEach = (" + teardown.getText(parsed) + ");"), context);
  return context;
}
test("W3U3 PR40 two-column A4 calibration fails only after the FAIL ledger is saved", async () => {
  const h = paperLedger(2);
  assert.equal(h.ledger[0].result, "FAIL"); assert.equal(JSON.parse(h.ledger[0].actual).a4.columns, 2);
  await assert.rejects(h.afterEach({ page: {} }, { status: "passed", title: "A4 calibration", testId: "a4-two" }), /paper selector/);
  assert.equal(h.saved.length, 1); assert.equal(h.saved[0][0].result, "FAIL");
});
test("W3U3 PR40 normal three-column A4 persists PASS without a teardown failure", async () => {
  const h = paperLedger(3);
  await h.afterEach({ page: {} }, { status: "passed", title: "A4 normal", testId: "a4-three" });
  assert.equal(h.saved.length, 1); assert.equal(h.saved[0][0].result, "PASS");
});

// Companion ownership check, not a substitute for the real React development browser mount.
function mountEffect() {
  const ts = createRequire(import.meta.url)("typescript-api");
  const source = readFileSync(new URL("../../apps/admin/components/CommentLabelPrint.tsx", import.meta.url), "utf8");
  const parsed = ts.createSourceFile("component.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  let effect: any;
  const visit = (n: any) => {
    if (!effect && ts.isCallExpression(n) && n.expression.getText(parsed) === "useEffect" && ts.isArrayLiteralExpression(n.arguments[1]) && !n.arguments[1].elements.length) effect = n.arguments[0];
    ts.forEachChild(n, visit);
  }; visit(parsed); assert.ok(effect);
  const context = { controller: { current: new AbortController() }, pending: { current: new Map() },
    AbortController, localStorage: { getItem: () => null }, setPaper() {}, setup: undefined as any };
  const js = ts.transpileModule("this.setup = (" + effect.getText(parsed) + ");", { compilerOptions: { target: ts.ScriptTarget.ES2023 } }).outputText;
  runInNewContext(js, context); return context;
}
test("W3U3 PR40 effect replay creates a fresh live controller after cleanup", () => {
  const h = mountEffect(), cleanup = h.setup(), first = h.controller.current;
  cleanup(); assert.equal(first.signal.aborted, true);
  const cleanup2 = h.setup(), second = h.controller.current;
  assert.notEqual(second, first); assert.equal(second.signal.aborted, false);
  cleanup2(); assert.equal(second.signal.aborted, true);
});
test("W3U3 PR40 cleanup owns its controller rather than aborting a later setup", () => {
  const h = mountEffect(), cleanup1 = h.setup(); h.setup(); const second = h.controller.current;
  cleanup1(); assert.equal(second.signal.aborted, false);
});
test("W3U3 PR40 StrictMode fixture uses real installed dev React and valid actual component code", () => {
  const fixture = commentLabelStrictFixture();
  assert.doesNotThrow(() => new Script(fixture.script));
  assert.match(fixture.script, /React\.StrictMode/); assert.match(fixture.script, /createRoot/);
  assert.match(fixture.script, /react\.development|ReactVersion|19\.3\.0/);
  assert.match(fixture.script, /Release the native modal's inert boundary/);
  assert.ok(fixture.html.includes("/__test/w3u3-strict.js"));
});
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
  const row = acceptedRow();
  const children = jsx(CommentLabelAction, { row });
  const h = env.mount(() => CommentLabelPrint({ locale: "en", store: store.id, session: "22222222-2222-4222-8222-222222222222", rows: [row], allowed: true, active: true,
    // Deliberately do not call privacy.expire or change a React key.
    onDenied: () => { denied++; }, children }));
  const controller = h.ref(AbortController);
  node(h, (n) => n.props["data-testid"] === `comment-label-single-${row.ref}`).props.onClick({ currentTarget: new Element() });
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

// PR #40 review P2: an unreachable A3 must not hold Print/Close for N × the 8 s write timeout. After the first
// transport/5xx failure the batch stops recording and opens native print with the failure notice.
test("W3U3 PR40 outage: first transport failure stops recording and still opens native print", async (t) => {
  const env = environment(t);
  const previousElement = (globalThis as any).HTMLElement;
  class Element { nodeType = 1; focus() {} }
  (globalThis as any).HTMLElement = Element;
  Object.assign(env.document, { body: new Element(), activeElement: null });
  t.after(() => { (globalThis as any).HTMLElement = previousElement; });
  let calls = 0, prints = 0;
  Object.assign(env.window, { print: () => prints++ });
  globalThis.fetch = async () => { calls++; throw new TypeError("synthetic network outage"); };
  const rows = ["101_1", "101_2", "101_3"].map((ref, i) => ({ ...acceptedRow(), ref, seq: i + 1 }));
  const children = [jsx(CommentLabelToolbar, {}), ...rows.map((row) => jsx(CommentLabelAction, { row }))];
  const h = env.mount(() => CommentLabelPrint({ locale: "en", store: store.id, session: "22222222-2222-4222-8222-222222222222", rows, allowed: true, active: true,
    onDenied() {}, children }));
  for (const row of rows) { node(h, (n) => n.props["data-testid"] === `comment-label-select-${row.ref}`).props.onChange(); h.flush(); }
  node(h, (n) => n.props["data-testid"] === "comment-label-preview").props.onClick({ currentTarget: new Element() }); h.flush();
  node(h, (n) => n.props["data-testid"] === "comment-label-print").props.onClick(); await h.settle();
  assert.equal(calls, 1, "an outage costs one bounded attempt, not one timeout per selected label");
  assert.equal(prints, 1, "native print still opens");
  assert.equal(nodes(h.output).some((n) => n.props["data-testid"] === "comment-label-status"), true, "failure notice shown");
});

// PR #40 review P2: 429/408 are throttle/timeout, not definitive. Stop like an outage instead of N more writes.
for (const status of [408, 429]) test(`W3U3 PR40 ${status} stops the batch like an outage`, async (t) => {
  const env = environment(t);
  const previousElement = (globalThis as any).HTMLElement;
  class Element { nodeType = 1; focus() {} }
  (globalThis as any).HTMLElement = Element;
  Object.assign(env.document, { body: new Element(), activeElement: null });
  t.after(() => { (globalThis as any).HTMLElement = previousElement; });
  let calls = 0, prints = 0;
  Object.assign(env.window, { print: () => prints++ });
  globalThis.fetch = async () => { calls++; return response({ code: status === 429 ? "rate_limited" : "timeout" }, status); };
  const rows = ["102_1", "102_2", "102_3"].map((ref, i) => ({ ...acceptedRow(), ref, seq: i + 1 }));
  const children = [jsx(CommentLabelToolbar, {}), ...rows.map((row) => jsx(CommentLabelAction, { row }))];
  const h = env.mount(() => CommentLabelPrint({ locale: "en", store: store.id, session: "22222222-2222-4222-8222-222222222222", rows, allowed: true, active: true,
    onDenied() {}, children }));
  for (const row of rows) { node(h, (n) => n.props["data-testid"] === `comment-label-select-${row.ref}`).props.onChange(); h.flush(); }
  node(h, (n) => n.props["data-testid"] === "comment-label-preview").props.onClick({ currentTarget: new Element() }); h.flush();
  node(h, (n) => n.props["data-testid"] === "comment-label-print").props.onClick(); await h.settle();
  assert.equal(calls, 1, `${status} costs one attempt, not one per label`);
  assert.equal(prints, 1, "native print still opens");
});
