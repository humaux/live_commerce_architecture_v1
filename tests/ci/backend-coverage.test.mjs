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
import { readFileSync, mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const usage = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8");

test("round 3: the real Git inventory includes a new route before git add", async () => {
  const { backendGoFiles } = await import("../../scripts/dev/check-backend-coverage.mjs");
  assert.equal(typeof backendGoFiles, "function");
  const fixture = mkdtempSync(path.join(tmpdir(), "ci-select-file-inventory-"));
  try {
    execFileSync("git", ["init", "-q"], { cwd: fixture });
    mkdirSync(path.join(fixture, "internal/httpapi"), { recursive: true });
    writeFileSync(path.join(fixture, "internal/httpapi/tracked.go"), "// Synthetic inventory fixture, never compiled.\n");
    execFileSync("git", ["add", "internal/httpapi/tracked.go"], { cwd: fixture });
    writeFileSync(path.join(fixture, "internal/httpapi/untracked_test.go"), "// Synthetic inventory fixture, never compiled.\n");
    assert.deepEqual(backendGoFiles(fixture), ["internal/httpapi/tracked.go", "internal/httpapi/untracked_test.go"]);
  } finally { rmSync(fixture, { recursive: true, force: true }); }
});

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
  // A covers entry that is an ANCESTOR of a BACKEND_ONLY entry is legal (the ecpay shape — round 5 removed the real
  // ecpayroute entry, so the precedence mechanism stays proven with the synthetic nested probe): the ancestor
  // selects its own modes; only files inside the backend-only package resolve to none.
  const ancestor = appendBrowserMode(miniBase, "--browser-probe-ancestor", "  go test ./tests/foundation", "internal/probepkg");
  assert.deepEqual(classifyBackendCoverage(ancestor, ["internal/probepkg", "internal/probepkg/worker"], { "internal/probepkg/worker": "synthetic: nested backend-only" }, {}), []);
});

// ---- CI-SELECT round 4 (K3 P2-1): the derivation must record BFF-built admin writes reached by real UI clicks. ----
test("round 4: the tracked derivation records the clicked bank-transfer write chain and flags no write-gap mode", () => {
  const derivation = JSON.parse(readFileSync(path.join(root, "output/ci-select-backend-browser/covers-derivation.json"), "utf8"));
  const offline = derivation["--browser-checkout-offline"];
  assert.ok(Array.isArray(offline), "covers-derivation.json lost the checkout-offline entry");
  assert.ok(offline.some((e) => typeof e.via === "string" && e.via.includes("ui-flow") && e.path.includes("bank-transfer") && e.package === "internal/merchantorders"),
    "checkout-offline clicks transfer-confirm/transfer-reject (tests/admin/checkout-offline.spec.ts) -> OrderBankTransfer.tsx -> logistics-client.ts -> offline.go DecideTransfer: the ui-flow evidence must be recorded");
  const diag = readFileSync(path.join(root, "output/ci-select-backend-browser/tools/r2-diagnostics.txt"), "utf8");
  assert.ok(!/^WRITE-GAP/m.test(diag), "a mode whose spec clicks admin write controls with no write-route cover is an under-selection hole (K3 P2-1); re-derive the covers");
});

test("round 2: the real SHARED and BACKEND_ONLY entries classify with no lc_covers entry anywhere (explicit lists suffice)", async () => {
  const { classifyBackendCoverage } = await import("../../scripts/dev/check-backend-coverage.mjs");
  const { SHARED_BACKEND_PACKAGES, BACKEND_ONLY_PACKAGES } = await import("../../scripts/dev/pr-modes.mjs");
  assert.ok(Object.keys(SHARED_BACKEND_PACKAGES).length >= 9, "round 2 classified 9 shared packages");
  // Round 5 (thread 4236971117) removed ecpayroute from BACKEND_ONLY: the CVS browser gates boot the real
  // ecpayroute.Routes dispatcher, so the package is classified by lc_covers evidence, not as backend-only.
  assert.ok(Object.keys(BACKEND_ONLY_PACKAGES).length >= 2, "round 2 classified tlsask, retention and ecpayroute; round 5 removed ecpayroute (real dispatcher wiring)");
  const dirs = ["internal/probepkg", ...Object.keys(SHARED_BACKEND_PACKAGES), ...Object.keys(BACKEND_ONLY_PACKAGES)];
  assert.deepEqual(classifyBackendCoverage(mini, dirs), []);
});

// ---- CI-SELECT round 5 (PR #39 review threads): the tracked derivation evidence for the three under-selections. ----
// The planner tests (pr-modes.test.mjs round 5) prove SELECTION; these prove the EVIDENCE behind it — the regenerated
// covers-derivation.json rows and the per-mode discovery diagnostics. A silent specs=0 discovery failure is exactly
// what hid threads 4236972349/4236972354, so round 5 also makes specs=0 explicit (SPECS-ZERO) with a documented
// allowlist instead of a quiet zero.
test("round 5: covers-derivation.json records the dispatcher wiring, the orders read chain and the ledger write chain", () => {
  const derivation = JSON.parse(readFileSync(path.join(root, "output/ci-select-backend-browser/covers-derivation.json"), "utf8"));
  // Thread 4236971117: tcvEnv (taiwan_cvs_env_test.go) imports ecpayroute:47 and wires it in startDispatcher:636-644;
  // the env header names its browser consumers, so the fixture-import tier records the package per mode.
  for (const mode of ["--browser-cvs", "--browser-click-sweep"]) {
    const rows = derivation[mode] ?? [];
    assert.ok(rows.some((e) => e.package === "internal/integrations/shipping/ecpay/ecpayroute" && e.via === "go-fixture-import" && e.handler.startsWith("tests/foundation/taiwan_cvs_env_test.go")),
      `${mode} must carry go-fixture-import evidence for ecpayroute (startDispatcher -> ecpayroute.Routes)`);
  }
  // Thread 4236972349: tracking-backfill.spec.ts clicks the orders table/search/expand chain -> orders-client.ts:73
  // -> orders.go -> merchantorders.
  const tb = derivation["--browser-tracking-backfill"] ?? [];
  assert.ok(tb.some((e) => e.package === "internal/merchantorders" && e.path.includes("orders")),
    "tracking-backfill must record the orders read chain (bareSpecs merge, thread 4236972349)");
  // Thread 4236972354: the ledger suite drives /en/inventory; Ledger.tsx:396 commands inventory/adjustments ->
  // handler.go:196 inventoryAdjustRoute -> internal/inventory.
  const al = derivation["--browser-admin-legacy"] ?? [];
  assert.ok(al.some((e) => e.package === "internal/inventory" && e.path.includes("inventory/adjustments")),
    "admin-legacy must record the ledger inventory/adjustments chain (suite discovery, thread 4236972354)");
});

test("round 5: the tracked derivation discovers the bare and suite-named specs (specs>0, drivers>0)", () => {
  const diag = readFileSync(path.join(root, "output/ci-select-backend-browser/tools/r2-diagnostics.txt"), "utf8");
  const statsOf = (mode) => {
    const m = new RegExp(`^${mode.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}: own=(\\d+) specs=(\\d+) urls=(\\d+) ui-flow drivers=(\\d+)`, "m").exec(diag);
    assert.ok(m, `${mode} has no derivation diagnostics line`);
    return { own: Number(m[1]), specs: Number(m[2]), drivers: Number(m[4]) };
  };
  // bareSpecs merge: tracking-backfill ("tracking-backfill.spec.ts" via brfPlaywright) and meta-health-ui
  // ("meta-health-ui.spec.ts" via brfPlaywright) both discovered specs=0 before round 5.
  for (const mode of ["--browser-tracking-backfill", "--browser-meta-health-ui"]) {
    const s = statsOf(mode);
    assert.ok(s.specs > 0 && s.drivers > 0, `${mode} still discovers no spec/driver: ${JSON.stringify(s)} (thread 4236972349)`);
  }
  // suite discovery: admin-legacy's harness names NO spec file — only the suites "ledger" (helper argument +
  // unquoted playwright.config.ts key), "identity-mock" and "entry-mock" (adminMockSuite forwarder).
  const al = statsOf("--browser-admin-legacy");
  assert.ok(al.specs > 0 && al.drivers > 0, `--browser-admin-legacy still discovers no spec/driver: ${JSON.stringify(al)} (thread 4236972354)`);
});

test("round 5 sweep: every real Go browser mode discovers a spec, or is explicitly explained (SPECS-ZERO allowlist)", () => {
  const diag = readFileSync(path.join(root, "output/ci-select-backend-browser/tools/r2-diagnostics.txt"), "utf8");
  const unexplained = diag.split("\n").filter((l) => /^SPECS-ZERO(?!-OK)/.test(l));
  assert.deepEqual(unexplained, [],
    "a Go-booting browser mode with specs=0 is a discovery failure: fix the derivation with evidence or add the mode to SPECS_ZERO_OK with a written reason (thread 4236972354 sweep)");
  // The single documented specs=0 Go-booting mode: --live-browser-input matches the browser universe by NAME but its
  // arm runs Go-only BRW SQL/executor checks (go test -run '^TestLiveBrowserInputBRW' + go vet), no Playwright spec.
  assert.match(diag, /^SPECS-ZERO-OK --live-browser-input:/m);
});

// CI-SPEED-3 ownership drift is checked by this already-registered CI suite, without workflow/runner edits.
import { existsSync } from "node:fs";
const ownershipGenerator = "scripts/dev/derive-browser-ownership.mjs";
const ownershipJSON = "output/ci-select-backend-browser/browser-ownership.json";

test("CI-SPEED-3: generated browser ownership is current and covers the registry", () => {
  assert.ok(existsSync(path.join(root, ownershipGenerator)), "CI-SPEED-3 requires the ownership generator");
  assert.ok(existsSync(path.join(root, ownershipJSON)), "CI-SPEED-3 requires its committed derivation beside lc_covers");
  execFileSync(process.execPath, [ownershipGenerator, "--check"], { cwd: root, stdio: ["ignore", "pipe", "pipe"] });
  const data = JSON.parse(readFileSync(path.join(root, ownershipJSON), "utf8"));
  assert.equal(data.version, 1);
  assert.ok(Object.keys(data.modes).length > 20);
  for (const [mode, entry] of Object.entries(data.modes)) {
    for (const field of ["specs", "inputs", "apps", "unresolved", "evidence"]) assert.ok(Array.isArray(entry[field]), `${mode}.${field}`);
    assert.ok(entry.evidence.length > 0, `${mode}: ownership must cite registry/spec/server evidence`);
    for (const spec of entry.specs) assert.ok(entry.inputs.includes(spec), `${mode}: ${spec} omitted from import closure`);
  }
});

test("CI-SPEED-3: drift checker rejects a corrupted ownership artifact on its own assertion", (t) => {
  assert.ok(existsSync(path.join(root, ownershipGenerator)), "CI-SPEED-3 requires the ownership generator");
  const dir = mkdtempSync(path.join(root, "output/ci-speed-3-drift-"));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const output = path.join(dir, "ownership.json");
  const run = (...args) => execFileSync(process.execPath, [ownershipGenerator, "--output", output, ...args], { cwd: root, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
  run();
  const original = readFileSync(output, "utf8"), data = JSON.parse(original);
  const mode = Object.keys(data.modes)[0];
  delete data.modes[mode];
  writeFileSync(output, JSON.stringify(data));
  assert.throws(() => run("--check"), (error) => {
    assert.equal(error.status, 1, "drift must fail without crashing or invoking a browser");
    assert.match(String(error.stderr), /ownership.*drift|drift.*ownership/i);
    return true;
  });
  writeFileSync(output, original);
  run("--check");
});
