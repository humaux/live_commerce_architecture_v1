#!/usr/bin/env bash
# Purpose: Repeat the real identity mode five times under bounded low-priority CPU load; no retries.
# Depends on: bash, node, nice, existing heartbeat-locked test-local.sh; run only in this unit worktree.
# Used by: fix-settings-real-flake local acceptance; stops only its own load children.
set -uo pipefail
cd "$(dirname "$0")/../.."
out=output/fix-settings-real-flake
load_pids=()
cleanup() { for pid in "${load_pids[@]}"; do kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; done; }
trap cleanup EXIT
trap 'exit 130' INT TERM
for worker in 1 2 3 4; do
  nice -n 19 node -e 'const end=Date.now()+3600000; let n=0; while(Date.now()<end){ for(let i=0;i<100000;i++)n=Math.sqrt(i+n); }' &
  load_pids+=("$!")
done
for round in 1 2 3 4 5; do
  date -u '+%FT%TZ'
  uptime
  for pid in "${load_pids[@]}"; do ps -p "$pid" -o pid=,ni=,pcpu=,etime=; done
  LC_TEST_LOCK_WAIT=14400 nice -n 10 bash scripts/dev/test-local.sh --browser-identity > "$out/repeat-$round.log" 2>&1
  rc=$?
  printf 'round=%s exit=%s\n' "$round" "$rc"
  tail -5 "$out/repeat-$round.log"
  if [[ "$rc" != 0 ]]; then exit "$rc"; fi
done
