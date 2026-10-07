// Purpose: unit tests of the pull-request gate planner (scripts/dev/pr-modes.mjs): backend-only and docs-only diffs run foundation-shards only, any other path adds the full browser
//   set, the deploy flag follows deploy/ and scripts/deploy*, and the browser set equals release-gate.sh's browser-mode universe minus the documented exclusions. Run by scripts/dev/test-node.sh.
// Depends on: scripts/dev/pr-modes.mjs, scripts/dev/test-local.sh (usage line), scripts/dev/release-gate.sh (the universe derivation is executed from its own source), bash.
// Used by: scripts/dev/test-node.sh, CI.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { EXCLUDED_MODES, browserModes, planPr } from "../../scripts/dev/pr-modes.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const usage = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8");
const FOUNDATION = ["foundation-shards"];
const full = browserModes(usage);

test("backend-only paths run foundation-shards only", () => {
  const r = planPr(["internal/orders/x.go", "cmd/api/main.go", "migrations/0130_x.sql", "contracts/invariants.json", "go.mod", "go.sum", "tests/foundation/orders_test.go", "deploy/compose.yml", "README.md", "apps/admin/NOTES.md"]);
  assert.deepEqual(r.modes, FOUNDATION);
});

test("node tests of deploy scripts and of the CI planner are backend-only (they cannot change a browser)", () => {
  assert.deepEqual(planPr(["internal/a.go", "tests/deploy/deploy-prep-r3.test.mjs", "tests/ci/pr-modes.test.mjs"]).modes, FOUNDATION);
  assert.deepEqual(planPr(["tests/deploy/x.test.mjs", "scripts/dev/pr-modes.mjs"]).modes.length > 1, true); // the planner itself still counts as UI
});

test("a UI path adds the whole browser set, including click-sweep and visual-lint", () => {
  for (const p of ["apps/admin/src/a.tsx", "packages/i18n/x.ts", "tests/admin/x.test.ts", "tests/e2e/x.spec.ts", "scripts/dev/test-local.sh", ".github/workflows/gates.yml", "playwright.config.ts", "package.json", "pnpm-lock.yaml"]) {
    const r = planPr(["internal/a.go", p]);
    assert.deepEqual(r.modes, [...FOUNDATION, ...full], p);
  }
  assert.ok(full.includes("--browser-click-sweep") && full.includes("--browser-visual-lint") && full.includes("--browser-e2e") && full.includes("--browser-webkit"));
});

test("docs-only and output-only diffs keep the single required check (foundation-shards)", () => {
  assert.deepEqual(planPr(["docs/delivery/GATES.md", "output/x/DELIVERY.md", "CHANGELOG.md"]).modes, FOUNDATION);
  assert.deepEqual(planPr([]).modes, FOUNDATION, "an empty diff still yields the one required check");
});

test("tests/foundation/browser_* is a browser path, other tests/foundation files are not", () => {
  assert.deepEqual(planPr(["tests/foundation/browser_x_test.go"]).modes, [...FOUNDATION, ...full]);
  assert.deepEqual(planPr(["tests/foundation/x_test.go"]).modes, FOUNDATION);
});

test("deploy flag: deploy/ and scripts/deploy* only", () => {
  assert.equal(planPr(["deploy/scripts/smoke.sh"]).deploy, true);
  assert.equal(planPr(["scripts/deploy-prep.sh"]).deploy, true);
  assert.equal(planPr(["scripts/deploy/x.sh"]).deploy, true);
  assert.equal(planPr(["internal/a.go", "docs/x.md"]).deploy, false);
  assert.equal(planPr(["scripts/dev/test-local.sh"]).deploy, false);
});

test("the browser set is release-gate's browser-mode universe minus the documented exclusions", () => {
  const src = readFileSync(path.join(root, "scripts/dev/release-gate.sh"), "utf8").split("\n");
  const derive = src.filter((l) => /^modes=\$\(sed /.test(l) || /^browser_modes=\$\(printf /.test(l)).join("\n");
  assert.equal(derive.split("\n").length, 2, "release-gate.sh derivation lines not found; update this test with the script");
  const universe = execFileSync("bash", ["-c", `${derive}\nprintf '%s\\n' $browser_modes`], { cwd: root, encoding: "utf8" }).trim().split("\n");
  assert.ok(universe.length > 20);
  assert.deepEqual(full, universe.filter((m) => !(m in EXCLUDED_MODES)));
  for (const [m, why] of Object.entries(EXCLUDED_MODES)) { assert.ok(universe.includes(m), `${m} is not in the universe any more: drop the exclusion`); assert.ok(why.length > 10); }
});
