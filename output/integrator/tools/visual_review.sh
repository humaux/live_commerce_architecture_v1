#!/bin/bash
# Purpose: review the complete visual-lint screenshot corpus from a CI artifact run.
# Depends on: gh, unzip, bash/find, Git worktrees and the existing external-agent runner.
# Used by: integrator visual acceptance; screenshots are synthetic fixtures, never credentials.
# visual_review.sh <gates-run-id> — visual acceptance of every page by Kimi K2.8 (vision) over the G-UI9 screenshot corpus of one CI run (integrator, 2026-10-08).
# Owner 2026-10-08: "所有的页面需要做真实点击测试、视觉的验收". Real clicks = G-UI8 click sweep (CI, 0 known defects); layout rules = G-UI9 visual lint (CI);
# this script adds the human-eye part the lint cannot judge (hierarchy, consistency across pages, text, mobile, empty states), rubric in visual-review-prompt.md.
# Steps: download the run's gate-browser-visual-lint_* artifacts (shots/ uploaded since ci(visual) 338ceb24) -> detached read-only worktree at the run's head SHA
# -> copy shots into it -> scripts/agents/ext-agent.sh with MODEL=kimi-for-coding (K2.8, image input) -> findings under output/visual-review/<run>/.
# Needs: gh, the Kimi subscription key file read by ext-agent.sh. Never sends secrets: screenshots show seeded MOCK data only.
set -euo pipefail
RUN=${1:?usage: visual_review.sh <gates-run-id>}
ROOT=/Volumes/data/live_commerce_architecture_v1; R=humaux/live_commerce_architecture_v1
OUT=$ROOT/output/visual-review/$RUN; mkdir -p "$OUT/dl"
SHA=$(gh run view "$RUN" -R $R --json headSha -q .headSha)
for id in $(gh api "repos/$R/actions/runs/$RUN/artifacts?per_page=100" -q '.artifacts[]|select(.name|test("visual-lint"))|.id'); do
  gh api "repos/$R/actions/artifacts/$id/zip" > "$OUT/dl/$id.zip" && unzip -qo "$OUT/dl/$id.zip" -d "$OUT/dl/$id"
done
WT=$ROOT/.worktrees/visual-review-$RUN
[ -d "$WT" ] || git -C $ROOT worktree add -q --detach "$WT" "$SHA"
mkdir -p "$WT/visual-shots"
# Download archives may preserve either output/playwright or just its run children.
# Discover shots directly so an interrupted run without index.json is still reviewable.
while IFS= read -r -d '' shots; do
  d=$(dirname "$shots")
  [ -d "$d/shots" ] && cp -R "$d/shots/." "$WT/visual-shots/"
  [ -f "$d/lint.md" ] && cat "$d/lint.md" >> "$WT/visual-shots/lint.md"
  if [ -f "$d/index.json" ]; then cp "$d/index.json" "$WT/visual-shots/index-$(basename "$d").json"; fi
done < <(find "$OUT/dl" -type d -path '*/ui-visual-audit/*/shots' -print0)
n=$(find "$WT/visual-shots" -name '*.png' | wc -l | tr -d ' ')
[ "$n" -gt 0 ] || { echo "no shots in run $RUN (was it before ci(visual) 338ceb24, or did visual-lint not run?)"; exit 3; }
echo "run $RUN @ ${SHA:0:8}: $n shots -> K2.8 review"
cd $ROOT/.worktrees/r3-integration
MODEL=kimi-for-coding PROVIDER=kimi bash scripts/agents/ext-agent.sh "$WT" $ROOT/output/integrator/tools/visual-review-prompt.md "$OUT" high > "$OUT/run.log" 2>&1 || true
F="$WT/output/visual-review/findings.md"
[ -f "$F" ] && cp "$F" "$OUT/findings.md" && head -1 "$OUT/findings.md" && grep -c '^- \[P1\]' "$OUT/findings.md" | sed 's/^/P1 count: /' || echo "no findings file; see $OUT/run.log"
