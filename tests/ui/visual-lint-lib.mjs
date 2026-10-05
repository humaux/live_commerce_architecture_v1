// G-UI9 deterministic layout lint (unit ui-visual-audit, owner 2026-10-05: "UI layout is not acceptable, the visual review was never completed").
//
// Two halves, deliberately separate so the rules are testable without a browser:
//   collect()   runs IN THE PAGE (Playwright page.evaluate): reads getBoundingClientRect / getComputedStyle of visible elements and returns a plain
//               JSON snapshot. It measures only: it never writes to the page. Element ids index window.__vaEls so describe() can name them later.
//   evaluate()  pure Node: snapshot -> violations. Every threshold is a named constant in T below. tests/ui/visual-lint-lib.test.mjs feeds it synthetic
//               rect fixtures; tests/ui/visual-lint-canary.mjs feeds a real known-bad and a known-good HTML page through collect() + evaluate().
//
// Rules (blocking = exit code of `--browser-visual-lint`; warn = reported, exit stays 0 for now; severityOf() is per rule, R7 per kind):
//   R1 block  horizontal overflow          document scrollWidth > viewport, or a visible element extending past the viewport edge
//   R2 block  form-row misalignment        fields of one visual row: label tops / control tops / single-line control heights differ; a button off the row
//   R3 block  table row consistency        a row > 1.6x the table's median row height because a cell stacks controls (row-tall), a row doubled by a stacked
//                                          cell next to single-line cells (row-stacked: every row of the table may be doubled, so no median sees it); a control wider than its cell
//   R4 warn   min text size                visible text under 12px (kind small-helper for helper/counter text)
//   R5 warn   tap targets (390 only)       interactive element under 40x40 css px (inline text links inside a sentence excepted)
//   R6 block  overlap                      two interactive elements, or two text boxes, intersecting by more than 2px
//   R7 block  clipped text in a control    text of a button / select / tab cut by an ancestor (clipped-control), spilling out of its own box (text-overflows-control: centred
//                                          nowrap text runs out on BOTH sides, which scrollWidth cannot see) or a select that cannot show its value (select-value-clipped);
//                                          labels and links (clipped-text, text-overflows-label) and text cut only by a scroll frame (cut-by-scroll-frame: reachable by
//                                          scrolling that box, cut at rest) are WARN
//   R8 warn   duplicate list labels        the same visible label twice inside one list / checklist container
//   R9 block  edge padding (390 only)      visible text or a control whose left/right edge is under 8px from the viewport edge (content flush to the screen)
//   R10 warn  fixed-bar occlusion          after scrolling to the middle and to the bottom, a fixed/sticky bar covers main content that scrolling cannot clear
//   R11 warn  narrow control               input / select / textarea narrower than a usable minimum (96px at 1586, 64px at 390)

// ---- thresholds: named, never tuned to make a finding disappear -----------------------------------------------------------------------------------------------
export const T = Object.freeze({
  OVERFLOW_TOLERANCE_PX: 1, // R1: sub-pixel rounding
  ROW_SAME_BAND_PX: 8, // R2: label boxes that start within this vertical distance are "one visual row"
  ROW_MAX_GAP_PX: 96, // R2: ...and are side by side (horizontal gap at most this) so two unrelated columns are not one row
  ROW_LABEL_TOP_DELTA_PX: 4, // R2: label tops of one row may differ by at most this
  ROW_CONTROL_TOP_DELTA_PX: 4, // R2: control top edges
  ROW_CONTROL_HEIGHT_DELTA_PX: 4, // R2: single-line control heights
  ROW_ACTION_DELTA_PX: 4, // R2: a button beside a field must share its top, bottom or centre within this
  TABLE_ROW_RATIO: 1.6, // R3: row height / median body-row height
  TABLE_MIN_BODY_ROWS: 3, // R3: a median of fewer rows says nothing
  CELL_OVERFLOW_PX: 1, // R3: a control sticking out of its cell by more than this
  MIN_FONT_PX: 12, // R4
  TAP_TARGET_MIN_PX: 40, // R5 (390 viewport)
  OVERLAP_MIN_PX: 2, // R6: both axes must intersect by more than this
  CLIP_TOLERANCE_PX: 1, // R7
  CLIP_MIN_BOX_PX: 8, // R7: a box narrower than this is a visually-hidden (sr-only) label, not truncated text
  SELECT_ARROW_PX: 20, // R7: room a native <select> keeps for its arrow when appearance is not none (a custom arrow lives in the padding)
  EDGE_MIN_PX: 8, // R9 (390): content closer to the left/right viewport edge than this is flush
  FULL_BLEED_SLACK_PX: 2, // R9: a button / link this close to the full viewport width is a full-bleed bar, not content
  OCCLUSION_COVER: 0.5, // R10: share of a visible text line / control a fixed bar must cover
  EDGE_ATTACH_PX: 2, // R10: a bar this close to the top or bottom viewport edge is attached to it
  CONTROL_MIN_WIDTH_DESKTOP_PX: 96, // R11 (1586)
  CONTROL_MIN_WIDTH_MOBILE_PX: 64, // R11 (390)
  MAX_RECORDS_PER_RULE: 5, // reporting: instances recorded (with crop) per rule per page; counts always cover every instance
});
export const RULES = Object.freeze({
  R1: "horizontal overflow", R2: "form-row misalignment", R3: "table row consistency", R4: "min text size", R5: "tap target (390)", R6: "overlap",
  R7: "clipped text", R8: "duplicate list label", R9: "edge padding (390)", R10: "fixed-bar occlusion", R11: "narrow control",
});
export const BLOCKING = Object.freeze(["R1", "R2", "R3", "R6", "R9"]); // whole rules; R7 blocks per kind below
export const BLOCKING_KINDS = Object.freeze(["R7:clipped-control", "R7:text-overflows-control", "R7:select-value-clipped"]);
export const severityOf = (rule, kind = "") => (BLOCKING.includes(rule) || BLOCKING_KINDS.includes(`${rule}:${kind}`) ? "block" : "warn");

// ---- geometry (pure) --------------------------------------------------------------------------------------------------------------------------------------------
const right = (r) => r.x + r.w, bottom = (r) => r.y + r.h;
export const overlap = (a, b) => ({ w: Math.min(right(a), right(b)) - Math.max(a.x, b.x), h: Math.min(bottom(a), bottom(b)) - Math.max(a.y, b.y) });
export const union = (rects) => {
  const x = Math.min(...rects.map((r) => r.x)), y = Math.min(...rects.map((r) => r.y));
  return { x, y, w: Math.max(...rects.map(right)) - x, h: Math.max(...rects.map(bottom)) - y };
};
export const median = (xs) => { const s = [...xs].sort((a, b) => a - b); const m = s.length >> 1; return s.length === 0 ? 0 : s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2; };
export const spread = (xs) => (xs.length ? Math.max(...xs) - Math.min(...xs) : 0);
const contains = (a, b) => a.x <= b.x + 0.5 && a.y <= b.y + 0.5 && right(a) >= right(b) - 0.5 && bottom(a) >= bottom(b) - 0.5;
const r1 = (n) => Math.round(n * 10) / 10;
// The ink box of a text rect: Range rects are the font's content area (taller than the glyphs), so stacked lines of tight line-height would "overlap" by
// leading alone. Compare the 1em band centred in the rect instead.
export const inkRect = (rect, fontSize) => (rect.h > fontSize ? { x: rect.x, y: rect.y + (rect.h - fontSize) / 2, w: rect.w, h: fontSize } : rect);

// ---- R1 horizontal overflow --------------------------------------------------------------------------------------------------------------------------------------
export function r1Overflow(s, t = T) {
  const out = [], vw = s.viewport.w;
  if (s.doc.scrollWidth > vw + t.OVERFLOW_TOLERANCE_PX)
    out.push({ rule: "R1", kind: "document-scroll-width", ids: [], rect: { x: 0, y: 0, w: s.doc.scrollWidth, h: Math.min(s.doc.scrollHeight ?? 900, 900) }, measured: { scrollWidth: s.doc.scrollWidth, viewportWidth: vw, overBy: s.doc.scrollWidth - vw } });
  const over = (s.boxes ?? []).filter((b) => {
    if (right(b) <= 0) return false; // wholly off the left edge: a skip link or an off-canvas panel, never reachable by scrolling
    if (b.pinned && b.x >= vw) return false; // a closed fixed drawer parked beyond the right edge
    return right(b) > vw + t.OVERFLOW_TOLERANCE_PX || b.x < -t.OVERFLOW_TOLERANCE_PX;
  });
  const overIds = new Set(over.map((b) => b.id));
  for (const b of over) {
    if (overIds.has(b.parent)) continue; // the outermost box that overflows is the finding, not every child inside it
    out.push({ rule: "R1", kind: "element-beyond-viewport", ids: [b.id], rect: { x: b.x, y: b.y, w: b.w, h: b.h }, measured: { left: r1(b.x), right: r1(right(b)), viewportWidth: vw, overBy: r1(Math.max(right(b) - vw, -b.x)) } });
  }
  return out;
}

// ---- R2 form-row misalignment -----------------------------------------------------------------------------------------------------------------------------------
export function r2FormRows(s, t = T) {
  const f = s.fields ?? [], out = [];
  const box = (x) => (x.label ? union([x.label.rect, x.rect]) : x.rect);
  const labelTop = (x) => (x.label ? x.label.rect.y : x.rect.y);
  const parent = f.map((_, i) => i);
  const find = (i) => { while (parent[i] !== i) { parent[i] = parent[parent[i]]; i = parent[i]; } return i; };
  for (let i = 0; i < f.length; i++) for (let j = i + 1; j < f.length; j++) {
    const a = f[i], b = f[j], ba = box(a), bb = box(b);
    if (contains(ba, bb) || contains(bb, ba)) continue;
    const gap = Math.max(bb.x - right(ba), ba.x - right(bb));
    if (gap < -2) continue; // not side by side
    const siblings = a.rowParent === b.rowParent && (a.rowLayout === "grid" || a.rowLayout === "flex") && overlap(ba, bb).h > 0;
    const band = Math.abs(labelTop(a) - labelTop(b)) <= t.ROW_SAME_BAND_PX && gap <= t.ROW_MAX_GAP_PX;
    if (siblings || band) parent[find(i)] = find(j);
  }
  const rows = new Map();
  f.forEach((x, i) => { const k = find(i); (rows.get(k) ?? rows.set(k, []).get(k)).push(x); });
  for (const members of rows.values()) {
    if (members.length < 2) continue;
    const rect = union(members.map(box)), ids = members.map((m) => m.id);
    const labelTops = members.filter((m) => m.label).map((m) => r1(m.label.rect.y));
    const controlTops = members.map((m) => r1(m.rect.y));
    const singles = members.filter((m) => !m.multiline);
    const heights = singles.map((m) => r1(m.rect.h));
    const measured = { labelTops, controlTops, singleLineHeights: heights, fields: members.map((m) => m.label?.text ?? m.id) };
    if (labelTops.length >= 2 && spread(labelTops) > t.ROW_LABEL_TOP_DELTA_PX) out.push({ rule: "R2", kind: "label-top", ids, rect, measured: { ...measured, spread: r1(spread(labelTops)) } });
    if (spread(controlTops) > t.ROW_CONTROL_TOP_DELTA_PX) out.push({ rule: "R2", kind: "control-top", ids, rect, measured: { ...measured, spread: r1(spread(controlTops)) } });
    if (heights.length >= 2 && spread(heights) > t.ROW_CONTROL_HEIGHT_DELTA_PX) out.push({ rule: "R2", kind: "control-height", ids: singles.map((m) => m.id), rect, measured: { ...measured, spread: r1(spread(heights)) } });
  }
  // a button that sits beside the fields of a row must line up with their controls (top, bottom or centre), or it "floats at a third height"
  for (const a of s.actions ?? []) {
    const refs = f.filter((m) => !m.multiline && (m.controlParent === a.parent || m.rowParent === a.parent) && overlap(box(m), a.rect).h > 0 && overlap(m.rect, a.rect).w <= 0);
    if (!refs.length) continue;
    const dist = (m) => ({ top: Math.abs(a.rect.y - m.rect.y), bottom: Math.abs(bottom(a.rect) - bottom(m.rect)), centre: Math.abs(a.rect.y + a.rect.h / 2 - (m.rect.y + m.rect.h / 2)) });
    const best = refs.map((m) => ({ m, d: dist(m) })).sort((p, q) => Math.min(...Object.values(p.d)) - Math.min(...Object.values(q.d)))[0];
    if (Math.min(...Object.values(best.d)) > t.ROW_ACTION_DELTA_PX)
      out.push({ rule: "R2", kind: "action-offset", ids: [a.id, best.m.id], rect: union([a.rect, best.m.rect]), measured: { actionTop: r1(a.rect.y), actionHeight: r1(a.rect.h), controlTop: r1(best.m.rect.y), controlHeight: r1(best.m.rect.h), deltaTop: r1(best.d.top), deltaBottom: r1(best.d.bottom), deltaCentre: r1(best.d.centre) } });
  }
  return out;
}

// ---- R3 table row consistency -----------------------------------------------------------------------------------------------------------------------------------
// Controls of a cell are stacked when they occupy two or more separate lines (vertical extents that do not overlap).
export function stackedLines(controls) {
  const lines = [];
  for (const c of [...controls].sort((a, b) => a.rect.y - b.rect.y)) {
    const last = lines[lines.length - 1];
    if (last && c.rect.y < last.bottom - 1) last.bottom = Math.max(last.bottom, bottom(c.rect));
    else lines.push({ bottom: bottom(c.rect) });
  }
  return lines.length;
}
export function r3Tables(s, t = T) {
  const out = [];
  for (const table of s.tables ?? []) {
    const body = table.rows.filter((r) => !r.header);
    const med = median(body.map((r) => r.rect.h));
    for (const row of body) {
      const stacked = row.cells.map((c) => ({ c, lines: stackedLines(c.controls) })).filter((x) => x.lines >= 2).sort((a, b) => b.lines - a.lines)[0];
      if (!stacked) continue;
      if (body.length >= t.TABLE_MIN_BODY_ROWS && row.rect.h > t.TABLE_ROW_RATIO * med)
        out.push({ rule: "R3", kind: "row-tall", ids: [row.id, ...stacked.c.controls.map((c) => c.id)], rect: row.rect, measured: { rowHeight: r1(row.rect.h), medianRowHeight: r1(med), ratio: r1(row.rect.h / med), stackedControls: stacked.c.controls.length, stackedLines: stacked.lines } });
      else {
        // every row of a variant matrix can be doubled the same way, which the median cannot see: compare with the tallest single-line control of the row
        const single = row.cells.filter((c) => c.controls.length && stackedLines(c.controls) < 2).flatMap((c) => c.controls);
        const base = single.length ? Math.max(...single.map((c) => c.rect.h)) : 0;
        if (base && row.rect.h > t.TABLE_ROW_RATIO * base)
          out.push({ rule: "R3", kind: "row-stacked", ids: [row.id, ...stacked.c.controls.map((c) => c.id)], rect: row.rect, measured: { rowHeight: r1(row.rect.h), tallestSingleLineControl: r1(base), ratio: r1(row.rect.h / base), stackedControls: stacked.c.controls.length, stackedLines: stacked.lines, medianRowHeight: r1(med) } });
      }
    }
    for (const row of table.rows) for (const cell of row.cells) {
      const outside = cell.controls.filter((c) => right(c.rect) > right(cell.rect) + t.CELL_OVERFLOW_PX || c.rect.x < cell.rect.x - t.CELL_OVERFLOW_PX);
      const scrollOver = cell.controls.length > 0 && cell.scrollW > cell.clientW + t.CELL_OVERFLOW_PX;
      if (outside.length || scrollOver)
        out.push({ rule: "R3", kind: "cell-overflow", ids: [cell.id, ...outside.map((c) => c.id)], rect: cell.rect, measured: { cellWidth: r1(cell.rect.w), cellScrollWidth: cell.scrollW, cellClientWidth: cell.clientW, controlsOutside: outside.map((c) => ({ id: c.id, left: r1(c.rect.x), right: r1(right(c.rect)), width: r1(c.rect.w) })), cellLeft: r1(cell.rect.x), cellRight: r1(right(cell.rect)) } });
    }
  }
  return out;
}

// ---- R4 min text size ---------------------------------------------------------------------------------------------------------------------------------------------
export function r4SmallText(s, t = T) {
  const out = [], seen = new Set();
  for (const x of s.texts ?? []) {
    if (x.icon || !(x.size < t.MIN_FONT_PX) || seen.has(x.id)) continue;
    seen.add(x.id);
    out.push({ rule: "R4", kind: x.helper ? "small-helper" : "small-text", ids: [x.id], rect: x.rects[0], measured: { fontSize: x.size, text: x.text } });
  }
  return out;
}

// ---- R5 tap targets (the caller applies it to the 390 viewport only) ---------------------------------------------------------------------------------------
export function r5TapTargets(s, t = T) {
  const out = [];
  for (const x of s.targets ?? []) {
    if (x.inline || x.disabled) continue;
    if (x.rect.w < t.TAP_TARGET_MIN_PX || x.rect.h < t.TAP_TARGET_MIN_PX)
      out.push({ rule: "R5", kind: "tap-target", ids: [x.id], rect: x.rect, measured: { width: r1(x.rect.w), height: r1(x.rect.h), tag: x.tag, text: x.text } });
  }
  return out;
}

// ---- R6 overlap ----------------------------------------------------------------------------------------------------------------------------------------------------
export function r6Overlap(s, t = T) {
  const out = [];
  const targets = (s.targets ?? []).filter((x) => !x.inline && !x.disabled).sort((a, b) => a.rect.x - b.rect.x);
  const byId = new Map(targets.map((x) => [x.id, x]));
  const related = (a, b) => { for (const [p, q] of [[a, b], [b, a]]) for (let n = q.parent, guard = 0; n !== undefined && n !== -1 && guard < 50; n = byId.get(n)?.parent, guard++) if (n === p.id) return true; return false; };
  for (let i = 0; i < targets.length; i++) for (let j = i + 1; j < targets.length && targets[j].rect.x < right(targets[i].rect) - t.OVERLAP_MIN_PX; j++) {
    const a = targets[i], b = targets[j];
    if (!!a.pinned !== !!b.pinned || related(a, b)) continue; // a pinned bar over scrolling content is a layering by design
    const o = overlap(a.rect, b.rect);
    if (o.w > t.OVERLAP_MIN_PX && o.h > t.OVERLAP_MIN_PX)
      out.push({ rule: "R6", kind: "interactive-overlap", ids: [a.id, b.id], rect: { x: Math.max(a.rect.x, b.rect.x), y: Math.max(a.rect.y, b.rect.y), w: r1(o.w), h: r1(o.h) }, measured: { overlapWidth: r1(o.w), overlapHeight: r1(o.h), a: { text: a.text, tag: a.tag, rect: a.rect }, b: { text: b.text, tag: b.tag, rect: b.rect } } });
  }
  const lines = [];
  for (const x of s.texts ?? []) if (!x.icon) for (const r of x.rects) lines.push({ id: x.id, text: x.text, pinned: !!x.pinned, rect: inkRect(r, x.size), raw: r });
  lines.sort((a, b) => a.rect.y - b.rect.y);
  for (let i = 0; i < lines.length; i++) for (let j = i + 1; j < lines.length && lines[j].rect.y < bottom(lines[i].rect) - t.OVERLAP_MIN_PX; j++) {
    const a = lines[i], b = lines[j];
    if (a.id === b.id || a.pinned !== b.pinned) continue;
    const o = overlap(a.rect, b.rect);
    if (o.w > t.OVERLAP_MIN_PX && o.h > t.OVERLAP_MIN_PX)
      out.push({ rule: "R6", kind: "text-overlap", ids: [a.id, b.id], rect: union([a.raw, b.raw]), measured: { overlapWidth: r1(o.w), overlapHeight: r1(o.h), a: a.text, b: b.text } });
  }
  return out;
}

// ---- R7 clipped text in controls ---------------------------------------------------------------------------------------------------------------------------
// textClips: a text line of a button / tab (control) or of a label / link / th (label) whose rendered box sticks out of the box that should hold it, either side
// (hiddenPx = the part outside): mode "hidden" = an overflow hidden/clip ancestor cuts it, "spill" = it runs out of its own control/label box (neighbours then paint over
// it: 'Add offer' reads 'dd offe'), "scroll" = only an overflow auto/scroll frame cuts it. selects: the selected option's text width against what the select can show.
export function r7Clipped(s, t = T) {
  const out = [];
  for (const c of s.textClips ?? []) {
    if (c.rect.w < t.CLIP_MIN_BOX_PX || c.hiddenPx <= t.CLIP_TOLERANCE_PX) continue; // a box under 8px is a visually-hidden label
    const kind = c.mode === "scroll" ? "cut-by-scroll-frame" : c.mode === "spill" ? (c.control ? "text-overflows-control" : "text-overflows-label") : c.control ? "clipped-control" : "clipped-text";
    out.push({ rule: "R7", kind, ids: [c.id], rect: c.textRect, measured: { hiddenPx: r1(c.hiddenPx), boxWidth: r1(c.rect.w), textWidth: r1(c.textRect.w), text: c.text } });
  }
  for (const x of s.selects ?? []) {
    const available = x.clientW - x.padL - x.padR - (x.nativeArrow ? t.SELECT_ARROW_PX : 0);
    if (x.clientW >= t.CLIP_MIN_BOX_PX && x.textW > available + t.CLIP_TOLERANCE_PX)
      out.push({ rule: "R7", kind: "select-value-clipped", ids: [x.id], rect: x.rect, measured: { valueWidth: r1(x.textW), availableWidth: r1(Math.max(0, available)), selectWidth: r1(x.rect.w), text: x.text } });
  }
  return out;
}

// ---- R8 duplicate list labels ---------------------------------------------------------------------------------------------------------------------------------------
export function r8Duplicates(s) {
  const out = [];
  for (const list of s.lists ?? []) {
    const groups = new Map();
    for (const it of list.items) (groups.get(it.text) ?? groups.set(it.text, []).get(it.text)).push(it);
    for (const [text, items] of groups)
      if (items.length > 1) out.push({ rule: "R8", kind: "duplicate-label", ids: [list.id, ...items.map((i) => i.id)], rect: list.rect, measured: { label: text, times: items.length, listItems: list.items.length } });
  }
  return out;
}

// ---- R9 edge padding (390 only: the caller applies it to the mobile viewport) -----------------------------------------------------------------------------
// Text and controls flush to the left/right edge of the screen. A button / link that spans the whole width is a full-bleed bar, an image or a background is not
// text: neither is content. A margin below zero is overflow (R1), not padding.
export function r9EdgePadding(s, t = T) {
  const out = [], vw = s.viewport.w, seen = new Set();
  const margins = (r) => ({ left: r.x, right: vw - right(r) });
  const flush = (m) => Math.min(m.left, m.right) < t.EDGE_MIN_PX && Math.min(m.left, m.right) >= -t.OVERFLOW_TOLERANCE_PX;
  for (const x of s.texts ?? []) {
    if (x.icon || seen.has(x.id)) continue;
    const line = x.rects.find((r) => flush(margins(r)));
    if (!line) continue;
    seen.add(x.id);
    const m = margins(line);
    out.push({ rule: "R9", kind: "text-flush", ids: [x.id], rect: line, measured: { leftMargin: r1(m.left), rightMargin: r1(m.right), text: x.text } });
  }
  for (const x of s.targets ?? []) {
    if (x.inline || x.disabled || seen.has(x.id)) continue;
    const fullBleed = x.rect.w >= vw - t.FULL_BLEED_SLACK_PX;
    if (fullBleed && !/^(input|select|textarea)$/.test(x.tag)) continue; // a full-width bar / button is full-bleed by design, a full-width field has no gutter
    const m = margins(x.rect);
    if (!flush(m)) continue;
    seen.add(x.id);
    out.push({ rule: "R9", kind: "control-flush", ids: [x.id], rect: x.rect, measured: { leftMargin: r1(m.left), rightMargin: r1(m.right), tag: x.tag, text: x.text } });
  }
  return out;
}

// ---- R10 fixed-bar occlusion ---------------------------------------------------------------------------------------------------------------------------------
// s = a snapshot taken with the page scrolled to `position` ("middle" | "bottom"); s.bars = visible fixed/sticky boxes with an opaque background, s.scrollY.
// A bar attached to the top or bottom viewport edge only passes over content while scrolling: at the middle it is not judged; at the bottom, where scrolling
// can no longer clear it, a bar attached to the bottom edge is. A floating pinned box (a side panel, a widget) is judged at both. What is covered is main
// content: text lines and controls that are not themselves pinned and are mostly inside the viewport.
export function r10Occlusion(s, position, t = T) {
  const out = [], sy = s.scrollY ?? 0, vh = s.viewport.h, view = { x: 0, y: sy, w: s.viewport.w, h: vh };
  const area = (r) => Math.max(0, r.w) * Math.max(0, r.h);
  const inter = (a, b) => { const o = overlap(a, b); return o.w > 0 && o.h > 0 ? { x: Math.max(a.x, b.x), y: Math.max(a.y, b.y), w: o.w, h: o.h } : null; };
  const content = [];
  for (const x of s.texts ?? []) if (!x.icon && !x.pinned) for (const r of x.rects) content.push({ id: x.id, kind: "text", text: x.text, rect: r });
  for (const x of s.targets ?? []) if (!x.pinned && !x.disabled) content.push({ id: x.id, kind: "control", text: x.text, rect: x.rect });
  for (const bar of s.bars ?? []) {
    const top = bar.rect.y - sy <= t.EDGE_ATTACH_PX, bot = sy + vh - bottom(bar.rect) <= t.EDGE_ATTACH_PX;
    if (position === "middle" && (top || bot)) continue;
    if (position === "bottom" && top && !bot) continue;
    for (const c of content) {
      const visible = inter(c.rect, view);
      if (!visible || area(visible) < 0.5 * area(c.rect)) continue;
      const hit = inter(visible, bar.rect);
      if (hit && area(hit) >= t.OCCLUSION_COVER * area(visible))
        out.push({ rule: "R10", kind: c.kind === "text" ? "bar-covers-text" : "bar-covers-control", ids: [bar.id, c.id], rect: hit, measured: { position, bar: bar.rect, covered: r1(area(hit) / area(visible)), what: c.text, attached: top ? "top" : bot ? "bottom" : "floating" } });
    }
  }
  return out.map((v) => ({ ...v, severity: severityOf(v.rule, v.kind) }));
}

// ---- R11 narrow control --------------------------------------------------------------------------------------------------------------------------------------
export function r11NarrowControls(s, { mobile = false } = {}, t = T) {
  const min = mobile ? t.CONTROL_MIN_WIDTH_MOBILE_PX : t.CONTROL_MIN_WIDTH_DESKTOP_PX, out = [];
  for (const f of s.fields ?? []) {
    if (f.type === "number" || /^(numeric|decimal)$/.test(f.inputmode ?? "") || f.rect.w >= min) continue; // a quantity field is short by nature
    out.push({ rule: "R11", kind: "control-too-narrow", ids: [f.id], rect: f.rect, measured: { width: r1(f.rect.w), minimum: min, tag: f.tag, type: f.type, label: f.label?.text ?? "" } });
  }
  return out;
}

// ---- all rules -------------------------------------------------------------------------------------------------------------------------------------------------------
export function evaluate(snapshot, { mobile = false } = {}, t = T) {
  return [...r1Overflow(snapshot, t), ...r2FormRows(snapshot, t), ...r3Tables(snapshot, t), ...r4SmallText(snapshot, t), ...(mobile ? r5TapTargets(snapshot, t) : []), ...r6Overlap(snapshot, t), ...r7Clipped(snapshot, t), ...r8Duplicates(snapshot), ...(mobile ? r9EdgePadding(snapshot, t) : []), ...r11NarrowControls(snapshot, { mobile }, t)]
    .map((v) => ({ ...v, severity: severityOf(v.rule, v.kind) }));
}

// ---- scroll passes (Playwright side, shared by the runner and the canary) -------------------------------------------------------------------------------------
export const SCROLL_POSITIONS = Object.freeze([["middle", 0.5], ["bottom", 1]]);
// Scrolls the document to a fraction of its scrollable height and lets sticky/fixed boxes settle. false = the page does not scroll (nothing to judge).
export async function scrollToFraction(page, fraction) {
  const scrolls = await page.evaluate((f) => { const max = document.documentElement.scrollHeight - window.innerHeight; if (max <= 8) return false; window.scrollTo(0, Math.round(max * f)); return true; }, fraction); // [READ/MEASURE] scrolls only
  if (scrolls) await page.waitForTimeout(150);
  return scrolls;
}

// ---- reporting helpers (pure) -------------------------------------------------------------------------------------------------------------------------------------
// A path key without sibling positions / ids / numbers: instances of the same component share one group.
export const pathKey = (p) => String(p ?? "").replace(/:nth-of-type\(\d+\)/g, "").replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "#").replace(/\d+/g, "#");
// The first MAX_RECORDS_PER_RULE instances per rule, distinct groups first, so a page with 200 alike chips still shows its other findings.
export function selectRecords(violations, pathFor, t = T) {
  const picked = [], perRule = {}, groups = {};
  const seen = new Set();
  for (const v of violations) {
    const g = `${v.rule}|${v.kind}|${pathKey(pathFor(v))}`;
    v.group = g; groups[v.rule] = groups[v.rule] ?? new Set(); groups[v.rule].add(g);
    if (seen.has(g)) continue;
    seen.add(g);
    if ((perRule[v.rule] = (perRule[v.rule] ?? 0) + 1) <= t.MAX_RECORDS_PER_RULE) picked.push(v);
  }
  return { picked, groupCounts: Object.fromEntries(Object.entries(groups).map(([k, v]) => [k, v.size])) };
}
export const countByRule = (violations) => { const c = {}; for (const v of violations) c[v.rule] = (c[v.rule] ?? 0) + 1; return c; };

// ---- in-page collector ---------------------------------------------------------------------------------------------------------------------------------------------
// Serialised by Playwright (page.evaluate(collect)): self-contained, read-only. Coordinates are document coordinates (viewport rect + scroll offset).
export function collect() {
  const sx = window.scrollX, sy = window.scrollY, doc = document.documentElement;
  const vw = doc.clientWidth, vh = window.innerHeight;
  const els = (window.__vaEls = []);
  const ids = new Map();
  const reg = (el) => { let i = ids.get(el); if (i === undefined) { i = els.length; els.push(el); ids.set(el, i); } return i; };
  const R = (r) => ({ x: Math.round((r.left + sx) * 10) / 10, y: Math.round((r.top + sy) * 10) / 10, w: Math.round(r.width * 10) / 10, h: Math.round(r.height * 10) / 10 });
  const cssCache = new Map();
  const cs = (el) => { let s = cssCache.get(el); if (!s) { s = getComputedStyle(el); cssCache.set(el, s); } return s; };
  const SKIP = new Set(["SCRIPT", "STYLE", "NOSCRIPT", "TEMPLATE", "HEAD", "META", "LINK", "TITLE", "BASE", "OPTION", "OPTGROUP", "DATALIST", "TEXTAREA", "BR", "WBR"]);
  const inter = (a, b) => { if (!a) return b; if (!b) return a; const l = Math.max(a.l, b.l), t = Math.max(a.t, b.t), r = Math.min(a.r, b.r), bt = Math.min(a.b, b.b); return r > l && bt > t ? { l, t, r, b: bt } : { l: 0, t: 0, r: 0, b: 0 }; };
  // clip a descendant is subject to: the padding boxes of its overflow-clipping ancestors, through its containing-block chain (an absolutely positioned
  // child escapes the clip of ancestors between it and its containing block, a fixed one escapes all). html/body overflow belongs to the viewport.
  // hard = only overflow hidden/clip counts (content past it is gone); soft also counts auto/scroll (content past it is reachable by scrolling that box).
  const clipper = (hard) => {
    const clips = (v) => (hard ? v === "hidden" || v === "clip" : v !== "visible");
    const selfClip = (el) => {
      if (el === document.body || el === doc) return null;
      const st = cs(el);
      const cx = clips(st.overflowX), cy = clips(st.overflowY);
      if (st.display === "inline" || (!cx && !cy)) return null;
      const r = el.getBoundingClientRect();
      return { l: cx ? r.left + el.clientLeft : -Infinity, t: cy ? r.top + el.clientTop : -Infinity, r: cx ? r.left + el.clientLeft + el.clientWidth : Infinity, b: cy ? r.top + el.clientTop + el.clientHeight : Infinity };
    };
    const childMemo = new Map(), memo = new Map();
    const childClip = (el) => {
      if (!el) return null;
      if (childMemo.has(el)) return childMemo.get(el);
      const c = inter(clipOf(el), selfClip(el));
      childMemo.set(el, c);
      return c;
    };
    const clipOf = (el) => {
      if (memo.has(el)) return memo.get(el);
      const p = el.parentElement, pos = cs(el).position;
      const c = !p ? null : pos === "fixed" ? null : pos === "absolute" ? childClip(el.offsetParent && el.offsetParent !== document.body ? el.offsetParent : null) : childClip(p);
      memo.set(el, c);
      return c;
    };
    return { clipOf, childClip };
  };
  const { clipOf, childClip } = clipper(false);
  const hardClip = clipper(true).childClip;
  const pinnedMemo = new Map();
  const pinned = (el) => {
    if (!el || el === document.body) return false;
    if (pinnedMemo.has(el)) return pinnedMemo.get(el);
    const pos = cs(el).position, v = pos === "fixed" || pos === "sticky" || pinned(el.parentElement);
    pinnedMemo.set(el, v);
    return v;
  };
  // visible rect (viewport coords) of an element, or null: rendered, not hidden, not clipped to nothing.
  const visMemo = new Map();
  const vis = (el) => {
    if (visMemo.has(el)) return visMemo.get(el);
    let out = null;
    const ok = typeof el.checkVisibility === "function" ? el.checkVisibility({ checkOpacity: true, checkVisibilityCSS: true }) : cs(el).visibility !== "hidden" && cs(el).display !== "none";
    if (ok) {
      const r = el.getBoundingClientRect();
      if (r.width > 0 && r.height > 0) {
        const c = inter({ l: r.left, t: r.top, r: r.right, b: r.bottom }, clipOf(el));
        if (c.r - c.l > 0.5 && c.b - c.t > 0.5) out = { raw: r, clip: c };
      }
    }
    visMemo.set(el, out);
    return out;
  };
  const norm = (s) => (s || "").replace(/\s+/g, " ").trim();
  const all = [...document.body.querySelectorAll("*")].filter((el) => !SKIP.has(el.tagName) && !(el instanceof SVGElement && el.tagName !== "svg"));

  // boxes: visible elements touching or past a vertical viewport edge (a superset of what R1 flags; r1Overflow applies the tolerance)
  const boxes = [], boxIds = new Set();
  for (const el of all) {
    const v = vis(el);
    if (!v) continue;
    const c = v.clip;
    if (c.r <= vw - 0.5 && c.l >= 0.5) continue;
    let p = el.parentElement, parent = -1;
    for (; p && p !== document.body; p = p.parentElement) if (boxIds.has(p)) { parent = ids.get(p); break; }
    const id = reg(el);
    boxIds.add(el);
    boxes.push({ id, ...R({ left: c.l, top: c.t, width: c.r - c.l, height: c.b - c.t }), parent, pinned: pinned(el) });
  }

  // fields + actions (R2)
  const FIELD = 'input:not([type="hidden"]):not([type="checkbox"]):not([type="radio"]):not([type="submit"]):not([type="button"]):not([type="reset"]):not([type="image"]):not([type="range"]):not([type="color"]):not([type="file"]), select, textarea';
  const lca = (a, b) => { for (let n = a; n; n = n.parentElement) if (n.contains(b)) return n; return null; };
  const fields = [];
  for (const el of document.body.querySelectorAll(FIELD)) {
    if (!vis(el)) continue;
    let label = el.labels && [...el.labels].find((l) => vis(l));
    if (!label) { const by = el.getAttribute("aria-labelledby"); const t = by && document.getElementById(by.split(/\s+/)[0]); if (t && vis(t)) label = t; }
    let wrap = label ? (label.contains(el) ? label : lca(label, el)) : el;
    if (!wrap || wrap === document.body || wrap.querySelectorAll(FIELD).length > 1) wrap = el;
    const rp = wrap.parentElement;
    if (!rp) continue;
    const d = cs(rp).display, dir = cs(rp).flexDirection;
    fields.push({
      id: reg(el), tag: el.tagName.toLowerCase(), type: el.getAttribute("type") || "", inputmode: el.getAttribute("inputmode") || "", multiline: el.tagName === "TEXTAREA" || (el.tagName === "SELECT" && (el.multiple || el.size > 1)),
      rect: R(el.getBoundingClientRect()), label: label ? { id: reg(label), rect: R(label.getBoundingClientRect()), text: norm(label.textContent).slice(0, 40) } : null,
      controlParent: reg(el.parentElement), rowParent: reg(rp), rowLayout: d.includes("grid") ? "grid" : d.includes("flex") ? (dir.startsWith("column") ? "flex-col" : "flex") : "block",
    });
  }
  const actions = [];
  for (const el of document.body.querySelectorAll('button, input[type="submit"], input[type="button"], [role="button"]')) {
    if (!vis(el) || cs(el).display === "inline" || !el.parentElement) continue;
    actions.push({ id: reg(el), rect: R(el.getBoundingClientRect()), parent: reg(el.parentElement), text: norm(el.textContent || el.value).slice(0, 30) });
  }

  // tables (R3): <table> and ARIA grids; header rows are not part of the median
  const tables = [], tableRoots = []; // tableRoots: the elements R8 must not treat as lists (a column of alike checkboxes is a column, not a checklist)
  const CONTROLS = 'input:not([type="hidden"]), select, textarea, button, a[href]';
  for (const t of document.body.querySelectorAll('table, [role="table"], [role="grid"], [role="treegrid"]')) {
    if (!vis(t)) continue;
    const isTable = t.tagName === "TABLE";
    const rowEls = isTable ? [...t.rows] : [...t.querySelectorAll('[role="row"]')];
    const rows = [];
    for (const row of rowEls) {
      if (!vis(row)) continue;
      const cellEls = isTable ? [...row.cells] : [...row.querySelectorAll('[role="cell"], [role="gridcell"], [role="columnheader"], [role="rowheader"]')];
      const header = (isTable && (row.parentElement.tagName === "THEAD" || (cellEls.length > 0 && cellEls.every((c) => c.tagName === "TH")))) || (!isTable && cellEls.length > 0 && cellEls.every((c) => c.getAttribute("role") === "columnheader"));
      rows.push({
        id: reg(row), rect: R(row.getBoundingClientRect()), header,
        cells: cellEls.filter((c) => vis(c)).map((c) => ({ id: reg(c), rect: R(c.getBoundingClientRect()), scrollW: c.scrollWidth, clientW: c.clientWidth, controls: [...c.querySelectorAll(CONTROLS)].filter((x) => vis(x)).map((x) => ({ id: reg(x), rect: R(x.getBoundingClientRect()) })) })),
      });
    }
    if (rows.length) { tables.push({ id: reg(t), rows }); tableRoots.push(t); }
  }

  // div-based grid "tables" (no table role): a container of >= 3 grid/flex rows whose cells run side by side and line up column by column, e.g. the variant matrix
  const cellsOf = (row) => [...row.children].filter((x) => !SKIP.has(x.tagName) && vis(x));
  const rowRecord = (row, header) => ({
    id: reg(row), rect: R(row.getBoundingClientRect()), header,
    cells: cellsOf(row).map((c) => ({ id: reg(c), rect: R(c.getBoundingClientRect()), scrollW: c.scrollWidth, clientW: c.clientWidth, controls: [...(c.matches(CONTROLS) ? [c] : []), ...c.querySelectorAll(CONTROLS)].filter((x) => vis(x)).map((x) => ({ id: reg(x), rect: R(x.getBoundingClientRect()) })) })),
  });
  for (const c of all) {
    if (c.children.length < 3 || c.children.length > 600 || /^(TABLE|TBODY|THEAD|TFOOT|TR|UL|OL|SELECT|DL|FORM)$/.test(c.tagName) || c.closest('table, [role="table"], [role="grid"], [role="treegrid"]')) continue;
    const rowEls = [...c.children].filter((r) => vis(r) && /grid|flex/.test(cs(r).display) && !cs(r).flexDirection.startsWith("column") && cellsOf(r).length >= 3);
    if (rowEls.length < 3) continue;
    const xs = (r) => cellsOf(r).slice(0, 3).map((x) => x.getBoundingClientRect().left);
    const first = xs(rowEls[0]);
    if (!rowEls.every((r) => { const v = xs(r); return v.every((x, i) => Math.abs(x - first[i]) <= 2) && v[0] + 8 < v[1] && v[1] + 8 < v[2]; })) continue;
    const rows = rowEls.map((r, i) => rowRecord(r, i === 0 && (r.getAttribute("aria-hidden") === "true" || ![...r.querySelectorAll(CONTROLS)].length) && rowEls.slice(1).some((o) => o.querySelector(CONTROLS))));
    tables.push({ id: reg(c), grid: true, rows }); tableRoots.push(c);
  }

  // text nodes (R4, R6): per visible text node the rendered line rects and the computed font size
  const texts = [], textClips = [];
  const CONTROLISH = 'button, select, input[type="button"], input[type="submit"], [role="button"], [role="tab"], [role="menuitem"], [role="switch"], summary';
  const LABELISH = 'label, a, th, legend, [role="link"]';
  const HELPER = /help|hint|counter|caption|note|desc|muted|meta|sub(title|text)?\b/i;
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  const range = document.createRange();
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    const text = norm(n.data), p = n.parentElement;
    if (!text || !p || SKIP.has(p.tagName) || p instanceof SVGElement || !vis(p)) continue;
    range.selectNodeContents(n);
    const clip = childClip(p); // what clips text inside p: p's own overflow plus everything above it
    const raw = [...range.getClientRects()].filter((r) => r.width > 0.5 && r.height > 0.5);
    const rects = raw.map((r) => inter({ l: r.left, t: r.top, r: r.right, b: r.bottom }, clip)).filter((r) => r.r - r.l > 0.5 && r.b - r.t > 0.5);
    // R7: the part of a text line of a control / label that lies outside the box that should hold it, on either side (scrollWidth cannot see the left side).
    const control = p.closest(CONTROLISH) ? true : p.closest(LABELISH) ? false : null;
    if (control !== null && raw.length) {
      const cut = (c) => (c ? Math.max(...raw.map((r) => r.width - Math.max(0, Math.min(r.right, c.r) - Math.max(r.left, c.l)))) : 0);
      const hardBox = hardClip(p), hidden = cut(hardBox), scrolled = cut(clip) - hidden;
      const owner = p.closest(control ? CONTROLISH : LABELISH), ob = owner.getBoundingClientRect();
      const floating = ["absolute", "fixed"].includes(cs(p).position); // a badge or popover text positioned outside its owner on purpose
      const spill = floating ? 0 : Math.max(...raw.map((r) => Math.max(0, ob.left - r.left) + Math.max(0, r.right - ob.right)));
      const [mode, px, box] = hidden > 0.5 ? ["hidden", hidden, hardBox] : spill > 0.5 ? ["spill", spill, { l: ob.left, t: ob.top, r: ob.right, b: ob.bottom }] : scrolled > 0.5 ? ["scroll", scrolled, clip] : [null, 0, null];
      if (mode && box && Number.isFinite(box.l + box.r))
        textClips.push({ id: reg(p), control, mode, hiddenPx: Math.round(px * 10) / 10, text: text.slice(0, 40), rect: R({ left: box.l, top: Number.isFinite(box.t) ? box.t : raw[0].top, width: box.r - box.l, height: Number.isFinite(box.b - box.t) ? box.b - box.t : raw[0].height }), textRect: R({ left: Math.min(...raw.map((r) => r.left)), top: Math.min(...raw.map((r) => r.top)), width: Math.max(...raw.map((r) => r.right)) - Math.min(...raw.map((r) => r.left)), height: Math.max(...raw.map((r) => r.bottom)) - Math.min(...raw.map((r) => r.top)) }) });
    }
    if (!rects.length) continue;
    const size = Math.round(parseFloat(cs(p).fontSize) * 100) / 100;
    const cls = typeof p.className === "string" ? p.className : "";
    const iconish = (p.closest('[aria-hidden="true"]') && text.length <= 3) || /(^|[\s_-])(icon|glyph|emoji|material-symbols[\w-]*|fa|fas|far)([\s_-]|$)/i.test(cls) || /^[^\p{L}\p{N}]{1,2}$/u.test(text);
    let helper = false;
    for (let a = p, d = 0; a && d < 3 && !helper; a = a.parentElement, d++) helper = a.tagName === "SMALL" || HELPER.test(typeof a.className === "string" ? a.className : "") || HELPER.test(a.getAttribute("data-testid") || "") || (a.id && !!document.querySelector(`[aria-describedby~="${CSS.escape(a.id)}"]`));
    texts.push({ id: reg(p), size, helper, icon: !!iconish, pinned: pinned(p), text: text.slice(0, 30), rects: rects.slice(0, 8).map((r) => R({ left: r.l, top: r.t, width: r.r - r.l, height: r.b - r.t })) });
  }

  // interactive targets (R5, R6)
  const TARGET = 'a[href], button, input:not([type="hidden"]), select, textarea, summary, [role="button"], [role="link"], [role="tab"], [role="menuitem"], [role="checkbox"], [role="switch"], [role="radio"]'; // a focusable scroll region (tabindex) is not a control
  const targets = [];
  for (const el of document.body.querySelectorAll(TARGET)) {
    if (el.disabled || el.getAttribute("aria-disabled") === "true") continue;
    let target = el, v = vis(el);
    if (el.tagName === "INPUT" && /^(checkbox|radio)$/i.test(el.type)) {
      const lab = el.labels && [...el.labels].find((l) => vis(l));
      if (lab && (!v || vis(lab).raw.width * vis(lab).raw.height > v.raw.width * v.raw.height)) { target = lab; v = vis(lab); }
    }
    if (!v) continue;
    const p = target.parentElement;
    const rest = p ? norm(p.textContent).replace(norm(target.textContent), "") : "";
    const inline = target.tagName === "A" && cs(target).display === "inline" && rest.length >= 3;
    const anc = p && p.closest(TARGET);
    targets.push({ id: reg(target), tag: target.tagName.toLowerCase(), type: target.getAttribute("type") || "", rect: R(target.getBoundingClientRect()), inline, pinned: pinned(target), parent: anc ? reg(anc) : -1, text: norm(target.textContent || target.getAttribute("aria-label") || target.value || target.getAttribute("name")).slice(0, 30) });
  }

  // selects (R7): the selected option's text against the room the select has for it; fixed/sticky bars (R10)
  const selects = [];
  const canvas = document.createElement("canvas").getContext("2d");
  for (const el of document.body.querySelectorAll("select")) {
    if (el.multiple || el.size > 1 || !vis(el)) continue;
    const st = cs(el), text = norm(el.selectedOptions[0] ? el.selectedOptions[0].text : "");
    if (!text) continue;
    canvas.font = st.font || `${st.fontStyle} ${st.fontWeight} ${st.fontSize} ${st.fontFamily}`;
    selects.push({ id: reg(el), rect: R(el.getBoundingClientRect()), clientW: el.clientWidth, padL: parseFloat(st.paddingLeft) || 0, padR: parseFloat(st.paddingRight) || 0, nativeArrow: (st.appearance || "auto") !== "none", textW: canvas.measureText(text).width, text: text.slice(0, 40) });
  }
  const bars = [];
  for (const el of all) {
    const st = cs(el);
    if ((st.position !== "fixed" && st.position !== "sticky") || (el.parentElement && pinned(el.parentElement))) continue; // the outermost pinned box
    const v = vis(el);
    if (!v || v.clip.r - v.clip.l < 32 || v.clip.b - v.clip.t < 16) continue;
    const alpha = (st.backgroundColor.match(/^rgba?\(([^)]*)\)/)?.[1].split(",")[3] ?? "1").trim();
    if (!(parseFloat(alpha) >= 0.5) && st.backgroundImage === "none") continue; // a transparent wrapper covers nothing
    bars.push({ id: reg(el), rect: R({ left: v.clip.l, top: v.clip.t, width: v.clip.r - v.clip.l, height: v.clip.b - v.clip.t }) });
  }

  // list / checklist containers (R8)
  const lists = [];
  const ITEM = ':scope > li, :scope > [role="listitem"], :scope > [role="option"], :scope > [role="menuitem"]';
  const textOf = (el) => norm(el.innerText).toLowerCase().replace(/[^\p{L}\p{N}\s]/gu, "").replace(/\s+/g, " ").trim(); // status marks (check / circle) are not part of the label
  const take = (container, items) => {
    const rows = [];
    for (const it of items) { if (!vis(it)) continue; const text = textOf(it); if (text.length >= 2 && text.length <= 80 && /[\p{L}\p{N}]/u.test(text)) rows.push({ id: reg(it), text }); }
    if (rows.length >= 2 && vis(container)) lists.push({ id: reg(container), rect: R(container.getBoundingClientRect()), items: rows });
  };
  for (const c of document.body.querySelectorAll('ul, ol, menu, [role="list"], [role="listbox"], [role="menu"]')) take(c, [...c.querySelectorAll(ITEM)].length ? c.querySelectorAll(ITEM) : c.children);
  const within = (el, root, max) => { let n = 0; for (let x = el; x && x !== root; x = x.parentElement) n++; return n <= max; }; // a checklist item sits at most two wrappers below its list; a label deeper down belongs to a repeated row
  const OWNER = 'fieldset, [role="radiogroup"], [role="group"], ul, ol, table, [role="table"], [role="grid"]';
  for (const c of document.body.querySelectorAll('fieldset, [role="radiogroup"], [role="group"]')) take(c, [...c.querySelectorAll("label")].filter((l) => l.parentElement.closest(OWNER) === c && !tableRoots.some((t) => t.contains(l)) && within(l, c, 3) && (l.querySelector('input[type="checkbox"], input[type="radio"]') || (l.htmlFor && document.getElementById(l.htmlFor)?.matches('input[type="checkbox"], input[type="radio"]')))));
  // a checklist made of sibling buttons / divs (not <ul>): the siblings that share one tag + class are the items, headings in between do not matter
  const sig = (el) => el.tagName + "." + (typeof el.className === "string" ? el.className : "");
  for (const c of all) {
    if (!c.children || c.children.length < 3 || /^(UL|OL|MENU|TABLE|TBODY|THEAD|TR|SELECT|FIELDSET|DL)$/.test(c.tagName)) continue;
    const groups = new Map();
    for (const k of c.children) { const g = sig(k); if (/\.\S/.test(g)) (groups.get(g) ?? groups.set(g, []).get(g)).push(k); }
    for (const g of groups.values()) if (g.length >= 3) take(c, g);
  }
  return { viewport: { w: vw, h: vh, innerWidth: window.innerWidth }, doc: { scrollWidth: doc.scrollWidth, scrollHeight: doc.scrollHeight, clientWidth: vw }, scrollY: Math.round(sy), boxes, fields, actions, tables, texts, textClips, selects, bars, targets, lists };
}

// In-page: resolve element ids (window.__vaEls) to selector path, test id and visible text.
export function describe(idList) {
  const esc = (s) => (window.CSS && CSS.escape ? CSS.escape(s) : s);
  const pathOf = (el) => {
    const parts = [];
    for (let cur = el, depth = 0; cur && cur.nodeType === 1 && cur !== document.body && cur !== document.documentElement && depth < 8; cur = cur.parentElement, depth++) {
      let s = cur.tagName.toLowerCase();
      if (cur.id) { parts.unshift(`${s}#${esc(cur.id)}`); break; }
      const tid = cur.getAttribute("data-testid");
      if (tid) { parts.unshift(`${s}[data-testid="${tid}"]`); break; }
      const cls = typeof cur.className === "string" ? cur.className.split(/\s+/).filter((c) => c && !/^(css-|jsx-|__|_)/.test(c)).slice(0, 2) : [];
      if (cls.length) s += "." + cls.map(esc).join(".");
      const same = cur.parentElement ? [...cur.parentElement.children].filter((c) => c.tagName === cur.tagName) : [];
      if (same.length > 1) s += `:nth-of-type(${same.indexOf(cur) + 1})`;
      parts.unshift(s);
    }
    return parts.join(" > ");
  };
  const out = {};
  for (const i of idList) { const el = (window.__vaEls || [])[i]; if (el) out[i] = { path: pathOf(el), tag: el.tagName.toLowerCase(), testid: el.getAttribute("data-testid") || "", text: (el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 40) }; }
  return out;
}
