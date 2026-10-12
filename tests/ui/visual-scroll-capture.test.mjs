// Purpose: counterexamples for continuous native product-pane screenshot coverage.
// Depends on: the real captureInnerScrollFrames helper; a fake Playwright transport and temporary output.
// Used by: scripts/dev/test-node.sh; DB-free tests do not claim browser acceptance.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import path from "node:path";
import { tmpdir } from "node:os";
import test from "node:test";
import { captureInnerScrollFrames, supplementalViewportFor, foldSupplementalCapture } from "./visual-scroll-capture.mjs";
import { VISUAL_VIEWPORTS } from "./sweep-shard-lib.mjs";

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
