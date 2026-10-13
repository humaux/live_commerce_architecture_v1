// Purpose: counterexamples for continuous product-pane evidence and fail-closed supplemental capture verdicts.
// Depends on: Node test/fs/crypto/vm, visual capture/lint helpers and the actual visual-audit task/verdict; synthetic transport and temporary output.
// Used by: scripts/dev/test-node.sh; DB-free tests do not claim browser acceptance.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import path from "node:path";
import { tmpdir } from "node:os";
import test from "node:test";
import { runInNewContext } from "node:vm";
import { captureInnerScrollFrames, supplementalViewportFor, foldSupplementalCapture } from "./visual-scroll-capture.mjs";
import { VISUAL_VIEWPORTS } from "./sweep-shard-lib.mjs";
import { RULES, BLOCKING, BLOCKING_KINDS } from "./visual-lint-lib.mjs";

function transport({ height = 3200, client = 600, top = 0, frozen = false, windowOnly = false, visible = client, count = 1 } = {}) {
  const actions = [];
  let scrollTop = top, windowY = 0;
  const geometry = () => ({ scrollTop, scrollHeight: height, clientHeight: client, clientWidth: 350, contentTop: 200, windowY, rect: { x: 20, y: 200, width: 350, height: client }, visible: { x: 20, y: 200, width: 350, height: visible }, wheelPoint: { x: 24, y: 200 + visible / 2 } });
  return {
    actions,
    locator: () => ({ count: async () => count, evaluate: async () => geometry() }),
    mouse: { move: async (x, y) => actions.push(["move", x, y]), wheel: async (x, y) => { actions.push(["wheel", x, y]); if (windowOnly) windowY += y; else if (!frozen) scrollTop = Math.max(0, Math.min(height - client, scrollTop + y)); } },
    waitForTimeout: async () => {},
    screenshot: async (options) => { actions.push(["screenshot", options]); return Buffer.from(`pane:${scrollTop},document:${windowY}`); },
  };
}
async function capture(page, options = {}) {
  const out = await mkdtemp(path.join(tmpdir(), "visual-scroll-"));
  try {
    const result = await captureInnerScrollFrames(page, { selector: ".pe-fields", out, filePrefix: "admin/products-new/zh-TW-390x844", ...options });
    for (const frame of result.frames) {
      const bytes = await readFile(path.join(out, frame.file));
      assert.equal(frame.sha256, createHash("sha256").update(bytes).digest("hex"));
      assert.equal(frame.bytes, bytes.length);
    }
    return result;
  } finally { await rm(out, { recursive: true, force: true }); }
}

test("native wheel frames cover tall section interiors, not just section headings", async () => {
  const page = transport(), result = await capture(page);
  assert.equal(result.complete, true);
  assert.ok(result.frames.length > 4, "one fullPage document screenshot misses inner content");
  let covered = 0;
  for (const frame of result.frames) {
    assert.ok(frame.visibleRange.from <= covered + 1, "an interior was skipped");
    covered = Math.max(covered, frame.visibleRange.to);
  }
  assert.equal(covered, 3200);
  assert.ok(page.actions.some(([kind]) => kind === "wheel"), "coverage must come from native wheel input");
  assert.ok(page.actions.filter(([kind]) => kind === "screenshot").every(([, options]) => options.fullPage === false), "supplemental frames are honest viewport captures");
});
test("a frozen pane fails instead of claiming complete evidence", async () => {
  const result = await capture(transport({ frozen: true }));
  assert.equal(result.complete, false); assert.match(result.reason, /no-progress/);
});
test("document-only wheel movement does not prove that the nested pane scrolled", async () => {
  const result = await capture(transport({ windowOnly: true }));
  assert.equal(result.complete, false); assert.match(result.reason, /no-progress/);
});
test("frame bound fails visibly and retains partial shots", async () => {
  const result = await capture(transport(), { maxFrames: 3 });
  assert.equal(result.complete, false); assert.match(result.reason, /frame-limit/);
  assert.equal(result.frames.length, 3);
});
test("initial offset is rewound using native input before shooting", async () => {
  const page = transport({ top: 1500 }), result = await capture(page);
  assert.equal(result.complete, true);
  assert.equal(result.frames[0].geometry.scrollTop, 0);
  assert.ok(page.actions.some(([kind, , y]) => kind === "wheel" && y < 0));
});
test("a hidden final slice is not mistaken for a reached end", async () => {
  const result = await capture(transport({ visible: 440 }));
  assert.equal(result.complete, false); assert.match(result.reason, /end-not-visible/);
});
test("a short pane is captured once without unnecessary scrolling", async () => {
  const page = transport({ height: 600 }), result = await capture(page);
  assert.equal(result.complete, true); assert.equal(result.frames.length, 1);
  assert.equal(page.actions.some(([kind]) => kind === "wheel"), false);
});
test("a missing required product pane fails, never skips a page", async () => {
  const result = await capture(transport({ count: 0 }));
  assert.equal(result.complete, false); assert.match(result.reason, /pane-count/);
});


test("1440 supplement covers only affected desktop admin routes without replacing the primary matrix", () => {
  assert.deepEqual(VISUAL_VIEWPORTS.map((v) => v.size), [{ width: 1586, height: 992 }, { width: 390, height: 844 }]);
  for (const locale of ["zh-TW", "zh-CN", "en"]) {
    for (const route of ["/ads", "/products/new", "/products/[product]"]) {
      const extra = supplementalViewportFor({ app: "admin", route, locale, v: VISUAL_VIEWPORTS[0] });
      assert.deepEqual(extra.size, { width: 1440, height: 900 });
      assert.equal(extra.mobile, false);
      assert.equal(supplementalViewportFor({ app: "admin", route, locale, v: VISUAL_VIEWPORTS[1] }), null);
    }
  }
  for (const app of ["storefront", "platform"]) assert.equal(supplementalViewportFor({ app, route: "/products/new", v: VISUAL_VIEWPORTS[0] }), null);
  for (const route of ["/", "/orders", "/products", "/ads-extra"]) assert.equal(supplementalViewportFor({ app: "admin", route, v: VISUAL_VIEWPORTS[0] }), null);
});
test("supplemental 1440 blockers are folded into the real owning verdict without inflating primary captures", () => {
  const parent = { file: "primary.png", sha256: "primary-hash", size: "1586x992", viewport: "desktop", counts: { R4: 1 }, groups: { R4: 1 }, blockingCounts: {}, blocking: 0, violations: [], issues: [], supplemental: [{ kind: "inner-scroll frames" }] };
  const extra = { file: "1440.png", sha256: "extra-hash", size: "1440x900", viewport: "supplemental-desktop", status: 200, ok: true, counts: { R1: 1, R4: 2 }, groups: { R1: 1, R4: 2 }, blockingCounts: { R1: 1 }, blocking: 1, violations: [{ rule: "R1", severity: "block" }], layout: { viewport: { w: 1440, h: 900 } }, supplemental: [{ kind: "inner-scroll frames", complete: true, frames: [{ file: "inner.png", sha256: "inner-hash" }] }] };
  const result = foldSupplementalCapture(parent, extra);
  assert.equal(result, parent);
  assert.deepEqual([parent.file, parent.sha256, parent.size, parent.viewport], ["primary.png", "primary-hash", "1586x992", "desktop"]);
  assert.deepEqual(parent.counts, { R1: 1, R4: 3 });
  assert.equal(parent.blocking, 1); assert.equal(parent.blockingCounts.R1, 1);
  assert.equal(parent.violations[0].supplementalViewport, "1440x900");
  assert.equal(parent.supplemental[1].kind, "supplemental viewport");
  assert.deepEqual(parent.supplemental[1].layout.viewport, { w: 1440, h: 900 });
  assert.equal(parent.supplemental[1].supplemental[0].frames[0].file, "inner.png");
});
test("supplemental capture failure cannot hide behind an existing successful primary screenshot", () => {
  for (const failure of [{ status: 503, file: "error.png", ok: false }, { status: 200, file: "", ok: true }, { status: 200, file: "ok.png", ok: true, stateFailed: true }, { status: 200, file: "ok.png", ok: true, failed: true }]) {
    const parent = { file: "primary.png", counts: {}, groups: {}, blockingCounts: {}, blocking: 0, violations: [], issues: [] };
    foldSupplementalCapture(parent, { ...failure, size: "1440x900", counts: {}, groups: {}, blockingCounts: {}, blocking: 0, violations: [] });
    assert.equal(parent.stateFailed, true);
    assert.ok(parent.issues.some((issue) => issue.includes("1440x900")));
  }
});

// Execute the actual runner task and verdict, including its pool's swallowed task-error behavior.
// Only browser/capture transport is fake; supplemental folding and G-UI9 aggregation are real.
async function supplementalTaskVerdict(failure) {
  const source = await readFile(new URL("./visual-audit.mjs", import.meta.url), "utf8"), results = [], closed = [], errors = [];
  const between = (start, end) => source.slice(source.indexOf(start), source.indexOf(end, source.indexOf(start)));
  const transport = {
    browser: { newContext: async (options) => {
      const extra = options.viewport.width === 1440;
      if (extra && failure === "newContext") throw new Error("injected supplemental newContext rejection");
      return { addInitScript: async () => {}, route: async () => {},
        newPage: async () => { if (extra && failure === "newPage") throw new Error("injected supplemental newPage rejection"); return {}; },
        close: async () => { closed.push(extra ? "supplemental" : "primary"); } };
    } },
    capture: async (_context, unit, captures = results) => {
      if (unit.supplementalViewport && failure === "emptyCapture") return;
      const rec = { ...unit, size: `${unit.v.size.width}x${unit.v.size.height}`, file: unit.supplementalViewport ? "1440.png" : "primary.png", ok: true, status: 200,
        counts: {}, groups: {}, blockingCounts: {}, blocking: 0, violations: [], issues: [] };
      captures.push(rec); return rec;
    },
    adminOrigin: "http://admin.example.invalid", HOST: "store.example.invalid", INIT_SCRIPT: "", monitor: () => ({}),
    supplementalViewportFor, foldSupplementalCapture, RULES, BLOCKING, BLOCKING_KINDS, assert, results, URL,
  };
  const adminTask = runInNewContext([
    source.match(/const sizeOf = .*;/)[0], source.match(/const ctxOptions = .*;/)[0],
    between("const newContext =", "const loginState ="), between("const adminTask =", "const bridge ="), "adminTask",
  ].join("\n"), transport);
  await adminTask({}, { id: "products-new", route: "/products/new", state: "new", auth: true, url: () => "http://admin.example.invalid/en/products/new" }, "en", VISUAL_VIEWPORTS[0])()
    .catch((error) => errors.push(error.message)); // same as pool(); primary-only missing coverage previously stayed green.
  assert.equal(results.length, 1, `primary capture must succeed before injecting: ${errors.join("; ")}`);
  const key = (r) => `${r.app}|${r.id}|${r.locale}|${r.size}`;
  const verdict = runInNewContext(`${between("const have =", "const report =")}\n({ exitCode, reasons, missing, stateFailures });`,
    { ...transport, key, expected: [results[0]], universe: [results[0]], exitCode: 0 });
  return { ...verdict, results, closed, errors };
}

test("G-UI9 fails closed when the required 1440 newContext rejects despite a successful primary shot", async (t) => {
  const r = await supplementalTaskVerdict("newContext");
  assert.equal(r.results.length, 1); assert.equal(r.results[0].file, "primary.png"); assert.equal(r.missing.length, 0);
  assert.equal(r.exitCode, 1, "G-UI9 must be RED even though the primary matrix is complete");
  assert.equal(r.stateFailures.length, 1);
  assert.ok(r.results[0].issues.some((s) => s.includes("1440x900") && s.includes("injected supplemental newContext rejection")), "explicit supplemental setup reason is retained");
  assert.equal(r.errors.length, 0, "failure is folded rather than swallowed by the task pool");
  assert.deepEqual(r.closed, ["primary"]);
  t.diagnostic(`Injected G-UI9 exit=${r.exitCode}: ${r.reasons.join("; ")}; ${r.results[0].issues.join("; ")}`);
});

test("supplemental setup/absent capture failures close allocated contexts and still fail G-UI9", async () => {
  for (const failure of ["newPage", "emptyCapture"]) {
    const r = await supplementalTaskVerdict(failure);
    assert.equal(r.exitCode, 1); assert.equal(r.stateFailures.length, 1); assert.equal(r.errors.length, 0);
    assert.ok(r.results[0].issues.some((s) => s.includes("1440x900")));
    assert.deepEqual(r.closed, ["primary", "supplemental"]);
  }
  const green = await supplementalTaskVerdict();
  assert.equal(green.exitCode, 0); assert.equal(green.results.length, 1);
  assert.equal(green.results[0].supplemental[0].file, "1440.png");
  assert.deepEqual(green.closed, ["primary", "supplemental"]);
});
