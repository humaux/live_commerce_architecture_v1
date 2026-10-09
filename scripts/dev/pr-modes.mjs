#!/usr/bin/env node
// Purpose: the deterministic gate set of a pull request into r3/integration, computed from the PR's changed paths (owner decision: the repo fixes the required checks, the merging
//   agent never picks modes). Always `foundation-shards`; any path outside the backend-only set adds every CI-runnable `--browser-*` mode of the test-local.sh mode registry.
//   Browser-tagged foundation inputs additionally select every real registry branch that compiles that tag (including non-browser-named modes).
//   Deploy runtime, smoke workflow/helpers and deploy-test edits select deploy-smoke. Unknown/missing foundation Go sources conservatively select tagged runners.
//   CLI unions legacy quoted/trimmed path classification; only raw paths are used for source lookup.
// Depends on: test-local.sh case registry (historical usage/dispatch fallback), Go build-constraint headers, git merge-base/head sources and NUL-delimited changed paths; env GITHUB_OUTPUT (modes, deploy).
// Used by: .github/workflows/gates.yml (plan job, pull_request only; the output feeds scripts/dev/ci-plan.mjs), tests/ci/pr-modes.test.mjs.
// Invariants: foundation-shards is ALWAYS present (so the one `required` check exists for docs-only PRs too); a mode is only ever dropped through EXCLUDED_MODES below.
//
// Backend-only set (changes here alone cannot alter a browser): internal/ cmd/ migrations/ contracts/ docs/ deploy/ output/ go.mod go.sum, tests/foundation/ except
//   tests/foundation/browser_* or browser-tagged Go sources, tests/deploy/ and tests/ci/ (node tests of shell scripts / this planner; scripts/ itself stays UI), and *.md anywhere. Everything else (apps/ packages/ tests/admin/ tests/e2e/ tests/ui/ scripts/ .github/ playwright.config.ts, package files...) is a UI path.
// Docs-only (docs/, output/, *.md) is a subset of backend-only: it runs foundation-shards (cheap, keeps the single required check) and no browser mode.
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
    const prepare = /^    lc_prepare\(\) \{\n([\s\S]*?)^    \}$/m.exec(body)?.[1];
    const run = /^    lc_run\(\) \{\n([\s\S]*?)^    \}$/m.exec(body)?.[1];
    if (!build || !fixture || prepare === undefined || run === undefined) throw new Error(`invalid mode entry ${m[1]}`);
    return { name: m[1], build, fixture, prepare, run, body };
  });
  if (!entries.length || new Set(entries.map((entry) => entry.name)).size !== entries.length) throw new Error("empty or duplicate mode registry");
  // Strip command functions before checking every case arm. A legal-but-unsupported
  // alias/label must fail closed, never disappear from the selector's universe.
  const declarations = registry.replace(/^([ \t]+)lc_(?:prepare|run)\(\) \{\n[\s\S]*?^\1\}/gm, "")
    .replace(/^[ \t]+lc_(?:prepare|run)\(\) \{[^\n]*\}[ \t]*$/gm, "");
  const arms = [];
  for (const line of declarations.split("\n").map((line) => line.trim())) {
    if (!line || line.startsWith("#") || ["lc_select_mode() {", 'case "$1" in', "fi", ";;", "esac", "}", "*) return 1 ;;"].includes(line)) continue;
    if (/^lc_(?:build|fixture)=[a-z]+$/.test(line) || /^if \[\[ .* \]\]; then$/.test(line)) continue;
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
function hasBrowserTag(source) {
  // Conservative scan: a package-like line inside a license comment cannot hide a later constraint.
  return /^\s*\/\/(?:go:build|\s*\+build)\b[^\n]*\bbrowser\b/m.test(source);
}

const BACKEND_PREFIXES = ["internal/", "cmd/", "migrations/", "contracts/", "docs/", "deploy/", "output/", "tests/foundation/", "tests/deploy/", "tests/ci/"];
const isBackendOnly = (p) =>
  p.endsWith(".md") || p === "go.mod" || p === "go.sum" ||
  (BACKEND_PREFIXES.some((x) => p.startsWith(x)) && !p.startsWith("tests/foundation/browser_"));
const isDeploy = (p) => p.startsWith("deploy/") || p.startsWith("scripts/deploy") || p.startsWith("tests/deploy/") ||
  p.startsWith(".github/scripts/") || /^\.github\/workflows\/deploy-smoke\.ya?ml$/.test(p);

/** Changed paths and source observations -> deterministic required gates; source lookup defaults to the worktree. */
export function planPr(paths, source = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8"), readSources = (file) => [workingSource(file)]) {
  const tagged = paths.some((file) => {
    if (!file.startsWith("tests/foundation/") || !file.endsWith(".go")) return false;
    const sources = readSources(file).filter((text) => text !== null);
    return !sources.length || sources.some(hasBrowserTag);
  });
  const needsBrowsers = tagged || paths.some((p) => !isBackendOnly(p));
  return { modes: [...new Set(["foundation-shards", ...(needsBrowsers ? browserModes(source) : []), ...(tagged ? taggedFoundationModes(source) : [])])], deploy: paths.some(isDeploy) };
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
    readSources = (file) => [revisionSource(base, file), revisionSource(head, file)];
  } else throw new Error("usage: pr-modes.mjs <base> [head] | --stdin");
  const r = planPr(paths, source, readSources);
  // Compatibility is with the old CLI, not merely planPr(rawPaths): Git's
  // quoted Unicode/tab names used to count as UI, and trim() could select smoke.
  // Never feed these display spellings into source lookup. The legacy CLI used
  // the checkout's registry even when an explicit head selected another one.
  const legacyPaths = legacyDisplay.split("\n").map((p) => p.trim()).filter(Boolean);
  if (legacyPaths.some((p) => !isBackendOnly(p)))
    r.modes = [...new Set([...r.modes, ...browserModes(readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8"))])];
  r.deploy ||= legacyPaths.some(isDeploy);
  console.error(`pr-modes: ${paths.length} changed path(s) -> ${r.modes.length} mode(s)${r.deploy ? "; deploy-smoke" : ""}`);
  if (env.GITHUB_OUTPUT) appendFileSync(env.GITHUB_OUTPUT, `modes=${JSON.stringify(r.modes)}\ndeploy=${r.deploy}\n`);
  console.log(JSON.stringify(r));
  return 0;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { process.exit(main()); } catch (e) { console.error(`pr-modes: ${e.message}`); process.exit(1); }
}
