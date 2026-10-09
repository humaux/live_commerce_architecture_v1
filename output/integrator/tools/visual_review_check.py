#!/usr/bin/env python3
# Purpose: refuse an incomplete visual-review report: every captured page in the merged index needs exactly one
#   "## <app> <page-id>: PASS|FIX" section, and the first line must be a VERDICT (PR #23 review).
# Depends on: the merged visual-shots/index.json written by visual_review.sh ({"shards":[{"shard","index"}]}), each index as produced by
#   tests/ui/visual-audit.mjs: "shots" = page objects {app, id, ...}, "captured"/"expected" = counts, "missing" = list.
# Used by: output/integrator/tools/visual_review.sh; tests/admin/browser-evidence.test.mjs.
import json, re, sys
index, findings = json.load(open(sys.argv[1])), open(sys.argv[2], encoding="utf-8").read()
idx = [s.get("index") or {} for s in index.get("shards", [])]
if any(i.get("missing") for i in idx): sys.exit("a shard reports missing shots; the corpus is incomplete")
pages = {(c["app"], c["id"]) for i in idx for c in i.get("shots", [])}
if not pages: sys.exit("no captured pages in the index")
if not re.match(r"VERDICT: (PASS|FIX)\b", findings): sys.exit("first line is not a VERDICT")
seen = {}
for app, pid in re.findall(r"^## (\S+) (\S+): (?:PASS|FIX)\b", findings, re.M):
    seen[(app, pid)] = seen.get((app, pid), 0) + 1
missing = sorted(pages - set(seen)); dup = sorted(k for k, n in seen.items() if n > 1)
if missing or dup:
    sys.exit(f"incomplete report: missing {len(missing)} page(s) {missing[:5]}, duplicated {dup[:5]}")
print(f"complete: {len(pages)} pages, one section each")
