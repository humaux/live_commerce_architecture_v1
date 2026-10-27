#!/usr/bin/env node
// Purpose: the deterministic gate set of a pull request into r3/integration, computed from the PR's changed paths (owner decision: the repo fixes the required checks, the merging
//   agent never picks modes). Always `foundation-shards`; any UI path adds every CI-runnable `--browser-*` mode of the test-local.sh mode registry; internal/, cmd/ and
//   migrations/ changes select browser modes from the registry's lc_covers data (CI-SELECT, see the block below).
//   Browser-tagged foundation inputs additionally select every real registry branch that compiles that tag (including non-browser-named modes).
//   Canonical matrix producer changes always select the live product-editor guard, from the Go consumer's single source declaration.
//   Deploy runtime, smoke workflow/helpers and deploy-test edits select deploy-smoke. Unknown/missing foundation Go sources conservatively select tagged runners.
//   CLI unions legacy quoted/trimmed path classification; only raw paths are used for source lookup.
// Depends on: mode registry, Go build headers and matrix producer declaration, git base/head/checkout sources and changed paths, the checkout's tracked Go sources (doc reads);
//   env GITHUB_OUTPUT (modes, deploy, docs_only).
// Used by: .github/workflows/gates.yml (plan job: pull_request diffs plus the schedule/workflow_dispatch full-browser-universe runs; the output feeds scripts/dev/ci-plan.mjs), tests/ci/pr-modes.test.mjs, tests/ci/backend-coverage.test.mjs.
// Invariants: foundation-shards is present unless EVERY changed path is test-inert documentation (CI-SPEED, owner 2026-10-27 "优化ci的速度"): docs/**, output/**,
//   contracts/**.md or a root *.md that no untagged Go test (the foundation shards' package set) nor a script such a test names reads — see
//   foundationDocReads() below. Then the plan is EMPTY and gates.yml's `required` reports an explicit docs-only success (plan + contract drift still ran;
//   foundation.yml still runs check-gates/test-node). A mode is only ever dropped through EXCLUDED_MODES below.
//
// Backend-only set (changes here alone cannot alter a browser): contracts/ docs/ deploy/ output/ go.mod go.sum, tests/foundation/ except
//   tests/foundation/browser_* or browser-tagged Go sources, tests/deploy/ and tests/ci/ (node tests of shell scripts / this planner; scripts/ itself stays UI), and *.md anywhere. Everything else (apps/ packages/ tests/admin/ tests/e2e/ tests/ui/ scripts/ .github/ playwright.config.ts, package files...) is a UI path.
// Docs-only (docs/, output/, *.md, contracts/) is a subset of backend-only: it runs foundation-shards and no browser mode — or NOTHING when every path is test-inert (CI-SPEED).
//
// CI-SELECT (owner-approved 2026-10-10; round 2 "Narrow per domain + nightly"): internal/, cmd/ and migrations/ are NOT backend-only — every Go-seeded browser mode runs the
//   real Go API on the real migrated PG, so e.g. an internal/live/stream.go change alters what --browser-live-console exercises (verified root cause of PR #30/#24: both
//   changed internal/ server code and merged without any browser mode; push runs on r3/integration re-verified no trunk browser health either — see the schedule trigger in
//   gates.yml). Round 1 derived lc_covers from the harness import CLOSURE, which gave 48/51 modes all 70 packages (every harness boots the full httpapi.NewHandler), making
//   any backend PR as heavy as a UI PR. Round 2 replaces each mode's covers with the DOMAIN packages it actually EXERCISES, derived evidence-first by
//   output/ci-select-backend-browser/tools/derive-narrow-covers.mjs: (a) path evidence — the API URLs its Playwright specs / node runners / Go harness call, mapped through
//   the admin+storefront BFF pass-through rules and route-file literals onto the real registrations in internal/{httpapi,identityhttp,buyerhttp}, each registration
//   attributed to the service packages its handler statement references; (b) narrow Go fixture evidence — the direct internal imports of the mode's OWN harness files and
//   the consumer-import rule for packages no mode names directly (recorded per mode in output/ci-select-backend-browser/covers-derivation.json). Deliberately NOT the
//   transitive closure below the called handlers: deeper service coupling is the nightly full-matrix's job (the owner's chosen safety net). Rules: a changed
//   internal/<pkg>/** selects declaring modes by package prefix or exact .go file; cmd/**, migrations/**, the SHARED_BACKEND_PACKAGES below, and any package
//   no mode declares select every lc_fixture=pg browser mode (conservative); BACKEND_ONLY_PACKAGES-listed packages select none.
//   check-backend-coverage also requires every file of FILE_CLASSIFIED_PACKAGES to be classified; generic httpapi coverage is forbidden.
//
// Excluded browser modes (EXCLUDED_MODES): modes of release-gate's browser universe that cannot run on a GitHub runner. Currently only `--stripe-browser`: its SP18 step needs a Stripe
//   test-mode key (STRIPE_SECRET_KEY in secrets.env + STRIPE_BROWSER=1 STRIPE_SANDBOX=1), gates.yml has no secrets by design, so it would end NOT_RUN. It stays an integrator-run
//   SANDBOX gate (docs/delivery/GATES.md). `--browser-webkit` is NOT excluded: gates.yml installs Playwright WebKit. Checked against test-local.sh, release-gate.sh, GATES.md 2026-10-07.
import { appendFileSync, readFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");

/** Browser modes that cannot run on GitHub, with the reason. A new entry needs a reason here and a row in docs/delivery/GATES.md. */
export const EXCLUDED_MODES = {
  "--stripe-browser": "SANDBOX: needs a Stripe test-mode key from secrets.env (STRIPE_SECRET_KEY, STRIPE_BROWSER=1, STRIPE_SANDBOX=1); gates.yml carries no secrets, so it would only be NOT_RUN",
};

/** Read the native case registry as data. Never execute a script fetched from a Git revision. */
export function modeEntries(source) {
  const begin = source.indexOf("# BEGIN MODE REGISTRY"), end = source.indexOf("# END MODE REGISTRY");
  if (begin < 0 || end <= begin) throw new Error("missing mode registry");
  const registry = source.slice(begin, end);
  const entries = [...registry.matchAll(/^[ \t]+(--[a-z0-9-]+|foundation)\)[ \t]*\n([\s\S]*?)(?=^[ \t]+(?:--[a-z0-9-]+|foundation|\*)\)|^esac$)/gm)].map((m) => {
    const body = m[2], build = /^    lc_build=(none|admin|storefront|both)$/m.exec(body)?.[1];
    const fixture = /^    lc_fixture=(none|pg)$/m.exec(body)?.[1];
    // CI-SELECT registry data (optional declaration): the internal packages the mode's harness wires. [] when the arm
    // declares lc_covers="" (node-only harness); null when the arm has no lc_covers line (probes, historical entries).
    const coversLine = /^    lc_covers="([^"]*)"$/m.exec(body);
    const prepare = /^    lc_prepare\(\) \{\n([\s\S]*?)^    \}$/m.exec(body)?.[1];
    const run = /^    lc_run\(\) \{\n([\s\S]*?)^    \}$/m.exec(body)?.[1];
    if (!build || !fixture || prepare === undefined || run === undefined) throw new Error(`invalid mode entry ${m[1]}`);
    return { name: m[1], build, fixture, prepare, run, body, covers: coversLine ? coversLine[1].split(/[ \t]+/).filter(Boolean) : null };
  });
  if (!entries.length || new Set(entries.map((entry) => entry.name)).size !== entries.length) throw new Error("empty or duplicate mode registry");
  // Strip command functions before checking every case arm. A legal-but-unsupported
  // alias/label must fail closed, never disappear from the selector's universe.
  const declarations = registry.replace(/^([ \t]+)lc_(?:prepare|run)\(\) \{\n[\s\S]*?^\1\}/gm, "")
    .replace(/^[ \t]+lc_(?:prepare|run)\(\) \{[^\n]*\}[ \t]*$/gm, "");
  const arms = [];
  for (const line of declarations.split("\n").map((line) => line.trim())) {
    if (!line || line.startsWith("#") || ["lc_select_mode() {", 'case "$1" in', "fi", ";;", "esac", "}", "*) return 1 ;;"].includes(line)) continue;
    if (/^lc_(?:build|fixture)=[a-z]+$/.test(line) || /^lc_covers="[A-Za-z0-9_./ -]*"$/.test(line) || /^if \[\[ .* \]\]; then$/.test(line)) continue;
    if (/^lc_browsers=(?:chromium|"chromium webkit")$/.test(line)) continue;
    const arm = /^(--[a-z0-9-]+|foundation)\)$/.exec(line);
    if (!arm) throw new Error("unsupported mode registry declaration");
    arms.push(arm[1]);
  }
  if (arms.length !== entries.length || arms.some((arm, i) => arm !== entries[i].name)) throw new Error("unparsed mode registry arm");
  return entries;
}

/** Mode names from the sole current registry; the legacy format is read only for historical Git refs. */
export function registryModes(source) {
  if (source.includes("# BEGIN MODE REGISTRY")) return modeEntries(source).map((entry) => entry.name);
  const m = /Usage: bash scripts\/dev\/test-local\.sh \[(.*)\]\\n'/.exec(source);
  if (!m) throw new Error("cannot parse the mode registry or historical usage line");
  return m[1].split("|");
}
/** Every CI-runnable browser mode, using release-gate's pattern and the existing secret-dependent exclusions. */
export function browserModes(testLocalSource) {
  const modes = registryModes(testLocalSource).filter((x) => /^--(.*browser.*|e2e)$/.test(x) && !(x in EXCLUDED_MODES));
  if (!modes.length) throw new Error("no browser modes in the usage line of scripts/dev/test-local.sh");
  return modes;
}

// The registry uses if/elif dispatch branches. Read their actual tag/package commands,
// including arrays and continued lines; do not invent per-file/test-name mode mappings.
function taggedFoundationModes(source) {
  if (source.includes("# BEGIN MODE REGISTRY")) {
    const modes = modeEntries(source).filter(({ body }) => {
      const flat = body.replace(/\\\r?\n/g, " ");
      return /-tags(?:["']?\s+|=)["']?[^\s"')]*\bbrowser\b/.test(flat) && flat.includes("./tests/foundation");
    }).map(({ name }) => name).filter((name) => !(name in EXCLUDED_MODES));
    if (!modes.length) throw new Error("no browser-tagged foundation runners in registry");
    return modes;
  }
  const flat = source.replace(/\\\r?\n/g, " ");
  const branches = [...flat.matchAll(/^(?:el)?if[^\n]*\$test_mode[^\n]*\bthen[ \t]*(?:#[^\n]*)?$/gm)];
  const selected = new Set();
  for (let i = 0; i < branches.length; i++) {
    const branch = branches[i], body = flat.slice(branch.index + branch[0].length, branches[i + 1]?.index ?? flat.length);
    if (!/\-tags(?:["']?\s+|=)["']?[^\s"')]*\bbrowser\b/.test(body) || !body.includes("./tests/foundation")) continue;
    for (const match of branch[0].matchAll(/\$test_mode["']?\s*==\s*["']?(--[\w-]+)/g)) selected.add(match[1]);
  }
  const modes = registryModes(source).filter((mode) => selected.has(mode) && !(mode in EXCLUDED_MODES));
  if (!modes.length) throw new Error("cannot derive browser-tagged foundation runners from test-local.sh dispatch registry");
  return modes;
}

function workingSource(file) {
  try { return readFileSync(path.join(root, file), "utf8"); }
  catch (error) { if (error.code === "ENOENT") return null; throw error; }
}
function git(args) {
  return execFileSync("git", args, { cwd: root, encoding: "utf8", maxBuffer: 64 << 20, stdio: ["ignore", "pipe", "pipe"] });
}
function revisionSource(ref, file) {
  try { return git(["show", `${ref}:${file}`]); }
  catch { return null; } // Missing/deleted/unreadable input cannot silently become a known untagged file.
}
// Strict: only a real //go:build line with a `browser` term takes a file out of the untagged (shard) set; a comment mentioning it does not.
function hasBrowserConstraint(source) { return /^\/\/go:build[^\n]*\bbrowser\b/m.test(source.slice(0, source.search(/^package\s/m) >>> 0)); }
function hasBrowserTag(source) {
  // Conservative scan: a package-like line inside a license comment cannot hide a later constraint.
  return /^\s*\/\/(?:go:build|\s*\+build)\b[^\n]*\bbrowser\b/m.test(source);
}

/**
 * Explicit classification list (CI-SELECT): internal packages no CI-runnable browser mode exercises, so a change
 * under them selects no browser mode (the nightly full matrix and foundation-shards still run). Every entry needs a
 * one-line reason, and scripts/dev/check-backend-coverage.mjs fails the gate when an internal package is neither
 * lc_covers-covered, SHARED, nor listed here. Round-2 entries come from the 2026-10-10 narrow derivation
 * (output/ci-select-backend-browser/tools/r2-diagnostics.txt: uncovered-package importer listings) — a new entry must
 * come from that derivation's evidence, not from a guess. Round 1's list was empty because the import CLOSURE made
 * every package "wired"; round 2 asks what the specs/harnesses actually EXERCISE.
 * Round 5 (PR #39 thread 4236971117) REMOVED the round-2 ecpayroute entry: its "non-browser tests only" premise was
 * false — TestBrowserTaiwanCvs calls tcvEnv.startDispatcher (tests/foundation/taiwan_cvs_env_test.go:636-644), which
 * wires the real ecpayroute.Routes into the claims dispatcher, and --browser-click-sweep reuses the same env. The
 * derivation now records that wiring as go-fixture-import evidence, and the CVS modes' lc_covers name the package.
 */
export const BACKEND_ONLY_PACKAGES = {
  "internal/tlsask": "TLS-ask listener middleware wired only by cmd/api main (cmd/api/tlsask.go); browser harnesses serve plain HTTP in-process and never construct it — a cmd/api change still selects all Go-booting browser modes via the cmd/** rule",
  "internal/retention": "retention sweeps run only in cmd/claims-worker and cmd/retention-admin; no browser harness starts a worker, and browser fixtures are always newer than any retention horizon",
};

/**
 * Shared package/file declarations (CI-SELECT task rule): a change under a package or to an exact file selects every Go-booting browser
 * mode, conservatively — they sit on the boot path or in the request grammar of every harness, so narrowing them per
 * domain would under-select on refactors. The Go-booting set is the 47 lc_fixture=pg modes plus
 * --browser-tracking-backfill (lc_fixture=none, but it boots the real Go API against the real PG through
 * scripts/dev/test-focused.sh); the three node-only modes boot no Go and stay out. Round-2 membership is decided from
 * the derivation's per-package mode counts (r2-diagnostics.txt): each entry below is exercised by 30+ of the 48
 * Go-seeded modes or is constructed at API boot. They are subtracted from the lc_covers lines (a covers entry would be
 * redundant with this rule).
 */
export const SHARED_BACKEND_PACKAGES = {
  "internal/orderattempt": "shared C1/C2 money-path executor for inline and claims-worker lanes; native cancellation, expiry and erasure share its SQL authority. Deliberately retain every Go-seeded browser mode during staged OA-1/OA-3 cutover rather than infer safety from the currently disabled public admission path",
  "internal/platform": "runtime configuration/service container every browser harness and cmd binary constructs (evidence: 45/48 Go-seeded modes)",
  "internal/httpapi/handler.go": "mux/service construction, shared request/scope/body handling and response/error mapping used across all route families; also registers the catalog product-write routes (handler.go:148-172), so even a catalog-scoped edit selects every Go-booting mode — deliberate over-selection (K3 round-4 review P2; safe direction, a future file split could narrow it)",
  "internal/httpapi/handler_test.go": "tests the shared handler composition and transport boundary, not one domain route",
  "internal/httpapi/claims.go": "claimsBody/claimsClassify/canonicalBearer are shared helpers called by unrelated ads/billing/customers/CVS/payment routes",
  "internal/httpapi/claims_test.go": "tests the shared claims transport helpers as well as claim routes",
  "internal/httpapi/studio.go": "studioRoute/studioDecodeRaw are shared by orders/shipments/CVS/inbox, beyond Studio routes",
  "internal/httpapi/settings.go": "bearerToken and settings transport plumbing serve almost every domain route family",
  "internal/httpapi/settings_test.go": "tests the shared bearer/settings transport plumbing",
  "internal/httpapi/domain_error_codes_test.go": "pins error contracts across multiple domain adapters",
  "internal/httpapi/purchase_entry_test.go": "tests purchase-entry routes implemented by the shared handler.go",
  "internal/command": "the command bus every service dispatches through (43/48)",
  "internal/httperror": "response/error grammar every handler of all three HTTP surfaces answers through (44/48)",
  "internal/pagination": "shared page/request grammar of every paginated admin and buyer list route (35/48)",
  "internal/identity": "auth/session/identity service: every admin browser mode completes login and session checks through it (34/48)",
  "internal/identityhttp": "identity HTTP surface (login/session/staff/password) mounted at every harness boot (34/48)",
  "internal/oidclogin": "OIDC login flow every authenticated admin browser mode walks through (31/48)",
  "internal/integrations/psp/stripe": "Stripe PSP adapter the internal/platform runtime constructs at API boot (internal/platform/stripe_runtime.go) — boot path of every harness",
};

/** Packages whose files need explicit classification; an ancestor declaration cannot classify a new route file. */
export const FILE_CLASSIFIED_PACKAGES = ["internal/httpapi"];

/** Exact match for file declarations; ancestor-or-self match for package declarations. Used by planner and gate. */
export function backendPathMatches(file, declaration) {
  return declaration.endsWith(".go") ? file === declaration : file === declaration || file.startsWith(declaration + "/");
}

/**
 * Browser modes selected by backend paths alone (internal/, cmd/, migrations/) — CI-SELECT. Selection is registry DATA,
 * resolved most-specific first: cmd/, migrations/ and SHARED_BACKEND_PACKAGES prefixes select every Go-booting browser
 * mode (lc_fixture=pg plus any mode with non-empty lc_covers — the gate forces a Go-running mode to declare covers, so
 * this is exactly the set that boots the real Go API, including --browser-tracking-backfill which uses real PG without
 * the shared fixture script); BACKEND_ONLY_PACKAGES select none EVEN under a covered ancestor prefix (an explicit
 * per-package classification beats a general one — the ecpay ancestor covers ecpayroute; round 5 removed the
 * ecpayroute entry itself because the CVS gates boot the real dispatcher, see BACKEND_ONLY_PACKAGES); then lc_covers
 * package-prefix or exact-file match selects the declaring browser-universe modes (uniform guard, round 4: a
 * non-universe mode's covers data can never answer a backend selection); anything else falls back to every Go-booting mode until
 * check-backend-coverage classifies it. check-backend-coverage fails contradictory data (a covers entry inside
 * BACKEND_ONLY, a package both SHARED and BACKEND_ONLY), so this precedence can never silently hide a classification.
 * Historical sources without the native registry carry no lc_covers data, so they fall back to the whole browser
 * universe rather than silently selecting nothing.
 */
export function backendBrowserModes(paths, source, backendOnly = BACKEND_ONLY_PACKAGES) {
  const hits = paths.filter((p) => p.startsWith("internal/") || p.startsWith("cmd/") || p.startsWith("migrations/"));
  if (!hits.length) return [];
  if (!source.includes("# BEGIN MODE REGISTRY")) return browserModes(source);
  const entries = modeEntries(source);
  const universe = new Set(browserModes(source));
  const goBoot = entries.filter((e) => universe.has(e.name) && (e.fixture === "pg" || (e.covers ?? []).length > 0)).map((e) => e.name);
  const selected = new Set();
  for (const p of hits) {
    if (p.startsWith("cmd/") || p.startsWith("migrations/") || Object.keys(SHARED_BACKEND_PACKAGES).some((s) => backendPathMatches(p, s))) {
      for (const m of goBoot) selected.add(m);
      continue;
    }
    if (Object.keys(backendOnly).some((b) => backendPathMatches(p, b))) continue;
    // Round 4 (K3 P2): the browser-universe guard applies uniformly to EVERY lc_covers entry, package-style and
    // file-style alike. A non-browser-universe mode's declaration is not browser acceptance; with no universe member
    // covering the path, selection falls through to the conservative goBoot fallback below (fail-closed), never to a
    // silently thinner set.
    const covering = entries.filter((e) => universe.has(e.name) && (e.covers ?? []).some((c) => backendPathMatches(p, c))).map((e) => e.name);
    if (covering.length) { for (const m of covering) selected.add(m); continue; }
    for (const m of goBoot) selected.add(m); // undeclared package: conservative until check-backend-coverage classifies it
  }
  return [...selected].filter((m) => !(m in EXCLUDED_MODES));
}

const BACKEND_PREFIXES = ["internal/", "cmd/", "migrations/", "contracts/", "docs/", "deploy/", "output/", "tests/foundation/", "tests/deploy/", "tests/ci/"];
const isBackendOnly = (p) =>
  p.endsWith(".md") || p === "go.mod" || p === "go.sum" ||
  (BACKEND_PREFIXES.some((x) => p.startsWith(x)) && !p.startsWith("tests/foundation/browser_"));
const isDeploy = (p) => p.startsWith("deploy/") || p.startsWith("scripts/deploy") || p.startsWith("tests/deploy/") ||
  p.startsWith(".github/scripts/") || /^\.github\/workflows\/deploy-smoke\.ya?ml$/.test(p);

/**
 * CI-SPEED test-inert documentation. A changed path can skip the foundation shards only when it is documentation (docs/**, output/**, contracts/**.md,
 * a root *.md; never Go, go.mod/go.sum/go.work — output/go.mod fences output/ out of ./...) AND no reader of it exists in what the shards execute: every
 * tracked Go file without a `browser` build constraint (shard:unit = ./... minus tests/foundation, shard:gNN = untagged tests/foundation) plus every
 * tracked script such a file names (one level: check_packet.py, deploy-prep-r3.test.mjs, ...). A reader is a path-shaped string literal
 * (docs|output|contracts/..., {x}/${x}/%s/* = one-segment wildcard, trailing "/" or a concatenated prefix = the whole subtree), a literal
 * filepath.Join chain ("docs", "runbooks", ...) or a root *.md basename. Full-line comments are ignored; anything else counts (over-reading = safe).
 * ponytail: lexical scan, not a call graph. A test that builds a doc path from a lone bare "docs"/"contracts" literal plus runtime data is not
 * seen (lone "output"/"docs" words are too common as field names to treat as reads); the push run on r3/integration still runs every shard after merge.
 * Upgrade path: test-level go list -deps + file-access tracing if such a reader appears.
 */
const IGNORED_READS = {
  // file -> reads that are not foundation-shard reads of the real repository, each with a reason.
  "scripts/dev/contractdrift/git.go": { "contracts/": "classifies changed paths by prefix (HasPrefix); the real contracts/ read is the plan job's contract-drift step, which runs on every PR regardless of the matrix" },
};
const isDocCandidate = (p) => /* RED fault injection (CI-SPEED): Go counted as documentation */ !/(^|\/)go\.(mod|sum|work)$/.test(p) &&
  (p.startsWith("docs/") || p.startsWith("output/") || (p.startsWith("contracts/") && p.endsWith(".md")) || (!p.includes("/") && p.endsWith(".md")));
const SCRIPT = /\.(sh|bash|py|mjs|cjs|js|ts)$/;
function literals(file, text) {
  const code = text.split("\n").filter((l) => !(/\.(go|m?js|cjs|ts)$/.test(file) ? /^\s*\/\//.test(l) : /^\s*#/.test(l))).join("\n");
  const out = [...code.matchAll(/"((?:[^"\\\n]|\\.)*)"|`([^`]*)`|'((?:[^'\\\n]|\\.)*)'/g)].map((m) => m[1] ?? m[2] ?? m[3]);
  for (const m of code.matchAll(/"[\w.-]+"(?:\s*,\s*"[\w.-]+")+/g)) out.push([...m[0].matchAll(/"([^"]+)"/g)].map((x) => x[1]).join("/"));
  return out;
}
/** Reads (exact path, subtree "x/", or glob with *) of documentation by what the foundation shards execute, scanned from the given tree. */
export function foundationDocReads(dir = root) {
  const files = execFileSync("git", ["ls-files", "-z"], { cwd: dir, encoding: "utf8", maxBuffer: 256 << 20 }).split("\0").filter(Boolean);
  const tracked = new Set(files), read = (f) => { try { return readFileSync(path.join(dir, f), "utf8"); } catch (e) { if (e.code === "ENOENT" || e.code === "EISDIR") return ""; throw e; } };
  const rootMd = files.filter((f) => !f.includes("/") && f.endsWith(".md")).map((f) => f.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
  const mdRe = rootMd.length ? new RegExp(`(?<![\\w./-])(${rootMd.join("|")})(?![\\w.-])`, "gu") : null;
  const goFiles = files.filter((f) => f.endsWith(".go") && !hasBrowserConstraint(read(f)));
  const scripts = new Set();
  for (const f of goFiles) for (const s of literals(f, read(f))) { const p = s.replace(/^(?:\.\.?\/)+/, ""); if (tracked.has(p) && SCRIPT.test(p)) scripts.add(p); }
  const reads = new Set();
  for (const f of [...goFiles, ...scripts]) for (const s of literals(f, read(f))) {
    for (const m of s.matchAll(/(?<![\w./-])(?:\.\.?\/)*((?:docs|output|contracts)\/[^\s"'`()]*)/g)) {
      const r = m[1].replace(/\$\{[^}]*\}|\{[^}]*\}|%[sdvq]/g, "*").replace(/\\n.*$/s, "");
      if (!IGNORED_READS[f]?.[r]) reads.add(r);
    }
    if (mdRe) for (const m of s.matchAll(mdRe)) reads.add(m[1]);
  }
  return [...reads].sort();
}
const readMatches = (read, p) => read.includes("*")
  ? new RegExp(`^${read.split("*").map((x) => x.replace(/[.+?^${}()|[\]\\]/g, "\\$&")).join("[^/]*")}(/|$)`).test(p)
  : read.endsWith("/") ? p.startsWith(read) : p === read || p.startsWith(`${read}/`);
/** Test-inert documentation: a doc candidate no foundation-shard reader names. */
export function isInertDoc(p, reads) { return isDocCandidate(p) && !reads.some((r) => readMatches(r, p)); }
let cachedReads;
const defaultInert = (p) => isInertDoc(p, cachedReads ??= foundationDocReads());

/** Changed paths and source observations -> deterministic required gates; source lookup defaults to the worktree. */
export function planPr(paths, source = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8"), readSources = (file) => [workingSource(file)], inert = defaultInert) {
  // CI-SPEED: nothing a foundation shard or browser can observe changed -> empty plan (gates.yml reports docs-only success explicitly).
  if (paths.length && paths.every((p) => inert(p))) return { modes: [], deploy: paths.some(isDeploy), docsOnly: true };
  const tagged = paths.some((file) => {
    if (!file.startsWith("tests/foundation/") || !file.endsWith(".go")) return false;
    const sources = readSources(file).filter((text) => text !== null);
    return !sources.length || sources.some(hasBrowserTag);
  });
  const needsBrowsers = tagged || paths.some((p) => !isBackendOnly(p));
  // Historical refs may predate the declaration. Union all available base/head/
  // checkout observations, so additions, removals and merge-ref drift stay guarded.
  const producers = readSources("tests/foundation/catalog_matrix_ledger_test.go").flatMap(matrixEvidenceSources);
  const matrix = paths.some((file) => producers.includes(file));
  // Backend-driven selections are always a subset of the full browser set, so UI-path plans keep their exact order.
  return { modes: [...new Set(["foundation-shards", ...(needsBrowsers ? browserModes(source) : []), ...backendBrowserModes(paths, source), ...(tagged ? taggedFoundationModes(source) : []), ...(matrix ? ["--browser-product-editor"] : [])])], deploy: paths.some(isDeploy), docsOnly: false };
}

// Parse only the named literal Go data table, never executable code or unrelated
// quoted strings. A malformed present declaration fails closed, not to an empty set.
function matrixEvidenceSources(text) {
  if (text === null || !/^[ \t]*var[ \t]+productEditorMatrixEvidenceSources\b/m.test(text)) return [];
  const body = /^[ \t]*var[ \t]+productEditorMatrixEvidenceSources[ \t]*=[ \t]*\[\]string\{([^}]+)\}/m.exec(text)?.[1];
  const paths = body?.trim().split(/\r?\n/).map((line) => /^\s*"([A-Za-z0-9_./-]+)",\s*$/.exec(line)?.[1]);
  if (!paths?.length || paths.some((file) => !file || file.startsWith("/") || file.split("/").includes("..")) || new Set(paths).size !== paths.length)
    throw new Error("invalid matrix evidence source declaration");
  return paths;
}

/** CLI: `<base> [head=HEAD]` compares merge-base/head; `--stdin` reads worktree plus HEAD for uncommitted tag removals. */
export function main(argv = process.argv.slice(2), env = process.env) {
  let paths, readSources, source, legacyDisplay;
  if (argv.length === 1 && argv[0] === "--stdin") {
    legacyDisplay = readFileSync(0, "utf8");
    paths = legacyDisplay.split(/\r?\n/).filter(Boolean);
    readSources = (file) => [workingSource(file), revisionSource("HEAD", file)];
  } else if (argv.length >= 1 && argv.length <= 2 && !argv[0].startsWith("--")) {
    const head = git(["rev-parse", "--verify", `${argv[1] ?? "HEAD"}^{commit}`]).trim();
    const base = git(["merge-base", argv[0], head]).trim();
    source = revisionSource(head, "scripts/dev/test-local.sh");
    if (source === null) throw new Error("cannot read the requested head's test-local.sh registry");
    // Both sides of a rename matter; -z preserves tabs/newlines instead of Git-quoting the path.
    paths = git(["diff", "--no-renames", "--name-only", "-z", base, head]).split("\0").filter(Boolean);
    legacyDisplay = git(["diff", "--name-only", `${base}...${head}`]);
    // Plus the checkout: in CI it is GitHub's merge, which may carry a build tag the base added after the fork (PR #22 review).
    readSources = (file) => [revisionSource(base, file), revisionSource(head, file), workingSource(file)];
  } else throw new Error("usage: pr-modes.mjs <base> [head] | --stdin");
  const r = planPr(paths, source, readSources);
  // CI checks out GitHub's merge of head into base, so the checkout's registry also has modes the base added after the
  // fork. Union them (never fewer): a head-only registry would skip those modes for tagged-source changes (PR #22 review).
  if (source !== undefined) {
    const merged = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8");
    if (merged !== source) r.modes = [...new Set([...r.modes, ...planPr(paths, merged, readSources).modes])];
  }
  // Compatibility is with the old CLI, not merely planPr(rawPaths): Git's
  // quoted Unicode/tab names used to count as UI, and trim() could select smoke.
  // Never feed these display spellings into source lookup. The legacy CLI used
  // the checkout's registry even when an explicit head selected another one.
  const legacyPaths = legacyDisplay.split("\n").map((p) => p.trim()).filter(Boolean);
  if (legacyPaths.some((p) => !isBackendOnly(p)))
    r.modes = [...new Set([...r.modes, ...browserModes(readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8"))])];
  r.deploy ||= legacyPaths.some(isDeploy);
  r.docsOnly = r.modes.length === 0;
  console.error(`pr-modes: ${paths.length} changed path(s) -> ${r.modes.length} mode(s)${r.deploy ? "; deploy-smoke" : ""}${r.docsOnly ? "; docs-only (test-inert paths)" : ""}`);
  if (env.GITHUB_OUTPUT) appendFileSync(env.GITHUB_OUTPUT, `modes=${JSON.stringify(r.modes)}\ndeploy=${r.deploy}\ndocs_only=${r.docsOnly}\n`);
  console.log(JSON.stringify(r));
  return 0;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { process.exit(main()); } catch (e) { console.error(`pr-modes: ${e.message}`); process.exit(1); }
}
