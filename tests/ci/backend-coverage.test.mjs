// Purpose: red→green tests of the CI-SELECT classification gate (scripts/dev/check-backend-coverage.mjs): every directory under
//   internal/ must be classified by a browser mode's lc_covers registry data (round 2: the NARROW domain packages the mode's
//   harness actually exercises), by SHARED_BACKEND_PACKAGES (cross-cutting boot/auth packages that select all PG browser modes)
//   or by BACKEND_ONLY_PACKAGES (cmd/worker-only packages) — the latter two with a one-line reason — so a new backend package
//   fails the gate until someone classifies it; a browser mode whose body runs Go must not carry empty lc_covers (that would
//   silently deselect it for every backend change), and every covers entry must match a real internal package directory
//   (stale entries select nothing).
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

test("round 3: a new httpapi file is unclassified until explicitly covered", async () => {
  const { classifyBackendFiles } = await import("../../scripts/dev/check-backend-coverage.mjs");
  assert.equal(typeof classifyBackendFiles, "function");
  const file = "internal/httpapi/unclassified_route.go";
  const files = execFileSync("git", ["ls-files", "-z", "--", "internal/"], { cwd: root, encoding: "utf8" }).split("\0").filter((f) => f.endsWith(".go"));
  const errors = classifyBackendFiles(usage, [...files, file]);
  assert.equal(errors.length, 1, JSON.stringify(errors));
  assert.match(errors[0], /unclassified_route.go.*classified by nobody/);
  const covered = appendBrowserMode(usage, "--browser-file-probe", "  go test ./tests/foundation", file);
  assert.deepEqual(classifyBackendFiles(covered, [...files, file]), []);
});

test("round 3: split packages cannot be re-widened by an ancestor lc_covers entry", async () => {
  const { classifyBackendFiles } = await import("../../scripts/dev/check-backend-coverage.mjs");
  assert.equal(typeof classifyBackendFiles, "function");
  const files = execFileSync("git", ["ls-files", "-z", "--", "internal/"], { cwd: root, encoding: "utf8" }).split("\0").filter((f) => f.endsWith(".go"));
  const widened = appendBrowserMode(usage, "--browser-broad-probe", "  go test ./tests/foundation", "internal/httpapi");
  assert.ok(classifyBackendFiles(widened, files).some((e) => /internal\/httpapi.*file-level/.test(e)));
});

test("round 3: stale file entries fail independently of a valid sibling package", async () => {
  const { classifyBackendFiles } = await import("../../scripts/dev/check-backend-coverage.mjs");
  assert.equal(typeof classifyBackendFiles, "function");
  const files = execFileSync("git", ["ls-files", "-z", "--", "internal/"], { cwd: root, encoding: "utf8" }).split("\0").filter((f) => f.endsWith(".go"));
  const stale = appendBrowserMode(usage, "--browser-file-stale", "  go test ./tests/foundation", "internal/httpapi/removed_route.go");
  assert.ok(classifyBackendFiles(stale, files).some((e) => /removed_route.go.*stale/.test(e)));
});

test("round 3: foundation-only file coverage does not classify a transport file", async () => {
  const { classifyBackendFiles } = await import("../../scripts/dev/check-backend-coverage.mjs");
  const files = execFileSync("git", ["ls-files", "-z", "--", "internal/"], { cwd: root, encoding: "utf8" }).split("\0").filter((f) => f.endsWith(".go"));
  const file = "internal/httpapi/unclassified_route.go";
  const onlyUnit = appendBrowserMode(usage, "--unit-file-probe", "  go test ./tests/foundation", file);
  assert.ok(classifyBackendFiles(onlyUnit, [...files, file]).some((e) => e.includes(`${file} is classified by nobody`)));
});

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

// ---- CI-SELECT round 2: three-way classification (lc_covers domain data, SHARED_BACKEND_PACKAGES, BACKEND_ONLY_PACKAGES). ----

// Hermetic one-arm registry so the explicit-list tests never trip the stale-entry check on real covers data.
const miniBase = [
  "# BEGIN MODE REGISTRY",
  "lc_select_mode() {",
  'case "$1" in',
  "  # APPEND MODES HERE",
  "esac",
  "}",
  "# END MODE REGISTRY",
].join("\n") + "\n";
const mini = appendBrowserMode(miniBase, "--browser-probe-shared", "  go test ./tests/foundation", "internal/probepkg");

test("round 2: a package no lc_covers entry reaches fails until SHARED or BACKEND_ONLY classifies it (mechanism, synthetic lists)", async () => {
  const { classifyBackendCoverage } = await import("../../scripts/dev/check-backend-coverage.mjs");
  const dirs = ["internal/probepkg", "internal/brandnewpkg"];
  const errors = classifyBackendCoverage(mini, dirs, {}, {});
  assert.equal(errors.length, 1, JSON.stringify(errors));
  assert.match(errors[0], /internal\/brandnewpkg/);
  assert.match(errors[0], /SHARED_BACKEND_PACKAGES/);
  // The injectable SHARED list classifies the package AND its subpackages — the same under() prefix semantics the
  // planner uses, so gate and selection can never drift apart.
  const shared = { "internal/brandnewpkg": "synthetic: shared mechanism test" };
  assert.deepEqual(classifyBackendCoverage(mini, dirs, {}, shared), []);
  assert.deepEqual(classifyBackendCoverage(mini, ["internal/probepkg", "internal/brandnewpkg/sub/deep"], {}, shared), []);
});

test("round 2 precedence: a covers entry inside BACKEND_ONLY, or a package both SHARED and BACKEND_ONLY, is a contradiction the gate must fail", async () => {
  const { classifyBackendCoverage } = await import("../../scripts/dev/check-backend-coverage.mjs");
  // The planner resolves BACKEND_ONLY before lc_covers (most specific wins), so this covers entry would be dead data.
  const dead = appendBrowserMode(miniBase, "--browser-probe-dead", "  go test ./tests/foundation", "internal/brandnewpkg/sub");
  const deadErrors = classifyBackendCoverage(dead, ["internal/brandnewpkg/sub"], { "internal/brandnewpkg": "synthetic: precedence test" }, {});
  assert.equal(deadErrors.length, 1, JSON.stringify(deadErrors));
  assert.match(deadErrors[0], /BACKEND_ONLY_PACKAGES/);
  // Same package classified both ways: SHARED selects all PG modes, BACKEND_ONLY selects none — refuse to guess.
  const bothErrors = classifyBackendCoverage(mini, ["internal/probepkg"], { "internal/probepkg": "synthetic: both lists" }, { "internal/probepkg": "synthetic: both lists" });
  assert.ok(bothErrors.some((e) => e.includes("both BACKEND_ONLY_PACKAGES and SHARED_BACKEND_PACKAGES")), JSON.stringify(bothErrors));
  // A covers entry that is an ANCESTOR of a BACKEND_ONLY entry is legal (the ecpay / ecpayroute shape): the ancestor
  // selects its own modes; only files inside the backend-only package resolve to none.
  const ancestor = appendBrowserMode(miniBase, "--browser-probe-ancestor", "  go test ./tests/foundation", "internal/probepkg");
  assert.deepEqual(classifyBackendCoverage(ancestor, ["internal/probepkg", "internal/probepkg/worker"], { "internal/probepkg/worker": "synthetic: nested backend-only" }, {}), []);
});

test("round 2: the real SHARED and BACKEND_ONLY entries classify with no lc_covers entry anywhere (explicit lists suffice)", async () => {
  const { classifyBackendCoverage } = await import("../../scripts/dev/check-backend-coverage.mjs");
  const { SHARED_BACKEND_PACKAGES, BACKEND_ONLY_PACKAGES } = await import("../../scripts/dev/pr-modes.mjs");
  assert.ok(Object.keys(SHARED_BACKEND_PACKAGES).length >= 9, "round 2 classified 9 shared packages");
  assert.ok(Object.keys(BACKEND_ONLY_PACKAGES).length >= 3, "round 2 classified 3 backend-only packages");
  const dirs = ["internal/probepkg", ...Object.keys(SHARED_BACKEND_PACKAGES), ...Object.keys(BACKEND_ONLY_PACKAGES)];
  assert.deepEqual(classifyBackendCoverage(mini, dirs), []);
});
