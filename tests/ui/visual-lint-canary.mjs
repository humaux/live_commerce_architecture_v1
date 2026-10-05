// G-UI9 canary: the in-page collector + the rules, end to end, on two small synthetic pages in real Chromium. A lint that cannot observe a failure proves
// nothing (verify-first), so the known-BAD page must trip every rule R1..R8 and the known-GOOD page must trip none. Run by
// `bash scripts/dev/test-local.sh --browser-visual-lint` before the stack is built; no network, no PG, no Next.
import assert from "node:assert/strict";
import { chromium } from "@playwright/test";
import { collect, describe, evaluate } from "./visual-lint-lib.mjs";

const HEAD = `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<style>body{margin:0;font:16px/1.4 sans-serif} label{display:block;height:20px} input,select,textarea,button{font:inherit;box-sizing:border-box}
.field{display:flex;flex-direction:column;width:280px} table{border-collapse:collapse;table-layout:fixed;width:200px} td,th{border:1px solid #999;padding:4px;height:40px;vertical-align:top}</style>`;

const BAD = `${HEAD}<body>
<div id="r1" style="width:1200px;height:10px;background:#ddd"></div>
<div id="r2" style="display:flex;gap:16px;margin:20px">
  <div class="field"><label for="n">Name</label><input id="n" style="height:34px"></div>
  <div class="field" style="margin-top:14px"><label for="m">Notes</label><textarea id="m" style="height:90px"></textarea></div>
  <button style="height:24px;margin-top:2px;align-self:flex-start">Add</button>
</div>
<table id="r3"><tr><th style="width:80px">SKU</th><th style="width:120px">Stock</th></tr>
<tr><td>A</td><td><input style="width:100px;height:30px"></td></tr><tr><td>B</td><td><input style="width:100px;height:30px"></td></tr>
<tr><td>C</td><td><input style="width:100px;height:30px"></td></tr>
<tr><td>D</td><td><div style="height:60px"><input type="checkbox"></div><input style="width:100px;height:30px"></td></tr>
<tr><td>E</td><td><input style="width:180px;height:30px"></td></tr></table>
<fieldset id="r3g" style="margin:8px;width:560px;border:0;padding:0">
  <div style="display:grid;grid-template-columns:80px 140px 160px 140px" aria-hidden="true"><span>Variant</span><span>Price</span><span>Stock</span><span>SKU</span></div>
  ${[1, 2, 3].map((i) => `<div style="display:grid;grid-template-columns:80px 140px 160px 140px;align-items:start;border-top:1px solid #999"><span>V${i}</span><input style="height:30px"><div><label style="display:block"><input type="checkbox"> no tracking</label><input style="height:30px;display:block"></div><input style="height:30px"></div>`).join("")}
</fieldset>
<p id="r4" style="font-size:10px;margin:8px">tiny body text</p><small class="hint" style="font-size:10px;margin:8px">tiny helper</small>
<button id="r5" style="width:28px;height:28px;margin:8px">x</button>
<button id="r6a" style="position:absolute;left:40px;top:520px;width:100px;height:40px">One</button><button id="r6b" style="position:absolute;left:100px;top:530px;width:100px;height:40px">Two</button>
<span style="position:absolute;left:300px;top:520px;font-size:16px">Overlapping words</span><span style="position:absolute;left:300px;top:526px;font-size:16px">on top of each other</span>
<button id="r7" style="display:block;width:60px;margin:8px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">A very long button label</button>
<fieldset id="r8b" style="margin-top:80px"><label><input type="checkbox"> Same label</label><label><input type="checkbox"> Same label</label><label><input type="checkbox"> Other</label></fieldset>
<section id="r8c"><h3>Required</h3><button class="it" style="display:block">Images ○</button><button class="it" style="display:block">Name ○</button><button class="it" style="display:block">Price ○</button><h3>Recommended</h3><button class="it" style="display:block">Images ✓</button><button class="it" style="display:block">SEO ○</button></section>
<ul id="r8" style="margin-top:20px"><li>Connect page</li><li>Add product</li><li>Connect page</li></ul>
</body>`;

const GOOD = `${HEAD}<body style="padding:16px">
<div style="display:flex;flex-wrap:wrap;gap:16px;align-items:flex-end;margin-bottom:16px">
  <div class="field"><label for="n">Name</label><input id="n" style="height:44px"></div>
  <div class="field"><label for="s">Status</label><select id="s" style="height:44px"><option>Draft</option></select></div>
  <button style="height:44px;min-width:96px">Add</button>
</div>
<table><tr><th style="width:80px">SKU</th><th style="width:120px">Stock</th></tr>
<tr><td>A</td><td><input style="width:100px;height:40px"></td></tr><tr><td>B</td><td><input style="width:100px;height:40px"></td></tr>
<tr><td>C</td><td><input style="width:100px;height:40px"></td></tr><tr><td>D</td><td><input style="width:100px;height:40px"></td></tr></table>
<div id="grid" style="margin:8px 0;width:100%">
  <div style="display:grid;grid-template-columns:60px repeat(3,minmax(0,1fr))" aria-hidden="true"><span>Variant</span><span>Price</span><span>Stock</span><span>SKU</span></div>
  ${[1, 2, 3].map((i) => `<div style="display:grid;grid-template-columns:60px repeat(3,minmax(0,1fr));align-items:center;border-top:1px solid #999"><span>V${i}</span><input style="height:40px;width:100%;min-width:0"><input style="height:40px;width:100%;min-width:0"><input style="height:40px;width:100%;min-width:0"></div>`).join("")}
</div>
<label for="hid" style="position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap">A visually hidden label that is longer than its one pixel box</label><input id="hid" style="width:200px;height:44px;display:block;margin-top:8px">
<p>Readable body text with a <a href="#x">link inside the sentence</a>.</p><small style="font-size:12px">helper at the floor</small>
<div style="margin-top:16px"><button style="height:44px;width:120px">Save</button> <button style="height:44px;width:120px">Cancel</button></div>
<ul><li>Connect page</li><li>Add product</li><li>Review order</li></ul><section><h3>Required</h3><button class="it" style="display:block;height:44px">Images ○</button><button class="it" style="display:block;height:44px">Name ○</button><button class="it" style="display:block;height:44px">Price ○</button><h3>Recommended</h3><button class="it" style="display:block;height:44px">Description ✓</button><button class="it" style="display:block;height:44px">SEO ○</button></section>
</body>`;

const browser = await chromium.launch({ headless: true });
const run = async (html, width, mobile) => {
  const context = await browser.newContext({ viewport: { width, height: 800 }, deviceScaleFactor: 1 });
  const page = await context.newPage();
  await page.setContent(html, { waitUntil: "load" });
  const snapshot = await page.evaluate(collect); // [READ/MEASURE] page.evaluate only measures
  const violations = evaluate(snapshot, { mobile });
  const named = await page.evaluate(describe, [...new Set(violations.flatMap((v) => v.ids))]);
  await context.close();
  return { violations, named, snapshot };
};
try {
  const bad = await run(BAD, 900, false);
  const rules = new Set(bad.violations.map((v) => v.rule));
  for (const r of ["R1", "R2", "R3", "R4", "R6", "R7", "R8"]) assert(rules.has(r), `known-bad page: ${r} not detected; found ${[...rules]}`);
  const kinds = (rule) => new Set(bad.violations.filter((v) => v.rule === rule).map((v) => v.kind));
  assert(kinds("R2").has("label-top") && kinds("R2").has("control-top") && kinds("R2").has("action-offset"), `R2 kinds ${[...kinds("R2")]}`);
  assert(kinds("R3").has("row-tall") && kinds("R3").has("cell-overflow") && kinds("R3").has("row-stacked"), `R3 kinds ${[...kinds("R3")]}`);
  assert(kinds("R4").has("small-text") && kinds("R4").has("small-helper"), `R4 kinds ${[...kinds("R4")]}`);
  assert(kinds("R6").has("interactive-overlap") && kinds("R6").has("text-overlap"), `R6 kinds ${[...kinds("R6")]}`);
  assert(!rules.has("R5"), "R5 must not run on the desktop viewport");
  const r8 = bad.violations.find((v) => v.rule === "R8");
  assert.equal(r8.measured.label, "connect page");
  assert.deepEqual(bad.violations.filter((v) => v.rule === "R8").map((v) => v.measured.label).sort(), ["connect page", "images", "same label"], "a checklist and a list are found; the column of alike \"no tracking\" checkboxes inside the matrix is not a list");
  const r1 = bad.violations.find((v) => v.rule === "R1" && v.kind === "element-beyond-viewport");
  assert.equal(bad.named[r1.ids[0]].path, "div#r1");
  const mobile = await run(BAD, 390, true);
  assert(mobile.violations.some((v) => v.rule === "R5" && v.measured.width === 28), "known-bad page at 390: the 28px button is not flagged by R5");
  const good = [await run(GOOD, 900, false), await run(GOOD, 390, true)];
  for (const g of good) assert.deepEqual(g.violations.map((v) => `${v.rule}:${v.kind}`), [], "known-good page must have no violation");
  console.log(`PASS visual-lint canary: bad page trips ${[...rules].sort().join(",")} (+R5 at 390), good page trips none (desktop and 390)`);
} finally {
  await browser.close();
}
