// Purpose: red→green tests of the CI-SELECT classification gate (scripts/dev/check-backend-coverage.mjs): every directory under
//   internal/ must be wired by at least one browser mode's lc_covers registry data or be explicitly listed in BACKEND_ONLY_PACKAGES
//   with a one-line reason, so a new backend package fails the gate until someone classifies it; a browser mode whose body runs Go
//   must not carry empty lc_covers (that would silently deselect it for every backend change), and every covers entry must match a
//   real internal package directory (stale entries select nothing).
// Depends on: scripts/dev/check-backend-coverage.mjs, scripts/dev/pr-modes.mjs, scripts/dev/test-local.sh, git ls-files.
// Used by: scripts/dev/test-node.sh, CI.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const usage = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8");

// The same universe the gate walks: package dirs of tracked non-test Go files under internal/.
function internalDirs() {
  return [...new Set(execFileSync("git", ["ls-files", "-z", "--", "internal/"], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 })
    .split("\0").filter(Boolean).filter((f) => f.endsWith(".go") && !f.endsWith("_test.go")).map((f) => path.posix.dirname(f)))].sort();
}

// Append one browser-universe probe arm (its name matches --browser-*); covers === null omits the lc_covers line entirely.
// fixture=none keeps the probe out of the planner's PG set; the gate under test does not depend on the fixture.
function appendBrowserMode(source, name, command, covers) {
  const entry = `  ${name})
    lc_build=none
    lc_fixture=none
${covers === null ? "" : `    lc_covers="${covers}"\n`}    lc_prepare() {
  :
    }
    lc_run() {
${command}
    }
    ;;
`;
  assert.ok(source.includes("  # APPEND MODES HERE"));
  return source.replace("  # APPEND MODES HERE", entry + "  # APPEND MODES HERE");
}

test("every real internal package directory is classified (covered by lc_covers or BACKEND_ONLY-listed)", async () => {
  const { classifyBackendCoverage } = await import("../../scripts/dev/check-backend-coverage.mjs");
  const dirs = internalDirs();
  assert.ok(dirs.length > 50, "the internal tree moved? update this test with it");
  assert.deepEqual(classifyBackendCoverage(usage, dirs), []);
});

test("a new unclassified package fails the gate until covered or explicitly BACKEND_ONLY", async () => {
  const { classifyBackendCoverage } = await import("../../scripts/dev/check-backend-coverage.mjs");
  const { BACKEND_ONLY_PACKAGES } = await import("../../scripts/dev/pr-modes.mjs");
  const dirs = [...internalDirs(), "internal/brandnewpkg"];
  const errors = classifyBackendCoverage(usage, dirs);
  assert.equal(errors.length, 1, JSON.stringify(errors));
  assert.match(errors[0], /internal\/brandnewpkg/);
  assert.match(errors[0], /lc_covers|BACKEND_ONLY_PACKAGES/);
  // The explicit list is the sanctioned second way to classify (synthetic entry here; the real list is derived data).
  assert.deepEqual(classifyBackendCoverage(usage, dirs, { ...BACKEND_ONLY_PACKAGES, "internal/brandnewpkg": "synthetic: gate-mechanism test entry" }), []);
});

test("BACKEND_ONLY_PACKAGES entries are internal packages with a real one-line reason", async () => {
  const { BACKEND_ONLY_PACKAGES } = await import("../../scripts/dev/pr-modes.mjs");
  for (const [pkg, why] of Object.entries(BACKEND_ONLY_PACKAGES)) {
    assert.ok(pkg.startsWith("internal/"), `${pkg}: only internal packages may be backend-only`);
    assert.ok(typeof why === "string" && why.length > 10, `${pkg}: needs a one-line reason`);
  }
});

test("every browser-universe mode declares lc_covers; a Go-running one must not declare it empty", async () => {
  const { classifyBackendCoverage } = await import("../../scripts/dev/check-backend-coverage.mjs");
  const dirs = internalDirs();
  // Synthetic probe: a browser mode whose body runs Go but declares lc_covers="" is a silent deselection hole -> red.
  const goProbe = appendBrowserMode(usage, "--browser-probe-go", "  go test -tags browser -run TestProbe ./tests/foundation", "");
  assert.ok(classifyBackendCoverage(goProbe, dirs).some((e) => e.includes("--browser-probe-go")), "Go-running browser mode with empty covers must fail");
  // A node-only browser mode with empty covers is legal (there is no Go harness to wire); a prose comment mentioning
  // "go test" must not flip the verdict (the gate strips Bash comments before looking for Go invocations).
  const nodeProbe = appendBrowserMode(usage, "--browser-probe-node", "  # no go test events here (prose only)\n  pnpm exec playwright test tests/e2e/probe.spec.ts", "");
  assert.deepEqual(classifyBackendCoverage(nodeProbe, dirs), []);
  // A browser mode with NO lc_covers line at all is red (the declaration is mandatory across the browser universe).
  const bareProbe = appendBrowserMode(usage, "--browser-probe-bare", "  pnpm exec playwright test tests/e2e/probe.spec.ts", null);
  assert.ok(classifyBackendCoverage(bareProbe, dirs).some((e) => e.includes("--browser-probe-bare")), "missing lc_covers line must fail");
});

test("covers entries must match a real internal package directory; parent prefixes are legal, stale entries are red", async () => {
  const { classifyBackendCoverage } = await import("../../scripts/dev/check-backend-coverage.mjs");
  const dirs = internalDirs();
  const stale = appendBrowserMode(usage, "--browser-probe-stale", "  go test ./tests/foundation", "internal/removedpkg");
  assert.ok(classifyBackendCoverage(stale, dirs).some((e) => e.includes("internal/removedpkg")), "a covers entry matching no package directory must fail");
  // A parent prefix legally covers every directory below it (kept explicit: it must NOT silently pass as stale-free).
  const parent = appendBrowserMode(usage, "--browser-probe-parent", "  go test ./tests/foundation", "internal/integrations");
  assert.deepEqual(classifyBackendCoverage(parent, dirs), []);
});
