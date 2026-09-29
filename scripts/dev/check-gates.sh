#!/usr/bin/env bash
# check-gates.sh — keep docs/delivery/GATES.md and the test runners honest (unit maintainability).
#  1. every mode in test-local.sh's usage line has a row in GATES.md, and every mode GATES.md names
#     exists (no undocumented gate, no stale row);
#  2. every tests/admin/*.spec.ts|*.test.ts is run by some gate: its file name appears in
#     scripts/dev/test-local.sh or tests/foundation/*.go, or it belongs to a playwright.config.ts suite
#     whose name a tests/foundation/*.go file selects (LC_BROWSER_SUITE).
# Usage: bash scripts/dev/check-gates.sh   (exit 1 on any finding)
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
python3 - <<'PY'
import glob, os, re, sys
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
for path in sorted(glob.glob("tests/admin/*.spec.ts") + glob.glob("tests/admin/*.test.ts")):
    name = os.path.basename(path)
    if name not in sh and name not in go and name not in via_suite:
        bad.append(f"{path} is run by no gate (test-local.sh, tests/foundation, or a selected playwright suite)")
if bad:
    print("\n".join("check-gates: " + b for b in bad), file=sys.stderr)
    sys.exit(1)
print(f"check-gates: ok ({len(modes)} modes, all documented; every tests/admin spec is run)")
PY
