#!/usr/bin/env node
// Purpose: the deterministic gate set of a pull request into r3/integration, computed from the PR's changed paths (owner decision: the repo fixes the required checks, the merging
//   agent never picks modes). Always `foundation-shards`; any path outside the backend-only set adds every CI-runnable `--browser-*` mode of the test-local.sh usage line.
//   Also says whether deploy-smoke must run (`deploy/` or `scripts/deploy*` changed). Conservative on purpose: an unknown path counts as UI, never as skipped.
// Depends on: scripts/dev/test-local.sh (usage line = the single mode list, same derivation as scripts/dev/release-gate.sh), git (`git diff --name-only <base>...<head>`); env GITHUB_OUTPUT (modes, deploy).
// Used by: .github/workflows/gates.yml (plan job, pull_request only; the output feeds scripts/dev/ci-plan.mjs), tests/ci/pr-modes.test.mjs.
// Invariants: foundation-shards is ALWAYS present (so the one `required` check exists for docs-only PRs too); a mode is only ever dropped through EXCLUDED_MODES below.
//
// Backend-only set (changes here alone cannot alter a browser): internal/ cmd/ migrations/ contracts/ docs/ deploy/ output/ go.mod go.sum, tests/foundation/ except
//   tests/foundation/browser_*, tests/deploy/ and tests/ci/ (node tests of shell scripts / this planner; scripts/ itself stays UI), and *.md anywhere. Everything else (apps/ packages/ tests/admin/ tests/e2e/ tests/ui/ scripts/ .github/ playwright.config.ts, package files...) is a UI path.
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

/** Every CI-runnable browser mode, parsed from test-local.sh's usage line with release-gate.sh's own pattern (names containing "browser", plus --browser-e2e) minus EXCLUDED_MODES. */
export function browserModes(testLocalSource) {
  const m = /Usage: bash scripts\/dev\/test-local\.sh \[(.*)\]\\n'/.exec(testLocalSource);
  if (!m) throw new Error("cannot parse the usage line of scripts/dev/test-local.sh");
  const modes = m[1].split("|").filter((x) => /^--(.*browser.*|e2e)$/.test(x) && !(x in EXCLUDED_MODES));
  if (!modes.length) throw new Error("no browser modes in the usage line of scripts/dev/test-local.sh");
  return modes;
}

const BACKEND_PREFIXES = ["internal/", "cmd/", "migrations/", "contracts/", "docs/", "deploy/", "output/", "tests/foundation/", "tests/deploy/", "tests/ci/"];
const isBackendOnly = (p) =>
  p.endsWith(".md") || p === "go.mod" || p === "go.sum" ||
  (BACKEND_PREFIXES.some((x) => p.startsWith(x)) && !p.startsWith("tests/foundation/browser_"));
const isDeploy = (p) => p.startsWith("deploy/") || p.startsWith("scripts/deploy");

/** changed paths -> { modes, deploy }. Pure; `source` is the test-local.sh text (default: the repo's). */
export function planPr(paths, source = readFileSync(path.join(root, "scripts/dev/test-local.sh"), "utf8")) {
  const needsBrowsers = paths.some((p) => !isBackendOnly(p));
  return { modes: needsBrowsers ? ["foundation-shards", ...browserModes(source)] : ["foundation-shards"], deploy: paths.some(isDeploy) };
}

/** CLI: `node scripts/dev/pr-modes.mjs <base> <head>` (diffs base...head) or `--stdin` (one path per line). Prints {modes,deploy}; appends modes=/deploy= to $GITHUB_OUTPUT. */
export function main(argv = process.argv.slice(2), env = process.env) {
  const raw = argv[0] === "--stdin" ? readFileSync(0, "utf8")
    : argv.length === 2 ? execFileSync("git", ["diff", "--name-only", `${argv[0]}...${argv[1]}`], { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 })
    : (() => { throw new Error("usage: pr-modes.mjs <base> <head> | --stdin"); })();
  const paths = raw.split("\n").map((s) => s.trim()).filter(Boolean);
  const r = planPr(paths);
  console.error(`pr-modes: ${paths.length} changed path(s) -> ${r.modes.length} mode(s)${r.deploy ? "; deploy-smoke" : ""}`);
  if (env.GITHUB_OUTPUT) appendFileSync(env.GITHUB_OUTPUT, `modes=${JSON.stringify(r.modes)}\ndeploy=${r.deploy}\n`);
  console.log(JSON.stringify(r));
  return 0;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { process.exit(main()); } catch (e) { console.error(`pr-modes: ${e.message}`); process.exit(1); }
}
