// Purpose: prove local browser evidence is isolated and harness roots retain ownership.
// Depends on: Node test/fs/child_process and ../browser-evidence.mjs; no product/browser/PG runtime.
// Used by: test-node; any failed filesystem fixture is retained for review.
import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const helper = new URL("../browser-evidence.mjs", import.meta.url).href;
test("browser evidence uses unique local runs or the selected harness parent without rewriting history", () => {
  const fixture = realpathSync(mkdtempSync(path.join(tmpdir(), "lc-browser-evidence-test-")));
  let passed = false;
  try {
    const historical = path.join(fixture, "historical.json");
    writeFileSync(historical, "immutable historical evidence\n");
    const run = (overrides = {}) => {
      const env = { ...process.env };
      delete env.LC_BROWSER_EVIDENCE;
      delete env.LC_BROWSER_EVIDENCE_ROOT;
      const result = spawnSync(process.execPath, ["--input-type=module", "-e",
        `import { browserEvidenceDirectory } from ${JSON.stringify(helper)}; console.log(JSON.stringify([browserEvidenceDirectory("screens"), browserEvidenceDirectory("screens")]));`],
      { cwd: fixture, env: { ...env, ...overrides }, encoding: "utf8", timeout: 10_000 });
      assert.equal(result.error, undefined);
      assert.equal(result.status, 0, result.stderr);
      return JSON.parse(result.stdout);
    };
    const local = run();
    assert.notEqual(local[0], local[1], "two standalone runs must not overwrite one another");
    for (const directory of local) {
      assert.equal(path.dirname(directory), path.join(fixture, "output/playwright"));
      writeFileSync(path.join(directory, "shot.png"), "synthetic evidence");
    }
    const root = path.join(fixture, "run-root"), suite = path.join(root, "suite");
    assert.deepEqual(run({ LC_BROWSER_EVIDENCE_ROOT: root }), [path.join(root, "screens"), path.join(root, "screens")]);
    assert.deepEqual(run({ LC_BROWSER_EVIDENCE_ROOT: root, LC_BROWSER_EVIDENCE: suite }), [path.join(suite, "screens"), path.join(suite, "screens")]);
    assert.equal(readFileSync(historical, "utf8"), "immutable historical evidence\n");
    passed = true;
  } finally {
    if (passed) rmSync(fixture, { recursive: true });
    else console.error(`Retained failed browser evidence fixture: ${fixture}`);
  }
});
