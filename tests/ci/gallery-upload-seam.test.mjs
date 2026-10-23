// Purpose: #147 TH-13 keeps the simulated gallery upload hold/release seam out of production client builds.
// Depends on: node:test/assert/fs and the pinned TypeScript parser; the gallery component and sole mode registry are inspected as data.
// Used by: scripts/dev/test-node.sh (Foundation CI); no browser, build or provider is started here.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import ts from "typescript-api";

const flag = 'process.env.NEXT_PUBLIC_LC_GALLERY_TEST === "1"';
const component = readFileSync(new URL("../../apps/admin/components/UiGalleryTemplates.tsx", import.meta.url), "utf8");

function assertBuildTimeGate(source) {
  const file = ts.createSourceFile("gallery.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  let seams = 0;
  function visit(node) {
    if ((ts.isIdentifier(node) && node.text === "__lcHoldUpload") ||
        (ts.isStringLiteral(node) && node.text === "lc-gallery-upload-release")) {
      seams++;
      let guarded = false;
      for (let parent = node.parent; parent; parent = parent.parent) {
        if (ts.isIfStatement(parent) && parent.expression.getText(file) === flag &&
            node.pos >= parent.thenStatement.pos && node.end <= parent.thenStatement.end) guarded = true;
      }
      assert.ok(guarded, "gallery upload seam must be inside the literal public build-time flag's enabled branch");
    }
    ts.forEachChild(node, visit);
  }
  visit(file);
  assert.ok(seams > 0, "the hold/release fixture must remain covered, not deleted to satisfy the guard");
}

test("gallery upload seam is unreachable without the public build-time flag", () => {
  assertBuildTimeGate(component);
});

test("the same guard rejects ungated, runtime-only and disabled-branch seams", () => {
  const seam = 'window.__lcHoldUpload; window.addEventListener("lc-gallery-upload-release", finish);';
  assertBuildTimeGate(`if (${flag}) { ${seam} }`);
  for (const source of [seam, `if (process.env.LC_UI_GALLERY === "1") { ${seam} }`,
    `if (${flag}) {} else { ${seam} }`, `if (${flag}) {} ${seam}`]) {
    assert.throws(() => assertBuildTimeGate(source), /literal public build-time flag/);
  }
});
