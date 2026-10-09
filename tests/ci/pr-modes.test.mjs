// Purpose: unit tests of the pull-request gate planner (scripts/dev/pr-modes.mjs): backend-only and docs-only diffs run foundation-shards only, any other path adds the full browser
//   set, deploy implementation changes select smoke; actual Git CLI quoting/whitespace must not reduce legacy selections.
// Depends on: scripts/dev/pr-modes.mjs, scripts/dev/test-local.sh, scripts/dev/release-gate.sh, bash and isolated local Git fixture repos.
// Used by: scripts/dev/test-node.sh, CI.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync, readdirSync, mkdirSync, mkdtempSync, writeFileSync, rmSync, renameSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { EXCLUDED_MODES, browserModes, planPr } from "../../scripts/dev/pr-modes.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const usage = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8");
const FOUNDATION = ["foundation-shards"];
const full = browserModes(usage);


// Append exactly one native registry entry; mode position/usage spelling is irrelevant.
function appendMode(source, name, command) {
  const entry = `  ${name})
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
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

test("backend-only paths run foundation-shards only", () => {
  const r = planPr(["internal/orders/x.go", "cmd/api/main.go", "migrations/0130_x.sql", "contracts/invariants.json", "go.mod", "go.sum", "tests/foundation/orders_test.go", "deploy/compose.yml", "README.md", "apps/admin/NOTES.md"], usage, () => ["package foundation_test\n"]);
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

test("legacy browser_* paths still select all browsers; a known untagged foundation file stays backend-only", () => {
  const untagged = () => ["package foundation_test\n"];
  assert.deepEqual(planPr(["tests/foundation/browser_x_test.go"], usage, untagged).modes, [...FOUNDATION, ...full]);
  assert.deepEqual(planPr(["tests/foundation/x_test.go"], usage, untagged).modes, FOUNDATION);
});

test("legacy deploy prefixes remain selected and unrelated paths do not select smoke", () => {
  assert.equal(planPr(["deploy/scripts/smoke.sh"]).deploy, true);
  assert.equal(planPr(["scripts/deploy-prep.sh"]).deploy, true);
  assert.equal(planPr(["scripts/deploy/x.sh"]).deploy, true);
  assert.equal(planPr(["internal/a.go", "docs/x.md"]).deploy, false);
  assert.equal(planPr(["scripts/dev/test-local.sh"]).deploy, false);
});

test("the browser set is release-gate's browser-mode universe minus the documented exclusions", () => {
  const src = readFileSync(path.join(root, "scripts/dev/release-gate.sh"), "utf8").split("\n");
  const derive = src.filter((l) => /^modes=\$\(bash scripts\/dev\/test-local\.sh --list\)/.test(l) || /^browser_modes=\$\(printf /.test(l)).join("\n");
  assert.equal(derive.split("\n").length, 2, "release-gate.sh derivation lines not found; update this test with the script");
  const universe = execFileSync("bash", ["-c", `${derive}\nprintf '%s\\n' $browser_modes`], { cwd: root, encoding: "utf8" }).trim().split("\n");
  assert.ok(universe.length > 20);
  assert.deepEqual(full, universe.filter((m) => !(m in EXCLUDED_MODES)));
  for (const [m, why] of Object.entries(EXCLUDED_MODES)) { assert.ok(universe.includes(m), `${m} is not in the universe any more: drop the exclusion`); assert.ok(why.length > 10); }
});

test("every real browser-tagged foundation file selects registry browser modes, including non-browser names", () => {
  const tagged = readdirSync(path.join(root, "tests/foundation")).filter((file) => file.endsWith(".go") && /^\/\/go:build.*\bbrowser\b/m.test(readFileSync(path.join(root, "tests/foundation", file), "utf8")));
  assert.ok(tagged.includes("account_process_test.go") && tagged.includes("studio_process_test.go"));
  for (const file of tagged) {
    const modes = planPr([`tests/foundation/${file}`], usage).modes;
    for (const mode of full) assert.ok(modes.includes(mode), `${file} omitted ${mode}`);
    // The actual registry runs TestStudioBackendSTU03RealAPIWorkerRestart in this non-browser-named mode.
    assert.ok(modes.includes("--studio-backend"), `${file} omitted the tagged process runner`);
  }
});

test("tagged runner discovery follows registry additions, command arrays and line continuations", () => {
  const source = appendMode(usage, "--new-tagged-process", '  args=(-race -tags=browser \\\n    -run TestFuture ./tests/foundation)\n  go test "${args[@]}"');
  const plan = planPr(["tests/foundation/account_process_test.go"], source);
  assert.ok(plan.modes.includes("--new-tagged-process"), "registry additions must not need a planner hand-list");
  assert.ok(!plan.modes.includes("--stripe-browser"), "existing secret-dependent exclusion remains");
});

test("deploy smoke is selected for its workflow, verdict/helpers and executable test inputs", () => {
  for (const file of [".github/workflows/deploy-smoke.yml", ".github/workflows/deploy-smoke.yaml", ".github/scripts/smoke-verdict.py", ".github/scripts/renamed-verdict.py", "deploy/scripts/smoke.sh", "deploy/scripts/smoke-r3.sh", "deploy/scripts/smoke-browser.mjs", "tests/deploy/deploy-prep-r3.test.mjs"])
    assert.equal(planPr([file], usage).deploy, true, file);
});

test("new selection never drops legacy modes or deploy for any tracked path", () => {
  // Frozen 424f17cc selection contract, intentionally independent of the repaired classifier.
  const prefixes = ["internal/", "cmd/", "migrations/", "contracts/", "docs/", "deploy/", "output/", "tests/foundation/", "tests/deploy/", "tests/ci/"];
  const files = execFileSync("git", ["ls-files", "-z"], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 }).split("\0").filter(Boolean);
  for (const file of [...files, "unknown/future.file", "tests/foundation/browser_removed_test.go"]) {
    const oldBackend = file.endsWith(".md") || file === "go.mod" || file === "go.sum" || (prefixes.some((p) => file.startsWith(p)) && !file.startsWith("tests/foundation/browser_"));
    const oldModes = oldBackend ? FOUNDATION : [...FOUNDATION, ...full], current = planPr([file], usage);
    for (const mode of oldModes) assert.ok(current.modes.includes(mode), `${file} lost ${mode}`);
    if (file.startsWith("deploy/") || file.startsWith("scripts/deploy")) assert.equal(current.deploy, true, file);
  }
});

function repository(t) {
  const parent = path.join(root, "output/ci-pr-modes-coverage"); mkdirSync(parent, { recursive: true });
  const dir = mkdtempSync(path.join(parent, "fixture-"));
  t.after(() => rmSync(dir, { recursive: true, force: true })); // Only this test's generated Git fixture.
  const git = (...args) => execFileSync("git", args, { cwd: dir, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] }).trim();
  const put = (file, text) => { mkdirSync(path.dirname(path.join(dir, file)), { recursive: true }); writeFileSync(path.join(dir, file), text); };
  git("init", "--quiet"); git("config", "user.name", "Synthetic planner test"); git("config", "user.email", "fixture@example.invalid");
  git("config", "commit.gpgsign", "false"); git("config", "core.hooksPath", path.join(dir, ".git/no-hooks"));
  put("scripts/dev/pr-modes.mjs", readFileSync(path.join(root, "scripts/dev/pr-modes.mjs"), "utf8")); put("scripts/dev/test-local.sh", usage);
  const commit = () => { git("add", "-A"); git("commit", "--quiet", "-m", "synthetic fixture"); return git("rev-parse", "HEAD"); };
  const cli = (args, input) => JSON.parse(execFileSync(process.execPath, [path.join(dir, "scripts/dev/pr-modes.mjs"), ...args], { cwd: dir, encoding: "utf8", input, env: { ...process.env, GITHUB_OUTPUT: "" }, stdio: ["pipe", "pipe", "pipe"] }));
  return { dir, git, put, commit, cli };
}

test("CLI classifies merge-base and explicit head, including removed tags, deleted files and renames", (t) => {
  const r = repository(t), file = "tests/foundation/process_test.go";
  r.put(file, "//go:build browser\n\npackage foundation_test\n"); const base = r.commit();
  r.put(file, "package foundation_test\n"); const head = r.commit();
  assert.ok(r.cli([base, head]).modes.includes("--studio-backend"), "removed tag is still a browser-affecting edit");
  // Give the supplied base a later untagged state: the merge-base still carries the browser tag.
  r.git("checkout", "--quiet", "-b", "other-base", base); r.put(file, "package foundation_test\n// other base\n"); const otherBase = r.commit();
  assert.ok(r.cli([otherBase, head]).modes.includes("--browser-identity"), "classify the diff's merge-base, not the latest base branch");
  r.git("checkout", "--quiet", "-b", "rename", base);
  renameSync(path.join(r.dir, file), path.join(r.dir, "tests/foundation/new\tname_test.go")); r.put("tests/foundation/new\tname_test.go", "package foundation_test\n"); const renamed = r.commit();
  assert.ok(r.cli([base, renamed]).modes.includes("--browser-identity"), "old rename path must stay in the diff");
  r.git("checkout", "--quiet", "-b", "deleted", base); rmSync(path.join(r.dir, file)); const deleted = r.commit();
  assert.ok(r.cli([base, deleted]).modes.includes("--studio-backend"), "deleted source must be read from the base");
});

test("CLI explicit head and stdin include tags outside the checkout and uncommitted tag removals", (t) => {
  const r = repository(t), file = "tests/foundation/synthetic_process_test.go";
  r.put(file, "package foundation_test\n"); const base = r.commit();
  r.put(file, "//go:build browser && linux\n\npackage foundation_test\n"); const head = r.commit();
  r.git("checkout", "--quiet", base);
  assert.ok(r.cli([base, head]).modes.includes("--browser-identity"), "read supplied head instead of current checkout");
  r.git("checkout", "--quiet", head); r.put(file, "package foundation_test\n");
  assert.ok(r.cli(["--stdin"], `${file}\n`).modes.includes("--studio-backend"), "stdin must consider committed pre-edit tags");
});

test("compound/legacy tags and unknown source are conservative; known untagged source stays backend-only", () => {
  for (const text of ["//go:build browser && linux\npackage f\n", "// +build browser,linux\npackage f\n", "/*\npackage license example\n*/\n//go:build browser\n\npackage f\n", null])
    assert.ok(planPr(["tests/foundation/new_process.go"], usage, () => [text]).modes.includes("--studio-backend"));
  assert.deepEqual(planPr(["tests/foundation/new_process.go"], usage, () => [null, "package f\nconst browser = 1\n"]).modes, FOUNDATION);
});

test("explicit-head CLI derives tagged runner modes from that head's real registry", (t) => {
  const r = repository(t), file = "tests/foundation/new_process_test.go";
  r.put(file, "package foundation_test\n"); const base = r.commit();
  r.put(file, "//go:build browser\n\npackage foundation_test\n");
  r.put("scripts/dev/test-local.sh", appendMode(usage, "--head-tagged-process", "  go test -tags browser ./tests/foundation"));
  const head = r.commit(); r.git("checkout", "--quiet", base);
  assert.ok(r.cli([base, head]).modes.includes("--head-tagged-process"));
});

// Frozen legacy CLI classifier; feed Git's REAL display output, not raw API
// paths. The prior API-only 6666-path oracle cannot observe this representation.
function legacyDisplayPlan(display) {
  const paths = display.split("\n").map((s) => s.trim()).filter(Boolean);
  const prefixes = ["internal/", "cmd/", "migrations/", "contracts/", "docs/", "deploy/", "output/", "tests/foundation/", "tests/deploy/", "tests/ci/"];
  const backend = (p) => p.endsWith(".md") || p === "go.mod" || p === "go.sum" ||
    (prefixes.some((prefix) => p.startsWith(prefix)) && !p.startsWith("tests/foundation/browser_"));
  return { modes: paths.some((p) => !backend(p)) ? [...FOUNDATION, ...full] : FOUNDATION,
    deploy: paths.some((p) => p.startsWith("deploy/") || p.startsWith("scripts/deploy")) };
}

for (const quotePath of ["true", "false"]) {
  for (const file of ["架构.md", "internal/示例.go", "tests/foundation/plain\tname_test.go", " deploy/scripts/synthetic-smoke.sh"]) {
    test(`real Git CLI preserves legacy selection: quotePath=${quotePath}, ${JSON.stringify(file)}`, (t) => {
      const r = repository(t);
      r.git("config", "core.quotePath", quotePath);
      r.put(file, "// synthetic untagged file\n"); const base = r.commit();
      r.put(file, "// synthetic edit, no browser tag\n"); const head = r.commit();
      const display = r.git("diff", "--name-only", `${base}...${head}`);
      const old = legacyDisplayPlan(display), next = r.cli([base, head]);
      // Pin that these cases really exercise quoted paths / trimming.
      if (quotePath === "true" && !file.startsWith(" ")) assert.ok(display.startsWith('"'));
      if (file.startsWith(" ")) assert.equal(old.deploy, true);
      for (const mode of old.modes) assert.ok(next.modes.includes(mode), `${JSON.stringify(file)} lost ${mode}`);
      if (old.deploy) assert.equal(next.deploy, true, "legacy trim must retain deploy-smoke");
    });
  }
}

test("stdin unions legacy trimming without trimming the source lookup path", (t) => {
  const r = repository(t), file = " tests/foundation/plain.go";
  r.put(file, "// synthetic untagged file\n"); r.commit();
  const paths = `${file}\n deploy/scripts/synthetic-smoke.sh\n`;
  const old = legacyDisplayPlan(paths), next = r.cli(["--stdin"], paths);
  for (const mode of old.modes) assert.ok(next.modes.includes(mode));
  assert.equal(next.deploy, true);
  assert.ok(!next.modes.includes("--studio-backend"), "legacy spelling is only classified, never used as a missing source lookup");
});
