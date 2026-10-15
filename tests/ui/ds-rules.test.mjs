// Purpose: proves the design-spec rule checker (scripts/dev/check-ds-rules.mjs) fails on every injected violation R1–R7 and passes the
//   spec-conforming equivalents, then that the live DS tree and every @ds-page file are clean.
// Depends on: node:test/assert, scripts/dev/check-ds-rules.mjs (pure check/scan), the packages/ui/src/ds and apps/admin sources (read only).
// Used by: scripts/dev/test-node.sh (CI); docs/design/DESIGN-SPEC.md §6.
import assert from "node:assert/strict";
import { test } from "node:test";
import { check, scan, scope, GLYPHS } from "../../scripts/dev/check-ds-rules.mjs";

const rules = (src, ext, file) => check(src, ext, file).map((v) => v.rule.split(" ")[0]);

test("CSS: injected colour, spacing and font-size literals are red; tokens are green", () => {
  assert.deepEqual(rules(".a{color:#fff}", ".css"), ["R1"]);
  assert.deepEqual(rules(".a{background:rgb(0 0 0 / 6%)}", ".css"), ["R1"]);
  assert.deepEqual(rules(".a{padding:12px 24px}", ".css"), ["R2"]);
  assert.deepEqual(rules(".a{margin-top:10px}", ".css"), ["R2"]);
  assert.deepEqual(rules(".a{gap:var(--ds-space-2) 7px}", ".css"), ["R2"]);
  assert.deepEqual(rules(".a{font-size:13px}", ".css"), ["R3"]);
  assert.deepEqual(rules(".a{color:var(--ds-text-1);padding:var(--ds-space-3) var(--ds-space-4);margin:0 auto;gap:1px;font-size:var(--ds-text-sm)}", ".css"), []);
  assert.deepEqual(rules(".a{padding-inline:calc(var(--ds-space-4) - 1px);width:100%}", ".css"), []);
  assert.deepEqual(rules("/* #fff 12px */ .a{border:1px solid var(--ds-border)}", ".css"), [], "comments and borders are not checked as spacing");
});

test("TSX: glyph icons, raw svg, inline style objects, plain className strings and colour strings are red", () => {
  for (const g of ["▸", "▾", "✕", "⌕", "•", "✓"]) assert.ok(GLYPHS.includes(g));
  assert.deepEqual(rules("<span aria-hidden>▾</span>", ".tsx", "x/Menu.tsx"), ["R4"]);
  assert.deepEqual(rules('const mark = "✕";', ".tsx", "x/Alert.tsx"), ["R4"]);
  assert.deepEqual(rules("<b>• 处理中</b>", ".tsx", "x/Badge.tsx"), ["R4"]);
  assert.deepEqual(rules('<svg viewBox="0 0 24 24"><path d="M0 0"/></svg>', ".tsx", "x/Nav.tsx"), ["R5"]);
  assert.deepEqual(rules("<div style={{ width: 400 }} />", ".tsx", "x/F.tsx"), ["R6"]);
  assert.deepEqual(rules('<div className="section-bar" />', ".tsx", "x/P.tsx"), ["R7"]);
  assert.deepEqual(rules('const c = "#3E8CD7";', ".ts", "x/c.ts"), ["R1"]);
});

test("TSX: spec-conforming equivalents are green", () => {
  assert.deepEqual(rules('<svg viewBox="0 0 24 24"><path d="M0 0"/></svg>', ".tsx", "packages/ui/src/ds/icons.tsx"), [], "icons.tsx owns the SVG set");
  assert.deepEqual(rules('<Icon name="chevron-down" size={16} />', ".tsx", "x/Menu.tsx"), []);
  assert.deepEqual(rules("<div style={{ '--cols': 3 }} className={s.grid} />", ".tsx", "x/G.tsx"), []);
  assert.deepEqual(rules('<a href="#sec-table">↑ 130% 相较昨天</a>', ".tsx", "x/K.tsx"), [], "anchors and arrows inside text are fine");
  assert.deepEqual(rules("<p>支持 1200 × 1200 px</p>", ".tsx", "x/U.tsx"), [], "a multiplication sign inside a sentence is text, not an icon");
  assert.deepEqual(rules("// ▾ old caret was a glyph\nconst a = 1;", ".tsx", "x/C.tsx"), [], "comments are ignored");
});

test("the live DS tree and every @ds-page file follow the spec (R1–R7)", () => {
  const files = scope();
  assert.ok(files.some((f) => f.startsWith("packages/ui/src/ds/")), "DS directory is in scope");
  const v = scan(files);
  assert.deepEqual(v.map((x) => `${x.file}:${x.line} ${x.rule}`), []);
});
