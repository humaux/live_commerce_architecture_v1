// Unit tests of the pure rules of the G-UI9 layout lint (tests/ui/visual-lint-lib.mjs) on synthetic rect fixtures: every rule is shown to flag its defect
// AND to stay quiet on the correct layout next to it, at and around its named threshold. Run by `test-local.sh --browser-visual-lint` before any build.
import assert from "node:assert/strict";
import test from "node:test";
import { T, BLOCKING, evaluate, inkRect, median, pathKey, r1Overflow, r2FormRows, r3Tables, r4SmallText, r5TapTargets, r6Overlap, r7Clipped, r8Duplicates, selectRecords, severityOf, stackedLines } from "./visual-lint-lib.mjs";

const rc = (x, y, w, h) => ({ x, y, w, h });
const snap = (extra = {}) => ({ viewport: { w: 1586, h: 992 }, doc: { scrollWidth: 1586, scrollHeight: 2000 }, ...extra });
// a labelled single-line field: label 20px high directly above a 40px control, both at x
let nextId = 100;
const field = (x, labelY, controlY, { w = 300, h = 40, multiline = false, parent = 1, layout = "grid", label = true } = {}) => ({
  id: nextId++, tag: "input", type: "text", multiline, rect: rc(x, controlY, w, h), label: label ? { id: nextId++, rect: rc(x, labelY, w, 20), text: `L${x}` } : null,
  controlParent: nextId++, rowParent: parent, rowLayout: layout,
});

test("thresholds are the named constants of the brief", () => {
  assert.deepEqual([T.ROW_LABEL_TOP_DELTA_PX, T.ROW_CONTROL_TOP_DELTA_PX, T.ROW_CONTROL_HEIGHT_DELTA_PX, T.ROW_SAME_BAND_PX], [4, 4, 4, 8]);
  assert.deepEqual([T.TABLE_ROW_RATIO, T.MIN_FONT_PX, T.TAP_TARGET_MIN_PX, T.OVERLAP_MIN_PX, T.CLIP_MIN_BOX_PX], [1.6, 12, 40, 2, 8]);
  assert.deepEqual([...BLOCKING], ["R1", "R2", "R3", "R6"]);
  assert.equal(severityOf("R1"), "block"); assert.equal(severityOf("R4"), "warn"); assert.equal(severityOf("R8"), "warn");
  assert.throws(() => { T.MIN_FONT_PX = 1; }, TypeError); // frozen: a rule cannot be loosened at run time
});

test("R1: document wider than the viewport, and the outermost element past the edge", () => {
  assert.equal(r1Overflow(snap()).length, 0);
  assert.equal(r1Overflow(snap({ doc: { scrollWidth: 1587 } })).length, 0, "1px of rounding is tolerated");
  const wide = r1Overflow(snap({ viewport: { w: 390, h: 844 }, doc: { scrollWidth: 520, scrollHeight: 3000 } }));
  assert.equal(wide.length, 1); assert.equal(wide[0].kind, "document-scroll-width"); assert.equal(wide[0].measured.overBy, 130);
  const boxes = [
    { id: 1, ...rc(0, 0, 600, 40), parent: -1 }, // 600 wide on a 390 viewport: the finding
    { id: 2, ...rc(10, 5, 580, 30), parent: 1 }, // inside it: not a second finding
    { id: 3, ...rc(-400, 0, 200, 20), parent: -1 }, // skip link parked off the left edge
    { id: 4, ...rc(400, 0, 300, 600), parent: -1, pinned: true }, // closed fixed drawer beyond the right edge
    { id: 5, ...rc(300, 100, 100, 20), parent: -1 }, // straddles the right edge by 10px
    { id: 6, ...rc(-5, 200, 100, 20), parent: -1 }, // straddles the left edge
  ];
  const v = r1Overflow(snap({ viewport: { w: 390, h: 844 }, doc: { scrollWidth: 390 }, boxes })).filter((x) => x.kind === "element-beyond-viewport");
  assert.deepEqual(v.map((x) => x.ids[0]), [1, 5, 6]);
  assert.equal(v[1].measured.overBy, 10);
});

test("R2: label tops, control tops and control heights of one visual row", () => {
  const ok = [field(0, 100, 124), field(320, 100, 124)];
  assert.equal(r2FormRows(snap({ fields: ok })).length, 0);
  // label+input beside label+textarea: the textarea row starts 6px lower, the input is 10px shorter
  const bad = [field(0, 100, 124, { h: 38 }), field(320, 106, 130, { h: 120, multiline: true })];
  const kinds = r2FormRows(snap({ fields: bad })).map((v) => v.kind).sort();
  assert.deepEqual(kinds, ["control-top", "label-top"]);
  const heights = r2FormRows(snap({ fields: [field(0, 100, 124, { h: 36 }), field(320, 100, 124, { h: 44 })] }));
  assert.deepEqual(heights.map((v) => v.kind), ["control-height"]);
  assert.equal(heights[0].measured.spread, 8);
  // exactly at the threshold is allowed, one pixel more is not
  assert.equal(r2FormRows(snap({ fields: [field(0, 100, 124), field(320, 104, 128)] })).length, 0);
  assert.equal(r2FormRows(snap({ fields: [field(0, 100, 124), field(320, 105, 129)] })).length, 2);
  // a textarea never counts for the single-line height rule
  assert.equal(r2FormRows(snap({ fields: [field(0, 100, 124, { h: 40 }), field(320, 100, 124, { h: 120, multiline: true })] })).length, 0);
});

test("R2: what is one row and what is not", () => {
  // grid siblings are one row however far apart their labels start (a hint line pushes the second label down 30px)
  const grid = r2FormRows(snap({ fields: [field(0, 100, 124, { parent: 7 }), field(900, 130, 154, { parent: 7 })] }));
  assert.ok(grid.some((v) => v.kind === "label-top"));
  // the same two fields in different, unrelated containers 900px apart are not a row
  assert.equal(r2FormRows(snap({ fields: [field(0, 100, 124, { parent: 7, layout: "block" }), field(900, 106, 130, { parent: 8, layout: "block" })] })).length, 0);
  // ...but side by side within the gap and within 8px of each other they are, whatever their parents
  assert.equal(r2FormRows(snap({ fields: [field(0, 100, 124, { parent: 7, layout: "block" }), field(340, 106, 130, { parent: 8, layout: "block" })] })).some((v) => v.kind === "label-top"), true);
  // stacked fields (one above the other) are different rows
  assert.equal(r2FormRows(snap({ fields: [field(0, 100, 124), field(0, 190, 214, { h: 60 })] })).length, 0);
});

test("R2: a button beside a field has to line up with its control", () => {
  const f = field(0, 100, 124, { h: 40, parent: 5, layout: "flex" });
  const btn = (y, h = 40) => ({ id: 900, rect: rc(320, y, 100, h), parent: 5, text: "Add" });
  assert.equal(r2FormRows(snap({ fields: [f], actions: [btn(124)] })).length, 0, "same top");
  assert.equal(r2FormRows(snap({ fields: [f], actions: [btn(130, 34)] })).length, 0, "same bottom");
  assert.equal(r2FormRows(snap({ fields: [f], actions: [btn(116, 56)] })).length, 0, "same centre");
  const floating = r2FormRows(snap({ fields: [f], actions: [btn(100, 20)] }));
  assert.equal(floating.length, 1); assert.equal(floating[0].kind, "action-offset"); assert.equal(floating[0].measured.deltaTop, 24);
  assert.equal(r2FormRows(snap({ fields: [f], actions: [{ ...btn(100, 20), parent: 99 }] })).length, 0, "a button of another container is not part of the row");
});

test("R3: a row made tall by stacked controls, a control wider than its cell", () => {
  const cell = (y, controls = [], extra = {}) => ({ id: nextId++, rect: rc(0, y, 120, 40), scrollW: 120, clientW: 120, controls, ...extra });
  const ctl = (y, w = 100, h = 30) => ({ id: nextId++, rect: rc(10, y, w, h) });
  const row = (y, h, cells, header = false) => ({ id: nextId++, rect: rc(0, y, 600, h), header, cells });
  const plain = (y) => row(y, 40, [cell(y, [ctl(y + 5, 100, 30)])]);
  const tall = row(160, 90, [cell(160, [ctl(162, 16, 16), ctl(190, 100, 30)])]); // checkbox above an input: two lines
  const table = { id: 1, rows: [row(0, 40, [], true), plain(40), plain(80), plain(120), tall] };
  const v = r3Tables(snap({ tables: [table] }));
  assert.equal(v.length, 1); assert.equal(v[0].kind, "row-tall");
  assert.equal(v[0].measured.ratio, 2.3); assert.equal(v[0].measured.stackedLines, 2);
  // a tall row without stacked controls (long wrapped text) is not this rule, and the header row does not move the median
  const wrapped = { id: 2, rows: [row(0, 200, [], true), plain(40), plain(80), plain(120), row(160, 90, [cell(160, [ctl(165, 100, 30)])])] };
  assert.equal(r3Tables(snap({ tables: [wrapped] })).length, 0);
  // two body rows: no median to speak of
  assert.equal(r3Tables(snap({ tables: [{ id: 3, rows: [plain(0), tall] }] })).length, 0);
  // overflow: the input is 150px wide in a 120px cell; also a cell whose content scrolls wider than the cell
  const out = r3Tables(snap({ tables: [{ id: 4, rows: [row(0, 40, [cell(0, [ctl(5, 150)])])] }] }));
  assert.equal(out.length, 1); assert.equal(out[0].kind, "cell-overflow"); assert.equal(out[0].measured.controlsOutside[0].width, 150);
  assert.equal(r3Tables(snap({ tables: [{ id: 5, rows: [row(0, 40, [cell(0, [ctl(5, 100)], { scrollW: 190 })])] }] })).length, 1);
  assert.equal(r3Tables(snap({ tables: [{ id: 6, rows: [row(0, 40, [cell(0, [ctl(5, 110)])])] }] })).length, 0, "110px in a 120px cell fits");
  assert.equal(stackedLines([{ rect: rc(0, 0, 10, 20) }, { rect: rc(20, 4, 10, 20) }]), 1, "side by side is one line");
  assert.equal(median([40, 40, 40, 90]), 40); assert.equal(median([1, 3]), 2);
});

test("R3: a matrix whose EVERY row is doubled by a stacked stock cell (the median cannot see it)", () => {
  const ctl = (x, y, w = 100, h = 40) => ({ id: nextId++, rect: rc(x, y, w, h) });
  const cell = (x, y, w, controls) => ({ id: nextId++, rect: rc(x, y, w, 110), scrollW: w, clientW: w, controls });
  // price cell: one 40px input; stock cell: checkbox on line one, 40px input on line two; the row is 110px, every row alike
  const row = (y) => ({ id: nextId++, rect: rc(0, y, 500, 110), header: false, cells: [cell(0, y, 100, []), cell(100, y, 140, [ctl(110, y + 8)]), cell(240, y, 160, [ctl(250, y + 8, 16, 16), ctl(250, y + 40, 140, 40)])] });
  const v = r3Tables(snap({ tables: [{ id: 1, rows: [{ id: nextId++, rect: rc(0, 0, 500, 36), header: true, cells: [] }, row(36), row(146), row(256)] }] }));
  assert.deepEqual(v.map((x) => x.kind), ["row-stacked", "row-stacked", "row-stacked"], "all three rows are doubled, none is an outlier");
  assert.equal(v[0].measured.tallestSingleLineControl, 40); assert.equal(v[0].measured.ratio, 2.8);
  // the same stack in a row only 1.5x the tallest single-line control is within the threshold
  const snug = { ...row(0), rect: rc(0, 0, 500, 60) };
  assert.equal(r3Tables(snap({ tables: [{ id: 2, rows: [snug, { ...snug, id: nextId++ }, { ...snug, id: nextId++ }] }] })).length, 0);
  // no single-line cell to compare with: nothing to say
  const alone = { id: nextId++, rect: rc(0, 0, 500, 110), header: false, cells: [cell(240, 0, 160, [ctl(250, 8, 16, 16), ctl(250, 40, 140, 40)])] };
  assert.equal(r3Tables(snap({ tables: [{ id: 3, rows: [alone, { ...alone, id: nextId++ }, { ...alone, id: nextId++ }] }] })).length, 0);
});

test("R4: small text, helper text apart, icons and duplicates excepted", () => {
  const tx = (id, size, extra = {}) => ({ id, size, helper: false, icon: false, text: "t", rects: [rc(0, id * 20, 50, 14)], ...extra });
  const v = r4SmallText(snap({ texts: [tx(1, 11.99), tx(2, 12), tx(3, 11, { helper: true }), tx(4, 8, { icon: true }), tx(1, 10), tx(5, 13)] }));
  assert.deepEqual(v.map((x) => [x.ids[0], x.kind]), [[1, "small-text"], [3, "small-helper"]]);
  assert.equal(v[0].measured.fontSize, 11.99);
});

test("R5: tap targets on the 390 viewport only", () => {
  const tg = (id, w, h, extra = {}) => ({ id, tag: "button", rect: rc(0, id * 50, w, h), inline: false, disabled: false, text: "x", ...extra });
  const s = snap({ targets: [tg(1, 39, 40), tg(2, 40, 40), tg(3, 100, 24), tg(4, 30, 20, { inline: true }), tg(5, 30, 20, { disabled: true })] });
  assert.deepEqual(r5TapTargets(s).map((v) => v.ids[0]), [1, 3]);
  assert.equal(evaluate(s, { mobile: false }).some((v) => v.rule === "R5"), false);
  assert.equal(evaluate(s, { mobile: true }).filter((v) => v.rule === "R5").length, 2);
});

test("R6: interactive elements and text boxes that overlap", () => {
  const tg = (id, x, y, w, h, extra = {}) => ({ id, tag: "button", rect: rc(x, y, w, h), inline: false, disabled: false, parent: -1, text: `b${id}`, ...extra });
  assert.equal(r6Overlap(snap({ targets: [tg(1, 0, 0, 100, 40), tg(2, 98, 0, 100, 40)] })).length, 0, "2px is allowed");
  const hit = r6Overlap(snap({ targets: [tg(1, 0, 0, 100, 40), tg(2, 96, 10, 100, 40)] }));
  assert.equal(hit.length, 1); assert.equal(hit[0].kind, "interactive-overlap"); assert.deepEqual([hit[0].measured.overlapWidth, hit[0].measured.overlapHeight], [4, 30]);
  assert.equal(r6Overlap(snap({ targets: [tg(1, 0, 0, 200, 40), tg(2, 10, 5, 50, 30, { parent: 1 })] })).length, 0, "nested controls are one control");
  assert.equal(r6Overlap(snap({ targets: [tg(1, 0, 0, 100, 40, { pinned: true }), tg(2, 50, 10, 100, 40)] })).length, 0, "a pinned bar over scrolling content is layering");
  assert.equal(r6Overlap(snap({ targets: [tg(1, 0, 0, 100, 40, { inline: true }), tg(2, 50, 10, 100, 40)] })).length, 0);
  // text: two lines of 14px type 20px apart do not overlap (the 17px content boxes would by leading, the 1em ink bands do not); 8px apart they do
  const t = (id, y, size = 14, extra = {}) => ({ id, size, icon: false, text: `t${id}`, rects: [rc(0, y, 120, 17)], ...extra });
  assert.equal(r6Overlap(snap({ texts: [t(1, 0), t(2, 14)] })).length, 0);
  const over = r6Overlap(snap({ texts: [t(1, 0), t(2, 6)] }));
  assert.equal(over.length, 1); assert.equal(over[0].kind, "text-overlap");
  assert.equal(r6Overlap(snap({ texts: [t(1, 0), t(1, 6)] })).length, 0, "lines of one element never overlap each other");
  assert.deepEqual(inkRect(rc(0, 0, 10, 17), 14), rc(0, 1.5, 10, 14));
});

test("R7: clipped labels, buttons and links", () => {
  const c = (id, scrollW, clientW) => ({ id, rect: rc(0, 0, clientW, 30), scrollW, clientW, text: "Save the whole configuration" });
  const v = r7Clipped(snap({ clips: [c(1, 200, 120), c(2, 121, 120), c(3, 122, 120)] }));
  assert.deepEqual(v.map((x) => x.ids[0]), [1, 3]);
  assert.equal(v[0].measured.hiddenPx, 80);
  // a visually-hidden (sr-only) label is a 1px box whose text is "wider" than it: not truncated text
  assert.equal(r7Clipped(snap({ clips: [{ id: 9, rect: rc(0, 0, 1, 1), scrollW: 70, clientW: 1, text: "Language" }, { id: 10, rect: rc(0, 0, 7, 20), scrollW: 70, clientW: 7, text: "x" }] })).length, 0);
  assert.equal(r7Clipped(snap({ clips: [{ id: 11, rect: rc(0, 0, 8, 20), scrollW: 70, clientW: 8, text: "Save all" }] })).length, 1);
});

test("R8: the same visible label twice in one list", () => {
  const list = (items) => ({ id: 1, rect: rc(0, 0, 200, 100), items: items.map((text, i) => ({ id: 10 + i, text })) });
  const v = r8Duplicates(snap({ lists: [list(["connect meta page", "add a product", "connect meta page"]), list(["a", "b", "c"])] }));
  assert.equal(v.length, 1); assert.equal(v[0].measured.label, "connect meta page"); assert.equal(v[0].measured.times, 2);
  assert.equal(r8Duplicates(snap({ lists: [list(["x1", "x1"]), { ...list(["y1", "y2"]), id: 2 }] })).length, 1);
});

test("evaluate() tags severity, selectRecords() keeps distinct groups first and counts every instance", () => {
  const s = snap({ doc: { scrollWidth: 2000 }, texts: Array.from({ length: 20 }, (_, i) => ({ id: i + 1, size: 10, helper: false, icon: false, text: "t", rects: [rc(0, i * 30, 40, 12)] })) });
  const v = evaluate(s);
  assert.deepEqual([...new Set(v.map((x) => x.rule))].sort(), ["R1", "R4"]);
  assert.equal(v.find((x) => x.rule === "R1").severity, "block"); assert.equal(v.find((x) => x.rule === "R4").severity, "warn");
  const pathFor = (x) => (x.rule === "R4" ? `ul > li:nth-of-type(${x.ids[0]})` : "document");
  const { picked, groupCounts } = selectRecords(v, pathFor);
  assert.equal(groupCounts.R4, 1, "twenty alike list items are one group");
  assert.equal(picked.filter((x) => x.rule === "R4").length, 1);
  assert.equal(v.filter((x) => x.rule === "R4").length, 20);
  assert.equal(pathKey("div.card:nth-of-type(3) > a[data-testid=\"order-12345\"]"), "div.card > a[data-testid=\"order-#\"]");
});
