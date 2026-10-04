#!/usr/bin/env bash
# Task-owned evidence orchestration. Waits without touching foreign processes/locks.
set -euo pipefail
cd /Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution
out=output/ads-attribution/r13-review
expected=4acbad5353814fc3940812e9e399fdb1253ed40f
stable=0
for ((attempt=1; attempt<=480; attempt++)); do
  [[ "$(git rev-parse HEAD)" == "$expected" ]] || exit 90
  git diff --quiet HEAD -- . ':!output' || exit 90
  sample=$(node -e 'const os=require("node:os");console.log([...os.loadavg(),os.cpus().length].join(" "))')
  printf '%s %s\n' "$(date -u +%FT%TZ)" "$sample" >> "$out/g07-quiet-preflight.log"
  if awk '{exit !($1<$4 && $2<$4)}' <<<"$sample"; then
    stable=$((stable+1))
  else
    stable=0
  fi
  if ((stable>=3)); then
    printf 'Quiet preflight: 3 samples 30s apart, 1m/5m load below logical CPUs; source=%s\n' "$expected"
    exec bash "$out/run-final.sh" g07
  fi
  sleep 30
done
printf 'NOT_RUN: quiet-machine window unavailable after 4 hours\n' >&2
exit 75
