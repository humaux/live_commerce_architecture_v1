#!/usr/bin/env bash
# Purpose: keep the gate registry (docs/delivery/GATES.md) and the test runners in sync, plus repo-wide static ratchets.
# Depends on: git grep, Go stdlib contractdrift, node (registry/architecture tests), header and UI architecture ratchets.
# Used by: CI (.github/workflows/foundation.yml), scripts/dev/release-gate.sh, every unit self-check (AGENT-PREAMBLE §2).
# check-gates.sh — keep docs/delivery/GATES.md and the test runners honest (unit maintainability).
#  1. every mode in test-local.sh's registry has a row in GATES.md, and every mode GATES.md names
#     exists (no undocumented gate, no stale row);
#  2. every tracked *.spec.*|*.test.* file (git ls-files, whole repo) is run by some gate: its file name
#     appears in scripts/dev/test-local.sh or tests/foundation/*.go, it matches a glob written in
#     scripts/dev/test-node.sh (the Node unit gate that CI runs), or it belongs to a playwright.config.ts
#     suite whose name a tests/foundation/*.go file selects (LC_BROWSER_SUITE);
#  3. CI (.github/workflows/foundation.yml) actually invokes scripts/dev/test-node.sh, so a Node suite is
#     never "covered" by a script no workflow runs.
#  4. every top-level tests/foundation test belongs to exactly one CI shard group (scripts/dev/shard-plan.mjs --check).
# Usage: bash scripts/dev/check-gates.sh   (exit 1 on any finding)
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
# PS1: changing platform domains must be configuration-only. Include all UI/legal
# source files, never build output, documentation examples, or test fixtures.
# git grep (always present here, unlike rg) over tracked files only: build output and node_modules are never tracked.
if git grep -niI -e 'xgdwm[.]com' -- 'apps/admin/*.ts' 'apps/admin/*.tsx' 'apps/admin/*.js' 'apps/admin/*.jsx' 'apps/admin/*.mjs' \
  'apps/admin/*.md' 'apps/admin/*.mdx' 'apps/admin/*.html' 'apps/admin/*.css' 'apps/storefront/*.ts' 'apps/storefront/*.tsx' \
  'apps/storefront/*.js' 'apps/storefront/*.jsx' 'apps/storefront/*.mjs' 'apps/storefront/*.md' 'apps/storefront/*.mdx' \
  'apps/storefront/*.html' 'apps/storefront/*.css' ':!**/tests/**'; then
  echo 'check-gates: PS1 hard-coded platform domain in UI/legal source' >&2
  exit 1
else
  grep_exit=$?
  # 1 means no match; any other status (bad pathspec, unreadable repo) must fail closed.
  [[ "$grep_exit" == 1 ]] || exit "$grep_exit"
fi
# CI-DRIFT: real source inventories and reviewed baseline; missing base refs/new/touched/stale drift fail closed.
go test ./scripts/dev/contractdrift
go run ./scripts/dev/contractdrift
# Browser artifact writes must stay in ignored run roots; history is input, never a mutable output.
node scripts/dev/check-browser-evidence.mjs
# UI W0 G-UI1 registry/parity and G-UI3/G-UI5 architecture ratchet.
node --test --experimental-strip-types tests/admin/shell-registry.test.ts tests/admin/shell-architecture.test.mjs
node scripts/dev/ui-architecture-gate.mjs
# CI foundation shards (gates.yml `foundation-shards`): every top-level tests/foundation test runs in exactly one balanced group, so a new test can never fall outside
# every shard (a test in no list lands in the catch-all group; a duplicate, or a plan without a catch-all, fails here). Regenerate: node scripts/dev/shard-plan.mjs --write.
node scripts/dev/shard-plan.mjs --check
# Syntax first: a merge can leave a gate script that no longer parses (R4: a lost `fi` broke every mode).
for s in scripts/dev/test-local.sh scripts/dev/test-local-runtime.sh scripts/dev/test-node.sh scripts/dev/test-focused.sh scripts/dev/release-gate.sh; do
  bash -n "$s" || { echo "check-gates: $s does not parse (bash -n)" >&2; exit 1; }
done
# Validate the same registry the selector reads; duplicate cases cannot silently shadow a mode.
node --input-type=module -e 'import {readFileSync} from "node:fs"; import {modeEntries} from "./scripts/dev/pr-modes.mjs"; modeEntries(readFileSync("scripts/dev/test-local.sh","utf8"));'
# CI-SELECT: every internal package dir AND every Go file of split packages (httpapi) must be wired by lc_covers or listed in
# BACKEND_ONLY_PACKAGES; browser-universe modes must carry the lc_covers line. A new/unclassified internal package fails here.
node scripts/dev/check-backend-coverage.mjs
# Worker-authority split (0096): no migration numbered after it may grant to the retired shared commerce_worker role
# (a grant there reaches no worker login; post_river/0019 asserts the same at apply time — this fails earlier, in CI).
for f in $(ls migrations/0*.sql | awk -F/ '$2 > "0096"'); do
  if grep -nE "TO[[:space:]]+commerce_worker([^_a-z]|$)" "$f" >/dev/null; then echo "check-gates: $f grants to commerce_worker (use the split authorities, migrations/0096)" >&2; exit 1; fi
done
python3 - <<'PY'
import fnmatch, glob, os, re, subprocess, sys
bad = []
sh = open("scripts/dev/test-local.sh").read()
gates = open("docs/delivery/GATES.md").read()
names = subprocess.check_output(["bash", "scripts/dev/test-local.sh", "--list"], text=True).splitlines()
if len(names) != len(set(names)):
    sys.exit("check-gates: duplicate mode in registry")
modes = set(names) - {"foundation"}
rows = set(re.findall(r"^\| `(--[a-z0-9-]+)` \|", gates, re.M))
bad += [f"mode {m} is in test-local.sh registry but has no GATES.md row" for m in sorted(modes - rows)]
bad += [f"GATES.md row {m} names a mode test-local.sh does not accept" for m in sorted(rows - modes)]
go = "".join(open(f).read() for f in glob.glob("tests/foundation/*.go"))
config = open("playwright.config.ts").read()
suites = {}  # suite name -> spec files
for name, body in re.findall(r'"?([a-z-]+)"?:\s*\[([^\]]*)\]', config):
    suites[name] = re.findall(r'"([^"]+\.(?:spec|test)\.ts)"', body)
selected = {s for s in suites if f'"{s}"' in go}
via_suite = {f for s in selected for f in suites[s]}
node_gate = open("scripts/dev/test-node.sh").read()
globs = re.findall(r"[\w./*-]+\.(?:test|spec)\.\w+", node_gate)  # literal paths and dir/*.test.ext globs
tracked = subprocess.run(["git", "ls-files", "*.spec.*", "*.test.*"], capture_output=True, text=True, check=True).stdout.split()
for path in sorted(tracked):
    name = os.path.basename(path)
    if not (name in sh or name in go or name in via_suite or any(fnmatch.fnmatch(path, g) for g in globs)):
        bad.append(f"{path} is run by no gate (test-local.sh, tests/foundation, test-node.sh, or a selected playwright suite)")
if "scripts/dev/test-node.sh" not in open(".github/workflows/foundation.yml").read():
    bad.append("foundation.yml does not run scripts/dev/test-node.sh (Node unit suites would be gated by nothing)")
if bad:
    print("\n".join("check-gates: " + b for b in bad), file=sys.stderr)
    sys.exit(1)
print(f"check-gates: ok ({len(modes)} modes, all documented; every tracked test file is run)")
PY
# Documentation ratchet (owner 2026-10-05): files added/changed since the base carry Purpose / Depends on / Used by headers.
bash scripts/dev/check-headers.sh
# Dependency register (PROCESS.md §5): every direct go.mod require has a docs/engineering/dependencies.md row at the same
# version (2026-10-09: PR #19 bumped x/net but the register still said v0.59.0, so reviews kept citing the vulnerable one).
# A submodule may share its parent's row when the row names it, e.g. river (+ `riverdriver/riverpgxv5`, `rivertype`).
python3 - <<'PY'
import json, re, subprocess, sys
# go mod edit -json is Go's own go.mod parser: every require form, comments and the // indirect marker handled structurally.
mods = [(r["Path"], r["Version"]) for r in json.loads(subprocess.run(["go", "mod", "edit", "-json"], check=True,
        capture_output=True, text=True).stdout).get("Require") or [] if not r.get("Indirect")]
if not mods:
    sys.exit("check-gates: parsed no direct requires from go.mod (fail closed)")
rows = [(m.group(1), l) for l in open("docs/engineering/dependencies.md") if (m := re.match(r"\| `([^`]+)`", l))]
bad = []
for mod, ver in mods:
    # Match on the row's first cell only; prose elsewhere in a row may name other modules.
    row = next((r for name, r in rows if name == mod), None) or next(
        (r for name, r in rows if mod.startswith(name + "/") and f"`{mod[len(name)+1:]}`" in r), None)
    cell = row.split("|")[2] if row else ""
    if row is None:
        bad.append(f"{mod} has no row in docs/engineering/dependencies.md")
    elif cell.split()[:1] != [ver]:   # leading version token only: "v0.42.0 (raised from v0.39.0 ...)" must not satisfy v0.39.0
        bad.append(f"{mod} is {ver} in go.mod but the register row says {cell.strip()}")
if bad:
    print("\n".join("check-gates: " + b for b in bad), file=sys.stderr)
    sys.exit(1)
PY
# Go formatting (2026-10-06: an unformatted test file only surfaced as a CRP10 failure deep in the full PG suite).
unformatted="$(gofmt -l cmd internal tests migrations scripts/dev/contractdrift 2>/dev/null || true)"
if [[ -n "$unformatted" ]]; then printf 'check-gates: gofmt needed:\n%s\n' "$unformatted" >&2; exit 1; fi
# Browser-tagged test files (//go:build browser) only compile in --browser-* modes, so a helper name clash there passes every
# PG shard and then breaks every browser gate (trunk a8d029ea: mustJSON redeclared). Type-check both tag sets here.
for tags in "" browser; do
  if ! go vet ${tags:+-tags "$tags"} ./tests/foundation >/dev/null 2>"${TMPDIR:-/tmp}/check-gates-vet.$$"; then
    echo "check-gates: go vet ${tags:+-tags $tags }./tests/foundation failed:" >&2; head -20 "${TMPDIR:-/tmp}/check-gates-vet.$$" >&2; rm -f "${TMPDIR:-/tmp}/check-gates-vet.$$"; exit 1
  fi
done
rm -f "${TMPDIR:-/tmp}/check-gates-vet.$$"
