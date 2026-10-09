#!/bin/bash
# Purpose: review the complete visual-lint screenshot corpus from a CI artifact run.
# Depends on: gh, unzip, bash/find, Git worktrees and the existing external-agent runner.
# Used by: integrator visual acceptance; screenshots are synthetic fixtures, never credentials.
# visual_review.sh <gates-run-id> — visual acceptance of every page by Kimi K2.8 (vision) over the G-UI9 screenshot corpus of one CI run (integrator, 2026-10-08).
# Owner 2026-10-08: "所有的页面需要做真实点击测试、视觉的验收". Real clicks = G-UI8 click sweep (CI, 0 known defects); layout rules = G-UI9 visual lint (CI);
# this script adds the human-eye part the lint cannot judge (hierarchy, consistency across pages, text, mobile, empty states), rubric in visual-review-prompt.md.
# Steps: download the run's gate-browser-visual-lint_* artifacts -> detached worktree at TRUSTED trunk (never the run's PR head:
# screenshots are data, and PR-controlled scripts must never run beside the provider key) -> copy shots and ONE merged index.json
# into it -> scripts/agents/ext-agent.sh READONLY=1 (no Bash; writes only output/ext-agents/) with MODEL=kimi-for-coding (K2.8, image input).
# Exit: 0 only with a findings file that starts with a VERDICT line; otherwise NOT_RUN is written and the exit is 4 (PR #23 review).
# Needs: gh, the Kimi subscription key file read by ext-agent.sh. Never sends secrets: screenshots show seeded MOCK data only.
set -euo pipefail
RUN=${1:?usage: visual_review.sh <gates-run-id>}
ROOT=/Volumes/data/live_commerce_architecture_v1; R=humaux/live_commerce_architecture_v1
OUT=$ROOT/output/visual-review/$RUN; rm -rf "$OUT"; mkdir -p "$OUT/dl"
SHA=$(gh run view "$RUN" -R $R --json headSha -q .headSha)   # recorded only; never checked out
git -C $ROOT fetch -q origin r3/integration; TRUST=$(git -C $ROOT rev-parse FETCH_HEAD)
# Complete corpus only (PR #23 review): every visual-lint shard job and the aggregate verdict succeeded, one artifact per shard.
jobs=$(gh api "repos/$R/actions/runs/$RUN/jobs?per_page=100" --paginate -q '.jobs[]|"\(.name)\t\(.conclusion)"')
shards=$(printf '%s\n' "$jobs" | grep -c '^gate (--browser-visual-lint' || true)
bad=$(printf '%s\n' "$jobs" | grep -E '^gate \(--browser-visual-lint|^aggregate\b' | grep -vc $'\tsuccess$' || true)
arts=$(gh api "repos/$R/actions/runs/$RUN/artifacts?per_page=100" -q '[.artifacts[]|select(.name|test("visual-lint"))]|length')
if [ "$shards" -eq 0 ] || [ "$bad" -ne 0 ] || [ "$arts" -ne "$shards" ] || ! printf '%s\n' "$jobs" | grep -q $'^aggregate\tsuccess$'; then
  printf 'NOT_RUN: incomplete visual corpus in run %s (shard jobs=%s, not successful incl. aggregate=%s, artifacts=%s)\n' "$RUN" "$shards" "$bad" "$arts" | tee "$OUT/findings.md"
  exit 4
fi
for id in $(gh api "repos/$R/actions/runs/$RUN/artifacts?per_page=100" -q '.artifacts[]|select(.name|test("visual-lint"))|.id'); do
  gh api "repos/$R/actions/artifacts/$id/zip" > "$OUT/dl/$id.zip" && unzip -qo "$OUT/dl/$id.zip" -d "$OUT/dl/$id"
done
WT=$ROOT/.worktrees/visual-review-$RUN
# Always a fresh worktree: a rerun must never accept a previous run's shots or verdict (PR #23 review).
[ -d "$WT" ] && git -C $ROOT worktree remove --force "$WT"
git -C $ROOT worktree add -q --detach "$WT" "$TRUST"
mkdir -p "$WT/visual-shots"
# Download archives may preserve either output/playwright or just its run children.
# Discover shots directly so an interrupted run without index.json is still reviewable.
while IFS= read -r -d '' shots; do
  d=$(dirname "$shots")
  [ -d "$d/shots" ] && cp -R "$d/shots/." "$WT/visual-shots/"
  [ -f "$d/lint.md" ] && cat "$d/lint.md" >> "$WT/visual-shots/lint.md"
done < <(find "$OUT/dl" -type d -path '*/ui-visual-audit/*/shots' -print0)
# One index.json for the reviewer: every shard's index, keyed by its artifact-relative path (unique even when shards finish in the same second).
python3 - "$OUT/dl" "$WT/visual-shots/index.json" <<'PY'
import json, pathlib, sys
dl, dst = pathlib.Path(sys.argv[1]), sys.argv[2]
shards = [{"shard": str(p.parent.relative_to(dl)), "index": json.loads(p.read_text())}
          for p in sorted(dl.glob("**/ui-visual-audit/*/index.json"))]
json.dump({"shards": shards}, open(dst, "w"), ensure_ascii=False, indent=1)
PY
n=$(find "$WT/visual-shots" -name '*.png' | wc -l | tr -d ' ')
[ "$n" -gt 0 ] || { echo "no shots in run $RUN (was it before ci(visual) 338ceb24, or did visual-lint not run?)"; exit 3; }
echo "run $RUN @ ${SHA:0:8} (reviewed in trunk ${TRUST:0:8}): $n shots -> K2.8 review"
cd $ROOT/.worktrees/r3-integration
rc=0; READONLY=1 MODEL=kimi-for-coding PROVIDER=kimi bash scripts/agents/ext-agent.sh "$WT" $ROOT/output/integrator/tools/visual-review-prompt.md "$OUT" high > "$OUT/run.log" 2>&1 || rc=$?
F="$WT/output/ext-agents/visual-review/findings.md"
if [ "$rc" -eq 0 ] && [ -f "$F" ] && head -1 "$F" | grep -qE '^VERDICT: (PASS|FIX)'; then
  cp "$F" "$OUT/findings.md"; head -1 "$OUT/findings.md"; grep -c '^- \[P1\]' "$OUT/findings.md" | sed 's/^/P1 count: /' || true
else
  printf 'NOT_RUN: no reviewer verdict (ext-agent rc=%s, findings file %s); see %s\n' "$rc" "$([ -f "$F" ] && echo 'without a VERDICT line' || echo missing)" "$OUT/run.log" | tee "$OUT/findings.md"
  exit 4
fi
