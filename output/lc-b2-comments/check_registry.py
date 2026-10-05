import re

sh = open("scripts/dev/test-local.sh").read()
gates = open("docs/delivery/GATES.md").read()
usage = re.search(r"Usage: bash scripts/dev/test-local\.sh \[(.*?)\]", sh)
assert usage, "no usage line"
modes = set(usage.group(1).split("|"))
rows = set(re.findall(r"^\| `(--[a-z0-9-]+)` \|", gates, re.M))
print("modes:", len(modes))
print("missing rows (in usage, no GATES.md row):", sorted(modes - rows))
print("stale rows (in GATES.md, no usage mode):", sorted(rows - modes))

# dup run branches
import collections
branches = re.findall(r'^elif \[\[ "\$test_mode" == (--[a-z0-9-]+) \]\]', sh, re.M)
dup = [m for m, n in collections.Counter(branches).items() if n > 1]
print("duplicated run branches:", dup)
