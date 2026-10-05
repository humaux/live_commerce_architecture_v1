#!/usr/bin/env bash
# Purpose: keep the gate registry (docs/delivery/GATES.md) and the test runners in sync, plus repo-wide static ratchets.
# Depends on: git grep, node (registry/architecture tests), scripts/dev/check-headers.sh, scripts/dev/ui-architecture-gate.mjs.
# Used by: CI (.github/workflows/foundation.yml), scripts/dev/release-gate.sh, every unit self-check (AGENT-PREAMBLE §2).
# check-gates.sh — keep docs/delivery/GATES.md and the test runners honest (unit maintainability).
#  1. every mode in test-local.sh's usage line has a row in GATES.md, and every mode GATES.md names
#     exists (no undocumented gate, no stale row);
#  2. every tracked *.spec.*|*.test.* file (git ls-files, whole repo) is run by some gate: its file name
#     appears in scripts/dev/test-local.sh or tests/foundation/*.go, it matches a glob written in
#     scripts/dev/test-node.sh (the Node unit gate that CI runs), or it belongs to a playwright.config.ts
#     suite whose name a tests/foundation/*.go file selects (LC_BROWSER_SUITE);
#  3. CI (.github/workflows/foundation.yml) actually invokes scripts/dev/test-node.sh, so a Node suite is
#     never "covered" by a script no workflow runs.
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
# UI W0 G-UI1 registry/parity and G-UI3/G-UI5 architecture ratchet.
node --test --experimental-strip-types tests/admin/shell-registry.test.ts tests/admin/shell-architecture.test.mjs
node scripts/dev/ui-architecture-gate.mjs
# Syntax first: a merge can leave a gate script that no longer parses (R4: a lost `fi` broke every mode).
for s in scripts/dev/test-local.sh scripts/dev/test-node.sh scripts/dev/test-focused.sh scripts/dev/release-gate.sh; do
  bash -n "$s" || { echo "check-gates: $s does not parse (bash -n)" >&2; exit 1; }
done
# A merge can also duplicate a mode's run branch: only the first `elif` runs, so a later copy is dead code that silently
# keeps stale commands (R4: the webkit..purchase-entry run branches existed three times, catalog-media twice with old text).
dup_modes=$(grep -oE '^elif \[\[ "\$test_mode" == --[a-z0-9-]+ \]\]' scripts/dev/test-local.sh | sort | uniq -d)
if [[ -n "$dup_modes" ]]; then
  echo "check-gates: duplicated run branch in scripts/dev/test-local.sh: $dup_modes" >&2; exit 1
fi
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
usage = re.search(r"Usage: bash scripts/dev/test-local\.sh \[(.*?)\]", sh)
if not usage:
    sys.exit("check-gates: no usage line in test-local.sh")
modes = set(usage.group(1).split("|"))
rows = set(re.findall(r"^\| `(--[a-z0-9-]+)` \|", gates, re.M))
bad += [f"mode {m} is in test-local.sh usage but has no GATES.md row" for m in sorted(modes - rows)]
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
