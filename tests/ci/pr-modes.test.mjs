// Purpose: unit tests of the pull-request gate planner (scripts/dev/pr-modes.mjs): docs/contracts/deploy/node-test diffs run foundation-shards only; internal/, cmd/ and
//   migrations/ select browser modes from the registry's lc_covers data (CI-SELECT: the browser modes run the real Go API on the real migrated PG); any other path adds the
//   full browser set; deploy implementation changes select smoke; actual Git CLI quoting/whitespace must not reduce legacy selections.
// Depends on: scripts/dev/pr-modes.mjs, scripts/dev/test-local.sh, scripts/dev/release-gate.sh, bash and isolated local Git fixture repos.
// Used by: scripts/dev/test-node.sh, CI.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync, readdirSync, mkdirSync, mkdtempSync, writeFileSync, rmSync, renameSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { EXCLUDED_MODES, browserModes, modeEntries, planPr } from "../../scripts/dev/pr-modes.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const usage = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8");
const FOUNDATION = ["foundation-shards"];
const full = browserModes(usage);
// CI-SELECT oracle, computed independently of the planner's own helpers: the browser universe with lc_fixture=pg, read
// from the same registry data. Backend changes that must reach every PG-backed browser mode are asserted against this.
const pgFull = modeEntries(usage).filter((e) => full.includes(e.name) && e.fixture === "pg").map((e) => e.name);


// Append exactly one native registry entry; mode position/usage spelling is irrelevant. covers !== null adds the
// lc_covers declaration line (CI-SELECT registry data: the internal packages the mode's harness wires).
function appendMode(source, name, command, covers = null) {
  const entry = `  ${name})
    lc_build=none
    lc_fixture=pg
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

// CI-SELECT split of the old "backend-only paths" test: its docs/contracts/deploy/node-test half genuinely cannot alter
// a browser and still runs foundation-shards only. Its internal/, cmd/ and migrations/ half moved to the tests below —
// those paths run INSIDE the browser modes (real Go API + real PG fixture), the verified root cause of PR #30/#24.
test("docs, contracts, deploy files and node-test dirs run foundation-shards only", () => {
  const r = planPr(["contracts/invariants.json", "go.mod", "go.sum", "tests/foundation/orders_test.go", "deploy/compose.yml", "README.md", "apps/admin/NOTES.md", "tests/deploy/deploy-prep-r3.test.mjs", "tests/ci/pr-modes.test.mjs", "output/x/DELIVERY.md"], usage, () => ["package foundation_test\n"]);
  assert.deepEqual(r.modes, FOUNDATION);
});

test("node tests of deploy scripts and of the CI planner are backend-only (they cannot change a browser)", () => {
  assert.deepEqual(planPr(["tests/deploy/deploy-prep-r3.test.mjs", "tests/ci/pr-modes.test.mjs"], usage).modes, FOUNDATION);
  assert.deepEqual(planPr(["tests/deploy/x.test.mjs", "scripts/dev/pr-modes.mjs"], usage).modes.length > 1, true); // the planner itself still counts as UI
});

test("internal/ is NOT backend-only any more: a package no mode declares falls to the conservative PG rule (CI-SELECT)", () => {
  const modes = planPr(["internal/a.go"], usage).modes;
  assert.ok(modes.length > FOUNDATION.length, "internal/a.go must select browser modes");
  for (const m of pgFull) assert.ok(modes.includes(m), `internal/a.go did not select ${m}`);
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

// ---- CI-SELECT (owner-approved 2026-10-10): internal/, cmd/ and migrations/ are NOT backend-only. ----
// Verified root cause: the browser modes run the real Go API on the real PG fixture, but the planner treated internal/,
// cmd/ and migrations/ as backend-only, so PR #30 (internal/live/stream.go — serves the A2/A3 comment stream that
// --browser-live-console clicks) and PR #24 (internal/integrations/metareply) never ran --browser-live-console, and push
// runs on r3/integration re-verified no trunk browser health at all. The fix is registry DATA: every browser mode
// declares the internal packages its harness exercises in its `lc_covers="..."` line, and the planner selects from that
// data. Round 2 (owner decision 2026-10-10 "narrow per domain + nightly") replaced round 1's import-closure covers —
// every harness boots the full API, so the closure gave 48 of 51 modes all 70 packages and any backend PR selected 48
// modes — with per-mode DOMAIN evidence: the API paths each mode's Playwright spec and Go harness actually call, mapped
// through the httpapi/identityhttp/buyerhttp route tables to the serving package. Derived by
// output/ci-select-backend-browser/tools/derive-narrow-covers.mjs, recorded per mode (path, handler file:line, package)
// in output/ci-select-backend-browser/covers-derivation.json; the nightly full browser matrix is the safety net.

test("internal/live/stream.go selects --browser-live-console (PR #30 root cause)", () => {
  assert.ok(planPr(["internal/live/stream.go"], usage).modes.includes("--browser-live-console"));
});

test("round 3: the real PR #30 diff selects live-console without the shared-httpapi explosion", () => {
  const files = execFileSync("git", ["show", "--format=", "--name-only", "b1bfbeb3"], { cwd: root, encoding: "utf8" }).split("\n").filter(Boolean);
  assert.ok(files.includes("internal/httpapi/live_stream.go"));
  const modes = planPr(files, usage).modes;
  assert.ok(modes.includes("--browser-live-console"));
  assert.ok(modes.length <= 25, `PR #30 must be domain-scoped, not all 48 browsers: ${modes.length}`);
});

test("round 3: live_stream transport alone selects live-console but not CVS", () => {
  const modes = planPr(["internal/httpapi/live_stream.go"], usage).modes;
  assert.ok(modes.includes("--browser-live-console"));
  assert.ok(!modes.includes("--browser-cvs"));
  assert.ok(modes.length <= 25, `live_stream transport selected ${modes.length} modes`);
});

test("round 3: handler.go and unknown httpapi files remain conservative", () => {
  for (const file of ["internal/httpapi/handler.go", "internal/httpapi/not_yet_classified.go"]) {
    const modes = planPr([file], usage).modes;
    for (const mode of pgFull) assert.ok(modes.includes(mode), `${file} omitted ${mode}`);
  }
});

test("round 3: a file-level lc_covers entry is exact, not its sibling directory", () => {
  const one = appendMode(usage, "--browser-file-one", "  go test ./tests/foundation", "internal/fileprobe/one.go");
  const source = appendMode(one, "--browser-file-two", "  go test ./tests/foundation", "internal/fileprobe/two.go");
  assert.deepEqual(planPr(["internal/fileprobe/one.go"], source).modes, [...FOUNDATION, "--browser-file-one"]);
  assert.deepEqual(planPr(["internal/fileprobe/two.go"], source).modes, [...FOUNDATION, "--browser-file-two"]);
});

test("round 3: a nonbrowser exact-file declaration cannot replace browser acceptance", () => {
  const source = appendMode(usage, "--unit-file-probe", "  go test ./tests/foundation", "internal/fileprobe/one.go");
  const modes = planPr(["internal/fileprobe/one.go"], source).modes;
  for (const mode of pgFull) assert.ok(modes.includes(mode));
  assert.ok(!modes.includes("--unit-file-probe"));
});

test("internal/integrations/metareply/x.go selects --browser-live-console (PR #24 root cause)", () => {
  assert.ok(planPr(["internal/integrations/metareply/x.go"], usage).modes.includes("--browser-live-console"));
});

// ---- CI-SELECT round 2: narrow per-domain covers. The two root-cause tests above must STAY fixed while unrelated
// domains stop being selected. Thresholds below encode the derived data (see r2-diagnostics.txt): live 18 modes,
// inbox 14, metareply 2; the nightly matrix catches anything the per-domain evidence missed.

test("round 2 narrowness: internal/live/stream.go does NOT select unrelated domains such as --browser-cvs", () => {
  const modes = planPr(["internal/live/stream.go"], usage).modes;
  assert.ok(modes.includes("--browser-live-console"), "PR #30 root cause must stay fixed");
  assert.ok(!modes.includes("--browser-cvs"), "the CVS-shipping harness exercises no internal/live route (covers-derivation.json)");
  assert.ok(modes.length <= 25, `a single-domain live change selected ${modes.length} modes; round 2 derives 18 + foundation (nightly is the safety net)`);
});

test("round 2 evidence: a metareply change selects exactly the modes whose harnesses drive MetaReply (live console + e2e)", () => {
  // internal/inbox does not import metareply and the inbox specs never call a metareply-served route — the evidence
  // selects --browser-live-console (comment fixtures) and --browser-e2e only; guessing more would re-widen round 1.
  assert.deepEqual(planPr(["internal/integrations/metareply/reply.go"], usage).modes, [...FOUNDATION, "--browser-live-console", "--browser-e2e"]);
});

test("round 2 narrowness: an inbox change selects the inbox modes but not the live console", () => {
  const modes = planPr(["internal/inbox/inbox.go"], usage).modes;
  assert.ok(modes.includes("--browser-inbox"));
  assert.ok(!modes.includes("--browser-live-console"), "the live-console harness never touches internal/inbox (no import, no exercised route)");
  assert.ok(modes.length <= 20, `inbox is one domain; selected ${modes.length} modes (derived: 14 + foundation)`);
});

test("round 2 SHARED_BACKEND_PACKAGES: every entry carries a reason and selects all PG browser modes", async () => {
  const { SHARED_BACKEND_PACKAGES } = await import("../../scripts/dev/pr-modes.mjs");
  assert.ok(Object.keys(SHARED_BACKEND_PACKAGES).length >= 9, "round 2 classified 9 shared packages; this is a floor, not a ceiling");
  for (const [pkg, why] of Object.entries(SHARED_BACKEND_PACKAGES)) {
    assert.ok(pkg.startsWith("internal/"), `${pkg}: only internal packages may be shared`);
    assert.ok(typeof why === "string" && why.length > 10, `${pkg}: needs a one-line reason`);
    const modes = planPr([pkg.endsWith(".go") ? pkg : `${pkg}/x.go`], usage).modes;
    for (const m of pgFull) assert.ok(modes.includes(m), `${pkg}/x.go did not select ${m}`);
  }
});

test("round 2 data invariant: SHARED packages are subtracted from lc_covers (they select all PG modes anyway)", async () => {
  const { SHARED_BACKEND_PACKAGES } = await import("../../scripts/dev/pr-modes.mjs");
  for (const e of modeEntries(usage))
    for (const c of e.covers ?? [])
      assert.ok(!(c in SHARED_BACKEND_PACKAGES), `${e.name} still lists shared package ${c} in lc_covers; re-run insert-narrow-covers.mjs`);
});

test("round 2 BACKEND_ONLY_PACKAGES: the real cmd/worker-only entries select no browser mode", async () => {
  const { BACKEND_ONLY_PACKAGES, backendBrowserModes } = await import("../../scripts/dev/pr-modes.mjs");
  assert.ok(Object.keys(BACKEND_ONLY_PACKAGES).length >= 3, "round 2 classified tlsask, retention and ecpayroute");
  for (const pkg of Object.keys(BACKEND_ONLY_PACKAGES))
    assert.deepEqual(backendBrowserModes([`${pkg}/x.go`], usage), [], `${pkg} must select nothing (a cmd/** change still selects all Go-booting modes via the cmd rule)`);
});

test("round 2: the conservative rules reach the Go-booting 48th mode (--browser-tracking-backfill)", () => {
  // lc_fixture=none, but it boots the real Go API against the real PG through scripts/dev/test-focused.sh — the
  // fixture marker describes the shared fixture script, not whether the mode runs Go. The Go-booting set is
  // lc_fixture=pg ∪ non-empty lc_covers (the gate forces a Go-running mode to declare covers; node-only modes
  // declare "" and stay out).
  for (const p of ["cmd/api/main.go", "migrations/0169_x.sql", "internal/platform/x.go", "internal/httperror/x.go", "internal/brandnewpkg/x.go"]) {
    const modes = planPr([p], usage).modes;
    assert.ok(modes.includes("--browser-tracking-backfill"), `${p} must select --browser-tracking-backfill (real Go + real PG)`);
  }
});

test("a new migration selects every PG-fixture browser mode (the modes run on the migrated schema)", () => {
  assert.ok(pgFull.length > 20, "the registry moved; update the oracle with it");
  const modes = planPr(["migrations/0169_x.sql"], usage).modes;
  for (const m of pgFull) assert.ok(modes.includes(m), `migrations/0169_x.sql did not select ${m}`);
});

test("docs-only and contracts-only diffs still select no browser mode", () => {
  assert.deepEqual(planPr(["docs/delivery/GATES.md", "contracts/invariants.json", "架构.md"], usage).modes, FOUNDATION);
});

test("cmd/ changes select every PG-fixture browser mode (the modes drive the binaries cmd/ builds)", () => {
  const modes = planPr(["cmd/api/main.go"], usage).modes;
  for (const m of pgFull) assert.ok(modes.includes(m), `cmd/api/main.go did not select ${m}`);
});

test("shared platform packages select every PG-fixture browser mode", async () => {
  const { SHARED_BACKEND_PACKAGES } = await import("../../scripts/dev/pr-modes.mjs");
  for (const pkg of ["internal/platform", "internal/httpapi/handler.go", "internal/command"]) {
    assert.ok(pkg in SHARED_BACKEND_PACKAGES, `${pkg} must be a declared shared platform package`);
    const modes = planPr([pkg.endsWith(".go") ? pkg : `${pkg}/x.go`], usage).modes;
    for (const m of pgFull) assert.ok(modes.includes(m), `${pkg}/x.go did not select ${m}`);
  }
});

test("an internal package no mode declares selects every PG-fixture browser mode until the gate classifies it", () => {
  const modes = planPr(["internal/brandnewpkg/x.go"], usage).modes;
  for (const m of pgFull) assert.ok(modes.includes(m), `internal/brandnewpkg/x.go did not select ${m}`);
});

test("lc_covers is registry DATA: declaring modes are selected by prefix match, non-declaring modes are not", () => {
  const source = appendMode(usage, "--probe-covers", "  go test ./tests/foundation", "internal/probepkg");
  assert.deepEqual(planPr(["internal/probepkg/x.go"], source).modes, [...FOUNDATION, "--probe-covers"]);
  assert.deepEqual(planPr(["internal/probepkg/sub/deep.go"], source).modes, [...FOUNDATION, "--probe-covers"], "a nested package is covered by its parent's entry");
  const modes = planPr(["internal/other/x.go"], source).modes; // nothing declares internal/other -> conservative PG rule
  for (const m of pgFull) assert.ok(modes.includes(m), m);
  assert.ok(!modes.includes("--probe-covers"), "the probe declares no other package");
});

test("backend changes never select the node-only browser modes (no Go harness, no PG fixture)", () => {
  const nodeOnly = ["--browser-platform-site", "--browser-admin-shell", "--browser-picklist"];
  for (const p of ["internal/live/stream.go", "cmd/api/main.go", "migrations/0169_x.sql", "internal/brandnewpkg/x.go"]) {
    const modes = planPr([p], usage).modes;
    for (const m of nodeOnly) assert.ok(!modes.includes(m), `${p} must not select ${m}`);
  }
});

test("BACKEND_ONLY_PACKAGES is explicit classified data; listed packages select no browser mode (mechanism, synthetic list)", async () => {
  const { BACKEND_ONLY_PACKAGES, backendBrowserModes } = await import("../../scripts/dev/pr-modes.mjs");
  for (const [pkg, why] of Object.entries(BACKEND_ONLY_PACKAGES)) {
    assert.ok(pkg.startsWith("internal/"), `${pkg}: only internal packages may be backend-only`);
    assert.ok(typeof why === "string" && why.length > 10, `${pkg}: needs a one-line reason`);
  }
  // Round 1 left BACKEND_ONLY empty; round 2 (2026-10-10) classified tlsask/retention/ecpayroute (asserted above).
  // The mechanism must keep working for any future classification, proved here with a synthetic list.
  assert.deepEqual(backendBrowserModes(["internal/classified/x.go"], usage, { "internal/classified": "synthetic: mechanism test" }), []);
  assert.ok(backendBrowserModes(["internal/classified/x.go"], usage).length >= pgFull.length, "an undeclared package stays conservative until classified");
});

test("a registry without lc_covers data (historical source) stays maximally conservative", async () => {
  const { backendBrowserModes } = await import("../../scripts/dev/pr-modes.mjs");
  // Shape of a historical test-local.sh: usage line + dispatch branch, no BEGIN MODE REGISTRY (pre-T02 refs the CLI can meet).
  const legacy = "printf 'Usage: bash scripts/dev/test-local.sh [foundation|--browser-x]\\n'\nif [[ $test_mode == --browser-x ]]; then\n  go test ./tests/foundation\nfi\n";
  assert.deepEqual(backendBrowserModes(["internal/live/stream.go"], legacy), ["--browser-x"]);
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

test("CLI selects modes the base added after divergence when the checkout is the PR merge (PR #22 review P1)", (t) => {
  const r = repository(t), file = "tests/foundation/synthetic_process_test.go";
  r.put(file, "//go:build browser\n\npackage foundation_test\n"); const fork = r.commit();
  r.put(file, "//go:build browser\n\npackage foundation_test\n// head edit\n"); const head = r.commit();
  // The base branch registers a new browser mode after the PR forked.
  r.git("checkout", "--quiet", "-b", "base-later", fork);
  const arm = /^  --browser-inbox\)\n[\s\S]*?(?=^  --)/m.exec(usage)[0];
  r.put("scripts/dev/test-local.sh", usage.replace(arm, arm.replace("--browser-inbox)", "--browser-synthetic-new)") + arm)); const base = r.commit();
  // CI checks out GitHub's merge of head into base: its registry has the new mode, the head's does not.
  r.git("merge", "--quiet", "--no-edit", head);
  assert.ok(r.cli([base, head]).modes.includes("--browser-synthetic-new"), "a mode the merge adds must be selected for a tagged-source change");
});

test("CLI reads the merge checkout's source when the base adds a browser tag after the fork (PR #22 review P1)", (t) => {
  const r = repository(t), file = "tests/foundation/synthetic_merge_test.go";
  r.put(file, "package foundation_test\n\nfunc a() {}\n"); const fork = r.commit();
  r.put(file, "package foundation_test\n\nfunc a() {}\n\nfunc headEdit() {}\n"); const head = r.commit();
  r.git("checkout", "--quiet", "-b", "base-tagged", fork);
  r.put(file, "//go:build browser\n\npackage foundation_test\n\nfunc a() {}\n"); const base = r.commit();
  // GitHub's conflict-free merge has the tag (from base) and the head edit; neither merge-base nor head has the tag.
  r.git("merge", "--quiet", "--no-edit", head);
  assert.ok(r.cli([base, head]).modes.includes("--studio-backend"), "the merged file is browser-tagged, so tagged runners must be selected");
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
