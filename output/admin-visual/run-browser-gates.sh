#!/usr/bin/env bash
# Purpose: Run existing browser gates without changing their assertions, recording each real command's exit code.
# Depends on: git, scripts/dev/test-local.sh and its isolated fixture/locking implementation; LC_TEST_LOCK_WAIT/LC_SWEEP_WORKERS.
# Used by: admin-visual local verification only; the TSV, not this loop's final shell status, is the per-gate authority.
set -u
cd /Volumes/data/live_commerce_architecture_v1/.worktrees/admin-visual
export LC_TEST_LOCK_WAIT=7200 LC_SWEEP_WORKERS=1
av_commit=$(git rev-parse HEAD)
git merge-base --is-ancestor 15540a95 "$av_commit" || { printf 'NOT_RUN: merge the machine-wide test lock before running gates\n'; exit 2; }
av_stamp=$(date -u +%Y%m%dT%H%M%SZ)
av_dir="output/admin-visual/gates-$av_stamp"
mkdir -p "$av_dir"
printf 'source\tmode\texit\tseconds\tlog\n' > "$av_dir/results.tsv"
for av_mode in "$@"; do
  if [[ "$(git rev-parse HEAD)" != "$av_commit" ]]; then
    printf 'SOURCE_CHANGED; stop before %s\n' "$av_mode"
    exit 2
  fi
  av_start=$SECONDS
  av_log="$av_dir/${av_mode#--}.log"
  bash scripts/dev/test-local.sh "$av_mode" > "$av_log" 2>&1
  av_code=$?
  printf '%s\t%s\t%s\t%s\t%s\n' "$av_commit" "$av_mode" "$av_code" "$((SECONDS-av_start))" "$av_log" >> "$av_dir/results.tsv"
  printf '%s exit=%s seconds=%s log=%s\n' "$av_mode" "$av_code" "$((SECONDS-av_start))" "$av_log"
  # A NOT_RUN must be resolved before queueing another long wait against the
  # same machine resource. Preserve its real exit; do not turn it into a pass.
  if [[ "$av_code" == 2 ]]; then exit 2; fi
done
