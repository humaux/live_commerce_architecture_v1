// Purpose: DS-1 gate that hard-coded hex colours in apps/admin never increase against tests/ui/hex-baseline.json, plus negatives proving the ratchet can fail (growth, a new file, id/entity look-alikes).
// Depends on: node:test/assert/fs, scripts/dev/check-hex-ratchet.mjs and the apps/admin source tree (read only).
// Used by: scripts/dev/test-node.sh and the --browser-ui-gallery mode.
import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, relative } from "node:path";
import { test } from "node:test";
import { BASELINE, compare, countHex, scan } from "../../scripts/dev/check-hex-ratchet.mjs";

const baseline = JSON.parse(readFileSync(BASELINE, "utf8"));

test("baseline is internally consistent: total equals the sum of its files", () => {
  assert.equal(Object.values(baseline.files).reduce((a, b) => a + b, 0), baseline.total);
  assert.ok(baseline.total > 0);
});

test("countHex counts colours, not ids, entities, comments or longer words", () => {
  assert.equal(countHex("a{color:#fff;background:#1a2b3c;border:1px solid #AABBCCDD;x:#0006}"), 4);
  assert.equal(countHex('<a href="#main">&#x27;</a> <span id="#facebook"/>'), 0, "anchors, entities and #facebook are not colours");
  assert.equal(countHex("/* #fff #000 */\n// #123456\n.a{color:var(--ds-text-1)}"), 0, "comments are not counted");
  assert.equal(countHex("rgb(0 0 0 / 6%)"), 0);
});

test("bypass: a /* inside a string or a // comment does not start a block that hides real colours", () => {
  assert.equal(countHex('const a = "/*"; const c = "#ff0000"; const b = "*/";'), 1, "double-quoted /* ... */ around a colour");
  assert.equal(countHex("const a = '/*'; const c = '#ff0000'; const b = '*/';"), 1, "single-quoted");
  assert.equal(countHex("const a = `/*`; const c = `#ff0000`; const b = `*/`;"), 1, "template literal");
  assert.equal(countHex('const a = `x ${"/*"} #ff0000 ${"*/"}`;'), 1, "inside a template expression");
  assert.equal(countHex('// see /* here\nconst c = "#ff0000";\n// done */'), 1, "a /* in a // comment is not a block start");
  assert.equal(countHex("a{content:'/*';color:#ff0000}b{content:'*/'}", ".css"), 1, "CSS string");
  assert.equal(countHex("a{background:url(//cdn.example/x.png);color:#ff0000}", ".css"), 1, "// is not a CSS comment");
  assert.equal(countHex("a{background:url(http://cdn.example/x.png);color:#ff0000}", ".scss"), 1, "url(http://) is not an SCSS comment");
});

test("uncertain // (JSX text, SCSS url) never drops the rest of the line", () => {
  assert.equal(countHex("const x = <div>https://example.test <span style={{ color: '#ff0000' }} /></div>;", ".tsx"), 1, "URL in JSX text");
  assert.equal(countHex("const x = <p>see //docs <i style={{ color: '#ff0000' }} /></p>;", ".jsx"), 1, "bare // in JSX text");
  assert.equal(countHex("a{background:url(//cdn.example/x.png);color:#ff0000}", ".scss"), 1, "protocol-relative SCSS url");
  assert.equal(countHex("const c = 1; // #ff0000\nconst d = <b>x</b>; // #00ff00", ".tsx"), 0, "real TSX line comments stay ignored");
  assert.equal(countHex("a{color:red} // #fff\nb{color:blue}", ".scss"), 0, "real SCSS line comment stays ignored");
});

test("comments are still ignored: block, trailing //, SCSS //, multi-line", () => {
  assert.equal(countHex("const c = 1; // #ff0000\n/* #00ff00\n   #0000ff */ const d = 2;"), 0);
  assert.equal(countHex("a{color:red} /* #fff */\n// #000\nb{color:blue}", ".scss"), 0);
  assert.equal(countHex("a{color:red} /* #fff */", ".css"), 0);
  assert.equal(countHex("const s = `a ${/* #fff */ 1} b`;"), 0);
  assert.equal(countHex('const s = "unterminated /* \nconst c = "#ff0000";'), 1, "a string never runs past its line");
});

test("scan: reads .js/.jsx/.scss too and skips only the root tests/public dirs by path", () => {
  const root = mkdtempSync(join(tmpdir(), "hex-ratchet-"));
  try {
    const put = (rel, text) => {
      mkdirSync(join(root, rel, ".."), { recursive: true });
      writeFileSync(join(root, rel), text);
    };
    put("src/a.js", 'const c = "#ff0000";');
    put("src/b.jsx", 'const c = "#ff0000";');
    put("src/c.scss", "a{color:#ff0000}");
    put("src/tests/d.css", "a{color:#ff0000}");
    put("components/public/e.css", "a{color:#ff0000}");
    put("tests/f.css", "a{color:#ff0000}");
    put("public/g.css", "a{color:#ff0000}");
    put("node_modules/h.css", "a{color:#ff0000}");
    put("src/deep/node_modules/i.css", "a{color:#ff0000}");
    put(".next/j.css", "a{color:#ff0000}");
    put("src/k.d.ts", 'export const c = "#ff0000";');
    const found = scan(root);
    assert.deepEqual(Object.keys(found.files).map((p) => relative(root, p)).sort(), ["components/public/e.css", "src/a.js", "src/b.jsx", "src/c.scss", "src/tests/d.css"]);
    assert.equal(found.total, 5);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("compare: growth of the total or of any file fails; a new file has an allowance of zero; shrinking passes", () => {
  const base = { total: 3, files: { "a.css": 2, "b.css": 1 } };
  assert.deepEqual(compare({ total: 3, files: { "a.css": 2, "b.css": 1 } }, base), []);
  assert.deepEqual(compare({ total: 2, files: { "a.css": 1, "b.css": 1 } }, base), []);
  assert.match(compare({ total: 4, files: { "a.css": 3, "b.css": 1 } }, base).join("\n"), /a\.css: 3 hex colours > baseline 2/);
  assert.match(compare({ total: 4, files: { "a.css": 2, "b.css": 1, "new.css": 1 } }, base).join("\n"), /new\.css: 1 hex colours > baseline 0/);
  assert.match(compare({ total: 4, files: { "a.css": 1, "b.css": 1, "c.css": 2 } }, base).join("\n"), /total hex colours 4 > baseline 3/);
});

test("the live tree holds the baseline (apps/admin hex colours did not increase)", () => {
  assert.deepEqual(compare(scan(), baseline), []);
});
